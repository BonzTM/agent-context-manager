package summarize

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/bonztm/agent-context-manager/internal/core"
)

func TestCLISummarizerUsesRunnerOutput(t *testing.T) {
	var gotStdin string
	runner := func(_ context.Context, _ []string, stdin string) (string, error) {
		gotStdin = stdin
		return "  THE SUMMARY  ", nil
	}
	s := Claude(runner, Deterministic{}) // prompt on stdin
	out, err := s.Summarize(context.Background(), core.SummarizeInput{
		Kind: core.SummaryLeaf, Sources: []string{"alpha beta", "gamma"}, TargetTokens: 50,
	})
	if err != nil {
		t.Fatalf("summarize: %v", err)
	}
	if out != "THE SUMMARY" {
		t.Fatalf("out = %q, want trimmed runner output", out)
	}
	if !strings.Contains(gotStdin, "alpha beta") {
		t.Fatalf("runner stdin did not include source text: %q", gotStdin)
	}
}

func TestCLISummarizerFallsBackOnError(t *testing.T) {
	runner := func(_ context.Context, _ []string, _ string) (string, error) {
		return "", errors.New("binary not found")
	}
	s := Codex(runner, Deterministic{}) // prompt as arg
	out, err := s.Summarize(context.Background(), core.SummarizeInput{
		Kind: core.SummaryLeaf, Sources: []string{"alpha", "beta"}, TargetTokens: 50,
	})
	if err != nil {
		t.Fatalf("summarize: %v", err)
	}
	// Deterministic fallback produces a structured digest.
	if !strings.Contains(out, "leaf summary") {
		t.Fatalf("expected deterministic fallback, got %q", out)
	}
}

func TestCLISummarizerFallsBackOnEmpty(t *testing.T) {
	runner := func(_ context.Context, _ []string, _ string) (string, error) {
		return "   ", nil
	}
	s := NewCLISummarizer([]string{"x"}, false, runner, Deterministic{})
	out, err := s.Summarize(context.Background(), core.SummarizeInput{
		Kind: core.SummaryCondensed, Sources: []string{"a"}, TargetTokens: 10,
	})
	if err != nil {
		t.Fatalf("summarize: %v", err)
	}
	if out == "" {
		t.Fatal("expected non-empty fallback output")
	}
}
