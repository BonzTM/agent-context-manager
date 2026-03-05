package mcp

import (
	"context"
	"testing"

	"github.com/joshd/agents-context/internal/contracts/v1"
	"github.com/joshd/agents-context/internal/core"
	"github.com/joshd/agents-context/internal/logging"
	"github.com/joshd/agents-context/internal/service/unconfigured"
)

type fakeService struct{}

func (f fakeService) GetContext(_ context.Context, _ v1.GetContextPayload) (v1.GetContextResult, *core.APIError) {
	return v1.GetContextResult{Status: "insufficient_context"}, nil
}

func (f fakeService) Fetch(_ context.Context, _ v1.FetchPayload) (v1.FetchResult, *core.APIError) {
	return v1.FetchResult{Items: []v1.FetchItem{}}, nil
}

func (f fakeService) ProposeMemory(_ context.Context, _ v1.ProposeMemoryPayload) (v1.ProposeMemoryResult, *core.APIError) {
	return v1.ProposeMemoryResult{CandidateID: 1, Status: "pending", Validation: v1.ProposeMemoryValidation{HardPassed: true, SoftPassed: true}}, nil
}

func (f fakeService) Work(_ context.Context, _ v1.WorkPayload) (v1.WorkResult, *core.APIError) {
	return v1.WorkResult{PlanKey: "plan.alpha", PlanStatus: "pending", Updated: 1}, nil
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

func (f fakeService) Regress(_ context.Context, _ v1.RegressPayload) (v1.RegressResult, *core.APIError) {
	return v1.RegressResult{}, nil
}

func (f fakeService) Bootstrap(_ context.Context, _ v1.BootstrapPayload) (v1.BootstrapResult, *core.APIError) {
	return v1.BootstrapResult{}, nil
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
	payload := []byte(`{"project_id":"soundspan","task_text":"x","phase":"execute"}`)
	result, err := Invoke(context.Background(), fakeService{}, "get_context", payload)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := result.(v1.GetContextResult); !ok {
		t.Fatalf("unexpected result type: %T", result)
	}
}

func TestInvoke_FetchAndWork(t *testing.T) {
	fetchResult, fetchErr := Invoke(context.Background(), fakeService{}, toolFetch, []byte(`{"project_id":"soundspan","keys":["docs/runtime.md"]}`))
	if fetchErr != nil {
		t.Fatalf("unexpected fetch error: %v", fetchErr)
	}
	if _, ok := fetchResult.(v1.FetchResult); !ok {
		t.Fatalf("unexpected fetch result type: %T", fetchResult)
	}

	workPayload := []byte(`{"project_id":"soundspan","plan_key":"plan.alpha","items":[{"key":"x.go","summary":"x","status":"pending"}]}`)
	workResult, workErr := Invoke(context.Background(), fakeService{}, toolWork, workPayload)
	if workErr != nil {
		t.Fatalf("unexpected work error: %v", workErr)
	}
	if _, ok := workResult.(v1.WorkResult); !ok {
		t.Fatalf("unexpected work result type: %T", workResult)
	}
}

func TestToolDefinitions_IncludeSchemaMetadata(t *testing.T) {
	defs := ToolDefinitions()
	if len(defs) != 5 {
		t.Fatalf("unexpected tool count: got %d want 5", len(defs))
	}

	expectedInputRefs := map[string]string{
		"get_context":       "https://agents-context.dev/spec/v1/cli.command.schema.json#/$defs/getContextPayload",
		"fetch":             "https://agents-context.dev/spec/v1/cli.command.schema.json#/$defs/fetchPayload",
		"propose_memory":    "https://agents-context.dev/spec/v1/cli.command.schema.json#/$defs/proposeMemoryPayload",
		"report_completion": "https://agents-context.dev/spec/v1/cli.command.schema.json#/$defs/reportCompletionPayload",
		"work":              "https://agents-context.dev/spec/v1/cli.command.schema.json#/$defs/workPayload",
	}
	expectedOutputRefs := map[string]string{
		"get_context":       "https://agents-context.dev/spec/v1/cli.result.schema.json#/$defs/getContextResult",
		"fetch":             "https://agents-context.dev/spec/v1/cli.result.schema.json#/$defs/fetchResult",
		"propose_memory":    "https://agents-context.dev/spec/v1/cli.result.schema.json#/$defs/proposeMemoryResult",
		"report_completion": "https://agents-context.dev/spec/v1/cli.result.schema.json#/$defs/reportCompletionResult",
		"work":              "https://agents-context.dev/spec/v1/cli.result.schema.json#/$defs/workResult",
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

func TestInvoke_UnconfiguredServiceReturnsNotImplemented(t *testing.T) {
	payload := []byte(`{"project_id":"soundspan","task_text":"x","phase":"execute"}`)
	_, err := Invoke(context.Background(), unconfigured.New(), "get_context", payload)
	if err == nil {
		t.Fatal("expected error")
	}
	if err.Code != "NOT_IMPLEMENTED" {
		t.Fatalf("unexpected code: %s", err.Code)
	}
}

func TestInvoke_HealthCheckRegressBootstrapRemainUnsupportedTools(t *testing.T) {
	tools := []string{"health_check", "health_fix", "coverage", "regress", "bootstrap"}
	for _, tool := range tools {
		t.Run(tool, func(t *testing.T) {
			_, err := Invoke(context.Background(), fakeService{}, tool, []byte(`{}`))
			if err == nil {
				t.Fatal("expected error")
			}
			if err.Code != "UNKNOWN_TOOL" {
				t.Fatalf("unexpected code: %s", err.Code)
			}
		})
	}
}

func TestInvokeWithLogger_LogsIngressDispatchAndResultOnSuccess(t *testing.T) {
	recorder := logging.NewRecorder()
	payload := []byte(`{"project_id":"soundspan","task_text":"x","phase":"execute"}`)

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
	payload := []byte(`{"project_id":"soundspan","task_text":"x","phase":"execute"}`)

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
