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

func TestClaudeReceiptMarkHookCoversGetContextJSONFlow(t *testing.T) {
	t.Parallel()

	raw, err := bootstrapTemplateFS.ReadFile("bootstrap_templates/claude-receipt-guard/files/.claude/hooks/acm-receipt-mark.sh")
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
		"request_declares_get_context",
		`"command"[[:space:]]*:[[:space:]]*"get_context"`,
	}
	for _, snippet := range requiredSnippets {
		if !strings.Contains(content, snippet) {
			t.Fatalf("receipt mark hook is missing snippet %q", snippet)
		}
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
