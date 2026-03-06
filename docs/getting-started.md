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

If you want prebuilt binaries instead, download the `acm-binaries` artifact from a successful `Go Build` GitHub Actions run and put `acm` on your `PATH`.

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

From your project root, generate an initial set of pointer candidates:

```bash
acm bootstrap --project myproject --project-root .
```

This scans your repo and creates pointer entries for discovered files. The `--project` flag is your project's identifier — use a short, stable name (e.g., `my-cool-app`).

Bootstrap defaults:
- `.gitignore` is respected (`--respect-gitignore` defaults on)
- Descriptions are generated with LLM assistance (`--llm-assist-descriptions` defaults on)
- Candidate lists are generated in memory only — add `--persist-candidates` to save them to `.acm/bootstrap_candidates.json` (or set a custom path with `--output-candidates-path`)
- `.acm/acm-rules.yaml` is seeded if it does not already exist
- `.acm/acm-tags.yaml` is seeded if it does not already exist, with inferred repo tag suggestions when bootstrap finds strong repeated terms
- `.acm/acm-tests.yaml` is seeded if neither canonical verification definition location exists
- `.env.example` is created or extended with ACM runtime variables
- `.gitignore` is updated to ignore `.acm/context.db`

Check what was indexed:

```bash
acm coverage --project myproject --project-root .
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

Bootstrap also creates or extends `.env.example` with ACM runtime defaults:

```dotenv
ACM_SQLITE_PATH=.acm/context.db
ACM_PG_DSN=postgres://user:pass@localhost:5432/agents_context?sslmode=disable
ACM_LOG_LEVEL=info
ACM_LOG_SINK=stderr
```

## Step 4: Sync rules into acm

```bash
acm sync --project myproject --mode working_tree
```

This syncs both file pointers and canonical rules in one pass.

Verify they landed:

```bash
acm health --project myproject --include-details
```

## Step 5: Verify the workflow

The following operations are what agents call during tasks — via CLI scripts, slash commands, or MCP tools. You can run them manually here to verify your setup works.

### get_context

Agents call this at the start of every task to get a scoped receipt:

```bash
acm get-context --project myproject \
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
- `plans` — active work plans for the project, with task counts and fetch keys for resumption
- `_meta` — receipt ID, resolved tags, budget info

Plans from prior runs are automatically included — agents can see in-progress work and choose to resume or start fresh. The `receipt_id` from `_meta` is the handle for all subsequent operations.

### fetch

Agents call this to pull full content for pointer keys from the receipt:

```bash
acm fetch --project myproject --key "myproject:src/signup.go#validate"
```

Or fetch everything referenced by the receipt:

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
  --tasks-json '[{"key":"add-validation","summary":"Add input validation logic","status":"in_progress"},{"key":"verify:tests","summary":"Run tests for changed behavior","status":"pending"},{"key":"verify:diff-review","summary":"Review diff for unintended changes","status":"pending"}]'
```

`tasks` are the canonical payload for rich tracking. Legacy `items` are still accepted for compatibility.

### report_completion

Agents call this to close a task. acm validates that changed files are within the receipt's scope:

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

For longer memory content, use `--content-file`. Memory tags and evidence keys also accept `--memory-tags-json` / `--evidence-keys-json` and `--memory-tags-file` / `--evidence-keys-file` (JSON arrays). Use `--tags-file` when you need to override the canonical tag dictionary used for runtime normalization. Add `--auto-promote` to skip quarantine and promote directly if validations pass.

Memories are available in future `get_context` calls when relevant tags match.

## Step 6: Wire agents to acm

Once the index and rules are set up, connect your agents so they call acm operations automatically.

### Claude Code

Install the slash command pack into your project:

```bash
./scripts/install-skill-pack.sh --claude-target .
```

This gives agents `/acm-get`, `/acm-report`, and `/acm-memory` commands.

Add a thin `CLAUDE.md` to your project root. A starter template is at [docs/examples/CLAUDE.md](examples/CLAUDE.md).

### Codex

Install the skill pack:

```bash
./scripts/install-skill-pack.sh --skip-claude
```

Add an `AGENTS.md` to your project root. A starter template is at [docs/examples/AGENTS.md](examples/AGENTS.md).

### MCP

For models with native tool support, use the MCP adapter:

```bash
acm-mcp tools          # list all 12 available tools
acm-mcp invoke --tool get_context --in payload.json
```

The MCP adapter exposes the same 12 operations as the CLI — five agent-facing (`get_context`, `fetch`, `work`, `propose_memory`, `report_completion`) and seven maintenance (`sync`, `health_check`, `health_fix`, `coverage`, `eval`, `verify`, `bootstrap`).

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

### Update rules

1. Edit `.acm/acm-rules.yaml`
2. Run `acm sync --project myproject --mode working_tree`
3. Run `acm health --project myproject` to verify

`sync` re-syncs both file pointers and canonical rules. If you only want to sync rules without touching file pointers, use `acm health-fix --project myproject --apply --fixer sync_ruleset` instead.

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
    select:
      phases: ["execute", "review"]
      changed_paths_any: ["cmd/**", "internal/**"]
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

When you include `--receipt-id` or `--plan-key`, `verify` reuses the existing `verify:tests` work item for definition-of-done updates.

## Storage

acm uses SQLite by default with zero configuration. When you run inside a repo, the database is created automatically at `<repo-root>/.acm/context.db`. acm also reads `<repo-root>/.env` when present, with process environment variables taking precedence.

To set a specific path:

```bash
export ACM_SQLITE_PATH=/path/to/context.db
```

For production or multi-writer environments, switch to Postgres:

```bash
export ACM_PG_DSN='postgres://user:pass@localhost:5432/agents_context?sslmode=disable'
```

See [SQLITE_OPERATIONS.md](SQLITE_OPERATIONS.md) for backup, restore, and rotation procedures.

## Next Steps

- Read [Concepts](concepts.md) if any terms are unclear
- Browse [example request templates](../skills/acm-broker/references/templates.md) for all command formats
- Review [ADR-001](ADR-001-context-broker.md) for architecture and design decisions
