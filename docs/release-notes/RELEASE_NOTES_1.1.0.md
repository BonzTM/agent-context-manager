# [1.1.0] Release Notes - 2026-07-09

Version 1.1.0 closes the reliability gaps found by exercising ACM against its
own multi-million-token development history. It makes capture genuinely
source-event-aware, repairs Codex assistant-turn wiring, reduces noisy recall,
locks down local data, and bounds every model subprocess.

There is no database migration and no command rename. Existing projects can
upgrade in place.

## Complete Codex turn capture

`acm init codex --global` now parses `~/.codex/config.toml` and guarantees that
`notify` is a top-level key. Earlier releases appended the key at the end of the
file; when the file ended inside a TOML table, Codex silently ignored it and ACM
captured user prompts and tools without final assistant messages.

The installer detects that legacy misplaced ACM block, relocates it atomically,
validates the updated TOML, and remains idempotent after repair. Existing
third-party top-level `notify` commands are still preserved.

## Equal text remains distinct

Prompt and tool hooks now use the host's `turn_id` and `tool_use_id`. When an
external ID is unavailable, the raw hook payload supplies the event identity.
This keeps replay idempotent while preserving legitimate repeated messages such
as two separate `yes` prompts or two tool calls with identical output.

Direct imports that provide neither an external ID nor a raw source payload
retain the existing content-based deduplication behavior.

## Quieter, more useful recall

Automatic recall no longer injects the first five results from an unfiltered OR
query. It now:

- extracts at most 12 salient prompt terms;
- suppresses low-information continuation prompts;
- over-fetches no more than 50 BM25 candidates;
- reranks by term coverage, current conversation, role, recency, and payload
  size; and
- penalizes oversized tool output that previously dominated results.

Recall blocks also give the correct recovery command: `msg_` results use
`acm describe <msg-id>`. `acm expand` remains the command for `sum_` nodes.

## Private local state

ACM now creates and enforces owner-only (`0600`) permissions for SQLite
databases, WAL/SHM sidecars, backups, and offloaded payloads. Opening an older
database automatically repairs a more permissive mode.

This protects against other local users under the normal filesystem threat
model. It is not encryption at rest; host administrators and compromised user
accounts remain outside that boundary.

## Bounded model subprocesses

Every Claude/Codex headless invocation now has a 120-second deadline. On Unix,
ACM terminates the entire subprocess group so a child cannot keep inherited
output pipes open after its parent exits. A final one-second `WaitDelay` bounds
pipe draining on every platform.

The same boundary covers LLM-backed compaction, synthesized expansion answers,
and every model-backed `acm map` attempt. Existing deterministic fallbacks and
finite retry/compaction limits remain in place.

## Honest window diagnostics

`acm window` now describes its output as ACM's persisted, synthetic context
view. Claude Code and Codex do not expose active-window replacement, so the
command is diagnostic on those hosts; only supplemental recall is injected into
their live prompts.

## Upgrade

```sh
go install github.com/bonztm/agent-context-manager/cmd/acm@v1.1.0

# Repair/verify global Codex wiring, then restart Codex.
acm init codex --global

# Open, migrate if needed, repair file permissions, and check integrity.
acm doctor
```

Assistant turns missed before this upgrade are not automatically reconstructed.
Future transcript reconciliation and backfill are tracked in
[issue #9](https://github.com/BonzTM/agent-context-manager/issues/9).

## Verification

The release passes the repository's complete `make verify` gate: module hygiene,
formatting, lint, `go vet`, unit tests, race tests, `govulncheck`, and a static
pure-Go build. The release workflow additionally cross-compiles Linux, macOS,
and Windows artifacts for amd64 and arm64.
