// Package privacy applies project capture exclusions and deterministic secret
// redaction before data reaches persistence, FTS, summaries, or backups.
package privacy

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/bonztm/agent-context-manager/internal/core"
)

const (
	maxJSONNodes            = 10_000
	maxRedactionsPerMessage = 10_000
)

type redactionPattern struct {
	class          string
	expression     *regexp.Regexp
	preservePrefix bool
}

// Policy is an immutable, validated capture policy.
type Policy struct {
	redact                bool
	ignoreSessionGlobs    []string
	statelessSessionGlobs []string
	toolGlobs             []string
	pathGlobs             []string
	excludedClass         map[string]bool
	allowedValues         []string
	redactions            []redactionPattern
	classDetectors        map[string]*regexp.Regexp
}

// SessionMode controls recall and persistence for a host session.
type SessionMode string

const (
	// SessionCapture persists messages and permits recall.
	SessionCapture SessionMode = "capture"
	// SessionIgnore permits neither recall nor persistence.
	SessionIgnore SessionMode = "ignore"
	// SessionStateless permits recall but does not persist.
	SessionStateless SessionMode = "stateless"
)

// Mode returns the deterministic policy mode for sessionID. Ignore takes
// precedence when patterns overlap.
func (policy *Policy) Mode(sessionID string) SessionMode {
	if matchesAny(policy.ignoreSessionGlobs, sessionID) {
		return SessionIgnore
	}
	if matchesAny(policy.statelessSessionGlobs, sessionID) {
		return SessionStateless
	}
	return SessionCapture
}

// Apply implements core.MessagePolicy.
func (policy *Policy) Apply(request core.IngestRequest) (core.IngestRequest, core.PolicyDecision, error) {
	if policy.Mode(request.SessionID) != SessionCapture {
		return request, core.PolicyDecision{SessionExcluded: true, MessagesExcluded: len(request.Messages)}, nil
	}
	filtered := request
	filtered.Messages = make([]core.IngestMessage, 0, len(request.Messages))
	var decision core.PolicyDecision
	for _, message := range request.Messages {
		if policy.excludeMessage(message) {
			decision.MessagesExcluded++
			continue
		}
		redacted, changed, err := policy.redactMessage(message)
		if err != nil {
			return request, decision, err
		}
		if changed {
			decision.MessagesRedacted++
		}
		filtered.Messages = append(filtered.Messages, redacted)
	}
	return filtered, decision, nil
}

func (policy *Policy) excludeMessage(message core.IngestMessage) bool {
	if matchesAny(policy.toolGlobs, message.ToolName) || policy.rawContainsExcludedPath(message.Raw) {
		return true
	}
	content := message.Content + "\n" + message.Raw
	for class := range policy.excludedClass {
		if detector := policy.classDetectors[class]; detector != nil && detector.MatchString(content) {
			return true
		}
	}
	return false
}

func (policy *Policy) redactMessage(message core.IngestMessage) (core.IngestMessage, bool, error) {
	if !policy.redact {
		return message, false, nil
	}
	content, err := policy.redactText(message.Content)
	if err != nil {
		return message, false, err
	}
	raw, err := policy.redactText(message.Raw)
	if err != nil {
		return message, false, err
	}
	changed := content != message.Content || raw != message.Raw
	message.Content, message.Raw = content, raw
	return message, changed, nil
}

func (policy *Policy) redactText(value string) (string, error) {
	result := value
	for _, pattern := range policy.redactions {
		var err error
		result, err = policy.redactPattern(result, pattern)
		if err != nil {
			return "", err
		}
	}
	return result, nil
}

func (policy *Policy) redactPattern(value string, pattern redactionPattern) (string, error) {
	matches := pattern.expression.FindAllStringIndex(value, maxRedactionsPerMessage+1)
	if len(matches) > maxRedactionsPerMessage {
		return "", fmt.Errorf("privacy: %s redactions exceed per-message limit", pattern.class)
	}
	var output strings.Builder
	previous := 0
	for _, indexes := range matches {
		match := value[indexes[0]:indexes[1]]
		output.WriteString(value[previous:indexes[0]])
		output.WriteString(policy.redactedMatch(match, pattern))
		previous = indexes[1]
	}
	output.WriteString(value[previous:])
	return output.String(), nil
}

func (policy *Policy) redactedMatch(match string, pattern redactionPattern) string {
	if policy.allowed(match) {
		return match
	}
	marker := "[REDACTED:" + pattern.class + "]"
	if pattern.preservePrefix {
		return assignmentPrefix(match) + marker
	}
	return marker
}

func (policy *Policy) allowed(match string) bool {
	for _, value := range policy.allowedValues {
		if strings.Contains(match, value) {
			return true
		}
	}
	return false
}

func assignmentPrefix(match string) string {
	index := strings.LastIndexAny(match, ":=")
	if index < 0 {
		return ""
	}
	rest := match[index+1:]
	space := len(rest) - len(strings.TrimLeft(rest, " \t"))
	prefix := match[:index+1] + rest[:space]
	if space < len(rest) && (rest[space] == '\'' || rest[space] == '"') {
		prefix += rest[space : space+1]
	}
	return prefix
}

func (policy *Policy) rawContainsExcludedPath(raw string) bool {
	if raw == "" || len(policy.pathGlobs) == 0 {
		return false
	}
	var root any
	if json.Unmarshal([]byte(raw), &root) != nil {
		return false
	}
	stack := []any{root}
	for visited := 0; len(stack) > 0 && visited < maxJSONNodes; visited++ {
		last := len(stack) - 1
		value := stack[last]
		stack = stack[:last]
		switch typed := value.(type) {
		case map[string]any:
			for key, child := range typed {
				if text, ok := child.(string); ok && pathKey(key) && matchesAny(policy.pathGlobs, text) {
					return true
				}
				stack = append(stack, child)
			}
		case []any:
			stack = append(stack, typed...)
		}
	}
	return false
}

func pathKey(key string) bool {
	lower := strings.ToLower(key)
	return strings.Contains(lower, "path") || strings.Contains(lower, "file") || lower == "cwd"
}

func matchesAny(patterns []string, value string) bool {
	for _, pattern := range patterns {
		matched, err := filepath.Match(pattern, value)
		if err == nil && matched {
			return true
		}
	}
	return false
}

func compilePattern(class, expression string, preservePrefix bool) (redactionPattern, error) {
	compiled, err := regexp.Compile(expression)
	if err != nil {
		return redactionPattern{}, fmt.Errorf("privacy: compile %s detector: %w", class, err)
	}
	return redactionPattern{class: class, expression: compiled, preservePrefix: preservePrefix}, nil
}
