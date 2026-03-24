# AGENTS.md

Starter operating contract for a repo that uses `acm`.

## Source Of Truth

- Follow this file first.
- Keep canonical rules in `.acm/acm-rules.yaml` (preferred) or `acm-rules.yaml` at the repo root.
- Keep canonical tags in `.acm/acm-tags.yaml` and executable checks in `.acm/acm-tests.yaml`.
- Keep canonical completion workflow gates in `.acm/acm-workflows.yaml` (preferred) or `acm-workflows.yaml`.
- If tool-specific instructions conflict with this file, this file wins unless a human explicitly says otherwise.

## Task Loop

See [.acm/acm-work-loop.md](.acm/acm-work-loop.md) for the full ACM command reference (CLI and MCP).

The short version: `context` → `work` → `verify` → `done`. Trivial single-file fixes can skip the ceremony.

## Working Rules

- Do not silently expand governed file scope. Use `work.plan.discovered_paths` when later-discovered files must be declared.
- Prefer small, reviewable changes over broad cleanup.
- Do not invent product requirements or compatibility guarantees the repo does not define.
- If verification fails, fix it or report it clearly. Do not claim success.
- Keep work state current when you pause, hand off, or hit a blocker.
- Mark obsolete tasks `superseded` instead of leaving them open or `blocked`.

## Ruleset Maintenance

1. Edit the canonical rules, tags, tests, or workflow files.
2. Run `acm sync --mode working_tree --insert-new-candidates` or `acm health --apply`.
3. Run `acm health --include-details` and resolve blocking findings.