package postgres

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/bonztm/agent-context-manager/internal/core"
	storagedomain "github.com/bonztm/agent-context-manager/internal/storage/domain"
)

const (
	defaultCandidateLimit = 32
	defaultMemoryLimit    = 16
	maxQueryLimit         = 512
	defaultPhase          = "execute"

	candidateStatusPending  = "pending"
	candidateStatusPromoted = "promoted"
	candidateStatusRejected = "rejected"

	workItemStatusPending    = core.WorkItemStatusPending
	workItemStatusInProgress = core.WorkItemStatusInProgress
	workItemStatusBlocked    = core.WorkItemStatusBlocked
	workItemStatusComplete   = core.WorkItemStatusComplete
)

type sqlArgs struct {
	values []any
}

func (a *sqlArgs) add(v any) string {
	a.values = append(a.values, v)
	return fmt.Sprintf("$%d", len(a.values))
}

func buildCandidatePointersQuery(input core.CandidatePointerQuery) (string, []any, error) {
	projectID := strings.TrimSpace(input.ProjectID)
	if projectID == "" {
		return "", nil, fmt.Errorf("project_id is required")
	}

	args := &sqlArgs{}
	projectIDArg := args.add(projectID)

	predicates := make([]string, 0, 2)
	if !input.StaleFilter.AllowStale {
		predicates = append(predicates, "p.is_stale = FALSE")
	}
	if input.StaleFilter.StaleBefore != nil {
		staleBeforeArg := args.add(input.StaleFilter.StaleBefore.UTC())
		predicates = append(predicates, fmt.Sprintf("(p.is_stale = FALSE OR (p.stale_at IS NOT NULL AND p.stale_at <= %s))", staleBeforeArg))
	}

	var sb strings.Builder
	sb.WriteString(`
SELECT
	p.pointer_key,
	p.path,
	p.anchor,
	p.kind,
	p.label,
	p.description,
	p.tags,
	p.is_rule,
	p.is_stale,
	p.updated_at
FROM acm_pointers p
WHERE p.project_id = `)
	sb.WriteString(projectIDArg)

	if len(predicates) > 0 {
		sb.WriteString(" AND ")
		sb.WriteString(strings.Join(predicates, " AND "))
	}

	sb.WriteString(`
ORDER BY p.pointer_key ASC`)

	return sb.String(), args.values, nil
}

func buildActiveMemoriesQuery(input core.ActiveMemoryQuery) (string, []any, error) {
	projectID := strings.TrimSpace(input.ProjectID)
	if projectID == "" {
		return "", nil, fmt.Errorf("project_id is required")
	}

	pointerKeys := normalizeStringList(input.PointerKeys)
	tags := normalizeStringList(input.Tags)
	limit := normalizeLimit(input.Limit, defaultMemoryLimit)

	args := &sqlArgs{}
	projectIDArg := args.add(projectID)

	filters := []string{"m.active = TRUE"}
	matchers := make([]string, 0, 2)
	if len(pointerKeys) > 0 {
		pointerKeysArg := args.add(pointerKeys)
		matchers = append(matchers, fmt.Sprintf("m.related_pointer_keys && %s::text[]", pointerKeysArg))
	}
	if len(tags) > 0 {
		tagsArg := args.add(tags)
		matchers = append(matchers, fmt.Sprintf("m.tags && %s::text[]", tagsArg))
	}
	if len(matchers) > 0 {
		filters = append(filters, "("+strings.Join(matchers, " OR ")+")")
	}

	var sb strings.Builder
	sb.WriteString(`
SELECT
	m.memory_id,
	m.category,
	m.subject,
	m.content,
	m.confidence,
	m.tags,
	m.related_pointer_keys,
	m.updated_at
FROM acm_memories m
WHERE m.project_id = `)
	sb.WriteString(projectIDArg)
	sb.WriteString(" AND ")
	sb.WriteString(strings.Join(filters, " AND "))
	sb.WriteString(`
ORDER BY m.confidence DESC, m.updated_at DESC, m.memory_id ASC
`)
	if !input.Unbounded {
		limitArg := args.add(limit)
		sb.WriteString("LIMIT ")
		sb.WriteString(limitArg)
	}

	return sb.String(), args.values, nil
}

func buildFetchReceiptScopeQuery(input core.ReceiptScopeQuery) (string, []any, error) {
	projectID := strings.TrimSpace(input.ProjectID)
	if projectID == "" {
		return "", nil, fmt.Errorf("project_id is required")
	}
	receiptID := strings.TrimSpace(input.ReceiptID)
	if receiptID == "" {
		return "", nil, fmt.Errorf("receipt_id is required")
	}

	return `
SELECT
	r.receipt_id,
	r.task_text,
	r.phase,
	r.resolved_tags,
	r.pointer_keys,
	r.memory_ids,
	r.initial_scope_paths,
	r.baseline_captured,
	r.baseline_paths_json
FROM acm_receipts r
WHERE r.project_id = $1
	AND r.receipt_id = $2
`, []any{projectID, receiptID}, nil
}

func buildLookupFetchStateQuery(input core.FetchLookupQuery) (string, []any, error) {
	projectID := strings.TrimSpace(input.ProjectID)
	if projectID == "" {
		return "", nil, fmt.Errorf("project_id is required")
	}
	receiptID := strings.TrimSpace(input.ReceiptID)
	if receiptID == "" {
		return "", nil, fmt.Errorf("receipt_id is required")
	}

	return `
SELECT
	r.receipt_id,
	COALESCE(run.run_id, 0) AS run_id,
	COALESCE(run.status, '') AS run_status,
	COALESCE(run.created_at, r.created_at) AS updated_at
FROM acm_receipts r
LEFT JOIN LATERAL (
	SELECT run_id, status, created_at
	FROM acm_runs
	WHERE project_id = r.project_id
		AND receipt_id = r.receipt_id
	ORDER BY created_at DESC, run_id DESC
	LIMIT 1
) run ON TRUE
WHERE r.project_id = $1
	AND r.receipt_id = $2
`, []any{projectID, receiptID}, nil
}

func buildLookupPointerByKeyQuery(input core.PointerLookupQuery) (string, []any, error) {
	projectID := strings.TrimSpace(input.ProjectID)
	if projectID == "" {
		return "", nil, fmt.Errorf("project_id is required")
	}
	pointerKey := strings.TrimSpace(input.PointerKey)
	if pointerKey == "" {
		return "", nil, fmt.Errorf("pointer_key is required")
	}

	return `
SELECT
	pointer_key,
	path,
	anchor,
	kind,
	label,
	description,
	tags,
	is_rule,
	is_stale,
	updated_at
FROM acm_pointers
WHERE project_id = $1
	AND pointer_key = $2
`, []any{projectID, pointerKey}, nil
}

func buildLookupMemoryByIDQuery(input core.MemoryLookupQuery) (string, []any, error) {
	projectID := strings.TrimSpace(input.ProjectID)
	if projectID == "" {
		return "", nil, fmt.Errorf("project_id is required")
	}
	if input.MemoryID <= 0 {
		return "", nil, fmt.Errorf("memory_id must be positive")
	}

	return `
SELECT
	memory_id,
	category,
	subject,
	content,
	confidence,
	tags,
	related_pointer_keys,
	updated_at
FROM acm_memories
WHERE project_id = $1
	AND memory_id = $2
`, []any{projectID, input.MemoryID}, nil
}

func buildListWorkItemsQuery(input core.FetchLookupQuery) (string, []any, error) {
	projectID := strings.TrimSpace(input.ProjectID)
	if projectID == "" {
		return "", nil, fmt.Errorf("project_id is required")
	}
	receiptID := strings.TrimSpace(input.ReceiptID)
	if receiptID == "" {
		return "", nil, fmt.Errorf("receipt_id is required")
	}

	return `
SELECT
	item_key,
	status,
	updated_at
FROM acm_work_items
WHERE project_id = $1
	AND receipt_id = $2
ORDER BY item_key ASC
`, []any{projectID, receiptID}, nil
}

func buildUpsertWorkItemsQuery(input core.WorkItemsUpsertInput) (string, []any, error) {
	projectID := strings.TrimSpace(input.ProjectID)
	if projectID == "" {
		return "", nil, fmt.Errorf("project_id is required")
	}
	receiptID := strings.TrimSpace(input.ReceiptID)
	if receiptID == "" {
		return "", nil, fmt.Errorf("receipt_id is required")
	}

	items, err := normalizeWorkItems(input.Items)
	if err != nil {
		return "", nil, err
	}
	if len(items) == 0 {
		return "", nil, fmt.Errorf("work items are required")
	}

	args := &sqlArgs{}
	projectIDArg := args.add(projectID)
	receiptIDArg := args.add(receiptID)

	valuesRows := make([]string, 0, len(items))
	for _, item := range items {
		itemKeyArg := args.add(item.ItemKey)
		statusArg := args.add(storageWorkItemStatus(item.Status))
		valuesRows = append(valuesRows, fmt.Sprintf("(%s, %s)", itemKeyArg, statusArg))
	}

	query := `
WITH incoming(item_key, status) AS (
	VALUES ` + strings.Join(valuesRows, ", ") + `
)
INSERT INTO acm_work_items (
	project_id,
	receipt_id,
	item_key,
	status,
	created_at,
	updated_at
)
SELECT
	` + projectIDArg + `,
	` + receiptIDArg + `,
	i.item_key,
	i.status,
	NOW(),
	NOW()
FROM incoming i
ON CONFLICT (project_id, receipt_id, item_key) DO UPDATE
SET
	status = EXCLUDED.status,
	updated_at = NOW()
`

	return query, args.values, nil
}

func buildInsertMemoryCandidateQuery(input core.MemoryPersistence, status string) (string, []any, error) {
	projectID := strings.TrimSpace(input.ProjectID)
	if projectID == "" {
		return "", nil, fmt.Errorf("project_id is required")
	}
	receiptID := strings.TrimSpace(input.ReceiptID)
	if receiptID == "" {
		return "", nil, fmt.Errorf("receipt_id is required")
	}

	status = strings.TrimSpace(status)
	if !isValidCandidateStatus(status) {
		return "", nil, fmt.Errorf("candidate status is invalid")
	}

	category := strings.TrimSpace(input.Category)
	if category == "" {
		return "", nil, fmt.Errorf("category is required")
	}
	subject := strings.TrimSpace(input.Subject)
	if subject == "" {
		return "", nil, fmt.Errorf("subject is required")
	}
	content := strings.TrimSpace(input.Content)
	if content == "" {
		return "", nil, fmt.Errorf("content is required")
	}
	if input.Confidence < 1 || input.Confidence > 5 {
		return "", nil, fmt.Errorf("confidence must be 1..5")
	}

	evidencePointerKeys := normalizeStringList(input.EvidencePointerKeys)
	if len(evidencePointerKeys) == 0 {
		return "", nil, fmt.Errorf("evidence_pointer_keys is required")
	}
	dedupeKey := strings.TrimSpace(input.DedupeKey)
	if dedupeKey == "" {
		return "", nil, fmt.Errorf("dedupe_key is required")
	}

	return `
INSERT INTO acm_memory_candidates (
	project_id,
	receipt_id,
	category,
	subject,
	content,
	confidence,
	tags,
	related_pointer_keys,
	evidence_pointer_keys,
	dedupe_key,
	status,
	hard_passed,
	soft_passed,
	validation_errors,
	validation_warnings,
	auto_promote,
	promotable
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17)
RETURNING candidate_id
`, []any{
			projectID,
			receiptID,
			category,
			subject,
			content,
			input.Confidence,
			nonNilStringList(normalizeStringList(input.Tags)),
			nonNilStringList(normalizeStringList(input.RelatedPointerKeys)),
			evidencePointerKeys,
			dedupeKey,
			status,
			input.Validation.HardPassed,
			input.Validation.SoftPassed,
			nonNilStringList(normalizeStringList(input.Validation.Errors)),
			nonNilStringList(normalizeStringList(input.Validation.Warnings)),
			input.AutoPromote,
			input.Promotable,
		}, nil
}

func buildInsertDurableMemoryQuery(input core.MemoryPersistence) (string, []any, error) {
	projectID := strings.TrimSpace(input.ProjectID)
	if projectID == "" {
		return "", nil, fmt.Errorf("project_id is required")
	}
	category := strings.TrimSpace(input.Category)
	if category == "" {
		return "", nil, fmt.Errorf("category is required")
	}
	subject := strings.TrimSpace(input.Subject)
	if subject == "" {
		return "", nil, fmt.Errorf("subject is required")
	}
	content := strings.TrimSpace(input.Content)
	if content == "" {
		return "", nil, fmt.Errorf("content is required")
	}
	if input.Confidence < 1 || input.Confidence > 5 {
		return "", nil, fmt.Errorf("confidence must be 1..5")
	}
	evidencePointerKeys := normalizeStringList(input.EvidencePointerKeys)
	if len(evidencePointerKeys) == 0 {
		return "", nil, fmt.Errorf("evidence_pointer_keys is required")
	}
	dedupeKey := strings.TrimSpace(input.DedupeKey)
	if dedupeKey == "" {
		return "", nil, fmt.Errorf("dedupe_key is required")
	}

	return `
INSERT INTO acm_memories (
	project_id,
	category,
	subject,
	content,
	confidence,
	tags,
	related_pointer_keys,
	evidence_pointer_keys,
	dedupe_key
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
ON CONFLICT DO NOTHING
RETURNING memory_id
`, []any{
			projectID,
			category,
			subject,
			content,
			input.Confidence,
			nonNilStringList(normalizeStringList(input.Tags)),
			nonNilStringList(normalizeStringList(input.RelatedPointerKeys)),
			evidencePointerKeys,
			dedupeKey,
		}, nil
}

func buildUpdateMemoryCandidateStatusQuery(candidateID int64, status string, promotedMemoryID int64) (string, []any, error) {
	if candidateID <= 0 {
		return "", nil, fmt.Errorf("candidate_id must be positive")
	}
	status = strings.TrimSpace(status)
	if !isValidCandidateStatus(status) {
		return "", nil, fmt.Errorf("candidate status is invalid")
	}

	return `
UPDATE acm_memory_candidates
SET
	status = $2,
	promoted_memory_id = CASE WHEN $3::bigint > 0 THEN $3 ELSE NULL END,
	updated_at = NOW()
WHERE candidate_id = $1
`, []any{candidateID, status, promotedMemoryID}, nil
}

func buildMarkDeletedPointersStaleQuery(projectID string, deletedPaths []string) (string, []any, error) {
	projectID = strings.TrimSpace(projectID)
	if projectID == "" {
		return "", nil, fmt.Errorf("project_id is required")
	}
	normalizedPaths := normalizeSyncPaths(deletedPaths)
	if len(normalizedPaths) == 0 {
		return "", nil, fmt.Errorf("deleted paths are required")
	}

	return `
UPDATE acm_pointers
SET
	is_stale = TRUE,
	stale_at = NOW(),
	updated_at = NOW()
WHERE project_id = $1
	AND path = ANY($2::text[])
	AND is_stale = FALSE
`, []any{projectID, normalizedPaths}, nil
}

func buildMarkMissingPointersStaleQuery(projectID string, presentPaths []string) (string, []any, error) {
	projectID = strings.TrimSpace(projectID)
	if projectID == "" {
		return "", nil, fmt.Errorf("project_id is required")
	}
	normalizedPaths := normalizeSyncPaths(presentPaths)
	if len(normalizedPaths) == 0 {
		return `
UPDATE acm_pointers
SET
	is_stale = TRUE,
	stale_at = NOW(),
	updated_at = NOW()
WHERE project_id = $1
	AND is_stale = FALSE
`, []any{projectID}, nil
	}

	return `
UPDATE acm_pointers
SET
	is_stale = TRUE,
	stale_at = NOW(),
	updated_at = NOW()
WHERE project_id = $1
	AND is_stale = FALSE
	AND NOT (path = ANY($2::text[]))
`, []any{projectID, normalizedPaths}, nil
}

func buildRefreshPointersQuery(projectID string, paths []core.SyncPath) (string, []any, error) {
	projectID = strings.TrimSpace(projectID)
	if projectID == "" {
		return "", nil, fmt.Errorf("project_id is required")
	}
	normalizedPaths, err := normalizeSyncPathRows(paths, true)
	if err != nil {
		return "", nil, err
	}
	if len(normalizedPaths) == 0 {
		return "", nil, fmt.Errorf("paths are required")
	}

	args := &sqlArgs{}
	projectIDArg := args.add(projectID)
	valuesRows := make([]string, 0, len(normalizedPaths))
	for _, p := range normalizedPaths {
		pathArg := args.add(p.Path)
		hashArg := args.add(p.ContentHash)
		valuesRows = append(valuesRows, fmt.Sprintf("(%s, %s)", pathArg, hashArg))
	}

	query := `
WITH sync(path, content_hash) AS (
	VALUES ` + strings.Join(valuesRows, ", ") + `
)
UPDATE acm_pointers p
SET
	content_hash = sync.content_hash,
	is_stale = FALSE,
	stale_at = NULL,
	updated_at = NOW()
FROM sync
WHERE p.project_id = ` + projectIDArg + `
	AND p.path = sync.path
	AND (p.is_stale = TRUE OR p.content_hash IS DISTINCT FROM sync.content_hash)
`
	return query, args.values, nil
}

func buildInsertPointerCandidatesQuery(projectID string, paths []core.SyncPath) (string, []any, error) {
	projectID = strings.TrimSpace(projectID)
	if projectID == "" {
		return "", nil, fmt.Errorf("project_id is required")
	}
	normalizedPaths, err := normalizeSyncPathRows(paths, true)
	if err != nil {
		return "", nil, err
	}
	if len(normalizedPaths) == 0 {
		return "", nil, fmt.Errorf("paths are required")
	}

	args := &sqlArgs{}
	projectIDArg := args.add(projectID)
	valuesRows := make([]string, 0, len(normalizedPaths))
	for _, p := range normalizedPaths {
		pathArg := args.add(p.Path)
		hashArg := args.add(p.ContentHash)
		valuesRows = append(valuesRows, fmt.Sprintf("(%s, %s)", pathArg, hashArg))
	}

	query := `
WITH sync(path, content_hash) AS (
	VALUES ` + strings.Join(valuesRows, ", ") + `
),
missing AS (
	SELECT s.path, s.content_hash
	FROM sync s
	LEFT JOIN acm_pointers p
		ON p.project_id = ` + projectIDArg + `
		AND p.path = s.path
	WHERE p.pointer_id IS NULL
)
INSERT INTO acm_pointer_candidates (
	project_id,
	path,
	content_hash
)
SELECT
	` + projectIDArg + `,
	m.path,
	m.content_hash
FROM missing m
ORDER BY m.path ASC
ON CONFLICT (project_id, path) DO NOTHING
`
	return query, args.values, nil
}

func isValidCandidateStatus(status string) bool {
	switch status {
	case candidateStatusPending, candidateStatusPromoted, candidateStatusRejected:
		return true
	default:
		return false
	}
}

func normalizePhase(value string) string {
	return storagedomain.NormalizePhase(value)
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
	return storagedomain.NormalizeStringList(values)
}

func normalizeInt64List(values []int64) []int64 {
	return storagedomain.NormalizeInt64List(values)
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

func normalizeSyncPaths(paths []string) []string {
	if len(paths) == 0 {
		return nil
	}

	seen := make(map[string]struct{}, len(paths))
	out := make([]string, 0, len(paths))
	for _, raw := range paths {
		normalized := normalizeSyncPath(raw)
		if normalized == "" {
			continue
		}
		if _, ok := seen[normalized]; ok {
			continue
		}
		seen[normalized] = struct{}{}
		out = append(out, normalized)
	}
	sort.Strings(out)
	return out
}

func normalizeSyncPathRows(paths []core.SyncPath, requireHash bool) ([]core.SyncPath, error) {
	if len(paths) == 0 {
		return nil, nil
	}

	byPath := make(map[string]core.SyncPath, len(paths))
	for _, raw := range paths {
		normalizedPath := normalizeSyncPath(raw.Path)
		if normalizedPath == "" {
			return nil, fmt.Errorf("path is required")
		}
		if raw.Deleted {
			continue
		}

		contentHash := strings.TrimSpace(raw.ContentHash)
		if requireHash && contentHash == "" {
			return nil, fmt.Errorf("content_hash is required for path %q", normalizedPath)
		}

		current := byPath[normalizedPath]
		if current.Path == "" || current.ContentHash == "" {
			byPath[normalizedPath] = core.SyncPath{
				Path:        normalizedPath,
				ContentHash: contentHash,
			}
		}
	}

	if len(byPath) == 0 {
		return nil, nil
	}

	keys := make([]string, 0, len(byPath))
	for path := range byPath {
		keys = append(keys, path)
	}
	sort.Strings(keys)

	out := make([]core.SyncPath, 0, len(keys))
	for _, key := range keys {
		out = append(out, byPath[key])
	}
	return out, nil
}

func normalizeSyncPath(raw string) string {
	return storagedomain.NormalizeRepoPath(raw)
}
