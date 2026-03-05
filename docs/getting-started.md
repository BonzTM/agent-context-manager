# Getting Started

This guide walks you through setting up acm in a project from scratch. Steps 1-4 are human setup. Steps 5+ show the agent-facing operations and how to wire them into your tools.

## Prerequisites

- Go 1.22+ installed
- A git repository you want to manage with acm

## Step 1: Install acm

```bash
go install github.com/joshd/agent-context-manager/cmd/acm@latest
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

Check what was indexed:

```bash
acm coverage --project myproject --project-root .
```

## Step 3: Write your rules

Create the directory and ruleset file:

```bash
mkdir -p .acm
```

Create `.acm/acm-rules.yaml`:

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
- `plans` — active work items
- `_meta` — receipt ID, resolved tags, budget info

The `receipt_id` from `_meta` is the handle for all subsequent operations.

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
  --tag backend \
  --tag auth
```

For longer memory content, use `--content-file`. Tags and evidence keys also accept `--tags-json` / `--evidence-keys-json` and `--tags-file` / `--evidence-keys-file` (JSON arrays). Add `--auto-promote` to skip quarantine and promote directly if validations pass.

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
acm-mcp tools          # list available tools
acm-mcp invoke --tool get_context --in payload.json
```

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

If your ruleset is in a non-standard location, use `--rules-file`:

```bash
acm sync --project myproject --mode working_tree --rules-file path/to/my-rules.yaml
```

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
acm regress --project myproject --eval-suite-path ./eval.json --minimum-recall 0.8
```

## Storage

acm uses SQLite by default with zero configuration. The database is created automatically at `<user-cache-dir>/agent-context-manager/context.db`.

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
