package cli

import (
	"errors"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/bonztm/agent-context-manager/internal/store"
)

func newDoctorCmd(a *app) *cobra.Command {
	return &cobra.Command{
		Use:     "doctor",
		GroupID: groupDiagnostics,
		Short:   "Open the database, apply migrations, and report health",
		Long: "Opens the resolved database (creating it if necessary), applies any pending\n" +
			"schema migrations, runs an integrity check (PRAGMA integrity_check plus\n" +
			"full-text index row parity), and reports the database path, project root,\n" +
			"and schema version. Use it to verify that acm can reach and initialize a\n" +
			"project's store.",
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
			health, err := store.CheckHealth(ctx, db)
			if err != nil {
				return err
			}

			out := cmd.OutOrStdout()
			fmt.Fprintf(out, "database:       %s\n", a.cfg.DBPath)
			fmt.Fprintf(out, "project root:   %s\n", a.cfg.ProjectRoot)
			fmt.Fprintf(out, "schema version: %d\n", version)
			fmt.Fprintf(out, "integrity:      %s\n", health.Integrity)
			fmt.Fprintf(out, "fts parity:     messages %d/%d, summaries %d/%d\n",
				health.MessageFTSRows, health.MessageRows, health.SummaryFTSRows, health.SummaryRows)
			if health.OK() {
				fmt.Fprintln(out, "status:         ok")
				return nil
			}
			fmt.Fprintln(out, "status:         PROBLEMS FOUND (see above)")
			return errors.New("doctor: database health check failed")
		},
	}
}
