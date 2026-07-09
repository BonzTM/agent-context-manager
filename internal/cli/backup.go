package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/bonztm/agent-context-manager/internal/store"
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
				stamp := clock.Now().UTC().Format("20060102-150405.000000000")
				dest = filepath.Join(filepath.Dir(a.cfg.DBPath), "backups", "acm-"+stamp+".db")
			}
			if err := backupDatabase(ctx, db, dest); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "backup written to %s\n", dest)
			return nil
		},
	}
}

func backupDatabase(ctx context.Context, db *store.DB, destination string) error {
	if err := os.MkdirAll(filepath.Dir(destination), 0o750); err != nil {
		return fmt.Errorf("backup: create destination dir: %w", err)
	}
	if _, err := os.Stat(destination); err == nil {
		return fmt.Errorf("backup: destination %s already exists", destination)
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("backup: inspect destination: %w", err)
	}
	if _, err := db.SQL().ExecContext(ctx, "VACUUM INTO ?", destination); err != nil {
		return fmt.Errorf("backup: vacuum into %s: %w", destination, err)
	}
	if err := os.Chmod(destination, 0o600); err != nil {
		return fmt.Errorf("backup: secure %s: %w", destination, err)
	}
	return verifyBackup(ctx, destination)
}

func verifyBackup(ctx context.Context, path string) error {
	backup, err := store.Open(ctx, path)
	if err != nil {
		return fmt.Errorf("backup: reopen for verification: %w", err)
	}
	defer func() { _ = backup.Close() }()
	health, err := store.CheckHealth(ctx, backup)
	if err != nil {
		return fmt.Errorf("backup: verify health: %w", err)
	}
	if !health.OK() {
		return fmt.Errorf("backup: verification failed for %s", path)
	}
	return nil
}
