package cli

import (
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"

	"github.com/bonztm/agent-context-manager/internal/agents"
	"github.com/bonztm/agent-context-manager/internal/core"
	"github.com/bonztm/agent-context-manager/internal/engine"
	"github.com/bonztm/agent-context-manager/internal/tokens"
)

func newHookCmd(a *app) *cobra.Command {
	const maxRecallCandidates = 50
	var (
		agentStr  string
		event     string
		recall    int
		noCompact bool
	)
	cmd := &cobra.Command{
		Use:     "hook",
		GroupID: groupCapture,
		Short:   "Handle an agent hook event: capture messages, inject recalled context",
		Long: "Reads a hook payload (JSON) on stdin for the given --agent and --event,\n" +
			"captures any messages it carries into the lossless store, and — for prompt\n" +
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
			"Recall searches the project history (OR over prompt terms) before the prompt\n" +
			"is stored, so the prompt cannot match itself. Recall is best-effort: a search\n" +
			"failure is logged and never blocks capture.\n\n" +
			"After capturing a turn-ending event, the conversation is opportunistically\n" +
			"compacted (deterministic summarizer, default budget) when it exceeds the soft\n" +
			"token threshold, so the summary DAG builds as you work with no manual step.\n" +
			"Disable with --no-compact; run 'acm compact' for tuned or LLM summarization.",
		Example: `  echo '{"session_id":"s","prompt":"hi"}' | acm hook --agent claude-code --event UserPromptSubmit
  echo '{"session_id":"s","tool_name":"Bash","tool_response":{}}' | acm hook --agent codex --event PostToolUse`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			agent := core.Agent(agentStr)
			if !agent.Valid() {
				return fmt.Errorf("hook: invalid --agent %q", agentStr)
			}
			if event == "" {
				return errors.New("hook: --event is required")
			}
			if recall < 0 {
				return errors.New("hook: --recall must be non-negative")
			}

			payload, err := io.ReadAll(cmd.InOrStdin())
			if err != nil {
				return fmt.Errorf("hook: read payload: %w", err)
			}

			req, err := agents.Capture(agent, event, payload)
			if err != nil {
				return err
			}

			ctx := cmd.Context()
			sq, db, err := a.newStore(ctx)
			if err != nil {
				return err
			}
			defer func() { _ = db.Close() }()
			svc := core.NewService(sq, clock, tokens.Heuristic{}, a.logger)

			// Recall runs BEFORE capturing the current prompt so the prompt cannot
			// match itself. It is best-effort: capture is the invariant, so a
			// recall failure is logged and never aborts the hook.
			if event == agents.EventUserPromptSubmit && len(req.Messages) > 0 && recall > 0 {
				prompt := req.Messages[0].Content
				terms := agents.RecallTerms(prompt)
				candidateLimit := min(recall, maxRecallCandidates/5) * 5
				hits, sErr := svc.Search(ctx, core.SearchQuery{
					Text:  strings.Join(terms, " "),
					Limit: candidateLimit,
					Any:   true,
				})
				switch {
				case sErr != nil:
					a.logger.Warn("recall search failed; continuing with capture", "error", sErr)
				case len(terms) == 0:
				default:
					hits = agents.RankRecall(hits, terms, core.DeriveConversationID(agent, req.SessionID), recall)
					if block := agents.RecallBlock(hits); block != "" {
						out, mErr := agents.AdditionalContextJSON(event, block)
						if mErr != nil {
							return mErr
						}
						if _, wErr := fmt.Fprintln(cmd.OutOrStdout(), string(out)); wErr != nil {
							return wErr
						}
					}
				}
			}

			// Capture.
			if req.SessionID == "" || len(req.Messages) == 0 {
				return nil
			}
			res, err := svc.Ingest(ctx, req)
			if err != nil {
				return err
			}

			// Opportunistic compaction on turn-ending events keeps the summary DAG
			// current without a manual 'acm compact' step. Compact is a cheap no-op
			// below the soft threshold, and a failure here is best-effort by
			// design: the messages are already safely captured.
			if !noCompact && (event == agents.EventStop || event == agents.EventTurnComplete) {
				comp := a.newCompactor(sq, engine.DefaultConfig(), nil)
				if _, cErr := comp.Compact(ctx, res.ConversationID); cErr != nil {
					a.logger.Warn("opportunistic compaction failed", "conversation", res.ConversationID, "error", cErr)
				}
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&agentStr, "agent", "", "host agent: claude-code|codex|opencode")
	cmd.Flags().StringVar(&event, "event", "", "hook event name (e.g. UserPromptSubmit, PostToolUse, agent-turn-complete)")
	cmd.Flags().IntVar(&recall, "recall", 5, "max recalled context items to inject on prompt events")
	cmd.Flags().BoolVar(&noCompact, "no-compact", false, "skip opportunistic compaction after turn-ending events")
	return cmd
}
