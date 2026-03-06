package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/bonztm/agent-context-manager/internal/adapters/mcp"
	"github.com/bonztm/agent-context-manager/internal/contracts/v1"
	"github.com/bonztm/agent-context-manager/internal/core"
	"github.com/bonztm/agent-context-manager/internal/logging"
	"github.com/bonztm/agent-context-manager/internal/runtime"
)

const mcpInvokeRequestID = "mcp.invoke"

type serviceFactory func(context.Context, logging.Logger) (core.Service, runtime.CleanupFunc, error)

func main() {
	logger := runtime.NewLogger()
	ctx := context.Background()

	if len(os.Args) < 2 {
		logger.Error(ctx, logging.EventACMMCP, "stage", "parse", "ok", false, "error_code", "MISSING_SUBCOMMAND")
		usage()
		os.Exit(2)
	}

	switch os.Args[1] {
	case "tools":
		logger.Info(ctx, logging.EventACMMCP, "stage", "start", "subcommand", "tools")
		printJSON(os.Stdout, map[string]any{"version": v1.Version, "tools": mcp.ToolDefinitions()})
		logger.Info(ctx, logging.EventACMMCP, "stage", "finish", "subcommand", "tools", "exit_code", 0)
		os.Exit(0)
	case "invoke":
		os.Exit(invoke(ctx, logger, os.Args[2:]))
	case "--help", "-h", "help":
		usage()
		os.Exit(0)
	default:
		logger.Error(ctx, logging.EventACMMCP, "stage", "parse", "subcommand", os.Args[1], "ok", false, "error_code", "UNKNOWN_SUBCOMMAND")
		fmt.Fprintf(os.Stderr, "unknown subcommand: %s\n", os.Args[1])
		usage()
		os.Exit(2)
	}
}

func invoke(ctx context.Context, logger logging.Logger, args []string) int {
	return invokeWithDeps(ctx, logger, args, os.Stdin, os.Stdout, time.Now, runtime.NewServiceFromEnvWithLogger)
}

func invokeWithDeps(ctx context.Context, logger logging.Logger, args []string, stdin io.Reader, stdout io.Writer, now func() time.Time, newService serviceFactory) int {
	logger = logging.Normalize(logger)
	logger.Info(ctx, logging.EventACMMCP, "stage", "start", "subcommand", "invoke")

	fs := flag.NewFlagSet("invoke", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	tool := fs.String("tool", "", "tool name: get_context|fetch|propose_memory|report_completion|work|sync|health_check|health_fix|coverage|eval|verify|bootstrap")
	in := fs.String("in", "-", "tool input JSON file or '-' for stdin")
	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return 0
		}
		logger.Error(ctx, logging.EventACMMCP, "stage", "parse_flags", "subcommand", "invoke", "ok", false, "error_code", "INVALID_FLAGS")
		writeEnvelope(stdout, envelopeForTool(*tool, now(), false, nil, &v1.ErrorPayload{Code: "INVALID_FLAGS", Message: err.Error()}))
		return 2
	}
	if *tool == "" {
		logger.Error(ctx, logging.EventACMMCP, "stage", "parse_flags", "subcommand", "invoke", "ok", false, "error_code", "MISSING_TOOL")
		writeEnvelope(stdout, envelopeForTool("", now(), false, nil, &v1.ErrorPayload{Code: "MISSING_TOOL", Message: "--tool is required"}))
		return 2
	}

	input, err := readInput(*in, stdin)
	if err != nil {
		logger.Error(ctx, logging.EventACMIORead, "stage", "read_input", "subcommand", "invoke", "ok", false, "path", *in, "error_code", "READ_FAILED")
		writeEnvelope(stdout, envelopeForTool(*tool, now(), false, nil, &v1.ErrorPayload{Code: "READ_FAILED", Message: err.Error()}))
		return 1
	}
	logger.Info(ctx, logging.EventACMIORead, "stage", "read_input", "subcommand", "invoke", "ok", true, "path", *in, "bytes", len(input))

	svc, closeService, err := newService(ctx, logger)
	if err != nil {
		logger.Error(ctx, logging.EventACMMCP, "stage", "service_init", "subcommand", "invoke", "ok", false, "error_code", "SERVICE_INIT_FAILED")
		writeEnvelope(stdout, envelopeForTool(*tool, now(), false, nil, &v1.ErrorPayload{Code: "SERVICE_INIT_FAILED", Message: err.Error()}))
		return 1
	}
	defer closeService()

	result, apiErr := mcp.InvokeWithLogger(ctx, svc, *tool, input, logger)
	if apiErr != nil {
		logger.Error(ctx, logging.EventACMMCP, "stage", "invoke", "subcommand", "invoke", "ok", false, "tool", *tool, "error_code", apiErr.Code)
		writeEnvelope(stdout, envelopeForTool(*tool, now(), false, nil, apiErr.ToPayload()))
		return 1
	}
	writeEnvelope(stdout, envelopeForTool(*tool, now(), true, result, nil))
	logger.Info(ctx, logging.EventACMMCP, "stage", "finish", "subcommand", "invoke", "tool", *tool, "exit_code", 0)
	return 0
}

func readInput(path string, stdin io.Reader) ([]byte, error) {
	if path == "-" {
		return io.ReadAll(stdin)
	}
	return os.ReadFile(path)
}

func printJSON(out io.Writer, v any) {
	enc := json.NewEncoder(out)
	enc.SetIndent("", "  ")
	_ = enc.Encode(v)
}

func writeEnvelope(out io.Writer, env v1.ResultEnvelope) {
	printJSON(out, env)
}

func envelopeForTool(tool string, now time.Time, ok bool, result any, err *v1.ErrorPayload) v1.ResultEnvelope {
	return v1.ResultEnvelope{
		Version:   v1.Version,
		Command:   v1.Command(tool),
		RequestID: mcpInvokeRequestID,
		OK:        ok,
		Timestamp: now.UTC().Format(time.RFC3339),
		Result:    result,
		Error:     err,
	}
}

func usage() {
	fmt.Println("acm-mcp - MCP adapter surface")
	fmt.Println("")
	fmt.Println("Usage:")
	fmt.Println("  acm-mcp tools")
	fmt.Println("  acm-mcp invoke --tool <name> --in <input.json|->")
	fmt.Println("")
	fmt.Println("MCP v1 tools:")
	fmt.Println("  get_context, fetch, propose_memory, report_completion, work")
	fmt.Println("  sync, health_check, health_fix, coverage, eval, verify, bootstrap")
	fmt.Println("")
	fmt.Println("Config Resolution:")
	fmt.Println("  1. Process environment (`ACM_*`) wins.")
	fmt.Println("  2. Repo-root `.env` is loaded when present.")
	fmt.Println("  3. `ACM_PG_DSN` takes precedence over SQLite.")
	fmt.Println("  4. Default SQLite path is `<repo-root>/.acm/context.db`.")
}
