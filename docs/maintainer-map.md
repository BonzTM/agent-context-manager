# Maintainer Map

Purpose: route maintainers and agents to the right files for common ACM changes.
Audience: maintainers working on ACM itself.
Update when: a change type gains new cross-file obligations or new verification expectations.
Not for: end-user setup or full architecture explanation.

Read [AGENTS.md](../AGENTS.md) first. Use this file when you know what kind of change you are making but do not want to rediscover every sync surface from scratch.

## Documentation Policy

- Keep the repo-root `AGENTS.md` as the fast path only.
- Keep cross-cutting routing metadata here instead of scattering subsystem docs through `internal/**`.
- Document obligations that code search does not reveal cheaply: parity, sync surfaces, workflow gates, and verification expectations.
- If a fact can be rediscovered quickly with search, prefer search over prose.

## Change Map

| Change Type | Start Here | Also Update | Verify Or Confirm |
|---|---|---|---|
| Command payload, result, or behavior change | `internal/contracts/v1/types.go`, `internal/contracts/v1/validate.go`, `internal/contracts/v1/command_catalog.go`, the matching file in `internal/service/backend/` | `spec/v1/cli.command.schema.json`, `spec/v1/cli.result.schema.json`, `spec/v1/mcp.tools.v1.json` when required by tests, `internal/commands/dispatch.go`, `cmd/acm/routes.go`, `cmd/acm/convenience.go`, `internal/core/service.go`, tests | `go test ./cmd/acm ./cmd/acm-mcp ./internal/contracts/v1 ./internal/commands ./internal/service/backend` |
| CLI flags, help, or subcommand routing | `cmd/acm/convenience.go`, `cmd/acm/routes.go`, `cmd/acm/main.go` | `internal/contracts/v1/command_catalog.go`, `cmd/acm/main_test.go`, `cmd/acm/convenience_test.go`, `cmd/acm/routes_test.go`, user-facing docs if behavior changed | `go test ./cmd/acm` |
| MCP tool metadata or adapter behavior | `internal/adapters/mcp/invoke.go`, `cmd/acm-mcp/main.go` | `internal/contracts/v1/command_catalog.go`, `cmd/acm-mcp/main_test.go`, spec files if metadata changed | `go test ./cmd/acm-mcp ./internal/adapters/mcp ./internal/contracts/v1` |
| Service logic without wire changes | matching file in `internal/service/backend/` | tests next to the package, docs only if user-visible behavior changed | `go test ./internal/service/backend` |
| Storage, migrations, or plan persistence | `internal/core/repository.go`, `internal/adapters/sqlite/`, `internal/adapters/postgres/`, `internal/storage/domain/` | both migration sets, `internal/testutil/repositorycontract/`, SQLite parity tests, Postgres integration tests when needed | `go test ./internal/adapters/sqlite ./internal/adapters/postgres ./internal/testutil/repositorycontract ./internal/storage/domain` and `go test ./internal/integration/...` with `ACM_PG_DSN` when required |
| Review, completion, or workflow-gate semantics | `.acm/acm-workflows.yaml`, `internal/service/backend/review.go`, `internal/service/backend/completion.go`, `internal/service/backend/work.go`, repo-local scripts under `scripts/` | `README.md`, `docs/getting-started.md`, `docs/examples/*`, `skills/acm-broker/**` | `go test ./internal/service/backend ./scripts/...` and `acm review --run` when appropriate |
| Verification selection or baseline checks | `.acm/acm-tests.yaml`, `internal/service/backend/verify.go` | docs when operator expectations changed, repo-local scripts if test commands changed | `acm verify --phase review --dry-run`, then the selected real verification |
| Rules, tags, or context loading behavior | `.acm/acm-rules.yaml`, `.acm/acm-tags.yaml`, `internal/service/backend/{context,ruleset,tags}.go` | `docs/getting-started.md`, examples or template guidance if operator expectations changed | `acm status --task-text "<task>" --phase execute`, `acm context --task-text "<task>" --phase execute`, `go test ./internal/service/backend` |
| Init, sync, status, health, or onboarding | `internal/bootstrap/`, `internal/service/backend/{maintenance,status}.go`, `README.md`, `docs/getting-started.md` | `docs/examples/*`, template files under `internal/bootstrap/bootstrap_templates/`, `skills/acm-broker/**` | `acm init`, `acm sync --mode working_tree --insert-new-candidates`, `acm health --include-details`, `acm status` |
| Staged planning contract | `docs/feature-plans.md`, `scripts/acm-feature-plan-validate.py`, `.acm/acm-tests.yaml`, maintainer guidance | `AGENTS.md`, docs that explain orchestration or verify/review boundaries, examples or templates only when product-facing expectations change | `python3 scripts/acm-feature-plan-validate.py --help`, then `acm verify` on a governed plan task; in this repo, governed multi-step work is required to use that staged contract |
| Maintainer docs only | `AGENTS.md`, `docs/maintainer-map.md`, `docs/maintainer-reference.md` | `README.md` and `docs/getting-started.md` only when boundaries or expectations for maintainers vs adopters changed | `go test ./...` if docs describe changed behavior; otherwise spot-check links and commands |

## High-Value Boundaries

- Product-facing docs live in `README.md` and `docs/getting-started.md`.
- Repo-maintainer fast path lives in `AGENTS.md`.
- Repo-maintainer slow-path reference lives in [docs/maintainer-reference.md](maintainer-reference.md).
- Generic starter templates live in `docs/examples/` and `internal/bootstrap/bootstrap_templates/`; they should explain the pattern without pretending to be this repo's contract.
