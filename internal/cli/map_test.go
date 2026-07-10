package cli

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestMapPassthroughCommand(t *testing.T) {
	dir := t.TempDir()
	in := filepath.Join(dir, "in.jsonl")
	out := filepath.Join(dir, "out.jsonl")
	if err := os.WriteFile(in, []byte("{\"id\":1}\n{\"id\":2}\n"), 0o600); err != nil {
		t.Fatalf("write input: %v", err)
	}

	dbPath := filepath.Join(dir, "acm.db")
	stdout := runACM(t, dbPath, "", "map", "--input", in, "--output", out, "--processor", "passthrough")
	if !strings.Contains(stdout, "2 ok") {
		t.Fatalf("map output = %q, want '2 ok'", stdout)
	}

	f, err := os.Open(out)
	if err != nil {
		t.Fatalf("open output: %v", err)
	}
	defer func() { _ = f.Close() }()
	lines := 0
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		var o struct {
			Index int             `json:"index"`
			OK    bool            `json:"ok"`
			Out   json.RawMessage `json:"output"`
		}
		if err := json.Unmarshal(sc.Bytes(), &o); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if !o.OK {
			t.Fatalf("line %d not ok", lines)
		}
		lines++
	}
	if lines != 2 {
		t.Fatalf("expected 2 output lines, got %d", lines)
	}
}

func TestMapCodexAgentCommandWithSchema(t *testing.T) {
	dir := t.TempDir()
	binDir := filepath.Join(dir, "bin")
	if err := os.Mkdir(binDir, 0o700); err != nil {
		t.Fatalf("create bin: %v", err)
	}
	fakeCodex := `#!/bin/sh
case " $* " in *"--sandbox read-only"*"--output-schema"*) ;; *) exit 70 ;; esac
cat >/dev/null
printf '%s\n' '{"type":"turn.started"}'
printf '%s\n' '{"type":"item.completed","item":{"type":"agent_message","text":"{\"label\":\"ok\"}"}}'
printf '%s\n' '{"type":"turn.completed"}'
`
	if err := os.WriteFile(filepath.Join(binDir, "codex"), []byte(fakeCodex), 0o700); err != nil {
		t.Fatalf("write fake Codex: %v", err)
	}
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))
	input := filepath.Join(dir, "in.jsonl")
	output := filepath.Join(dir, "out.jsonl")
	schema := filepath.Join(dir, "schema.json")
	if err := os.WriteFile(input, []byte("{\"id\":1}\n"), 0o600); err != nil {
		t.Fatalf("write input: %v", err)
	}
	if err := os.WriteFile(schema, []byte(`{"type":"object","properties":{"label":{"const":"ok"}},"required":["label"]}`), 0o600); err != nil {
		t.Fatalf("write schema: %v", err)
	}
	stdout := runACM(t, filepath.Join(dir, "acm.db"), "", "map", "--input", input, "--output", output,
		"--processor", "codex-agent", "--schema", schema, "--concurrency", "1")
	if !strings.Contains(stdout, "1 ok") {
		t.Fatalf("map output = %q", stdout)
	}
	info, err := os.Stat(output)
	if err != nil {
		t.Fatalf("stat output: %v", err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("output mode = %o, want 600", info.Mode().Perm())
	}
}

func TestAgenticMapRejectsInvalidLimitsBeforeExecution(t *testing.T) {
	options := &mapOptions{
		input: "in.jsonl", output: "out.jsonl", processor: "claude-agent",
		maxTools: 101, maxTurns: 1, itemTimeout: time.Second,
	}
	if _, err := options.mapper(); err == nil || !strings.Contains(err.Error(), "max tools") {
		t.Fatalf("invalid agent limit error = %v", err)
	}
}

func TestAgenticMapZeroBudgetsResolveToFiniteDefaults(t *testing.T) {
	options := &mapOptions{processor: "codex-agent"}
	if calls := options.effectiveMaxCalls(); calls != defaultAgentMaxCalls {
		t.Fatalf("agent max calls = %d, want %d", calls, defaultAgentMaxCalls)
	}
	if timeout := options.effectiveRunTimeout(); timeout != defaultAgentRunTimeout {
		t.Fatalf("agent run timeout = %s, want %s", timeout, defaultAgentRunTimeout)
	}
	options.processor = "codex"
	if calls := options.effectiveMaxCalls(); calls != 0 {
		t.Fatalf("single-response max calls = %d, want disabled", calls)
	}
	if timeout := options.effectiveRunTimeout(); timeout != 0 {
		t.Fatalf("single-response run timeout = %s, want disabled", timeout)
	}
}
