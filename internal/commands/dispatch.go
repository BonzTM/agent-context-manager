package commands

import (
	"context"

	"github.com/bonztm/agent-context-manager/internal/contracts/v1"
	"github.com/bonztm/agent-context-manager/internal/core"
)

type handlerFunc func(context.Context, core.Service, any) (any, *core.APIError)

var handlers = map[v1.Command]handlerFunc{
	v1.CommandGetContext: func(ctx context.Context, svc core.Service, payload any) (any, *core.APIError) {
		return svc.GetContext(ctx, payload.(v1.GetContextPayload))
	},
	v1.CommandFetch: func(ctx context.Context, svc core.Service, payload any) (any, *core.APIError) {
		return svc.Fetch(ctx, payload.(v1.FetchPayload))
	},
	v1.CommandProposeMemory: func(ctx context.Context, svc core.Service, payload any) (any, *core.APIError) {
		return svc.ProposeMemory(ctx, payload.(v1.ProposeMemoryPayload))
	},
	v1.CommandReportCompletion: func(ctx context.Context, svc core.Service, payload any) (any, *core.APIError) {
		return svc.ReportCompletion(ctx, payload.(v1.ReportCompletionPayload))
	},
	v1.CommandReview: func(ctx context.Context, svc core.Service, payload any) (any, *core.APIError) {
		return svc.Review(ctx, payload.(v1.ReviewPayload))
	},
	v1.CommandWork: func(ctx context.Context, svc core.Service, payload any) (any, *core.APIError) {
		return svc.Work(ctx, payload.(v1.WorkPayload))
	},
	v1.CommandHistorySearch: func(ctx context.Context, svc core.Service, payload any) (any, *core.APIError) {
		return svc.HistorySearch(ctx, payload.(v1.HistorySearchPayload))
	},
	v1.CommandSync: func(ctx context.Context, svc core.Service, payload any) (any, *core.APIError) {
		return svc.Sync(ctx, payload.(v1.SyncPayload))
	},
	v1.CommandHealthCheck: func(ctx context.Context, svc core.Service, payload any) (any, *core.APIError) {
		return svc.HealthCheck(ctx, payload.(v1.HealthCheckPayload))
	},
	v1.CommandHealthFix: func(ctx context.Context, svc core.Service, payload any) (any, *core.APIError) {
		return svc.HealthFix(ctx, payload.(v1.HealthFixPayload))
	},
	v1.CommandCoverage: func(ctx context.Context, svc core.Service, payload any) (any, *core.APIError) {
		return svc.Coverage(ctx, payload.(v1.CoveragePayload))
	},
	v1.CommandEval: func(ctx context.Context, svc core.Service, payload any) (any, *core.APIError) {
		return svc.Eval(ctx, payload.(v1.EvalPayload))
	},
	v1.CommandVerify: func(ctx context.Context, svc core.Service, payload any) (any, *core.APIError) {
		return svc.Verify(ctx, payload.(v1.VerifyPayload))
	},
	v1.CommandBootstrap: func(ctx context.Context, svc core.Service, payload any) (any, *core.APIError) {
		return svc.Bootstrap(ctx, payload.(v1.BootstrapPayload))
	},
}

func Dispatch(ctx context.Context, svc core.Service, command v1.Command, payload any) (any, *core.APIError) {
	handler, ok := handlers[command]
	if !ok {
		return nil, core.NewError("INVALID_COMMAND", "command is not recognized", nil)
	}
	return handler(ctx, svc, payload)
}

func ProjectIDFromPayload(payload any) string {
	return v1.ProjectIDFromPayload(payload)
}
