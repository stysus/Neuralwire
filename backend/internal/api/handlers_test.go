package api

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"testing"

	"neuralwire/backend/internal/auth"
	"neuralwire/backend/internal/database"
	"neuralwire/backend/internal/fetcher"
	"neuralwire/backend/internal/models"
	"neuralwire/backend/internal/repository"
)

const (
	testAdminUser = "admin"
	testAdminPass = "admin123"
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
		Auth:         auth.NewManager("test-secret", 0),
		AdminUser:    testAdminUser,
		AdminPass:    testAdminPass,
		Logger:       log.New(io.Discard, "", 0),
	})
}

func doJSON(t *testing.T, s *Server, method, path string, body any) *httptest.ResponseRecorder {
	return doJSONAs(t, s, method, path, body, "")
}

// doJSONAs sends a request with an optional Authorization bearer token.
func doJSONAs(t *testing.T, s *Server, method, path string, body any, token string) *httptest.ResponseRecorder {
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
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)
	return rec
}

// adminToken logs in and returns a valid bearer token for the test server.
func adminToken(t *testing.T, s *Server) string {
	t.Helper()
	rec := doJSON(t, s, http.MethodPost, "/api/admin/login", map[string]string{
		"username": testAdminUser,
		"password": testAdminPass,
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("login status = %d, want 200 (body: %s)", rec.Code, rec.Body.String())
	}
	var resp loginResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode login: %v", err)
	}
	if resp.Token == "" {
		t.Fatal("login response missing token")
	}
	return resp.Token
}

func decodeNews(t *testing.T, rec *httptest.ResponseRecorder) models.News {
	t.Helper()
	var n models.News
	if err := json.Unmarshal(rec.Body.Bytes(), &n); err != nil {
		t.Fatalf("decode news: %v (body: %s)", err, rec.Body.String())
	}
	return n
}

func decodeList(t *testing.T, rec *httptest.ResponseRecorder) paginatedResponse {
	t.Helper()
	var resp paginatedResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode list: %v (body: %s)", err, rec.Body.String())
	}
	return resp
}

func TestHealth(t *testing.T) {
	s := newTestServer(t)
	rec := doJSON(t, s, http.MethodGet, "/api/health", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("health status = %d, want 200", rec.Code)
	}
}

func TestLogin(t *testing.T) {
	s := newTestServer(t)

	// Correct credentials return a token.
	token := adminToken(t, s)
	if token == "" {
		t.Fatal("expected non-empty token")
	}

	// Wrong password is rejected.
	bad := doJSON(t, s, http.MethodPost, "/api/admin/login", map[string]string{
		"username": testAdminUser,
		"password": "wrong",
	})
	if bad.Code != http.StatusUnauthorized {
		t.Errorf("bad password status = %d, want 401", bad.Code)
	}

	// Unknown username is rejected.
	unknown := doJSON(t, s, http.MethodPost, "/api/admin/login", map[string]string{
		"username": "nobody",
		"password": testAdminPass,
	})
	if unknown.Code != http.StatusUnauthorized {
		t.Errorf("unknown user status = %d, want 401", unknown.Code)
	}

	// Malformed body is rejected.
	malformed := doJSON(t, s, http.MethodPost, "/api/admin/login", "not json")
	if malformed.Code != http.StatusBadRequest {
		t.Errorf("malformed login status = %d, want 400", malformed.Code)
	}
}

func TestAdminRequiresAuth(t *testing.T) {
	s := newTestServer(t)

	// No token -> 401.
	if rec := doJSON(t, s, http.MethodGet, "/api/admin/news", nil); rec.Code != http.StatusUnauthorized {
		t.Errorf("admin list without token = %d, want 401", rec.Code)
	}

	// Garbage token -> 403.
	if rec := doJSONAs(t, s, http.MethodGet, "/api/admin/news", nil, "garbage"); rec.Code != http.StatusForbidden {
		t.Errorf("admin list with bad token = %d, want 403", rec.Code)
	}

	// Token signed with a different secret -> 403.
	forged := auth.NewManager("other-secret", 0)
	token, err := forged.IssueToken("admin")
	if err != nil {
		t.Fatalf("issue forged token: %v", err)
	}
	if rec := doJSONAs(t, s, http.MethodGet, "/api/admin/news", nil, token); rec.Code != http.StatusForbidden {
		t.Errorf("admin list with forged token = %d, want 403", rec.Code)
	}

	// The public list endpoint stays open.
	if rec := doJSON(t, s, http.MethodGet, "/api/news", nil); rec.Code != http.StatusOK {
		t.Errorf("public list status = %d, want 200", rec.Code)
	}
}

func TestNewsLifecycle(t *testing.T) {
	s := newTestServer(t)
	token := adminToken(t, s)

	// Create draft.
	create := doJSONAs(t, s, http.MethodPost, "/api/admin/news", map[string]any{
		"title":    "OpenAI announces new model",
		"url":      "https://openai.com/blog/new-model",
		"source":   "OpenAI Blog",
		"category": "ai",
		"summary":  "A test summary.",
	}, token)
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
	if got := decodeList(t, list).Pagination.Total; got != 0 {
		t.Errorf("public list total = %d, want 0 for drafts", got)
	}

	// Public detail of a draft returns 404.
	detail := doJSON(t, s, http.MethodGet, "/api/news/"+itoa(created.ID), nil)
	if detail.Code != http.StatusNotFound {
		t.Errorf("draft detail status = %d, want 404", detail.Code)
	}

	// Admin detail returns the draft with full content.
	adminDetail := doJSONAs(t, s, http.MethodGet, "/api/admin/news/"+itoa(created.ID), nil, token)
	if adminDetail.Code != http.StatusOK {
		t.Errorf("admin detail status = %d, want 200", adminDetail.Code)
	}

	// Publish.
	pub := doJSONAs(t, s, http.MethodPost, "/api/admin/news/"+itoa(created.ID)+"/publish", nil, token)
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
	if got := decodeList(t, list).Pagination.Total; got != 1 {
		t.Errorf("public list total = %d, want 1", got)
	}

	// Filter by category.
	byCat := doJSON(t, s, http.MethodGet, "/api/news?category=research", nil)
	if got := decodeList(t, byCat).Pagination.Total; got != 0 {
		t.Errorf("research filtered total = %d, want 0", got)
	}

	// Reject.
	rej := doJSONAs(t, s, http.MethodPost, "/api/admin/news/"+itoa(created.ID)+"/reject", nil, token)
	if rej.Code != http.StatusOK {
		t.Fatalf("reject status = %d, want 200", rej.Code)
	}
	if got := decodeNews(t, rej).Status; got != models.StatusRejected {
		t.Errorf("rejected status = %s, want rejected", got)
	}

	// Rejected article disappears from public list.
	list = doJSON(t, s, http.MethodGet, "/api/news", nil)
	if got := decodeList(t, list).Pagination.Total; got != 0 {
		t.Errorf("public list total after reject = %d, want 0", got)
	}

	// Delete.
	del := doJSONAs(t, s, http.MethodDelete, "/api/admin/news/"+itoa(created.ID), nil, token)
	if del.Code != http.StatusNoContent {
		t.Fatalf("delete status = %d, want 204", del.Code)
	}

	// Deleting again returns 404.
	del = doJSONAs(t, s, http.MethodDelete, "/api/admin/news/"+itoa(created.ID), nil, token)
	if del.Code != http.StatusNotFound {
		t.Errorf("second delete status = %d, want 404", del.Code)
	}
}

func TestAdminListIncludesAllStatuses(t *testing.T) {
	s := newTestServer(t)
	token := adminToken(t, s)

	// Three articles: one per status.
	ids := make([]int64, 3)
	for i, title := range []string{"Draft story", "Published story", "Rejected story"} {
		rec := doJSONAs(t, s, http.MethodPost, "/api/admin/news", map[string]any{
			"title": title,
			"url":   "https://example.com/" + itoa(int64(i)),
		}, token)
		if rec.Code != http.StatusCreated {
			t.Fatalf("create %d status = %d", i, rec.Code)
		}
		ids[i] = decodeNews(t, rec).ID
	}
	if err := s.newsRepo.SetStatus(ids[1], models.StatusPublished); err != nil {
		t.Fatalf("publish: %v", err)
	}
	if err := s.newsRepo.SetStatus(ids[2], models.StatusRejected); err != nil {
		t.Fatalf("reject: %v", err)
	}

	// Public list only shows the published article.
	pubList := doJSON(t, s, http.MethodGet, "/api/news", nil)
	if got := decodeList(t, pubList).Pagination.Total; got != 1 {
		t.Errorf("public list total = %d, want 1", got)
	}

	// Admin list shows all three.
	adminList := doJSONAs(t, s, http.MethodGet, "/api/admin/news", nil, token)
	if adminList.Code != http.StatusOK {
		t.Fatalf("admin list status = %d, want 200", adminList.Code)
	}
	resp := decodeList(t, adminList)
	if resp.Pagination.Total != 3 {
		t.Errorf("admin list total = %d, want 3", resp.Pagination.Total)
	}

	// Status filter.
	for _, status := range []models.NewsStatus{models.StatusDraft, models.StatusPublished, models.StatusRejected} {
		filtered := doJSONAs(t, s, http.MethodGet, "/api/admin/news?status="+string(status), nil, token)
		got := decodeList(t, filtered).Pagination.Total
		if got != 1 {
			t.Errorf("admin list status=%s total = %d, want 1", status, got)
		}
		for _, n := range decodeList(t, filtered).Data {
			if n.Status != status {
				t.Errorf("admin list status=%s returned status=%s", status, n.Status)
			}
		}
	}

	// Invalid status filter.
	invalid := doJSONAs(t, s, http.MethodGet, "/api/admin/news?status=bogus", nil, token)
	if invalid.Code != http.StatusBadRequest {
		t.Errorf("invalid status = %d, want 400", invalid.Code)
	}

	// Admin list pagination.
	paged := doJSONAs(t, s, http.MethodGet, "/api/admin/news?page=1&page_size=2", nil, token)
	resp = decodeList(t, paged)
	if len(resp.Data) != 2 || resp.Pagination.TotalPages != 2 {
		t.Errorf("admin pagination got len=%d pages=%d, want len=2 pages=2", len(resp.Data), resp.Pagination.TotalPages)
	}
}

func TestAdminDetailAnyStatus(t *testing.T) {
	s := newTestServer(t)
	token := adminToken(t, s)

	rec := doJSONAs(t, s, http.MethodPost, "/api/admin/news", map[string]any{
		"title":   "Full article",
		"url":     "https://example.com/full",
		"content": "The complete body content.",
	}, token)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create status = %d, want 201", rec.Code)
	}
	created := decodeNews(t, rec)

	// Draft detail includes content.
	detail := doJSONAs(t, s, http.MethodGet, "/api/admin/news/"+itoa(created.ID), nil, token)
	if detail.Code != http.StatusOK {
		t.Fatalf("admin detail status = %d, want 200", detail.Code)
	}
	if got := decodeNews(t, detail).Content; got != "The complete body content." {
		t.Errorf("admin detail content = %q, want full content", got)
	}

	// Admin detail of a missing article -> 404.
	missing := doJSONAs(t, s, http.MethodGet, "/api/admin/news/99999", nil, token)
	if missing.Code != http.StatusNotFound {
		t.Errorf("admin detail missing = %d, want 404", missing.Code)
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
	token := adminToken(t, s)

	if rec := doJSONAs(t, s, http.MethodPost, "/api/admin/news", map[string]any{"title": ""}, token); rec.Code != http.StatusBadRequest {
		t.Errorf("missing title status = %d, want 400", rec.Code)
	}
	if rec := doJSONAs(t, s, http.MethodPost, "/api/admin/news", map[string]any{"title": "T", "url": ""}, token); rec.Code != http.StatusBadRequest {
		t.Errorf("missing url status = %d, want 400", rec.Code)
	}
	if rec := doJSONAs(t, s, http.MethodPost, "/api/admin/news", "not json", token); rec.Code != http.StatusBadRequest {
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

// fakeFeedFetcher is a test double for the manual fetch endpoint.
type fakeFeedFetcher struct {
	stats fetcher.FetchStats
	err   error
	calls int
}

func (f *fakeFeedFetcher) FetchAll(_ context.Context) (fetcher.FetchStats, error) {
	f.calls++
	return f.stats, f.err
}

func TestFetchEndpoint(t *testing.T) {
	fake := &fakeFeedFetcher{stats: fetcher.FetchStats{
		TotalNew:          3,
		Scraped:           2,
		Fallback:          1,
		SkippedLowQuality: 10,
		Sources: []fetcher.SourceStats{
			{Name: "OpenAI Blog", Inserted: 2, Scraped: 2},
			{Name: "Reddit r/artificial", Inserted: 1, Fallback: 1},
		},
	}}
	s := newTestServer(t)
	s.fetcher = fake
	token := adminToken(t, s)

	rec := doJSONAs(t, s, http.MethodPost, "/api/admin/fetch", nil, token)
	if rec.Code != http.StatusOK {
		t.Fatalf("fetch status = %d, want 200 (body: %s)", rec.Code, rec.Body.String())
	}
	if fake.calls != 1 {
		t.Errorf("FetchAll calls = %d, want 1", fake.calls)
	}
	var resp fetcher.FetchStats
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode fetch response: %v", err)
	}
	if resp.TotalNew != 3 || resp.Scraped != 2 || resp.Fallback != 1 || resp.SkippedLowQuality != 10 {
		t.Errorf("fetch stats = %+v, want total_new=3 scraped=2 fallback=1 skipped=10", resp)
	}
	if len(resp.Sources) != 2 || resp.Sources[0].Name != "OpenAI Blog" {
		t.Errorf("fetch sources = %+v, want 2 sources starting with OpenAI Blog", resp.Sources)
	}
}

func TestFetchEndpointRequiresAuth(t *testing.T) {
	s := newTestServer(t)
	s.fetcher = &fakeFeedFetcher{}
	// No token -> 401.
	if rec := doJSON(t, s, http.MethodPost, "/api/admin/fetch", nil); rec.Code != http.StatusUnauthorized {
		t.Errorf("fetch without token = %d, want 401", rec.Code)
	}
}

func TestFetchEndpointWithoutFetcher(t *testing.T) {
	s := newTestServer(t) // no fetcher configured
	token := adminToken(t, s)

	rec := doJSONAs(t, s, http.MethodPost, "/api/admin/fetch", nil, token)
	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("fetch without fetcher = %d, want 503", rec.Code)
	}
}

func TestAdminListStableOrdering(t *testing.T) {
	s := newTestServer(t)
	token := adminToken(t, s)

	// Create three drafts in quick succession. created_at has second
	// precision, so id DESC must break the tie deterministically.
	var ids []int64
	for i := 0; i < 3; i++ {
		rec := doJSONAs(t, s, http.MethodPost, "/api/admin/news", map[string]any{
			"title": "Stable order story " + itoa(int64(i)),
			"url":   "https://example.com/stable/" + itoa(int64(i)),
		}, token)
		if rec.Code != http.StatusCreated {
			t.Fatalf("create %d status = %d", i, rec.Code)
		}
		ids = append(ids, decodeNews(t, rec).ID)
	}

	rec := doJSONAs(t, s, http.MethodGet, "/api/admin/news", nil, token)
	if rec.Code != http.StatusOK {
		t.Fatalf("admin list status = %d", rec.Code)
	}
	resp := decodeList(t, rec)
	if len(resp.Data) != 3 {
		t.Fatalf("admin list len = %d, want 3", len(resp.Data))
	}
	// Newest first: ids must be descending.
	for i := 1; i < len(resp.Data); i++ {
		if resp.Data[i].ID > resp.Data[i-1].ID {
			t.Errorf("admin list not stable: id[%d]=%d > id[%d]=%d", i, resp.Data[i].ID, i-1, resp.Data[i-1].ID)
		}
	}
	if resp.Data[0].ID != ids[2] {
		t.Errorf("newest first id = %d, want %d", resp.Data[0].ID, ids[2])
	}
}
