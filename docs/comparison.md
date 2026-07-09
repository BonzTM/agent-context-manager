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
| Active-window ownership | augment (hosts do not expose it) | ✅ full | ✅ full | ✅ full | ✅ in-place prompt rewrite |
| Automatic recall injection on prompt | ✅ (Claude Code, Codex) | ✗ (pull only) | ✗ (pull only) | ✗ (summaries in window) | ✅ (scope-escalating) |
| Drill-down retrieval (grep/expand/describe) | ✅ shell commands | ✅ agent tools | ✅ agent tools | ✅ agent tools + slash cmds | ✅ agent tools |
| Search covers summaries | ✅ | ✅ (grouped by summary) | ✅ | ✅ | ✅ |
| Large-content offload | ✅ (token threshold, disk + type-aware exploration summaries) | ✅ (type-aware exploration summaries) | ✅ | ✗ (truncation only) | ✅ (artifact blobs, dedup, previews) |
| LLM-synthesized `expand-query` | ✅ (`--synthesize`, cited msg ids, filter fallback) | ✗ | ✅ (sub-agent, grants) | ✅ (separate model/timeout) | ✗ |
| Off-context batch map (`llm_map`) | ✅ (worker pool, validation-feedback retries) | ✅ (+ `agentic_map`, exactly-once item states) | ✗ | ✗ | ✗ |
| Works with zero infrastructure (one binary, no host fork) | ✅ | ✗ | ✗ | ✗ | ✗ (needs OpenCode runtime) |
| Multi-project routing from one install | ✅ | n/a | ✗ | ✗ | ✗ |
| Operational hygiene (doctor, backup) | ✅ integrity + FTS parity check, `VACUUM INTO` backup | unspecified | ✅ rich (TUI, rotate, repair, transplant) | ✅ (doctor, clean, backup) | ✅ (doctor, retention, snapshots, GC) |

## Where acm deliberately differs

**Augmentation over ownership.** Claude Code and Codex expose hooks and
supplemental-context injection, not control of the live message array — so on
those hosts *no* implementation can own the window, and acm is designed for
exactly that constraint: a lossless side-record, push recall at prompt time, and
pull drill-down through the shell tool the agent already has. The window-owning
implementations above each require adopting their host (volt, OpenClaw, Hermes)
or a host with a transform API (OpenCode).

**One binary, many agents, per-project state.** acm is the only implementation
in this set that covers multiple unrelated host agents from one install, keeps
each project's history in that project's own `.acm/` directory, and needs no
runtime beyond the binary itself.

**Push recall.** Surfacing relevant prior context into each new turn (the
`<acm-recall>` block) is unique to acm and opencode-lcm in this set; the
window-owning engines instead keep summaries permanently in context.

## Known gaps against the references

These are real deltas, tracked as roadmap items rather than claimed away:

- **No active-window ownership anywhere, including OpenCode.** The OpenCode
  plugin captures messages and tool calls; it does not yet rewrite the outgoing
  prompt (`experimental.chat.messages.transform`) the way opencode-lcm does, and
  OpenCode recall is drill-down only.
- **Retrieval remains lexical.** acm filters prompts to a bounded salient-term
  query, obtains BM25 candidates, and reranks by coverage, current conversation,
  role, recency, and payload size. It still lacks semantic embeddings, learned
  ranking, and opencode-lcm's scope escalation.
- **No `agentic_map`.** acm ships `llm_map` mechanics (worker pool, required
  fields, validation-feedback retries); volt's tool-using per-item sub-agents
  and exactly-once DB-backed item states are out of scope for a hookable CLI.
- **Coarser session lifecycle.** Session filtering patterns, retention pruning,
  pinning, and cross-session carry-over (lossless-claw, hermes-lcm,
  opencode-lcm) are not implemented; acm's per-project databases keep the blast
  radius small, but offer no per-session policy.
