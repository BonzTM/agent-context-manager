package agents

import (
	"bufio"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/bonztm/agent-context-manager/internal/core"
)

// transcriptScanBuffer bounds a single transcript line. Claude Code lines carry
// whole assistant turns including tool payloads, so allow large ones.
const (
	transcriptScanBuffer = 16 * 1024 * 1024
	transcriptMaxLines   = 100_000
)

// TranscriptReport describes bounded reconciliation work without exposing
// transcript contents in logs or diagnostics.
type TranscriptReport struct {
	Lines        int
	Malformed    int
	Assistant    int
	LimitReached bool
}

// transcriptLine is the subset of a Claude Code transcript JSONL entry that
// assistant-text capture needs. Unknown fields are ignored.
type transcriptLine struct {
	Type    string `json:"type"`
	UUID    string `json:"uuid"`
	Message struct {
		Role    string        `json:"role"`
		Content []textContent `json:"content"`
	} `json:"message"`
}

type textContent struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type codexTranscriptLine struct {
	Type    string `json:"type"`
	Payload struct {
		Type    string        `json:"type"`
		Role    string        `json:"role"`
		ID      string        `json:"id"`
		Content []textContent `json:"content"`
	} `json:"payload"`
}

type codexEventLine struct {
	Type    string `json:"type"`
	Payload struct {
		Type   string `json:"type"`
		TurnID string `json:"turn_id"`
	} `json:"payload"`
}

type codexTranscriptState struct {
	turnID  string
	pending core.IngestMessage
}

// ReconcileTranscriptAssistant reads a supported host transcript and returns
// final assistant text with stable source IDs plus bounded scan diagnostics.
func ReconcileTranscriptAssistant(agent core.Agent, path string) ([]core.IngestMessage, TranscriptReport, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, TranscriptReport{}, fmt.Errorf("agents: open transcript: %w", err)
	}
	defer func() { _ = f.Close() }()

	var out []core.IngestMessage
	var report TranscriptReport
	var codexState codexTranscriptState
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), transcriptScanBuffer)
	for sc.Scan() {
		if report.Lines == transcriptMaxLines {
			report.LimitReached = true
			break
		}
		report.Lines++
		raw := sc.Bytes()
		if len(raw) == 0 {
			continue
		}
		message, relevant, malformed := parseTranscriptRecord(agent, raw, &codexState)
		if malformed {
			report.Malformed++
			continue
		}
		if !relevant {
			continue
		}
		out = append(out, message)
		report.Assistant++
	}
	if err := sc.Err(); err != nil {
		return nil, report, fmt.Errorf("agents: read transcript: %w", err)
	}
	if message, ok := codexState.finish(); agent == core.AgentCodex && ok {
		out = append(out, message)
		report.Assistant++
	}
	return out, report, nil
}

func parseTranscriptRecord(agent core.Agent, raw []byte, codexState *codexTranscriptState) (core.IngestMessage, bool, bool) {
	switch agent {
	case core.AgentClaude:
		return parseClaudeTranscriptLine(raw)
	case core.AgentCodex:
		return codexState.consume(raw)
	default:
		return core.IngestMessage{}, false, false
	}
}

func parseClaudeTranscriptLine(raw []byte) (core.IngestMessage, bool, bool) {
	var line transcriptLine
	if err := json.Unmarshal(raw, &line); err != nil {
		return core.IngestMessage{}, false, true
	}
	if line.Type != "assistant" || line.Message.Role != "assistant" {
		return core.IngestMessage{}, false, false
	}
	parts := textParts(line.Message.Content)
	return transcriptMessage(parts, line.UUID, raw)
}

func (state *codexTranscriptState) consume(raw []byte) (core.IngestMessage, bool, bool) {
	var envelope struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return core.IngestMessage{}, false, true
	}
	switch envelope.Type {
	case "event_msg":
		return state.consumeEvent(raw)
	case "response_item":
		return state.consumeResponse(raw)
	default:
		return core.IngestMessage{}, false, false
	}
}

func (state *codexTranscriptState) consumeEvent(raw []byte) (core.IngestMessage, bool, bool) {
	var line codexEventLine
	if err := json.Unmarshal(raw, &line); err != nil {
		return core.IngestMessage{}, false, true
	}
	switch line.Payload.Type {
	case "task_started":
		state.turnID = line.Payload.TurnID
		state.pending = core.IngestMessage{}
	case "task_complete":
		return state.finishResult()
	}
	return core.IngestMessage{}, false, false
}

func (state *codexTranscriptState) consumeResponse(raw []byte) (core.IngestMessage, bool, bool) {
	var line codexTranscriptLine
	if err := json.Unmarshal(raw, &line); err != nil {
		return core.IngestMessage{}, false, true
	}
	if line.Type != "response_item" || line.Payload.Type != "message" || line.Payload.Role != "assistant" {
		return core.IngestMessage{}, false, false
	}
	parts := textParts(line.Payload.Content)
	message, relevant, malformed := transcriptMessage(parts, line.Payload.ID, raw)
	if relevant {
		state.pending = message
	}
	return core.IngestMessage{}, false, malformed
}

func (state *codexTranscriptState) finishResult() (core.IngestMessage, bool, bool) {
	message, ok := state.finish()
	return message, ok, false
}

func (state *codexTranscriptState) finish() (core.IngestMessage, bool) {
	if state.pending.Content == "" {
		return core.IngestMessage{}, false
	}
	if state.turnID != "" {
		state.pending.ExternalID = state.turnID
	}
	message := state.pending
	state.pending = core.IngestMessage{}
	state.turnID = ""
	return message, true
}

func textParts(content []textContent) []string {
	parts := make([]string, 0, len(content))
	for _, item := range content {
		if item.Type == "text" || item.Type == "output_text" {
			if item.Text != "" {
				parts = append(parts, item.Text)
			}
		}
	}
	return parts
}

func transcriptMessage(parts []string, externalID string, raw []byte) (core.IngestMessage, bool, bool) {
	if len(parts) == 0 {
		return core.IngestMessage{}, false, false
	}
	if externalID == "" {
		externalID = fmt.Sprintf("transcript-%x", sha256.Sum256(raw))
	}
	return core.IngestMessage{
		Role: core.RoleAssistant, Content: strings.Join(parts, "\n"), ExternalID: externalID,
	}, true, false
}
