package cli

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/bonztm/agent-context-manager/internal/agentmap"
	"github.com/bonztm/agent-context-manager/internal/llmmap"
	"github.com/bonztm/agent-context-manager/internal/summarize"
)

type mapOptions struct {
	input, output, processor, prompt, stateDir string
	schemaPath                                 string
	require                                    []string
	concurrency, maxRetries                    int
	maxItemBytes, maxOutputBytes               int
	maxItems, maxCalls                         int
	maxTools, maxTurns                         int
	maxInputBytes                              int64
	runTimeout, itemTimeout                    time.Duration
}

const (
	defaultAgentMaxCalls   = 100
	defaultAgentRunTimeout = time.Hour
)

func newMapCmd(_ *app) *cobra.Command {
	options := &mapOptions{}
	cmd := &cobra.Command{
		Use:     "map",
		GroupID: groupBatch,
		Short:   "Process a JSONL dataset off-context with a bounded worker pool",
		Long: "Reads --input as JSONL (one JSON item per line), processes each item\n" +
			"independently with bounded concurrency and validated retries, and writes a\n" +
			"result line per item to --output. The dataset never enters the agent's context\n" +
			"window; only one item enters each processor attempt.\n\n" +
			"Processors:\n" +
			"  passthrough  (default) echo each item unchanged — offline demo / validation\n" +
			"  claude|codex reuse that agent's model for a single response\n" +
			"  claude-agent|codex-agent run a bounded read-only tool-using session\n\n" +
			"Each output line is {index, ok, output, error, attempts}; results preserve\n" +
			"input order. --require and --schema validate output; failures are fed back\n" +
			"into retries up to --max-retries.",
		Example: `  acm map --input items.jsonl --output results.jsonl
  acm map --input files.jsonl --output out.jsonl --processor claude \
    --prompt "Classify this file. Return {\"label\": ...}" --require label
  acm map --input tasks.jsonl --output findings.jsonl --processor codex-agent \
    --schema finding.schema.json --max-tools 12 --max-turns 6`,
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
	flags.StringVar(&options.processor, "processor", "passthrough", "processor: passthrough|claude|codex|claude-agent|codex-agent")
	flags.StringVar(&options.prompt, "prompt", "", "instruction prepended to each item")
	flags.StringSliceVar(&options.require, "require", nil, "comma-separated JSON fields each output must contain")
	flags.StringVar(&options.schemaPath, "schema", "", "JSON Schema file for output validation")
	flags.IntVar(&options.concurrency, "concurrency", 8, "max concurrent item workers")
	flags.IntVar(&options.maxRetries, "max-retries", 2, "retries per item on error or failed validation")
	flags.Int64Var(&options.maxInputBytes, "max-input-bytes", 1<<30, "maximum total input bytes")
	flags.IntVar(&options.maxItemBytes, "max-item-bytes", 1<<20, "maximum bytes in one input item")
	flags.IntVar(&options.maxOutputBytes, "max-output-bytes", 1<<20, "maximum bytes in one processor output")
	flags.IntVar(&options.maxItems, "max-items", 100_000, "maximum input item count")
	flags.IntVar(&options.maxCalls, "max-calls", 0, "maximum worst-case processor calls (0 disables)")
	flags.DurationVar(&options.runTimeout, "run-timeout", 0, "maximum duration for the complete run (0 disables)")
	flags.StringVar(&options.stateDir, "state-dir", "", "resume state directory (default <output>.acm-map-state)")
	flags.IntVar(&options.maxTools, "max-tools", 16, "maximum tool calls per agentic attempt")
	flags.IntVar(&options.maxTurns, "max-turns", 8, "maximum turns per agentic attempt")
	flags.DurationVar(&options.itemTimeout, "item-timeout", 2*time.Minute, "maximum duration per agentic attempt")
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
	schemaValidator, schemaHash, schemaPath, err := loadMapSchema(options.schemaPath)
	if err != nil {
		return nil, err
	}
	processor, err := options.buildProcessor(schemaPath)
	if err != nil {
		return nil, err
	}
	configKey, err := mapConfigKey(options, schemaHash)
	if err != nil {
		return nil, err
	}
	return &llmmap.Mapper{
		Concurrency: options.concurrency, MaxRetries: options.maxRetries,
		MaxInputBytes: options.maxInputBytes, MaxItemBytes: options.maxItemBytes,
		MaxOutputBytes: options.maxOutputBytes, MaxItems: options.maxItems,
		MaxCalls: options.effectiveMaxCalls(), RunTimeout: options.effectiveRunTimeout(),
		StateDir: options.stateDir, ConfigKey: configKey,
		Process: processor, Validate: llmmap.CombineValidators(mapValidator(options.require), schemaValidator),
	}, nil
}

func loadMapSchema(path string) (llmmap.Validator, string, string, error) {
	validate, hash, err := llmmap.LoadJSONSchema(path)
	if err != nil || path == "" {
		return validate, hash, path, err
	}
	absolute, err := filepath.Abs(path)
	if err != nil {
		return nil, "", "", fmt.Errorf("map: resolve output schema: %w", err)
	}
	return validate, hash, absolute, nil
}

func (options *mapOptions) buildProcessor(schemaPath string) (llmmap.Processor, error) {
	if !options.isAgentic() {
		return buildProcessor(options.processor, options.prompt)
	}
	request := agentmap.Request{
		Host: agentmap.Host(options.processor), SchemaPath: schemaPath,
		MaxTools: options.maxTools, MaxTurns: options.maxTurns, Timeout: options.itemTimeout,
	}
	if err := request.Validate(); err != nil {
		return nil, err
	}
	return func(ctx context.Context, item json.RawMessage, _ int, feedback string) (json.RawMessage, error) {
		attemptRequest := request
		attemptRequest.Prompt = itemPrompt(options.prompt, item, feedback, true)
		return agentmap.Run(ctx, attemptRequest)
	}, nil
}

func (options *mapOptions) isAgentic() bool {
	return options.processor == string(agentmap.HostClaude) || options.processor == string(agentmap.HostCodex)
}

func (options *mapOptions) effectiveMaxCalls() int {
	if options.isAgentic() && options.maxCalls == 0 {
		return defaultAgentMaxCalls
	}
	return options.maxCalls
}

func (options *mapOptions) effectiveRunTimeout() time.Duration {
	if options.isAgentic() && options.runTimeout == 0 {
		return defaultAgentRunTimeout
	}
	return options.runTimeout
}

func mapValidator(require []string) llmmap.Validator {
	if len(require) == 0 {
		return nil
	}
	return llmmap.RequireFields(require...)
}

func mapConfigKey(options *mapOptions, schemaHash string) (string, error) {
	contract := struct {
		Processor   string        `json:"processor"`
		Prompt      string        `json:"prompt"`
		Require     []string      `json:"require"`
		SchemaHash  string        `json:"schema_hash"`
		MaxTools    int           `json:"max_tools"`
		MaxTurns    int           `json:"max_turns"`
		ItemTimeout time.Duration `json:"item_timeout"`
	}{
		Processor: options.processor, Prompt: options.prompt, Require: options.require,
		SchemaHash: schemaHash, MaxTools: options.maxTools, MaxTurns: options.maxTurns,
		ItemTimeout: options.itemTimeout,
	}
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
		return nil, fmt.Errorf("unknown processor %q (want passthrough|claude|codex|claude-agent|codex-agent)", name)
	}
}

// execProcessor reuses a host agent's model to process each item in headless
// mode, prepending the instruction prompt to the item JSON. On retries the
// previous attempt's error is fed back so the model can correct its output.
func execProcessor(argv []string, promptOnStdin bool, prompt string) llmmap.Processor {
	return func(ctx context.Context, item json.RawMessage, _ int, feedback string) (json.RawMessage, error) {
		full := itemPrompt(prompt, item, feedback, false)
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

func itemPrompt(prompt string, item json.RawMessage, feedback string, agentic bool) string {
	full := prompt + "\n\nITEM:\n" + string(item)
	if agentic {
		full += "\n\nUse only read-only tools. Return only the final JSON value."
	}
	if feedback != "" {
		full += "\n\nYour previous attempt failed: " + feedback + "\nCorrect the output."
	}
	return full
}
