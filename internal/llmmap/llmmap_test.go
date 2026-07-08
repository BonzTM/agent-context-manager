package llmmap

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeInput(t *testing.T, lines []string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "in.jsonl")
	content := strings.Join(lines, "\n") + "\n"
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write input: %v", err)
	}
	return path
}

func readOutputs(t *testing.T, path string) []Output {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("open output: %v", err)
	}
	defer func() { _ = f.Close() }()
	var out []Output
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		var o Output
		if err := json.Unmarshal(sc.Bytes(), &o); err != nil {
			t.Fatalf("decode output: %v", err)
		}
		out = append(out, o)
	}
	return out
}

func TestMapperOrderRetryAndValidation(t *testing.T) {
	in := writeInput(t, []string{`{"id":1}`, `{"id":2}`, `{"id":3}`, `{"id":99}`})
	outPath := filepath.Join(t.TempDir(), "out.jsonl")

	var feedbackSeen string
	proc := func(_ context.Context, item json.RawMessage, attempt int, feedback string) (json.RawMessage, error) {
		var v struct {
			ID int `json:"id"`
		}
		if err := json.Unmarshal(item, &v); err != nil {
			return nil, err
		}
		if v.ID == 99 {
			return nil, errors.New("permanent failure")
		}
		if v.ID == 2 && attempt == 1 {
			return json.RawMessage(`{"id":2}`), nil // missing "out" -> fails validation, retried
		}
		if v.ID == 2 && attempt == 2 {
			feedbackSeen = feedback // the retry must carry the validation error
		}
		return json.RawMessage(fmt.Sprintf(`{"id":%d,"out":"ok"}`, v.ID)), nil
	}

	m := &Mapper{Concurrency: 2, MaxRetries: 2, Process: proc, Validate: RequireFields("out")}
	res, err := m.Run(context.Background(), in, outPath)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if res.Total != 4 || res.Succeeded != 3 || res.Failed != 1 {
		t.Fatalf("result = %+v, want 4/3/1", res)
	}

	outs := readOutputs(t, outPath)
	if len(outs) != 4 {
		t.Fatalf("expected 4 output lines, got %d", len(outs))
	}
	// Order preserved by index.
	for i, o := range outs {
		if o.Index != i {
			t.Fatalf("output %d has index %d (order not preserved)", i, o.Index)
		}
	}
	if !outs[1].OK || outs[1].Attempts != 2 {
		t.Fatalf("item 2 should succeed on attempt 2: %+v", outs[1])
	}
	if !strings.Contains(feedbackSeen, "missing required field") {
		t.Fatalf("retry did not receive validation feedback, got %q", feedbackSeen)
	}
	if outs[3].OK || outs[3].Attempts != 3 {
		t.Fatalf("item 99 should fail after 3 attempts: %+v", outs[3])
	}
}

func TestMapperRejectsInvalidInputJSON(t *testing.T) {
	in := writeInput(t, []string{`{"ok":1}`, `not json`})
	out := filepath.Join(t.TempDir(), "out.jsonl")
	m := &Mapper{Process: func(_ context.Context, item json.RawMessage, _ int, _ string) (json.RawMessage, error) {
		return item, nil
	}}
	if _, err := m.Run(context.Background(), in, out); err == nil {
		t.Fatal("expected error for invalid input JSON")
	}
}

func TestMapperPassthrough(t *testing.T) {
	in := writeInput(t, []string{`{"a":1}`, `{"b":2}`})
	out := filepath.Join(t.TempDir(), "out.jsonl")
	m := &Mapper{Concurrency: 4, Process: func(_ context.Context, item json.RawMessage, _ int, _ string) (json.RawMessage, error) {
		return item, nil
	}}
	res, err := m.Run(context.Background(), in, out)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if res.Succeeded != 2 {
		t.Fatalf("expected 2 succeeded, got %+v", res)
	}
}
