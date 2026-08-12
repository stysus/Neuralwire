// Package api exposes the Neuralwire REST API.
package api

import (
	"encoding/json"
	"log"
	"net/http"

	"neuralwire/backend/internal/repository"
)

// Server wires repositories and middleware into an http.Handler.
type Server struct {
	newsRepo     *repository.NewsRepository
	categoryRepo *repository.CategoryRepository
	allowOrigins []string
	logger       *log.Logger
}

// ServerOptions configures the API server.
type ServerOptions struct {
	NewsRepo     *repository.NewsRepository
	CategoryRepo *repository.CategoryRepository
	AllowOrigins []string
	Logger       *log.Logger
}

// NewServer builds a Server.
func NewServer(opts ServerOptions) *Server {
	if len(opts.AllowOrigins) == 0 {
		opts.AllowOrigins = []string{"http://localhost:5173"}
	}
	if opts.Logger == nil {
		opts.Logger = log.Default()
	}
	return &Server{
		newsRepo:     opts.NewsRepo,
		categoryRepo: opts.CategoryRepo,
		allowOrigins: opts.AllowOrigins,
		logger:       opts.Logger,
	}
}

// Handler assembles the route table and middleware stack.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /api/health", s.handleHealth)
	mux.HandleFunc("GET /api/news", s.handleListNews)
	mux.HandleFunc("GET /api/news/{id}", s.handleGetNews)
	mux.HandleFunc("GET /api/categories", s.handleListCategories)

	mux.HandleFunc("POST /api/admin/news", s.handleCreateNews)
	mux.HandleFunc("POST /api/admin/news/{id}/publish", s.handlePublishNews)
	mux.HandleFunc("POST /api/admin/news/{id}/reject", s.handleRejectNews)
	mux.HandleFunc("DELETE /api/admin/news/{id}", s.handleDeleteNews)

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
