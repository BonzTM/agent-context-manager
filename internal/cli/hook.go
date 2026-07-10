package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"

	"github.com/bonztm/agent-context-manager/internal/agents"
	"github.com/bonztm/agent-context-manager/internal/core"
	"github.com/bonztm/agent-context-manager/internal/engine"
	"github.com/bonztm/agent-context-manager/internal/privacy"
	"github.com/bonztm/agent-context-manager/internal/store"
)

type hookOptions struct {
	agentStr  string
	event     string
	recall    int
	noCompact bool
}

func newHookCmd(a *app) *cobra.Command {
	options := &hookOptions{}
	cmd := &cobra.Command{
		Use:     "hook [notification-json]",
		GroupID: groupCapture,
		Short:   "Handle an agent hook event: capture messages, inject recalled context",
		Long: "Reads a hook payload (JSON) on stdin, or from the single positional argument\n" +
			"used by Codex notify, for the given --agent and --event. It captures any\n" +
			"messages it carries into the lossless store, and — for prompt\n" +
			"events — prints a hookSpecificOutput JSON object injecting relevant recalled\n" +
			"context. This is the entrypoint the per-agent hook configs (written by\n" +
			"'acm init') invoke; you rarely run it by hand.\n\n" +
			"Recognized events:\n" +
			"  UserPromptSubmit       capture the prompt and inject recalled context\n" +
			"  PostToolUse            capture a tool result\n" +
			"  Stop                   (Claude Code) capture assistant turns from the\n" +
			"                         session transcript, then compact if over budget\n" +
			"  agent-turn-complete    (Codex notify) capture user + final assistant\n" +
			"                         message, then compact if over budget\n" +
			"  SessionStart           no-op for capture\n\n" +
			"Recall searches messages and summaries (OR over prompt terms) before the prompt\n" +
			"is stored, excluding the current raw tail. Recall is best-effort: a search\n" +
			"failure is logged and never blocks capture.\n\n" +
			"After capturing a turn-ending event, the conversation is opportunistically\n" +
			"compacted (deterministic summarizer, default budget) when it exceeds the soft\n" +
			"token threshold, so the summary DAG builds as you work with no manual step.\n" +
			"Disable with --no-compact; run 'acm compact' for tuned or LLM summarization.",
		Example: `  echo '{"session_id":"s","prompt":"hi"}' | acm hook --agent claude-code --event UserPromptSubmit
  echo '{"session_id":"s","tool_name":"Bash","tool_response":{}}' | acm hook --agent codex --event PostToolUse`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error { return options.run(a, cmd, args) },
	}
	options.bindFlags(cmd)
	return cmd
}

func (options *hookOptions) bindFlags(cmd *cobra.Command) {
	flags := cmd.Flags()
	flags.StringVar(&options.agentStr, "agent", "", "host agent: claude-code|codex|opencode")
	flags.StringVar(&options.event, "event", "", "hook event name (e.g. UserPromptSubmit, PostToolUse, agent-turn-complete)")
	flags.IntVar(&options.recall, "recall", 5, "max recalled context items to inject on prompt events (0..10)")
	flags.BoolVar(&options.noCompact, "no-compact", false, "skip opportunistic compaction after turn-ending events")
}

func (options *hookOptions) run(a *app, cmd *cobra.Command, args []string) error {
	agent, request, err := options.captureRequest(cmd.InOrStdin(), args)
	if err != nil {
		return err
	}
	ctx := cmd.Context()
	sq, db, err := a.newStore(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = db.Close() }()
	if err := options.handleRecall(ctx, a, cmd, sq, agent, request); err != nil {
		return err
	}
	return options.persistCapture(ctx, a, sq, a.coreService(sq), request)
}

func (options *hookOptions) captureRequest(stdin io.Reader, args []string) (core.Agent, core.IngestRequest, error) {
	agent := core.Agent(options.agentStr)
	if !agent.Valid() {
		return agent, core.IngestRequest{}, fmt.Errorf("hook: invalid --agent %q", options.agentStr)
	}
	if options.event == "" {
		return agent, core.IngestRequest{}, errors.New("hook: --event is required")
	}
	if options.recall < 0 || options.recall > agents.MaxRecallItems {
		return agent, core.IngestRequest{}, fmt.Errorf("hook: --recall must be between 0 and %d", agents.MaxRecallItems)
	}
	payload, err := hookPayload(stdin, args)
	if err != nil {
		return agent, core.IngestRequest{}, fmt.Errorf("hook: read payload: %w", err)
	}
	request, err := agents.Capture(agent, options.event, payload)
	return agent, request, err
}

func (options *hookOptions) handleRecall(ctx context.Context, a *app, cmd *cobra.Command, sq *store.SQLite, agent core.Agent, request core.IngestRequest) error {
	if options.event != agents.EventUserPromptSubmit || len(request.Messages) == 0 || options.recall == 0 || a.policy.Mode(request.SessionID) == privacy.SessionIgnore {
		return nil
	}
	hits, err := automaticRecall(ctx, sq, agent, request.SessionID, request.Messages[0].Content, options.recall)
	if err != nil {
		a.logger.Warn("recall search failed; continuing with capture", "error", err)
		return nil
	}
	block := agents.RecallBlock(hits)
	if block == "" {
		return nil
	}
	return emitRecall(cmd, options.event, block)
}

func (options *hookOptions) persistCapture(ctx context.Context, a *app, sq *store.SQLite, service *core.Service, request core.IngestRequest) error {
	if request.SessionID == "" || len(request.Messages) == 0 {
		return nil
	}
	result, err := service.Ingest(ctx, request)
	if err != nil {
		return err
	}
	if options.noCompact || (options.event != agents.EventStop && options.event != agents.EventTurnComplete) {
		return nil
	}
	compactor := a.newCompactor(sq, engine.DefaultConfig(), nil)
	if _, err := compactor.Compact(ctx, result.ConversationID); err != nil {
		a.logger.Warn("opportunistic compaction failed", "conversation", result.ConversationID, "error", err)
	}
	return nil
}

func automaticRecall(ctx context.Context, sq *store.SQLite, agent core.Agent, sessionID, prompt string, limit int) ([]agents.RecallHit, error) {
	terms := agents.RecallTerms(prompt)
	if len(terms) == 0 {
		return nil, nil
	}
	conversationID := core.DeriveConversationID(agent, sessionID)
	excluded, err := freshTailIDs(ctx, sq, conversationID)
	if err != nil {
		return nil, err
	}
	messageLimit, summaryLimit := recallCandidateLimits(limit)
	query := core.SearchQuery{Text: strings.Join(terms, " "), Limit: messageLimit, Any: true}
	messages, err := sq.SearchMessages(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("search messages: %w", err)
	}
	query.Limit = summaryLimit
	summaries, err := sq.SearchSummaries(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("search summaries: %w", err)
	}
	candidates := agents.MessageRecallHits(messages, excluded)
	candidates = append(candidates, agents.SummaryRecallHits(summaries)...)
	return agents.RankRecall(candidates, terms, conversationID, clock.Now(), limit), nil
}

func freshTailIDs(ctx context.Context, sq *store.SQLite, conversationID string) (map[string]struct{}, error) {
	cfg := engine.DefaultConfig()
	ids, err := sq.RecentConversationalMessageIDs(ctx, conversationID, cfg.FreshTailMessages, cfg.FreshTailTokens, agents.MaxFreshTailRows)
	if err != nil {
		return nil, fmt.Errorf("load fresh tail: %w", err)
	}
	excluded := make(map[string]struct{}, len(ids))
	for _, id := range ids {
		excluded[id] = struct{}{}
	}
	return excluded, nil
}

func recallCandidateLimits(limit int) (messages, summaries int) {
	total := min(agents.MaxRecallCandidates, max(limit, 1)*10)
	summaries = min(agents.MaxSummaryCandidates, max(total/5, 1))
	return total - summaries, summaries
}

func emitRecall(cmd *cobra.Command, event, block string) error {
	out, err := agents.AdditionalContextJSON(event, block)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintln(cmd.OutOrStdout(), string(out))
	return err
}

func hookPayload(stdin io.Reader, args []string) ([]byte, error) {
	if len(args) == 1 {
		return []byte(args[0]), nil
	}
	payload, err := io.ReadAll(stdin)
	if err != nil {
		return nil, fmt.Errorf("hook: read payload: %w", err)
	}
	return payload, nil
}
