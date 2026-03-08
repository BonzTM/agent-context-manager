package domain

import (
	"fmt"
	"path"
	"sort"
	"strings"
	"time"

	"github.com/bonztm/agent-context-manager/internal/core"
)

const (
	WorkPlanListScopeCurrent   = "current"
	WorkPlanListScopeDeferred  = "deferred"
	WorkPlanListScopeCompleted = "completed"
	WorkPlanListScopeAll       = "all"
	DefaultPhase               = "execute"
)

type NormalizedRunSummary struct {
	ProjectID              string
	RequestID              string
	ReceiptID              string
	TaskText               string
	Phase                  string
	Status                 string
	ResolvedTags           []string
	PointerKeys            []string
	MemoryIDs              []int64
	FilesChanged           []string
	DefinitionOfDoneIssues []string
	Outcome                string
}

type NormalizedVerificationBatch struct {
	BatchRunID      string
	ProjectID       string
	ReceiptID       string
	PlanKey         string
	Phase           string
	TestsSourcePath string
	Status          string
	Passed          bool
	SelectedTestIDs []string
	Results         []core.VerificationTestRun
	CreatedAt       time.Time
}

type NormalizedReviewAttempt struct {
	ProjectID          string
	ReceiptID          string
	PlanKey            string
	ReviewKey          string
	Summary            string
	Fingerprint        string
	Status             string
	Passed             bool
	Outcome            string
	WorkflowSourcePath string
	CommandArgv        []string
	CommandCWD         string
	TimeoutSec         int
	ExitCode           *int
	TimedOut           bool
	StdoutExcerpt      string
	StderrExcerpt      string
	CreatedAt          time.Time
}

func NormalizeWorkPlanMode(raw core.WorkPlanMode) core.WorkPlanMode {
	switch strings.TrimSpace(string(raw)) {
	case string(core.WorkPlanModeReplace):
		return core.WorkPlanModeReplace
	default:
		return core.WorkPlanModeMerge
	}
}

func NormalizePhase(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "plan":
		return "plan"
	case "review":
		return "review"
	case "execute":
		return "execute"
	default:
		return DefaultPhase
	}
}

func RankCandidatePointers(candidates []core.CandidatePointer, input core.CandidatePointerQuery, defaultLimit int) []core.CandidatePointer {
	if len(candidates) == 0 {
		return nil
	}

	phase := NormalizePhase(input.Phase)
	limit := input.Limit
	if limit <= 0 {
		limit = defaultLimit
	}
	taskTokens := TokenizeCandidateQuery(input.TaskText)
	queryTags := NormalizeStringList(input.Tags)

	ranked := make([]core.CandidatePointer, 0, len(candidates))
	for _, candidate := range candidates {
		textRank := CandidateTextMatchRank(taskTokens, candidate)
		tagOverlap := CandidateTagOverlap(candidate.Tags, queryTags)
		if len(taskTokens) > 0 || len(queryTags) > 0 {
			if textRank == 0 && tagOverlap == 0 {
				continue
			}
		}

		candidate.Rank = ((float64(tagOverlap) * 10.0) + (float64(textRank) * 5.0)) * CandidatePhaseWeight(phase, candidate)
		ranked = append(ranked, candidate)
	}

	sort.Slice(ranked, func(i, j int) bool {
		if ranked[i].IsRule != ranked[j].IsRule {
			return ranked[i].IsRule
		}
		if ranked[i].Rank != ranked[j].Rank {
			return ranked[i].Rank > ranked[j].Rank
		}
		return ranked[i].Key < ranked[j].Key
	})
	if !input.Unbounded && len(ranked) > limit {
		ranked = ranked[:limit]
	}
	return ranked
}

func CandidateTextMatchRank(tokens []string, pointer core.CandidatePointer) int {
	if len(tokens) == 0 {
		return 0
	}
	searchSpace := strings.ToLower(strings.Join([]string{
		pointer.Label,
		pointer.Description,
		strings.Join(pointer.Tags, " "),
	}, " "))
	score := 0
	for _, token := range tokens {
		if strings.Contains(searchSpace, token) {
			score++
		}
	}
	return score
}

func CandidatePhaseWeight(phase string, pointer core.CandidatePointer) float64 {
	switch phase {
	case "plan":
		switch CandidatePointerKind(pointer) {
		case "rule":
			return 3
		case "doc":
			return 2
		default:
			return 1
		}
	case "execute":
		switch CandidatePointerKind(pointer) {
		case "code":
			return 3
		case "test":
			return 2
		case "rule":
			return 1
		default:
			return 1
		}
	case "review":
		switch CandidatePointerKind(pointer) {
		case "rule":
			return 3
		case "test":
			return 2
		case "code":
			return 1
		default:
			return 1
		}
	default:
		return 1
	}
}

func CandidatePointerKind(pointer core.CandidatePointer) string {
	if pointer.IsRule {
		return "rule"
	}
	kind := strings.ToLower(strings.TrimSpace(pointer.Kind))
	switch kind {
	case "doc", "docs", "documentation":
		return "doc"
	case "test", "tests":
		return "test"
	}
	if strings.Contains(strings.ToLower(pointer.Path), "_test.") {
		return "test"
	}
	return "code"
}

func TokenizeCandidateQuery(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	fields := strings.FieldsFunc(strings.ToLower(raw), func(r rune) bool {
		return !(r == '.' || r == '_' || r == '-' || r == '/' || r == ':' || (r >= '0' && r <= '9') || (r >= 'a' && r <= 'z'))
	})
	return NormalizeStringList(fields)
}

func CandidateTagOverlap(values, targets []string) int {
	if len(values) == 0 || len(targets) == 0 {
		return 0
	}
	targetSet := make(map[string]struct{}, len(targets))
	for _, value := range targets {
		targetSet[value] = struct{}{}
	}
	count := 0
	seen := map[string]struct{}{}
	for _, value := range values {
		if _, ok := targetSet[value]; !ok {
			continue
		}
		if _, dupe := seen[value]; dupe {
			continue
		}
		seen[value] = struct{}{}
		count++
	}
	return count
}

func NormalizeWorkPlanListScope(raw string) string {
	switch strings.TrimSpace(raw) {
	case WorkPlanListScopeCurrent:
		return WorkPlanListScopeCurrent
	case WorkPlanListScopeDeferred:
		return WorkPlanListScopeDeferred
	case WorkPlanListScopeCompleted:
		return WorkPlanListScopeCompleted
	default:
		return WorkPlanListScopeAll
	}
}

func WorkPlanListSearchPattern(raw string) string {
	trimmed := strings.ToLower(strings.TrimSpace(raw))
	if trimmed == "" {
		return ""
	}
	replacer := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)
	return "%" + replacer.Replace(trimmed) + "%"
}

func NormalizeWorkPlanStages(raw core.WorkPlanStages) core.WorkPlanStages {
	out := core.WorkPlanStages{
		SpecOutline:        NormalizeWorkItemStatus(raw.SpecOutline),
		RefinedSpec:        NormalizeWorkItemStatus(raw.RefinedSpec),
		ImplementationPlan: NormalizeWorkItemStatus(raw.ImplementationPlan),
	}
	if strings.TrimSpace(raw.SpecOutline) == "" {
		out.SpecOutline = core.WorkItemStatusPending
	}
	if strings.TrimSpace(raw.RefinedSpec) == "" {
		out.RefinedSpec = core.WorkItemStatusPending
	}
	if strings.TrimSpace(raw.ImplementationPlan) == "" {
		out.ImplementationPlan = core.WorkItemStatusPending
	}
	return out
}

func NormalizeWorkPlanTasks(tasks []core.WorkItem) []core.WorkItem {
	if len(tasks) == 0 {
		return nil
	}

	priority := map[string]int{
		core.WorkItemStatusComplete:   0,
		core.WorkItemStatusCompleted:  0,
		core.WorkItemStatusPending:    1,
		core.WorkItemStatusInProgress: 2,
		core.WorkItemStatusBlocked:    3,
	}
	byKey := make(map[string]core.WorkItem, len(tasks))
	for _, raw := range tasks {
		itemKey := strings.TrimSpace(raw.ItemKey)
		if itemKey == "" {
			continue
		}
		status := NormalizeWorkItemStatus(raw.Status)
		normalized := core.WorkItem{
			ItemKey:            itemKey,
			Summary:            strings.TrimSpace(raw.Summary),
			Status:             status,
			ParentTaskKey:      strings.TrimSpace(raw.ParentTaskKey),
			DependsOn:          NormalizeStringList(raw.DependsOn),
			AcceptanceCriteria: NormalizeStringList(raw.AcceptanceCriteria),
			References:         NormalizeStringList(raw.References),
			ExternalRefs:       NormalizeStringList(raw.ExternalRefs),
			BlockedReason:      strings.TrimSpace(raw.BlockedReason),
			Outcome:            strings.TrimSpace(raw.Outcome),
			Evidence:           NormalizeStringList(raw.Evidence),
			UpdatedAt:          raw.UpdatedAt.UTC(),
		}
		current, exists := byKey[itemKey]
		if !exists || priority[status] >= priority[current.Status] {
			byKey[itemKey] = normalized
		}
	}
	if len(byKey) == 0 {
		return nil
	}
	keys := make([]string, 0, len(byKey))
	for key := range byKey {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	out := make([]core.WorkItem, 0, len(keys))
	for _, key := range keys {
		out = append(out, byKey[key])
	}
	return out
}

func BuildNextWorkPlanState(current core.WorkPlan, found bool, input core.WorkPlanUpsertInput, mode core.WorkPlanMode) core.WorkPlan {
	projectID := strings.TrimSpace(input.ProjectID)
	planKey := strings.TrimSpace(input.PlanKey)
	next := core.WorkPlan{
		ProjectID: projectID,
		PlanKey:   planKey,
		Status:    core.PlanStatusPending,
		Stages: core.WorkPlanStages{
			SpecOutline:        core.PlanStatusPending,
			RefinedSpec:        core.PlanStatusPending,
			ImplementationPlan: core.PlanStatusPending,
		},
	}
	if found {
		next = current
	}
	if mode == core.WorkPlanModeReplace {
		next = core.WorkPlan{
			ProjectID: projectID,
			PlanKey:   planKey,
			Status:    core.PlanStatusPending,
			Stages: core.WorkPlanStages{
				SpecOutline:        core.PlanStatusPending,
				RefinedSpec:        core.PlanStatusPending,
				ImplementationPlan: core.PlanStatusPending,
			},
		}
	}

	trimmedReceipt := strings.TrimSpace(input.ReceiptID)
	if trimmedReceipt != "" || mode == core.WorkPlanModeReplace {
		next.ReceiptID = trimmedReceipt
	}

	trimmedTitle := strings.TrimSpace(input.Title)
	if trimmedTitle != "" || mode == core.WorkPlanModeReplace {
		next.Title = trimmedTitle
	}

	trimmedObjective := strings.TrimSpace(input.Objective)
	if trimmedObjective != "" || mode == core.WorkPlanModeReplace {
		next.Objective = trimmedObjective
	}

	trimmedKind := strings.TrimSpace(input.Kind)
	if trimmedKind != "" || mode == core.WorkPlanModeReplace {
		next.Kind = strings.ToLower(trimmedKind)
	}

	trimmedParentPlanKey := strings.TrimSpace(input.ParentPlanKey)
	if trimmedParentPlanKey != "" || mode == core.WorkPlanModeReplace {
		next.ParentPlanKey = trimmedParentPlanKey
	}

	trimmedStatus := strings.TrimSpace(input.Status)
	if trimmedStatus != "" {
		next.Status = NormalizeWorkItemStatus(trimmedStatus)
	}
	if mode == core.WorkPlanModeReplace && trimmedStatus == "" {
		next.Status = core.PlanStatusPending
	}

	if input.Stages.SpecOutline != "" || input.Stages.RefinedSpec != "" || input.Stages.ImplementationPlan != "" || mode == core.WorkPlanModeReplace {
		if mode == core.WorkPlanModeReplace {
			next.Stages = core.WorkPlanStages{}
		}
		next.Stages = MergeWorkPlanStages(next.Stages, input.Stages, mode)
	}
	next.Stages = NormalizeWorkPlanStages(next.Stages)

	if input.InScope != nil || mode == core.WorkPlanModeReplace {
		next.InScope = NormalizeStringList(input.InScope)
	}
	if input.OutOfScope != nil || mode == core.WorkPlanModeReplace {
		next.OutOfScope = NormalizeStringList(input.OutOfScope)
	}
	if input.Constraints != nil || mode == core.WorkPlanModeReplace {
		next.Constraints = NormalizeStringList(input.Constraints)
	}
	if input.References != nil || mode == core.WorkPlanModeReplace {
		next.References = NormalizeStringList(input.References)
	}
	if input.ExternalRefs != nil || mode == core.WorkPlanModeReplace {
		next.ExternalRefs = NormalizeStringList(input.ExternalRefs)
	}

	next.ProjectID = projectID
	next.PlanKey = planKey
	next.Status = NormalizeWorkItemStatus(next.Status)
	return next
}

func MergeWorkPlanStages(current, incoming core.WorkPlanStages, mode core.WorkPlanMode) core.WorkPlanStages {
	out := current
	if mode == core.WorkPlanModeReplace {
		out = core.WorkPlanStages{}
	}
	if strings.TrimSpace(incoming.SpecOutline) != "" || mode == core.WorkPlanModeReplace {
		out.SpecOutline = NormalizeWorkItemStatus(incoming.SpecOutline)
	}
	if strings.TrimSpace(incoming.RefinedSpec) != "" || mode == core.WorkPlanModeReplace {
		out.RefinedSpec = NormalizeWorkItemStatus(incoming.RefinedSpec)
	}
	if strings.TrimSpace(incoming.ImplementationPlan) != "" || mode == core.WorkPlanModeReplace {
		out.ImplementationPlan = NormalizeWorkItemStatus(incoming.ImplementationPlan)
	}
	return out
}

func NormalizeRunReceiptSummary(input core.RunReceiptSummary) (NormalizedRunSummary, error) {
	projectID := strings.TrimSpace(input.ProjectID)
	if projectID == "" {
		return NormalizedRunSummary{}, fmt.Errorf("project_id is required")
	}
	status := strings.TrimSpace(input.Status)
	if status == "" {
		status = "accepted"
	}
	phase := strings.TrimSpace(input.Phase)
	if phase == "" {
		phase = "execute"
	}
	return NormalizedRunSummary{
		ProjectID:              projectID,
		RequestID:              strings.TrimSpace(input.RequestID),
		ReceiptID:              strings.TrimSpace(input.ReceiptID),
		TaskText:               strings.TrimSpace(input.TaskText),
		Phase:                  phase,
		Status:                 status,
		ResolvedTags:           NormalizeStringList(input.ResolvedTags),
		PointerKeys:            NormalizeStringList(input.PointerKeys),
		MemoryIDs:              NormalizeInt64List(input.MemoryIDs),
		FilesChanged:           NormalizeStringList(input.FilesChanged),
		DefinitionOfDoneIssues: NormalizeStringList(input.DefinitionOfDoneIssues),
		Outcome:                strings.TrimSpace(input.Outcome),
	}, nil
}

func NormalizeReceiptScope(input core.ReceiptScope) (NormalizedRunSummary, error) {
	projectID := strings.TrimSpace(input.ProjectID)
	if projectID == "" {
		return NormalizedRunSummary{}, fmt.Errorf("project_id is required")
	}
	receiptID := strings.TrimSpace(input.ReceiptID)
	if receiptID == "" {
		return NormalizedRunSummary{}, fmt.Errorf("receipt_id is required")
	}
	phase := strings.TrimSpace(input.Phase)
	if phase == "" {
		phase = "execute"
	}
	return NormalizedRunSummary{
		ProjectID:    projectID,
		ReceiptID:    receiptID,
		TaskText:     strings.TrimSpace(input.TaskText),
		Phase:        phase,
		ResolvedTags: NormalizeStringList(input.ResolvedTags),
		PointerKeys:  NormalizeStringList(input.PointerKeys),
		MemoryIDs:    NormalizeInt64List(input.MemoryIDs),
	}, nil
}

func NormalizeVerificationBatch(input core.VerificationBatch) (NormalizedVerificationBatch, error) {
	projectID := strings.TrimSpace(input.ProjectID)
	if projectID == "" {
		return NormalizedVerificationBatch{}, fmt.Errorf("project_id is required")
	}
	batchRunID := strings.TrimSpace(input.BatchRunID)
	if batchRunID == "" {
		return NormalizedVerificationBatch{}, fmt.Errorf("batch_run_id is required")
	}
	status := strings.TrimSpace(input.Status)
	if status != "passed" && status != "failed" {
		return NormalizedVerificationBatch{}, fmt.Errorf("status must be passed|failed")
	}

	results := make([]core.VerificationTestRun, 0, len(input.Results))
	for i, raw := range input.Results {
		testID := strings.TrimSpace(raw.TestID)
		if testID == "" {
			return NormalizedVerificationBatch{}, fmt.Errorf("results[%d].test_id is required", i)
		}
		definitionHash := strings.TrimSpace(raw.DefinitionHash)
		if definitionHash == "" {
			return NormalizedVerificationBatch{}, fmt.Errorf("results[%d].definition_hash is required", i)
		}
		resultStatus := strings.TrimSpace(raw.Status)
		switch resultStatus {
		case "passed", "failed", "timed_out", "errored", "skipped":
		default:
			return NormalizedVerificationBatch{}, fmt.Errorf("results[%d].status is invalid", i)
		}
		timeoutSec := raw.TimeoutSec
		if timeoutSec <= 0 {
			return NormalizedVerificationBatch{}, fmt.Errorf("results[%d].timeout_sec must be > 0", i)
		}
		expectedExitCode := raw.ExpectedExitCode
		if expectedExitCode < 0 || expectedExitCode > 255 {
			return NormalizedVerificationBatch{}, fmt.Errorf("results[%d].expected_exit_code must be 0..255", i)
		}
		durationMS := raw.DurationMS
		if durationMS < 0 {
			durationMS = 0
		}
		startedAt := raw.StartedAt.UTC()
		if startedAt.IsZero() {
			startedAt = time.Now().UTC()
		}
		finishedAt := raw.FinishedAt.UTC()
		if finishedAt.IsZero() || finishedAt.Before(startedAt) {
			finishedAt = startedAt
		}
		results = append(results, core.VerificationTestRun{
			BatchRunID:       batchRunID,
			ProjectID:        projectID,
			TestID:           testID,
			DefinitionHash:   definitionHash,
			Summary:          strings.TrimSpace(raw.Summary),
			CommandArgv:      NormalizeStringListPreserveOrder(raw.CommandArgv),
			CommandCWD:       strings.TrimSpace(raw.CommandCWD),
			TimeoutSec:       timeoutSec,
			ExpectedExitCode: expectedExitCode,
			SelectionReasons: NormalizeStringListPreserveOrder(raw.SelectionReasons),
			Status:           resultStatus,
			ExitCode:         raw.ExitCode,
			DurationMS:       durationMS,
			StdoutExcerpt:    strings.TrimSpace(raw.StdoutExcerpt),
			StderrExcerpt:    strings.TrimSpace(raw.StderrExcerpt),
			StartedAt:        startedAt,
			FinishedAt:       finishedAt,
		})
	}

	createdAt := input.CreatedAt.UTC()
	if createdAt.IsZero() {
		createdAt = time.Now().UTC()
	}

	return NormalizedVerificationBatch{
		BatchRunID:      batchRunID,
		ProjectID:       projectID,
		ReceiptID:       strings.TrimSpace(input.ReceiptID),
		PlanKey:         strings.TrimSpace(input.PlanKey),
		Phase:           strings.TrimSpace(input.Phase),
		TestsSourcePath: strings.TrimSpace(input.TestsSourcePath),
		Status:          status,
		Passed:          input.Passed,
		SelectedTestIDs: NormalizeStringListPreserveOrder(input.SelectedTestIDs),
		Results:         results,
		CreatedAt:       createdAt,
	}, nil
}

func NormalizeReviewAttempt(input core.ReviewAttempt) (NormalizedReviewAttempt, error) {
	projectID := strings.TrimSpace(input.ProjectID)
	if projectID == "" {
		return NormalizedReviewAttempt{}, fmt.Errorf("project_id is required")
	}
	receiptID := strings.TrimSpace(input.ReceiptID)
	if receiptID == "" {
		return NormalizedReviewAttempt{}, fmt.Errorf("receipt_id is required")
	}
	reviewKey := strings.TrimSpace(input.ReviewKey)
	if reviewKey == "" {
		return NormalizedReviewAttempt{}, fmt.Errorf("review_key is required")
	}
	fingerprint := strings.TrimSpace(input.Fingerprint)
	if fingerprint == "" {
		return NormalizedReviewAttempt{}, fmt.Errorf("fingerprint is required")
	}
	status := strings.TrimSpace(input.Status)
	if status != "passed" && status != "failed" {
		return NormalizedReviewAttempt{}, fmt.Errorf("status must be passed|failed")
	}
	timeoutSec := input.TimeoutSec
	if timeoutSec <= 0 {
		return NormalizedReviewAttempt{}, fmt.Errorf("timeout_sec must be > 0")
	}
	if input.ExitCode != nil {
		exitCode := *input.ExitCode
		if exitCode < 0 || exitCode > 255 {
			return NormalizedReviewAttempt{}, fmt.Errorf("exit_code must be 0..255 when provided")
		}
	}
	createdAt := input.CreatedAt.UTC()
	if createdAt.IsZero() {
		createdAt = time.Now().UTC()
	}
	return NormalizedReviewAttempt{
		ProjectID:          projectID,
		ReceiptID:          receiptID,
		PlanKey:            strings.TrimSpace(input.PlanKey),
		ReviewKey:          reviewKey,
		Summary:            strings.TrimSpace(input.Summary),
		Fingerprint:        fingerprint,
		Status:             status,
		Passed:             input.Passed,
		Outcome:            strings.TrimSpace(input.Outcome),
		WorkflowSourcePath: strings.TrimSpace(input.WorkflowSourcePath),
		CommandArgv:        NormalizeStringListPreserveOrder(input.CommandArgv),
		CommandCWD:         strings.TrimSpace(input.CommandCWD),
		TimeoutSec:         timeoutSec,
		ExitCode:           input.ExitCode,
		TimedOut:           input.TimedOut,
		StdoutExcerpt:      strings.TrimSpace(input.StdoutExcerpt),
		StderrExcerpt:      strings.TrimSpace(input.StderrExcerpt),
		CreatedAt:          createdAt,
	}, nil
}

func NormalizeProposeMemoryPersistence(input core.ProposeMemoryPersistence) (core.ProposeMemoryPersistence, error) {
	projectID := strings.TrimSpace(input.ProjectID)
	if projectID == "" {
		return core.ProposeMemoryPersistence{}, fmt.Errorf("project_id is required")
	}
	receiptID := strings.TrimSpace(input.ReceiptID)
	if receiptID == "" {
		return core.ProposeMemoryPersistence{}, fmt.Errorf("receipt_id is required")
	}
	category := strings.TrimSpace(input.Category)
	if category == "" {
		return core.ProposeMemoryPersistence{}, fmt.Errorf("category is required")
	}
	subject := strings.TrimSpace(input.Subject)
	if subject == "" {
		return core.ProposeMemoryPersistence{}, fmt.Errorf("subject is required")
	}
	content := strings.TrimSpace(input.Content)
	if content == "" {
		return core.ProposeMemoryPersistence{}, fmt.Errorf("content is required")
	}
	if input.Confidence < 1 || input.Confidence > 5 {
		return core.ProposeMemoryPersistence{}, fmt.Errorf("confidence must be 1..5")
	}
	evidencePointerKeys := NormalizeStringList(input.EvidencePointerKeys)
	if len(evidencePointerKeys) == 0 {
		return core.ProposeMemoryPersistence{}, fmt.Errorf("evidence_pointer_keys is required")
	}
	dedupeKey := strings.TrimSpace(input.DedupeKey)
	if dedupeKey == "" {
		return core.ProposeMemoryPersistence{}, fmt.Errorf("dedupe_key is required")
	}

	return core.ProposeMemoryPersistence{
		ProjectID:           projectID,
		ReceiptID:           receiptID,
		Category:            category,
		Subject:             subject,
		Content:             content,
		Confidence:          input.Confidence,
		Tags:                NormalizeStringList(input.Tags),
		RelatedPointerKeys:  NormalizeStringList(input.RelatedPointerKeys),
		EvidencePointerKeys: evidencePointerKeys,
		DedupeKey:           dedupeKey,
		Validation: core.ProposeMemoryValidation{
			HardPassed: input.Validation.HardPassed,
			SoftPassed: input.Validation.SoftPassed,
			Errors:     NormalizeStringList(input.Validation.Errors),
			Warnings:   NormalizeStringList(input.Validation.Warnings),
		},
		AutoPromote: input.AutoPromote,
		Promotable:  input.Promotable,
	}, nil
}

func NormalizeSyncApplyInput(input core.SyncApplyInput) (core.SyncApplyInput, error) {
	projectID := strings.TrimSpace(input.ProjectID)
	if projectID == "" {
		return core.SyncApplyInput{}, fmt.Errorf("project_id is required")
	}

	mode := strings.TrimSpace(input.Mode)
	if mode == "" {
		mode = "changed"
	}
	if mode != "changed" && mode != "full" && mode != "working_tree" {
		return core.SyncApplyInput{}, fmt.Errorf("mode must be changed|full|working_tree")
	}

	pathByKey := make(map[string]core.SyncPath, len(input.Paths))
	for _, raw := range input.Paths {
		pathKey := NormalizeRepoPath(raw.Path)
		if pathKey == "" {
			continue
		}
		if raw.Deleted {
			current, ok := pathByKey[pathKey]
			if ok && !current.Deleted {
				continue
			}
			pathByKey[pathKey] = core.SyncPath{Path: pathKey, Deleted: true}
			continue
		}

		contentHash := strings.TrimSpace(raw.ContentHash)
		if contentHash == "" {
			return core.SyncApplyInput{}, fmt.Errorf("content_hash is required for path %q", pathKey)
		}
		pathByKey[pathKey] = core.SyncPath{
			Path:        pathKey,
			ContentHash: contentHash,
		}
	}

	keys := make([]string, 0, len(pathByKey))
	for key := range pathByKey {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	paths := make([]core.SyncPath, 0, len(keys))
	for _, key := range keys {
		paths = append(paths, pathByKey[key])
	}
	return core.SyncApplyInput{
		ProjectID:           projectID,
		Mode:                mode,
		InsertNewCandidates: input.InsertNewCandidates,
		Paths:               paths,
	}, nil
}

func NormalizeWorkItems(items []core.WorkItem) ([]core.WorkItem, error) {
	if len(items) == 0 {
		return nil, nil
	}

	priority := map[string]int{
		core.WorkItemStatusComplete:   0,
		core.WorkItemStatusCompleted:  0,
		core.WorkItemStatusPending:    1,
		core.WorkItemStatusInProgress: 2,
		core.WorkItemStatusBlocked:    3,
	}

	byKey := make(map[string]core.WorkItem, len(items))
	for _, raw := range items {
		itemKey := NormalizeRepoPath(raw.ItemKey)
		if itemKey == "" {
			return nil, fmt.Errorf("work item key is required")
		}
		status := NormalizeWorkItemStatus(raw.Status)

		current, exists := byKey[itemKey]
		if !exists || priority[status] >= priority[current.Status] {
			byKey[itemKey] = core.WorkItem{ItemKey: itemKey, Status: status}
		}
	}

	keys := make([]string, 0, len(byKey))
	for key := range byKey {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	out := make([]core.WorkItem, 0, len(keys))
	for _, key := range keys {
		out = append(out, byKey[key])
	}

	return out, nil
}

func DerivePlanStatus(items []core.WorkItem) string {
	if len(items) == 0 {
		return core.PlanStatusPending
	}

	hasPending := false
	hasInProgress := false
	hasBlocked := false
	hasCompleted := false

	for _, item := range items {
		switch NormalizeWorkItemStatus(item.Status) {
		case core.WorkItemStatusBlocked:
			hasBlocked = true
		case core.WorkItemStatusInProgress:
			hasInProgress = true
		case core.WorkItemStatusComplete:
			hasCompleted = true
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
	case hasCompleted:
		return core.PlanStatusComplete
	default:
		return core.PlanStatusPending
	}
}

func StorageWorkItemStatus(raw string) string {
	switch NormalizeWorkItemStatus(raw) {
	case core.WorkItemStatusComplete:
		return core.WorkItemStatusCompleted
	default:
		return NormalizeWorkItemStatus(raw)
	}
}

func NormalizeWorkItemStatus(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case core.WorkItemStatusComplete, core.WorkItemStatusCompleted:
		return core.WorkItemStatusComplete
	case core.WorkItemStatusInProgress:
		return core.WorkItemStatusInProgress
	case core.WorkItemStatusBlocked:
		return core.WorkItemStatusBlocked
	default:
		return core.WorkItemStatusPending
	}
}

func NormalizeRepoPath(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return ""
	}
	withSlashes := strings.ReplaceAll(trimmed, `\`, "/")
	cleaned := path.Clean(withSlashes)
	if cleaned == "." {
		return ""
	}
	return cleaned
}

func NormalizeStringList(values []string) []string {
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
	sort.Strings(out)
	return out
}

func NormalizeStringListPreserveOrder(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	out := make([]string, 0, len(values))
	for _, raw := range values {
		trimmed := strings.TrimSpace(raw)
		if trimmed == "" {
			continue
		}
		out = append(out, trimmed)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func NormalizeInt64List(values []int64) []int64 {
	if len(values) == 0 {
		return nil
	}
	seen := make(map[int64]struct{}, len(values))
	out := make([]int64, 0, len(values))
	for _, v := range values {
		if _, ok := seen[v]; ok {
			continue
		}
		seen[v] = struct{}{}
		out = append(out, v)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}
