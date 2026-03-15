# Contributing to acm

Thank you for your interest in contributing to acm (agent-context-manager).

## Getting Started

1. Fork the repository and clone your fork.
2. Install Go 1.26+.
3. Run `go test ./...` to verify everything builds and passes.
4. Create a branch for your change.

## Development Workflow

```bash
# Build all binaries
go build -o dist/acm ./cmd/acm
go build -o dist/acm-mcp ./cmd/acm-mcp
go build -o dist/acm-web ./cmd/acm-web

# Run tests
go test ./...

# Run a specific package's tests
go test ./internal/service/backend/...
```

## Pull Requests

- Keep PRs focused on a single change.
- Include tests for new behavior under `cmd/**` or `internal/**`.
- Ensure `go test ./...` passes before submitting.
- Follow the existing code style — no linter configuration is imposed, but consistency with surrounding code is expected.

## What to Contribute

- Bug fixes with a failing test that demonstrates the fix.
- Documentation improvements.
- New init templates or agent integration assets.
- Storage adapter improvements (SQLite and Postgres must stay in parity).

## Architecture Notes

- `internal/contracts/v1` and `spec/v1` must move in lockstep — any payload, validation, or command change must update both plus their tests.
- CLI (`cmd/acm`) and MCP (`cmd/acm-mcp`) surfaces must stay in parity.
- SQLite and Postgres adapters must maintain behavioral equivalence.

See [docs/maintainer-reference.md](docs/maintainer-reference.md) and [docs/maintainer-map.md](docs/maintainer-map.md) for detailed architecture and change-routing guidance.

## License

By contributing, you agree that your contributions will be licensed under the [Apache License 2.0](LICENSE).
