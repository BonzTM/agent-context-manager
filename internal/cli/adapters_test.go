package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestHookCapturesAndInjectsRecall(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "acm.db")

	// Seed prior history with an assistant message worth recalling.
	prior := `{"agent":"claude-code","session_id":"prior","messages":[
		{"role":"assistant","content":"use exponential backoff in client.go","external_id":"a1"}
	]}`
	runACM(t, dbPath, prior, "ingest")

	// A new prompt mentioning "backoff" should pull the prior message back via
	// recall (OR match), and the prompt itself should be captured.
	payload := `{"session_id":"cur","prompt":"how do I add backoff retries"}`
	out := runACM(t, dbPath, payload, "hook", "--agent", "claude-code", "--event", "UserPromptSubmit")
	if !strings.Contains(out, "hookSpecificOutput") {
		t.Fatalf("hook output missing hookSpecificOutput:\n%s", out)
	}
	if !strings.Contains(out, "acm-recall") || !strings.Contains(out, "backoff") {
		t.Fatalf("hook output missing recalled context:\n%s", out)
	}

	// The prompt was captured: prior assistant + current user prompt = 2 messages.
	stats := runACM(t, dbPath, "", "stats")
	if !strings.Contains(stats, "messages:      2") {
		t.Fatalf("stats after hook = %q, want 2 messages", stats)
	}
}

func TestHookPostToolUseCapturesWithoutRecall(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "acm.db")
	payload := `{"session_id":"s","tool_name":"Bash","tool_input":{"command":"go test"},"tool_response":{"stdout":"ok"}}`
	out := runACM(t, dbPath, payload, "hook", "--agent", "claude-code", "--event", "PostToolUse")
	// PostToolUse is capture-only: no recall output.
	if strings.Contains(out, "hookSpecificOutput") {
		t.Fatalf("PostToolUse should not inject recall:\n%s", out)
	}
	stats := runACM(t, dbPath, "", "stats")
	if !strings.Contains(stats, "messages:      1") {
		t.Fatalf("stats = %q, want 1 captured tool message", stats)
	}
}

func TestHookCapturesCodexNotifyArgument(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "acm.db")
	payload := `{"type":"agent-turn-complete","thread-id":"thread-1","turn-id":"turn-1","input-messages":["test prompt"],"last-assistant-message":"test response"}`
	runACM(t, dbPath, "", "hook", "--agent", "codex", "--event", "agent-turn-complete", payload)

	stats := runACM(t, dbPath, "", "stats")
	if !strings.Contains(stats, "messages:      2") {
		t.Fatalf("stats after Codex notify = %q, want user and assistant messages", stats)
	}
}

func TestInitWritesAssets(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "acm.db")
	out := runACM(t, dbPath, "", "init", "claude-code")
	if !strings.Contains(out, "wrote") {
		t.Fatalf("init output = %q, want 'wrote'", out)
	}
	settings := filepath.Join(dir, ".acm", "init", "claude-code", "settings.snippet.json")
	if _, err := os.Stat(settings); err != nil {
		t.Fatalf("expected settings snippet at %s: %v", settings, err)
	}
}

func TestHookStopCapturesTranscriptAndCompacts(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "acm.db")

	// Build a transcript with enough assistant text to exceed a default-config
	// no-op (content volume is irrelevant to the assertion; compaction is a
	// cheap no-op below threshold and must not error either way).
	transcript := filepath.Join(dir, "session.jsonl")
	lines := `{"type":"assistant","uuid":"a1","message":{"role":"assistant","content":[{"type":"text","text":"first answer"}]}}
{"type":"assistant","uuid":"a2","message":{"role":"assistant","content":[{"type":"text","text":"second answer"}]}}`
	if err := os.WriteFile(transcript, []byte(lines), 0o600); err != nil {
		t.Fatal(err)
	}

	payload := fmt.Sprintf(`{"session_id":"s1","transcript_path":%q}`, transcript)
	runACM(t, dbPath, payload, "hook", "--agent", "claude-code", "--event", "Stop")

	stats := runACM(t, dbPath, "", "stats")
	if !strings.Contains(stats, "messages:      2") {
		t.Fatalf("stats after Stop hook = %q, want 2 assistant messages", stats)
	}

	// Re-running the same Stop hook must not duplicate (transcript re-read).
	runACM(t, dbPath, payload, "hook", "--agent", "claude-code", "--event", "Stop")
	stats = runACM(t, dbPath, "", "stats")
	if !strings.Contains(stats, "messages:      2") {
		t.Fatalf("stats after repeated Stop hook = %q, want still 2", stats)
	}
}

func TestCodexBackfillIsDryRunFirstAndIdempotent(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "acm.db")
	transcript := filepath.Join(dir, "rollout.jsonl")
	lines := `{"type":"event_msg","payload":{"type":"task_started","turn_id":"turn-1"}}
{"type":"response_item","payload":{"type":"message","role":"assistant","id":"response-1","content":[{"type":"output_text","text":"recovered answer"}]}}
{"type":"response_item","payload":
{"type":"event_msg","payload":{"type":"task_complete","turn_id":"turn-1"}}`
	if err := os.WriteFile(transcript, []byte(lines), 0o600); err != nil {
		t.Fatal(err)
	}
	rawHook := fmt.Sprintf(`{"session_id":"codex-session","transcript_path":%q}`, transcript)
	payload := fmt.Sprintf(`{"agent":"codex","session_id":"codex-session","messages":[{"role":"user","content":"recover this","external_id":"turn-1:input:0","raw":%q}]}`, rawHook)
	runACM(t, dbPath, payload, "ingest")

	dryRun := runACM(t, dbPath, "", "backfill")
	if !strings.Contains(dryRun, "missing=1 appended=0 malformed=1 mode=dry-run") {
		t.Fatalf("dry-run output = %q", dryRun)
	}
	assertMessageCount(t, dbPath, 1)
	apply := runACM(t, dbPath, "", "backfill", "--apply")
	if !strings.Contains(apply, "missing=1 appended=1 malformed=1 mode=apply") {
		t.Fatalf("apply output = %q", apply)
	}
	assertMessageCount(t, dbPath, 2)
	second := runACM(t, dbPath, "", "backfill", "--apply")
	if !strings.Contains(second, "missing=0 appended=0 malformed=1 mode=apply") {
		t.Fatalf("second apply output = %q", second)
	}
}

func assertMessageCount(t *testing.T, dbPath string, count int) {
	t.Helper()
	stats := runACM(t, dbPath, "", "stats")
	want := fmt.Sprintf("messages:      %d", count)
	if !strings.Contains(stats, want) {
		t.Fatalf("stats = %q, want %q", stats, want)
	}
}
