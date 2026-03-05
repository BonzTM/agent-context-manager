package postgres

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"math"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/joshd/agents-context/internal/contracts/v1"
	"github.com/joshd/agents-context/internal/core"
)

const RetrievalVersion = "postgres.get_context.v1"

const (
	syncModeChanged       = "changed"
	syncModeFull          = "full"
	syncModeWorkingTree   = "working_tree"
	defaultSyncGitRange   = "HEAD~1..HEAD"
	defaultSyncProjectDir = "."
	defaultHealthDetails  = true
	defaultHealthFindings = 100
	defaultMinimumRecall  = 0.8

	requiredVerifyTestsKey      = "verify:tests"
	requiredVerifyDiffReviewKey = "verify:diff-review"

	defaultBootstrapOutputPath   = "bootstrap_candidates.json"
	defaultBootstrapRespectGit   = true
	defaultBootstrapLLMAssist    = true
	maxBootstrapWalkErrorSamples = 25
)

var healthTagPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]{0,63}$`)

type gitRunnerFunc func(ctx context.Context, projectRoot string, args ...string) (string, error)

type syncPathRecord struct {
	Path        string
	ContentHash string
	Deleted     bool
}

type syncOperationError struct {
	operation string
	err       error
}

func (e *syncOperationError) Error() string {
	if e == nil || e.err == nil {
		return ""
	}
	return e.err.Error()
}

func (e *syncOperationError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.err
}

type Service struct {
	repo          core.Repository
	runGitCommand gitRunnerFunc
}

func New(repo core.Repository) (*Service, error) {
	if repo == nil {
		return nil, fmt.Errorf("repository is required")
	}
	return &Service{
		repo:          repo,
		runGitCommand: runGitCommand,
	}, nil
}

func (s *Service) Fetch(ctx context.Context, payload v1.FetchPayload) (v1.FetchResult, *core.APIError) {
	if s == nil || s.repo == nil {
		return v1.FetchResult{}, core.NewError("INTERNAL_ERROR", "postgres service repository is not configured", nil)
	}

	projectID := strings.TrimSpace(payload.ProjectID)
	keys := fetchPayloadKeys(payload)

	items := make([]v1.FetchItem, 0, len(keys))
	notFound := make([]string, 0, len(keys))
	versionMismatches := make([]v1.FetchVersionMismatch, 0, len(keys))

	for _, key := range keys {
		var (
			item  v1.FetchItem
			found bool
			err   error
		)

		if receiptID, ok := parsePlanFetchKey(key); ok {
			item, found, err = s.fetchPlanItem(ctx, projectID, key, receiptID)
			if err != nil {
				return v1.FetchResult{}, fetchInternalError("lookup_fetch_state", err)
			}
		} else if memoryID, ok := parseMemoryFetchKey(key); ok {
			item, found, err = s.fetchMemoryItem(ctx, projectID, key, memoryID)
			if err != nil {
				return v1.FetchResult{}, fetchInternalError("lookup_memory_by_id", err)
			}
		} else {
			item, found, err = s.fetchPointerItem(ctx, projectID, key)
			if err != nil {
				return v1.FetchResult{}, fetchInternalError("lookup_pointer_by_key", err)
			}
		}

		if !found {
			notFound = append(notFound, key)
			continue
		}
		items = append(items, item)

		expectedVersion := strings.TrimSpace(payload.ExpectedVersions[key])
		if expectedVersion != "" && expectedVersion != item.Version {
			versionMismatches = append(versionMismatches, v1.FetchVersionMismatch{
				Key:      key,
				Expected: expectedVersion,
				Actual:   item.Version,
			})
		}
	}

	result := v1.FetchResult{Items: items}
	if len(notFound) > 0 {
		result.NotFound = notFound
	}
	if len(versionMismatches) > 0 {
		result.VersionMismatches = versionMismatches
	}

	return result, nil
}

func (s *Service) Work(ctx context.Context, payload v1.WorkPayload) (v1.WorkResult, *core.APIError) {
	if s == nil || s.repo == nil {
		return v1.WorkResult{}, core.NewError("INTERNAL_ERROR", "postgres service repository is not configured", nil)
	}

	projectID := strings.TrimSpace(payload.ProjectID)
	planKey := strings.TrimSpace(payload.PlanKey)
	receiptID := strings.TrimSpace(payload.ReceiptID)
	if receiptID == "" {
		if derivedReceiptID, ok := parsePlanFetchKey(planKey); ok {
			receiptID = derivedReceiptID
		}
	}
	if receiptID == "" {
		return v1.WorkResult{}, core.NewError(
			"INVALID_INPUT",
			"receipt_id is required; provide receipt_id or plan_key in format plan:<receipt_id>",
			map[string]any{
				"project_id": projectID,
				"plan_key":   planKey,
			},
		)
	}

	if _, err := s.repo.LookupFetchState(ctx, core.FetchLookupQuery{
		ProjectID: projectID,
		ReceiptID: receiptID,
	}); err != nil {
		if errors.Is(err, core.ErrFetchLookupNotFound) {
			return v1.WorkResult{}, core.NewError(
				"NOT_FOUND",
				"fetch state was not found",
				map[string]any{
					"project_id": projectID,
					"receipt_id": receiptID,
				},
			)
		}
		return v1.WorkResult{}, workInternalError("lookup_fetch_state", err)
	}

	workItems := workPayloadItems(payload)
	updatedCount := 0
	if len(workItems) > 0 {
		upserted, upsertErr := s.repo.UpsertWorkItems(ctx, core.WorkItemsUpsertInput{
			ProjectID: projectID,
			ReceiptID: receiptID,
			Items:     workItems,
		})
		if upsertErr != nil {
			return v1.WorkResult{}, workInternalError("upsert_work_items", upsertErr)
		}
		updatedCount = upserted
	}

	storedItems, err := s.repo.ListWorkItems(ctx, core.FetchLookupQuery{
		ProjectID: projectID,
		ReceiptID: receiptID,
	})
	if err != nil {
		return v1.WorkResult{}, workInternalError("list_work_items", err)
	}

	planStatus := derivePlanStatusFromWorkItems(storedItems)
	if planKey == "" {
		planKey = "plan:" + receiptID
	}

	return v1.WorkResult{
		PlanKey:    planKey,
		PlanStatus: planStatus,
		Updated:    updatedCount,
	}, nil
}

func (s *Service) ProposeMemory(ctx context.Context, payload v1.ProposeMemoryPayload) (v1.ProposeMemoryResult, *core.APIError) {
	if s == nil || s.repo == nil {
		return v1.ProposeMemoryResult{}, core.NewError("INTERNAL_ERROR", "postgres service repository is not configured", nil)
	}

	scope, err := s.repo.FetchReceiptScope(ctx, core.ReceiptScopeQuery{
		ProjectID: payload.ProjectID,
		ReceiptID: payload.ReceiptID,
	})
	if err != nil {
		if errors.Is(err, core.ErrReceiptScopeNotFound) {
			return v1.ProposeMemoryResult{}, core.NewError(
				"NOT_FOUND",
				"receipt scope was not found",
				map[string]any{
					"project_id": strings.TrimSpace(payload.ProjectID),
					"receipt_id": strings.TrimSpace(payload.ReceiptID),
				},
			)
		}
		return v1.ProposeMemoryResult{}, proposeMemoryInternalError("fetch_receipt_scope", err)
	}

	normalizedMemory := normalizeProposedMemory(payload.Memory)
	validation := validateProposedMemoryScope(normalizedMemory, scope.PointerKeys)
	dedupeKey := deterministicMemoryDedupeKey(normalizedMemory)
	autoPromote := effectiveAutoPromote(payload.AutoPromote)
	promotable := autoPromote && validation.HardPassed && validation.SoftPassed

	persisted, err := s.repo.PersistProposedMemory(ctx, core.ProposeMemoryPersistence{
		ProjectID:           payload.ProjectID,
		ReceiptID:           payload.ReceiptID,
		Category:            strings.TrimSpace(string(normalizedMemory.Category)),
		Subject:             normalizedMemory.Subject,
		Content:             normalizedMemory.Content,
		Confidence:          normalizedMemory.Confidence,
		Tags:                append([]string(nil), normalizedMemory.Tags...),
		RelatedPointerKeys:  append([]string(nil), normalizedMemory.RelatedPointerKeys...),
		EvidencePointerKeys: append([]string(nil), normalizedMemory.EvidencePointerKeys...),
		DedupeKey:           dedupeKey,
		Validation: core.ProposeMemoryValidation{
			HardPassed: validation.HardPassed,
			SoftPassed: validation.SoftPassed,
			Errors:     append([]string(nil), validation.Errors...),
			Warnings:   append([]string(nil), validation.Warnings...),
		},
		AutoPromote: autoPromote,
		Promotable:  promotable,
	})
	if err != nil {
		return v1.ProposeMemoryResult{}, proposeMemoryInternalError("persist_proposed_memory", err)
	}

	result := v1.ProposeMemoryResult{
		CandidateID: int(persisted.CandidateID),
		Status:      strings.TrimSpace(persisted.Status),
		Validation:  validation,
	}
	if persisted.PromotedMemoryID > 0 {
		result.PromotedMemoryID = int(persisted.PromotedMemoryID)
	}

	return result, nil
}

func (s *Service) ReportCompletion(ctx context.Context, payload v1.ReportCompletionPayload) (v1.ReportCompletionResult, *core.APIError) {
	if s == nil || s.repo == nil {
		return v1.ReportCompletionResult{}, core.NewError("INTERNAL_ERROR", "postgres service repository is not configured", nil)
	}

	scope, err := s.repo.FetchReceiptScope(ctx, core.ReceiptScopeQuery{
		ProjectID: payload.ProjectID,
		ReceiptID: payload.ReceiptID,
	})
	if err != nil {
		if errors.Is(err, core.ErrReceiptScopeNotFound) {
			return v1.ReportCompletionResult{}, core.NewError(
				"NOT_FOUND",
				"receipt scope was not found",
				map[string]any{
					"project_id": strings.TrimSpace(payload.ProjectID),
					"receipt_id": strings.TrimSpace(payload.ReceiptID),
				},
			)
		}
		return v1.ReportCompletionResult{}, reportCompletionInternalError("fetch_receipt_scope", err)
	}

	filesChanged := normalizeCompletionPaths(payload.FilesChanged)
	allowedPaths := normalizeCompletionPaths(scope.PointerPaths)

	allowed := make(map[string]struct{}, len(allowedPaths))
	for _, filePath := range allowedPaths {
		allowed[filePath] = struct{}{}
	}

	violations := make([]v1.CompletionViolation, 0)
	for _, filePath := range filesChanged {
		if _, ok := allowed[filePath]; ok {
			continue
		}
		violations = append(violations, v1.CompletionViolation{
			Path:   filePath,
			Reason: "path is outside receipt scope",
		})
	}

	workItems, err := s.repo.ListWorkItems(ctx, core.FetchLookupQuery{
		ProjectID: payload.ProjectID,
		ReceiptID: payload.ReceiptID,
	})
	if err != nil {
		return v1.ReportCompletionResult{}, reportCompletionInternalError("list_work_items", err)
	}

	definitionOfDoneIssues := evaluateDefinitionOfDoneIssues(workItems)
	scopeMode := effectiveScopeMode(payload.ScopeMode)
	if scopeMode == v1.ScopeModeStrict && (len(violations) > 0 || len(definitionOfDoneIssues) > 0) {
		return v1.ReportCompletionResult{
			Accepted:               false,
			Violations:             violations,
			DefinitionOfDoneIssues: definitionOfDoneIssues,
		}, nil
	}

	runStatus := "accepted"
	if len(violations) > 0 {
		switch scopeMode {
		case v1.ScopeModeAutoIndex:
			stubs := buildAutoIndexPointerStubs(payload.ProjectID, violations)
			if len(stubs) > 0 {
				if _, err := s.repo.UpsertPointerStubs(ctx, strings.TrimSpace(payload.ProjectID), stubs); err != nil {
					return v1.ReportCompletionResult{}, reportCompletionInternalError("upsert_pointer_stubs", err)
				}
			}
			runStatus = "accepted_with_auto_index"
		default:
			runStatus = "accepted_with_warnings"
		}
	}
	if len(definitionOfDoneIssues) > 0 {
		runStatus = "accepted_with_warnings"
	}

	ids, err := s.repo.SaveRunReceiptSummary(ctx, core.RunReceiptSummary{
		ProjectID:              payload.ProjectID,
		ReceiptID:              payload.ReceiptID,
		TaskText:               scope.TaskText,
		Phase:                  scope.Phase,
		ResolvedTags:           scope.ResolvedTags,
		PointerKeys:            scope.PointerKeys,
		MemoryIDs:              scope.MemoryIDs,
		Status:                 runStatus,
		FilesChanged:           filesChanged,
		DefinitionOfDoneIssues: definitionOfDoneIssues,
		Outcome:                strings.TrimSpace(payload.Outcome),
	})
	if err != nil {
		return v1.ReportCompletionResult{}, reportCompletionInternalError("save_run_receipt_summary", err)
	}

	return v1.ReportCompletionResult{
		Accepted:               true,
		Violations:             violations,
		DefinitionOfDoneIssues: definitionOfDoneIssues,
		RunID:                  int(ids.RunID),
	}, nil
}

func (s *Service) Sync(ctx context.Context, payload v1.SyncPayload) (v1.SyncResult, *core.APIError) {
	if s == nil || s.repo == nil {
		return v1.SyncResult{}, core.NewError("INTERNAL_ERROR", "postgres service repository is not configured", nil)
	}

	mode := normalizeSyncMode(payload.Mode)
	gitRange := normalizeSyncGitRange(mode, payload.GitRange)
	projectRoot := normalizeSyncProjectRoot(payload.ProjectRoot)
	insertNewCandidates := effectiveInsertNewCandidates(payload.InsertNewCandidates)
	projectID := strings.TrimSpace(payload.ProjectID)

	paths, err := s.collectSyncPaths(ctx, mode, gitRange, projectRoot)
	if err != nil {
		return v1.SyncResult{}, syncInternalError(syncOperationFromError(err), err)
	}

	applied, err := s.repo.ApplySync(ctx, core.SyncApplyInput{
		ProjectID:           projectID,
		Mode:                mode,
		InsertNewCandidates: insertNewCandidates,
		Paths:               toCoreSyncPaths(paths),
	})
	if err != nil {
		return v1.SyncResult{}, syncInternalError("apply_sync", err)
	}

	if _, err := s.syncCanonicalRulesets(ctx, projectID, projectRoot, true); err != nil {
		return v1.SyncResult{}, syncInternalError("sync_ruleset", err)
	}

	return v1.SyncResult{
		Updated:            applied.Updated,
		MarkedStale:        applied.MarkedStale,
		NewCandidates:      applied.NewCandidates,
		DeletedMarkedStale: applied.DeletedMarkedStale,
		ProcessedPaths:     processedSyncPaths(paths),
	}, nil
}

func (s *Service) HealthCheck(ctx context.Context, payload v1.HealthCheckPayload) (v1.HealthCheckResult, *core.APIError) {
	if s == nil || s.repo == nil {
		return v1.HealthCheckResult{}, core.NewError("INTERNAL_ERROR", "postgres service repository is not configured", nil)
	}

	candidates, err := s.repo.FetchCandidatePointers(ctx, core.CandidatePointerQuery{
		ProjectID: strings.TrimSpace(payload.ProjectID),
		TaskText:  "",
		Limit:     candidateFetchLimit,
		StaleFilter: core.StaleFilter{
			AllowStale: true,
		},
	})
	if err != nil {
		return v1.HealthCheckResult{}, healthCheckInternalError("fetch_candidate_pointers", err)
	}

	memories, err := s.repo.FetchActiveMemories(ctx, core.ActiveMemoryQuery{
		ProjectID: strings.TrimSpace(payload.ProjectID),
		Limit:     candidateFetchLimit,
	})
	if err != nil {
		return v1.HealthCheckResult{}, healthCheckInternalError("fetch_active_memories", err)
	}

	includeDetails := effectiveHealthIncludeDetails(payload.IncludeDetails)
	maxFindings := effectiveMaxFindingsPerCheck(payload.MaxFindingsPerCheck)
	checks := buildHealthChecks(candidates, memories, includeDetails, maxFindings)

	totalFindings := 0
	for _, check := range checks {
		totalFindings += check.Count
	}

	return v1.HealthCheckResult{
		Summary: v1.HealthSummary{
			OK:            totalFindings == 0,
			TotalFindings: totalFindings,
		},
		Checks: checks,
	}, nil
}

func (s *Service) Regress(ctx context.Context, payload v1.RegressPayload) (v1.RegressResult, *core.APIError) {
	if s == nil || s.repo == nil {
		return v1.RegressResult{}, core.NewError("INTERNAL_ERROR", "postgres service repository is not configured", nil)
	}

	suite, err := loadRegressSuite(payload)
	if err != nil {
		return v1.RegressResult{}, regressInternalError("load_eval_suite", err)
	}
	if len(suite) == 0 {
		return v1.RegressResult{}, regressInternalError("load_eval_suite", fmt.Errorf("evaluation suite is empty"))
	}

	minimumRecall := effectiveMinimumRecall(payload.MinimumRecall)

	caseResults := make([]v1.RegressCaseResult, 0, len(suite))
	totalTP := 0
	totalFP := 0
	totalFN := 0

	for i, testCase := range suite {
		ctxResult, apiErr := s.GetContext(ctx, v1.GetContextPayload{
			ProjectID: payload.ProjectID,
			TaskText:  testCase.TaskText,
			Phase:     testCase.Phase,
		})
		if apiErr != nil {
			return v1.RegressResult{}, regressInternalError(
				"get_context",
				fmt.Errorf("case %d failed: %s (%s)", i, apiErr.Message, apiErr.Code),
			)
		}

		expected := expectedRegressArtifacts(testCase)
		predicted := predictedRegressArtifacts(ctxResult)
		tp, fp, fn := confusionCounts(expected, predicted)
		precision, recall, f1 := metricsFromCounts(tp, fp, fn)

		totalTP += tp
		totalFP += fp
		totalFN += fn

		caseResult := v1.RegressCaseResult{
			Index:     i,
			Precision: precision,
			Recall:    recall,
			F1:        f1,
		}
		if note := regressCaseNote(ctxResult.Status); note != "" {
			caseResult.Notes = note
		}
		caseResults = append(caseResults, caseResult)
	}

	aggregatePrecision, aggregateRecall, aggregateF1 := metricsFromCounts(totalTP, totalFP, totalFN)
	return v1.RegressResult{
		TotalCases: len(suite),
		Aggregate: v1.RegressAggregate{
			Precision: aggregatePrecision,
			Recall:    aggregateRecall,
			F1:        aggregateF1,
		},
		MinimumRecall: minimumRecall,
		Pass:          aggregateRecall >= minimumRecall,
		Cases:         caseResults,
	}, nil
}

func (s *Service) Bootstrap(ctx context.Context, payload v1.BootstrapPayload) (v1.BootstrapResult, *core.APIError) {
	if s == nil || s.repo == nil {
		return v1.BootstrapResult{}, core.NewError("INTERNAL_ERROR", "postgres service repository is not configured", nil)
	}

	projectRoot := normalizeBootstrapProjectRoot(payload.ProjectRoot)
	outputPath := resolveBootstrapOutputPath(projectRoot, payload.OutputCandidatesPath)

	paths, warnings, err := s.collectBootstrapPaths(ctx, projectRoot, outputPath, effectiveRespectGitIgnore(payload.RespectGitIgnore))
	if err != nil {
		return v1.BootstrapResult{}, bootstrapInternalError("collect_project_paths", err)
	}

	rulesetSync, err := s.syncCanonicalRulesets(ctx, strings.TrimSpace(payload.ProjectID), projectRoot, true)
	if err != nil {
		return v1.BootstrapResult{}, bootstrapInternalError("parse_ruleset", err)
	}
	warnings = append(warnings, canonicalRulesetWarnings(rulesetSync)...)

	if err := writeBootstrapCandidates(outputPath, paths); err != nil {
		return v1.BootstrapResult{}, bootstrapInternalError("write_candidates", err)
	}

	warnings = normalizeValues(warnings)

	result := v1.BootstrapResult{
		CandidateCount:       len(paths),
		OutputCandidatesPath: outputPath,
	}
	if len(warnings) > 0 {
		result.Warnings = warnings
	}
	return result, nil
}

func fetchLookupVersion(lookup core.FetchLookup) string {
	if lookup.RunID <= 0 {
		return ""
	}
	return fmt.Sprintf("%d", lookup.RunID)
}

func fetchPayloadKeys(payload v1.FetchPayload) []string {
	return fetchPayloadKeysWithReceipt(payload.Keys, payload.ReceiptID)
}

func fetchPayloadKeysWithReceipt(keys []string, receiptID string) []string {
	normalizedKeys := normalizeValues(keys)
	if len(normalizedKeys) > 0 {
		return normalizedKeys
	}

	trimmedReceiptID := strings.TrimSpace(receiptID)
	if trimmedReceiptID == "" {
		return nil
	}

	return []string{"plan:" + trimmedReceiptID}
}

func parsePlanFetchKey(raw string) (string, bool) {
	key := strings.TrimSpace(raw)
	if !strings.HasPrefix(strings.ToLower(key), "plan:") {
		return "", false
	}
	receiptID := strings.TrimSpace(key[len("plan:"):])
	if receiptID == "" {
		return "", false
	}
	return receiptID, true
}

func parseMemoryFetchKey(raw string) (int64, bool) {
	key := strings.TrimSpace(raw)
	if !strings.HasPrefix(strings.ToLower(key), "mem:") {
		return 0, false
	}
	idText := strings.TrimSpace(key[len("mem:"):])
	if idText == "" {
		return 0, false
	}
	memoryID, err := strconv.ParseInt(idText, 10, 64)
	if err != nil || memoryID <= 0 {
		return 0, false
	}
	return memoryID, true
}

func (s *Service) fetchPlanItem(ctx context.Context, projectID, key, receiptID string) (v1.FetchItem, bool, error) {
	lookup, err := s.repo.LookupFetchState(ctx, core.FetchLookupQuery{
		ProjectID: projectID,
		ReceiptID: receiptID,
	})
	if err != nil {
		if errors.Is(err, core.ErrFetchLookupNotFound) {
			return v1.FetchItem{}, false, nil
		}
		return v1.FetchItem{}, false, err
	}

	workItems := normalizeWorkItems(lookup.WorkItems)
	planStatus := normalizePlanStatus(lookup.PlanStatus)
	if planStatus == core.PlanStatusPending {
		planStatus = derivePlanStatusFromWorkItems(workItems)
	}

	version := fetchLookupVersion(lookup)
	return v1.FetchItem{
		Key:     key,
		Type:    "plan",
		Summary: fmt.Sprintf("Plan %s is %s", strings.TrimSpace(lookup.ReceiptID), planStatus),
		Status:  planStatus,
		Version: version,
	}, true, nil
}

func (s *Service) fetchPointerItem(ctx context.Context, projectID, key string) (v1.FetchItem, bool, error) {
	normalizedKey := strings.TrimSpace(key)
	if normalizedKey == "" {
		return v1.FetchItem{}, false, nil
	}

	pointer, err := s.repo.LookupPointerByKey(ctx, core.PointerLookupQuery{
		ProjectID:  projectID,
		PointerKey: normalizedKey,
	})
	if err != nil {
		if errors.Is(err, core.ErrPointerLookupNotFound) {
			return v1.FetchItem{}, false, nil
		}
		return v1.FetchItem{}, false, err
	}

	summary := pointerSummary(pointer)
	versionSeed := indexEntryVersion(pointer.Key, pointer.Path, pointer.Anchor, pointer.Kind, pointer.Label, summary, pointer.UpdatedAt.UTC().String())
	item := v1.FetchItem{
		Key:     key,
		Type:    pointerFetchType(pointer),
		Summary: summary,
		Version: versionSeed,
	}
	if content, ok := readPointerFetchContent(pointer.Path); ok {
		item.Content = content
		item.Version = indexEntryVersion(versionSeed, content)
	}

	return item, true, nil
}

func pointerFetchType(pointer core.CandidatePointer) string {
	if pointer.IsRule {
		return "rule"
	}
	return "suggestion"
}

func readPointerFetchContent(pointerPath string) (string, bool) {
	cleanPath := strings.TrimSpace(pointerPath)
	if cleanPath == "" {
		return "", false
	}
	content, err := os.ReadFile(cleanPath)
	if err != nil {
		return "", false
	}
	return string(content), true
}

func (s *Service) fetchMemoryItem(ctx context.Context, projectID, key string, memoryID int64) (v1.FetchItem, bool, error) {
	if memoryID <= 0 {
		return v1.FetchItem{}, false, nil
	}

	memory, err := s.repo.LookupMemoryByID(ctx, core.MemoryLookupQuery{
		ProjectID: projectID,
		MemoryID:  memoryID,
	})
	if err != nil {
		if errors.Is(err, core.ErrMemoryLookupNotFound) {
			return v1.FetchItem{}, false, nil
		}
		return v1.FetchItem{}, false, err
	}

	summary := memorySummary(memory)
	return v1.FetchItem{
		Key:     key,
		Type:    "memory",
		Summary: summary,
		Content: memory.Content,
		Version: indexEntryVersion(
			fmt.Sprintf("%d", memory.ID),
			memory.Subject,
			memory.Content,
			fmt.Sprintf("%d", memory.Confidence),
			memory.UpdatedAt.UTC().String(),
		),
	}, true, nil
}

func workPayloadItems(payload v1.WorkPayload) []core.WorkItem {
	if len(payload.Items) == 0 {
		return nil
	}

	items := make([]core.WorkItem, 0, len(payload.Items))
	for _, item := range payload.Items {
		items = append(items, core.WorkItem{
			ItemKey: item.Key,
			Status:  string(item.Status),
		})
	}

	return normalizeWorkItems(items)
}

func workItemsFromPaths(paths []string) []core.WorkItem {
	normalizedPaths := normalizeCompletionPaths(paths)
	if len(normalizedPaths) == 0 {
		return nil
	}

	items := make([]core.WorkItem, 0, len(normalizedPaths))
	for _, itemKey := range normalizedPaths {
		items = append(items, core.WorkItem{
			ItemKey: itemKey,
			Status:  core.WorkItemStatusComplete,
		})
	}

	return normalizeWorkItems(items)
}

func normalizeWorkItems(items []core.WorkItem) []core.WorkItem {
	if len(items) == 0 {
		return nil
	}

	priority := map[string]int{
		core.WorkItemStatusComplete:   0,
		core.WorkItemStatusPending:    1,
		core.WorkItemStatusInProgress: 2,
		core.WorkItemStatusBlocked:    3,
	}

	byItemKey := make(map[string]core.WorkItem, len(items))
	for _, item := range items {
		normalizedKey := normalizeCompletionPath(item.ItemKey)
		if normalizedKey == "" {
			continue
		}

		normalizedStatus := normalizeWorkItemStatus(item.Status)
		current, exists := byItemKey[normalizedKey]
		if !exists || priority[normalizedStatus] >= priority[current.Status] {
			byItemKey[normalizedKey] = core.WorkItem{
				ItemKey:   normalizedKey,
				Status:    normalizedStatus,
				UpdatedAt: item.UpdatedAt.UTC(),
			}
		}
	}

	if len(byItemKey) == 0 {
		return nil
	}

	keys := make([]string, 0, len(byItemKey))
	for key := range byItemKey {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	normalized := make([]core.WorkItem, 0, len(keys))
	for _, key := range keys {
		normalized = append(normalized, byItemKey[key])
	}
	return normalized
}

func normalizeWorkItemStatus(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case core.WorkItemStatusBlocked:
		return core.WorkItemStatusBlocked
	case core.WorkItemStatusInProgress:
		return core.WorkItemStatusInProgress
	case core.WorkItemStatusComplete, core.WorkItemStatusCompleted:
		return core.WorkItemStatusComplete
	default:
		return core.WorkItemStatusPending
	}
}

func normalizePlanStatus(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case core.PlanStatusBlocked:
		return core.PlanStatusBlocked
	case core.PlanStatusInProgress:
		return core.PlanStatusInProgress
	case core.PlanStatusComplete, core.PlanStatusCompleted:
		return core.PlanStatusComplete
	default:
		return core.PlanStatusPending
	}
}

func derivePlanStatusFromWorkItems(items []core.WorkItem) string {
	if len(items) == 0 {
		return core.PlanStatusPending
	}

	hasPending := false
	hasInProgress := false
	hasBlocked := false
	hasComplete := false

	for _, item := range items {
		switch normalizeWorkItemStatus(item.Status) {
		case core.WorkItemStatusBlocked:
			hasBlocked = true
		case core.WorkItemStatusInProgress:
			hasInProgress = true
		case core.WorkItemStatusComplete:
			hasComplete = true
		default:
			hasPending = true
		}
	}

	switch {
	case hasBlocked:
		return core.PlanStatusBlocked
	case hasInProgress:
		return core.PlanStatusInProgress
	case hasPending:
		return core.PlanStatusPending
	case hasComplete:
		return core.PlanStatusComplete
	default:
		return core.PlanStatusPending
	}
}

func evaluateDefinitionOfDoneIssues(items []core.WorkItem) []string {
	normalizedItems := normalizeWorkItems(items)
	if len(normalizedItems) == 0 {
		return nil
	}

	statusByKey := make(map[string]string, len(normalizedItems))
	for _, item := range normalizedItems {
		statusByKey[item.ItemKey] = normalizeWorkItemStatus(item.Status)
	}

	requiredKeys := []string{requiredVerifyTestsKey, requiredVerifyDiffReviewKey}
	issues := make([]string, 0, len(requiredKeys))
	for _, requiredKey := range requiredKeys {
		status, ok := statusByKey[requiredKey]
		if !ok {
			issues = append(issues, fmt.Sprintf("required verification work item is missing: %s", requiredKey))
			continue
		}
		if status != core.WorkItemStatusComplete {
			issues = append(issues, fmt.Sprintf("required verification work item is not complete: %s (status=%s)", requiredKey, status))
		}
	}

	if len(issues) == 0 {
		return nil
	}
	return issues
}

func normalizeCompletionPaths(paths []string) []string {
	if len(paths) == 0 {
		return nil
	}

	seen := make(map[string]struct{}, len(paths))
	out := make([]string, 0, len(paths))
	for _, raw := range paths {
		normalized := normalizeCompletionPath(raw)
		if normalized == "" {
			continue
		}
		if _, ok := seen[normalized]; ok {
			continue
		}
		seen[normalized] = struct{}{}
		out = append(out, normalized)
	}

	sort.Strings(out)
	return out
}

func normalizeCompletionPath(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return ""
	}
	normalizedSlashes := strings.ReplaceAll(trimmed, "\\", "/")
	cleaned := path.Clean(normalizedSlashes)
	if cleaned == "." {
		return ""
	}
	return cleaned
}

func (s *Service) collectSyncPaths(ctx context.Context, mode, gitRange, projectRoot string) ([]syncPathRecord, error) {
	switch mode {
	case syncModeFull:
		return s.collectFullSyncPaths(ctx, projectRoot)
	case syncModeWorkingTree:
		return s.collectWorkingTreeSyncPaths(ctx, projectRoot)
	default:
		return s.collectChangedSyncPaths(ctx, gitRange, projectRoot)
	}
}

func (s *Service) collectChangedSyncPaths(ctx context.Context, gitRange, projectRoot string) ([]syncPathRecord, error) {
	diffOutput, err := s.runGit(ctx, projectRoot, "diff", "--name-status", "--find-renames", gitRange)
	if err != nil {
		return nil, wrapSyncOperationError("git_diff", err)
	}

	paths, err := parseChangedNameStatus(diffOutput)
	if err != nil {
		return nil, wrapSyncOperationError("git_diff_parse", err)
	}
	if len(paths) == 0 {
		return nil, nil
	}

	livePaths := make([]string, 0, len(paths))
	for _, record := range paths {
		if record.Deleted {
			continue
		}
		livePaths = append(livePaths, record.Path)
	}

	hashByPath, err := s.resolveContentHashes(ctx, projectRoot, syncRangeEndRef(gitRange), livePaths)
	if err != nil {
		return nil, wrapSyncOperationError("git_ls_tree", err)
	}

	for i := range paths {
		if paths[i].Deleted {
			continue
		}
		contentHash := strings.TrimSpace(hashByPath[paths[i].Path])
		if contentHash == "" {
			return nil, wrapSyncOperationError("git_ls_tree", fmt.Errorf("missing blob hash for path %q", paths[i].Path))
		}
		paths[i].ContentHash = contentHash
	}
	return paths, nil
}

func (s *Service) collectWorkingTreeSyncPaths(ctx context.Context, projectRoot string) ([]syncPathRecord, error) {
	diffOutput, err := s.runGit(ctx, projectRoot, "diff", "--name-status", "--find-renames", "HEAD")
	if err != nil {
		return nil, wrapSyncOperationError("git_diff", err)
	}

	paths, err := parseChangedNameStatus(diffOutput)
	if err != nil {
		return nil, wrapSyncOperationError("git_diff_parse", err)
	}

	untrackedOutput, err := s.runGit(ctx, projectRoot, "ls-files", "--others", "--exclude-standard")
	if err != nil {
		return nil, wrapSyncOperationError("git_ls_files", err)
	}

	byPath := make(map[string]syncPathRecord, len(paths))
	for _, record := range paths {
		if record.Path == "" {
			continue
		}
		byPath[record.Path] = record
	}
	for _, filePath := range parseBootstrapGitPaths(untrackedOutput) {
		if filePath == "" {
			continue
		}
		byPath[filePath] = syncPathRecord{Path: filePath, Deleted: false}
	}

	if len(byPath) == 0 {
		return nil, nil
	}

	livePaths := make([]string, 0, len(byPath))
	for _, record := range byPath {
		if record.Deleted {
			continue
		}
		livePaths = append(livePaths, record.Path)
	}
	hashByPath, err := computeFileHashes(projectRoot, livePaths)
	if err != nil {
		return nil, wrapSyncOperationError("read_working_tree", err)
	}

	keys := make([]string, 0, len(byPath))
	for key := range byPath {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	result := make([]syncPathRecord, 0, len(keys))
	for _, key := range keys {
		record := byPath[key]
		if !record.Deleted {
			record.ContentHash = strings.TrimSpace(hashByPath[record.Path])
			if record.ContentHash == "" {
				return nil, wrapSyncOperationError("read_working_tree", fmt.Errorf("missing content hash for path %q", record.Path))
			}
		}
		result = append(result, record)
	}

	return result, nil
}

func (s *Service) collectFullSyncPaths(ctx context.Context, projectRoot string) ([]syncPathRecord, error) {
	output, err := s.runGit(ctx, projectRoot, "ls-tree", "-r", "HEAD")
	if err != nil {
		return nil, wrapSyncOperationError("git_ls_tree", err)
	}

	hashByPath, err := parseLsTreeHashes(output)
	if err != nil {
		return nil, wrapSyncOperationError("git_ls_tree_parse", err)
	}

	keys := make([]string, 0, len(hashByPath))
	for key := range hashByPath {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	out := make([]syncPathRecord, 0, len(keys))
	for _, key := range keys {
		out = append(out, syncPathRecord{
			Path:        key,
			ContentHash: hashByPath[key],
			Deleted:     false,
		})
	}
	return out, nil
}

func parseChangedNameStatus(output string) ([]syncPathRecord, error) {
	byPath := make(map[string]syncPathRecord)
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		columns := strings.Split(line, "\t")
		if len(columns) < 2 {
			return nil, fmt.Errorf("invalid name-status line: %q", line)
		}

		status := strings.TrimSpace(columns[0])
		switch {
		case strings.HasPrefix(status, "R"):
			if len(columns) < 3 {
				return nil, fmt.Errorf("invalid rename line: %q", line)
			}
			addSyncPathRecord(byPath, normalizeCompletionPath(columns[1]), true)
			addSyncPathRecord(byPath, normalizeCompletionPath(columns[2]), false)
		case strings.HasPrefix(status, "D"):
			addSyncPathRecord(byPath, normalizeCompletionPath(columns[1]), true)
		default:
			addSyncPathRecord(byPath, normalizeCompletionPath(columns[1]), false)
		}
	}

	keys := make([]string, 0, len(byPath))
	for key := range byPath {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	out := make([]syncPathRecord, 0, len(keys))
	for _, key := range keys {
		out = append(out, byPath[key])
	}
	return out, nil
}

func addSyncPathRecord(byPath map[string]syncPathRecord, path string, deleted bool) {
	if path == "" {
		return
	}

	current, exists := byPath[path]
	if !exists {
		byPath[path] = syncPathRecord{Path: path, Deleted: deleted}
		return
	}

	if !deleted {
		current.Deleted = false
	}
	byPath[path] = current
}

func parseLsTreeHashes(output string) (map[string]string, error) {
	hashByPath := make(map[string]string)
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		parts := strings.SplitN(line, "\t", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid ls-tree line: %q", line)
		}
		meta := strings.Fields(parts[0])
		if len(meta) < 3 {
			return nil, fmt.Errorf("invalid ls-tree metadata: %q", line)
		}
		if strings.TrimSpace(meta[1]) != "blob" {
			continue
		}

		contentHash := strings.TrimSpace(meta[2])
		if contentHash == "" {
			return nil, fmt.Errorf("missing blob hash in line: %q", line)
		}
		filePath := normalizeCompletionPath(parts[1])
		if filePath == "" {
			continue
		}
		hashByPath[filePath] = contentHash
	}
	return hashByPath, nil
}

func (s *Service) resolveContentHashes(ctx context.Context, projectRoot, ref string, paths []string) (map[string]string, error) {
	normalizedPaths := normalizeCompletionPaths(paths)
	if len(normalizedPaths) == 0 {
		return map[string]string{}, nil
	}

	if strings.TrimSpace(ref) == "" {
		ref = "HEAD"
	}

	hashes, err := s.lookupHashesForRef(ctx, projectRoot, ref, normalizedPaths)
	if err != nil {
		if ref == "HEAD" {
			return nil, err
		}
		hashes, err = s.lookupHashesForRef(ctx, projectRoot, "HEAD", normalizedPaths)
		if err != nil {
			return nil, err
		}
	}

	if ref != "HEAD" {
		missing := missingPathsForHashes(normalizedPaths, hashes)
		if len(missing) > 0 {
			fallbackHashes, fallbackErr := s.lookupHashesForRef(ctx, projectRoot, "HEAD", missing)
			if fallbackErr != nil {
				return nil, fallbackErr
			}
			for key, value := range fallbackHashes {
				hashes[key] = value
			}
		}
	}

	if missing := missingPathsForHashes(normalizedPaths, hashes); len(missing) > 0 {
		return nil, fmt.Errorf("missing blob hashes for paths: %s", strings.Join(missing, ", "))
	}

	return hashes, nil
}

func (s *Service) lookupHashesForRef(ctx context.Context, projectRoot, ref string, paths []string) (map[string]string, error) {
	args := append([]string{"ls-tree", "-r", ref, "--"}, paths...)
	output, err := s.runGit(ctx, projectRoot, args...)
	if err != nil {
		return nil, err
	}
	return parseLsTreeHashes(output)
}

func missingPathsForHashes(paths []string, hashes map[string]string) []string {
	if len(paths) == 0 {
		return nil
	}

	missing := make([]string, 0, len(paths))
	for _, p := range paths {
		if strings.TrimSpace(hashes[p]) != "" {
			continue
		}
		missing = append(missing, p)
	}
	return missing
}

func computeFileHashes(projectRoot string, paths []string) (map[string]string, error) {
	normalizedPaths := normalizeCompletionPaths(paths)
	if len(normalizedPaths) == 0 {
		return map[string]string{}, nil
	}

	root := normalizeSyncProjectRoot(projectRoot)
	hashes := make(map[string]string, len(normalizedPaths))
	for _, relativePath := range normalizedPaths {
		fullPath := filepath.Join(root, filepath.FromSlash(relativePath))
		blob, err := os.ReadFile(fullPath)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				continue
			}
			return nil, fmt.Errorf("read %s: %w", relativePath, err)
		}
		sum := sha256.Sum256(blob)
		hashes[relativePath] = hex.EncodeToString(sum[:])
	}

	return hashes, nil
}

func processedSyncPaths(paths []syncPathRecord) []string {
	if len(paths) == 0 {
		return nil
	}
	out := make([]string, 0, len(paths))
	for _, record := range paths {
		out = append(out, record.Path)
	}
	return out
}

func toCoreSyncPaths(paths []syncPathRecord) []core.SyncPath {
	if len(paths) == 0 {
		return nil
	}
	out := make([]core.SyncPath, 0, len(paths))
	for _, record := range paths {
		out = append(out, core.SyncPath{
			Path:        record.Path,
			ContentHash: record.ContentHash,
			Deleted:     record.Deleted,
		})
	}
	return out
}

func normalizeSyncMode(mode string) string {
	normalized := strings.TrimSpace(strings.ToLower(mode))
	switch normalized {
	case syncModeFull:
		return syncModeFull
	case syncModeWorkingTree:
		return syncModeWorkingTree
	default:
		return syncModeChanged
	}
}

func normalizeSyncGitRange(mode, gitRange string) string {
	normalized := strings.TrimSpace(gitRange)
	if mode == syncModeChanged && normalized == "" {
		return defaultSyncGitRange
	}
	return normalized
}

func syncRangeEndRef(gitRange string) string {
	trimmed := strings.TrimSpace(gitRange)
	if trimmed == "" {
		return "HEAD"
	}
	if strings.Contains(trimmed, "...") {
		parts := strings.SplitN(trimmed, "...", 2)
		end := strings.TrimSpace(parts[1])
		if end != "" {
			return end
		}
		return "HEAD"
	}
	if strings.Contains(trimmed, "..") {
		parts := strings.SplitN(trimmed, "..", 2)
		end := strings.TrimSpace(parts[1])
		if end != "" {
			return end
		}
	}
	return "HEAD"
}

func normalizeSyncProjectRoot(projectRoot string) string {
	trimmed := strings.TrimSpace(projectRoot)
	if trimmed == "" {
		return defaultSyncProjectDir
	}
	return trimmed
}

func effectiveInsertNewCandidates(insertNewCandidates *bool) bool {
	if insertNewCandidates == nil {
		return true
	}
	return *insertNewCandidates
}

func (s *Service) runGit(ctx context.Context, projectRoot string, args ...string) (string, error) {
	if s != nil && s.runGitCommand != nil {
		return s.runGitCommand(ctx, projectRoot, args...)
	}
	return runGitCommand(ctx, projectRoot, args...)
}

func runGitCommand(ctx context.Context, projectRoot string, args ...string) (string, error) {
	command := exec.CommandContext(ctx, "git", args...)
	command.Dir = normalizeSyncProjectRoot(projectRoot)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	command.Stdout = &stdout
	command.Stderr = &stderr

	if err := command.Run(); err != nil {
		stderrText := strings.TrimSpace(stderr.String())
		if stderrText == "" {
			stderrText = strings.TrimSpace(stdout.String())
		}
		if stderrText == "" {
			return "", fmt.Errorf("git %s: %w", strings.Join(args, " "), err)
		}
		return "", fmt.Errorf("git %s: %w: %s", strings.Join(args, " "), err, stderrText)
	}

	return stdout.String(), nil
}

func wrapSyncOperationError(operation string, err error) error {
	if err == nil {
		return nil
	}
	return &syncOperationError{operation: operation, err: err}
}

func syncOperationFromError(err error) string {
	var opErr *syncOperationError
	if errors.As(err, &opErr) && strings.TrimSpace(opErr.operation) != "" {
		return opErr.operation
	}
	return "sync"
}

func syncInternalError(operation string, err error) *core.APIError {
	return core.NewError(
		"INTERNAL_ERROR",
		"failed to sync project",
		map[string]any{
			"operation": operation,
			"error":     err.Error(),
		},
	)
}

func effectiveHealthIncludeDetails(includeDetails *bool) bool {
	if includeDetails == nil {
		return defaultHealthDetails
	}
	return *includeDetails
}

func effectiveMaxFindingsPerCheck(maxFindings *int) int {
	if maxFindings == nil || *maxFindings < 1 {
		return defaultHealthFindings
	}
	return *maxFindings
}

func buildHealthChecks(candidates []core.CandidatePointer, memories []core.ActiveMemory, includeDetails bool, maxFindings int) []v1.HealthCheckItem {
	checks := []v1.HealthCheckItem{
		healthCheckItem("duplicate_labels", "warn", duplicateLabelFindings(candidates), includeDetails, maxFindings),
		healthCheckItem("empty_descriptions", "warn", emptyDescriptionFindings(candidates), includeDetails, maxFindings),
		healthCheckItem("orphan_relations", "info", []string{}, includeDetails, maxFindings),
		healthCheckItem("pending_quarantines", "info", []string{}, includeDetails, maxFindings),
		healthCheckItem("stale_pointers", "warn", stalePointerFindings(candidates), includeDetails, maxFindings),
		healthCheckItem("unknown_tags", "warn", unknownTagFindings(candidates, memories), includeDetails, maxFindings),
		healthCheckItem("weak_memories", "warn", weakMemoryFindings(memories), includeDetails, maxFindings),
	}

	sort.Slice(checks, func(i, j int) bool {
		return checks[i].Name < checks[j].Name
	})
	return checks
}

func healthCheckItem(name, severity string, findings []string, includeDetails bool, maxFindings int) v1.HealthCheckItem {
	normalizedFindings := normalizeValues(findings)
	item := v1.HealthCheckItem{
		Name:     name,
		Severity: severity,
		Count:    len(normalizedFindings),
	}
	if includeDetails && len(normalizedFindings) > 0 {
		limit := minInt(len(normalizedFindings), maxFindings)
		item.Samples = append([]string(nil), normalizedFindings[:limit]...)
	}
	return item
}

func stalePointerFindings(candidates []core.CandidatePointer) []string {
	out := make([]string, 0)
	for _, candidate := range candidates {
		if !candidate.IsStale {
			continue
		}
		key := strings.TrimSpace(candidate.Key)
		if key == "" {
			key = normalizeCompletionPath(candidate.Path)
		}
		if key == "" {
			continue
		}
		out = append(out, key)
	}
	return out
}

func emptyDescriptionFindings(candidates []core.CandidatePointer) []string {
	out := make([]string, 0)
	for _, candidate := range candidates {
		if strings.TrimSpace(candidate.Description) != "" {
			continue
		}
		key := strings.TrimSpace(candidate.Key)
		if key == "" {
			key = normalizeCompletionPath(candidate.Path)
		}
		if key == "" {
			continue
		}
		out = append(out, key)
	}
	return out
}

func duplicateLabelFindings(candidates []core.CandidatePointer) []string {
	byLabel := make(map[string][]string)
	for _, candidate := range candidates {
		label := strings.TrimSpace(candidate.Label)
		if label == "" {
			continue
		}
		key := strings.TrimSpace(candidate.Key)
		if key == "" {
			key = normalizeCompletionPath(candidate.Path)
		}
		if key == "" {
			continue
		}
		byLabel[label] = append(byLabel[label], key)
	}

	labels := make([]string, 0, len(byLabel))
	for label := range byLabel {
		labels = append(labels, label)
	}
	sort.Strings(labels)

	out := make([]string, 0)
	for _, label := range labels {
		keys := normalizeValues(byLabel[label])
		if len(keys) < 2 {
			continue
		}
		for _, key := range keys {
			out = append(out, fmt.Sprintf("%s:%s", label, key))
		}
	}
	return out
}

func unknownTagFindings(candidates []core.CandidatePointer, memories []core.ActiveMemory) []string {
	out := make([]string, 0)
	for _, candidate := range candidates {
		key := strings.TrimSpace(candidate.Key)
		if key == "" {
			key = normalizeCompletionPath(candidate.Path)
		}
		if key == "" {
			key = "pointer"
		}
		for _, tag := range candidate.Tags {
			tag = strings.TrimSpace(tag)
			if tag == "" || healthTagPattern.MatchString(tag) {
				continue
			}
			out = append(out, fmt.Sprintf("pointer:%s:%s", key, tag))
		}
	}
	for _, memory := range memories {
		for _, tag := range memory.Tags {
			tag = strings.TrimSpace(tag)
			if tag == "" || healthTagPattern.MatchString(tag) {
				continue
			}
			out = append(out, fmt.Sprintf("memory:%d:%s", memory.ID, tag))
		}
	}
	return out
}

func weakMemoryFindings(memories []core.ActiveMemory) []string {
	out := make([]string, 0)
	for _, memory := range memories {
		if memory.Confidence <= 2 {
			out = append(out, fmt.Sprintf("memory:%d:low_confidence", memory.ID))
		}
		if len(normalizeValues(memory.RelatedPointerKeys)) == 0 {
			out = append(out, fmt.Sprintf("memory:%d:no_related_pointer_keys", memory.ID))
		}
	}
	return out
}

func loadRegressSuite(payload v1.RegressPayload) ([]v1.RegressCase, error) {
	if len(payload.EvalSuiteInline) > 0 {
		return normalizeAndValidateRegressSuite(payload.EvalSuiteInline)
	}

	suitePath := strings.TrimSpace(payload.EvalSuitePath)
	if suitePath == "" {
		return nil, fmt.Errorf("evaluation suite source is required")
	}

	content, err := os.ReadFile(suitePath)
	if err != nil {
		return nil, fmt.Errorf("read eval suite path: %w", err)
	}

	var inline []v1.RegressCase
	if err := json.Unmarshal(content, &inline); err == nil {
		if len(inline) == 0 {
			return nil, fmt.Errorf("evaluation suite file is empty")
		}
		return normalizeAndValidateRegressSuite(inline)
	}

	var wrapped struct {
		Cases []v1.RegressCase `json:"cases"`
	}
	if err := json.Unmarshal(content, &wrapped); err != nil {
		return nil, fmt.Errorf("parse eval suite path: %w", err)
	}
	if len(wrapped.Cases) == 0 {
		return nil, fmt.Errorf("evaluation suite file has no cases")
	}
	return normalizeAndValidateRegressSuite(wrapped.Cases)
}

func normalizeAndValidateRegressSuite(cases []v1.RegressCase) ([]v1.RegressCase, error) {
	normalized := make([]v1.RegressCase, 0, len(cases))
	for i := range cases {
		current := v1.RegressCase{
			TaskText:               strings.TrimSpace(cases[i].TaskText),
			Phase:                  cases[i].Phase,
			ExpectedPointerKeys:    normalizeValues(cases[i].ExpectedPointerKeys),
			ExpectedMemorySubjects: normalizeValues(cases[i].ExpectedMemorySubjects),
		}
		if current.TaskText == "" || len(current.TaskText) > 4000 {
			return nil, fmt.Errorf("eval suite case %d task_text invalid", i)
		}
		if current.Phase != v1.PhasePlan && current.Phase != v1.PhaseExecute && current.Phase != v1.PhaseReview {
			return nil, fmt.Errorf("eval suite case %d phase invalid", i)
		}
		normalized = append(normalized, current)
	}
	return normalized, nil
}

func expectedRegressArtifacts(testCase v1.RegressCase) map[string]struct{} {
	expected := make(map[string]struct{}, len(testCase.ExpectedPointerKeys)+len(testCase.ExpectedMemorySubjects))
	for _, key := range normalizeValues(testCase.ExpectedPointerKeys) {
		expected["pointer:"+key] = struct{}{}
	}
	for _, subject := range normalizeValues(testCase.ExpectedMemorySubjects) {
		expected["memory:"+subject] = struct{}{}
	}
	return expected
}

func predictedRegressArtifacts(result v1.GetContextResult) map[string]struct{} {
	predicted := make(map[string]struct{})
	if result.Status != "ok" || result.Receipt == nil {
		return predicted
	}

	for _, key := range receiptPointerKeys(result.Receipt) {
		normalized := strings.TrimSpace(key)
		if normalized == "" {
			continue
		}
		predicted["pointer:"+normalized] = struct{}{}
	}
	for _, subject := range receiptMemorySubjects(result.Receipt) {
		normalized := strings.TrimSpace(subject)
		if normalized == "" {
			continue
		}
		predicted["memory:"+normalized] = struct{}{}
	}
	return predicted
}

func receiptPointerKeys(receipt *v1.ContextReceipt) []string {
	payload := receiptJSONMap(receipt)
	if len(payload) == 0 {
		return nil
	}

	keys := make(map[string]struct{})
	collectEntryValues(payload, "pointers", []string{"key"}, keys)
	collectEntryValues(payload, "rules", []string{"key"}, keys)
	collectEntryValues(payload, "suggestions", []string{"key"}, keys)
	return mapKeysSorted(keys)
}

func receiptMemorySubjects(receipt *v1.ContextReceipt) []string {
	payload := receiptJSONMap(receipt)
	if len(payload) == 0 {
		return nil
	}

	subjects := make(map[string]struct{})
	collectEntryValues(payload, "memories", []string{"subject", "summary"}, subjects)
	return mapKeysSorted(subjects)
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

func collectEntryValues(payload map[string]any, key string, fieldNames []string, dest map[string]struct{}) {
	if len(fieldNames) == 0 {
		return
	}
	entries, ok := payload[key].([]any)
	if !ok {
		return
	}
	for _, rawEntry := range entries {
		entry, ok := rawEntry.(map[string]any)
		if !ok {
			continue
		}

		for _, fieldName := range fieldNames {
			value, ok := entry[fieldName]
			if !ok {
				continue
			}
			stringValue, ok := value.(string)
			if !ok {
				continue
			}
			normalized := strings.TrimSpace(stringValue)
			if normalized == "" {
				continue
			}
			dest[normalized] = struct{}{}
			break
		}
	}
}

func confusionCounts(expected, predicted map[string]struct{}) (int, int, int) {
	tp := 0
	fp := 0
	fn := 0

	for artifact := range predicted {
		if _, ok := expected[artifact]; ok {
			tp++
			continue
		}
		fp++
	}
	for artifact := range expected {
		if _, ok := predicted[artifact]; ok {
			continue
		}
		fn++
	}
	return tp, fp, fn
}

func metricsFromCounts(tp, fp, fn int) (float64, float64, float64) {
	if tp == 0 && fp == 0 && fn == 0 {
		return 1, 1, 1
	}

	precision := 1.0
	if tp+fp > 0 {
		precision = float64(tp) / float64(tp+fp)
	}
	recall := 1.0
	if tp+fn > 0 {
		recall = float64(tp) / float64(tp+fn)
	}

	f1 := 0.0
	if precision+recall > 0 {
		f1 = (2 * precision * recall) / (precision + recall)
	}

	return roundMetric(precision), roundMetric(recall), roundMetric(f1)
}

func roundMetric(value float64) float64 {
	return math.Round(value*1_000_000) / 1_000_000
}

func regressCaseNote(status string) string {
	status = strings.TrimSpace(status)
	switch status {
	case "ok":
		return ""
	case "insufficient_context":
		return "insufficient_context"
	case "":
		return "status:unknown"
	default:
		return "status:" + status
	}
}

func effectiveMinimumRecall(minimumRecall *float64) float64 {
	if minimumRecall == nil {
		return defaultMinimumRecall
	}
	return roundMetric(*minimumRecall)
}

func normalizeBootstrapProjectRoot(projectRoot string) string {
	trimmed := strings.TrimSpace(projectRoot)
	if trimmed == "" {
		return defaultSyncProjectDir
	}
	absRoot, err := filepath.Abs(trimmed)
	if err != nil {
		return filepath.Clean(trimmed)
	}
	return filepath.Clean(absRoot)
}

func resolveBootstrapOutputPath(projectRoot string, outputPath *string) string {
	effectiveOutput := defaultBootstrapOutputPath
	if outputPath != nil && strings.TrimSpace(*outputPath) != "" {
		effectiveOutput = strings.TrimSpace(*outputPath)
	}
	if filepath.IsAbs(effectiveOutput) {
		return filepath.Clean(effectiveOutput)
	}
	return filepath.Clean(filepath.Join(projectRoot, effectiveOutput))
}

func effectiveRespectGitIgnore(respectGitIgnore *bool) bool {
	if respectGitIgnore == nil {
		return defaultBootstrapRespectGit
	}
	return *respectGitIgnore
}

func effectiveLLMAssistDescriptions(llmAssistDescriptions *bool) bool {
	if llmAssistDescriptions == nil {
		return defaultBootstrapLLMAssist
	}
	return *llmAssistDescriptions
}

func (s *Service) collectBootstrapPaths(ctx context.Context, projectRoot, outputPath string, respectGitIgnore bool) ([]string, []string, error) {
	excludedPath := bootstrapOutputRelativePath(projectRoot, outputPath)
	warnings := make([]string, 0)

	if respectGitIgnore {
		gitOutput, err := s.runGit(ctx, projectRoot, "ls-files", "--cached", "--others", "--exclude-standard")
		if err == nil {
			return filterBootstrapPaths(parseBootstrapGitPaths(gitOutput), excludedPath), warnings, nil
		}
		warnings = append(warnings, "respect_gitignore fallback to filesystem walk")
	}

	paths, walkWarnings, err := collectBootstrapPathsFromWalk(ctx, projectRoot)
	if err != nil {
		return nil, nil, err
	}
	warnings = append(warnings, walkWarnings...)
	return filterBootstrapPaths(paths, excludedPath), warnings, nil
}

func parseBootstrapGitPaths(output string) []string {
	lines := strings.Split(output, "\n")
	paths := make([]string, 0, len(lines))
	for _, line := range lines {
		normalized := normalizeCompletionPath(line)
		if normalized == "" {
			continue
		}
		paths = append(paths, normalized)
	}
	return normalizeCompletionPaths(paths)
}

func collectBootstrapPathsFromWalk(ctx context.Context, projectRoot string) ([]string, []string, error) {
	paths := make([]string, 0)
	warnings := make([]string, 0)

	err := filepath.WalkDir(projectRoot, func(current string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			if len(warnings) < maxBootstrapWalkErrorSamples {
				warnings = append(warnings, fmt.Sprintf("skip:%s", normalizeWalkWarningPath(projectRoot, current)))
			}
			return nil
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		if entry.IsDir() {
			if entry.Name() == ".git" {
				return filepath.SkipDir
			}
			return nil
		}
		if !entry.Type().IsRegular() {
			return nil
		}

		relative, relErr := filepath.Rel(projectRoot, current)
		if relErr != nil {
			if len(warnings) < maxBootstrapWalkErrorSamples {
				warnings = append(warnings, fmt.Sprintf("skip:%s", normalizeWalkWarningPath(projectRoot, current)))
			}
			return nil
		}
		normalized := normalizeCompletionPath(relative)
		if normalized == "" {
			return nil
		}
		paths = append(paths, normalized)
		return nil
	})
	if err != nil {
		return nil, nil, err
	}

	return normalizeCompletionPaths(paths), normalizeValues(warnings), nil
}

func normalizeWalkWarningPath(projectRoot, candidatePath string) string {
	relative, err := filepath.Rel(projectRoot, candidatePath)
	if err == nil {
		if normalized := normalizeCompletionPath(relative); normalized != "" {
			return normalized
		}
	}
	cleaned := normalizeCompletionPath(candidatePath)
	if cleaned != "" {
		return cleaned
	}
	return strings.TrimSpace(candidatePath)
}

func bootstrapOutputRelativePath(projectRoot, outputPath string) string {
	relative, err := filepath.Rel(projectRoot, outputPath)
	if err != nil {
		return ""
	}
	normalized := normalizeCompletionPath(relative)
	if normalized == "" || normalized == "." || strings.HasPrefix(normalized, "../") {
		return ""
	}
	return normalized
}

func filterBootstrapPaths(paths []string, excludedPath string) []string {
	if excludedPath == "" {
		return append([]string(nil), paths...)
	}
	filtered := make([]string, 0, len(paths))
	for _, candidatePath := range paths {
		if candidatePath == excludedPath {
			continue
		}
		filtered = append(filtered, candidatePath)
	}
	return filtered
}

func writeBootstrapCandidates(outputPath string, paths []string) error {
	payload := struct {
		Candidates []string `json:"candidates"`
	}{
		Candidates: append([]string(nil), paths...),
	}

	blob, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal candidates: %w", err)
	}
	blob = append(blob, '\n')

	if err := os.MkdirAll(filepath.Dir(outputPath), 0o755); err != nil {
		return fmt.Errorf("create output directory: %w", err)
	}
	if err := os.WriteFile(outputPath, blob, 0o644); err != nil {
		return fmt.Errorf("write candidate output: %w", err)
	}
	return nil
}

func healthCheckInternalError(operation string, err error) *core.APIError {
	return core.NewError(
		"INTERNAL_ERROR",
		"failed to run health check",
		map[string]any{
			"operation": operation,
			"error":     err.Error(),
		},
	)
}

func regressInternalError(operation string, err error) *core.APIError {
	return core.NewError(
		"INTERNAL_ERROR",
		"failed to run regress suite",
		map[string]any{
			"operation": operation,
			"error":     err.Error(),
		},
	)
}

func bootstrapInternalError(operation string, err error) *core.APIError {
	return core.NewError(
		"INTERNAL_ERROR",
		"failed to bootstrap candidates",
		map[string]any{
			"operation": operation,
			"error":     err.Error(),
		},
	)
}

func fetchInternalError(operation string, err error) *core.APIError {
	return core.NewError(
		"INTERNAL_ERROR",
		"failed to fetch context",
		map[string]any{
			"operation": operation,
			"error":     err.Error(),
		},
	)
}

func workInternalError(operation string, err error) *core.APIError {
	return core.NewError(
		"INTERNAL_ERROR",
		"failed to persist work state",
		map[string]any{
			"operation": operation,
			"error":     err.Error(),
		},
	)
}

func reportCompletionInternalError(operation string, err error) *core.APIError {
	return core.NewError(
		"INTERNAL_ERROR",
		"failed to report completion",
		map[string]any{
			"operation": operation,
			"error":     err.Error(),
		},
	)
}

func normalizeProposedMemory(memory v1.MemoryPayload) v1.MemoryPayload {
	return v1.MemoryPayload{
		Category:            v1.MemoryCategory(strings.TrimSpace(string(memory.Category))),
		Subject:             strings.TrimSpace(memory.Subject),
		Content:             strings.TrimSpace(memory.Content),
		RelatedPointerKeys:  normalizeValues(memory.RelatedPointerKeys),
		Tags:                normalizeCanonicalTags(memory.Tags),
		Confidence:          memory.Confidence,
		EvidencePointerKeys: normalizeValues(memory.EvidencePointerKeys),
	}
}

func validateProposedMemoryScope(memory v1.MemoryPayload, receiptPointerKeys []string) v1.ProposeMemoryValidation {
	pointerScope := make(map[string]struct{}, len(receiptPointerKeys))
	for _, key := range normalizeValues(receiptPointerKeys) {
		pointerScope[key] = struct{}{}
	}

	errorsList := make([]string, 0, 2)
	warnings := make([]string, 0, 1)

	if len(memory.EvidencePointerKeys) == 0 {
		errorsList = append(errorsList, "memory.evidence_pointer_keys must not be empty after normalization")
	} else if missingEvidence := pointerKeysOutsideScope(memory.EvidencePointerKeys, pointerScope); len(missingEvidence) > 0 {
		errorsList = append(errorsList, "memory.evidence_pointer_keys outside receipt scope: "+strings.Join(missingEvidence, ", "))
	}

	if missingRelated := pointerKeysOutsideScope(memory.RelatedPointerKeys, pointerScope); len(missingRelated) > 0 {
		warnings = append(warnings, "memory.related_pointer_keys outside receipt scope: "+strings.Join(missingRelated, ", "))
	}

	return v1.ProposeMemoryValidation{
		HardPassed: len(errorsList) == 0,
		SoftPassed: len(warnings) == 0,
		Errors:     nonNilStrings(errorsList),
		Warnings:   nonNilStrings(warnings),
	}
}

func pointerKeysOutsideScope(pointerKeys []string, scope map[string]struct{}) []string {
	if len(pointerKeys) == 0 {
		return nil
	}

	out := make([]string, 0, len(pointerKeys))
	for _, key := range pointerKeys {
		if _, ok := scope[key]; ok {
			continue
		}
		out = append(out, key)
	}
	return out
}

func normalizeValues(values []string) []string {
	if len(values) == 0 {
		return nil
	}

	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))
	for _, raw := range values {
		trimmed := strings.TrimSpace(raw)
		if trimmed == "" {
			continue
		}
		if _, ok := seen[trimmed]; ok {
			continue
		}
		seen[trimmed] = struct{}{}
		out = append(out, trimmed)
	}
	if len(out) == 0 {
		return nil
	}
	sort.Strings(out)
	return out
}

func deterministicMemoryDedupeKey(memory v1.MemoryPayload) string {
	var b strings.Builder
	b.WriteString(strings.TrimSpace(string(memory.Category)))
	b.WriteString("\n")
	b.WriteString(memory.Subject)
	b.WriteString("\n")
	b.WriteString(memory.Content)
	b.WriteString("\n")
	b.WriteString(fmt.Sprintf("%d\n", memory.Confidence))

	for _, tag := range memory.Tags {
		b.WriteString("tag:")
		b.WriteString(tag)
		b.WriteString("\n")
	}
	for _, related := range memory.RelatedPointerKeys {
		b.WriteString("rel:")
		b.WriteString(related)
		b.WriteString("\n")
	}
	for _, evidence := range memory.EvidencePointerKeys {
		b.WriteString("evidence:")
		b.WriteString(evidence)
		b.WriteString("\n")
	}

	digest := sha256.Sum256([]byte(b.String()))
	return hex.EncodeToString(digest[:])
}

func nonNilStrings(values []string) []string {
	if len(values) == 0 {
		return []string{}
	}
	return values
}

func effectiveAutoPromote(autoPromote *bool) bool {
	if autoPromote == nil {
		return true
	}
	return *autoPromote
}

func proposeMemoryInternalError(operation string, err error) *core.APIError {
	return core.NewError(
		"INTERNAL_ERROR",
		"failed to propose memory",
		map[string]any{
			"operation": operation,
			"error":     err.Error(),
		},
	)
}

func notImplemented(op string) *core.APIError {
	return core.NewError(
		"NOT_IMPLEMENTED",
		"service backend for operation is not wired yet",
		map[string]any{"operation": op},
	)
}

func (s *Service) Coverage(ctx context.Context, payload v1.CoveragePayload) (v1.CoverageResult, *core.APIError) {
	if s == nil || s.repo == nil {
		return v1.CoverageResult{}, core.NewError("INTERNAL_ERROR", "postgres service repository is not configured", nil)
	}

	projectRoot := normalizeSyncProjectRoot(payload.ProjectRoot)
	paths, err := s.collectCoveragePaths(ctx, projectRoot)
	if err != nil {
		return v1.CoverageResult{}, coverageInternalError("collect_project_paths", err)
	}

	inventory, err := s.repo.ListPointerInventory(ctx, strings.TrimSpace(payload.ProjectID))
	if err != nil {
		return v1.CoverageResult{}, coverageInternalError("list_pointer_inventory", err)
	}

	pointerByPath := make(map[string]core.PointerInventory, len(inventory))
	for _, item := range inventory {
		normalizedPath := normalizeCompletionPath(item.Path)
		if normalizedPath == "" {
			continue
		}
		current, exists := pointerByPath[normalizedPath]
		if !exists {
			pointerByPath[normalizedPath] = core.PointerInventory{Path: normalizedPath, IsStale: item.IsStale}
			continue
		}
		current.IsStale = current.IsStale || item.IsStale
		pointerByPath[normalizedPath] = current
	}

	indexedCount := 0
	unindexed := make([]string, 0)
	for _, filePath := range paths {
		if _, ok := pointerByPath[filePath]; ok {
			indexedCount++
			continue
		}
		unindexed = append(unindexed, filePath)
	}

	stale := make([]string, 0)
	for filePath, item := range pointerByPath {
		if !item.IsStale {
			continue
		}
		stale = append(stale, filePath)
	}
	sort.Strings(stale)

	zeroCoverageDirs := zeroCoverageDirectories(paths, pointerByPath)

	totalFiles := len(paths)
	coveragePercent := 100.0
	if totalFiles > 0 {
		coveragePercent = math.Round((float64(indexedCount)/float64(totalFiles)*100.0)*100.0) / 100.0
	}

	return v1.CoverageResult{
		Summary: v1.CoverageSummary{
			TotalFiles:      totalFiles,
			IndexedFiles:    indexedCount,
			UnindexedFiles:  len(unindexed),
			StaleFiles:      len(stale),
			CoveragePercent: coveragePercent,
		},
		UnindexedPaths:   normalizeValues(unindexed),
		StalePaths:       normalizeValues(stale),
		ZeroCoverageDirs: zeroCoverageDirs,
	}, nil
}

func (s *Service) collectCoveragePaths(ctx context.Context, projectRoot string) ([]string, error) {
	gitOutput, err := s.runGit(ctx, projectRoot, "ls-files", "--cached", "--others", "--exclude-standard")
	if err == nil {
		return parseBootstrapGitPaths(gitOutput), nil
	}

	paths, _, walkErr := collectBootstrapPathsFromWalk(ctx, projectRoot)
	if walkErr != nil {
		return nil, walkErr
	}
	return paths, nil
}

func zeroCoverageDirectories(paths []string, pointerByPath map[string]core.PointerInventory) []string {
	if len(paths) == 0 {
		return nil
	}

	dirTotal := make(map[string]int)
	dirCovered := make(map[string]int)
	for _, filePath := range paths {
		dir := path.Dir(filePath)
		if dir == "." || dir == "" {
			continue
		}
		dirTotal[dir]++
		if _, ok := pointerByPath[filePath]; ok {
			dirCovered[dir]++
		}
	}

	out := make([]string, 0)
	for dir, total := range dirTotal {
		if total <= 0 {
			continue
		}
		if dirCovered[dir] > 0 {
			continue
		}
		out = append(out, dir)
	}
	sort.Strings(out)
	return out
}

func effectiveScopeMode(mode v1.ScopeMode) v1.ScopeMode {
	switch mode {
	case v1.ScopeModeAutoIndex:
		return v1.ScopeModeAutoIndex
	case v1.ScopeModeStrict:
		return v1.ScopeModeStrict
	case v1.ScopeModeWarn:
		return v1.ScopeModeWarn
	default:
		return v1.ScopeModeWarn
	}
}

func coverageInternalError(operation string, err error) *core.APIError {
	return core.NewError(
		"INTERNAL_ERROR",
		"failed to compute coverage report",
		map[string]any{
			"operation": operation,
			"error":     err.Error(),
		},
	)
}

func buildAutoIndexPointerStubs(projectID string, violations []v1.CompletionViolation) []core.PointerStub {
	projectID = strings.TrimSpace(projectID)
	seenPath := make(map[string]struct{}, len(violations))
	stubs := make([]core.PointerStub, 0, len(violations))
	for _, violation := range violations {
		normalizedPath := normalizeCompletionPath(violation.Path)
		if normalizedPath == "" {
			continue
		}
		if _, exists := seenPath[normalizedPath]; exists {
			continue
		}
		seenPath[normalizedPath] = struct{}{}

		kind := inferPointerKindFromPath(normalizedPath)
		label := fmt.Sprintf("Auto-indexed: %s", path.Base(normalizedPath))
		stubs = append(stubs, core.PointerStub{
			PointerKey:  fmt.Sprintf("%s:%s", projectID, normalizedPath),
			Path:        normalizedPath,
			Kind:        kind,
			Label:       label,
			Description: "Auto-indexed pointer stub created by scope gate. Curate label, description, and tags.",
			Tags:        inferPointerTagsFromPath(normalizedPath, kind),
		})
	}
	return stubs
}

func inferPointerKindFromPath(filePath string) string {
	pathValue := strings.ToLower(strings.TrimSpace(filePath))
	switch {
	case strings.Contains(pathValue, "/test/"),
		strings.Contains(pathValue, "/tests/"),
		strings.HasSuffix(pathValue, "_test.go"),
		strings.HasSuffix(pathValue, ".test.ts"),
		strings.HasSuffix(pathValue, ".test.tsx"),
		strings.HasSuffix(pathValue, ".spec.ts"),
		strings.HasSuffix(pathValue, ".spec.tsx"),
		strings.HasSuffix(pathValue, ".spec.js"),
		strings.HasSuffix(pathValue, ".spec.jsx"):
		return "test"
	case strings.HasPrefix(pathValue, "docs/"),
		strings.HasSuffix(pathValue, ".md"),
		strings.HasSuffix(pathValue, ".mdx"),
		strings.HasSuffix(pathValue, ".rst"),
		strings.HasSuffix(pathValue, ".adoc"):
		return "doc"
	case strings.HasPrefix(pathValue, "scripts/"),
		strings.HasSuffix(pathValue, ".sh"),
		strings.HasSuffix(pathValue, ".bash"),
		strings.HasSuffix(pathValue, ".ps1"),
		strings.HasSuffix(pathValue, ".bat"):
		return "command"
	default:
		return "code"
	}
}

func inferPointerTagsFromPath(filePath, kind string) []string {
	tags := []string{"auto-indexed", kind}
	baseName := strings.TrimSuffix(path.Base(filePath), path.Ext(filePath))
	if normalized := normalizeCanonicalTag(baseName); healthTagPattern.MatchString(normalized) {
		tags = append(tags, normalized)
	}
	segments := strings.Split(path.Dir(filePath), "/")
	for _, segment := range segments {
		normalized := normalizeCanonicalTag(segment)
		if !healthTagPattern.MatchString(normalized) {
			continue
		}
		tags = append(tags, normalized)
	}
	return normalizeCanonicalTags(tags)
}
