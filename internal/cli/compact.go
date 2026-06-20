package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/bonztm/agent-context-manager/internal/engine"
)

func newCompactCmd(a *app) *cobra.Command {
	cfg := engine.DefaultConfig()
	var summarizerName string
	cmd := &cobra.Command{
		Use:     "compact [conversation-id...]",
		GroupID: groupCompaction,
		Short:   "Compact conversations into the summary DAG under the token budget",
		Long: "Runs the compaction loop: folds the oldest spans (outside the protected fresh\n" +
			"tail) into leaf and condensed summaries until the active window is under the\n" +
			"soft token threshold. The verbatim originals are preserved and remain\n" +
			"recoverable with 'acm expand'. With no arguments, every conversation is\n" +
			"compacted; otherwise only the given conversation ids.\n\n" +
			"Summarizers:\n" +
			"  deterministic  (default) structural, offline, no model call\n" +
			"  claude|codex   reuse the host agent's own model in headless mode\n" +
			"                 (falls back to deterministic on any error)\n\n" +
			"The budget defaults target a ~200K-token model window; tune with the flags\n" +
			"below. For real compression the leaf chunk size should exceed the summary\n" +
			"target so many input tokens fold into a smaller summary.",
		Example: `  acm compact
  acm compact conv_1a2b3c --summarizer claude
  acm compact --model-context-tokens 1000000 --soft-fraction 0.5`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			summarizer, sErr := summarizerByName(summarizerName)
			if sErr != nil {
				return sErr
			}
			comp, sq, db, err := a.newEngine(ctx, cfg, summarizer)
			if err != nil {
				return err
			}
			defer func() { _ = db.Close() }()

			ids := args
			if len(ids) == 0 {
				ids, err = sq.ListConversationIDs(ctx)
				if err != nil {
					return err
				}
			}

			out := cmd.OutOrStdout()
			if len(ids) == 0 {
				fmt.Fprintln(out, "no conversations to compact")
				return nil
			}
			for _, id := range ids {
				res, cErr := comp.Compact(ctx, id)
				if cErr != nil {
					return cErr
				}
				fmt.Fprintf(out, "%s: %d -> %d tokens (leaves +%d, condensed +%d)\n",
					id, res.BeforeTokens, res.AfterTokens, res.Leaves, res.Condensed)
			}
			return nil
		},
	}
	cmd.Flags().IntVar(&cfg.ModelContextTokens, "model-context-tokens", cfg.ModelContextTokens, "the host model's context window in tokens")
	cmd.Flags().Float64Var(&cfg.SoftFraction, "soft-fraction", cfg.SoftFraction, "compact when the window exceeds this fraction of the model window")
	cmd.Flags().IntVar(&cfg.FreshTailMessages, "fresh-tail", cfg.FreshTailMessages, "most recent messages always kept raw")
	cmd.Flags().IntVar(&cfg.LeafChunkTokens, "leaf-chunk-tokens", cfg.LeafChunkTokens, "max source tokens folded into one leaf summary")
	cmd.Flags().StringVar(&summarizerName, "summarizer", "deterministic", "summarizer: deterministic|claude|codex")
	return cmd
}
