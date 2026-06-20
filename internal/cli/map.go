package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/bonztm/agent-context-manager/internal/llmmap"
	"github.com/bonztm/agent-context-manager/internal/summarize"
)

func newMapCmd(_ *app) *cobra.Command {
	var (
		input       string
		output      string
		processor   string
		prompt      string
		require     []string
		concurrency int
		maxRetries  int
	)
	cmd := &cobra.Command{
		Use:     "map",
		GroupID: groupBatch,
		Short:   "Process a JSONL dataset off-context with a bounded worker pool",
		Long: "Reads --input as JSONL (one JSON item per line), processes each item\n" +
			"independently with bounded concurrency and validated retries, and writes a\n" +
			"result line per item to --output. The dataset never enters the agent's context\n" +
			"window, so accuracy stays stable regardless of size.\n\n" +
			"Processors:\n" +
			"  passthrough  (default) echo each item unchanged — offline demo / validation\n" +
			"  claude|codex reuse that agent's model in headless mode, prepending --prompt\n\n" +
			"Each output line is {index, ok, output, error, attempts}; results preserve\n" +
			"input order. --require lists JSON fields each output must contain (items that\n" +
			"fail validation are retried up to --max-retries).",
		Example: `  acm map --input items.jsonl --output results.jsonl
  acm map --input files.jsonl --output out.jsonl --processor claude \
    --prompt "Classify this file. Return {\"label\": ...}" --require label`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if input == "" || output == "" {
				return errors.New("map: --input and --output are required")
			}
			proc, err := buildProcessor(processor, prompt)
			if err != nil {
				return err
			}
			var validate llmmap.Validator
			if len(require) > 0 {
				validate = llmmap.RequireFields(require...)
			}

			mapper := &llmmap.Mapper{
				Concurrency: concurrency,
				MaxRetries:  maxRetries,
				Process:     proc,
				Validate:    validate,
			}
			res, err := mapper.Run(cmd.Context(), input, output)
			if err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "mapped %d items: %d ok, %d failed -> %s\n",
				res.Total, res.Succeeded, res.Failed, output)
			return nil
		},
	}
	cmd.Flags().StringVar(&input, "input", "", "input JSONL file (one item per line)")
	cmd.Flags().StringVar(&output, "output", "", "output JSONL file")
	cmd.Flags().StringVar(&processor, "processor", "passthrough", "per-item processor: passthrough|claude|codex")
	cmd.Flags().StringVar(&prompt, "prompt", "", "instruction prepended to each item (for claude/codex)")
	cmd.Flags().StringSliceVar(&require, "require", nil, "comma-separated JSON fields each output must contain")
	cmd.Flags().IntVar(&concurrency, "concurrency", 8, "max concurrent item workers")
	cmd.Flags().IntVar(&maxRetries, "max-retries", 2, "retries per item on error or failed validation")
	return cmd
}

func buildProcessor(name, prompt string) (llmmap.Processor, error) {
	switch name {
	case "passthrough":
		return func(_ context.Context, item json.RawMessage, _ int) (json.RawMessage, error) {
			return item, nil
		}, nil
	case "claude":
		return execProcessor([]string{"claude", "-p", "--model", "haiku", "--output-format", "text"}, true, prompt), nil
	case "codex":
		return execProcessor([]string{"codex", "exec", "-c", "model=gpt-5.4-mini"}, false, prompt), nil
	default:
		return nil, fmt.Errorf("unknown processor %q (want passthrough|claude|codex)", name)
	}
}

// execProcessor reuses a host agent's model to process each item in headless
// mode, prepending the instruction prompt to the item JSON.
func execProcessor(argv []string, promptOnStdin bool, prompt string) llmmap.Processor {
	return func(ctx context.Context, item json.RawMessage, _ int) (json.RawMessage, error) {
		full := prompt + "\n\nITEM:\n" + string(item)
		runArgv := argv
		stdin := ""
		if promptOnStdin {
			stdin = full
		} else {
			runArgv = append(append([]string{}, argv...), full)
		}
		out, err := summarize.ExecRunner(ctx, runArgv, stdin)
		if err != nil {
			return nil, err
		}
		return json.RawMessage(strings.TrimSpace(out)), nil
	}
}
