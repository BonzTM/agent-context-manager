# 1. Agents recover compacted context through their shell tool

- Status: Accepted
- Date: 2026-06-18

## Context

`acm` keeps a lossless record of every conversation and compacts older context
into a summary DAG. For this to be useful, the agent must be able to recover the
verbatim detail behind a summary on demand — mid-task, when it decides it needs
it.

Most of `acm`'s work needs no agent-callable interface at all: capture happens
through hooks, compaction runs against the local database, and recall is injected
by hooks. Storage, status, and maintenance are ordinary CLI commands. The only
capability a locally invoked binary cannot provide on its own is **model-initiated
retrieval** — the model deciding, during its turn, to fetch the original behind a
summary.

Coding agents already expose a general-purpose shell tool. The model can run
`acm expand` / `acm grep` / `acm describe` through that tool, with the recovered
content returned as ordinary tool output.

## Decision

Agent-initiated retrieval is implemented as `acm` CLI subcommands invoked through
the agent's existing shell tool. `acm init` documents these commands in the
project's agent instructions and grants the binary a standing permission so the
calls do not prompt.

`acm` runs **no network server and no background daemon**.

## Consequences

**Positive**

- No additional runtime surface: no long-running process, no per-agent server
  registration, no network listener. This keeps the security and operational
  footprint minimal — relevant for environments that restrict outbound network
  access.
- Aligns with the local-binary, hook-based integration model.
- The same mechanism works uniformly across every supported agent.

**Trade-offs**

- Retrieval results are returned as text rather than schema-validated structures.
- On agents that sandbox or gate the shell tool, the binary must be granted a
  standing permission (handled by `acm init`).
- Discoverability relies on the injected instruction block rather than a tool the
  agent lists natively.

## Alternatives considered

A typed tool interface (exposing retrieval as first-class agent tools) was
considered. It would add a persistent process and per-agent registration for a
capability the shell tool already covers, so it was rejected for the current
design. The retrieval commands are structured so that such an interface could
wrap them later without changing the engine.
