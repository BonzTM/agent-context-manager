# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

Architectural retrofit applying patterns from [AMM](https://github.com/bonztm/agent-memory-manager): ultra-thin entrypoints, JSON-RPC 2.0 MCP protocol, centralized error codes, and expanded documentation.

### Added

- JSON-RPC 2.0 stdio MCP server — `acm-mcp` now implements the standard MCP protocol (`initialize`, `tools/list`, `tools/call`) over line-delimited JSON on stdin/stdout
- `internal/contracts/v1/errors.go` — centralized error code constants (`ErrCodeInvalidJSON`, `ErrCodeUnknownTool`, etc.) and error source constants (`ErrSourceValidation`, `ErrSourceDispatch`, `ErrSourceBackend`, `ErrSourceAdapter`)
- `Source` field on `ErrorPayload` and `core.APIError` — traces error origin for debugging (`json:"source,omitempty"`)
- `core.NewErrorWithSource()` constructor — creates errors with explicit source tagging
- `internal/service/backend/errors.go` — backend error helper stamping `ErrSourceBackend`
- `internal/contracts/v1/command_catalog_test.go` — catalog completeness test asserting all 12 specs have non-empty metadata, no duplicates
- `internal/contracts/v1/errors_test.go` — error code uniqueness and format tests, `ErrorPayload.Source` JSON round-trip test
- `internal/core/errors_test.go` — `APIError.ToPayload()` source propagation test
- `internal/adapters/mcp/jsonrpc.go` — JSON-RPC 2.0 request/response types, parse/serialize helpers, standard error codes
- `internal/adapters/mcp/protocol.go` — MCP method dispatch: `initialize`, `notifications/initialized`, `tools/list`, `tools/call`
- `internal/adapters/mcp/server.go` — line-delimited stdio server loop
- `internal/adapters/mcp/jsonrpc_test.go`, `protocol_test.go`, `app_test.go` — full test coverage for JSON-RPC parsing, MCP protocol methods, and server startup
- `docs/architecture.md` — layer diagram, request lifecycle, error propagation, storage parity, command catalog reference
- `docs/cli-reference.md` — full CLI command reference extracted from README
- `docs/mcp-reference.md` — MCP protocol reference rewritten for JSON-RPC 2.0
- `docs/integration.md` — generic integration guide with handshake flow, tool calling, runtime configuration snippets

### Changed

- **BREAKING**: `acm-mcp` is now a JSON-RPC 2.0 stdio server; `acm-mcp tools` and `acm-mcp invoke` subcommands removed
- `cmd/acm/main.go` — reduced to 11-line thin shell delegating to `cli.RunCLI()`
- `cmd/acm-mcp/main.go` — reduced to 11-line thin shell delegating to `mcp.RunMCP()`
- CLI routing (`convenience.go`, `routes.go`) moved from `cmd/acm/` to `internal/adapters/cli/`
- MCP dispatch logic moved from `cmd/acm-mcp/` to `internal/adapters/mcp/`
- `internal/adapters/mcp/invoke.go` — `ToolDef` updated to MCP format (`inputSchema` camelCase, `title` and `output_schema` removed)
- `spec/v1/mcp.tools.v1.json` — tool contract schema updated to match JSON-RPC tool format (`name`, `description`, `inputSchema`)
- Ad-hoc error code string literals replaced with `v1.ErrCode*` constants across `validate.go`, `dispatch.go`, `run.go`, `invoke.go`, and all backend service files
- All backend error creation routed through `backendError()` helper to stamp `ErrSourceBackend`
- `README.md` — CLI Reference and MCP sections replaced with summaries linking to new dedicated docs
- `docs/getting-started.md` — cross-references to new CLI, MCP, and integration docs added
- `skills/acm-broker/assets/requests/mcp_*.json` — all 8 MCP example payloads converted from legacy invoke format to JSON-RPC 2.0 `tools/call` requests
- `skills/acm-broker/{claude,codex,opencode}/README.md` — `acm-mcp invoke` references replaced with JSON-RPC protocol guidance

### Removed

- `acm-mcp tools` subcommand — replaced by `tools/list` JSON-RPC method
- `acm-mcp invoke` subcommand — replaced by `tools/call` JSON-RPC method
- `cmd/acm-mcp/main_test.go` — test coverage moved to `internal/adapters/mcp/app_test.go`
- `cmd/acm/convenience_test.go`, `cmd/acm/main_test.go`, `cmd/acm/routes_test.go` — test coverage moved to `internal/adapters/cli/`

## [1.1.2] - 2026-03-24

Fix for `work` command not supporting clearing `parent_task_key` once set, plus agent directive improvements and plan validator hardening.

### Fixed

- `work` merge logic now supports clearing `parent_task_key` by explicitly sending an empty string; previously, empty values were silently ignored and the stored value persisted
- `WorkTaskPayload.ParentTaskKey` changed from `string` to `*string` to distinguish "not provided" (nil, preserve existing) from "explicitly clear" (empty string)
- Plan validator (`acm-feature-plan-validate.py`) now skips tasks with `status=superseded` during validation
- Plan validator no longer errors on gate tasks (`verify:tests`, `review:*`) that have stale `parent_task_key` values

### Added

- `AGENTS.md` — Build And Verify section with build/test/lint commands for all agents
- `AGENTS.md` — Common Mistakes section with 10 repo-specific anti-patterns
- `AGENTS.md` — Decision Authority section distinguishing autonomous vs human-required agent decisions
- `CONTRIBUTING.md` — Go Style And Patterns section with errors, logging, package boundaries, and style guidance referencing the coding-handbook
- `internal/storage/domain/domain_test.go` — tests for `parent_task_key` clear and preserve merge behavior

### Changed

- `docs/examples/CLAUDE.md` — replaced duplicated ACM workflow loop with concise slash-command mapping table
- Bootstrap template `acm-feature-plan-validate.py` — fully synced to current canonical version
- `CLAUDE.md` — kept minimal as routing-only file (build commands now in `AGENTS.md`)

See [docs/release-notes/RELEASE_NOTES_1.1.2.md](docs/release-notes/RELEASE_NOTES_1.1.2.md) for the full release notes.

## [1.1.1] - 2026-03-23

Post-release cleanup: completes memory surface removal, improves Claude/Codex hooks, adds architecture diagrams, and hardens docs and init templates.

### Added

- Architecture diagrams — Excalidraw source files and PNG exports for layer and flow diagrams (`docs/architecture/`)
- `docs/maintainer-reference.md` — architecture diagram pointers
- `codex-hooks` init template — Codex-compatible hooks with receipt guard, session-context injection, prompt guard, and stop guard (`.codex/hooks/`)
- Schema file parity tests (`schema_files_test.go`) and web embed tests (`embed_test.go`)

### Changed

- Claude hooks (`acm-receipt-guard.sh`, `acm-session-context.sh`, `acm-stop-guard.sh`) — improved error handling and robustness
- `AGENTS.md` and `CLAUDE.md` — updated for post-memory workflow; AMM integration notes added
- Init templates (`starter-contract`, `detailed-planning-enforcement`) — updated `AGENTS.md`, `CLAUDE.md`, and `acm-rules.yaml` to reflect memory removal and hook improvements
- Skill-pack docs (`SKILL.md`, Claude/Codex/OpenCode READMEs) — AMM migration notes added
- `acm health` — `unknown_tags` check now only inspects pointer tags; stale memory tag references removed from canonical tags
- `acm fetch` — cleaned up dead memory-key code paths
- `spec/v1/README.md` — updated tool count and surface descriptions
- Web dashboard — removed Memories nav link from all pages; removed memory-related CSS and JS

### Removed

- `web/memories.html` — Memories page fully removed from web dashboard
- `spec/v1/shared.schema.json` — remaining memory-era schema definitions removed
- `spec/v1/cli.result.schema.json` — remaining memory result definitions removed
- `skills/acm-broker/assets/requests/mcp_memory.json` and `memory.json` — request templates removed
- `canonical_tags.json` — `memory` tag removed from embedded tag dictionary

See [docs/release-notes/RELEASE_NOTES_1.1.1.md](docs/release-notes/RELEASE_NOTES_1.1.1.md) for the full release notes.

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

[1.1.2]: https://github.com/BonzTM/agent-context-manager/releases/tag/1.1.2
[1.1.1]: https://github.com/BonzTM/agent-context-manager/releases/tag/1.1.1
[1.1.0]: https://github.com/BonzTM/agent-context-manager/releases/tag/1.1.0
[1.0.0]: https://github.com/BonzTM/agent-context-manager/releases/tag/1.0.0
