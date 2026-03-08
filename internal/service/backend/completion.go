package backend

import (
	"context"
	"errors"
	"fmt"
	"path"
	"strings"

	bootstrapkit "github.com/bonztm/agent-context-manager/internal/bootstrap"
	"github.com/bonztm/agent-context-manager/internal/contracts/v1"
	"github.com/bonztm/agent-context-manager/internal/core"
	"github.com/bonztm/agent-context-manager/internal/workspace"
)

func (s *Service) ReportCompletion(ctx context.Context, payload v1.ReportCompletionPayload) (v1.ReportCompletionResult, *core.APIError) {
	if s == nil || s.repo == nil {
		return v1.ReportCompletionResult{}, core.NewError("INTERNAL_ERROR", "service repository is not configured", nil)
	}

	tagNormalizer, err := s.loadCanonicalTagNormalizer(s.defaultProjectRoot(), payload.TagsFile)
	if err != nil {
		return v1.ReportCompletionResult{}, reportCompletionInternalError("load_canonical_tags", err)
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
	for _, filePath := range completionScopeManagedPaths() {
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

	workflowRequirements, workflowSource, err := s.loadWorkflowCompletionRequirements(s.defaultProjectRoot(), payload.TagsFile)
	if err != nil {
		return v1.ReportCompletionResult{}, workflowDefinitionsAPIError(workflowSource.SourcePath, err)
	}
	requiredTaskKeys := defaultCompletionRequiredTaskKeys()
	var requiredWorkflowDefinitions []workflowRequiredTaskDefinition
	if hasConfiguredWorkflowRequiredTasks(workflowSource, workflowRequirements) {
		requiredWorkflowDefinitions = matchWorkflowRequiredTaskDefinitions(workflowRequirements, verifySelectionContext{
			Phase:        v1.Phase(strings.TrimSpace(scope.Phase)),
			Tags:         normalizeValues(scope.ResolvedTags),
			PointerKeys:  normalizeValues(scope.PointerKeys),
			FilesChanged: filesChanged,
		})
		requiredTaskKeys = make([]string, 0, len(requiredWorkflowDefinitions))
		for _, definition := range requiredWorkflowDefinitions {
			requiredTaskKeys = append(requiredTaskKeys, definition.Key)
		}
	}

	definitionOfDoneIssues, apiErr := s.evaluateDefinitionOfDoneIssues(ctx, payload.ProjectID, payload.ReceiptID, filesChanged, workItems, requiredTaskKeys, requiredWorkflowDefinitions, scope, workflowSource.SourcePath)
	if apiErr != nil {
		return v1.ReportCompletionResult{}, apiErr
	}
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
			stubs := buildAutoIndexPointerStubs(payload.ProjectID, violations, tagNormalizer)
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
	if len(definitionOfDoneIssues) > 0 && runStatus == "accepted" {
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

func completionScopeManagedPaths() []string {
	return normalizeCompletionPaths([]string{
		".gitignore",
		workspace.DotEnvExampleFileName,
		bootstrapkit.DefaultOutputCandidatesPath,
		canonicalTagsDefaultFilePath,
		canonicalRulesetPrimarySourcePath,
		canonicalRulesetSecondarySourcePath,
		workflowDefinitionsPrimarySourcePath,
		workflowDefinitionsSecondarySourcePath,
		verifyTestsPrimarySourcePath,
		verifyTestsSecondarySourcePath,
	})
}

func (s *Service) evaluateDefinitionOfDoneIssues(ctx context.Context, projectID, receiptID string, filesChanged []string, items []core.WorkItem, requiredTaskKeys []string, requiredDefinitions []workflowRequiredTaskDefinition, scope core.ReceiptScope, workflowSourcePath string) ([]string, *core.APIError) {
	normalizedFilesChanged := normalizeCompletionPaths(filesChanged)
	if len(normalizedFilesChanged) == 0 {
		return []string{"files_changed must include at least one repository-relative path"}, nil
	}

	normalizedRequiredTaskKeys := normalizeCompletionRequiredTaskKeys(requiredTaskKeys)
	if len(normalizedRequiredTaskKeys) == 0 {
		return nil, nil
	}

	normalizedItems := normalizeWorkItems(items)
	if len(normalizedItems) == 0 {
		issues := make([]string, 0, len(normalizedRequiredTaskKeys))
		for _, requiredKey := range normalizedRequiredTaskKeys {
			issues = append(issues, missingCompletionWorkItemIssue(requiredKey))
		}
		return issues, nil
	}

	statusByKey := make(map[string]string, len(normalizedItems))
	for _, item := range normalizedItems {
		statusByKey[item.ItemKey] = normalizeWorkItemStatus(item.Status)
	}

	issues := make([]string, 0, len(normalizedRequiredTaskKeys))
	for _, requiredKey := range normalizedRequiredTaskKeys {
		status, ok := statusByKey[requiredKey]
		if !ok {
			issues = append(issues, missingCompletionWorkItemIssue(requiredKey))
			continue
		}
		if status != core.WorkItemStatusComplete {
			issues = append(issues, incompleteCompletionWorkItemIssue(requiredKey, status))
		}
	}

	for _, definition := range requiredDefinitions {
		if definition.Run == nil || !definition.RerunRequiresNewFingerprint {
			continue
		}
		status, ok := statusByKey[definition.Key]
		if !ok || status != core.WorkItemStatusComplete {
			continue
		}
		attempts, apiErr := s.listReviewAttempts(ctx, projectID, receiptID, definition.Key)
		if apiErr != nil {
			return nil, apiErr
		}
		if len(attempts) == 0 {
			issues = append(issues, staleReviewCompletionWorkItemIssue(definition.Key))
			continue
		}
		fingerprint, apiErr := computeReviewFingerprint(s.defaultProjectRoot(), projectID, receiptID, definition.Key, workflowSourcePath, *definition.Run, scope)
		if apiErr != nil {
			return nil, apiErr
		}
		attempt, ok := latestReviewAttemptByFingerprint(attempts, fingerprint)
		if !ok || !attempt.Passed {
			issues = append(issues, staleReviewCompletionWorkItemIssue(definition.Key))
		}
	}

	if len(issues) == 0 {
		return nil, nil
	}
	return issues, nil
}

func missingCompletionWorkItemIssue(requiredKey string) string {
	if requiredKey == requiredVerifyTestsKey {
		return fmt.Sprintf("required verification work item is missing: %s", requiredKey)
	}
	return fmt.Sprintf("required workflow work item is missing: %s", requiredKey)
}

func incompleteCompletionWorkItemIssue(requiredKey, status string) string {
	if requiredKey == requiredVerifyTestsKey {
		return fmt.Sprintf("required verification work item is not complete: %s (status=%s)", requiredKey, status)
	}
	return fmt.Sprintf("required workflow work item is not complete: %s (status=%s)", requiredKey, status)
}

func staleReviewCompletionWorkItemIssue(requiredKey string) string {
	return fmt.Sprintf("required workflow review is stale for the current scoped fingerprint: %s", requiredKey)
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

func buildAutoIndexPointerStubs(projectID string, violations []v1.CompletionViolation, tagNormalizer canonicalTagNormalizer) []core.PointerStub {
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
		label := normalizedPath
		stubs = append(stubs, core.PointerStub{
			PointerKey:  fmt.Sprintf("%s:%s", projectID, normalizedPath),
			Path:        normalizedPath,
			Kind:        kind,
			Label:       label,
			Description: fmt.Sprintf("Auto-indexed %s pointer stub for %s. Curate label, description, and tags.", kind, normalizedPath),
			Tags:        inferPointerTagsFromPath(normalizedPath, kind, tagNormalizer),
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

func inferPointerTagsFromPath(filePath, kind string, tagNormalizer canonicalTagNormalizer) []string {
	tags := []string{"auto-indexed", kind}
	baseName := strings.TrimSuffix(path.Base(filePath), path.Ext(filePath))
	if normalized := tagNormalizer.normalizeTag(baseName); healthTagPattern.MatchString(normalized) {
		tags = append(tags, normalized)
	}
	segments := strings.Split(path.Dir(filePath), "/")
	for _, segment := range segments {
		normalized := tagNormalizer.normalizeTag(segment)
		if !healthTagPattern.MatchString(normalized) {
			continue
		}
		tags = append(tags, normalized)
	}
	return tagNormalizer.normalizeTags(tags)
}
