package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"slices"
	"time"

	"github.com/spf13/cobra"

	"github.com/bonztm/agent-context-manager/internal/core"
	"github.com/bonztm/agent-context-manager/internal/engine"
	"github.com/bonztm/agent-context-manager/internal/summarize"
)

func newExpandCmd(a *app) *cobra.Command {
	var (
		toMessages bool
		asJSON     bool
	)
	cmd := &cobra.Command{
		Use:     "expand <summary-id>",
		GroupID: groupRetrieval,
		Short:   "Expand a summary back to its sources (lossless drill-down)",
		Long: "Reverses compaction for a summary. By default it shows the direct sources —\n" +
			"the source messages of a leaf summary, or the child summaries of a condensed\n" +
			"summary. With --messages it walks the entire DAG beneath the summary down to\n" +
			"every verbatim source message, in order. This is the primary lossless-recovery\n" +
			"command the agent runs through its shell tool.",
		Example: `  acm expand sum_1a2b3c
  acm expand sum_1a2b3c --messages
  acm expand sum_1a2b3c --json`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			comp, _, db, err := a.newEngine(ctx, engine.DefaultConfig(), nil)
			if err != nil {
				return err
			}
			defer func() { _ = db.Close() }()

			out := cmd.OutOrStdout()
			if toMessages {
				msgs, mErr := comp.ExpandToMessages(ctx, args[0])
				if mErr != nil {
					return mErr
				}
				return printMessages(out, msgs, asJSON)
			}

			exp, eErr := comp.Expand(ctx, args[0])
			if eErr != nil {
				return eErr
			}
			if asJSON {
				return json.NewEncoder(out).Encode(exp)
			}
			fmt.Fprintf(out, "summary %s (%s depth=%d, covers %d messages)\n",
				exp.Summary.ID, exp.Summary.Kind, exp.Summary.Depth, exp.Summary.DescendantMessageCount)
			for _, m := range exp.Messages {
				fmt.Fprintf(out, "  message %s [seq=%d %s] %s\n", m.ID, m.Seq, m.Role, truncateLine(m.Content, 100))
			}
			for _, ch := range exp.Children {
				fmt.Fprintf(out, "  summary %s [depth=%d covers %d] %s\n", ch.ID, ch.Depth, ch.DescendantMessageCount, truncateLine(ch.Content, 100))
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&toMessages, "messages", false, "walk the DAG down to all verbatim source messages")
	cmd.Flags().BoolVar(&asJSON, "json", false, "emit as JSON")
	return cmd
}

// synthesisSourceCharBudget bounds how much expanded content is handed to the
// model for a synthesized answer (~15K tokens), newest messages preferred.
const synthesisSourceCharBudget = 60_000

// synthesisTimeout bounds a synthesized answer; expansion answers are
// interactive, so allow longer than a compaction summary but never unbounded.
const synthesisTimeout = 120 * time.Second

func newExpandQueryCmd(a *app) *cobra.Command {
	var (
		asJSON         bool
		synthesize     bool
		summarizerName string
		maxTokens      int
	)
	cmd := &cobra.Command{
		Use:     "expand-query <summary-id> <query>",
		GroupID: groupRetrieval,
		Short:   "Expand a summary and return source messages matching a query",
		Long: "Walks the DAG beneath a summary to its verbatim messages and returns only\n" +
			"those whose content contains the query (case-insensitive). Use it for focused\n" +
			"lossless recall when you know what you are looking for inside a compacted span.\n\n" +
			"With --synthesize, the host agent's model (claude or codex, in headless mode)\n" +
			"answers the query directly over the expanded messages, citing the msg_ ids it\n" +
			"drew on — resolve them with 'acm describe'. If the model is unavailable the\n" +
			"command degrades to the plain filtered output.",
		Example: `  acm expand-query sum_1a2b3c "client.go"
  acm expand-query sum_1a2b3c retry backoff --json
  acm expand-query sum_1a2b3c "why did we pick sqlite" --synthesize`,
		Args: cobra.MinimumNArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			comp, _, db, err := a.newEngine(ctx, engine.DefaultConfig(), nil)
			if err != nil {
				return err
			}
			defer func() { _ = db.Close() }()

			query := joinArgs(args[1:])
			msgs, qErr := comp.ExpandQuery(ctx, args[0], query)
			if qErr != nil {
				return qErr
			}
			if !synthesize {
				return printMessages(cmd.OutOrStdout(), msgs, asJSON)
			}

			// Substring filtering can miss semantically relevant messages; for
			// synthesis, an empty match set falls back to the full expansion so
			// the model can still answer.
			if len(msgs) == 0 {
				if msgs, qErr = comp.ExpandToMessages(ctx, args[0]); qErr != nil {
					return qErr
				}
			}
			answer, sErr := synthesizeAnswer(ctx, summarizerName, query, msgs, maxTokens)
			if sErr != nil {
				a.logger.Warn("synthesis unavailable; falling back to filtered output", "error", sErr)
				return printMessages(cmd.OutOrStdout(), msgs, asJSON)
			}
			out := cmd.OutOrStdout()
			fmt.Fprintln(out, answer)
			fmt.Fprintln(out, "---")
			fmt.Fprintln(out, "cited msg_ ids resolve with: acm describe <id>")
			return nil
		},
	}
	cmd.Flags().BoolVar(&asJSON, "json", false, "emit matches as JSON")
	cmd.Flags().BoolVar(&synthesize, "synthesize", false, "answer the query with the host agent's model over the expanded messages")
	cmd.Flags().StringVar(&summarizerName, "summarizer", "claude", "model for --synthesize: claude|codex")
	cmd.Flags().IntVar(&maxTokens, "max-tokens", 2000, "answer size budget for --synthesize")
	return cmd
}

// synthesizeAnswer runs the question over the messages with the named agent
// model. Sources are gathered newest-first against a character budget, then
// restored to chronological order, so the freshest context survives when the
// expansion is large.
func synthesizeAnswer(ctx context.Context, name, question string, msgs []core.Message, maxTokens int) (string, error) {
	summarizer, err := summarizerByName(name)
	if err != nil {
		return "", err
	}
	cliSum, ok := summarizer.(*summarize.CLISummarizer)
	if !ok {
		return "", fmt.Errorf("expand-query: --synthesize requires an LLM summarizer (claude|codex), got %q", name)
	}

	sources := make([]string, 0, len(msgs))
	total := 0
	for _, m := range slices.Backward(msgs) {
		src := fmt.Sprintf("[%s seq=%d %s] %s", m.ID, m.Seq, m.Role, m.Content)
		if total+len(src) > synthesisSourceCharBudget && len(sources) > 0 {
			break
		}
		total += len(src)
		sources = append(sources, src)
	}
	slices.Reverse(sources)

	answerCtx, cancel := context.WithTimeout(ctx, synthesisTimeout)
	defer cancel()
	return cliSum.Answer(answerCtx, question, sources, maxTokens)
}

func printMessages(out io.Writer, msgs []core.Message, asJSON bool) error {
	if asJSON {
		return json.NewEncoder(out).Encode(msgs)
	}
	if len(msgs) == 0 {
		_, err := fmt.Fprintln(out, "no messages")
		return err
	}
	for _, m := range msgs {
		if _, err := fmt.Fprintf(out, "message %s [seq=%d %s]\n%s\n---\n", m.ID, m.Seq, m.Role, m.Content); err != nil {
			return err
		}
	}
	return nil
}
