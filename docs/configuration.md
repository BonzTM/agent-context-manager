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
| `--fresh-tail-tokens` | `4000` | Minimum recent conversational tokens kept raw |
| `--leaf-chunk-tokens` | `4000` | Maximum source tokens folded into one leaf summary |
| `--leaf-target-tokens` | `600` | Target size of a leaf summary |
| `--condensed-target-tokens` | `1000` | Target size of a condensed summary |
| `--condense-fanout` | `4` | Same-depth summaries folded into one condensed node |
| `--condense-chunk-tokens` | `8000` | Maximum source tokens folded into one condensed node |
| `--max-depth` | `3` | Maximum condensed-summary depth |
| `--truncate-tokens` | `512` | Deterministic fallback summary size |
| `--large-file-threshold` | `25000` | Offload compacted messages above this size; `0` disables |
| `--max-iterations` | `200` | Maximum compaction operations per conversation |
| `--hard-fraction` | `0.8` | Warn when a finished pass is still above this fraction |
| `--summarizer` | `deterministic` | `deterministic`, `claude`, or `codex` |

Compaction also runs opportunistically from the capture hooks on turn-ending
events (`Stop`, `agent-turn-complete`) with the deterministic summarizer and
default budget; pass `--no-compact` to `acm hook` to disable it. Invalid flag
combinations fail before the summary DAG or active window is modified. Summary
targets must fit below the soft budget and their corresponding chunk limits;
the error names the flags to lower or the model window to raise.

For meaningful compression, the leaf chunk size should comfortably exceed the
summary target, so that many tokens of input fold into a smaller summary.
The fresh tail protects recent system, user, and assistant messages by both
count and tokens. Tool results remain eligible for early compaction and
large-file offload so a tool-heavy turn cannot consume the entire raw window.

## Window diagnostics

`acm window <conversation>` renders ACM's persisted synthetic view and counts
the exact text shown, including summary wrappers. Stored summary estimates are
retained alongside rendered costs so wrapper overhead remains visible.

Use `--breakdown` for raw/summary and role/depth token subtotals, represented
message counts, sequence gaps or overlaps, offload-reference counts, and the
token estimator name. Combine it with `--json` for a structured report;
`--json` without `--breakdown` retains the existing item-array shape with
additive per-item diagnostic fields.

## Privacy policy

ACM loads `.acm-policy.toml` from the project root before opening the store.
Secret redaction defaults on even when the file is absent; the explicit
`redact = false` setting is required to disable it. Session, tool, structured
path, and content-class exclusions prevent matching messages from creating
rows. See [Security and privacy](security-and-privacy.md) for the full schema,
detectors, bounds, threat model, and false-positive handling.

Use `ignore_sessions` for sessions that receive neither recall nor writes, and
`stateless_sessions` for recall-enabled sessions that create no rows. See
[Session lifecycle](session-lifecycle.md) for retention and carry-over.

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
| `--max-input-bytes` | `1073741824` | Maximum total input bytes |
| `--max-item-bytes` | `1048576` | Maximum bytes in one input item |
| `--max-output-bytes` | `1048576` | Maximum bytes in one processor output |
| `--max-items` | `100000` | Maximum input item count |
| `--max-calls` | `0` | Maximum worst-case processor calls; `0` disables the budget |
| `--run-timeout` | `0s` | Maximum duration for the complete run; `0` disables the deadline |
| `--state-dir` | `<output>.acm-map-state` | Durable per-item resume state |

The hard caps are 64 concurrent workers, 8 GiB of input, 8 MiB per input or
output item, 1,000,000 items, 2,000,000 lines, 10 attempts per item, and 24
hours per run. ACM validates the complete input and worst-case call count before
invoking a processor. It then streams at most `2 * concurrency + 1` items
through memory and persists each attempt under `--state-dir`. The directory
also holds an owner-only validated input snapshot so later processing cannot
observe a different file than the one that passed preflight.

Cancellation or a process failure leaves resume state in place and does not
publish a partial output. Re-run the identical command to process only
unfinished items and assemble one ordered result per input. Changing the input,
processor contract, attempts, or byte limits requires removing the old state or
choosing another `--state-dir`. A successful run removes its state directory.
