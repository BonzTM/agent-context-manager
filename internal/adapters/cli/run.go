package cli

import (
	"context"
	"encoding/json"
	"io"
	"reflect"
	"strings"
	"time"

	"github.com/bonztm/agent-context-manager/internal/contracts/v1"
	"github.com/bonztm/agent-context-manager/internal/core"
	"github.com/bonztm/agent-context-manager/internal/logging"
)

const (
	commandFetch = v1.Command("fetch")
	commandWork  = v1.Command("work")
)

func Run(ctx context.Context, svc core.Service, in io.Reader, out io.Writer, now func() time.Time) int {
	return RunWithLogger(ctx, svc, in, out, now, nil)
}

func RunWithLogger(ctx context.Context, svc core.Service, in io.Reader, out io.Writer, now func() time.Time, logger logging.Logger) int {
	logger = logging.Normalize(logger)

	input, err := io.ReadAll(in)
	if err != nil {
		logger.Error(ctx, logging.EventCLIIngressRead, "ok", false, "error_code", "READ_FAILED")
		logger.Error(ctx, logging.EventCLIFailure, "stage", "read", "error_code", "READ_FAILED")
		writeEnvelope(out, v1.ResultEnvelope{
			Version:   v1.Version,
			Command:   "",
			RequestID: "",
			OK:        false,
			Timestamp: now().UTC().Format(time.RFC3339),
			Error:     &v1.ErrorPayload{Code: "READ_FAILED", Message: err.Error()},
		})
		logger.Info(ctx, logging.EventCLIResult, "ok", false, "error_code", "READ_FAILED")
		return 1
	}
	logger.Info(ctx, logging.EventCLIIngressRead, "ok", true, "bytes", len(input))

	env, payload, valErr := v1.DecodeAndValidateCommand(input)
	projectID := projectIDFromPayload(payload)
	if valErr != nil {
		logger.Error(ctx, logging.EventCLIIngressValidate, "ok", false, "command", string(env.Command), "request_id", env.RequestID, "error_code", valErr.Code)
		logger.Error(ctx, logging.EventCLIFailure, "stage", "validate", "command", string(env.Command), "request_id", env.RequestID, "error_code", valErr.Code)
		writeEnvelope(out, v1.ResultEnvelope{
			Version:   v1.Version,
			Command:   env.Command,
			RequestID: env.RequestID,
			OK:        false,
			Timestamp: now().UTC().Format(time.RFC3339),
			Error:     valErr,
		})
		logger.Info(ctx, logging.EventCLIResult, "ok", false, "command", string(env.Command), "request_id", env.RequestID, "error_code", valErr.Code)
		return 1
	}
	validateFields := []any{"ok", true, "command", string(env.Command), "request_id", env.RequestID}
	if projectID != "" {
		validateFields = append(validateFields, "project_id", projectID)
	}
	logger.Info(ctx, logging.EventCLIIngressValidate, validateFields...)

	dispatchStartFields := []any{"phase", "start", "command", string(env.Command), "request_id", env.RequestID}
	if projectID != "" {
		dispatchStartFields = append(dispatchStartFields, "project_id", projectID)
	}
	logger.Info(ctx, logging.EventCLIDispatch, dispatchStartFields...)
	result, apiErr := dispatch(ctx, svc, env.Command, payload)
	if apiErr != nil {
		dispatchFailFields := []any{"phase", "finish", "ok", false, "command", string(env.Command), "request_id", env.RequestID, "error_code", apiErr.Code}
		if projectID != "" {
			dispatchFailFields = append(dispatchFailFields, "project_id", projectID)
		}
		logger.Error(ctx, logging.EventCLIDispatch, dispatchFailFields...)
		logger.Error(ctx, logging.EventCLIFailure, "stage", "dispatch", "command", string(env.Command), "request_id", env.RequestID, "error_code", apiErr.Code)
		writeEnvelope(out, v1.ResultEnvelope{
			Version:   v1.Version,
			Command:   env.Command,
			RequestID: env.RequestID,
			OK:        false,
			Timestamp: now().UTC().Format(time.RFC3339),
			Error:     apiErr.ToPayload(),
		})
		resultFields := []any{"ok", false, "command", string(env.Command), "request_id", env.RequestID, "error_code", apiErr.Code}
		if projectID != "" {
			resultFields = append(resultFields, "project_id", projectID)
		}
		logger.Info(ctx, logging.EventCLIResult, resultFields...)
		return 1
	}
	dispatchFinishFields := []any{"phase", "finish", "ok", true, "command", string(env.Command), "request_id", env.RequestID}
	if projectID != "" {
		dispatchFinishFields = append(dispatchFinishFields, "project_id", projectID)
	}
	logger.Info(ctx, logging.EventCLIDispatch, dispatchFinishFields...)

	writeEnvelope(out, v1.ResultEnvelope{
		Version:   v1.Version,
		Command:   env.Command,
		RequestID: env.RequestID,
		OK:        true,
		Timestamp: now().UTC().Format(time.RFC3339),
		Result:    result,
	})
	resultFields := []any{"ok", true, "command", string(env.Command), "request_id", env.RequestID}
	if projectID != "" {
		resultFields = append(resultFields, "project_id", projectID)
	}
	logger.Info(ctx, logging.EventCLIResult, resultFields...)
	return 0
}

func dispatch(ctx context.Context, svc core.Service, command v1.Command, payload any) (any, *core.APIError) {
	switch command {
	case v1.CommandGetContext:
		return svc.GetContext(ctx, payload.(v1.GetContextPayload))
	case commandFetch:
		return svc.Fetch(ctx, payload.(v1.FetchPayload))
	case v1.CommandProposeMemory:
		return svc.ProposeMemory(ctx, payload.(v1.ProposeMemoryPayload))
	case commandWork:
		return svc.Work(ctx, payload.(v1.WorkPayload))
	case v1.CommandHistorySearch:
		return svc.HistorySearch(ctx, payload.(v1.HistorySearchPayload))
	case v1.CommandReportCompletion:
		return svc.ReportCompletion(ctx, payload.(v1.ReportCompletionPayload))
	case v1.CommandSync:
		return svc.Sync(ctx, payload.(v1.SyncPayload))
	case v1.CommandHealthCheck:
		return svc.HealthCheck(ctx, payload.(v1.HealthCheckPayload))
	case v1.CommandHealthFix:
		return svc.HealthFix(ctx, payload.(v1.HealthFixPayload))
	case v1.CommandCoverage:
		return svc.Coverage(ctx, payload.(v1.CoveragePayload))
	case v1.CommandEval:
		return svc.Eval(ctx, payload.(v1.EvalPayload))
	case v1.CommandVerify:
		return svc.Verify(ctx, payload.(v1.VerifyPayload))
	case v1.CommandBootstrap:
		return svc.Bootstrap(ctx, payload.(v1.BootstrapPayload))
	default:
		return nil, core.NewError("INVALID_COMMAND", "command is not recognized", nil)
	}
}

func writeEnvelope(out io.Writer, env v1.ResultEnvelope) {
	enc := json.NewEncoder(out)
	enc.SetIndent("", "  ")
	_ = enc.Encode(env)
}

func projectIDFromPayload(payload any) string {
	switch p := payload.(type) {
	case v1.GetContextPayload:
		return strings.TrimSpace(p.ProjectID)
	case v1.FetchPayload:
		return strings.TrimSpace(p.ProjectID)
	case v1.ProposeMemoryPayload:
		return strings.TrimSpace(p.ProjectID)
	case v1.ReportCompletionPayload:
		return strings.TrimSpace(p.ProjectID)
	case v1.WorkPayload:
		return strings.TrimSpace(p.ProjectID)
	case v1.HistorySearchPayload:
		return strings.TrimSpace(p.ProjectID)
	case v1.SyncPayload:
		return strings.TrimSpace(p.ProjectID)
	case v1.HealthCheckPayload:
		return strings.TrimSpace(p.ProjectID)
	case v1.HealthFixPayload:
		return strings.TrimSpace(p.ProjectID)
	case v1.CoveragePayload:
		return strings.TrimSpace(p.ProjectID)
	case v1.EvalPayload:
		return strings.TrimSpace(p.ProjectID)
	case v1.VerifyPayload:
		return strings.TrimSpace(p.ProjectID)
	case v1.BootstrapPayload:
		return strings.TrimSpace(p.ProjectID)
	case map[string]any:
		return projectIDFromMap(p)
	default:
		return projectIDFromStruct(payload)
	}
}

func projectIDFromMap(payload map[string]any) string {
	value, ok := payload["project_id"]
	if !ok {
		return ""
	}
	projectID, ok := value.(string)
	if !ok {
		return ""
	}
	return strings.TrimSpace(projectID)
}

func projectIDFromStruct(payload any) string {
	value := reflect.ValueOf(payload)
	if !value.IsValid() {
		return ""
	}
	if value.Kind() == reflect.Pointer {
		if value.IsNil() {
			return ""
		}
		value = value.Elem()
	}
	if value.Kind() != reflect.Struct {
		return ""
	}
	projectField := value.FieldByName("ProjectID")
	if !projectField.IsValid() || projectField.Kind() != reflect.String {
		return ""
	}
	return strings.TrimSpace(projectField.String())
}
