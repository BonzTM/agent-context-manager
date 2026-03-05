package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/joshd/agent-context-manager/internal/adapters/cli"
	"github.com/joshd/agent-context-manager/internal/contracts/v1"
	"github.com/joshd/agent-context-manager/internal/logging"
	"github.com/joshd/agent-context-manager/internal/runtime"
)

func main() {
	logger := runtime.NewLogger()
	ctx := context.Background()

	if len(os.Args) < 2 {
		logger.Error(ctx, logging.EventACMRun, "stage", "parse", "ok", false, "error_code", "MISSING_SUBCOMMAND")
		usage()
		os.Exit(2)
	}

	switch os.Args[1] {
	case "run":
		os.Exit(run(ctx, logger, os.Args[2:]))
	case "validate":
		os.Exit(validate(ctx, logger, os.Args[2:]))
	case "get-context",
		"fetch",
		"propose-memory",
		"work",
		"report-completion",
		"sync",
		"health",
		"health-check",
		"health-fix",
		"coverage",
		"regress",
		"bootstrap":
		os.Exit(runConvenience(ctx, logger, os.Args[1], os.Args[2:]))
	case "--help", "-h", "help":
		usage()
		os.Exit(0)
	default:
		logger.Error(ctx, logging.EventACMRun, "stage", "parse", "subcommand", os.Args[1], "ok", false, "error_code", "UNKNOWN_SUBCOMMAND")
		fmt.Fprintf(os.Stderr, "unknown subcommand: %s\n", os.Args[1])
		usage()
		os.Exit(2)
	}
}

func run(ctx context.Context, logger logging.Logger, args []string) int {
	logger = logging.Normalize(logger)
	logger.Info(ctx, logging.EventACMRun, "stage", "start", "subcommand", "run")

	fs := flag.NewFlagSet("run", flag.ContinueOnError)
	inPath := fs.String("in", "-", "input request file path or '-' for stdin")
	if err := fs.Parse(args); err != nil {
		logger.Error(ctx, logging.EventACMRun, "stage", "parse_flags", "subcommand", "run", "ok", false, "error_code", "INVALID_FLAGS")
		return 2
	}

	in, closeInput, err := openInput(*inPath)
	if err != nil {
		logger.Error(ctx, logging.EventACMIORead, "stage", "open_input", "subcommand", "run", "ok", false, "path", *inPath, "error_code", "READ_FAILED")
		fmt.Fprintf(os.Stderr, "failed to open input: %v\n", err)
		return 2
	}
	defer closeInput()
	logger.Info(ctx, logging.EventACMIORead, "stage", "open_input", "subcommand", "run", "ok", true, "path", *inPath)

	svc, closeService, err := runtime.NewServiceFromEnvWithLogger(ctx, logger)
	if err != nil {
		logger.Error(ctx, logging.EventACMRun, "stage", "service_init", "subcommand", "run", "ok", false, "error_code", "SERVICE_INIT_FAILED")
		fmt.Fprintf(os.Stderr, "failed to initialize service: %v\n", err)
		return 1
	}
	defer closeService()

	code := cli.RunWithLogger(ctx, svc, in, os.Stdout, time.Now, logger)
	logger.Info(ctx, logging.EventACMRun, "stage", "finish", "subcommand", "run", "exit_code", code)
	return code
}

func validate(ctx context.Context, logger logging.Logger, args []string) int {
	logger = logging.Normalize(logger)
	logger.Info(ctx, logging.EventACMRun, "stage", "start", "subcommand", "validate")

	fs := flag.NewFlagSet("validate", flag.ContinueOnError)
	inPath := fs.String("in", "-", "input request file path or '-' for stdin")
	if err := fs.Parse(args); err != nil {
		logger.Error(ctx, logging.EventACMRun, "stage", "parse_flags", "subcommand", "validate", "ok", false, "error_code", "INVALID_FLAGS")
		return 2
	}

	in, closeFn, err := openInput(*inPath)
	if err != nil {
		logger.Error(ctx, logging.EventACMIORead, "stage", "open_input", "subcommand", "validate", "ok", false, "path", *inPath, "error_code", "READ_FAILED")
		fmt.Fprintf(os.Stderr, "failed to open input: %v\n", err)
		return 2
	}
	defer closeFn()
	logger.Info(ctx, logging.EventACMIORead, "stage", "open_input", "subcommand", "validate", "ok", true, "path", *inPath)

	b, err := io.ReadAll(in)
	if err != nil {
		logger.Error(ctx, logging.EventACMIORead, "stage", "read_input", "subcommand", "validate", "ok", false, "error_code", "READ_FAILED")
		fmt.Fprintf(os.Stderr, "failed to read input: %v\n", err)
		return 2
	}
	logger.Info(ctx, logging.EventACMIORead, "stage", "read_input", "subcommand", "validate", "ok", true, "bytes", len(b))
	_, _, valErr := v1.DecodeAndValidateCommand(b)
	if valErr != nil {
		logger.Error(ctx, logging.EventACMRun, "stage", "validate", "subcommand", "validate", "ok", false, "error_code", valErr.Code)
		fmt.Printf("{\n  \"ok\": false,\n  \"error\": {\n    \"code\": %q,\n    \"message\": %q\n  }\n}\n", valErr.Code, valErr.Message)
		return 1
	}
	logger.Info(ctx, logging.EventACMRun, "stage", "validate", "subcommand", "validate", "ok", true)
	fmt.Println("{\n  \"ok\": true\n}")
	logger.Info(ctx, logging.EventACMRun, "stage", "finish", "subcommand", "validate", "exit_code", 0)
	return 0
}

func openInput(path string) (io.Reader, func(), error) {
	if path == "-" {
		return os.Stdin, func() {}, nil
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, nil, err
	}
	return f, func() { _ = f.Close() }, nil
}

func usage() {
	fmt.Println("acm - agent context manager CLI")
	fmt.Println("")
	fmt.Println("Usage:")
	fmt.Println("  acm run --in <request.json|->")
	fmt.Println("  acm validate --in <request.json|->")
	fmt.Println("  acm get-context [flags]")
	fmt.Println("  acm fetch [flags]")
	fmt.Println("  acm propose-memory [flags]")
	fmt.Println("  acm work [flags]")
	fmt.Println("  acm report-completion [flags]")
	fmt.Println("  acm sync [flags]")
	fmt.Println("  acm health [flags]  # alias for health-check")
	fmt.Println("  acm health-check [flags]")
	fmt.Println("  acm health-fix [flags]")
	fmt.Println("  acm coverage [flags]")
	fmt.Println("  acm regress [flags]")
	fmt.Println("  acm bootstrap [flags]")
	fmt.Println("")
	fmt.Println("Convenience command examples:")
	fmt.Println("  # Context retrieval")
	fmt.Println("  acm get-context --project myproject --task-text \"Add sync checks\" --phase execute")
	fmt.Println("  acm fetch --project myproject --key plan:req-12345678 --expect plan:req-12345678=v3")
	fmt.Println("  # Work and completion")
	fmt.Println("  acm work --project myproject --receipt-id req-12345678 --tasks-json '[{\"key\":\"verify:tests\",\"summary\":\"Run tests\",\"status\":\"pending\"}]'")
	fmt.Println("  acm report-completion --project myproject --receipt-id req-12345678 --file-changed cmd/acm/main.go --outcome \"Implemented command\"")
	fmt.Println("  # Maintenance")
	fmt.Println("  acm sync --project myproject --mode changed --git-range HEAD~1..HEAD")
	fmt.Println("  acm health --project myproject --include-details")
	fmt.Println("  acm bootstrap --project myproject --project-root . --persist-candidates")
}
