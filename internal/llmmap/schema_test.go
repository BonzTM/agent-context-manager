package llmmap

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeSchema(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "output.schema.json")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write schema: %v", err)
	}
	return path
}

func TestLoadJSONSchemaValidatesFullContract(t *testing.T) {
	path := writeSchema(t, `{
		"type":"object",
		"properties":{"count":{"type":"integer","minimum":1}},
		"required":["count"],
		"additionalProperties":false
	}`)
	validate, hash, err := LoadJSONSchema(path)
	if err != nil {
		t.Fatalf("load schema: %v", err)
	}
	if len(hash) != 64 {
		t.Fatalf("schema hash length = %d, want 64", len(hash))
	}
	if err := validate(json.RawMessage(`{"count":2}`)); err != nil {
		t.Fatalf("valid output rejected: %v", err)
	}
	for _, invalid := range []string{`{"count":0}`, `{"count":"2"}`, `{"count":2,"extra":true}`} {
		if err := validate(json.RawMessage(invalid)); err == nil {
			t.Fatalf("invalid output accepted: %s", invalid)
		}
	}
}

func TestLoadJSONSchemaRejectsExternalReferences(t *testing.T) {
	path := writeSchema(t, `{"$ref":"https://example.com/output.schema.json"}`)
	_, _, err := LoadJSONSchema(path)
	if err == nil || !strings.Contains(err.Error(), "external schema reference") {
		t.Fatalf("external reference error = %v", err)
	}
}

func TestLoadJSONSchemaRejectsOversizedFile(t *testing.T) {
	path := writeSchema(t, `{"description":"`+strings.Repeat("a", maxSchemaBytes)+`"}`)
	if _, _, err := LoadJSONSchema(path); err == nil || !strings.Contains(err.Error(), "exceeds") {
		t.Fatalf("oversized schema error = %v", err)
	}
}

func TestCombineValidatorsReturnsFirstFailure(t *testing.T) {
	validate := CombineValidators(RequireFields("name"), func(json.RawMessage) error {
		return nil
	})
	if err := validate(json.RawMessage(`{"id":1}`)); err == nil || !strings.Contains(err.Error(), "name") {
		t.Fatalf("combined validation error = %v", err)
	}
}

func TestSchemaInvalidOutputRetriesWithFeedback(t *testing.T) {
	schemaPath := writeSchema(t, `{"type":"object","properties":{"count":{"type":"integer","minimum":1}},"required":["count"]}`)
	validate, _, err := LoadJSONSchema(schemaPath)
	if err != nil {
		t.Fatalf("load schema: %v", err)
	}
	input := writeInput(t, []string{`{"id":1}`})
	output := filepath.Join(t.TempDir(), "out.jsonl")
	feedback := ""
	mapper := &Mapper{Concurrency: 1, MaxRetries: 1, Validate: validate, Process: func(_ context.Context, _ json.RawMessage, attempt int, prior string) (json.RawMessage, error) {
		feedback = prior
		if attempt == 1 {
			return json.RawMessage(`{"count":0}`), nil
		}
		return json.RawMessage(`{"count":2}`), nil
	}}
	result, err := mapper.Run(context.Background(), input, output)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if result.Succeeded != 1 || !strings.Contains(feedback, "schema validation") {
		t.Fatalf("result = %+v, feedback = %q", result, feedback)
	}
	lines := readOutputs(t, output)
	if len(lines) != 1 || lines[0].Attempts != 2 || !lines[0].OK {
		t.Fatalf("schema retry output = %+v", lines)
	}
}

func TestSchemaInvalidOutputIsTerminalAfterRetries(t *testing.T) {
	schemaPath := writeSchema(t, `{"type":"integer"}`)
	validate, _, err := LoadJSONSchema(schemaPath)
	if err != nil {
		t.Fatalf("load schema: %v", err)
	}
	input := writeInput(t, []string{`{"id":1}`})
	output := filepath.Join(t.TempDir(), "out.jsonl")
	mapper := &Mapper{Concurrency: 1, MaxRetries: 1, Validate: validate, Process: func(_ context.Context, _ json.RawMessage, _ int, _ string) (json.RawMessage, error) {
		return json.RawMessage(`{"not":"integer"}`), nil
	}}
	result, err := mapper.Run(context.Background(), input, output)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	lines := readOutputs(t, output)
	if result.Failed != 1 || len(lines) != 1 || lines[0].OK || lines[0].Attempts != 2 {
		t.Fatalf("terminal schema failure: result=%+v output=%+v", result, lines)
	}
}
