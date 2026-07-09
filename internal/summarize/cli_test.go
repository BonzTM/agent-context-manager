package summarize

import (
	"context"
	"errors"
	"runtime"
	"strings"
	"testing"
	"time"

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

func TestAnswerUsesRunnerOutput(t *testing.T) {
	var gotPrompt string
	runner := func(_ context.Context, argv []string, stdin string) (string, error) {
		gotPrompt = stdin
		if stdin == "" && len(argv) > 0 {
			gotPrompt = argv[len(argv)-1]
		}
		return "  because sqlite is zero-infrastructure [msg_abc]  ", nil
	}
	s := Claude(runner, Deterministic{})
	out, err := s.Answer(context.Background(), "why sqlite?",
		[]string{"[msg_abc seq=1 user] we want zero infrastructure"}, 500)
	if err != nil {
		t.Fatalf("answer: %v", err)
	}
	if out != "because sqlite is zero-infrastructure [msg_abc]" {
		t.Fatalf("out = %q, want trimmed runner output", out)
	}
	for _, want := range []string{"why sqlite?", "msg_abc", "500 tokens", "Cite the message ids"} {
		if !strings.Contains(gotPrompt, want) {
			t.Errorf("answer prompt missing %q:\n%s", want, gotPrompt)
		}
	}
}

func TestAnswerErrorsPropagate(t *testing.T) {
	s := Codex(func(_ context.Context, _ []string, _ string) (string, error) {
		return "", errors.New("binary not found")
	}, Deterministic{})
	if _, err := s.Answer(context.Background(), "q", []string{"src"}, 100); err == nil {
		t.Fatal("expected error to propagate (no deterministic fallback for answers)")
	}

	empty := NewCLISummarizer([]string{"x"}, true, func(_ context.Context, _ []string, _ string) (string, error) {
		return "   ", nil
	}, Deterministic{})
	if _, err := empty.Answer(context.Background(), "q", []string{"src"}, 100); err == nil {
		t.Fatal("expected empty model output to be an error")
	}
}

func TestExecRunnerTimesOut(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("process-group timeout assertion uses a POSIX shell")
	}
	started := time.Now()
	_, err := execRunner(context.Background(), []string{"sh", "-c", "sleep 10"}, "", 20*time.Millisecond)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("error = %v, want context deadline exceeded", err)
	}
	if elapsed := time.Since(started); elapsed > time.Second {
		t.Fatalf("timeout returned after %s, want under 1s", elapsed)
	}
}
