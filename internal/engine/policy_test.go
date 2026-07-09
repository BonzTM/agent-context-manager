package engine

import (
	"strings"
	"testing"

	"github.com/bonztm/agent-context-manager/internal/core"
)

func TestConfigRejectsTargetsOutsideSoftBudget(t *testing.T) {
	cfg := DefaultConfig()
	cfg.ModelContextTokens = 1_000

	err := cfg.Validate()
	if err == nil || !strings.Contains(err.Error(), "600-token soft budget") {
		t.Fatalf("Validate() error = %v, want exact soft-budget remediation", err)
	}
}

func TestConfigRejectsUnsafeBudgetAndChunking(t *testing.T) {
	cfg := DefaultConfig()
	cfg.ModelContextTokens = 0
	assertInvalidConfig(t, cfg, "model context tokens")
	cfg = DefaultConfig()
	cfg.SoftFraction = 0
	assertInvalidConfig(t, cfg, "soft fraction")
	cfg = DefaultConfig()
	cfg.HardFraction = cfg.SoftFraction
	assertInvalidConfig(t, cfg, "hard fraction")
	cfg = DefaultConfig()
	cfg.LeafTargetTokens = cfg.LeafChunkTokens
	assertInvalidConfig(t, cfg, "leaf target")
	cfg = DefaultConfig()
	cfg.CondensedTargetTokens = cfg.CondenseChunkTokens
	assertInvalidConfig(t, cfg, "condensed target")
}

func TestConfigRejectsUnsafeLoopBounds(t *testing.T) {
	cfg := DefaultConfig()
	cfg.FreshTailTokens = -1
	assertInvalidConfig(t, cfg, "cannot be negative")
	cfg = DefaultConfig()
	cfg.CondenseFanout = 1
	assertInvalidConfig(t, cfg, "condense fanout")
	cfg = DefaultConfig()
	cfg.MaxDepth = 0
	assertInvalidConfig(t, cfg, "max depth")
	cfg = DefaultConfig()
	cfg.MaxIterations = 0
	assertInvalidConfig(t, cfg, "max iterations")
}

func assertInvalidConfig(t *testing.T, cfg Config, want string) {
	t.Helper()
	err := cfg.Validate()
	if err == nil || !strings.Contains(err.Error(), want) {
		t.Fatalf("Validate() error = %v, want substring %q", err, want)
	}
}

func TestProtectedSetSkipsToolResults(t *testing.T) {
	window := []windowItem{
		{typ: core.ContextMessage, role: core.RoleUser, tokens: 100},
		{typ: core.ContextMessage, role: core.RoleTool, tokens: 1_000},
		{typ: core.ContextMessage, role: core.RoleAssistant, tokens: 100},
	}

	protected := protectedSet(window, 2, 150)
	if !protected[0] || !protected[2] || protected[1] {
		t.Fatalf("protected indices = %+v, want conversational messages only", protected)
	}
}

func TestCondenseRunBoundsFanInAndTokens(t *testing.T) {
	window := make([]windowItem, 5)
	for i := range window {
		window[i] = windowItem{typ: core.ContextSummary, depth: 0, tokens: 100}
	}

	start, end, ok := condenseRun(window, 3, 3, 300)
	if !ok || start != 0 || end != 3 {
		t.Fatalf("condenseRun() = %d,%d,%t, want 0,3,true", start, end, ok)
	}
	if _, _, ok = condenseRun(window, 3, 3, 299); ok {
		t.Fatal("condenseRun accepted a block above the token cap")
	}
}
