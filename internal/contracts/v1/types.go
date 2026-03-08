package v1

import "encoding/json"

const Version = "acm.v1"

type Command string

const (
	CommandGetContext       Command = "get_context"
	CommandFetch            Command = "fetch"
	CommandProposeMemory    Command = "propose_memory"
	CommandReportCompletion Command = "report_completion"
	CommandReview           Command = "review"
	CommandWork             Command = "work"
	CommandHistorySearch    Command = "history_search"
	CommandSync             Command = "sync"
	CommandHealthCheck      Command = "health_check"
	CommandHealthFix        Command = "health_fix"
	CommandStatus           Command = "status"
	CommandCoverage         Command = "coverage"
	CommandEval             Command = "eval"
	CommandVerify           Command = "verify"
	CommandBootstrap        Command = "bootstrap"
)

type Phase string

const (
	PhasePlan    Phase = "plan"
	PhaseExecute Phase = "execute"
	PhaseReview  Phase = "review"
)

type ScopeMode string

const (
	ScopeModeStrict    ScopeMode = "strict"
	ScopeModeWarn      ScopeMode = "warn"
	ScopeModeAutoIndex ScopeMode = "auto_index"
)

type MemoryCategory string

const (
	MemoryCategoryDecision   MemoryCategory = "decision"
	MemoryCategoryGotcha     MemoryCategory = "gotcha"
	MemoryCategoryPattern    MemoryCategory = "pattern"
	MemoryCategoryPreference MemoryCategory = "preference"
)

type CommandEnvelope struct {
	Version   string          `json:"version"`
	Command   Command         `json:"command"`
	RequestID string          `json:"request_id"`
	Payload   json.RawMessage `json:"payload"`
}

type ErrorPayload struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Details any    `json:"details,omitempty"`
}

type ResultEnvelope struct {
	Version   string        `json:"version"`
	Command   Command       `json:"command"`
	RequestID string        `json:"request_id"`
	OK        bool          `json:"ok"`
	Timestamp string        `json:"timestamp"`
	Result    any           `json:"result,omitempty"`
	Error     *ErrorPayload `json:"error,omitempty"`
}

type RetrievalCaps struct {
	MaxNonRulePointers int `json:"max_non_rule_pointers,omitempty"`
	MaxRulePointers    int `json:"max_rule_pointers,omitempty"`
	MaxHops            int `json:"max_hops,omitempty"`
	MaxHopExpansion    int `json:"max_hop_expansion,omitempty"`
	MaxMemories        int `json:"max_memories,omitempty"`
	MinPointerCount    int `json:"min_pointer_count,omitempty"`
	WordBudgetLimit    int `json:"word_budget_limit,omitempty"`
}

type GetContextPayload struct {
	ProjectID    string         `json:"project_id"`
	TaskText     string         `json:"task_text"`
	Phase        Phase          `json:"phase"`
	TagsFile     string         `json:"tags_file,omitempty"`
	Unbounded    *bool          `json:"unbounded,omitempty"`
	Caps         *RetrievalCaps `json:"caps,omitempty"`
	AllowStale   bool           `json:"allow_stale,omitempty"`
	FallbackMode string         `json:"fallback_mode,omitempty"`
}

type FetchPayload struct {
	ProjectID        string            `json:"project_id"`
	Keys             []string          `json:"keys,omitempty"`
	ReceiptID        string            `json:"receipt_id,omitempty"`
	ExpectedVersions map[string]string `json:"expected_versions,omitempty"`
}

type MemoryPayload struct {
	Category            MemoryCategory `json:"category"`
	Subject             string         `json:"subject"`
	Content             string         `json:"content"`
	RelatedPointerKeys  []string       `json:"related_pointer_keys"`
	Tags                []string       `json:"tags"`
	Confidence          int            `json:"confidence"`
	EvidencePointerKeys []string       `json:"evidence_pointer_keys"`
}

type ProposeMemoryPayload struct {
	ProjectID   string        `json:"project_id"`
	ReceiptID   string        `json:"receipt_id"`
	TagsFile    string        `json:"tags_file,omitempty"`
	Memory      MemoryPayload `json:"memory"`
	AutoPromote *bool         `json:"auto_promote,omitempty"`
}

type ReportCompletionPayload struct {
	ProjectID    string    `json:"project_id"`
	ReceiptID    string    `json:"receipt_id"`
	TagsFile     string    `json:"tags_file,omitempty"`
	FilesChanged []string  `json:"files_changed"`
	Outcome      string    `json:"outcome"`
	ScopeMode    ScopeMode `json:"scope_mode,omitempty"`
}

type WorkItemStatus string

const (
	WorkItemStatusPending    WorkItemStatus = "pending"
	WorkItemStatusInProgress WorkItemStatus = "in_progress"
	WorkItemStatusComplete   WorkItemStatus = "complete"
	WorkItemStatusBlocked    WorkItemStatus = "blocked"
)

type WorkPlanMode string

const (
	WorkPlanModeMerge   WorkPlanMode = "merge"
	WorkPlanModeReplace WorkPlanMode = "replace"
)

type WorkPlanStagesPayload struct {
	SpecOutline        WorkItemStatus `json:"spec_outline,omitempty"`
	RefinedSpec        WorkItemStatus `json:"refined_spec,omitempty"`
	ImplementationPlan WorkItemStatus `json:"implementation_plan,omitempty"`
}

type WorkPlanPayload struct {
	Title         string                 `json:"title,omitempty"`
	Objective     string                 `json:"objective,omitempty"`
	Kind          string                 `json:"kind,omitempty"`
	ParentPlanKey string                 `json:"parent_plan_key,omitempty"`
	Status        WorkItemStatus         `json:"status,omitempty"`
	Stages        *WorkPlanStagesPayload `json:"stages,omitempty"`
	InScope       []string               `json:"in_scope,omitempty"`
	OutOfScope    []string               `json:"out_of_scope,omitempty"`
	Constraints   []string               `json:"constraints,omitempty"`
	References    []string               `json:"references,omitempty"`
	ExternalRefs  []string               `json:"external_refs,omitempty"`
}

type WorkTaskPayload struct {
	Key                string         `json:"key"`
	Summary            string         `json:"summary"`
	Status             WorkItemStatus `json:"status"`
	ParentTaskKey      string         `json:"parent_task_key,omitempty"`
	DependsOn          []string       `json:"depends_on,omitempty"`
	AcceptanceCriteria []string       `json:"acceptance_criteria,omitempty"`
	References         []string       `json:"references,omitempty"`
	ExternalRefs       []string       `json:"external_refs,omitempty"`
	BlockedReason      string         `json:"blocked_reason,omitempty"`
	Outcome            string         `json:"outcome,omitempty"`
	Evidence           []string       `json:"evidence,omitempty"`
}

type WorkPayload struct {
	ProjectID string            `json:"project_id"`
	PlanKey   string            `json:"plan_key,omitempty"`
	PlanTitle string            `json:"plan_title,omitempty"`
	ReceiptID string            `json:"receipt_id,omitempty"`
	Mode      WorkPlanMode      `json:"mode,omitempty"`
	Plan      *WorkPlanPayload  `json:"plan,omitempty"`
	Tasks     []WorkTaskPayload `json:"tasks,omitempty"`
}

type HistoryScope string

const (
	HistoryScopeCurrent   HistoryScope = "current"
	HistoryScopeDeferred  HistoryScope = "deferred"
	HistoryScopeCompleted HistoryScope = "completed"
	HistoryScopeAll       HistoryScope = "all"
)

type HistoryEntity string

const (
	HistoryEntityAll     HistoryEntity = "all"
	HistoryEntityMemory  HistoryEntity = "memory"
	HistoryEntityWork    HistoryEntity = "work"
	HistoryEntityReceipt HistoryEntity = "receipt"
	HistoryEntityRun     HistoryEntity = "run"
)

type HistorySearchPayload struct {
	ProjectID string        `json:"project_id"`
	Entity    HistoryEntity `json:"entity,omitempty"`
	Query     string        `json:"query,omitempty"`
	Scope     HistoryScope  `json:"scope,omitempty"`
	Kind      string        `json:"kind,omitempty"`
	Limit     int           `json:"limit,omitempty"`
	Unbounded *bool         `json:"unbounded,omitempty"`
}

type SyncPayload struct {
	ProjectID           string `json:"project_id"`
	Mode                string `json:"mode,omitempty"`
	GitRange            string `json:"git_range,omitempty"`
	ProjectRoot         string `json:"project_root,omitempty"`
	RulesFile           string `json:"rules_file,omitempty"`
	TagsFile            string `json:"tags_file,omitempty"`
	InsertNewCandidates *bool  `json:"insert_new_candidates,omitempty"`
}

type HealthCheckPayload struct {
	ProjectID           string `json:"project_id"`
	IncludeDetails      *bool  `json:"include_details,omitempty"`
	MaxFindingsPerCheck *int   `json:"max_findings_per_check,omitempty"`
}

type HealthFixer string

const (
	HealthFixerAll                HealthFixer = "all"
	HealthFixerSyncWorkingTree    HealthFixer = "sync_working_tree"
	HealthFixerIndexUncoveredFile HealthFixer = "index_uncovered_files"
	HealthFixerSyncRuleset        HealthFixer = "sync_ruleset"
)

type HealthFixPayload struct {
	ProjectID   string        `json:"project_id"`
	Apply       *bool         `json:"apply,omitempty"`
	ProjectRoot string        `json:"project_root,omitempty"`
	RulesFile   string        `json:"rules_file,omitempty"`
	TagsFile    string        `json:"tags_file,omitempty"`
	Fixers      []HealthFixer `json:"fixers,omitempty"`
}

type StatusPayload struct {
	ProjectID     string `json:"project_id"`
	ProjectRoot   string `json:"project_root,omitempty"`
	RulesFile     string `json:"rules_file,omitempty"`
	TagsFile      string `json:"tags_file,omitempty"`
	TestsFile     string `json:"tests_file,omitempty"`
	WorkflowsFile string `json:"workflows_file,omitempty"`
	TaskText      string `json:"task_text,omitempty"`
	Phase         Phase  `json:"phase,omitempty"`
}

type CoveragePayload struct {
	ProjectID   string `json:"project_id"`
	ProjectRoot string `json:"project_root,omitempty"`
}

type EvalCase struct {
	TaskText               string   `json:"task_text"`
	Phase                  Phase    `json:"phase"`
	ExpectedPointerKeys    []string `json:"expected_pointer_keys,omitempty"`
	ExpectedMemorySubjects []string `json:"expected_memory_subjects,omitempty"`
}

type EvalPayload struct {
	ProjectID       string     `json:"project_id"`
	EvalSuitePath   string     `json:"eval_suite_path,omitempty"`
	EvalSuiteInline []EvalCase `json:"eval_suite_inline,omitempty"`
	MinimumRecall   *float64   `json:"minimum_recall,omitempty"`
	TagsFile        string     `json:"tags_file,omitempty"`
}

type VerifyPayload struct {
	ProjectID    string   `json:"project_id"`
	ReceiptID    string   `json:"receipt_id,omitempty"`
	PlanKey      string   `json:"plan_key,omitempty"`
	Phase        Phase    `json:"phase,omitempty"`
	TestIDs      []string `json:"test_ids,omitempty"`
	FilesChanged []string `json:"files_changed,omitempty"`
	TestsFile    string   `json:"tests_file,omitempty"`
	TagsFile     string   `json:"tags_file,omitempty"`
	DryRun       bool     `json:"dry_run,omitempty"`
}

type BootstrapPayload struct {
	ProjectID            string   `json:"project_id"`
	ProjectRoot          string   `json:"project_root"`
	RulesFile            string   `json:"rules_file,omitempty"`
	TagsFile             string   `json:"tags_file,omitempty"`
	PersistCandidates    *bool    `json:"persist_candidates,omitempty"`
	RespectGitIgnore     *bool    `json:"respect_gitignore,omitempty"`
	OutputCandidatesPath *string  `json:"output_candidates_path,omitempty"`
	ApplyTemplates       []string `json:"apply_templates,omitempty"`
}

type ContextBudget struct {
	Unit      string `json:"unit"`
	Limit     int    `json:"limit"`
	Used      int    `json:"used"`
	Remaining int    `json:"remaining"`
}

type ContextRule struct {
	RuleID      string `json:"rule_id"`
	Key         string `json:"key"`
	Summary     string `json:"summary"`
	Enforcement string `json:"enforcement"`
	Content     string `json:"content,omitempty"`
}

type ContextSuggestion struct {
	Key     string `json:"key"`
	Summary string `json:"summary"`
}

type ContextMemory struct {
	Key     string `json:"key"`
	Summary string `json:"summary"`
}

type ContextPlan struct {
	Key       string         `json:"key"`
	Summary   string         `json:"summary"`
	Status    WorkItemStatus `json:"status"`
	FetchKeys []string       `json:"fetch_keys,omitempty"`
}

type ContextPlanTaskCounts struct {
	Total      int `json:"total"`
	Pending    int `json:"pending"`
	InProgress int `json:"in_progress"`
	Blocked    int `json:"blocked"`
	Complete   int `json:"complete"`
}

type ContextReceiptMeta struct {
	ReceiptID        string        `json:"receipt_id"`
	RetrievalVersion string        `json:"retrieval_version"`
	ProjectID        string        `json:"project_id"`
	TaskText         string        `json:"task_text"`
	Phase            Phase         `json:"phase"`
	ResolvedTags     []string      `json:"resolved_tags"`
	Budget           ContextBudget `json:"budget"`
}

type ContextReceipt struct {
	Rules       []ContextRule       `json:"rules"`
	Suggestions []ContextSuggestion `json:"suggestions"`
	Memories    []ContextMemory     `json:"memories"`
	Plans       []ContextPlan       `json:"plans"`
	Meta        ContextReceiptMeta  `json:"_meta"`
}

type GetContextDiagnostics struct {
	InitialPointerCount int    `json:"initial_pointer_count,omitempty"`
	FallbackUsed        bool   `json:"fallback_used,omitempty"`
	FallbackMode        string `json:"fallback_mode,omitempty"`
}

type GetContextResult struct {
	Status      string                 `json:"status"`
	Receipt     *ContextReceipt        `json:"receipt,omitempty"`
	Diagnostics *GetContextDiagnostics `json:"diagnostics,omitempty"`
}

type FetchItem struct {
	Key     string `json:"key"`
	Type    string `json:"type"`
	Summary string `json:"summary"`
	Content string `json:"content,omitempty"`
	Status  string `json:"status,omitempty"`
	Version string `json:"version,omitempty"`
}

type FetchVersionMismatch struct {
	Key      string `json:"key"`
	Expected string `json:"expected"`
	Actual   string `json:"actual"`
}

type FetchResult struct {
	Items             []FetchItem            `json:"items"`
	NotFound          []string               `json:"not_found,omitempty"`
	VersionMismatches []FetchVersionMismatch `json:"version_mismatches,omitempty"`
}

type ProposeMemoryValidation struct {
	HardPassed bool     `json:"hard_passed"`
	SoftPassed bool     `json:"soft_passed"`
	Errors     []string `json:"errors"`
	Warnings   []string `json:"warnings"`
}

type ProposeMemoryResult struct {
	CandidateID      int                     `json:"candidate_id"`
	Status           string                  `json:"status"`
	PromotedMemoryID int                     `json:"promoted_memory_id,omitempty"`
	Validation       ProposeMemoryValidation `json:"validation"`
}

type CompletionViolation struct {
	Path   string `json:"path"`
	Reason string `json:"reason"`
}

type ReportCompletionResult struct {
	Accepted               bool                  `json:"accepted"`
	Violations             []CompletionViolation `json:"violations"`
	DefinitionOfDoneIssues []string              `json:"definition_of_done_issues,omitempty"`
	RunID                  int                   `json:"run_id,omitempty"`
}

type WorkResult struct {
	PlanKey    string `json:"plan_key"`
	PlanStatus string `json:"plan_status"`
	Updated    int    `json:"updated"`
	TaskCount  int    `json:"task_count,omitempty"`
}

type HistoryItem struct {
	Key           string                 `json:"key"`
	Entity        HistoryEntity          `json:"entity"`
	Summary       string                 `json:"summary"`
	Status        string                 `json:"status,omitempty"`
	Scope         HistoryScope           `json:"scope,omitempty"`
	PlanKey       string                 `json:"plan_key,omitempty"`
	ReceiptID     string                 `json:"receipt_id,omitempty"`
	RunID         int64                  `json:"run_id,omitempty"`
	RequestID     string                 `json:"request_id,omitempty"`
	Phase         Phase                  `json:"phase,omitempty"`
	Kind          string                 `json:"kind,omitempty"`
	ParentPlanKey string                 `json:"parent_plan_key,omitempty"`
	TaskCounts    *ContextPlanTaskCounts `json:"task_counts,omitempty"`
	FetchKeys     []string               `json:"fetch_keys,omitempty"`
	UpdatedAt     string                 `json:"updated_at"`
}

type HistorySearchResult struct {
	Entity HistoryEntity `json:"entity"`
	Scope  HistoryScope  `json:"scope,omitempty"`
	Query  string        `json:"query,omitempty"`
	Limit  int           `json:"limit"`
	Count  int           `json:"count"`
	Items  []HistoryItem `json:"items"`
}

type SyncResult struct {
	Updated            int      `json:"updated"`
	MarkedStale        int      `json:"marked_stale"`
	NewCandidates      int      `json:"new_candidates"`
	IndexedStubs       int      `json:"indexed_stubs"`
	DeletedMarkedStale int      `json:"deleted_marked_stale"`
	ProcessedPaths     []string `json:"processed_paths,omitempty"`
}

type HealthSummary struct {
	OK            bool `json:"ok"`
	TotalFindings int  `json:"total_findings"`
}

type HealthCheckItem struct {
	Name     string   `json:"name"`
	Severity string   `json:"severity"`
	Count    int      `json:"count"`
	Samples  []string `json:"samples,omitempty"`
}

type HealthCheckResult struct {
	Summary HealthSummary     `json:"summary"`
	Checks  []HealthCheckItem `json:"checks"`
}

type HealthFixAction struct {
	Fixer HealthFixer `json:"fixer"`
	Count int         `json:"count"`
	Notes []string    `json:"notes,omitempty"`
}

type HealthFixResult struct {
	DryRun         bool              `json:"dry_run"`
	PlannedActions []HealthFixAction `json:"planned_actions"`
	AppliedActions []HealthFixAction `json:"applied_actions"`
	Summary        string            `json:"summary"`
}

type StatusSummary struct {
	Ready        bool `json:"ready"`
	MissingCount int  `json:"missing_count"`
}

type StatusProject struct {
	ProjectID              string `json:"project_id"`
	ProjectRoot            string `json:"project_root"`
	DetectedRepoRoot       string `json:"detected_repo_root,omitempty"`
	Backend                string `json:"backend"`
	PostgresConfigured     bool   `json:"postgres_configured,omitempty"`
	SQLitePath             string `json:"sqlite_path,omitempty"`
	UsesImplicitSQLitePath bool   `json:"uses_implicit_sqlite_path,omitempty"`
	Unbounded              bool   `json:"unbounded"`
}

type StatusSource struct {
	Kind         string   `json:"kind"`
	SourcePath   string   `json:"source_path"`
	AbsolutePath string   `json:"absolute_path,omitempty"`
	Exists       bool     `json:"exists"`
	Loaded       bool     `json:"loaded"`
	ItemCount    int      `json:"item_count,omitempty"`
	Notes        []string `json:"notes,omitempty"`
}

type StatusIntegration struct {
	ID              string   `json:"id"`
	Summary         string   `json:"summary,omitempty"`
	Installed       bool     `json:"installed"`
	PresentTargets  int      `json:"present_targets"`
	ExpectedTargets int      `json:"expected_targets"`
	MissingTargets  []string `json:"missing_targets,omitempty"`
}

type StatusRetrievalSelection struct {
	Key     string   `json:"key"`
	Kind    string   `json:"kind"`
	IsRule  bool     `json:"is_rule,omitempty"`
	Reasons []string `json:"reasons,omitempty"`
}

type StatusRetrieval struct {
	TaskText         string                     `json:"task_text,omitempty"`
	Phase            Phase                      `json:"phase,omitempty"`
	Status           string                     `json:"status"`
	Diagnostics      *GetContextDiagnostics     `json:"diagnostics,omitempty"`
	ResolvedTags     []string                   `json:"resolved_tags,omitempty"`
	RuleCount        int                        `json:"rule_count,omitempty"`
	SuggestionCount  int                        `json:"suggestion_count,omitempty"`
	MemoryCount      int                        `json:"memory_count,omitempty"`
	PlanCount        int                        `json:"plan_count,omitempty"`
	SelectedPointers []StatusRetrievalSelection `json:"selected_pointers,omitempty"`
	Error            string                     `json:"error,omitempty"`
}

type StatusMissingItem struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type StatusResult struct {
	Summary      StatusSummary       `json:"summary"`
	Project      StatusProject       `json:"project"`
	Sources      []StatusSource      `json:"sources"`
	Integrations []StatusIntegration `json:"integrations"`
	Retrieval    *StatusRetrieval    `json:"retrieval,omitempty"`
	Missing      []StatusMissingItem `json:"missing,omitempty"`
}

type CoverageSummary struct {
	TotalFiles      int     `json:"total_files"`
	IndexedFiles    int     `json:"indexed_files"`
	UnindexedFiles  int     `json:"unindexed_files"`
	StaleFiles      int     `json:"stale_files"`
	CoveragePercent float64 `json:"coverage_percent"`
}

type CoverageResult struct {
	Summary          CoverageSummary `json:"summary"`
	UnindexedPaths   []string        `json:"unindexed_paths,omitempty"`
	StalePaths       []string        `json:"stale_paths,omitempty"`
	ZeroCoverageDirs []string        `json:"zero_coverage_dirs,omitempty"`
}

type EvalAggregate struct {
	Precision float64 `json:"precision"`
	Recall    float64 `json:"recall"`
	F1        float64 `json:"f1"`
}

type EvalCaseResult struct {
	Index     int     `json:"index"`
	Precision float64 `json:"precision"`
	Recall    float64 `json:"recall"`
	F1        float64 `json:"f1"`
	Notes     string  `json:"notes,omitempty"`
}

type EvalResult struct {
	TotalCases    int              `json:"total_cases"`
	Aggregate     EvalAggregate    `json:"aggregate"`
	MinimumRecall float64          `json:"minimum_recall"`
	Pass          bool             `json:"pass"`
	Cases         []EvalCaseResult `json:"cases,omitempty"`
}

type VerifyStatus string

const (
	VerifyStatusDryRun          VerifyStatus = "dry_run"
	VerifyStatusNoTestsSelected VerifyStatus = "no_tests_selected"
	VerifyStatusPassed          VerifyStatus = "passed"
	VerifyStatusFailed          VerifyStatus = "failed"
)

type VerifyTestStatus string

const (
	VerifyTestStatusPassed   VerifyTestStatus = "passed"
	VerifyTestStatusFailed   VerifyTestStatus = "failed"
	VerifyTestStatusTimedOut VerifyTestStatus = "timed_out"
	VerifyTestStatusErrored  VerifyTestStatus = "errored"
	VerifyTestStatusSkipped  VerifyTestStatus = "skipped"
)

type VerifySelection struct {
	TestID           string   `json:"test_id"`
	Summary          string   `json:"summary"`
	SelectionReasons []string `json:"selection_reasons"`
}

type VerifyTestResult struct {
	TestID         string           `json:"test_id"`
	Status         VerifyTestStatus `json:"status"`
	DefinitionHash string           `json:"definition_hash"`
	ExitCode       *int             `json:"exit_code,omitempty"`
	DurationMS     int              `json:"duration_ms,omitempty"`
	StdoutExcerpt  string           `json:"stdout_excerpt,omitempty"`
	StderrExcerpt  string           `json:"stderr_excerpt,omitempty"`
}

type VerifyResult struct {
	Status          VerifyStatus       `json:"status"`
	BatchRunID      string             `json:"batch_run_id,omitempty"`
	SelectedTestIDs []string           `json:"selected_test_ids"`
	Selected        []VerifySelection  `json:"selected"`
	Passed          bool               `json:"passed"`
	Results         []VerifyTestResult `json:"results,omitempty"`
}

type BootstrapResult struct {
	CandidateCount       int                       `json:"candidate_count"`
	IndexedStubs         int                       `json:"indexed_stubs"`
	CandidatesPersisted  bool                      `json:"candidates_persisted"`
	OutputCandidatesPath string                    `json:"output_candidates_path,omitempty"`
	Warnings             []string                  `json:"warnings,omitempty"`
	TemplateResults      []BootstrapTemplateResult `json:"template_results,omitempty"`
}

type BootstrapTemplateConflict struct {
	Path   string `json:"path"`
	Reason string `json:"reason"`
}

type BootstrapTemplateResult struct {
	TemplateID       string                      `json:"template_id"`
	Created          []string                    `json:"created,omitempty"`
	Updated          []string                    `json:"updated,omitempty"`
	Unchanged        []string                    `json:"unchanged,omitempty"`
	SkippedConflicts []BootstrapTemplateConflict `json:"skipped_conflicts,omitempty"`
}
