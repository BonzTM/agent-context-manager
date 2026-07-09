package install

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/BurntSushi/toml"

	"github.com/bonztm/agent-context-manager/internal/agents"
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
	assertTopLevelNotify(t, cfg)
	if _, err := Run(core.AgentCodex, home, true); err != nil {
		t.Fatalf("re-apply: %v", err)
	}
	if got := readFile(t, filepath.Join(home, ".codex", "config.toml")); got != cfg {
		t.Fatalf("re-apply changed top-level notify:\n%s", got)
	}
}

func TestCodexNotifyRelocatesLegacyNestedEntry(t *testing.T) {
	home := t.TempDir()
	path := filepath.Join(home, ".codex", "config.toml")
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatal(err)
	}
	legacy := "[hooks.state]\ntrusted = true\n\n" + acmNotifyMarker + " (global)\n" + acmNotifyLine + "\n"
	if err := os.WriteFile(path, []byte(legacy), 0o600); err != nil {
		t.Fatal(err)
	}

	if _, err := Run(core.AgentCodex, home, true); err != nil {
		t.Fatalf("apply: %v", err)
	}
	got := readFile(t, path)
	assertTopLevelNotify(t, got)
	if strings.Count(got, acmNotifyLine) != 1 {
		t.Fatalf("notify count = %d, want 1:\n%s", strings.Count(got, acmNotifyLine), got)
	}
	if !strings.Contains(got, "[hooks.state]\ntrusted = true") {
		t.Fatalf("existing table was not preserved:\n%s", got)
	}
}

func assertTopLevelNotify(t *testing.T, text string) {
	t.Helper()
	var config map[string]any
	if _, err := toml.Decode(text, &config); err != nil {
		t.Fatalf("parse config: %v\n%s", err, text)
	}
	if _, exists := config["notify"]; !exists {
		t.Fatalf("top-level notify missing:\n%s", text)
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

func TestInstructionsWriteThroughSymlink(t *testing.T) {
	home := t.TempDir()
	dotfiles := filepath.Join(home, "dotfiles")
	if err := os.MkdirAll(filepath.Join(home, ".claude"), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(dotfiles, 0o750); err != nil {
		t.Fatal(err)
	}
	realFile := filepath.Join(dotfiles, "CLAUDE.md")
	if err := os.WriteFile(realFile, []byte("# My global instructions\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(home, ".claude", "CLAUDE.md")
	if err := os.Symlink(realFile, link); err != nil {
		t.Fatal(err)
	}

	if _, err := Run(core.AgentClaude, home, true); err != nil {
		t.Fatalf("apply: %v", err)
	}

	// The symlink must survive (not be replaced by a regular file)...
	fi, err := os.Lstat(link)
	if err != nil {
		t.Fatal(err)
	}
	if fi.Mode()&os.ModeSymlink == 0 {
		t.Fatal("CLAUDE.md symlink was orphaned into a regular file")
	}
	// ...the content must have landed in the dotfiles target...
	content := readFile(t, realFile)
	if !strings.Contains(content, markerStart) || !strings.Contains(content, "acm expand") {
		t.Fatalf("dotfiles target missing drill-down block:\n%s", content)
	}
	if !strings.Contains(content, "# My global instructions") {
		t.Fatal("existing dotfiles content was lost")
	}
	// ...and the target's original permissions are preserved.
	tfi, err := os.Stat(realFile)
	if err != nil {
		t.Fatal(err)
	}
	if tfi.Mode().Perm() != 0o644 {
		t.Fatalf("target mode changed to %v, want 0644 preserved", tfi.Mode().Perm())
	}
}

func TestSettingsWriteThroughSymlink(t *testing.T) {
	home := t.TempDir()
	dotfiles := filepath.Join(home, "dotfiles")
	if err := os.MkdirAll(filepath.Join(home, ".claude"), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(dotfiles, 0o750); err != nil {
		t.Fatal(err)
	}
	realFile := filepath.Join(dotfiles, "settings.json")
	if err := os.WriteFile(realFile, []byte(`{"model":"sonnet"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(home, ".claude", "settings.json")
	if err := os.Symlink(realFile, link); err != nil {
		t.Fatal(err)
	}

	if _, err := Run(core.AgentClaude, home, true); err != nil {
		t.Fatalf("apply: %v", err)
	}
	fi, err := os.Lstat(link)
	if err != nil {
		t.Fatal(err)
	}
	if fi.Mode()&os.ModeSymlink == 0 {
		t.Fatal("settings.json symlink was orphaned into a regular file")
	}
	var m map[string]any
	if err := json.Unmarshal([]byte(readFile(t, realFile)), &m); err != nil {
		t.Fatalf("parse target: %v", err)
	}
	if m["model"] != "sonnet" {
		t.Fatal("existing settings lost through symlinked write")
	}
	if _, ok := m["hooks"]; !ok {
		t.Fatal("hooks not written through the symlink")
	}
}

func TestInstructionsSkipUnmanagedManualBlock(t *testing.T) {
	home := t.TempDir()
	claudeMd := filepath.Join(home, ".claude", "CLAUDE.md")
	if err := os.MkdirAll(filepath.Dir(claudeMd), 0o750); err != nil {
		t.Fatal(err)
	}
	// A hand-pasted copy of the drill-down doc, without acm's markers.
	manual := "# Global\n\n" + agents.DrillDownDoc
	if err := os.WriteFile(claudeMd, []byte(manual), 0o600); err != nil {
		t.Fatal(err)
	}

	res, err := Run(core.AgentClaude, home, true)
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	content := readFile(t, claudeMd)
	if strings.Count(content, "## acm — lossless long-context") != 1 {
		t.Fatalf("manual block was duplicated:\n%s", content)
	}
	if strings.Contains(content, markerStart) {
		t.Fatal("acm rewrote a hand-maintained block it should have left alone")
	}
	found := false
	for _, c := range res.Changes {
		if strings.Contains(c.Summary, "added manually") && c.Skipped {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected an unmanaged-block skip notice in changes: %+v", res.Changes)
	}
}

func TestWriteThroughDanglingSymlink(t *testing.T) {
	home := t.TempDir()
	dotfiles := filepath.Join(home, "dotfiles")
	if err := os.MkdirAll(filepath.Join(home, ".claude"), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(dotfiles, 0o750); err != nil {
		t.Fatal(err)
	}
	// Link exists, target does not (fresh dotfiles checkout).
	link := filepath.Join(home, ".claude", "CLAUDE.md")
	target := filepath.Join(dotfiles, "CLAUDE.md")
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}

	if _, err := Run(core.AgentClaude, home, true); err != nil {
		t.Fatalf("apply: %v", err)
	}
	fi, err := os.Lstat(link)
	if err != nil {
		t.Fatal(err)
	}
	if fi.Mode()&os.ModeSymlink == 0 {
		t.Fatal("dangling symlink was replaced instead of populated")
	}
	if !strings.Contains(readFile(t, target), "acm expand") {
		t.Fatal("content not created at the dangling link's target")
	}
}
