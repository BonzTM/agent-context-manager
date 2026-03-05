package postgres

import (
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/joshd/agent-context-manager/internal/core"
)

func TestBuildCandidatePointersQuery_DeterministicInputs(t *testing.T) {
	staleBefore := time.Date(2026, 3, 4, 9, 0, 0, 0, time.UTC)
	q := core.CandidatePointerQuery{
		ProjectID: "project-a",
		TaskText:  "  fix flaky memory retrieval ",
		Phase:     " plan ",
		Tags:      []string{"ops", "backend", "ops", " "},
		Limit:     0,
		StaleFilter: core.StaleFilter{
			AllowStale:  true,
			StaleBefore: &staleBefore,
		},
	}

	sql, args, err := buildCandidatePointersQuery(q)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(sql, "ORDER BY p.is_rule DESC, score DESC, p.pointer_key ASC") {
		t.Fatalf("unexpected ordering clause:\n%s", sql)
	}
	if !strings.Contains(sql, "WHEN 'plan' THEN") || !strings.Contains(sql, "WHEN 'execute' THEN") || !strings.Contains(sql, "WHEN 'review' THEN") {
		t.Fatalf("expected phase-weighted score expression:\n%s", sql)
	}
	if gotPhase, ok := args[1].(string); !ok || gotPhase != "plan" {
		t.Fatalf("expected phase arg plan, got %#v", args[1])
	}

	gotTags, ok := args[4].([]string)
	if !ok {
		t.Fatalf("expected tags arg []string, got %T", args[4])
	}
	wantTags := []string{"backend", "ops"}
	if !reflect.DeepEqual(gotTags, wantTags) {
		t.Fatalf("unexpected tags ordering: got %v want %v", gotTags, wantTags)
	}

	if gotLimit, ok := args[len(args)-1].(int); !ok || gotLimit != defaultCandidateLimit {
		t.Fatalf("expected default limit %d, got %#v", defaultCandidateLimit, args[len(args)-1])
	}
}

func TestBuildCandidatePointersQuery_DefaultPhase(t *testing.T) {
	sql, args, err := buildCandidatePointersQuery(core.CandidatePointerQuery{
		ProjectID: "project-a",
		TaskText:  "retrieve context",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(sql, "CASE $2::text") {
		t.Fatalf("expected phase argument in score expression:\n%s", sql)
	}
	if got, ok := args[1].(string); !ok || got != "execute" {
		t.Fatalf("expected default phase execute, got %#v", args[1])
	}
}

func TestBuildCandidatePointersQuery_UsesLiteralUnderscoreTestPathFallback(t *testing.T) {
	sql, _, err := buildCandidatePointersQuery(core.CandidatePointerQuery{
		ProjectID: "project-a",
		TaskText:  "execute tests",
		Phase:     "execute",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(sql, "strpos(lower(p.path), '_test.') > 0") {
		t.Fatalf("expected literal _test path fallback in phase weighting:\n%s", sql)
	}
	if strings.Contains(sql, "LIKE '%_test.%'") {
		t.Fatalf("did not expect wildcard LIKE _test fallback:\n%s", sql)
	}
}

func TestIsValidMigrationFilename(t *testing.T) {
	tests := []struct {
		name  string
		valid bool
	}{
		{name: "0001_init.sql", valid: true},
		{name: "../0002_bad.sql", valid: false},
		{name: "0002-bad.sql", valid: false},
		{name: `migrations\0003_bad.sql`, valid: false},
	}

	for _, tc := range tests {
		if got := isValidMigrationFilename(tc.name); got != tc.valid {
			t.Fatalf("filename %q validity mismatch: got %v want %v", tc.name, got, tc.valid)
		}
	}
}

func TestBuildFetchReceiptScopeQuery_DeterministicOrdering(t *testing.T) {
	sql, args, err := buildFetchReceiptScopeQuery(core.ReceiptScopeQuery{
		ProjectID: " project-a ",
		ReceiptID: " receipt-123 ",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(sql, "ARRAY_AGG(DISTINCT p.path ORDER BY p.path)") {
		t.Fatalf("expected deterministic pointer-path ordering in query:\n%s", sql)
	}
	if !strings.Contains(sql, "r.pointer_keys") {
		t.Fatalf("expected receipt metadata columns in query:\n%s", sql)
	}
	wantArgs := []any{"project-a", "receipt-123"}
	if !reflect.DeepEqual(args, wantArgs) {
		t.Fatalf("unexpected args: got %#v want %#v", args, wantArgs)
	}
}

func TestBuildFetchReceiptScopeQuery_ValidatesInput(t *testing.T) {
	if _, _, err := buildFetchReceiptScopeQuery(core.ReceiptScopeQuery{ProjectID: "", ReceiptID: "receipt-123"}); err == nil {
		t.Fatal("expected project_id validation error")
	}
	if _, _, err := buildFetchReceiptScopeQuery(core.ReceiptScopeQuery{ProjectID: "project-a", ReceiptID: ""}); err == nil {
		t.Fatal("expected receipt_id validation error")
	}
}

func TestBuildInsertMemoryCandidateQuery_DeterministicArgs(t *testing.T) {
	sql, args, err := buildInsertMemoryCandidateQuery(core.ProposeMemoryPersistence{
		ProjectID:           " project-a ",
		ReceiptID:           " receipt-123 ",
		Category:            "decision",
		Subject:             "  Keep deterministic dedupe  ",
		Content:             "  Persist candidate before promotion  ",
		Confidence:          4,
		Tags:                []string{"backend", "ops", "backend", " "},
		RelatedPointerKeys:  []string{"ptr:b", "ptr:a", "ptr:b"},
		EvidencePointerKeys: []string{"ptr:scope-2", "ptr:scope-1", "ptr:scope-2"},
		DedupeKey:           " dedupe-abc ",
		Validation: core.ProposeMemoryValidation{
			HardPassed: true,
			SoftPassed: false,
			Errors:     []string{},
			Warnings:   []string{"related pointer outside scope"},
		},
		AutoPromote: true,
		Promotable:  false,
	}, candidateStatusPending)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(sql, "INSERT INTO ctx_memory_candidates") {
		t.Fatalf("expected candidate insert SQL, got:\n%s", sql)
	}
	if !strings.Contains(sql, "RETURNING candidate_id") {
		t.Fatalf("expected RETURNING clause, got:\n%s", sql)
	}

	if got, ok := args[6].([]string); !ok || !reflect.DeepEqual(got, []string{"backend", "ops"}) {
		t.Fatalf("unexpected tags arg: %#v", args[6])
	}
	if got, ok := args[7].([]string); !ok || !reflect.DeepEqual(got, []string{"ptr:a", "ptr:b"}) {
		t.Fatalf("unexpected related_pointer_keys arg: %#v", args[7])
	}
	if got, ok := args[8].([]string); !ok || !reflect.DeepEqual(got, []string{"ptr:scope-1", "ptr:scope-2"}) {
		t.Fatalf("unexpected evidence_pointer_keys arg: %#v", args[8])
	}
	if gotStatus, ok := args[10].(string); !ok || gotStatus != candidateStatusPending {
		t.Fatalf("unexpected status arg: %#v", args[10])
	}
}

func TestBuildInsertMemoryCandidateQuery_ValidatesInput(t *testing.T) {
	_, _, err := buildInsertMemoryCandidateQuery(core.ProposeMemoryPersistence{
		ProjectID:  "project-a",
		ReceiptID:  "receipt-123",
		Category:   "decision",
		Subject:    "subject",
		Content:    "content",
		Confidence: 3,
		DedupeKey:  "dedupe",
	}, "invalid-status")
	if err == nil {
		t.Fatal("expected status validation error")
	}

	_, _, err = buildInsertMemoryCandidateQuery(core.ProposeMemoryPersistence{
		ProjectID:  "project-a",
		ReceiptID:  "receipt-123",
		Category:   "decision",
		Subject:    "subject",
		Content:    "content",
		Confidence: 3,
		DedupeKey:  "dedupe",
	}, candidateStatusPending)
	if err == nil {
		t.Fatal("expected evidence validation error")
	}
}

func TestBuildInsertDurableMemoryQuery_DeterministicArgs(t *testing.T) {
	sql, args, err := buildInsertDurableMemoryQuery(core.ProposeMemoryPersistence{
		ProjectID:           " project-a ",
		Category:            "decision",
		Subject:             "  Subject  ",
		Content:             "  Content  ",
		Confidence:          5,
		Tags:                []string{"b", "a", "b"},
		RelatedPointerKeys:  []string{"ptr:2", "ptr:1"},
		EvidencePointerKeys: []string{"ptr:1"},
		DedupeKey:           " dedupe-key ",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(sql, "INSERT INTO ctx_memories") || !strings.Contains(sql, "ON CONFLICT DO NOTHING") {
		t.Fatalf("expected durable memory insert with conflict handling, got:\n%s", sql)
	}
	if got, ok := args[5].([]string); !ok || !reflect.DeepEqual(got, []string{"a", "b"}) {
		t.Fatalf("unexpected tags arg: %#v", args[5])
	}
}

func TestBuildUpdateMemoryCandidateStatusQuery_ValidatesInput(t *testing.T) {
	if _, _, err := buildUpdateMemoryCandidateStatusQuery(0, candidateStatusPending, 0); err == nil {
		t.Fatal("expected candidate_id validation error")
	}
	if _, _, err := buildUpdateMemoryCandidateStatusQuery(1, "bad", 0); err == nil {
		t.Fatal("expected status validation error")
	}

	sql, args, err := buildUpdateMemoryCandidateStatusQuery(7, candidateStatusPromoted, 11)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(sql, "UPDATE ctx_memory_candidates") {
		t.Fatalf("expected candidate update SQL, got:\n%s", sql)
	}
	wantArgs := []any{int64(7), candidateStatusPromoted, int64(11)}
	if !reflect.DeepEqual(args, wantArgs) {
		t.Fatalf("unexpected args: got %#v want %#v", args, wantArgs)
	}
}

func TestBuildMarkDeletedPointersStaleQuery_DeterministicAndValidated(t *testing.T) {
	sql, args, err := buildMarkDeletedPointersStaleQuery(" project-a ", []string{" b/path.go ", "a/path.go", "a/path.go", ""})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(sql, "UPDATE ctx_pointers") || !strings.Contains(sql, "path = ANY") {
		t.Fatalf("unexpected SQL:\n%s", sql)
	}
	wantArgs := []any{"project-a", []string{"a/path.go", "b/path.go"}}
	if !reflect.DeepEqual(args, wantArgs) {
		t.Fatalf("unexpected args: got %#v want %#v", args, wantArgs)
	}

	if _, _, err := buildMarkDeletedPointersStaleQuery("", []string{"a/path.go"}); err == nil {
		t.Fatal("expected project_id validation error")
	}
	if _, _, err := buildMarkDeletedPointersStaleQuery("project-a", []string{"", " "}); err == nil {
		t.Fatal("expected deleted paths validation error")
	}
}

func TestBuildMarkMissingPointersStaleQuery_DeterministicAndValidated(t *testing.T) {
	sql, args, err := buildMarkMissingPointersStaleQuery(" project-a ", []string{" b/path.go ", "a/path.go", "a/path.go"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(sql, "UPDATE ctx_pointers") || !strings.Contains(sql, "NOT (path = ANY") {
		t.Fatalf("unexpected SQL:\n%s", sql)
	}
	wantArgs := []any{"project-a", []string{"a/path.go", "b/path.go"}}
	if !reflect.DeepEqual(args, wantArgs) {
		t.Fatalf("unexpected args: got %#v want %#v", args, wantArgs)
	}

	if _, _, err := buildMarkMissingPointersStaleQuery("", []string{"a/path.go"}); err == nil {
		t.Fatal("expected project_id validation error")
	}
}

func TestBuildRefreshPointersQuery_DeterministicAndValidated(t *testing.T) {
	sql, args, err := buildRefreshPointersQuery(" project-a ", []core.SyncPath{
		{Path: " b/path.go ", ContentHash: "bbbb"},
		{Path: "a/path.go", ContentHash: "aaaa"},
		{Path: "a/path.go", ContentHash: "aaaa"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(sql, "UPDATE ctx_pointers p") || !strings.Contains(sql, "content_hash") {
		t.Fatalf("unexpected SQL:\n%s", sql)
	}
	wantArgs := []any{"project-a", "a/path.go", "aaaa", "b/path.go", "bbbb"}
	if !reflect.DeepEqual(args, wantArgs) {
		t.Fatalf("unexpected args: got %#v want %#v", args, wantArgs)
	}

	if _, _, err := buildRefreshPointersQuery("", []core.SyncPath{{Path: "a", ContentHash: "x"}}); err == nil {
		t.Fatal("expected project_id validation error")
	}
	if _, _, err := buildRefreshPointersQuery("project-a", []core.SyncPath{{Path: "a", ContentHash: ""}}); err == nil {
		t.Fatal("expected content_hash validation error")
	}
}

func TestBuildInsertPointerCandidatesQuery_DeterministicAndValidated(t *testing.T) {
	sql, args, err := buildInsertPointerCandidatesQuery(" project-a ", []core.SyncPath{
		{Path: " b/path.go ", ContentHash: "bbbb"},
		{Path: "a/path.go", ContentHash: "aaaa"},
		{Path: "a/path.go", ContentHash: "aaaa"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(sql, "INSERT INTO ctx_pointer_candidates") || !strings.Contains(sql, "ON CONFLICT (project_id, path) DO NOTHING") {
		t.Fatalf("unexpected SQL:\n%s", sql)
	}
	wantArgs := []any{"project-a", "a/path.go", "aaaa", "b/path.go", "bbbb"}
	if !reflect.DeepEqual(args, wantArgs) {
		t.Fatalf("unexpected args: got %#v want %#v", args, wantArgs)
	}

	if _, _, err := buildInsertPointerCandidatesQuery("", []core.SyncPath{{Path: "a", ContentHash: "x"}}); err == nil {
		t.Fatal("expected project_id validation error")
	}
	if _, _, err := buildInsertPointerCandidatesQuery("project-a", []core.SyncPath{{Path: "", ContentHash: "x"}}); err == nil {
		t.Fatal("expected path validation error")
	}
}

func TestBuildLookupFetchStateQuery_DeterministicAndValidated(t *testing.T) {
	sql, args, err := buildLookupFetchStateQuery(core.FetchLookupQuery{
		ProjectID: " project-a ",
		ReceiptID: " receipt-123 ",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(sql, "FROM ctx_receipts") || !strings.Contains(sql, "LEFT JOIN LATERAL") {
		t.Fatalf("unexpected SQL:\n%s", sql)
	}
	if !strings.Contains(sql, "ORDER BY created_at DESC, run_id DESC") {
		t.Fatalf("expected deterministic run ordering in lookup query:\n%s", sql)
	}
	wantArgs := []any{"project-a", "receipt-123"}
	if !reflect.DeepEqual(args, wantArgs) {
		t.Fatalf("unexpected args: got %#v want %#v", args, wantArgs)
	}

	if _, _, err := buildLookupFetchStateQuery(core.FetchLookupQuery{ProjectID: "", ReceiptID: "receipt-123"}); err == nil {
		t.Fatal("expected project_id validation error")
	}
	if _, _, err := buildLookupFetchStateQuery(core.FetchLookupQuery{ProjectID: "project-a", ReceiptID: ""}); err == nil {
		t.Fatal("expected receipt_id validation error")
	}
}

func TestBuildLookupPointerByKeyQuery_DeterministicAndValidated(t *testing.T) {
	sql, args, err := buildLookupPointerByKeyQuery(core.PointerLookupQuery{
		ProjectID:  " project-a ",
		PointerKey: " pointer-123 ",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(sql, "FROM ctx_pointers") || !strings.Contains(sql, "pointer_key = $2") {
		t.Fatalf("unexpected SQL:\n%s", sql)
	}
	wantArgs := []any{"project-a", "pointer-123"}
	if !reflect.DeepEqual(args, wantArgs) {
		t.Fatalf("unexpected args: got %#v want %#v", args, wantArgs)
	}

	if _, _, err := buildLookupPointerByKeyQuery(core.PointerLookupQuery{ProjectID: "", PointerKey: "pointer-123"}); err == nil {
		t.Fatal("expected project_id validation error")
	}
	if _, _, err := buildLookupPointerByKeyQuery(core.PointerLookupQuery{ProjectID: "project-a", PointerKey: ""}); err == nil {
		t.Fatal("expected pointer_key validation error")
	}
}

func TestBuildLookupMemoryByIDQuery_DeterministicAndValidated(t *testing.T) {
	sql, args, err := buildLookupMemoryByIDQuery(core.MemoryLookupQuery{
		ProjectID: " project-a ",
		MemoryID:  42,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(sql, "FROM ctx_memories") || !strings.Contains(sql, "memory_id = $2") {
		t.Fatalf("unexpected SQL:\n%s", sql)
	}
	wantArgs := []any{"project-a", int64(42)}
	if !reflect.DeepEqual(args, wantArgs) {
		t.Fatalf("unexpected args: got %#v want %#v", args, wantArgs)
	}

	if _, _, err := buildLookupMemoryByIDQuery(core.MemoryLookupQuery{ProjectID: "", MemoryID: 42}); err == nil {
		t.Fatal("expected project_id validation error")
	}
	if _, _, err := buildLookupMemoryByIDQuery(core.MemoryLookupQuery{ProjectID: "project-a", MemoryID: 0}); err == nil {
		t.Fatal("expected memory_id validation error")
	}
}

func TestBuildListWorkItemsQuery_DeterministicAndValidated(t *testing.T) {
	sql, args, err := buildListWorkItemsQuery(core.FetchLookupQuery{
		ProjectID: " project-a ",
		ReceiptID: " receipt-123 ",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(sql, "FROM ctx_work_items") || !strings.Contains(sql, "ORDER BY item_key ASC") {
		t.Fatalf("unexpected SQL:\n%s", sql)
	}
	wantArgs := []any{"project-a", "receipt-123"}
	if !reflect.DeepEqual(args, wantArgs) {
		t.Fatalf("unexpected args: got %#v want %#v", args, wantArgs)
	}

	if _, _, err := buildListWorkItemsQuery(core.FetchLookupQuery{ProjectID: "", ReceiptID: "receipt-123"}); err == nil {
		t.Fatal("expected project_id validation error")
	}
	if _, _, err := buildListWorkItemsQuery(core.FetchLookupQuery{ProjectID: "project-a", ReceiptID: ""}); err == nil {
		t.Fatal("expected receipt_id validation error")
	}
}

func TestBuildUpsertWorkItemsQuery_DeterministicAndValidated(t *testing.T) {
	sql, args, err := buildUpsertWorkItemsQuery(core.WorkItemsUpsertInput{
		ProjectID: " project-a ",
		ReceiptID: " receipt-123 ",
		Items: []core.WorkItem{
			{ItemKey: " src/b.go ", Status: core.WorkItemStatusCompleted},
			{ItemKey: "src/a.go", Status: core.WorkItemStatusInProgress},
			{ItemKey: "src/a.go", Status: core.WorkItemStatusBlocked},
			{ItemKey: "src/c.go", Status: "unknown"},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(sql, "INSERT INTO ctx_work_items") || !strings.Contains(sql, "ON CONFLICT (project_id, receipt_id, item_key) DO UPDATE") {
		t.Fatalf("unexpected SQL:\n%s", sql)
	}
	if !strings.Contains(sql, "WITH incoming(item_key, status)") {
		t.Fatalf("expected deterministic VALUES CTE in SQL:\n%s", sql)
	}

	wantArgs := []any{
		"project-a",
		"receipt-123",
		"src/a.go", core.WorkItemStatusBlocked,
		"src/b.go", core.WorkItemStatusCompleted,
		"src/c.go", core.WorkItemStatusPending,
	}
	if !reflect.DeepEqual(args, wantArgs) {
		t.Fatalf("unexpected args: got %#v want %#v", args, wantArgs)
	}

	if _, _, err := buildUpsertWorkItemsQuery(core.WorkItemsUpsertInput{ProjectID: "", ReceiptID: "receipt-123", Items: []core.WorkItem{{ItemKey: "src/a.go", Status: core.WorkItemStatusPending}}}); err == nil {
		t.Fatal("expected project_id validation error")
	}
	if _, _, err := buildUpsertWorkItemsQuery(core.WorkItemsUpsertInput{ProjectID: "project-a", ReceiptID: "", Items: []core.WorkItem{{ItemKey: "src/a.go", Status: core.WorkItemStatusPending}}}); err == nil {
		t.Fatal("expected receipt_id validation error")
	}
	if _, _, err := buildUpsertWorkItemsQuery(core.WorkItemsUpsertInput{ProjectID: "project-a", ReceiptID: "receipt-123", Items: []core.WorkItem{{ItemKey: "", Status: core.WorkItemStatusPending}}}); err == nil {
		t.Fatal("expected item key validation error")
	}
}

func TestNormalizeWorkItemStatus_NormalizesLegacyCompleted(t *testing.T) {
	if got := normalizeWorkItemStatus(core.WorkItemStatusCompleted); got != core.WorkItemStatusComplete {
		t.Fatalf("legacy completed should normalize to complete, got %q", got)
	}
	if got := normalizeWorkItemStatus(core.WorkItemStatusComplete); got != core.WorkItemStatusComplete {
		t.Fatalf("canonical complete should remain complete, got %q", got)
	}
	if got := storageWorkItemStatus(core.WorkItemStatusComplete); got != core.WorkItemStatusCompleted {
		t.Fatalf("storage status should persist as completed for compatibility, got %q", got)
	}
}
