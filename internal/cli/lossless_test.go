package cli

import (
	"bytes"
	"context"
	"path/filepath"
	"strings"
	"testing"
)

// runACM executes the root command with args and a fixed db, returning stdout.
func runACM(t *testing.T, dbPath, stdin string, args ...string) string {
	t.Helper()
	root := newRootCmd()
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&out)
	root.SetIn(strings.NewReader(stdin))
	root.SetArgs(append([]string{"--db", dbPath}, args...))
	if err := root.ExecuteContext(context.Background()); err != nil {
		t.Fatalf("acm %v: %v\noutput:\n%s", args, err, out.String())
	}
	return out.String()
}

func TestIngestThenGrepAndDescribe(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "acm.db")

	payload := `{
		"agent": "codex",
		"session_id": "sess-xyz",
		"title": "demo",
		"messages": [
			{"role": "user", "content": "add retry with exponential backoff", "external_id": "u1"},
			{"role": "assistant", "content": "implemented exponential backoff in client.go", "external_id": "a1"}
		]
	}`

	ingestOut := runACM(t, dbPath, payload, "ingest")
	if !strings.Contains(ingestOut, "appended 2") {
		t.Fatalf("ingest output = %q, want appended 2", ingestOut)
	}

	// FTS grep finds the assistant message.
	grepOut := runACM(t, dbPath, "", "grep", "backoff")
	if !strings.Contains(grepOut, "assistant") {
		t.Fatalf("grep output missing assistant hit:\n%s", grepOut)
	}

	// The hit line begins with the message ID; describe it.
	id := strings.Fields(grepOut)[0]
	descOut := runACM(t, dbPath, "", "describe", id)
	if !strings.Contains(descOut, "role:") || !strings.Contains(descOut, "backoff") {
		t.Fatalf("describe output unexpected:\n%s", descOut)
	}

	statsOut := runACM(t, dbPath, "", "stats")
	if !strings.Contains(statsOut, "messages:      2") {
		t.Fatalf("stats output = %q, want 2 messages", statsOut)
	}
}

func TestGrepNoMatches(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "acm.db")
	out := runACM(t, dbPath, "", "grep", "nonexistentterm")
	if !strings.Contains(out, "no matches") {
		t.Fatalf("expected no matches, got %q", out)
	}
}
