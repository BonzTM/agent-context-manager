package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/bonztm/agent-context-manager/internal/core"
)

const maxLifecycleBatch = 1_000

// ConversationByID loads a conversation by its derived ID.
func (s *SQLite) ConversationByID(ctx context.Context, id string) (core.Conversation, error) {
	conversation, err := scanConversation(s.db.QueryRowContext(ctx,
		"SELECT "+conversationColumns+" FROM conversations WHERE id = ?", id))
	if errors.Is(err, sql.ErrNoRows) {
		return core.Conversation{}, core.ErrNotFound
	}
	if err != nil {
		return core.Conversation{}, fmt.Errorf("store: load conversation by id: %w", err)
	}
	return conversation, nil
}

// SetConversationPin adds or removes a retention pin.
func (s *SQLite) SetConversationPin(ctx context.Context, conversationID string, pinned bool) error {
	if _, err := s.ConversationByID(ctx, conversationID); err != nil {
		return err
	}
	if !pinned {
		_, err := s.db.ExecContext(ctx, "DELETE FROM conversation_pins WHERE conversation_id = ?", conversationID)
		return wrapLifecycleError(err, "remove conversation pin")
	}
	_, err := s.db.ExecContext(ctx, `
INSERT INTO conversation_pins (conversation_id, pinned_at) VALUES (?, ?)
ON CONFLICT (conversation_id) DO NOTHING`, conversationID, fmtTime(s.clock.Now()))
	return wrapLifecycleError(err, "set conversation pin")
}

// MarkSummaryExpanded records a successful lossless traversal.
func (s *SQLite) MarkSummaryExpanded(ctx context.Context, summaryID string) error {
	_, err := s.db.ExecContext(ctx, `
INSERT INTO summary_expansions (summary_id, expanded_at) VALUES (?, ?)
ON CONFLICT (summary_id) DO UPDATE SET expanded_at = excluded.expanded_at`, summaryID, fmtTime(s.clock.Now()))
	return wrapLifecycleError(err, "mark summary expanded")
}

// ListPruneCandidates returns a bounded oldest-first retention plan.
func (s *SQLite) ListPruneCandidates(ctx context.Context, before time.Time, limit int) ([]core.PruneCandidate, error) {
	if limit <= 0 || limit > maxLifecycleBatch {
		return nil, fmt.Errorf("store: prune limit must be between 1 and %d", maxLifecycleBatch)
	}
	const query = `
SELECT c.id, c.agent, c.session_id, c.title, c.created_at, c.updated_at,
       CASE WHEN p.conversation_id IS NULL THEN 0 ELSE 1 END AS pinned,
       COUNT(s.id) AS summary_count,
       COALESCE(SUM(CASE WHEN s.id IS NOT NULL AND e.summary_id IS NULL THEN 1 ELSE 0 END), 0) AS unexpanded
FROM conversations c
LEFT JOIN conversation_pins p ON p.conversation_id = c.id
LEFT JOIN summaries s ON s.conversation_id = c.id
LEFT JOIN summary_expansions e ON e.summary_id = s.id
WHERE c.updated_at < ?
GROUP BY c.id, p.conversation_id
ORDER BY c.updated_at ASC
LIMIT ?`
	rows, err := s.db.QueryContext(ctx, query, fmtTime(before), limit)
	if err != nil {
		return nil, fmt.Errorf("store: list prune candidates: %w", err)
	}
	defer func() { _ = rows.Close() }()
	return scanPruneCandidates(rows)
}

func scanPruneCandidates(rows *sql.Rows) ([]core.PruneCandidate, error) {
	var candidates []core.PruneCandidate
	for rows.Next() {
		var candidate core.PruneCandidate
		var agent, created, updated string
		if err := rows.Scan(&candidate.Conversation.ID, &agent, &candidate.Conversation.SessionID, &candidate.Conversation.Title,
			&created, &updated, &candidate.Pinned, &candidate.SummaryCount, &candidate.UnexpandedSummary); err != nil {
			return nil, fmt.Errorf("store: scan prune candidate: %w", err)
		}
		candidate.Conversation.Agent = core.Agent(agent)
		var err error
		candidate.Conversation.CreatedAt, err = parseTime(created)
		if err != nil {
			return nil, fmt.Errorf("store: parse prune created time: %w", err)
		}
		candidate.Conversation.UpdatedAt, err = parseTime(updated)
		if err != nil {
			return nil, fmt.Errorf("store: parse prune updated time: %w", err)
		}
		candidates = append(candidates, candidate)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("store: iterate prune candidates: %w", err)
	}
	return candidates, nil
}

// ListConversationSummaries returns every summary ordered by deepest level,
// then chronological coverage.
func (s *SQLite) ListConversationSummaries(ctx context.Context, conversationID string) ([]core.Summary, error) {
	rows, err := s.db.QueryContext(ctx, "SELECT "+summaryColumns+" FROM summaries WHERE conversation_id = ? ORDER BY depth DESC, earliest_seq ASC", conversationID)
	if err != nil {
		return nil, fmt.Errorf("store: list conversation summaries: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var summaries []core.Summary
	for rows.Next() {
		summary, sErr := scanSummary(rows)
		if sErr != nil {
			return nil, fmt.Errorf("store: scan conversation summary: %w", sErr)
		}
		summaries = append(summaries, summary)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("store: iterate conversation summaries: %w", err)
	}
	return summaries, nil
}

// DeleteConversations removes a bounded batch atomically and returns offload
// paths for post-commit filesystem cleanup.
func (s *SQLite) DeleteConversations(ctx context.Context, ids []string) (result core.DeleteConversationsResult, err error) {
	if len(ids) == 0 || len(ids) > maxLifecycleBatch {
		return result, fmt.Errorf("store: delete batch must contain 1..%d conversations", maxLifecycleBatch)
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return result, fmt.Errorf("store: begin conversation delete: %w", err)
	}
	defer rollbackOnErr(tx, &err)
	for _, id := range ids {
		paths, deleted, dErr := deleteConversationRows(ctx, tx, id)
		if dErr != nil {
			return result, dErr
		}
		result.OffloadPaths = append(result.OffloadPaths, paths...)
		result.Deleted += deleted
	}
	if err = tx.Commit(); err != nil {
		return result, fmt.Errorf("store: commit conversation delete: %w", err)
	}
	return result, nil
}

func deleteConversationRows(ctx context.Context, tx *sql.Tx, id string) ([]string, int, error) {
	paths, err := selectOffloadPaths(ctx, tx, id)
	if err != nil {
		return nil, 0, err
	}
	if _, err = tx.ExecContext(ctx, "DELETE FROM messages_fts WHERE conversation_id = ?", id); err != nil {
		return nil, 0, fmt.Errorf("store: delete message FTS rows: %w", err)
	}
	if _, err = tx.ExecContext(ctx, "DELETE FROM summaries_fts WHERE conversation_id = ?", id); err != nil {
		return nil, 0, fmt.Errorf("store: delete summary FTS rows: %w", err)
	}
	result, err := tx.ExecContext(ctx, "DELETE FROM conversations WHERE id = ?", id)
	if err != nil {
		return nil, 0, fmt.Errorf("store: delete conversation: %w", err)
	}
	count, err := result.RowsAffected()
	if err != nil {
		return nil, 0, fmt.Errorf("store: count deleted conversation: %w", err)
	}
	return paths, int(count), nil
}

func selectOffloadPaths(ctx context.Context, tx *sql.Tx, conversationID string) ([]string, error) {
	rows, err := tx.QueryContext(ctx, "SELECT storage_uri FROM large_files WHERE conversation_id = ?", conversationID)
	if err != nil {
		return nil, fmt.Errorf("store: list deleted offloads: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var paths []string
	for rows.Next() {
		var path string
		if err := rows.Scan(&path); err != nil {
			return nil, fmt.Errorf("store: scan deleted offload: %w", err)
		}
		paths = append(paths, path)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("store: iterate deleted offloads: %w", err)
	}
	return paths, nil
}

func wrapLifecycleError(err error, operation string) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("store: %s: %w", operation, err)
}
