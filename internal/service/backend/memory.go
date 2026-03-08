package backend

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"

	"github.com/bonztm/agent-context-manager/internal/contracts/v1"
	"github.com/bonztm/agent-context-manager/internal/core"
)

func (s *Service) ProposeMemory(ctx context.Context, payload v1.ProposeMemoryPayload) (v1.ProposeMemoryResult, *core.APIError) {
	if s == nil || s.repo == nil {
		return v1.ProposeMemoryResult{}, core.NewError("INTERNAL_ERROR", "service repository is not configured", nil)
	}

	tagNormalizer, err := s.loadCanonicalTagNormalizer(s.defaultProjectRoot(), payload.TagsFile)
	if err != nil {
		return v1.ProposeMemoryResult{}, proposeMemoryInternalError("load_canonical_tags", err)
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

	normalizedMemory := normalizeProposedMemory(payload.Memory, tagNormalizer)
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
