package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"

	"github.com/spf13/cobra"

	"github.com/bonztm/agent-context-manager/internal/agents"
	"github.com/bonztm/agent-context-manager/internal/core"
	"github.com/bonztm/agent-context-manager/internal/engine"
	opencodectx "github.com/bonztm/agent-context-manager/internal/opencode"
	"github.com/bonztm/agent-context-manager/internal/privacy"
	"github.com/bonztm/agent-context-manager/internal/store"
)

const maxOpenCodeContextInput = 1 << 20

type openCodeContextInput struct {
	SessionID string `json:"session_id"`
	Prompt    string `json:"prompt"`
}

func newOpenCodeContextCmd(a *app) *cobra.Command {
	return &cobra.Command{
		Use:     "opencode-context",
		GroupID: groupCapture,
		Short:   "Build OpenCode's active-window and automatic-recall plan",
		Long: "Reads {session_id,prompt} JSON on stdin and returns the versioned context\n" +
			"plan consumed by ACM's embedded OpenCode plugin. The command archives context\n" +
			"outside the protected fresh tail, assembles summary roots, and selects recall.\n" +
			"It is an integration protocol; interactive use is primarily for diagnostics.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			input, err := readOpenCodeContextInput(cmd.InOrStdin())
			if err != nil {
				return err
			}
			plan, err := buildOpenCodeContextPlan(cmd.Context(), a, input)
			if err != nil {
				return err
			}
			return json.NewEncoder(cmd.OutOrStdout()).Encode(plan)
		},
	}
}

func readOpenCodeContextInput(reader io.Reader) (openCodeContextInput, error) {
	content, err := io.ReadAll(io.LimitReader(reader, maxOpenCodeContextInput+1))
	if err != nil {
		return openCodeContextInput{}, fmt.Errorf("opencode-context: read input: %w", err)
	}
	if len(content) > maxOpenCodeContextInput {
		return openCodeContextInput{}, fmt.Errorf("opencode-context: input exceeds %d bytes", maxOpenCodeContextInput)
	}
	var input openCodeContextInput
	if err := json.Unmarshal(content, &input); err != nil {
		return input, fmt.Errorf("opencode-context: decode input: %w", err)
	}
	if input.SessionID == "" {
		return input, errors.New("opencode-context: session_id is required")
	}
	return input, nil
}

func buildOpenCodeContextPlan(ctx context.Context, a *app, input openCodeContextInput) (opencodectx.Plan, error) {
	config := engine.DefaultConfig()
	sq, db, err := a.newStore(ctx)
	if err != nil {
		return opencodectx.Plan{}, err
	}
	defer func() { _ = db.Close() }()
	mode := a.policy.Mode(input.SessionID)
	if mode == privacy.SessionIgnore {
		return opencodectx.BuildPlan(nil, nil, "", config.FreshTailMessages)
	}
	conversationID := core.DeriveConversationID(core.AgentOpenCode, input.SessionID)
	var items []engine.RenderedItem
	if mode == privacy.SessionCapture {
		compactor := a.newCompactor(sq, config, nil)
		if _, archiveErr := compactor.ArchiveToFreshTail(ctx, conversationID); archiveErr != nil {
			return opencodectx.Plan{}, fmt.Errorf("opencode-context: archive: %w", archiveErr)
		}
		items, err = compactor.Assemble(ctx, conversationID)
		if err != nil {
			return opencodectx.Plan{}, fmt.Errorf("opencode-context: assemble: %w", err)
		}
	}
	recall, err := openCodeRecall(ctx, sq, input, config)
	if err != nil {
		return opencodectx.Plan{}, err
	}
	externalIDs, err := activeExternalIDs(ctx, sq, items)
	if err != nil {
		return opencodectx.Plan{}, err
	}
	return opencodectx.BuildPlan(items, externalIDs, agents.RecallBlock(recall), config.FreshTailMessages)
}

func openCodeRecall(ctx context.Context, sq *store.SQLite, input openCodeContextInput, config engine.Config) ([]agents.RecallHit, error) {
	hits, err := agents.AutomaticRecall(ctx, sq, agents.AutomaticRecallRequest{
		Agent: core.AgentOpenCode, SessionID: input.SessionID, Prompt: input.Prompt,
		Limit: 5, FreshTailMessages: config.FreshTailMessages,
		FreshTailTokens: config.FreshTailTokens, Now: clock.Now(),
	})
	if err != nil {
		return nil, fmt.Errorf("opencode-context: recall: %w", err)
	}
	return hits, nil
}

func activeExternalIDs(ctx context.Context, sq *store.SQLite, items []engine.RenderedItem) (map[string]string, error) {
	if len(items) > 4096 {
		return nil, errors.New("opencode-context: active window exceeds 4096 items")
	}
	external := make(map[string]string, len(items))
	for _, item := range items {
		if item.Type != core.ContextMessage {
			continue
		}
		message, err := sq.GetMessage(ctx, item.RefID)
		if err != nil {
			return nil, fmt.Errorf("opencode-context: load active message: %w", err)
		}
		external[item.RefID] = message.ExternalID
	}
	return external, nil
}
