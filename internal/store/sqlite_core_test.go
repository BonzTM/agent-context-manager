package store

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/bonztm/agent-context-manager/internal/core"
	"github.com/bonztm/agent-context-manager/internal/testutil"
)

func openTestStore(t *testing.T) *SQLite {
	t.Helper()
	ctx := context.Background()
	db, err := Open(ctx, filepath.Join(t.TempDir(), ".acm", "acm.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err = db.MigrateUp(ctx); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return NewSQLite(db, testutil.NewFakeClock(time.Date(2026, 6, 18, 12, 0, 0, 0, time.UTC)))
}

func mustConversation(t *testing.T, sq *SQLite) core.Conversation {
	t.Helper()
	conv, err := sq.EnsureConversation(context.Background(), core.ConversationInput{Agent: core.AgentClaude, SessionID: "s"})
	if err != nil {
		t.Fatalf("ensure conversation: %v", err)
	}
	return conv
}

func TestAppendMessageIsIdempotentAtStoreLevel(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	sq := openTestStore(t)
	conv := mustConversation(t, sq)

	in := core.MessageInput{ConversationID: conv.ID, Role: core.RoleUser, Content: "hello", TokenCount: 2, ExternalID: "m1"}
	first, created, err := sq.AppendMessage(ctx, in)
	if err != nil || !created {
		t.Fatalf("first append = (%v, created=%v)", err, created)
	}
	second, created, err := sq.AppendMessage(ctx, in)
	if err != nil {
		t.Fatalf("second append: %v", err)
	}
	if created {
		t.Fatal("second append created a duplicate")
	}
	if second.ID != first.ID || second.Seq != first.Seq {
		t.Fatalf("idempotent append changed identity: %+v vs %+v", second, first)
	}
}

func TestGetMessageMissingIsErrNotFound(t *testing.T) {
	t.Parallel()
	sq := openTestStore(t)
	_, err := sq.GetMessage(context.Background(), "msg_missing")
	if !errors.Is(err, core.ErrNotFound) {
		t.Fatalf("err = %v, want errors.Is ErrNotFound", err)
	}
	_, err = sq.GetSummary(context.Background(), "sum_missing")
	if !errors.Is(err, core.ErrNotFound) {
		t.Fatalf("summary err = %v, want errors.Is ErrNotFound", err)
	}
	_, err = sq.ConversationBySession(context.Background(), core.AgentCodex, "nope")
	if !errors.Is(err, core.ErrNotFound) {
		t.Fatalf("conversation err = %v, want errors.Is ErrNotFound", err)
	}
}

// TestConcurrentAppendAcrossHandles proves the multi-process capture path:
// several independent DB handles (one per simulated hook process) append to the
// same database file concurrently, and every message must land. Regression test
// for the deferred-transaction SQLITE_BUSY_SNAPSHOT message loss.
func TestConcurrentAppendAcrossHandles(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), ".acm", "acm.db")

	first, err := Open(ctx, path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { _ = first.Close() })
	if err = first.MigrateUp(ctx); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	clock := testutil.NewFakeClock(time.Date(2026, 6, 18, 12, 0, 0, 0, time.UTC))
	conv := mustConversation(t, NewSQLite(first, clock))

	const writers = 12
	var wg sync.WaitGroup
	errs := make([]error, writers)
	for i := range writers {
		wg.Go(func() {
			db, oErr := Open(ctx, path)
			if oErr != nil {
				errs[i] = oErr
				return
			}
			defer func() { _ = db.Close() }()
			sq := NewSQLite(db, clock)
			_, _, errs[i] = sq.AppendMessage(ctx, core.MessageInput{
				ConversationID: conv.ID,
				Role:           core.RoleUser,
				Content:        fmt.Sprintf("concurrent message %d", i),
				TokenCount:     3,
				ExternalID:     fmt.Sprintf("c%d", i),
			})
		})
	}
	wg.Wait()

	for i, err := range errs {
		if err != nil {
			t.Errorf("writer %d failed: %v", i, err)
		}
	}
	msgs, err := NewSQLite(first, clock).ListMessages(ctx, conv.ID, 0, 0)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(msgs) != writers {
		t.Fatalf("stored %d of %d concurrent messages (lossless capture violated)", len(msgs), writers)
	}
}

func TestSearchSummariesFindsCompactedContent(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	sq := openTestStore(t)
	conv := mustConversation(t, sq)

	msg, _, err := sq.AppendMessage(ctx, core.MessageInput{
		ConversationID: conv.ID, Role: core.RoleUser, Content: "we chose exponential backoff", TokenCount: 4, ExternalID: "m1",
	})
	if err != nil {
		t.Fatalf("append: %v", err)
	}
	if _, err = sq.CreateLeafSummary(ctx, core.LeafSummaryInput{
		ConversationID:         conv.ID,
		Content:                "decided on jittered retry policy for the client",
		TokenCount:             8,
		SourceMessageIDs:       []string{msg.ID},
		EarliestSeq:            msg.Seq,
		LatestSeq:              msg.Seq,
		DescendantMessageCount: 1,
	}); err != nil {
		t.Fatalf("create leaf: %v", err)
	}

	hits, err := sq.SearchSummaries(ctx, core.SearchQuery{Text: "jittered retry"})
	if err != nil {
		t.Fatalf("search summaries: %v", err)
	}
	if len(hits) != 1 {
		t.Fatalf("summary hits = %d, want 1", len(hits))
	}
	if hits[0].Summary.ConversationID != conv.ID {
		t.Fatalf("hit conversation = %q, want %q", hits[0].Summary.ConversationID, conv.ID)
	}

	sub, err := sq.SearchSummaries(ctx, core.SearchQuery{Text: "retry policy", Mode: core.SearchSubstr})
	if err != nil {
		t.Fatalf("substr search summaries: %v", err)
	}
	if len(sub) != 1 {
		t.Fatalf("substr summary hits = %d, want 1", len(sub))
	}
}

func TestCreateLargeFileKeepsOriginalCreatedAt(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), ".acm", "acm.db")
	db, err := Open(ctx, path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err = db.MigrateUp(ctx); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	clock := testutil.NewFakeClock(time.Date(2026, 6, 18, 12, 0, 0, 0, time.UTC))
	sq := NewSQLite(db, clock)
	conv := mustConversation(t, sq)

	lf := core.LargeFile{ConversationID: conv.ID, MessageID: "msg_x", StorageURI: "/tmp/x", ByteSize: 1}
	first, err := sq.CreateLargeFile(ctx, lf)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	clock.Advance(time.Hour)
	second, err := sq.CreateLargeFile(ctx, lf)
	if err != nil {
		t.Fatalf("re-create: %v", err)
	}
	if !second.CreatedAt.Equal(first.CreatedAt) {
		t.Fatalf("created_at changed on idempotent upsert: %v vs %v", second.CreatedAt, first.CreatedAt)
	}
}

func TestOpenWritesSelfIgnoreIntoAcmDir(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	dir := filepath.Join(t.TempDir(), ".acm")
	db, err := Open(ctx, filepath.Join(dir, "acm.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	data, err := os.ReadFile(filepath.Join(dir, ".gitignore"))
	if err != nil {
		t.Fatalf("read .gitignore: %v", err)
	}
	if string(data) != "*\n" {
		t.Fatalf(".gitignore = %q, want %q", string(data), "*\n")
	}
}
