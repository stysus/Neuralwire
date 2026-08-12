package scraper

import (
	"context"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func discardLogger() *log.Logger {
	return log.New(io.Discard, "", 0)
}

// articleHTML is a fixture page with navigation, an ad block, a sidebar and
// the actual article body.
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
		<p>Researchers published a landmark study on transformer efficiency this year.</p>
		<p>The new architecture cuts inference cost by 40 percent while improving accuracy.</p>
		<p>Industry observers expect wide adoption across cloud platforms in the coming months.</p>
	</article>
	<aside class="sidebar">
		Related articles: ten tips for machine learning beginners.
		Subscribe to our newsletter for daily updates.
	</aside>
	<footer>Copyright 2026 Neuralwire News. All rights reserved.</footer>
</body>
</html>`

func TestScrapeExtractsArticle(t *testing.T) {
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
	// Core body paragraphs are present.
	for _, want := range []string{
		"landmark study on transformer efficiency",
		"cuts inference cost by 40 percent",
		"wide adoption across cloud platforms",
	} {
		if !strings.Contains(article.Content, want) {
			t.Errorf("Content missing %q; got:\n%s", want, article.Content)
		}
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

func TestNormalizeText(t *testing.T) {
	got := normalizeText("  First line \n\n\n\n  Second line  \n\n  \nThird line \n")
	want := "First line\n\nSecond line\n\nThird line"
	if got != want {
		t.Errorf("normalizeText = %q, want %q", got, want)
	}
}
