package v1

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestCommandCatalogCompleteness(t *testing.T) {
	specs := CommandSpecs()
	if got, want := len(specs), 12; got != want {
		t.Fatalf("unexpected command spec count: got %d want %d", got, want)
	}

	seenCommands := make(map[Command]struct{}, len(specs))
	seenSubcommands := make(map[string]struct{}, len(specs))

	for _, spec := range specs {
		if strings.TrimSpace(string(spec.Command)) == "" {
			t.Fatal("command name must not be empty")
		}
		if strings.TrimSpace(spec.CLISubcommand) == "" {
			t.Fatalf("command %q has empty CLISubcommand", spec.Command)
		}
		if strings.TrimSpace(spec.CLIUsage) == "" {
			t.Fatalf("command %q has empty CLIUsage", spec.Command)
		}
		if strings.TrimSpace(spec.CLISummary) == "" {
			t.Fatalf("command %q has empty CLISummary", spec.Command)
		}
		if strings.TrimSpace(spec.InputSchemaDef) == "" {
			t.Fatalf("command %q has empty InputSchemaDef", spec.Command)
		}
		if strings.TrimSpace(spec.ResultSchemaDef) == "" {
			t.Fatalf("command %q has empty ResultSchemaDef", spec.Command)
		}
		if strings.TrimSpace(spec.ToolTitle) == "" {
			t.Fatalf("command %q has empty ToolTitle", spec.Command)
		}
		if strings.TrimSpace(spec.ToolDescription) == "" {
			t.Fatalf("command %q has empty ToolDescription", spec.Command)
		}
		if spec.decode == nil {
			t.Fatalf("command %q has nil decode function", spec.Command)
		}

		if _, ok := seenCommands[spec.Command]; ok {
			t.Fatalf("duplicate command name: %q", spec.Command)
		}
		seenCommands[spec.Command] = struct{}{}

		if _, ok := seenSubcommands[spec.CLISubcommand]; ok {
			t.Fatalf("duplicate CLI subcommand: %q", spec.CLISubcommand)
		}
		seenSubcommands[spec.CLISubcommand] = struct{}{}

		assertDecodeDoesNotPanic(t, spec, nil)
		assertDecodeDoesNotPanic(t, spec, json.RawMessage(`{}`))
	}
}

func assertDecodeDoesNotPanic(t *testing.T, spec CommandSpec, raw json.RawMessage) {
	t.Helper()

	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("command %q decode panicked for input %s: %v", spec.Command, string(raw), r)
		}
	}()

	_, _ = spec.Decode(raw, ValidationDefaults{})
}
