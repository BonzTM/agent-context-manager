# Changelog

All notable changes to this project are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Changed

- Displayed versions are v-less everywhere: `acm version` strips the module
  version's `v` prefix, and release titles and stamped binaries use bare
  `X.Y.Z`. The `v` exists only on git tags, where the Go toolchain requires it
  for canonical resolution; the release workflow now mirrors a bare `X.Y.Z`
  alias tag onto the same commit so `go install ...@X.Y.Z` also resolves.

## [1.0.0] - 2026-07-09

Initial release: a single local binary that gives Claude Code, Codex, and
OpenCode durable, recoverable context — every conversation captured verbatim
into a per-project SQLite database, compacted into a summary DAG under a token
budget, and recoverable on demand through the agent's own shell tool.

### Added

- Lossless message store with idempotent ingestion, safe concurrent capture
  (every SQLite transaction begins `BEGIN IMMEDIATE`, so parallel agent hooks
  queue on the busy timeout and never lose a message), and full-text search
  over messages and summaries.
- Summary DAG compaction with a two-threshold token budget, a protected fresh
  tail, escalating size-guarded summarization, and a deterministic fallback.
  Compaction runs opportunistically from the capture hooks on turn-ending
  events (`Stop`, `agent-turn-complete`), so the DAG builds as you work;
  disable per-invocation with `acm hook --no-compact`. `acm compact` exposes
  the full budget: `--model-context-tokens`, `--soft-fraction`,
  `--hard-fraction`, `--fresh-tail`, `--leaf-chunk-tokens`,
  `--leaf-target-tokens`, and `--condensed-target-tokens`.
- Large-file offload to disk with type-aware exploration summaries: JSON,
  CSV/TSV, SQL, and source code get deterministic schema- or structure-level
  descriptions with no model call; unstructured content falls to the
  configured summarizer, then truncation. The extractor used is recorded and
  shown by `acm describe`.
- On-demand retrieval: `acm grep` (messages and the summary DAG, FTS5 ranked
  or literal substring), `acm expand`, `acm expand-query`, `acm describe`, and
  `acm window`. `acm expand-query --synthesize` answers the query directly
  with the host agent's model over the expanded messages, citing the `msg_`
  ids it drew on, and degrades to plain filtered output when the model is
  unavailable.
- Deterministic (offline) summarizer as the default, plus optional agent-model
  summarizers that reuse the host agent's headless CLI with depth-aware
  prompts (leaves keep concrete detail; higher DAG levels keep arcs and
  durable narrative).
- Agent integration: `acm hook` (capture and best-effort recall injection) for
  Claude Code and Codex, a self-contained OpenCode plugin, and `acm init` to
  generate per-project integration assets. Claude Code assistant turns are
  reconciled from the session transcript on `Stop`, keyed on the transcript
  line uuid so re-reads dedupe.
- `acm init --global` — safe, idempotent, atomic installation into an agent's
  user-level configuration, covering every project from one install
  (`--dry-run` to preview). Claude Code: `~/.claude/settings.json` hooks
  (UserPromptSubmit + PostToolUse + Stop) and the `Bash(acm:*)` permission.
  Codex: `~/.codex/hooks.json` plus `notify`. OpenCode: the embedded plugin
  dropped into the auto-load directory (no npm).
- `acm map` — off-context batch processing of JSONL datasets through a fixed
  worker pool with validated retries that feed the previous failure back to
  the processor.
- Operational commands: `acm doctor` (migrations, `PRAGMA integrity_check`,
  FTS row-parity), `acm backup` (consistent `VACUUM INTO` snapshot),
  `acm stats`, and `acm version` (reports module version and VCS metadata on
  `go install`-built binaries).
- Per-project state under `<project>/.acm/`, created with a self-excluding
  `.gitignore`; the database path resolves from `--db`, `$ACM_DB`,
  `$CLAUDE_PROJECT_DIR`, then the nearest ancestor `.acm/` directory.
- CI running `make verify` on every push and pull request, weekly Dependabot
  updates, and a tag-triggered release workflow publishing stamped static
  binaries for linux/darwin on amd64/arm64.

See [docs/release-notes/RELEASE_NOTES_1.0.0.md](docs/release-notes/RELEASE_NOTES_1.0.0.md) for the full release notes.

[Unreleased]: https://github.com/BonzTM/agent-context-manager/compare/v1.0.0...HEAD
[1.0.0]: https://github.com/BonzTM/agent-context-manager/releases/tag/v1.0.0
