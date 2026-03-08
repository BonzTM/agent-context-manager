package sqlite

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/bonztm/agent-context-manager/internal/core"
	"github.com/bonztm/agent-context-manager/internal/testutil/repositorycontract"
)

func TestRepositoryParity(t *testing.T) {
	ctx := context.Background()

	dbPath := filepath.Join(t.TempDir(), "ctx.sqlite")
	repo, err := New(ctx, Config{Path: dbPath})
	if err != nil {
		t.Fatalf("new sqlite repository: %v", err)
	}
	t.Cleanup(func() { _ = repo.Close() })

	repositorycontract.RunRepositoryParity(t, repositorycontract.ContractConfig{
		BackendLabel:        "sqlite",
		ProjectID:           fmt.Sprintf("project.sqlite.%d", time.Now().UTC().UnixNano()),
		Repo:                repo,
		IncludeServiceFlows: true,
	})
}

func TestRepository_LookupPointerByKeyAndLookupMemoryByID(t *testing.T) {
	ctx := context.Background()
	projectID := fmt.Sprintf("project.sqlite.lookup.%d", time.Now().UTC().UnixNano())

	dbPath := filepath.Join(t.TempDir(), "ctx.sqlite")
	repo, err := New(ctx, Config{Path: dbPath})
	if err != nil {
		t.Fatalf("new sqlite repository: %v", err)
	}
	t.Cleanup(func() { _ = repo.Close() })

	seedPointer(t, ctx, repo, seedPointerRow{
		ProjectID:   projectID,
		PointerKey:  "pointer.lookup",
		Path:        "docs/lookup.md",
		Kind:        "doc",
		Label:       "Lookup pointer",
		Description: "Pointer used for fetch expansion lookup tests",
		Tags:        []string{"lookup", "fetch"},
		ContentHash: "lookup-hash",
	})

	memoryID := seedMemory(t, ctx, repo, seedMemoryRow{
		ProjectID:           projectID,
		Category:            "decision",
		Subject:             "Lookup memory",
		Content:             "Fetch expansion should resolve memory by id.",
		Confidence:          4,
		Tags:                []string{"memory", "fetch"},
		RelatedPointerKeys:  []string{"pointer.lookup"},
		EvidencePointerKeys: []string{"pointer.lookup"},
		DedupeKey:           "lookup-memory-dedupe",
	})

	pointer, err := repo.LookupPointerByKey(ctx, core.PointerLookupQuery{
		ProjectID:  " " + projectID + " ",
		PointerKey: " pointer.lookup ",
	})
	if err != nil {
		t.Fatalf("lookup pointer by key: %v", err)
	}
	if pointer.Key != "pointer.lookup" || pointer.Path != "docs/lookup.md" || pointer.Kind != "doc" {
		t.Fatalf("unexpected pointer lookup result: %+v", pointer)
	}
	if !reflect.DeepEqual(pointer.Tags, []string{"fetch", "lookup"}) {
		t.Fatalf("unexpected pointer tags: %+v", pointer.Tags)
	}

	memory, err := repo.LookupMemoryByID(ctx, core.MemoryLookupQuery{
		ProjectID: " " + projectID + " ",
		MemoryID:  memoryID,
	})
	if err != nil {
		t.Fatalf("lookup memory by id: %v", err)
	}
	if memory.ID != memoryID || memory.Category != "decision" || memory.Subject != "Lookup memory" {
		t.Fatalf("unexpected memory lookup result: %+v", memory)
	}
	if !reflect.DeepEqual(memory.RelatedPointerKeys, []string{"pointer.lookup"}) {
		t.Fatalf("unexpected memory related pointers: %+v", memory.RelatedPointerKeys)
	}

	if _, err := repo.LookupPointerByKey(ctx, core.PointerLookupQuery{ProjectID: projectID, PointerKey: "pointer.missing"}); !errors.Is(err, core.ErrPointerLookupNotFound) {
		t.Fatalf("expected pointer lookup not found error, got %v", err)
	}
	if _, err := repo.LookupMemoryByID(ctx, core.MemoryLookupQuery{ProjectID: projectID, MemoryID: memoryID + 999}); !errors.Is(err, core.ErrMemoryLookupNotFound) {
		t.Fatalf("expected memory lookup not found error, got %v", err)
	}
}

func TestRepository_WorkPlanHierarchyRoundTrip(t *testing.T) {
	ctx := context.Background()
	projectID := fmt.Sprintf("project.sqlite.workplan.%d", time.Now().UTC().UnixNano())

	dbPath := filepath.Join(t.TempDir(), "ctx.sqlite")
	repo, err := New(ctx, Config{Path: dbPath})
	if err != nil {
		t.Fatalf("new sqlite repository: %v", err)
	}
	t.Cleanup(func() { _ = repo.Close() })

	result, err := repo.UpsertWorkPlan(ctx, core.WorkPlanUpsertInput{
		ProjectID:     projectID,
		PlanKey:       "plan:receipt.abc123",
		ReceiptID:     "receipt.abc123",
		Mode:          core.WorkPlanModeReplace,
		Title:         "Import Optimization",
		Objective:     "Keep task fetches compact",
		Kind:          "story",
		ParentPlanKey: "plan:receipt.parent123",
		ExternalRefs:  []string{"jira:ACM-1"},
		Tasks: []core.WorkItem{
			{
				ItemKey:       "task.blocked",
				Summary:       "Resolve API limit issue",
				Status:        core.WorkItemStatusBlocked,
				ParentTaskKey: "task.epic",
				ExternalRefs:  []string{"linear:ENG-3"},
			},
			{
				ItemKey: "task.active",
				Summary: "Ship MCP parity",
				Status:  core.WorkItemStatusInProgress,
			},
			{
				ItemKey: "task.done",
				Summary: "Cut migration",
				Status:  core.WorkItemStatusComplete,
			},
		},
	})
	if err != nil {
		t.Fatalf("upsert work plan: %v", err)
	}
	if result.Plan.Kind != "story" || result.Plan.ParentPlanKey != "plan:receipt.parent123" {
		t.Fatalf("unexpected upserted plan metadata: %+v", result.Plan)
	}

	plan, err := repo.LookupWorkPlan(ctx, core.WorkPlanLookupQuery{
		ProjectID: projectID,
		PlanKey:   "plan:receipt.abc123",
	})
	if err != nil {
		t.Fatalf("lookup work plan: %v", err)
	}
	if plan.Kind != "story" || plan.ParentPlanKey != "plan:receipt.parent123" {
		t.Fatalf("unexpected plan hierarchy fields: %+v", plan)
	}
	if !reflect.DeepEqual(plan.ExternalRefs, []string{"jira:ACM-1"}) {
		t.Fatalf("unexpected plan external refs: %+v", plan.ExternalRefs)
	}
	if len(plan.Tasks) != 3 {
		t.Fatalf("expected three tasks, got %+v", plan.Tasks)
	}
	if plan.Tasks[0].ItemKey != "task.active" || plan.Tasks[1].ItemKey != "task.blocked" || plan.Tasks[2].ItemKey != "task.done" {
		t.Fatalf("unexpected persisted task order: %+v", plan.Tasks)
	}
	if plan.Tasks[1].ParentTaskKey != "task.epic" {
		t.Fatalf("unexpected parent task key: %+v", plan.Tasks[1])
	}
	if !reflect.DeepEqual(plan.Tasks[1].ExternalRefs, []string{"linear:ENG-3"}) {
		t.Fatalf("unexpected task external refs: %+v", plan.Tasks[1].ExternalRefs)
	}

	summaries, err := repo.ListWorkPlans(ctx, core.WorkPlanListQuery{
		ProjectID: projectID,
		Limit:     8,
	})
	if err != nil {
		t.Fatalf("list work plans: %v", err)
	}
	if len(summaries) != 1 {
		t.Fatalf("expected one work plan summary, got %+v", summaries)
	}
	summary := summaries[0]
	if summary.Kind != "story" || summary.ParentPlanKey != "plan:receipt.parent123" {
		t.Fatalf("unexpected summary metadata: %+v", summary)
	}
	if summary.TaskCountTotal != 3 || summary.TaskCountBlocked != 1 || summary.TaskCountInProgress != 1 || summary.TaskCountComplete != 1 {
		t.Fatalf("unexpected summary counts: %+v", summary)
	}
	if !reflect.DeepEqual(summary.ActiveTaskKeys, []string{"task.blocked", "task.active"}) {
		t.Fatalf("unexpected active task keys: %+v", summary.ActiveTaskKeys)
	}
}

type seedPointerRow struct {
	ProjectID   string
	PointerKey  string
	Path        string
	Kind        string
	Label       string
	Description string
	Tags        []string
	ContentHash string
}

func seedPointer(t *testing.T, ctx context.Context, repo *Repository, row seedPointerRow) {
	t.Helper()

	tagsJSON, err := encodeStringList(row.Tags)
	if err != nil {
		t.Fatalf("encode tags: %v", err)
	}
	_, err = repo.db.ExecContext(ctx, `
INSERT INTO acm_pointers (
	project_id,
	pointer_key,
	path,
	anchor,
	kind,
	label,
	description,
	tags_json,
	is_rule,
	is_stale,
	content_hash,
	updated_at
) VALUES (?, ?, ?, '', ?, ?, ?, ?, 0, 0, ?, unixepoch())
`, row.ProjectID, row.PointerKey, row.Path, row.Kind, row.Label, row.Description, tagsJSON, row.ContentHash)
	if err != nil {
		t.Fatalf("seed pointer %q: %v", row.PointerKey, err)
	}
}

type seedMemoryRow struct {
	ProjectID           string
	Category            string
	Subject             string
	Content             string
	Confidence          int
	Tags                []string
	RelatedPointerKeys  []string
	EvidencePointerKeys []string
	DedupeKey           string
}

func seedMemory(t *testing.T, ctx context.Context, repo *Repository, row seedMemoryRow) int64 {
	t.Helper()

	tagsJSON, err := encodeStringList(row.Tags)
	if err != nil {
		t.Fatalf("encode memory tags: %v", err)
	}
	relatedJSON, err := encodeStringList(row.RelatedPointerKeys)
	if err != nil {
		t.Fatalf("encode related pointers: %v", err)
	}
	evidenceJSON, err := encodeStringList(row.EvidencePointerKeys)
	if err != nil {
		t.Fatalf("encode evidence pointers: %v", err)
	}

	insertResult, err := repo.db.ExecContext(ctx, `
INSERT INTO acm_memories (
	project_id,
	category,
	subject,
	content,
	confidence,
	tags_json,
	related_pointer_keys_json,
	evidence_pointer_keys_json,
	dedupe_key,
	active,
	created_at,
	updated_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, 1, unixepoch(), unixepoch())
`, row.ProjectID, row.Category, row.Subject, row.Content, row.Confidence, tagsJSON, relatedJSON, evidenceJSON, row.DedupeKey)
	if err != nil {
		t.Fatalf("seed memory %q: %v", row.Subject, err)
	}
	memoryID, err := insertResult.LastInsertId()
	if err != nil {
		t.Fatalf("read seeded memory id: %v", err)
	}
	return memoryID
}
