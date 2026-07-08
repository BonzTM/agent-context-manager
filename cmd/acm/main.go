// Command acm is the agent-context-manager entrypoint. main wires only process
// lifecycle: a single root context cancelled on SIGINT/SIGTERM. All real work
// lives in internal/cli, which logs any failure exactly once (through the
// configured logger, so --log-json/--log-level apply) and returns the exit code.
package main

import (
	"context"
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

	return cli.Execute(ctx)
}
