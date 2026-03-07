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
		Name: "0001_acm_foundation.sql",
		SQL: `
CREATE TABLE IF NOT EXISTS acm_pointers (
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

CREATE INDEX IF NOT EXISTS idx_acm_pointers_project_updated
	ON acm_pointers (project_id, updated_at DESC);
CREATE INDEX IF NOT EXISTS idx_acm_pointers_project_path
	ON acm_pointers (project_id, path);

CREATE TABLE IF NOT EXISTS acm_pointer_links (
	project_id TEXT NOT NULL,
	from_key TEXT NOT NULL,
	to_key TEXT NOT NULL,
	created_at INTEGER NOT NULL DEFAULT (unixepoch()),
	PRIMARY KEY (project_id, from_key, to_key),
	FOREIGN KEY (project_id, from_key) REFERENCES acm_pointers (project_id, pointer_key) ON DELETE CASCADE,
	FOREIGN KEY (project_id, to_key) REFERENCES acm_pointers (project_id, pointer_key) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_acm_pointer_links_project_to_key
	ON acm_pointer_links (project_id, to_key);

CREATE TABLE IF NOT EXISTS acm_memories (
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

CREATE INDEX IF NOT EXISTS idx_acm_memories_project_active
	ON acm_memories (project_id, active, updated_at DESC);
CREATE UNIQUE INDEX IF NOT EXISTS uq_acm_memories_project_dedupe_active
	ON acm_memories (project_id, dedupe_key)
	WHERE active = 1 AND dedupe_key IS NOT NULL;

CREATE TABLE IF NOT EXISTS acm_receipts (
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

CREATE INDEX IF NOT EXISTS idx_acm_receipts_project_created
	ON acm_receipts (project_id, created_at DESC);

CREATE TABLE IF NOT EXISTS acm_runs (
	run_id INTEGER PRIMARY KEY AUTOINCREMENT,
	project_id TEXT NOT NULL,
	request_id TEXT NOT NULL DEFAULT '',
	receipt_id TEXT NOT NULL,
	status TEXT NOT NULL,
	files_changed_json TEXT NOT NULL DEFAULT '[]',
	outcome TEXT NOT NULL DEFAULT '',
	summary_json TEXT NOT NULL DEFAULT '{}',
	created_at INTEGER NOT NULL DEFAULT (unixepoch()),
	FOREIGN KEY (receipt_id) REFERENCES acm_receipts (receipt_id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_acm_runs_project_created
	ON acm_runs (project_id, created_at DESC);
`,
	},
	{
		Name: "0002_acm_propose_memory.sql",
		SQL: `
CREATE TABLE IF NOT EXISTS acm_memory_candidates (
	candidate_id INTEGER PRIMARY KEY AUTOINCREMENT,
	project_id TEXT NOT NULL,
	receipt_id TEXT NOT NULL,
	category TEXT NOT NULL CHECK (category IN ('decision', 'gotcha', 'pattern', 'preference')),
	subject TEXT NOT NULL,
	content TEXT NOT NULL,
	confidence INTEGER NOT NULL CHECK (confidence BETWEEN 1 AND 5),
	tags_json TEXT NOT NULL DEFAULT '[]',
	related_pointer_keys_json TEXT NOT NULL DEFAULT '[]',
	evidence_pointer_keys_json TEXT NOT NULL CHECK (
		json_valid(evidence_pointer_keys_json)
		AND json_type(evidence_pointer_keys_json) = 'array'
		AND json_array_length(evidence_pointer_keys_json) >= 1
	),
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
	FOREIGN KEY (promoted_memory_id) REFERENCES acm_memories (memory_id) ON DELETE SET NULL
);

CREATE INDEX IF NOT EXISTS idx_acm_memory_candidates_project_created
	ON acm_memory_candidates (project_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_acm_memory_candidates_project_status_created
	ON acm_memory_candidates (project_id, status, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_acm_memory_candidates_receipt_created
	ON acm_memory_candidates (receipt_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_acm_memory_candidates_project_dedupe
	ON acm_memory_candidates (project_id, dedupe_key);
`,
	},
	{
		Name: "0003_acm_sync.sql",
		SQL: `
CREATE TABLE IF NOT EXISTS acm_pointer_candidates (
	candidate_id INTEGER PRIMARY KEY AUTOINCREMENT,
	project_id TEXT NOT NULL,
	path TEXT NOT NULL,
	content_hash TEXT NOT NULL,
	created_at INTEGER NOT NULL DEFAULT (unixepoch()),
	updated_at INTEGER NOT NULL DEFAULT (unixepoch()),
	last_seen_at INTEGER NOT NULL DEFAULT (unixepoch()),
	UNIQUE (project_id, path)
);

CREATE INDEX IF NOT EXISTS idx_acm_pointer_candidates_project_created
	ON acm_pointer_candidates (project_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_acm_pointer_candidates_project_updated
	ON acm_pointer_candidates (project_id, updated_at DESC);
CREATE INDEX IF NOT EXISTS idx_acm_pointer_candidates_project_hash
	ON acm_pointer_candidates (project_id, content_hash);
`,
	},
	{
		Name: "0004_acm_work_items.sql",
		SQL: `
CREATE TABLE IF NOT EXISTS acm_work_items (
	work_item_id INTEGER PRIMARY KEY AUTOINCREMENT,
	project_id TEXT NOT NULL,
	receipt_id TEXT NOT NULL,
	item_key TEXT NOT NULL,
	status TEXT NOT NULL CHECK (status IN ('pending', 'in_progress', 'blocked', 'completed')),
	created_at INTEGER NOT NULL DEFAULT (unixepoch()),
	updated_at INTEGER NOT NULL DEFAULT (unixepoch()),
	UNIQUE (project_id, receipt_id, item_key),
	FOREIGN KEY (receipt_id) REFERENCES acm_receipts (receipt_id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_acm_work_items_project_receipt
	ON acm_work_items (project_id, receipt_id, item_key);
CREATE INDEX IF NOT EXISTS idx_acm_work_items_project_receipt_status
	ON acm_work_items (project_id, receipt_id, status, item_key);
CREATE INDEX IF NOT EXISTS idx_acm_work_items_project_receipt_updated
	ON acm_work_items (project_id, receipt_id, updated_at DESC);
`,
	},
	{
		Name: "0005_acm_work_plans.sql",
		SQL: `
CREATE TABLE IF NOT EXISTS acm_work_plans (
	plan_id INTEGER PRIMARY KEY AUTOINCREMENT,
	project_id TEXT NOT NULL,
	plan_key TEXT NOT NULL,
	receipt_id TEXT NULL,
	title TEXT NOT NULL DEFAULT '',
	objective TEXT NOT NULL DEFAULT '',
	status TEXT NOT NULL CHECK (status IN ('pending', 'in_progress', 'blocked', 'completed')),
	stage_spec_outline TEXT NOT NULL DEFAULT 'pending' CHECK (stage_spec_outline IN ('pending', 'in_progress', 'blocked', 'completed')),
	stage_refined_spec TEXT NOT NULL DEFAULT 'pending' CHECK (stage_refined_spec IN ('pending', 'in_progress', 'blocked', 'completed')),
	stage_implementation_plan TEXT NOT NULL DEFAULT 'pending' CHECK (stage_implementation_plan IN ('pending', 'in_progress', 'blocked', 'completed')),
	in_scope_json TEXT NOT NULL DEFAULT '[]',
	out_of_scope_json TEXT NOT NULL DEFAULT '[]',
	constraints_json TEXT NOT NULL DEFAULT '[]',
	references_json TEXT NOT NULL DEFAULT '[]',
	created_at INTEGER NOT NULL DEFAULT (unixepoch()),
	updated_at INTEGER NOT NULL DEFAULT (unixepoch()),
	UNIQUE (project_id, plan_key)
);

CREATE INDEX IF NOT EXISTS idx_acm_work_plans_project_status_updated
	ON acm_work_plans (project_id, status, updated_at DESC);
CREATE INDEX IF NOT EXISTS idx_acm_work_plans_project_receipt_updated
	ON acm_work_plans (project_id, receipt_id, updated_at DESC);

CREATE TABLE IF NOT EXISTS acm_work_plan_tasks (
	task_id INTEGER PRIMARY KEY AUTOINCREMENT,
	project_id TEXT NOT NULL,
	plan_key TEXT NOT NULL,
	task_key TEXT NOT NULL,
	summary TEXT NOT NULL,
	status TEXT NOT NULL CHECK (status IN ('pending', 'in_progress', 'blocked', 'completed')),
	depends_on_json TEXT NOT NULL DEFAULT '[]',
	acceptance_criteria_json TEXT NOT NULL DEFAULT '[]',
	references_json TEXT NOT NULL DEFAULT '[]',
	blocked_reason TEXT NOT NULL DEFAULT '',
	outcome TEXT NOT NULL DEFAULT '',
	evidence_json TEXT NOT NULL DEFAULT '[]',
	created_at INTEGER NOT NULL DEFAULT (unixepoch()),
	updated_at INTEGER NOT NULL DEFAULT (unixepoch()),
	UNIQUE (project_id, plan_key, task_key),
	FOREIGN KEY (project_id, plan_key) REFERENCES acm_work_plans (project_id, plan_key) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_acm_work_plan_tasks_project_plan_status
	ON acm_work_plan_tasks (project_id, plan_key, status, task_key);
CREATE INDEX IF NOT EXISTS idx_acm_work_plan_tasks_project_plan_updated
	ON acm_work_plan_tasks (project_id, plan_key, updated_at DESC);
`,
	},
	{
		Name: "0006_acm_work_plan_hierarchy.sql",
		SQL: `
ALTER TABLE acm_work_plans
	ADD COLUMN kind TEXT NOT NULL DEFAULT '';
ALTER TABLE acm_work_plans
	ADD COLUMN parent_plan_key TEXT NOT NULL DEFAULT '';
ALTER TABLE acm_work_plans
	ADD COLUMN external_refs_json TEXT NOT NULL DEFAULT '[]';

CREATE INDEX IF NOT EXISTS idx_acm_work_plans_project_parent_updated
	ON acm_work_plans (project_id, parent_plan_key, updated_at DESC);

ALTER TABLE acm_work_plan_tasks
	ADD COLUMN parent_task_key TEXT NOT NULL DEFAULT '';
ALTER TABLE acm_work_plan_tasks
	ADD COLUMN external_refs_json TEXT NOT NULL DEFAULT '[]';

CREATE INDEX IF NOT EXISTS idx_acm_work_plan_tasks_project_plan_parent
	ON acm_work_plan_tasks (project_id, plan_key, parent_task_key, task_key);
`,
	},
	{
		Name: "0007_acm_verification_runs.sql",
		SQL: `
CREATE TABLE IF NOT EXISTS acm_verification_batches (
	batch_run_id TEXT PRIMARY KEY,
	project_id TEXT NOT NULL,
	receipt_id TEXT NOT NULL DEFAULT '',
	plan_key TEXT NOT NULL DEFAULT '',
	phase TEXT NOT NULL DEFAULT '',
	tests_source_path TEXT NOT NULL DEFAULT '',
	status TEXT NOT NULL CHECK (status IN ('passed', 'failed')),
	passed INTEGER NOT NULL DEFAULT 0 CHECK (passed IN (0, 1)),
	selected_test_ids_json TEXT NOT NULL DEFAULT '[]',
	created_at INTEGER NOT NULL DEFAULT (unixepoch())
);

CREATE INDEX IF NOT EXISTS idx_acm_verification_batches_project_created
	ON acm_verification_batches (project_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_acm_verification_batches_project_receipt_created
	ON acm_verification_batches (project_id, receipt_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_acm_verification_batches_project_plan_created
	ON acm_verification_batches (project_id, plan_key, created_at DESC);

CREATE TABLE IF NOT EXISTS acm_verification_results (
	result_id INTEGER PRIMARY KEY AUTOINCREMENT,
	batch_run_id TEXT NOT NULL,
	project_id TEXT NOT NULL,
	test_id TEXT NOT NULL,
	definition_hash TEXT NOT NULL,
	summary TEXT NOT NULL DEFAULT '',
	command_argv_json TEXT NOT NULL DEFAULT '[]',
	command_cwd TEXT NOT NULL DEFAULT '.',
	timeout_sec INTEGER NOT NULL DEFAULT 300,
	expected_exit_code INTEGER NOT NULL DEFAULT 0,
	selection_reasons_json TEXT NOT NULL DEFAULT '[]',
	status TEXT NOT NULL CHECK (status IN ('passed', 'failed', 'timed_out', 'errored', 'skipped')),
	exit_code INTEGER NULL,
	duration_ms INTEGER NOT NULL DEFAULT 0,
	stdout_excerpt TEXT NOT NULL DEFAULT '',
	stderr_excerpt TEXT NOT NULL DEFAULT '',
	started_at INTEGER NOT NULL DEFAULT (unixepoch()),
	finished_at INTEGER NOT NULL DEFAULT (unixepoch()),
	FOREIGN KEY (batch_run_id) REFERENCES acm_verification_batches (batch_run_id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_acm_verification_results_batch_started
	ON acm_verification_results (batch_run_id, started_at, result_id);
CREATE INDEX IF NOT EXISTS idx_acm_verification_results_project_test_started
	ON acm_verification_results (project_id, test_id, started_at DESC);
`,
	},
	{
		Name: "0008_acm_sqlite_parity.sql",
		SQL: `
CREATE TABLE IF NOT EXISTS acm_pointer_links_new (
	project_id TEXT NOT NULL,
	from_key TEXT NOT NULL,
	to_key TEXT NOT NULL,
	created_at INTEGER NOT NULL DEFAULT (unixepoch()),
	PRIMARY KEY (project_id, from_key, to_key),
	FOREIGN KEY (project_id, from_key) REFERENCES acm_pointers (project_id, pointer_key) ON DELETE CASCADE,
	FOREIGN KEY (project_id, to_key) REFERENCES acm_pointers (project_id, pointer_key) ON DELETE CASCADE
);

INSERT INTO acm_pointer_links_new (project_id, from_key, to_key, created_at)
SELECT l.project_id, l.from_key, l.to_key, l.created_at
FROM acm_pointer_links l
WHERE EXISTS (
	SELECT 1 FROM acm_pointers p
	WHERE p.project_id = l.project_id AND p.pointer_key = l.from_key
)
AND EXISTS (
	SELECT 1 FROM acm_pointers p
	WHERE p.project_id = l.project_id AND p.pointer_key = l.to_key
);

DROP TABLE acm_pointer_links;
ALTER TABLE acm_pointer_links_new RENAME TO acm_pointer_links;

CREATE INDEX IF NOT EXISTS idx_acm_pointer_links_project_to_key
	ON acm_pointer_links (project_id, to_key);

CREATE TABLE IF NOT EXISTS acm_memory_candidates_new (
	candidate_id INTEGER PRIMARY KEY AUTOINCREMENT,
	project_id TEXT NOT NULL,
	receipt_id TEXT NOT NULL,
	category TEXT NOT NULL CHECK (category IN ('decision', 'gotcha', 'pattern', 'preference')),
	subject TEXT NOT NULL,
	content TEXT NOT NULL,
	confidence INTEGER NOT NULL CHECK (confidence BETWEEN 1 AND 5),
	tags_json TEXT NOT NULL DEFAULT '[]',
	related_pointer_keys_json TEXT NOT NULL DEFAULT '[]',
	evidence_pointer_keys_json TEXT NOT NULL CHECK (
		json_valid(evidence_pointer_keys_json)
		AND json_type(evidence_pointer_keys_json) = 'array'
		AND json_array_length(evidence_pointer_keys_json) >= 1
	),
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
	FOREIGN KEY (promoted_memory_id) REFERENCES acm_memories (memory_id) ON DELETE SET NULL
);

INSERT INTO acm_memory_candidates_new (
	candidate_id,
	project_id,
	receipt_id,
	category,
	subject,
	content,
	confidence,
	tags_json,
	related_pointer_keys_json,
	evidence_pointer_keys_json,
	dedupe_key,
	status,
	promoted_memory_id,
	hard_passed,
	soft_passed,
	validation_errors_json,
	validation_warnings_json,
	auto_promote,
	promotable,
	created_at,
	updated_at
)
SELECT
	candidate_id,
	project_id,
	receipt_id,
	category,
	subject,
	content,
	confidence,
	tags_json,
	related_pointer_keys_json,
	evidence_pointer_keys_json,
	dedupe_key,
	status,
	promoted_memory_id,
	hard_passed,
	soft_passed,
	validation_errors_json,
	validation_warnings_json,
	auto_promote,
	promotable,
	created_at,
	updated_at
FROM acm_memory_candidates
WHERE json_valid(evidence_pointer_keys_json)
  AND json_type(evidence_pointer_keys_json) = 'array'
  AND json_array_length(evidence_pointer_keys_json) >= 1;

DROP TABLE acm_memory_candidates;
ALTER TABLE acm_memory_candidates_new RENAME TO acm_memory_candidates;

CREATE INDEX IF NOT EXISTS idx_acm_memory_candidates_project_created
	ON acm_memory_candidates (project_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_acm_memory_candidates_project_status_created
	ON acm_memory_candidates (project_id, status, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_acm_memory_candidates_receipt_created
	ON acm_memory_candidates (receipt_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_acm_memory_candidates_project_dedupe
	ON acm_memory_candidates (project_id, dedupe_key);
`,
	},
	{
		Name: "0009_acm_run_history_indexes.sql",
		SQL: `
CREATE INDEX IF NOT EXISTS idx_acm_runs_project_receipt_created
	ON acm_runs (project_id, receipt_id, created_at DESC, run_id DESC);
`,
	},
	{
		Name: "0010_acm_review_attempts.sql",
		SQL: `
CREATE TABLE IF NOT EXISTS acm_review_attempts (
	attempt_id INTEGER PRIMARY KEY AUTOINCREMENT,
	project_id TEXT NOT NULL,
	receipt_id TEXT NOT NULL,
	plan_key TEXT NOT NULL DEFAULT '',
	review_key TEXT NOT NULL,
	summary TEXT NOT NULL DEFAULT '',
	fingerprint TEXT NOT NULL,
	status TEXT NOT NULL,
	passed INTEGER NOT NULL DEFAULT 0,
	outcome TEXT NOT NULL DEFAULT '',
	workflow_source_path TEXT NOT NULL DEFAULT '',
	command_argv_json TEXT NOT NULL DEFAULT '[]',
	command_cwd TEXT NOT NULL DEFAULT '',
	timeout_sec INTEGER NOT NULL DEFAULT 0,
	exit_code INTEGER NULL,
	timed_out INTEGER NOT NULL DEFAULT 0,
	stdout_excerpt TEXT NOT NULL DEFAULT '',
	stderr_excerpt TEXT NOT NULL DEFAULT '',
	created_at INTEGER NOT NULL DEFAULT (unixepoch()),
	FOREIGN KEY (receipt_id) REFERENCES acm_receipts (receipt_id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_acm_review_attempts_project_receipt_key_created
	ON acm_review_attempts (project_id, receipt_id, review_key, created_at DESC, attempt_id DESC);

CREATE INDEX IF NOT EXISTS idx_acm_review_attempts_project_receipt_fingerprint
	ON acm_review_attempts (project_id, receipt_id, review_key, fingerprint);
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
CREATE TABLE IF NOT EXISTS acm_schema_migrations (
	migration_name TEXT PRIMARY KEY,
	applied_at INTEGER NOT NULL DEFAULT (unixepoch())
)`); err != nil {
		return fmt.Errorf("ensure schema migrations table: %w", err)
	}

	for _, migration := range migrations {
		var applied int
		if err := tx.QueryRowContext(
			ctx,
			`SELECT COUNT(1) FROM acm_schema_migrations WHERE migration_name = ?`,
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
			`INSERT INTO acm_schema_migrations (migration_name) VALUES (?)`,
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
