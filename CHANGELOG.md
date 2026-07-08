# Changelog

All notable changes to this project are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

- Lossless message store with idempotent ingestion and full-text search.
- Summary DAG compaction with a two-threshold token budget, a protected fresh
  tail, escalating size-guarded summarization, and a deterministic fallback.
- Large-file offload to disk with exploration summaries.
- On-demand retrieval: `acm grep`, `acm expand`, `acm expand-query`,
  `acm describe`, and `acm window`.
- Deterministic (offline) summarizer and optional agent-model summarizers that
  reuse the host agent's headless CLI, with depth-aware prompts (leaves keep
  concrete detail; higher DAG levels keep arcs and durable narrative).
- Agent integration: `acm hook` (capture and recall injection) for Claude Code
  and Codex, an OpenCode plugin, and `acm init` to generate integration assets.
- `Stop` hook capture for Claude Code: assistant turns are reconciled from the
  session transcript (idempotent on transcript line uuid). `acm init` and
  `acm init --global` wire the hook.
- Opportunistic compaction from the capture hooks on turn-ending events
  (`Stop`, `agent-turn-complete`), so the summary DAG builds as you work;
  disable per-invocation with `acm hook --no-compact`.
- `acm init --global` — safe, idempotent, atomic installation of acm's hooks and
  drill-down instructions into an agent's user-level configuration, covering
  every project from one install (`--dry-run` to preview). For Claude Code it
  merges `~/.claude/settings.json` (UserPromptSubmit + PostToolUse + Stop and
  the `Bash(acm:*)` permission); for Codex it merges `~/.codex/hooks.json` and
  adds `notify`; for OpenCode it drops a self-contained, embedded plugin into
  OpenCode's auto-load directory (no npm).
- `acm grep` searches summary content as well as messages (skip with
  `--no-summaries`); `--json` emits `{"messages": [...], "summaries": [...]}`.
- `acm map` — off-context batch processing of JSONL datasets through a fixed
  worker pool with validated retries that feed the previous failure back to the
  processor.
- `acm compact` tuning flags: `--leaf-target-tokens`,
  `--condensed-target-tokens`, and `--hard-fraction`; a finished pass still
  above the hard threshold logs a warning with tuning hints.
- Operational commands: `acm doctor` (migrations, `PRAGMA integrity_check`,
  FTS row-parity), `acm backup` (consistent `VACUUM INTO` snapshot),
  `acm stats`, and `acm version` (reports module version and VCS metadata on
  `go install`-built binaries).
- `.acm/` state directories are created with a self-excluding `.gitignore`.
- CI running `make verify` on every push and pull request, weekly Dependabot
  updates, and a tag-triggered release workflow publishing stamped static
  binaries for linux/darwin on amd64/arm64.

### Changed

- Environment variables use the `ACM_` prefix: `ACM_DB`, `ACM_LOG_LEVEL`,
  `ACM_LOG_JSON` (formerly `LCM_DB`, `LOG_LEVEL`, `LOG_JSON`).
- Every SQLite transaction begins as `BEGIN IMMEDIATE` (`_txlock=immediate`),
  so concurrent acm processes (agent hooks fire in parallel) queue on the busy
  timeout instead of failing — concurrent capture never loses messages.
- The active window extends with messages ingested after the last compaction:
  `acm window` and `acm compact` always operate on the live conversation
  instead of a frozen snapshot.
- Recall injection is best-effort: a search failure is logged and never blocks
  capturing the prompt.
- The boundary error log honors `--log-json` and `--log-level`.

### Removed

- Unused pre-release surface: the `session_key` and `archived` conversation
  columns (schema migration 0004), the `PostToolUseFailure` hook event, and the
  redundant `idx_messages_conv_seq` index.

[Unreleased]: https://github.com/bonztm/agent-context-manager
