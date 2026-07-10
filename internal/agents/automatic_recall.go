package agents

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/bonztm/agent-context-manager/internal/core"
)

// AutomaticRecallStore is the bounded retrieval surface used by host adapters.
type AutomaticRecallStore interface {
	RecentConversationalMessageIDs(ctx context.Context, conversationID string, minMessages, minTokens, maxRows int) ([]string, error)
	SearchMessages(ctx context.Context, query core.SearchQuery) ([]core.SearchHit, error)
	SearchSummaries(ctx context.Context, query core.SearchQuery) ([]core.SummaryHit, error)
}

// AutomaticRecallRequest supplies all policy and time inputs explicitly.
type AutomaticRecallRequest struct {
	Agent             core.Agent
	SessionID         string
	Prompt            string
	Limit             int
	FreshTailMessages int
	FreshTailTokens   int
	Now               time.Time
}

// AutomaticRecall searches, excludes the current tail, and ranks combined hits.
func AutomaticRecall(ctx context.Context, store AutomaticRecallStore, request AutomaticRecallRequest) ([]RecallHit, error) {
	if request.Limit < 0 || request.Limit > MaxRecallItems || request.FreshTailMessages < 0 || request.FreshTailTokens < 0 {
		return nil, errors.New("agents: invalid automatic recall limits")
	}
	terms := RecallTerms(request.Prompt)
	if request.Limit == 0 || len(terms) == 0 {
		return nil, nil
	}
	conversationID := core.DeriveConversationID(request.Agent, request.SessionID)
	excluded, err := automaticFreshTail(ctx, store, conversationID, request)
	if err != nil {
		return nil, err
	}
	messageLimit, summaryLimit := RecallCandidateLimits(request.Limit)
	query := core.SearchQuery{Text: strings.Join(terms, " "), Limit: messageLimit, Any: true}
	messages, err := store.SearchMessages(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("agents: search messages: %w", err)
	}
	query.Limit = summaryLimit
	summaries, err := store.SearchSummaries(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("agents: search summaries: %w", err)
	}
	candidates := MessageRecallHits(messages, excluded)
	candidates = append(candidates, SummaryRecallHits(summaries)...)
	return RankRecall(candidates, terms, conversationID, request.Now, request.Limit), nil
}

func automaticFreshTail(ctx context.Context, store AutomaticRecallStore, conversationID string, request AutomaticRecallRequest) (map[string]struct{}, error) {
	ids, err := store.RecentConversationalMessageIDs(
		ctx, conversationID, request.FreshTailMessages, request.FreshTailTokens, MaxFreshTailRows,
	)
	if err != nil {
		return nil, fmt.Errorf("agents: load fresh tail: %w", err)
	}
	excluded := make(map[string]struct{}, len(ids))
	for _, id := range ids {
		excluded[id] = struct{}{}
	}
	return excluded, nil
}

// RecallCandidateLimits splits the bounded candidate budget by result limit.
func RecallCandidateLimits(limit int) (messages, summaries int) {
	total := min(MaxRecallCandidates, max(limit, 1)*10)
	summaries = min(MaxSummaryCandidates, max(total/5, 1))
	return total - summaries, summaries
}
