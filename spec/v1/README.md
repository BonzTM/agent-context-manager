# ctx v1 Schemas

This directory defines the v1 wire contract for the context broker.

## Files

- `shared.schema.json`: reusable types used by CLI and MCP contracts.
- `cli.command.schema.json`: input envelope for `ctx` CLI JSON mode.
- `cli.result.schema.json`: output envelope for `ctx` CLI JSON mode.
- `mcp.tools.v1.json`: MCP tool contracts with exact input/output schema refs.

## CLI Contract

`ctx` in JSON mode should accept a request matching `cli.command.schema.json` and emit a response matching `cli.result.schema.json`.

## MCP Contract

Expose exactly five tools in v1.1:

1. `get_context`
2. `fetch`
3. `propose_memory`
4. `report_completion`
5. `work`

Tool input/output shapes are referenced from CLI payload/result defs to guarantee parity.

The v1.1 MCP flow is index-first:

- `get_context` returns an index-first receipt with scoped rules, suggestions, memories, and plans.
- Each rule entry now includes `rule_id`, a deterministic stable identifier derived from the existing rule `key` semantics (no additional input required).
- `fetch` resolves receipt/plan-scoped artifacts by key, or via `receipt_id` shorthand when keys are omitted.
- `work` updates plan-scoped work item state and supports `receipt_id` without `plan_key`.
- `work` also supports status-only retrieval with zero `items`.
- For work updates, standard verification keys are `verify:tests` and `verify:diff-review`.
- `propose_memory` and `report_completion` remain receipt-scoped write operations.

Advisory scope mode defaults to `warn` when `scope_mode` is omitted. When work items are present, `scope_mode=strict` enforces verification checks and `scope_mode=warn` surfaces warnings.

## Notes

- These schemas are framework-agnostic and can be implemented in Go, TypeScript, Rust, or Python.
- Recommended implementation approach for Go:
  - define structs from schema
  - validate with a JSON Schema library at ingress/egress
  - keep retrieval logic in a shared core package used by both CLI and MCP adapters.
