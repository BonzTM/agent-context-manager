# agent-context-manager (`acm`)

**Lossless long-context management for AI coding agents.** A single local binary
that gives Claude Code, Codex, and OpenCode durable, recoverable context that
survives compaction — so your agents stop forgetting and stop re-reading.

[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)
![Go 1.26+](https://img.shields.io/badge/Go-1.26%2B-00ADD8.svg)

> [!NOTE]
> Not to be confused with the unrelated project once named *agent-context-manager*
> that was renamed [agent-workflow-manager](https://github.com/bonztm/agent-workflow-manager).
> This is a different tool with a different purpose.

---

## Overview

As an agent conversation grows, the model's context window fills and older turns
are summarized away or dropped. Detail is lost, and the agent compensates by
re-reading files and repeating work.

`acm` solves this by maintaining a **lossless record** of every conversation in a
small per-project SQLite database. As context grows, older spans are compacted
into a hierarchy of summaries — but the verbatim originals are never destroyed,
and the agent can recover any of them on demand. The approach follows the
[Lossless Context Management](https://papers.voltropy.com/LCM) model: deterministic,
engine-owned context management with a guarantee of lossless retrievability.

It is **zero-infrastructure**: no service to run, no database to provision, no
network server. Just a binary, your agent's hooks, and a `.acm/` directory in
each project.

## Key capabilities

- **Lossless capture** — every user, assistant, and tool message is persisted
  verbatim. Ingestion is idempotent, so re-reading a transcript never duplicates.
- **Summary DAG compaction** — older context is folded into leaf and condensed
  summaries under a configurable token budget, while a protected "fresh tail" of
  recent messages is always kept raw.
- **On-demand recovery** — any summary expands back to its exact source messages.
  The agent drills down through its normal shell tool (`acm expand`, `acm grep`).
- **Automatic recall** — relevant prior context is surfaced into each new turn.
- **Large-file offload** — oversized payloads are moved to disk with a compact
  exploration summary, keeping the working context lean.
- **Off-context batch processing** — `acm map` processes arbitrarily large
  datasets through a bounded worker pool without the data ever entering the
  agent's context window.
- **Bring-your-own model** — optional LLM summarization reuses the host agent's
  own model in headless mode, so `acm` holds no separate credentials. A fully
  offline deterministic summarizer is the default and the fallback.

## How it works

```
agent turn ──hook──▶ acm (capture)              ┌─ verbatim messages (lossless)
                          │                      │
                          ▼                      ▼
              per-project .acm/acm.db ──▶  summary DAG  ◀── compaction (token budget)
                          ▲                      │
                          │                      ▼
agent shell ──acm expand/grep──▶ recover ◀─ lossless pointers
```

The binary is the single source of truth. Agents integrate through hooks
(capture and recall injection) and recover detail through their existing shell
tool. See [docs/architecture.md](docs/architecture.md) for the full design.

## Installation

Requires Go 1.26 or newer.

```sh
go install github.com/bonztm/agent-context-manager/cmd/acm@latest
```

Or build from source (produces a static, dependency-free binary):

```sh
git clone https://github.com/bonztm/agent-context-manager
cd agent-context-manager
CGO_ENABLED=0 go build -o acm ./cmd/acm
```

## Quick start

Install acm once into an agent's global configuration and it covers every
project — the per-project database is resolved from the working directory at hook
time, and a `.acm/` directory is created on first use:

```sh
acm init claude-code --global          # preview the changes (dry run)
acm init claude-code --global --apply  # install for every project
```

The install is safe and idempotent: it merges acm's hooks and drill-down
instructions into your existing config without overwriting other settings, and
re-running changes nothing. Repeat for `codex` and `opencode` as needed.

Prefer per-project, committable setup instead? Omit `--global` to generate
snippets under `.acm/init/<agent>/` for you to merge:

```sh
cd your-project
acm init claude-code      # writes snippets + instructions, never edits your config
```

Either way, `acm` then runs automatically as you work. You can also drive it
directly:

```sh
acm stats                 # what's stored
acm grep "auth refactor"  # search the full history
acm compact               # compact conversations under the token budget
acm window <conversation> # inspect the assembled active context
acm expand <summary-id>   # recover a summary's verbatim sources
```

## Integrations

| Agent | Capture | Automatic recall | Drill-down | Active-window control |
|-------|:-------:|:----------------:|:----------:|:---------------------:|
| Claude Code | ✅ hooks | ✅ | ✅ shell tool | augment |
| Codex | ✅ hooks | ✅ | ✅ shell tool | augment |
| OpenCode | ✅ plugin | ✅ | ✅ shell tool | full (plugin) |

Setup for each agent is in [docs/integrations.md](docs/integrations.md).

## Commands

| Command | Purpose |
|---------|---------|
| `acm init <agent>` | Generate integration assets for an agent |
| `acm hook` | Handle an agent hook event (capture + recall injection) |
| `acm ingest` | Ingest captured messages (JSON on stdin) |
| `acm grep <query>` | Search the lossless message history |
| `acm describe <id>` | Show a message, summary, or offloaded file |
| `acm compact` | Compact conversations into the summary DAG |
| `acm expand <id>` | Expand a summary to its verbatim sources |
| `acm expand-query <id> <query>` | Expand a summary, filtered to matching messages |
| `acm window <id>` | Show a conversation's assembled active context |
| `acm map` | Process a JSONL dataset off-context (bounded worker pool) |
| `acm stats` | Report aggregate store counts |
| `acm doctor` | Open the database, migrate, and report health |
| `acm version` | Print version information |

## Configuration

`acm` stores per-project state under `<project>/.acm/`. The database path resolves
automatically (`$LCM_DB` → `$CLAUDE_PROJECT_DIR/.acm/acm.db` → nearest ancestor
`.acm/` → `<cwd>/.acm/acm.db`). Compaction thresholds and the summarizer are
tunable via flags. See [docs/configuration.md](docs/configuration.md).

## Data and privacy

All state is local to each project's `.acm/` directory. `acm` opens no network
connections and runs no daemon. Optional LLM summarization invokes your already
authenticated agent CLI; the deterministic default makes no external calls at all.

## Development

```sh
make verify    # tidy, format check, lint, vet, test, race, vuln scan, build
make help      # list all targets
```

See [CONTRIBUTING.md](CONTRIBUTING.md) for conventions and project layout.

## License

[MIT](LICENSE)
