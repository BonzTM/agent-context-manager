package cli

import (
	"testing"

	"github.com/bonztm/agent-context-manager/internal/core"
	"github.com/bonztm/agent-context-manager/internal/engine"
)

func TestWindowBreakdownBalancesRenderedTokens(t *testing.T) {
	items := []engine.RenderedItem{
		{Type: core.ContextSummary, Tokens: 5, RenderedTokens: 7, Depth: 1, EarliestSeq: 1, LatestSeq: 2, RepresentedMessages: 2, IsSummary: true},
		{Type: core.ContextMessage, Role: core.RoleAssistant, Tokens: 3, RenderedTokens: 3, EarliestSeq: 3, LatestSeq: 3, RepresentedMessages: 1},
		{Type: core.ContextMessage, Role: core.RoleTool, Tokens: 4, RenderedTokens: 4, EarliestSeq: 4, LatestSeq: 4, RepresentedMessages: 1, HasOffloadReference: true},
	}

	report := buildWindowBreakdown(items)
	if report.RenderedTokens != 14 || report.StoredTokens != 12 {
		t.Fatalf("token totals = rendered %d stored %d", report.RenderedTokens, report.StoredTokens)
	}
	categoryTotal := report.SummaryDepthTokens[1] + report.RoleTokens[core.RoleAssistant] + report.RoleTokens[core.RoleTool]
	if categoryTotal != report.RenderedTokens {
		t.Fatalf("category total = %d, want %d", categoryTotal, report.RenderedTokens)
	}
	if !report.CoverageComplete || report.RepresentedMessages != 4 || report.OffloadReferences != 1 {
		t.Fatalf("unexpected diagnostics: %+v", report)
	}
}

func TestSequenceCoverageReportsGapsAndOverlaps(t *testing.T) {
	items := []engine.RenderedItem{
		{EarliestSeq: 1, LatestSeq: 2},
		{EarliestSeq: 4, LatestSeq: 6},
		{EarliestSeq: 6, LatestSeq: 7},
	}

	complete, gaps, overlaps := sequenceCoverage(items)
	if complete || len(gaps) != 1 || gaps[0] != (sequenceRange{Start: 3, End: 3}) {
		t.Fatalf("gaps = %+v, complete=%t", gaps, complete)
	}
	if len(overlaps) != 1 || overlaps[0] != (sequenceRange{Start: 6, End: 6}) {
		t.Fatalf("overlaps = %+v", overlaps)
	}
}
