# ADR-001: Deterministic Context Broker (`acm`) for LLM Task Context and Memory

> Historical note: this ADR describes the original broker/retrieval direction. It is no longer the current architecture contract.

- Status: Superseded
- Original Date: 2026-03-04
- Superseded Date: 2026-03-10

## Why It Was Superseded

The original design centered ACM around deterministic retrieval, pointer ranking, hop expansion, and eval-style tuning. The current unreleased product direction is narrower:

- `context` is now a retrieval-free core operation
- ranked retrieval, hop expansion, and budget estimation are no longer part of the supported public story
- `eval` has been removed
- standalone `coverage` has been folded into health/status
- governed closure depends on effective scope and baseline-derived task deltas, not on ranked pointer selection

## Current Architecture Summary

Use the current docs as the source of truth:

- [Getting Started](../getting-started.md)
- [Concepts](../concepts.md)
- [Schema Reference](../../spec/v1/README.md)

Current core model:

1. `context` returns hard rules, active plans, relevant memory, optional `initial_scope_paths`, and a context-time baseline.
2. Agents search the repo natively.
3. If governed file work expands beyond the initial receipt scope, agents record those paths through `work.plan.discovered_paths`.
4. `verify`, runnable `review`, and `done` compute the real task delta from the receipt baseline.
5. Effective scope is `initial_scope_paths + discovered_paths + ACM-managed governance paths`.

This file is retained only as design history for how ACM reached the simpler control-plane model.
