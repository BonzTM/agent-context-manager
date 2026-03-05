package sqlite

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"

	_ "modernc.org/sqlite"

	"github.com/joshd/agents-context/internal/core"
)

const (
	defaultCandidateLimit = 32
	defaultHopLimit       = 64
	defaultMemoryLimit    = 16
	maxQueryLimit         = 512
	maxHopDepth           = 6
	defaultPhase          = "execute"

	candidateStatusPending  = "pending"
	candidateStatusPromoted = "promoted"
	candidateStatusRejected = "rejected"

	workItemStatusPending    = core.WorkItemStatusPending
	workItemStatusInProgress = core.WorkItemStatusInProgress
	workItemStatusBlocked    = core.WorkItemStatusBlocked
	workItemStatusComplete   = core.WorkItemStatusComplete
	workItemStatusCompleted  = core.WorkItemStatusCompleted
)

type Repository struct {
	db *sql.DB
}

var _ core.Repository = (*Repository)(nil)

func New(ctx context.Context, cfg Config) (*Repository, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	dbPath := cfg.NormalizedPath()
	if err := ensureParentDirectory(dbPath); err != nil {
		return nil, err
	}

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("open sqlite db: %w", err)
	}

	if _, err := db.ExecContext(ctx, `PRAGMA foreign_keys = ON`); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("set sqlite foreign_keys pragma: %w", err)
	}
	if _, err := db.ExecContext(ctx, `PRAGMA busy_timeout = 5000`); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("set sqlite busy_timeout pragma: %w", err)
	}
	if _, err := db.ExecContext(ctx, `PRAGMA journal_mode = WAL`); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("set sqlite journal_mode pragma: %w", err)
	}
	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("ping sqlite db: %w", err)
	}
	if err := applyMigrations(ctx, db); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("apply sqlite migrations: %w", err)
	}

	return &Repository{db: db}, nil
}

func (r *Repository) Close() error {
	if r == nil || r.db == nil {
		return nil
	}
	return r.db.Close()
}

func (r *Repository) FetchCandidatePointers(_ context.Context, input core.CandidatePointerQuery) ([]core.CandidatePointer, error) {
	if r == nil || r.db == nil {
		return nil, fmt.Errorf("sqlite db is required")
	}

	projectID := strings.TrimSpace(input.ProjectID)
	if projectID == "" {
		return nil, fmt.Errorf("project_id is required")
	}

	phase := normalizePhase(input.Phase)
	limit := normalizeLimit(input.Limit, defaultCandidateLimit)
	queryTags := normalizeStringList(input.Tags)
	taskTokens := tokenize(strings.TrimSpace(input.TaskText))
	input.StaleFilter.StaleBefore = normalizeStaleBefore(input.StaleFilter.StaleBefore)

	rows, err := r.db.Query(`
SELECT
	pointer_key,
	path,
	anchor,
	kind,
	label,
	description,
	tags_json,
	is_rule,
	is_stale,
	stale_at,
	updated_at
FROM ctx_pointers
WHERE project_id = ?
`, projectID)
	if err != nil {
		return nil, fmt.Errorf("query candidate pointers: %w", err)
	}
	defer rows.Close()

	candidates := make([]core.CandidatePointer, 0)
	for rows.Next() {
		var (
			tagsJSON   string
			isRuleInt  int64
			isStaleInt int64
			staleAtSec sql.NullInt64
			updatedAt  int64
			row        core.CandidatePointer
		)
		if err := rows.Scan(
			&row.Key,
			&row.Path,
			&row.Anchor,
			&row.Kind,
			&row.Label,
			&row.Description,
			&tagsJSON,
			&isRuleInt,
			&isStaleInt,
			&staleAtSec,
			&updatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan candidate pointer: %w", err)
		}

		tags, err := decodeStringList(tagsJSON)
		if err != nil {
			return nil, fmt.Errorf("decode candidate pointer tags: %w", err)
		}

		row.Tags = tags
		row.IsRule = isRuleInt != 0
		row.IsStale = isStaleInt != 0
		row.UpdatedAt = unixTime(updatedAt)

		var staleAt *time.Time
		if staleAtSec.Valid {
			t := unixTime(staleAtSec.Int64)
			staleAt = &t
		}
		if !matchesStaleFilter(row.IsStale, staleAt, input.StaleFilter) {
			continue
		}

		textRank := textMatchRank(taskTokens, row)
		tagOverlap := overlapCount(row.Tags, queryTags)
		if len(taskTokens) > 0 || len(queryTags) > 0 {
			if textRank == 0 && tagOverlap == 0 {
				continue
			}
		}

		weight := phaseWeight(phase, row)
		row.Rank = ((float64(tagOverlap) * 10.0) + (float64(textRank) * 5.0)) * weight
		candidates = append(candidates, row)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate candidate pointers: %w", err)
	}

	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].IsRule != candidates[j].IsRule {
			return candidates[i].IsRule
		}
		if candidates[i].Rank != candidates[j].Rank {
			return candidates[i].Rank > candidates[j].Rank
		}
		return candidates[i].Key < candidates[j].Key
	})
	if len(candidates) > limit {
		candidates = candidates[:limit]
	}
	return candidates, nil
}

func (r *Repository) FetchRelatedHopPointers(_ context.Context, input core.RelatedHopPointersQuery) ([]core.HopPointer, error) {
	if r == nil || r.db == nil {
		return nil, fmt.Errorf("sqlite db is required")
	}

	projectID := strings.TrimSpace(input.ProjectID)
	if projectID == "" {
		return nil, fmt.Errorf("project_id is required")
	}
	origins := normalizeStringList(input.PointerKeys)
	if len(origins) == 0 {
		return nil, fmt.Errorf("pointer_keys is required")
	}

	maxHops := input.MaxHops
	if maxHops <= 0 {
		maxHops = 1
	}
	if maxHops > maxHopDepth {
		maxHops = maxHopDepth
	}
	limit := normalizeLimit(input.Limit, defaultHopLimit)
	input.StaleFilter.StaleBefore = normalizeStaleBefore(input.StaleFilter.StaleBefore)

	type link struct {
		From string
		To   string
	}
	linkRows, err := r.db.Query(`
SELECT from_key, to_key
FROM ctx_pointer_links
WHERE project_id = ?
`, projectID)
	if err != nil {
		return nil, fmt.Errorf("query pointer links: %w", err)
	}
	defer linkRows.Close()

	adjacency := make(map[string][]string)
	for linkRows.Next() {
		var current link
		if err := linkRows.Scan(&current.From, &current.To); err != nil {
			return nil, fmt.Errorf("scan pointer link: %w", err)
		}
		adjacency[current.From] = append(adjacency[current.From], current.To)
	}
	if err := linkRows.Err(); err != nil {
		return nil, fmt.Errorf("iterate pointer links: %w", err)
	}
	for key := range adjacency {
		sort.Strings(adjacency[key])
	}

	type bfsNode struct {
		Key string
		Hop int
	}
	type hopKey struct {
		Origin string
		Target string
	}
	bestHopByPair := make(map[hopKey]int)
	targetKeys := make(map[string]struct{})

	for _, origin := range origins {
		visited := map[string]struct{}{origin: {}}
		queue := []bfsNode{{Key: origin, Hop: 0}}

		for len(queue) > 0 {
			node := queue[0]
			queue = queue[1:]
			if node.Hop >= maxHops {
				continue
			}

			for _, next := range adjacency[node.Key] {
				if _, seen := visited[next]; seen {
					continue
				}
				visited[next] = struct{}{}

				nextHop := node.Hop + 1
				pair := hopKey{Origin: origin, Target: next}
				if current, exists := bestHopByPair[pair]; !exists || nextHop < current {
					bestHopByPair[pair] = nextHop
				}
				targetKeys[next] = struct{}{}
				queue = append(queue, bfsNode{Key: next, Hop: nextHop})
			}
		}
	}

	if len(targetKeys) == 0 {
		return nil, nil
	}

	targetList := mapKeysSorted(targetKeys)
	pointerByKey, err := r.loadPointersByKey(projectID, targetList)
	if err != nil {
		return nil, err
	}

	hops := make([]core.HopPointer, 0, len(bestHopByPair))
	for pair, hopCount := range bestHopByPair {
		pointer, ok := pointerByKey[pair.Target]
		if !ok {
			continue
		}
		var staleAt *time.Time
		if !pointer.staleAt.IsZero() {
			t := pointer.staleAt
			staleAt = &t
		}
		if !matchesStaleFilter(pointer.IsStale, staleAt, input.StaleFilter) {
			continue
		}
		hops = append(hops, core.HopPointer{
			SourceKey: pair.Origin,
			HopCount:  hopCount,
			Pointer: core.CandidatePointer{
				Key:         pointer.Key,
				Path:        pointer.Path,
				Anchor:      pointer.Anchor,
				Kind:        pointer.Kind,
				Label:       pointer.Label,
				Description: pointer.Description,
				Tags:        append([]string(nil), pointer.Tags...),
				IsRule:      pointer.IsRule,
				IsStale:     pointer.IsStale,
				UpdatedAt:   pointer.UpdatedAt,
			},
		})
	}

	sort.Slice(hops, func(i, j int) bool {
		if hops[i].HopCount != hops[j].HopCount {
			return hops[i].HopCount < hops[j].HopCount
		}
		if hops[i].SourceKey != hops[j].SourceKey {
			return hops[i].SourceKey < hops[j].SourceKey
		}
		return hops[i].Pointer.Key < hops[j].Pointer.Key
	})
	if len(hops) > limit {
		hops = hops[:limit]
	}
	return hops, nil
}

func (r *Repository) FetchActiveMemories(_ context.Context, input core.ActiveMemoryQuery) ([]core.ActiveMemory, error) {
	if r == nil || r.db == nil {
		return nil, fmt.Errorf("sqlite db is required")
	}

	projectID := strings.TrimSpace(input.ProjectID)
	if projectID == "" {
		return nil, fmt.Errorf("project_id is required")
	}
	limit := normalizeLimit(input.Limit, defaultMemoryLimit)
	queryPointerKeys := normalizeStringList(input.PointerKeys)
	queryTags := normalizeStringList(input.Tags)

	rows, err := r.db.Query(`
SELECT
	memory_id,
	category,
	subject,
	content,
	confidence,
	tags_json,
	related_pointer_keys_json,
	updated_at
FROM ctx_memories
WHERE project_id = ?
	AND active = 1
`, projectID)
	if err != nil {
		return nil, fmt.Errorf("query active memories: %w", err)
	}
	defer rows.Close()

	memories := make([]core.ActiveMemory, 0)
	for rows.Next() {
		var (
			tagsJSON    string
			relatedJSON string
			updatedAt   int64
			row         core.ActiveMemory
		)
		if err := rows.Scan(
			&row.ID,
			&row.Category,
			&row.Subject,
			&row.Content,
			&row.Confidence,
			&tagsJSON,
			&relatedJSON,
			&updatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan active memory: %w", err)
		}

		tags, err := decodeStringList(tagsJSON)
		if err != nil {
			return nil, fmt.Errorf("decode memory tags: %w", err)
		}
		related, err := decodeStringList(relatedJSON)
		if err != nil {
			return nil, fmt.Errorf("decode memory related pointers: %w", err)
		}

		if len(queryPointerKeys) > 0 || len(queryTags) > 0 {
			if overlapCount(related, queryPointerKeys) == 0 && overlapCount(tags, queryTags) == 0 {
				continue
			}
		}

		row.Tags = tags
		row.RelatedPointerKeys = related
		row.UpdatedAt = unixTime(updatedAt)
		memories = append(memories, row)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate active memories: %w", err)
	}

	sort.Slice(memories, func(i, j int) bool {
		if memories[i].Confidence != memories[j].Confidence {
			return memories[i].Confidence > memories[j].Confidence
		}
		if !memories[i].UpdatedAt.Equal(memories[j].UpdatedAt) {
			return memories[i].UpdatedAt.After(memories[j].UpdatedAt)
		}
		return memories[i].ID < memories[j].ID
	})
	if len(memories) > limit {
		memories = memories[:limit]
	}
	return memories, nil
}

func (r *Repository) ListPointerInventory(ctx context.Context, projectID string) ([]core.PointerInventory, error) {
	if r == nil || r.db == nil {
		return nil, fmt.Errorf("sqlite db is required")
	}

	projectID = strings.TrimSpace(projectID)
	if projectID == "" {
		return nil, fmt.Errorf("project_id is required")
	}

	rows, err := r.db.QueryContext(ctx, `
SELECT
	path,
	MAX(is_stale) AS is_stale
FROM ctx_pointers
WHERE project_id = ?
GROUP BY path
ORDER BY path ASC
`, projectID)
	if err != nil {
		return nil, fmt.Errorf("query pointer inventory: %w", err)
	}
	defer rows.Close()

	results := make([]core.PointerInventory, 0)
	for rows.Next() {
		var (
			pathValue  string
			isStaleInt int64
		)
		if err := rows.Scan(&pathValue, &isStaleInt); err != nil {
			return nil, fmt.Errorf("scan pointer inventory: %w", err)
		}
		normalizedPath := normalizeSyncPath(pathValue)
		if normalizedPath == "" {
			continue
		}
		results = append(results, core.PointerInventory{
			Path:    normalizedPath,
			IsStale: isStaleInt != 0,
		})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate pointer inventory: %w", err)
	}
	return results, nil
}

func (r *Repository) UpsertPointerStubs(ctx context.Context, projectID string, stubs []core.PointerStub) (int, error) {
	if r == nil || r.db == nil {
		return 0, fmt.Errorf("sqlite db is required")
	}

	normalized, err := normalizePointerStubs(projectID, stubs)
	if err != nil {
		return 0, err
	}
	if len(normalized) == 0 {
		return 0, nil
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	updated := 0
	for _, stub := range normalized {
		tagsJSON, encodeErr := encodeStringList(stub.Tags)
		if encodeErr != nil {
			return 0, fmt.Errorf("encode pointer stub tags: %w", encodeErr)
		}

		isRule := boolToInt(strings.EqualFold(stub.Kind, "rule"))
		tag, execErr := tx.ExecContext(ctx, `
INSERT INTO ctx_pointers (
	project_id,
	pointer_key,
	path,
	anchor,
	kind,
	label,
	description,
	tags_json,
	is_rule,
	is_stale,
	stale_at,
	content_hash,
	updated_at
)
VALUES (?, ?, ?, '', ?, ?, ?, ?, ?, 0, NULL, NULL, unixepoch())
ON CONFLICT(project_id, pointer_key) DO UPDATE SET
	path = excluded.path,
	anchor = excluded.anchor,
	is_stale = 0,
	stale_at = NULL,
	updated_at = unixepoch()
`,
			strings.TrimSpace(projectID),
			stub.PointerKey,
			stub.Path,
			stub.Kind,
			stub.Label,
			stub.Description,
			tagsJSON,
			isRule,
		)
		if execErr != nil {
			return 0, fmt.Errorf("upsert pointer stubs: %w", execErr)
		}
		rowsAffected, rowsErr := tag.RowsAffected()
		if rowsErr != nil {
			return 0, fmt.Errorf("read pointer stub rows affected: %w", rowsErr)
		}
		updated += int(rowsAffected)
	}

	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("commit tx: %w", err)
	}
	return updated, nil
}

func (r *Repository) FetchReceiptScope(ctx context.Context, input core.ReceiptScopeQuery) (core.ReceiptScope, error) {
	if r == nil || r.db == nil {
		return core.ReceiptScope{}, fmt.Errorf("sqlite db is required")
	}

	projectID := strings.TrimSpace(input.ProjectID)
	if projectID == "" {
		return core.ReceiptScope{}, fmt.Errorf("project_id is required")
	}
	receiptID := strings.TrimSpace(input.ReceiptID)
	if receiptID == "" {
		return core.ReceiptScope{}, fmt.Errorf("receipt_id is required")
	}

	var (
		taskText         string
		phase            string
		resolvedTagsJSON string
		pointerKeysJSON  string
		memoryIDsJSON    string
	)
	err := r.db.QueryRowContext(
		ctx,
		`
SELECT task_text, phase, resolved_tags_json, pointer_keys_json, memory_ids_json
FROM ctx_receipts
WHERE project_id = ?
	AND receipt_id = ?
`,
		projectID,
		receiptID,
	).Scan(&taskText, &phase, &resolvedTagsJSON, &pointerKeysJSON, &memoryIDsJSON)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return core.ReceiptScope{}, core.ErrReceiptScopeNotFound
		}
		return core.ReceiptScope{}, fmt.Errorf("query receipt scope: %w", err)
	}

	resolvedTags, err := decodeStringList(resolvedTagsJSON)
	if err != nil {
		return core.ReceiptScope{}, fmt.Errorf("decode resolved_tags: %w", err)
	}
	pointerKeys, err := decodeStringList(pointerKeysJSON)
	if err != nil {
		return core.ReceiptScope{}, fmt.Errorf("decode pointer_keys: %w", err)
	}
	memoryIDs, err := decodeInt64List(memoryIDsJSON)
	if err != nil {
		return core.ReceiptScope{}, fmt.Errorf("decode memory_ids: %w", err)
	}
	pointerPaths, err := r.pointerPathsByKeys(ctx, projectID, pointerKeys)
	if err != nil {
		return core.ReceiptScope{}, err
	}

	return core.ReceiptScope{
		ProjectID:    projectID,
		ReceiptID:    receiptID,
		TaskText:     strings.TrimSpace(taskText),
		Phase:        strings.TrimSpace(phase),
		ResolvedTags: resolvedTags,
		PointerKeys:  pointerKeys,
		MemoryIDs:    memoryIDs,
		PointerPaths: pointerPaths,
	}, nil
}

func (r *Repository) LookupFetchState(ctx context.Context, input core.FetchLookupQuery) (core.FetchLookup, error) {
	if r == nil || r.db == nil {
		return core.FetchLookup{}, fmt.Errorf("sqlite db is required")
	}

	projectID := strings.TrimSpace(input.ProjectID)
	if projectID == "" {
		return core.FetchLookup{}, fmt.Errorf("project_id is required")
	}
	receiptID := strings.TrimSpace(input.ReceiptID)
	if receiptID == "" {
		return core.FetchLookup{}, fmt.Errorf("receipt_id is required")
	}

	var (
		lookupReceiptID string
		runID           int64
		runStatus       sql.NullString
		updatedAt       int64
	)
	err := r.db.QueryRowContext(ctx, `
SELECT
	r.receipt_id,
	COALESCE(run.run_id, 0) AS run_id,
	COALESCE(run.status, '') AS run_status,
	COALESCE(run.created_at, r.created_at) AS updated_at
FROM ctx_receipts r
LEFT JOIN (
	SELECT run_id, status, created_at
	FROM ctx_runs
	WHERE project_id = ?
		AND receipt_id = ?
	ORDER BY created_at DESC, run_id DESC
	LIMIT 1
) run ON 1 = 1
WHERE r.project_id = ?
	AND r.receipt_id = ?
`, projectID, receiptID, projectID, receiptID).Scan(&lookupReceiptID, &runID, &runStatus, &updatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return core.FetchLookup{}, core.ErrFetchLookupNotFound
		}
		return core.FetchLookup{}, fmt.Errorf("query fetch lookup: %w", err)
	}

	workItems, err := r.ListWorkItems(ctx, core.FetchLookupQuery{
		ProjectID: projectID,
		ReceiptID: receiptID,
	})
	if err != nil {
		return core.FetchLookup{}, err
	}

	return core.FetchLookup{
		ProjectID:  projectID,
		ReceiptID:  lookupReceiptID,
		RunID:      runID,
		RunStatus:  strings.TrimSpace(runStatus.String),
		PlanStatus: derivePlanStatus(workItems),
		WorkItems:  workItems,
		UpdatedAt:  unixTime(updatedAt),
	}, nil
}

func (r *Repository) LookupPointerByKey(ctx context.Context, input core.PointerLookupQuery) (core.CandidatePointer, error) {
	if r == nil || r.db == nil {
		return core.CandidatePointer{}, fmt.Errorf("sqlite db is required")
	}

	projectID := strings.TrimSpace(input.ProjectID)
	if projectID == "" {
		return core.CandidatePointer{}, fmt.Errorf("project_id is required")
	}
	pointerKey := strings.TrimSpace(input.PointerKey)
	if pointerKey == "" {
		return core.CandidatePointer{}, fmt.Errorf("pointer_key is required")
	}

	var (
		tagsJSON   string
		isRuleInt  int64
		isStaleInt int64
		updatedAt  int64
		row        core.CandidatePointer
	)
	err := r.db.QueryRowContext(ctx, `
SELECT
	pointer_key,
	path,
	anchor,
	kind,
	label,
	description,
	tags_json,
	is_rule,
	is_stale,
	updated_at
FROM ctx_pointers
WHERE project_id = ?
	AND pointer_key = ?
`, projectID, pointerKey).Scan(
		&row.Key,
		&row.Path,
		&row.Anchor,
		&row.Kind,
		&row.Label,
		&row.Description,
		&tagsJSON,
		&isRuleInt,
		&isStaleInt,
		&updatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return core.CandidatePointer{}, core.ErrPointerLookupNotFound
		}
		return core.CandidatePointer{}, fmt.Errorf("query pointer lookup: %w", err)
	}

	tags, err := decodeStringList(tagsJSON)
	if err != nil {
		return core.CandidatePointer{}, fmt.Errorf("decode pointer tags: %w", err)
	}

	return core.CandidatePointer{
		Key:         strings.TrimSpace(row.Key),
		Path:        normalizeSyncPath(row.Path),
		Anchor:      strings.TrimSpace(row.Anchor),
		Kind:        strings.TrimSpace(row.Kind),
		Label:       strings.TrimSpace(row.Label),
		Description: strings.TrimSpace(row.Description),
		Tags:        normalizeStringList(tags),
		IsRule:      isRuleInt != 0,
		IsStale:     isStaleInt != 0,
		UpdatedAt:   unixTime(updatedAt),
	}, nil
}

func (r *Repository) LookupMemoryByID(ctx context.Context, input core.MemoryLookupQuery) (core.ActiveMemory, error) {
	if r == nil || r.db == nil {
		return core.ActiveMemory{}, fmt.Errorf("sqlite db is required")
	}

	projectID := strings.TrimSpace(input.ProjectID)
	if projectID == "" {
		return core.ActiveMemory{}, fmt.Errorf("project_id is required")
	}
	if input.MemoryID <= 0 {
		return core.ActiveMemory{}, fmt.Errorf("memory_id must be positive")
	}

	var (
		tagsJSON    string
		relatedJSON string
		updatedAt   int64
		row         core.ActiveMemory
	)
	err := r.db.QueryRowContext(ctx, `
SELECT
	memory_id,
	category,
	subject,
	content,
	confidence,
	tags_json,
	related_pointer_keys_json,
	updated_at
FROM ctx_memories
WHERE project_id = ?
	AND memory_id = ?
`, projectID, input.MemoryID).Scan(
		&row.ID,
		&row.Category,
		&row.Subject,
		&row.Content,
		&row.Confidence,
		&tagsJSON,
		&relatedJSON,
		&updatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return core.ActiveMemory{}, core.ErrMemoryLookupNotFound
		}
		return core.ActiveMemory{}, fmt.Errorf("query memory lookup: %w", err)
	}

	tags, err := decodeStringList(tagsJSON)
	if err != nil {
		return core.ActiveMemory{}, fmt.Errorf("decode memory tags: %w", err)
	}
	relatedPointerKeys, err := decodeStringList(relatedJSON)
	if err != nil {
		return core.ActiveMemory{}, fmt.Errorf("decode memory related pointers: %w", err)
	}

	return core.ActiveMemory{
		ID:                 row.ID,
		Category:           strings.TrimSpace(row.Category),
		Subject:            strings.TrimSpace(row.Subject),
		Content:            strings.TrimSpace(row.Content),
		Confidence:         row.Confidence,
		Tags:               normalizeStringList(tags),
		RelatedPointerKeys: normalizeStringList(relatedPointerKeys),
		UpdatedAt:          unixTime(updatedAt),
	}, nil
}

func (r *Repository) UpsertWorkItems(ctx context.Context, input core.WorkItemsUpsertInput) (int, error) {
	if r == nil || r.db == nil {
		return 0, fmt.Errorf("sqlite db is required")
	}

	projectID := strings.TrimSpace(input.ProjectID)
	if projectID == "" {
		return 0, fmt.Errorf("project_id is required")
	}
	receiptID := strings.TrimSpace(input.ReceiptID)
	if receiptID == "" {
		return 0, fmt.Errorf("receipt_id is required")
	}

	items, err := normalizeWorkItems(input.Items)
	if err != nil {
		return 0, err
	}
	if len(items) == 0 {
		return 0, fmt.Errorf("work items are required")
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	updated := 0
	for _, item := range items {
		tag, execErr := tx.ExecContext(ctx, `
INSERT INTO ctx_work_items (
	project_id,
	receipt_id,
	item_key,
	status,
	created_at,
	updated_at
) VALUES (?, ?, ?, ?, unixepoch(), unixepoch())
ON CONFLICT(project_id, receipt_id, item_key) DO UPDATE SET
	status = excluded.status,
	updated_at = unixepoch()
`, projectID, receiptID, item.ItemKey, storageWorkItemStatus(item.Status))
		if execErr != nil {
			return 0, fmt.Errorf("upsert work item: %w", execErr)
		}
		rowsAffected, rowsErr := tag.RowsAffected()
		if rowsErr != nil {
			return 0, fmt.Errorf("read work item rows affected: %w", rowsErr)
		}
		updated += int(rowsAffected)
	}

	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("commit tx: %w", err)
	}

	return updated, nil
}

func (r *Repository) ListWorkItems(ctx context.Context, input core.FetchLookupQuery) ([]core.WorkItem, error) {
	if r == nil || r.db == nil {
		return nil, fmt.Errorf("sqlite db is required")
	}

	projectID := strings.TrimSpace(input.ProjectID)
	if projectID == "" {
		return nil, fmt.Errorf("project_id is required")
	}
	receiptID := strings.TrimSpace(input.ReceiptID)
	if receiptID == "" {
		return nil, fmt.Errorf("receipt_id is required")
	}

	rows, err := r.db.QueryContext(ctx, `
SELECT item_key, status, updated_at
FROM ctx_work_items
WHERE project_id = ?
	AND receipt_id = ?
ORDER BY item_key ASC
`, projectID, receiptID)
	if err != nil {
		return nil, fmt.Errorf("query work items: %w", err)
	}
	defer rows.Close()

	items := make([]core.WorkItem, 0)
	for rows.Next() {
		var (
			itemKey   string
			status    string
			updatedAt int64
		)
		if err := rows.Scan(&itemKey, &status, &updatedAt); err != nil {
			return nil, fmt.Errorf("scan work item: %w", err)
		}

		normalizedKey := normalizeSyncPath(itemKey)
		if normalizedKey == "" {
			continue
		}

		items = append(items, core.WorkItem{
			ItemKey:   normalizedKey,
			Status:    normalizeWorkItemStatus(status),
			UpdatedAt: unixTime(updatedAt),
		})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate work items: %w", err)
	}

	return items, nil
}

func (r *Repository) PersistProposedMemory(ctx context.Context, input core.ProposeMemoryPersistence) (core.ProposeMemoryPersistenceResult, error) {
	if r == nil || r.db == nil {
		return core.ProposeMemoryPersistenceResult{}, fmt.Errorf("sqlite db is required")
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

	tagsJSON, err := encodeStringList(nonNilStringList(normalized.Tags))
	if err != nil {
		return core.ProposeMemoryPersistenceResult{}, err
	}
	relatedJSON, err := encodeStringList(nonNilStringList(normalized.RelatedPointerKeys))
	if err != nil {
		return core.ProposeMemoryPersistenceResult{}, err
	}
	evidenceJSON, err := encodeStringList(normalized.EvidencePointerKeys)
	if err != nil {
		return core.ProposeMemoryPersistenceResult{}, err
	}
	errorsJSON, err := encodeStringList(nonNilStringList(normalized.Validation.Errors))
	if err != nil {
		return core.ProposeMemoryPersistenceResult{}, err
	}
	warningsJSON, err := encodeStringList(nonNilStringList(normalized.Validation.Warnings))
	if err != nil {
		return core.ProposeMemoryPersistenceResult{}, err
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return core.ProposeMemoryPersistenceResult{}, fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	insertResult, err := tx.ExecContext(ctx, `
INSERT INTO ctx_memory_candidates (
	project_id,
	receipt_id,
	category,
	subject,
	content,
	confidence,
	tags_json,
	related_pointer_keys_json,
	evidence_pointer_keys_json,
	dedupe_key,
	status,
	hard_passed,
	soft_passed,
	validation_errors_json,
	validation_warnings_json,
	auto_promote,
	promotable,
	created_at,
	updated_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, unixepoch(), unixepoch())
`,
		normalized.ProjectID,
		normalized.ReceiptID,
		normalized.Category,
		normalized.Subject,
		normalized.Content,
		normalized.Confidence,
		tagsJSON,
		relatedJSON,
		evidenceJSON,
		normalized.DedupeKey,
		initialStatus,
		boolToInt(normalized.Validation.HardPassed),
		boolToInt(normalized.Validation.SoftPassed),
		errorsJSON,
		warningsJSON,
		boolToInt(normalized.AutoPromote),
		boolToInt(normalized.Promotable),
	)
	if err != nil {
		return core.ProposeMemoryPersistenceResult{}, fmt.Errorf("insert memory candidate: %w", err)
	}
	candidateID, err := insertResult.LastInsertId()
	if err != nil {
		return core.ProposeMemoryPersistenceResult{}, fmt.Errorf("read memory candidate id: %w", err)
	}

	out := core.ProposeMemoryPersistenceResult{
		CandidateID: candidateID,
		Status:      initialStatus,
	}
	if !normalized.Promotable {
		if err := tx.Commit(); err != nil {
			return core.ProposeMemoryPersistenceResult{}, fmt.Errorf("commit tx: %w", err)
		}
		return out, nil
	}

	insertMemoryResult, err := tx.ExecContext(ctx, `
INSERT OR IGNORE INTO ctx_memories (
	project_id,
	category,
	subject,
	content,
	confidence,
	tags_json,
	related_pointer_keys_json,
	evidence_pointer_keys_json,
	dedupe_key,
	active,
	created_at,
	updated_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, 1, unixepoch(), unixepoch())
`,
		normalized.ProjectID,
		normalized.Category,
		normalized.Subject,
		normalized.Content,
		normalized.Confidence,
		tagsJSON,
		relatedJSON,
		evidenceJSON,
		normalized.DedupeKey,
	)
	if err != nil {
		return core.ProposeMemoryPersistenceResult{}, fmt.Errorf("insert durable memory: %w", err)
	}
	insertedRows, err := insertMemoryResult.RowsAffected()
	if err != nil {
		return core.ProposeMemoryPersistenceResult{}, fmt.Errorf("read durable memory rows affected: %w", err)
	}

	finalStatus := candidateStatusRejected
	if insertedRows > 0 {
		promotedMemoryID, err := insertMemoryResult.LastInsertId()
		if err != nil {
			return core.ProposeMemoryPersistenceResult{}, fmt.Errorf("read durable memory id: %w", err)
		}
		out.PromotedMemoryID = promotedMemoryID
		finalStatus = candidateStatusPromoted
	}

	_, err = tx.ExecContext(ctx, `
UPDATE ctx_memory_candidates
SET
	status = ?,
	promoted_memory_id = CASE WHEN ? > 0 THEN ? ELSE NULL END,
	updated_at = unixepoch()
WHERE candidate_id = ?
`,
		finalStatus,
		out.PromotedMemoryID,
		out.PromotedMemoryID,
		candidateID,
	)
	if err != nil {
		return core.ProposeMemoryPersistenceResult{}, fmt.Errorf("update memory candidate status: %w", err)
	}

	out.Status = finalStatus
	if err := tx.Commit(); err != nil {
		return core.ProposeMemoryPersistenceResult{}, fmt.Errorf("commit tx: %w", err)
	}
	return out, nil
}

func (r *Repository) SaveRunReceiptSummary(ctx context.Context, input core.RunReceiptSummary) (core.RunReceiptIDs, error) {
	if r == nil || r.db == nil {
		return core.RunReceiptIDs{}, fmt.Errorf("sqlite db is required")
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

	resolvedTagsJSON, err := encodeStringList(nonNilStringList(normalized.ResolvedTags))
	if err != nil {
		return core.RunReceiptIDs{}, err
	}
	pointerKeysJSON, err := encodeStringList(nonNilStringList(normalized.PointerKeys))
	if err != nil {
		return core.RunReceiptIDs{}, err
	}
	memoryIDsJSON, err := encodeInt64List(nonNilInt64List(normalized.MemoryIDs))
	if err != nil {
		return core.RunReceiptIDs{}, err
	}
	filesChangedJSON, err := encodeStringList(nonNilStringList(normalized.FilesChanged))
	if err != nil {
		return core.RunReceiptIDs{}, err
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

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return core.RunReceiptIDs{}, fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	_, err = tx.ExecContext(ctx, `
INSERT INTO ctx_receipts (
	receipt_id,
	project_id,
	task_text,
	phase,
	resolved_tags_json,
	pointer_keys_json,
	memory_ids_json,
	summary_json,
	created_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, unixepoch())
ON CONFLICT(receipt_id) DO UPDATE
SET
	project_id = excluded.project_id,
	task_text = excluded.task_text,
	phase = excluded.phase,
	resolved_tags_json = excluded.resolved_tags_json,
	pointer_keys_json = excluded.pointer_keys_json,
	memory_ids_json = excluded.memory_ids_json,
	summary_json = excluded.summary_json
`,
		normalized.ReceiptID,
		normalized.ProjectID,
		normalized.TaskText,
		normalized.Phase,
		resolvedTagsJSON,
		pointerKeysJSON,
		memoryIDsJSON,
		string(receiptJSON),
	)
	if err != nil {
		return core.RunReceiptIDs{}, fmt.Errorf("upsert receipt summary: %w", err)
	}

	insertRunResult, err := tx.ExecContext(ctx, `
INSERT INTO ctx_runs (
	project_id,
	request_id,
	receipt_id,
	status,
	files_changed_json,
	outcome,
	summary_json,
	created_at
) VALUES (?, ?, ?, ?, ?, ?, ?, unixepoch())
`,
		normalized.ProjectID,
		normalized.RequestID,
		normalized.ReceiptID,
		normalized.Status,
		filesChangedJSON,
		normalized.Outcome,
		string(runJSON),
	)
	if err != nil {
		return core.RunReceiptIDs{}, fmt.Errorf("insert run summary: %w", err)
	}
	runID, err := insertRunResult.LastInsertId()
	if err != nil {
		return core.RunReceiptIDs{}, fmt.Errorf("read run id: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return core.RunReceiptIDs{}, fmt.Errorf("commit tx: %w", err)
	}
	return core.RunReceiptIDs{
		RunID:     runID,
		ReceiptID: normalized.ReceiptID,
	}, nil
}

func (r *Repository) ApplySync(ctx context.Context, input core.SyncApplyInput) (core.SyncApplyResult, error) {
	if r == nil || r.db == nil {
		return core.SyncApplyResult{}, fmt.Errorf("sqlite db is required")
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

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return core.SyncApplyResult{}, fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	out := core.SyncApplyResult{}
	if len(deletedPaths) > 0 {
		query := `
UPDATE ctx_pointers
SET
	is_stale = 1,
	stale_at = unixepoch(),
	updated_at = unixepoch()
WHERE project_id = ?
	AND is_stale = 0
	AND path IN (` + placeholders(len(deletedPaths)) + `)
`
		args := make([]any, 0, len(deletedPaths)+1)
		args = append(args, normalized.ProjectID)
		for _, p := range deletedPaths {
			args = append(args, p)
		}
		tag, err := tx.ExecContext(ctx, query, args...)
		if err != nil {
			return core.SyncApplyResult{}, fmt.Errorf("mark deleted pointers stale: %w", err)
		}
		rowsAffected, err := tag.RowsAffected()
		if err != nil {
			return core.SyncApplyResult{}, fmt.Errorf("read deleted stale rows affected: %w", err)
		}
		out.DeletedMarkedStale = int(rowsAffected)
	}

	if normalized.Mode == "full" {
		var (
			query string
			args  []any
		)
		if len(presentPaths) == 0 {
			query = `
UPDATE ctx_pointers
SET
	is_stale = 1,
	stale_at = unixepoch(),
	updated_at = unixepoch()
WHERE project_id = ?
	AND is_stale = 0
`
			args = []any{normalized.ProjectID}
		} else {
			query = `
UPDATE ctx_pointers
SET
	is_stale = 1,
	stale_at = unixepoch(),
	updated_at = unixepoch()
WHERE project_id = ?
	AND is_stale = 0
	AND path NOT IN (` + placeholders(len(presentPaths)) + `)
`
			args = make([]any, 0, len(presentPaths)+1)
			args = append(args, normalized.ProjectID)
			for _, p := range presentPaths {
				args = append(args, p)
			}
		}
		tag, err := tx.ExecContext(ctx, query, args...)
		if err != nil {
			return core.SyncApplyResult{}, fmt.Errorf("mark missing pointers stale: %w", err)
		}
		rowsAffected, err := tag.RowsAffected()
		if err != nil {
			return core.SyncApplyResult{}, fmt.Errorf("read missing stale rows affected: %w", err)
		}
		out.MarkedStale = int(rowsAffected)
	}

	for _, row := range presentRows {
		tag, err := tx.ExecContext(ctx, `
UPDATE ctx_pointers
SET
	content_hash = ?,
	is_stale = 0,
	stale_at = NULL,
	updated_at = unixepoch()
WHERE project_id = ?
	AND path = ?
	AND (is_stale = 1 OR IFNULL(content_hash, '') <> ?)
`,
			row.ContentHash,
			normalized.ProjectID,
			row.Path,
			row.ContentHash,
		)
		if err != nil {
			return core.SyncApplyResult{}, fmt.Errorf("refresh pointers: %w", err)
		}
		rowsAffected, err := tag.RowsAffected()
		if err != nil {
			return core.SyncApplyResult{}, fmt.Errorf("read refresh pointers rows affected: %w", err)
		}
		out.Updated += int(rowsAffected)
	}

	if normalized.InsertNewCandidates {
		for _, row := range presentRows {
			tag, err := tx.ExecContext(ctx, `
INSERT OR IGNORE INTO ctx_pointer_candidates (
	project_id,
	path,
	content_hash,
	created_at,
	updated_at,
	last_seen_at
) SELECT ?, ?, ?, unixepoch(), unixepoch(), unixepoch()
WHERE NOT EXISTS (
	SELECT 1
	FROM ctx_pointers
	WHERE project_id = ?
		AND path = ?
)
`,
				normalized.ProjectID,
				row.Path,
				row.ContentHash,
				normalized.ProjectID,
				row.Path,
			)
			if err != nil {
				return core.SyncApplyResult{}, fmt.Errorf("insert pointer candidates: %w", err)
			}
			rowsAffected, err := tag.RowsAffected()
			if err != nil {
				return core.SyncApplyResult{}, fmt.Errorf("read pointer candidate rows affected: %w", err)
			}
			out.NewCandidates += int(rowsAffected)
		}
	}

	if err := tx.Commit(); err != nil {
		return core.SyncApplyResult{}, fmt.Errorf("commit tx: %w", err)
	}
	return out, nil
}

type pointerRow struct {
	Key         string
	Path        string
	Anchor      string
	Kind        string
	Label       string
	Description string
	Tags        []string
	IsRule      bool
	IsStale     bool
	staleAt     time.Time
	UpdatedAt   time.Time
}

func (r *Repository) loadPointersByKey(projectID string, keys []string) (map[string]pointerRow, error) {
	if len(keys) == 0 {
		return map[string]pointerRow{}, nil
	}

	query := `
SELECT
	pointer_key,
	path,
	anchor,
	kind,
	label,
	description,
	tags_json,
	is_rule,
	is_stale,
	stale_at,
	updated_at
FROM ctx_pointers
WHERE project_id = ?
	AND pointer_key IN (` + placeholders(len(keys)) + `)
`

	args := make([]any, 0, len(keys)+1)
	args = append(args, projectID)
	for _, key := range keys {
		args = append(args, key)
	}

	rows, err := r.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("query pointers by key: %w", err)
	}
	defer rows.Close()

	out := make(map[string]pointerRow)
	for rows.Next() {
		var (
			item       pointerRow
			tagsJSON   string
			isRuleInt  int64
			isStaleInt int64
			staleAtSec sql.NullInt64
			updatedAt  int64
		)
		if err := rows.Scan(
			&item.Key,
			&item.Path,
			&item.Anchor,
			&item.Kind,
			&item.Label,
			&item.Description,
			&tagsJSON,
			&isRuleInt,
			&isStaleInt,
			&staleAtSec,
			&updatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan pointer by key: %w", err)
		}
		tags, err := decodeStringList(tagsJSON)
		if err != nil {
			return nil, fmt.Errorf("decode pointer tags: %w", err)
		}
		item.Tags = tags
		item.IsRule = isRuleInt != 0
		item.IsStale = isStaleInt != 0
		if staleAtSec.Valid {
			item.staleAt = unixTime(staleAtSec.Int64)
		}
		item.UpdatedAt = unixTime(updatedAt)
		out[item.Key] = item
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate pointers by key: %w", err)
	}
	return out, nil
}

func (r *Repository) pointerPathsByKeys(ctx context.Context, projectID string, pointerKeys []string) ([]string, error) {
	if len(pointerKeys) == 0 {
		return nil, nil
	}

	query := `
SELECT path
FROM ctx_pointers
WHERE project_id = ?
	AND pointer_key IN (` + placeholders(len(pointerKeys)) + `)
`
	args := make([]any, 0, len(pointerKeys)+1)
	args = append(args, projectID)
	for _, key := range pointerKeys {
		args = append(args, key)
	}

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query pointer paths by keys: %w", err)
	}
	defer rows.Close()

	paths := make([]string, 0, len(pointerKeys))
	for rows.Next() {
		var pointerPath string
		if err := rows.Scan(&pointerPath); err != nil {
			return nil, fmt.Errorf("scan pointer path: %w", err)
		}
		paths = append(paths, normalizeSyncPath(pointerPath))
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate pointer paths: %w", err)
	}
	return normalizeStringList(paths), nil
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

func ensureParentDirectory(dbPath string) error {
	if dbPath == "" || dbPath == ":memory:" {
		return nil
	}
	parent := filepath.Dir(dbPath)
	if parent == "." || parent == "" {
		return nil
	}
	if err := os.MkdirAll(parent, 0o755); err != nil {
		return fmt.Errorf("create sqlite parent directory: %w", err)
	}
	return nil
}

func placeholders(n int) string {
	if n <= 0 {
		return ""
	}
	parts := make([]string, n)
	for i := range parts {
		parts[i] = "?"
	}
	return strings.Join(parts, ", ")
}

func textMatchRank(tokens []string, pointer core.CandidatePointer) int {
	if len(tokens) == 0 {
		return 0
	}
	searchSpace := strings.ToLower(strings.Join([]string{
		pointer.Key,
		pointer.Path,
		pointer.Anchor,
		pointer.Kind,
		pointer.Label,
		pointer.Description,
		strings.Join(pointer.Tags, " "),
	}, " "))
	score := 0
	for _, token := range tokens {
		if strings.Contains(searchSpace, token) {
			score++
		}
	}
	return score
}

func phaseWeight(phase string, pointer core.CandidatePointer) float64 {
	pointerType := pointerKind(pointer)
	switch phase {
	case "plan":
		switch pointerType {
		case "rule":
			return 3
		case "doc":
			return 2
		default:
			return 1
		}
	case "execute":
		switch pointerType {
		case "code":
			return 3
		case "test":
			return 2
		case "rule":
			return 1
		default:
			return 1
		}
	case "review":
		switch pointerType {
		case "rule":
			return 3
		case "test":
			return 2
		case "code":
			return 1
		default:
			return 1
		}
	default:
		return 1
	}
}

func pointerKind(pointer core.CandidatePointer) string {
	if pointer.IsRule {
		return "rule"
	}
	kind := strings.ToLower(strings.TrimSpace(pointer.Kind))
	switch kind {
	case "doc", "docs", "documentation":
		return "doc"
	case "test", "tests":
		return "test"
	}
	if strings.HasSuffix(pointer.Path, "_test.go") {
		return "test"
	}
	return "code"
}

func tokenize(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	fields := strings.FieldsFunc(strings.ToLower(raw), func(r rune) bool {
		return !(r == '.' || r == '_' || r == '-' || r == '/' || r == ':' || (r >= '0' && r <= '9') || (r >= 'a' && r <= 'z'))
	})
	return normalizeStringList(fields)
}

func overlapCount(values, targets []string) int {
	if len(values) == 0 || len(targets) == 0 {
		return 0
	}
	targetSet := make(map[string]struct{}, len(targets))
	for _, value := range targets {
		targetSet[value] = struct{}{}
	}
	count := 0
	seen := map[string]struct{}{}
	for _, value := range values {
		if _, ok := targetSet[value]; !ok {
			continue
		}
		if _, dupe := seen[value]; dupe {
			continue
		}
		seen[value] = struct{}{}
		count++
	}
	return count
}

func matchesStaleFilter(isStale bool, staleAt *time.Time, filter core.StaleFilter) bool {
	if !filter.AllowStale && isStale {
		return false
	}
	if filter.StaleBefore != nil {
		if !isStale {
			return true
		}
		if staleAt == nil {
			return false
		}
		return !staleAt.After(filter.StaleBefore.UTC())
	}
	return true
}

func encodeStringList(values []string) (string, error) {
	raw, err := json.Marshal(values)
	if err != nil {
		return "", fmt.Errorf("marshal string list: %w", err)
	}
	return string(raw), nil
}

func decodeStringList(raw string) ([]string, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return nil, nil
	}
	var values []string
	if err := json.Unmarshal([]byte(trimmed), &values); err != nil {
		return nil, fmt.Errorf("unmarshal string list: %w", err)
	}
	return normalizeStringList(values), nil
}

func encodeInt64List(values []int64) (string, error) {
	raw, err := json.Marshal(values)
	if err != nil {
		return "", fmt.Errorf("marshal int64 list: %w", err)
	}
	return string(raw), nil
}

func decodeInt64List(raw string) ([]int64, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return nil, nil
	}
	var values []int64
	if err := json.Unmarshal([]byte(trimmed), &values); err != nil {
		return nil, fmt.Errorf("unmarshal int64 list: %w", err)
	}
	return normalizeInt64List(values), nil
}

func boolToInt(v bool) int {
	if v {
		return 1
	}
	return 0
}

func unixTime(sec int64) time.Time {
	if sec <= 0 {
		return time.Unix(0, 0).UTC()
	}
	return time.Unix(sec, 0).UTC()
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
			pathByKey[pathKey] = core.SyncPath{Path: pathKey, Deleted: true}
			continue
		}

		contentHash := strings.TrimSpace(raw.ContentHash)
		if contentHash == "" {
			return core.SyncApplyInput{}, fmt.Errorf("content_hash is required for path %q", pathKey)
		}
		pathByKey[pathKey] = core.SyncPath{
			Path:        pathKey,
			ContentHash: contentHash,
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

func newReceiptID() (string, error) {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", fmt.Errorf("generate receipt id: %w", err)
	}
	return fmt.Sprintf("receipt-%d-%s", time.Now().UTC().UnixNano(), hex.EncodeToString(b[:])), nil
}

func normalizePhase(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "plan":
		return "plan"
	case "review":
		return "review"
	case "execute":
		return "execute"
	default:
		return defaultPhase
	}
}

func normalizeLimit(v int, fallback int) int {
	if v <= 0 {
		v = fallback
	}
	if v > maxQueryLimit {
		v = maxQueryLimit
	}
	return v
}

func normalizeStringList(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))
	for _, raw := range values {
		trimmed := strings.TrimSpace(raw)
		if trimmed == "" {
			continue
		}
		if _, ok := seen[trimmed]; ok {
			continue
		}
		seen[trimmed] = struct{}{}
		out = append(out, trimmed)
	}
	sort.Strings(out)
	return out
}

func normalizeInt64List(values []int64) []int64 {
	if len(values) == 0 {
		return nil
	}
	seen := make(map[int64]struct{}, len(values))
	out := make([]int64, 0, len(values))
	for _, v := range values {
		if _, ok := seen[v]; ok {
			continue
		}
		seen[v] = struct{}{}
		out = append(out, v)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

func normalizeStaleBefore(t *time.Time) *time.Time {
	if t == nil {
		return nil
	}
	utc := t.UTC()
	return &utc
}

func nonNilStringList(values []string) []string {
	if len(values) == 0 {
		return []string{}
	}
	return values
}

func nonNilInt64List(values []int64) []int64 {
	if len(values) == 0 {
		return []int64{}
	}
	return values
}

func normalizeWorkItems(items []core.WorkItem) ([]core.WorkItem, error) {
	if len(items) == 0 {
		return nil, nil
	}

	priority := map[string]int{
		workItemStatusComplete:   0,
		workItemStatusCompleted:  0,
		workItemStatusPending:    1,
		workItemStatusInProgress: 2,
		workItemStatusBlocked:    3,
	}

	byKey := make(map[string]core.WorkItem, len(items))
	for _, raw := range items {
		itemKey := normalizeSyncPath(raw.ItemKey)
		if itemKey == "" {
			return nil, fmt.Errorf("work item key is required")
		}
		status := normalizeWorkItemStatus(raw.Status)

		current, exists := byKey[itemKey]
		if !exists || priority[status] >= priority[current.Status] {
			byKey[itemKey] = core.WorkItem{ItemKey: itemKey, Status: status}
		}
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

	return out, nil
}

func storageWorkItemStatus(raw string) string {
	switch normalizeWorkItemStatus(raw) {
	case workItemStatusComplete:
		return workItemStatusCompleted
	default:
		return normalizeWorkItemStatus(raw)
	}
}

func normalizeWorkItemStatus(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case workItemStatusComplete, workItemStatusCompleted:
		return workItemStatusComplete
	case workItemStatusInProgress:
		return workItemStatusInProgress
	case workItemStatusBlocked:
		return workItemStatusBlocked
	default:
		return workItemStatusPending
	}
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

func normalizeSyncPath(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return ""
	}
	withSlashes := strings.ReplaceAll(trimmed, `\`, "/")
	cleaned := path.Clean(withSlashes)
	if cleaned == "." {
		return ""
	}
	return cleaned
}

func mapKeysSorted(values map[string]struct{}) []string {
	if len(values) == 0 {
		return nil
	}
	out := make([]string, 0, len(values))
	for key := range values {
		out = append(out, key)
	}
	sort.Strings(out)
	return out
}
