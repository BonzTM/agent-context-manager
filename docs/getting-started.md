# Getting Started

This guide walks you through setting up acm in a project from scratch. Steps 1-4 are human setup. Steps 5+ show the agent-facing operations and how to wire them into your tools.
The public contract is built around the smaller core surface described here rather than preserving every older alias or workflow shape.

This guide is for adopting ACM in another repository. If you are maintaining ACM itself, use [AGENTS.md](../AGENTS.md), [docs/maintainer-map.md](maintainer-map.md), and [docs/maintainer-reference.md](maintainer-reference.md) for the repo-specific maintainer workflow instead of treating this file as the maintainer contract.

## Prerequisites

- Go 1.26+ installed if you plan to use `go install` or build from source
- A git repository you want to manage with acm

## Pick a Mode

You do not need the full ACM surface on day 1.

- `plans-only`: `init`, `context`, and `work`
- `governed workflow`: add `verify`, `review`, and `done`
- `full brokered flow`: add `fetch`, optional review gates, and the richer template set

`acm init` starts from the minimal core. Heavier templates stay opt-in.

## Step 1: Install acm

Preferred path:

```bash
go install github.com/bonztm/agent-context-manager/cmd/acm@latest
go install github.com/bonztm/agent-context-manager/cmd/acm-mcp@latest
go install github.com/bonztm/agent-context-manager/cmd/acm-web@latest   # optional web dashboard
```

Go installs binaries to `$GOBIN` if it is set, otherwise to `$(go env GOPATH)/bin` (typically `~/go/bin`). That directory must be on your `PATH`.

```bash
export PATH="$(go env GOPATH)/bin:$PATH"
```

If you want prebuilt binaries instead, download the `acm-binaries` artifact from a successful `Go Build` GitHub Actions run and put `acm`, `acm-mcp`, and optionally `acm-web` on your `PATH`.

If you are building locally from a checkout:

```bash
git clone https://github.com/bonztm/agent-context-manager.git
cd agent-context-manager
go build -o dist/acm ./cmd/acm
go build -o dist/acm-mcp ./cmd/acm-mcp
go build -o dist/acm-web ./cmd/acm-web    # optional
export PATH="$PWD/dist:$PATH"
```

Verify it works:

```bash
acm --help
```

## Step 2: Initialize the Core

From your project root, seed ACM and generate the initial ACM inventory:

```bash
acm init
```

If you want a heavier starter later, rerun `init` with one or more additive templates:

```bash
acm init \
  --apply-template starter-contract \
  --apply-template verify-generic \
  --apply-template claude-command-pack
```

Swap in `--apply-template codex-pack` when you want repo-local Codex companion docs under `.codex/acm-broker/` instead of Claude-specific command assets.
Use `--apply-template opencode-pack` when you want repo-local OpenCode companion docs under `.opencode/acm-broker/` without assuming a global OpenCode skill location.

This scans your repo and creates auto-indexed pointer stubs for discovered files so `fetch`, health, and governed scope checks can work immediately. When `--project` is omitted, acm uses `ACM_PROJECT_ID` first and otherwise infers the project identifier from the effective repo root. Pass `--project` when you want a short stable namespace that differs from the folder name.

`init` automatically:

- Respects `.gitignore` (on by default)
- Seeds `.acm/acm-rules.yaml`, `.acm/acm-tags.yaml`, `.acm/acm-tests.yaml`, and `.acm/acm-workflows.yaml` when missing
- Creates/extends `.env.example` with ACM runtime variables
- Adds `.acm/context.db*` entries to `.gitignore`
- Auto-indexes discovered repo files into pointer stubs

Use `--persist-candidates` to save the enumerated file list to `.acm/init_candidates.json`.

Templates (`--apply-template`) are repeatable and safe to re-run. They only create missing files, upgrade pristine scaffolds, and merge additive JSON fragments — they never delete or overwrite files you've edited. Built-ins: `starter-contract`, `detailed-planning-enforcement`, `verify-generic`, `verify-go`, `verify-ts`, `verify-python`, `verify-rust`, `codex-pack`, `codex-hooks`, `opencode-pack`, `claude-command-pack`, `claude-hooks`, `git-hooks-precommit`. See [docs/examples/init-templates.md](examples/init-templates.md) for the seeded files and template-specific behavior.

If you want to inspect indexing drift later, use `acm health --include-details` or `acm status`. The standalone `coverage` command is gone; the useful signals now live in health/status.

## Step 3: Configure your project

### Rules

`init` creates `.acm/acm-rules.yaml` when it is missing. Replace the blank scaffold with your project rules:

```yaml
version: acm.rules.v1
rules:
  - id: rule_context_first
    summary: Always call context before reading or editing files.
    content: Run `acm context` first, then follow all hard rules in the receipt.
    enforcement: hard
    tags: [startup, context]

  - id: rule_minimal_fetch
    summary: Keep context compact by fetching only what is required.
    content: Fetch only the keys needed for the active task and current step.
    enforcement: soft
    tags: [context, efficiency]

  - id: rule_done
    summary: Close every task with a completion report.
    content: Call done with changed files for file-backed work when you know them, or let ACM auto-detect the receipt delta. When that detected delta is empty, the closure is effectively no-file.
    enforcement: hard
    tags: [completion, audit]

  - id: rule_verify_before_completion
    summary: Run verify before done when code changes.
    content: Use verify to satisfy executable checks before closing code changes with done.
    enforcement: hard
    tags: [verification, quality]
```

These are starter rules. Add, remove, or modify them to match how you want agents to behave in your project. See [concepts.md](concepts.md) for the full rule format reference.

### Tags

`init` creates `.acm/acm-tags.yaml` when it is missing, with inferred repo tag suggestions when strong repeated terms are found. Use this file to add repo-local canonical tags and aliases on top of acm's embedded base dictionary. See [examples/acm-tags.yaml](examples/acm-tags.yaml) for the format.

### Verification checks

`init` creates `.acm/acm-tests.yaml` when neither `.acm/acm-tests.yaml` nor `acm-tests.yaml` exists. It starts as a blank skeleton:

```yaml
version: acm.tests.v1
defaults:
  cwd: .
  timeout_sec: 300
tests: []
```

For a starter verify profile, rerun `init` with one of:

- `acm init --apply-template verify-generic` for a language-agnostic profile that works out of the box
- `acm init --apply-template verify-go` for Go repos
- `acm init --apply-template verify-ts` for TypeScript repos
- `acm init --apply-template verify-python` for Python repos
- `acm init --apply-template verify-rust` for Rust repos
- `acm init --apply-template detailed-planning-enforcement` for the richer feature-plan contract; it can be applied directly or after `starter-contract` + `verify-generic`, and it upgrades those pristine files when present

### Workflow gates

`init` creates `.acm/acm-workflows.yaml` when neither canonical location exists. It starts as a thin skeleton:

```yaml
version: acm.workflows.v1
completion:
  required_tasks: []
```

This is intentionally minimal. Add `run` blocks when you want gates like `review --run` to execute a repo-local script. See [concepts.md](concepts.md#workflow-gate-definitions) for the full format.

### Environment

`init` creates or extends `.env.example` with ACM runtime defaults:

```dotenv
ACM_PROJECT_ID=myproject
ACM_PROJECT_ROOT=/path/to/repo
ACM_SQLITE_PATH=.acm/context.db
ACM_PG_DSN=postgres://user:pass@localhost:5432/agents_context?sslmode=disable
ACM_UNBOUNDED=false # true removes built-in history/list caps for supported surfaces
ACM_LOG_LEVEL=info
ACM_LOG_SINK=stderr
```

### Starter examples

For a stronger baseline than the blank scaffolds, copy these into your repo and trim to fit:

| File | Description |
|---|---|
| [examples/acm-rules.yaml](examples/acm-rules.yaml) | Rules with common agent constraints |
| [examples/acm-tags.yaml](examples/acm-tags.yaml) | Tag aliases for typical projects |
| [examples/acm-tests.yaml](examples/acm-tests.yaml) | Verification check definitions |
| [examples/acm-workflows.yaml](examples/acm-workflows.yaml) | Completion gate definitions |
| [examples/AGENTS.md](examples/AGENTS.md) | Root repo contract for Codex and other AGENTS-aware tools |
| [examples/CLAUDE.md](examples/CLAUDE.md) | Claude Code instructions |

## Step 4: Sync rules into acm

```bash
acm sync --mode working_tree
```

This syncs both file pointers and canonical rules in one pass.

Verify they landed:

```bash
acm health --include-details
```

## Step 5: Verify the workflow

The following operations are what agents call during tasks — via CLI scripts, slash commands, or MCP tools. You can run them manually here to verify your setup works.

> **Note:** All examples below omit `--project` because acm infers the project from the repo root (or `ACM_PROJECT_ID` if set). Pass `--project <id>` explicitly only when you need a namespace that differs from the folder name.

> **Maintenance note:** when you change rules, tags, tests, workflows, onboarding, or tool-surface behavior, run `acm sync --mode working_tree --insert-new-candidates` and `acm health --include-details` before `done`.

### context

Agents call this at the start of every task to get a scoped receipt:

```bash
acm context \
  --task-text "add input validation to the signup form" \
  --phase execute
```

For longer task descriptions, use `--task-file` instead:

```bash
echo "Refactor the signup flow to validate email format, password strength, and CSRF tokens" > task.txt
acm context --task-file task.txt --phase execute
```

The response is a JSON receipt containing:
- `rules` — hard constraints (full content included for hard enforcement rules)
- `plans` — active work plans for the project, with fetch keys for resumption
- `initial_scope_paths` — explicitly known starting scope when the caller already knows it
- `_meta` — receipt ID, resolved tags, task metadata, and the baseline ACM will later use to compute the real task delta for `verify` and `done`

Plans from prior runs are automatically included — agents can see in-progress work and choose to resume or start fresh. The `receipt_id` from `_meta` is the handle for all subsequent operations.

### fetch

Agents call this to hydrate full content only when a returned key actually needs to be opened:

```bash
acm fetch --key "myproject:src/signup.go#validate"
```

Or derive the plan fetch key from the receipt:

```bash
acm fetch --receipt-id <receipt-id>
```

To fetch a list of keys from a file:

```bash
echo '["myproject:src/signup.go#validate", "myproject:src/signup_test.go"]' > keys.json
acm fetch --keys-file keys.json
```

### work

Agents call this to track multi-step progress. Plans and tasks survive context compaction, so `context` can return active plans after chat resets.

One-shot (no temp files) with inline JSON:

```bash
acm work --receipt-id <receipt-id> --mode merge \
  --plan-json '{"title":"Signup validation","objective":"Implement + verify validation changes","status":"in_progress"}' \
  --tasks-json '[{"key":"add-validation","summary":"Add input validation logic","status":"in_progress"},{"key":"verify:tests","summary":"Run tests for changed behavior","status":"pending"}]'
```

`tasks` are the canonical payload for tracking. If you only need to record a single review-gate outcome, use `review` instead.

When governed file work expands beyond the receipt's initial scope, record the later-discovered files through `plan.discovered_paths` in `work` before expecting `review` to pass. `done` also accepts path-like entries from `plan.in_scope`, but `discovered_paths` remains the right way to declare later-found concrete files.

#### Optional richer staged plans

ACM's built-in `work` schema already includes `plan.stages`, `parent_task_key`, `depends_on`, and `acceptance_criteria`. A repo can use those fields to define a stricter staged-plan contract for multi-step work without changing ACM itself.

Typical pattern:

- use a root plan with an explicit kind plus explicit scope metadata
- mirror `plan.stages.spec_outline` / `refined_spec` / `implementation_plan` with top-level `stage:*` tasks
- keep one orchestrator on the root plan and treat leaf tasks as the atomic execution units
- when a plan reaches a terminal status, ACM reconciles those stage fields from the matching `stage:*` task statuses during auto-close
- require `acceptance_criteria` and exact `references` on atomic leaf tasks
- use child plans with `kind=feature_stream` and `parent_plan_key` for parallel execution streams
- run a repo-local validator from `verify`; `ACM_PLAN_KEY` and `ACM_RECEIPT_ID` are injected into verify commands automatically

This repo uses that pattern in [feature-plans.md](feature-plans.md) and enforces it with `scripts/acm-feature-plan-validate.py`, but that stricter staged-plan contract remains repo policy, not an ACM product default.

### verify versus review

`verify` and `review` are complementary, not interchangeable:

| Need | Use |
|---|---|
| Deterministic repo-defined executable checks selected from `.acm/acm-tests.yaml` | `verify` |
| One named workflow signoff gate from `.acm/acm-workflows.yaml` | `review` |

Typical governed closeout is `work` -> `verify` -> `review --run` when required -> `done`.

### review

Agents call this when a workflow gate needs a single review outcome without assembling a full `work` payload. When the workflow definition includes a runnable gate, prefer `--run`:

```bash
acm review \
  --receipt-id <receipt-id> \
  --run
```

`review` is intentionally thin — it lowers to a single `work.tasks[]` merge update.

Use `review` for one named workflow signoff gate. Do not use it as a substitute for deterministic repo checks or bulk test execution.

- **Run mode**: acm loads the matching task from `.acm/acm-workflows.yaml`, executes its `run` block, persists a review-attempt record, and updates the work-task snapshot. When fingerprint dedupe is enabled, ACM only skips reruns after a passing attempt already assessed the current fingerprint; failed or interrupted same-fingerprint attempts rerun until any configured `max_attempts` budget is exhausted. `done` requires a passing runnable review for the current fingerprint when dedupe is enabled. The scoped fingerprint covers effective scope: receipt `initial_scope_paths`, work-declared `discovered_paths`, and ACM-managed governance files that completion reporting already allows outside project file scope, including repo-root `AGENTS.md` and `CLAUDE.md`.
- **Defaults**: `key=review:cross-llm`, `summary="Cross-LLM review"`.
- Use `--plan-key` instead of `--receipt-id` when resuming from a plan, or `--key` if your workflow uses a different task key.

If you need to record a manual review note instead of running a workflow command, omit `--run`:

```bash
acm review \
  --receipt-id <receipt-id> \
  --outcome "Cross-checked the fix with a second model and found no new blockers."
```

Manual fields (`--status`, `--outcome`, `--blocked-reason`, `--evidence`) are only for non-run mode and do not satisfy runnable review gates that define `run`. Use `--status superseded` when a review step is no longer relevant and should close cleanly instead of being left open. Keep raw reviewer commands in repo-local scripts referenced by `.acm/acm-workflows.yaml`.

### acm history discovery

Use the public history surface to rediscover active or archived plans, receipts, and runs without direct database access:

```bash
acm history --entity work --scope current
acm history --entity work --query "signup validation"
acm history --entity work --query "bootstrap" --scope completed
acm history --entity all --limit 20
acm history --entity receipt --query "signup validation"
```

One surface:

- **`acm history`** — compact discovery across work, receipts, and runs; set `--entity work` when you need work-specific `--scope` or `--kind` filters

Results are compact and include `fetch_keys`, so agents can search first and then `fetch` the exact payloads they need.

### status

Use `status` when you need one command to explain the current ACM setup before debugging a workflow:

```bash
acm status --task-text "what would context load for this task?" --phase execute
```

It reports the active project and backend, which repo-local rules/tags/tests/workflows files were discovered and loaded, which init-managed integrations are installed, any missing setup, and warnings for stale plans or plan-status drift.

### verify

Before `done` for code changes, run repo-defined executable verification:

```bash
acm verify --receipt-id <receipt-id> --phase review \
  --file-changed src/signup.go \
  --file-changed src/signup_test.go
```

Use `verify` for deterministic repo-defined checks from `.acm/acm-tests.yaml`. If you need one named workflow signoff gate instead, use `review`.

### done

Agents call this to close a task after verification is satisfied. ACM computes the task delta from the receipt baseline. When changed files are supplied, acm cross-checks them against the detected delta, surfaces mismatches as violations, validates that file-backed work stays within effective scope (`initial_scope_paths` + `discovered_paths` + path-like `plan.in_scope` entries + ACM-managed governance files), and checks any configured completion task keys from `.acm/acm-workflows.yaml`. The built-in managed governance set includes repo-root `AGENTS.md`, `CLAUDE.md`, and the canonical `.acm/**` contract files. If no workflow gates are configured, file-backed closures fall back to `verify:tests`.

```bash
acm done \
  --receipt-id <receipt-id> \
  --file-changed src/signup.go \
  --file-changed src/signup_test.go \
  --outcome "Added email and password validation with tests"
```

File-based alternatives for scripted workflows:

```bash
echo '["src/signup.go", "src/signup_test.go"]' > changed.json
acm done \
  --receipt-id <receipt-id> \
  --files-changed-file changed.json \
  --outcome-file outcome.txt
```

You can also omit `files_changed` entirely and let ACM use the baseline-derived task delta. When that detected delta is empty, the closeout is effectively no-file:

```bash
acm done \
  --receipt-id <receipt-id> \
  --outcome "Drafted the feature plan and recorded follow-up tasks"
```

## Step 6: Wire agents to acm

Once the index and rules are set up, connect your agents so they call acm operations automatically.

### Claude Code

Install the slash command pack into your project:

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/bonztm/agent-context-manager/main/scripts/install-skill-pack.sh) --claude
```

Run this from your project root. It installs `/acm-context`, `/acm-work`, `/acm-review`, `/acm-verify`, and `/acm-done` slash commands into `.claude/commands/`. The default slash-command loop is `/acm-context`, `/acm-work`, `/acm-verify`, and `/acm-done`.

If you already have this repo checked out locally, the equivalent command is `./scripts/install-skill-pack.sh --claude`.

If you prefer to seed the same files directly through `init`, rerun:

```bash
acm init \
  --apply-template claude-command-pack \
  --apply-template claude-hooks \
  --apply-template git-hooks-precommit
```

`claude-hooks` is additive and opt-in. It seeds `.claude/settings.json` plus hook scripts that re-inject the ACM loop on Claude session start/compaction, keep edits blocked until `/acm-context` or an equivalent `context` request succeeds in the current session, nudge `/acm-work` once edits span files or governed scope expands, and block stop until edited work is closed with `acm done`.

`git-hooks-precommit` seeds `.githooks/pre-commit`, which forwards staged additions, modifications, renames, type changes, and deletions into `acm verify --phase review`. Enable it with:

```bash
git config core.hooksPath .githooks
```

Add a thin `CLAUDE.md` to your project root. A starter template is at [docs/examples/CLAUDE.md](examples/CLAUDE.md).

### Codex

Install the skill pack:

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/bonztm/agent-context-manager/main/scripts/install-skill-pack.sh) --codex
```

This installs the global `acm-broker` skill to `~/.codex/skills/acm-broker`, including `codex/README.md` and `codex/AGENTS.example.md` companion docs inside the installed skill.

If you already have this repo checked out locally, the equivalent command is `./scripts/install-skill-pack.sh --codex`.

If you also want repo-local Codex companion docs in the project itself, seed them with:

```bash
acm init --apply-template codex-pack
```

That template creates:

- `.codex/acm-broker/README.md`
- `.codex/acm-broker/AGENTS.example.md`

If you also want the experimental repo-local Codex hook layer, seed it with:

```bash
acm init --apply-template codex-hooks
```

That template creates:

- `.codex/config.toml`
- `.codex/hooks.json`
- `.codex/hooks/acm-common.sh`
- `.codex/hooks/acm-session-context.sh`
- `.codex/hooks/acm-prompt-guard.sh`
- `.codex/hooks/acm-stop-guard.sh`

The hook layer is opt-in and intentionally narrower than Claude's hook pack. Today it only covers startup guidance, prompt-time context nudges, and a one-time stop reminder, and it depends on Codex's current experimental hook support.

Keep the repo-root `AGENTS.md` authoritative. A starter template is at [docs/examples/AGENTS.md](examples/AGENTS.md), and the Codex companion example above is there to make the full ACM loop explicit for Codex-driven repos.

Codex is a primary ACM operator, not only a review backend. The normal loop is:

1. `context`
2. `work`
3. `verify`
4. `review`
5. `done`

Claude and Codex hand off through the same ACM state. A common pattern is:

1. Start in one tool with `context`
2. Record or update the durable plan with `work`
3. Let the other tool resume from the same `receipt_id` or `plan_key`
4. Close with shared `verify`, `review`, and `done` history instead of vendor-local notes

There is intentionally no fake slash-command or Claude-equivalent hook parity here. The default Codex path still relies on the installed skill, the repo-root `AGENTS.md`, and normal CLI/MCP access; `codex-hooks` is an optional experimental helper layer.

### OpenCode

Install the repo-local OpenCode companion docs into your project:

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/bonztm/agent-context-manager/main/scripts/install-skill-pack.sh) --opencode
```

Run this from your project root. It installs:

- `.opencode/acm-broker/README.md`
- `.opencode/acm-broker/AGENTS.example.md`

If you already have this repo checked out locally, the equivalent command is `./scripts/install-skill-pack.sh --opencode`.

Use `--opencode` when you are adding OpenCode guidance to an existing repo immediately.

If you prefer to seed the same files through `init`, rerun:

```bash
acm init --apply-template opencode-pack
```

Use `opencode-pack` when you are already bootstrapping the repo with `acm init` and want the OpenCode companion docs created alongside the rest of the starter ACM assets.

Keep the repo-root `AGENTS.md` authoritative. The current OpenCode path is explicit and repo-local: companion docs plus normal `acm` CLI or MCP access.

OpenCode is a primary ACM operator, not only a review backend. The normal loop is:

1. `context`
2. `work`
3. `verify`
4. `review`
5. `done`

Use OpenCode's native repo search and edit tools normally; ACM provides durable state, rules, verification, review, and governed closeout.

Short walkthrough:

1. Install the companion docs with `--opencode`, or seed them during `acm init` with `opencode-pack`.
2. Keep `AGENTS.md` authoritative and use `.opencode/acm-broker/README.md` as the thin OpenCode companion.
3. Start implementation or debugging work with `acm context --project <id> --task-text "..." --phase execute`.
4. If the task spans multiple steps or files, persist it with `acm work`; add `plan.discovered_paths` before review/done when governed scope expands.
5. Run `acm verify` before completion, then `acm review --run` when `.acm/acm-workflows.yaml` selects a runnable review gate, then `acm done`.

For already isolated/containerized hosts, prefer workflow `run.argv` that uses `scripts/acm-cross-review.sh --yolo`; the shared high-trust shortcut avoids nested sandbox conflicts while relying on the outer container boundary.

There is intentionally no undocumented global OpenCode skill path or hidden hook story here. ACM does not currently ship an OpenCode hook pack because this repo has not documented a verified native OpenCode hook mechanism yet.

### MCP

For models with native tool support, use the MCP adapter:

```bash
acm-mcp tools          # list all 13 available tools
acm-mcp invoke --tool context --in payload.json
```

The MCP adapter exposes the same 12 convenience-routed operations plus one backend-only export surface:

- **Core** (4): `context`, `work`, `verify`, `done`
- **Supporting** (3): `fetch`, `review`, `history`
- **Advanced backend-only** (1): `export`
- **Maintenance** (4): `sync`, `health`, `status`, `init`

Use `export` through `acm run --in <request.json>` or `acm-mcp invoke --tool export --in <payload.json>` when you need stable JSON or Markdown output for ACM-owned context, fetch, history, or status data. Example envelope: [examples/export-request.json](examples/export-request.json).

For interactive CLI use, `context`, `fetch`, `history`, and `status` also accept `--format json|markdown`, plus `--out-file` and `--force`, to emit raw rendered artifacts through that same backend export path. Example command: [examples/context-export-command.txt](examples/context-export-command.txt).

## Optional: Web Dashboard

`acm-web` provides a read-only web dashboard for humans to see what agents are working on without touching the CLI.

```bash
acm-web                       # starts on :8080
acm-web serve --addr :9090    # custom port
```

Pages:

- **Board** (`/`) — Kanban board with Pending, In Progress, Blocked, and Done columns. Tasks are tree-sorted with children beneath parents. Click any card for details including navigable parent/child links and rolled-up progress.
- **Status** (`/status.html`) — Project info, loaded sources, installed integrations, and warnings.
- **Health** (`/healthz`) — JSON liveness probe for k8s.

`acm-web` reads the same environment variables as `acm` (`ACM_PROJECT_ID`, `ACM_PG_DSN`, etc.) and shares the same database. A `Dockerfile.acm-web` is provided for containerized deployment.

## Step 7: Ongoing Maintenance And Advanced Surfaces

### Keep the index fresh

After code changes:

```bash
acm sync --mode changed --git-range HEAD~1..HEAD
```

Or sync against the working tree (includes uncommitted changes):

```bash
acm sync --mode working_tree
```

### Update rules and workflow gates

1. Edit `.acm/acm-rules.yaml` and, when needed, `.acm/acm-workflows.yaml`
2. Run `acm sync --mode working_tree`
3. Run `acm health` to verify

`sync` re-syncs both file pointers and canonical rules. Use `--insert-new-candidates` to auto-index any unindexed files during sync. To sync rules only (without touching file pointers), use `acm health --fix sync_ruleset`.

If your ruleset or tag dictionary is in a non-standard location, pass `--rules-file` and/or `--tags-file`:

```bash
acm sync --mode working_tree --rules-file path/to/my-rules.yaml --tags-file path/to/my-tags.yaml
```

### Check health

```bash
acm health --include-details
```

Reports stale pointers, orphan relations, unknown tags, and other drift.

### Fix issues automatically

```bash
acm health --fix all
```

Run `acm health --help` to list fixers directly in the CLI.

Available fixers:

- `all` — run the default fixer set (`sync_working_tree`, `index_unindexed_files`, `sync_ruleset`)
- `sync_working_tree` — re-sync file hashes from disk
- `index_unindexed_files` — add missing files to the index
- `sync_ruleset` — re-sync rules from canonical ruleset

Use `--dry-run` when you want a preview without changing state:

```bash
acm health --fix all --dry-run
acm health --fix sync_ruleset --dry-run
```

### Run verify checks

For repo-defined executable verification, add `.acm/acm-tests.yaml` (preferred) or `acm-tests.yaml` in the repo root:

```yaml
version: acm.tests.v1

defaults:
  cwd: .
  timeout_sec: 300

tests:
  - id: go-unit
    summary: Run Go unit tests for the repo
    command:
      argv: ["go", "test", "./..."]
      env:
        GOFLAGS: "-count=1"
    select:
      phases: ["execute", "review"]
      changed_paths_any: ["cmd/**", "internal/**"]
    expected:
      exit_code: 0

  - id: smoke
    summary: Run repo smoke checks on every verification pass
    command:
      argv: ["go", "test", "./cmd/...", "./internal/..."]
    select:
      always_run: true
    expected:
      exit_code: 0
```

Inspect selection without executing:

```bash
acm verify --phase review --file-changed internal/auth/service.go --dry-run
```

Run the selected checks:

```bash
acm verify --phase review --file-changed internal/auth/service.go
```

When you include `--receipt-id` or `--plan-key`, `verify` updates the `verify:tests` work task for definition-of-done tracking.

For repo-defined completion gates, add `.acm/acm-workflows.yaml` (preferred) or `acm-workflows.yaml` in the repo root:

```yaml
version: acm.workflows.v1
completion:
  required_tasks:
    - key: verify:tests
      select:
        changed_paths_any: ["cmd/**", "internal/**", "go.mod", "go.sum"]

    - key: review:cross-llm
      summary: Cross-LLM review
      rerun_requires_new_fingerprint: true
      select:
        phases: ["review"]
        changed_paths_any: ["cmd/**", "internal/**", "spec/**"]
      run:
        # Edit argv to match your reviewer setup (provider, model, reasoning level, native
        # permission/sandbox flags, etc.)
        argv: ["scripts/acm-cross-review.sh", "--provider", "codex", "--yolo"]
        cwd: .
        timeout_sec: 1800
```

Init seeds the thin `required_tasks: []` skeleton by default. Adding gates like `review:cross-llm` is an opt-in repo policy choice.

When a gate defines `run`, agents satisfy it with `acm review --run` (or `run=true`). ACM only skips reruns after a passing attempt already assessed the current fingerprint; failed or interrupted same-fingerprint attempts rerun until any configured `max_attempts` budget is exhausted. The scoped fingerprint covers effective scope (`initial_scope_paths` + `discovered_paths`) plus ACM-managed governance files that completion reporting already allows outside project file scope, including repo-root `AGENTS.md` and `CLAUDE.md`. Without `run`, agents can use manual `review` fields or a direct `work` update, but those manual notes do not satisfy runnable gates. Repo-local reviewer choices such as provider selection, model, reasoning effort, and the shared `--yolo` high-trust shortcut belong in the workflow `run.argv`; for Claude, `--yolo` maps to dangerous permission bypass, and `--dangerously-skip-permissions` remains available as the explicit provider-native form.
If the working tree is dirty but the active effective scope captures zero of those changes into the runnable review set, the runner should fail fast and tell the agent to rerun `context` with a broader task or declare the missing files through `work`. Runnable review output should also surface the total repo-changed versus scoped-changed file counts for quick diagnosis.

Keep richer plan-shape enforcement in `verify`, not `completion.required_tasks`, unless you truly need an additional final gate. Workflow selectors operate on phase, tags, paths, and pointers; they do not currently distinguish thin plans from `kind=feature` plans by themselves.

## Storage

acm uses SQLite by default with zero configuration. When you run inside a repo, the database is created automatically at `<repo-root>/.acm/context.db`. acm also reads `<repo-root>/.env` when present, with process environment variables taking precedence.

If you need to run acm from outside the repo directory, set `ACM_PROJECT_ROOT`. If the repo root name is not the namespace you want, also set `ACM_PROJECT_ID`:

```bash
export ACM_PROJECT_ID=myproject
export ACM_PROJECT_ROOT=/path/to/repo
```

To set a specific SQLite path:

```bash
export ACM_SQLITE_PATH=/path/to/context.db
```

For production or multi-writer environments, switch to Postgres:

```bash
export ACM_PG_DSN='postgres://user:pass@localhost:5432/agents_context?sslmode=disable'
```

See [SQLite Operations](sqlite.md) for backup, restore, and rotation procedures.

## Next Steps

- Read [Concepts](concepts.md) if any terms are unclear
- Browse [example request templates](../skills/acm-broker/references/templates.md) for all command formats
