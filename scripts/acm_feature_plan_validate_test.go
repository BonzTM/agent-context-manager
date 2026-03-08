package scripts_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func cleanACMEnv(overrides ...string) []string {
	base := os.Environ()
	filtered := make([]string, 0, len(base)+len(overrides))
	for _, entry := range base {
		switch {
		case strings.HasPrefix(entry, "ACM_PLAN_KEY="),
			strings.HasPrefix(entry, "ACM_RECEIPT_ID="),
			strings.HasPrefix(entry, "ACM_PROJECT_ID="):
			continue
		default:
			filtered = append(filtered, entry)
		}
	}
	filtered = append(filtered, overrides...)
	return filtered
}

func TestACMFeaturePlanValidateSkipsWhenReceiptPlanHasNoContent(t *testing.T) {
	t.Parallel()

	tempRoot := t.TempDir()
	binDir := filepath.Join(tempRoot, "bin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatalf("mkdir bin: %v", err)
	}

	writeExecutable(t, filepath.Join(binDir, "acm"), `#!/usr/bin/env bash
set -euo pipefail
cat <<'JSON'
{"result":{"items":[{"content":""}]}}
JSON
`)

	cmd := exec.Command("python3", "scripts/acm-feature-plan-validate.py")
	cmd.Dir = repoRoot(t)
	cmd.Env = cleanACMEnv(
		"PATH="+binDir+string(os.PathListSeparator)+os.Getenv("PATH"),
		"ACM_PROJECT_ID=agent-context-manager",
		"ACM_RECEIPT_ID=receipt-test",
	)

	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("expected skip for unmaterialized receipt plan, got error %v\n%s", err, string(output))
	}
	if !strings.Contains(string(output), "skip - active plan plan:receipt-test has no materialized content") {
		t.Fatalf("expected unmaterialized-plan skip message, got %s", string(output))
	}
}

func TestACMFeaturePlanValidateStillFailsWhenFeatureParentPlanHasNoContent(t *testing.T) {
	t.Parallel()

	tempRoot := t.TempDir()
	binDir := filepath.Join(tempRoot, "bin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatalf("mkdir bin: %v", err)
	}

	writeExecutable(t, filepath.Join(binDir, "acm"), `#!/usr/bin/env bash
set -euo pipefail
plan_key="${@: -1}"
case "${plan_key}" in
  plan:receipt-test)
    cat <<'JSON'
{"result":{"items":[{"content":"{\"kind\":\"feature_stream\",\"parent_plan_key\":\"plan:root\",\"title\":\"Stream\",\"objective\":\"Validate stream\",\"in_scope\":[\"scope\"],\"out_of_scope\":[\"other\"],\"references\":[\"ref\"],\"tasks\":[{\"key\":\"verify:tests\",\"summary\":\"Verify\"},{\"key\":\"task:leaf\",\"summary\":\"Leaf\",\"acceptance_criteria\":[\"done\"]}]}"}]}}
JSON
    ;;
  plan:root)
    cat <<'JSON'
{"result":{"items":[{"content":""}]}}
JSON
    ;;
  *)
    echo "unexpected plan key: ${plan_key}" >&2
    exit 1
    ;;
esac
`)

	cmd := exec.Command("python3", "scripts/acm-feature-plan-validate.py")
	cmd.Dir = repoRoot(t)
	cmd.Env = cleanACMEnv(
		"PATH="+binDir+string(os.PathListSeparator)+os.Getenv("PATH"),
		"ACM_PROJECT_ID=agent-context-manager",
		"ACM_RECEIPT_ID=receipt-test",
	)

	output, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("expected failure when a feature plan parent is unmaterialized, output=%s", string(output))
	}
	if !strings.Contains(string(output), "fetched plan plan:root had empty content") {
		t.Fatalf("expected strict parent-plan failure, got %s", string(output))
	}
}
