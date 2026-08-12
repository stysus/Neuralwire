package api

import (
	"crypto/subtle"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"neuralwire/backend/internal/models"
)

const (
	defaultPageSize = 20
	maxPageSize     = 100
)

// paginatedResponse is the standard envelope for list endpoints.
type paginatedResponse struct {
	Data       []models.News `json:"data"`
	Pagination pagination    `json:"pagination"`
}

type pagination struct {
	Page       int `json:"page"`
	PageSize   int `json:"page_size"`
	Total      int `json:"total"`
	TotalPages int `json:"total_pages"`
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	s.writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleListNews(w http.ResponseWriter, r *http.Request) {
	category := strings.TrimSpace(r.URL.Query().Get("category"))

	page, err := parsePositiveInt(r.URL.Query().Get("page"), 1)
	if err != nil {
		s.writeError(w, http.StatusBadRequest, "invalid page parameter")
		return
	}
	pageSize, err := parsePositiveInt(r.URL.Query().Get("page_size"), defaultPageSize)
	if err != nil {
		s.writeError(w, http.StatusBadRequest, "invalid page_size parameter")
		return
	}
	if pageSize > maxPageSize {
		pageSize = maxPageSize
	}

	news, total, err := s.newsRepo.ListPublished(category, page, pageSize)
	if err != nil {
		s.logger.Printf("api: list news: %v", err)
		s.writeError(w, http.StatusInternalServerError, "failed to list news")
		return
	}

	totalPages := 0
	if total > 0 {
		totalPages = (total + pageSize - 1) / pageSize
	}
	if news == nil {
		news = []models.News{}
	}

	s.writeJSON(w, http.StatusOK, paginatedResponse{
		Data: news,
		Pagination: pagination{
			Page:       page,
			PageSize:   pageSize,
			Total:      total,
			TotalPages: totalPages,
		},
	})
}

func (s *Server) handleGetNews(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		s.writeError(w, http.StatusBadRequest, "invalid news id")
		return
	}

	news, err := s.newsRepo.GetByID(id)
	if err != nil {
		s.logger.Printf("api: get news %d: %v", id, err)
		s.writeError(w, http.StatusInternalServerError, "failed to load news")
		return
	}
	if news == nil || news.Status != models.StatusPublished {
		s.writeError(w, http.StatusNotFound, "news not found")
		return
	}
	s.writeJSON(w, http.StatusOK, news)
}

func (s *Server) handleListCategories(w http.ResponseWriter, r *http.Request) {
	categories, err := s.categoryRepo.List()
	if err != nil {
		s.logger.Printf("api: list categories: %v", err)
		s.writeError(w, http.StatusInternalServerError, "failed to list categories")
		return
	}
	if categories == nil {
		categories = []models.Category{}
	}
	s.writeJSON(w, http.StatusOK, map[string]any{"data": categories})
}

// --- admin handlers -------------------------------------------------------

type loginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type loginResponse struct {
	Token     string `json:"token"`
	TokenType string `json:"token_type"`
	ExpiresIn int64  `json:"expires_in"`
}

// handleLogin authenticates admin credentials and returns a signed bearer
// token. It is the only public route under /api/admin/.
func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&req); err != nil {
		s.writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	if !s.validCredentials(req.Username, req.Password) {
		s.writeError(w, http.StatusUnauthorized, "invalid credentials")
		return
	}

	token, err := s.auth.IssueToken(req.Username)
	if err != nil {
		s.logger.Printf("api: issue token: %v", err)
		s.writeError(w, http.StatusInternalServerError, "failed to issue token")
		return
	}

	s.writeJSON(w, http.StatusOK, loginResponse{
		Token:     token,
		TokenType: "Bearer",
		ExpiresIn: int64(s.auth.TokenTTL().Seconds()),
	})
}

// validCredentials compares credentials in constant time so timing does not
// leak whether the username or the password was wrong.
func (s *Server) validCredentials(username, password string) bool {
	userOK := subtle.ConstantTimeCompare([]byte(username), []byte(s.adminUser)) == 1
	passOK := subtle.ConstantTimeCompare([]byte(password), []byte(s.adminPass)) == 1
	return userOK && passOK
}

// handleAdminListNews returns a paginated list of every article, optionally
// filtered by status. Unlike the public endpoint, drafts and rejected
// articles are included.
func (s *Server) handleAdminListNews(w http.ResponseWriter, r *http.Request) {
	status := models.NewsStatus(strings.TrimSpace(r.URL.Query().Get("status")))
	if status != "" && !status.Valid() {
		s.writeError(w, http.StatusBadRequest, "invalid status parameter (want draft, published or rejected)")
		return
	}

	page, err := parsePositiveInt(r.URL.Query().Get("page"), 1)
	if err != nil {
		s.writeError(w, http.StatusBadRequest, "invalid page parameter")
		return
	}
	pageSize, err := parsePositiveInt(r.URL.Query().Get("page_size"), defaultPageSize)
	if err != nil {
		s.writeError(w, http.StatusBadRequest, "invalid page_size parameter")
		return
	}
	if pageSize > maxPageSize {
		pageSize = maxPageSize
	}

	news, total, err := s.newsRepo.ListAdmin(string(status), page, pageSize)
	if err != nil {
		s.logger.Printf("api: admin list news: %v", err)
		s.writeError(w, http.StatusInternalServerError, "failed to list news")
		return
	}

	totalPages := 0
	if total > 0 {
		totalPages = (total + pageSize - 1) / pageSize
	}
	if news == nil {
		news = []models.News{}
	}

	s.writeJSON(w, http.StatusOK, paginatedResponse{
		Data: news,
		Pagination: pagination{
			Page:       page,
			PageSize:   pageSize,
			Total:      total,
			TotalPages: totalPages,
		},
	})
}

// handleAdminGetNews returns the full article (including content) for any
// status.
func (s *Server) handleAdminGetNews(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		s.writeError(w, http.StatusBadRequest, "invalid news id")
		return
	}

	news, err := s.newsRepo.GetByID(id)
	if err != nil {
		s.logger.Printf("api: admin get news %d: %v", id, err)
		s.writeError(w, http.StatusInternalServerError, "failed to load news")
		return
	}
	if news == nil {
		s.writeError(w, http.StatusNotFound, "news not found")
		return
	}
	s.writeJSON(w, http.StatusOK, news)
}

type createNewsRequest struct {
	Title    string `json:"title"`
	URL      string `json:"url"`
	Source   string `json:"source"`
	Category string `json:"category"`
	Summary  string `json:"summary"`
	Content  string `json:"content"`
	ImageURL string `json:"image_url"`
}

func (s *Server) handleCreateNews(w http.ResponseWriter, r *http.Request) {
	var req createNewsRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&req); err != nil {
		s.writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	req.Title = strings.TrimSpace(req.Title)
	req.URL = strings.TrimSpace(req.URL)
	if req.Title == "" {
		s.writeError(w, http.StatusBadRequest, "title is required")
		return
	}
	if req.URL == "" {
		s.writeError(w, http.StatusBadRequest, "url is required")
		return
	}
	if req.Category == "" {
		req.Category = "ai"
	}
	if _, err := s.categoryRepo.EnsureCreated(req.Category); err != nil {
		s.logger.Printf("api: ensure category %q: %v", req.Category, err)
	}

	article := models.News{
		Title:    req.Title,
		URL:      req.URL,
		Source:   req.Source,
		Category: req.Category,
		Summary:  req.Summary,
		Content:  req.Content,
		ImageURL: req.ImageURL,
		Status:   models.StatusDraft,
	}

	id, err := s.newsRepo.Create(article)
	if err != nil {
		s.logger.Printf("api: create news: %v", err)
		s.writeError(w, http.StatusInternalServerError, "failed to create news")
		return
	}

	created, err := s.newsRepo.GetByID(id)
	if err != nil || created == nil {
		s.logger.Printf("api: load created news %d: %v", id, err)
		s.writeError(w, http.StatusInternalServerError, "failed to load created news")
		return
	}
	s.writeJSON(w, http.StatusCreated, created)
}

func (s *Server) handlePublishNews(w http.ResponseWriter, r *http.Request) {
	s.handleStatusTransition(w, r, models.StatusPublished)
}

func (s *Server) handleRejectNews(w http.ResponseWriter, r *http.Request) {
	s.handleStatusTransition(w, r, models.StatusRejected)
}

func (s *Server) handleStatusTransition(w http.ResponseWriter, r *http.Request, status models.NewsStatus) {
	id, err := pathID(r)
	if err != nil {
		s.writeError(w, http.StatusBadRequest, "invalid news id")
		return
	}

	news, err := s.newsRepo.GetByID(id)
	if err != nil {
		s.logger.Printf("api: get news %d: %v", id, err)
		s.writeError(w, http.StatusInternalServerError, "failed to load news")
		return
	}
	if news == nil {
		s.writeError(w, http.StatusNotFound, "news not found")
		return
	}
	if news.Status == status {
		// Idempotent transition: already in the requested state.
		s.writeJSON(w, http.StatusOK, news)
		return
	}
	if err := s.newsRepo.SetStatus(id, status); err != nil {
		s.logger.Printf("api: set news %d status to %s: %v", id, status, err)
		s.writeError(w, http.StatusInternalServerError, "failed to update news")
		return
	}

	updated, err := s.newsRepo.GetByID(id)
	if err != nil || updated == nil {
		s.logger.Printf("api: load updated news %d: %v", id, err)
		s.writeError(w, http.StatusInternalServerError, "failed to load updated news")
		return
	}
	s.writeJSON(w, http.StatusOK, updated)
}

func (s *Server) handleDeleteNews(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		s.writeError(w, http.StatusBadRequest, "invalid news id")
		return
	}

	news, err := s.newsRepo.GetByID(id)
	if err != nil {
		s.logger.Printf("api: get news %d: %v", id, err)
		s.writeError(w, http.StatusInternalServerError, "failed to load news")
		return
	}
	if news == nil {
		s.writeError(w, http.StatusNotFound, "news not found")
		return
	}

	if err := s.newsRepo.Delete(id); err != nil {
		s.logger.Printf("api: delete news %d: %v", id, err)
		s.writeError(w, http.StatusInternalServerError, "failed to delete news")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// --- helpers --------------------------------------------------------------

func pathID(r *http.Request) (int64, error) {
	return strconv.ParseInt(r.PathValue("id"), 10, 64)
}

func parsePositiveInt(raw string, fallback int) (int, error) {
	if strings.TrimSpace(raw) == "" {
		return fallback, nil
	}
	n, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil || n < 1 {
		return 0, errors.New("invalid integer")
	}
	return n, nil
}
