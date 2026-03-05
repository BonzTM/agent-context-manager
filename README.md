# agents-context

Deterministic context broker for LLM agents.

## Scope

This repository hosts:

- shared core service interfaces
- `ctx` CLI adapter
- `ctx-mcp` adapter surface
- versioned wire contracts in `spec/v1`

Current implementation focus is contract-accurate request/response handling and command/tool dispatch wiring.

## Quick start

```bash
go test ./...
go run ./cmd/ctx --help
go run ./cmd/ctx-mcp --help
```

## CLI usage

The v1 JSON envelope flow is still available:

```bash
ctx run --in request.json
ctx validate --in request.json
```

Convenience subcommands build the same v1 envelope internally:

```bash
# Context retrieval
ctx get-context --project soundspan --task-text "Add health checks" --phase execute
ctx fetch --project soundspan --key plan:req-12345678 --expect plan:req-12345678=v3

# Work + memory
ctx work --project soundspan --receipt-id req-12345678 --items-file ./work-items.json
ctx propose-memory --project soundspan --receipt-id req-12345678 --category decision --subject "Use shared logger" --content "Prefer wrappers" --confidence 4 --evidence-key rule:ctx/rule-1
ctx report-completion --project soundspan --receipt-id req-12345678 --file-changed cmd/ctx/main.go --outcome "Implemented convenience commands"

# Maintenance
ctx sync --project soundspan --mode changed --git-range HEAD~1..HEAD
ctx health --project soundspan --include-details
ctx health-fix --project soundspan --apply --fixer sync_working_tree
ctx coverage --project soundspan --project-root .
ctx regress --project soundspan --eval-suite-path ./eval/ctx.json --minimum-recall 0.9
ctx bootstrap --project soundspan --project-root . --respect-gitignore
```

## Canonical Rules Onboarding Examples

Starter templates for downstream onboarding:

- `docs/examples/canonical-ruleset.yaml`
- `docs/examples/AGENTS.md`
- `docs/examples/CLAUDE.md`

Canonical ruleset files are discovered at `.ctx/canonical-ruleset.yaml` (preferred) or `ctx-rules.yaml` and must use `version: ctx.rules.v1`.

Rule maintenance flow:

1. Add/remove/update rule entries in your canonical ruleset.
2. Run `sync` or `health_fix apply`.
3. Run `health_check` and resolve blocking findings.

See `docs/ADR-001-context-broker.md` for command intent and maintenance context.

## Runtime backend

- Default: `ctx run` and `ctx-mcp invoke` use the SQLite-backed runtime service.
- Postgres override: set `CTX_PG_DSN` to wire both commands to the Postgres-backed service.
- SQLite path override: set `CTX_SQLITE_PATH` to choose the SQLite database file location.
  - default path: `<user-cache-dir>/agents-context/context.db` (falls back to `/tmp/agents-context-context.db` when user cache dir is unavailable).
  - operational guidance: `docs/SQLITE_OPERATIONS.md`

```bash
export CTX_PG_DSN='postgres://user:pass@localhost:5432/agents_context?sslmode=disable'
export CTX_SQLITE_PATH='/absolute/path/to/agents-context.db'
```

## SQLite hardening checklist

1. Set `CTX_SQLITE_PATH` explicitly for deployed environments.
2. Keep the DB on persistent storage (not `/tmp`).
3. Restrict permissions (`0700` directory, `0600` database file).
4. Schedule online backups and retention cleanup.
5. Switch to Postgres (`CTX_PG_DSN`) when write concurrency/volume grows.

## Skills integration

This repo includes a portable skill package for skill-capable agents:

- `skills/ctx-broker/SKILL.md`
- `skills/ctx-broker/references/templates.md`
- `skills/ctx-broker/assets/requests/*.json`

Install locally into Codex skills:

```bash
mkdir -p "$HOME/.codex/skills"
cp -R skills/ctx-broker "$HOME/.codex/skills/ctx-broker"
```

Then restart the client to load the skill metadata.

Claude Code install (slash-command pack):

```bash
mkdir -p .claude/commands
cp skills/ctx-broker/claude/commands/*.md .claude/commands/
```

Then restart Claude Code to reload commands.

One-command installer (Codex + Claude):

```bash
./scripts/install-skill-pack.sh
```

Optional flags:

```bash
./scripts/install-skill-pack.sh --claude-target /path/to/project
./scripts/install-skill-pack.sh --skip-claude
./scripts/install-skill-pack.sh --skip-codex
```
