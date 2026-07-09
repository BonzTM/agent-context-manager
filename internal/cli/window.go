package cli

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/bonztm/agent-context-manager/internal/engine"
)

func newWindowCmd(a *app) *cobra.Command {
	var asJSON bool
	cmd := &cobra.Command{
		Use:     "window <conversation-id>",
		GroupID: groupRetrieval,
		Short:   "Show ACM's assembled context view for a conversation",
		Long: "Renders ACM's persisted, synthetic window for a conversation: a\n" +
			"mix of raw recent messages and summary pointers standing in for compacted\n" +
			"spans, followed by a total item and token count. Before any compaction it is\n" +
			"simply the raw messages in order. Claude Code and Codex do not expose active-\n" +
			"window replacement, so this is diagnostic rather than their live prompt.\n" +
			"Get a conversation id from 'acm grep --json'\n" +
			"or 'acm describe'.",
		Example: `  acm window conv_1a2b3c
  acm window conv_1a2b3c --json`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			comp, _, db, err := a.newEngine(ctx, engine.DefaultConfig(), nil)
			if err != nil {
				return err
			}
			defer func() { _ = db.Close() }()

			items, aErr := comp.Assemble(ctx, args[0])
			if aErr != nil {
				return aErr
			}

			out := cmd.OutOrStdout()
			if asJSON {
				return json.NewEncoder(out).Encode(items)
			}
			total := 0
			for _, it := range items {
				kind := string(it.Role)
				if it.IsSummary {
					kind = fmt.Sprintf("summary d%d", it.Depth)
				}
				fmt.Fprintf(out, "[%-12s tokens=%5d] %s\n", kind, it.Tokens, truncateLine(it.Content, 120))
				total += it.Tokens
			}
			fmt.Fprintf(out, "--- %d items, %d tokens ---\n", len(items), total)
			return nil
		},
	}
	cmd.Flags().BoolVar(&asJSON, "json", false, "emit the window as JSON")
	return cmd
}
