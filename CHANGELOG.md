# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

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

[1.0.0]: https://github.com/BonzTM/agent-context-manager/releases/tag/v1.0.0
