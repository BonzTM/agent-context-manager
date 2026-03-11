package backend

import (
	"context"
	"strings"

	"github.com/bonztm/agent-context-manager/internal/core"
)

func (s *Service) syncTerminalWorkPlanStatus(ctx context.Context, projectID, receiptID string, plan core.WorkPlan, items []core.WorkItem) (core.WorkPlan, bool, *core.APIError) {
	if s == nil || s.planRepo == nil {
		return plan, false, nil
	}

	planKey := strings.TrimSpace(plan.PlanKey)
	if planKey == "" {
		return plan, false, nil
	}

	normalizedItems := normalizeWorkItems(items)
	if len(normalizedItems) == 0 {
		return plan, false, nil
	}

	derivedStatus := derivePlanStatusFromWorkItems(normalizedItems)
	if !isTerminalPlanStatus(derivedStatus) {
		return plan, false, nil
	}

	currentStatus := normalizePlanStatus(plan.Status)
	if currentStatus == derivedStatus {
		return plan, false, nil
	}

	upsertResult, err := s.planRepo.UpsertWorkPlan(ctx, core.WorkPlanUpsertInput{
		ProjectID: strings.TrimSpace(projectID),
		PlanKey:   planKey,
		ReceiptID: firstNonEmpty(strings.TrimSpace(receiptID), strings.TrimSpace(plan.ReceiptID)),
		Mode:      core.WorkPlanModeMerge,
		Status:    derivedStatus,
	})
	if err != nil {
		return core.WorkPlan{}, false, workInternalError("autoclose_work_plan", err)
	}
	return upsertResult.Plan, true, nil
}
