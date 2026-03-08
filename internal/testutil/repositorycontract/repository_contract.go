package repositorycontract

import (
	"context"
	"fmt"
	"reflect"
	"testing"
	"time"

	"github.com/bonztm/agent-context-manager/internal/core"
)

type ContractRepository interface {
	core.Repository
	core.WorkPlanRepository
	core.HistoryRepository
}

type ContractConfig struct {
	BackendLabel        string
	ProjectID           string
	Repo                ContractRepository
	IncludeServiceFlows bool
}

func RunRepositoryParity(t *testing.T, cfg ContractConfig) {
	t.Helper()

	if cfg.Repo == nil {
		t.Fatal("repository is required")
	}

	projectID := cfg.ProjectID
	if projectID == "" {
		projectID = fmt.Sprintf("project.%s.%d", cfg.BackendLabel, time.Now().UTC().UnixNano())
	}

	t.Run("candidate_pointer_retrieval", func(t *testing.T) {
		runCandidatePointerRetrieval(t, projectID+".candidates", cfg.Repo)
	})
	t.Run("candidate_pointer_ranking", func(t *testing.T) {
		runCandidatePointerRanking(t, projectID+".candidate-ranking", cfg.Repo)
	})
	t.Run("lookup_round_trip", func(t *testing.T) {
		runLookupRoundTrip(t, projectID+".lookup", cfg.Repo)
	})
	t.Run("active_memory_retrieval", func(t *testing.T) {
		runActiveMemoryRetrieval(t, projectID+".memory", cfg.Repo)
	})
	t.Run("work_plan_round_trip", func(t *testing.T) {
		runWorkPlanRoundTrip(t, projectID+".workplan", cfg.Repo)
	})
	t.Run("review_attempt_round_trip", func(t *testing.T) {
		runReviewAttemptRoundTrip(t, projectID+".review", cfg.Repo)
	})
	t.Run("run_summary_round_trip", func(t *testing.T) {
		runRunSummaryRoundTrip(t, projectID+".runs", cfg.Repo)
	})
	if cfg.IncludeServiceFlows {
		t.Run("service_flows", func(t *testing.T) {
			RunServiceFlows(t, ServiceFlowConfig{
				BackendLabel: cfg.BackendLabel,
				ProjectID:    projectID + ".service",
				Repo:         cfg.Repo,
			})
		})
	}
}

func runCandidatePointerRetrieval(t *testing.T, projectID string, repo ContractRepository) {
	t.Helper()
	ctx := context.Background()

	stubs := []core.PointerStub{
		{
			PointerKey:  "pointer.rule.plan",
			Path:        "docs/rules.md",
			Kind:        "rule",
			Label:       "Plan rule",
			Description: "Plan workflow rule",
			Tags:        []string{"plan", "workflow"},
		},
		{
			PointerKey:  "pointer.doc.plan",
			Path:        "docs/plan.md",
			Kind:        "doc",
			Label:       "Plan guide",
			Description: "Plan workflow guide",
			Tags:        []string{"guide", "plan"},
		},
		{
			PointerKey:  "pointer.code.execute",
			Path:        "internal/runtime/service.go",
			Kind:        "code",
			Label:       "Execute handler",
			Description: "Execute runtime handler",
			Tags:        []string{"execute", "runtime"},
		},
		{
			PointerKey:  "pointer.test.execute",
			Path:        "internal/runtime/service_test.go",
			Kind:        "test",
			Label:       "Execute tests",
			Description: "Execute runtime tests",
			Tags:        []string{"execute", "tests"},
		},
	}
	if _, err := repo.UpsertPointerStubs(ctx, projectID, stubs); err != nil {
		t.Fatalf("seed candidate pointers: %v", err)
	}

	planRows, err := repo.FetchCandidatePointers(ctx, core.CandidatePointerQuery{
		ProjectID: projectID,
		TaskText:  "plan",
		Phase:     " PLAN ",
		Limit:     10,
	})
	if err != nil {
		t.Fatalf("fetch plan candidates: %v", err)
	}
	if got := candidateKeys(planRows); !reflect.DeepEqual(got, []string{"pointer.rule.plan", "pointer.doc.plan"}) {
		t.Fatalf("unexpected plan candidate order: got %v", got)
	}

	reviewRows, err := repo.FetchCandidatePointers(ctx, core.CandidatePointerQuery{
		ProjectID: projectID,
		TaskText:  "execute",
		Tags:      []string{"execute"},
		Phase:     " ReViEw ",
		Limit:     10,
	})
	if err != nil {
		t.Fatalf("fetch review candidates: %v", err)
	}
	if got := candidateKeys(reviewRows); !reflect.DeepEqual(got, []string{"pointer.test.execute", "pointer.code.execute"}) {
		t.Fatalf("unexpected review candidate order: got %v", got)
	}
}

func runCandidatePointerRanking(t *testing.T, projectID string, repo ContractRepository) {
	t.Helper()
	ctx := context.Background()

	stubs := []core.PointerStub{
		{
			PointerKey:  "pointer.doc.alpha",
			Path:        "docs/alpha.md",
			Kind:        "doc",
			Label:       "Alpha guide",
			Description: "Alpha guidance for review and planning",
			Tags:        []string{"alpha", "focus"},
		},
		{
			PointerKey:  "pointer.code.alpha",
			Path:        "internal/alpha/runtime.go",
			Kind:        "code",
			Label:       "Alpha runtime",
			Description: "Alpha execution handler",
			Tags:        []string{"alpha", "focus"},
		},
		{
			PointerKey:  "pointer.test.alpha",
			Path:        "internal/alpha/runtime_test.go",
			Kind:        "test",
			Label:       "Alpha tests",
			Description: "Alpha verification coverage",
			Tags:        []string{"alpha", "focus"},
		},
		{
			PointerKey:  "pointer.code.full",
			Path:        "internal/ranking/full.go",
			Kind:        "code",
			Label:       "Alpha beta ranking",
			Description: "Alpha beta retrieval ranking reference",
			Tags:        []string{"focus", "ranking"},
		},
		{
			PointerKey:  "pointer.code.tag",
			Path:        "internal/ranking/tag.go",
			Kind:        "code",
			Label:       "Ranking tag signal",
			Description: "Focus-only retrieval signal",
			Tags:        []string{"focus"},
		},
		{
			PointerKey:  "pointer.code.text",
			Path:        "internal/ranking/text.go",
			Kind:        "code",
			Label:       "Alpha text signal",
			Description: "Text-only retrieval signal",
			Tags:        []string{"ranking"},
		},
	}
	if _, err := repo.UpsertPointerStubs(ctx, projectID, stubs); err != nil {
		t.Fatalf("seed ranking candidate pointers: %v", err)
	}

	executeRows, err := repo.FetchCandidatePointers(ctx, core.CandidatePointerQuery{
		ProjectID: projectID,
		TaskText:  "alpha",
		Tags:      []string{"focus"},
		Phase:     "execute",
		Limit:     10,
	})
	if err != nil {
		t.Fatalf("fetch execute ranking candidates: %v", err)
	}
	if got := candidateKeys(executeRows); !reflect.DeepEqual(got, []string{
		"pointer.code.alpha",
		"pointer.code.full",
		"pointer.code.tag",
		"pointer.test.alpha",
		"pointer.code.text",
		"pointer.doc.alpha",
	}) {
		t.Fatalf("unexpected execute ranking order: got %v", got)
	}
	assertCandidateRanks(t, executeRows, map[string]float64{
		"pointer.code.alpha": 45,
		"pointer.code.full":  45,
		"pointer.code.tag":   30,
		"pointer.test.alpha": 30,
		"pointer.code.text":  15,
		"pointer.doc.alpha":  15,
	})

	reviewRows, err := repo.FetchCandidatePointers(ctx, core.CandidatePointerQuery{
		ProjectID: projectID,
		TaskText:  "alpha",
		Tags:      []string{"focus"},
		Phase:     "review",
		Limit:     10,
	})
	if err != nil {
		t.Fatalf("fetch review ranking candidates: %v", err)
	}
	if got := candidateKeys(reviewRows); !reflect.DeepEqual(got, []string{
		"pointer.test.alpha",
		"pointer.code.alpha",
		"pointer.code.full",
		"pointer.doc.alpha",
		"pointer.code.tag",
		"pointer.code.text",
	}) {
		t.Fatalf("unexpected review ranking order: got %v", got)
	}
	assertCandidateRanks(t, reviewRows, map[string]float64{
		"pointer.test.alpha": 30,
		"pointer.code.alpha": 15,
		"pointer.code.full":  15,
		"pointer.doc.alpha":  15,
		"pointer.code.tag":   10,
		"pointer.code.text":  5,
	})

	planRows, err := repo.FetchCandidatePointers(ctx, core.CandidatePointerQuery{
		ProjectID: projectID,
		TaskText:  "alpha",
		Tags:      []string{"focus"},
		Phase:     "plan",
		Limit:     10,
	})
	if err != nil {
		t.Fatalf("fetch plan ranking candidates: %v", err)
	}
	if got := candidateKeys(planRows); !reflect.DeepEqual(got, []string{
		"pointer.doc.alpha",
		"pointer.code.alpha",
		"pointer.code.full",
		"pointer.test.alpha",
		"pointer.code.tag",
		"pointer.code.text",
	}) {
		t.Fatalf("unexpected plan ranking order: got %v", got)
	}
	assertCandidateRanks(t, planRows, map[string]float64{
		"pointer.doc.alpha":  30,
		"pointer.code.alpha": 15,
		"pointer.code.full":  15,
		"pointer.test.alpha": 15,
		"pointer.code.tag":   10,
		"pointer.code.text":  5,
	})

	signalRows, err := repo.FetchCandidatePointers(ctx, core.CandidatePointerQuery{
		ProjectID: projectID,
		TaskText:  "alpha beta",
		Tags:      []string{"focus"},
		Phase:     "execute",
		Limit:     3,
	})
	if err != nil {
		t.Fatalf("fetch signal ranking candidates: %v", err)
	}
	if got := candidateKeys(signalRows); !reflect.DeepEqual(got, []string{
		"pointer.code.full",
		"pointer.code.alpha",
		"pointer.code.tag",
	}) {
		t.Fatalf("unexpected signal ranking order: got %v", got)
	}
	assertCandidateRanks(t, signalRows, map[string]float64{
		"pointer.code.full":  60,
		"pointer.code.alpha": 45,
		"pointer.code.tag":   30,
	})

	unboundedRows, err := repo.FetchCandidatePointers(ctx, core.CandidatePointerQuery{
		ProjectID: projectID,
		TaskText:  "alpha",
		Tags:      []string{"focus"},
		Phase:     "execute",
		Unbounded: true,
		Limit:     1,
	})
	if err != nil {
		t.Fatalf("fetch unbounded ranking candidates: %v", err)
	}
	if len(unboundedRows) != 6 {
		t.Fatalf("expected six unbounded ranking candidates, got %d", len(unboundedRows))
	}
}

func assertCandidateRanks(t *testing.T, rows []core.CandidatePointer, want map[string]float64) {
	t.Helper()

	got := make(map[string]float64, len(rows))
	for _, row := range rows {
		got[row.Key] = row.Rank
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected candidate ranks: got %#v want %#v", got, want)
	}
}

func runLookupRoundTrip(t *testing.T, projectID string, repo ContractRepository) {
	t.Helper()
	ctx := context.Background()

	if _, err := repo.UpsertPointerStubs(ctx, projectID, []core.PointerStub{{
		PointerKey:  "pointer.lookup",
		Path:        "docs/lookup.md",
		Kind:        "doc",
		Label:       "Lookup pointer",
		Description: "Pointer used for fetch expansion lookup tests",
		Tags:        []string{"fetch", "lookup"},
	}}); err != nil {
		t.Fatalf("seed lookup pointer: %v", err)
	}

	memory := persistPromotedMemory(t, ctx, repo, projectID, "pointer.lookup", "Lookup memory", "Fetch expansion should resolve memory by id.")

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

	activeMemory, err := repo.LookupMemoryByID(ctx, core.MemoryLookupQuery{
		ProjectID: " " + projectID + " ",
		MemoryID:  memory.ID,
	})
	if err != nil {
		t.Fatalf("lookup memory by id: %v", err)
	}
	if activeMemory.ID != memory.ID || activeMemory.Category != "decision" || activeMemory.Subject != "Lookup memory" {
		t.Fatalf("unexpected memory lookup result: %+v", activeMemory)
	}
	if !reflect.DeepEqual(activeMemory.RelatedPointerKeys, []string{"pointer.lookup"}) {
		t.Fatalf("unexpected memory related pointers: %+v", activeMemory.RelatedPointerKeys)
	}
}

func runActiveMemoryRetrieval(t *testing.T, projectID string, repo ContractRepository) {
	t.Helper()
	ctx := context.Background()

	if _, err := repo.UpsertPointerStubs(ctx, projectID, []core.PointerStub{{
		PointerKey:  "pointer.memory",
		Path:        "docs/memory.md",
		Kind:        "doc",
		Label:       "Memory pointer",
		Description: "Pointer used for active memory retrieval",
		Tags:        []string{"memory", "retrieval"},
	}}); err != nil {
		t.Fatalf("seed memory pointer: %v", err)
	}

	persistPromotedMemory(t, ctx, repo, projectID, "pointer.memory", "Backend parity", "Direct repository parity coverage is required.")

	rows, err := repo.FetchActiveMemories(ctx, core.ActiveMemoryQuery{
		ProjectID:   projectID,
		PointerKeys: []string{"pointer.memory"},
		Tags:        []string{"memory"},
		Limit:       10,
	})
	if err != nil {
		t.Fatalf("fetch active memories: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("expected one active memory, got %+v", rows)
	}
	if rows[0].Subject != "Backend parity" || rows[0].Category != "decision" {
		t.Fatalf("unexpected active memory: %+v", rows[0])
	}
}

func runWorkPlanRoundTrip(t *testing.T, projectID string, repo ContractRepository) {
	t.Helper()
	ctx := context.Background()

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

func runReviewAttemptRoundTrip(t *testing.T, projectID string, repo ContractRepository) {
	t.Helper()
	ctx := context.Background()
	exitCode := 0

	if err := repo.UpsertReceiptScope(ctx, core.ReceiptScope{
		ProjectID: projectID,
		ReceiptID: "receipt-review-1234",
		TaskText:  "repository contract review attempt seed",
		Phase:     "review",
	}); err != nil {
		t.Fatalf("seed receipt scope: %v", err)
	}

	attemptID, err := repo.SaveReviewAttempt(ctx, core.ReviewAttempt{
		ProjectID:          projectID,
		ReceiptID:          "receipt-review-1234",
		PlanKey:            "plan:receipt-review-1234",
		ReviewKey:          "review:cross-llm",
		Summary:            "Cross-LLM review completed",
		Fingerprint:        "sha256:review-attempt",
		Status:             "passed",
		Passed:             true,
		Outcome:            "No blocking issues.",
		WorkflowSourcePath: ".acm/acm-workflows.yaml",
		CommandArgv:        []string{"sh", "-c", "true"},
		CommandCWD:         ".",
		TimeoutSec:         30,
		ExitCode:           &exitCode,
		StdoutExcerpt:      "ok",
		CreatedAt:          time.Now().UTC(),
	})
	if err != nil {
		t.Fatalf("save review attempt: %v", err)
	}
	if attemptID <= 0 {
		t.Fatalf("expected positive review attempt id, got %d", attemptID)
	}

	rows, err := repo.ListReviewAttempts(ctx, core.ReviewAttemptListQuery{
		ProjectID: projectID,
		ReceiptID: "receipt-review-1234",
		ReviewKey: "review:cross-llm",
	})
	if err != nil {
		t.Fatalf("list review attempts: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("expected one review attempt, got %+v", rows)
	}
	if rows[0].AttemptID != attemptID || !rows[0].Passed || rows[0].Fingerprint != "sha256:review-attempt" {
		t.Fatalf("unexpected review attempt row: %+v", rows[0])
	}
}

func runRunSummaryRoundTrip(t *testing.T, projectID string, repo ContractRepository) {
	t.Helper()
	ctx := context.Background()

	runIDs, err := repo.SaveRunReceiptSummary(ctx, core.RunReceiptSummary{
		ProjectID:    projectID,
		RequestID:    "req-runsummary-12345",
		ReceiptID:    "receipt-runsummary-12345",
		TaskText:     "Verify repository contract parity",
		Phase:        "execute",
		Status:       "accepted",
		ResolvedTags: []string{"parity", "tests"},
		PointerKeys:  []string{"pointer.rule.plan"},
		FilesChanged: []string{"internal/testutil/repositorycontract/repository_contract.go"},
		Outcome:      "Parity suite persisted run summary.",
	})
	if err != nil {
		t.Fatalf("save run receipt summary: %v", err)
	}
	if runIDs.RunID <= 0 {
		t.Fatalf("expected positive run id, got %+v", runIDs)
	}

	rows, err := repo.ListRunHistory(ctx, core.RunHistoryListQuery{
		ProjectID: projectID,
		Query:     "repository contract parity",
		Limit:     10,
	})
	if err != nil {
		t.Fatalf("list run history: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("expected one run history row, got %+v", rows)
	}
	if rows[0].RunID != runIDs.RunID || rows[0].ReceiptID != "receipt-runsummary-12345" {
		t.Fatalf("unexpected run history row: %+v", rows[0])
	}

	row, err := repo.LookupRunHistory(ctx, core.RunHistoryLookupQuery{
		ProjectID: projectID,
		RunID:     runIDs.RunID,
	})
	if err != nil {
		t.Fatalf("lookup run history: %v", err)
	}
	if row.RunID != runIDs.RunID || row.RequestID != "req-runsummary-12345" {
		t.Fatalf("unexpected run history lookup: %+v", row)
	}
	if !reflect.DeepEqual(row.FilesChanged, []string{"internal/testutil/repositorycontract/repository_contract.go"}) {
		t.Fatalf("unexpected run history files_changed: %+v", row.FilesChanged)
	}
}

func persistPromotedMemory(t *testing.T, ctx context.Context, repo ContractRepository, projectID, pointerKey, subject, content string) core.ActiveMemory {
	t.Helper()

	result, err := repo.PersistProposedMemory(ctx, core.ProposeMemoryPersistence{
		ProjectID:           projectID,
		ReceiptID:           "receipt-memory-1234",
		Category:            "decision",
		Subject:             subject,
		Content:             content,
		Confidence:          4,
		Tags:                []string{"memory", "parity"},
		RelatedPointerKeys:  []string{pointerKey},
		EvidencePointerKeys: []string{pointerKey},
		DedupeKey:           projectID + ":" + pointerKey + ":" + subject,
		Validation: core.ProposeMemoryValidation{
			HardPassed: true,
			SoftPassed: true,
		},
		AutoPromote: true,
		Promotable:  true,
	})
	if err != nil {
		t.Fatalf("persist proposed memory: %v", err)
	}
	if result.PromotedMemoryID <= 0 {
		t.Fatalf("expected promoted memory id, got %+v", result)
	}

	activeMemory, err := repo.LookupMemoryByID(ctx, core.MemoryLookupQuery{
		ProjectID: projectID,
		MemoryID:  result.PromotedMemoryID,
	})
	if err != nil {
		t.Fatalf("lookup promoted memory: %v", err)
	}
	return activeMemory
}

func candidateKeys(rows []core.CandidatePointer) []string {
	keys := make([]string, 0, len(rows))
	for _, row := range rows {
		keys = append(keys, row.Key)
	}
	return keys
}
