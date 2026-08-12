package slug

import "testing"

func TestFromTitle(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{name: "simple", input: "Hello World", want: "hello-world"},
		{name: "multiple spaces", input: "OpenAI   releases   GPT", want: "openai-releases-gpt"},
		{name: "punctuation", input: "AI, ML & LLMs: 2025 Guide!", want: "ai-ml-llms-2025-guide"},
		{name: "non-ascii dropped", input: "ニュース の 発表", want: "untitled"},
		{name: "mixed case", input: "GoLang Backend", want: "golang-backend"},
		{name: "dashes preserved", input: "state-of-the-art Models", want: "state-of-the-art-models"},
		{name: "empty", input: "   ", want: "untitled"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := FromTitle(tt.input); got != tt.want {
				t.Errorf("FromTitle(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestUnique(t *testing.T) {
	used := map[string]bool{"hello-world": true, "hello-world-2": true}

	exists := func(s string) (bool, error) { return used[s], nil }

	got, err := Unique("hello-world", exists)
	if err != nil {
		t.Fatalf("Unique returned error: %v", err)
	}
	if got != "hello-world-3" {
		t.Errorf("Unique = %q, want %q", got, "hello-world-3")
	}
}

func TestFromName(t *testing.T) {
	if got := FromName("Machine Learning"); got != "machine-learning" {
		t.Errorf("FromName = %q, want %q", got, "machine-learning")
	}
	if got := FromName(""); got != "other" {
		t.Errorf("FromName(empty) = %q, want %q", got, "other")
	}
}
