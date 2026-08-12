// Package fetcher polls configured RSS/Atom feeds and inserts articles as
// drafts in a semi-automatic moderation workflow.
package fetcher

import (
	"context"
	"errors"
	"log"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/mmcdole/gofeed"

	"neuralwire/backend/internal/ai"
	"neuralwire/backend/internal/models"
	"neuralwire/backend/internal/scraper"
)

// SourceStore is the subset of RSS source persistence the fetcher needs.
type SourceStore interface {
	ListEnabled() ([]models.RSSSource, error)
	UpdateLastFetched(id int64, t time.Time) error
}

// NewsStore is the subset of news persistence the fetcher needs.
type NewsStore interface {
	ExistsByURL(url string) (bool, error)
	Create(n models.News) (int64, error)
}

// ContentScraper fetches an article URL and extracts its full readable
// content. It is satisfied by *scraper.Scraper and by test doubles.
type ContentScraper interface {
	Scrape(ctx context.Context, url string) (*scraper.Article, error)
}

// defaultScrapeMax caps how many newest articles per source are scraped in
// one fetch cycle.
const defaultScrapeMax = 20

// defaultMinContentChars is the minimum content length for a fetched article
// to be kept as a draft. Shorter content (usually an RSS excerpt) is skipped
// as low quality.
const defaultMinContentChars = 500

// FetcherOptions configures the fetcher.
type FetcherOptions struct {
	Sources    SourceStore
	News       NewsStore
	Summarizer ai.Summarizer
	Scraper    ContentScraper
	// ScrapeMax is the number of newest articles per source to attempt
	// full-content scraping for (default 20). Zero uses the default.
	ScrapeMax int
	// MinContentChars is the minimum content length for a fetched article to
	// be kept as a draft (default 500). Zero uses the default; a negative
	// value disables the quality filter.
	MinContentChars int
	HTTPClient      *http.Client
	Logger          *log.Logger
}

// Fetcher polls RSS sources and stores new articles as drafts.
type Fetcher struct {
	sources         SourceStore
	news            NewsStore
	summarizer      ai.Summarizer
	scraper         ContentScraper
	scrapeMax       int
	minContentChars int
	client          *http.Client
	parser          *gofeed.Parser
	logger          *log.Logger
}

// NewFetcher builds a Fetcher.
func NewFetcher(opts FetcherOptions) *Fetcher {
	if opts.HTTPClient == nil {
		opts.HTTPClient = &http.Client{Timeout: 30 * time.Second}
	}
	if opts.Logger == nil {
		opts.Logger = log.Default()
	}
	if opts.ScrapeMax <= 0 {
		opts.ScrapeMax = defaultScrapeMax
	}
	if opts.MinContentChars == 0 {
		opts.MinContentChars = defaultMinContentChars
	}
	parser := gofeed.NewParser()
	parser.Client = opts.HTTPClient
	parser.UserAgent = "Mozilla/5.0 (compatible; NeuralwireBot/1.0; +https://neuralwire.example)"

	return &Fetcher{
		sources:         opts.Sources,
		news:            opts.News,
		summarizer:      opts.Summarizer,
		scraper:         opts.Scraper,
		scrapeMax:       opts.ScrapeMax,
		minContentChars: opts.MinContentChars,
		client:          opts.HTTPClient,
		parser:          parser,
		logger:          opts.Logger,
	}
}

// SourceStats summarizes one fetch cycle for a single RSS source.
type SourceStats struct {
	Name              string `json:"name"`
	Inserted          int    `json:"inserted"`
	Scraped           int    `json:"scraped"`
	Fallback          int    `json:"fallback"`
	SkippedLowQuality int    `json:"skipped_low_quality"`
	Error             string `json:"error,omitempty"`
}

// FetchStats aggregates the results of a full fetch cycle across sources.
type FetchStats struct {
	TotalNew          int           `json:"total_new"`
	Scraped           int           `json:"scraped"`
	Fallback          int           `json:"fallback"`
	SkippedLowQuality int           `json:"skipped_low_quality"`
	Sources           []SourceStats `json:"sources"`
}

// FetchAll polls every enabled source. Errors on individual sources are
// reported per source and do not abort the remaining sources.
func (f *Fetcher) FetchAll(ctx context.Context) (FetchStats, error) {
	stats := FetchStats{}
	sources, err := f.sources.ListEnabled()
	if err != nil {
		return stats, err
	}
	if len(sources) == 0 {
		f.logger.Printf("fetcher: no enabled rss sources")
		return stats, nil
	}

	f.logger.Printf("fetcher: fetching %d rss source(s)", len(sources))
	var fetchErr error
	for _, src := range sources {
		srcStats, err := f.fetchSource(ctx, src)
		if err != nil {
			fetchErr = errors.Join(fetchErr, err)
			srcStats.Error = err.Error()
			f.logger.Printf("fetcher: source %q failed: %v", src.Name, err)
		}
		stats.Sources = append(stats.Sources, srcStats)
		stats.TotalNew += srcStats.Inserted
		stats.Scraped += srcStats.Scraped
		stats.Fallback += srcStats.Fallback
		stats.SkippedLowQuality += srcStats.SkippedLowQuality
	}
	return stats, fetchErr
}

func (f *Fetcher) fetchSource(ctx context.Context, src models.RSSSource) (SourceStats, error) {
	feed, err := f.parser.ParseURLWithContext(src.URL, ctx)
	if err != nil {
		return SourceStats{Name: src.Name, Error: err.Error()}, err
	}
	return f.processFeed(ctx, src, feed)
}

// processFeed turns parsed feed items into draft articles. For each new
// item it first tries to scrape the full article content from the item's
// link; when scraping is unavailable, fails or the per-source budget is
// exhausted, it falls back to the RSS excerpt. Articles whose final content
// is below the quality threshold are skipped rather than stored as drafts.
// It returns per-source statistics for the cycle.
func (f *Fetcher) processFeed(ctx context.Context, src models.RSSSource, feed *gofeed.Feed) (SourceStats, error) {
	stats := SourceStats{Name: src.Name}
	inserted := 0
	scraped := 0
	fallback := 0
	skippedLowQuality := 0
	scrapeAttempts := 0

	for _, item := range feed.Items {
		if item == nil {
			continue
		}
		link := strings.TrimSpace(item.Link)
		if link == "" {
			continue
		}

		exists, err := f.news.ExistsByURL(link)
		if err != nil {
			f.logger.Printf("fetcher: exists check failed for %q: %v", link, err)
			continue
		}
		if exists {
			continue
		}

		title := strings.TrimSpace(item.Title)
		content := rssContent(item)
		usedScrape := false

		if f.scraper != nil && scrapeAttempts < f.scrapeMax {
			scrapeAttempts++
			article, scrapeErr := f.scraper.Scrape(ctx, link)
			if scrapeErr == nil && article != nil && strings.TrimSpace(article.Content) != "" {
				content = article.Content
				// Prefer the scraped title when it is longer/more descriptive.
				if better := preferTitle(title, article.Title); better != "" {
					title = better
				}
				usedScrape = true
			} else if scrapeErr != nil {
				f.logger.Printf("fetcher: scrape failed for %q: %v", link, scrapeErr)
			}
		}

		if title == "" {
			continue
		}

		// Quality gate: only keep drafts with a substantial article body.
		// Short content means the scrape failed or hit the budget and the RSS
		// excerpt alone is not enough for a complete news article.
		if f.minContentChars > 0 && utf8.RuneCountInString(content) < f.minContentChars {
			skippedLowQuality++
			f.logger.Printf(
				"fetcher: skipped %q (low quality, content %d chars < %d)",
				link, utf8.RuneCountInString(content), f.minContentChars,
			)
			continue
		}

		// Summarize whatever content was determined (full scraped text when
		// available, otherwise the RSS excerpt).
		summary := f.summarizer.Summarize(ctx, title, content)

		imageURL := firstImage(item)
		if usedScrape && !validImageURL(imageURL) {
			// The RSS feed did not provide a usable image; use the first
			// image from the scraped content (already absolute).
			if extracted := scraper.FirstImage(content); extracted != "" {
				imageURL = extracted
			}
		}

		article := models.News{
			Title:    title,
			URL:      link,
			Source:   src.Name,
			Category: src.Category,
			Summary:  summary,
			Content:  content,
			ImageURL: imageURL,
			Status:   models.StatusDraft,
		}

		if _, err := f.news.Create(article); err != nil {
			f.logger.Printf("fetcher: insert failed for %q: %v", link, err)
			continue
		}
		inserted++
		if usedScrape {
			scraped++
		} else {
			fallback++
		}
	}

	stats.Inserted = inserted
	stats.Scraped = scraped
	stats.Fallback = fallback
	stats.SkippedLowQuality = skippedLowQuality

	if err := f.sources.UpdateLastFetched(src.ID, time.Now()); err != nil {
		f.logger.Printf("fetcher: update last_fetched for %q: %v", src.Name, err)
	}
	f.logger.Printf(
		"fetcher: source %q inserted %d new draft(s) (scraped: %d, fallback: %d, skipped-low-quality: %d)",
		src.Name, inserted, scraped, fallback, skippedLowQuality,
	)
	return stats, nil
}

// rssContent returns the richest excerpt the feed provides.
func rssContent(item *gofeed.Item) string {
	content := item.Content
	if strings.TrimSpace(content) == "" {
		content = item.Description
	}
	return content
}

// preferTitle picks the better title between the RSS title and the scraped
// page title. Scraped <title> tags often carry a site suffix ("Story -
// Site"), which is stripped before comparison; the scraped title wins only
// when it is at least as descriptive as the RSS title.
func preferTitle(rssTitle, scrapedTitle string) string {
	rssTitle = strings.TrimSpace(rssTitle)
	scrapedTitle = strings.TrimSpace(scrapedTitle)
	if scrapedTitle == "" {
		return rssTitle
	}
	if rssTitle == "" {
		return scrapedTitle
	}
	for _, sep := range []string{" - ", " | ", " — ", " – "} {
		if i := strings.Index(scrapedTitle, sep); i > 0 {
			if prefix := strings.TrimSpace(scrapedTitle[:i]); prefix != "" {
				scrapedTitle = prefix
			}
			break
		}
	}
	if len(scrapedTitle) >= len(rssTitle) {
		return scrapedTitle
	}
	return rssTitle
}

// firstImage extracts a representative image URL from an RSS item.
func firstImage(item *gofeed.Item) string {
	if item.Image != nil && item.Image.URL != "" {
		return item.Image.URL
	}
	for _, enclosure := range item.Enclosures {
		if strings.HasPrefix(enclosure.Type, "image/") && enclosure.URL != "" {
			return enclosure.URL
		}
	}
	return ""
}

// validImageURL reports whether a URL is usable as an article image. Only
// absolute http(s) URLs qualify; empty, relative or protocol-relative URLs
// are treated as invalid so the scraper can supply a better one.
func validImageURL(u string) bool {
	u = strings.TrimSpace(u)
	return strings.HasPrefix(u, "http://") || strings.HasPrefix(u, "https://")
}
