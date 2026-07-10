# [1.2.0] Release Notes - 2026-07-10

Version 1.2.0 completes the repository's initial roadmap: OpenCode can now use
ACM's summary DAG as its outgoing active window, all supported hosts receive the
same bounded automatic recall, and the operational surfaces around capture,
privacy, retention, diagnostics, and batch processing are explicitly governed.

This release adds one automatic SQLite migration for conversation pins and
summary-expansion acknowledgements. Opening a project with the new binary,
including through `acm doctor`, applies it in place.

## OpenCode active-window ownership

The single plugin installed by `acm init --global opencode` now uses OpenCode's
message, system, and compaction transforms. Before a prompt is sent, the plugin
asks the new bounded `acm opencode-context` protocol to:

- archive eligible messages outside the count-and-token protected fresh tail;
- preserve active summary roots and their `acm expand` recovery pointers;
- append the same bounded automatic recall used by the hook integrations; and
- retain a compact resume note through OpenCode compaction.

Synthetic archive and recall parts carry metadata and reserved prefixes. Both
the plugin and the ingestion policy reject them before persistence, preventing
the transformed prompt from being recursively captured. See [PR #22](https://github.com/BonzTM/agent-context-manager/pull/22).

Claude Code and Codex remain augmentation-only because their hook APIs do not
expose the outgoing message array; they continue to receive supplemental recall
and lossless shell drill-down.

## Capture health and bounded compaction

`acm doctor` now checks Codex notify/hook installation, executable resolution,
capture freshness, file modes, and prompt-bearing conversations without an
assistant row. `acm backfill` performs a bounded, dry-run-first rollout scan and
can reconcile missing final assistant turns idempotently by turn ID. Codex
global installation also adds a redundant `Stop` reconciliation hook. See
[PR #16](https://github.com/BonzTM/agent-context-manager/pull/16).

Compaction policy rejects invalid fractions, unreachable budgets, unsafe finite
bounds, and summary targets that cannot fit their chunks before changing the
window. The fresh tail now has message-count and token floors, while tool output
remains eligible for early compaction and offload. See [PR #15](https://github.com/BonzTM/agent-context-manager/pull/15).

`acm window --breakdown` reports exact rendered and stored estimates, role/depth
subtotals, represented-message coverage, gaps/overlaps, offload references, and
the estimator name. Existing JSON output remains compatible with additive item
fields. See [PR #14](https://github.com/BonzTM/agent-context-manager/pull/14).

## Privacy and lifecycle governance

Project-root `.acm-policy.toml` files can exclude sessions, tools, structured
paths, and content classes. Deterministic secret redaction defaults on and runs
before conversation rows, raw payloads, FTS, summaries, offloads, and backups
can receive the original value. The local-data threat model documents the
owner-only filesystem boundary and why database encryption remains deferred.
See [PR #17](https://github.com/BonzTM/agent-context-manager/pull/17).

Session policy distinguishes ignored sessions from stateless recall-only
sessions. `acm prune` remains a preview unless `--apply` is explicit and requires
a verified backup; `acm pin` exempts conversations, and `acm carry-over` seeds a
new session from a bounded summary layer while pinning its source. See
[PR #18](https://github.com/BonzTM/agent-context-manager/pull/18).

## Recall and batch processing

Automatic recall now searches raw messages plus active and historical summary
candidates, excludes the protected current tail, applies separate summary/result
quotas, and emits type-correct `acm describe` or `acm expand` guidance. A
fixed-clock anonymized corpus gates exact top-k order, Recall@k, and MRR. See
[PR #21](https://github.com/BonzTM/agent-context-manager/pull/21).

`acm map` preflights complete input, streams through a bounded worker queue,
enforces input/item/output/call/time limits, writes synced per-item state, and
resumes only compatible unfinished runs. Optional JSON Schema validation joins
the bounded retry path. See [PR #19](https://github.com/BonzTM/agent-context-manager/pull/19).

The `claude-agent` and `codex-agent` map processors add bounded tool-using
sessions with host read-only controls, observed turn/tool limits, deadlines,
and process-group termination. See [PR #20](https://github.com/BonzTM/agent-context-manager/pull/20).

## Upgrade

```sh
go install github.com/bonztm/agent-context-manager/cmd/acm@v1.2.0

# Open the database, apply migration 0006, and verify integrity.
acm doctor

# Install the new OpenCode transform and refreshed Codex hooks.
acm init opencode --global
acm init codex --global

# Preview any recoverable Codex capture gaps; apply only after review.
acm backfill
acm backfill --apply
```

`acm init` is not required to migrate a database or upgrade the binary. Run it
for the host integrations whose generated assets changed, then restart those
agents so they load the updated configuration. Existing retained data is not
retroactively redacted when a new `.acm-policy.toml` is added.

## Verification

Every feature PR and this release pass the repository's `make verify` gate:
module hygiene, formatting, lint, `go vet`, unit tests, race tests,
`govulncheck`, and a static pure-Go build. CI also cross-compiles Linux, macOS,
and Windows for amd64 and arm64 before merge; the release workflow packages the
same six targets with per-archive SHA-256 files.
