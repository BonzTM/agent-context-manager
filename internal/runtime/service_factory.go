package runtime

import (
	"context"
	"fmt"

	"github.com/bonztm/agent-context-manager/internal/adapters/postgres"
	sqliteadapter "github.com/bonztm/agent-context-manager/internal/adapters/sqlite"
	"github.com/bonztm/agent-context-manager/internal/core"
	"github.com/bonztm/agent-context-manager/internal/logging"
	postgressvc "github.com/bonztm/agent-context-manager/internal/service/postgres"
	"github.com/bonztm/agent-context-manager/internal/workspace"
)

type CleanupFunc func()

func NewServiceFromEnv(ctx context.Context) (core.Service, CleanupFunc, error) {
	return NewServiceFromEnvWithLogger(ctx, NewLogger())
}

func NewServiceFromEnvWithLogger(ctx context.Context, logger logging.Logger) (core.Service, CleanupFunc, error) {
	return NewServiceWithLogger(ctx, ConfigFromEnv(), logger)
}

func NewService(ctx context.Context, cfg Config) (core.Service, CleanupFunc, error) {
	return NewServiceWithLogger(ctx, cfg, NewLogger())
}

func NewServiceWithLogger(ctx context.Context, cfg Config, logger logging.Logger) (core.Service, CleanupFunc, error) {
	logger = logging.Normalize(logger)

	if cfg.PostgresConfigured() {
		repo, err := postgres.New(ctx, postgres.Config{DSN: cfg.PostgresDSN})
		if err != nil {
			return nil, nil, fmt.Errorf("initialize postgres repository: %w", err)
		}

		svc, err := postgressvc.New(repo)
		if err != nil {
			repo.Close()
			return nil, nil, fmt.Errorf("initialize postgres service: %w", err)
		}

		return core.WithLogging(svc, logger), func() {
			repo.Close()
		}, nil
	}

	if err := ensureImplicitSQLiteGitIgnore(cfg); err != nil {
		return nil, nil, fmt.Errorf("ensure sqlite gitignore entry: %w", err)
	}

	sqliteRepo, err := sqliteadapter.New(ctx, sqliteadapter.Config{
		Path: cfg.EffectiveSQLitePath(),
	})
	if err != nil {
		return nil, nil, fmt.Errorf("initialize sqlite repository: %w", err)
	}

	svc, err := postgressvc.New(sqliteRepo)
	if err != nil {
		_ = sqliteRepo.Close()
		return nil, nil, fmt.Errorf("initialize sqlite service: %w", err)
	}

	return core.WithLogging(svc, logger), func() {
		_ = sqliteRepo.Close()
	}, nil
}

func ensureImplicitSQLiteGitIgnore(cfg Config) error {
	if !cfg.UsesImplicitSQLitePath() || !cfg.ProjectIsRepo {
		return nil
	}

	relativePath := workspace.RelativePathWithinRoot(cfg.ProjectRoot, cfg.EffectiveSQLitePath())
	if relativePath == "" {
		return nil
	}

	return workspace.EnsureGitIgnoreContains(cfg.ProjectRoot, workspace.SQLiteGitIgnoreEntries(relativePath)...)
}
