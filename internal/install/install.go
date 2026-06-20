// Package install performs global, idempotent, non-destructive installation of
// acm's hooks and drill-down instructions into a host agent's user-level
// configuration. A single install applies to every project, because acm
// resolves the per-project database from the working directory at hook time.
//
// Edits are safe by construction: JSON configs are parsed and merged (existing
// keys are preserved; acm's entries are added only if absent), TOML is only
// appended to (never rewritten), and instruction blocks are marker-guarded so
// re-running updates in place. acm never overwrites a config it cannot parse.
package install

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/bonztm/agent-context-manager/internal/agents"
	"github.com/bonztm/agent-context-manager/internal/core"
)

// Change describes one configuration file edit (real or, in a dry run, planned).
type Change struct {
	Path    string
	Summary string
	Applied bool
	Skipped bool
}

// Result is the outcome of an install run.
type Result struct {
	Agent   core.Agent
	Apply   bool
	Changes []Change
	Notes   []string
}

// Run installs acm globally for an agent under home. When apply is false it is a
// dry run: it reports what would change without writing anything.
func Run(agent core.Agent, home string, apply bool) (Result, error) {
	res := Result{Agent: agent, Apply: apply}
	switch agent {
	case core.AgentClaude:
		if err := res.add(claudeSettings(home, apply)); err != nil {
			return Result{}, err
		}
		if err := res.add(instructions(filepath.Join(home, ".claude", "CLAUDE.md"), apply)); err != nil {
			return Result{}, err
		}
	case core.AgentCodex:
		if err := res.add(codexNotify(home, apply)); err != nil {
			return Result{}, err
		}
		if err := res.add(instructions(filepath.Join(home, ".codex", "AGENTS.md"), apply)); err != nil {
			return Result{}, err
		}
		res.Notes = append(res.Notes,
			"Codex per-prompt recall: wire 'acm hook --agent codex --event UserPromptSubmit' into your Codex hooks config (format varies by version; see docs/integrations.md).")
	case core.AgentOpenCode:
		if err := res.add(opencodeConfig(home, apply)); err != nil {
			return Result{}, err
		}
		if err := res.add(instructions(filepath.Join(home, ".config", "opencode", "AGENTS.md"), apply)); err != nil {
			return Result{}, err
		}
		res.Notes = append(res.Notes,
			"Install the acm-opencode plugin so OpenCode can load it (npm package 'acm-opencode', or link plugins/opencode-acm). Ensure the 'acm' binary is on PATH.")
	default:
		return Result{}, fmt.Errorf("install: unknown agent %q (want claude-code|codex|opencode)", agent)
	}
	return res, nil
}

func (r *Result) add(c Change, err error) error {
	if err != nil {
		return err
	}
	r.Changes = append(r.Changes, c)
	return nil
}

// --- Claude Code: ~/.claude/settings.json ---

func claudeSettings(home string, apply bool) (Change, error) {
	path := filepath.Join(home, ".claude", "settings.json")
	m, err := readJSON(path)
	if err != nil {
		return Change{}, err
	}
	changed := ensureAllow(m, "Bash(acm:*)")
	changed = ensureClaudeHook(m, "UserPromptSubmit", "", "acm hook --agent claude-code --event UserPromptSubmit") || changed
	changed = ensureClaudeHook(m, "PostToolUse", "*", "acm hook --agent claude-code --event PostToolUse") || changed
	return finishJSON(path, m, changed, apply, "hooks + Bash(acm:*) permission")
}

func ensureAllow(m map[string]any, perm string) bool {
	perms := childMap(m, "permissions")
	allow := anySlice(perms["allow"])
	if hasString(allow, perm) {
		perms["allow"] = allow
		return false
	}
	perms["allow"] = append(allow, perm)
	return true
}

func ensureClaudeHook(m map[string]any, event, matcher, command string) bool {
	hooks := childMap(m, "hooks")
	entries := anySlice(hooks[event])
	for _, e := range entries {
		em, ok := e.(map[string]any)
		if !ok {
			continue
		}
		for _, h := range anySlice(em["hooks"]) {
			hm, ok := h.(map[string]any)
			if !ok {
				continue
			}
			if cmd, ok := hm["command"].(string); ok && strings.Contains(cmd, command) {
				return false
			}
		}
	}
	entry := map[string]any{"hooks": []any{map[string]any{"type": "command", "command": command}}}
	if matcher != "" {
		entry["matcher"] = matcher
	}
	hooks[event] = append(entries, entry)
	return true
}

// --- OpenCode: ~/.config/opencode/opencode.json ---

func opencodeConfig(home string, apply bool) (Change, error) {
	path := filepath.Join(home, ".config", "opencode", "opencode.json")
	m, err := readJSON(path)
	if err != nil {
		return Change{}, err
	}
	plugins := anySlice(m["plugin"])
	changed := false
	if !hasString(plugins, "acm-opencode") {
		m["plugin"] = append(plugins, "acm-opencode")
		changed = true
	}
	if _, ok := m["$schema"]; !ok {
		m["$schema"] = "https://opencode.ai/config.json"
		changed = true
	}
	return finishJSON(path, m, changed, apply, "acm-opencode plugin")
}

// --- Codex: ~/.codex/config.toml (append-only notify) ---

var notifyRe = regexp.MustCompile(`(?m)^[ \t]*notify[ \t]*=`)

func codexNotify(home string, apply bool) (Change, error) {
	path := filepath.Join(home, ".codex", "config.toml")
	text, err := readText(path)
	if err != nil {
		return Change{}, err
	}
	if strings.Contains(text, "agent-turn-complete") && strings.Contains(text, "acm hook") {
		return Change{Path: path, Summary: "notify already configured", Skipped: true}, nil
	}
	if notifyRe.MatchString(text) {
		return Change{Path: path, Summary: "existing notify left unchanged (point it at: acm hook --agent codex --event agent-turn-complete)", Skipped: true}, nil
	}
	block := "\n# acm: capture each turn's final assistant message (global)\n" +
		`notify = ["acm", "hook", "--agent", "codex", "--event", "agent-turn-complete"]` + "\n"
	if apply {
		if wErr := writeText(path, text+block); wErr != nil {
			return Change{}, wErr
		}
	}
	return Change{Path: path, Summary: verb(apply) + " notify (assistant-turn capture)", Applied: apply}, nil
}

// --- Drill-down instructions (marker-guarded markdown append) ---

const (
	markerStart = "<!-- acm:start -->"
	markerEnd   = "<!-- acm:end -->"
)

func instructions(path string, apply bool) (Change, error) {
	text, err := readText(path)
	if err != nil {
		return Change{}, err
	}
	newText, changed := ensureInstructions(text, agents.DrillDownDoc)
	if !changed {
		return Change{Path: path, Summary: "drill-down instructions present", Skipped: true}, nil
	}
	if apply {
		if wErr := writeText(path, newText); wErr != nil {
			return Change{}, wErr
		}
	}
	return Change{Path: path, Summary: verb(apply) + " drill-down instructions", Applied: apply}, nil
}

func ensureInstructions(text, block string) (string, bool) {
	wrapped := markerStart + "\n" + strings.TrimRight(block, "\n") + "\n" + markerEnd
	start := strings.Index(text, markerStart)
	end := strings.Index(text, markerEnd)
	if start >= 0 && end > start {
		updated := text[:start] + wrapped + text[end+len(markerEnd):]
		return updated, updated != text
	}
	if strings.TrimSpace(text) == "" {
		return wrapped + "\n", true
	}
	return strings.TrimRight(text, "\n") + "\n\n" + wrapped + "\n", true
}

// --- shared helpers ---

func finishJSON(path string, m map[string]any, changed, apply bool, what string) (Change, error) {
	if !changed {
		return Change{Path: path, Summary: "already configured (" + what + ")", Skipped: true}, nil
	}
	if apply {
		if err := writeJSON(path, m); err != nil {
			return Change{}, err
		}
	}
	return Change{Path: path, Summary: verb(apply) + " " + what, Applied: apply}, nil
}

func verb(apply bool) string {
	if apply {
		return "installed"
	}
	return "would install"
}

func childMap(m map[string]any, key string) map[string]any {
	if c, ok := m[key].(map[string]any); ok {
		return c
	}
	c := map[string]any{}
	m[key] = c
	return c
}

func anySlice(v any) []any {
	if s, ok := v.([]any); ok {
		return s
	}
	return nil
}

func hasString(s []any, want string) bool {
	for _, e := range s {
		if v, ok := e.(string); ok && v == want {
			return true
		}
	}
	return false
}

func readJSON(path string) (map[string]any, error) {
	data, err := os.ReadFile(path) //nolint:gosec // G304: path is the user's home dir plus a fixed config filename, not external input.
	if errors.Is(err, os.ErrNotExist) {
		return map[string]any{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("install: read %s: %w", path, err)
	}
	if strings.TrimSpace(string(data)) == "" {
		return map[string]any{}, nil
	}
	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("install: %s is not valid JSON; acm will not overwrite it. Fix or move it, then retry: %w", path, err)
	}
	if m == nil {
		m = map[string]any{}
	}
	return m, nil
}

func writeJSON(path string, m map[string]any) error {
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return fmt.Errorf("install: encode %s: %w", path, err)
	}
	return writeFile(path, append(data, '\n'))
}

func readText(path string) (string, error) {
	data, err := os.ReadFile(path) //nolint:gosec // G304: path is the user's home dir plus a fixed config filename, not external input.
	if errors.Is(err, os.ErrNotExist) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("install: read %s: %w", path, err)
	}
	return string(data), nil
}

func writeText(path, text string) error {
	return writeFile(path, []byte(text))
}

func writeFile(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return fmt.Errorf("install: create dir for %s: %w", path, err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return fmt.Errorf("install: write %s: %w", path, err)
	}
	return nil
}
