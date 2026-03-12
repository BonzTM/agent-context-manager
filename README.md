# acm — Modular Control Plane for AI Coding Agents

acm is a repo-owned control plane for AI coding agents. It gives Claude, Codex, and MCP clients shared durable state outside any one model session or vendor surface.

- **Rules and receipts replace giant always-loaded markdown** — hard rules are carried in task receipts, not left to best-effort prompt compliance.
- **Work plans survive context loss and agent handoffs** — plans and tasks live in SQLite or Postgres, so a later `context` call can resume the real state of the work.
- **Durable memory is shared across agents** — evidence-backed memory stays with the repo workflow instead of disappearing into vendor-local memory features.
- **Closure is auditable** — `verify`, `review`, and `done` record what was checked, what was required, and what closed the task.
- **`context` hydrates work without replacing native search** — acm frames the task with rules, active work, durable memory, and any explicitly known initial scope while the agent still uses its own file-reading tools.

acm is intentionally modular. You can adopt only the pieces you need.

```mermaid
flowchart TB
    subgraph human [" "]
        direction LR
        init["🔧 acm init"] --> configure["Configure<br/>rules · tags · tests · workflows"]
        configure --> sync["acm sync"]
    end

    subgraph agent [" "]
        direction TB
        context["acm context"] --> fetch["acm fetch"]
        fetch --> work["acm work"]
        work --> code["Execute Work"]
        code --> verify["acm verify"]
        verify --> review["acm review"]
        review --> done["acm done"]
        done -.-> memory["acm memory"]
        memory -.-> context
    end

    subgraph monitor [" "]
        direction LR
        web["📊 acm-web"]
        health["acm health · acm status"]
    end

    db[("SQLite / Postgres")]

    sync --> db
    context <--> db
    work --> db
    done --> db
    memory --> db
    web <--> db
    health <--> db

    style human fill:transparent,stroke:#58a6ff,stroke-width:1px,stroke-dasharray:5 5
    style agent fill:transparent,stroke:#3fb950,stroke-width:1px,stroke-dasharray:5 5
    style monitor fill:transparent,stroke:#d29922,stroke-width:1px,stroke-dasharray:5 5
```

## Adoption Modes

- **plans-only**: use `init`, `context`, and `work` when the main need is durable task state that survives compaction or cross-agent handoff.
- **plans+memory**: add `memory` when you want repo-owned facts and decisions that Claude and Codex can both reuse.
- **governed workflow**: add `verify`, `review`, and `done` when you want explicit completion gates and audit history.
- **full brokered flow**: use `context`, explicit `fetch` / history hydration, and governed review/closeout when you also want compact always-loaded context and receipt-scoped execution.

The repo you are reading uses a stricter dogfood workflow than many adopters need. `acm init` starts with the minimal core and lets you opt into heavier templates later.
acm is unreleased. The public contract is still being cleaned up around this smaller core surface, so prefer the current `init/context/work/memory/verify/done` story over older compatibility aliases when they disagree.

## Install

Preferred install path:

```bash
go install github.com/bonztm/agent-context-manager/cmd/acm@latest
go install github.com/bonztm/agent-context-manager/cmd/acm-mcp@latest
go install github.com/bonztm/agent-context-manager/cmd/acm-web@latest
```

Go installs binaries to `$GOBIN` if it is set, otherwise to `$(go env GOPATH)/bin` (typically `~/go/bin`). That directory must be on your `PATH`.

```bash
export PATH="$(go env GOPATH)/bin:$PATH"
```

If you want prebuilt binaries instead, download the `acm-binaries` artifact from a successful `Go Build` GitHub Actions run and place `acm`, `acm-mcp`, and `acm-web` on your `PATH`.

If you are working from a checkout, build from source:

```bash
git clone https://github.com/bonztm/agent-context-manager.git
cd agent-context-manager
go build -o dist/acm ./cmd/acm
go build -o dist/acm-mcp ./cmd/acm-mcp
go build -o dist/acm-web ./cmd/acm-web
```

## Quick Start (5 minutes)

### 1. Initialize your project

Scan your repo, seed repo-local ACM files, and materialize the initial ACM inventory:

```bash
acm init
```

Init respects `.gitignore` by default. It also:

- Seeds `.acm/acm-rules.yaml`, `.acm/acm-tags.yaml`, `.acm/acm-tests.yaml`, and `.acm/acm-workflows.yaml` when missing
- Appends `.acm/context.db`, `.acm/context.db-shm`, and `.acm/context.db-wal` to `.gitignore`
- Creates or extends `.env.example`
- Auto-indexes discovered repo files into pointer stubs so `fetch`, health, memory evidence, and governed scope checks work immediately

Use `--persist-candidates` to save the enumerated file list to `.acm/init_candidates.json`.

When `--project` is omitted, acm resolves the project namespace from `ACM_PROJECT_ID` first and otherwise infers it from the repo root folder name. Pass `--project` explicitly when you want a stable namespace that differs from the folder name.

If you want a heavier starter, rerun `init` with one or more additive templates:

```bash
acm init \
  --apply-template starter-contract \
  --apply-template verify-generic \
  --apply-template claude-command-pack \
  --apply-template claude-hooks \
  --apply-template git-hooks-precommit
```

`--apply-template` is repeatable and safe to re-run. Templates only create missing files, upgrade pristine scaffolds, and merge additive JSON fragments (e.g. `.claude/settings.json`). They never delete files or overwrite files you've edited.
Add `--apply-template codex-pack` when you want repo-local Codex companion docs under `.codex/acm-broker/`.

Starter verify profiles:

- `verify-generic` — language-agnostic `.acm/acm-tests.yaml` that works out of the box
- `verify-go` — Go-oriented `.acm/acm-tests.yaml`
- `verify-ts` — TypeScript-oriented `.acm/acm-tests.yaml`
- `verify-python` — Python-oriented `.acm/acm-tests.yaml`
- `verify-rust` — Rust-oriented `.acm/acm-tests.yaml`

Planning profile:

- `detailed-planning-enforcement` — seeds `docs/feature-plans.md` and `scripts/acm-feature-plan-validate.py`, and upgrades pristine `starter-contract` / `verify-generic` scaffolds to the richer feature-planning workflow

Tooling companions:

- `codex-pack` — seeds `.codex/acm-broker/README.md` and `.codex/acm-broker/AGENTS.example.md` so Codex has repo-local companion docs in addition to the global skill install
- `claude-command-pack` — seeds `.claude/commands/*` and `.claude/acm-broker/*`
- `claude-hooks` — seeds `.claude/hooks/acm-receipt-guard.sh`, `.claude/hooks/acm-receipt-mark.sh`, `.claude/hooks/acm-session-context.sh`, `.claude/hooks/acm-edit-state.sh`, and `.claude/hooks/acm-stop-guard.sh` to inject the ACM loop at session start, block edits until `/acm-context` succeeds, require `/acm-work` before untracked multi-file edits, and block stop until edits are reported
- `git-hooks-precommit` — seeds `.githooks/pre-commit` for staged-file `acm verify` gating; enable with `git config core.hooksPath .githooks`

### 2. Fill in your seeded rules

Init creates `.acm/acm-rules.yaml` if it does not already exist. Replace the blank scaffold with your project rules:

```yaml
version: acm.rules.v1
rules:
  - id: rule_context_first
    summary: Always call context before reading or editing files.
    enforcement: hard
    tags: [startup]

  - id: rule_done
    summary: Close every task with done.
    enforcement: hard
    tags: [completion]

  - id: rule_verify_before_completion
    summary: Run verify before done when code changes.
    enforcement: hard
    tags: [verification]
```

### 3. Sync rules into acm

```bash
acm sync --mode working_tree
```

### 4. Set up agent integration

Wire agents to acm via slash commands, skill packs, or MCP tools — see [Getting Started](docs/getting-started.md) for adopter setup details.

Once connected, most adopters mainly use `context`, `work`, `verify`, `done`, and `memory`, with `fetch`, `review`, and `history` as supporting surfaces. The advanced backend-only `export` surface is available through `acm run` or MCP when you need stable JSON or Markdown artifact rendering. You can test any operation manually via CLI (e.g., `acm context --task-text "fix the login bug" --phase execute`). See the [CLI Reference](#cli-reference) below.

If you are maintaining ACM itself rather than adopting it in another repo, use [AGENTS.md](AGENTS.md), [docs/maintainer-map.md](docs/maintainer-map.md), and [docs/maintainer-reference.md](docs/maintainer-reference.md) for the repo's maintainer workflow. This README stays product-facing.

## Agent Integration

### Claude Code (slash commands)

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/bonztm/agent-context-manager/main/scripts/install-skill-pack.sh) --claude
```

Run this from your project root. It installs `/acm-context`, `/acm-work`, `/acm-review`, `/acm-verify`, `/acm-done`, and `/acm-memory` slash commands into `.claude/commands/`.

If you already have this repo checked out locally, the equivalent command is `./scripts/install-skill-pack.sh --claude`.

### Codex (skill pack)

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/bonztm/agent-context-manager/main/scripts/install-skill-pack.sh) --codex
```

Installs the acm-broker skill to `~/.codex/skills/acm-broker` so Codex can share the same ACM plans, memory, verification state, and completion history used by Claude or MCP clients. The installed skill also includes `codex/README.md` and `codex/AGENTS.example.md` companion docs.

If you already have this repo checked out locally, the equivalent command is `./scripts/install-skill-pack.sh --codex`.

If you want repo-local Codex companion files in the project itself, also run:

```bash
acm init --apply-template codex-pack
```

That seeds `.codex/acm-broker/README.md` and `.codex/acm-broker/AGENTS.example.md`. Keep the repo-root `AGENTS.md` authoritative; the Codex companion files are there to make the full ACM loop explicit for Codex-driven repos, not to replace the root contract.

Codex can drive the same core workflow directly: `context`, `work`, `verify`, `review`, `done`, and `memory`. The absence of slash commands or Claude-style hooks is intentional; Codex relies on the installed skill, repo-root `AGENTS.md`, and normal CLI/MCP access.

### MCP (tool-native models)

```bash
acm-mcp invoke --tool context --in payload.json
```

Thirteen tools exposed:

- **Core agent-facing** (5): `context`, `work`, `memory`, `verify`, `done`
- **Supporting agent-facing** (3): `fetch`, `review`, `history`
- **Advanced backend-only** (1): `export`
- **Maintenance** (4): `sync`, `health`, `status`, `init`

`verify` and `review` are intentionally different:

- `verify` runs deterministic repo-defined executable checks from `.acm/acm-tests.yaml` and updates `verify:tests`.
- `review` satisfies one named workflow gate from `.acm/acm-workflows.yaml`; in run mode it executes that gate's `run` block and records review attempts for one review task such as `review:cross-llm`.

Use the commands for different questions:

| Question | Command |
|---|---|
| "Which repo-defined checks apply to this task and current diff?" | `verify` |
| "Has this one named workflow signoff gate been satisfied?" | `review` |

Typical governed closeout sequence: `work` -> `verify` -> `review --run` when required -> `done`.

## CLI Reference

All commands support `--help` for full flag documentation.

### Core Agent-Facing

```bash
acm context        [--project <id>] (--task-text <text>|--task-file <path>) [--phase <plan|execute|review>] [--tags-file <path>] [--scope-path <path>]...
acm work           [--project <id>] [--plan-key <key>|--receipt-id <id>] [--plan-title <text>] [--mode <merge|replace>] [--discovered-path <path>]... [--plan-file <path>|--plan-json <json>] [--tasks-file <path>|--tasks-json <json>]
acm memory         [--project <id>] --receipt-id <id> --category <cat> --subject <text> (--content <text>|--content-file <path>) --confidence <1-5> [--evidence-key <key>|--evidence-path <path>]... [--evidence-keys-file <path>|--evidence-keys-json <json>|--evidence-paths-file <path>|--evidence-paths-json <json>] [--related-key <key>|--related-path <path>]... [--related-keys-file <path>|--related-keys-json <json>|--related-paths-file <path>|--related-paths-json <json>] [--memory-tag <tag>]... [--memory-tags-file <path>|--memory-tags-json <json>] [--tags-file <path>] [--auto-promote]
acm verify        [--project <id>] [--receipt-id <id>] [--plan-key <key>] [--phase <plan|execute|review>] [--test-id <id>]... [--file-changed <path>]... [--files-changed-file <path>|--files-changed-json <json>] [--tests-file <path>] [--tags-file <path>] [--dry-run]
acm done           [--project <id>] --receipt-id <id> [--file-changed <path>]... [--files-changed-file <path>|--files-changed-json <json>] (--outcome <text>|--outcome-file <path>) [--scope-mode <strict|warn>] [--tags-file <path>]
```

`done` accepts omitted or empty `files_changed`. ACM computes the task delta from the receipt baseline when that baseline is available, so omission can mean "auto-detect the real delta" rather than only "no-file closure." When the detected delta is empty, the closeout is effectively no-file. When files are supplied explicitly, ACM cross-checks them against the detected delta, surfaces mismatches as violations, and still uses the detected delta as the source of truth for scope and completion-gate checks. For scope validation, `done` accepts receipt `initial_scope_paths`, plan `discovered_paths`, ACM-managed governance files, and path-like entries from `plan.in_scope` so feature-wide plans can close cleanly without forcing every owned path into the original receipt.

### Supporting Agent-Facing

```bash
acm fetch          [--project <id>] [--key <key>]... [--keys-file <path>|--keys-json <json>] [--expect <key=version>]... [--expected-versions-file <path>|--expected-versions-json <json>] [--receipt-id <id>]
acm review         [--project <id>] (--receipt-id <id>|--plan-key <key>) [--run] [--key <task-key>] [--summary <text>] [--status <pending|in_progress|complete|blocked|superseded>] [--outcome <text>|--outcome-file <path>] [--blocked-reason <text>] [--evidence <text>]... [--evidence-file <path>|--evidence-json <json>] [--tags-file <path>]
acm history        [--project <id>] [--entity <all|work|memory|receipt|run>] [--query <text>|--query-file <path>] [--scope <current|deferred|completed|all>] [--kind <kind>] [--limit <n>] [--unbounded]
```

If `--project` is omitted, convenience commands default to `ACM_PROJECT_ID` and otherwise infer the project from the effective repo root name. Explicit `--project` still wins.

For raw rendered artifacts, use the backend-only `export` command through `acm run` or MCP. Example request envelope: [docs/examples/export-request.json](docs/examples/export-request.json).

For ad hoc CLI rendering, `context`, `fetch`, `history`, and `status` also accept `--format json|markdown`, plus `--out-file` and `--force` for raw artifact output through the same backend export path. Example command: [docs/examples/context-export-command.txt](docs/examples/context-export-command.txt).

Most list and text flags support inline values and `--*-file` alternatives (`-` for stdin). JSON list/object inputs also support `--*-json` for one-shot agent calls without temporary files.

`review` is intentionally thin — it lowers to a single `work.tasks[]` merge update.

| Surface | `verify` | `review` |
|---|---|---|
| Source of truth | `.acm/acm-tests.yaml` | `.acm/acm-workflows.yaml` |
| Execution fan-out | zero or more selected checks | exactly one named gate |
| Recorded task | `verify:tests` | one review task such as `review:cross-llm` |
| Primary use | deterministic executable checks | workflow signoff or secondary reviewer gate |
| Wrong use | not a substitute for reviewer signoff | not a substitute for bulk repo checks |

**Defaults** (when flags are omitted): `key=review:cross-llm`, `summary="Cross-LLM review"`, `status=complete`.

**Run mode** (`--run` or `run=true`): acm loads the matching task from `.acm/acm-workflows.yaml`, executes its `run` block, persists an append-only review-attempt record, and updates the work-task snapshot. When fingerprint dedupe is enabled, ACM only skips reruns after a passing attempt already assessed the current fingerprint; failed or interrupted same-fingerprint attempts rerun until any configured `max_attempts` budget is exhausted. `done` requires a passing runnable review for the current fingerprint when dedupe is enabled. The scoped fingerprint covers effective scope: receipt `initial_scope_paths`, work-declared `discovered_paths`, and ACM-managed governance files that completion reporting already allows outside project file scope.
If the repo has uncommitted changes but the current effective scope captures none of them, the review runner should block and tell you to refresh `context` or declare the missing files through `work` rather than silently reviewing nothing.

**Manual mode** (no `--run`): use `--status`, `--outcome`, `--blocked-reason`, and `--evidence` to record a review note directly. Manual notes do not satisfy runnable review gates that define `run`, and these fields are ignored in run mode. Use `status=superseded` when a review gate or planned step is obsolete and should close cleanly instead of lingering as blocked.

Keep raw reviewer commands in repo-local scripts and workflow definitions, not maintainer prose. If a repo-local reviewer script needs model-specific settings, pass them through the workflow `run.argv` list.

**History discovery:** use `acm history` for both work-specific and multi-entity discovery. Set `--entity work` when you need `--scope` or `--kind`; use other entities for memories, receipts, or runs. Results include `fetch_keys` for follow-up `acm fetch`.

### Human-Facing Setup And Maintenance

```bash
acm init          [--project <id>] [--project-root .] [--apply-template <id>]... [--persist-candidates] [--respect-gitignore] [--output-candidates-path <path>] [--rules-file <path>] [--tags-file <path>]
acm sync          [--project <id>] --mode <changed|full|working_tree> [--git-range <range>] [--project-root <path>] [--insert-new-candidates] [--rules-file <path>] [--tags-file <path>]
acm health        [--project <id>] [--include-details] [--max-findings-per-check <n>] | [--fix <name>]... [--dry-run|--apply] [--project-root <path>] [--rules-file <path>] [--tags-file <path>]
acm status        [--project <id>] [--project-root <path>] [--rules-file <path>] [--tags-file <path>] [--tests-file <path>] [--workflows-file <path>] [--task-text <text>|--task-file <path>] [--phase <plan|execute|review>]
```

`acm health` is the only human CLI surface for repository health. Use it without `--fix` to inspect drift, add `--fix <name>` to apply a specific fixer by default, add `--dry-run` to preview without changing state, or use `--fix all` / `--apply` with no `--fix` values to run the default fixer set. It now warns on stale work plans, plans whose tasks are all terminal but whose plan status drifted, and plans left open only for administrative closeout. Run `acm health --help` to see the available fixers and preview/apply examples.

`acm status` is the preferred diagnostics surface. It reports the active project, runtime/backend details, loaded rules/tags/tests/workflows, installed init-managed integrations, missing setup, and non-blocking warnings for stale or drifted plans. With `--task-text`, it also previews the simplified `context` load.

### Optional Structured JSON Contract Mode

Use direct convenience commands for day-to-day work. `acm run` and `acm validate` operate on the full `acm.v1` request envelope when you want:

- One complete JSON request per call (scripts, CI)
- Request fixtures checked into a repo for repeatable workflows
- Payload validation before execution

Envelope shape:

```json
{
  "version": "acm.v1",
  "command": "context",
  "request_id": "req-context-001",
  "payload": {
    "project_id": "my-cool-app",
    "task_text": "add input validation to the signup form",
    "phase": "execute"
  }
}
```

Run or validate it with:

```bash
acm run --in request.json
acm validate --in request.json
```

MCP tools use the same payload schema but omit the outer envelope because the tool name already identifies the command. See [Schema Reference](spec/v1/README.md) and [skills/acm-broker/assets/requests](skills/acm-broker/assets/requests) for worked request examples.
Structured payloads may omit `project_id` when runtime defaults are configured; `acm run`, `acm validate`, and `acm-mcp invoke` resolve it in the same order as convenience commands.

## Web Dashboard

`acm-web` is a read-only web dashboard that gives humans a live view of what agents are working on. It reuses the same `core.Service` and storage backend as `acm` and `acm-mcp`, bundled into a single binary via `go:embed`.

### Running

```bash
acm-web                       # starts on :8080
acm-web serve --addr :9090    # custom port
```

### Pages

| Page | URL | Description |
|---|---|---|
| Board | `/` | Kanban board with Pending, In Progress, Blocked, and Done columns. Tasks are tree-sorted so children appear beneath their parent. Click any card for a detail modal with navigable parent/child/dependency links and rolled-up progress for parent tasks. |
| Memories | `/memories.html` | All durable memories with category, content, and confidence. |
| Status | `/status.html` | Project info, loaded sources, installed integrations, and warnings. |
| Health | `/healthz` | JSON liveness probe for k8s readiness/liveness checks. |

The board supports a scope toggle (Current / Completed / All) and polls the API every 10 seconds.

### Configuration

`acm-web` reads the same environment variables as `acm` and `acm-mcp` (`ACM_PROJECT_ID`, `ACM_PG_DSN`, `ACM_SQLITE_PATH`, etc.). No additional configuration is needed beyond what you already have for the CLI.

### Docker

A `Dockerfile.acm-web` is provided for containerized deployment:

```bash
docker build -f Dockerfile.acm-web -t acm-web .
docker run -p 8080:8080 -e ACM_PG_DSN='...' acm-web
```

## Storage Backend

SQLite is zero-config by default. acm resolves config in this order:

1. Process environment (`ACM_*`)
2. Explicit `--project` / `project_id` wins when provided
3. Otherwise `ACM_PROJECT_ID` sets the default project namespace
4. Otherwise `ACM_PROJECT_ROOT` pins the repo root when running acm from another directory and the repo-root name is inferred
5. Repo-root `.env` is loaded when present
6. If `ACM_PG_DSN` is set, Postgres is used
7. Otherwise SQLite defaults to `<repo-root>/.acm/context.db`

Init scaffolding is responsible for adding the implicit SQLite files to `.gitignore` when you want repo-local setup materialized.

Set `ACM_PG_DSN` for Postgres when you need write concurrency.

```bash
# Optional stable namespace override when folder names vary
export ACM_PROJECT_ID=my-cool-app

# SQLite override
export ACM_SQLITE_PATH=/path/to/context.db

# Postgres
export ACM_PG_DSN='postgres://user:pass@localhost:5432/agents_context?sslmode=disable'
```

See [SQLite Operations](docs/sqlite.md) for deployment, backup, and rotation guidance.

## Documentation

User guides:

- [Getting Started](docs/getting-started.md) — full walkthrough from zero to working acm setup
- [Web Dashboard](#web-dashboard) — read-only kanban board, memories, and status pages
- [Concepts](docs/concepts.md) — what pointers, receipts, rules, memories, plans, and tags are
- [SQLite Operations](docs/sqlite.md) — deployment, backup, and rotation
- [Schema Reference](spec/v1/README.md) — v1 wire contract schemas
- [Skill Templates](skills/acm-broker/references/templates.md) — request/response examples

Architecture (contributors):

- [Logging Standards](docs/logging.md) — structured logging contract

## Configuration Files

acm doesn't ship project rules or opinions. You author configuration in repo-local YAML files, and acm discovers, ingests, and enforces them.

### Rules (`.acm/acm-rules.yaml`)

Define behavioral constraints for agents. Hard rules are always included in receipts; soft rules are summary-only. Use `--rules-file` on `sync`, `health --fix`, or `init` to override auto-discovery.

### Tags (`.acm/acm-tags.yaml`)

Repo-local canonical tag aliases that extend acm's embedded base dictionary. Merged on every runtime call. Use `--tags-file` on any command that does tag normalization to override.

### Verification (`.acm/acm-tests.yaml`)

Repo-defined executable checks for `verify`. v1 definitions are argv-only. Use `--tests-file` on `verify` to override auto-discovery.

When verify context is available, ACM also injects generic metadata for repo-local scripts:

- `ACM_RECEIPT_ID` and `ACM_PLAN_KEY`
- `ACM_VERIFY_PHASE`
- `ACM_VERIFY_TAGS_JSON`
- `ACM_VERIFY_FILES_CHANGED_JSON`

That metadata is policy-neutral. Repos can use it for targeted test selection, plan-aware guards, or other local workflow checks without making those policies part of ACM's product defaults.

### Repo-Local Staged Plan Conventions

ACM's built-in `work` schema already supports richer planning detail through `plan.stages`, `parent_task_key`, `depends_on`, and `acceptance_criteria`. Repos can layer a stricter feature-planning contract on top of those fields without changing ACM itself.

This repo does that for governed multi-step work: root plans use `kind=feature`, `kind=maintenance`, or `kind=governance`, track `spec_outline` / `refined_spec` / `implementation_plan`, group work under top-level `stage:*` tasks, and treat leaf tasks with exact `references` plus explicit `acceptance_criteria` as the atomic units of execution. Terminal plan auto-close also reconciles the plan-stage fields from those `stage:*` task statuses so completed staged plans do not linger with stale stage metadata. `acm verify` enforces the contract through `scripts/acm-feature-plan-validate.py`. That stricter staged-plan schema is repo policy, not an ACM product default. See [docs/feature-plans.md](docs/feature-plans.md).

### Workflows (`.acm/acm-workflows.yaml`)

Completion gates that control which work task keys must be satisfied before `done` succeeds. Runnable review gates can define `max_attempts` and `rerun_requires_new_fingerprint` for bounded final-gate retries. When no workflow gates are configured, acm falls back to requiring `verify:tests`.

### Init Templates

Templates are seed-only — they create missing files but never overwrite edited ones. Built-ins:

| Template | What it seeds |
|---|---|
| `starter-contract` | `AGENTS.md`, `CLAUDE.md`, richer starter ruleset |
| `detailed-planning-enforcement` | Richer feature-plan contract, `docs/feature-plans.md`, and `scripts/acm-feature-plan-validate.py` |
| `verify-generic` | Language-agnostic `.acm/acm-tests.yaml` that works out of the box |
| `verify-go` | Go-oriented `.acm/acm-tests.yaml` |
| `verify-ts` | TypeScript-oriented `.acm/acm-tests.yaml` |
| `verify-python` | Python-oriented `.acm/acm-tests.yaml` |
| `verify-rust` | Rust-oriented `.acm/acm-tests.yaml` |
| `codex-pack` | `.codex/acm-broker/README.md`, `.codex/acm-broker/AGENTS.example.md` |
| `claude-command-pack` | `.claude/commands/*`, `.claude/acm-broker/*` |
| `claude-hooks` | Claude hook settings plus ACM process guard scripts |
| `git-hooks-precommit` | `.githooks/pre-commit` |

See [docs/examples/init-templates.md](docs/examples/init-templates.md) for usage examples. Format references: [acm-rules.yaml](docs/examples/acm-rules.yaml), [acm-tags.yaml](docs/examples/acm-tags.yaml), [acm-workflows.yaml](docs/examples/acm-workflows.yaml). For a concrete richer planning contract layered on top of ACM's built-in schema, see [docs/feature-plans.md](docs/feature-plans.md). Full authoring workflow: [Getting Started](docs/getting-started.md).

## Environment Variables

```bash
export ACM_PROJECT_ID=my-cool-app      # optional stable project namespace
export ACM_PROJECT_ROOT=/path/to/repo  # optional when running acm from another directory
export ACM_UNBOUNDED=false             # true removes built-in history/list caps for supported surfaces
export ACM_LOG_LEVEL=debug             # debug|info|warn|error (default: info)
export ACM_LOG_SINK=stderr             # stderr|stdout|discard (default: stderr)
```

## License

This project is licensed under the Apache License, Version 2.0 (`Apache-2.0`). See [LICENSE](LICENSE).
