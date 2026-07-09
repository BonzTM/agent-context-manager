# [1.0.1] Release Notes - 2026-07-09

A fast-follow patch for 1.0.0. It makes global installs safe for symlinked configs (a dotfiles-repo `CLAUDE.md` or `settings.json` no longer loses its link), stops init from ever duplicating a hand-pasted instruction block, tightens the verification gate, aligns release packaging with the sibling projects, and picks up the Go 1.26.5 standard-library security fixes. No schema, command, or contract changes — a drop-in upgrade.

## Your symlinks survive installs

Many setups symlink `~/.claude/CLAUDE.md`, `AGENTS.md`, or `settings.json` into a dotfiles repository. acm's crash-atomic config writes replaced files by renaming a temp file into place — which silently converted such a symlink into a regular file, orphaning it from the dotfiles repo. Writes now resolve the symlink chain first and atomically replace the **final target**:

- The link — and the dotfiles workflow behind it — survives every install, with content landing in the repo where you expect it.
- A *dangling* link (fresh dotfiles checkout, target not yet created) gets its target file created, so the link starts working.
- The target's existing file permissions are preserved; new files are created `0600`.
- Crash-atomicity is unchanged: a kill mid-write can never truncate or corrupt a live config.

## No duplicate instruction blocks

`acm init` manages its drill-down block with marker comments and was already idempotent for managed blocks — but a copy pasted in by hand (no markers) was invisible to that check, so init would append a second, managed copy. Init now recognizes the block by its heading, leaves the hand-maintained copy untouched, and prints a notice suggesting the markers if you'd like acm to manage updates.

## A stricter gate and consistent releases

- `make verify` now runs a read-only `tidy-check` (`go mod tidy -diff`): CI fails on committed `go.mod`/`go.sum` drift instead of silently auto-correcting it in the build workspace. `make tidy` remains the local fixer. Credit where due: this flaw was caught by agent-workflow-manager's cross-LLM review gate.
- Release assets now match the sibling projects: per-architecture archives (`acm-<version>-<os>-<arch>.tar.gz`, `.zip` on Windows) with per-archive `.sha256` checksums across linux/darwin/windows on amd64/arm64, and a CI cross-compile check so platform breaks surface at PR time.
- Versions display without a `v` everywhere (`acm version`, release titles, stamped binaries); the `v` prefix lives only on git tags, where the Go toolchain requires it — the release workflow keeps both tag forms on every release commit so `go install …@1.0.1` and `@v1.0.1` both resolve.

## Security

- Go 1.26.5: the standard library fixes GO-2026-5856 (`crypto/tls`) and GO-2026-4970 (`os`), both flagged by the CI vulnerability gate on older toolchains.

## Before you upgrade

- Nothing required — drop-in from 1.0.0. If a previous install already orphaned a symlinked config, relink it once (`ln -sf <dotfiles-path> <config-path>`); from this release forward, installs preserve it.
