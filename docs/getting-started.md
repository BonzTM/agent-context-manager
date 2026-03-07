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
  --apply-template verify-go \
  --apply-template claude-command-pack
```

This scans your repo and creates auto-indexed pointer stubs for discovered files so `get-context` can work immediately. When `--project` is omitted, acm uses `ACM_PROJECT_ID` first and otherwise infers the project identifier from the effective repo root. Pass `--project` when you want a short stable namespace that differs from the folder name.

Bootstrap defaults:
- `.gitignore` is respected (`--respect-gitignore` defaults on)
- Enumerated file lists are generated in memory only — add `--persist-candidates` to save them to `.acm/bootstrap_candidates.json` (or set a custom path with `--output-candidates-path`)
- `.acm/acm-rules.yaml` is seeded if it does not already exist
- `.acm/acm-tags.yaml` is seeded if it does not already exist, with inferred repo tag suggestions when bootstrap finds strong repeated terms
- `.acm/acm-tests.yaml` is seeded if neither canonical verification definition location exists
- `.acm/acm-workflows.yaml` is seeded if neither canonical workflow definition location exists
- `.env.example` is created or extended with ACM runtime variables
- `.gitignore` is updated to ignore `.acm/context.db`, `.acm/context.db-shm`, and `.acm/context.db-wal`
- discovered repo files are auto-indexed into initial pointer stubs

Template defaults:
- `--apply-template` is repeatable and safe to re-run later
- templates only create missing files, upgrade ACM-owned blank scaffolds when they are still pristine, and merge known additive JSON fragments
- templates never delete files or overwrite edited repo files; conflicting existing files are skipped
- built-ins currently include `starter-contract`, `verify-go`, `claude-command-pack`, `claude-receipt-guard`, and `git-hooks-precommit`

Check what was indexed:

```bash
acm coverage --project-root .
```

## Step 3: Fill in your rules

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

These are starter rules. Add, remove, or modify them to match how you want agents to behave in your project. See [concepts.md](concepts.md) for the rule format reference.

Bootstrap also creates `.acm/acm-tags.yaml` when it is missing. acm seeds it with inferred repo tag suggestions when bootstrap finds strong repeated terms; otherwise it falls back to an empty structured file. Use that file to add repo-local canonical tags and aliases on top of acm's embedded base dictionary. A starter example lives at [examples/acm-tags.yaml](examples/acm-tags.yaml).

Bootstrap also creates `.acm/acm-tests.yaml` when neither `.acm/acm-tests.yaml` nor `acm-tests.yaml` already exists. It starts as a blank structured skeleton:

```yaml
version: acm.tests.v1
defaults:
  cwd: .
  timeout_sec: 300
tests: []
```

If you want a Go-oriented starter instead of the blank skeleton, rerun bootstrap with `--apply-template verify-go`. That template only upgrades ACM-owned blank scaffolds while they are still pristine.

Bootstrap also creates `.acm/acm-workflows.yaml` when neither `.acm/acm-workflows.yaml` nor `acm-workflows.yaml` already exists. It starts as a thin structured skeleton:

```yaml
version: acm.workflows.v1
completion:
  required_tasks: []
```

That file is intentionally minimal. Add `run` blocks there only when you want repo-local gates such as `review --run` / `run=true` to execute a maintained script or command.

Bootstrap also creates or extends `.env.example` with ACM runtime defaults:

```dotenv
ACM_PROJECT_ID=myproject
ACM_PROJECT_ROOT=/path/to/repo
ACM_SQLITE_PATH=.acm/context.db
ACM_PG_DSN=postgres://user:pass@localhost:5432/agents_context?sslmode=disable
ACM_UNBOUNDED=false
ACM_LOG_LEVEL=info
ACM_LOG_SINK=stderr
```

If you want a stronger baseline than the blank scaffolds, start by copying these starter files into your repo and trimming them down:

- [examples/acm-rules.yaml](examples/acm-rules.yaml)
- [examples/acm-tags.yaml](examples/acm-tags.yaml)
- [examples/acm-tests.yaml](examples/acm-tests.yaml)
- [examples/acm-workflows.yaml](examples/acm-workflows.yaml)
- [examples/AGENTS.md](examples/AGENTS.md)
- [examples/CLAUDE.md](examples/CLAUDE.md)

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
acm get-context --project myproject --task-file task.txt --phase execute
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
acm fetch --project myproject --key "myproject:src/signup.go#validate"
```

Or derive the plan fetch key from the receipt:

```bash
acm fetch --project myproject --receipt-id <receipt-id>
```

To fetch a list of keys from a file:

```bash
echo '["myproject:src/signup.go#validate", "myproject:src/signup_test.go"]' > keys.json
acm fetch --project myproject --keys-file keys.json
```

### work

Agents call this to track multi-step progress. Plans and tasks survive context compaction, so `get_context` can return active plans after chat resets.

One-shot (no temp files) with inline JSON:

```bash
acm work --project myproject --receipt-id <receipt-id> --mode merge \
  --plan-json '{"title":"Signup validation","objective":"Implement + verify validation changes","status":"in_progress"}' \
  --tasks-json '[{"key":"add-validation","summary":"Add input validation logic","status":"in_progress"},{"key":"verify:tests","summary":"Run tests for changed behavior","status":"pending"}]'
```

`tasks` are the canonical payload for rich tracking. Legacy `items` are still accepted for compatibility. `verify:diff-review` remains available as an optional manual workflow task, and `.acm/acm-workflows.yaml` can require additional task keys, but acm falls back to `verify:tests` when no workflow gates are configured.

If you only need to record one review-gate outcome instead of managing a broader task set, use `review`.

### review

Agents call this when a workflow gate needs a single review outcome without assembling a full `work` payload. When the workflow definition includes a runnable gate, prefer `--run`:

```bash
acm review --project myproject \
  --receipt-id <receipt-id> \
  --run
```

`review` is intentionally thin. It lowers to a single `work.tasks[]` merge update. In run mode, acm loads the matching task from `.acm/acm-workflows.yaml`, executes its `run` block, persists an append-only review-attempt record, and updates the latest work-task snapshot. Runnable review gates default to terminal-gate behavior: same-fingerprint reruns are skipped, `max_attempts` is optional, and `report_completion` requires a fresh passing review for the current scoped fingerprint when fingerprint dedupe is enabled. By default it still uses `key=review:cross-llm` and `summary="Cross-LLM review"`. Use `--plan-key` instead of `--receipt-id` when resuming from a plan, or override `--key` if your workflow gate uses a different task key.

If you need to record a manual review note instead of running a workflow command, omit `--run`:

```bash
acm review --project myproject \
  --receipt-id <receipt-id> \
  --outcome "Cross-checked the fix with a second model and found no new blockers."
```

Manual `status`, `outcome`, `blocked_reason`, and `evidence` fields are only for non-run mode. Keep raw reviewer commands in repo-local scripts referenced by `.acm/acm-workflows.yaml`, not in AGENTS files, skill prompts, or other maintainer prose.

### work history

Use the public history surface to rediscover active or archived plans, memories, receipts, and runs without direct database access:

```bash
acm work list --project myproject --scope current
acm work search --project myproject --query "signup validation"
acm work search --project myproject --query "bootstrap" --scope completed
acm history search --project myproject --entity all --limit 20
acm history search --project myproject --entity memory --query "postgres indexing"
acm history search --project myproject --entity receipt --query "signup validation"
```

`work list` and `work search` are the work-specific history surfaces and accept work-only filters such as `--scope` and `--kind`. Generic `history search` is the umbrella for multi-entity discovery and keeps to `--entity`, `--query`, `--limit`, and `--unbounded`.

Results stay compact and include targeted `fetch_keys`, so agents can search first and then `fetch` the exact plan, memory, receipt, or run payloads they need.

### verify

Before `report_completion` for code changes, run repo-defined executable verification:

```bash
acm verify --project myproject --receipt-id <receipt-id> --phase review \
  --file-changed src/signup.go \
  --file-changed src/signup_test.go
```

### report_completion

Agents call this to close a task after verification is satisfied. acm validates that changed files are within the receipt's scope and checks any configured completion task keys from `.acm/acm-workflows.yaml` (defaulting to `verify:tests` when no workflow gates are configured):

```bash
acm report-completion --project myproject \
  --receipt-id <receipt-id> \
  --file-changed src/signup.go \
  --file-changed src/signup_test.go \
  --outcome "Added email and password validation with tests"
```

File-based alternatives for scripted workflows:

```bash
echo '["src/signup.go", "src/signup_test.go"]' > changed.json
acm report-completion --project myproject \
  --receipt-id <receipt-id> \
  --files-changed-file changed.json \
  --outcome-file outcome.txt
```

### propose_memory

Agents call this when they discover something worth remembering for future tasks:

```bash
acm propose-memory --project myproject \
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
  --apply-template claude-receipt-guard \
  --apply-template git-hooks-precommit
```

`claude-receipt-guard` is additive and opt-in. It seeds `.claude/settings.json` plus hook scripts that keep edits blocked until `/acm-get` or an equivalent `get_context` request succeeds in the current Claude session.

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
acm-mcp tools          # list all 14 available tools
acm-mcp invoke --tool get_context --in payload.json
```

The MCP adapter exposes the same 14 operations as the CLI — seven agent-facing (`get_context`, `fetch`, `work`, `review`, `history_search`, `propose_memory`, `report_completion`) and seven maintenance (`sync`, `health_check`, `health_fix`, `coverage`, `eval`, `verify`, `bootstrap`). `review` is the thin single-task review surface, lowers to `work` internally, and can execute a workflow-defined `run` block when `run=true`. Use `history_search` for compact `work`, `memory`, `receipt`, `run`, or `all` discovery.

## Step 7: Ongoing maintenance

### Keep the index fresh

After code changes:

```bash
acm sync --project myproject --mode changed --git-range HEAD~1..HEAD
```

Or sync against the working tree (includes uncommitted changes):

```bash
acm sync --project myproject --mode working_tree
```

### Update rules and workflow gates

1. Edit `.acm/acm-rules.yaml` and, when needed, `.acm/acm-workflows.yaml`
2. Run `acm sync --project myproject --mode working_tree`
3. Run `acm health --project myproject` to verify

`sync` re-syncs both file pointers and canonical rules. With `--insert-new-candidates` enabled, uncovered files are auto-indexed into usable pointer stubs during sync. If you only want to sync rules without touching file pointers, use `acm health-fix --project myproject --apply --fixer sync_ruleset` instead.

If your ruleset is in a non-standard location, use `--rules-file`. If you also keep a repo-local tag dictionary outside the default location, pass `--tags-file` the same way:

```bash
acm sync --project myproject --mode working_tree --rules-file path/to/my-rules.yaml --tags-file path/to/my-tags.yaml
```

The same `--tags-file` override is available on `get-context`, `propose-memory`, `report-completion`, `sync`, `health-fix`, `eval`, `verify`, and `bootstrap` when you want runtime tag normalization to use a repo-local dictionary explicitly.

### Check health

```bash
acm health --project myproject --include-details
```

Reports stale pointers, orphan relations, unknown tags, and other drift.

### Fix issues automatically

```bash
acm health-fix --project myproject --apply
```

Available fixers:
- `sync_working_tree` — re-sync file hashes from disk
- `index_uncovered_files` — add missing files to the index
- `sync_ruleset` — re-sync rules from canonical ruleset

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
acm eval --project myproject --eval-suite-path ./eval.json --minimum-recall 0.8
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
acm verify --project myproject --phase review --file-changed internal/auth/service.go --dry-run
```

Run the selected checks:

```bash
acm verify --project myproject --phase review --file-changed internal/auth/service.go
```

When you include `--receipt-id` or `--plan-key`, `verify` reuses the existing `verify:tests` work item for definition-of-done updates. `verify:diff-review` can still exist as an optional manual review task, and workflow definitions can require other task keys too, but acm does not enforce them unless `.acm/acm-workflows.yaml` declares them.

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
        argv: ["scripts/acm-cross-review.sh"]
        cwd: .
        timeout_sec: 1800
```

Bootstrap still seeds the thin `required_tasks: []` skeleton by default. Adding gates like `review:cross-llm` is an explicit repo policy choice. When a gate defines `run`, agents should normally satisfy it with `acm review --run` or `run=true`; ACM will skip same-fingerprint reruns and only enforce retry limits when the workflow explicitly sets `max_attempts`. Otherwise they can use manual `review` fields or a direct `work` update. Keep raw reviewer commands in repo-local scripts or workflow definitions, not maintainer prose.

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
