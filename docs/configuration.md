# Configuration

`acm` is configured through command-line flags and environment variables. Flags
take precedence over environment variables, which take precedence over built-in
defaults.

## Global flags

These are available on every command:

| Flag | Env | Default | Description |
|------|-----|---------|-------------|
| `--db` | `LCM_DB` | resolved (see below) | Path to the SQLite database |
| `--log-level` | `LOG_LEVEL` | `info` | `debug`, `info`, `warn`, or `error` |
| `--log-json` | `LOG_JSON` | `false` | Emit JSON logs instead of text |

## Database path resolution

When `--db` is not given, the database path is resolved in this order:

1. `$LCM_DB`
2. `$CLAUDE_PROJECT_DIR/.acm/acm.db` (Claude Code sets this for spawned tools)
3. The nearest ancestor directory containing a `.acm/` directory
4. `<current working directory>/.acm/acm.db`

This makes a project resolve to the same database regardless of which
subdirectory a command is invoked from. The working directory alone is never
trusted, because agents launch tools from varying directories.

## Project layout

All state lives under `<project>/.acm/`:

```
.acm/
  acm.db          SQLite database (created automatically)
  files/          Offloaded large-file payloads
  init/<agent>/   Integration assets written by `acm init`
```

The `.acm/` directory should be excluded from version control.

## Compaction tuning

`acm compact` exposes the token budget and chunking knobs. Defaults target a
~200,000-token model window.

| Flag | Default | Description |
|------|---------|-------------|
| `--model-context-tokens` | `200000` | The host model's context window |
| `--soft-fraction` | `0.6` | Compact when the window exceeds this fraction of the model window |
| `--fresh-tail` | `8` | Most recent messages always kept raw |
| `--leaf-chunk-tokens` | `4000` | Maximum source tokens folded into one leaf summary |
| `--summarizer` | `deterministic` | `deterministic`, `claude`, or `codex` |

For meaningful compression, the leaf chunk size should comfortably exceed the
summary target, so that many tokens of input fold into a smaller summary.

## Summarizers

- **`deterministic`** (default) — a structural, no-LLM summarizer. Fully offline
  and reproducible. It compresses by truncation and structuring, so it is most
  effective when the input is larger than the summary target.
- **`claude`** — reuses Claude Code's model via `claude -p` in headless mode.
- **`codex`** — reuses Codex's model via `codex exec` in headless mode.

The LLM summarizers reuse the host agent's existing authentication; `acm` stores
no credentials of its own. Any failure (binary missing, authentication, rate
limit) falls back to the deterministic summarizer, so compaction always succeeds.

> The exact headless invocation flags depend on the installed agent CLI version.
> The deterministic fallback guarantees correct behavior if they differ.

## `acm map` options

| Flag | Default | Description |
|------|---------|-------------|
| `--input` | _(required)_ | Input JSONL file, one item per line |
| `--output` | _(required)_ | Output JSONL file |
| `--processor` | `passthrough` | `passthrough`, `claude`, or `codex` |
| `--prompt` | _(empty)_ | Instruction prepended to each item (for `claude`/`codex`) |
| `--require` | _(none)_ | Comma-separated JSON fields each output must contain |
| `--concurrency` | `8` | Maximum concurrent item workers |
| `--max-retries` | `2` | Retries per item on error or failed validation |
