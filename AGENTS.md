# AGENTS.md - agent-context-manager Maintainer Contract

This is the authoritative fast-path contract for work in this repository.
Read this file first, then use the linked maintainer docs when you need slower-path reference material.

## Purpose

- Use this file for the day-to-day task loop and repo-wide invariants.
- Use [docs/maintainer-map.md](docs/maintainer-map.md) when you know the change type but not the file set.
- Use [docs/maintainer-reference.md](docs/maintainer-reference.md) for architecture, package map, command checklists, test taxonomy, and troubleshooting.

## Source Of Truth

- This file is the top-level contract. Tool companions defer here.
- Canonical rules: `.acm/acm-rules.yaml`
- Canonical tags: `.acm/acm-tags.yaml`
- Canonical verification: `.acm/acm-tests.yaml`
- Canonical workflow gates: `.acm/acm-workflows.yaml`
- `docs/examples/` contains generic starter templates, not this repo's contract.

## Fast Path

1. Read this file. If Claude, also read `CLAUDE.md`.
2. If you need orientation before reading code, run `acm status --project agent-context-manager --task-text "<task>" --phase <plan|execute|review>`.
3. Run `acm context --project agent-context-manager --task-text "<task>" --phase <plan|execute|review>`.
4. Follow the returned hard rules. Use `acm fetch` only for keys you need.
5. Run `acm work` for multi-step, multi-file, handoff-prone, or governed-scope-expanding work.
6. Run `acm verify` for code, config, contract, onboarding, or behavior changes.
7. Run `acm review --run` when `.acm/acm-workflows.yaml` requires a runnable review gate.
8. Run `acm done`.

If `acm` is not on `PATH`, bootstrap the toolchain before continuing:

```bash
# 1. Ensure Go is available (requires sudo for /usr/local install)
if ! command -v go &>/dev/null; then
  curl -fsSL https://go.dev/dl/go1.26.0.linux-amd64.tar.gz -o /tmp/go.tar.gz \
    && sudo tar -C /usr/local -xzf /tmp/go.tar.gz \
    && rm /tmp/go.tar.gz
  export PATH="/usr/local/go/bin:$PATH"
fi

# 2. Build and install acm binaries from this repo
if ! command -v acm &>/dev/null; then
  go build -o /tmp/acm ./cmd/acm \
    && go build -o /tmp/acm-mcp ./cmd/acm-mcp \
    && sudo mv /tmp/acm /tmp/acm-mcp /usr/local/bin/
fi
```

Do not substitute `go run ./cmd/acm` unless you are explicitly testing source-build behavior.

When changing rules, tags, tests, workflows, onboarding, or tool-surface behavior, run `acm sync --project agent-context-manager --mode working_tree --insert-new-candidates` and `acm health --project agent-context-manager --include-details` before `done`.

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

- Do not silently widen governed scope. Re-run `context` or declare later-discovered files through `work.plan.discovered_paths`.
- Prefer small, reviewable changes over broad cleanup.
- Do not invent compatibility promises or product requirements the repo does not define.
- If verification fails, fix it or report it clearly. Do not claim success.
- If a planned task or review gate becomes obsolete, mark it `superseded` instead of leaving it open or `blocked`.
- `acm health` and `acm status` warnings about stale plans, plan-status drift, or plans left open only for administrative closeout are bookkeeping regressions. Clean them up before `done` when they are part of your task.
- Governed multi-step work in this repo uses the staged plan contract in `docs/feature-plans.md`, not thin ad hoc task lists.
- Governed root plans must always carry `spec_outline`, `refined_spec`, and `implementation_plan`; do not start implementation with only a vague root objective.
- The root plan owner acts as the orchestrator for multi-file or multi-step work: keep the whole-plan spec, scope, verification, review, and closeout there; keep leaf tasks narrow enough for low-context execution.
- When the runtime supports sub-agents, prefer delegating bounded leaf tasks so the orchestrator keeps the full-plan context. When it does not, execute the same leaf tasks sequentially and return to the root plan between them.
- Keep leaf tasks so tight that an assignee can succeed from the listed `references`, `acceptance_criteria`, and `depends_on` edges without inventing missing scope.
- Planned behavior-changing Go work under `cmd/**` or `internal/**` should add a `tdd:red` task before implementation, or a `tdd:exemption` task with a concrete justification.
- Repo-local verify treats non-test Go changes under `cmd/**` or `internal/**` as behavior changes unless that exemption is present.

## Slow Path Docs

- Architecture, package map, command checklist, test patterns, troubleshooting: [docs/maintainer-reference.md](docs/maintainer-reference.md)
- Detailed change routing: [docs/maintainer-map.md](docs/maintainer-map.md)
- Staged planning contract: [docs/feature-plans.md](docs/feature-plans.md)
- Product and adopter setup: [docs/getting-started.md](docs/getting-started.md)
- Schema and MCP contract overview: [spec/v1/README.md](spec/v1/README.md)
