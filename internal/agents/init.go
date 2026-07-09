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

// DrillDownDoc is the shared instruction block teaching the model how to recover
// compacted context through its own shell tool. acm init writes it for each
// agent so the model knows the drill-down commands exist.
const DrillDownDoc = `## acm — lossless long-context

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
    ],
    "Stop": [
      { "hooks": [ { "type": "command", "command": "acm hook --agent claude-code --event Stop" } ] }
    ]
  }
}
`

const codexHooksSnippet = `{
  "hooks": {
    "UserPromptSubmit": [
      { "hooks": [ { "type": "command", "command": "acm hook --agent codex --event UserPromptSubmit" } ] }
    ],
    "PostToolUse": [
      { "hooks": [ { "type": "command", "command": "acm hook --agent codex --event PostToolUse" } ] }
    ],
    "Stop": [
      { "hooks": [ { "type": "command", "command": "acm hook --agent codex --event Stop" } ] }
    ]
  }
}
`

// BuildInit returns the integration assets and instructions for an agent.
func BuildInit(agent core.Agent) (InitPlan, error) {
	switch agent {
	case core.AgentClaude:
		return InitPlan{
			Assets: []Asset{
				{RelPath: "settings.snippet.json", Content: claudeSettings},
				{RelPath: "CLAUDE.acm.md", Content: DrillDownDoc},
			},
			Instructions: instructions(core.AgentClaude,
				"Merge settings.snippet.json into .claude/settings.json (hooks + the Bash(acm:*) permission),",
				"and append CLAUDE.acm.md to your project's CLAUDE.md."),
		}, nil
	case core.AgentCodex:
		return InitPlan{
			Assets: []Asset{
				{RelPath: "hooks.snippet.json", Content: codexHooksSnippet},
				{RelPath: "AGENTS.acm.md", Content: DrillDownDoc},
			},
			Instructions: instructions(core.AgentCodex,
				"Merge hooks.snippet.json into ~/.codex/hooks.json (or <repo>/.codex/hooks.json for a trusted project),",
				"add notify for assistant-turn capture in global ~/.codex/config.toml, and append AGENTS.acm.md to your AGENTS.md.",
				"Tip: 'acm init --global codex' does all of this for every project."),
		}, nil
	case core.AgentOpenCode:
		return InitPlan{
			Assets: []Asset{
				{RelPath: "AGENTS.acm.md", Content: DrillDownDoc},
			},
			Instructions: instructions(core.AgentOpenCode,
				"Install the embedded OpenCode plugin with 'acm init --global opencode' (writes ~/.config/opencode/plugin/acm.ts),",
				"or for this project copy that plugin into <repo>/.opencode/plugin/. Append AGENTS.acm.md to your AGENTS.md."),
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
