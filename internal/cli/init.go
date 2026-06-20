package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/bonztm/agent-context-manager/internal/agents"
	"github.com/bonztm/agent-context-manager/internal/config"
	"github.com/bonztm/agent-context-manager/internal/core"
	"github.com/bonztm/agent-context-manager/internal/install"
)

func newInitCmd(a *app) *cobra.Command {
	var (
		global bool
		dryRun bool
	)
	cmd := &cobra.Command{
		Use:     "init <agent>",
		GroupID: groupSetup,
		Short:   "Generate or install integration for an agent (hooks + drill-down)",
		Long: "Sets up acm integration for an agent: capture-and-recall hooks plus the\n" +
			"drill-down instructions that document acm's recovery commands.\n\n" +
			"Modes:\n" +
			"  (default)          Generate per-project snippets under\n" +
			"                     <project>/.acm/init/<agent>/ for you to merge. Never\n" +
			"                     touches existing config.\n" +
			"  --global           Install into the agent's user-level config so every\n" +
			"                     project is covered by one install. Applies by default;\n" +
			"                     safely merges acm's hooks/plugin and drill-down\n" +
			"                     instructions (idempotent; existing keys preserved;\n" +
			"                     invalid configs are never overwritten).\n" +
			"  --global --dry-run Preview the exact global changes without writing.\n\n" +
			"With a single global install, acm captures into whichever project you are\n" +
			"working in — the database is resolved from the working directory at hook time,\n" +
			"and a .acm/ directory is created on first write.\n\n" +
			"Supported agents: claude-code, codex, opencode.",
		Example: `  acm init claude-code                   # project snippets to merge
  acm init claude-code --global --dry-run # preview a global install
  acm init claude-code --global           # install globally for every project`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			agent := core.Agent(args[0])
			if global {
				return runGlobalInit(cmd, agent, !dryRun)
			}
			return runProjectInit(cmd, a, agent)
		},
	}
	cmd.Flags().BoolVar(&global, "global", false, "install into the agent's user-level config (covers every project)")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "with --global, preview the changes without writing")
	return cmd
}

func runGlobalInit(cmd *cobra.Command, agent core.Agent, apply bool) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("init: resolve home directory: %w", err)
	}
	res, err := install.Run(agent, home, apply)
	if err != nil {
		return err
	}

	out := cmd.OutOrStdout()
	for _, c := range res.Changes {
		marker := "•"
		if c.Skipped {
			marker = "–"
		}
		fmt.Fprintf(out, "%s %s\n    %s\n", marker, c.Summary, c.Path)
	}
	for _, n := range res.Notes {
		fmt.Fprintf(out, "note: %s\n", n)
	}
	if !apply {
		fmt.Fprintln(out, "\nDry run — no files were changed. Run without --dry-run to install.")
	} else {
		fmt.Fprintln(out, "\nInstalled. Restart the agent to load the new configuration.")
	}
	return nil
}

func runProjectInit(cmd *cobra.Command, a *app, agent core.Agent) error {
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
}
