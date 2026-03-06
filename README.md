# acm — Context Manager for LLM Agents

acm manages the context pipeline between you and your LLM agents.

- **You define what matters** — index your codebase, write your rules, scope what agents see.
- **acm delivers it** — task-scoped retrieval returns only relevant rules, code pointers, memories, and work state. Context windows stay light.
- **Agents follow it** — rules are delivered as hard constraints, not suggestions buried in a long file. Scope violations are caught on completion.
- **Everything persists** — memories, work items, and run history are stored outside any model's memory. They survive context compaction, session boundaries, and model switches.

acm is infrastructure, not opinions. It doesn't ship default rules or enforce a workflow. You define the rules, you seed the index, acm enforces and delivers.

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

### 1. Bootstrap your project

Scan your repo, seed repo-local ACM files, and materialize an initial auto-indexed pointer set:

```bash
acm bootstrap --project my-cool-app --project-root .
```

Bootstrap respects `.gitignore` by default and generates descriptions with LLM assistance. Use `--persist-candidates` to save the enumerated file list to `.acm/bootstrap_candidates.json`.
Bootstrap also seeds `.acm/acm-rules.yaml` when it is missing, seeds `.acm/acm-tags.yaml` with inferred repo tag suggestions when possible, seeds a blank structured `.acm/acm-tests.yaml`, appends `.acm/context.db` to `.gitignore`, creates or extends `.env.example`, and auto-indexes discovered repo files into initial pointer stubs so `get_context` works immediately.

### 2. Fill in your seeded rules

Bootstrap creates `.acm/acm-rules.yaml` if it does not already exist. Replace the blank scaffold with your project rules:

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

  - id: rule_verify_before_completion
    summary: Run verify before report_completion when code changes.
    enforcement: hard
    tags: [verification]
```

### 3. Sync rules into acm

```bash
acm sync --project my-cool-app --mode working_tree
```

### 4. Set up agent integration

Wire agents to acm via slash commands, skill packs, or MCP tools — see [Getting Started](docs/getting-started.md) for details.

Once connected, agents call acm operations automatically during tasks:

1. `get_context` — retrieves a scoped receipt (rules, suggestions, memories, active plans from prior runs)
2. `fetch` — pulls full content for pointer keys, plan keys, or task keys
3. `work` — creates/updates structured plans with tasks (survives context compaction)
4. `verify` — runs repo-defined executable checks and updates `verify:tests` when work context is present
5. `report_completion` — closes the task, validates scope, and enforces `verify:tests`
6. `propose_memory` — saves durable facts for future retrieval

You can test any operation manually via CLI (e.g., `acm get-context --project my-cool-app --task-text "fix the login bug" --phase execute`). See the [CLI Reference](#cli-reference) below.

## Agent Integration

### Claude Code (slash commands)

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/bonztm/agent-context-manager/main/scripts/install-skill-pack.sh) --claude /path/to/your/project
```

This installs `/acm-get`, `/acm-work`, `/acm-verify`, `/acm-report`, `/acm-memory`, and `/acm-eval` slash commands.

If you already have this repo checked out locally, the equivalent command is `./scripts/install-skill-pack.sh --claude /path/to/your/project`.

### Codex (skill pack)

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/bonztm/agent-context-manager/main/scripts/install-skill-pack.sh) --codex
```

Installs the acm-broker skill to `~/.codex/skills/acm-broker`.

If you already have this repo checked out locally, the equivalent command is `./scripts/install-skill-pack.sh --codex`.

### MCP (tool-native models)

```bash
acm-mcp invoke --tool get_context --in payload.json
```

Twelve tools exposed — the five agent-facing operations (`get_context`, `fetch`, `work`, `propose_memory`, `report_completion`) plus seven maintenance operations (`sync`, `health_check`, `health_fix`, `coverage`, `eval`, `verify`, `bootstrap`).

## CLI Reference

All commands support `--help` for full flag documentation.

### Agent-facing (called by agents via CLI, skills, or MCP)

```bash
acm get-context    --project <id> (--task-text <text>|--task-file <path>) --phase <plan|execute|review> [--tags-file <path>]
acm fetch          --project <id> [--key <key>]... [--keys-file <path>|--keys-json <json>] [--expect <key=version>]... [--expected-versions-file <path>|--expected-versions-json <json>] [--receipt-id <id>]
acm work           --project <id> [--plan-key <key>|--receipt-id <id>] [--mode <merge|replace>] [--plan-file <path>|--plan-json <json>] [--tasks-file <path>|--tasks-json <json>] [--items-file <path>|--items-json <json>]
acm propose-memory --project <id> --receipt-id <id> --category <cat> --subject <text> (--content <text>|--content-file <path>) --confidence <1-5> --evidence-key <key> [--memory-tag <tag>]... [--memory-tags-file <path>|--memory-tags-json <json>] [--tags-file <path>] [--auto-promote]
acm report-completion --project <id> --receipt-id <id> [--file-changed <path>]... [--files-changed-file <path>|--files-changed-json <json>] (--outcome <text>|--outcome-file <path>) [--scope-mode <strict|warn|auto_index>] [--tags-file <path>]
```

Most list and text flags support inline values and `--*-file` alternatives (`-` for stdin). JSON list/object inputs also support `--*-json` for one-shot agent calls without temporary files.

### Human-facing (setup and maintenance)

```bash
acm bootstrap     --project <id> --project-root . [--persist-candidates] [--respect-gitignore] [--llm-assist-descriptions] [--output-candidates-path <path>] [--rules-file <path>] [--tags-file <path>]
acm sync          --project <id> --mode <changed|full|working_tree> [--insert-new-candidates] [--rules-file <path>] [--tags-file <path>]
acm health        --project <id> [--include-details]
acm health-fix    --project <id> --apply [--fixer <name>] [--rules-file <path>] [--tags-file <path>]
acm coverage      --project <id> --project-root .
acm eval          --project <id> (--eval-suite-path ./eval.json|--eval-suite-inline-file <path>|--eval-suite-inline-json <json>) [--minimum-recall <0..1>] [--tags-file <path>]
acm verify        --project <id> [--receipt-id <id>] [--plan-key <key>] [--phase <plan|execute|review>] [--test-id <id>]... [--file-changed <path>]... [--files-changed-file <path>|--files-changed-json <json>] [--tests-file <path>] [--tags-file <path>] [--dry-run]
```

### Structured JSON Contract Mode

`acm run` and `acm validate` operate on the full `acm.v1` request envelope. This is the canonical machine-facing contract behind the convenience CLI commands, checked-in request fixtures, and thin adapters built around acm.

Use it when you want:

- one complete JSON request per call in scripts or CI
- request fixtures checked into a repo for repeatable agent workflows
- validation of a payload before execution

Envelope shape:

```json
{
  "version": "acm.v1",
  "command": "get_context",
  "request_id": "req-get-context-001",
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

## Storage Backend

SQLite is zero-config by default. acm resolves config in this order:

1. Process environment (`ACM_*`)
2. Repo-root `.env`
3. Implicit SQLite at `<repo-root>/.acm/context.db`

When acm chooses that implicit repo-local SQLite path, it also ensures `.gitignore` contains `.acm/context.db`, `.acm/context.db-shm`, and `.acm/context.db-wal`.

Set `ACM_PG_DSN` for Postgres when you need write concurrency.

```bash
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

- [ADR-001: Context Broker](docs/architecture/ADR-001-context-broker.md) — design decisions and data model
- [Proposal: Executable Verification](docs/architecture/proposal-verify.md) — eval vs verify design
- [Logging Standards](docs/logging.md) — structured logging contract

## Canonical Rules

acm doesn't ship project rules. You author them in `.acm/acm-rules.yaml` (preferred) or `acm-rules.yaml` in the project root, and acm ingests and enforces them. Use `--rules-file` on `sync`, `health-fix`, or `bootstrap` to override discovery with an explicit path.

Canonical tag normalization starts from the embedded base dictionary and merges repo-local overrides from `.acm/acm-tags.yaml` on every runtime call. Use `--tags-file` on `get-context`, `propose-memory`, `report-completion`, `sync`, `health-fix`, `eval`, `verify`, or `bootstrap` to point acm at a non-default tag dictionary file.

Executable verification definitions live in `.acm/acm-tests.yaml` (preferred) or `acm-tests.yaml` in the project root. `bootstrap` now seeds the preferred `.acm/acm-tests.yaml` skeleton when neither canonical location exists. Use `--tests-file` on `verify` to override discovery with an explicit path. v1 definitions are argv-only and let projects define reusable repo-local verification checks without introducing a second planning model.

See [docs/examples/acm-rules.yaml](docs/examples/acm-rules.yaml) and [docs/examples/acm-tags.yaml](docs/examples/acm-tags.yaml) for the formats, and [Getting Started](docs/getting-started.md) for the full authoring and maintenance workflow.

## Logging

```bash
export ACM_LOG_LEVEL=debug   # debug|info|warn|error (default: info)
export ACM_LOG_SINK=stderr   # stderr|stdout|discard (default: stderr)
```

## License

This project is licensed under the GNU Affero General Public License v3.0 only (`AGPL-3.0-only`). See [LICENSE](LICENSE).
