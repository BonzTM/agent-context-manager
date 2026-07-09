package privacy

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bonztm/agent-context-manager/internal/core"
)

func TestMissingPolicyUsesSecureRedactionDefaults(t *testing.T) {
	policy, err := Load(t.TempDir())
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	request := core.IngestRequest{
		Agent: core.AgentCodex, SessionID: "session",
		Messages: []core.IngestMessage{{
			Role: core.RoleUser, Content: "token sk-abcdefghijklmnopqrstuvwxyz",
			Raw: `{"api_key":"supersecret123"}`,
		}},
	}

	filtered, decision, err := policy.Apply(request)
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	message := filtered.Messages[0]
	if strings.Contains(message.Content, "sk-abcdefghijklmnopqrstuvwxyz") || strings.Contains(message.Raw, "supersecret123") {
		t.Fatalf("secret survived redaction: %+v", message)
	}
	if decision.MessagesRedacted != 1 || !strings.Contains(message.Content, "[REDACTED:api-token]") {
		t.Fatalf("unexpected decision/message: %+v %+v", decision, message)
	}
}

func TestPolicyExcludesSessionsToolsPathsAndClasses(t *testing.T) {
	config := diskPolicy{
		ExcludeSessions: []string{"scratch-*"}, ExcludeTools: []string{"Secret*"},
		ExcludePaths: []string{"/private/*"}, ExcludeContentClasses: []string{"personal-data"},
	}
	policy, err := newPolicy(config)
	if err != nil {
		t.Fatalf("new policy: %v", err)
	}
	assertSessionExcluded(t, policy)
	assertMessageExcluded(t, policy, core.IngestMessage{Role: core.RoleTool, ToolName: "SecretRead", Content: "value"})
	assertMessageExcluded(t, policy, core.IngestMessage{Role: core.RoleTool, ToolName: "Read", Content: "value", Raw: `{"file_path":"/private/key.txt"}`})
	assertMessageExcluded(t, policy, core.IngestMessage{Role: core.RoleUser, Content: "person@example.com"})
}

func TestPolicySupportsExplicitOptOutAndAllowValue(t *testing.T) {
	disabled := false
	policy, err := newPolicy(diskPolicy{Redact: &disabled})
	if err != nil {
		t.Fatalf("new disabled policy: %v", err)
	}
	message := applyOne(t, policy, core.IngestMessage{Role: core.RoleUser, Content: "password=notasecret"})
	if message.Content != "password=notasecret" {
		t.Fatalf("explicit opt-out redacted content: %q", message.Content)
	}
	policy, err = newPolicy(diskPolicy{AllowValues: []string{"notasecret"}})
	if err != nil {
		t.Fatalf("new allow policy: %v", err)
	}
	message = applyOne(t, policy, core.IngestMessage{Role: core.RoleUser, Content: "password=notasecret"})
	if message.Content != "password=notasecret" {
		t.Fatalf("allowed value was redacted: %q", message.Content)
	}
}

func TestSessionModesHaveDeterministicPrecedence(t *testing.T) {
	policy, err := newPolicy(diskPolicy{
		IgnoreSessions: []string{"blocked-*"}, StatelessSessions: []string{"stateless-*", "blocked-*"},
	})
	if err != nil {
		t.Fatalf("new policy: %v", err)
	}
	if policy.Mode("normal") != SessionCapture || policy.Mode("stateless-1") != SessionStateless || policy.Mode("blocked-1") != SessionIgnore {
		t.Fatalf("unexpected modes: normal=%s stateless=%s blocked=%s",
			policy.Mode("normal"), policy.Mode("stateless-1"), policy.Mode("blocked-1"))
	}
}

func TestLoadRejectsInvalidGlobAndUnknownClass(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, FileName), []byte("exclude_sessions = [\"[\"]\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(root); err == nil {
		t.Fatal("Load accepted invalid glob")
	}
	if err := os.WriteFile(filepath.Join(root, FileName), []byte("exclude_content_classes = [\"unknown\"]\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(root); err == nil {
		t.Fatal("Load accepted unknown content class")
	}
}

func assertSessionExcluded(t *testing.T, policy *Policy) {
	t.Helper()
	request := core.IngestRequest{Agent: core.AgentCodex, SessionID: "scratch-1", Messages: []core.IngestMessage{{Role: core.RoleUser, Content: "ignored"}}}
	filtered, decision, err := policy.Apply(request)
	if err != nil || !decision.SessionExcluded || len(filtered.Messages) != 1 {
		t.Fatalf("session exclusion = %+v %+v err=%v", filtered, decision, err)
	}
}

func assertMessageExcluded(t *testing.T, policy *Policy, message core.IngestMessage) {
	t.Helper()
	request := core.IngestRequest{Agent: core.AgentCodex, SessionID: "kept", Messages: []core.IngestMessage{message}}
	filtered, decision, err := policy.Apply(request)
	if err != nil || len(filtered.Messages) != 0 || decision.MessagesExcluded != 1 {
		t.Fatalf("message exclusion = %+v %+v err=%v", filtered, decision, err)
	}
}

func applyOne(t *testing.T, policy *Policy, message core.IngestMessage) core.IngestMessage {
	t.Helper()
	request := core.IngestRequest{Agent: core.AgentCodex, SessionID: "kept", Messages: []core.IngestMessage{message}}
	filtered, _, err := policy.Apply(request)
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	return filtered.Messages[0]
}
