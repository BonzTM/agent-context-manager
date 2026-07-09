package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
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

func TestBackupCreatesPrivateSnapshot(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "acm.db")
	dest := filepath.Join(dir, "backup.db")
	runACM(t, dbPath, `{"agent":"codex","session_id":"backup","messages":[]}`, "ingest")
	runACM(t, dbPath, "", "backup", dest)

	info, err := os.Stat(dest)
	if err != nil {
		t.Fatalf("stat backup: %v", err)
	}
	if got := info.Mode().Perm(); got != 0o600 {
		t.Fatalf("backup mode = %o, want 600", got)
	}
}

func TestExpandQuerySynthesizeFallsBackWithoutAgentCLI(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "acm.db")

	payload := `{"agent":"codex","session_id":"syn","messages":[
		{"role":"user","content":"we picked sqlite for zero infrastructure ` + strings.Repeat("architecture ", 100) + `","external_id":"s1"},
		{"role":"assistant","content":"agreed, sqlite it is ` + strings.Repeat("decision ", 100) + `","external_id":"s2"}]}`
	runACM(t, dbPath, payload, "ingest")

	// Build a leaf summary so there is a sum_ id to expand.
	grepOut := runACM(t, dbPath, "", "grep", "--json", "sqlite")
	var res struct {
		Messages []struct {
			Message struct{ ConversationID string }
		}
	}
	if err := json.Unmarshal([]byte(grepOut), &res); err != nil || len(res.Messages) == 0 {
		t.Fatalf("grep json: %v (%s)", err, grepOut)
	}
	conv := res.Messages[0].Message.ConversationID
	runACM(t, dbPath, "", "compact", conv,
		"--model-context-tokens", "1000", "--soft-fraction", "0.1", "--fresh-tail", "0", "--fresh-tail-tokens", "0",
		"--leaf-chunk-tokens", "300", "--leaf-target-tokens", "40", "--condensed-target-tokens", "40",
		"--condense-chunk-tokens", "300", "--truncate-tokens", "20")
	sumOut := runACM(t, dbPath, "", "grep", "sqlite")
	sumID := ""
	for f := range strings.FieldsSeq(sumOut) {
		if strings.HasPrefix(f, "sum_") {
			sumID = f
			break
		}
	}
	if sumID == "" {
		t.Fatalf("no summary produced:\n%s", sumOut)
	}

	// With no agent CLI on PATH, --synthesize must degrade to the filtered
	// message output instead of failing.
	t.Setenv("PATH", t.TempDir())
	out := runACM(t, dbPath, "", "expand-query", sumID, "sqlite", "--synthesize")
	if !strings.Contains(out, "message msg_") || !strings.Contains(out, "sqlite") {
		t.Fatalf("fallback output missing filtered messages:\n%s", out)
	}
}
