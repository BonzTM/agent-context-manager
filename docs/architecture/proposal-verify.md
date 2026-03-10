# Proposal: Executable Verification v1 for acm

> Historical note: this proposal predates the Phase 2 simplification. It is retained as design history, not as the current contract.

- Status: Superseded in part
- Original Date: 2026-03-05
- Superseded Date: 2026-03-10

## What Changed

The original proposal treated `eval` and `verify` as parallel public surfaces. That is no longer true.

Current state:

- `verify` remains the executable verification surface
- `eval` has been removed from the public product and contract
- repositories define executable checks in `.acm/acm-tests.yaml`
- `verify` selects repo-defined checks from phase, changed files, tags, and explicit test ids
- `verify` updates `verify:tests` when receipt or plan context is present

## Current Contract Pointers

Use these as the living sources of truth:

- [Getting Started](../getting-started.md#verify)
- [Concepts](../concepts.md#executable-verification-definitions)
- [Schema Reference](../../spec/v1/README.md)

## Historical Takeaway

The important surviving design choice from this proposal is that executable verification is separate from planning and completion state, but durable enough to feed definition-of-done tracking through `work` / `verify:tests`.
