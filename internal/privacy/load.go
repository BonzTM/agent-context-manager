package privacy

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/BurntSushi/toml"
)

const (
	// FileName is the committable project-root privacy policy.
	FileName       = ".acm-policy.toml"
	maxPolicyBytes = 1 << 20
	maxPolicyRules = 128
)

type diskPolicy struct {
	Redact                *bool    `toml:"redact"`
	ExcludeSessions       []string `toml:"exclude_sessions"` // compatibility alias for stateless_sessions
	IgnoreSessions        []string `toml:"ignore_sessions"`
	StatelessSessions     []string `toml:"stateless_sessions"`
	ExcludeTools          []string `toml:"exclude_tools"`
	ExcludePaths          []string `toml:"exclude_paths"`
	ExcludeContentClasses []string `toml:"exclude_content_classes"`
	AllowValues           []string `toml:"allow_values"`
}

// Load reads a project policy. A missing file returns secure redaction
// defaults; exclusions are opt-in.
func Load(projectRoot string) (*Policy, error) {
	config := diskPolicy{}
	path := filepath.Join(projectRoot, FileName)
	content, err := readPolicy(path)
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	if err == nil {
		if _, err = toml.Decode(string(content), &config); err != nil {
			return nil, fmt.Errorf("privacy: parse %s: %w", path, err)
		}
	}
	return newPolicy(config)
}

func newPolicy(config diskPolicy) (*Policy, error) {
	redact := true
	if config.Redact != nil {
		redact = *config.Redact
	}
	if err := validateRules(config); err != nil {
		return nil, err
	}
	redactions, detectors, err := compileDetectors()
	if err != nil {
		return nil, err
	}
	excluded := make(map[string]bool, len(config.ExcludeContentClasses))
	for _, class := range config.ExcludeContentClasses {
		if detectors[class] == nil {
			return nil, fmt.Errorf("privacy: unsupported content class %q", class)
		}
		excluded[class] = true
	}
	return &Policy{
		redact: redact, ignoreSessionGlobs: config.IgnoreSessions,
		statelessSessionGlobs: append(config.StatelessSessions, config.ExcludeSessions...), toolGlobs: config.ExcludeTools,
		pathGlobs: config.ExcludePaths, excludedClass: excluded, allowedValues: config.AllowValues,
		redactions: redactions, classDetectors: detectors,
	}, nil
}

func validateRules(config diskPolicy) error {
	ruleSets := [][]string{config.ExcludeSessions, config.IgnoreSessions, config.StatelessSessions, config.ExcludeTools, config.ExcludePaths}
	for _, rules := range ruleSets {
		if len(rules) > maxPolicyRules {
			return fmt.Errorf("privacy: rule count exceeds %d", maxPolicyRules)
		}
		for _, rule := range rules {
			if _, err := filepath.Match(rule, ""); err != nil {
				return fmt.Errorf("privacy: invalid glob %q: %w", rule, err)
			}
		}
	}
	if len(config.ExcludeContentClasses) > maxPolicyRules || len(config.AllowValues) > maxPolicyRules {
		return fmt.Errorf("privacy: rule count exceeds %d", maxPolicyRules)
	}
	return nil
}

func compileDetectors() ([]redactionPattern, map[string]*regexp.Regexp, error) {
	specs := []struct {
		class, expression string
		preservePrefix    bool
	}{
		{"private-key", `(?s)-----BEGIN (?:[A-Z ]+ )?PRIVATE KEY-----.*?-----END (?:[A-Z ]+ )?PRIVATE KEY-----`, false},
		{"aws-key", `\b(?:AKIA|ASIA)[A-Z0-9]{16}\b`, false},
		{"github-token", `\bgh[pousr]_[A-Za-z0-9_]{20,}\b`, false},
		{"api-token", `\bsk-[A-Za-z0-9_-]{20,}\b`, false},
		{"jwt", `\beyJ[A-Za-z0-9_-]+\.eyJ[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+\b`, false},
		{"bearer-token", `(?i)\bBearer[ \t]+[A-Za-z0-9._~+/=-]{12,}`, false},
		{"credential", `(?i)"?(?:api[_-]?key|access[_-]?token|auth[_-]?token|password|passwd|secret)"?[ \t]*[:=][ \t]*["']?[A-Za-z0-9._~+/=-]{8,}`, true},
	}
	patterns := make([]redactionPattern, 0, len(specs))
	detectors := make(map[string]*regexp.Regexp, len(specs)+1)
	for _, spec := range specs {
		pattern, err := compilePattern(spec.class, spec.expression, spec.preservePrefix)
		if err != nil {
			return nil, nil, err
		}
		patterns = append(patterns, pattern)
		detectors[spec.class] = pattern.expression
	}
	personalExpression := `(?i)\b[0-9]{3}-[0-9]{2}-[0-9]{4}\b|\b[A-Z0-9._%+-]+@[A-Z0-9.-]+\.[A-Z]{2,}\b`
	personal, err := regexp.Compile(personalExpression)
	if err != nil {
		return nil, nil, fmt.Errorf("privacy: compile personal-data detector: %w", err)
	}
	detectors["personal-data"] = personal
	detectors["secrets"], err = regexp.Compile(joinDetectorExpressions(specs))
	if err != nil {
		return nil, nil, fmt.Errorf("privacy: compile combined secret detector: %w", err)
	}
	return patterns, detectors, nil
}

func joinDetectorExpressions(specs []struct {
	class, expression string
	preservePrefix    bool
},
) string {
	parts := make([]string, 0, len(specs))
	for _, spec := range specs {
		parts = append(parts, "(?:"+spec.expression+")")
	}
	return strings.Join(parts, "|")
}

func readPolicy(path string) ([]byte, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = file.Close() }()
	content, err := io.ReadAll(io.LimitReader(file, maxPolicyBytes+1))
	if err != nil {
		return nil, fmt.Errorf("privacy: read %s: %w", path, err)
	}
	if len(content) > maxPolicyBytes {
		return nil, fmt.Errorf("privacy: %s exceeds %d bytes", path, maxPolicyBytes)
	}
	return content, nil
}
