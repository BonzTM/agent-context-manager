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
  reuse the host agent's headless CLI.
- Agent integration: `acm hook` (capture and recall injection) for Claude Code
  and Codex, an OpenCode plugin, and `acm init` to generate integration assets.
- `acm init --global` — safe, idempotent installation of acm's hooks and
  drill-down instructions into an agent's user-level configuration, covering
  every project from one install (`--dry-run` to preview). For Claude Code it
  merges `~/.claude/settings.json`; for Codex it merges `~/.codex/hooks.json`
  (UserPromptSubmit + PostToolUse) and adds `notify`; for OpenCode it drops a
  self-contained, embedded plugin into OpenCode's auto-load directory (no npm).
- `acm map` — off-context batch processing of JSONL datasets with bounded
  concurrency and validated retries.
- Operational commands: `acm doctor`, `acm stats`, `acm version`.

[Unreleased]: https://github.com/bonztm/agent-context-manager
