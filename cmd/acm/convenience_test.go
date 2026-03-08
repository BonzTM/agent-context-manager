package main

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/bonztm/agent-context-manager/internal/adapters/cli"
	"github.com/bonztm/agent-context-manager/internal/contracts/v1"
	"github.com/bonztm/agent-context-manager/internal/core"
	"github.com/bonztm/agent-context-manager/internal/logging"
	"github.com/bonztm/agent-context-manager/internal/runtime"
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

func TestBuildFetchEnvelope_LoadsInlineJSON(t *testing.T) {
	env, err := buildConvenienceEnvelope("fetch", []string{
		"--project", "myproject",
		"--keys-json", `["plan:req-12345678","rule:myproject/rule-2"]`,
		"--expected-versions-json", `{"plan:req-12345678":"v3","rule:myproject/rule-2":"v7"}`,
	}, fixedNow)
	if err != nil {
		t.Fatalf("buildConvenienceEnvelope returned error: %v", err)
	}

	var payload v1.FetchPayload
	if err := json.Unmarshal(env.Payload, &payload); err != nil {
		t.Fatalf("failed to decode payload: %v", err)
	}
	if len(payload.Keys) != 2 {
		t.Fatalf("expected 2 keys from inline json, got %d", len(payload.Keys))
	}
	if payload.ExpectedVersions["plan:req-12345678"] != "v3" {
		t.Fatalf("unexpected version for plan key: %q", payload.ExpectedVersions["plan:req-12345678"])
	}
	if payload.ExpectedVersions["rule:myproject/rule-2"] != "v7" {
		t.Fatalf("unexpected version for rule key: %q", payload.ExpectedVersions["rule:myproject/rule-2"])
	}
}

func TestBuildFetchEnvelope_ReceiptShorthandOmitsEmptyKeys(t *testing.T) {
	env, err := buildConvenienceEnvelope("fetch", []string{
		"--project", "myproject",
		"--receipt-id", "req-87654321",
	}, fixedNow)
	if err != nil {
		t.Fatalf("buildConvenienceEnvelope returned error: %v", err)
	}

	var raw map[string]any
	if err := json.Unmarshal(env.Payload, &raw); err != nil {
		t.Fatalf("failed to decode raw payload: %v", err)
	}
	if _, ok := raw["keys"]; ok {
		t.Fatalf("expected keys to be omitted for receipt shorthand payload, got %v", raw["keys"])
	}

	var payload v1.FetchPayload
	if err := json.Unmarshal(env.Payload, &payload); err != nil {
		t.Fatalf("failed to decode payload: %v", err)
	}
	if payload.ReceiptID != "req-87654321" {
		t.Fatalf("unexpected receipt id: %s", payload.ReceiptID)
	}
	if len(payload.Keys) != 0 {
		t.Fatalf("expected no keys, got %d", len(payload.Keys))
	}
}

func TestBuildWorkEnvelope_LoadsTasksFile(t *testing.T) {
	tasksPath := filepath.Join(t.TempDir(), "tasks.json")
	if err := os.WriteFile(tasksPath, []byte(`[
		{"key":"verify:tests","summary":"Run tests","status":"pending"},
		{"key":"verify:diff-review","summary":"Review diff","status":"complete","outcome":"No regressions"}
	]`), 0o644); err != nil {
		t.Fatalf("failed to write test fixture: %v", err)
	}

	env, err := buildConvenienceEnvelope("work", []string{
		"--project", "myproject",
		"--request", "req-12345678",
		"--receipt-id", "req-87654321",
		"--tasks-file", tasksPath,
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
	if len(payload.Tasks) != 2 {
		t.Fatalf("expected 2 work tasks, got %d", len(payload.Tasks))
	}
	if payload.Tasks[0].Key != "verify:tests" {
		t.Fatalf("unexpected first task key: %q", payload.Tasks[0].Key)
	}
}

func TestBuildWorkEnvelope_LoadsTasksJSON(t *testing.T) {
	env, err := buildConvenienceEnvelope("work", []string{
		"--project", "myproject",
		"--request", "req-12345678",
		"--receipt-id", "req-87654321",
		"--tasks-json", `[{"key":"verify:tests","summary":"Run tests","status":"pending"}]`,
	}, fixedNow)
	if err != nil {
		t.Fatalf("buildConvenienceEnvelope returned error: %v", err)
	}

	var payload v1.WorkPayload
	if err := json.Unmarshal(env.Payload, &payload); err != nil {
		t.Fatalf("failed to decode payload: %v", err)
	}
	if len(payload.Tasks) != 1 {
		t.Fatalf("expected 1 work task, got %d", len(payload.Tasks))
	}
	if payload.Tasks[0].Key != "verify:tests" {
		t.Fatalf("unexpected task key: %q", payload.Tasks[0].Key)
	}
}

func TestBuildWorkEnvelope_TasksJSONConflict(t *testing.T) {
	tasksPath := filepath.Join(t.TempDir(), "tasks.json")
	if err := os.WriteFile(tasksPath, []byte(`[]`), 0o644); err != nil {
		t.Fatalf("failed to write test fixture: %v", err)
	}

	_, err := buildConvenienceEnvelope("work", []string{
		"--project", "myproject",
		"--tasks-file", tasksPath,
		"--tasks-json", `[]`,
	}, fixedNow)
	if err == nil {
		t.Fatal("expected conflict error, got nil")
	}
	if !strings.Contains(err.Error(), "use only one of --tasks-file or --tasks-json") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestBuildWorkEnvelope_LoadsPlanAndTasksJSON(t *testing.T) {
	env, err := buildConvenienceEnvelope("work", []string{
		"--project", "myproject",
		"--request", "req-12345678",
		"--plan-key", "plan:req-87654321",
		"--receipt-id", "req-87654321",
		"--mode", "replace",
		"--plan-json", `{
			"title":"Bootstrap this repo",
			"objective":"Capture spec, tasks, and outcomes in acm",
			"status":"in_progress",
			"stages":{
				"spec_outline":"complete",
				"refined_spec":"in_progress",
				"implementation_plan":"pending"
			},
			"in_scope":["internal/service/backend","cmd/acm"],
			"out_of_scope":["release automation"],
			"constraints":["no breaking APIs"],
			"references":["docs/getting-started.md"]
		}`,
		"--tasks-json", `[{
			"key":"task.bootstrap",
			"summary":"Implement bootstrap flow",
			"status":"in_progress",
			"depends_on":["task.spec"],
			"acceptance_criteria":["bootstrap persists canonical rules"],
			"references":["doc:bootstrap-spec"],
			"blocked_reason":"waiting for review",
			"outcome":"command executes end to end",
			"evidence":["go test ./..."]
		}]`,
	}, fixedNow)
	if err != nil {
		t.Fatalf("buildConvenienceEnvelope returned error: %v", err)
	}

	var payload v1.WorkPayload
	if err := json.Unmarshal(env.Payload, &payload); err != nil {
		t.Fatalf("failed to decode payload: %v", err)
	}
	if payload.Mode != v1.WorkPlanModeReplace {
		t.Fatalf("unexpected mode: %q", payload.Mode)
	}
	if payload.Plan == nil {
		t.Fatal("expected plan payload")
	}
	if payload.Plan.Title != "Bootstrap this repo" || payload.Plan.Objective != "Capture spec, tasks, and outcomes in acm" {
		t.Fatalf("unexpected plan metadata: %+v", payload.Plan)
	}
	if payload.Plan.Stages == nil || payload.Plan.Stages.SpecOutline != v1.WorkItemStatusComplete {
		t.Fatalf("unexpected plan stages: %+v", payload.Plan.Stages)
	}
	if len(payload.Tasks) != 1 {
		t.Fatalf("expected 1 task, got %d", len(payload.Tasks))
	}
	if payload.Tasks[0].Key != "task.bootstrap" || payload.Tasks[0].Status != v1.WorkItemStatusInProgress {
		t.Fatalf("unexpected task payload: %+v", payload.Tasks[0])
	}
}

func TestBuildReviewEnvelope_LoadsOutcomeFileAndEvidenceJSON(t *testing.T) {
	outcomePath := filepath.Join(t.TempDir(), "review-outcome.txt")
	if err := os.WriteFile(outcomePath, []byte("Cross-LLM review passed with no blocking issues."), 0o644); err != nil {
		t.Fatalf("write outcome fixture: %v", err)
	}

	env, err := buildConvenienceEnvelope("review", []string{
		"--project", "myproject",
		"--request", "req-12345678",
		"--receipt-id", "receipt-87654321",
		"--outcome-file", outcomePath,
		"--evidence-json", `["review://cross-llm/run-1","comment://issue-42"]`,
	}, fixedNow)
	if err != nil {
		t.Fatalf("buildConvenienceEnvelope returned error: %v", err)
	}
	if env.Command != v1.CommandReview {
		t.Fatalf("unexpected command: %s", env.Command)
	}

	var payload v1.ReviewPayload
	if err := json.Unmarshal(env.Payload, &payload); err != nil {
		t.Fatalf("failed to decode payload: %v", err)
	}
	if payload.ReceiptID != "receipt-87654321" {
		t.Fatalf("unexpected receipt id: %q", payload.ReceiptID)
	}
	if payload.Key != "" {
		t.Fatalf("unexpected review key: %q", payload.Key)
	}
	if payload.Status != "" {
		t.Fatalf("unexpected review status: %q", payload.Status)
	}
	if payload.Outcome != "Cross-LLM review passed with no blocking issues." {
		t.Fatalf("unexpected review outcome: %q", payload.Outcome)
	}
	if len(payload.Evidence) != 2 {
		t.Fatalf("expected 2 evidence values, got %d", len(payload.Evidence))
	}
}

func TestBuildReviewEnvelope_RequiresSelectionContext(t *testing.T) {
	_, err := buildConvenienceEnvelope("review", []string{
		"--project", "myproject",
	}, fixedNow)
	if err == nil {
		t.Fatal("expected error for missing review selection context")
	}
	if !strings.Contains(err.Error(), "review requires --receipt-id or --plan-key") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestBuildReviewEnvelope_RunModeIncludesTagsFile(t *testing.T) {
	env, err := buildConvenienceEnvelope("review", []string{
		"--project", "myproject",
		"--receipt-id", "receipt-87654321",
		"--run",
		"--tags-file", ".acm/acm-tags.yaml",
	}, fixedNow)
	if err != nil {
		t.Fatalf("buildConvenienceEnvelope returned error: %v", err)
	}

	var payload v1.ReviewPayload
	if err := json.Unmarshal(env.Payload, &payload); err != nil {
		t.Fatalf("failed to decode payload: %v", err)
	}
	if !payload.Run {
		t.Fatalf("expected run=true, got %+v", payload)
	}
	if payload.TagsFile != ".acm/acm-tags.yaml" {
		t.Fatalf("unexpected tags file: %q", payload.TagsFile)
	}
}

func TestBuildReviewEnvelope_RunModeRejectsManualOutcomeFields(t *testing.T) {
	_, err := buildConvenienceEnvelope("review", []string{
		"--project", "myproject",
		"--receipt-id", "receipt-87654321",
		"--run",
		"--outcome", "No blocking review findings.",
	}, fixedNow)
	if err == nil {
		t.Fatal("expected error for run mode with manual outcome")
	}
	if !strings.Contains(err.Error(), "--outcome and --outcome-file are only supported when --run is omitted") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestBuildHistorySearchEnvelope_ForWorkListDefaultsToCurrent(t *testing.T) {
	env, err := buildConvenienceEnvelope("work-list", []string{
		"--project", "myproject",
		"--limit", "7",
		"--unbounded",
	}, fixedNow)
	if err != nil {
		t.Fatalf("buildConvenienceEnvelope returned error: %v", err)
	}
	if env.Command != v1.CommandHistorySearch {
		t.Fatalf("unexpected command: %s", env.Command)
	}

	var payload v1.HistorySearchPayload
	if err := json.Unmarshal(env.Payload, &payload); err != nil {
		t.Fatalf("failed to decode payload: %v", err)
	}
	if payload.ProjectID != "myproject" || payload.Entity != v1.HistoryEntityWork || payload.Scope != v1.HistoryScopeCurrent || payload.Limit != 7 || payload.Query != "" {
		t.Fatalf("unexpected payload: %+v", payload)
	}
	if payload.Unbounded == nil || !*payload.Unbounded {
		t.Fatalf("expected unbounded payload flag, got %+v", payload.Unbounded)
	}
}

func TestBuildHistorySearchEnvelope_ForWorkSearchLoadsQueryFile(t *testing.T) {
	queryPath := filepath.Join(t.TempDir(), "query.txt")
	if err := os.WriteFile(queryPath, []byte("  bootstrap history  \n"), 0o644); err != nil {
		t.Fatalf("write query fixture: %v", err)
	}

	env, err := buildConvenienceEnvelope("work-search", []string{
		"--project", "myproject",
		"--query-file", queryPath,
		"--scope", "completed",
		"--kind", "story",
	}, fixedNow)
	if err != nil {
		t.Fatalf("buildConvenienceEnvelope returned error: %v", err)
	}

	var payload v1.HistorySearchPayload
	if err := json.Unmarshal(env.Payload, &payload); err != nil {
		t.Fatalf("failed to decode payload: %v", err)
	}
	if payload.ProjectID != "myproject" || payload.Entity != v1.HistoryEntityWork || payload.Query != "bootstrap history" || payload.Scope != v1.HistoryScopeCompleted || payload.Kind != "story" || payload.Limit != 20 {
		t.Fatalf("unexpected payload: %+v", payload)
	}
}

func TestBuildHistorySearchEnvelope_RequiresQueryForSearchCommands(t *testing.T) {
	if _, err := buildConvenienceEnvelope("work-search", []string{
		"--project", "myproject",
	}, fixedNow); err == nil || !strings.Contains(err.Error(), "--query or --query-file is required") {
		t.Fatalf("expected query requirement error, got %v", err)
	}
}

func TestBuildHistorySearchEnvelope_ForGenericHistoryDefaultsToAllEntities(t *testing.T) {
	env, err := buildConvenienceEnvelope("history-search", []string{
		"--project", "myproject",
		"--limit", "9",
	}, fixedNow)
	if err != nil {
		t.Fatalf("buildConvenienceEnvelope returned error: %v", err)
	}

	var payload v1.HistorySearchPayload
	if err := json.Unmarshal(env.Payload, &payload); err != nil {
		t.Fatalf("failed to decode payload: %v", err)
	}
	if payload.ProjectID != "myproject" || payload.Entity != v1.HistoryEntityAll || payload.Scope != "" || payload.Kind != "" || payload.Limit != 9 || payload.Query != "" {
		t.Fatalf("unexpected payload: %+v", payload)
	}
}

func TestBuildHistorySearchEnvelope_AllowsMemoryEntity(t *testing.T) {
	env, err := buildConvenienceEnvelope("history-search", []string{
		"--project", "myproject",
		"--entity", "memory",
		"--query", "bootstrap",
	}, fixedNow)
	if err != nil {
		t.Fatalf("buildConvenienceEnvelope returned error: %v", err)
	}

	var payload v1.HistorySearchPayload
	if err := json.Unmarshal(env.Payload, &payload); err != nil {
		t.Fatalf("failed to decode payload: %v", err)
	}
	if payload.Entity != v1.HistoryEntityMemory || payload.Query != "bootstrap" {
		t.Fatalf("unexpected payload: %+v", payload)
	}
}

func TestBuildHistorySearchEnvelope_GenericHistoryRejectsWorkOnlyFlags(t *testing.T) {
	_, err := buildConvenienceEnvelope("history-search", []string{
		"--project", "myproject",
		"--scope", "all",
	}, fixedNow)
	if err == nil {
		t.Fatal("expected error for generic history scope flag")
	}
	if !strings.Contains(err.Error(), "flag provided but not defined") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestBuildVerifyEnvelope_RequiresSelectionContext(t *testing.T) {
	_, err := buildConvenienceEnvelope("verify", []string{
		"--project", "myproject",
	}, fixedNow)
	if err == nil {
		t.Fatal("expected error for missing verify selection context")
	}
	if !strings.Contains(err.Error(), "verify requires --test-id or selection context") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestBuildProposeMemoryEnvelope_ContentFile(t *testing.T) {
	contentPath := filepath.Join(t.TempDir(), "memory-content.txt")
	if err := os.WriteFile(contentPath, []byte("Prefer shared logger wrappers"), 0o644); err != nil {
		t.Fatalf("write content fixture: %v", err)
	}
	memoryTagsPath := filepath.Join(t.TempDir(), "memory-tags.json")
	if err := os.WriteFile(memoryTagsPath, []byte(`["go"]`), 0o644); err != nil {
		t.Fatalf("write memory tags fixture: %v", err)
	}

	env, err := buildConvenienceEnvelope("propose-memory", []string{
		"--project", "myproject",
		"--request", "req-12345678",
		"--receipt-id", "req-87654321",
		"--category", "decision",
		"--subject", "Use shared logger",
		"--content-file", contentPath,
		"--confidence", "4",
		"--tags-file", ".acm/acm-tags.yaml",
		"--related-key", "rule:myproject/rule-1",
		"--memory-tag", "logging",
		"--memory-tags-file", memoryTagsPath,
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
	if len(payload.Memory.Tags) != 2 {
		t.Fatalf("expected 2 memory tags, got %d", len(payload.Memory.Tags))
	}
	if payload.TagsFile != ".acm/acm-tags.yaml" {
		t.Fatalf("unexpected tags_file: %q", payload.TagsFile)
	}
	if payload.AutoPromote == nil || *payload.AutoPromote {
		t.Fatalf("expected auto_promote=false, got %+v", payload.AutoPromote)
	}
}

func TestBuildProposeMemoryEnvelope_LoadsInlineJSONArrays(t *testing.T) {
	env, err := buildConvenienceEnvelope("propose-memory", []string{
		"--project", "myproject",
		"--receipt-id", "req-87654321",
		"--category", "decision",
		"--subject", "Use shared logger",
		"--content", "Prefer one logger wrapper",
		"--confidence", "4",
		"--related-keys-json", `["rule:myproject/rule-1","rule:myproject/rule-2"]`,
		"--memory-tags-json", `["logging","go"]`,
		"--evidence-keys-json", `["rule:myproject/rule-2"]`,
	}, fixedNow)
	if err != nil {
		t.Fatalf("buildConvenienceEnvelope returned error: %v", err)
	}

	var payload v1.ProposeMemoryPayload
	if err := json.Unmarshal(env.Payload, &payload); err != nil {
		t.Fatalf("failed to decode payload: %v", err)
	}
	if len(payload.Memory.RelatedPointerKeys) != 2 {
		t.Fatalf("expected 2 related keys, got %d", len(payload.Memory.RelatedPointerKeys))
	}
	if len(payload.Memory.Tags) != 2 {
		t.Fatalf("expected 2 tags, got %d", len(payload.Memory.Tags))
	}
	if len(payload.Memory.EvidencePointerKeys) != 1 {
		t.Fatalf("expected 1 evidence key, got %d", len(payload.Memory.EvidencePointerKeys))
	}
}

func TestBuildProposeMemoryEnvelope_LoadsMemoryTagsFile(t *testing.T) {
	memoryTagsPath := filepath.Join(t.TempDir(), "memory-tags.json")
	if err := os.WriteFile(memoryTagsPath, []byte(`["logging","go"]`), 0o644); err != nil {
		t.Fatalf("write memory tags fixture: %v", err)
	}

	env, err := buildConvenienceEnvelope("propose-memory", []string{
		"--project", "myproject",
		"--receipt-id", "req-87654321",
		"--category", "decision",
		"--subject", "Use shared logger",
		"--content", "Prefer one logger wrapper",
		"--confidence", "4",
		"--memory-tags-file", memoryTagsPath,
		"--tags-file", ".acm/acm-tags.yaml",
		"--evidence-key", "rule:myproject/rule-2",
	}, fixedNow)
	if err != nil {
		t.Fatalf("buildConvenienceEnvelope returned error: %v", err)
	}

	var payload v1.ProposeMemoryPayload
	if err := json.Unmarshal(env.Payload, &payload); err != nil {
		t.Fatalf("failed to decode payload: %v", err)
	}
	if payload.TagsFile != ".acm/acm-tags.yaml" {
		t.Fatalf("unexpected tags_file: %q", payload.TagsFile)
	}
	if len(payload.Memory.Tags) != 2 {
		t.Fatalf("expected 2 tags, got %d", len(payload.Memory.Tags))
	}
}

func TestBuildProposeMemoryEnvelope_RejectsLegacyTagFlags(t *testing.T) {
	for _, legacyFlag := range []string{"--tag", "--tags-json"} {
		t.Run(legacyFlag, func(t *testing.T) {
			args := []string{
				"--project", "myproject",
				"--receipt-id", "req-87654321",
				"--category", "decision",
				"--subject", "Use shared logger",
				"--content", "Prefer one logger wrapper",
				"--confidence", "4",
				"--evidence-key", "rule:myproject/rule-2",
				legacyFlag,
			}
			if legacyFlag == "--tag" {
				args = append(args, "logging")
			} else {
				args = append(args, `["logging"]`)
			}
			_, err := buildConvenienceEnvelope("propose-memory", args, fixedNow)
			if err == nil {
				t.Fatal("expected flag parsing error, got nil")
			}
			if !strings.Contains(err.Error(), "flag provided but not defined") {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestBuildGetContextEnvelope_TagsFileFlag(t *testing.T) {
	env, err := buildConvenienceEnvelope("get-context", []string{
		"--project", "myproject",
		"--task-text", "Check sync tags",
		"--phase", "execute",
		"--tags-file", ".acm/acm-tags.yaml",
	}, fixedNow)
	if err != nil {
		t.Fatalf("buildConvenienceEnvelope returned error: %v", err)
	}

	var payload v1.GetContextPayload
	if err := json.Unmarshal(env.Payload, &payload); err != nil {
		t.Fatalf("failed to decode payload: %v", err)
	}
	if payload.TagsFile != ".acm/acm-tags.yaml" {
		t.Fatalf("unexpected tags_file: %q", payload.TagsFile)
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

func TestBuildReportCompletionEnvelope_TagsFileFlag(t *testing.T) {
	env, err := buildConvenienceEnvelope("report-completion", []string{
		"--project", "myproject",
		"--receipt-id", "req-12345678",
		"--file-changed", "cmd/acm/main.go",
		"--outcome", "Done",
		"--tags-file", ".acm/acm-tags.yaml",
	}, fixedNow)
	if err != nil {
		t.Fatalf("buildConvenienceEnvelope returned error: %v", err)
	}

	var payload v1.ReportCompletionPayload
	if err := json.Unmarshal(env.Payload, &payload); err != nil {
		t.Fatalf("failed to decode payload: %v", err)
	}
	if payload.TagsFile != ".acm/acm-tags.yaml" {
		t.Fatalf("unexpected tags_file: %q", payload.TagsFile)
	}
}

func TestBuildReportCompletionEnvelope_LoadsFilesChangedJSON(t *testing.T) {
	env, err := buildConvenienceEnvelope("report-completion", []string{
		"--project", "myproject",
		"--receipt-id", "req-87654321",
		"--outcome", "Done",
		"--files-changed-json", `["cmd/acm/main.go","cmd/acm/convenience.go"]`,
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
}

func TestBuildEvalEnvelope_LoadsInlineJSONSuite(t *testing.T) {
	env, err := buildConvenienceEnvelope("eval", []string{
		"--project", "myproject",
		"--eval-suite-inline-json", `[{"task_text":"Check sync","phase":"execute","expected_pointer_keys":["rule:myproject/rule-1"]}]`,
	}, fixedNow)
	if err != nil {
		t.Fatalf("buildConvenienceEnvelope returned error: %v", err)
	}

	var payload v1.EvalPayload
	if err := json.Unmarshal(env.Payload, &payload); err != nil {
		t.Fatalf("failed to decode payload: %v", err)
	}
	if payload.EvalSuitePath != "" {
		t.Fatalf("expected empty eval_suite_path, got %q", payload.EvalSuitePath)
	}
	if len(payload.EvalSuiteInline) != 1 {
		t.Fatalf("expected 1 inline case, got %d", len(payload.EvalSuiteInline))
	}
	if payload.EvalSuiteInline[0].TaskText != "Check sync" {
		t.Fatalf("unexpected task text: %q", payload.EvalSuiteInline[0].TaskText)
	}
}

func TestBuildSyncEnvelope_RulesAndTagsFileFlags(t *testing.T) {
	env, err := buildConvenienceEnvelope("sync", []string{
		"--project", "myproject",
		"--rules-file", ".acm/acm-rules.yaml",
		"--tags-file", ".acm/acm-tags.yaml",
	}, fixedNow)
	if err != nil {
		t.Fatalf("buildConvenienceEnvelope returned error: %v", err)
	}

	var payload v1.SyncPayload
	if err := json.Unmarshal(env.Payload, &payload); err != nil {
		t.Fatalf("failed to decode payload: %v", err)
	}
	if payload.RulesFile != ".acm/acm-rules.yaml" {
		t.Fatalf("unexpected rules_file: %q", payload.RulesFile)
	}
	if payload.TagsFile != ".acm/acm-tags.yaml" {
		t.Fatalf("unexpected tags_file: %q", payload.TagsFile)
	}
}

func TestBuildHealthFixEnvelope_RulesAndTagsFileFlags(t *testing.T) {
	env, err := buildConvenienceEnvelope("health-fix", []string{
		"--project", "myproject",
		"--rules-file", "custom-rules.yaml",
		"--tags-file", "custom-tags.json",
		"--fixer", "sync_ruleset",
	}, fixedNow)
	if err != nil {
		t.Fatalf("buildConvenienceEnvelope returned error: %v", err)
	}

	var payload v1.HealthFixPayload
	if err := json.Unmarshal(env.Payload, &payload); err != nil {
		t.Fatalf("failed to decode payload: %v", err)
	}
	if payload.RulesFile != "custom-rules.yaml" {
		t.Fatalf("unexpected rules_file: %q", payload.RulesFile)
	}
	if payload.TagsFile != "custom-tags.json" {
		t.Fatalf("unexpected tags_file: %q", payload.TagsFile)
	}
}

func TestBuildStatusEnvelope_LoadsTaskFileAndOptionalSources(t *testing.T) {
	taskFile := filepath.Join(t.TempDir(), "task.txt")
	if err := os.WriteFile(taskFile, []byte(" diagnose get_context drift \n"), 0o644); err != nil {
		t.Fatalf("write task file: %v", err)
	}

	env, err := buildConvenienceEnvelope("status", []string{
		"--project", "myproject",
		"--project-root", ".",
		"--rules-file", ".acm/acm-rules.yaml",
		"--tags-file", ".acm/acm-tags.yaml",
		"--tests-file", ".acm/acm-tests.yaml",
		"--workflows-file", ".acm/acm-workflows.yaml",
		"--task-file", taskFile,
		"--phase", "review",
	}, fixedNow)
	if err != nil {
		t.Fatalf("buildConvenienceEnvelope returned error: %v", err)
	}

	var payload v1.StatusPayload
	if err := json.Unmarshal(env.Payload, &payload); err != nil {
		t.Fatalf("failed to decode payload: %v", err)
	}
	if payload.ProjectID != "myproject" {
		t.Fatalf("unexpected project_id: %q", payload.ProjectID)
	}
	if payload.ProjectRoot != "." {
		t.Fatalf("unexpected project_root: %q", payload.ProjectRoot)
	}
	if payload.RulesFile != ".acm/acm-rules.yaml" || payload.TagsFile != ".acm/acm-tags.yaml" {
		t.Fatalf("unexpected rules/tags files: %+v", payload)
	}
	if payload.TestsFile != ".acm/acm-tests.yaml" || payload.WorkflowsFile != ".acm/acm-workflows.yaml" {
		t.Fatalf("unexpected tests/workflows files: %+v", payload)
	}
	if payload.TaskText != "diagnose get_context drift" {
		t.Fatalf("unexpected task_text: %q", payload.TaskText)
	}
	if payload.Phase != v1.PhaseReview {
		t.Fatalf("unexpected phase: %q", payload.Phase)
	}
}

func TestBuildBootstrapEnvelope_RulesAndTagsFileFlags(t *testing.T) {
	env, err := buildConvenienceEnvelope("bootstrap", []string{
		"--project", "myproject",
		"--project-root", ".",
		"--apply-template", "starter-contract",
		"--apply-template", "verify-go",
		"--rules-file", "legacy-rules.yaml",
		"--tags-file", "legacy-tags.json",
		"--persist-candidates",
	}, fixedNow)
	if err != nil {
		t.Fatalf("buildConvenienceEnvelope returned error: %v", err)
	}

	var payload v1.BootstrapPayload
	if err := json.Unmarshal(env.Payload, &payload); err != nil {
		t.Fatalf("failed to decode payload: %v", err)
	}
	if payload.RulesFile != "legacy-rules.yaml" {
		t.Fatalf("unexpected rules_file: %q", payload.RulesFile)
	}
	if payload.TagsFile != "legacy-tags.json" {
		t.Fatalf("unexpected tags_file: %q", payload.TagsFile)
	}
	if want := []string{"starter-contract", "verify-go"}; !reflect.DeepEqual(payload.ApplyTemplates, want) {
		t.Fatalf("unexpected apply_templates: got %v want %v", payload.ApplyTemplates, want)
	}
	if payload.PersistCandidates == nil || !*payload.PersistCandidates {
		t.Fatalf("expected persist_candidates=true, got %+v", payload.PersistCandidates)
	}
}

func TestBuildBootstrapEnvelope_AllowsOmittedProjectAndRoot(t *testing.T) {
	env, err := buildConvenienceEnvelope("bootstrap", []string{
		"--apply-template", "starter-contract",
	}, fixedNow)
	if err != nil {
		t.Fatalf("buildConvenienceEnvelope returned error: %v", err)
	}

	var payload v1.BootstrapPayload
	if err := json.Unmarshal(env.Payload, &payload); err != nil {
		t.Fatalf("failed to decode payload: %v", err)
	}
	if payload.ProjectID != "" {
		t.Fatalf("expected empty project_id for runtime inference, got %q", payload.ProjectID)
	}
	if payload.ProjectRoot != "" {
		t.Fatalf("expected empty project_root for runtime inference, got %q", payload.ProjectRoot)
	}
	if want := []string{"starter-contract"}; !reflect.DeepEqual(payload.ApplyTemplates, want) {
		t.Fatalf("unexpected apply_templates: got %v want %v", payload.ApplyTemplates, want)
	}
}

func TestBuildEvalEnvelope_TagsFileFlag(t *testing.T) {
	env, err := buildConvenienceEnvelope("eval", []string{
		"--project", "myproject",
		"--eval-suite-inline-json", `[{"task_text":"Check sync","phase":"execute"}]`,
		"--tags-file", ".acm/acm-tags.yaml",
	}, fixedNow)
	if err != nil {
		t.Fatalf("buildConvenienceEnvelope returned error: %v", err)
	}

	var payload v1.EvalPayload
	if err := json.Unmarshal(env.Payload, &payload); err != nil {
		t.Fatalf("failed to decode payload: %v", err)
	}
	if payload.TagsFile != ".acm/acm-tags.yaml" {
		t.Fatalf("unexpected tags_file: %q", payload.TagsFile)
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

func TestRunConvenienceWithDeps_DefaultsProjectIDFromEnv(t *testing.T) {
	t.Setenv(runtime.ProjectIDEnvVar, "env-project")

	svc := &convenienceFakeService{}
	out := &bytes.Buffer{}
	recorder := logging.NewRecorder()

	code := runConvenienceWithDeps(
		context.Background(),
		recorder,
		"get-context",
		[]string{"--request", "req-12345678", "--task-text", "Add health checks", "--phase", "execute"},
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
	if got := svc.getContextCalls[0].ProjectID; got != "env-project" {
		t.Fatalf("unexpected inferred project id: %q", got)
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

func (f *convenienceFakeService) Review(_ context.Context, _ v1.ReviewPayload) (v1.ReviewResult, *core.APIError) {
	return v1.ReviewResult{}, nil
}

func (f *convenienceFakeService) Work(_ context.Context, _ v1.WorkPayload) (v1.WorkResult, *core.APIError) {
	return v1.WorkResult{}, nil
}

func (f *convenienceFakeService) HistorySearch(_ context.Context, _ v1.HistorySearchPayload) (v1.HistorySearchResult, *core.APIError) {
	return v1.HistorySearchResult{}, nil
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

func (f *convenienceFakeService) Status(_ context.Context, _ v1.StatusPayload) (v1.StatusResult, *core.APIError) {
	return v1.StatusResult{}, nil
}

func (f *convenienceFakeService) Coverage(_ context.Context, _ v1.CoveragePayload) (v1.CoverageResult, *core.APIError) {
	return v1.CoverageResult{}, nil
}

func (f *convenienceFakeService) Eval(_ context.Context, _ v1.EvalPayload) (v1.EvalResult, *core.APIError) {
	return v1.EvalResult{}, nil
}

func (f *convenienceFakeService) Verify(_ context.Context, _ v1.VerifyPayload) (v1.VerifyResult, *core.APIError) {
	return v1.VerifyResult{}, nil
}

func (f *convenienceFakeService) Bootstrap(_ context.Context, _ v1.BootstrapPayload) (v1.BootstrapResult, *core.APIError) {
	return v1.BootstrapResult{}, nil
}
