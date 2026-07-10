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
		FreshTailTokens:       80,
		LeafChunkTokens:       250,
		LeafTargetTokens:      40,
		CondensedTargetTokens: 60,
		CondenseFanout:        2,
		CondenseChunkTokens:   250,
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
		if it.RenderedTokens != (tokens.Heuristic{}).Count(it.Content) {
			t.Fatalf("rendered tokens = %d, want exact content count", it.RenderedTokens)
		}
		if it.EarliestSeq <= 0 || it.LatestSeq < it.EarliestSeq || it.RepresentedMessages <= 0 {
			t.Fatalf("invalid coverage metadata: %+v", it)
		}
		if it.IsSummary {
			summaries++
			if it.RenderedTokens <= it.Tokens {
				t.Fatalf("summary wrapper cost missing: rendered=%d stored=%d", it.RenderedTokens, it.Tokens)
			}
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

func TestArchiveToFreshTailCompactsBelowSoftThreshold(t *testing.T) {
	ctx := context.Background()
	compactor, sqliteStore, conversation := setup(t, testConfig())
	// Five messages remain below the 500-token soft threshold but exceed the
	// protected tail.
	appendMessages(t, sqliteStore, conversation.ID, 5)
	normal, err := compactor.Compact(ctx, conversation.ID)
	if err != nil {
		t.Fatalf("normal compact: %v", err)
	}
	if normal.Compacted {
		t.Fatalf("normal compact unexpectedly changed below-soft window: %+v", normal)
	}
	archived, err := compactor.ArchiveToFreshTail(ctx, conversation.ID)
	if err != nil {
		t.Fatalf("archive to fresh tail: %v", err)
	}
	if !archived.Compacted || archived.Leaves == 0 {
		t.Fatalf("archive result = %+v", archived)
	}
	items, err := compactor.Assemble(ctx, conversation.ID)
	if err != nil {
		t.Fatalf("assemble: %v", err)
	}
	if len(items) < 3 || !items[0].IsSummary || items[len(items)-1].IsSummary {
		t.Fatalf("archived window = %+v", items)
	}
	second, err := compactor.ArchiveToFreshTail(ctx, conversation.ID)
	if err != nil || second.Compacted {
		t.Fatalf("second archive = %+v err=%v", second, err)
	}
}

func TestToolHeavyCompactionProtectsConversationalTail(t *testing.T) {
	ctx := context.Background()
	cfg := testConfig()
	cfg.LargeFileThreshold = 100
	comp, sq, conv := setup(t, cfg)
	counter := tokens.Heuristic{}

	for i := range 4 {
		appendRoleMessage(t, sq, conv.ID, core.RoleUser, fmt.Sprintf("old-%d ", i)+strings.Repeat("history ", 80), fmt.Sprintf("old-%d", i))
	}
	recentUser := appendRoleMessage(t, sq, conv.ID, core.RoleUser, strings.Repeat("recent-user ", 20), "recent-user")
	tool := appendRoleMessage(t, sq, conv.ID, core.RoleTool, strings.Repeat("tool-output ", 600), "recent-tool")
	recentAssistant := appendRoleMessage(t, sq, conv.ID, core.RoleAssistant, strings.Repeat("recent-assistant ", 20), "recent-assistant")

	res, err := comp.Compact(ctx, conv.ID)
	if err != nil {
		t.Fatalf("compact: %v", err)
	}
	if res.AfterTokens > int(cfg.SoftFraction*float64(cfg.ModelContextTokens)) {
		t.Fatalf("after tokens = %d, soft budget exceeded", res.AfterTokens)
	}
	items, err := comp.Assemble(ctx, conv.ID)
	if err != nil {
		t.Fatalf("assemble: %v", err)
	}
	assertRawMessages(t, items, recentUser.ID, recentAssistant.ID)
	for _, item := range items {
		if item.RefID == tool.ID {
			t.Fatal("oversized tool result remained raw in the protected tail")
		}
	}
	if counter.Count(tool.Content) <= cfg.LargeFileThreshold {
		t.Fatal("tool fixture did not exceed offload threshold")
	}
}

func TestInvalidConfigFailsBeforePersistingWindow(t *testing.T) {
	ctx := context.Background()
	cfg := engine.DefaultConfig()
	cfg.ModelContextTokens = 1_000
	comp, sq, conv := setup(t, cfg)
	appendRoleMessage(t, sq, conv.ID, core.RoleUser, "unchanged", "invalid-config")

	if _, err := comp.Compact(ctx, conv.ID); err == nil {
		t.Fatal("Compact() accepted targets outside the soft budget")
	}
	items, err := sq.ListContextItems(ctx, conv.ID)
	if err != nil {
		t.Fatalf("list context items: %v", err)
	}
	if len(items) != 0 {
		t.Fatalf("invalid config persisted %d context items", len(items))
	}
}

func appendRoleMessage(t *testing.T, sq *store.SQLite, conversationID string, role core.Role, content, externalID string) core.Message {
	t.Helper()
	message, _, err := sq.AppendMessage(context.Background(), core.MessageInput{
		ConversationID: conversationID, Role: role, Content: content,
		TokenCount: tokens.Heuristic{}.Count(content), ExternalID: externalID,
	})
	if err != nil {
		t.Fatalf("append %s: %v", externalID, err)
	}
	return message
}

func assertRawMessages(t *testing.T, items []engine.RenderedItem, ids ...string) {
	t.Helper()
	found := make(map[string]bool, len(ids))
	for _, item := range items {
		found[item.RefID] = true
	}
	for _, id := range ids {
		if !found[id] {
			t.Fatalf("protected message %s was compacted", id)
		}
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

// TestWindowGrowsAfterCompaction is the regression test for the frozen-window
// bug: messages ingested after a compaction pass must appear in the assembled
// window and be compactible by later passes.
func TestWindowGrowsAfterCompaction(t *testing.T) {
	ctx := context.Background()
	comp, sq, conv := setup(t, testConfig())
	appendMessages(t, sq, conv.ID, 12)

	if _, err := comp.Compact(ctx, conv.ID); err != nil {
		t.Fatalf("first compact: %v", err)
	}

	// Ingest three more messages AFTER the window was persisted.
	counter := tokens.Heuristic{}
	for i := range 3 {
		content := fmt.Sprintf("late message %02d %s", i, strings.Repeat("newtok ", 60))
		if _, _, err := sq.AppendMessage(ctx, core.MessageInput{
			ConversationID: conv.ID,
			Role:           core.RoleUser,
			Content:        content,
			TokenCount:     counter.Count(content),
			ExternalID:     fmt.Sprintf("late%d", i),
		}); err != nil {
			t.Fatalf("append late %d: %v", i, err)
		}
	}

	items, err := comp.Assemble(ctx, conv.ID)
	if err != nil {
		t.Fatalf("assemble: %v", err)
	}
	late := 0
	for _, it := range items {
		if strings.HasPrefix(it.Content, "late message") {
			late++
		}
	}
	if late != 3 {
		t.Fatalf("assembled window shows %d late messages, want 3", late)
	}

	// A second compact must see (and be able to fold) the late messages too.
	res, err := comp.Compact(ctx, conv.ID)
	if err != nil {
		t.Fatalf("second compact: %v", err)
	}
	if res.BeforeTokens <= 0 || !res.Compacted {
		t.Fatalf("second compact ignored the late messages: %+v", res)
	}
}

// TestOffloadUsesTypeAwareExtractor proves structured payloads get a
// deterministic schema-level exploration summary with no summarizer call, while
// unstructured content still goes through the configured summarizer.
func TestOffloadUsesTypeAwareExtractor(t *testing.T) {
	ctx := context.Background()
	cfg := testConfig()
	cfg.LargeFileThreshold = 50
	comp, sq, conv := setup(t, cfg)

	counter := tokens.Heuristic{}
	rows := make([]string, 0, 300)
	for i := range 300 {
		rows = append(rows, fmt.Sprintf(`{"id":%d,"label":"item-%d"}`, i, i))
	}
	jsonBig := "[" + strings.Join(rows, ",") + "]"
	jsonMsg, _, err := sq.AppendMessage(ctx, core.MessageInput{
		ConversationID: conv.ID, Role: core.RoleTool, Content: jsonBig,
		TokenCount: counter.Count(jsonBig), ExternalID: "json-big",
	})
	if err != nil {
		t.Fatalf("append json: %v", err)
	}
	proseBig := "PROSE " + strings.Repeat("wordy narrative filler ", 200)
	proseMsg, _, err := sq.AppendMessage(ctx, core.MessageInput{
		ConversationID: conv.ID, Role: core.RoleUser, Content: proseBig,
		TokenCount: counter.Count(proseBig), ExternalID: "prose-big",
	})
	if err != nil {
		t.Fatalf("append prose: %v", err)
	}
	appendMessages(t, sq, conv.ID, 11)

	if _, err = comp.Compact(ctx, conv.ID); err != nil {
		t.Fatalf("compact: %v", err)
	}

	jf, err := sq.GetLargeFile(ctx, core.DeriveFileID(conv.ID, jsonMsg.ID))
	if err != nil {
		t.Fatalf("get json file: %v", err)
	}
	if jf.Extractor != "json" {
		t.Fatalf("json extractor = %q, want json", jf.Extractor)
	}
	if !strings.Contains(jf.ExplorationSummary, "array, 300 elements") {
		t.Fatalf("json exploration summary lacks shape: %s", jf.ExplorationSummary)
	}

	pf, err := sq.GetLargeFile(ctx, core.DeriveFileID(conv.ID, proseMsg.ID))
	if err != nil {
		t.Fatalf("get prose file: %v", err)
	}
	if pf.Extractor != "summarizer" {
		t.Fatalf("prose extractor = %q, want summarizer", pf.Extractor)
	}
}
