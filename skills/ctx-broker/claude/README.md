# Claude Command Pack - ctx-broker

This folder provides Claude Code slash-command prompts that mirror the `ctx-broker` workflow.

## Commands

- `/ctx-get <task text>`
  - runs context retrieval first, surfaces hard rules, and treats code pointers as advisory.
  - includes a `fetch` step with `receipt_id` shorthand (or explicit keys when needed).
- `/ctx-report <receipt_id> <comma-separated files> <outcome summary>`
  - runs completion reporting and applies scope-gate semantics.
  - includes an optional `work` step using `receipt_id` with optional `plan_key` and optional/zero items.
- `/ctx-memory <receipt_id> <category> <subject> <content>`
  - proposes durable memory in broker format.

## Install into a project

```bash
mkdir -p .claude/commands
cp skills/ctx-broker/claude/commands/*.md .claude/commands/
```

Then restart Claude Code so commands are reloaded.

## Runtime notes

- Default backend: SQLite unless `CTX_PG_DSN` is set.
- Scope mode defaults to advisory `warn` when `scope_mode` is omitted.
- Optional logger controls:
  - `CTX_LOG_LEVEL=debug|info|warn|error`
  - `CTX_LOG_SINK=stderr|stdout|discard`
