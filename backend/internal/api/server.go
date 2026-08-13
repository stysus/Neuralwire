// Package api exposes the Neuralwire REST API.
package api

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"sync"

	"neuralwire/backend/internal/auth"
	"neuralwire/backend/internal/fetcher"
	"neuralwire/backend/internal/repository"
)

// FeedFetcher runs one RSS fetch cycle and reports its progress. It is
// satisfied by *fetcher.Fetcher and by test doubles.
type FeedFetcher interface {
	FetchAll(ctx context.Context) (fetcher.FetchStats, error)
	Progress() fetcher.FetchProgress
}

// Server wires repositories and middleware into an http.Handler.
type Server struct {
	newsRepo     *repository.NewsRepository
	categoryRepo *repository.CategoryRepository
	settingsRepo *repository.SettingsRepository
	allowOrigins []string
	auth         *auth.Manager
	adminUser    string
	adminPass    string
	fetcher      FeedFetcher
	logger       *log.Logger

	// fetchMu guards the in-flight manual fetch cancel function so the admin
	// cancel endpoint can abort a running cycle without data races.
	fetchMu     sync.Mutex
	fetchCancel context.CancelFunc
}

// ServerOptions configures the API server.
type ServerOptions struct {
	NewsRepo     *repository.NewsRepository
	CategoryRepo *repository.CategoryRepository
	SettingsRepo *repository.SettingsRepository
	AllowOrigins []string
	Auth         *auth.Manager
	AdminUser    string
	AdminPass    string
	// Fetcher drives the manual POST /api/admin/fetch endpoint. When nil the
	// endpoint responds 503.
	Fetcher FeedFetcher
	Logger  *log.Logger
}

// NewServer builds a Server.
func NewServer(opts ServerOptions) *Server {
	if len(opts.AllowOrigins) == 0 {
		opts.AllowOrigins = []string{"http://localhost:5173"}
	}
	if opts.Auth == nil {
		opts.Auth = auth.NewManager("", 0)
	}
	if opts.Logger == nil {
		opts.Logger = log.Default()
	}
	return &Server{
		newsRepo:     opts.NewsRepo,
		categoryRepo: opts.CategoryRepo,
		settingsRepo: opts.SettingsRepo,
		allowOrigins: opts.AllowOrigins,
		auth:         opts.Auth,
		adminUser:    opts.AdminUser,
		adminPass:    opts.AdminPass,
		fetcher:      opts.Fetcher,
		logger:       opts.Logger,
	}
}

// Handler assembles the route table and middleware stack. All routes under
// /api/admin/ require a bearer token except POST /api/admin/login.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /api/health", s.handleHealth)
	mux.HandleFunc("GET /api/news", s.handleListNews)
	mux.HandleFunc("GET /api/news/{id}", s.handleGetNews)
	mux.HandleFunc("GET /api/news/trending", s.handleTrendingNews)
	mux.HandleFunc("GET /api/news/{id}/related", s.handleRelatedNews)
	mux.HandleFunc("POST /api/news/{id}/view", s.handleRecordView)
	mux.HandleFunc("GET /api/categories", s.handleListCategories)

	// Admin login is public; every other /api/admin/* route is protected.
	mux.HandleFunc("POST /api/admin/login", s.handleLogin)

	admin := http.NewServeMux()
	admin.HandleFunc("GET /api/admin/news", s.handleAdminListNews)
	admin.HandleFunc("GET /api/admin/news/{id}", s.handleAdminGetNews)
	admin.HandleFunc("POST /api/admin/news", s.handleCreateNews)
	admin.HandleFunc("PUT /api/admin/news/{id}", s.handleUpdateNews)
	admin.HandleFunc("POST /api/admin/news/{id}/publish", s.handlePublishNews)
	admin.HandleFunc("POST /api/admin/news/{id}/reject", s.handleRejectNews)
	admin.HandleFunc("DELETE /api/admin/news/{id}", s.handleDeleteNews)
	admin.HandleFunc("DELETE /api/admin/news", s.handleDeleteNewsByStatus)
	mux.Handle("POST /api/admin/fetch", s.requireAuth(http.HandlerFunc(s.handleFetchNews)))
	mux.Handle("POST /api/admin/fetch/cancel", s.requireAuth(http.HandlerFunc(s.handleCancelFetch)))
	mux.Handle("GET /api/admin/fetch/progress", s.requireAuth(http.HandlerFunc(s.handleFetchProgress)))
	mux.Handle("GET /api/admin/settings", s.requireAuth(http.HandlerFunc(s.handleGetSettings)))
	mux.Handle("PUT /api/admin/settings", s.requireAuth(http.HandlerFunc(s.handleUpdateSettings)))
	mux.Handle("/api/admin/", s.requireAuth(admin))

	return s.recover(s.log(s.cors(mux)))
}

// --- response helpers -----------------------------------------------------

func (s *Server) writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		s.logger.Printf("api: encode response: %v", err)
	}
}

func (s *Server) writeError(w http.ResponseWriter, status int, message string) {
	s.writeJSON(w, status, map[string]string{"error": message})
}
