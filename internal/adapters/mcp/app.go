package mcp

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/bonztm/agent-context-manager/internal/buildinfo"
	"github.com/bonztm/agent-context-manager/internal/contracts/v1"
	"github.com/bonztm/agent-context-manager/internal/logging"
	"github.com/bonztm/agent-context-manager/internal/runtime"
)

var newMCPService = runtime.NewServiceFromEnvWithLogger

func RunMCP(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	if stdin == nil {
		stdin = os.Stdin
	}
	if stdout == nil {
		stdout = os.Stdout
	}
	if stderr == nil {
		stderr = os.Stderr
	}

	logger := runtime.NewLogger()
	ctx := context.Background()

	fs := flag.NewFlagSet("acm-mcp", flag.ContinueOnError)
	fs.SetOutput(stderr)
	showHelp := fs.Bool("help", false, "print help and exit")
	fs.BoolVar(showHelp, "h", false, "print help and exit")
	showVersion := fs.Bool("version", false, "print version and exit")
	fs.BoolVar(showVersion, "v", false, "print version and exit")

	if err := fs.Parse(args); err != nil {
		logger.Error(ctx, logging.EventACMMCP, "stage", "parse_flags", "ok", false, "error_code", v1.ErrCodeInvalidFlags)
		return 2
	}
	if *showHelp {
		printUsage(stdout)
		return 0
	}
	if *showVersion {
		printVersion(stdout, "acm-mcp")
		return 0
	}
	if fs.NArg() > 0 {
		logger.Error(ctx, logging.EventACMMCP, "stage", "parse_args", "ok", false, "error_code", v1.ErrCodeUnknownSubcommand)
		fmt.Fprintf(stderr, "unknown arguments: %s\n", strings.Join(fs.Args(), " "))
		printUsage(stdout)
		return 2
	}

	logger.Info(ctx, logging.EventACMMCP, "stage", "start", "mode", "stdio")
	svc, closeService, err := newMCPService(ctx, logger)
	if err != nil {
		logger.Error(ctx, logging.EventACMMCP, "stage", "service_init", "ok", false, "error_code", v1.ErrCodeServiceInitFailed)
		fmt.Fprintf(stderr, "failed to initialize service: %v\n", err)
		return 1
	}
	if closeService != nil {
		defer closeService()
	}

	server := NewServer(svc, logger)
	if err := server.Serve(ctx, stdin, stdout); err != nil {
		logger.Error(ctx, logging.EventACMMCP, "stage", "serve", "ok", false, "error_code", v1.ErrCodeInternalError)
		fmt.Fprintf(stderr, "stdio server failed: %v\n", err)
		return 1
	}

	logger.Info(ctx, logging.EventACMMCP, "stage", "finish", "mode", "stdio", "exit_code", 0)
	return 0
}

func usage() {
	printUsage(os.Stdout)
}

func printUsage(w io.Writer) {
	if w == nil {
		w = os.Stdout
	}
	fmt.Fprintln(w, "acm-mcp - MCP JSON-RPC 2.0 stdio server")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Usage:")
	fmt.Fprintln(w, "  acm-mcp [--help] [--version]")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Protocol:")
	fmt.Fprintln(w, "  Reads one JSON-RPC 2.0 request per line from stdin.")
	fmt.Fprintln(w, "  Writes one compact JSON-RPC 2.0 response per line to stdout.")
	fmt.Fprintln(w, "  Supported methods: initialize, notifications/initialized, tools/list, tools/call")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Config Resolution:")
	fmt.Fprintln(w, "  1. Process environment (`ACM_*`) wins.")
	fmt.Fprintln(w, "  2. Input `project_id` wins when provided.")
	fmt.Fprintln(w, "  3. Otherwise `ACM_PROJECT_ID` sets the default project namespace.")
	fmt.Fprintln(w, "  4. Otherwise the repo-root name is inferred, using `ACM_PROJECT_ROOT` when the shell is elsewhere.")
	fmt.Fprintln(w, "  5. Repo-root `.env` is loaded when present.")
	fmt.Fprintln(w, "  6. `ACM_PG_DSN` takes precedence over SQLite.")
	fmt.Fprintln(w, "  7. Default SQLite path is `<repo-root>/.acm/context.db`.")
	fmt.Fprintln(w, "  8. `ACM_UNBOUNDED=true` removes built-in list caps for supported tools.")
}

func printVersion(w io.Writer, binaryName string) {
	if w == nil {
		w = os.Stdout
	}
	fmt.Fprintln(w, buildinfo.Banner(binaryName))
}
