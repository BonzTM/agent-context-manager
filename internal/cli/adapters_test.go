package cli

import (
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
