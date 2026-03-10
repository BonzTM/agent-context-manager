package backend

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/bonztm/agent-context-manager/internal/contracts/v1"
	"github.com/bonztm/agent-context-manager/internal/core"
	"github.com/bonztm/agent-context-manager/internal/workspace"
)

func (s *Service) HistorySearch(ctx context.Context, payload v1.HistorySearchPayload) (v1.HistorySearchResult, *core.APIError) {
	if s == nil || s.repo == nil {
		return v1.HistorySearchResult{}, core.NewError("INTERNAL_ERROR", "service repository is not configured", nil)
	}
	entity := normalizeHistoryEntity(payload.Entity)
	scope := normalizeHistoryScope(payload.Scope)
	unbounded := effectiveUnbounded(payload.Unbounded)
	limit := normalizeHistorySearchLimit(payload.Limit, unbounded)
	query := strings.TrimSpace(payload.Query)
	items, apiErr := s.historyItems(ctx, strings.TrimSpace(payload.ProjectID), entity, scope, strings.TrimSpace(payload.Kind), query, limit, unbounded)
	if apiErr != nil {
		return v1.HistorySearchResult{}, apiErr
	}

	result := v1.HistorySearchResult{
		Entity: entity,
		Query:  query,
		Limit:  limit,
		Count:  len(items),
		Items:  items,
	}
	if entity == v1.HistoryEntityWork {
		result.Scope = scope
	}
	return result, nil
}

func (s *Service) historyItems(ctx context.Context, projectID string, entity v1.HistoryEntity, scope v1.HistoryScope, kind, query string, limit int, unbounded bool) ([]v1.HistoryItem, *core.APIError) {
	switch entity {
	case v1.HistoryEntityWork:
		return s.listWorkHistoryItems(ctx, projectID, scope, kind, query, limit, unbounded)
	case v1.HistoryEntityMemory:
		return s.listMemoryHistoryItems(ctx, projectID, query, limit, unbounded)
	case v1.HistoryEntityReceipt:
		return s.listReceiptHistoryItems(ctx, projectID, query, limit, unbounded)
	case v1.HistoryEntityRun:
		return s.listRunHistoryItems(ctx, projectID, query, limit, unbounded)
	case v1.HistoryEntityAll:
		memoryItems, apiErr := s.listMemoryHistoryItems(ctx, projectID, query, limit, unbounded)
		if apiErr != nil {
			return nil, apiErr
		}
		workItems, apiErr := s.listWorkHistoryItems(ctx, projectID, v1.HistoryScopeAll, kind, query, limit, unbounded)
		if apiErr != nil {
			return nil, apiErr
		}
		receiptItems, apiErr := s.listReceiptHistoryItems(ctx, projectID, query, limit, unbounded)
		if apiErr != nil {
			return nil, apiErr
		}
		runItems, apiErr := s.listRunHistoryItems(ctx, projectID, query, limit, unbounded)
		if apiErr != nil {
			return nil, apiErr
		}
		allItems := make([]v1.HistoryItem, 0, len(memoryItems)+len(workItems)+len(receiptItems)+len(runItems))
		allItems = append(allItems, memoryItems...)
		allItems = append(allItems, workItems...)
		allItems = append(allItems, receiptItems...)
		allItems = append(allItems, runItems...)
		sort.SliceStable(allItems, func(i, j int) bool {
			ti := historyItemSortTime(allItems[i])
			tj := historyItemSortTime(allItems[j])
			if !ti.Equal(tj) {
				return ti.After(tj)
			}
			return allItems[i].Key < allItems[j].Key
		})
		if !unbounded && limit > 0 && len(allItems) > limit {
			allItems = allItems[:limit]
		}
		return allItems, nil
	default:
		return nil, core.NewError("INVALID_INPUT", "history entity is not supported", map[string]any{"entity": entity})
	}
}

func (s *Service) listMemoryHistoryItems(ctx context.Context, projectID, query string, limit int, unbounded bool) ([]v1.HistoryItem, *core.APIError) {
	if s.historyRepo == nil {
		return nil, core.NewError(
			"NOT_IMPLEMENTED",
			"memory history search requires history storage support",
			map[string]any{"operation": "history", "entity": "memory"},
		)
	}

	rows, err := s.historyRepo.ListMemoryHistory(ctx, core.MemoryHistoryListQuery{
		ProjectID: projectID,
		Query:     query,
		Limit:     limit,
		Unbounded: unbounded,
	})
	if err != nil {
		return nil, internalError("list_memory_history", err)
	}

	items := make([]v1.HistoryItem, 0, len(rows))
	for _, row := range rows {
		if row.MemoryID <= 0 {
			continue
		}
		summary := strings.TrimSpace(row.Subject)
		if summary == "" {
			summary = strings.TrimSpace(row.Content)
		}
		if summary == "" {
			summary = fmt.Sprintf("Memory %d", row.MemoryID)
		}
		items = append(items, v1.HistoryItem{
			Key:       fmt.Sprintf("mem:%d", row.MemoryID),
			Entity:    v1.HistoryEntityMemory,
			Summary:   summary,
			Kind:      strings.TrimSpace(row.Category),
			FetchKeys: []string{fmt.Sprintf("mem:%d", row.MemoryID)},
			UpdatedAt: historyTimestamp(row.UpdatedAt),
		})
	}
	return items, nil
}

func (s *Service) listWorkHistoryItems(ctx context.Context, projectID string, scope v1.HistoryScope, kind, query string, limit int, unbounded bool) ([]v1.HistoryItem, *core.APIError) {
	if s.planRepo == nil {
		return nil, core.NewError(
			"NOT_IMPLEMENTED",
			"work history search requires plan storage support",
			map[string]any{"operation": "history", "entity": "work"},
		)
	}

	rows, err := s.planRepo.ListWorkPlans(ctx, core.WorkPlanListQuery{
		ProjectID: projectID,
		Scope:     string(scope),
		Query:     query,
		Kind:      kind,
		Limit:     limit,
		Unbounded: unbounded,
	})
	if err != nil {
		return nil, internalError("list_work_plans", err)
	}

	items := make([]v1.HistoryItem, 0, len(rows))
	for _, row := range rows {
		planKey := strings.TrimSpace(row.PlanKey)
		if planKey == "" {
			continue
		}
		status := normalizePlanStatus(row.Status)
		summary := strings.TrimSpace(row.Summary)
		if summary == "" {
			summary = fmt.Sprintf("Plan %s is %s", planKey, status)
		}
		items = append(items, v1.HistoryItem{
			Key:           planKey,
			Entity:        v1.HistoryEntityWork,
			Summary:       summary,
			Status:        status,
			Scope:         historyScopeFromPlanStatus(status),
			PlanKey:       planKey,
			ReceiptID:     strings.TrimSpace(row.ReceiptID),
			Kind:          strings.TrimSpace(row.Kind),
			ParentPlanKey: strings.TrimSpace(row.ParentPlanKey),
			TaskCounts: &v1.ContextPlanTaskCounts{
				Total:      maxZero(row.TaskCountTotal),
				Pending:    maxZero(row.TaskCountPending),
				InProgress: maxZero(row.TaskCountInProgress),
				Blocked:    maxZero(row.TaskCountBlocked),
				Complete:   maxZero(row.TaskCountComplete),
			},
			FetchKeys: []string{planKey},
			UpdatedAt: historyTimestamp(row.UpdatedAt),
		})
	}
	return items, nil
}

func (s *Service) listReceiptHistoryItems(ctx context.Context, projectID, query string, limit int, unbounded bool) ([]v1.HistoryItem, *core.APIError) {
	if s.historyRepo == nil {
		return nil, core.NewError(
			"NOT_IMPLEMENTED",
			"receipt history search requires history storage support",
			map[string]any{"operation": "history", "entity": "receipt"},
		)
	}

	rows, err := s.historyRepo.ListReceiptHistory(ctx, core.ReceiptHistoryListQuery{
		ProjectID: projectID,
		Query:     query,
		Limit:     limit,
		Unbounded: unbounded,
	})
	if err != nil {
		return nil, internalError("list_receipt_history", err)
	}

	items := make([]v1.HistoryItem, 0, len(rows))
	for _, row := range rows {
		receiptID := strings.TrimSpace(row.ReceiptID)
		if receiptID == "" {
			continue
		}
		summary := strings.TrimSpace(row.TaskText)
		if summary == "" {
			summary = fmt.Sprintf("Receipt %s", receiptID)
		}
		items = append(items, v1.HistoryItem{
			Key:       receiptFetchKey(receiptID),
			Entity:    v1.HistoryEntityReceipt,
			Summary:   summary,
			Status:    strings.TrimSpace(row.LatestStatus),
			ReceiptID: receiptID,
			RequestID: strings.TrimSpace(row.LatestRequestID),
			Phase:     v1.Phase(strings.TrimSpace(row.Phase)),
			FetchKeys: []string{receiptFetchKey(receiptID)},
			UpdatedAt: historyTimestamp(row.UpdatedAt),
		})
	}
	return items, nil
}

func (s *Service) listRunHistoryItems(ctx context.Context, projectID, query string, limit int, unbounded bool) ([]v1.HistoryItem, *core.APIError) {
	if s.historyRepo == nil {
		return nil, core.NewError(
			"NOT_IMPLEMENTED",
			"run history search requires history storage support",
			map[string]any{"operation": "history", "entity": "run"},
		)
	}

	rows, err := s.historyRepo.ListRunHistory(ctx, core.RunHistoryListQuery{
		ProjectID: projectID,
		Query:     query,
		Limit:     limit,
		Unbounded: unbounded,
	})
	if err != nil {
		return nil, internalError("list_run_history", err)
	}

	items := make([]v1.HistoryItem, 0, len(rows))
	for _, row := range rows {
		if row.RunID <= 0 {
			continue
		}
		summary := strings.TrimSpace(row.Outcome)
		if summary == "" {
			summary = strings.TrimSpace(row.TaskText)
		}
		if summary == "" {
			summary = fmt.Sprintf("Run %d", row.RunID)
		}
		items = append(items, v1.HistoryItem{
			Key:       runFetchKey(row.RunID),
			Entity:    v1.HistoryEntityRun,
			Summary:   summary,
			Status:    strings.TrimSpace(row.Status),
			ReceiptID: strings.TrimSpace(row.ReceiptID),
			RunID:     row.RunID,
			RequestID: strings.TrimSpace(row.RequestID),
			Phase:     v1.Phase(strings.TrimSpace(row.Phase)),
			FetchKeys: []string{runFetchKey(row.RunID)},
			UpdatedAt: historyTimestamp(row.UpdatedAt),
		})
	}
	return items, nil
}

func historyTimestamp(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Format(time.RFC3339)
}

func historyItemSortTime(item v1.HistoryItem) time.Time {
	if strings.TrimSpace(item.UpdatedAt) == "" {
		return time.Time{}
	}
	parsed, err := time.Parse(time.RFC3339, item.UpdatedAt)
	if err != nil {
		return time.Time{}
	}
	return parsed
}

func normalizeHistoryScope(raw v1.HistoryScope) v1.HistoryScope {
	switch strings.TrimSpace(string(raw)) {
	case string(v1.HistoryScopeCurrent):
		return v1.HistoryScopeCurrent
	case string(v1.HistoryScopeDeferred):
		return v1.HistoryScopeDeferred
	case string(v1.HistoryScopeCompleted):
		return v1.HistoryScopeCompleted
	case string(v1.HistoryScopeAll):
		return v1.HistoryScopeAll
	default:
		return v1.HistoryScopeAll
	}
}

func normalizeHistoryEntity(raw v1.HistoryEntity) v1.HistoryEntity {
	switch strings.TrimSpace(string(raw)) {
	case string(v1.HistoryEntityAll):
		return v1.HistoryEntityAll
	case string(v1.HistoryEntityMemory):
		return v1.HistoryEntityMemory
	case string(v1.HistoryEntityReceipt):
		return v1.HistoryEntityReceipt
	case string(v1.HistoryEntityRun):
		return v1.HistoryEntityRun
	case string(v1.HistoryEntityWork):
		return v1.HistoryEntityWork
	default:
		return v1.HistoryEntityAll
	}
}

func historyScopeFromPlanStatus(status string) v1.HistoryScope {
	switch normalizePlanStatus(status) {
	case core.PlanStatusBlocked:
		return v1.HistoryScopeDeferred
	case core.PlanStatusComplete, core.PlanStatusCompleted:
		return v1.HistoryScopeCompleted
	default:
		return v1.HistoryScopeCurrent
	}
}

func effectiveUnbounded(explicit *bool) bool {
	if explicit != nil {
		return *explicit
	}
	return workspace.LookupEnvBool("", unboundedEnvVar, nil)
}

func normalizeHistorySearchLimit(limit int, unbounded bool) int {
	if unbounded {
		return 0
	}
	switch {
	case limit <= 0:
		return 20
	case limit > 100:
		return 100
	default:
		return limit
	}
}
