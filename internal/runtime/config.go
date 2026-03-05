package runtime

import (
	"os"
	"path/filepath"
	"strings"
)

const PostgresDSNEnvVar = "CTX_PG_DSN"
const SQLitePathEnvVar = "CTX_SQLITE_PATH"

type Config struct {
	PostgresDSN string
	SQLitePath  string
}

func ConfigFromEnv() Config {
	return Config{
		PostgresDSN: strings.TrimSpace(os.Getenv(PostgresDSNEnvVar)),
		SQLitePath:  strings.TrimSpace(os.Getenv(SQLitePathEnvVar)),
	}
}

func (c Config) PostgresConfigured() bool {
	return strings.TrimSpace(c.PostgresDSN) != ""
}

func (c Config) EffectiveSQLitePath() string {
	if path := strings.TrimSpace(c.SQLitePath); path != "" {
		return filepath.Clean(path)
	}

	cacheDir, err := os.UserCacheDir()
	if err == nil && strings.TrimSpace(cacheDir) != "" {
		return filepath.Join(cacheDir, "agent-context-manager", "context.db")
	}
	return filepath.Join(os.TempDir(), "agent-context-manager-context.db")
}
