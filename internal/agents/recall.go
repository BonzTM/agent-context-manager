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

// RecallBlock formats search hits into an injectable context block. It returns
// "" when there is nothing relevant to inject (so the caller can skip output).
func RecallBlock(hits []core.SearchHit) string {
	if len(hits) == 0 {
		return ""
	}
	lines := make([]string, 0, len(hits)+3)
	lines = append(lines,
		"<acm-recall>",
		"Relevant earlier context from this project's history. Drill down for the",
		"verbatim message with: acm describe <msg-id>  (or search more: acm grep <pattern>).",
	)
	for _, h := range hits {
		lines = append(lines, fmt.Sprintf("- [%s seq=%d %s] %s",
			h.Message.ID, h.Message.Seq, h.Message.Role, oneLine(h.Snippet)))
	}
	lines = append(lines, "</acm-recall>")
	return strings.Join(lines, "\n")
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

// RankRecall selects the strongest hits using lexical coverage, host context,
// role, recency, BM25 input order, and a penalty for oversized tool payloads.
func RankRecall(hits []core.SearchHit, terms []string, conversationID string, limit int) []core.SearchHit {
	if limit <= 0 || len(hits) == 0 {
		return nil
	}
	newest := newestHitTime(hits)
	scores := make([]int, len(hits))
	for i, hit := range hits {
		scores[i] = recallScore(hit, terms, conversationID, newest, len(hits)-i)
	}

	selected := make([]bool, len(hits))
	out := make([]core.SearchHit, 0, min(limit, len(hits)))
	for range min(limit, len(hits)) {
		best := bestRecallIndex(scores, selected)
		selected[best] = true
		out = append(out, hits[best])
	}
	return out
}

func recallScore(hit core.SearchHit, terms []string, conversationID string, newest time.Time, bm25Rank int) int {
	content := strings.ToLower(hit.Message.Content)
	matched := 0
	for _, term := range terms {
		if strings.Contains(content, term) {
			matched++
		}
	}
	score := matched*100 + min(bm25Rank, 25)
	if hit.Message.ConversationID == conversationID {
		score += 75
	}
	score += roleRecallScore(hit.Message.Role)
	score += recencyRecallScore(newest.Sub(hit.Message.CreatedAt))
	if hit.Message.Role == core.RoleTool {
		score -= min(hit.Message.TokenCount/200, 50)
	}
	return score
}

func newestHitTime(hits []core.SearchHit) time.Time {
	var newest time.Time
	for _, hit := range hits {
		if hit.Message.CreatedAt.After(newest) {
			newest = hit.Message.CreatedAt
		}
	}
	return newest
}

func bestRecallIndex(scores []int, selected []bool) int {
	best := -1
	for i, score := range scores {
		if selected[i] || (best >= 0 && score <= scores[best]) {
			continue
		}
		best = i
	}
	return best
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
