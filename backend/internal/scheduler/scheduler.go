// Package scheduler runs the RSS fetcher on a cron schedule.
package scheduler

import (
	"context"
	"log"
	"time"

	"github.com/robfig/cron/v3"

	"neuralwire/backend/internal/fetcher"
)

// Scheduler drives periodic RSS fetching.
type Scheduler struct {
	cron         *cron.Cron
	fetcher      *fetcher.Fetcher
	logger       *log.Logger
	runOnStartup bool
	fetchTimeout time.Duration
}

// Options configures the Scheduler.
type Options struct {
	CronSchedule string
	Fetcher      *fetcher.Fetcher
	RunOnStartup bool
	FetchTimeout time.Duration
	Logger       *log.Logger
}

// New builds a Scheduler. CronSchedule uses standard 5-field cron syntax;
// the default "0 */6 * * *" runs every 6 hours.
func New(opts Options) *Scheduler {
	if opts.CronSchedule == "" {
		opts.CronSchedule = "0 */6 * * *"
	}
	if opts.FetchTimeout <= 0 {
		opts.FetchTimeout = 10 * time.Minute
	}
	if opts.Logger == nil {
		opts.Logger = log.Default()
	}

	s := &Scheduler{
		cron:         cron.New(),
		fetcher:      opts.Fetcher,
		logger:       opts.Logger,
		runOnStartup: opts.RunOnStartup,
		fetchTimeout: opts.FetchTimeout,
	}

	if _, err := s.cron.AddFunc(opts.CronSchedule, s.runFetch); err != nil {
		s.logger.Printf("scheduler: invalid cron schedule %q, falling back to every 6h: %v", opts.CronSchedule, err)
		if _, err := s.cron.AddFunc("0 */6 * * *", s.runFetch); err != nil {
			panic("scheduler: cannot register fallback schedule: " + err.Error())
		}
	}
	return s
}

// Start begins the cron loop and optionally runs an initial fetch.
func (s *Scheduler) Start() {
	s.cron.Start()
	if s.runOnStartup {
		s.logger.Printf("scheduler: running initial fetch")
		go s.runFetch()
	}
}

// Stop halts the cron loop and waits for running jobs to finish.
func (s *Scheduler) Stop() {
	ctx := s.cron.Stop()
	select {
	case <-ctx.Done():
	case <-time.After(30 * time.Second):
		s.logger.Printf("scheduler: timed out waiting for running job")
	}
}

func (s *Scheduler) runFetch() {
	s.logger.Printf("scheduler: fetch cycle started")
	ctx, cancel := context.WithTimeout(context.Background(), s.fetchTimeout)
	defer cancel()

	if err := s.fetcher.FetchAll(ctx); err != nil {
		s.logger.Printf("scheduler: fetch cycle finished with errors: %v", err)
		return
	}
	s.logger.Printf("scheduler: fetch cycle finished")
}
