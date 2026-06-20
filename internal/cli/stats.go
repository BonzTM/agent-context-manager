package cli

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
)

func newStatsCmd(a *app) *cobra.Command {
	var asJSON bool
	cmd := &cobra.Command{
		Use:     "stats",
		GroupID: groupDiagnostics,
		Short:   "Report aggregate counts for the store",
		Long: "Reports the number of conversations and messages stored and the total\n" +
			"estimated token count across all messages. Use --json for machine-readable\n" +
			"output.",
		Example: "  acm stats\n  acm stats --json",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := cmd.Context()
			svc, db, err := a.newService(ctx)
			if err != nil {
				return err
			}
			defer func() { _ = db.Close() }()

			st, err := svc.Stats(ctx)
			if err != nil {
				return err
			}

			out := cmd.OutOrStdout()
			if asJSON {
				return json.NewEncoder(out).Encode(st)
			}
			fmt.Fprintf(out, "conversations: %d\n", st.Conversations)
			fmt.Fprintf(out, "messages:      %d\n", st.Messages)
			fmt.Fprintf(out, "tokens:        %d\n", st.TotalTokens)
			return nil
		},
	}
	cmd.Flags().BoolVar(&asJSON, "json", false, "emit stats as JSON")
	return cmd
}
