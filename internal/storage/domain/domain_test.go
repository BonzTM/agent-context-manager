package domain

import (
	"reflect"
	"testing"

	"github.com/bonztm/agent-context-manager/internal/core"
)

func TestMergeIncomingWorkPlanTasksPreservesExistingMetadata(t *testing.T) {
	current := []core.WorkItem{{
		ItemKey:            "impl.merge",
		Summary:            "Preserve metadata",
		Status:             core.WorkItemStatusBlocked,
		ParentTaskKey:      "stage:implementation-plan",
		DependsOn:          []string{"spec:merge"},
		AcceptanceCriteria: []string{"acceptance survives merge"},
		References:         []string{"docs/feature-plans.md"},
		ExternalRefs:       []string{"jira:ACM-42"},
		BlockedReason:      "waiting on review",
		Evidence:           []string{"verifyrun:seed"},
	}}

	merged := MergeIncomingWorkPlanTasks(current, []core.WorkItem{{
		ItemKey:  "impl.merge",
		Summary:  "Preserve metadata",
		Status:   core.WorkItemStatusComplete,
		Outcome:  "completed after verification",
		Evidence: []string{"verifyrun:final"},
	}}, core.WorkPlanModeMerge)

	if len(merged) != 1 {
		t.Fatalf("expected one merged task, got %+v", merged)
	}
	task := merged[0]
	if task.Status != core.WorkItemStatusComplete || task.Outcome != "completed after verification" {
		t.Fatalf("unexpected merged task state: %+v", task)
	}
	if task.ParentTaskKey != "stage:implementation-plan" {
		t.Fatalf("expected parent task key to survive merge, got %+v", task)
	}
	if !reflect.DeepEqual(task.DependsOn, []string{"spec:merge"}) {
		t.Fatalf("expected depends_on to survive merge, got %+v", task.DependsOn)
	}
	if !reflect.DeepEqual(task.AcceptanceCriteria, []string{"acceptance survives merge"}) {
		t.Fatalf("expected acceptance criteria to survive merge, got %+v", task.AcceptanceCriteria)
	}
	if !reflect.DeepEqual(task.References, []string{"docs/feature-plans.md"}) {
		t.Fatalf("expected references to survive merge, got %+v", task.References)
	}
	if !reflect.DeepEqual(task.ExternalRefs, []string{"jira:ACM-42"}) {
		t.Fatalf("expected external refs to survive merge, got %+v", task.ExternalRefs)
	}
	if task.BlockedReason != "" {
		t.Fatalf("expected blocked reason to clear after non-blocked merge update, got %+v", task)
	}
	if !reflect.DeepEqual(task.Evidence, []string{"verifyrun:final"}) {
		t.Fatalf("expected evidence to update, got %+v", task.Evidence)
	}
}
