# Architecture

`acm` is a local-first context manager for AI coding agents. It keeps a complete,
verbatim record of every conversation and presents the agent with a compacted —
but fully recoverable — view of that history. The design follows the
[Lossless Context Management](https://papers.voltropy.com/LCM) model: context
management is handled by a deterministic engine rather than left to the model,
with a guarantee that every original message remains retrievable.

## Design goals

- **Lossless.** Nothing the agent produces is ever destroyed. Summaries are a
  derived view; the verbatim originals are the source of truth.
- **Local and zero-infrastructure.** One binary, an embedded SQLite database per
  project, no service, no network listener.
- **Deterministic.** Compaction is engine-owned and reproducible. Summarization
  has a deterministic default and a guaranteed-terminating fallback.
- **Universal.** A single core serves multiple host agents through thin adapters.

## Two tiers

**1. Universal core** — the engine in the `acm` binary, backed by
`<project>/.acm/acm.db`. Identical across every agent. It owns the lossless
store, the summary DAG, compaction, retrieval, and batch processing.

**2. Per-agent adapters** — thin integration layers. They capture messages from
each agent and inject recalled context. The degree of control varies by what each
agent exposes: OpenCode permits deterministic ownership of the active window;
Claude Code and Codex are augmented alongside their own context handling.

## Components

### Lossless store

Every message is written verbatim to the `messages` table before any compaction.
Each message carries a content-derived identity hash; the message ID is derived
from `(conversation, identity hash)`, so re-ingesting the same source line is a
no-op. A conversation is keyed by `(agent, session id)` with a derived ID, making
ingestion idempotent without read-back. Full-text search (SQLite FTS5) indexes all
message content.

### Summary DAG

As the active context grows, older spans are compacted into **summary nodes**
forming a directed acyclic graph:

- **Leaf summaries** (depth 0) cover spans of raw messages.
- **Condensed summaries** (depth ≥ 1) cover lower summaries, so the history gains
  a multi-resolution structure.

Every node records **lossless pointers** — `summary_messages` links a leaf to its
source messages; `summary_parents` links a condensed node to its children — so any
node can be walked back to the verbatim originals.

### Compaction loop

Compaction is governed by a two-threshold token budget over the active window:

1. Below the soft threshold, nothing happens — the store is a passive log with no
   overhead.
2. Above it, the oldest compactible span (outside the protected fresh tail) is
   folded into a leaf summary and replaced in the window by a pointer.
3. When enough same-depth summaries accumulate, they are condensed into a
   higher-depth node.

The most recent messages (the **fresh tail**) are never compacted.

### Escalating summarization

Each summary is produced under a size guard, so a summary is never larger than its
input:

1. **Normal** — summarize to the target size.
2. **Aggressive** — retry at half the target.
3. **Deterministic truncation** — a guaranteed-terminating fallback that needs no
   model.

The default summarizer is deterministic (structural, offline). Optionally, `acm`
reuses the host agent's own model in headless mode for higher-quality summaries,
falling back to deterministic on any error.

### Large-file offload

Message content above a token threshold is written to disk under `.acm/files/`,
recorded in `large_files` with a compact exploration summary, and represented in
the working context by that summary rather than the full payload. Exploration
summaries are type-aware: JSON, CSV/TSV, SQL, and source code get a
deterministic schema- or structure-level description with no model call
(recorded in the row's `extractor` column and shown by `acm describe`);
unstructured content falls to the configured summarizer, then to truncation.
The verbatim content remains recoverable both from the store and from the
on-disk file.

### Retrieval

The agent recovers detail through its normal shell tool:

- `acm grep` — full-text or substring search across the entire history,
  including summary content (compacted spans stay findable).
- `acm expand` — reverse compaction: a summary's direct sources, or a full walk
  down to every verbatim message.
- `acm describe` — metadata and content for any message, summary, or offloaded
  file.

Relevant prior context is also surfaced automatically into new turns by the
capture/recall hook.

### Off-context batch processing (`acm map`)

For bulk work over large datasets, `acm map` reads a JSONL file, processes each
item independently through a bounded worker pool with validated retries, and
writes results to a JSONL file. The dataset never enters the agent's context
window, which keeps accuracy stable regardless of dataset size.

## Data model

| Table | Purpose |
|-------|---------|
| `conversations` | One row per agent session (keyed by agent + session id) |
| `messages` | Verbatim, append-only message store |
| `messages_fts` | Full-text index over message content |
| `summaries` | Summary DAG nodes (leaf and condensed) |
| `summary_messages` | Lossless pointers: leaf → source messages |
| `summary_parents` | DAG edges: condensed → child summaries |
| `summaries_fts` | Full-text index over summary content |
| `context_items` | The ordered active window (messages + summary pointers) |
| `large_files` | Offloaded payloads with exploration summaries |

Schema is managed with embedded, ordered migrations applied automatically when the
database is opened.

## Storage and concurrency

The database is pure-Go SQLite (`modernc.org/sqlite`), so the binary builds
statically with no C toolchain. SQLite is single-writer; within a process the
connection pool is capped at one connection, and every transaction begins as
`BEGIN IMMEDIATE` so concurrent acm processes (agent hooks routinely fire in
parallel) queue on the busy timeout instead of failing a deferred read-to-write
upgrade. Concurrent capture is therefore lossless across processes, which is
covered by a dedicated regression test.
