# CLAUDE.md - agent-context-manager Claude Companion

Everything in `AGENTS.md` applies. This file only adds Claude-specific mappings.

## ACM Workflow

See [.acm/acm-work-loop.md](.acm/acm-work-loop.md) for the full command reference. Claude slash-command equivalents:

| AGENTS.md step | Claude command |
|---|---|
| `acm context` | `/acm-context [phase] <task>` |
| `acm work` | `/acm-work` |
| `acm verify` | `/acm-verify` |
| `acm review --run` | `/acm-review <id> {"run":true}` |
| `acm done` | `/acm-done` |

Direct CLI (`acm sync`, `acm health`, `acm history`, `acm status`) has no slash-command wrappers — call those directly.

## Memory (AMM)

AMM is available via MCP tools in this session. Query it early and often — see `AGENTS.md` § Memory for the full contract.

- **At session start**, run `amm recall|amm_recall` with mode `ambient` to load relevant prior context.
- **Before decisions or when uncertain**, query `amm recall|amm_recall` — don't guess when AMM might already know.
- **After stable decisions or lessons learned**, commit them with `amm remember|amm_remember`.
- Use `amm expand|amm_expand` to expand thin recall items when you need more detail.

## Claude-Only Notes

- Use project id `agent-context-manager`.
- If the receipt is stale or too narrow, re-run `/acm-context` with a better task description.
- If governed scope expands, declare new files through `/acm-work` before `/acm-review` or `/acm-done`.
- If `acm` or `go` is missing, follow the bootstrap block in `AGENTS.md` § Fast Path before proceeding.
- Use installed `acm` and `acm-mcp` binaries, not `go run ./cmd/...`, for normal workflow.
