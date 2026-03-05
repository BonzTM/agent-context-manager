package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/joshd/agent-context-manager/internal/adapters/mcp"
	"github.com/joshd/agent-context-manager/internal/contracts/v1"
	"github.com/joshd/agent-context-manager/internal/logging"
	"github.com/joshd/agent-context-manager/internal/runtime"
)

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
		printJSON(map[string]any{"version": v1.Version, "tools": mcp.ToolDefinitions()})
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
	logger = logging.Normalize(logger)
	logger.Info(ctx, logging.EventACMMCP, "stage", "start", "subcommand", "invoke")

	fs := flag.NewFlagSet("invoke", flag.ContinueOnError)
	tool := fs.String("tool", "", "tool name: get_context|fetch|work|propose_memory|report_completion")
	in := fs.String("in", "-", "tool input JSON file or '-' for stdin")
	if err := fs.Parse(args); err != nil {
		logger.Error(ctx, logging.EventACMMCP, "stage", "parse_flags", "subcommand", "invoke", "ok", false, "error_code", "INVALID_FLAGS")
		return 2
	}
	if *tool == "" {
		logger.Error(ctx, logging.EventACMMCP, "stage", "parse_flags", "subcommand", "invoke", "ok", false, "error_code", "MISSING_TOOL")
		fmt.Fprintln(os.Stderr, "--tool is required")
		return 2
	}

	input, err := readInput(*in)
	if err != nil {
		logger.Error(ctx, logging.EventACMIORead, "stage", "read_input", "subcommand", "invoke", "ok", false, "path", *in, "error_code", "READ_FAILED")
		printJSON(map[string]any{"ok": false, "error": map[string]any{"code": "READ_FAILED", "message": err.Error()}})
		return 1
	}
	logger.Info(ctx, logging.EventACMIORead, "stage", "read_input", "subcommand", "invoke", "ok", true, "path", *in, "bytes", len(input))

	svc, closeService, err := runtime.NewServiceFromEnvWithLogger(ctx, logger)
	if err != nil {
		logger.Error(ctx, logging.EventACMMCP, "stage", "service_init", "subcommand", "invoke", "ok", false, "error_code", "SERVICE_INIT_FAILED")
		printJSON(map[string]any{"ok": false, "error": map[string]any{"code": "SERVICE_INIT_FAILED", "message": err.Error()}})
		return 1
	}
	defer closeService()

	result, apiErr := mcp.InvokeWithLogger(ctx, svc, *tool, input, logger)
	if apiErr != nil {
		logger.Error(ctx, logging.EventACMMCP, "stage", "invoke", "subcommand", "invoke", "ok", false, "tool", *tool, "error_code", apiErr.Code)
		printJSON(map[string]any{"ok": false, "error": apiErr.ToPayload()})
		return 1
	}
	printJSON(map[string]any{"ok": true, "result": result})
	logger.Info(ctx, logging.EventACMMCP, "stage", "finish", "subcommand", "invoke", "tool", *tool, "exit_code", 0)
	return 0
}

func readInput(path string) ([]byte, error) {
	if path == "-" {
		return io.ReadAll(os.Stdin)
	}
	return os.ReadFile(path)
}

func printJSON(v any) {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	_ = enc.Encode(v)
}

func usage() {
	fmt.Println("acm-mcp - MCP adapter surface")
	fmt.Println("")
	fmt.Println("Usage:")
	fmt.Println("  acm-mcp tools")
	fmt.Println("  acm-mcp invoke --tool <name> --in <input.json|->")
	fmt.Println("")
	fmt.Println("MCP v1.1 tools (index-first):")
	fmt.Println("  get_context, fetch, work, propose_memory, report_completion")
}
