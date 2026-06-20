package cli

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/spf13/cobra"

	"github.com/bonztm/agent-context-manager/internal/core"
	"github.com/bonztm/agent-context-manager/internal/engine"
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

func newExpandQueryCmd(a *app) *cobra.Command {
	var asJSON bool
	cmd := &cobra.Command{
		Use:     "expand-query <summary-id> <query>",
		GroupID: groupRetrieval,
		Short:   "Expand a summary and return only source messages matching a query",
		Long: "Walks the DAG beneath a summary to its verbatim messages and returns only\n" +
			"those whose content contains the query (case-insensitive). Use it for focused\n" +
			"lossless recall when you know what you are looking for inside a compacted span.",
		Example: `  acm expand-query sum_1a2b3c "client.go"
  acm expand-query sum_1a2b3c retry backoff --json`,
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
			return printMessages(cmd.OutOrStdout(), msgs, asJSON)
		},
	}
	cmd.Flags().BoolVar(&asJSON, "json", false, "emit matches as JSON")
	return cmd
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
