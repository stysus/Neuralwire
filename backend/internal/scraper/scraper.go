// Package scraper fetches article URLs and extracts the full readable
// content (title + body text) from the HTML. It uses a readability-style
// extraction that strips navigation, ads, sidebars and other boilerplate.
package scraper

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"

	readability "codeberg.org/readeck/go-readability/v2"
	"github.com/PuerkitoBio/goquery"
)

// Article is the extracted readable content of a single web page.
type Article struct {
	// Title is the cleaned page title; it may be empty.
	Title string
	// Content is the readable body text with boilerplate removed.
	Content string
	// URL is the final URL after redirects were followed.
	URL string
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
	Logger *log.Logger
}

// Scraper fetches and extracts readable article content.
type Scraper struct {
	client    *http.Client
	userAgent string
	maxBytes  int64
	timeout   time.Duration
	logger    *log.Logger
}

// New builds a Scraper.
func New(opts Options) *Scraper {
	if opts.Timeout <= 0 {
		opts.Timeout = 15 * time.Second
	}
	if opts.UserAgent == "" {
		opts.UserAgent = "Mozilla/5.0 (compatible; NeuralwireBot/1.0; +https://neuralwire.example)"
	}
	if opts.MaxBytes <= 0 {
		opts.MaxBytes = 5 << 20 // 5 MiB
	}
	if opts.Logger == nil {
		opts.Logger = log.Default()
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

	cleaned, err := stripBoilerplate(body)
	if err != nil {
		return nil, fmt.Errorf("clean boilerplate: %w", err)
	}

	article, err := readability.FromReader(bytes.NewReader(cleaned), pageURL)
	if err != nil {
		return nil, fmt.Errorf("extract readable content: %w", err)
	}

	var text strings.Builder
	if err := article.RenderText(&text); err != nil {
		return nil, fmt.Errorf("render readable content: %w", err)
	}
	content := normalizeText(text.String())
	if content == "" {
		return nil, errors.New("no readable content found")
	}

	return &Article{
		Title:   strings.TrimSpace(article.Title()),
		Content: content,
		URL:     finalURL,
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

// isHTMLContentType reports whether a Content-Type header looks like HTML.
func isHTMLContentType(ct string) bool {
	ct = strings.ToLower(ct)
	return strings.Contains(ct, "text/html") || strings.Contains(ct, "application/xhtml+xml")
}

// normalizeText trims every line and collapses runs of blank lines so the
// stored body text is clean and compact.
func normalizeText(s string) string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	lines := strings.Split(s, "\n")

	out := make([]string, 0, len(lines))
	prevEmpty := false
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			if !prevEmpty && len(out) > 0 {
				out = append(out, "")
			}
			prevEmpty = true
			continue
		}
		out = append(out, line)
		prevEmpty = false
	}
	for len(out) > 0 && out[0] == "" {
		out = out[1:]
	}
	for len(out) > 0 && out[len(out)-1] == "" {
		out = out[:len(out)-1]
	}
	return strings.Join(out, "\n")
}
