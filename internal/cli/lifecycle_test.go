package cli

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
)

func TestPruneIsDryRunBackupFirstAndPinSafe(t *testing.T) {
	root := t.TempDir()
	dbPath := filepath.Join(root, "acm.db")
	pinned := ingestLifecycleConversation(t, dbPath, "pinned", "pinned-marker", false)
	unexpanded := ingestLifecycleConversation(t, dbPath, "unexpanded", "sealed-marker", true)
	expanded := ingestLifecycleConversation(t, dbPath, "expanded", "reviewed-marker", true)
	plain := ingestLifecycleConversation(t, dbPath, "plain", "plain-marker", false)
	runACM(t, dbPath, "", "expand", summaryForMarker(t, dbPath, "reviewed-marker"))
	runACM(t, dbPath, "", "pin", pinned)

	dryRun := runACM(t, dbPath, "", "prune", "--older-than", "1ns")
	assertPruneStatus(t, dryRun, pinned, "pinned")
	assertPruneStatus(t, dryRun, unexpanded, "unexpanded")
	assertPruneStatus(t, dryRun, expanded, "eligible")
	assertPruneStatus(t, dryRun, plain, "eligible")
	assertConversationCount(t, dbPath, 4)

	backup := filepath.Join(root, "pre-prune.db")
	apply := runACM(t, dbPath, "", "prune", "--older-than", "1ns", "--apply", "--backup", backup)
	if !strings.Contains(apply, "deleted=2") {
		t.Fatalf("apply output = %q", apply)
	}
	assertConversationCount(t, dbPath, 2)
	assertConversationCount(t, backup, 4)
	if output := runACM(t, dbPath, "", "grep", "--substr", "plain-marker"); !strings.Contains(output, "no matches") {
		t.Fatalf("deleted conversation remains searchable: %q", output)
	}

	forcedBackup := filepath.Join(root, "forced-prune.db")
	forced := runACM(t, dbPath, "", "prune", "--older-than", "1ns", "--apply", "--force", "--backup", forcedBackup)
	if !strings.Contains(forced, "deleted=1") {
		t.Fatalf("forced output = %q", forced)
	}
	assertConversationCount(t, dbPath, 1)
	if output := runACM(t, dbPath, "", "grep", "--substr", "pinned-marker"); strings.Contains(output, "no matches") {
		t.Fatalf("pinned conversation was deleted: %q", output)
	}
}

func TestCarryOverSelectsDeepestSummariesAndDedupes(t *testing.T) {
	root := t.TempDir()
	dbPath := filepath.Join(root, "acm.db")
	source := ingestDeepLifecycleConversation(t, dbPath)

	first := runACM(t, dbPath, "", "carry-over", source, "continued-session")
	if !strings.Contains(first, "depth=2 summaries=1 appended=1 deduped=0 pinned=true") {
		t.Fatalf("first carry-over = %q", first)
	}
	second := runACM(t, dbPath, "", "carry-over", source, "continued-session")
	if !strings.Contains(second, "depth=2 summaries=1 appended=0 deduped=1 pinned=true") {
		t.Fatalf("second carry-over = %q", second)
	}
	plan := runACM(t, dbPath, "", "prune", "--older-than", "1ns", "--force")
	assertPruneStatus(t, plan, source, "pinned")
	carried := runACM(t, dbPath, "", "grep", "--substr", "--json", "acm-carry-over")
	var results grepResult
	if err := json.Unmarshal([]byte(carried), &results); err != nil || len(results.Messages) != 1 {
		t.Fatalf("carry-over messages = %q err=%v", carried, err)
	}
}

func ingestLifecycleConversation(t *testing.T, dbPath, session, marker string, compact bool) string {
	t.Helper()
	content := marker
	if compact {
		content += " " + strings.Repeat("history ", 600)
	}
	payload := fmt.Sprintf(`{"agent":"codex","session_id":%q,"messages":[{"role":"user","content":%q,"external_id":"m1"}]}`, session, content)
	runACM(t, dbPath, payload, "ingest")
	conversationID := conversationForMarker(t, dbPath, marker)
	if compact {
		compactLifecycleConversation(t, dbPath, conversationID, "0.1")
	}
	return conversationID
}

func ingestDeepLifecycleConversation(t *testing.T, dbPath string) string {
	t.Helper()
	var messages strings.Builder
	for index := range 8 {
		if index > 0 {
			messages.WriteByte(',')
		}
		fmt.Fprintf(&messages, `{"role":"user","content":%q,"external_id":%q}`,
			fmt.Sprintf("deep-marker-%d %s", index, strings.Repeat("history ", 80)), fmt.Sprintf("m%d", index))
	}
	payload := fmt.Sprintf(`{"agent":"codex","session_id":"deep-source","messages":[%s]}`, messages.String())
	runACM(t, dbPath, payload, "ingest")
	conversationID := conversationForMarker(t, dbPath, "deep-marker-0")
	compactLifecycleConversation(t, dbPath, conversationID, "0.05")
	return conversationID
}

func compactLifecycleConversation(t *testing.T, dbPath, conversationID, softFraction string) {
	t.Helper()
	runACM(t, dbPath, "", "compact", conversationID,
		"--model-context-tokens", "1000", "--soft-fraction", softFraction, "--fresh-tail", "0", "--fresh-tail-tokens", "0",
		"--leaf-chunk-tokens", "220", "--leaf-target-tokens", "20", "--condensed-target-tokens", "20",
		"--condense-fanout", "2", "--condense-chunk-tokens", "100", "--truncate-tokens", "10")
}

func conversationForMarker(t *testing.T, dbPath, marker string) string {
	t.Helper()
	output := runACM(t, dbPath, "", "grep", "--substr", "--json", marker)
	var results grepResult
	if err := json.Unmarshal([]byte(output), &results); err != nil || len(results.Messages) == 0 {
		t.Fatalf("grep %s = %q err=%v", marker, output, err)
	}
	return results.Messages[0].Message.ConversationID
}

func summaryForMarker(t *testing.T, dbPath, marker string) string {
	t.Helper()
	output := runACM(t, dbPath, "", "grep", "--substr", "--json", marker)
	var results grepResult
	if err := json.Unmarshal([]byte(output), &results); err != nil || len(results.Summaries) != 1 {
		t.Fatalf("summary grep %s = %q err=%v", marker, output, err)
	}
	return results.Summaries[0].Summary.ID
}

func assertPruneStatus(t *testing.T, output, conversationID, status string) {
	t.Helper()
	line := conversationID + " "
	for candidate := range strings.SplitSeq(output, "\n") {
		if strings.HasPrefix(candidate, line) && strings.Contains(candidate, "status="+status) {
			return
		}
	}
	t.Fatalf("missing %s status for %s in %q", status, conversationID, output)
}

func assertConversationCount(t *testing.T, dbPath string, count int) {
	t.Helper()
	stats := runACM(t, dbPath, "", "stats")
	want := fmt.Sprintf("conversations: %d", count)
	if !strings.Contains(stats, want) {
		t.Fatalf("stats = %q, want %q", stats, want)
	}
}
