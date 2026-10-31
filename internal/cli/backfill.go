package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/bonztm/agent-context-manager/internal/agents"
	"github.com/bonztm/agent-context-manager/internal/core"
	"github.com/bonztm/agent-context-manager/internal/store"
)

const (
	defaultBackfillConversations = 1_000
	defaultBackfillTranscripts   = 32
)

type backfillResult struct {
	ConversationID string
	Transcripts    int
	Unavailable    int
	Candidates     int
	Missing        int
	Malformed      int
	Appended       int
}

// transcriptScan aggregates the assistant candidates and diagnostics from one
// conversation's rollouts. Unavailable counts rollouts with no copy on disk.
type transcriptScan struct {
	candidates  []core.IngestMessage
	malformed   int
	unavailable int
}

func newBackfillCmd(a *app) *cobra.Command {
	var apply bool
	var maxConversations, maxTranscripts int
	cmd := &cobra.Command{
		Use:     "backfill [conversation-id...]",
		GroupID: groupDiagnostics,
		Short:   "Preview or reconcile missing Codex assistant turns from rollouts",
		Args:    cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, ids []string) error {
			if maxConversations <= 0 || maxTranscripts <= 0 {
				return errors.New("backfill: limits must be positive")
			}
			return runBackfill(cmd, a, ids, apply, maxConversations, maxTranscripts)
		},
	}
	cmd.Flags().BoolVar(&apply, "apply", false, "persist missing assistant turns (default is dry-run)")
	cmd.Flags().IntVar(&maxConversations, "max-conversations", defaultBackfillConversations, "maximum conversations scanned")
	cmd.Flags().IntVar(&maxTranscripts, "max-transcripts", defaultBackfillTranscripts, "maximum rollout paths per conversation")
	return cmd
}

func runBackfill(cmd *cobra.Command, a *app, ids []string, apply bool, maxConversations, maxTranscripts int) error {
	ctx := cmd.Context()
	sq, db, err := a.newStore(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = db.Close() }()
	conversations, err := selectBackfillConversations(ctx, sq, ids, maxConversations)
	if err != nil {
		return err
	}
	service := a.coreService(sq)
	for _, conversation := range conversations {
		result, bErr := backfillConversation(ctx, service, sq, conversation, apply, maxTranscripts)
		if bErr != nil {
			return bErr
		}
		if _, wErr := fmt.Fprintf(cmd.OutOrStdout(), "%s: transcripts=%d unavailable=%d candidates=%d missing=%d appended=%d malformed=%d mode=%s\n",
			result.ConversationID, result.Transcripts, result.Unavailable, result.Candidates, result.Missing, result.Appended, result.Malformed,
			backfillMode(apply)); wErr != nil {
			return wErr
		}
	}
	return nil
}

func selectBackfillConversations(ctx context.Context, sq *store.SQLite, ids []string, limit int) ([]core.Conversation, error) {
	all, err := sq.ListConversations(ctx, core.AgentCodex)
	if err != nil {
		return nil, err
	}
	if len(ids) == 0 {
		if len(all) > limit {
			return nil, fmt.Errorf("backfill: %d Codex conversations exceed --max-conversations %d", len(all), limit)
		}
		return all, nil
	}
	if len(ids) > limit {
		return nil, fmt.Errorf("backfill: %d requested conversations exceed --max-conversations %d", len(ids), limit)
	}
	return filterConversations(all, ids)
}

func filterConversations(all []core.Conversation, ids []string) ([]core.Conversation, error) {
	byID := make(map[string]core.Conversation, len(all))
	for _, conversation := range all {
		byID[conversation.ID] = conversation
	}
	selected := make([]core.Conversation, 0, len(ids))
	for _, id := range ids {
		conversation, ok := byID[id]
		if !ok {
			return nil, fmt.Errorf("backfill: Codex conversation %s not found", id)
		}
		selected = append(selected, conversation)
	}
	return selected, nil
}

func backfillConversation(ctx context.Context, service *core.Service, sq *store.SQLite, conversation core.Conversation, apply bool, maxTranscripts int) (backfillResult, error) {
	messages, err := sq.ListMessages(ctx, conversation.ID, 0, 0)
	if err != nil {
		return backfillResult{}, err
	}
	paths, err := transcriptPaths(messages, maxTranscripts)
	if err != nil {
		return backfillResult{}, err
	}
	scan, err := scanCodexTranscripts(paths)
	if err != nil {
		return backfillResult{}, err
	}
	missing := missingTranscriptMessages(messages, scan.candidates)
	result := backfillResult{
		ConversationID: conversation.ID, Transcripts: len(paths), Unavailable: scan.unavailable,
		Candidates: len(scan.candidates), Missing: len(missing), Malformed: scan.malformed,
	}
	if !apply || len(missing) == 0 {
		return result, nil
	}
	ingested, err := service.Ingest(ctx, core.IngestRequest{Agent: core.AgentCodex, SessionID: conversation.SessionID, Messages: missing})
	result.Appended = ingested.Appended
	return result, err
}

func transcriptPaths(messages []core.Message, limit int) ([]string, error) {
	seen := make(map[string]bool)
	var paths []string
	for _, message := range messages {
		var payload struct {
			TranscriptPath string `json:"transcript_path"`
		}
		if message.Raw == "" || json.Unmarshal([]byte(message.Raw), &payload) != nil || payload.TranscriptPath == "" || seen[payload.TranscriptPath] {
			continue
		}
		if len(paths) == limit {
			return nil, fmt.Errorf("backfill: conversation %s exceeds --max-transcripts %d", message.ConversationID, limit)
		}
		seen[payload.TranscriptPath] = true
		paths = append(paths, payload.TranscriptPath)
	}
	return paths, nil
}

// scanCodexTranscripts reads each rollout once. A rollout with no copy on disk
// (deleted, or pruned by Codex) is counted as unavailable and skipped, so one
// lost file never aborts the whole backfill; any other read error does.
func scanCodexTranscripts(paths []string) (transcriptScan, error) {
	var scan transcriptScan
	for _, path := range paths {
		messages, report, err := agents.ReconcileTranscriptAssistant(core.AgentCodex, path)
		if errors.Is(err, os.ErrNotExist) {
			scan.unavailable++
			continue
		}
		if err != nil {
			return transcriptScan{}, err
		}
		if report.LimitReached {
			return transcriptScan{}, fmt.Errorf("backfill: transcript %s reached its line limit", path)
		}
		scan.malformed += report.Malformed
		scan.candidates = append(scan.candidates, messages...)
	}
	return scan, nil
}

func missingTranscriptMessages(existing []core.Message, candidates []core.IngestMessage) []core.IngestMessage {
	ids := make(map[string]bool, len(existing))
	for _, message := range existing {
		if message.Role == core.RoleAssistant {
			ids[message.ExternalID] = true
		}
	}
	missing := make([]core.IngestMessage, 0, len(candidates))
	for _, candidate := range candidates {
		if !ids[candidate.ExternalID] {
			ids[candidate.ExternalID] = true
			missing = append(missing, candidate)
		}
	}
	return missing
}

func backfillMode(apply bool) string {
	if apply {
		return "apply"
	}
	return "dry-run"
}
