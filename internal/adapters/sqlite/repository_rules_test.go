package sqlite

import (
	"context"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/bonztm/agent-context-manager/internal/core"
)

func TestSyncRulePointers_UpsertsAndMarksMissingAsStale(t *testing.T) {
	ctx := context.Background()
	repo, err := New(ctx, Config{Path: filepath.Join(t.TempDir(), "ctx.sqlite")})
	if err != nil {
		t.Fatalf("new repository: %v", err)
	}
	t.Cleanup(func() { _ = repo.Close() })

	projectID := "project.alpha"
	sourcePath := ".acm/acm-rules.yaml"
	firstPointers := []core.RulePointer{
		{
			PointerKey:  "project.alpha:.acm/acm-rules.yaml#rule.alpha",
			SourcePath:  sourcePath,
			RuleID:      "rule.alpha",
			Summary:     "alpha summary",
			Content:     "alpha content",
			Enforcement: "hard",
			Tags:        []string{"ops"},
		},
		{
			PointerKey:  "project.alpha:.acm/acm-rules.yaml#rule.beta",
			SourcePath:  sourcePath,
			RuleID:      "rule.beta",
			Summary:     "beta summary",
			Content:     "beta content",
			Enforcement: "soft",
			Tags:        []string{"policy"},
		},
	}

	firstResult, err := repo.SyncRulePointers(ctx, core.RulePointerSyncInput{
		ProjectID:  projectID,
		SourcePath: sourcePath,
		Pointers:   firstPointers,
	})
	if err != nil {
		t.Fatalf("first sync rule pointers: %v", err)
	}
	if firstResult.Upserted != 2 || firstResult.MarkedStale != 0 {
		t.Fatalf("unexpected first sync result: %+v", firstResult)
	}

	secondResult, err := repo.SyncRulePointers(ctx, core.RulePointerSyncInput{
		ProjectID:  projectID,
		SourcePath: sourcePath,
		Pointers: []core.RulePointer{
			{
				PointerKey:  "project.alpha:.acm/acm-rules.yaml#rule.alpha",
				SourcePath:  sourcePath,
				RuleID:      "rule.alpha",
				Summary:     "alpha summary updated",
				Content:     "",
				Enforcement: "hard",
				Tags:        []string{"ops"},
			},
		},
	})
	if err != nil {
		t.Fatalf("second sync rule pointers: %v", err)
	}
	if secondResult.Upserted != 1 {
		t.Fatalf("unexpected second upsert count: %d", secondResult.Upserted)
	}
	if secondResult.MarkedStale != 1 {
		t.Fatalf("unexpected second stale count: %d", secondResult.MarkedStale)
	}

	rows, err := repo.db.QueryContext(ctx, `
SELECT pointer_key, is_stale, label, description, tags_json
FROM acm_pointers
WHERE project_id = ?
	AND path = ?
ORDER BY pointer_key
`, projectID, sourcePath)
	if err != nil {
		t.Fatalf("query synced rule pointers: %v", err)
	}
	defer rows.Close()

	type persistedRule struct {
		PointerKey  string
		IsStale     int64
		Label       string
		Description string
		Tags        []string
	}
	persisted := make([]persistedRule, 0, 2)
	for rows.Next() {
		var (
			row      persistedRule
			tagsJSON string
		)
		if err := rows.Scan(&row.PointerKey, &row.IsStale, &row.Label, &row.Description, &tagsJSON); err != nil {
			t.Fatalf("scan synced rule pointer: %v", err)
		}
		tags, decodeErr := decodeStringList(tagsJSON)
		if decodeErr != nil {
			t.Fatalf("decode synced rule pointer tags: %v", decodeErr)
		}
		row.Tags = tags
		persisted = append(persisted, row)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate synced rule pointers: %v", err)
	}
	if len(persisted) != 2 {
		t.Fatalf("unexpected persisted pointer count: %d", len(persisted))
	}

	if persisted[0].PointerKey != "project.alpha:.acm/acm-rules.yaml#rule.alpha" {
		t.Fatalf("unexpected active pointer key: %q", persisted[0].PointerKey)
	}
	if persisted[0].IsStale != 0 {
		t.Fatalf("expected active rule pointer, got stale=%d", persisted[0].IsStale)
	}
	if persisted[0].Description != "alpha summary updated" {
		t.Fatalf("expected empty content fallback to summary, got %q", persisted[0].Description)
	}
	wantActiveTags := []string{"canonical-rule", "enforcement-hard", "ops", "rule"}
	if !reflect.DeepEqual(persisted[0].Tags, wantActiveTags) {
		t.Fatalf("unexpected active tags: got %v want %v", persisted[0].Tags, wantActiveTags)
	}

	if persisted[1].PointerKey != "project.alpha:.acm/acm-rules.yaml#rule.beta" {
		t.Fatalf("unexpected stale pointer key: %q", persisted[1].PointerKey)
	}
	if persisted[1].IsStale != 1 {
		t.Fatalf("expected stale rule pointer, got stale=%d", persisted[1].IsStale)
	}
}
