// Package install performs global, idempotent, non-destructive installation of
// acm's hooks and drill-down instructions into a host agent's user-level
// configuration. A single install applies to every project, because acm
// resolves the per-project database from the working directory at hook time.
//
// Edits are safe by construction: JSON and TOML configs are parsed before they
// are changed, existing keys are preserved, and instruction blocks are
// marker-guarded so re-running updates in place. acm never overwrites a config
// it cannot parse.
package install

import (
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"

	"github.com/bonztm/agent-context-manager/internal/agents"
	"github.com/bonztm/agent-context-manager/internal/core"
)

// opencodePluginSrc is the canonical OpenCode plugin, embedded so a global
// install can drop it into OpenCode's auto-load directory with no npm step.
//
//go:embed assets/opencode-acm.ts
var opencodePluginSrc string

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
		if err := res.add(codexHooks(home, apply)); err != nil {
			return Result{}, err
		}
		if err := res.add(codexNotify(home, apply)); err != nil {
			return Result{}, err
		}
		if err := res.add(instructions(filepath.Join(home, ".codex", "AGENTS.md"), apply)); err != nil {
			return Result{}, err
		}
		res.Notes = append(res.Notes,
			"Codex hooks (~/.codex/hooks.json) are user-level and need no project trust. notify (assistant-turn capture) is set in ~/.codex/config.toml and must be global, which it now is.")
	case core.AgentOpenCode:
		if err := res.add(opencodePlugin(home, apply)); err != nil {
			return Result{}, err
		}
		if err := res.add(instructions(filepath.Join(opencodeConfigDir(home), "AGENTS.md"), apply)); err != nil {
			return Result{}, err
		}
		res.Notes = append(res.Notes,
			"The OpenCode plugin is auto-loaded from its directory (no opencode.json edit needed) and shells out to 'acm' — ensure the binary is on PATH.")
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
	changed = ensureHookEntry(m, "UserPromptSubmit", "", "acm hook --agent claude-code --event UserPromptSubmit") || changed
	changed = ensureHookEntry(m, "PostToolUse", "*", "acm hook --agent claude-code --event PostToolUse") || changed
	// Stop reconciles assistant turns from the transcript and triggers
	// opportunistic compaction — without it, capture is user+tool only.
	changed = ensureHookEntry(m, "Stop", "", "acm hook --agent claude-code --event Stop") || changed
	return finishJSON(path, m, changed, apply, "hooks + Bash(acm:*) permission")
}

// --- Codex: ~/.codex/hooks.json (standalone JSON, same nested schema as Claude) ---

func codexHooks(home string, apply bool) (Change, error) {
	path := filepath.Join(home, ".codex", "hooks.json")
	m, err := readJSON(path)
	if err != nil {
		return Change{}, err
	}
	// UserPromptSubmit ignores matchers; PostToolUse with no matcher fires on
	// every tool. The command is a single string, per Codex's hooks schema.
	changed := ensureHookEntry(m, "UserPromptSubmit", "", "acm hook --agent codex --event UserPromptSubmit")
	changed = ensureHookEntry(m, "PostToolUse", "", "acm hook --agent codex --event PostToolUse") || changed
	return finishJSON(path, m, changed, apply, "UserPromptSubmit + PostToolUse hooks")
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

// ensureHookEntry adds a command hook for an event under the top-level "hooks"
// object, idempotently. The nested schema ({event: [{matcher?, hooks: [{type,
// command}]}]}) is shared by Claude Code settings.json and Codex hooks.json.
func ensureHookEntry(m map[string]any, event, matcher, command string) bool {
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

// --- OpenCode: drop the plugin into the auto-load directory ---

// opencodeConfigDir resolves OpenCode's config directory: $OPENCODE_CONFIG_DIR,
// then $XDG_CONFIG_HOME/opencode, else <home>/.config/opencode. OpenCode
// auto-loads any plugin/*.ts under it — no opencode.json edit is needed.
func opencodeConfigDir(home string) string {
	if dir := os.Getenv("OPENCODE_CONFIG_DIR"); dir != "" {
		return dir
	}
	if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
		return filepath.Join(xdg, "opencode")
	}
	return filepath.Join(home, ".config", "opencode")
}

func opencodePlugin(home string, apply bool) (Change, error) {
	path := filepath.Join(opencodeConfigDir(home), "plugin", "acm.ts")
	existing, err := readText(path)
	if err != nil {
		return Change{}, err
	}
	if existing == opencodePluginSrc {
		return Change{Path: path, Summary: "plugin up to date", Skipped: true}, nil
	}
	if apply {
		if wErr := writeText(path, opencodePluginSrc); wErr != nil {
			return Change{}, wErr
		}
	}
	what := "OpenCode plugin (auto-loaded)"
	if existing != "" {
		what = "OpenCode plugin (updated to current version)"
	}
	return Change{Path: path, Summary: verb(apply) + " " + what, Applied: apply}, nil
}

// --- Codex: ~/.codex/config.toml (top-level notify) ---

const acmNotifyMarker = "# acm: capture each turn's final assistant message"

const acmNotifyLine = `notify = ["acm", "hook", "--agent", "codex", "--event", "agent-turn-complete"]`

func codexNotify(home string, apply bool) (Change, error) {
	path := filepath.Join(home, ".codex", "config.toml")
	text, err := readText(path)
	if err != nil {
		return Change{}, err
	}
	var config map[string]any
	if _, err = toml.Decode(text, &config); err != nil {
		return Change{}, fmt.Errorf("install: parse %s: %w", path, err)
	}

	if _, exists := config["notify"]; exists {
		return Change{Path: path, Summary: "top-level notify already configured", Skipped: true}, nil
	}

	cleaned := removeLegacyACMNotify(text)
	newText := acmNotifyMarker + " (global)\n" + acmNotifyLine + "\n\n" + strings.TrimLeft(cleaned, "\r\n")
	var updated map[string]any
	if _, err = toml.Decode(newText, &updated); err != nil {
		return Change{}, fmt.Errorf("install: validate %s update: %w", path, err)
	}
	if _, exists := updated["notify"]; !exists {
		return Change{}, fmt.Errorf("install: validate %s update: top-level notify missing", path)
	}
	if apply {
		if wErr := writeText(path, newText); wErr != nil {
			return Change{}, wErr
		}
	}
	return Change{Path: path, Summary: verb(apply) + " notify (assistant-turn capture)", Applied: apply}, nil
}

func removeLegacyACMNotify(text string) string {
	lines := strings.SplitAfter(text, "\n")
	out := make([]string, 0, len(lines))
	for i := 0; i < len(lines); i++ {
		marker := strings.TrimSpace(lines[i])
		if marker != acmNotifyMarker && marker != acmNotifyMarker+" (global)" {
			out = append(out, lines[i])
			continue
		}
		if i+1 < len(lines) && strings.TrimSpace(lines[i+1]) == acmNotifyLine {
			i++
			continue
		}
		out = append(out, lines[i])
	}
	return strings.Join(out, "")
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
	newText, changed, unmanaged := ensureInstructions(text, agents.DrillDownDoc)
	if unmanaged {
		return Change{
			Path:    path,
			Summary: "drill-down instructions present (added manually, without acm's markers — left untouched; wrap the block in " + markerStart + " / " + markerEnd + " to let acm manage updates)",
			Skipped: true,
		}, nil
	}
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

// instructionsHeading identifies the drill-down block even when a user pasted
// it by hand without acm's markers, so init never appends a duplicate copy.
var instructionsHeading = strings.SplitN(agents.DrillDownDoc, "\n", 2)[0]

// ensureInstructions returns the updated file text. unmanaged reports that the
// block's heading exists outside acm's markers (a hand-maintained copy), which
// acm deliberately refuses to touch or duplicate.
func ensureInstructions(text, block string) (newText string, changed, unmanaged bool) {
	wrapped := markerStart + "\n" + strings.TrimRight(block, "\n") + "\n" + markerEnd
	start := strings.Index(text, markerStart)
	end := strings.Index(text, markerEnd)
	if start >= 0 && end > start {
		updated := text[:start] + wrapped + text[end+len(markerEnd):]
		return updated, updated != text, false
	}
	if strings.Contains(text, instructionsHeading) {
		return text, false, true
	}
	if strings.TrimSpace(text) == "" {
		return wrapped + "\n", true, false
	}
	return strings.TrimRight(text, "\n") + "\n\n" + wrapped + "\n", true, false
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
	data, err := os.ReadFile(path)
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
	data, err := os.ReadFile(path)
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

// writeFile writes atomically (temp file + rename in the same directory) so a
// crash mid-write can never truncate or corrupt a user's live agent config.
// Symlinks are followed first and the rename replaces the final TARGET, never
// the link itself — users commonly symlink CLAUDE.md/AGENTS.md (or a whole
// config) into a dotfiles repo, and a naive rename would orphan that link by
// converting it into a regular file. An existing target's file mode is
// preserved; new files are created 0600.
func writeFile(path string, data []byte) error {
	target, err := resolveSymlinks(path)
	if err != nil {
		return fmt.Errorf("install: resolve %s: %w", path, err)
	}

	mode := os.FileMode(0o600)
	if fi, sErr := os.Stat(target); sErr == nil {
		mode = fi.Mode().Perm()
	}

	dir := filepath.Dir(target)
	if mkErr := os.MkdirAll(dir, 0o750); mkErr != nil {
		return fmt.Errorf("install: create dir for %s: %w", target, mkErr)
	}
	tmp, err := os.CreateTemp(dir, filepath.Base(target)+".tmp-*")
	if err != nil {
		return fmt.Errorf("install: create temp for %s: %w", target, err)
	}
	tmpName := tmp.Name()
	defer func() { _ = os.Remove(tmpName) }() // no-op after a successful rename

	if err := tmp.Chmod(mode); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("install: chmod temp for %s: %w", target, err)
	}
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("install: write temp for %s: %w", target, err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("install: close temp for %s: %w", target, err)
	}
	if err := os.Rename(tmpName, target); err != nil {
		return fmt.Errorf("install: replace %s: %w", target, err)
	}
	return nil
}

// resolveSymlinks walks the symlink chain by hand so that, unlike
// filepath.EvalSymlinks, a DANGLING final link still resolves to the intended
// target location (the file is then created there and the link starts working).
func resolveSymlinks(path string) (string, error) {
	const maxHops = 40
	resolved := path
	for range maxHops {
		fi, err := os.Lstat(resolved)
		if errors.Is(err, os.ErrNotExist) {
			return resolved, nil // does not exist yet; create here
		}
		if err != nil {
			return "", err
		}
		if fi.Mode()&os.ModeSymlink == 0 {
			return resolved, nil
		}
		next, err := os.Readlink(resolved)
		if err != nil {
			return "", err
		}
		if !filepath.IsAbs(next) {
			next = filepath.Join(filepath.Dir(resolved), next)
		}
		resolved = next
	}
	return "", errors.New("too many levels of symbolic links")
}
