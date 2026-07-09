# Configuration

`acm` is configured through command-line flags and environment variables. Flags
take precedence over environment variables, which take precedence over built-in
defaults.

## Global flags

These are available on every command:

| Flag | Env | Default | Description |
|------|-----|---------|-------------|
| `--db` | `ACM_DB` | resolved (see below) | Path to the SQLite database |
| `--log-level` | `ACM_LOG_LEVEL` | `info` | `debug`, `info`, `warn`, or `error` |
| `--log-json` | `ACM_LOG_JSON` | `false` | Emit JSON logs instead of text |

## Database path resolution

When `--db` is not given, the database path is resolved in this order:

1. `$ACM_DB`
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
  .gitignore      Written on creation (`*`) so the directory self-excludes
  acm.db          SQLite database (created automatically)
  backups/        Snapshots written by `acm backup`
  files/          Offloaded large-file payloads
  init/<agent>/   Integration assets written by `acm init`
```

The `.acm/` directory is excluded from version control automatically: acm writes
a `*` `.gitignore` into it on first use.

## Compaction tuning

`acm compact` exposes the token budget and chunking knobs. Defaults target a
~200,000-token model window.

| Flag | Default | Description |
|------|---------|-------------|
| `--model-context-tokens` | `200000` | The host model's context window |
| `--soft-fraction` | `0.6` | Compact when the window exceeds this fraction of the model window |
| `--fresh-tail` | `8` | Most recent messages always kept raw |
| `--leaf-chunk-tokens` | `4000` | Maximum source tokens folded into one leaf summary |
| `--leaf-target-tokens` | `600` | Target size of a leaf summary |
| `--condensed-target-tokens` | `1000` | Target size of a condensed summary |
| `--hard-fraction` | `0.8` | Warn when a finished pass is still above this fraction |
| `--summarizer` | `deterministic` | `deterministic`, `claude`, or `codex` |

Compaction also runs opportunistically from the capture hooks on turn-ending
events (`Stop`, `agent-turn-complete`) with the deterministic summarizer and
default budget; pass `--no-compact` to `acm hook` to disable it. When you lower
`--model-context-tokens`, lower the two target sizes with it — summaries larger
than the remaining budget cannot bring the window under the threshold.

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
Every headless model call has a 120-second deadline. On Unix the entire process
group is terminated at the deadline; inherited output pipes have a final
one-second drain bound on every platform.

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
