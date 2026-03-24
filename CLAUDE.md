# CLAUDE.md - agent-context-manager Claude Companion

Everything in `AGENTS.md` applies. This file adds Claude-specific notes.

If ACM is available in your session, also follow [.acm/AGENTS-ACM.md](.acm/AGENTS-ACM.md).  If you are unaware or unsure of what ACM is, do not read the file.

## ACM Workflow (when available)

See [.acm/acm-work-loop.md](.acm/acm-work-loop.md) for the full command reference. Claude slash-command equivalents:

| AGENTS-ACM.md step | Claude command |
|---|---|
| `acm context` | `/acm-context [phase] <task>` |
| `acm work` | `/acm-work` |
| `acm verify` | `/acm-verify` |
| `acm review --run` | `/acm-review <id> {"run":true}` |
| `acm done` | `/acm-done` |

Direct CLI (`acm sync`, `acm health`, `acm history`, `acm status`) has no slash-command wrappers — call those directly.

## Claude-Only Notes

- Use project id `agent-context-manager`.
- If using ACM and the receipt is stale or too narrow, re-run `/acm-context` with a better task description.
- If governed scope expands, declare new files through `/acm-work` before `/acm-review` or `/acm-done`.
- If `acm` or `go` is missing, follow the bootstrap block in [.acm/AGENTS-ACM.md](.acm/AGENTS-ACM.md).
- Use installed `acm` and `acm-mcp` binaries, not `go run ./cmd/...`, for normal workflow.
