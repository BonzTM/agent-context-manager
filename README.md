# ctx — Context Manager for LLM Agents

ctx is a deterministic context broker that sits between you and your LLM agents. It does four things:

1. **Keeps context windows light** — stores an index of your codebase as pointers (key + one-line summary). Agents retrieve only what's relevant to the current task, then fetch full content on demand.
2. **Enforces your rules** — you write the rules, ctx stores them, and delivers them as hard constraints that agents must follow. Rules aren't suggestions buried in a long file — they're structured, scoped, and delivered at the right time.
3. **Provides canonical memory** — durable facts learned from completed work, stored outside any model's memory system. Any agent, any session, any model can access them.
4. **Tracks work across sessions** — stateful, idempotent work items that survive context compaction. When an agent loses its place, it can pick up where it left off.

ctx is infrastructure, not opinions. It doesn't ship default rules or enforce a workflow. You define the rules, you seed the index, ctx enforces and delivers.

## Install

```bash
go install github.com/joshd/agents-context/cmd/ctx@latest
go install github.com/joshd/agents-context/cmd/ctx-mcp@latest
```

Or build from source:

```bash
git clone https://github.com/joshd/agents-context.git
cd agents-context
go build ./cmd/ctx
go build ./cmd/ctx-mcp
```

## Quick Start (5 minutes)

### 1. Bootstrap your project

Scan your repo and generate an initial pointer index:

```bash
ctx bootstrap --project myapp --project-root .
```

### 2. Write your rules

Create `.ctx/canonical-ruleset.yaml`:

```yaml
version: ctx.rules.v1
rules:
  - id: rule_get_context_first
    summary: Always call get_context before reading or editing files.
    enforcement: hard
    tags: [startup]

  - id: rule_report_completion
    summary: Close every task with report_completion.
    enforcement: hard
    tags: [completion]
```

### 3. Sync rules into ctx

```bash
ctx health-fix --project myapp --apply --fixer sync_ruleset
```

### 4. Retrieve context for a task

```bash
ctx get-context --project myapp --task-text "fix the login timeout bug" --phase execute
```

This returns a receipt with:
- **Rules** — hard constraints the agent must follow
- **Suggestions** — code/doc/test pointers relevant to the task (advisory, not mandatory)
- **Memories** — durable facts from past work
- **Plans** — active work items

### 5. Fetch full content

The receipt contains keys and summaries. To get full content for a specific pointer:

```bash
ctx fetch --project myapp --key "myapp:src/auth/login.go#handleTimeout"
```

Or fetch everything from the receipt at once:

```bash
ctx fetch --project myapp --receipt-id <receipt-id-from-step-4>
```

### 6. Report completion

```bash
ctx report-completion --project myapp \
  --receipt-id <receipt-id> \
  --file-changed src/auth/login.go \
  --outcome "Fixed timeout by increasing session TTL"
```

That's it. For the full workflow (including work tracking and memory), see [Getting Started](docs/getting-started.md).

## Agent Integration

### Claude Code (slash commands)

```bash
./scripts/install-skill-pack.sh --claude-target /path/to/your/project
```

This installs `/ctx-get`, `/ctx-report`, and `/ctx-memory` slash commands.

### Codex (skill pack)

```bash
./scripts/install-skill-pack.sh --skip-claude
```

Installs the ctx-broker skill to `~/.codex/skills/ctx-broker`.

### MCP (tool-native models)

```bash
ctx-mcp invoke --tool get_context --in payload.json
```

Five tools exposed: `get_context`, `fetch`, `work`, `propose_memory`, `report_completion`.

## CLI Reference

All commands support `--help` for full flag documentation.

### Core workflow

```bash
ctx get-context   --project <id> --task-text <text> --phase <plan|execute|review>
ctx fetch         --project <id> --key <pointer-key> [--receipt-id <id>]
ctx work          --project <id> --receipt-id <id> [--items-file <path>]
ctx propose-memory --project <id> --receipt-id <id> --category <cat> --subject <text> --content <text> --confidence <1-5> --evidence-key <key>
ctx report-completion --project <id> --receipt-id <id> --file-changed <path> --outcome <text>
```

### Maintenance

```bash
ctx sync          --project <id> --mode <changed|full|working_tree>
ctx health        --project <id> [--include-details]
ctx health-fix    --project <id> --apply [--fixer <name>]
ctx coverage      --project <id> --project-root .
ctx regress       --project <id> --eval-suite-path ./eval.json
ctx bootstrap     --project <id> --project-root .
```

### JSON envelope mode

The original JSON envelope interface is still available for programmatic use:

```bash
ctx run --in request.json
ctx validate --in request.json
```

## Storage Backend

SQLite by default (zero config). Set `CTX_PG_DSN` for Postgres when you need write concurrency.

```bash
# SQLite (default)
export CTX_SQLITE_PATH=/path/to/context.db

# Postgres
export CTX_PG_DSN='postgres://user:pass@localhost:5432/agents_context?sslmode=disable'
```

See [SQLite Operations](docs/SQLITE_OPERATIONS.md) for deployment, backup, and rotation guidance.

## Documentation

- [Getting Started](docs/getting-started.md) — full walkthrough from zero to working ctx setup
- [Concepts](docs/concepts.md) — what pointers, receipts, rules, memories, and work items are
- [Architecture (ADR-001)](docs/ADR-001-context-broker.md) — design decisions and data model
- [SQLite Operations](docs/SQLITE_OPERATIONS.md) — deployment and backup procedures
- [Logging Standards](docs/LOGGING_STANDARDS.md) — structured logging contract (for contributors)
- [Schema Reference](spec/v1/README.md) — v1 wire contract schemas
- [Skill Templates](skills/ctx-broker/references/templates.md) — request/response examples

## Canonical Rules

ctx doesn't ship rules. You author them in `.ctx/canonical-ruleset.yaml` (or `ctx-rules.yaml`), and ctx ingests and enforces them.

See [docs/examples/canonical-ruleset.yaml](docs/examples/canonical-ruleset.yaml) for the format, and [Getting Started](docs/getting-started.md) for the full rule authoring and maintenance workflow.

## Logging

```bash
export CTX_LOG_LEVEL=debug   # debug|info|warn|error (default: info)
export CTX_LOG_SINK=stderr   # stderr|stdout|discard (default: stderr)
```
