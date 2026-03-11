# AGENTS.md

Operating contract for a repo that uses `acm` and wants enforced detailed feature planning.

## Source Of Truth

- Follow this file first.
- Keep canonical rules in `.acm/acm-rules.yaml` (preferred) or `acm-rules.yaml` at the repo root.
- Keep canonical tags in `.acm/acm-tags.yaml` and executable checks in `.acm/acm-tests.yaml`.
- Keep canonical completion workflow gates in `.acm/acm-workflows.yaml` (preferred) or `acm-workflows.yaml`.
- If tool-specific instructions conflict with this file, this file wins unless a human explicitly says otherwise.

## Required Task Loop

1. Read this file and the human task.
2. Run `acm context` before opening or editing project files.
3. Follow all hard rules returned in the receipt.
4. Use `fetch` only for the pointers, plans, and task keys needed for the current step.
5. When a task spans multiple steps, multiple files, or a likely handoff, create or update `work`.
6. For net-new feature work or large capability expansions, create the repo's detailed feature plan before implementation.
7. If code, config, schema, or other executable behavior changes, run `verify` before `done`.
8. If `.acm/acm-workflows.yaml` requires review task keys such as `review:cross-llm`, prefer `review --run` when the task defines a `run` block; otherwise use manual `review` fields or `work` before `done`.
9. End every task with `done`, including every changed file for file-backed work when you know them, or letting ACM derive the task delta from the receipt baseline. When that detected delta is empty, the closeout is effectively no-file.
10. If you learn a reusable decision, gotcha, or preference, record it with `memory`.

When the task changes rules, tags, tests, workflows, onboarding, or tool-surface behavior, refresh broker state with `acm sync --mode working_tree --insert-new-candidates` and then run `acm health --include-details` before `done`.

If you need to resume after compaction or inspect archived work, use direct CLI `acm history` with `--entity work` for plan/task discovery or another entity for memories, receipts, and runs, then `acm fetch` the returned `fetch_keys`.
If you need to debug project setup, loaded ACM files, integrations, or what `context` would load for a task, use `acm status`.

## Working Rules

- Do not silently expand governed file scope. Refresh context first if the task spills into adjacent systems, and use `work.plan.discovered_paths` when later-discovered files must be declared for review/done.
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

For code changes, include a `verify:tests` task. Add other task keys when they help resumption, coordination, or are required by `.acm/acm-workflows.yaml`. For single review-gate updates, `review` is the thinner convenience wrapper around `work`; use `review --run` for runnable workflow gates, and keep manual `status` / `outcome` / `blocked_reason` / `evidence` fields for non-run mode. If a planned task or review gate becomes obsolete, mark it `superseded` instead of leaving it open or `blocked`.

## Feature Plans

Use the richer ACM feature plan contract in this repo for net-new feature work and large capability expansions.

- Create a root ACM plan with `kind=feature` before implementation.
- Root feature plans must include `objective`, `in_scope`, `out_of_scope`, `constraints`, `references`, and stage statuses for `spec_outline`, `refined_spec`, and `implementation_plan`.
- Root feature plans must include top-level `stage:spec-outline`, `stage:refined-spec`, `stage:implementation-plan`, and `verify:tests` tasks.
- Put concrete child tasks beneath the `stage:*` grouping tasks with `parent_task_key`.
- Atomic tasks are leaf tasks. Leaf tasks must carry explicit `acceptance_criteria`.
- When the work splits into parallel execution streams, use `kind=feature_stream` plus `parent_plan_key`.
- Keep the detailed contract in `docs/feature-plans.md`.
- Let `scripts/acm-feature-plan-validate.py` enforce the schema through `verify`.

Use thinner plans for bugfixes, narrow maintenance, review-only work, or workflow-governance changes.

## Ruleset Maintenance

1. Edit the canonical rules, tags, tests, or workflow files.
2. Run `acm sync --mode working_tree --insert-new-candidates` or `acm health --apply`.
3. Run `acm health --include-details` and resolve blocking findings.

## Tool-Specific Companions

`CLAUDE.md`, slash commands, and Codex skills should stay thin and map their workflow back to this file.
If they disagree with this file, this file is authoritative.
