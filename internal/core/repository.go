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
)

const (
	WorkItemStatusPending    = "pending"
	WorkItemStatusInProgress = "in_progress"
	WorkItemStatusBlocked    = "blocked"
	WorkItemStatusComplete   = "complete"
	WorkItemStatusCompleted  = "completed"

	PlanStatusPending    = "pending"
	PlanStatusInProgress = "in_progress"
	PlanStatusBlocked    = "blocked"
	PlanStatusComplete   = "complete"
	PlanStatusCompleted  = "completed"
)

type StaleFilter struct {
	AllowStale  bool
	StaleBefore *time.Time
}

type CandidatePointerQuery struct {
	ProjectID   string
	TaskText    string
	Phase       string
	Tags        []string
	Limit       int
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
	Rank        float64
	UpdatedAt   time.Time
}

type RelatedHopPointersQuery struct {
	ProjectID   string
	PointerKeys []string
	MaxHops     int
	Limit       int
	StaleFilter StaleFilter
}

type HopPointer struct {
	SourceKey string
	HopCount  int
	Pointer   CandidatePointer
}

type ActiveMemoryQuery struct {
	ProjectID   string
	PointerKeys []string
	Tags        []string
	Limit       int
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
	ProjectID    string
	ReceiptID    string
	TaskText     string
	Phase        string
	ResolvedTags []string
	PointerKeys  []string
	MemoryIDs    []int64
	PointerPaths []string
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
	ItemKey   string
	Status    string
	UpdatedAt time.Time
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

type ProposeMemoryValidation struct {
	HardPassed bool
	SoftPassed bool
	Errors     []string
	Warnings   []string
}

type ProposeMemoryPersistence struct {
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
	Validation          ProposeMemoryValidation
	AutoPromote         bool
	Promotable          bool
}

type ProposeMemoryPersistenceResult struct {
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

type Repository interface {
	FetchCandidatePointers(context.Context, CandidatePointerQuery) ([]CandidatePointer, error)
	FetchRelatedHopPointers(context.Context, RelatedHopPointersQuery) ([]HopPointer, error)
	FetchActiveMemories(context.Context, ActiveMemoryQuery) ([]ActiveMemory, error)
	ListPointerInventory(context.Context, string) ([]PointerInventory, error)
	UpsertPointerStubs(context.Context, string, []PointerStub) (int, error)
	FetchReceiptScope(context.Context, ReceiptScopeQuery) (ReceiptScope, error)
	LookupFetchState(context.Context, FetchLookupQuery) (FetchLookup, error)
	LookupPointerByKey(context.Context, PointerLookupQuery) (CandidatePointer, error)
	LookupMemoryByID(context.Context, MemoryLookupQuery) (ActiveMemory, error)
	PersistProposedMemory(context.Context, ProposeMemoryPersistence) (ProposeMemoryPersistenceResult, error)
	SaveRunReceiptSummary(context.Context, RunReceiptSummary) (RunReceiptIDs, error)
	UpsertWorkItems(context.Context, WorkItemsUpsertInput) (int, error)
	ListWorkItems(context.Context, FetchLookupQuery) ([]WorkItem, error)
	ApplySync(context.Context, SyncApplyInput) (SyncApplyResult, error)
	SyncRulePointers(context.Context, RulePointerSyncInput) (RulePointerSyncResult, error)
}
