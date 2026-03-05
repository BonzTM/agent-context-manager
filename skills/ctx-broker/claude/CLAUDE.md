# CLAUDE.md - ctx-broker Skill Companion

Use this companion when running `ctx-broker` workflow inside Claude Code.

## Required Order

1. Run context retrieval first (`/ctx-get ...`).
2. Follow the returned `get_context` rules block (or rule pointers) as hard constraints.
3. Treat code pointers as advisory and run `fetch` with `receipt_id` shorthand (or explicit keys when needed).
4. Execute work; if context is stale/insufficient, retrieve again.
5. On completion, run `/ctx-report ...`.
6. If plan tracking is active, post `work` updates using `receipt_id` (with optional `plan_key`) or send zero items for status-only retrieval.
7. If a durable discovery was made, run `/ctx-memory ...`.

## Contract Notes

- Keep broker payloads valid against `ctx.v1` contract.
- Do not treat code pointer paths as hard edit boundaries.
- Do treat `get_context` rules as mandatory.
- Scope mode defaults to advisory `warn` when omitted.
- If retrieval is insufficient, refine task text and retrieve again.
