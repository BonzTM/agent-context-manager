package runtime

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/bonztm/agent-context-manager/internal/workspace"
)

func TestConfigFromEnv_ReadsExplicitSQLitePath(t *testing.T) {
	t.Setenv(PostgresDSNEnvVar, "")
	t.Setenv(SQLitePathEnvVar, " /tmp/custom-sqlite.db ")

	cfg := ConfigFromEnv()
	if cfg.PostgresConfigured() {
		t.Fatalf("expected postgres to be unconfigured")
	}
	if got, want := cfg.EffectiveSQLitePath(), filepath.Clean("/tmp/custom-sqlite.db"); got != want {
		t.Fatalf("unexpected sqlite path: got %q want %q", got, want)
	}
}

func TestConfigFromEnv_DefaultSQLitePathUsesRepoRoot(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, ".git"), 0o755); err != nil {
		t.Fatalf("mkdir .git: %v", err)
	}
	subdir := filepath.Join(root, "internal", "runtime")
	if err := os.MkdirAll(subdir, 0o755); err != nil {
		t.Fatalf("mkdir subdir: %v", err)
	}

	restore := withWorkingDir(t, subdir)
	defer restore()

	t.Setenv(PostgresDSNEnvVar, "")
	t.Setenv(SQLitePathEnvVar, "")

	cfg := ConfigFromEnv()
	if !cfg.ProjectIsRepo {
		t.Fatal("expected repo root detection")
	}
	if got, want := cfg.ProjectRoot, root; got != want {
		t.Fatalf("unexpected project root: got %q want %q", got, want)
	}
	if got, want := cfg.EffectiveSQLitePath(), filepath.Join(root, filepath.FromSlash(workspace.DefaultSQLiteRelativePath)); got != want {
		t.Fatalf("unexpected sqlite path: got %q want %q", got, want)
	}
}

func TestConfigFromEnv_LoadsDotEnvFromRepoRoot(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, ".git"), 0o755); err != nil {
		t.Fatalf("mkdir .git: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, ".env"), []byte("ACM_PG_DSN=postgres://ctx:ctx@localhost:5432/acm?sslmode=disable\nACM_SQLITE_PATH=.acm/custom.db\n"), 0o644); err != nil {
		t.Fatalf("write .env: %v", err)
	}

	restore := withWorkingDir(t, root)
	defer restore()

	unsetEnv(t, PostgresDSNEnvVar)
	unsetEnv(t, SQLitePathEnvVar)

	cfg := ConfigFromEnv()
	if got, want := cfg.PostgresDSN, "postgres://ctx:ctx@localhost:5432/acm?sslmode=disable"; got != want {
		t.Fatalf("unexpected postgres dsn: got %q want %q", got, want)
	}
	if got, want := cfg.EffectiveSQLitePath(), filepath.Join(root, ".acm", "custom.db"); got != want {
		t.Fatalf("unexpected sqlite path from .env: got %q want %q", got, want)
	}
}

func TestConfigFromEnv_ProcessEnvOverridesDotEnv(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, ".git"), 0o755); err != nil {
		t.Fatalf("mkdir .git: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, ".env"), []byte("ACM_PG_DSN=postgres://dot-env\n"), 0o644); err != nil {
		t.Fatalf("write .env: %v", err)
	}

	restore := withWorkingDir(t, root)
	defer restore()

	t.Setenv(PostgresDSNEnvVar, "postgres://process-env")

	cfg := ConfigFromEnv()
	if got, want := cfg.PostgresDSN, "postgres://process-env"; got != want {
		t.Fatalf("unexpected postgres dsn: got %q want %q", got, want)
	}
}

func withWorkingDir(t *testing.T, dir string) func() {
	t.Helper()

	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir %q: %v", dir, err)
	}
	return func() {
		if err := os.Chdir(wd); err != nil {
			t.Fatalf("restore wd: %v", err)
		}
	}
}

func unsetEnv(t *testing.T, key string) {
	t.Helper()

	previous, hadPrevious := os.LookupEnv(key)
	if err := os.Unsetenv(key); err != nil {
		t.Fatalf("unsetenv %s: %v", key, err)
	}
	t.Cleanup(func() {
		var err error
		if hadPrevious {
			err = os.Setenv(key, previous)
		} else {
			err = os.Unsetenv(key)
		}
		if err != nil {
			t.Fatalf("restore env %s: %v", key, err)
		}
	})
}
