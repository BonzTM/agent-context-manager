package mcp

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/bonztm/agent-context-manager/internal/contracts/v1"
	"github.com/bonztm/agent-context-manager/internal/core"
	"github.com/bonztm/agent-context-manager/internal/logging"
	"github.com/bonztm/agent-context-manager/internal/runtime"
	"github.com/bonztm/agent-context-manager/internal/service/unconfigured"
)

type fakeService struct{}

type capturingService struct {
	fakeService
	coveragePayload v1.CoveragePayload
}

func (f fakeService) GetContext(_ context.Context, _ v1.GetContextPayload) (v1.GetContextResult, *core.APIError) {
	return v1.GetContextResult{Status: "insufficient_context"}, nil
}

func (f fakeService) Fetch(_ context.Context, _ v1.FetchPayload) (v1.FetchResult, *core.APIError) {
	return v1.FetchResult{Items: []v1.FetchItem{}}, nil
}

func (f fakeService) ProposeMemory(_ context.Context, _ v1.ProposeMemoryPayload) (v1.ProposeMemoryResult, *core.APIError) {
	return v1.ProposeMemoryResult{CandidateID: 1, Status: "pending", Validation: v1.ProposeMemoryValidation{HardPassed: true, SoftPassed: true}}, nil
}

func (f fakeService) Review(_ context.Context, _ v1.ReviewPayload) (v1.ReviewResult, *core.APIError) {
	return v1.ReviewResult{
		PlanKey:      "plan:receipt-1234",
		PlanStatus:   "pending",
		Updated:      1,
		ReviewKey:    v1.DefaultReviewTaskKey,
		ReviewStatus: v1.WorkItemStatusComplete,
	}, nil
}

func (f fakeService) Work(_ context.Context, _ v1.WorkPayload) (v1.WorkResult, *core.APIError) {
	return v1.WorkResult{PlanKey: "plan:receipt-1234", PlanStatus: "pending", Updated: 1}, nil
}

func (f fakeService) HistorySearch(_ context.Context, _ v1.HistorySearchPayload) (v1.HistorySearchResult, *core.APIError) {
	return v1.HistorySearchResult{}, nil
}

func (f fakeService) ReportCompletion(_ context.Context, _ v1.ReportCompletionPayload) (v1.ReportCompletionResult, *core.APIError) {
	return v1.ReportCompletionResult{Accepted: true}, nil
}

func (f fakeService) Sync(_ context.Context, _ v1.SyncPayload) (v1.SyncResult, *core.APIError) {
	return v1.SyncResult{}, nil
}

func (f fakeService) HealthCheck(_ context.Context, _ v1.HealthCheckPayload) (v1.HealthCheckResult, *core.APIError) {
	return v1.HealthCheckResult{}, nil
}

func (f fakeService) HealthFix(_ context.Context, _ v1.HealthFixPayload) (v1.HealthFixResult, *core.APIError) {
	return v1.HealthFixResult{}, nil
}

func (f fakeService) Coverage(_ context.Context, _ v1.CoveragePayload) (v1.CoverageResult, *core.APIError) {
	return v1.CoverageResult{}, nil
}

func (f fakeService) Eval(_ context.Context, _ v1.EvalPayload) (v1.EvalResult, *core.APIError) {
	return v1.EvalResult{}, nil
}

func (f fakeService) Verify(_ context.Context, _ v1.VerifyPayload) (v1.VerifyResult, *core.APIError) {
	return v1.VerifyResult{}, nil
}

func (f fakeService) Bootstrap(_ context.Context, _ v1.BootstrapPayload) (v1.BootstrapResult, *core.APIError) {
	return v1.BootstrapResult{}, nil
}

func (c *capturingService) Coverage(_ context.Context, payload v1.CoveragePayload) (v1.CoverageResult, *core.APIError) {
	c.coveragePayload = payload
	return v1.CoverageResult{}, nil
}

func TestInvoke_UnknownTool(t *testing.T) {
	_, err := Invoke(context.Background(), fakeService{}, "unknown", []byte(`{}`))
	if err == nil {
		t.Fatal("expected error")
	}
	if err.Code != "UNKNOWN_TOOL" {
		t.Fatalf("unexpected code: %s", err.Code)
	}
}

func TestInvoke_GetContext(t *testing.T) {
	payload := []byte(`{"project_id":"my-cool-app","task_text":"x","phase":"execute"}`)
	result, err := Invoke(context.Background(), fakeService{}, "get_context", payload)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := result.(v1.GetContextResult); !ok {
		t.Fatalf("unexpected result type: %T", result)
	}
}

func TestInvoke_DefaultsProjectIDFromEnv(t *testing.T) {
	t.Setenv(runtime.ProjectIDEnvVar, "env-project")

	result, err := Invoke(context.Background(), fakeService{}, "get_context", []byte(`{"task_text":"x","phase":"execute"}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := result.(v1.GetContextResult); !ok {
		t.Fatalf("unexpected result type: %T", result)
	}
}

func TestInvoke_ProjectRootInferenceOverridesEnvProjectID(t *testing.T) {
	t.Setenv(runtime.ProjectIDEnvVar, "env-project")
	projectRoot := filepath.Join(t.TempDir(), "Target Repo")
	svc := &capturingService{}

	result, err := Invoke(context.Background(), svc, toolCoverage, []byte(`{"project_root":"`+projectRoot+`"}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := result.(v1.CoverageResult); !ok {
		t.Fatalf("unexpected result type: %T", result)
	}
	if got, want := svc.coveragePayload.ProjectID, "Target-Repo"; got != want {
		t.Fatalf("unexpected inferred project_id: got %q want %q", got, want)
	}
}

func TestInvoke_RejectsUnknownInputFields(t *testing.T) {
	_, err := Invoke(context.Background(), fakeService{}, "get_context", []byte(`{"project_id":"my-cool-app","task_text":"x","phase":"execute","extra":true}`))
	if err == nil {
		t.Fatal("expected error")
	}
	if err.Code != "INVALID_TOOL_INPUT" {
		t.Fatalf("unexpected code: %s", err.Code)
	}
}

func TestInvoke_FetchAndWork(t *testing.T) {
	fetchResult, fetchErr := Invoke(context.Background(), fakeService{}, toolFetch, []byte(`{"project_id":"my-cool-app","keys":["docs/runtime.md"]}`))
	if fetchErr != nil {
		t.Fatalf("unexpected fetch error: %v", fetchErr)
	}
	if _, ok := fetchResult.(v1.FetchResult); !ok {
		t.Fatalf("unexpected fetch result type: %T", fetchResult)
	}

	workPayload := []byte(`{"project_id":"my-cool-app","plan_key":"plan:receipt-1234","tasks":[{"key":"x.go","summary":"x","status":"pending"}]}`)
	workResult, workErr := Invoke(context.Background(), fakeService{}, toolWork, workPayload)
	if workErr != nil {
		t.Fatalf("unexpected work error: %v", workErr)
	}
	if _, ok := workResult.(v1.WorkResult); !ok {
		t.Fatalf("unexpected work result type: %T", workResult)
	}

	reviewPayload := []byte(`{"project_id":"my-cool-app","receipt_id":"receipt-1234","outcome":"No blocking review findings."}`)
	reviewResult, reviewErr := Invoke(context.Background(), fakeService{}, toolReview, reviewPayload)
	if reviewErr != nil {
		t.Fatalf("unexpected review error: %v", reviewErr)
	}
	if _, ok := reviewResult.(v1.ReviewResult); !ok {
		t.Fatalf("unexpected review result type: %T", reviewResult)
	}
}

func TestInvoke_RemainingTools(t *testing.T) {
	tests := []struct {
		name    string
		tool    string
		payload []byte
		assert  func(t *testing.T, result any)
	}{
		{
			name:    "history_search",
			tool:    toolHistorySearch,
			payload: []byte(`{"project_id":"my-cool-app","entity":"memory","query":"bootstrap"}`),
			assert: func(t *testing.T, result any) {
				t.Helper()
				if _, ok := result.(v1.HistorySearchResult); !ok {
					t.Fatalf("unexpected history_search result type: %T", result)
				}
			},
		},
		{
			name:    "sync",
			tool:    toolSync,
			payload: []byte(`{"project_id":"my-cool-app"}`),
			assert: func(t *testing.T, result any) {
				t.Helper()
				if _, ok := result.(v1.SyncResult); !ok {
					t.Fatalf("unexpected sync result type: %T", result)
				}
			},
		},
		{
			name:    "health_check",
			tool:    toolHealthCheck,
			payload: []byte(`{"project_id":"my-cool-app"}`),
			assert: func(t *testing.T, result any) {
				t.Helper()
				if _, ok := result.(v1.HealthCheckResult); !ok {
					t.Fatalf("unexpected health_check result type: %T", result)
				}
			},
		},
		{
			name:    "health_fix",
			tool:    toolHealthFix,
			payload: []byte(`{"project_id":"my-cool-app"}`),
			assert: func(t *testing.T, result any) {
				t.Helper()
				if _, ok := result.(v1.HealthFixResult); !ok {
					t.Fatalf("unexpected health_fix result type: %T", result)
				}
			},
		},
		{
			name:    "coverage",
			tool:    toolCoverage,
			payload: []byte(`{"project_id":"my-cool-app"}`),
			assert: func(t *testing.T, result any) {
				t.Helper()
				if _, ok := result.(v1.CoverageResult); !ok {
					t.Fatalf("unexpected coverage result type: %T", result)
				}
			},
		},
		{
			name:    "eval",
			tool:    toolEval,
			payload: []byte(`{"project_id":"my-cool-app","eval_suite_inline":[{"task_text":"Check sync","phase":"execute"}]}`),
			assert: func(t *testing.T, result any) {
				t.Helper()
				if _, ok := result.(v1.EvalResult); !ok {
					t.Fatalf("unexpected eval result type: %T", result)
				}
			},
		},
		{
			name:    "verify",
			tool:    toolVerify,
			payload: []byte(`{"project_id":"my-cool-app","phase":"execute","files_changed":["go.mod"]}`),
			assert: func(t *testing.T, result any) {
				t.Helper()
				if _, ok := result.(v1.VerifyResult); !ok {
					t.Fatalf("unexpected verify result type: %T", result)
				}
			},
		},
		{
			name:    "bootstrap",
			tool:    toolBootstrap,
			payload: []byte(`{"project_id":"my-cool-app","project_root":"."}`),
			assert: func(t *testing.T, result any) {
				t.Helper()
				if _, ok := result.(v1.BootstrapResult); !ok {
					t.Fatalf("unexpected bootstrap result type: %T", result)
				}
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result, err := Invoke(context.Background(), fakeService{}, tc.tool, tc.payload)
			if err != nil {
				t.Fatalf("unexpected %s error: %v", tc.tool, err)
			}
			tc.assert(t, result)
		})
	}
}

func TestToolDefinitions_IncludeSchemaMetadata(t *testing.T) {
	defs := ToolDefinitions()
	if len(defs) != 14 {
		t.Fatalf("unexpected tool count: got %d want 14", len(defs))
	}

	expectedInputRefs := map[string]string{
		"get_context":       "https://agent-context-manager.dev/spec/v1/cli.command.schema.json#/$defs/getContextPayload",
		"fetch":             "https://agent-context-manager.dev/spec/v1/cli.command.schema.json#/$defs/fetchPayload",
		"propose_memory":    "https://agent-context-manager.dev/spec/v1/cli.command.schema.json#/$defs/proposeMemoryPayload",
		"report_completion": "https://agent-context-manager.dev/spec/v1/cli.command.schema.json#/$defs/reportCompletionPayload",
		"review":            "https://agent-context-manager.dev/spec/v1/cli.command.schema.json#/$defs/reviewPayload",
		"work":              "https://agent-context-manager.dev/spec/v1/cli.command.schema.json#/$defs/workPayload",
		"history_search":    "https://agent-context-manager.dev/spec/v1/cli.command.schema.json#/$defs/historySearchPayload",
		"sync":              "https://agent-context-manager.dev/spec/v1/cli.command.schema.json#/$defs/syncPayload",
		"health_check":      "https://agent-context-manager.dev/spec/v1/cli.command.schema.json#/$defs/healthCheckPayload",
		"health_fix":        "https://agent-context-manager.dev/spec/v1/cli.command.schema.json#/$defs/healthFixPayload",
		"coverage":          "https://agent-context-manager.dev/spec/v1/cli.command.schema.json#/$defs/coveragePayload",
		"eval":              "https://agent-context-manager.dev/spec/v1/cli.command.schema.json#/$defs/evalPayload",
		"verify":            "https://agent-context-manager.dev/spec/v1/cli.command.schema.json#/$defs/verifyPayload",
		"bootstrap":         "https://agent-context-manager.dev/spec/v1/cli.command.schema.json#/$defs/bootstrapPayload",
	}
	expectedOutputRefs := map[string]string{
		"get_context":       "https://agent-context-manager.dev/spec/v1/cli.result.schema.json#/$defs/getContextResult",
		"fetch":             "https://agent-context-manager.dev/spec/v1/cli.result.schema.json#/$defs/fetchResult",
		"propose_memory":    "https://agent-context-manager.dev/spec/v1/cli.result.schema.json#/$defs/proposeMemoryResult",
		"report_completion": "https://agent-context-manager.dev/spec/v1/cli.result.schema.json#/$defs/reportCompletionResult",
		"review":            "https://agent-context-manager.dev/spec/v1/cli.result.schema.json#/$defs/reviewResult",
		"work":              "https://agent-context-manager.dev/spec/v1/cli.result.schema.json#/$defs/workResult",
		"history_search":    "https://agent-context-manager.dev/spec/v1/cli.result.schema.json#/$defs/historySearchResult",
		"sync":              "https://agent-context-manager.dev/spec/v1/cli.result.schema.json#/$defs/syncResult",
		"health_check":      "https://agent-context-manager.dev/spec/v1/cli.result.schema.json#/$defs/healthCheckResult",
		"health_fix":        "https://agent-context-manager.dev/spec/v1/cli.result.schema.json#/$defs/healthFixResult",
		"coverage":          "https://agent-context-manager.dev/spec/v1/cli.result.schema.json#/$defs/coverageResult",
		"eval":              "https://agent-context-manager.dev/spec/v1/cli.result.schema.json#/$defs/evalResult",
		"verify":            "https://agent-context-manager.dev/spec/v1/cli.result.schema.json#/$defs/verifyResult",
		"bootstrap":         "https://agent-context-manager.dev/spec/v1/cli.result.schema.json#/$defs/bootstrapResult",
	}

	for _, def := range defs {
		if got := def.InputSchema["$schema"]; got != schemaDraft202012 {
			t.Fatalf("tool %q missing input schema draft metadata: %v", def.Name, got)
		}
		if got := def.OutputSchema["$schema"]; got != schemaDraft202012 {
			t.Fatalf("tool %q missing output schema draft metadata: %v", def.Name, got)
		}
		if got := def.InputSchema["$ref"]; got != expectedInputRefs[def.Name] {
			t.Fatalf("tool %q unexpected input schema ref: %v", def.Name, got)
		}
		if got := def.OutputSchema["$ref"]; got != expectedOutputRefs[def.Name] {
			t.Fatalf("tool %q unexpected output schema ref: %v", def.Name, got)
		}
	}
}

func TestToolDefinitions_MatchSpecFile(t *testing.T) {
	specPath := filepath.Join("..", "..", "..", "spec", "v1", "mcp.tools.v1.json")
	data, err := os.ReadFile(specPath)
	if err != nil {
		t.Fatalf("read spec file: %v", err)
	}

	var spec struct {
		Version string    `json:"version"`
		Tools   []ToolDef `json:"tools"`
	}
	if err := json.Unmarshal(data, &spec); err != nil {
		t.Fatalf("unmarshal spec file: %v", err)
	}
	if spec.Version != v1.Version {
		t.Fatalf("unexpected spec version: got %q want %q", spec.Version, v1.Version)
	}

	if got, want := ToolDefinitions(), spec.Tools; !reflect.DeepEqual(got, want) {
		t.Fatalf("tool definition drift detected\nruntime: %+v\nspec: %+v", got, want)
	}
}

func TestInvoke_UnconfiguredServiceReturnsNotImplemented(t *testing.T) {
	payload := []byte(`{"project_id":"my-cool-app","task_text":"x","phase":"execute"}`)
	_, err := Invoke(context.Background(), unconfigured.New(), "get_context", payload)
	if err == nil {
		t.Fatal("expected error")
	}
	if err.Code != "NOT_IMPLEMENTED" {
		t.Fatalf("unexpected code: %s", err.Code)
	}
}

func TestInvokeWithLogger_LogsIngressDispatchAndResultOnSuccess(t *testing.T) {
	recorder := logging.NewRecorder()
	payload := []byte(`{"project_id":"my-cool-app","task_text":"x","phase":"execute"}`)

	result, err := InvokeWithLogger(context.Background(), fakeService{}, "get_context", payload, recorder)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := result.(v1.GetContextResult); !ok {
		t.Fatalf("unexpected result type: %T", result)
	}

	entries := recorder.Entries()
	if len(entries) != 5 {
		t.Fatalf("expected 5 log entries, got %d", len(entries))
	}
	if entries[0].Event != logging.EventMCPIngressRead || entries[0].Fields["ok"] != true {
		t.Fatalf("unexpected ingress read entry: %+v", entries[0])
	}
	if entries[1].Event != logging.EventMCPIngressValidate || entries[1].Fields["ok"] != true {
		t.Fatalf("unexpected validate entry: %+v", entries[1])
	}
	if entries[2].Event != logging.EventMCPDispatch || entries[2].Fields["phase"] != "start" {
		t.Fatalf("unexpected dispatch start entry: %+v", entries[2])
	}
	if entries[3].Event != logging.EventMCPDispatch || entries[3].Fields["phase"] != "finish" || entries[3].Fields["ok"] != true {
		t.Fatalf("unexpected dispatch finish entry: %+v", entries[3])
	}
	if entries[4].Event != logging.EventMCPResult || entries[4].Fields["ok"] != true {
		t.Fatalf("unexpected result entry: %+v", entries[4])
	}
}

func TestInvokeWithLogger_LogsValidationFailure(t *testing.T) {
	recorder := logging.NewRecorder()
	_, err := InvokeWithLogger(context.Background(), fakeService{}, "get_context", []byte(`{`), recorder)
	if err == nil {
		t.Fatal("expected error")
	}
	if err.Code != "INVALID_TOOL_INPUT" {
		t.Fatalf("unexpected error code: %s", err.Code)
	}

	entries := recorder.Entries()
	if len(entries) != 4 {
		t.Fatalf("expected 4 log entries, got %d", len(entries))
	}
	if entries[1].Event != logging.EventMCPIngressValidate || entries[1].Fields["ok"] != false {
		t.Fatalf("unexpected validate failure entry: %+v", entries[1])
	}
	if entries[2].Event != logging.EventMCPFailure || entries[2].Fields["stage"] != "validate" {
		t.Fatalf("unexpected failure entry: %+v", entries[2])
	}
	if entries[3].Event != logging.EventMCPResult || entries[3].Fields["ok"] != false {
		t.Fatalf("unexpected result entry: %+v", entries[3])
	}
}

func TestInvokeWithLogger_LogsUnknownToolFailure(t *testing.T) {
	recorder := logging.NewRecorder()
	_, err := InvokeWithLogger(context.Background(), fakeService{}, "unknown", []byte(`{}`), recorder)
	if err == nil {
		t.Fatal("expected error")
	}
	if err.Code != "UNKNOWN_TOOL" {
		t.Fatalf("unexpected error code: %s", err.Code)
	}

	entries := recorder.Entries()
	if len(entries) != 4 {
		t.Fatalf("expected 4 log entries, got %d", len(entries))
	}
	if entries[1].Event != logging.EventMCPIngressValidate || entries[1].Fields["ok"] != false {
		t.Fatalf("unexpected validate entry: %+v", entries[1])
	}
	if entries[2].Event != logging.EventMCPFailure || entries[2].Fields["stage"] != "validate" {
		t.Fatalf("unexpected failure entry: %+v", entries[2])
	}
	if entries[3].Event != logging.EventMCPResult || entries[3].Fields["ok"] != false {
		t.Fatalf("unexpected result entry: %+v", entries[3])
	}
}

func TestInvokeWithLogger_LogsDispatchFailure(t *testing.T) {
	recorder := logging.NewRecorder()
	payload := []byte(`{"project_id":"my-cool-app","task_text":"x","phase":"execute"}`)

	_, err := InvokeWithLogger(context.Background(), unconfigured.New(), "get_context", payload, recorder)
	if err == nil {
		t.Fatal("expected error")
	}
	if err.Code != "NOT_IMPLEMENTED" {
		t.Fatalf("unexpected error code: %s", err.Code)
	}

	entries := recorder.Entries()
	if len(entries) != 6 {
		t.Fatalf("expected 6 log entries, got %d", len(entries))
	}
	if entries[3].Event != logging.EventMCPDispatch || entries[3].Fields["ok"] != false {
		t.Fatalf("unexpected dispatch failure entry: %+v", entries[3])
	}
	if entries[4].Event != logging.EventMCPFailure || entries[4].Fields["stage"] != "dispatch" {
		t.Fatalf("unexpected failure entry: %+v", entries[4])
	}
	if entries[5].Event != logging.EventMCPResult || entries[5].Fields["ok"] != false {
		t.Fatalf("unexpected result entry: %+v", entries[5])
	}
}
