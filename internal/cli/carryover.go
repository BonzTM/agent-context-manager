package cli

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/bonztm/agent-context-manager/internal/core"
	"github.com/bonztm/agent-context-manager/internal/privacy"
	"github.com/bonztm/agent-context-manager/internal/store"
)

const (
	defaultCarryOverSummaries = 16
	maxCarryOverSummaries     = 64
	maxCarryOverChars         = 60_000
)

func newCarryOverCmd(a *app) *cobra.Command {
	var agentName string
	var depth, limit int
	cmd := &cobra.Command{
		Use:     "carry-over <source-conversation> <target-session>",
		GroupID: groupCompaction,
		Short:   "Seed a new session from a prior conversation's deepest summaries",
		Args:    cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			if depth < -1 || limit <= 0 || limit > maxCarryOverSummaries {
				return errors.New("carry-over: --depth must be -1 or greater and --max-summaries must be 1..64")
			}
			return runCarryOver(cmd, a, args[0], args[1], agentName, depth, limit)
		},
	}
	cmd.Flags().StringVar(&agentName, "agent", "", "target agent (default: source conversation agent)")
	cmd.Flags().IntVar(&depth, "depth", -1, "summary depth to retain (-1 selects deepest available)")
	cmd.Flags().IntVar(&limit, "max-summaries", defaultCarryOverSummaries, "maximum summaries copied into the seed")
	return cmd
}

func runCarryOver(cmd *cobra.Command, a *app, sourceID, targetSession, agentName string, depth, limit int) error {
	if a.policy.Mode(targetSession) != privacy.SessionCapture {
		return fmt.Errorf("carry-over: target session %q is not persistent under project policy", targetSession)
	}
	ctx := cmd.Context()
	sq, db, err := a.newStore(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = db.Close() }()
	source, err := sq.ConversationByID(ctx, sourceID)
	if err != nil {
		return err
	}
	targetAgent, err := carryOverAgent(source.Agent, agentName)
	if err != nil {
		return err
	}
	summaries, err := selectCarryOverSummaries(ctx, sq, sourceID, depth, limit)
	if err != nil {
		return err
	}
	content, selectedDepth, err := renderCarryOver(sourceID, summaries)
	if err != nil {
		return err
	}
	result, err := a.coreService(sq).Ingest(ctx, core.IngestRequest{
		Agent: targetAgent, SessionID: targetSession,
		Messages: []core.IngestMessage{{
			Role: core.RoleSystem, Content: content,
			ExternalID: fmt.Sprintf("carry-over:%s:depth:%d", sourceID, selectedDepth),
		}},
	})
	if err != nil {
		return err
	}
	if pErr := sq.SetConversationPin(ctx, sourceID, true); pErr != nil {
		return fmt.Errorf("carry-over: seed stored but source pin failed: %w", pErr)
	}
	_, err = fmt.Fprintf(cmd.OutOrStdout(), "source=%s target=%s depth=%d summaries=%d appended=%d deduped=%d pinned=true\n",
		sourceID, result.ConversationID, selectedDepth, len(summaries), result.Appended, result.Deduped)
	return err
}

func carryOverAgent(source core.Agent, requested string) (core.Agent, error) {
	if requested == "" {
		return source, nil
	}
	agent := core.Agent(requested)
	if !agent.Valid() {
		return "", fmt.Errorf("carry-over: invalid target agent %q", requested)
	}
	return agent, nil
}

func selectCarryOverSummaries(ctx context.Context, sq *store.SQLite, conversationID string, depth, limit int) ([]core.Summary, error) {
	summaries, err := sq.ListConversationSummaries(ctx, conversationID)
	if err != nil {
		return nil, err
	}
	if len(summaries) == 0 {
		return nil, fmt.Errorf("carry-over: source conversation %s has no summaries", conversationID)
	}
	selectedDepth := depth
	if selectedDepth < 0 {
		selectedDepth = summaries[0].Depth
	}
	var selected []core.Summary
	for _, summary := range summaries {
		if summary.Depth == selectedDepth {
			selected = append(selected, summary)
		}
	}
	if len(selected) == 0 {
		return nil, fmt.Errorf("carry-over: source has no summaries at depth %d", selectedDepth)
	}
	if len(selected) > limit {
		return nil, fmt.Errorf("carry-over: %d summaries at depth %d exceed --max-summaries %d", len(selected), selectedDepth, limit)
	}
	return selected, nil
}

func renderCarryOver(sourceID string, summaries []core.Summary) (string, int, error) {
	depth := summaries[0].Depth
	var output strings.Builder
	fmt.Fprintf(&output, "<acm-carry-over source=%q depth=%d>\n", sourceID, depth)
	for _, summary := range summaries {
		fmt.Fprintf(&output, "<summary id=%q depth=%d seq=%d-%d>\n%s\n</summary>\n",
			summary.ID, summary.Depth, summary.EarliestSeq, summary.LatestSeq, summary.Content)
		if output.Len() > maxCarryOverChars {
			return "", 0, fmt.Errorf("carry-over: rendered seed exceeds %d characters", maxCarryOverChars)
		}
	}
	output.WriteString("</acm-carry-over>")
	return output.String(), depth, nil
}
