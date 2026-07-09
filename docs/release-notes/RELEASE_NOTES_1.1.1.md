# [1.1.1] Release Notes - 2026-07-09

Version 1.1.1 repairs Codex assistant-turn capture in the 1.1.0 integration.
There is no database migration, configuration change, or command rename.

## Codex notify payloads

Codex invokes the configured `notify` command with one JSON positional
argument. ACM 1.1.0 installed the command at the correct top-level TOML
location, but `acm hook` still read payloads only from stdin. Prompt and tool
hooks worked; completed assistant messages did not reach the store.

`acm hook` now accepts the single positional payload used by Codex while
preserving stdin input for `UserPromptSubmit`, `PostToolUse`, and Claude Code
hooks. A regression test exercises the documented `agent-turn-complete`
argument shape and verifies that both user and assistant messages are stored.

## Upgrade

```sh
go install github.com/bonztm/agent-context-manager/cmd/acm@v1.1.1
acm init codex --global
acm doctor
```

The existing top-level `notify` configuration installed by 1.1.0 is already
correct; rerunning `acm init` is an idempotent verification step.

Assistant turns missed before this patch are not reconstructed automatically.
Integration health checks and transcript backfill remain tracked in
[issue #9](https://github.com/BonzTM/agent-context-manager/issues/9).

## Verification

The release passes `make verify`, including formatting, lint, `go vet`, unit
tests, race tests, `govulncheck`, and the static pure-Go build. The release
workflow cross-compiles Linux, macOS, and Windows artifacts for amd64 and arm64.
