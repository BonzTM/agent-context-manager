package llmmap

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
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
	if err := sc.Err(); err != nil {
		t.Fatalf("scan output: %v", err)
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
	calls := 0
	m := &Mapper{Process: func(_ context.Context, item json.RawMessage, _ int, _ string) (json.RawMessage, error) {
		calls++
		return item, nil
	}}
	if _, err := m.Run(context.Background(), in, out); err == nil {
		t.Fatal("expected error for invalid input JSON")
	}
	if calls != 0 {
		t.Fatalf("processor called %d times before full input validation", calls)
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

func TestMapperRejectsExcessiveLimitsBeforeProcessorWork(t *testing.T) {
	in := writeInput(t, []string{`{"id":1}`, `{"id":2}`})
	tests := []struct {
		name   string
		mapper Mapper
	}{
		{name: "concurrency", mapper: Mapper{Concurrency: hardConcurrency + 1}},
		{name: "attempts", mapper: Mapper{MaxRetries: hardAttempts}},
		{name: "negative retries", mapper: Mapper{MaxRetries: -1}},
		{name: "input bytes", mapper: Mapper{MaxInputBytes: hardInputBytes + 1}},
		{name: "item bytes", mapper: Mapper{MaxItemBytes: hardItemBytes + 1}},
		{name: "item count", mapper: Mapper{MaxItems: hardMaxItems + 1}},
		{name: "run timeout", mapper: Mapper{RunTimeout: hardRunTimeout + time.Second}},
		{name: "call budget", mapper: Mapper{MaxCalls: 1}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			calls := 0
			test.mapper.Process = func(_ context.Context, item json.RawMessage, _ int, _ string) (json.RawMessage, error) {
				calls++
				return item, nil
			}
			out := filepath.Join(t.TempDir(), "out.jsonl")
			if _, err := test.mapper.Run(context.Background(), in, out); err == nil {
				t.Fatal("expected limit validation error")
			}
			if calls != 0 {
				t.Fatalf("processor called %d times before limit validation", calls)
			}
		})
	}
}

func TestMapperRejectsOversizedDatasetBeforeProcessorWork(t *testing.T) {
	in := writeInput(t, []string{`{"id":1}`, `{"id":2}`})
	out := filepath.Join(t.TempDir(), "out.jsonl")
	calls := 0
	mapper := &Mapper{MaxInputBytes: 8, Process: func(_ context.Context, item json.RawMessage, _ int, _ string) (json.RawMessage, error) {
		calls++
		return item, nil
	}}
	if _, err := mapper.Run(context.Background(), in, out); err == nil || !strings.Contains(err.Error(), "input exceeds") {
		t.Fatalf("oversized input error = %v", err)
	}
	if calls != 0 {
		t.Fatalf("processor called %d times before dataset size validation", calls)
	}
}

func TestMapperPeakInFlightIsIndependentOfItemCount(t *testing.T) {
	lines := make([]string, 200)
	for i := range lines {
		lines[i] = fmt.Sprintf(`{"id":%d}`, i)
	}
	in := writeInput(t, lines)
	out := filepath.Join(t.TempDir(), "out.jsonl")
	mapper := &Mapper{Concurrency: 3, Process: func(_ context.Context, item json.RawMessage, _ int, _ string) (json.RawMessage, error) {
		return item, nil
	}}
	result, err := mapper.Run(context.Background(), in, out)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if result.Total != len(lines) {
		t.Fatalf("total = %d, want %d", result.Total, len(lines))
	}
	if result.PeakInFlight < 1 || result.PeakInFlight > 7 {
		t.Fatalf("peak in flight = %d, want 1..7 for workers, queue, and producer", result.PeakInFlight)
	}
}

func TestMapperCancellationPreservesStateAndResumeSkipsCompleted(t *testing.T) {
	in := writeInput(t, []string{`{"id":0}`, `{"id":1}`, `{"id":2}`, `{"id":3}`})
	dir := t.TempDir()
	out := filepath.Join(dir, "out.jsonl")
	if err := os.WriteFile(out, []byte("existing complete output\n"), 0o600); err != nil {
		t.Fatalf("write existing output: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	first := &Mapper{Concurrency: 1, MaxRetries: 2, ConfigKey: "stable", Process: func(ctx context.Context, item json.RawMessage, _ int, _ string) (json.RawMessage, error) {
		var value struct {
			ID int `json:"id"`
		}
		if err := json.Unmarshal(item, &value); err != nil {
			return nil, err
		}
		if value.ID == 1 {
			cancel()
			<-ctx.Done()
			return nil, ctx.Err()
		}
		return item, nil
	}}
	if _, err := first.Run(ctx, in, out); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled run error = %v, want context.Canceled", err)
	}
	content, err := os.ReadFile(out)
	if err != nil {
		t.Fatalf("read existing output: %v", err)
	}
	if string(content) != "existing complete output\n" {
		t.Fatalf("cancelled run changed complete output: %q", content)
	}
	stateDir := out + ".acm-map-state"
	if _, statErr := os.Stat(stateDir); statErr != nil {
		t.Fatalf("resume state missing after cancellation: %v", statErr)
	}
	stateInfo, err := os.Stat(stateDir)
	if err != nil {
		t.Fatalf("stat resume state: %v", err)
	}
	if stateInfo.Mode().Perm() != 0o700 {
		t.Fatalf("resume state mode = %o, want 700", stateInfo.Mode().Perm())
	}
	snapshotInfo, err := os.Stat(filepath.Join(stateDir, "input.jsonl"))
	if err != nil {
		t.Fatalf("stat input snapshot: %v", err)
	}
	if snapshotInfo.Mode().Perm() != 0o600 {
		t.Fatalf("input snapshot mode = %o, want 600", snapshotInfo.Mode().Perm())
	}

	var resumed []int
	second := &Mapper{Concurrency: 1, MaxRetries: 2, ConfigKey: "stable", Process: func(_ context.Context, item json.RawMessage, _ int, _ string) (json.RawMessage, error) {
		var value struct {
			ID int `json:"id"`
		}
		if decodeErr := json.Unmarshal(item, &value); decodeErr != nil {
			return nil, decodeErr
		}
		resumed = append(resumed, value.ID)
		return item, nil
	}}
	result, err := second.Run(context.Background(), in, out)
	if err != nil {
		t.Fatalf("resume: %v", err)
	}
	if result.Succeeded != 4 || slices.Contains(resumed, 0) {
		t.Fatalf("resume result = %+v, processed IDs = %v", result, resumed)
	}
	outputs := readOutputs(t, out)
	for index, output := range outputs {
		if output.Index != index || !output.OK {
			t.Fatalf("output %d = %+v", index, output)
		}
	}
	if _, err := os.Stat(stateDir); !os.IsNotExist(err) {
		t.Fatalf("completed resume state still exists: %v", err)
	}
}

func TestMapperResumeRejectsChangedConfigurationBeforeWork(t *testing.T) {
	in := writeInput(t, []string{`{"id":0}`, `{"id":1}`})
	out := filepath.Join(t.TempDir(), "out.jsonl")
	ctx, cancel := context.WithCancel(context.Background())
	first := &Mapper{Concurrency: 1, ConfigKey: "first", Process: func(ctx context.Context, item json.RawMessage, _ int, _ string) (json.RawMessage, error) {
		cancel()
		<-ctx.Done()
		return nil, ctx.Err()
	}}
	if _, err := first.Run(ctx, in, out); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled run error = %v", err)
	}
	calls := 0
	second := &Mapper{ConfigKey: "changed", Process: func(_ context.Context, item json.RawMessage, _ int, _ string) (json.RawMessage, error) {
		calls++
		return item, nil
	}}
	if _, err := second.Run(context.Background(), in, out); err == nil || !strings.Contains(err.Error(), "does not match") {
		t.Fatalf("resume mismatch error = %v", err)
	}
	if calls != 0 {
		t.Fatalf("processor called %d times for mismatched resume", calls)
	}
}
