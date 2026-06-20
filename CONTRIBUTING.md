# Contributing

Thanks for your interest in improving `acm`.

## Requirements

- Go 1.26 or newer.
- A C compiler is only needed to run the test suite under the race detector
  (`make race`); the application itself builds with `CGO_ENABLED=0`.

## Workflow

1. Fork and branch from `main`.
2. Make your change with accompanying tests.
3. Run the full verification gate before opening a pull request:

   ```sh
   make verify
   ```

   `make verify` runs, in order: dependency tidy, format check, lint, `go vet`,
   tests, the race detector, a vulnerability scan, and the build. It is the same
   gate used in review, so a green run locally means a green run upstream.

## Project layout

```
cmd/acm/              Entry point (process lifecycle only).
internal/
  config/             Configuration loading and validation.
  store/              SQLite persistence and migrations.
  core/               Domain model, the Store contract, and the service.
  tokens/             Token-count estimation.
  summarize/          Summarizers (deterministic and agent-CLI backed).
  engine/             Compaction loop and active-window assembly.
  agents/             Per-agent capture, recall, and init assets.
  install/            Global agent installation (safe config merge); embeds the
                      OpenCode plugin (assets/opencode-acm.ts).
  llmmap/             Off-context batch processing.
  cli/                Command-line surface.
docs/                 Documentation.
```

## Conventions

The codebase follows a stdlib-first Go style:

- Thin `main`; all fallible work returns errors up to a single boundary.
- Constructor wiring with no global state; `context.Context` is the first
  argument and is never stored in a struct.
- Errors are wrapped with `%w` and matched with `errors.Is` / `errors.As`, never
  by string. Sentinel errors are defined in the consumer package.
- Interfaces are defined by their consumers and kept small.
- Enumerations are string types, not `iota`.
- Time is injected through a `Clock` seam so tests are deterministic.

Lint and formatting rules are enforced by `make lint` and `make fmt` (configured
in `.golangci.yml`). Please keep new code warning-free.

## Tests

- Unit and integration tests use the standard library `testing` package against a
  real SQLite database.
- Keep tests deterministic — use the injected clock rather than wall time, and
  fixed inputs rather than randomness.

## Architecture decisions

Significant design decisions are recorded as ADRs under [`docs/adr/`](docs/adr/).
Add a new numbered ADR when making a decision with lasting architectural impact.
