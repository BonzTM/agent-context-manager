package summarize

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"time"
)

const (
	maxStreamingEvents    = 10_000
	maxStreamingLineBytes = 8 << 20
)

// EventHandler consumes one JSON event line from a headless agent process.
type EventHandler func([]byte) error

// ExecJSONLines runs a headless agent and streams a bounded number of bounded
// JSONL events to handler. Cancellation terminates the process group on Unix.
func ExecJSONLines(ctx context.Context, argv []string, stdin string, timeout time.Duration, maxEvents, maxLineBytes int, handler EventHandler) error {
	if err := validateStreamRequest(argv, timeout, maxEvents, maxLineBytes, handler); err != nil {
		return err
	}
	callCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	//nolint:gosec // G204: argv uses fixed host-CLI templates; prompts are stdin data.
	command := exec.CommandContext(callCtx, argv[0], argv[1:]...)
	configureCommand(command)
	command.WaitDelay = time.Second
	command.Stdin = strings.NewReader(stdin)
	stdout, err := command.StdoutPipe()
	if err != nil {
		return fmt.Errorf("summarize: open %s stdout: %w", argv[0], err)
	}
	stderr := &boundedBuffer{limit: maxLineBytes}
	command.Stderr = stderr
	if err := command.Start(); err != nil {
		return fmt.Errorf("summarize: start %s: %w", argv[0], err)
	}
	scanErr := consumeEvents(stdout, maxEvents, maxLineBytes, handler)
	if scanErr != nil {
		cancel()
	}
	waitErr := command.Wait()
	return streamRunError(callCtx, argv[0], scanErr, waitErr, stderr.String())
}

func validateStreamRequest(argv []string, timeout time.Duration, maxEvents, maxLineBytes int, handler EventHandler) error {
	if len(argv) == 0 || handler == nil {
		return errors.New("summarize: streaming command and handler are required")
	}
	if timeout <= 0 || maxEvents < 1 || maxEvents > maxStreamingEvents || maxLineBytes < 1 || maxLineBytes > maxStreamingLineBytes {
		return errors.New("summarize: invalid streaming execution limits")
	}
	return nil
}

func consumeEvents(stdout io.Reader, maxEvents, maxLineBytes int, handler EventHandler) error {
	scanner := bufio.NewScanner(stdout)
	scanner.Buffer(make([]byte, 0, min(64*1024, maxLineBytes)), maxLineBytes)
	for range maxEvents {
		if !scanner.Scan() {
			return scannerError(scanner)
		}
		if err := handler(scanner.Bytes()); err != nil {
			return err
		}
	}
	if scanner.Scan() {
		return fmt.Errorf("summarize: agent emitted more than %d events", maxEvents)
	}
	return scannerError(scanner)
}

func scannerError(scanner *bufio.Scanner) error {
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("summarize: read agent events: %w", err)
	}
	return nil
}

func streamRunError(ctx context.Context, binary string, scanErr, waitErr error, stderr string) error {
	if scanErr != nil {
		return scanErr
	}
	if ctx.Err() != nil {
		return fmt.Errorf("summarize: exec %s: %w", binary, ctx.Err())
	}
	if waitErr != nil {
		return fmt.Errorf("summarize: exec %s: %w: %s", binary, waitErr, strings.TrimSpace(stderr))
	}
	return nil
}

type boundedBuffer struct {
	buffer bytes.Buffer
	limit  int
}

func (buffer *boundedBuffer) Write(content []byte) (int, error) {
	written := len(content)
	remaining := buffer.limit - buffer.buffer.Len()
	if remaining > 0 {
		_, _ = buffer.buffer.Write(content[:min(remaining, len(content))])
	}
	return written, nil
}

func (buffer *boundedBuffer) String() string { return buffer.buffer.String() }
