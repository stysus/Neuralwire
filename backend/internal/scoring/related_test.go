package scoring

import "testing"

func TestRelatedScoreSignals(t *testing.T) {
	tests := []struct {
		name    string
		curCat  string
		curSrc  string
		curTxt  string
		candCat string
		candSrc string
		candTxt string
		wantGT  int // score must be > this
	}{
		{
			name:   "same category and source plus keyword overlap",
			curCat: "ai", curSrc: "DeepMind Blog",
			curTxt:  "Gemma 4 12B multimodal model released by Google DeepMind",
			candCat: "ai", candSrc: "DeepMind Blog",
			candTxt: "Gemma model multimodal encoder free for laptops",
			wantGT:  50,
		},
		{
			name:   "different category and source, no keywords",
			curCat: "ai", curSrc: "DeepMind Blog",
			curTxt:  "Gemma 4 12B multimodal model released",
			candCat: "industry", candSrc: "TechCrunch AI",
			candTxt: "startup funding round acquisition",
			wantGT:  -1, // any score is acceptable, but typically 0
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := RelatedScore(tt.curCat, tt.curSrc, tt.curTxt, tt.candCat, tt.candSrc, tt.candTxt)
			if got <= tt.wantGT {
				t.Errorf("RelatedScore = %d, want > %d", got, tt.wantGT)
			}
		})
	}
}

func TestRelatedScoreSameCategoryRanking(t *testing.T) {
	// Same category alone should outrank a different-category article.
	sameCat := RelatedScore("ai", "Src", "gemma model", "ai", "Other", "totally unrelated topic")
	difCat := RelatedScore("ai", "Src", "gemma model", "industry", "Other", "totally unrelated topic")
	if sameCat <= difCat {
		t.Errorf("same-category score %d should exceed different-category %d", sameCat, difCat)
	}
}

func TestRelatedKeywords(t *testing.T) {
	kws := RelatedKeywords("Gemma 4 12B multimodal model released for laptops")
	if kws["gemma"] == 0 {
		t.Error("expected keyword gemma extracted")
	}
	if kws["for"] != 0 {
		t.Error("stopword 'for' should be excluded")
	}
	if kws["4"] != 0 {
		t.Error("pure numeric token should be excluded")
	}
}
