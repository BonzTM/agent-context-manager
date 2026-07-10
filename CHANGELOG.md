# Changelog

All notable changes to this project are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

- `acm window --breakdown` reports exact rendered versus stored token estimates,
  raw/summary and role/depth subtotals, represented-message counts, sequence
  gaps/overlaps, offload references, and the active token estimator. Existing
  `--json` item output remains compatible and gains additive per-item rendered
  cost and coverage fields.
- Compaction configuration now fails fast on unreachable budgets, invalid
  fractions, unsafe loop/fanout/depth limits, and summary targets that cannot
  fit their chunks. New flags expose token-based fresh-tail protection,
  condensation input caps, and the engine's existing finite safety bounds.
- `acm backfill` previews and optionally reconciles missing Codex assistant
  turns from bounded rollout scans. `acm doctor` now verifies Codex notify/hook
  installation, executable resolution, file modes, capture freshness, and
  prompt-bearing conversations with no assistant rows.
- Project-root `.acm-policy.toml` files can exclude session/tool/path/content
  classes and declare reviewed allow-values. Missing policy files retain secure
  default redaction.
- `acm prune` provides dry-run-first, backup-verified retention with pins and
  expansion gates; `acm pin` exempts conversations; and `acm carry-over` seeds
  a new session from the deepest bounded summary layer while pinning its source.
- `acm map` now exposes total-input and item/output byte limits, item and
  worst-case call budgets, a run timeout, and an optional resume-state
  directory.
- `acm map --processor claude-agent|codex-agent` runs bounded tool-using
  sessions per item with host read-only controls, observed tool/turn limits,
  per-item deadlines, and the existing durable item-state contract.
- `acm map --schema` compiles a bounded local JSON Schema before processor work,
  hashes it into resume identity, and feeds validation failures into retries.

### Changed

- The protected fresh tail now covers recent conversational messages by both
  count and tokens while leaving tool results eligible for early compaction and
  offload. Condensed nodes consume exactly the configured fanout and never more
  than the configured token cap.
- Codex global installation now adds a `Stop` hook as a redundant transcript
  reconciliation path. Rollout capture keeps the final assistant message per
  task boundary and keys it by turn ID, matching notify deduplication.
- Session policy now distinguishes ignored sessions (no recall or writes) from
  stateless sessions (recall without writes). Successful summary traversal is
  recorded as a conservative retention acknowledgement.
- Map runs preflight the complete input before model work, stream through a
  `2 * concurrency + 1` in-memory bound, persist synced per-item attempt state
  and a validated input snapshot, resume only unfinished items, and publish
  ordered output only after success.

### Security

- Deterministic redaction now runs before conversation/message persistence and
  transforms both canonical content and raw hook JSON. Common private keys,
  access/API tokens, JWTs, bearer tokens, and credential assignments therefore
  cannot reach FTS, summaries, offloads, logs, or subsequent backups.
- The local-data threat model and ADR 0002 document the limits of owner-only
  modes and defer database encryption until driver, licensing, key management,
  migration, recovery, and rollback requirements are resolved.
- Agentic map isolates Claude to read-only built-ins with no MCP servers and
  runs Codex under its read-only sandbox without user rules or configuration.
  Both stream bounded events and terminate the child process group on limits.

## [1.1.1] - 2026-07-09

Codex assistant-turn capture patch. The `notify` command installed by 1.1.0
was correctly placed at the top level, but `acm hook` read only stdin while
Codex supplies notification JSON as one positional argument.

### Fixed

- `acm hook` now accepts Codex's single positional notification payload while
  retaining stdin payloads for lifecycle hooks. `agent-turn-complete` events
  therefore persist both the input messages and final assistant response.

See [docs/release-notes/RELEASE_NOTES_1.1.1.md](docs/release-notes/RELEASE_NOTES_1.1.1.md) for the full release notes.

## [1.1.0] - 2026-07-09

Capture-correctness and operational-hardening release: Codex assistant turns
are wired reliably, repeated source events remain lossless, automatic recall is
quieter and better ranked, local state is owner-only, and every headless model
call has a hard deadline. The database schema and command names are unchanged.

### Fixed

- Codex global installation now parses `config.toml`, places `notify` at the
  top level, and relocates the legacy ACM block when an earlier release
  accidentally appended it inside the final TOML table.
- Capture now uses hook `turn_id` and `tool_use_id` identities, with the raw
  event payload as an idempotency fallback, so equal prompts or tool results
  from distinct events are no longer collapsed.
- Recall blocks direct `msg_` IDs to `acm describe` instead of the summary-only
  `acm expand` command.
- Automatic recall filters low-signal prompt terms and deterministically reranks
  BM25 candidates by coverage, current conversation, role, recency, and payload
  size instead of injecting the raw top-five OR matches.

### Changed

- Headless Claude/Codex calls now have a 120-second deadline. Unix process
  groups are terminated together, and inherited output pipes have a final
  one-second drain bound.
- `acm window` now describes its output as ACM's synthetic persisted view, not
  the live prompt on augmentation-only hosts.

### Security

- Databases and backups are enforced to owner-only mode (`0600`) on creation
  and open; existing permissive database files are repaired automatically.

See [docs/release-notes/RELEASE_NOTES_1.1.0.md](docs/release-notes/RELEASE_NOTES_1.1.0.md) for the full release notes.

## [1.0.1] - 2026-07-09

Fast-follow patch for 1.0.0: symlink-safe global installs, duplicate-proof
instruction blocks, a stricter verification gate, sibling-consistent release
packaging, and a security toolchain bump. No schema, command, or contract
changes — a drop-in upgrade.

### Added

- Contributor Covenant code of conduct, matching the sibling
  agent-workflow-manager repo's community meta set.

### Fixed

- Global installs no longer orphan symlinked configs: the atomic write now
  follows the symlink chain and replaces the final target file, so a
  CLAUDE.md/AGENTS.md or settings.json linked into a dotfiles repo keeps its
  link (a dangling link gets its target created), and the target's file mode
  is preserved. Previously the temp-file rename converted the symlink itself
  into a regular file.
- `acm init` recognizes a hand-pasted drill-down block (without acm's
  markers) by its heading and skips it with a notice instead of appending a
  duplicate managed copy.

### Changed

- Release pipeline aligned with the sibling repos: publishing a GitHub
  release triggers a per-architecture matrix (linux/darwin/windows on
  amd64/arm64) that uploads `acm-<version>-<os>-<arch>.tar.gz` archives
  (`.zip` on Windows) with per-archive `.sha256` checksums, replacing the
  previous raw per-platform binaries; the workflow mirrors whichever tag
  form is missing (bare `X.Y.Z` or `vX.Y.Z`) onto the release commit so
  house-style bare tags and canonical Go module resolution coexist; and CI
  gains a cross-compile check for every released platform.
- Displayed versions are v-less everywhere: `acm version` strips the module
  version's `v` prefix, and release titles and stamped binaries use bare
  `X.Y.Z`; the `v` exists only on git tags, where the Go toolchain requires
  it.
- `make verify` uses a read-only `tidy-check` (`go mod tidy -diff`) instead
  of the in-place `tidy`, so CI fails on committed go.mod/go.sum drift
  rather than silently auto-correcting it in the workspace; `make tidy`
  remains the local fixer. (Found by agent-workflow-manager's cross-LLM
  review gate.)

### Security

- Raised the `go` directive to 1.26.5, whose standard library fixes
  GO-2026-5856 (`crypto/tls`) and GO-2026-4970 (`os`) — flagged by the CI
  vulnerability gate on 1.26.4 toolchains.

See [docs/release-notes/RELEASE_NOTES_1.0.1.md](docs/release-notes/RELEASE_NOTES_1.0.1.md) for the full release notes.

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

[Unreleased]: https://github.com/BonzTM/agent-context-manager/compare/v1.1.1...HEAD
[1.1.1]: https://github.com/BonzTM/agent-context-manager/releases/tag/v1.1.1
[1.1.0]: https://github.com/BonzTM/agent-context-manager/releases/tag/v1.1.0
[1.0.1]: https://github.com/BonzTM/agent-context-manager/releases/tag/v1.0.1
[1.0.0]: https://github.com/BonzTM/agent-context-manager/releases/tag/v1.0.0
