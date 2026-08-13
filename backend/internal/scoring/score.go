// Package scoring rates the news value of fetched articles. It combines an
// AI opinion (when available) with deterministic heuristic signals into a
// single advisory score and HIGH/MEDIUM/LOW label. Scoring never publishes:
// admins remain the final decision makers.
package scoring

import (
	"context"
	"encoding/json"
	"regexp"
	"strconv"
	"strings"

	"neuralwire/backend/internal/ai"
	"neuralwire/backend/internal/models"
)

// AIScorer is the AI component. *ai.openAISummarizer satisfies it via the
// Summarizer interface; test doubles implement it directly.
type AIScorer interface {
	ScoreValue(ctx context.Context, title, content, source string) (ai.ValueScore, bool)
}

// HeuristicScorer produces deterministic sub-scores from the article text.
type HeuristicScorer interface {
	Score(title, content, source string) HeuristicResult
}

// HeuristicResult is the deterministic rule-based assessment.
type HeuristicResult struct {
	Score   int `json:"score"`
	Source  int `json:"source"`
	Signal  int `json:"signal"`
	Quality int `json:"quality"`
}

// ThresholdStore loads the configurable HIGH/MEDIUM/LOW bounds.
type ThresholdStore interface {
	GetScoreThresholds() models.ScoreThresholds
}

// ScoreService combines AI + heuristic scoring into the final advisory score.
type ScoreService struct {
	ai      AIScorer
	heur    HeuristicScorer
	thresh  ThresholdStore
	weights ScoreWeights
}

// ScoreWeights controls how much each signal contributes to the final score.
type ScoreWeights struct {
	AI        float64 // weight of the AI score (default 0.6)
	Heuristic float64 // weight of the heuristic score (default 0.4)
}

// Result is the final advisory rating stored on a news article.
type Result struct {
	Score          int     `json:"score"`
	Breakdown      string  `json:"breakdown"`
	Confidence     float64 `json:"confidence"`
	Recommendation string  `json:"recommendation"`
	Reason         string  `json:"reason"`
	Label          string  `json:"label"`
	Method         string  `json:"method"`
}

// NewScoreService wires the components together.
func NewScoreService(aiScorer AIScorer, heur HeuristicScorer, thresh ThresholdStore) *ScoreService {
	return &ScoreService{
		ai:      aiScorer,
		heur:    heur,
		thresh:  thresh,
		weights: ScoreWeights{AI: 0.6, Heuristic: 0.4},
	}
}

// Score evaluates the article and returns the final advisory rating. When AI
// scoring is unavailable it degrades to the pure heuristic score with a low
// confidence, so drafts always carry a value score.
func (s *ScoreService) Score(ctx context.Context, title, content, source string) Result {
	heur := s.heur.Score(title, content, source)

	// Try AI first; fall back to heuristic alone on any failure.
	aiScore, ok := s.ai.ScoreValue(ctx, title, content, source)
	if !ok {
		label := s.thresh.GetScoreThresholds().Label(heur.Score)
		bd, _ := json.Marshal(map[string]any{
			"ai":        "unavailable",
			"heuristic": heur,
		})
		return Result{
			Score:          heur.Score,
			Breakdown:      string(bd),
			Confidence:     0.3,
			Recommendation: "review",
			Reason:         "AI scoring unavailable; scored by heuristic rules.",
			Label:          label,
			Method:         "heuristic",
		}
	}

	// Weighted blend: 60% AI + 40% heuristic.
	final := int(float64(aiScore.Score)*s.weights.AI + float64(heur.Score)*s.weights.Heuristic)
	if final > 100 {
		final = 100
	}
	if final < 0 {
		final = 0
	}

	label := s.thresh.GetScoreThresholds().Label(final)
	bd, _ := json.Marshal(map[string]any{
		"ai": map[string]int{
			"score":   aiScore.Score,
			"impact":  aiScore.Impact,
			"novelty": aiScore.Novelty,
			"quality": aiScore.Quality,
		},
		"heuristic": heur,
		"weights": map[string]float64{
			"ai": s.weights.AI, "heuristic": s.weights.Heuristic,
		},
	})

	reason := aiScore.Reason
	if strings.TrimSpace(reason) == "" {
		reason = "No AI reason provided."
	}
	reason = strings.TrimSpace(strings.Join([]string{reason, " (heuristic score: " + strconv.Itoa(heur.Score) + ")"}, " "))

	return Result{
		Score:          final,
		Breakdown:      string(bd),
		Confidence:     aiScore.Confidence,
		Recommendation: aiScore.Recommendation,
		Reason:         reason,
		Label:          label,
		Method:         "ai+heuristic",
	}
}

// Apply fills a models.News with the scoring Result fields.
func Apply(n *models.News, r Result) {
	n.ValueScore = r.Score
	n.ValueBreakdown = r.Breakdown
	n.ValueConfidence = r.Confidence
	n.ValueRecommendation = r.Recommendation
	n.ValueReason = r.Reason
	n.ValueLabel = r.Label
	n.ValueMethod = r.Method
}

// ---------- heuristic implementation ----------

// wordBoundary is used to count approximate words.
var wordRegexp = regexp.MustCompile(`\S+`)

// numberRegexp matches concrete figures: percentages, counts, amounts.
var numberRegexp = regexp.MustCompile(`(?i)\d+\s*(%|billion|million|thousand|k\b|tokens|parameters|params|users|models?|dollars?|\$)`)

// strongSignalRegexp matches headline words that usually indicate a
// high-impact announcement.
var strongSignalRegexp = regexp.MustCompile(`(?i)\b(launch|launches|releases|release|announces|announcement|unveils|introduces|breakthrough|research|paper|benchmark|funding|raises|acquisition|partnership|survey|report|study|opens|available|GA|general availability)\b`)

// weakSignalRegexp matches words that suggest speculation or low substance.
var weakSignalRegexp = regexp.MustCompile(`(?i)\b(rumor|rumours|reportedly|leaks?|alleged|speculat|may|might|possibly|says sources?|mystery)\b`)

// authoritySources are publishers we treat as high-authority primary sources.
var authoritySources = map[string]bool{
	"openai blog": true, "google ai blog": true, "anthropic blog": true,
	"meta ai blog": true, "deepmind blog": true, "hugging face blog": true,
	"aws machine learning": true, "github blog": true, "mit ai news": true,
	"arxiv ai": true, "techcrunch ai": true, "venturebeat ai": true,
	"the verge ai": true, "machine learning mastery": true,
}

// RuleScorer is the default deterministic HeuristicScorer.
type RuleScorer struct{}

// NewRuleScorer builds the default heuristic scorer.
func NewRuleScorer() *RuleScorer {
	return &RuleScorer{}
}

// Score computes heuristic sub-scores from source authority, headline
// signals, and evidence density (numbers + length).
func (r *RuleScorer) Score(title, content, source string) HeuristicResult {
	res := HeuristicResult{}

	// Source authority: primary official/publisher sources score higher.
	if authoritySources[strings.ToLower(strings.TrimSpace(source))] {
		res.Source = 80
	} else if strings.TrimSpace(source) != "" {
		res.Source = 50
	} else {
		res.Source = 30
	}

	// Headline signals.
	signal := 40
	if strongSignalRegexp.MatchString(title) {
		signal += 35
	}
	if weakSignalRegexp.MatchString(title) {
		signal -= 25
	}
	if signal < 0 {
		signal = 0
	}
	if signal > 100 {
		signal = 100
	}
	res.Signal = signal

	// Evidence density: concrete numbers and reasonable article length.
	quality := 40
	if numberRegexp.MatchString(title + " " + content) {
		quality += 25
	}
	words := len(wordRegexp.FindAllString(content, -1))
	if words >= 600 {
		quality += 20
	} else if words >= 200 {
		quality += 10
	}
	if weakSignalRegexp.MatchString(content) {
		quality -= 15
	}
	if quality < 0 {
		quality = 0
	}
	if quality > 100 {
		quality = 100
	}
	res.Quality = quality

	// Weighted heuristic composite: source 35%, signal 35%, quality 30%.
	score := int(0.35*float64(res.Source) + 0.35*float64(res.Signal) + 0.30*float64(res.Quality))
	if score > 100 {
		score = 100
	}
	if score < 0 {
		score = 0
	}
	res.Score = score
	return res
}
