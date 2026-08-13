// Package scraper fetches article URLs and extracts the full readable
// content (title + clean HTML body) from the page. It uses a
// readability-style extraction that strips navigation, ads, sidebars and
// other boilerplate while preserving the article's semantic structure
// (headings, paragraphs, lists, figures and images).
package scraper

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	readability "codeberg.org/readeck/go-readability/v2"
	"github.com/PuerkitoBio/goquery"
	"golang.org/x/net/html"
)

// Article is the extracted readable content of a single web page.
type Article struct {
	// Title is the cleaned page title; it may be empty.
	Title string
	// Content is the clean article HTML with boilerplate removed and all
	// media URLs rewritten to absolute form. It is safe to render with
	// Svelte's {@html ...}.
	Content string
	// URL is the final URL after redirects were followed.
	URL string
	// ImageURL is the cover/featured image URL extracted from the page (e.g. Open Graph).
	ImageURL string
}

// Options configures the Scraper.
type Options struct {
	// Timeout bounds each scrape attempt including the HTTP round trip and
	// extraction (default 15s).
	Timeout time.Duration
	// UserAgent is sent with every request (default is a browser-like UA).
	UserAgent string
	// MaxBytes caps the size of the response body read (default 5 MiB).
	MaxBytes int64
	// Logger receives scrape diagnostics.
	Logger *slog.Logger
}

// Scraper fetches and extracts readable article content.
type Scraper struct {
	client    *http.Client
	userAgent string
	maxBytes  int64
	timeout   time.Duration
	logger    *slog.Logger
}

// New builds a Scraper.
func New(opts Options) *Scraper {
	if opts.Timeout <= 0 {
		opts.Timeout = 15 * time.Second
	}
	if opts.UserAgent == "" {
		opts.UserAgent = "Mozilla/5.0 (compatible; NeuralwireBot/1.0-dev; +https://neuralwire.example)"
	}
	if opts.MaxBytes <= 0 {
		opts.MaxBytes = 5 << 20 // 5 MiB
	}
	if opts.Logger == nil {
		opts.Logger = slog.Default()
	}
	return &Scraper{
		// The per-request timeout is enforced by the context; the client has
		// no global timeout so redirects are handled by the default policy.
		client:    &http.Client{},
		userAgent: opts.UserAgent,
		maxBytes:  opts.MaxBytes,
		timeout:   opts.Timeout,
		logger:    opts.Logger,
	}
}

// Scrape fetches url and extracts the readable article. It returns an error
// when the page cannot be fetched, is not HTML, is too large, or contains no
// readable body text. Redirects are followed automatically.
func (s *Scraper) Scrape(ctx context.Context, rawURL string) (*Article, error) {
	ctx, cancel := context.WithTimeout(ctx, s.timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("User-Agent", s.userAgent)
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch %s: %w", rawURL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fetch %s: unexpected status %d", rawURL, resp.StatusCode)
	}

	contentType := resp.Header.Get("Content-Type")
	if !isHTMLContentType(contentType) {
		return nil, fmt.Errorf("fetch %s: not HTML (content-type %q)", rawURL, contentType)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, s.maxBytes+1))
	if err != nil {
		return nil, fmt.Errorf("read body: %w", err)
	}
	if int64(len(body)) > s.maxBytes {
		return nil, fmt.Errorf("fetch %s: response exceeds %d bytes", rawURL, s.maxBytes)
	}

	// Use the final URL (after redirects) so relative links resolve.
	finalURL := resp.Request.URL.String()
	pageURL, err := url.Parse(finalURL)
	if err != nil {
		return nil, fmt.Errorf("parse final url: %w", err)
	}

	// Extract Open Graph or meta cover image from the raw page body before cleaning
	metaImage := extractMetaImage(body, pageURL)

	cleaned, err := stripBoilerplate(body)
	if err != nil {
		return nil, fmt.Errorf("clean boilerplate: %w", err)
	}

	article, err := readability.FromReader(bytes.NewReader(cleaned), pageURL)
	if err != nil {
		return nil, fmt.Errorf("extract readable content: %w", err)
	}
	if article.Node == nil {
		return nil, errors.New("no readable content found")
	}

	// Render the extracted article as clean HTML, keeping headings,
	// paragraphs, lists, figures and images, and rewrite media URLs to
	// absolute against the final (post-redirect) URL.
	content, err := renderArticleHTML(article.Node, pageURL)
	if err != nil {
		return nil, fmt.Errorf("render readable content: %w", err)
	}
	if strings.TrimSpace(content) == "" {
		return nil, errors.New("no readable content found")
	}

	return &Article{
		Title:    strings.TrimSpace(article.Title()),
		Content:  content,
		URL:      finalURL,
		ImageURL: metaImage,
	}, nil
}

// boilerplateSelector matches page furniture that must never appear in the
// extracted article: site chrome, navigation, ads, sidebars and interactive
// widgets.
const boilerplateSelector = `
	nav, header, footer, aside, form, script, style, noscript, iframe,
	.menu, .navigation, .navbar, .nav,
	.sidebar, .widget, .related, .recommended,
	.ad, .ads, .advertisement, .ad-container, .adsense, .sponsored, .sponsor,
	.promo, .banner, .banner-ad, .popup, .modal,
	.comments, .share, .social, .newsletter, .subscribe, .cookie, .footnote
`

// stripBoilerplate removes site chrome and advertising containers from the
// raw HTML before readability extraction, so ads and sidebars never leak
// into the stored article body.
func stripBoilerplate(body []byte) ([]byte, error) {
	doc, err := goquery.NewDocumentFromReader(bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	doc.Find(boilerplateSelector).Remove()
	html, err := doc.Html()
	if err != nil {
		return nil, err
	}
	return []byte(html), nil
}

// placeholderRegex matches leftover template placeholders such as
// [[duration]] or [[reading_time]].
var placeholderRegex = regexp.MustCompile(`\[\[[^\]]*\]\]`)

// renderArticleHTML serializes the extracted article node as clean HTML and
// post-processes it: media URLs (img src/srcset, source, video poster) are
// rewritten to absolute URLs, leftover template placeholders and empty links
// are removed, and stray scripts are dropped.
func renderArticleHTML(node *html.Node, base *url.URL) (string, error) {
	doc := goquery.NewDocumentFromNode(node)

	// Stray scripts/styles should never survive extraction.
	doc.Find("script, style, noscript").Remove()

	// Drop elements whose entire content is a template placeholder, e.g.
	// <time>[[duration]]</time>.
	doc.Find("span, div, time, p, figure, figcaption, li, a, i, em, strong, small, b, td, th").
		Each(func(_ int, s *goquery.Selection) {
			if isPlaceholderOnly(s.Text()) {
				s.Remove()
			}
		})

	// Remove links that carry neither text nor a destination.
	doc.Find("a").Each(func(_ int, s *goquery.Selection) {
		if strings.TrimSpace(s.Text()) == "" && s.AttrOr("href", "") == "" {
			s.Remove()
		}
	})

	// Rewrite media URLs to absolute against the final article URL.
	doc.Find("img").Each(func(_ int, s *goquery.Selection) {
		src := s.AttrOr("src", "")
		if src == "" {
			src = s.AttrOr("data-src", "")
		}
		if abs, err := resolveURL(base, src); err == nil {
			s.SetAttr("src", abs)
		}
		if sset := s.AttrOr("srcset", ""); sset != "" {
			s.SetAttr("srcset", resolveSrcset(base, sset))
		}
	})
	doc.Find("source").Each(func(_ int, s *goquery.Selection) {
		if src := s.AttrOr("src", ""); src != "" {
			if abs, err := resolveURL(base, src); err == nil {
				s.SetAttr("src", abs)
			}
		}
		if sset := s.AttrOr("srcset", ""); sset != "" {
			s.SetAttr("srcset", resolveSrcset(base, sset))
		}
	})
	doc.Find("video").Each(func(_ int, s *goquery.Selection) {
		if poster := s.AttrOr("poster", ""); poster != "" {
			if abs, err := resolveURL(base, poster); err == nil {
				s.SetAttr("poster", abs)
			}
		}
	})

	out, err := doc.Html()
	if err != nil {
		return "", err
	}

	// Safety net: strip any placeholder syntax that survived element removal.
	out = placeholderRegex.ReplaceAllString(out, "")
	return strings.TrimSpace(out), nil
}

// isPlaceholderOnly reports whether s contains some text but nothing beyond
// template placeholders and whitespace. Empty text is never placeholder-only,
// so containers holding only media (e.g. <figure><img></figure>) survive.
func isPlaceholderOnly(s string) bool {
	if strings.TrimSpace(s) == "" {
		return false
	}
	stripped := placeholderRegex.ReplaceAllString(s, "")
	return strings.TrimSpace(stripped) == ""
}

// FirstImage returns the src of the first <img> in an HTML fragment, or an
// empty string when the fragment contains no images. The scraped content is
// expected to already have absolute media URLs (renderArticleHTML rewrites
// them), so the returned URL is ready to store in news.image_url.
func FirstImage(content string) string {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(content))
	if err != nil {
		return ""
	}
	img := doc.Find("img").First()
	if img.Length() == 0 {
		return ""
	}
	if src := strings.TrimSpace(img.AttrOr("src", "")); src != "" {
		return src
	}
	if src := strings.TrimSpace(img.AttrOr("data-src", "")); src != "" {
		return src
	}
	return ""
}

// UpgradeImageURL rewrites a cover image URL to request a higher-resolution,
// optimized variant from well-known image CDNs. Many publishers ship cover
// images as small thumbnails (e.g. Contentful's ?w=300&q=30) that look blurry
// when rendered as a 16:9 hero; this raises the width/quality parameters
// while leaving the underlying asset untouched. Unknown hosts are returned
// unchanged.
func UpgradeImageURL(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	u, err := url.Parse(raw)
	if err != nil {
		return raw
	}

	host := strings.ToLower(u.Hostname())
	q := u.Query()
	switch {
	case strings.Contains(host, "ctfassets.net"): // Contentful
		q.Set("w", "1600")
		q.Set("q", "80")
		q.Set("fm", "webp")
	case strings.Contains(host, "imgix.net"): // imgix
		q.Set("w", "1600")
		q.Set("q", "80")
		q.Set("auto", "compress,format")
	case strings.Contains(host, "cloudinary.com"): // Cloudinary
		q.Set("w", "1600")
		q.Set("q", "auto")
		q.Set("f", "auto")
	case strings.Contains(host, "unsplash.com"): // Unsplash
		// Skip the /featured/800x450/ fallback URLs produced by the image
		// generator: they already carry a fixed resolution and a keyword
		// query string that must not be mangled.
		if strings.Contains(u.Path, "/featured/") {
			return raw
		}
		q.Set("w", "1600")
		q.Set("q", "80")
		q.Set("auto", "format")
	case strings.Contains(host, "googleusercontent.com"): // Google CDN
		// googleusercontent URLs embed the size in the path tail (=w300-h200
		// or =s300). Normalize to a wide "=s1600" form.
		path := u.Path
		if idx := strings.LastIndex(path, "="); idx >= 0 {
			u.Path = path[:idx] + "=s1600"
		}
		u.RawQuery = ""
		return u.String()
	case strings.Contains(host, "wp.com") || strings.Contains(host, "wordpress.com") ||
		strings.Contains(host, "github.blog"): // WP.com / Jetpack-style resizing params
		if q.Get("w") != "" {
			q.Set("w", "1600")
		}
		if q.Get("resize") != "" {
			q.Set("resize", "1600,900")
		}
	default:
		// Unknown CDN: leave untouched rather than guessing.
		return raw
	}

	u.RawQuery = q.Encode()
	return u.String()
}

// resolveURL resolves ref against base. It returns data/blob/mailto/
// javascript/tel URIs untouched, and handles protocol-relative URLs.
func resolveURL(base *url.URL, ref string) (string, error) {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return "", errors.New("empty url")
	}
	switch {
	case strings.HasPrefix(strings.ToLower(ref), "data:"),
		strings.HasPrefix(strings.ToLower(ref), "blob:"),
		strings.HasPrefix(strings.ToLower(ref), "mailto:"),
		strings.HasPrefix(strings.ToLower(ref), "javascript:"),
		strings.HasPrefix(strings.ToLower(ref), "tel:"):
		return ref, nil
	}
	u, err := url.Parse(ref)
	if err != nil {
		return "", err
	}
	return base.ResolveReference(u).String(), nil
}

// resolveSrcset rewrites every URL in an HTML srcset attribute
// ("url1 1x, url2 2x") to absolute.
func resolveSrcset(base *url.URL, srcset string) string {
	parts := strings.Split(srcset, ",")
	for i, part := range parts {
		fields := strings.Fields(part)
		if len(fields) == 0 {
			continue
		}
		if abs, err := resolveURL(base, fields[0]); err == nil {
			fields[0] = abs
		}
		parts[i] = strings.Join(fields, " ")
	}
	return strings.Join(parts, ", ")
}

// isHTMLContentType reports whether a Content-Type header looks like HTML.
func isHTMLContentType(ct string) bool {
	ct = strings.ToLower(ct)
	return strings.Contains(ct, "text/html") || strings.Contains(ct, "application/xhtml+xml")
}

// extractMetaImage extracts a representative image URL from the page headers (e.g. og:image, twitter:image).
func extractMetaImage(body []byte, base *url.URL) string {
	doc, err := goquery.NewDocumentFromReader(bytes.NewReader(body))
	if err != nil {
		return ""
	}

	var rawImg string
	// 1. Try og:image
	if val, ok := doc.Find(`meta[property="og:image"]`).Attr("content"); ok && val != "" {
		rawImg = val
	}
	// 2. Try twitter:image
	if rawImg == "" {
		if val, ok := doc.Find(`meta[name="twitter:image"]`).Attr("content"); ok && val != "" {
			rawImg = val
		}
	}
	// 3. Try link image_src
	if rawImg == "" {
		if val, ok := doc.Find(`link[rel="image_src"]`).Attr("href"); ok && val != "" {
			rawImg = val
		}
	}

	if rawImg != "" {
		if abs, err := resolveURL(base, rawImg); err == nil {
			return abs
		}
	}
	return ""
}
