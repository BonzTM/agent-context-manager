package install

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bonztm/agent-context-manager/internal/core"
)

func readFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(data)
}

func TestClaudeGlobalInstallIsIdempotentAndPreservesKeys(t *testing.T) {
	home := t.TempDir()
	// Pre-existing settings with unrelated keys and an existing hook.
	settings := filepath.Join(home, ".claude", "settings.json")
	if err := os.MkdirAll(filepath.Dir(settings), 0o750); err != nil {
		t.Fatal(err)
	}
	existing := `{"model":"sonnet","permissions":{"allow":["Bash(ls:*)"]},"hooks":{"Stop":[{"hooks":[{"type":"command","command":"echo done"}]}]}}`
	if err := os.WriteFile(settings, []byte(existing), 0o600); err != nil {
		t.Fatal(err)
	}

	// Dry run writes nothing.
	if _, err := Run(core.AgentClaude, home, false); err != nil {
		t.Fatalf("dry run: %v", err)
	}
	if readFile(t, settings) != existing {
		t.Fatal("dry run modified the settings file")
	}

	// Apply.
	res, err := Run(core.AgentClaude, home, true)
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	if len(res.Changes) == 0 {
		t.Fatal("expected changes")
	}

	var m map[string]any
	if err := json.Unmarshal([]byte(readFile(t, settings)), &m); err != nil {
		t.Fatalf("parse merged: %v", err)
	}
	// Unrelated keys preserved.
	if m["model"] != "sonnet" {
		t.Fatal("unrelated 'model' key was lost")
	}
	perms := asMap(t, m["permissions"])
	allow := asSlice(t, perms["allow"])
	if !containsStr(allow, "Bash(ls:*)") || !containsStr(allow, "Bash(acm:*)") {
		t.Fatalf("permissions not merged correctly: %v", allow)
	}
	hooks := asMap(t, m["hooks"])
	if _, ok := hooks["Stop"]; !ok {
		t.Fatal("existing Stop hook was lost")
	}
	if _, ok := hooks["UserPromptSubmit"]; !ok {
		t.Fatal("acm UserPromptSubmit hook not added")
	}

	// Re-applying must not duplicate anything.
	before := readFile(t, settings)
	if _, err := Run(core.AgentClaude, home, true); err != nil {
		t.Fatalf("re-apply: %v", err)
	}
	if readFile(t, settings) != before {
		t.Fatal("second apply changed the file (not idempotent)")
	}
	// And the drill-down instructions landed.
	claudeMd := readFile(t, filepath.Join(home, ".claude", "CLAUDE.md"))
	if !strings.Contains(claudeMd, markerStart) || !strings.Contains(claudeMd, "acm expand") {
		t.Fatalf("CLAUDE.md missing drill-down block:\n%s", claudeMd)
	}
}

func TestOpencodePluginInstalledToAutoLoadDir(t *testing.T) {
	home := t.TempDir()
	t.Setenv("OPENCODE_CONFIG_DIR", "")
	t.Setenv("XDG_CONFIG_HOME", "")

	res, err := Run(core.AgentOpenCode, home, true)
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	if len(res.Changes) == 0 {
		t.Fatal("expected changes")
	}
	path := filepath.Join(home, ".config", "opencode", "plugin", "acm.ts")
	content := readFile(t, path)
	if !strings.Contains(content, "export default") {
		t.Fatalf("plugin file does not look like a plugin:\n%s", content)
	}

	// Re-apply: idempotent (file unchanged).
	if _, err := Run(core.AgentOpenCode, home, true); err != nil {
		t.Fatalf("re-apply: %v", err)
	}
	if readFile(t, path) != content {
		t.Fatal("re-apply changed the plugin file")
	}
}

func TestCodexHooksInstalledAndIdempotent(t *testing.T) {
	home := t.TempDir()
	if _, err := Run(core.AgentCodex, home, true); err != nil {
		t.Fatalf("apply: %v", err)
	}
	path := filepath.Join(home, ".codex", "hooks.json")
	var m map[string]any
	if err := json.Unmarshal([]byte(readFile(t, path)), &m); err != nil {
		t.Fatalf("parse hooks.json: %v", err)
	}
	hooks := asMap(t, m["hooks"])
	for _, ev := range []string{"UserPromptSubmit", "PostToolUse"} {
		if len(asSlice(t, hooks[ev])) == 0 {
			t.Fatalf("no %s hook installed", ev)
		}
	}

	// Idempotent re-apply.
	before := readFile(t, path)
	if _, err := Run(core.AgentCodex, home, true); err != nil {
		t.Fatalf("re-apply: %v", err)
	}
	if readFile(t, path) != before {
		t.Fatal("re-apply changed hooks.json (not idempotent)")
	}
}

func TestCodexNotifyGuarded(t *testing.T) {
	home := t.TempDir()
	cfg := filepath.Join(home, ".codex", "config.toml")
	if err := os.MkdirAll(filepath.Dir(cfg), 0o750); err != nil {
		t.Fatal(err)
	}
	// Existing notify must not be clobbered.
	if err := os.WriteFile(cfg, []byte("notify = [\"mytool\"]\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	res, err := Run(core.AgentCodex, home, true)
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	if !strings.Contains(readFile(t, cfg), `notify = ["mytool"]`) {
		t.Fatal("existing notify was overwritten")
	}
	if strings.Count(readFile(t, cfg), "notify") != 1 {
		t.Fatal("a second notify was added")
	}
	// A note should explain the skipped notify.
	if len(res.Notes) == 0 && !anySkipped(res) {
		t.Fatal("expected the existing-notify case to be surfaced")
	}
}

func TestCodexNotifyAddedWhenAbsent(t *testing.T) {
	home := t.TempDir()
	if _, err := Run(core.AgentCodex, home, true); err != nil {
		t.Fatalf("apply: %v", err)
	}
	cfg := readFile(t, filepath.Join(home, ".codex", "config.toml"))
	if !strings.Contains(cfg, "agent-turn-complete") {
		t.Fatalf("notify not added:\n%s", cfg)
	}
}

func TestInvalidJSONIsNotOverwritten(t *testing.T) {
	home := t.TempDir()
	settings := filepath.Join(home, ".claude", "settings.json")
	if err := os.MkdirAll(filepath.Dir(settings), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(settings, []byte("{ this is not json"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Run(core.AgentClaude, home, true); err == nil {
		t.Fatal("expected an error for invalid JSON")
	}
	if !strings.Contains(readFile(t, settings), "this is not json") {
		t.Fatal("invalid config was overwritten")
	}
}

func asMap(t *testing.T, v any) map[string]any {
	t.Helper()
	m, ok := v.(map[string]any)
	if !ok {
		t.Fatalf("expected map, got %T", v)
	}
	return m
}

func asSlice(t *testing.T, v any) []any {
	t.Helper()
	s, ok := v.([]any)
	if !ok {
		t.Fatalf("expected slice, got %T", v)
	}
	return s
}

func containsStr(s []any, want string) bool {
	for _, e := range s {
		if v, ok := e.(string); ok && v == want {
			return true
		}
	}
	return false
}

func anySkipped(r Result) bool {
	for _, c := range r.Changes {
		if c.Skipped {
			return true
		}
	}
	return false
}
