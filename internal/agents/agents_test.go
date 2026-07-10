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
	hits := []RecallHit{
		{Kind: RecallMessage, ID: "msg_abc", Seq: 12, Role: core.RoleAssistant, Snippet: "use exponential backoff"},
		{Kind: RecallSummary, ID: "sum_xyz", Depth: 1, EarliestSeq: 1, LatestSeq: 9, Snippet: "retry decision"},
	}
	block := RecallBlock(hits)
	if !strings.Contains(block, "<acm-recall>") || !strings.Contains(block, "</acm-recall>") {
		t.Fatalf("block missing wrapper: %q", block)
	}
	if !strings.Contains(block, "msg_abc") || !strings.Contains(block, "backoff") {
		t.Fatalf("block missing hit content: %q", block)
	}
	if !strings.Contains(block, "acm describe msg_abc") || !strings.Contains(block, "acm expand sum_xyz") {
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
	hits := []RecallHit{
		{Kind: RecallMessage, ID: "msg_tool", ConversationID: "other", Role: core.RoleTool, Content: "acm window tokens findings", TokenCount: 10_000, CreatedAt: now},
		{Kind: RecallMessage, ID: "msg_decision", ConversationID: "current", Role: core.RoleAssistant, Content: "The ACM window budget is 120000 tokens", TokenCount: 20, CreatedAt: now.Add(-time.Hour)},
		{Kind: RecallMessage, ID: "msg_old", ConversationID: "other", Role: core.RoleUser, Content: "ACM window token notes", TokenCount: 10, CreatedAt: now.Add(-60 * 24 * time.Hour)},
	}

	got := RankRecall(hits, []string{"acm", "window", "tokens", "findings"}, "current", now, 2)
	if len(got) != 2 || got[0].ID != "msg_decision" {
		t.Fatalf("ranked ids = %v, want decision first", recallIDs(got))
	}
}

func TestMessageRecallHitsExcludesFreshTailIDs(t *testing.T) {
	hits := []core.SearchHit{
		{Message: core.Message{ID: "msg_fresh", Content: "cobalt strategy"}},
		{Message: core.Message{ID: "msg_old", Content: "cobalt strategy"}},
	}
	converted := MessageRecallHits(hits, map[string]struct{}{"msg_fresh": {}})
	if len(converted) != 1 || converted[0].ID != "msg_old" {
		t.Fatalf("converted hits = %+v", converted)
	}
}

func TestRankRecallCapsSummaryResults(t *testing.T) {
	now := time.Date(2026, 7, 9, 12, 0, 0, 0, time.UTC)
	hits := []RecallHit{
		{Kind: RecallSummary, ID: "sum_1", Content: "cobalt migration plan", Active: true, CreatedAt: now},
		{Kind: RecallSummary, ID: "sum_2", Content: "cobalt migration plan", Active: true, CreatedAt: now},
		{Kind: RecallSummary, ID: "sum_3", Content: "cobalt migration plan", Active: true, CreatedAt: now},
		{Kind: RecallMessage, ID: "msg_1", Role: core.RoleAssistant, Content: "cobalt migration plan", CreatedAt: now},
		{Kind: RecallMessage, ID: "msg_2", Role: core.RoleUser, Content: "cobalt migration plan", CreatedAt: now},
		{Kind: RecallMessage, ID: "msg_3", Role: core.RoleTool, Content: "cobalt migration plan", CreatedAt: now},
	}
	ranked := RankRecall(hits, []string{"cobalt", "migration", "plan"}, "current", now, 5)
	summaries := 0
	for _, hit := range ranked {
		if hit.Kind == RecallSummary {
			summaries++
		}
	}
	if len(ranked) != 5 || summaries != MaxSummaryResults {
		t.Fatalf("ranked = %v, summaries = %d", recallIDs(ranked), summaries)
	}
}

func recallIDs(hits []RecallHit) []string {
	ids := make([]string, 0, len(hits))
	for _, hit := range hits {
		ids = append(ids, hit.ID)
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
