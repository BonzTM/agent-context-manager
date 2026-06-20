// Command acm is the agent-context-manager entrypoint. main wires only process
// lifecycle: a single root context cancelled on SIGINT/SIGTERM, and a one-shot
// boundary log of any error before a non-zero exit. All real work lives in
// internal/cli so it is testable and returns errors up to here.
package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/bonztm/agent-context-manager/internal/cli"
)

func main() {
	// os.Exit lives here so that run's deferred cleanup (signal stop) always
	// executes before the process exits.
	os.Exit(run())
}

func run() int {
	// One root context for the whole process; cancelled on the first signal.
	// stop() restores default handling so a second signal kills hard.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := cli.Execute(ctx); err != nil {
		// Errors are wrapped with %w on the way up and logged exactly once here.
		slog.Error("acm failed", "error", err)
		return 1
	}
	return 0
}
