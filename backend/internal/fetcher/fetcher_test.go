package fetcher

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/mmcdole/gofeed"

	"neuralwire/backend/internal/models"
	"neuralwire/backend/internal/scraper"
)

// --- test doubles ---------------------------------------------------------

type fakeSourceStore struct {
	sources []models.RSSSource
	updated []int64
}

func (f *fakeSourceStore) ListEnabled() ([]models.RSSSource, error) {
	return f.sources, nil
}

func (f *fakeSourceStore) UpdateLastFetched(id int64, _ time.Time) error {
	f.updated = append(f.updated, id)
	return nil
}

type fakeNewsStore struct {
	mu      sync.Mutex
	urls    map[string]bool
	created []models.News
}

func newFakeNewsStore() *fakeNewsStore {
	return &fakeNewsStore{urls: map[string]bool{}}
}

func (f *fakeNewsStore) ExistsByURL(url string) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.urls[url], nil
}

func (f *fakeNewsStore) Create(n models.News) (int64, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.urls[n.URL] = true
	f.created = append(f.created, n)
	return int64(len(f.created)), nil
}

type fakeSummarizer struct {
	mu       sync.Mutex
	contents []string
}

func (f *fakeSummarizer) Summarize(_ context.Context, title, content string) string {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.contents = append(f.contents, content)
	return "summary:" + title
}

func (f *fakeSummarizer) lastContent() string {
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.contents) == 0 {
		return ""
	}
	return f.contents[len(f.contents)-1]
}

type fakeScraper struct {
	mu     sync.Mutex
	calls  int
	result *scraper.Article
	err    error
	// failURL, when set, makes Scrape fail only for this exact URL.
	failURL string
}

func (f *fakeScraper) Scrape(_ context.Context, url string) (*scraper.Article, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls++
	if f.failURL != "" && url == f.failURL {
		return nil, errors.New("scrape failed")
	}
	if f.result != nil {
		// Echo the requested URL so tests can verify which item was scraped.
		cp := *f.result
		cp.URL = url
		return &cp, f.err
	}
	return nil, f.err
}

func (f *fakeScraper) callCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.calls
}

// --- helpers --------------------------------------------------------------

func testFeed(items ...*gofeed.Item) *gofeed.Feed {
	return &gofeed.Feed{Items: items}
}

func testItem(title, link, content string) *gofeed.Item {
	return &gofeed.Item{Title: title, Link: link, Content: content}
}

func newTestFetcher(scraperImpl ContentScraper, scrapeMax int, logBuf *bytes.Buffer) *Fetcher {
	logger := log.New(io.Discard, "", 0)
	if logBuf != nil {
		logger = log.New(logBuf, "", 0)
	}
	return &Fetcher{
		sources:    &fakeSourceStore{sources: []models.RSSSource{{ID: 1, Name: "Test Feed", Category: "ai"}}},
		news:       newFakeNewsStore(),
		summarizer: &fakeSummarizer{},
		scraper:    scraperImpl,
		scrapeMax:  scrapeMax,
		client:     nil,
		parser:     gofeed.NewParser(),
		logger:     logger,
	}
}

// --- tests ----------------------------------------------------------------

func TestPreferTitle(t *testing.T) {
	tests := []struct {
		name    string
		rss     string
		scraped string
		want    string
	}{
		{name: "scraped longer wins", rss: "Short title", scraped: "A much longer and descriptive title", want: "A much longer and descriptive title"},
		{name: "rss longer wins", rss: "A very long curated rss title about the story", scraped: "Short", want: "A very long curated rss title about the story"},
		{name: "scraped suffix stripped", rss: "Story Title", scraped: "Story Title - Example News Site", want: "Story Title"},
		{name: "scraped empty keeps rss", rss: "Story Title", scraped: "", want: "Story Title"},
		{name: "rss empty uses scraped", rss: "", scraped: "Scraped Only", want: "Scraped Only"},
		{name: "both empty", rss: "", scraped: "", want: ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := preferTitle(tt.rss, tt.scraped); got != tt.want {
				t.Errorf("preferTitle(%q, %q) = %q, want %q", tt.rss, tt.scraped, got, tt.want)
			}
		})
	}
}

func TestProcessFeedUsesScrapedContent(t *testing.T) {
	f := newTestFetcher(&fakeScraper{
		result: &scraper.Article{Title: "Full Article Title - Test Feed", Content: "FULL BODY TEXT FROM SCRAPER"},
	}, 20, nil)

	src := f.sources.(*fakeSourceStore)
	feed := testFeed(
		testItem("RSS Title", "https://example.com/a", "Short excerpt A"),
	)
	if err := f.processFeed(context.Background(), src.sources[0], feed); err != nil {
		t.Fatalf("processFeed: %v", err)
	}

	news := f.news.(*fakeNewsStore)
	if len(news.created) != 1 {
		t.Fatalf("created %d articles, want 1", len(news.created))
	}
	created := news.created[0]
	if created.Content != "FULL BODY TEXT FROM SCRAPER" {
		t.Errorf("Content = %q, want scraped body", created.Content)
	}
	if created.Title != "Full Article Title" {
		t.Errorf("Title = %q, want cleaned scraped title", created.Title)
	}
}

func TestProcessFeedFallsBackOnScrapeError(t *testing.T) {
	f := newTestFetcher(&fakeScraper{err: io.ErrUnexpectedEOF}, 20, nil)

	src := f.sources.(*fakeSourceStore)
	feed := testFeed(
		testItem("RSS Title", "https://example.com/a", "<p>Excerpt A</p>"),
	)
	if err := f.processFeed(context.Background(), src.sources[0], feed); err != nil {
		t.Fatalf("processFeed: %v", err)
	}

	news := f.news.(*fakeNewsStore)
	if len(news.created) != 1 {
		t.Fatalf("created %d articles, want 1", len(news.created))
	}
	created := news.created[0]
	if created.Content != "<p>Excerpt A</p>" {
		t.Errorf("Content = %q, want RSS excerpt fallback", created.Content)
	}
	if created.Title != "RSS Title" {
		t.Errorf("Title = %q, want RSS title", created.Title)
	}
}

func TestProcessFeedNoScraperUsesExcerpt(t *testing.T) {
	f := newTestFetcher(nil, 20, nil)

	src := f.sources.(*fakeSourceStore)
	feed := testFeed(
		testItem("T", "https://example.com/a", ""),
		testItem("T2", "https://example.com/b", "<p>Desc B</p>"),
	)
	// First item has no content/description at all.
	fi := feed.Items[0]
	fi.Description = "Description fallback A"
	if err := f.processFeed(context.Background(), src.sources[0], feed); err != nil {
		t.Fatalf("processFeed: %v", err)
	}

	news := f.news.(*fakeNewsStore)
	if len(news.created) != 2 {
		t.Fatalf("created %d articles, want 2", len(news.created))
	}
	if news.created[0].Content != "Description fallback A" {
		t.Errorf("Content[0] = %q, want description fallback", news.created[0].Content)
	}
	if news.created[1].Content != "<p>Desc B</p>" {
		t.Errorf("Content[1] = %q, want content field", news.created[1].Content)
	}
}

func TestProcessFeedScrapeBudget(t *testing.T) {
	scraperImpl := &fakeScraper{result: &scraper.Article{Title: "Scraped", Content: "FULL"}}
	f := newTestFetcher(scraperImpl, 2, nil)

	src := f.sources.(*fakeSourceStore)
	feed := testFeed()
	for i := 0; i < 5; i++ {
		feed.Items = append(feed.Items, testItem(
			"Story "+itoa(i),
			"https://example.com/"+itoa(i),
			"Excerpt "+itoa(i),
		))
	}
	if err := f.processFeed(context.Background(), src.sources[0], feed); err != nil {
		t.Fatalf("processFeed: %v", err)
	}

	if got := scraperImpl.callCount(); got != 2 {
		t.Errorf("scrape calls = %d, want 2 (budget)", got)
	}
	news := f.news.(*fakeNewsStore)
	if len(news.created) != 5 {
		t.Fatalf("created %d articles, want 5", len(news.created))
	}
	if news.created[0].Content != "FULL" || news.created[1].Content != "FULL" {
		t.Errorf("newest items should be scraped, got %q / %q", news.created[0].Content, news.created[1].Content)
	}
	if news.created[2].Content != "Excerpt 2" || news.created[4].Content != "Excerpt 4" {
		t.Errorf("items beyond budget should fall back, got %q / %q", news.created[2].Content, news.created[4].Content)
	}
}

func TestProcessFeedSkipsDuplicatesBeforeScrape(t *testing.T) {
	scraperImpl := &fakeScraper{result: &scraper.Article{Title: "Scraped", Content: "FULL"}}
	f := newTestFetcher(scraperImpl, 20, nil)

	news := f.news.(*fakeNewsStore)
	news.urls["https://example.com/dup"] = true

	src := f.sources.(*fakeSourceStore)
	feed := testFeed(
		testItem("Duplicate", "https://example.com/dup", "Excerpt"),
		testItem("Fresh", "https://example.com/fresh", "Excerpt"),
	)
	if err := f.processFeed(context.Background(), src.sources[0], feed); err != nil {
		t.Fatalf("processFeed: %v", err)
	}

	if got := scraperImpl.callCount(); got != 1 {
		t.Errorf("scrape calls = %d, want 1 (duplicate skipped)", got)
	}
	if len(news.created) != 1 {
		t.Errorf("created %d articles, want 1", len(news.created))
	}
}

func TestProcessFeedSummarizerGetsDeterminedContent(t *testing.T) {
	f := newTestFetcher(&fakeScraper{
		result: &scraper.Article{Title: "Scraped Title", Content: "FULL SCRAPED BODY"},
	}, 20, nil)

	sum := f.summarizer.(*fakeSummarizer)
	src := f.sources.(*fakeSourceStore)
	feed := testFeed(
		testItem("RSS Title", "https://example.com/a", "Short excerpt"),
	)
	if err := f.processFeed(context.Background(), src.sources[0], feed); err != nil {
		t.Fatalf("processFeed: %v", err)
	}
	if got := sum.lastContent(); got != "FULL SCRAPED BODY" {
		t.Errorf("summarizer received %q, want full scraped body", got)
	}
}

func TestProcessFeedLogsScrapedVsFallback(t *testing.T) {
	var buf bytes.Buffer
	scraperImpl := &fakeScraper{
		result:  &scraper.Article{Title: "Scraped", Content: "FULL"},
		failURL: "https://example.com/b",
	}
	f := newTestFetcher(scraperImpl, 20, &buf)

	src := f.sources.(*fakeSourceStore)
	feed := testFeed(
		testItem("A", "https://example.com/a", "Excerpt A"),
		testItem("B", "https://example.com/b", "Excerpt B"),
	)
	if err := f.processFeed(context.Background(), src.sources[0], feed); err != nil {
		t.Fatalf("processFeed: %v", err)
	}

	line := buf.String()
	if !strings.Contains(line, "inserted 2 new draft(s) (scraped: 1, fallback: 1)") {
		t.Errorf("log line = %q, want scraped/fallback counts", strings.TrimSpace(line))
	}
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var out []byte
	for n > 0 {
		out = append([]byte{byte('0' + n%10)}, out...)
		n /= 10
	}
	return string(out)
}
