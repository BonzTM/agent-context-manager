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

	"github.com/bonztm/agent-context-manager/internal/core"
)

type Repository struct {
	pool *pgxpool.Pool
}

var _ core.Repository = (*Repository)(nil)
var _ core.WorkPlanRepository = (*Repository)(nil)
var _ core.VerificationRepository = (*Repository)(nil)

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
FROM acm_pointers
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
INSERT INTO acm_pointers (
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

func (r *Repository) UpsertWorkPlan(ctx context.Context, input core.WorkPlanUpsertInput) (core.WorkPlanUpsertResult, error) {
	if r == nil || r.pool == nil {
		return core.WorkPlanUpsertResult{}, fmt.Errorf("postgres pool is required")
	}

	projectID := strings.TrimSpace(input.ProjectID)
	if projectID == "" {
		return core.WorkPlanUpsertResult{}, fmt.Errorf("project_id is required")
	}
	planKey := strings.TrimSpace(input.PlanKey)
	if planKey == "" {
		return core.WorkPlanUpsertResult{}, fmt.Errorf("plan_key is required")
	}
	mode := normalizeWorkPlanMode(input.Mode)

	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return core.WorkPlanUpsertResult{}, fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	current, found, err := lookupWorkPlanRowTx(ctx, tx, projectID, planKey)
	if err != nil {
		return core.WorkPlanUpsertResult{}, err
	}

	next := buildNextWorkPlanState(current, found, input, mode)
	if err := upsertWorkPlanRowTx(ctx, tx, next); err != nil {
		return core.WorkPlanUpsertResult{}, err
	}

	updated := 0
	normalizedTasks := normalizeWorkPlanTasks(input.Tasks)
	if mode == core.WorkPlanModeReplace {
		tag, err := tx.Exec(ctx, `
DELETE FROM acm_work_plan_tasks
WHERE project_id = $1
	AND plan_key = $2
`, projectID, planKey)
		if err != nil {
			return core.WorkPlanUpsertResult{}, fmt.Errorf("delete work plan tasks: %w", err)
		}
		updated += int(tag.RowsAffected())
	}

	for _, task := range normalizedTasks {
		tag, err := tx.Exec(ctx, `
INSERT INTO acm_work_plan_tasks (
	project_id,
	plan_key,
	task_key,
	summary,
	status,
	parent_task_key,
	depends_on,
	acceptance_criteria,
	references_list,
	external_refs,
	blocked_reason,
	outcome,
	evidence,
	created_at,
	updated_at
) VALUES (
	$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, NOW(), NOW()
)
ON CONFLICT(project_id, plan_key, task_key) DO UPDATE SET
	summary = EXCLUDED.summary,
	status = EXCLUDED.status,
	parent_task_key = EXCLUDED.parent_task_key,
	depends_on = EXCLUDED.depends_on,
	acceptance_criteria = EXCLUDED.acceptance_criteria,
	references_list = EXCLUDED.references_list,
	external_refs = EXCLUDED.external_refs,
	blocked_reason = EXCLUDED.blocked_reason,
	outcome = EXCLUDED.outcome,
	evidence = EXCLUDED.evidence,
	updated_at = NOW()
`, projectID, planKey, task.ItemKey, strings.TrimSpace(task.Summary), storageWorkItemStatus(task.Status), strings.TrimSpace(task.ParentTaskKey), nonNilStringList(task.DependsOn), nonNilStringList(task.AcceptanceCriteria), nonNilStringList(task.References), nonNilStringList(task.ExternalRefs), strings.TrimSpace(task.BlockedReason), strings.TrimSpace(task.Outcome), nonNilStringList(task.Evidence))
		if err != nil {
			return core.WorkPlanUpsertResult{}, fmt.Errorf("upsert work plan task: %w", err)
		}
		updated += int(tag.RowsAffected())
	}

	if strings.TrimSpace(input.Status) == "" {
		tasks, err := listWorkPlanTasksTx(ctx, tx, projectID, planKey)
		if err != nil {
			return core.WorkPlanUpsertResult{}, err
		}
		derivedStatus := derivePlanStatus(tasks)
		if _, err := tx.Exec(ctx, `
UPDATE acm_work_plans
SET status = $3, updated_at = NOW()
WHERE project_id = $1
	AND plan_key = $2
`, projectID, planKey, storageWorkItemStatus(derivedStatus)); err != nil {
			return core.WorkPlanUpsertResult{}, fmt.Errorf("update work plan status: %w", err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return core.WorkPlanUpsertResult{}, fmt.Errorf("commit tx: %w", err)
	}

	plan, err := r.LookupWorkPlan(ctx, core.WorkPlanLookupQuery{
		ProjectID: projectID,
		PlanKey:   planKey,
	})
	if err != nil {
		return core.WorkPlanUpsertResult{}, err
	}

	return core.WorkPlanUpsertResult{
		Plan:    plan,
		Updated: updated,
	}, nil
}

func (r *Repository) LookupWorkPlan(ctx context.Context, input core.WorkPlanLookupQuery) (core.WorkPlan, error) {
	if r == nil || r.pool == nil {
		return core.WorkPlan{}, fmt.Errorf("postgres pool is required")
	}

	projectID := strings.TrimSpace(input.ProjectID)
	if projectID == "" {
		return core.WorkPlan{}, fmt.Errorf("project_id is required")
	}
	planKey := strings.TrimSpace(input.PlanKey)
	receiptID := strings.TrimSpace(input.ReceiptID)
	if planKey == "" && receiptID == "" {
		return core.WorkPlan{}, fmt.Errorf("plan_key or receipt_id is required")
	}

	if planKey == "" {
		err := r.pool.QueryRow(ctx, `
SELECT plan_key
FROM acm_work_plans
WHERE project_id = $1
	AND receipt_id = $2
ORDER BY updated_at DESC, plan_key ASC
LIMIT 1
`, projectID, receiptID).Scan(&planKey)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return core.WorkPlan{}, core.ErrWorkPlanNotFound
			}
			return core.WorkPlan{}, fmt.Errorf("query work plan key by receipt: %w", err)
		}
	}

	plan, found, err := lookupWorkPlanRow(ctx, r.pool, projectID, planKey)
	if err != nil {
		return core.WorkPlan{}, err
	}
	if !found {
		return core.WorkPlan{}, core.ErrWorkPlanNotFound
	}

	tasks, err := listWorkPlanTasks(ctx, r.pool, projectID, planKey)
	if err != nil {
		return core.WorkPlan{}, err
	}
	plan.Tasks = tasks
	return plan, nil
}

func (r *Repository) ListWorkPlans(ctx context.Context, input core.WorkPlanListQuery) ([]core.WorkPlanSummary, error) {
	if r == nil || r.pool == nil {
		return nil, fmt.Errorf("postgres pool is required")
	}

	projectID := strings.TrimSpace(input.ProjectID)
	if projectID == "" {
		return nil, fmt.Errorf("project_id is required")
	}
	limit := input.Limit
	if limit <= 0 {
		limit = 8
	}
	if limit > 100 {
		limit = 100
	}

	rows, err := r.pool.Query(ctx, `
SELECT
	p.plan_key,
	p.title,
	p.status,
	p.kind,
	p.parent_plan_key,
	COUNT(t.task_id)::bigint AS task_count_total,
	COUNT(*) FILTER (WHERE t.status = 'pending')::bigint AS task_count_pending,
	COUNT(*) FILTER (WHERE t.status = 'in_progress')::bigint AS task_count_in_progress,
	COUNT(*) FILTER (WHERE t.status = 'blocked')::bigint AS task_count_blocked,
	COUNT(*) FILTER (WHERE t.status = 'completed')::bigint AS task_count_completed,
	p.updated_at
FROM acm_work_plans p
LEFT JOIN acm_work_plan_tasks t
	ON t.project_id = p.project_id
	AND t.plan_key = p.plan_key
WHERE p.project_id = $1
	AND p.status <> 'completed'
GROUP BY p.plan_key, p.title, p.status, p.kind, p.parent_plan_key, p.updated_at
ORDER BY p.updated_at DESC, p.plan_key ASC
LIMIT $2
`, projectID, limit)
	if err != nil {
		return nil, fmt.Errorf("query work plans: %w", err)
	}
	defer rows.Close()

	out := make([]core.WorkPlanSummary, 0)
	for rows.Next() {
		var (
			planKey             string
			title               string
			status              string
			kind                string
			parentPlanKey       string
			taskCountTotal      int64
			taskCountPending    int64
			taskCountInProgress int64
			taskCountBlocked    int64
			taskCountComplete   int64
			updatedAt           time.Time
		)
		if err := rows.Scan(
			&planKey,
			&title,
			&status,
			&kind,
			&parentPlanKey,
			&taskCountTotal,
			&taskCountPending,
			&taskCountInProgress,
			&taskCountBlocked,
			&taskCountComplete,
			&updatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan work plan summary: %w", err)
		}
		planKey = strings.TrimSpace(planKey)
		if planKey == "" {
			continue
		}
		normalizedStatus := normalizeWorkItemStatus(status)
		summary := strings.TrimSpace(title)
		if summary == "" {
			summary = fmt.Sprintf("Plan %s is %s", planKey, normalizedStatus)
		}
		if taskCountTotal > 0 {
			summary = fmt.Sprintf("%s (%d tasks)", summary, taskCountTotal)
		}
		out = append(out, core.WorkPlanSummary{
			PlanKey:             planKey,
			Summary:             summary,
			Status:              normalizedStatus,
			Kind:                strings.TrimSpace(kind),
			ParentPlanKey:       strings.TrimSpace(parentPlanKey),
			TaskCountTotal:      int(taskCountTotal),
			TaskCountPending:    int(taskCountPending),
			TaskCountInProgress: int(taskCountInProgress),
			TaskCountBlocked:    int(taskCountBlocked),
			TaskCountComplete:   int(taskCountComplete),
			UpdatedAt:           updatedAt.UTC(),
		})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate work plans: %w", err)
	}

	if len(out) == 0 {
		return nil, nil
	}

	activeTaskKeysByPlan, err := listActiveWorkPlanTaskKeys(ctx, r.pool, projectID, workPlanSummaryKeys(out))
	if err != nil {
		return nil, err
	}
	for i := range out {
		out[i].ActiveTaskKeys = append([]string(nil), activeTaskKeysByPlan[out[i].PlanKey]...)
	}

	return out, nil
}

func listActiveWorkPlanTaskKeys(ctx context.Context, q pgxRowsQuerier, projectID string, planKeys []string) (map[string][]string, error) {
	if len(planKeys) == 0 {
		return nil, nil
	}

	rows, err := q.Query(ctx, `
SELECT
	plan_key,
	task_key
FROM acm_work_plan_tasks
WHERE project_id = $1
	AND plan_key = ANY($2)
	AND status <> 'completed'
ORDER BY
	plan_key ASC,
	CASE status
		WHEN 'blocked' THEN 0
		WHEN 'in_progress' THEN 1
		WHEN 'pending' THEN 2
		ELSE 3
	END ASC,
	task_key ASC
`, projectID, planKeys)
	if err != nil {
		return nil, fmt.Errorf("query active work plan task keys: %w", err)
	}
	defer rows.Close()

	out := make(map[string][]string, len(planKeys))
	for rows.Next() {
		var planKey string
		var taskKey string
		if err := rows.Scan(&planKey, &taskKey); err != nil {
			return nil, fmt.Errorf("scan active work plan task key: %w", err)
		}
		planKey = strings.TrimSpace(planKey)
		taskKey = strings.TrimSpace(taskKey)
		if planKey == "" || taskKey == "" {
			continue
		}
		out[planKey] = append(out[planKey], taskKey)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate active work plan task keys: %w", err)
	}

	return out, nil
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
INSERT INTO acm_receipts (
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
INSERT INTO acm_runs (
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

func (r *Repository) UpsertReceiptScope(ctx context.Context, input core.ReceiptScope) error {
	if r == nil || r.pool == nil {
		return fmt.Errorf("postgres pool is required")
	}

	normalized, err := normalizeReceiptScope(input)
	if err != nil {
		return err
	}

	_, err = r.pool.Exec(ctx, `
INSERT INTO acm_receipts (
	receipt_id,
	project_id,
	task_text,
	phase,
	resolved_tags,
	pointer_keys,
	memory_ids,
	summary_json
) VALUES ($1, $2, $3, $4, $5, $6, $7, '{}'::jsonb)
ON CONFLICT (receipt_id) DO UPDATE
SET
	project_id = EXCLUDED.project_id,
	task_text = EXCLUDED.task_text,
	phase = EXCLUDED.phase,
	resolved_tags = EXCLUDED.resolved_tags,
	pointer_keys = EXCLUDED.pointer_keys,
	memory_ids = EXCLUDED.memory_ids
`, normalized.ReceiptID, normalized.ProjectID, normalized.TaskText, normalized.Phase, nonNilStringList(normalized.ResolvedTags), nonNilStringList(normalized.PointerKeys), nonNilInt64List(normalized.MemoryIDs))
	if err != nil {
		return fmt.Errorf("upsert receipt scope: %w", err)
	}
	return nil
}

func (r *Repository) SaveVerificationBatch(ctx context.Context, input core.VerificationBatch) error {
	if r == nil || r.pool == nil {
		return fmt.Errorf("postgres pool is required")
	}

	normalized, err := normalizeVerificationBatch(input)
	if err != nil {
		return err
	}

	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	_, err = tx.Exec(ctx, `
INSERT INTO acm_verification_batches (
	batch_run_id,
	project_id,
	receipt_id,
	plan_key,
	phase,
	tests_source_path,
	status,
	passed,
	selected_test_ids,
	created_at
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
`, normalized.BatchRunID, normalized.ProjectID, normalized.ReceiptID, normalized.PlanKey, normalized.Phase, normalized.TestsSourcePath, normalized.Status, normalized.Passed, nonNilStringListPreserveOrder(normalized.SelectedTestIDs), normalized.CreatedAt)
	if err != nil {
		return fmt.Errorf("insert verification batch: %w", err)
	}

	for _, result := range normalized.Results {
		_, err := tx.Exec(ctx, `
INSERT INTO acm_verification_results (
	batch_run_id,
	project_id,
	test_id,
	definition_hash,
	summary,
	command_argv,
	command_cwd,
	timeout_sec,
	expected_exit_code,
	selection_reasons,
	status,
	exit_code,
	duration_ms,
	stdout_excerpt,
	stderr_excerpt,
	started_at,
	finished_at
) VALUES (
	$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17
)
`, normalized.BatchRunID, normalized.ProjectID, result.TestID, result.DefinitionHash, result.Summary, nonNilStringListPreserveOrder(result.CommandArgv), result.CommandCWD, result.TimeoutSec, result.ExpectedExitCode, nonNilStringListPreserveOrder(result.SelectionReasons), result.Status, result.ExitCode, result.DurationMS, result.StdoutExcerpt, result.StderrExcerpt, result.StartedAt, result.FinishedAt)
		if err != nil {
			return fmt.Errorf("insert verification result %s: %w", result.TestID, err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit tx: %w", err)
	}
	return nil
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

func normalizeWorkPlanMode(raw core.WorkPlanMode) core.WorkPlanMode {
	switch strings.TrimSpace(string(raw)) {
	case string(core.WorkPlanModeReplace):
		return core.WorkPlanModeReplace
	default:
		return core.WorkPlanModeMerge
	}
}

func normalizeWorkPlanStages(raw core.WorkPlanStages) core.WorkPlanStages {
	out := core.WorkPlanStages{
		SpecOutline:        normalizeWorkItemStatus(raw.SpecOutline),
		RefinedSpec:        normalizeWorkItemStatus(raw.RefinedSpec),
		ImplementationPlan: normalizeWorkItemStatus(raw.ImplementationPlan),
	}
	if strings.TrimSpace(raw.SpecOutline) == "" {
		out.SpecOutline = core.WorkItemStatusPending
	}
	if strings.TrimSpace(raw.RefinedSpec) == "" {
		out.RefinedSpec = core.WorkItemStatusPending
	}
	if strings.TrimSpace(raw.ImplementationPlan) == "" {
		out.ImplementationPlan = core.WorkItemStatusPending
	}
	return out
}

func normalizeWorkPlanTasks(tasks []core.WorkItem) []core.WorkItem {
	if len(tasks) == 0 {
		return nil
	}

	priority := map[string]int{
		workItemStatusComplete:   0,
		workItemStatusCompleted:  0,
		workItemStatusPending:    1,
		workItemStatusInProgress: 2,
		workItemStatusBlocked:    3,
	}
	byKey := make(map[string]core.WorkItem, len(tasks))
	for _, raw := range tasks {
		itemKey := strings.TrimSpace(raw.ItemKey)
		if itemKey == "" {
			continue
		}
		status := normalizeWorkItemStatus(raw.Status)
		normalized := core.WorkItem{
			ItemKey:            itemKey,
			Summary:            strings.TrimSpace(raw.Summary),
			Status:             status,
			ParentTaskKey:      strings.TrimSpace(raw.ParentTaskKey),
			DependsOn:          normalizeStringList(raw.DependsOn),
			AcceptanceCriteria: normalizeStringList(raw.AcceptanceCriteria),
			References:         normalizeStringList(raw.References),
			ExternalRefs:       normalizeStringList(raw.ExternalRefs),
			BlockedReason:      strings.TrimSpace(raw.BlockedReason),
			Outcome:            strings.TrimSpace(raw.Outcome),
			Evidence:           normalizeStringList(raw.Evidence),
			UpdatedAt:          raw.UpdatedAt.UTC(),
		}
		current, exists := byKey[itemKey]
		if !exists || priority[status] >= priority[current.Status] {
			byKey[itemKey] = normalized
		}
	}
	if len(byKey) == 0 {
		return nil
	}
	keys := make([]string, 0, len(byKey))
	for key := range byKey {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	out := make([]core.WorkItem, 0, len(keys))
	for _, key := range keys {
		out = append(out, byKey[key])
	}
	return out
}

func buildNextWorkPlanState(current core.WorkPlan, found bool, input core.WorkPlanUpsertInput, mode core.WorkPlanMode) core.WorkPlan {
	projectID := strings.TrimSpace(input.ProjectID)
	planKey := strings.TrimSpace(input.PlanKey)
	next := core.WorkPlan{
		ProjectID: projectID,
		PlanKey:   planKey,
		Status:    core.PlanStatusPending,
		Stages: core.WorkPlanStages{
			SpecOutline:        core.PlanStatusPending,
			RefinedSpec:        core.PlanStatusPending,
			ImplementationPlan: core.PlanStatusPending,
		},
	}
	if found {
		next = current
	}
	if mode == core.WorkPlanModeReplace {
		next = core.WorkPlan{
			ProjectID: projectID,
			PlanKey:   planKey,
			Status:    core.PlanStatusPending,
			Stages: core.WorkPlanStages{
				SpecOutline:        core.PlanStatusPending,
				RefinedSpec:        core.PlanStatusPending,
				ImplementationPlan: core.PlanStatusPending,
			},
		}
	}

	trimmedReceipt := strings.TrimSpace(input.ReceiptID)
	if trimmedReceipt != "" || mode == core.WorkPlanModeReplace {
		next.ReceiptID = trimmedReceipt
	}

	trimmedTitle := strings.TrimSpace(input.Title)
	if trimmedTitle != "" || mode == core.WorkPlanModeReplace {
		next.Title = trimmedTitle
	}

	trimmedObjective := strings.TrimSpace(input.Objective)
	if trimmedObjective != "" || mode == core.WorkPlanModeReplace {
		next.Objective = trimmedObjective
	}

	trimmedKind := strings.TrimSpace(input.Kind)
	if trimmedKind != "" || mode == core.WorkPlanModeReplace {
		next.Kind = strings.ToLower(trimmedKind)
	}

	trimmedParentPlanKey := strings.TrimSpace(input.ParentPlanKey)
	if trimmedParentPlanKey != "" || mode == core.WorkPlanModeReplace {
		next.ParentPlanKey = trimmedParentPlanKey
	}

	trimmedStatus := strings.TrimSpace(input.Status)
	if trimmedStatus != "" {
		next.Status = normalizeWorkItemStatus(trimmedStatus)
	}
	if mode == core.WorkPlanModeReplace && trimmedStatus == "" {
		next.Status = core.PlanStatusPending
	}

	if input.Stages.SpecOutline != "" || input.Stages.RefinedSpec != "" || input.Stages.ImplementationPlan != "" || mode == core.WorkPlanModeReplace {
		if mode == core.WorkPlanModeReplace {
			next.Stages = core.WorkPlanStages{}
		}
		next.Stages = mergeWorkPlanStages(next.Stages, input.Stages, mode)
	}
	next.Stages = normalizeWorkPlanStages(next.Stages)

	if input.InScope != nil || mode == core.WorkPlanModeReplace {
		next.InScope = normalizeStringList(input.InScope)
	}
	if input.OutOfScope != nil || mode == core.WorkPlanModeReplace {
		next.OutOfScope = normalizeStringList(input.OutOfScope)
	}
	if input.Constraints != nil || mode == core.WorkPlanModeReplace {
		next.Constraints = normalizeStringList(input.Constraints)
	}
	if input.References != nil || mode == core.WorkPlanModeReplace {
		next.References = normalizeStringList(input.References)
	}
	if input.ExternalRefs != nil || mode == core.WorkPlanModeReplace {
		next.ExternalRefs = normalizeStringList(input.ExternalRefs)
	}

	next.ProjectID = projectID
	next.PlanKey = planKey
	next.Status = normalizeWorkItemStatus(next.Status)
	return next
}

func mergeWorkPlanStages(current, incoming core.WorkPlanStages, mode core.WorkPlanMode) core.WorkPlanStages {
	out := current
	if mode == core.WorkPlanModeReplace {
		out = core.WorkPlanStages{}
	}
	if strings.TrimSpace(incoming.SpecOutline) != "" || mode == core.WorkPlanModeReplace {
		out.SpecOutline = normalizeWorkItemStatus(incoming.SpecOutline)
	}
	if strings.TrimSpace(incoming.RefinedSpec) != "" || mode == core.WorkPlanModeReplace {
		out.RefinedSpec = normalizeWorkItemStatus(incoming.RefinedSpec)
	}
	if strings.TrimSpace(incoming.ImplementationPlan) != "" || mode == core.WorkPlanModeReplace {
		out.ImplementationPlan = normalizeWorkItemStatus(incoming.ImplementationPlan)
	}
	return out
}

func upsertWorkPlanRowTx(ctx context.Context, tx pgx.Tx, plan core.WorkPlan) error {
	receiptValue := any(nil)
	if strings.TrimSpace(plan.ReceiptID) != "" {
		receiptValue = strings.TrimSpace(plan.ReceiptID)
	}
	_, err := tx.Exec(ctx, `
INSERT INTO acm_work_plans (
	project_id,
	plan_key,
	receipt_id,
	title,
	objective,
	kind,
	parent_plan_key,
	status,
	stage_spec_outline,
	stage_refined_spec,
	stage_implementation_plan,
	in_scope,
	out_of_scope,
	constraints_list,
	references_list,
	external_refs,
	created_at,
	updated_at
) VALUES (
	$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, NOW(), NOW()
)
ON CONFLICT(project_id, plan_key) DO UPDATE SET
	receipt_id = EXCLUDED.receipt_id,
	title = EXCLUDED.title,
	objective = EXCLUDED.objective,
	kind = EXCLUDED.kind,
	parent_plan_key = EXCLUDED.parent_plan_key,
	status = EXCLUDED.status,
	stage_spec_outline = EXCLUDED.stage_spec_outline,
	stage_refined_spec = EXCLUDED.stage_refined_spec,
	stage_implementation_plan = EXCLUDED.stage_implementation_plan,
	in_scope = EXCLUDED.in_scope,
	out_of_scope = EXCLUDED.out_of_scope,
	constraints_list = EXCLUDED.constraints_list,
	references_list = EXCLUDED.references_list,
	external_refs = EXCLUDED.external_refs,
	updated_at = NOW()
`, strings.TrimSpace(plan.ProjectID), strings.TrimSpace(plan.PlanKey), receiptValue, strings.TrimSpace(plan.Title), strings.TrimSpace(plan.Objective), strings.ToLower(strings.TrimSpace(plan.Kind)), strings.TrimSpace(plan.ParentPlanKey), storageWorkItemStatus(plan.Status), storageWorkItemStatus(plan.Stages.SpecOutline), storageWorkItemStatus(plan.Stages.RefinedSpec), storageWorkItemStatus(plan.Stages.ImplementationPlan), nonNilStringList(plan.InScope), nonNilStringList(plan.OutOfScope), nonNilStringList(plan.Constraints), nonNilStringList(plan.References), nonNilStringList(plan.ExternalRefs))
	if err != nil {
		return fmt.Errorf("upsert work plan: %w", err)
	}
	return nil
}

func lookupWorkPlanRowTx(ctx context.Context, tx pgx.Tx, projectID, planKey string) (core.WorkPlan, bool, error) {
	row := tx.QueryRow(ctx, `
SELECT
	COALESCE(receipt_id, ''),
	title,
	objective,
	kind,
	parent_plan_key,
	status,
	stage_spec_outline,
	stage_refined_spec,
	stage_implementation_plan,
	in_scope,
	out_of_scope,
	constraints_list,
	references_list,
	external_refs,
	updated_at
FROM acm_work_plans
WHERE project_id = $1
	AND plan_key = $2
`, projectID, planKey)

	var (
		receiptID           string
		title               string
		objective           string
		kind                string
		parentPlanKey       string
		status              string
		stageSpecOutline    string
		stageRefinedSpec    string
		stageImplementation string
		inScope             []string
		outOfScope          []string
		constraints         []string
		references          []string
		externalRefs        []string
		updatedAt           time.Time
	)
	if err := row.Scan(&receiptID, &title, &objective, &kind, &parentPlanKey, &status, &stageSpecOutline, &stageRefinedSpec, &stageImplementation, &inScope, &outOfScope, &constraints, &references, &externalRefs, &updatedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return core.WorkPlan{}, false, nil
		}
		return core.WorkPlan{}, false, fmt.Errorf("query work plan: %w", err)
	}

	return core.WorkPlan{
		ProjectID:     projectID,
		PlanKey:       planKey,
		ReceiptID:     strings.TrimSpace(receiptID),
		Title:         strings.TrimSpace(title),
		Objective:     strings.TrimSpace(objective),
		Kind:          strings.TrimSpace(kind),
		ParentPlanKey: strings.TrimSpace(parentPlanKey),
		Status:        normalizeWorkItemStatus(status),
		Stages:        normalizeWorkPlanStages(core.WorkPlanStages{SpecOutline: stageSpecOutline, RefinedSpec: stageRefinedSpec, ImplementationPlan: stageImplementation}),
		InScope:       normalizeStringList(inScope),
		OutOfScope:    normalizeStringList(outOfScope),
		Constraints:   normalizeStringList(constraints),
		References:    normalizeStringList(references),
		ExternalRefs:  normalizeStringList(externalRefs),
		UpdatedAt:     updatedAt.UTC(),
	}, true, nil
}

func listWorkPlanTasksTx(ctx context.Context, tx pgx.Tx, projectID, planKey string) ([]core.WorkItem, error) {
	rows, err := tx.Query(ctx, `
SELECT
	task_key,
	summary,
	status,
	parent_task_key,
	depends_on,
	acceptance_criteria,
	references_list,
	external_refs,
	blocked_reason,
	outcome,
	evidence,
	updated_at
FROM acm_work_plan_tasks
WHERE project_id = $1
	AND plan_key = $2
ORDER BY task_key ASC
`, projectID, planKey)
	if err != nil {
		return nil, fmt.Errorf("query work plan tasks: %w", err)
	}
	defer rows.Close()

	items := make([]core.WorkItem, 0)
	for rows.Next() {
		var item core.WorkItem
		if err := rows.Scan(&item.ItemKey, &item.Summary, &item.Status, &item.ParentTaskKey, &item.DependsOn, &item.AcceptanceCriteria, &item.References, &item.ExternalRefs, &item.BlockedReason, &item.Outcome, &item.Evidence, &item.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan work plan task: %w", err)
		}
		item.ItemKey = strings.TrimSpace(item.ItemKey)
		if item.ItemKey == "" {
			continue
		}
		item.Summary = strings.TrimSpace(item.Summary)
		item.Status = normalizeWorkItemStatus(item.Status)
		item.ParentTaskKey = strings.TrimSpace(item.ParentTaskKey)
		item.DependsOn = normalizeStringList(item.DependsOn)
		item.AcceptanceCriteria = normalizeStringList(item.AcceptanceCriteria)
		item.References = normalizeStringList(item.References)
		item.ExternalRefs = normalizeStringList(item.ExternalRefs)
		item.BlockedReason = strings.TrimSpace(item.BlockedReason)
		item.Outcome = strings.TrimSpace(item.Outcome)
		item.Evidence = normalizeStringList(item.Evidence)
		item.UpdatedAt = item.UpdatedAt.UTC()
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate work plan tasks: %w", err)
	}
	return items, nil
}

func lookupWorkPlanRow(ctx context.Context, q pgxRowsQuerier, projectID, planKey string) (core.WorkPlan, bool, error) {
	row := q.QueryRow(ctx, `
SELECT
	COALESCE(receipt_id, ''),
	title,
	objective,
	kind,
	parent_plan_key,
	status,
	stage_spec_outline,
	stage_refined_spec,
	stage_implementation_plan,
	in_scope,
	out_of_scope,
	constraints_list,
	references_list,
	external_refs,
	updated_at
FROM acm_work_plans
WHERE project_id = $1
	AND plan_key = $2
`, projectID, planKey)

	var (
		receiptID           string
		title               string
		objective           string
		kind                string
		parentPlanKey       string
		status              string
		stageSpecOutline    string
		stageRefinedSpec    string
		stageImplementation string
		inScope             []string
		outOfScope          []string
		constraints         []string
		references          []string
		externalRefs        []string
		updatedAt           time.Time
	)
	if err := row.Scan(&receiptID, &title, &objective, &kind, &parentPlanKey, &status, &stageSpecOutline, &stageRefinedSpec, &stageImplementation, &inScope, &outOfScope, &constraints, &references, &externalRefs, &updatedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return core.WorkPlan{}, false, nil
		}
		return core.WorkPlan{}, false, fmt.Errorf("query work plan: %w", err)
	}

	return core.WorkPlan{
		ProjectID:     projectID,
		PlanKey:       planKey,
		ReceiptID:     strings.TrimSpace(receiptID),
		Title:         strings.TrimSpace(title),
		Objective:     strings.TrimSpace(objective),
		Kind:          strings.TrimSpace(kind),
		ParentPlanKey: strings.TrimSpace(parentPlanKey),
		Status:        normalizeWorkItemStatus(status),
		Stages:        normalizeWorkPlanStages(core.WorkPlanStages{SpecOutline: stageSpecOutline, RefinedSpec: stageRefinedSpec, ImplementationPlan: stageImplementation}),
		InScope:       normalizeStringList(inScope),
		OutOfScope:    normalizeStringList(outOfScope),
		Constraints:   normalizeStringList(constraints),
		References:    normalizeStringList(references),
		ExternalRefs:  normalizeStringList(externalRefs),
		UpdatedAt:     updatedAt.UTC(),
	}, true, nil
}

type pgxRowsQuerier interface {
	QueryRow(context.Context, string, ...any) pgx.Row
	Query(context.Context, string, ...any) (pgx.Rows, error)
}

func listWorkPlanTasks(ctx context.Context, q pgxRowsQuerier, projectID, planKey string) ([]core.WorkItem, error) {
	rows, err := q.Query(ctx, `
SELECT
	task_key,
	summary,
	status,
	parent_task_key,
	depends_on,
	acceptance_criteria,
	references_list,
	external_refs,
	blocked_reason,
	outcome,
	evidence,
	updated_at
FROM acm_work_plan_tasks
WHERE project_id = $1
	AND plan_key = $2
ORDER BY task_key ASC
`, projectID, planKey)
	if err != nil {
		return nil, fmt.Errorf("query work plan tasks: %w", err)
	}
	defer rows.Close()

	items := make([]core.WorkItem, 0)
	for rows.Next() {
		var item core.WorkItem
		if err := rows.Scan(&item.ItemKey, &item.Summary, &item.Status, &item.ParentTaskKey, &item.DependsOn, &item.AcceptanceCriteria, &item.References, &item.ExternalRefs, &item.BlockedReason, &item.Outcome, &item.Evidence, &item.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan work plan task: %w", err)
		}
		item.ItemKey = strings.TrimSpace(item.ItemKey)
		if item.ItemKey == "" {
			continue
		}
		item.Summary = strings.TrimSpace(item.Summary)
		item.Status = normalizeWorkItemStatus(item.Status)
		item.ParentTaskKey = strings.TrimSpace(item.ParentTaskKey)
		item.DependsOn = normalizeStringList(item.DependsOn)
		item.AcceptanceCriteria = normalizeStringList(item.AcceptanceCriteria)
		item.References = normalizeStringList(item.References)
		item.ExternalRefs = normalizeStringList(item.ExternalRefs)
		item.BlockedReason = strings.TrimSpace(item.BlockedReason)
		item.Outcome = strings.TrimSpace(item.Outcome)
		item.Evidence = normalizeStringList(item.Evidence)
		item.UpdatedAt = item.UpdatedAt.UTC()
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate work plan tasks: %w", err)
	}
	return items, nil
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

type normalizedVerificationBatch struct {
	BatchRunID      string
	ProjectID       string
	ReceiptID       string
	PlanKey         string
	Phase           string
	TestsSourcePath string
	Status          string
	Passed          bool
	SelectedTestIDs []string
	Results         []core.VerificationTestRun
	CreatedAt       time.Time
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

func normalizeReceiptScope(input core.ReceiptScope) (normalizedRunSummary, error) {
	projectID := strings.TrimSpace(input.ProjectID)
	if projectID == "" {
		return normalizedRunSummary{}, fmt.Errorf("project_id is required")
	}

	receiptID := strings.TrimSpace(input.ReceiptID)
	if receiptID == "" {
		return normalizedRunSummary{}, fmt.Errorf("receipt_id is required")
	}

	phase := strings.TrimSpace(input.Phase)
	if phase == "" {
		phase = "execute"
	}

	return normalizedRunSummary{
		ProjectID:    projectID,
		ReceiptID:    receiptID,
		TaskText:     strings.TrimSpace(input.TaskText),
		Phase:        phase,
		ResolvedTags: normalizeStringList(input.ResolvedTags),
		PointerKeys:  normalizeStringList(input.PointerKeys),
		MemoryIDs:    normalizeInt64List(input.MemoryIDs),
	}, nil
}

func normalizeVerificationBatch(input core.VerificationBatch) (normalizedVerificationBatch, error) {
	projectID := strings.TrimSpace(input.ProjectID)
	if projectID == "" {
		return normalizedVerificationBatch{}, fmt.Errorf("project_id is required")
	}
	batchRunID := strings.TrimSpace(input.BatchRunID)
	if batchRunID == "" {
		return normalizedVerificationBatch{}, fmt.Errorf("batch_run_id is required")
	}
	status := strings.TrimSpace(input.Status)
	if status != "passed" && status != "failed" {
		return normalizedVerificationBatch{}, fmt.Errorf("status must be passed|failed")
	}

	results := make([]core.VerificationTestRun, 0, len(input.Results))
	for i, raw := range input.Results {
		testID := strings.TrimSpace(raw.TestID)
		if testID == "" {
			return normalizedVerificationBatch{}, fmt.Errorf("results[%d].test_id is required", i)
		}
		definitionHash := strings.TrimSpace(raw.DefinitionHash)
		if definitionHash == "" {
			return normalizedVerificationBatch{}, fmt.Errorf("results[%d].definition_hash is required", i)
		}
		resultStatus := strings.TrimSpace(raw.Status)
		switch resultStatus {
		case "passed", "failed", "timed_out", "errored", "skipped":
		default:
			return normalizedVerificationBatch{}, fmt.Errorf("results[%d].status is invalid", i)
		}
		timeoutSec := raw.TimeoutSec
		if timeoutSec <= 0 {
			return normalizedVerificationBatch{}, fmt.Errorf("results[%d].timeout_sec must be > 0", i)
		}
		expectedExitCode := raw.ExpectedExitCode
		if expectedExitCode < 0 || expectedExitCode > 255 {
			return normalizedVerificationBatch{}, fmt.Errorf("results[%d].expected_exit_code must be 0..255", i)
		}
		durationMS := raw.DurationMS
		if durationMS < 0 {
			durationMS = 0
		}
		startedAt := raw.StartedAt.UTC()
		if startedAt.IsZero() {
			startedAt = time.Now().UTC()
		}
		finishedAt := raw.FinishedAt.UTC()
		if finishedAt.IsZero() || finishedAt.Before(startedAt) {
			finishedAt = startedAt
		}
		results = append(results, core.VerificationTestRun{
			BatchRunID:       batchRunID,
			ProjectID:        projectID,
			TestID:           testID,
			DefinitionHash:   definitionHash,
			Summary:          strings.TrimSpace(raw.Summary),
			CommandArgv:      normalizeStringListPreserveOrder(raw.CommandArgv),
			CommandCWD:       strings.TrimSpace(raw.CommandCWD),
			TimeoutSec:       timeoutSec,
			ExpectedExitCode: expectedExitCode,
			SelectionReasons: normalizeStringListPreserveOrder(raw.SelectionReasons),
			Status:           resultStatus,
			ExitCode:         raw.ExitCode,
			DurationMS:       durationMS,
			StdoutExcerpt:    strings.TrimSpace(raw.StdoutExcerpt),
			StderrExcerpt:    strings.TrimSpace(raw.StderrExcerpt),
			StartedAt:        startedAt,
			FinishedAt:       finishedAt,
		})
	}

	createdAt := input.CreatedAt.UTC()
	if createdAt.IsZero() {
		createdAt = time.Now().UTC()
	}

	return normalizedVerificationBatch{
		BatchRunID:      batchRunID,
		ProjectID:       projectID,
		ReceiptID:       strings.TrimSpace(input.ReceiptID),
		PlanKey:         strings.TrimSpace(input.PlanKey),
		Phase:           strings.TrimSpace(input.Phase),
		TestsSourcePath: strings.TrimSpace(input.TestsSourcePath),
		Status:          status,
		Passed:          input.Passed,
		SelectedTestIDs: normalizeStringListPreserveOrder(input.SelectedTestIDs),
		Results:         results,
		CreatedAt:       createdAt,
	}, nil
}

func normalizeStringListPreserveOrder(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	out := make([]string, 0, len(values))
	for _, raw := range values {
		trimmed := strings.TrimSpace(raw)
		if trimmed == "" {
			continue
		}
		out = append(out, trimmed)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func nonNilStringListPreserveOrder(values []string) []string {
	if len(values) == 0 {
		return []string{}
	}
	return values
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

func workPlanSummaryKeys(rows []core.WorkPlanSummary) []string {
	if len(rows) == 0 {
		return nil
	}

	keys := make([]string, 0, len(rows))
	for _, row := range rows {
		planKey := strings.TrimSpace(row.PlanKey)
		if planKey == "" {
			continue
		}
		keys = append(keys, planKey)
	}
	return keys
}
