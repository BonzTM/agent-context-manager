package v1

import (
	"encoding/json"
	"reflect"
	"strings"
)

type CommandGroup string

const (
	CommandGroupWorkflow    CommandGroup = "workflow"
	CommandGroupMaintenance CommandGroup = "maintenance"
)

type CommandSpec struct {
	Command         Command
	CLISubcommand   string
	CLIUsage        string
	CLISummary      string
	Group           CommandGroup
	InputSchemaDef  string
	ResultSchemaDef string
	ToolTitle       string
	ToolDescription string
	decode          func(json.RawMessage, ValidationDefaults) (any, *ErrorPayload)
}

func (s CommandSpec) Decode(raw json.RawMessage, defaults ValidationDefaults) (any, *ErrorPayload) {
	if s.decode == nil {
		return nil, validationError("INVALID_COMMAND", "command is not recognized")
	}
	return s.decode(raw, normalizeValidationDefaults(defaults))
}

var commandCatalog = []CommandSpec{
	newCommandSpec(
		CommandGetContext,
		"get-context",
		"acm get-context [--project <id>] [--task-text <text>|--task-file <path>] [--tags-file <path>] [--unbounded[=true|false]] [flags]",
		"Resolve a scoped receipt with rules, pointers, memories, and active work.",
		CommandGroupWorkflow,
		"getContextPayload",
		"getContextResult",
		"Get Task Context",
		"Deterministically resolve task-scoped pointers, rules, and memories.",
		func(raw json.RawMessage, defaults ValidationDefaults) (any, *ErrorPayload) {
			return decodeValidatedCommandPayload(raw, defaults,
				func(p *GetContextPayload, defaults ValidationDefaults) {
					p.ProjectID = defaultProjectID(p.ProjectID, defaults)
				},
				validateGetContextPayload,
			)
		},
	),
	newCommandSpec(
		CommandFetch,
		"fetch",
		"acm fetch [--project <id>] [--key <pointer>]... [--keys-file <path>] [--keys-json <json>] [--receipt-id <id>] [--expect <key=version>]... [--expected-versions-file <path>] [--expected-versions-json <json>]",
		"Fetch pointer, plan, or task content by key, with optional version checks.",
		CommandGroupWorkflow,
		"fetchPayload",
		"fetchResult",
		"Fetch Task Context",
		"Fetch deterministic task context by pointer keys and optional expected versions.",
		func(raw json.RawMessage, defaults ValidationDefaults) (any, *ErrorPayload) {
			return decodeValidatedCommandPayloadWithFields(raw, defaults,
				func(p *FetchPayload, defaults ValidationDefaults) {
					p.ProjectID = defaultProjectID(p.ProjectID, defaults)
				},
				validateFetchPayload,
			)
		},
	),
	newCommandSpec(
		CommandProposeMemory,
		"propose-memory",
		"acm propose-memory [--project <id>] --receipt-id <id> --category <name> --subject <text> (--content <text>|--content-file <path>) --confidence <1-5> [--memory-tag <tag>]... [--memory-tags-file <path>|--memory-tags-json <json>] [--tags-file <path>] [flags]",
		"Propose durable memory tied to a receipt, evidence, and canonical tags.",
		CommandGroupWorkflow,
		"proposeMemoryPayload",
		"proposeMemoryResult",
		"Propose Durable Memory",
		"Submit a memory candidate tied to evidence and receipt scope.",
		func(raw json.RawMessage, defaults ValidationDefaults) (any, *ErrorPayload) {
			return decodeValidatedCommandPayload(raw, defaults,
				func(p *ProposeMemoryPayload, defaults ValidationDefaults) {
					p.ProjectID = defaultProjectID(p.ProjectID, defaults)
				},
				validateProposeMemoryPayload,
			)
		},
	),
	newCommandSpec(
		CommandReportCompletion,
		"report-completion",
		"acm report-completion [--project <id>] --receipt-id <id> [--outcome <text>|--outcome-file <path>] [--file-changed <path>]... [--files-changed-file <path>] [--files-changed-json <json>] [--scope-mode <mode>] [--tags-file <path>]",
		"Close a receipt, validate scope, and enforce configured completion task gates.",
		CommandGroupWorkflow,
		"reportCompletionPayload",
		"reportCompletionResult",
		"Report Task Completion",
		"Validate changed files against the active receipt and persist run summary.",
		func(raw json.RawMessage, defaults ValidationDefaults) (any, *ErrorPayload) {
			return decodeValidatedCommandPayloadWithFields(raw, defaults,
				func(p *ReportCompletionPayload, defaults ValidationDefaults) {
					p.ProjectID = defaultProjectID(p.ProjectID, defaults)
				},
				validateReportCompletionPayload,
			)
		},
	),
	newCommandSpec(
		CommandReview,
		"review",
		"acm review [--project <id>] [--receipt-id <id>|--plan-key <key>] [--run] [--key <task-key>] [--summary <text>] [--status <pending|in_progress|complete|blocked>] [--outcome <text>|--outcome-file <path>] [--blocked-reason <text>] [--evidence <text>]... [--evidence-file <path>|--evidence-json <json>] [--tags-file <path>]",
		"Record or execute a single review gate such as `review:cross-llm` through the work tracker.",
		CommandGroupWorkflow,
		"reviewPayload",
		"reviewResult",
		"Record Review Gate",
		"Record or execute a single review task gate such as review:cross-llm through the work tracker.",
		func(raw json.RawMessage, defaults ValidationDefaults) (any, *ErrorPayload) {
			return decodeValidatedCommandPayloadWithFields(raw, defaults,
				func(p *ReviewPayload, defaults ValidationDefaults) {
					p.ProjectID = defaultProjectID(p.ProjectID, defaults)
				},
				validateReviewPayload,
			)
		},
	),
	newCommandSpec(
		CommandWork,
		"work",
		"acm work [--project <id>] [--plan-key <key>|--receipt-id <id>] [--plan-title <text>] [--mode <merge|replace>] [--plan-file <path>|--plan-json <json>] [--tasks-file <path>|--tasks-json <json>]",
		"Create or update structured plans and tasks that survive compaction.",
		CommandGroupWorkflow,
		"workPayload",
		"workResult",
		"Submit Completion Work",
		"Submit plan-scoped work task updates with status and optional outcomes.",
		func(raw json.RawMessage, defaults ValidationDefaults) (any, *ErrorPayload) {
			return decodeValidatedCommandPayload(raw, defaults,
				func(p *WorkPayload, defaults ValidationDefaults) {
					p.ProjectID = defaultProjectID(p.ProjectID, defaults)
				},
				validateWorkPayload,
			)
		},
	),
	newCommandSpec(
		CommandHistorySearch,
		"history-search",
		"acm history search [--project <id>] [--entity <all|work|memory|receipt|run>] [--query <text>|--query-file <path>] [--limit <n>] [--unbounded[=true|false]]",
		"Search recent work, memory, receipt, and run history without direct database access.",
		CommandGroupWorkflow,
		"historySearchPayload",
		"historySearchResult",
		"Search History",
		"List or search work plans, memories, receipts, and runs without direct database access.",
		func(raw json.RawMessage, defaults ValidationDefaults) (any, *ErrorPayload) {
			return decodeValidatedCommandPayload(raw, defaults,
				func(p *HistorySearchPayload, defaults ValidationDefaults) {
					p.ProjectID = defaultProjectID(p.ProjectID, defaults)
				},
				validateHistorySearchPayload,
			)
		},
	),
	newCommandSpec(
		CommandSync,
		"sync",
		"acm sync [--project <id>] [--mode changed|full|working_tree] [--git-range <range>] [--project-root <path>] [--rules-file <path>] [--tags-file <path>] [--insert-new-candidates[=true|false]]",
		"Refresh repository pointers and canonical rules from the working tree or git history.",
		CommandGroupMaintenance,
		"syncPayload",
		"syncResult",
		"Sync Project Inventory",
		"Refresh indexed repository pointers from git or the working tree.",
		func(raw json.RawMessage, defaults ValidationDefaults) (any, *ErrorPayload) {
			return decodeValidatedCommandPayload(raw, defaults,
				func(p *SyncPayload, defaults ValidationDefaults) {
					p.ProjectID = defaultProjectIDForRoot(p.ProjectID, p.ProjectRoot, defaults)
				},
				validateSyncPayload,
			)
		},
	),
	newCommandSpec(
		CommandHealthCheck,
		"health-check",
		"acm health-check [--project <id>] [--include-details[=true|false]] [--max-findings-per-check <n>]",
		"Inspect repository health without making changes.",
		CommandGroupMaintenance,
		"healthCheckPayload",
		"healthCheckResult",
		"Check Project Health",
		"Inspect ACM repository health and return findings without making changes.",
		func(raw json.RawMessage, defaults ValidationDefaults) (any, *ErrorPayload) {
			return decodeValidatedCommandPayload(raw, defaults,
				func(p *HealthCheckPayload, defaults ValidationDefaults) {
					p.ProjectID = defaultProjectID(p.ProjectID, defaults)
				},
				validateHealthCheckPayload,
			)
		},
	),
	newCommandSpec(
		CommandHealthFix,
		"health-fix",
		"acm health-fix [--project <id>] [--apply[=true|false]] [--project-root <path>] [--rules-file <path>] [--tags-file <path>] [--fixer <name>]...",
		"Plan or apply repair actions such as sync_working_tree, index_uncovered_files, and sync_ruleset.",
		CommandGroupMaintenance,
		"healthFixPayload",
		"healthFixResult",
		"Fix Project Health",
		"Plan or apply ACM health fixes such as ruleset sync and working tree repair.",
		func(raw json.RawMessage, defaults ValidationDefaults) (any, *ErrorPayload) {
			return decodeValidatedCommandPayloadWithFields(raw, defaults,
				func(p *HealthFixPayload, defaults ValidationDefaults) {
					p.ProjectID = defaultProjectIDForRoot(p.ProjectID, p.ProjectRoot, defaults)
				},
				validateHealthFixPayload,
			)
		},
	),
	newCommandSpec(
		CommandStatus,
		"status",
		"acm status [--project <id>] [--project-root <path>] [--rules-file <path>] [--tags-file <path>] [--tests-file <path>] [--workflows-file <path>] [--task-text <text>|--task-file <path>] [--phase <plan|execute|review>]",
		"Explain active project/runtime state, loaded ACM files, installed integrations, and optional retrieval reasoning.",
		CommandGroupMaintenance,
		"statusPayload",
		"statusResult",
		"Inspect ACM Status",
		"Explain current project/runtime state, loaded ACM files, installed integrations, and optional get_context retrieval reasoning.",
		func(raw json.RawMessage, defaults ValidationDefaults) (any, *ErrorPayload) {
			return decodeValidatedCommandPayload(raw, defaults,
				func(p *StatusPayload, defaults ValidationDefaults) {
					p.ProjectID = defaultProjectIDForRoot(p.ProjectID, p.ProjectRoot, defaults)
				},
				validateStatusPayload,
			)
		},
	),
	newCommandSpec(
		CommandCoverage,
		"coverage",
		"acm coverage [--project <id>] [--project-root <path>]",
		"Measure repository indexing coverage against the current project tree.",
		CommandGroupMaintenance,
		"coveragePayload",
		"coverageResult",
		"Measure Coverage",
		"Report repository indexing coverage against the current project tree.",
		func(raw json.RawMessage, defaults ValidationDefaults) (any, *ErrorPayload) {
			return decodeValidatedCommandPayload(raw, defaults,
				func(p *CoveragePayload, defaults ValidationDefaults) {
					p.ProjectID = defaultProjectIDForRoot(p.ProjectID, p.ProjectRoot, defaults)
				},
				validateCoveragePayload,
			)
		},
	),
	newCommandSpec(
		CommandEval,
		"eval",
		"acm eval [--project <id>] (--eval-suite-path <path> | --eval-suite-inline-file <path> | --eval-suite-inline-json <json>) [--minimum-recall <0..1>] [--tags-file <path>]",
		"Run retrieval-quality evaluation cases against ACM context selection.",
		CommandGroupMaintenance,
		"evalPayload",
		"evalResult",
		"Run Retrieval Evaluation",
		"Run retrieval evaluation cases against ACM context selection behavior.",
		func(raw json.RawMessage, defaults ValidationDefaults) (any, *ErrorPayload) {
			return decodeValidatedCommandPayload(raw, defaults,
				func(p *EvalPayload, defaults ValidationDefaults) {
					p.ProjectID = defaultProjectID(p.ProjectID, defaults)
				},
				validateEvalPayload,
			)
		},
	),
	newCommandSpec(
		CommandVerify,
		"verify",
		"acm verify [--project <id>] [--receipt-id <id>] [--plan-key <key>] [--phase <plan|execute|review>] [--test-id <id>]... [--file-changed <path>]... [--files-changed-file <path>|--files-changed-json <json>] [--tests-file <path>] [--tags-file <path>] [--dry-run]",
		"Select and execute repo-defined verification checks from `.acm/acm-tests.yaml` or `acm-tests.yaml`.",
		CommandGroupMaintenance,
		"verifyPayload",
		"verifyResult",
		"Run Executable Verification",
		"Select and execute repo-defined verification checks from acm test definitions.",
		func(raw json.RawMessage, defaults ValidationDefaults) (any, *ErrorPayload) {
			return decodeValidatedCommandPayloadWithFields(raw, defaults,
				func(p *VerifyPayload, defaults ValidationDefaults) {
					p.ProjectID = defaultProjectID(p.ProjectID, defaults)
				},
				validateVerifyPayload,
			)
		},
	),
	newCommandSpec(
		CommandBootstrap,
		"bootstrap",
		"acm bootstrap [--project <id>] [--project-root <path>] [--apply-template <id>]... [--rules-file <path>] [--tags-file <path>] [--persist-candidates[=true|false]] [--respect-gitignore[=true|false]] [--output-candidates-path <path>]",
		"Seed repo-local ACM files, optionally apply additive templates, and scan a repository for initial pointer candidates.",
		CommandGroupMaintenance,
		"bootstrapPayload",
		"bootstrapResult",
		"Bootstrap Repository",
		"Scan a repository, seed ACM files, and optionally apply additive bootstrap templates.",
		func(raw json.RawMessage, defaults ValidationDefaults) (any, *ErrorPayload) {
			return decodeValidatedCommandPayloadWithFields(raw, defaults,
				func(p *BootstrapPayload, defaults ValidationDefaults) {
					p.ProjectID = defaultProjectIDForRoot(p.ProjectID, p.ProjectRoot, defaults)
				},
				validateBootstrapPayload,
			)
		},
	),
}

var commandCatalogByCommand = buildCommandCatalogByCommand(commandCatalog)

func CommandCatalog() []CommandSpec {
	out := make([]CommandSpec, len(commandCatalog))
	copy(out, commandCatalog)
	return out
}

func CommandSpecs() []CommandSpec {
	return CommandCatalog()
}

func WorkflowCommandCatalog() []CommandSpec {
	return commandCatalogByGroup(CommandGroupWorkflow)
}

func MaintenanceCommandCatalog() []CommandSpec {
	return commandCatalogByGroup(CommandGroupMaintenance)
}

func LookupCommandSpec(command Command) (CommandSpec, bool) {
	spec, ok := commandCatalogByCommand[command]
	return spec, ok
}

func LookupCommand(command Command) (CommandSpec, bool) {
	return LookupCommandSpec(command)
}

func LookupCommandByCLISubcommand(subcommand string) (CommandSpec, bool) {
	trimmed := strings.TrimSpace(subcommand)
	for _, spec := range commandCatalog {
		if spec.CLISubcommand == trimmed {
			return spec, true
		}
	}
	return CommandSpec{}, false
}

func CommandNames() []string {
	out := make([]string, 0, len(commandCatalog))
	for _, spec := range commandCatalog {
		out = append(out, string(spec.Command))
	}
	return out
}

func CommandFromToolName(tool string) (Command, bool) {
	spec, ok := LookupCommand(Command(strings.TrimSpace(tool)))
	if !ok {
		return "", false
	}
	return spec.Command, true
}

func BuildEnvelopeForCommand(command Command, requestID string, payload json.RawMessage) ([]byte, error) {
	return json.Marshal(CommandEnvelope{
		Version:   Version,
		Command:   command,
		RequestID: strings.TrimSpace(requestID),
		Payload:   payload,
	})
}

func CommandEnum() []string {
	return CommandNames()
}

func ProjectIDFromPayload(payload any) string {
	switch p := payload.(type) {
	case GetContextPayload:
		return strings.TrimSpace(p.ProjectID)
	case FetchPayload:
		return strings.TrimSpace(p.ProjectID)
	case ProposeMemoryPayload:
		return strings.TrimSpace(p.ProjectID)
	case ReportCompletionPayload:
		return strings.TrimSpace(p.ProjectID)
	case ReviewPayload:
		return strings.TrimSpace(p.ProjectID)
	case WorkPayload:
		return strings.TrimSpace(p.ProjectID)
	case HistorySearchPayload:
		return strings.TrimSpace(p.ProjectID)
	case SyncPayload:
		return strings.TrimSpace(p.ProjectID)
	case HealthCheckPayload:
		return strings.TrimSpace(p.ProjectID)
	case HealthFixPayload:
		return strings.TrimSpace(p.ProjectID)
	case StatusPayload:
		return strings.TrimSpace(p.ProjectID)
	case CoveragePayload:
		return strings.TrimSpace(p.ProjectID)
	case EvalPayload:
		return strings.TrimSpace(p.ProjectID)
	case VerifyPayload:
		return strings.TrimSpace(p.ProjectID)
	case BootstrapPayload:
		return strings.TrimSpace(p.ProjectID)
	case map[string]any:
		if raw, ok := p["project_id"].(string); ok {
			return strings.TrimSpace(raw)
		}
	case map[string]string:
		return strings.TrimSpace(p["project_id"])
	default:
		value := reflect.ValueOf(payload)
		if !value.IsValid() {
			return ""
		}
		if value.Kind() == reflect.Pointer {
			if value.IsNil() {
				return ""
			}
			value = value.Elem()
		}
		if value.Kind() != reflect.Struct {
			return ""
		}
		field := value.FieldByName("ProjectID")
		if !field.IsValid() || field.Kind() != reflect.String {
			return ""
		}
		return strings.TrimSpace(field.String())
	}
	return ""
}

func decodeValidatedCommandPayload[Payload any](
	raw json.RawMessage,
	defaults ValidationDefaults,
	applyDefaults func(*Payload, ValidationDefaults),
	validate func(*Payload) error,
) (any, *ErrorPayload) {
	var payload Payload
	if err := decodeStrict(raw, &payload); err != nil {
		return nil, validationError("INVALID_PAYLOAD", err.Error())
	}
	if applyDefaults != nil {
		applyDefaults(&payload, defaults)
	}
	if validate != nil {
		if err := validate(&payload); err != nil {
			return nil, validationError("INVALID_PAYLOAD", err.Error())
		}
	}
	return payload, nil
}

func decodeValidatedCommandPayloadWithFields[Payload any](
	raw json.RawMessage,
	defaults ValidationDefaults,
	applyDefaults func(*Payload, ValidationDefaults),
	validate func(*Payload, map[string]json.RawMessage) error,
) (any, *ErrorPayload) {
	var payload Payload
	if err := decodeStrict(raw, &payload); err != nil {
		return nil, validationError("INVALID_PAYLOAD", err.Error())
	}
	if applyDefaults != nil {
		applyDefaults(&payload, defaults)
	}
	fields, err := decodeObjectFields(raw)
	if err != nil {
		return nil, validationError("INVALID_PAYLOAD", err.Error())
	}
	if validate != nil {
		if err := validate(&payload, fields); err != nil {
			return nil, validationError("INVALID_PAYLOAD", err.Error())
		}
	}
	return payload, nil
}

func newCommandSpec(
	command Command,
	cliSubcommand string,
	cliUsage string,
	cliSummary string,
	group CommandGroup,
	inputSchemaDef string,
	resultSchemaDef string,
	toolTitle string,
	toolDescription string,
	decode func(json.RawMessage, ValidationDefaults) (any, *ErrorPayload),
) CommandSpec {
	return CommandSpec{
		Command:         command,
		CLISubcommand:   cliSubcommand,
		CLIUsage:        cliUsage,
		CLISummary:      cliSummary,
		Group:           group,
		InputSchemaDef:  inputSchemaDef,
		ResultSchemaDef: resultSchemaDef,
		ToolTitle:       toolTitle,
		ToolDescription: toolDescription,
		decode:          decode,
	}
}

func buildCommandCatalogByCommand(specs []CommandSpec) map[Command]CommandSpec {
	out := make(map[Command]CommandSpec, len(specs))
	for _, spec := range specs {
		out[spec.Command] = spec
	}
	return out
}

func commandCatalogByGroup(group CommandGroup) []CommandSpec {
	out := make([]CommandSpec, 0, len(commandCatalog))
	for _, spec := range commandCatalog {
		if spec.Group == group {
			out = append(out, spec)
		}
	}
	return out
}
