# acm — Context Manager for LLM Agents

acm manages the context pipeline between you and your LLM agents.

- **You define what matters** — index your codebase, write your rules, scope what agents see.
- **acm delivers it** — task-scoped retrieval returns only relevant rules, code pointers, memories, and work state. Context windows stay light.
- **Agents follow it** — rules are delivered as hard constraints, not suggestions buried in a long file. Scope violations are caught on completion.
- **Everything persists** — memories, work items, and run history are stored outside any model's memory. They survive context compaction, session boundaries, and model switches.

acm is infrastructure, not opinions. It doesn't ship default rules or enforce a workflow. You define the rules, you seed the index, acm enforces and delivers.

## Install

```bash
go install github.com/joshd/agent-context-manager/cmd/acm@latest
go install github.com/joshd/agent-context-manager/cmd/acm-mcp@latest
```

Or build from source:

```bash
git clone https://github.com/joshd/agent-context-manager.git
cd agent-context-manager
go build ./cmd/acm
go build ./cmd/acm-mcp
```

## Quick Start (5 minutes)

### 1. Bootstrap your project

Scan your repo and generate an initial pointer index:

```bash
acm bootstrap --project my-cool-app --project-root .
```

Bootstrap respects `.gitignore` by default and generates descriptions with LLM assistance. Use `--persist-candidates` to save the candidate list to `.acm/bootstrap_candidates.json`.

### 2. Write your rules

Create `.acm/acm-rules.yaml`:

```yaml
version: acm.rules.v1
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

### 3. Sync rules into acm

```bash
acm sync --project my-cool-app --mode working_tree
```

### 4. Set up agent integration

Wire agents to acm via slash commands, skill packs, or MCP tools — see [Getting Started](docs/getting-started.md) for details.

Once connected, agents call acm operations automatically during tasks:

1. `get_context` — retrieves a scoped receipt (rules, suggestions, memories, work state)
2. `fetch` — pulls full content for selected pointer keys
3. `work` — tracks multi-step progress (survives context compaction)
4. `report_completion` — closes the task, validates scope
5. `propose_memory` — saves durable facts for future retrieval

You can test any operation manually via CLI (e.g., `acm get-context --project my-cool-app --task-text "fix the login bug" --phase execute`). See the [CLI Reference](#cli-reference) below.

## Agent Integration

### Claude Code (slash commands)

```bash
./scripts/install-skill-pack.sh --claude-target /path/to/your/project
```

This installs `/acm-get`, `/acm-report`, and `/acm-memory` slash commands.

### Codex (skill pack)

```bash
./scripts/install-skill-pack.sh --skip-claude
```

Installs the acm-broker skill to `~/.codex/skills/acm-broker`.

### MCP (tool-native models)

```bash
acm-mcp invoke --tool get_context --in payload.json
```

Five tools exposed: `get_context`, `fetch`, `work`, `propose_memory`, `report_completion`.

## CLI Reference

All commands support `--help` for full flag documentation.

### Agent-facing (called by agents via CLI, skills, or MCP)

```bash
acm get-context    --project <id> (--task-text <text>|--task-file <path>) --phase <plan|execute|review>
acm fetch          --project <id> [--key <key>]... [--keys-file <path>|--keys-json <json>] [--expect <key=version>]... [--expected-versions-file <path>|--expected-versions-json <json>] [--receipt-id <id>]
acm work           --project <id> [--plan-key <key>|--receipt-id <id>] [--mode <merge|replace>] [--plan-file <path>|--plan-json <json>] [--tasks-file <path>|--tasks-json <json>] [--items-file <path>|--items-json <json>]
acm propose-memory --project <id> --receipt-id <id> --category <cat> --subject <text> (--content <text>|--content-file <path>) --confidence <1-5> --evidence-key <key> [--auto-promote]
acm report-completion --project <id> --receipt-id <id> [--file-changed <path>]... [--files-changed-file <path>|--files-changed-json <json>] (--outcome <text>|--outcome-file <path>) [--scope-mode <strict|warn|auto_index>]
```

Most list and text flags support inline values and `--*-file` alternatives (`-` for stdin). JSON list/object inputs also support `--*-json` for one-shot agent calls without temporary files.

### Human-facing (setup and maintenance)

```bash
acm bootstrap     --project <id> --project-root . [--persist-candidates] [--respect-gitignore] [--llm-assist-descriptions] [--output-candidates-path <path>] [--rules-file <path>]
acm sync          --project <id> --mode <changed|full|working_tree> [--insert-new-candidates] [--rules-file <path>]
acm health        --project <id> [--include-details]
acm health-fix    --project <id> --apply [--fixer <name>] [--rules-file <path>]
acm coverage      --project <id> --project-root .
acm regress       --project <id> (--eval-suite-path ./eval.json|--eval-suite-inline-file <path>|--eval-suite-inline-json <json>)
```

### JSON envelope mode

The original JSON envelope interface is still available for programmatic use:

```bash
acm run --in request.json
acm validate --in request.json
```

## Storage Backend

SQLite by default (zero config). Set `ACM_PG_DSN` for Postgres when you need write concurrency.

```bash
# SQLite (default)
export ACM_SQLITE_PATH=/path/to/context.db

# Postgres
export ACM_PG_DSN='postgres://user:pass@localhost:5432/agents_context?sslmode=disable'
```

See [SQLite Operations](docs/SQLITE_OPERATIONS.md) for deployment, backup, and rotation guidance.

## Documentation

- [Getting Started](docs/getting-started.md) — full walkthrough from zero to working acm setup
- [Concepts](docs/concepts.md) — what pointers, receipts, rules, memories, and work items are
- [Architecture (ADR-001)](docs/ADR-001-context-broker.md) — design decisions and data model
- [SQLite Operations](docs/SQLITE_OPERATIONS.md) — deployment and backup procedures
- [Logging Standards](docs/LOGGING_STANDARDS.md) — structured logging contract (for contributors)
- [Schema Reference](spec/v1/README.md) — v1 wire contract schemas
- [Skill Templates](skills/acm-broker/references/templates.md) — request/response examples

## Canonical Rules

acm doesn't ship rules. You author them in `.acm/acm-rules.yaml` (preferred) or `acm-rules.yaml` in the project root, and acm ingests and enforces them. Use `--rules-file` on `sync`, `health-fix`, or `bootstrap` to override discovery with an explicit path.

See [docs/examples/acm-rules.yaml](docs/examples/acm-rules.yaml) for the format, and [Getting Started](docs/getting-started.md) for the full rule authoring and maintenance workflow.

## Logging

```bash
export ACM_LOG_LEVEL=debug   # debug|info|warn|error (default: info)
export ACM_LOG_SINK=stderr   # stderr|stdout|discard (default: stderr)
```

## License

This project is licensed under the GNU Affero General Public License v3.0 only (`AGPL-3.0-only`). See [LICENSE](LICENSE).
