// Package scheduler runs the optional auto-fetch / auto-publish pipeline.
// It reads its configuration from app_settings (editable via the admin
// panel), so the operator can enable it, change the interval, and filter
// auto-published articles by category and value score — without touching
// code or env vars (STY-57).
package scheduler

import (
	"context"
	"log/slog"
	"strings"
	"sync"
	"time"

	"neuralwire/backend/internal/fetcher"
	"neuralwire/backend/internal/models"
	"neuralwire/backend/internal/repository"
)

// Job runs one fetch + (optionally) auto-publish cycle.
type Job struct {
	Fetch    func(ctx context.Context) (fetcher.FetchStats, error)
	AutoPost func(ctx context.Context, cfg models.AutoPublishConfig) (int, error)
}

// Scheduler periodically runs a Job while the configuration is enabled.
type Scheduler struct {
	repo    *repository.SettingsRepository
	job     Job
	logger  *slog.Logger
	stop    chan struct{}
	stopped chan struct{}
	mu      sync.Mutex
	// running reports whether the scheduler loop goroutine is alive. It is
	// toggled by Start/Stop (i.e. the admin "Start/Stop config" buttons).
	running bool
	// active reports whether the stored config has Enabled=true. The loop
	// checks it each tick; admin can save config (enabled true) without the
	// loop acting on it until Start is pressed.
	active  bool
	lastRun time.Time
}

// New builds a Scheduler. A nil logger uses slog.Default().
func New(repo *repository.SettingsRepository, job Job, logger *slog.Logger) *Scheduler {
	if logger == nil {
		logger = slog.Default()
	}
	return &Scheduler{
		repo:    repo,
		job:     job,
		logger:  logger,
		stop:    make(chan struct{}),
		stopped: make(chan struct{}),
	}
}

// Start launches the scheduler loop. It is idempotent: subsequent calls are
// no-ops while the scheduler is already running. The loop reads the config
// from the repository on every tick, so admin-panel changes take effect on
// the next cycle without a restart.
func (s *Scheduler) Start() {
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return
	}
	s.running = true
	s.mu.Unlock()

	go s.loop()
}

// Stop terminates the scheduler loop.
func (s *Scheduler) Stop() {
	s.mu.Lock()
	if !s.running {
		s.mu.Unlock()
		return
	}
	s.running = false
	close(s.stop)
	s.mu.Unlock()
	<-s.stopped
}

// SetActive toggles whether the scheduler acts on the stored config. This is
// the admin "Start config / Stop config" control: saving config with
// Enabled=true does not run anything until SetActive(true) is called.
func (s *Scheduler) SetActive(active bool) {
	s.mu.Lock()
	s.active = active
	s.mu.Unlock()
}

// Active reports whether the scheduler is currently active.
func (s *Scheduler) Active() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.active
}

func (s *Scheduler) loop() {
	defer close(s.stopped)

	// Run immediately if enabled so the operator sees the effect right away;
	// otherwise wait one interval before the first check.
	s.runOnceIfEnabled()

	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-s.stop:
			return
		case <-ticker.C:
			s.runOnceIfEnabled()
		}
	}
}

// runOnceIfEnabled checks the stored config and, when the scheduler is
// active (Start config pressed) and the stored config has Enabled=true, runs
// a cycle if the interval has elapsed.
func (s *Scheduler) runOnceIfEnabled() {
	s.mu.Lock()
	active := s.active
	s.mu.Unlock()
	if !active {
		return
	}

	cfg := s.repo.GetAutoPublishConfig()
	if !cfg.Enabled {
		return
	}

	interval := time.Duration(cfg.IntervalMinutes) * time.Minute
	if interval <= 0 {
		interval = 6 * time.Hour
	}

	// Re-run at most once per interval, using an in-memory marker. On
	// process restart the first cycle runs immediately, which is acceptable.
	s.mu.Lock()
	if !s.lastRun.IsZero() && time.Since(s.lastRun) < interval {
		s.mu.Unlock()
		return
	}
	s.mu.Unlock()

	s.logger.Info("scheduler: running auto cycle", "interval_minutes", cfg.IntervalMinutes)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()

	stats, err := s.job.Fetch(ctx)
	if err != nil {
		s.logger.Error("scheduler: fetch cycle failed", "error", err)
	} else {
		s.logger.Info("scheduler: fetch complete",
			"new", stats.TotalNew,
			"scraped", stats.Scraped,
			"fallback", stats.Fallback,
		)
	}

	if cfg.AutoPostEnabled {
		n, err := s.job.AutoPost(ctx, cfg)
		if err != nil {
			s.logger.Error("scheduler: auto post failed", "error", err)
		} else {
			s.logger.Info("scheduler: auto post complete", "published", n)
		}
	}

	s.mu.Lock()
	s.lastRun = time.Now()
	s.mu.Unlock()
}

// filterMatch reports whether an article passes the auto-publish category and
// score filters. An empty categories whitelist means "all".
func filterMatch(n models.News, cfg models.AutoPublishConfig) bool {
	if len(cfg.Categories) > 0 {
		found := false
		for _, c := range cfg.Categories {
			if strings.EqualFold(strings.TrimSpace(c), strings.TrimSpace(n.Category)) {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	if cfg.MinScoreLabel != "" {
		// Label comparison is order-independent here: the caller passes the
		// minimum label, and the article must be at or above it. The simple
		// ranking is low < medium < high.
		if !labelAtLeast(n.ValueLabel, cfg.MinScoreLabel) {
			return false
		}
	}
	return true
}

// labelAtLeast reports whether got is at least want in the low < medium <
// high ranking. Unknown labels are treated as below any threshold.
func labelAtLeast(got, want string) bool {
	rank := map[string]int{"low": 1, "medium": 2, "high": 3}
	g, ok := rank[strings.ToLower(strings.TrimSpace(got))]
	if !ok {
		return false
	}
	w, ok := rank[strings.ToLower(strings.TrimSpace(want))]
	if !ok {
		return false
	}
	return g >= w
}
