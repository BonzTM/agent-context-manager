package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode"

	"github.com/bonztm/agent-context-manager/internal/core"
)

// SQLite implements core.Store on top of an open *DB. Construct it with
// NewSQLite so it shares the single pooled connection opened by Open.
type SQLite struct {
	db    *sql.DB
	clock core.Clock
}

// Compile-time proof that *SQLite satisfies the consumer-defined contract.
var _ core.Store = (*SQLite)(nil)

// NewSQLite wraps an open database with the given clock (used to stamp
// created_at/updated_at). Pass the same clock the service uses.
func NewSQLite(d *DB, clock core.Clock) *SQLite {
	return &SQLite{db: d.SQL(), clock: clock}
}

const (
	messageColumns      = "id, conversation_id, seq, role, content, token_count, tool_name, external_id, identity_hash, raw, created_at"
	conversationColumns = "id, agent, session_id, session_key, title, archived, created_at, updated_at"
)

// rowScanner abstracts *sql.Row and *sql.Rows for the scan helpers.
type rowScanner interface {
	Scan(dest ...any) error
}

// EnsureConversation upserts by (agent, session_id) in a single statement,
// filling empty title/session_key without clobbering set values, then reads the
// row back.
func (s *SQLite) EnsureConversation(ctx context.Context, in core.ConversationInput) (core.Conversation, error) {
	id := core.DeriveConversationID(in.Agent, in.SessionID)
	now := fmtTime(s.clock.Now())

	const q = `
INSERT INTO conversations (id, agent, session_id, session_key, title, archived, created_at, updated_at)
VALUES (?, ?, ?, ?, ?, 0, ?, ?)
ON CONFLICT (agent, session_id) DO UPDATE SET
    session_key = CASE WHEN excluded.session_key <> '' THEN excluded.session_key ELSE conversations.session_key END,
    title       = CASE WHEN excluded.title <> '' THEN excluded.title ELSE conversations.title END,
    updated_at  = excluded.updated_at`
	if _, err := s.db.ExecContext(ctx, q, id, string(in.Agent), in.SessionID, in.SessionKey, in.Title, now, now); err != nil {
		return core.Conversation{}, fmt.Errorf("store: upsert conversation: %w", err)
	}
	return s.ConversationBySession(ctx, in.Agent, in.SessionID)
}

// ConversationBySession loads a conversation by (agent, session_id).
func (s *SQLite) ConversationBySession(ctx context.Context, agent core.Agent, sessionID string) (core.Conversation, error) {
	row := s.db.QueryRowContext(ctx,
		"SELECT "+conversationColumns+" FROM conversations WHERE agent = ? AND session_id = ?",
		string(agent), sessionID)
	conv, err := scanConversation(row)
	if errors.Is(err, sql.ErrNoRows) {
		return core.Conversation{}, core.ErrNotFound
	}
	if err != nil {
		return core.Conversation{}, fmt.Errorf("store: load conversation: %w", err)
	}
	return conv, nil
}

// AppendMessage appends a message inside a transaction (message row + FTS row +
// conversation touch). It is idempotent: a message whose deterministic ID
// already exists is returned with created=false and nothing is written.
func (s *SQLite) AppendMessage(ctx context.Context, in core.MessageInput) (msgOut core.Message, created bool, err error) {
	idHash := core.IdentityHash(in.ExternalID, string(in.Role), in.Content)
	msgID := core.DeriveMessageID(in.ConversationID, idHash)

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return core.Message{}, false, fmt.Errorf("store: begin tx: %w", err)
	}
	// Roll back unless we committed; a rollback after Commit returns ErrTxDone
	// and is expected. Surface an unexpected rollback failure only when the
	// operation otherwise succeeded, so it never masks the real error.
	defer func() {
		if rbErr := tx.Rollback(); rbErr != nil && !errors.Is(rbErr, sql.ErrTxDone) && err == nil {
			err = fmt.Errorf("store: rollback: %w", rbErr)
		}
	}()

	// Idempotency: return the existing row untouched if we have already stored it.
	existing, err := scanMessage(tx.QueryRowContext(ctx,
		"SELECT "+messageColumns+" FROM messages WHERE id = ?", msgID))
	switch {
	case err == nil:
		if cErr := tx.Commit(); cErr != nil {
			return core.Message{}, false, fmt.Errorf("store: commit: %w", cErr)
		}
		return existing, false, nil
	case !errors.Is(err, sql.ErrNoRows):
		return core.Message{}, false, fmt.Errorf("store: probe message: %w", err)
	}

	var seq int64
	if sErr := tx.QueryRowContext(ctx,
		"SELECT COALESCE(MAX(seq), 0) + 1 FROM messages WHERE conversation_id = ?",
		in.ConversationID).Scan(&seq); sErr != nil {
		return core.Message{}, false, fmt.Errorf("store: next seq: %w", sErr)
	}

	now := s.clock.Now().UTC()
	msg := core.Message{
		ID:             msgID,
		ConversationID: in.ConversationID,
		Seq:            seq,
		Role:           in.Role,
		Content:        in.Content,
		TokenCount:     in.TokenCount,
		ToolName:       in.ToolName,
		ExternalID:     in.ExternalID,
		IdentityHash:   idHash,
		Raw:            in.Raw,
		CreatedAt:      now,
	}

	if _, iErr := tx.ExecContext(ctx,
		"INSERT INTO messages ("+messageColumns+") VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)",
		msg.ID, msg.ConversationID, msg.Seq, string(msg.Role), msg.Content, msg.TokenCount,
		msg.ToolName, msg.ExternalID, msg.IdentityHash, msg.Raw, fmtTime(now)); iErr != nil {
		return core.Message{}, false, fmt.Errorf("store: insert message: %w", iErr)
	}
	if _, fErr := tx.ExecContext(ctx,
		"INSERT INTO messages_fts (content, message_id, conversation_id, role) VALUES (?, ?, ?, ?)",
		msg.Content, msg.ID, msg.ConversationID, string(msg.Role)); fErr != nil {
		return core.Message{}, false, fmt.Errorf("store: index message: %w", fErr)
	}
	if _, uErr := tx.ExecContext(ctx,
		"UPDATE conversations SET updated_at = ? WHERE id = ?", fmtTime(now), in.ConversationID); uErr != nil {
		return core.Message{}, false, fmt.Errorf("store: touch conversation: %w", uErr)
	}

	if cErr := tx.Commit(); cErr != nil {
		return core.Message{}, false, fmt.Errorf("store: commit: %w", cErr)
	}
	return msg, true, nil
}

// GetMessage loads a message by ID.
func (s *SQLite) GetMessage(ctx context.Context, id string) (core.Message, error) {
	msg, err := scanMessage(s.db.QueryRowContext(ctx,
		"SELECT "+messageColumns+" FROM messages WHERE id = ?", id))
	if errors.Is(err, sql.ErrNoRows) {
		return core.Message{}, core.ErrNotFound
	}
	if err != nil {
		return core.Message{}, fmt.Errorf("store: get message: %w", err)
	}
	return msg, nil
}

// ListMessages returns messages with seq > afterSeq, ordered by seq.
func (s *SQLite) ListMessages(ctx context.Context, conversationID string, afterSeq int64, limit int) ([]core.Message, error) {
	q := "SELECT " + messageColumns + " FROM messages WHERE conversation_id = ? AND seq > ? ORDER BY seq ASC"
	args := []any{conversationID, afterSeq}
	if limit > 0 {
		q += " LIMIT ?"
		args = append(args, limit)
	}
	rows, err := s.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("store: list messages: %w", err)
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
		return nil, fmt.Errorf("store: iterate messages: %w", err)
	}
	return out, nil
}

// SearchMessages runs an FTS MATCH (default) or a case-insensitive substring
// scan (SearchSubstr) and returns ranked hits.
func (s *SQLite) SearchMessages(ctx context.Context, q core.SearchQuery) ([]core.SearchHit, error) {
	limit := q.Limit
	if limit <= 0 {
		limit = 20
	}
	if q.Mode == core.SearchSubstr {
		return s.searchSubstr(ctx, q, limit)
	}
	return s.searchFTS(ctx, q, limit)
}

func (s *SQLite) searchFTS(ctx context.Context, q core.SearchQuery, limit int) ([]core.SearchHit, error) {
	expr := ftsMatchExpr(q.Text, q.Any)
	if expr == "" {
		return nil, nil
	}
	sql := `
SELECT m.id, m.conversation_id, m.seq, m.role, m.content, m.token_count, m.tool_name,
       m.external_id, m.identity_hash, m.raw, m.created_at,
       snippet(messages_fts, 0, '[', ']', '…', 12) AS snip,
       bm25(messages_fts) AS score
FROM messages_fts
JOIN messages m ON m.id = messages_fts.message_id
WHERE messages_fts MATCH ?`
	args := []any{expr}
	if q.ConversationID != "" {
		sql += " AND m.conversation_id = ?"
		args = append(args, q.ConversationID)
	}
	sql += " ORDER BY score ASC LIMIT ?"
	args = append(args, limit)
	return s.queryHits(ctx, sql, args, true)
}

func (s *SQLite) searchSubstr(ctx context.Context, q core.SearchQuery, limit int) ([]core.SearchHit, error) {
	if q.Text == "" {
		return nil, nil
	}
	sql := `
SELECT m.id, m.conversation_id, m.seq, m.role, m.content, m.token_count, m.tool_name,
       m.external_id, m.identity_hash, m.raw, m.created_at,
       '' AS snip, 0.0 AS score
FROM messages m
WHERE instr(lower(m.content), lower(?)) > 0`
	args := []any{q.Text}
	if q.ConversationID != "" {
		sql += " AND m.conversation_id = ?"
		args = append(args, q.ConversationID)
	}
	sql += " ORDER BY m.conversation_id, m.seq LIMIT ?"
	args = append(args, limit)
	return s.queryHits(ctx, sql, args, false)
}

func (s *SQLite) queryHits(ctx context.Context, query string, args []any, withSnippet bool) ([]core.SearchHit, error) {
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("store: search: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var out []core.SearchHit
	for rows.Next() {
		var (
			m         core.Message
			role      string
			createdAt string
			snip      string
			score     float64
		)
		if sErr := rows.Scan(&m.ID, &m.ConversationID, &m.Seq, &role, &m.Content, &m.TokenCount,
			&m.ToolName, &m.ExternalID, &m.IdentityHash, &m.Raw, &createdAt, &snip, &score); sErr != nil {
			return nil, fmt.Errorf("store: scan hit: %w", sErr)
		}
		m.Role = core.Role(role)
		t, tErr := parseTime(createdAt)
		if tErr != nil {
			return nil, tErr
		}
		m.CreatedAt = t
		if !withSnippet || snip == "" {
			snip = truncate(m.Content, 200)
		}
		out = append(out, core.SearchHit{Message: m, Snippet: snip, Score: score})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("store: iterate hits: %w", err)
	}
	return out, nil
}

// Stats reports aggregate counts.
func (s *SQLite) Stats(ctx context.Context) (core.Stats, error) {
	var st core.Stats
	if err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM conversations").Scan(&st.Conversations); err != nil {
		return core.Stats{}, fmt.Errorf("store: count conversations: %w", err)
	}
	if err := s.db.QueryRowContext(ctx,
		"SELECT COUNT(*), COALESCE(SUM(token_count), 0) FROM messages").Scan(&st.Messages, &st.TotalTokens); err != nil {
		return core.Stats{}, fmt.Errorf("store: count messages: %w", err)
	}
	return st, nil
}

func scanMessage(sc rowScanner) (core.Message, error) {
	var (
		m         core.Message
		role      string
		createdAt string
	)
	if err := sc.Scan(&m.ID, &m.ConversationID, &m.Seq, &role, &m.Content, &m.TokenCount,
		&m.ToolName, &m.ExternalID, &m.IdentityHash, &m.Raw, &createdAt); err != nil {
		return core.Message{}, err
	}
	m.Role = core.Role(role)
	t, err := parseTime(createdAt)
	if err != nil {
		return core.Message{}, err
	}
	m.CreatedAt = t
	return m, nil
}

func scanConversation(sc rowScanner) (core.Conversation, error) {
	var (
		c         core.Conversation
		agent     string
		archived  int64
		createdAt string
		updatedAt string
	)
	if err := sc.Scan(&c.ID, &agent, &c.SessionID, &c.SessionKey, &c.Title, &archived, &createdAt, &updatedAt); err != nil {
		return core.Conversation{}, err
	}
	c.Agent = core.Agent(agent)
	c.Archived = archived != 0
	created, err := parseTime(createdAt)
	if err != nil {
		return core.Conversation{}, err
	}
	updated, err := parseTime(updatedAt)
	if err != nil {
		return core.Conversation{}, err
	}
	c.CreatedAt = created
	c.UpdatedAt = updated
	return c, nil
}

// ftsMatchExpr turns free user text into a safe FTS5 MATCH expression: each
// alphanumeric token is double-quoted (so punctuation can't inject syntax) and
// the tokens are joined with AND (space) or, when any is true, OR. Returns ""
// when there is nothing to match.
func ftsMatchExpr(text string, any bool) string {
	fields := strings.FieldsFunc(text, func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsNumber(r)
	})
	if len(fields) == 0 {
		return ""
	}
	quoted := make([]string, 0, len(fields))
	for _, f := range fields {
		quoted = append(quoted, `"`+f+`"`)
	}
	sep := " "
	if any {
		sep = " OR "
	}
	return strings.Join(quoted, sep)
}

func truncate(s string, max int) string {
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	return string(r[:max]) + "…"
}

func fmtTime(t time.Time) string {
	return t.UTC().Format(time.RFC3339Nano)
}

func parseTime(s string) (time.Time, error) {
	t, err := time.Parse(time.RFC3339Nano, s)
	if err != nil {
		return time.Time{}, fmt.Errorf("store: parse time %q: %w", s, err)
	}
	return t, nil
}
