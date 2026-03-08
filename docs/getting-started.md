# Getting Started

This guide walks you through setting up acm in a project from scratch. Steps 1-4 are human setup. Steps 5+ show the agent-facing operations and how to wire them into your tools.

## Prerequisites

- Go 1.26+ installed if you plan to use `go install` or build from source
- A git repository you want to manage with acm

## Step 1: Install acm

Preferred path:

```bash
go install github.com/bonztm/agent-context-manager/cmd/acm@latest
go install github.com/bonztm/agent-context-manager/cmd/acm-mcp@latest
```

Go installs binaries to `$GOBIN` if it is set, otherwise to `$(go env GOPATH)/bin` (typically `~/go/bin`). That directory must be on your `PATH`.

```bash
export PATH="$(go env GOPATH)/bin:$PATH"
```

If you want prebuilt binaries instead, download the `acm-binaries` artifact from a successful `Go Build` GitHub Actions run and put both `acm` and `acm-mcp` on your `PATH`.

If you are building locally from a checkout:

```bash
git clone https://github.com/bonztm/agent-context-manager.git
cd agent-context-manager
go build -o dist/acm ./cmd/acm
go build -o dist/acm-mcp ./cmd/acm-mcp
export PATH="$PWD/dist:$PATH"
```

Verify it works:

```bash
acm --help
```

## Step 2: Bootstrap your index

From your project root, seed ACM and generate an initial retrievable pointer set:

```bash
acm bootstrap
```

If you want a heavier starter later, rerun bootstrap with one or more additive templates:

```bash
acm bootstrap \
  --apply-template starter-contract \
  --apply-template verify-generic \
  --apply-template claude-command-pack
```

This scans your repo and creates auto-indexed pointer stubs for discovered files so `get-context` can work immediately. When `--project` is omitted, acm uses `ACM_PROJECT_ID` first and otherwise infers the project identifier from the effective repo root. Pass `--project` when you want a short stable namespace that differs from the folder name.

Bootstrap automatically:

- Respects `.gitignore` (on by default)
- Seeds `.acm/acm-rules.yaml`, `.acm/acm-tags.yaml`, `.acm/acm-tests.yaml`, and `.acm/acm-workflows.yaml` when missing
- Creates/extends `.env.example` with ACM runtime variables
- Adds `.acm/context.db*` entries to `.gitignore`
- Auto-indexes discovered repo files into pointer stubs

Use `--persist-candidates` to save the enumerated file list to `.acm/bootstrap_candidates.json`.

Templates (`--apply-template`) are repeatable and safe to re-run. They only create missing files, upgrade pristine scaffolds, and merge additive JSON fragments — they never delete or overwrite files you've edited. Built-ins: `starter-contract`, `detailed-planning-enforcement`, `verify-generic`, `verify-go`, `verify-ts`, `verify-python`, `verify-rust`, `claude-command-pack`, `claude-hooks`, `git-hooks-precommit`.

Check what was indexed:

```bash
acm coverage --project-root .
```

## Step 3: Configure your project

### Rules

Bootstrap creates `.acm/acm-rules.yaml` when it is missing. Replace the blank scaffold with your project rules:

```yaml
version: acm.rules.v1
rules:
  - id: rule_get_context_first
    summary: Always call get_context before reading or editing files.
    content: Retrieve context first, then follow all hard rules in the receipt.
    enforcement: hard
    tags: [startup, context]

  - id: rule_minimal_fetch
    summary: Keep context compact by fetching only what is required.
    content: Fetch only the keys needed for the active task and current step.
    enforcement: soft
    tags: [context, efficiency]

  - id: rule_report_completion
    summary: Close every task with a completion report.
    content: Call report_completion with files_changed and a concise outcome summary.
    enforcement: hard
    tags: [completion, audit]

  - id: rule_verify_before_completion
    summary: Run verify before report_completion when code changes.
    content: Use verify to satisfy executable checks before closing code changes with report_completion.
    enforcement: hard
    tags: [verification, quality]
```

These are starter rules. Add, remove, or modify them to match how you want agents to behave in your project. See [concepts.md](concepts.md) for the full rule format reference.

### Tags

Bootstrap creates `.acm/acm-tags.yaml` when it is missing, with inferred repo tag suggestions when strong repeated terms are found. Use this file to add repo-local canonical tags and aliases on top of acm's embedded base dictionary. See [examples/acm-tags.yaml](examples/acm-tags.yaml) for the format.

### Verification checks

Bootstrap creates `.acm/acm-tests.yaml` when neither `.acm/acm-tests.yaml` nor `acm-tests.yaml` exists. It starts as a blank skeleton:

```yaml
version: acm.tests.v1
defaults:
  cwd: .
  timeout_sec: 300
tests: []
```

For a starter verify profile, rerun bootstrap with one of:

- `acm bootstrap --apply-template verify-generic` for a language-agnostic profile that works out of the box
- `acm bootstrap --apply-template verify-go` for Go repos
- `acm bootstrap --apply-template verify-ts` for TypeScript repos
- `acm bootstrap --apply-template verify-python` for Python repos
- `acm bootstrap --apply-template verify-rust` for Rust repos
- `acm bootstrap --apply-template detailed-planning-enforcement` for the richer feature-plan contract; it can be applied directly or after `starter-contract` + `verify-generic`, and it upgrades those pristine files when present

### Workflow gates

Bootstrap creates `.acm/acm-workflows.yaml` when neither canonical location exists. It starts as a thin skeleton:

```yaml
version: acm.workflows.v1
completion:
  required_tasks: []
```

This is intentionally minimal. Add `run` blocks when you want gates like `review --run` to execute a repo-local script. See [concepts.md](concepts.md#workflow-gate-definitions) for the full format.

### Environment

Bootstrap creates or extends `.env.example` with ACM runtime defaults:

```dotenv
ACM_PROJECT_ID=myproject
ACM_PROJECT_ROOT=/path/to/repo
ACM_SQLITE_PATH=.acm/context.db
ACM_PG_DSN=postgres://user:pass@localhost:5432/agents_context?sslmode=disable
ACM_UNBOUNDED=false
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
| [examples/AGENTS.md](examples/AGENTS.md) | Codex agent instructions |
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

### get_context

Agents call this at the start of every task to get a scoped receipt:

```bash
acm get-context \
  --task-text "add input validation to the signup form" \
  --phase execute
```

For longer task descriptions, use `--task-file` instead:

```bash
echo "Refactor the signup flow to validate email format, password strength, and CSRF tokens" > task.txt
acm get-context --task-file task.txt --phase execute
```

The response is a JSON receipt containing:
- `rules` — hard constraints (full content included for hard enforcement rules)
- `suggestions` — relevant code/doc/test pointers (key + summary only)
- `memories` — durable facts from past work
- `plans` — active work plans for the project, with fetch keys for resumption
- `_meta` — receipt ID, resolved tags, and budget accounting metadata

Plans from prior runs are automatically included — agents can see in-progress work and choose to resume or start fresh. The `receipt_id` from `_meta` is the handle for all subsequent operations.

### fetch

Agents call this to pull full content for pointer keys from the receipt:

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

Agents call this to track multi-step progress. Plans and tasks survive context compaction, so `get_context` can return active plans after chat resets.

One-shot (no temp files) with inline JSON:

```bash
acm work --receipt-id <receipt-id> --mode merge \
  --plan-json '{"title":"Signup validation","objective":"Implement + verify validation changes","status":"in_progress"}' \
  --tasks-json '[{"key":"add-validation","summary":"Add input validation logic","status":"in_progress"},{"key":"verify:tests","summary":"Run tests for changed behavior","status":"pending"}]'
```

`tasks` are the canonical payload for tracking. If you only need to record a single review-gate outcome, use `review` instead.

#### Optional richer feature plans

ACM's built-in `work` schema already includes `plan.stages`, `parent_task_key`, `depends_on`, and `acceptance_criteria`. A repo can use those fields to define a stricter feature-plan contract for net-new feature work without changing ACM itself.

Typical pattern:

- use a root plan with `kind=feature` plus explicit scope metadata
- mirror `plan.stages.spec_outline` / `refined_spec` / `implementation_plan` with top-level `stage:*` tasks
- treat leaf tasks as the atomic tasks and require `acceptance_criteria` on those leaves
- use child plans with `kind=feature_stream` and `parent_plan_key` for parallel execution streams
- run a repo-local validator from `verify`; `ACM_PLAN_KEY` and `ACM_RECEIPT_ID` are injected into verify commands automatically

This repo uses that pattern in [feature-plans.md](feature-plans.md) and enforces it with `scripts/acm-feature-plan-validate.py`.

### review

Agents call this when a workflow gate needs a single review outcome without assembling a full `work` payload. When the workflow definition includes a runnable gate, prefer `--run`:

```bash
acm review \
  --receipt-id <receipt-id> \
  --run
```

`review` is intentionally thin — it lowers to a single `work.tasks[]` merge update.

- **Run mode**: acm loads the matching task from `.acm/acm-workflows.yaml`, executes its `run` block, persists a review-attempt record, and updates the work-task snapshot. Same-fingerprint reruns are skipped; `report_completion` requires a fresh passing review when fingerprint dedupe is enabled. The scoped fingerprint covers receipt pointer paths plus ACM-managed governance files that completion reporting already allows outside pointer scope.
- **Defaults**: `key=review:cross-llm`, `summary="Cross-LLM review"`.
- Use `--plan-key` instead of `--receipt-id` when resuming from a plan, or `--key` if your workflow uses a different task key.

If you need to record a manual review note instead of running a workflow command, omit `--run`:

```bash
acm review \
  --receipt-id <receipt-id> \
  --outcome "Cross-checked the fix with a second model and found no new blockers."
```

Manual fields (`--status`, `--outcome`, `--blocked-reason`, `--evidence`) are only for non-run mode. Keep raw reviewer commands in repo-local scripts referenced by `.acm/acm-workflows.yaml`.

### work history

Use the public history surface to rediscover active or archived plans, memories, receipts, and runs without direct database access:

```bash
acm work list --scope current
acm work search --query "signup validation"
acm work search --query "bootstrap" --scope completed
acm history search --entity all --limit 20
acm history search --entity memory --query "postgres indexing"
acm history search --entity receipt --query "signup validation"
```

Two surfaces:

- **`work list` / `work search`** — work-specific, accepts `--scope` and `--kind` filters
- **`history search`** — multi-entity discovery across work, memories, receipts, and runs

Results are compact and include `fetch_keys`, so agents can search first and then `fetch` the exact payloads they need.

### status

Use `status` when you need one command to explain the current ACM setup before debugging a workflow:

```bash
acm status --task-text "why did get_context pick these pointers?" --phase execute
```

It reports the active project and backend, which repo-local rules/tags/tests/workflows files were discovered and loaded, which bootstrap integrations are installed, and any missing setup. `acm doctor` is an alias, but `status` is the canonical command name.

### verify

Before `report_completion` for code changes, run repo-defined executable verification:

```bash
acm verify --receipt-id <receipt-id> --phase review \
  --file-changed src/signup.go \
  --file-changed src/signup_test.go
```

### report_completion

Agents call this to close a task after verification is satisfied. acm validates that changed files are within the receipt's scope and checks any configured completion task keys from `.acm/acm-workflows.yaml` (defaulting to `verify:tests` when no workflow gates are configured):

```bash
acm report-completion \
  --receipt-id <receipt-id> \
  --file-changed src/signup.go \
  --file-changed src/signup_test.go \
  --outcome "Added email and password validation with tests"
```

File-based alternatives for scripted workflows:

```bash
echo '["src/signup.go", "src/signup_test.go"]' > changed.json
acm report-completion \
  --receipt-id <receipt-id> \
  --files-changed-file changed.json \
  --outcome-file outcome.txt
```

### propose_memory

Agents call this when they discover something worth remembering for future tasks:

```bash
acm propose-memory \
  --receipt-id <receipt-id> \
  --category gotcha \
  --subject "signup form requires CSRF token" \
  --content "The signup endpoint validates a CSRF token from the session cookie. Tests must set this header or they get 403." \
  --confidence 4 \
  --evidence-key "myproject:src/signup.go#validate" \
  --memory-tag backend \
  --memory-tag auth
```

For longer memory content, use `--content-file`. Memory tags, evidence keys, and related keys also accept `--memory-tags-json` / `--evidence-keys-json` / `--related-keys-json` and `--memory-tags-file` / `--evidence-keys-file` / `--related-keys-file` (JSON arrays). Use `--related-key` (repeatable) to link the memory to related pointers beyond the evidence chain. Use `--tags-file` when you need to override the canonical tag dictionary used for runtime normalization. Add `--auto-promote` to skip quarantine and promote directly if validations pass.

Memories are available in future `get_context` calls when relevant tags match.

## Step 6: Wire agents to acm

Once the index and rules are set up, connect your agents so they call acm operations automatically.

### Claude Code

Install the slash command pack into your project:

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/bonztm/agent-context-manager/main/scripts/install-skill-pack.sh) --claude
```

Run this from your project root. It installs `/acm-get`, `/acm-work`, `/acm-review`, `/acm-verify`, `/acm-report`, `/acm-memory`, and `/acm-eval` slash commands into `.claude/commands/`.

If you already have this repo checked out locally, the equivalent command is `./scripts/install-skill-pack.sh --claude`.

If you prefer to seed the same files directly through bootstrap, rerun:

```bash
acm bootstrap \
  --apply-template claude-command-pack \
  --apply-template claude-hooks \
  --apply-template git-hooks-precommit
```

`claude-hooks` is additive and opt-in. It seeds `.claude/settings.json` plus hook scripts that re-inject the ACM loop on Claude session start/compaction, keep edits blocked until `/acm-get` or an equivalent `get_context` request succeeds in the current session, nudge `/acm-work` once edits span files, and block stop until edited work is closed with `acm report-completion`. The older `claude-receipt-guard` template name still works as a compatibility alias.

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

If you already have this repo checked out locally, the equivalent command is `./scripts/install-skill-pack.sh --codex`.

Add an `AGENTS.md` to your project root. A starter template is at [docs/examples/AGENTS.md](examples/AGENTS.md).

### MCP

For models with native tool support, use the MCP adapter:

```bash
acm-mcp tools          # list all 15 available tools
acm-mcp invoke --tool get_context --in payload.json
```

The MCP adapter exposes the same 15 operations as the CLI:

- **Agent-facing** (7): `get_context`, `fetch`, `work`, `review`, `history_search`, `propose_memory`, `report_completion`
- **Maintenance** (8): `sync`, `health_check`, `health_fix`, `status`, `coverage`, `eval`, `verify`, `bootstrap`

## Step 7: Ongoing maintenance

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

`sync` re-syncs both file pointers and canonical rules. Use `--insert-new-candidates` to auto-index any uncovered files during sync. To sync rules only (without touching file pointers), use `acm health --fix sync_ruleset`.

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
- `all` — run the default fixer set (`sync_working_tree`, `index_uncovered_files`, `sync_ruleset`)
- `sync_working_tree` — re-sync file hashes from disk
- `index_uncovered_files` — add missing files to the index
- `sync_ruleset` — re-sync rules from canonical ruleset

Use `--dry-run` when you want a preview without changing state:

```bash
acm health --fix all --dry-run
acm health --fix sync_ruleset --dry-run
```

### Run eval and verify checks

Create an eval suite to verify retrieval quality:

```json
[
  {
    "task_text": "fix the login bug",
    "phase": "execute",
    "expected_pointer_keys": ["myproject:src/auth/login.go"]
  }
]
```

```bash
acm eval --eval-suite-path ./eval.json --minimum-recall 0.8
```

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
        argv: ["scripts/acm-cross-review.sh", "--model", "gpt-5.3-codex", "--reasoning-effort", "xhigh"]
        cwd: .
        timeout_sec: 1800
```

Bootstrap seeds the thin `required_tasks: []` skeleton by default. Adding gates like `review:cross-llm` is an opt-in repo policy choice.

When a gate defines `run`, agents satisfy it with `acm review --run` (or `run=true`). ACM skips same-fingerprint reruns and only enforces retry limits when `max_attempts` is set. The scoped fingerprint covers receipt pointer paths plus ACM-managed governance files that completion reporting already allows outside pointer scope. Without `run`, agents can use manual `review` fields or a direct `work` update. Repo-local reviewer choices such as a Codex model or reasoning effort belong in the workflow `run.argv`.
If the working tree is dirty but the active receipt scopes zero of those changes into the runnable review set, the runner should fail fast and tell the agent to rerun `get_context` with a broader task. Runnable review output should also surface the total repo-changed versus scoped-changed file counts for quick diagnosis.

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
- Review [ADR-001](architecture/ADR-001-context-broker.md) for architecture and design decisions
