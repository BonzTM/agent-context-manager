package summarize

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"

	"github.com/bonztm/agent-context-manager/internal/core"
)

// Runner executes a command with stdin and returns its stdout. It is a seam so
// CLISummarizer can be tested without invoking a real agent CLI.
type Runner func(ctx context.Context, argv []string, stdin string) (string, error)

// ExecRunner runs argv as a subprocess (no shell), feeding stdin and capturing
// stdout. exec.Command does not interpret a shell, and argv is built from a
// fixed agent-CLI template plus the summarization prompt passed as data, so
// there is no command injection here.
func ExecRunner(ctx context.Context, argv []string, stdin string) (string, error) {
	if len(argv) == 0 {
		return "", errors.New("summarize: empty command")
	}
	//nolint:gosec // G204: argv is a configured agent CLI (claude/codex); the prompt is passed as data (stdin/arg), not shell-interpreted — exec.Command runs no shell.
	cmd := exec.CommandContext(ctx, argv[0], argv[1:]...)
	cmd.Stdin = strings.NewReader(stdin)
	var out, errBuf bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errBuf
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("summarize: exec %s: %w: %s", argv[0], err, strings.TrimSpace(errBuf.String()))
	}
	return out.String(), nil
}

// CLISummarizer reuses a host agent's own model for summarization by invoking it
// in headless mode (e.g. claude -p, codex exec) — so acm carries no separate
// credentials. Any failure (binary missing, auth, rate limit) falls back to the
// deterministic summarizer, guaranteeing forward progress.
type CLISummarizer struct {
	argv          []string
	promptOnStdin bool
	run           Runner
	fallback      core.Summarizer
}

// NewCLISummarizer builds a CLI-backed summarizer. If promptOnStdin is true the
// prompt is fed on stdin; otherwise it is appended as the final argv element.
func NewCLISummarizer(argv []string, promptOnStdin bool, run Runner, fallback core.Summarizer) *CLISummarizer {
	return &CLISummarizer{argv: argv, promptOnStdin: promptOnStdin, run: run, fallback: fallback}
}

// Claude builds a summarizer that reuses Claude Code's model via `claude -p`.
// (Validate the exact flags against your installed CLI; the deterministic
// fallback covers any mismatch.)
func Claude(run Runner, fallback core.Summarizer) *CLISummarizer {
	return NewCLISummarizer([]string{"claude", "-p", "--model", "haiku", "--output-format", "text"}, true, run, fallback)
}

// Codex builds a summarizer that reuses Codex's model via `codex exec`.
func Codex(run Runner, fallback core.Summarizer) *CLISummarizer {
	return NewCLISummarizer([]string{"codex", "exec", "-c", "model=gpt-5.4-mini"}, false, run, fallback)
}

// Summarize runs the agent CLI, falling back to the deterministic summarizer on
// any error or empty output.
func (c *CLISummarizer) Summarize(ctx context.Context, in core.SummarizeInput) (string, error) {
	prompt := buildPrompt(in)
	argv := c.argv
	stdin := ""
	if c.promptOnStdin {
		stdin = prompt
	} else {
		argv = append(append([]string{}, c.argv...), prompt)
	}

	out, err := c.run(ctx, argv, stdin)
	if err != nil {
		return c.fallback.Summarize(ctx, in)
	}
	res := strings.TrimSpace(out)
	if res == "" {
		return c.fallback.Summarize(ctx, in)
	}
	return res, nil
}

// Answer asks the host agent's model a focused question over the sources and
// returns the synthesized answer. Unlike Summarize it has no deterministic
// fallback — there is no offline way to answer a question — so an error or
// empty output is returned to the caller, which should degrade to showing the
// raw sources instead.
func (c *CLISummarizer) Answer(ctx context.Context, question string, sources []string, maxTokens int) (string, error) {
	prompt := buildAnswerPrompt(question, sources, maxTokens)
	argv := c.argv
	stdin := ""
	if c.promptOnStdin {
		stdin = prompt
	} else {
		argv = append(append([]string{}, c.argv...), prompt)
	}

	out, err := c.run(ctx, argv, stdin)
	if err != nil {
		return "", fmt.Errorf("summarize: answer: %w", err)
	}
	res := strings.TrimSpace(out)
	if res == "" {
		return "", errors.New("summarize: answer: empty model output")
	}
	return res, nil
}

func buildAnswerPrompt(question string, sources []string, maxTokens int) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Answer the question below using ONLY the sources that follow, in at most %d tokens. ", maxTokens)
	fmt.Fprint(&b, "Cite the message ids (msg_...) you drew on. ")
	fmt.Fprint(&b, "If the sources do not contain the answer, say so plainly. Output only the answer, no preamble.\n\n")
	fmt.Fprintf(&b, "QUESTION: %s\n\n", question)
	for i, s := range sources {
		fmt.Fprintf(&b, "--- source %d ---\n%s\n", i+1, s)
	}
	return b.String()
}

func buildPrompt(in core.SummarizeInput) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Compress the following %s context into at most %d tokens. ", in.Kind, in.TargetTokens)
	fmt.Fprint(&b, depthGuidance(in.Depth))
	fmt.Fprint(&b, "Output only the summary, no preamble.\n\n")
	for i, s := range in.Sources {
		fmt.Fprintf(&b, "--- source %d ---\n%s\n", i+1, s)
	}
	return b.String()
}

// depthGuidance tailors what a summary should preserve to its level in the DAG:
// leaves keep concrete detail, low condensed levels keep the narrative arc, and
// deep levels keep only what stays true for the whole project.
func depthGuidance(depth int) string {
	switch {
	case depth <= 0:
		return "Preserve key decisions, identifiers, file paths, commands, error messages, and unresolved questions. "
	case depth == 1:
		return "Summarize the arc of work: goals, decisions and their outcomes, files touched, and anything still unresolved. Drop step-by-step detail. "
	default:
		return "Produce a durable, self-contained narrative: the goal, the final outcome, lasting decisions, and what carries forward. Drop transient detail. "
	}
}
