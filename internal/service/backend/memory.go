package backend

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/bonztm/agent-context-manager/internal/contracts/v1"
	"github.com/bonztm/agent-context-manager/internal/core"
)

func (s *Service) Memory(ctx context.Context, payload v1.MemoryCommandPayload) (v1.MemoryResult, *core.APIError) {
	if s == nil || s.repo == nil {
		return v1.MemoryResult{}, core.NewError("INTERNAL_ERROR", "service repository is not configured", nil)
	}

	receiptID, planKey, apiErr := resolveReceiptPlanSelection(payload.ProjectID, payload.ReceiptID, payload.PlanKey)
	if apiErr != nil {
		return v1.MemoryResult{}, apiErr
	}

	tagNormalizer, err := s.loadCanonicalTagNormalizer(s.defaultProjectRoot(), payload.TagsFile)
	if err != nil {
		return v1.MemoryResult{}, proposeMemoryInternalError("load_canonical_tags", err)
	}

	scope, err := s.repo.FetchReceiptScope(ctx, core.ReceiptScopeQuery{
		ProjectID: payload.ProjectID,
		ReceiptID: receiptID,
	})
	if err != nil {
		if errors.Is(err, core.ErrReceiptScopeNotFound) {
			return v1.MemoryResult{}, core.NewError(
				"NOT_FOUND",
				"receipt scope was not found",
				map[string]any{
					"project_id": strings.TrimSpace(payload.ProjectID),
					"receipt_id": receiptID,
				},
			)
		}
		return v1.MemoryResult{}, proposeMemoryInternalError("fetch_receipt_scope", err)
	}

	plan, apiErr := s.loadEffectiveWorkPlan(ctx, payload.ProjectID, receiptID, planKey)
	if apiErr != nil {
		return v1.MemoryResult{}, apiErr
	}

	normalizedMemory := normalizeProposedMemory(payload.Memory, tagNormalizer)
	validation, apiErr := s.validateProposedMemoryScope(ctx, payload.ProjectID, normalizedMemory, scope, plan)
	if apiErr != nil {
		return v1.MemoryResult{}, apiErr
	}
	dedupeKey := deterministicMemoryDedupeKey(normalizedMemory)
	autoPromote := effectiveAutoPromote(payload.AutoPromote)
	promotable := autoPromote && validation.HardPassed && validation.SoftPassed

	persisted, err := s.repo.PersistMemory(ctx, core.MemoryPersistence{
		ProjectID:           payload.ProjectID,
		ReceiptID:           receiptID,
		Category:            strings.TrimSpace(string(normalizedMemory.Category)),
		Subject:             normalizedMemory.Subject,
		Content:             normalizedMemory.Content,
		Confidence:          normalizedMemory.Confidence,
		Tags:                append([]string(nil), normalizedMemory.Tags...),
		RelatedPointerKeys:  append([]string(nil), normalizedMemory.RelatedPointerKeys...),
		EvidencePointerKeys: append([]string(nil), normalizedMemory.EvidencePointerKeys...),
		DedupeKey:           dedupeKey,
		Validation: core.MemoryValidation{
			HardPassed: validation.HardPassed,
			SoftPassed: validation.SoftPassed,
			Errors:     append([]string(nil), validation.Errors...),
			Warnings:   append([]string(nil), validation.Warnings...),
		},
		AutoPromote: autoPromote,
		Promotable:  promotable,
	})
	if err != nil {
		return v1.MemoryResult{}, proposeMemoryInternalError("persist_proposed_memory", err)
	}

	result := v1.MemoryResult{
		CandidateID: int(persisted.CandidateID),
		Status:      strings.TrimSpace(persisted.Status),
		Validation:  validation,
	}
	if persisted.PromotedMemoryID > 0 {
		result.PromotedMemoryID = int(persisted.PromotedMemoryID)
	}

	return result, nil
}

func normalizeProposedMemory(memory v1.MemoryPayload, tagNormalizer canonicalTagNormalizer) v1.MemoryPayload {
	return v1.MemoryPayload{
		Category:            v1.MemoryCategory(strings.TrimSpace(string(memory.Category))),
		Subject:             strings.TrimSpace(memory.Subject),
		Content:             strings.TrimSpace(memory.Content),
		RelatedPointerKeys:  normalizeValues(memory.RelatedPointerKeys),
		Tags:                tagNormalizer.normalizeTags(memory.Tags),
		Confidence:          memory.Confidence,
		EvidencePointerKeys: normalizeValues(memory.EvidencePointerKeys),
	}
}

func (s *Service) validateProposedMemoryScope(ctx context.Context, projectID string, memory v1.MemoryPayload, scope core.ReceiptScope, plan *core.WorkPlan) (v1.MemoryValidation, *core.APIError) {
	exactPointerScope := make(map[string]struct{}, len(scope.PointerKeys))
	for _, key := range normalizeValues(scope.PointerKeys) {
		exactPointerScope[key] = struct{}{}
	}

	pathScope := make(map[string]struct{})
	for _, filePath := range effectiveScopePaths(scope, plan) {
		pathScope[filePath] = struct{}{}
	}

	errorsList := make([]string, 0, 2)
	warnings := make([]string, 0, 1)

	if len(memory.EvidencePointerKeys) == 0 {
		errorsList = append(errorsList, "memory.evidence_pointer_keys must not be empty after normalization")
	} else if missingEvidence, apiErr := s.pointerKeysOutsideEffectiveScope(ctx, projectID, memory.EvidencePointerKeys, exactPointerScope, pathScope); apiErr != nil {
		return v1.MemoryValidation{}, apiErr
	} else if len(missingEvidence) > 0 {
		errorsList = append(errorsList, "memory.evidence_pointer_keys outside effective scope: "+strings.Join(missingEvidence, ", "))
	}

	if missingRelated, apiErr := s.pointerKeysOutsideEffectiveScope(ctx, projectID, memory.RelatedPointerKeys, exactPointerScope, pathScope); apiErr != nil {
		return v1.MemoryValidation{}, apiErr
	} else if len(missingRelated) > 0 {
		warnings = append(warnings, "memory.related_pointer_keys outside effective scope: "+strings.Join(missingRelated, ", "))
	}

	return v1.MemoryValidation{
		HardPassed: len(errorsList) == 0,
		SoftPassed: len(warnings) == 0,
		Errors:     nonNilStrings(errorsList),
		Warnings:   nonNilStrings(warnings),
	}, nil
}

func (s *Service) pointerKeysOutsideEffectiveScope(ctx context.Context, projectID string, pointerKeys []string, exactPointerScope, pathScope map[string]struct{}) ([]string, *core.APIError) {
	if len(pointerKeys) == 0 {
		return nil, nil
	}

	out := make([]string, 0, len(pointerKeys))
	for _, key := range pointerKeys {
		withinScope, apiErr := s.pointerKeyWithinEffectiveScope(ctx, projectID, key, exactPointerScope, pathScope)
		if apiErr != nil {
			return nil, apiErr
		}
		if withinScope {
			continue
		}
		out = append(out, key)
	}
	return out, nil
}

func (s *Service) pointerKeyWithinEffectiveScope(ctx context.Context, projectID, pointerKey string, exactPointerScope, pathScope map[string]struct{}) (bool, *core.APIError) {
	normalizedKey := strings.TrimSpace(pointerKey)
	if normalizedKey == "" {
		return false, nil
	}
	if _, ok := exactPointerScope[normalizedKey]; ok {
		return true, nil
	}
	if s == nil || s.repo == nil {
		return false, core.NewError("INTERNAL_ERROR", "service repository is not configured", nil)
	}

	pointer, err := s.repo.LookupPointerByKey(ctx, core.PointerLookupQuery{
		ProjectID:  strings.TrimSpace(projectID),
		PointerKey: normalizedKey,
	})
	if err != nil {
		if errors.Is(err, core.ErrPointerLookupNotFound) {
			return false, nil
		}
		return false, core.NewError(
			"INTERNAL_ERROR",
			"failed to validate memory pointer scope",
			map[string]any{
				"project_id":  strings.TrimSpace(projectID),
				"pointer_key": normalizedKey,
				"error":       err.Error(),
			},
		)
	}

	normalizedPath := normalizePointerScopePath(s.defaultProjectRoot(), pointer.Path)
	if normalizedPath == "" {
		return false, nil
	}
	_, ok := pathScope[normalizedPath]
	return ok, nil
}

func normalizePointerScopePath(projectRoot, raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return ""
	}
	normalizedSlashes := strings.ReplaceAll(trimmed, "\\", "/")
	if strings.HasPrefix(normalizedSlashes, "/") || isWindowsAbsolutePath(normalizedSlashes) {
		root := filepath.Clean(strings.TrimSpace(projectRoot))
		if root == "" {
			return ""
		}
		relativePath, err := filepath.Rel(root, filepath.Clean(trimmed))
		if err != nil {
			return ""
		}
		return normalizeCompletionPath(filepath.ToSlash(relativePath))
	}
	return normalizeCompletionPath(trimmed)
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
		"failed to record memory",
		map[string]any{
			"operation": operation,
			"error":     err.Error(),
		},
	)
}
