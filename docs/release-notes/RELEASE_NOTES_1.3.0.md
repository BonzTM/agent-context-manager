# [1.3.0] Release Notes - 2026-10-31

Version 1.3.0 restores the OpenCode integration on OpenCode 2.x, makes Codex
backfill work against the rollouts Codex actually keeps on disk, and builds with
Go 1.26.6 for a standard-library security fix.

This release has no database migration. It drops OpenCode 1.x support: the
embedded plugin uses the OpenCode 2.x plugin API only. Stay on 1.2.0 if you
still run OpenCode 1.x.

## OpenCode 2.x plugin

OpenCode 2.x rejects the 1.x plugin at load time ("Plugin must export a
default definition with an id and an effect or setup function"). Since an
OpenCode 2.x upgrade, capture, the active window, and automatic recall had
silently stopped. The plugin installed by `acm init --global opencode` now:

- exports a `{ id, setup }` plugin definition;
- installs to `~/.config/opencode/plugins/acm/server.ts`, OpenCode 2.x's local
  plugin layout;
- captures user prompts, assistant steps, and tool calls from the 2.x session
  event stream, handling only events for its own project directory;
- shapes the outgoing context in the session `context` hook with the unchanged
  `acm opencode-context` plan, and adds the resume note in the `compaction`
  hook; and
- runs `acm` asynchronously through a bounded, ordered queue with timeouts,
  reconnecting the event stream with capped backoff.

The installer reports, but never removes, a leftover 1.x plugin at
`~/.config/opencode/plugin/acm.ts`. OpenCode 2.x logs a load warning for it on
every start until you remove it.

## Codex backfill

`acm backfill` aborted the whole run on the first recorded rollout that was no
longer at its path. Codex compresses older rollouts in place
(`<rollout>.jsonl.zst`) and moves archived threads to
`~/.codex/archived_sessions/`, so most historical rollouts looked missing.
Backfill and the Codex Stop hook now read the recorded path, then the compressed
copy beside it, then the archived copy. A rollout with no copy on disk is
reported per conversation as `unavailable=N` and skipped; the run continues.
This adds `github.com/klauspost/compress` for zstd decoding.

## Security and dependencies

- Go 1.26.6 picks up the fix for GO-2026-6218 (quadratic complexity in
  `net/url` path resolution, reachable through JSON Schema compilation in
  `acm map`).
- `github.com/pressly/goose/v3` 3.29.0 and `modernc.org/sqlite` 1.60.1.

## Upgrade

```sh
go install github.com/bonztm/agent-context-manager/cmd/acm@v1.3.0

# Install the OpenCode 2.x plugin, then remove the 1.x plugin if the
# installer reports one.
acm init opencode --global
rm ~/.config/opencode/plugin/acm.ts

# Recover Codex assistant turns from compressed or archived rollouts.
acm backfill
acm backfill --apply
```

Restart OpenCode after `acm init` so it loads the new plugin. `acm init` for
Claude Code and Codex is optional unless your hooks predate 1.2.0.

## Verification

Every PR in this release passes the repository's `make verify` gate: module
hygiene, formatting, lint, `go vet`, unit tests, race tests, `govulncheck`, and
a static pure-Go build. The OpenCode plugin has a Bun replay test over recorded
OpenCode 2.x events, and was verified live on OpenCode 2.1.4 across two
projects on one server.
