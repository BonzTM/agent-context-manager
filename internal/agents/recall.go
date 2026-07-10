package agents

import (
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"time"
	"unicode"

	"github.com/bonztm/agent-context-manager/internal/core"
)

// RecallKind identifies the drill-down contract for one automatic recall hit.
type RecallKind string

const (
	// RecallMessage points to one verbatim message.
	RecallMessage RecallKind = "message"
	// RecallSummary points to one expandable summary DAG node.
	RecallSummary RecallKind = "summary"
)

// RecallHit is the common deterministic ranking shape for messages and summaries.
type RecallHit struct {
	Kind           RecallKind
	ID             string
	ConversationID string
	Role           core.Role
	Content        string
	Snippet        string
	TokenCount     int
	Seq            int64
	Depth          int
	EarliestSeq    int64
	LatestSeq      int64
	CreatedAt      time.Time
	Active         bool
	SourceRank     int
}

const (
	// MaxRecallItems is the hard injected-result bound.
	MaxRecallItems = 10
	// MaxRecallCandidates is the combined message/summary search bound.
	MaxRecallCandidates = 50
	// MaxSummaryCandidates is the summary share of the candidate bound.
	MaxSummaryCandidates = 10
	// MaxSummaryResults prevents summary nodes from flooding injected context.
	MaxSummaryResults = 2
	// MaxFreshTailRows bounds identity-only current-tail discovery.
	MaxFreshTailRows      = 4096
	maxRecallSnippetRunes = 300
)

// RecallBlock formats ranked message and summary hits for hook injection.
func RecallBlock(hits []RecallHit) string {
	if len(hits) == 0 {
		return ""
	}
	lines := make([]string, 0, len(hits)+3)
	lines = append(lines,
		"<acm-recall>",
		"Relevant earlier context from this project's history. Drill down for the",
		"source with acm describe <msg-id> or acm expand <sum-id>.",
	)
	for _, hit := range hits {
		lines = append(lines, recallLine(hit))
	}
	lines = append(lines, "</acm-recall>")
	return strings.Join(lines, "\n")
}

func recallLine(hit RecallHit) string {
	snippet := truncateRunes(oneLine(hit.Snippet), maxRecallSnippetRunes)
	if hit.Kind == RecallSummary {
		return fmt.Sprintf("- [%s depth=%d seq=%d-%d; acm expand %s] %s",
			hit.ID, hit.Depth, hit.EarliestSeq, hit.LatestSeq, hit.ID, snippet)
	}
	return fmt.Sprintf("- [%s seq=%d %s; acm describe %s] %s",
		hit.ID, hit.Seq, hit.Role, hit.ID, snippet)
}

const maxRecallTerms = 12

// RecallTerms extracts a bounded set of useful search terms from a prompt.
// Low-signal prompts produce no terms, which suppresses noisy recall injection.
func RecallTerms(prompt string) []string {
	fields := strings.FieldsFunc(prompt, func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsNumber(r)
	})
	terms := make([]string, 0, min(len(fields), maxRecallTerms))
	for _, field := range fields {
		term := strings.ToLower(field)
		if !useRecallTerm(term) || containsTerm(terms, term) {
			continue
		}
		terms = append(terms, term)
		if len(terms) == maxRecallTerms {
			break
		}
	}
	if len(terms) < 2 {
		return nil
	}
	return terms
}

// MessageRecallHits converts message search output and excludes current-tail IDs.
func MessageRecallHits(hits []core.SearchHit, excluded map[string]struct{}) []RecallHit {
	out := make([]RecallHit, 0, len(hits))
	for rank, hit := range hits {
		if _, skip := excluded[hit.Message.ID]; skip {
			continue
		}
		out = append(out, RecallHit{
			Kind: RecallMessage, ID: hit.Message.ID, ConversationID: hit.Message.ConversationID,
			Role: hit.Message.Role, Content: hit.Message.Content, Snippet: hit.Snippet,
			TokenCount: hit.Message.TokenCount, Seq: hit.Message.Seq,
			CreatedAt: hit.Message.CreatedAt, SourceRank: rank,
		})
	}
	return out
}

// SummaryRecallHits converts active and historical summary search output.
func SummaryRecallHits(hits []core.SummaryHit) []RecallHit {
	out := make([]RecallHit, 0, len(hits))
	for rank, hit := range hits {
		out = append(out, RecallHit{
			Kind: RecallSummary, ID: hit.Summary.ID, ConversationID: hit.Summary.ConversationID,
			Content: hit.Summary.Content, Snippet: hit.Snippet, TokenCount: hit.Summary.TokenCount,
			Depth: hit.Summary.Depth, EarliestSeq: hit.Summary.EarliestSeq,
			LatestSeq: hit.Summary.LatestSeq, CreatedAt: hit.Summary.CreatedAt,
			Active: hit.Active, SourceRank: rank,
		})
	}
	return out
}

// RankRecall selects deterministic combined hits under total and per-kind quotas.
func RankRecall(hits []RecallHit, terms []string, conversationID string, now time.Time, limit int) []RecallHit {
	if limit <= 0 || len(hits) == 0 {
		return nil
	}
	scores := make([]int, len(hits))
	for i, hit := range hits {
		scores[i] = recallScore(hit, terms, conversationID, now)
	}

	selected := make([]bool, len(hits))
	out := make([]RecallHit, 0, min(limit, len(hits)))
	messageCount, summaryCount := 0, 0
	for range min(limit, len(hits), MaxRecallItems) {
		best := bestRecallIndex(hits, scores, selected, messageCount, summaryCount, limit)
		if best < 0 {
			break
		}
		selected[best] = true
		out = append(out, hits[best])
		if hits[best].Kind == RecallSummary {
			summaryCount++
		} else {
			messageCount++
		}
	}
	return out
}

func recallScore(hit RecallHit, terms []string, conversationID string, now time.Time) int {
	content := strings.ToLower(hit.Content)
	matched := 0
	for _, term := range terms {
		if strings.Contains(content, term) {
			matched++
		}
	}
	score := matched*100 + max(25-hit.SourceRank, 0)
	if hit.ConversationID == conversationID {
		score += 75
	}
	score += recencyRecallScore(max(now.Sub(hit.CreatedAt), 0))
	if hit.Kind == RecallSummary {
		score += 60
		if hit.Active {
			score += 40
		}
		return score
	}
	score += roleRecallScore(hit.Role)
	if hit.Role == core.RoleTool {
		score -= min(hit.TokenCount/200, 50)
	}
	return score
}

func bestRecallIndex(hits []RecallHit, scores []int, selected []bool, messages, summaries, limit int) int {
	best := -1
	for i, score := range scores {
		if selected[i] || !withinRecallQuota(hits[i].Kind, messages, summaries, limit) || (best >= 0 && score <= scores[best]) {
			continue
		}
		best = i
	}
	return best
}

func withinRecallQuota(kind RecallKind, messages, summaries, limit int) bool {
	if kind == RecallSummary {
		return summaries < min(MaxSummaryResults, limit)
	}
	return messages < min(limit, MaxRecallItems)
}

func roleRecallScore(role core.Role) int {
	switch role {
	case core.RoleAssistant:
		return 100
	case core.RoleUser:
		return 80
	case core.RoleSystem:
		return 40
	case core.RoleTool:
		return 0
	default:
		return 0
	}
}

func recencyRecallScore(age time.Duration) int {
	switch {
	case age < 24*time.Hour:
		return 30
	case age < 7*24*time.Hour:
		return 20
	case age < 30*24*time.Hour:
		return 10
	default:
		return 0
	}
}

func containsTerm(terms []string, candidate string) bool {
	return slices.Contains(terms, candidate)
}

func useRecallTerm(term string) bool {
	if len([]rune(term)) < 3 && term != "go" && term != "db" && term != "ci" && term != "ui" && term != "pr" {
		return false
	}
	switch term {
	case "about", "after", "again", "also", "and", "back", "been", "before", "being", "can", "could", "did", "does",
		"explain", "fix", "for", "from", "have", "here", "how", "into", "just", "let", "like", "more", "now", "okay", "pick", "please",
		"should", "some", "than", "that", "the", "their", "them", "then", "there", "these", "they", "this", "those",
		"through", "want", "was", "were", "what", "when", "where", "which", "who", "why", "will", "with", "would",
		"yes", "you", "your":
		return false
	default:
		return true
	}
}

// hookOutput mirrors the JSON shape Claude Code and Codex accept for injecting
// supplemental context from a hook (rendered as a system reminder / developer
// message respectively).
type hookOutput struct {
	HookSpecificOutput hookSpecificOutput `json:"hookSpecificOutput"`
}

type hookSpecificOutput struct {
	HookEventName     string `json:"hookEventName"`
	AdditionalContext string `json:"additionalContext"`
}

// AdditionalContextJSON renders the hook output that injects context for the
// given event. It is valid for both Claude Code and Codex.
func AdditionalContextJSON(event, context string) ([]byte, error) {
	out, err := json.Marshal(hookOutput{
		HookSpecificOutput: hookSpecificOutput{HookEventName: event, AdditionalContext: context},
	})
	if err != nil {
		return nil, fmt.Errorf("agents: marshal hook output: %w", err)
	}
	return out, nil
}

func oneLine(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

func truncateRunes(value string, limit int) string {
	runes := []rune(value)
	if len(runes) <= limit {
		return value
	}
	return string(runes[:limit]) + "..."
}
