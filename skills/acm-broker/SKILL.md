---
name: acm-broker
description: Use the agent-context-manager broker (CLI or MCP) to retrieve context receipts, follow hard get_context rule constraints, use code pointers as advisory suggestions, fetch plan artifacts (or receipt shorthand), post work updates/status, propose durable memory, and report completion with deterministic JSON contracts.
---

# acm-broker

Use this skill when a task needs brokered context retrieval, hard rule compliance, plan artifact fetches, work status updates, or durable memory/reporting through `agent-context-manager`.

## Required Flow

1. Call `get_context` first.
2. Read and follow the returned rules block (or rule pointers) as hard constraints.
3. Treat code/doc/test pointers as advisory suggestions for where to start.
4. Call `fetch` for plan/work artifacts needed to execute accurately (or use `receipt_id` shorthand without explicit keys).
5. Execute work; if context is insufficient or stale, refine task text and call `get_context` again.
6. Call `work` with `receipt_id` (optionally without `plan_key`) to publish updates. Use `tasks` payloads and `verify:tests` as the built-in executable verification task key. `verify:diff-review` is optional if the repo wants an explicit manual review task.
7. When code changes are involved, call `verify` before `report_completion`. Include `receipt_id` or `plan_key` when available so `verify` can update `verify:tests`.
8. Call `report_completion` with files changed and outcome after verification is satisfied.
9. Propose durable memory with `propose_memory` when appropriate.

## Interfaces

- These commands assume installed `acm` and `acm-mcp` binaries are available on `PATH`.
- CLI path:
  - `acm validate --in <request.json>`
  - `acm run --in <request.json>`
- MCP path:
  - `acm-mcp invoke --tool get_context --in <payload.json>`
  - `acm-mcp invoke --tool fetch --in <payload.json>`
  - `acm-mcp invoke --tool work --in <payload.json>`
  - `acm-mcp invoke --tool verify --in <payload.json>`
  - `acm-mcp invoke --tool report_completion --in <payload.json>`
  - `acm-mcp invoke --tool propose_memory --in <payload.json>`
  - `acm-mcp invoke --tool eval --in <payload.json>`

Defaults:
- SQLite backend is default when `ACM_PG_DSN` is unset.
- Optional logging controls:
  - `ACM_LOG_LEVEL=debug|info|warn|error`
  - `ACM_LOG_SINK=stderr|stdout|discard`

## Templates

Use templates from `references/templates.md` and `assets/requests/*.json`.

## Rules

- Keep all requests valid `acm.v1` JSON contracts.
- Never skip `get_context` before execution.
- Treat the `get_context` rules block (or rule pointers) as mandatory requirements.
- Treat code pointer paths as advisory guidance, not as mandatory edit boundaries.
- Treat advisory scope as `warn` by default unless an explicit `scope_mode` override is required.
- When `work.tasks` is non-empty, include `verify:tests` for executable verification tracking.
- `verify:diff-review` is optional workflow metadata, not a built-in acm completion gate.
- For code changes, run `verify` before `report_completion` unless the repo rules explicitly allow otherwise.
- For `report_completion`, `scope_mode=strict` blocks on incomplete `verify:tests`; `scope_mode=warn` surfaces warnings.
- If suggested pointers are insufficient, refine/re-run `get_context` before forcing progress.
- Preserve structured JSON output for all broker interactions.
