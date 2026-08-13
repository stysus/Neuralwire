// Package fetcher polls configured RSS/Atom feeds and inserts articles as
// drafts in a semi-automatic moderation workflow.
package fetcher

import (
	"context"
	"errors"
	"log"
	"math/rand/v2"
	"net/http"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/mmcdole/gofeed"

	"neuralwire/backend/internal/ai"
	"neuralwire/backend/internal/models"
	"neuralwire/backend/internal/scoring"
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
const defaultScrapeMax = 5

// defaultMinContentChars is the minimum content length for a fetched article
// to be kept as a draft. Shorter content (usually an RSS excerpt) is skipped
// as low quality.
const defaultMinContentChars = 500

// FetcherOptions configures the fetcher.
type FetcherOptions struct {
	Sources        SourceStore
	News           NewsStore
	Summarizer     ai.Summarizer
	Scraper        ContentScraper
	ImageGenerator ai.ImageGenerator
	// ScrapeMax is the number of newest articles per source to attempt
	// full-content scraping for (default 20). Zero uses the default.
	ScrapeMax int
	// MinContentChars is the minimum content length for a fetched article to
	// be kept as a draft (default 500). Zero uses the default; a negative
	// value disables the quality filter.
	MinContentChars int
	// ScrapeDelayMin and ScrapeDelayMax bound the politeness delay before
	// every external request (RSS feed fetch and article scrape). A random
	// delay in [min, max] is applied so upstream sites are not hammered.
	// A zero or negative max disables the delay entirely.
	ScrapeDelayMin time.Duration
	ScrapeDelayMax time.Duration
	// MaxInsertPerSource caps how many new articles from one source are
	// stored as drafts in a single fetch cycle, regardless of whether they
	// were scraped or fell back to the RSS excerpt. Zero or negative means
	// unlimited (only the scrape budget applies).
	MaxInsertPerSource int
	// Scorer rates each new article's news value (AI + heuristic weighted)
	// and attaches an advisory score/label. When nil, drafts are inserted
	// with a zero score. Scoring never auto-publishes.
	Scorer *scoring.ScoreService
	// UserAgent is sent on RSS feed requests. Empty uses the default
	// NeuralwireBot dev UA.
	UserAgent  string
	HTTPClient *http.Client
	Logger     *log.Logger
}

// Fetcher polls RSS sources and stores new articles as drafts.
type Fetcher struct {
	sources         SourceStore
	news            NewsStore
	summarizer      ai.Summarizer
	scraper         ContentScraper
	imageGenerator  ai.ImageGenerator
	scrapeMax       int
	minContentChars int
	scrapeDelayMin  time.Duration
	scrapeDelayMax  time.Duration
	maxInsert       int
	scorer          *scoring.ScoreService
	client          *http.Client
	parser          *gofeed.Parser
	logger          *log.Logger
	mu              sync.RWMutex
	progress        FetchProgress
}

// FetchProgress is a point-in-time snapshot of an in-flight fetch cycle,
// reported by GET /api/admin/fetch/progress so the admin UI can render a
// live percentage.
type FetchProgress struct {
	Running       bool      `json:"running"`
	TotalSources  int       `json:"total_sources"`
	DoneSources   int       `json:"done_sources"`
	CurrentSource string    `json:"current_source,omitempty"`
	Percent       int       `json:"percent"`
	StartedAt     time.Time `json:"started_at,omitempty"`
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
	parser.UserAgent = opts.UserAgent
	if parser.UserAgent == "" {
		parser.UserAgent = "Mozilla/5.0 (compatible; NeuralwireBot/1.0-dev; +https://neuralwire.example)"
	}

	return &Fetcher{
		sources:         opts.Sources,
		news:            opts.News,
		summarizer:      opts.Summarizer,
		scraper:         opts.Scraper,
		imageGenerator:  opts.ImageGenerator,
		scrapeMax:       opts.ScrapeMax,
		minContentChars: opts.MinContentChars,
		scrapeDelayMin:  opts.ScrapeDelayMin,
		scrapeDelayMax:  opts.ScrapeDelayMax,
		maxInsert:       opts.MaxInsertPerSource,
		scorer:          opts.Scorer,
		client:          opts.HTTPClient,
		parser:          parser,
		logger:          opts.Logger,
	}
}

// throttle sleeps for a random duration in [ScrapeDelayMin, ScrapeDelayMax]
// before an external request, so upstream sites see at most one request every
// 1-2 seconds. A non-positive max disables the delay. It returns early when
// the context is cancelled.
func (f *Fetcher) throttle(ctx context.Context) error {
	if f.scrapeDelayMax <= 0 {
		return nil
	}
	lo := f.scrapeDelayMin
	hi := f.scrapeDelayMax
	if lo > hi {
		lo, hi = hi, lo
	}
	var d time.Duration
	if hi > lo {
		d = lo + time.Duration(rand.Int64N(int64(hi-lo)))
	} else {
		d = lo
	}
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
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

// Progress returns a snapshot of the current fetch cycle progress.
func (f *Fetcher) Progress() FetchProgress {
	f.mu.RLock()
	defer f.mu.RUnlock()
	return f.progress
}

// setProgress stores the given progress snapshot and updates the percent.
func (f *Fetcher) setProgress(running bool, total, done int, current string, started time.Time) {
	f.mu.Lock()
	defer f.mu.Unlock()
	p := FetchProgress{
		Running:       running,
		TotalSources:  total,
		DoneSources:   done,
		CurrentSource: current,
		StartedAt:     started,
	}
	if total > 0 {
		p.Percent = done * 100 / total
	} else {
		p.Percent = 100
	}
	f.progress = p
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

	started := time.Now()
	f.setProgress(true, len(sources), 0, sources[0].Name, started)
	defer f.setProgress(false, len(sources), len(sources), "", started)

	f.logger.Printf("fetcher: fetching %d rss source(s)", len(sources))
	var fetchErr error
	for i, src := range sources {
		f.setProgress(true, len(sources), i, src.Name, started)
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
	if err := f.throttle(ctx); err != nil {
		return SourceStats{Name: src.Name, Error: err.Error()}, err
	}
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
// The loop stops once maxInsert drafts have been stored for the source.
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
		scrapedImageURL := ""
		usedScrape := false

		if f.scraper != nil && scrapeAttempts < f.scrapeMax {
			scrapeAttempts++
			if err := f.throttle(ctx); err != nil {
				return stats, err
			}
			article, scrapeErr := f.scraper.Scrape(ctx, link)
			if scrapeErr == nil && article != nil && strings.TrimSpace(article.Content) != "" {
				content = article.Content
				scrapedImageURL = article.ImageURL
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

		// Auto-categorize based on article content using AI.
		// The RSS source category is used as the default fallback.
		articleCategory := f.summarizer.Categorize(ctx, title, content, src.Category)

		imageURL := firstImage(item)
		if !validImageURL(imageURL) {
			imageURL = ""
		}

		if imageURL == "" && usedScrape {
			if validImageURL(scrapedImageURL) {
				imageURL = scrapedImageURL
			} else {
				extracted := scraper.FirstImage(content)
				if validImageURL(extracted) {
					imageURL = extracted
				}
			}
		}

		// Request a high-resolution variant from known CDNs (e.g. Contentful's
		// default ?w=300&q=30 thumbnails) so the hero renders sharply.
		if validImageURL(imageURL) {
			imageURL = scraper.UpgradeImageURL(imageURL)
		}

		if imageURL == "" && f.imageGenerator != nil {
			imageURL = f.imageGenerator.Generate(ctx, title, articleCategory)
		}

		article := models.News{
			Title:    title,
			URL:      link,
			Source:   src.Name,
			Category: articleCategory,
			Summary:  summary,
			// Curator model: the full article text is only used as material
			// for the AI summary and is deliberately NOT stored, so the site
			// never republishes original copyrighted content. Readers are
			// directed to the source via the article URL.
			Content:  "",
			ImageURL: imageURL,
			Status:   models.StatusDraft,
		}

		// Advisory news-value scoring: AI + heuristic weighted. This is only a
		// recommendation; the article stays a draft for admin review.
		if f.scorer != nil {
			scoreResult := f.scorer.Score(ctx, title, content, src.Name)
			scoring.Apply(&article, scoreResult)
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

		// Per-source insert budget: stop storing drafts once the cap is hit so
		// a single fetch cycle can never flood drafts even when RSS excerpts
		// are long enough to pass the quality gate.
		if f.maxInsert > 0 && inserted >= f.maxInsert {
			f.logger.Printf(
				"fetcher: source %q reached insert budget %d; ignoring remaining items",
				src.Name, f.maxInsert,
			)
			break
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

func validImageURL(u string) bool {
	u = strings.TrimSpace(u)
	if !strings.HasPrefix(u, "http://") && !strings.HasPrefix(u, "https://") {
		return false
	}
	uLower := strings.ToLower(u)
	skipKeywords := []string{
		"avatar", "logo", "icon", "favicon", "gravatar", "placeholder",
		"spacer", "blank", "transparent", "ad-", "ads-",
	}
	for _, kw := range skipKeywords {
		if strings.Contains(uLower, kw) {
			return false
		}
	}
	return true
}
