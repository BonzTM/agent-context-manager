# ADR-001: Deterministic Context Broker (`ctx`) for LLM Task Context and Memory

> This is an architecture decision record. For getting started, see [getting-started.md](getting-started.md). For terminology, see [concepts.md](concepts.md).

- Status: Proposed
- Date: 2026-03-04
- Owners: Human operator + agent maintainers
- Scope: Project-local LLM context retrieval and durable machine-usable memory

## Context

Current agent startup loads large, mostly task-irrelevant context before any task-specific work starts. This creates context-window overhead, repeated rediscovery, and blind file search. We want a simple, maintainable architecture that:

1. Minimizes always-loaded context.
2. Retrieves only task-relevant files/rules/memories.
3. Enforces behavior outside model compliance.
4. Remains human-reviewable and easy to operate.
5. Avoids vector DB dependencies (Postgres + SQL only).

## Decision

Build a deterministic context broker called `ctx` with one shared core implementation and two front doors:

1. `ctx` CLI (primary interface, lowest operational burden).
2. `ctx` MCP server (thin adapter over same core for tool-native models).

No model runtime gets direct SQL access. Models only interact via broker operations and structured JSON contracts.

## Architecture

### Shared Core

`internal/` provides:

- `core/`: service interface, repository interface, error types, logging decorator.
- `contracts/v1/`: payload types, validation, wire contract.
- `service/postgres/`: retrieval, scoring, sync, health, health_fix, ruleset ingestion.
- `adapters/cli/`: CLI request dispatch.
- `adapters/mcp/`: MCP tool dispatch.
- `adapters/postgres/`, `adapters/sqlite/`: storage backends.

### Interfaces

#### CLI (`cmd/ctx/`)

Convenience subcommands (build v1 envelopes internally):

- `ctx get-context` -> `get_context(task_text, phase, project_id)`
- `ctx fetch` -> `fetch(project_id, keys?, receipt_id?, expected_versions?)`
- `ctx work` -> `work(project_id, receipt_id, plan_key?, items?)`
- `ctx propose-memory` -> `propose_memory(receipt_id, payload_json)`
- `ctx report-completion` -> `report_completion(receipt_id, files_changed, outcome)`
- `ctx sync` -> pointer/hash upkeep from git diff
- `ctx health` / `ctx health-check` -> integrity and drift report
- `ctx health-fix` -> apply safe remediations (sync_working_tree, index_uncovered_files, sync_ruleset)
- `ctx regress` -> retrieval regression suite
- `ctx bootstrap` -> initial pointer candidate generation
- `ctx coverage` -> file coverage analysis

JSON envelope mode is also available via `ctx run --in request.json` and `ctx validate --in request.json`.

#### MCP (`cmd/ctx-mcp/`)

Tools:

- `get_context`
- `fetch`
- `propose_memory`
- `report_completion`
- `work`

The v1.1 MCP contract is index-first: `get_context` returns a scoped receipt index, then `fetch`/`work` operate on those plan and pointer keys while `propose_memory`/`report_completion` stay receipt-scoped.

v1.1 ergonomics defaults:

- Advisory scope mode defaults to `warn` when `scope_mode` is omitted.
- `fetch` accepts `receipt_id` shorthand without explicit `keys`.
- `work` accepts `receipt_id` without `plan_key`; `items` may be empty for status-only reads.
- Work updates should include verification items keyed `verify:tests` and `verify:diff-review` for DoD tracking.
- `get_context` rule entries expose `rule_id`, derived deterministically from the existing stable rule key semantics.

MCP layer is intentionally thin and delegates all business logic to `context-core`.

## Data Model

### 1) `context_pointers`

Purpose: task entry points for code/docs/tests/rules/commands.

```sql
CREATE TABLE context_pointers (
  id              bigserial PRIMARY KEY,
  project_id      text NOT NULL,
  path            text NOT NULL,
  anchor          text,
  pointer_key     text GENERATED ALWAYS AS (
                    project_id || ':' || path || COALESCE('#' || anchor, '')
                  ) STORED,
  kind            text NOT NULL CHECK (kind IN ('code','rule','doc','test','command')),
  label           text NOT NULL,
  description     text NOT NULL,
  tags            text[] NOT NULL DEFAULT '{}',
  relates_to      text[] NOT NULL DEFAULT '{}',
  content_hash    text,
  is_stale        boolean NOT NULL DEFAULT false,
  search_tsv      tsvector GENERATED ALWAYS AS (
                    to_tsvector('english', coalesce(label,'') || ' ' || coalesce(description,''))
                  ) STORED,
  UNIQUE (pointer_key)
);

CREATE INDEX idx_pointers_project ON context_pointers(project_id);
CREATE INDEX idx_pointers_tags ON context_pointers USING gin(tags);
CREATE INDEX idx_pointers_search ON context_pointers USING gin(search_tsv);
CREATE INDEX idx_pointers_kind ON context_pointers(project_id, kind);
```

### 2) `agent_memories`

Purpose: durable, curated facts learned from completed work.

```sql
CREATE TABLE agent_memories (
  id                    bigserial PRIMARY KEY,
  project_id            text NOT NULL,
  category              text NOT NULL CHECK (category IN ('decision','gotcha','pattern','preference')),
  subject               text NOT NULL,
  content               text NOT NULL CHECK (char_length(content) <= 600),
  related_pointer_keys  text[] NOT NULL DEFAULT '{}',
  tags                  text[] NOT NULL DEFAULT '{}',
  confidence            smallint NOT NULL DEFAULT 3 CHECK (confidence BETWEEN 1 AND 5),
  evidence_pointer_keys text[] NOT NULL DEFAULT '{}'
                        CHECK (coalesce(array_length(evidence_pointer_keys, 1), 0) >= 1),
  dedupe_key            text,
  superseded_by         bigint REFERENCES agent_memories(id),
  search_tsv            tsvector GENERATED ALWAYS AS (
                          to_tsvector('english', coalesce(subject,'') || ' ' || coalesce(content,''))
                        ) STORED
);

CREATE UNIQUE INDEX uq_memories_dedupe_key ON agent_memories(project_id, dedupe_key)
WHERE dedupe_key IS NOT NULL;

CREATE INDEX idx_memories_project_active
  ON agent_memories(project_id) WHERE superseded_by IS NULL;
CREATE INDEX idx_memories_tags ON agent_memories USING gin(tags);
CREATE INDEX idx_memories_search ON agent_memories USING gin(search_tsv);
```

### 3) `agent_runs`

Purpose: append-only session/run summaries for human review and "reference last session" context.

```sql
CREATE TABLE agent_runs (
  id                bigserial PRIMARY KEY,
  project_id        text NOT NULL,
  task_text         text NOT NULL,
  phase             text NOT NULL CHECK (phase IN ('plan','execute','review')),
  resolved_tags     text[] NOT NULL,
  receipt           jsonb NOT NULL,
  receipt_id        text NOT NULL,
  retrieval_version text NOT NULL,
  outcome           text,
  files_changed     text[] DEFAULT '{}',
  pointers_updated  text[] DEFAULT '{}',
  memories_proposed int[] DEFAULT '{}',
  created_at        timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX idx_runs_project ON agent_runs(project_id, created_at DESC);
CREATE UNIQUE INDEX uq_runs_receipt_id ON agent_runs(project_id, receipt_id);
```

### 4) `memory_candidates`

Purpose: two-stage memory ingestion and quarantine.

```sql
CREATE TABLE memory_candidates (
  id                    bigserial PRIMARY KEY,
  run_id                bigint REFERENCES agent_runs(id),
  project_id            text NOT NULL,
  category              text NOT NULL CHECK (category IN ('decision','gotcha','pattern','preference')),
  subject               text NOT NULL,
  content               text NOT NULL CHECK (char_length(content) <= 600),
  related_pointer_keys  text[] NOT NULL DEFAULT '{}',
  tags                  text[] NOT NULL DEFAULT '{}',
  confidence            smallint NOT NULL DEFAULT 3 CHECK (confidence BETWEEN 1 AND 5),
  evidence_pointer_keys text[] NOT NULL DEFAULT '{}' CHECK (coalesce(array_length(evidence_pointer_keys,1),0) >= 1),
  status                text NOT NULL DEFAULT 'pending'
                        CHECK (status IN ('pending','promoted','rejected')),
  promoted_to           bigint REFERENCES agent_memories(id),
  rejection_reason      text,
  created_at            timestamptz NOT NULL DEFAULT now()
);
```

## Canonical Tags

Use one human-owned `canonical_tags.json` file (not a DB table). All retrieval and write paths normalize tags through this dictionary.

Current implementation location: `internal/service/postgres/canonical_tags.json`.

Dictionary shape:

```json
{
  "canonical_tags": {
    "backend": ["backend", "api", "server"]
  }
}
```

Rules:
- keys are canonical tag values;
- values are accepted aliases (case-insensitive);
- retrieval normalizes task-derived tags and selected pointer/memory tags through this dictionary;
- `propose_memory` normalizes incoming `memory.tags` through the same dictionary before validation/persistence.

## Retrieval Contract

`get_context(task_text, phase='execute', project_id)`:

1. Normalize task text into 3-6 canonical tags.
2. Query candidates where `(tags overlap OR FTS match)` and `is_stale = false` by default (stale inclusion only when `allow_stale=true`).
3. Score deterministically:
   - `base = tag_overlap_count*10 + ts_rank*5`
   - phase weights:
     - `plan`: rule*3, doc*2, code*1
     - `execute`: code*3, test*2, rule*1
     - `review`: rule*3, test*2, code*1
   - implementation detail: pointer type uses `is_rule`, then `kind` (`doc|docs|documentation`, `test|tests`), with `_test` path fallback for test pointers.
4. Split `rule` vs non-rule.
5. Include all matching rules by default (`caps.max_rule_pointers` can explicitly bound this).
6. Include top non-rules with default cap 8 (`caps.max_non_rule_pointers` override supported).
7. Expand related hops for non-rules up to `caps.max_hops` (default 1), cap `max_hop_expansion` (default +5).
8. Fetch active memories by selected pointer keys and tags, confidence-ranked, cap 6.
9. If fewer than `min_pointer_count` (default 2), widen once to FTS-only by dropping tag overlap while preserving task text and stale policy; if still below threshold, return `insufficient_context`.
10. Return an index-first context receipt with `receipt_id`, `retrieval_version`, indexed artifacts/keys, reasons, and budget accounting.

## Scope Gate Enforcement

`report_completion(receipt_id, files_changed, outcome)` enforces:

1. `files_changed` must be subset of receipt pointer paths.
2. Allow configured generated-file exceptions.
3. Violations default to advisory warnings (`scope_mode=warn`).
4. `scope_mode=strict` can enforce rejection and require re-retrieval; CI repeats strict scope checks and blocks merge on violation.
5. When work items are present, `verify:tests` and `verify:diff-review` are enforced in `strict` mode and surfaced as warnings in `warn` mode.

## Memory Ingestion Contract

`propose_memory(receipt_id, payload_json)`:

1. Validate strict schema.
2. Require non-empty evidence pointers that resolve to real pointers.
3. Enforce canonical tags and confidence range.
4. Compute `dedupe_key` and reject duplicate durable memory.
5. Insert into `memory_candidates`.
6. Promote only when validation policy passes.

## Automation

### `ctx sync --changed`

- Read changed paths from git diff.
- Recompute hashes.
- Mark stale/fresh and upsert pointers.
- Insert candidates for new files by conventions.
- Keep deleted pointers but mark stale.

### `ctx health-check`

Reports:

- stale pointers
- orphan relations
- unknown tags
- duplicate labels by project
- empty descriptions
- weak/unsupported memories
- pending quarantines

### `ctx health_fix apply`

- Apply safe remediations for health findings after canonical rule updates.
- Use when add/remove/update operations leave drift that can be auto-corrected.
- Re-run `ctx health-check` after apply to confirm final state.

### `ctx regress`

- Read `eval_suite.json`.
- Run `get_context` for each test case.
- Score precision/recall/F1.
- Fail CI if aggregate recall below threshold.

## Onboarding Templates

Use these starter examples when bootstrapping downstream repos with canonical rules:

- `docs/examples/canonical-ruleset.yaml`
- `docs/examples/AGENTS.md`
- `docs/examples/CLAUDE.md`

Canonical ruleset files are discovered at `.ctx/canonical-ruleset.yaml` (preferred) or `ctx-rules.yaml` and must declare `version: ctx.rules.v1`.

`CLAUDE.md` should remain a thin companion that maps to `AGENTS.md` as source of truth.
Rule maintenance flow is add/remove/update, then run `sync` or `health_fix apply`, then verify with `health_check`.

## Always-Load Core Packet

Load a small packet (~150 words) for every task:

1. Project identity.
2. Broker operations.
3. Scope and structured I/O rules.
4. Required loop: classify -> retrieve -> execute -> update.
5. `insufficient_context` fallback behavior.
6. Tag dictionary location.

## Consequences

### Benefits

1. Major context-window reduction.
2. Deterministic, auditable retrieval.
3. Model non-compliance is contained by external gates.
4. Strong human operability via logs, health checks, and run summaries.

### Tradeoffs

1. Requires curation quality of pointers/tags.
2. Requires eval suite upkeep.
3. Adds broker tooling surface (CLI + MCP).

## Rollout Plan

1. ~~Implement schema + core + CLI (get_context, propose_memory, report_completion).~~ Done.
2. ~~Add sync, health_check, health_fix, regress, coverage, bootstrap.~~ Done.
3. ~~Add MCP adapter over same core.~~ Done.
4. ~~Add convenience CLI subcommands (flag-based, no JSON construction).~~ Done.
5. ~~Add canonical ruleset ingestion and rule pointer sync.~~ Done.
6. Add CI gates (scope + regression threshold).
7. Pilot in one project, then templatize.

## Rejected Alternatives

1. Full monolithic startup docs on every task: rejected for token cost.
2. Direct SQL by model runtime: rejected for safety/compliance.
3. Vector search dependency: rejected for simplicity and operational scope.
