package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/bonztm/agent-context-manager/internal/core"
	"github.com/bonztm/agent-context-manager/internal/engine"
)

type sequenceRange struct {
	Start int64 `json:"start"`
	End   int64 `json:"end"`
}

type windowBreakdown struct {
	Items               []engine.RenderedItem `json:"items"`
	Estimator           string                `json:"estimator"`
	StoredTokens        int                   `json:"stored_tokens"`
	RenderedTokens      int                   `json:"rendered_tokens"`
	RawTokens           int                   `json:"raw_tokens"`
	SummaryTokens       int                   `json:"summary_tokens"`
	RoleTokens          map[core.Role]int     `json:"role_tokens"`
	SummaryDepthTokens  map[int]int           `json:"summary_depth_tokens"`
	RepresentedMessages int                   `json:"represented_messages"`
	CoverageComplete    bool                  `json:"coverage_complete"`
	CoverageGaps        []sequenceRange       `json:"coverage_gaps"`
	CoverageOverlaps    []sequenceRange       `json:"coverage_overlaps"`
	OffloadReferences   int                   `json:"offload_references"`
}

func newWindowCmd(a *app) *cobra.Command {
	var (
		asJSON    bool
		breakdown bool
	)
	cmd := &cobra.Command{
		Use:     "window <conversation-id>",
		GroupID: groupRetrieval,
		Short:   "Show ACM's assembled context view for a conversation",
		Long: "Renders ACM's persisted, synthetic window for a conversation: a\n" +
			"mix of raw recent messages and summary pointers standing in for compacted\n" +
			"spans, followed by a total item and token count. Before any compaction it is\n" +
			"simply the raw messages in order. Claude Code and Codex do not expose active-\n" +
			"window replacement, so this is diagnostic rather than their live prompt.\n" +
			"Get a conversation id from 'acm grep --json'\n" +
			"or 'acm describe'.",
		Example: `  acm window conv_1a2b3c
  acm window conv_1a2b3c --breakdown
  acm window conv_1a2b3c --breakdown --json`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			comp, _, db, err := a.newEngine(ctx, engine.DefaultConfig(), nil)
			if err != nil {
				return err
			}
			defer func() { _ = db.Close() }()

			items, aErr := comp.Assemble(ctx, args[0])
			if aErr != nil {
				return aErr
			}

			out := cmd.OutOrStdout()
			report := buildWindowBreakdown(items)
			if asJSON && breakdown {
				return json.NewEncoder(out).Encode(report)
			}
			if asJSON {
				return json.NewEncoder(out).Encode(items)
			}
			if err := printWindowItems(out, items); err != nil {
				return err
			}
			if breakdown {
				return printWindowBreakdown(out, report)
			}
			return printWindowTotal(out, report)
		},
	}
	cmd.Flags().BoolVar(&asJSON, "json", false, "emit the window as JSON")
	cmd.Flags().BoolVar(&breakdown, "breakdown", false, "report rendered/stored tokens, roles, depths, coverage, and offloads")
	return cmd
}

func buildWindowBreakdown(items []engine.RenderedItem) windowBreakdown {
	report := windowBreakdown{
		Items: items, Estimator: "heuristic", RoleTokens: make(map[core.Role]int),
		SummaryDepthTokens: make(map[int]int),
	}
	for _, item := range items {
		report.StoredTokens += item.Tokens
		report.RenderedTokens += item.RenderedTokens
		report.RepresentedMessages += item.RepresentedMessages
		if item.HasOffloadReference {
			report.OffloadReferences++
		}
		if item.IsSummary {
			report.SummaryTokens += item.RenderedTokens
			report.SummaryDepthTokens[item.Depth] += item.RenderedTokens
		} else {
			report.RawTokens += item.RenderedTokens
			report.RoleTokens[item.Role] += item.RenderedTokens
		}
	}
	report.CoverageComplete, report.CoverageGaps, report.CoverageOverlaps = sequenceCoverage(items)
	return report
}

func sequenceCoverage(items []engine.RenderedItem) (bool, []sequenceRange, []sequenceRange) {
	var gaps, overlaps []sequenceRange
	for i := 1; i < len(items); i++ {
		previous, current := items[i-1], items[i]
		if current.EarliestSeq > previous.LatestSeq+1 {
			gaps = append(gaps, sequenceRange{Start: previous.LatestSeq + 1, End: current.EarliestSeq - 1})
		}
		if current.EarliestSeq <= previous.LatestSeq {
			overlaps = append(overlaps, sequenceRange{Start: current.EarliestSeq, End: min(previous.LatestSeq, current.LatestSeq)})
		}
	}
	return len(gaps) == 0 && len(overlaps) == 0, gaps, overlaps
}

func printWindowItems(out io.Writer, items []engine.RenderedItem) error {
	for _, item := range items {
		kind := string(item.Role)
		if item.IsSummary {
			kind = fmt.Sprintf("summary d%d", item.Depth)
		}
		if _, err := fmt.Fprintf(out, "[%-12s %s] %s\n", kind, tokenLabel(item), truncateLine(item.Content, 120)); err != nil {
			return err
		}
	}
	return nil
}

func tokenLabel(item engine.RenderedItem) string {
	if item.Tokens == item.RenderedTokens {
		return fmt.Sprintf("tokens=%5d", item.RenderedTokens)
	}
	return fmt.Sprintf("tokens=%5d stored=%5d", item.RenderedTokens, item.Tokens)
}

func printWindowTotal(out io.Writer, report windowBreakdown) error {
	if report.StoredTokens == report.RenderedTokens {
		_, err := fmt.Fprintf(out, "--- %d items, %d rendered tokens ---\n", len(report.Items), report.RenderedTokens)
		return err
	}
	_, err := fmt.Fprintf(out, "--- %d items, %d rendered tokens (%d stored estimate) ---\n",
		len(report.Items), report.RenderedTokens, report.StoredTokens)
	return err
}

func printWindowBreakdown(out io.Writer, report windowBreakdown) error {
	lines := []string{
		"estimator: " + report.Estimator,
		fmt.Sprintf("tokens: rendered=%d stored=%d raw=%d summary=%d", report.RenderedTokens, report.StoredTokens, report.RawTokens, report.SummaryTokens),
		"roles: " + roleTokenText(report.RoleTokens),
		"summary depths: " + depthTokenText(report.SummaryDepthTokens),
		fmt.Sprintf("represented messages: %d", report.RepresentedMessages),
		fmt.Sprintf("sequence coverage: complete=%t gaps=%s overlaps=%s", report.CoverageComplete, rangeText(report.CoverageGaps), rangeText(report.CoverageOverlaps)),
		fmt.Sprintf("offload references: %d", report.OffloadReferences),
	}
	_, err := fmt.Fprintln(out, "---\n"+strings.Join(lines, "\n"))
	return err
}

func roleTokenText(tokens map[core.Role]int) string {
	roles := []core.Role{core.RoleSystem, core.RoleUser, core.RoleAssistant, core.RoleTool}
	parts := make([]string, 0, len(roles))
	for _, role := range roles {
		parts = append(parts, fmt.Sprintf("%s=%d", role, tokens[role]))
	}
	return strings.Join(parts, " ")
}

func depthTokenText(tokens map[int]int) string {
	depths := make([]int, 0, len(tokens))
	for depth := range tokens {
		depths = append(depths, depth)
	}
	sort.Ints(depths)
	parts := make([]string, 0, len(depths))
	for _, depth := range depths {
		parts = append(parts, fmt.Sprintf("d%d=%d", depth, tokens[depth]))
	}
	return strings.Join(parts, " ")
}

func rangeText(ranges []sequenceRange) string {
	if len(ranges) == 0 {
		return "none"
	}
	parts := make([]string, 0, len(ranges))
	for _, item := range ranges {
		parts = append(parts, fmt.Sprintf("%d-%d", item.Start, item.End))
	}
	return strings.Join(parts, ",")
}
