package backend

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"

	bootstrapkit "github.com/bonztm/agent-context-manager/internal/bootstrap"
	"github.com/bonztm/agent-context-manager/internal/contracts/v1"
	"github.com/bonztm/agent-context-manager/internal/core"
	"github.com/bonztm/agent-context-manager/internal/workspace"
)

func (s *Service) HealthCheck(ctx context.Context, payload v1.HealthCheckPayload) (v1.HealthCheckResult, *core.APIError) {
	if s == nil || s.repo == nil {
		return v1.HealthCheckResult{}, core.NewError("INTERNAL_ERROR", "service repository is not configured", nil)
	}

	candidates, err := s.repo.FetchCandidatePointers(ctx, core.CandidatePointerQuery{
		ProjectID: strings.TrimSpace(payload.ProjectID),
		TaskText:  "",
		Unbounded: true,
		StaleFilter: core.StaleFilter{
			AllowStale: true,
		},
	})
	if err != nil {
		return v1.HealthCheckResult{}, healthCheckInternalError("fetch_candidate_pointers", err)
	}

	memories, err := s.repo.FetchActiveMemories(ctx, core.ActiveMemoryQuery{
		ProjectID: strings.TrimSpace(payload.ProjectID),
		Unbounded: true,
	})
	if err != nil {
		return v1.HealthCheckResult{}, healthCheckInternalError("fetch_active_memories", err)
	}

	includeDetails := effectiveHealthIncludeDetails(payload.IncludeDetails)
	maxFindings := effectiveMaxFindingsPerCheck(payload.MaxFindingsPerCheck)
	coverageResult, apiErr := s.Coverage(ctx, v1.CoveragePayload{ProjectID: strings.TrimSpace(payload.ProjectID)})
	if apiErr != nil {
		return v1.HealthCheckResult{}, healthCheckInternalError("coverage", fmt.Errorf("%s", apiErr.Message))
	}
	checks := buildHealthChecks(candidates, memories, coverageResult.UnindexedPaths, includeDetails, maxFindings)

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

func (s *Service) Eval(ctx context.Context, payload v1.EvalPayload) (v1.EvalResult, *core.APIError) {
	if s == nil || s.repo == nil {
		return v1.EvalResult{}, core.NewError("INTERNAL_ERROR", "service repository is not configured", nil)
	}

	suite, err := loadEvalSuite(payload)
	if err != nil {
		return v1.EvalResult{}, evalInternalError("load_eval_suite", err)
	}
	if len(suite) == 0 {
		return v1.EvalResult{}, evalInternalError("load_eval_suite", fmt.Errorf("evaluation suite is empty"))
	}

	minimumRecall := effectiveMinimumRecall(payload.MinimumRecall)

	caseResults := make([]v1.EvalCaseResult, 0, len(suite))
	totalTP := 0
	totalFP := 0
	totalFN := 0

	for i, testCase := range suite {
		ctxResult, apiErr := s.GetContext(ctx, v1.GetContextPayload{
			ProjectID: payload.ProjectID,
			TaskText:  testCase.TaskText,
			Phase:     testCase.Phase,
			TagsFile:  payload.TagsFile,
		})
		if apiErr != nil {
			return v1.EvalResult{}, evalInternalError(
				"get_context",
				fmt.Errorf("case %d failed: %s (%s)", i, apiErr.Message, apiErr.Code),
			)
		}

		expected := expectedEvalArtifacts(testCase)
		predicted := predictedEvalArtifacts(ctxResult)
		tp, fp, fn := confusionCounts(expected, predicted)
		precision, recall, f1 := metricsFromCounts(tp, fp, fn)

		totalTP += tp
		totalFP += fp
		totalFN += fn

		caseResult := v1.EvalCaseResult{
			Index:     i,
			Precision: precision,
			Recall:    recall,
			F1:        f1,
		}
		if note := evalCaseNote(ctxResult.Status); note != "" {
			caseResult.Notes = note
		}
		caseResults = append(caseResults, caseResult)
	}

	aggregatePrecision, aggregateRecall, aggregateF1 := metricsFromCounts(totalTP, totalFP, totalFN)
	return v1.EvalResult{
		TotalCases: len(suite),
		Aggregate: v1.EvalAggregate{
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
		return v1.BootstrapResult{}, core.NewError("INTERNAL_ERROR", "service repository is not configured", nil)
	}

	projectRoot := s.effectiveProjectRoot(payload.ProjectRoot)
	outputPath, persistCandidates := bootstrapkit.ResolveOutputPath(projectRoot, derefString(payload.OutputCandidatesPath), effectivePersistCandidates(payload.PersistCandidates))
	excludedPaths := bootstrapManagedRelativePaths(projectRoot, outputPath, payload.RulesFile, payload.TagsFile)
	templates, err := bootstrapkit.ResolveTemplates(payload.ApplyTemplates)
	if err != nil {
		var unknown bootstrapkit.UnknownTemplateError
		if errors.As(err, &unknown) {
			return v1.BootstrapResult{}, core.NewError(
				"INVALID_INPUT",
				"unknown bootstrap template",
				map[string]any{"template_id": unknown.TemplateID},
			)
		}
		return v1.BootstrapResult{}, bootstrapInternalError("load_templates", err)
	}

	paths, warnings, err := s.collectBootstrapPaths(ctx, projectRoot, outputPath, payload.RulesFile, payload.TagsFile, effectiveRespectGitIgnore(payload.RespectGitIgnore))
	if err != nil {
		return v1.BootstrapResult{}, bootstrapInternalError("collect_project_paths", err)
	}

	if err := bootstrapkit.EnsureProjectScaffold(projectRoot, payload.RulesFile); err != nil {
		return v1.BootstrapResult{}, bootstrapInternalError("seed_scaffold", err)
	}
	if err := syncBootstrapCanonicalTagsFile(projectRoot, payload.TagsFile, paths); err != nil {
		return v1.BootstrapResult{}, bootstrapInternalError("seed_tags", err)
	}

	templateResults := []v1.BootstrapTemplateResult(nil)
	if len(templates) > 0 {
		appliedTemplates, err := bootstrapkit.ApplyTemplates(projectRoot, payload.ProjectID, templates)
		if err != nil {
			return v1.BootstrapResult{}, bootstrapInternalError("apply_templates", err)
		}
		templateResults = appliedTemplates.TemplateResults
		paths = mergeBootstrapTemplateCandidatePaths(paths, appliedTemplates.CandidatePaths, excludedPaths)
	}

	rulesetSync, err := s.syncCanonicalRulesets(ctx, strings.TrimSpace(payload.ProjectID), projectRoot, payload.RulesFile, payload.TagsFile, true)
	if err != nil {
		return v1.BootstrapResult{}, bootstrapInternalError("parse_ruleset", err)
	}
	warnings = append(warnings, canonicalRulesetWarnings(rulesetSync)...)

	indexedStubs, err := s.upsertAutoIndexedPaths(ctx, strings.TrimSpace(payload.ProjectID), projectRoot, payload.TagsFile, paths)
	if err != nil {
		return v1.BootstrapResult{}, bootstrapInternalError("upsert_pointer_stubs", err)
	}

	if persistCandidates {
		if err := bootstrapkit.WriteCandidates(outputPath, paths); err != nil {
			return v1.BootstrapResult{}, bootstrapInternalError("write_candidates", err)
		}
	}

	warnings = normalizeValues(warnings)

	result := v1.BootstrapResult{
		CandidateCount:      len(paths),
		IndexedStubs:        indexedStubs,
		CandidatesPersisted: persistCandidates,
		TemplateResults:     templateResults,
	}
	if persistCandidates {
		result.OutputCandidatesPath = outputPath
	}
	if len(warnings) > 0 {
		result.Warnings = warnings
	}
	return result, nil
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

func buildHealthChecks(candidates []core.CandidatePointer, memories []core.ActiveMemory, unindexedPaths []string, includeDetails bool, maxFindings int) []v1.HealthCheckItem {
	checks := []v1.HealthCheckItem{
		healthCheckItem("duplicate_labels", "warn", duplicateLabelFindings(candidates), includeDetails, maxFindings),
		healthCheckItem("empty_descriptions", "warn", emptyDescriptionFindings(candidates), includeDetails, maxFindings),
		healthCheckItem("orphan_relations", "info", []string{}, includeDetails, maxFindings),
		healthCheckItem("pending_quarantines", "info", []string{}, includeDetails, maxFindings),
		healthCheckItem("stale_pointers", "warn", stalePointerFindings(candidates), includeDetails, maxFindings),
		healthCheckItem("unindexed_files", "warn", normalizeValues(unindexedPaths), includeDetails, maxFindings),
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

func loadEvalSuite(payload v1.EvalPayload) ([]v1.EvalCase, error) {
	if len(payload.EvalSuiteInline) > 0 {
		return normalizeAndValidateEvalSuite(payload.EvalSuiteInline)
	}

	suitePath := strings.TrimSpace(payload.EvalSuitePath)
	if suitePath == "" {
		return nil, fmt.Errorf("evaluation suite source is required")
	}

	content, err := os.ReadFile(suitePath)
	if err != nil {
		return nil, fmt.Errorf("read eval suite path: %w", err)
	}

	var inline []v1.EvalCase
	if err := json.Unmarshal(content, &inline); err == nil {
		if len(inline) == 0 {
			return nil, fmt.Errorf("evaluation suite file is empty")
		}
		return normalizeAndValidateEvalSuite(inline)
	}

	var wrapped struct {
		Cases []v1.EvalCase `json:"cases"`
	}
	if err := json.Unmarshal(content, &wrapped); err != nil {
		return nil, fmt.Errorf("parse eval suite path: %w", err)
	}
	if len(wrapped.Cases) == 0 {
		return nil, fmt.Errorf("evaluation suite file has no cases")
	}
	return normalizeAndValidateEvalSuite(wrapped.Cases)
}

func normalizeAndValidateEvalSuite(cases []v1.EvalCase) ([]v1.EvalCase, error) {
	normalized := make([]v1.EvalCase, 0, len(cases))
	for i := range cases {
		current := v1.EvalCase{
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

func expectedEvalArtifacts(testCase v1.EvalCase) map[string]struct{} {
	expected := make(map[string]struct{}, len(testCase.ExpectedPointerKeys)+len(testCase.ExpectedMemorySubjects))
	for _, key := range normalizeValues(testCase.ExpectedPointerKeys) {
		expected["pointer:"+key] = struct{}{}
	}
	for _, subject := range normalizeValues(testCase.ExpectedMemorySubjects) {
		expected["memory:"+subject] = struct{}{}
	}
	return expected
}

func predictedEvalArtifacts(result v1.GetContextResult) map[string]struct{} {
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

func evalCaseNote(status string) string {
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

func effectiveRespectGitIgnore(respectGitIgnore *bool) bool {
	if respectGitIgnore == nil {
		return defaultBootstrapRespectGit
	}
	return *respectGitIgnore
}

func effectivePersistCandidates(persistCandidates *bool) bool {
	if persistCandidates == nil {
		return defaultBootstrapPersist
	}
	return *persistCandidates
}

func (s *Service) collectBootstrapPaths(ctx context.Context, projectRoot, outputPath, rulesFile, tagsFile string, respectGitIgnore bool) ([]string, []string, error) {
	excludedPaths := bootstrapManagedRelativePaths(projectRoot, outputPath, rulesFile, tagsFile)
	warnings := make([]string, 0)

	if respectGitIgnore {
		gitOutput, err := s.runGit(ctx, projectRoot, "ls-files", "--cached", "--others", "--exclude-standard")
		if err == nil {
			return filterBootstrapPaths(parseBootstrapGitPaths(gitOutput), excludedPaths), warnings, nil
		}
		warnings = append(warnings, "respect_gitignore fallback to filesystem walk")
	}

	paths, walkWarnings, err := collectBootstrapPathsFromWalk(ctx, projectRoot)
	if err != nil {
		return nil, nil, err
	}
	warnings = append(warnings, walkWarnings...)
	return filterBootstrapPaths(paths, excludedPaths), warnings, nil
}

func parseBootstrapGitPaths(output string) []string {
	lines := strings.Split(output, "\n")
	paths := make([]string, 0, len(lines))
	for _, line := range lines {
		normalized := normalizeCompletionPath(line)
		if normalized == "" {
			continue
		}
		if isManagedProjectPath(normalized) {
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
			switch entry.Name() {
			case ".git", ".acm":
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
		if isManagedProjectPath(normalized) {
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

func bootstrapManagedRelativePaths(projectRoot, outputPath, rulesFile, tagsFile string) []string {
	managed := make([]string, 0, len(canonicalRulesetDefaultPaths)+9)
	if relativeOutputPath := bootstrapOutputRelativePath(projectRoot, outputPath); relativeOutputPath != "" {
		managed = append(managed, relativeOutputPath)
	}
	managed = append(managed, ".gitignore", workspace.DotEnvExampleFileName)
	managed = append(managed, canonicalTagsDefaultFilePath)
	managed = append(managed, canonicalRulesetDefaultPaths...)
	managed = append(managed, workflowDefinitionsPrimarySourcePath, workflowDefinitionsSecondarySourcePath)
	managed = append(managed, verifyTestsPrimarySourcePath, verifyTestsSecondarySourcePath)
	if relativeRulesPath := bootstrapManagedRelativePath(projectRoot, rulesFile); relativeRulesPath != "" {
		managed = append(managed, relativeRulesPath)
	}
	if relativeTagsPath := bootstrapManagedRelativePath(projectRoot, tagsFile); relativeTagsPath != "" {
		managed = append(managed, relativeTagsPath)
	}
	return normalizeCompletionPaths(managed)
}

func bootstrapManagedRelativePath(projectRoot, rawPath string) string {
	trimmed := strings.TrimSpace(rawPath)
	if trimmed == "" {
		return ""
	}
	if filepath.IsAbs(trimmed) {
		return bootstrapOutputRelativePath(projectRoot, trimmed)
	}
	return normalizeCompletionPath(trimmed)
}

func filterBootstrapPaths(paths []string, excludedPaths []string) []string {
	excluded := map[string]struct{}{}
	for _, excludedPath := range excludedPaths {
		if trimmedExcludedPath := strings.TrimSpace(excludedPath); trimmedExcludedPath != "" {
			excluded[trimmedExcludedPath] = struct{}{}
		}
	}

	filtered := make([]string, 0, len(paths))
	for _, candidatePath := range paths {
		if isManagedProjectPath(candidatePath) {
			continue
		}
		if _, skip := excluded[candidatePath]; skip {
			continue
		}
		filtered = append(filtered, candidatePath)
	}
	return filtered
}

func mergeBootstrapTemplateCandidatePaths(existing []string, additional []string, excluded []string) []string {
	merged := append([]string(nil), existing...)
	merged = append(merged, additional...)
	return filterBootstrapPaths(normalizeCompletionPaths(merged), excluded)
}

func isManagedProjectPath(raw string) bool {
	normalized := normalizeCompletionPath(raw)
	if normalized == "" {
		return false
	}

	switch normalized {
	case ".gitignore",
		workspace.DotEnvFileName,
		workspace.DotEnvExampleFileName,
		canonicalRulesetSecondarySourcePath,
		workflowDefinitionsSecondarySourcePath,
		verifyTestsSecondarySourcePath:
		return true
	}

	return normalized == ".acm" || strings.HasPrefix(normalized, ".acm/")
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

func evalInternalError(operation string, err error) *core.APIError {
	return core.NewError(
		"INTERNAL_ERROR",
		"failed to run eval suite",
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

func derefString(value *string) string {
	if value == nil {
		return ""
	}
	return strings.TrimSpace(*value)
}
