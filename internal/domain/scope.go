package domain

import (
	"fmt"
	"path"
	"path/filepath"
	"strings"
)

func NormalizeRepoPath(value string) (string, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return "", fmt.Errorf("path is required")
	}
	if filepath.IsAbs(trimmed) {
		return "", fmt.Errorf("path must be repo-relative: %s", value)
	}

	normalized := filepath.ToSlash(trimmed)
	cleaned := path.Clean(normalized)
	if cleaned == "." || cleaned == "" {
		return "", fmt.Errorf("path is required")
	}
	if cleaned == ".." || strings.HasPrefix(cleaned, "../") || strings.HasPrefix(cleaned, "/") {
		return "", fmt.Errorf("path must not escape the repository: %s", value)
	}

	return cleaned, nil
}

func NormalizeRepoDir(value string) (string, error) {
	normalized, err := NormalizeRepoPath(value)
	if err != nil {
		return "", err
	}
	return strings.TrimSuffix(normalized, "/") + "/", nil
}

func ValidateStoredRepoFilePath(value string) error {
	normalized, err := NormalizeRepoPath(value)
	if err != nil {
		return err
	}
	if strings.HasSuffix(filepath.ToSlash(strings.TrimSpace(value)), "/") {
		return fmt.Errorf("path must point to a file")
	}
	if normalized != value {
		return fmt.Errorf("path must be stored as a normalized repo-relative file path")
	}
	return nil
}

func ValidateStoredRepoDirPath(value string) error {
	normalized, err := NormalizeRepoDir(value)
	if err != nil {
		return err
	}
	if normalized != value {
		return fmt.Errorf("path must be stored as a normalized repo-relative directory ending in /")
	}
	return nil
}

func ScopeMatchesPath(scopePrefix, candidate string) bool {
	normalizedScope, err := NormalizeRepoDir(scopePrefix)
	if err != nil {
		return false
	}
	normalizedCandidate, err := NormalizeRepoPath(candidate)
	if err != nil {
		return false
	}

	return normalizedCandidate == strings.TrimSuffix(normalizedScope, "/") || strings.HasPrefix(normalizedCandidate, normalizedScope)
}

func ScopesOverlap(a, b string) bool {
	normalizedA, errA := NormalizeRepoDir(a)
	normalizedB, errB := NormalizeRepoDir(b)
	if errA != nil || errB != nil {
		return false
	}
	return strings.HasPrefix(normalizedA, normalizedB) || strings.HasPrefix(normalizedB, normalizedA)
}
