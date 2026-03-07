package logging

const (
	OperationGetContext       = "get_context"
	OperationFetch            = "fetch"
	OperationProposeMemory    = "propose_memory"
	OperationReview           = "review"
	OperationWork             = "work"
	OperationHistorySearch    = "history_search"
	OperationReportCompletion = "report_completion"
	OperationSync             = "sync"
	OperationHealthCheck      = "health_check"
	OperationHealthFix        = "health_fix"
	OperationCoverage         = "coverage"
	OperationEval             = "eval"
	OperationVerify           = "verify"
	OperationBootstrap        = "bootstrap"
)

const (
	EventServiceOperationStart  = "service.operation.start"
	EventServiceOperationFinish = "service.operation.finish"

	EventCLIIngressRead     = "cli.ingress.read"
	EventCLIIngressValidate = "cli.ingress.validate"
	EventCLIDispatch        = "cli.dispatch"
	EventCLIResult          = "cli.result"
	EventCLIFailure         = "cli.failure"

	EventMCPIngressRead     = "mcp.ingress.read"
	EventMCPIngressValidate = "mcp.ingress.validate"
	EventMCPDispatch        = "mcp.dispatch"
	EventMCPResult          = "mcp.result"
	EventMCPFailure         = "mcp.failure"

	EventACMRun    = "acm.run"
	EventACMMCP    = "acm.mcp"
	EventACMIORead = "acm.io.read"
)
