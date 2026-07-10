package opencode

import (
	"strings"
	"testing"

	"github.com/bonztm/agent-context-manager/internal/core"
	"github.com/bonztm/agent-context-manager/internal/engine"
)

func TestBuildPlanRendersFreshMessagesAndSummaryRoots(t *testing.T) {
	items := []engine.RenderedItem{
		{Type: core.ContextSummary, RefID: "sum_old", Content: "old decision", EarliestSeq: 1, LatestSeq: 6},
		{Type: core.ContextMessage, RefID: "msg_recent", Content: "recent", EarliestSeq: 7, LatestSeq: 7},
	}
	plan, err := BuildPlan(items, map[string]string{"msg_recent": "oc_recent"}, "<acm-recall>hit</acm-recall>", 8)
	if err != nil {
		t.Fatalf("build plan: %v", err)
	}
	if plan.Version != 1 || len(plan.FreshMessageIDs) != 1 || plan.FreshMessageIDs[0] != "oc_recent" {
		t.Fatalf("plan identity = %+v", plan)
	}
	for _, content := range []string{plan.ArchivePlaceholder, plan.SummaryText, plan.ResumeNote} {
		if !strings.HasPrefix(content, SyntheticPrefix) || !strings.Contains(content, "sum_old") {
			t.Fatalf("synthetic content = %q", content)
		}
	}
	if plan.RecallText != "<acm-recall>hit</acm-recall>" {
		t.Fatalf("recall text = %q", plan.RecallText)
	}
}

func TestBuildPlanWithoutSummariesDoesNotArchive(t *testing.T) {
	plan, err := BuildPlan([]engine.RenderedItem{{Type: core.ContextMessage, RefID: "msg_1"}}, map[string]string{"msg_1": "oc_1"}, "", 8)
	if err != nil {
		t.Fatalf("build plan: %v", err)
	}
	if plan.ArchivePlaceholder != "" || plan.SummaryText != "" || plan.ResumeNote != "" {
		t.Fatalf("raw-only plan contains archive content: %+v", plan)
	}
}

func TestBuildPlanRejectsUnknownItemType(t *testing.T) {
	_, err := BuildPlan([]engine.RenderedItem{{Type: "unknown"}}, nil, "", 8)
	if err == nil {
		t.Fatal("expected unknown item error")
	}
}

func TestBuildPlanBoundsRecallText(t *testing.T) {
	plan, err := BuildPlan(nil, nil, strings.Repeat("r", maxRecallChars+1), 8)
	if err != nil {
		t.Fatalf("build plan: %v", err)
	}
	if len([]rune(plan.RecallText)) != maxRecallChars || !strings.HasSuffix(plan.RecallText, "...") {
		t.Fatalf("recall text was not truncated: %d runes", len([]rune(plan.RecallText)))
	}
}
