# CLAUDE.md - agent-context-manager Claude Companion

IMPORTANT: Read `AGENTS.md` first. It is the authoritative contract for this repository.
After any compaction, reset, or handoff, re-read `AGENTS.md` and restart the ACM flow before resuming work.

## Source Of Truth

- `AGENTS.md` is authoritative.
- This file exists only to map Claude workflows onto the repo contract.
- If this file conflicts with `AGENTS.md`, follow `AGENTS.md`.

## Claude Workflow

1. Start with `/acm-context` for the current task and phase. It is the slash-command wrapper for `acm context`.
2. Read the returned hard rules before opening or editing broad areas of the repo.
3. Let `/acm-context` capture the receipt and hard rules. Use `acm fetch` only when a returned plan, task, memory, or pointer key actually needs to be hydrated.
4. Use `/acm-work` as soon as the task is multi-step, multi-file, likely to need resumption, or when discovered file scope must be declared for governed review/done flows.
5. Use `/acm-verify` before `/acm-done` for code, config, contract, onboarding, or behavior changes.
6. Use `/acm-done` to close the task. ACM may auto-detect the task delta, but explicit discovered paths still belong in `/acm-work`.
7. Use `/acm-memory` for durable decisions, recurring pitfalls, and repository preferences. It is the slash-command alias for `acm memory`.

## Claude-Specific Notes

- Use the project id `agent-context-manager`.
- Keep task prompts specific enough that the returned rules, plans, memory, and any explicit initial scope match the real task.
- If the receipt is stale, too broad, or too narrow, re-run `/acm-context` with a better task description instead of guessing.
- Do not skip `/acm-work` just because the task started small. Once it becomes multi-step, track it.
- Do not claim success when `/acm-verify` failed or was skipped for executable changes.
- If the slash-command pack is unavailable, use the installed `acm` and `acm-mcp` binaries directly rather than falling back to `go run ./cmd/...` for normal repo workflow.

## Repo-Specific Expectations

- Contract changes must update schemas, validation, and tests together.
- CLI changes must keep MCP/help/tool metadata aligned.
- Storage changes must preserve Postgres/SQLite parity.
- Onboarding changes must preserve the clean init path for this repo and other ACM-managed repos.
- Workflow or behavior changes must update repo docs, examples, and broker assets in the same change.
- Net-new feature work must use the repo-local feature plan contract in `docs/feature-plans.md`: `kind=feature`, explicit scope metadata, `plan.stages`, top-level `stage:*` tasks, and leaf-task `acceptance_criteria`. `acm verify` runs `scripts/acm-feature-plan-validate.py` when relevant.
- Verification coverage is currently anchored by:
  - `smoke` on every run
  - `acm-feature-plan-help` when feature-plan governance surfaces change
  - `acm-feature-plan-validate` when feature-relevant work is active; non-feature or unmaterialized receipt plans should skip cleanly
  - `cli-build` for `cmd/**`, `internal/**`, `go.mod`, and `go.sum` changes in `execute` or `review`
  - `full-go-suite` in `review` for broader product-surface changes

## Maintenance Commands

When changing governance or onboarding behavior, the normal maintenance loop is:

1. `/acm-work ...` to track the change
2. `/acm-verify ...`
3. `acm sync --project agent-context-manager --mode working_tree --insert-new-candidates`
4. `acm health --project agent-context-manager --include-details`
5. `/acm-done ...`
