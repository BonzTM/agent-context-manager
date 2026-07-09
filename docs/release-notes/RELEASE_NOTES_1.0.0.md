# [1.0.0] Release Notes - 2026-07-09

The first release of `acm` (agent-context-manager): lossless long-context management for AI coding agents. One local binary gives Claude Code, Codex, and OpenCode a durable, recoverable record of every conversation — captured verbatim into a per-project SQLite database, compacted into a hierarchy of summaries as context grows, and recoverable on demand through the shell tool your agent already has. No service to run, no database to provision, no network listener, no API keys.

## Your agent stops forgetting

As a session grows, agents summarize away or drop older turns, then compensate by re-reading files and repeating work. acm keeps the originals. Every user prompt, tool result, and assistant turn its hooks can see is stored verbatim the moment it happens — and capture is built to survive the real world:

- **Concurrent hooks never lose a message.** Agent hooks fire in parallel; every acm write takes the database lock up front, so simultaneous captures queue instead of failing. This is covered by a dedicated multi-process regression test.
- **Ingestion is idempotent everywhere.** Re-reading a transcript, a re-fired hook, a retried notify — all collapse into the messages already stored.
- **Claude Code assistant turns are captured too.** Claude Code's hooks don't carry assistant text, so acm reconciles it from the session transcript on every `Stop` event, deduplicated by transcript line.

## Context compacts; nothing is destroyed

Above a configurable token budget, older spans fold into **leaf summaries**, and stacks of summaries condense into higher-level ones — a summary DAG in which every node keeps lossless pointers back to its verbatim sources. The most recent messages are never compacted, and compaction runs opportunistically from the capture hooks as you work, with a deterministic offline summarizer as the default. Optionally, `acm compact --summarizer claude|codex` reuses your agent's own model in headless mode for higher-quality summaries — acm holds no credentials of its own, and any model failure falls back to the deterministic path, so compaction always succeeds.

## Anything can be recovered, mid-task

The model drills down through ordinary shell commands, documented for it at install time:

- `acm grep` — relevance-ranked full-text search across the entire history, including summary content, or a literal substring scan.
- `acm expand` — reverse compaction: a summary's direct sources, or the full walk down to every verbatim message.
- `acm expand-query --synthesize` — ask a question of a compacted span and get a focused answer from your agent's model, citing the exact `msg_` ids it drew on; without a model available it degrades to plain filtered output.
- `acm describe` — metadata and content for any message, summary, or offloaded file.

Relevant prior context is also surfaced automatically into each new turn on Claude Code and Codex, injected by the prompt hook before the prompt itself is stored.

## Big payloads stay out of the window

Message content above a token threshold is offloaded to disk and represented by a compact **exploration summary**. These are type-aware: JSON gets a key/shape schema, CSV gets rows-by-columns and a header, SQL gets statement kinds and tables touched, and source code gets a declaration outline — all deterministic, with zero model calls. `acm describe` shows which extractor produced each description.

## Work over data without spending context

`acm map` processes a JSONL dataset item-by-item through a fixed worker pool — optionally with your agent's model — validating each output and feeding failures back into retries. The dataset never enters the agent's context window, so accuracy doesn't degrade with input size.

## One install covers every project

```sh
go install github.com/BonzTM/agent-context-manager/cmd/acm@latest
acm init claude-code --global    # --dry-run to preview; repeat for codex, opencode
```

The global install is idempotent and atomic: it merges acm's hooks into your existing configuration without touching anything else, and re-running changes nothing. The per-project database is resolved from the working directory at hook time, and each project's `.acm/` directory is created on first use with a self-excluding `.gitignore`.

## Built to be operated

- `acm doctor` — opens and migrates the database, runs a SQLite integrity check, and verifies the full-text indexes match the base tables.
- `acm backup` — a consistent snapshot copy via `VACUUM INTO`, safe against concurrent writers.
- `acm stats`, `acm version` — store counts and build identification (version metadata is reported correctly even for `go install`-built binaries).
- Structured logs (`--log-level`, `--log-json`, or `ACM_LOG_LEVEL` / `ACM_LOG_JSON`), embedded ordered schema migrations applied automatically on open, and CI that runs the same `make verify` gate contributors run locally.

## Data and privacy

All state lives in each project's `.acm/` directory. acm opens no network connections and runs no daemon. Treat `.acm/` contents as sensitive — a verbatim history contains whatever appeared in prompts and tool output. Optional LLM summarization invokes your locally installed, already-authenticated agent CLI as a subprocess; the deterministic default makes no external calls at all.

## Getting started

Fresh install — there is no upgrade path to consider. Requires Go 1.26+ to build or install from source; release binaries for linux/darwin (amd64/arm64) are attached to this release. See the [README](../../README.md) for the quick start, [docs/integrations.md](../integrations.md) for per-agent setup, and [docs/comparison.md](../comparison.md) for how acm relates to the other implementations of the [Lossless Context Management](https://papers.voltropy.com/LCM) model.
