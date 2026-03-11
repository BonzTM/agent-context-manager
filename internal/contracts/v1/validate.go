package v1

import (
	"bytes"
	"encoding/json"
	"fmt"
	pathpkg "path"
	"regexp"
	"strings"

	"github.com/bonztm/agent-context-manager/internal/projectid"
)

var (
	requestIDRe  = regexp.MustCompile(`^[A-Za-z0-9._:-]{8,128}$`)
	projectIDRe  = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{1,63}$`)
	tagRe        = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]{0,63}$`)
	testIDRe     = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]{0,127}$`)
	pointerKeyRe = regexp.MustCompile(`^[^\s]+:[^\s#]+(?:#[^\s]+)?$`)
	planKindRe   = regexp.MustCompile(`^[a-z][a-z0-9_-]{0,63}$`)
)

type ValidationDefaults struct {
	ProjectID string
}

func DecodeAndValidateCommand(data []byte) (CommandEnvelope, any, *ErrorPayload) {
	return DecodeAndValidateCommandWithDefaults(data, ValidationDefaults{})
}

func DecodeAndValidateCommandWithDefaults(data []byte, defaults ValidationDefaults) (CommandEnvelope, any, *ErrorPayload) {
	var env CommandEnvelope
	if err := decodeStrict(data, &env); err != nil {
		return CommandEnvelope{}, nil, validationError("INVALID_JSON", err.Error())
	}

	if env.Version != Version {
		return CommandEnvelope{}, nil, validationError("INVALID_VERSION", "version must be acm.v1")
	}
	spec, ok := LookupCommand(env.Command)
	if !ok {
		return CommandEnvelope{}, nil, validationError("INVALID_COMMAND", "command is not recognized")
	}
	if !requestIDRe.MatchString(env.RequestID) {
		return CommandEnvelope{}, nil, validationError("INVALID_REQUEST_ID", "request_id format is invalid")
	}
	if len(env.Payload) == 0 || string(env.Payload) == "null" {
		return CommandEnvelope{}, nil, validationError("INVALID_PAYLOAD", "payload is required")
	}

	payload, errp := spec.Decode(env.Payload, normalizeValidationDefaults(defaults))
	if errp != nil {
		return CommandEnvelope{}, nil, errp
	}

	return env, payload, nil
}

func validateContextPayload(p *ContextPayload) error {
	if err := validateProjectID(p.ProjectID); err != nil {
		return err
	}
	if strings.TrimSpace(p.TaskText) == "" || len(p.TaskText) > 4000 {
		return fmt.Errorf("task_text must be 1..4000 chars")
	}
	if p.Phase != PhasePlan && p.Phase != PhaseExecute && p.Phase != PhaseReview {
		return fmt.Errorf("phase must be plan|execute|review")
	}
	if err := validateTagsFile(p.TagsFile); err != nil {
		return err
	}
	if err := validateRelativePathList(p.InitialScopePaths, 4096, "initial_scope_paths"); err != nil {
		return err
	}
	return nil
}

func validateFetchPayload(p *FetchPayload, fields map[string]json.RawMessage) error {
	if err := validateProjectID(p.ProjectID); err != nil {
		return err
	}

	receiptID := strings.TrimSpace(p.ReceiptID)
	if err := validateOptionalArrayField(fields, "keys", len(p.Keys)); err != nil {
		return err
	}
	if len(p.Keys) == 0 && receiptID == "" {
		return fmt.Errorf("either keys or receipt_id is required")
	}
	if len(p.Keys) > 256 {
		return fmt.Errorf("keys may include at most 256 entries")
	}
	if err := validateUniqueStrings(p.Keys, "keys"); err != nil {
		return err
	}
	for i, key := range p.Keys {
		if err := validateBoundedKey(key, 512); err != nil {
			return fmt.Errorf("keys[%d] %w", i, err)
		}
	}
	if receiptID != "" && !requestIDRe.MatchString(receiptID) {
		return fmt.Errorf("receipt_id format is invalid")
	}
	if len(p.ExpectedVersions) > 256 {
		return fmt.Errorf("expected_versions may include at most 256 entries")
	}
	if len(p.Keys) == 0 && len(p.ExpectedVersions) > 0 {
		return fmt.Errorf("expected_versions requires keys")
	}
	for key, version := range p.ExpectedVersions {
		if err := validateBoundedKey(key, 512); err != nil {
			return fmt.Errorf("expected_versions key %q invalid: %w", key, err)
		}
		trimmedVersion := strings.TrimSpace(version)
		if trimmedVersion == "" || len(trimmedVersion) > 128 {
			return fmt.Errorf("expected_versions[%q] must be 1..128 chars", key)
		}
	}
	return nil
}

func validateMemoryCommandPayload(p *MemoryCommandPayload) error {
	if err := validateProjectID(p.ProjectID); err != nil {
		return err
	}
	if err := validateReceiptOrPlanReference(p.ReceiptID, p.PlanKey); err != nil {
		return err
	}
	if err := validateTagsFile(p.TagsFile); err != nil {
		return err
	}
	if err := validateMemoryPayload(&p.Memory); err != nil {
		return err
	}
	return nil
}

func validateMemoryPayload(m *MemoryPayload) error {
	switch m.Category {
	case MemoryCategoryDecision, MemoryCategoryGotcha, MemoryCategoryPattern, MemoryCategoryPreference:
	default:
		return fmt.Errorf("memory.category is invalid")
	}
	if strings.TrimSpace(m.Subject) == "" || len(m.Subject) > 160 {
		return fmt.Errorf("memory.subject must be 1..160 chars")
	}
	if strings.TrimSpace(m.Content) == "" || len(m.Content) > 600 {
		return fmt.Errorf("memory.content must be 1..600 chars")
	}
	if m.Confidence < 1 || m.Confidence > 5 {
		return fmt.Errorf("memory.confidence must be 1..5")
	}
	if err := validatePointerKeyList(m.RelatedPointerKeys, 0, 32, "memory.related_pointer_keys"); err != nil {
		return err
	}
	if err := validatePointerKeyList(m.EvidencePointerKeys, 1, 16, "memory.evidence_pointer_keys"); err != nil {
		return err
	}
	if err := validateTags(m.Tags); err != nil {
		return err
	}
	return nil
}

func validateDonePayload(p *DonePayload, fields map[string]json.RawMessage) error {
	if err := validateProjectID(p.ProjectID); err != nil {
		return err
	}
	if err := validateReceiptOrPlanReference(p.ReceiptID, p.PlanKey); err != nil {
		return err
	}
	if strings.TrimSpace(p.Outcome) == "" || len(p.Outcome) > 1600 {
		return fmt.Errorf("outcome must be 1..1600 chars")
	}
	if err := validateScopeMode(p.ScopeMode); err != nil {
		return err
	}
	if err := validateTagsFile(p.TagsFile); err != nil {
		return err
	}
	if raw, ok := fields["files_changed"]; ok && bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return fmt.Errorf("files_changed must be an array")
	}
	if raw, ok := fields["no_file_changes"]; ok && bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return fmt.Errorf("no_file_changes must be a boolean")
	}
	if err := validateRelativePathList(p.FilesChanged, 256, "files_changed"); err != nil {
		return err
	}
	if p.NoFileChanges {
		if _, ok := fields["files_changed"]; ok {
			return fmt.Errorf("files_changed cannot be combined with no_file_changes")
		}
	}
	return nil
}

func validateReviewPayload(p *ReviewPayload, fields map[string]json.RawMessage) error {
	if err := validateProjectID(p.ProjectID); err != nil {
		return err
	}

	if err := validateReceiptOrPlanReference(p.ReceiptID, p.PlanKey); err != nil {
		return err
	}
	if p.Key != "" {
		if err := validateBoundedKey(p.Key, 512); err != nil {
			return fmt.Errorf("key %w", err)
		}
	}
	if p.Summary != "" {
		trimmed := strings.TrimSpace(p.Summary)
		if trimmed == "" || len(trimmed) > 600 {
			return fmt.Errorf("summary must be 1..600 chars when provided")
		}
	}
	if p.Status != "" {
		if err := validateWorkItemStatusValue(p.Status, "status"); err != nil {
			return err
		}
	}
	if p.BlockedReason != "" {
		trimmed := strings.TrimSpace(p.BlockedReason)
		if trimmed == "" || len(trimmed) > 600 {
			return fmt.Errorf("blocked_reason must be 1..600 chars when provided")
		}
		if p.Status != "" && p.Status != WorkItemStatusBlocked {
			return fmt.Errorf("blocked_reason requires status=blocked when status is provided")
		}
	}
	if p.Outcome != "" {
		trimmed := strings.TrimSpace(p.Outcome)
		if trimmed == "" || len(trimmed) > 1600 {
			return fmt.Errorf("outcome must be 1..1600 chars when provided")
		}
	}
	if p.Status == WorkItemStatusBlocked && strings.TrimSpace(p.BlockedReason) == "" {
		return fmt.Errorf("blocked_reason is required when status=blocked")
	}
	if err := validateOptionalArrayField(fields, "evidence", len(p.Evidence)); err != nil {
		return err
	}
	if err := validateStringList(p.Evidence, 128, 1600, "evidence"); err != nil {
		return err
	}
	if err := validateTagsFile(p.TagsFile); err != nil {
		return err
	}
	if p.Run {
		if p.Status != "" {
			return fmt.Errorf("status must be omitted when run=true")
		}
		if strings.TrimSpace(p.Outcome) != "" {
			return fmt.Errorf("outcome must be omitted when run=true")
		}
		if strings.TrimSpace(p.BlockedReason) != "" {
			return fmt.Errorf("blocked_reason must be omitted when run=true")
		}
		if len(p.Evidence) > 0 {
			return fmt.Errorf("evidence must be omitted when run=true")
		}
	}
	return nil
}

func validateWorkPayload(p *WorkPayload) error {
	if err := validateProjectID(p.ProjectID); err != nil {
		return err
	}

	rawPlanKey := p.PlanKey
	planKey := strings.TrimSpace(rawPlanKey)
	receiptID := strings.TrimSpace(p.ReceiptID)
	if planKey == "" && receiptID == "" {
		return fmt.Errorf("either plan_key or receipt_id is required")
	}
	if rawPlanKey != "" && rawPlanKey != planKey {
		return fmt.Errorf("plan_key must not include surrounding whitespace")
	}
	if planKey != "" {
		if err := validatePlanKeyFormat(planKey, "plan_key"); err != nil {
			return err
		}
		derivedReceiptID := strings.TrimSpace(planKey[len("plan:"):])
		if receiptID != "" && receiptID != derivedReceiptID {
			return fmt.Errorf("plan_key and receipt_id must reference the same receipt")
		}
	}
	if p.PlanTitle != "" {
		trimmedTitle := strings.TrimSpace(p.PlanTitle)
		if trimmedTitle == "" || len(trimmedTitle) > 200 {
			return fmt.Errorf("plan_title must be 1..200 chars when provided")
		}
	}
	if receiptID != "" && !requestIDRe.MatchString(receiptID) {
		return fmt.Errorf("receipt_id format is invalid")
	}
	if p.Mode != "" && p.Mode != WorkPlanModeMerge && p.Mode != WorkPlanModeReplace {
		return fmt.Errorf("mode must be merge|replace")
	}
	if p.Plan != nil {
		if p.Plan.Title != "" {
			trimmed := strings.TrimSpace(p.Plan.Title)
			if trimmed == "" || len(trimmed) > 200 {
				return fmt.Errorf("plan.title must be 1..200 chars when provided")
			}
		}
		if p.Plan.Objective != "" {
			trimmed := strings.TrimSpace(p.Plan.Objective)
			if trimmed == "" || len(trimmed) > 2000 {
				return fmt.Errorf("plan.objective must be 1..2000 chars when provided")
			}
		}
		if p.Plan.Kind != "" {
			trimmed := strings.TrimSpace(p.Plan.Kind)
			if !planKindRe.MatchString(trimmed) {
				return fmt.Errorf("plan.kind must match ^[a-z][a-z0-9_-]{0,63}$")
			}
		}
		if p.Plan.ParentPlanKey != "" {
			if err := validatePlanKeyFormat(p.Plan.ParentPlanKey, "plan.parent_plan_key"); err != nil {
				return err
			}
		}
		if p.Plan.Status != "" {
			if err := validateWorkItemStatusValue(p.Plan.Status, "plan.status"); err != nil {
				return err
			}
		}
		if p.Plan.Stages != nil {
			if p.Plan.Stages.SpecOutline != "" {
				if err := validateWorkItemStatusValue(p.Plan.Stages.SpecOutline, "plan.stages.spec_outline"); err != nil {
					return err
				}
			}
			if p.Plan.Stages.RefinedSpec != "" {
				if err := validateWorkItemStatusValue(p.Plan.Stages.RefinedSpec, "plan.stages.refined_spec"); err != nil {
					return err
				}
			}
			if p.Plan.Stages.ImplementationPlan != "" {
				if err := validateWorkItemStatusValue(p.Plan.Stages.ImplementationPlan, "plan.stages.implementation_plan"); err != nil {
					return err
				}
			}
		}
		if err := validateStringList(p.Plan.InScope, 128, 600, "plan.in_scope"); err != nil {
			return err
		}
		if err := validateStringList(p.Plan.OutOfScope, 128, 600, "plan.out_of_scope"); err != nil {
			return err
		}
		if err := validateRelativePathList(p.Plan.DiscoveredPaths, 4096, "plan.discovered_paths"); err != nil {
			return err
		}
		if err := validateStringList(p.Plan.Constraints, 128, 600, "plan.constraints"); err != nil {
			return err
		}
		if err := validateStringList(p.Plan.References, 256, 2048, "plan.references"); err != nil {
			return err
		}
		if err := validateStringList(p.Plan.ExternalRefs, 256, 2048, "plan.external_refs"); err != nil {
			return err
		}
	}
	if len(p.Tasks) > 256 {
		return fmt.Errorf("tasks may include at most 256 entries")
	}
	for i, task := range p.Tasks {
		prefix := fmt.Sprintf("tasks[%d]", i)
		if err := validateBoundedKey(task.Key, 512); err != nil {
			return fmt.Errorf("%s.key %w", prefix, err)
		}
		if strings.TrimSpace(task.Summary) == "" || len(task.Summary) > 600 {
			return fmt.Errorf("%s.summary must be 1..600 chars", prefix)
		}
		if err := validateWorkItemStatusValue(task.Status, prefix+".status"); err != nil {
			return err
		}
		if task.ParentTaskKey != "" {
			if err := validateBoundedKey(task.ParentTaskKey, 512); err != nil {
				return fmt.Errorf("%s.parent_task_key %w", prefix, err)
			}
		}
		if err := validateBoundedKeyList(task.DependsOn, 128, 512, prefix+".depends_on"); err != nil {
			return err
		}
		if err := validateStringList(task.AcceptanceCriteria, 128, 600, prefix+".acceptance_criteria"); err != nil {
			return err
		}
		if err := validateStringList(task.References, 256, 2048, prefix+".references"); err != nil {
			return err
		}
		if err := validateStringList(task.ExternalRefs, 256, 2048, prefix+".external_refs"); err != nil {
			return err
		}
		if task.BlockedReason != "" {
			trimmed := strings.TrimSpace(task.BlockedReason)
			if trimmed == "" || len(trimmed) > 600 {
				return fmt.Errorf("%s.blocked_reason must be 1..600 chars when provided", prefix)
			}
		}
		if task.Outcome != "" {
			trimmed := strings.TrimSpace(task.Outcome)
			if trimmed == "" || len(trimmed) > 1600 {
				return fmt.Errorf("%s.outcome must be 1..1600 chars when provided", prefix)
			}
		}
		if err := validateStringList(task.Evidence, 128, 1600, prefix+".evidence"); err != nil {
			return err
		}
	}
	return nil
}

func validateWorkItemStatusValue(status WorkItemStatus, field string) error {
	switch status {
	case WorkItemStatusPending, WorkItemStatusInProgress, WorkItemStatusComplete, WorkItemStatusBlocked, WorkItemStatusSuperseded:
		return nil
	default:
		return fmt.Errorf("%s must be pending|in_progress|complete|blocked|superseded", field)
	}
}

func validateStringList(values []string, maxItems, maxLen int, field string) error {
	if len(values) > maxItems {
		return fmt.Errorf("%s may include at most %d entries", field, maxItems)
	}
	for i, raw := range values {
		trimmed := strings.TrimSpace(raw)
		if trimmed == "" || len(trimmed) > maxLen {
			return fmt.Errorf("%s[%d] must be 1..%d chars", field, i, maxLen)
		}
	}
	return nil
}

func validateBoundedKeyList(values []string, maxItems, maxKeyLen int, field string) error {
	if len(values) > maxItems {
		return fmt.Errorf("%s may include at most %d entries", field, maxItems)
	}
	for i, raw := range values {
		if err := validateBoundedKey(raw, maxKeyLen); err != nil {
			return fmt.Errorf("%s[%d] %w", field, i, err)
		}
	}
	return nil
}

func validateSyncPayload(p *SyncPayload) error {
	if err := validateProjectID(p.ProjectID); err != nil {
		return err
	}
	if p.Mode != "" && p.Mode != "changed" && p.Mode != "full" && p.Mode != "working_tree" {
		return fmt.Errorf("mode must be changed|full|working_tree")
	}
	if p.GitRange != "" && len(p.GitRange) > 200 {
		return fmt.Errorf("git_range too long")
	}
	if err := validateOptionalProjectRoot(p.ProjectRoot); err != nil {
		return err
	}
	if err := validateRulesFile(p.RulesFile); err != nil {
		return err
	}
	if err := validateTagsFile(p.TagsFile); err != nil {
		return err
	}
	return nil
}

func validateHistorySearchPayload(p *HistorySearchPayload) error {
	if err := validateProjectID(p.ProjectID); err != nil {
		return err
	}
	if p.Entity != "" &&
		p.Entity != HistoryEntityAll &&
		p.Entity != HistoryEntityMemory &&
		p.Entity != HistoryEntityWork &&
		p.Entity != HistoryEntityReceipt &&
		p.Entity != HistoryEntityRun {
		return fmt.Errorf("entity must be all|memory|work|receipt|run")
	}
	if strings.TrimSpace(p.Query) != "" && len(strings.TrimSpace(p.Query)) > 4000 {
		return fmt.Errorf("query must be 1..4000 chars when provided")
	}
	entity := normalizeHistoryEntityValue(p.Entity)
	if p.Scope != "" {
		if entity != HistoryEntityWork {
			return fmt.Errorf("scope is only supported when entity=work")
		}
		if p.Scope != HistoryScopeCurrent &&
			p.Scope != HistoryScopeDeferred &&
			p.Scope != HistoryScopeCompleted &&
			p.Scope != HistoryScopeAll {
			return fmt.Errorf("scope must be current|deferred|completed|all")
		}
	}
	if strings.TrimSpace(p.Kind) != "" {
		if entity != HistoryEntityWork {
			return fmt.Errorf("kind is only supported when entity=work")
		}
		if len(strings.TrimSpace(p.Kind)) > 64 {
			return fmt.Errorf("kind must be 1..64 chars when provided")
		}
		if !planKindRe.MatchString(strings.TrimSpace(p.Kind)) {
			return fmt.Errorf("kind format is invalid")
		}
	}
	if p.Limit != 0 && (p.Limit < 1 || p.Limit > 100) {
		return fmt.Errorf("limit must be between 1 and 100")
	}
	return nil
}

func validateHealthPayload(p *HealthPayload, fields map[string]json.RawMessage) error {
	if err := validateProjectID(p.ProjectID); err != nil {
		return err
	}
	if p.MaxFindingsPerCheck != nil && (*p.MaxFindingsPerCheck < 1 || *p.MaxFindingsPerCheck > 500) {
		return fmt.Errorf("max_findings_per_check must be between 1 and 500")
	}
	if p.ProjectRoot != "" && len(strings.TrimSpace(p.ProjectRoot)) == 0 {
		return fmt.Errorf("project_root must be non-empty when provided")
	}
	if len(p.ProjectRoot) > 2048 {
		return fmt.Errorf("project_root too long")
	}
	if err := validateRulesFile(p.RulesFile); err != nil {
		return err
	}
	if err := validateTagsFile(p.TagsFile); err != nil {
		return err
	}
	if err := validateOptionalArrayField(fields, "fixers", len(p.Fixers)); err != nil {
		return err
	}
	if len(p.Fixers) > 4 {
		return fmt.Errorf("fixers may include at most 4 entries")
	}
	if err := validateUniqueStrings(healthFixerStrings(p.Fixers), "fixers"); err != nil {
		return err
	}
	for i, fixer := range p.Fixers {
		switch fixer {
		case HealthFixerAll, HealthFixerSyncWorkingTree, HealthFixerIndexUnindexedFile, HealthFixerSyncRuleset:
		default:
			return fmt.Errorf("fixers[%d] is invalid", i)
		}
	}
	isFixMode := len(p.Fixers) > 0 || p.Apply != nil || strings.TrimSpace(p.ProjectRoot) != "" || strings.TrimSpace(p.RulesFile) != "" || strings.TrimSpace(p.TagsFile) != ""
	if isFixMode {
		if p.IncludeDetails != nil || p.MaxFindingsPerCheck != nil {
			return fmt.Errorf("include_details and max_findings_per_check are only valid in health inspection mode")
		}
	}
	return nil
}

func normalizeHistoryEntityValue(raw HistoryEntity) HistoryEntity {
	switch strings.TrimSpace(string(raw)) {
	case string(HistoryEntityAll):
		return HistoryEntityAll
	case string(HistoryEntityMemory):
		return HistoryEntityMemory
	case string(HistoryEntityReceipt):
		return HistoryEntityReceipt
	case string(HistoryEntityRun):
		return HistoryEntityRun
	case string(HistoryEntityWork):
		return HistoryEntityWork
	default:
		return HistoryEntityAll
	}
}

func validateStatusPayload(p *StatusPayload) error {
	if err := validateProjectID(p.ProjectID); err != nil {
		return err
	}
	if err := validateOptionalProjectRoot(p.ProjectRoot); err != nil {
		return err
	}
	if err := validateRulesFile(p.RulesFile); err != nil {
		return err
	}
	if err := validateTagsFile(p.TagsFile); err != nil {
		return err
	}
	if err := validateTestsFile(p.TestsFile); err != nil {
		return err
	}
	if err := validateWorkflowsFile(p.WorkflowsFile); err != nil {
		return err
	}
	if trimmed := strings.TrimSpace(p.TaskText); trimmed != "" {
		if len(trimmed) > 4000 {
			return fmt.Errorf("task_text too long")
		}
	}
	if p.Phase != "" && p.Phase != PhasePlan && p.Phase != PhaseExecute && p.Phase != PhaseReview {
		return fmt.Errorf("phase must be plan|execute|review")
	}
	return nil
}

func validateVerifyPayload(p *VerifyPayload, fields map[string]json.RawMessage) error {
	if err := validateProjectID(p.ProjectID); err != nil {
		return err
	}

	receiptID := strings.TrimSpace(p.ReceiptID)
	planKey := strings.TrimSpace(p.PlanKey)
	if receiptID != "" || planKey != "" {
		if err := validateReceiptOrPlanReference(receiptID, planKey); err != nil {
			return err
		}
	}
	if p.Phase != "" && p.Phase != PhasePlan && p.Phase != PhaseExecute && p.Phase != PhaseReview {
		return fmt.Errorf("phase must be plan|execute|review")
	}
	if err := validateOptionalArrayField(fields, "test_ids", len(p.TestIDs)); err != nil {
		return err
	}
	if len(p.TestIDs) > 256 {
		return fmt.Errorf("test_ids may include at most 256 entries")
	}
	if err := validateUniqueStrings(p.TestIDs, "test_ids"); err != nil {
		return err
	}
	for i, raw := range p.TestIDs {
		trimmed := strings.TrimSpace(raw)
		if !testIDRe.MatchString(trimmed) {
			return fmt.Errorf("test_ids[%d] format is invalid", i)
		}
	}
	if err := validateOptionalArrayField(fields, "files_changed", len(p.FilesChanged)); err != nil {
		return err
	}
	if err := validateRelativePathList(p.FilesChanged, 4096, "files_changed"); err != nil {
		return err
	}
	if err := validateTestsFile(p.TestsFile); err != nil {
		return err
	}
	if err := validateTagsFile(p.TagsFile); err != nil {
		return err
	}
	if len(p.TestIDs) == 0 && receiptID == "" && planKey == "" && p.Phase == "" && len(p.FilesChanged) == 0 {
		return fmt.Errorf("test_ids or selection context is required")
	}
	return nil
}

func validateInitPayload(p *InitPayload, fields map[string]json.RawMessage) error {
	if err := validateProjectID(p.ProjectID); err != nil {
		return err
	}
	if err := validateOptionalProjectRoot(p.ProjectRoot); err != nil {
		return err
	}
	if err := validateRulesFile(p.RulesFile); err != nil {
		return err
	}
	if err := validateTagsFile(p.TagsFile); err != nil {
		return err
	}
	if p.OutputCandidatesPath != nil {
		value := strings.TrimSpace(*p.OutputCandidatesPath)
		if value == "" {
			return fmt.Errorf("output_candidates_path must be non-empty when provided")
		}
		if len(value) > 2048 {
			return fmt.Errorf("output_candidates_path too long")
		}
	}
	if err := validateOptionalArrayField(fields, "apply_templates", len(p.ApplyTemplates)); err != nil {
		return err
	}
	if err := validateUniqueStrings(p.ApplyTemplates, "apply_templates"); err != nil {
		return err
	}
	for i, templateID := range p.ApplyTemplates {
		if err := validateBoundedKey(templateID, 128); err != nil {
			return fmt.Errorf("apply_templates[%d] %w", i, err)
		}
	}
	return nil
}

func validateRulesFile(rulesFile string) error {
	return validateOptionalFilePath("rules_file", rulesFile)
}

func validateTagsFile(tagsFile string) error {
	return validateOptionalFilePath("tags_file", tagsFile)
}

func validateTestsFile(testsFile string) error {
	return validateOptionalFilePath("tests_file", testsFile)
}

func validateWorkflowsFile(workflowsFile string) error {
	return validateOptionalFilePath("workflows_file", workflowsFile)
}

func validateOptionalFilePath(fieldName, filePath string) error {
	trimmed := strings.TrimSpace(filePath)
	if trimmed == "" {
		return nil
	}
	if len(trimmed) > 2048 {
		return fmt.Errorf("%s too long", fieldName)
	}
	return nil
}

func validatePlanKeyFormat(planKey, field string) error {
	trimmed := strings.TrimSpace(planKey)
	if planKey != trimmed {
		return fmt.Errorf("%s must not include surrounding whitespace", field)
	}
	if err := validateBoundedKey(trimmed, 256); err != nil {
		return fmt.Errorf("%s %w", field, err)
	}
	if !strings.HasPrefix(trimmed, "plan:") {
		return fmt.Errorf("%s must use format plan:<receipt_id>", field)
	}
	derivedReceiptID := strings.TrimSpace(trimmed[len("plan:"):])
	if derivedReceiptID == "" || !requestIDRe.MatchString(derivedReceiptID) {
		return fmt.Errorf("%s must use format plan:<receipt_id>", field)
	}
	return nil
}

func validateReceiptOrPlanReference(receiptID, planKey string) error {
	trimmedReceiptID := strings.TrimSpace(receiptID)
	trimmedPlanKey := strings.TrimSpace(planKey)
	if trimmedReceiptID == "" && trimmedPlanKey == "" {
		return fmt.Errorf("either receipt_id or plan_key is required")
	}
	if trimmedReceiptID != "" && !requestIDRe.MatchString(trimmedReceiptID) {
		return fmt.Errorf("receipt_id format is invalid")
	}
	if trimmedPlanKey == "" {
		return nil
	}
	if err := validatePlanKeyFormat(trimmedPlanKey, "plan_key"); err != nil {
		return err
	}
	derivedReceiptID := strings.TrimSpace(trimmedPlanKey[len("plan:"):])
	if trimmedReceiptID != "" && trimmedReceiptID != derivedReceiptID {
		return fmt.Errorf("plan_key and receipt_id must reference the same receipt")
	}
	return nil
}

func validateBoundedKey(value string, maxLength int) error {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" || len(trimmed) > maxLength {
		return fmt.Errorf("must be 1..%d chars", maxLength)
	}
	return nil
}

func validateProjectID(v string) error {
	if !projectIDRe.MatchString(v) {
		return fmt.Errorf("project_id format is invalid")
	}
	return nil
}

func normalizeValidationDefaults(defaults ValidationDefaults) ValidationDefaults {
	return ValidationDefaults{
		ProjectID: strings.TrimSpace(defaults.ProjectID),
	}
}

func defaultProjectID(projectID string, defaults ValidationDefaults) string {
	if trimmed := strings.TrimSpace(projectID); trimmed != "" {
		return trimmed
	}
	return strings.TrimSpace(defaults.ProjectID)
}

func defaultProjectIDForRoot(projectID, projectRoot string, defaults ValidationDefaults) string {
	if trimmed := strings.TrimSpace(projectID); trimmed != "" {
		return trimmed
	}
	if inferred := projectid.FromRoot(projectRoot); inferred != "" {
		return inferred
	}
	return strings.TrimSpace(defaults.ProjectID)
}

func validateScopeMode(mode ScopeMode) error {
	switch mode {
	case "", ScopeModeStrict, ScopeModeWarn:
		return nil
	default:
		return fmt.Errorf("scope_mode must be strict|warn")
	}
}

func validateTags(tags []string) error {
	if len(tags) > 64 {
		return fmt.Errorf("memory.tags may include at most 64 entries")
	}
	if err := validateUniqueStrings(tags, "memory.tags"); err != nil {
		return err
	}
	for i, t := range tags {
		if !tagRe.MatchString(t) {
			return fmt.Errorf("memory.tags[%d] format is invalid", i)
		}
	}
	return nil
}

func validateRelativePath(path string) error {
	trimmed := strings.TrimSpace(path)
	if trimmed == "" {
		return fmt.Errorf("relative path must not be empty")
	}
	normalized := strings.ReplaceAll(trimmed, "\\", "/")
	if strings.HasPrefix(normalized, "/") {
		return fmt.Errorf("absolute paths are not allowed")
	}
	if len(normalized) >= 3 && normalized[1] == ':' && normalized[2] == '/' {
		return fmt.Errorf("absolute paths are not allowed")
	}
	cleaned := pathpkg.Clean(normalized)
	if cleaned == "." || cleaned == ".." || strings.HasPrefix(cleaned, "../") {
		return fmt.Errorf("path must be repository-relative")
	}
	return nil
}

func decodeStrict(data []byte, out any) error {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(out); err != nil {
		return err
	}
	var extra any
	if err := dec.Decode(&extra); err == nil {
		return fmt.Errorf("unexpected trailing JSON tokens")
	}
	return nil
}

func decodeObjectFields(raw json.RawMessage) (map[string]json.RawMessage, error) {
	fields := map[string]json.RawMessage{}
	if err := json.Unmarshal(raw, &fields); err != nil {
		return nil, err
	}
	return fields, nil
}

func validateOptionalArrayField(fields map[string]json.RawMessage, field string, length int) error {
	raw, ok := fields[field]
	if !ok {
		return nil
	}
	if bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return fmt.Errorf("%s must be an array", field)
	}
	if length == 0 {
		return fmt.Errorf("%s must not be empty when provided", field)
	}
	return nil
}

func validateRequiredArrayField(fields map[string]json.RawMessage, field string) error {
	raw, ok := fields[field]
	if !ok {
		return fmt.Errorf("%s is required", field)
	}
	if bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return fmt.Errorf("%s must be an array", field)
	}
	return nil
}

func validateUniqueStrings(values []string, field string) error {
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		if _, ok := seen[value]; ok {
			return fmt.Errorf("%s must not contain duplicates", field)
		}
		seen[value] = struct{}{}
	}
	return nil
}

func validatePointerKeyList(values []string, minItems, maxItems int, field string) error {
	if len(values) < minItems {
		return fmt.Errorf("%s must include at least %d entries", field, minItems)
	}
	if len(values) > maxItems {
		return fmt.Errorf("%s may include at most %d entries", field, maxItems)
	}
	if err := validateUniqueStrings(values, field); err != nil {
		return err
	}
	for i, value := range values {
		if err := validateBoundedKey(value, 512); err != nil {
			return fmt.Errorf("%s[%d] %w", field, i, err)
		}
		if !pointerKeyRe.MatchString(value) {
			return fmt.Errorf("%s[%d] format is invalid", field, i)
		}
	}
	return nil
}

func validateRelativePathList(values []string, maxItems int, field string) error {
	if len(values) > maxItems {
		return fmt.Errorf("%s may include at most %d entries", field, maxItems)
	}
	if err := validateUniqueStrings(values, field); err != nil {
		return err
	}
	for i, value := range values {
		if err := validateRelativePath(value); err != nil {
			return fmt.Errorf("%s[%d] %w", field, i, err)
		}
	}
	return nil
}

func validateUniqueBoundedStringList(values []string, maxItems, maxLen int, field string) error {
	if len(values) > maxItems {
		return fmt.Errorf("%s may include at most %d entries", field, maxItems)
	}
	if err := validateUniqueStrings(values, field); err != nil {
		return err
	}
	for i, raw := range values {
		trimmed := strings.TrimSpace(raw)
		if trimmed == "" || len(trimmed) > maxLen {
			return fmt.Errorf("%s[%d] must be 1..%d chars", field, i, maxLen)
		}
	}
	return nil
}

func validateOptionalProjectRoot(projectRoot string) error {
	if projectRoot == "" {
		return nil
	}
	if len(strings.TrimSpace(projectRoot)) == 0 {
		return fmt.Errorf("project_root must be non-empty when provided")
	}
	if len(projectRoot) > 2048 {
		return fmt.Errorf("project_root too long")
	}
	return nil
}

func healthFixerStrings(fixers []HealthFixer) []string {
	values := make([]string, 0, len(fixers))
	for _, fixer := range fixers {
		values = append(values, string(fixer))
	}
	return values
}

func validationError(code, message string) *ErrorPayload {
	return &ErrorPayload{Code: code, Message: message}
}
