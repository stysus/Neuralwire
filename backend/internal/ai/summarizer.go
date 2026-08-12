// Package ai integrates with an OpenAI-compatible summarization API.
package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"regexp"
	"strings"
	"time"
)

// Summarizer produces a short summary for a news article.
type Summarizer interface {
	// Summarize returns a summary for the given title and content. It never
	// fails: when no API key is configured or the upstream call errors, it
	// falls back to a plain truncation of the content.
	Summarize(ctx context.Context, title, content string) string
}

// SummarizerOptions configures the OpenAI-compatible client.
type SummarizerOptions struct {
	APIKey        string
	Model         string
	BaseURL       string
	Timeout       time.Duration
	MaxInputChars int
	FallbackChars int
	Logger        *log.Logger
}

type openAISummarizer struct {
	apiKey        string
	model         string
	endpoint      string
	client        *http.Client
	maxInputChars int
	fallbackChars int
	logger        *log.Logger
}

// NewSummarizer builds a Summarizer. With an empty APIKey it always uses
// the truncation fallback.
func NewSummarizer(opts SummarizerOptions) Summarizer {
	if opts.MaxInputChars <= 0 {
		opts.MaxInputChars = 8000
	}
	if opts.FallbackChars <= 0 {
		opts.FallbackChars = 300
	}
	if opts.Timeout <= 0 {
		opts.Timeout = 30 * time.Second
	}
	if opts.Logger == nil {
		opts.Logger = log.Default()
	}
	return &openAISummarizer{
		apiKey:        opts.APIKey,
		model:         opts.Model,
		endpoint:      strings.TrimRight(opts.BaseURL, "/") + "/chat/completions",
		client:        &http.Client{Timeout: opts.Timeout},
		maxInputChars: opts.MaxInputChars,
		fallbackChars: opts.FallbackChars,
		logger:        opts.Logger,
	}
}

type chatRequest struct {
	Model     string        `json:"model"`
	Messages  []chatMessage `json:"messages"`
	MaxTokens int           `json:"max_tokens"`
}

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
}

func (s *openAISummarizer) Summarize(ctx context.Context, title, content string) string {
	if s.apiKey == "" {
		return fallback(content, s.fallbackChars)
	}

	prompt := fmt.Sprintf(
		"Summarize the following AI news article in 2-3 concise sentences for a general audience. "+
			"Write in English. Do not use markdown or bullet points.\n\nTitle: %s\n\nContent: %s",
		title, truncate(stripHTML(content), s.maxInputChars),
	)

	reqBody, err := json.Marshal(chatRequest{
		Model: s.model,
		Messages: []chatMessage{
			{Role: "system", Content: "You are a professional news editor writing concise summaries."},
			{Role: "user", Content: prompt},
		},
		MaxTokens: 300,
	})
	if err != nil {
		s.logger.Printf("ai: marshal request: %v", err)
		return fallback(content, s.fallbackChars)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.endpoint, bytes.NewReader(reqBody))
	if err != nil {
		s.logger.Printf("ai: build request: %v", err)
		return fallback(content, s.fallbackChars)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+s.apiKey)

	resp, err := s.client.Do(req)
	if err != nil {
		s.logger.Printf("ai: request failed: %v", err)
		return fallback(content, s.fallbackChars)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		s.logger.Printf("ai: upstream returned %s: %s", resp.Status, string(body))
		return fallback(content, s.fallbackChars)
	}

	var parsed chatResponse
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&parsed); err != nil {
		s.logger.Printf("ai: decode response: %v", err)
		return fallback(content, s.fallbackChars)
	}
	if len(parsed.Choices) == 0 || strings.TrimSpace(parsed.Choices[0].Message.Content) == "" {
		s.logger.Printf("ai: empty response from upstream")
		return fallback(content, s.fallbackChars)
	}
	return strings.TrimSpace(parsed.Choices[0].Message.Content)
}

// fallback returns the first n characters of the plain-text content.
func fallback(content string, n int) string {
	text := strings.Join(strings.Fields(stripHTML(content)), " ")
	return truncate(text, n)
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return strings.TrimSpace(s[:n]) + "..."
}

var tagRegex = regexp.MustCompile(`<[^>]*>`)

func stripHTML(s string) string {
	return tagRegex.ReplaceAllString(s, " ")
}
