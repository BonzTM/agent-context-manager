package core

import (
	"context"
	"errors"
	"time"
)

var (
	ErrReceiptScopeNotFound  = errors.New("receipt scope not found")
	ErrFetchLookupNotFound   = errors.New("fetch lookup not found")
	ErrPointerLookupNotFound = errors.New("pointer lookup not found")
	ErrMemoryLookupNotFound  = errors.New("memory lookup not found")
	ErrWorkPlanNotFound      = errors.New("work plan not found")
)

const (
	WorkItemStatusPending    = "pending"
	WorkItemStatusInProgress = "in_progress"
	WorkItemStatusBlocked    = "blocked"
	WorkItemStatusComplete   = "complete"
	WorkItemStatusSuperseded = "superseded"
	WorkItemStatusCompleted  = "completed"

	PlanStatusPending    = "pending"
	PlanStatusInProgress = "in_progress"
	PlanStatusBlocked    = "blocked"
	PlanStatusComplete   = "complete"
	PlanStatusSuperseded = "superseded"
	PlanStatusCompleted  = "completed"
)

type StaleFilter struct {
	AllowStale  bool
	StaleBefore *time.Time
}

type CandidatePointerQuery struct {
	ProjectID   string
	Limit       int
	Unbounded   bool
	StaleFilter StaleFilter
}

type CandidatePointer struct {
	Key         string
	Path        string
	Anchor      string
	Kind        string
	Label       string
	Description string
	Tags        []string
	IsRule      bool
	IsStale     bool
	UpdatedAt   time.Time
}

type ActiveMemoryQuery struct {
	ProjectID   string
	PointerKeys []string
	Tags        []string
	Limit       int
	Unbounded   bool
}

type ActiveMemory struct {
	ID                 int64
	Category           string
	Subject            string
	Content            string
	Confidence         int
	Tags               []string
	RelatedPointerKeys []string
	UpdatedAt          time.Time
}

type PointerInventory struct {
	Path    string
	IsStale bool
}

type PointerStub struct {
	PointerKey  string
	Path        string
	Kind        string
	Label       string
	Description string
	Tags        []string
}

type RunReceiptSummary struct {
	ProjectID              string
	RequestID              string
	ReceiptID              string
	TaskText               string
	Phase                  string
	Status                 string
	ResolvedTags           []string
	PointerKeys            []string
	MemoryIDs              []int64
	FilesChanged           []string
	DefinitionOfDoneIssues []string
	Outcome                string
}

type RunReceiptIDs struct {
	RunID     int64
	ReceiptID string
}

type ReceiptScopeQuery struct {
	ProjectID string
	ReceiptID string
}

type ReceiptScope struct {
	ProjectID         string
	ReceiptID         string
	TaskText          string
	Phase             string
	ResolvedTags      []string
	PointerKeys       []string
	MemoryIDs         []int64
	InitialScopePaths []string
	BaselineCaptured  bool
	BaselinePaths     []SyncPath
}

type FetchLookupQuery struct {
	ProjectID string
	ReceiptID string
}

type PointerLookupQuery struct {
	ProjectID  string
	PointerKey string
}

type MemoryLookupQuery struct {
	ProjectID string
	MemoryID  int64
}

type WorkItem struct {
	ItemKey            string
	Summary            string
	Status             string
	ParentTaskKey      string
	DependsOn          []string
	AcceptanceCriteria []string
	References         []string
	ExternalRefs       []string
	BlockedReason      string
	Outcome            string
	Evidence           []string
	UpdatedAt          time.Time
}

type FetchLookup struct {
	ProjectID  string
	ReceiptID  string
	RunID      int64
	RunStatus  string
	PlanStatus string
	WorkItems  []WorkItem
	UpdatedAt  time.Time
}

type MemoryValidation struct {
	HardPassed bool
	SoftPassed bool
	Errors     []string
	Warnings   []string
}

type MemoryPersistence struct {
	ProjectID           string
	ReceiptID           string
	Category            string
	Subject             string
	Content             string
	Confidence          int
	Tags                []string
	RelatedPointerKeys  []string
	EvidencePointerKeys []string
	DedupeKey           string
	Validation          MemoryValidation
	AutoPromote         bool
	Promotable          bool
}

type MemoryPersistenceResult struct {
	CandidateID      int64
	Status           string
	PromotedMemoryID int64
}

type SyncPath struct {
	Path        string
	ContentHash string
	Deleted     bool
}

type SyncApplyInput struct {
	ProjectID           string
	Mode                string
	InsertNewCandidates bool
	Paths               []SyncPath
}

type SyncApplyResult struct {
	Updated            int
	MarkedStale        int
	NewCandidates      int
	DeletedMarkedStale int
}

type RulePointer struct {
	PointerKey  string
	SourcePath  string
	RuleID      string
	Summary     string
	Content     string
	Enforcement string
	Tags        []string
}

type RulePointerSyncInput struct {
	ProjectID  string
	SourcePath string
	Pointers   []RulePointer
}

type RulePointerSyncResult struct {
	Upserted    int
	MarkedStale int
}

type WorkItemsUpsertInput struct {
	ProjectID string
	ReceiptID string
	Items     []WorkItem
}

type WorkPlanMode string

const (
	WorkPlanModeMerge   WorkPlanMode = "merge"
	WorkPlanModeReplace WorkPlanMode = "replace"
)

type WorkPlanStages struct {
	SpecOutline        string
	RefinedSpec        string
	ImplementationPlan string
}

type WorkPlan struct {
	ProjectID       string
	PlanKey         string
	ReceiptID       string
	Title           string
	Objective       string
	Kind            string
	ParentPlanKey   string
	Status          string
	Stages          WorkPlanStages
	InScope         []string
	OutOfScope      []string
	DiscoveredPaths []string
	Constraints     []string
	References      []string
	ExternalRefs    []string
	Tasks           []WorkItem
	UpdatedAt       time.Time
}

type WorkPlanUpsertInput struct {
	ProjectID       string
	PlanKey         string
	ReceiptID       string
	Mode            WorkPlanMode
	Title           string
	Objective       string
	Kind            string
	ParentPlanKey   string
	Status          string
	Stages          WorkPlanStages
	InScope         []string
	OutOfScope      []string
	DiscoveredPaths []string
	Constraints     []string
	References      []string
	ExternalRefs    []string
	Tasks           []WorkItem
}

type WorkPlanUpsertResult struct {
	Plan    WorkPlan
	Updated int
}

type WorkPlanLookupQuery struct {
	ProjectID string
	PlanKey   string
	ReceiptID string
}

type WorkPlanListQuery struct {
	ProjectID string
	Scope     string
	Query     string
	Kind      string
	Limit     int
	Unbounded bool
}

type WorkPlanSummary struct {
	ReceiptID           string
	Title               string
	Objective           string
	PlanKey             string
	Summary             string
	Status              string
	Kind                string
	ParentPlanKey       string
	ActiveTaskKeys      []string
	TaskCountTotal      int
	TaskCountPending    int
	TaskCountInProgress int
	TaskCountBlocked    int
	TaskCountComplete   int
	UpdatedAt           time.Time
}

type ReceiptHistoryListQuery struct {
	ProjectID string
	Query     string
	Limit     int
	Unbounded bool
}

type ReceiptHistorySummary struct {
	ReceiptID       string
	TaskText        string
	Phase           string
	LatestRequestID string
	LatestStatus    string
	UpdatedAt       time.Time
}

type MemoryHistoryListQuery struct {
	ProjectID string
	Query     string
	Limit     int
	Unbounded bool
}

type MemoryHistorySummary struct {
	MemoryID   int64
	Category   string
	Subject    string
	Content    string
	Confidence int
	UpdatedAt  time.Time
}

type RunHistoryListQuery struct {
	ProjectID string
	Query     string
	Limit     int
	Unbounded bool
}

type RunHistoryLookupQuery struct {
	ProjectID string
	RunID     int64
}

type RunHistorySummary struct {
	RunID        int64
	ReceiptID    string
	RequestID    string
	TaskText     string
	Phase        string
	Status       string
	FilesChanged []string
	Outcome      string
	UpdatedAt    time.Time
}

type ReviewAttempt struct {
	AttemptID          int64
	ProjectID          string
	ReceiptID          string
	PlanKey            string
	ReviewKey          string
	Summary            string
	Fingerprint        string
	Status             string
	Passed             bool
	Outcome            string
	WorkflowSourcePath string
	CommandArgv        []string
	CommandCWD         string
	TimeoutSec         int
	ExitCode           *int
	TimedOut           bool
	StdoutExcerpt      string
	StderrExcerpt      string
	CreatedAt          time.Time
}

type ReviewAttemptListQuery struct {
	ProjectID string
	ReceiptID string
	ReviewKey string
}

type VerificationBatch struct {
	BatchRunID      string
	ProjectID       string
	ReceiptID       string
	PlanKey         string
	Phase           string
	TestsSourcePath string
	Status          string
	Passed          bool
	SelectedTestIDs []string
	Results         []VerificationTestRun
	CreatedAt       time.Time
}

type VerificationTestRun struct {
	BatchRunID       string
	ProjectID        string
	TestID           string
	DefinitionHash   string
	Summary          string
	CommandArgv      []string
	CommandCWD       string
	TimeoutSec       int
	ExpectedExitCode int
	SelectionReasons []string
	Status           string
	ExitCode         *int
	DurationMS       int
	StdoutExcerpt    string
	StderrExcerpt    string
	StartedAt        time.Time
	FinishedAt       time.Time
}

type Repository interface {
	FetchCandidatePointers(context.Context, CandidatePointerQuery) ([]CandidatePointer, error)
	FetchActiveMemories(context.Context, ActiveMemoryQuery) ([]ActiveMemory, error)
	ListPointerInventory(context.Context, string) ([]PointerInventory, error)
	UpsertPointerStubs(context.Context, string, []PointerStub) (int, error)
	UpsertReceiptScope(context.Context, ReceiptScope) error
	FetchReceiptScope(context.Context, ReceiptScopeQuery) (ReceiptScope, error)
	LookupFetchState(context.Context, FetchLookupQuery) (FetchLookup, error)
	LookupPointerByKey(context.Context, PointerLookupQuery) (CandidatePointer, error)
	LookupMemoryByID(context.Context, MemoryLookupQuery) (ActiveMemory, error)
	PersistMemory(context.Context, MemoryPersistence) (MemoryPersistenceResult, error)
	SaveRunReceiptSummary(context.Context, RunReceiptSummary) (RunReceiptIDs, error)
	SaveReviewAttempt(context.Context, ReviewAttempt) (int64, error)
	ListReviewAttempts(context.Context, ReviewAttemptListQuery) ([]ReviewAttempt, error)
	UpsertWorkItems(context.Context, WorkItemsUpsertInput) (int, error)
	ListWorkItems(context.Context, FetchLookupQuery) ([]WorkItem, error)
	ApplySync(context.Context, SyncApplyInput) (SyncApplyResult, error)
	SyncRulePointers(context.Context, RulePointerSyncInput) (RulePointerSyncResult, error)
}

// WorkPlanRepository is an optional extension for richer plan/task persistence.
// Implementations may be asserted from Repository by services that support plan-aware workflows.
type WorkPlanRepository interface {
	UpsertWorkPlan(context.Context, WorkPlanUpsertInput) (WorkPlanUpsertResult, error)
	LookupWorkPlan(context.Context, WorkPlanLookupQuery) (WorkPlan, error)
	ListWorkPlans(context.Context, WorkPlanListQuery) ([]WorkPlanSummary, error)
}

type HistoryRepository interface {
	ListMemoryHistory(context.Context, MemoryHistoryListQuery) ([]MemoryHistorySummary, error)
	ListReceiptHistory(context.Context, ReceiptHistoryListQuery) ([]ReceiptHistorySummary, error)
	ListRunHistory(context.Context, RunHistoryListQuery) ([]RunHistorySummary, error)
	LookupRunHistory(context.Context, RunHistoryLookupQuery) (RunHistorySummary, error)
}

// VerificationRepository is an optional extension for durable executable verification storage.
type VerificationRepository interface {
	SaveVerificationBatch(context.Context, VerificationBatch) error
}
