# Getting Started

This guide walks you through setting up ctx in a project from scratch. By the end, you'll have a working context broker with rules, indexed pointers, and an agent-ready workflow.

## Prerequisites

- Go 1.22+ installed
- A git repository you want to manage with ctx

## Step 1: Install ctx

```bash
go install github.com/joshd/agents-context/cmd/ctx@latest
```

Verify it works:

```bash
ctx --help
```

## Step 2: Bootstrap your index

From your project root, generate an initial set of pointer candidates:

```bash
ctx bootstrap --project myproject --project-root . --respect-gitignore
```

This scans your repo and creates pointer entries for discovered files. The `--project` flag is your project's identifier — use a short, stable name (e.g., `myapp`, `soundspan`).

Check what was indexed:

```bash
ctx coverage --project myproject --project-root .
```

## Step 3: Write your rules

Create the directory and ruleset file:

```bash
mkdir -p .ctx
```

Create `.ctx/canonical-ruleset.yaml`:

```yaml
version: ctx.rules.v1
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

## Step 4: Sync rules into ctx

```bash
ctx health-fix --project myproject --apply --fixer sync_ruleset
```

Verify they landed:

```bash
ctx health --project myproject --include-details
```

## Step 5: Try the workflow

### Retrieve context

```bash
ctx get-context --project myproject \
  --task-text "add input validation to the signup form" \
  --phase execute
```

The response is a JSON receipt containing:
- `rules` — hard constraints (full content included for hard enforcement rules)
- `suggestions` — relevant code/doc/test pointers (key + summary only)
- `memories` — durable facts from past work
- `plans` — active work items
- `_meta` — receipt ID, resolved tags, budget info

Save the `receipt_id` from `_meta` — you'll need it for the next steps.

### Fetch details

To get full content for a specific pointer from the suggestions:

```bash
ctx fetch --project myproject --key "myproject:src/signup.go#validate"
```

Or fetch everything referenced by the receipt:

```bash
ctx fetch --project myproject --receipt-id <receipt-id>
```

### Track work

If the task has multiple steps, track them:

Create a `work-items.json` file:

```json
[
  {"key": "add-validation", "summary": "Add input validation logic", "status": "in_progress"},
  {"key": "verify:tests", "summary": "Run tests for changed behavior", "status": "pending"},
  {"key": "verify:diff-review", "summary": "Review diff for unintended changes", "status": "pending"}
]
```

```bash
ctx work --project myproject --receipt-id <receipt-id> --items-file work-items.json
```

Update status as work progresses by resubmitting with updated statuses — work items are upserted by key.

### Report completion

```bash
ctx report-completion --project myproject \
  --receipt-id <receipt-id> \
  --file-changed src/signup.go \
  --file-changed src/signup_test.go \
  --outcome "Added email and password validation with tests"
```

### Propose a memory

If the agent discovered something worth remembering:

```bash
ctx propose-memory --project myproject \
  --receipt-id <receipt-id> \
  --category gotcha \
  --subject "signup form requires CSRF token" \
  --content "The signup endpoint validates a CSRF token from the session cookie. Tests must set this header or they get 403." \
  --confidence 4 \
  --evidence-key "myproject:src/signup.go#validate" \
  --tag backend \
  --tag auth
```

This memory will be available in future `get_context` calls when relevant tags match.

## Step 6: Set up agent integration

### Claude Code

Install the slash command pack into your project:

```bash
./scripts/install-skill-pack.sh --claude-target .
```

This gives agents `/ctx-get`, `/ctx-report`, and `/ctx-memory` commands.

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
ctx-mcp tools          # list available tools
ctx-mcp invoke --tool get_context --in payload.json
```

## Step 7: Ongoing maintenance

### Keep the index fresh

After code changes:

```bash
ctx sync --project myproject --mode changed --git-range HEAD~1..HEAD
```

Or sync against the working tree (includes uncommitted changes):

```bash
ctx sync --project myproject --mode working_tree --project-root .
```

### Update rules

1. Edit `.ctx/canonical-ruleset.yaml`
2. Run `ctx health-fix --project myproject --apply --fixer sync_ruleset`
3. Run `ctx health --project myproject` to verify

### Check health

```bash
ctx health --project myproject --include-details
```

Reports stale pointers, orphan relations, unknown tags, and other drift.

### Fix issues automatically

```bash
ctx health-fix --project myproject --apply
```

Available fixers:
- `sync_working_tree` — re-sync file hashes from disk
- `index_uncovered_files` — add missing files to the index
- `sync_ruleset` — re-sync rules from canonical ruleset

### Run regression tests

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
ctx regress --project myproject --eval-suite-path ./eval.json --minimum-recall 0.8
```

## Storage

ctx uses SQLite by default with zero configuration. The database is created automatically at `<user-cache-dir>/agents-context/context.db`.

To set a specific path:

```bash
export CTX_SQLITE_PATH=/path/to/context.db
```

For production or multi-writer environments, switch to Postgres:

```bash
export CTX_PG_DSN='postgres://user:pass@localhost:5432/agents_context?sslmode=disable'
```

See [SQLITE_OPERATIONS.md](SQLITE_OPERATIONS.md) for backup, restore, and rotation procedures.

## Next Steps

- Read [Concepts](concepts.md) if any terms are unclear
- Browse [example request templates](../skills/ctx-broker/references/templates.md) for all command formats
- Review [ADR-001](ADR-001-context-broker.md) for architecture and design decisions
