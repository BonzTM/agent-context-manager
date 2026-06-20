// Package llmmap implements the LCM paper's operator-level parallelism:
// llm_map / agentic_map. A dataset is read from a JSONL file on disk, each item
// is processed independently by a bounded worker pool, outputs are validated and
// retried, and results are written back to a JSONL file — all entirely
// off-context, so the agent can process datasets of arbitrary size without the
// input or output ever entering its window. The paper credits this mechanism
// for its long-context win over plan-your-own-chunking approaches.
package llmmap

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sync"
)

// Processor handles a single item. attempt starts at 1 and increases on retry,
// so a Processor may vary its prompt with attempt (e.g. feed back the prior
// validation error). It returns the item's output as raw JSON.
type Processor func(ctx context.Context, item json.RawMessage, attempt int) (json.RawMessage, error)

// Validator checks an output. A nil return means valid; a non-nil error triggers
// a retry (up to MaxRetries). A nil Validator accepts any valid JSON.
type Validator func(json.RawMessage) error

// Mapper runs a Processor over a JSONL dataset with bounded concurrency and
// validated retries.
type Mapper struct {
	Concurrency int
	MaxRetries  int
	Process     Processor
	Validate    Validator
}

// Output is one result line written to the output JSONL file.
type Output struct {
	Index    int             `json:"index"`
	OK       bool            `json:"ok"`
	Output   json.RawMessage `json:"output,omitempty"`
	Error    string          `json:"error,omitempty"`
	Attempts int             `json:"attempts"`
}

// Result summarizes a run.
type Result struct {
	Total     int
	Succeeded int
	Failed    int
}

// Run processes every JSONL line of inputPath and writes a result line per item
// to outputPath, preserving input order. The worker pool size is Concurrency
// (>=1). Item processing never enters the caller's context; only the Result
// counts are returned.
func (m *Mapper) Run(ctx context.Context, inputPath, outputPath string) (Result, error) {
	items, err := readItems(inputPath)
	if err != nil {
		return Result{}, err
	}

	concurrency := max(m.Concurrency, 1)

	outputs := make([]Output, len(items))
	sem := make(chan struct{}, concurrency)
	var wg sync.WaitGroup

	for i, raw := range items {
		wg.Add(1)
		go func(idx int, item json.RawMessage) {
			defer wg.Done()
			select {
			case sem <- struct{}{}:
				defer func() { <-sem }()
			case <-ctx.Done():
				outputs[idx] = Output{Index: idx, OK: false, Error: ctx.Err().Error()}
				return
			}
			outputs[idx] = m.processOne(ctx, idx, item)
		}(i, raw)
	}
	wg.Wait()

	if err := writeOutputs(outputPath, outputs); err != nil {
		return Result{}, err
	}

	res := Result{Total: len(outputs)}
	for _, o := range outputs {
		if o.OK {
			res.Succeeded++
		} else {
			res.Failed++
		}
	}
	return res, nil
}

func (m *Mapper) processOne(ctx context.Context, idx int, item json.RawMessage) Output {
	attempts := max(m.MaxRetries+1, 1)
	var lastErr error
	for attempt := 1; attempt <= attempts; attempt++ {
		if ctx.Err() != nil {
			return Output{Index: idx, OK: false, Error: ctx.Err().Error(), Attempts: attempt - 1}
		}
		out, err := m.Process(ctx, item, attempt)
		if err != nil {
			lastErr = err
			continue
		}
		if m.Validate != nil {
			if vErr := m.Validate(out); vErr != nil {
				lastErr = fmt.Errorf("validation: %w", vErr)
				continue
			}
		}
		return Output{Index: idx, OK: true, Output: out, Attempts: attempt}
	}
	msg := "exhausted retries"
	if lastErr != nil {
		msg = lastErr.Error()
	}
	return Output{Index: idx, OK: false, Error: msg, Attempts: attempts}
}

// RequireFields returns a Validator that checks the output is a JSON object
// containing every named key — a light schema sufficient for most map outputs
// (the agent CLIs can additionally enforce a full JSON Schema at the model layer).
func RequireFields(fields ...string) Validator {
	return func(out json.RawMessage) error {
		var obj map[string]json.RawMessage
		if err := json.Unmarshal(out, &obj); err != nil {
			return fmt.Errorf("output is not a JSON object: %w", err)
		}
		for _, f := range fields {
			if _, ok := obj[f]; !ok {
				return fmt.Errorf("missing required field %q", f)
			}
		}
		return nil
	}
}

func readItems(path string) ([]json.RawMessage, error) {
	f, err := os.Open(path) //nolint:gosec // G304: input path is a user-supplied CLI argument; reading it is the command's purpose.
	if err != nil {
		return nil, fmt.Errorf("llmmap: open input: %w", err)
	}
	defer func() { _ = f.Close() }()

	var items []json.RawMessage
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 8*1024*1024) // allow large lines
	line := 0
	for sc.Scan() {
		line++
		raw := sc.Bytes()
		if len(trimSpace(raw)) == 0 {
			continue
		}
		var probe json.RawMessage
		if err := json.Unmarshal(raw, &probe); err != nil {
			return nil, fmt.Errorf("llmmap: input line %d is not valid JSON: %w", line, err)
		}
		items = append(items, append(json.RawMessage(nil), probe...))
	}
	if err := sc.Err(); err != nil {
		return nil, fmt.Errorf("llmmap: read input: %w", err)
	}
	return items, nil
}

func writeOutputs(path string, outputs []Output) error {
	f, err := os.Create(path) //nolint:gosec // G304: output path is a user-supplied CLI argument.
	if err != nil {
		return fmt.Errorf("llmmap: create output: %w", err)
	}
	defer func() { _ = f.Close() }()

	w := bufio.NewWriter(f)
	enc := json.NewEncoder(w)
	for _, o := range outputs {
		if err := enc.Encode(o); err != nil {
			return fmt.Errorf("llmmap: encode output: %w", err)
		}
	}
	if err := w.Flush(); err != nil {
		return fmt.Errorf("llmmap: flush output: %w", err)
	}
	return nil
}

func trimSpace(b []byte) []byte {
	start, end := 0, len(b)
	for start < end && isSpace(b[start]) {
		start++
	}
	for end > start && isSpace(b[end-1]) {
		end--
	}
	return b[start:end]
}

func isSpace(c byte) bool {
	return c == ' ' || c == '\t' || c == '\r' || c == '\n'
}
