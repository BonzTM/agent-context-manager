// Package agentmap runs bounded, read-only tool-using host agents for map items.
package agentmap

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/bonztm/agent-context-manager/internal/summarize"
)

const (
	defaultModel        = "gpt-5.4-mini"
	hardMaxTools        = 100
	hardMaxTurns        = 50
	hardItemTimeout     = 30 * time.Minute
	maxEventLineBytes   = 2 << 20
	baseEventBudget     = 512
	toolEventMultiplier = 8
	turnEventMultiplier = 16
)

// Host identifies a supported tool-using agent CLI.
type Host string

const (
	// HostClaude runs Claude Code with read-only tools and plan permissions.
	HostClaude Host = "claude-agent"
	// HostCodex runs Codex with its read-only sandbox.
	HostCodex Host = "codex-agent"
)

// Request describes one bounded agentic map attempt.
type Request struct {
	Host       Host
	Prompt     string
	SchemaPath string
	MaxTools   int
	MaxTurns   int
	Timeout    time.Duration
}

// Run executes one tool-using agent and returns its final response.
func Run(ctx context.Context, request Request) (json.RawMessage, error) {
	if err := request.Validate(); err != nil {
		return nil, err
	}
	argv, stdin, err := request.command()
	if err != nil {
		return nil, err
	}
	state := newEventState(request)
	maxEvents := baseEventBudget + request.MaxTools*toolEventMultiplier + request.MaxTurns*turnEventMultiplier
	err = summarize.ExecJSONLines(ctx, argv, stdin, request.Timeout, maxEvents, maxEventLineBytes, state.consume)
	if err != nil {
		return nil, fmt.Errorf("agentmap: %s: %w", request.Host, err)
	}
	if len(state.output) == 0 {
		return nil, fmt.Errorf("agentmap: %s returned no final response", request.Host)
	}
	return json.RawMessage(state.output), nil
}

// Validate rejects unsupported hosts and unbounded execution limits.
func (request Request) Validate() error {
	if request.Host != HostClaude && request.Host != HostCodex {
		return fmt.Errorf("agentmap: unsupported host %q", request.Host)
	}
	if request.MaxTools < 1 || request.MaxTools > hardMaxTools {
		return fmt.Errorf("agentmap: max tools must be 1..%d", hardMaxTools)
	}
	if request.MaxTurns < 1 || request.MaxTurns > hardMaxTurns {
		return fmt.Errorf("agentmap: max turns must be 1..%d", hardMaxTurns)
	}
	if request.Timeout <= 0 || request.Timeout > hardItemTimeout {
		return fmt.Errorf("agentmap: item timeout must be positive and at most %s", hardItemTimeout)
	}
	return nil
}

func (request Request) command() ([]string, string, error) {
	switch request.Host {
	case HostClaude:
		return claudeCommand(request), request.Prompt, nil
	case HostCodex:
		return codexCommand(request), request.Prompt, nil
	default:
		return nil, "", fmt.Errorf("agentmap: unsupported host %q", request.Host)
	}
}

func claudeCommand(request Request) []string {
	return []string{
		"claude", "-p", "--model", "haiku", "--output-format", "stream-json", "--verbose",
		"--permission-mode", "plan", "--safe-mode", "--strict-mcp-config",
		"--mcp-config", `{"mcpServers":{}}`, "--disable-slash-commands", "--no-chrome",
		"--no-session-persistence", "--tools", "Read,Glob,Grep",
		"--disallowedTools", "Edit,Write,NotebookEdit", "--max-turns", strconv.Itoa(request.MaxTurns),
	}
}

func codexCommand(request Request) []string {
	argv := []string{
		"codex", "exec", "--sandbox", "read-only", "--ephemeral", "--json", "--color", "never",
		"--strict-config", "--ignore-user-config", "--ignore-rules", "--skip-git-repo-check",
		"-m", defaultModel,
	}
	if request.SchemaPath != "" {
		argv = append(argv, "--output-schema", request.SchemaPath)
	}
	return append(argv, "-")
}

type eventState struct {
	host               Host
	maxTools, maxTurns int
	tools, turns       int
	output             string
	seenTurns          map[string]struct{}
}

func newEventState(request Request) *eventState {
	return &eventState{
		host: request.Host, maxTools: request.MaxTools, maxTurns: request.MaxTurns,
		seenTurns: make(map[string]struct{}, request.MaxTurns),
	}
}

func (state *eventState) consume(line []byte) error {
	var event hostEvent
	if err := json.Unmarshal(line, &event); err != nil {
		return fmt.Errorf("decode host event: %w", err)
	}
	switch state.host {
	case HostClaude:
		return state.consumeClaude(event)
	case HostCodex:
		return state.consumeCodex(event)
	default:
		return errors.New("agentmap: event host was not validated")
	}
}

func (state *eventState) consumeClaude(event hostEvent) error {
	if event.Type == "assistant" && event.Message != nil {
		if _, exists := state.seenTurns[event.Message.ID]; !exists {
			state.seenTurns[event.Message.ID] = struct{}{}
			state.turns++
		}
		for _, content := range event.Message.Content {
			if content.Type == "tool_use" {
				state.tools++
			}
		}
	}
	if event.Type == "result" {
		if event.IsError {
			return errors.New("claude agent returned an error result")
		}
		state.output = event.Result
		state.turns = max(state.turns, event.NumTurns)
	}
	return state.checkLimits()
}

func (state *eventState) consumeCodex(event hostEvent) error {
	if event.Type == "turn.started" {
		state.turns++
	}
	if event.Type == "item.started" && event.Item != nil && isToolItem(event.Item.Type) {
		state.tools++
	}
	if event.Type == "item.completed" && event.Item != nil && event.Item.Type == "agent_message" {
		state.output = event.Item.Text
	}
	return state.checkLimits()
}

func (state *eventState) checkLimits() error {
	if state.tools > state.maxTools {
		return fmt.Errorf("agent tool calls exceed %d", state.maxTools)
	}
	if state.turns > state.maxTurns {
		return fmt.Errorf("agent turns exceed %d", state.maxTurns)
	}
	return nil
}

func isToolItem(itemType string) bool {
	switch itemType {
	case "agent_message", "reasoning", "plan":
		return false
	default:
		return true
	}
}

type hostEvent struct {
	Type     string       `json:"type"`
	Result   string       `json:"result"`
	NumTurns int          `json:"num_turns"`
	IsError  bool         `json:"is_error"`
	Item     *hostItem    `json:"item"`
	Message  *hostMessage `json:"message"`
}

type hostItem struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type hostMessage struct {
	ID      string        `json:"id"`
	Content []hostContent `json:"content"`
}

type hostContent struct {
	Type string `json:"type"`
}
