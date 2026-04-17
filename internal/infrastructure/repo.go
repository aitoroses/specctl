package infrastructure

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

func FindRepoRoot(start string) (string, error) {
	current, err := filepath.Abs(start)
	if err != nil {
		return "", fmt.Errorf("resolving working directory: %w", err)
	}

	for {
		specsDir := filepath.Join(current, ".specs")
		info, err := os.Stat(specsDir)
		if err == nil && info.IsDir() {
			return current, nil
		}
		if err != nil && !os.IsNotExist(err) {
			return "", fmt.Errorf("checking %s: %w", specsDir, err)
		}

		parent := filepath.Dir(current)
		if parent == current {
			return "", fmt.Errorf("could not find repo root containing .specs from %s", start)
		}
		current = parent
	}
}

func FindAllTrackingFiles(specsDir string) ([]string, error) {
	entries, err := os.ReadDir(specsDir)
	if err != nil {
		return nil, fmt.Errorf("reading specs directory: %w", err)
	}

	paths := make([]string, 0)
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		charterDir := filepath.Join(specsDir, entry.Name())
		files, err := os.ReadDir(charterDir)
		if err != nil {
			return nil, fmt.Errorf("reading charter directory %s: %w", charterDir, err)
		}
		for _, file := range files {
			if file.IsDir() || filepath.Ext(file.Name()) != ".yaml" || file.Name() == "CHARTER.yaml" {
				continue
			}
			paths = append(paths, filepath.Join(charterDir, file.Name()))
		}
	}

	sort.Strings(paths)
	return paths, nil
}

func RelativeRepoPath(repoRoot, path string) string {
	rel, err := filepath.Rel(repoRoot, path)
	if err != nil {
		return filepath.ToSlash(path)
	}
	return filepath.ToSlash(rel)
}

func RelativeTrackingPath(repoRoot, charter, slug string) string {
	return filepath.ToSlash(filepath.Join(".specs", charter, slug+".yaml"))
}

func RelativeCharterPath(charter string) string {
	return filepath.ToSlash(filepath.Join(".specs", charter, "CHARTER.yaml"))
}

func TempSiblingPattern(path string) string {
	base := filepath.Base(path)
	return "." + strings.TrimSuffix(base, filepath.Ext(base)) + ".*.tmp"
}
