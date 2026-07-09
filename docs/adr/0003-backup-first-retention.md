# 3. Retention is explicit, backup-first, and expansion-gated

- Status: Accepted
- Date: 2026-07-09

## Context

ACM's defining invariant is recoverability of retained canonical messages.
Retention intentionally destroys that source data, its summary DAG, search
index entries, and offloaded payloads. An age threshold alone cannot distinguish
forgotten history from history a user expects to drill into later.

## Decision

Retention is an explicit maintenance operation with these invariants:

1. `acm prune` defaults to dry-run and prints the exact bounded candidate set.
2. `--apply` creates and integrity-checks an owner-only SQLite snapshot before
   beginning deletion.
3. Pins always win; `--force` cannot override a pin.
4. Conversations with any summary root that has never been successfully
   expanded are skipped unless `--force` is explicit.
5. Database rows and FTS entries are deleted in one transaction. Offload files
   are removed after commit, and cleanup failures preserve the backup path in
   the error.
6. Carry-over pins its source so copied summary pointers cannot be pruned from
   underneath the target session.

## Consequences

- "Lossless" applies to retained canonical history. Applied retention is a
  deliberate, auditable exception with a verified rollback snapshot.
- Expansion acknowledgement is conservative: expanding one root does not imply
  that unrelated summary roots were reviewed.
- Backups may contain data that policy or retention later removes and must be
  governed as sensitive local state.
- There is no scheduled pruning or hidden retention timer.
