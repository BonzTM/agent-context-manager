package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/bonztm/agent-context-manager/internal/store"
)

func newDescribeCmd(a *app) *cobra.Command {
	var asJSON bool
	cmd := &cobra.Command{
		Use:     "describe <id>",
		GroupID: groupRetrieval,
		Short:   "Show metadata and content for a message, summary, or offloaded file",
		Long: "Looks up an entity by ID and prints its metadata and content. The ID prefix\n" +
			"selects the kind:\n" +
			"  msg_   a verbatim message (full content)\n" +
			"  sum_   a summary node (metadata + summary text; expand it with 'acm expand')\n" +
			"  file_  an offloaded large file (path + exploration summary; read it with cat)\n\n" +
			"This is a drill-down command the agent runs through its shell tool.",
		Example: `  acm describe msg_1a2b3c
  acm describe sum_9f8e7d --json`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			sq, db, err := a.newStore(ctx)
			if err != nil {
				return err
			}
			defer func() { _ = db.Close() }()

			id := args[0]
			out := cmd.OutOrStdout()
			switch {
			case strings.HasPrefix(id, "sum_"):
				return describeSummary(ctx, out, sq, id, asJSON)
			case strings.HasPrefix(id, "file_"):
				return describeFile(ctx, out, sq, id, asJSON)
			default:
				return describeMessage(ctx, out, sq, id, asJSON)
			}
		},
	}
	cmd.Flags().BoolVar(&asJSON, "json", false, "emit the entity as JSON")
	return cmd
}

func describeMessage(ctx context.Context, out io.Writer, sq *store.SQLite, id string, asJSON bool) error {
	msg, err := sq.GetMessage(ctx, id)
	if err != nil {
		return err
	}
	if asJSON {
		return json.NewEncoder(out).Encode(msg)
	}
	fmt.Fprintf(out, "id:           %s\n", msg.ID)
	fmt.Fprintf(out, "conversation: %s\n", msg.ConversationID)
	fmt.Fprintf(out, "seq:          %d\n", msg.Seq)
	fmt.Fprintf(out, "role:         %s\n", msg.Role)
	fmt.Fprintf(out, "tokens:       %d\n", msg.TokenCount)
	fmt.Fprintf(out, "created:      %s\n", msg.CreatedAt.Format(time.RFC3339))
	if msg.ToolName != "" {
		fmt.Fprintf(out, "tool:         %s\n", msg.ToolName)
	}
	if msg.ExternalID != "" {
		fmt.Fprintf(out, "external_id:  %s\n", msg.ExternalID)
	}
	fmt.Fprintln(out, "---")
	fmt.Fprintln(out, msg.Content)
	return nil
}

func describeSummary(ctx context.Context, out io.Writer, sq *store.SQLite, id string, asJSON bool) error {
	sum, err := sq.GetSummary(ctx, id)
	if err != nil {
		return err
	}
	if asJSON {
		return json.NewEncoder(out).Encode(sum)
	}
	fmt.Fprintf(out, "id:            %s\n", sum.ID)
	fmt.Fprintf(out, "conversation:  %s\n", sum.ConversationID)
	fmt.Fprintf(out, "kind:          %s\n", sum.Kind)
	fmt.Fprintf(out, "depth:         %d\n", sum.Depth)
	fmt.Fprintf(out, "tokens:        %d\n", sum.TokenCount)
	fmt.Fprintf(out, "sources:       %d\n", sum.SourceCount)
	fmt.Fprintf(out, "covers msgs:   %d (seq %d-%d)\n", sum.DescendantMessageCount, sum.EarliestSeq, sum.LatestSeq)
	fmt.Fprintf(out, "expand with:   acm expand %s\n", sum.ID)
	fmt.Fprintln(out, "---")
	fmt.Fprintln(out, sum.Content)
	return nil
}

func describeFile(ctx context.Context, out io.Writer, sq *store.SQLite, id string, asJSON bool) error {
	lf, err := sq.GetLargeFile(ctx, id)
	if err != nil {
		return err
	}
	if asJSON {
		return json.NewEncoder(out).Encode(lf)
	}
	fmt.Fprintf(out, "id:           %s\n", lf.ID)
	fmt.Fprintf(out, "conversation: %s\n", lf.ConversationID)
	fmt.Fprintf(out, "message:      %s\n", lf.MessageID)
	fmt.Fprintf(out, "path:         %s\n", lf.StorageURI)
	fmt.Fprintf(out, "bytes:        %d\n", lf.ByteSize)
	fmt.Fprintf(out, "tokens:       %d\n", lf.TokenCount)
	if lf.Extractor != "" {
		fmt.Fprintf(out, "extractor:    %s\n", lf.Extractor)
	}
	fmt.Fprintln(out, "--- exploration summary (read full content with: cat <path>) ---")
	fmt.Fprintln(out, lf.ExplorationSummary)
	return nil
}
