package cli

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	opencodectx "github.com/bonztm/agent-context-manager/internal/opencode"
)

func TestOpenCodeContextArchivesRecallsAndIsIdempotent(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "acm.db")
	messages := make([]map[string]string, 0, 12)
	for index := range 12 {
		content := fmt.Sprintf("turn %d ", index) + strings.Repeat("context ", 600)
		if index == 0 {
			content = "zircon deployment contract " + content
		}
		messages = append(messages, map[string]string{
			"role": "assistant", "content": content, "external_id": fmt.Sprintf("oc-%d", index),
		})
	}
	payload, err := json.Marshal(map[string]any{
		"agent": "opencode", "session_id": "session-1", "messages": messages,
	})
	if err != nil {
		t.Fatalf("marshal ingest: %v", err)
	}
	runACM(t, dbPath, string(payload), "ingest")

	input := `{"session_id":"session-1","prompt":"what was the zircon deployment contract"}`
	first := decodeOpenCodePlan(t, runACM(t, dbPath, input, "opencode-context"))
	if len(first.SummaryRefs) == 0 || !strings.HasPrefix(first.SummaryText, opencodectx.SyntheticPrefix) {
		t.Fatalf("plan did not archive old messages: %+v", first)
	}
	fresh := make(map[string]bool, len(first.FreshMessageIDs))
	for _, id := range first.FreshMessageIDs {
		fresh[id] = true
	}
	if fresh["oc-0"] || !fresh["oc-11"] {
		t.Fatalf("fresh ids = %v, want old excluded and newest retained", first.FreshMessageIDs)
	}
	if !strings.Contains(first.RecallText, "[zircon]") ||
		!strings.Contains(first.RecallText, "[deployment]") ||
		!strings.Contains(first.RecallText, "[contract]") ||
		!strings.Contains(first.RecallText, "acm describe msg_") {
		t.Fatalf("recall text missing archived match: %q", first.RecallText)
	}

	second := decodeOpenCodePlan(t, runACM(t, dbPath, input, "opencode-context"))
	if strings.Join(first.SummaryRefs, ",") != strings.Join(second.SummaryRefs, ",") {
		t.Fatalf("second plan changed summary roots: %v -> %v", first.SummaryRefs, second.SummaryRefs)
	}
}

func TestReadOpenCodeContextInputRejectsMissingSession(t *testing.T) {
	_, err := readOpenCodeContextInput(strings.NewReader(`{"prompt":"hello"}`))
	if err == nil || !strings.Contains(err.Error(), "session_id is required") {
		t.Fatalf("missing session error = %v", err)
	}
}

func decodeOpenCodePlan(t *testing.T, output string) opencodectx.Plan {
	t.Helper()
	var plan opencodectx.Plan
	if err := json.Unmarshal([]byte(output), &plan); err != nil {
		t.Fatalf("decode plan: %v\n%s", err, output)
	}
	return plan
}
