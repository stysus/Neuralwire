// Package api exposes the Neuralwire REST API.
package api

import (
	"encoding/json"
	"log"
	"net/http"

	"neuralwire/backend/internal/auth"
	"neuralwire/backend/internal/repository"
)

// Server wires repositories and middleware into an http.Handler.
type Server struct {
	newsRepo     *repository.NewsRepository
	categoryRepo *repository.CategoryRepository
	allowOrigins []string
	auth         *auth.Manager
	adminUser    string
	adminPass    string
	logger       *log.Logger
}

// ServerOptions configures the API server.
type ServerOptions struct {
	NewsRepo     *repository.NewsRepository
	CategoryRepo *repository.CategoryRepository
	AllowOrigins []string
	Auth         *auth.Manager
	AdminUser    string
	AdminPass    string
	Logger       *log.Logger
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
		allowOrigins: opts.AllowOrigins,
		auth:         opts.Auth,
		adminUser:    opts.AdminUser,
		adminPass:    opts.AdminPass,
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
	mux.HandleFunc("GET /api/categories", s.handleListCategories)

	// Admin login is public; every other /api/admin/* route is protected.
	mux.HandleFunc("POST /api/admin/login", s.handleLogin)

	admin := http.NewServeMux()
	admin.HandleFunc("GET /api/admin/news", s.handleAdminListNews)
	admin.HandleFunc("GET /api/admin/news/{id}", s.handleAdminGetNews)
	admin.HandleFunc("POST /api/admin/news", s.handleCreateNews)
	admin.HandleFunc("POST /api/admin/news/{id}/publish", s.handlePublishNews)
	admin.HandleFunc("POST /api/admin/news/{id}/reject", s.handleRejectNews)
	admin.HandleFunc("DELETE /api/admin/news/{id}", s.handleDeleteNews)
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
