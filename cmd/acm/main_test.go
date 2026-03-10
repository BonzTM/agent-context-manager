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
		"Structured JSON Automation:",
		"Agent Workflow Commands:",
		"Maintenance Commands:",
		"acm --version | -v",
		"Shared Conventions:",
		"High-Signal Requirements:",
		"Config Resolution:",
		"Environment Variables:",
		"Managed Repo Files:",
		"First-Run Recovery:",
		"acm context [--project <id>] [--task-text <text>|--task-file <path>] [--tags-file <path>] [--scope-path <path>]...",
		"acm history [--project <id>] [--entity <all|work|memory|receipt|run>] [--query <text>|--query-file <path>] [--scope <current|deferred|completed|all>] [--kind <kind>] [--limit <n>] [--unbounded[=true|false]]",
		"acm review [--project <id>] [--receipt-id <id>|--plan-key <key>] [--run] [--key <task-key>] [--summary <text>] [--status <pending|in_progress|complete|blocked>] [--outcome <text>|--outcome-file <path>] [--blocked-reason <text>] [--evidence <text>]... [--evidence-file <path>|--evidence-json <json>] [--tags-file <path>]",
		"acm health [--project <id>] [--include-details[=true|false]] [--max-findings-per-check <n>] | [--fix <name>]... [--dry-run[=true|false]] [--apply[=true|false]] [--project-root <path>] [--rules-file <path>] [--tags-file <path>]",
		"acm status [--project <id>] [--project-root <path>] [--rules-file <path>] [--tags-file <path>] [--tests-file <path>] [--workflows-file <path>] [--task-text <text>|--task-file <path>] [--phase <plan|execute|review>]",
		"acm verify [--project <id>] [--receipt-id <id>] [--plan-key <key>] [--phase <plan|execute|review>] [--test-id <id>]... [--file-changed <path>]... [--files-changed-file <path>|--files-changed-json <json>] [--tests-file <path>] [--tags-file <path>] [--dry-run]",
		"acm init",
		"acm init --apply-template starter-contract --apply-template verify-generic",
		"export ACM_PG_DSN='postgres://user:pass@localhost:5432/agents_context?sslmode=disable'",
		"export ACM_PROJECT_ID=myproject",
		"`ACM_PROJECT_ID`: Optional default project identifier for convenience, run, validate, and MCP tool calls.",
		"`ACM_UNBOUNDED`: `true|false`. When true, history surfaces stop applying built-in result caps.",
		"`.acm/acm-workflows.yaml` or `acm-workflows.yaml`: repo-local completion gate definitions.",
		"Run `acm health --help` to list available fixers and preview/apply examples.",
		"Convenience commands accept optional `--project`; explicit values override env and repo-root defaults.",
		"Optional bool flags accept `--flag`, `--flag=true`, or `--flag=false`.",
		"`context` requires one of `--task-text` or `--task-file`.",
		"`memory` requires `--receipt-id` or `--plan-key`, `--category`, `--subject`, `--confidence`, one of `--content` or `--content-file`, and at least one evidence key.",
		"`done` requires `--receipt-id` or `--plan-key` and one of `--outcome` or `--outcome-file`.",
	}
	for _, snippet := range requiredSnippets {
		if !strings.Contains(output, snippet) {
			t.Fatalf("main usage is missing snippet %q\noutput:\n%s", snippet, output)
		}
	}
	for _, hiddenSnippet := range []string{
		"acm get-context",
		"acm propose-memory",
		"acm report-completion",
		"acm bootstrap",
		"acm doctor",
		"acm history-search",
		"acm history --entity",
		"acm work-list",
		"acm work-search",
		"acm health-check",
		"acm health-fix",
	} {
		if strings.Contains(output, hiddenSnippet) {
			t.Fatalf("main usage should not advertise %q\noutput:\n%s", hiddenSnippet, output)
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
