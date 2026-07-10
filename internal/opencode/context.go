// Package opencode renders the bounded context plan consumed by ACM's plugin.
package opencode

import (
	"errors"
	"fmt"
	"strings"

	"github.com/bonztm/agent-context-manager/internal/core"
	"github.com/bonztm/agent-context-manager/internal/engine"
)

const (
	// SyntheticPrefix marks context that capture must never re-index.
	SyntheticPrefix = "[Archived by acm:"
	planVersion     = 1
	maxWindowItems  = 4096
	maxSummaryRoots = 128
	maxSummaryChars = 12_000
	maxRecallChars  = 20_000
	maxResumeChars  = 4_000
	maxRootChars    = 600
)

// Plan is the versioned JSON contract between the binary and embedded plugin.
type Plan struct {
	Version            int      `json:"version"`
	FreshTailMessages  int      `json:"fresh_tail_messages"`
	FreshMessageIDs    []string `json:"fresh_message_ids"`
	SummaryRefs        []string `json:"summary_refs"`
	ArchivePlaceholder string   `json:"archive_placeholder,omitempty"`
	SummaryText        string   `json:"summary_text,omitempty"`
	RecallText         string   `json:"recall_text,omitempty"`
	ResumeNote         string   `json:"resume_note,omitempty"`
}

// BuildPlan converts ACM's assembled window into a bounded plugin contract.
func BuildPlan(items []engine.RenderedItem, externalIDs map[string]string, recall string, freshTailMessages int) (Plan, error) {
	if len(items) > maxWindowItems || freshTailMessages < 0 || freshTailMessages > maxWindowItems {
		return Plan{}, errors.New("opencode: invalid context plan bounds")
	}
	plan := Plan{Version: planVersion, FreshTailMessages: freshTailMessages, RecallText: truncate(recall, maxRecallChars)}
	rootLines := make([]string, 0, min(len(items), maxSummaryRoots))
	for _, item := range items {
		switch item.Type {
		case core.ContextMessage:
			if externalID := externalIDs[item.RefID]; externalID != "" {
				plan.FreshMessageIDs = append(plan.FreshMessageIDs, externalID)
			}
		case core.ContextSummary:
			if len(plan.SummaryRefs) == maxSummaryRoots {
				return Plan{}, fmt.Errorf("opencode: active summary roots exceed %d", maxSummaryRoots)
			}
			plan.SummaryRefs = append(plan.SummaryRefs, item.RefID)
			rootLines = append(rootLines, fmt.Sprintf("- %s (seq %d-%d): %s",
				item.RefID, item.EarliestSeq, item.LatestSeq, truncate(item.Content, maxRootChars)))
		default:
			return Plan{}, fmt.Errorf("opencode: unknown context item type %q", item.Type)
		}
	}
	if len(plan.SummaryRefs) > 0 {
		plan.ArchivePlaceholder = archivePlaceholder(plan.SummaryRefs)
		plan.SummaryText = summaryText(plan.SummaryRefs, rootLines)
		plan.ResumeNote = resumeNote(plan.SummaryRefs, rootLines)
	}
	return plan, nil
}

func archivePlaceholder(refs []string) string {
	return fmt.Sprintf("%s older message elided; active summaries %s. Recover with acm expand <sum-id>.]",
		SyntheticPrefix, strings.Join(refs, ", "))
}

func summaryText(refs, rootLines []string) string {
	text := fmt.Sprintf("%s older context is represented by %d active summary nodes.]\nSummary roots:\n%s\nUse acm expand <sum-id> for verbatim sources.",
		SyntheticPrefix, len(refs), strings.Join(rootLines, "\n"))
	return truncate(text, maxSummaryChars)
}

func resumeNote(refs, rootLines []string) string {
	text := fmt.Sprintf("%s resume note; active summaries %s.]\n%s\nUse acm expand <sum-id>, acm describe <msg-id>, or acm grep <pattern> for details.",
		SyntheticPrefix, strings.Join(refs, ", "), strings.Join(rootLines, "\n"))
	return truncate(text, maxResumeChars)
}

func truncate(value string, limit int) string {
	runes := []rune(value)
	if len(runes) <= limit {
		return value
	}
	return string(runes[:limit-3]) + "..."
}
