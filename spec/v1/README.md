# acm v1 Schemas

This directory defines the v1 wire contract for the context broker.

## Files

- `shared.schema.json`: reusable types used by CLI and MCP contracts.
- `cli.command.schema.json`: input envelope for `acm` CLI JSON mode.
- `cli.result.schema.json`: output envelope for `acm` CLI JSON mode.
- `mcp.tools.v1.json`: MCP tool contracts with exact input/output schema refs.

## CLI Contract

`acm` in JSON mode should accept a request matching `cli.command.schema.json` and emit a response matching `cli.result.schema.json`.

## MCP Contract

The MCP adapter exposes thirteen tools — all CLI operations are available via MCP:

Agent-facing:

1. `get_context`
2. `fetch`
3. `propose_memory`
4. `report_completion`
5. `work`
6. `history_search`

Maintenance:

7. `sync`
8. `health_check`
9. `health_fix`
10. `coverage`
11. `eval`
12. `verify`
13. `bootstrap`

Tool input/output shapes are referenced from CLI payload/result defs to guarantee parity.

The MCP flow is index-first:

- `get_context` returns an index-first receipt with scoped rules, suggestions, memories, and current plans.
- Each rule entry now includes `rule_id`, a deterministic stable identifier derived from the existing rule `key` semantics (no additional input required).
- `fetch` resolves receipt/plan-scoped artifacts by key, or via `receipt_id` shorthand when keys are omitted.
- `work` creates/updates structured plans with tasks (max 256 per request). Supports `receipt_id` without `plan_key` (derives `plan_key` as `plan:<receipt_id>`). `mode` controls merge vs replace semantics.
- `history_search` lists or searches compact work, memory, receipt, and run history and returns targeted `fetch_keys` for selective follow-up retrieval.
- For work updates, `verify:tests` is the built-in executable verification key. `verify:diff-review` is optional workflow metadata.
- `eval` is the public retrieval-evaluation command/tool name. `verify` selects repo-defined executable checks from `.acm/acm-tests.yaml` or `acm-tests.yaml`, with `tests_file` as the explicit override.
- `propose_memory` and `report_completion` remain receipt-scoped write operations.

Advisory scope mode defaults to `warn` when `scope_mode` is omitted. When work items are present, `scope_mode=strict` enforces verification checks and `scope_mode=warn` surfaces warnings.

## Notes

- These schemas are framework-agnostic and can be implemented in Go, TypeScript, Rust, or Python.
- Recommended implementation approach for Go:
  - define structs from schema
  - validate with a JSON Schema library at ingress/egress
  - keep retrieval logic in a shared core package used by both CLI and MCP adapters.
