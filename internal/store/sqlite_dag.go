package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/bonztm/agent-context-manager/internal/core"
)

const summaryColumns = "id, conversation_id, kind, depth, content, token_count, source_count, descendant_message_count, earliest_seq, latest_seq, created_at"

// rollbackOnErr is the canonical deferred-rollback helper. A rollback after a
// successful Commit returns ErrTxDone and is expected; any other rollback
// failure is surfaced only when the operation otherwise succeeded so it never
// masks the real error. Use as: defer rollbackOnErr(tx, &err) with a named err.
func rollbackOnErr(tx *sql.Tx, err *error) {
	if rbErr := tx.Rollback(); rbErr != nil && !errors.Is(rbErr, sql.ErrTxDone) && *err == nil {
		*err = fmt.Errorf("store: rollback: %w", rbErr)
	}
}

// CreateLeafSummary inserts a leaf summary over the given messages, with its
// lossless message pointers and FTS row, in one transaction. It is idempotent:
// an identical summary (same derived ID) is returned untouched.
func (s *SQLite) CreateLeafSummary(ctx context.Context, in core.LeafSummaryInput) (out core.Summary, err error) {
	id := core.DeriveSummaryID(in.ConversationID, core.SummaryLeaf, 0, in.SourceMessageIDs)

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return core.Summary{}, fmt.Errorf("store: begin tx: %w", err)
	}
	defer rollbackOnErr(tx, &err)

	existing, gErr := scanSummary(tx.QueryRowContext(ctx, "SELECT "+summaryColumns+" FROM summaries WHERE id = ?", id))
	switch {
	case gErr == nil:
		if cErr := tx.Commit(); cErr != nil {
			return core.Summary{}, fmt.Errorf("store: commit: %w", cErr)
		}
		return existing, nil
	case !errors.Is(gErr, sql.ErrNoRows):
		return core.Summary{}, fmt.Errorf("store: probe summary: %w", gErr)
	}

	now := s.clock.Now().UTC()
	sum := core.Summary{
		ID:                     id,
		ConversationID:         in.ConversationID,
		Kind:                   core.SummaryLeaf,
		Depth:                  0,
		Content:                in.Content,
		TokenCount:             in.TokenCount,
		SourceCount:            len(in.SourceMessageIDs),
		DescendantMessageCount: in.DescendantMessageCount,
		EarliestSeq:            in.EarliestSeq,
		LatestSeq:              in.LatestSeq,
		CreatedAt:              now,
	}
	if insErr := insertSummary(ctx, tx, sum); insErr != nil {
		return core.Summary{}, insErr
	}
	for _, mid := range in.SourceMessageIDs {
		if _, lErr := tx.ExecContext(ctx,
			"INSERT OR IGNORE INTO summary_messages (summary_id, message_id) VALUES (?, ?)", id, mid); lErr != nil {
			return core.Summary{}, fmt.Errorf("store: link message: %w", lErr)
		}
	}
	if cErr := tx.Commit(); cErr != nil {
		return core.Summary{}, fmt.Errorf("store: commit: %w", cErr)
	}
	return sum, nil
}

// CreateCondensedSummary inserts a condensed summary over child summaries, with
// its DAG edges and FTS row, in one transaction. It is idempotent.
func (s *SQLite) CreateCondensedSummary(ctx context.Context, in core.CondensedSummaryInput) (out core.Summary, err error) {
	id := core.DeriveSummaryID(in.ConversationID, core.SummaryCondensed, in.Depth, in.ChildSummaryIDs)

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return core.Summary{}, fmt.Errorf("store: begin tx: %w", err)
	}
	defer rollbackOnErr(tx, &err)

	existing, gErr := scanSummary(tx.QueryRowContext(ctx, "SELECT "+summaryColumns+" FROM summaries WHERE id = ?", id))
	switch {
	case gErr == nil:
		if cErr := tx.Commit(); cErr != nil {
			return core.Summary{}, fmt.Errorf("store: commit: %w", cErr)
		}
		return existing, nil
	case !errors.Is(gErr, sql.ErrNoRows):
		return core.Summary{}, fmt.Errorf("store: probe summary: %w", gErr)
	}

	now := s.clock.Now().UTC()
	sum := core.Summary{
		ID:                     id,
		ConversationID:         in.ConversationID,
		Kind:                   core.SummaryCondensed,
		Depth:                  in.Depth,
		Content:                in.Content,
		TokenCount:             in.TokenCount,
		SourceCount:            len(in.ChildSummaryIDs),
		DescendantMessageCount: in.DescendantMessageCount,
		EarliestSeq:            in.EarliestSeq,
		LatestSeq:              in.LatestSeq,
		CreatedAt:              now,
	}
	if insErr := insertSummary(ctx, tx, sum); insErr != nil {
		return core.Summary{}, insErr
	}
	for _, cid := range in.ChildSummaryIDs {
		if _, lErr := tx.ExecContext(ctx,
			"INSERT OR IGNORE INTO summary_parents (parent_id, child_id) VALUES (?, ?)", id, cid); lErr != nil {
			return core.Summary{}, fmt.Errorf("store: link child summary: %w", lErr)
		}
	}
	if cErr := tx.Commit(); cErr != nil {
		return core.Summary{}, fmt.Errorf("store: commit: %w", cErr)
	}
	return sum, nil
}

func insertSummary(ctx context.Context, tx *sql.Tx, sum core.Summary) error {
	if _, err := tx.ExecContext(ctx,
		"INSERT INTO summaries ("+summaryColumns+") VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)",
		sum.ID, sum.ConversationID, string(sum.Kind), sum.Depth, sum.Content, sum.TokenCount,
		sum.SourceCount, sum.DescendantMessageCount, sum.EarliestSeq, sum.LatestSeq, fmtTime(sum.CreatedAt)); err != nil {
		return fmt.Errorf("store: insert summary: %w", err)
	}
	if _, err := tx.ExecContext(ctx,
		"INSERT INTO summaries_fts (content, summary_id, conversation_id, depth) VALUES (?, ?, ?, ?)",
		sum.Content, sum.ID, sum.ConversationID, sum.Depth); err != nil {
		return fmt.Errorf("store: index summary: %w", err)
	}
	return nil
}

// ListConversationIDs returns all conversation IDs, oldest first.
func (s *SQLite) ListConversationIDs(ctx context.Context) ([]string, error) {
	rows, err := s.db.QueryContext(ctx, "SELECT id FROM conversations ORDER BY created_at ASC")
	if err != nil {
		return nil, fmt.Errorf("store: list conversations: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var out []string
	for rows.Next() {
		var id string
		if sErr := rows.Scan(&id); sErr != nil {
			return nil, fmt.Errorf("store: scan conversation id: %w", sErr)
		}
		out = append(out, id)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("store: iterate conversations: %w", err)
	}
	return out, nil
}

// SearchSummaries runs an FTS MATCH over summary content and returns ranked
// hits, mirroring SearchMessages for the DAG so grep covers compacted spans
// too. Substring mode falls back to a case-insensitive scan.
func (s *SQLite) SearchSummaries(ctx context.Context, q core.SearchQuery) ([]core.SummaryHit, error) {
	limit := q.Limit
	if limit <= 0 {
		limit = 20
	}

	var query string
	args := make([]any, 0, 3)
	if q.Mode == core.SearchSubstr {
		if q.Text == "" {
			return nil, nil
		}
		query = "SELECT " + prefixedSummaryColumns("s") + `, '' AS snip, 0.0 AS score,
       EXISTS (SELECT 1 FROM context_items ci WHERE ci.item_type = 'summary' AND ci.ref_id = s.id) AS active
FROM summaries s
WHERE instr(lower(s.content), lower(?)) > 0`
		args = append(args, q.Text)
		if q.ConversationID != "" {
			query += " AND s.conversation_id = ?"
			args = append(args, q.ConversationID)
		}
		query += " ORDER BY s.conversation_id, s.earliest_seq LIMIT ?"
	} else {
		expr := ftsMatchExpr(q.Text, q.Any)
		if expr == "" {
			return nil, nil
		}
		query = "SELECT " + prefixedSummaryColumns("s") + `,
       snippet(summaries_fts, 0, '[', ']', '…', 12) AS snip,
       bm25(summaries_fts) AS score,
       EXISTS (SELECT 1 FROM context_items ci WHERE ci.item_type = 'summary' AND ci.ref_id = s.id) AS active
FROM summaries_fts
JOIN summaries s ON s.id = summaries_fts.summary_id
WHERE summaries_fts MATCH ?`
		args = append(args, expr)
		if q.ConversationID != "" {
			query += " AND s.conversation_id = ?"
			args = append(args, q.ConversationID)
		}
		query += " ORDER BY score ASC LIMIT ?"
	}
	args = append(args, limit)

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("store: search summaries: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var out []core.SummaryHit
	for rows.Next() {
		var (
			sum       core.Summary
			kind      string
			createdAt string
			snip      string
			score     float64
			active    bool
		)
		if sErr := rows.Scan(&sum.ID, &sum.ConversationID, &kind, &sum.Depth, &sum.Content, &sum.TokenCount,
			&sum.SourceCount, &sum.DescendantMessageCount, &sum.EarliestSeq, &sum.LatestSeq, &createdAt,
			&snip, &score, &active); sErr != nil {
			return nil, fmt.Errorf("store: scan summary hit: %w", sErr)
		}
		sum.Kind = core.SummaryKind(kind)
		t, tErr := parseTime(createdAt)
		if tErr != nil {
			return nil, tErr
		}
		sum.CreatedAt = t
		if snip == "" {
			snip = truncate(sum.Content, 200)
		}
		out = append(out, core.SummaryHit{Summary: sum, Snippet: snip, Score: score, Active: active})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("store: iterate summary hits: %w", err)
	}
	return out, nil
}

// GetSummary loads a summary by ID.
func (s *SQLite) GetSummary(ctx context.Context, id string) (core.Summary, error) {
	sum, err := scanSummary(s.db.QueryRowContext(ctx, "SELECT "+summaryColumns+" FROM summaries WHERE id = ?", id))
	if errors.Is(err, sql.ErrNoRows) {
		return core.Summary{}, core.ErrNotFound
	}
	if err != nil {
		return core.Summary{}, fmt.Errorf("store: get summary: %w", err)
	}
	return sum, nil
}

// SummaryMessages returns the source messages of a leaf summary, ordered by seq.
func (s *SQLite) SummaryMessages(ctx context.Context, summaryID string) ([]core.Message, error) {
	rows, err := s.db.QueryContext(ctx,
		//nolint:gosec // G202: concatenates only the fixed column list through the alias helper; every value is parameterized.
		"SELECT "+prefixedMessageColumns("m")+" FROM summary_messages sm JOIN messages m ON m.id = sm.message_id WHERE sm.summary_id = ? ORDER BY m.seq ASC",
		summaryID)
	if err != nil {
		return nil, fmt.Errorf("store: summary messages: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var out []core.Message
	for rows.Next() {
		m, sErr := scanMessage(rows)
		if sErr != nil {
			return nil, fmt.Errorf("store: scan message: %w", sErr)
		}
		out = append(out, m)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("store: iterate summary messages: %w", err)
	}
	return out, nil
}

// SummaryChildren returns the child summaries of a condensed summary, ordered by
// their earliest covered position.
func (s *SQLite) SummaryChildren(ctx context.Context, summaryID string) ([]core.Summary, error) {
	rows, err := s.db.QueryContext(ctx,
		//nolint:gosec // G202: concatenates only the fixed column list through the alias helper; every value is parameterized.
		"SELECT "+prefixedSummaryColumns("s")+" FROM summary_parents sp JOIN summaries s ON s.id = sp.child_id WHERE sp.parent_id = ? ORDER BY s.earliest_seq ASC",
		summaryID)
	if err != nil {
		return nil, fmt.Errorf("store: summary children: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var out []core.Summary
	for rows.Next() {
		sum, sErr := scanSummary(rows)
		if sErr != nil {
			return nil, fmt.Errorf("store: scan summary: %w", sErr)
		}
		out = append(out, sum)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("store: iterate children: %w", err)
	}
	return out, nil
}

// ListContextItems returns the conversation's active window in order.
func (s *SQLite) ListContextItems(ctx context.Context, conversationID string) ([]core.ContextItem, error) {
	rows, err := s.db.QueryContext(ctx,
		"SELECT ordinal, item_type, ref_id FROM context_items WHERE conversation_id = ? ORDER BY ordinal ASC", conversationID)
	if err != nil {
		return nil, fmt.Errorf("store: list context items: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var out []core.ContextItem
	for rows.Next() {
		var (
			it      core.ContextItem
			typeStr string
		)
		if sErr := rows.Scan(&it.Ordinal, &typeStr, &it.RefID); sErr != nil {
			return nil, fmt.Errorf("store: scan context item: %w", sErr)
		}
		it.Type = core.ContextItemType(typeStr)
		out = append(out, it)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("store: iterate context items: %w", err)
	}
	return out, nil
}

// ReplaceContextItems atomically replaces a conversation's active window.
func (s *SQLite) ReplaceContextItems(ctx context.Context, conversationID string, items []core.ContextItem) (err error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("store: begin tx: %w", err)
	}
	defer rollbackOnErr(tx, &err)

	if _, dErr := tx.ExecContext(ctx, "DELETE FROM context_items WHERE conversation_id = ?", conversationID); dErr != nil {
		return fmt.Errorf("store: clear context items: %w", dErr)
	}
	for i, it := range items {
		if _, iErr := tx.ExecContext(ctx,
			"INSERT INTO context_items (conversation_id, ordinal, item_type, ref_id) VALUES (?, ?, ?, ?)",
			conversationID, i, string(it.Type), it.RefID); iErr != nil {
			return fmt.Errorf("store: insert context item: %w", iErr)
		}
	}
	if cErr := tx.Commit(); cErr != nil {
		return fmt.Errorf("store: commit: %w", cErr)
	}
	return nil
}

// CreateLargeFile records an offloaded large file. It is idempotent on the
// derived ID; an existing row keeps its original created_at, which is what the
// returned value reports.
func (s *SQLite) CreateLargeFile(ctx context.Context, lf core.LargeFile) (core.LargeFile, error) {
	now := s.clock.Now().UTC()
	if lf.ID == "" {
		lf.ID = core.DeriveFileID(lf.ConversationID, lf.MessageID)
	}
	var createdAt string
	if err := s.db.QueryRowContext(ctx,
		`INSERT INTO large_files (id, conversation_id, message_id, storage_uri, byte_size, token_count, exploration_summary, extractor, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		 ON CONFLICT (id) DO UPDATE SET
		   storage_uri = excluded.storage_uri,
		   byte_size = excluded.byte_size,
		   token_count = excluded.token_count,
		   exploration_summary = excluded.exploration_summary,
		   extractor = excluded.extractor
		 RETURNING created_at`,
		lf.ID, lf.ConversationID, lf.MessageID, lf.StorageURI, lf.ByteSize, lf.TokenCount, lf.ExplorationSummary, lf.Extractor, fmtTime(now)).
		Scan(&createdAt); err != nil {
		return core.LargeFile{}, fmt.Errorf("store: upsert large file: %w", err)
	}
	t, err := parseTime(createdAt)
	if err != nil {
		return core.LargeFile{}, err
	}
	lf.CreatedAt = t
	return lf, nil
}

// GetLargeFile loads an offloaded large file by ID.
func (s *SQLite) GetLargeFile(ctx context.Context, id string) (core.LargeFile, error) {
	var (
		lf        core.LargeFile
		createdAt string
	)
	err := s.db.QueryRowContext(ctx,
		"SELECT id, conversation_id, message_id, storage_uri, byte_size, token_count, exploration_summary, extractor, created_at FROM large_files WHERE id = ?", id).
		Scan(&lf.ID, &lf.ConversationID, &lf.MessageID, &lf.StorageURI, &lf.ByteSize, &lf.TokenCount, &lf.ExplorationSummary, &lf.Extractor, &createdAt)
	if errors.Is(err, sql.ErrNoRows) {
		return core.LargeFile{}, core.ErrNotFound
	}
	if err != nil {
		return core.LargeFile{}, fmt.Errorf("store: get large file: %w", err)
	}
	t, err := parseTime(createdAt)
	if err != nil {
		return core.LargeFile{}, err
	}
	lf.CreatedAt = t
	return lf, nil
}

func scanSummary(sc rowScanner) (core.Summary, error) {
	var (
		sum       core.Summary
		kind      string
		createdAt string
	)
	if err := sc.Scan(&sum.ID, &sum.ConversationID, &kind, &sum.Depth, &sum.Content, &sum.TokenCount,
		&sum.SourceCount, &sum.DescendantMessageCount, &sum.EarliestSeq, &sum.LatestSeq, &createdAt); err != nil {
		return core.Summary{}, err
	}
	sum.Kind = core.SummaryKind(kind)
	t, err := parseTime(createdAt)
	if err != nil {
		return core.Summary{}, err
	}
	sum.CreatedAt = t
	return sum, nil
}

// prefixedMessageColumns / prefixedSummaryColumns render the column lists with a
// table alias so they can be selected from a JOIN.
func prefixedMessageColumns(alias string) string {
	return aliasColumns(alias, messageColumns)
}

func prefixedSummaryColumns(alias string) string {
	return aliasColumns(alias, summaryColumns)
}

func aliasColumns(alias, columns string) string {
	parts := strings.Split(columns, ", ")
	for i, p := range parts {
		parts[i] = alias + "." + p
	}
	return strings.Join(parts, ", ")
}
