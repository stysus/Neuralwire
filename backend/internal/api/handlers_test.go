package api

import (
	"bytes"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"testing"

	"neuralwire/backend/internal/database"
	"neuralwire/backend/internal/models"
	"neuralwire/backend/internal/repository"
)

// newTestServer builds a Server backed by an in-memory SQLite database.
func newTestServer(t *testing.T) *Server {
	t.Helper()
	db, err := database.Open(":memory:")
	if err != nil {
		t.Fatalf("open in-memory db: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	if err := database.Migrate(db); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	if err := database.Seed(db); err != nil {
		t.Fatalf("seed: %v", err)
	}

	return NewServer(ServerOptions{
		NewsRepo:     repository.NewNewsRepository(db),
		CategoryRepo: repository.NewCategoryRepository(db),
		AllowOrigins: []string{"http://localhost:5173", "http://127.0.0.1:5173"},
		Logger:       log.New(io.Discard, "", 0),
	})
}

func doJSON(t *testing.T, s *Server, method, path string, body any) *httptest.ResponseRecorder {
	t.Helper()
	var reader io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal body: %v", err)
		}
		reader = bytes.NewReader(raw)
	}
	req := httptest.NewRequest(method, path, reader)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)
	return rec
}

func decodeNews(t *testing.T, rec *httptest.ResponseRecorder) models.News {
	t.Helper()
	var n models.News
	if err := json.Unmarshal(rec.Body.Bytes(), &n); err != nil {
		t.Fatalf("decode news: %v (body: %s)", err, rec.Body.String())
	}
	return n
}

func TestHealth(t *testing.T) {
	s := newTestServer(t)
	rec := doJSON(t, s, http.MethodGet, "/api/health", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("health status = %d, want 200", rec.Code)
	}
}

func TestNewsLifecycle(t *testing.T) {
	s := newTestServer(t)

	// Create draft.
	create := doJSON(t, s, http.MethodPost, "/api/admin/news", map[string]any{
		"title":    "OpenAI announces new model",
		"url":      "https://openai.com/blog/new-model",
		"source":   "OpenAI Blog",
		"category": "ai",
		"summary":  "A test summary.",
	})
	if create.Code != http.StatusCreated {
		t.Fatalf("create status = %d, want 201 (body: %s)", create.Code, create.Body.String())
	}
	created := decodeNews(t, create)
	if created.Status != models.StatusDraft {
		t.Errorf("created status = %s, want draft", created.Status)
	}
	if created.Slug != "openai-announces-new-model" {
		t.Errorf("created slug = %q, want openai-announces-new-model", created.Slug)
	}

	// Draft must not be visible on public endpoints.
	list := doJSON(t, s, http.MethodGet, "/api/news", nil)
	if list.Code != http.StatusOK {
		t.Fatalf("list status = %d, want 200", list.Code)
	}
	var listed paginatedResponse
	if err := json.Unmarshal(list.Body.Bytes(), &listed); err != nil {
		t.Fatalf("decode list: %v", err)
	}
	if listed.Pagination.Total != 0 {
		t.Errorf("public list total = %d, want 0 for drafts", listed.Pagination.Total)
	}

	// Public detail of a draft returns 404.
	detail := doJSON(t, s, http.MethodGet, "/api/news/"+itoa(created.ID), nil)
	if detail.Code != http.StatusNotFound {
		t.Errorf("draft detail status = %d, want 404", detail.Code)
	}

	// Publish.
	pub := doJSON(t, s, http.MethodPost, "/api/admin/news/"+itoa(created.ID)+"/publish", nil)
	if pub.Code != http.StatusOK {
		t.Fatalf("publish status = %d, want 200 (body: %s)", pub.Code, pub.Body.String())
	}
	published := decodeNews(t, pub)
	if published.Status != models.StatusPublished {
		t.Errorf("published status = %s, want published", published.Status)
	}
	if published.PublishedAt == nil {
		t.Errorf("published_at should be set after publish")
	}

	// Now visible.
	list = doJSON(t, s, http.MethodGet, "/api/news", nil)
	if err := json.Unmarshal(list.Body.Bytes(), &listed); err != nil {
		t.Fatalf("decode list: %v", err)
	}
	if listed.Pagination.Total != 1 {
		t.Errorf("public list total = %d, want 1", listed.Pagination.Total)
	}

	// Filter by category.
	byCat := doJSON(t, s, http.MethodGet, "/api/news?category=research", nil)
	if err := json.Unmarshal(byCat.Body.Bytes(), &listed); err != nil {
		t.Fatalf("decode list: %v", err)
	}
	if listed.Pagination.Total != 0 {
		t.Errorf("research filtered total = %d, want 0", listed.Pagination.Total)
	}

	// Reject.
	rej := doJSON(t, s, http.MethodPost, "/api/admin/news/"+itoa(created.ID)+"/reject", nil)
	if rej.Code != http.StatusOK {
		t.Fatalf("reject status = %d, want 200", rej.Code)
	}
	if got := decodeNews(t, rej).Status; got != models.StatusRejected {
		t.Errorf("rejected status = %s, want rejected", got)
	}

	// Rejected article disappears from public list.
	list = doJSON(t, s, http.MethodGet, "/api/news", nil)
	if err := json.Unmarshal(list.Body.Bytes(), &listed); err != nil {
		t.Fatalf("decode list: %v", err)
	}
	if listed.Pagination.Total != 0 {
		t.Errorf("public list total after reject = %d, want 0", listed.Pagination.Total)
	}

	// Delete.
	del := doJSON(t, s, http.MethodDelete, "/api/admin/news/"+itoa(created.ID), nil)
	if del.Code != http.StatusNoContent {
		t.Fatalf("delete status = %d, want 204", del.Code)
	}

	// Deleting again returns 404.
	del = doJSON(t, s, http.MethodDelete, "/api/admin/news/"+itoa(created.ID), nil)
	if del.Code != http.StatusNotFound {
		t.Errorf("second delete status = %d, want 404", del.Code)
	}
}

func TestListValidation(t *testing.T) {
	s := newTestServer(t)
	if rec := doJSON(t, s, http.MethodGet, "/api/news?page=abc", nil); rec.Code != http.StatusBadRequest {
		t.Errorf("page=abc status = %d, want 400", rec.Code)
	}
	if rec := doJSON(t, s, http.MethodGet, "/api/news?page=0", nil); rec.Code != http.StatusBadRequest {
		t.Errorf("page=0 status = %d, want 400", rec.Code)
	}
	if rec := doJSON(t, s, http.MethodGet, "/api/news?page_size=1000", nil); rec.Code != http.StatusOK {
		t.Errorf("page_size=1000 status = %d, want 200 (clamped)", rec.Code)
	}
}

func TestCreateValidation(t *testing.T) {
	s := newTestServer(t)
	if rec := doJSON(t, s, http.MethodPost, "/api/admin/news", map[string]any{"title": ""}); rec.Code != http.StatusBadRequest {
		t.Errorf("missing title status = %d, want 400", rec.Code)
	}
	if rec := doJSON(t, s, http.MethodPost, "/api/admin/news", map[string]any{"title": "T", "url": ""}); rec.Code != http.StatusBadRequest {
		t.Errorf("missing url status = %d, want 400", rec.Code)
	}
	if rec := doJSON(t, s, http.MethodPost, "/api/admin/news", "not json"); rec.Code != http.StatusBadRequest {
		t.Errorf("bad json status = %d, want 400", rec.Code)
	}
}

func TestCategories(t *testing.T) {
	s := newTestServer(t)
	rec := doJSON(t, s, http.MethodGet, "/api/categories", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("categories status = %d, want 200", rec.Code)
	}
	var resp struct {
		Data []models.Category `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode categories: %v", err)
	}
	if len(resp.Data) == 0 {
		t.Error("expected seeded categories")
	}
}

func TestCORSPreflight(t *testing.T) {
	s := newTestServer(t)
	req := httptest.NewRequest(http.MethodOptions, "/api/news", nil)
	req.Header.Set("Origin", "http://localhost:5173")
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Errorf("preflight status = %d, want 204", rec.Code)
	}
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "http://localhost:5173" {
		t.Errorf("allow-origin = %q, want http://localhost:5173", got)
	}
}

func itoa(n int64) string {
	if n == 0 {
		return "0"
	}
	var buf [20]byte
	pos := len(buf)
	for n > 0 {
		pos--
		buf[pos] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[pos:])
}
