# CLAUDE.md

Claude companion for a repo whose primary contract is `AGENTS.md`.

## Source Of Truth

- Follow `AGENTS.md` first.
- If this file conflicts with `AGENTS.md`, `AGENTS.md` wins.

## Claude Workflow

See [.acm/acm-work-loop.md](.acm/acm-work-loop.md) for the full command reference. Claude slash-command equivalents:

| ACM command | Claude slash command |
|---|---|
| `acm context` | `/acm-context` |
| `acm work` | `/acm-work` |
| `acm verify` | `/acm-verify` |
| `acm review --run` | `/acm-review` |
| `acm done` | `/acm-done` |

Direct CLI (`acm sync`, `acm health`, `acm history`, `acm status`, `acm fetch`) has no slash-command wrappers — call those directly.

## Notes

- If the receipt looks stale or too narrow, re-run `/acm-context` with a better task description.
- If governed scope expands, declare new files through `/acm-work` before `/acm-review` or `/acm-done`.
- Do not claim success when `/acm-verify` failed or was skipped for code changes.
- When blocked on a missing decision, surface it instead of improvising.
