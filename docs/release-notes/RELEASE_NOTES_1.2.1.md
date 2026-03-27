# [1.2.1] Release Notes - 2026-03-27

## Release Summary

Release distribution and documentation cleanup. Cross-platform binary builds for Linux, macOS, and Windows across amd64 and arm64. A new GitHub Actions release workflow automates archive uploads to GitHub Releases on tag publish. The version string is now injected via ldflags at build time so `acm --version` reports the release tag. Stale `acm-mcp invoke` references from the pre-JSON-RPC era are cleaned up across CLI help text, docs, and skill-pack assets.

## Added

### Release Workflow

- `.github/workflows/release.yml` — new workflow triggered on `release: [published]`. Builds all three binaries (`acm`, `acm-mcp`, `acm-web`) for 6 platform targets, packages them as `.tar.gz` (Linux/macOS) or `.zip` (Windows), and uploads to the GitHub Release via `gh release upload --clobber`.

### Version Injection

- `internal/buildinfo/buildinfo.go` — new `version` linker variable. When set (via `-ldflags "-X ...buildinfo.version=v1.2.1"`), it takes precedence over the `commitShort` fallback and VCS metadata from `debug.ReadBuildInfo`.
- `internal/buildinfo/buildinfo_test.go` — `TestVersion_UsesInjectedVersionOverCommitShort` verifies the precedence chain: `version` > `commitShort` > VCS revision > `"dev"`.
- Release workflow injects both `buildinfo.version` (tag name) and `buildinfo.commitShort` (commit SHA) at build time.

### Windows Support

- Windows build targets (`windows/amd64`, `windows/arm64`) added to both CI build and release workflows.
- Binaries get `.exe` extension on Windows.
- Release archives use `.zip` for Windows instead of `.tar.gz`.

## Changed

### CI Build Workflow

- `.github/workflows/go-build.yml` — replaced single-platform build with a matrix strategy across 6 targets: `linux/{amd64,arm64}`, `darwin/{amd64,arm64}`, `windows/{amd64,arm64}`.
- `pull_request` trigger removed (push to `main` and `workflow_dispatch` remain).
- Artifact names now include platform: `acm-linux-amd64`, `acm-darwin-arm64`, etc.
- Strip and size-optimize flags (`-s -w`) added to ldflags.
- Existing buildinfo tests updated to reset both `version` and `commitShort` in cleanup, ensuring test isolation with the new precedence chain.

### Documentation Cleanup (stale `acm-mcp invoke` references)

- `internal/adapters/cli/app.go` — CLI `--help` text updated from `` `acm-mcp tools` prints the machine-readable MCP tool directory `` to `` `acm-mcp --help` describes the JSON-RPC 2.0 MCP server ``.
- `docs/cli-reference.md` — replaced stale `acm-mcp invoke` reference with JSON-RPC 2.0 `tools/call` guidance and link to MCP Reference.
- `skills/acm-broker/SKILL.md` — replaced 7-line per-tool `acm-mcp invoke` example block with 3-line JSON-RPC 2.0 protocol summary.
- `skills/acm-broker/references/templates.md` — all 7 MCP example invocations converted from `acm-mcp invoke --tool <name> --in <file>` to piped `tools/call` JSON-RPC commands.

## Admin/Operations

- Binary rebuild required — `buildinfo.Version()` now checks the `version` variable first.
- Database schema unchanged from 1.2.0.
- No data migration required.
- CI artifacts are now per-platform. Downstream automation that expected a single `acm-binaries` artifact should update to `acm-{goos}-{goarch}`.

## Deployment and Distribution

- Go install: `go install github.com/bonztm/agent-context-manager/cmd/acm@v1.2.1`
- Go install (MCP): `go install github.com/bonztm/agent-context-manager/cmd/acm-mcp@v1.2.1`
- Go install (Web): `go install github.com/bonztm/agent-context-manager/cmd/acm-web@v1.2.1`
- Source: `https://github.com/BonzTM/agent-context-manager`

```bash
go install github.com/bonztm/agent-context-manager/cmd/acm@v1.2.1
go install github.com/bonztm/agent-context-manager/cmd/acm-mcp@v1.2.1
go install github.com/bonztm/agent-context-manager/cmd/acm-web@v1.2.1
```

Prebuilt binaries are available from the GitHub Release page for:
- `linux/amd64`, `linux/arm64` (`.tar.gz`)
- `darwin/amd64`, `darwin/arm64` (`.tar.gz`)
- `windows/amd64`, `windows/arm64` (`.zip`)

## Breaking Changes

- CI artifact naming changed from `acm-binaries` to `acm-{goos}-{goarch}`. This only affects consumers of GitHub Actions artifacts, not CLI or MCP users.

## Known Issues

- None identified.

## Compatibility and Migration

- Requires Go 1.26+ for `go install` or building from source.
- Direct upgrade from 1.2.0 or earlier. No data migration required.
- CLI and MCP usage unchanged.
- Wire format (JSON payloads) unchanged.

## Full Changelog

- Compare changes: https://github.com/BonzTM/agent-context-manager/compare/1.2.0...1.2.1
- Full changelog: https://github.com/BonzTM/agent-context-manager/commits/1.2.1
