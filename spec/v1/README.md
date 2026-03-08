# acm v1 Schemas

This directory defines the v1 wire contract for the context broker, including the preferred diagnostics surface `status`.

## Files

- `shared.schema.json`: reusable types used by CLI and MCP contracts.
- `cli.command.schema.json`: input envelope for `acm` CLI JSON mode.
- `cli.result.schema.json`: output envelope for `acm` CLI JSON mode.
- `mcp.tools.v1.json`: MCP tool contracts with exact input/output schema refs.

## CLI Contract

`acm` in JSON mode should accept a request matching `cli.command.schema.json` and emit a response matching `cli.result.schema.json`.
For project-scoped commands, `project_id` may be omitted when runtime defaults are configured. Resolution order is explicit `--project` / payload `project_id`, then `ACM_PROJECT_ID`, then inferred effective repo root name.

## MCP Contract

The MCP adapter exposes fifteen tools — all CLI operations are available via MCP:

Agent-facing:

1. `get_context`
2. `fetch`
3. `propose_memory`
4. `report_completion`
5. `review`
6. `work`
7. `history_search`

Maintenance:

8. `sync`
9. `health_check`
10. `health_fix`
11. `status`
12. `coverage`
13. `eval`
14. `verify`
15. `bootstrap`

Tool input/output shapes are referenced from CLI payload/result defs to guarantee parity.

The MCP flow is index-first:

- `get_context` returns an index-first receipt with scoped rules, suggestions, memories, and current plans.
- Each rule entry now includes `rule_id`, a deterministic stable identifier derived from the existing rule `key` semantics (no additional input required).
- `fetch` resolves receipt/plan-scoped artifacts by key, or derives the plan fetch key from `receipt_id` when keys are omitted.
- `review` remains work-backed and defaults to `review:cross-llm` with `complete` status when callers omit manual fields; it can also execute a workflow-defined `run` command before recording the final review task status.
- `work` creates/updates structured plans with tasks (max 256 per request). Supports `receipt_id` without `plan_key` (derives `plan_key` as `plan:<receipt_id>`). `mode` controls merge vs replace semantics.
- `history_search` lists or searches compact work, memory, receipt, and run history and returns targeted `fetch_keys` for selective follow-up retrieval. `entity` defaults to `all`; `scope` and `kind` are only valid when `entity=work`.
- For work updates, `verify:tests` is the built-in executable verification key. `verify:diff-review` is optional workflow metadata.
- `eval` is the public retrieval-evaluation command/tool name. `verify` selects repo-defined executable checks from `.acm/acm-tests.yaml` or `acm-tests.yaml`, with `tests_file` as the explicit override.
- `status` reports the active project/backend/runtime snapshot, loaded rules/tags/tests/workflows, installed bootstrap integrations, and optionally a `get_context`-style retrieval preview when callers provide `task_text`.
- `bootstrap` accepts repeatable `apply_templates` ids and reports per-template `template_results`. Template application is additive-only: create missing files, upgrade ACM-owned pristine scaffolds, and merge known additive JSON fragments without overwriting edited repo files.
- `report_completion` can enforce repo-defined completion task keys from `.acm/acm-workflows.yaml` or `acm-workflows.yaml`; runnable review gates may also require a fresh passing attempt for the current scoped fingerprint when fingerprint dedupe is enabled. When no workflow gates are configured, acm falls back to `verify:tests`.
- `propose_memory` and `report_completion` remain receipt-scoped write operations.

`get_context.caps.word_budget_limit` defaults to `1200` and is reported as accounting metadata in `_meta.budget`; it is not a truncation cutoff. `report_completion.scope_mode` defaults to `warn` when omitted. When work tasks are present, `scope_mode=strict` enforces configured completion checks and `scope_mode=warn` surfaces warnings.

## Notes

- These schemas are framework-agnostic and can be implemented in Go, TypeScript, Rust, or Python.
- Recommended implementation approach for Go:
  - define structs from schema
  - validate with a JSON Schema library at ingress/egress
  - keep retrieval logic in a shared core package used by both CLI and MCP adapters.
