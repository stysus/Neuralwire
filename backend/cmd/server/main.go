// Command server runs the Neuralwire REST API backend.
package main

import (
	"context"
	"errors"
	"io"
	"log"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"neuralwire/backend/internal/ai"
	"neuralwire/backend/internal/api"
	"neuralwire/backend/internal/auth"
	"neuralwire/backend/internal/config"
	"neuralwire/backend/internal/database"
	"neuralwire/backend/internal/fetcher"
	"neuralwire/backend/internal/metrics"
	"neuralwire/backend/internal/models"
	"neuralwire/backend/internal/repository"
	"neuralwire/backend/internal/scheduler"
	"neuralwire/backend/internal/scoring"
	"neuralwire/backend/internal/scraper"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	// Structured logging: one slog handler drives everything. Components that
	// still take a *log.Logger get slog.NewLogLogger, so their output keeps
	// the same key=value structure, level and timestamp.
	handler := buildLogHandler(cfg.LogLevel, cfg.LogFormat, os.Stdout)
	slogLogger := slog.New(handler)
	logger := slog.NewLogLogger(handler, slog.LevelInfo)

	db, err := database.Open(cfg.DatabasePath)
	if err != nil {
		logger.Fatalf("database: %v", err)
	}
	defer db.Close()

	if err := database.Migrate(db); err != nil {
		logger.Fatalf("migrate: %v", err)
	}
	if err := database.Seed(db); err != nil {
		logger.Fatalf("seed: %v", err)
	}

	newsRepo := repository.NewNewsRepository(db)
	categoryRepo := repository.NewCategoryRepository(db)
	sourceRepo := repository.NewRSSSourceRepository(db)
	settingsRepo := repository.NewSettingsRepository(db)

	// Production hardening: refuse to boot with insecure defaults so a
	// misconfigured deploy never goes live with admin/admin123 or a dev secret.
	devSecret := "neuralwire-dev-secret-7f3c9a1e4b8d2f6a"
	if cfg.AppEnv == "production" {
		if cfg.AdminUsername == "admin" && cfg.AdminPassword == "admin123" {
			logger.Fatalf("config: refusing to start in production with default admin credentials. Set ADMIN_USERNAME/ADMIN_PASSWORD.")
		}
		if cfg.AdminTokenSecret == "" || cfg.AdminTokenSecret == devSecret {
			logger.Fatalf("config: refusing to start in production with the development ADMIN_TOKEN_SECRET. Set a strong secret.")
		}
	} else if cfg.AdminUsername == "admin" && cfg.AdminPassword == "admin123" {
		logger.Printf("WARNING: using default admin credentials. Set ADMIN_USERNAME/ADMIN_PASSWORD before deployment.")
	}
	authManager := auth.NewManager(cfg.AdminTokenSecret, 0)

	summarizer := ai.NewSummarizer(ai.SummarizerOptions{
		APIKey:  cfg.AISummaryAPIKey,
		Model:   cfg.AISummaryModel,
		BaseURL: cfg.AISummaryBaseURL,
		Timeout: 30 * time.Second,
		Logger:  slogLogger,
	})

	// AI image generation is enabled only when the provider supports it
	// (auto-detected from provider/base URL, overridable via
	// AI_IMAGE_GENERATION_ENABLED). Text-only providers skip straight to the
	// stock cover fallback instead of spamming unsupported image requests.
	imgGenEnabled := true
	if cfg.AIImageGenerationEnabled != nil {
		imgGenEnabled = *cfg.AIImageGenerationEnabled
	} else {
		imgGenEnabled = config.SupportsImageGeneration(cfg.AISummaryProvider, cfg.AISummaryBaseURL)
	}
	if !imgGenEnabled {
		logger.Printf("ai: image generation disabled (provider does not support images); using stock covers")
	}
	imageGenerator := ai.NewImageGenerator(cfg.AISummaryAPIKey, cfg.AISummaryBaseURL, imgGenEnabled, slogLogger)

	// Advisory news-value scoring (AI + heuristic weighted). Admins remain
	// the final decision makers; scoring never auto-publishes.
	scoreService := scoring.NewScoreService(
		summarizer,
		scoring.NewRuleScorer(),
		settingsRepo,
	)

	rssFetcher := fetcher.NewFetcher(fetcher.FetcherOptions{
		Sources:        sourceRepo,
		News:           newsRepo,
		Summarizer:     summarizer,
		ImageGenerator: imageGenerator,
		Scraper: scraper.New(scraper.Options{
			Timeout:   cfg.ScrapeTimeout,
			UserAgent: cfg.UserAgent,
			Logger:    slogLogger,
		}),
		ScrapeMax:          cfg.ScrapeMaxPerSource,
		MinContentChars:    cfg.ScrapeMinContentChars,
		ScrapeDelayMin:     cfg.ScrapeDelayMin,
		ScrapeDelayMax:     cfg.ScrapeDelayMax,
		MaxInsertPerSource: cfg.ScrapeMaxInsertPerSource,
		Scorer:             scoreService,
		UserAgent:          cfg.UserAgent,
		HTTPClient:         &http.Client{Timeout: 30 * time.Second},
		Logger:             slogLogger,
	})

	// Shared metrics collector: wired into the API server (requests, latency,
	// fetch cycles) and the AI package (upstream AI calls).
	appMetrics := metrics.New()
	ai.SetMetrics(appMetrics)

	// Auto fetch/publish scheduler (STY-57): reads its config from
	// app_settings (editable via the admin panel). Disabled by default.
	sched := scheduler.New(settingsRepo, scheduler.Job{
		Fetch: func(ctx context.Context) (fetcher.FetchStats, error) {
			return rssFetcher.FetchAll(ctx)
		},
		AutoPost: func(ctx context.Context, cfg models.AutoPublishConfig) (int, error) {
			// Publish eligible drafts: category whitelist + minimum score label.
			candidates, err := newsRepo.AutoPublishCandidates(cfg.Categories, cfg.MinScoreLabel, 50)
			if err != nil {
				return 0, err
			}
			published := 0
			for _, n := range candidates {
				if err := newsRepo.SetStatus(n.ID, models.StatusPublished); err != nil {
					slogLogger.Error("scheduler: auto publish failed", "id", n.ID, "error", err)
					continue
				}
				published++
				slogLogger.Info("scheduler: auto published",
					"id", n.ID, "title", n.Title, "label", n.ValueLabel)
			}
			return published, nil
		},
	}, slogLogger)
	sched.Start()
	defer sched.Stop()

	srv := api.NewServer(api.ServerOptions{
		NewsRepo:           newsRepo,
		CategoryRepo:       categoryRepo,
		SettingsRepo:       settingsRepo,
		ViewRateLimit:      cfg.ViewRateLimit,
		TrendingCacheTTL:   time.Duration(cfg.TrendingCacheTTLSeconds) * time.Second,
		TrustProxy:         cfg.TrustProxy,
		LoginRateLimit:     cfg.LoginRateLimit,
		GlobalRateLimit:    cfg.GlobalRateLimit,
		DisableCompression: !cfg.HTTPCompressionEnabled,
		Metrics:            appMetrics,
		AllowOrigins:       cfg.CORSAllowOrigins,
		Auth:               authManager,
		AdminUser:          cfg.AdminUsername,
		AdminPass:          cfg.AdminPassword,
		Fetcher:            rssFetcher,
		Logger:             logger,
		Slog:               slogLogger,
		StaticDir:          cfg.StaticDir,
		UploadDir:          cfg.UploadDir,
		Scheduler:          sched,
	})

	httpServer := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           srv.Handler(),
		ReadHeaderTimeout: 10 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		logger.Printf("listening on %s", httpServer.Addr)
		if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	select {
	case err := <-errCh:
		logger.Fatalf("server: %v", err)
	case <-ctx.Done():
	}

	logger.Printf("shutting down...")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		logger.Printf("shutdown: %v", err)
	}
	logger.Printf("server stopped")
}

// buildLogHandler constructs a slog handler from the configured level and
// format. Text is the default for local development; JSON suits production
// log aggregation.
func buildLogHandler(level, format string, w io.Writer) slog.Handler {
	opts := &slog.HandlerOptions{Level: parseLogLevel(level)}
	switch strings.ToLower(strings.TrimSpace(format)) {
	case "json":
		return slog.NewJSONHandler(w, opts)
	default:
		return slog.NewTextHandler(w, opts)
	}
}

// parseLogLevel maps a LOG_LEVEL string to a slog.Level, defaulting to Info.
func parseLogLevel(level string) slog.Level {
	switch strings.ToLower(strings.TrimSpace(level)) {
	case "debug":
		return slog.LevelDebug
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
