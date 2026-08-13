package ai

import (
	"context"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestImageGeneratorDisabledSkipsGeneration(t *testing.T) {
	// Even with an API key + base URL, disabled generator must never call
	// the upstream image endpoint and must return a stock Unsplash fallback.
	called := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	g := NewImageGenerator("test-key", srv.URL, false, log.New(io.Discard, "", 0))
	got := g.Generate(context.Background(), "Some Article Title", "ai")
	if called {
		t.Error("disabled image generator should not call upstream")
	}
	if !strings.Contains(got, "images.unsplash.com/featured") {
		t.Errorf("expected stock Unsplash fallback, got %q", got)
	}
}

func TestImageGeneratorUnsupportedSkipsFutureCalls(t *testing.T) {
	// First call hits 404 (unsupported) -> marks generator unsupported;
	// second call must skip upstream entirely.
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte(`{"error":"not found"}`))
	}))
	defer srv.Close()

	g := NewImageGenerator("test-key", srv.URL, true, log.New(io.Discard, "", 0))
	g.Generate(context.Background(), "Title One", "ai")
	g.Generate(context.Background(), "Title Two", "ai")
	if calls != 1 {
		t.Errorf("upstream image calls = %d, want 1 (second call skipped)", calls)
	}
}

func TestImageGeneratorSuccessReturnsURL(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"data":[{"url":"https://img.example.com/cover.png"}]}`))
	}))
	defer srv.Close()

	g := NewImageGenerator("test-key", srv.URL, true, log.New(io.Discard, "", 0))
	got := g.Generate(context.Background(), "Title", "ai")
	if got != "https://img.example.com/cover.png" {
		t.Errorf("Generate = %q, want generated image URL", got)
	}
}
