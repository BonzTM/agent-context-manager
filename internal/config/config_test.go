package config

import (
	"log/slog"
	"os"
	"path/filepath"
	"testing"
)

func TestLoadResolvesExplicitFlagToAbsolutePath(t *testing.T) {
	// An explicit --db wins over everything and is made absolute.
	cfg, err := Load(Options{DBPath: "rel/path/acm.db"})
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if !filepath.IsAbs(cfg.DBPath) {
		t.Fatalf("DBPath not absolute: %q", cfg.DBPath)
	}
	if got := filepath.Base(cfg.DBPath); got != "acm.db" {
		t.Fatalf("DBPath base = %q, want acm.db", got)
	}
}

func TestLoadEnvDBPathTakesPrecedenceOverClaudeProjectDir(t *testing.T) {
	dir := t.TempDir()
	want := filepath.Join(dir, "custom.db")
	t.Setenv(EnvDBPath, want)
	t.Setenv(EnvClaudeProjectDir, filepath.Join(dir, "elsewhere"))

	cfg, err := Load(Options{})
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.DBPath != want {
		t.Fatalf("DBPath = %q, want %q", cfg.DBPath, want)
	}
}

func TestLoadClaudeProjectDirResolvesUnderDotAcm(t *testing.T) {
	dir := t.TempDir()
	t.Setenv(EnvClaudeProjectDir, dir)
	// Ensure LCM_DB is not set in this test's environment.
	t.Setenv(EnvDBPath, "")

	cfg, err := Load(Options{})
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	want := filepath.Join(dir, DirName, DBFileName)
	if cfg.DBPath != want {
		t.Fatalf("DBPath = %q, want %q", cfg.DBPath, want)
	}
	if cfg.ProjectRoot != dir {
		t.Fatalf("ProjectRoot = %q, want %q", cfg.ProjectRoot, dir)
	}
}

func TestLoadWalksUpToAncestorWithDotAcm(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, DirName), 0o750); err != nil {
		t.Fatal(err)
	}
	nested := filepath.Join(root, "a", "b", "c")
	if err := os.MkdirAll(nested, 0o750); err != nil {
		t.Fatal(err)
	}
	// No flag, no env: resolution must walk up from cwd to the ancestor .acm.
	t.Setenv(EnvDBPath, "")
	t.Setenv(EnvClaudeProjectDir, "")
	t.Chdir(nested)

	cfg, err := Load(Options{})
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	want := filepath.Join(root, DirName, DBFileName)
	if cfg.DBPath != want {
		t.Fatalf("DBPath = %q, want %q", cfg.DBPath, want)
	}
}

func TestLoadInvalidLogLevelFails(t *testing.T) {
	if _, err := Load(Options{DBPath: "x.db", LogLevel: "nonsense"}); err == nil {
		t.Fatal("expected error for invalid log level, got nil")
	}
}

func TestLoadDefaultLogLevelIsInfo(t *testing.T) {
	t.Setenv("LOG_LEVEL", "")
	cfg, err := Load(Options{DBPath: "x.db"})
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.LogLevel != slog.LevelInfo {
		t.Fatalf("LogLevel = %v, want info", cfg.LogLevel)
	}
}
