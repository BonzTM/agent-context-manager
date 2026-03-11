# CLAUDE.md - agent-context-manager Claude Companion

Everything in `AGENTS.md` applies. This file only adds Claude-specific mappings.

## Slash-Command Workflow

The ACM task loop from `AGENTS.md` maps to these slash commands:

| AGENTS.md step | Claude command |
|---|---|
| `acm context` | `/acm-context [phase] <task>` |
| `acm work` | `/acm-work` |
| `acm verify` | `/acm-verify` |
| `acm review --run` | `/acm-review <id> {"run":true}` |
| `acm done` | `/acm-done` |
| `acm memory` | `/acm-memory` |

Direct CLI (`acm sync`, `acm health`, `acm history`, `acm status`) has no slash-command wrappers — call those directly.

## Claude-Only Notes

- Use project id `agent-context-manager`.
- If the receipt is stale or too narrow, re-run `/acm-context` with a better task description.
- If governed scope expands, declare new files through `/acm-work` before `/acm-review` or `/acm-done`.
- If `acm` or `go` is missing, follow the bootstrap block in `AGENTS.md` § Fast Path before proceeding.
- Use installed `acm` and `acm-mcp` binaries, not `go run ./cmd/...`, for normal workflow.
