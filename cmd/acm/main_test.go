package main

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/bonztm/agent-context-manager/internal/logging"
)

func TestPrintMainUsage_IncludesCommandDirectoryAndRecovery(t *testing.T) {
	var buf bytes.Buffer
	printMainUsage(&buf)

	output := buf.String()
	requiredSnippets := []string{
		"Entry Points:",
		"Agent Workflow Commands:",
		"Maintenance Commands:",
		"acm --version | -v",
		"Shared Conventions:",
		"High-Signal Requirements:",
		"Config Resolution:",
		"Environment Variables:",
		"Managed Repo Files:",
		"First-Run Recovery:",
		"acm get-context [--project <id>] [--task-text <text>|--task-file <path>] [--tags-file <path>] [--unbounded[=true|false]] [flags]",
		"acm work list [--project <id>] [--scope <current|deferred|completed|all>] [--kind <kind>] [--limit <n>] [--unbounded[=true|false]]",
		"acm work search [--project <id>] (--query <text>|--query-file <path>) [--scope <current|deferred|completed|all>] [--kind <kind>] [--limit <n>] [--unbounded[=true|false]]",
		"acm history search [--project <id>] [--entity <all|work|memory|receipt|run>] [--query <text>|--query-file <path>] [--limit <n>] [--unbounded[=true|false]]",
		"acm review [--project <id>] [--receipt-id <id>|--plan-key <key>] [--run] [--key <task-key>] [--summary <text>] [--status <pending|in_progress|complete|blocked>] [--outcome <text>|--outcome-file <path>] [--blocked-reason <text>] [--evidence <text>]... [--evidence-file <path>|--evidence-json <json>] [--tags-file <path>]",
		"acm verify [--project <id>] [--receipt-id <id>] [--plan-key <key>] [--phase <plan|execute|review>] [--test-id <id>]... [--file-changed <path>]... [--files-changed-file <path>|--files-changed-json <json>] [--tests-file <path>] [--tags-file <path>] [--dry-run]",
		"acm bootstrap",
		"acm bootstrap --apply-template starter-contract --apply-template verify-go",
		"export ACM_PG_DSN='postgres://user:pass@localhost:5432/agents_context?sslmode=disable'",
		"export ACM_PROJECT_ID=myproject",
		"`ACM_PROJECT_ID`: Optional default project identifier for convenience, run, validate, and MCP tool calls.",
		"`ACM_UNBOUNDED`: `true|false`. When true, retrieval/list surfaces stop applying built-in result caps.",
		"`.acm/acm-workflows.yaml` or `acm-workflows.yaml`: repo-local completion gate definitions.",
		"`--request` and `--request-id` are aliases on convenience commands.",
		"Convenience commands accept optional `--project` or `--project-id`; explicit values override env and repo-root defaults.",
		"Optional bool flags accept `--flag`, `--flag=true`, or `--flag=false`.",
	}
	for _, snippet := range requiredSnippets {
		if !strings.Contains(output, snippet) {
			t.Fatalf("main usage is missing snippet %q\noutput:\n%s", snippet, output)
		}
	}
}

func TestPrintVersionWritesBinaryBanner(t *testing.T) {
	var buf bytes.Buffer
	printVersion(&buf, "acm")

	output := strings.TrimSpace(buf.String())
	if !strings.HasPrefix(output, "acm ") {
		t.Fatalf("unexpected version output: %q", output)
	}
	if output == "acm" {
		t.Fatalf("expected version suffix in output: %q", output)
	}
}

func TestRunHelpReturnsSuccess(t *testing.T) {
	code := run(context.Background(), logging.NewRecorder(), []string{"--help"})
	if code != 0 {
		t.Fatalf("unexpected exit code for run --help: got %d want 0", code)
	}
}

func TestValidateHelpReturnsSuccess(t *testing.T) {
	code := validate(context.Background(), logging.NewRecorder(), []string{"--help"})
	if code != 0 {
		t.Fatalf("unexpected exit code for validate --help: got %d want 0", code)
	}
}

func TestNestedConvenienceSubcommand(t *testing.T) {
	tests := []struct {
		command string
		args    []string
		want    string
		ok      bool
	}{
		{command: "work", args: []string{"list"}, want: "work-list", ok: true},
		{command: "work", args: []string{"search"}, want: "work-search", ok: true},
		{command: "work", args: []string{"--project", "x"}, want: "", ok: false},
		{command: "history", args: []string{"search"}, want: "history-search", ok: true},
	}
	for _, tc := range tests {
		got, ok := nestedConvenienceSubcommand(tc.command, tc.args)
		if got != tc.want || ok != tc.ok {
			t.Fatalf("nestedConvenienceSubcommand(%q, %v) = (%q, %v), want (%q, %v)", tc.command, tc.args, got, ok, tc.want, tc.ok)
		}
	}
}
