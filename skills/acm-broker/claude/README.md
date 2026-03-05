# Claude Command Pack - acm-broker

This folder provides Claude Code slash-command prompts that mirror the `acm-broker` workflow.

## Commands

- `/acm-get <task text>`
  - runs context retrieval first, surfaces hard rules, and treats code pointers as advisory.
  - includes a `fetch` step with `receipt_id` shorthand (or explicit keys when needed).
- `/acm-report <receipt_id> <comma-separated files> <outcome summary>`
  - runs completion reporting and applies scope-gate semantics.
  - includes an optional `work` step using `receipt_id` with optional `plan_key` and optional/zero tasks (`items` still accepted for compatibility).
- `/acm-memory <receipt_id> <category> <subject> <content>`
  - proposes durable memory in broker format.

## Install into a project

```bash
mkdir -p .claude/commands
cp skills/acm-broker/claude/commands/*.md .claude/commands/
```

Then restart Claude Code so commands are reloaded.

## Runtime notes

- Default backend: SQLite unless `ACM_PG_DSN` is set.
- Scope mode defaults to advisory `warn` when `scope_mode` is omitted.
- Optional logger controls:
  - `ACM_LOG_LEVEL=debug|info|warn|error`
  - `ACM_LOG_SINK=stderr|stdout|discard`
