package scoring

import (
	"context"
	"encoding/json"
	"testing"

	"neuralwire/backend/internal/ai"
	"neuralwire/backend/internal/models"
)

// fakeAIScorer lets tests control the AI verdict.
type fakeAIScorer struct {
	score ai.ValueScore
	ok    bool
}

func (f *fakeAIScorer) ScoreValue(_ context.Context, title, content, source string) (ai.ValueScore, bool) {
	return f.score, f.ok
}

// staticThresholds implements ThresholdStore with fixed bounds.
type staticThresholds struct {
	t models.ScoreThresholds
}

func (s staticThresholds) GetScoreThresholds() models.ScoreThresholds { return s.t }

func testService(score ai.ValueScore, ok bool) *ScoreService {
	return NewScoreService(
		&fakeAIScorer{score: score, ok: ok},
		NewRuleScorer(),
		staticThresholds{models.DefaultScoreThresholds()},
	)
}

func TestScoreUsesAIWhenAvailable(t *testing.T) {
	s := testService(ai.ValueScore{
		Score:          80,
		Impact:         90,
		Novelty:        85,
		Quality:        75,
		Confidence:     0.9,
		Recommendation: "publish",
		Reason:         "Major model release with broad impact.",
	}, true)

	res := s.Score(context.Background(), "OpenAI launches GPT-5 with 1T parameters", "big content", "OpenAI Blog")
	// Weighted: 0.6*80 + 0.4*heuristic. Heuristic is at least 40 => 0.4*40=16, total ~64+.
	if res.Method != "ai+heuristic" {
		t.Errorf("Method = %q, want ai+heuristic", res.Method)
	}
	if res.Recommendation != "publish" {
		t.Errorf("Recommendation = %q, want publish", res.Recommendation)
	}
	if res.Score < 60 {
		t.Errorf("Score = %d, want >= 60 for high-impact article", res.Score)
	}
	if res.Label != "MEDIUM" && res.Label != "HIGH" {
		t.Errorf("Label = %q, want MEDIUM or HIGH", res.Label)
	}
	if res.Confidence != 0.9 {
		t.Errorf("Confidence = %v, want 0.9", res.Confidence)
	}
	var bd map[string]any
	if err := json.Unmarshal([]byte(res.Breakdown), &bd); err != nil {
		t.Fatalf("breakdown not JSON: %v", err)
	}
	if _, ok := bd["ai"]; !ok {
		t.Errorf("breakdown missing ai key: %v", bd)
	}
}

func TestScoreFallsBackToHeuristic(t *testing.T) {
	s := testService(ai.ValueScore{}, false)
	res := s.Score(context.Background(), "OpenAI launches GPT-5", "content", "OpenAI Blog")
	if res.Method != "heuristic" {
		t.Errorf("Method = %q, want heuristic fallback", res.Method)
	}
	if res.Score <= 0 {
		t.Errorf("Score = %d, want positive heuristic score", res.Score)
	}
	if res.Label == "" {
		t.Errorf("Label empty, want HIGH/MEDIUM/LOW")
	}
}

func TestRuleScorerHighImpact(t *testing.T) {
	r := NewRuleScorer()
	res := r.Score("OpenAI launches GPT-5 with 1 trillion parameters and 97% benchmark", "long content about a major release with concrete numbers", "OpenAI Blog")
	if res.Score < 70 {
		t.Errorf("Score = %d, want >= 70 for authoritative launch", res.Score)
	}
}

func TestRuleScorerLowSignal(t *testing.T) {
	r := NewRuleScorer()
	res := r.Score("Rumor: maybe a new product might come", "possibly some speculation without numbers", "Unknown Blog")
	if res.Score >= 60 {
		t.Errorf("Score = %d, want < 60 for speculation", res.Score)
	}
}

func TestThresholdLabel(t *testing.T) {
	th := models.DefaultScoreThresholds()
	cases := []struct {
		score int
		want  string
	}{
		{30, "LOW"},
		{59, "LOW"},
		{60, "MEDIUM"},
		{79, "MEDIUM"},
		{80, "HIGH"},
		{100, "HIGH"},
	}
	for _, c := range cases {
		if got := th.Label(c.score); got != c.want {
			t.Errorf("Label(%d) = %q, want %q", c.score, got, c.want)
		}
	}
}
