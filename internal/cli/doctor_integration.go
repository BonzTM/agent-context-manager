package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/BurntSushi/toml"

	"github.com/bonztm/agent-context-manager/internal/core"
	"github.com/bonztm/agent-context-manager/internal/store"
)

const (
	maxIntegrationConfigBytes = 1 << 20
	maxDoctorConversations    = 1_000
	maxReportedRoleGaps       = 20
)

type integrationReport struct {
	CodexDetected bool
	Executable    string
	LatestCapture time.Time
	RoleGaps      []string
	RoleGapCount  int
	Findings      []string
}

func checkIntegrations(ctx context.Context, sq *store.SQLite, dbPath, home string) (integrationReport, error) {
	report, err := checkCodexFiles(home)
	if err != nil {
		return report, err
	}
	report.Findings = append(report.Findings, permissionFindings(dbPath)...)
	counts, err := sq.ConversationRoleCounts(ctx, core.AgentCodex, maxDoctorConversations)
	if err != nil {
		return report, err
	}
	for _, count := range counts {
		if count.UpdatedAt.After(report.LatestCapture) {
			report.LatestCapture = count.UpdatedAt
		}
		if count.Users > 0 && count.Assistants == 0 {
			report.RoleGapCount++
			if len(report.RoleGaps) < maxReportedRoleGaps {
				report.RoleGaps = append(report.RoleGaps, fmt.Sprintf("%s (user=%d tool=%d assistant=0)", count.ConversationID, count.Users, count.Tools))
			}
		}
	}
	if report.RoleGapCount > 0 {
		report.Findings = append(report.Findings, fmt.Sprintf("%d Codex conversation(s) have prompts but no assistant capture", report.RoleGapCount))
	}
	return report, nil
}

func checkCodexFiles(home string) (integrationReport, error) {
	codexDir := filepath.Join(home, ".codex")
	if _, err := os.Stat(codexDir); os.IsNotExist(err) {
		return integrationReport{}, nil
	} else if err != nil {
		return integrationReport{}, fmt.Errorf("doctor: inspect Codex directory: %w", err)
	}
	report := integrationReport{CodexDetected: true}
	notify, findings, err := checkCodexNotify(filepath.Join(codexDir, "config.toml"))
	if err != nil {
		return report, err
	}
	report.Findings = append(report.Findings, findings...)
	if len(notify) > 0 {
		report.Executable, err = exec.LookPath(notify[0])
		if err != nil {
			report.Findings = append(report.Findings, fmt.Sprintf("Codex notify executable %q is not resolvable", notify[0]))
		}
	}
	hookFindings, err := checkCodexHooks(filepath.Join(codexDir, "hooks.json"))
	report.Findings = append(report.Findings, hookFindings...)
	return report, err
}

func checkCodexNotify(path string) ([]string, []string, error) {
	content, err := readBoundedConfig(path)
	if os.IsNotExist(err) {
		return nil, []string{"Codex config.toml is missing the top-level ACM notify command"}, nil
	}
	if err != nil {
		return nil, nil, err
	}
	var config struct {
		Notify []string `toml:"notify"`
	}
	if _, err = toml.Decode(string(content), &config); err != nil {
		return nil, nil, fmt.Errorf("doctor: parse Codex config: %w", err)
	}
	if !isACMNotify(config.Notify) {
		return config.Notify, []string{"Codex config.toml is missing the top-level ACM notify command"}, nil
	}
	return config.Notify, nil, nil
}

func isACMNotify(command []string) bool {
	want := []string{"hook", "--agent", "codex", "--event", "agent-turn-complete"}
	if len(command) != len(want)+1 || filepath.Base(command[0]) != "acm" {
		return false
	}
	return strings.Join(command[1:], "\x00") == strings.Join(want, "\x00")
}

func checkCodexHooks(path string) ([]string, error) {
	content, err := readBoundedConfig(path)
	if os.IsNotExist(err) {
		return []string{"Codex hooks.json is missing ACM hooks"}, nil
	}
	if err != nil {
		return nil, err
	}
	var config struct {
		Hooks map[string][]struct {
			Hooks []struct {
				Command string `json:"command"`
			} `json:"hooks"`
		} `json:"hooks"`
	}
	if err = json.Unmarshal(content, &config); err != nil {
		return nil, fmt.Errorf("doctor: parse Codex hooks: %w", err)
	}
	var findings []string
	for _, event := range []string{"UserPromptSubmit", "PostToolUse", "Stop"} {
		if !hasHookCommand(config.Hooks[event], "acm hook --agent codex --event "+event) {
			findings = append(findings, "Codex "+event+" ACM hook is missing")
		}
	}
	return findings, nil
}

func hasHookCommand(entries []struct {
	Hooks []struct {
		Command string `json:"command"`
	} `json:"hooks"`
}, command string,
) bool {
	for _, entry := range entries {
		for _, hook := range entry.Hooks {
			if hook.Command == command {
				return true
			}
		}
	}
	return false
}

func readBoundedConfig(path string) ([]byte, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = file.Close() }()
	content, err := io.ReadAll(io.LimitReader(file, maxIntegrationConfigBytes+1))
	if err != nil {
		return nil, fmt.Errorf("doctor: read %s: %w", path, err)
	}
	if len(content) > maxIntegrationConfigBytes {
		return nil, fmt.Errorf("doctor: config %s exceeds %d bytes", path, maxIntegrationConfigBytes)
	}
	return content, nil
}

func permissionFindings(dbPath string) []string {
	var findings []string
	for _, path := range []string{dbPath, dbPath + "-wal", dbPath + "-shm"} {
		info, err := os.Stat(path)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			findings = append(findings, fmt.Sprintf("cannot inspect permissions for %s: %v", path, err))
			continue
		}
		if info.Mode().Perm() != 0o600 {
			findings = append(findings, fmt.Sprintf("%s mode is %04o, want 0600", path, info.Mode().Perm()))
		}
	}
	return findings
}
