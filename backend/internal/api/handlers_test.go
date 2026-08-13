package api

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

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
		SettingsRepo: repository.NewSettingsRepository(db),
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
	stats    fetcher.FetchStats
	err      error
	calls    int
	progress fetcher.FetchProgress
}

func (f *fakeFeedFetcher) FetchAll(_ context.Context) (fetcher.FetchStats, error) {
	f.calls++
	return f.stats, f.err
}

func (f *fakeFeedFetcher) Progress() fetcher.FetchProgress {
	return f.progress
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

func TestFetchProgressEndpoint(t *testing.T) {
	fake := &fakeFeedFetcher{progress: fetcher.FetchProgress{
		Running:       true,
		TotalSources:  14,
		DoneSources:   7,
		CurrentSource: "OpenAI Blog",
		Percent:       50,
	}}
	s := newTestServer(t)
	s.fetcher = fake
	token := adminToken(t, s)

	rec := doJSONAs(t, s, http.MethodGet, "/api/admin/fetch/progress", nil, token)
	if rec.Code != http.StatusOK {
		t.Fatalf("progress status = %d, want 200 (body: %s)", rec.Code, rec.Body.String())
	}
	var resp fetcher.FetchProgress
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode progress response: %v", err)
	}
	if !resp.Running || resp.Percent != 50 || resp.TotalSources != 14 || resp.DoneSources != 7 {
		t.Errorf("progress = %+v, want running=true percent=50 total=14 done=7", resp)
	}
}

func TestFetchProgressEndpointRequiresAuth(t *testing.T) {
	s := newTestServer(t)
	s.fetcher = &fakeFeedFetcher{}
	if rec := doJSON(t, s, http.MethodGet, "/api/admin/fetch/progress", nil); rec.Code != http.StatusUnauthorized {
		t.Errorf("progress without token = %d, want 401", rec.Code)
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

func TestCancelFetchEndpoint(t *testing.T) {
	s := newTestServer(t)
	token := adminToken(t, s)

	// Cancel with no cycle in progress -> 200, cancelled=false.
	rec := doJSONAs(t, s, http.MethodPost, "/api/admin/fetch/cancel", nil, token)
	if rec.Code != http.StatusOK {
		t.Fatalf("cancel without cycle status = %d, want 200", rec.Code)
	}
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode cancel: %v", err)
	}
	if body["cancelled"] != false {
		t.Errorf("cancelled = %v, want false when no cycle", body["cancelled"])
	}

	// Simulate an in-flight fetch by stashing a cancel func, then cancel.
	_, cancel := context.WithCancel(context.Background())
	defer cancel()
	s.fetchMu.Lock()
	s.fetchCancel = cancel
	s.fetchMu.Unlock()

	rec = doJSONAs(t, s, http.MethodPost, "/api/admin/fetch/cancel", nil, token)
	if rec.Code != http.StatusOK {
		t.Fatalf("cancel active status = %d, want 200", rec.Code)
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode cancel: %v", err)
	}
	if body["cancelled"] != true {
		t.Errorf("cancelled = %v, want true when cycle active", body["cancelled"])
	}

	// Requires auth.
	if rec := doJSON(t, s, http.MethodPost, "/api/admin/fetch/cancel", nil); rec.Code != http.StatusUnauthorized {
		t.Errorf("cancel without token = %d, want 401", rec.Code)
	}
}

func TestSettingsEndpoints(t *testing.T) {
	s := newTestServer(t)
	token := adminToken(t, s)

	// GET defaults
	rec := doJSONAs(t, s, http.MethodGet, "/api/admin/settings", nil, token)
	if rec.Code != http.StatusOK {
		t.Fatalf("get settings status = %d", rec.Code)
	}
	var got models.ScoreThresholds
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode settings: %v", err)
	}
	if got.HighMin != 80 || got.MediumMin != 60 || got.LowMax != 59 {
		t.Errorf("default thresholds = %+v, want 80/60/59", got)
	}

	// PUT new thresholds
	rec = doJSONAs(t, s, http.MethodPut, "/api/admin/settings", models.ScoreThresholds{
		LowMax: 49, MediumMin: 50, MediumMax: 74, HighMin: 75,
	}, token)
	if rec.Code != http.StatusOK {
		t.Fatalf("put settings status = %d", rec.Code)
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode settings: %v", err)
	}
	if got.HighMin != 75 || got.MediumMin != 50 {
		t.Errorf("updated thresholds = %+v, want 75/50", got)
	}

	// Requires auth
	if rec := doJSON(t, s, http.MethodGet, "/api/admin/settings", nil); rec.Code != http.StatusUnauthorized {
		t.Errorf("get settings without token = %d, want 401", rec.Code)
	}
}

// createPublishedArticle creates a draft then publishes it via the admin API.
func createPublishedArticle(t *testing.T, s *Server, token string, title string) models.News {
	t.Helper()
	create := doJSONAs(t, s, http.MethodPost, "/api/admin/news", map[string]any{
		"title":    title,
		"url":      "https://example.com/" + strings.ReplaceAll(title, " ", "-"),
		"source":   "OpenAI Blog",
		"category": "ai",
		"summary":  "A test summary.",
	}, token)
	if create.Code != http.StatusCreated {
		t.Fatalf("create status = %d, want 201 (body: %s)", create.Code, create.Body.String())
	}
	created := decodeNews(t, create)
	pub := doJSONAs(t, s, http.MethodPost, "/api/admin/news/"+strconv.FormatInt(created.ID, 10)+"/publish", nil, token)
	if pub.Code != http.StatusOK {
		t.Fatalf("publish status = %d, want 200", pub.Code)
	}
	return created
}

func TestRecordViewAndTrending(t *testing.T) {
	s := newTestServer(t)
	token := adminToken(t, s)

	a := createPublishedArticle(t, s, token, "Trending Article A")
	b := createPublishedArticle(t, s, token, "Trending Article B")

	// Record several views: A gets 3 distinct viewers, B gets 1.
	for i := 0; i < 3; i++ {
		rec := doJSON(t, s, http.MethodPost,
			"/api/news/"+strconv.FormatInt(a.ID, 10)+"/view",
			map[string]string{"viewer_key": "viewer-" + strconv.Itoa(i)})
		if rec.Code != http.StatusOK {
			t.Fatalf("record view A status = %d, want 200", rec.Code)
		}
	}
	// Same viewer twice -> deduped to one view.
	rec := doJSON(t, s, http.MethodPost, "/api/news/"+strconv.FormatInt(a.ID, 10)+"/view", map[string]string{"viewer_key": "viewer-0"})
	if rec.Code != http.StatusOK {
		t.Fatalf("record dup view status = %d, want 200", rec.Code)
	}
	rec = doJSON(t, s, http.MethodPost, "/api/news/"+strconv.FormatInt(b.ID, 10)+"/view", map[string]string{"viewer_key": "viewer-x"})
	if rec.Code != http.StatusOK {
		t.Fatalf("record view B status = %d, want 200", rec.Code)
	}

	// Trending all time: A (3) should rank above B (1).
	tr := doJSON(t, s, http.MethodGet, "/api/news/trending?window=all&limit=5", nil)
	if tr.Code != http.StatusOK {
		t.Fatalf("trending status = %d, want 200", tr.Code)
	}
	var resp struct {
		Data []models.News `json:"data"`
	}
	if err := json.Unmarshal(tr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode trending: %v", err)
	}
	if len(resp.Data) != 2 {
		t.Fatalf("trending len = %d, want 2", len(resp.Data))
	}
	if resp.Data[0].ID != a.ID {
		t.Errorf("trending[0].ID = %d, want %d (most viewed first)", resp.Data[0].ID, a.ID)
	}
	if resp.Data[0].ViewCount != 3 {
		t.Errorf("trending[0].ViewCount = %d, want 3", resp.Data[0].ViewCount)
	}
	if resp.Data[1].ViewCount != 1 {
		t.Errorf("trending[1].ViewCount = %d, want 1", resp.Data[1].ViewCount)
	}

	// View on a nonexistent id is a silent no-op (200 ok).
	rec = doJSON(t, s, http.MethodPost, "/api/news/99999/view", map[string]string{"viewer_key": "x"})
	if rec.Code != http.StatusOK {
		t.Errorf("record view missing article status = %d, want 200", rec.Code)
	}
}

func TestRelatedNewsEndpoint(t *testing.T) {
	s := newTestServer(t)
	token := adminToken(t, s)

	// Create three articles in the same category plus one in another.
	base := createPublishedArticle(t, s, token, "Gemma 4 12B multimodal model released by Google DeepMind")
	_ = createPublishedArticle(t, s, token, "Gemma 4 multimodal model for laptops from DeepMind")
	_ = createPublishedArticle(t, s, token, "Another gemma multimodal model update")
	otherCat := createPublishedArticle(t, s, token, "Startup raises funding round")

	rec := doJSON(t, s, http.MethodGet, "/api/news/"+strconv.FormatInt(base.ID, 10)+"/related?limit=10", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("related status = %d, want 200 (body: %s)", rec.Code, rec.Body.String())
	}
	var resp struct {
		Data []models.News `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode related: %v", err)
	}
	if len(resp.Data) == 0 {
		t.Fatalf("related empty, want >= 1")
	}
	// The base article must not appear in its own related list.
	for _, n := range resp.Data {
		if n.ID == base.ID {
			t.Errorf("related contains the current article id %d", base.ID)
		}
	}
	// Articles sharing gemma/multimodal keywords must rank before the
	// unrelated one, even though the test helper gives every article the same
	// category and source (so keyword overlap is the only differentiator).
	var otherIdx = -1
	for i, n := range resp.Data {
		if n.ID == otherCat.ID {
			otherIdx = i
		}
	}
	if otherIdx < 0 {
		t.Fatalf("unrelated article missing from related list")
	}
	for i, n := range resp.Data {
		if i > otherIdx && strings.Contains(strings.ToLower(n.Title), "gemma") {
			t.Errorf("gemma article ranked after unrelated one: idx=%d title=%q, otherIdx=%d", i, n.Title, otherIdx)
		}
	}
}

func TestSearchNewsEndpoint(t *testing.T) {
	s := newTestServer(t)
	token := adminToken(t, s)

	_ = createPublishedArticle(t, s, token, "OpenAI releases Gemini killer model")
	_ = createPublishedArticle(t, s, token, "Weather forecasting with cyclones")
	_ = createPublishedArticle(t, s, token, "Google DeepMind robotics breakthrough")

	// Search for "cyclone" should return exactly the weather article.
	rec := doJSON(t, s, http.MethodGet, "/api/news?q=cyclone", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("search status = %d, want 200", rec.Code)
	}
	resp := decodeList(t, rec)
	if resp.Pagination.Total != 1 {
		t.Fatalf("search total = %d, want 1", resp.Pagination.Total)
	}
	if !strings.Contains(strings.ToLower(resp.Data[0].Title), "cyclone") {
		t.Errorf("search result = %q, want cyclone article", resp.Data[0].Title)
	}

	// Search that matches nothing returns empty list, 200.
	rec = doJSON(t, s, http.MethodGet, "/api/news?q=zzzznomatch", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("search no-match status = %d, want 200", rec.Code)
	}
	resp = decodeList(t, rec)
	if resp.Pagination.Total != 0 || len(resp.Data) != 0 {
		t.Errorf("search no-match total = %d len = %d, want 0/0", resp.Pagination.Total, len(resp.Data))
	}

	// Search combined with category filter.
	rec = doJSON(t, s, http.MethodGet, "/api/news?q=model&category=ai", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("search+category status = %d, want 200", rec.Code)
	}
}

func TestRecordViewRateLimit(t *testing.T) {
	db, err := database.Open(":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	if err := database.Migrate(db); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	s := NewServer(ServerOptions{
		NewsRepo:      repository.NewNewsRepository(db),
		CategoryRepo:  repository.NewCategoryRepository(db),
		SettingsRepo:  repository.NewSettingsRepository(db),
		ViewRateLimit: 2, // allow 2 views per IP
		Auth:          auth.NewManager("test-secret", 0),
		AdminUser:     testAdminUser,
		AdminPass:     testAdminPass,
		Logger:        log.New(io.Discard, "", 0),
	})
	token := adminToken(t, s)
	a := createPublishedArticle(t, s, token, "Rate Limited Article")

	// Need the server handler; call through s.Handler() with a fixed IP.
	h := s.Handler()
	do := func() int {
		req := httptest.NewRequest(http.MethodPost, "/api/news/"+strconv.FormatInt(a.ID, 10)+"/view", bytes.NewReader([]byte(`{"viewer_key":"x"}`)))
		req.RemoteAddr = "203.0.113.9:12345"
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		return rec.Code
	}
	if code := do(); code != http.StatusOK {
		t.Fatalf("view 1 = %d, want 200", code)
	}
	if code := do(); code != http.StatusOK {
		t.Fatalf("view 2 = %d, want 200", code)
	}
	if code := do(); code != http.StatusTooManyRequests {
		t.Errorf("view 3 = %d, want 429 (rate limited)", code)
	}
}

func TestTrendingCache(t *testing.T) {
	db, err := database.Open(":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	if err := database.Migrate(db); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	s := NewServer(ServerOptions{
		NewsRepo:         repository.NewNewsRepository(db),
		CategoryRepo:     repository.NewCategoryRepository(db),
		SettingsRepo:     repository.NewSettingsRepository(db),
		TrendingCacheTTL: 5 * time.Minute,
		Auth:             auth.NewManager("test-secret", 0),
		AdminUser:        testAdminUser,
		AdminPass:        testAdminPass,
		Logger:           log.New(io.Discard, "", 0),
	})
	token := adminToken(t, s)
	a := createPublishedArticle(t, s, token, "Trending Cache Article")

	// Record a view so the article appears in trending (JOIN article_views).
	recView := doJSON(t, s, http.MethodPost, "/api/news/"+strconv.FormatInt(a.ID, 10)+"/view", map[string]string{"viewer_key": "cache-test"})
	if recView.Code != http.StatusOK {
		t.Fatalf("record view status = %d", recView.Code)
	}

	// First request: not cached.
	rec := doJSON(t, s, http.MethodGet, "/api/news/trending?window=all&limit=5", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("trending status = %d", rec.Code)
	}
	var first struct {
		Data   []models.News `json:"data"`
		Cached bool          `json:"cached"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &first); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if first.Cached {
		t.Error("first request should not be cached")
	}
	if len(first.Data) == 0 {
		t.Fatal("trending empty")
	}

	// Second request: served from cache.
	rec = doJSON(t, s, http.MethodGet, "/api/news/trending?window=all&limit=5", nil)
	var second struct {
		Data   []models.News `json:"data"`
		Cached bool          `json:"cached"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &second); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !second.Cached {
		t.Error("second request should be cached")
	}
	if len(second.Data) != len(first.Data) {
		t.Errorf("cached data len = %d, want %d", len(second.Data), len(first.Data))
	}
}

func TestSearchMultiWordAnd(t *testing.T) {
	s := newTestServer(t)
	token := adminToken(t, s)

	_ = createPublishedArticle(t, s, token, "Gemma 4 multimodal model for laptops")
	_ = createPublishedArticle(t, s, token, "Gemma vision model update")
	_ = createPublishedArticle(t, s, token, "Weather cyclone forecasting")
	_ = createPublishedArticle(t, s, token, "OpenAI releases another model")

	// Multi-word query "gemma model" must match articles containing BOTH
	// words in title or summary (AND semantics, order-independent).
	rec := doJSON(t, s, http.MethodGet, "/api/news?q=gemma+model&page_size=20", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("search status = %d, want 200", rec.Code)
	}
	resp := decodeList(t, rec)
	if resp.Pagination.Total != 2 {
		t.Fatalf("multi-word total = %d, want 2 (gemma AND model)", resp.Pagination.Total)
	}
	for _, n := range resp.Data {
		title := strings.ToLower(n.Title)
		if !strings.Contains(title, "gemma") || !strings.Contains(title, "model") {
			t.Errorf("result %q does not contain both tokens", n.Title)
		}
	}

	// A query with an absent token returns nothing.
	rec = doJSON(t, s, http.MethodGet, "/api/news?q=gemma+cyclone", nil)
	resp = decodeList(t, rec)
	if resp.Pagination.Total != 0 {
		t.Errorf("absent-token total = %d, want 0", resp.Pagination.Total)
	}
}

func TestClientIPTrustProxy(t *testing.T) {
	s := newTestServer(t) // trustProxy = false default

	req := httptest.NewRequest(http.MethodGet, "/api/news", nil)
	req.RemoteAddr = "203.0.113.9:12345"
	req.Header.Set("X-Forwarded-For", "1.2.3.4")
	if got := s.clientIP(req); got != "203.0.113.9" {
		t.Errorf("clientIP without trustProxy = %q, want RemoteAddr (XFF ignored)", got)
	}

	// With trustProxy enabled, XFF is used.
	s.trustProxy = true
	if got := s.clientIP(req); got != "1.2.3.4" {
		t.Errorf("clientIP with trustProxy = %q, want XFF", got)
	}

	// XFF with multiple hops -> first entry.
	req.Header.Set("X-Forwarded-For", "1.2.3.4, 5.6.7.8")
	if got := s.clientIP(req); got != "1.2.3.4" {
		t.Errorf("clientIP multi-hop = %q, want first XFF entry", got)
	}
}

func TestSecurityHeaders(t *testing.T) {
	s := newTestServer(t)
	rec := doJSON(t, s, http.MethodGet, "/api/health", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("health status = %d", rec.Code)
	}
	h := rec.Header()
	checks := []struct {
		key   string
		value string
	}{
		{"X-Content-Type-Options", "nosniff"},
		{"X-Frame-Options", "DENY"},
		{"Referrer-Policy", "strict-origin-when-cross-origin"},
		{"Permissions-Policy", "camera=(), microphone=(), geolocation=()"},
	}
	for _, c := range checks {
		if got := h.Get(c.key); got != c.value {
			t.Errorf("%s = %q, want %q", c.key, got, c.value)
		}
	}
	if got := h.Get("Content-Security-Policy"); got == "" {
		t.Error("Content-Security-Policy header missing")
	}
}

func TestLoginRateLimit(t *testing.T) {
	db, err := database.Open(":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	if err := database.Migrate(db); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	s := NewServer(ServerOptions{
		NewsRepo:       repository.NewNewsRepository(db),
		CategoryRepo:   repository.NewCategoryRepository(db),
		SettingsRepo:   repository.NewSettingsRepository(db),
		LoginRateLimit: 2, // allow 2 attempts per IP
		Auth:           auth.NewManager("test-secret", 0),
		AdminUser:      testAdminUser,
		AdminPass:      testAdminPass,
		Logger:         log.New(io.Discard, "", 0),
	})

	h := s.Handler()
	doLogin := func() int {
		req := httptest.NewRequest(http.MethodPost, "/api/admin/login",
			bytes.NewReader([]byte(`{"username":"admin","password":"wrong"}`)))
		req.RemoteAddr = "198.51.100.7:12345"
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		return rec.Code
	}
	if code := doLogin(); code != http.StatusUnauthorized {
		t.Fatalf("attempt 1 = %d, want 401", code)
	}
	if code := doLogin(); code != http.StatusUnauthorized {
		t.Fatalf("attempt 2 = %d, want 401", code)
	}
	if code := doLogin(); code != http.StatusTooManyRequests {
		t.Errorf("attempt 3 = %d, want 429 (rate limited)", code)
	}
}

func TestValidCredentials(t *testing.T) {
	s := newTestServer(t) // admin/admin123 dari test helper

	cases := []struct {
		user, pass string
		want       bool
	}{
		{testAdminUser, testAdminPass, true},
		{testAdminUser, "wrong", false},
		{"wrong", testAdminPass, false},
		{"wrong", "wrong", false},
		{"", "", false},
	}
	for _, c := range cases {
		if got := s.validCredentials(c.user, c.pass); got != c.want {
			t.Errorf("validCredentials(%q, %q) = %v, want %v", c.user, c.pass, got, c.want)
		}
	}
}

func TestGlobalRateLimit(t *testing.T) {
	db, err := database.Open(":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	if err := database.Migrate(db); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	s := NewServer(ServerOptions{
		NewsRepo:        repository.NewNewsRepository(db),
		CategoryRepo:    repository.NewCategoryRepository(db),
		SettingsRepo:    repository.NewSettingsRepository(db),
		GlobalRateLimit: 3, // allow 3 requests per IP
		Auth:            auth.NewManager("test-secret", 0),
		AdminUser:       testAdminUser,
		AdminPass:       testAdminPass,
		Logger:          log.New(io.Discard, "", 0),
	})

	h := s.Handler()
	do := func() int {
		req := httptest.NewRequest(http.MethodGet, "/api/news", nil)
		req.RemoteAddr = "203.0.113.55:12345"
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		return rec.Code
	}

	// First 3 requests pass.
	for i := 0; i < 3; i++ {
		if code := do(); code != http.StatusOK {
			t.Fatalf("request %d = %d, want 200", i+1, code)
		}
	}
	// 4th should be 429.
	if code := do(); code != http.StatusTooManyRequests {
		t.Errorf("request 4 = %d, want 429 (global rate limited)", code)
	}
}

func TestCSRFProtect(t *testing.T) {
	s := newTestServer(t) // allowOrigins includes localhost:5173
	token := adminToken(t, s)
	h := s.Handler()

	// Helper: POST to create article with optional Origin.
	doPost := func(origin string) int {
		body := []byte(`{"title":"CSRF ` + origin + `","url":"https://example.com/x","category":"ai"}`)
		req := httptest.NewRequest(http.MethodPost, "/api/admin/news", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+token)
		if origin != "" {
			req.Header.Set("Origin", origin)
		}
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		return rec.Code
	}

	// No Origin (curl-like) -> passes CSRF (created).
	if code := doPost(""); code != http.StatusCreated {
		req := httptest.NewRequest(http.MethodPost, "/api/admin/news",
			bytes.NewReader([]byte(`{"title":"CSRF x","url":"https://example.com/x","category":"ai"}`)))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		t.Logf("no-origin status=%d body=%s", rec.Code, rec.Body.String())
		t.Errorf("no-origin POST = %d, want 201", code)
	}
	// Allowed origin -> passes CSRF.
	if code := doPost("http://localhost:5173"); code == http.StatusForbidden {
		t.Error("allowed-origin POST should not be rejected by CSRF")
	}
	// Hostile origin -> 403.
	if code := doPost("https://evil.com"); code != http.StatusForbidden {
		t.Errorf("hostile-origin POST = %d, want 403", code)
	}
}
