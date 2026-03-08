package backend

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	bootstrapkit "github.com/bonztm/agent-context-manager/internal/bootstrap"
	"github.com/bonztm/agent-context-manager/internal/contracts/v1"
	"github.com/bonztm/agent-context-manager/internal/core"
	"github.com/bonztm/agent-context-manager/internal/workspace"
)

var statusTemplateIDs = []string{
	"claude-command-pack",
	"claude-hooks",
	"detailed-planning-enforcement",
	"git-hooks-precommit",
	"starter-contract",
	"verify-generic",
	"verify-go",
	"verify-python",
	"verify-rust",
	"verify-ts",
}

func (s *Service) Status(ctx context.Context, payload v1.StatusPayload) (v1.StatusResult, *core.APIError) {
	if s == nil || s.repo == nil {
		return v1.StatusResult{}, core.NewError("INTERNAL_ERROR", "service repository is not configured", nil)
	}

	projectRoot := s.effectiveProjectRoot(payload.ProjectRoot)
	detectedRoot := workspace.DetectRoot(projectRoot)

	result := v1.StatusResult{
		Project: v1.StatusProject{
			ProjectID:              strings.TrimSpace(payload.ProjectID),
			ProjectRoot:            projectRoot,
			DetectedRepoRoot:       strings.TrimSpace(detectedRoot.Path),
			Backend:                firstNonEmpty(strings.TrimSpace(s.runtimeStatus.Backend), "unknown"),
			PostgresConfigured:     s.runtimeStatus.PostgresConfigured,
			SQLitePath:             strings.TrimSpace(s.runtimeStatus.SQLitePath),
			UsesImplicitSQLitePath: s.runtimeStatus.UsesImplicitSQLitePath,
			Unbounded:              effectiveUnbounded(nil),
		},
	}

	if !detectedRoot.IsRepo {
		result.Missing = append(result.Missing, v1.StatusMissingItem{
			Code:    "project_root_not_repo",
			Message: "project root is not inside a git repository",
		})
	}

	ruleSources, ruleMissing := s.statusRules(projectRoot, payload.RulesFile, payload.TagsFile, payload.ProjectID)
	tagSource, tagMissing := s.statusTags(projectRoot, payload.TagsFile)
	testSource, testMissing := s.statusTests(projectRoot, payload.TestsFile, payload.TagsFile, payload.ProjectID)
	workflowSource, workflowMissing := s.statusWorkflows(projectRoot, payload.WorkflowsFile, payload.TagsFile)

	result.Sources = append(result.Sources, ruleSources...)
	result.Sources = append(result.Sources, tagSource)
	result.Sources = append(result.Sources, testSource)
	result.Sources = append(result.Sources, workflowSource)
	result.Missing = append(result.Missing, ruleMissing...)
	result.Missing = append(result.Missing, tagMissing...)
	result.Missing = append(result.Missing, testMissing...)
	result.Missing = append(result.Missing, workflowMissing...)

	sort.Slice(result.Sources, func(i, j int) bool {
		if result.Sources[i].Kind != result.Sources[j].Kind {
			return result.Sources[i].Kind < result.Sources[j].Kind
		}
		return result.Sources[i].SourcePath < result.Sources[j].SourcePath
	})

	integrations, err := statusIntegrations(projectRoot)
	if err != nil {
		result.Missing = append(result.Missing, v1.StatusMissingItem{
			Code:    "template_catalog_unavailable",
			Message: fmt.Sprintf("bootstrap template catalog is unavailable: %v", err),
		})
	} else {
		result.Integrations = integrations
	}

	if strings.TrimSpace(payload.TaskText) != "" {
		retrieval := s.statusRetrievalPreview(ctx, payload, projectRoot)
		result.Retrieval = &retrieval
		if retrieval.Error != "" {
			result.Missing = append(result.Missing, v1.StatusMissingItem{
				Code:    "retrieval_preview_error",
				Message: retrieval.Error,
			})
		}
	}

	result.Missing = dedupeStatusMissing(result.Missing)
	result.Summary = v1.StatusSummary{
		Ready:        len(result.Missing) == 0,
		MissingCount: len(result.Missing),
	}
	return result, nil
}

func (s *Service) statusRules(projectRoot, rulesFile, tagsFile, projectID string) ([]v1.StatusSource, []v1.StatusMissingItem) {
	sources, err := discoverCanonicalRulesetSources(projectRoot, rulesFile)
	if err != nil {
		return []v1.StatusSource{{
				Kind:   "rules",
				Loaded: false,
				Notes:  []string{err.Error()},
			}},
			[]v1.StatusMissingItem{{
				Code:    "rules_source_invalid",
				Message: err.Error(),
			}}
	}

	tagNormalizer, tagErr := s.loadCanonicalTagNormalizer(projectRoot, tagsFile)
	items := make([]v1.StatusSource, 0, len(sources))
	missing := make([]v1.StatusMissingItem, 0)
	totalRules := 0
	for _, source := range sources {
		item := v1.StatusSource{
			Kind:         "rules",
			SourcePath:   source.SourcePath,
			AbsolutePath: source.AbsolutePath,
			Exists:       source.Exists,
		}
		switch {
		case !source.Exists:
		case tagErr != nil:
			item.Notes = []string{fmt.Sprintf("load canonical tags: %v", tagErr)}
		default:
			rules, parseErr := parseCanonicalRulesetFile(source, strings.TrimSpace(projectID), tagNormalizer)
			if parseErr != nil {
				item.Notes = []string{parseErr.Error()}
			} else {
				item.Loaded = true
				item.ItemCount = len(rules)
				totalRules += len(rules)
			}
		}
		items = append(items, item)
	}

	if totalRules == 0 {
		switch {
		case len(sources) == 0:
			missing = append(missing, v1.StatusMissingItem{Code: "rules_missing", Message: "no canonical rules files were discovered"})
		case anyStatusSourceExists(items):
			missing = append(missing, v1.StatusMissingItem{Code: "rules_unloaded", Message: "canonical rules files exist but could not be loaded"})
		default:
			paths := make([]string, 0, len(items))
			for _, item := range items {
				if item.SourcePath != "" {
					paths = append(paths, item.SourcePath)
				}
			}
			missing = append(missing, v1.StatusMissingItem{
				Code:    "rules_missing",
				Message: fmt.Sprintf("no canonical rules files were found at %s", strings.Join(paths, " or ")),
			})
		}
	}

	return items, missing
}

func (s *Service) statusTags(projectRoot, tagsFile string) (v1.StatusSource, []v1.StatusMissingItem) {
	source, err := discoverCanonicalTagsSource(projectRoot, tagsFile)
	if err != nil {
		return v1.StatusSource{
				Kind:   "tags",
				Loaded: false,
				Notes:  []string{err.Error()},
			}, []v1.StatusMissingItem{{
				Code:    "tags_source_invalid",
				Message: err.Error(),
			}}
	}

	item := v1.StatusSource{
		Kind:         "tags",
		SourcePath:   source.SourcePath,
		AbsolutePath: source.AbsolutePath,
		Exists:       source.Exists,
	}
	if !source.Exists {
		return item, []v1.StatusMissingItem{{
			Code:    "tags_missing",
			Message: fmt.Sprintf("repo-local canonical tags file is missing at %s", source.SourcePath),
		}}
	}

	document, err := parseCanonicalTagsFile(source)
	if err != nil {
		item.Notes = []string{err.Error()}
		return item, []v1.StatusMissingItem{{
			Code:    "tags_invalid",
			Message: err.Error(),
		}}
	}
	item.Loaded = true
	item.ItemCount = len(document.CanonicalTags)
	return item, nil
}

func (s *Service) statusTests(projectRoot, testsFile, tagsFile, projectID string) (v1.StatusSource, []v1.StatusMissingItem) {
	source, err := discoverVerifyTestsSource(projectRoot, testsFile)
	if err != nil {
		return v1.StatusSource{
				Kind:   "tests",
				Loaded: false,
				Notes:  []string{err.Error()},
			}, []v1.StatusMissingItem{{
				Code:    "tests_source_invalid",
				Message: err.Error(),
			}}
	}

	item := v1.StatusSource{
		Kind:         "tests",
		SourcePath:   source.SourcePath,
		AbsolutePath: source.AbsolutePath,
		Exists:       source.Exists,
	}
	if !source.Exists {
		return item, []v1.StatusMissingItem{{
			Code:    "tests_missing",
			Message: fmt.Sprintf("verification definitions file is missing at %s", source.SourcePath),
		}}
	}

	definitions, loadedSource, err := s.loadVerifyDefinitions(projectRoot, testsFile, tagsFile)
	if loadedSource.SourcePath != "" {
		item.SourcePath = loadedSource.SourcePath
		item.AbsolutePath = loadedSource.AbsolutePath
		item.Exists = loadedSource.Exists
	}
	if err != nil {
		item.Notes = []string{err.Error()}
		return item, []v1.StatusMissingItem{{
			Code:    "tests_invalid",
			Message: err.Error(),
		}}
	}
	item.Loaded = true
	item.ItemCount = len(definitions)
	if len(definitions) == 0 {
		return item, []v1.StatusMissingItem{{
			Code:    "tests_empty",
			Message: "verification definitions loaded but no tests are configured",
		}}
	}
	return item, nil
}

func (s *Service) statusWorkflows(projectRoot, workflowsFile, tagsFile string) (v1.StatusSource, []v1.StatusMissingItem) {
	source, err := discoverWorkflowDefinitionsSource(projectRoot, workflowsFile)
	if err != nil {
		return v1.StatusSource{
				Kind:   "workflows",
				Loaded: false,
				Notes:  []string{err.Error()},
			}, []v1.StatusMissingItem{{
				Code:    "workflows_source_invalid",
				Message: err.Error(),
			}}
	}

	item := v1.StatusSource{
		Kind:         "workflows",
		SourcePath:   source.SourcePath,
		AbsolutePath: source.AbsolutePath,
		Exists:       source.Exists,
	}
	if !source.Exists {
		return item, []v1.StatusMissingItem{{
			Code:    "workflows_missing",
			Message: fmt.Sprintf("workflow definitions file is missing at %s", source.SourcePath),
		}}
	}

	definitions, loadedSource, err := s.loadWorkflowCompletionRequirements(projectRoot, workflowsFile, tagsFile)
	if loadedSource.SourcePath != "" {
		item.SourcePath = loadedSource.SourcePath
		item.AbsolutePath = loadedSource.AbsolutePath
		item.Exists = loadedSource.Exists
	}
	if err != nil {
		item.Notes = []string{err.Error()}
		return item, []v1.StatusMissingItem{{
			Code:    "workflows_invalid",
			Message: err.Error(),
		}}
	}
	item.Loaded = true
	item.ItemCount = len(definitions)
	return item, nil
}

func (s *Service) statusRetrievalPreview(ctx context.Context, payload v1.StatusPayload, projectRoot string) v1.StatusRetrieval {
	taskText := strings.TrimSpace(payload.TaskText)
	phase := payload.Phase
	if phase == "" {
		phase = v1.PhaseExecute
	}
	result := v1.StatusRetrieval{
		TaskText: taskText,
		Phase:    phase,
		Status:   "unavailable",
	}

	tagNormalizer, err := s.loadCanonicalTagNormalizer(projectRoot, payload.TagsFile)
	if err != nil {
		result.Error = fmt.Sprintf("load canonical tags: %v", err)
		return result
	}

	unbounded := effectiveUnbounded(nil)
	caps := normalizeCaps(nil)
	if unbounded {
		caps = applyUnboundedCaps(caps)
	}
	fallbackMode := fallbackWidenOnce
	diagnostics := &v1.GetContextDiagnostics{FallbackMode: fallbackMode}

	selected, err := s.selectPointers(ctx, v1.GetContextPayload{
		ProjectID: payload.ProjectID,
		TaskText:  taskText,
		Phase:     phase,
		TagsFile:  payload.TagsFile,
	}, caps, unbounded, false, tagNormalizer)
	if err != nil {
		result.Error = fmt.Sprintf("select pointers: %v", err)
		return result
	}
	diagnostics.InitialPointerCount = len(selected.Pointers)

	if len(selected.Pointers) < caps.MinPointerCount {
		diagnostics.FallbackUsed = true
		selected, err = s.selectPointers(ctx, v1.GetContextPayload{
			ProjectID: payload.ProjectID,
			TaskText:  taskText,
			Phase:     phase,
			TagsFile:  payload.TagsFile,
		}, caps, unbounded, true, tagNormalizer)
		if err != nil {
			result.Error = fmt.Sprintf("select pointers fallback: %v", err)
			return result
		}
	}

	result.Diagnostics = diagnostics
	if len(selected.Pointers) < caps.MinPointerCount {
		result.Status = "insufficient_context"
		result.SelectedPointers = statusSelectedPointers(selected.Pointers)
		return result
	}

	activeMemories, err := s.fetchMemories(ctx, payload.ProjectID, selected.PointerKeys, selected.PointerTags, caps.MaxMemories, unbounded)
	if err != nil {
		result.Error = fmt.Sprintf("fetch memories: %v", err)
		return result
	}

	rulesSelected, suggestionsSelected := splitSelectedPointers(selected.Pointers)
	rules := makeContextRules(rulesSelected)
	suggestions := makeContextSuggestions(suggestionsSelected)
	memories := makeContextMemories(activeMemories)
	resolvedTags := resolveTags(selected.PointerTags, activeMemories, tagNormalizer)
	budget := estimateBudget(caps.WordBudgetLimit, taskText, resolvedTags, rules, suggestions, memories)
	receiptID := deterministicReceiptID(v1.GetContextPayload{
		ProjectID: payload.ProjectID,
		TaskText:  taskText,
		Phase:     phase,
	}, resolvedTags, rules, suggestions, memories, budget)
	plans := s.makeContextPlans(ctx, payload.ProjectID, receiptID, unbounded)

	result.Status = "ok"
	result.ResolvedTags = resolvedTags
	result.RuleCount = len(rules)
	result.SuggestionCount = len(suggestions)
	result.MemoryCount = len(memories)
	result.PlanCount = len(plans)
	result.SelectedPointers = statusSelectedPointers(selected.Pointers)
	return result
}

func statusSelectedPointers(selected []selectedPointer) []v1.StatusRetrievalSelection {
	if len(selected) == 0 {
		return nil
	}
	out := make([]v1.StatusRetrievalSelection, 0, len(selected))
	for _, entry := range selected {
		out = append(out, v1.StatusRetrievalSelection{
			Key:     entry.Pointer.Key,
			Kind:    strings.TrimSpace(entry.Pointer.Kind),
			IsRule:  entry.Pointer.IsRule,
			Reasons: append([]string(nil), entry.Why...),
		})
	}
	return out
}

func statusIntegrations(projectRoot string) ([]v1.StatusIntegration, error) {
	templates, err := bootstrapkit.ResolveTemplates(statusTemplateIDs)
	if err != nil {
		return nil, err
	}

	out := make([]v1.StatusIntegration, 0, len(templates))
	for _, template := range templates {
		expectedTargets := make([]string, 0, len(template.Operations))
		missingTargets := make([]string, 0, len(template.Operations))
		presentTargets := 0
		for _, op := range template.Operations {
			target := strings.TrimSpace(op.Target)
			if target == "" {
				continue
			}
			expectedTargets = append(expectedTargets, target)
			if _, err := os.Stat(filepath.Join(projectRoot, filepath.FromSlash(target))); err == nil {
				presentTargets++
				continue
			}
			missingTargets = append(missingTargets, target)
		}

		out = append(out, v1.StatusIntegration{
			ID:              template.ID,
			Summary:         strings.TrimSpace(template.Summary),
			Installed:       len(expectedTargets) > 0 && presentTargets == len(expectedTargets),
			PresentTargets:  presentTargets,
			ExpectedTargets: len(expectedTargets),
			MissingTargets:  missingTargets,
		})
	}

	sort.Slice(out, func(i, j int) bool {
		return out[i].ID < out[j].ID
	})
	return out, nil
}

func anyStatusSourceExists(items []v1.StatusSource) bool {
	for _, item := range items {
		if item.Exists {
			return true
		}
	}
	return false
}

func dedupeStatusMissing(items []v1.StatusMissingItem) []v1.StatusMissingItem {
	if len(items) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(items))
	out := make([]v1.StatusMissingItem, 0, len(items))
	for _, item := range items {
		key := strings.TrimSpace(item.Code) + "\x00" + strings.TrimSpace(item.Message)
		if key == "\x00" {
			continue
		}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, item)
	}
	return out
}
