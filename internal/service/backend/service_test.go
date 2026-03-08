package backend

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	bootstrapkit "github.com/bonztm/agent-context-manager/internal/bootstrap"
	"github.com/bonztm/agent-context-manager/internal/contracts/v1"
	"github.com/bonztm/agent-context-manager/internal/core"
	"gopkg.in/yaml.v3"
)

type fakeRepository struct {
	candidateResults       [][]core.CandidatePointer
	candidateErrors        []error
	hopResults             [][]core.HopPointer
	memoryResults          [][]core.ActiveMemory
	memoryErrors           []error
	inventoryResults       []core.PointerInventory
	inventoryErrors        []error
	scopeResults           []core.ReceiptScope
	scopeErrors            []error
	proposeResults         []core.ProposeMemoryPersistenceResult
	proposeErrors          []error
	syncResults            []core.SyncApplyResult
	syncErrors             []error
	upsertStubResults      []int
	upsertStubErrors       []error
	fetchLookupResults     []core.FetchLookup
	fetchLookupErrors      []error
	pointerLookupResults   []core.CandidatePointer
	pointerLookupErrors    []error
	memoryLookupResults    []core.ActiveMemory
	memoryLookupErrors     []error
	workUpsertResults      []int
	workUpsertErrors       []error
	workListResults        [][]core.WorkItem
	workListErrors         []error
	workPlanUpsertResult   []core.WorkPlanUpsertResult
	workPlanUpsertErrors   []error
	workPlanLookupResult   []core.WorkPlan
	workPlanLookupErrors   []error
	workPlanListResults    [][]core.WorkPlanSummary
	workPlanListErrors     []error
	memoryHistoryResults   [][]core.MemoryHistorySummary
	memoryHistoryErrors    []error
	receiptHistoryResults  [][]core.ReceiptHistorySummary
	receiptHistoryErrors   []error
	runHistoryResults      [][]core.RunHistorySummary
	runHistoryErrors       []error
	runHistoryLookup       []core.RunHistorySummary
	runHistoryLookupErrors []error
	verifySaveErrors       []error
	reviewAttemptResults   [][]core.ReviewAttempt
	reviewAttemptErrors    []error

	candidateCalls         []core.CandidatePointerQuery
	hopCalls               []core.RelatedHopPointersQuery
	memoryCalls            []core.ActiveMemoryQuery
	inventoryCalls         []string
	scopeCalls             []core.ReceiptScopeQuery
	receiptUpsertCalls     []core.ReceiptScope
	proposeCalls           []core.ProposeMemoryPersistence
	saveCalls              []core.RunReceiptSummary
	syncCalls              []core.SyncApplyInput
	upsertStubProjectIDs   []string
	upsertStubCalls        [][]core.PointerStub
	fetchLookupCalls       []core.FetchLookupQuery
	pointerLookupCalls     []core.PointerLookupQuery
	memoryLookupCalls      []core.MemoryLookupQuery
	workUpsertCalls        []core.WorkItemsUpsertInput
	workListCalls          []core.FetchLookupQuery
	workPlanUpsertCalls    []core.WorkPlanUpsertInput
	workPlanLookupCalls    []core.WorkPlanLookupQuery
	workPlanListCalls      []core.WorkPlanListQuery
	memoryHistoryCalls     []core.MemoryHistoryListQuery
	receiptHistoryCalls    []core.ReceiptHistoryListQuery
	runHistoryCalls        []core.RunHistoryListQuery
	runHistoryLookupCalls  []core.RunHistoryLookupQuery
	verifySaveCalls        []core.VerificationBatch
	reviewAttemptCalls     []core.ReviewAttempt
	reviewAttemptListCalls []core.ReviewAttemptListQuery

	saveResult         core.RunReceiptIDs
	saveError          error
	receiptUpsertError error
}

func (f *fakeRepository) FetchCandidatePointers(_ context.Context, input core.CandidatePointerQuery) ([]core.CandidatePointer, error) {
	f.candidateCalls = append(f.candidateCalls, input)
	idx := len(f.candidateCalls) - 1
	if idx < len(f.candidateErrors) && f.candidateErrors[idx] != nil {
		return nil, f.candidateErrors[idx]
	}
	if idx >= len(f.candidateResults) {
		return nil, nil
	}
	return append([]core.CandidatePointer(nil), f.candidateResults[idx]...), nil
}

func (f *fakeRepository) FetchRelatedHopPointers(_ context.Context, input core.RelatedHopPointersQuery) ([]core.HopPointer, error) {
	f.hopCalls = append(f.hopCalls, input)
	idx := len(f.hopCalls) - 1
	if idx >= len(f.hopResults) {
		return nil, nil
	}
	return append([]core.HopPointer(nil), f.hopResults[idx]...), nil
}

func (f *fakeRepository) FetchActiveMemories(_ context.Context, input core.ActiveMemoryQuery) ([]core.ActiveMemory, error) {
	f.memoryCalls = append(f.memoryCalls, input)
	idx := len(f.memoryCalls) - 1
	if idx < len(f.memoryErrors) && f.memoryErrors[idx] != nil {
		return nil, f.memoryErrors[idx]
	}
	if idx >= len(f.memoryResults) {
		return nil, nil
	}
	return append([]core.ActiveMemory(nil), f.memoryResults[idx]...), nil
}

func (f *fakeRepository) ListPointerInventory(_ context.Context, projectID string) ([]core.PointerInventory, error) {
	f.inventoryCalls = append(f.inventoryCalls, strings.TrimSpace(projectID))
	idx := len(f.inventoryCalls) - 1
	if idx < len(f.inventoryErrors) && f.inventoryErrors[idx] != nil {
		return nil, f.inventoryErrors[idx]
	}
	if len(f.inventoryResults) == 0 {
		return nil, nil
	}
	return append([]core.PointerInventory(nil), f.inventoryResults...), nil
}

func (f *fakeRepository) UpsertPointerStubs(_ context.Context, projectID string, stubs []core.PointerStub) (int, error) {
	f.upsertStubProjectIDs = append(f.upsertStubProjectIDs, strings.TrimSpace(projectID))
	copied := make([]core.PointerStub, 0, len(stubs))
	for _, stub := range stubs {
		copied = append(copied, core.PointerStub{
			PointerKey:  stub.PointerKey,
			Path:        stub.Path,
			Kind:        stub.Kind,
			Label:       stub.Label,
			Description: stub.Description,
			Tags:        append([]string(nil), stub.Tags...),
		})
	}
	f.upsertStubCalls = append(f.upsertStubCalls, copied)

	idx := len(f.upsertStubCalls) - 1
	if idx < len(f.upsertStubErrors) && f.upsertStubErrors[idx] != nil {
		return 0, f.upsertStubErrors[idx]
	}
	if idx < len(f.upsertStubResults) {
		return f.upsertStubResults[idx], nil
	}
	return len(copied), nil
}

func (f *fakeRepository) FetchReceiptScope(_ context.Context, input core.ReceiptScopeQuery) (core.ReceiptScope, error) {
	f.scopeCalls = append(f.scopeCalls, input)
	idx := len(f.scopeCalls) - 1
	if idx < len(f.scopeErrors) && f.scopeErrors[idx] != nil {
		return core.ReceiptScope{}, f.scopeErrors[idx]
	}
	if idx >= len(f.scopeResults) {
		return core.ReceiptScope{}, core.ErrReceiptScopeNotFound
	}
	scope := f.scopeResults[idx]
	scope.ResolvedTags = append([]string(nil), scope.ResolvedTags...)
	scope.PointerKeys = append([]string(nil), scope.PointerKeys...)
	scope.MemoryIDs = append([]int64(nil), scope.MemoryIDs...)
	scope.PointerPaths = append([]string(nil), scope.PointerPaths...)
	return scope, nil
}

func (f *fakeRepository) UpsertReceiptScope(_ context.Context, input core.ReceiptScope) error {
	f.receiptUpsertCalls = append(f.receiptUpsertCalls, core.ReceiptScope{
		ProjectID:    strings.TrimSpace(input.ProjectID),
		ReceiptID:    strings.TrimSpace(input.ReceiptID),
		TaskText:     strings.TrimSpace(input.TaskText),
		Phase:        strings.TrimSpace(input.Phase),
		ResolvedTags: append([]string(nil), input.ResolvedTags...),
		PointerKeys:  append([]string(nil), input.PointerKeys...),
		MemoryIDs:    append([]int64(nil), input.MemoryIDs...),
	})
	return f.receiptUpsertError
}

func (f *fakeRepository) LookupFetchState(_ context.Context, input core.FetchLookupQuery) (core.FetchLookup, error) {
	f.fetchLookupCalls = append(f.fetchLookupCalls, input)
	idx := len(f.fetchLookupCalls) - 1
	if idx < len(f.fetchLookupErrors) && f.fetchLookupErrors[idx] != nil {
		return core.FetchLookup{}, f.fetchLookupErrors[idx]
	}
	if idx >= len(f.fetchLookupResults) {
		return core.FetchLookup{}, core.ErrFetchLookupNotFound
	}

	lookup := f.fetchLookupResults[idx]
	lookup.ProjectID = strings.TrimSpace(lookup.ProjectID)
	lookup.ReceiptID = strings.TrimSpace(lookup.ReceiptID)
	lookup.RunStatus = strings.TrimSpace(lookup.RunStatus)
	lookup.WorkItems = append([]core.WorkItem(nil), lookup.WorkItems...)
	return lookup, nil
}

func (f *fakeRepository) LookupPointerByKey(_ context.Context, input core.PointerLookupQuery) (core.CandidatePointer, error) {
	f.pointerLookupCalls = append(f.pointerLookupCalls, core.PointerLookupQuery{
		ProjectID:  strings.TrimSpace(input.ProjectID),
		PointerKey: strings.TrimSpace(input.PointerKey),
	})

	idx := len(f.pointerLookupCalls) - 1
	if idx < len(f.pointerLookupErrors) && f.pointerLookupErrors[idx] != nil {
		return core.CandidatePointer{}, f.pointerLookupErrors[idx]
	}
	if idx >= len(f.pointerLookupResults) {
		return core.CandidatePointer{}, core.ErrPointerLookupNotFound
	}
	return f.pointerLookupResults[idx], nil
}

func (f *fakeRepository) LookupMemoryByID(_ context.Context, input core.MemoryLookupQuery) (core.ActiveMemory, error) {
	f.memoryLookupCalls = append(f.memoryLookupCalls, core.MemoryLookupQuery{
		ProjectID: strings.TrimSpace(input.ProjectID),
		MemoryID:  input.MemoryID,
	})

	idx := len(f.memoryLookupCalls) - 1
	if idx < len(f.memoryLookupErrors) && f.memoryLookupErrors[idx] != nil {
		return core.ActiveMemory{}, f.memoryLookupErrors[idx]
	}
	if idx >= len(f.memoryLookupResults) {
		return core.ActiveMemory{}, core.ErrMemoryLookupNotFound
	}
	return f.memoryLookupResults[idx], nil
}

func (f *fakeRepository) SaveReviewAttempt(_ context.Context, input core.ReviewAttempt) (int64, error) {
	f.reviewAttemptCalls = append(f.reviewAttemptCalls, input)
	idx := len(f.reviewAttemptCalls) - 1
	if idx < len(f.reviewAttemptErrors) && f.reviewAttemptErrors[idx] != nil {
		return 0, f.reviewAttemptErrors[idx]
	}
	return int64(idx + 1), nil
}

func (f *fakeRepository) ListReviewAttempts(_ context.Context, input core.ReviewAttemptListQuery) ([]core.ReviewAttempt, error) {
	f.reviewAttemptListCalls = append(f.reviewAttemptListCalls, core.ReviewAttemptListQuery{
		ProjectID: strings.TrimSpace(input.ProjectID),
		ReceiptID: strings.TrimSpace(input.ReceiptID),
		ReviewKey: strings.TrimSpace(input.ReviewKey),
	})
	idx := len(f.reviewAttemptListCalls) - 1
	if idx < len(f.reviewAttemptErrors) && f.reviewAttemptErrors[idx] != nil {
		return nil, f.reviewAttemptErrors[idx]
	}
	if idx < len(f.reviewAttemptResults) {
		return append([]core.ReviewAttempt(nil), f.reviewAttemptResults[idx]...), nil
	}
	if len(f.reviewAttemptCalls) == 0 {
		return nil, nil
	}
	out := make([]core.ReviewAttempt, 0, len(f.reviewAttemptCalls))
	for i, attempt := range f.reviewAttemptCalls {
		copied := attempt
		if copied.AttemptID == 0 {
			copied.AttemptID = int64(i + 1)
		}
		out = append(out, copied)
	}
	return out, nil
}

func (f *fakeRepository) UpsertWorkItems(_ context.Context, input core.WorkItemsUpsertInput) (int, error) {
	f.workUpsertCalls = append(f.workUpsertCalls, core.WorkItemsUpsertInput{
		ProjectID: strings.TrimSpace(input.ProjectID),
		ReceiptID: strings.TrimSpace(input.ReceiptID),
		Items:     append([]core.WorkItem(nil), input.Items...),
	})

	idx := len(f.workUpsertCalls) - 1
	if idx < len(f.workUpsertErrors) && f.workUpsertErrors[idx] != nil {
		return 0, f.workUpsertErrors[idx]
	}
	if idx < len(f.workUpsertResults) {
		return f.workUpsertResults[idx], nil
	}
	return len(input.Items), nil
}

func (f *fakeRepository) ListWorkItems(_ context.Context, input core.FetchLookupQuery) ([]core.WorkItem, error) {
	f.workListCalls = append(f.workListCalls, core.FetchLookupQuery{
		ProjectID: strings.TrimSpace(input.ProjectID),
		ReceiptID: strings.TrimSpace(input.ReceiptID),
	})

	idx := len(f.workListCalls) - 1
	if idx < len(f.workListErrors) && f.workListErrors[idx] != nil {
		return nil, f.workListErrors[idx]
	}
	if idx >= len(f.workListResults) {
		return nil, nil
	}
	return append([]core.WorkItem(nil), f.workListResults[idx]...), nil
}

func (f *fakeRepository) UpsertWorkPlan(_ context.Context, input core.WorkPlanUpsertInput) (core.WorkPlanUpsertResult, error) {
	f.workPlanUpsertCalls = append(f.workPlanUpsertCalls, input)
	idx := len(f.workPlanUpsertCalls) - 1
	if idx < len(f.workPlanUpsertErrors) && f.workPlanUpsertErrors[idx] != nil {
		return core.WorkPlanUpsertResult{}, f.workPlanUpsertErrors[idx]
	}
	if idx < len(f.workPlanUpsertResult) {
		return f.workPlanUpsertResult[idx], nil
	}

	plan := core.WorkPlan{
		ProjectID:     input.ProjectID,
		PlanKey:       input.PlanKey,
		ReceiptID:     input.ReceiptID,
		Title:         input.Title,
		Objective:     input.Objective,
		Kind:          input.Kind,
		ParentPlanKey: input.ParentPlanKey,
		Status:        input.Status,
		Stages:        input.Stages,
		InScope:       append([]string(nil), input.InScope...),
		OutOfScope:    append([]string(nil), input.OutOfScope...),
		Constraints:   append([]string(nil), input.Constraints...),
		References:    append([]string(nil), input.References...),
		ExternalRefs:  append([]string(nil), input.ExternalRefs...),
		Tasks:         append([]core.WorkItem(nil), input.Tasks...),
	}
	if strings.TrimSpace(plan.Status) == "" {
		plan.Status = core.PlanStatusPending
	}
	return core.WorkPlanUpsertResult{
		Plan:    plan,
		Updated: len(input.Tasks),
	}, nil
}

func (f *fakeRepository) LookupWorkPlan(_ context.Context, input core.WorkPlanLookupQuery) (core.WorkPlan, error) {
	f.workPlanLookupCalls = append(f.workPlanLookupCalls, input)
	idx := len(f.workPlanLookupCalls) - 1
	if idx < len(f.workPlanLookupErrors) && f.workPlanLookupErrors[idx] != nil {
		return core.WorkPlan{}, f.workPlanLookupErrors[idx]
	}
	if idx >= len(f.workPlanLookupResult) {
		return core.WorkPlan{}, core.ErrWorkPlanNotFound
	}
	return f.workPlanLookupResult[idx], nil
}

func (f *fakeRepository) ListWorkPlans(_ context.Context, input core.WorkPlanListQuery) ([]core.WorkPlanSummary, error) {
	f.workPlanListCalls = append(f.workPlanListCalls, input)
	idx := len(f.workPlanListCalls) - 1
	if idx < len(f.workPlanListErrors) && f.workPlanListErrors[idx] != nil {
		return nil, f.workPlanListErrors[idx]
	}
	if idx >= len(f.workPlanListResults) {
		return nil, nil
	}
	return append([]core.WorkPlanSummary(nil), f.workPlanListResults[idx]...), nil
}

func (f *fakeRepository) ListReceiptHistory(_ context.Context, input core.ReceiptHistoryListQuery) ([]core.ReceiptHistorySummary, error) {
	f.receiptHistoryCalls = append(f.receiptHistoryCalls, input)
	idx := len(f.receiptHistoryCalls) - 1
	if idx < len(f.receiptHistoryErrors) && f.receiptHistoryErrors[idx] != nil {
		return nil, f.receiptHistoryErrors[idx]
	}
	if idx >= len(f.receiptHistoryResults) {
		return nil, nil
	}
	return append([]core.ReceiptHistorySummary(nil), f.receiptHistoryResults[idx]...), nil
}

func (f *fakeRepository) ListMemoryHistory(_ context.Context, input core.MemoryHistoryListQuery) ([]core.MemoryHistorySummary, error) {
	f.memoryHistoryCalls = append(f.memoryHistoryCalls, input)
	idx := len(f.memoryHistoryCalls) - 1
	if idx < len(f.memoryHistoryErrors) && f.memoryHistoryErrors[idx] != nil {
		return nil, f.memoryHistoryErrors[idx]
	}
	if idx >= len(f.memoryHistoryResults) {
		return nil, nil
	}
	return append([]core.MemoryHistorySummary(nil), f.memoryHistoryResults[idx]...), nil
}

func (f *fakeRepository) ListRunHistory(_ context.Context, input core.RunHistoryListQuery) ([]core.RunHistorySummary, error) {
	f.runHistoryCalls = append(f.runHistoryCalls, input)
	idx := len(f.runHistoryCalls) - 1
	if idx < len(f.runHistoryErrors) && f.runHistoryErrors[idx] != nil {
		return nil, f.runHistoryErrors[idx]
	}
	if idx >= len(f.runHistoryResults) {
		return nil, nil
	}
	return append([]core.RunHistorySummary(nil), f.runHistoryResults[idx]...), nil
}

func (f *fakeRepository) LookupRunHistory(_ context.Context, input core.RunHistoryLookupQuery) (core.RunHistorySummary, error) {
	f.runHistoryLookupCalls = append(f.runHistoryLookupCalls, input)
	idx := len(f.runHistoryLookupCalls) - 1
	if idx < len(f.runHistoryLookupErrors) && f.runHistoryLookupErrors[idx] != nil {
		return core.RunHistorySummary{}, f.runHistoryLookupErrors[idx]
	}
	if idx >= len(f.runHistoryLookup) {
		return core.RunHistorySummary{}, core.ErrFetchLookupNotFound
	}
	return f.runHistoryLookup[idx], nil
}

func (f *fakeRepository) PersistProposedMemory(_ context.Context, input core.ProposeMemoryPersistence) (core.ProposeMemoryPersistenceResult, error) {
	f.proposeCalls = append(f.proposeCalls, input)
	idx := len(f.proposeCalls) - 1
	if idx < len(f.proposeErrors) && f.proposeErrors[idx] != nil {
		return core.ProposeMemoryPersistenceResult{}, f.proposeErrors[idx]
	}
	if idx < len(f.proposeResults) {
		result := f.proposeResults[idx]
		if result.CandidateID == 0 {
			result.CandidateID = int64(idx + 1)
		}
		return result, nil
	}

	result := core.ProposeMemoryPersistenceResult{
		CandidateID: int64(idx + 1),
		Status:      "pending",
	}
	if !input.Validation.HardPassed {
		result.Status = "rejected"
		return result, nil
	}
	if input.Promotable {
		result.Status = "promoted"
		result.PromotedMemoryID = int64(100 + idx)
	}
	return result, nil
}

func (f *fakeRepository) SaveRunReceiptSummary(_ context.Context, input core.RunReceiptSummary) (core.RunReceiptIDs, error) {
	f.saveCalls = append(f.saveCalls, input)
	if f.saveError != nil {
		return core.RunReceiptIDs{}, f.saveError
	}
	if f.saveResult.RunID == 0 {
		return core.RunReceiptIDs{RunID: 1, ReceiptID: input.ReceiptID}, nil
	}
	return f.saveResult, nil
}

func (f *fakeRepository) SaveVerificationBatch(_ context.Context, input core.VerificationBatch) error {
	copied := core.VerificationBatch{
		BatchRunID:      strings.TrimSpace(input.BatchRunID),
		ProjectID:       strings.TrimSpace(input.ProjectID),
		ReceiptID:       strings.TrimSpace(input.ReceiptID),
		PlanKey:         strings.TrimSpace(input.PlanKey),
		Phase:           strings.TrimSpace(input.Phase),
		TestsSourcePath: strings.TrimSpace(input.TestsSourcePath),
		Status:          strings.TrimSpace(input.Status),
		Passed:          input.Passed,
		SelectedTestIDs: append([]string(nil), input.SelectedTestIDs...),
		CreatedAt:       input.CreatedAt,
	}
	copied.Results = make([]core.VerificationTestRun, 0, len(input.Results))
	for _, result := range input.Results {
		copied.Results = append(copied.Results, core.VerificationTestRun{
			BatchRunID:       strings.TrimSpace(result.BatchRunID),
			ProjectID:        strings.TrimSpace(result.ProjectID),
			TestID:           strings.TrimSpace(result.TestID),
			DefinitionHash:   strings.TrimSpace(result.DefinitionHash),
			Summary:          strings.TrimSpace(result.Summary),
			CommandArgv:      append([]string(nil), result.CommandArgv...),
			CommandCWD:       strings.TrimSpace(result.CommandCWD),
			TimeoutSec:       result.TimeoutSec,
			ExpectedExitCode: result.ExpectedExitCode,
			SelectionReasons: append([]string(nil), result.SelectionReasons...),
			Status:           strings.TrimSpace(result.Status),
			ExitCode:         result.ExitCode,
			DurationMS:       result.DurationMS,
			StdoutExcerpt:    strings.TrimSpace(result.StdoutExcerpt),
			StderrExcerpt:    strings.TrimSpace(result.StderrExcerpt),
			StartedAt:        result.StartedAt,
			FinishedAt:       result.FinishedAt,
		})
	}
	f.verifySaveCalls = append(f.verifySaveCalls, copied)
	idx := len(f.verifySaveCalls) - 1
	if idx < len(f.verifySaveErrors) && f.verifySaveErrors[idx] != nil {
		return f.verifySaveErrors[idx]
	}
	return nil
}

func (f *fakeRepository) ApplySync(_ context.Context, input core.SyncApplyInput) (core.SyncApplyResult, error) {
	f.syncCalls = append(f.syncCalls, input)
	idx := len(f.syncCalls) - 1
	if idx < len(f.syncErrors) && f.syncErrors[idx] != nil {
		return core.SyncApplyResult{}, f.syncErrors[idx]
	}
	if idx < len(f.syncResults) {
		return f.syncResults[idx], nil
	}
	return core.SyncApplyResult{}, nil
}

func TestGetContext_NormalPathReturnsOKAndReceipt(t *testing.T) {
	repo := &fakeRepository{
		candidateResults: [][]core.CandidatePointer{{
			candidate("rule:startup", "AGENTS.md", true, []string{"governance"}),
			candidate("code:service", "internal/service/backend/get_context.go", false, []string{"backend"}),
			candidate("doc:spec", "spec/v1/README.md", false, []string{"docs"}),
		}},
		hopResults: [][]core.HopPointer{{
			hop("code:service", 1, candidate("test:get-context", "internal/service/backend/service_test.go", false, []string{"tests"})),
		}},
		memoryResults: [][]core.ActiveMemory{{
			memory(101, "Default caps behavior", "schema defaults apply when caps omitted", []string{"backend"}, []string{"code:service"}),
		}},
	}
	svc, err := New(repo)
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	payload := v1.GetContextPayload{
		ProjectID: "project.alpha",
		TaskText:  "implement deterministic get context flow",
		Phase:     v1.PhaseExecute,
	}

	result, apiErr := svc.GetContext(context.Background(), payload)
	if apiErr != nil {
		t.Fatalf("unexpected API error: %+v", apiErr)
	}
	if result.Status != "ok" {
		t.Fatalf("unexpected status: %q", result.Status)
	}
	if result.Receipt == nil {
		t.Fatal("expected receipt")
	}
	if result.Receipt.Meta.RetrievalVersion != RetrievalVersion {
		t.Fatalf("unexpected retrieval version: %q", result.Receipt.Meta.RetrievalVersion)
	}
	if result.Receipt.Meta.ReceiptID == "" {
		t.Fatal("expected non-empty receipt_id")
	}
	if result.Receipt.Meta.Budget.Unit != "words" {
		t.Fatalf("unexpected budget unit: %q", result.Receipt.Meta.Budget.Unit)
	}

	rules := receiptIndexEntries(result.Receipt, "rules")
	if got, want := receiptIndexKeys(rules), []string{"rule:startup"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected rule index keys: got %v want %v", got, want)
	}
	if len(rules) != 1 {
		t.Fatalf("expected one rule entry, got %d", len(rules))
	}
	if got, want := entryString(rules[0], "rule_id"), entryString(rules[0], "key"); got != want {
		t.Fatalf("unexpected stable rule_id: got %q want %q", got, want)
	}
	suggestions := receiptIndexEntries(result.Receipt, "suggestions")
	if got, want := receiptIndexKeys(suggestions), []string{"code:service", "doc:spec", "test:get-context"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected suggestion index keys: got %v want %v", got, want)
	}
	memories := receiptIndexEntries(result.Receipt, "memories")
	if len(memories) != 1 {
		t.Fatalf("unexpected memory index count: got %d want 1", len(memories))
	}
	if got := entryString(memories[0], "key"); got != "mem:101" {
		t.Fatalf("unexpected memory key: got %q want %q", got, "mem:101")
	}
	if got := strings.TrimSpace(entryString(memories[0], "summary")); got == "" {
		t.Fatalf("expected non-empty memory summary entry: %+v", memories[0])
	}
	plans := receiptIndexEntries(result.Receipt, "plans")
	if len(plans) != 1 {
		t.Fatalf("unexpected plan index count: got %d want 1", len(plans))
	}
	if got := entryString(plans[0], "key"); got != "plan:"+result.Receipt.Meta.ReceiptID {
		t.Fatalf("unexpected plan key: got %q want %q", got, "plan:"+result.Receipt.Meta.ReceiptID)
	}
	if got := entryString(plans[0], "status"); got != core.PlanStatusPending {
		t.Fatalf("unexpected plan status: got %q want %q", got, core.PlanStatusPending)
	}
	if got := strings.TrimSpace(entryString(plans[0], "summary")); got == "" {
		t.Fatalf("expected non-empty plan summary entry: %+v", plans[0])
	}

	meta := receiptMeta(result.Receipt)
	if got := strings.TrimSpace(anyToString(meta["receipt_id"])); got != result.Receipt.Meta.ReceiptID {
		t.Fatalf("unexpected receipt meta receipt_id: got %q want %q", got, result.Receipt.Meta.ReceiptID)
	}

	if result.Diagnostics == nil {
		t.Fatal("expected diagnostics")
	}
	if result.Diagnostics.InitialPointerCount != 4 {
		t.Fatalf("unexpected initial pointer count: got %d want 4", result.Diagnostics.InitialPointerCount)
	}
	if result.Diagnostics.FallbackUsed {
		t.Fatal("did not expect fallback")
	}
	if len(repo.candidateCalls) != 1 {
		t.Fatalf("expected 1 candidate query, got %d", len(repo.candidateCalls))
	}
	if len(repo.memoryCalls) != 1 {
		t.Fatalf("expected 1 memory query, got %d", len(repo.memoryCalls))
	}
	if len(repo.receiptUpsertCalls) != 1 {
		t.Fatalf("expected 1 receipt scope upsert, got %d", len(repo.receiptUpsertCalls))
	}
	if got := repo.receiptUpsertCalls[0].ReceiptID; got != result.Receipt.Meta.ReceiptID {
		t.Fatalf("unexpected persisted receipt_id: got %q want %q", got, result.Receipt.Meta.ReceiptID)
	}
	if got := repo.receiptUpsertCalls[0].ProjectID; got != payload.ProjectID {
		t.Fatalf("unexpected persisted project_id: got %q want %q", got, payload.ProjectID)
	}
	if got := repo.receiptUpsertCalls[0].Phase; got != string(payload.Phase) {
		t.Fatalf("unexpected persisted phase: got %q want %q", got, payload.Phase)
	}
	if got := repo.receiptUpsertCalls[0].TaskText; got != payload.TaskText {
		t.Fatalf("unexpected persisted task_text: got %q want %q", got, payload.TaskText)
	}

	repo2 := &fakeRepository{
		candidateResults: repo.candidateResults,
		hopResults:       repo.hopResults,
		memoryResults:    repo.memoryResults,
	}
	svc2, err := New(repo2)
	if err != nil {
		t.Fatalf("new service 2: %v", err)
	}
	result2, apiErr2 := svc2.GetContext(context.Background(), payload)
	if apiErr2 != nil {
		t.Fatalf("unexpected API error on second run: %+v", apiErr2)
	}
	if result2.Receipt == nil {
		t.Fatal("expected second receipt")
	}
	if result2.Receipt.Meta.ReceiptID != result.Receipt.Meta.ReceiptID {
		t.Fatalf("expected deterministic receipt_id, got %q and %q", result.Receipt.Meta.ReceiptID, result2.Receipt.Meta.ReceiptID)
	}
}

func TestGetContext_FiltersManagedProjectPointersFromRetrieval(t *testing.T) {
	repo := &fakeRepository{
		candidateResults: [][]core.CandidatePointer{{
			candidate("rule:startup", "AGENTS.md", true, []string{"governance"}),
			candidate("managed:gitignore", ".gitignore", false, []string{"config"}),
			candidate("managed:dbwal", ".acm/context.db-wal", false, []string{"config"}),
			candidate("code:service", "internal/service/backend/get_context.go", false, []string{"backend"}),
		}},
		hopResults: [][]core.HopPointer{{
			hop("code:service", 1, candidate("managed:tests", ".acm/acm-tests.yaml", false, []string{"config"})),
			hop("code:service", 1, candidate("test:get-context", "internal/service/backend/service_test.go", false, []string{"tests"})),
		}},
		memoryResults: [][]core.ActiveMemory{{}},
	}
	svc, err := New(repo)
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	result, apiErr := svc.GetContext(context.Background(), v1.GetContextPayload{
		ProjectID: "project.alpha",
		TaskText:  "retrieve useful implementation context",
		Phase:     v1.PhaseExecute,
		Caps: &v1.RetrievalCaps{
			MinPointerCount: 1,
		},
	})
	if apiErr != nil {
		t.Fatalf("unexpected API error: %+v", apiErr)
	}
	if result.Status != "ok" || result.Receipt == nil {
		t.Fatalf("unexpected result: %+v", result)
	}

	wantKeys := []string{"code:service", "rule:startup", "test:get-context"}
	if got := pointerKeys(result.Receipt); !reflect.DeepEqual(got, wantKeys) {
		t.Fatalf("unexpected pointer keys: got %v want %v", got, wantKeys)
	}
	if result.Diagnostics == nil || result.Diagnostics.InitialPointerCount != 3 {
		t.Fatalf("unexpected diagnostics: %+v", result.Diagnostics)
	}
	if len(repo.receiptUpsertCalls) != 1 {
		t.Fatalf("expected one receipt scope upsert, got %d", len(repo.receiptUpsertCalls))
	}
	wantPersistedKeys := []string{"rule:startup", "code:service", "test:get-context"}
	if got := repo.receiptUpsertCalls[0].PointerKeys; !reflect.DeepEqual(got, wantPersistedKeys) {
		t.Fatalf("unexpected persisted pointer keys: got %v want %v", got, wantPersistedKeys)
	}
}

func TestGetContext_AllowsCanonicalRulesFromManagedPaths(t *testing.T) {
	repo := &fakeRepository{
		candidateResults: [][]core.CandidatePointer{{
			candidate("rule:hard", ".acm/acm-rules.yaml", true, []string{"governance", "enforcement-hard"}),
			candidate("rule:root", "acm-rules.yaml", true, []string{"governance", "enforcement-soft"}),
			candidate("managed:tests", ".acm/acm-tests.yaml", false, []string{"config"}),
			candidate("managed:tags", ".acm/acm-tags.yaml", false, []string{"config"}),
			candidate("code:service", "internal/service/backend/get_context.go", false, []string{"backend"}),
		}},
		hopResults: [][]core.HopPointer{{
			hop("code:service", 1, candidate("rule:hop", ".acm/acm-rules.yaml", true, []string{"governance", "enforcement-hard"})),
			hop("code:service", 1, candidate("managed:dbwal", ".acm/context.db-wal", false, []string{"config"})),
		}},
		memoryResults: [][]core.ActiveMemory{{}},
	}
	svc, err := New(repo)
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	result, apiErr := svc.GetContext(context.Background(), v1.GetContextPayload{
		ProjectID: "project.alpha",
		TaskText:  "enforce repo hard rules",
		Phase:     v1.PhaseExecute,
		Caps: &v1.RetrievalCaps{
			MinPointerCount: 1,
		},
	})
	if apiErr != nil {
		t.Fatalf("unexpected API error: %+v", apiErr)
	}
	if result.Status != "ok" || result.Receipt == nil {
		t.Fatalf("unexpected result: %+v", result)
	}

	if got, want := receiptIndexKeys(receiptIndexEntries(result.Receipt, "rules")), []string{"rule:hard", "rule:hop", "rule:root"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected rule keys: got %v want %v", got, want)
	}
	if got, want := receiptIndexKeys(receiptIndexEntries(result.Receipt, "suggestions")), []string{"code:service"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected suggestion keys: got %v want %v", got, want)
	}
	if result.Diagnostics == nil || result.Diagnostics.InitialPointerCount != 4 {
		t.Fatalf("unexpected diagnostics: %+v", result.Diagnostics)
	}
}

func TestGetContext_MaxRulePointersZeroMeansUncapped(t *testing.T) {
	repo := &fakeRepository{
		candidateResults: [][]core.CandidatePointer{{
			candidate("rule:alpha", ".acm/acm-rules.yaml", true, []string{"governance"}),
			candidate("rule:beta", ".acm/acm-rules.yaml", true, []string{"governance"}),
			candidate("code:service", "internal/service/backend/get_context.go", false, []string{"backend"}),
		}},
		memoryResults: [][]core.ActiveMemory{{}},
	}
	svc, err := New(repo)
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	result, apiErr := svc.GetContext(context.Background(), v1.GetContextPayload{
		ProjectID: "project.alpha",
		TaskText:  "validate uncapped rule defaults",
		Phase:     v1.PhaseExecute,
		Caps: &v1.RetrievalCaps{
			MaxRulePointers: 0,
			MinPointerCount: 1,
		},
	})
	if apiErr != nil {
		t.Fatalf("unexpected API error: %+v", apiErr)
	}
	if result.Status != "ok" || result.Receipt == nil {
		t.Fatalf("unexpected result: %+v", result)
	}

	if got, want := receiptIndexKeys(receiptIndexEntries(result.Receipt, "rules")), []string{"rule:alpha", "rule:beta"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected rule keys for max_rule_pointers=0: got %v want %v", got, want)
	}
}

func TestGetContext_InsufficientContextAfterFallback(t *testing.T) {
	repo := &fakeRepository{
		candidateResults: [][]core.CandidatePointer{
			{candidate("rule:one", "AGENTS.md", true, []string{"governance"})},
			{candidate("rule:two", "AGENTS.md", true, []string{"governance"})},
		},
	}
	svc, err := New(repo)
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	payload := v1.GetContextPayload{
		ProjectID: "project.alpha",
		TaskText:  "narrow query that should trigger fallback",
		Phase:     v1.PhasePlan,
	}

	result, apiErr := svc.GetContext(context.Background(), payload)
	if apiErr != nil {
		t.Fatalf("unexpected API error: %+v", apiErr)
	}
	if result.Status != "insufficient_context" {
		t.Fatalf("unexpected status: %q", result.Status)
	}
	if result.Receipt != nil {
		t.Fatal("did not expect receipt")
	}
	if result.Diagnostics == nil {
		t.Fatal("expected diagnostics")
	}
	if result.Diagnostics.InitialPointerCount != 1 {
		t.Fatalf("unexpected initial pointer count: got %d want 1", result.Diagnostics.InitialPointerCount)
	}
	if !result.Diagnostics.FallbackUsed {
		t.Fatal("expected fallback to be used")
	}
	if result.Diagnostics.FallbackMode != fallbackWidenOnce {
		t.Fatalf("unexpected fallback mode: %q", result.Diagnostics.FallbackMode)
	}
	if got := len(repo.candidateCalls); got != 2 {
		t.Fatalf("expected 2 candidate queries, got %d", got)
	}
	if repo.candidateCalls[1].TaskText != payload.TaskText {
		t.Fatalf("expected fallback task text to remain for FTS-only widening, got %q", repo.candidateCalls[1].TaskText)
	}
	if len(repo.candidateCalls[1].Tags) != 0 {
		t.Fatalf("expected fallback query tags to be cleared, got %v", repo.candidateCalls[1].Tags)
	}
	if repo.candidateCalls[1].StaleFilter.AllowStale != payload.AllowStale {
		t.Fatalf("expected fallback stale policy to match payload allow_stale=%v, got %v", payload.AllowStale, repo.candidateCalls[1].StaleFilter.AllowStale)
	}
	if len(repo.memoryCalls) != 0 {
		t.Fatalf("did not expect memory query on insufficient context, got %d", len(repo.memoryCalls))
	}
}

func TestGetContext_CapsRulesNonRulesAndMemories(t *testing.T) {
	repo := &fakeRepository{
		candidateResults: [][]core.CandidatePointer{{
			candidate("rule:a", "AGENTS.md", true, []string{"rules"}),
			candidate("rule:b", "CLAUDE.md", true, []string{"rules"}),
			candidate("code:1", "internal/a.go", false, []string{"backend"}),
			candidate("code:2", "internal/b.go", false, []string{"backend"}),
			candidate("doc:1", "README.md", false, []string{"docs"}),
		}},
		hopResults: [][]core.HopPointer{{
			hop("code:1", 1, candidate("test:1", "internal/a_test.go", false, []string{"tests"})),
			hop("code:1", 1, candidate("test:2", "internal/b_test.go", false, []string{"tests"})),
		}},
		memoryResults: [][]core.ActiveMemory{{
			memory(7, "One", "first", []string{"backend"}, []string{"code:1"}),
			memory(8, "Two", "second", []string{"docs"}, []string{"doc:1"}),
		}},
	}
	svc, err := New(repo)
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	payload := v1.GetContextPayload{
		ProjectID: "project.alpha",
		TaskText:  "validate cap handling",
		Phase:     v1.PhaseExecute,
		Caps: &v1.RetrievalCaps{
			MaxNonRulePointers: 1,
			MaxHopExpansion:    1,
			MaxMemories:        1,
			MinPointerCount:    1,
		},
	}

	result, apiErr := svc.GetContext(context.Background(), payload)
	if apiErr != nil {
		t.Fatalf("unexpected API error: %+v", apiErr)
	}
	if result.Status != "ok" {
		t.Fatalf("unexpected status: %q", result.Status)
	}
	if result.Receipt == nil {
		t.Fatal("expected receipt")
	}

	if got, want := receiptIndexKeys(receiptIndexEntries(result.Receipt, "rules")), []string{"rule:a", "rule:b"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected rule index keys: got %v want %v", got, want)
	}
	if got, want := receiptIndexKeys(receiptIndexEntries(result.Receipt, "suggestions")), []string{"code:1", "test:1"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected suggestion index keys: got %v want %v", got, want)
	}
	memoryEntries := receiptIndexEntries(result.Receipt, "memories")
	if len(memoryEntries) != 1 {
		t.Fatalf("unexpected memory index count: got %d want 1", len(memoryEntries))
	}
	if got := entryString(memoryEntries[0], "key"); got != "mem:7" {
		t.Fatalf("unexpected memory key: got %q want %q", got, "mem:7")
	}
	if got := strings.TrimSpace(entryString(memoryEntries[0], "summary")); got == "" {
		t.Fatalf("expected non-empty memory summary entry: %+v", memoryEntries[0])
	}
	planEntries := receiptIndexEntries(result.Receipt, "plans")
	if len(planEntries) != 1 {
		t.Fatalf("expected one plan entry, got %d", len(planEntries))
	}
	if got := entryString(planEntries[0], "status"); got != core.PlanStatusPending {
		t.Fatalf("unexpected plan status: got %q want %q", got, core.PlanStatusPending)
	}

	if len(repo.hopCalls) != 1 {
		t.Fatalf("expected one hop query, got %d", len(repo.hopCalls))
	}
	if !reflect.DeepEqual(repo.hopCalls[0].PointerKeys, []string{"code:1"}) {
		t.Fatalf("unexpected hop pointer keys: %v", repo.hopCalls[0].PointerKeys)
	}
	if len(repo.memoryCalls) != 1 {
		t.Fatalf("expected one memory query, got %d", len(repo.memoryCalls))
	}
	if repo.memoryCalls[0].Limit != 1 {
		t.Fatalf("expected memory query limit 1, got %d", repo.memoryCalls[0].Limit)
	}
}

func TestGetContext_PhaseAndCanonicalTagsThreadedToRetrieval(t *testing.T) {
	repo := &fakeRepository{
		candidateResults: [][]core.CandidatePointer{{
			candidate("rule:a", "AGENTS.md", true, []string{"Rules"}),
			candidate("code:svc", "internal/service/backend/get_context.go", false, []string{"API"}),
		}},
		memoryResults: [][]core.ActiveMemory{{}},
	}
	svc, err := New(repo)
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	result, apiErr := svc.GetContext(context.Background(), v1.GetContextPayload{
		ProjectID: "project.alpha",
		TaskText:  "policy API tests for review",
		Phase:     v1.PhaseReview,
		Caps: &v1.RetrievalCaps{
			MinPointerCount: 1,
		},
	})
	if apiErr != nil {
		t.Fatalf("unexpected API error: %+v", apiErr)
	}
	if result.Status != "ok" || result.Receipt == nil {
		t.Fatalf("unexpected result: %+v", result)
	}
	if len(repo.candidateCalls) != 1 {
		t.Fatalf("expected one candidate call, got %d", len(repo.candidateCalls))
	}
	if repo.candidateCalls[0].Phase != string(v1.PhaseReview) {
		t.Fatalf("expected review phase in candidate query, got %q", repo.candidateCalls[0].Phase)
	}
	wantQueryTags := []string{"backend", "governance", "review", "test"}
	if !reflect.DeepEqual(repo.candidateCalls[0].Tags, wantQueryTags) {
		t.Fatalf("unexpected canonical query tags: got %v want %v", repo.candidateCalls[0].Tags, wantQueryTags)
	}
	if len(repo.memoryCalls) != 1 {
		t.Fatalf("expected one memory query, got %d", len(repo.memoryCalls))
	}
	wantMemoryTags := []string{"backend", "governance"}
	if !reflect.DeepEqual(repo.memoryCalls[0].Tags, wantMemoryTags) {
		t.Fatalf("unexpected canonical memory tags: got %v want %v", repo.memoryCalls[0].Tags, wantMemoryTags)
	}
	if !reflect.DeepEqual(result.Receipt.Meta.ResolvedTags, wantMemoryTags) {
		t.Fatalf("unexpected resolved tags: got %v want %v", result.Receipt.Meta.ResolvedTags, wantMemoryTags)
	}
}

func TestGetContext_DefaultRepoTagsFileDiscoveryMergesCanonicalAliases(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".acm"), 0o755); err != nil {
		t.Fatalf("mkdir .acm: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, ".acm", "acm-tags.yaml"), []byte("version: acm.tags.v1\ncanonical_tags:\n  backend:\n    - svc\n"), 0o644); err != nil {
		t.Fatalf("write tags file: %v", err)
	}
	withWorkingDir(t, root)

	repo := &fakeRepository{
		scopeResults: []core.ReceiptScope{{
			ProjectID:    "project.alpha",
			ReceiptID:    "receipt.abc123",
			PointerPaths: []string{},
		}},
	}
	svc, err := New(repo)
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	result, apiErr := svc.GetContext(context.Background(), v1.GetContextPayload{
		ProjectID:    "project.alpha",
		TaskText:     "fix svc bootstrap gap",
		Phase:        v1.PhaseExecute,
		FallbackMode: "none",
	})
	if apiErr != nil {
		t.Fatalf("unexpected API error: %+v", apiErr)
	}
	if result.Status != "insufficient_context" {
		t.Fatalf("unexpected status: %+v", result)
	}
	if len(repo.candidateCalls) != 1 {
		t.Fatalf("expected one candidate query, got %d", len(repo.candidateCalls))
	}
	wantTags := []string{"backend", "bootstrap"}
	if !reflect.DeepEqual(repo.candidateCalls[0].Tags, wantTags) {
		t.Fatalf("unexpected query tags: got %v want %v", repo.candidateCalls[0].Tags, wantTags)
	}
}

func TestGetContext_MaxHopsHonored(t *testing.T) {
	repo := &fakeRepository{
		candidateResults: [][]core.CandidatePointer{{
			candidate("rule:a", "AGENTS.md", true, []string{"rules"}),
			candidate("code:1", "internal/a.go", false, []string{"backend"}),
		}},
		hopResults: [][]core.HopPointer{{
			hop("code:1", 1, candidate("test:1", "internal/a_test.go", false, []string{"tests"})),
			hop("code:1", 2, candidate("doc:2", "docs/ADR-001-context-broker.md", false, []string{"docs"})),
			hop("code:1", 3, candidate("code:3", "internal/c.go", false, []string{"backend"})),
		}},
		memoryResults: [][]core.ActiveMemory{{}},
	}
	svc, err := New(repo)
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	result, apiErr := svc.GetContext(context.Background(), v1.GetContextPayload{
		ProjectID: "project.alpha",
		TaskText:  "hop expansion coverage",
		Phase:     v1.PhaseExecute,
		Caps: &v1.RetrievalCaps{
			MaxHops:         2,
			MaxHopExpansion: 5,
			MinPointerCount: 1,
		},
	})
	if apiErr != nil {
		t.Fatalf("unexpected API error: %+v", apiErr)
	}
	if result.Status != "ok" || result.Receipt == nil {
		t.Fatalf("unexpected result: %+v", result)
	}
	if len(repo.hopCalls) != 1 {
		t.Fatalf("expected one hop query, got %d", len(repo.hopCalls))
	}
	if repo.hopCalls[0].MaxHops != 2 {
		t.Fatalf("expected max_hops=2, got %d", repo.hopCalls[0].MaxHops)
	}
	wantKeys := []string{"code:1", "doc:2", "rule:a", "test:1"}
	if got := pointerKeys(result.Receipt); !reflect.DeepEqual(got, wantKeys) {
		t.Fatalf("unexpected pointer keys: got %v want %v", got, wantKeys)
	}
	foundDocSuggestion := false
	for _, entry := range receiptIndexEntries(result.Receipt, "suggestions") {
		if entryString(entry, "key") != "doc:2" {
			continue
		}
		foundDocSuggestion = true
		if strings.TrimSpace(entryString(entry, "summary")) == "" {
			t.Fatalf("expected non-empty summary for doc:2 suggestion: %+v", entry)
		}
		break
	}
	if !foundDocSuggestion {
		t.Fatalf("expected doc:2 suggestion entry in receipt index")
	}
}

func TestProposeMemory_AutoPromoteOmittedDefaultsTrue(t *testing.T) {
	repo := &fakeRepository{
		scopeResults: []core.ReceiptScope{{
			ProjectID:   "project.alpha",
			ReceiptID:   "receipt.abc123",
			PointerKeys: []string{"ptr:scope-a", "ptr:scope-b"},
		}},
	}
	svc, err := New(repo)
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	result, apiErr := svc.ProposeMemory(context.Background(), v1.ProposeMemoryPayload{
		ProjectID: "project.alpha",
		ReceiptID: "receipt.abc123",
		Memory: v1.MemoryPayload{
			Category:            v1.MemoryCategoryDecision,
			Subject:             "Deterministic persistence",
			Content:             "Insert candidate first, then try durable promotion.",
			RelatedPointerKeys:  []string{"ptr:scope-b"},
			Tags:                []string{"backend"},
			Confidence:          4,
			EvidencePointerKeys: []string{"ptr:scope-a"},
		},
	})
	if apiErr != nil {
		t.Fatalf("unexpected API error: %+v", apiErr)
	}
	if result.Status != "promoted" {
		t.Fatalf("expected promoted status, got %+v", result)
	}
	if result.PromotedMemoryID == 0 {
		t.Fatalf("expected promoted memory id, got %+v", result)
	}
	if len(repo.proposeCalls) != 1 {
		t.Fatalf("expected one persistence call, got %d", len(repo.proposeCalls))
	}
	if !repo.proposeCalls[0].AutoPromote || !repo.proposeCalls[0].Promotable {
		t.Fatalf("expected auto-promote default true with promotable=true, got %+v", repo.proposeCalls[0])
	}
}

func TestProposeMemory_AutoPromoteFalseReturnsPending(t *testing.T) {
	repo := &fakeRepository{
		scopeResults: []core.ReceiptScope{{
			ProjectID:   "project.alpha",
			ReceiptID:   "receipt.abc123",
			PointerKeys: []string{"ptr:scope-a", "ptr:scope-b"},
		}},
	}
	svc, err := New(repo)
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	result, apiErr := svc.ProposeMemory(context.Background(), v1.ProposeMemoryPayload{
		ProjectID:   "project.alpha",
		ReceiptID:   "receipt.abc123",
		AutoPromote: boolPtr(false),
		Memory: v1.MemoryPayload{
			Category:            v1.MemoryCategoryDecision,
			Subject:             "Deterministic persistence",
			Content:             "Insert candidate first, then try durable promotion.",
			RelatedPointerKeys:  []string{"ptr:scope-b"},
			Tags:                []string{"backend"},
			Confidence:          4,
			EvidencePointerKeys: []string{"ptr:scope-a"},
		},
	})
	if apiErr != nil {
		t.Fatalf("unexpected API error: %+v", apiErr)
	}
	if result.Status != "pending" {
		t.Fatalf("expected pending status, got %+v", result)
	}
	if len(repo.proposeCalls) != 1 {
		t.Fatalf("expected one persistence call, got %d", len(repo.proposeCalls))
	}
	if repo.proposeCalls[0].AutoPromote || repo.proposeCalls[0].Promotable {
		t.Fatalf("expected auto-promote disabled call, got %+v", repo.proposeCalls[0])
	}
}

func TestProposeMemory_AutoPromoteTrueHardAndSoftPassPromotes(t *testing.T) {
	repo := &fakeRepository{
		scopeResults: []core.ReceiptScope{{
			ProjectID:   "project.alpha",
			ReceiptID:   "receipt.abc123",
			PointerKeys: []string{"ptr:scope-a", "ptr:scope-b"},
		}},
	}
	svc, err := New(repo)
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	result, apiErr := svc.ProposeMemory(context.Background(), v1.ProposeMemoryPayload{
		ProjectID:   "project.alpha",
		ReceiptID:   "receipt.abc123",
		AutoPromote: boolPtr(true),
		Memory: v1.MemoryPayload{
			Category:            v1.MemoryCategoryDecision,
			Subject:             "Scoped promotion",
			Content:             "Promote when hard and soft validation pass.",
			RelatedPointerKeys:  []string{"ptr:scope-b"},
			Tags:                []string{"backend"},
			Confidence:          4,
			EvidencePointerKeys: []string{"ptr:scope-a"},
		},
	})
	if apiErr != nil {
		t.Fatalf("unexpected API error: %+v", apiErr)
	}
	if result.Status != "promoted" || result.PromotedMemoryID == 0 {
		t.Fatalf("expected promoted result, got %+v", result)
	}
}

func TestProposeMemory_HardFailEvidenceOutsideScopeRejected(t *testing.T) {
	repo := &fakeRepository{
		scopeResults: []core.ReceiptScope{{
			ProjectID:   "project.alpha",
			ReceiptID:   "receipt.abc123",
			PointerKeys: []string{"ptr:scope-a"},
		}},
	}
	svc, err := New(repo)
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	result, apiErr := svc.ProposeMemory(context.Background(), v1.ProposeMemoryPayload{
		ProjectID:   "project.alpha",
		ReceiptID:   "receipt.abc123",
		AutoPromote: boolPtr(true),
		Memory: v1.MemoryPayload{
			Category:            v1.MemoryCategoryDecision,
			Subject:             "Bad evidence",
			Content:             "Evidence points outside receipt scope.",
			RelatedPointerKeys:  []string{"ptr:scope-a"},
			Tags:                []string{"backend"},
			Confidence:          3,
			EvidencePointerKeys: []string{"ptr:outside"},
		},
	})
	if apiErr != nil {
		t.Fatalf("unexpected API error: %+v", apiErr)
	}
	if result.Status != "rejected" {
		t.Fatalf("expected rejected status, got %+v", result)
	}
	if result.Validation.HardPassed {
		t.Fatalf("expected hard validation failure, got %+v", result.Validation)
	}
	if len(result.Validation.Errors) == 0 {
		t.Fatalf("expected hard validation errors, got %+v", result.Validation)
	}
	if len(repo.proposeCalls) != 1 {
		t.Fatalf("expected candidate persistence call even on hard fail, got %d", len(repo.proposeCalls))
	}
}

func TestProposeMemory_SoftFailWithAutoPromoteTrueReturnsPending(t *testing.T) {
	repo := &fakeRepository{
		scopeResults: []core.ReceiptScope{{
			ProjectID:   "project.alpha",
			ReceiptID:   "receipt.abc123",
			PointerKeys: []string{"ptr:scope-a"},
		}},
	}
	svc, err := New(repo)
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	result, apiErr := svc.ProposeMemory(context.Background(), v1.ProposeMemoryPayload{
		ProjectID:   "project.alpha",
		ReceiptID:   "receipt.abc123",
		AutoPromote: boolPtr(true),
		Memory: v1.MemoryPayload{
			Category:            v1.MemoryCategoryDecision,
			Subject:             "Soft warning",
			Content:             "Related pointer is out of scope.",
			RelatedPointerKeys:  []string{"ptr:outside"},
			Tags:                []string{"backend"},
			Confidence:          3,
			EvidencePointerKeys: []string{"ptr:scope-a"},
		},
	})
	if apiErr != nil {
		t.Fatalf("unexpected API error: %+v", apiErr)
	}
	if result.Status != "pending" {
		t.Fatalf("expected pending status, got %+v", result)
	}
	if !result.Validation.HardPassed || result.Validation.SoftPassed {
		t.Fatalf("expected hard pass + soft fail, got %+v", result.Validation)
	}
	if len(result.Validation.Warnings) == 0 {
		t.Fatalf("expected soft warnings, got %+v", result.Validation)
	}
	if len(repo.proposeCalls) != 1 || repo.proposeCalls[0].Promotable {
		t.Fatalf("expected persisted non-promotable candidate, got %+v", repo.proposeCalls)
	}
}

func TestProposeMemory_DuplicateConflictReturnsRejected(t *testing.T) {
	repo := &fakeRepository{
		scopeResults: []core.ReceiptScope{{
			ProjectID:   "project.alpha",
			ReceiptID:   "receipt.abc123",
			PointerKeys: []string{"ptr:scope-a"},
		}},
		proposeResults: []core.ProposeMemoryPersistenceResult{{
			CandidateID:      17,
			Status:           "rejected",
			PromotedMemoryID: 0,
		}},
	}
	svc, err := New(repo)
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	result, apiErr := svc.ProposeMemory(context.Background(), v1.ProposeMemoryPayload{
		ProjectID:   "project.alpha",
		ReceiptID:   "receipt.abc123",
		AutoPromote: boolPtr(true),
		Memory: v1.MemoryPayload{
			Category:            v1.MemoryCategoryDecision,
			Subject:             "Duplicate",
			Content:             "Same durable memory already exists.",
			RelatedPointerKeys:  []string{"ptr:scope-a"},
			Tags:                []string{"backend"},
			Confidence:          5,
			EvidencePointerKeys: []string{"ptr:scope-a"},
		},
	})
	if apiErr != nil {
		t.Fatalf("unexpected API error: %+v", apiErr)
	}
	if result.Status != "rejected" || result.CandidateID != 17 {
		t.Fatalf("expected rejected duplicate result, got %+v", result)
	}
	if !result.Validation.HardPassed || !result.Validation.SoftPassed {
		t.Fatalf("expected validation pass with duplicate rejection, got %+v", result.Validation)
	}
}

func TestProposeMemory_UnknownReceiptReturnsNotFound(t *testing.T) {
	repo := &fakeRepository{
		scopeErrors: []error{core.ErrReceiptScopeNotFound},
	}
	svc, err := New(repo)
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	_, apiErr := svc.ProposeMemory(context.Background(), v1.ProposeMemoryPayload{
		ProjectID: "project.alpha",
		ReceiptID: "receipt.missing",
		Memory: v1.MemoryPayload{
			Category:            v1.MemoryCategoryDecision,
			Subject:             "x",
			Content:             "y",
			Confidence:          3,
			EvidencePointerKeys: []string{"ptr:scope-a"},
		},
	})
	if apiErr == nil {
		t.Fatal("expected API error")
	}
	if apiErr.Code != "NOT_FOUND" {
		t.Fatalf("unexpected API error code: got %q want %q", apiErr.Code, "NOT_FOUND")
	}
	if len(repo.proposeCalls) != 0 {
		t.Fatalf("did not expect persistence call, got %d", len(repo.proposeCalls))
	}
}

func TestProposeMemory_FetchScopeErrorReturnsInternalError(t *testing.T) {
	repo := &fakeRepository{
		scopeErrors: []error{errors.New("scope lookup failed")},
	}
	svc, err := New(repo)
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	_, apiErr := svc.ProposeMemory(context.Background(), v1.ProposeMemoryPayload{
		ProjectID: "project.alpha",
		ReceiptID: "receipt.abc123",
		Memory: v1.MemoryPayload{
			Category:            v1.MemoryCategoryDecision,
			Subject:             "x",
			Content:             "y",
			Confidence:          3,
			EvidencePointerKeys: []string{"ptr:scope-a"},
		},
	})
	if apiErr == nil {
		t.Fatal("expected API error")
	}
	if apiErr.Code != "INTERNAL_ERROR" {
		t.Fatalf("unexpected API error code: got %q want %q", apiErr.Code, "INTERNAL_ERROR")
	}
	if len(repo.proposeCalls) != 0 {
		t.Fatalf("did not expect persistence call, got %d", len(repo.proposeCalls))
	}
}

func TestProposeMemory_PersistErrorReturnsInternalError(t *testing.T) {
	repo := &fakeRepository{
		scopeResults: []core.ReceiptScope{{
			ProjectID:   "project.alpha",
			ReceiptID:   "receipt.abc123",
			PointerKeys: []string{"ptr:scope-a"},
		}},
		proposeErrors: []error{errors.New("insert failed")},
	}
	svc, err := New(repo)
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	_, apiErr := svc.ProposeMemory(context.Background(), v1.ProposeMemoryPayload{
		ProjectID: "project.alpha",
		ReceiptID: "receipt.abc123",
		Memory: v1.MemoryPayload{
			Category:            v1.MemoryCategoryDecision,
			Subject:             "x",
			Content:             "y",
			Confidence:          3,
			EvidencePointerKeys: []string{"ptr:scope-a"},
		},
	})
	if apiErr == nil {
		t.Fatal("expected API error")
	}
	if apiErr.Code != "INTERNAL_ERROR" {
		t.Fatalf("unexpected API error code: got %q want %q", apiErr.Code, "INTERNAL_ERROR")
	}
}

func TestProposeMemory_NormalizationAndDedupeKeyDeterministic(t *testing.T) {
	repo := &fakeRepository{
		scopeResults: []core.ReceiptScope{
			{
				ProjectID:   "project.alpha",
				ReceiptID:   "receipt.abc123",
				PointerKeys: []string{"ptr:scope-a", "ptr:scope-b"},
			},
			{
				ProjectID:   "project.alpha",
				ReceiptID:   "receipt.abc123",
				PointerKeys: []string{"ptr:scope-a", "ptr:scope-b"},
			},
		},
	}
	svc, err := New(repo)
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	payloadA := v1.ProposeMemoryPayload{
		ProjectID:   "project.alpha",
		ReceiptID:   "receipt.abc123",
		AutoPromote: boolPtr(false),
		Memory: v1.MemoryPayload{
			Category:            v1.MemoryCategoryDecision,
			Subject:             "  Deterministic subject  ",
			Content:             "  Deterministic content  ",
			RelatedPointerKeys:  []string{"ptr:scope-b", "ptr:scope-a", "ptr:scope-b"},
			Tags:                []string{"ops", "backend", "ops", " "},
			Confidence:          4,
			EvidencePointerKeys: []string{"ptr:scope-a", "ptr:scope-b", "ptr:scope-a"},
		},
	}
	payloadB := v1.ProposeMemoryPayload{
		ProjectID:   "project.alpha",
		ReceiptID:   "receipt.abc123",
		AutoPromote: boolPtr(false),
		Memory: v1.MemoryPayload{
			Category:            v1.MemoryCategoryDecision,
			Subject:             "Deterministic subject",
			Content:             "Deterministic content",
			RelatedPointerKeys:  []string{"ptr:scope-a", "ptr:scope-b"},
			Tags:                []string{"backend", "ops"},
			Confidence:          4,
			EvidencePointerKeys: []string{"ptr:scope-b", "ptr:scope-a"},
		},
	}

	if _, apiErr := svc.ProposeMemory(context.Background(), payloadA); apiErr != nil {
		t.Fatalf("unexpected API error on payloadA: %+v", apiErr)
	}
	if _, apiErr := svc.ProposeMemory(context.Background(), payloadB); apiErr != nil {
		t.Fatalf("unexpected API error on payloadB: %+v", apiErr)
	}

	if len(repo.proposeCalls) != 2 {
		t.Fatalf("expected 2 persistence calls, got %d", len(repo.proposeCalls))
	}

	first := repo.proposeCalls[0]
	second := repo.proposeCalls[1]
	if first.DedupeKey != second.DedupeKey {
		t.Fatalf("expected deterministic dedupe key, got %q and %q", first.DedupeKey, second.DedupeKey)
	}
	if first.Subject != "Deterministic subject" || first.Content != "Deterministic content" {
		t.Fatalf("expected normalized subject/content, got %q / %q", first.Subject, first.Content)
	}
	if !reflect.DeepEqual(first.Tags, second.Tags) {
		t.Fatalf("expected deterministic normalized tags, got %v and %v", first.Tags, second.Tags)
	}
	if !reflect.DeepEqual(first.RelatedPointerKeys, second.RelatedPointerKeys) {
		t.Fatalf("expected deterministic normalized related keys, got %v and %v", first.RelatedPointerKeys, second.RelatedPointerKeys)
	}
	if !reflect.DeepEqual(first.EvidencePointerKeys, second.EvidencePointerKeys) {
		t.Fatalf("expected deterministic normalized evidence keys, got %v and %v", first.EvidencePointerKeys, second.EvidencePointerKeys)
	}
}

func TestProposeMemory_CanonicalTagNormalization(t *testing.T) {
	repo := &fakeRepository{
		scopeResults: []core.ReceiptScope{{
			ProjectID:   "project.alpha",
			ReceiptID:   "receipt.abc123",
			PointerKeys: []string{"ptr:scope-a"},
		}},
	}
	svc, err := New(repo)
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	_, apiErr := svc.ProposeMemory(context.Background(), v1.ProposeMemoryPayload{
		ProjectID: "project.alpha",
		ReceiptID: "receipt.abc123",
		Memory: v1.MemoryPayload{
			Category:            v1.MemoryCategoryDecision,
			Subject:             "Canonical tags",
			Content:             "Ensure propose_memory normalizes tags against canonical aliases.",
			RelatedPointerKeys:  []string{"ptr:scope-a"},
			Tags:                []string{"API", "backend", "Policies", "  "},
			Confidence:          4,
			EvidencePointerKeys: []string{"ptr:scope-a"},
		},
	})
	if apiErr != nil {
		t.Fatalf("unexpected API error: %+v", apiErr)
	}
	if len(repo.proposeCalls) != 1 {
		t.Fatalf("expected one persistence call, got %d", len(repo.proposeCalls))
	}
	wantTags := []string{"backend", "governance"}
	if !reflect.DeepEqual(repo.proposeCalls[0].Tags, wantTags) {
		t.Fatalf("unexpected canonical propose tags: got %v want %v", repo.proposeCalls[0].Tags, wantTags)
	}
}

func TestProposeMemory_TagsFileOverrideNormalizesCustomAliases(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".acm"), 0o755); err != nil {
		t.Fatalf("mkdir .acm: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, ".acm", "acm-tags.yaml"), []byte("version: acm.tags.v1\ncanonical_tags:\n  backend:\n    - svc\n"), 0o644); err != nil {
		t.Fatalf("write tags file: %v", err)
	}
	withWorkingDir(t, root)

	repo := &fakeRepository{
		scopeResults: []core.ReceiptScope{{
			ProjectID:   "project.alpha",
			ReceiptID:   "receipt.abc123",
			PointerKeys: []string{"ptr:scope-a"},
		}},
	}
	svc, err := New(repo)
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	_, apiErr := svc.ProposeMemory(context.Background(), v1.ProposeMemoryPayload{
		ProjectID: "project.alpha",
		ReceiptID: "receipt.abc123",
		TagsFile:  ".acm/acm-tags.yaml",
		Memory: v1.MemoryPayload{
			Category:            v1.MemoryCategoryDecision,
			Subject:             "Custom aliases",
			Content:             "Ensure propose_memory honors repo-local aliases.",
			RelatedPointerKeys:  []string{"ptr:scope-a"},
			Tags:                []string{"svc", "backend"},
			Confidence:          4,
			EvidencePointerKeys: []string{"ptr:scope-a"},
		},
	})
	if apiErr != nil {
		t.Fatalf("unexpected API error: %+v", apiErr)
	}
	if len(repo.proposeCalls) != 1 {
		t.Fatalf("expected one persistence call, got %d", len(repo.proposeCalls))
	}
	wantTags := []string{"backend"}
	if !reflect.DeepEqual(repo.proposeCalls[0].Tags, wantTags) {
		t.Fatalf("unexpected canonical propose tags: got %v want %v", repo.proposeCalls[0].Tags, wantTags)
	}
}

func TestReportCompletion_AcceptsInScopeAndPersistsSummary(t *testing.T) {
	repo := &fakeRepository{
		scopeResults: []core.ReceiptScope{{
			ProjectID:    "project.alpha",
			ReceiptID:    "receipt.abc123",
			TaskText:     "implement completion path",
			Phase:        "execute",
			ResolvedTags: []string{"backend"},
			PointerKeys:  []string{"code:repo"},
			MemoryIDs:    []int64{7},
			PointerPaths: []string{"internal/service/backend/service.go", "internal/core/repository.go"},
		}},
		saveResult: core.RunReceiptIDs{RunID: 42, ReceiptID: "receipt.abc123"},
	}
	svc, err := New(repo)
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	result, apiErr := svc.ReportCompletion(context.Background(), v1.ReportCompletionPayload{
		ProjectID:    "project.alpha",
		ReceiptID:    "receipt.abc123",
		FilesChanged: []string{"internal/core/repository.go"},
		Outcome:      "completed",
	})
	if apiErr != nil {
		t.Fatalf("unexpected API error: %+v", apiErr)
	}
	if !result.Accepted {
		t.Fatalf("expected accepted result: %+v", result)
	}
	if result.RunID != 42 {
		t.Fatalf("unexpected run id: got %d want 42", result.RunID)
	}
	if len(result.Violations) != 0 {
		t.Fatalf("expected no violations, got %+v", result.Violations)
	}
	if len(repo.scopeCalls) != 1 {
		t.Fatalf("expected one scope lookup, got %d", len(repo.scopeCalls))
	}
	if len(repo.saveCalls) != 1 {
		t.Fatalf("expected one save call, got %d", len(repo.saveCalls))
	}
	if repo.saveCalls[0].Status != "accepted_with_warnings" {
		t.Fatalf("unexpected status persisted: %q", repo.saveCalls[0].Status)
	}
	wantIssues := []string{"required verification work item is missing: verify:tests"}
	if !reflect.DeepEqual(result.DefinitionOfDoneIssues, wantIssues) {
		t.Fatalf("unexpected DoD issues: got %v want %v", result.DefinitionOfDoneIssues, wantIssues)
	}
	if !reflect.DeepEqual(repo.saveCalls[0].DefinitionOfDoneIssues, wantIssues) {
		t.Fatalf("unexpected persisted DoD issues: got %v want %v", repo.saveCalls[0].DefinitionOfDoneIssues, wantIssues)
	}
	if repo.saveCalls[0].TaskText != "implement completion path" || repo.saveCalls[0].Phase != "execute" {
		t.Fatalf("expected scope metadata in persisted summary, got task=%q phase=%q", repo.saveCalls[0].TaskText, repo.saveCalls[0].Phase)
	}
}

func TestReportCompletion_StrictModeRejectsOutOfScopeWithoutPersistence(t *testing.T) {
	repo := &fakeRepository{
		scopeResults: []core.ReceiptScope{{
			ProjectID:    "project.alpha",
			ReceiptID:    "receipt.abc123",
			PointerPaths: []string{"internal/service/backend/service.go"},
		}},
	}
	svc, err := New(repo)
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	result, apiErr := svc.ReportCompletion(context.Background(), v1.ReportCompletionPayload{
		ProjectID:    "project.alpha",
		ReceiptID:    "receipt.abc123",
		FilesChanged: []string{"README.md", "internal/service/backend/service.go"},
		Outcome:      "completed",
		ScopeMode:    v1.ScopeModeStrict,
	})
	if apiErr != nil {
		t.Fatalf("unexpected API error: %+v", apiErr)
	}
	if result.Accepted {
		t.Fatalf("expected rejection: %+v", result)
	}
	wantViolations := []v1.CompletionViolation{{
		Path:   "README.md",
		Reason: "path is outside receipt scope",
	}}
	if !reflect.DeepEqual(result.Violations, wantViolations) {
		t.Fatalf("unexpected violations: got %+v want %+v", result.Violations, wantViolations)
	}
	if len(repo.saveCalls) != 0 {
		t.Fatalf("did not expect save on rejection, got %d", len(repo.saveCalls))
	}
}

func TestReportCompletion_StrictModeAcceptsManagedAcmFiles(t *testing.T) {
	repo := &fakeRepository{
		scopeResults: []core.ReceiptScope{{
			ProjectID:    "project.alpha",
			ReceiptID:    "receipt.abc123",
			PointerPaths: []string{"internal/core/repository.go"},
		}},
		workListResults: [][]core.WorkItem{{
			{ItemKey: "verify:tests", Status: core.WorkItemStatusComplete},
		}},
		saveResult: core.RunReceiptIDs{RunID: 72, ReceiptID: "receipt.abc123"},
	}
	svc, err := New(repo)
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	result, apiErr := svc.ReportCompletion(context.Background(), v1.ReportCompletionPayload{
		ProjectID: "project.alpha",
		ReceiptID: "receipt.abc123",
		FilesChanged: []string{
			".acm/acm-rules.yaml",
			".acm/acm-tags.yaml",
			".acm/acm-tests.yaml",
			".acm/acm-workflows.yaml",
			".gitignore",
		},
		Outcome:   "updated ACM-managed onboarding files",
		ScopeMode: v1.ScopeModeStrict,
	})
	if apiErr != nil {
		t.Fatalf("unexpected API error: %+v", apiErr)
	}
	if !result.Accepted {
		t.Fatalf("expected strict-mode acceptance for ACM-managed files: %+v", result)
	}
	if len(result.Violations) != 0 {
		t.Fatalf("expected no scope violations, got %+v", result.Violations)
	}
	if len(repo.saveCalls) != 1 {
		t.Fatalf("expected one persisted run summary, got %d", len(repo.saveCalls))
	}
}

func TestReportCompletion_StrictModeAcceptsCompletedVerifyTestsWithoutDiffReview(t *testing.T) {
	repo := &fakeRepository{
		scopeResults: []core.ReceiptScope{{
			ProjectID:    "project.alpha",
			ReceiptID:    "receipt.abc123",
			PointerPaths: []string{"internal/core/repository.go"},
		}},
		workListResults: [][]core.WorkItem{{
			{ItemKey: "verify:tests", Status: core.WorkItemStatusComplete},
		}},
	}
	svc, err := New(repo)
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	result, apiErr := svc.ReportCompletion(context.Background(), v1.ReportCompletionPayload{
		ProjectID:    "project.alpha",
		ReceiptID:    "receipt.abc123",
		FilesChanged: []string{"internal/core/repository.go"},
		Outcome:      "completed",
		ScopeMode:    v1.ScopeModeStrict,
	})
	if apiErr != nil {
		t.Fatalf("unexpected API error: %+v", apiErr)
	}
	if !result.Accepted {
		t.Fatalf("expected strict-mode acceptance: %+v", result)
	}
	if len(result.Violations) != 0 {
		t.Fatalf("expected no scope violations, got %+v", result.Violations)
	}
	if got, want := result.DefinitionOfDoneIssues, []string(nil); !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected DoD issues: got %v want %v", got, want)
	}
	if len(repo.saveCalls) != 1 {
		t.Fatalf("expected one persisted run summary, got %d", len(repo.saveCalls))
	}
}

func TestReportCompletion_WarnModeAcceptsIncompleteDefinitionOfDoneAndPersistsWarning(t *testing.T) {
	repo := &fakeRepository{
		scopeResults: []core.ReceiptScope{{
			ProjectID:    "project.alpha",
			ReceiptID:    "receipt.abc123",
			PointerPaths: []string{"internal/core/repository.go"},
		}},
		workListResults: [][]core.WorkItem{{
			{ItemKey: "verify:tests", Status: core.WorkItemStatusInProgress},
		}},
		saveResult: core.RunReceiptIDs{RunID: 73, ReceiptID: "receipt.abc123"},
	}
	svc, err := New(repo)
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	result, apiErr := svc.ReportCompletion(context.Background(), v1.ReportCompletionPayload{
		ProjectID:    "project.alpha",
		ReceiptID:    "receipt.abc123",
		FilesChanged: []string{"internal/core/repository.go"},
		Outcome:      "completed",
		ScopeMode:    v1.ScopeModeWarn,
	})
	if apiErr != nil {
		t.Fatalf("unexpected API error: %+v", apiErr)
	}
	if !result.Accepted {
		t.Fatalf("expected warn-mode acceptance: %+v", result)
	}
	wantIssues := []string{
		"required verification work item is not complete: verify:tests (status=in_progress)",
	}
	if !reflect.DeepEqual(result.DefinitionOfDoneIssues, wantIssues) {
		t.Fatalf("unexpected DoD issues: got %v want %v", result.DefinitionOfDoneIssues, wantIssues)
	}
	if len(repo.saveCalls) != 1 {
		t.Fatalf("expected one persisted run summary, got %d", len(repo.saveCalls))
	}
	if repo.saveCalls[0].Status != "accepted_with_warnings" {
		t.Fatalf("unexpected persisted status: %q", repo.saveCalls[0].Status)
	}
	if !reflect.DeepEqual(repo.saveCalls[0].DefinitionOfDoneIssues, wantIssues) {
		t.Fatalf("unexpected persisted DoD issues: got %v want %v", repo.saveCalls[0].DefinitionOfDoneIssues, wantIssues)
	}
}

func TestReportCompletion_NoWorkItemsWarnModeFlagsMissingVerify(t *testing.T) {
	repo := &fakeRepository{
		scopeResults: []core.ReceiptScope{{
			ProjectID:    "project.alpha",
			ReceiptID:    "receipt.abc123",
			PointerPaths: []string{"internal/core/repository.go"},
		}},
		workListResults: [][]core.WorkItem{{}},
		saveResult:      core.RunReceiptIDs{RunID: 58, ReceiptID: "receipt.abc123"},
	}
	svc, err := New(repo)
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	result, apiErr := svc.ReportCompletion(context.Background(), v1.ReportCompletionPayload{
		ProjectID:    "project.alpha",
		ReceiptID:    "receipt.abc123",
		FilesChanged: []string{"internal/core/repository.go"},
		Outcome:      "completed",
		ScopeMode:    v1.ScopeModeWarn,
	})
	if apiErr != nil {
		t.Fatalf("unexpected API error: %+v", apiErr)
	}
	if !result.Accepted {
		t.Fatalf("expected acceptance in warn mode when no work items exist: %+v", result)
	}
	wantIssues := []string{
		"required verification work item is missing: verify:tests",
	}
	if !reflect.DeepEqual(result.DefinitionOfDoneIssues, wantIssues) {
		t.Fatalf("unexpected DoD issues without work items: got %v want %v", result.DefinitionOfDoneIssues, wantIssues)
	}
	if len(repo.saveCalls) != 1 {
		t.Fatalf("expected one persisted run summary, got %d", len(repo.saveCalls))
	}
	if repo.saveCalls[0].Status != "accepted_with_warnings" {
		t.Fatalf("unexpected persisted status when no work items exist: %q", repo.saveCalls[0].Status)
	}
	if !reflect.DeepEqual(repo.saveCalls[0].DefinitionOfDoneIssues, wantIssues) {
		t.Fatalf("unexpected persisted DoD issues without work items: got %v want %v", repo.saveCalls[0].DefinitionOfDoneIssues, wantIssues)
	}
}

func TestReportCompletion_NoWorkItemsStrictRejectsMissingVerify(t *testing.T) {
	repo := &fakeRepository{
		scopeResults: []core.ReceiptScope{{
			ProjectID:    "project.alpha",
			ReceiptID:    "receipt.abc123",
			PointerPaths: []string{"internal/core/repository.go"},
		}},
		workListResults: [][]core.WorkItem{{}},
	}
	svc, err := New(repo)
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	result, apiErr := svc.ReportCompletion(context.Background(), v1.ReportCompletionPayload{
		ProjectID:    "project.alpha",
		ReceiptID:    "receipt.abc123",
		FilesChanged: []string{"internal/core/repository.go"},
		Outcome:      "completed",
		ScopeMode:    v1.ScopeModeStrict,
	})
	if apiErr != nil {
		t.Fatalf("unexpected API error: %+v", apiErr)
	}
	if result.Accepted {
		t.Fatalf("expected strict-mode rejection without verify work: %+v", result)
	}
	wantIssues := []string{
		"required verification work item is missing: verify:tests",
	}
	if !reflect.DeepEqual(result.DefinitionOfDoneIssues, wantIssues) {
		t.Fatalf("unexpected DoD issues: got %v want %v", result.DefinitionOfDoneIssues, wantIssues)
	}
	if len(repo.saveCalls) != 0 {
		t.Fatalf("expected no persisted run summary on strict rejection, got %d", len(repo.saveCalls))
	}
}

func TestReportCompletion_BlankWorkflowDefinitionsFallBackToDefaultVerifyGate(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".acm"), 0o755); err != nil {
		t.Fatalf("mkdir .acm: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, ".acm", "acm-workflows.yaml"), []byte("version: acm.workflows.v1\ncompletion:\n  required_tasks: []\n"), 0o644); err != nil {
		t.Fatalf("write workflows file: %v", err)
	}

	repo := &fakeRepository{
		scopeResults: []core.ReceiptScope{{
			ProjectID:    "project.alpha",
			ReceiptID:    "receipt.abc123",
			PointerPaths: []string{"internal/core/repository.go"},
		}},
		workListResults: [][]core.WorkItem{{}},
	}
	svc, err := NewWithProjectRoot(repo, root)
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	result, apiErr := svc.ReportCompletion(context.Background(), v1.ReportCompletionPayload{
		ProjectID:    "project.alpha",
		ReceiptID:    "receipt.abc123",
		FilesChanged: []string{"internal/core/repository.go"},
		Outcome:      "completed",
		ScopeMode:    v1.ScopeModeStrict,
	})
	if apiErr != nil {
		t.Fatalf("unexpected API error: %+v", apiErr)
	}
	if result.Accepted {
		t.Fatalf("expected strict-mode rejection without verify work: %+v", result)
	}
	wantIssues := []string{"required verification work item is missing: verify:tests"}
	if !reflect.DeepEqual(result.DefinitionOfDoneIssues, wantIssues) {
		t.Fatalf("unexpected DoD issues: got %v want %v", result.DefinitionOfDoneIssues, wantIssues)
	}
}

func TestReportCompletion_StrictModeRejectsMissingConfiguredWorkflowGate(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".acm"), 0o755); err != nil {
		t.Fatalf("mkdir .acm: %v", err)
	}
	workflowsYAML := "version: acm.workflows.v1\ncompletion:\n  required_tasks:\n    - key: verify:tests\n      select:\n        changed_paths_any: [\"internal/**\"]\n    - key: review:cross-llm\n      select:\n        phases: [\"execute\", \"review\"]\n        changed_paths_any: [\"internal/**\"]\n"
	if err := os.WriteFile(filepath.Join(root, ".acm", "acm-workflows.yaml"), []byte(workflowsYAML), 0o644); err != nil {
		t.Fatalf("write workflows file: %v", err)
	}

	repo := &fakeRepository{
		scopeResults: []core.ReceiptScope{{
			ProjectID:    "project.alpha",
			ReceiptID:    "receipt.abc123",
			Phase:        "review",
			PointerPaths: []string{"internal/core/repository.go"},
		}},
		workListResults: [][]core.WorkItem{{
			{ItemKey: "verify:tests", Status: core.WorkItemStatusComplete},
		}},
	}
	svc, err := NewWithProjectRoot(repo, root)
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	result, apiErr := svc.ReportCompletion(context.Background(), v1.ReportCompletionPayload{
		ProjectID:    "project.alpha",
		ReceiptID:    "receipt.abc123",
		FilesChanged: []string{"internal/core/repository.go"},
		Outcome:      "completed",
		ScopeMode:    v1.ScopeModeStrict,
	})
	if apiErr != nil {
		t.Fatalf("unexpected API error: %+v", apiErr)
	}
	if result.Accepted {
		t.Fatalf("expected strict-mode rejection when configured review gate is missing: %+v", result)
	}
	wantIssues := []string{"required workflow work item is missing: review:cross-llm"}
	if !reflect.DeepEqual(result.DefinitionOfDoneIssues, wantIssues) {
		t.Fatalf("unexpected DoD issues: got %v want %v", result.DefinitionOfDoneIssues, wantIssues)
	}
}

func TestReportCompletion_ConfiguredWorkflowSelectorsCanNarrowRequiredGates(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".acm"), 0o755); err != nil {
		t.Fatalf("mkdir .acm: %v", err)
	}
	workflowsYAML := "version: acm.workflows.v1\ncompletion:\n  required_tasks:\n    - key: review:cross-llm\n      select:\n        changed_paths_any: [\"internal/**\"]\n"
	if err := os.WriteFile(filepath.Join(root, ".acm", "acm-workflows.yaml"), []byte(workflowsYAML), 0o644); err != nil {
		t.Fatalf("write workflows file: %v", err)
	}

	repo := &fakeRepository{
		scopeResults: []core.ReceiptScope{{
			ProjectID:    "project.alpha",
			ReceiptID:    "receipt.abc123",
			Phase:        "review",
			PointerPaths: []string{"README.md"},
		}},
		workListResults: [][]core.WorkItem{{}},
		saveResult:      core.RunReceiptIDs{RunID: 88, ReceiptID: "receipt.abc123"},
	}
	svc, err := NewWithProjectRoot(repo, root)
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	result, apiErr := svc.ReportCompletion(context.Background(), v1.ReportCompletionPayload{
		ProjectID:    "project.alpha",
		ReceiptID:    "receipt.abc123",
		FilesChanged: []string{"README.md"},
		Outcome:      "completed",
		ScopeMode:    v1.ScopeModeStrict,
	})
	if apiErr != nil {
		t.Fatalf("unexpected API error: %+v", apiErr)
	}
	if !result.Accepted {
		t.Fatalf("expected strict-mode acceptance when workflow selectors do not match: %+v", result)
	}
	if got, want := result.DefinitionOfDoneIssues, []string(nil); !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected DoD issues: got %v want %v", got, want)
	}
	if len(repo.saveCalls) != 1 {
		t.Fatalf("expected one persisted run summary, got %d", len(repo.saveCalls))
	}
}

func TestReportCompletion_InvalidWorkflowDefinitionsReturnUserInputError(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".acm"), 0o755); err != nil {
		t.Fatalf("mkdir .acm: %v", err)
	}
	workflowsYAML := "version: acm.workflows.v1\ncompletion:\n  required_tasks:\n    - key: review:cross-llm\n      run:\n        argv: [\"scripts/acm-cross-review.sh\"]\n        bad_field: true\n"
	if err := os.WriteFile(filepath.Join(root, ".acm", "acm-workflows.yaml"), []byte(workflowsYAML), 0o644); err != nil {
		t.Fatalf("write workflows file: %v", err)
	}

	repo := &fakeRepository{
		scopeResults: []core.ReceiptScope{{
			ProjectID:    "project.alpha",
			ReceiptID:    "receipt.abc123",
			Phase:        "review",
			PointerPaths: []string{"internal/core/repository.go"},
		}},
		workListResults: [][]core.WorkItem{{}},
	}
	svc, err := NewWithProjectRoot(repo, root)
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	_, apiErr := svc.ReportCompletion(context.Background(), v1.ReportCompletionPayload{
		ProjectID:    "project.alpha",
		ReceiptID:    "receipt.abc123",
		FilesChanged: []string{"internal/core/repository.go"},
		Outcome:      "completed",
		ScopeMode:    v1.ScopeModeStrict,
	})
	if apiErr == nil {
		t.Fatal("expected API error")
	}
	if apiErr.Code != "INVALID_INPUT" {
		t.Fatalf("unexpected error code: %+v", apiErr)
	}
	if !strings.Contains(apiErr.Message, "workflow definitions are invalid") {
		t.Fatalf("unexpected error message: %+v", apiErr)
	}
}

func TestReportCompletion_EmptyFilesChangedWarnsInDefaultMode(t *testing.T) {
	repo := &fakeRepository{
		scopeResults: []core.ReceiptScope{{
			ProjectID: "project.alpha",
			ReceiptID: "receipt.abc123",
		}},
		workListResults: [][]core.WorkItem{{
			{ItemKey: "verify:tests", Status: core.WorkItemStatusComplete},
		}},
		saveResult: core.RunReceiptIDs{RunID: 74, ReceiptID: "receipt.abc123"},
	}
	svc, err := New(repo)
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	result, apiErr := svc.ReportCompletion(context.Background(), v1.ReportCompletionPayload{
		ProjectID:    "project.alpha",
		ReceiptID:    "receipt.abc123",
		FilesChanged: []string{},
		Outcome:      "completed",
	})
	if apiErr != nil {
		t.Fatalf("unexpected API error: %+v", apiErr)
	}
	if !result.Accepted {
		t.Fatalf("expected warn-mode acceptance: %+v", result)
	}
	wantIssues := []string{"files_changed must include at least one repository-relative path"}
	if !reflect.DeepEqual(result.DefinitionOfDoneIssues, wantIssues) {
		t.Fatalf("unexpected DoD issues: got %v want %v", result.DefinitionOfDoneIssues, wantIssues)
	}
	if len(repo.saveCalls) != 1 {
		t.Fatalf("expected one persisted run summary, got %d", len(repo.saveCalls))
	}
}

func TestReportCompletion_EmptyFilesChangedStrictRejects(t *testing.T) {
	repo := &fakeRepository{
		scopeResults: []core.ReceiptScope{{
			ProjectID: "project.alpha",
			ReceiptID: "receipt.abc123",
		}},
		workListResults: [][]core.WorkItem{{
			{ItemKey: "verify:tests", Status: core.WorkItemStatusComplete},
		}},
	}
	svc, err := New(repo)
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	result, apiErr := svc.ReportCompletion(context.Background(), v1.ReportCompletionPayload{
		ProjectID:    "project.alpha",
		ReceiptID:    "receipt.abc123",
		FilesChanged: []string{},
		Outcome:      "completed",
		ScopeMode:    v1.ScopeModeStrict,
	})
	if apiErr != nil {
		t.Fatalf("unexpected API error: %+v", apiErr)
	}
	if result.Accepted {
		t.Fatalf("expected strict-mode rejection: %+v", result)
	}
	wantIssues := []string{"files_changed must include at least one repository-relative path"}
	if !reflect.DeepEqual(result.DefinitionOfDoneIssues, wantIssues) {
		t.Fatalf("unexpected DoD issues: got %v want %v", result.DefinitionOfDoneIssues, wantIssues)
	}
	if len(repo.saveCalls) != 0 {
		t.Fatalf("expected no persisted run summary on strict rejection, got %d", len(repo.saveCalls))
	}
}

func TestReportCompletion_DefaultModeWarnAcceptsOutOfScopeAndPersistsSummary(t *testing.T) {
	repo := &fakeRepository{
		scopeResults: []core.ReceiptScope{{
			ProjectID:    "project.alpha",
			ReceiptID:    "receipt.abc123",
			TaskText:     "default warn scope check",
			Phase:        "execute",
			ResolvedTags: []string{"backend"},
			PointerKeys:  []string{"code:repo"},
			PointerPaths: []string{"src/allowed.go"},
		}},
		saveResult: core.RunReceiptIDs{RunID: 64, ReceiptID: "receipt.abc123"},
	}
	svc, err := New(repo)
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	result, apiErr := svc.ReportCompletion(context.Background(), v1.ReportCompletionPayload{
		ProjectID:    "project.alpha",
		ReceiptID:    "receipt.abc123",
		FilesChanged: []string{"src/allowed.go", "src/outside.go"},
		Outcome:      "completed",
	})
	if apiErr != nil {
		t.Fatalf("unexpected API error: %+v", apiErr)
	}
	if !result.Accepted {
		t.Fatalf("expected accepted result in default warn mode: %+v", result)
	}
	if result.RunID != 64 {
		t.Fatalf("unexpected run id: got %d want 64", result.RunID)
	}
	if len(result.Violations) != 1 || result.Violations[0].Path != "src/outside.go" {
		t.Fatalf("unexpected violations: %+v", result.Violations)
	}
	if len(repo.saveCalls) != 1 {
		t.Fatalf("expected one persisted run summary, got %d", len(repo.saveCalls))
	}
	if repo.saveCalls[0].Status != "accepted_with_warnings" {
		t.Fatalf("unexpected persisted status: %q", repo.saveCalls[0].Status)
	}
}

func TestReportCompletion_UnknownReceiptReturnsNotFound(t *testing.T) {
	repo := &fakeRepository{
		scopeErrors: []error{core.ErrReceiptScopeNotFound},
	}
	svc, err := New(repo)
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	_, apiErr := svc.ReportCompletion(context.Background(), v1.ReportCompletionPayload{
		ProjectID:    "project.alpha",
		ReceiptID:    "receipt.missing",
		FilesChanged: []string{"internal/core/repository.go"},
		Outcome:      "completed",
	})
	if apiErr == nil {
		t.Fatal("expected API error")
	}
	if apiErr.Code != "NOT_FOUND" {
		t.Fatalf("unexpected API error code: got %q want %q", apiErr.Code, "NOT_FOUND")
	}
	if len(repo.saveCalls) != 0 {
		t.Fatalf("did not expect save call, got %d", len(repo.saveCalls))
	}
}

func TestReportCompletion_FetchScopeErrorReturnsInternalError(t *testing.T) {
	repo := &fakeRepository{
		scopeErrors: []error{errors.New("db fetch failed")},
	}
	svc, err := New(repo)
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	_, apiErr := svc.ReportCompletion(context.Background(), v1.ReportCompletionPayload{
		ProjectID:    "project.alpha",
		ReceiptID:    "receipt.abc123",
		FilesChanged: []string{"internal/core/repository.go"},
		Outcome:      "completed",
	})
	if apiErr == nil {
		t.Fatal("expected API error")
	}
	if apiErr.Code != "INTERNAL_ERROR" {
		t.Fatalf("unexpected API error code: got %q want %q", apiErr.Code, "INTERNAL_ERROR")
	}
}

func TestReportCompletion_SaveErrorReturnsInternalError(t *testing.T) {
	repo := &fakeRepository{
		scopeResults: []core.ReceiptScope{{
			ProjectID:    "project.alpha",
			ReceiptID:    "receipt.abc123",
			PointerPaths: []string{"internal/core/repository.go"},
		}},
		saveError: errors.New("insert failed"),
	}
	svc, err := New(repo)
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	_, apiErr := svc.ReportCompletion(context.Background(), v1.ReportCompletionPayload{
		ProjectID:    "project.alpha",
		ReceiptID:    "receipt.abc123",
		FilesChanged: []string{"internal/core/repository.go"},
		Outcome:      "completed",
	})
	if apiErr == nil {
		t.Fatal("expected API error")
	}
	if apiErr.Code != "INTERNAL_ERROR" {
		t.Fatalf("unexpected API error code: got %q want %q", apiErr.Code, "INTERNAL_ERROR")
	}
}

func TestReportCompletion_NormalizesFilesChangedDeterministically(t *testing.T) {
	repo := &fakeRepository{
		scopeResults: []core.ReceiptScope{{
			ProjectID:    "project.alpha",
			ReceiptID:    "receipt.abc123",
			PointerPaths: []string{"docs/readme.md", "src/pkg/file.go"},
		}},
	}
	svc, err := New(repo)
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	result, apiErr := svc.ReportCompletion(context.Background(), v1.ReportCompletionPayload{
		ProjectID: "project.alpha",
		ReceiptID: "receipt.abc123",
		FilesChanged: []string{
			" ./src//pkg/./file.go ",
			"docs\\readme.md",
			"src/pkg/../pkg/file.go",
			"docs/readme.md",
			" ",
		},
		Outcome: "completed",
	})
	if apiErr != nil {
		t.Fatalf("unexpected API error: %+v", apiErr)
	}
	if !result.Accepted {
		t.Fatalf("expected accepted result: %+v", result)
	}
	if len(repo.saveCalls) != 1 {
		t.Fatalf("expected one save call, got %d", len(repo.saveCalls))
	}
	wantFiles := []string{"docs/readme.md", "src/pkg/file.go"}
	if !reflect.DeepEqual(repo.saveCalls[0].FilesChanged, wantFiles) {
		t.Fatalf("unexpected normalized files: got %v want %v", repo.saveCalls[0].FilesChanged, wantFiles)
	}
}

func TestSync_ChangedDefaultsAndDeterministicProcessedPaths(t *testing.T) {
	repo := &fakeRepository{
		syncResults: []core.SyncApplyResult{{
			Updated:            3,
			MarkedStale:        0,
			NewCandidates:      1,
			DeletedMarkedStale: 2,
		}},
	}
	svc, err := New(repo)
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	var gitCalls []string
	svc.runGitCommand = func(_ context.Context, projectRoot string, args ...string) (string, error) {
		call := projectRoot + "::" + strings.Join(args, " ")
		gitCalls = append(gitCalls, call)
		switch strings.Join(args, " ") {
		case "diff --name-status --find-renames HEAD~1..HEAD":
			return "M\t ./b//two.go \nR100\told/path.go\tnew\\\\path.go\nD\tz/delete.go\nA\ta/one.go\nM\ta/one.go\n", nil
		case "ls-tree -r HEAD -- a/one.go b/two.go new/path.go":
			return "" +
				"100644 blob aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa\ta/one.go\n" +
				"100644 blob bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb\tb/two.go\n" +
				"100644 blob cccccccccccccccccccccccccccccccccccccccc\tnew/path.go\n", nil
		default:
			t.Fatalf("unexpected git call: %s", call)
		}
		return "", nil
	}

	result, apiErr := svc.Sync(context.Background(), v1.SyncPayload{
		ProjectID: "project.alpha",
	})
	if apiErr != nil {
		t.Fatalf("unexpected API error: %+v", apiErr)
	}
	if len(gitCalls) != 2 {
		t.Fatalf("expected 2 git calls, got %d", len(gitCalls))
	}
	if gitCalls[0] != ".::diff --name-status --find-renames HEAD~1..HEAD" {
		t.Fatalf("unexpected first git call: %s", gitCalls[0])
	}
	if len(repo.syncCalls) != 1 {
		t.Fatalf("expected one apply sync call, got %d", len(repo.syncCalls))
	}
	call := repo.syncCalls[0]
	if call.Mode != "changed" {
		t.Fatalf("unexpected mode: %q", call.Mode)
	}
	if !call.InsertNewCandidates {
		t.Fatalf("expected insert_new_candidates default true")
	}

	wantPaths := []core.SyncPath{
		{Path: "a/one.go", ContentHash: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"},
		{Path: "b/two.go", ContentHash: "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"},
		{Path: "new/path.go", ContentHash: "cccccccccccccccccccccccccccccccccccccccc"},
		{Path: "old/path.go", Deleted: true},
		{Path: "z/delete.go", Deleted: true},
	}
	if !reflect.DeepEqual(call.Paths, wantPaths) {
		t.Fatalf("unexpected sync paths: got %#v want %#v", call.Paths, wantPaths)
	}

	wantProcessed := []string{"a/one.go", "b/two.go", "new/path.go", "old/path.go", "z/delete.go"}
	if !reflect.DeepEqual(result.ProcessedPaths, wantProcessed) {
		t.Fatalf("unexpected processed paths: got %v want %v", result.ProcessedPaths, wantProcessed)
	}
	if result.Updated != 3 || result.MarkedStale != 0 || result.NewCandidates != 3 || result.IndexedStubs != 3 || result.DeletedMarkedStale != 2 {
		t.Fatalf("unexpected result counts: %+v", result)
	}
	if len(repo.upsertStubCalls) != 1 {
		t.Fatalf("expected one stub upsert call, got %d", len(repo.upsertStubCalls))
	}
	if got := repo.upsertStubProjectIDs[0]; got != "project.alpha" {
		t.Fatalf("unexpected stub upsert project id: %q", got)
	}
	if got := len(repo.upsertStubCalls[0]); got != 3 {
		t.Fatalf("unexpected stub upsert count: %d", got)
	}
}

func TestSync_ExplicitInsertNewCandidatesFalseHonored(t *testing.T) {
	repo := &fakeRepository{
		syncResults: []core.SyncApplyResult{{Updated: 1}},
	}
	svc, err := New(repo)
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	var gitCalls []string
	svc.runGitCommand = func(_ context.Context, projectRoot string, args ...string) (string, error) {
		call := projectRoot + "::" + strings.Join(args, " ")
		gitCalls = append(gitCalls, call)
		switch strings.Join(args, " ") {
		case "diff --name-status --find-renames base..main":
			return "M\tsrc/main.go\n", nil
		case "ls-tree -r main -- src/main.go":
			return "100644 blob dddddddddddddddddddddddddddddddddddddddd\tsrc/main.go\n", nil
		default:
			t.Fatalf("unexpected git call: %s", call)
		}
		return "", nil
	}

	result, apiErr := svc.Sync(context.Background(), v1.SyncPayload{
		ProjectID:           "project.alpha",
		Mode:                "changed",
		GitRange:            "base..main",
		InsertNewCandidates: boolPtr(false),
	})
	if apiErr != nil {
		t.Fatalf("unexpected API error: %+v", apiErr)
	}
	if len(repo.syncCalls) != 1 {
		t.Fatalf("expected one apply sync call, got %d", len(repo.syncCalls))
	}
	if repo.syncCalls[0].InsertNewCandidates {
		t.Fatalf("expected explicit false insert_new_candidates to be honored")
	}
	if !reflect.DeepEqual(result.ProcessedPaths, []string{"src/main.go"}) {
		t.Fatalf("unexpected processed paths: %v", result.ProcessedPaths)
	}
	if result.NewCandidates != 0 || result.IndexedStubs != 0 {
		t.Fatalf("expected zero indexed stubs when disabled, got %+v", result)
	}
	if len(repo.upsertStubCalls) != 0 {
		t.Fatalf("did not expect stub upsert calls when disabled, got %d", len(repo.upsertStubCalls))
	}
	if len(gitCalls) != 2 || gitCalls[0] != ".::diff --name-status --find-renames base..main" {
		t.Fatalf("unexpected git calls: %v", gitCalls)
	}
}

func TestSync_FullModeMapsRepositoryCounters(t *testing.T) {
	repo := &fakeRepository{
		syncResults: []core.SyncApplyResult{{
			Updated:            7,
			MarkedStale:        3,
			NewCandidates:      2,
			DeletedMarkedStale: 0,
		}},
	}
	svc, err := New(repo)
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	svc.runGitCommand = func(_ context.Context, projectRoot string, args ...string) (string, error) {
		if projectRoot != "/repo/root" {
			t.Fatalf("unexpected project root: %s", projectRoot)
		}
		if strings.Join(args, " ") != "ls-tree -r HEAD" {
			t.Fatalf("unexpected git args: %v", args)
		}
		return "" +
			"100644 blob ffffffffffffffffffffffffffffffffffffffff\tz/file.go\n" +
			"100644 blob 1111111111111111111111111111111111111111\ta/file.go\n", nil
	}

	result, apiErr := svc.Sync(context.Background(), v1.SyncPayload{
		ProjectID:   "project.alpha",
		Mode:        "full",
		ProjectRoot: "/repo/root",
	})
	if apiErr != nil {
		t.Fatalf("unexpected API error: %+v", apiErr)
	}

	if len(repo.syncCalls) != 1 {
		t.Fatalf("expected one apply sync call, got %d", len(repo.syncCalls))
	}
	call := repo.syncCalls[0]
	if call.Mode != "full" {
		t.Fatalf("unexpected mode: %q", call.Mode)
	}
	wantPaths := []core.SyncPath{
		{Path: "a/file.go", ContentHash: "1111111111111111111111111111111111111111"},
		{Path: "z/file.go", ContentHash: "ffffffffffffffffffffffffffffffffffffffff"},
	}
	if !reflect.DeepEqual(call.Paths, wantPaths) {
		t.Fatalf("unexpected paths: got %#v want %#v", call.Paths, wantPaths)
	}
	if result.Updated != 7 || result.MarkedStale != 3 || result.NewCandidates != 2 || result.IndexedStubs != 2 || result.DeletedMarkedStale != 0 {
		t.Fatalf("unexpected result counters: %+v", result)
	}
	if !reflect.DeepEqual(result.ProcessedPaths, []string{"a/file.go", "z/file.go"}) {
		t.Fatalf("unexpected processed paths: %v", result.ProcessedPaths)
	}
	if len(repo.upsertStubCalls) != 1 || len(repo.upsertStubCalls[0]) != 2 {
		t.Fatalf("expected 2 stub upserts, got %+v", repo.upsertStubCalls)
	}
}

func TestSync_RepositoryErrorMapsInternalError(t *testing.T) {
	repo := &fakeRepository{
		syncErrors: []error{errors.New("apply failed")},
	}
	svc, err := New(repo)
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	svc.runGitCommand = func(_ context.Context, _ string, args ...string) (string, error) {
		switch strings.Join(args, " ") {
		case "diff --name-status --find-renames HEAD~1..HEAD":
			return "M\tsrc/main.go\n", nil
		case "ls-tree -r HEAD -- src/main.go":
			return "100644 blob 0123456789abcdef0123456789abcdef01234567\tsrc/main.go\n", nil
		default:
			t.Fatalf("unexpected git args: %v", args)
		}
		return "", nil
	}

	_, apiErr := svc.Sync(context.Background(), v1.SyncPayload{
		ProjectID: "project.alpha",
	})
	if apiErr == nil {
		t.Fatal("expected API error")
	}
	if apiErr.Code != "INTERNAL_ERROR" {
		t.Fatalf("unexpected API error code: got %q want INTERNAL_ERROR", apiErr.Code)
	}
	details, ok := apiErr.Details.(map[string]any)
	if !ok {
		t.Fatalf("expected map details, got %T", apiErr.Details)
	}
	if got := details["operation"]; got != "apply_sync" {
		t.Fatalf("unexpected operation detail: %#v", details)
	}
}

func TestHealthCheck_DefaultsDeterministicOrderingAndCapping(t *testing.T) {
	maxFindings := 1
	repo := &fakeRepository{
		candidateResults: [][]core.CandidatePointer{{
			{
				Key:         "ptr:one",
				Path:        "internal/a.go",
				Label:       "duplicate",
				Description: "",
				Tags:        []string{"backend", "BAD_TAG"},
				IsStale:     true,
			},
			{
				Key:         "ptr:two",
				Path:        "internal/b.go",
				Label:       "duplicate",
				Description: "ok",
				Tags:        []string{"backend"},
			},
			{
				Key:         "ptr:three",
				Path:        "internal/c.go",
				Label:       "unique",
				Description: "",
				Tags:        []string{"bad tag"},
			},
		}},
		memoryResults: [][]core.ActiveMemory{{
			{ID: 1, Confidence: 1, Tags: []string{"BadTag"}, RelatedPointerKeys: nil},
			{ID: 2, Confidence: 4, Tags: []string{"backend"}, RelatedPointerKeys: []string{"ptr:one"}},
		}},
		inventoryResults: []core.PointerInventory{
			{Path: "internal/a.go"},
			{Path: "internal/b.go"},
			{Path: "internal/c.go"},
		},
	}
	svc, err := New(repo)
	if err != nil {
		t.Fatalf("new service: %v", err)
	}
	svc.runGitCommand = func(_ context.Context, _ string, args ...string) (string, error) {
		if strings.Join(args, " ") != "ls-files --cached --others --exclude-standard" {
			t.Fatalf("unexpected git args: %v", args)
		}
		return "internal/a.go\ninternal/b.go\ninternal/c.go\n", nil
	}

	result, apiErr := svc.HealthCheck(context.Background(), v1.HealthCheckPayload{
		ProjectID:           "project.alpha",
		MaxFindingsPerCheck: &maxFindings,
	})
	if apiErr != nil {
		t.Fatalf("unexpected API error: %+v", apiErr)
	}

	if result.Summary.OK {
		t.Fatalf("expected summary not ok: %+v", result.Summary)
	}
	if result.Summary.TotalFindings != 10 {
		t.Fatalf("unexpected total findings: got %d want 10", result.Summary.TotalFindings)
	}

	wantOrder := []string{
		"duplicate_labels",
		"empty_descriptions",
		"orphan_relations",
		"pending_quarantines",
		"stale_pointers",
		"unindexed_files",
		"unknown_tags",
		"weak_memories",
	}
	gotOrder := make([]string, 0, len(result.Checks))
	for _, check := range result.Checks {
		gotOrder = append(gotOrder, check.Name)
		if len(check.Samples) > 1 {
			t.Fatalf("expected capped samples <=1 for %s, got %v", check.Name, check.Samples)
		}
	}
	if !reflect.DeepEqual(gotOrder, wantOrder) {
		t.Fatalf("unexpected check order: got %v want %v", gotOrder, wantOrder)
	}
	if len(result.Checks[0].Samples) == 0 {
		t.Fatalf("expected include_details default to include samples")
	}
	if len(repo.candidateCalls) != 1 || !repo.candidateCalls[0].Unbounded {
		t.Fatalf("expected unbounded candidate health query, got %+v", repo.candidateCalls)
	}
	if len(repo.memoryCalls) != 1 || !repo.memoryCalls[0].Unbounded {
		t.Fatalf("expected unbounded memory health query, got %+v", repo.memoryCalls)
	}
}

func TestStatus_ReportsMissingCanonicalSources(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".git"), 0o755); err != nil {
		t.Fatalf("mkdir .git: %v", err)
	}

	svc, err := NewWithRuntimeStatus(&fakeRepository{}, root, RuntimeStatusSnapshot{
		Backend:                "sqlite",
		SQLitePath:             filepath.Join(root, ".acm", "context.db"),
		UsesImplicitSQLitePath: true,
	})
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	result, apiErr := svc.Status(context.Background(), v1.StatusPayload{ProjectID: "project.alpha"})
	if apiErr != nil {
		t.Fatalf("unexpected API error: %+v", apiErr)
	}
	if result.Summary.Ready {
		t.Fatalf("expected missing sources to make status not ready")
	}
	if result.Project.Backend != "sqlite" {
		t.Fatalf("unexpected backend: %q", result.Project.Backend)
	}
	integrationIDs := make([]string, 0, len(result.Integrations))
	for _, integration := range result.Integrations {
		integrationIDs = append(integrationIDs, integration.ID)
	}
	for _, id := range []string{"starter-contract", "detailed-planning-enforcement", "verify-generic", "verify-go", "verify-python", "verify-rust", "verify-ts"} {
		if !containsString(integrationIDs, id) {
			t.Fatalf("expected integration %q in %+v", id, integrationIDs)
		}
	}
	missingCodes := make([]string, 0, len(result.Missing))
	for _, item := range result.Missing {
		missingCodes = append(missingCodes, item.Code)
	}
	for _, code := range []string{"rules_missing", "tags_missing", "tests_missing", "workflows_missing"} {
		if !containsString(missingCodes, code) {
			t.Fatalf("expected missing code %q in %+v", code, missingCodes)
		}
	}
}

func TestStatus_PreviewsRetrievalAndLoadedSources(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".git"), 0o755); err != nil {
		t.Fatalf("mkdir .git: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(root, ".acm"), 0o755); err != nil {
		t.Fatalf("mkdir .acm: %v", err)
	}
	files := map[string]string{
		".acm/acm-rules.yaml":     "version: acm.rules.v1\nrules:\n  - summary: Keep tests green\n",
		".acm/acm-tags.yaml":      "version: acm.tags.v1\ncanonical_tags:\n  backend:\n    - svc\n",
		".acm/acm-tests.yaml":     "version: acm.tests.v1\ndefaults:\n  cwd: .\n  timeout_sec: 120\ntests:\n  - id: smoke\n    summary: Run smoke tests\n    command:\n      argv: [\"go\", \"test\", \"./...\"]\n",
		".acm/acm-workflows.yaml": "version: acm.workflows.v1\ncompletion:\n  required_tasks:\n    - key: verify:tests\n",
	}
	for rel, contents := range files {
		full := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.WriteFile(full, []byte(contents), 0o644); err != nil {
			t.Fatalf("write %s: %v", rel, err)
		}
	}

	repo := &fakeRepository{
		candidateResults: [][]core.CandidatePointer{{
			candidate("rule:tests", ".acm/acm-rules.yaml", true, []string{"governance"}),
			candidate("code:status", "internal/service/backend/status.go", false, []string{"backend"}),
		}},
		memoryResults: [][]core.ActiveMemory{{}},
	}
	svc, err := NewWithRuntimeStatus(repo, root, RuntimeStatusSnapshot{
		Backend:                "sqlite",
		SQLitePath:             filepath.Join(root, ".acm", "context.db"),
		UsesImplicitSQLitePath: true,
	})
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	result, apiErr := svc.Status(context.Background(), v1.StatusPayload{
		ProjectID: "project.alpha",
		TaskText:  "diagnose why get_context chose these pointers",
		Phase:     v1.PhaseExecute,
	})
	if apiErr != nil {
		t.Fatalf("unexpected API error: %+v", apiErr)
	}
	if !result.Summary.Ready {
		t.Fatalf("expected ready status, got missing %+v", result.Missing)
	}
	if result.Retrieval == nil || result.Retrieval.Status != "ok" {
		t.Fatalf("expected retrieval preview, got %+v", result.Retrieval)
	}
	if len(result.Retrieval.SelectedPointers) != 2 {
		t.Fatalf("expected 2 selected pointers, got %+v", result.Retrieval.SelectedPointers)
	}
	sourceKinds := make([]string, 0, len(result.Sources))
	for _, source := range result.Sources {
		sourceKinds = append(sourceKinds, source.Kind)
	}
	for _, kind := range []string{"rules", "tags", "tests", "workflows"} {
		if !containsString(sourceKinds, kind) {
			t.Fatalf("expected source kind %q in %+v", kind, result.Sources)
		}
	}
	if got := repo.candidateCalls[0].Phase; got != string(v1.PhaseExecute) {
		t.Fatalf("unexpected retrieval phase: %q", got)
	}
}

func TestHealthCheck_IncludeDetailsFalseOmitsSamples(t *testing.T) {
	repo := &fakeRepository{
		candidateResults: [][]core.CandidatePointer{{
			{
				Key:         "ptr:one",
				Path:        "internal/a.go",
				Label:       "duplicate",
				Description: "",
				IsStale:     true,
			},
			{
				Key:         "ptr:two",
				Path:        "internal/b.go",
				Label:       "duplicate",
				Description: "",
			},
		}},
		inventoryResults: []core.PointerInventory{
			{Path: "internal/a.go"},
			{Path: "internal/b.go"},
		},
		memoryResults: [][]core.ActiveMemory{{}},
	}
	svc, err := New(repo)
	if err != nil {
		t.Fatalf("new service: %v", err)
	}
	svc.runGitCommand = func(_ context.Context, _ string, args ...string) (string, error) {
		if strings.Join(args, " ") != "ls-files --cached --others --exclude-standard" {
			t.Fatalf("unexpected git args: %v", args)
		}
		return "internal/a.go\ninternal/b.go\n", nil
	}

	includeDetails := false
	result, apiErr := svc.HealthCheck(context.Background(), v1.HealthCheckPayload{
		ProjectID:      "project.alpha",
		IncludeDetails: &includeDetails,
	})
	if apiErr != nil {
		t.Fatalf("unexpected API error: %+v", apiErr)
	}

	for _, check := range result.Checks {
		if len(check.Samples) != 0 {
			t.Fatalf("expected no samples when include_details=false, got check=%s samples=%v", check.Name, check.Samples)
		}
	}
}

func TestHealthCheck_EmptyIndexFlagsUnindexedFiles(t *testing.T) {
	repo := &fakeRepository{
		candidateResults: [][]core.CandidatePointer{{}},
		memoryResults:    [][]core.ActiveMemory{{}},
		inventoryResults: []core.PointerInventory{},
	}
	svc, err := New(repo)
	if err != nil {
		t.Fatalf("new service: %v", err)
	}
	svc.runGitCommand = func(_ context.Context, _ string, args ...string) (string, error) {
		if strings.Join(args, " ") != "ls-files --cached --others --exclude-standard" {
			t.Fatalf("unexpected git args: %v", args)
		}
		return "README.md\ninternal/service/backend/service.go\n", nil
	}

	result, apiErr := svc.HealthCheck(context.Background(), v1.HealthCheckPayload{
		ProjectID: "project.alpha",
	})
	if apiErr != nil {
		t.Fatalf("unexpected API error: %+v", apiErr)
	}
	if result.Summary.OK {
		t.Fatalf("expected non-ok summary for empty index: %+v", result.Summary)
	}
	var unindexed *v1.HealthCheckItem
	for i := range result.Checks {
		if result.Checks[i].Name == "unindexed_files" {
			unindexed = &result.Checks[i]
			break
		}
	}
	if unindexed == nil {
		t.Fatalf("expected unindexed_files check in %+v", result.Checks)
	}
	if unindexed.Count != 2 {
		t.Fatalf("unexpected unindexed count: got %d want 2", unindexed.Count)
	}
	if len(unindexed.Samples) == 0 {
		t.Fatalf("expected unindexed file samples, got %+v", unindexed)
	}
}

func TestHealthCheck_RepositoryErrorMapsInternalError(t *testing.T) {
	repo := &fakeRepository{
		candidateErrors: []error{errors.New("candidate query failed")},
	}
	svc, err := New(repo)
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	_, apiErr := svc.HealthCheck(context.Background(), v1.HealthCheckPayload{ProjectID: "project.alpha"})
	if apiErr == nil {
		t.Fatal("expected API error")
	}
	if apiErr.Code != "INTERNAL_ERROR" {
		t.Fatalf("unexpected error code: %s", apiErr.Code)
	}
	details, ok := apiErr.Details.(map[string]any)
	if !ok {
		t.Fatalf("expected map details, got %T", apiErr.Details)
	}
	if details["operation"] != "fetch_candidate_pointers" {
		t.Fatalf("unexpected operation detail: %#v", details)
	}
}

func TestEval_InlineSuiteDeterministicMetricsAndInsufficientContextHandling(t *testing.T) {
	repo := &fakeRepository{
		candidateResults: [][]core.CandidatePointer{
			{
				candidate("rule:a", "AGENTS.md", true, []string{"rules"}),
				candidate("code:x", "internal/x.go", false, []string{"backend"}),
			},
			{
				candidate("rule:only", "AGENTS.md", true, []string{"rules"}),
			},
			{
				candidate("rule:still-only", "AGENTS.md", true, []string{"rules"}),
			},
		},
		memoryResults: [][]core.ActiveMemory{
			{
				memory(10, "Memory A", "content", []string{"backend"}, []string{"code:x"}),
			},
		},
	}
	svc, err := New(repo)
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	result, apiErr := svc.Eval(context.Background(), v1.EvalPayload{
		ProjectID: "project.alpha",
		EvalSuiteInline: []v1.EvalCase{
			{
				TaskText:               "case one",
				Phase:                  v1.PhaseExecute,
				ExpectedPointerKeys:    []string{"rule:a"},
				ExpectedMemorySubjects: []string{"Memory A"},
			},
			{
				TaskText:            "case two",
				Phase:               v1.PhasePlan,
				ExpectedPointerKeys: []string{"rule:missing"},
			},
		},
	})
	if apiErr != nil {
		t.Fatalf("unexpected API error: %+v", apiErr)
	}

	if result.TotalCases != 2 {
		t.Fatalf("unexpected total cases: %d", result.TotalCases)
	}
	if result.MinimumRecall != 0.8 {
		t.Fatalf("expected default minimum recall 0.8, got %v", result.MinimumRecall)
	}
	if result.Pass {
		t.Fatalf("expected pass=false, got %+v", result)
	}

	if len(result.Cases) != 2 {
		t.Fatalf("unexpected case count: %d", len(result.Cases))
	}
	if result.Cases[0].Precision != 0.666667 || result.Cases[0].Recall != 1 || result.Cases[0].F1 != 0.8 {
		t.Fatalf("unexpected metrics for case 0: %+v", result.Cases[0])
	}
	if result.Cases[1].Recall != 0 || result.Cases[1].F1 != 0 {
		t.Fatalf("unexpected metrics for case 1: %+v", result.Cases[1])
	}
	if result.Cases[1].Notes != "insufficient_context" {
		t.Fatalf("expected insufficient_context note, got %q", result.Cases[1].Notes)
	}
	if result.Aggregate.Precision != 0.666667 || result.Aggregate.Recall != 0.666667 || result.Aggregate.F1 != 0.666667 {
		t.Fatalf("unexpected aggregate metrics: %+v", result.Aggregate)
	}
}

func TestEval_FileSuiteAndThresholdBehavior(t *testing.T) {
	tmpDir := t.TempDir()
	suitePath := filepath.Join(tmpDir, "suite.json")
	content := `[{"task_text":"file suite case","phase":"execute","expected_pointer_keys":["rule:a"]}]`
	if err := os.WriteFile(suitePath, []byte(content), 0o644); err != nil {
		t.Fatalf("write suite file: %v", err)
	}

	minimumRecall := 0.5
	repo := &fakeRepository{
		candidateResults: [][]core.CandidatePointer{{
			candidate("rule:a", "AGENTS.md", true, []string{"rules"}),
			candidate("code:x", "internal/x.go", false, []string{"backend"}),
		}},
		memoryResults: [][]core.ActiveMemory{{}},
	}
	svc, err := New(repo)
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	result, apiErr := svc.Eval(context.Background(), v1.EvalPayload{
		ProjectID:     "project.alpha",
		EvalSuitePath: suitePath,
		MinimumRecall: &minimumRecall,
	})
	if apiErr != nil {
		t.Fatalf("unexpected API error: %+v", apiErr)
	}

	if result.TotalCases != 1 {
		t.Fatalf("unexpected total cases: %d", result.TotalCases)
	}
	if result.MinimumRecall != 0.5 {
		t.Fatalf("unexpected minimum_recall: %v", result.MinimumRecall)
	}
	if !result.Pass {
		t.Fatalf("expected pass=true, got %+v", result)
	}
}

func TestVerify_DryRunSelectsDeterministicallyWithoutPersistence(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".acm"), 0o755); err != nil {
		t.Fatalf("mkdir .acm: %v", err)
	}
	testsYAML := `version: acm.tests.v1
defaults:
  cwd: .
  timeout_sec: 45
tests:
  - id: alpha-unit
    summary: Run alpha verification
    command:
      argv: ["go", "test", "./internal/..."]
    select:
      phases: ["review"]
      tags_any: ["backend"]
      changed_paths_any: ["internal/**"]
    expected:
      exit_code: 0
  - id: beta-pointer
    summary: Run pointer verification
    command:
      argv: ["acm", "eval", "--project", "project.alpha", "--eval-suite-path", ".acm/eval.json"]
    select:
      pointer_keys_any: ["code:repo"]
    expected:
      exit_code: 0
  - id: gamma-smoke
    summary: Repo smoke test
    command:
      argv: ["echo", "noop"]
      env:
        ACM_VERIFY_TEST_MODE: smoke
    select:
      always_run: true
  - id: delta-unscoped
    summary: Unscoped test should not auto-select
    command:
      argv: ["echo", "noop"]
`
	if err := os.WriteFile(filepath.Join(root, ".acm", "acm-tests.yaml"), []byte(testsYAML), 0o644); err != nil {
		t.Fatalf("write tests file: %v", err)
	}

	withWorkingDir(t, root)

	repo := &fakeRepository{
		scopeResults: []core.ReceiptScope{{
			ProjectID:    "project.alpha",
			ReceiptID:    "receipt.abc123",
			Phase:        "execute",
			ResolvedTags: []string{"backend"},
			PointerKeys:  []string{"code:repo"},
		}},
	}
	svc, err := New(repo)
	if err != nil {
		t.Fatalf("new service: %v", err)
	}
	svc.runVerifyCommand = func(_ context.Context, _ string, _ verifyTestDefinition, _ map[string]string) verifyCommandRun {
		t.Fatal("runVerifyCommand should not be called for dry-run")
		return verifyCommandRun{}
	}

	result, apiErr := svc.Verify(context.Background(), v1.VerifyPayload{
		ProjectID:    "project.alpha",
		ReceiptID:    "receipt.abc123",
		Phase:        v1.PhaseReview,
		FilesChanged: []string{"internal/service/backend/service.go"},
		DryRun:       true,
	})
	if apiErr != nil {
		t.Fatalf("unexpected API error: %+v", apiErr)
	}
	if result.Status != v1.VerifyStatusDryRun {
		t.Fatalf("unexpected status: got %q want %q", result.Status, v1.VerifyStatusDryRun)
	}
	if got, want := result.SelectedTestIDs, []string{"alpha-unit", "beta-pointer", "gamma-smoke"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected selected test ids: got %v want %v", got, want)
	}
	if len(result.Selected) != 3 || result.Selected[0].TestID != "alpha-unit" || result.Selected[1].TestID != "beta-pointer" || result.Selected[2].TestID != "gamma-smoke" {
		t.Fatalf("unexpected selected results: %+v", result.Selected)
	}
	if got, want := result.Selected[2].SelectionReasons, []string{"always_run=true"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected always_run selection reasons: got %v want %v", got, want)
	}
	if result.Passed {
		t.Fatalf("expected passed=false for dry run, got %+v", result.Passed)
	}
	if len(result.Results) != 0 {
		t.Fatalf("expected no executed results on dry run, got %+v", result.Results)
	}
	if result.BatchRunID != "" {
		t.Fatalf("expected no batch run id on dry run, got %q", result.BatchRunID)
	}
	if len(repo.verifySaveCalls) != 0 {
		t.Fatalf("expected no persisted verification batches, got %d", len(repo.verifySaveCalls))
	}
	if len(repo.workUpsertCalls) != 0 {
		t.Fatalf("expected no work upserts on dry run, got %d", len(repo.workUpsertCalls))
	}
}

func TestVerify_ExecutesPersistsAndUpdatesWork(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".acm"), 0o755); err != nil {
		t.Fatalf("mkdir .acm: %v", err)
	}
	testsYAML := `version: acm.tests.v1
defaults:
  cwd: .
  timeout_sec: 45
tests:
  - id: alpha-unit
    summary: Run alpha verification
    command:
      argv: ["go", "test", "./internal/..."]
    select:
      phases: ["review"]
      tags_any: ["backend"]
      changed_paths_any: ["internal/**"]
    expected:
      exit_code: 0
  - id: beta-pointer
    summary: Run pointer verification
    command:
      argv: ["acm", "eval", "--project", "project.alpha", "--eval-suite-path", ".acm/eval.json"]
      timeout_sec: 60
      env:
        ACM_VERIFY_PROFILE: pointer
    select:
      pointer_keys_any: ["code:repo"]
    expected:
      exit_code: 0
  - id: gamma-smoke
    summary: Run smoke verification
    command:
      argv: ["echo", "noop"]
    select:
      always_run: true
`
	if err := os.WriteFile(filepath.Join(root, ".acm", "acm-tests.yaml"), []byte(testsYAML), 0o644); err != nil {
		t.Fatalf("write tests file: %v", err)
	}

	withWorkingDir(t, root)

	repo := &fakeRepository{
		scopeResults: []core.ReceiptScope{{
			ProjectID:    "project.alpha",
			ReceiptID:    "receipt.abc123",
			Phase:        "execute",
			ResolvedTags: []string{"backend"},
			PointerKeys:  []string{"code:repo"},
		}},
		workUpsertResults: []int{1},
	}
	svc, err := New(repo)
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	base := time.Unix(1_700_000_000, 0).UTC()
	svc.runVerifyCommand = func(_ context.Context, _ string, def verifyTestDefinition, extraEnv map[string]string) verifyCommandRun {
		switch def.ID {
		case "alpha-unit":
			exitCode := 0
			return verifyCommandRun{
				ExitCode:   &exitCode,
				Stdout:     "alpha ok\n",
				StartedAt:  base,
				FinishedAt: base.Add(2 * time.Second),
			}
		case "beta-pointer":
			if got := def.Env["ACM_VERIFY_PROFILE"]; got != "pointer" {
				t.Fatalf("expected verify env for beta-pointer, got %q", got)
			}
			if got := extraEnv["ACM_RECEIPT_ID"]; got != "receipt.abc123" {
				t.Fatalf("unexpected injected ACM_RECEIPT_ID: %+v", extraEnv)
			}
			if got := extraEnv["ACM_PLAN_KEY"]; got != "plan:receipt.abc123" {
				t.Fatalf("unexpected injected ACM_PLAN_KEY: %+v", extraEnv)
			}
			exitCode := 1
			return verifyCommandRun{
				ExitCode:   &exitCode,
				Stderr:     "aggregate recall below threshold\n",
				StartedAt:  base.Add(3 * time.Second),
				FinishedAt: base.Add(4 * time.Second),
				Err:        errors.New("exit status 1"),
			}
		case "gamma-smoke":
			exitCode := 0
			return verifyCommandRun{
				ExitCode:   &exitCode,
				Stdout:     "smoke ok\n",
				StartedAt:  base.Add(5 * time.Second),
				FinishedAt: base.Add(6 * time.Second),
			}
		default:
			t.Fatalf("unexpected test id: %s", def.ID)
			return verifyCommandRun{}
		}
	}

	result, apiErr := svc.Verify(context.Background(), v1.VerifyPayload{
		ProjectID: "project.alpha",
		ReceiptID: "receipt.abc123",
		PlanKey:   "plan:receipt.abc123",
		Phase:     v1.PhaseReview,
		FilesChanged: []string{
			"internal/service/backend/service.go",
		},
	})
	if apiErr != nil {
		t.Fatalf("unexpected API error: %+v", apiErr)
	}
	if result.Status != v1.VerifyStatusFailed {
		t.Fatalf("unexpected status: got %q want %q", result.Status, v1.VerifyStatusFailed)
	}
	if result.Passed {
		t.Fatalf("expected passed=false, got %+v", result.Passed)
	}
	if result.BatchRunID == "" {
		t.Fatal("expected batch run id")
	}
	if got, want := result.SelectedTestIDs, []string{"alpha-unit", "beta-pointer", "gamma-smoke"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected selected test ids: got %v want %v", got, want)
	}
	if len(result.Results) != 3 {
		t.Fatalf("unexpected result count: %d", len(result.Results))
	}
	if result.Results[0].Status != v1.VerifyTestStatusPassed || result.Results[1].Status != v1.VerifyTestStatusFailed || result.Results[2].Status != v1.VerifyTestStatusPassed {
		t.Fatalf("unexpected verify results: %+v", result.Results)
	}
	if len(repo.verifySaveCalls) != 1 {
		t.Fatalf("expected one persisted verification batch, got %d", len(repo.verifySaveCalls))
	}

	saved := repo.verifySaveCalls[0]
	if saved.Status != "failed" || saved.TestsSourcePath != ".acm/acm-tests.yaml" {
		t.Fatalf("unexpected persisted batch: %+v", saved)
	}
	if saved.ReceiptID != "receipt.abc123" || saved.PlanKey != "plan:receipt.abc123" {
		t.Fatalf("unexpected persisted verify scope: %+v", saved)
	}
	if got, want := saved.SelectedTestIDs, []string{"alpha-unit", "beta-pointer", "gamma-smoke"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected persisted selected test ids: got %v want %v", got, want)
	}
	if len(saved.Results) != 3 {
		t.Fatalf("unexpected persisted results: %+v", saved.Results)
	}
	if saved.Results[0].TimeoutSec != 45 {
		t.Fatalf("expected default timeout inheritance, got %d", saved.Results[0].TimeoutSec)
	}
	if saved.Results[1].TimeoutSec != 60 {
		t.Fatalf("expected explicit timeout 60, got %d", saved.Results[1].TimeoutSec)
	}
	if len(repo.workUpsertCalls) != 1 {
		t.Fatalf("expected one work upsert call, got %d", len(repo.workUpsertCalls))
	}
	if got := repo.workUpsertCalls[0].Items[0].ItemKey; got != "verify:tests" {
		t.Fatalf("unexpected work item key: %q", got)
	}
	if got := repo.workUpsertCalls[0].Items[0].Status; got != core.WorkItemStatusBlocked {
		t.Fatalf("unexpected work item status: %q", got)
	}
	if got := repo.workUpsertCalls[0].Items[0].Outcome; !strings.Contains(got, "2/3 verification tests passed") {
		t.Fatalf("unexpected work outcome: %q", got)
	}
	if got := repo.workUpsertCalls[0].Items[0].Evidence; len(got) != 4 || got[0] != "verifyrun:"+result.BatchRunID {
		t.Fatalf("unexpected work evidence: %v", got)
	}
}

func TestVerify_WorkEvidenceIsCapped(t *testing.T) {
	repo := &fakeRepository{
		workUpsertResults: []int{1},
	}
	svc, err := New(repo)
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	results := make([]v1.VerifyTestResult, 0, 256)
	for i := 0; i < 256; i++ {
		results = append(results, v1.VerifyTestResult{TestID: fmt.Sprintf("test-%03d", i)})
	}

	apiErr := svc.updateVerifyWork(context.Background(), "project.alpha", verifySelectionContext{
		ReceiptID: "receipt.abc123",
		PlanKey:   "plan:receipt.abc123",
	}, "verify-batch-1", results, true)
	if apiErr != nil {
		t.Fatalf("unexpected API error: %+v", apiErr)
	}
	if len(repo.workUpsertCalls) != 1 {
		t.Fatalf("expected one work upsert call, got %d", len(repo.workUpsertCalls))
	}
	got := repo.workUpsertCalls[0].Items[0].Evidence
	if len(got) != maxVerifyWorkEvidenceEntries {
		t.Fatalf("unexpected evidence count: got %d want %d", len(got), maxVerifyWorkEvidenceEntries)
	}
	if got[0] != "verifyrun:verify-batch-1" {
		t.Fatalf("unexpected first evidence entry: %q", got[0])
	}
}

func TestVerify_InjectsDerivedPlanKeyIntoCommandEnv(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".acm"), 0o755); err != nil {
		t.Fatalf("mkdir .acm: %v", err)
	}
	testsYAML := `version: acm.tests.v1
tests:
  - id: smoke
    summary: Run smoke verification
    command:
      argv: ["echo", "noop"]
    select:
      always_run: true
`
	if err := os.WriteFile(filepath.Join(root, ".acm", "acm-tests.yaml"), []byte(testsYAML), 0o644); err != nil {
		t.Fatalf("write tests file: %v", err)
	}

	withWorkingDir(t, root)

	repo := &fakeRepository{
		scopeResults: []core.ReceiptScope{{
			ProjectID: "project.alpha",
			ReceiptID: "receipt.abc123",
			Phase:     "execute",
		}},
		workUpsertResults: []int{1},
	}
	svc, err := New(repo)
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	var gotEnv map[string]string
	svc.runVerifyCommand = func(_ context.Context, _ string, _ verifyTestDefinition, extraEnv map[string]string) verifyCommandRun {
		gotEnv = make(map[string]string, len(extraEnv))
		for key, value := range extraEnv {
			gotEnv[key] = value
		}
		exitCode := 0
		now := time.Now().UTC()
		return verifyCommandRun{
			ExitCode:   &exitCode,
			Stdout:     "smoke ok\n",
			StartedAt:  now,
			FinishedAt: now.Add(100 * time.Millisecond),
		}
	}

	result, apiErr := svc.Verify(context.Background(), v1.VerifyPayload{
		ProjectID: "project.alpha",
		ReceiptID: "receipt.abc123",
	})
	if apiErr != nil {
		t.Fatalf("unexpected API error: %+v", apiErr)
	}
	if result.Status != v1.VerifyStatusPassed || !result.Passed {
		t.Fatalf("unexpected verify result: %+v", result)
	}
	if gotEnv["ACM_RECEIPT_ID"] != "receipt.abc123" {
		t.Fatalf("unexpected injected ACM_RECEIPT_ID: %+v", gotEnv)
	}
	if gotEnv["ACM_PLAN_KEY"] != "plan:receipt.abc123" {
		t.Fatalf("unexpected derived ACM_PLAN_KEY: %+v", gotEnv)
	}
}

func TestVerify_RejectsAlwaysRunCombinedWithSelectors(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".acm"), 0o755); err != nil {
		t.Fatalf("mkdir .acm: %v", err)
	}
	testsYAML := `version: acm.tests.v1
defaults:
  cwd: .
  timeout_sec: 45
tests:
  - id: invalid-default
    summary: Invalid always run selector mix
    command:
      argv: ["echo", "noop"]
    select:
      always_run: true
      phases: ["review"]
`
	if err := os.WriteFile(filepath.Join(root, ".acm", "acm-tests.yaml"), []byte(testsYAML), 0o644); err != nil {
		t.Fatalf("write tests file: %v", err)
	}

	withWorkingDir(t, root)

	repo := &fakeRepository{
		scopeResults: []core.ReceiptScope{{
			ProjectID:    "project.alpha",
			ReceiptID:    "receipt.abc123",
			PointerPaths: []string{},
		}},
	}
	svc, err := New(repo)
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	_, apiErr := svc.Verify(context.Background(), v1.VerifyPayload{
		ProjectID: "project.alpha",
		Phase:     v1.PhaseReview,
	})
	if apiErr == nil {
		t.Fatal("expected API error")
	}
	if apiErr.Code != "INVALID_INPUT" {
		t.Fatalf("unexpected error code: %s", apiErr.Code)
	}
}

func TestResolveProjectSourcePath_PreservesAbsoluteOverrides(t *testing.T) {
	projectRoot := filepath.Join(t.TempDir(), "repo")
	absolute := filepath.Join(t.TempDir(), "acm-tests.yaml")

	sourcePath, absolutePath, err := resolveProjectSourcePath(projectRoot, absolute)
	if err != nil {
		t.Fatalf("resolveProjectSourcePath returned error: %v", err)
	}
	if sourcePath != filepath.ToSlash(filepath.Clean(absolute)) {
		t.Fatalf("unexpected source path: got %q want %q", sourcePath, filepath.ToSlash(filepath.Clean(absolute)))
	}
	if absolutePath != filepath.Clean(absolute) {
		t.Fatalf("unexpected absolute path: got %q want %q", absolutePath, filepath.Clean(absolute))
	}
}

func TestRunVerifyCommand_AppliesCommandEnv(t *testing.T) {
	root := t.TempDir()
	def := verifyTestDefinition{
		Argv:       []string{os.Args[0], "-test.run=TestRunVerifyCommandHelperProcess", "--"},
		CWD:        ".",
		TimeoutSec: 5,
		Env: map[string]string{
			"GO_WANT_VERIFY_HELPER_PROCESS": "1",
			"ACM_VERIFY_ENV_CHECK":          "expected-value",
		},
	}

	run := runVerifyCommand(context.Background(), root, def, map[string]string{
		"ACM_RECEIPT_ID": "receipt.abc123",
		"ACM_PLAN_KEY":   "plan:receipt.abc123",
	})
	if run.Err != nil {
		t.Fatalf("unexpected command error: %v\nstdout=%q\nstderr=%q", run.Err, run.Stdout, run.Stderr)
	}
	if run.ExitCode == nil || *run.ExitCode != 0 {
		t.Fatalf("unexpected exit code: %+v", run.ExitCode)
	}
}

func TestRunVerifyCommand_LoadsDotEnvBackedACMRuntimeEnv(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, ".env"), []byte("ACM_PG_DSN=postgres://dotenv\n"), 0o644); err != nil {
		t.Fatalf("write .env: %v", err)
	}

	def := verifyTestDefinition{
		Argv:       []string{os.Args[0], "-test.run=TestRunACMCommandDotEnvHelperProcess", "--"},
		CWD:        ".",
		TimeoutSec: 5,
		Env: map[string]string{
			"GO_WANT_ACM_COMMAND_DOTENV_HELPER_PROCESS": "1",
			"ACM_EXPECTED_PG_DSN":                       "postgres://dotenv",
			"ACM_EXPECTED_RECEIPT_ID":                   "receipt.abc123",
			"ACM_EXPECTED_PLAN_KEY":                     "plan:receipt.abc123",
		},
	}

	run := runVerifyCommand(context.Background(), root, def, map[string]string{
		"ACM_RECEIPT_ID": "receipt.abc123",
		"ACM_PLAN_KEY":   "plan:receipt.abc123",
	})
	if run.Err != nil {
		t.Fatalf("unexpected command error: %v\nstdout=%q\nstderr=%q", run.Err, run.Stdout, run.Stderr)
	}
	if run.ExitCode == nil || *run.ExitCode != 0 {
		t.Fatalf("unexpected exit code: %+v", run.ExitCode)
	}
}

func TestRunVerifyCommand_CommandEnvOverridesDotEnvBackedACMRuntimeEnv(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, ".env"), []byte("ACM_PG_DSN=postgres://dotenv\n"), 0o644); err != nil {
		t.Fatalf("write .env: %v", err)
	}

	def := verifyTestDefinition{
		Argv:       []string{os.Args[0], "-test.run=TestRunACMCommandDotEnvHelperProcess", "--"},
		CWD:        ".",
		TimeoutSec: 5,
		Env: map[string]string{
			"GO_WANT_ACM_COMMAND_DOTENV_HELPER_PROCESS": "1",
			"ACM_PG_DSN":          "postgres://command",
			"ACM_EXPECTED_PG_DSN": "postgres://command",
		},
	}

	run := runVerifyCommand(context.Background(), root, def, nil)
	if run.Err != nil {
		t.Fatalf("unexpected command error: %v\nstdout=%q\nstderr=%q", run.Err, run.Stdout, run.Stderr)
	}
	if run.ExitCode == nil || *run.ExitCode != 0 {
		t.Fatalf("unexpected exit code: %+v", run.ExitCode)
	}
}

func TestRunWorkflowReviewCommand_LoadsDotEnvBackedACMRuntimeEnv(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, ".env"), []byte("ACM_PG_DSN=postgres://dotenv\n"), 0o644); err != nil {
		t.Fatalf("write .env: %v", err)
	}

	command := workflowRunDefinition{
		Argv:       []string{os.Args[0], "-test.run=TestRunACMCommandDotEnvHelperProcess", "--"},
		CWD:        ".",
		TimeoutSec: 5,
		Env: map[string]string{
			"GO_WANT_ACM_COMMAND_DOTENV_HELPER_PROCESS": "1",
			"ACM_EXPECTED_PG_DSN":                       "postgres://dotenv",
			"ACM_EXPECTED_REVIEW_KEY":                   "review:cross-llm",
			"ACM_EXPECTED_PLAN_KEY":                     "plan:receipt.abc123",
		},
	}

	run := runWorkflowReviewCommand(context.Background(), root, command, map[string]string{
		"ACM_REVIEW_KEY": "review:cross-llm",
		"ACM_PLAN_KEY":   "plan:receipt.abc123",
	})
	if run.Err != nil {
		t.Fatalf("unexpected command error: %v\nstdout=%q\nstderr=%q", run.Err, run.Stdout, run.Stderr)
	}
	if run.ExitCode == nil || *run.ExitCode != 0 {
		t.Fatalf("unexpected exit code: %+v", run.ExitCode)
	}
}

func TestRunVerifyCommandHelperProcess(t *testing.T) {
	if os.Getenv("GO_WANT_VERIFY_HELPER_PROCESS") != "1" {
		return
	}
	if got, want := os.Getenv("ACM_VERIFY_ENV_CHECK"), "expected-value"; got != want {
		fmt.Fprintf(os.Stderr, "unexpected env: got %q want %q\n", got, want)
		os.Exit(3)
	}
	if got, want := os.Getenv("ACM_RECEIPT_ID"), "receipt.abc123"; got != want {
		fmt.Fprintf(os.Stderr, "unexpected receipt env: got %q want %q\n", got, want)
		os.Exit(3)
	}
	if got, want := os.Getenv("ACM_PLAN_KEY"), "plan:receipt.abc123"; got != want {
		fmt.Fprintf(os.Stderr, "unexpected plan env: got %q want %q\n", got, want)
		os.Exit(3)
	}
	fmt.Fprintln(os.Stdout, "env ok")
	os.Exit(0)
}

func TestRunACMCommandDotEnvHelperProcess(t *testing.T) {
	if os.Getenv("GO_WANT_ACM_COMMAND_DOTENV_HELPER_PROCESS") != "1" {
		return
	}

	checks := map[string]string{
		"ACM_PG_DSN":     os.Getenv("ACM_EXPECTED_PG_DSN"),
		"ACM_RECEIPT_ID": os.Getenv("ACM_EXPECTED_RECEIPT_ID"),
		"ACM_PLAN_KEY":   os.Getenv("ACM_EXPECTED_PLAN_KEY"),
		"ACM_REVIEW_KEY": os.Getenv("ACM_EXPECTED_REVIEW_KEY"),
	}
	for key, want := range checks {
		if want == "" {
			continue
		}
		if got := os.Getenv(key); got != want {
			fmt.Fprintf(os.Stderr, "unexpected %s: got %q want %q\n", key, got, want)
			os.Exit(3)
		}
	}

	fmt.Fprintln(os.Stdout, "dotenv env ok")
	os.Exit(0)
}

func TestEval_LoadSuiteErrorMapsInternalError(t *testing.T) {
	repo := &fakeRepository{
		scopeResults: []core.ReceiptScope{{
			ProjectID: "project.alpha",
			ReceiptID: "receipt.abc123",
		}},
	}
	svc, err := New(repo)
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	_, apiErr := svc.Eval(context.Background(), v1.EvalPayload{
		ProjectID:     "project.alpha",
		EvalSuitePath: filepath.Join(t.TempDir(), "missing.json"),
	})
	if apiErr == nil {
		t.Fatal("expected API error")
	}
	if apiErr.Code != "INTERNAL_ERROR" {
		t.Fatalf("unexpected error code: %s", apiErr.Code)
	}
	details, ok := apiErr.Details.(map[string]any)
	if !ok {
		t.Fatalf("expected map details, got %T", apiErr.Details)
	}
	if details["operation"] != "load_eval_suite" {
		t.Fatalf("unexpected operation detail: %#v", details)
	}
}

func TestBootstrap_DefaultEphemeralAndDeterministicEnumeration(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "dir"), 0o755); err != nil {
		t.Fatalf("mkdir dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "a.txt"), []byte("a"), 0o644); err != nil {
		t.Fatalf("write a.txt: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "dir", "b.go"), []byte("package dir"), 0o644); err != nil {
		t.Fatalf("write b.go: %v", err)
	}

	respectGitIgnore := false
	repo := &fakeRepository{
		scopeResults: []core.ReceiptScope{{
			ProjectID: "project.alpha",
			ReceiptID: "receipt.abc123",
		}},
	}
	svc, err := New(repo)
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	result, apiErr := svc.Bootstrap(context.Background(), v1.BootstrapPayload{
		ProjectID:        "project.alpha",
		ProjectRoot:      root,
		RespectGitIgnore: &respectGitIgnore,
	})
	if apiErr != nil {
		t.Fatalf("unexpected API error: %+v", apiErr)
	}

	if result.CandidatesPersisted {
		t.Fatalf("expected candidates_persisted=false by default")
	}
	if result.OutputCandidatesPath != "" {
		t.Fatalf("expected empty output path when not persisting, got %q", result.OutputCandidatesPath)
	}
	if result.CandidateCount != 2 {
		t.Fatalf("unexpected candidate count: got %d want 2", result.CandidateCount)
	}
	if result.IndexedStubs != 2 {
		t.Fatalf("unexpected indexed stub count: got %d want 2", result.IndexedStubs)
	}
	if len(repo.upsertStubCalls) != 1 || len(repo.upsertStubCalls[0]) != 2 {
		t.Fatalf("expected 2 stub upserts, got %+v", repo.upsertStubCalls)
	}
	defaultPersistPath := filepath.Join(root, ".acm", "bootstrap_candidates.json")
	if _, err := os.Stat(defaultPersistPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected no persisted candidates file by default, stat err=%v", err)
	}

	again, apiErr := svc.Bootstrap(context.Background(), v1.BootstrapPayload{
		ProjectID:        "project.alpha",
		ProjectRoot:      root,
		RespectGitIgnore: &respectGitIgnore,
	})
	if apiErr != nil {
		t.Fatalf("unexpected API error on second run: %+v", apiErr)
	}
	if again.CandidateCount != 2 {
		t.Fatalf("expected deterministic candidate count across runs, got %d", again.CandidateCount)
	}
	if again.IndexedStubs != 2 {
		t.Fatalf("expected deterministic indexed stub count across runs, got %d", again.IndexedStubs)
	}
}

func TestBootstrap_PersistCandidatesWritesDefaultAcmPath(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "dir"), 0o755); err != nil {
		t.Fatalf("mkdir dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "a.txt"), []byte("a"), 0o644); err != nil {
		t.Fatalf("write a.txt: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "dir", "b.go"), []byte("package dir"), 0o644); err != nil {
		t.Fatalf("write b.go: %v", err)
	}

	persist := true
	respectGitIgnore := false
	repo := &fakeRepository{
		scopeResults: []core.ReceiptScope{{
			ProjectID: "project.alpha",
			ReceiptID: "receipt.abc123",
		}},
	}
	svc, err := New(repo)
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	result, apiErr := svc.Bootstrap(context.Background(), v1.BootstrapPayload{
		ProjectID:         "project.alpha",
		ProjectRoot:       root,
		PersistCandidates: &persist,
		RespectGitIgnore:  &respectGitIgnore,
	})
	if apiErr != nil {
		t.Fatalf("unexpected API error: %+v", apiErr)
	}

	outputPath := filepath.Join(root, ".acm", "bootstrap_candidates.json")
	if !result.CandidatesPersisted {
		t.Fatalf("expected candidates_persisted=true")
	}
	if result.OutputCandidatesPath != outputPath {
		t.Fatalf("unexpected output path: got %q want %q", result.OutputCandidatesPath, outputPath)
	}
	if result.CandidateCount != 2 {
		t.Fatalf("unexpected candidate count: got %d want 2", result.CandidateCount)
	}
	if result.IndexedStubs != 2 {
		t.Fatalf("unexpected indexed stub count: got %d want 2", result.IndexedStubs)
	}

	raw, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("read output file: %v", err)
	}
	var parsed struct {
		Candidates []string `json:"candidates"`
	}
	if err := json.Unmarshal(raw, &parsed); err != nil {
		t.Fatalf("parse output file: %v", err)
	}
	wantCandidates := []string{"a.txt", "dir/b.go"}
	if !reflect.DeepEqual(parsed.Candidates, wantCandidates) {
		t.Fatalf("unexpected candidates: got %v want %v", parsed.Candidates, wantCandidates)
	}
}

func TestBootstrap_CustomOutputPathAndWarningsDeterministic(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "a.txt"), []byte("a"), 0o644); err != nil {
		t.Fatalf("write a.txt: %v", err)
	}

	output := "reports/candidates.json"
	repo := &fakeRepository{
		scopeResults: []core.ReceiptScope{{
			ProjectID: "project.alpha",
			ReceiptID: "receipt.abc123",
		}},
	}
	svc, err := New(repo)
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	result, apiErr := svc.Bootstrap(context.Background(), v1.BootstrapPayload{
		ProjectID:            "project.alpha",
		ProjectRoot:          root,
		OutputCandidatesPath: &output,
	})
	if apiErr != nil {
		t.Fatalf("unexpected API error: %+v", apiErr)
	}

	wantPath := filepath.Join(root, "reports", "candidates.json")
	if result.OutputCandidatesPath != wantPath {
		t.Fatalf("unexpected output path: got %q want %q", result.OutputCandidatesPath, wantPath)
	}
	if !result.CandidatesPersisted {
		t.Fatalf("expected candidates_persisted=true when output path is explicit")
	}
	if result.CandidateCount != 1 {
		t.Fatalf("unexpected candidate count: got %d want 1", result.CandidateCount)
	}
	wantWarnings := []string{"respect_gitignore fallback to filesystem walk"}
	if !reflect.DeepEqual(result.Warnings, wantWarnings) {
		t.Fatalf("unexpected warnings: got %v want %v", result.Warnings, wantWarnings)
	}
}

func TestBootstrap_SeedsCanonicalScaffoldFiles(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "a.txt"), []byte("a"), 0o644); err != nil {
		t.Fatalf("write a.txt: %v", err)
	}

	respectGitIgnore := false
	repo := &fakeRepository{}
	svc, err := New(repo)
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	_, apiErr := svc.Bootstrap(context.Background(), v1.BootstrapPayload{
		ProjectID:        "project.alpha",
		ProjectRoot:      root,
		RespectGitIgnore: &respectGitIgnore,
	})
	if apiErr != nil {
		t.Fatalf("unexpected API error: %+v", apiErr)
	}

	rulesRaw, err := os.ReadFile(filepath.Join(root, ".acm", "acm-rules.yaml"))
	if err != nil {
		t.Fatalf("read scaffolded rules file: %v", err)
	}
	if string(rulesRaw) != "version: acm.rules.v1\nrules: []\n" {
		t.Fatalf("unexpected scaffolded rules contents: %q", string(rulesRaw))
	}

	tagsRaw, err := os.ReadFile(filepath.Join(root, ".acm", "acm-tags.yaml"))
	if err != nil {
		t.Fatalf("read scaffolded tags file: %v", err)
	}
	if string(tagsRaw) != "version: acm.tags.v1\ncanonical_tags: {}\n" {
		t.Fatalf("unexpected scaffolded tags contents: %q", string(tagsRaw))
	}

	testsRaw, err := os.ReadFile(filepath.Join(root, ".acm", "acm-tests.yaml"))
	if err != nil {
		t.Fatalf("read scaffolded tests file: %v", err)
	}
	if string(testsRaw) != "version: acm.tests.v1\ndefaults:\n  cwd: .\n  timeout_sec: 300\ntests: []\n" {
		t.Fatalf("unexpected scaffolded tests contents: %q", string(testsRaw))
	}

	workflowsRaw, err := os.ReadFile(filepath.Join(root, ".acm", "acm-workflows.yaml"))
	if err != nil {
		t.Fatalf("read scaffolded workflows file: %v", err)
	}
	if string(workflowsRaw) != "version: acm.workflows.v1\ncompletion:\n  required_tasks: []\n" {
		t.Fatalf("unexpected scaffolded workflows contents: %q", string(workflowsRaw))
	}

	envExampleRaw, err := os.ReadFile(filepath.Join(root, ".env.example"))
	if err != nil {
		t.Fatalf("read scaffolded env example: %v", err)
	}
	wantEnvExample := "# ACM runtime configuration\n# Copy this file to .env to override local defaults.\nACM_PROJECT_ID=myproject\nACM_PROJECT_ROOT=/path/to/repo\nACM_SQLITE_PATH=.acm/context.db\nACM_PG_DSN=postgres://user:pass@localhost:5432/agents_context?sslmode=disable\nACM_UNBOUNDED=false\nACM_LOG_LEVEL=info\nACM_LOG_SINK=stderr\n"
	if string(envExampleRaw) != wantEnvExample {
		t.Fatalf("unexpected scaffolded env example contents: %q", string(envExampleRaw))
	}

	gitignoreRaw, err := os.ReadFile(filepath.Join(root, ".gitignore"))
	if err != nil {
		t.Fatalf("read scaffolded gitignore: %v", err)
	}
	if string(gitignoreRaw) != ".acm/context.db\n.acm/context.db-shm\n.acm/context.db-wal\n" {
		t.Fatalf("unexpected scaffolded gitignore contents: %q", string(gitignoreRaw))
	}
}

func TestBootstrap_ExcludesManagedFilesFromInitialCandidateIndex(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".acm"), 0o755); err != nil {
		t.Fatalf("mkdir .acm: %v", err)
	}
	files := map[string]string{
		"README.md":               "# hello\n",
		".env":                    "SECRET=value\n",
		".env.example":            "ACM_SQLITE_PATH=.acm/context.db\n",
		".gitignore":              ".acm/context.db\n",
		"acm-rules.yaml":          "version: acm.rules.v1\nrules: []\n",
		"acm-tests.yaml":          "version: acm.tests.v1\ndefaults:\n  cwd: .\n  timeout_sec: 60\ntests: []\n",
		"acm-workflows.yaml":      "version: acm.workflows.v1\ncompletion:\n  required_tasks: []\n",
		".acm/context.db":         "sqlite",
		".acm/context.db-wal":     "wal",
		".acm/context.db-shm":     "shm",
		".acm/acm-rules.yaml":     "version: acm.rules.v1\nrules: []\n",
		".acm/acm-tags.yaml":      "version: acm.tags.v1\ncanonical_tags: {}\n",
		".acm/acm-tests.yaml":     "version: acm.tests.v1\ndefaults:\n  cwd: .\n  timeout_sec: 300\ntests: []\n",
		".acm/acm-workflows.yaml": "version: acm.workflows.v1\ncompletion:\n  required_tasks: []\n",
	}
	for rel, contents := range files {
		full := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", filepath.Dir(full), err)
		}
		if err := os.WriteFile(full, []byte(contents), 0o644); err != nil {
			t.Fatalf("write %s: %v", rel, err)
		}
	}

	respectGitIgnore := false
	repo := &fakeRepository{}
	svc, err := New(repo)
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	result, apiErr := svc.Bootstrap(context.Background(), v1.BootstrapPayload{
		ProjectID:        "project.alpha",
		ProjectRoot:      root,
		RespectGitIgnore: &respectGitIgnore,
	})
	if apiErr != nil {
		t.Fatalf("unexpected API error: %+v", apiErr)
	}
	if result.CandidateCount != 1 || result.IndexedStubs != 1 {
		t.Fatalf("unexpected bootstrap counts: %+v", result)
	}
	if len(repo.upsertStubCalls) != 1 || len(repo.upsertStubCalls[0]) != 1 {
		t.Fatalf("expected exactly one stub upsert, got %+v", repo.upsertStubCalls)
	}
	if got := repo.upsertStubCalls[0][0].Path; got != "README.md" {
		t.Fatalf("unexpected indexed path: got %q want %q", got, "README.md")
	}
}

func TestBootstrap_SeedsSuggestedCanonicalTagsWhenRepoSignalsThem(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "internal", "receipt"), 0o755); err != nil {
		t.Fatalf("mkdir receipt dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "internal", "receipt", "fetch_receipt.go"), []byte("package receipt"), 0o644); err != nil {
		t.Fatalf("write fetch_receipt.go: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "internal", "receipt", "report_receipt.go"), []byte("package receipt"), 0o644); err != nil {
		t.Fatalf("write report_receipt.go: %v", err)
	}

	respectGitIgnore := false
	repo := &fakeRepository{}
	svc, err := New(repo)
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	_, apiErr := svc.Bootstrap(context.Background(), v1.BootstrapPayload{
		ProjectID:        "project.alpha",
		ProjectRoot:      root,
		RespectGitIgnore: &respectGitIgnore,
	})
	if apiErr != nil {
		t.Fatalf("unexpected API error: %+v", apiErr)
	}

	tagsRaw, err := os.ReadFile(filepath.Join(root, ".acm", "acm-tags.yaml"))
	if err != nil {
		t.Fatalf("read scaffolded tags file: %v", err)
	}

	doc := canonicalTagsDocumentV1{}
	if err := yaml.Unmarshal(tagsRaw, &doc); err != nil {
		t.Fatalf("parse scaffolded tags file: %v", err)
	}
	if doc.Version != canonicalTagsVersionV1 {
		t.Fatalf("unexpected scaffolded tags version: %q", doc.Version)
	}
	if _, ok := doc.CanonicalTags["receipt"]; !ok {
		t.Fatalf("expected inferred receipt tag, got %v", doc.CanonicalTags)
	}
}

func TestBootstrap_PopulatesExistingBlankCanonicalTagsFileWithSuggestions(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".acm"), 0o755); err != nil {
		t.Fatalf("mkdir .acm: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(root, "internal", "receipt"), 0o755); err != nil {
		t.Fatalf("mkdir receipt dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "internal", "receipt", "fetch_receipt.go"), []byte("package receipt"), 0o644); err != nil {
		t.Fatalf("write fetch_receipt.go: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "internal", "receipt", "report_receipt.go"), []byte("package receipt"), 0o644); err != nil {
		t.Fatalf("write report_receipt.go: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, ".acm", "acm-tags.yaml"), []byte("version: acm.tags.v1\ncanonical_tags: {}\n"), 0o644); err != nil {
		t.Fatalf("write blank tags file: %v", err)
	}

	respectGitIgnore := false
	repo := &fakeRepository{}
	svc, err := New(repo)
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	_, apiErr := svc.Bootstrap(context.Background(), v1.BootstrapPayload{
		ProjectID:        "project.alpha",
		ProjectRoot:      root,
		RespectGitIgnore: &respectGitIgnore,
	})
	if apiErr != nil {
		t.Fatalf("unexpected API error: %+v", apiErr)
	}

	tagsRaw, err := os.ReadFile(filepath.Join(root, ".acm", "acm-tags.yaml"))
	if err != nil {
		t.Fatalf("read scaffolded tags file: %v", err)
	}

	doc := canonicalTagsDocumentV1{}
	if err := yaml.Unmarshal(tagsRaw, &doc); err != nil {
		t.Fatalf("parse scaffolded tags file: %v", err)
	}
	if _, ok := doc.CanonicalTags["receipt"]; !ok {
		t.Fatalf("expected inferred receipt tag, got %v", doc.CanonicalTags)
	}
}

func TestBootstrap_DoesNotOverwriteExistingCanonicalScaffoldFiles(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".acm"), 0o755); err != nil {
		t.Fatalf("mkdir .acm: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "a.txt"), []byte("a"), 0o644); err != nil {
		t.Fatalf("write a.txt: %v", err)
	}
	rulesContent := []byte("version: acm.rules.v1\nrules:\n  - summary: Keep tests green\n")
	if err := os.WriteFile(filepath.Join(root, ".acm", "acm-rules.yaml"), rulesContent, 0o644); err != nil {
		t.Fatalf("write existing rules file: %v", err)
	}
	tagsContent := []byte("version: acm.tags.v1\ncanonical_tags:\n  backend:\n    - svc\n")
	if err := os.WriteFile(filepath.Join(root, ".acm", "acm-tags.yaml"), tagsContent, 0o644); err != nil {
		t.Fatalf("write existing tags file: %v", err)
	}
	testsContent := []byte("version: acm.tests.v1\ndefaults:\n  cwd: tools\n  timeout_sec: 120\ntests:\n  - id: smoke\n    summary: Run smoke tests\n    command:\n      argv: [\"go\", \"test\", \"./...\"]\n")
	if err := os.WriteFile(filepath.Join(root, ".acm", "acm-tests.yaml"), testsContent, 0o644); err != nil {
		t.Fatalf("write existing tests file: %v", err)
	}
	workflowsContent := []byte("version: acm.workflows.v1\ncompletion:\n  required_tasks:\n    - key: verify:tests\n")
	if err := os.WriteFile(filepath.Join(root, ".acm", "acm-workflows.yaml"), workflowsContent, 0o644); err != nil {
		t.Fatalf("write existing workflows file: %v", err)
	}
	envExampleContent := []byte("ACM_SQLITE_PATH=.acm/existing.db\n")
	if err := os.WriteFile(filepath.Join(root, ".env.example"), envExampleContent, 0o644); err != nil {
		t.Fatalf("write existing env example: %v", err)
	}
	gitignoreContent := []byte("node_modules/\n.acm/context.db\n")
	if err := os.WriteFile(filepath.Join(root, ".gitignore"), gitignoreContent, 0o644); err != nil {
		t.Fatalf("write existing gitignore: %v", err)
	}

	respectGitIgnore := false
	repo := &fakeRepository{}
	svc, err := New(repo)
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	_, apiErr := svc.Bootstrap(context.Background(), v1.BootstrapPayload{
		ProjectID:        "project.alpha",
		ProjectRoot:      root,
		RespectGitIgnore: &respectGitIgnore,
	})
	if apiErr != nil {
		t.Fatalf("unexpected API error: %+v", apiErr)
	}

	rulesRaw, err := os.ReadFile(filepath.Join(root, ".acm", "acm-rules.yaml"))
	if err != nil {
		t.Fatalf("read rules file: %v", err)
	}
	if !reflect.DeepEqual(rulesRaw, rulesContent) {
		t.Fatalf("rules file was overwritten: got %q want %q", string(rulesRaw), string(rulesContent))
	}

	tagsRaw, err := os.ReadFile(filepath.Join(root, ".acm", "acm-tags.yaml"))
	if err != nil {
		t.Fatalf("read tags file: %v", err)
	}
	if !reflect.DeepEqual(tagsRaw, tagsContent) {
		t.Fatalf("tags file was overwritten: got %q want %q", string(tagsRaw), string(tagsContent))
	}

	testsRaw, err := os.ReadFile(filepath.Join(root, ".acm", "acm-tests.yaml"))
	if err != nil {
		t.Fatalf("read tests file: %v", err)
	}
	if !reflect.DeepEqual(testsRaw, testsContent) {
		t.Fatalf("tests file was overwritten: got %q want %q", string(testsRaw), string(testsContent))
	}

	workflowsRaw, err := os.ReadFile(filepath.Join(root, ".acm", "acm-workflows.yaml"))
	if err != nil {
		t.Fatalf("read workflows file: %v", err)
	}
	if !reflect.DeepEqual(workflowsRaw, workflowsContent) {
		t.Fatalf("workflows file was overwritten: got %q want %q", string(workflowsRaw), string(workflowsContent))
	}

	envExampleRaw, err := os.ReadFile(filepath.Join(root, ".env.example"))
	if err != nil {
		t.Fatalf("read env example: %v", err)
	}
	wantEnvExample := "ACM_SQLITE_PATH=.acm/existing.db\n\n# ACM runtime configuration\nACM_PROJECT_ID=myproject\nACM_PROJECT_ROOT=/path/to/repo\nACM_PG_DSN=postgres://user:pass@localhost:5432/agents_context?sslmode=disable\nACM_UNBOUNDED=false\nACM_LOG_LEVEL=info\nACM_LOG_SINK=stderr\n"
	if string(envExampleRaw) != wantEnvExample {
		t.Fatalf("unexpected env example contents: got %q want %q", string(envExampleRaw), wantEnvExample)
	}

	gitignoreRaw, err := os.ReadFile(filepath.Join(root, ".gitignore"))
	if err != nil {
		t.Fatalf("read gitignore: %v", err)
	}
	wantGitIgnore := "node_modules/\n.acm/context.db\n.acm/context.db-shm\n.acm/context.db-wal\n"
	if string(gitignoreRaw) != wantGitIgnore {
		t.Fatalf("unexpected gitignore contents: got %q want %q", string(gitignoreRaw), wantGitIgnore)
	}
}

func TestBootstrap_DoesNotSeedPrimaryTestsFileWhenRootTestsFileExists(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "a.txt"), []byte("a"), 0o644); err != nil {
		t.Fatalf("write a.txt: %v", err)
	}
	rootTestsContent := []byte("version: acm.tests.v1\ndefaults:\n  cwd: .\n  timeout_sec: 60\ntests:\n  - id: root-smoke\n    summary: Root tests file\n    command:\n      argv: [\"true\"]\n")
	if err := os.WriteFile(filepath.Join(root, "acm-tests.yaml"), rootTestsContent, 0o644); err != nil {
		t.Fatalf("write root tests file: %v", err)
	}

	respectGitIgnore := false
	repo := &fakeRepository{}
	svc, err := New(repo)
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	_, apiErr := svc.Bootstrap(context.Background(), v1.BootstrapPayload{
		ProjectID:        "project.alpha",
		ProjectRoot:      root,
		RespectGitIgnore: &respectGitIgnore,
	})
	if apiErr != nil {
		t.Fatalf("unexpected API error: %+v", apiErr)
	}

	if _, err := os.Stat(filepath.Join(root, ".acm", "acm-tests.yaml")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected no primary scaffold when root tests file exists, stat err=%v", err)
	}

	gotRootTestsContent, err := os.ReadFile(filepath.Join(root, "acm-tests.yaml"))
	if err != nil {
		t.Fatalf("read root tests file: %v", err)
	}
	if !reflect.DeepEqual(gotRootTestsContent, rootTestsContent) {
		t.Fatalf("root tests file was overwritten: got %q want %q", string(gotRootTestsContent), string(rootTestsContent))
	}
}

func TestBootstrap_DoesNotSeedPrimaryWorkflowsFileWhenRootWorkflowsFileExists(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "a.txt"), []byte("a"), 0o644); err != nil {
		t.Fatalf("write a.txt: %v", err)
	}
	rootWorkflowsContent := []byte("version: acm.workflows.v1\ncompletion:\n  required_tasks:\n    - key: review:cross-llm\n")
	if err := os.WriteFile(filepath.Join(root, "acm-workflows.yaml"), rootWorkflowsContent, 0o644); err != nil {
		t.Fatalf("write root workflows file: %v", err)
	}

	respectGitIgnore := false
	repo := &fakeRepository{}
	svc, err := New(repo)
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	_, apiErr := svc.Bootstrap(context.Background(), v1.BootstrapPayload{
		ProjectID:        "project.alpha",
		ProjectRoot:      root,
		RespectGitIgnore: &respectGitIgnore,
	})
	if apiErr != nil {
		t.Fatalf("unexpected API error: %+v", apiErr)
	}

	if _, err := os.Stat(filepath.Join(root, ".acm", "acm-workflows.yaml")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected no primary scaffold when root workflows file exists, stat err=%v", err)
	}

	gotRootWorkflowsContent, err := os.ReadFile(filepath.Join(root, "acm-workflows.yaml"))
	if err != nil {
		t.Fatalf("read root workflows file: %v", err)
	}
	if !reflect.DeepEqual(gotRootWorkflowsContent, rootWorkflowsContent) {
		t.Fatalf("root workflows file was overwritten: got %q want %q", string(gotRootWorkflowsContent), string(rootWorkflowsContent))
	}
}

func TestBootstrap_ApplyStarterContractSeedsContractsAndIndexesThem(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("# hello\n"), 0o644); err != nil {
		t.Fatalf("write README: %v", err)
	}

	respectGitIgnore := false
	repo := &fakeRepository{}
	svc, err := New(repo)
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	result, apiErr := svc.Bootstrap(context.Background(), v1.BootstrapPayload{
		ProjectID:        "project.alpha",
		ProjectRoot:      root,
		RespectGitIgnore: &respectGitIgnore,
		ApplyTemplates:   []string{"starter-contract"},
	})
	if apiErr != nil {
		t.Fatalf("unexpected API error: %+v", apiErr)
	}

	if result.CandidateCount != 3 || result.IndexedStubs != 3 {
		t.Fatalf("unexpected bootstrap counts: %+v", result)
	}
	if len(repo.upsertStubCalls) != 1 {
		t.Fatalf("expected one stub upsert, got %+v", repo.upsertStubCalls)
	}
	gotPaths := make([]string, 0, len(repo.upsertStubCalls[0]))
	for _, stub := range repo.upsertStubCalls[0] {
		gotPaths = append(gotPaths, stub.Path)
	}
	if wantPaths := []string{"AGENTS.md", "CLAUDE.md", "README.md"}; !reflect.DeepEqual(gotPaths, wantPaths) {
		t.Fatalf("unexpected indexed paths: got %v want %v", gotPaths, wantPaths)
	}

	templateResult, ok := bootstrapTemplateResultByID(result.TemplateResults, "starter-contract")
	if !ok {
		t.Fatalf("expected starter-contract template result, got %+v", result.TemplateResults)
	}
	if wantCreated := []string{"AGENTS.md", "CLAUDE.md"}; !reflect.DeepEqual(templateResult.Created, wantCreated) {
		t.Fatalf("unexpected created paths: got %v want %v", templateResult.Created, wantCreated)
	}
	if wantUpdated := []string{".acm/acm-rules.yaml"}; !reflect.DeepEqual(templateResult.Updated, wantUpdated) {
		t.Fatalf("unexpected updated paths: got %v want %v", templateResult.Updated, wantUpdated)
	}

	agentsRaw, err := os.ReadFile(filepath.Join(root, "AGENTS.md"))
	if err != nil {
		t.Fatalf("read AGENTS.md: %v", err)
	}
	if !strings.Contains(string(agentsRaw), "## Required Task Loop") {
		t.Fatalf("unexpected AGENTS.md contents: %q", string(agentsRaw))
	}

	rulesRaw, err := os.ReadFile(filepath.Join(root, ".acm", "acm-rules.yaml"))
	if err != nil {
		t.Fatalf("read scaffolded rules: %v", err)
	}
	if !strings.Contains(string(rulesRaw), "rule_startup_get_context") {
		t.Fatalf("expected starter rules scaffold, got %q", string(rulesRaw))
	}
}

func TestBootstrap_ApplyDetailedPlanningEnforcementSeedsFeaturePlanningScaffold(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("# hello\n"), 0o644); err != nil {
		t.Fatalf("write README: %v", err)
	}

	respectGitIgnore := false
	repo := &fakeRepository{}
	svc, err := New(repo)
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	result, apiErr := svc.Bootstrap(context.Background(), v1.BootstrapPayload{
		ProjectID:        "project.alpha",
		ProjectRoot:      root,
		RespectGitIgnore: &respectGitIgnore,
		ApplyTemplates:   []string{"detailed-planning-enforcement"},
	})
	if apiErr != nil {
		t.Fatalf("unexpected API error: %+v", apiErr)
	}

	templateResult, ok := bootstrapTemplateResultByID(result.TemplateResults, "detailed-planning-enforcement")
	if !ok {
		t.Fatalf("expected detailed-planning-enforcement template result, got %+v", result.TemplateResults)
	}
	if wantCreated := []string{"AGENTS.md", "CLAUDE.md", "docs/feature-plans.md", "scripts/acm-feature-plan-validate.py"}; !reflect.DeepEqual(templateResult.Created, wantCreated) {
		t.Fatalf("unexpected created paths: got %v want %v", templateResult.Created, wantCreated)
	}
	if wantUpdated := []string{".acm/acm-rules.yaml", ".acm/acm-tests.yaml"}; !reflect.DeepEqual(templateResult.Updated, wantUpdated) {
		t.Fatalf("unexpected updated paths: got %v want %v", templateResult.Updated, wantUpdated)
	}
	if len(repo.upsertStubCalls) != 1 {
		t.Fatalf("expected one stub upsert, got %+v", repo.upsertStubCalls)
	}
	gotPaths := make([]string, 0, len(repo.upsertStubCalls[0]))
	for _, stub := range repo.upsertStubCalls[0] {
		gotPaths = append(gotPaths, stub.Path)
	}
	for _, required := range []string{"AGENTS.md", "CLAUDE.md", "README.md", "docs/feature-plans.md", "scripts/acm-feature-plan-validate.py"} {
		if !containsString(gotPaths, required) {
			t.Fatalf("expected indexed template path %q in %v", required, gotPaths)
		}
	}

	agentsRaw, err := os.ReadFile(filepath.Join(root, "AGENTS.md"))
	if err != nil {
		t.Fatalf("read AGENTS.md: %v", err)
	}
	if !strings.Contains(string(agentsRaw), "## Feature Plans") {
		t.Fatalf("expected detailed feature plan guidance, got %q", string(agentsRaw))
	}

	rulesRaw, err := os.ReadFile(filepath.Join(root, ".acm", "acm-rules.yaml"))
	if err != nil {
		t.Fatalf("read rules scaffold: %v", err)
	}
	if !strings.Contains(string(rulesRaw), "rule_feature_plan_schema") {
		t.Fatalf("expected feature plan rule scaffold, got %q", string(rulesRaw))
	}

	testsRaw, err := os.ReadFile(filepath.Join(root, ".acm", "acm-tests.yaml"))
	if err != nil {
		t.Fatalf("read tests scaffold: %v", err)
	}
	if !strings.Contains(string(testsRaw), "id: feature-plan-validate") {
		t.Fatalf("expected feature plan verify scaffold, got %q", string(testsRaw))
	}

	planDocRaw, err := os.ReadFile(filepath.Join(root, "docs", "feature-plans.md"))
	if err != nil {
		t.Fatalf("read feature plan doc: %v", err)
	}
	if !strings.Contains(string(planDocRaw), "kind=feature_stream") {
		t.Fatalf("expected feature stream guidance, got %q", string(planDocRaw))
	}

	validatorInfo, err := os.Stat(filepath.Join(root, "scripts", "acm-feature-plan-validate.py"))
	if err != nil {
		t.Fatalf("stat feature plan validator: %v", err)
	}
	if validatorInfo.Mode().Perm()&0o111 == 0 {
		t.Fatalf("expected executable validator mode, got %v", validatorInfo.Mode().Perm())
	}
}

func TestBootstrap_DetailedPlanningEnforcementUpgradesPristineStarterScaffolds(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("# hello\n"), 0o644); err != nil {
		t.Fatalf("write README: %v", err)
	}

	respectGitIgnore := false
	repo := &fakeRepository{}
	svc, err := New(repo)
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	_, apiErr := svc.Bootstrap(context.Background(), v1.BootstrapPayload{
		ProjectID:        "project.alpha",
		ProjectRoot:      root,
		RespectGitIgnore: &respectGitIgnore,
		ApplyTemplates:   []string{"starter-contract", "verify-generic"},
	})
	if apiErr != nil {
		t.Fatalf("unexpected API error on first run: %+v", apiErr)
	}

	result, apiErr := svc.Bootstrap(context.Background(), v1.BootstrapPayload{
		ProjectID:        "project.alpha",
		ProjectRoot:      root,
		RespectGitIgnore: &respectGitIgnore,
		ApplyTemplates:   []string{"detailed-planning-enforcement"},
	})
	if apiErr != nil {
		t.Fatalf("unexpected API error on second run: %+v", apiErr)
	}

	templateResult, ok := bootstrapTemplateResultByID(result.TemplateResults, "detailed-planning-enforcement")
	if !ok {
		t.Fatalf("expected detailed-planning-enforcement template result, got %+v", result.TemplateResults)
	}
	if wantCreated := []string{"docs/feature-plans.md", "scripts/acm-feature-plan-validate.py"}; !reflect.DeepEqual(templateResult.Created, wantCreated) {
		t.Fatalf("unexpected created paths: got %v want %v", templateResult.Created, wantCreated)
	}
	if wantUpdated := []string{".acm/acm-rules.yaml", ".acm/acm-tests.yaml", "AGENTS.md", "CLAUDE.md"}; !reflect.DeepEqual(templateResult.Updated, wantUpdated) {
		t.Fatalf("unexpected updated paths: got %v want %v", templateResult.Updated, wantUpdated)
	}
	if templateResult.SkippedConflicts != nil {
		t.Fatalf("expected no template conflicts, got %+v", templateResult.SkippedConflicts)
	}

	agentsRaw, err := os.ReadFile(filepath.Join(root, "AGENTS.md"))
	if err != nil {
		t.Fatalf("read AGENTS.md: %v", err)
	}
	if strings.Contains(string(agentsRaw), "## Optional Feature Plans") || !strings.Contains(string(agentsRaw), "## Feature Plans") {
		t.Fatalf("expected mandatory feature plan guidance, got %q", string(agentsRaw))
	}

	claudeRaw, err := os.ReadFile(filepath.Join(root, "CLAUDE.md"))
	if err != nil {
		t.Fatalf("read CLAUDE.md: %v", err)
	}
	if !strings.Contains(string(claudeRaw), "scripts/acm-feature-plan-validate.py") {
		t.Fatalf("expected CLAUDE guidance to mention the validator, got %q", string(claudeRaw))
	}

	testsRaw, err := os.ReadFile(filepath.Join(root, ".acm", "acm-tests.yaml"))
	if err != nil {
		t.Fatalf("read tests scaffold: %v", err)
	}
	for _, snippet := range []string{"id: feature-plan-help", "id: feature-plan-validate"} {
		if !strings.Contains(string(testsRaw), snippet) {
			t.Fatalf("expected upgraded tests scaffold to include %q, got %q", snippet, string(testsRaw))
		}
	}
}

func TestBootstrap_ApplyVerifyProfilesReplacePristineTestsScaffold(t *testing.T) {
	t.Parallel()

	cases := []struct {
		templateID string
		snippets   []string
	}{
		{
			templateID: "verify-generic",
			snippets: []string{
				`id: smoke`,
				`argv: ["acm", "status", "--project-root", "."]`,
				`id: repo-diff-check`,
			},
		},
		{
			templateID: "verify-go",
			snippets: []string{
				`id: smoke`,
				`id: go-build`,
			},
		},
		{
			templateID: "verify-python",
			snippets: []string{
				`id: smoke`,
				`id: python-compile`,
			},
		},
		{
			templateID: "verify-rust",
			snippets: []string{
				`id: smoke`,
				`id: cargo-check`,
			},
		},
		{
			templateID: "verify-ts",
			snippets: []string{
				`id: smoke`,
				`id: ts-build`,
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.templateID, func(t *testing.T) {
			root := t.TempDir()
			if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("# hello\n"), 0o644); err != nil {
				t.Fatalf("write README: %v", err)
			}

			respectGitIgnore := false
			repo := &fakeRepository{}
			svc, err := New(repo)
			if err != nil {
				t.Fatalf("new service: %v", err)
			}

			result, apiErr := svc.Bootstrap(context.Background(), v1.BootstrapPayload{
				ProjectID:        "project.alpha",
				ProjectRoot:      root,
				RespectGitIgnore: &respectGitIgnore,
				ApplyTemplates:   []string{tc.templateID},
			})
			if apiErr != nil {
				t.Fatalf("unexpected API error: %+v", apiErr)
			}

			templateResult, ok := bootstrapTemplateResultByID(result.TemplateResults, tc.templateID)
			if !ok {
				t.Fatalf("expected %s template result, got %+v", tc.templateID, result.TemplateResults)
			}
			if wantUpdated := []string{".acm/acm-tests.yaml"}; !reflect.DeepEqual(templateResult.Updated, wantUpdated) {
				t.Fatalf("unexpected updated paths: got %v want %v", templateResult.Updated, wantUpdated)
			}

			testsRaw, err := os.ReadFile(filepath.Join(root, ".acm", "acm-tests.yaml"))
			if err != nil {
				t.Fatalf("read tests scaffold: %v", err)
			}
			for _, snippet := range tc.snippets {
				if !strings.Contains(string(testsRaw), snippet) {
					t.Fatalf("expected %s starter contents to include %q, got %q", tc.templateID, snippet, string(testsRaw))
				}
			}
		})
	}
}

func TestBootstrap_ReapplyStarterContractTemplateIsNoOp(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("# hello\n"), 0o644); err != nil {
		t.Fatalf("write README: %v", err)
	}

	respectGitIgnore := false
	repo := &fakeRepository{}
	svc, err := New(repo)
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	_, apiErr := svc.Bootstrap(context.Background(), v1.BootstrapPayload{
		ProjectID:        "project.alpha",
		ProjectRoot:      root,
		RespectGitIgnore: &respectGitIgnore,
		ApplyTemplates:   []string{"starter-contract"},
	})
	if apiErr != nil {
		t.Fatalf("unexpected API error on first run: %+v", apiErr)
	}

	again, apiErr := svc.Bootstrap(context.Background(), v1.BootstrapPayload{
		ProjectID:        "project.alpha",
		ProjectRoot:      root,
		RespectGitIgnore: &respectGitIgnore,
		ApplyTemplates:   []string{"starter-contract"},
	})
	if apiErr != nil {
		t.Fatalf("unexpected API error on second run: %+v", apiErr)
	}

	templateResult, ok := bootstrapTemplateResultByID(again.TemplateResults, "starter-contract")
	if !ok {
		t.Fatalf("expected starter-contract template result, got %+v", again.TemplateResults)
	}
	if templateResult.Created != nil || templateResult.Updated != nil {
		t.Fatalf("expected no created or updated paths on rerun, got %+v", templateResult)
	}
	if wantUnchanged := []string{".acm/acm-rules.yaml", "AGENTS.md", "CLAUDE.md"}; !reflect.DeepEqual(templateResult.Unchanged, wantUnchanged) {
		t.Fatalf("unexpected unchanged paths: got %v want %v", templateResult.Unchanged, wantUnchanged)
	}
}

func TestBootstrap_TemplateConflictDoesNotOverwriteEditedFiles(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".acm"), 0o755); err != nil {
		t.Fatalf("mkdir .acm: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("# hello\n"), 0o644); err != nil {
		t.Fatalf("write README: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "AGENTS.md"), []byte("# custom\n"), 0o644); err != nil {
		t.Fatalf("write AGENTS.md: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, ".acm", "acm-rules.yaml"), []byte(bootstrapkit.BlankRulesContents), 0o644); err != nil {
		t.Fatalf("write pristine rules: %v", err)
	}

	respectGitIgnore := false
	repo := &fakeRepository{}
	svc, err := New(repo)
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	result, apiErr := svc.Bootstrap(context.Background(), v1.BootstrapPayload{
		ProjectID:        "project.alpha",
		ProjectRoot:      root,
		RespectGitIgnore: &respectGitIgnore,
		ApplyTemplates:   []string{"starter-contract"},
	})
	if apiErr != nil {
		t.Fatalf("unexpected API error: %+v", apiErr)
	}

	templateResult, ok := bootstrapTemplateResultByID(result.TemplateResults, "starter-contract")
	if !ok {
		t.Fatalf("expected starter-contract template result, got %+v", result.TemplateResults)
	}
	if len(templateResult.SkippedConflicts) != 1 {
		t.Fatalf("expected one skipped conflict, got %+v", templateResult.SkippedConflicts)
	}
	if got := templateResult.SkippedConflicts[0]; got.Path != "AGENTS.md" || got.Reason != "existing file differs" {
		t.Fatalf("unexpected skipped conflict: %+v", got)
	}

	agentsRaw, err := os.ReadFile(filepath.Join(root, "AGENTS.md"))
	if err != nil {
		t.Fatalf("read AGENTS.md: %v", err)
	}
	if string(agentsRaw) != "# custom\n" {
		t.Fatalf("AGENTS.md was overwritten: %q", string(agentsRaw))
	}
}

func TestBootstrap_ApplyClaudeCommandPackIndexesCreatedFiles(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("# hello\n"), 0o644); err != nil {
		t.Fatalf("write README: %v", err)
	}

	respectGitIgnore := false
	repo := &fakeRepository{}
	svc, err := New(repo)
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	result, apiErr := svc.Bootstrap(context.Background(), v1.BootstrapPayload{
		ProjectID:        "project.alpha",
		ProjectRoot:      root,
		RespectGitIgnore: &respectGitIgnore,
		ApplyTemplates:   []string{"claude-command-pack"},
	})
	if apiErr != nil {
		t.Fatalf("unexpected API error: %+v", apiErr)
	}

	if result.CandidateCount != 10 || result.IndexedStubs != 10 {
		t.Fatalf("unexpected bootstrap counts: %+v", result)
	}
	templateResult, ok := bootstrapTemplateResultByID(result.TemplateResults, "claude-command-pack")
	if !ok {
		t.Fatalf("expected claude-command-pack result, got %+v", result.TemplateResults)
	}
	if len(templateResult.Created) != 9 {
		t.Fatalf("expected 9 created files, got %+v", templateResult.Created)
	}

	gotPaths := make([]string, 0, len(repo.upsertStubCalls[0]))
	for _, stub := range repo.upsertStubCalls[0] {
		gotPaths = append(gotPaths, stub.Path)
	}
	for _, required := range []string{
		".claude/acm-broker/README.md",
		".claude/commands/acm-get.md",
		".claude/commands/acm-review.md",
	} {
		if !containsString(gotPaths, required) {
			t.Fatalf("expected indexed template path %q in %v", required, gotPaths)
		}
	}
}

func TestBootstrap_ClaudeHooksMergesSettingsJSONIdempotently(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".claude"), 0o755); err != nil {
		t.Fatalf("mkdir .claude: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("# hello\n"), 0o644); err != nil {
		t.Fatalf("write README: %v", err)
	}
	settings := `{"permissions":{"allow":["Bash"]},"hooks":{"PostToolUse":[{"matcher":"Read","hooks":[{"type":"command","command":"echo read"}]}]}}`
	if err := os.WriteFile(filepath.Join(root, ".claude", "settings.json"), []byte(settings), 0o644); err != nil {
		t.Fatalf("write settings.json: %v", err)
	}

	respectGitIgnore := false
	repo := &fakeRepository{}
	svc, err := New(repo)
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	result, apiErr := svc.Bootstrap(context.Background(), v1.BootstrapPayload{
		ProjectID:        "project.alpha",
		ProjectRoot:      root,
		RespectGitIgnore: &respectGitIgnore,
		ApplyTemplates:   []string{"claude-hooks"},
	})
	if apiErr != nil {
		t.Fatalf("unexpected API error: %+v", apiErr)
	}

	templateResult, ok := bootstrapTemplateResultByID(result.TemplateResults, "claude-hooks")
	if !ok {
		t.Fatalf("expected claude-hooks result, got %+v", result.TemplateResults)
	}
	if wantCreated := []string{
		".claude/hooks/acm-edit-state.sh",
		".claude/hooks/acm-receipt-guard.sh",
		".claude/hooks/acm-receipt-mark.sh",
		".claude/hooks/acm-session-context.sh",
		".claude/hooks/acm-stop-guard.sh",
	}; !reflect.DeepEqual(templateResult.Created, wantCreated) {
		t.Fatalf("unexpected created paths: got %v want %v", templateResult.Created, wantCreated)
	}
	if wantUpdated := []string{".claude/settings.json"}; !reflect.DeepEqual(templateResult.Updated, wantUpdated) {
		t.Fatalf("unexpected updated paths: got %v want %v", templateResult.Updated, wantUpdated)
	}

	settingsRaw, err := os.ReadFile(filepath.Join(root, ".claude", "settings.json"))
	if err != nil {
		t.Fatalf("read settings.json: %v", err)
	}
	var parsed map[string]any
	if err := json.Unmarshal(settingsRaw, &parsed); err != nil {
		t.Fatalf("parse settings.json: %v", err)
	}
	if _, ok := parsed["permissions"]; !ok {
		t.Fatalf("expected existing settings to remain, got %v", parsed)
	}
	hooks, ok := parsed["hooks"].(map[string]any)
	if !ok {
		t.Fatalf("expected hooks object, got %T", parsed["hooks"])
	}
	if _, ok := hooks["PreToolUse"]; !ok {
		t.Fatalf("expected PreToolUse hook to be merged, got %v", hooks)
	}
	postHooks, ok := hooks["PostToolUse"].([]any)
	if !ok || len(postHooks) < 2 {
		t.Fatalf("expected merged PostToolUse hooks, got %v", hooks["PostToolUse"])
	}

	again, apiErr := svc.Bootstrap(context.Background(), v1.BootstrapPayload{
		ProjectID:        "project.alpha",
		ProjectRoot:      root,
		RespectGitIgnore: &respectGitIgnore,
		ApplyTemplates:   []string{"claude-hooks"},
	})
	if apiErr != nil {
		t.Fatalf("unexpected API error on rerun: %+v", apiErr)
	}
	againResult, ok := bootstrapTemplateResultByID(again.TemplateResults, "claude-hooks")
	if !ok {
		t.Fatalf("expected claude-hooks result on rerun, got %+v", again.TemplateResults)
	}
	if againResult.Created != nil || againResult.Updated != nil {
		t.Fatalf("expected no created or updated paths on rerun, got %+v", againResult)
	}
	if wantUnchanged := []string{
		".claude/hooks/acm-edit-state.sh",
		".claude/hooks/acm-receipt-guard.sh",
		".claude/hooks/acm-receipt-mark.sh",
		".claude/hooks/acm-session-context.sh",
		".claude/hooks/acm-stop-guard.sh",
		".claude/settings.json",
	}; !reflect.DeepEqual(againResult.Unchanged, wantUnchanged) {
		t.Fatalf("unexpected unchanged paths on rerun: got %v want %v", againResult.Unchanged, wantUnchanged)
	}
}

func TestBootstrap_ClaudeReceiptGuardAliasAppliesClaudeHooksTemplate(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".claude"), 0o755); err != nil {
		t.Fatalf("mkdir .claude: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("# hello\n"), 0o644); err != nil {
		t.Fatalf("write README: %v", err)
	}

	respectGitIgnore := false
	svc, err := New(&fakeRepository{})
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	result, apiErr := svc.Bootstrap(context.Background(), v1.BootstrapPayload{
		ProjectID:        "project.alpha",
		ProjectRoot:      root,
		RespectGitIgnore: &respectGitIgnore,
		ApplyTemplates:   []string{"claude-receipt-guard"},
	})
	if apiErr != nil {
		t.Fatalf("unexpected API error: %+v", apiErr)
	}

	if _, ok := bootstrapTemplateResultByID(result.TemplateResults, "claude-hooks"); !ok {
		t.Fatalf("expected claude-receipt-guard alias to apply claude-hooks, got %+v", result.TemplateResults)
	}
}

func TestBootstrap_UnknownTemplateReturnsInvalidInput(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("# hello\n"), 0o644); err != nil {
		t.Fatalf("write README: %v", err)
	}

	respectGitIgnore := false
	repo := &fakeRepository{}
	svc, err := New(repo)
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	_, apiErr := svc.Bootstrap(context.Background(), v1.BootstrapPayload{
		ProjectID:        "project.alpha",
		ProjectRoot:      root,
		RespectGitIgnore: &respectGitIgnore,
		ApplyTemplates:   []string{"missing-template"},
	})
	if apiErr == nil {
		t.Fatal("expected API error")
	}
	if apiErr.Code != "INVALID_INPUT" {
		t.Fatalf("unexpected error code: %s", apiErr.Code)
	}
	if _, err := os.Stat(filepath.Join(root, ".acm")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected no bootstrap side effects on invalid template, stat err=%v", err)
	}
}

func TestBootstrap_PathCollectionErrorMapsInternalError(t *testing.T) {
	repo := &fakeRepository{}
	svc, err := New(repo)
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	respectGitIgnore := false
	_, apiErr := svc.Bootstrap(ctx, v1.BootstrapPayload{
		ProjectID:        "project.alpha",
		ProjectRoot:      t.TempDir(),
		RespectGitIgnore: &respectGitIgnore,
	})
	if apiErr == nil {
		t.Fatal("expected API error")
	}
	if apiErr.Code != "INTERNAL_ERROR" {
		t.Fatalf("unexpected error code: %s", apiErr.Code)
	}
	details, ok := apiErr.Details.(map[string]any)
	if !ok {
		t.Fatalf("expected map details, got %T", apiErr.Details)
	}
	if details["operation"] != "collect_project_paths" {
		t.Fatalf("unexpected operation detail: %#v", details)
	}
}

func candidate(key, path string, isRule bool, tags []string) core.CandidatePointer {
	return core.CandidatePointer{
		Key:         key,
		Path:        path,
		Kind:        "code",
		Label:       key,
		Description: "desc " + key,
		Tags:        append([]string(nil), tags...),
		IsRule:      isRule,
	}
}

func hop(source string, hopCount int, pointer core.CandidatePointer) core.HopPointer {
	return core.HopPointer{SourceKey: source, HopCount: hopCount, Pointer: pointer}
}

func memory(id int64, subject, content string, tags []string, related []string) core.ActiveMemory {
	return core.ActiveMemory{
		ID:                 id,
		Category:           "decision",
		Subject:            subject,
		Content:            content,
		Confidence:         4,
		Tags:               append([]string(nil), tags...),
		RelatedPointerKeys: append([]string(nil), related...),
	}
}

func bootstrapTemplateResultByID(results []v1.BootstrapTemplateResult, templateID string) (v1.BootstrapTemplateResult, bool) {
	for _, result := range results {
		if result.TemplateID == templateID {
			return result, true
		}
	}
	return v1.BootstrapTemplateResult{}, false
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func pointerKeys(receipt *v1.ContextReceipt) []string {
	if receipt == nil {
		return nil
	}

	keys := make(map[string]struct{}, len(receipt.Rules)+len(receipt.Suggestions))
	for _, entry := range receipt.Rules {
		key := strings.TrimSpace(entry.Key)
		if key == "" {
			continue
		}
		keys[key] = struct{}{}
	}
	for _, entry := range receipt.Suggestions {
		key := strings.TrimSpace(entry.Key)
		if key == "" {
			continue
		}
		keys[key] = struct{}{}
	}
	return mapKeysSorted(keys)
}

func receiptIndexEntries(receipt *v1.ContextReceipt, index string) []map[string]any {
	payload := receiptJSONMap(receipt)
	if len(payload) == 0 {
		return nil
	}

	switch index {
	case "rules", "suggestions", "memories", "plans":
		return normalizeIndexEntries(payload[index])
	default:
		return nil
	}
}

func normalizeIndexEntries(raw any) []map[string]any {
	entries, ok := raw.([]any)
	if !ok {
		return nil
	}
	out := make([]map[string]any, 0, len(entries))
	for _, rawEntry := range entries {
		entry, ok := rawEntry.(map[string]any)
		if !ok {
			continue
		}
		out = append(out, entry)
	}
	return out
}

func receiptIndexKeys(entries []map[string]any) []string {
	keys := make([]string, 0, len(entries))
	for _, entry := range entries {
		key := strings.TrimSpace(entryString(entry, "key"))
		if key == "" {
			continue
		}
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func receiptMeta(receipt *v1.ContextReceipt) map[string]any {
	payload := receiptJSONMap(receipt)
	if len(payload) == 0 {
		return nil
	}
	if meta, ok := payload["_meta"].(map[string]any); ok {
		return meta
	}
	return nil
}

func entryString(entry map[string]any, field string) string {
	if entry == nil {
		return ""
	}
	return anyToString(entry[field])
}

func entryStringSlice(entry map[string]any, field string) []string {
	if entry == nil {
		return nil
	}
	raw := entry[field]
	values, ok := raw.([]any)
	if !ok {
		return nil
	}
	out := make([]string, 0, len(values))
	for _, value := range values {
		normalized := strings.TrimSpace(anyToString(value))
		if normalized == "" {
			continue
		}
		out = append(out, normalized)
	}
	return out
}

func anyToString(value any) string {
	switch typed := value.(type) {
	case string:
		return typed
	case float64:
		if typed == float64(int64(typed)) {
			return strconv.FormatInt(int64(typed), 10)
		}
		return strings.TrimSpace(strconv.FormatFloat(typed, 'f', -1, 64))
	case int:
		return strconv.Itoa(typed)
	case int64:
		return strconv.FormatInt(typed, 10)
	default:
		return ""
	}
}

func boolPtr(v bool) *bool {
	return &v
}

func TestReportCompletion_WarnModeAcceptsOutOfScopeAndPersistsSummary(t *testing.T) {
	repo := &fakeRepository{
		scopeResults: []core.ReceiptScope{{
			ProjectID:    "project.alpha",
			ReceiptID:    "receipt.abc123",
			TaskText:     "warn mode scope check",
			Phase:        "execute",
			ResolvedTags: []string{"backend"},
			PointerKeys:  []string{"code:repo"},
			PointerPaths: []string{"src/allowed.go"},
		}},
		saveResult: core.RunReceiptIDs{RunID: 88, ReceiptID: "receipt.abc123"},
	}
	svc, err := New(repo)
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	result, apiErr := svc.ReportCompletion(context.Background(), v1.ReportCompletionPayload{
		ProjectID:    "project.alpha",
		ReceiptID:    "receipt.abc123",
		FilesChanged: []string{"src/allowed.go", "src/outside.go"},
		Outcome:      "completed with exploratory touch",
		ScopeMode:    v1.ScopeModeWarn,
	})
	if apiErr != nil {
		t.Fatalf("unexpected API error: %+v", apiErr)
	}
	if !result.Accepted {
		t.Fatalf("expected accepted result in warn mode: %+v", result)
	}
	if result.RunID != 88 {
		t.Fatalf("unexpected run id: got %d want 88", result.RunID)
	}
	if len(result.Violations) != 1 || result.Violations[0].Path != "src/outside.go" {
		t.Fatalf("unexpected violations: %+v", result.Violations)
	}
	if len(repo.saveCalls) != 1 {
		t.Fatalf("expected one persisted run summary, got %d", len(repo.saveCalls))
	}
	if repo.saveCalls[0].Status != "accepted_with_warnings" {
		t.Fatalf("unexpected persisted status: %q", repo.saveCalls[0].Status)
	}
}

func TestReportCompletion_AutoIndexModeAcceptsOutOfScopeAndUpsertsPointerStubs(t *testing.T) {
	repo := &fakeRepository{
		scopeResults: []core.ReceiptScope{{
			ProjectID:    "project.alpha",
			ReceiptID:    "receipt.abc123",
			TaskText:     "auto index scope check",
			Phase:        "execute",
			ResolvedTags: []string{"backend"},
			PointerKeys:  []string{"code:repo"},
			PointerPaths: []string{"src/allowed.go"},
		}},
		saveResult: core.RunReceiptIDs{RunID: 109, ReceiptID: "receipt.abc123"},
	}
	svc, err := New(repo)
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	result, apiErr := svc.ReportCompletion(context.Background(), v1.ReportCompletionPayload{
		ProjectID:    "project.alpha",
		ReceiptID:    "receipt.abc123",
		FilesChanged: []string{"src/allowed.go", "src/new.go", "scripts/reindex.sh"},
		Outcome:      "completed with exploratory touches",
		ScopeMode:    v1.ScopeModeAutoIndex,
	})
	if apiErr != nil {
		t.Fatalf("unexpected API error: %+v", apiErr)
	}
	if !result.Accepted {
		t.Fatalf("expected accepted result in auto_index mode: %+v", result)
	}
	if result.RunID != 109 {
		t.Fatalf("unexpected run id: got %d want 109", result.RunID)
	}
	if len(result.Violations) != 2 {
		t.Fatalf("expected 2 violations, got %+v", result.Violations)
	}
	if len(repo.upsertStubCalls) != 1 {
		t.Fatalf("expected one pointer stub upsert call, got %d", len(repo.upsertStubCalls))
	}
	if len(repo.saveCalls) != 1 {
		t.Fatalf("expected one persisted run summary, got %d", len(repo.saveCalls))
	}
	if repo.saveCalls[0].Status != "accepted_with_auto_index" {
		t.Fatalf("unexpected persisted status: %q", repo.saveCalls[0].Status)
	}

	stubByPath := make(map[string]core.PointerStub)
	for _, stub := range repo.upsertStubCalls[0] {
		stubByPath[stub.Path] = stub
	}

	srcStub, ok := stubByPath["src/new.go"]
	if !ok {
		t.Fatalf("expected auto-index stub for src/new.go, got %+v", repo.upsertStubCalls[0])
	}
	if srcStub.Kind != "code" {
		t.Fatalf("unexpected src/new.go kind: %q", srcStub.Kind)
	}
	if srcStub.PointerKey != "project.alpha:src/new.go" {
		t.Fatalf("unexpected src/new.go pointer key: %q", srcStub.PointerKey)
	}

	scriptStub, ok := stubByPath["scripts/reindex.sh"]
	if !ok {
		t.Fatalf("expected auto-index stub for scripts/reindex.sh, got %+v", repo.upsertStubCalls[0])
	}
	if scriptStub.Kind != "command" {
		t.Fatalf("unexpected scripts/reindex.sh kind: %q", scriptStub.Kind)
	}
	if scriptStub.PointerKey != "project.alpha:scripts/reindex.sh" {
		t.Fatalf("unexpected scripts/reindex.sh pointer key: %q", scriptStub.PointerKey)
	}
}

func TestReportCompletion_AutoIndexUsesRepoLocalCanonicalTags(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".acm"), 0o755); err != nil {
		t.Fatalf("mkdir .acm: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, ".acm", "acm-tags.yaml"), []byte("version: acm.tags.v1\ncanonical_tags:\n  backend:\n    - svc\n"), 0o644); err != nil {
		t.Fatalf("write tags file: %v", err)
	}
	withWorkingDir(t, root)

	repo := &fakeRepository{
		scopeResults: []core.ReceiptScope{{
			ProjectID:    "project.alpha",
			ReceiptID:    "receipt.abc123",
			PointerPaths: []string{"src/allowed.go"},
		}},
		saveResult: core.RunReceiptIDs{RunID: 110, ReceiptID: "receipt.abc123"},
	}
	svc, err := New(repo)
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	_, apiErr := svc.ReportCompletion(context.Background(), v1.ReportCompletionPayload{
		ProjectID:    "project.alpha",
		ReceiptID:    "receipt.abc123",
		FilesChanged: []string{"src/allowed.go", "src/svc/new.go"},
		Outcome:      "done",
		ScopeMode:    v1.ScopeModeAutoIndex,
	})
	if apiErr != nil {
		t.Fatalf("unexpected API error: %+v", apiErr)
	}
	if len(repo.upsertStubCalls) != 1 || len(repo.upsertStubCalls[0]) != 1 {
		t.Fatalf("expected one auto-index stub, got %+v", repo.upsertStubCalls)
	}
	wantTags := []string{"auto-indexed", "backend", "code", "new", "src"}
	if !reflect.DeepEqual(repo.upsertStubCalls[0][0].Tags, wantTags) {
		t.Fatalf("unexpected auto-index tags: got %v want %v", repo.upsertStubCalls[0][0].Tags, wantTags)
	}
}

func TestReportCompletion_AutoIndexModeUpsertFailureReturnsInternalError(t *testing.T) {
	repo := &fakeRepository{
		scopeResults: []core.ReceiptScope{{
			ProjectID:    "project.alpha",
			ReceiptID:    "receipt.abc123",
			PointerPaths: []string{"src/allowed.go"},
		}},
		upsertStubErrors: []error{errors.New("upsert failed")},
	}
	svc, err := New(repo)
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	_, apiErr := svc.ReportCompletion(context.Background(), v1.ReportCompletionPayload{
		ProjectID:    "project.alpha",
		ReceiptID:    "receipt.abc123",
		FilesChanged: []string{"src/allowed.go", "src/new.go"},
		Outcome:      "attempted completion",
		ScopeMode:    v1.ScopeModeAutoIndex,
	})
	if apiErr == nil {
		t.Fatal("expected API error")
	}
	if apiErr.Code != "INTERNAL_ERROR" {
		t.Fatalf("unexpected API error code: got %q want %q", apiErr.Code, "INTERNAL_ERROR")
	}
	if len(repo.saveCalls) != 0 {
		t.Fatalf("did not expect persisted run summary on upsert failure, got %d", len(repo.saveCalls))
	}
}

func TestSync_WorkingTreeModeIncludesUntrackedAndUsesFilesystemHashes(t *testing.T) {
	repo := &fakeRepository{
		syncResults: []core.SyncApplyResult{{Updated: 2, NewCandidates: 1}},
	}
	svc, err := New(repo)
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	projectRoot := t.TempDir()
	if err := os.MkdirAll(filepath.Join(projectRoot, "src"), 0o755); err != nil {
		t.Fatalf("mkdir src: %v", err)
	}
	if err := os.WriteFile(filepath.Join(projectRoot, "src", "tracked.go"), []byte("package src\n\nfunc tracked() {}\n"), 0o644); err != nil {
		t.Fatalf("write tracked.go: %v", err)
	}
	if err := os.WriteFile(filepath.Join(projectRoot, "src", "new.go"), []byte("package src\n\nfunc created() {}\n"), 0o644); err != nil {
		t.Fatalf("write new.go: %v", err)
	}

	var gitCalls []string
	svc.runGitCommand = func(_ context.Context, root string, args ...string) (string, error) {
		if root != projectRoot {
			t.Fatalf("unexpected project root: %s", root)
		}
		joined := strings.Join(args, " ")
		gitCalls = append(gitCalls, joined)
		switch joined {
		case "diff --name-status --find-renames HEAD":
			return "M\tsrc/tracked.go\nM\t.gitignore\n", nil
		case "ls-files --others --exclude-standard":
			return "src/new.go\n.acm/context.db-wal\n", nil
		default:
			t.Fatalf("unexpected git args: %s", joined)
		}
		return "", nil
	}

	result, apiErr := svc.Sync(context.Background(), v1.SyncPayload{
		ProjectID:   "project.alpha",
		Mode:        "working_tree",
		ProjectRoot: projectRoot,
	})
	if apiErr != nil {
		t.Fatalf("unexpected API error: %+v", apiErr)
	}
	if len(repo.syncCalls) != 1 {
		t.Fatalf("expected one apply sync call, got %d", len(repo.syncCalls))
	}
	call := repo.syncCalls[0]
	if call.Mode != "working_tree" {
		t.Fatalf("unexpected sync mode: %q", call.Mode)
	}
	if len(call.Paths) != 2 {
		t.Fatalf("expected 2 sync paths, got %d", len(call.Paths))
	}
	for _, p := range call.Paths {
		if p.Deleted {
			t.Fatalf("did not expect deleted path in working tree test: %+v", p)
		}
		if strings.TrimSpace(p.ContentHash) == "" {
			t.Fatalf("expected content hash for path %+v", p)
		}
	}
	if !reflect.DeepEqual(result.ProcessedPaths, []string{"src/new.go", "src/tracked.go"}) {
		t.Fatalf("unexpected processed paths: %v", result.ProcessedPaths)
	}
	if len(gitCalls) != 2 {
		t.Fatalf("expected two git calls, got %v", gitCalls)
	}
}

func TestCoverage_ComputesSummaryAndDetails(t *testing.T) {
	repo := &fakeRepository{
		inventoryResults: []core.PointerInventory{
			{Path: "src/covered.go", IsStale: false},
			{Path: "src/stale.go", IsStale: true},
			{Path: "docs/old.md", IsStale: true},
		},
	}
	svc, err := New(repo)
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	svc.runGitCommand = func(_ context.Context, _ string, args ...string) (string, error) {
		if strings.Join(args, " ") != "ls-files --cached --others --exclude-standard" {
			t.Fatalf("unexpected git args: %v", args)
		}
		return "src/covered.go\nsrc/stale.go\nsrc/unindexed.go\ncmd/tool/main.go\n", nil
	}

	result, apiErr := svc.Coverage(context.Background(), v1.CoveragePayload{
		ProjectID: "project.alpha",
	})
	if apiErr != nil {
		t.Fatalf("unexpected API error: %+v", apiErr)
	}
	if len(repo.inventoryCalls) != 1 || repo.inventoryCalls[0] != "project.alpha" {
		t.Fatalf("unexpected inventory calls: %v", repo.inventoryCalls)
	}
	if result.Summary.TotalFiles != 4 || result.Summary.IndexedFiles != 2 || result.Summary.UnindexedFiles != 2 || result.Summary.StaleFiles != 2 {
		t.Fatalf("unexpected coverage summary: %+v", result.Summary)
	}
	if !reflect.DeepEqual(result.UnindexedPaths, []string{"cmd/tool/main.go", "src/unindexed.go"}) {
		t.Fatalf("unexpected unindexed paths: %v", result.UnindexedPaths)
	}
	if !reflect.DeepEqual(result.StalePaths, []string{"docs/old.md", "src/stale.go"}) {
		t.Fatalf("unexpected stale paths: %v", result.StalePaths)
	}
	if !reflect.DeepEqual(result.ZeroCoverageDirs, []string{"cmd/tool"}) {
		t.Fatalf("unexpected zero coverage dirs: %v", result.ZeroCoverageDirs)
	}
}

func TestCoverage_ExcludesManagedFilesFromCoverageSet(t *testing.T) {
	repo := &fakeRepository{
		inventoryResults: []core.PointerInventory{
			{Path: "src/covered.go", IsStale: false},
		},
	}
	svc, err := New(repo)
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	svc.runGitCommand = func(_ context.Context, _ string, args ...string) (string, error) {
		if strings.Join(args, " ") != "ls-files --cached --others --exclude-standard" {
			t.Fatalf("unexpected git args: %v", args)
		}
		return ".gitignore\n.env.example\n.acm/acm-tests.yaml\n.acm/context.db-wal\nsrc/covered.go\nsrc/unindexed.go\n", nil
	}

	result, apiErr := svc.Coverage(context.Background(), v1.CoveragePayload{
		ProjectID: "project.alpha",
	})
	if apiErr != nil {
		t.Fatalf("unexpected API error: %+v", apiErr)
	}
	if result.Summary.TotalFiles != 2 || result.Summary.IndexedFiles != 1 || result.Summary.UnindexedFiles != 1 {
		t.Fatalf("unexpected coverage summary: %+v", result.Summary)
	}
	if !reflect.DeepEqual(result.UnindexedPaths, []string{"src/unindexed.go"}) {
		t.Fatalf("unexpected unindexed paths: %v", result.UnindexedPaths)
	}
}

func TestCoverage_InventoryErrorMapsInternalError(t *testing.T) {
	repo := &fakeRepository{
		inventoryErrors: []error{errors.New("inventory failed")},
	}
	svc, err := New(repo)
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	svc.runGitCommand = func(_ context.Context, _ string, _ ...string) (string, error) {
		return "src/file.go\n", nil
	}

	_, apiErr := svc.Coverage(context.Background(), v1.CoveragePayload{
		ProjectID: "project.alpha",
	})
	if apiErr == nil {
		t.Fatal("expected API error")
	}
	if apiErr.Code != "INTERNAL_ERROR" {
		t.Fatalf("unexpected API error code: %s", apiErr.Code)
	}
	details, ok := apiErr.Details.(map[string]any)
	if !ok {
		t.Fatalf("expected map details, got %T", apiErr.Details)
	}
	if details["operation"] != "list_pointer_inventory" {
		t.Fatalf("unexpected operation detail: %#v", details)
	}
}

func TestFetch_PlanKeyReturnsLookupSummary(t *testing.T) {
	repo := &fakeRepository{
		workPlanLookupResult: []core.WorkPlan{{
			ProjectID: "project.alpha",
			PlanKey:   "plan:receipt.abc123",
			ReceiptID: "receipt.abc123",
			Title:     "Execution plan",
			Status:    core.PlanStatusCompleted,
			UpdatedAt: time.Date(2026, 3, 5, 12, 0, 0, 0, time.UTC),
			Tasks: []core.WorkItem{{
				ItemKey: "src/a.go",
				Status:  core.WorkItemStatusCompleted,
			}},
		}},
	}
	svc, err := New(repo)
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	key := "plan:receipt.abc123"
	result, apiErr := svc.Fetch(context.Background(), v1.FetchPayload{
		ProjectID: "project.alpha",
		Keys:      []string{key},
	})
	if apiErr != nil {
		t.Fatalf("unexpected API error: %+v", apiErr)
	}
	if len(result.Items) != 1 {
		t.Fatalf("expected one fetch item, got %+v", result)
	}
	item := result.Items[0]
	if item.Key != key || item.Type != "plan" || item.Status != core.PlanStatusComplete {
		t.Fatalf("unexpected fetch item: %+v", item)
	}
	if strings.TrimSpace(item.Version) == "" {
		t.Fatalf("expected non-empty plan version: %+v", item)
	}
	if !strings.Contains(item.Content, "\"plan_key\":\"plan:receipt.abc123\"") {
		t.Fatalf("expected serialized plan content, got %q", item.Content)
	}
	if len(result.NotFound) != 0 || len(result.VersionMismatches) != 0 {
		t.Fatalf("unexpected fetch metadata: %+v", result)
	}
	if len(repo.workPlanLookupCalls) != 1 {
		t.Fatalf("expected one plan lookup call, got %d", len(repo.workPlanLookupCalls))
	}
	if repo.workPlanLookupCalls[0].ProjectID != "project.alpha" || repo.workPlanLookupCalls[0].ReceiptID != "receipt.abc123" {
		t.Fatalf("unexpected plan lookup query: %+v", repo.workPlanLookupCalls[0])
	}
}

func TestFetch_TaskKeyReturnsStoredTaskContent(t *testing.T) {
	repo := &fakeRepository{
		workPlanLookupResult: []core.WorkPlan{{
			ProjectID: "project.alpha",
			PlanKey:   "plan:receipt.abc123",
			ReceiptID: "receipt.abc123",
			Tasks: []core.WorkItem{
				{
					ItemKey:            "task.alpha",
					Summary:            "Split provider adapter",
					Status:             core.WorkItemStatusInProgress,
					ParentTaskKey:      "task.epic",
					AcceptanceCriteria: []string{"Adapter compiles"},
					ExternalRefs:       []string{"jira:WEB-11"},
				},
			},
		}},
	}
	svc, err := New(repo)
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	key := "task:plan:receipt.abc123#task.alpha"
	result, apiErr := svc.Fetch(context.Background(), v1.FetchPayload{
		ProjectID: "project.alpha",
		Keys:      []string{key},
	})
	if apiErr != nil {
		t.Fatalf("unexpected API error: %+v", apiErr)
	}
	if len(result.Items) != 1 {
		t.Fatalf("expected one fetch item, got %+v", result)
	}
	item := result.Items[0]
	if item.Key != key || item.Type != "task" || item.Status != core.WorkItemStatusInProgress {
		t.Fatalf("unexpected task fetch item: %+v", item)
	}
	if !strings.Contains(item.Content, "\"parent_task_key\":\"task.epic\"") {
		t.Fatalf("expected task content payload, got %q", item.Content)
	}
	if len(repo.workPlanLookupCalls) != 1 {
		t.Fatalf("expected one plan lookup call, got %d", len(repo.workPlanLookupCalls))
	}
	if repo.workPlanLookupCalls[0].PlanKey != "plan:receipt.abc123" {
		t.Fatalf("unexpected plan lookup query: %+v", repo.workPlanLookupCalls[0])
	}
}

func TestFetch_PlanKeyFallsBackToLegacyLookupWhenPlanRowMissing(t *testing.T) {
	repo := &fakeRepository{
		fetchLookupResults: []core.FetchLookup{{
			ProjectID:  "project.alpha",
			ReceiptID:  "receipt.abc123",
			RunID:      17,
			PlanStatus: core.PlanStatusCompleted,
			WorkItems: []core.WorkItem{
				{ItemKey: "src/a.go", Status: core.WorkItemStatusCompleted},
			},
		}},
	}
	svc, err := New(repo)
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	key := "plan:receipt.abc123"
	result, apiErr := svc.Fetch(context.Background(), v1.FetchPayload{
		ProjectID: "project.alpha",
		Keys:      []string{key},
	})
	if apiErr != nil {
		t.Fatalf("unexpected API error: %+v", apiErr)
	}
	if len(result.Items) != 1 {
		t.Fatalf("expected one fetch item, got %+v", result)
	}
	item := result.Items[0]
	if item.Key != key || item.Type != "plan" || item.Status != core.PlanStatusComplete || item.Version != "17" {
		t.Fatalf("unexpected fetch item: %+v", item)
	}
	if len(repo.workPlanLookupCalls) != 1 {
		t.Fatalf("expected one plan lookup call, got %d", len(repo.workPlanLookupCalls))
	}
	if len(repo.fetchLookupCalls) != 1 {
		t.Fatalf("expected one legacy fetch lookup call, got %d", len(repo.fetchLookupCalls))
	}
}

func TestFetch_ReceiptAndRunKeysReturnStructuredHistoryContent(t *testing.T) {
	repo := &fakeRepository{
		scopeResults: []core.ReceiptScope{{
			ProjectID:    "project.alpha",
			ReceiptID:    "receipt.abc123",
			TaskText:     "Trace onboarding history",
			Phase:        string(v1.PhaseExecute),
			ResolvedTags: []string{"backend"},
			PointerKeys:  []string{"code:pointer"},
			MemoryIDs:    []int64{42},
		}},
		fetchLookupResults: []core.FetchLookup{{
			ProjectID:  "project.alpha",
			ReceiptID:  "receipt.abc123",
			RunID:      17,
			RunStatus:  "accepted",
			PlanStatus: core.PlanStatusInProgress,
			WorkItems: []core.WorkItem{
				{ItemKey: "verify:tests", Status: core.WorkItemStatusComplete},
			},
			UpdatedAt: time.Date(2026, 3, 6, 18, 4, 5, 0, time.UTC),
		}},
		runHistoryLookup: []core.RunHistorySummary{{
			RunID:        17,
			ReceiptID:    "receipt.abc123",
			RequestID:    "req-12345678",
			TaskText:     "Trace onboarding history",
			Phase:        string(v1.PhaseExecute),
			Status:       "accepted",
			FilesChanged: []string{"internal/service/backend/service.go"},
			Outcome:      "Captured receipt and run history",
			UpdatedAt:    time.Date(2026, 3, 6, 18, 4, 5, 0, time.UTC),
		}},
	}
	svc, err := New(repo)
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	result, apiErr := svc.Fetch(context.Background(), v1.FetchPayload{
		ProjectID: "project.alpha",
		Keys:      []string{"receipt:receipt.abc123", "run:17"},
	})
	if apiErr != nil {
		t.Fatalf("unexpected API error: %+v", apiErr)
	}
	if len(result.Items) != 2 {
		t.Fatalf("expected two fetch items, got %+v", result)
	}
	if result.Items[0].Type != "receipt" || !strings.Contains(result.Items[0].Content, "\"receipt_id\":\"receipt.abc123\"") || !strings.Contains(result.Items[0].Content, "\"memory_keys\":[\"mem:42\"]") {
		t.Fatalf("unexpected receipt fetch item: %+v", result.Items[0])
	}
	if result.Items[1].Type != "run" || !strings.Contains(result.Items[1].Content, "\"run_id\":17") || !strings.Contains(result.Items[1].Content, "\"files_changed\":[\"internal/service/backend/service.go\"]") {
		t.Fatalf("unexpected run fetch item: %+v", result.Items[1])
	}
}

func TestFetchPayloadKeysWithReceipt_DerivesPlanKeyWhenKeysOmitted(t *testing.T) {
	keys := fetchPayloadKeysWithReceipt(nil, " receipt.abc123 ")
	if !reflect.DeepEqual(keys, []string{"plan:receipt.abc123"}) {
		t.Fatalf("unexpected derived keys: %+v", keys)
	}
}

func TestFetchPayloadKeysWithReceipt_PreservesExplicitKeys(t *testing.T) {
	keys := fetchPayloadKeysWithReceipt([]string{" mem:42 ", "plan:receipt.abc123"}, "receipt.other")
	if !reflect.DeepEqual(keys, []string{"mem:42", "plan:receipt.abc123"}) {
		t.Fatalf("unexpected normalized keys: %+v", keys)
	}
}

func TestFetchPayloadKeys_DerivesPlanKeyFromPayloadReceiptID(t *testing.T) {
	keys := fetchPayloadKeys(v1.FetchPayload{
		ReceiptID: " receipt.abc123 ",
	})
	if !reflect.DeepEqual(keys, []string{"plan:receipt.abc123"}) {
		t.Fatalf("unexpected derived keys: %+v", keys)
	}
}

func TestParsePlanFetchKey_RejectsMixedCasePrefix(t *testing.T) {
	if _, ok := parsePlanFetchKey("PLAN:receipt.abc123"); ok {
		t.Fatal("expected PLAN: prefix to be rejected")
	}
}

func TestParsePlanFetchKey_RejectsInvalidReceiptID(t *testing.T) {
	if _, ok := parsePlanFetchKey("plan:short"); ok {
		t.Fatal("expected invalid receipt_id suffix to be rejected")
	}
}

func TestParseTaskFetchKey_ParsesPlanAndTask(t *testing.T) {
	ref, ok := parseTaskFetchKey("task:plan:receipt.abc123#task.alpha")
	if !ok {
		t.Fatal("expected task fetch key to parse")
	}
	if ref.PlanKey != "plan:receipt.abc123" || ref.ReceiptID != "receipt.abc123" || ref.TaskKey != "task.alpha" {
		t.Fatalf("unexpected task fetch ref: %+v", ref)
	}
}

func TestParseTaskFetchKey_RejectsInvalidPlanKey(t *testing.T) {
	if _, ok := parseTaskFetchKey("task:plan:short#task.alpha"); ok {
		t.Fatal("expected task fetch key with invalid plan receipt to be rejected")
	}
}

func TestTaskFetchKey_SkipsOverlongCompositeKeys(t *testing.T) {
	planKey := "plan:" + strings.Repeat("r", 32)
	taskKey := strings.Repeat("t", 600)
	if got := taskFetchKey(planKey, taskKey); got != "" {
		t.Fatalf("expected overlong task fetch key to be dropped, got %q", got)
	}
}

func TestReceiptAndRunFetchKeys(t *testing.T) {
	if got := receiptFetchKey(" receipt.abc123 "); got != "receipt:receipt.abc123" {
		t.Fatalf("unexpected receipt fetch key: %q", got)
	}
	if receiptID, ok := parseReceiptFetchKey("receipt:receipt.abc123"); !ok || receiptID != "receipt.abc123" {
		t.Fatalf("unexpected parsed receipt key: %q %v", receiptID, ok)
	}
	if _, ok := parseReceiptFetchKey("receipt:short"); ok {
		t.Fatal("expected invalid receipt fetch key to be rejected")
	}
	if got := runFetchKey(17); got != "run:17" {
		t.Fatalf("unexpected run fetch key: %q", got)
	}
	if runID, ok := parseRunFetchKey("run:17"); !ok || runID != 17 {
		t.Fatalf("unexpected parsed run key: %d %v", runID, ok)
	}
	if _, ok := parseRunFetchKey("run:not-a-number"); ok {
		t.Fatal("expected invalid run fetch key to be rejected")
	}
}

func TestFetch_PointerKeyReturnsContentWhenReadable(t *testing.T) {
	tmpDir := t.TempDir()
	pointerPath := filepath.Join(tmpDir, "pointer.txt")
	pointerContent := "pointer payload for fetch"
	if err := os.WriteFile(pointerPath, []byte(pointerContent), 0o644); err != nil {
		t.Fatalf("write pointer content: %v", err)
	}

	repo := &fakeRepository{
		pointerLookupResults: []core.CandidatePointer{
			candidate("code:pointer", pointerPath, false, []string{"backend"}),
		},
	}
	svc, err := New(repo)
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	key := "code:pointer"
	result, apiErr := svc.Fetch(context.Background(), v1.FetchPayload{
		ProjectID: "project.alpha",
		Keys:      []string{key},
	})
	if apiErr != nil {
		t.Fatalf("unexpected API error: %+v", apiErr)
	}
	if len(result.Items) != 1 {
		t.Fatalf("expected one fetch item, got %+v", result)
	}
	item := result.Items[0]
	if item.Key != key || item.Type != "suggestion" {
		t.Fatalf("unexpected pointer fetch item identity: %+v", item)
	}
	if item.Content != pointerContent {
		t.Fatalf("unexpected pointer content: got %q want %q", item.Content, pointerContent)
	}
	if strings.TrimSpace(item.Summary) == "" {
		t.Fatalf("expected non-empty pointer summary: %+v", item)
	}
	if strings.TrimSpace(item.Version) == "" {
		t.Fatalf("expected non-empty pointer version: %+v", item)
	}
	if len(result.NotFound) != 0 || len(result.VersionMismatches) != 0 {
		t.Fatalf("unexpected fetch metadata: %+v", result)
	}
	if len(repo.pointerLookupCalls) != 1 {
		t.Fatalf("expected one pointer lookup call, got %d", len(repo.pointerLookupCalls))
	}
	if repo.pointerLookupCalls[0].ProjectID != "project.alpha" || repo.pointerLookupCalls[0].PointerKey != key {
		t.Fatalf("unexpected pointer lookup query: %+v", repo.pointerLookupCalls[0])
	}
}

func TestFetch_PointerKeyReadsRelativePathFromServiceProjectRoot(t *testing.T) {
	projectRoot := t.TempDir()
	pointerPath := filepath.Join(projectRoot, "docs", "pointer.txt")
	if err := os.MkdirAll(filepath.Dir(pointerPath), 0o755); err != nil {
		t.Fatalf("mkdir pointer dir: %v", err)
	}
	pointerContent := "pointer payload for fetch"
	if err := os.WriteFile(pointerPath, []byte(pointerContent), 0o644); err != nil {
		t.Fatalf("write pointer content: %v", err)
	}

	otherDir := t.TempDir()
	originalWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(otherDir); err != nil {
		t.Fatalf("chdir away from project root: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(originalWD)
	})

	repo := &fakeRepository{
		pointerLookupResults: []core.CandidatePointer{
			candidate("code:pointer", "docs/pointer.txt", false, []string{"backend"}),
		},
	}
	svc, err := NewWithProjectRoot(repo, projectRoot)
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	result, apiErr := svc.Fetch(context.Background(), v1.FetchPayload{
		ProjectID: "project.alpha",
		Keys:      []string{"code:pointer"},
	})
	if apiErr != nil {
		t.Fatalf("unexpected API error: %+v", apiErr)
	}
	if len(result.Items) != 1 {
		t.Fatalf("expected one fetch item, got %+v", result)
	}
	if result.Items[0].Content != pointerContent {
		t.Fatalf("unexpected pointer content: got %q want %q", result.Items[0].Content, pointerContent)
	}
}

func TestFetch_MemoryKeyReturnsFullContent(t *testing.T) {
	repo := &fakeRepository{
		memoryLookupResults: []core.ActiveMemory{
			memory(42, "Persisted memory", "full memory body", []string{"backend"}, []string{"code:pointer"}),
		},
	}
	svc, err := New(repo)
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	key := "mem:42"
	result, apiErr := svc.Fetch(context.Background(), v1.FetchPayload{
		ProjectID: "project.alpha",
		Keys:      []string{key},
	})
	if apiErr != nil {
		t.Fatalf("unexpected API error: %+v", apiErr)
	}
	if len(result.Items) != 1 {
		t.Fatalf("expected one fetch item, got %+v", result)
	}
	item := result.Items[0]
	if item.Key != key || item.Type != "memory" {
		t.Fatalf("unexpected memory fetch item identity: %+v", item)
	}
	if item.Content != "full memory body" {
		t.Fatalf("unexpected memory content: got %q want %q", item.Content, "full memory body")
	}
	if item.Summary != "Persisted memory" {
		t.Fatalf("unexpected memory summary: got %q want %q", item.Summary, "Persisted memory")
	}
	if strings.TrimSpace(item.Version) == "" {
		t.Fatalf("expected non-empty memory version: %+v", item)
	}
	if len(result.NotFound) != 0 || len(result.VersionMismatches) != 0 {
		t.Fatalf("unexpected fetch metadata: %+v", result)
	}
	if len(repo.memoryLookupCalls) != 1 {
		t.Fatalf("expected one memory lookup call, got %d", len(repo.memoryLookupCalls))
	}
	if repo.memoryLookupCalls[0].ProjectID != "project.alpha" || repo.memoryLookupCalls[0].MemoryID != 42 {
		t.Fatalf("unexpected memory lookup query: %+v", repo.memoryLookupCalls[0])
	}
}

func TestFetch_UnknownKeyReturnsNotFound(t *testing.T) {
	repo := &fakeRepository{}
	svc, err := New(repo)
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	result, apiErr := svc.Fetch(context.Background(), v1.FetchPayload{
		ProjectID: "project.alpha",
		Keys:      []string{"unknown:receipt.abc123"},
	})
	if apiErr != nil {
		t.Fatalf("unexpected API error: %+v", apiErr)
	}
	if len(result.Items) != 0 {
		t.Fatalf("expected zero fetch items, got %+v", result.Items)
	}
	if !reflect.DeepEqual(result.NotFound, []string{"unknown:receipt.abc123"}) {
		t.Fatalf("unexpected not_found: %+v", result.NotFound)
	}
	if len(repo.fetchLookupCalls) != 0 {
		t.Fatalf("did not expect fetch lookup call, got %d", len(repo.fetchLookupCalls))
	}
	if len(repo.pointerLookupCalls) != 1 {
		t.Fatalf("expected one pointer lookup for unknown pointer key, got %d", len(repo.pointerLookupCalls))
	}
}

func TestFetch_LookupErrorMapsInternalError(t *testing.T) {
	repo := &fakeRepository{
		workPlanLookupErrors: []error{errors.New("lookup failed")},
	}
	svc, err := New(repo)
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	_, apiErr := svc.Fetch(context.Background(), v1.FetchPayload{
		ProjectID: "project.alpha",
		Keys:      []string{"plan:receipt.abc123"},
	})
	if apiErr == nil {
		t.Fatal("expected API error")
	}
	if apiErr.Code != "INTERNAL_ERROR" {
		t.Fatalf("unexpected API error code: %q", apiErr.Code)
	}
	details, ok := apiErr.Details.(map[string]any)
	if !ok {
		t.Fatalf("expected map details, got %T", apiErr.Details)
	}
	if details["operation"] != "lookup_work_plan" {
		t.Fatalf("unexpected operation detail: %#v", details)
	}
}

func TestFetch_LegacyLookupErrorMapsLegacyOperation(t *testing.T) {
	repo := &fakeRepository{
		fetchLookupErrors: []error{errors.New("legacy lookup failed")},
	}
	svc, err := New(repo)
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	_, apiErr := svc.Fetch(context.Background(), v1.FetchPayload{
		ProjectID: "project.alpha",
		Keys:      []string{"plan:receipt.abc123"},
	})
	if apiErr == nil {
		t.Fatal("expected API error")
	}
	if apiErr.Code != "INTERNAL_ERROR" {
		t.Fatalf("unexpected API error code: %q", apiErr.Code)
	}
	details, ok := apiErr.Details.(map[string]any)
	if !ok {
		t.Fatalf("expected map details, got %T", apiErr.Details)
	}
	if details["operation"] != "lookup_fetch_state" {
		t.Fatalf("unexpected operation detail: %#v", details)
	}
}

func TestFetch_VersionMismatchIsReported(t *testing.T) {
	repo := &fakeRepository{
		workPlanLookupResult: []core.WorkPlan{{
			ProjectID: "project.alpha",
			PlanKey:   "plan:receipt.abc123",
			ReceiptID: "receipt.abc123",
			Status:    core.PlanStatusCompleted,
			UpdatedAt: time.Date(2026, 3, 5, 12, 0, 0, 0, time.UTC),
		}},
	}
	svc, err := New(repo)
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	key := "plan:receipt.abc123"
	result, apiErr := svc.Fetch(context.Background(), v1.FetchPayload{
		ProjectID: "project.alpha",
		Keys:      []string{key},
		ExpectedVersions: map[string]string{
			key: "99",
		},
	})
	if apiErr != nil {
		t.Fatalf("unexpected API error: %+v", apiErr)
	}
	if len(result.VersionMismatches) != 1 {
		t.Fatalf("expected one version mismatch, got %+v", result.VersionMismatches)
	}
	mismatch := result.VersionMismatches[0]
	if mismatch.Key != key || mismatch.Expected != "99" || strings.TrimSpace(mismatch.Actual) == "" || mismatch.Actual == "99" {
		t.Fatalf("unexpected version mismatch: %+v", mismatch)
	}
}

func TestWork_PersistsCompletedWorkItemsAndDerivesPlanStatus(t *testing.T) {
	repo := &fakeRepository{}
	svc, err := New(repo)
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	result, apiErr := svc.Work(context.Background(), v1.WorkPayload{
		ProjectID: "project.alpha",
		PlanKey:   "plan:receipt.abc123",
		ReceiptID: "receipt.abc123",
		Tasks: []v1.WorkTaskPayload{
			{Key: "src/b.go", Status: v1.WorkItemStatusComplete},
			{Key: "src/a.go", Status: v1.WorkItemStatusComplete},
			{Key: "src/a.go", Status: v1.WorkItemStatusComplete},
		},
	})
	if apiErr != nil {
		t.Fatalf("unexpected API error: %+v", apiErr)
	}
	if result.PlanKey != "plan:receipt.abc123" || result.PlanStatus != core.PlanStatusComplete || result.Updated != 2 || result.TaskCount != 2 {
		t.Fatalf("unexpected work result: %+v", result)
	}
	if len(repo.fetchLookupCalls) != 0 {
		t.Fatalf("did not expect fetch lookup calls, got %d", len(repo.fetchLookupCalls))
	}
	if len(repo.workPlanUpsertCalls) != 1 {
		t.Fatalf("expected one work plan upsert call, got %d", len(repo.workPlanUpsertCalls))
	}
	if len(repo.workUpsertCalls) != 1 {
		t.Fatalf("expected one mirrored work upsert call, got %d", len(repo.workUpsertCalls))
	}
	if len(repo.workListCalls) != 0 {
		t.Fatalf("did not expect work list calls, got %d", len(repo.workListCalls))
	}

	call := repo.workPlanUpsertCalls[0]
	wantItems := []core.WorkItem{
		{ItemKey: "src/a.go", Status: core.WorkItemStatusComplete},
		{ItemKey: "src/b.go", Status: core.WorkItemStatusComplete},
	}
	if !reflect.DeepEqual(call.Tasks, wantItems) {
		t.Fatalf("unexpected work plan tasks: got %+v want %+v", call.Tasks, wantItems)
	}
	mirrorCall := repo.workUpsertCalls[0]
	if !reflect.DeepEqual(mirrorCall.Items, wantItems) {
		t.Fatalf("unexpected mirrored work items: got %+v want %+v", mirrorCall.Items, wantItems)
	}
}

func TestWork_DerivesReceiptIDFromPlanKeyWhenReceiptIDOmitted(t *testing.T) {
	repo := &fakeRepository{}
	svc, err := New(repo)
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	result, apiErr := svc.Work(context.Background(), v1.WorkPayload{
		ProjectID: "project.alpha",
		PlanKey:   "plan:receipt.abc123",
		Tasks: []v1.WorkTaskPayload{
			{Key: "src/a.go", Status: v1.WorkItemStatusInProgress},
		},
	})
	if apiErr != nil {
		t.Fatalf("unexpected API error: %+v", apiErr)
	}
	if result.PlanKey != "plan:receipt.abc123" || result.PlanStatus != core.PlanStatusInProgress || result.Updated != 1 || result.TaskCount != 1 {
		t.Fatalf("unexpected work result: %+v", result)
	}
	if len(repo.fetchLookupCalls) != 0 {
		t.Fatalf("did not expect fetch lookup calls, got %d", len(repo.fetchLookupCalls))
	}
	if len(repo.workPlanUpsertCalls) != 1 {
		t.Fatalf("expected one work plan upsert call, got %d", len(repo.workPlanUpsertCalls))
	}
	if repo.workPlanUpsertCalls[0].ReceiptID != "receipt.abc123" {
		t.Fatalf("expected derived receipt id for plan upsert, got %q", repo.workPlanUpsertCalls[0].ReceiptID)
	}
	if len(repo.workUpsertCalls) != 1 {
		t.Fatalf("expected one mirrored work upsert call, got %d", len(repo.workUpsertCalls))
	}
	if repo.workUpsertCalls[0].ReceiptID != "receipt.abc123" {
		t.Fatalf("expected derived receipt id for mirrored upsert, got %q", repo.workUpsertCalls[0].ReceiptID)
	}
	if len(repo.workListCalls) != 0 {
		t.Fatalf("did not expect work list calls, got %d", len(repo.workListCalls))
	}
}

func TestWork_ReceiptIDOnlyAllowsStatusCheckAndDerivesPlanKey(t *testing.T) {
	repo := &fakeRepository{}
	svc, err := New(repo)
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	result, apiErr := svc.Work(context.Background(), v1.WorkPayload{
		ProjectID: "project.alpha",
		ReceiptID: "receipt.abc123",
	})
	if apiErr != nil {
		t.Fatalf("unexpected API error: %+v", apiErr)
	}
	if result.PlanKey != "plan:receipt.abc123" || result.PlanStatus != core.PlanStatusPending || result.Updated != 0 || result.TaskCount != 0 {
		t.Fatalf("unexpected work result: %+v", result)
	}
	if len(repo.workPlanUpsertCalls) != 1 {
		t.Fatalf("expected one work plan upsert call, got %d", len(repo.workPlanUpsertCalls))
	}
	if len(repo.workUpsertCalls) != 0 {
		t.Fatalf("did not expect work upsert calls, got %d", len(repo.workUpsertCalls))
	}
	if len(repo.workListCalls) != 0 {
		t.Fatalf("did not expect work list calls, got %d", len(repo.workListCalls))
	}
	upsert := repo.workPlanUpsertCalls[0]
	if upsert.PlanKey != "plan:receipt.abc123" || upsert.ReceiptID != "receipt.abc123" {
		t.Fatalf("unexpected plan upsert identifiers: %+v", upsert)
	}
}

func TestWork_MissingPlanAndReceiptReturnsInvalidInput(t *testing.T) {
	repo := &fakeRepository{}
	svc, err := New(repo)
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	_, apiErr := svc.Work(context.Background(), v1.WorkPayload{
		ProjectID: "project.alpha",
		Tasks: []v1.WorkTaskPayload{
			{Key: "src/a.go", Status: v1.WorkItemStatusComplete},
		},
	})
	if apiErr == nil {
		t.Fatal("expected API error")
	}
	if apiErr.Code != "INVALID_INPUT" {
		t.Fatalf("unexpected API error code: %q", apiErr.Code)
	}
	if !strings.Contains(apiErr.Message, "plan_key or receipt_id is required") {
		t.Fatalf("unexpected API error message: %q", apiErr.Message)
	}
	details, ok := apiErr.Details.(map[string]any)
	if !ok {
		t.Fatalf("expected map details, got %T", apiErr.Details)
	}
	if details["plan_key"] != "" {
		t.Fatalf("unexpected plan_key detail: %#v", details)
	}
	if len(repo.fetchLookupCalls) != 0 {
		t.Fatalf("did not expect fetch lookup call, got %d", len(repo.fetchLookupCalls))
	}
}

func TestWork_InvalidPlanKeyFormatReturnsInvalidInput(t *testing.T) {
	repo := &fakeRepository{}
	svc, err := New(repo)
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	_, apiErr := svc.Work(context.Background(), v1.WorkPayload{
		ProjectID: "project.alpha",
		PlanKey:   "plan.release.v1",
		Tasks: []v1.WorkTaskPayload{
			{Key: "src/a.go", Summary: "update", Status: v1.WorkItemStatusComplete},
		},
	})
	if apiErr == nil {
		t.Fatal("expected API error")
	}
	if apiErr.Code != "INVALID_INPUT" {
		t.Fatalf("unexpected API error code: %q", apiErr.Code)
	}
	if !strings.Contains(apiErr.Message, "plan_key must use format plan:<receipt_id>") {
		t.Fatalf("unexpected API error message: %q", apiErr.Message)
	}
}

func TestWork_PlanKeyWhitespaceReturnsInvalidInput(t *testing.T) {
	repo := &fakeRepository{}
	svc, err := New(repo)
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	_, apiErr := svc.Work(context.Background(), v1.WorkPayload{
		ProjectID: "project.alpha",
		PlanKey:   " plan:receipt.abc123 ",
		Tasks: []v1.WorkTaskPayload{
			{Key: "src/a.go", Summary: "update", Status: v1.WorkItemStatusComplete},
		},
	})
	if apiErr == nil {
		t.Fatal("expected API error")
	}
	if apiErr.Code != "INVALID_INPUT" {
		t.Fatalf("unexpected API error code: %q", apiErr.Code)
	}
	if !strings.Contains(apiErr.Message, "plan_key must not include surrounding whitespace") {
		t.Fatalf("unexpected API error message: %q", apiErr.Message)
	}
}

func TestWork_PlanKeyReceiptMismatchReturnsInvalidInput(t *testing.T) {
	repo := &fakeRepository{}
	svc, err := New(repo)
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	_, apiErr := svc.Work(context.Background(), v1.WorkPayload{
		ProjectID: "project.alpha",
		PlanKey:   "plan:receipt.abc123",
		ReceiptID: "receipt.other",
		Tasks: []v1.WorkTaskPayload{
			{Key: "src/a.go", Summary: "update", Status: v1.WorkItemStatusComplete},
		},
	})
	if apiErr == nil {
		t.Fatal("expected API error")
	}
	if apiErr.Code != "INVALID_INPUT" {
		t.Fatalf("unexpected API error code: %q", apiErr.Code)
	}
	if !strings.Contains(apiErr.Message, "plan_key and receipt_id must reference the same receipt") {
		t.Fatalf("unexpected API error message: %q", apiErr.Message)
	}
}

func TestWork_UpsertPlanErrorMapsInternalError(t *testing.T) {
	repo := &fakeRepository{
		workPlanUpsertErrors: []error{errors.New("plan upsert failed")},
	}
	svc, err := New(repo)
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	_, apiErr := svc.Work(context.Background(), v1.WorkPayload{
		ProjectID: "project.alpha",
		PlanKey:   "plan:receipt.abc123",
		ReceiptID: "receipt.abc123",
		Tasks: []v1.WorkTaskPayload{
			{Key: "src/a.go", Status: v1.WorkItemStatusComplete},
		},
	})
	if apiErr == nil {
		t.Fatal("expected API error")
	}
	if apiErr.Code != "INTERNAL_ERROR" {
		t.Fatalf("unexpected API error code: %q", apiErr.Code)
	}
	details, ok := apiErr.Details.(map[string]any)
	if !ok {
		t.Fatalf("expected map details, got %T", apiErr.Details)
	}
	if details["operation"] != "upsert_work_plan" {
		t.Fatalf("unexpected operation detail: %#v", details)
	}
}

func TestWork_UpsertErrorMapsInternalError(t *testing.T) {
	repo := &fakeRepository{
		fetchLookupResults: []core.FetchLookup{{
			ProjectID: "project.alpha",
			ReceiptID: "receipt.abc123",
			RunID:     8,
		}},
		workUpsertErrors: []error{errors.New("upsert failed")},
	}
	svc, err := New(repo)
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	_, apiErr := svc.Work(context.Background(), v1.WorkPayload{
		ProjectID: "project.alpha",
		PlanKey:   "plan:receipt.abc123",
		ReceiptID: "receipt.abc123",
		Tasks: []v1.WorkTaskPayload{
			{Key: "src/a.go", Status: v1.WorkItemStatusComplete},
		},
	})
	if apiErr == nil {
		t.Fatal("expected API error")
	}
	if apiErr.Code != "INTERNAL_ERROR" {
		t.Fatalf("unexpected API error code: %q", apiErr.Code)
	}
	details, ok := apiErr.Details.(map[string]any)
	if !ok {
		t.Fatalf("expected map details, got %T", apiErr.Details)
	}
	if details["operation"] != "upsert_work_items" {
		t.Fatalf("unexpected operation detail: %#v", details)
	}
}

func TestWork_ListErrorsAreIgnoredWhenPlanRepositoryAvailable(t *testing.T) {
	repo := &fakeRepository{
		workListErrors: []error{errors.New("list failed")},
	}
	svc, err := New(repo)
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	result, apiErr := svc.Work(context.Background(), v1.WorkPayload{
		ProjectID: "project.alpha",
		PlanKey:   "plan:receipt.abc123",
		ReceiptID: "receipt.abc123",
		Tasks: []v1.WorkTaskPayload{
			{Key: "src/a.go", Status: v1.WorkItemStatusComplete},
		},
	})
	if apiErr != nil {
		t.Fatalf("unexpected API error: %+v", apiErr)
	}
	if result.PlanStatus != core.PlanStatusComplete || result.Updated != 1 {
		t.Fatalf("unexpected work result: %+v", result)
	}
	if len(repo.workListCalls) != 0 {
		t.Fatalf("did not expect list_work_items calls, got %d", len(repo.workListCalls))
	}
}

func TestWorkPayloadTasks_UsesPayloadStatuses(t *testing.T) {
	payload := v1.WorkPayload{
		Tasks: []v1.WorkTaskPayload{
			{Key: " src/a.go ", Summary: "implement", Status: v1.WorkItemStatusInProgress, Outcome: "started"},
			{Key: "src/a.go", Summary: "blocked by dependency", Status: v1.WorkItemStatusBlocked, Outcome: "waiting"},
			{Key: "src/b.go", Summary: "verify", Status: v1.WorkItemStatusComplete, Outcome: "done"},
		},
	}

	items := workPayloadTasks(payload)
	want := []core.WorkItem{
		{ItemKey: "src/a.go", Summary: "blocked by dependency", Status: core.WorkItemStatusBlocked, Outcome: "waiting"},
		{ItemKey: "src/b.go", Summary: "verify", Status: core.WorkItemStatusComplete, Outcome: "done"},
	}
	if !reflect.DeepEqual(items, want) {
		t.Fatalf("unexpected normalized items: got %+v want %+v", items, want)
	}
}

func TestDerivePlanStatusFromWorkItems_Deterministic(t *testing.T) {
	tests := []struct {
		name  string
		items []core.WorkItem
		want  string
	}{
		{name: "empty defaults pending", items: nil, want: core.PlanStatusPending},
		{name: "all completed", items: []core.WorkItem{{Status: core.WorkItemStatusCompleted}, {Status: core.WorkItemStatusCompleted}}, want: core.PlanStatusComplete},
		{name: "pending dominates completed", items: []core.WorkItem{{Status: core.WorkItemStatusCompleted}, {Status: core.WorkItemStatusPending}}, want: core.PlanStatusPending},
		{name: "in progress dominates pending", items: []core.WorkItem{{Status: core.WorkItemStatusPending}, {Status: core.WorkItemStatusInProgress}}, want: core.PlanStatusInProgress},
		{name: "blocked dominates all", items: []core.WorkItem{{Status: core.WorkItemStatusCompleted}, {Status: core.WorkItemStatusBlocked}, {Status: core.WorkItemStatusInProgress}}, want: core.PlanStatusBlocked},
		{name: "unknown treated as pending", items: []core.WorkItem{{Status: "unknown"}}, want: core.PlanStatusPending},
	}

	for _, tc := range tests {
		if got := derivePlanStatusFromWorkItems(tc.items); got != tc.want {
			t.Fatalf("%s: got %q want %q", tc.name, got, tc.want)
		}
	}
}

func TestWork_UpsertsPlanAndTasksWhenPlanRepositoryAvailable(t *testing.T) {
	repo := &fakeRepository{
		workPlanUpsertResult: []core.WorkPlanUpsertResult{{
			Plan: core.WorkPlan{
				ProjectID: "project.alpha",
				PlanKey:   "plan:receipt.abc123",
				Status:    core.PlanStatusInProgress,
				Tasks: []core.WorkItem{
					{ItemKey: "task.alpha", Status: core.WorkItemStatusInProgress},
				},
			},
			Updated: 1,
		}},
	}
	svc, err := New(repo)
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	result, apiErr := svc.Work(context.Background(), v1.WorkPayload{
		ProjectID: "project.alpha",
		PlanKey:   "plan:receipt.abc123",
		ReceiptID: "receipt.abc123",
		Mode:      v1.WorkPlanModeMerge,
		Plan: &v1.WorkPlanPayload{
			Title:     "Import Optimization",
			Objective: "Track execution centrally in acm",
		},
		Tasks: []v1.WorkTaskPayload{
			{
				Key:                "task.alpha",
				Summary:            "Audit pagination behavior",
				Status:             v1.WorkItemStatusInProgress,
				AcceptanceCriteria: []string{"No truncation above provider limits"},
			},
		},
	})
	if apiErr != nil {
		t.Fatalf("unexpected API error: %+v", apiErr)
	}

	if result.PlanKey != "plan:receipt.abc123" || result.PlanStatus != core.PlanStatusInProgress || result.Updated != 1 || result.TaskCount != 1 {
		t.Fatalf("unexpected work result: %+v", result)
	}
	if len(repo.workPlanUpsertCalls) != 1 {
		t.Fatalf("expected one work plan upsert call, got %d", len(repo.workPlanUpsertCalls))
	}
	call := repo.workPlanUpsertCalls[0]
	if call.Mode != core.WorkPlanModeMerge {
		t.Fatalf("unexpected plan mode: %q", call.Mode)
	}
	if call.Title != "Import Optimization" {
		t.Fatalf("unexpected plan title: %q", call.Title)
	}
	if len(call.Tasks) != 1 {
		t.Fatalf("expected one task in plan upsert, got %d", len(call.Tasks))
	}
	if call.Tasks[0].ItemKey != "task.alpha" || call.Tasks[0].Summary != "Audit pagination behavior" {
		t.Fatalf("unexpected upserted task: %+v", call.Tasks[0])
	}
	if len(repo.workUpsertCalls) != 1 {
		t.Fatalf("expected mirrored work item upsert for receipt-scoped DoD checks")
	}
}

func TestFetchPlanItem_UsesStoredPlanRepositoryContent(t *testing.T) {
	repo := &fakeRepository{
		workPlanLookupResult: []core.WorkPlan{{
			ProjectID: "project.alpha",
			PlanKey:   "plan:receipt.abc123",
			ReceiptID: "receipt.abc123",
			Title:     "Import Optimization",
			Objective: "Consolidate planning state in acm",
			Status:    core.PlanStatusBlocked,
			Stages: core.WorkPlanStages{
				SpecOutline:        core.PlanStatusComplete,
				RefinedSpec:        core.PlanStatusInProgress,
				ImplementationPlan: core.PlanStatusPending,
			},
			Tasks: []core.WorkItem{
				{
					ItemKey:            "kh-22",
					Summary:            "Audit pagination",
					Status:             core.WorkItemStatusBlocked,
					BlockedReason:      "Awaiting upstream API quota",
					AcceptanceCriteria: []string{"Imports paginate beyond provider limits"},
				},
			},
		}},
	}
	svc, err := New(repo)
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	item, found, err := svc.fetchPlanItem(context.Background(), "project.alpha", "plan:receipt.abc123", "receipt.abc123")
	if err != nil {
		t.Fatalf("fetchPlanItem returned error: %v", err)
	}
	if !found {
		t.Fatal("expected plan item to be found")
	}
	if item.Type != "plan" || item.Status != core.PlanStatusBlocked {
		t.Fatalf("unexpected fetch item metadata: %+v", item)
	}
	if !strings.Contains(item.Content, "\"objective\":\"Consolidate planning state in acm\"") {
		t.Fatalf("expected serialized plan content to include objective, got %s", item.Content)
	}
	if len(repo.workPlanLookupCalls) != 1 {
		t.Fatalf("expected one plan lookup call, got %d", len(repo.workPlanLookupCalls))
	}
	if repo.workPlanLookupCalls[0].PlanKey != "plan:receipt.abc123" {
		t.Fatalf("unexpected plan lookup key: %+v", repo.workPlanLookupCalls[0])
	}
}

func TestMakeContextPlans_UsesStoredPlanSummariesWhenAvailable(t *testing.T) {
	repo := &fakeRepository{
		workPlanListResults: [][]core.WorkPlanSummary{{
			{
				PlanKey:             "plan:receipt.abc123",
				Summary:             "Import Optimization (3 tasks)",
				Status:              core.PlanStatusInProgress,
				ActiveTaskKeys:      []string{"task.blocked", "task.active"},
				TaskCountTotal:      3,
				TaskCountPending:    1,
				TaskCountInProgress: 1,
				TaskCountBlocked:    1,
				TaskCountComplete:   0,
			},
		}},
	}
	svc, err := New(repo)
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	plans := svc.makeContextPlans(context.Background(), "project.alpha", "receipt.abc123", false)
	if len(plans) != 1 {
		t.Fatalf("expected one persisted context plan, got %+v", plans)
	}
	if plans[0].Key != "plan:receipt.abc123" || plans[0].Status != v1.WorkItemStatusInProgress {
		t.Fatalf("unexpected context plan: %+v", plans[0])
	}
	if !reflect.DeepEqual(plans[0].FetchKeys, []string{"plan:receipt.abc123"}) {
		t.Fatalf("unexpected plan fetch keys: %+v", plans[0].FetchKeys)
	}
	if len(repo.workPlanLookupCalls) != 0 {
		t.Fatalf("did not expect per-plan lookups, got %d", len(repo.workPlanLookupCalls))
	}
}

func TestMakeContextPlans_UsesPlanOnlyFetchKeys(t *testing.T) {
	repo := &fakeRepository{
		workPlanListResults: [][]core.WorkPlanSummary{{
			{
				PlanKey:        "plan:receipt.abc123",
				Summary:        "Import Optimization",
				Status:         core.PlanStatusInProgress,
				ActiveTaskKeys: []string{strings.Repeat("t", 600), "task.short"},
			},
		}},
	}
	svc, err := New(repo)
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	plans := svc.makeContextPlans(context.Background(), "project.alpha", "receipt.abc123", false)
	if len(plans) != 1 {
		t.Fatalf("expected one persisted context plan, got %+v", plans)
	}
	if !reflect.DeepEqual(plans[0].FetchKeys, []string{"plan:receipt.abc123"}) {
		t.Fatalf("unexpected filtered plan fetch keys: %+v", plans[0].FetchKeys)
	}
	if len(repo.workPlanListCalls) != 1 || repo.workPlanListCalls[0].Scope != string(v1.HistoryScopeCurrent) {
		t.Fatalf("expected current-scope work plan lookup, got %+v", repo.workPlanListCalls)
	}
}

func TestHistorySearch_MapsPlanSummariesAndDefaults(t *testing.T) {
	repo := &fakeRepository{
		workPlanListResults: [][]core.WorkPlanSummary{{
			{
				PlanKey:             "plan:receipt.active123",
				ReceiptID:           "receipt.active123",
				Title:               "Bootstrap follow-up",
				Objective:           "Verify first-run onboarding and history search",
				Summary:             "Bootstrap follow-up (2 tasks)",
				Status:              core.PlanStatusInProgress,
				Kind:                "story",
				ParentPlanKey:       "plan:receipt.parent123",
				ActiveTaskKeys:      []string{"task.verify", "task.docs"},
				TaskCountTotal:      2,
				TaskCountPending:    0,
				TaskCountInProgress: 1,
				TaskCountBlocked:    0,
				TaskCountComplete:   1,
				UpdatedAt:           time.Date(2026, 3, 6, 15, 4, 5, 0, time.UTC),
			},
			{
				PlanKey:             "plan:receipt.done123",
				ReceiptID:           "receipt.done123",
				Title:               "Released v1",
				Objective:           "Ship the initial history surface",
				Summary:             "Released v1 (3 tasks)",
				Status:              core.PlanStatusCompleted,
				Kind:                "feature",
				TaskCountTotal:      3,
				TaskCountPending:    0,
				TaskCountInProgress: 0,
				TaskCountBlocked:    0,
				TaskCountComplete:   3,
				UpdatedAt:           time.Date(2026, 3, 6, 16, 5, 6, 0, time.UTC),
			},
		}},
	}
	svc, err := New(repo)
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	result, apiErr := svc.HistorySearch(context.Background(), v1.HistorySearchPayload{
		ProjectID: "project.alpha",
		Query:     " bootstrap ",
	})
	if apiErr != nil {
		t.Fatalf("unexpected API error: %+v", apiErr)
	}
	if len(repo.workPlanListCalls) != 1 {
		t.Fatalf("expected one work plan list call, got %d", len(repo.workPlanListCalls))
	}
	if got := repo.workPlanListCalls[0]; got.ProjectID != "project.alpha" || got.Query != "bootstrap" || got.Scope != string(v1.HistoryScopeAll) || got.Limit != 20 || got.Unbounded {
		t.Fatalf("unexpected work plan list query: %+v", got)
	}
	if result.Entity != v1.HistoryEntityAll || result.Scope != "" || result.Query != "bootstrap" || result.Limit != 20 || result.Count != 2 {
		t.Fatalf("unexpected history result metadata: %+v", result)
	}
	if len(result.Items) != 2 {
		t.Fatalf("expected two items, got %+v", result.Items)
	}
	itemsByKey := map[string]v1.HistoryItem{}
	for _, item := range result.Items {
		itemsByKey[item.Key] = item
	}
	if item := itemsByKey["plan:receipt.active123"]; item.Entity != v1.HistoryEntityWork || item.Scope != v1.HistoryScopeCurrent || !reflect.DeepEqual(item.FetchKeys, []string{"plan:receipt.active123"}) {
		t.Fatalf("unexpected active plan summary: %+v", item)
	}
	if item := itemsByKey["plan:receipt.done123"]; item.Entity != v1.HistoryEntityWork || item.Scope != v1.HistoryScopeCompleted || !reflect.DeepEqual(item.FetchKeys, []string{"plan:receipt.done123"}) {
		t.Fatalf("unexpected completed plan summary: %+v", item)
	}
}

func TestHistorySearch_UsesExplicitScopeKindAndLimit(t *testing.T) {
	repo := &fakeRepository{
		workPlanListResults: [][]core.WorkPlanSummary{{}},
	}
	svc, err := New(repo)
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	result, apiErr := svc.HistorySearch(context.Background(), v1.HistorySearchPayload{
		ProjectID: "project.alpha",
		Entity:    v1.HistoryEntityWork,
		Scope:     v1.HistoryScopeDeferred,
		Kind:      "story",
		Limit:     7,
	})
	if apiErr != nil {
		t.Fatalf("unexpected API error: %+v", apiErr)
	}
	if got := repo.workPlanListCalls[0]; got.Scope != string(v1.HistoryScopeDeferred) || got.Kind != "story" || got.Limit != 7 || got.Unbounded {
		t.Fatalf("unexpected explicit history query: %+v", got)
	}
	if result.Entity != v1.HistoryEntityWork || result.Scope != v1.HistoryScopeDeferred || result.Limit != 7 || result.Count != 0 || len(result.Items) != 0 {
		t.Fatalf("unexpected history result: %+v", result)
	}
}

func TestHistorySearch_UnboundedUsesAllScopeWithoutLimitCap(t *testing.T) {
	repo := &fakeRepository{
		workPlanListResults: [][]core.WorkPlanSummary{{}},
	}
	svc, err := New(repo)
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	value := true
	result, apiErr := svc.HistorySearch(context.Background(), v1.HistorySearchPayload{
		ProjectID: "project.alpha",
		Query:     "bootstrap",
		Unbounded: &value,
	})
	if apiErr != nil {
		t.Fatalf("unexpected API error: %+v", apiErr)
	}
	if got := repo.workPlanListCalls[0]; !got.Unbounded || got.Limit != 0 || got.Scope != string(v1.HistoryScopeAll) {
		t.Fatalf("unexpected unbounded history query: %+v", got)
	}
	if result.Limit != 0 {
		t.Fatalf("expected unbounded history result limit 0, got %+v", result)
	}
}

func TestHistorySearch_MemoryEntityReturnsCompactMemoryItems(t *testing.T) {
	repo := &fakeRepository{
		memoryHistoryResults: [][]core.MemoryHistorySummary{{
			{
				MemoryID:   17,
				Category:   "implementation",
				Subject:    "Prefer work search for archived plans",
				Content:    "Fallback content",
				Confidence: 4,
				UpdatedAt:  time.Date(2026, 3, 6, 13, 0, 0, 0, time.UTC),
			},
		}},
	}
	svc, err := New(repo)
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	result, apiErr := svc.HistorySearch(context.Background(), v1.HistorySearchPayload{
		ProjectID: "project.alpha",
		Entity:    v1.HistoryEntityMemory,
		Query:     "archived plans",
		Limit:     15,
	})
	if apiErr != nil {
		t.Fatalf("unexpected API error: %+v", apiErr)
	}
	if result.Entity != v1.HistoryEntityMemory || result.Count != 1 || len(result.Items) != 1 {
		t.Fatalf("unexpected history result: %+v", result)
	}
	item := result.Items[0]
	if item.Key != "mem:17" || item.Entity != v1.HistoryEntityMemory || item.Kind != "implementation" {
		t.Fatalf("unexpected memory history item: %+v", item)
	}
	if !reflect.DeepEqual(item.FetchKeys, []string{"mem:17"}) {
		t.Fatalf("unexpected memory fetch keys: %+v", item.FetchKeys)
	}
	if len(repo.memoryHistoryCalls) != 1 {
		t.Fatalf("expected one memory history query, got %d", len(repo.memoryHistoryCalls))
	}
	if got := repo.memoryHistoryCalls[0]; got.ProjectID != "project.alpha" || got.Query != "archived plans" || got.Limit != 15 || got.Unbounded {
		t.Fatalf("unexpected memory history query: %+v", got)
	}
}

func TestHistorySearch_AllEntitiesReturnsMemoriesReceiptsRunsAndWork(t *testing.T) {
	repo := &fakeRepository{
		memoryHistoryResults: [][]core.MemoryHistorySummary{{
			{
				MemoryID:   17,
				Category:   "implementation",
				Subject:    "Memory history entry",
				Content:    "Keep work items compact",
				Confidence: 5,
				UpdatedAt:  time.Date(2026, 3, 6, 13, 0, 0, 0, time.UTC),
			},
		}},
		workPlanListResults: [][]core.WorkPlanSummary{{
			{
				PlanKey:   "plan:receipt.work123",
				ReceiptID: "receipt.work123",
				Summary:   "Current plan",
				Status:    core.PlanStatusInProgress,
				UpdatedAt: time.Date(2026, 3, 6, 10, 0, 0, 0, time.UTC),
			},
		}},
		receiptHistoryResults: [][]core.ReceiptHistorySummary{{
			{
				ReceiptID:       "receipt.receipt123",
				TaskText:        "Receipt history entry",
				Phase:           string(v1.PhaseReview),
				LatestRequestID: "req-12345678",
				LatestStatus:    "accepted",
				UpdatedAt:       time.Date(2026, 3, 6, 11, 0, 0, 0, time.UTC),
			},
		}},
		runHistoryResults: [][]core.RunHistorySummary{{
			{
				RunID:     33,
				ReceiptID: "receipt.run123",
				RequestID: "req-87654321",
				TaskText:  "Run history entry",
				Phase:     string(v1.PhaseExecute),
				Status:    "accepted",
				Outcome:   "Run completed",
				UpdatedAt: time.Date(2026, 3, 6, 12, 0, 0, 0, time.UTC),
			},
		}},
	}
	svc, err := New(repo)
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	result, apiErr := svc.HistorySearch(context.Background(), v1.HistorySearchPayload{
		ProjectID: "project.alpha",
		Entity:    v1.HistoryEntityAll,
	})
	if apiErr != nil {
		t.Fatalf("unexpected API error: %+v", apiErr)
	}
	if result.Entity != v1.HistoryEntityAll || result.Count != 4 || len(result.Items) != 4 {
		t.Fatalf("unexpected history result: %+v", result)
	}
	if result.Items[0].Entity != v1.HistoryEntityMemory || result.Items[0].Key != "mem:17" {
		t.Fatalf("expected newest memory item first, got %+v", result.Items)
	}
	if result.Items[1].Entity != v1.HistoryEntityRun || result.Items[1].Key != "run:33" {
		t.Fatalf("expected run item second, got %+v", result.Items)
	}
	if result.Items[2].Entity != v1.HistoryEntityReceipt || result.Items[2].Key != "receipt:receipt.receipt123" {
		t.Fatalf("expected receipt item third, got %+v", result.Items)
	}
	if result.Items[3].Entity != v1.HistoryEntityWork || result.Items[3].Key != "plan:receipt.work123" {
		t.Fatalf("expected work item fourth, got %+v", result.Items)
	}
	if len(repo.workPlanListCalls) != 1 || repo.workPlanListCalls[0].Scope != string(v1.HistoryScopeAll) {
		t.Fatalf("unexpected work history query: %+v", repo.workPlanListCalls)
	}
	if len(repo.memoryHistoryCalls) != 1 || len(repo.receiptHistoryCalls) != 1 || len(repo.runHistoryCalls) != 1 {
		t.Fatalf("expected one memory/receipt/run history query, got memories=%d receipts=%d runs=%d", len(repo.memoryHistoryCalls), len(repo.receiptHistoryCalls), len(repo.runHistoryCalls))
	}
}

func TestReview_RunExecutesWorkflowCommandAndRecordsCompleteTask(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".acm"), 0o755); err != nil {
		t.Fatalf("mkdir .acm: %v", err)
	}
	workflowsYAML := "version: acm.workflows.v1\ncompletion:\n  required_tasks:\n    - key: review:cross-llm\n      summary: Cross-LLM review\n      run:\n        argv: [\"scripts/acm-cross-review.sh\"]\n        cwd: .\n        timeout_sec: 600\n        env:\n          ACM_REVIEW_PROVIDER: codex\n"
	if err := os.WriteFile(filepath.Join(root, ".acm", "acm-workflows.yaml"), []byte(workflowsYAML), 0o644); err != nil {
		t.Fatalf("write workflows file: %v", err)
	}

	repo := &fakeRepository{
		scopeResults: []core.ReceiptScope{{
			ProjectID: "project.alpha",
			ReceiptID: "receipt.abc123",
		}},
	}
	svc, err := NewWithProjectRoot(repo, root)
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	var gotCommand workflowRunDefinition
	var gotEnv map[string]string
	svc.runReviewCommand = func(_ context.Context, projectRoot string, command workflowRunDefinition, extraEnv map[string]string) verifyCommandRun {
		if projectRoot != root {
			t.Fatalf("unexpected project root: got %q want %q", projectRoot, root)
		}
		gotCommand = command
		gotEnv = extraEnv
		exitCode := 0
		now := time.Now().UTC()
		return verifyCommandRun{
			ExitCode:   &exitCode,
			Stdout:     "PASS: Cross-LLM review passed with no blocking findings.",
			StartedAt:  now,
			FinishedAt: now.Add(250 * time.Millisecond),
		}
	}

	result, apiErr := svc.Review(context.Background(), v1.ReviewPayload{
		ProjectID: "project.alpha",
		ReceiptID: "receipt.abc123",
		Run:       true,
	})
	if apiErr != nil {
		t.Fatalf("unexpected API error: %+v", apiErr)
	}
	if !result.Executed || result.ReviewKey != v1.DefaultReviewTaskKey || result.ReviewStatus != v1.WorkItemStatusComplete {
		t.Fatalf("unexpected review result: %+v", result)
	}
	if result.AttemptsRun != 1 || result.MaxAttempts != 0 || result.PassingRuns != 1 {
		t.Fatalf("unexpected review attempt counts: %+v", result)
	}
	if result.Execution == nil || result.Execution.TimeoutSec != 600 || len(result.Execution.CommandArgv) != 1 || result.Execution.CommandArgv[0] != "scripts/acm-cross-review.sh" {
		t.Fatalf("unexpected execution payload: %+v", result.Execution)
	}
	if got := gotCommand.Env["ACM_REVIEW_PROVIDER"]; got != "codex" {
		t.Fatalf("unexpected command env: got %q want %q", got, "codex")
	}
	if gotEnv["ACM_PLAN_KEY"] != "plan:receipt.abc123" || gotEnv["ACM_REVIEW_KEY"] != v1.DefaultReviewTaskKey {
		t.Fatalf("unexpected injected env: %+v", gotEnv)
	}
	if gotEnv["ACM_REVIEW_ATTEMPT"] != "1" || gotEnv["ACM_REVIEW_MAX_ATTEMPTS"] != "0" {
		t.Fatalf("unexpected review attempt env: %+v", gotEnv)
	}
	if len(repo.reviewAttemptCalls) != 1 || repo.reviewAttemptCalls[0].Status != "passed" {
		t.Fatalf("expected one saved passing review attempt, got %+v", repo.reviewAttemptCalls)
	}
	if len(repo.workPlanUpsertCalls) != 1 || len(repo.workPlanUpsertCalls[0].Tasks) != 1 {
		t.Fatalf("expected one work plan upsert with one task, got %+v", repo.workPlanUpsertCalls)
	}
	task := repo.workPlanUpsertCalls[0].Tasks[0]
	if task.ItemKey != v1.DefaultReviewTaskKey || task.Status != string(v1.WorkItemStatusComplete) {
		t.Fatalf("unexpected recorded review task: %+v", task)
	}
	if task.Summary != "Cross-LLM review" {
		t.Fatalf("unexpected review summary: %q", task.Summary)
	}
	if !strings.Contains(task.Outcome, "PASS:") {
		t.Fatalf("unexpected review outcome: %q", task.Outcome)
	}
}

func TestReview_RunRecordsBlockedTaskWhenCommandFails(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".acm"), 0o755); err != nil {
		t.Fatalf("mkdir .acm: %v", err)
	}
	workflowsYAML := "version: acm.workflows.v1\ncompletion:\n  required_tasks:\n    - key: review:cross-llm\n      run:\n        argv: [\"scripts/acm-cross-review.sh\"]\n        timeout_sec: 300\n"
	if err := os.WriteFile(filepath.Join(root, ".acm", "acm-workflows.yaml"), []byte(workflowsYAML), 0o644); err != nil {
		t.Fatalf("write workflows file: %v", err)
	}

	repo := &fakeRepository{
		scopeResults: []core.ReceiptScope{{
			ProjectID: "project.alpha",
			ReceiptID: "receipt.abc123",
		}},
	}
	svc, err := NewWithProjectRoot(repo, root)
	if err != nil {
		t.Fatalf("new service: %v", err)
	}
	svc.runReviewCommand = func(_ context.Context, _ string, _ workflowRunDefinition, _ map[string]string) verifyCommandRun {
		exitCode := 1
		now := time.Now().UTC()
		return verifyCommandRun{
			ExitCode:   &exitCode,
			Stdout:     "FAIL: Missing schema update coverage.",
			StartedAt:  now,
			FinishedAt: now.Add(100 * time.Millisecond),
			Err:        errors.New("exit status 1"),
		}
	}

	result, apiErr := svc.Review(context.Background(), v1.ReviewPayload{
		ProjectID: "project.alpha",
		ReceiptID: "receipt.abc123",
		Run:       true,
	})
	if apiErr != nil {
		t.Fatalf("unexpected API error: %+v", apiErr)
	}
	if result.ReviewStatus != v1.WorkItemStatusBlocked {
		t.Fatalf("expected blocked review status, got %+v", result)
	}
	if result.AttemptsRun != 1 || result.MaxAttempts != 0 || result.PassingRuns != 0 {
		t.Fatalf("unexpected review attempt counts: %+v", result)
	}
	if len(repo.reviewAttemptCalls) != 1 || repo.reviewAttemptCalls[0].Status != "failed" {
		t.Fatalf("expected one saved failed review attempt, got %+v", repo.reviewAttemptCalls)
	}
	if len(repo.workPlanUpsertCalls) != 1 || len(repo.workPlanUpsertCalls[0].Tasks) != 1 {
		t.Fatalf("expected one work plan upsert with one task, got %+v", repo.workPlanUpsertCalls)
	}
	task := repo.workPlanUpsertCalls[0].Tasks[0]
	if task.Status != string(v1.WorkItemStatusBlocked) {
		t.Fatalf("unexpected blocked task status: %+v", task)
	}
	if !strings.Contains(task.Outcome, "Missing schema update coverage") {
		t.Fatalf("unexpected blocked outcome: %q", task.Outcome)
	}
}

func TestReview_RunInjectsDerivedReceiptIDForPlanKeyOnlyRequests(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".acm"), 0o755); err != nil {
		t.Fatalf("mkdir .acm: %v", err)
	}
	workflowsYAML := "version: acm.workflows.v1\ncompletion:\n  required_tasks:\n    - key: review:cross-llm\n      summary: Cross-LLM review\n      run:\n        argv: [\"scripts/acm-cross-review.sh\"]\n        timeout_sec: 600\n"
	if err := os.WriteFile(filepath.Join(root, ".acm", "acm-workflows.yaml"), []byte(workflowsYAML), 0o644); err != nil {
		t.Fatalf("write workflows file: %v", err)
	}

	repo := &fakeRepository{
		scopeResults: []core.ReceiptScope{{
			ProjectID: "project.alpha",
			ReceiptID: "receipt.abc123",
		}},
	}
	svc, err := NewWithProjectRoot(repo, root)
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	var gotEnv map[string]string
	svc.runReviewCommand = func(_ context.Context, _ string, _ workflowRunDefinition, extraEnv map[string]string) verifyCommandRun {
		gotEnv = extraEnv
		exitCode := 0
		now := time.Now().UTC()
		return verifyCommandRun{
			ExitCode:   &exitCode,
			Stdout:     "PASS: Derived receipt env works.",
			StartedAt:  now,
			FinishedAt: now.Add(100 * time.Millisecond),
		}
	}

	result, apiErr := svc.Review(context.Background(), v1.ReviewPayload{
		ProjectID: "project.alpha",
		PlanKey:   "plan:receipt.abc123",
		Run:       true,
	})
	if apiErr != nil {
		t.Fatalf("unexpected API error: %+v", apiErr)
	}
	if result.ReviewStatus != v1.WorkItemStatusComplete {
		t.Fatalf("unexpected review result: %+v", result)
	}
	if gotEnv["ACM_RECEIPT_ID"] != "receipt.abc123" {
		t.Fatalf("unexpected derived ACM_RECEIPT_ID: %+v", gotEnv)
	}
	if gotEnv["ACM_PLAN_KEY"] != "plan:receipt.abc123" {
		t.Fatalf("unexpected derived ACM_PLAN_KEY: %+v", gotEnv)
	}
}

func TestReview_RunRequiresWorkflowRunCommand(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".acm"), 0o755); err != nil {
		t.Fatalf("mkdir .acm: %v", err)
	}
	workflowsYAML := "version: acm.workflows.v1\ncompletion:\n  required_tasks:\n    - key: review:cross-llm\n"
	if err := os.WriteFile(filepath.Join(root, ".acm", "acm-workflows.yaml"), []byte(workflowsYAML), 0o644); err != nil {
		t.Fatalf("write workflows file: %v", err)
	}

	repo := &fakeRepository{}
	svc, err := NewWithProjectRoot(repo, root)
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	_, apiErr := svc.Review(context.Background(), v1.ReviewPayload{
		ProjectID: "project.alpha",
		ReceiptID: "receipt.abc123",
		Run:       true,
	})
	if apiErr == nil {
		t.Fatal("expected API error")
	}
	if apiErr.Code != "INVALID_INPUT" {
		t.Fatalf("unexpected error code: %+v", apiErr)
	}
}

func TestReview_RunRequiresConfiguredWorkflowKey(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".acm"), 0o755); err != nil {
		t.Fatalf("mkdir .acm: %v", err)
	}
	workflowsYAML := "version: acm.workflows.v1\ncompletion:\n  required_tasks:\n    - key: review:human\n      run:\n        argv: [\"scripts/acm-human-review.sh\"]\n"
	if err := os.WriteFile(filepath.Join(root, ".acm", "acm-workflows.yaml"), []byte(workflowsYAML), 0o644); err != nil {
		t.Fatalf("write workflows file: %v", err)
	}

	repo := &fakeRepository{}
	svc, err := NewWithProjectRoot(repo, root)
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	_, apiErr := svc.Review(context.Background(), v1.ReviewPayload{
		ProjectID: "project.alpha",
		ReceiptID: "receipt.abc123",
		Key:       v1.DefaultReviewTaskKey,
		Run:       true,
	})
	if apiErr == nil {
		t.Fatal("expected API error")
	}
	if apiErr.Code != "INVALID_INPUT" {
		t.Fatalf("unexpected error code: %+v", apiErr)
	}
	if !strings.Contains(apiErr.Message, "review key is not configured") {
		t.Fatalf("unexpected error message: %+v", apiErr)
	}
	if len(repo.workPlanUpsertCalls) != 0 {
		t.Fatalf("expected no work plan upsert, got %+v", repo.workPlanUpsertCalls)
	}
}

func TestReview_RunRejectsInvalidWorkflowDefinitions(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".acm"), 0o755); err != nil {
		t.Fatalf("mkdir .acm: %v", err)
	}
	workflowsYAML := "version: acm.workflows.v1\ncompletion:\n  required_tasks:\n    - key: review:cross-llm\n      run:\n        argv: [\"scripts/acm-cross-review.sh\"]\n        bad_field: true\n"
	if err := os.WriteFile(filepath.Join(root, ".acm", "acm-workflows.yaml"), []byte(workflowsYAML), 0o644); err != nil {
		t.Fatalf("write workflows file: %v", err)
	}

	repo := &fakeRepository{}
	svc, err := NewWithProjectRoot(repo, root)
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	_, apiErr := svc.Review(context.Background(), v1.ReviewPayload{
		ProjectID: "project.alpha",
		ReceiptID: "receipt.abc123",
		Run:       true,
	})
	if apiErr == nil {
		t.Fatal("expected API error")
	}
	if apiErr.Code != "INVALID_INPUT" {
		t.Fatalf("unexpected error code: %+v", apiErr)
	}
	if !strings.Contains(apiErr.Message, "workflow definitions are invalid") {
		t.Fatalf("unexpected error message: %+v", apiErr)
	}
	if len(repo.workPlanUpsertCalls) != 0 {
		t.Fatalf("expected no work plan upsert, got %+v", repo.workPlanUpsertCalls)
	}
}

func TestReview_RunRejectsZeroMaxAttempts(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".acm"), 0o755); err != nil {
		t.Fatalf("mkdir .acm: %v", err)
	}
	workflowsYAML := "version: acm.workflows.v1\ncompletion:\n  required_tasks:\n    - key: review:cross-llm\n      max_attempts: 0\n      run:\n        argv: [\"scripts/acm-cross-review.sh\"]\n"
	if err := os.WriteFile(filepath.Join(root, ".acm", "acm-workflows.yaml"), []byte(workflowsYAML), 0o644); err != nil {
		t.Fatalf("write workflows file: %v", err)
	}

	repo := &fakeRepository{}
	svc, err := NewWithProjectRoot(repo, root)
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	_, apiErr := svc.Review(context.Background(), v1.ReviewPayload{
		ProjectID: "project.alpha",
		ReceiptID: "receipt.abc123",
		Run:       true,
	})
	if apiErr == nil {
		t.Fatal("expected API error")
	}
	if apiErr.Code != "INVALID_INPUT" {
		t.Fatalf("unexpected error code: %+v", apiErr)
	}
	details, _ := apiErr.Details.(map[string]any)
	detail, _ := details["error"].(string)
	if !strings.Contains(detail, "max_attempts must be 1..16 when provided") {
		t.Fatalf("unexpected error details: %+v", apiErr)
	}
}

func TestReview_RunSkipsDuplicateFingerprintWithoutExecutingRunner(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".acm"), 0o755); err != nil {
		t.Fatalf("mkdir .acm: %v", err)
	}
	workflowsYAML := "version: acm.workflows.v1\ncompletion:\n  required_tasks:\n    - key: review:cross-llm\n      summary: Cross-LLM review\n      run:\n        argv: [\"scripts/acm-cross-review.sh\"]\n"
	if err := os.WriteFile(filepath.Join(root, ".acm", "acm-workflows.yaml"), []byte(workflowsYAML), 0o644); err != nil {
		t.Fatalf("write workflows file: %v", err)
	}

	scope := core.ReceiptScope{ProjectID: "project.alpha", ReceiptID: "receipt.abc123"}
	fingerprint, apiErr := computeReviewFingerprint(root, "project.alpha", "receipt.abc123", v1.DefaultReviewTaskKey, ".acm/acm-workflows.yaml", workflowRunDefinition{
		Argv:       []string{"scripts/acm-cross-review.sh"},
		CWD:        ".",
		TimeoutSec: 300,
	}, scope)
	if apiErr != nil {
		t.Fatalf("compute fingerprint: %+v", apiErr)
	}

	repo := &fakeRepository{
		scopeResults: []core.ReceiptScope{scope},
		reviewAttemptResults: [][]core.ReviewAttempt{{
			{
				AttemptID:   1,
				ProjectID:   "project.alpha",
				ReceiptID:   "receipt.abc123",
				ReviewKey:   v1.DefaultReviewTaskKey,
				Fingerprint: fingerprint,
				Status:      "passed",
				Passed:      true,
				Outcome:     "Review gate passed (1/2 attempts, 1 passing run(s)): PASS",
			},
		}},
	}
	svc, err := NewWithProjectRoot(repo, root)
	if err != nil {
		t.Fatalf("new service: %v", err)
	}
	svc.runReviewCommand = func(context.Context, string, workflowRunDefinition, map[string]string) verifyCommandRun {
		t.Fatal("runner should not execute for duplicate fingerprint")
		return verifyCommandRun{}
	}

	result, apiErr := svc.Review(context.Background(), v1.ReviewPayload{
		ProjectID: "project.alpha",
		ReceiptID: "receipt.abc123",
		Run:       true,
	})
	if apiErr != nil {
		t.Fatalf("unexpected API error: %+v", apiErr)
	}
	if result.Executed || result.ReviewStatus != v1.WorkItemStatusComplete {
		t.Fatalf("unexpected duplicate review result: %+v", result)
	}
	if result.SkippedReason == "" || result.AttemptsRun != 1 || result.PassingRuns != 1 {
		t.Fatalf("unexpected duplicate review metadata: %+v", result)
	}
	if len(repo.reviewAttemptCalls) != 0 {
		t.Fatalf("expected no new review attempt save, got %+v", repo.reviewAttemptCalls)
	}
	if len(repo.workPlanUpsertCalls) != 1 || len(repo.workPlanUpsertCalls[0].Tasks) != 1 {
		t.Fatalf("expected one work plan upsert with one task, got %+v", repo.workPlanUpsertCalls)
	}
	if got := repo.workPlanUpsertCalls[0].Tasks[0].Summary; got != "Cross-LLM review" {
		t.Fatalf("unexpected duplicate review summary: %q", got)
	}
}

func TestComputeReviewFingerprintIncludesCompletionManagedPaths(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".acm"), 0o755); err != nil {
		t.Fatalf("mkdir .acm: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, ".acm", "acm-tests.yaml"), []byte("version: acm.tests.v1\n"), 0o644); err != nil {
		t.Fatalf("write tests file: %v", err)
	}

	command := workflowRunDefinition{
		Argv:       []string{"scripts/acm-cross-review.sh"},
		TimeoutSec: 300,
	}
	first, apiErr := computeReviewFingerprint(root, "project.alpha", "receipt.abc123", v1.DefaultReviewTaskKey, ".acm/acm-workflows.yaml", command, core.ReceiptScope{})
	if apiErr != nil {
		t.Fatalf("compute first fingerprint: %+v", apiErr)
	}
	if err := os.WriteFile(filepath.Join(root, ".acm", "acm-tests.yaml"), []byte("version: acm.tests.v1\nsmoke: []\n"), 0o644); err != nil {
		t.Fatalf("rewrite tests file: %v", err)
	}
	second, apiErr := computeReviewFingerprint(root, "project.alpha", "receipt.abc123", v1.DefaultReviewTaskKey, ".acm/acm-workflows.yaml", command, core.ReceiptScope{})
	if apiErr != nil {
		t.Fatalf("compute second fingerprint: %+v", apiErr)
	}
	if first == second {
		t.Fatalf("expected managed completion files to affect fingerprint, got %q", first)
	}
}

func TestReview_RunBlocksWhenMaxAttemptsAreExhausted(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".acm"), 0o755); err != nil {
		t.Fatalf("mkdir .acm: %v", err)
	}
	workflowsYAML := "version: acm.workflows.v1\ncompletion:\n  required_tasks:\n    - key: review:cross-llm\n      summary: Cross-LLM review\n      max_attempts: 2\n      run:\n        argv: [\"scripts/acm-cross-review.sh\"]\n"
	if err := os.WriteFile(filepath.Join(root, ".acm", "acm-workflows.yaml"), []byte(workflowsYAML), 0o644); err != nil {
		t.Fatalf("write workflows file: %v", err)
	}

	repo := &fakeRepository{
		scopeResults: []core.ReceiptScope{{
			ProjectID: "project.alpha",
			ReceiptID: "receipt.abc123",
		}},
		reviewAttemptResults: [][]core.ReviewAttempt{{
			{AttemptID: 1, ProjectID: "project.alpha", ReceiptID: "receipt.abc123", ReviewKey: v1.DefaultReviewTaskKey, Fingerprint: "sha256:first", Status: "failed", Outcome: "first failure"},
			{AttemptID: 2, ProjectID: "project.alpha", ReceiptID: "receipt.abc123", ReviewKey: v1.DefaultReviewTaskKey, Fingerprint: "sha256:second", Status: "failed", Outcome: "second failure"},
		}},
	}
	svc, err := NewWithProjectRoot(repo, root)
	if err != nil {
		t.Fatalf("new service: %v", err)
	}
	svc.runReviewCommand = func(context.Context, string, workflowRunDefinition, map[string]string) verifyCommandRun {
		t.Fatal("runner should not execute once max_attempts is exhausted")
		return verifyCommandRun{}
	}

	result, apiErr := svc.Review(context.Background(), v1.ReviewPayload{
		ProjectID: "project.alpha",
		ReceiptID: "receipt.abc123",
		Run:       true,
	})
	if apiErr != nil {
		t.Fatalf("unexpected API error: %+v", apiErr)
	}
	if result.Executed || result.ReviewStatus != v1.WorkItemStatusBlocked {
		t.Fatalf("unexpected exhausted review result: %+v", result)
	}
	if result.SkippedReason == "" || result.AttemptsRun != 2 || result.MaxAttempts != 2 {
		t.Fatalf("unexpected exhausted review metadata: %+v", result)
	}
	if len(repo.reviewAttemptCalls) != 0 {
		t.Fatalf("expected no new review attempt save, got %+v", repo.reviewAttemptCalls)
	}
	if len(repo.workPlanUpsertCalls) != 1 || len(repo.workPlanUpsertCalls[0].Tasks) != 1 {
		t.Fatalf("expected one work plan upsert with one task, got %+v", repo.workPlanUpsertCalls)
	}
	if got := repo.workPlanUpsertCalls[0].Tasks[0].Summary; got != "Cross-LLM review" {
		t.Fatalf("unexpected exhausted review summary: %q", got)
	}
}

func TestReportCompletionFlagsStaleReviewForCurrentFingerprint(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".acm"), 0o755); err != nil {
		t.Fatalf("mkdir .acm: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(root, "internal"), 0o755); err != nil {
		t.Fatalf("mkdir internal: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "internal", "review.txt"), []byte("new content"), 0o644); err != nil {
		t.Fatalf("write scoped file: %v", err)
	}
	workflowsYAML := "version: acm.workflows.v1\ncompletion:\n  required_tasks:\n    - key: review:cross-llm\n      select:\n        phases: [\"review\"]\n        changed_paths_any: [\"internal/**\"]\n      run:\n        argv: [\"scripts/acm-cross-review.sh\"]\n"
	if err := os.WriteFile(filepath.Join(root, ".acm", "acm-workflows.yaml"), []byte(workflowsYAML), 0o644); err != nil {
		t.Fatalf("write workflows file: %v", err)
	}

	repo := &fakeRepository{
		scopeResults: []core.ReceiptScope{{
			ProjectID:    "project.alpha",
			ReceiptID:    "receipt.abc123",
			Phase:        "review",
			PointerPaths: []string{"internal/review.txt"},
		}},
		workListResults: [][]core.WorkItem{{
			{
				ItemKey: v1.DefaultReviewTaskKey,
				Status:  string(v1.WorkItemStatusComplete),
				Outcome: "Review gate passed",
			},
		}},
		reviewAttemptResults: [][]core.ReviewAttempt{{
			{
				AttemptID:   1,
				ProjectID:   "project.alpha",
				ReceiptID:   "receipt.abc123",
				ReviewKey:   v1.DefaultReviewTaskKey,
				Fingerprint: "sha256:stale",
				Status:      "passed",
				Passed:      true,
				Outcome:     "Review gate passed",
			},
		}},
	}
	svc, err := NewWithProjectRoot(repo, root)
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	result, apiErr := svc.ReportCompletion(context.Background(), v1.ReportCompletionPayload{
		ProjectID:    "project.alpha",
		ReceiptID:    "receipt.abc123",
		FilesChanged: []string{"internal/review.txt"},
		Outcome:      "done",
		ScopeMode:    v1.ScopeModeStrict,
	})
	if apiErr != nil {
		t.Fatalf("unexpected API error: %+v", apiErr)
	}
	if result.Accepted {
		t.Fatalf("expected stale review to block strict report completion: %+v", result)
	}
	if len(result.DefinitionOfDoneIssues) != 1 || !strings.Contains(result.DefinitionOfDoneIssues[0], "stale for the current scoped fingerprint") {
		t.Fatalf("unexpected definition_of_done_issues: %+v", result.DefinitionOfDoneIssues)
	}
}

func TestReportCompletionFlagsStaleReviewForManagedGovernanceFileCurrentFingerprint(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".acm"), 0o755); err != nil {
		t.Fatalf("mkdir .acm: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, ".acm", "acm-tests.yaml"), []byte("version: acm.tests.v1\n"), 0o644); err != nil {
		t.Fatalf("write tests file: %v", err)
	}
	workflowsYAML := "version: acm.workflows.v1\ncompletion:\n  required_tasks:\n    - key: review:cross-llm\n      select:\n        phases: [\"review\"]\n        changed_paths_any: [\".acm/**\"]\n      run:\n        argv: [\"scripts/acm-cross-review.sh\"]\n"
	if err := os.WriteFile(filepath.Join(root, ".acm", "acm-workflows.yaml"), []byte(workflowsYAML), 0o644); err != nil {
		t.Fatalf("write workflows file: %v", err)
	}

	scope := core.ReceiptScope{
		ProjectID: "project.alpha",
		ReceiptID: "receipt.abc123",
		Phase:     "review",
	}
	command := workflowRunDefinition{
		Argv:       []string{"scripts/acm-cross-review.sh"},
		TimeoutSec: 300,
	}
	staleFingerprint, apiErr := computeReviewFingerprint(root, "project.alpha", "receipt.abc123", v1.DefaultReviewTaskKey, ".acm/acm-workflows.yaml", command, scope)
	if apiErr != nil {
		t.Fatalf("compute stale fingerprint: %+v", apiErr)
	}
	if err := os.WriteFile(filepath.Join(root, ".acm", "acm-tests.yaml"), []byte("version: acm.tests.v1\nsmoke: []\n"), 0o644); err != nil {
		t.Fatalf("rewrite tests file: %v", err)
	}

	repo := &fakeRepository{
		scopeResults: []core.ReceiptScope{scope},
		workListResults: [][]core.WorkItem{{
			{
				ItemKey: v1.DefaultReviewTaskKey,
				Status:  string(v1.WorkItemStatusComplete),
				Outcome: "Review gate passed",
			},
		}},
		reviewAttemptResults: [][]core.ReviewAttempt{{
			{
				AttemptID:   1,
				ProjectID:   "project.alpha",
				ReceiptID:   "receipt.abc123",
				ReviewKey:   v1.DefaultReviewTaskKey,
				Fingerprint: staleFingerprint,
				Status:      "passed",
				Passed:      true,
				Outcome:     "Review gate passed",
			},
		}},
	}
	svc, err := NewWithProjectRoot(repo, root)
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	result, apiErr := svc.ReportCompletion(context.Background(), v1.ReportCompletionPayload{
		ProjectID:    "project.alpha",
		ReceiptID:    "receipt.abc123",
		FilesChanged: []string{".acm/acm-tests.yaml"},
		Outcome:      "done",
		ScopeMode:    v1.ScopeModeStrict,
	})
	if apiErr != nil {
		t.Fatalf("unexpected API error: %+v", apiErr)
	}
	if result.Accepted {
		t.Fatalf("expected stale managed governance review to block strict report completion: %+v", result)
	}
	if len(result.DefinitionOfDoneIssues) != 1 || !strings.Contains(result.DefinitionOfDoneIssues[0], "stale for the current scoped fingerprint") {
		t.Fatalf("unexpected definition_of_done_issues: %+v", result.DefinitionOfDoneIssues)
	}
}

func withWorkingDir(t *testing.T, dir string) {
	t.Helper()

	previous, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir %s: %v", dir, err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(previous); err != nil {
			t.Fatalf("restore cwd %s: %v", previous, err)
		}
	})
}
