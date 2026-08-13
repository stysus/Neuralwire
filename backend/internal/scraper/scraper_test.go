package scraper

import (
	"context"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

func discardLogger() *log.Logger {
	return log.New(io.Discard, "", 0)
}

// articleHTML is a fixture page with navigation, an ad block, a sidebar,
// headings, images (relative and absolute), a figure and template junk that
// must all be handled correctly by the scraper.
const articleHTML = `<!DOCTYPE html>
<html>
<head>
	<title>Deep Learning Advances in 2025 - Neuralwire News</title>
	<meta property="og:title" content="Deep Learning Advances in 2025">
</head>
<body>
	<nav>
		<a href="/">Home</a>
		<a href="/tech">Tech</a>
		<a href="/about">About us</a>
	</nav>
	<header>
		<h1>Deep Learning Advances in 2025</h1>
		<div class="byline">By Jane Doe</div>
	</header>
	<div class="ad">
		Sponsored content: Buy our product today! Limited offer.
	</div>
	<article>
		<h2>Why efficiency matters</h2>
		<p>Researchers published a landmark study on transformer efficiency this year.</p>
		<figure>
			<img src="/img/chart.png" alt="Efficiency chart">
			<figcaption>Figure 1: Inference cost over time.</figcaption>
		</figure>
		<p>The new architecture cuts inference cost by 40 percent while improving accuracy.</p>
		<img src="https://cdn.example.com/banner.jpg" alt="Banner">
		<p>Industry observers expect wide adoption across cloud platforms in the coming months.</p>
		<figure>
			<img src="/img/hero.png" alt="Hero image">
		</figure>
		<time>[[duration]]</time>
		<span class="share-count">[[share_count]]</span>
		<a class="track" href=""></a>
		<ul>
			<li>First finding</li>
			<li>Second finding</li>
		</ul>
	</article>
	<aside class="sidebar">
		Related articles: ten tips for machine learning beginners.
		Subscribe to our newsletter for daily updates.
	</aside>
	<footer>Copyright 2026 Neuralwire News. All rights reserved.</footer>
</body>
</html>`

func TestScrapeExtractsCleanHTML(t *testing.T) {
	var gotUA string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotUA = r.Header.Get("User-Agent")
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		io.WriteString(w, articleHTML)
	}))
	defer srv.Close()

	s := New(Options{Logger: discardLogger(), Timeout: 5 * time.Second})
	article, err := s.Scrape(context.Background(), srv.URL)
	if err != nil {
		t.Fatalf("Scrape: %v", err)
	}

	// Title is the cleaned page title without the site suffix.
	if article.Title != "Deep Learning Advances in 2025" {
		t.Errorf("Title = %q, want %q", article.Title, "Deep Learning Advances in 2025")
	}

	// Content is semantic HTML: headings, paragraphs, lists and figures.
	for _, tag := range []string{"<h2", "<p", "<ul", "<li", "<figure", "<img"} {
		if !strings.Contains(article.Content, tag) {
			t.Errorf("Content missing %q element:\n%s", tag, article.Content)
		}
	}

	// Relative image src is rewritten to an absolute URL.
	if want := srv.URL + "/img/chart.png"; !strings.Contains(article.Content, want) {
		t.Errorf("Content missing absolute image URL %q:\n%s", want, article.Content)
	}
	// Absolute image URL is preserved.
	if !strings.Contains(article.Content, "https://cdn.example.com/banner.jpg") {
		t.Errorf("Content missing absolute image URL:\n%s", article.Content)
	}
	// A figure containing only an image (no caption) must survive cleanup.
	if want := srv.URL + "/img/hero.png"; !strings.Contains(article.Content, want) {
		t.Errorf("Content missing media-only figure image %q:\n%s", want, article.Content)
	}

	// Core body text survives.
	for _, want := range []string{
		"landmark study on transformer efficiency",
		"cuts inference cost by 40 percent",
		"wide adoption across cloud platforms",
		"First finding",
	} {
		if !strings.Contains(article.Content, want) {
			t.Errorf("Content missing %q; got:\n%s", want, article.Content)
		}
	}

	// Template placeholders and empty links are removed.
	if strings.Contains(article.Content, "[[duration]]") || strings.Contains(article.Content, "[[share_count]]") {
		t.Errorf("Content contains template placeholder:\n%s", article.Content)
	}
	if strings.Contains(article.Content, "track") {
		t.Errorf("Content contains empty link element:\n%s", article.Content)
	}

	// Boilerplate is stripped.
	for _, unwanted := range []string{"Sponsored content", "ten tips for machine learning", "Subscribe to our newsletter", "All rights reserved"} {
		if strings.Contains(article.Content, unwanted) {
			t.Errorf("Content contains boilerplate %q:\n%s", unwanted, article.Content)
		}
	}

	// A browser-like User-Agent is sent.
	if gotUA == "" || !strings.Contains(gotUA, "Mozilla") {
		t.Errorf("User-Agent = %q, want browser-like UA", gotUA)
	}
}

func TestScrapeFollowsRedirect(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/article", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		io.WriteString(w, articleHTML)
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/article", http.StatusFound)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	s := New(Options{Logger: discardLogger(), Timeout: 5 * time.Second})
	article, err := s.Scrape(context.Background(), srv.URL)
	if err != nil {
		t.Fatalf("Scrape: %v", err)
	}
	if article.URL != srv.URL+"/article" {
		t.Errorf("URL = %q, want %q (final redirect target)", article.URL, srv.URL+"/article")
	}
	// Relative images resolve against the FINAL URL after redirects.
	if want := srv.URL + "/img/chart.png"; !strings.Contains(article.Content, want) {
		t.Errorf("Content missing absolute image URL %q after redirect:\n%s", want, article.Content)
	}
	if !strings.Contains(article.Content, "transformer efficiency") {
		t.Errorf("Content missing after redirect:\n%s", article.Content)
	}
}

func TestScrapeRejectsNonHTML(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, `{"ok":true}`)
	}))
	defer srv.Close()

	s := New(Options{Logger: discardLogger(), Timeout: 5 * time.Second})
	if _, err := s.Scrape(context.Background(), srv.URL); err == nil {
		t.Error("Scrape(non-HTML) succeeded, want error")
	}
}

func TestScrapeRejectsErrorStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "gone", http.StatusNotFound)
	}))
	defer srv.Close()

	s := New(Options{Logger: discardLogger(), Timeout: 5 * time.Second})
	if _, err := s.Scrape(context.Background(), srv.URL); err == nil {
		t.Error("Scrape(404) succeeded, want error")
	}
}

func TestScrapeTimeout(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(2 * time.Second)
	}))
	defer srv.Close()

	s := New(Options{Logger: discardLogger(), Timeout: 200 * time.Millisecond})
	start := time.Now()
	if _, err := s.Scrape(context.Background(), srv.URL); err == nil {
		t.Fatal("Scrape(slow page) succeeded, want timeout error")
	}
	if elapsed := time.Since(start); elapsed > time.Second {
		t.Errorf("Scrape took %s, want to time out quickly", elapsed)
	}
}

func TestScrapeHonorsParentContext(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(2 * time.Second)
	}))
	defer srv.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // already cancelled

	s := New(Options{Logger: discardLogger(), Timeout: 5 * time.Second})
	if _, err := s.Scrape(ctx, srv.URL); err == nil {
		t.Error("Scrape(cancelled ctx) succeeded, want error")
	}
}

func TestResolveURL(t *testing.T) {
	base, _ := url.Parse("https://example.com/news/2025/transformers")

	tests := []struct {
		name string
		ref  string
		want string
	}{
		{name: "relative", ref: "img/chart.png", want: "https://example.com/news/2025/img/chart.png"},
		{name: "absolute path", ref: "/img/chart.png", want: "https://example.com/img/chart.png"},
		{name: "absolute url", ref: "https://cdn.example.com/b.jpg", want: "https://cdn.example.com/b.jpg"},
		{name: "protocol relative", ref: "//cdn.example.com/b.jpg", want: "https://cdn.example.com/b.jpg"},
		{name: "data uri untouched", ref: "data:image/png;base64,AAAA", want: "data:image/png;base64,AAAA"},
		{name: "parent traversal", ref: "../../img/x.png", want: "https://example.com/img/x.png"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := resolveURL(base, tt.ref)
			if err != nil {
				t.Fatalf("resolveURL(%q): %v", tt.ref, err)
			}
			if got != tt.want {
				t.Errorf("resolveURL(%q) = %q, want %q", tt.ref, got, tt.want)
			}
		})
	}

	if _, err := resolveURL(base, ""); err == nil {
		t.Error("resolveURL(empty) succeeded, want error")
	}
}

func TestResolveSrcset(t *testing.T) {
	base, _ := url.Parse("https://example.com/story")
	got := resolveSrcset(base, "/img/a.jpg 1x, https://cdn.example.com/b.jpg 2x, /img/c.png 480w")
	want := "https://example.com/img/a.jpg 1x, https://cdn.example.com/b.jpg 2x, https://example.com/img/c.png 480w"
	if got != want {
		t.Errorf("resolveSrcset = %q, want %q", got, want)
	}
}

func TestIsPlaceholderOnly(t *testing.T) {
	if !isPlaceholderOnly("[[duration]]") {
		t.Error("isPlaceholderOnly([[duration]]) = false, want true")
	}
	if !isPlaceholderOnly("  [[reading_time]]  \n") {
		t.Error("isPlaceholderOnly(whitespace wrapped) = false, want true")
	}
	if isPlaceholderOnly("Read for [[duration]] minutes") {
		t.Error("isPlaceholderOnly(with real text) = true, want false")
	}
	if isPlaceholderOnly("") {
		t.Error("isPlaceholderOnly(empty) = true, want false")
	}
}

func TestFirstImage(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    string
	}{
		{
			name:    "first image src",
			content: `<p>Intro</p><img src="https://cdn.example.com/a.png"><img src="https://cdn.example.com/b.png">`,
			want:    "https://cdn.example.com/a.png",
		},
		{
			name:    "data-src fallback",
			content: `<p>Intro</p><img data-src="https://cdn.example.com/lazy.png" class="lazy">`,
			want:    "https://cdn.example.com/lazy.png",
		},
		{
			name:    "empty src uses data-src",
			content: `<img src="" data-src="https://cdn.example.com/ds.png">`,
			want:    "https://cdn.example.com/ds.png",
		},
		{
			name:    "no images",
			content: `<p>Just text, no media.</p>`,
			want:    "",
		},
		{
			name:    "malformed html",
			content: `<p>unclosed`,
			want:    "",
		},
		{
			name:    "empty content",
			content: "",
			want:    "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := FirstImage(tt.content); got != tt.want {
				t.Errorf("FirstImage() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestUpgradeImageURL(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "contentful low res gets upgraded",
			in:   "https://images.ctfassets.net/space/asset/cover.webp?w=300&q=30",
			want: "https://images.ctfassets.net/space/asset/cover.webp?fm=webp&q=80&w=1600",
		},
		{
			name: "contentful no params gets defaults",
			in:   "https://images.ctfassets.net/space/asset/cover.jpg",
			want: "https://images.ctfassets.net/space/asset/cover.jpg?fm=webp&q=80&w=1600",
		},
		{
			name: "imgix",
			in:   "https://example.imgix.net/photo.jpg?w=200&q=40",
			want: "https://example.imgix.net/photo.jpg?auto=compress%2Cformat&q=80&w=1600",
		},
		{
			name: "cloudinary",
			in:   "https://res.cloudinary.com/demo/image/upload/w_300/v1/sample.jpg",
			want: "https://res.cloudinary.com/demo/image/upload/w_300/v1/sample.jpg?f=auto&q=auto&w=1600",
		},
		{
			name: "unsplash",
			in:   "https://images.unsplash.com/photo-123?w=600&q=50&auto=format",
			want: "https://images.unsplash.com/photo-123?auto=format&q=80&w=1600",
		},
		{
			name: "googleusercontent path sizing",
			in:   "https://lh3.googleusercontent.com/abc123=w300-h200-p",
			want: "https://lh3.googleusercontent.com/abc123=s1600",
		},
		{
			name: "wp.com resize",
			in:   "https://github.blog/wp-content/uploads/2026/08/f.png?resize=1024%2C576",
			want: "https://github.blog/wp-content/uploads/2026/08/f.png?resize=1600%2C900",
		},
		{
			name: "unknown host untouched",
			in:   "https://cdn.example.com/img.png?w=100",
			want: "https://cdn.example.com/img.png?w=100",
		},
		{
			name: "empty",
			in:   "",
			want: "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := UpgradeImageURL(tt.in); got != tt.want {
				t.Errorf("UpgradeImageURL(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}
