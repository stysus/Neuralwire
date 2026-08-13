package scoring

import (
	"regexp"
	"strings"
)

// RelatedKeywords extracts meaningful keywords from an article's title and
// summary: lowercase tokens, stripped of punctuation/numbers, excluding
// common stopwords. Short tokens (<3 chars) and pure numbers are dropped.
func RelatedKeywords(text string) map[string]int {
	re := regexp.MustCompile(`[a-z][a-z0-9]{1,}`)
	tokens := re.FindAllString(strings.ToLower(text), -1)

	stopwords := map[string]bool{
		"the": true, "and": true, "for": true, "with": true, "from": true,
		"that": true, "this": true, "are": true, "was": true, "were": true,
		"has": true, "have": true, "had": true, "its": true, "their": true,
		"them": true, "they": true, "you": true, "your": true, "our": true,
		"not": true, "but": true, "how": true, "what": true, "why": true,
		"who": true, "when": true, "where": true, "which": true, "into": true,
		"about": true, "over": true, "under": true, "more": true, "most": true,
		"than": true, "will": true, "would": true, "can": true, "could": true,
		"should": true, "may": true, "might": true, "new": true, "one": true,
		"two": true, "via": true, "also": true, "using": true, "used": true,
		"based": true, "such": true, "these": true, "those": true, "been": true,
		"being": true, "make": true, "makes": true, "made": true, "use": true,
		"help": true, "helps": true, "first": true, "after": true, "before": true,
		"while": true, "out": true, "all": true,
		"some": true, "any": true, "each": true, "both": true, "few": true,
	}

	keywords := make(map[string]int)
	for _, tok := range tokens {
		if stopwords[tok] {
			continue
		}
		// Ignore purely numeric tokens and very short ones.
		if isNumeric(tok) || len(tok) < 3 {
			continue
		}
		keywords[tok]++
	}
	return keywords
}

// RelatedScore computes a relevance score between the current article and a
// candidate. Signals (weights tunable):
//   - same category: +40
//   - same source: +10
//   - keyword overlap in title+summary: +2 per shared keyword
//
// Returns a score >= 0; higher is more related.
func RelatedScore(currentCategory, currentSource, currentText, candCategory, candSource, candText string) int {
	score := 0
	if strings.EqualFold(strings.TrimSpace(currentCategory), strings.TrimSpace(candCategory)) {
		score += 40
	}
	if strings.EqualFold(strings.TrimSpace(currentSource), strings.TrimSpace(candSource)) {
		score += 10
	}

	cur := RelatedKeywords(currentText)
	cand := RelatedKeywords(candText)
	// Count shared keywords (weighted by min frequency to reward overlap
	// strength, capped to avoid title-token spam dominating).
	for kw, c := range cur {
		if cc, ok := cand[kw]; ok {
			n := c
			if cc < n {
				n = cc
			}
			if n > 2 {
				n = 2
			}
			score += 2 * n
		}
	}
	return score
}

var numericRe = regexp.MustCompile(`^[0-9]+$`)

func isNumeric(s string) bool {
	return numericRe.MatchString(s)
}
