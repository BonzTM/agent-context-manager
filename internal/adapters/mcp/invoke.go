package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/bonztm/agent-context-manager/internal/contracts/v1"
	"github.com/bonztm/agent-context-manager/internal/core"
	"github.com/bonztm/agent-context-manager/internal/logging"
)

const (
	toolFetch       = "fetch"
	toolWork        = "work"
	toolSync        = "sync"
	toolHealthCheck = "health_check"
	toolHealthFix   = "health_fix"
	toolCoverage    = "coverage"
	toolEval        = "eval"
	toolVerify      = "verify"
	toolBootstrap   = "bootstrap"
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
		{
			Name:         toolSync,
			Title:        "Sync Project Inventory",
			Description:  "Refresh indexed repository pointers from git or the working tree.",
			InputSchema:  schemaRef(commandSchemaID, "syncPayload"),
			OutputSchema: schemaRef(resultSchemaID, "syncResult"),
		},
		{
			Name:         toolHealthCheck,
			Title:        "Check Project Health",
			Description:  "Inspect ACM repository health and return findings without making changes.",
			InputSchema:  schemaRef(commandSchemaID, "healthCheckPayload"),
			OutputSchema: schemaRef(resultSchemaID, "healthCheckResult"),
		},
		{
			Name:         toolHealthFix,
			Title:        "Fix Project Health",
			Description:  "Plan or apply ACM health fixes such as ruleset sync and working tree repair.",
			InputSchema:  schemaRef(commandSchemaID, "healthFixPayload"),
			OutputSchema: schemaRef(resultSchemaID, "healthFixResult"),
		},
		{
			Name:         toolCoverage,
			Title:        "Measure Coverage",
			Description:  "Report repository indexing coverage against the current project tree.",
			InputSchema:  schemaRef(commandSchemaID, "coveragePayload"),
			OutputSchema: schemaRef(resultSchemaID, "coverageResult"),
		},
		{
			Name:         toolEval,
			Title:        "Run Retrieval Evaluation",
			Description:  "Run retrieval evaluation cases against ACM context selection behavior.",
			InputSchema:  schemaRef(commandSchemaID, "evalPayload"),
			OutputSchema: schemaRef(resultSchemaID, "evalResult"),
		},
		{
			Name:         toolVerify,
			Title:        "Run Executable Verification",
			Description:  "Select and execute repo-defined verification checks from acm test definitions.",
			InputSchema:  schemaRef(commandSchemaID, "verifyPayload"),
			OutputSchema: schemaRef(resultSchemaID, "verifyResult"),
		},
		{
			Name:         toolBootstrap,
			Title:        "Bootstrap Repository",
			Description:  "Scan a repository and generate or persist initial ACM candidates.",
			InputSchema:  schemaRef(commandSchemaID, "bootstrapPayload"),
			OutputSchema: schemaRef(resultSchemaID, "bootstrapResult"),
		},
	}
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

	switch tool {
	case string(v1.CommandGetContext):
		return invokeTypedTool(ctx, logger, tool, input, func(p v1.GetContextPayload) string {
			return p.ProjectID
		}, func(p v1.GetContextPayload) (v1.GetContextResult, *core.APIError) {
			return svc.GetContext(ctx, p)
		})
	case toolFetch:
		return invokeTypedTool(ctx, logger, tool, input, func(p v1.FetchPayload) string {
			return p.ProjectID
		}, func(p v1.FetchPayload) (v1.FetchResult, *core.APIError) {
			return svc.Fetch(ctx, p)
		})
	case string(v1.CommandProposeMemory):
		return invokeTypedTool(ctx, logger, tool, input, func(p v1.ProposeMemoryPayload) string {
			return p.ProjectID
		}, func(p v1.ProposeMemoryPayload) (v1.ProposeMemoryResult, *core.APIError) {
			return svc.ProposeMemory(ctx, p)
		})
	case string(v1.CommandReportCompletion):
		return invokeTypedTool(ctx, logger, tool, input, func(p v1.ReportCompletionPayload) string {
			return p.ProjectID
		}, func(p v1.ReportCompletionPayload) (v1.ReportCompletionResult, *core.APIError) {
			return svc.ReportCompletion(ctx, p)
		})
	case toolWork:
		return invokeTypedTool(ctx, logger, tool, input, func(p v1.WorkPayload) string {
			return p.ProjectID
		}, func(p v1.WorkPayload) (v1.WorkResult, *core.APIError) {
			return svc.Work(ctx, p)
		})
	case toolSync:
		return invokeTypedTool(ctx, logger, tool, input, func(p v1.SyncPayload) string {
			return p.ProjectID
		}, func(p v1.SyncPayload) (v1.SyncResult, *core.APIError) {
			return svc.Sync(ctx, p)
		})
	case toolHealthCheck:
		return invokeTypedTool(ctx, logger, tool, input, func(p v1.HealthCheckPayload) string {
			return p.ProjectID
		}, func(p v1.HealthCheckPayload) (v1.HealthCheckResult, *core.APIError) {
			return svc.HealthCheck(ctx, p)
		})
	case toolHealthFix:
		return invokeTypedTool(ctx, logger, tool, input, func(p v1.HealthFixPayload) string {
			return p.ProjectID
		}, func(p v1.HealthFixPayload) (v1.HealthFixResult, *core.APIError) {
			return svc.HealthFix(ctx, p)
		})
	case toolCoverage:
		return invokeTypedTool(ctx, logger, tool, input, func(p v1.CoveragePayload) string {
			return p.ProjectID
		}, func(p v1.CoveragePayload) (v1.CoverageResult, *core.APIError) {
			return svc.Coverage(ctx, p)
		})
	case toolEval:
		return invokeTypedTool(ctx, logger, tool, input, func(p v1.EvalPayload) string {
			return p.ProjectID
		}, func(p v1.EvalPayload) (v1.EvalResult, *core.APIError) {
			return svc.Eval(ctx, p)
		})
	case toolVerify:
		return invokeTypedTool(ctx, logger, tool, input, func(p v1.VerifyPayload) string {
			return p.ProjectID
		}, func(p v1.VerifyPayload) (v1.VerifyResult, *core.APIError) {
			return svc.Verify(ctx, p)
		})
	case toolBootstrap:
		return invokeTypedTool(ctx, logger, tool, input, func(p v1.BootstrapPayload) string {
			return p.ProjectID
		}, func(p v1.BootstrapPayload) (v1.BootstrapResult, *core.APIError) {
			return svc.Bootstrap(ctx, p)
		})
	default:
		err := core.NewError("UNKNOWN_TOOL", "tool is not supported in v1", map[string]any{"tool": tool})
		logger.Error(ctx, logging.EventMCPIngressValidate, "ok", false, "tool", tool, "error_code", err.Code)
		logger.Error(ctx, logging.EventMCPFailure, "stage", "validate", "tool", tool, "error_code", err.Code)
		logger.Info(ctx, logging.EventMCPResult, "ok", false, "tool", tool, "error_code", err.Code)
		return nil, err
	}
}

func invokeTypedTool[Payload any, Result any](ctx context.Context, logger logging.Logger, tool string, input []byte, projectID func(Payload) string, run func(Payload) (Result, *core.APIError)) (any, *core.APIError) {
	rawProjectID := projectIDFromRawToolInput(input)
	if err := validateRawToolInput(tool, json.RawMessage(input)); err != nil {
		return mcpToolInputError(ctx, logger, tool, rawProjectID, err)
	}

	var payload Payload
	if err := json.Unmarshal(input, &payload); err != nil {
		return mcpToolInputError(ctx, logger, tool, rawProjectID, err)
	}

	normalizedProjectID := strings.TrimSpace(projectID(payload))
	if normalizedProjectID == "" {
		normalizedProjectID = rawProjectID
	}
	logMCPValidateSuccess(ctx, logger, tool, normalizedProjectID)
	logMCPDispatchStart(ctx, logger, tool, normalizedProjectID)

	result, apiErr := run(payload)
	if apiErr != nil {
		return mcpDispatchError(ctx, logger, tool, normalizedProjectID, apiErr)
	}

	logMCPDispatchSuccess(ctx, logger, tool, normalizedProjectID)
	logMCPResultSuccess(ctx, logger, tool, normalizedProjectID)
	return result, nil
}

func validateRawToolInput(tool string, raw json.RawMessage) error {
	wrapped := map[string]any{
		"version":    v1.Version,
		"command":    tool,
		"request_id": "mcp.invoke",
		"payload":    raw,
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
