# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [1.2.0] - 2026-03-26

Architectural retrofit and code quality pass. MCP server migrates to JSON-RPC 2.0, CLI and MCP routing move into internal adapters behind ultra-thin entrypoints, error codes are centralized, and four reference docs ship. Status constant duplication resolved, monolithic test file split into per-command files, hand-rolled helpers replaced with Go builtins, storage parity coverage strengthened, and cmd/ entrypoints gain smoke tests.

### Added

- JSON-RPC 2.0 stdio MCP server — `acm-mcp` now implements the standard MCP protocol (`initialize`, `tools/list`, `tools/call`) over line-delimited JSON on stdin/stdout
- `internal/contracts/v1/errors.go` — centralized error code constants and error source constants
- `Source` field on `ErrorPayload` and `core.APIError` — traces error origin for debugging
- `core.NewErrorWithSource()` constructor and `internal/service/backend/errors.go` helper
- `internal/adapters/mcp/jsonrpc.go`, `protocol.go`, `server.go` — JSON-RPC 2.0 types, MCP method dispatch, and stdio server loop
- `internal/adapters/mcp/jsonrpc_test.go`, `protocol_test.go`, `app_test.go` — full MCP test coverage
- `internal/contracts/v1/command_catalog_test.go` — catalog completeness test
- `internal/contracts/v1/errors_test.go` — error code uniqueness and format tests
- `internal/core/errors_test.go` — `APIError.ToPayload()` source propagation test
- `cmd/acm/main_test.go`, `cmd/acm-mcp/main_test.go`, `cmd/acm-web/main_test.go` — entrypoint smoke tests
- 2 new shared parity contract subtests (rule sync roundtrip, DoD JSON roundtrip) in `repositorycontract/repository_contract.go`
- `docs/architecture.md`, `docs/cli-reference.md`, `docs/mcp-reference.md`, `docs/integration.md`

### Changed

- **BREAKING**: `acm-mcp` is now a JSON-RPC 2.0 stdio server; `acm-mcp tools` and `acm-mcp invoke` subcommands removed
- `cmd/acm/main.go` and `cmd/acm-mcp/main.go` — reduced to 11-line thin shells delegating to adapter `Run*` functions
- CLI routing moved from `cmd/acm/` to `internal/adapters/cli/`; MCP dispatch moved to `internal/adapters/mcp/`
- `internal/adapters/mcp/invoke.go` — `ToolDef` updated to MCP format; `spec/v1/mcp.tools.v1.json` updated to match
- Ad-hoc error code string literals replaced with `v1.ErrCode*` constants across validation, dispatch, and backend
- `README.md` and `docs/getting-started.md` — updated with links to new reference docs
- `skills/acm-broker/` — MCP example payloads and READMEs converted to JSON-RPC 2.0

### Fixed

- `complete` vs `completed` status duplication — removed dead `WorkItemStatusCompleted` and `PlanStatusCompleted` constants; collapsed dual-case switch branches; normalization functions retain `"completed"` literal matching as safety net for legacy data

### Refactored

- Split monolithic `service_test.go` (7645 lines, 150 tests) into `fakes_test.go`, `helpers_test.go`, and 10 per-command test files
- Replaced hand-rolled `minInt`, `maxZero`, `maxInt` helpers with Go 1.21+ builtin `min()` and `max()`
- Promoted 2 SQLite-only parity tests to shared contract — both adapters now run them automatically
- Added doc comment to `ProjectIDFromPayload` reflect fallback explaining forward-compatibility purpose

### Removed

- `acm-mcp tools` and `acm-mcp invoke` subcommands — replaced by JSON-RPC methods
- `internal/service/backend/service_test.go` — split into 12 per-command test files
- `internal/adapters/sqlite/repository_rules_test.go` and `repository_run_summary_test.go` — promoted to shared contract

See [docs/release-notes/RELEASE_NOTES_1.2.0.md](docs/release-notes/RELEASE_NOTES_1.2.0.md) for the full release notes.

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

[1.2.0]: https://github.com/BonzTM/agent-context-manager/releases/tag/1.2.0
[1.1.2]: https://github.com/BonzTM/agent-context-manager/releases/tag/1.1.2
[1.1.1]: https://github.com/BonzTM/agent-context-manager/releases/tag/1.1.1
[1.1.0]: https://github.com/BonzTM/agent-context-manager/releases/tag/1.1.0
[1.0.0]: https://github.com/BonzTM/agent-context-manager/releases/tag/1.0.0
