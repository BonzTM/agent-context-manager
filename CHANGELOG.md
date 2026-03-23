# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [1.1.0] - 2026-03-23

Memory subsystem removed in favor of [Agent Memory Manager (AMM)](https://github.com/bonztm/agent-memory-manager).

### Added

- `docs/deprecation/memory-removal.md` — migration guidance for adopters moving from ACM memory to AMM

### Changed

- `context` receipts no longer include memories or memory-derived tags; receipt IDs differ from 1.0.0 for projects that had active memories
- `health` reports 10 check categories (down from 11); `weak_memories` removed, `unknown_tags` inspects pointer tags only
- `history` entity list is now `all|work|receipt|run` (memory entity removed)
- MCP tool catalog exposes 12 tools (down from 13)
- Claude command pack produces 7 slash commands (down from 8); `/acm-memory` removed
- All skill-pack and documentation references to `acm memory` and "durable memory" removed

### Removed

- `acm memory` command — CLI, MCP tool, HTTP API, command dispatch, and all contract types
- `fetch mem:<id>` key lookups — memory keys return not-found
- `/api/memories` HTTP routes and web dashboard Memories page
- Storage adapter methods, query builders, and domain normalization for memory persistence
- `ContextMemory`, `ExportMemoryDocument`, and `MemoryCount` from receipt and status types
- `spec/v1/` schema definitions for memory command, result, and entity enums

### Migration

- Database migration DDL preserved — existing `acm_memories` and `acm_memory_candidates` tables remain inert
- Direct upgrade from 1.0.0; no data migration required
- Adopters using `acm memory` should adopt AMM before upgrading
- See `docs/deprecation/memory-removal.md` for detailed guidance

See [docs/release-notes/RELEASE_NOTES_1.1.0.md](docs/release-notes/RELEASE_NOTES_1.1.0.md) for the full release notes.

## [1.0.0] - 2026-03-15

Initial public release of acm (agent-context-manager).

### Added

- Core agent workflow: `context`, `work`, `memory`, `verify`, `done`
- Supporting surfaces: `fetch`, `review`, `history`
- Human-facing setup: `init`, `sync`, `health`, `status`
- Backend-only `export` surface via `acm run` or MCP
- Init templates: `starter-contract`, `detailed-planning-enforcement`, `verify-generic`, `verify-go`, `verify-ts`, `verify-python`, `verify-rust`, `codex-pack`, `opencode-pack`, `claude-command-pack`, `claude-hooks`, `git-hooks-precommit`
- Agent integrations: Claude Code (slash commands), Codex (global skill), OpenCode (repo-local companion docs), MCP (13 tools)
- Storage backends: SQLite (zero-config default) and Postgres (multi-writer)
- Web dashboard (`acm-web`): Board, Memories, Status, Health pages with Docker support
- Configuration: rules, tags, tests, workflows in `.acm/` YAML files
- Wire contract: `spec/v1/` with full JSON schema definitions and CLI/MCP parity
- Documentation: README, getting-started guide, concepts, feature-plans, SQLite operations, logging standards, examples
- Four adoption modes: plans-only, plans+memory, governed workflow, full brokered flow

See [docs/release-notes/RELEASE_NOTES_1.0.0.md](docs/release-notes/RELEASE_NOTES_1.0.0.md) for the full release notes.

[1.1.0]: https://github.com/BonzTM/agent-context-manager/releases/tag/v1.1.0
[1.0.0]: https://github.com/BonzTM/agent-context-manager/releases/tag/v1.0.0
