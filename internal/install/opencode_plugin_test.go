package install

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// fakeAcm records each invocation as "<args>\t<stdin>" and answers
// opencode-context with a fixed plan.
const fakeAcm = `#!/bin/sh
input=$(cat)
printf '%s\t%s\n' "$*" "$input" >> "$ACM_FAKE_LOG"
if [ "$1" = "opencode-context" ]; then cat "$ACM_FAKE_PLAN"; fi
`

const fakePlan = `{"version":1,"fresh_tail_messages":2,"fresh_message_ids":["msg_old_fresh"],` +
	`"summary_refs":["sum_1"],"archive_placeholder":"[Archived by acm: sum_1]","summary_text":"summary of old turns",` +
	`"recall_text":"<acm-recall>hit</acm-recall>","resume_note":"[Archived by acm: resume note]"}`

type replayCall struct {
	args  string
	input string
}

type replayOutput struct {
	Request struct {
		System   []map[string]any `json:"system"`
		Messages []struct {
			ID      string           `json:"id"`
			Role    string           `json:"role"`
			Content []map[string]any `json:"content"`
		} `json:"messages"`
	} `json:"request"`
	Compaction struct {
		System []map[string]any `json:"system"`
	} `json:"compaction"`
}

// runReplay executes the embedded plugin under Bun against the recorded events
// with a fake acm on PATH, returning the recorded acm calls and hook output.
func runReplay(t *testing.T) ([]replayCall, replayOutput) {
	t.Helper()
	bun, err := exec.LookPath("bun")
	if err != nil {
		t.Skip("bun not on PATH; skipping OpenCode plugin replay")
	}
	tmp := t.TempDir()
	bin, project := filepath.Join(tmp, "bin"), filepath.Join(tmp, "project")
	for _, dir := range []string{bin, project} {
		if mkErr := os.MkdirAll(dir, 0o755); mkErr != nil {
			t.Fatal(mkErr)
		}
	}
	pluginPath, logPath, planPath := filepath.Join(tmp, "server.ts"), filepath.Join(tmp, "calls.log"), filepath.Join(tmp, "plan.json")
	writeTestFile(t, filepath.Join(bin, "acm"), fakeAcm, 0o755)
	writeTestFile(t, pluginPath, opencodePluginSrc, 0o644)
	writeTestFile(t, planPath, fakePlan, 0o644)
	harness, err := filepath.Abs(filepath.Join("testdata", "opencode_replay.ts"))
	if err != nil {
		t.Fatal(err)
	}
	events, err := filepath.Abs(filepath.Join("testdata", "opencode-events.jsonl"))
	if err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, bun, harness, pluginPath, events, project)
	cmd.Env = append(os.Environ(), "PATH="+bin+string(os.PathListSeparator)+os.Getenv("PATH"),
		"ACM_FAKE_LOG="+logPath, "ACM_FAKE_PLAN="+planPath)
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("replay failed: %v\n%s", err, out)
	}
	var result replayOutput
	if err := json.Unmarshal(out, &result); err != nil {
		t.Fatalf("decode replay output: %v\n%s", err, out)
	}
	return readCalls(t, logPath), result
}

func writeTestFile(t *testing.T, path, content string, mode os.FileMode) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), mode); err != nil {
		t.Fatal(err)
	}
}

func readCalls(t *testing.T, path string) []replayCall {
	t.Helper()
	text := strings.TrimSpace(readFile(t, path))
	calls := make([]replayCall, 0, strings.Count(text, "\n")+1)
	for line := range strings.SplitSeq(text, "\n") {
		args, input, _ := strings.Cut(line, "\t")
		calls = append(calls, replayCall{args: args, input: input})
	}
	return calls
}

func ingestedMessage(t *testing.T, call replayCall) map[string]any {
	t.Helper()
	var payload struct {
		SessionID string           `json:"session_id"`
		Messages  []map[string]any `json:"messages"`
	}
	if err := json.Unmarshal([]byte(call.input), &payload); err != nil || len(payload.Messages) != 1 {
		t.Fatalf("bad ingest payload %q: %v", call.input, err)
	}
	if payload.SessionID != "ses_test" {
		t.Fatalf("ingest routed to session %q, want ses_test", payload.SessionID)
	}
	return payload.Messages[0]
}

func TestOpencodePluginCapturesV2Events(t *testing.T) {
	calls, _ := runReplay(t)
	want := []struct{ args, role, id, content string }{
		{args: "doctor"},
		{"ingest", "user", "msg_user1", "Use the bash tool to run: echo acm-probe-ok"},
		{"ingest", "tool", "toolu_1", "[tool shell]\ninput: {\"command\":\"echo acm-probe-ok\"}\noutput: acm-probe-ok\n"},
		{"ingest", "assistant", "msg_step2", "done\nsecond block"},
		{"ingest", "tool", "toolu_2", "[tool read]\ninput: {}\noutput: file not found"},
		// The context hook ingests its anchor before planning; the acmContext
		// part already on the anchor is not part of the captured prompt.
		{"ingest", "user", "msg_new", "new question"},
		{args: "opencode-context"},
	}
	if len(calls) != len(want) {
		t.Fatalf("got %d acm calls, want %d: %+v", len(calls), len(want), calls)
	}
	for i, w := range want {
		if calls[i].args != w.args {
			t.Fatalf("call %d: args %q, want %q", i, calls[i].args, w.args)
		}
		if w.role == "" {
			continue
		}
		msg := ingestedMessage(t, calls[i])
		if msg["role"] != w.role || msg["external_id"] != w.id || msg["content"] != w.content {
			t.Fatalf("call %d: got %v, want role=%s id=%s content=%q", i, msg, w.role, w.id, w.content)
		}
	}
}

func TestOpencodePluginAppliesContextPlan(t *testing.T) {
	_, out := runReplay(t)
	msgs := out.Request.Messages
	if len(msgs) != 6 {
		t.Fatalf("got %d messages, want 6", len(msgs))
	}
	const placeholder = "[Archived by acm: sum_1]"
	assertParts(t, "archived user", msgs[0].Content, `[{"text":"`+placeholder+`","type":"text"}]`)
	assertParts(t, "fresh assistant", msgs[1].Content, `[{"text":"kept verbatim","type":"text"}]`)
	assertParts(t, "archived tool call", msgs[2].Content, `[{"id":"call_1","input":{"path":"a.txt"},"name":"read","type":"tool-call"}]`)
	assertParts(t, "archived tool result", msgs[3].Content,
		`[{"id":"call_1","name":"read","result":{"type":"text","value":"`+placeholder+`"},"type":"tool-result"}]`)
	assertParts(t, "recent assistant", msgs[4].Content, `[{"text":"recent answer","type":"text"}]`)
	assertParts(t, "anchor", msgs[5].Content, `[{"text":"new question","type":"text"},`+
		`{"metadata":{"acmContext":"retrieved-context"},"text":"<acm-recall>hit</acm-recall>","type":"text"},`+
		`{"metadata":{"acmContext":"archive-summary"},"text":"summary of old turns","type":"text"}]`)
	if len(out.Request.System) != 2 {
		t.Fatalf("system hint not added exactly once: %v", out.Request.System)
	}
	if hint, ok := out.Request.System[1]["text"].(string); !ok || !strings.HasPrefix(hint, "[Archived by acm:") {
		t.Fatalf("system hint missing: %v", out.Request.System)
	}
	if len(out.Compaction.System) != 1 || out.Compaction.System[0]["text"] != "[Archived by acm: resume note]" {
		t.Fatalf("compaction resume note missing: %v", out.Compaction.System)
	}
}

func assertParts(t *testing.T, label string, parts []map[string]any, want string) {
	t.Helper()
	var buf strings.Builder
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(parts); err != nil {
		t.Fatal(err)
	}
	if got := strings.TrimSpace(buf.String()); got != want {
		t.Fatalf("%s:\n got  %s\n want %s", label, got, want)
	}
}
