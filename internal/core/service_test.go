package core_test

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/bonztm/agent-context-manager/internal/core"
	"github.com/bonztm/agent-context-manager/internal/store"
	"github.com/bonztm/agent-context-manager/internal/testutil"
	"github.com/bonztm/agent-context-manager/internal/tokens"
)

func newTestService(t *testing.T) *core.Service {
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
	return core.NewService(store.NewSQLite(db, clock), clock, tokens.Heuristic{}, nil)
}

func TestIngestIsIdempotent(t *testing.T) {
	ctx := context.Background()
	svc := newTestService(t)

	req := core.IngestRequest{
		Agent:     core.AgentClaude,
		SessionID: "session-1",
		Messages: []core.IngestMessage{
			{Role: core.RoleUser, Content: "refactor the auth module", ExternalID: "m1"},
			{Role: core.RoleAssistant, Content: "done, here is the diff", ExternalID: "m2"},
		},
	}

	first, err := svc.Ingest(ctx, req)
	if err != nil {
		t.Fatalf("first ingest: %v", err)
	}
	if first.Appended != 2 || first.Deduped != 0 {
		t.Fatalf("first ingest = %+v, want appended 2 deduped 0", first)
	}
	if first.Tokens <= 0 {
		t.Fatalf("expected positive token count, got %d", first.Tokens)
	}

	// Re-ingesting the same messages (e.g. a transcript re-read) must add nothing.
	second, err := svc.Ingest(ctx, req)
	if err != nil {
		t.Fatalf("second ingest: %v", err)
	}
	if second.Appended != 0 || second.Deduped != 2 {
		t.Fatalf("second ingest = %+v, want appended 0 deduped 2", second)
	}
	if second.ConversationID != first.ConversationID {
		t.Fatalf("conversation id changed: %q vs %q", second.ConversationID, first.ConversationID)
	}

	st, err := svc.Stats(ctx)
	if err != nil {
		t.Fatalf("stats: %v", err)
	}
	if st.Conversations != 1 || st.Messages != 2 {
		t.Fatalf("stats = %+v, want 1 conversation 2 messages", st)
	}
}

func TestIngestRejectsInvalidAgent(t *testing.T) {
	ctx := context.Background()
	svc := newTestService(t)
	if _, err := svc.Ingest(ctx, core.IngestRequest{Agent: "bogus", SessionID: "s"}); err == nil {
		t.Fatal("expected error for invalid agent")
	}
}

func TestSearchAndDescribe(t *testing.T) {
	ctx := context.Background()
	svc := newTestService(t)

	res, err := svc.Ingest(ctx, core.IngestRequest{
		Agent:     core.AgentOpenCode,
		SessionID: "s2",
		Messages: []core.IngestMessage{
			{Role: core.RoleUser, Content: "please optimize the database query", ExternalID: "a"},
			{Role: core.RoleAssistant, Content: "I added an index on tenant_id", ExternalID: "b"},
		},
	})
	if err != nil {
		t.Fatalf("ingest: %v", err)
	}

	hits, err := svc.Search(ctx, core.SearchQuery{Text: "database"})
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(hits) != 1 {
		t.Fatalf("fts hits = %d, want 1", len(hits))
	}
	if hits[0].Message.ConversationID != res.ConversationID {
		t.Fatalf("hit conversation mismatch")
	}

	// Substring search finds the index mention.
	sub, err := svc.Search(ctx, core.SearchQuery{Text: "tenant_id", Mode: core.SearchSubstr})
	if err != nil {
		t.Fatalf("substr search: %v", err)
	}
	if len(sub) != 1 {
		t.Fatalf("substr hits = %d, want 1", len(sub))
	}

	got, err := svc.DescribeMessage(ctx, sub[0].Message.ID)
	if err != nil {
		t.Fatalf("describe: %v", err)
	}
	if got.Content != "I added an index on tenant_id" {
		t.Fatalf("describe content = %q", got.Content)
	}
}

func TestDescribeMissingReturnsNotFound(t *testing.T) {
	ctx := context.Background()
	svc := newTestService(t)
	_, err := svc.DescribeMessage(ctx, "msg_does_not_exist")
	if !errors.Is(err, core.ErrNotFound) {
		t.Fatalf("err = %v, want errors.Is core.ErrNotFound (the sentinel must survive wrapping)", err)
	}
}
