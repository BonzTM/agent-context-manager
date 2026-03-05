package core

import (
	"context"

	"github.com/joshd/agents-context/internal/contracts/v1"
)

type Service interface {
	GetContext(context.Context, v1.GetContextPayload) (v1.GetContextResult, *APIError)
	Fetch(context.Context, v1.FetchPayload) (v1.FetchResult, *APIError)
	ProposeMemory(context.Context, v1.ProposeMemoryPayload) (v1.ProposeMemoryResult, *APIError)
	Work(context.Context, v1.WorkPayload) (v1.WorkResult, *APIError)
	ReportCompletion(context.Context, v1.ReportCompletionPayload) (v1.ReportCompletionResult, *APIError)
	Sync(context.Context, v1.SyncPayload) (v1.SyncResult, *APIError)
	HealthCheck(context.Context, v1.HealthCheckPayload) (v1.HealthCheckResult, *APIError)
	HealthFix(context.Context, v1.HealthFixPayload) (v1.HealthFixResult, *APIError)
	Coverage(context.Context, v1.CoveragePayload) (v1.CoverageResult, *APIError)
	Regress(context.Context, v1.RegressPayload) (v1.RegressResult, *APIError)
	Bootstrap(context.Context, v1.BootstrapPayload) (v1.BootstrapResult, *APIError)
}
