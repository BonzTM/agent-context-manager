// Package cli wires acm's command-line surface. main does nothing but hand the
// signal-aware root context to Execute; every fallible action lives in a RunE so
// it returns an error that main logs exactly once at the process boundary.
//
// acm is a subcommand tree (hook, capture, init, grep, describe, expand, ...),
// which is the case the handbook reserves cobra for; simple single-purpose
// tools would use stdlib flag instead.
package cli

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/bonztm/agent-context-manager/internal/config"
	"github.com/bonztm/agent-context-manager/internal/core"
	"github.com/bonztm/agent-context-manager/internal/engine"
	"github.com/bonztm/agent-context-manager/internal/store"
	"github.com/bonztm/agent-context-manager/internal/summarize"
	"github.com/bonztm/agent-context-manager/internal/tokens"
)

// Command group IDs, used to organize the root help output.
const (
	groupSetup       = "setup"
	groupCapture     = "capture"
	groupRetrieval   = "retrieval"
	groupCompaction  = "compaction"
	groupBatch       = "batch"
	groupDiagnostics = "diagnostics"
)

// app is the shared dependency holder for the command tree. It is populated in
// the root PersistentPreRunE (config + logger) and exposes openStore for the
// commands that need the database.
type app struct {
	opts   config.Options
	cfg    config.Config
	logger *slog.Logger
}

// Execute builds the root command and runs it against ctx. ctx is the
// signal-cancelled root context from main.
func Execute(ctx context.Context) error {
	if err := newRootCmd().ExecuteContext(ctx); err != nil {
		return fmt.Errorf("acm: %w", err)
	}
	return nil
}

func newRootCmd() *cobra.Command {
	a := &app{}

	root := &cobra.Command{
		Use:   "acm",
		Short: "Lossless long-context management for AI coding agents",
		Long: `acm (agent-context-manager) is a local, lossless long-context manager for AI
coding agents — Claude Code, Codex, and OpenCode.

It records every conversation verbatim in a per-project SQLite database under
.acm/, compacts older context into a recoverable hierarchy of summaries under a
token budget, and lets the agent recover any original on demand. There is no
service to run and no network connection — just this binary, your agent's hooks,
and a .acm/ directory in each project.

Getting started:
  acm init claude-code        Generate hook + drill-down integration for an agent.

Concepts:
  conversation   One agent session.            (id prefix: conv_)
  message        A verbatim user/assistant/tool turn.  (id prefix: msg_)
  summary        A compacted span in the summary DAG.  (id prefix: sum_)
  window         The active context: recent raw messages plus summary pointers.

The database is resolved from --db, then $ACM_DB, then $CLAUDE_PROJECT_DIR/.acm,
then the nearest ancestor .acm/ directory. Run 'acm <command> --help' for full
details on any command.`,
		Example: `  acm init claude-code           # wire up an agent
  acm grep "auth refactor"       # search the full history
  acm compact                    # compact under the token budget
  acm expand sum_1a2b3c          # recover a summary's verbatim sources`,
		// Errors are logged once in main; cobra should neither print them nor
		// dump usage on a runtime (non-usage) failure.
		SilenceErrors: true,
		SilenceUsage:  true,
		PersistentPreRunE: func(cmd *cobra.Command, _ []string) error {
			cfg, err := config.Load(a.opts)
			if err != nil {
				return err
			}
			a.cfg = cfg
			a.logger = newLogger(cfg)
			return nil
		},
	}

	pf := root.PersistentFlags()
	pf.StringVar(&a.opts.DBPath, "db", "", "path to the acm SQLite database (default: resolve <project>/.acm/acm.db)")
	pf.StringVar(&a.opts.LogLevel, "log-level", "", "log level: debug|info|warn|error (default info)")
	pf.BoolVar(&a.opts.LogJSON, "log-json", false, "emit JSON logs instead of text")

	root.AddGroup(
		&cobra.Group{ID: groupSetup, Title: "Setup & Integration:"},
		&cobra.Group{ID: groupCapture, Title: "Capture & Recall:"},
		&cobra.Group{ID: groupRetrieval, Title: "Retrieval & Drill-down:"},
		&cobra.Group{ID: groupCompaction, Title: "Compaction:"},
		&cobra.Group{ID: groupBatch, Title: "Batch Processing:"},
		&cobra.Group{ID: groupDiagnostics, Title: "Diagnostics:"},
	)
	root.SetHelpCommandGroupID(groupDiagnostics)
	root.SetCompletionCommandGroupID(groupDiagnostics)

	root.AddCommand(
		newVersionCmd(),
		newDoctorCmd(a),
		newIngestCmd(a),
		newGrepCmd(a),
		newDescribeCmd(a),
		newStatsCmd(a),
		newCompactCmd(a),
		newExpandCmd(a),
		newExpandQueryCmd(a),
		newWindowCmd(a),
		newHookCmd(a),
		newInitCmd(a),
		newMapCmd(a),
	)
	return root
}

// clock is the single time source shared by the store, service, and engine.
var clock core.Clock = core.SystemClock{}

// newStore opens (and migrates) the database, returning the SQLite store and
// the owning handle. The caller must Close the *store.DB.
func (a *app) newStore(ctx context.Context) (*store.SQLite, *store.DB, error) {
	db, err := a.openStore(ctx)
	if err != nil {
		return nil, nil, err
	}
	return store.NewSQLite(db, clock), db, nil
}

// newService wires the core service over the store.
func (a *app) newService(ctx context.Context) (*core.Service, *store.DB, error) {
	sq, db, err := a.newStore(ctx)
	if err != nil {
		return nil, nil, err
	}
	return core.NewService(sq, clock, tokens.Heuristic{}, a.logger), db, nil
}

// newEngine wires the compaction engine over the store with the given config.
// It also returns the SQLite store for commands that read summaries/files
// directly. Pass engine.DefaultConfig() when compaction tuning is irrelevant.
func (a *app) newEngine(ctx context.Context, cfg engine.Config, summarizer core.Summarizer) (*engine.Compactor, *store.SQLite, *store.DB, error) {
	sq, db, err := a.newStore(ctx)
	if err != nil {
		return nil, nil, nil, err
	}
	return a.newCompactor(sq, cfg, summarizer), sq, db, nil
}

// newCompactor wires a compaction engine over an already-open store. A nil
// summarizer selects the deterministic default.
func (a *app) newCompactor(sq *store.SQLite, cfg engine.Config, summarizer core.Summarizer) *engine.Compactor {
	if summarizer == nil {
		summarizer = summarize.Deterministic{}
	}
	filesDir := filepath.Join(filepath.Dir(a.cfg.DBPath), "files")
	return engine.New(sq, summarizer, tokens.Heuristic{}, clock, cfg, filesDir, a.logger)
}

// summarizerByName builds the configured summarizer. "claude"/"codex" reuse the
// host agent's own model via headless CLI, falling back to deterministic.
func summarizerByName(name string) (core.Summarizer, error) {
	det := summarize.Deterministic{}
	switch name {
	case "", "deterministic":
		return det, nil
	case "claude":
		return summarize.Claude(summarize.ExecRunner, det), nil
	case "codex":
		return summarize.Codex(summarize.ExecRunner, det), nil
	default:
		return nil, fmt.Errorf("unknown summarizer %q (want deterministic|claude|codex)", name)
	}
}

// openStore opens the resolved database and brings the schema up to date. acm
// self-migrates on open because it is a local single-user tool; there is no
// separate migration deploy step to gate.
func (a *app) openStore(ctx context.Context) (*store.DB, error) {
	db, err := store.Open(ctx, a.cfg.DBPath)
	if err != nil {
		return nil, err
	}
	if err := db.MigrateUp(ctx); err != nil {
		_ = db.Close()
		return nil, err
	}
	return db, nil
}

// newLogger builds the single structured logger for the process, writing to
// stderr so command stdout stays reserved for machine-readable command output.
func newLogger(cfg config.Config) *slog.Logger {
	opts := &slog.HandlerOptions{Level: cfg.LogLevel}
	var h slog.Handler
	if cfg.LogJSON {
		h = slog.NewJSONHandler(os.Stderr, opts)
	} else {
		h = slog.NewTextHandler(os.Stderr, opts)
	}
	return slog.New(h)
}
