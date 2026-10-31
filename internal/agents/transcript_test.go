package agents

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/klauspost/compress/zstd"

	"github.com/bonztm/agent-context-manager/internal/core"
)

const oneTurnRollout = `{"type":"event_msg","payload":{"type":"task_started","turn_id":"turn-1"}}
{"type":"response_item","payload":{"type":"message","role":"assistant","id":"a1","content":[{"type":"output_text","text":"compressed answer"}]}}
{"type":"event_msg","payload":{"type":"task_complete","turn_id":"turn-1"}}
`

func writeZstd(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	encoder, err := zstd.NewWriter(nil)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = encoder.Close() }()
	if err := os.WriteFile(path, encoder.EncodeAll([]byte(content), nil), 0o600); err != nil {
		t.Fatal(err)
	}
}

func assertOneTurn(t *testing.T, messages []core.IngestMessage, err error) {
	t.Helper()
	if err != nil {
		t.Fatalf("reconcile: %v", err)
	}
	if len(messages) != 1 || messages[0].ExternalID != "turn-1" || messages[0].Content != "compressed answer" {
		t.Fatalf("unexpected messages: %+v", messages)
	}
}

func TestCodexRolloutReadsCompressedCopy(t *testing.T) {
	recorded := filepath.Join(t.TempDir(), "sessions", "2026", "06", "25", "rollout-x.jsonl")
	writeZstd(t, recorded+".zst", oneTurnRollout)

	messages, _, err := ReconcileTranscriptAssistant(core.AgentCodex, recorded)
	assertOneTurn(t, messages, err)
}

func TestCodexRolloutReadsArchivedCopy(t *testing.T) {
	home := t.TempDir()
	recorded := filepath.Join(home, "sessions", "2026", "06", "25", "rollout-x.jsonl")
	writeZstd(t, filepath.Join(home, "archived_sessions", "rollout-x.jsonl.zst"), oneTurnRollout)

	messages, _, err := ReconcileTranscriptAssistant(core.AgentCodex, recorded)
	assertOneTurn(t, messages, err)
}

func TestMissingRolloutWrapsNotExist(t *testing.T) {
	recorded := filepath.Join(t.TempDir(), "sessions", "2026", "06", "25", "gone.jsonl")

	_, _, err := ReconcileTranscriptAssistant(core.AgentCodex, recorded)
	if !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("err = %v, want os.ErrNotExist", err)
	}
}

func TestCorruptCompressedRolloutIsAnError(t *testing.T) {
	recorded := filepath.Join(t.TempDir(), "rollout-x.jsonl")
	if err := os.WriteFile(recorded+".zst", []byte("not zstd"), 0o600); err != nil {
		t.Fatal(err)
	}

	_, _, err := ReconcileTranscriptAssistant(core.AgentCodex, recorded)
	if err == nil || errors.Is(err, os.ErrNotExist) {
		t.Fatalf("err = %v, want a read error that is not os.ErrNotExist", err)
	}
}

func TestClaudeTranscriptHasNoCodexFallbacks(t *testing.T) {
	recorded := filepath.Join(t.TempDir(), "transcript.jsonl")
	writeZstd(t, recorded+".zst", oneTurnRollout)

	_, _, err := ReconcileTranscriptAssistant(core.AgentClaude, recorded)
	if !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("err = %v, want os.ErrNotExist", err)
	}
}
