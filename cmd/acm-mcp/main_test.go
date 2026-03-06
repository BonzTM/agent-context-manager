package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/bonztm/agent-context-manager/internal/contracts/v1"
	"github.com/bonztm/agent-context-manager/internal/core"
	"github.com/bonztm/agent-context-manager/internal/logging"
	"github.com/bonztm/agent-context-manager/internal/runtime"
)

func TestInvokeWithDeps_SuccessWritesResultEnvelope(t *testing.T) {
	var out bytes.Buffer
	code := invokeWithDeps(
		context.Background(),
		logging.NewRecorder(),
		[]string{"--tool", "get_context"},
		strings.NewReader(`{"project_id":"my-cool-app","task_text":"fix drift","phase":"execute"}`),
		&out,
		fixedMCPNow,
		func(_ context.Context, _ logging.Logger) (core.Service, runtime.CleanupFunc, error) {
			return mcpMainFakeService{}, func() {}, nil
		},
	)
	if code != 0 {
		t.Fatalf("unexpected exit code: got %d want 0", code)
	}

	var env v1.ResultEnvelope
	if err := json.Unmarshal(out.Bytes(), &env); err != nil {
		t.Fatalf("unmarshal envelope: %v", err)
	}
	if !env.OK {
		t.Fatalf("expected ok=true, got error %+v", env.Error)
	}
	if env.Command != v1.CommandGetContext {
		t.Fatalf("unexpected command: %q", env.Command)
	}
	if env.RequestID != mcpInvokeRequestID {
		t.Fatalf("unexpected request_id: %q", env.RequestID)
	}
	if env.Timestamp != fixedMCPNow().UTC().Format(time.RFC3339) {
		t.Fatalf("unexpected timestamp: %q", env.Timestamp)
	}
}

func TestInvokeWithDeps_ServiceFailureWritesResultEnvelope(t *testing.T) {
	var out bytes.Buffer
	code := invokeWithDeps(
		context.Background(),
		logging.NewRecorder(),
		[]string{"--tool", "get_context"},
		strings.NewReader(`{"project_id":"my-cool-app","task_text":"fix drift","phase":"execute"}`),
		&out,
		fixedMCPNow,
		func(_ context.Context, _ logging.Logger) (core.Service, runtime.CleanupFunc, error) {
			return mcpMainFailingService{}, func() {}, nil
		},
	)
	if code != 1 {
		t.Fatalf("unexpected exit code: got %d want 1", code)
	}

	var env v1.ResultEnvelope
	if err := json.Unmarshal(out.Bytes(), &env); err != nil {
		t.Fatalf("unmarshal envelope: %v", err)
	}
	if env.OK {
		t.Fatal("expected ok=false")
	}
	if env.Error == nil || env.Error.Code != "NOT_IMPLEMENTED" {
		t.Fatalf("unexpected error payload: %+v", env.Error)
	}
	if env.Command != v1.CommandGetContext {
		t.Fatalf("unexpected command: %q", env.Command)
	}
}

func TestInvokeWithDeps_MissingToolWritesStructuredError(t *testing.T) {
	var out bytes.Buffer
	code := invokeWithDeps(
		context.Background(),
		logging.NewRecorder(),
		nil,
		strings.NewReader(`{}`),
		&out,
		fixedMCPNow,
		func(_ context.Context, _ logging.Logger) (core.Service, runtime.CleanupFunc, error) {
			t.Fatal("service factory should not be called")
			return nil, nil, nil
		},
	)
	if code != 2 {
		t.Fatalf("unexpected exit code: got %d want 2", code)
	}

	var env invokeWrapperEnvelope
	if err := json.Unmarshal(out.Bytes(), &env); err != nil {
		t.Fatalf("unmarshal envelope: %v", err)
	}
	if env.OK {
		t.Fatal("expected ok=false")
	}
	if env.Error == nil || env.Error.Code != "MISSING_TOOL" {
		t.Fatalf("unexpected error payload: %+v", env.Error)
	}
	if env.Tool != "" {
		t.Fatalf("expected empty tool, got %q", env.Tool)
	}
}

func TestInvokeWithDeps_InvalidFlagsWriteStructuredError(t *testing.T) {
	var out bytes.Buffer
	code := invokeWithDeps(
		context.Background(),
		logging.NewRecorder(),
		[]string{"--bogus"},
		strings.NewReader(`{}`),
		&out,
		fixedMCPNow,
		func(_ context.Context, _ logging.Logger) (core.Service, runtime.CleanupFunc, error) {
			t.Fatal("service factory should not be called")
			return nil, nil, nil
		},
	)
	if code != 2 {
		t.Fatalf("unexpected exit code: got %d want 2", code)
	}

	var env invokeWrapperEnvelope
	if err := json.Unmarshal(out.Bytes(), &env); err != nil {
		t.Fatalf("unmarshal envelope: %v", err)
	}
	if env.OK {
		t.Fatal("expected ok=false")
	}
	if env.Error == nil || env.Error.Code != "INVALID_FLAGS" {
		t.Fatalf("unexpected error payload: %+v", env.Error)
	}
}

func TestInvokeWithDeps_UnknownToolWritesStructuredError(t *testing.T) {
	var out bytes.Buffer
	code := invokeWithDeps(
		context.Background(),
		logging.NewRecorder(),
		[]string{"--tool", "bogus"},
		strings.NewReader(`{}`),
		&out,
		fixedMCPNow,
		func(_ context.Context, _ logging.Logger) (core.Service, runtime.CleanupFunc, error) {
			t.Fatal("service factory should not be called")
			return nil, nil, nil
		},
	)
	if code != 2 {
		t.Fatalf("unexpected exit code: got %d want 2", code)
	}

	var env invokeWrapperEnvelope
	if err := json.Unmarshal(out.Bytes(), &env); err != nil {
		t.Fatalf("unmarshal envelope: %v", err)
	}
	if env.OK {
		t.Fatal("expected ok=false")
	}
	if env.Tool != "bogus" {
		t.Fatalf("unexpected tool: %q", env.Tool)
	}
	if env.Error == nil || env.Error.Code != "UNKNOWN_TOOL" {
		t.Fatalf("unexpected error payload: %+v", env.Error)
	}
}

func TestInvokeWithDeps_HelpWritesUsage(t *testing.T) {
	var out bytes.Buffer
	code := invokeWithDeps(
		context.Background(),
		logging.NewRecorder(),
		[]string{"--help"},
		strings.NewReader(`{}`),
		&out,
		fixedMCPNow,
		func(_ context.Context, _ logging.Logger) (core.Service, runtime.CleanupFunc, error) {
			t.Fatal("service factory should not be called")
			return nil, nil, nil
		},
	)
	if code != 0 {
		t.Fatalf("unexpected exit code: got %d want 0", code)
	}
	text := out.String()
	if !strings.Contains(text, "acm-mcp invoke - invoke one MCP tool with JSON input") {
		t.Fatalf("unexpected help output: %q", text)
	}
	if !strings.Contains(text, "--tool <name>") {
		t.Fatalf("unexpected help output: %q", text)
	}
}

func fixedMCPNow() time.Time {
	return time.Date(2026, 3, 5, 12, 0, 0, 0, time.UTC)
}

type mcpMainFakeService struct{}

func (mcpMainFakeService) GetContext(_ context.Context, _ v1.GetContextPayload) (v1.GetContextResult, *core.APIError) {
	return v1.GetContextResult{Status: "insufficient_context"}, nil
}

func (mcpMainFakeService) Fetch(_ context.Context, _ v1.FetchPayload) (v1.FetchResult, *core.APIError) {
	return v1.FetchResult{}, nil
}

func (mcpMainFakeService) ProposeMemory(_ context.Context, _ v1.ProposeMemoryPayload) (v1.ProposeMemoryResult, *core.APIError) {
	return v1.ProposeMemoryResult{}, nil
}

func (mcpMainFakeService) ReportCompletion(_ context.Context, _ v1.ReportCompletionPayload) (v1.ReportCompletionResult, *core.APIError) {
	return v1.ReportCompletionResult{}, nil
}

func (mcpMainFakeService) Work(_ context.Context, _ v1.WorkPayload) (v1.WorkResult, *core.APIError) {
	return v1.WorkResult{}, nil
}

func (mcpMainFakeService) Sync(_ context.Context, _ v1.SyncPayload) (v1.SyncResult, *core.APIError) {
	return v1.SyncResult{}, nil
}

func (mcpMainFakeService) HealthCheck(_ context.Context, _ v1.HealthCheckPayload) (v1.HealthCheckResult, *core.APIError) {
	return v1.HealthCheckResult{}, nil
}

func (mcpMainFakeService) HealthFix(_ context.Context, _ v1.HealthFixPayload) (v1.HealthFixResult, *core.APIError) {
	return v1.HealthFixResult{}, nil
}

func (mcpMainFakeService) Coverage(_ context.Context, _ v1.CoveragePayload) (v1.CoverageResult, *core.APIError) {
	return v1.CoverageResult{}, nil
}

func (mcpMainFakeService) Eval(_ context.Context, _ v1.EvalPayload) (v1.EvalResult, *core.APIError) {
	return v1.EvalResult{}, nil
}

func (mcpMainFakeService) Verify(_ context.Context, _ v1.VerifyPayload) (v1.VerifyResult, *core.APIError) {
	return v1.VerifyResult{}, nil
}

func (mcpMainFakeService) Bootstrap(_ context.Context, _ v1.BootstrapPayload) (v1.BootstrapResult, *core.APIError) {
	return v1.BootstrapResult{}, nil
}

type mcpMainFailingService struct {
	mcpMainFakeService
}

func (mcpMainFailingService) GetContext(_ context.Context, _ v1.GetContextPayload) (v1.GetContextResult, *core.APIError) {
	return v1.GetContextResult{}, core.NewError("NOT_IMPLEMENTED", errors.New("boom").Error(), nil)
}
