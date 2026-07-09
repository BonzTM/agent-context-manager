package cli

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"

	"github.com/bonztm/agent-context-manager/internal/core"
)

const defaultPruneLimit = 1_000

func newPruneCmd(a *app) *cobra.Command {
	var (
		olderThan time.Duration
		apply     bool
		force     bool
		backup    string
		limit     int
	)
	cmd := &cobra.Command{
		Use:     "prune",
		GroupID: groupCompaction,
		Short:   "Preview or delete old conversations with backup-first safeguards",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if olderThan <= 0 || limit <= 0 || limit > defaultPruneLimit {
				return errors.New("prune: --older-than must be positive and --max-conversations must be 1..1000")
			}
			return runPrune(cmd, a, olderThan, apply, force, backup, limit)
		},
	}
	cmd.Flags().DurationVar(&olderThan, "older-than", 30*24*time.Hour, "select conversations inactive for this duration")
	cmd.Flags().BoolVar(&apply, "apply", false, "create a verified backup and delete eligible conversations")
	cmd.Flags().BoolVar(&force, "force", false, "allow deletion when summaries have never been expanded")
	cmd.Flags().StringVar(&backup, "backup", "", "pre-delete backup path (default: project backups directory)")
	cmd.Flags().IntVar(&limit, "max-conversations", defaultPruneLimit, "maximum conversations considered")
	return cmd
}

func runPrune(cmd *cobra.Command, a *app, olderThan time.Duration, apply, force bool, backupPath string, limit int) error {
	ctx := cmd.Context()
	sq, db, err := a.newStore(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = db.Close() }()
	candidates, err := sq.ListPruneCandidates(ctx, clock.Now().Add(-olderThan), limit)
	if err != nil {
		return err
	}
	ids := printPrunePlan(cmd, candidates, force)
	if !apply || len(ids) == 0 {
		fmt.Fprintf(cmd.OutOrStdout(), "mode=%s eligible=%d considered=%d\n", pruneMode(apply), len(ids), len(candidates))
		return nil
	}
	if backupPath == "" {
		stamp := clock.Now().UTC().Format("20060102-150405.000000000")
		backupPath = filepath.Join(filepath.Dir(a.cfg.DBPath), "backups", "pre-prune-"+stamp+".db")
	}
	if bErr := backupDatabase(ctx, db, backupPath); bErr != nil {
		return bErr
	}
	result, err := sq.DeleteConversations(ctx, ids)
	if err != nil {
		return err
	}
	if err := removeOffloads(result.OffloadPaths); err != nil {
		return fmt.Errorf("prune: database committed but offload cleanup failed; restore %s if rollback is required: %w", backupPath, err)
	}
	fmt.Fprintf(cmd.OutOrStdout(), "backup=%s deleted=%d offloads=%d mode=apply\n", backupPath, result.Deleted, len(result.OffloadPaths))
	return nil
}

func printPrunePlan(cmd *cobra.Command, candidates []core.PruneCandidate, force bool) []string {
	var ids []string
	for _, candidate := range candidates {
		status := "eligible"
		switch {
		case candidate.Pinned:
			status = "pinned"
		case candidate.UnexpandedSummary > 0 && !force:
			status = "unexpanded"
		default:
			ids = append(ids, candidate.Conversation.ID)
		}
		fmt.Fprintf(cmd.OutOrStdout(), "%s updated=%s summaries=%d unexpanded=%d status=%s\n",
			candidate.Conversation.ID, candidate.Conversation.UpdatedAt.UTC().Format(time.RFC3339),
			candidate.SummaryCount, candidate.UnexpandedSummary, status)
	}
	return ids
}

func removeOffloads(paths []string) error {
	for _, path := range paths {
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("remove %s: %w", path, err)
		}
	}
	return nil
}

func pruneMode(apply bool) string {
	if apply {
		return "apply"
	}
	return "dry-run"
}
