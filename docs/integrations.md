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

---

## Claude Code

`acm init claude-code` generates:

- `settings.snippet.json` — hooks for `UserPromptSubmit` (capture + recall) and
  `PostToolUse` (capture), plus a permission entry allowing the `acm` command so
  drill-down does not prompt.
- `CLAUDE.acm.md` — the drill-down instruction block.

**Setup**

1. Merge `settings.snippet.json` into `.claude/settings.json`.
2. Append `CLAUDE.acm.md` to your project's `CLAUDE.md`.

Claude Code exposes capture and supplemental-context injection but not direct
ownership of the active message array, so `acm` augments Claude Code's own context
handling with a lossless side-record and recall.

---

## Codex

`acm init codex` generates:

- `hooks.snippet.toml` — capture/recall hook commands.
- `AGENTS.acm.md` — the drill-down instruction block.

**Setup**

1. Wire the hook commands into your Codex hooks configuration. Project-level
   configuration requires the project to be trusted.
2. Append `AGENTS.acm.md` to your project's `AGENTS.md`.

> Codex ignores `notify` in project-level configuration. To capture the final
> assistant message of each turn, set the `agent-turn-complete` notify command
> globally in `~/.codex/config.toml`.

Codex's shell tool may be sandboxed or gated; the permission/trust setup ensures
`acm` drill-down commands run without repeated prompts.

---

## OpenCode

OpenCode is integrated through a plugin (in `plugins/opencode-acm`) rather than
shell hooks, because its plugin API can deterministically own the active context
window.

`acm init opencode` generates:

- `opencode.snippet.json` — the plugin reference for `opencode.json`.
- `AGENTS.acm.md` — the drill-down instruction block.

**Setup**

1. Install the plugin and merge `opencode.snippet.json` into your `opencode.json`.
2. Append `AGENTS.acm.md` to your project's `AGENTS.md`.
3. Ensure the `acm` binary is on `PATH`; the plugin shells out to it.

By default the plugin captures messages and advertises the drill-down commands.
Full active-window ownership (the plugin assembling exactly what the model sees)
is available as an opt-in once validated for your OpenCode version.

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
