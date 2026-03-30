package api

import (
	"errors"
	"fmt"
	"path/filepath"
	"slices"
	"strings"
)

// Sentinel errors returned by path validation.
var (
	errPathTraversal = errors.New("path traversal not allowed")
	errPathEscapes   = errors.New("path escapes output directory")
)

// safePath joins rawPath with outputPath and validates that the result stays
// within outputPath. For glob paths (containing *), the static prefix before
// the first wildcard is validated.
func (a *API) safePath(rawPath string) (string, error) {
	// Strip leading slashes so filepath.Join does not treat rawPath as absolute.
	rawPath = strings.TrimLeft(rawPath, "/")

	if slices.Contains(strings.Split(rawPath, "/"), "..") {
		return "", errPathTraversal
	}

	joined := filepath.Join(a.outputPath, rawPath)
	prefix, _, hasWildcard := strings.Cut(joined, "*")

	if !hasWildcard {
		clean := filepath.Clean(joined)

		if !a.isUnder(clean) {
			return "", errPathEscapes
		}

		return clean, nil
	}

	// For glob paths, verify the static prefix before the first wildcard.
	if !a.isUnder(filepath.Clean(prefix)) {
		return "", errPathEscapes
	}

	return joined, nil
}

// isUnder reports whether path equals outputPath or is directly beneath it.
func (a *API) isUnder(path string) bool {
	return path == a.outputPath ||
		strings.HasPrefix(path, a.outputPath+string(filepath.Separator))
}

// globSafe expands pattern with filepath.Glob and returns only results that
// are confirmed to be under outputPath.
func (a *API) globSafe(pattern string) ([]string, error) {
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return nil, fmt.Errorf("glob pattern: %w", err)
	}

	result := make([]string, 0, len(matches))

	for _, match := range matches {
		if a.isUnder(filepath.Clean(match)) {
			result = append(result, match)
		}
	}

	return result, nil
}
