# AGENTS.md

Starter operating contract for a repo that uses `acm`.
Keep this file as the fast path, then move heavier architecture, checklist, or troubleshooting material into linked repo-local maintainer docs as the project grows.

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
6. If code, config, schema, or other executable behavior changes, run `verify` before `done`.
7. If `.acm/acm-workflows.yaml` requires review task keys such as `review:cross-llm`, prefer `review --run` when the task defines a `run` block; otherwise use manual `review` fields or `work` before `done`.
8. End every task with `done`, including every changed file for file-backed work when you know them, or letting ACM derive the task delta from the receipt baseline. When that detected delta is empty, the closeout is effectively no-file.
   Built-in governance files such as repo-root `AGENTS.md`, `CLAUDE.md`, and canonical `.acm/**` contract files are already treated as managed completion scope.
When the task changes rules, tags, tests, workflows, onboarding, or tool-surface behavior, refresh broker state with `acm sync --mode working_tree --insert-new-candidates` and then run `acm health --include-details` before `done`.

If you need to resume after compaction or inspect archived work, use direct CLI `acm history` with `--entity work` for plan/task discovery or another entity for receipts and runs, then `acm fetch` the returned `fetch_keys`.
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

## Optional Feature Plans

If the repo wants stricter planning for net-new feature work:

- require root plans with `kind=feature`, explicit scope metadata, and `plan.stages.spec_outline` / `refined_spec` / `implementation_plan`
- require top-level `stage:*` tasks with child tasks linked through `parent_task_key`
- treat leaf tasks as the atomic tasks and require `acceptance_criteria` on those leaves
- use `kind=feature_stream` plus `parent_plan_key` for parallel execution streams
- enforce the schema through a repo-local `verify` script selected from `.acm/acm-tests.yaml`

## Ruleset Maintenance

1. Edit the canonical rules, tags, tests, or workflow files.
2. Run `acm sync` or `acm health --apply`.
3. Run `acm health` and resolve blocking findings.

## Web Dashboard

If `acm-web` is installed, humans can view agent work at `http://localhost:8080/` without touching the CLI. Run `acm-web` or `acm-web serve --addr :9090` for a custom port. The dashboard is read-only and shares the same database as `acm`.

## Tool-Specific Companions

`CLAUDE.md`, slash commands, and Codex skills should stay thin and map their workflow back to this file.
If they disagree with this file, this file is authoritative.
