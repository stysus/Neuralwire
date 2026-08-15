package api

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"neuralwire/backend/internal/models"
	"neuralwire/backend/internal/repository"
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

// handleHealth reports liveness and, importantly, DB reachability. It returns
// 200 when the database answers a ping and 503 when it does not, so load
// balancers and orchestrators can take the instance out of rotation. It is
// exempt from rate limiting and never cached.
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()
	if err := s.newsRepo.Ping(ctx); err != nil {
		s.slog.Error("api: health check failed", "error", err)
		s.writeJSON(w, http.StatusServiceUnavailable, map[string]string{"status": "unhealthy"})
		return
	}
	s.writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// handleMetrics renders the runtime counters in Prometheus text exposition
// format for scraping by monitoring agents.
func (s *Server) handleMetrics(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
	s.metrics.WritePrometheus(w)
}

// sitemapOrigin is the public origin used to build absolute sitemap URLs.
// In production it is derived from the request host; in dev it falls back
// to localhost.
func (s *Server) sitemapOrigin(r *http.Request) string {
	if host := strings.TrimSpace(r.Host); host != "" {
		return "https://" + host
	}
	return "https://localhost"
}

// handleSitemap renders an XML sitemap of all public pages: home, about,
// search, every category, and every published article. It is generated from
// the database on demand so it always reflects the current content (the old
// frontend version was prerendered at build time and missed articles).
func (s *Server) handleSitemap(w http.ResponseWriter, r *http.Request) {
	origin := s.sitemapOrigin(r)

	categories, err := s.categoryRepo.List()
	if err != nil {
		s.slog.Error("api: sitemap categories", "error", err)
		s.writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	// Fetch up to 1000 published articles (max page size is 100).
	articles, _, err := s.newsRepo.ListPublished("", "", 1, 100)
	if err != nil {
		s.slog.Error("api: sitemap news", "error", err)
		s.writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	var b strings.Builder
	b.WriteString(`<?xml version="1.0" encoding="UTF-8"?>` + "\n")
	b.WriteString(`<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">` + "\n")

	writeURL := func(loc string, lastmod, changefreq string, priority string) {
		b.WriteString("  <url>\n")
		b.WriteString("    <loc>" + xmlEscape(origin+loc) + "</loc>\n")
		if lastmod != "" {
			b.WriteString("    <lastmod>" + lastmod + "</lastmod>\n")
		}
		b.WriteString("    <changefreq>" + changefreq + "</changefreq>\n")
		b.WriteString("    <priority>" + priority + "</priority>\n")
		b.WriteString("  </url>\n")
	}

	writeURL("/", time.Now().UTC().Format("2006-01-02"), "daily", "1.0")
	writeURL("/about", "", "monthly", "0.5")
	writeURL("/search", "", "weekly", "0.4")

	for _, c := range categories {
		writeURL("/category/"+c.Slug, "", "daily", "0.6")
	}

	for _, a := range articles {
		lastmod := ""
		if a.PublishedAt != nil {
			lastmod = a.PublishedAt.UTC().Format("2006-01-02")
		}
		writeURL("/"+a.Slug, lastmod, "weekly", "0.8")
	}

	b.WriteString(`</urlset>` + "\n")

	w.Header().Set("Content-Type", "application/xml; charset=utf-8")
	w.Header().Set("Cache-Control", "public, max-age=3600")
	_, _ = w.Write([]byte(b.String()))
}

// xmlEscape escapes the five XML special characters in a string.
func xmlEscape(s string) string {
	replacer := strings.NewReplacer(
		"&", "&amp;",
		"<", "&lt;",
		">", "&gt;",
		`"`, "&quot;",
		"'", "&apos;",
	)
	return replacer.Replace(s)
}

// handleRobotsTXT serves robots.txt with the admin area disallowed and the
// sitemap location derived from the request host so it is always correct.
func (s *Server) handleRobotsTXT(w http.ResponseWriter, r *http.Request) {
	origin := s.sitemapOrigin(r)
	body := "# allow crawling everywhere except the admin panel\n" +
		"User-agent: *\n" +
		"Disallow: /admin\n\n" +
		"Sitemap: " + origin + "/sitemap.xml\n"
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Cache-Control", "public, max-age=3600")
	_, _ = w.Write([]byte(body))
}

func (s *Server) handleListNews(w http.ResponseWriter, r *http.Request) {
	category := strings.TrimSpace(r.URL.Query().Get("category"))
	query := strings.TrimSpace(r.URL.Query().Get("q"))

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

	news, total, err := s.newsRepo.ListPublished(category, query, page, pageSize)
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

// handleTrendingNews returns the most-viewed published articles for the
// requested window (?window=day|week|all, default week) and limit
// (?limit=, default 5). Public endpoint. Results are cached briefly (TTL)
// because the underlying query aggregates the full view log.
func (s *Server) handleTrendingNews(w http.ResponseWriter, r *http.Request) {
	window := repository.TrendingWindow(strings.TrimSpace(r.URL.Query().Get("window")))
	switch window {
	case repository.TrendingDay, repository.TrendingWeek, repository.TrendingAll:
	default:
		window = repository.TrendingWeek
	}

	limit := 5
	if v := strings.TrimSpace(r.URL.Query().Get("limit")); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			limit = n
		}
	}

	cacheKey := "trending:" + string(window) + ":" + strconv.Itoa(limit)
	if s.trendingCache != nil {
		if v, ok := s.trendingCache.Get(cacheKey); ok {
			if news, isNews := v.([]models.News); isNews {
				s.writeJSON(w, http.StatusOK, map[string]any{
					"window": window,
					"data":   news,
					"cached": true,
				})
				return
			}
		}
	}

	news, err := s.newsRepo.ListTrending(window, limit)
	if err != nil {
		s.logger.Printf("api: trending news: %v", err)
		s.writeError(w, http.StatusInternalServerError, "failed to load trending news")
		return
	}
	if news == nil {
		news = []models.News{}
	}
	if s.trendingCache != nil {
		s.trendingCache.Set(cacheKey, news)
	}
	s.writeJSON(w, http.StatusOK, map[string]any{
		"window": window,
		"data":   news,
		"cached": false,
	})
}

// handleRecordView counts one read of a published article. It accepts an
// optional JSON body {viewer_key} for per-visitor deduplication. The
// response is always 200 {ok:true} so the frontend tracking call is a
// fire-and-forget; recording errors are logged, not surfaced.
func (s *Server) handleRecordView(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		s.writeError(w, http.StatusBadRequest, "invalid news id")
		return
	}

	// Per-IP anti-abuse: the same visitor can only record a limited number of
	// views per window. When limited, return 429 with Retry-After.
	if s.viewLimiter != nil {
		ok, retryAfter := s.viewLimiter.Allow(s.clientIP(r))
		if !ok {
			w.Header().Set("Retry-After", strconv.Itoa(int(retryAfter.Seconds())))
			s.writeError(w, http.StatusTooManyRequests, "rate limit exceeded for view tracking")
			return
		}
	}

	var body struct {
		ViewerKey string `json:"viewer_key"`
	}
	if r.Body != nil {
		_ = json.NewDecoder(io.LimitReader(r.Body, 4096)).Decode(&body)
	}
	body.ViewerKey = strings.TrimSpace(body.ViewerKey)
	if len(body.ViewerKey) > 128 {
		body.ViewerKey = body.ViewerKey[:128]
	}

	if err := s.newsRepo.RecordView(id, body.ViewerKey); err != nil {
		s.logger.Printf("api: record view for %d: %v", id, err)
	}
	s.writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// handleRelatedNews returns published articles most similar to the given one,
// ranked by relevance (category + source + keyword overlap). Public endpoint.
func (s *Server) handleRelatedNews(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		s.writeError(w, http.StatusBadRequest, "invalid news id")
		return
	}

	limit := 12
	if v := strings.TrimSpace(r.URL.Query().Get("limit")); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			limit = n
		}
	}

	news, err := s.newsRepo.ListRelated(id, limit)
	if err != nil {
		s.logger.Printf("api: related news %d: %v", id, err)
		s.writeError(w, http.StatusInternalServerError, "failed to load related news")
		return
	}
	if news == nil {
		news = []models.News{}
	}
	s.writeJSON(w, http.StatusOK, map[string]any{"data": news})
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
// token. It is the only public route under /api/admin/. Per-IP rate limiting
// slows brute-force attempts.
func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	// Rate limit by IP before doing any work, so brute-force attempts are
	// throttled even when they never send a valid body.
	if s.loginLimiter != nil {
		ok, retryAfter := s.loginLimiter.Allow(s.clientIP(r))
		if !ok {
			w.Header().Set("Retry-After", strconv.Itoa(int(retryAfter.Seconds())))
			s.writeError(w, http.StatusTooManyRequests, "too many login attempts, try again later")
			return
		}
	}

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

	category := strings.TrimSpace(r.URL.Query().Get("category"))
	valueLabel := strings.ToUpper(strings.TrimSpace(r.URL.Query().Get("value_label")))
	if valueLabel != "" && valueLabel != "HIGH" && valueLabel != "MEDIUM" && valueLabel != "LOW" {
		valueLabel = ""
	}

	news, total, err := s.newsRepo.ListAdmin(string(status), category, valueLabel, page, pageSize)
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

// handleFetchNews manually triggers one RSS fetch cycle and returns the
// aggregate statistics. It is protected by requireAuth.
func (s *Server) handleFetchNews(w http.ResponseWriter, r *http.Request) {
	if s.fetcher == nil {
		s.writeError(w, http.StatusServiceUnavailable, "fetcher is not configured")
		return
	}

	// Allow the full cycle to finish; individual scrapes are bounded by their
	// own timeouts, and the request context is overridden so a client
	// disconnect does not abort the fetch midway. The cancel function is kept
	// so POST /api/admin/fetch/cancel can abort the running cycle.
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()

	s.fetchMu.Lock()
	s.fetchCancel = cancel
	s.fetchMu.Unlock()
	defer func() {
		s.fetchMu.Lock()
		s.fetchCancel = nil
		s.fetchMu.Unlock()
	}()

	stats, err := s.fetcher.FetchAll(ctx)
	if err != nil {
		s.metrics.FetchCycle(true)
		if errors.Is(err, context.Canceled) {
			s.logger.Printf("api: fetch cycle cancelled by admin")
			s.writeError(w, http.StatusConflict, "fetch cycle cancelled")
			return
		}
		s.logger.Printf("api: fetch cycle finished with errors: %v", err)
		// Still report partial results; a per-source error is not a total
		// failure of the manual trigger.
	} else {
		s.metrics.FetchCycle(false)
	}
	s.writeJSON(w, http.StatusOK, stats)
}

// handleCancelFetch aborts the currently running manual fetch cycle, if any.
func (s *Server) handleCancelFetch(w http.ResponseWriter, r *http.Request) {
	s.fetchMu.Lock()
	cancel := s.fetchCancel
	s.fetchMu.Unlock()

	if cancel == nil {
		s.writeJSON(w, http.StatusOK, map[string]any{"cancelled": false, "message": "no fetch cycle in progress"})
		return
	}
	cancel()
	s.logger.Printf("api: fetch cancel requested")
	s.writeJSON(w, http.StatusOK, map[string]any{"cancelled": true, "message": "fetch cancellation requested"})
}

// handleFetchProgress reports a snapshot of the in-flight fetch cycle so the
// admin UI can render a live percentage. It is protected by requireAuth.
func (s *Server) handleFetchProgress(w http.ResponseWriter, r *http.Request) {
	if s.fetcher == nil {
		s.writeError(w, http.StatusServiceUnavailable, "fetcher is not configured")
		return
	}
	s.writeJSON(w, http.StatusOK, s.fetcher.Progress())
}

// handleGetSettings returns the admin-configurable scoring thresholds.
func (s *Server) handleGetSettings(w http.ResponseWriter, r *http.Request) {
	if s.settingsRepo == nil {
		s.writeError(w, http.StatusServiceUnavailable, "settings repository is not configured")
		return
	}
	s.writeJSON(w, http.StatusOK, s.settingsRepo.GetScoreThresholds())
}

// handleUpdateSettings persists the scoring thresholds. Values are clamped
// by the repository; a missing key falls back to its default.
func (s *Server) handleUpdateSettings(w http.ResponseWriter, r *http.Request) {
	if s.settingsRepo == nil {
		s.writeError(w, http.StatusServiceUnavailable, "settings repository is not configured")
		return
	}
	var req models.ScoreThresholds
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&req); err != nil {
		s.writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if err := s.settingsRepo.SetScoreThresholds(req); err != nil {
		s.logger.Printf("api: update settings: %v", err)
		s.writeError(w, http.StatusInternalServerError, "failed to update settings")
		return
	}
	s.writeJSON(w, http.StatusOK, s.settingsRepo.GetScoreThresholds())
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

type updateNewsRequest struct {
	Title    string `json:"title"`
	Category string `json:"category"`
	Summary  string `json:"summary"`
	Content  string `json:"content"`
	ImageURL string `json:"image_url"`
}

func (s *Server) handleUpdateNews(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		s.writeError(w, http.StatusBadRequest, "invalid news id")
		return
	}

	var req updateNewsRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&req); err != nil {
		s.writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	req.Title = strings.TrimSpace(req.Title)
	if req.Title == "" {
		s.writeError(w, http.StatusBadRequest, "title is required")
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
		Category: req.Category,
		Summary:  req.Summary,
		Content:  req.Content,
		ImageURL: req.ImageURL,
	}

	if err := s.newsRepo.Update(id, article); err != nil {
		s.logger.Printf("api: update news %d: %v", id, err)
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

func (s *Server) handleDeleteNewsByStatus(w http.ResponseWriter, r *http.Request) {
	status := models.NewsStatus(strings.TrimSpace(r.URL.Query().Get("status")))
	if status == "" || !status.Valid() {
		s.writeError(w, http.StatusBadRequest, "invalid or missing status parameter")
		return
	}

	if err := s.newsRepo.DeleteByStatus(string(status)); err != nil {
		s.logger.Printf("api: delete news by status %s: %v", status, err)
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
