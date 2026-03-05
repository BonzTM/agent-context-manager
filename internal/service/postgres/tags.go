package postgres

import (
	_ "embed"
	"encoding/json"
	"sort"
	"strings"
)

const maxTaskCanonicalTags = 6

//go:embed canonical_tags.json
var canonicalTagsJSON []byte

type canonicalTagsConfig struct {
	CanonicalTags map[string][]string `json:"canonical_tags"`
}

var canonicalTagAliasMap = loadCanonicalTagAliasMap(canonicalTagsJSON)

func loadCanonicalTagAliasMap(raw []byte) map[string]string {
	cfg := canonicalTagsConfig{}
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return map[string]string{}
	}

	aliasMap := make(map[string]string)
	for canonical, aliases := range cfg.CanonicalTags {
		canonical = strings.ToLower(strings.TrimSpace(canonical))
		if canonical == "" {
			continue
		}
		aliasMap[canonical] = canonical
		for _, alias := range aliases {
			alias = strings.ToLower(strings.TrimSpace(alias))
			if alias == "" {
				continue
			}
			aliasMap[alias] = canonical
		}
	}
	return aliasMap
}

func normalizeCanonicalTag(raw string) string {
	tag := strings.ToLower(strings.TrimSpace(raw))
	if tag == "" {
		return ""
	}
	if canonical, ok := canonicalTagAliasMap[tag]; ok {
		return canonical
	}
	return tag
}

func normalizeCanonicalTags(values []string) []string {
	if len(values) == 0 {
		return nil
	}

	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))
	for _, raw := range values {
		tag := normalizeCanonicalTag(raw)
		if tag == "" {
			continue
		}
		if _, ok := seen[tag]; ok {
			continue
		}
		seen[tag] = struct{}{}
		out = append(out, tag)
	}
	if len(out) == 0 {
		return nil
	}
	sort.Strings(out)
	return out
}

func canonicalTagsFromTaskText(taskText string) []string {
	if strings.TrimSpace(taskText) == "" || len(canonicalTagAliasMap) == 0 {
		return nil
	}

	seen := make(map[string]struct{})
	tokens := wordPattern.FindAllString(taskText, -1)
	for _, token := range tokens {
		token = strings.ToLower(strings.TrimSpace(token))
		if token == "" {
			continue
		}
		if canonical, ok := canonicalTagAliasMap[token]; ok {
			seen[canonical] = struct{}{}
		}
		for _, part := range strings.FieldsFunc(token, isTagTokenSeparator) {
			if canonical, ok := canonicalTagAliasMap[part]; ok {
				seen[canonical] = struct{}{}
			}
		}
	}

	out := mapKeysSorted(seen)
	if len(out) > maxTaskCanonicalTags {
		return out[:maxTaskCanonicalTags]
	}
	return out
}

func isTagTokenSeparator(r rune) bool {
	switch r {
	case '.', '_', ':', '/', '-':
		return true
	default:
		return false
	}
}
