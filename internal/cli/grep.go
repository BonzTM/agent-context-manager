package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/bonztm/agent-context-manager/internal/core"
)

// grepResult is the --json shape: message hits and summary hits side by side.
type grepResult struct {
	Messages  []core.SearchHit  `json:"messages"`
	Summaries []core.SummaryHit `json:"summaries"`
}

func newGrepCmd(a *app) *cobra.Command {
	var (
		conversation string
		limit        int
		substr       bool
		noSummaries  bool
		asJSON       bool
	)
	cmd := &cobra.Command{
		Use:     "grep <query>",
		GroupID: groupRetrieval,
		Short:   "Search the lossless message history and the summary DAG",
		Long: "Searches stored message content across the whole project history, plus the\n" +
			"summary DAG (so compacted spans are findable too; skip them with\n" +
			"--no-summaries). By default it runs a full-text (FTS5) match ranked by\n" +
			"relevance; --substr does a case-insensitive literal substring scan. Restrict\n" +
			"to one conversation with --conversation. Each result line begins with an id:\n" +
			"pass msg_ ids to 'acm describe' and sum_ ids to 'acm expand'. This is a\n" +
			"drill-down command the agent runs via its shell tool.",
		Example: `  acm grep "exponential backoff"
  acm grep --substr "TODO(" --limit 50
  acm grep auth --conversation conv_1a2b3c --json`,
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			mode := core.SearchFTS
			if substr {
				mode = core.SearchSubstr
			}
			q := core.SearchQuery{
				Text:           strings.Join(args, " "),
				Mode:           mode,
				ConversationID: conversation,
				Limit:          limit,
			}

			ctx := cmd.Context()
			sq, db, err := a.newStore(ctx)
			if err != nil {
				return err
			}
			defer func() { _ = db.Close() }()

			hits, err := sq.SearchMessages(ctx, q)
			if err != nil {
				return err
			}
			var sumHits []core.SummaryHit
			if !noSummaries {
				sumHits, err = sq.SearchSummaries(ctx, q)
				if err != nil {
					return err
				}
			}

			out := cmd.OutOrStdout()
			if asJSON {
				return json.NewEncoder(out).Encode(grepResult{Messages: hits, Summaries: sumHits})
			}
			if len(hits) == 0 && len(sumHits) == 0 {
				fmt.Fprintln(out, "no matches")
				return nil
			}
			for _, h := range hits {
				snippet := strings.ReplaceAll(h.Snippet, "\n", " ")
				fmt.Fprintf(out, "%s  [%s seq=%d %s]  %s\n",
					h.Message.ID, h.Message.ConversationID, h.Message.Seq, h.Message.Role, snippet)
			}
			for _, h := range sumHits {
				snippet := strings.ReplaceAll(h.Snippet, "\n", " ")
				fmt.Fprintf(out, "%s  [%s depth=%d covers seq %d-%d]  %s\n",
					h.Summary.ID, h.Summary.ConversationID, h.Summary.Depth,
					h.Summary.EarliestSeq, h.Summary.LatestSeq, snippet)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&conversation, "conversation", "", "limit search to a conversation ID")
	cmd.Flags().IntVar(&limit, "limit", 20, "maximum number of hits per kind")
	cmd.Flags().BoolVar(&substr, "substr", false, "literal case-insensitive substring search instead of FTS")
	cmd.Flags().BoolVar(&noSummaries, "no-summaries", false, "search messages only, skip the summary DAG")
	cmd.Flags().BoolVar(&asJSON, "json", false, "emit hits as JSON")
	return cmd
}
