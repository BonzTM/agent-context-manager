# CLAUDE.md

Claude companion for a repo whose primary contract is `AGENTS.md`.

## Source Of Truth

- Follow `AGENTS.md` first.
- Use this file only to map Claude's workflow to the repo contract.
- If this file conflicts with `AGENTS.md`, `AGENTS.md` wins.

## Claude Workflow

Follow the task loop in `AGENTS.md`. For Claude, the ACM commands map to slash commands:

| ACM step | Claude slash command |
|---|---|
| `acm context` | `/acm-context [phase] <task>` |
| `acm work` | `/acm-work` |
| `acm verify` | `/acm-verify` |
| `acm review --run` | `/acm-review <id> {"run":true}` |
| `acm done` | `/acm-done` |

Direct CLI (`acm sync`, `acm health`, `acm history`, `acm status`) has no slash-command aliases — call those directly when needed.

## Claude-Specific Notes

- Keep prompts specific enough that `context` can load the right rules, plans, and any explicit initial scope.
- If the receipt looks stale or too narrow, re-run `/acm-context` with a better task description instead of guessing.
- If governed file work expands beyond the initial receipt scope, record the new files through `/acm-work` before expecting `/acm-review` or `/acm-done` to pass.
- Do not claim success when `/acm-verify` failed or was skipped for code changes.
- `/acm-review` stays thin. Use `{"run":true}` for runnable workflow gates because manual complete notes do not satisfy runnable gates, and reserve manual `status`, `outcome`, `blocked_reason`, and `evidence` fields for non-run mode.
- If `/acm-review {"run":true}` reports repo changes but zero scoped review files, the receipt or declared discovered scope is too narrow. Re-run `/acm-context` or update `/acm-work` before retrying review.
- If the repo defines a richer feature-plan contract, populate the required `plan.stages`, `stage:*` tasks, `parent_task_key`, and leaf `acceptance_criteria` before implementation, then let `verify` enforce it.
- When blocked on a missing product or architectural decision, surface the decision instead of improvising it.

## Web Dashboard

If `acm-web` is installed, humans can view agent work at `http://localhost:8080/` — a read-only kanban board and status page. No agent interaction needed.

## Ruleset Maintenance

When `.acm/acm-rules.yaml`, `.acm/acm-tags.yaml`, `.acm/acm-tests.yaml`, or `.acm/acm-workflows.yaml` changes, refresh broker state with `acm sync` or `acm health --apply`, then run `acm health`.
