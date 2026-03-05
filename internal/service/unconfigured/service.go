package unconfigured

import (
	"context"

	"github.com/joshd/agents-context/internal/contracts/v1"
	"github.com/joshd/agents-context/internal/core"
)

type Service struct{}

func New() *Service {
	return &Service{}
}

func (s *Service) GetContext(_ context.Context, _ v1.GetContextPayload) (v1.GetContextResult, *core.APIError) {
	return v1.GetContextResult{}, notImplemented("get_context")
}

func (s *Service) Fetch(_ context.Context, _ v1.FetchPayload) (v1.FetchResult, *core.APIError) {
	return v1.FetchResult{}, notImplemented("fetch")
}

func (s *Service) ProposeMemory(_ context.Context, _ v1.ProposeMemoryPayload) (v1.ProposeMemoryResult, *core.APIError) {
	return v1.ProposeMemoryResult{}, notImplemented("propose_memory")
}

func (s *Service) Work(_ context.Context, _ v1.WorkPayload) (v1.WorkResult, *core.APIError) {
	return v1.WorkResult{}, notImplemented("work")
}

func (s *Service) ReportCompletion(_ context.Context, _ v1.ReportCompletionPayload) (v1.ReportCompletionResult, *core.APIError) {
	return v1.ReportCompletionResult{}, notImplemented("report_completion")
}

func (s *Service) Sync(_ context.Context, _ v1.SyncPayload) (v1.SyncResult, *core.APIError) {
	return v1.SyncResult{}, notImplemented("sync")
}

func (s *Service) HealthCheck(_ context.Context, _ v1.HealthCheckPayload) (v1.HealthCheckResult, *core.APIError) {
	return v1.HealthCheckResult{}, notImplemented("health_check")
}

func (s *Service) HealthFix(_ context.Context, _ v1.HealthFixPayload) (v1.HealthFixResult, *core.APIError) {
	return v1.HealthFixResult{}, notImplemented("health_fix")
}

func (s *Service) Coverage(_ context.Context, _ v1.CoveragePayload) (v1.CoverageResult, *core.APIError) {
	return v1.CoverageResult{}, notImplemented("coverage")
}

func (s *Service) Regress(_ context.Context, _ v1.RegressPayload) (v1.RegressResult, *core.APIError) {
	return v1.RegressResult{}, notImplemented("regress")
}

func (s *Service) Bootstrap(_ context.Context, _ v1.BootstrapPayload) (v1.BootstrapResult, *core.APIError) {
	return v1.BootstrapResult{}, notImplemented("bootstrap")
}

func notImplemented(op string) *core.APIError {
	return core.NewError(
		"NOT_IMPLEMENTED",
		"service backend for operation is not wired yet",
		map[string]any{"operation": op},
	)
}
