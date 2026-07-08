package core

import (
	"context"
	"errors"
)

// Sentinel errors returned by Store implementations. Callers match with
// errors.Is, never on error strings.
var (
	// ErrNotFound is returned when a requested entity does not exist.
	ErrNotFound = errors.New("core: not found")
	// ErrConflict is returned on a constraint violation that is not an expected
	// idempotent duplicate.
	ErrConflict = errors.New("core: conflict")
)

// SearchMode selects how SearchMessages interprets the query text.
type SearchMode string

// Search modes.
const (
	// SearchFTS (the default) runs a tokenized full-text MATCH ranked by bm25.
	SearchFTS SearchMode = ""
	// SearchSubstr runs a case-insensitive literal substring scan.
	SearchSubstr SearchMode = "substr"
)

// ConversationInput is the upsert payload for EnsureConversation.
type ConversationInput struct {
	Agent     Agent
	SessionID string
	Title     string
}

// MessageInput is the append payload for AppendMessage. TokenCount is supplied
// by the caller (the service computes it via the token counter) so the store
// stays free of counting policy.
type MessageInput struct {
	ConversationID string
	Role           Role
	Content        string
	TokenCount     int
	ToolName       string
	ExternalID     string
	Raw            string
}

// SearchQuery parameterizes SearchMessages. An empty ConversationID searches
// across all conversations.
type SearchQuery struct {
	Text           string
	Mode           SearchMode
	ConversationID string
	Limit          int
	// Any ORs the query terms instead of ANDing them. Use it for broad recall
	// (matching any salient word of a prompt); leave false for precise grep.
	Any bool
}

// SearchHit is one ranked search result.
type SearchHit struct {
	Message Message
	Snippet string
	Score   float64
}

// SummaryHit is one ranked summary search result.
type SummaryHit struct {
	Summary Summary
	Snippet string
	Score   float64
}

// Store is acm's persistence contract, defined here by its consumer (the core
// service) and implemented by internal/store. Methods take a context first and
// wrap storage errors; lookups that miss return ErrNotFound.
type Store interface {
	// EnsureConversation returns the existing conversation for (Agent,
	// SessionID) or creates it. It is idempotent.
	EnsureConversation(ctx context.Context, in ConversationInput) (Conversation, error)
	// ConversationBySession loads a conversation by (Agent, SessionID).
	ConversationBySession(ctx context.Context, agent Agent, sessionID string) (Conversation, error)
	// AppendMessage appends a message, returning it and whether it was newly
	// created (false means an identical message already existed — idempotent).
	AppendMessage(ctx context.Context, in MessageInput) (msg Message, created bool, err error)
	// GetMessage loads a message by ID.
	GetMessage(ctx context.Context, id string) (Message, error)
	// ListMessages returns messages in a conversation with seq greater than
	// afterSeq, ordered by seq ascending, up to limit (limit <= 0 means all).
	ListMessages(ctx context.Context, conversationID string, afterSeq int64, limit int) ([]Message, error)
	// SearchMessages returns ranked hits for the query.
	SearchMessages(ctx context.Context, q SearchQuery) ([]SearchHit, error)
	// Stats reports aggregate counts.
	Stats(ctx context.Context) (Stats, error)
}
