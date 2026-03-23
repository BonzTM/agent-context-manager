# CLAUDE.md

Claude companion for a repo whose primary contract is `AGENTS.md`.

## Source Of Truth

- Follow `AGENTS.md` first.
- Use this file only to map Claude's workflow to the repo contract.
- If this file conflicts with `AGENTS.md`, `AGENTS.md` wins.

## Claude Workflow

For non-trivial work (multi-step, multi-file, or governed), follow this loop. Trivial single-file fixes can skip the ACM ceremony.

1. Start with `/acm-context ...`.
2. Read the returned hard rules before touching files.
3. Use `/acm-work ...` when the task is multi-step, spans multiple files, or needs durable state.
4. Use `/acm-verify ...` before `/acm-done ...` for any code, config, schema, or executable behavior change.
5. Use `/acm-review <receipt_id-or-plan_key> {"run":true}` when `.acm/acm-workflows.yaml` requires a review task such as `review:cross-llm` and the task defines a `run` block; otherwise use manual review JSON or `/acm-work ...`.
6. Use `/acm-done ...` to close the task; include changed files for file-backed work when you have them, or let ACM compute the task delta from the receipt baseline. When that detected delta is empty, the closeout is effectively no-file. ACM may enforce additional task keys from `.acm/acm-workflows.yaml` when file-backed work is detected.
If the task changes rules, tags, tests, workflows, onboarding, or tool-surface behavior, run direct CLI `acm sync --mode working_tree --insert-new-candidates` and `acm health --include-details` before `/acm-done`.

If you need historical discovery after compaction, use direct CLI `acm history` with `--entity work` for plan/task discovery or another entity for receipts and runs, then `acm fetch` the returned `fetch_keys`; the default slash-command pack does not add a dedicated `/acm-history` command.
If you need runtime or setup diagnostics, use direct CLI `acm status`.

## Claude-Specific Notes

- Keep prompts specific enough that `context` returns the right rules, active work, and any explicitly known scope.
- If the receipt looks stale or too narrow, re-run `/acm-context` with a better task description instead of guessing.
- If governed file work expands beyond the initial receipt scope, record the new files through `/acm-work` before expecting `/acm-review` or `/acm-done` to pass.
- Do not claim success when `/acm-verify` failed or was skipped for code changes.
- `/acm-review` stays thin. Use `{"run":true}` for runnable workflow gates because manual complete notes do not satisfy runnable gates, and reserve manual `status`, `outcome`, `blocked_reason`, and `evidence` fields for non-run mode.
- If `/acm-review {"run":true}` reports repo changes but zero scoped review files, the receipt or declared discovered scope is too narrow. Re-run `/acm-context` or update `/acm-work` before retrying review.
- If the repo defines a richer feature-plan contract, populate the required `plan.stages`, `stage:*` tasks, `parent_task_key`, and leaf `acceptance_criteria` before implementation, then let `verify` enforce it.
- When blocked on a missing product or architectural decision, surface the decision instead of improvising it.

## Ruleset Maintenance

When `.acm/acm-rules.yaml`, `.acm/acm-tags.yaml`, `.acm/acm-tests.yaml`, or `.acm/acm-workflows.yaml` changes, refresh broker state with `acm sync` or `acm health --apply`, then run `acm health`.
