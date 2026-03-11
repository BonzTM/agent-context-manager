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
	memoryResults          [][]core.ActiveMemory
	memoryErrors           []error
	inventoryResults       []core.PointerInventory
	inventoryErrors        []error
	scopeResults           []core.ReceiptScope
	scopeErrors            []error
	proposeResults         []core.MemoryPersistenceResult
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
	memoryCalls            []core.ActiveMemoryQuery
	inventoryCalls         []string
	scopeCalls             []core.ReceiptScopeQuery
	receiptUpsertCalls     []core.ReceiptScope
	proposeCalls           []core.MemoryPersistence
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
	storedWorkPlans    map[string]core.WorkPlan
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
	scope.InitialScopePaths = append([]string(nil), scope.InitialScopePaths...)
	scope.BaselinePaths = append([]core.SyncPath(nil), scope.BaselinePaths...)
	return scope, nil
}

func (f *fakeRepository) UpsertReceiptScope(_ context.Context, input core.ReceiptScope) error {
	f.receiptUpsertCalls = append(f.receiptUpsertCalls, core.ReceiptScope{
		ProjectID:         strings.TrimSpace(input.ProjectID),
		ReceiptID:         strings.TrimSpace(input.ReceiptID),
		TaskText:          strings.TrimSpace(input.TaskText),
		Phase:             strings.TrimSpace(input.Phase),
		ResolvedTags:      append([]string(nil), input.ResolvedTags...),
		PointerKeys:       append([]string(nil), input.PointerKeys...),
		MemoryIDs:         append([]int64(nil), input.MemoryIDs...),
		InitialScopePaths: append([]string(nil), input.InitialScopePaths...),
		BaselineCaptured:  input.BaselineCaptured,
		BaselinePaths:     append([]core.SyncPath(nil), input.BaselinePaths...),
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
		f.storeWorkPlan(f.workPlanUpsertResult[idx].Plan)
		return f.workPlanUpsertResult[idx], nil
	}

	plan := f.buildStoredWorkPlan(input)
	if strings.TrimSpace(plan.Status) == "" {
		plan.Status = core.PlanStatusPending
	}
	f.storeWorkPlan(plan)
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
		if plan, ok := f.lookupStoredWorkPlan(strings.TrimSpace(input.ProjectID), strings.TrimSpace(input.PlanKey)); ok {
			return plan, nil
		}
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
		return f.listStoredWorkPlans(strings.TrimSpace(input.ProjectID), strings.TrimSpace(input.Scope)), nil
	}
	return append([]core.WorkPlanSummary(nil), f.workPlanListResults[idx]...), nil
}

func (f *fakeRepository) buildStoredWorkPlan(input core.WorkPlanUpsertInput) core.WorkPlan {
	plan, found := f.lookupStoredWorkPlan(strings.TrimSpace(input.ProjectID), strings.TrimSpace(input.PlanKey))
	if !found || input.Mode == core.WorkPlanModeReplace {
		plan = core.WorkPlan{
			ProjectID: strings.TrimSpace(input.ProjectID),
			PlanKey:   strings.TrimSpace(input.PlanKey),
		}
	}

	plan.ProjectID = strings.TrimSpace(input.ProjectID)
	plan.PlanKey = strings.TrimSpace(input.PlanKey)
	if receiptID := strings.TrimSpace(input.ReceiptID); receiptID != "" || !found || input.Mode == core.WorkPlanModeReplace {
		plan.ReceiptID = receiptID
	}
	if title := strings.TrimSpace(input.Title); title != "" || input.Mode == core.WorkPlanModeReplace {
		plan.Title = title
	}
	if objective := strings.TrimSpace(input.Objective); objective != "" || input.Mode == core.WorkPlanModeReplace {
		plan.Objective = objective
	}
	if kind := strings.TrimSpace(input.Kind); kind != "" || input.Mode == core.WorkPlanModeReplace {
		plan.Kind = kind
	}
	if parentPlanKey := strings.TrimSpace(input.ParentPlanKey); parentPlanKey != "" || input.Mode == core.WorkPlanModeReplace {
		plan.ParentPlanKey = parentPlanKey
	}
	if status := strings.TrimSpace(input.Status); status != "" || input.Mode == core.WorkPlanModeReplace {
		plan.Status = status
	}
	if input.Mode == core.WorkPlanModeReplace || strings.TrimSpace(input.Stages.SpecOutline) != "" || strings.TrimSpace(input.Stages.RefinedSpec) != "" || strings.TrimSpace(input.Stages.ImplementationPlan) != "" {
		plan.Stages = mergeFakeWorkPlanStages(plan.Stages, input.Stages, input.Mode)
	}
	if input.Mode == core.WorkPlanModeReplace || len(input.InScope) > 0 {
		plan.InScope = append([]string(nil), input.InScope...)
	}
	if input.Mode == core.WorkPlanModeReplace || len(input.OutOfScope) > 0 {
		plan.OutOfScope = append([]string(nil), input.OutOfScope...)
	}
	if input.Mode == core.WorkPlanModeReplace || len(input.DiscoveredPaths) > 0 {
		plan.DiscoveredPaths = append([]string(nil), input.DiscoveredPaths...)
	}
	if input.Mode == core.WorkPlanModeReplace || len(input.Constraints) > 0 {
		plan.Constraints = append([]string(nil), input.Constraints...)
	}
	if input.Mode == core.WorkPlanModeReplace || len(input.References) > 0 {
		plan.References = append([]string(nil), input.References...)
	}
	if input.Mode == core.WorkPlanModeReplace || len(input.ExternalRefs) > 0 {
		plan.ExternalRefs = append([]string(nil), input.ExternalRefs...)
	}
	if input.Mode == core.WorkPlanModeReplace {
		plan.Tasks = append([]core.WorkItem(nil), input.Tasks...)
	} else if len(input.Tasks) > 0 {
		plan.Tasks = mergeFakeWorkPlanTasks(plan.Tasks, input.Tasks)
	}
	return plan
}

func (f *fakeRepository) storeWorkPlan(plan core.WorkPlan) {
	projectID := strings.TrimSpace(plan.ProjectID)
	planKey := strings.TrimSpace(plan.PlanKey)
	if projectID == "" || planKey == "" {
		return
	}
	if f.storedWorkPlans == nil {
		f.storedWorkPlans = make(map[string]core.WorkPlan)
	}
	f.storedWorkPlans[fakeWorkPlanStorageKey(projectID, planKey)] = plan
}

func (f *fakeRepository) lookupStoredWorkPlan(projectID, planKey string) (core.WorkPlan, bool) {
	if f == nil || f.storedWorkPlans == nil || projectID == "" || planKey == "" {
		return core.WorkPlan{}, false
	}
	plan, ok := f.storedWorkPlans[fakeWorkPlanStorageKey(projectID, planKey)]
	return plan, ok
}

func (f *fakeRepository) listStoredWorkPlans(projectID, scope string) []core.WorkPlanSummary {
	if f == nil || len(f.storedWorkPlans) == 0 || projectID == "" {
		return nil
	}

	out := make([]core.WorkPlanSummary, 0, len(f.storedWorkPlans))
	for _, plan := range f.storedWorkPlans {
		if strings.TrimSpace(plan.ProjectID) != projectID {
			continue
		}
		status := normalizePlanStatus(plan.Status)
		switch strings.TrimSpace(scope) {
		case string(v1.HistoryScopeCompleted):
			if !isTerminalPlanStatus(status) {
				continue
			}
		case string(v1.HistoryScopeCurrent), "":
			if isTerminalPlanStatus(status) {
				continue
			}
		}

		tasks := normalizeWorkItems(plan.Tasks)
		activeTaskKeys := make([]string, 0, len(tasks))
		summary := core.WorkPlanSummary{
			ReceiptID:     strings.TrimSpace(plan.ReceiptID),
			Title:         strings.TrimSpace(plan.Title),
			Objective:     strings.TrimSpace(plan.Objective),
			PlanKey:       strings.TrimSpace(plan.PlanKey),
			Summary:       strings.TrimSpace(plan.Title),
			Status:        status,
			Kind:          strings.TrimSpace(plan.Kind),
			ParentPlanKey: strings.TrimSpace(plan.ParentPlanKey),
		}
		for _, task := range tasks {
			switch normalizeWorkItemStatus(task.Status) {
			case core.WorkItemStatusPending:
				summary.TaskCountPending++
				activeTaskKeys = append(activeTaskKeys, task.ItemKey)
			case core.WorkItemStatusInProgress:
				summary.TaskCountInProgress++
				activeTaskKeys = append(activeTaskKeys, task.ItemKey)
			case core.WorkItemStatusBlocked:
				summary.TaskCountBlocked++
				activeTaskKeys = append(activeTaskKeys, task.ItemKey)
			case core.WorkItemStatusComplete:
				summary.TaskCountComplete++
			}
		}
		summary.TaskCountTotal = len(tasks)
		summary.ActiveTaskKeys = activeTaskKeys
		out = append(out, summary)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].PlanKey < out[j].PlanKey
	})
	return out
}

func fakeWorkPlanStorageKey(projectID, planKey string) string {
	return projectID + "\x00" + planKey
}

func mergeFakeWorkPlanStages(current, incoming core.WorkPlanStages, mode core.WorkPlanMode) core.WorkPlanStages {
	if mode == core.WorkPlanModeReplace {
		return incoming
	}
	next := current
	if value := strings.TrimSpace(incoming.SpecOutline); value != "" {
		next.SpecOutline = value
	}
	if value := strings.TrimSpace(incoming.RefinedSpec); value != "" {
		next.RefinedSpec = value
	}
	if value := strings.TrimSpace(incoming.ImplementationPlan); value != "" {
		next.ImplementationPlan = value
	}
	return next
}

func mergeFakeWorkPlanTasks(current, incoming []core.WorkItem) []core.WorkItem {
	byKey := make(map[string]core.WorkItem, len(current)+len(incoming))
	for _, item := range normalizeWorkItems(current) {
		byKey[item.ItemKey] = item
	}
	for _, item := range normalizeWorkItems(incoming) {
		byKey[item.ItemKey] = item
	}

	keys := make([]string, 0, len(byKey))
	for key := range byKey {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	merged := make([]core.WorkItem, 0, len(keys))
	for _, key := range keys {
		merged = append(merged, byKey[key])
	}
	return merged
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

func (f *fakeRepository) PersistMemory(_ context.Context, input core.MemoryPersistence) (core.MemoryPersistenceResult, error) {
	f.proposeCalls = append(f.proposeCalls, input)
	idx := len(f.proposeCalls) - 1
	if idx < len(f.proposeErrors) && f.proposeErrors[idx] != nil {
		return core.MemoryPersistenceResult{}, f.proposeErrors[idx]
	}
	if idx < len(f.proposeResults) {
		result := f.proposeResults[idx]
		if result.CandidateID == 0 {
			result.CandidateID = int64(idx + 1)
		}
		return result, nil
	}

	result := core.MemoryPersistenceResult{
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

func TestContext_NormalPathReturnsOKAndReceipt(t *testing.T) {
	root := t.TempDir()
	writeRepoFile(t, root, ".acm/acm-rules.yaml", strings.Join([]string{
		"version: acm.rules.v1",
		"rules:",
		"  - id: rule_startup",
		"    summary: Startup rule",
		"    content: Keep the context receipt deterministic.",
		"    enforcement: hard",
		"    tags: [governance]",
		"",
	}, "\n"))
	withWorkingDir(t, root)

	repo := &fakeRepository{
		memoryResults: [][]core.ActiveMemory{{
			memory(101, "Default caps behavior", "schema defaults apply when caps omitted", []string{"backend"}, []string{"code:service"}),
		}},
	}
	svc, err := New(repo)
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	payload := v1.ContextPayload{
		ProjectID:         "project.alpha",
		TaskText:          "implement deterministic get context flow",
		Phase:             v1.PhaseExecute,
		InitialScopePaths: []string{"internal/service/backend/context.go", "spec/v1/README.md"},
	}

	result, apiErr := svc.Context(context.Background(), payload)
	if apiErr != nil {
		t.Fatalf("unexpected API error: %+v", apiErr)
	}
	if result.Status != "ok" {
		t.Fatalf("unexpected status: %q", result.Status)
	}
	if result.Receipt == nil {
		t.Fatal("expected receipt")
	}
	if result.Receipt.Meta.ReceiptID == "" {
		t.Fatal("expected non-empty receipt_id")
	}
	if got := result.Receipt.InitialScopePaths; !reflect.DeepEqual(got, payload.InitialScopePaths) {
		t.Fatalf("unexpected initial scope paths: got %v want %v", got, payload.InitialScopePaths)
	}

	rules := receiptIndexEntries(result.Receipt, "rules")
	if got, want := receiptIndexKeys(rules), []string{"project.alpha:.acm/acm-rules.yaml#rule_startup"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected rule index keys: got %v want %v", got, want)
	}
	if len(rules) != 1 {
		t.Fatalf("expected one rule entry, got %d", len(rules))
	}
	if got, want := entryString(rules[0], "rule_id"), "rule_startup"; got != want {
		t.Fatalf("unexpected stable rule_id: got %q want %q", got, want)
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
	if !result.Receipt.Meta.BaselineCaptured {
		t.Fatal("expected baseline_captured to be true")
	}

	if len(repo.candidateCalls) != 0 {
		t.Fatalf("did not expect candidate queries, got %d", len(repo.candidateCalls))
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
	if got := repo.receiptUpsertCalls[0].PointerKeys; !reflect.DeepEqual(got, []string{"project.alpha:.acm/acm-rules.yaml#rule_startup"}) {
		t.Fatalf("unexpected persisted pointer keys: got %v want %v", got, []string{"project.alpha:.acm/acm-rules.yaml#rule_startup"})
	}
	wantPersistedPaths := []string{"internal/service/backend/context.go", "spec/v1/README.md"}
	if got := repo.receiptUpsertCalls[0].InitialScopePaths; !reflect.DeepEqual(got, wantPersistedPaths) {
		t.Fatalf("unexpected persisted initial scope paths: got %v want %v", got, wantPersistedPaths)
	}

	repo2 := &fakeRepository{
		memoryResults: repo.memoryResults,
	}
	svc2, err := New(repo2)
	if err != nil {
		t.Fatalf("new service 2: %v", err)
	}
	result2, apiErr2 := svc2.Context(context.Background(), payload)
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

func TestContext_FallsBackToFilesystemBaselineWhenGitUnavailable(t *testing.T) {
	root := t.TempDir()
	writeRepoFile(t, root, ".acm/acm-rules.yaml", "version: acm.rules.v1\nrules:\n  - id: rule_startup\n    summary: Startup rule\n    content: Keep the context receipt deterministic.\n    enforcement: hard\n")
	writeRepoFile(t, root, "src/main.go", "package src\n\nfunc main() {}\n")
	withWorkingDir(t, root)

	repo := &fakeRepository{}
	svc, err := New(repo)
	if err != nil {
		t.Fatalf("new service: %v", err)
	}
	svc.runGitCommand = func(_ context.Context, _ string, _ ...string) (string, error) {
		return "", errors.New("git unavailable")
	}

	result, apiErr := svc.Context(context.Background(), v1.ContextPayload{
		ProjectID: "project.alpha",
		TaskText:  "fallback baseline",
		Phase:     v1.PhaseExecute,
	})
	if apiErr != nil {
		t.Fatalf("unexpected API error: %+v", apiErr)
	}
	if result.Receipt == nil {
		t.Fatal("expected receipt")
	}
	if !result.Receipt.Meta.BaselineCaptured {
		t.Fatal("expected filesystem fallback baseline to be captured")
	}
	if len(repo.receiptUpsertCalls) != 1 {
		t.Fatalf("expected one receipt scope upsert, got %d", len(repo.receiptUpsertCalls))
	}
	if !repo.receiptUpsertCalls[0].BaselineCaptured {
		t.Fatal("expected persisted receipt scope baseline to be captured")
	}
	gotPaths := syncPathPaths(repo.receiptUpsertCalls[0].BaselinePaths)
	wantPaths := []string{".acm/acm-rules.yaml", "src/main.go"}
	if !reflect.DeepEqual(gotPaths, wantPaths) {
		t.Fatalf("unexpected fallback baseline paths: got %v want %v", gotPaths, wantPaths)
	}
}

func TestContext_AllowsCanonicalRulesFromManagedPaths(t *testing.T) {
	root := t.TempDir()
	writeRepoFile(t, root, ".acm/acm-rules.yaml", strings.Join([]string{
		"version: acm.rules.v1",
		"rules:",
		"  - id: rule_hard",
		"    summary: Managed hard rule",
		"    content: Keep managed rules visible.",
		"    enforcement: hard",
		"    tags: [governance, enforcement-hard]",
		"",
	}, "\n"))
	writeRepoFile(t, root, "acm-rules.yaml", strings.Join([]string{
		"version: acm.rules.v1",
		"rules:",
		"  - id: rule_root",
		"    summary: Root soft rule",
		"    content: Keep fallback rules visible.",
		"    enforcement: soft",
		"    tags: [governance, enforcement-soft]",
		"",
	}, "\n"))
	withWorkingDir(t, root)

	repo := &fakeRepository{memoryResults: [][]core.ActiveMemory{{}}}
	svc, err := New(repo)
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	result, apiErr := svc.Context(context.Background(), v1.ContextPayload{
		ProjectID: "project.alpha",
		TaskText:  "enforce repo hard rules",
		Phase:     v1.PhaseExecute,
	})
	if apiErr != nil {
		t.Fatalf("unexpected API error: %+v", apiErr)
	}
	if result.Status != "ok" || result.Receipt == nil {
		t.Fatalf("unexpected result: %+v", result)
	}

	if got, want := receiptIndexKeys(receiptIndexEntries(result.Receipt, "rules")), []string{"project.alpha:.acm/acm-rules.yaml#rule_hard", "project.alpha:acm-rules.yaml#rule_root"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected rule keys: got %v want %v", got, want)
	}
}

func TestContext_WithoutRulesStillReturnsReceipt(t *testing.T) {
	repo := &fakeRepository{
		candidateResults: [][]core.CandidatePointer{{}},
		memoryResults:    [][]core.ActiveMemory{{}},
	}
	svc, err := New(repo)
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	result, apiErr := svc.Context(context.Background(), v1.ContextPayload{
		ProjectID: "project.alpha",
		TaskText:  "minimal context should still succeed",
		Phase:     v1.PhaseExecute,
	})
	if apiErr != nil {
		t.Fatalf("unexpected API error: %+v", apiErr)
	}
	if result.Status != "ok" || result.Receipt == nil {
		t.Fatalf("unexpected result: %+v", result)
	}
	if len(result.Receipt.Rules) != 0 {
		t.Fatalf("expected no rules, got %+v", result.Receipt.Rules)
	}
}

func TestContext_LoadsCanonicalRulesWithoutIndexedPointers(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".acm"), 0o755); err != nil {
		t.Fatalf("mkdir .acm: %v", err)
	}
	rules := strings.Join([]string{
		"version: acm.rules.v1",
		"rules:",
		"  - id: rule_simple_context",
		"    summary: Always verify before done",
		"    content: Verify executable changes before closing the receipt.",
		"    enforcement: hard",
		"    tags: [verify, workflow]",
		"",
	}, "\n")
	if err := os.WriteFile(filepath.Join(root, ".acm", "acm-rules.yaml"), []byte(rules), 0o644); err != nil {
		t.Fatalf("write rules file: %v", err)
	}
	withWorkingDir(t, root)

	repo := &fakeRepository{
		candidateResults: [][]core.CandidatePointer{{}},
		memoryResults:    [][]core.ActiveMemory{{}},
	}
	svc, err := New(repo)
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	result, apiErr := svc.Context(context.Background(), v1.ContextPayload{
		ProjectID: "project.alpha",
		TaskText:  "verify direct ruleset loading",
		Phase:     v1.PhaseExecute,
	})
	if apiErr != nil {
		t.Fatalf("unexpected API error: %+v", apiErr)
	}
	if result.Status != "ok" || result.Receipt == nil {
		t.Fatalf("unexpected result: %+v", result)
	}
	if got, want := receiptIndexKeys(receiptIndexEntries(result.Receipt, "rules")), []string{"project.alpha:.acm/acm-rules.yaml#rule_simple_context"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected rule keys: got %v want %v", got, want)
	}
	if len(repo.candidateCalls) != 0 {
		t.Fatalf("did not expect indexed candidate queries when canonical rules exist, got %d", len(repo.candidateCalls))
	}
	if len(repo.receiptUpsertCalls) != 1 {
		t.Fatalf("expected one receipt scope upsert, got %d", len(repo.receiptUpsertCalls))
	}
	if got, want := repo.receiptUpsertCalls[0].PointerKeys, []string{"project.alpha:.acm/acm-rules.yaml#rule_simple_context"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected persisted pointer keys: got %v want %v", got, want)
	}
}

func TestContext_DefaultMemoryLimitStillApplies(t *testing.T) {
	repo := &fakeRepository{
		memoryResults: [][]core.ActiveMemory{{
			memory(7, "One", "first", []string{"backend"}, []string{"code:1"}),
			memory(8, "Two", "second", []string{"docs"}, []string{"doc:1"}),
			memory(9, "Three", "third", []string{"test"}, []string{"test:1"}),
			memory(10, "Four", "fourth", []string{"infra"}, []string{"ops:1"}),
			memory(11, "Five", "fifth", []string{"governance"}, []string{"rule:a"}),
			memory(12, "Six", "sixth", []string{"release"}, []string{"ops:2"}),
			memory(13, "Seven", "seventh", []string{"extra"}, []string{"ops:3"}),
		}},
	}
	svc, err := New(repo)
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	payload := v1.ContextPayload{
		ProjectID: "project.alpha",
		TaskText:  "validate default memory handling",
		Phase:     v1.PhaseExecute,
	}

	result, apiErr := svc.Context(context.Background(), payload)
	if apiErr != nil {
		t.Fatalf("unexpected API error: %+v", apiErr)
	}
	if result.Status != "ok" {
		t.Fatalf("unexpected status: %q", result.Status)
	}
	if result.Receipt == nil {
		t.Fatal("expected receipt")
	}

	if got := len(receiptIndexEntries(result.Receipt, "rules")); got != 0 {
		t.Fatalf("expected no rules without canonical rules files, got %d", got)
	}
	memoryEntries := receiptIndexEntries(result.Receipt, "memories")
	if len(memoryEntries) != defaultMaxMemories {
		t.Fatalf("unexpected memory index count: got %d want %d", len(memoryEntries), defaultMaxMemories)
	}
	planEntries := receiptIndexEntries(result.Receipt, "plans")
	if len(planEntries) != 1 {
		t.Fatalf("expected one plan entry, got %d", len(planEntries))
	}
	if got := entryString(planEntries[0], "status"); got != core.PlanStatusPending {
		t.Fatalf("unexpected plan status: got %q want %q", got, core.PlanStatusPending)
	}

	if len(repo.memoryCalls) != 1 {
		t.Fatalf("expected one memory query, got %d", len(repo.memoryCalls))
	}
	if repo.memoryCalls[0].Limit != defaultMaxMemories {
		t.Fatalf("expected memory query limit %d, got %d", defaultMaxMemories, repo.memoryCalls[0].Limit)
	}
}

func TestContext_PersistsOnlyPositiveMemoryIDs(t *testing.T) {
	root := t.TempDir()
	withWorkingDir(t, root)

	repo := &fakeRepository{
		memoryResults: [][]core.ActiveMemory{{
			memory(0, "Zero", "placeholder memory should not persist an id", []string{"backend"}, []string{"code:zero"}),
			memory(-9, "Negative", "invalid memory should not persist an id", []string{"backend"}, []string{"code:negative"}),
			memory(42, "Real", "valid memory id should be persisted", []string{"backend"}, []string{"code:real"}),
		}},
	}
	svc, err := New(repo)
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	result, apiErr := svc.Context(context.Background(), v1.ContextPayload{
		ProjectID: "project.alpha",
		TaskText:  "persist only valid memory ids",
		Phase:     v1.PhaseExecute,
	})
	if apiErr != nil {
		t.Fatalf("unexpected API error: %+v", apiErr)
	}
	if result.Status != "ok" || result.Receipt == nil {
		t.Fatalf("unexpected result: %+v", result)
	}
	if len(repo.receiptUpsertCalls) != 1 {
		t.Fatalf("expected one receipt scope upsert, got %d", len(repo.receiptUpsertCalls))
	}
	if got, want := repo.receiptUpsertCalls[0].MemoryIDs, []int64{42}; !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected persisted memory ids: got %v want %v", got, want)
	}
}

func TestContext_PhaseAndCanonicalTagsThreadedToRuleAndMemoryQueries(t *testing.T) {
	repo := &fakeRepository{
		memoryResults: [][]core.ActiveMemory{{}},
	}
	svc, err := New(repo)
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	result, apiErr := svc.Context(context.Background(), v1.ContextPayload{
		ProjectID: "project.alpha",
		TaskText:  "policy API tests for review",
		Phase:     v1.PhaseReview,
	})
	if apiErr != nil {
		t.Fatalf("unexpected API error: %+v", apiErr)
	}
	if result.Status != "ok" || result.Receipt == nil {
		t.Fatalf("unexpected result: %+v", result)
	}
	wantQueryTags := []string{"backend", "governance", "review", "test"}
	if len(repo.candidateCalls) != 0 {
		t.Fatalf("did not expect candidate calls, got %d", len(repo.candidateCalls))
	}
	if len(repo.memoryCalls) != 1 {
		t.Fatalf("expected one memory query, got %d", len(repo.memoryCalls))
	}
	wantMemoryTags := wantQueryTags
	if !reflect.DeepEqual(repo.memoryCalls[0].Tags, wantMemoryTags) {
		t.Fatalf("unexpected canonical memory tags: got %v want %v", repo.memoryCalls[0].Tags, wantMemoryTags)
	}
	if !containsAllStrings(result.Receipt.Meta.ResolvedTags, wantMemoryTags) {
		t.Fatalf("expected resolved tags to contain %v, got %v", wantMemoryTags, result.Receipt.Meta.ResolvedTags)
	}
}

func TestContext_DefaultRepoTagsFileDiscoveryMergesCanonicalAliases(t *testing.T) {
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
			ProjectID:         "project.alpha",
			ReceiptID:         "receipt.abc123",
			InitialScopePaths: []string{},
		}},
	}
	svc, err := New(repo)
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	result, apiErr := svc.Context(context.Background(), v1.ContextPayload{
		ProjectID: "project.alpha",
		TaskText:  "fix svc bootstrap gap",
		Phase:     v1.PhaseExecute,
	})
	if apiErr != nil {
		t.Fatalf("unexpected API error: %+v", apiErr)
	}
	if result.Status != "ok" {
		t.Fatalf("unexpected status: %+v", result)
	}
	if result.Receipt == nil {
		t.Fatal("expected receipt")
	}
	wantTags := []string{"backend", "bootstrap"}
	if len(repo.candidateCalls) != 0 {
		t.Fatalf("did not expect candidate queries, got %d", len(repo.candidateCalls))
	}
	if len(repo.memoryCalls) != 1 {
		t.Fatalf("expected one memory query, got %d", len(repo.memoryCalls))
	}
	if !reflect.DeepEqual(repo.memoryCalls[0].Tags, wantTags) {
		t.Fatalf("unexpected memory tags: got %v want %v", repo.memoryCalls[0].Tags, wantTags)
	}
}

func TestMemory_AutoPromoteOmittedDefaultsTrue(t *testing.T) {
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

	result, apiErr := svc.Memory(context.Background(), v1.MemoryCommandPayload{
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

func TestMemory_AutoPromoteFalseReturnsPending(t *testing.T) {
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

	result, apiErr := svc.Memory(context.Background(), v1.MemoryCommandPayload{
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

func TestMemory_AutoPromoteTrueHardAndSoftPassPromotes(t *testing.T) {
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

	result, apiErr := svc.Memory(context.Background(), v1.MemoryCommandPayload{
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

func TestMemory_HardFailEvidenceOutsideScopeRejected(t *testing.T) {
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

	result, apiErr := svc.Memory(context.Background(), v1.MemoryCommandPayload{
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

func TestMemory_EffectiveScopeAllowsDiscoveredPathEvidenceKeys(t *testing.T) {
	repo := &fakeRepository{
		scopeResults: []core.ReceiptScope{{
			ProjectID:         "project.alpha",
			ReceiptID:         "receipt.abc123",
			PointerKeys:       []string{"project.alpha:.acm/acm-rules.yaml#rule.scope"},
			InitialScopePaths: []string{"src/initial.go"},
		}},
		pointerLookupResults: []core.CandidatePointer{
			candidate("project.alpha:src/discovered.go#handler", "src/discovered.go", false, []string{"backend"}),
			candidate("project.alpha:src/discovered.go", "src/discovered.go", false, []string{"backend"}),
		},
		workPlanLookupResult: []core.WorkPlan{{
			ProjectID:       "project.alpha",
			PlanKey:         "plan:receipt.abc123",
			ReceiptID:       "receipt.abc123",
			DiscoveredPaths: []string{"src/discovered.go"},
		}},
	}
	svc, err := New(repo)
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	result, apiErr := svc.Memory(context.Background(), v1.MemoryCommandPayload{
		ProjectID:   "project.alpha",
		ReceiptID:   "receipt.abc123",
		AutoPromote: boolPtr(true),
		Memory: v1.MemoryPayload{
			Category:            v1.MemoryCategoryDecision,
			Subject:             "Discovered path evidence",
			Content:             "Evidence keys should be allowed when their paths are inside effective scope.",
			RelatedPointerKeys:  []string{"project.alpha:src/discovered.go"},
			Tags:                []string{"backend"},
			Confidence:          4,
			EvidencePointerKeys: []string{"project.alpha:src/discovered.go#handler"},
		},
	})
	if apiErr != nil {
		t.Fatalf("unexpected API error: %+v", apiErr)
	}
	if result.Status != "promoted" || !result.Validation.HardPassed || !result.Validation.SoftPassed {
		t.Fatalf("expected clean promoted result, got %+v", result)
	}
	if len(repo.workPlanLookupCalls) != 1 {
		t.Fatalf("expected one plan lookup call, got %d", len(repo.workPlanLookupCalls))
	}
	if len(repo.pointerLookupCalls) != 2 {
		t.Fatalf("expected two pointer lookups, got %d", len(repo.pointerLookupCalls))
	}
	if len(repo.proposeCalls) != 1 {
		t.Fatalf("expected one persistence call, got %d", len(repo.proposeCalls))
	}
	if got := repo.proposeCalls[0].EvidencePointerKeys; !reflect.DeepEqual(got, []string{"project.alpha:src/discovered.go#handler"}) {
		t.Fatalf("unexpected evidence pointer keys: %v", got)
	}
}

func TestMemory_PlanKeyOnlyResolvesReceiptScopeAndDiscoveredPaths(t *testing.T) {
	repo := &fakeRepository{
		scopeResults: []core.ReceiptScope{{
			ProjectID:         "project.alpha",
			ReceiptID:         "receipt.abc123",
			PointerKeys:       []string{"project.alpha:.acm/acm-rules.yaml#rule.scope"},
			InitialScopePaths: []string{"src/initial.go"},
		}},
		pointerLookupResults: []core.CandidatePointer{
			candidate("project.alpha:src/discovered.go#handler", "src/discovered.go", false, []string{"backend"}),
		},
		workPlanLookupResult: []core.WorkPlan{{
			ProjectID:       "project.alpha",
			PlanKey:         "plan:receipt.abc123",
			ReceiptID:       "receipt.abc123",
			DiscoveredPaths: []string{"src/discovered.go"},
		}},
	}
	svc, err := New(repo)
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	result, apiErr := svc.Memory(context.Background(), v1.MemoryCommandPayload{
		ProjectID: "project.alpha",
		PlanKey:   "plan:receipt.abc123",
		Memory: v1.MemoryPayload{
			Category:            v1.MemoryCategoryDecision,
			Subject:             "Plan-only discovered scope",
			Content:             "Memory should resolve the receipt from plan_key and honor discovered paths.",
			RelatedPointerKeys:  []string{"project.alpha:.acm/acm-rules.yaml#rule.scope"},
			Tags:                []string{"backend"},
			Confidence:          4,
			EvidencePointerKeys: []string{"project.alpha:src/discovered.go#handler"},
		},
	})
	if apiErr != nil {
		t.Fatalf("unexpected API error: %+v", apiErr)
	}
	if result.Status != "promoted" || !result.Validation.HardPassed {
		t.Fatalf("expected promoted result, got %+v", result)
	}
	if len(repo.scopeCalls) != 1 || repo.scopeCalls[0].ReceiptID != "receipt.abc123" {
		t.Fatalf("expected derived receipt scope lookup, got %+v", repo.scopeCalls)
	}
	if len(repo.workPlanLookupCalls) != 1 || repo.workPlanLookupCalls[0].PlanKey != "plan:receipt.abc123" {
		t.Fatalf("unexpected plan lookup calls: %+v", repo.workPlanLookupCalls)
	}
	if len(repo.proposeCalls) != 1 || repo.proposeCalls[0].ReceiptID != "receipt.abc123" {
		t.Fatalf("expected persisted memory to use derived receipt id, got %+v", repo.proposeCalls)
	}
}

func TestMemory_EffectiveScopeRejectsOutOfScopePathEvidenceKeys(t *testing.T) {
	repo := &fakeRepository{
		scopeResults: []core.ReceiptScope{{
			ProjectID:         "project.alpha",
			ReceiptID:         "receipt.abc123",
			PointerKeys:       []string{"project.alpha:.acm/acm-rules.yaml#rule.scope"},
			InitialScopePaths: []string{"src/allowed.go"},
		}},
		pointerLookupResults: []core.CandidatePointer{
			candidate("project.alpha:src/outside.go#handler", "src/outside.go", false, []string{"backend"}),
			candidate("project.alpha:src/discovered.go", "src/discovered.go", false, []string{"backend"}),
		},
		workPlanLookupResult: []core.WorkPlan{{
			ProjectID:       "project.alpha",
			PlanKey:         "plan:receipt.abc123",
			ReceiptID:       "receipt.abc123",
			DiscoveredPaths: []string{"src/discovered.go"},
		}},
	}
	svc, err := New(repo)
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	result, apiErr := svc.Memory(context.Background(), v1.MemoryCommandPayload{
		ProjectID:   "project.alpha",
		ReceiptID:   "receipt.abc123",
		AutoPromote: boolPtr(true),
		Memory: v1.MemoryPayload{
			Category:            v1.MemoryCategoryDecision,
			Subject:             "Out of scope evidence",
			Content:             "Evidence keys outside effective scope should be rejected.",
			RelatedPointerKeys:  []string{"project.alpha:src/discovered.go"},
			Tags:                []string{"backend"},
			Confidence:          4,
			EvidencePointerKeys: []string{"project.alpha:src/outside.go#handler"},
		},
	})
	if apiErr != nil {
		t.Fatalf("unexpected API error: %+v", apiErr)
	}
	if result.Status != "rejected" || result.Validation.HardPassed {
		t.Fatalf("expected rejected hard-fail result, got %+v", result)
	}
	if len(result.Validation.Errors) == 0 || !strings.Contains(result.Validation.Errors[0], "outside effective scope") {
		t.Fatalf("expected effective-scope validation error, got %+v", result.Validation)
	}
}

func TestMemory_EffectiveScopeRejectsInventedPointerKeysEvenForAllowedPaths(t *testing.T) {
	repo := &fakeRepository{
		scopeResults: []core.ReceiptScope{{
			ProjectID:         "project.alpha",
			ReceiptID:         "receipt.abc123",
			PointerKeys:       []string{"project.alpha:.acm/acm-rules.yaml#rule.scope"},
			InitialScopePaths: []string{"src/allowed.go"},
		}},
		workPlanLookupResult: []core.WorkPlan{{
			ProjectID:       "project.alpha",
			PlanKey:         "plan:receipt.abc123",
			ReceiptID:       "receipt.abc123",
			DiscoveredPaths: []string{"src/discovered.go"},
		}},
	}
	svc, err := New(repo)
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	result, apiErr := svc.Memory(context.Background(), v1.MemoryCommandPayload{
		ProjectID:   "project.alpha",
		ReceiptID:   "receipt.abc123",
		AutoPromote: boolPtr(true),
		Memory: v1.MemoryPayload{
			Category:            v1.MemoryCategoryDecision,
			Subject:             "Invented pointer key",
			Content:             "Allowed paths must still resolve to real indexed pointers.",
			RelatedPointerKeys:  []string{"project.alpha:src/discovered.go#invented"},
			Tags:                []string{"backend"},
			Confidence:          4,
			EvidencePointerKeys: []string{"project.alpha:src/allowed.go#invented"},
		},
	})
	if apiErr != nil {
		t.Fatalf("unexpected API error: %+v", apiErr)
	}
	if result.Status != "rejected" || result.Validation.HardPassed {
		t.Fatalf("expected rejected hard-fail result, got %+v", result)
	}
	if len(result.Validation.Errors) == 0 || !strings.Contains(result.Validation.Errors[0], "outside effective scope") {
		t.Fatalf("expected effective-scope validation error, got %+v", result.Validation)
	}
	if len(repo.pointerLookupCalls) != 2 {
		t.Fatalf("expected two pointer lookups for invented keys, got %d", len(repo.pointerLookupCalls))
	}
}

func TestMemory_SoftFailWithAutoPromoteTrueReturnsPending(t *testing.T) {
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

	result, apiErr := svc.Memory(context.Background(), v1.MemoryCommandPayload{
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

func TestMemory_DuplicateConflictReturnsRejected(t *testing.T) {
	repo := &fakeRepository{
		scopeResults: []core.ReceiptScope{{
			ProjectID:   "project.alpha",
			ReceiptID:   "receipt.abc123",
			PointerKeys: []string{"ptr:scope-a"},
		}},
		proposeResults: []core.MemoryPersistenceResult{{
			CandidateID:      17,
			Status:           "rejected",
			PromotedMemoryID: 0,
		}},
	}
	svc, err := New(repo)
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	result, apiErr := svc.Memory(context.Background(), v1.MemoryCommandPayload{
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

func TestMemory_UnknownReceiptReturnsNotFound(t *testing.T) {
	repo := &fakeRepository{
		scopeErrors: []error{core.ErrReceiptScopeNotFound},
	}
	svc, err := New(repo)
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	_, apiErr := svc.Memory(context.Background(), v1.MemoryCommandPayload{
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

func TestMemory_FetchScopeErrorReturnsInternalError(t *testing.T) {
	repo := &fakeRepository{
		scopeErrors: []error{errors.New("scope lookup failed")},
	}
	svc, err := New(repo)
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	_, apiErr := svc.Memory(context.Background(), v1.MemoryCommandPayload{
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

func TestMemory_PersistErrorReturnsInternalError(t *testing.T) {
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

	_, apiErr := svc.Memory(context.Background(), v1.MemoryCommandPayload{
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

func TestMemory_NormalizationAndDedupeKeyDeterministic(t *testing.T) {
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

	payloadA := v1.MemoryCommandPayload{
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
	payloadB := v1.MemoryCommandPayload{
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

	if _, apiErr := svc.Memory(context.Background(), payloadA); apiErr != nil {
		t.Fatalf("unexpected API error on payloadA: %+v", apiErr)
	}
	if _, apiErr := svc.Memory(context.Background(), payloadB); apiErr != nil {
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

func TestMemory_CanonicalTagNormalization(t *testing.T) {
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

	_, apiErr := svc.Memory(context.Background(), v1.MemoryCommandPayload{
		ProjectID: "project.alpha",
		ReceiptID: "receipt.abc123",
		Memory: v1.MemoryPayload{
			Category:            v1.MemoryCategoryDecision,
			Subject:             "Canonical tags",
			Content:             "Ensure memory normalizes tags against canonical aliases.",
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

func TestMemory_TagsFileOverrideNormalizesCustomAliases(t *testing.T) {
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

	_, apiErr := svc.Memory(context.Background(), v1.MemoryCommandPayload{
		ProjectID: "project.alpha",
		ReceiptID: "receipt.abc123",
		TagsFile:  ".acm/acm-tags.yaml",
		Memory: v1.MemoryPayload{
			Category:            v1.MemoryCategoryDecision,
			Subject:             "Custom aliases",
			Content:             "Ensure memory honors repo-local aliases.",
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

func TestDone_AcceptsInScopeAndPersistsSummary(t *testing.T) {
	repo := &fakeRepository{
		scopeResults: []core.ReceiptScope{{
			ProjectID:         "project.alpha",
			ReceiptID:         "receipt.abc123",
			TaskText:          "implement completion path",
			Phase:             "execute",
			ResolvedTags:      []string{"backend"},
			PointerKeys:       []string{"code:repo"},
			MemoryIDs:         []int64{7},
			InitialScopePaths: []string{"internal/service/backend/service.go", "internal/core/repository.go"},
		}},
		saveResult: core.RunReceiptIDs{RunID: 42, ReceiptID: "receipt.abc123"},
	}
	svc, err := New(repo)
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	result, apiErr := svc.Done(context.Background(), v1.DonePayload{
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

func TestDone_WarnModeCrossChecksSuppliedFilesAgainstDetectedDelta(t *testing.T) {
	root := t.TempDir()
	writeRepoFile(t, root, "src/allowed.go", "package src\n\nfunc allowed() {}\n")
	withWorkingDir(t, root)

	repo := &fakeRepository{
		scopeResults: []core.ReceiptScope{{
			ProjectID:         "project.alpha",
			ReceiptID:         "receipt.abc123",
			InitialScopePaths: []string{"src/allowed.go"},
			BaselineCaptured:  true,
			BaselinePaths: []core.SyncPath{{
				Path:        "src/allowed.go",
				ContentHash: "stale-hash",
			}},
		}},
		workListResults: [][]core.WorkItem{{
			{ItemKey: "verify:tests", Status: core.WorkItemStatusComplete},
		}},
		saveResult: core.RunReceiptIDs{RunID: 55, ReceiptID: "receipt.abc123"},
	}
	svc, err := New(repo)
	if err != nil {
		t.Fatalf("new service: %v", err)
	}
	svc.runGitCommand = func(_ context.Context, projectRoot string, args ...string) (string, error) {
		if projectRoot != root && projectRoot != "." {
			t.Fatalf("unexpected project root: %q", projectRoot)
		}
		switch strings.Join(args, " ") {
		case "diff --name-status --find-renames HEAD":
			return "M\tsrc/allowed.go\n", nil
		case "ls-files --others --exclude-standard":
			return "", nil
		default:
			t.Fatalf("unexpected git args: %v", args)
		}
		return "", nil
	}

	result, apiErr := svc.Done(context.Background(), v1.DonePayload{
		ProjectID:    "project.alpha",
		ReceiptID:    "receipt.abc123",
		FilesChanged: []string{"src/allowed.go", "src/extra.go"},
		Outcome:      "completed with explicit list",
		ScopeMode:    v1.ScopeModeWarn,
	})
	if apiErr != nil {
		t.Fatalf("unexpected API error: %+v", apiErr)
	}
	if !result.Accepted {
		t.Fatalf("expected accepted warn-mode result: %+v", result)
	}
	wantViolations := []v1.CompletionViolation{{
		Path:   "src/extra.go",
		Reason: "path was supplied in files_changed but was not detected in the receipt baseline delta",
	}}
	if !reflect.DeepEqual(result.Violations, wantViolations) {
		t.Fatalf("unexpected violations: got %+v want %+v", result.Violations, wantViolations)
	}
	if len(repo.saveCalls) != 1 {
		t.Fatalf("expected one persisted run summary, got %d", len(repo.saveCalls))
	}
	if got, want := repo.saveCalls[0].FilesChanged, []string{"src/allowed.go"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected persisted files_changed: got %v want %v", got, want)
	}
}

func TestDone_StrictModeRejectsDetectedFilesOmittedFromSuppliedList(t *testing.T) {
	root := t.TempDir()
	writeRepoFile(t, root, "src/one.go", "package src\n\nfunc one() {}\n")
	writeRepoFile(t, root, "src/two.go", "package src\n\nfunc two() {}\n")
	withWorkingDir(t, root)

	repo := &fakeRepository{
		scopeResults: []core.ReceiptScope{{
			ProjectID:         "project.alpha",
			ReceiptID:         "receipt.abc123",
			InitialScopePaths: []string{"src/one.go", "src/two.go"},
			BaselineCaptured:  true,
			BaselinePaths: []core.SyncPath{
				{Path: "src/one.go", ContentHash: "baseline-one"},
				{Path: "src/two.go", ContentHash: "baseline-two"},
			},
		}},
		workListResults: [][]core.WorkItem{{
			{ItemKey: "verify:tests", Status: core.WorkItemStatusComplete},
		}},
	}
	svc, err := New(repo)
	if err != nil {
		t.Fatalf("new service: %v", err)
	}
	svc.runGitCommand = func(_ context.Context, projectRoot string, args ...string) (string, error) {
		if projectRoot != root && projectRoot != "." {
			t.Fatalf("unexpected project root: %q", projectRoot)
		}
		switch strings.Join(args, " ") {
		case "diff --name-status --find-renames HEAD":
			return "M\tsrc/one.go\nM\tsrc/two.go\n", nil
		case "ls-files --others --exclude-standard":
			return "", nil
		default:
			t.Fatalf("unexpected git args: %v", args)
		}
		return "", nil
	}

	result, apiErr := svc.Done(context.Background(), v1.DonePayload{
		ProjectID:    "project.alpha",
		ReceiptID:    "receipt.abc123",
		FilesChanged: []string{"src/one.go"},
		Outcome:      "completed with incomplete explicit list",
		ScopeMode:    v1.ScopeModeStrict,
	})
	if apiErr != nil {
		t.Fatalf("unexpected API error: %+v", apiErr)
	}
	if result.Accepted {
		t.Fatalf("expected strict-mode rejection: %+v", result)
	}
	wantViolations := []v1.CompletionViolation{{
		Path:   "src/two.go",
		Reason: "path was detected in the receipt baseline delta but was omitted from files_changed",
	}}
	if !reflect.DeepEqual(result.Violations, wantViolations) {
		t.Fatalf("unexpected violations: got %+v want %+v", result.Violations, wantViolations)
	}
	if len(repo.saveCalls) != 0 {
		t.Fatalf("did not expect persisted run summary on rejection, got %d", len(repo.saveCalls))
	}
}

func TestDone_OmittedFilesChangedUsesDetectedDelta(t *testing.T) {
	root := t.TempDir()
	writeRepoFile(t, root, "src/allowed.go", "package src\n\nfunc allowed() {}\n")
	withWorkingDir(t, root)

	repo := &fakeRepository{
		scopeResults: []core.ReceiptScope{{
			ProjectID:         "project.alpha",
			ReceiptID:         "receipt.abc123",
			InitialScopePaths: []string{"src/allowed.go"},
			BaselineCaptured:  true,
			BaselinePaths: []core.SyncPath{{
				Path:        "src/allowed.go",
				ContentHash: "baseline",
			}},
		}},
		workListResults: [][]core.WorkItem{{
			{ItemKey: "verify:tests", Status: core.WorkItemStatusComplete},
		}},
		saveResult: core.RunReceiptIDs{RunID: 56, ReceiptID: "receipt.abc123"},
	}
	svc, err := New(repo)
	if err != nil {
		t.Fatalf("new service: %v", err)
	}
	svc.runGitCommand = func(_ context.Context, projectRoot string, args ...string) (string, error) {
		if projectRoot != root && projectRoot != "." {
			t.Fatalf("unexpected project root: %q", projectRoot)
		}
		switch strings.Join(args, " ") {
		case "diff --name-status --find-renames HEAD":
			return "M\tsrc/allowed.go\n", nil
		case "ls-files --others --exclude-standard":
			return "", nil
		default:
			t.Fatalf("unexpected git args: %v", args)
		}
		return "", nil
	}

	result, apiErr := svc.Done(context.Background(), v1.DonePayload{
		ProjectID: "project.alpha",
		ReceiptID: "receipt.abc123",
		Outcome:   "completed with detected delta only",
		ScopeMode: v1.ScopeModeStrict,
	})
	if apiErr != nil {
		t.Fatalf("unexpected API error: %+v", apiErr)
	}
	if !result.Accepted {
		t.Fatalf("expected acceptance with detected delta: %+v", result)
	}
	if len(result.Violations) != 0 {
		t.Fatalf("expected no violations, got %+v", result.Violations)
	}
	if len(repo.saveCalls) != 1 {
		t.Fatalf("expected one persisted run summary, got %d", len(repo.saveCalls))
	}
	if got, want := repo.saveCalls[0].FilesChanged, []string{"src/allowed.go"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected persisted files_changed: got %v want %v", got, want)
	}
}

func TestDone_ExplicitNoFileChangesRejectsDetectedDelta(t *testing.T) {
	root := t.TempDir()
	writeRepoFile(t, root, "src/allowed.go", "package src\n\nfunc allowed() {}\n")
	withWorkingDir(t, root)

	repo := &fakeRepository{
		scopeResults: []core.ReceiptScope{{
			ProjectID:         "project.alpha",
			ReceiptID:         "receipt.abc123",
			InitialScopePaths: []string{"src/allowed.go"},
			BaselineCaptured:  true,
			BaselinePaths: []core.SyncPath{{
				Path:        "src/allowed.go",
				ContentHash: "baseline",
			}},
		}},
	}
	svc, err := New(repo)
	if err != nil {
		t.Fatalf("new service: %v", err)
	}
	svc.runGitCommand = func(_ context.Context, projectRoot string, args ...string) (string, error) {
		if projectRoot != root && projectRoot != "." {
			t.Fatalf("unexpected project root: %q", projectRoot)
		}
		switch strings.Join(args, " ") {
		case "diff --name-status --find-renames HEAD":
			return "M\tsrc/allowed.go\n", nil
		case "ls-files --others --exclude-standard":
			return "", nil
		default:
			t.Fatalf("unexpected git args: %v", args)
		}
		return "", nil
	}

	_, apiErr := svc.Done(context.Background(), v1.DonePayload{
		ProjectID:     "project.alpha",
		ReceiptID:     "receipt.abc123",
		NoFileChanges: true,
		Outcome:       "completed with explicit no-file declaration",
	})
	if apiErr == nil {
		t.Fatal("expected API error")
	}
	if apiErr.Code != "INVALID_INPUT" {
		t.Fatalf("unexpected API error code: %s", apiErr.Code)
	}
}

func TestDone_StrictModeRejectsOutOfScopeWithoutPersistence(t *testing.T) {
	repo := &fakeRepository{
		scopeResults: []core.ReceiptScope{{
			ProjectID:         "project.alpha",
			ReceiptID:         "receipt.abc123",
			InitialScopePaths: []string{"internal/service/backend/service.go"},
		}},
	}
	svc, err := New(repo)
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	result, apiErr := svc.Done(context.Background(), v1.DonePayload{
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
		Reason: "path is outside effective scope",
	}}
	if !reflect.DeepEqual(result.Violations, wantViolations) {
		t.Fatalf("unexpected violations: got %+v want %+v", result.Violations, wantViolations)
	}
	if len(repo.saveCalls) != 0 {
		t.Fatalf("did not expect save on rejection, got %d", len(repo.saveCalls))
	}
}

func TestDone_PlanKeyOnlyUsesDerivedReceiptIDAndDiscoveredPaths(t *testing.T) {
	repo := &fakeRepository{
		scopeResults: []core.ReceiptScope{{
			ProjectID:         "project.alpha",
			ReceiptID:         "receipt.abc123",
			InitialScopePaths: []string{"src/initial.go"},
		}},
		workPlanLookupResult: []core.WorkPlan{{
			ProjectID:       "project.alpha",
			PlanKey:         "plan:receipt.abc123",
			ReceiptID:       "receipt.abc123",
			DiscoveredPaths: []string{"src/discovered.go"},
		}},
		workListResults: [][]core.WorkItem{{
			{ItemKey: "verify:tests", Status: core.WorkItemStatusComplete},
		}},
		saveResult: core.RunReceiptIDs{RunID: 88, ReceiptID: "receipt.abc123"},
	}
	svc, err := New(repo)
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	result, apiErr := svc.Done(context.Background(), v1.DonePayload{
		ProjectID:    "project.alpha",
		PlanKey:      "plan:receipt.abc123",
		FilesChanged: []string{"src/discovered.go"},
		Outcome:      "completed with plan-only context",
		ScopeMode:    v1.ScopeModeStrict,
	})
	if apiErr != nil {
		t.Fatalf("unexpected API error: %+v", apiErr)
	}
	if !result.Accepted {
		t.Fatalf("expected acceptance, got %+v", result)
	}
	if len(result.Violations) != 0 {
		t.Fatalf("expected no violations, got %+v", result.Violations)
	}
	if len(repo.scopeCalls) != 1 || repo.scopeCalls[0].ReceiptID != "receipt.abc123" {
		t.Fatalf("expected derived receipt scope lookup, got %+v", repo.scopeCalls)
	}
	if len(repo.workListCalls) != 1 || repo.workListCalls[0].ReceiptID != "receipt.abc123" {
		t.Fatalf("expected derived receipt work-item lookup, got %+v", repo.workListCalls)
	}
	if len(repo.workPlanLookupCalls) != 1 || repo.workPlanLookupCalls[0].PlanKey != "plan:receipt.abc123" {
		t.Fatalf("unexpected plan lookup calls: %+v", repo.workPlanLookupCalls)
	}
	if len(repo.saveCalls) != 1 || repo.saveCalls[0].ReceiptID != "receipt.abc123" {
		t.Fatalf("expected persisted summary to use derived receipt id, got %+v", repo.saveCalls)
	}
}

func TestDone_StrictModeAcceptsManagedAcmFiles(t *testing.T) {
	repo := &fakeRepository{
		scopeResults: []core.ReceiptScope{{
			ProjectID:         "project.alpha",
			ReceiptID:         "receipt.abc123",
			InitialScopePaths: []string{"internal/core/repository.go"},
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

	result, apiErr := svc.Done(context.Background(), v1.DonePayload{
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

func TestDone_StrictModeAcceptsPlanInScopePaths(t *testing.T) {
	repo := &fakeRepository{
		scopeResults: []core.ReceiptScope{{
			ProjectID:         "project.alpha",
			ReceiptID:         "receipt.abc123",
			InitialScopePaths: []string{"src/initial.go"},
		}},
		workPlanLookupResult: []core.WorkPlan{{
			ProjectID: "project.alpha",
			PlanKey:   "plan:receipt.abc123",
			ReceiptID: "receipt.abc123",
			InScope:   []string{"README.md", "docs"},
		}},
		workListResults: [][]core.WorkItem{{
			{ItemKey: "verify:tests", Status: core.WorkItemStatusComplete},
		}},
		saveResult: core.RunReceiptIDs{RunID: 91, ReceiptID: "receipt.abc123"},
	}
	svc, err := New(repo)
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	result, apiErr := svc.Done(context.Background(), v1.DonePayload{
		ProjectID:    "project.alpha",
		PlanKey:      "plan:receipt.abc123",
		FilesChanged: []string{"README.md", "docs/getting-started.md"},
		Outcome:      "completed within plan-owned scope",
		ScopeMode:    v1.ScopeModeStrict,
	})
	if apiErr != nil {
		t.Fatalf("unexpected API error: %+v", apiErr)
	}
	if !result.Accepted {
		t.Fatalf("expected acceptance: %+v", result)
	}
	if len(result.Violations) != 0 {
		t.Fatalf("did not expect scope violations: %+v", result.Violations)
	}
	if len(repo.saveCalls) != 1 {
		t.Fatalf("expected closeout persistence, got %d", len(repo.saveCalls))
	}
}

func TestDone_StrictModeAcceptsCompletedVerifyTestsWithoutDiffReview(t *testing.T) {
	repo := &fakeRepository{
		scopeResults: []core.ReceiptScope{{
			ProjectID:         "project.alpha",
			ReceiptID:         "receipt.abc123",
			InitialScopePaths: []string{"internal/core/repository.go"},
		}},
		workListResults: [][]core.WorkItem{{
			{ItemKey: "verify:tests", Status: core.WorkItemStatusComplete},
		}},
	}
	svc, err := New(repo)
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	result, apiErr := svc.Done(context.Background(), v1.DonePayload{
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

func TestDone_WarnModeAcceptsIncompleteDefinitionOfDoneAndPersistsWarning(t *testing.T) {
	repo := &fakeRepository{
		scopeResults: []core.ReceiptScope{{
			ProjectID:         "project.alpha",
			ReceiptID:         "receipt.abc123",
			InitialScopePaths: []string{"internal/core/repository.go"},
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

	result, apiErr := svc.Done(context.Background(), v1.DonePayload{
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

func TestDone_NoWorkItemsWarnModeFlagsMissingVerify(t *testing.T) {
	repo := &fakeRepository{
		scopeResults: []core.ReceiptScope{{
			ProjectID:         "project.alpha",
			ReceiptID:         "receipt.abc123",
			InitialScopePaths: []string{"internal/core/repository.go"},
		}},
		workListResults: [][]core.WorkItem{{}},
		saveResult:      core.RunReceiptIDs{RunID: 58, ReceiptID: "receipt.abc123"},
	}
	svc, err := New(repo)
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	result, apiErr := svc.Done(context.Background(), v1.DonePayload{
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

func TestDone_NoWorkItemsStrictRejectsMissingVerify(t *testing.T) {
	repo := &fakeRepository{
		scopeResults: []core.ReceiptScope{{
			ProjectID:         "project.alpha",
			ReceiptID:         "receipt.abc123",
			InitialScopePaths: []string{"internal/core/repository.go"},
		}},
		workListResults: [][]core.WorkItem{{}},
	}
	svc, err := New(repo)
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	result, apiErr := svc.Done(context.Background(), v1.DonePayload{
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

func TestDone_BlankWorkflowDefinitionsFallBackToDefaultVerifyGate(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".acm"), 0o755); err != nil {
		t.Fatalf("mkdir .acm: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, ".acm", "acm-workflows.yaml"), []byte("version: acm.workflows.v1\ncompletion:\n  required_tasks: []\n"), 0o644); err != nil {
		t.Fatalf("write workflows file: %v", err)
	}

	repo := &fakeRepository{
		scopeResults: []core.ReceiptScope{{
			ProjectID:         "project.alpha",
			ReceiptID:         "receipt.abc123",
			InitialScopePaths: []string{"internal/core/repository.go"},
		}},
		workListResults: [][]core.WorkItem{{}},
	}
	svc, err := NewWithProjectRoot(repo, root)
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	result, apiErr := svc.Done(context.Background(), v1.DonePayload{
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

func TestDone_StrictModeRejectsMissingConfiguredWorkflowGate(t *testing.T) {
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
			ProjectID:         "project.alpha",
			ReceiptID:         "receipt.abc123",
			Phase:             "review",
			InitialScopePaths: []string{"internal/core/repository.go"},
		}},
		workListResults: [][]core.WorkItem{{
			{ItemKey: "verify:tests", Status: core.WorkItemStatusComplete},
		}},
	}
	svc, err := NewWithProjectRoot(repo, root)
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	result, apiErr := svc.Done(context.Background(), v1.DonePayload{
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

func TestDone_ConfiguredWorkflowSelectorsCanNarrowRequiredGates(t *testing.T) {
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
			ProjectID:         "project.alpha",
			ReceiptID:         "receipt.abc123",
			Phase:             "review",
			InitialScopePaths: []string{"README.md"},
		}},
		workListResults: [][]core.WorkItem{{}},
		saveResult:      core.RunReceiptIDs{RunID: 88, ReceiptID: "receipt.abc123"},
	}
	svc, err := NewWithProjectRoot(repo, root)
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	result, apiErr := svc.Done(context.Background(), v1.DonePayload{
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

func TestDone_InvalidWorkflowDefinitionsReturnUserInputError(t *testing.T) {
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
			ProjectID:         "project.alpha",
			ReceiptID:         "receipt.abc123",
			Phase:             "review",
			InitialScopePaths: []string{"internal/core/repository.go"},
		}},
		workListResults: [][]core.WorkItem{{}},
	}
	svc, err := NewWithProjectRoot(repo, root)
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	_, apiErr := svc.Done(context.Background(), v1.DonePayload{
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

func TestDone_ExplicitNoFileChangesAcceptsCleanlyInDefaultMode(t *testing.T) {
	repo := &fakeRepository{
		scopeResults: []core.ReceiptScope{{
			ProjectID: "project.alpha",
			ReceiptID: "receipt.abc123",
		}},
		saveResult: core.RunReceiptIDs{RunID: 74, ReceiptID: "receipt.abc123"},
	}
	svc, err := New(repo)
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	result, apiErr := svc.Done(context.Background(), v1.DonePayload{
		ProjectID:     "project.alpha",
		ReceiptID:     "receipt.abc123",
		NoFileChanges: true,
		Outcome:       "completed",
	})
	if apiErr != nil {
		t.Fatalf("unexpected API error: %+v", apiErr)
	}
	if !result.Accepted {
		t.Fatalf("expected default-mode acceptance: %+v", result)
	}
	if got, want := result.DefinitionOfDoneIssues, []string(nil); !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected DoD issues: got %v want %v", got, want)
	}
	if len(repo.saveCalls) != 1 {
		t.Fatalf("expected one persisted run summary, got %d", len(repo.saveCalls))
	}
	if repo.saveCalls[0].Status != "accepted" {
		t.Fatalf("unexpected persisted status: %q", repo.saveCalls[0].Status)
	}
}

func TestDone_ExplicitNoFileChangesStrictAcceptsWithoutDefaultVerifyGate(t *testing.T) {
	repo := &fakeRepository{
		scopeResults: []core.ReceiptScope{{
			ProjectID: "project.alpha",
			ReceiptID: "receipt.abc123",
		}},
		saveResult: core.RunReceiptIDs{RunID: 75, ReceiptID: "receipt.abc123"},
	}
	svc, err := New(repo)
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	result, apiErr := svc.Done(context.Background(), v1.DonePayload{
		ProjectID:     "project.alpha",
		ReceiptID:     "receipt.abc123",
		NoFileChanges: true,
		Outcome:       "completed",
		ScopeMode:     v1.ScopeModeStrict,
	})
	if apiErr != nil {
		t.Fatalf("unexpected API error: %+v", apiErr)
	}
	if !result.Accepted {
		t.Fatalf("expected strict-mode acceptance for legitimate no-file completion: %+v", result)
	}
	if got, want := result.DefinitionOfDoneIssues, []string(nil); !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected DoD issues: got %v want %v", got, want)
	}
	if len(repo.saveCalls) != 1 {
		t.Fatalf("expected one persisted run summary on strict acceptance, got %d", len(repo.saveCalls))
	}
	if repo.saveCalls[0].Status != "accepted" {
		t.Fatalf("unexpected persisted status: %q", repo.saveCalls[0].Status)
	}
}

func TestDone_ExplicitNoFileChangesStillEnforcesMatchingWorkflowGate(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".acm"), 0o755); err != nil {
		t.Fatalf("mkdir .acm: %v", err)
	}
	workflowsYAML := "version: acm.workflows.v1\ncompletion:\n  required_tasks:\n    - key: review:cross-llm\n      select:\n        always_run: true\n"
	if err := os.WriteFile(filepath.Join(root, ".acm", "acm-workflows.yaml"), []byte(workflowsYAML), 0o644); err != nil {
		t.Fatalf("write workflows file: %v", err)
	}

	repo := &fakeRepository{
		scopeResults: []core.ReceiptScope{{
			ProjectID: "project.alpha",
			ReceiptID: "receipt.abc123",
			Phase:     "review",
		}},
		saveResult: core.RunReceiptIDs{RunID: 76, ReceiptID: "receipt.abc123"},
	}
	svc, err := NewWithProjectRoot(repo, root)
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	result, apiErr := svc.Done(context.Background(), v1.DonePayload{
		ProjectID:     "project.alpha",
		ReceiptID:     "receipt.abc123",
		NoFileChanges: true,
		Outcome:       "planned",
		ScopeMode:     v1.ScopeModeStrict,
	})
	if apiErr != nil {
		t.Fatalf("unexpected API error: %+v", apiErr)
	}
	if result.Accepted {
		t.Fatalf("expected strict-mode rejection when a matching no-file workflow gate is missing: %+v", result)
	}
	wantIssues := []string{"required workflow work item is missing: review:cross-llm"}
	if got, want := result.DefinitionOfDoneIssues, wantIssues; !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected DoD issues: got %v want %v", got, want)
	}
	if len(repo.saveCalls) != 0 {
		t.Fatalf("expected no persisted run summary on strict rejection, got %d", len(repo.saveCalls))
	}
}

func TestDone_WithoutReliableBaselineRequiresExplicitIntent(t *testing.T) {
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

	_, apiErr := svc.Done(context.Background(), v1.DonePayload{
		ProjectID: "project.alpha",
		ReceiptID: "receipt.abc123",
		Outcome:   "completed",
	})
	if apiErr == nil {
		t.Fatal("expected API error")
	}
	if apiErr.Code != "INVALID_INPUT" {
		t.Fatalf("unexpected API error code: %s", apiErr.Code)
	}
}

func TestDone_DefaultModeWarnAcceptsOutOfScopeAndPersistsSummary(t *testing.T) {
	repo := &fakeRepository{
		scopeResults: []core.ReceiptScope{{
			ProjectID:         "project.alpha",
			ReceiptID:         "receipt.abc123",
			TaskText:          "default warn scope check",
			Phase:             "execute",
			ResolvedTags:      []string{"backend"},
			PointerKeys:       []string{"code:repo"},
			InitialScopePaths: []string{"src/allowed.go"},
		}},
		saveResult: core.RunReceiptIDs{RunID: 64, ReceiptID: "receipt.abc123"},
	}
	svc, err := New(repo)
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	result, apiErr := svc.Done(context.Background(), v1.DonePayload{
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

func TestDone_UnknownReceiptReturnsNotFound(t *testing.T) {
	repo := &fakeRepository{
		scopeErrors: []error{core.ErrReceiptScopeNotFound},
	}
	svc, err := New(repo)
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	_, apiErr := svc.Done(context.Background(), v1.DonePayload{
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

func TestDone_FetchScopeErrorReturnsInternalError(t *testing.T) {
	repo := &fakeRepository{
		scopeErrors: []error{errors.New("db fetch failed")},
	}
	svc, err := New(repo)
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	_, apiErr := svc.Done(context.Background(), v1.DonePayload{
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

func TestDone_SaveErrorReturnsInternalError(t *testing.T) {
	repo := &fakeRepository{
		scopeResults: []core.ReceiptScope{{
			ProjectID:         "project.alpha",
			ReceiptID:         "receipt.abc123",
			InitialScopePaths: []string{"internal/core/repository.go"},
		}},
		saveError: errors.New("insert failed"),
	}
	svc, err := New(repo)
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	_, apiErr := svc.Done(context.Background(), v1.DonePayload{
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

func TestDone_NormalizesFilesChangedDeterministically(t *testing.T) {
	repo := &fakeRepository{
		scopeResults: []core.ReceiptScope{{
			ProjectID:         "project.alpha",
			ReceiptID:         "receipt.abc123",
			InitialScopePaths: []string{"docs/readme.md", "src/pkg/file.go"},
		}},
	}
	svc, err := New(repo)
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	result, apiErr := svc.Done(context.Background(), v1.DonePayload{
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

	health, apiErr := svc.Health(context.Background(), v1.HealthPayload{
		ProjectID:           "project.alpha",
		MaxFindingsPerCheck: &maxFindings,
	})
	if apiErr != nil {
		t.Fatalf("unexpected API error: %+v", apiErr)
	}
	result := health.Check
	if result == nil {
		t.Fatalf("expected check result, got %+v", health)
	}

	if result.Summary.OK {
		t.Fatalf("expected summary not ok: %+v", result.Summary)
	}
	if result.Summary.TotalFindings != 10 {
		t.Fatalf("unexpected total findings: got %d want 10", result.Summary.TotalFindings)
	}

	wantOrder := []string{
		"administrative_closeout_plans",
		"duplicate_labels",
		"empty_descriptions",
		"orphan_relations",
		"pending_quarantines",
		"stale_pointers",
		"stale_work_plans",
		"terminal_plan_status_drift",
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
	var duplicateLabels *v1.HealthCheckItem
	for i := range result.Checks {
		if result.Checks[i].Name == "duplicate_labels" {
			duplicateLabels = &result.Checks[i]
			break
		}
	}
	if duplicateLabels == nil || len(duplicateLabels.Samples) == 0 {
		t.Fatalf("expected include_details default to include samples for non-empty checks, got %+v", result.Checks)
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

func TestStatus_PreviewsContextAndLoadedSources(t *testing.T) {
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
		TaskText:  "diagnose why context chose these pointers",
		Phase:     v1.PhaseExecute,
	})
	if apiErr != nil {
		t.Fatalf("unexpected API error: %+v", apiErr)
	}
	if !result.Summary.Ready {
		t.Fatalf("expected ready status, got missing %+v", result.Missing)
	}
	if result.Context == nil || result.Context.Status != "ok" {
		t.Fatalf("expected context preview, got %+v", result.Context)
	}
	if result.Context.RuleCount != 1 {
		t.Fatalf("expected 1 rule in context preview, got %+v", result.Context)
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
	if len(repo.candidateCalls) != 0 {
		t.Fatalf("did not expect indexed candidate queries for status preview, got %d", len(repo.candidateCalls))
	}
}

func TestStatus_WarnsAboutStaleAndAdministrativePlans(t *testing.T) {
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
		".acm/acm-tests.yaml":     "version: acm.tests.v1\ndefaults:\n  cwd: .\n  timeout_sec: 120\ntests: []\n",
		".acm/acm-workflows.yaml": "version: acm.workflows.v1\ncompletion:\n  required_tasks: []\n",
	}
	for rel, contents := range files {
		full := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.WriteFile(full, []byte(contents), 0o644); err != nil {
			t.Fatalf("write %s: %v", rel, err)
		}
	}

	now := time.Now().UTC()
	repo := &fakeRepository{
		workPlanListResults: [][]core.WorkPlanSummary{{
			{
				PlanKey:   "plan:receipt.stale",
				Status:    core.PlanStatusInProgress,
				ReceiptID: "receipt.stale",
				UpdatedAt: now.Add(-8 * 24 * time.Hour),
			},
			{
				PlanKey:   "plan:receipt.closeout",
				Status:    core.PlanStatusInProgress,
				ReceiptID: "receipt.closeout",
				UpdatedAt: now,
			},
		}},
		workPlanLookupResult: []core.WorkPlan{
			{
				ProjectID: "project.alpha",
				PlanKey:   "plan:receipt.stale",
				ReceiptID: "receipt.stale",
				Status:    core.PlanStatusInProgress,
				Tasks: []core.WorkItem{
					{ItemKey: "verify:tests", Status: core.WorkItemStatusComplete},
				},
			},
			{
				ProjectID: "project.alpha",
				PlanKey:   "plan:receipt.closeout",
				ReceiptID: "receipt.closeout",
				Status:    core.PlanStatusInProgress,
				Tasks: []core.WorkItem{
					{ItemKey: "strict-close", Summary: "Administrative closeout", Status: core.WorkItemStatusPending},
				},
			},
		},
	}
	svc, err := NewWithRuntimeStatus(repo, root, RuntimeStatusSnapshot{
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
	if result.Summary.WarningCount != 3 {
		t.Fatalf("unexpected warning count: got %d want 3", result.Summary.WarningCount)
	}
	warningCodes := make([]string, 0, len(result.Warnings))
	for _, item := range result.Warnings {
		warningCodes = append(warningCodes, item.Code)
	}
	for _, code := range []string{"stale_work_plan", "terminal_plan_status_drift", "administrative_closeout_plan"} {
		if !containsString(warningCodes, code) {
			t.Fatalf("expected warning code %q in %+v", code, result.Warnings)
		}
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
	health, apiErr := svc.Health(context.Background(), v1.HealthPayload{
		ProjectID:      "project.alpha",
		IncludeDetails: &includeDetails,
	})
	if apiErr != nil {
		t.Fatalf("unexpected API error: %+v", apiErr)
	}
	result := health.Check
	if result == nil {
		t.Fatalf("expected check result, got %+v", health)
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

	health, apiErr := svc.Health(context.Background(), v1.HealthPayload{
		ProjectID: "project.alpha",
	})
	if apiErr != nil {
		t.Fatalf("unexpected API error: %+v", apiErr)
	}
	result := health.Check
	if result == nil {
		t.Fatalf("expected check result, got %+v", health)
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

	_, apiErr := svc.Health(context.Background(), v1.HealthPayload{ProjectID: "project.alpha"})
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
	if got, want := result.SelectedTestIDs, []string{"alpha-unit", "gamma-smoke"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected selected test ids: got %v want %v", got, want)
	}
	if len(result.Selected) != 2 || result.Selected[0].TestID != "alpha-unit" || result.Selected[1].TestID != "gamma-smoke" {
		t.Fatalf("unexpected selected results: %+v", result.Selected)
	}
	if got, want := result.Selected[1].SelectionReasons, []string{"always_run=true"}; !reflect.DeepEqual(got, want) {
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
		case "gamma-smoke":
			if got := extraEnv["ACM_RECEIPT_ID"]; got != "receipt.abc123" {
				t.Fatalf("unexpected injected ACM_RECEIPT_ID: %+v", extraEnv)
			}
			if got := extraEnv["ACM_PLAN_KEY"]; got != "plan:receipt.abc123" {
				t.Fatalf("unexpected injected ACM_PLAN_KEY: %+v", extraEnv)
			}
			if got := extraEnv["ACM_VERIFY_PHASE"]; got != "review" {
				t.Fatalf("unexpected injected ACM_VERIFY_PHASE: %+v", extraEnv)
			}
			if got := extraEnv["ACM_VERIFY_TAGS_JSON"]; got != `["backend"]` {
				t.Fatalf("unexpected injected ACM_VERIFY_TAGS_JSON: %+v", extraEnv)
			}
			if got := extraEnv["ACM_VERIFY_FILES_CHANGED_JSON"]; got != `["internal/service/backend/service.go"]` {
				t.Fatalf("unexpected injected ACM_VERIFY_FILES_CHANGED_JSON: %+v", extraEnv)
			}
			exitCode := 0
			return verifyCommandRun{
				ExitCode:   &exitCode,
				Stdout:     "smoke ok\n",
				StartedAt:  base.Add(3 * time.Second),
				FinishedAt: base.Add(4 * time.Second),
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
	if result.Status != v1.VerifyStatusPassed {
		t.Fatalf("unexpected status: got %q want %q", result.Status, v1.VerifyStatusPassed)
	}
	if !result.Passed {
		t.Fatalf("expected passed=true, got %+v", result.Passed)
	}
	if result.BatchRunID == "" {
		t.Fatal("expected batch run id")
	}
	if got, want := result.SelectedTestIDs, []string{"alpha-unit", "gamma-smoke"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected selected test ids: got %v want %v", got, want)
	}
	if len(result.Results) != 2 {
		t.Fatalf("unexpected result count: %d", len(result.Results))
	}
	if result.Results[0].Status != v1.VerifyTestStatusPassed || result.Results[1].Status != v1.VerifyTestStatusPassed {
		t.Fatalf("unexpected verify results: %+v", result.Results)
	}
	if len(repo.verifySaveCalls) != 1 {
		t.Fatalf("expected one persisted verification batch, got %d", len(repo.verifySaveCalls))
	}

	saved := repo.verifySaveCalls[0]
	if saved.Status != "passed" || saved.TestsSourcePath != ".acm/acm-tests.yaml" {
		t.Fatalf("unexpected persisted batch: %+v", saved)
	}
	if saved.ReceiptID != "receipt.abc123" || saved.PlanKey != "plan:receipt.abc123" {
		t.Fatalf("unexpected persisted verify scope: %+v", saved)
	}
	if got, want := saved.SelectedTestIDs, []string{"alpha-unit", "gamma-smoke"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected persisted selected test ids: got %v want %v", got, want)
	}
	if len(saved.Results) != 2 {
		t.Fatalf("unexpected persisted results: %+v", saved.Results)
	}
	if saved.Results[0].TimeoutSec != 45 {
		t.Fatalf("expected default timeout inheritance, got %d", saved.Results[0].TimeoutSec)
	}
	if saved.Results[1].TimeoutSec != 45 {
		t.Fatalf("expected default timeout inheritance, got %d", saved.Results[1].TimeoutSec)
	}
	if len(repo.workUpsertCalls) != 1 {
		t.Fatalf("expected one work upsert call, got %d", len(repo.workUpsertCalls))
	}
	if got := repo.workUpsertCalls[0].Items[0].ItemKey; got != "verify:tests" {
		t.Fatalf("unexpected work item key: %q", got)
	}
	if got := repo.workUpsertCalls[0].Items[0].Status; got != core.WorkItemStatusComplete {
		t.Fatalf("unexpected work item status: %q", got)
	}
	if got := repo.workUpsertCalls[0].Items[0].Outcome; !strings.Contains(got, "2 verification tests passed") {
		t.Fatalf("unexpected work outcome: %q", got)
	}
	if got := repo.workUpsertCalls[0].Items[0].Evidence; len(got) != 3 || got[0] != "verifyrun:"+result.BatchRunID {
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
	if gotEnv["ACM_VERIFY_PHASE"] != "execute" {
		t.Fatalf("unexpected derived ACM_VERIFY_PHASE: %+v", gotEnv)
	}
	if gotEnv["ACM_VERIFY_TAGS_JSON"] != "[]" {
		t.Fatalf("unexpected derived ACM_VERIFY_TAGS_JSON: %+v", gotEnv)
	}
	if gotEnv["ACM_VERIFY_FILES_CHANGED_JSON"] != "[]" {
		t.Fatalf("unexpected derived ACM_VERIFY_FILES_CHANGED_JSON: %+v", gotEnv)
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
			ProjectID:         "project.alpha",
			ReceiptID:         "receipt.abc123",
			InitialScopePaths: []string{},
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

func TestRunVerifyCommand_DoesNotLoadDotEnvBackedACMRuntimeEnv(t *testing.T) {
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
			"ACM_EXPECTED_PG_DSN":                       "__EXPECT_EMPTY__",
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
		got := os.Getenv(key)
		if want == "__EXPECT_EMPTY__" {
			if got != "" {
				fmt.Fprintf(os.Stderr, "expected %s to be unset, got %q\n", key, got)
				os.Exit(3)
			}
			continue
		}
		if got != want {
			fmt.Fprintf(os.Stderr, "unexpected %s: got %q want %q\n", key, got, want)
			os.Exit(3)
		}
	}

	fmt.Fprintln(os.Stdout, "dotenv env ok")
	os.Exit(0)
}

func TestInit_DefaultEphemeralAndDeterministicEnumeration(t *testing.T) {
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

	result, apiErr := svc.Init(context.Background(), v1.InitPayload{
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
	defaultPersistPath := filepath.Join(root, ".acm", "init_candidates.json")
	if _, err := os.Stat(defaultPersistPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected no persisted candidates file by default, stat err=%v", err)
	}

	again, apiErr := svc.Init(context.Background(), v1.InitPayload{
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

func TestInit_PersistCandidatesWritesDefaultAcmPath(t *testing.T) {
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

	result, apiErr := svc.Init(context.Background(), v1.InitPayload{
		ProjectID:         "project.alpha",
		ProjectRoot:       root,
		PersistCandidates: &persist,
		RespectGitIgnore:  &respectGitIgnore,
	})
	if apiErr != nil {
		t.Fatalf("unexpected API error: %+v", apiErr)
	}

	outputPath := filepath.Join(root, ".acm", "init_candidates.json")
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

func TestInit_CustomOutputPathAndWarningsDeterministic(t *testing.T) {
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

	result, apiErr := svc.Init(context.Background(), v1.InitPayload{
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

func TestInit_SeedsCanonicalScaffoldFiles(t *testing.T) {
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

	_, apiErr := svc.Init(context.Background(), v1.InitPayload{
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

func TestInit_ExcludesManagedFilesFromInitialCandidateIndex(t *testing.T) {
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

	result, apiErr := svc.Init(context.Background(), v1.InitPayload{
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

func TestInit_SeedsSuggestedCanonicalTagsWhenRepoSignalsThem(t *testing.T) {
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

	_, apiErr := svc.Init(context.Background(), v1.InitPayload{
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

func TestInit_PopulatesExistingBlankCanonicalTagsFileWithSuggestions(t *testing.T) {
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

	_, apiErr := svc.Init(context.Background(), v1.InitPayload{
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

func TestInit_DoesNotOverwriteExistingCanonicalScaffoldFiles(t *testing.T) {
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

	_, apiErr := svc.Init(context.Background(), v1.InitPayload{
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

func TestInit_DoesNotSeedPrimaryTestsFileWhenRootTestsFileExists(t *testing.T) {
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

	_, apiErr := svc.Init(context.Background(), v1.InitPayload{
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

func TestInit_DoesNotSeedPrimaryWorkflowsFileWhenRootWorkflowsFileExists(t *testing.T) {
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

	_, apiErr := svc.Init(context.Background(), v1.InitPayload{
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

func TestInit_ApplyStarterContractSeedsContractsAndIndexesThem(t *testing.T) {
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

	result, apiErr := svc.Init(context.Background(), v1.InitPayload{
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

	templateResult, ok := initTemplateResultByID(result.TemplateResults, "starter-contract")
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
	if !strings.Contains(string(rulesRaw), "rule_startup_context") {
		t.Fatalf("expected starter rules scaffold, got %q", string(rulesRaw))
	}
}

func TestInit_ApplyDetailedPlanningEnforcementSeedsFeaturePlanningScaffold(t *testing.T) {
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

	result, apiErr := svc.Init(context.Background(), v1.InitPayload{
		ProjectID:        "project.alpha",
		ProjectRoot:      root,
		RespectGitIgnore: &respectGitIgnore,
		ApplyTemplates:   []string{"detailed-planning-enforcement"},
	})
	if apiErr != nil {
		t.Fatalf("unexpected API error: %+v", apiErr)
	}

	templateResult, ok := initTemplateResultByID(result.TemplateResults, "detailed-planning-enforcement")
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

func TestInit_DetailedPlanningEnforcementUpgradesPristineStarterScaffolds(t *testing.T) {
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

	_, apiErr := svc.Init(context.Background(), v1.InitPayload{
		ProjectID:        "project.alpha",
		ProjectRoot:      root,
		RespectGitIgnore: &respectGitIgnore,
		ApplyTemplates:   []string{"starter-contract", "verify-generic"},
	})
	if apiErr != nil {
		t.Fatalf("unexpected API error on first run: %+v", apiErr)
	}

	result, apiErr := svc.Init(context.Background(), v1.InitPayload{
		ProjectID:        "project.alpha",
		ProjectRoot:      root,
		RespectGitIgnore: &respectGitIgnore,
		ApplyTemplates:   []string{"detailed-planning-enforcement"},
	})
	if apiErr != nil {
		t.Fatalf("unexpected API error on second run: %+v", apiErr)
	}

	templateResult, ok := initTemplateResultByID(result.TemplateResults, "detailed-planning-enforcement")
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

func TestInit_ApplyVerifyProfilesReplacePristineTestsScaffold(t *testing.T) {
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

			result, apiErr := svc.Init(context.Background(), v1.InitPayload{
				ProjectID:        "project.alpha",
				ProjectRoot:      root,
				RespectGitIgnore: &respectGitIgnore,
				ApplyTemplates:   []string{tc.templateID},
			})
			if apiErr != nil {
				t.Fatalf("unexpected API error: %+v", apiErr)
			}

			templateResult, ok := initTemplateResultByID(result.TemplateResults, tc.templateID)
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

func TestInit_ReapplyStarterContractTemplateIsNoOp(t *testing.T) {
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

	_, apiErr := svc.Init(context.Background(), v1.InitPayload{
		ProjectID:        "project.alpha",
		ProjectRoot:      root,
		RespectGitIgnore: &respectGitIgnore,
		ApplyTemplates:   []string{"starter-contract"},
	})
	if apiErr != nil {
		t.Fatalf("unexpected API error on first run: %+v", apiErr)
	}

	again, apiErr := svc.Init(context.Background(), v1.InitPayload{
		ProjectID:        "project.alpha",
		ProjectRoot:      root,
		RespectGitIgnore: &respectGitIgnore,
		ApplyTemplates:   []string{"starter-contract"},
	})
	if apiErr != nil {
		t.Fatalf("unexpected API error on second run: %+v", apiErr)
	}

	templateResult, ok := initTemplateResultByID(again.TemplateResults, "starter-contract")
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

func TestInit_TemplateConflictDoesNotOverwriteEditedFiles(t *testing.T) {
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

	result, apiErr := svc.Init(context.Background(), v1.InitPayload{
		ProjectID:        "project.alpha",
		ProjectRoot:      root,
		RespectGitIgnore: &respectGitIgnore,
		ApplyTemplates:   []string{"starter-contract"},
	})
	if apiErr != nil {
		t.Fatalf("unexpected API error: %+v", apiErr)
	}

	templateResult, ok := initTemplateResultByID(result.TemplateResults, "starter-contract")
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

func TestInit_ApplyClaudeCommandPackIndexesCreatedFiles(t *testing.T) {
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

	result, apiErr := svc.Init(context.Background(), v1.InitPayload{
		ProjectID:        "project.alpha",
		ProjectRoot:      root,
		RespectGitIgnore: &respectGitIgnore,
		ApplyTemplates:   []string{"claude-command-pack"},
	})
	if apiErr != nil {
		t.Fatalf("unexpected API error: %+v", apiErr)
	}

	if result.CandidateCount != 9 || result.IndexedStubs != 9 {
		t.Fatalf("unexpected bootstrap counts: %+v", result)
	}
	templateResult, ok := initTemplateResultByID(result.TemplateResults, "claude-command-pack")
	if !ok {
		t.Fatalf("expected claude-command-pack result, got %+v", result.TemplateResults)
	}
	if len(templateResult.Created) != 8 {
		t.Fatalf("expected 8 created files, got %+v", templateResult.Created)
	}

	gotPaths := make([]string, 0, len(repo.upsertStubCalls[0]))
	for _, stub := range repo.upsertStubCalls[0] {
		gotPaths = append(gotPaths, stub.Path)
	}
	for _, required := range []string{
		".claude/acm-broker/README.md",
		".claude/commands/acm-context.md",
		".claude/commands/acm-review.md",
	} {
		if !containsString(gotPaths, required) {
			t.Fatalf("expected indexed template path %q in %v", required, gotPaths)
		}
	}
}

func TestInit_ClaudeHooksMergesSettingsJSONIdempotently(t *testing.T) {
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

	result, apiErr := svc.Init(context.Background(), v1.InitPayload{
		ProjectID:        "project.alpha",
		ProjectRoot:      root,
		RespectGitIgnore: &respectGitIgnore,
		ApplyTemplates:   []string{"claude-hooks"},
	})
	if apiErr != nil {
		t.Fatalf("unexpected API error: %+v", apiErr)
	}

	templateResult, ok := initTemplateResultByID(result.TemplateResults, "claude-hooks")
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

	again, apiErr := svc.Init(context.Background(), v1.InitPayload{
		ProjectID:        "project.alpha",
		ProjectRoot:      root,
		RespectGitIgnore: &respectGitIgnore,
		ApplyTemplates:   []string{"claude-hooks"},
	})
	if apiErr != nil {
		t.Fatalf("unexpected API error on rerun: %+v", apiErr)
	}
	againResult, ok := initTemplateResultByID(again.TemplateResults, "claude-hooks")
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

func TestInit_RemovedClaudeReceiptGuardAliasReturnsInvalidInput(t *testing.T) {
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

	_, apiErr := svc.Init(context.Background(), v1.InitPayload{
		ProjectID:        "project.alpha",
		ProjectRoot:      root,
		RespectGitIgnore: &respectGitIgnore,
		ApplyTemplates:   []string{"claude-receipt-guard"},
	})
	if apiErr == nil || apiErr.Code != "INVALID_INPUT" {
		t.Fatalf("expected invalid input for removed legacy template alias, got %+v", apiErr)
	}
}

func TestInit_UnknownTemplateReturnsInvalidInput(t *testing.T) {
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

	_, apiErr := svc.Init(context.Background(), v1.InitPayload{
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
		t.Fatalf("expected no init side effects on invalid template, stat err=%v", err)
	}
}

func TestInit_PathCollectionErrorMapsInternalError(t *testing.T) {
	repo := &fakeRepository{}
	svc, err := New(repo)
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	respectGitIgnore := false
	_, apiErr := svc.Init(ctx, v1.InitPayload{
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

func initTemplateResultByID(results []v1.InitTemplateResult, templateID string) (v1.InitTemplateResult, bool) {
	for _, result := range results {
		if result.TemplateID == templateID {
			return result, true
		}
	}
	return v1.InitTemplateResult{}, false
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

	keys := make(map[string]struct{}, len(receipt.Rules))
	for _, entry := range receipt.Rules {
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
	case "rules", "memories", "plans":
		return normalizeIndexEntries(payload[index])
	default:
		return nil
	}
}

func receiptJSONMap(receipt *v1.ContextReceipt) map[string]any {
	if receipt == nil {
		return nil
	}
	raw, err := json.Marshal(receipt)
	if err != nil {
		return nil
	}
	payload := make(map[string]any)
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil
	}
	return payload
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

func containsAllStrings(haystack, needles []string) bool {
	available := make(map[string]struct{}, len(haystack))
	for _, value := range haystack {
		available[strings.TrimSpace(value)] = struct{}{}
	}
	for _, value := range needles {
		if _, ok := available[strings.TrimSpace(value)]; !ok {
			return false
		}
	}
	return true
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

func writeRepoFile(t *testing.T, root, relPath, contents string) {
	t.Helper()
	full := filepath.Join(root, filepath.FromSlash(relPath))
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(relPath), err)
	}
	if err := os.WriteFile(full, []byte(contents), 0o644); err != nil {
		t.Fatalf("write %s: %v", relPath, err)
	}
}

func syncPathPaths(paths []core.SyncPath) []string {
	out := make([]string, 0, len(paths))
	for _, path := range paths {
		out = append(out, path.Path)
	}
	return out
}

func TestDone_WarnModeAcceptsOutOfScopeAndPersistsSummary(t *testing.T) {
	repo := &fakeRepository{
		scopeResults: []core.ReceiptScope{{
			ProjectID:         "project.alpha",
			ReceiptID:         "receipt.abc123",
			TaskText:          "warn mode scope check",
			Phase:             "execute",
			ResolvedTags:      []string{"backend"},
			PointerKeys:       []string{"code:repo"},
			InitialScopePaths: []string{"src/allowed.go"},
		}},
		saveResult: core.RunReceiptIDs{RunID: 88, ReceiptID: "receipt.abc123"},
	}
	svc, err := New(repo)
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	result, apiErr := svc.Done(context.Background(), v1.DonePayload{
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
	if len(repo.upsertStubCalls) != 0 {
		t.Fatalf("did not expect pointer stub upserts in warn mode, got %+v", repo.upsertStubCalls)
	}
	if repo.saveCalls[0].Status != "accepted_with_warnings" {
		t.Fatalf("unexpected persisted status: %q", repo.saveCalls[0].Status)
	}
}

func TestBuildIndexedPointerStubs_UsesRepoLocalCanonicalTags(t *testing.T) {
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
			ProjectID:         "project.alpha",
			ReceiptID:         "receipt.abc123",
			InitialScopePaths: []string{"src/allowed.go"},
		}},
		saveResult: core.RunReceiptIDs{RunID: 110, ReceiptID: "receipt.abc123"},
	}
	svc, err := New(repo)
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	tagNormalizer, err := svc.loadCanonicalTagNormalizer(root, "")
	if err != nil {
		t.Fatalf("load canonical tags: %v", err)
	}
	stubs := buildIndexedPointerStubs("project.alpha", []v1.CompletionViolation{{
		Path:   "src/svc/new.go",
		Reason: "index unindexed file",
	}}, tagNormalizer)
	if len(stubs) != 1 {
		t.Fatalf("expected one indexed stub, got %+v", stubs)
	}
	if stubs[0].PointerKey != "project.alpha:src/svc/new.go" {
		t.Fatalf("unexpected pointer key: %q", stubs[0].PointerKey)
	}
	if stubs[0].Kind != "code" {
		t.Fatalf("unexpected stub kind: %q", stubs[0].Kind)
	}
	wantTags := []string{"backend", "code", "indexed", "new", "src"}
	if !reflect.DeepEqual(stubs[0].Tags, wantTags) {
		t.Fatalf("unexpected indexed tags: got %v want %v", stubs[0].Tags, wantTags)
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

func TestComputeInventoryHealth_ComputesSummaryAndDetails(t *testing.T) {
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

	result, apiErr := svc.computeInventoryHealth(context.Background(), "project.alpha", "")
	if apiErr != nil {
		t.Fatalf("unexpected API error: %+v", apiErr)
	}
	if len(repo.inventoryCalls) != 1 || repo.inventoryCalls[0] != "project.alpha" {
		t.Fatalf("unexpected inventory calls: %v", repo.inventoryCalls)
	}
	if result.Summary.TotalFiles != 4 || result.Summary.IndexedFiles != 2 || result.Summary.UnindexedFiles != 2 || result.Summary.StaleFiles != 2 {
		t.Fatalf("unexpected inventory summary: %+v", result.Summary)
	}
	if !reflect.DeepEqual(result.UnindexedPaths, []string{"cmd/tool/main.go", "src/unindexed.go"}) {
		t.Fatalf("unexpected unindexed paths: %v", result.UnindexedPaths)
	}
	if !reflect.DeepEqual(result.StalePaths, []string{"docs/old.md", "src/stale.go"}) {
		t.Fatalf("unexpected stale paths: %v", result.StalePaths)
	}
	if !reflect.DeepEqual(result.UnindexedDirs, []string{"cmd/tool"}) {
		t.Fatalf("unexpected unindexed dirs: %v", result.UnindexedDirs)
	}
}

func TestComputeInventoryHealth_ExcludesManagedFilesFromTrackedSet(t *testing.T) {
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

	result, apiErr := svc.computeInventoryHealth(context.Background(), "project.alpha", "")
	if apiErr != nil {
		t.Fatalf("unexpected API error: %+v", apiErr)
	}
	if result.Summary.TotalFiles != 2 || result.Summary.IndexedFiles != 1 || result.Summary.UnindexedFiles != 1 {
		t.Fatalf("unexpected inventory summary: %+v", result.Summary)
	}
	if !reflect.DeepEqual(result.UnindexedPaths, []string{"src/unindexed.go"}) {
		t.Fatalf("unexpected unindexed paths: %v", result.UnindexedPaths)
	}
}

func TestComputeInventoryHealth_InventoryErrorMapsInternalError(t *testing.T) {
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

	_, apiErr := svc.computeInventoryHealth(context.Background(), "project.alpha", "")
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

func TestFetch_PlanKeyMissingReturnsNotFoundWithoutLegacyLookup(t *testing.T) {
	repo := &fakeRepository{}
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
	if len(result.Items) != 0 {
		t.Fatalf("expected zero fetch items, got %+v", result)
	}
	if !reflect.DeepEqual(result.NotFound, []string{key}) {
		t.Fatalf("unexpected not_found list: %+v", result.NotFound)
	}
	if len(repo.workPlanLookupCalls) != 1 {
		t.Fatalf("expected one plan lookup call, got %d", len(repo.workPlanLookupCalls))
	}
	if len(repo.fetchLookupCalls) != 0 {
		t.Fatalf("did not expect legacy fetch lookup calls, got %d", len(repo.fetchLookupCalls))
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
	if result.Items[0].Type != "receipt" || !strings.Contains(result.Items[0].Content, "\"receipt_id\":\"receipt.abc123\"") || !strings.Contains(result.Items[0].Content, "\"memory_keys\":[\"mem:42\"]") || !strings.Contains(result.Items[0].Content, "\"baseline_captured\":false") {
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
	if item.Key != key || item.Type != "pointer" {
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

func TestFetch_ReceiptLookupErrorMapsLegacyOperation(t *testing.T) {
	repo := &fakeRepository{
		fetchLookupErrors: []error{errors.New("legacy lookup failed")},
	}
	svc, err := New(repo)
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	_, apiErr := svc.Fetch(context.Background(), v1.FetchPayload{
		ProjectID: "project.alpha",
		Keys:      []string{"receipt:receipt.abc123"},
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
	if len(repo.workPlanUpsertCalls) != 2 {
		t.Fatalf("expected two work plan upsert calls after auto-close, got %d", len(repo.workPlanUpsertCalls))
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
	if call := repo.workPlanUpsertCalls[1]; call.Status != core.PlanStatusComplete || len(call.Tasks) != 0 {
		t.Fatalf("expected terminal status sync upsert, got %+v", call)
	}
}

func TestWork_AutoCloseSyncsStageMetadataFromStageTasks(t *testing.T) {
	repo := &fakeRepository{}
	svc, err := New(repo)
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	result, apiErr := svc.Work(context.Background(), v1.WorkPayload{
		ProjectID: "project.alpha",
		PlanKey:   "plan:receipt.abc123",
		ReceiptID: "receipt.abc123",
		Plan: &v1.WorkPlanPayload{
			Stages: &v1.WorkPlanStagesPayload{
				SpecOutline:        v1.WorkItemStatusInProgress,
				RefinedSpec:        v1.WorkItemStatusPending,
				ImplementationPlan: v1.WorkItemStatusPending,
			},
		},
		Tasks: []v1.WorkTaskPayload{
			{Key: "stage:spec-outline", Summary: "Spec outline", Status: v1.WorkItemStatusComplete},
			{Key: "stage:refined-spec", Summary: "Refined spec", Status: v1.WorkItemStatusComplete},
			{Key: "stage:implementation-plan", Summary: "Implementation plan", Status: v1.WorkItemStatusComplete},
			{Key: "impl:apply-fix", Summary: "Apply fix", Status: v1.WorkItemStatusComplete},
		},
	})
	if apiErr != nil {
		t.Fatalf("unexpected API error: %+v", apiErr)
	}
	if result.PlanStatus != core.PlanStatusComplete {
		t.Fatalf("unexpected plan status: %+v", result)
	}
	if len(repo.workPlanUpsertCalls) != 2 {
		t.Fatalf("expected two work plan upsert calls after auto-close, got %d", len(repo.workPlanUpsertCalls))
	}
	call := repo.workPlanUpsertCalls[1]
	if call.Status != core.PlanStatusComplete {
		t.Fatalf("expected terminal status sync upsert, got %+v", call)
	}
	wantStages := core.WorkPlanStages{
		SpecOutline:        core.WorkItemStatusComplete,
		RefinedSpec:        core.WorkItemStatusComplete,
		ImplementationPlan: core.WorkItemStatusComplete,
	}
	if !reflect.DeepEqual(call.Stages, wantStages) {
		t.Fatalf("expected terminal sync to reconcile stages: got %+v want %+v", call.Stages, wantStages)
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
		{name: "all superseded", items: []core.WorkItem{{Status: core.WorkItemStatusSuperseded}, {Status: core.WorkItemStatusSuperseded}}, want: core.PlanStatusSuperseded},
		{name: "pending dominates completed", items: []core.WorkItem{{Status: core.WorkItemStatusCompleted}, {Status: core.WorkItemStatusPending}}, want: core.PlanStatusPending},
		{name: "in progress dominates pending", items: []core.WorkItem{{Status: core.WorkItemStatusPending}, {Status: core.WorkItemStatusInProgress}}, want: core.PlanStatusInProgress},
		{name: "blocked dominates all", items: []core.WorkItem{{Status: core.WorkItemStatusCompleted}, {Status: core.WorkItemStatusBlocked}, {Status: core.WorkItemStatusInProgress}}, want: core.PlanStatusBlocked},
		{name: "complete beats superseded when mixed", items: []core.WorkItem{{Status: core.WorkItemStatusSuperseded}, {Status: core.WorkItemStatusComplete}}, want: core.PlanStatusComplete},
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
				Title:               "Init follow-up",
				Objective:           "Verify first-run onboarding and history search",
				Summary:             "Init follow-up (2 tasks)",
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
				Subject:    "Prefer history search for archived plans",
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
	if len(repo.workPlanUpsertCalls) != 2 || len(repo.workPlanUpsertCalls[0].Tasks) != 1 {
		t.Fatalf("expected review task upsert plus terminal status sync, got %+v", repo.workPlanUpsertCalls)
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
	if repo.workPlanUpsertCalls[1].Status != core.PlanStatusComplete {
		t.Fatalf("expected terminal status sync upsert, got %+v", repo.workPlanUpsertCalls[1])
	}
}

func TestReview_RunInjectsEffectiveScopeAndTaskDeltaEnv(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".acm"), 0o755); err != nil {
		t.Fatalf("mkdir .acm: %v", err)
	}
	workflowsYAML := "version: acm.workflows.v1\ncompletion:\n  required_tasks:\n    - key: review:cross-llm\n      summary: Cross-LLM review\n      run:\n        argv: [\"scripts/acm-cross-review.sh\"]\n        timeout_sec: 600\n"
	if err := os.WriteFile(filepath.Join(root, ".acm", "acm-workflows.yaml"), []byte(workflowsYAML), 0o644); err != nil {
		t.Fatalf("write workflows file: %v", err)
	}
	writeRepoFile(t, root, "src/scoped.go", "package src\n\nfunc scoped() string { return \"before\" }\n")
	writeRepoFile(t, root, "docs/discovered.md", "notes\n")
	baselinePaths := []string{".acm/acm-workflows.yaml", "docs/discovered.md", "src/scoped.go"}
	baselineHashes, err := computeFileHashes(root, baselinePaths)
	if err != nil {
		t.Fatalf("compute baseline hashes: %v", err)
	}
	writeRepoFile(t, root, "src/scoped.go", "package src\n\nfunc scoped() string { return \"after\" }\n")

	repo := &fakeRepository{
		scopeResults: []core.ReceiptScope{{
			ProjectID:         "project.alpha",
			ReceiptID:         "receipt.abc123",
			InitialScopePaths: []string{"src/scoped.go"},
			BaselineCaptured:  true,
			BaselinePaths: []core.SyncPath{
				{Path: ".acm/acm-workflows.yaml", ContentHash: baselineHashes[".acm/acm-workflows.yaml"]},
				{Path: "docs/discovered.md", ContentHash: baselineHashes["docs/discovered.md"]},
				{Path: "src/scoped.go", ContentHash: baselineHashes["src/scoped.go"]},
			},
		}},
		workPlanLookupResult: []core.WorkPlan{
			{
				ProjectID:       "project.alpha",
				ReceiptID:       "receipt.abc123",
				PlanKey:         "plan:receipt.abc123",
				DiscoveredPaths: []string{"docs/discovered.md"},
			},
			{
				ProjectID:       "project.alpha",
				ReceiptID:       "receipt.abc123",
				PlanKey:         "plan:receipt.abc123",
				DiscoveredPaths: []string{"docs/discovered.md"},
			},
		},
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
	if !result.Executed || result.ReviewStatus != v1.WorkItemStatusComplete {
		t.Fatalf("unexpected review result: %+v", result)
	}
	if gotEnv["ACM_REVIEW_BASELINE_CAPTURED"] != "true" || gotEnv["ACM_REVIEW_TASK_DELTA_SOURCE"] != "receipt_baseline" {
		t.Fatalf("unexpected baseline env: %+v", gotEnv)
	}

	var changedPaths []string
	if err := json.Unmarshal([]byte(gotEnv["ACM_REVIEW_CHANGED_PATHS_JSON"]), &changedPaths); err != nil {
		t.Fatalf("decode changed paths env: %v", err)
	}
	if !reflect.DeepEqual(changedPaths, []string{"src/scoped.go"}) {
		t.Fatalf("unexpected changed paths env: %v", changedPaths)
	}

	var effectiveScope []string
	if err := json.Unmarshal([]byte(gotEnv["ACM_REVIEW_EFFECTIVE_SCOPE_PATHS_JSON"]), &effectiveScope); err != nil {
		t.Fatalf("decode effective scope env: %v", err)
	}
	if !containsString(effectiveScope, "src/scoped.go") || !containsString(effectiveScope, "docs/discovered.md") {
		t.Fatalf("unexpected effective scope env: %v", effectiveScope)
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
	}, scope, nil)
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
	if len(repo.workPlanUpsertCalls) != 2 || len(repo.workPlanUpsertCalls[0].Tasks) != 1 {
		t.Fatalf("expected review task upsert plus terminal status sync, got %+v", repo.workPlanUpsertCalls)
	}
	if got := repo.workPlanUpsertCalls[0].Tasks[0].Summary; got != "Cross-LLM review" {
		t.Fatalf("unexpected duplicate review summary: %q", got)
	}
	if repo.workPlanUpsertCalls[1].Status != core.PlanStatusComplete {
		t.Fatalf("expected terminal status sync upsert, got %+v", repo.workPlanUpsertCalls[1])
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
	first, apiErr := computeReviewFingerprint(root, "project.alpha", "receipt.abc123", v1.DefaultReviewTaskKey, ".acm/acm-workflows.yaml", command, core.ReceiptScope{}, nil)
	if apiErr != nil {
		t.Fatalf("compute first fingerprint: %+v", apiErr)
	}
	if err := os.WriteFile(filepath.Join(root, ".acm", "acm-tests.yaml"), []byte("version: acm.tests.v1\nsmoke: []\n"), 0o644); err != nil {
		t.Fatalf("rewrite tests file: %v", err)
	}
	second, apiErr := computeReviewFingerprint(root, "project.alpha", "receipt.abc123", v1.DefaultReviewTaskKey, ".acm/acm-workflows.yaml", command, core.ReceiptScope{}, nil)
	if apiErr != nil {
		t.Fatalf("compute second fingerprint: %+v", apiErr)
	}
	if first == second {
		t.Fatalf("expected managed completion files to affect fingerprint, got %q", first)
	}
}

func TestReview_RunRerunsAfterInterruptedSameFingerprintAttempt(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".acm"), 0o755); err != nil {
		t.Fatalf("mkdir .acm: %v", err)
	}
	workflowsYAML := "version: acm.workflows.v1\ncompletion:\n  required_tasks:\n    - key: review:cross-llm\n      summary: Cross-LLM review\n      rerun_requires_new_fingerprint: true\n      run:\n        argv: [\"scripts/acm-cross-review.sh\"]\n        timeout_sec: 300\n"
	if err := os.WriteFile(filepath.Join(root, ".acm", "acm-workflows.yaml"), []byte(workflowsYAML), 0o644); err != nil {
		t.Fatalf("write workflows file: %v", err)
	}

	repo := &fakeRepository{
		scopeResults: []core.ReceiptScope{{
			ProjectID: "project.alpha",
			ReceiptID: "receipt.abc123",
		}},
		reviewAttemptResults: [][]core.ReviewAttempt{{
			{
				AttemptID:   1,
				ProjectID:   "project.alpha",
				ReceiptID:   "receipt.abc123",
				ReviewKey:   v1.DefaultReviewTaskKey,
				Fingerprint: "sha256:36ea12d376b79ca844bf8f7f89dd90475b6366e283ac5895b72672cbf3a5d72f",
				Status:      "failed",
				Passed:      false,
				Outcome:     "Review gate failed: signal: terminated",
			},
		}},
	}
	svc, err := NewWithProjectRoot(repo, root)
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	runnerCalls := 0
	svc.runReviewCommand = func(context.Context, string, workflowRunDefinition, map[string]string) verifyCommandRun {
		runnerCalls++
		exitCode := 0
		now := time.Now().UTC()
		return verifyCommandRun{
			ExitCode:   &exitCode,
			Stdout:     "PASS: Cross-LLM review passed after rerun.",
			StartedAt:  now,
			FinishedAt: now.Add(100 * time.Millisecond),
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
	if !result.Executed || result.ReviewStatus != v1.WorkItemStatusComplete {
		t.Fatalf("expected interrupted duplicate fingerprint to rerun, got %+v", result)
	}
	if runnerCalls != 1 {
		t.Fatalf("expected runner to execute once, got %d", runnerCalls)
	}
	if len(repo.reviewAttemptCalls) != 1 || repo.reviewAttemptCalls[0].Status != "passed" {
		t.Fatalf("expected a new persisted passing attempt, got %+v", repo.reviewAttemptCalls)
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
			ProjectID:         "project.alpha",
			ReceiptID:         "receipt.abc123",
			Phase:             "review",
			InitialScopePaths: []string{"internal/review.txt"},
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

	result, apiErr := svc.Done(context.Background(), v1.DonePayload{
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
	staleFingerprint, apiErr := computeReviewFingerprint(root, "project.alpha", "receipt.abc123", v1.DefaultReviewTaskKey, ".acm/acm-workflows.yaml", command, scope, nil)
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

	result, apiErr := svc.Done(context.Background(), v1.DonePayload{
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

func TestContext_DoesNotFallbackToFilesystemBaselineWhenGitUnavailableInsideGitRepo(t *testing.T) {
	root := t.TempDir()
	writeRepoFile(t, root, ".acm/acm-rules.yaml", "version: acm.rules.v1\nrules:\n  - id: rule_startup\n    summary: Startup rule\n    content: Keep the context receipt deterministic.\n    enforcement: hard\n")
	writeRepoFile(t, root, "src/main.go", "package src\n\nfunc main() {}\n")
	if err := os.MkdirAll(filepath.Join(root, ".git"), 0o755); err != nil {
		t.Fatalf("mkdir .git: %v", err)
	}
	withWorkingDir(t, root)

	repo := &fakeRepository{}
	svc, err := New(repo)
	if err != nil {
		t.Fatalf("new service: %v", err)
	}
	svc.runGitCommand = func(_ context.Context, _ string, _ ...string) (string, error) {
		return "", errors.New("git unavailable")
	}

	result, apiErr := svc.Context(context.Background(), v1.ContextPayload{
		ProjectID: "project.alpha",
		TaskText:  "fallback baseline blocked in git repo",
		Phase:     v1.PhaseExecute,
	})
	if apiErr != nil {
		t.Fatalf("unexpected API error: %+v", apiErr)
	}
	if result.Receipt == nil {
		t.Fatal("expected receipt")
	}
	if result.Receipt.Meta.BaselineCaptured {
		t.Fatal("expected baseline capture to stay unavailable when git metadata exists but git calls fail")
	}
	if len(repo.receiptUpsertCalls) != 1 {
		t.Fatalf("expected one receipt scope upsert, got %d", len(repo.receiptUpsertCalls))
	}
	if repo.receiptUpsertCalls[0].BaselineCaptured {
		t.Fatal("expected persisted baseline capture to remain false")
	}
	if len(repo.receiptUpsertCalls[0].BaselinePaths) != 0 {
		t.Fatalf("expected no persisted baseline paths, got %+v", repo.receiptUpsertCalls[0].BaselinePaths)
	}
}

func TestMemory_EffectiveScopeAcceptsDirectoryScopedEvidenceKeys(t *testing.T) {
	repo := &fakeRepository{
		scopeResults: []core.ReceiptScope{{
			ProjectID:         "project.alpha",
			ReceiptID:         "receipt.abc123",
			PointerKeys:       []string{"project.alpha:.acm/acm-rules.yaml#rule.scope"},
			InitialScopePaths: []string{"src"},
		}},
		pointerLookupResults: []core.CandidatePointer{
			candidate("project.alpha:src/allowed.go#handler", "src/allowed.go", false, []string{"backend"}),
		},
	}
	svc, err := New(repo)
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	result, apiErr := svc.Memory(context.Background(), v1.MemoryCommandPayload{
		ProjectID:   "project.alpha",
		ReceiptID:   "receipt.abc123",
		AutoPromote: boolPtr(true),
		Memory: v1.MemoryPayload{
			Category:            v1.MemoryCategoryDecision,
			Subject:             "Directory scoped evidence",
			Content:             "Evidence under a scoped directory should be accepted.",
			Tags:                []string{"backend"},
			Confidence:          4,
			EvidencePointerKeys: []string{"project.alpha:src/allowed.go#handler"},
		},
	})
	if apiErr != nil {
		t.Fatalf("unexpected API error: %+v", apiErr)
	}
	if result.Status != "promoted" || !result.Validation.HardPassed {
		t.Fatalf("expected promoted result, got %+v", result)
	}
	if len(repo.proposeCalls) != 1 {
		t.Fatalf("expected persisted memory, got %+v", repo.proposeCalls)
	}
}

func TestDone_StrictModeAcceptsDirectoryScopedChildFile(t *testing.T) {
	repo := &fakeRepository{
		scopeResults: []core.ReceiptScope{{
			ProjectID:         "project.alpha",
			ReceiptID:         "receipt.abc123",
			InitialScopePaths: []string{"internal"},
		}},
		workListResults: [][]core.WorkItem{{
			{ItemKey: "verify:tests", Status: core.WorkItemStatusComplete},
		}},
	}
	svc, err := New(repo)
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	result, apiErr := svc.Done(context.Background(), v1.DonePayload{
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
		t.Fatalf("expected strict-mode acceptance for directory-scoped child file: %+v", result)
	}
	if len(result.Violations) != 0 {
		t.Fatalf("expected no scope violations, got %+v", result.Violations)
	}
}

func TestDone_StrictModeBlocksCompletedRunnableReviewWithoutPassingExecution(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".acm"), 0o755); err != nil {
		t.Fatalf("mkdir .acm: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(root, "internal"), 0o755); err != nil {
		t.Fatalf("mkdir internal: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "internal", "review.txt"), []byte("draft\n"), 0o644); err != nil {
		t.Fatalf("write review file: %v", err)
	}
	workflowsYAML := "version: acm.workflows.v1\ncompletion:\n  required_tasks:\n    - key: review:cross-llm\n      select:\n        phases: [\"review\"]\n        changed_paths_any: [\"internal/**\"]\n      run:\n        argv: [\"scripts/acm-cross-review.sh\"]\n"
	if err := os.WriteFile(filepath.Join(root, ".acm", "acm-workflows.yaml"), []byte(workflowsYAML), 0o644); err != nil {
		t.Fatalf("write workflows file: %v", err)
	}

	repo := &fakeRepository{
		scopeResults: []core.ReceiptScope{{
			ProjectID:         "project.alpha",
			ReceiptID:         "receipt.abc123",
			Phase:             "review",
			InitialScopePaths: []string{"internal/review.txt"},
		}},
		workListResults: [][]core.WorkItem{{
			{ItemKey: v1.DefaultReviewTaskKey, Status: core.WorkItemStatusComplete, Outcome: "Manual note only"},
		}},
		reviewAttemptResults: [][]core.ReviewAttempt{nil},
	}
	svc, err := NewWithProjectRoot(repo, root)
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	result, apiErr := svc.Done(context.Background(), v1.DonePayload{
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
		t.Fatalf("expected strict completion to reject runnable review without a passing execution: %+v", result)
	}
	if len(result.DefinitionOfDoneIssues) != 1 || !strings.Contains(result.DefinitionOfDoneIssues[0], "no passing execution") {
		t.Fatalf("unexpected definition_of_done_issues: %+v", result.DefinitionOfDoneIssues)
	}
}

func TestReview_ManualCompleteRejectsRunnableWorkflowGate(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".acm"), 0o755); err != nil {
		t.Fatalf("mkdir .acm: %v", err)
	}
	workflowsYAML := "version: acm.workflows.v1\ncompletion:\n  required_tasks:\n    - key: review:cross-llm\n      summary: Cross-LLM review\n      run:\n        argv: [\"scripts/acm-cross-review.sh\"]\n"
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
		Outcome:   "Manual review note",
	})
	if apiErr == nil {
		t.Fatal("expected manual complete review to be rejected for runnable gate")
	}
	if apiErr.Code != "INVALID_INPUT" {
		t.Fatalf("unexpected error code: %s", apiErr.Code)
	}
	if !strings.Contains(apiErr.Message, "run=true") {
		t.Fatalf("unexpected error message: %+v", apiErr)
	}
}

func TestReview_RunRerunsAfterFailedSameFingerprintAttempt(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".acm"), 0o755); err != nil {
		t.Fatalf("mkdir .acm: %v", err)
	}
	workflowsYAML := "version: acm.workflows.v1\ncompletion:\n  required_tasks:\n    - key: review:cross-llm\n      summary: Cross-LLM review\n      rerun_requires_new_fingerprint: true\n      run:\n        argv: [\"scripts/acm-cross-review.sh\"]\n        timeout_sec: 300\n"
	if err := os.WriteFile(filepath.Join(root, ".acm", "acm-workflows.yaml"), []byte(workflowsYAML), 0o644); err != nil {
		t.Fatalf("write workflows file: %v", err)
	}

	scope := core.ReceiptScope{ProjectID: "project.alpha", ReceiptID: "receipt.abc123"}
	fingerprint, apiErr := computeReviewFingerprint(root, "project.alpha", "receipt.abc123", v1.DefaultReviewTaskKey, ".acm/acm-workflows.yaml", workflowRunDefinition{
		Argv:       []string{"scripts/acm-cross-review.sh"},
		TimeoutSec: 300,
	}, scope, nil)
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
				Status:      "failed",
				Passed:      false,
				Outcome:     "Review gate failed",
			},
		}},
	}
	svc, err := NewWithProjectRoot(repo, root)
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	runnerCalls := 0
	svc.runReviewCommand = func(context.Context, string, workflowRunDefinition, map[string]string) verifyCommandRun {
		runnerCalls++
		exitCode := 0
		now := time.Now().UTC()
		return verifyCommandRun{
			ExitCode:   &exitCode,
			Stdout:     "PASS: Cross-LLM review passed after rerun.",
			StartedAt:  now,
			FinishedAt: now.Add(100 * time.Millisecond),
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
	if !result.Executed || result.ReviewStatus != v1.WorkItemStatusComplete {
		t.Fatalf("expected failed duplicate fingerprint to rerun, got %+v", result)
	}
	if runnerCalls != 1 {
		t.Fatalf("expected runner to execute once, got %d", runnerCalls)
	}
	if len(repo.reviewAttemptCalls) != 1 || repo.reviewAttemptCalls[0].Status != "passed" {
		t.Fatalf("expected a new persisted passing attempt, got %+v", repo.reviewAttemptCalls)
	}
}

func TestComputeReviewFingerprintChangesWhenScopedDirectoryChildChanges(t *testing.T) {
	root := t.TempDir()
	writeRepoFile(t, root, "scripts/acm-cross-review.sh", "#!/usr/bin/env bash\nexit 0\n")
	writeRepoFile(t, root, "src/nested/review.txt", "before\n")

	command := workflowRunDefinition{
		Argv:       []string{"scripts/acm-cross-review.sh"},
		TimeoutSec: 300,
	}
	scope := core.ReceiptScope{InitialScopePaths: []string{"src"}}
	first, apiErr := computeReviewFingerprint(root, "project.alpha", "receipt.abc123", v1.DefaultReviewTaskKey, ".acm/acm-workflows.yaml", command, scope, nil)
	if apiErr != nil {
		t.Fatalf("compute first fingerprint: %+v", apiErr)
	}
	writeRepoFile(t, root, "src/nested/review.txt", "after\n")
	second, apiErr := computeReviewFingerprint(root, "project.alpha", "receipt.abc123", v1.DefaultReviewTaskKey, ".acm/acm-workflows.yaml", command, scope, nil)
	if apiErr != nil {
		t.Fatalf("compute second fingerprint: %+v", apiErr)
	}
	if first == second {
		t.Fatalf("expected scoped directory child changes to affect fingerprint, got %q", first)
	}
}

func TestComputeReviewFingerprintChangesWhenWorkflowArgumentFileChanges(t *testing.T) {
	root := t.TempDir()
	writeRepoFile(t, root, "scripts/review_runner.py", "print('review')\n")
	writeRepoFile(t, root, "configs/review.json", "{\"level\":1}\n")

	command := workflowRunDefinition{
		Argv:       []string{"python3", "scripts/review_runner.py", "configs/review.json"},
		TimeoutSec: 300,
	}
	first, apiErr := computeReviewFingerprint(root, "project.alpha", "receipt.abc123", v1.DefaultReviewTaskKey, ".acm/acm-workflows.yaml", command, core.ReceiptScope{}, nil)
	if apiErr != nil {
		t.Fatalf("compute first fingerprint: %+v", apiErr)
	}
	writeRepoFile(t, root, "configs/review.json", "{\"level\":2}\n")
	second, apiErr := computeReviewFingerprint(root, "project.alpha", "receipt.abc123", v1.DefaultReviewTaskKey, ".acm/acm-workflows.yaml", command, core.ReceiptScope{}, nil)
	if apiErr != nil {
		t.Fatalf("compute second fingerprint: %+v", apiErr)
	}
	if first == second {
		t.Fatalf("expected workflow argument files to affect fingerprint, got %q", first)
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
