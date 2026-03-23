package bootstrap

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEmbeddedTemplateManifestsUseInitTemplateVersion(t *testing.T) {
	t.Parallel()

	manifestPaths, err := filepath.Glob(filepath.Join("bootstrap_templates", "*", "template.yaml"))
	if err != nil {
		t.Fatalf("glob embedded manifests: %v", err)
	}
	if len(manifestPaths) == 0 {
		t.Fatal("expected embedded template manifests")
	}

	for _, manifestPath := range manifestPaths {
		manifestPath := manifestPath
		t.Run(filepath.Base(filepath.Dir(manifestPath)), func(t *testing.T) {
			raw, err := initTemplateFS.ReadFile(filepath.ToSlash(manifestPath))
			if err != nil {
				t.Fatalf("read manifest %s: %v", manifestPath, err)
			}
			if !strings.Contains(string(raw), "version: acm.init-template.v1") {
				t.Fatalf("manifest %s must use acm.init-template.v1", manifestPath)
			}
		})
	}
}

func TestClaudeCommandPackTemplateMatchesSkillPack(t *testing.T) {
	t.Parallel()

	cases := map[string]string{
		"bootstrap_templates/claude-command-pack/files/.claude/acm-broker/CLAUDE.md":    "../../skills/acm-broker/claude/CLAUDE.md",
		"bootstrap_templates/claude-command-pack/files/.claude/acm-broker/README.md":    "../../skills/acm-broker/claude/README.md",
		"bootstrap_templates/claude-command-pack/files/.claude/commands/acm-context.md": "../../skills/acm-broker/claude/commands/acm-context.md",
		"bootstrap_templates/claude-command-pack/files/.claude/commands/acm-done.md":    "../../skills/acm-broker/claude/commands/acm-done.md",
		"bootstrap_templates/claude-command-pack/files/.claude/commands/acm-review.md":  "../../skills/acm-broker/claude/commands/acm-review.md",
		"bootstrap_templates/claude-command-pack/files/.claude/commands/acm-verify.md":  "../../skills/acm-broker/claude/commands/acm-verify.md",
		"bootstrap_templates/claude-command-pack/files/.claude/commands/acm-work.md":    "../../skills/acm-broker/claude/commands/acm-work.md",
	}

	for embeddedPath, repoRelativePath := range cases {
		t.Run(filepath.Base(embeddedPath), func(t *testing.T) {
			expected, err := os.ReadFile(filepath.Clean(repoRelativePath))
			if err != nil {
				t.Fatalf("read skill-pack asset: %v", err)
			}
			actual, err := initTemplateFS.ReadFile(embeddedPath)
			if err != nil {
				t.Fatalf("read embedded template asset: %v", err)
			}
			if string(actual) != string(expected) {
				t.Fatalf("template asset drifted from skill-pack counterpart: %s", embeddedPath)
			}
		})
	}
}

func TestCodexPackTemplateMatchesSkillPack(t *testing.T) {
	t.Parallel()

	cases := map[string]string{
		"bootstrap_templates/codex-pack/files/.codex/acm-broker/README.md":         "../../skills/acm-broker/codex/README.md",
		"bootstrap_templates/codex-pack/files/.codex/acm-broker/AGENTS.example.md": "../../skills/acm-broker/codex/AGENTS.example.md",
	}

	for embeddedPath, repoRelativePath := range cases {
		t.Run(filepath.Base(embeddedPath), func(t *testing.T) {
			expected, err := os.ReadFile(filepath.Clean(repoRelativePath))
			if err != nil {
				t.Fatalf("read skill-pack asset: %v", err)
			}
			actual, err := initTemplateFS.ReadFile(embeddedPath)
			if err != nil {
				t.Fatalf("read embedded template asset: %v", err)
			}
			if string(actual) != string(expected) {
				t.Fatalf("template asset drifted from skill-pack counterpart: %s", embeddedPath)
			}
		})
	}
}

func TestCodexHooksTemplateEnablesExperimentalFeature(t *testing.T) {
	t.Parallel()

	raw, err := initTemplateFS.ReadFile("bootstrap_templates/codex-hooks/files/.codex/config.toml")
	if err != nil {
		t.Fatalf("read embedded Codex hooks config: %v", err)
	}
	content := string(raw)
	for _, snippet := range []string{"[features]", "codex_hooks = true"} {
		if !strings.Contains(content, snippet) {
			t.Fatalf("Codex hooks config is missing snippet %q", snippet)
		}
	}
	for _, forbidden := range []string{"claude-hooks", "PreToolUse", "PostToolUse"} {
		if strings.Contains(content, forbidden) {
			t.Fatalf("Codex hooks config must not imply unsupported snippet %q", forbidden)
		}
	}
}

func TestCodexHooksTemplateSeedsOnlyCurrentLifecycleEvents(t *testing.T) {
	t.Parallel()

	raw, err := initTemplateFS.ReadFile("bootstrap_templates/codex-hooks/files/.codex/hooks.json")
	if err != nil {
		t.Fatalf("read embedded Codex hooks manifest: %v", err)
	}
	content := string(raw)
	for _, snippet := range []string{"SessionStart", "UserPromptSubmit", "Stop", "acm-session-context.sh", "acm-prompt-guard.sh", "acm-stop-guard.sh", "statusMessage"} {
		if !strings.Contains(content, snippet) {
			t.Fatalf("Codex hooks manifest is missing snippet %q", snippet)
		}
	}
	for _, forbidden := range []string{"SessionStop", "PreToolUse", "PostToolUse", "acm-receipt-mark.sh", "acm-edit-state.sh"} {
		if strings.Contains(content, forbidden) {
			t.Fatalf("Codex hooks manifest must not imply unsupported snippet %q", forbidden)
		}
	}
}

func TestOpenCodePackTemplateMatchesSkillPack(t *testing.T) {
	t.Parallel()

	cases := map[string]string{
		"bootstrap_templates/opencode-pack/files/.opencode/acm-broker/README.md":         "../../skills/acm-broker/opencode/README.md",
		"bootstrap_templates/opencode-pack/files/.opencode/acm-broker/AGENTS.example.md": "../../skills/acm-broker/opencode/AGENTS.example.md",
	}

	for embeddedPath, repoRelativePath := range cases {
		t.Run(filepath.Base(embeddedPath), func(t *testing.T) {
			expected, err := os.ReadFile(filepath.Clean(repoRelativePath))
			if err != nil {
				t.Fatalf("read skill-pack asset: %v", err)
			}
			actual, err := initTemplateFS.ReadFile(embeddedPath)
			if err != nil {
				t.Fatalf("read embedded template asset: %v", err)
			}
			if string(actual) != string(expected) {
				t.Fatalf("template asset drifted from skill-pack counterpart: %s", embeddedPath)
			}
		})
	}
}

func TestRepoClaudeCommandPackMatchesSkillPack(t *testing.T) {
	t.Parallel()

	cases := map[string]string{
		"../../.claude/acm-broker/CLAUDE.md":    "../../skills/acm-broker/claude/CLAUDE.md",
		"../../.claude/acm-broker/README.md":    "../../skills/acm-broker/claude/README.md",
		"../../.claude/commands/acm-context.md": "../../skills/acm-broker/claude/commands/acm-context.md",
		"../../.claude/commands/acm-done.md":    "../../skills/acm-broker/claude/commands/acm-done.md",
		"../../.claude/commands/acm-review.md":  "../../skills/acm-broker/claude/commands/acm-review.md",
		"../../.claude/commands/acm-verify.md":  "../../skills/acm-broker/claude/commands/acm-verify.md",
		"../../.claude/commands/acm-work.md":    "../../skills/acm-broker/claude/commands/acm-work.md",
	}

	for repoRelativePath, skillRelativePath := range cases {
		t.Run(filepath.Base(repoRelativePath), func(t *testing.T) {
			actual, err := os.ReadFile(filepath.Clean(repoRelativePath))
			if err != nil {
				t.Fatalf("read repo asset: %v", err)
			}
			expected, err := os.ReadFile(filepath.Clean(skillRelativePath))
			if err != nil {
				t.Fatalf("read skill-pack asset: %v", err)
			}
			if string(actual) != string(expected) {
				t.Fatalf("repo asset drifted from skill-pack counterpart: %s", repoRelativePath)
			}
		})
	}
}

func TestRepoCodexCompanionMatchesSkillPack(t *testing.T) {
	t.Parallel()

	cases := map[string]string{
		"../../.codex/acm-broker/README.md":         "../../skills/acm-broker/codex/README.md",
		"../../.codex/acm-broker/AGENTS.example.md": "../../skills/acm-broker/codex/AGENTS.example.md",
	}

	for repoRelativePath, skillRelativePath := range cases {
		t.Run(filepath.Base(repoRelativePath), func(t *testing.T) {
			actual, err := os.ReadFile(filepath.Clean(repoRelativePath))
			if err != nil {
				t.Fatalf("read repo asset: %v", err)
			}
			expected, err := os.ReadFile(filepath.Clean(skillRelativePath))
			if err != nil {
				t.Fatalf("read skill-pack asset: %v", err)
			}
			if string(actual) != string(expected) {
				t.Fatalf("repo asset drifted from skill-pack counterpart: %s", repoRelativePath)
			}
		})
	}
}

func TestRepoOpenCodeCompanionMatchesSkillPack(t *testing.T) {
	t.Parallel()

	cases := map[string]string{
		"../../.opencode/acm-broker/README.md":         "../../skills/acm-broker/opencode/README.md",
		"../../.opencode/acm-broker/AGENTS.example.md": "../../skills/acm-broker/opencode/AGENTS.example.md",
	}

	for repoRelativePath, skillRelativePath := range cases {
		t.Run(filepath.Base(repoRelativePath), func(t *testing.T) {
			actual, err := os.ReadFile(filepath.Clean(repoRelativePath))
			if err != nil {
				t.Fatalf("read repo asset: %v", err)
			}
			expected, err := os.ReadFile(filepath.Clean(skillRelativePath))
			if err != nil {
				t.Fatalf("read skill-pack asset: %v", err)
			}
			if string(actual) != string(expected) {
				t.Fatalf("repo asset drifted from skill-pack counterpart: %s", repoRelativePath)
			}
		})
	}
}

func TestClaudeHooksReceiptMarkHookCoversContextJSONFlow(t *testing.T) {
	t.Parallel()

	raw, err := initTemplateFS.ReadFile("bootstrap_templates/claude-hooks/files/.claude/hooks/acm-receipt-mark.sh")
	if err != nil {
		t.Fatalf("read receipt mark hook: %v", err)
	}
	content := string(raw)
	requiredSnippets := []string{
		"is_task_context_command",
		"--task-(text|file)",
		"(-h|--help)",
		"acm[[:space:]]+run",
		"extract_acm_input_path",
		"request_declares_command",
		`request_declares_command "$INPUT_PATH" "context"`,
		`request_declares_command "$INPUT_PATH" "work"`,
		`request_declares_command "$INPUT_PATH" "verify"`,
		`request_declares_command "$INPUT_PATH" "done"`,
		"acm-mcp[[:space:]]+invoke",
		"acm[[:space:]]+done",
	}
	for _, snippet := range requiredSnippets {
		if !strings.Contains(content, snippet) {
			t.Fatalf("receipt mark hook is missing snippet %q", snippet)
		}
	}
	for _, forbidden := range []string{"get_context", "report_completion", "report-completion"} {
		if strings.Contains(content, forbidden) {
			t.Fatalf("receipt mark hook must not retain legacy command snippet %q", forbidden)
		}
	}
}

func TestClaudeHooksSettingsIncludeProcessHooks(t *testing.T) {
	t.Parallel()

	raw, err := initTemplateFS.ReadFile("bootstrap_templates/claude-hooks/files/.claude/settings.json")
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
			"/acm-context",
			"/acm-done",
		},
		"bootstrap_templates/claude-hooks/files/.claude/hooks/acm-stop-guard.sh": {
			`"Stop"`,
			`decision: "block"`,
			"/acm-verify",
			"/acm-done",
		},
	}

	for path, snippets := range cases {
		raw, err := initTemplateFS.ReadFile(path)
		if err != nil {
			t.Fatalf("read hook asset %s: %v", path, err)
		}
		content := string(raw)
		for _, snippet := range snippets {
			if !strings.Contains(content, snippet) {
				t.Fatalf("hook asset %s is missing snippet %q", path, snippet)
			}
		}
		for _, forbidden := range []string{"/acm-get", "/acm-report"} {
			if strings.Contains(content, forbidden) {
				t.Fatalf("hook asset %s must not retain legacy command snippet %q", path, forbidden)
			}
		}
	}
}

func TestResolveTemplatesRejectsRemovedClaudeReceiptGuardAlias(t *testing.T) {
	t.Parallel()

	_, err := ResolveTemplates([]string{"claude-receipt-guard"})
	if err == nil {
		t.Fatalf("expected removed legacy template alias to be rejected")
	}
}

func TestGitHooksPrecommitTemplateIncludesDeletedFiles(t *testing.T) {
	t.Parallel()

	raw, err := initTemplateFS.ReadFile("bootstrap_templates/git-hooks-precommit/files/.githooks/pre-commit")
	if err != nil {
		t.Fatalf("read pre-commit hook: %v", err)
	}
	content := string(raw)
	if !strings.Contains(content, "--diff-filter=ACMRTD") {
		t.Fatalf("pre-commit hook must include staged deletions in the verify diff filter")
	}
}

func TestStarterAndDetailedContractsCarryMaintenanceAndDiscoveredScopeGuidance(t *testing.T) {
	t.Parallel()

	cases := map[string][]string{
		"bootstrap_templates/starter-contract/files/AGENTS.md": {
			"acm sync --mode working_tree --insert-new-candidates",
			"acm health --include-details",
			"work.plan.discovered_paths",
		},
		"bootstrap_templates/detailed-planning-enforcement/files/AGENTS.md": {
			"acm sync --mode working_tree --insert-new-candidates",
			"acm health --include-details",
			"work.plan.discovered_paths",
		},
		"bootstrap_templates/starter-contract/files/CLAUDE.md": {
			"acm sync --mode working_tree --insert-new-candidates",
			"acm health --include-details",
		},
		"bootstrap_templates/detailed-planning-enforcement/files/CLAUDE.md": {
			"acm sync --mode working_tree --insert-new-candidates",
			"acm health --include-details",
		},
	}

	for path, snippets := range cases {
		raw, err := initTemplateFS.ReadFile(path)
		if err != nil {
			t.Fatalf("read template %s: %v", path, err)
		}
		content := string(raw)
		for _, snippet := range snippets {
			if !strings.Contains(content, snippet) {
				t.Fatalf("template %s is missing snippet %q", path, snippet)
			}
		}
	}
}

func TestStarterAndDetailedRulesetsDescribeEffectiveScopeAndBaselineDone(t *testing.T) {
	t.Parallel()

	cases := []string{
		"bootstrap_templates/starter-contract/files/.acm/acm-rules.yaml",
		"bootstrap_templates/detailed-planning-enforcement/files/.acm/acm-rules.yaml",
	}

	for _, path := range cases {
		raw, err := initTemplateFS.ReadFile(path)
		if err != nil {
			t.Fatalf("read rules template %s: %v", path, err)
		}
		content := string(raw)
		for _, snippet := range []string{"plan.discovered_paths", "receipt baseline", "effectively no-file"} {
			if !strings.Contains(content, snippet) {
				t.Fatalf("rules template %s is missing snippet %q", path, snippet)
			}
		}
	}
}

func TestExampleRulesetCarriesDiscoveredScopeAndMaintenanceGuidance(t *testing.T) {
	t.Parallel()

	raw, err := os.ReadFile(filepath.Clean("../../docs/examples/acm-rules.yaml"))
	if err != nil {
		t.Fatalf("read example ruleset: %v", err)
	}
	content := string(raw)
	for _, snippet := range []string{
		"work.plan.discovered_paths",
		"acm sync --mode working_tree --insert-new-candidates",
		"acm health --include-details",
		"receipt baseline",
		"effectively no-file",
	} {
		if !strings.Contains(content, snippet) {
			t.Fatalf("example ruleset is missing snippet %q", snippet)
		}
	}
}

func TestSkillReferencesAndWorkFixturesCoverDiscoveredScopeAndMaintenanceLoop(t *testing.T) {
	t.Parallel()

	referencePath := filepath.Clean("../../skills/acm-broker/references/templates.md")
	referenceRaw, err := os.ReadFile(referencePath)
	if err != nil {
		t.Fatalf("read skill reference: %v", err)
	}
	referenceContent := string(referenceRaw)
	for _, snippet := range []string{
		"plan.discovered_paths",
		"acm sync --mode working_tree --insert-new-candidates",
		"acm health --include-details",
	} {
		if !strings.Contains(referenceContent, snippet) {
			t.Fatalf("skill reference is missing snippet %q", snippet)
		}
	}

	for _, path := range []string{
		"../../skills/acm-broker/assets/requests/work.json",
		"../../skills/acm-broker/assets/requests/mcp_work.json",
	} {
		raw, err := os.ReadFile(filepath.Clean(path))
		if err != nil {
			t.Fatalf("read skill request asset %s: %v", path, err)
		}
		if !strings.Contains(string(raw), "\"discovered_paths\"") {
			t.Fatalf("skill request asset %s must model discovered_paths", path)
		}
	}
}

func TestClaudeDoneAndVerifyPromptsMatchBaselineErgonomics(t *testing.T) {
	t.Parallel()

	doneRaw, err := initTemplateFS.ReadFile("bootstrap_templates/claude-command-pack/files/.claude/commands/acm-done.md")
	if err != nil {
		t.Fatalf("read embedded done command: %v", err)
	}
	doneContent := string(doneRaw)
	for _, snippet := range []string{"<receipt_id-or-plan_key>", "-- <outcome summary>", "baseline-derived delta"} {
		if !strings.Contains(doneContent, snippet) {
			t.Fatalf("done prompt is missing snippet %q", snippet)
		}
	}
	for _, forbidden := range []string{"comma-separated-files|none", "Use `none` or `-`"} {
		if strings.Contains(doneContent, forbidden) {
			t.Fatalf("done prompt must not retain stale snippet %q", forbidden)
		}
	}

	verifyRaw, err := initTemplateFS.ReadFile("bootstrap_templates/claude-command-pack/files/.claude/commands/acm-verify.md")
	if err != nil {
		t.Fatalf("read embedded verify command: %v", err)
	}
	verifyContent := string(verifyRaw)
	for _, snippet := range []string{"[comma-separated-files]", "baseline detection is unavailable", "one of `plan`, `execute`, or `review`"} {
		if !strings.Contains(verifyContent, snippet) {
			t.Fatalf("verify prompt is missing snippet %q", snippet)
		}
	}
}

func TestClaudeBrokerCompanionCoversMaintenanceAndDiscoveredScope(t *testing.T) {
	t.Parallel()

	raw, err := initTemplateFS.ReadFile("bootstrap_templates/claude-command-pack/files/.claude/acm-broker/CLAUDE.md")
	if err != nil {
		t.Fatalf("read embedded Claude companion: %v", err)
	}
	content := string(raw)
	for _, snippet := range []string{
		"plan.discovered_paths",
		"acm sync --mode working_tree --insert-new-candidates",
		"acm health --include-details",
		"receipt baseline",
		"effectively no-file",
	} {
		if !strings.Contains(content, snippet) {
			t.Fatalf("Claude companion is missing snippet %q", snippet)
		}
	}
}

func TestCodexCompanionCoversPrimaryWorkflowWithoutFakeClaudeParity(t *testing.T) {
	t.Parallel()

	raw, err := initTemplateFS.ReadFile("bootstrap_templates/codex-pack/files/.codex/acm-broker/README.md")
	if err != nil {
		t.Fatalf("read embedded Codex companion: %v", err)
	}
	content := string(raw)
	for _, snippet := range []string{
		"acm init --apply-template codex-pack",
		"acm init --apply-template codex-hooks",
		"work.plan.discovered_paths",
		"acm sync --mode working_tree --insert-new-candidates",
		"acm health --include-details",
		"`acm context`",
		"`acm work`",
		"`acm verify`",
		"`acm done`",
	} {
		if !strings.Contains(content, snippet) {
			t.Fatalf("Codex companion is missing snippet %q", snippet)
		}
	}
	for _, forbidden := range []string{"/acm-context", "SessionStart", "claude-hooks"} {
		if strings.Contains(content, forbidden) {
			t.Fatalf("Codex companion must not imply Claude-only surface snippet %q", forbidden)
		}
	}
}

func TestCodexCompanionExampleTreatsCodexAsPrimaryOperator(t *testing.T) {
	t.Parallel()

	raw, err := initTemplateFS.ReadFile("bootstrap_templates/codex-pack/files/.codex/acm-broker/AGENTS.example.md")
	if err != nil {
		t.Fatalf("read embedded Codex AGENTS example: %v", err)
	}
	content := string(raw)
	for _, snippet := range []string{
		"Codex is a primary ACM operator",
		"acm context",
		"acm verify",
		"acm done",
		"work.plan.discovered_paths",
	} {
		if !strings.Contains(content, snippet) {
			t.Fatalf("Codex AGENTS example is missing snippet %q", snippet)
		}
	}
}

func TestOpenCodeCompanionCoversPrimaryWorkflowWithoutInventedGlobalHooks(t *testing.T) {
	t.Parallel()

	raw, err := initTemplateFS.ReadFile("bootstrap_templates/opencode-pack/files/.opencode/acm-broker/README.md")
	if err != nil {
		t.Fatalf("read embedded OpenCode companion: %v", err)
	}
	content := string(raw)
	for _, snippet := range []string{
		"acm init --apply-template opencode-pack",
		"work.plan.discovered_paths",
		"acm sync --mode working_tree --insert-new-candidates",
		"acm health --include-details",
		"`acm context`",
		"`acm work`",
		"`acm verify`",
		"`acm done`",
		"documentation only",
	} {
		if !strings.Contains(content, snippet) {
			t.Fatalf("OpenCode companion is missing snippet %q", snippet)
		}
	}
	for _, forbidden := range []string{"/acm-context", "SessionStart", "~/.opencode/skills"} {
		if strings.Contains(content, forbidden) {
			t.Fatalf("OpenCode companion must not imply unsupported integration snippet %q", forbidden)
		}
	}
}

func TestOpenCodeCompanionExampleTreatsOpenCodeAsPrimaryOperator(t *testing.T) {
	t.Parallel()

	raw, err := initTemplateFS.ReadFile("bootstrap_templates/opencode-pack/files/.opencode/acm-broker/AGENTS.example.md")
	if err != nil {
		t.Fatalf("read embedded OpenCode AGENTS example: %v", err)
	}
	content := string(raw)
	for _, snippet := range []string{
		"OpenCode is a primary ACM operator",
		"acm context",
		"acm verify",
		"acm done",
		"work.plan.discovered_paths",
		".opencode/acm-broker/",
	} {
		if !strings.Contains(content, snippet) {
			t.Fatalf("OpenCode AGENTS example is missing snippet %q", snippet)
		}
	}
}

func TestDetailedPlanningTemplateValidatorSkipsUnmaterializedReceiptPlans(t *testing.T) {
	t.Parallel()

	raw, err := initTemplateFS.ReadFile("bootstrap_templates/detailed-planning-enforcement/files/scripts/acm-feature-plan-validate.py")
	if err != nil {
		t.Fatalf("read embedded validator: %v", err)
	}
	content := string(raw)
	for _, snippet := range []string{"allow_unmaterialized=True", "has no materialized content in this receipt context"} {
		if !strings.Contains(content, snippet) {
			t.Fatalf("embedded validator is missing snippet %q", snippet)
		}
	}
}

func TestDetailedPlanningTemplateDocsMentionUnmaterializedReceiptContexts(t *testing.T) {
	t.Parallel()

	raw, err := initTemplateFS.ReadFile("bootstrap_templates/detailed-planning-enforcement/files/docs/feature-plans.md")
	if err != nil {
		t.Fatalf("read embedded feature-plan docs: %v", err)
	}
	if !strings.Contains(string(raw), "receipt contexts that do not materialize a concrete plan") {
		t.Fatalf("embedded feature-plan docs must mention unmaterialized receipt contexts")
	}
}
func TestInitTemplateDocsListCodexPack(t *testing.T) {
	t.Parallel()

	raw, err := os.ReadFile(filepath.Clean("../../docs/examples/init-templates.md"))
	if err != nil {
		t.Fatalf("read init-template docs: %v", err)
	}
	content := string(raw)
	for _, snippet := range []string{"`codex-pack`", ".codex/acm-broker/README.md", ".codex/acm-broker/AGENTS.example.md"} {
		if !strings.Contains(content, snippet) {
			t.Fatalf("init-template docs are missing snippet %q", snippet)
		}
	}
}

func TestInitTemplateDocsListCodexHooks(t *testing.T) {
	t.Parallel()

	raw, err := os.ReadFile(filepath.Clean("../../docs/examples/init-templates.md"))
	if err != nil {
		t.Fatalf("read init-template docs: %v", err)
	}
	content := string(raw)
	for _, snippet := range []string{"`codex-hooks`", ".codex/config.toml", ".codex/hooks.json", ".codex/hooks/acm-prompt-guard.sh"} {
		if !strings.Contains(content, snippet) {
			t.Fatalf("init-template docs are missing snippet %q", snippet)
		}
	}
}

func TestInitTemplateDocsListOpenCodePack(t *testing.T) {
	t.Parallel()

	raw, err := os.ReadFile(filepath.Clean("../../docs/examples/init-templates.md"))
	if err != nil {
		t.Fatalf("read init-template docs: %v", err)
	}
	content := string(raw)
	for _, snippet := range []string{"`opencode-pack`", ".opencode/acm-broker/README.md", ".opencode/acm-broker/AGENTS.example.md"} {
		if !strings.Contains(content, snippet) {
			t.Fatalf("init-template docs are missing snippet %q", snippet)
		}
	}
}

func TestInstallSkillPackMentionsCodexCompanionDocs(t *testing.T) {
	t.Parallel()

	raw, err := os.ReadFile(filepath.Clean("../../scripts/install-skill-pack.sh"))
	if err != nil {
		t.Fatalf("read install script: %v", err)
	}
	content := string(raw)
	for _, snippet := range []string{"Installed Codex companion docs", "codex-pack"} {
		if !strings.Contains(content, snippet) {
			t.Fatalf("install script is missing snippet %q", snippet)
		}
	}
	if !strings.Contains(content, "codex-hooks") {
		t.Fatalf("install script should mention the optional codex-hooks template")
	}
}

func TestInstallSkillPackMentionsOpenCodeCompanionDocs(t *testing.T) {
	t.Parallel()

	raw, err := os.ReadFile(filepath.Clean("../../scripts/install-skill-pack.sh"))
	if err != nil {
		t.Fatalf("read install script: %v", err)
	}
	content := string(raw)
	for _, snippet := range []string{"Installed OpenCode companion docs", "opencode-pack", "--opencode"} {
		if !strings.Contains(content, snippet) {
			t.Fatalf("install script is missing snippet %q", snippet)
		}
	}
}
