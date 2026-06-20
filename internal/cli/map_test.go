package cli

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
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

	f, err := os.Open(out) //nolint:gosec // test-controlled path
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
