package store

import (
	"context"
	"fmt"
)

// Health is the result of CheckHealth: SQLite's own integrity verdict plus
// row-count parity between the base tables and their FTS indexes (a drifted
// FTS index silently degrades grep and recall).
type Health struct {
	Integrity      string // "ok" or SQLite's first integrity_check finding
	MessageRows    int64
	MessageFTSRows int64
	SummaryRows    int64
	SummaryFTSRows int64
}

// OK reports whether every check passed.
func (h Health) OK() bool {
	return h.Integrity == "ok" &&
		h.MessageRows == h.MessageFTSRows &&
		h.SummaryRows == h.SummaryFTSRows
}

// CheckHealth runs PRAGMA integrity_check and compares base-table row counts
// against their FTS mirrors.
func CheckHealth(ctx context.Context, d *DB) (Health, error) {
	var h Health
	if err := d.sql.QueryRowContext(ctx, "PRAGMA integrity_check(1)").Scan(&h.Integrity); err != nil {
		return Health{}, fmt.Errorf("store: integrity check: %w", err)
	}
	counts := []struct {
		table string
		dest  *int64
	}{
		{"messages", &h.MessageRows},
		{"messages_fts", &h.MessageFTSRows},
		{"summaries", &h.SummaryRows},
		{"summaries_fts", &h.SummaryFTSRows},
	}
	for _, c := range counts {
		if err := d.sql.QueryRowContext(ctx, "SELECT COUNT(*) FROM "+c.table).Scan(c.dest); err != nil {
			return Health{}, fmt.Errorf("store: count %s: %w", c.table, err)
		}
	}
	return h, nil
}
