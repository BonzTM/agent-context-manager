package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/joshd/agent-context-manager/internal/contracts/v1"
	"github.com/joshd/agent-context-manager/internal/core"
	"github.com/joshd/agent-context-manager/internal/logging"
	"github.com/joshd/agent-context-manager/internal/service/unconfigured"
)

type fakeService struct{}

func (f fakeService) GetContext(_ context.Context, _ v1.GetContextPayload) (v1.GetContextResult, *core.APIError) {
	return v1.GetContextResult{Status: "insufficient_context"}, nil
}

func (f fakeService) Fetch(_ context.Context, _ v1.FetchPayload) (v1.FetchResult, *core.APIError) {
	return v1.FetchResult{Items: []v1.FetchItem{}}, nil
}

func (f fakeService) ProposeMemory(_ context.Context, _ v1.ProposeMemoryPayload) (v1.ProposeMemoryResult, *core.APIError) {
	return v1.ProposeMemoryResult{}, nil
}

func (f fakeService) Work(_ context.Context, _ v1.WorkPayload) (v1.WorkResult, *core.APIError) {
	return v1.WorkResult{PlanKey: "plan.alpha", PlanStatus: "pending", Updated: 1}, nil
}

func (f fakeService) ReportCompletion(_ context.Context, _ v1.ReportCompletionPayload) (v1.ReportCompletionResult, *core.APIError) {
	return v1.ReportCompletionResult{}, nil
}

func (f fakeService) Sync(_ context.Context, _ v1.SyncPayload) (v1.SyncResult, *core.APIError) {
	return v1.SyncResult{}, nil
}

func (f fakeService) HealthCheck(_ context.Context, _ v1.HealthCheckPayload) (v1.HealthCheckResult, *core.APIError) {
	return v1.HealthCheckResult{}, nil
}

func (f fakeService) HealthFix(_ context.Context, _ v1.HealthFixPayload) (v1.HealthFixResult, *core.APIError) {
	return v1.HealthFixResult{DryRun: true, PlannedActions: []v1.HealthFixAction{}, AppliedActions: []v1.HealthFixAction{}, Summary: "ok"}, nil
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

func TestRun_SuccessEnvelope(t *testing.T) {
	in := bytes.NewBufferString(`{
		"version":"ctx.v1",
		"command":"get_context",
		"request_id":"req-12345",
		"payload":{
			"project_id":"my-cool-app",
			"task_text":"x",
			"phase":"execute"
		}
	}`)
	out := &bytes.Buffer{}
	code := Run(context.Background(), fakeService{}, in, out, func() time.Time { return time.Date(2026, 3, 4, 10, 0, 0, 0, time.UTC) })
	if code != 0 {
		t.Fatalf("expected exit code 0 got %d", code)
	}

	var env v1.ResultEnvelope
	if err := json.Unmarshal(out.Bytes(), &env); err != nil {
		t.Fatalf("failed to parse output: %v", err)
	}
	if !env.OK {
		t.Fatalf("expected ok=true, got false: %+v", env.Error)
	}
	if env.Command != v1.CommandGetContext {
		t.Fatalf("unexpected command: %s", env.Command)
	}
}

func TestRun_ValidationFailure(t *testing.T) {
	in := bytes.NewBufferString(`{"version":"ctx.v1","command":"get_context","request_id":"bad","payload":{}}`)
	out := &bytes.Buffer{}
	code := Run(context.Background(), fakeService{}, in, out, time.Now)
	if code == 0 {
		t.Fatalf("expected nonzero code")
	}

	var env v1.ResultEnvelope
	if err := json.Unmarshal(out.Bytes(), &env); err != nil {
		t.Fatalf("failed to parse output: %v", err)
	}
	if env.OK {
		t.Fatalf("expected ok=false")
	}
	if env.Error == nil {
		t.Fatalf("expected error payload")
	}
}

func TestRun_UnconfiguredServiceNotImplementedEnvelope(t *testing.T) {
	in := bytes.NewBufferString(`{
		"version":"ctx.v1",
		"command":"get_context",
		"request_id":"req-12345",
		"payload":{
			"project_id":"my-cool-app",
			"task_text":"x",
			"phase":"execute"
		}
	}`)
	out := &bytes.Buffer{}
	code := Run(context.Background(), unconfigured.New(), in, out, func() time.Time {
		return time.Date(2026, 3, 4, 10, 0, 0, 0, time.UTC)
	})
	if code != 1 {
		t.Fatalf("expected exit code 1 got %d", code)
	}

	var env v1.ResultEnvelope
	if err := json.Unmarshal(out.Bytes(), &env); err != nil {
		t.Fatalf("failed to parse output: %v", err)
	}
	if env.OK {
		t.Fatalf("expected ok=false")
	}
	if env.Error == nil {
		t.Fatalf("expected error payload")
	}
	if env.Error.Code != "NOT_IMPLEMENTED" {
		t.Fatalf("unexpected error code: %s", env.Error.Code)
	}
}

func TestRun_DispatchesHealthCheckRegressAndBootstrap(t *testing.T) {
	tests := []struct {
		name    string
		command string
		payload string
	}{
		{
			name:    "health_check",
			command: "health_check",
			payload: `{"project_id":"my-cool-app"}`,
		},
		{
			name:    "health_fix",
			command: "health_fix",
			payload: `{"project_id":"my-cool-app"}`,
		},
		{
			name:    "coverage",
			command: "coverage",
			payload: `{"project_id":"my-cool-app"}`,
		},
		{
			name:    "regress",
			command: "regress",
			payload: `{"project_id":"my-cool-app","eval_suite_inline":[{"task_text":"x","phase":"execute"}]}`,
		},
		{
			name:    "bootstrap",
			command: "bootstrap",
			payload: `{"project_id":"my-cool-app","project_root":"."}`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			in := bytes.NewBufferString(`{
				"version":"ctx.v1",
				"command":"` + tc.command + `",
				"request_id":"req-12345",
				"payload":` + tc.payload + `
			}`)
			out := &bytes.Buffer{}
			code := Run(context.Background(), fakeService{}, in, out, func() time.Time {
				return time.Date(2026, 3, 4, 10, 0, 0, 0, time.UTC)
			})
			if code != 0 {
				t.Fatalf("expected exit code 0 got %d", code)
			}

			var env v1.ResultEnvelope
			if err := json.Unmarshal(out.Bytes(), &env); err != nil {
				t.Fatalf("failed to parse output: %v", err)
			}
			if !env.OK {
				t.Fatalf("expected ok=true, got false: %+v", env.Error)
			}
			if env.Command != v1.Command(tc.command) {
				t.Fatalf("unexpected command: got %q want %q", env.Command, tc.command)
			}
		})
	}
}

func TestDispatch_RoutesFetchAndWork(t *testing.T) {
	tests := []struct {
		name    string
		command v1.Command
		payload any
	}{
		{
			name:    "fetch",
			command: commandFetch,
			payload: v1.FetchPayload{ProjectID: "my-cool-app", Keys: []string{"docs/runtime.md"}},
		},
		{
			name:    "work",
			command: commandWork,
			payload: v1.WorkPayload{ProjectID: "my-cool-app", PlanKey: "plan.alpha", Items: []v1.WorkItemPayload{{Key: "x.go", Summary: "x", Status: v1.WorkItemStatusPending}}},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result, apiErr := dispatch(context.Background(), fakeService{}, tc.command, tc.payload)
			if apiErr != nil {
				t.Fatalf("unexpected API error: %+v", apiErr)
			}
			switch tc.command {
			case commandFetch:
				if _, ok := result.(v1.FetchResult); !ok {
					t.Fatalf("unexpected fetch result type: %T", result)
				}
			case commandWork:
				if _, ok := result.(v1.WorkResult); !ok {
					t.Fatalf("unexpected work result type: %T", result)
				}
			}
		})
	}
}

func TestProjectIDFromPayload_ExtractsFromMapAndStruct(t *testing.T) {
	if got := projectIDFromPayload(map[string]any{"project_id": "  my-cool-app  "}); got != "my-cool-app" {
		t.Fatalf("unexpected map project id: %q", got)
	}
	if got := projectIDFromPayload(struct{ ProjectID string }{ProjectID: "  my-cool-app  "}); got != "my-cool-app" {
		t.Fatalf("unexpected struct project id: %q", got)
	}
	if got := projectIDFromPayload(&struct{ ProjectID string }{ProjectID: "  my-cool-app  "}); got != "my-cool-app" {
		t.Fatalf("unexpected pointer struct project id: %q", got)
	}
}

func TestRunWithLogger_LogsIngressDispatchAndResultOnSuccess(t *testing.T) {
	in := bytes.NewBufferString(`{
		"version":"ctx.v1",
		"command":"get_context",
		"request_id":"req-12345",
		"payload":{
			"project_id":"my-cool-app",
			"task_text":"x",
			"phase":"execute"
		}
	}`)
	out := &bytes.Buffer{}
	recorder := logging.NewRecorder()

	code := RunWithLogger(context.Background(), fakeService{}, in, out, func() time.Time {
		return time.Date(2026, 3, 4, 10, 0, 0, 0, time.UTC)
	}, recorder)
	if code != 0 {
		t.Fatalf("expected exit code 0 got %d", code)
	}

	entries := recorder.Entries()
	if len(entries) != 5 {
		t.Fatalf("expected 5 log entries, got %d", len(entries))
	}
	if entries[0].Event != logging.EventCLIIngressRead {
		t.Fatalf("unexpected event[0]: %s", entries[0].Event)
	}
	if entries[1].Event != logging.EventCLIIngressValidate {
		t.Fatalf("unexpected event[1]: %s", entries[1].Event)
	}
	if entries[2].Event != logging.EventCLIDispatch || entries[2].Fields["phase"] != "start" {
		t.Fatalf("unexpected dispatch start entry: %+v", entries[2])
	}
	if entries[3].Event != logging.EventCLIDispatch || entries[3].Fields["phase"] != "finish" || entries[3].Fields["ok"] != true {
		t.Fatalf("unexpected dispatch finish entry: %+v", entries[3])
	}
	if entries[4].Event != logging.EventCLIResult || entries[4].Fields["ok"] != true {
		t.Fatalf("unexpected result entry: %+v", entries[4])
	}
	for _, idx := range []int{1, 2, 3, 4} {
		if got := entries[idx].Fields["project_id"]; got != "my-cool-app" {
			t.Fatalf("entry[%d] missing project_id: %+v", idx, entries[idx])
		}
	}
}

func TestRunWithLogger_LogsReadFailure(t *testing.T) {
	out := &bytes.Buffer{}
	recorder := logging.NewRecorder()

	code := RunWithLogger(context.Background(), fakeService{}, errorReader{err: errors.New("boom")}, out, time.Now, recorder)
	if code == 0 {
		t.Fatal("expected nonzero code")
	}

	entries := recorder.Entries()
	if len(entries) != 3 {
		t.Fatalf("expected 3 log entries, got %d", len(entries))
	}
	if entries[0].Event != logging.EventCLIIngressRead || entries[0].Fields["ok"] != false {
		t.Fatalf("unexpected ingress read failure entry: %+v", entries[0])
	}
	if entries[1].Event != logging.EventCLIFailure || entries[1].Fields["stage"] != "read" {
		t.Fatalf("unexpected failure entry: %+v", entries[1])
	}
	if entries[2].Event != logging.EventCLIResult || entries[2].Fields["ok"] != false {
		t.Fatalf("unexpected result entry: %+v", entries[2])
	}
}

func TestRunWithLogger_LogsValidationFailure(t *testing.T) {
	in := bytes.NewBufferString(`{"version":"ctx.v1","command":"get_context","request_id":"bad","payload":{}}`)
	out := &bytes.Buffer{}
	recorder := logging.NewRecorder()

	code := RunWithLogger(context.Background(), fakeService{}, in, out, time.Now, recorder)
	if code == 0 {
		t.Fatal("expected nonzero code")
	}

	entries := recorder.Entries()
	if len(entries) != 4 {
		t.Fatalf("expected 4 log entries, got %d", len(entries))
	}
	if entries[0].Event != logging.EventCLIIngressRead || entries[0].Fields["ok"] != true {
		t.Fatalf("unexpected read entry: %+v", entries[0])
	}
	if entries[1].Event != logging.EventCLIIngressValidate || entries[1].Fields["ok"] != false {
		t.Fatalf("unexpected validate entry: %+v", entries[1])
	}
	if entries[2].Event != logging.EventCLIFailure || entries[2].Fields["stage"] != "validate" {
		t.Fatalf("unexpected failure entry: %+v", entries[2])
	}
	if entries[3].Event != logging.EventCLIResult || entries[3].Fields["ok"] != false {
		t.Fatalf("unexpected result entry: %+v", entries[3])
	}
	if got := entries[1].Fields["error_code"]; got != "INVALID_REQUEST_ID" {
		t.Fatalf("unexpected validation error code: %v", got)
	}
}

func TestRunWithLogger_LogsDispatchFailure(t *testing.T) {
	in := bytes.NewBufferString(`{
		"version":"ctx.v1",
		"command":"get_context",
		"request_id":"req-12345",
		"payload":{
			"project_id":"my-cool-app",
			"task_text":"x",
			"phase":"execute"
		}
	}`)
	out := &bytes.Buffer{}
	recorder := logging.NewRecorder()

	code := RunWithLogger(context.Background(), unconfigured.New(), in, out, time.Now, recorder)
	if code == 0 {
		t.Fatal("expected nonzero code")
	}

	entries := recorder.Entries()
	if len(entries) != 6 {
		t.Fatalf("expected 6 log entries, got %d", len(entries))
	}
	if entries[3].Event != logging.EventCLIDispatch || entries[3].Fields["ok"] != false {
		t.Fatalf("unexpected dispatch failure entry: %+v", entries[3])
	}
	if entries[4].Event != logging.EventCLIFailure || entries[4].Fields["stage"] != "dispatch" {
		t.Fatalf("unexpected failure entry: %+v", entries[4])
	}
	if got := entries[4].Fields["error_code"]; got != "NOT_IMPLEMENTED" {
		t.Fatalf("unexpected dispatch error code: %v", got)
	}
	if entries[5].Event != logging.EventCLIResult || entries[5].Fields["ok"] != false {
		t.Fatalf("unexpected result entry: %+v", entries[5])
	}
}

type errorReader struct {
	err error
}

func (r errorReader) Read(_ []byte) (int, error) {
	return 0, r.err
}
