// Package ai integrates with an OpenAI-compatible summarization API.
package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"html"
	"log/slog"
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

	// Categorize classifies an article into one of the valid categories
	// using AI. Falls back to defaultCategory on error or missing API key.
	Categorize(ctx context.Context, title, content, defaultCategory string) string

	// ScoreValue rates an article's news value on a 0-100 scale with a
	// breakdown, confidence, an advisory recommendation, and a reason.
	// It returns ok=false when AI is unavailable (no key, upstream error),
	// so callers can fall back to heuristic scoring.
	ScoreValue(ctx context.Context, title, content, source string) (ValueScore, bool)
}

// ValueScore is the AI's rating of an article's news value. It is advisory:
// the backend computes the final weighted score and admins decide publishing.
type ValueScore struct {
	Score          int     `json:"score"`          // 0-100 AI opinion
	Impact         int     `json:"impact"`         // 0-100 sub-score
	Novelty        int     `json:"novelty"`        // 0-100 sub-score
	Quality        int     `json:"quality"`        // 0-100 sub-score
	Confidence     float64 `json:"confidence"`     // 0-1
	Recommendation string  `json:"recommendation"` // publish / consider / review
	Reason         string  `json:"reason"`
	Breakdown      string  `json:"breakdown"` // JSON string of sub-scores
}

// SummarizerOptions configures the OpenAI-compatible client.
type SummarizerOptions struct {
	APIKey        string
	Model         string
	BaseURL       string
	Timeout       time.Duration
	MaxInputChars int
	FallbackChars int
	Logger        *slog.Logger
}

type openAISummarizer struct {
	apiKey        string
	model         string
	endpoint      string
	client        *http.Client
	maxInputChars int
	fallbackChars int
	logger        *slog.Logger
}

// NewSummarizer builds a Summarizer. With an empty APIKey it always uses
// the truncation fallback.
func NewSummarizer(opts SummarizerOptions) Summarizer {
	if opts.MaxInputChars <= 0 {
		opts.MaxInputChars = 8000
	}
	if opts.FallbackChars <= 0 {
		opts.FallbackChars = 800
	}
	if opts.Timeout <= 0 {
		opts.Timeout = 30 * time.Second
	}
	if opts.Logger == nil {
		opts.Logger = slog.Default()
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
	apiKey, baseURL, model := resolveAIConfig(s.apiKey, s.model, s.endpoint)
	if apiKey == "" {
		return fallback(content, s.fallbackChars)
	}

	endpoint := s.endpoint
	if baseURL != "" {
		endpoint = strings.TrimRight(baseURL, "/") + "/chat/completions"
	}

	prompt := fmt.Sprintf(
		"Summarize the following AI news article for a tech-savvy audience. "+
			"First, write a 2-sentence overview of the key announcement. "+
			"Second, write a 1-sentence explanation highlighting its practical implications for developers or the tech industry. "+
			"Write in English. Keep it highly informative, concise, and do not use markdown.\n\nTitle: %s\n\nContent: %s",
		title, truncate(stripHTML(content), s.maxInputChars),
	)

	text, ok := chatCompletion(ctx, s.client, s.logger, endpoint, apiKey, model,
		"You are a professional news editor writing concise summaries.",
		prompt, 1200)
	if !ok {
		return fallback(content, s.fallbackChars)
	}
	return html.UnescapeString(text)
}

// validCategories defines the set of allowed category slugs.
var validCategories = map[string]bool{
	"ai":               true,
	"tools":            true,
	"research":         true,
	"industry":         true,
	"machine-learning": true,
}

func (s *openAISummarizer) Categorize(ctx context.Context, title, content, defaultCategory string) string {
	apiKey, baseURL, model := resolveAIConfig(s.apiKey, s.model, s.endpoint)
	if apiKey == "" {
		return defaultCategory
	}

	endpoint := s.endpoint
	if baseURL != "" {
		endpoint = strings.TrimRight(baseURL, "/") + "/chat/completions"
	}

	prompt := fmt.Sprintf(
		"Classify the following tech news article into exactly ONE of these categories: ai, tools, research, industry, machine-learning\n\n"+
			"Category definitions:\n"+
			"- ai: General artificial intelligence news, announcements, product launches, model releases, AI policy and safety\n"+
			"- tools: Developer tools, frameworks, libraries, platforms, SDKs, APIs, cloud services, deployment guides\n"+
			"- research: Academic papers, scientific breakthroughs, experimental results, theoretical advances, novel algorithms\n"+
			"- industry: Business deals, funding, acquisitions, partnerships, market analysis, enterprise adoption, company strategy\n"+
			"- machine-learning: ML techniques, training methods, fine-tuning, optimization, datasets, benchmarks, MLOps\n\n"+
			"Respond with ONLY the category slug (one of: ai, tools, research, industry, machine-learning). No explanation.\n\n"+
			"Title: %s\n\nContent: %s",
		title, truncate(stripHTML(content), 2000),
	)

	text, ok := chatCompletion(ctx, s.client, s.logger, endpoint, apiKey, model,
		"You are a news classifier. Respond with only a single category slug.",
		prompt, 400)
	if !ok {
		return defaultCategory
	}

	result := strings.TrimSpace(strings.ToLower(text))
	if validCategories[result] {
		return result
	}
	s.logger.Warn("ai: categorize returned invalid category, using default",
		"result", result,
		"default", defaultCategory,
	)
	return defaultCategory
}

// ScoreValue asks the AI to rate the article's news value. The returned
// scores are advisory; the caller combines them with heuristic signals and
// admins remain the final decision makers. ok is false when no API key is
// configured or the upstream call fails.
func (s *openAISummarizer) ScoreValue(ctx context.Context, title, content, source string) (ValueScore, bool) {
	apiKey, baseURL, model := resolveAIConfig(s.apiKey, s.model, s.endpoint)
	if apiKey == "" {
		return ValueScore{}, false
	}

	endpoint := s.endpoint
	if baseURL != "" {
		endpoint = strings.TrimRight(baseURL, "/") + "/chat/completions"
	}

	prompt := fmt.Sprintf(
		"Rate the news value of the following AI/tech article on a scale of 0 to 100 "+
			"for a curated tech news website that filters high-value AI news.\n\n"+
			"Consider three sub-scores, each 0-100:\n"+
			"- impact: how many people/companies it affects and how significant the industry shift is\n"+
			"- novelty: whether it is genuinely new vs a rehash; source authority (official announcement > rumor)\n"+
			"- quality: strength of evidence, presence of concrete data, depth and originality of the report\n\n"+
			"Return ONLY valid JSON, no markdown, exactly in this shape:\n"+
			"{\"score\": <0-100>, \"impact\": <0-100>, \"novelty\": <0-100>, \"quality\": <0-100>, "+
			"\"confidence\": <0.0-1.0>, \"recommendation\": \"publish\"|\"consider\"|\"review\", \"reason\": \"<1-2 sentences>\"}\n\n"+
			"Source: %s\nTitle: %s\n\nContent:\n%s",
		source, title, truncate(stripHTML(content), s.maxInputChars),
	)

	text, ok := chatCompletion(ctx, s.client, s.logger, endpoint, apiKey, model,
		"You are a news editor scoring article value. Always respond with the exact JSON shape requested.",
		prompt, 1600)
	if !ok {
		return ValueScore{}, false
	}

	return parseValueScore(text)
}

// valueScoreJSON mirrors the AI's JSON response for decoding.
type valueScoreJSON struct {
	Score          int     `json:"score"`
	Impact         int     `json:"impact"`
	Novelty        int     `json:"novelty"`
	Quality        int     `json:"quality"`
	Confidence     float64 `json:"confidence"`
	Recommendation string  `json:"recommendation"`
	Reason         string  `json:"reason"`
}

// parseValueScore extracts a ValueScore from the AI response text. It is
// tolerant: strips code fences, finds the first JSON object, and clamps all
// numbers to valid ranges. It returns ok=false when no JSON could be parsed.
func parseValueScore(text string) (ValueScore, bool) {
	text = strings.TrimSpace(text)
	// Strip surrounding ```json ... ``` fences if present.
	if strings.HasPrefix(text, "```") {
		if start := strings.Index(text, "\n"); start >= 0 {
			text = text[start+1:]
		}
		text = strings.TrimSuffix(text, "```")
		text = strings.TrimSpace(text)
	}
	// Find the first '{' and last '}' to isolate the JSON object.
	start := strings.Index(text, "{")
	end := strings.LastIndex(text, "}")
	if start < 0 || end <= start {
		return ValueScore{}, false
	}
	var parsed valueScoreJSON
	if err := json.Unmarshal([]byte(text[start:end+1]), &parsed); err != nil {
		return ValueScore{}, false
	}

	clamp := func(v int) int {
		if v < 0 {
			return 0
		}
		if v > 100 {
			return 100
		}
		return v
	}
	conf := parsed.Confidence
	if conf < 0 {
		conf = 0
	}
	if conf > 1 {
		conf = 1
	}
	rec := strings.ToLower(strings.TrimSpace(parsed.Recommendation))
	switch rec {
	case "publish", "consider", "review":
	default:
		rec = "review"
	}

	bd, _ := json.Marshal(map[string]int{
		"impact":  clamp(parsed.Impact),
		"novelty": clamp(parsed.Novelty),
		"quality": clamp(parsed.Quality),
	})
	return ValueScore{
		Score:          clamp(parsed.Score),
		Impact:         clamp(parsed.Impact),
		Novelty:        clamp(parsed.Novelty),
		Quality:        clamp(parsed.Quality),
		Confidence:     conf,
		Recommendation: rec,
		Reason:         strings.TrimSpace(parsed.Reason),
		Breakdown:      string(bd),
	}, true
}

// fallback returns the first n characters of the plain-text content.
func fallback(content string, n int) string {
	text := strings.Join(strings.Fields(stripHTML(content)), " ")
	text = html.UnescapeString(text)

	// Clean up common navigation and sharing boilerplate prefixes
	prefixes := []string{
		"Back to Articles",
		"Back to articles",
		"back to articles",
		"Share on Twitter",
		"Share on Facebook",
		"Share on LinkedIn",
		"TL;DR:",
		"TL;DR",
	}
	for {
		changed := false
		for _, p := range prefixes {
			if strings.HasPrefix(text, p) {
				text = strings.TrimPrefix(text, p)
				text = strings.TrimSpace(text)
				changed = true
			}
		}
		if !changed {
			break
		}
	}

	return truncate(text, n)
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	sub := s[:n]
	// Try to find the last sentence ending (. ! ?) within the limit
	lastDot := strings.LastIndexAny(sub, ".!?")
	if lastDot > n/2 {
		return strings.TrimSpace(sub[:lastDot+1])
	}
	// Fallback to cutting at the last word boundary
	lastSpace := strings.LastIndex(sub, " ")
	if lastSpace > n/2 {
		return strings.TrimSpace(sub[:lastSpace]) + "..."
	}
	return strings.TrimSpace(sub) + "..."
}

var tagRegex = regexp.MustCompile(`<[^>]*>`)

func stripHTML(s string) string {
	return tagRegex.ReplaceAllString(s, " ")
}
