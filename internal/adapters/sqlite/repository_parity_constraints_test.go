package sqlite

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bonztm/agent-context-manager/internal/core"
)

func TestTextMatchRank_IgnoresNonSearchMetadata(t *testing.T) {
	score := textMatchRank([]string{"fetch-only-key"}, core.CandidatePointer{
		Key:         "code:fetch-only-key",
		Path:        "internal/fetch-only-key/service.go",
		Anchor:      "fetch-only-key",
		Kind:        "fetch-only-key",
		Label:       "Repository service",
		Description: "Persists receipt scope",
		Tags:        []string{"backend"},
	})
	if score != 0 {
		t.Fatalf("expected zero score from key/path-only token match, got %d", score)
	}
}

func TestPointerKind_TreatsUnderscoreTestExtensionAsTest(t *testing.T) {
	if got := pointerKind(core.CandidatePointer{Path: "pkg/widget_test.ts"}); got != "test" {
		t.Fatalf("unexpected pointer kind: got %q want %q", got, "test")
	}
}

func TestSQLiteMigrations_EnforcePointerLinkForeignKeys(t *testing.T) {
	ctx := context.Background()
	repo, err := New(ctx, Config{Path: filepath.Join(t.TempDir(), "ctx.sqlite")})
	if err != nil {
		t.Fatalf("new sqlite repository: %v", err)
	}
	defer func() { _ = repo.Close() }()

	if _, err := repo.db.ExecContext(ctx, `
INSERT INTO acm_pointers (project_id, pointer_key, path, anchor, kind, label, description, tags_json, is_rule, is_stale)
VALUES (?, ?, ?, '', 'code', ?, ?, '[]', 0, 0)
`, "project.alpha", "code:from", "internal/from.go", "From", "from pointer"); err != nil {
		t.Fatalf("insert source pointer: %v", err)
	}

	_, err = repo.db.ExecContext(ctx, `
INSERT INTO acm_pointer_links (project_id, from_key, to_key)
VALUES (?, ?, ?)
`, "project.alpha", "code:from", "code:missing")
	if err == nil {
		t.Fatal("expected foreign key error for orphan pointer link")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "foreign key") {
		t.Fatalf("unexpected foreign key error: %v", err)
	}
}

func TestSQLiteMigrations_RequireEvidencePointersOnMemoryCandidates(t *testing.T) {
	ctx := context.Background()
	repo, err := New(ctx, Config{Path: filepath.Join(t.TempDir(), "ctx.sqlite")})
	if err != nil {
		t.Fatalf("new sqlite repository: %v", err)
	}
	defer func() { _ = repo.Close() }()

	_, err = repo.db.ExecContext(ctx, `
INSERT INTO acm_memory_candidates (
	project_id,
	receipt_id,
	category,
	subject,
	content,
	confidence,
	tags_json,
	related_pointer_keys_json,
	evidence_pointer_keys_json,
	dedupe_key,
	status,
	hard_passed,
	soft_passed,
	validation_errors_json,
	validation_warnings_json,
	auto_promote,
	promotable
) VALUES (?, ?, ?, ?, ?, ?, '[]', '[]', ?, ?, 'pending', 1, 1, '[]', '[]', 0, 0)
`, "project.alpha", "receipt.abc123", "decision", "Use sqlite", "Local default backend", 3, "[]", "cand-1")
	if err == nil {
		t.Fatal("expected evidence pointer check failure")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "check") {
		t.Fatalf("unexpected check constraint error: %v", err)
	}
}
