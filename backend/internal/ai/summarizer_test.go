package ai

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func noopLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func TestSummarizeWithoutAPIKeyFallsBack(t *testing.T) {
	s := NewSummarizer(SummarizerOptions{
		FallbackChars: 300,
		Logger:        noopLogger(),
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

func TestParseValueScore(t *testing.T) {
	got, ok := parseValueScore(`{"score": 85, "impact": 90, "novelty": 80, "quality": 75, "confidence": 0.85, "recommendation": "publish", "reason": "Major model release."}`)
	if !ok {
		t.Fatal("expected parse success")
	}
	if got.Score != 85 || got.Impact != 90 || got.Novelty != 80 || got.Quality != 75 {
		t.Errorf("subscores = %+v, want 85/90/80/75", got)
	}
	if got.Confidence != 0.85 || got.Recommendation != "publish" {
		t.Errorf("confidence/recommendation = %v/%q", got.Confidence, got.Recommendation)
	}
	if got.Breakdown == "" {
		t.Error("breakdown should be a JSON string")
	}
}

func TestParseValueScoreToleratesFences(t *testing.T) {
	got, ok := parseValueScore("```json\n{\"score\": 42, \"impact\": 40, \"novelty\": 40, \"quality\": 45, \"confidence\": 0.5, \"recommendation\": \"review\", \"reason\": \"ok\"}\n```")
	if !ok {
		t.Fatal("expected parse success with code fences")
	}
	if got.Score != 42 {
		t.Errorf("Score = %d, want 42", got.Score)
	}
}

func TestParseValueScoreClampsAndDefaults(t *testing.T) {
	got, ok := parseValueScore(`{"score": 250, "impact": -5, "novelty": 100, "quality": 50, "confidence": 5, "recommendation": "whatever", "reason": "x"}`)
	if !ok {
		t.Fatal("expected parse success")
	}
	if got.Score != 100 {
		t.Errorf("Score = %d, want clamped 100", got.Score)
	}
	if got.Impact != 0 {
		t.Errorf("Impact = %d, want clamped 0", got.Impact)
	}
	if got.Confidence != 1 {
		t.Errorf("Confidence = %v, want clamped 1", got.Confidence)
	}
	if got.Recommendation != "review" {
		t.Errorf("Recommendation = %q, want default review", got.Recommendation)
	}
}

func TestParseValueScoreInvalid(t *testing.T) {
	if _, ok := parseValueScore("no json here"); ok {
		t.Error("expected failure for non-JSON input")
	}
	if _, ok := parseValueScore(""); ok {
		t.Error("expected failure for empty input")
	}
}

func TestChatCompletionRetriesOnEmptyBody(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		var req chatRequest
		_ = json.NewDecoder(r.Body).Decode(&req)
		w.Header().Set("Content-Type", "application/json")
		if calls == 1 {
			// First attempt: empty content (reasoning model exhausted budget).
			w.Write([]byte(`{"choices":[{"message":{"content":""}}]}`))
			return
		}
		// Second attempt: doubled max_tokens should be present.
		if req.MaxTokens != 1000 {
			t.Errorf("retry max_tokens = %d, want 1000 (doubled)", req.MaxTokens)
		}
		w.Write([]byte(`{"choices":[{"message":{"content":"recovered answer"}}]}`))
	}))
	defer server.Close()

	s := NewSummarizer(SummarizerOptions{
		APIKey:  "test-key",
		Model:   "gpt-4o-mini",
		BaseURL: server.URL,
		Logger:  noopLogger(),
		Timeout: 5 * time.Second,
	})

	text, ok := chatCompletion(context.Background(), s.(*openAISummarizer).client, noopLogger(),
		server.URL+"/chat/completions", "test-key", "gpt-4o-mini", "sys", "user", 500)
	if !ok {
		t.Fatal("expected retry to succeed")
	}
	if text != "recovered answer" {
		t.Errorf("text = %q, want recovered answer", text)
	}
	if calls != 2 {
		t.Errorf("upstream calls = %d, want 2 (original + retry)", calls)
	}
}

func TestChatCompletionNoRetryOnRealFailure(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		http.Error(w, "boom", http.StatusInternalServerError)
	}))
	defer server.Close()

	_, ok := chatCompletion(context.Background(), &http.Client{Timeout: 5 * time.Second}, noopLogger(),
		server.URL+"/chat/completions", "test-key", "gpt-4o-mini", "sys", "user", 500)
	if ok {
		t.Error("expected failure on 500")
	}
	if calls != 1 {
		t.Errorf("upstream calls = %d, want 1 (no retry on HTTP error)", calls)
	}
}
