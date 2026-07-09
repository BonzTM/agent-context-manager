package cli

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCheckCodexFilesHealthy(t *testing.T) {
	home := t.TempDir()
	installHealthyCodexFixture(t, home)

	report, err := checkCodexFiles(home)
	if err != nil {
		t.Fatalf("check Codex files: %v", err)
	}
	if !report.CodexDetected || len(report.Findings) != 0 || report.Executable == "" {
		t.Fatalf("unexpected report: %+v", report)
	}
}

func TestCheckCodexFilesRejectsNestedNotify(t *testing.T) {
	home := t.TempDir()
	installHealthyCodexFixture(t, home)
	config := "[hooks.state]\nnotify = [\"acm\", \"hook\", \"--agent\", \"codex\", \"--event\", \"agent-turn-complete\"]\n"
	if err := os.WriteFile(filepath.Join(home, ".codex", "config.toml"), []byte(config), 0o600); err != nil {
		t.Fatal(err)
	}

	report, err := checkCodexFiles(home)
	if err != nil {
		t.Fatalf("check Codex files: %v", err)
	}
	if !containsFinding(report.Findings, "top-level ACM notify") {
		t.Fatalf("nested notify findings = %+v", report.Findings)
	}
}

func TestDoctorReturnsErrorForNestedNotify(t *testing.T) {
	home := t.TempDir()
	installHealthyCodexFixture(t, home)
	t.Setenv("HOME", home)
	config := "[hooks.state]\nnotify = [\"acm\", \"hook\", \"--agent\", \"codex\", \"--event\", \"agent-turn-complete\"]\n"
	if err := os.WriteFile(filepath.Join(home, ".codex", "config.toml"), []byte(config), 0o600); err != nil {
		t.Fatal(err)
	}
	root := newRootCmd()
	var output bytes.Buffer
	root.SetOut(&output)
	root.SetErr(&output)
	root.SetArgs([]string{"--db", filepath.Join(t.TempDir(), "acm.db"), "doctor"})
	if err := root.ExecuteContext(context.Background()); err == nil {
		t.Fatal("doctor accepted nested Codex notify")
	}
	if !strings.Contains(output.String(), "PROBLEMS FOUND") {
		t.Fatalf("doctor output = %q", output.String())
	}
}

func TestDoctorDetectsCodexAssistantRoleGap(t *testing.T) {
	home := t.TempDir()
	installHealthyCodexFixture(t, home)
	t.Setenv("HOME", home)
	dbPath := filepath.Join(t.TempDir(), "acm.db")
	payload := `{"agent":"codex","session_id":"gap","messages":[{"role":"user","content":"uncaptured answer","external_id":"turn-1:input:0"}]}`
	runACM(t, dbPath, payload, "ingest")

	root := newRootCmd()
	var output bytes.Buffer
	root.SetOut(&output)
	root.SetErr(&output)
	root.SetArgs([]string{"--db", dbPath, "doctor"})
	if err := root.ExecuteContext(context.Background()); err == nil {
		t.Fatal("doctor accepted a Codex conversation with no assistant rows")
	}
	if !strings.Contains(output.String(), "assistant=0") {
		t.Fatalf("doctor output = %q", output.String())
	}
}

func installHealthyCodexFixture(t *testing.T, home string) {
	t.Helper()
	codexDir := filepath.Join(home, ".codex")
	binDir := filepath.Join(home, "bin")
	if err := os.MkdirAll(codexDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(binDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(binDir, "acm"), []byte("#!/bin/sh\nexit 0\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))
	config := "notify = [\"acm\", \"hook\", \"--agent\", \"codex\", \"--event\", \"agent-turn-complete\"]\n"
	if err := os.WriteFile(filepath.Join(codexDir, "config.toml"), []byte(config), 0o600); err != nil {
		t.Fatal(err)
	}
	hooks := `{"hooks":{"UserPromptSubmit":[{"hooks":[{"command":"acm hook --agent codex --event UserPromptSubmit"}]}],"PostToolUse":[{"hooks":[{"command":"acm hook --agent codex --event PostToolUse"}]}],"Stop":[{"hooks":[{"command":"acm hook --agent codex --event Stop"}]}]}}`
	if err := os.WriteFile(filepath.Join(codexDir, "hooks.json"), []byte(hooks), 0o600); err != nil {
		t.Fatal(err)
	}
}

func containsFinding(findings []string, want string) bool {
	for _, finding := range findings {
		if strings.Contains(finding, want) {
			return true
		}
	}
	return false
}
