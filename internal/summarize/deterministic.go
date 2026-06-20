// Package summarize provides Summarizer implementations behind the
// core.Summarizer seam. Deterministic is the default and the always-available
// fallback: it builds a structural digest with no LLM call, so compaction works
// offline and reproducibly. LLM-backed summarizers (reusing the host agent's
// model) are provided alongside it.
package summarize

import (
	"context"
	"fmt"
	"strings"

	"github.com/bonztm/agent-context-manager/internal/core"
)

// Deterministic produces a structural, no-LLM summary: a header plus one
// trimmed preview line per source, bounded by the requested token target.
type Deterministic struct{}

// Summarize renders the digest. It never returns an error.
func (Deterministic) Summarize(_ context.Context, in core.SummarizeInput) (string, error) {
	if len(in.Sources) == 0 {
		return "", nil
	}
	target := in.TargetTokens
	if target <= 0 {
		target = 512
	}
	// Split the token budget across sources (~4 chars per token), with a floor so
	// each source still gets a usable preview.
	perSourceChars := max(target/len(in.Sources), 8) * 4

	lines := make([]string, 0, len(in.Sources)+1)
	lines = append(lines, fmt.Sprintf("[%s summary depth=%d sources=%d]", in.Kind, in.Depth, len(in.Sources)))
	for _, src := range in.Sources {
		lines = append(lines, "- "+previewLine(src, perSourceChars))
	}
	return strings.Join(lines, "\n"), nil
}

// previewLine collapses internal whitespace and truncates to maxChars runes.
func previewLine(s string, maxChars int) string {
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
