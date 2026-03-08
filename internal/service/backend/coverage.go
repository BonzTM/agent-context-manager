package backend

import (
	"context"
	"math"
	"path"
	"sort"
	"strings"

	"github.com/bonztm/agent-context-manager/internal/contracts/v1"
	"github.com/bonztm/agent-context-manager/internal/core"
)

func (s *Service) Coverage(ctx context.Context, payload v1.CoveragePayload) (v1.CoverageResult, *core.APIError) {
	if s == nil || s.repo == nil {
		return v1.CoverageResult{}, core.NewError("INTERNAL_ERROR", "service repository is not configured", nil)
	}

	projectRoot := s.effectiveProjectRoot(payload.ProjectRoot)
	paths, err := s.collectCoveragePaths(ctx, projectRoot)
	if err != nil {
		return v1.CoverageResult{}, coverageInternalError("collect_project_paths", err)
	}

	inventory, err := s.repo.ListPointerInventory(ctx, strings.TrimSpace(payload.ProjectID))
	if err != nil {
		return v1.CoverageResult{}, coverageInternalError("list_pointer_inventory", err)
	}

	pointerByPath := make(map[string]core.PointerInventory, len(inventory))
	for _, item := range inventory {
		normalizedPath := normalizeCompletionPath(item.Path)
		if normalizedPath == "" {
			continue
		}
		current, exists := pointerByPath[normalizedPath]
		if !exists {
			pointerByPath[normalizedPath] = core.PointerInventory{Path: normalizedPath, IsStale: item.IsStale}
			continue
		}
		current.IsStale = current.IsStale || item.IsStale
		pointerByPath[normalizedPath] = current
	}

	indexedCount := 0
	unindexed := make([]string, 0)
	for _, filePath := range paths {
		if _, ok := pointerByPath[filePath]; ok {
			indexedCount++
			continue
		}
		unindexed = append(unindexed, filePath)
	}

	stale := make([]string, 0)
	for filePath, item := range pointerByPath {
		if !item.IsStale {
			continue
		}
		stale = append(stale, filePath)
	}
	sort.Strings(stale)

	zeroCoverageDirs := zeroCoverageDirectories(paths, pointerByPath)

	totalFiles := len(paths)
	coveragePercent := 100.0
	if totalFiles > 0 {
		coveragePercent = math.Round((float64(indexedCount)/float64(totalFiles)*100.0)*100.0) / 100.0
	}

	return v1.CoverageResult{
		Summary: v1.CoverageSummary{
			TotalFiles:      totalFiles,
			IndexedFiles:    indexedCount,
			UnindexedFiles:  len(unindexed),
			StaleFiles:      len(stale),
			CoveragePercent: coveragePercent,
		},
		UnindexedPaths:   normalizeValues(unindexed),
		StalePaths:       normalizeValues(stale),
		ZeroCoverageDirs: zeroCoverageDirs,
	}, nil
}

func (s *Service) collectCoveragePaths(ctx context.Context, projectRoot string) ([]string, error) {
	gitOutput, err := s.runGit(ctx, projectRoot, "ls-files", "--cached", "--others", "--exclude-standard")
	if err == nil {
		return parseBootstrapGitPaths(gitOutput), nil
	}

	paths, _, walkErr := collectBootstrapPathsFromWalk(ctx, projectRoot)
	if walkErr != nil {
		return nil, walkErr
	}
	return filterBootstrapPaths(paths, nil), nil
}

func zeroCoverageDirectories(paths []string, pointerByPath map[string]core.PointerInventory) []string {
	if len(paths) == 0 {
		return nil
	}

	dirTotal := make(map[string]int)
	dirCovered := make(map[string]int)
	for _, filePath := range paths {
		dir := path.Dir(filePath)
		if dir == "." || dir == "" {
			continue
		}
		dirTotal[dir]++
		if _, ok := pointerByPath[filePath]; ok {
			dirCovered[dir]++
		}
	}

	out := make([]string, 0)
	for dir, total := range dirTotal {
		if total <= 0 {
			continue
		}
		if dirCovered[dir] > 0 {
			continue
		}
		out = append(out, dir)
	}
	sort.Strings(out)
	return out
}

func coverageInternalError(operation string, err error) *core.APIError {
	return core.NewError(
		"INTERNAL_ERROR",
		"failed to compute coverage report",
		map[string]any{
			"operation": operation,
			"error":     err.Error(),
		},
	)
}
