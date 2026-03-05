package sqlite

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"sort"
	"testing"
	"time"

	"github.com/joshd/agent-context-manager/internal/contracts/v1"
	"github.com/joshd/agent-context-manager/internal/core"
	postgressvc "github.com/joshd/agent-context-manager/internal/service/postgres"
)

func TestRepositoryParity_ServiceFlows(t *testing.T) {
	ctx := context.Background()
	projectID := fmt.Sprintf("project.sqlite.%d", time.Now().UTC().UnixNano())

	dbPath := filepath.Join(t.TempDir(), "ctx.sqlite")
	repo, err := New(ctx, Config{Path: dbPath})
	if err != nil {
		t.Fatalf("new sqlite repository: %v", err)
	}
	t.Cleanup(func() { _ = repo.Close() })

	seedPointer(t, ctx, repo, seedPointerRow{
		ProjectID:   projectID,
		PointerKey:  "pointer.runtime.default",
		Path:        "docs/runtime.md",
		Kind:        "doc",
		Label:       "Runtime default backend",
		Description: "SQLite default runtime path",
		Tags:        []string{"runtime", "sqlite"},
		ContentHash: "old-hash",
	})

	svc, err := postgressvc.New(repo)
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	getContextResult, apiErr := svc.GetContext(ctx, v1.GetContextPayload{
		ProjectID: projectID,
		TaskText:  "verify sqlite runtime backend path",
		Phase:     v1.PhaseExecute,
		Caps: &v1.RetrievalCaps{
			MinPointerCount: 1,
			MaxMemories:     2,
		},
	})
	if apiErr != nil {
		t.Fatalf("get_context API error: %+v", apiErr)
	}
	if getContextResult.Status != "ok" || getContextResult.Receipt == nil {
		t.Fatalf("unexpected get_context result: %+v", getContextResult)
	}

	receipt := getContextResult.Receipt
	pointerKeySet := make(map[string]struct{}, len(receipt.Rules)+len(receipt.Suggestions))
	for _, rule := range receipt.Rules {
		pointerKeySet[rule.Key] = struct{}{}
	}
	for _, suggestion := range receipt.Suggestions {
		pointerKeySet[suggestion.Key] = struct{}{}
	}
	pointerKeys := make([]string, 0, len(pointerKeySet))
	for key := range pointerKeySet {
		if key == "" {
			continue
		}
		pointerKeys = append(pointerKeys, key)
	}
	sort.Strings(pointerKeys)

	_, err = repo.SaveRunReceiptSummary(ctx, core.RunReceiptSummary{
		ProjectID:    projectID,
		ReceiptID:    receipt.Meta.ReceiptID,
		TaskText:     receipt.Meta.TaskText,
		Phase:        string(receipt.Meta.Phase),
		Status:       "accepted",
		ResolvedTags: append([]string(nil), receipt.Meta.ResolvedTags...),
		PointerKeys:  pointerKeys,
	})
	if err != nil {
		t.Fatalf("save receipt scope summary: %v", err)
	}

	autoPromote := false
	proposeResult, apiErr := svc.ProposeMemory(ctx, v1.ProposeMemoryPayload{
		ProjectID:   projectID,
		ReceiptID:   receipt.Meta.ReceiptID,
		AutoPromote: &autoPromote,
		Memory: v1.MemoryPayload{
			Category:            v1.MemoryCategoryDecision,
			Subject:             "SQLite default backend enabled",
			Content:             "Runtime defaults to SQLite when ACM_PG_DSN is unset.",
			RelatedPointerKeys:  []string{pointerKeys[0]},
			Tags:                []string{"runtime", "sqlite"},
			Confidence:          4,
			EvidencePointerKeys: []string{pointerKeys[0]},
		},
	})
	if apiErr != nil {
		t.Fatalf("propose_memory API error: %+v", apiErr)
	}
	if proposeResult.CandidateID <= 0 || proposeResult.Status != "pending" {
		t.Fatalf("unexpected propose_memory result: %+v", proposeResult)
	}
	if !proposeResult.Validation.HardPassed || !proposeResult.Validation.SoftPassed {
		t.Fatalf("unexpected propose validation: %+v", proposeResult.Validation)
	}

	reportResult, apiErr := svc.ReportCompletion(ctx, v1.ReportCompletionPayload{
		ProjectID:    projectID,
		ReceiptID:    receipt.Meta.ReceiptID,
		FilesChanged: []string{"docs/runtime.md"},
		Outcome:      "sqlite flow accepted",
	})
	if apiErr != nil {
		t.Fatalf("report_completion API error: %+v", apiErr)
	}
	if !reportResult.Accepted || reportResult.RunID <= 0 {
		t.Fatalf("unexpected report_completion result: %+v", reportResult)
	}

	workResult, apiErr := svc.Work(ctx, v1.WorkPayload{
		ProjectID: projectID,
		PlanKey:   "plan:" + receipt.Meta.ReceiptID,
		ReceiptID: receipt.Meta.ReceiptID,
		Items: []v1.WorkItemPayload{
			{Key: "docs/runtime.md", Status: v1.WorkItemStatusComplete},
		},
	})
	if apiErr != nil {
		t.Fatalf("work API error: %+v", apiErr)
	}
	if workResult.PlanKey != "plan:"+receipt.Meta.ReceiptID || workResult.PlanStatus != string(core.PlanStatusComplete) || workResult.Updated != 1 {
		t.Fatalf("unexpected work result: %+v", workResult)
	}

	workItems, err := repo.ListWorkItems(ctx, core.FetchLookupQuery{
		ProjectID: projectID,
		ReceiptID: receipt.Meta.ReceiptID,
	})
	if err != nil {
		t.Fatalf("list work items: %v", err)
	}
	if len(workItems) != 1 {
		t.Fatalf("expected one work item, got %+v", workItems)
	}
	if workItems[0].ItemKey != "docs/runtime.md" || workItems[0].Status != core.WorkItemStatusComplete {
		t.Fatalf("unexpected persisted work item: %+v", workItems[0])
	}

	fetchLookup, err := repo.LookupFetchState(ctx, core.FetchLookupQuery{
		ProjectID: projectID,
		ReceiptID: receipt.Meta.ReceiptID,
	})
	if err != nil {
		t.Fatalf("lookup fetch state: %v", err)
	}
	if fetchLookup.PlanStatus != core.PlanStatusComplete {
		t.Fatalf("unexpected plan status: %q", fetchLookup.PlanStatus)
	}
	if fetchLookup.RunID != int64(reportResult.RunID) {
		t.Fatalf("expected fetch lookup to return latest report_completion run_id %d, got %d", reportResult.RunID, fetchLookup.RunID)
	}

	projectRoot := setupGitRepo(t, map[string]string{
		"docs/runtime.md": "runtime pointer content",
		"docs/new.md":     "new pointer candidate",
	})
	syncResult, apiErr := svc.Sync(ctx, v1.SyncPayload{
		ProjectID:   projectID,
		Mode:        "full",
		ProjectRoot: projectRoot,
	})
	if apiErr != nil {
		t.Fatalf("sync API error: %+v", apiErr)
	}

	wantProcessed := []string{"docs/new.md", "docs/runtime.md"}
	if !reflect.DeepEqual(syncResult.ProcessedPaths, wantProcessed) {
		t.Fatalf("unexpected processed paths: got %v want %v", syncResult.ProcessedPaths, wantProcessed)
	}
	if syncResult.Updated != 1 {
		t.Fatalf("unexpected sync updated count: got %d want 1", syncResult.Updated)
	}
	if syncResult.NewCandidates != 1 {
		t.Fatalf("unexpected sync new_candidates count: got %d want 1", syncResult.NewCandidates)
	}
	if syncResult.MarkedStale != 0 || syncResult.DeletedMarkedStale != 0 {
		t.Fatalf("unexpected stale counters: %+v", syncResult)
	}
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

func setupGitRepo(t *testing.T, files map[string]string) string {
	t.Helper()

	root := t.TempDir()
	runCommand(t, root, "git", "init")
	runCommand(t, root, "git", "config", "user.email", "sqlite-test@example.com")
	runCommand(t, root, "git", "config", "user.name", "SQLite Test")

	paths := make([]string, 0, len(files))
	for p := range files {
		paths = append(paths, p)
	}
	sort.Strings(paths)
	for _, p := range paths {
		abs := filepath.Join(root, filepath.FromSlash(p))
		if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
			t.Fatalf("mkdir %q: %v", abs, err)
		}
		if err := os.WriteFile(abs, []byte(files[p]), 0o644); err != nil {
			t.Fatalf("write file %q: %v", abs, err)
		}
	}

	runCommand(t, root, "git", "add", ".")
	runCommand(t, root, "git", "commit", "-m", "seed")
	return root
}

func runCommand(t *testing.T, dir string, name string, args ...string) {
	t.Helper()

	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("run %s %v: %v\n%s", name, args, err, string(out))
	}
}
