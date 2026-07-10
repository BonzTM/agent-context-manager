// Package engine implements the LCM compaction loop and active-window assembly
// over the store. It realizes the Voltropy paper's mechanics: a two-threshold
// budget, leaf+condensed summarization into a DAG, an escalating size-guarded
// summarizer, a protected fresh tail, and large-file offload — all deterministic
// and engine-owned, with lossless pointers preserved so any summary can be
// expanded back to its source.
package engine

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/bonztm/agent-context-manager/internal/core"
	"github.com/bonztm/agent-context-manager/internal/explore"
)

// Store is the persistence surface the engine needs. It is a consumer-defined
// interface; *store.SQLite satisfies it.
type Store interface {
	ListMessages(ctx context.Context, conversationID string, afterSeq int64, limit int) ([]core.Message, error)
	GetMessage(ctx context.Context, id string) (core.Message, error)
	GetSummary(ctx context.Context, id string) (core.Summary, error)
	CreateLeafSummary(ctx context.Context, in core.LeafSummaryInput) (core.Summary, error)
	CreateCondensedSummary(ctx context.Context, in core.CondensedSummaryInput) (core.Summary, error)
	SummaryMessages(ctx context.Context, summaryID string) ([]core.Message, error)
	SummaryChildren(ctx context.Context, summaryID string) ([]core.Summary, error)
	MarkSummaryExpanded(ctx context.Context, summaryID string) error
	ListContextItems(ctx context.Context, conversationID string) ([]core.ContextItem, error)
	ReplaceContextItems(ctx context.Context, conversationID string, items []core.ContextItem) error
	CreateLargeFile(ctx context.Context, lf core.LargeFile) (core.LargeFile, error)
}

// Config tunes the compaction loop. Defaults come from DefaultConfig; fractions
// are of ModelContextTokens.
type Config struct {
	ModelContextTokens    int
	SoftFraction          float64 // begin compacting above this
	HardFraction          float64 // warn when a finished pass is still above this
	FreshTailMessages     int     // most recent conversational messages kept raw
	FreshTailTokens       int     // minimum recent conversational tokens kept raw
	LeafChunkTokens       int     // max source tokens folded into one leaf
	LeafTargetTokens      int     // target size of a leaf summary
	CondensedTargetTokens int     // target size of a condensed summary
	CondenseFanout        int     // condense once this many same-depth summaries stack up
	CondenseChunkTokens   int     // max source tokens folded into one condensed summary
	MaxDepth              int     // deepest condensed level
	TruncateTokens        int     // deterministic fallback truncation (Algorithm 3, level 3)
	LargeFileThreshold    int     // message token count above which content is offloaded
	MaxIterations         int     // safety bound on the loop
}

// DefaultConfig returns sensible defaults tuned for a ~200K-token model window.
func DefaultConfig() Config {
	return Config{
		ModelContextTokens:    200_000,
		SoftFraction:          0.6,
		HardFraction:          0.8,
		FreshTailMessages:     8,
		FreshTailTokens:       4_000,
		LeafChunkTokens:       4_000,
		LeafTargetTokens:      600,
		CondensedTargetTokens: 1_000,
		CondenseFanout:        4,
		CondenseChunkTokens:   8_000,
		MaxDepth:              3,
		TruncateTokens:        512,
		LargeFileThreshold:    25_000,
		MaxIterations:         200,
	}
}

// Compactor runs compaction and assembles windows for one store.
type Compactor struct {
	store      Store
	summarizer core.Summarizer
	counter    core.TokenCounter
	clock      core.Clock
	cfg        Config
	filesDir   string
	logger     *slog.Logger
}

// New builds a Compactor. filesDir is where offloaded large files are written
// (empty disables offload). logger may be nil.
func New(store Store, summarizer core.Summarizer, counter core.TokenCounter, clock core.Clock, cfg Config, filesDir string, logger *slog.Logger) *Compactor {
	if logger == nil {
		logger = slog.New(slog.DiscardHandler)
	}
	return &Compactor{
		store:      store,
		summarizer: summarizer,
		counter:    counter,
		clock:      clock,
		cfg:        cfg,
		filesDir:   filesDir,
		logger:     logger,
	}
}

// Result reports what a Compact pass did.
type Result struct {
	BeforeTokens int
	AfterTokens  int
	Leaves       int
	Condensed    int
	Compacted    bool
}

// windowItem is one entry in the in-memory active window during compaction.
type windowItem struct {
	typ         core.ContextItemType
	refID       string
	role        core.Role
	tokens      int
	earliestSeq int64
	latestSeq   int64
	depth       int // 0 for messages and leaf summaries
}

// Compact brings a conversation's active window under the soft threshold by
// folding the oldest compactible spans into the summary DAG, protecting the
// fresh tail. It is idempotent: a window already under threshold is left as-is
// (beyond persisting the initial seeding).
func (c *Compactor) Compact(ctx context.Context, conversationID string) (Result, error) {
	return c.compactWindow(ctx, conversationID, false)
}

// ArchiveToFreshTail folds every compactible message outside the protected
// fresh tail. Owning hosts use it before replacing archived prompt content.
func (c *Compactor) ArchiveToFreshTail(ctx context.Context, conversationID string) (Result, error) {
	return c.compactWindow(ctx, conversationID, true)
}

func (c *Compactor) compactWindow(ctx context.Context, conversationID string, archiveAll bool) (Result, error) {
	if err := c.cfg.Validate(); err != nil {
		return Result{}, err
	}
	w, err := c.buildWindow(ctx, conversationID)
	if err != nil {
		return Result{}, err
	}

	res := Result{BeforeTokens: windowTokens(w)}
	soft := int(c.cfg.SoftFraction * float64(c.cfg.ModelContextTokens))
	for range c.cfg.MaxIterations {
		if !archiveAll && windowTokens(w) <= soft {
			break
		}
		protected := protectedSet(w, c.cfg.FreshTailMessages, c.cfg.FreshTailTokens)
		if start, end, ok := findMessageBlock(w, protected, c.cfg.LeafChunkTokens); ok {
			leaf, lErr := c.makeLeaf(ctx, conversationID, w[start:end])
			if lErr != nil {
				return Result{}, lErr
			}
			w = replaceRange(w, start, end, leaf)
			res.Leaves++
			continue
		}
		if start, end, ok := condenseRun(w, c.cfg.CondenseFanout, c.cfg.MaxDepth, c.cfg.CondenseChunkTokens); ok {
			cond, cErr := c.makeCondensed(ctx, conversationID, w[start:end])
			if cErr != nil {
				return Result{}, cErr
			}
			w = replaceRange(w, start, end, cond)
			res.Condensed++
			continue
		}
		break // cannot reduce further without touching the fresh tail / max depth
	}

	if pErr := c.persist(ctx, conversationID, w); pErr != nil {
		return Result{}, pErr
	}
	res.AfterTokens = windowTokens(w)
	res.Compacted = res.Leaves > 0 || res.Condensed > 0
	if hard := int(c.cfg.HardFraction * float64(c.cfg.ModelContextTokens)); !archiveAll && hard > 0 && res.AfterTokens > hard {
		c.logger.Warn("window still above hard threshold after compaction",
			"conversation", conversationID, "tokens", res.AfterTokens, "hard", hard,
			"hint", "lower --leaf-target-tokens/--condensed-target-tokens or raise --fresh-tail limits")
	}
	c.logger.Debug("compacted", "conversation", conversationID, "archive_all", archiveAll,
		"before", res.BeforeTokens, "after", res.AfterTokens, "leaves", res.Leaves, "condensed", res.Condensed)
	return res, nil
}

// buildWindow reconstructs the conversation's active window: the persisted
// context items (if any), extended with every message that arrived after the
// last persisted item so the window always tracks the live conversation. With
// no persisted window it is simply all messages in order.
func (c *Compactor) buildWindow(ctx context.Context, conversationID string) ([]windowItem, error) {
	items, err := c.store.ListContextItems(ctx, conversationID)
	if err != nil {
		return nil, err
	}

	var (
		w       []windowItem
		lastSeq int64
	)
	for _, it := range items {
		switch it.Type {
		case core.ContextMessage:
			m, mErr := c.store.GetMessage(ctx, it.RefID)
			if mErr != nil {
				return nil, mErr
			}
			w = append(w, windowItem{typ: core.ContextMessage, refID: m.ID, role: m.Role, tokens: m.TokenCount, earliestSeq: m.Seq, latestSeq: m.Seq})
			lastSeq = max(lastSeq, m.Seq)
		case core.ContextSummary:
			s, sErr := c.store.GetSummary(ctx, it.RefID)
			if sErr != nil {
				return nil, sErr
			}
			w = append(w, windowItem{typ: core.ContextSummary, refID: s.ID, tokens: s.TokenCount, earliestSeq: s.EarliestSeq, latestSeq: s.LatestSeq, depth: s.Depth})
			lastSeq = max(lastSeq, s.LatestSeq)
		default:
			return nil, fmt.Errorf("engine: unknown context item type %q", it.Type)
		}
	}

	// Messages ingested after the window was last persisted are appended so
	// compaction and assembly never operate on a stale, frozen window.
	msgs, err := c.store.ListMessages(ctx, conversationID, lastSeq, 0)
	if err != nil {
		return nil, err
	}
	for _, m := range msgs {
		w = append(w, windowItem{typ: core.ContextMessage, refID: m.ID, role: m.Role, tokens: m.TokenCount, earliestSeq: m.Seq, latestSeq: m.Seq})
	}
	return w, nil
}

func (c *Compactor) persist(ctx context.Context, conversationID string, w []windowItem) error {
	items := make([]core.ContextItem, 0, len(w))
	for i, it := range w {
		items = append(items, core.ContextItem{Ordinal: i, Type: it.typ, RefID: it.refID})
	}
	return c.store.ReplaceContextItems(ctx, conversationID, items)
}

func (c *Compactor) makeLeaf(ctx context.Context, conversationID string, block []windowItem) (windowItem, error) {
	msgIDs := make([]string, 0, len(block))
	sources := make([]string, 0, len(block))
	for _, it := range block {
		m, err := c.store.GetMessage(ctx, it.refID)
		if err != nil {
			return windowItem{}, err
		}
		msgIDs = append(msgIDs, m.ID)
		src, sErr := c.sourceText(ctx, m)
		if sErr != nil {
			return windowItem{}, sErr
		}
		sources = append(sources, src)
	}

	content, tokens, err := c.summarizeWithGuard(ctx, core.SummaryLeaf, 0, sources, c.cfg.LeafTargetTokens)
	if err != nil {
		return windowItem{}, err
	}

	sum, err := c.store.CreateLeafSummary(ctx, core.LeafSummaryInput{
		ConversationID:         conversationID,
		Content:                content,
		TokenCount:             tokens,
		SourceMessageIDs:       msgIDs,
		EarliestSeq:            block[0].earliestSeq,
		LatestSeq:              block[len(block)-1].latestSeq,
		DescendantMessageCount: len(block),
	})
	if err != nil {
		return windowItem{}, err
	}
	return windowItem{typ: core.ContextSummary, refID: sum.ID, tokens: sum.TokenCount, earliestSeq: sum.EarliestSeq, latestSeq: sum.LatestSeq, depth: 0}, nil
}

func (c *Compactor) makeCondensed(ctx context.Context, conversationID string, block []windowItem) (windowItem, error) {
	childIDs := make([]string, 0, len(block))
	sources := make([]string, 0, len(block))
	descendants := 0
	for _, it := range block {
		s, err := c.store.GetSummary(ctx, it.refID)
		if err != nil {
			return windowItem{}, err
		}
		childIDs = append(childIDs, s.ID)
		sources = append(sources, s.Content)
		descendants += s.DescendantMessageCount
	}
	depth := block[0].depth + 1

	content, tokens, err := c.summarizeWithGuard(ctx, core.SummaryCondensed, depth, sources, c.cfg.CondensedTargetTokens)
	if err != nil {
		return windowItem{}, err
	}

	sum, err := c.store.CreateCondensedSummary(ctx, core.CondensedSummaryInput{
		ConversationID:         conversationID,
		Depth:                  depth,
		Content:                content,
		TokenCount:             tokens,
		ChildSummaryIDs:        childIDs,
		EarliestSeq:            block[0].earliestSeq,
		LatestSeq:              block[len(block)-1].latestSeq,
		DescendantMessageCount: descendants,
	})
	if err != nil {
		return windowItem{}, err
	}
	return windowItem{typ: core.ContextSummary, refID: sum.ID, tokens: sum.TokenCount, earliestSeq: sum.EarliestSeq, latestSeq: sum.LatestSeq, depth: depth}, nil
}

// sourceText renders a message for summarization, offloading oversized content
// to disk and substituting an exploration placeholder so huge payloads never
// enter a summary or the active window.
func (c *Compactor) sourceText(ctx context.Context, m core.Message) (string, error) {
	if c.cfg.LargeFileThreshold > 0 && m.TokenCount > c.cfg.LargeFileThreshold {
		lf, err := c.offload(ctx, m)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("%s: [large content offloaded -> %s (~%d tokens) at %s]\nexploration: %s",
			m.Role, lf.ID, lf.TokenCount, lf.StorageURI, lf.ExplorationSummary), nil
	}
	return fmt.Sprintf("%s: %s", m.Role, m.Content), nil
}

// offload writes a message's verbatim content to filesDir and records a
// large_files row with an exploration summary. Structured content (JSON, CSV,
// SQL, code) gets a deterministic type-aware description with no model call;
// everything else falls to the configured summarizer, then to truncation. The
// stored URI is a real path the agent can read directly.
func (c *Compactor) offload(ctx context.Context, m core.Message) (core.LargeFile, error) {
	dir := filepath.Join(c.filesDir, m.ConversationID)
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return core.LargeFile{}, fmt.Errorf("engine: create files dir: %w", err)
	}
	path := filepath.Join(dir, m.ID+".txt")
	if err := os.WriteFile(path, []byte(m.Content), 0o600); err != nil {
		return core.LargeFile{}, fmt.Errorf("engine: write large file: %w", err)
	}

	exploration, extractor, ok := explore.Describe(m.Content)
	if !ok {
		extractor = "summarizer"
		var sErr error
		exploration, sErr = c.summarizer.Summarize(ctx, core.SummarizeInput{
			Kind:         core.SummaryLeaf,
			Sources:      []string{m.Content},
			TargetTokens: 200,
		})
		if sErr != nil {
			// Offload still proceeds; fall back to a truncated head as the summary.
			exploration = truncateToTokens(m.Content, 200)
			extractor = "truncation"
		}
	}
	return c.store.CreateLargeFile(ctx, core.LargeFile{
		ConversationID:     m.ConversationID,
		MessageID:          m.ID,
		StorageURI:         path,
		ByteSize:           int64(len(m.Content)),
		TokenCount:         m.TokenCount,
		ExplorationSummary: exploration,
		Extractor:          extractor,
	})
}

// summarizeWithGuard implements the paper's escalation (Algorithm 3): normal,
// then aggressive (half target), then a deterministic truncation that always
// terminates — accepting the first result that is genuinely smaller than the
// combined input.
func (c *Compactor) summarizeWithGuard(ctx context.Context, kind core.SummaryKind, depth int, sources []string, target int) (string, int, error) {
	inputTokens := 0
	for _, s := range sources {
		inputTokens += c.counter.Count(s)
	}

	normal, err := c.summarizer.Summarize(ctx, core.SummarizeInput{Kind: kind, Depth: depth, Sources: sources, TargetTokens: target})
	if err != nil {
		return "", 0, fmt.Errorf("engine: summarize: %w", err)
	}
	if t := c.counter.Count(normal); t < inputTokens {
		return normal, t, nil
	}

	aggressive, err := c.summarizer.Summarize(ctx, core.SummarizeInput{Kind: kind, Depth: depth, Sources: sources, TargetTokens: max(target/2, c.cfg.TruncateTokens)})
	if err != nil {
		return "", 0, fmt.Errorf("engine: summarize (aggressive): %w", err)
	}
	if t := c.counter.Count(aggressive); t < inputTokens {
		return aggressive, t, nil
	}

	truncated := truncateToTokens(strings.Join(sources, "\n"), c.cfg.TruncateTokens)
	return truncated, c.counter.Count(truncated), nil
}

// --- pure window helpers ---

func windowTokens(w []windowItem) int {
	total := 0
	for _, it := range w {
		total += it.tokens
	}
	return total
}

// protectedSet preserves recent conversational turns by both message count and
// token floor. Tool results stay eligible for early compaction and offload.
func protectedSet(w []windowItem, freshTail, freshTokens int) map[int]bool {
	protected := make(map[int]bool)
	if freshTail <= 0 && freshTokens <= 0 {
		return protected
	}
	seen, tokenTotal := 0, 0
	for i := len(w) - 1; i >= 0 && (seen < freshTail || tokenTotal < freshTokens); i-- {
		if isConversationalMessage(w[i]) {
			protected[i] = true
			seen++
			tokenTotal += w[i].tokens
		}
	}
	return protected
}

func isConversationalMessage(item windowItem) bool {
	return item.typ == core.ContextMessage && item.role != core.RoleTool
}

// findMessageBlock returns the first maximal run of consecutive, unprotected
// message items, capped at leafChunkTokens.
func findMessageBlock(w []windowItem, protected map[int]bool, leafChunkTokens int) (start, end int, ok bool) {
	i := 0
	for i < len(w) && (w[i].typ != core.ContextMessage || protected[i]) {
		i++
	}
	if i >= len(w) {
		return 0, 0, false
	}
	end = i
	tok := 0
	for end < len(w) && w[end].typ == core.ContextMessage && !protected[end] {
		tok += w[end].tokens
		end++
		if tok >= leafChunkTokens {
			break
		}
	}
	return i, end, true
}

// condenseRun returns the first exactly fanout-sized run below maxDepth whose
// total source size stays within chunkTokens.
func condenseRun(w []windowItem, fanout, maxDepth, chunkTokens int) (start, end int, ok bool) {
	i := 0
	for i < len(w) {
		if w[i].typ != core.ContextSummary {
			i++
			continue
		}
		d := w[i].depth
		j, tokens := i, 0
		for j < len(w) && j-i < fanout && w[j].typ == core.ContextSummary && w[j].depth == d {
			if tokens+w[j].tokens > chunkTokens {
				break
			}
			tokens += w[j].tokens
			j++
		}
		if d < maxDepth && j-i == fanout {
			return i, j, true
		}
		i++
	}
	return 0, 0, false
}

// replaceRange returns a new window with w[start:end] replaced by item.
func replaceRange(w []windowItem, start, end int, item windowItem) []windowItem {
	out := make([]windowItem, 0, len(w)-(end-start)+1)
	out = append(out, w[:start]...)
	out = append(out, item)
	out = append(out, w[end:]...)
	return out
}

func truncateToTokens(s string, targetTokens int) string {
	maxChars := targetTokens * 4
	r := []rune(s)
	if len(r) <= maxChars {
		return s
	}
	return string(r[:maxChars]) + "…"
}
