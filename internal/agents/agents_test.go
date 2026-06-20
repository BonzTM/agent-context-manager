package agents

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/bonztm/agent-context-manager/internal/core"
)

func TestRecallBlock(t *testing.T) {
	if RecallBlock(nil) != "" {
		t.Fatal("empty hits should yield empty block")
	}
	hits := []core.SearchHit{
		{Message: core.Message{ID: "msg_abc", Seq: 12, Role: core.RoleAssistant}, Snippet: "use exponential backoff"},
	}
	block := RecallBlock(hits)
	if !strings.Contains(block, "<acm-recall>") || !strings.Contains(block, "</acm-recall>") {
		t.Fatalf("block missing wrapper: %q", block)
	}
	if !strings.Contains(block, "msg_abc") || !strings.Contains(block, "backoff") {
		t.Fatalf("block missing hit content: %q", block)
	}
}

func TestAdditionalContextJSON(t *testing.T) {
	raw, err := AdditionalContextJSON("UserPromptSubmit", "hello")
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var out struct {
		HookSpecificOutput struct {
			HookEventName     string `json:"hookEventName"`
			AdditionalContext string `json:"additionalContext"`
		} `json:"hookSpecificOutput"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if out.HookSpecificOutput.HookEventName != "UserPromptSubmit" || out.HookSpecificOutput.AdditionalContext != "hello" {
		t.Fatalf("unexpected output: %+v", out)
	}
}

func TestBuildInit(t *testing.T) {
	for _, agent := range []core.Agent{core.AgentClaude, core.AgentCodex, core.AgentOpenCode} {
		plan, err := BuildInit(agent)
		if err != nil {
			t.Fatalf("BuildInit(%s): %v", agent, err)
		}
		if len(plan.Assets) == 0 {
			t.Fatalf("BuildInit(%s): no assets", agent)
		}
		if plan.Instructions == "" {
			t.Fatalf("BuildInit(%s): no instructions", agent)
		}
		for _, as := range plan.Assets {
			if as.RelPath == "" || as.Content == "" {
				t.Fatalf("BuildInit(%s): empty asset %+v", agent, as)
			}
		}
	}
	if _, err := BuildInit("bogus"); err == nil {
		t.Fatal("expected error for unknown agent")
	}
}
