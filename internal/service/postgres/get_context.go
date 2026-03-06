package postgres

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/bonztm/agent-context-manager/internal/contracts/v1"
	"github.com/bonztm/agent-context-manager/internal/core"
)

const (
	defaultMaxNonRulePointers = 8
	defaultMaxRulePointers    = 0 // 0 means uncapped
	defaultMaxHops            = 1
	defaultMaxHopExpansion    = 5
	defaultMaxMemories        = 6
	defaultMinPointerCount    = 2
	defaultWordBudgetLimit    = 1200

	schemaMaxNonRulePointers = 32
	schemaMaxRulePointers    = 512
	schemaMaxHops            = 3
	schemaMaxHopExpansion    = 32
	schemaMaxMemories        = 32
	schemaMaxMinPointerCount = 8
	schemaMaxWordBudgetLimit = 10000

	candidateFetchLimit = 512
	fallbackWidenOnce   = "widen_once"
	fallbackNone        = "none"
)

var wordPattern = regexp.MustCompile(`[A-Za-z0-9]+(?:[._:/-][A-Za-z0-9]+)*`)

type effectiveCaps struct {
	MaxNonRulePointers int
	MaxRulePointers    int
	MaxHops            int
	MaxHopExpansion    int
	MaxMemories        int
	MinPointerCount    int
	WordBudgetLimit    int
}

type selectedPointer struct {
	Pointer core.CandidatePointer
	Why     []string
}

type pointerSelection struct {
	Pointers    []selectedPointer
	PointerKeys []string
	PointerTags []string
}

func (s *Service) GetContext(ctx context.Context, payload v1.GetContextPayload) (v1.GetContextResult, *core.APIError) {
	if s == nil || s.repo == nil {
		return v1.GetContextResult{}, core.NewError("INTERNAL_ERROR", "postgres service repository is not configured", nil)
	}

	tagNormalizer, err := s.loadCanonicalTagNormalizer("", payload.TagsFile)
	if err != nil {
		return v1.GetContextResult{}, internalError("load_canonical_tags", err)
	}

	unbounded := effectiveUnbounded(payload.Unbounded)
	caps := normalizeCaps(payload.Caps)
	if unbounded {
		caps = applyUnboundedCaps(caps)
	}
	fallbackMode := normalizeFallbackMode(payload.FallbackMode)
	diagnostics := &v1.GetContextDiagnostics{FallbackMode: fallbackMode}

	selected, err := s.selectPointers(ctx, payload, caps, unbounded, false, tagNormalizer)
	if err != nil {
		return v1.GetContextResult{}, internalError("fetch_candidate_pointers", err)
	}
	diagnostics.InitialPointerCount = len(selected.Pointers)

	if len(selected.Pointers) < caps.MinPointerCount && fallbackMode == fallbackWidenOnce {
		diagnostics.FallbackUsed = true
		selected, err = s.selectPointers(ctx, payload, caps, unbounded, true, tagNormalizer)
		if err != nil {
			return v1.GetContextResult{}, internalError("fetch_candidate_pointers_fallback", err)
		}
	}

	if len(selected.Pointers) < caps.MinPointerCount {
		return v1.GetContextResult{
			Status:      "insufficient_context",
			Diagnostics: diagnostics,
		}, nil
	}

	activeMemories, err := s.fetchMemories(ctx, payload.ProjectID, selected.PointerKeys, selected.PointerTags, caps.MaxMemories, unbounded)
	if err != nil {
		return v1.GetContextResult{}, internalError("fetch_active_memories", err)
	}

	rulesSelected, suggestionsSelected := splitSelectedPointers(selected.Pointers)
	rules := makeContextRules(rulesSelected)
	suggestions := makeContextSuggestions(suggestionsSelected)
	memories := makeContextMemories(activeMemories)
	resolvedTags := resolveTags(selected.PointerTags, activeMemories, tagNormalizer)
	budget := estimateBudget(caps.WordBudgetLimit, payload.TaskText, resolvedTags, rules, suggestions, memories)

	receiptID := deterministicReceiptID(payload, resolvedTags, rules, suggestions, memories, budget)
	plans := s.makeContextPlans(ctx, payload.ProjectID, receiptID, unbounded)

	receipt := v1.ContextReceipt{
		Rules:       rules,
		Suggestions: suggestions,
		Memories:    memories,
		Plans:       plans,
		Meta: v1.ContextReceiptMeta{
			ReceiptID:        receiptID,
			RetrievalVersion: RetrievalVersion,
			ProjectID:        payload.ProjectID,
			TaskText:         payload.TaskText,
			Phase:            payload.Phase,
			ResolvedTags:     resolvedTags,
			Budget:           budget,
		},
	}

	if err := s.repo.UpsertReceiptScope(ctx, core.ReceiptScope{
		ProjectID:    strings.TrimSpace(payload.ProjectID),
		ReceiptID:    receiptID,
		TaskText:     strings.TrimSpace(payload.TaskText),
		Phase:        strings.TrimSpace(string(payload.Phase)),
		ResolvedTags: append([]string(nil), resolvedTags...),
		PointerKeys:  append([]string(nil), selected.PointerKeys...),
		MemoryIDs:    activeMemoryIDs(activeMemories),
	}); err != nil {
		return v1.GetContextResult{}, internalError("persist_receipt_scope", err)
	}

	return v1.GetContextResult{
		Status:      "ok",
		Receipt:     &receipt,
		Diagnostics: diagnostics,
	}, nil
}

func activeMemoryIDs(memories []core.ActiveMemory) []int64 {
	if len(memories) == 0 {
		return nil
	}
	ids := make([]int64, 0, len(memories))
	for _, memory := range memories {
		if memory.ID <= 0 {
			continue
		}
		ids = append(ids, memory.ID)
	}
	if len(ids) == 0 {
		return nil
	}
	return ids
}

func (s *Service) selectPointers(ctx context.Context, payload v1.GetContextPayload, caps effectiveCaps, unbounded bool, fallback bool, tagNormalizer canonicalTagNormalizer) (pointerSelection, error) {
	taskText := strings.TrimSpace(payload.TaskText)
	queryTags := tagNormalizer.canonicalTagsFromTaskText(taskText)
	allowStale := payload.AllowStale
	if fallback {
		// Widen fallback to FTS-only while preserving the original task text and stale policy.
		queryTags = nil
	}

	candidates, err := s.repo.FetchCandidatePointers(ctx, core.CandidatePointerQuery{
		ProjectID: payload.ProjectID,
		TaskText:  taskText,
		Phase:     strings.TrimSpace(string(payload.Phase)),
		Tags:      queryTags,
		Limit:     candidateFetchLimit,
		Unbounded: unbounded,
		StaleFilter: core.StaleFilter{
			AllowStale: allowStale,
		},
	})
	if err != nil {
		return pointerSelection{}, err
	}
	candidates = filterManagedCandidatePointers(candidates)

	rules, nonRules := splitCandidatePointers(candidates)
	if !unbounded && caps.MaxRulePointers > 0 && len(rules) > caps.MaxRulePointers {
		rules = rules[:caps.MaxRulePointers]
	}
	if !unbounded && len(nonRules) > caps.MaxNonRulePointers {
		nonRules = nonRules[:caps.MaxNonRulePointers]
	}

	selection := pointerSelection{Pointers: make([]selectedPointer, 0, len(rules)+len(nonRules)+caps.MaxHopExpansion)}
	seen := make(map[string]struct{}, len(rules)+len(nonRules)+caps.MaxHopExpansion)
	nonRuleKeys := make([]string, 0, len(nonRules))
	for _, pointer := range rules {
		if addUniquePointer(&selection.Pointers, seen, pointer, []string{"rule pointer included"}) {
			continue
		}
	}
	for _, pointer := range nonRules {
		if addUniquePointer(&selection.Pointers, seen, pointer, []string{"non-rule candidate selected"}) {
			nonRuleKeys = append(nonRuleKeys, pointer.Key)
		}
	}

	if caps.MaxHops > 0 && caps.MaxHopExpansion > 0 && len(nonRuleKeys) > 0 {
		hops, err := s.repo.FetchRelatedHopPointers(ctx, core.RelatedHopPointersQuery{
			ProjectID:   payload.ProjectID,
			PointerKeys: nonRuleKeys,
			MaxHops:     caps.MaxHops,
			Limit:       caps.MaxHopExpansion,
			Unbounded:   unbounded,
			StaleFilter: core.StaleFilter{
				AllowStale: allowStale,
			},
		})
		if err != nil {
			return pointerSelection{}, err
		}

		expanded := 0
		for _, hop := range hops {
			if hop.HopCount < 1 || hop.HopCount > caps.MaxHops {
				continue
			}
			if shouldFilterManagedRetrievalPointer(hop.Pointer) {
				continue
			}
			why := []string{fmt.Sprintf("related %d-hop expansion", hop.HopCount)}
			source := strings.TrimSpace(hop.SourceKey)
			if source != "" {
				why = []string{fmt.Sprintf("related %d-hop from %s", hop.HopCount, source)}
			}
			if addUniquePointer(&selection.Pointers, seen, hop.Pointer, why) {
				expanded++
			}
			if !unbounded && expanded >= caps.MaxHopExpansion {
				break
			}
		}
	}

	selection.PointerKeys = make([]string, 0, len(selection.Pointers))
	tagSet := make(map[string]struct{}, len(selection.Pointers))
	for _, entry := range selection.Pointers {
		selection.PointerKeys = append(selection.PointerKeys, entry.Pointer.Key)
		for _, tag := range tagNormalizer.normalizeTags(entry.Pointer.Tags) {
			tagSet[tag] = struct{}{}
		}
	}

	selection.PointerTags = mapKeysSorted(tagSet)
	return selection, nil
}

func filterManagedCandidatePointers(candidates []core.CandidatePointer) []core.CandidatePointer {
	if len(candidates) == 0 {
		return nil
	}

	filtered := make([]core.CandidatePointer, 0, len(candidates))
	for _, candidate := range candidates {
		if shouldFilterManagedRetrievalPointer(candidate) {
			continue
		}
		filtered = append(filtered, candidate)
	}
	return filtered
}

func shouldFilterManagedRetrievalPointer(candidate core.CandidatePointer) bool {
	if !isManagedProjectPath(candidate.Path) {
		return false
	}
	if candidate.IsRule && isCanonicalRulesetSourcePath(candidate.Path) {
		return false
	}
	return true
}

func isCanonicalRulesetSourcePath(raw string) bool {
	switch normalizeCompletionPath(raw) {
	case canonicalRulesetPrimarySourcePath, canonicalRulesetSecondarySourcePath:
		return true
	default:
		return false
	}
}

func (s *Service) fetchMemories(ctx context.Context, projectID string, pointerKeys, tags []string, maxMemories int, unbounded bool) ([]core.ActiveMemory, error) {
	if maxMemories <= 0 && !unbounded {
		return nil, nil
	}

	memories, err := s.repo.FetchActiveMemories(ctx, core.ActiveMemoryQuery{
		ProjectID:   projectID,
		PointerKeys: pointerKeys,
		Tags:        tags,
		Limit:       maxMemories,
		Unbounded:   unbounded,
	})
	if err != nil {
		return nil, err
	}
	if !unbounded && len(memories) > maxMemories {
		memories = memories[:maxMemories]
	}
	return memories, nil
}

func normalizeCaps(caps *v1.RetrievalCaps) effectiveCaps {
	out := effectiveCaps{
		MaxNonRulePointers: defaultMaxNonRulePointers,
		MaxRulePointers:    defaultMaxRulePointers,
		MaxHops:            defaultMaxHops,
		MaxHopExpansion:    defaultMaxHopExpansion,
		MaxMemories:        defaultMaxMemories,
		MinPointerCount:    defaultMinPointerCount,
		WordBudgetLimit:    defaultWordBudgetLimit,
	}
	if caps == nil {
		return out
	}

	if caps.MaxNonRulePointers > 0 {
		out.MaxNonRulePointers = minInt(caps.MaxNonRulePointers, schemaMaxNonRulePointers)
	}
	if caps.MaxRulePointers > 0 {
		out.MaxRulePointers = minInt(caps.MaxRulePointers, schemaMaxRulePointers)
	}
	if caps.MaxHops > 0 {
		out.MaxHops = minInt(caps.MaxHops, schemaMaxHops)
	}
	if caps.MaxHopExpansion > 0 {
		out.MaxHopExpansion = minInt(caps.MaxHopExpansion, schemaMaxHopExpansion)
	}
	if caps.MaxMemories > 0 {
		out.MaxMemories = minInt(caps.MaxMemories, schemaMaxMemories)
	}
	if caps.MinPointerCount > 0 {
		out.MinPointerCount = minInt(caps.MinPointerCount, schemaMaxMinPointerCount)
	}
	if caps.WordBudgetLimit > 0 {
		out.WordBudgetLimit = minInt(caps.WordBudgetLimit, schemaMaxWordBudgetLimit)
	}

	return out
}

func applyUnboundedCaps(caps effectiveCaps) effectiveCaps {
	caps.MaxRulePointers = 0
	caps.MaxNonRulePointers = schemaMaxNonRulePointers
	caps.MaxHops = schemaMaxHops
	caps.MaxHopExpansion = schemaMaxHopExpansion
	caps.MaxMemories = schemaMaxMemories
	return caps
}

func normalizeFallbackMode(mode string) string {
	mode = strings.TrimSpace(mode)
	if mode == "" {
		return fallbackWidenOnce
	}
	return mode
}

func splitCandidatePointers(candidates []core.CandidatePointer) ([]core.CandidatePointer, []core.CandidatePointer) {
	rules := make([]core.CandidatePointer, 0)
	nonRules := make([]core.CandidatePointer, 0)
	for _, candidate := range candidates {
		if candidate.IsRule {
			rules = append(rules, candidate)
			continue
		}
		nonRules = append(nonRules, candidate)
	}
	return rules, nonRules
}

func splitSelectedPointers(selected []selectedPointer) ([]selectedPointer, []selectedPointer) {
	rules := make([]selectedPointer, 0, len(selected))
	suggestions := make([]selectedPointer, 0, len(selected))
	for _, entry := range selected {
		if entry.Pointer.IsRule {
			rules = append(rules, entry)
			continue
		}
		suggestions = append(suggestions, entry)
	}
	return rules, suggestions
}

func addUniquePointer(dest *[]selectedPointer, seen map[string]struct{}, pointer core.CandidatePointer, why []string) bool {
	key := strings.TrimSpace(pointer.Key)
	if key == "" {
		return false
	}
	if _, ok := seen[key]; ok {
		return false
	}
	seen[key] = struct{}{}
	*dest = append(*dest, selectedPointer{
		Pointer: pointer,
		Why:     normalizeWhy(why),
	})
	return true
}

func makeContextRules(selected []selectedPointer) []v1.ContextRule {
	out := make([]v1.ContextRule, 0, len(selected))
	for _, entry := range selected {
		ruleKey := strings.TrimSpace(entry.Pointer.Key)
		if ruleKey == "" {
			continue
		}

		summary := strings.TrimSpace(entry.Pointer.Label)
		if summary == "" {
			summary = pointerSummary(entry.Pointer)
		}

		rule := v1.ContextRule{
			RuleID:      ruleIDFromPointerKey(ruleKey),
			Key:         ruleKey,
			Summary:     summary,
			Enforcement: ruleEnforcementFromTags(entry.Pointer.Tags),
		}

		content := strings.TrimSpace(entry.Pointer.Description)
		if content != "" && content != summary {
			rule.Content = content
		}

		out = append(out, rule)
	}
	return out
}

func ruleIDFromPointerKey(pointerKey string) string {
	key := strings.TrimSpace(pointerKey)
	if key == "" {
		return ""
	}
	separator := strings.LastIndex(key, "#")
	if separator < 0 || separator >= len(key)-1 {
		return key
	}
	ruleID := strings.TrimSpace(key[separator+1:])
	if ruleID == "" {
		return key
	}
	return ruleID
}

func ruleEnforcementFromTags(tags []string) string {
	for _, tag := range normalizeCanonicalTags(tags) {
		switch tag {
		case ruleTagEnforcementSoft:
			return "soft"
		case ruleTagEnforcementHard:
			return "hard"
		}
	}
	return "required"
}

func makeContextSuggestions(selected []selectedPointer) []v1.ContextSuggestion {
	out := make([]v1.ContextSuggestion, 0, len(selected))
	for _, entry := range selected {
		out = append(out, v1.ContextSuggestion{
			Key:     entry.Pointer.Key,
			Summary: pointerSummary(entry.Pointer),
		})
	}
	return out
}

func makeContextMemories(memories []core.ActiveMemory) []v1.ContextMemory {
	out := make([]v1.ContextMemory, 0, len(memories))
	for _, memory := range memories {
		out = append(out, v1.ContextMemory{
			Key:     fmt.Sprintf("mem:%d", memory.ID),
			Summary: memorySummary(memory),
		})
	}
	return out
}

func (s *Service) makeContextPlans(ctx context.Context, projectID, receiptID string, unbounded bool) []v1.ContextPlan {
	if s != nil && s.repo != nil {
		if planRepo, ok := s.repo.(core.WorkPlanRepository); ok {
			planRows, err := planRepo.ListWorkPlans(ctx, core.WorkPlanListQuery{
				ProjectID: strings.TrimSpace(projectID),
				Scope:     string(v1.HistoryScopeCurrent),
				Limit:     8,
				Unbounded: unbounded,
			})
			if err == nil && len(planRows) > 0 {
				plans := make([]v1.ContextPlan, 0, len(planRows))
				for _, row := range planRows {
					planKey := strings.TrimSpace(row.PlanKey)
					if planKey == "" {
						continue
					}
					status := normalizePlanStatus(row.Status)
					summary := strings.TrimSpace(row.Summary)
					if summary == "" {
						summary = fmt.Sprintf("Plan %s is %s", planKey, status)
					}
					plans = append(plans, v1.ContextPlan{
						Key:       planKey,
						Summary:   summary,
						Status:    v1.WorkItemStatus(status),
						FetchKeys: contextPlanFetchKeys(planKey),
					})
				}
				if len(plans) > 0 {
					return plans
				}
			}
		}
	}

	receiptID = strings.TrimSpace(receiptID)
	if receiptID == "" {
		return nil
	}
	status := v1.WorkItemStatusPending
	return []v1.ContextPlan{{
		Key:       "plan:" + receiptID,
		Summary:   fmt.Sprintf("Plan %s is %s", receiptID, status),
		Status:    status,
		FetchKeys: contextPlanFetchKeys("plan:" + receiptID),
	}}
}

func maxZero(value int) int {
	if value < 0 {
		return 0
	}
	return value
}

func contextPlanFetchKeys(planKey string) []string {
	normalizedPlanKey := strings.TrimSpace(planKey)
	if normalizedPlanKey == "" {
		return nil
	}
	return []string{normalizedPlanKey}
}

func resolveTags(pointerTags []string, memories []core.ActiveMemory, tagNormalizer canonicalTagNormalizer) []string {
	resolved := make(map[string]struct{}, len(pointerTags)+len(memories))
	for _, tag := range tagNormalizer.normalizeTags(pointerTags) {
		resolved[tag] = struct{}{}
	}
	for _, memory := range memories {
		for _, tag := range tagNormalizer.normalizeTags(memory.Tags) {
			resolved[tag] = struct{}{}
		}
	}
	return mapKeysSorted(resolved)
}

func estimateBudget(limit int, taskText string, tags []string, rules []v1.ContextRule, suggestions []v1.ContextSuggestion, memories []v1.ContextMemory) v1.ContextBudget {
	used := countWords(taskText)
	for _, tag := range tags {
		used += countWords(tag)
	}
	for _, rule := range rules {
		used += countWords(rule.Key)
		used += countWords(rule.Summary)
		used += countWords(rule.Enforcement)
		used += countWords(rule.Content)
	}
	for _, suggestion := range suggestions {
		used += countWords(suggestion.Key)
		used += countWords(suggestion.Summary)
	}
	for _, memory := range memories {
		used += countWords(memory.Key)
		used += countWords(memory.Summary)
	}

	remaining := limit - used
	if remaining < 0 {
		remaining = 0
	}
	return v1.ContextBudget{
		Unit:      "words",
		Limit:     limit,
		Used:      used,
		Remaining: remaining,
	}
}

func deterministicReceiptID(payload v1.GetContextPayload, resolvedTags []string, rules []v1.ContextRule, suggestions []v1.ContextSuggestion, memories []v1.ContextMemory, budget v1.ContextBudget) string {
	var b strings.Builder
	b.WriteString(RetrievalVersion)
	b.WriteString("\n")
	b.WriteString(strings.TrimSpace(payload.ProjectID))
	b.WriteString("\n")
	b.WriteString(strings.TrimSpace(payload.TaskText))
	b.WriteString("\n")
	b.WriteString(string(payload.Phase))
	b.WriteString("\n")

	for _, tag := range resolvedTags {
		b.WriteString("tag:")
		b.WriteString(tag)
		b.WriteString("\n")
	}
	for _, rule := range rules {
		b.WriteString("rule:")
		b.WriteString(rule.Key)
		b.WriteString("|")
		b.WriteString(rule.Summary)
		b.WriteString("|")
		b.WriteString(rule.Enforcement)
		b.WriteString("|")
		b.WriteString(rule.Content)
		b.WriteString("\n")
	}
	for _, suggestion := range suggestions {
		b.WriteString("suggestion:")
		b.WriteString(suggestion.Key)
		b.WriteString("|")
		b.WriteString(suggestion.Summary)
		b.WriteString("\n")
	}
	for _, memory := range memories {
		b.WriteString("memory:")
		b.WriteString(memory.Key)
		b.WriteString("|")
		b.WriteString(memory.Summary)
		b.WriteString("\n")
	}
	b.WriteString(fmt.Sprintf("budget:%d|%d|%d", budget.Limit, budget.Used, budget.Remaining))

	digest := sha256.Sum256([]byte(b.String()))
	return "receipt-" + hex.EncodeToString(digest[:12])
}

func pointerSummary(pointer core.CandidatePointer) string {
	description := strings.TrimSpace(pointer.Description)
	if description == "" {
		description = strings.TrimSpace(pointer.Label)
	}
	if description == "" {
		description = strings.TrimSpace(pointer.Path)
	}
	if description == "" {
		description = strings.TrimSpace(pointer.Key)
	}
	return description
}

func memorySummary(memory core.ActiveMemory) string {
	summary := strings.TrimSpace(memory.Subject)
	if summary == "" {
		summary = strings.TrimSpace(memory.Content)
	}
	if summary == "" {
		summary = fmt.Sprintf("Memory %d", memory.ID)
	}
	return summary
}

func indexEntryVersion(parts ...string) string {
	var b strings.Builder
	for _, part := range parts {
		b.WriteString(strings.TrimSpace(part))
		b.WriteString("\n")
	}
	digest := sha256.Sum256([]byte(b.String()))
	return hex.EncodeToString(digest[:8])
}

func countWords(text string) int {
	if strings.TrimSpace(text) == "" {
		return 0
	}
	return len(wordPattern.FindAllString(text, -1))
}

func mapKeysSorted(values map[string]struct{}) []string {
	if len(values) == 0 {
		return nil
	}
	out := make([]string, 0, len(values))
	for key := range values {
		out = append(out, key)
	}
	sort.Strings(out)
	return out
}

func normalizeWhy(why []string) []string {
	if len(why) == 0 {
		return []string{"repository selection"}
	}
	out := make([]string, 0, len(why))
	for _, value := range why {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		out = append(out, value)
	}
	if len(out) == 0 {
		return []string{"repository selection"}
	}
	return out
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func internalError(operation string, err error) *core.APIError {
	return core.NewError(
		"INTERNAL_ERROR",
		"failed to resolve context",
		map[string]any{
			"operation": operation,
			"error":     err.Error(),
		},
	)
}
