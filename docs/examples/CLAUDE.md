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
5. Use `/acm-report ...` to close the task.
6. Use `/acm-memory ...` for durable decisions and gotchas.
7. Use `/acm-eval ...` only for retrieval-quality maintenance, not task completion.

If you need historical plan discovery, use direct CLI `work search --scope all`. If you need receipts or runs too, use `history search --entity all` or MCP `history_search`, then `fetch` the returned `fetch_keys`; the default slash-command pack does not add a dedicated `/acm-history` command.

## Claude-Specific Notes

- Keep prompts specific enough that `get_context` can retrieve the right pointers.
- If the receipt looks stale or too narrow, re-run `/acm-get` with a better task description instead of guessing.
- Do not claim success when `/acm-verify` failed or was skipped for code changes.
- When blocked on a missing product or architectural decision, surface the decision instead of improvising it.

## Ruleset Maintenance

When `.acm/acm-rules.yaml`, `.acm/acm-tags.yaml`, or `.acm/acm-tests.yaml` changes, refresh broker state with `sync` or `health-fix --apply`, then run `health`.
