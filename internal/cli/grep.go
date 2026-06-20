package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/bonztm/agent-context-manager/internal/core"
)

func newGrepCmd(a *app) *cobra.Command {
	var (
		conversation string
		limit        int
		substr       bool
		asJSON       bool
	)
	cmd := &cobra.Command{
		Use:     "grep <query>",
		GroupID: groupRetrieval,
		Short:   "Search the lossless message history",
		Long: "Searches stored message content across the whole project history. By default\n" +
			"it runs a full-text (FTS5) match ranked by relevance; --substr does a\n" +
			"case-insensitive literal substring scan. Restrict to one conversation with\n" +
			"--conversation. Each result line begins with the message id, which you can pass\n" +
			"to 'acm describe'. This is a drill-down command the agent runs via its shell\n" +
			"tool.",
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
			svc, db, err := a.newService(ctx)
			if err != nil {
				return err
			}
			defer func() { _ = db.Close() }()

			hits, err := svc.Search(ctx, q)
			if err != nil {
				return err
			}

			out := cmd.OutOrStdout()
			if asJSON {
				return json.NewEncoder(out).Encode(hits)
			}
			if len(hits) == 0 {
				fmt.Fprintln(out, "no matches")
				return nil
			}
			for _, h := range hits {
				snippet := strings.ReplaceAll(h.Snippet, "\n", " ")
				fmt.Fprintf(out, "%s  [%s seq=%d %s]  %s\n",
					h.Message.ID, h.Message.ConversationID, h.Message.Seq, h.Message.Role, snippet)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&conversation, "conversation", "", "limit search to a conversation ID")
	cmd.Flags().IntVar(&limit, "limit", 20, "maximum number of hits")
	cmd.Flags().BoolVar(&substr, "substr", false, "literal case-insensitive substring search instead of FTS")
	cmd.Flags().BoolVar(&asJSON, "json", false, "emit hits as JSON")
	return cmd
}
