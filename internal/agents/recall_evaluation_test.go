package agents

import (
	"encoding/json"
	"os"
	"slices"
	"testing"
	"time"

	"github.com/bonztm/agent-context-manager/internal/core"
)

type recallCorpus struct {
	Clock            time.Time          `json:"clock"`
	MinimumRecallAtK float64            `json:"minimum_recall_at_k"`
	MinimumMRR       float64            `json:"minimum_mrr"`
	Cases            []recallCorpusCase `json:"cases"`
}

type recallCorpusCase struct {
	Name                string                  `json:"name"`
	Prompt              string                  `json:"prompt"`
	CurrentConversation string                  `json:"current_conversation"`
	Limit               int                     `json:"limit"`
	Candidates          []recallCorpusCandidate `json:"candidates"`
	ExpectedTopK        []string                `json:"expected_top_k"`
	RelevantIDs         []string                `json:"relevant_ids"`
}

type recallCorpusCandidate struct {
	Kind           RecallKind `json:"kind"`
	ID             string     `json:"id"`
	ConversationID string     `json:"conversation_id"`
	Role           core.Role  `json:"role"`
	Content        string     `json:"content"`
	TokenCount     int        `json:"token_count"`
	Seq            int64      `json:"seq"`
	Depth          int        `json:"depth"`
	EarliestSeq    int64      `json:"earliest_seq"`
	LatestSeq      int64      `json:"latest_seq"`
	CreatedAt      time.Time  `json:"created_at"`
	Active         bool       `json:"active"`
}

func TestRecallCorpusBaseline(t *testing.T) {
	corpus := loadRecallCorpus(t)
	if len(corpus.Cases) == 0 || len(corpus.Cases) > 32 {
		t.Fatalf("corpus cases = %d, want 1..32", len(corpus.Cases))
	}
	recallTotal, reciprocalRankTotal := 0.0, 0.0
	expectedRoles := make(map[core.Role]bool)
	expectedSummary := false
	for _, testCase := range corpus.Cases {
		if len(testCase.Candidates) > MaxRecallCandidates || testCase.Limit > MaxRecallItems {
			t.Fatalf("case %s exceeds recall bounds", testCase.Name)
		}
		hits := fixtureRecallHits(testCase.Candidates)
		terms := RecallTerms(testCase.Prompt)
		ranked := RankRecall(hits, terms, testCase.CurrentConversation, corpus.Clock, testCase.Limit)
		ids := recallIDs(ranked)
		if !slices.Equal(ids, testCase.ExpectedTopK) {
			t.Errorf("case %s top-k = %v, want %v", testCase.Name, ids, testCase.ExpectedTopK)
		}
		recall, reciprocalRank := retrievalMetrics(ids, testCase.RelevantIDs)
		recallTotal += recall
		reciprocalRankTotal += reciprocalRank
		markExpectedKinds(testCase, expectedRoles, &expectedSummary)
		second := recallIDs(RankRecall(hits, terms, testCase.CurrentConversation, corpus.Clock, testCase.Limit))
		if !slices.Equal(ids, second) {
			t.Errorf("case %s ranking is nondeterministic: %v vs %v", testCase.Name, ids, second)
		}
	}
	recallAtK := recallTotal / float64(len(corpus.Cases))
	mrr := reciprocalRankTotal / float64(len(corpus.Cases))
	if recallAtK < corpus.MinimumRecallAtK || mrr < corpus.MinimumMRR {
		t.Fatalf("recall baseline regressed: Recall@k=%.3f MRR=%.3f, minimum %.3f/%.3f",
			recallAtK, mrr, corpus.MinimumRecallAtK, corpus.MinimumMRR)
	}
	for _, role := range []core.Role{core.RoleSystem, core.RoleUser, core.RoleAssistant, core.RoleTool} {
		if !expectedRoles[role] {
			t.Errorf("corpus has no expected top-k result for role %s", role)
		}
	}
	if !expectedSummary {
		t.Error("corpus has no expected summary result")
	}
	t.Logf("recall corpus: cases=%d Recall@k=%.3f MRR=%.3f", len(corpus.Cases), recallAtK, mrr)
}

func loadRecallCorpus(t *testing.T) recallCorpus {
	t.Helper()
	content, err := os.ReadFile("testdata/recall_corpus.json")
	if err != nil {
		t.Fatalf("read recall corpus: %v", err)
	}
	var corpus recallCorpus
	if err := json.Unmarshal(content, &corpus); err != nil {
		t.Fatalf("decode recall corpus: %v", err)
	}
	return corpus
}

func fixtureRecallHits(candidates []recallCorpusCandidate) []RecallHit {
	hits := make([]RecallHit, 0, len(candidates))
	for rank, candidate := range candidates {
		hits = append(hits, RecallHit{
			Kind: candidate.Kind, ID: candidate.ID, ConversationID: candidate.ConversationID,
			Role: candidate.Role, Content: candidate.Content, Snippet: candidate.Content,
			TokenCount: candidate.TokenCount, Seq: candidate.Seq, Depth: candidate.Depth,
			EarliestSeq: candidate.EarliestSeq, LatestSeq: candidate.LatestSeq,
			CreatedAt: candidate.CreatedAt, Active: candidate.Active, SourceRank: rank,
		})
	}
	return hits
}

func retrievalMetrics(ranked, relevant []string) (recall, reciprocalRank float64) {
	if len(relevant) == 0 {
		return 1, 1
	}
	found := 0
	for rank, id := range ranked {
		if !slices.Contains(relevant, id) {
			continue
		}
		found++
		if reciprocalRank == 0 {
			reciprocalRank = 1 / float64(rank+1)
		}
	}
	return float64(found) / float64(len(relevant)), reciprocalRank
}

func markExpectedKinds(testCase recallCorpusCase, roles map[core.Role]bool, summary *bool) {
	for _, expected := range testCase.ExpectedTopK {
		for _, candidate := range testCase.Candidates {
			if candidate.ID != expected {
				continue
			}
			if candidate.Kind == RecallSummary {
				*summary = true
			} else {
				roles[candidate.Role] = true
			}
		}
	}
}
