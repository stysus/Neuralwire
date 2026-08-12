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

	"github.com/mmcdole/gofeed"

	"neuralwire/backend/internal/ai"
	"neuralwire/backend/internal/models"
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

// FetcherOptions configures the fetcher.
type FetcherOptions struct {
	Sources    SourceStore
	News       NewsStore
	Summarizer ai.Summarizer
	HTTPClient *http.Client
	Logger     *log.Logger
}

// Fetcher polls RSS sources and stores new articles as drafts.
type Fetcher struct {
	sources    SourceStore
	news       NewsStore
	summarizer ai.Summarizer
	client     *http.Client
	parser     *gofeed.Parser
	logger     *log.Logger
}

// NewFetcher builds a Fetcher.
func NewFetcher(opts FetcherOptions) *Fetcher {
	if opts.HTTPClient == nil {
		opts.HTTPClient = &http.Client{Timeout: 30 * time.Second}
	}
	if opts.Logger == nil {
		opts.Logger = log.Default()
	}
	parser := gofeed.NewParser()
	parser.Client = opts.HTTPClient
	parser.UserAgent = "Mozilla/5.0 (compatible; NeuralwireBot/1.0; +https://neuralwire.example)"

	return &Fetcher{
		sources:    opts.Sources,
		news:       opts.News,
		summarizer: opts.Summarizer,
		client:     opts.HTTPClient,
		parser:     parser,
		logger:     opts.Logger,
	}
}

// FetchAll polls every enabled source. Errors on individual sources are
// logged and do not abort the remaining sources.
func (f *Fetcher) FetchAll(ctx context.Context) error {
	sources, err := f.sources.ListEnabled()
	if err != nil {
		return err
	}
	if len(sources) == 0 {
		f.logger.Printf("fetcher: no enabled rss sources")
		return nil
	}

	f.logger.Printf("fetcher: fetching %d rss source(s)", len(sources))
	var fetchErr error
	for _, src := range sources {
		if err := f.fetchSource(ctx, src); err != nil {
			fetchErr = errors.Join(fetchErr, err)
			f.logger.Printf("fetcher: source %q failed: %v", src.Name, err)
		}
	}
	return fetchErr
}

func (f *Fetcher) fetchSource(ctx context.Context, src models.RSSSource) error {
	feed, err := f.parser.ParseURLWithContext(src.URL, ctx)
	if err != nil {
		return err
	}

	inserted := 0
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
		if title == "" {
			continue
		}

		content := item.Content
		if strings.TrimSpace(content) == "" {
			content = item.Description
		}

		summary := f.summarizer.Summarize(ctx, title, content)

		article := models.News{
			Title:    title,
			URL:      link,
			Source:   src.Name,
			Category: src.Category,
			Summary:  summary,
			Content:  content,
			ImageURL: firstImage(item),
			Status:   models.StatusDraft,
		}

		if _, err := f.news.Create(article); err != nil {
			f.logger.Printf("fetcher: insert failed for %q: %v", link, err)
			continue
		}
		inserted++
	}

	if err := f.sources.UpdateLastFetched(src.ID, time.Now()); err != nil {
		f.logger.Printf("fetcher: update last_fetched for %q: %v", src.Name, err)
	}
	f.logger.Printf("fetcher: source %q inserted %d new draft(s)", src.Name, inserted)
	return nil
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
