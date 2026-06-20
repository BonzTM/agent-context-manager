package cli

import "strings"

// truncateLine collapses whitespace and truncates s to at most maxChars runes,
// for single-line previews in command output.
func truncateLine(s string, maxChars int) string {
	collapsed := strings.Join(strings.Fields(s), " ")
	if maxChars < 1 {
		maxChars = 1
	}
	r := []rune(collapsed)
	if len(r) <= maxChars {
		return collapsed
	}
	return string(r[:maxChars]) + "…"
}

// joinArgs joins remaining positional args into a single space-separated query.
func joinArgs(args []string) string {
	return strings.Join(args, " ")
}
