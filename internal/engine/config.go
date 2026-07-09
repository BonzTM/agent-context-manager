package engine

import (
	"errors"
	"fmt"
)

const (
	maxCondenseFanout = 64
	maxSummaryDepth   = 32
	maxIterations     = 10_000
)

// Validate rejects compaction policies that cannot converge within their
// declared budget or that remove the engine's finite work bounds.
func (cfg Config) Validate() error {
	if err := cfg.validateBudget(); err != nil {
		return err
	}
	if err := cfg.validateChunking(); err != nil {
		return err
	}
	return cfg.validateBounds()
}

func (cfg Config) validateBudget() error {
	if cfg.ModelContextTokens <= 0 {
		return fmt.Errorf("engine: model context tokens must be positive, got %d", cfg.ModelContextTokens)
	}
	if cfg.SoftFraction <= 0 || cfg.SoftFraction >= 1 {
		return fmt.Errorf("engine: soft fraction must be between 0 and 1, got %g", cfg.SoftFraction)
	}
	if cfg.HardFraction <= cfg.SoftFraction || cfg.HardFraction > 1 {
		return fmt.Errorf("engine: hard fraction must be greater than soft fraction and at most 1, got %g", cfg.HardFraction)
	}
	softTokens := int(cfg.SoftFraction * float64(cfg.ModelContextTokens))
	if cfg.LeafTargetTokens >= softTokens || cfg.CondensedTargetTokens >= softTokens {
		return fmt.Errorf("engine: summary targets (%d leaf, %d condensed) must be smaller than the %d-token soft budget; lower target flags or raise --model-context-tokens", cfg.LeafTargetTokens, cfg.CondensedTargetTokens, softTokens)
	}
	return nil
}

func (cfg Config) validateChunking() error {
	if cfg.LeafChunkTokens <= 0 || cfg.LeafTargetTokens <= 0 || cfg.LeafTargetTokens >= cfg.LeafChunkTokens {
		return fmt.Errorf("engine: leaf target must be positive and smaller than leaf chunk (%d >= %d)", cfg.LeafTargetTokens, cfg.LeafChunkTokens)
	}
	if cfg.CondenseChunkTokens <= 0 || cfg.CondensedTargetTokens <= 0 || cfg.CondensedTargetTokens >= cfg.CondenseChunkTokens {
		return fmt.Errorf("engine: condensed target must be positive and smaller than condensed chunk (%d >= %d)", cfg.CondensedTargetTokens, cfg.CondenseChunkTokens)
	}
	if cfg.TruncateTokens <= 0 || cfg.TruncateTokens >= min(cfg.LeafChunkTokens, cfg.CondenseChunkTokens) {
		return fmt.Errorf("engine: truncate tokens must be positive and smaller than both chunk limits, got %d", cfg.TruncateTokens)
	}
	return nil
}

func (cfg Config) validateBounds() error {
	if cfg.FreshTailMessages < 0 || cfg.FreshTailTokens < 0 || cfg.LargeFileThreshold < 0 {
		return errors.New("engine: fresh-tail and large-file limits cannot be negative")
	}
	if cfg.CondenseFanout < 2 || cfg.CondenseFanout > maxCondenseFanout {
		return fmt.Errorf("engine: condense fanout must be between 2 and %d, got %d", maxCondenseFanout, cfg.CondenseFanout)
	}
	if cfg.MaxDepth <= 0 || cfg.MaxDepth > maxSummaryDepth {
		return fmt.Errorf("engine: max depth must be between 1 and %d, got %d", maxSummaryDepth, cfg.MaxDepth)
	}
	if cfg.MaxIterations <= 0 || cfg.MaxIterations > maxIterations {
		return fmt.Errorf("engine: max iterations must be between 1 and %d, got %d", maxIterations, cfg.MaxIterations)
	}
	return nil
}
