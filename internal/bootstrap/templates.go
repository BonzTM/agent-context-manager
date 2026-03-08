package bootstrap

import (
	"bytes"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"reflect"
	"sort"
	"strconv"
	"strings"

	"github.com/bonztm/agent-context-manager/internal/contracts/v1"
	"gopkg.in/yaml.v3"
)

//go:embed all:bootstrap_templates/**
var bootstrapTemplateFS embed.FS

const (
	bootstrapTemplateManifestVersion             = "acm.bootstrap-template.v1"
	bootstrapTemplateOpCreateIfMissing           = "create_if_missing"
	bootstrapTemplateOpCreateOrReplaceIfPristine = "create_or_replace_if_pristine"
	bootstrapTemplateOpReplaceIfPristine         = "replace_if_pristine"
	bootstrapTemplateOpMergeJSON                 = "merge_json"
	BlankRulesContents                           = "version: acm.rules.v1\nrules: []\n"
	BlankTestsContents                           = "version: acm.tests.v1\ndefaults:\n  cwd: .\n  timeout_sec: 300\ntests: []\n"
	BlankWorkflowsContents                       = "version: acm.workflows.v1\ncompletion:\n  required_tasks: []\n"
)

var bootstrapTemplateAliases = map[string]string{
	"claude-receipt-guard": "claude-hooks",
}

type bootstrapTemplateManifest struct {
	Version    string                               `yaml:"version"`
	ID         string                               `yaml:"id"`
	Summary    string                               `yaml:"summary"`
	Operations []bootstrapTemplateOperationManifest `yaml:"operations"`
}

type bootstrapTemplateOperationManifest struct {
	Type     string   `yaml:"type"`
	Target   string   `yaml:"target"`
	Source   string   `yaml:"source,omitempty"`
	Mode     string   `yaml:"mode,omitempty"`
	Pristine []string `yaml:"pristine,omitempty"`
}

type Template struct {
	ID         string
	Summary    string
	Operations []bootstrapTemplateOperation
}

type bootstrapTemplateOperation struct {
	Type     string
	Target   string
	Source   string
	Mode     os.FileMode
	Pristine []string
}

type bootstrapTemplateContext struct {
	ProjectID string
	RepoName  string
}

type ApplyResult struct {
	TemplateResults []v1.BootstrapTemplateResult
	CandidatePaths  []string
}

type UnknownTemplateError struct {
	TemplateID string
}

func (e UnknownTemplateError) Error() string {
	return fmt.Sprintf("unknown bootstrap template %q", e.TemplateID)
}

func ResolveTemplates(templateIDs []string) ([]Template, error) {
	normalizedIDs := normalizeValues(templateIDs)
	if len(normalizedIDs) == 0 {
		return nil, nil
	}

	catalog, err := loadBootstrapTemplateCatalog()
	if err != nil {
		return nil, err
	}

	templates := make([]Template, 0, len(normalizedIDs))
	seen := make(map[string]struct{}, len(normalizedIDs))
	for _, templateID := range normalizedIDs {
		canonicalID := canonicalBootstrapTemplateID(templateID)
		if _, ok := seen[canonicalID]; ok {
			continue
		}
		seen[canonicalID] = struct{}{}

		template, ok := catalog[canonicalID]
		if !ok {
			return nil, UnknownTemplateError{TemplateID: templateID}
		}
		templates = append(templates, template)
	}

	return templates, nil
}

func canonicalBootstrapTemplateID(templateID string) string {
	if alias, ok := bootstrapTemplateAliases[strings.TrimSpace(templateID)]; ok {
		return alias
	}
	return strings.TrimSpace(templateID)
}

func loadBootstrapTemplateCatalog() (map[string]Template, error) {
	manifestPaths, err := fs.Glob(bootstrapTemplateFS, "bootstrap_templates/*/template.yaml")
	if err != nil {
		return nil, fmt.Errorf("glob manifests: %w", err)
	}
	sort.Strings(manifestPaths)

	catalog := make(map[string]Template, len(manifestPaths))
	for _, manifestPath := range manifestPaths {
		raw, err := bootstrapTemplateFS.ReadFile(manifestPath)
		if err != nil {
			return nil, fmt.Errorf("read manifest %s: %w", manifestPath, err)
		}

		var manifest bootstrapTemplateManifest
		if err := yaml.Unmarshal(raw, &manifest); err != nil {
			return nil, fmt.Errorf("parse manifest %s: %w", manifestPath, err)
		}
		if strings.TrimSpace(manifest.Version) != bootstrapTemplateManifestVersion {
			return nil, fmt.Errorf("manifest %s has unsupported version %q", manifestPath, manifest.Version)
		}

		templateID := strings.TrimSpace(manifest.ID)
		if templateID == "" {
			return nil, fmt.Errorf("manifest %s is missing id", manifestPath)
		}
		if _, exists := catalog[templateID]; exists {
			return nil, fmt.Errorf("duplicate bootstrap template id %q", templateID)
		}

		operations := make([]bootstrapTemplateOperation, 0, len(manifest.Operations))
		for i, operation := range manifest.Operations {
			resolved, err := resolveBootstrapTemplateOperation(manifestPath, templateID, i, operation)
			if err != nil {
				return nil, err
			}
			operations = append(operations, resolved)
		}
		if len(operations) == 0 {
			return nil, fmt.Errorf("template %q must define at least one operation", templateID)
		}

		catalog[templateID] = Template{
			ID:         templateID,
			Summary:    strings.TrimSpace(manifest.Summary),
			Operations: operations,
		}
	}

	return catalog, nil
}

func resolveBootstrapTemplateOperation(manifestPath, templateID string, index int, manifest bootstrapTemplateOperationManifest) (bootstrapTemplateOperation, error) {
	target := normalizeRelativePath(manifest.Target)
	if target == "" {
		return bootstrapTemplateOperation{}, fmt.Errorf("template %q operation %d has invalid target %q", templateID, index, manifest.Target)
	}

	switch strings.TrimSpace(manifest.Type) {
	case bootstrapTemplateOpCreateIfMissing, bootstrapTemplateOpCreateOrReplaceIfPristine, bootstrapTemplateOpReplaceIfPristine, bootstrapTemplateOpMergeJSON:
	default:
		return bootstrapTemplateOperation{}, fmt.Errorf("template %q operation %d has unsupported type %q", templateID, index, manifest.Type)
	}

	mode, err := parseBootstrapTemplateMode(manifest.Mode)
	if err != nil {
		return bootstrapTemplateOperation{}, fmt.Errorf("template %q operation %d: %w", templateID, index, err)
	}

	var source string
	if requiresBootstrapTemplateSource(strings.TrimSpace(manifest.Type)) {
		source, err = resolveBootstrapTemplateSource(manifestPath, manifest.Source)
		if err != nil {
			return bootstrapTemplateOperation{}, fmt.Errorf("template %q operation %d: %w", templateID, index, err)
		}
	}

	return bootstrapTemplateOperation{
		Type:     strings.TrimSpace(manifest.Type),
		Target:   target,
		Source:   source,
		Mode:     mode,
		Pristine: normalizeValues(manifest.Pristine),
	}, nil
}

func requiresBootstrapTemplateSource(opType string) bool {
	switch opType {
	case bootstrapTemplateOpCreateIfMissing, bootstrapTemplateOpCreateOrReplaceIfPristine, bootstrapTemplateOpReplaceIfPristine, bootstrapTemplateOpMergeJSON:
		return true
	default:
		return false
	}
}

func resolveBootstrapTemplateSource(manifestPath, source string) (string, error) {
	trimmed := strings.TrimSpace(source)
	if trimmed == "" {
		return "", fmt.Errorf("source is required")
	}
	if strings.HasPrefix(trimmed, "/") || strings.Contains(trimmed, "\\") {
		return "", fmt.Errorf("source %q is invalid", source)
	}

	cleaned := path.Clean(trimmed)
	if cleaned == "." || cleaned == ".." || strings.HasPrefix(cleaned, "../") {
		return "", fmt.Errorf("source %q is invalid", source)
	}

	resolved := path.Join(path.Dir(manifestPath), cleaned)
	info, err := fs.Stat(bootstrapTemplateFS, resolved)
	if err != nil {
		return "", fmt.Errorf("source %q not found", resolved)
	}
	if info.IsDir() {
		return "", fmt.Errorf("source %q must be a file", resolved)
	}
	return resolved, nil
}

func parseBootstrapTemplateMode(raw string) (os.FileMode, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return 0o644, nil
	}
	value, err := strconv.ParseUint(trimmed, 8, 32)
	if err != nil {
		return 0, fmt.Errorf("invalid mode %q", raw)
	}
	return os.FileMode(value), nil
}

func ApplyTemplates(projectRoot, projectID string, templates []Template) (ApplyResult, error) {
	if len(templates) == 0 {
		return ApplyResult{}, nil
	}

	ctx := bootstrapTemplateContext{
		ProjectID: strings.TrimSpace(projectID),
		RepoName:  filepath.Base(filepath.Clean(projectRoot)),
	}

	results := make([]v1.BootstrapTemplateResult, 0, len(templates))
	candidatePaths := make([]string, 0)
	for _, template := range templates {
		result, createdPaths, err := applyBootstrapTemplate(projectRoot, ctx, template)
		if err != nil {
			return ApplyResult{}, fmt.Errorf("apply template %s: %w", template.ID, err)
		}
		results = append(results, result)
		candidatePaths = append(candidatePaths, createdPaths...)
	}

	return ApplyResult{
		TemplateResults: results,
		CandidatePaths:  normalizeRelativePaths(candidatePaths),
	}, nil
}

func applyBootstrapTemplate(projectRoot string, ctx bootstrapTemplateContext, template Template) (v1.BootstrapTemplateResult, []string, error) {
	result := v1.BootstrapTemplateResult{TemplateID: template.ID}
	createdPaths := make([]string, 0)

	for _, operation := range template.Operations {
		created, err := applyBootstrapTemplateOperation(projectRoot, ctx, operation, &result)
		if err != nil {
			return v1.BootstrapTemplateResult{}, nil, err
		}
		if created {
			createdPaths = append(createdPaths, operation.Target)
		}
	}

	result.Created = normalizeRelativePaths(result.Created)
	result.Updated = normalizeRelativePaths(result.Updated)
	result.Unchanged = normalizeRelativePaths(result.Unchanged)
	sortBootstrapTemplateConflicts(result.SkippedConflicts)
	if len(result.Created) == 0 {
		result.Created = nil
	}
	if len(result.Updated) == 0 {
		result.Updated = nil
	}
	if len(result.Unchanged) == 0 {
		result.Unchanged = nil
	}
	if len(result.SkippedConflicts) == 0 {
		result.SkippedConflicts = nil
	}

	return result, normalizeRelativePaths(createdPaths), nil
}

func applyBootstrapTemplateOperation(projectRoot string, ctx bootstrapTemplateContext, operation bootstrapTemplateOperation, result *v1.BootstrapTemplateResult) (bool, error) {
	switch operation.Type {
	case bootstrapTemplateOpCreateIfMissing:
		return applyBootstrapTemplateCreateIfMissing(projectRoot, ctx, operation, result)
	case bootstrapTemplateOpCreateOrReplaceIfPristine:
		return applyBootstrapTemplateCreateOrReplaceIfPristine(projectRoot, ctx, operation, result)
	case bootstrapTemplateOpReplaceIfPristine:
		return applyBootstrapTemplateReplaceIfPristine(projectRoot, ctx, operation, result)
	case bootstrapTemplateOpMergeJSON:
		return applyBootstrapTemplateMergeJSON(projectRoot, ctx, operation, result)
	default:
		return false, fmt.Errorf("unsupported template operation %q", operation.Type)
	}
}

func applyBootstrapTemplateCreateIfMissing(projectRoot string, ctx bootstrapTemplateContext, operation bootstrapTemplateOperation, result *v1.BootstrapTemplateResult) (bool, error) {
	rendered, err := renderBootstrapTemplateAsset(operation.Source, ctx)
	if err != nil {
		return false, err
	}

	targetPath := filepath.Join(projectRoot, filepath.FromSlash(operation.Target))
	status, err := writeBootstrapTemplateFile(targetPath, rendered, operation.Mode)
	if err != nil {
		return false, err
	}

	switch status {
	case "created":
		result.Created = append(result.Created, operation.Target)
		return true, nil
	case "updated":
		result.Updated = append(result.Updated, operation.Target)
		return false, nil
	case "unchanged":
		result.Unchanged = append(result.Unchanged, operation.Target)
		return false, nil
	case "conflict":
		result.SkippedConflicts = append(result.SkippedConflicts, v1.BootstrapTemplateConflict{
			Path:   operation.Target,
			Reason: "existing file differs",
		})
		return false, nil
	default:
		return false, fmt.Errorf("unexpected file status %q", status)
	}
}

func applyBootstrapTemplateCreateOrReplaceIfPristine(projectRoot string, ctx bootstrapTemplateContext, operation bootstrapTemplateOperation, result *v1.BootstrapTemplateResult) (bool, error) {
	rendered, err := renderBootstrapTemplateAsset(operation.Source, ctx)
	if err != nil {
		return false, err
	}

	targetPath := filepath.Join(projectRoot, filepath.FromSlash(operation.Target))
	existing, err := os.ReadFile(targetPath)
	switch {
	case errors.Is(err, os.ErrNotExist):
		if err := writeBootstrapTemplateNewFile(targetPath, rendered, operation.Mode); err != nil {
			return false, err
		}
		result.Created = append(result.Created, operation.Target)
		return true, nil
	case err != nil:
		return false, err
	}

	if bytes.Equal(existing, rendered) {
		if modeUpdated, err := ensureBootstrapTemplateMode(targetPath, operation.Mode); err != nil {
			return false, err
		} else if modeUpdated {
			result.Updated = append(result.Updated, operation.Target)
		} else {
			result.Unchanged = append(result.Unchanged, operation.Target)
		}
		return false, nil
	}

	pristineMatch, err := matchesBootstrapPristineContent(existing, operation.Pristine)
	if err != nil {
		return false, err
	}
	if !pristineMatch {
		result.SkippedConflicts = append(result.SkippedConflicts, v1.BootstrapTemplateConflict{
			Path:   operation.Target,
			Reason: "existing file differs from bootstrap scaffold",
		})
		return false, nil
	}

	if err := os.WriteFile(targetPath, rendered, operation.Mode); err != nil {
		return false, err
	}
	result.Updated = append(result.Updated, operation.Target)
	return false, nil
}

func applyBootstrapTemplateReplaceIfPristine(projectRoot string, ctx bootstrapTemplateContext, operation bootstrapTemplateOperation, result *v1.BootstrapTemplateResult) (bool, error) {
	targetPath := filepath.Join(projectRoot, filepath.FromSlash(operation.Target))
	existing, err := os.ReadFile(targetPath)
	switch {
	case errors.Is(err, os.ErrNotExist):
		return false, nil
	case err != nil:
		return false, err
	}

	rendered, err := renderBootstrapTemplateAsset(operation.Source, ctx)
	if err != nil {
		return false, err
	}
	if bytes.Equal(existing, rendered) {
		if modeUpdated, err := ensureBootstrapTemplateMode(targetPath, operation.Mode); err != nil {
			return false, err
		} else if modeUpdated {
			result.Updated = append(result.Updated, operation.Target)
		} else {
			result.Unchanged = append(result.Unchanged, operation.Target)
		}
		return false, nil
	}

	pristineMatch, err := matchesBootstrapPristineContent(existing, operation.Pristine)
	if err != nil {
		return false, err
	}
	if !pristineMatch {
		result.SkippedConflicts = append(result.SkippedConflicts, v1.BootstrapTemplateConflict{
			Path:   operation.Target,
			Reason: "existing file differs from bootstrap scaffold",
		})
		return false, nil
	}

	if err := os.WriteFile(targetPath, rendered, operation.Mode); err != nil {
		return false, err
	}
	result.Updated = append(result.Updated, operation.Target)
	return false, nil
}

func applyBootstrapTemplateMergeJSON(projectRoot string, ctx bootstrapTemplateContext, operation bootstrapTemplateOperation, result *v1.BootstrapTemplateResult) (bool, error) {
	rendered, err := renderBootstrapTemplateAsset(operation.Source, ctx)
	if err != nil {
		return false, err
	}

	var sourceValue any
	if err := json.Unmarshal(rendered, &sourceValue); err != nil {
		return false, fmt.Errorf("parse json asset %s: %w", operation.Source, err)
	}

	targetPath := filepath.Join(projectRoot, filepath.FromSlash(operation.Target))
	existing, err := os.ReadFile(targetPath)
	switch {
	case errors.Is(err, os.ErrNotExist):
		blob, err := marshalBootstrapTemplateJSON(sourceValue)
		if err != nil {
			return false, err
		}
		if err := writeBootstrapTemplateNewFile(targetPath, blob, operation.Mode); err != nil {
			return false, err
		}
		result.Created = append(result.Created, operation.Target)
		return true, nil
	case err != nil:
		return false, err
	}

	var targetValue any
	if err := json.Unmarshal(existing, &targetValue); err != nil {
		result.SkippedConflicts = append(result.SkippedConflicts, v1.BootstrapTemplateConflict{
			Path:   operation.Target,
			Reason: "existing file is not valid JSON",
		})
		return false, nil
	}

	mergedValue, changed, err := mergeBootstrapJSONValues(targetValue, sourceValue)
	if err != nil {
		result.SkippedConflicts = append(result.SkippedConflicts, v1.BootstrapTemplateConflict{
			Path:   operation.Target,
			Reason: err.Error(),
		})
		return false, nil
	}
	if !changed {
		if modeUpdated, err := ensureBootstrapTemplateMode(targetPath, operation.Mode); err != nil {
			return false, err
		} else if modeUpdated {
			result.Updated = append(result.Updated, operation.Target)
		} else {
			result.Unchanged = append(result.Unchanged, operation.Target)
		}
		return false, nil
	}

	blob, err := marshalBootstrapTemplateJSON(mergedValue)
	if err != nil {
		return false, err
	}
	if err := os.WriteFile(targetPath, blob, operation.Mode); err != nil {
		return false, err
	}
	result.Updated = append(result.Updated, operation.Target)
	return false, nil
}

func renderBootstrapTemplateAsset(source string, ctx bootstrapTemplateContext) ([]byte, error) {
	raw, err := bootstrapTemplateFS.ReadFile(source)
	if err != nil {
		return nil, fmt.Errorf("read asset %s: %w", source, err)
	}

	replacer := strings.NewReplacer(
		"{{project_id}}", ctx.ProjectID,
		"{{repo_name}}", ctx.RepoName,
	)
	return []byte(replacer.Replace(string(raw))), nil
}

func writeBootstrapTemplateFile(targetPath string, content []byte, mode os.FileMode) (string, error) {
	existing, err := os.ReadFile(targetPath)
	switch {
	case errors.Is(err, os.ErrNotExist):
		if err := writeBootstrapTemplateNewFile(targetPath, content, mode); err != nil {
			return "", err
		}
		return "created", nil
	case err != nil:
		return "", err
	}

	if !bytes.Equal(existing, content) {
		return "conflict", nil
	}
	modeUpdated, err := ensureBootstrapTemplateMode(targetPath, mode)
	if err != nil {
		return "", err
	}
	if modeUpdated {
		return "updated", nil
	}
	return "unchanged", nil
}

func writeBootstrapTemplateNewFile(targetPath string, content []byte, mode os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
		return err
	}
	file, err := os.OpenFile(targetPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, mode)
	if err != nil {
		return err
	}
	defer file.Close()

	if _, err := file.Write(content); err != nil {
		return err
	}
	return nil
}

func ensureBootstrapTemplateMode(targetPath string, mode os.FileMode) (bool, error) {
	info, err := os.Stat(targetPath)
	if err != nil {
		return false, err
	}
	current := info.Mode().Perm()
	desired := mode.Perm()
	if current == desired {
		return false, nil
	}
	if err := os.Chmod(targetPath, desired); err != nil {
		return false, err
	}
	return true, nil
}

func matchesBootstrapPristineContent(existing []byte, pristineIDs []string) (bool, error) {
	for _, pristineID := range pristineIDs {
		pristine, ok := bootstrapPristineContent(pristineID)
		if !ok {
			return false, fmt.Errorf("unknown pristine content %q", pristineID)
		}
		if bytes.Equal(existing, pristine) {
			return true, nil
		}
	}
	return false, nil
}

func bootstrapPristineContent(pristineID string) ([]byte, bool) {
	switch strings.TrimSpace(pristineID) {
	case "blank_rules_v1":
		return []byte(BlankRulesContents), true
	case "blank_tests_v1":
		return []byte(BlankTestsContents), true
	case "blank_workflows_v1":
		return []byte(BlankWorkflowsContents), true
	case "starter_contract_agents_v1":
		return bootstrapPristineEmbeddedContent("bootstrap_templates/starter-contract/files/AGENTS.md")
	case "starter_contract_claude_v1":
		return bootstrapPristineEmbeddedContent("bootstrap_templates/starter-contract/files/CLAUDE.md")
	case "starter_contract_rules_v1":
		return bootstrapPristineEmbeddedContent("bootstrap_templates/starter-contract/files/.acm/acm-rules.yaml")
	case "verify_generic_tests_v1":
		return bootstrapPristineEmbeddedContent("bootstrap_templates/verify-generic/files/.acm/acm-tests.yaml")
	default:
		return nil, false
	}
}

func bootstrapPristineEmbeddedContent(source string) ([]byte, bool) {
	raw, err := bootstrapTemplateFS.ReadFile(source)
	if err != nil {
		return nil, false
	}
	return raw, true
}

func marshalBootstrapTemplateJSON(value any) ([]byte, error) {
	blob, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(blob, '\n'), nil
}

func mergeBootstrapJSONValues(target, source any) (any, bool, error) {
	switch typedSource := source.(type) {
	case map[string]any:
		typedTarget, ok := target.(map[string]any)
		if !ok {
			return nil, false, fmt.Errorf("existing JSON structure conflicts with template")
		}
		merged := cloneBootstrapJSONValue(typedTarget).(map[string]any)
		changed := false
		for key, sourceValue := range typedSource {
			targetValue, exists := merged[key]
			if !exists {
				merged[key] = cloneBootstrapJSONValue(sourceValue)
				changed = true
				continue
			}
			mergedValue, childChanged, err := mergeBootstrapJSONValues(targetValue, sourceValue)
			if err != nil {
				return nil, false, err
			}
			if childChanged {
				merged[key] = mergedValue
				changed = true
			}
		}
		return merged, changed, nil
	case []any:
		typedTarget, ok := target.([]any)
		if !ok {
			return nil, false, fmt.Errorf("existing JSON structure conflicts with template")
		}
		merged := cloneBootstrapJSONValue(typedTarget).([]any)
		changed := false
		for _, sourceValue := range typedSource {
			if bootstrapJSONArrayContains(merged, sourceValue) {
				continue
			}
			merged = append(merged, cloneBootstrapJSONValue(sourceValue))
			changed = true
		}
		return merged, changed, nil
	default:
		if reflect.DeepEqual(target, source) {
			return target, false, nil
		}
		return nil, false, fmt.Errorf("existing JSON structure conflicts with template")
	}
}

func bootstrapJSONArrayContains(values []any, candidate any) bool {
	for _, value := range values {
		if reflect.DeepEqual(value, candidate) {
			return true
		}
	}
	return false
}

func cloneBootstrapJSONValue(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		cloned := make(map[string]any, len(typed))
		for key, child := range typed {
			cloned[key] = cloneBootstrapJSONValue(child)
		}
		return cloned
	case []any:
		cloned := make([]any, 0, len(typed))
		for _, child := range typed {
			cloned = append(cloned, cloneBootstrapJSONValue(child))
		}
		return cloned
	default:
		return typed
	}
}

func sortBootstrapTemplateConflicts(conflicts []v1.BootstrapTemplateConflict) {
	sort.Slice(conflicts, func(i, j int) bool {
		if conflicts[i].Path == conflicts[j].Path {
			return conflicts[i].Reason < conflicts[j].Reason
		}
		return conflicts[i].Path < conflicts[j].Path
	})
}

func normalizeValues(values []string) []string {
	if len(values) == 0 {
		return nil
	}

	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))
	for _, raw := range values {
		trimmed := strings.TrimSpace(raw)
		if trimmed == "" {
			continue
		}
		if _, ok := seen[trimmed]; ok {
			continue
		}
		seen[trimmed] = struct{}{}
		out = append(out, trimmed)
	}
	if len(out) == 0 {
		return nil
	}
	sort.Strings(out)
	return out
}

func normalizeRelativePaths(paths []string) []string {
	if len(paths) == 0 {
		return nil
	}

	seen := make(map[string]struct{}, len(paths))
	out := make([]string, 0, len(paths))
	for _, raw := range paths {
		normalized := normalizeRelativePath(raw)
		if normalized == "" {
			continue
		}
		if _, ok := seen[normalized]; ok {
			continue
		}
		seen[normalized] = struct{}{}
		out = append(out, normalized)
	}
	sort.Strings(out)
	return out
}

func normalizeRelativePath(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return ""
	}
	normalizedSlashes := strings.ReplaceAll(trimmed, "\\", "/")
	if strings.HasPrefix(normalizedSlashes, "/") || isWindowsAbsolutePath(normalizedSlashes) {
		return ""
	}
	cleaned := path.Clean(normalizedSlashes)
	if cleaned == "." || cleaned == ".." || strings.HasPrefix(cleaned, "../") {
		return ""
	}
	return cleaned
}

func isWindowsAbsolutePath(value string) bool {
	return len(value) >= 3 && ((value[0] >= 'A' && value[0] <= 'Z') || (value[0] >= 'a' && value[0] <= 'z')) && value[1] == ':' && value[2] == '/'
}
