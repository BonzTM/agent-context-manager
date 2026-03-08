package bootstrap

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestClaudeCommandPackTemplateMatchesSkillPack(t *testing.T) {
	t.Parallel()

	cases := map[string]string{
		"bootstrap_templates/claude-command-pack/files/.claude/acm-broker/CLAUDE.md":   "../../skills/acm-broker/claude/CLAUDE.md",
		"bootstrap_templates/claude-command-pack/files/.claude/acm-broker/README.md":   "../../skills/acm-broker/claude/README.md",
		"bootstrap_templates/claude-command-pack/files/.claude/commands/acm-eval.md":   "../../skills/acm-broker/claude/commands/acm-eval.md",
		"bootstrap_templates/claude-command-pack/files/.claude/commands/acm-get.md":    "../../skills/acm-broker/claude/commands/acm-get.md",
		"bootstrap_templates/claude-command-pack/files/.claude/commands/acm-memory.md": "../../skills/acm-broker/claude/commands/acm-memory.md",
		"bootstrap_templates/claude-command-pack/files/.claude/commands/acm-report.md": "../../skills/acm-broker/claude/commands/acm-report.md",
		"bootstrap_templates/claude-command-pack/files/.claude/commands/acm-review.md": "../../skills/acm-broker/claude/commands/acm-review.md",
		"bootstrap_templates/claude-command-pack/files/.claude/commands/acm-verify.md": "../../skills/acm-broker/claude/commands/acm-verify.md",
		"bootstrap_templates/claude-command-pack/files/.claude/commands/acm-work.md":   "../../skills/acm-broker/claude/commands/acm-work.md",
	}

	for embeddedPath, repoRelativePath := range cases {
		t.Run(filepath.Base(embeddedPath), func(t *testing.T) {
			expected, err := os.ReadFile(filepath.Clean(repoRelativePath))
			if err != nil {
				t.Fatalf("read skill-pack asset: %v", err)
			}
			actual, err := bootstrapTemplateFS.ReadFile(embeddedPath)
			if err != nil {
				t.Fatalf("read embedded template asset: %v", err)
			}
			if string(actual) != string(expected) {
				t.Fatalf("template asset drifted from skill-pack counterpart: %s", embeddedPath)
			}
		})
	}
}

func TestClaudeHooksReceiptMarkHookCoversGetContextJSONFlow(t *testing.T) {
	t.Parallel()

	raw, err := bootstrapTemplateFS.ReadFile("bootstrap_templates/claude-hooks/files/.claude/hooks/acm-receipt-mark.sh")
	if err != nil {
		t.Fatalf("read receipt mark hook: %v", err)
	}
	content := string(raw)
	requiredSnippets := []string{
		"is_task_get_context_command",
		"--task-(text|file)",
		"(-h|--help)",
		"acm[[:space:]]+run",
		"extract_acm_input_path",
		"request_declares_command",
		`request_declares_command "$INPUT_PATH" "get_context"`,
		`request_declares_command "$INPUT_PATH" "work"`,
		`request_declares_command "$INPUT_PATH" "verify"`,
		`request_declares_command "$INPUT_PATH" "report_completion"`,
		"acm-mcp[[:space:]]+invoke",
		"report-completion",
	}
	for _, snippet := range requiredSnippets {
		if !strings.Contains(content, snippet) {
			t.Fatalf("receipt mark hook is missing snippet %q", snippet)
		}
	}
}

func TestClaudeHooksSettingsIncludeProcessHooks(t *testing.T) {
	t.Parallel()

	raw, err := bootstrapTemplateFS.ReadFile("bootstrap_templates/claude-hooks/files/.claude/settings.json")
	if err != nil {
		t.Fatalf("read Claude settings template: %v", err)
	}
	content := string(raw)
	requiredSnippets := []string{
		`"SessionStart"`,
		`"UserPromptSubmit"`,
		`"Stop"`,
		"Edit|MultiEdit|Write|NotebookEdit",
		"acm-session-context.sh",
		"acm-edit-state.sh",
		"acm-stop-guard.sh",
	}
	for _, snippet := range requiredSnippets {
		if !strings.Contains(content, snippet) {
			t.Fatalf("Claude hook settings are missing snippet %q", snippet)
		}
	}
}

func TestClaudeProcessHooksTrackWorkflowState(t *testing.T) {
	t.Parallel()

	cases := map[string][]string{
		"bootstrap_templates/claude-hooks/files/.claude/hooks/acm-receipt-guard.sh": {
			"files.txt",
			"/acm-work",
			"multi-file",
		},
		"bootstrap_templates/claude-hooks/files/.claude/hooks/acm-edit-state.sh": {
			"files.txt",
			`"${STATE_DIR}/verified"`,
			`"${STATE_DIR}/reported"`,
		},
		"bootstrap_templates/claude-hooks/files/.claude/hooks/acm-session-context.sh": {
			"AGENTS.md",
			"/acm-get",
			"/acm-report",
			"/acm-memory",
		},
		"bootstrap_templates/claude-hooks/files/.claude/hooks/acm-stop-guard.sh": {
			`"Stop"`,
			`decision: "block"`,
			"/acm-verify",
			"/acm-report",
		},
	}

	for path, snippets := range cases {
		raw, err := bootstrapTemplateFS.ReadFile(path)
		if err != nil {
			t.Fatalf("read hook asset %s: %v", path, err)
		}
		content := string(raw)
		for _, snippet := range snippets {
			if !strings.Contains(content, snippet) {
				t.Fatalf("hook asset %s is missing snippet %q", path, snippet)
			}
		}
	}
}

func TestResolveTemplatesMapsClaudeReceiptGuardAlias(t *testing.T) {
	t.Parallel()

	templates, err := ResolveTemplates([]string{"claude-receipt-guard", "claude-hooks"})
	if err != nil {
		t.Fatalf("resolve templates: %v", err)
	}
	if len(templates) != 1 {
		t.Fatalf("expected alias and canonical template ids to dedupe, got %d entries", len(templates))
	}
	if templates[0].ID != "claude-hooks" {
		t.Fatalf("expected canonical template id claude-hooks, got %q", templates[0].ID)
	}
}

func TestGitHooksPrecommitTemplateIncludesDeletedFiles(t *testing.T) {
	t.Parallel()

	raw, err := bootstrapTemplateFS.ReadFile("bootstrap_templates/git-hooks-precommit/files/.githooks/pre-commit")
	if err != nil {
		t.Fatalf("read pre-commit hook: %v", err)
	}
	content := string(raw)
	if !strings.Contains(content, "--diff-filter=ACMRTD") {
		t.Fatalf("pre-commit hook must include staged deletions in the verify diff filter")
	}
}
