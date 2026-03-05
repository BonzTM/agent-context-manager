package main

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/joshd/agent-context-manager/internal/adapters/cli"
	"github.com/joshd/agent-context-manager/internal/contracts/v1"
	"github.com/joshd/agent-context-manager/internal/core"
	"github.com/joshd/agent-context-manager/internal/logging"
	"github.com/joshd/agent-context-manager/internal/runtime"
)

func TestParseExpectedVersion(t *testing.T) {
	key, version, err := parseExpectedVersion("plan:req-12345678=v4")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if key != "plan:req-12345678" {
		t.Fatalf("unexpected key: %q", key)
	}
	if version != "v4" {
		t.Fatalf("unexpected version: %q", version)
	}

	if _, _, err := parseExpectedVersion("missing-separator"); err == nil {
		t.Fatal("expected error for invalid value")
	}
}

func TestBuildFetchEnvelope_ParsesRepeatedFlags(t *testing.T) {
	env, err := buildConvenienceEnvelope("fetch", []string{
		"--project", "myproject",
		"--request", "req-12345678",
		"--key", "plan:req-12345678",
		"--key", "rule:myproject/rule-1",
		"--expect", "plan:req-12345678=v4",
		"--expect", "rule:myproject/rule-1=v2",
	}, fixedNow)
	if err != nil {
		t.Fatalf("buildConvenienceEnvelope returned error: %v", err)
	}
	if env.Command != v1.CommandFetch {
		t.Fatalf("unexpected command: %s", env.Command)
	}
	if env.RequestID != "req-12345678" {
		t.Fatalf("unexpected request id: %s", env.RequestID)
	}

	var payload v1.FetchPayload
	if err := json.Unmarshal(env.Payload, &payload); err != nil {
		t.Fatalf("failed to decode payload: %v", err)
	}
	if payload.ProjectID != "myproject" {
		t.Fatalf("unexpected project id: %s", payload.ProjectID)
	}
	if len(payload.Keys) != 2 {
		t.Fatalf("expected 2 keys, got %d", len(payload.Keys))
	}
	if payload.ExpectedVersions["plan:req-12345678"] != "v4" {
		t.Fatalf("unexpected expected version for plan key: %q", payload.ExpectedVersions["plan:req-12345678"])
	}
	if payload.ExpectedVersions["rule:myproject/rule-1"] != "v2" {
		t.Fatalf("unexpected expected version for rule key: %q", payload.ExpectedVersions["rule:myproject/rule-1"])
	}
}

func TestBuildFetchEnvelope_LoadsKeysFile(t *testing.T) {
	keysPath := filepath.Join(t.TempDir(), "keys.json")
	if err := os.WriteFile(keysPath, []byte(`["plan:req-12345678","rule:myproject/rule-2"]`), 0o644); err != nil {
		t.Fatalf("write keys fixture: %v", err)
	}

	env, err := buildConvenienceEnvelope("fetch", []string{
		"--project", "myproject",
		"--keys-file", keysPath,
		"--receipt-id", "req-87654321",
	}, fixedNow)
	if err != nil {
		t.Fatalf("buildConvenienceEnvelope returned error: %v", err)
	}

	var payload v1.FetchPayload
	if err := json.Unmarshal(env.Payload, &payload); err != nil {
		t.Fatalf("failed to decode payload: %v", err)
	}
	if payload.ReceiptID != "req-87654321" {
		t.Fatalf("unexpected receipt id: %s", payload.ReceiptID)
	}
	if len(payload.Keys) != 2 {
		t.Fatalf("expected 2 keys from file, got %d", len(payload.Keys))
	}
}

func TestBuildWorkEnvelope_LoadsItemsFile(t *testing.T) {
	itemsPath := filepath.Join(t.TempDir(), "items.json")
	if err := os.WriteFile(itemsPath, []byte(`[
		{"key":"verify:tests","summary":"Run tests","status":"pending"},
		{"key":"verify:diff-review","summary":"Review diff","status":"complete","outcome":"No regressions"}
	]`), 0o644); err != nil {
		t.Fatalf("failed to write test fixture: %v", err)
	}

	env, err := buildConvenienceEnvelope("work", []string{
		"--project", "myproject",
		"--request", "req-12345678",
		"--receipt-id", "req-87654321",
		"--items-file", itemsPath,
	}, fixedNow)
	if err != nil {
		t.Fatalf("buildConvenienceEnvelope returned error: %v", err)
	}
	if env.Command != v1.CommandWork {
		t.Fatalf("unexpected command: %s", env.Command)
	}

	var payload v1.WorkPayload
	if err := json.Unmarshal(env.Payload, &payload); err != nil {
		t.Fatalf("failed to decode payload: %v", err)
	}
	if payload.ReceiptID != "req-87654321" {
		t.Fatalf("unexpected receipt id: %s", payload.ReceiptID)
	}
	if len(payload.Items) != 2 {
		t.Fatalf("expected 2 work items, got %d", len(payload.Items))
	}
	if payload.Items[0].Key != "verify:tests" {
		t.Fatalf("unexpected first item key: %q", payload.Items[0].Key)
	}
}

func TestBuildProposeMemoryEnvelope_ContentFile(t *testing.T) {
	contentPath := filepath.Join(t.TempDir(), "memory-content.txt")
	if err := os.WriteFile(contentPath, []byte("Prefer shared logger wrappers"), 0o644); err != nil {
		t.Fatalf("write content fixture: %v", err)
	}

	env, err := buildConvenienceEnvelope("propose-memory", []string{
		"--project", "myproject",
		"--request", "req-12345678",
		"--receipt-id", "req-87654321",
		"--category", "decision",
		"--subject", "Use shared logger",
		"--content-file", contentPath,
		"--confidence", "4",
		"--related-key", "rule:myproject/rule-1",
		"--tag", "logging",
		"--evidence-key", "rule:myproject/rule-1",
		"--auto-promote=false",
	}, fixedNow)
	if err != nil {
		t.Fatalf("buildConvenienceEnvelope returned error: %v", err)
	}
	if env.Command != v1.CommandProposeMemory {
		t.Fatalf("unexpected command: %s", env.Command)
	}

	var payload v1.ProposeMemoryPayload
	if err := json.Unmarshal(env.Payload, &payload); err != nil {
		t.Fatalf("failed to decode payload: %v", err)
	}
	if payload.Memory.Content != "Prefer shared logger wrappers" {
		t.Fatalf("unexpected content: %q", payload.Memory.Content)
	}
	if payload.AutoPromote == nil || *payload.AutoPromote {
		t.Fatalf("expected auto_promote=false, got %+v", payload.AutoPromote)
	}
}

func TestBuildReportCompletionEnvelope_FilesAndOutcomeFromFiles(t *testing.T) {
	filesPath := filepath.Join(t.TempDir(), "files.json")
	if err := os.WriteFile(filesPath, []byte(`["cmd/acm/main.go","cmd/acm/convenience.go"]`), 0o644); err != nil {
		t.Fatalf("write files fixture: %v", err)
	}
	outcomePath := filepath.Join(t.TempDir(), "outcome.txt")
	if err := os.WriteFile(outcomePath, []byte("Implemented script-friendly flags"), 0o644); err != nil {
		t.Fatalf("write outcome fixture: %v", err)
	}

	env, err := buildConvenienceEnvelope("report-completion", []string{
		"--project", "myproject",
		"--receipt-id", "req-87654321",
		"--files-changed-file", filesPath,
		"--outcome-file", outcomePath,
	}, fixedNow)
	if err != nil {
		t.Fatalf("buildConvenienceEnvelope returned error: %v", err)
	}

	var payload v1.ReportCompletionPayload
	if err := json.Unmarshal(env.Payload, &payload); err != nil {
		t.Fatalf("failed to decode payload: %v", err)
	}
	if len(payload.FilesChanged) != 2 {
		t.Fatalf("expected 2 changed files, got %d", len(payload.FilesChanged))
	}
	if payload.Outcome != "Implemented script-friendly flags" {
		t.Fatalf("unexpected outcome: %q", payload.Outcome)
	}
}

func TestRunConvenienceWithDeps_EndToEndGetContext(t *testing.T) {
	svc := &convenienceFakeService{}
	out := &bytes.Buffer{}
	recorder := logging.NewRecorder()

	code := runConvenienceWithDeps(
		context.Background(),
		recorder,
		"get-context",
		[]string{"--project", "myproject", "--request", "req-12345678", "--task-text", "Add health checks", "--phase", "execute"},
		out,
		fixedNow,
		func(_ context.Context, _ logging.Logger) (core.Service, runtime.CleanupFunc, error) {
			return svc, func() {}, nil
		},
		cli.RunWithLogger,
	)
	if code != 0 {
		t.Fatalf("expected exit code 0, got %d", code)
	}
	if len(svc.getContextCalls) != 1 {
		t.Fatalf("expected one get_context call, got %d", len(svc.getContextCalls))
	}

	var env v1.ResultEnvelope
	if err := json.Unmarshal(out.Bytes(), &env); err != nil {
		t.Fatalf("failed to parse output envelope: %v", err)
	}
	if !env.OK {
		t.Fatalf("expected ok=true, got error %+v", env.Error)
	}
	if env.Command != v1.CommandGetContext {
		t.Fatalf("unexpected command: %s", env.Command)
	}
	if env.RequestID != "req-12345678" {
		t.Fatalf("unexpected request id: %s", env.RequestID)
	}
}

func fixedNow() time.Time {
	return time.Date(2026, 3, 5, 12, 0, 0, 0, time.UTC)
}

type convenienceFakeService struct {
	getContextCalls []v1.GetContextPayload
}

func (f *convenienceFakeService) GetContext(_ context.Context, payload v1.GetContextPayload) (v1.GetContextResult, *core.APIError) {
	f.getContextCalls = append(f.getContextCalls, payload)
	return v1.GetContextResult{Status: "insufficient_context"}, nil
}

func (f *convenienceFakeService) Fetch(_ context.Context, _ v1.FetchPayload) (v1.FetchResult, *core.APIError) {
	return v1.FetchResult{}, nil
}

func (f *convenienceFakeService) ProposeMemory(_ context.Context, _ v1.ProposeMemoryPayload) (v1.ProposeMemoryResult, *core.APIError) {
	return v1.ProposeMemoryResult{}, nil
}

func (f *convenienceFakeService) Work(_ context.Context, _ v1.WorkPayload) (v1.WorkResult, *core.APIError) {
	return v1.WorkResult{}, nil
}

func (f *convenienceFakeService) ReportCompletion(_ context.Context, _ v1.ReportCompletionPayload) (v1.ReportCompletionResult, *core.APIError) {
	return v1.ReportCompletionResult{}, nil
}

func (f *convenienceFakeService) Sync(_ context.Context, _ v1.SyncPayload) (v1.SyncResult, *core.APIError) {
	return v1.SyncResult{}, nil
}

func (f *convenienceFakeService) HealthCheck(_ context.Context, _ v1.HealthCheckPayload) (v1.HealthCheckResult, *core.APIError) {
	return v1.HealthCheckResult{}, nil
}

func (f *convenienceFakeService) HealthFix(_ context.Context, _ v1.HealthFixPayload) (v1.HealthFixResult, *core.APIError) {
	return v1.HealthFixResult{}, nil
}

func (f *convenienceFakeService) Coverage(_ context.Context, _ v1.CoveragePayload) (v1.CoverageResult, *core.APIError) {
	return v1.CoverageResult{}, nil
}

func (f *convenienceFakeService) Regress(_ context.Context, _ v1.RegressPayload) (v1.RegressResult, *core.APIError) {
	return v1.RegressResult{}, nil
}

func (f *convenienceFakeService) Bootstrap(_ context.Context, _ v1.BootstrapPayload) (v1.BootstrapResult, *core.APIError) {
	return v1.BootstrapResult{}, nil
}
