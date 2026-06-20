package engine

import (
	"cmp"
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/bonztm/agent-context-manager/internal/core"
)

// RenderedItem is one entry of an assembled active window, ready to send to a
// model: a raw message or a summary rendered as a self-describing pseudo-message.
type RenderedItem struct {
	Type      core.ContextItemType
	RefID     string
	Role      core.Role
	Content   string
	Tokens    int
	Depth     int
	IsSummary bool
}

// Assemble renders the conversation's active window. If no window has been
// persisted yet (never compacted) it renders the raw messages in order, so the
// assembled view always reflects what the model would currently see.
func (c *Compactor) Assemble(ctx context.Context, conversationID string) ([]RenderedItem, error) {
	items, err := c.store.ListContextItems(ctx, conversationID)
	if err != nil {
		return nil, err
	}
	if len(items) == 0 {
		msgs, mErr := c.store.ListMessages(ctx, conversationID, 0, 0)
		if mErr != nil {
			return nil, mErr
		}
		out := make([]RenderedItem, 0, len(msgs))
		for _, m := range msgs {
			out = append(out, messageItem(m))
		}
		return out, nil
	}

	out := make([]RenderedItem, 0, len(items))
	for _, it := range items {
		switch it.Type {
		case core.ContextMessage:
			m, gErr := c.store.GetMessage(ctx, it.RefID)
			if gErr != nil {
				return nil, gErr
			}
			out = append(out, messageItem(m))
		case core.ContextSummary:
			s, gErr := c.store.GetSummary(ctx, it.RefID)
			if gErr != nil {
				return nil, gErr
			}
			out = append(out, RenderedItem{
				Type:      core.ContextSummary,
				RefID:     s.ID,
				Role:      core.RoleUser,
				Content:   renderSummary(s),
				Tokens:    s.TokenCount,
				Depth:     s.Depth,
				IsSummary: true,
			})
		default:
			return nil, fmt.Errorf("engine: unknown context item type %q", it.Type)
		}
	}
	return out, nil
}

func messageItem(m core.Message) RenderedItem {
	return RenderedItem{Type: core.ContextMessage, RefID: m.ID, Role: m.Role, Content: m.Content, Tokens: m.TokenCount}
}

// renderSummary wraps a summary as a self-describing pseudo-message so the model
// can reason about its age, scope, and how to drill down (acm expand <id>).
func renderSummary(s core.Summary) string {
	return fmt.Sprintf("<summary id=%q depth=%d messages=%d seq=%d-%d>\n%s\n</summary>",
		s.ID, s.Depth, s.DescendantMessageCount, s.EarliestSeq, s.LatestSeq, s.Content)
}

// Expansion is the direct, one-level expansion of a summary: its source
// messages (leaf) or child summaries (condensed).
type Expansion struct {
	Summary  core.Summary
	Messages []core.Message
	Children []core.Summary
}

// Expand returns the direct sources of a summary, reversing one level of
// compaction.
func (c *Compactor) Expand(ctx context.Context, summaryID string) (Expansion, error) {
	s, err := c.store.GetSummary(ctx, summaryID)
	if err != nil {
		return Expansion{}, err
	}
	if s.Kind == core.SummaryLeaf {
		msgs, mErr := c.store.SummaryMessages(ctx, summaryID)
		if mErr != nil {
			return Expansion{}, mErr
		}
		return Expansion{Summary: s, Messages: msgs}, nil
	}
	children, cErr := c.store.SummaryChildren(ctx, summaryID)
	if cErr != nil {
		return Expansion{}, cErr
	}
	return Expansion{Summary: s, Children: children}, nil
}

// ExpandToMessages walks the DAG beneath a summary down to every verbatim source
// message, deduplicated and ordered by sequence — full lossless recovery.
func (c *Compactor) ExpandToMessages(ctx context.Context, summaryID string) ([]core.Message, error) {
	seen := make(map[string]bool)
	var out []core.Message

	var walk func(id string) error
	walk = func(id string) error {
		s, err := c.store.GetSummary(ctx, id)
		if err != nil {
			return err
		}
		if s.Kind == core.SummaryLeaf {
			msgs, mErr := c.store.SummaryMessages(ctx, id)
			if mErr != nil {
				return mErr
			}
			for _, m := range msgs {
				if !seen[m.ID] {
					seen[m.ID] = true
					out = append(out, m)
				}
			}
			return nil
		}
		children, cErr := c.store.SummaryChildren(ctx, id)
		if cErr != nil {
			return cErr
		}
		for _, ch := range children {
			if wErr := walk(ch.ID); wErr != nil {
				return wErr
			}
		}
		return nil
	}

	if err := walk(summaryID); err != nil {
		return nil, err
	}
	slices.SortFunc(out, func(a, b core.Message) int { return cmp.Compare(a.Seq, b.Seq) })
	return out, nil
}

// ExpandQuery expands a summary to its source messages and returns those whose
// content contains query (case-insensitive) — focused lossless recall.
func (c *Compactor) ExpandQuery(ctx context.Context, summaryID, query string) ([]core.Message, error) {
	msgs, err := c.ExpandToMessages(ctx, summaryID)
	if err != nil {
		return nil, err
	}
	if query == "" {
		return msgs, nil
	}
	needle := strings.ToLower(query)
	var out []core.Message
	for _, m := range msgs {
		if strings.Contains(strings.ToLower(m.Content), needle) {
			out = append(out, m)
		}
	}
	return out, nil
}
