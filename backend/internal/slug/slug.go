// Package slug provides title-to-slug conversion for news articles.
package slug

import (
	"strings"
	"unicode"
)

// FromTitle converts a title into a URL-safe slug. Non-ASCII characters
// are dropped, whitespace becomes hyphens, and sequences are collapsed.
func FromTitle(title string) string {
	var b strings.Builder
	prevDash := false

	for _, r := range strings.ToLower(strings.TrimSpace(title)) {
		switch {
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			if r < 128 {
				b.WriteRune(r)
				prevDash = false
			}
		case unicode.IsSpace(r) || r == '-' || r == '_':
			if !prevDash && b.Len() > 0 {
				b.WriteByte('-')
				prevDash = true
			}
		}
	}

	out := strings.Trim(b.String(), "-")
	if out == "" {
		return "untitled"
	}
	return out
}

// FromName converts a category/name into a slug, falling back to "other"
// when the input contains no usable ASCII characters.
func FromName(name string) string {
	s := FromTitle(name)
	if s == "untitled" {
		return "other"
	}
	return s
}

// Unique returns a slug guaranteed unique against the exists function by
// appending a numeric suffix when necessary.
func Unique(base string, exists func(string) (bool, error)) (string, error) {
	candidate := base
	for i := 2; ; i++ {
		ok, err := exists(candidate)
		if err != nil {
			return "", err
		}
		if !ok {
			return candidate, nil
		}
		candidate = base + "-" + itoa(i)
	}
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var buf [20]byte
	pos := len(buf)
	for n > 0 {
		pos--
		buf[pos] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[pos:])
}
