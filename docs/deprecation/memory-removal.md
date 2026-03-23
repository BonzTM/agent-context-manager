# Memory Subsystem Deprecation

ACM's built-in memory subsystem (`acm memory`) is deprecated in favor of [Agent Memory Manager (AMM)](https://github.com/bonztm/agent-memory-manager), a dedicated persistent memory substrate for agents.

## Why

ACM's memory was a simple 4-category system (decision, gotcha, pattern, preference) with a propose-then-promote lifecycle tied to receipts and work plans. AMM replaces it with a purpose-built memory system offering:

- 16 typed durable memory records (preference, fact, decision, episode, identity, procedure, constraint, and more)
- 5-layer memory architecture (working, history, compression, canonical, derived indexes)
- 9 retrieval modes including ambient recall
- Scoped memory (global, project, session) with orthogonal privacy levels
- Background reflection, compression, and maintenance workers
- FTS5 full-text search and provenance tracking
- Multi-signal scoring with 10-factor ranking

ACM's memory was tightly coupled to receipt scope and evidence pointers, which limited its usefulness as a general-purpose memory system. AMM operates independently and can serve any agent runtime.

## Migration Path

### Phase 1 — Deprecate and Hollow Out (Current)

**Status: Active**

What changed:

- `acm memory` still works but returns a `deprecation_notice` field in every response, directing users to AMM.
- `acm context` no longer fetches or injects memories into receipts. The `memories` field in context receipts is always empty. Memory-derived tags no longer appear in `resolvedTags`. `MemoryIDs` are no longer persisted in receipt scopes.
- `acm status` context preview no longer fetches memories. `MemoryCount` is always 0.
- `acm health` no longer fetches memories for the `unknown_tags` and `weak_memories` checks. These checks still exist but only operate on pointer data.
- `acm done` no longer persists `MemoryIDs` in run summaries (because `context` no longer populates them).
- `acm fetch mem:<id>` still works for looking up existing memories.
- `acm export` still renders memory documents for existing memories.
- `acm history --entity memory` still lists memory history.

**Behavioral change:** Receipt IDs are now computed without memory input. The same task text and phase will produce different receipt IDs than before this change if the project had active memories. This is expected and acceptable — receipt IDs are deterministic within a given ACM version but not guaranteed stable across versions.

**Action for adopters:** Install and configure AMM for durable agent memory. ACM's memory data remains in the database and is accessible via `fetch` and `export` but is no longer injected into the active workflow.

### Phase 2 — Remove Storage (Planned)

What will change:

- Remove memory-related database tables (`acm_memories`, `acm_memory_candidates`) from both SQLite and Postgres migrations.
- Remove repository interface methods: `FetchActiveMemories`, `LookupMemoryByID`, `PersistMemory`, `ListMemoryHistory`.
- Remove storage adapter implementations in `internal/adapters/sqlite/` and `internal/adapters/postgres/`.
- Remove storage domain normalization in `internal/storage/domain/domain.go`.
- Remove parity tests and integration tests for memory persistence.
- Remove query builders for memory SQL.
- `acm fetch mem:<id>` will return not-found for all memory keys.
- `acm export` memory document rendering will return errors for memory items.
- `acm history --entity memory` will return empty results.

**Migration required:** Before upgrading to Phase 2, export any memory data you want to preserve using `acm fetch mem:<id>` or the HTTP API `/api/memories`.

### Phase 3 — Remove Command Surface (Planned)

What will change:

- Remove the `acm memory` command entirely from CLI, MCP tool catalog, and HTTP API.
- Remove `CommandMemory` from the command dispatch table.
- Remove `MemoryPayload`, `MemoryCommandPayload`, `MemoryResult`, `MemoryValidation`, `MemoryCategory` types from contracts.
- Remove `MemoryPayload` validation from `internal/contracts/v1/validate.go`.
- Remove the `Memory()` method from the core service interface.
- Remove `memory.go` from the backend service.
- Remove `parseMemoryFetchKey`, `fetchMemoryItem` from fetch.go.
- Remove `exportMemoryDocument` from export.go.
- Remove memory-related health check helpers (`unknownTagFindings` memory branch, `weakMemoryFindings`).
- Remove `ContextMemory` type and the `Memories` field from `ContextReceipt`.
- Remove `ExportMemoryDocument` type and memory-related export fields.
- Remove memory references from CLI convenience functions and route builders.
- Update all documentation: README.md, docs/getting-started.md, docs/concepts.md, docs/maintainer-reference.md, skills/acm-broker/*, docs/examples/*.
- Remove memory-related tests across all test files.

**Breaking change:** Any automation or script that calls `acm memory` will fail. Adopt AMM before upgrading to Phase 3.

## Files Affected by Full Removal (Phase 2 + 3)

For reference, the complete set of files that will be modified or have memory-related code removed across all phases:

| Layer | Files |
|---|---|
| Contracts/Types | `internal/contracts/v1/types.go`, `internal/contracts/v1/validate.go`, `internal/contracts/v1/command_catalog.go` |
| Command Dispatch | `internal/commands/dispatch.go` |
| Core Interfaces | `internal/core/service.go`, `internal/core/repository.go` |
| Service Backend | `internal/service/backend/memory.go`, `internal/service/backend/context.go`, `internal/service/backend/fetch.go`, `internal/service/backend/export.go`, `internal/service/backend/maintenance.go`, `internal/service/backend/completion.go`, `internal/service/backend/status.go` |
| Storage Domain | `internal/storage/domain/domain.go` |
| SQLite Adapter | `internal/adapters/sqlite/repository.go`, `internal/adapters/sqlite/migrations.go` |
| Postgres Adapter | `internal/adapters/postgres/repository.go`, `internal/adapters/postgres/queries.go`, `internal/adapters/postgres/migrations/0001_acm_foundation.sql`, `internal/adapters/postgres/migrations/0002_acm_propose_memory.sql` |
| HTTP Adapter | `internal/adapters/http/handler.go` |
| MCP Adapter | `internal/adapters/mcp/invoke.go` |
| CLI | `cmd/acm/convenience.go`, `cmd/acm/routes.go` |
| Tests | `internal/service/backend/service_test.go`, `internal/service/backend/export_test.go`, `internal/adapters/sqlite/repository_parity_test.go`, `internal/adapters/sqlite/repository_parity_constraints_test.go`, `internal/adapters/postgres/queries_test.go`, `internal/integration/postgres_step10_integration_test.go`, `internal/adapters/http/handler_test.go`, `internal/adapters/mcp/invoke_test.go`, `cmd/acm/convenience_test.go` |
| Config | `.acm/acm-tags.yaml`, `.acm/acm-rules.yaml` |
| Documentation | `README.md`, `docs/getting-started.md`, `docs/concepts.md`, `docs/maintainer-reference.md`, `docs/release-notes/RELEASE_NOTES_1.0.0.md`, `skills/acm-broker/SKILL.md`, `skills/acm-broker/references/templates.md`, `skills/acm-broker/claude/commands/acm-memory.md` |
