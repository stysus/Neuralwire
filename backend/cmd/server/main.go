// Command server runs the Neuralwire REST API backend.
package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"neuralwire/backend/internal/ai"
	"neuralwire/backend/internal/api"
	"neuralwire/backend/internal/auth"
	"neuralwire/backend/internal/config"
	"neuralwire/backend/internal/database"
	"neuralwire/backend/internal/fetcher"
	"neuralwire/backend/internal/repository"
	"neuralwire/backend/internal/scoring"
	"neuralwire/backend/internal/scraper"
)

func main() {
	logger := log.New(os.Stdout, "[neuralwire] ", log.LstdFlags)

	cfg, err := config.Load()
	if err != nil {
		logger.Fatalf("config: %v", err)
	}

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

	if cfg.AdminUsername == "admin" && cfg.AdminPassword == "admin123" {
		logger.Printf("WARNING: using default admin credentials. Set ADMIN_USERNAME/ADMIN_PASSWORD before deployment.")
	}
	authManager := auth.NewManager(cfg.AdminTokenSecret, 0)

	summarizer := ai.NewSummarizer(ai.SummarizerOptions{
		APIKey:  cfg.AISummaryAPIKey,
		Model:   cfg.AISummaryModel,
		BaseURL: cfg.AISummaryBaseURL,
		Timeout: 30 * time.Second,
		Logger:  logger,
	})

	imageGenerator := ai.NewImageGenerator(cfg.AISummaryAPIKey, cfg.AISummaryBaseURL, logger)

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
			UserAgent: "Mozilla/5.0 (compatible; NeuralwireBot/1.0; +https://neuralwire.example)",
			Logger:    logger,
		}),
		ScrapeMax:          cfg.ScrapeMaxPerSource,
		MinContentChars:    cfg.ScrapeMinContentChars,
		ScrapeDelayMin:     cfg.ScrapeDelayMin,
		ScrapeDelayMax:     cfg.ScrapeDelayMax,
		MaxInsertPerSource: cfg.ScrapeMaxInsertPerSource,
		Scorer:             scoreService,
		HTTPClient:         &http.Client{Timeout: 30 * time.Second},
		Logger:             logger,
	})

	srv := api.NewServer(api.ServerOptions{
		NewsRepo:     newsRepo,
		CategoryRepo: categoryRepo,
		SettingsRepo: settingsRepo,
		AllowOrigins: cfg.CORSAllowOrigins,
		Auth:         authManager,
		AdminUser:    cfg.AdminUsername,
		AdminPass:    cfg.AdminPassword,
		Fetcher:      rssFetcher,
		Logger:       logger,
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
