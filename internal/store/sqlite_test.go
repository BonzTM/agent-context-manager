package store

import (
	"context"
	"path/filepath"
	"testing"
)

func TestOpenMigrateAndVersion(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "acm.db")

	db, err := Open(ctx, path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	if err = db.MigrateUp(ctx); err != nil {
		t.Fatalf("MigrateUp: %v", err)
	}

	version, err := db.SchemaVersion(ctx)
	if err != nil {
		t.Fatalf("SchemaVersion: %v", err)
	}
	if version < 1 {
		t.Fatalf("schema version = %d, want >= 1", version)
	}

	// The first migration seeds a known meta row.
	var origin string
	if err := db.SQL().QueryRowContext(ctx,
		`SELECT value FROM meta WHERE key = 'schema_origin'`).Scan(&origin); err != nil {
		t.Fatalf("query meta: %v", err)
	}
	if origin != "agent-context-manager" {
		t.Fatalf("schema_origin = %q, want agent-context-manager", origin)
	}
}

func TestMigrateUpIsIdempotent(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "acm.db")

	db, err := Open(ctx, path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	if err = db.MigrateUp(ctx); err != nil {
		t.Fatalf("MigrateUp (first): %v", err)
	}
	if err = db.MigrateUp(ctx); err != nil {
		t.Fatalf("MigrateUp (second): %v", err)
	}
}

func TestOpenEmptyPathFails(t *testing.T) {
	if _, err := Open(context.Background(), ""); err == nil {
		t.Fatal("expected error for empty path, got nil")
	}
}
