package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/joshd/agents-context/internal/contracts/v1"
	"github.com/joshd/agents-context/internal/core"
	"github.com/joshd/agents-context/internal/logging"
)

const (
	toolFetch = "fetch"
	toolWork  = "work"
)

type ToolDef struct {
	Name         string         `json:"name"`
	Title        string         `json:"title"`
	Description  string         `json:"description"`
	InputSchema  map[string]any `json:"input_schema"`
	OutputSchema map[string]any `json:"output_schema"`
}

func ToolDefinitions() []ToolDef {
	return []ToolDef{
		{
			Name:         string(v1.CommandGetContext),
			Title:        "Get Task Context",
			Description:  "Deterministically resolve task-scoped pointers, rules, and memories.",
			InputSchema:  schemaRef(commandSchemaID, "getContextPayload"),
			OutputSchema: schemaRef(resultSchemaID, "getContextResult"),
		},
		{
			Name:         toolFetch,
			Title:        "Fetch Task Context",
			Description:  "Fetch deterministic task context by pointer keys and optional expected versions.",
			InputSchema:  schemaRef(commandSchemaID, "fetchPayload"),
			OutputSchema: schemaRef(resultSchemaID, "fetchResult"),
		},
		{
			Name:         string(v1.CommandProposeMemory),
			Title:        "Propose Durable Memory",
			Description:  "Submit a memory candidate tied to evidence and receipt scope.",
			InputSchema:  schemaRef(commandSchemaID, "proposeMemoryPayload"),
			OutputSchema: schemaRef(resultSchemaID, "proposeMemoryResult"),
		},
		{
			Name:         string(v1.CommandReportCompletion),
			Title:        "Report Task Completion",
			Description:  "Validate changed files against the active receipt and persist run summary.",
			InputSchema:  schemaRef(commandSchemaID, "reportCompletionPayload"),
			OutputSchema: schemaRef(resultSchemaID, "reportCompletionResult"),
		},
		{
			Name:         toolWork,
			Title:        "Submit Completion Work",
			Description:  "Submit plan-scoped work item updates with status and optional outcomes.",
			InputSchema:  schemaRef(commandSchemaID, "workPayload"),
			OutputSchema: schemaRef(resultSchemaID, "workResult"),
		},
	}
}

const (
	schemaDraft202012 = "https://json-schema.org/draft/2020-12/schema"
	commandSchemaID   = "https://agents-context.dev/spec/v1/cli.command.schema.json"
	resultSchemaID    = "https://agents-context.dev/spec/v1/cli.result.schema.json"
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

	switch tool {
	case string(v1.CommandGetContext):
		var p v1.GetContextPayload
		if err := json.Unmarshal(input, &p); err != nil {
			return mcpToolInputError(ctx, logger, tool, "", err)
		}
		if err := validateToolInput(tool, p); err != nil {
			return mcpToolInputError(ctx, logger, tool, strings.TrimSpace(p.ProjectID), err)
		}
		projectID := strings.TrimSpace(p.ProjectID)
		logMCPValidateSuccess(ctx, logger, tool, projectID)
		logMCPDispatchStart(ctx, logger, tool, projectID)
		result, apiErr := svc.GetContext(ctx, p)
		if apiErr != nil {
			return mcpDispatchError(ctx, logger, tool, projectID, apiErr)
		}
		logMCPDispatchSuccess(ctx, logger, tool, projectID)
		logMCPResultSuccess(ctx, logger, tool, projectID)
		return result, nil
	case toolFetch:
		var p v1.FetchPayload
		if err := json.Unmarshal(input, &p); err != nil {
			return mcpToolInputError(ctx, logger, tool, "", err)
		}
		if err := validateToolInput(tool, p); err != nil {
			return mcpToolInputError(ctx, logger, tool, strings.TrimSpace(p.ProjectID), err)
		}
		projectID := strings.TrimSpace(p.ProjectID)
		logMCPValidateSuccess(ctx, logger, tool, projectID)
		logMCPDispatchStart(ctx, logger, tool, projectID)
		result, apiErr := svc.Fetch(ctx, p)
		if apiErr != nil {
			return mcpDispatchError(ctx, logger, tool, projectID, apiErr)
		}
		logMCPDispatchSuccess(ctx, logger, tool, projectID)
		logMCPResultSuccess(ctx, logger, tool, projectID)
		return result, nil
	case string(v1.CommandProposeMemory):
		var p v1.ProposeMemoryPayload
		if err := json.Unmarshal(input, &p); err != nil {
			return mcpToolInputError(ctx, logger, tool, "", err)
		}
		if err := validateToolInput(tool, p); err != nil {
			return mcpToolInputError(ctx, logger, tool, strings.TrimSpace(p.ProjectID), err)
		}
		projectID := strings.TrimSpace(p.ProjectID)
		logMCPValidateSuccess(ctx, logger, tool, projectID)
		logMCPDispatchStart(ctx, logger, tool, projectID)
		result, apiErr := svc.ProposeMemory(ctx, p)
		if apiErr != nil {
			return mcpDispatchError(ctx, logger, tool, projectID, apiErr)
		}
		logMCPDispatchSuccess(ctx, logger, tool, projectID)
		logMCPResultSuccess(ctx, logger, tool, projectID)
		return result, nil
	case string(v1.CommandReportCompletion):
		var p v1.ReportCompletionPayload
		if err := json.Unmarshal(input, &p); err != nil {
			return mcpToolInputError(ctx, logger, tool, "", err)
		}
		if err := validateToolInput(tool, p); err != nil {
			return mcpToolInputError(ctx, logger, tool, strings.TrimSpace(p.ProjectID), err)
		}
		projectID := strings.TrimSpace(p.ProjectID)
		logMCPValidateSuccess(ctx, logger, tool, projectID)
		logMCPDispatchStart(ctx, logger, tool, projectID)
		result, apiErr := svc.ReportCompletion(ctx, p)
		if apiErr != nil {
			return mcpDispatchError(ctx, logger, tool, projectID, apiErr)
		}
		logMCPDispatchSuccess(ctx, logger, tool, projectID)
		logMCPResultSuccess(ctx, logger, tool, projectID)
		return result, nil
	case toolWork:
		var p v1.WorkPayload
		if err := json.Unmarshal(input, &p); err != nil {
			return mcpToolInputError(ctx, logger, tool, "", err)
		}
		if err := validateToolInput(tool, p); err != nil {
			return mcpToolInputError(ctx, logger, tool, strings.TrimSpace(p.ProjectID), err)
		}
		projectID := strings.TrimSpace(p.ProjectID)
		logMCPValidateSuccess(ctx, logger, tool, projectID)
		logMCPDispatchStart(ctx, logger, tool, projectID)
		result, apiErr := svc.Work(ctx, p)
		if apiErr != nil {
			return mcpDispatchError(ctx, logger, tool, projectID, apiErr)
		}
		logMCPDispatchSuccess(ctx, logger, tool, projectID)
		logMCPResultSuccess(ctx, logger, tool, projectID)
		return result, nil
	default:
		err := core.NewError("UNKNOWN_TOOL", "tool is not supported in v1", map[string]any{"tool": tool})
		logger.Error(ctx, logging.EventMCPIngressValidate, "ok", false, "tool", tool, "error_code", err.Code)
		logger.Error(ctx, logging.EventMCPFailure, "stage", "validate", "tool", tool, "error_code", err.Code)
		logger.Info(ctx, logging.EventMCPResult, "ok", false, "tool", tool, "error_code", err.Code)
		return nil, err
	}
}

func validateToolInput(tool string, payload any) error {
	raw, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	wrapped := map[string]any{
		"version":    v1.Version,
		"command":    tool,
		"request_id": "mcp.invoke",
		"payload":    json.RawMessage(raw),
	}
	blob, err := json.Marshal(wrapped)
	if err != nil {
		return err
	}
	_, _, valErr := v1.DecodeAndValidateCommand(blob)
	if valErr != nil {
		return fmt.Errorf("%s: %s", valErr.Code, valErr.Message)
	}
	return nil
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
