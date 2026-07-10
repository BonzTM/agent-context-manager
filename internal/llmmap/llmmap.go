// Package llmmap implements bounded, streaming, resumable off-context map runs.
package llmmap

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"
)

const (
	defaultConcurrency = 8
	defaultInputBytes  = 1 << 30
	defaultItemBytes   = 1 << 20
	defaultOutputBytes = 1 << 20
	defaultMaxItems    = 100_000
	defaultRunTimeout  = 0
	hardConcurrency    = 64
	hardInputBytes     = 8 << 30
	hardItemBytes      = 8 << 20
	hardMaxItems       = 1_000_000
	hardMaxLines       = 2_000_000
	hardMaxCalls       = 10_000_000
	hardAttempts       = 10
	hardRunTimeout     = 24 * time.Hour
	maxErrorBytes      = 4 << 10
	maxManifestBytes   = 64 << 10
	maxStateBytes      = hardItemBytes + maxErrorBytes + (64 << 10)
	stateVersion       = 1
)

// Processor handles a single item. attempt starts at 1; feedback carries the
// prior processing or validation error.
type Processor func(ctx context.Context, item json.RawMessage, attempt int, feedback string) (json.RawMessage, error)

// Validator checks an output. A nil return means valid.
type Validator func(json.RawMessage) error

// Mapper runs a Processor with bounded resources and durable item states.
type Mapper struct {
	Concurrency    int
	MaxRetries     int
	MaxInputBytes  int64
	MaxItemBytes   int
	MaxOutputBytes int
	MaxItems       int
	MaxCalls       int
	RunTimeout     time.Duration
	StateDir       string
	ConfigKey      string
	Process        Processor
	Validate       Validator
}

// Output is one ordered result line.
type Output struct {
	Index    int             `json:"index"`
	OK       bool            `json:"ok"`
	Output   json.RawMessage `json:"output,omitempty"`
	Error    string          `json:"error,omitempty"`
	Attempts int             `json:"attempts"`
}

// Result summarizes a run and its maximum buffered/in-progress item count.
type Result struct {
	Total        int
	Succeeded    int
	Failed       int
	PeakInFlight int
}

// ItemStatus is the durable per-item lifecycle shared by llm and agentic map.
type ItemStatus string

const (
	// ItemPending represents an item that has not started processing.
	ItemPending ItemStatus = "pending"
	// ItemRunning represents an item with a durable attempt in progress.
	ItemRunning ItemStatus = "running"
	// ItemCompleted represents an item with validated output.
	ItemCompleted ItemStatus = "completed"
	// ItemFailed represents an item that exhausted its allowed attempts.
	ItemFailed ItemStatus = "failed"
)

// ItemState is persisted atomically after every attempt and terminal result.
type ItemState struct {
	Index     int             `json:"index"`
	InputHash string          `json:"input_hash"`
	Status    ItemStatus      `json:"status"`
	Attempts  int             `json:"attempts"`
	Output    json.RawMessage `json:"output,omitempty"`
	Error     string          `json:"error,omitempty"`
}

type runManifest struct {
	Version        int    `json:"version"`
	InputPath      string `json:"input_path"`
	InputHash      string `json:"input_hash"`
	Items          int    `json:"items"`
	ConfigKey      string `json:"config_key"`
	MaxAttempts    int    `json:"max_attempts"`
	MaxInputBytes  int64  `json:"max_input_bytes"`
	MaxItemBytes   int    `json:"max_item_bytes"`
	MaxOutputBytes int    `json:"max_output_bytes"`
}

type runSettings struct {
	concurrency, maxAttempts, maxItemBytes, maxOutputBytes, maxItems, maxCalls int
	maxInputBytes                                                              int64
	stateDir                                                                   string
}

type datasetInfo struct {
	path, hash string
	items      int
}

type workItem struct {
	index, priorAttempts int
	raw                  json.RawMessage
	hash, feedback       string
}

type stateStore struct{ dir string }

type workerPool struct {
	ctx            context.Context
	cancel         context.CancelFunc
	work           chan workItem
	errors         chan error
	wait           sync.WaitGroup
	inFlight, peak atomic.Int64
}

// Run validates the full input and every hard limit before model work, streams
// items through a bounded worker pool, and atomically assembles ordered output.
func (mapper *Mapper) Run(ctx context.Context, inputPath, outputPath string) (Result, error) {
	settings, err := mapper.settings(outputPath)
	if err != nil {
		return Result{}, err
	}
	if pathErr := distinctPaths(inputPath, outputPath); pathErr != nil {
		return Result{}, pathErr
	}
	dataset, err := preflightInput(inputPath, settings)
	if err != nil {
		return Result{}, err
	}
	if settings.maxCalls > 0 && dataset.items*settings.maxAttempts > settings.maxCalls {
		return Result{}, fmt.Errorf("llmmap: worst-case calls %d exceed --max-calls %d", dataset.items*settings.maxAttempts, settings.maxCalls)
	}
	store, err := openStateStore(settings.stateDir, dataset, mapper.ConfigKey, settings)
	if err != nil {
		return Result{}, err
	}
	snapshotPath, err := prepareInputSnapshot(store, dataset, settings)
	if err != nil {
		return Result{}, err
	}
	dataset.path = snapshotPath
	runCtx, cancel := mapRunContext(ctx, mapper.RunTimeout)
	defer cancel()
	result, err := mapper.processStream(runCtx, dataset, store, settings)
	if err != nil {
		return result, err
	}
	result, err = assembleOutput(outputPath, store, dataset.items, result.PeakInFlight)
	if err != nil {
		return result, err
	}
	if err := os.RemoveAll(store.dir); err != nil {
		return result, fmt.Errorf("llmmap: remove completed state: %w", err)
	}
	return result, nil
}

func (mapper *Mapper) settings(outputPath string) (runSettings, error) {
	settings := runSettings{
		concurrency:    valueOrDefault(mapper.Concurrency, defaultConcurrency),
		maxAttempts:    max(mapper.MaxRetries+1, 1),
		maxInputBytes:  int64ValueOrDefault(mapper.MaxInputBytes, defaultInputBytes),
		maxItemBytes:   valueOrDefault(mapper.MaxItemBytes, defaultItemBytes),
		maxOutputBytes: valueOrDefault(mapper.MaxOutputBytes, defaultOutputBytes),
		maxItems:       valueOrDefault(mapper.MaxItems, defaultMaxItems), maxCalls: mapper.MaxCalls,
		stateDir: mapper.StateDir,
	}
	if settings.stateDir == "" {
		settings.stateDir = outputPath + ".acm-map-state"
	}
	if mapper.Process == nil {
		return settings, errors.New("llmmap: processor is required")
	}
	if outputPath == "" {
		return settings, errors.New("llmmap: output path is required")
	}
	if mapper.MaxRetries < 0 || settings.concurrency < 1 || settings.concurrency > hardConcurrency || settings.maxAttempts > hardAttempts {
		return settings, fmt.Errorf("llmmap: concurrency must be 1..%d and attempts 1..%d", hardConcurrency, hardAttempts)
	}
	if settings.maxItemBytes < 1 || settings.maxItemBytes > hardItemBytes || settings.maxOutputBytes < 1 || settings.maxOutputBytes > hardItemBytes {
		return settings, fmt.Errorf("llmmap: item/output bytes must be 1..%d", hardItemBytes)
	}
	if settings.maxInputBytes < 1 || settings.maxInputBytes > hardInputBytes {
		return settings, fmt.Errorf("llmmap: input bytes must be 1..%d", int64(hardInputBytes))
	}
	if settings.maxItems < 1 || settings.maxItems > hardMaxItems || settings.maxCalls < 0 || settings.maxCalls > hardMaxCalls || mapper.RunTimeout < defaultRunTimeout || mapper.RunTimeout > hardRunTimeout {
		return settings, errors.New("llmmap: invalid item, call, or timeout limit")
	}
	return settings, nil
}

func distinctPaths(inputPath, outputPath string) error {
	input, err := filepath.Abs(inputPath)
	if err != nil {
		return fmt.Errorf("llmmap: resolve input: %w", err)
	}
	output, err := filepath.Abs(outputPath)
	if err != nil {
		return fmt.Errorf("llmmap: resolve output: %w", err)
	}
	if filepath.Clean(input) == filepath.Clean(output) {
		return errors.New("llmmap: input and output must be different files")
	}
	return nil
}

func preflightInput(path string, settings runSettings) (datasetInfo, error) {
	absolute, err := filepath.Abs(path)
	if err != nil {
		return datasetInfo{}, fmt.Errorf("llmmap: resolve input: %w", err)
	}
	file, err := os.Open(absolute)
	if err != nil {
		return datasetInfo{}, fmt.Errorf("llmmap: open input: %w", err)
	}
	defer func() { _ = file.Close() }()
	hash := sha256.New()
	reader := io.TeeReader(file, hash)
	count, err := scanJSONL(reader, settings.maxInputBytes, settings.maxItemBytes, settings.maxItems, nil)
	if err != nil {
		return datasetInfo{}, err
	}
	return datasetInfo{path: absolute, hash: hex.EncodeToString(hash.Sum(nil)), items: count}, nil
}

func prepareInputSnapshot(store stateStore, dataset datasetInfo, settings runSettings) (string, error) {
	path := filepath.Join(store.dir, "input.jsonl")
	existing, err := preflightInput(path, settings)
	if err == nil {
		if existing.hash != dataset.hash || existing.items != dataset.items {
			return "", errors.New("llmmap: resume input snapshot does not match source input")
		}
		return path, nil
	}
	if !errors.Is(err, os.ErrNotExist) {
		return "", err
	}
	if err := writeInputSnapshot(path, dataset, settings); err != nil {
		return "", err
	}
	return path, nil
}

func writeInputSnapshot(path string, dataset datasetInfo, settings runSettings) error {
	source, err := os.Open(dataset.path)
	if err != nil {
		return fmt.Errorf("llmmap: reopen input for snapshot: %w", err)
	}
	defer func() { _ = source.Close() }()
	temporary, err := os.CreateTemp(filepath.Dir(path), ".input-*.tmp")
	if err != nil {
		return fmt.Errorf("llmmap: create input snapshot: %w", err)
	}
	temporaryPath := temporary.Name()
	defer func() { _ = os.Remove(temporaryPath) }()
	hash := sha256.New()
	reader := io.TeeReader(source, io.MultiWriter(temporary, hash))
	items, scanErr := scanJSONL(reader, settings.maxInputBytes, settings.maxItemBytes, settings.maxItems, nil)
	if scanErr != nil {
		_ = temporary.Close()
		return scanErr
	}
	actualHash := hex.EncodeToString(hash.Sum(nil))
	if items != dataset.items || actualHash != dataset.hash {
		_ = temporary.Close()
		return errors.New("llmmap: input changed while creating validated snapshot")
	}
	return finishInputSnapshot(temporary, temporaryPath, path)
}

func finishInputSnapshot(file *os.File, temporaryPath, path string) error {
	if err := file.Sync(); err != nil {
		_ = file.Close()
		return fmt.Errorf("llmmap: sync input snapshot: %w", err)
	}
	if err := file.Chmod(0o600); err != nil {
		_ = file.Close()
		return fmt.Errorf("llmmap: secure input snapshot: %w", err)
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("llmmap: close input snapshot: %w", err)
	}
	if err := os.Rename(temporaryPath, path); err != nil {
		return fmt.Errorf("llmmap: publish input snapshot: %w", err)
	}
	return nil
}

func scanJSONL(reader io.Reader, maxInputBytes int64, maxBytes, maxItems int, consume func(int, json.RawMessage) error) (int, error) {
	limited := &io.LimitedReader{R: reader, N: maxInputBytes + 1}
	scanner := bufio.NewScanner(limited)
	scanner.Buffer(make([]byte, 0, 64*1024), maxBytes+1)
	items, line := 0, 0
	for scanner.Scan() {
		line++
		if line > hardMaxLines {
			return items, fmt.Errorf("llmmap: input exceeds %d lines", hardMaxLines)
		}
		raw := trimSpace(scanner.Bytes())
		if len(raw) == 0 {
			continue
		}
		if len(raw) > maxBytes {
			return items, fmt.Errorf("llmmap: input line %d exceeds %d bytes", line, maxBytes)
		}
		if !json.Valid(raw) {
			return items, fmt.Errorf("llmmap: input line %d is not valid JSON", line)
		}
		if items == maxItems {
			return items, fmt.Errorf("llmmap: input exceeds %d items", maxItems)
		}
		if consume != nil {
			if err := consume(items, append(json.RawMessage(nil), raw...)); err != nil {
				return items, err
			}
		}
		items++
	}
	if err := scanner.Err(); err != nil {
		return items, fmt.Errorf("llmmap: read input: %w", err)
	}
	if limited.N == 0 {
		return items, fmt.Errorf("llmmap: input exceeds %d bytes", maxInputBytes)
	}
	return items, nil
}

func openStateStore(dir string, dataset datasetInfo, configKey string, settings runSettings) (stateStore, error) {
	store := stateStore{dir: dir}
	itemsDir := filepath.Join(dir, "items")
	if err := os.MkdirAll(itemsDir, 0o700); err != nil {
		return store, fmt.Errorf("llmmap: create state directory: %w", err)
	}
	//nolint:gosec // G302: directories require the owner execute bit for traversal.
	if err := os.Chmod(dir, 0o700); err != nil {
		return store, fmt.Errorf("llmmap: secure state directory: %w", err)
	}
	//nolint:gosec // G302: directories require the owner execute bit for traversal.
	if err := os.Chmod(itemsDir, 0o700); err != nil {
		return store, fmt.Errorf("llmmap: secure item state directory: %w", err)
	}
	manifest := runManifest{
		Version: stateVersion, InputPath: dataset.path, InputHash: dataset.hash,
		Items: dataset.items, ConfigKey: configKey, MaxAttempts: settings.maxAttempts,
		MaxInputBytes: settings.maxInputBytes,
		MaxItemBytes:  settings.maxItemBytes, MaxOutputBytes: settings.maxOutputBytes,
	}
	path := filepath.Join(dir, "manifest.json")
	existing, err := readManifest(path)
	if os.IsNotExist(err) {
		return store, writeJSONAtomic(path, manifest)
	}
	if err != nil {
		return store, err
	}
	if existing != manifest {
		return store, errors.New("llmmap: resume state does not match input or processing configuration")
	}
	return store, nil
}

func (mapper *Mapper) processStream(ctx context.Context, dataset datasetInfo, store stateStore, settings runSettings) (Result, error) {
	file, err := os.Open(dataset.path)
	if err != nil {
		return Result{}, fmt.Errorf("llmmap: reopen input: %w", err)
	}
	defer func() { _ = file.Close() }()
	pool := newWorkerPool(ctx, settings.concurrency)
	defer pool.cancel()
	mapper.startWorkers(pool, store, settings)
	_, scanErr := scanJSONL(file, settings.maxInputBytes, settings.maxItemBytes, settings.maxItems, func(index int, raw json.RawMessage) error {
		return pool.enqueue(store, index, raw)
	})
	return pool.finish(ctx, dataset.items, scanErr)
}

func newWorkerPool(parent context.Context, concurrency int) *workerPool {
	ctx, cancel := context.WithCancel(parent)
	return &workerPool{
		ctx: ctx, cancel: cancel, work: make(chan workItem, concurrency),
		errors: make(chan error, 1),
	}
}

func (mapper *Mapper) startWorkers(pool *workerPool, store stateStore, settings runSettings) {
	for range settings.concurrency {
		pool.wait.Go(func() {
			mapper.worker(pool.ctx, store, pool.work, settings, &pool.inFlight, pool.report)
		})
	}
}

func (pool *workerPool) enqueue(store stateStore, index int, raw json.RawMessage) error {
	item, skip, err := resumableWork(store, index, raw)
	if err != nil || skip {
		return err
	}
	current := pool.inFlight.Add(1)
	updatePeak(&pool.peak, current)
	select {
	case pool.work <- item:
		return nil
	case <-pool.ctx.Done():
		pool.inFlight.Add(-1)
		return pool.ctx.Err()
	}
}

func (pool *workerPool) report(err error) {
	select {
	case pool.errors <- err:
		pool.cancel()
	default:
	}
}

func (pool *workerPool) finish(parent context.Context, total int, scanErr error) (Result, error) {
	close(pool.work)
	pool.wait.Wait()
	result := Result{Total: total, PeakInFlight: int(pool.peak.Load())}
	select {
	case workerErr := <-pool.errors:
		return result, workerErr
	default:
	}
	if scanErr != nil {
		return result, scanErr
	}
	if err := parent.Err(); err != nil {
		return result, err
	}
	return result, nil
}

func (mapper *Mapper) worker(ctx context.Context, store stateStore, work <-chan workItem, settings runSettings, inFlight *atomic.Int64, reportError func(error)) {
	for range settings.maxItems {
		item, ok := <-work
		if !ok {
			return
		}
		state, err := mapper.processOne(ctx, store, item, settings)
		if err == nil {
			err = store.save(state)
		}
		inFlight.Add(-1)
		if err != nil {
			reportError(err)
			return
		}
	}
}

func (mapper *Mapper) processOne(ctx context.Context, store stateStore, item workItem, settings runSettings) (ItemState, error) {
	state := ItemState{Index: item.index, InputHash: item.hash, Status: ItemRunning, Attempts: item.priorAttempts, Error: item.feedback}
	for attempt := item.priorAttempts + 1; attempt <= settings.maxAttempts; attempt++ {
		if ctx.Err() != nil {
			return state, nil //nolint:nilerr // the worker persists this resumable nonterminal state
		}
		state.Attempts = attempt
		if err := store.save(state); err != nil {
			return state, err
		}
		output, err := mapper.Process(ctx, item.raw, attempt, state.Error)
		if err == nil {
			err = mapper.validateOutput(output, settings.maxOutputBytes)
		}
		if err == nil {
			state.Status, state.Output, state.Error = ItemCompleted, output, ""
			return state, nil
		}
		state.Error = boundedError(err)
		if err := store.save(state); err != nil {
			return state, err
		}
	}
	state.Status = ItemFailed
	return state, nil
}

func (mapper *Mapper) validateOutput(output json.RawMessage, maxBytes int) error {
	if len(output) > maxBytes {
		return fmt.Errorf("output exceeds %d bytes", maxBytes)
	}
	if !json.Valid(output) {
		return errors.New("output is not valid JSON")
	}
	if mapper.Validate != nil {
		return mapper.Validate(output)
	}
	return nil
}

func resumableWork(store stateStore, index int, raw json.RawMessage) (workItem, bool, error) {
	hash := fmt.Sprintf("%x", sha256.Sum256(raw))
	state, err := store.load(index)
	if os.IsNotExist(err) {
		return workItem{index: index, raw: raw, hash: hash}, false, nil
	}
	if err != nil {
		return workItem{}, false, err
	}
	if state.InputHash != hash {
		return workItem{}, false, fmt.Errorf("llmmap: item %d does not match resume state", index)
	}
	if state.Status == ItemCompleted || state.Status == ItemFailed {
		return workItem{}, true, nil
	}
	return workItem{index: index, raw: raw, hash: hash, priorAttempts: state.Attempts, feedback: state.Error}, false, nil
}

func assembleOutput(path string, store stateStore, count, peak int) (Result, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return Result{}, fmt.Errorf("llmmap: create output directory: %w", err)
	}
	temporary, err := os.CreateTemp(filepath.Dir(path), ".acm-map-*.tmp")
	if err != nil {
		return Result{}, fmt.Errorf("llmmap: create temporary output: %w", err)
	}
	temporaryPath := temporary.Name()
	defer func() { _ = os.Remove(temporaryPath) }()
	result := Result{Total: count, PeakInFlight: peak}
	writer := bufio.NewWriter(temporary)
	encoder := json.NewEncoder(writer)
	for index := range count {
		state, sErr := store.load(index)
		if sErr != nil || (state.Status != ItemCompleted && state.Status != ItemFailed) {
			_ = temporary.Close()
			return result, fmt.Errorf("llmmap: item %d has no terminal state", index)
		}
		output := Output{Index: index, OK: state.Status == ItemCompleted, Output: state.Output, Error: state.Error, Attempts: state.Attempts}
		if output.OK {
			result.Succeeded++
		} else {
			result.Failed++
		}
		if err := encoder.Encode(output); err != nil {
			_ = temporary.Close()
			return result, fmt.Errorf("llmmap: encode output: %w", err)
		}
	}
	if err := finishAtomicOutput(temporary, writer, temporaryPath, path); err != nil {
		return result, err
	}
	return result, nil
}

func finishAtomicOutput(file *os.File, writer *bufio.Writer, temporaryPath, outputPath string) error {
	if err := writer.Flush(); err != nil {
		_ = file.Close()
		return fmt.Errorf("llmmap: flush output: %w", err)
	}
	if err := file.Sync(); err != nil {
		_ = file.Close()
		return fmt.Errorf("llmmap: sync output: %w", err)
	}
	if err := file.Chmod(0o600); err != nil {
		_ = file.Close()
		return fmt.Errorf("llmmap: secure output: %w", err)
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("llmmap: close output: %w", err)
	}
	if err := os.Rename(temporaryPath, outputPath); err != nil {
		return fmt.Errorf("llmmap: publish output: %w", err)
	}
	return nil
}

func (store stateStore) path(index int) string {
	return filepath.Join(store.dir, "items", fmt.Sprintf("%012d.json", index))
}

func (store stateStore) load(index int) (ItemState, error) {
	var state ItemState
	content, err := readFileBounded(store.path(index), maxStateBytes)
	if err != nil {
		return state, err
	}
	if err := json.Unmarshal(content, &state); err != nil {
		return state, fmt.Errorf("llmmap: decode item state %d: %w", index, err)
	}
	return state, nil
}

func (store stateStore) save(state ItemState) error {
	return writeJSONAtomic(store.path(state.Index), state)
}

func readManifest(path string) (runManifest, error) {
	var manifest runManifest
	content, err := readFileBounded(path, maxManifestBytes)
	if err != nil {
		return manifest, err
	}
	if err := json.Unmarshal(content, &manifest); err != nil {
		return manifest, fmt.Errorf("llmmap: decode manifest: %w", err)
	}
	return manifest, nil
}

func readFileBounded(path string, limit int64) ([]byte, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = file.Close() }()
	content, err := io.ReadAll(io.LimitReader(file, limit+1))
	if err != nil {
		return nil, fmt.Errorf("llmmap: read state: %w", err)
	}
	if int64(len(content)) > limit {
		return nil, fmt.Errorf("llmmap: file %s exceeds %d bytes", path, limit)
	}
	return content, nil
}

func writeJSONAtomic(path string, value any) error {
	content, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("llmmap: encode state: %w", err)
	}
	temporary, err := os.CreateTemp(filepath.Dir(path), ".state-*.tmp")
	if err != nil {
		return fmt.Errorf("llmmap: create state: %w", err)
	}
	temporaryPath := temporary.Name()
	defer func() { _ = os.Remove(temporaryPath) }()
	if err := temporary.Chmod(0o600); err != nil {
		_ = temporary.Close()
		return fmt.Errorf("llmmap: secure state: %w", err)
	}
	if _, err := temporary.Write(content); err != nil {
		_ = temporary.Close()
		return fmt.Errorf("llmmap: write state: %w", err)
	}
	if err := temporary.Sync(); err != nil {
		_ = temporary.Close()
		return fmt.Errorf("llmmap: sync state: %w", err)
	}
	if err := temporary.Close(); err != nil {
		return fmt.Errorf("llmmap: close state: %w", err)
	}
	if err := os.Rename(temporaryPath, path); err != nil {
		return fmt.Errorf("llmmap: publish state: %w", err)
	}
	return nil
}

func updatePeak(peak *atomic.Int64, current int64) {
	for observed := peak.Load(); current > observed; observed = peak.Load() {
		if peak.CompareAndSwap(observed, current) {
			return
		}
	}
}

func mapRunContext(parent context.Context, timeout time.Duration) (context.Context, context.CancelFunc) {
	if timeout > 0 {
		return context.WithTimeout(parent, timeout)
	}
	return context.WithCancel(parent)
}

func boundedError(err error) string {
	message := err.Error()
	if len(message) > maxErrorBytes {
		return message[:maxErrorBytes]
	}
	return message
}

func valueOrDefault(value, fallback int) int {
	if value == 0 {
		return fallback
	}
	return value
}

func int64ValueOrDefault(value, fallback int64) int64 {
	if value == 0 {
		return fallback
	}
	return value
}

// RequireFields checks that output is a JSON object containing every key.
func RequireFields(fields ...string) Validator {
	return func(out json.RawMessage) error {
		var object map[string]json.RawMessage
		if err := json.Unmarshal(out, &object); err != nil {
			return fmt.Errorf("output is not a JSON object: %w", err)
		}
		for _, field := range fields {
			if _, ok := object[field]; !ok {
				return fmt.Errorf("missing required field %q", field)
			}
		}
		return nil
	}
}

func trimSpace(value []byte) []byte {
	start, end := 0, len(value)
	for start < end && isSpace(value[start]) {
		start++
	}
	for end > start && isSpace(value[end-1]) {
		end--
	}
	return value[start:end]
}

func isSpace(value byte) bool {
	return value == ' ' || value == '\t' || value == '\r' || value == '\n'
}
