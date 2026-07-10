package cli

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/bonztm/agent-context-manager/internal/llmmap"
	"github.com/bonztm/agent-context-manager/internal/summarize"
)

type mapOptions struct {
	input, output, processor, prompt, stateDir string
	require                                    []string
	concurrency, maxRetries                    int
	maxItemBytes, maxOutputBytes               int
	maxItems, maxCalls                         int
	maxInputBytes                              int64
	runTimeout                                 time.Duration
}

func newMapCmd(_ *app) *cobra.Command {
	options := &mapOptions{}
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
		RunE: func(cmd *cobra.Command, _ []string) error { return options.run(cmd) },
	}
	options.bindFlags(cmd)
	return cmd
}

func (options *mapOptions) bindFlags(cmd *cobra.Command) {
	flags := cmd.Flags()
	flags.StringVar(&options.input, "input", "", "input JSONL file (one item per line)")
	flags.StringVar(&options.output, "output", "", "output JSONL file")
	flags.StringVar(&options.processor, "processor", "passthrough", "per-item processor: passthrough|claude|codex")
	flags.StringVar(&options.prompt, "prompt", "", "instruction prepended to each item (for claude/codex)")
	flags.StringSliceVar(&options.require, "require", nil, "comma-separated JSON fields each output must contain")
	flags.IntVar(&options.concurrency, "concurrency", 8, "max concurrent item workers")
	flags.IntVar(&options.maxRetries, "max-retries", 2, "retries per item on error or failed validation")
	flags.Int64Var(&options.maxInputBytes, "max-input-bytes", 1<<30, "maximum total input bytes")
	flags.IntVar(&options.maxItemBytes, "max-item-bytes", 1<<20, "maximum bytes in one input item")
	flags.IntVar(&options.maxOutputBytes, "max-output-bytes", 1<<20, "maximum bytes in one processor output")
	flags.IntVar(&options.maxItems, "max-items", 100_000, "maximum input item count")
	flags.IntVar(&options.maxCalls, "max-calls", 0, "maximum worst-case processor calls (0 disables)")
	flags.DurationVar(&options.runTimeout, "run-timeout", 0, "maximum duration for the complete run (0 disables)")
	flags.StringVar(&options.stateDir, "state-dir", "", "resume state directory (default <output>.acm-map-state)")
}

func (options *mapOptions) run(cmd *cobra.Command) error {
	mapper, err := options.mapper()
	if err != nil {
		return err
	}
	result, err := mapper.Run(cmd.Context(), options.input, options.output)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(cmd.OutOrStdout(), "mapped %d items: %d ok, %d failed (peak buffered: %d) -> %s\n",
		result.Total, result.Succeeded, result.Failed, result.PeakInFlight, options.output)
	return err
}

func (options *mapOptions) mapper() (*llmmap.Mapper, error) {
	if options.input == "" || options.output == "" {
		return nil, errors.New("map: --input and --output are required")
	}
	processor, err := buildProcessor(options.processor, options.prompt)
	if err != nil {
		return nil, err
	}
	configKey, err := mapConfigKey(options.processor, options.prompt, options.require)
	if err != nil {
		return nil, err
	}
	return &llmmap.Mapper{
		Concurrency: options.concurrency, MaxRetries: options.maxRetries,
		MaxInputBytes: options.maxInputBytes, MaxItemBytes: options.maxItemBytes,
		MaxOutputBytes: options.maxOutputBytes, MaxItems: options.maxItems,
		MaxCalls: options.maxCalls, RunTimeout: options.runTimeout,
		StateDir: options.stateDir, ConfigKey: configKey,
		Process: processor, Validate: mapValidator(options.require),
	}, nil
}

func mapValidator(require []string) llmmap.Validator {
	if len(require) == 0 {
		return nil
	}
	return llmmap.RequireFields(require...)
}

func mapConfigKey(processor, prompt string, require []string) (string, error) {
	contract := struct {
		Processor string   `json:"processor"`
		Prompt    string   `json:"prompt"`
		Require   []string `json:"require"`
	}{Processor: processor, Prompt: prompt, Require: require}
	encoded, err := json.Marshal(contract)
	if err != nil {
		return "", fmt.Errorf("map: encode processor contract: %w", err)
	}
	return fmt.Sprintf("%x", sha256.Sum256(encoded)), nil
}

func buildProcessor(name, prompt string) (llmmap.Processor, error) {
	switch name {
	case "passthrough":
		return func(_ context.Context, item json.RawMessage, _ int, _ string) (json.RawMessage, error) {
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
// mode, prepending the instruction prompt to the item JSON. On retries the
// previous attempt's error is fed back so the model can correct its output.
func execProcessor(argv []string, promptOnStdin bool, prompt string) llmmap.Processor {
	return func(ctx context.Context, item json.RawMessage, _ int, feedback string) (json.RawMessage, error) {
		full := prompt + "\n\nITEM:\n" + string(item)
		if feedback != "" {
			full += "\n\nYour previous attempt failed: " + feedback + "\nCorrect the output."
		}
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
