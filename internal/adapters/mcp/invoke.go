package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/bonztm/agent-context-manager/internal/commands"
	"github.com/bonztm/agent-context-manager/internal/contracts/v1"
	"github.com/bonztm/agent-context-manager/internal/core"
	"github.com/bonztm/agent-context-manager/internal/logging"
	"github.com/bonztm/agent-context-manager/internal/runtime"
)

const (
	toolFetch         = string(v1.CommandFetch)
	toolReview        = string(v1.CommandReview)
	toolWork          = string(v1.CommandWork)
	toolHistorySearch = string(v1.CommandHistorySearch)
	toolSync          = string(v1.CommandSync)
	toolHealthCheck   = string(v1.CommandHealthCheck)
	toolHealthFix     = string(v1.CommandHealthFix)
	toolCoverage      = string(v1.CommandCoverage)
	toolEval          = string(v1.CommandEval)
	toolVerify        = string(v1.CommandVerify)
	toolBootstrap     = string(v1.CommandBootstrap)
)

type ToolDef struct {
	Name         string         `json:"name"`
	Title        string         `json:"title"`
	Description  string         `json:"description"`
	InputSchema  map[string]any `json:"input_schema"`
	OutputSchema map[string]any `json:"output_schema"`
}

func ToolDefinitions() []ToolDef {
	specs := v1.CommandSpecs()
	defs := make([]ToolDef, 0, len(specs))
	for _, spec := range specs {
		defs = append(defs, ToolDef{
			Name:         string(spec.Command),
			Title:        spec.ToolTitle,
			Description:  spec.ToolDescription,
			InputSchema:  schemaRef(commandSchemaID, spec.InputSchemaDef),
			OutputSchema: schemaRef(resultSchemaID, spec.ResultSchemaDef),
		})
	}
	return defs
}

const (
	schemaDraft202012 = "https://json-schema.org/draft/2020-12/schema"
	commandSchemaID   = "https://agent-context-manager.dev/spec/v1/cli.command.schema.json"
	resultSchemaID    = "https://agent-context-manager.dev/spec/v1/cli.result.schema.json"
)

func schemaRef(schemaID, defName string) map[string]any {
	return map[string]any{
		"$schema": schemaDraft202012,
		"$ref":    fmt.Sprintf("%s#/$defs/%s", schemaID, defName),
	}
}

func Invoke(ctx context.Context, svc core.Service, tool string, input []byte) (any, *core.APIError) {
	return InvokeWithLogger(ctx, svc, tool, input, nil)
}

func InvokeWithLogger(ctx context.Context, svc core.Service, tool string, input []byte, logger logging.Logger) (any, *core.APIError) {
	logger = logging.Normalize(logger)
	logger.Info(ctx, logging.EventMCPIngressRead, "ok", true, "tool", tool, "bytes", len(input))

	command, ok := v1.CommandFromToolName(tool)
	if !ok {
		err := core.NewError("UNKNOWN_TOOL", "tool is not supported in v1", map[string]any{"tool": tool})
		logger.Error(ctx, logging.EventMCPIngressValidate, "ok", false, "tool", tool, "error_code", err.Code)
		logger.Error(ctx, logging.EventMCPFailure, "stage", "validate", "tool", tool, "error_code", err.Code)
		logger.Info(ctx, logging.EventMCPResult, "ok", false, "tool", tool, "error_code", err.Code)
		return nil, err
	}
	rawProjectID := projectIDFromRawToolInput(input)
	defaults := validationDefaultsFromRuntime()
	effectiveProjectID := rawProjectID
	if effectiveProjectID == "" {
		effectiveProjectID = defaults.ProjectID
	}

	payload, err := decodeValidatedToolPayload(command, json.RawMessage(input), defaults)
	if err != nil {
		return mcpToolInputError(ctx, logger, tool, effectiveProjectID, err)
	}

	normalizedProjectID := commands.ProjectIDFromPayload(payload)
	if normalizedProjectID == "" {
		normalizedProjectID = effectiveProjectID
	}
	logMCPValidateSuccess(ctx, logger, tool, normalizedProjectID)
	logMCPDispatchStart(ctx, logger, tool, normalizedProjectID)

	result, apiErr := commands.Dispatch(ctx, svc, command, payload)
	if apiErr != nil {
		return mcpDispatchError(ctx, logger, tool, normalizedProjectID, apiErr)
	}

	logMCPDispatchSuccess(ctx, logger, tool, normalizedProjectID)
	logMCPResultSuccess(ctx, logger, tool, normalizedProjectID)
	return result, nil
}

func decodeValidatedToolPayload(command v1.Command, raw json.RawMessage, defaults v1.ValidationDefaults) (any, error) {
	blob, err := v1.BuildEnvelopeForCommand(command, "mcp.invoke", raw)
	if err != nil {
		return nil, err
	}
	_, payload, valErr := v1.DecodeAndValidateCommandWithDefaults(blob, defaults)
	if valErr != nil {
		return nil, fmt.Errorf("%s: %s", valErr.Code, valErr.Message)
	}
	return payload, nil
}

func validationDefaultsFromRuntime() v1.ValidationDefaults {
	cfg := runtime.ConfigFromEnv()
	return v1.ValidationDefaults{
		ProjectID: cfg.EffectiveProjectID(),
	}
}

func projectIDFromRawToolInput(input []byte) string {
	var payload map[string]json.RawMessage
	if err := json.Unmarshal(input, &payload); err != nil {
		return ""
	}
	raw, ok := payload["project_id"]
	if !ok {
		return ""
	}
	var projectID string
	if err := json.Unmarshal(raw, &projectID); err != nil {
		return ""
	}
	return strings.TrimSpace(projectID)
}

func mcpToolInputError(ctx context.Context, logger logging.Logger, tool, projectID string, err error) (any, *core.APIError) {
	apiErr := core.NewError("INVALID_TOOL_INPUT", err.Error(), nil)
	fields := []any{"ok", false, "tool", tool, "error_code", apiErr.Code}
	if projectID != "" {
		fields = append(fields, "project_id", projectID)
	}
	logger.Error(ctx, logging.EventMCPIngressValidate, fields...)
	failureFields := []any{"stage", "validate", "tool", tool, "error_code", apiErr.Code}
	if projectID != "" {
		failureFields = append(failureFields, "project_id", projectID)
	}
	logger.Error(ctx, logging.EventMCPFailure, failureFields...)
	resultFields := []any{"ok", false, "tool", tool, "error_code", apiErr.Code}
	if projectID != "" {
		resultFields = append(resultFields, "project_id", projectID)
	}
	logger.Info(ctx, logging.EventMCPResult, resultFields...)
	return nil, apiErr
}

func mcpDispatchError(ctx context.Context, logger logging.Logger, tool, projectID string, apiErr *core.APIError) (any, *core.APIError) {
	dispatchFields := []any{"phase", "finish", "ok", false, "tool", tool, "error_code", apiErr.Code}
	if projectID != "" {
		dispatchFields = append(dispatchFields, "project_id", projectID)
	}
	logger.Error(ctx, logging.EventMCPDispatch, dispatchFields...)
	failureFields := []any{"stage", "dispatch", "tool", tool, "error_code", apiErr.Code}
	if projectID != "" {
		failureFields = append(failureFields, "project_id", projectID)
	}
	logger.Error(ctx, logging.EventMCPFailure, failureFields...)
	resultFields := []any{"ok", false, "tool", tool, "error_code", apiErr.Code}
	if projectID != "" {
		resultFields = append(resultFields, "project_id", projectID)
	}
	logger.Info(ctx, logging.EventMCPResult, resultFields...)
	return nil, apiErr
}

func logMCPValidateSuccess(ctx context.Context, logger logging.Logger, tool, projectID string) {
	fields := []any{"ok", true, "tool", tool}
	if projectID != "" {
		fields = append(fields, "project_id", projectID)
	}
	logger.Info(ctx, logging.EventMCPIngressValidate, fields...)
}

func logMCPDispatchStart(ctx context.Context, logger logging.Logger, tool, projectID string) {
	fields := []any{"phase", "start", "tool", tool}
	if projectID != "" {
		fields = append(fields, "project_id", projectID)
	}
	logger.Info(ctx, logging.EventMCPDispatch, fields...)
}

func logMCPDispatchSuccess(ctx context.Context, logger logging.Logger, tool, projectID string) {
	fields := []any{"phase", "finish", "ok", true, "tool", tool}
	if projectID != "" {
		fields = append(fields, "project_id", projectID)
	}
	logger.Info(ctx, logging.EventMCPDispatch, fields...)
}

func logMCPResultSuccess(ctx context.Context, logger logging.Logger, tool, projectID string) {
	fields := []any{"ok", true, "tool", tool}
	if projectID != "" {
		fields = append(fields, "project_id", projectID)
	}
	logger.Info(ctx, logging.EventMCPResult, fields...)
}
