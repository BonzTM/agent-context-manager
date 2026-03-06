package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/bonztm/agent-context-manager/internal/adapters/mcp"
	"github.com/bonztm/agent-context-manager/internal/buildinfo"
	"github.com/bonztm/agent-context-manager/internal/contracts/v1"
	"github.com/bonztm/agent-context-manager/internal/core"
	"github.com/bonztm/agent-context-manager/internal/logging"
	"github.com/bonztm/agent-context-manager/internal/runtime"
)

const mcpInvokeRequestID = "mcp.invoke"

type serviceFactory func(context.Context, logging.Logger) (core.Service, runtime.CleanupFunc, error)

type invokeWrapperEnvelope struct {
	Version   string           `json:"version"`
	RequestID string           `json:"request_id"`
	Tool      string           `json:"tool,omitempty"`
	OK        bool             `json:"ok"`
	Timestamp string           `json:"timestamp"`
	Error     *v1.ErrorPayload `json:"error,omitempty"`
}

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
	case "--version", "-v", "version":
		printVersion(os.Stdout, "acm-mcp")
		os.Exit(0)
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

	if wantsHelp(args) {
		invokeUsage(stdout)
		return 0
	}

	fs := flag.NewFlagSet("invoke", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	tool := fs.String("tool", "", "tool name: get_context|fetch|propose_memory|report_completion|work|history_search|sync|health_check|health_fix|coverage|eval|verify|bootstrap")
	in := fs.String("in", "-", "tool input JSON file or '-' for stdin")
	if err := fs.Parse(args); err != nil {
		logger.Error(ctx, logging.EventACMMCP, "stage", "parse_flags", "subcommand", "invoke", "ok", false, "error_code", "INVALID_FLAGS")
		writeInvokeWrapper(stdout, invokeWrapperError(*tool, now(), "INVALID_FLAGS", err.Error()))
		return 2
	}
	if *tool == "" {
		logger.Error(ctx, logging.EventACMMCP, "stage", "parse_flags", "subcommand", "invoke", "ok", false, "error_code", "MISSING_TOOL")
		writeInvokeWrapper(stdout, invokeWrapperError("", now(), "MISSING_TOOL", "--tool is required"))
		return 2
	}

	command, ok := commandForTool(*tool)
	if !ok {
		logger.Error(ctx, logging.EventACMMCP, "stage", "parse_flags", "subcommand", "invoke", "ok", false, "tool", *tool, "error_code", "UNKNOWN_TOOL")
		writeInvokeWrapper(stdout, invokeWrapperError(*tool, now(), "UNKNOWN_TOOL", "tool is not recognized"))
		return 2
	}

	input, err := readInput(*in, stdin)
	if err != nil {
		logger.Error(ctx, logging.EventACMIORead, "stage", "read_input", "subcommand", "invoke", "ok", false, "path", *in, "error_code", "READ_FAILED")
		writeEnvelope(stdout, envelopeForCommand(command, now(), false, nil, &v1.ErrorPayload{Code: "READ_FAILED", Message: err.Error()}))
		return 1
	}
	logger.Info(ctx, logging.EventACMIORead, "stage", "read_input", "subcommand", "invoke", "ok", true, "path", *in, "bytes", len(input))

	svc, closeService, err := newService(ctx, logger)
	if err != nil {
		logger.Error(ctx, logging.EventACMMCP, "stage", "service_init", "subcommand", "invoke", "ok", false, "error_code", "SERVICE_INIT_FAILED")
		writeEnvelope(stdout, envelopeForCommand(command, now(), false, nil, &v1.ErrorPayload{Code: "SERVICE_INIT_FAILED", Message: err.Error()}))
		return 1
	}
	defer closeService()

	result, apiErr := mcp.InvokeWithLogger(ctx, svc, *tool, input, logger)
	if apiErr != nil {
		logger.Error(ctx, logging.EventACMMCP, "stage", "invoke", "subcommand", "invoke", "ok", false, "tool", *tool, "error_code", apiErr.Code)
		writeEnvelope(stdout, envelopeForCommand(command, now(), false, nil, apiErr.ToPayload()))
		return 1
	}
	writeEnvelope(stdout, envelopeForCommand(command, now(), true, result, nil))
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

func writeInvokeWrapper(out io.Writer, env invokeWrapperEnvelope) {
	printJSON(out, env)
}

func envelopeForCommand(command v1.Command, now time.Time, ok bool, result any, err *v1.ErrorPayload) v1.ResultEnvelope {
	return v1.ResultEnvelope{
		Version:   v1.Version,
		Command:   command,
		RequestID: mcpInvokeRequestID,
		OK:        ok,
		Timestamp: now.UTC().Format(time.RFC3339),
		Result:    result,
		Error:     err,
	}
}

func invokeWrapperError(tool string, now time.Time, code, message string) invokeWrapperEnvelope {
	return invokeWrapperEnvelope{
		Version:   v1.Version,
		RequestID: mcpInvokeRequestID,
		Tool:      strings.TrimSpace(tool),
		OK:        false,
		Timestamp: now.UTC().Format(time.RFC3339),
		Error:     &v1.ErrorPayload{Code: code, Message: message},
	}
}

func wantsHelp(args []string) bool {
	for _, arg := range args {
		switch strings.TrimSpace(arg) {
		case "--help", "-h", "help":
			return true
		}
	}
	return false
}

func commandForTool(tool string) (v1.Command, bool) {
	switch strings.TrimSpace(tool) {
	case string(v1.CommandGetContext):
		return v1.CommandGetContext, true
	case string(v1.CommandFetch):
		return v1.CommandFetch, true
	case string(v1.CommandProposeMemory):
		return v1.CommandProposeMemory, true
	case string(v1.CommandReportCompletion):
		return v1.CommandReportCompletion, true
	case string(v1.CommandWork):
		return v1.CommandWork, true
	case string(v1.CommandHistorySearch):
		return v1.CommandHistorySearch, true
	case string(v1.CommandSync):
		return v1.CommandSync, true
	case string(v1.CommandHealthCheck):
		return v1.CommandHealthCheck, true
	case string(v1.CommandHealthFix):
		return v1.CommandHealthFix, true
	case string(v1.CommandCoverage):
		return v1.CommandCoverage, true
	case string(v1.CommandEval):
		return v1.CommandEval, true
	case string(v1.CommandVerify):
		return v1.CommandVerify, true
	case string(v1.CommandBootstrap):
		return v1.CommandBootstrap, true
	default:
		return "", false
	}
}

func usage() {
	printUsage(os.Stdout)
}

func printUsage(w io.Writer) {
	if w == nil {
		w = os.Stdout
	}
	fmt.Fprintln(w, "acm-mcp - MCP adapter surface")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Usage:")
	fmt.Fprintln(w, "  acm-mcp tools")
	fmt.Fprintln(w, "  acm-mcp invoke --tool <name> --in <input.json|->")
	fmt.Fprintln(w, "  acm-mcp --version | -v")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "MCP v1 tools:")
	fmt.Fprintln(w, "  get_context, fetch, propose_memory, report_completion, work, history_search")
	fmt.Fprintln(w, "  sync, health_check, health_fix, coverage, eval, verify, bootstrap")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Config Resolution:")
	fmt.Fprintln(w, "  1. Process environment (`ACM_*`) wins.")
	fmt.Fprintln(w, "  2. `ACM_PROJECT_ROOT` can pin the project root when the current shell is elsewhere.")
	fmt.Fprintln(w, "  3. Repo-root `.env` is loaded when present.")
	fmt.Fprintln(w, "  4. `ACM_PG_DSN` takes precedence over SQLite.")
	fmt.Fprintln(w, "  5. Default SQLite path is `<repo-root>/.acm/context.db`.")
	fmt.Fprintln(w, "  6. `ACM_UNBOUNDED=true` removes built-in retrieval/list caps for supported tools.")
}

func printVersion(w io.Writer, binaryName string) {
	if w == nil {
		w = os.Stdout
	}
	fmt.Fprintln(w, buildinfo.Banner(binaryName))
}

func invokeUsage(w io.Writer) {
	fmt.Fprintln(w, "acm-mcp invoke - invoke one MCP tool with JSON input")
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "Usage:")
	fmt.Fprintln(w, "  acm-mcp invoke --tool <name> [--in <input.json|->]")
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "Flags:")
	fmt.Fprintln(w, "  --tool <name>   required MCP tool name")
	fmt.Fprintln(w, "  --in <path>     JSON input file or '-' for stdin (default: -)")
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "Tool names:")
	fmt.Fprintln(w, "  get_context, fetch, propose_memory, report_completion, work, history_search")
	fmt.Fprintln(w, "  sync, health_check, health_fix, coverage, eval, verify, bootstrap")
}
