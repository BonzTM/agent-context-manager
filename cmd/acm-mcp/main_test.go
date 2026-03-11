package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/bonztm/agent-context-manager/internal/adapters/mcp"
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
		[]string{"--tool", "context"},
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
	if env.Command != v1.CommandContext {
		t.Fatalf("unexpected command: %q", env.Command)
	}
	if env.RequestID != mcpInvokeRequestID {
		t.Fatalf("unexpected request_id: %q", env.RequestID)
	}
	if env.Timestamp != fixedMCPNow().UTC().Format(time.RFC3339) {
		t.Fatalf("unexpected timestamp: %q", env.Timestamp)
	}
}

func TestInvokeWithDeps_ServiceFailureWritesStructuredError(t *testing.T) {
	var out bytes.Buffer
	code := invokeWithDeps(
		context.Background(),
		logging.NewRecorder(),
		[]string{"--tool", "context"},
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

	var env invokeWrapperEnvelope
	if err := json.Unmarshal(out.Bytes(), &env); err != nil {
		t.Fatalf("unmarshal envelope: %v", err)
	}
	if env.OK {
		t.Fatal("expected ok=false")
	}
	if env.Tool != "context" {
		t.Fatalf("unexpected tool: %q", env.Tool)
	}
	if env.Error == nil || env.Error.Code != "NOT_IMPLEMENTED" {
		t.Fatalf("unexpected error payload: %+v", env.Error)
	}
}

func TestInvokeWithDeps_ReadFailureWritesStructuredError(t *testing.T) {
	missingPath := filepath.Join(t.TempDir(), "missing.json")
	var out bytes.Buffer
	code := invokeWithDeps(
		context.Background(),
		logging.NewRecorder(),
		[]string{"--tool", "context", "--in", missingPath},
		strings.NewReader(`{}`),
		&out,
		fixedMCPNow,
		func(_ context.Context, _ logging.Logger) (core.Service, runtime.CleanupFunc, error) {
			t.Fatal("service factory should not be called")
			return nil, nil, nil
		},
	)
	if code != 1 {
		t.Fatalf("unexpected exit code: got %d want 1", code)
	}

	var env invokeWrapperEnvelope
	if err := json.Unmarshal(out.Bytes(), &env); err != nil {
		t.Fatalf("unmarshal envelope: %v", err)
	}
	if env.OK {
		t.Fatal("expected ok=false")
	}
	if env.Tool != "context" {
		t.Fatalf("unexpected tool: %q", env.Tool)
	}
	if env.Error == nil || env.Error.Code != "READ_FAILED" {
		t.Fatalf("unexpected error payload: %+v", env.Error)
	}
}

func TestInvokeWithDeps_ServiceInitFailureWritesStructuredError(t *testing.T) {
	var out bytes.Buffer
	code := invokeWithDeps(
		context.Background(),
		logging.NewRecorder(),
		[]string{"--tool", "context"},
		strings.NewReader(`{"project_id":"my-cool-app","task_text":"fix drift","phase":"execute"}`),
		&out,
		fixedMCPNow,
		func(_ context.Context, _ logging.Logger) (core.Service, runtime.CleanupFunc, error) {
			return nil, func() {}, errors.New("boot failed")
		},
	)
	if code != 1 {
		t.Fatalf("unexpected exit code: got %d want 1", code)
	}

	var env invokeWrapperEnvelope
	if err := json.Unmarshal(out.Bytes(), &env); err != nil {
		t.Fatalf("unmarshal envelope: %v", err)
	}
	if env.OK {
		t.Fatal("expected ok=false")
	}
	if env.Tool != "context" {
		t.Fatalf("unexpected tool: %q", env.Tool)
	}
	if env.Error == nil || env.Error.Code != "SERVICE_INIT_FAILED" {
		t.Fatalf("unexpected error payload: %+v", env.Error)
	}
}

func TestInvokeWithDeps_InvalidToolInputWritesStructuredError(t *testing.T) {
	var out bytes.Buffer
	code := invokeWithDeps(
		context.Background(),
		logging.NewRecorder(),
		[]string{"--tool", "context"},
		strings.NewReader(`{`),
		&out,
		fixedMCPNow,
		func(_ context.Context, _ logging.Logger) (core.Service, runtime.CleanupFunc, error) {
			return mcpMainFakeService{}, func() {}, nil
		},
	)
	if code != 1 {
		t.Fatalf("unexpected exit code: got %d want 1", code)
	}

	var env invokeWrapperEnvelope
	if err := json.Unmarshal(out.Bytes(), &env); err != nil {
		t.Fatalf("unmarshal envelope: %v", err)
	}
	if env.OK {
		t.Fatal("expected ok=false")
	}
	if env.Tool != "context" {
		t.Fatalf("unexpected tool: %q", env.Tool)
	}
	if env.Error == nil || env.Error.Code != "INVALID_JSON" {
		t.Fatalf("unexpected error payload: %+v", env.Error)
	}
}

func TestInvokeWithDeps_HistoryDispatchesThroughWrapper(t *testing.T) {
	var out bytes.Buffer
	code := invokeWithDeps(
		context.Background(),
		logging.NewRecorder(),
		[]string{"--tool", "history"},
		strings.NewReader(`{"project_id":"my-cool-app","entity":"memory","query":"bootstrap"}`),
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
	if env.Command != v1.CommandHistorySearch {
		t.Fatalf("unexpected command: %q", env.Command)
	}
}

func TestInvokeWithDeps_ReviewDispatchesThroughWrapper(t *testing.T) {
	var out bytes.Buffer
	code := invokeWithDeps(
		context.Background(),
		logging.NewRecorder(),
		[]string{"--tool", "review"},
		strings.NewReader(`{"project_id":"my-cool-app","receipt_id":"receipt-1234","outcome":"No blocking review findings."}`),
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
	if env.Command != v1.CommandReview {
		t.Fatalf("unexpected command: %q", env.Command)
	}
}

func TestInvokeWithDeps_ReviewRunDispatchesThroughWrapper(t *testing.T) {
	var out bytes.Buffer
	svc := &mcpMainReviewCaptureService{}
	code := invokeWithDeps(
		context.Background(),
		logging.NewRecorder(),
		[]string{"--tool", "review"},
		strings.NewReader(`{"project_id":"my-cool-app","receipt_id":"receipt-1234","run":true,"tags_file":".acm/acm-tags.yaml"}`),
		&out,
		fixedMCPNow,
		func(_ context.Context, _ logging.Logger) (core.Service, runtime.CleanupFunc, error) {
			return svc, func() {}, nil
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
	if env.Command != v1.CommandReview {
		t.Fatalf("unexpected command: %q", env.Command)
	}
	if len(svc.reviewCalls) != 1 {
		t.Fatalf("expected one review call, got %d", len(svc.reviewCalls))
	}
	if got := svc.reviewCalls[0]; !got.Run || got.TagsFile != ".acm/acm-tags.yaml" || got.ReceiptID != "receipt-1234" {
		t.Fatalf("unexpected review payload: %+v", got)
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

func TestInvokeWithDeps_RemovedLegacyToolsWriteStructuredError(t *testing.T) {
	for _, tool := range []string{"get_context", "propose_memory", "report_completion", "bootstrap"} {
		t.Run(tool, func(t *testing.T) {
			var out bytes.Buffer
			code := invokeWithDeps(
				context.Background(),
				logging.NewRecorder(),
				[]string{"--tool", tool},
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
			if env.Tool != tool {
				t.Fatalf("unexpected tool: %q", env.Tool)
			}
			if env.Error == nil || env.Error.Code != "UNKNOWN_TOOL" {
				t.Fatalf("unexpected error payload: %+v", env.Error)
			}
		})
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
	if !strings.Contains(text, "history") {
		t.Fatalf("expected history in help output: %q", text)
	}
	if !strings.Contains(text, "review") {
		t.Fatalf("expected review in help output: %q", text)
	}
}

func TestUsage_IncludesVersionFlag(t *testing.T) {
	var out bytes.Buffer
	printUsage(&out)
	text := out.String()
	if !strings.Contains(text, "acm-mcp --version | -v") {
		t.Fatalf("expected version usage line, got %q", text)
	}
}

func TestWriteToolsJSON_MatchesRuntimeAndSpec(t *testing.T) {
	var out bytes.Buffer
	writeToolsJSON(&out)

	var got struct {
		Version string        `json:"version"`
		Tools   []mcp.ToolDef `json:"tools"`
	}
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal tools json: %v", err)
	}
	if got.Version != v1.Version {
		t.Fatalf("unexpected version: got %q want %q", got.Version, v1.Version)
	}
	if gotTools := got.Tools; !reflect.DeepEqual(gotTools, mcp.ToolDefinitions()) {
		t.Fatalf("tools output drift detected\noutput: %+v\nruntime: %+v", gotTools, mcp.ToolDefinitions())
	}

	specPath := filepath.Join("..", "..", "spec", "v1", "mcp.tools.v1.json")
	specRaw, err := os.ReadFile(specPath)
	if err != nil {
		t.Fatalf("read spec file: %v", err)
	}
	var spec struct {
		Version string        `json:"version"`
		Tools   []mcp.ToolDef `json:"tools"`
	}
	if err := json.Unmarshal(specRaw, &spec); err != nil {
		t.Fatalf("unmarshal spec file: %v", err)
	}
	if !reflect.DeepEqual(got.Tools, spec.Tools) {
		t.Fatalf("tools output drift detected against spec\noutput: %+v\nspec: %+v", got.Tools, spec.Tools)
	}
}

func TestPrintVersionWritesBinaryBanner(t *testing.T) {
	var out bytes.Buffer
	printVersion(&out, "acm-mcp")
	text := strings.TrimSpace(out.String())
	if !strings.HasPrefix(text, "acm-mcp ") {
		t.Fatalf("unexpected version output: %q", text)
	}
}

func fixedMCPNow() time.Time {
	return time.Date(2026, 3, 5, 12, 0, 0, 0, time.UTC)
}

type mcpMainFakeService struct{}

func (mcpMainFakeService) Context(_ context.Context, _ v1.ContextPayload) (v1.ContextResult, *core.APIError) {
	return v1.ContextResult{Status: "ok"}, nil
}

func (mcpMainFakeService) Fetch(_ context.Context, _ v1.FetchPayload) (v1.FetchResult, *core.APIError) {
	return v1.FetchResult{}, nil
}

func (mcpMainFakeService) Memory(_ context.Context, _ v1.MemoryCommandPayload) (v1.MemoryResult, *core.APIError) {
	return v1.MemoryResult{}, nil
}

func (mcpMainFakeService) Review(_ context.Context, _ v1.ReviewPayload) (v1.ReviewResult, *core.APIError) {
	return v1.ReviewResult{}, nil
}

func (mcpMainFakeService) Done(_ context.Context, _ v1.DonePayload) (v1.DoneResult, *core.APIError) {
	return v1.DoneResult{}, nil
}

func (mcpMainFakeService) Work(_ context.Context, _ v1.WorkPayload) (v1.WorkResult, *core.APIError) {
	return v1.WorkResult{}, nil
}

func (mcpMainFakeService) HistorySearch(_ context.Context, _ v1.HistorySearchPayload) (v1.HistorySearchResult, *core.APIError) {
	return v1.HistorySearchResult{}, nil
}

func (mcpMainFakeService) Sync(_ context.Context, _ v1.SyncPayload) (v1.SyncResult, *core.APIError) {
	return v1.SyncResult{}, nil
}

func (mcpMainFakeService) Health(_ context.Context, _ v1.HealthPayload) (v1.HealthResult, *core.APIError) {
	return v1.HealthResult{Mode: "check", Check: &v1.HealthCheckResult{}}, nil
}

func (mcpMainFakeService) Status(_ context.Context, _ v1.StatusPayload) (v1.StatusResult, *core.APIError) {
	return v1.StatusResult{}, nil
}

func (mcpMainFakeService) Verify(_ context.Context, _ v1.VerifyPayload) (v1.VerifyResult, *core.APIError) {
	return v1.VerifyResult{}, nil
}

func (mcpMainFakeService) Init(_ context.Context, _ v1.InitPayload) (v1.InitResult, *core.APIError) {
	return v1.InitResult{}, nil
}

type mcpMainFailingService struct {
	mcpMainFakeService
}

func (mcpMainFailingService) Context(_ context.Context, _ v1.ContextPayload) (v1.ContextResult, *core.APIError) {
	return v1.ContextResult{}, core.NewError("NOT_IMPLEMENTED", errors.New("boom").Error(), nil)
}

type mcpMainReviewCaptureService struct {
	mcpMainFakeService
	reviewCalls []v1.ReviewPayload
}

func (s *mcpMainReviewCaptureService) Review(_ context.Context, payload v1.ReviewPayload) (v1.ReviewResult, *core.APIError) {
	s.reviewCalls = append(s.reviewCalls, payload)
	return v1.ReviewResult{ReviewKey: payload.Key, Executed: payload.Run}, nil
}
