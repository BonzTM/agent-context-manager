package sqlite

import (
	"context"
	"encoding/json"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/joshd/agents-context/internal/core"
)

func TestSaveRunReceiptSummary_PersistsDefinitionOfDoneIssuesInRunSummaryJSON(t *testing.T) {
	ctx := context.Background()
	repo, err := New(ctx, Config{Path: filepath.Join(t.TempDir(), "ctx.sqlite")})
	if err != nil {
		t.Fatalf("new repository: %v", err)
	}
	t.Cleanup(func() { _ = repo.Close() })

	ids, err := repo.SaveRunReceiptSummary(ctx, core.RunReceiptSummary{
		ProjectID: "project.alpha",
		ReceiptID: "receipt.abc123",
		Status:    "accepted_with_warnings",
		DefinitionOfDoneIssues: []string{
			"missing verify:tests",
			" missing verify:diff-review ",
			"missing verify:tests",
		},
	})
	if err != nil {
		t.Fatalf("save run receipt summary: %v", err)
	}

	var summaryJSON string
	if err := repo.db.QueryRowContext(ctx, `SELECT summary_json FROM ctx_runs WHERE run_id = ?`, ids.RunID).Scan(&summaryJSON); err != nil {
		t.Fatalf("query persisted run summary json: %v", err)
	}

	var summary struct {
		Status                 string   `json:"status"`
		DefinitionOfDoneIssues []string `json:"definition_of_done_issues"`
	}
	if err := json.Unmarshal([]byte(summaryJSON), &summary); err != nil {
		t.Fatalf("unmarshal persisted run summary json: %v", err)
	}

	if summary.Status != "accepted_with_warnings" {
		t.Fatalf("unexpected persisted status: %q", summary.Status)
	}
	wantIssues := []string{"missing verify:diff-review", "missing verify:tests"}
	if !reflect.DeepEqual(summary.DefinitionOfDoneIssues, wantIssues) {
		t.Fatalf("unexpected persisted DoD issues: got %v want %v", summary.DefinitionOfDoneIssues, wantIssues)
	}
}
