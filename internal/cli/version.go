package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/bonztm/agent-context-manager/internal/buildinfo"
)

func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "version",
		GroupID: groupDiagnostics,
		Short:   "Print version information",
		Long:    "Prints the acm version, the git commit it was built from, and the build date.",
		Example: "  acm version",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			_, err := fmt.Fprintf(cmd.OutOrStdout(), "%s %s (commit %s, built %s)\n",
				buildinfo.Name, buildinfo.Version, buildinfo.Commit, buildinfo.Date)
			return err
		},
	}
}
