package postgres

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"path"
	"sort"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/joshd/agents-context/internal/core"
)

type Repository struct {
	pool *pgxpool.Pool
}

var _ core.Repository = (*Repository)(nil)

func New(ctx context.Context, cfg Config) (*Repository, error) {
	poolCfg, err := cfg.PoolConfig()
	if err != nil {
		return nil, err
	}

	pool, err := pgxpool.NewWithConfig(ctx, poolCfg)
	if err != nil {
		return nil, fmt.Errorf("create postgres pool: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping postgres: %w", err)
	}

	repo, err := NewWithPool(pool)
	if err != nil {
		pool.Close()
		return nil, err
	}
	if err := repo.Migrate(ctx); err != nil {
		repo.Close()
		return nil, fmt.Errorf("apply postgres migrations: %w", err)
	}

	return repo, nil
}

func NewWithPool(pool *pgxpool.Pool) (*Repository, error) {
	if pool == nil {
		return nil, fmt.Errorf("postgres pool is required")
	}
	return &Repository{pool: pool}, nil
}

func (r *Repository) Close() {
	if r == nil || r.pool == nil {
		return
	}
	r.pool.Close()
}

func (r *Repository) Migrate(ctx context.Context) error {
	if r == nil || r.pool == nil {
		return fmt.Errorf("postgres pool is required")
	}
	return ApplyMigrations(ctx, r.pool)
}

func (r *Repository) FetchCandidatePointers(ctx context.Context, input core.CandidatePointerQuery) ([]core.CandidatePointer, error) {
	if r == nil || r.pool == nil {
		return nil, fmt.Errorf("postgres pool is required")
	}

	input.StaleFilter.StaleBefore = normalizeStaleBefore(input.StaleFilter.StaleBefore)

	query, args, err := buildCandidatePointersQuery(input)
	if err != nil {
		return nil, err
	}

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query candidate pointers: %w", err)
	}
	defer rows.Close()

	results := make([]core.CandidatePointer, 0)
	for rows.Next() {
		var row struct {
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
			Rank        float32
		}
		if err := rows.Scan(
			&row.Key,
			&row.Path,
			&row.Anchor,
			&row.Kind,
			&row.Label,
			&row.Description,
			&row.Tags,
			&row.IsRule,
			&row.IsStale,
			&row.UpdatedAt,
			&row.Rank,
		); err != nil {
			return nil, fmt.Errorf("scan candidate pointer: %w", err)
		}

		results = append(results, core.CandidatePointer{
			Key:         row.Key,
			Path:        row.Path,
			Anchor:      row.Anchor,
			Kind:        row.Kind,
			Label:       row.Label,
			Description: row.Description,
			Tags:        row.Tags,
			IsRule:      row.IsRule,
			IsStale:     row.IsStale,
			Rank:        float64(row.Rank),
			UpdatedAt:   row.UpdatedAt,
		})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate candidate pointers: %w", err)
	}

	return results, nil
}

func (r *Repository) FetchRelatedHopPointers(ctx context.Context, input core.RelatedHopPointersQuery) ([]core.HopPointer, error) {
	if r == nil || r.pool == nil {
		return nil, fmt.Errorf("postgres pool is required")
	}

	input.StaleFilter.StaleBefore = normalizeStaleBefore(input.StaleFilter.StaleBefore)

	query, args, err := buildRelatedHopPointersQuery(input)
	if err != nil {
		return nil, err
	}

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query related hop pointers: %w", err)
	}
	defer rows.Close()

	results := make([]core.HopPointer, 0)
	for rows.Next() {
		var row struct {
			SourceKey   string
			HopCount    int
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
		if err := rows.Scan(
			&row.SourceKey,
			&row.HopCount,
			&row.Key,
			&row.Path,
			&row.Anchor,
			&row.Kind,
			&row.Label,
			&row.Description,
			&row.Tags,
			&row.IsRule,
			&row.IsStale,
			&row.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan related hop pointer: %w", err)
		}

		results = append(results, core.HopPointer{
			SourceKey: row.SourceKey,
			HopCount:  row.HopCount,
			Pointer: core.CandidatePointer{
				Key:         row.Key,
				Path:        row.Path,
				Anchor:      row.Anchor,
				Kind:        row.Kind,
				Label:       row.Label,
				Description: row.Description,
				Tags:        row.Tags,
				IsRule:      row.IsRule,
				IsStale:     row.IsStale,
				Rank:        0,
				UpdatedAt:   row.UpdatedAt,
			},
		})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate related hop pointers: %w", err)
	}

	return results, nil
}

func (r *Repository) FetchActiveMemories(ctx context.Context, input core.ActiveMemoryQuery) ([]core.ActiveMemory, error) {
	if r == nil || r.pool == nil {
		return nil, fmt.Errorf("postgres pool is required")
	}

	query, args, err := buildActiveMemoriesQuery(input)
	if err != nil {
		return nil, err
	}

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query active memories: %w", err)
	}
	defer rows.Close()

	results := make([]core.ActiveMemory, 0)
	for rows.Next() {
		var row struct {
			ID                 int64
			Category           string
			Subject            string
			Content            string
			Confidence         int
			Tags               []string
			RelatedPointerKeys []string
			UpdatedAt          time.Time
		}
		if err := rows.Scan(
			&row.ID,
			&row.Category,
			&row.Subject,
			&row.Content,
			&row.Confidence,
			&row.Tags,
			&row.RelatedPointerKeys,
			&row.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan active memory: %w", err)
		}

		results = append(results, core.ActiveMemory{
			ID:                 row.ID,
			Category:           row.Category,
			Subject:            row.Subject,
			Content:            row.Content,
			Confidence:         row.Confidence,
			Tags:               row.Tags,
			RelatedPointerKeys: row.RelatedPointerKeys,
			UpdatedAt:          row.UpdatedAt,
		})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate active memories: %w", err)
	}

	return results, nil
}

func (r *Repository) ListPointerInventory(ctx context.Context, projectID string) ([]core.PointerInventory, error) {
	if r == nil || r.pool == nil {
		return nil, fmt.Errorf("postgres pool is required")
	}

	projectID = strings.TrimSpace(projectID)
	if projectID == "" {
		return nil, fmt.Errorf("project_id is required")
	}

	rows, err := r.pool.Query(ctx, `
SELECT
	path,
	BOOL_OR(is_stale) AS is_stale
FROM ctx_pointers
WHERE project_id = $1
GROUP BY path
ORDER BY path ASC
`, projectID)
	if err != nil {
		return nil, fmt.Errorf("query pointer inventory: %w", err)
	}
	defer rows.Close()

	results := make([]core.PointerInventory, 0)
	for rows.Next() {
		var row core.PointerInventory
		if err := rows.Scan(&row.Path, &row.IsStale); err != nil {
			return nil, fmt.Errorf("scan pointer inventory: %w", err)
		}
		row.Path = normalizeSyncPath(row.Path)
		if row.Path == "" {
			continue
		}
		results = append(results, row)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate pointer inventory: %w", err)
	}

	return results, nil
}

func (r *Repository) UpsertPointerStubs(ctx context.Context, projectID string, stubs []core.PointerStub) (int, error) {
	if r == nil || r.pool == nil {
		return 0, fmt.Errorf("postgres pool is required")
	}

	normalized, err := normalizePointerStubs(projectID, stubs)
	if err != nil {
		return 0, err
	}
	if len(normalized) == 0 {
		return 0, nil
	}

	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return 0, fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	updated := 0
	for _, stub := range normalized {
		isRule := strings.EqualFold(stub.Kind, "rule")
		tag, execErr := tx.Exec(ctx, `
INSERT INTO ctx_pointers (
	project_id,
	pointer_key,
	path,
	anchor,
	kind,
	label,
	description,
	tags,
	is_rule,
	is_stale,
	stale_at,
	content_hash,
	updated_at
)
VALUES (
	$1,
	$2,
	$3,
	'',
	$4,
	$5,
	$6,
	$7,
	$8,
	FALSE,
	NULL,
	NULL,
	NOW()
)
ON CONFLICT (project_id, pointer_key) DO UPDATE SET
	path = EXCLUDED.path,
	anchor = EXCLUDED.anchor,
	is_stale = FALSE,
	stale_at = NULL,
	updated_at = NOW()
`,
			strings.TrimSpace(projectID),
			stub.PointerKey,
			stub.Path,
			stub.Kind,
			stub.Label,
			stub.Description,
			nonNilStringList(stub.Tags),
			isRule,
		)
		if execErr != nil {
			return 0, fmt.Errorf("upsert pointer stubs: %w", execErr)
		}
		updated += int(tag.RowsAffected())
	}

	if err := tx.Commit(ctx); err != nil {
		return 0, fmt.Errorf("commit tx: %w", err)
	}
	return updated, nil
}

func (r *Repository) FetchReceiptScope(ctx context.Context, input core.ReceiptScopeQuery) (core.ReceiptScope, error) {
	if r == nil || r.pool == nil {
		return core.ReceiptScope{}, fmt.Errorf("postgres pool is required")
	}

	query, args, err := buildFetchReceiptScopeQuery(input)
	if err != nil {
		return core.ReceiptScope{}, err
	}

	var row struct {
		ReceiptID    string
		TaskText     string
		Phase        string
		ResolvedTags []string
		PointerKeys  []string
		MemoryIDs    []int64
		PointerPaths []string
	}
	if err := r.pool.QueryRow(ctx, query, args...).Scan(
		&row.ReceiptID,
		&row.TaskText,
		&row.Phase,
		&row.ResolvedTags,
		&row.PointerKeys,
		&row.MemoryIDs,
		&row.PointerPaths,
	); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return core.ReceiptScope{}, core.ErrReceiptScopeNotFound
		}
		return core.ReceiptScope{}, fmt.Errorf("query receipt scope: %w", err)
	}

	return core.ReceiptScope{
		ProjectID:    strings.TrimSpace(input.ProjectID),
		ReceiptID:    row.ReceiptID,
		TaskText:     strings.TrimSpace(row.TaskText),
		Phase:        strings.TrimSpace(row.Phase),
		ResolvedTags: normalizeStringList(row.ResolvedTags),
		PointerKeys:  normalizeStringList(row.PointerKeys),
		MemoryIDs:    normalizeInt64List(row.MemoryIDs),
		PointerPaths: normalizeStringList(row.PointerPaths),
	}, nil
}

func (r *Repository) LookupFetchState(ctx context.Context, input core.FetchLookupQuery) (core.FetchLookup, error) {
	if r == nil || r.pool == nil {
		return core.FetchLookup{}, fmt.Errorf("postgres pool is required")
	}

	query, args, err := buildLookupFetchStateQuery(input)
	if err != nil {
		return core.FetchLookup{}, err
	}

	var row struct {
		ReceiptID string
		RunID     int64
		RunStatus string
		UpdatedAt time.Time
	}
	if err := r.pool.QueryRow(ctx, query, args...).Scan(
		&row.ReceiptID,
		&row.RunID,
		&row.RunStatus,
		&row.UpdatedAt,
	); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return core.FetchLookup{}, core.ErrFetchLookupNotFound
		}
		return core.FetchLookup{}, fmt.Errorf("query fetch lookup: %w", err)
	}

	workItems, err := r.ListWorkItems(ctx, input)
	if err != nil {
		return core.FetchLookup{}, err
	}

	return core.FetchLookup{
		ProjectID:  strings.TrimSpace(input.ProjectID),
		ReceiptID:  row.ReceiptID,
		RunID:      row.RunID,
		RunStatus:  strings.TrimSpace(row.RunStatus),
		PlanStatus: derivePlanStatus(workItems),
		WorkItems:  workItems,
		UpdatedAt:  row.UpdatedAt.UTC(),
	}, nil
}

func (r *Repository) LookupPointerByKey(ctx context.Context, input core.PointerLookupQuery) (core.CandidatePointer, error) {
	if r == nil || r.pool == nil {
		return core.CandidatePointer{}, fmt.Errorf("postgres pool is required")
	}

	query, args, err := buildLookupPointerByKeyQuery(input)
	if err != nil {
		return core.CandidatePointer{}, err
	}

	var row struct {
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
	if err := r.pool.QueryRow(ctx, query, args...).Scan(
		&row.Key,
		&row.Path,
		&row.Anchor,
		&row.Kind,
		&row.Label,
		&row.Description,
		&row.Tags,
		&row.IsRule,
		&row.IsStale,
		&row.UpdatedAt,
	); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return core.CandidatePointer{}, core.ErrPointerLookupNotFound
		}
		return core.CandidatePointer{}, fmt.Errorf("query pointer lookup: %w", err)
	}

	return core.CandidatePointer{
		Key:         strings.TrimSpace(row.Key),
		Path:        normalizeSyncPath(row.Path),
		Anchor:      strings.TrimSpace(row.Anchor),
		Kind:        strings.TrimSpace(row.Kind),
		Label:       strings.TrimSpace(row.Label),
		Description: strings.TrimSpace(row.Description),
		Tags:        normalizeStringList(row.Tags),
		IsRule:      row.IsRule,
		IsStale:     row.IsStale,
		UpdatedAt:   row.UpdatedAt.UTC(),
	}, nil
}

func (r *Repository) LookupMemoryByID(ctx context.Context, input core.MemoryLookupQuery) (core.ActiveMemory, error) {
	if r == nil || r.pool == nil {
		return core.ActiveMemory{}, fmt.Errorf("postgres pool is required")
	}

	query, args, err := buildLookupMemoryByIDQuery(input)
	if err != nil {
		return core.ActiveMemory{}, err
	}

	var row struct {
		ID                 int64
		Category           string
		Subject            string
		Content            string
		Confidence         int
		Tags               []string
		RelatedPointerKeys []string
		UpdatedAt          time.Time
	}
	if err := r.pool.QueryRow(ctx, query, args...).Scan(
		&row.ID,
		&row.Category,
		&row.Subject,
		&row.Content,
		&row.Confidence,
		&row.Tags,
		&row.RelatedPointerKeys,
		&row.UpdatedAt,
	); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return core.ActiveMemory{}, core.ErrMemoryLookupNotFound
		}
		return core.ActiveMemory{}, fmt.Errorf("query memory lookup: %w", err)
	}

	return core.ActiveMemory{
		ID:                 row.ID,
		Category:           strings.TrimSpace(row.Category),
		Subject:            strings.TrimSpace(row.Subject),
		Content:            strings.TrimSpace(row.Content),
		Confidence:         row.Confidence,
		Tags:               normalizeStringList(row.Tags),
		RelatedPointerKeys: normalizeStringList(row.RelatedPointerKeys),
		UpdatedAt:          row.UpdatedAt.UTC(),
	}, nil
}

func (r *Repository) UpsertWorkItems(ctx context.Context, input core.WorkItemsUpsertInput) (int, error) {
	if r == nil || r.pool == nil {
		return 0, fmt.Errorf("postgres pool is required")
	}

	query, args, err := buildUpsertWorkItemsQuery(input)
	if err != nil {
		return 0, err
	}

	tag, err := r.pool.Exec(ctx, query, args...)
	if err != nil {
		return 0, fmt.Errorf("upsert work items: %w", err)
	}

	return int(tag.RowsAffected()), nil
}

func (r *Repository) ListWorkItems(ctx context.Context, input core.FetchLookupQuery) ([]core.WorkItem, error) {
	if r == nil || r.pool == nil {
		return nil, fmt.Errorf("postgres pool is required")
	}

	query, args, err := buildListWorkItemsQuery(input)
	if err != nil {
		return nil, err
	}

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query work items: %w", err)
	}
	defer rows.Close()

	items := make([]core.WorkItem, 0)
	for rows.Next() {
		var item core.WorkItem
		if err := rows.Scan(&item.ItemKey, &item.Status, &item.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan work item: %w", err)
		}
		item.ItemKey = normalizeSyncPath(item.ItemKey)
		item.Status = normalizeWorkItemStatus(item.Status)
		item.UpdatedAt = item.UpdatedAt.UTC()
		if item.ItemKey == "" {
			continue
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate work items: %w", err)
	}

	return items, nil
}

func (r *Repository) PersistProposedMemory(ctx context.Context, input core.ProposeMemoryPersistence) (core.ProposeMemoryPersistenceResult, error) {
	if r == nil || r.pool == nil {
		return core.ProposeMemoryPersistenceResult{}, fmt.Errorf("postgres pool is required")
	}

	normalized, err := normalizeProposeMemoryPersistence(input)
	if err != nil {
		return core.ProposeMemoryPersistenceResult{}, err
	}
	if normalized.Promotable && (!normalized.Validation.HardPassed || !normalized.Validation.SoftPassed) {
		return core.ProposeMemoryPersistenceResult{}, fmt.Errorf("promotable requires hard and soft validation pass")
	}

	initialStatus := candidateStatusPending
	if !normalized.Validation.HardPassed {
		initialStatus = candidateStatusRejected
	}

	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return core.ProposeMemoryPersistenceResult{}, fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	insertCandidateSQL, insertCandidateArgs, err := buildInsertMemoryCandidateQuery(normalized, initialStatus)
	if err != nil {
		return core.ProposeMemoryPersistenceResult{}, err
	}

	var candidateID int64
	if err := tx.QueryRow(ctx, insertCandidateSQL, insertCandidateArgs...).Scan(&candidateID); err != nil {
		return core.ProposeMemoryPersistenceResult{}, fmt.Errorf("insert memory candidate: %w", err)
	}

	out := core.ProposeMemoryPersistenceResult{
		CandidateID: candidateID,
		Status:      initialStatus,
	}

	if !normalized.Promotable {
		if err := tx.Commit(ctx); err != nil {
			return core.ProposeMemoryPersistenceResult{}, fmt.Errorf("commit tx: %w", err)
		}
		return out, nil
	}

	insertMemorySQL, insertMemoryArgs, err := buildInsertDurableMemoryQuery(normalized)
	if err != nil {
		return core.ProposeMemoryPersistenceResult{}, err
	}

	var promotedMemoryID int64
	insertErr := tx.QueryRow(ctx, insertMemorySQL, insertMemoryArgs...).Scan(&promotedMemoryID)
	if insertErr != nil && !errors.Is(insertErr, pgx.ErrNoRows) {
		return core.ProposeMemoryPersistenceResult{}, fmt.Errorf("insert durable memory: %w", insertErr)
	}

	finalStatus := candidateStatusRejected
	if insertErr == nil {
		finalStatus = candidateStatusPromoted
		out.PromotedMemoryID = promotedMemoryID
	}

	updateCandidateSQL, updateCandidateArgs, err := buildUpdateMemoryCandidateStatusQuery(candidateID, finalStatus, out.PromotedMemoryID)
	if err != nil {
		return core.ProposeMemoryPersistenceResult{}, err
	}
	if _, err := tx.Exec(ctx, updateCandidateSQL, updateCandidateArgs...); err != nil {
		return core.ProposeMemoryPersistenceResult{}, fmt.Errorf("update memory candidate status: %w", err)
	}

	out.Status = finalStatus
	if err := tx.Commit(ctx); err != nil {
		return core.ProposeMemoryPersistenceResult{}, fmt.Errorf("commit tx: %w", err)
	}

	return out, nil
}

func (r *Repository) SaveRunReceiptSummary(ctx context.Context, input core.RunReceiptSummary) (core.RunReceiptIDs, error) {
	if r == nil || r.pool == nil {
		return core.RunReceiptIDs{}, fmt.Errorf("postgres pool is required")
	}

	normalized, err := normalizeRunReceiptSummary(input)
	if err != nil {
		return core.RunReceiptIDs{}, err
	}
	if normalized.ReceiptID == "" {
		normalized.ReceiptID, err = newReceiptID()
		if err != nil {
			return core.RunReceiptIDs{}, err
		}
	}

	receiptJSON, err := json.Marshal(struct {
		TaskText     string   `json:"task_text"`
		Phase        string   `json:"phase"`
		ResolvedTags []string `json:"resolved_tags,omitempty"`
		PointerKeys  []string `json:"pointer_keys,omitempty"`
		MemoryIDs    []int64  `json:"memory_ids,omitempty"`
	}{
		TaskText:     normalized.TaskText,
		Phase:        normalized.Phase,
		ResolvedTags: normalized.ResolvedTags,
		PointerKeys:  normalized.PointerKeys,
		MemoryIDs:    normalized.MemoryIDs,
	})
	if err != nil {
		return core.RunReceiptIDs{}, fmt.Errorf("marshal receipt summary: %w", err)
	}

	runJSON, err := json.Marshal(struct {
		RequestID              string   `json:"request_id,omitempty"`
		Status                 string   `json:"status"`
		FilesChanged           []string `json:"files_changed,omitempty"`
		DefinitionOfDoneIssues []string `json:"definition_of_done_issues,omitempty"`
		Outcome                string   `json:"outcome,omitempty"`
	}{
		RequestID:              normalized.RequestID,
		Status:                 normalized.Status,
		FilesChanged:           normalized.FilesChanged,
		DefinitionOfDoneIssues: normalized.DefinitionOfDoneIssues,
		Outcome:                normalized.Outcome,
	})
	if err != nil {
		return core.RunReceiptIDs{}, fmt.Errorf("marshal run summary: %w", err)
	}

	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return core.RunReceiptIDs{}, fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	_, err = tx.Exec(ctx, `
INSERT INTO ctx_receipts (
	receipt_id,
	project_id,
	task_text,
	phase,
	resolved_tags,
	pointer_keys,
	memory_ids,
	summary_json
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8::jsonb)
ON CONFLICT (receipt_id) DO UPDATE
SET
	project_id = EXCLUDED.project_id,
	task_text = EXCLUDED.task_text,
	phase = EXCLUDED.phase,
	resolved_tags = EXCLUDED.resolved_tags,
	pointer_keys = EXCLUDED.pointer_keys,
	memory_ids = EXCLUDED.memory_ids,
	summary_json = EXCLUDED.summary_json
`, normalized.ReceiptID, normalized.ProjectID, normalized.TaskText, normalized.Phase, nonNilStringList(normalized.ResolvedTags), nonNilStringList(normalized.PointerKeys), nonNilInt64List(normalized.MemoryIDs), receiptJSON)
	if err != nil {
		return core.RunReceiptIDs{}, fmt.Errorf("upsert receipt summary: %w", err)
	}

	var runID int64
	err = tx.QueryRow(ctx, `
INSERT INTO ctx_runs (
	project_id,
	request_id,
	receipt_id,
	status,
	files_changed,
	outcome,
	summary_json
) VALUES ($1, $2, $3, $4, $5, $6, $7::jsonb)
RETURNING run_id
`, normalized.ProjectID, normalized.RequestID, normalized.ReceiptID, normalized.Status, nonNilStringList(normalized.FilesChanged), normalized.Outcome, runJSON).Scan(&runID)
	if err != nil {
		return core.RunReceiptIDs{}, fmt.Errorf("insert run summary: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return core.RunReceiptIDs{}, fmt.Errorf("commit tx: %w", err)
	}

	return core.RunReceiptIDs{
		RunID:     runID,
		ReceiptID: normalized.ReceiptID,
	}, nil
}

func (r *Repository) ApplySync(ctx context.Context, input core.SyncApplyInput) (core.SyncApplyResult, error) {
	if r == nil || r.pool == nil {
		return core.SyncApplyResult{}, fmt.Errorf("postgres pool is required")
	}

	normalized, err := normalizeSyncApplyInput(input)
	if err != nil {
		return core.SyncApplyResult{}, err
	}

	deletedPaths := make([]string, 0, len(normalized.Paths))
	presentPaths := make([]string, 0, len(normalized.Paths))
	presentRows := make([]core.SyncPath, 0, len(normalized.Paths))
	for _, syncPath := range normalized.Paths {
		if syncPath.Deleted {
			deletedPaths = append(deletedPaths, syncPath.Path)
			continue
		}
		presentPaths = append(presentPaths, syncPath.Path)
		presentRows = append(presentRows, syncPath)
	}

	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return core.SyncApplyResult{}, fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	result := core.SyncApplyResult{}

	if len(deletedPaths) > 0 {
		query, args, err := buildMarkDeletedPointersStaleQuery(normalized.ProjectID, deletedPaths)
		if err != nil {
			return core.SyncApplyResult{}, err
		}
		tag, err := tx.Exec(ctx, query, args...)
		if err != nil {
			return core.SyncApplyResult{}, fmt.Errorf("mark deleted pointers stale: %w", err)
		}
		result.DeletedMarkedStale = int(tag.RowsAffected())
	}

	if normalized.Mode == "full" {
		query, args, err := buildMarkMissingPointersStaleQuery(normalized.ProjectID, presentPaths)
		if err != nil {
			return core.SyncApplyResult{}, err
		}
		tag, err := tx.Exec(ctx, query, args...)
		if err != nil {
			return core.SyncApplyResult{}, fmt.Errorf("mark missing pointers stale: %w", err)
		}
		result.MarkedStale = int(tag.RowsAffected())
	}

	if len(presentRows) > 0 {
		query, args, err := buildRefreshPointersQuery(normalized.ProjectID, presentRows)
		if err != nil {
			return core.SyncApplyResult{}, err
		}
		tag, err := tx.Exec(ctx, query, args...)
		if err != nil {
			return core.SyncApplyResult{}, fmt.Errorf("refresh pointers: %w", err)
		}
		result.Updated = int(tag.RowsAffected())

		if normalized.InsertNewCandidates {
			query, args, err = buildInsertPointerCandidatesQuery(normalized.ProjectID, presentRows)
			if err != nil {
				return core.SyncApplyResult{}, err
			}
			tag, err = tx.Exec(ctx, query, args...)
			if err != nil {
				return core.SyncApplyResult{}, fmt.Errorf("insert pointer candidates: %w", err)
			}
			result.NewCandidates = int(tag.RowsAffected())
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return core.SyncApplyResult{}, fmt.Errorf("commit tx: %w", err)
	}

	return result, nil
}

func normalizePointerStubs(projectID string, stubs []core.PointerStub) ([]core.PointerStub, error) {
	projectID = strings.TrimSpace(projectID)
	if projectID == "" {
		return nil, fmt.Errorf("project_id is required")
	}
	if len(stubs) == 0 {
		return nil, nil
	}

	stubByKey := make(map[string]core.PointerStub, len(stubs))
	for _, raw := range stubs {
		normalizedPath := normalizeSyncPath(raw.Path)
		if normalizedPath == "" {
			continue
		}

		pointerKey := strings.TrimSpace(raw.PointerKey)
		if pointerKey == "" {
			pointerKey = fmt.Sprintf("%s:%s", projectID, normalizedPath)
		}

		kind := normalizePointerStubKind(raw.Kind)

		label := strings.TrimSpace(raw.Label)
		if label == "" {
			label = strings.TrimSpace(path.Base(normalizedPath))
			if label == "" || label == "." || label == "/" {
				label = normalizedPath
			}
		}

		description := strings.TrimSpace(raw.Description)
		if description == "" {
			description = "Auto-indexed pointer stub. Curate label, description, and tags."
		}

		tags := normalizeStringList(raw.Tags)
		if len(tags) == 0 {
			tags = []string{"auto-indexed", kind}
		}

		stubByKey[pointerKey] = core.PointerStub{
			PointerKey:  pointerKey,
			Path:        normalizedPath,
			Kind:        kind,
			Label:       label,
			Description: description,
			Tags:        tags,
		}
	}

	if len(stubByKey) == 0 {
		return nil, nil
	}

	keys := make([]string, 0, len(stubByKey))
	for key := range stubByKey {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	out := make([]core.PointerStub, 0, len(keys))
	for _, key := range keys {
		out = append(out, stubByKey[key])
	}
	return out, nil
}

func normalizePointerStubKind(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "rule":
		return "rule"
	case "doc":
		return "doc"
	case "test":
		return "test"
	case "command":
		return "command"
	default:
		return "code"
	}
}

type normalizedRunSummary struct {
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

func normalizeRunReceiptSummary(input core.RunReceiptSummary) (normalizedRunSummary, error) {
	projectID := strings.TrimSpace(input.ProjectID)
	if projectID == "" {
		return normalizedRunSummary{}, fmt.Errorf("project_id is required")
	}

	status := strings.TrimSpace(input.Status)
	if status == "" {
		status = "accepted"
	}

	phase := strings.TrimSpace(input.Phase)
	if phase == "" {
		phase = "execute"
	}

	return normalizedRunSummary{
		ProjectID:              projectID,
		RequestID:              strings.TrimSpace(input.RequestID),
		ReceiptID:              strings.TrimSpace(input.ReceiptID),
		TaskText:               strings.TrimSpace(input.TaskText),
		Phase:                  phase,
		Status:                 status,
		ResolvedTags:           normalizeStringList(input.ResolvedTags),
		PointerKeys:            normalizeStringList(input.PointerKeys),
		MemoryIDs:              normalizeInt64List(input.MemoryIDs),
		FilesChanged:           normalizeStringList(input.FilesChanged),
		DefinitionOfDoneIssues: normalizeStringList(input.DefinitionOfDoneIssues),
		Outcome:                strings.TrimSpace(input.Outcome),
	}, nil
}

func normalizeProposeMemoryPersistence(input core.ProposeMemoryPersistence) (core.ProposeMemoryPersistence, error) {
	projectID := strings.TrimSpace(input.ProjectID)
	if projectID == "" {
		return core.ProposeMemoryPersistence{}, fmt.Errorf("project_id is required")
	}
	receiptID := strings.TrimSpace(input.ReceiptID)
	if receiptID == "" {
		return core.ProposeMemoryPersistence{}, fmt.Errorf("receipt_id is required")
	}
	category := strings.TrimSpace(input.Category)
	if category == "" {
		return core.ProposeMemoryPersistence{}, fmt.Errorf("category is required")
	}
	subject := strings.TrimSpace(input.Subject)
	if subject == "" {
		return core.ProposeMemoryPersistence{}, fmt.Errorf("subject is required")
	}
	content := strings.TrimSpace(input.Content)
	if content == "" {
		return core.ProposeMemoryPersistence{}, fmt.Errorf("content is required")
	}
	if input.Confidence < 1 || input.Confidence > 5 {
		return core.ProposeMemoryPersistence{}, fmt.Errorf("confidence must be 1..5")
	}

	evidencePointerKeys := normalizeStringList(input.EvidencePointerKeys)
	if len(evidencePointerKeys) == 0 {
		return core.ProposeMemoryPersistence{}, fmt.Errorf("evidence_pointer_keys is required")
	}

	dedupeKey := strings.TrimSpace(input.DedupeKey)
	if dedupeKey == "" {
		return core.ProposeMemoryPersistence{}, fmt.Errorf("dedupe_key is required")
	}

	return core.ProposeMemoryPersistence{
		ProjectID:           projectID,
		ReceiptID:           receiptID,
		Category:            category,
		Subject:             subject,
		Content:             content,
		Confidence:          input.Confidence,
		Tags:                normalizeStringList(input.Tags),
		RelatedPointerKeys:  normalizeStringList(input.RelatedPointerKeys),
		EvidencePointerKeys: evidencePointerKeys,
		DedupeKey:           dedupeKey,
		Validation: core.ProposeMemoryValidation{
			HardPassed: input.Validation.HardPassed,
			SoftPassed: input.Validation.SoftPassed,
			Errors:     normalizeStringList(input.Validation.Errors),
			Warnings:   normalizeStringList(input.Validation.Warnings),
		},
		AutoPromote: input.AutoPromote,
		Promotable:  input.Promotable,
	}, nil
}

func newReceiptID() (string, error) {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", fmt.Errorf("generate receipt id: %w", err)
	}
	return fmt.Sprintf("receipt-%d-%s", time.Now().UTC().UnixNano(), hex.EncodeToString(b[:])), nil
}

func normalizeSyncApplyInput(input core.SyncApplyInput) (core.SyncApplyInput, error) {
	projectID := strings.TrimSpace(input.ProjectID)
	if projectID == "" {
		return core.SyncApplyInput{}, fmt.Errorf("project_id is required")
	}

	mode := strings.TrimSpace(input.Mode)
	if mode == "" {
		mode = "changed"
	}
	if mode != "changed" && mode != "full" && mode != "working_tree" {
		return core.SyncApplyInput{}, fmt.Errorf("mode must be changed|full|working_tree")
	}

	pathByKey := make(map[string]core.SyncPath, len(input.Paths))
	for _, raw := range input.Paths {
		pathKey := normalizeSyncPath(raw.Path)
		if pathKey == "" {
			continue
		}

		if raw.Deleted {
			current, ok := pathByKey[pathKey]
			if ok && !current.Deleted {
				continue
			}
			pathByKey[pathKey] = core.SyncPath{
				Path:    pathKey,
				Deleted: true,
			}
			continue
		}

		contentHash := strings.TrimSpace(raw.ContentHash)
		if contentHash == "" {
			return core.SyncApplyInput{}, fmt.Errorf("content_hash is required for path %q", pathKey)
		}
		pathByKey[pathKey] = core.SyncPath{
			Path:        pathKey,
			ContentHash: contentHash,
			Deleted:     false,
		}
	}

	keys := make([]string, 0, len(pathByKey))
	for key := range pathByKey {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	paths := make([]core.SyncPath, 0, len(keys))
	for _, key := range keys {
		paths = append(paths, pathByKey[key])
	}

	return core.SyncApplyInput{
		ProjectID:           projectID,
		Mode:                mode,
		InsertNewCandidates: input.InsertNewCandidates,
		Paths:               paths,
	}, nil
}

func derivePlanStatus(items []core.WorkItem) string {
	if len(items) == 0 {
		return core.PlanStatusPending
	}

	hasPending := false
	hasInProgress := false
	hasBlocked := false
	hasCompleted := false

	for _, item := range items {
		switch normalizeWorkItemStatus(item.Status) {
		case core.WorkItemStatusBlocked:
			hasBlocked = true
		case core.WorkItemStatusInProgress:
			hasInProgress = true
		case core.WorkItemStatusComplete:
			hasCompleted = true
		default:
			hasPending = true
		}
	}

	switch {
	case hasBlocked:
		return core.PlanStatusBlocked
	case hasInProgress:
		return core.PlanStatusInProgress
	case hasPending:
		return core.PlanStatusPending
	case hasCompleted:
		return core.PlanStatusComplete
	default:
		return core.PlanStatusPending
	}
}
