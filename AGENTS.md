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

For code changes, include `verify:tests`. Use `review --run` for runnable workflow gates.

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

## Working Norms

- `.acm/context.db*` are local runtime state — stay in `.gitignore`.
- Prefer small, reviewable changes over broad cleanup.
- Do not invent compatibility promises or product requirements the repo doesn't define.
- If verification fails, fix it or report it. Do not claim success.
