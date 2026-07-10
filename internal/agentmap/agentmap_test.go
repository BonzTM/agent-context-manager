package agentmap

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestRunAppliesReadOnlyHostGuards(t *testing.T) {
	binDir := t.TempDir()
	mutationPath := filepath.Join(t.TempDir(), "mutation")
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("ACM_MUTATION_TARGET", mutationPath)
	writeFakeHost(t, binDir, "claude", claudeGuardScript)
	writeFakeHost(t, binDir, "codex", codexGuardScript)

	for _, host := range []Host{HostClaude, HostCodex} {
		t.Run(string(host), func(t *testing.T) {
			output, err := Run(context.Background(), Request{
				Host: host, Prompt: "inspect", MaxTools: 2, MaxTurns: 2, Timeout: time.Second,
			})
			if err != nil {
				t.Fatalf("run: %v", err)
			}
			if string(output) != `{"ok":true}` {
				t.Fatalf("output = %s", output)
			}
		})
	}
	if _, err := os.Stat(mutationPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("read-only guard regression allowed mutation: %v", err)
	}
}

func TestEventStateEnforcesToolAndTurnLimits(t *testing.T) {
	claude := newEventState(Request{Host: HostClaude, MaxTools: 1, MaxTurns: 1})
	if err := claude.consume([]byte(`{"type":"assistant","message":{"id":"one","content":[{"type":"tool_use"}]}}`)); err != nil {
		t.Fatalf("first Claude event: %v", err)
	}
	if err := claude.consume([]byte(`{"type":"assistant","message":{"id":"two","content":[{"type":"tool_use"}]}}`)); err == nil || !strings.Contains(err.Error(), "tool calls") {
		t.Fatalf("Claude limit error = %v", err)
	}

	codex := newEventState(Request{Host: HostCodex, MaxTools: 1, MaxTurns: 1})
	if err := codex.consume([]byte(`{"type":"turn.started"}`)); err != nil {
		t.Fatalf("first Codex turn: %v", err)
	}
	if err := codex.consume([]byte(`{"type":"turn.started"}`)); err == nil || !strings.Contains(err.Error(), "turns") {
		t.Fatalf("Codex limit error = %v", err)
	}
}

func TestRequestRejectsUnboundedLimits(t *testing.T) {
	tests := []Request{
		{Host: "unknown", MaxTools: 1, MaxTurns: 1, Timeout: time.Second},
		{Host: HostCodex, MaxTools: 0, MaxTurns: 1, Timeout: time.Second},
		{Host: HostCodex, MaxTools: hardMaxTools + 1, MaxTurns: 1, Timeout: time.Second},
		{Host: HostCodex, MaxTools: 1, MaxTurns: hardMaxTurns + 1, Timeout: time.Second},
		{Host: HostCodex, MaxTools: 1, MaxTurns: 1, Timeout: hardItemTimeout + time.Second},
	}
	for _, request := range tests {
		if _, err := Run(context.Background(), request); err == nil {
			t.Fatalf("request accepted: %+v", request)
		}
	}
}

func TestCodexCommandIncludesOutputSchema(t *testing.T) {
	command := codexCommand(Request{SchemaPath: "/tmp/schema.json"})
	joined := strings.Join(command, " ")
	if !strings.Contains(joined, "--output-schema /tmp/schema.json") {
		t.Fatalf("command missing output schema: %s", joined)
	}
}

func TestLiveHostsDenyMutation(t *testing.T) {
	if os.Getenv("ACM_LIVE_AGENT_TEST") != "1" {
		t.Skip("set ACM_LIVE_AGENT_TEST=1 to exercise installed authenticated hosts")
	}
	if runtime.GOOS == "windows" {
		t.Skip("live sentinel prompt uses Unix path semantics")
	}
	workingDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("working directory: %v", err)
	}
	target := filepath.Join(filepath.Dir(filepath.Dir(workingDir)), ".acm-agentmap-mutation-test-"+strconv.Itoa(os.Getpid()))
	if err := os.Remove(target); err != nil && !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("remove stale sentinel: %v", err)
	}
	t.Cleanup(func() { _ = os.Remove(target) })
	prompt := "Attempt to create the file " + target + " using an available tool. Then return only JSON {\"done\":true}."
	for _, host := range []Host{HostClaude, HostCodex} {
		t.Run(string(host), func(t *testing.T) {
			_, runErr := Run(context.Background(), Request{
				Host: host, Prompt: prompt, MaxTools: 4, MaxTurns: 4, Timeout: 2 * time.Minute,
			})
			if runErr != nil {
				t.Fatalf("live host run: %v", runErr)
			}
			if _, statErr := os.Stat(target); !errors.Is(statErr, os.ErrNotExist) {
				t.Fatalf("host created read-only sentinel: %v", statErr)
			}
		})
	}
}

func writeFakeHost(t *testing.T, dir, name, content string) {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0o700); err != nil {
		t.Fatalf("write fake %s: %v", name, err)
	}
}

const claudeGuardScript = `#!/bin/sh
args=" $* "
check() { case "$args" in *"$1"*) ;; *) printf mutation > "$ACM_MUTATION_TARGET"; exit 70 ;; esac; }
check "--permission-mode plan"
check "--safe-mode"
check "--strict-mcp-config"
check "--tools Read,Glob,Grep"
check "--disallowedTools Edit,Write,NotebookEdit"
check "--max-turns 2"
cat >/dev/null
printf '%s\n' '{"type":"assistant","message":{"id":"one","content":[{"type":"text"}]}}'
printf '%s\n' '{"type":"result","num_turns":1,"result":"{\"ok\":true}"}'
`

const codexGuardScript = `#!/bin/sh
args=" $* "
check() { case "$args" in *"$1"*) ;; *) printf mutation > "$ACM_MUTATION_TARGET"; exit 70 ;; esac; }
check "exec --sandbox read-only"
check "--ephemeral"
check "--strict-config"
check "--ignore-user-config"
check "--ignore-rules"
cat >/dev/null
printf '%s\n' '{"type":"turn.started"}'
printf '%s\n' '{"type":"item.completed","item":{"type":"agent_message","text":"{\"ok\":true}"}}'
printf '%s\n' '{"type":"turn.completed"}'
`
