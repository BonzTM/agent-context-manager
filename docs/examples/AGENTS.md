# AGENTS.md (Starter)

Minimal startup contract for repositories using `acm`.

## Required Startup Order

1. Read this file.
2. Confirm canonical rules exist at `.acm/acm-rules.yaml` (or `acm-rules.yaml` in the project root).
3. Run `get_context` for the task.
4. Run `fetch` only for needed keys.
5. Implement the task.
6. Run `report_completion`.

## Canonical Rules Maintenance

When a rule changes:

1. Add/remove/update rule entries in your canonical ruleset file.
2. Run `sync` or `health_fix apply`.
3. Run `health_check` and resolve blocking issues.

## CLAUDE.md Mapping

`CLAUDE.md` should stay thin and map Claude behavior to this file.
If `CLAUDE.md` and `AGENTS.md` disagree, `AGENTS.md` is authoritative.
