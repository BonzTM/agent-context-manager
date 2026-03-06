package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/bonztm/agent-context-manager/internal/adapters/cli"
	"github.com/bonztm/agent-context-manager/internal/buildinfo"
	"github.com/bonztm/agent-context-manager/internal/contracts/v1"
	"github.com/bonztm/agent-context-manager/internal/logging"
	"github.com/bonztm/agent-context-manager/internal/runtime"
)

type helpCommand struct {
	usage   string
	summary string
}

var entryPointCommands = []helpCommand{
	{
		usage:   "acm run --in <request.json|->",
		summary: "Execute a full v1 command envelope from stdin or a file.",
	},
	{
		usage:   "acm validate --in <request.json|->",
		summary: "Validate a full v1 command envelope without executing it.",
	},
}

var workflowCommands = []helpCommand{
	{
		usage:   "acm get-context --project <id> [--task-text <text>|--task-file <path>] [--tags-file <path>] [--unbounded[=true|false]] [flags]",
		summary: "Resolve a scoped receipt with rules, pointers, memories, and active work.",
	},
	{
		usage:   "acm fetch --project <id> [--key <pointer>]... [--keys-file <path>] [--keys-json <json>] [--receipt-id <id>] [--expect <key=version>]... [--expected-versions-file <path>] [--expected-versions-json <json>]",
		summary: "Fetch pointer, plan, or task content by key, with optional version checks.",
	},
	{
		usage:   "acm propose-memory --project <id> --receipt-id <id> --category <name> --subject <text> (--content <text>|--content-file <path>) --confidence <1-5> [--memory-tag <tag>]... [--memory-tags-file <path>|--memory-tags-json <json>] [--tags-file <path>] [flags]",
		summary: "Propose durable memory tied to a receipt, evidence, and canonical tags.",
	},
	{
		usage:   "acm work --project <id> [--plan-key <key>|--receipt-id <id>] [--plan-title <text>] [--mode <merge|replace>] [--plan-file <path>|--plan-json <json>] [--tasks-file <path>|--tasks-json <json>] [--items-file <path>|--items-json <json>]",
		summary: "Create or update structured plans and tasks that survive compaction.",
	},
	{
		usage:   "acm work list --project <id> [--scope <current|deferred|completed|all>] [--kind <kind>] [--limit <n>] [--unbounded[=true|false]]",
		summary: "List compact plan summaries for current, deferred, completed, or all work.",
	},
	{
		usage:   "acm work search --project <id> (--query <text>|--query-file <path>) [--scope <current|deferred|completed|all>] [--kind <kind>] [--limit <n>] [--unbounded[=true|false]]",
		summary: "Search plan and task history without direct database access.",
	},
	{
		usage:   "acm history search --project <id> [--entity <all|work|memory|receipt|run>] [--query <text>|--query-file <path>] [--scope <current|deferred|completed|all>] [--kind <kind>] [--limit <n>] [--unbounded[=true|false]]",
		summary: "Search recent work, memory, receipt, and run history without direct database access.",
	},
	{
		usage:   "acm report-completion --project <id> --receipt-id <id> [--outcome <text>|--outcome-file <path>] [--file-changed <path>]... [--files-changed-file <path>] [--files-changed-json <json>] [--scope-mode <mode>] [--tags-file <path>]",
		summary: "Close a receipt, validate scope, and persist the completion summary.",
	},
}

var maintenanceCommands = []helpCommand{
	{
		usage:   "acm sync --project <id> [--mode changed|full|working_tree] [--git-range <range>] [--project-root <path>] [--rules-file <path>] [--tags-file <path>] [--insert-new-candidates[=true|false]]",
		summary: "Refresh repository pointers and canonical rules from the working tree or git history.",
	},
	{
		usage:   "acm health [flags]",
		summary: "Alias for `acm health-check`.",
	},
	{
		usage:   "acm health-check --project <id> [--include-details[=true|false]] [--max-findings-per-check <n>]",
		summary: "Inspect repository health without making changes.",
	},
	{
		usage:   "acm health-fix --project <id> [--apply[=true|false>] [--project-root <path>] [--rules-file <path>] [--tags-file <path>] [--fixer <name>]...",
		summary: "Plan or apply repair actions such as sync_working_tree, index_uncovered_files, and sync_ruleset.",
	},
	{
		usage:   "acm coverage --project <id> [--project-root <path>]",
		summary: "Measure repository indexing coverage against the current project tree.",
	},
	{
		usage:   "acm eval --project <id> (--eval-suite-path <path> | --eval-suite-inline-file <path> | --eval-suite-inline-json <json>) [--minimum-recall <0..1>] [--tags-file <path>]",
		summary: "Run retrieval-quality evaluation cases against ACM context selection.",
	},
	{
		usage:   "acm verify --project <id> [--receipt-id <id>] [--plan-key <key>] [--phase <plan|execute|review>] [--test-id <id>]... [--file-changed <path>]... [--files-changed-file <path>|--files-changed-json <json>] [--tests-file <path>] [--tags-file <path>] [--dry-run]",
		summary: "Select and execute repo-defined verification checks from `.acm/acm-tests.yaml` or `acm-tests.yaml`.",
	},
	{
		usage:   "acm bootstrap --project <id> --project-root <path> [--rules-file <path>] [--tags-file <path>] [--persist-candidates[=true|false]] [--respect-gitignore[=true|false]] [--llm-assist-descriptions[=true|false]] [--output-candidates-path <path>]",
		summary: "Seed repo-local ACM files and scan a repository for initial pointer candidates.",
	},
}

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
	case "work":
		if nested, ok := nestedConvenienceSubcommand(os.Args[1], os.Args[2:]); ok {
			os.Exit(runConvenience(ctx, logger, nested, os.Args[3:]))
		}
		os.Exit(runConvenience(ctx, logger, os.Args[1], os.Args[2:]))
	case "history":
		if nested, ok := nestedConvenienceSubcommand(os.Args[1], os.Args[2:]); ok {
			os.Exit(runConvenience(ctx, logger, nested, os.Args[3:]))
		}
		logger.Error(ctx, logging.EventACMRun, "stage", "parse", "subcommand", "history", "ok", false, "error_code", "UNKNOWN_SUBCOMMAND")
		fmt.Fprintln(os.Stderr, "history requires a nested subcommand such as `search`")
		usage()
		os.Exit(2)
	case "get-context",
		"fetch",
		"propose-memory",
		"report-completion",
		"sync",
		"health",
		"health-check",
		"health-fix",
		"coverage",
		"eval",
		"verify",
		"bootstrap":
		os.Exit(runConvenience(ctx, logger, os.Args[1], os.Args[2:]))
	case "--version", "-v", "version":
		printVersion(os.Stdout, "acm")
		os.Exit(0)
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

func nestedConvenienceSubcommand(command string, args []string) (string, bool) {
	if len(args) == 0 {
		return "", false
	}
	if strings.HasPrefix(args[0], "-") {
		return "", false
	}
	switch command {
	case "work":
		switch args[0] {
		case "list":
			return "work-list", true
		case "search":
			return "work-search", true
		}
	case "history":
		switch args[0] {
		case "search":
			return "history-search", true
		}
	}
	return "", false
}

func run(ctx context.Context, logger logging.Logger, args []string) int {
	logger = logging.Normalize(logger)
	logger.Info(ctx, logging.EventACMRun, "stage", "start", "subcommand", "run")

	fs := flag.NewFlagSet("run", flag.ContinueOnError)
	configureRunUsage(fs)
	inPath := fs.String("in", "-", "input request file path or '-' for stdin")
	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return 0
		}
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
	configureValidateUsage(fs)
	inPath := fs.String("in", "-", "input request file path or '-' for stdin")
	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return 0
		}
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
	printMainUsage(os.Stdout)
}

func printVersion(w io.Writer, binaryName string) {
	if w == nil {
		w = os.Stdout
	}
	fmt.Fprintln(w, buildinfo.Banner(binaryName))
}

func printMainUsage(w io.Writer) {
	if w == nil {
		w = os.Stdout
	}

	fmt.Fprintln(w, "acm - agent context manager CLI")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Usage:")
	fmt.Fprintln(w, "  acm <command> [flags]")
	fmt.Fprintln(w, "  acm --version | -v")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Entry Points:")
	printHelpCommands(w, entryPointCommands)
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Agent Workflow Commands:")
	printHelpCommands(w, workflowCommands)
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Maintenance Commands:")
	printHelpCommands(w, maintenanceCommands)
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Shared Conventions:")
	fmt.Fprintln(w, "  - Every convenience command requires `--project` or `--project-id`.")
	fmt.Fprintln(w, "  - Run `acm <subcommand> --help` for exhaustive flags and examples for one command.")
	fmt.Fprintln(w, "  - `--project` and `--project-id` are aliases on convenience commands.")
	fmt.Fprintln(w, "  - `--request` and `--request-id` are aliases on convenience commands.")
	fmt.Fprintln(w, "  - Most text/list payloads support inline values, `--*-json`, and `--*-file` variants.")
	fmt.Fprintln(w, "  - `-` means stdin for `--in` and file-backed flags.")
	fmt.Fprintln(w, "  - Repeatable flags may be provided more than once.")
	fmt.Fprintln(w, "  - Optional bool flags accept `--flag`, `--flag=true`, or `--flag=false`.")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "High-Signal Requirements:")
	fmt.Fprintln(w, "  - `get-context` requires one of `--task-text` or `--task-file`.")
	fmt.Fprintln(w, "  - `propose-memory` requires `--receipt-id`, `--category`, `--subject`, `--confidence`, and one of `--content` or `--content-file`.")
	fmt.Fprintln(w, "  - `report-completion` requires `--receipt-id` and one of `--outcome` or `--outcome-file`.")
	fmt.Fprintln(w, "  - `eval` requires exactly one of `--eval-suite-path`, `--eval-suite-inline-file`, or `--eval-suite-inline-json`.")
	fmt.Fprintln(w, "  - `work` enforces exclusive payload groups such as `--plan-file|--plan-json`, `--tasks-file|--tasks-json`, and `--items-file|--items-json`.")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Config Resolution:")
	fmt.Fprintln(w, "  1. Process environment (`ACM_*`) wins.")
	fmt.Fprintln(w, "  2. Repo-root `.env` is loaded when present.")
	fmt.Fprintln(w, "  3. If `ACM_PG_DSN` is set, Postgres is used.")
	fmt.Fprintln(w, "  4. Otherwise SQLite defaults to `<repo-root>/.acm/context.db`.")
	fmt.Fprintln(w, "  5. Outside a repo, SQLite defaults to `<cwd>/.acm/context.db`.")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Environment Variables:")
	fmt.Fprintln(w, "  - `ACM_PG_DSN`: Postgres DSN. If set, Postgres is the active backend.")
	fmt.Fprintln(w, "  - `ACM_SQLITE_PATH`: Optional explicit SQLite path. Relative paths resolve from the detected project root.")
	fmt.Fprintln(w, "  - `ACM_UNBOUNDED`: `true|false`. When true, retrieval/list surfaces stop applying built-in result caps.")
	fmt.Fprintln(w, "  - `ACM_LOG_LEVEL`: `debug|info|warn|error`.")
	fmt.Fprintln(w, "  - `ACM_LOG_SINK`: `stderr|stdout|discard`.")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Managed Repo Files:")
	fmt.Fprintln(w, "  - `.acm/context.db`, `.acm/context.db-shm`, `.acm/context.db-wal`: implicit repo-local SQLite database and sidecars.")
	fmt.Fprintln(w, "  - `.acm/acm-rules.yaml` or `acm-rules.yaml`: canonical rules.")
	fmt.Fprintln(w, "  - `.acm/acm-tags.yaml`: repo-local canonical tag overrides.")
	fmt.Fprintln(w, "  - `.acm/acm-tests.yaml` or `acm-tests.yaml`: repo-local executable verification definitions.")
	fmt.Fprintln(w, "  - `.env`: repo-local runtime/env overrides, loaded automatically.")
	fmt.Fprintln(w, "  - `.env.example`: seeded bootstrap example for ACM runtime variables.")
	fmt.Fprintln(w, "  - `.acm/bootstrap_candidates.json`: optional persisted bootstrap candidate output.")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "First-Run Recovery:")
	fmt.Fprintln(w, "  # zero-config local bootstrap")
	fmt.Fprintln(w, "  acm bootstrap --project myproject --project-root .")
	fmt.Fprintln(w, "  acm health --project myproject --include-details")
	fmt.Fprintln(w, "  # after later edits, refresh changed files")
	fmt.Fprintln(w, "  acm sync --project myproject --mode working_tree --insert-new-candidates")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "  # force explicit local SQLite")
	fmt.Fprintln(w, "  export ACM_SQLITE_PATH=.acm/context.db")
	fmt.Fprintln(w, "  acm health --project myproject --include-details")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "  # switch to Postgres")
	fmt.Fprintln(w, "  export ACM_PG_DSN='postgres://user:pass@localhost:5432/agents_context?sslmode=disable'")
	fmt.Fprintln(w, "  acm health --project myproject --include-details")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "More Help:")
	fmt.Fprintln(w, "  - `acm-mcp --help` describes the MCP adapter surface.")
	fmt.Fprintln(w, "  - `acm-mcp tools` prints the machine-readable MCP tool directory.")
}

func printHelpCommands(w io.Writer, commands []helpCommand) {
	for _, command := range commands {
		fmt.Fprintf(w, "  %s\n", command.usage)
		fmt.Fprintf(w, "    %s\n", command.summary)
	}
}

func configureRunUsage(fs *flag.FlagSet) {
	if fs == nil {
		return
	}
	fs.SetOutput(os.Stderr)
	fs.Usage = func() {
		out := fs.Output()
		fmt.Fprintln(out, "Usage:")
		fmt.Fprintln(out, "  acm run --in <request.json|->")
		fmt.Fprintln(out)
		fmt.Fprintln(out, "Purpose:")
		fmt.Fprintln(out, "  Execute a full v1 command envelope from stdin or a file.")
		fmt.Fprintln(out)
		fmt.Fprintln(out, "Flags:")
		fs.PrintDefaults()
		fmt.Fprintln(out)
		fmt.Fprintln(out, "Examples:")
		fmt.Fprintln(out, "  acm run --in request.json")
		fmt.Fprintln(out, "  cat request.json | acm run --in -")
	}
}

func configureValidateUsage(fs *flag.FlagSet) {
	if fs == nil {
		return
	}
	fs.SetOutput(os.Stderr)
	fs.Usage = func() {
		out := fs.Output()
		fmt.Fprintln(out, "Usage:")
		fmt.Fprintln(out, "  acm validate --in <request.json|->")
		fmt.Fprintln(out)
		fmt.Fprintln(out, "Purpose:")
		fmt.Fprintln(out, "  Validate a full v1 command envelope without executing it.")
		fmt.Fprintln(out)
		fmt.Fprintln(out, "Flags:")
		fs.PrintDefaults()
		fmt.Fprintln(out)
		fmt.Fprintln(out, "Examples:")
		fmt.Fprintln(out, "  acm validate --in request.json")
		fmt.Fprintln(out, "  cat request.json | acm validate --in -")
	}
}
