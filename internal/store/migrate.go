package store

import (
	"context"
	"embed"
	"fmt"
	"sync"

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

// gooseMu serializes all goose use: goose configures package-level state
// (SetBaseFS/SetDialect), so concurrent MigrateUp/SchemaVersion calls — e.g.
// parallel tests opening independent stores — would race without it.
var gooseMu sync.Mutex

// MigrateUp applies all pending migrations from the embedded FS, bringing the
// schema to the latest version. acm runs this every time it opens the store:
// it is a local single-user tool, so self-migrating on open keeps the project
// database current without a separate deploy step. It is safe for concurrent
// use; goose's package-level configuration is serialized internally.
func (d *DB) MigrateUp(ctx context.Context) error {
	gooseMu.Lock()
	defer gooseMu.Unlock()
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
	gooseMu.Lock()
	defer gooseMu.Unlock()
	if err := goose.SetDialect(migrationsDialect); err != nil {
		return 0, fmt.Errorf("store: set goose dialect %q: %w", migrationsDialect, err)
	}
	v, err := goose.GetDBVersionContext(ctx, d.sql)
	if err != nil {
		return 0, fmt.Errorf("store: read schema version: %w", err)
	}
	return v, nil
}
