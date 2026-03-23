# CLAUDE.md - agent-context-manager Claude Companion

Everything in `AGENTS.md` applies. This file only adds Claude-specific mappings.

## Slash-Command Workflow

For non-trivial work sessions (multi-step, multi-file, or governed), the ACM task loop from `AGENTS.md` maps to these slash commands:

| AGENTS.md step | Claude command |
|---|---|
| `acm context` | `/acm-context [phase] <task>` |
| `acm work` | `/acm-work` |
| `acm verify` | `/acm-verify` |
| `acm review --run` | `/acm-review <id> {"run":true}` |
| `acm done` | `/acm-done` |

Direct CLI (`acm sync`, `acm health`, `acm history`, `acm status`) has no slash-command wrappers — call those directly.

## Memory (AMM)

AMM is available via MCP tools in this session. Use it per `AGENTS.md` § Memory:

- `amm_recall` — check for relevant prior knowledge at task start or decision points.
- `amm_remember` — commit stable decisions or lessons learned.
- `amm_expand` — expand thin recall items when you need more detail.

## Claude-Only Notes

- Use project id `agent-context-manager`.
- If the receipt is stale or too narrow, re-run `/acm-context` with a better task description.
- If governed scope expands, declare new files through `/acm-work` before `/acm-review` or `/acm-done`.
- If `acm` or `go` is missing, follow the bootstrap block in `AGENTS.md` § Fast Path before proceeding.
- Use installed `acm` and `acm-mcp` binaries, not `go run ./cmd/...`, for normal workflow.
