package infrastructure

import (
	"bufio"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

var fileLineRef = regexp.MustCompile(`([a-zA-Z0-9_\-./]+\.[a-zA-Z0-9]+):(\d+)`)

// FileLineReference represents a file:line reference found in spec content.
type FileLineReference struct {
	Path string
	Line int
	Raw  string
}

// FileIndex stores basename lookup candidates for a project tree.
type FileIndex struct {
	ProjectRoot string
	Basenames   map[string][]string
}

// ExtractFileReferences extracts file:line references from text content.
func ExtractFileReferences(content string) []FileLineReference {
	matches := fileLineRef.FindAllStringSubmatch(content, -1)
	var refs []FileLineReference
	for _, m := range matches {
		lineNum, err := strconv.Atoi(m[2])
		if err != nil {
			continue
		}
		refs = append(refs, FileLineReference{
			Path: m[1],
			Line: lineNum,
			Raw:  m[0],
		})
	}
	return refs
}

// BuildFileIndex indexes repo-relative file paths by basename.
func BuildFileIndex(projectRoot string) (*FileIndex, error) {
	index := &FileIndex{
		ProjectRoot: projectRoot,
		Basenames:   make(map[string][]string),
	}
	err := filepath.WalkDir(projectRoot, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			switch d.Name() {
			case ".git", "node_modules", ".next", "dist", "build", ".turbo":
				return filepath.SkipDir
			}
			return nil
		}
		rel, err := filepath.Rel(projectRoot, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		base := filepath.Base(rel)
		index.Basenames[base] = append(index.Basenames[base], rel)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return index, nil
}

// CheckFileReference verifies a file exists and has at least the given number of lines.
func CheckFileReference(basePath string, ref FileLineReference, index *FileIndex, hints []string) error {
	fullPath, displayPath := resolveFileReferencePath(basePath, ref.Path, index, normalizeHintPaths(hints))
	f, err := os.Open(fullPath)
	if err != nil {
		return fmt.Errorf("file not found: %s", ref.Raw)
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	lineCount := 0
	for scanner.Scan() {
		lineCount++
		if lineCount >= ref.Line {
			return nil
		}
	}

	return fmt.Errorf("file %s has only %d lines, reference points to line %d", displayPath, lineCount, ref.Line)
}

// FindProjectRoot walks up from specPath to find the project root.
// It prefers a directory containing ".specs" for legacy layouts, then falls back
// to the git worktree root so nested specs directories still resolve correctly.
// Falls back to the spec file's directory if neither marker is found.
func FindProjectRoot(specPath string) (string, error) {
	absPath, err := filepath.Abs(specPath)
	if err != nil {
		return "", fmt.Errorf("cannot resolve absolute path: %w", err)
	}
	dir := filepath.Dir(absPath)
	for {
		if filepath.Base(dir) == ".specs" {
			return filepath.Dir(dir), nil
		}
		candidate := filepath.Join(dir, ".specs")
		if info, err := os.Stat(candidate); err == nil && info.IsDir() {
			return dir, nil
		}
		gitDir := filepath.Join(dir, ".git")
		if info, err := os.Stat(gitDir); err == nil && info.IsDir() {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return filepath.Dir(absPath), nil
		}
		dir = parent
	}
}

// DeduplicateRefs returns unique file references by Path+Line.
func DeduplicateRefs(refs []FileLineReference) []FileLineReference {
	seen := make(map[string]bool)
	var unique []FileLineReference
	for _, ref := range refs {
		key := fmt.Sprintf("%s:%d", ref.Path, ref.Line)
		if !seen[key] {
			seen[key] = true
			unique = append(unique, ref)
		}
	}
	return unique
}

func resolveFileReferencePath(projectRoot, rawPath string, index *FileIndex, hints []string) (string, string) {
	if strings.HasPrefix(rawPath, "/") {
		return rawPath, rawPath
	}

	normalized := filepath.ToSlash(strings.TrimSpace(rawPath))
	explicit := filepath.Join(projectRoot, filepath.FromSlash(normalized))
	isPartialPath := strings.Contains(normalized, "/")

	if index == nil {
		return explicit, normalized
	}

	var candidates []string
	if isPartialPath {
		if _, err := os.Stat(explicit); err == nil {
			return explicit, normalized
		}
		candidates = suffixCandidates(index, normalized)
	} else {
		candidates = index.Basenames[filepath.Base(normalized)]
		if len(candidates) == 0 {
			candidates = dynamicBasenameCandidates(index, normalized)
		}
	}
	if len(candidates) == 0 {
		if _, err := os.Stat(explicit); err == nil {
			return explicit, normalized
		}
		return explicit, normalized
	}
	if len(candidates) == 1 {
		candidate := candidates[0]
		return filepath.Join(projectRoot, filepath.FromSlash(candidate)), candidate
	}

	bestPath := ""
	bestScore := -1
	tied := false
	for _, candidate := range candidates {
		score := scoreCandidatePath(candidate, hints)
		if score > bestScore {
			bestPath = candidate
			bestScore = score
			tied = false
			continue
		}
		if score == bestScore {
			tied = true
		}
	}
	if bestScore > 0 && !tied {
		return filepath.Join(projectRoot, filepath.FromSlash(bestPath)), bestPath
	}

	if _, err := os.Stat(explicit); err == nil {
		return explicit, normalized
	}
	return explicit, normalized
}

func suffixCandidates(index *FileIndex, suffix string) []string {
	var candidates []string
	normalizedSuffix := filepath.ToSlash(strings.TrimPrefix(suffix, "./"))
	for _, paths := range index.Basenames {
		for _, candidate := range paths {
			if candidate == normalizedSuffix || strings.HasSuffix(candidate, "/"+normalizedSuffix) {
				candidates = append(candidates, candidate)
			}
		}
	}
	return candidates
}

func dynamicBasenameCandidates(index *FileIndex, basename string) []string {
	var candidates []string
	for key, paths := range index.Basenames {
		if strings.TrimPrefix(key, "$") == basename {
			candidates = append(candidates, paths...)
		}
	}
	return candidates
}

func normalizeHintPaths(hints []string) []string {
	normalized := make([]string, 0, len(hints))
	for _, hint := range hints {
		h := strings.TrimSpace(hint)
		if h == "" {
			continue
		}
		if idx := strings.LastIndex(h, ":"); idx > 0 {
			if _, err := strconv.Atoi(h[idx+1:]); err == nil {
				h = h[:idx]
			}
		}
		normalized = append(normalized, filepath.ToSlash(h))
	}
	return normalized
}

func scoreCandidatePath(candidate string, hints []string) int {
	best := 0
	for _, hint := range hints {
		if hint == candidate {
			return 1000
		}
		score := sharedSegmentPrefixCount(candidate, hint)
		if score > best {
			best = score
		}
	}
	return best
}

func sharedSegmentPrefixCount(a, b string) int {
	aParts := strings.Split(filepath.ToSlash(a), "/")
	bParts := strings.Split(filepath.ToSlash(b), "/")
	limit := len(aParts)
	if len(bParts) < limit {
		limit = len(bParts)
	}
	score := 0
	for i := 0; i < limit; i++ {
		if aParts[i] != bParts[i] {
			break
		}
		score++
	}
	return score
}
