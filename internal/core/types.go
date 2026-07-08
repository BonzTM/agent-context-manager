// Package core holds acm's domain model and the consumer-defined Store contract.
// It depends only on the standard library; the SQLite implementation lives in
// internal/store and imports core, never the other way around.
package core

import "time"

// Agent identifies the host coding agent a conversation belongs to. The values
// are stable string identifiers (not iota) so they round-trip through the
// database and JSON unambiguously.
type Agent string

// Supported host agents.
const (
	AgentClaude   Agent = "claude-code"
	AgentCodex    Agent = "codex"
	AgentOpenCode Agent = "opencode"
)

// Valid reports whether a is a recognized agent.
func (a Agent) Valid() bool {
	switch a {
	case AgentClaude, AgentCodex, AgentOpenCode:
		return true
	default:
		return false
	}
}

// Role is the author role of a message, mirroring the OpenAI/Anthropic message
// roles the host agents emit.
type Role string

// Message roles.
const (
	RoleSystem    Role = "system"
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
	RoleTool      Role = "tool"
)

// Valid reports whether r is a recognized role.
func (r Role) Valid() bool {
	switch r {
	case RoleSystem, RoleUser, RoleAssistant, RoleTool:
		return true
	default:
		return false
	}
}

// Conversation is one agent session. It is keyed in the store by (Agent,
// SessionID); the ID is derived deterministically from that pair so repeated
// ingestion of the same session is idempotent.
type Conversation struct {
	ID        string
	Agent     Agent
	SessionID string
	Title     string
	CreatedAt time.Time
	UpdatedAt time.Time
}

// Message is a single verbatim turn in a conversation. Content is the canonical
// text; Raw, when present, holds the original event JSON so nothing the agent
// emitted is lost. ID and IdentityHash are derived so re-ingesting the same
// source line does not create duplicates.
type Message struct {
	ID             string
	ConversationID string
	Seq            int64
	Role           Role
	Content        string
	TokenCount     int
	ToolName       string
	ExternalID     string
	IdentityHash   string
	Raw            string
	CreatedAt      time.Time
}

// Stats summarizes what the store currently holds.
type Stats struct {
	Conversations int64
	Messages      int64
	TotalTokens   int64
}
