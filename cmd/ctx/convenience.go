package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/joshd/agents-context/internal/adapters/cli"
	"github.com/joshd/agents-context/internal/contracts/v1"
	"github.com/joshd/agents-context/internal/core"
	"github.com/joshd/agents-context/internal/logging"
	"github.com/joshd/agents-context/internal/runtime"
)

type serviceFactory func(context.Context, logging.Logger) (core.Service, runtime.CleanupFunc, error)

type adapterRunner func(context.Context, core.Service, io.Reader, io.Writer, func() time.Time, logging.Logger) int

func runConvenience(ctx context.Context, logger logging.Logger, subcommand string, args []string) int {
	return runConvenienceWithDeps(
		ctx,
		logger,
		subcommand,
		args,
		os.Stdout,
		time.Now,
		runtime.NewServiceFromEnvWithLogger,
		cli.RunWithLogger,
	)
}

func runConvenienceWithDeps(
	ctx context.Context,
	logger logging.Logger,
	subcommand string,
	args []string,
	out io.Writer,
	now func() time.Time,
	newService serviceFactory,
	runAdapter adapterRunner,
) int {
	logger = logging.Normalize(logger)
	logger.Info(ctx, logging.EventCtxRun, "stage", "start", "subcommand", subcommand)

	if out == nil {
		out = os.Stdout
	}
	if now == nil {
		now = time.Now
	}
	if newService == nil {
		newService = runtime.NewServiceFromEnvWithLogger
	}
	if runAdapter == nil {
		runAdapter = cli.RunWithLogger
	}

	env, err := buildConvenienceEnvelope(subcommand, args, now)
	if err != nil {
		if errors.Is(err, flag.ErrHelp) {
			logger.Info(ctx, logging.EventCtxRun, "stage", "finish", "subcommand", subcommand, "exit_code", 0)
			return 0
		}
		logger.Error(ctx, logging.EventCtxRun, "stage", "parse_flags", "subcommand", subcommand, "ok", false, "error_code", "INVALID_FLAGS")
		fmt.Fprintf(os.Stderr, "failed to parse %s flags: %v\n", subcommand, err)
		return 2
	}

	request, err := json.Marshal(env)
	if err != nil {
		logger.Error(ctx, logging.EventCtxRun, "stage", "marshal", "subcommand", subcommand, "ok", false, "error_code", "INTERNAL_ERROR")
		fmt.Fprintf(os.Stderr, "failed to build request: %v\n", err)
		return 1
	}

	svc, closeService, err := newService(ctx, logger)
	if err != nil {
		logger.Error(ctx, logging.EventCtxRun, "stage", "service_init", "subcommand", subcommand, "ok", false, "error_code", "SERVICE_INIT_FAILED")
		fmt.Fprintf(os.Stderr, "failed to initialize service: %v\n", err)
		return 1
	}
	defer closeService()

	code := runAdapter(ctx, svc, bytes.NewReader(request), out, now, logger)
	logger.Info(ctx, logging.EventCtxRun, "stage", "finish", "subcommand", subcommand, "exit_code", code)
	return code
}

func buildConvenienceEnvelope(subcommand string, args []string, now func() time.Time) (v1.CommandEnvelope, error) {
	switch subcommand {
	case "get-context":
		return buildGetContextEnvelope(args, now)
	case "fetch":
		return buildFetchEnvelope(args, now)
	case "propose-memory":
		return buildProposeMemoryEnvelope(args, now)
	case "work":
		return buildWorkEnvelope(args, now)
	case "report-completion":
		return buildReportCompletionEnvelope(args, now)
	case "sync":
		return buildSyncEnvelope(args, now)
	case "health", "health-check":
		return buildHealthCheckEnvelope(args, now)
	case "health-fix":
		return buildHealthFixEnvelope(args, now)
	case "coverage":
		return buildCoverageEnvelope(args, now)
	case "regress":
		return buildRegressEnvelope(args, now)
	case "bootstrap":
		return buildBootstrapEnvelope(args, now)
	default:
		return v1.CommandEnvelope{}, fmt.Errorf("unknown subcommand: %s", subcommand)
	}
}

func buildGetContextEnvelope(args []string, now func() time.Time) (v1.CommandEnvelope, error) {
	fs := newCommandFlagSet(
		"get-context",
		"ctx get-context --project-id <id> --task-text <text> [flags]",
		"ctx get-context --project-id soundspan --task-text \"Add sync checks\" --phase execute",
	)
	projectID, requestID := addProjectAndRequestFlags(fs)
	taskText := fs.String("task-text", "", "task description")
	phase := fs.String("phase", string(v1.PhaseExecute), "phase: plan|execute|review")
	scopeMode := fs.String("scope-mode", "", "scope mode: strict|warn|auto_index")
	allowStale := fs.Bool("allow-stale", false, "allow stale pointers")
	fallbackMode := fs.String("fallback-mode", "", "fallback mode: widen_once|none")
	maxNonRulePointers := fs.Int("max-non-rule-pointers", -1, "caps.max_non_rule_pointers")
	maxRulePointers := fs.Int("max-rule-pointers", -1, "caps.max_rule_pointers")
	maxHops := fs.Int("max-hops", -1, "caps.max_hops")
	maxHopExpansion := fs.Int("max-hop-expansion", -1, "caps.max_hop_expansion")
	maxMemories := fs.Int("max-memories", -1, "caps.max_memories")
	minPointerCount := fs.Int("min-pointer-count", -1, "caps.min_pointer_count")
	wordBudgetLimit := fs.Int("word-budget-limit", -1, "caps.word_budget_limit")
	if err := parseCommandFlags(fs, args); err != nil {
		return v1.CommandEnvelope{}, err
	}
	if err := requireFlag("project-id", *projectID); err != nil {
		return v1.CommandEnvelope{}, err
	}
	if err := requireFlag("task-text", *taskText); err != nil {
		return v1.CommandEnvelope{}, err
	}

	payload := v1.GetContextPayload{
		ProjectID:  strings.TrimSpace(*projectID),
		TaskText:   *taskText,
		Phase:      v1.Phase(strings.TrimSpace(*phase)),
		AllowStale: *allowStale,
	}
	if trimmedScopeMode := strings.TrimSpace(*scopeMode); trimmedScopeMode != "" {
		payload.ScopeMode = v1.ScopeMode(trimmedScopeMode)
	}
	if trimmedFallbackMode := strings.TrimSpace(*fallbackMode); trimmedFallbackMode != "" {
		payload.FallbackMode = trimmedFallbackMode
	}
	caps := v1.RetrievalCaps{}
	capsSet := false
	if *maxNonRulePointers >= 0 {
		caps.MaxNonRulePointers = *maxNonRulePointers
		capsSet = true
	}
	if *maxRulePointers >= 0 {
		caps.MaxRulePointers = *maxRulePointers
		capsSet = true
	}
	if *maxHops >= 0 {
		caps.MaxHops = *maxHops
		capsSet = true
	}
	if *maxHopExpansion >= 0 {
		caps.MaxHopExpansion = *maxHopExpansion
		capsSet = true
	}
	if *maxMemories >= 0 {
		caps.MaxMemories = *maxMemories
		capsSet = true
	}
	if *minPointerCount >= 0 {
		caps.MinPointerCount = *minPointerCount
		capsSet = true
	}
	if *wordBudgetLimit >= 0 {
		caps.WordBudgetLimit = *wordBudgetLimit
		capsSet = true
	}
	if capsSet {
		payload.Caps = &caps
	}

	return buildEnvelope(v1.CommandGetContext, *requestID, payload, now)
}

func buildFetchEnvelope(args []string, now func() time.Time) (v1.CommandEnvelope, error) {
	fs := newCommandFlagSet(
		"fetch",
		"ctx fetch --project-id <id> [--key <pointer>]... [--receipt-id <id>] [--expect <key=version>]...",
		"ctx fetch --project-id soundspan --key plan:req-12345678 --expect plan:req-12345678=v3",
	)
	projectID, requestID := addProjectAndRequestFlags(fs)
	receiptID := fs.String("receipt-id", "", "receipt ID to fetch via shorthand")
	var keys repeatedStringFlag
	var expects repeatedStringFlag
	fs.Var(&keys, "key", "key to fetch (repeatable)")
	fs.Var(&expects, "expect", "expected version in key=version form (repeatable)")
	if err := parseCommandFlags(fs, args); err != nil {
		return v1.CommandEnvelope{}, err
	}
	if err := requireFlag("project-id", *projectID); err != nil {
		return v1.CommandEnvelope{}, err
	}

	payload := v1.FetchPayload{
		ProjectID: strings.TrimSpace(*projectID),
		Keys:      keys.Values(),
		ReceiptID: strings.TrimSpace(*receiptID),
	}
	if len(expects) > 0 {
		payload.ExpectedVersions = make(map[string]string, len(expects))
		for _, expectedArg := range expects {
			key, version, err := parseExpectedVersion(expectedArg)
			if err != nil {
				return v1.CommandEnvelope{}, err
			}
			payload.ExpectedVersions[key] = version
		}
	}

	return buildEnvelope(v1.CommandFetch, *requestID, payload, now)
}

func buildProposeMemoryEnvelope(args []string, now func() time.Time) (v1.CommandEnvelope, error) {
	fs := newCommandFlagSet(
		"propose-memory",
		"ctx propose-memory --project-id <id> --receipt-id <id> --category <name> --subject <text> --content <text> --confidence <1-5> [flags]",
		"ctx propose-memory --project-id soundspan --receipt-id req-12345678 --category decision --subject \"Use health check\" --content \"Run before completion\" --confidence 4 --evidence-key rule:ctx/rule-1",
	)
	projectID, requestID := addProjectAndRequestFlags(fs)
	receiptID := fs.String("receipt-id", "", "receipt ID")
	category := fs.String("category", "", "memory category: decision|gotcha|pattern|preference")
	subject := fs.String("subject", "", "memory subject")
	content := fs.String("content", "", "memory content")
	confidence := fs.Int("confidence", 0, "memory confidence (1-5)")
	var relatedKeys repeatedStringFlag
	var tags repeatedStringFlag
	var evidenceKeys repeatedStringFlag
	fs.Var(&relatedKeys, "related-key", "related pointer key (repeatable)")
	fs.Var(&tags, "tag", "memory tag (repeatable)")
	fs.Var(&evidenceKeys, "evidence-key", "evidence pointer key (repeatable)")
	autoPromote := optionalBoolFlag{}
	fs.Var(&autoPromote, "auto-promote", "auto promote if validations pass (optional bool)")
	if err := parseCommandFlags(fs, args); err != nil {
		return v1.CommandEnvelope{}, err
	}
	if err := requireFlag("project-id", *projectID); err != nil {
		return v1.CommandEnvelope{}, err
	}
	if err := requireFlag("receipt-id", *receiptID); err != nil {
		return v1.CommandEnvelope{}, err
	}
	if err := requireFlag("category", *category); err != nil {
		return v1.CommandEnvelope{}, err
	}
	if err := requireFlag("subject", *subject); err != nil {
		return v1.CommandEnvelope{}, err
	}
	if err := requireFlag("content", *content); err != nil {
		return v1.CommandEnvelope{}, err
	}
	if *confidence == 0 {
		return v1.CommandEnvelope{}, fmt.Errorf("--confidence is required")
	}

	payload := v1.ProposeMemoryPayload{
		ProjectID: strings.TrimSpace(*projectID),
		ReceiptID: strings.TrimSpace(*receiptID),
		Memory: v1.MemoryPayload{
			Category:            v1.MemoryCategory(strings.TrimSpace(*category)),
			Subject:             *subject,
			Content:             *content,
			RelatedPointerKeys:  relatedKeys.Values(),
			Tags:                tags.Values(),
			Confidence:          *confidence,
			EvidencePointerKeys: evidenceKeys.Values(),
		},
	}
	if autoPromote.IsSet() {
		payload.AutoPromote = autoPromote.Ptr()
	}

	return buildEnvelope(v1.CommandProposeMemory, *requestID, payload, now)
}

func buildWorkEnvelope(args []string, now func() time.Time) (v1.CommandEnvelope, error) {
	fs := newCommandFlagSet(
		"work",
		"ctx work --project-id <id> [--plan-key <key>|--receipt-id <id>] [--plan-title <text>] [--items-file <path>]",
		"ctx work --project-id soundspan --receipt-id req-12345678 --items-file ./work-items.json",
	)
	projectID, requestID := addProjectAndRequestFlags(fs)
	planKey := fs.String("plan-key", "", "plan key")
	planTitle := fs.String("plan-title", "", "plan title")
	receiptID := fs.String("receipt-id", "", "receipt ID")
	itemsFile := fs.String("items-file", "", "JSON file containing an array of work items")
	if err := parseCommandFlags(fs, args); err != nil {
		return v1.CommandEnvelope{}, err
	}
	if err := requireFlag("project-id", *projectID); err != nil {
		return v1.CommandEnvelope{}, err
	}

	payload := v1.WorkPayload{
		ProjectID: strings.TrimSpace(*projectID),
		PlanKey:   strings.TrimSpace(*planKey),
		PlanTitle: strings.TrimSpace(*planTitle),
		ReceiptID: strings.TrimSpace(*receiptID),
	}
	if trimmedItemsFile := strings.TrimSpace(*itemsFile); trimmedItemsFile != "" {
		items, err := readWorkItemsFromFile(trimmedItemsFile)
		if err != nil {
			return v1.CommandEnvelope{}, err
		}
		payload.Items = items
	}

	return buildEnvelope(v1.CommandWork, *requestID, payload, now)
}

func buildReportCompletionEnvelope(args []string, now func() time.Time) (v1.CommandEnvelope, error) {
	fs := newCommandFlagSet(
		"report-completion",
		"ctx report-completion --project-id <id> --receipt-id <id> --outcome <text> [--file-changed <path>]... [--scope-mode <mode>]",
		"ctx report-completion --project-id soundspan --receipt-id req-12345678 --file-changed cmd/ctx/main.go --outcome \"Done\"",
	)
	projectID, requestID := addProjectAndRequestFlags(fs)
	receiptID := fs.String("receipt-id", "", "receipt ID")
	outcome := fs.String("outcome", "", "completion outcome summary")
	scopeMode := fs.String("scope-mode", "", "scope mode: strict|warn|auto_index")
	var filesChanged repeatedStringFlag
	fs.Var(&filesChanged, "file-changed", "repository-relative changed file path (repeatable)")
	if err := parseCommandFlags(fs, args); err != nil {
		return v1.CommandEnvelope{}, err
	}
	if err := requireFlag("project-id", *projectID); err != nil {
		return v1.CommandEnvelope{}, err
	}
	if err := requireFlag("receipt-id", *receiptID); err != nil {
		return v1.CommandEnvelope{}, err
	}
	if err := requireFlag("outcome", *outcome); err != nil {
		return v1.CommandEnvelope{}, err
	}

	payload := v1.ReportCompletionPayload{
		ProjectID:    strings.TrimSpace(*projectID),
		ReceiptID:    strings.TrimSpace(*receiptID),
		FilesChanged: filesChanged.Values(),
		Outcome:      *outcome,
	}
	if trimmedScopeMode := strings.TrimSpace(*scopeMode); trimmedScopeMode != "" {
		payload.ScopeMode = v1.ScopeMode(trimmedScopeMode)
	}

	return buildEnvelope(v1.CommandReportCompletion, *requestID, payload, now)
}

func buildSyncEnvelope(args []string, now func() time.Time) (v1.CommandEnvelope, error) {
	fs := newCommandFlagSet(
		"sync",
		"ctx sync --project-id <id> [--mode changed|full|working_tree] [--git-range <range>] [--project-root <path>] [--insert-new-candidates[=true|false]]",
		"ctx sync --project-id soundspan --mode changed --git-range HEAD~1..HEAD",
	)
	projectID, requestID := addProjectAndRequestFlags(fs)
	mode := fs.String("mode", "", "sync mode: changed|full|working_tree")
	gitRange := fs.String("git-range", "", "git revision range")
	projectRoot := fs.String("project-root", "", "project root for sync")
	insertNewCandidates := optionalBoolFlag{}
	fs.Var(&insertNewCandidates, "insert-new-candidates", "insert uncovered candidates (optional bool)")
	if err := parseCommandFlags(fs, args); err != nil {
		return v1.CommandEnvelope{}, err
	}
	if err := requireFlag("project-id", *projectID); err != nil {
		return v1.CommandEnvelope{}, err
	}

	payload := v1.SyncPayload{
		ProjectID:   strings.TrimSpace(*projectID),
		Mode:        strings.TrimSpace(*mode),
		GitRange:    strings.TrimSpace(*gitRange),
		ProjectRoot: strings.TrimSpace(*projectRoot),
	}
	if insertNewCandidates.IsSet() {
		payload.InsertNewCandidates = insertNewCandidates.Ptr()
	}

	return buildEnvelope(v1.CommandSync, *requestID, payload, now)
}

func buildHealthCheckEnvelope(args []string, now func() time.Time) (v1.CommandEnvelope, error) {
	fs := newCommandFlagSet(
		"health-check",
		"ctx health-check --project-id <id> [--include-details[=true|false]] [--max-findings-per-check <n>]",
		"ctx health-check --project-id soundspan --include-details --max-findings-per-check 50",
	)
	projectID, requestID := addProjectAndRequestFlags(fs)
	includeDetails := optionalBoolFlag{}
	fs.Var(&includeDetails, "include-details", "include detailed findings (optional bool)")
	maxFindingsPerCheck := fs.Int("max-findings-per-check", -1, "maximum findings per check (1..500)")
	if err := parseCommandFlags(fs, args); err != nil {
		return v1.CommandEnvelope{}, err
	}
	if err := requireFlag("project-id", *projectID); err != nil {
		return v1.CommandEnvelope{}, err
	}

	payload := v1.HealthCheckPayload{ProjectID: strings.TrimSpace(*projectID)}
	if includeDetails.IsSet() {
		payload.IncludeDetails = includeDetails.Ptr()
	}
	if *maxFindingsPerCheck >= 0 {
		payload.MaxFindingsPerCheck = maxFindingsPerCheck
	}

	return buildEnvelope(v1.CommandHealthCheck, *requestID, payload, now)
}

func buildHealthFixEnvelope(args []string, now func() time.Time) (v1.CommandEnvelope, error) {
	fs := newCommandFlagSet(
		"health-fix",
		"ctx health-fix --project-id <id> [--apply[=true|false>] [--project-root <path>] [--fixer <name>]...",
		"ctx health-fix --project-id soundspan --apply --fixer sync_working_tree",
	)
	projectID, requestID := addProjectAndRequestFlags(fs)
	applyChanges := optionalBoolFlag{}
	fs.Var(&applyChanges, "apply", "apply actions instead of dry-run (optional bool)")
	projectRoot := fs.String("project-root", "", "project root for fixers")
	var fixers repeatedStringFlag
	fs.Var(&fixers, "fixer", "fixer to run (repeatable): sync_working_tree|index_uncovered_files|sync_ruleset")
	if err := parseCommandFlags(fs, args); err != nil {
		return v1.CommandEnvelope{}, err
	}
	if err := requireFlag("project-id", *projectID); err != nil {
		return v1.CommandEnvelope{}, err
	}

	payload := v1.HealthFixPayload{
		ProjectID:   strings.TrimSpace(*projectID),
		ProjectRoot: strings.TrimSpace(*projectRoot),
	}
	if applyChanges.IsSet() {
		payload.Apply = applyChanges.Ptr()
	}
	if len(fixers) > 0 {
		payload.Fixers = make([]v1.HealthFixer, 0, len(fixers))
		for _, fixer := range fixers {
			payload.Fixers = append(payload.Fixers, v1.HealthFixer(fixer))
		}
	}

	return buildEnvelope(v1.CommandHealthFix, *requestID, payload, now)
}

func buildCoverageEnvelope(args []string, now func() time.Time) (v1.CommandEnvelope, error) {
	fs := newCommandFlagSet(
		"coverage",
		"ctx coverage --project-id <id> [--project-root <path>]",
		"ctx coverage --project-id soundspan --project-root .",
	)
	projectID, requestID := addProjectAndRequestFlags(fs)
	projectRoot := fs.String("project-root", "", "project root")
	if err := parseCommandFlags(fs, args); err != nil {
		return v1.CommandEnvelope{}, err
	}
	if err := requireFlag("project-id", *projectID); err != nil {
		return v1.CommandEnvelope{}, err
	}

	payload := v1.CoveragePayload{
		ProjectID:   strings.TrimSpace(*projectID),
		ProjectRoot: strings.TrimSpace(*projectRoot),
	}
	return buildEnvelope(v1.CommandCoverage, *requestID, payload, now)
}

func buildRegressEnvelope(args []string, now func() time.Time) (v1.CommandEnvelope, error) {
	fs := newCommandFlagSet(
		"regress",
		"ctx regress --project-id <id> (--eval-suite-path <path> | --eval-suite-inline-file <path>) [--minimum-recall <0..1>]",
		"ctx regress --project-id soundspan --eval-suite-path ./eval/ctx.json --minimum-recall 0.9",
	)
	projectID, requestID := addProjectAndRequestFlags(fs)
	evalSuitePath := fs.String("eval-suite-path", "", "path to eval suite JSON file")
	evalSuiteInlineFile := fs.String("eval-suite-inline-file", "", "path to JSON array of inline regress cases")
	minimumRecall := fs.Float64("minimum-recall", -1, "minimum recall threshold (0..1)")
	if err := parseCommandFlags(fs, args); err != nil {
		return v1.CommandEnvelope{}, err
	}
	if err := requireFlag("project-id", *projectID); err != nil {
		return v1.CommandEnvelope{}, err
	}

	trimmedEvalSuitePath := strings.TrimSpace(*evalSuitePath)
	trimmedEvalSuiteInlineFile := strings.TrimSpace(*evalSuiteInlineFile)
	if trimmedEvalSuitePath == "" && trimmedEvalSuiteInlineFile == "" {
		return v1.CommandEnvelope{}, fmt.Errorf("--eval-suite-path or --eval-suite-inline-file is required")
	}
	if trimmedEvalSuitePath != "" && trimmedEvalSuiteInlineFile != "" {
		return v1.CommandEnvelope{}, fmt.Errorf("use only one of --eval-suite-path or --eval-suite-inline-file")
	}

	payload := v1.RegressPayload{
		ProjectID:     strings.TrimSpace(*projectID),
		EvalSuitePath: trimmedEvalSuitePath,
	}
	if trimmedEvalSuiteInlineFile != "" {
		evalSuiteInline, err := readRegressCasesFromFile(trimmedEvalSuiteInlineFile)
		if err != nil {
			return v1.CommandEnvelope{}, err
		}
		payload.EvalSuiteInline = evalSuiteInline
	}
	if *minimumRecall >= 0 {
		payload.MinimumRecall = minimumRecall
	}

	return buildEnvelope(v1.CommandRegress, *requestID, payload, now)
}

func buildBootstrapEnvelope(args []string, now func() time.Time) (v1.CommandEnvelope, error) {
	fs := newCommandFlagSet(
		"bootstrap",
		"ctx bootstrap --project-id <id> --project-root <path> [--respect-gitignore[=true|false]] [--llm-assist-descriptions[=true|false]] [--output-candidates-path <path>]",
		"ctx bootstrap --project-id soundspan --project-root . --respect-gitignore",
	)
	projectID, requestID := addProjectAndRequestFlags(fs)
	projectRoot := fs.String("project-root", "", "project root to analyze")
	respectGitIgnore := optionalBoolFlag{}
	fs.Var(&respectGitIgnore, "respect-gitignore", "respect .gitignore while scanning (optional bool)")
	llmAssistDescriptions := optionalBoolFlag{}
	fs.Var(&llmAssistDescriptions, "llm-assist-descriptions", "enable generated description assistance (optional bool)")
	outputCandidatesPath := fs.String("output-candidates-path", "", "output file for bootstrap candidates")
	if err := parseCommandFlags(fs, args); err != nil {
		return v1.CommandEnvelope{}, err
	}
	if err := requireFlag("project-id", *projectID); err != nil {
		return v1.CommandEnvelope{}, err
	}
	if err := requireFlag("project-root", *projectRoot); err != nil {
		return v1.CommandEnvelope{}, err
	}

	payload := v1.BootstrapPayload{
		ProjectID:   strings.TrimSpace(*projectID),
		ProjectRoot: strings.TrimSpace(*projectRoot),
	}
	if respectGitIgnore.IsSet() {
		payload.RespectGitIgnore = respectGitIgnore.Ptr()
	}
	if llmAssistDescriptions.IsSet() {
		payload.LLMAssistDescriptions = llmAssistDescriptions.Ptr()
	}
	if trimmedOutputPath := strings.TrimSpace(*outputCandidatesPath); trimmedOutputPath != "" {
		payload.OutputCandidatesPath = &trimmedOutputPath
	}

	return buildEnvelope(v1.CommandBootstrap, *requestID, payload, now)
}

func buildEnvelope(command v1.Command, requestID string, payload any, now func() time.Time) (v1.CommandEnvelope, error) {
	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		return v1.CommandEnvelope{}, err
	}
	trimmedRequestID := strings.TrimSpace(requestID)
	if trimmedRequestID == "" {
		trimmedRequestID = newRequestID(command, now)
	}
	return v1.CommandEnvelope{
		Version:   v1.Version,
		Command:   command,
		RequestID: trimmedRequestID,
		Payload:   payloadJSON,
	}, nil
}

func newRequestID(command v1.Command, now func() time.Time) string {
	if now == nil {
		now = time.Now
	}
	commandPart := strings.ReplaceAll(string(command), "_", "-")
	return fmt.Sprintf("req-%s-%d", commandPart, now().UTC().UnixNano())
}

func addProjectAndRequestFlags(fs *flag.FlagSet) (*string, *string) {
	projectID := new(string)
	requestID := new(string)
	fs.StringVar(projectID, "project-id", "", "project identifier")
	fs.StringVar(projectID, "project", "", "alias for --project-id")
	fs.StringVar(requestID, "request-id", "", "request identifier (defaults to generated value)")
	fs.StringVar(requestID, "request", "", "alias for --request-id")
	return projectID, requestID
}

func newCommandFlagSet(name, usageLine string, examples ...string) *flag.FlagSet {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	fs.Usage = func() {
		out := fs.Output()
		fmt.Fprintln(out, "Usage:")
		fmt.Fprintf(out, "  %s\n", usageLine)
		fmt.Fprintln(out)
		fmt.Fprintln(out, "Flags:")
		fs.PrintDefaults()
		if len(examples) > 0 {
			fmt.Fprintln(out)
			fmt.Fprintln(out, "Examples:")
			for _, example := range examples {
				fmt.Fprintf(out, "  %s\n", example)
			}
		}
	}
	return fs
}

func parseCommandFlags(fs *flag.FlagSet, args []string) error {
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() > 0 {
		fs.Usage()
		return fmt.Errorf("unexpected positional arguments: %s", strings.Join(fs.Args(), " "))
	}
	return nil
}

func requireFlag(flagName, value string) error {
	if strings.TrimSpace(value) == "" {
		return fmt.Errorf("--%s is required", flagName)
	}
	return nil
}

func parseExpectedVersion(raw string) (string, string, error) {
	parts := strings.SplitN(raw, "=", 2)
	if len(parts) != 2 {
		return "", "", fmt.Errorf("invalid --expect value %q: expected key=version", raw)
	}
	key := strings.TrimSpace(parts[0])
	version := strings.TrimSpace(parts[1])
	if key == "" || version == "" {
		return "", "", fmt.Errorf("invalid --expect value %q: key and version are required", raw)
	}
	return key, version, nil
}

func readWorkItemsFromFile(path string) ([]v1.WorkItemPayload, error) {
	var items []v1.WorkItemPayload
	if err := readJSONFileStrict(path, &items); err != nil {
		return nil, fmt.Errorf("read --items-file %s: %w", path, err)
	}
	return items, nil
}

func readRegressCasesFromFile(path string) ([]v1.RegressCase, error) {
	var cases []v1.RegressCase
	if err := readJSONFileStrict(path, &cases); err != nil {
		return nil, fmt.Errorf("read --eval-suite-inline-file %s: %w", path, err)
	}
	return cases, nil
}

func readJSONFileStrict(path string, out any) error {
	var (
		data []byte
		err  error
	)
	if path == "-" {
		data, err = io.ReadAll(os.Stdin)
	} else {
		data, err = os.ReadFile(path)
	}
	if err != nil {
		return err
	}

	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(out); err != nil {
		return err
	}
	var extra any
	if err := decoder.Decode(&extra); err == nil {
		return fmt.Errorf("unexpected trailing JSON tokens")
	}
	return nil
}

type repeatedStringFlag []string

func (r *repeatedStringFlag) String() string {
	return strings.Join(*r, ",")
}

func (r *repeatedStringFlag) Set(value string) error {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return fmt.Errorf("value must not be empty")
	}
	*r = append(*r, trimmed)
	return nil
}

func (r repeatedStringFlag) Values() []string {
	return append([]string(nil), r...)
}

type optionalBoolFlag struct {
	set   bool
	value bool
}

func (f *optionalBoolFlag) String() string {
	if !f.set {
		return ""
	}
	return strconv.FormatBool(f.value)
}

func (f *optionalBoolFlag) Set(raw string) error {
	if strings.TrimSpace(raw) == "" {
		raw = "true"
	}
	value, err := strconv.ParseBool(raw)
	if err != nil {
		return fmt.Errorf("invalid boolean value %q", raw)
	}
	f.set = true
	f.value = value
	return nil
}

func (f *optionalBoolFlag) IsBoolFlag() bool {
	return true
}

func (f *optionalBoolFlag) IsSet() bool {
	return f.set
}

func (f *optionalBoolFlag) Ptr() *bool {
	if !f.set {
		return nil
	}
	value := f.value
	return &value
}
