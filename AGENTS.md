# AGENTS.md - agent-context-manager Contributor Contract

This is the authoritative contract for work in this repository.
Read this file first, then use the linked docs when you need deeper reference material.

## Quick Start

1. Read this file for repo rules and change routing.
2. See [CONTRIBUTING.md](CONTRIBUTING.md) for build, test, and PR workflow.
3. If your agent runtime provides ACM, see [.acm/AGENTS-ACM.md](.acm/AGENTS-ACM.md) for the enhanced workflow.  If you are unaware or unsure of what ACM is, do not read the file.
4. If using Claude, also read [CLAUDE.md](CLAUDE.md).

## Repo-Wide Invariants

- Schema lockstep: `internal/contracts/v1` and `spec/v1` move with the corresponding tests.
- CLI/MCP parity: `cmd/acm`, `cmd/acm-mcp`, the command catalog, generated tool metadata, and related tests stay aligned.
- Storage parity: SQLite and Postgres adapters, migrations, and parity-sensitive coverage stay aligned unless a difference is explicitly intentional and documented.
- Onboarding invariants: `init` must produce a usable first-run state from a clean repo.
- Docs sync: user-facing install, help, onboarding, or tool-surface changes update `README.md`, `docs/getting-started.md`, `docs/examples/*`, and `skills/acm-broker/**` together.

## Maintainer Routing

| If you are changing... | Start here | Also keep in sync | Verify or confirm with |
|---|---|---|---|
| Command payloads, results, or semantics | `internal/contracts/v1/`, `internal/service/backend/` | `spec/v1/`, `internal/commands/dispatch.go`, `cmd/acm/`, `cmd/acm-mcp/`, tests | [docs/maintainer-map.md](docs/maintainer-map.md) |
| CLI flags, help text, or routing | `cmd/acm/convenience.go`, `cmd/acm/routes.go` | `internal/contracts/v1/command_catalog.go`, `cmd/acm/*_test.go`, docs | [docs/maintainer-map.md](docs/maintainer-map.md) |
| Storage, migrations, or plan persistence | `internal/core/repository.go`, `internal/adapters/sqlite/`, `internal/adapters/postgres/` | repository contract tests, parity tests, integration tests | [docs/maintainer-map.md](docs/maintainer-map.md) |
| Review, verify, done, or workflow gates | `.acm/acm-workflows.yaml`, `internal/service/backend/{review,verify,completion,work}.go` | repo-local scripts, docs, examples, skill-pack assets | [docs/maintainer-map.md](docs/maintainer-map.md) |
| Init, sync, health, onboarding, or templates | `internal/bootstrap/`, `README.md`, `docs/getting-started.md` | `docs/examples/*`, template files, `skills/acm-broker/**` | [docs/maintainer-map.md](docs/maintainer-map.md) |
| Feature-planning contract | `docs/feature-plans.md`, `.acm/acm-tests.yaml`, `scripts/acm-feature-plan-validate.py` | maintainer docs, examples, workflow guidance | [docs/maintainer-map.md](docs/maintainer-map.md) |

## Working Norms

- Prefer small, reviewable changes over broad cleanup.
- Include tests for new behavior under `cmd/**` or `internal/**`.
- Do not invent compatibility promises or product requirements the repo does not define.
- If verification fails, fix it or report it clearly. Do not claim success.
- If a planned task or review gate becomes obsolete, mark it `superseded` instead of leaving it open or `blocked`.
- Planned behavior-changing Go work under `cmd/**` or `internal/**` should add a `tdd:red` task before implementation, or a `tdd:exemption` task with a concrete justification.
- Repo-local verify treats non-test Go changes under `cmd/**` or `internal/**` as behavior changes unless that exemption is present.

## Reference Docs

- Architecture, package map, command checklist, test patterns, troubleshooting: [docs/maintainer-reference.md](docs/maintainer-reference.md)
- Detailed change routing: [docs/maintainer-map.md](docs/maintainer-map.md)
- Staged planning contract: [docs/feature-plans.md](docs/feature-plans.md)
- Product and adopter setup: [docs/getting-started.md](docs/getting-started.md)
- Schema and MCP contract overview: [spec/v1/README.md](spec/v1/README.md)
