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
