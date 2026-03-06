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
		"Shared Conventions:",
		"High-Signal Requirements:",
		"Config Resolution:",
		"Environment Variables:",
		"Managed Repo Files:",
		"First-Run Recovery:",
		"acm get-context --project <id> [--task-text <text>|--task-file <path>] [--tags-file <path>] [flags]",
		"acm verify --project <id> [--receipt-id <id>] [--plan-key <key>] [--phase <plan|execute|review>] [--test-id <id>]... [--file-changed <path>]... [--files-changed-file <path>|--files-changed-json <json>] [--tests-file <path>] [--tags-file <path>] [--dry-run]",
		"acm bootstrap --project myproject --project-root .",
		"export ACM_PG_DSN='postgres://user:pass@localhost:5432/agents_context?sslmode=disable'",
		"`--request` and `--request-id` are aliases on convenience commands.",
		"Optional bool flags accept `--flag`, `--flag=true`, or `--flag=false`.",
	}
	for _, snippet := range requiredSnippets {
		if !strings.Contains(output, snippet) {
			t.Fatalf("main usage is missing snippet %q\noutput:\n%s", snippet, output)
		}
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
