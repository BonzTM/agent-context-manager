package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newPinCmd(a *app) *cobra.Command {
	return &cobra.Command{
		Use:     "pin <conversation-id>",
		GroupID: groupCompaction,
		Short:   "Protect a conversation from retention pruning",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			sq, db, err := a.newStore(ctx)
			if err != nil {
				return err
			}
			defer func() { _ = db.Close() }()
			if pErr := sq.SetConversationPin(ctx, args[0], true); pErr != nil {
				return pErr
			}
			_, err = fmt.Fprintf(cmd.OutOrStdout(), "pinned %s\n", args[0])
			return err
		},
	}
}
