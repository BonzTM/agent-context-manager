package core

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"
)

// Clock is the time seam: production wires SystemClock, tests a fake. It is kept
// here (consumer-defined) so core never reads the wall clock ambiently.
type Clock interface {
	Now() time.Time
}

// SystemClock is the production Clock. It always reports UTC so persisted
// timestamps are timezone-free.
type SystemClock struct{}

// Now returns the current UTC time.
func (SystemClock) Now() time.Time { return time.Now().UTC() }

// TokenCounter estimates the token cost of a string. The interface is
// consumer-defined here; internal/tokens supplies the implementation.
type TokenCounter interface {
	Count(string) int
}

// Service is acm's application core: it turns ingestion requests into stored,
// token-counted, deduplicated messages and answers retrieval queries. It holds
// no global state and is safe for sequential use per process invocation.
type Service struct {
	store   Store
	clock   Clock
	counter TokenCounter
	logger  *slog.Logger
}

// NewService wires the core service. logger may be nil (a no-op logger is used).
func NewService(store Store, clock Clock, counter TokenCounter, logger *slog.Logger) *Service {
	if logger == nil {
		logger = slog.New(slog.DiscardHandler)
	}
	return &Service{store: store, clock: clock, counter: counter, logger: logger}
}

// IngestMessage is one message in an ingestion request, in source order.
type IngestMessage struct {
	Role       Role
	Content    string
	ToolName   string
	ExternalID string
	Raw        string
}

// IngestRequest captures one or more messages for a single agent session.
type IngestRequest struct {
	Agent     Agent
	SessionID string
	Title     string
	Messages  []IngestMessage
}

// IngestResult reports what an ingestion produced.
type IngestResult struct {
	ConversationID string
	Appended       int // newly stored messages
	Deduped        int // messages that already existed
	Tokens         int // tokens across newly stored messages
}

// Ingest ensures the conversation exists and appends each message, computing
// token counts and skipping duplicates. It is safe to call repeatedly with
// overlapping message sets (e.g. transcript re-reads): only new messages land.
func (s *Service) Ingest(ctx context.Context, req IngestRequest) (IngestResult, error) {
	if !req.Agent.Valid() {
		return IngestResult{}, fmt.Errorf("ingest: invalid agent %q", req.Agent)
	}
	if req.SessionID == "" {
		return IngestResult{}, errors.New("ingest: empty session id")
	}

	conv, err := s.store.EnsureConversation(ctx, ConversationInput{
		Agent:     req.Agent,
		SessionID: req.SessionID,
		Title:     req.Title,
	})
	if err != nil {
		return IngestResult{}, fmt.Errorf("ingest: ensure conversation: %w", err)
	}

	var res IngestResult
	res.ConversationID = conv.ID
	for i, m := range req.Messages {
		if !m.Role.Valid() {
			return IngestResult{}, fmt.Errorf("ingest: message %d has invalid role %q", i, m.Role)
		}
		tokens := s.counter.Count(m.Content)
		msg, created, aErr := s.store.AppendMessage(ctx, MessageInput{
			ConversationID: conv.ID,
			Role:           m.Role,
			Content:        m.Content,
			TokenCount:     tokens,
			ToolName:       m.ToolName,
			ExternalID:     m.ExternalID,
			Raw:            m.Raw,
		})
		if aErr != nil {
			return IngestResult{}, fmt.Errorf("ingest: append message %d: %w", i, aErr)
		}
		if created {
			res.Appended++
			res.Tokens += msg.TokenCount
		} else {
			res.Deduped++
		}
	}
	s.logger.Debug("ingested",
		"conversation", conv.ID, "agent", req.Agent,
		"appended", res.Appended, "deduped", res.Deduped, "tokens", res.Tokens)
	return res, nil
}

// Search returns ranked message hits for the query.
func (s *Service) Search(ctx context.Context, q SearchQuery) ([]SearchHit, error) {
	hits, err := s.store.SearchMessages(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("search: %w", err)
	}
	return hits, nil
}

// DescribeMessage loads a single message by ID for drill-down.
func (s *Service) DescribeMessage(ctx context.Context, id string) (Message, error) {
	msg, err := s.store.GetMessage(ctx, id)
	if err != nil {
		return Message{}, fmt.Errorf("describe: %w", err)
	}
	return msg, nil
}

// Stats reports aggregate store counts.
func (s *Service) Stats(ctx context.Context) (Stats, error) {
	st, err := s.store.Stats(ctx)
	if err != nil {
		return Stats{}, fmt.Errorf("stats: %w", err)
	}
	return st, nil
}
