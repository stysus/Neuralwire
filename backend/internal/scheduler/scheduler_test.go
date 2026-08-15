package scheduler

import (
	"testing"

	"neuralwire/backend/internal/models"
)

func TestLabelAtLeast(t *testing.T) {
	cases := []struct {
		got, want string
		wantOK    bool
	}{
		{"high", "high", true},
		{"high", "medium", true},
		{"high", "low", true},
		{"medium", "medium", true},
		{"medium", "low", true},
		{"medium", "high", false},
		{"low", "low", true},
		{"low", "medium", false},
		{"", "low", false},
		{"unknown", "low", false},
		{"LOW", "low", true}, // case-insensitive
		{"HIGH", "medium", true},
	}
	for _, c := range cases {
		if got := labelAtLeast(c.got, c.want); got != c.wantOK {
			t.Errorf("labelAtLeast(%q, %q) = %v, want %v", c.got, c.want, got, c.wantOK)
		}
	}
}

func TestFilterMatch(t *testing.T) {
	cfg := models.AutoPublishConfig{
		Categories:    []string{"ai"},
		MinScoreLabel: "high",
	}
	news := models.News{Category: "AI", ValueLabel: "HIGH"}
	if !filterMatch(news, cfg) {
		t.Error("expected AI + HIGH to match category ai + min high")
	}
	news.Category = "Industry"
	if filterMatch(news, cfg) {
		t.Error("expected Industry to be filtered out by category whitelist")
	}
	news.Category = "AI"
	news.ValueLabel = "MEDIUM"
	if filterMatch(news, cfg) {
		t.Error("expected MEDIUM to be filtered out by min high")
	}
	// Empty whitelist = all categories.
	cfg.Categories = []string{}
	news.Category = "Tools"
	news.ValueLabel = "HIGH"
	if !filterMatch(news, cfg) {
		t.Error("expected empty category whitelist to allow any category")
	}
}
