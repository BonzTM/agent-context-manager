package agents

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/bonztm/agent-context-manager/internal/core"
)

// transcriptScanBuffer bounds a single transcript line. Claude Code lines carry
// whole assistant turns including tool payloads, so allow large ones.
const transcriptScanBuffer = 16 * 1024 * 1024

// transcriptLine is the subset of a Claude Code transcript JSONL entry that
// assistant-text capture needs. Unknown fields are ignored.
type transcriptLine struct {
	Type    string `json:"type"`
	UUID    string `json:"uuid"`
	Message struct {
		Role    string `json:"role"`
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
	} `json:"message"`
}

// captureTranscriptAssistant reads a Claude Code session transcript (JSONL) and
// returns the assistant text turns as ingestible messages, keyed on the
// transcript line's uuid so re-reads dedupe. Tool_use and thinking blocks are
// skipped: tool activity is captured by the PostToolUse hook, and thinking is
// not part of the conversation contract.
func captureTranscriptAssistant(path string) ([]core.IngestMessage, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("agents: open transcript: %w", err)
	}
	defer func() { _ = f.Close() }()

	var out []core.IngestMessage
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), transcriptScanBuffer)
	for sc.Scan() {
		raw := sc.Bytes()
		if len(raw) == 0 {
			continue
		}
		var line transcriptLine
		if uErr := json.Unmarshal(raw, &line); uErr != nil {
			continue // non-JSON or foreign lines are not ours to fail on
		}
		if line.Type != "assistant" || line.Message.Role != "assistant" {
			continue
		}
		var parts []string
		for _, c := range line.Message.Content {
			if c.Type == "text" && c.Text != "" {
				parts = append(parts, c.Text)
			}
		}
		if len(parts) == 0 {
			continue
		}
		out = append(out, core.IngestMessage{
			Role:       core.RoleAssistant,
			Content:    strings.Join(parts, "\n"),
			ExternalID: line.UUID,
		})
	}
	if err := sc.Err(); err != nil {
		return nil, fmt.Errorf("agents: read transcript: %w", err)
	}
	return out, nil
}
