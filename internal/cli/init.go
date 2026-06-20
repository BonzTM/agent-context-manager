package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/bonztm/agent-context-manager/internal/agents"
	"github.com/bonztm/agent-context-manager/internal/config"
	"github.com/bonztm/agent-context-manager/internal/core"
)

func newInitCmd(a *app) *cobra.Command {
	return &cobra.Command{
		Use:     "init <agent>",
		GroupID: groupSetup,
		Short:   "Generate integration assets (hooks, drill-down instructions) for an agent",
		Long: "Generates the per-agent hook configuration and drill-down instruction assets\n" +
			"under <project>/.acm/init/<agent>/ and prints next steps. It never mutates your\n" +
			"existing agent configuration — it writes snippets for you to review and merge,\n" +
			"so nothing is clobbered.\n\n" +
			"Supported agents: claude-code, codex, opencode.\n\n" +
			"The generated assets wire capture-and-recall hooks and document the drill-down\n" +
			"commands (acm expand/grep/describe) for the model. See docs/integrations.md for\n" +
			"the per-agent merge steps.",
		Example: `  acm init claude-code
  acm init codex
  acm init opencode`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			agent := core.Agent(args[0])
			plan, err := agents.BuildInit(agent)
			if err != nil {
				return err
			}

			dir := filepath.Join(a.cfg.ProjectRoot, config.DirName, "init", string(agent))
			if mErr := os.MkdirAll(dir, 0o750); mErr != nil {
				return fmt.Errorf("init: create dir: %w", mErr)
			}

			out := cmd.OutOrStdout()
			for _, as := range plan.Assets {
				path := filepath.Join(dir, as.RelPath)
				if dErr := os.MkdirAll(filepath.Dir(path), 0o750); dErr != nil {
					return fmt.Errorf("init: create asset dir: %w", dErr)
				}
				if wErr := os.WriteFile(path, []byte(as.Content), 0o600); wErr != nil {
					return fmt.Errorf("init: write asset: %w", wErr)
				}
				fmt.Fprintf(out, "wrote %s\n", path)
			}
			fmt.Fprintln(out, plan.Instructions)
			return nil
		},
	}
}
