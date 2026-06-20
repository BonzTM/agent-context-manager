package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newDoctorCmd(a *app) *cobra.Command {
	return &cobra.Command{
		Use:     "doctor",
		GroupID: groupDiagnostics,
		Short:   "Open the database, apply migrations, and report health",
		Long: "Opens the resolved database (creating it if necessary), applies any pending\n" +
			"schema migrations, and reports the database path, project root, and schema\n" +
			"version. Use it to verify that acm can reach and initialize a project's store.",
		Example: "  acm doctor\n  acm --db /path/to/.acm/acm.db doctor",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := cmd.Context()
			db, err := a.openStore(ctx)
			if err != nil {
				return err
			}
			defer func() { _ = db.Close() }()

			version, err := db.SchemaVersion(ctx)
			if err != nil {
				return err
			}

			out := cmd.OutOrStdout()
			fmt.Fprintf(out, "database:       %s\n", a.cfg.DBPath)
			fmt.Fprintf(out, "project root:   %s\n", a.cfg.ProjectRoot)
			fmt.Fprintf(out, "schema version: %d\n", version)
			fmt.Fprintln(out, "status:         ok")
			return nil
		},
	}
}
