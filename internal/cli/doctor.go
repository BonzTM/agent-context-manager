package cli

import (
	"errors"
	"fmt"
	"io"
	"os"
	"time"

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
			sq, db, err := a.newStore(ctx)
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
			home, err := os.UserHomeDir()
			if err != nil {
				return fmt.Errorf("doctor: resolve home: %w", err)
			}
			integration, err := checkIntegrations(ctx, sq, a.cfg.DBPath, home)
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
			printIntegrationReport(out, integration)
			if health.OK() && len(integration.Findings) == 0 {
				fmt.Fprintln(out, "status:         ok")
				return nil
			}
			fmt.Fprintln(out, "status:         PROBLEMS FOUND (see above)")
			return errors.New("doctor: health check failed")
		},
	}
}

func printIntegrationReport(out io.Writer, report integrationReport) {
	if !report.CodexDetected {
		fmt.Fprintln(out, "codex:          not detected")
		return
	}
	fmt.Fprintf(out, "codex acm:      executable=%s latest_capture=%s\n", valueOr(report.Executable, "unresolved"), formatCaptureTime(report.LatestCapture))
	for _, gap := range report.RoleGaps {
		fmt.Fprintln(out, "role gap:       "+gap)
	}
	for _, finding := range report.Findings {
		fmt.Fprintln(out, "finding:        "+finding)
	}
}

func formatCaptureTime(value time.Time) string {
	if value.IsZero() {
		return "none"
	}
	return value.UTC().Format(time.RFC3339)
}

func valueOr(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}
