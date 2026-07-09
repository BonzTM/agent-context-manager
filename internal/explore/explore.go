// Package explore builds deterministic, type-aware exploration summaries for
// offloaded large content. Structured payloads (JSON, CSV/TSV, SQL) and source
// code get a schema- or structure-level description with no model call; content
// no extractor recognizes is left to the caller's summarizer. The summary's job
// is orientation — telling the agent what the offloaded file is and whether to
// read it — not compression.
package explore

import (
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"
)

// maxSummaryChars bounds every extractor's output (~200 tokens).
const maxSummaryChars = 800

// Extractor names recorded on the large_files row for observability.
const (
	ExtractorJSON = "json"
	ExtractorCSV  = "csv"
	ExtractorSQL  = "sql"
	ExtractorCode = "code"
)

// Describe returns a deterministic exploration summary for content when one of
// the type-aware extractors recognizes it, along with the extractor's name.
// ok is false when no extractor applies and the caller should fall back to its
// configured summarizer.
func Describe(content string) (summary, extractor string, ok bool) {
	trimmed := strings.TrimSpace(content)
	if trimmed == "" {
		return "", "", false
	}
	if s, isJSON := describeJSON(trimmed); isJSON {
		return clamp(s), ExtractorJSON, true
	}
	if s, isCSV := describeCSV(trimmed); isCSV {
		return clamp(s), ExtractorCSV, true
	}
	if s, isSQL := describeSQL(trimmed); isSQL {
		return clamp(s), ExtractorSQL, true
	}
	if s, isCode := describeCode(trimmed); isCode {
		return clamp(s), ExtractorCode, true
	}
	return "", "", false
}

// --- JSON ---

func describeJSON(s string) (string, bool) {
	if s[0] != '{' && s[0] != '[' {
		return "", false
	}
	var v any
	if err := json.Unmarshal([]byte(s), &v); err != nil {
		return "", false
	}
	return "JSON " + describeValue(v, 0), true
}

// describeValue renders a shallow shape description: object keys with value
// kinds one level deep, array length plus the shape of the first element.
func describeValue(v any, depth int) string {
	switch t := v.(type) {
	case map[string]any:
		keys := make([]string, 0, len(t))
		for k := range t {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		shown := keys
		if len(shown) > 12 {
			shown = shown[:12]
		}
		parts := make([]string, 0, len(shown))
		for _, k := range shown {
			if depth == 0 {
				parts = append(parts, k+" ("+kindOf(t[k])+")")
			} else {
				parts = append(parts, k)
			}
		}
		desc := fmt.Sprintf("object, %d keys: %s", len(t), strings.Join(parts, ", "))
		if len(keys) > len(shown) {
			desc += ", …"
		}
		return desc
	case []any:
		if len(t) == 0 {
			return "array, empty"
		}
		if depth >= 2 {
			return fmt.Sprintf("array, %d elements", len(t))
		}
		return fmt.Sprintf("array, %d elements; element 0: %s", len(t), describeValue(t[0], depth+1))
	default:
		return kindOf(v) + " value"
	}
}

func kindOf(v any) string {
	switch t := v.(type) {
	case map[string]any:
		return fmt.Sprintf("object[%d]", len(t))
	case []any:
		return fmt.Sprintf("array[%d]", len(t))
	case string:
		return "string"
	case float64:
		return "number"
	case bool:
		return "bool"
	case nil:
		return "null"
	default:
		return "value"
	}
}

// --- CSV / TSV ---

func describeCSV(s string) (string, bool) {
	lines := nonEmptyLines(s, 8)
	if len(lines) < 2 {
		return "", false
	}
	for _, sep := range []string{"\t", ","} {
		cols := strings.Count(lines[0], sep) + 1
		if cols < 2 {
			continue
		}
		consistent := true
		for _, l := range lines[1:] {
			if strings.Count(l, sep)+1 != cols {
				consistent = false
				break
			}
		}
		if !consistent {
			continue
		}
		total := strings.Count(strings.TrimSpace(s), "\n") + 1
		name := "CSV"
		if sep == "\t" {
			name = "TSV"
		}
		header := strings.Split(lines[0], sep)
		if len(header) > 12 {
			header = header[:12]
		}
		for i := range header {
			header[i] = strings.TrimSpace(header[i])
		}
		return fmt.Sprintf("%s, %d rows × %d columns; header: %s",
			name, total-1, cols, strings.Join(header, ", ")), true
	}
	return "", false
}

// --- SQL ---

var (
	sqlKeywords = map[string]bool{
		"SELECT": true, "INSERT": true, "UPDATE": true, "DELETE": true,
		"CREATE": true, "ALTER": true, "DROP": true, "WITH": true,
		"BEGIN": true, "PRAGMA": true, "EXPLAIN": true,
	}
	sqlTableRe = regexp.MustCompile(`(?i)\b(?:FROM|JOIN|INTO|UPDATE|TABLE(?:\s+IF\s+(?:NOT\s+)?EXISTS)?)\s+([A-Za-z_][A-Za-z0-9_.]*)`)
)

// sqlBodyRe requires a secondary structural keyword so prose that merely opens
// with "Select ..." or "Create ..." is not misclassified as a SQL script.
var sqlBodyRe = regexp.MustCompile(`(?i)\b(FROM|INTO|TABLE|SET|VALUES|WHERE)\b`)

func describeSQL(s string) (string, bool) {
	first := strings.ToUpper(firstWord(s))
	if !sqlKeywords[first] || !sqlBodyRe.MatchString(s) {
		return "", false
	}
	// English prose can open with a keyword and contain "from"; real SQL
	// scripts carry structural punctuation (terminators, parens, operators)
	// or span multiple lines.
	if !strings.ContainsAny(s, ";(=*") && !strings.Contains(s, "\n") {
		return "", false
	}
	kinds := map[string]int{}
	statements := 0
	for stmt := range strings.SplitSeq(s, ";") {
		w := strings.ToUpper(firstWord(stmt))
		if w == "" {
			continue
		}
		if !sqlKeywords[w] {
			// A non-SQL leading word mid-stream means this is prose that merely
			// starts with a keyword, not a SQL script.
			return "", false
		}
		kinds[w]++
		statements++
	}
	if statements == 0 {
		return "", false
	}
	kindNames := make([]string, 0, len(kinds))
	for k := range kinds {
		kindNames = append(kindNames, k)
	}
	sort.Strings(kindNames)
	parts := make([]string, 0, len(kindNames))
	for _, k := range kindNames {
		parts = append(parts, fmt.Sprintf("%d %s", kinds[k], k))
	}

	tables := uniqueMatches(sqlTableRe, s, 8)
	desc := fmt.Sprintf("SQL, %d statements (%s)", statements, strings.Join(parts, ", "))
	if len(tables) > 0 {
		desc += "; tables: " + strings.Join(tables, ", ")
	}
	return desc, true
}

// --- Source code ---

// codeDeclRe matches top-level declaration lines across the common languages
// agents handle (Go, Python, JS/TS, Rust, Java-ish).
var codeDeclRe = regexp.MustCompile(`^\s*(package |import |func |type |class |def |function |fn |pub fn |const |interface |struct |impl |module |public class |export )`)

func describeCode(s string) (string, bool) {
	lines := strings.Split(s, "\n")
	var decls []string
	nonBlank := 0
	for _, l := range lines {
		if strings.TrimSpace(l) == "" {
			continue
		}
		nonBlank++
		if codeDeclRe.MatchString(l) {
			decls = append(decls, strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(l), "{")))
		}
	}
	// Require enough declaration density that prose does not masquerade as code.
	if nonBlank < 5 || len(decls) < 3 || len(decls)*20 < nonBlank {
		return "", false
	}
	shown := decls
	if len(shown) > 12 {
		shown = shown[:12]
	}
	desc := fmt.Sprintf("Code, %d lines, %d top-level declarations:\n  %s",
		len(lines), len(decls), strings.Join(shown, "\n  "))
	if len(decls) > len(shown) {
		desc += "\n  …"
	}
	return desc, true
}

// --- helpers ---

func nonEmptyLines(s string, limit int) []string {
	var out []string
	for l := range strings.SplitSeq(s, "\n") {
		if strings.TrimSpace(l) == "" {
			continue
		}
		out = append(out, l)
		if len(out) == limit {
			break
		}
	}
	return out
}

func firstWord(s string) string {
	fields := strings.Fields(s)
	if len(fields) == 0 {
		return ""
	}
	return strings.Trim(fields[0], "(")
}

func uniqueMatches(re *regexp.Regexp, s string, limit int) []string {
	seen := map[string]bool{}
	var out []string
	for _, m := range re.FindAllStringSubmatch(s, -1) {
		name := m[1]
		if seen[strings.ToLower(name)] {
			continue
		}
		seen[strings.ToLower(name)] = true
		out = append(out, name)
		if len(out) == limit {
			break
		}
	}
	return out
}

func clamp(s string) string {
	r := []rune(s)
	if len(r) <= maxSummaryChars {
		return s
	}
	return string(r[:maxSummaryChars]) + "…"
}
