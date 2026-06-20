package agents

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/bonztm/agent-context-manager/internal/core"
)

// RecallBlock formats search hits into an injectable context block. It returns
// "" when there is nothing relevant to inject (so the caller can skip output).
func RecallBlock(hits []core.SearchHit) string {
	if len(hits) == 0 {
		return ""
	}
	lines := make([]string, 0, len(hits)+3)
	lines = append(lines,
		"<acm-recall>",
		"Relevant earlier context from this project's history. Drill down for the",
		"verbatim original with: acm expand <id>  (or search more: acm grep <pattern>).",
	)
	for _, h := range hits {
		lines = append(lines, fmt.Sprintf("- [%s seq=%d %s] %s",
			h.Message.ID, h.Message.Seq, h.Message.Role, oneLine(h.Snippet)))
	}
	lines = append(lines, "</acm-recall>")
	return strings.Join(lines, "\n")
}

// hookOutput mirrors the JSON shape Claude Code and Codex accept for injecting
// supplemental context from a hook (rendered as a system reminder / developer
// message respectively).
type hookOutput struct {
	HookSpecificOutput hookSpecificOutput `json:"hookSpecificOutput"`
}

type hookSpecificOutput struct {
	HookEventName     string `json:"hookEventName"`
	AdditionalContext string `json:"additionalContext"`
}

// AdditionalContextJSON renders the hook output that injects context for the
// given event. It is valid for both Claude Code and Codex.
func AdditionalContextJSON(event, context string) ([]byte, error) {
	out, err := json.Marshal(hookOutput{
		HookSpecificOutput: hookSpecificOutput{HookEventName: event, AdditionalContext: context},
	})
	if err != nil {
		return nil, fmt.Errorf("agents: marshal hook output: %w", err)
	}
	return out, nil
}

func oneLine(s string) string {
	return strings.Join(strings.Fields(s), " ")
}
