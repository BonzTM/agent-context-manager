package runtime

import (
	"path/filepath"
	"testing"
)

func TestConfigFromEnv_ReadsSQLitePath(t *testing.T) {
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

func TestConfigFromEnv_DefaultSQLitePathIsNonEmpty(t *testing.T) {
	t.Setenv(PostgresDSNEnvVar, "")
	t.Setenv(SQLitePathEnvVar, "")

	cfg := ConfigFromEnv()
	if got := cfg.EffectiveSQLitePath(); got == "" {
		t.Fatal("expected non-empty default sqlite path")
	}
}
