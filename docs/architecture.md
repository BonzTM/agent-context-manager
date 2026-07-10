# Architecture

`acm` is a local-first context manager for AI coding agents. It keeps a complete
record of every privacy-policy-permitted conversation and presents the agent
with a compacted, fully recoverable view of retained history. The design follows the
[Lossless Context Management](https://papers.voltropy.com/LCM) model: context
management is handled by a deterministic engine rather than left to the model,
with a guarantee that every original message remains retrievable.

## Design goals

- **Lossless after policy.** Retained canonical messages are never destroyed.
  Summaries are derived; excluded data is intentionally absent and recognized
  secrets are replaced before the canonical source is created. Explicit
  backup-first retention is the documented destructive exception.
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

Every permitted message is filtered and redacted before it reaches the
`messages` table or FTS. The resulting canonical message is retained before any compaction.
Each message carries an identity hash derived from its source event ID, its raw
hook payload, or finally its role and content for direct imports with neither.
This makes source-event replay a no-op without collapsing equal text from two
different hook events. A conversation is keyed by `(agent, session id)` with a
derived ID, making ingestion idempotent without read-back. Full-text search
(SQLite FTS5) indexes all message content.

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
OpenCode's owning adapter invokes the same bounded loop before each prompt even
below the soft threshold, folding every eligible older message so the outgoing
array can be represented by summary roots plus the protected raw tail. Claude
Code and Codex retain the threshold-triggered behavior because their hooks
cannot replace the host message array.

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
capture/recall hook. It extracts a bounded set of salient prompt terms, obtains
bounded BM25 message and summary candidates, excludes the current protected
raw tail, then reranks them by lexical coverage, current conversation, role,
recency, active-summary status, source order, and payload size. Low-signal
prompts inject no recall block. A fixed-clock fixture corpus gates exact top-k,
Recall@k, MRR, and deterministic ordering.

`acm window` renders ACM's persisted, synthetic active view. OpenCode's message
transform applies that view to the outgoing prompt and injects automatic recall.
The Claude Code and Codex adapters cannot replace the host's live message array,
so the view is diagnostic on those hosts; they receive supplemental recall.

### Off-context batch processing (`acm map`)

For bulk work over large datasets, `acm map` validates and streams a JSONL file,
processes each item independently through a bounded worker pool with validated
retries, and spools synced per-item state to disk. Successful runs assemble the
states in input order and publish the output with a same-directory rename only
after it has been flushed and synced. Interrupted runs leave their state for a
compatible restart, which skips completed and terminally failed items. The
dataset never enters the agent's context window, and in-memory work is bounded
by worker concurrency rather than item count.

Single-response processors call a headless model once per attempt. The
`claude-agent` and `codex-agent` processors instead consume each host's JSONL
event stream while a tool-using session runs under host read-only controls. ACM
counts turns and tool starts, applies an item deadline, bounds event bytes and
count, and kills the process group when a limit is crossed. Both modes use the
same persisted `pending`/`running`/`completed`/`failed` item contract.

Optional JSON Schema output validation is compiled offline before the run. The
schema hash forms part of the resume identity, and validation errors enter the
same bounded feedback/retry path as processor errors.

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
