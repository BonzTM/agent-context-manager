package cli

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/spf13/cobra"

	"github.com/bonztm/agent-context-manager/internal/core"
)

// ingestPayload is the JSON acm ingest reads from stdin. It is the generic
// ingestion entrypoint the agent hook adapters pipe captured messages into.
type ingestPayload struct {
	Agent     string                 `json:"agent"`
	SessionID string                 `json:"session_id"`
	Title     string                 `json:"title"`
	Messages  []ingestMessagePayload `json:"messages"`
}

type ingestMessagePayload struct {
	Role       string `json:"role"`
	Content    string `json:"content"`
	ToolName   string `json:"tool_name"`
	ExternalID string `json:"external_id"`
	Raw        string `json:"raw"`
}

func newIngestCmd(a *app) *cobra.Command {
	var asJSON bool
	cmd := &cobra.Command{
		Use:     "ingest",
		GroupID: groupCapture,
		Short:   "Ingest captured messages (JSON on stdin) into the lossless store",
		Long: "Reads a JSON ingestion payload from stdin, applies the project privacy\n" +
			"policy, and stores retained messages while computing token counts and skipping\n" +
			"duplicates. This is the generic capture\n" +
			"entrypoint the per-agent hook adapters and the OpenCode plugin pipe turns into.\n\n" +
			"Payload shape:\n" +
			"  {\n" +
			"    \"agent\": \"claude-code|codex|opencode\",\n" +
			"    \"session_id\": \"<session>\",\n" +
			"    \"title\": \"<optional>\",\n" +
			"    \"messages\": [\n" +
			"      {\"role\": \"user|assistant|tool\", \"content\": \"...\",\n" +
			"       \"tool_name\": \"<optional>\", \"external_id\": \"<optional>\"}\n" +
			"    ]\n" +
			"  }",
		Example: `  echo '{"agent":"codex","session_id":"s1","messages":[{"role":"user","content":"hi"}]}' | acm ingest
  acm ingest --json < payload.json`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			raw, err := io.ReadAll(cmd.InOrStdin())
			if err != nil {
				return fmt.Errorf("read stdin: %w", err)
			}
			var p ingestPayload
			if err = json.Unmarshal(raw, &p); err != nil {
				return fmt.Errorf("parse ingest payload: %w", err)
			}

			req := core.IngestRequest{
				Agent:     core.Agent(p.Agent),
				SessionID: p.SessionID,
				Title:     p.Title,
				Messages:  make([]core.IngestMessage, 0, len(p.Messages)),
			}
			for _, m := range p.Messages {
				req.Messages = append(req.Messages, core.IngestMessage{
					Role:       core.Role(m.Role),
					Content:    m.Content,
					ToolName:   m.ToolName,
					ExternalID: m.ExternalID,
					Raw:        m.Raw,
				})
			}

			ctx := cmd.Context()
			svc, db, err := a.newService(ctx)
			if err != nil {
				return err
			}
			defer func() { _ = db.Close() }()

			res, err := svc.Ingest(ctx, req)
			if err != nil {
				return err
			}

			out := cmd.OutOrStdout()
			if asJSON {
				return json.NewEncoder(out).Encode(res)
			}
			fmt.Fprintf(out, "conversation %s: appended %d, deduped %d, excluded %d, redacted %d, tokens %d\n",
				res.ConversationID, res.Appended, res.Deduped, res.Excluded, res.Redacted, res.Tokens)
			return nil
		},
	}
	cmd.Flags().BoolVar(&asJSON, "json", false, "emit the result as JSON")
	return cmd
}
