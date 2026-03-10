package logging

const (
	OperationContext       = "context"
	OperationFetch         = "fetch"
	OperationMemory        = "memory"
	OperationReview        = "review"
	OperationWork          = "work"
	OperationHistorySearch = "history"
	OperationDone          = "done"
	OperationSync          = "sync"
	OperationHealth        = "health"
	OperationStatus        = "status"
	OperationVerify        = "verify"
	OperationInit          = "init"
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
