package agents

import (
	"fmt"
	"strings"

	"github.com/bonztm/agent-context-manager/internal/core"
)

// Asset is one generated integration file, written under .acm/init/<agent>/.
type Asset struct {
	RelPath string
	Content string
}

// InitPlan is the set of assets plus human instructions for wiring an agent.
type InitPlan struct {
	Assets       []Asset
	Instructions string
}

// drillDownDoc is the shared instruction block teaching the model how to recover
// compacted context through its own shell tool. acm init writes it for each
// agent so the model knows the drill-down commands exist.
const drillDownDoc = `## acm — lossless long-context

This project uses acm (agent-context-manager) for long-context management. Older
conversation is compacted into ` + "`<summary id=\"sum_...\">`" + ` pointers and surfaced via
` + "`<acm-recall>`" + ` blocks. To recover the exact verbatim original on demand, run in
your shell:

    acm expand <id>          # expand a summary to its source messages
    acm grep "<pattern>"     # full-text search the entire project history
    acm describe <id>        # show a message, summary, or offloaded file by id

Prefer these over guessing — the full history is always retrievable.
`

const claudeSettings = `{
  "permissions": {
    "allow": ["Bash(acm:*)"]
  },
  "hooks": {
    "UserPromptSubmit": [
      { "hooks": [ { "type": "command", "command": "acm hook --agent claude-code --event UserPromptSubmit" } ] }
    ],
    "PostToolUse": [
      { "matcher": "*", "hooks": [ { "type": "command", "command": "acm hook --agent claude-code --event PostToolUse" } ] }
    ]
  }
}
`

const codexHooks = `# Codex hooks for acm. Wire these into your Codex hooks configuration
# (~/.codex/config.toml [hooks] or hooks.json, per your Codex version). The
# capture commands are safe to run from any project; recall is emitted on
# user-prompt-submit. Note: Codex ignores 'notify' in project config, so set the
# agent-turn-complete capture globally if you want assistant-message capture.

user-prompt-submit = "acm hook --agent codex --event UserPromptSubmit"
post-tool-use      = "acm hook --agent codex --event PostToolUse"
# global notify (in ~/.codex/config.toml): notify = ["acm","hook","--agent","codex","--event","agent-turn-complete"]
`

const opencodeConfig = `{
  "$schema": "https://opencode.ai/config.json",
  "plugin": ["acm-opencode"],
  "_comment": "The acm OpenCode plugin (plugins/opencode-acm) captures events into acm and, on chat.messages.transform, can assemble the active window. It shells out to the 'acm' binary; ensure it is on PATH."
}
`

// BuildInit returns the integration assets and instructions for an agent.
func BuildInit(agent core.Agent) (InitPlan, error) {
	switch agent {
	case core.AgentClaude:
		return InitPlan{
			Assets: []Asset{
				{RelPath: "settings.snippet.json", Content: claudeSettings},
				{RelPath: "CLAUDE.acm.md", Content: drillDownDoc},
			},
			Instructions: instructions(core.AgentClaude,
				"Merge settings.snippet.json into .claude/settings.json (hooks + the Bash(acm:*) permission),",
				"and append CLAUDE.acm.md to your project's CLAUDE.md."),
		}, nil
	case core.AgentCodex:
		return InitPlan{
			Assets: []Asset{
				{RelPath: "hooks.snippet.toml", Content: codexHooks},
				{RelPath: "AGENTS.acm.md", Content: drillDownDoc},
			},
			Instructions: instructions(core.AgentCodex,
				"Wire hooks.snippet.toml into your Codex hooks config (project .codex/config.toml requires trust;",
				"notify must be set globally), and append AGENTS.acm.md to your project's AGENTS.md."),
		}, nil
	case core.AgentOpenCode:
		return InitPlan{
			Assets: []Asset{
				{RelPath: "opencode.snippet.json", Content: opencodeConfig},
				{RelPath: "AGENTS.acm.md", Content: drillDownDoc},
			},
			Instructions: instructions(core.AgentOpenCode,
				"Merge opencode.snippet.json into your opencode.json (the acm-opencode plugin in plugins/opencode-acm),",
				"and append AGENTS.acm.md to your project's AGENTS.md."),
		}, nil
	default:
		return InitPlan{}, fmt.Errorf("agents: unknown agent %q", agent)
	}
}

func instructions(agent core.Agent, lines ...string) string {
	parts := make([]string, 0, len(lines)+2)
	parts = append(parts, fmt.Sprintf("Generated acm integration assets for %s. Next steps:", agent))
	for _, l := range lines {
		parts = append(parts, "  - "+l)
	}
	parts = append(parts, "  - The drill-down commands (acm expand/grep/describe) run through the agent's shell tool.")
	return strings.Join(parts, "\n")
}
