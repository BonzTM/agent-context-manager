package summarize

import (
	"context"
	"strings"
	"testing"

	"github.com/bonztm/agent-context-manager/internal/core"
)

func TestDeterministicSummarizeShrinksAndStructures(t *testing.T) {
	var d Deterministic
	sources := []string{
		"user: " + strings.Repeat("alpha ", 200),
		"assistant: " + strings.Repeat("beta ", 200),
		"tool: " + strings.Repeat("gamma ", 200),
	}
	out, err := d.Summarize(context.Background(), core.SummarizeInput{
		Kind:         core.SummaryLeaf,
		Depth:        0,
		Sources:      sources,
		TargetTokens: 60,
	})
	if err != nil {
		t.Fatalf("summarize: %v", err)
	}
	if !strings.Contains(out, "leaf summary") {
		t.Fatalf("summary missing header: %q", out)
	}
	if strings.Count(out, "\n- ") != 3 {
		t.Fatalf("expected 3 bullet lines, got: %q", out)
	}
	// The digest must be smaller than the combined input.
	inputLen := 0
	for _, s := range sources {
		inputLen += len(s)
	}
	if len(out) >= inputLen {
		t.Fatalf("summary not smaller than input: %d >= %d", len(out), inputLen)
	}
}

func TestDeterministicSummarizeEmpty(t *testing.T) {
	var d Deterministic
	out, err := d.Summarize(context.Background(), core.SummarizeInput{Sources: nil})
	if err != nil {
		t.Fatalf("summarize: %v", err)
	}
	if out != "" {
		t.Fatalf("expected empty summary, got %q", out)
	}
}
