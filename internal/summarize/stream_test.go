package summarize

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestExecJSONLinesStreamsEvents(t *testing.T) {
	var events []string
	err := ExecJSONLines(context.Background(), []string{"sh", "-c", `printf '%s\n' '{"n":1}' '{"n":2}'`}, "", time.Second, 2, 1024, func(line []byte) error {
		events = append(events, string(line))
		return nil
	})
	if err != nil {
		t.Fatalf("stream: %v", err)
	}
	if len(events) != 2 {
		t.Fatalf("events = %v", events)
	}
}

func TestExecJSONLinesEnforcesEventAndLineBounds(t *testing.T) {
	eventErr := ExecJSONLines(context.Background(), []string{"sh", "-c", `printf '%s\n' '{}' '{}'`}, "", time.Second, 1, 1024, func([]byte) error { return nil })
	if eventErr == nil || !strings.Contains(eventErr.Error(), "more than 1 events") {
		t.Fatalf("event limit error = %v", eventErr)
	}
	lineErr := ExecJSONLines(context.Background(), []string{"sh", "-c", `printf '%0100d\n' 1`}, "", time.Second, 2, 16, func([]byte) error { return nil })
	if lineErr == nil || !strings.Contains(lineErr.Error(), "token too long") {
		t.Fatalf("line limit error = %v", lineErr)
	}
}

func TestExecJSONLinesCancelsProcessOnHandlerFailure(t *testing.T) {
	want := errors.New("stop events")
	started := time.Now()
	err := ExecJSONLines(context.Background(), []string{"sh", "-c", `printf '%s\n' '{}'; sleep 10`}, "", 5*time.Second, 2, 1024, func([]byte) error {
		return want
	})
	if !errors.Is(err, want) {
		t.Fatalf("handler error = %v", err)
	}
	if elapsed := time.Since(started); elapsed > 2*time.Second {
		t.Fatalf("process cancellation took %s", elapsed)
	}
}
