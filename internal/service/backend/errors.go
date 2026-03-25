package backend

import (
	"github.com/bonztm/agent-context-manager/internal/contracts/v1"
	"github.com/bonztm/agent-context-manager/internal/core"
)

func backendError(code, message string, details any) *core.APIError {
	return core.NewErrorWithSource(code, message, v1.ErrSourceBackend, details)
}
