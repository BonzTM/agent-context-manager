# CLAUDE.md

Claude companion for a repo whose primary contract is `AGENTS.md`.

## Source Of Truth

- Follow `AGENTS.md` first.
- Use this file only to map Claude's workflow to the repo contract.
- If this file conflicts with `AGENTS.md`, `AGENTS.md` wins.

## Claude Workflow

1. Start with `/acm-get ...`.
2. Read the returned hard rules before touching files.
3. Use `/acm-work ...` when the task is multi-step, spans multiple files, or needs durable state.
4. Use `/acm-verify ...` before `/acm-report ...` for any code, config, schema, or executable behavior change.
5. Use `/acm-review <receipt_id-or-plan_key> {"run":true}` when `.acm/acm-workflows.yaml` requires a review task such as `review:cross-llm` and the task defines a `run` block; otherwise use manual review JSON or `/acm-work ...`.
6. Use `/acm-report ...` to close the task; it may enforce additional task keys from `.acm/acm-workflows.yaml`.
7. Use `/acm-memory ...` for durable decisions and gotchas.
8. Use `/acm-eval ...` only for retrieval-quality maintenance, not task completion.

If you need historical plan discovery, use direct CLI `work search --scope all`. If you need receipts, runs, or memories too, use `history search --entity all`, `history search --entity memory`, or MCP `history_search`, then `fetch` the returned `fetch_keys`; the default slash-command pack does not add a dedicated `/acm-history` command.
If you need runtime or setup diagnostics, use direct CLI `acm status`; `acm doctor` is only an alias.

## Claude-Specific Notes

- Keep prompts specific enough that `get_context` can retrieve the right pointers.
- If the receipt looks stale or too narrow, re-run `/acm-get` with a better task description instead of guessing.
- Do not claim success when `/acm-verify` failed or was skipped for code changes.
- `/acm-review` stays thin. Use `{"run":true}` for runnable workflow gates and reserve manual `status`, `outcome`, `blocked_reason`, and `evidence` fields for non-run mode.
- If `/acm-review {"run":true}` reports repo changes but zero scoped review files, the receipt is too narrow. Re-run `/acm-get` with the broader task before retrying review.
- If the repo defines a richer feature-plan contract, populate the required `plan.stages`, `stage:*` tasks, `parent_task_key`, and leaf `acceptance_criteria` before implementation, then let `verify` enforce it.
- When blocked on a missing product or architectural decision, surface the decision instead of improvising it.

## Ruleset Maintenance

When `.acm/acm-rules.yaml`, `.acm/acm-tags.yaml`, `.acm/acm-tests.yaml`, or `.acm/acm-workflows.yaml` changes, refresh broker state with `sync` or `health --apply`, then run `health`.
