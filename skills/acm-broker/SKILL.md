---
name: acm-broker
description: Use the agent-context-manager broker (CLI or MCP) to load repo-owned rules and durable state, post work/review updates, share memory across agents, and close tasks with deterministic JSON contracts.
---

# acm-broker

Use this skill when a task needs repo-owned agent state, hard rule compliance, durable planning, shared memory, or deterministic completion reporting through `agent-context-manager`.

## Required Flow

1. Call `context` first.
2. Read and follow the returned rules block (or rule pointers) as hard constraints.
3. Call `fetch` only for plan, task, memory, or pointer content you actually need to hydrate (or use `receipt_id` shorthand without explicit keys).
4. Execute work; if the task evolves, call `work` with `receipt_id` (optionally without `plan_key`) to publish broader updates. Use `tasks` payloads and `verify:tests` as the built-in executable verification task key. `.acm/acm-workflows.yaml` may require additional task keys. If the repo defines a richer feature-plan contract, populate the required `plan.stages`, top-level `stage:*` tasks, `parent_task_key`, and leaf `acceptance_criteria` before implementation; `verify` may enforce that schema.
5. When governed work discovers file scope beyond the initial receipt, record it through `work.plan.discovered_paths` before relying on `review` or `done` to pass.
6. Call `review` when you only need to record a single review-gate outcome. It lowers to one `work` task update; use `run=true` when the repo workflow defines a runnable review gate, because manual complete notes do not satisfy runnable gates. Runnable review gates only skip reruns after a passing attempt already assessed the current fingerprint; failed or interrupted same-fingerprint attempts rerun until any configured `max_attempts` budget is exhausted.
7. When code changes are involved, call `verify` before `done`. Include `receipt_id` or `plan_key` when available so `verify` can update `verify:tests`.
8. Call `done` with changed files for file-backed work when you know them; otherwise omit or leave `files_changed` empty and let ACM derive the delta from the receipt baseline. When that detected delta is empty, the closeout is effectively no-file.
9. Propose durable memory with `memory` when appropriate, including evidence that stays inside the task's effective scope. For CLI calls, prefer `--evidence-path` when you only know governed repo-relative files and `--evidence-key` when you already have exact fetched pointer keys.

When the task changes repo governance or onboarding state such as rules, tags, tests, workflows, or tool-surface behavior, also run `acm sync --mode working_tree --insert-new-candidates` and `acm health --include-details` before `done`.

## Interfaces

- These commands assume installed `acm` and `acm-mcp` binaries are available on `PATH`.
- Preferred CLI path:
  - `acm context ...`
  - `acm fetch ...`
  - `acm work ...`
  - `acm review ...`
  - `acm verify ...`
  - `acm done ...`
  - `acm memory ...`
- Optional structured JSON automation path:
  - `acm validate --in <request.json>`
  - `acm run --in <request.json>`
- MCP path:
  - `acm-mcp invoke --tool context --in <payload.json>`
  - `acm-mcp invoke --tool fetch --in <payload.json>`
  - `acm-mcp invoke --tool review --in <payload.json>`
  - `acm-mcp invoke --tool work --in <payload.json>`
  - `acm-mcp invoke --tool verify --in <payload.json>`
  - `acm-mcp invoke --tool done --in <payload.json>`
  - `acm-mcp invoke --tool memory --in <payload.json>`

Defaults:
- SQLite backend is default when `ACM_PG_DSN` is unset.
- `ACM_PROJECT_ID` can provide a default project namespace for convenience, `run`, `validate`, and MCP calls; otherwise acm infers from the effective repo root.
- Optional logging controls:
  - `ACM_LOG_LEVEL=debug|info|warn|error`
  - `ACM_LOG_SINK=stderr|stdout|discard`

## Templates

Use templates from `references/templates.md` and `assets/requests/*.json`.
For Codex-first repo setup, see `codex/README.md` and `codex/AGENTS.example.md` inside this skill for the companion docs that pair the global skill with a repo-root `AGENTS.md`.

## Rules

- Keep all requests valid `acm.v1` JSON contracts.
- Never skip `context` before execution.
- Treat the `context` rules block (or rule pointers) as mandatory requirements.
- Do not invent or silently widen governed file scope. Record discovered paths through `work` when they matter to `review` or `done`.
- If a planned step or review gate becomes obsolete, mark it `superseded` instead of leaving it open or `blocked`.
- `memory` requires evidence. Use exact receipt rule keys or indexed pointer keys whose repo-relative paths fall inside effective scope, or use the CLI `--evidence-path` shorthand to derive those keys from governed repo-relative files.
- Treat advisory scope as `warn` by default unless an explicit `scope_mode` override is required.
- `review` is a thin convenience that lowers to one `work.tasks[]` update. Defaults: `key=review:cross-llm`, `summary="Cross-LLM review"`, `status=complete`.
- When the repo workflow defines a runnable review gate, prefer `run=true` and keep manual `status`, `outcome`, `blocked_reason`, and `evidence` fields for non-run mode.
- Runnable review gates may return a skipped result when the current scoped fingerprint was already assessed or an explicitly configured `max_attempts` budget is exhausted.
- When `work.tasks` is non-empty, include `verify:tests` for executable verification tracking.
- `verify:diff-review` is optional workflow metadata, not a built-in acm completion gate.
- Some repos use `verify` to enforce richer feature-plan schemas built on `kind=feature`, stage tasks, task hierarchy, and leaf-task acceptance criteria. Follow the repo-local contract when it exists.
- For code changes, run `verify` before `done` unless the repo rules explicitly allow otherwise.
- For `done`, `scope_mode=strict` blocks on incomplete required completion tasks. When changed files are supplied and no workflow gates are configured, ACM falls back to `verify:tests`; `scope_mode=warn` surfaces warnings.
- `health` and `status` warn on stale work plans, terminal-task plan status drift, and administrative-closeout-only plans.
- No-file `done` calls are valid for legitimate planning, research, or review-only closures.
- If the receipt is too narrow or the task materially changed, refine and re-run `context` instead of guessing.
- Preserve structured JSON output for all broker interactions.
