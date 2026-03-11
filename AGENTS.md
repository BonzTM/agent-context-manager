# AGENTS.md - agent-context-manager Maintainer Contract

This is the single authoritative operating contract for all agent work in this repository.
Tool-specific companions (e.g. `CLAUDE.md`) add only tool-specific mappings and defer here for everything else.
After compaction, reset, or handoff, re-read this file and restart the ACM task loop.

## Architecture

### What ACM is

A repo-owned control plane for AI coding agents. It gives Claude, Codex, and MCP clients shared durable state outside any single model session.

### System diagram

```
┌─────────────────────────────────────────────────────────────────────┐
│                        Agent (Claude / Codex / MCP client)          │
│                                                                     │
│   context ──► work ──► verify ──► review ──► done ──► memory        │
│      │                    │          │                    │         │
│      ▼                    ▼          ▼                    ▼         │
│   receipt              plan/tasks  test results        completion   │
└─────────┬──────────────────────────┬────────────────────────────────┘
          │                          │
          ▼                          ▼
┌──────────────────┐    ┌─────────────────────────────┐
│  Repo-local YAML │    │  ACM Service                │
│                  │    │                             │
│  acm-rules.yaml  │───►│  commands/dispatch.go       │
│  acm-tags.yaml   │    │       ▼                     │
│  acm-tests.yaml  │    │  service/backend/*.go       │
│  acm-workflows   │    │       ▼                     │
│                  │    │  ┌─────────┬──────────┐     │
└──────────────────┘    │  │ SQLite  │ Postgres │     │
                        │  │ adapter │ adapter  │     │
                        │  └─────────┴──────────┘     │
                        └─────────────────────────────┘
```

### Entrypoints

- `cmd/acm/` — CLI binary. Convenience subcommands + structured `run`/`validate` envelope mode.
- `cmd/acm-mcp/` — MCP adapter. Same 12 operations as CLI, tool-native JSON.

### Key packages

```
internal/
  contracts/v1/   — payload types, validation, command catalog, JSON schemas
  commands/       — dispatch: command → service method
  core/           — Service interface, Repository interface, domain errors
  service/backend/— all business logic (context, work, verify, done, etc.)
  adapters/
    sqlite/       — SQLite repository implementation
    postgres/     — Postgres repository implementation
    cli/          — CLI adapter (run/validate envelope)
    mcp/          — MCP adapter (invoke)
  runtime/        — config resolution, service factory, logger
  bootstrap/      — init templates and scaffold logic
  storage/domain/ — shared storage domain types
  workspace/      — repo root detection, .env loading
```

### Invariants an agent should know

- **12 commands, 1 interface**: every command goes through `core.Service` (service.go:9-22). The service interface IS the API.
- **Schema lockstep**: `internal/contracts/v1/` types ↔ `spec/v1/*.json` schemas ↔ tests. Change one, change all three.
- **Storage parity**: SQLite and Postgres adapters implement `core.Repository`. Both must pass the same contract tests in `internal/testutil/repositorycontract/`.
- **CLI/MCP parity**: both surfaces dispatch through `internal/commands/dispatch.go`. Same payloads, same results.

### Adding or changing a command (checklist)

Every command touches these locations. Miss one and a parity or schema test will fail.

1. **Command constant** — `internal/contracts/v1/types.go` (add to `Command` const block)
2. **Payload + result types** — `internal/contracts/v1/types.go` (struct definitions)
3. **Validation / decode** — `internal/contracts/v1/validate.go` (add `decode*Payload` func)
4. **Command catalog entry** — `internal/contracts/v1/command_catalog.go` (wire spec: CLI usage, MCP title/description, schema refs, decode func)
5. **CLI command schema** — `spec/v1/cli.command.schema.json` (add `$defs/<command>Payload`)
6. **CLI result schema** — `spec/v1/cli.result.schema.json` (add `$defs/<command>Result`)
7. **Service interface** — `internal/core/service.go` (add method)
8. **Business logic** — `internal/service/backend/<command>.go` (implement the method)
9. **Command dispatch** — `internal/commands/dispatch.go` (add to `handlers` map)
10. **CLI routing** — `cmd/acm/routes.go` (add case in `canonicalRouteBuilder`)
11. **CLI flag parsing** — `cmd/acm/convenience.go` (add `build<Command>Envelope` func)
12. **Storage** — if the command needs new persistence: both `internal/adapters/sqlite/` and `internal/adapters/postgres/`, plus `internal/core/repository.go` interface

The MCP adapter (`internal/adapters/mcp/invoke.go`) auto-generates tool definitions from the command catalog — no manual MCP wiring needed.

### Test patterns

| Pattern | Location | When to use |
|---|---|---|
| **Unit tests** | `*_test.go` next to source | Default. Test a single package in isolation. |
| **Repository contract tests** | `internal/testutil/repositorycontract/` | Shared test suite that both SQLite and Postgres adapters must pass. Add cases here for new repository behavior. |
| **Parity constraint tests** | `internal/adapters/sqlite/repository_parity_*_test.go` | SQLite-specific assertions that the contract tests don't cover (e.g. migration edge cases). |
| **Integration tests** | `internal/integration/*_test.go` | Require a live Postgres instance (`ACM_PG_DSN`). Test cross-adapter parity at the service level. These don't run in normal `go test ./...`. |
| **Schema/spec drift tests** | `cmd/acm-mcp/main_test.go`, `internal/contracts/v1/schema_files_test.go` | Assert that runtime Go types, command catalog, and `spec/v1/*.json` files stay in sync. These break when you change a command but miss a schema file. |
| **CLI envelope tests** | `cmd/acm/main_test.go`, `cmd/acm/convenience_test.go`, `cmd/acm/routes_test.go` | Assert that CLI flag parsing produces the expected envelopes and that subcommand routing is complete. |

Run `go test ./...` for the full local suite (excludes integration tests).
Run `go test ./internal/integration/...` separately with `ACM_PG_DSN` set for Postgres parity.

## Source Of Truth

- This file is the top-level contract. Tool companions defer here.
- Canonical rules: `.acm/acm-rules.yaml` (delivered to agents via `context` receipts)
- Canonical tags: `.acm/acm-tags.yaml`
- Canonical verification: `.acm/acm-tests.yaml`
- Canonical workflow gates: `.acm/acm-workflows.yaml`
- `docs/examples/` contains generic starter templates, not this repo's contract.

## Task Loop

Complete before making code changes:

1. Read this file. If Claude, also read `CLAUDE.md`.
2. `acm context --project agent-context-manager --task-text "<task>" --phase <plan|execute|review>`
3. Follow the returned hard rules. Use `acm fetch` only for keys you need.
4. `acm work` when multi-step, multi-file, or handoff-prone.
5. `acm verify` for executable changes.
6. `acm review --run` when `.acm/acm-workflows.yaml` requires it.
7. `acm done`
8. `acm memory` for durable decisions.

If `acm` is not on `PATH`, fix the environment. Do not substitute `go run ./cmd/acm` unless testing source-build behavior.

## Repository Rules

Rules live in `.acm/acm-rules.yaml` and reach agents through receipts. Summary for human readers:

- **Schema lockstep**: `internal/contracts/v1` ↔ `spec/v1` ↔ tests move together.
- **CLI/MCP parity**: `cmd/acm` ↔ `cmd/acm-mcp` ↔ MCP tools ↔ tests move together.
- **Storage parity**: both adapters, both migration sets, both test suites.
- **Onboarding**: `init` must produce a usable first-run state from a clean repo.
- **Docs sync**: user-facing changes update README, docs, examples, and skill-pack assets together.

## When To Use work

- Task takes more than one material step
- More than one file or subsystem is involved
- Includes planning, verification, or handoff
- Needs durable state that survives compaction

For code changes, include `verify:tests`. Use `review --run` for runnable workflow gates. If a planned task or review gate becomes obsolete, mark it `superseded` instead of leaving it open or `blocked`.

## Feature Plans

Net-new feature work uses the contract in `docs/feature-plans.md`:
`kind=feature` root plans → `stage:*` tasks → leaf tasks with `acceptance_criteria`.
Enforced by `scripts/acm-feature-plan-validate.py` via `acm verify`.

## Verification

`.acm/acm-tests.yaml` defines what runs. Current baseline:

- `smoke` — always
- `cli-build` — `execute`/`review` for `cmd/**`, `internal/**`, `go.mod`, `go.sum`
- `full-go-suite` — `review` for broader product changes
- `acm-feature-plan-validate` — feature-relevant work

Update `.acm/acm-tests.yaml` if your change is not covered.

## Governance Maintenance

When changing governance, onboarding, or tool-surface behavior:

1. Update repo-local ACM files and product code.
2. `acm sync --project agent-context-manager --mode working_tree --insert-new-candidates`
3. `acm health --project agent-context-manager --include-details`
4. Update docs and skill-pack assets in the same change.

Keep these files coherent: `.acm/*.yaml`, `README.md`, `docs/getting-started.md`, `docs/examples/*`, `skills/acm-broker/**`.

`acm health` and `acm status` both warn on stale open plans, terminal-task plan status drift, and plans left open only for administrative closeout. Treat those warnings as bookkeeping regressions and clean them up before `done` when they are part of your task.

## Troubleshooting

| Symptom | Cause | Fix |
|---|---|---|
| `done` fails with scope violation | Changed files outside `initial_scope_paths` + `discovered_paths` | Declare the files via `work` with `plan.discovered_paths`, or re-run `context` with a broader task description. |
| `review --run` reports zero scoped files | Receipt or declared scope doesn't cover the files you changed | Same as above: expand scope via `work` or re-run `context`. |
| `verify` selects no tests | Changed paths don't match any `select.changed_paths_any` in `.acm/acm-tests.yaml` | Pass `--file-changed` explicitly, or add a selector to `.acm/acm-tests.yaml`. |
| `TestWriteToolsJSON_MatchesRuntimeAndSpec` fails | Command catalog or MCP tool metadata drifted from `spec/v1/mcp.tools.v1.json` | Regenerate the spec file or align the catalog entry. See the command checklist above. |
| `schema_files_test` fails | Go types in `contracts/v1/` drifted from `spec/v1/*.json` schemas | Update both the Go types and the JSON schema `$defs` together. |
| `smoke` passes but `full-go-suite` fails | A package outside the smoke subset has a regression | Run `go test ./...` locally to find it. Smoke only covers `cmd/acm`, `cmd/acm-mcp`, `internal/runtime`, `internal/service/backend`. |
| `context` returns stale or wrong rules | `.acm/acm-rules.yaml` was edited but not synced | Run `acm sync --mode working_tree` then `acm health`. |
| Receipt is too narrow for the real task | Task description was too specific or vague | Re-run `context` with a better `--task-text`. Do not guess. |

## Working Norms

- `.acm/context.db*` are local runtime state — stay in `.gitignore`.
- Prefer small, reviewable changes over broad cleanup.
- Do not invent compatibility promises or product requirements the repo doesn't define.
- If verification fails, fix it or report it. Do not claim success.
