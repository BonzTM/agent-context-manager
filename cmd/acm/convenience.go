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

	"github.com/bonztm/agent-context-manager/internal/adapters/cli"
	"github.com/bonztm/agent-context-manager/internal/contracts/v1"
	"github.com/bonztm/agent-context-manager/internal/core"
	"github.com/bonztm/agent-context-manager/internal/logging"
	"github.com/bonztm/agent-context-manager/internal/runtime"
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
	logger.Info(ctx, logging.EventACMRun, "stage", "start", "subcommand", subcommand)

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
			logger.Info(ctx, logging.EventACMRun, "stage", "finish", "subcommand", subcommand, "exit_code", 0)
			return 0
		}
		logger.Error(ctx, logging.EventACMRun, "stage", "parse_flags", "subcommand", subcommand, "ok", false, "error_code", "INVALID_FLAGS")
		fmt.Fprintf(os.Stderr, "failed to parse %s flags: %v\n", subcommand, err)
		return 2
	}

	request, err := json.Marshal(env)
	if err != nil {
		logger.Error(ctx, logging.EventACMRun, "stage", "marshal", "subcommand", subcommand, "ok", false, "error_code", "INTERNAL_ERROR")
		fmt.Fprintf(os.Stderr, "failed to build request: %v\n", err)
		return 1
	}

	svc, closeService, err := newService(ctx, logger)
	if err != nil {
		logger.Error(ctx, logging.EventACMRun, "stage", "service_init", "subcommand", subcommand, "ok", false, "error_code", "SERVICE_INIT_FAILED")
		fmt.Fprintf(os.Stderr, "failed to initialize service: %v\n", err)
		return 1
	}
	defer closeService()

	code := runAdapter(ctx, svc, bytes.NewReader(request), out, now, logger)
	logger.Info(ctx, logging.EventACMRun, "stage", "finish", "subcommand", subcommand, "exit_code", code)
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
	case "work-list", "work-search", "history-search":
		return buildHistorySearchEnvelope(subcommand, args, now)
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
	case "eval":
		return buildEvalEnvelope(args, now)
	case "verify":
		return buildVerifyEnvelope(args, now)
	case "bootstrap":
		return buildBootstrapEnvelope(args, now)
	default:
		return v1.CommandEnvelope{}, fmt.Errorf("unknown subcommand: %s", subcommand)
	}
}

func buildGetContextEnvelope(args []string, now func() time.Time) (v1.CommandEnvelope, error) {
	fs := newCommandFlagSet(
		"get-context",
		"acm get-context --project <id> [--task-text <text>|--task-file <path>] [--tags-file <path>] [flags]",
		"acm get-context --project myproject --task-text \"Add sync checks\" --phase execute",
	)
	projectID, requestID := addProjectAndRequestFlags(fs)
	taskText := fs.String("task-text", "", "task description")
	taskFile := fs.String("task-file", "", "file containing task text ('-' for stdin)")
	phase := fs.String("phase", string(v1.PhaseExecute), "phase: plan|execute|review")
	tagsFile := fs.String("tags-file", "", "explicit canonical tag dictionary file path (overrides default discovery)")
	scopeMode := fs.String("scope-mode", "", "scope mode: strict|warn|auto_index")
	allowStale := fs.Bool("allow-stale", false, "allow stale pointers")
	fallbackMode := fs.String("fallback-mode", "", "fallback mode: widen_once|none")
	unbounded := optionalBoolFlag{}
	maxNonRulePointers := fs.Int("max-non-rule-pointers", -1, "caps.max_non_rule_pointers")
	maxRulePointers := fs.Int("max-rule-pointers", -1, "caps.max_rule_pointers")
	maxHops := fs.Int("max-hops", -1, "caps.max_hops")
	maxHopExpansion := fs.Int("max-hop-expansion", -1, "caps.max_hop_expansion")
	maxMemories := fs.Int("max-memories", -1, "caps.max_memories")
	minPointerCount := fs.Int("min-pointer-count", -1, "caps.min_pointer_count")
	wordBudgetLimit := fs.Int("word-budget-limit", -1, "caps.word_budget_limit")
	fs.Var(&unbounded, "unbounded", "remove built-in retrieval caps (optional bool)")
	if err := parseCommandFlags(fs, args); err != nil {
		return v1.CommandEnvelope{}, err
	}
	if err := requireFlag("project", *projectID); err != nil {
		return v1.CommandEnvelope{}, err
	}

	trimmedTaskText := strings.TrimSpace(*taskText)
	trimmedTaskFile := strings.TrimSpace(*taskFile)
	if trimmedTaskText != "" && trimmedTaskFile != "" {
		return v1.CommandEnvelope{}, fmt.Errorf("use only one of --task-text or --task-file")
	}
	if trimmedTaskText == "" {
		if trimmedTaskFile == "" {
			return v1.CommandEnvelope{}, fmt.Errorf("--task-text or --task-file is required")
		}
		blob, err := readTextFile(trimmedTaskFile)
		if err != nil {
			return v1.CommandEnvelope{}, fmt.Errorf("read --task-file %s: %w", trimmedTaskFile, err)
		}
		trimmedTaskText = strings.TrimSpace(blob)
	}
	if trimmedTaskText == "" {
		return v1.CommandEnvelope{}, fmt.Errorf("task text must not be empty")
	}

	payload := v1.GetContextPayload{
		ProjectID:  strings.TrimSpace(*projectID),
		TaskText:   trimmedTaskText,
		Phase:      v1.Phase(strings.TrimSpace(*phase)),
		TagsFile:   strings.TrimSpace(*tagsFile),
		AllowStale: *allowStale,
	}
	if trimmedScopeMode := strings.TrimSpace(*scopeMode); trimmedScopeMode != "" {
		payload.ScopeMode = v1.ScopeMode(trimmedScopeMode)
	}
	if trimmedFallbackMode := strings.TrimSpace(*fallbackMode); trimmedFallbackMode != "" {
		payload.FallbackMode = trimmedFallbackMode
	}
	if unbounded.set {
		value := unbounded.value
		payload.Unbounded = &value
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
		"acm fetch --project <id> [--key <pointer>]... [--keys-file <path>] [--keys-json <json>] [--receipt-id <id>] [--expect <key=version>]... [--expected-versions-file <path>] [--expected-versions-json <json>]",
		"acm fetch --project myproject --key plan:req-12345678 --expect plan:req-12345678=v3",
	)
	projectID, requestID := addProjectAndRequestFlags(fs)
	receiptID := fs.String("receipt-id", "", "receipt ID to fetch via shorthand")
	keysFile := fs.String("keys-file", "", "JSON file containing an array of keys")
	keysJSON := fs.String("keys-json", "", "inline JSON array of keys")
	expectedVersionsFile := fs.String("expected-versions-file", "", "JSON file containing an object map of key->version")
	expectedVersionsJSON := fs.String("expected-versions-json", "", "inline JSON object map of key->version")
	var keys repeatedStringFlag
	var expects repeatedStringFlag
	fs.Var(&keys, "key", "key to fetch (repeatable)")
	fs.Var(&expects, "expect", "expected version in key=version form (repeatable)")
	if err := parseCommandFlags(fs, args); err != nil {
		return v1.CommandEnvelope{}, err
	}
	if err := requireFlag("project", *projectID); err != nil {
		return v1.CommandEnvelope{}, err
	}

	allKeys := keys.Values()
	if trimmedKeysFile := strings.TrimSpace(*keysFile); trimmedKeysFile != "" {
		fileKeys, err := readStringListFromFile(trimmedKeysFile, "--keys-file")
		if err != nil {
			return v1.CommandEnvelope{}, err
		}
		allKeys = mergeUnique(allKeys, fileKeys)
	}
	if trimmedKeysJSON := strings.TrimSpace(*keysJSON); trimmedKeysJSON != "" {
		inlineKeys, err := readStringListFromJSON(trimmedKeysJSON, "--keys-json")
		if err != nil {
			return v1.CommandEnvelope{}, err
		}
		allKeys = mergeUnique(allKeys, inlineKeys)
	}

	expectedVersions := map[string]string{}
	if trimmedExpectedVersionsFile := strings.TrimSpace(*expectedVersionsFile); trimmedExpectedVersionsFile != "" {
		fileMap, err := readStringMapFromFile(trimmedExpectedVersionsFile, "--expected-versions-file")
		if err != nil {
			return v1.CommandEnvelope{}, err
		}
		for k, v := range fileMap {
			expectedVersions[k] = v
		}
	}
	if trimmedExpectedVersionsJSON := strings.TrimSpace(*expectedVersionsJSON); trimmedExpectedVersionsJSON != "" {
		inlineMap, err := readStringMapFromJSON(trimmedExpectedVersionsJSON, "--expected-versions-json")
		if err != nil {
			return v1.CommandEnvelope{}, err
		}
		for k, v := range inlineMap {
			expectedVersions[k] = v
		}
	}
	for _, expectedArg := range expects {
		key, version, err := parseExpectedVersion(expectedArg)
		if err != nil {
			return v1.CommandEnvelope{}, err
		}
		expectedVersions[key] = version
	}

	payload := v1.FetchPayload{
		ProjectID: strings.TrimSpace(*projectID),
		Keys:      allKeys,
		ReceiptID: strings.TrimSpace(*receiptID),
	}
	if len(allKeys) == 0 {
		payload.Keys = nil
	}
	if len(expectedVersions) > 0 {
		payload.ExpectedVersions = expectedVersions
	}

	return buildEnvelope(v1.CommandFetch, *requestID, payload, now)
}

func buildProposeMemoryEnvelope(args []string, now func() time.Time) (v1.CommandEnvelope, error) {
	fs := newCommandFlagSet(
		"propose-memory",
		"acm propose-memory --project <id> --receipt-id <id> --category <name> --subject <text> (--content <text>|--content-file <path>) --confidence <1-5> [--memory-tag <tag>]... [--memory-tags-file <path>|--memory-tags-json <json>] [--tags-file <path>] [flags]",
		"acm propose-memory --project myproject --receipt-id req-12345678 --category decision --subject \"Use health check\" --content \"Run before completion\" --confidence 4 --evidence-key rule:myproject/rule-1",
	)
	projectID, requestID := addProjectAndRequestFlags(fs)
	receiptID := fs.String("receipt-id", "", "receipt ID")
	category := fs.String("category", "", "memory category: decision|gotcha|pattern|preference")
	subject := fs.String("subject", "", "memory subject")
	content := fs.String("content", "", "memory content")
	contentFile := fs.String("content-file", "", "file containing memory content ('-' for stdin)")
	confidence := fs.Int("confidence", 0, "memory confidence (1-5)")
	canonicalTagsFile := fs.String("tags-file", "", "explicit canonical tag dictionary file path (overrides default discovery)")
	relatedKeysFile := fs.String("related-keys-file", "", "JSON file containing an array of related pointer keys")
	relatedKeysJSON := fs.String("related-keys-json", "", "inline JSON array of related pointer keys")
	memoryTagsFile := fs.String("memory-tags-file", "", "JSON file containing an array of memory tags")
	memoryTagsJSON := fs.String("memory-tags-json", "", "inline JSON array of memory tags")
	evidenceKeysFile := fs.String("evidence-keys-file", "", "JSON file containing an array of evidence pointer keys")
	evidenceKeysJSON := fs.String("evidence-keys-json", "", "inline JSON array of evidence pointer keys")
	var relatedKeys repeatedStringFlag
	var memoryTags repeatedStringFlag
	var evidenceKeys repeatedStringFlag
	fs.Var(&relatedKeys, "related-key", "related pointer key (repeatable)")
	fs.Var(&memoryTags, "memory-tag", "memory tag (repeatable)")
	fs.Var(&evidenceKeys, "evidence-key", "evidence pointer key (repeatable)")
	autoPromote := optionalBoolFlag{}
	fs.Var(&autoPromote, "auto-promote", "auto promote if validations pass (optional bool)")
	if err := parseCommandFlags(fs, args); err != nil {
		return v1.CommandEnvelope{}, err
	}
	if err := requireFlag("project", *projectID); err != nil {
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
	if *confidence == 0 {
		return v1.CommandEnvelope{}, fmt.Errorf("--confidence is required")
	}

	trimmedContent := strings.TrimSpace(*content)
	trimmedContentFile := strings.TrimSpace(*contentFile)
	if trimmedContent != "" && trimmedContentFile != "" {
		return v1.CommandEnvelope{}, fmt.Errorf("use only one of --content or --content-file")
	}
	if trimmedContent == "" {
		if trimmedContentFile == "" {
			return v1.CommandEnvelope{}, fmt.Errorf("--content or --content-file is required")
		}
		blob, err := readTextFile(trimmedContentFile)
		if err != nil {
			return v1.CommandEnvelope{}, fmt.Errorf("read --content-file %s: %w", trimmedContentFile, err)
		}
		trimmedContent = strings.TrimSpace(blob)
	}
	if trimmedContent == "" {
		return v1.CommandEnvelope{}, fmt.Errorf("memory content must not be empty")
	}

	allRelatedKeys := relatedKeys.Values()
	if trimmedRelatedKeysFile := strings.TrimSpace(*relatedKeysFile); trimmedRelatedKeysFile != "" {
		fileValues, err := readStringListFromFile(trimmedRelatedKeysFile, "--related-keys-file")
		if err != nil {
			return v1.CommandEnvelope{}, err
		}
		allRelatedKeys = mergeUnique(allRelatedKeys, fileValues)
	}
	if trimmedRelatedKeysJSON := strings.TrimSpace(*relatedKeysJSON); trimmedRelatedKeysJSON != "" {
		inlineValues, err := readStringListFromJSON(trimmedRelatedKeysJSON, "--related-keys-json")
		if err != nil {
			return v1.CommandEnvelope{}, err
		}
		allRelatedKeys = mergeUnique(allRelatedKeys, inlineValues)
	}

	allTags := memoryTags.Values()
	if trimmedMemoryTagsFile := strings.TrimSpace(*memoryTagsFile); trimmedMemoryTagsFile != "" {
		fileValues, err := readStringListFromFile(trimmedMemoryTagsFile, "--memory-tags-file")
		if err != nil {
			return v1.CommandEnvelope{}, err
		}
		allTags = mergeUnique(allTags, fileValues)
	}
	if trimmedMemoryTagsJSON := strings.TrimSpace(*memoryTagsJSON); trimmedMemoryTagsJSON != "" {
		inlineValues, err := readStringListFromJSON(trimmedMemoryTagsJSON, "--memory-tags-json")
		if err != nil {
			return v1.CommandEnvelope{}, err
		}
		allTags = mergeUnique(allTags, inlineValues)
	}

	allEvidenceKeys := evidenceKeys.Values()
	if trimmedEvidenceKeysFile := strings.TrimSpace(*evidenceKeysFile); trimmedEvidenceKeysFile != "" {
		fileValues, err := readStringListFromFile(trimmedEvidenceKeysFile, "--evidence-keys-file")
		if err != nil {
			return v1.CommandEnvelope{}, err
		}
		allEvidenceKeys = mergeUnique(allEvidenceKeys, fileValues)
	}
	if trimmedEvidenceKeysJSON := strings.TrimSpace(*evidenceKeysJSON); trimmedEvidenceKeysJSON != "" {
		inlineValues, err := readStringListFromJSON(trimmedEvidenceKeysJSON, "--evidence-keys-json")
		if err != nil {
			return v1.CommandEnvelope{}, err
		}
		allEvidenceKeys = mergeUnique(allEvidenceKeys, inlineValues)
	}

	payload := v1.ProposeMemoryPayload{
		ProjectID: strings.TrimSpace(*projectID),
		ReceiptID: strings.TrimSpace(*receiptID),
		TagsFile:  strings.TrimSpace(*canonicalTagsFile),
		Memory: v1.MemoryPayload{
			Category:            v1.MemoryCategory(strings.TrimSpace(*category)),
			Subject:             *subject,
			Content:             trimmedContent,
			RelatedPointerKeys:  allRelatedKeys,
			Tags:                allTags,
			Confidence:          *confidence,
			EvidencePointerKeys: allEvidenceKeys,
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
		"acm work --project <id> [--plan-key <key>|--receipt-id <id>] [--plan-title <text>] [--mode <merge|replace>] [--plan-file <path>|--plan-json <json>] [--tasks-file <path>|--tasks-json <json>] [--items-file <path>|--items-json <json>]",
		"acm work --project myproject --receipt-id req-12345678 --tasks-json '[{\"key\":\"verify:tests\",\"summary\":\"Run tests\",\"status\":\"pending\"}]'",
	)
	projectID, requestID := addProjectAndRequestFlags(fs)
	planKey := fs.String("plan-key", "", "plan key")
	planTitle := fs.String("plan-title", "", "plan title")
	receiptID := fs.String("receipt-id", "", "receipt ID")
	mode := fs.String("mode", "", "work plan update mode: merge|replace")
	planFile := fs.String("plan-file", "", "JSON file containing work plan metadata")
	planJSON := fs.String("plan-json", "", "inline JSON object containing work plan metadata")
	tasksFile := fs.String("tasks-file", "", "JSON file containing an array of plan tasks")
	tasksJSON := fs.String("tasks-json", "", "inline JSON array containing plan tasks")
	itemsFile := fs.String("items-file", "", "JSON file containing an array of work items")
	itemsJSON := fs.String("items-json", "", "inline JSON array of work items")
	if err := parseCommandFlags(fs, args); err != nil {
		return v1.CommandEnvelope{}, err
	}
	if err := requireFlag("project", *projectID); err != nil {
		return v1.CommandEnvelope{}, err
	}

	payload := v1.WorkPayload{
		ProjectID: strings.TrimSpace(*projectID),
		PlanKey:   strings.TrimSpace(*planKey),
		PlanTitle: strings.TrimSpace(*planTitle),
		ReceiptID: strings.TrimSpace(*receiptID),
		Mode:      v1.WorkPlanMode(strings.TrimSpace(*mode)),
	}
	trimmedPlanFile := strings.TrimSpace(*planFile)
	trimmedPlanJSON := strings.TrimSpace(*planJSON)
	if trimmedPlanFile != "" && trimmedPlanJSON != "" {
		return v1.CommandEnvelope{}, fmt.Errorf("use only one of --plan-file or --plan-json")
	}
	if trimmedPlanFile != "" {
		plan, err := readWorkPlanFromFile(trimmedPlanFile)
		if err != nil {
			return v1.CommandEnvelope{}, err
		}
		payload.Plan = &plan
	}
	if trimmedPlanJSON != "" {
		plan, err := readWorkPlanFromJSON(trimmedPlanJSON)
		if err != nil {
			return v1.CommandEnvelope{}, err
		}
		payload.Plan = &plan
	}

	trimmedTasksFile := strings.TrimSpace(*tasksFile)
	trimmedTasksJSON := strings.TrimSpace(*tasksJSON)
	if trimmedTasksFile != "" && trimmedTasksJSON != "" {
		return v1.CommandEnvelope{}, fmt.Errorf("use only one of --tasks-file or --tasks-json")
	}

	trimmedItemsFile := strings.TrimSpace(*itemsFile)
	trimmedItemsJSON := strings.TrimSpace(*itemsJSON)
	if trimmedItemsFile != "" && trimmedItemsJSON != "" {
		return v1.CommandEnvelope{}, fmt.Errorf("use only one of --items-file or --items-json")
	}
	if (trimmedTasksFile != "" || trimmedTasksJSON != "") && (trimmedItemsFile != "" || trimmedItemsJSON != "") {
		return v1.CommandEnvelope{}, fmt.Errorf("use tasks flags or items flags, not both")
	}
	if trimmedTasksFile != "" {
		tasks, err := readWorkTasksFromFile(trimmedTasksFile)
		if err != nil {
			return v1.CommandEnvelope{}, err
		}
		payload.Tasks = tasks
	}
	if trimmedTasksJSON != "" {
		tasks, err := readWorkTasksFromJSON(trimmedTasksJSON)
		if err != nil {
			return v1.CommandEnvelope{}, err
		}
		payload.Tasks = tasks
	}
	if trimmedItemsFile != "" {
		items, err := readWorkItemsFromFile(trimmedItemsFile)
		if err != nil {
			return v1.CommandEnvelope{}, err
		}
		payload.Items = items
	}
	if trimmedItemsJSON != "" {
		items, err := readWorkItemsFromJSON(trimmedItemsJSON)
		if err != nil {
			return v1.CommandEnvelope{}, err
		}
		payload.Items = items
	}

	return buildEnvelope(v1.CommandWork, *requestID, payload, now)
}

func buildHistorySearchEnvelope(subcommand string, args []string, now func() time.Time) (v1.CommandEnvelope, error) {
	usageLine := "acm work search --project <id> (--query <text>|--query-file <path>) [--scope <current|deferred|completed|all>] [--kind <kind>] [--limit <n>] [--unbounded[=true|false]]"
	example := "acm work search --project myproject --query \"bootstrap\" --scope all --limit 10"
	defaultScope := v1.HistoryScopeCurrent
	defaultEntity := v1.HistoryEntityWork
	queryRequired := true

	switch subcommand {
	case "work-list":
		usageLine = "acm work list --project <id> [--scope <current|deferred|completed|all>] [--kind <kind>] [--limit <n>] [--unbounded[=true|false]]"
		example = "acm work list --project myproject --scope current --limit 20"
		defaultScope = v1.HistoryScopeCurrent
		queryRequired = false
	case "work-search":
		usageLine = "acm work search --project <id> (--query <text>|--query-file <path>) [--scope <current|deferred|completed|all>] [--kind <kind>] [--limit <n>] [--unbounded[=true|false]]"
		example = "acm work search --project myproject --query \"MCP parity\" --scope current"
	case "history-search":
		usageLine = "acm history search --project <id> [--entity <all|work|receipt|run>] [--query <text>|--query-file <path>] [--scope <current|deferred|completed|all>] [--kind <kind>] [--limit <n>] [--unbounded[=true|false]]"
		example = "acm history search --project myproject --entity all --query \"bootstrap\" --limit 25"
		defaultScope = v1.HistoryScopeAll
		defaultEntity = v1.HistoryEntityAll
		queryRequired = false
	}

	fs := newCommandFlagSet(subcommand, usageLine, example)
	projectID, requestID := addProjectAndRequestFlags(fs)
	entity := fs.String("entity", string(defaultEntity), "history entity: all|work|receipt|run")
	query := fs.String("query", "", "search text applied to plan and task summaries")
	queryFile := fs.String("query-file", "", "file containing search text ('-' for stdin)")
	scope := fs.String("scope", string(defaultScope), "history scope: current|deferred|completed|all")
	kind := fs.String("kind", "", "optional plan kind filter")
	limit := fs.Int("limit", 20, "maximum number of plans to return (1-100)")
	unbounded := optionalBoolFlag{}
	fs.Var(&unbounded, "unbounded", "remove built-in history result caps (optional bool)")
	if err := parseCommandFlags(fs, args); err != nil {
		return v1.CommandEnvelope{}, err
	}
	if err := requireFlag("project", *projectID); err != nil {
		return v1.CommandEnvelope{}, err
	}

	trimmedQuery := strings.TrimSpace(*query)
	trimmedQueryFile := strings.TrimSpace(*queryFile)
	if trimmedQuery != "" && trimmedQueryFile != "" {
		return v1.CommandEnvelope{}, fmt.Errorf("use only one of --query or --query-file")
	}
	if trimmedQuery == "" && trimmedQueryFile != "" {
		blob, err := readTextFile(trimmedQueryFile)
		if err != nil {
			return v1.CommandEnvelope{}, fmt.Errorf("read --query-file %s: %w", trimmedQueryFile, err)
		}
		trimmedQuery = strings.TrimSpace(blob)
	}
	if queryRequired && trimmedQuery == "" {
		return v1.CommandEnvelope{}, fmt.Errorf("--query or --query-file is required")
	}

	payload := v1.HistorySearchPayload{
		ProjectID: strings.TrimSpace(*projectID),
		Entity:    v1.HistoryEntity(strings.TrimSpace(*entity)),
		Query:     trimmedQuery,
		Scope:     v1.HistoryScope(strings.TrimSpace(*scope)),
		Kind:      strings.TrimSpace(*kind),
	}
	if *limit > 0 {
		payload.Limit = *limit
	}
	if unbounded.set {
		value := unbounded.value
		payload.Unbounded = &value
	}

	return buildEnvelope(v1.CommandHistorySearch, *requestID, payload, now)
}

func buildReportCompletionEnvelope(args []string, now func() time.Time) (v1.CommandEnvelope, error) {
	fs := newCommandFlagSet(
		"report-completion",
		"acm report-completion --project <id> --receipt-id <id> [--outcome <text>|--outcome-file <path>] [--file-changed <path>]... [--files-changed-file <path>] [--files-changed-json <json>] [--scope-mode <mode>] [--tags-file <path>]",
		"acm report-completion --project myproject --receipt-id req-12345678 --file-changed cmd/acm/main.go --outcome \"Done\"",
	)
	projectID, requestID := addProjectAndRequestFlags(fs)
	receiptID := fs.String("receipt-id", "", "receipt ID")
	outcome := fs.String("outcome", "", "completion outcome summary")
	outcomeFile := fs.String("outcome-file", "", "file containing completion outcome ('-' for stdin)")
	scopeMode := fs.String("scope-mode", "", "scope mode: strict|warn|auto_index")
	tagsFile := fs.String("tags-file", "", "explicit canonical tag dictionary file path (overrides default discovery)")
	filesChangedFile := fs.String("files-changed-file", "", "JSON file containing an array of changed file paths")
	filesChangedJSON := fs.String("files-changed-json", "", "inline JSON array of changed file paths")
	var filesChanged repeatedStringFlag
	fs.Var(&filesChanged, "file-changed", "repository-relative changed file path (repeatable)")
	if err := parseCommandFlags(fs, args); err != nil {
		return v1.CommandEnvelope{}, err
	}
	if err := requireFlag("project", *projectID); err != nil {
		return v1.CommandEnvelope{}, err
	}
	if err := requireFlag("receipt-id", *receiptID); err != nil {
		return v1.CommandEnvelope{}, err
	}

	trimmedOutcome := strings.TrimSpace(*outcome)
	trimmedOutcomeFile := strings.TrimSpace(*outcomeFile)
	if trimmedOutcome != "" && trimmedOutcomeFile != "" {
		return v1.CommandEnvelope{}, fmt.Errorf("use only one of --outcome or --outcome-file")
	}
	if trimmedOutcome == "" {
		if trimmedOutcomeFile == "" {
			return v1.CommandEnvelope{}, fmt.Errorf("--outcome or --outcome-file is required")
		}
		blob, err := readTextFile(trimmedOutcomeFile)
		if err != nil {
			return v1.CommandEnvelope{}, fmt.Errorf("read --outcome-file %s: %w", trimmedOutcomeFile, err)
		}
		trimmedOutcome = strings.TrimSpace(blob)
	}
	if trimmedOutcome == "" {
		return v1.CommandEnvelope{}, fmt.Errorf("outcome must not be empty")
	}

	allFilesChanged := filesChanged.Values()
	if trimmedFilesChangedFile := strings.TrimSpace(*filesChangedFile); trimmedFilesChangedFile != "" {
		fileValues, err := readStringListFromFile(trimmedFilesChangedFile, "--files-changed-file")
		if err != nil {
			return v1.CommandEnvelope{}, err
		}
		allFilesChanged = mergeUnique(allFilesChanged, fileValues)
	}
	if trimmedFilesChangedJSON := strings.TrimSpace(*filesChangedJSON); trimmedFilesChangedJSON != "" {
		inlineValues, err := readStringListFromJSON(trimmedFilesChangedJSON, "--files-changed-json")
		if err != nil {
			return v1.CommandEnvelope{}, err
		}
		allFilesChanged = mergeUnique(allFilesChanged, inlineValues)
	}

	payload := v1.ReportCompletionPayload{
		ProjectID:    strings.TrimSpace(*projectID),
		ReceiptID:    strings.TrimSpace(*receiptID),
		TagsFile:     strings.TrimSpace(*tagsFile),
		FilesChanged: allFilesChanged,
		Outcome:      trimmedOutcome,
	}
	if trimmedScopeMode := strings.TrimSpace(*scopeMode); trimmedScopeMode != "" {
		payload.ScopeMode = v1.ScopeMode(trimmedScopeMode)
	}

	return buildEnvelope(v1.CommandReportCompletion, *requestID, payload, now)
}

func buildSyncEnvelope(args []string, now func() time.Time) (v1.CommandEnvelope, error) {
	fs := newCommandFlagSet(
		"sync",
		"acm sync --project <id> [--mode changed|full|working_tree] [--git-range <range>] [--project-root <path>] [--rules-file <path>] [--tags-file <path>] [--insert-new-candidates[=true|false]]",
		"acm sync --project myproject --mode changed --git-range HEAD~1..HEAD",
	)
	projectID, requestID := addProjectAndRequestFlags(fs)
	mode := fs.String("mode", "", "sync mode: changed|full|working_tree")
	gitRange := fs.String("git-range", "", "git revision range")
	projectRoot := fs.String("project-root", "", "project root for sync")
	rulesFile := fs.String("rules-file", "", "explicit canonical rules file path (overrides default discovery)")
	tagsFile := fs.String("tags-file", "", "explicit canonical tag dictionary file path (overrides default discovery)")
	insertNewCandidates := optionalBoolFlag{}
	fs.Var(&insertNewCandidates, "insert-new-candidates", "auto-index uncovered files as pointer stubs (optional bool)")
	if err := parseCommandFlags(fs, args); err != nil {
		return v1.CommandEnvelope{}, err
	}
	if err := requireFlag("project", *projectID); err != nil {
		return v1.CommandEnvelope{}, err
	}

	payload := v1.SyncPayload{
		ProjectID:   strings.TrimSpace(*projectID),
		Mode:        strings.TrimSpace(*mode),
		GitRange:    strings.TrimSpace(*gitRange),
		ProjectRoot: strings.TrimSpace(*projectRoot),
		RulesFile:   strings.TrimSpace(*rulesFile),
		TagsFile:    strings.TrimSpace(*tagsFile),
	}
	if insertNewCandidates.IsSet() {
		payload.InsertNewCandidates = insertNewCandidates.Ptr()
	}

	return buildEnvelope(v1.CommandSync, *requestID, payload, now)
}

func buildHealthCheckEnvelope(args []string, now func() time.Time) (v1.CommandEnvelope, error) {
	fs := newCommandFlagSet(
		"health-check",
		"acm health-check --project <id> [--include-details[=true|false]] [--max-findings-per-check <n>]",
		"acm health --project myproject --include-details --max-findings-per-check 50",
	)
	projectID, requestID := addProjectAndRequestFlags(fs)
	includeDetails := optionalBoolFlag{}
	fs.Var(&includeDetails, "include-details", "include detailed findings (optional bool)")
	maxFindingsPerCheck := fs.Int("max-findings-per-check", -1, "maximum findings per check (1..500)")
	if err := parseCommandFlags(fs, args); err != nil {
		return v1.CommandEnvelope{}, err
	}
	if err := requireFlag("project", *projectID); err != nil {
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
		"acm health-fix --project <id> [--apply[=true|false]] [--project-root <path>] [--rules-file <path>] [--tags-file <path>] [--fixer <name>]...",
		"acm health-fix --project myproject --apply --fixer sync_working_tree",
	)
	projectID, requestID := addProjectAndRequestFlags(fs)
	applyChanges := optionalBoolFlag{}
	fs.Var(&applyChanges, "apply", "apply actions instead of dry-run (optional bool)")
	projectRoot := fs.String("project-root", "", "project root for fixers")
	rulesFile := fs.String("rules-file", "", "explicit canonical rules file path (overrides default discovery)")
	tagsFile := fs.String("tags-file", "", "explicit canonical tag dictionary file path (overrides default discovery)")
	var fixers repeatedStringFlag
	fs.Var(&fixers, "fixer", "fixer to run (repeatable): sync_working_tree|index_uncovered_files|sync_ruleset")
	if err := parseCommandFlags(fs, args); err != nil {
		return v1.CommandEnvelope{}, err
	}
	if err := requireFlag("project", *projectID); err != nil {
		return v1.CommandEnvelope{}, err
	}

	payload := v1.HealthFixPayload{
		ProjectID:   strings.TrimSpace(*projectID),
		ProjectRoot: strings.TrimSpace(*projectRoot),
		RulesFile:   strings.TrimSpace(*rulesFile),
		TagsFile:    strings.TrimSpace(*tagsFile),
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
		"acm coverage --project <id> [--project-root <path>]",
		"acm coverage --project myproject --project-root .",
	)
	projectID, requestID := addProjectAndRequestFlags(fs)
	projectRoot := fs.String("project-root", "", "project root")
	if err := parseCommandFlags(fs, args); err != nil {
		return v1.CommandEnvelope{}, err
	}
	if err := requireFlag("project", *projectID); err != nil {
		return v1.CommandEnvelope{}, err
	}

	payload := v1.CoveragePayload{
		ProjectID:   strings.TrimSpace(*projectID),
		ProjectRoot: strings.TrimSpace(*projectRoot),
	}
	return buildEnvelope(v1.CommandCoverage, *requestID, payload, now)
}

func buildEvalEnvelope(args []string, now func() time.Time) (v1.CommandEnvelope, error) {
	fs := newCommandFlagSet(
		"eval",
		"acm eval --project <id> (--eval-suite-path <path> | --eval-suite-inline-file <path> | --eval-suite-inline-json <json>) [--minimum-recall <0..1>] [--tags-file <path>]",
		"acm eval --project myproject --eval-suite-path ./eval.json --minimum-recall 0.9",
	)
	projectID, requestID := addProjectAndRequestFlags(fs)
	evalSuitePath := fs.String("eval-suite-path", "", "path to eval suite JSON file")
	evalSuiteInlineFile := fs.String("eval-suite-inline-file", "", "path to JSON array of inline eval cases")
	evalSuiteInlineJSON := fs.String("eval-suite-inline-json", "", "inline JSON array of eval cases")
	minimumRecall := fs.Float64("minimum-recall", -1, "minimum recall threshold (0..1)")
	tagsFile := fs.String("tags-file", "", "explicit canonical tag dictionary file path (overrides default discovery)")
	if err := parseCommandFlags(fs, args); err != nil {
		return v1.CommandEnvelope{}, err
	}
	if err := requireFlag("project", *projectID); err != nil {
		return v1.CommandEnvelope{}, err
	}

	trimmedEvalSuitePath := strings.TrimSpace(*evalSuitePath)
	trimmedEvalSuiteInlineFile := strings.TrimSpace(*evalSuiteInlineFile)
	trimmedEvalSuiteInlineJSON := strings.TrimSpace(*evalSuiteInlineJSON)
	selectedSources := 0
	if trimmedEvalSuitePath != "" {
		selectedSources++
	}
	if trimmedEvalSuiteInlineFile != "" {
		selectedSources++
	}
	if trimmedEvalSuiteInlineJSON != "" {
		selectedSources++
	}
	if selectedSources == 0 {
		return v1.CommandEnvelope{}, fmt.Errorf("--eval-suite-path, --eval-suite-inline-file, or --eval-suite-inline-json is required")
	}
	if selectedSources > 1 {
		return v1.CommandEnvelope{}, fmt.Errorf("use only one of --eval-suite-path, --eval-suite-inline-file, or --eval-suite-inline-json")
	}

	payload := v1.EvalPayload{
		ProjectID:     strings.TrimSpace(*projectID),
		EvalSuitePath: trimmedEvalSuitePath,
		TagsFile:      strings.TrimSpace(*tagsFile),
	}
	if trimmedEvalSuiteInlineFile != "" {
		evalSuiteInline, err := readEvalCasesFromFile(trimmedEvalSuiteInlineFile)
		if err != nil {
			return v1.CommandEnvelope{}, err
		}
		payload.EvalSuiteInline = evalSuiteInline
	}
	if trimmedEvalSuiteInlineJSON != "" {
		evalSuiteInline, err := readEvalCasesFromJSON(trimmedEvalSuiteInlineJSON)
		if err != nil {
			return v1.CommandEnvelope{}, err
		}
		payload.EvalSuiteInline = evalSuiteInline
	}
	if *minimumRecall >= 0 {
		payload.MinimumRecall = minimumRecall
	}

	return buildEnvelope(v1.CommandEval, *requestID, payload, now)
}

func buildVerifyEnvelope(args []string, now func() time.Time) (v1.CommandEnvelope, error) {
	fs := newCommandFlagSet(
		"verify",
		"acm verify --project <id> [--receipt-id <id>] [--plan-key <key>] [--phase <plan|execute|review>] [--test-id <id>]... [--file-changed <path>]... [--files-changed-file <path>|--files-changed-json <json>] [--tests-file <path>] [--tags-file <path>] [--dry-run]",
		"acm verify --project myproject --phase review --file-changed internal/service/postgres/service.go --dry-run",
	)
	projectID, requestID := addProjectAndRequestFlags(fs)
	receiptID := fs.String("receipt-id", "", "receipt ID")
	planKey := fs.String("plan-key", "", "plan key")
	phase := fs.String("phase", "", "phase: plan|execute|review")
	testsFile := fs.String("tests-file", "", "explicit verification definitions file path (overrides default discovery)")
	tagsFile := fs.String("tags-file", "", "explicit canonical tag dictionary file path (overrides default discovery)")
	filesChangedFile := fs.String("files-changed-file", "", "JSON file containing an array of changed files")
	filesChangedJSON := fs.String("files-changed-json", "", "inline JSON array of changed files")
	dryRun := fs.Bool("dry-run", false, "select tests without executing them")
	var testIDs repeatedStringFlag
	var filesChanged repeatedStringFlag
	fs.Var(&testIDs, "test-id", "verification test id (repeatable)")
	fs.Var(&filesChanged, "file-changed", "repository-relative changed file path (repeatable)")
	if err := parseCommandFlags(fs, args); err != nil {
		return v1.CommandEnvelope{}, err
	}
	if err := requireFlag("project", *projectID); err != nil {
		return v1.CommandEnvelope{}, err
	}

	allFilesChanged := filesChanged.Values()
	if trimmedFilesChangedFile := strings.TrimSpace(*filesChangedFile); trimmedFilesChangedFile != "" {
		fileValues, err := readStringListFromFile(trimmedFilesChangedFile, "--files-changed-file")
		if err != nil {
			return v1.CommandEnvelope{}, err
		}
		allFilesChanged = mergeUnique(allFilesChanged, fileValues)
	}
	if trimmedFilesChangedJSON := strings.TrimSpace(*filesChangedJSON); trimmedFilesChangedJSON != "" {
		inlineValues, err := readStringListFromJSON(trimmedFilesChangedJSON, "--files-changed-json")
		if err != nil {
			return v1.CommandEnvelope{}, err
		}
		allFilesChanged = mergeUnique(allFilesChanged, inlineValues)
	}

	payload := v1.VerifyPayload{
		ProjectID:    strings.TrimSpace(*projectID),
		ReceiptID:    strings.TrimSpace(*receiptID),
		PlanKey:      strings.TrimSpace(*planKey),
		TestIDs:      mergeUnique(nil, testIDs.Values()),
		FilesChanged: allFilesChanged,
		TestsFile:    strings.TrimSpace(*testsFile),
		TagsFile:     strings.TrimSpace(*tagsFile),
		DryRun:       *dryRun,
	}
	if trimmedPhase := strings.TrimSpace(*phase); trimmedPhase != "" {
		payload.Phase = v1.Phase(trimmedPhase)
	}
	if len(payload.TestIDs) == 0 && payload.ReceiptID == "" && payload.PlanKey == "" && payload.Phase == "" && len(payload.FilesChanged) == 0 {
		return v1.CommandEnvelope{}, fmt.Errorf("verify requires --test-id or selection context (--receipt-id, --plan-key, --phase, or --file-changed)")
	}

	return buildEnvelope(v1.CommandVerify, *requestID, payload, now)
}

func buildBootstrapEnvelope(args []string, now func() time.Time) (v1.CommandEnvelope, error) {
	fs := newCommandFlagSet(
		"bootstrap",
		"acm bootstrap --project <id> --project-root <path> [--rules-file <path>] [--tags-file <path>] [--persist-candidates[=true|false]] [--respect-gitignore[=true|false]] [--llm-assist-descriptions[=true|false]] [--output-candidates-path <path>]",
		"acm bootstrap --project myproject --project-root . --respect-gitignore",
	)
	projectID, requestID := addProjectAndRequestFlags(fs)
	projectRoot := fs.String("project-root", "", "project root to analyze")
	rulesFile := fs.String("rules-file", "", "explicit canonical rules file path (overrides default discovery)")
	tagsFile := fs.String("tags-file", "", "explicit canonical tag dictionary file path (overrides default discovery)")
	persistCandidates := optionalBoolFlag{}
	fs.Var(&persistCandidates, "persist-candidates", "persist bootstrap candidates to disk (optional bool)")
	respectGitIgnore := optionalBoolFlag{}
	fs.Var(&respectGitIgnore, "respect-gitignore", "respect .gitignore while scanning (optional bool)")
	llmAssistDescriptions := optionalBoolFlag{}
	fs.Var(&llmAssistDescriptions, "llm-assist-descriptions", "enable generated description assistance (optional bool)")
	outputCandidatesPath := fs.String("output-candidates-path", "", "output file for bootstrap candidates (implies persistence)")
	if err := parseCommandFlags(fs, args); err != nil {
		return v1.CommandEnvelope{}, err
	}
	if err := requireFlag("project", *projectID); err != nil {
		return v1.CommandEnvelope{}, err
	}
	if err := requireFlag("project-root", *projectRoot); err != nil {
		return v1.CommandEnvelope{}, err
	}

	payload := v1.BootstrapPayload{
		ProjectID:   strings.TrimSpace(*projectID),
		ProjectRoot: strings.TrimSpace(*projectRoot),
		RulesFile:   strings.TrimSpace(*rulesFile),
		TagsFile:    strings.TrimSpace(*tagsFile),
	}
	if respectGitIgnore.IsSet() {
		payload.RespectGitIgnore = respectGitIgnore.Ptr()
	}
	if persistCandidates.IsSet() {
		payload.PersistCandidates = persistCandidates.Ptr()
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

func mergeUnique(base []string, additional []string) []string {
	seen := make(map[string]struct{}, len(base)+len(additional))
	out := make([]string, 0, len(base)+len(additional))
	for _, raw := range append(append([]string(nil), base...), additional...) {
		trimmed := strings.TrimSpace(raw)
		if trimmed == "" {
			continue
		}
		if _, exists := seen[trimmed]; exists {
			continue
		}
		seen[trimmed] = struct{}{}
		out = append(out, trimmed)
	}
	return out
}

func readTextFile(path string) (string, error) {
	if strings.TrimSpace(path) == "-" {
		blob, err := io.ReadAll(os.Stdin)
		if err != nil {
			return "", err
		}
		return string(blob), nil
	}
	blob, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return string(blob), nil
}

func readWorkItemsFromFile(path string) ([]v1.WorkItemPayload, error) {
	var items []v1.WorkItemPayload
	if err := readJSONFileStrict(path, &items); err != nil {
		return nil, fmt.Errorf("read --items-file %s: %w", path, err)
	}
	return items, nil
}

func readWorkPlanFromFile(path string) (v1.WorkPlanPayload, error) {
	var plan v1.WorkPlanPayload
	if err := readJSONFileStrict(path, &plan); err != nil {
		return v1.WorkPlanPayload{}, fmt.Errorf("read --plan-file %s: %w", path, err)
	}
	return plan, nil
}

func readWorkPlanFromJSON(raw string) (v1.WorkPlanPayload, error) {
	var plan v1.WorkPlanPayload
	if err := readJSONInlineStrict(raw, &plan); err != nil {
		return v1.WorkPlanPayload{}, fmt.Errorf("read --plan-json: %w", err)
	}
	return plan, nil
}

func readWorkTasksFromFile(path string) ([]v1.WorkTaskPayload, error) {
	var tasks []v1.WorkTaskPayload
	if err := readJSONFileStrict(path, &tasks); err != nil {
		return nil, fmt.Errorf("read --tasks-file %s: %w", path, err)
	}
	return tasks, nil
}

func readWorkTasksFromJSON(raw string) ([]v1.WorkTaskPayload, error) {
	var tasks []v1.WorkTaskPayload
	if err := readJSONInlineStrict(raw, &tasks); err != nil {
		return nil, fmt.Errorf("read --tasks-json: %w", err)
	}
	return tasks, nil
}

func readWorkItemsFromJSON(raw string) ([]v1.WorkItemPayload, error) {
	var items []v1.WorkItemPayload
	if err := readJSONInlineStrict(raw, &items); err != nil {
		return nil, fmt.Errorf("read --items-json: %w", err)
	}
	return items, nil
}

func readEvalCasesFromFile(path string) ([]v1.EvalCase, error) {
	var cases []v1.EvalCase
	if err := readJSONFileStrict(path, &cases); err != nil {
		return nil, fmt.Errorf("read --eval-suite-inline-file %s: %w", path, err)
	}
	return cases, nil
}

func readEvalCasesFromJSON(raw string) ([]v1.EvalCase, error) {
	var cases []v1.EvalCase
	if err := readJSONInlineStrict(raw, &cases); err != nil {
		return nil, fmt.Errorf("read --eval-suite-inline-json: %w", err)
	}
	return cases, nil
}

func readStringListFromFile(path, flagName string) ([]string, error) {
	var values []string
	if err := readJSONFileStrict(path, &values); err != nil {
		return nil, fmt.Errorf("read %s %s: %w", flagName, path, err)
	}
	return mergeUnique(nil, values), nil
}

func readStringListFromJSON(raw, flagName string) ([]string, error) {
	var values []string
	if err := readJSONInlineStrict(raw, &values); err != nil {
		return nil, fmt.Errorf("read %s: %w", flagName, err)
	}
	return mergeUnique(nil, values), nil
}

func readStringMapFromFile(path, flagName string) (map[string]string, error) {
	values := map[string]string{}
	if err := readJSONFileStrict(path, &values); err != nil {
		return nil, fmt.Errorf("read %s %s: %w", flagName, path, err)
	}
	normalized := make(map[string]string, len(values))
	for k, v := range values {
		key := strings.TrimSpace(k)
		value := strings.TrimSpace(v)
		if key == "" || value == "" {
			continue
		}
		normalized[key] = value
	}
	return normalized, nil
}

func readStringMapFromJSON(raw, flagName string) (map[string]string, error) {
	values := map[string]string{}
	if err := readJSONInlineStrict(raw, &values); err != nil {
		return nil, fmt.Errorf("read %s: %w", flagName, err)
	}
	normalized := make(map[string]string, len(values))
	for k, v := range values {
		key := strings.TrimSpace(k)
		value := strings.TrimSpace(v)
		if key == "" || value == "" {
			continue
		}
		normalized[key] = value
	}
	return normalized, nil
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

	return decodeJSONStrict(data, out)
}

func readJSONInlineStrict(raw string, out any) error {
	return decodeJSONStrict([]byte(raw), out)
}

func decodeJSONStrict(data []byte, out any) error {
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
