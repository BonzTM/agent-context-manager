// Package agents adapts each host coding agent (Claude Code, Codex, OpenCode) to
// acm's core. It parses hook payloads into ingestible messages (capture),
// formats recalled context for injection ("push"), and generates the per-agent
// integration assets that acm init writes.
//
// Capture is hook-payload based, plus transcript reconciliation where a hook
// alone cannot see the assistant's turns: Claude Code's Stop hook carries a
// transcript_path, which is re-read (idempotently, keyed on line uuid) to
// capture assistant text; Codex's agent-turn-complete payload carries the
// turn's final assistant message directly.
package agents

import (
	"encoding/json"
	"fmt"

	"github.com/bonztm/agent-context-manager/internal/core"
)

// Hook event names acm understands. These match the event identifiers Claude
// Code and Codex use; OpenCode capture flows through the plugin via acm ingest.
const (
	EventUserPromptSubmit = "UserPromptSubmit"
	EventPostToolUse      = "PostToolUse"
	// EventStop fires when a Claude Code turn ends; its payload carries
	// transcript_path, from which assistant text is reconciled.
	EventStop         = "Stop"
	EventSessionStart = "SessionStart"
	// EventTurnComplete is Codex's agent-turn-complete notify event, which
	// carries the turn's user input messages and the final assistant message.
	EventTurnComplete = "agent-turn-complete"
)

// rawHook is the union of fields acm reads across agents' hook payloads. Unknown
// fields are ignored; absent fields stay zero.
type rawHook struct {
	SessionID            string          `json:"session_id"`
	ThreadID             string          `json:"thread-id"`
	TurnID               string          `json:"turn-id"`
	HookTurnID           string          `json:"turn_id"`
	ToolUseID            string          `json:"tool_use_id"`
	Cwd                  string          `json:"cwd"`
	Prompt               string          `json:"prompt"`
	ToolName             string          `json:"tool_name"`
	ToolInput            json.RawMessage `json:"tool_input"`
	ToolResponse         json.RawMessage `json:"tool_response"`
	ToolOutput           json.RawMessage `json:"tool_output"`
	TranscriptPath       string          `json:"transcript_path"`
	LastAssistantMessage string          `json:"last-assistant-message"`
	InputMessages        []string        `json:"input-messages"`
}

// Capture parses a hook payload for the given agent and event into an ingestion
// request. The returned request may have zero messages (events with nothing to
// capture, e.g. a bare Stop). An error is returned only on malformed JSON.
func Capture(agent core.Agent, event string, payload []byte) (core.IngestRequest, error) {
	var h rawHook
	if err := json.Unmarshal(payload, &h); err != nil {
		return core.IngestRequest{}, fmt.Errorf("agents: parse hook payload: %w", err)
	}

	sessionID := firstNonEmpty(h.SessionID, h.ThreadID)
	req := core.IngestRequest{Agent: agent, SessionID: sessionID}

	switch event {
	case EventUserPromptSubmit:
		if h.Prompt != "" {
			req.Messages = append(req.Messages, core.IngestMessage{
				Role:       core.RoleUser,
				Content:    h.Prompt,
				ExternalID: inputMessageID(firstNonEmpty(h.HookTurnID, h.TurnID), 0),
				Raw:        string(payload),
			})
		}
	case EventPostToolUse:
		if h.ToolName != "" {
			req.Messages = append(req.Messages, core.IngestMessage{
				Role:       core.RoleTool,
				ToolName:   h.ToolName,
				Content:    formatTool(h.ToolName, h.ToolInput, toolOutput(h)),
				ExternalID: h.ToolUseID,
				Raw:        string(payload),
			})
		}
	case EventStop:
		// Stop payloads do not carry the assistant's text, but point at the host
		// transcript; reconcile assistant turns from it.
		// Ingestion dedupes on the transcript line uuid, so re-reads are no-ops.
		if h.TranscriptPath != "" {
			msgs, _, tErr := ReconcileTranscriptAssistant(agent, h.TranscriptPath)
			if tErr != nil {
				return core.IngestRequest{}, tErr
			}
			req.Messages = append(req.Messages, msgs...)
		}
	case EventTurnComplete:
		for i, m := range h.InputMessages {
			if m != "" {
				req.Messages = append(req.Messages, core.IngestMessage{
					Role:       core.RoleUser,
					Content:    m,
					ExternalID: inputMessageID(h.TurnID, i),
				})
			}
		}
		if h.LastAssistantMessage != "" {
			req.Messages = append(req.Messages, core.IngestMessage{
				Role:       core.RoleAssistant,
				Content:    h.LastAssistantMessage,
				ExternalID: h.TurnID, // stable per turn, so re-notify dedupes
			})
		}
	default:
		// SessionStart, Stop, and unknown events carry nothing to capture here.
	}
	return req, nil
}

func inputMessageID(turnID string, index int) string {
	if turnID == "" {
		return ""
	}
	return fmt.Sprintf("%s:input:%d", turnID, index)
}

func toolOutput(h rawHook) json.RawMessage {
	if len(h.ToolResponse) > 0 {
		return h.ToolResponse
	}
	return h.ToolOutput
}

func formatTool(name string, input, output json.RawMessage) string {
	in := "{}"
	if len(input) > 0 {
		in = string(input)
	}
	out := ""
	if len(output) > 0 {
		out = string(output)
	}
	return fmt.Sprintf("[tool %s]\ninput: %s\noutput: %s", name, in, out)
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}
