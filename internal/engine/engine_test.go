package engine_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/bonztm/agent-context-manager/internal/core"
	"github.com/bonztm/agent-context-manager/internal/engine"
	"github.com/bonztm/agent-context-manager/internal/store"
	"github.com/bonztm/agent-context-manager/internal/summarize"
	"github.com/bonztm/agent-context-manager/internal/testutil"
	"github.com/bonztm/agent-context-manager/internal/tokens"
)

func testConfig() engine.Config {
	return engine.Config{
		ModelContextTokens:    1000,
		SoftFraction:          0.5, // soft = 500 tokens
		HardFraction:          0.8,
		FreshTailMessages:     2,
		LeafChunkTokens:       250,
		LeafTargetTokens:      40,
		CondensedTargetTokens: 60,
		CondenseFanout:        2,
		MaxDepth:              3,
		TruncateTokens:        64,
		LargeFileThreshold:    1_000_000, // effectively disabled
		MaxIterations:         100,
	}
}

func setup(t *testing.T, cfg engine.Config) (*engine.Compactor, *store.SQLite, core.Conversation) {
	t.Helper()
	ctx := context.Background()
	db, err := store.Open(ctx, filepath.Join(t.TempDir(), "acm.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err = db.MigrateUp(ctx); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	clock := testutil.NewFakeClock(time.Date(2026, 6, 18, 12, 0, 0, 0, time.UTC))
	sq := store.NewSQLite(db, clock)
	conv, err := sq.EnsureConversation(ctx, core.ConversationInput{Agent: core.AgentClaude, SessionID: "s"})
	if err != nil {
		t.Fatalf("ensure conversation: %v", err)
	}
	comp := engine.New(sq, summarize.Deterministic{}, tokens.Heuristic{}, clock, cfg, t.TempDir(), nil)
	return comp, sq, conv
}

func appendMessages(t *testing.T, sq *store.SQLite, convID string, n int) {
	t.Helper()
	ctx := context.Background()
	counter := tokens.Heuristic{}
	for i := range n {
		content := fmt.Sprintf("message %02d %s", i, strings.Repeat("token ", 60))
		if _, _, err := sq.AppendMessage(ctx, core.MessageInput{
			ConversationID: convID,
			Role:           core.RoleUser,
			Content:        content,
			TokenCount:     counter.Count(content),
			ExternalID:     fmt.Sprintf("m%d", i),
		}); err != nil {
			t.Fatalf("append %d: %v", i, err)
		}
	}
}

func TestCompactShrinksWindowAndIsLossless(t *testing.T) {
	ctx := context.Background()
	comp, sq, conv := setup(t, testConfig())
	appendMessages(t, sq, conv.ID, 12)

	res, err := comp.Compact(ctx, conv.ID)
	if err != nil {
		t.Fatalf("compact: %v", err)
	}
	soft := 500
	if !res.Compacted || res.Leaves == 0 {
		t.Fatalf("expected compaction with leaves, got %+v", res)
	}
	if res.AfterTokens >= res.BeforeTokens {
		t.Fatalf("tokens did not shrink: %+v", res)
	}
	if res.AfterTokens > soft {
		t.Fatalf("after tokens %d still above soft %d", res.AfterTokens, soft)
	}

	// Window: must contain at least one summary, and the tail must be raw messages.
	items, err := comp.Assemble(ctx, conv.ID)
	if err != nil {
		t.Fatalf("assemble: %v", err)
	}
	var summaries int
	for _, it := range items {
		if it.IsSummary {
			summaries++
		}
	}
	if summaries == 0 {
		t.Fatal("assembled window has no summary items")
	}
	if items[len(items)-1].IsSummary {
		t.Fatal("fresh tail should end in a raw message, not a summary")
	}

	// Lossless recovery: expanding the first summary yields the earliest messages
	// verbatim.
	ctxItems, err := sq.ListContextItems(ctx, conv.ID)
	if err != nil {
		t.Fatalf("list context items: %v", err)
	}
	var firstSummaryID string
	for _, it := range ctxItems {
		if it.Type == core.ContextSummary {
			firstSummaryID = it.RefID
			break
		}
	}
	if firstSummaryID == "" {
		t.Fatal("no summary context item found")
	}
	recovered, err := comp.ExpandToMessages(ctx, firstSummaryID)
	if err != nil {
		t.Fatalf("expand to messages: %v", err)
	}
	if len(recovered) == 0 {
		t.Fatal("expansion recovered no messages")
	}
	if !strings.HasPrefix(recovered[0].Content, "message 00") {
		t.Fatalf("first recovered message = %q, want prefix 'message 00'", truncate(recovered[0].Content, 20))
	}
	// Verbatim: the recovered content matches exactly what was stored.
	want := "message 00 " + strings.Repeat("token ", 60)
	if recovered[0].Content != want {
		t.Fatalf("recovered content not verbatim:\n got %q", recovered[0].Content)
	}
}

func TestCompactIsIdempotent(t *testing.T) {
	ctx := context.Background()
	comp, sq, conv := setup(t, testConfig())
	appendMessages(t, sq, conv.ID, 12)

	if _, err := comp.Compact(ctx, conv.ID); err != nil {
		t.Fatalf("first compact: %v", err)
	}
	second, err := comp.Compact(ctx, conv.ID)
	if err != nil {
		t.Fatalf("second compact: %v", err)
	}
	// Already under threshold: a second pass should do no new work.
	if second.Leaves != 0 || second.Condensed != 0 {
		t.Fatalf("second compact did work: %+v", second)
	}
}

func TestCompactBelowSoftIsNoop(t *testing.T) {
	ctx := context.Background()
	comp, sq, conv := setup(t, testConfig())
	appendMessages(t, sq, conv.ID, 2) // well under soft

	res, err := comp.Compact(ctx, conv.ID)
	if err != nil {
		t.Fatalf("compact: %v", err)
	}
	if res.Compacted {
		t.Fatalf("did not expect compaction, got %+v", res)
	}
}

func TestLargeFileOffload(t *testing.T) {
	ctx := context.Background()
	cfg := testConfig()
	cfg.LargeFileThreshold = 50 // tokens; force offload of the big message
	comp, sq, conv := setup(t, cfg)

	counter := tokens.Heuristic{}
	big := "HUGEFILE " + strings.Repeat("data ", 400) // well over 50 tokens
	bigMsg, _, err := sq.AppendMessage(ctx, core.MessageInput{
		ConversationID: conv.ID, Role: core.RoleUser, Content: big, TokenCount: counter.Count(big), ExternalID: "big",
	})
	if err != nil {
		t.Fatalf("append big: %v", err)
	}
	appendMessages(t, sq, conv.ID, 11)

	if _, err = comp.Compact(ctx, conv.ID); err != nil {
		t.Fatalf("compact: %v", err)
	}

	fileID := core.DeriveFileID(conv.ID, bigMsg.ID)
	lf, err := sq.GetLargeFile(ctx, fileID)
	if err != nil {
		t.Fatalf("get large file: %v", err)
	}
	if lf.StorageURI == "" {
		t.Fatal("large file has no storage uri")
	}
	if _, err = os.Stat(lf.StorageURI); err != nil {
		t.Fatalf("offloaded file not written: %v", err)
	}
	// The verbatim content is still recoverable from the message store (lossless).
	got, err := sq.GetMessage(ctx, bigMsg.ID)
	if err != nil {
		t.Fatalf("get message: %v", err)
	}
	if got.Content != big {
		t.Fatal("offloaded message content not preserved in store")
	}
}

func truncate(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n])
}
