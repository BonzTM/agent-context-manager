# Integrations

`acm` integrates with each agent through hooks (for capture and recall) and the
agent's own shell tool (for on-demand recovery). Run `acm init <agent>` in a
project to generate the integration assets, then follow the printed steps.

`acm init` never modifies your existing agent configuration. It writes snippets
to `<project>/.acm/init/<agent>/` for you to review and merge, so nothing is
overwritten.

Two integration surfaces are common to all agents:

- **Capture + recall** — a hook calls `acm hook` on each event. It stores the
  turn and, on a user prompt, injects relevant recalled context.
- **Drill-down** — the agent recovers compacted detail by running `acm expand`,
  `acm grep`, and `acm describe` through its shell tool. `acm init` writes an
  instruction block that documents these commands for the model.

## Global install (recommended)

Instead of per-project setup, install once into the agent's user-level
configuration to cover every project:

```sh
acm init <agent> --global --dry-run  # preview the exact changes
acm init <agent> --global            # install (act by default)
```

The install is **safe and idempotent**: it parses your existing config, adds only
acm's entries (preserving everything else), and re-running makes no further
changes. It never overwrites a config it cannot parse. Use `--dry-run` to preview
without writing. Because acm resolves the database from the working directory at
hook time, one global install captures into whichever project you are working in,
creating a `.acm/` directory on first use.

What it installs per agent:

| Agent | Config / files written | Instructions appended |
|-------|------------------------|-----------------------|
| Claude Code | `~/.claude/settings.json` — `UserPromptSubmit`+`PostToolUse`+`Stop` hooks + `Bash(acm:*)` permission | `~/.claude/CLAUDE.md` |
| Codex | `~/.codex/hooks.json` — `UserPromptSubmit`+`PostToolUse`+`Stop` hooks; `~/.codex/config.toml` — `notify` (assistant-turn capture) | `~/.codex/AGENTS.md` |
| OpenCode | `~/.config/opencode/plugins/acm/server.ts` — the plugin (OpenCode 2.x), auto-loaded (no `opencode.json` edit) | `~/.config/opencode/AGENTS.md` |

Notes:

- **Codex** hooks live in `~/.codex/hooks.json` (user-level, no project trust
  required). `notify` must be a top-level key in the global config, and the
  installer parses the TOML and places it there.
- **OpenCode** loads the plugin automatically from its plugins directory; the
  plugin is embedded in the `acm` binary and written on install — no npm step.
  Ensure `acm` is on `PATH` (the plugin shells out to it). If a 1.x plugin from
  an earlier acm release is still at `~/.config/opencode/plugin/acm.ts`, the
  installer prints a note with the `rm` command; it never deletes the file.

The per-project setup below remains available (omit `--global`) when you prefer
committable, repo-scoped configuration.

---

## Claude Code

`acm init claude-code` generates:

- `settings.snippet.json` — hooks for `UserPromptSubmit` (capture + recall),
  `PostToolUse` (capture), and `Stop` (assistant-turn capture from the session
  transcript + opportunistic compaction), plus a permission entry allowing the
  `acm` command so drill-down does not prompt.
- `CLAUDE.acm.md` — the drill-down instruction block.

**Setup**

1. Merge `settings.snippet.json` into `.claude/settings.json`.
2. Append `CLAUDE.acm.md` to your project's `CLAUDE.md`.

Claude Code exposes capture and supplemental-context injection but not direct
ownership of the active message array, so `acm` augments Claude Code's own context
handling with a lossless side-record and recall.

---

## Codex

Codex loads hooks from `hooks.json` (user-level `~/.codex/hooks.json`, or
`<repo>/.codex/hooks.json` for a trusted project). `acm init codex` generates:

- `hooks.snippet.json` — `UserPromptSubmit` (capture + recall), `PostToolUse`
  (capture), and `Stop` (idempotent rollout reconciliation) hooks.
- `AGENTS.acm.md` — the drill-down instruction block.

**Setup** (or just run `acm init --global codex`, which does all of this)

1. Merge `hooks.snippet.json` into `~/.codex/hooks.json`.
2. For assistant-turn capture, add `notify` as a top-level key in the global
   `~/.codex/config.toml` (Codex ignores `notify` in project config):
   `notify = ["acm", "hook", "--agent", "codex", "--event", "agent-turn-complete"]`.
   Codex appends its notification JSON as one positional argument; `acm hook`
   accepts that form in addition to the stdin payload used by lifecycle hooks.
3. Append `AGENTS.acm.md` to your project's `AGENTS.md`.

Codex's shell tool may be sandboxed or gated; on a trusted project the `acm`
drill-down commands run without repeated prompts.

`acm doctor` verifies the executable, top-level notify command, all three hooks,
owner-only database modes, latest Codex capture time, and conversations with
prompts but no assistant rows. To recover assistant turns missed by an older or
broken integration, preview `acm backfill`, then persist the reported missing
turns with `acm backfill --apply`. Rollout scans and repeated applies are bounded
and idempotent by Codex turn ID.

Codex compresses older rollouts in place (`<rollout>.jsonl.zst`) and moves
archived threads to `~/.codex/archived_sessions/`. Backfill reads the recorded
path first, then the compressed copy beside it, then the archived copy
(plain or compressed). A rollout with no copy on disk is reported as
`unavailable=N` for its conversation and skipped; the run continues.

---

## OpenCode

OpenCode is integrated through a plugin written for the OpenCode 2.x plugin API.
OpenCode auto-loads each directory under its `plugins/` directory as a local
plugin, using `server.ts` as the server entrypoint (Bun runs TypeScript
natively), so installation is just dropping one self-contained file — no
`opencode.json` edit, no npm. OpenCode 1.x is not supported.

The simplest path is `acm init --global opencode`, which writes the embedded
plugin to `~/.config/opencode/plugins/acm/server.ts` and appends the drill-down
instructions to `~/.config/opencode/AGENTS.md`. For a single project, copy that
file to `<repo>/.opencode/plugins/acm/server.ts` instead.

Ensure the `acm` binary is on `PATH`; the plugin shells out to it. OpenCode
creates one plugin instance per project directory, and each instance runs `acm`
in that directory:

- **Capture.** The plugin reads OpenCode's session event stream. It ingests a
  user prompt when its inbox item is delivered, an assistant step's text when
  the step ends, and each tool call when it succeeds or fails. Ingestion is
  serialized and idempotent. The stream is not replayed after a disconnect, so
  events emitted while the plugin reconnects are not captured.
- **Active window.** Before each model request, the session `context` hook
  requests a bounded, versioned plan from `acm opencode-context`. The binary
  archives messages outside the protected count-and-token tail, assembles active
  summary roots, and selects automatic recall. The plugin replaces archived text
  and tool-result payloads with the archive placeholder, drops archived
  reasoning, and appends the summary and `<acm-recall>` context to the current
  user message. It leaves messages that carry provider-native replay data
  unchanged.
- **Guidance.** The `context` hook adds a drill-down hint to the system prompt,
  and the `compaction` hook adds a bounded resume note.

Generated context is marked with an `acmContext` metadata field and a reserved
`[Archived by acm:` or `<acm-recall>` prefix. Both the plugin and the capture
policy reject those markers before persistence, preventing transformed context
from feeding back into the lossless store. Messages intentionally beginning
with either reserved marker are therefore excluded from capture.

---

## Optional: reuse the agent's model for summarization

By default `acm` summarizes deterministically and offline. To use higher-quality
LLM summaries that reuse the agent's own model and credentials:

```sh
acm compact --summarizer claude    # or: codex
```

This invokes the agent's headless mode. It falls back to the deterministic
summarizer if the agent CLI is unavailable. Review your agent's terms for headless
and subscription use before enabling.
