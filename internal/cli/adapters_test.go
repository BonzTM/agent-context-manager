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
