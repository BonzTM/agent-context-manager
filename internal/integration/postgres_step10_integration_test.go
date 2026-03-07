//go:build integration

package integration

import (
	"context"
	"fmt"
	"os"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/bonztm/agent-context-manager/internal/contracts/v1"
	"github.com/bonztm/agent-context-manager/internal/logging"
	"github.com/bonztm/agent-context-manager/internal/runtime"
	postgressvc "github.com/bonztm/agent-context-manager/internal/service/postgres"
)

const (
	integrationDSNEnvVar = "ACM_PG_DSN"
)

func TestRuntimePostgresIntegration_Step10Evidence(t *testing.T) {
	dsn := strings.TrimSpace(os.Getenv(integrationDSNEnvVar))
	if dsn == "" {
		t.Skipf("%s is required", integrationDSNEnvVar)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	svc, cleanup, err := runtime.NewServiceWithLogger(ctx, runtime.Config{PostgresDSN: dsn}, logging.NewDiscardLogger())
	if err != nil {
		t.Fatalf("new runtime service: %v", err)
	}
	t.Cleanup(cleanup)

	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("connect postgres for assertions: %v", err)
	}
	t.Cleanup(pool.Close)

	assertMigrationsApplied(t, ctx, pool)

	projectID := fmt.Sprintf("step10.integration.%d", time.Now().UTC().UnixNano())
	pointers := []seedPointer{
		{
			Key:         "pointer.runtime.init",
			Path:        "internal/runtime/service_factory.go",
			Label:       "Runtime Postgres init path",
			Description: "Runtime wiring should initialize a migrated Postgres repository",
			Tags:        []string{"postgres", "runtime", "integration"},
		},
		{
			Key:         "pointer.service.report",
			Path:        "internal/service/postgres/service.go",
			Label:       "Report completion write path",
			Description: "Report completion persists run summaries tied to receipt scope",
			Tags:        []string{"postgres", "report_completion", "integration"},
		},
	}
	for _, pointer := range pointers {
		seedPointerRow(t, ctx, pool, projectID, pointer)
	}

	getResult, apiErr := svc.GetContext(ctx, v1.GetContextPayload{
		ProjectID: projectID,
		TaskText:  "postgres integration evidence for runtime migration and report completion",
		Phase:     v1.PhaseExecute,
		Caps: &v1.RetrievalCaps{
			MaxNonRulePointers: 4,
			MaxRulePointers:    0,
			MaxHops:            1,
			MaxHopExpansion:    0,
			MaxMemories:        0,
			MinPointerCount:    2,
		},
	})
	if apiErr != nil {
		t.Fatalf("get_context API error: %+v", apiErr)
	}
	if getResult.Status != "ok" {
		t.Fatalf("expected get_context status ok, got %q", getResult.Status)
	}
	if getResult.Receipt == nil {
		t.Fatal("expected non-nil receipt")
	}
	if got := getResult.Receipt.Meta.RetrievalVersion; got != postgressvc.RetrievalVersion {
		t.Fatalf("expected retrieval version %q, got %q", postgressvc.RetrievalVersion, got)
	}

	pointerKeys := receiptPointerKeys(getResult.Receipt)
	if len(pointerKeys) < 2 {
		t.Fatalf("expected at least 2 pointer keys from receipt, got %d", len(pointerKeys))
	}
	pointerPaths := lookupPointerPathsByKey(t, ctx, pool, getResult.Receipt.Meta.ProjectID, pointerKeys)
	if len(pointerPaths) < 1 {
		t.Fatal("expected at least 1 pointer path from receipt")
	}

	upsertReceiptScope(t, ctx, pool, getResult.Receipt, pointerKeys)

	autoPromote := false
	proposeResult, apiErr := svc.ProposeMemory(ctx, v1.ProposeMemoryPayload{
		ProjectID:   projectID,
		ReceiptID:   getResult.Receipt.Meta.ReceiptID,
		AutoPromote: &autoPromote,
		Memory: v1.MemoryPayload{
			Category:            v1.MemoryCategoryDecision,
			Subject:             "step-10 integration receipt scope",
			Content:             "Live Postgres integration validates migration init and write paths.",
			RelatedPointerKeys:  []string{pointerKeys[0]},
			Tags:                []string{"postgres", "integration"},
			Confidence:          4,
			EvidencePointerKeys: []string{pointerKeys[0]},
		},
	})
	if apiErr != nil {
		t.Fatalf("propose_memory API error: %+v", apiErr)
	}
	if proposeResult.CandidateID <= 0 {
		t.Fatalf("expected candidate_id > 0, got %d", proposeResult.CandidateID)
	}
	if proposeResult.Status != "pending" {
		t.Fatalf("expected propose_memory status pending, got %q", proposeResult.Status)
	}
	if !proposeResult.Validation.HardPassed {
		t.Fatalf("expected hard_passed=true, got false with errors: %v", proposeResult.Validation.Errors)
	}
	if !proposeResult.Validation.SoftPassed {
		t.Fatalf("expected soft_passed=true, got false with warnings: %v", proposeResult.Validation.Warnings)
	}

	var candidateStatus string
	var candidateReceiptID string
	if err := pool.QueryRow(ctx, `
SELECT status, receipt_id
FROM acm_memory_candidates
WHERE candidate_id = $1
`, proposeResult.CandidateID).Scan(&candidateStatus, &candidateReceiptID); err != nil {
		t.Fatalf("query persisted memory candidate: %v", err)
	}
	if candidateStatus != proposeResult.Status {
		t.Fatalf("expected persisted candidate status %q, got %q", proposeResult.Status, candidateStatus)
	}
	if candidateReceiptID != getResult.Receipt.Meta.ReceiptID {
		t.Fatalf("expected persisted candidate receipt_id %q, got %q", getResult.Receipt.Meta.ReceiptID, candidateReceiptID)
	}

	reportOutcome := "step-10 integration report completion accepted"
	reportResult, apiErr := svc.ReportCompletion(ctx, v1.ReportCompletionPayload{
		ProjectID:    projectID,
		ReceiptID:    getResult.Receipt.Meta.ReceiptID,
		FilesChanged: []string{pointerPaths[0]},
		Outcome:      reportOutcome,
	})
	if apiErr != nil {
		t.Fatalf("report_completion API error: code=%s message=%s details=%v", apiErr.Code, apiErr.Message, apiErr.Details)
	}
	if !reportResult.Accepted {
		t.Fatalf("expected accepted completion, got violations: %+v", reportResult.Violations)
	}
	if reportResult.RunID <= 0 {
		t.Fatalf("expected run_id > 0, got %d", reportResult.RunID)
	}

	var persistedStatus string
	var persistedOutcome string
	var persistedFiles []string
	if err := pool.QueryRow(ctx, `
SELECT status, outcome, files_changed
FROM acm_runs
WHERE run_id = $1
`, reportResult.RunID).Scan(&persistedStatus, &persistedOutcome, &persistedFiles); err != nil {
		t.Fatalf("query persisted run summary: %v", err)
	}
	if persistedStatus != "accepted" {
		t.Fatalf("expected persisted run status accepted, got %q", persistedStatus)
	}
	if persistedOutcome != reportOutcome {
		t.Fatalf("expected persisted run outcome %q, got %q", reportOutcome, persistedOutcome)
	}
	if len(persistedFiles) == 0 || !slices.Contains(persistedFiles, pointerPaths[0]) {
		t.Fatalf("expected persisted files_changed to include %q, got %v", pointerPaths[0], persistedFiles)
	}
}

func assertMigrationsApplied(t *testing.T, ctx context.Context, pool *pgxpool.Pool) {
	t.Helper()

	var relName string
	if err := pool.QueryRow(ctx, `SELECT COALESCE(to_regclass('public.acm_schema_migrations')::text, '')`).Scan(&relName); err != nil {
		t.Fatalf("query schema migration relation: %v", err)
	}
	if relName != "acm_schema_migrations" {
		t.Fatalf("expected acm_schema_migrations relation, got %q", relName)
	}

	rows, err := pool.Query(ctx, `SELECT migration_name FROM acm_schema_migrations ORDER BY migration_name`)
	if err != nil {
		t.Fatalf("query migration records: %v", err)
	}
	defer rows.Close()

	got := make([]string, 0, 4)
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			t.Fatalf("scan migration record: %v", err)
		}
		got = append(got, name)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate migration records: %v", err)
	}

	want := []string{
		"0001_acm_foundation.sql",
		"0002_acm_propose_memory.sql",
		"0003_acm_sync.sql",
		"0004_acm_work_items.sql",
		"0005_acm_work_plans.sql",
		"0006_acm_work_plan_hierarchy.sql",
		"0007_acm_verification_runs.sql",
		"0008_acm_run_history_indexes.sql",
		"0010_acm_review_attempts.sql",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected migration record set: got %v want %v", got, want)
	}
}

type seedPointer struct {
	Key         string
	Path        string
	Label       string
	Description string
	Tags        []string
}

func seedPointerRow(t *testing.T, ctx context.Context, pool *pgxpool.Pool, projectID string, pointer seedPointer) {
	t.Helper()

	if _, err := pool.Exec(ctx, `
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
	is_stale
) VALUES ($1, $2, $3, '', 'code', $4, $5, $6, FALSE, FALSE)
ON CONFLICT (project_id, pointer_key) DO UPDATE
SET
	path = EXCLUDED.path,
	kind = EXCLUDED.kind,
	label = EXCLUDED.label,
	description = EXCLUDED.description,
	tags = EXCLUDED.tags,
	is_rule = EXCLUDED.is_rule,
	is_stale = EXCLUDED.is_stale,
	stale_at = NULL,
	updated_at = NOW()
`, projectID, pointer.Key, pointer.Path, pointer.Label, pointer.Description, pointer.Tags); err != nil {
		t.Fatalf("seed pointer %q: %v", pointer.Key, err)
	}
}

func upsertReceiptScope(t *testing.T, ctx context.Context, pool *pgxpool.Pool, receipt *v1.ContextReceipt, pointerKeys []string) {
	t.Helper()

	memoryIDs := receiptMemoryIDs(receipt)

	if _, err := pool.Exec(ctx, `
INSERT INTO acm_receipts (
	receipt_id,
	project_id,
	task_text,
	phase,
	resolved_tags,
	pointer_keys,
	memory_ids,
	summary_json
) VALUES ($1, $2, $3, $4, $5, $6, $7, '{"source":"integration"}'::jsonb)
ON CONFLICT (receipt_id) DO UPDATE
SET
	project_id = EXCLUDED.project_id,
	task_text = EXCLUDED.task_text,
	phase = EXCLUDED.phase,
	resolved_tags = EXCLUDED.resolved_tags,
	pointer_keys = EXCLUDED.pointer_keys,
	memory_ids = EXCLUDED.memory_ids,
	summary_json = EXCLUDED.summary_json
`, receipt.Meta.ReceiptID, receipt.Meta.ProjectID, receipt.Meta.TaskText, string(receipt.Meta.Phase), receipt.Meta.ResolvedTags, pointerKeys, memoryIDs); err != nil {
		t.Fatalf("upsert receipt scope: %v", err)
	}
}

func receiptPointerKeys(receipt *v1.ContextReceipt) []string {
	if receipt == nil {
		return nil
	}

	keys := make(map[string]struct{}, len(receipt.Rules)+len(receipt.Suggestions))
	for _, rule := range receipt.Rules {
		key := strings.TrimSpace(rule.Key)
		if key != "" {
			keys[key] = struct{}{}
		}
	}
	for _, suggestion := range receipt.Suggestions {
		key := strings.TrimSpace(suggestion.Key)
		if key != "" {
			keys[key] = struct{}{}
		}
	}

	out := make([]string, 0, len(keys))
	for key := range keys {
		out = append(out, key)
	}
	slices.Sort(out)
	return out
}

func lookupPointerPathsByKey(t *testing.T, ctx context.Context, pool *pgxpool.Pool, projectID string, pointerKeys []string) []string {
	t.Helper()
	if strings.TrimSpace(projectID) == "" || len(pointerKeys) == 0 {
		return nil
	}

	rows, err := pool.Query(ctx, `
SELECT path
FROM acm_pointers
WHERE project_id = $1
	AND pointer_key = ANY($2)
ORDER BY path ASC
`, projectID, pointerKeys)
	if err != nil {
		t.Fatalf("query pointer paths by key: %v", err)
	}
	defer rows.Close()

	out := make([]string, 0, len(pointerKeys))
	for rows.Next() {
		var path string
		if err := rows.Scan(&path); err != nil {
			t.Fatalf("scan pointer path by key: %v", err)
		}
		normalized := strings.TrimSpace(path)
		if normalized == "" {
			continue
		}
		out = append(out, normalized)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate pointer paths by key: %v", err)
	}
	return out
}

func receiptMemoryIDs(receipt *v1.ContextReceipt) []int64 {
	if receipt == nil {
		return nil
	}

	seen := make(map[int64]struct{}, len(receipt.Memories))
	out := make([]int64, 0, len(receipt.Memories))
	for _, memory := range receipt.Memories {
		key := strings.TrimSpace(memory.Key)
		if !strings.HasPrefix(key, "mem:") {
			continue
		}
		rawID := strings.TrimSpace(strings.TrimPrefix(key, "mem:"))
		if rawID == "" {
			continue
		}
		id, err := strconv.ParseInt(rawID, 10, 64)
		if err != nil || id <= 0 {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	slices.Sort(out)
	return out
}
