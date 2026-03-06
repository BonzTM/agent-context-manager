package core

import (
	"context"
	"strings"
	"time"

	"github.com/bonztm/agent-context-manager/internal/contracts/v1"
	"github.com/bonztm/agent-context-manager/internal/logging"
)

type loggingService struct {
	next   Service
	logger logging.Logger
	now    func() time.Time
}

var _ Service = (*loggingService)(nil)

func WithLogging(next Service, logger logging.Logger) Service {
	return WithLoggingClock(next, logger, time.Now)
}

func WithLoggingClock(next Service, logger logging.Logger, now func() time.Time) Service {
	if next == nil {
		return nil
	}
	if now == nil {
		now = time.Now
	}
	return &loggingService{
		next:   next,
		logger: logging.Normalize(logger),
		now:    now,
	}
}

func (s *loggingService) GetContext(ctx context.Context, payload v1.GetContextPayload) (v1.GetContextResult, *APIError) {
	return withOperation(ctx, s.now, s.logger, logging.OperationGetContext, payload.ProjectID, func() (v1.GetContextResult, *APIError) {
		return s.next.GetContext(ctx, payload)
	})
}

func (s *loggingService) Fetch(ctx context.Context, payload v1.FetchPayload) (v1.FetchResult, *APIError) {
	return withOperation(ctx, s.now, s.logger, logging.OperationFetch, payload.ProjectID, func() (v1.FetchResult, *APIError) {
		return s.next.Fetch(ctx, payload)
	})
}

func (s *loggingService) ProposeMemory(ctx context.Context, payload v1.ProposeMemoryPayload) (v1.ProposeMemoryResult, *APIError) {
	return withOperation(ctx, s.now, s.logger, logging.OperationProposeMemory, payload.ProjectID, func() (v1.ProposeMemoryResult, *APIError) {
		return s.next.ProposeMemory(ctx, payload)
	})
}

func (s *loggingService) Work(ctx context.Context, payload v1.WorkPayload) (v1.WorkResult, *APIError) {
	return withOperation(ctx, s.now, s.logger, logging.OperationWork, payload.ProjectID, func() (v1.WorkResult, *APIError) {
		return s.next.Work(ctx, payload)
	})
}

func (s *loggingService) ReportCompletion(ctx context.Context, payload v1.ReportCompletionPayload) (v1.ReportCompletionResult, *APIError) {
	return withOperation(ctx, s.now, s.logger, logging.OperationReportCompletion, payload.ProjectID, func() (v1.ReportCompletionResult, *APIError) {
		return s.next.ReportCompletion(ctx, payload)
	})
}

func (s *loggingService) Sync(ctx context.Context, payload v1.SyncPayload) (v1.SyncResult, *APIError) {
	return withOperation(ctx, s.now, s.logger, logging.OperationSync, payload.ProjectID, func() (v1.SyncResult, *APIError) {
		return s.next.Sync(ctx, payload)
	})
}

func (s *loggingService) HealthCheck(ctx context.Context, payload v1.HealthCheckPayload) (v1.HealthCheckResult, *APIError) {
	return withOperation(ctx, s.now, s.logger, logging.OperationHealthCheck, payload.ProjectID, func() (v1.HealthCheckResult, *APIError) {
		return s.next.HealthCheck(ctx, payload)
	})
}

func (s *loggingService) HealthFix(ctx context.Context, payload v1.HealthFixPayload) (v1.HealthFixResult, *APIError) {
	return withOperation(ctx, s.now, s.logger, logging.OperationHealthFix, payload.ProjectID, func() (v1.HealthFixResult, *APIError) {
		return s.next.HealthFix(ctx, payload)
	})
}

func (s *loggingService) Coverage(ctx context.Context, payload v1.CoveragePayload) (v1.CoverageResult, *APIError) {
	return withOperation(ctx, s.now, s.logger, logging.OperationCoverage, payload.ProjectID, func() (v1.CoverageResult, *APIError) {
		return s.next.Coverage(ctx, payload)
	})
}

func (s *loggingService) Eval(ctx context.Context, payload v1.EvalPayload) (v1.EvalResult, *APIError) {
	return withOperation(ctx, s.now, s.logger, logging.OperationEval, payload.ProjectID, func() (v1.EvalResult, *APIError) {
		return s.next.Eval(ctx, payload)
	})
}

func (s *loggingService) Verify(ctx context.Context, payload v1.VerifyPayload) (v1.VerifyResult, *APIError) {
	return withOperation(ctx, s.now, s.logger, logging.OperationVerify, payload.ProjectID, func() (v1.VerifyResult, *APIError) {
		return s.next.Verify(ctx, payload)
	})
}

func (s *loggingService) Bootstrap(ctx context.Context, payload v1.BootstrapPayload) (v1.BootstrapResult, *APIError) {
	return withOperation(ctx, s.now, s.logger, logging.OperationBootstrap, payload.ProjectID, func() (v1.BootstrapResult, *APIError) {
		return s.next.Bootstrap(ctx, payload)
	})
}

func withOperation[T any](ctx context.Context, now func() time.Time, logger logging.Logger, operation, projectID string, run func() (T, *APIError)) (T, *APIError) {
	startedAt := now()
	projectID = strings.TrimSpace(projectID)

	startFields := []any{"operation", operation}
	if projectID != "" {
		startFields = append(startFields, "project_id", projectID)
	}
	logger.Info(ctx, logging.EventServiceOperationStart, startFields...)

	result, apiErr := run()
	durationMS := now().Sub(startedAt).Milliseconds()
	if durationMS < 0 {
		durationMS = 0
	}

	finishFields := []any{
		"operation", operation,
		"duration_ms", durationMS,
		"ok", apiErr == nil,
	}
	if projectID != "" {
		finishFields = append(finishFields, "project_id", projectID)
	}
	if apiErr != nil && strings.TrimSpace(apiErr.Code) != "" {
		finishFields = append(finishFields, "error_code", apiErr.Code)
	}

	if apiErr != nil {
		logger.Error(ctx, logging.EventServiceOperationFinish, finishFields...)
	} else {
		logger.Info(ctx, logging.EventServiceOperationFinish, finishFields...)
	}

	return result, apiErr
}
