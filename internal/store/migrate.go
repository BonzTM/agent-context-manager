package store

import (
	"context"
	"embed"
	"fmt"

	"github.com/pressly/goose/v3"
)

// migrationsFS embeds the goose-tagged SQL migrations so a built binary carries
// its own schema and applies it without shipping loose .sql files, per
// golang/services/database.md (migrations live with the code and run as an
// explicit, ordered step).
//
//go:embed migrations/*.sql
var migrationsFS embed.FS

// migrationsDialect is goose's dialect identifier for SQLite.
const migrationsDialect = "sqlite3"

// MigrateUp applies all pending migrations from the embedded FS, bringing the
// schema to the latest version. acm runs this every time it opens the store:
// it is a local single-user tool, so self-migrating on open keeps the project
// database current without a separate deploy step.
//
// goose configures package-level state (SetBaseFS/SetDialect), so MigrateUp is
// not safe to call concurrently with other goose use; acm opens one store per
// process invocation.
func (d *DB) MigrateUp(ctx context.Context) error {
	goose.SetBaseFS(migrationsFS)
	// Route goose's own chatter away; acm's slog is the single log surface.
	goose.SetLogger(goose.NopLogger())
	if err := goose.SetDialect(migrationsDialect); err != nil {
		return fmt.Errorf("store: set goose dialect %q: %w", migrationsDialect, err)
	}
	if err := goose.UpContext(ctx, d.sql, "migrations"); err != nil {
		return fmt.Errorf("store: apply migrations: %w", err)
	}
	return nil
}

// SchemaVersion reports the current goose schema version applied to the DB.
func (d *DB) SchemaVersion(ctx context.Context) (int64, error) {
	if err := goose.SetDialect(migrationsDialect); err != nil {
		return 0, fmt.Errorf("store: set goose dialect %q: %w", migrationsDialect, err)
	}
	v, err := goose.GetDBVersionContext(ctx, d.sql)
	if err != nil {
		return 0, fmt.Errorf("store: read schema version: %w", err)
	}
	return v, nil
}
