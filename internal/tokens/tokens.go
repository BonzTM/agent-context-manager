// Package tokens provides token-count estimation behind the core.TokenCounter
// seam. The default Heuristic is fast and dependency-free; it deliberately does
// not replicate any model's exact tokenizer (the LCM budgeting thresholds are
// coarse, and the reference implementations use approximate counters too). A
// model-specific counter can be swapped in at the wiring site.
package tokens

import "unicode"

// Heuristic estimates tokens without external data files.
type Heuristic struct{}

// Count estimates the number of tokens in s as the larger of a character-based
// (~4 chars/token) and a word-based (~1.33 tokens/word) approximation, since
// either measure alone is fooled by very long words or whitespace-heavy text.
func (Heuristic) Count(s string) int {
	if s == "" {
		return 0
	}

	chars := len([]rune(s))
	byChars := (chars + 3) / 4

	words := 0
	inWord := false
	for _, r := range s {
		if unicode.IsSpace(r) {
			inWord = false
			continue
		}
		if !inWord {
			words++
			inWord = true
		}
	}
	byWords := (words*4 + 2) / 3

	if byWords > byChars {
		return byWords
	}
	return byChars
}
