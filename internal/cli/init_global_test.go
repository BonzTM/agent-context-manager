package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInitGlobalDryRunThenApply(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	dbPath := filepath.Join(t.TempDir(), "acm.db")

	// --dry-run writes nothing.
	out := runACM(t, dbPath, "", "init", "claude-code", "--global", "--dry-run")
	if !strings.Contains(out, "Dry run") {
		t.Fatalf("expected dry-run notice, got:\n%s", out)
	}
	if _, err := os.Stat(filepath.Join(home, ".claude", "settings.json")); err == nil {
		t.Fatal("dry run created settings.json")
	}

	// Default (no --dry-run) applies the global config + instructions.
	out = runACM(t, dbPath, "", "init", "claude-code", "--global")
	if !strings.Contains(out, "Installed") {
		t.Fatalf("expected install confirmation, got:\n%s", out)
	}
	if _, err := os.Stat(filepath.Join(home, ".claude", "settings.json")); err != nil {
		t.Fatalf("settings.json not written: %v", err)
	}
	if _, err := os.Stat(filepath.Join(home, ".claude", "CLAUDE.md")); err != nil {
		t.Fatalf("CLAUDE.md not written: %v", err)
	}
}
