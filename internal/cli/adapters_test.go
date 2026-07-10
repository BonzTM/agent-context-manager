package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bonztm/agent-context-manager/internal/agents"
	"github.com/bonztm/agent-context-manager/internal/core"
	"github.com/bonztm/agent-context-manager/internal/store"
)

func TestHookCapturesAndInjectsRecall(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "acm.db")

	// Seed prior history with an assistant message worth recalling.
	prior := `{"agent":"claude-code","session_id":"prior","messages":[
		{"role":"assistant","content":"use exponential backoff in client.go","external_id":"a1"}
	]}`
	runACM(t, dbPath, prior, "ingest")

	// A new prompt mentioning "backoff" should pull the prior message back via
	// recall (OR match), and the prompt itself should be captured.
	payload := `{"session_id":"cur","prompt":"how do I add backoff retries"}`
	out := runACM(t, dbPath, payload, "hook", "--agent", "claude-code", "--event", "UserPromptSubmit")
	if !strings.Contains(out, "hookSpecificOutput") {
		t.Fatalf("hook output missing hookSpecificOutput:\n%s", out)
	}
	if !strings.Contains(out, "acm-recall") || !strings.Contains(out, "backoff") {
		t.Fatalf("hook output missing recalled context:\n%s", out)
	}

	// The prompt was captured: prior assistant + current user prompt = 2 messages.
	stats := runACM(t, dbPath, "", "stats")
	if !strings.Contains(stats, "messages:      2") {
		t.Fatalf("stats after hook = %q, want 2 messages", stats)
	}
}

func TestHookExcludesCurrentFreshTailFromRecall(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "acm.db")
	current := `{"agent":"codex","session_id":"current","messages":[{"role":"assistant","content":"fresh cobalt strategy should not be reinjected","external_id":"fresh"}]}`
	historical := `{"agent":"codex","session_id":"prior","messages":[{"role":"assistant","content":"historical cobalt migration strategy","external_id":"old"}]}`
	runACM(t, dbPath, current, "ingest")
	runACM(t, dbPath, historical, "ingest")
	payload := `{"session_id":"current","turn_id":"next","prompt":"what was the cobalt migration strategy"}`
	output := runACM(t, dbPath, payload, "hook", "--agent", "codex", "--event", "UserPromptSubmit")
	if !strings.Contains(output, "historical") || !strings.Contains(output, "[cobalt]") {
		t.Fatalf("historical match missing: %q", output)
	}
	if strings.Contains(output, "fresh cobalt") {
		t.Fatalf("fresh-tail match was redundantly injected: %q", output)
	}
}

func TestHookInjectsSummaryOnlyMatchWithExpandGuidance(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "acm.db")
	ctx := context.Background()
	db, err := store.Open(ctx, dbPath)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if migrationErr := db.MigrateUp(ctx); migrationErr != nil {
		_ = db.Close()
		t.Fatalf("migrate: %v", migrationErr)
	}
	sq := store.NewSQLite(db, clock)
	conversation, err := sq.EnsureConversation(ctx, core.ConversationInput{Agent: core.AgentClaude, SessionID: "prior"})
	if err != nil {
		_ = db.Close()
		t.Fatalf("ensure conversation: %v", err)
	}
	message, _, err := sq.AppendMessage(ctx, core.MessageInput{
		ConversationID: conversation.ID, Role: core.RoleAssistant,
		Content: "a decision that uses different source wording", TokenCount: 8, ExternalID: "source",
	})
	if err != nil {
		_ = db.Close()
		t.Fatalf("append source: %v", err)
	}
	summary, err := sq.CreateLeafSummary(ctx, core.LeafSummaryInput{
		ConversationID: conversation.ID, Content: "zephyr cache eviction policy uses LRU",
		TokenCount: 8, SourceMessageIDs: []string{message.ID}, EarliestSeq: message.Seq,
		LatestSeq: message.Seq, DescendantMessageCount: 1,
	})
	if err != nil {
		_ = db.Close()
		t.Fatalf("create summary: %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("close seed database: %v", err)
	}
	payload := `{"session_id":"current","prompt":"what is the zephyr cache eviction policy"}`
	output := runACM(t, dbPath, payload, "hook", "--agent", "claude-code", "--event", "UserPromptSubmit")
	if !strings.Contains(output, summary.ID) || !strings.Contains(output, "acm expand "+summary.ID) {
		t.Fatalf("summary recall missing expand guidance: %q", output)
	}
	if strings.Contains(output, "acm describe "+summary.ID) {
		t.Fatalf("summary recall used message guidance: %q", output)
	}
}

func TestRecallCandidateLimitsAndFlagBounds(t *testing.T) {
	messages, summaries := agents.RecallCandidateLimits(5)
	if messages != 40 || summaries != 10 {
		t.Fatalf("default candidate limits = %d/%d, want 40/10", messages, summaries)
	}
	messages, summaries = agents.RecallCandidateLimits(1)
	if messages+summaries != 10 || summaries != 2 {
		t.Fatalf("small candidate limits = %d/%d, want total 10 with 2 summaries", messages, summaries)
	}
	options := &hookOptions{agentStr: "codex", event: agents.EventUserPromptSubmit, recall: agents.MaxRecallItems + 1}
	if _, _, err := options.captureRequest(strings.NewReader(`{}`), nil); err == nil || !strings.Contains(err.Error(), "between 0 and") {
		t.Fatalf("invalid recall limit error = %v", err)
	}
}

func TestHookPostToolUseCapturesWithoutRecall(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "acm.db")
	payload := `{"session_id":"s","tool_name":"Bash","tool_input":{"command":"go test"},"tool_response":{"stdout":"ok"}}`
	out := runACM(t, dbPath, payload, "hook", "--agent", "claude-code", "--event", "PostToolUse")
	// PostToolUse is capture-only: no recall output.
	if strings.Contains(out, "hookSpecificOutput") {
		t.Fatalf("PostToolUse should not inject recall:\n%s", out)
	}
	stats := runACM(t, dbPath, "", "stats")
	if !strings.Contains(stats, "messages:      1") {
		t.Fatalf("stats = %q, want 1 captured tool message", stats)
	}
}

func TestHookCapturesCodexNotifyArgument(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "acm.db")
	payload := `{"type":"agent-turn-complete","thread-id":"thread-1","turn-id":"turn-1","input-messages":["test prompt"],"last-assistant-message":"test response"}`
	runACM(t, dbPath, "", "hook", "--agent", "codex", "--event", "agent-turn-complete", payload)

	stats := runACM(t, dbPath, "", "stats")
	if !strings.Contains(stats, "messages:      2") {
		t.Fatalf("stats after Codex notify = %q, want user and assistant messages", stats)
	}
}

func TestInitWritesAssets(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "acm.db")
	out := runACM(t, dbPath, "", "init", "claude-code")
	if !strings.Contains(out, "wrote") {
		t.Fatalf("init output = %q, want 'wrote'", out)
	}
	settings := filepath.Join(dir, ".acm", "init", "claude-code", "settings.snippet.json")
	if _, err := os.Stat(settings); err != nil {
		t.Fatalf("expected settings snippet at %s: %v", settings, err)
	}
}

func TestHookStopCapturesTranscriptAndCompacts(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "acm.db")

	// Build a transcript with enough assistant text to exceed a default-config
	// no-op (content volume is irrelevant to the assertion; compaction is a
	// cheap no-op below threshold and must not error either way).
	transcript := filepath.Join(dir, "session.jsonl")
	lines := `{"type":"assistant","uuid":"a1","message":{"role":"assistant","content":[{"type":"text","text":"first answer"}]}}
{"type":"assistant","uuid":"a2","message":{"role":"assistant","content":[{"type":"text","text":"second answer"}]}}`
	if err := os.WriteFile(transcript, []byte(lines), 0o600); err != nil {
		t.Fatal(err)
	}

	payload := fmt.Sprintf(`{"session_id":"s1","transcript_path":%q}`, transcript)
	runACM(t, dbPath, payload, "hook", "--agent", "claude-code", "--event", "Stop")

	stats := runACM(t, dbPath, "", "stats")
	if !strings.Contains(stats, "messages:      2") {
		t.Fatalf("stats after Stop hook = %q, want 2 assistant messages", stats)
	}

	// Re-running the same Stop hook must not duplicate (transcript re-read).
	runACM(t, dbPath, payload, "hook", "--agent", "claude-code", "--event", "Stop")
	stats = runACM(t, dbPath, "", "stats")
	if !strings.Contains(stats, "messages:      2") {
		t.Fatalf("stats after repeated Stop hook = %q, want still 2", stats)
	}
}

func TestCodexBackfillIsDryRunFirstAndIdempotent(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "acm.db")
	transcript := filepath.Join(dir, "rollout.jsonl")
	lines := `{"type":"event_msg","payload":{"type":"task_started","turn_id":"turn-1"}}
{"type":"response_item","payload":{"type":"message","role":"assistant","id":"response-1","content":[{"type":"output_text","text":"recovered answer"}]}}
{"type":"response_item","payload":
{"type":"event_msg","payload":{"type":"task_complete","turn_id":"turn-1"}}`
	if err := os.WriteFile(transcript, []byte(lines), 0o600); err != nil {
		t.Fatal(err)
	}
	rawHook := fmt.Sprintf(`{"session_id":"codex-session","transcript_path":%q}`, transcript)
	payload := fmt.Sprintf(`{"agent":"codex","session_id":"codex-session","messages":[{"role":"user","content":"recover this","external_id":"turn-1:input:0","raw":%q}]}`, rawHook)
	runACM(t, dbPath, payload, "ingest")

	dryRun := runACM(t, dbPath, "", "backfill")
	if !strings.Contains(dryRun, "missing=1 appended=0 malformed=1 mode=dry-run") {
		t.Fatalf("dry-run output = %q", dryRun)
	}
	assertMessageCount(t, dbPath, 1)
	apply := runACM(t, dbPath, "", "backfill", "--apply")
	if !strings.Contains(apply, "missing=1 appended=1 malformed=1 mode=apply") {
		t.Fatalf("apply output = %q", apply)
	}
	assertMessageCount(t, dbPath, 2)
	second := runACM(t, dbPath, "", "backfill", "--apply")
	if !strings.Contains(second, "missing=0 appended=0 malformed=1 mode=apply") {
		t.Fatalf("second apply output = %q", second)
	}
}

func assertMessageCount(t *testing.T, dbPath string, count int) {
	t.Helper()
	stats := runACM(t, dbPath, "", "stats")
	want := fmt.Sprintf("messages:      %d", count)
	if !strings.Contains(stats, want) {
		t.Fatalf("stats = %q, want %q", stats, want)
	}
}

func TestPrivacyPolicyExcludesSessionWithoutRows(t *testing.T) {
	root := t.TempDir()
	dbPath := filepath.Join(root, "acm.db")
	if err := os.WriteFile(filepath.Join(root, ".acm-policy.toml"), []byte("exclude_sessions = [\"private-*\"]\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	payload := `{"agent":"codex","session_id":"private-1","messages":[{"role":"user","content":"do not store"}]}`
	output := runACM(t, dbPath, payload, "ingest")
	if !strings.Contains(output, "excluded 1") {
		t.Fatalf("ingest output = %q", output)
	}
	stats := runACM(t, dbPath, "", "stats")
	if !strings.Contains(stats, "conversations: 0") || !strings.Contains(stats, "messages:      0") {
		t.Fatalf("excluded session created rows: %q", stats)
	}
}

func TestIgnoreAndStatelessSessionModes(t *testing.T) {
	root := t.TempDir()
	dbPath := filepath.Join(root, "acm.db")
	policy := "ignore_sessions = [\"ignored-*\"]\nstateless_sessions = [\"stateless-*\"]\n"
	if err := os.WriteFile(filepath.Join(root, ".acm-policy.toml"), []byte(policy), 0o600); err != nil {
		t.Fatal(err)
	}
	prior := `{"agent":"codex","session_id":"prior","messages":[{"role":"assistant","content":"use cobalt migration strategy","external_id":"a1"}]}`
	runACM(t, dbPath, prior, "ingest")

	ignored := `{"session_id":"ignored-1","turn_id":"turn-i","prompt":"what was the cobalt strategy"}`
	if output := runACM(t, dbPath, ignored, "hook", "--agent", "codex", "--event", "UserPromptSubmit"); output != "" {
		t.Fatalf("ignored session received recall: %q", output)
	}
	stateless := `{"session_id":"stateless-1","turn_id":"turn-s","prompt":"what was the cobalt strategy"}`
	output := runACM(t, dbPath, stateless, "hook", "--agent", "codex", "--event", "UserPromptSubmit")
	if !strings.Contains(output, "acm-recall") || !strings.Contains(output, "cobalt") {
		t.Fatalf("stateless session missed recall: %q", output)
	}
	assertMessageCount(t, dbPath, 1)
}

func TestPrivacyRedactionPrecedesAllPersistence(t *testing.T) {
	root := t.TempDir()
	dbPath := filepath.Join(root, "acm.db")
	secret := "sk-abcdefghijklmnopqrstuvwxyz123456"
	content := "token " + secret + " " + strings.Repeat("payload ", 600)
	raw := `{"api_key":"supersecret123","transcript_path":"/tmp/session"}`
	payload := fmt.Sprintf(`{"agent":"codex","session_id":"redacted","messages":[{"role":"tool","tool_name":"Read","content":%q,"external_id":"tool-1","raw":%q}]}`, content, raw)
	output := runACM(t, dbPath, payload, "ingest")
	if !strings.Contains(output, "redacted 1") {
		t.Fatalf("ingest output = %q", output)
	}
	grepOutput := runACM(t, dbPath, "", "grep", "--substr", "--json", "REDACTED:api-token")
	var results grepResult
	if err := json.Unmarshal([]byte(grepOutput), &results); err != nil || len(results.Messages) != 1 {
		t.Fatalf("redacted grep = %q err=%v", grepOutput, err)
	}
	message := results.Messages[0].Message
	described := runACM(t, dbPath, "", "describe", "--json", message.ID)
	assertSecretAbsent(t, described, secret, "supersecret123")

	runACM(t, dbPath, "", "compact", message.ConversationID,
		"--model-context-tokens", "1000", "--soft-fraction", "0.1", "--fresh-tail", "0", "--fresh-tail-tokens", "0",
		"--leaf-chunk-tokens", "1000", "--leaf-target-tokens", "40", "--condensed-target-tokens", "40",
		"--condense-chunk-tokens", "1000", "--truncate-tokens", "20", "--large-file-threshold", "100")
	search := runACM(t, dbPath, "", "grep", "--substr", secret)
	if !strings.Contains(search, "no matches") {
		t.Fatalf("secret reached FTS or summary: %q", search)
	}
	offload, err := os.ReadFile(filepath.Join(root, "files", message.ConversationID, message.ID+".txt"))
	if err != nil {
		t.Fatalf("read offload: %v", err)
	}
	assertSecretAbsent(t, string(offload), secret, "supersecret123")

	backup := filepath.Join(root, "backup.db")
	runACM(t, dbPath, "", "backup", backup)
	backupMessage := runACM(t, backup, "", "describe", "--json", message.ID)
	assertSecretAbsent(t, backupMessage, secret, "supersecret123")
}

func assertSecretAbsent(t *testing.T, content string, secrets ...string) {
	t.Helper()
	for _, secret := range secrets {
		if strings.Contains(content, secret) {
			t.Fatalf("secret %q persisted in %q", secret, content)
		}
	}
}
