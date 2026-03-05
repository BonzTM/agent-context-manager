# CLAUDE.md (Starter)

Claude-specific companion for a repository whose primary contract is `AGENTS.md`.

## Source of Truth

- Follow `AGENTS.md` startup order and rules.
- If any instruction conflicts, prefer `AGENTS.md`.

## Minimal acm Flow

1. Retrieve task context with `get_context`.
2. Read only required artifacts via `fetch`.
3. Execute requested work.
4. Send `report_completion` with changed files and outcome.

## Canonical Rules Changes

When canonical rules are edited, follow the same maintenance flow defined in `AGENTS.md`:
add/remove/update rules, then run `sync` or `health_fix apply`, then `health_check`.
