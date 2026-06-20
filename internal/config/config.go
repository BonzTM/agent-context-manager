// Package config resolves and validates all acm process configuration in one
// place. acm is a local CLI, not a long-running service: the central decision
// it must make at startup is *which* per-project SQLite database to operate on,
// because the host agents (Claude Code, Codex, OpenCode) launch acm with
// inconsistent working directories. Resolution precedence and the rationale
// live in golang/foundations/configuration.md; the DB-path precedence chain is
// documented on resolveDBPath.
//
// There are no package-level globals and no init(): everything is built by Load
// and threaded explicitly.
package config

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
)

const (
	// DirName is the per-project acm state directory.
	DirName = ".acm"
	// DBFileName is the SQLite database inside DirName.
	DBFileName = "acm.db"
	// EnvDBPath overrides the resolved database path outright.
	EnvDBPath = "LCM_DB"
	// EnvClaudeProjectDir is set by Claude Code in a spawned tool's environment
	// and points at the project root; we trust it over the working directory.
	EnvClaudeProjectDir = "CLAUDE_PROJECT_DIR"
)

// Options are the raw inputs gathered from CLI flags before resolution. The
// zero value is valid and selects fully defaulted behavior.
type Options struct {
	// DBPath is the value of --db ("" means resolve via the precedence chain).
	DBPath string
	// LogLevel is the value of --log-level ("" means env LOG_LEVEL or "info").
	LogLevel string
	// LogJSON forces JSON logs; env LOG_JSON can also enable it.
	LogJSON bool
}

// Config is the resolved, validated configuration threaded through the process.
type Config struct {
	// DBPath is the absolute path to the SQLite database acm operates on.
	DBPath string
	// ProjectRoot is the directory that owns the .acm state dir (informational;
	// used to scope captured sessions and resolve relative paths).
	ProjectRoot string
	// LogLevel is the minimum slog level.
	LogLevel slog.Level
	// LogJSON selects JSON output (machine collection) over text (local dev).
	LogJSON bool
}

// Load resolves Options into a validated Config. It performs no I/O beyond
// reading the environment and the working directory, and never creates files;
// opening or migrating the database is the caller's separate, explicit step.
func Load(opts Options) (Config, error) {
	dbPath, projectRoot, err := resolveDBPath(opts.DBPath)
	if err != nil {
		return Config{}, err
	}

	levelStr := opts.LogLevel
	if levelStr == "" {
		levelStr = envOr("LOG_LEVEL", "info")
	}
	level, err := parseLevel(levelStr)
	if err != nil {
		return Config{}, err
	}

	cfg := Config{
		DBPath:      dbPath,
		ProjectRoot: projectRoot,
		LogLevel:    level,
		LogJSON:     opts.LogJSON || envBool(os.Getenv("LOG_JSON")),
	}
	if err := cfg.Validate(); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

// Validate enforces the invariants that must hold before acm touches the store.
func (c Config) Validate() error {
	if c.DBPath == "" {
		return errors.New("config: resolved database path is empty")
	}
	if !filepath.IsAbs(c.DBPath) {
		return fmt.Errorf("config: database path must be absolute, got %q", c.DBPath)
	}
	return nil
}

// resolveDBPath selects the SQLite database path. Precedence, highest first:
//
//  1. the --db flag (flagVal),
//  2. the LCM_DB environment variable,
//  3. $CLAUDE_PROJECT_DIR/.acm/acm.db (Claude Code sets this for tools),
//  4. the nearest ancestor directory of the cwd that already contains .acm,
//  5. <cwd>/.acm/acm.db.
//
// Bare cwd is never trusted on its own (step 4 walks up first) because agents
// invoke acm from varying directories; the same project must resolve to the
// same database regardless of where the agent happened to be.
func resolveDBPath(flagVal string) (dbPath, projectRoot string, err error) {
	switch {
	case flagVal != "":
		dbPath, err = filepath.Abs(flagVal)
	case os.Getenv(EnvDBPath) != "":
		dbPath, err = filepath.Abs(os.Getenv(EnvDBPath))
	case os.Getenv(EnvClaudeProjectDir) != "":
		dbPath = filepath.Join(os.Getenv(EnvClaudeProjectDir), DirName, DBFileName)
	default:
		var root string
		root, err = findAncestorWithDir(DirName)
		if err != nil {
			return "", "", err
		}
		dbPath = filepath.Join(root, DirName, DBFileName)
	}
	if err != nil {
		return "", "", fmt.Errorf("config: resolve db path: %w", err)
	}
	return dbPath, deriveProjectRoot(dbPath), nil
}

// deriveProjectRoot returns the directory that owns the .acm dir for dbPath.
// For the canonical .../<root>/.acm/acm.db layout that is <root>; for an
// arbitrary explicit --db path it is simply the file's directory.
func deriveProjectRoot(dbPath string) string {
	parent := filepath.Dir(dbPath)
	if filepath.Base(parent) == DirName {
		return filepath.Dir(parent)
	}
	return parent
}

// findAncestorWithDir walks up from the working directory looking for a
// directory that already contains a child directory named name. It returns that
// ancestor, or the working directory itself if none is found before the
// filesystem root.
func findAncestorWithDir(name string) (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("getwd: %w", err)
	}
	dir := cwd
	for {
		if info, statErr := os.Stat(filepath.Join(dir, name)); statErr == nil && info.IsDir() {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			// Reached the filesystem root without a hit: default to cwd so a
			// fresh project gets its .acm created alongside where acm was run.
			return cwd, nil
		}
		dir = parent
	}
}

func parseLevel(s string) (slog.Level, error) {
	var level slog.Level
	// slog.Level implements encoding.TextUnmarshaler and accepts
	// debug/info/warn/error (case-insensitive).
	if err := level.UnmarshalText([]byte(s)); err != nil {
		return 0, fmt.Errorf("config: invalid log level %q: %w", s, err)
	}
	return level, nil
}

func envOr(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return fallback
}

// envBool parses a boolean-ish environment value; anything unparseable or empty
// is treated as false (the conservative default for an opt-in like JSON logs).
func envBool(v string) bool {
	if v == "" {
		return false
	}
	b, err := strconv.ParseBool(v)
	return err == nil && b
}
