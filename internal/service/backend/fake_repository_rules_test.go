package backend

import (
	"context"

	"github.com/bonztm/agent-context-manager/internal/core"
)

func (f *fakeRepository) SyncRulePointers(_ context.Context, _ core.RulePointerSyncInput) (core.RulePointerSyncResult, error) {
	return core.RulePointerSyncResult{}, nil
}
