package cli

import (
	"errors"
	"fmt"
	"io"

	"github.com/spf13/cobra"

	"github.com/bonztm/agent-context-manager/internal/agents"
	"github.com/bonztm/agent-context-manager/internal/core"
)

func newHookCmd(a *app) *cobra.Command {
	var (
		agentStr string
		event    string
		recall   int
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
			"  agent-turn-complete    (Codex notify) capture user + final assistant message\n" +
			"  SessionStart, Stop     no-op for capture\n\n" +
			"Recall searches the project history (OR over prompt terms) before the prompt\n" +
			"is stored, so the prompt cannot match itself.",
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

			payload, err := io.ReadAll(cmd.InOrStdin())
			if err != nil {
				return fmt.Errorf("hook: read payload: %w", err)
			}

			req, err := agents.Capture(agent, event, payload)
			if err != nil {
				return err
			}

			ctx := cmd.Context()
			svc, db, err := a.newService(ctx)
			if err != nil {
				return err
			}
			defer func() { _ = db.Close() }()

			// Recall runs BEFORE capturing the current prompt so the prompt cannot
			// match itself.
			if event == agents.EventUserPromptSubmit && len(req.Messages) > 0 {
				prompt := req.Messages[0].Content
				hits, sErr := svc.Search(ctx, core.SearchQuery{Text: prompt, Limit: recall, Any: true})
				if sErr != nil {
					return sErr
				}
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

			// Capture.
			if req.SessionID != "" && len(req.Messages) > 0 {
				if _, iErr := svc.Ingest(ctx, req); iErr != nil {
					return iErr
				}
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&agentStr, "agent", "", "host agent: claude-code|codex|opencode")
	cmd.Flags().StringVar(&event, "event", "", "hook event name (e.g. UserPromptSubmit, PostToolUse, agent-turn-complete)")
	cmd.Flags().IntVar(&recall, "recall", 5, "max recalled context items to inject on prompt events")
	return cmd
}
