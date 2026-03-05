package postgres

import (
	"context"

	"github.com/joshd/agent-context-manager/internal/core"
)

func (f *fakeRepository) SyncRulePointers(_ context.Context, _ core.RulePointerSyncInput) (core.RulePointerSyncResult, error) {
	return core.RulePointerSyncResult{}, nil
}
