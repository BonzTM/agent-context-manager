# acm — Modular Control Plane for AI Coding Agents

acm is a repo-owned control plane for AI coding agents. It gives Claude, Codex, and MCP clients shared durable state outside any one model session or vendor surface.

- **Rules and receipts replace giant always-loaded markdown** — hard rules are carried in task receipts, not left to best-effort prompt compliance.
- **Work plans survive context loss and agent handoffs** — plans and tasks live in SQLite or Postgres, so a later `context` call can resume the real state of the work.
- **Durable memory is shared across agents** — evidence-backed memory stays with the repo workflow instead of disappearing into vendor-local memory features.
- **Closure is auditable** — `verify`, `review`, and `done` record what was checked, what was required, and what closed the task.
- **`context` hydrates work without replacing native search** — acm frames the task with rules, active work, durable memory, and any explicitly known initial scope while the agent still uses its own file-reading tools.

acm is intentionally modular. You can adopt only the pieces you need.

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
```

Go installs binaries to `$GOBIN` if it is set, otherwise to `$(go env GOPATH)/bin` (typically `~/go/bin`). That directory must be on your `PATH`.

```bash
export PATH="$(go env GOPATH)/bin:$PATH"
```

If you want prebuilt binaries instead, download the `acm-binaries` artifact from a successful `Go Build` GitHub Actions run and place `acm` and `acm-mcp` on your `PATH`.

If you are working from a checkout, build from source:

```bash
git clone https://github.com/bonztm/agent-context-manager.git
cd agent-context-manager
go build -o dist/acm ./cmd/acm
go build -o dist/acm-mcp ./cmd/acm-mcp
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

Wire agents to acm via slash commands, skill packs, or MCP tools — see [Getting Started](docs/getting-started.md) for details.

Once connected, most agents only need the core loop:

1. `context` — retrieves hard rules, active plans, relevant memory, any explicitly known initial scope paths, and a task baseline for later verify/done delta detection
2. `work` — records or updates durable task state
3. `verify` — runs repo-defined executable checks when code or behavior changed
4. `done` — closes the task, validates effective scope, and records completion history
5. `memory` — saves durable facts for future cross-agent reuse

When you change rules, tags, tests, workflows, onboarding, or tool-surface behavior, run `acm sync --mode working_tree --insert-new-candidates` and `acm health --include-details` before `done`.

Supporting surfaces stay available when you need them:

- `fetch` — hydrate full plan, task, memory, or explicit pointer content from returned keys
- `review` — record or run explicit review gates
- `history` — rediscover archived plans, receipts, runs, and memories

You can test any operation manually via CLI (e.g., `acm context --task-text "fix the login bug" --phase execute`). See the [CLI Reference](#cli-reference) below.

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

Twelve tools exposed:

- **Core agent-facing** (5): `context`, `work`, `memory`, `verify`, `done`
- **Supporting agent-facing** (3): `fetch`, `review`, `history`
- **Maintenance** (4): `sync`, `health`, `status`, `init`

`review` is intentionally thin — it records one review task through the existing `work` path and can execute a workflow-defined `run` block when requested.

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

`done` accepts omitted or empty `files_changed`. ACM computes the task delta from the receipt baseline when that baseline is available, so omission can mean "auto-detect the real delta" rather than only "no-file closure." When the detected delta is empty, the closeout is effectively no-file. When files are supplied explicitly, ACM cross-checks them against the detected delta, surfaces mismatches as violations, and still uses the detected delta as the source of truth for scope and completion-gate checks.

### Supporting Agent-Facing

```bash
acm fetch          [--project <id>] [--key <key>]... [--keys-file <path>|--keys-json <json>] [--expect <key=version>]... [--expected-versions-file <path>|--expected-versions-json <json>] [--receipt-id <id>]
acm review         [--project <id>] (--receipt-id <id>|--plan-key <key>) [--run] [--key <task-key>] [--summary <text>] [--status <pending|in_progress|complete|blocked>] [--outcome <text>|--outcome-file <path>] [--blocked-reason <text>] [--evidence <text>]... [--evidence-file <path>|--evidence-json <json>] [--tags-file <path>]
acm history        [--project <id>] [--entity <all|work|memory|receipt|run>] [--query <text>|--query-file <path>] [--scope <current|deferred|completed|all>] [--kind <kind>] [--limit <n>] [--unbounded]
```

If `--project` is omitted, convenience commands default to `ACM_PROJECT_ID` and otherwise infer the project from the effective repo root name. Explicit `--project` still wins.

Most list and text flags support inline values and `--*-file` alternatives (`-` for stdin). JSON list/object inputs also support `--*-json` for one-shot agent calls without temporary files.

`review` is intentionally thin — it lowers to a single `work.tasks[]` merge update.

**Defaults** (when flags are omitted): `key=review:cross-llm`, `summary="Cross-LLM review"`, `status=complete`.

**Run mode** (`--run` or `run=true`): acm loads the matching task from `.acm/acm-workflows.yaml`, executes its `run` block, persists an append-only review-attempt record, and updates the work-task snapshot. Runnable gates are terminal-gate by default — same-fingerprint reruns are skipped, `max_attempts` is optional, and `done` requires a fresh passing review when fingerprint dedupe is enabled. The scoped fingerprint covers effective scope: receipt `initial_scope_paths`, work-declared `discovered_paths`, and ACM-managed governance files that completion reporting already allows outside project file scope.
If the repo has uncommitted changes but the current effective scope captures none of them, the review runner should block and tell you to refresh `context` or declare the missing files through `work` rather than silently reviewing nothing.

**Manual mode** (no `--run`): use `--status`, `--outcome`, `--blocked-reason`, and `--evidence` to record a review note directly. These fields are ignored in run mode.

Keep raw reviewer commands in repo-local scripts and workflow definitions, not maintainer prose. If a repo-local reviewer script needs model-specific settings, pass them through the workflow `run.argv` list, for example `scripts/acm-cross-review.sh --model gpt-5.3-codex --reasoning-effort xhigh`.

**History discovery:** use `acm history` for both work-specific and multi-entity discovery. Set `--entity work` when you need `--scope` or `--kind`; use other entities for memories, receipts, or runs. Results include `fetch_keys` for follow-up `acm fetch`.

### Human-Facing Setup And Maintenance

```bash
acm init          [--project <id>] [--project-root .] [--apply-template <id>]... [--persist-candidates] [--respect-gitignore] [--output-candidates-path <path>] [--rules-file <path>] [--tags-file <path>]
acm sync          [--project <id>] --mode <changed|full|working_tree> [--git-range <range>] [--project-root <path>] [--insert-new-candidates] [--rules-file <path>] [--tags-file <path>]
acm health        [--project <id>] [--include-details] [--max-findings-per-check <n>] | [--fix <name>]... [--dry-run|--apply] [--project-root <path>] [--rules-file <path>] [--tags-file <path>]
acm status        [--project <id>] [--project-root <path>] [--rules-file <path>] [--tags-file <path>] [--tests-file <path>] [--workflows-file <path>] [--task-text <text>|--task-file <path>] [--phase <plan|execute|review>]
```

`acm health` is the only human CLI surface for repository health. Use it without `--fix` to inspect drift, add `--fix <name>` to apply a specific fixer by default, add `--dry-run` to preview without changing state, or use `--fix all` / `--apply` with no `--fix` values to run the default fixer set. Run `acm health --help` to see the available fixers and preview/apply examples.

`acm status` is the preferred diagnostics surface. It reports the active project, runtime/backend details, loaded rules/tags/tests/workflows, installed init-managed integrations, and missing setup. With `--task-text`, it also previews the simplified `context` load.

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
- [Concepts](docs/concepts.md) — what pointers, receipts, rules, memories, plans, and tags are
- [SQLite Operations](docs/sqlite.md) — deployment, backup, and rotation
- [Schema Reference](spec/v1/README.md) — v1 wire contract schemas
- [Skill Templates](skills/acm-broker/references/templates.md) — request/response examples

Architecture (contributors):

- [ADR-001: Context Broker](docs/architecture/ADR-001-context-broker.md) — historical design record; superseded by the simpler retrieval-free core model
- [Proposal: Executable Verification](docs/architecture/proposal-verify.md) — historical verify proposal; `eval` has since been removed
- [Logging Standards](docs/logging.md) — structured logging contract

## Configuration Files

acm doesn't ship project rules or opinions. You author configuration in repo-local YAML files, and acm discovers, ingests, and enforces them.

### Rules (`.acm/acm-rules.yaml`)

Define behavioral constraints for agents. Hard rules are always included in receipts; soft rules are summary-only. Use `--rules-file` on `sync`, `health --fix`, or `init` to override auto-discovery.

### Tags (`.acm/acm-tags.yaml`)

Repo-local canonical tag aliases that extend acm's embedded base dictionary. Merged on every runtime call. Use `--tags-file` on any command that does tag normalization to override.

### Verification (`.acm/acm-tests.yaml`)

Repo-defined executable checks for `verify`. v1 definitions are argv-only. Use `--tests-file` on `verify` to override auto-discovery.

### Repo-Local Feature Plan Conventions

ACM's built-in `work` schema already supports richer planning detail through `plan.stages`, `parent_task_key`, `depends_on`, and `acceptance_criteria`. Repos can layer a stricter feature-planning contract on top of those fields without changing ACM itself.

This repo does that for net-new feature work: root plans use `kind=feature`, track `spec_outline` / `refined_spec` / `implementation_plan`, group work under top-level `stage:*` tasks, and treat leaf tasks with explicit `acceptance_criteria` as the atomic units of execution. `acm verify` enforces the contract through `scripts/acm-feature-plan-validate.py`. See [docs/feature-plans.md](docs/feature-plans.md).

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

This project is licensed under the GNU Affero General Public License v3.0 only (`AGPL-3.0-only`). See [LICENSE](LICENSE).
