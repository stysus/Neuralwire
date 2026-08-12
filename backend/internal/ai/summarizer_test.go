package ai

import (
	"context"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func noopLogger() *log.Logger {
	return log.New(io.Discard, "", 0)
}

func TestSummarizeWithoutAPIKeyFallsBack(t *testing.T) {
	s := NewSummarizer(SummarizerOptions{
		Logger: noopLogger(),
	})
	content := "<p>First sentence. " + strings.Repeat("word ", 100) + "</p>"

	got := s.Summarize(context.Background(), "Title", content)
	if got == "" {
		t.Fatal("expected non-empty fallback summary")
	}
	if strings.ContainsAny(got, "<>") {
		t.Errorf("fallback summary contains HTML tags: %q", got)
	}
	if len(got) > 400 {
		t.Errorf("fallback summary too long: %d chars", len(got))
	}
}

func TestSummarizeUsesUpstream(t *testing.T) {
	var capturedBody string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer test-key" {
			t.Errorf("Authorization = %q, want Bearer test-key", got)
		}
		body, _ := io.ReadAll(r.Body)
		capturedBody = string(body)
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"choices":[{"message":{"content":"A crisp summary."}}]}`))
	}))
	defer server.Close()

	s := NewSummarizer(SummarizerOptions{
		APIKey:  "test-key",
		Model:   "gpt-4o-mini",
		BaseURL: server.URL,
		Logger:  noopLogger(),
		Timeout: 5 * time.Second,
	})

	got := s.Summarize(context.Background(), "Great Title", "<p>Body text</p>")
	if got != "A crisp summary." {
		t.Errorf("Summarize = %q, want %q", got, "A crisp summary.")
	}
	if !strings.Contains(capturedBody, "gpt-4o-mini") {
		t.Errorf("request body missing model: %s", capturedBody)
	}
	if !strings.Contains(capturedBody, "Great Title") {
		t.Errorf("request body missing title: %s", capturedBody)
	}
}

func TestSummarizeFallsBackOnUpstreamError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "boom", http.StatusInternalServerError)
	}))
	defer server.Close()

	s := NewSummarizer(SummarizerOptions{
		APIKey:  "test-key",
		Model:   "gpt-4o-mini",
		BaseURL: server.URL,
		Logger:  noopLogger(),
		Timeout: 5 * time.Second,
	})

	got := s.Summarize(context.Background(), "Title", "<p>"+strings.Repeat("word ", 100)+"</p>")
	if got == "" {
		t.Fatal("expected fallback summary on upstream error")
	}
}
