// Package store is the SQLite-backed persistence layer for acm. It owns opening
// the per-project database with deliberate pool limits and PRAGMAs, applying
// the embedded goose migrations, and the typed queries that back the lossless
// store and summary DAG.
//
// SQLite is single-writer: the pool is capped at one connection so reads and
// writes serialize within a process, per golang/services/database.md ("set all
// four pool limits; SQLite is single-writer"). Because multiple acm processes
// can write the same database concurrently (agent hooks fire in parallel),
// every transaction starts as BEGIN IMMEDIATE (_txlock=immediate): the write
// lock is taken up front, where the busy timeout applies, instead of on a
// deferred read->write upgrade, which fails immediately with
// SQLITE_BUSY_SNAPSHOT and would lose the write. WAL + the busy timeout keep
// concurrent processes responsive.
package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	// modernc.org/sqlite registers the pure-Go "sqlite" driver, so acm builds
	// with CGO_ENABLED=0 and ships a static binary (no libsqlite3, no cgo).
	_ "modernc.org/sqlite"
)

// maxConns caps the pool at a single connection. SQLite allows one writer at a
// time; serializing through one connection trades read concurrency (irrelevant
// for a local single-user tool) for the elimination of lock contention.
const maxConns = 1

// pingTimeout bounds the connectivity check so Open fails fast rather than
// hanging on a wedged filesystem.
const pingTimeout = 5 * time.Second

// DB is an open handle to an acm SQLite database. It wraps the *sql.DB pool and
// is safe for concurrent use by the standard library's connection pool.
type DB struct {
	sql  *sql.DB
	path string
}

// Open opens (creating the parent directory and file as needed) the SQLite
// database at path, applies the four pool limits and connection PRAGMAs, and
// verifies connectivity with a bounded ping. The special path ":memory:" opens
// a shared-cache in-memory database for tests.
func Open(ctx context.Context, path string) (*DB, error) {
	if path == "" {
		return nil, errors.New("store: empty database path")
	}
	if path != ":memory:" {
		if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
			return nil, fmt.Errorf("store: create db dir: %w", err)
		}
	}

	sqldb, err := sql.Open("sqlite", dsnFor(path))
	if err != nil {
		return nil, fmt.Errorf("store: open sqlite: %w", err)
	}

	// All four pool limits are set explicitly (never the database/sql defaults).
	// With a single connection the lifetimes only need to prevent needless churn
	// of a local file handle, so they are disabled (0 == no expiry) deliberately.
	sqldb.SetMaxOpenConns(maxConns)
	sqldb.SetMaxIdleConns(maxConns)
	sqldb.SetConnMaxLifetime(0)
	sqldb.SetConnMaxIdleTime(0)

	pingCtx, cancel := context.WithTimeout(ctx, pingTimeout)
	defer cancel()
	if err := sqldb.PingContext(pingCtx); err != nil {
		_ = sqldb.Close()
		return nil, fmt.Errorf("store: ping sqlite: %w", err)
	}
	return &DB{sql: sqldb, path: path}, nil
}

// dsnFor builds a modernc DSN that starts every transaction as BEGIN IMMEDIATE
// (so concurrent processes queue on the busy timeout instead of failing a
// deferred read->write upgrade) and turns on foreign keys, WAL journaling, a
// busy timeout, and NORMAL synchronous mode via the driver's _pragma params.
func dsnFor(path string) string {
	const pragmas = "_txlock=immediate" +
		"&_pragma=busy_timeout(5000)" +
		"&_pragma=journal_mode(WAL)" +
		"&_pragma=foreign_keys(1)" +
		"&_pragma=synchronous(1)"
	if path == ":memory:" {
		// Shared cache keeps the in-memory DB alive across the single pooled
		// connection for the lifetime of the handle.
		return "file::memory:?cache=shared&" + pragmas
	}
	return "file:" + path + "?" + pragmas
}

// SQL returns the underlying pool for query execution by sibling files.
func (d *DB) SQL() *sql.DB { return d.sql }

// Path returns the database path Open was called with.
func (d *DB) Path() string { return d.path }

// Close releases the pool.
func (d *DB) Close() error {
	if err := d.sql.Close(); err != nil {
		return fmt.Errorf("store: close: %w", err)
	}
	return nil
}
