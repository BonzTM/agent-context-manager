package agents

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

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
	if !strings.Contains(block, "acm describe <msg-id>") || strings.Contains(block, "acm expand <id>") {
		t.Fatalf("block has incorrect drill-down command: %q", block)
	}
}

func TestRecallTermsSuppressLowSignalContinuation(t *testing.T) {
	if terms := RecallTerms("ok, we're back with it disabled. let's pick back up"); terms != nil {
		t.Fatalf("terms = %v, want low-signal prompt suppressed", terms)
	}
	want := []string{"acm", "window", "tokens", "findings"}
	got := RecallTerms("Explain the ACM window tokens and fix the findings")
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("terms = %v, want %v", got, want)
	}
}

func TestRankRecallPrefersDecisionOverOversizedToolPayload(t *testing.T) {
	now := time.Date(2026, 7, 9, 12, 0, 0, 0, time.UTC)
	hits := []core.SearchHit{
		{Message: core.Message{ID: "msg_tool", ConversationID: "other", Role: core.RoleTool, Content: "acm window tokens findings", TokenCount: 10_000, CreatedAt: now}},
		{Message: core.Message{ID: "msg_decision", ConversationID: "current", Role: core.RoleAssistant, Content: "The ACM window budget is 120000 tokens", TokenCount: 20, CreatedAt: now.Add(-time.Hour)}},
		{Message: core.Message{ID: "msg_old", ConversationID: "other", Role: core.RoleUser, Content: "ACM window token notes", TokenCount: 10, CreatedAt: now.Add(-60 * 24 * time.Hour)}},
	}

	got := RankRecall(hits, []string{"acm", "window", "tokens", "findings"}, "current", 2)
	if len(got) != 2 || got[0].Message.ID != "msg_decision" {
		t.Fatalf("ranked ids = %v, want decision first", recallIDs(got))
	}
}

func recallIDs(hits []core.SearchHit) []string {
	ids := make([]string, 0, len(hits))
	for _, hit := range hits {
		ids = append(ids, hit.Message.ID)
	}
	return ids
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
