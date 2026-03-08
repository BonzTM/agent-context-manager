package scripts_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestACMCrossReviewAcceptsWorkflowModelArgs(t *testing.T) {
	t.Parallel()

	args := runCrossReviewScript(t, []string{"--model", "gpt-5.4", "--reasoning-effort", "high"})
	assertArgSequence(t, args, "--model", "gpt-5.4")
	assertArgSequence(t, args, "-c", `model_reasoning_effort="high"`)
}

func TestACMCrossReviewUsesDefaultModelArgs(t *testing.T) {
	t.Parallel()

	args := runCrossReviewScript(t, nil)
	assertArgSequence(t, args, "--model", "gpt-5.3-codex")
	assertArgSequence(t, args, "-c", `model_reasoning_effort="xhigh"`)
}

func TestACMCrossReviewIncludesManagedAndDeletedScopedFiles(t *testing.T) {
	t.Parallel()

	tempRoot := t.TempDir()
	binDir := filepath.Join(tempRoot, "bin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatalf("mkdir bin: %v", err)
	}

	capturePath := filepath.Join(tempRoot, "codex-args.txt")
	promptPath := filepath.Join(tempRoot, "codex-prompt.txt")
	writeExecutable(t, filepath.Join(binDir, "codex"), `#!/usr/bin/env bash
set -euo pipefail
capture_path="${ACM_TEST_CAPTURE:?}"
prompt_path="${ACM_TEST_PROMPT:?}"
printf '%s\n' "$@" >"${capture_path}"
cat >"${prompt_path}"
output_path=""
while [[ $# -gt 0 ]]; do
  case "$1" in
    --output-last-message)
      output_path="$2"
      shift 2
      ;;
    *)
      shift
      ;;
  esac
done
printf '{"status":"pass","summary":"ok","findings":[]}\n' >"${output_path}"
`)
	writeExecutable(t, filepath.Join(binDir, "acm"), `#!/usr/bin/env bash
set -euo pipefail
cat <<'JSON'
{"result":{"items":[{"content":"{\"pointer_paths\":[\"docs/deleted.md\"]}"}]}}
JSON
`)

	projectRoot := filepath.Join(tempRoot, "project-root")
	if err := os.MkdirAll(filepath.Join(projectRoot, ".acm"), 0o755); err != nil {
		t.Fatalf("mkdir .acm: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(projectRoot, "docs"), 0o755); err != nil {
		t.Fatalf("mkdir docs: %v", err)
	}
	if err := os.WriteFile(filepath.Join(projectRoot, ".acm", "acm-tests.yaml"), []byte("version: acm.tests.v1\n"), 0o644); err != nil {
		t.Fatalf("write tests file: %v", err)
	}
	if err := os.WriteFile(filepath.Join(projectRoot, "docs", "deleted.md"), []byte("gone\n"), 0o644); err != nil {
		t.Fatalf("write deleted file: %v", err)
	}

	runGit(t, projectRoot, "init")
	runGit(t, projectRoot, "config", "user.email", "test@example.com")
	runGit(t, projectRoot, "config", "user.name", "Test User")
	runGit(t, projectRoot, "add", ".")
	runGit(t, projectRoot, "commit", "-m", "initial state")

	if err := os.Remove(filepath.Join(projectRoot, "docs", "deleted.md")); err != nil {
		t.Fatalf("remove deleted file: %v", err)
	}
	if err := os.WriteFile(filepath.Join(projectRoot, ".acm", "acm-tests.yaml"), []byte("version: acm.tests.v1\nsmoke: []\n"), 0o644); err != nil {
		t.Fatalf("rewrite tests file: %v", err)
	}

	cmd := exec.Command("bash", "scripts/acm-cross-review.sh")
	cmd.Dir = repoRoot(t)
	cmd.Env = append(os.Environ(),
		"PATH="+binDir+string(os.PathListSeparator)+os.Getenv("PATH"),
		"ACM_PROJECT_ID=agent-context-manager",
		"ACM_RECEIPT_ID=receipt-test",
		"ACM_PROJECT_ROOT="+projectRoot,
		"ACM_TEST_CAPTURE="+capturePath,
		"ACM_TEST_PROMPT="+promptPath,
	)

	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("run cross-review script: %v\n%s", err, string(output))
	}
	if !strings.Contains(string(output), "PASS: ok") {
		t.Fatalf("unexpected script output: %s", string(output))
	}

	prompt, err := os.ReadFile(promptPath)
	if err != nil {
		t.Fatalf("read captured prompt: %v", err)
	}
	promptText := string(prompt)
	if !strings.Contains(promptText, ".acm/acm-tests.yaml") {
		t.Fatalf("expected managed completion file in prompt, got %s", promptText)
	}
	if !strings.Contains(promptText, "docs/deleted.md") {
		t.Fatalf("expected deleted scoped file in prompt, got %s", promptText)
	}
	if !strings.Contains(promptText, "- repo_changed: 2") || !strings.Contains(promptText, "- scoped_changed: 2") {
		t.Fatalf("expected scope counts in prompt, got %s", promptText)
	}
	if !strings.Contains(string(output), "scoped 2/2 changed files") {
		t.Fatalf("expected scope counts in output, got %s", string(output))
	}
}

func TestACMCrossReviewFailsWhenRepoChangesExistButScopedSetIsEmpty(t *testing.T) {
	t.Parallel()

	tempRoot := t.TempDir()
	binDir := filepath.Join(tempRoot, "bin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatalf("mkdir bin: %v", err)
	}

	capturePath := filepath.Join(tempRoot, "codex-args.txt")
	writeExecutable(t, filepath.Join(binDir, "codex"), `#!/usr/bin/env bash
set -euo pipefail
capture_path="${ACM_TEST_CAPTURE:?}"
printf '%s\n' "$@" >"${capture_path}"
exit 0
`)
	writeExecutable(t, filepath.Join(binDir, "acm"), `#!/usr/bin/env bash
set -euo pipefail
cat <<'JSON'
{"result":{"items":[{"content":"{\"pointer_paths\":[\"docs/in-scope.md\"]}"}]}}
JSON
`)

	projectRoot := filepath.Join(tempRoot, "project-root")
	if err := os.MkdirAll(filepath.Join(projectRoot, "docs"), 0o755); err != nil {
		t.Fatalf("mkdir docs: %v", err)
	}
	if err := os.WriteFile(filepath.Join(projectRoot, "docs", "outside-scope.md"), []byte("draft\n"), 0o644); err != nil {
		t.Fatalf("write changed file: %v", err)
	}

	runGit(t, projectRoot, "init")
	runGit(t, projectRoot, "config", "user.email", "test@example.com")
	runGit(t, projectRoot, "config", "user.name", "Test User")
	runGit(t, projectRoot, "add", ".")
	runGit(t, projectRoot, "commit", "-m", "initial state")

	if err := os.WriteFile(filepath.Join(projectRoot, "docs", "outside-scope.md"), []byte("updated draft\n"), 0o644); err != nil {
		t.Fatalf("rewrite changed file: %v", err)
	}

	cmd := exec.Command("bash", "scripts/acm-cross-review.sh")
	cmd.Dir = repoRoot(t)
	cmd.Env = append(os.Environ(),
		"PATH="+binDir+string(os.PathListSeparator)+os.Getenv("PATH"),
		"ACM_PROJECT_ID=agent-context-manager",
		"ACM_RECEIPT_ID=receipt-test",
		"ACM_PROJECT_ROOT="+projectRoot,
		"ACM_TEST_CAPTURE="+capturePath,
	)

	output, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("expected cross-review script to fail when scoped set is empty, output=%s", string(output))
	}
	if !strings.Contains(string(output), "Review gate blocked before model execution") {
		t.Fatalf("expected empty-scope failure message, got %s", string(output))
	}
	if !strings.Contains(string(output), "1 repo change(s), 0 scoped change(s)") {
		t.Fatalf("expected repo/scoped counts in failure output, got %s", string(output))
	}
	if _, statErr := os.Stat(capturePath); !os.IsNotExist(statErr) {
		t.Fatalf("expected codex not to run on empty scoped review, stat err=%v", statErr)
	}
}

func runCrossReviewScript(t *testing.T, reviewArgs []string) []string {
	t.Helper()

	tempRoot := t.TempDir()
	binDir := filepath.Join(tempRoot, "bin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatalf("mkdir bin: %v", err)
	}

	capturePath := filepath.Join(tempRoot, "codex-args.txt")
	writeExecutable(t, filepath.Join(binDir, "codex"), `#!/usr/bin/env bash
set -euo pipefail
capture_path="${ACM_TEST_CAPTURE:?}"
printf '%s\n' "$@" >"${capture_path}"
output_path=""
while [[ $# -gt 0 ]]; do
  case "$1" in
    --output-last-message)
      output_path="$2"
      shift 2
      ;;
    *)
      shift
      ;;
  esac
done
cat >/dev/null
printf '{"status":"pass","summary":"ok","findings":[]}\n' >"${output_path}"
`)
	writeExecutable(t, filepath.Join(binDir, "acm"), `#!/usr/bin/env bash
exit 0
`)

	projectRoot := filepath.Join(tempRoot, "project-root")
	if err := os.MkdirAll(projectRoot, 0o755); err != nil {
		t.Fatalf("mkdir project root: %v", err)
	}

	cmdArgs := append([]string{"scripts/acm-cross-review.sh"}, reviewArgs...)
	cmd := exec.Command("bash", cmdArgs...)
	cmd.Dir = repoRoot(t)
	cmd.Env = append(os.Environ(),
		"PATH="+binDir+string(os.PathListSeparator)+os.Getenv("PATH"),
		"ACM_PROJECT_ID=agent-context-manager",
		"ACM_RECEIPT_ID=receipt-test",
		"ACM_PROJECT_ROOT="+projectRoot,
		"ACM_TEST_CAPTURE="+capturePath,
	)

	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("run cross-review script: %v\n%s", err, string(output))
	}
	if !strings.Contains(string(output), "PASS: ok") {
		t.Fatalf("unexpected script output: %s", string(output))
	}

	raw, err := os.ReadFile(capturePath)
	if err != nil {
		t.Fatalf("read captured codex args: %v", err)
	}
	return splitNonEmptyLines(string(raw))
}

func repoRoot(t *testing.T) string {
	t.Helper()

	root, err := filepath.Abs("..")
	if err != nil {
		t.Fatalf("resolve repo root: %v", err)
	}
	return root
}

func writeExecutable(t *testing.T, path, content string) {
	t.Helper()

	if err := os.WriteFile(path, []byte(content), 0o755); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()

	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s failed: %v\n%s", strings.Join(args, " "), err, string(output))
	}
}

func splitNonEmptyLines(raw string) []string {
	lines := strings.Split(raw, "\n")
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		if trimmed := strings.TrimSpace(line); trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}

func assertArgSequence(t *testing.T, args []string, want ...string) {
	t.Helper()

	for i := 0; i+len(want) <= len(args); i++ {
		match := true
		for j := range want {
			if args[i+j] != want[j] {
				match = false
				break
			}
		}
		if match {
			return
		}
	}
	t.Fatalf("argument sequence %q not found in %q", want, args)
}
