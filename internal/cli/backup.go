package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

func newBackupCmd(a *app) *cobra.Command {
	return &cobra.Command{
		Use:     "backup [destination]",
		GroupID: groupDiagnostics,
		Short:   "Write a consistent snapshot copy of the project database",
		Long: "Copies the live database to a standalone SQLite file using VACUUM INTO,\n" +
			"which is safe against concurrent writers and produces a compact, consistent\n" +
			"snapshot. With no destination it writes\n" +
			"<project>/.acm/backups/acm-<timestamp>.db. The destination must not exist.",
		Example: "  acm backup\n  acm backup /tmp/acm-snapshot.db",
		Args:    cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			db, err := a.openStore(ctx)
			if err != nil {
				return err
			}
			defer func() { _ = db.Close() }()

			var dest string
			if len(args) == 1 {
				dest = args[0]
			} else {
				stamp := clock.Now().UTC().Format("20060102-150405")
				dest = filepath.Join(filepath.Dir(a.cfg.DBPath), "backups", "acm-"+stamp+".db")
			}
			if err := os.MkdirAll(filepath.Dir(dest), 0o750); err != nil {
				return fmt.Errorf("backup: create destination dir: %w", err)
			}
			if _, err := os.Stat(dest); err == nil {
				return fmt.Errorf("backup: destination %s already exists", dest)
			}

			if _, err := db.SQL().ExecContext(ctx, "VACUUM INTO ?", dest); err != nil {
				return fmt.Errorf("backup: vacuum into %s: %w", dest, err)
			}
			if err := os.Chmod(dest, 0o600); err != nil {
				return fmt.Errorf("backup: secure %s: %w", dest, err)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "backup written to %s\n", dest)
			return nil
		},
	}
}
