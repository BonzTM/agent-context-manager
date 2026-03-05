package sqlite

import (
	"context"
	"database/sql"
	"fmt"
)

type migration struct {
	Name string
	SQL  string
}

var migrations = []migration{
	{
		Name: "0001_ctx_foundation.sql",
		SQL: `
CREATE TABLE IF NOT EXISTS ctx_pointers (
	pointer_id INTEGER PRIMARY KEY AUTOINCREMENT,
	project_id TEXT NOT NULL,
	pointer_key TEXT NOT NULL,
	path TEXT NOT NULL,
	anchor TEXT NOT NULL DEFAULT '',
	kind TEXT NOT NULL,
	label TEXT NOT NULL,
	description TEXT NOT NULL,
	tags_json TEXT NOT NULL DEFAULT '[]',
	is_rule INTEGER NOT NULL DEFAULT 0 CHECK (is_rule IN (0, 1)),
	is_stale INTEGER NOT NULL DEFAULT 0 CHECK (is_stale IN (0, 1)),
	stale_at INTEGER NULL,
	content_hash TEXT NULL,
	updated_at INTEGER NOT NULL DEFAULT (unixepoch()),
	UNIQUE (project_id, pointer_key)
);

CREATE INDEX IF NOT EXISTS idx_ctx_pointers_project_updated
	ON ctx_pointers (project_id, updated_at DESC);
CREATE INDEX IF NOT EXISTS idx_ctx_pointers_project_path
	ON ctx_pointers (project_id, path);

CREATE TABLE IF NOT EXISTS ctx_pointer_links (
	project_id TEXT NOT NULL,
	from_key TEXT NOT NULL,
	to_key TEXT NOT NULL,
	created_at INTEGER NOT NULL DEFAULT (unixepoch()),
	PRIMARY KEY (project_id, from_key, to_key)
);

CREATE INDEX IF NOT EXISTS idx_ctx_pointer_links_project_to_key
	ON ctx_pointer_links (project_id, to_key);

CREATE TABLE IF NOT EXISTS ctx_memories (
	memory_id INTEGER PRIMARY KEY AUTOINCREMENT,
	project_id TEXT NOT NULL,
	category TEXT NOT NULL,
	subject TEXT NOT NULL,
	content TEXT NOT NULL,
	confidence INTEGER NOT NULL CHECK (confidence BETWEEN 1 AND 5),
	tags_json TEXT NOT NULL DEFAULT '[]',
	related_pointer_keys_json TEXT NOT NULL DEFAULT '[]',
	evidence_pointer_keys_json TEXT NOT NULL DEFAULT '[]',
	dedupe_key TEXT NULL,
	active INTEGER NOT NULL DEFAULT 1 CHECK (active IN (0, 1)),
	created_at INTEGER NOT NULL DEFAULT (unixepoch()),
	updated_at INTEGER NOT NULL DEFAULT (unixepoch())
);

CREATE INDEX IF NOT EXISTS idx_ctx_memories_project_active
	ON ctx_memories (project_id, active, updated_at DESC);
CREATE UNIQUE INDEX IF NOT EXISTS uq_ctx_memories_project_dedupe_active
	ON ctx_memories (project_id, dedupe_key)
	WHERE active = 1 AND dedupe_key IS NOT NULL;

CREATE TABLE IF NOT EXISTS ctx_receipts (
	receipt_id TEXT PRIMARY KEY,
	project_id TEXT NOT NULL,
	task_text TEXT NOT NULL DEFAULT '',
	phase TEXT NOT NULL DEFAULT 'execute',
	resolved_tags_json TEXT NOT NULL DEFAULT '[]',
	pointer_keys_json TEXT NOT NULL DEFAULT '[]',
	memory_ids_json TEXT NOT NULL DEFAULT '[]',
	summary_json TEXT NOT NULL DEFAULT '{}',
	created_at INTEGER NOT NULL DEFAULT (unixepoch())
);

CREATE INDEX IF NOT EXISTS idx_ctx_receipts_project_created
	ON ctx_receipts (project_id, created_at DESC);

CREATE TABLE IF NOT EXISTS ctx_runs (
	run_id INTEGER PRIMARY KEY AUTOINCREMENT,
	project_id TEXT NOT NULL,
	request_id TEXT NOT NULL DEFAULT '',
	receipt_id TEXT NOT NULL,
	status TEXT NOT NULL,
	files_changed_json TEXT NOT NULL DEFAULT '[]',
	outcome TEXT NOT NULL DEFAULT '',
	summary_json TEXT NOT NULL DEFAULT '{}',
	created_at INTEGER NOT NULL DEFAULT (unixepoch()),
	FOREIGN KEY (receipt_id) REFERENCES ctx_receipts (receipt_id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_ctx_runs_project_created
	ON ctx_runs (project_id, created_at DESC);
`,
	},
	{
		Name: "0002_ctx_propose_memory.sql",
		SQL: `
CREATE TABLE IF NOT EXISTS ctx_memory_candidates (
	candidate_id INTEGER PRIMARY KEY AUTOINCREMENT,
	project_id TEXT NOT NULL,
	receipt_id TEXT NOT NULL,
	category TEXT NOT NULL CHECK (category IN ('decision', 'gotcha', 'pattern', 'preference')),
	subject TEXT NOT NULL,
	content TEXT NOT NULL,
	confidence INTEGER NOT NULL CHECK (confidence BETWEEN 1 AND 5),
	tags_json TEXT NOT NULL DEFAULT '[]',
	related_pointer_keys_json TEXT NOT NULL DEFAULT '[]',
	evidence_pointer_keys_json TEXT NOT NULL DEFAULT '[]',
	dedupe_key TEXT NOT NULL,
	status TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'promoted', 'rejected')),
	promoted_memory_id INTEGER NULL,
	hard_passed INTEGER NOT NULL CHECK (hard_passed IN (0, 1)),
	soft_passed INTEGER NOT NULL CHECK (soft_passed IN (0, 1)),
	validation_errors_json TEXT NOT NULL DEFAULT '[]',
	validation_warnings_json TEXT NOT NULL DEFAULT '[]',
	auto_promote INTEGER NOT NULL DEFAULT 1 CHECK (auto_promote IN (0, 1)),
	promotable INTEGER NOT NULL DEFAULT 0 CHECK (promotable IN (0, 1)),
	created_at INTEGER NOT NULL DEFAULT (unixepoch()),
	updated_at INTEGER NOT NULL DEFAULT (unixepoch()),
	FOREIGN KEY (promoted_memory_id) REFERENCES ctx_memories (memory_id) ON DELETE SET NULL
);

CREATE INDEX IF NOT EXISTS idx_ctx_memory_candidates_project_created
	ON ctx_memory_candidates (project_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_ctx_memory_candidates_project_status_created
	ON ctx_memory_candidates (project_id, status, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_ctx_memory_candidates_receipt_created
	ON ctx_memory_candidates (receipt_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_ctx_memory_candidates_project_dedupe
	ON ctx_memory_candidates (project_id, dedupe_key);
`,
	},
	{
		Name: "0003_ctx_sync.sql",
		SQL: `
CREATE TABLE IF NOT EXISTS ctx_pointer_candidates (
	candidate_id INTEGER PRIMARY KEY AUTOINCREMENT,
	project_id TEXT NOT NULL,
	path TEXT NOT NULL,
	content_hash TEXT NOT NULL,
	created_at INTEGER NOT NULL DEFAULT (unixepoch()),
	updated_at INTEGER NOT NULL DEFAULT (unixepoch()),
	last_seen_at INTEGER NOT NULL DEFAULT (unixepoch()),
	UNIQUE (project_id, path)
);

CREATE INDEX IF NOT EXISTS idx_ctx_pointer_candidates_project_created
	ON ctx_pointer_candidates (project_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_ctx_pointer_candidates_project_updated
	ON ctx_pointer_candidates (project_id, updated_at DESC);
CREATE INDEX IF NOT EXISTS idx_ctx_pointer_candidates_project_hash
	ON ctx_pointer_candidates (project_id, content_hash);
`,
	},
	{
		Name: "0004_ctx_work_items.sql",
		SQL: `
CREATE TABLE IF NOT EXISTS ctx_work_items (
	work_item_id INTEGER PRIMARY KEY AUTOINCREMENT,
	project_id TEXT NOT NULL,
	receipt_id TEXT NOT NULL,
	item_key TEXT NOT NULL,
	status TEXT NOT NULL CHECK (status IN ('pending', 'in_progress', 'blocked', 'completed')),
	created_at INTEGER NOT NULL DEFAULT (unixepoch()),
	updated_at INTEGER NOT NULL DEFAULT (unixepoch()),
	UNIQUE (project_id, receipt_id, item_key),
	FOREIGN KEY (receipt_id) REFERENCES ctx_receipts (receipt_id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_ctx_work_items_project_receipt
	ON ctx_work_items (project_id, receipt_id, item_key);
CREATE INDEX IF NOT EXISTS idx_ctx_work_items_project_receipt_status
	ON ctx_work_items (project_id, receipt_id, status, item_key);
CREATE INDEX IF NOT EXISTS idx_ctx_work_items_project_receipt_updated
	ON ctx_work_items (project_id, receipt_id, updated_at DESC);
`,
	},
}

func applyMigrations(ctx context.Context, db *sql.DB) error {
	if db == nil {
		return fmt.Errorf("sqlite db is required")
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin migrations tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.ExecContext(ctx, `
CREATE TABLE IF NOT EXISTS ctx_schema_migrations (
	migration_name TEXT PRIMARY KEY,
	applied_at INTEGER NOT NULL DEFAULT (unixepoch())
)`); err != nil {
		return fmt.Errorf("ensure schema migrations table: %w", err)
	}

	for _, migration := range migrations {
		var applied int
		if err := tx.QueryRowContext(
			ctx,
			`SELECT COUNT(1) FROM ctx_schema_migrations WHERE migration_name = ?`,
			migration.Name,
		).Scan(&applied); err != nil {
			return fmt.Errorf("check migration %s: %w", migration.Name, err)
		}
		if applied > 0 {
			continue
		}

		if _, err := tx.ExecContext(ctx, migration.SQL); err != nil {
			return fmt.Errorf("apply migration %s: %w", migration.Name, err)
		}
		if _, err := tx.ExecContext(
			ctx,
			`INSERT INTO ctx_schema_migrations (migration_name) VALUES (?)`,
			migration.Name,
		); err != nil {
			return fmt.Errorf("record migration %s: %w", migration.Name, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit migrations tx: %w", err)
	}
	return nil
}
