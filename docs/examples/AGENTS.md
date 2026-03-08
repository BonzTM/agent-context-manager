# AGENTS.md

Default operating contract for a repo that uses `acm`.
Start here, then customize once the project develops stronger local conventions.

## Source Of Truth

- Follow this file first.
- Keep canonical rules in `.acm/acm-rules.yaml` (preferred) or `acm-rules.yaml` at the repo root.
- Keep canonical tags in `.acm/acm-tags.yaml` and executable checks in `.acm/acm-tests.yaml`.
- Keep canonical completion workflow gates in `.acm/acm-workflows.yaml` (preferred) or `acm-workflows.yaml`.
- If tool-specific instructions conflict with this file, this file wins unless a human explicitly says otherwise.

## Required Task Loop

1. Read this file and the human task.
2. Run `get_context` before opening or editing project files.
3. Follow all hard rules returned in the receipt.
4. Use `fetch` only for the pointers, plans, and task keys needed for the current step.
5. When a task spans multiple steps, multiple files, or a likely handoff, create or update `work`.
6. If code, config, schema, or other executable behavior changes, run `verify` before `report_completion`.
7. If `.acm/acm-workflows.yaml` requires review task keys such as `review:cross-llm`, prefer `review --run` when the task defines a `run` block; otherwise use manual `review` fields or `work` before `report_completion`.
8. End every task with `report_completion`, including every changed file and a concise outcome.
9. If you learn a reusable decision, gotcha, or preference, record it with `propose_memory`.

If you need to resume after compaction or inspect archived work, use `work list` or `work search --scope all` for plan discovery. If you need receipts, runs, or durable memories too, use `history search --entity all` or `history search --entity memory`, then `fetch` the returned `fetch_keys`.
If you need to debug project setup, loaded ACM files, integrations, or why retrieval is behaving a certain way, use `status` (preferred) or `doctor` (alias).

## Working Rules

- Do not silently expand scope. Refresh context first if the task spills into adjacent systems.
- Prefer small, reviewable changes over broad cleanup.
- Do not invent product requirements, compatibility guarantees, or migration behavior when the repo does not define them.
- If verification fails, either fix the issue or report the failure clearly. Do not claim the task is complete as if checks passed.
- Keep work state current when you pause, hand off, or hit a blocker.

## When To Use work

Use `work` when any of the following are true:

- the task will take more than one material step
- more than one file or subsystem is involved
- the task includes explicit planning, verification, or handoff
- you need durable task state that should survive compaction or session reset

For code changes, include a `verify:tests` task. Add other task keys when they help resumption, coordination, or are required by `.acm/acm-workflows.yaml`. For single review-gate updates, `review` is the thinner convenience wrapper around `work`; use `review --run` for runnable workflow gates, and keep manual `status` / `outcome` / `evidence` fields for non-run mode.

## Optional Feature Plans

If the repo wants stricter planning for net-new feature work:

- require root plans with `kind=feature`, explicit scope metadata, and `plan.stages.spec_outline` / `refined_spec` / `implementation_plan`
- require top-level `stage:*` tasks with child tasks linked through `parent_task_key`
- treat leaf tasks as the atomic tasks and require `acceptance_criteria` on those leaves
- use `kind=feature_stream` plus `parent_plan_key` for parallel execution streams
- enforce the schema through a repo-local `verify` script selected from `.acm/acm-tests.yaml`

## Ruleset Maintenance

1. Edit the canonical rules, tags, tests, or workflow files.
2. Run `sync` or `health --apply`.
3. Run `health` and resolve blocking findings.

## Tool-Specific Companions

`CLAUDE.md`, slash commands, and Codex skills should stay thin and map their workflow back to this file.
If they disagree with this file, this file is authoritative.
