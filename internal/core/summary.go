package core

import (
	"context"
	"time"
)

// SummaryKind distinguishes a leaf summary (directly over raw messages) from a
// condensed summary (over other summaries), forming the levels of the DAG.
type SummaryKind string

// Summary kinds.
const (
	SummaryLeaf      SummaryKind = "leaf"
	SummaryCondensed SummaryKind = "condensed"
)

// Summary is a node in the summary DAG. It compresses a span of the
// conversation while lossless pointers (summary_messages / summary_parents)
// preserve the path back to the verbatim originals.
type Summary struct {
	ID                     string
	ConversationID         string
	Kind                   SummaryKind
	Depth                  int
	Content                string
	TokenCount             int
	SourceCount            int // direct children (messages for leaf, summaries for condensed)
	DescendantMessageCount int // raw messages ultimately covered
	EarliestSeq            int64
	LatestSeq              int64
	CreatedAt              time.Time
}

// ContextItemType tags an entry in the assembled active window.
type ContextItemType string

// Context item types.
const (
	ContextMessage ContextItemType = "message"
	ContextSummary ContextItemType = "summary"
)

// ContextItem is one ordered entry in a conversation's active window: either a
// raw message or a summary pointer that stands in for a compacted span.
type ContextItem struct {
	Ordinal int
	Type    ContextItemType
	RefID   string
}

// LargeFile is an offloaded oversized payload: the verbatim content lives on
// disk at StorageURI, with only an exploration summary kept inline. Extractor
// records how that summary was produced (a type-aware deterministic extractor,
// the configured summarizer, or the truncation fallback).
type LargeFile struct {
	ID                 string
	ConversationID     string
	MessageID          string
	StorageURI         string
	ByteSize           int64
	TokenCount         int
	ExplorationSummary string
	Extractor          string
	CreatedAt          time.Time
}

// LeafSummaryInput is the payload for creating a leaf summary over messages.
type LeafSummaryInput struct {
	ConversationID         string
	Content                string
	TokenCount             int
	SourceMessageIDs       []string
	EarliestSeq            int64
	LatestSeq              int64
	DescendantMessageCount int
}

// CondensedSummaryInput is the payload for creating a condensed summary over
// child summaries at Depth-1.
type CondensedSummaryInput struct {
	ConversationID         string
	Depth                  int
	Content                string
	TokenCount             int
	ChildSummaryIDs        []string
	EarliestSeq            int64
	LatestSeq              int64
	DescendantMessageCount int
}

// SummarizeInput asks a Summarizer to compress Sources to roughly TargetTokens.
// The engine owns the escalation/size-guard policy around this call; the
// Summarizer only needs to produce a best-effort summary at the target.
type SummarizeInput struct {
	Kind         SummaryKind
	Depth        int
	Sources      []string
	TargetTokens int
}

// Summarizer produces a summary of the source texts. Implementations may be
// deterministic (no LLM) or LLM-backed; the engine treats them uniformly.
type Summarizer interface {
	Summarize(ctx context.Context, in SummarizeInput) (string, error)
}
