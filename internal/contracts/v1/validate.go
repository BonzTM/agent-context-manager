package v1

import (
	"bytes"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

var (
	requestIDRe  = regexp.MustCompile(`^[A-Za-z0-9._:-]{8,128}$`)
	projectIDRe  = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{1,63}$`)
	tagRe        = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]{0,63}$`)
	pointerKeyRe = regexp.MustCompile(`^[^\s]+:[^\s#]+(?:#[^\s]+)?$`)
)

func DecodeAndValidateCommand(data []byte) (CommandEnvelope, any, *ErrorPayload) {
	var env CommandEnvelope
	if err := decodeStrict(data, &env); err != nil {
		return CommandEnvelope{}, nil, validationError("INVALID_JSON", err.Error())
	}

	if env.Version != Version {
		return CommandEnvelope{}, nil, validationError("INVALID_VERSION", "version must be acm.v1")
	}
	if !isValidCommand(env.Command) {
		return CommandEnvelope{}, nil, validationError("INVALID_COMMAND", "command is not recognized")
	}
	if !requestIDRe.MatchString(env.RequestID) {
		return CommandEnvelope{}, nil, validationError("INVALID_REQUEST_ID", "request_id format is invalid")
	}
	if len(env.Payload) == 0 || string(env.Payload) == "null" {
		return CommandEnvelope{}, nil, validationError("INVALID_PAYLOAD", "payload is required")
	}

	payload, errp := decodePayload(env.Command, env.Payload)
	if errp != nil {
		return CommandEnvelope{}, nil, errp
	}

	return env, payload, nil
}

func decodePayload(command Command, raw json.RawMessage) (any, *ErrorPayload) {
	switch command {
	case CommandGetContext:
		var p GetContextPayload
		if err := decodeStrict(raw, &p); err != nil {
			return nil, validationError("INVALID_PAYLOAD", err.Error())
		}
		if err := validateGetContextPayload(&p); err != nil {
			return nil, validationError("INVALID_PAYLOAD", err.Error())
		}
		return p, nil
	case CommandFetch:
		var p FetchPayload
		if err := decodeStrict(raw, &p); err != nil {
			return nil, validationError("INVALID_PAYLOAD", err.Error())
		}
		if err := validateFetchPayload(&p); err != nil {
			return nil, validationError("INVALID_PAYLOAD", err.Error())
		}
		return p, nil
	case CommandProposeMemory:
		var p ProposeMemoryPayload
		if err := decodeStrict(raw, &p); err != nil {
			return nil, validationError("INVALID_PAYLOAD", err.Error())
		}
		if err := validateProposeMemoryPayload(&p); err != nil {
			return nil, validationError("INVALID_PAYLOAD", err.Error())
		}
		return p, nil
	case CommandReportCompletion:
		var p ReportCompletionPayload
		if err := decodeStrict(raw, &p); err != nil {
			return nil, validationError("INVALID_PAYLOAD", err.Error())
		}
		if err := validateReportCompletionPayload(&p); err != nil {
			return nil, validationError("INVALID_PAYLOAD", err.Error())
		}
		return p, nil
	case CommandWork:
		var p WorkPayload
		if err := decodeStrict(raw, &p); err != nil {
			return nil, validationError("INVALID_PAYLOAD", err.Error())
		}
		if err := validateWorkPayload(&p); err != nil {
			return nil, validationError("INVALID_PAYLOAD", err.Error())
		}
		return p, nil
	case CommandSync:
		var p SyncPayload
		if err := decodeStrict(raw, &p); err != nil {
			return nil, validationError("INVALID_PAYLOAD", err.Error())
		}
		if err := validateSyncPayload(&p); err != nil {
			return nil, validationError("INVALID_PAYLOAD", err.Error())
		}
		return p, nil
	case CommandHealthCheck:
		var p HealthCheckPayload
		if err := decodeStrict(raw, &p); err != nil {
			return nil, validationError("INVALID_PAYLOAD", err.Error())
		}
		if err := validateHealthCheckPayload(&p); err != nil {
			return nil, validationError("INVALID_PAYLOAD", err.Error())
		}
		return p, nil
	case CommandHealthFix:
		var p HealthFixPayload
		if err := decodeStrict(raw, &p); err != nil {
			return nil, validationError("INVALID_PAYLOAD", err.Error())
		}
		if err := validateHealthFixPayload(&p); err != nil {
			return nil, validationError("INVALID_PAYLOAD", err.Error())
		}
		return p, nil
	case CommandCoverage:
		var p CoveragePayload
		if err := decodeStrict(raw, &p); err != nil {
			return nil, validationError("INVALID_PAYLOAD", err.Error())
		}
		if err := validateCoveragePayload(&p); err != nil {
			return nil, validationError("INVALID_PAYLOAD", err.Error())
		}
		return p, nil
	case CommandRegress:
		var p RegressPayload
		if err := decodeStrict(raw, &p); err != nil {
			return nil, validationError("INVALID_PAYLOAD", err.Error())
		}
		if err := validateRegressPayload(&p); err != nil {
			return nil, validationError("INVALID_PAYLOAD", err.Error())
		}
		return p, nil
	case CommandBootstrap:
		var p BootstrapPayload
		if err := decodeStrict(raw, &p); err != nil {
			return nil, validationError("INVALID_PAYLOAD", err.Error())
		}
		if err := validateBootstrapPayload(&p); err != nil {
			return nil, validationError("INVALID_PAYLOAD", err.Error())
		}
		return p, nil
	default:
		return nil, validationError("INVALID_COMMAND", "command is not recognized")
	}
}

func isValidCommand(command Command) bool {
	switch command {
	case CommandGetContext,
		CommandFetch,
		CommandProposeMemory,
		CommandReportCompletion,
		CommandWork,
		CommandSync,
		CommandHealthCheck,
		CommandHealthFix,
		CommandCoverage,
		CommandRegress,
		CommandBootstrap:
		return true
	default:
		return false
	}
}

func validateGetContextPayload(p *GetContextPayload) error {
	if err := validateProjectID(p.ProjectID); err != nil {
		return err
	}
	if strings.TrimSpace(p.TaskText) == "" || len(p.TaskText) > 4000 {
		return fmt.Errorf("task_text must be 1..4000 chars")
	}
	if p.Phase != PhasePlan && p.Phase != PhaseExecute && p.Phase != PhaseReview {
		return fmt.Errorf("phase must be plan|execute|review")
	}
	if err := validateScopeMode(p.ScopeMode); err != nil {
		return err
	}
	if p.FallbackMode != "" && p.FallbackMode != "widen_once" && p.FallbackMode != "none" {
		return fmt.Errorf("fallback_mode must be widen_once|none")
	}
	return nil
}

func validateFetchPayload(p *FetchPayload) error {
	if err := validateProjectID(p.ProjectID); err != nil {
		return err
	}

	receiptID := strings.TrimSpace(p.ReceiptID)
	if len(p.Keys) == 0 && receiptID == "" {
		return fmt.Errorf("either keys or receipt_id is required")
	}
	if len(p.Keys) > 256 {
		return fmt.Errorf("keys may include at most 256 entries")
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

func validateProposeMemoryPayload(p *ProposeMemoryPayload) error {
	if err := validateProjectID(p.ProjectID); err != nil {
		return err
	}
	if !requestIDRe.MatchString(p.ReceiptID) {
		return fmt.Errorf("receipt_id format is invalid")
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
	if len(m.EvidencePointerKeys) < 1 {
		return fmt.Errorf("memory.evidence_pointer_keys must not be empty")
	}
	for _, k := range append(append([]string{}, m.RelatedPointerKeys...), m.EvidencePointerKeys...) {
		if !pointerKeyRe.MatchString(k) {
			return fmt.Errorf("pointer key format is invalid")
		}
	}
	if err := validateTags(m.Tags); err != nil {
		return err
	}
	return nil
}

func validateReportCompletionPayload(p *ReportCompletionPayload) error {
	if err := validateProjectID(p.ProjectID); err != nil {
		return err
	}
	if !requestIDRe.MatchString(p.ReceiptID) {
		return fmt.Errorf("receipt_id format is invalid")
	}
	if strings.TrimSpace(p.Outcome) == "" || len(p.Outcome) > 1600 {
		return fmt.Errorf("outcome must be 1..1600 chars")
	}
	if err := validateScopeMode(p.ScopeMode); err != nil {
		return err
	}
	for _, path := range p.FilesChanged {
		if err := validateRelativePath(path); err != nil {
			return err
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
		if err := validateBoundedKey(planKey, 256); err != nil {
			return fmt.Errorf("plan_key %w", err)
		}
		if !strings.HasPrefix(planKey, "plan:") {
			return fmt.Errorf("plan_key must use format plan:<receipt_id>")
		}
		derivedReceiptID := strings.TrimSpace(planKey[len("plan:"):])
		if derivedReceiptID == "" || !requestIDRe.MatchString(derivedReceiptID) {
			return fmt.Errorf("plan_key must use format plan:<receipt_id>")
		}
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
	if len(p.Tasks) > 0 && len(p.Items) > 0 {
		return fmt.Errorf("use only one of tasks or items")
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
		if err := validateStringList(p.Plan.Constraints, 128, 600, "plan.constraints"); err != nil {
			return err
		}
		if err := validateStringList(p.Plan.References, 256, 2048, "plan.references"); err != nil {
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
		if err := validateBoundedKeyList(task.DependsOn, 128, 512, prefix+".depends_on"); err != nil {
			return err
		}
		if err := validateStringList(task.AcceptanceCriteria, 128, 600, prefix+".acceptance_criteria"); err != nil {
			return err
		}
		if err := validateStringList(task.References, 256, 2048, prefix+".references"); err != nil {
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
	if len(p.Items) > 256 {
		return fmt.Errorf("items may include at most 256 entries")
	}
	for i, item := range p.Items {
		if err := validateBoundedKey(item.Key, 512); err != nil {
			return fmt.Errorf("items[%d].key %w", i, err)
		}
		if strings.TrimSpace(item.Summary) == "" || len(item.Summary) > 600 {
			return fmt.Errorf("items[%d].summary must be 1..600 chars", i)
		}
		if err := validateWorkItemStatusValue(item.Status, fmt.Sprintf("items[%d].status", i)); err != nil {
			return err
		}
		if item.Outcome != "" {
			trimmedOutcome := strings.TrimSpace(item.Outcome)
			if trimmedOutcome == "" || len(trimmedOutcome) > 1600 {
				return fmt.Errorf("items[%d].outcome must be 1..1600 chars when provided", i)
			}
		}
	}
	return nil
}

func validateWorkItemStatusValue(status WorkItemStatus, field string) error {
	switch status {
	case WorkItemStatusPending, WorkItemStatusInProgress, WorkItemStatusComplete, WorkItemStatusBlocked:
		return nil
	default:
		return fmt.Errorf("%s must be pending|in_progress|complete|blocked", field)
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
	if err := validateRulesFile(p.RulesFile); err != nil {
		return err
	}
	return nil
}

func validateHealthCheckPayload(p *HealthCheckPayload) error {
	if err := validateProjectID(p.ProjectID); err != nil {
		return err
	}
	if p.MaxFindingsPerCheck != nil && (*p.MaxFindingsPerCheck < 1 || *p.MaxFindingsPerCheck > 500) {
		return fmt.Errorf("max_findings_per_check must be between 1 and 500")
	}
	return nil
}

func validateHealthFixPayload(p *HealthFixPayload) error {
	if err := validateProjectID(p.ProjectID); err != nil {
		return err
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
	for i, fixer := range p.Fixers {
		switch fixer {
		case HealthFixerSyncWorkingTree, HealthFixerIndexUncoveredFile, HealthFixerSyncRuleset:
		default:
			return fmt.Errorf("fixers[%d] is invalid", i)
		}
	}
	return nil
}

func validateCoveragePayload(p *CoveragePayload) error {
	if err := validateProjectID(p.ProjectID); err != nil {
		return err
	}
	if p.ProjectRoot != "" && len(strings.TrimSpace(p.ProjectRoot)) == 0 {
		return fmt.Errorf("project_root must be non-empty when provided")
	}
	if len(p.ProjectRoot) > 2048 {
		return fmt.Errorf("project_root too long")
	}
	return nil
}

func validateRegressPayload(p *RegressPayload) error {
	if err := validateProjectID(p.ProjectID); err != nil {
		return err
	}
	if p.EvalSuitePath == "" && len(p.EvalSuiteInline) == 0 {
		return fmt.Errorf("either eval_suite_path or eval_suite_inline is required")
	}
	if p.EvalSuitePath != "" && len(p.EvalSuiteInline) > 0 {
		return fmt.Errorf("use only one of eval_suite_path or eval_suite_inline")
	}
	if p.MinimumRecall != nil && (*p.MinimumRecall < 0 || *p.MinimumRecall > 1) {
		return fmt.Errorf("minimum_recall must be between 0 and 1")
	}
	for i := range p.EvalSuiteInline {
		c := p.EvalSuiteInline[i]
		if strings.TrimSpace(c.TaskText) == "" || len(c.TaskText) > 4000 {
			return fmt.Errorf("eval_suite_inline[%d].task_text invalid", i)
		}
		if c.Phase != PhasePlan && c.Phase != PhaseExecute && c.Phase != PhaseReview {
			return fmt.Errorf("eval_suite_inline[%d].phase invalid", i)
		}
	}
	return nil
}

func validateBootstrapPayload(p *BootstrapPayload) error {
	if err := validateProjectID(p.ProjectID); err != nil {
		return err
	}
	if strings.TrimSpace(p.ProjectRoot) == "" {
		return fmt.Errorf("project_root is required")
	}
	if err := validateRulesFile(p.RulesFile); err != nil {
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
	return nil
}

func validateRulesFile(rulesFile string) error {
	trimmed := strings.TrimSpace(rulesFile)
	if trimmed == "" {
		return nil
	}
	if len(trimmed) > 2048 {
		return fmt.Errorf("rules_file too long")
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

func validateScopeMode(mode ScopeMode) error {
	switch mode {
	case "", ScopeModeStrict, ScopeModeWarn, ScopeModeAutoIndex:
		return nil
	default:
		return fmt.Errorf("scope_mode must be strict|warn|auto_index")
	}
}

func validateTags(tags []string) error {
	for _, t := range tags {
		if !tagRe.MatchString(t) {
			return fmt.Errorf("tag format is invalid: %q", t)
		}
	}
	return nil
}

func validateRelativePath(path string) error {
	if strings.TrimSpace(path) == "" {
		return fmt.Errorf("relative path must not be empty")
	}
	if strings.HasPrefix(path, "/") || strings.HasPrefix(path, "\\") {
		return fmt.Errorf("absolute paths are not allowed")
	}
	if len(path) >= 3 && ((path[1] == ':' && (path[2] == '\\' || path[2] == '/')) || (path[0] == '.' && path[1] == '.')) {
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

func validationError(code, message string) *ErrorPayload {
	return &ErrorPayload{Code: code, Message: message}
}
