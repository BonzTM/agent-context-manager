# How acm compares to other LCM implementations

Several implementations of the [Lossless Context Management](https://papers.voltropy.com/LCM)
model exist, each shaped by what its host agent allows. This page maps acm's
surface against them so you can judge fit — and so acm's own claims stay honest.

The implementations compared:

- **volt** — the paper's reference implementation: a full coding agent (an
  OpenCode fork) whose engine owns the active context outright, backed by an
  embedded PostgreSQL store.
- **lossless-claw** — an OpenClaw context-engine plugin (TypeScript) that
  replaces OpenClaw's compaction and owns window assembly, with a companion
  operations TUI.
- **hermes-lcm** — a Hermes Agent context-engine plugin (Python) that owns the
  active window through the host's pluggable engine slot.
- **opencode-lcm** — an OpenCode plugin (TypeScript) that rewrites the outgoing
  prompt in place, with deterministic (no-LLM) summarization only.

## Capability matrix

| Capability | acm | volt | lossless-claw | hermes-lcm | opencode-lcm |
|---|---|---|---|---|---|
| Host agents | Claude Code, Codex, OpenCode | itself (own agent) | OpenClaw | Hermes | OpenCode |
| Storage | per-project SQLite (`.acm/`) | embedded PostgreSQL | one global SQLite | one per-profile SQLite | per-project SQLite (`.lcm/`) |
| Verbatim lossless store | ✅ | ✅ | ✅ | ✅ | ✅ (message/part level) |
| Summary DAG (leaf + condensed) | ✅ | ✅ | ✅ | ✅ | ✅ (balanced tree, rebuilt) |
| Two-threshold token budget | ✅ soft/hard | ✅ soft async / hard blocking | approximated (proactive + overflow) | ✅ (fraction + chunk floor) | ✗ (char/message counts) |
| Protected fresh tail | ✅ | ✅ | ✅ | ✅ | ✅ |
| Escalating summarization (normal → aggressive → deterministic truncate) | ✅ | ✅ | ✅ | ✅ | ✗ (deterministic only) |
| Deterministic summarizer as primary mode | ✅ | ✗ (L3 fallback only) | ✗ (L3 fallback only) | ✗ (L3 fallback only) | ✅ |
| Active-window ownership | ✅ OpenCode transform; augment on Claude Code/Codex | ✅ full | ✅ full | ✅ full | ✅ in-place prompt rewrite |
| Automatic recall injection on prompt | ✅ (all supported hosts) | ✗ (pull only) | ✗ (pull only) | ✗ (summaries in window) | ✅ (scope-escalating) |
| Drill-down retrieval (grep/expand/describe) | ✅ shell commands | ✅ agent tools | ✅ agent tools | ✅ agent tools + slash cmds | ✅ agent tools |
| Search covers summaries | ✅ | ✅ (grouped by summary) | ✅ | ✅ | ✅ |
| Large-content offload | ✅ (token threshold, disk + type-aware exploration summaries) | ✅ (type-aware exploration summaries) | ✅ | ✗ (truncation only) | ✅ (artifact blobs, dedup, previews) |
| LLM-synthesized `expand-query` | ✅ (`--synthesize`, cited msg ids, filter fallback) | ✗ | ✅ (sub-agent, grants) | ✅ (separate model/timeout) | ✗ |
| Off-context batch map (`llm_map`) | ✅ (streaming, resumable, JSON Schema, read-only `agentic_map`) | ✅ (`agentic_map`, exactly-once item states) | ✗ | ✗ | ✗ |
| Works with zero infrastructure (one binary, no host fork) | ✅ | ✗ | ✗ | ✗ | ✗ (needs OpenCode runtime) |
| Multi-project routing from one install | ✅ | n/a | ✗ | ✗ | ✗ |
| Operational hygiene (doctor, backup) | ✅ integrity + FTS parity check, `VACUUM INTO` backup | unspecified | ✅ rich (TUI, rotate, repair, transplant) | ✅ (doctor, clean, backup) | ✅ (doctor, retention, snapshots, GC) |

## Where acm deliberately differs

**Ownership where the host permits it.** Claude Code and Codex expose hooks and
supplemental-context injection, not control of the live message array. On those
hosts acm keeps a lossless side-record, pushes recall at prompt time, and offers
drill-down through the existing shell tool. OpenCode exposes a message transform,
so acm additionally replaces archived payloads with summary pointers while
preserving the protected fresh tail.

**One binary, many agents, per-project state.** acm is the only implementation
in this set that covers multiple unrelated host agents from one install, keeps
each project's history in that project's own `.acm/` directory, and needs no
runtime beyond the binary itself.

**Push recall.** Surfacing relevant prior context into each new turn (the
`<acm-recall>` block) is unique to acm and opencode-lcm in this set; the
window-owning engines instead keep summaries permanently in context.

## Known gaps against the references

These are real deltas, tracked as roadmap items rather than claimed away:

- **Retrieval remains lexical.** acm filters prompts to a bounded salient-term
  query, obtains BM25 candidates, and reranks by coverage, current conversation,
  role, recency, and payload size. It still lacks semantic embeddings, learned
  ranking, and opencode-lcm's scope escalation.
