package v1

const (
	ErrCodeInvalidJSON      = "INVALID_JSON"
	ErrCodeInvalidVersion   = "INVALID_VERSION"
	ErrCodeInvalidCommand   = "INVALID_COMMAND"
	ErrCodeInvalidPayload   = "INVALID_PAYLOAD"
	ErrCodeInvalidRequestID = "INVALID_REQUEST_ID"
	ErrCodeInvalidInput     = "INVALID_INPUT"
	ErrCodeValidationError  = "VALIDATION_ERROR"
)

const (
	ErrCodeUnknownTool       = "UNKNOWN_TOOL"
	ErrCodeMissingTool       = "MISSING_TOOL"
	ErrCodeServiceInitFailed = "SERVICE_INIT_FAILED"
	ErrCodeDispatchFailed    = "DISPATCH_FAILED"
	ErrCodeReadFailed        = "READ_FAILED"
	ErrCodeWriteFailed       = "WRITE_FAILED"
	ErrCodeInternalError     = "INTERNAL_ERROR"
	ErrCodeNotFound          = "NOT_FOUND"
	ErrCodeNotImplemented    = "NOT_IMPLEMENTED"
)

const (
	ErrCodeMissingSubcommand = "MISSING_SUBCOMMAND"
	ErrCodeUnknownSubcommand = "UNKNOWN_SUBCOMMAND"
	ErrCodeInvalidFlags      = "INVALID_FLAGS"
)

const (
	ErrSourceValidation = "validation"
	ErrSourceDispatch   = "dispatch"
	ErrSourceBackend    = "backend"
	ErrSourceAdapter    = "adapter"
)
