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
6. Call `work` with `receipt_id` (optionally without `plan_key`) to publish updates, or send zero `items` for status-only retrieval. When posting updates, include `verify:tests` and `verify:diff-review` work items.
7. Call `report_completion` with files changed and outcome.
8. Propose durable memory with `propose_memory` when appropriate.

## Interfaces

- CLI path:
  - `go run ./cmd/acm validate --in <request.json>`
  - `go run ./cmd/acm run --in <request.json>`
- MCP path:
  - `go run ./cmd/acm-mcp invoke --tool get_context --in <payload.json>`
  - `go run ./cmd/acm-mcp invoke --tool fetch --in <payload.json>`
  - `go run ./cmd/acm-mcp invoke --tool work --in <payload.json>`
  - `go run ./cmd/acm-mcp invoke --tool report_completion --in <payload.json>`
  - `go run ./cmd/acm-mcp invoke --tool propose_memory --in <payload.json>`

Defaults:
- SQLite backend is default when `CTX_PG_DSN` is unset.
- Optional logging controls:
  - `CTX_LOG_LEVEL=debug|info|warn|error`
  - `CTX_LOG_SINK=stderr|stdout|discard`

## Templates

Use templates from `references/templates.md` and `assets/requests/*.json`.

## Rules

- Keep all requests valid `ctx.v1` JSON contracts.
- Never skip `get_context` before execution.
- Treat the `get_context` rules block (or rule pointers) as mandatory requirements.
- Treat code pointer paths as advisory guidance, not as mandatory edit boundaries.
- Treat advisory scope as `warn` by default unless an explicit `scope_mode` override is required.
- When `work.items` is non-empty, include `verify:tests` and `verify:diff-review` quality-gate items.
- For those verification items, use mode-aware enforcement: `scope_mode=strict` is blocking, `scope_mode=warn` surfaces warnings.
- If suggested pointers are insufficient, refine/re-run `get_context` before forcing progress.
- Preserve structured JSON output for all broker interactions.
