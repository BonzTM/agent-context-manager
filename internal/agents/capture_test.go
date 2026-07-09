package agents

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bonztm/agent-context-manager/internal/core"
)

func TestCaptureClaudeUserPrompt(t *testing.T) {
	payload := []byte(`{"session_id":"s1","turn_id":"turn-1","hook_event_name":"UserPromptSubmit","prompt":"refactor the auth module"}`)
	req, err := Capture(core.AgentClaude, EventUserPromptSubmit, payload)
	if err != nil {
		t.Fatalf("capture: %v", err)
	}
	if req.SessionID != "s1" || len(req.Messages) != 1 {
		t.Fatalf("unexpected request: %+v", req)
	}
	if req.Messages[0].Role != core.RoleUser || req.Messages[0].Content != "refactor the auth module" {
		t.Fatalf("unexpected message: %+v", req.Messages[0])
	}
	if req.Messages[0].ExternalID != "turn-1:input:0" {
		t.Fatalf("external id = %q", req.Messages[0].ExternalID)
	}
}

func TestCaptureClaudePostToolUse(t *testing.T) {
	payload := []byte(`{"session_id":"s1","tool_use_id":"tool-1","tool_name":"Bash","tool_input":{"command":"ls"},"tool_response":{"stdout":"a\nb"}}`)
	req, err := Capture(core.AgentClaude, EventPostToolUse, payload)
	if err != nil {
		t.Fatalf("capture: %v", err)
	}
	if len(req.Messages) != 1 || req.Messages[0].Role != core.RoleTool {
		t.Fatalf("expected one tool message: %+v", req)
	}
	if req.Messages[0].ToolName != "Bash" {
		t.Fatalf("tool name = %q", req.Messages[0].ToolName)
	}
	if req.Messages[0].ExternalID != "tool-1" {
		t.Fatalf("external id = %q", req.Messages[0].ExternalID)
	}
}

func TestCaptureCodexTurnComplete(t *testing.T) {
	payload := []byte(`{"thread-id":"t1","turn-id":"u1","input-messages":["do the thing"],"last-assistant-message":"did the thing"}`)
	req, err := Capture(core.AgentCodex, EventTurnComplete, payload)
	if err != nil {
		t.Fatalf("capture: %v", err)
	}
	if req.SessionID != "t1" {
		t.Fatalf("session id = %q, want t1", req.SessionID)
	}
	if len(req.Messages) != 2 {
		t.Fatalf("expected user+assistant messages, got %d", len(req.Messages))
	}
	if req.Messages[0].Role != core.RoleUser || req.Messages[1].Role != core.RoleAssistant {
		t.Fatalf("unexpected roles: %+v", req.Messages)
	}
	if req.Messages[1].ExternalID != "u1" {
		t.Fatalf("assistant external id = %q, want turn id u1", req.Messages[1].ExternalID)
	}
	if req.Messages[0].ExternalID != "u1:input:0" {
		t.Fatalf("user external id = %q, want turn input id", req.Messages[0].ExternalID)
	}
}

func TestCaptureStopHasNoMessages(t *testing.T) {
	req, err := Capture(core.AgentClaude, EventStop, []byte(`{"session_id":"s1"}`))
	if err != nil {
		t.Fatalf("capture: %v", err)
	}
	if len(req.Messages) != 0 {
		t.Fatalf("expected no messages for Stop, got %d", len(req.Messages))
	}
}

func TestCaptureMalformedJSON(t *testing.T) {
	if _, err := Capture(core.AgentClaude, EventUserPromptSubmit, []byte("{not json")); err == nil {
		t.Fatal("expected error for malformed payload")
	}
}

func TestCaptureClaudeStopReadsTranscript(t *testing.T) {
	transcript := filepath.Join(t.TempDir(), "session.jsonl")
	lines := strings.Join([]string{
		`{"type":"user","uuid":"u1","message":{"role":"user","content":[{"type":"text","text":"do the thing"}]}}`,
		`{"type":"assistant","uuid":"a1","message":{"role":"assistant","content":[{"type":"text","text":"working on it"},{"type":"tool_use","name":"Bash"}]}}`,
		`{"type":"assistant","uuid":"a2","message":{"role":"assistant","content":[{"type":"text","text":"done, tests pass"}]}}`,
		`not json at all`,
	}, "\n")
	if err := os.WriteFile(transcript, []byte(lines), 0o600); err != nil {
		t.Fatal(err)
	}

	payload := fmt.Sprintf(`{"session_id":"s1","transcript_path":%q}`, transcript)
	req, err := Capture(core.AgentClaude, EventStop, []byte(payload))
	if err != nil {
		t.Fatalf("capture: %v", err)
	}
	if len(req.Messages) != 2 {
		t.Fatalf("expected 2 assistant messages, got %d: %+v", len(req.Messages), req.Messages)
	}
	if req.Messages[0].Role != core.RoleAssistant || req.Messages[0].Content != "working on it" {
		t.Fatalf("unexpected first assistant message: %+v", req.Messages[0])
	}
	if req.Messages[0].ExternalID != "a1" || req.Messages[1].ExternalID != "a2" {
		t.Fatalf("assistant messages must carry transcript uuids: %+v", req.Messages)
	}
}

func TestCaptureClaudeStopWithoutTranscriptIsEmpty(t *testing.T) {
	req, err := Capture(core.AgentClaude, EventStop, []byte(`{"session_id":"s1"}`))
	if err != nil {
		t.Fatalf("capture: %v", err)
	}
	if len(req.Messages) != 0 {
		t.Fatalf("expected no messages, got %d", len(req.Messages))
	}
}

func TestCaptureCodexStopReadsRollout(t *testing.T) {
	transcript := filepath.Join(t.TempDir(), "rollout.jsonl")
	lines := strings.Join([]string{
		`{"type":"event_msg","payload":{"type":"task_started","turn_id":"turn-1"}}`,
		`{"type":"response_item","payload":{"type":"message","role":"assistant","id":"a1","content":[{"type":"output_text","text":"first answer"}]}}`,
		`{"type":"response_item","payload":`,
		`{"type":"response_item","payload":{"type":"message","role":"assistant","id":"a1-final","content":[{"type":"output_text","text":"first final"}]}}`,
		`{"type":"event_msg","payload":{"type":"task_complete","turn_id":"turn-1"}}`,
		`{"type":"event_msg","payload":{"type":"task_started","turn_id":"turn-2"}}`,
		`{"type":"response_item","payload":{"type":"message","role":"assistant","id":"a2","content":[{"type":"output_text","text":"final answer"}]}}`,
		`{"type":"event_msg","payload":{"type":"task_complete","turn_id":"turn-2"}}`,
	}, "\n")
	if err := os.WriteFile(transcript, []byte(lines), 0o600); err != nil {
		t.Fatal(err)
	}

	messages, report, err := ReconcileTranscriptAssistant(core.AgentCodex, transcript)
	if err != nil {
		t.Fatalf("reconcile: %v", err)
	}
	if len(messages) != 2 || messages[0].ExternalID != "turn-1" || messages[1].ExternalID != "turn-2" {
		t.Fatalf("unexpected messages: %+v", messages)
	}
	if messages[0].Content != "first final" {
		t.Fatalf("first turn content = %q, want final assistant message", messages[0].Content)
	}
	if report.Lines != 8 || report.Malformed != 1 || report.Assistant != 2 || report.LimitReached {
		t.Fatalf("unexpected report: %+v", report)
	}
}

func TestTranscriptReconciliationHasLineLimit(t *testing.T) {
	transcript := filepath.Join(t.TempDir(), "bounded.jsonl")
	content := strings.Repeat("{}\n", transcriptMaxLines+1)
	if err := os.WriteFile(transcript, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	_, report, err := ReconcileTranscriptAssistant(core.AgentCodex, transcript)
	if err != nil {
		t.Fatalf("reconcile: %v", err)
	}
	if !report.LimitReached || report.Lines != transcriptMaxLines {
		t.Fatalf("unexpected report: %+v", report)
	}
}
