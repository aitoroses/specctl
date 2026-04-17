package infrastructure

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

func ResolveGitRevision(repoRoot, ref string) (string, error) {
	cmd := exec.Command("git", "-C", repoRoot, "rev-parse", "--verify", ref+"^{commit}")
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("resolve checkpoint %q: %s", ref, strings.TrimSpace(stderr.String()))
	}
	return strings.TrimSpace(stdout.String()), nil
}

func ReadGitFile(repoRoot, ref, relativePath string) ([]byte, error) {
	gitPath := filepath.ToSlash(relativePath)
	// git show <ref>:<path> resolves <path> relative to the git toplevel,
	// not the -C directory. Compute the prefix from repoRoot to toplevel.
	prefix, err := gitSubdirPrefix(repoRoot)
	if err == nil && prefix != "" {
		gitPath = prefix + gitPath
	}
	cmd := exec.Command("git", "-C", repoRoot, "show", ref+":"+gitPath)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("read %s at %s: %s", gitPath, ref, strings.TrimSpace(stderr.String()))
	}
	return stdout.Bytes(), nil
}

func gitSubdirPrefix(repoRoot string) (string, error) {
	absRoot, err := filepath.Abs(repoRoot)
	if err != nil {
		return "", err
	}
	if resolved, err := filepath.EvalSymlinks(absRoot); err == nil {
		absRoot = resolved
	}
	cmd := exec.Command("git", "-C", repoRoot, "rev-parse", "--show-toplevel")
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	if err := cmd.Run(); err != nil {
		return "", err
	}
	toplevel := strings.TrimSpace(stdout.String())
	if resolved, err := filepath.EvalSymlinks(toplevel); err == nil {
		toplevel = resolved
	}
	rel, err := filepath.Rel(toplevel, absRoot)
	if err != nil {
		return "", err
	}
	rel = filepath.ToSlash(rel)
	if rel == "." || rel == "" {
		return "", nil
	}
	return rel + "/", nil
}

type ScopeFile struct {
	Path    string
	Tracked bool
}

func EnsureGitMetadata(repoRoot string) error {
	cmd := exec.Command("git", "-C", repoRoot, "rev-parse", "--show-toplevel")
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("inspect git metadata: %s", strings.TrimSpace(stderr.String()))
	}
	return nil
}

func GitDiffNameOnly(repoRoot, fromRef, toRef string, paths []string) ([]string, error) {
	args := []string{"diff", "--name-only", fromRef + ".." + toRef}
	if len(paths) > 0 {
		args = append(args, "--")
		args = append(args, paths...)
	}
	return gitPathList(repoRoot, args...)
}

func GitStatusPorcelain(repoRoot string, paths []string) ([]string, error) {
	args := []string{"status", "--porcelain", "--untracked-files=all"}
	if len(paths) > 0 {
		args = append(args, "--")
		args = append(args, paths...)
	}

	cmd := exec.Command("git", append([]string{"-C", repoRoot}, args...)...)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("git %s: %s", strings.Join(args, " "), strings.TrimSpace(stderr.String()))
	}

	seen := map[string]struct{}{}
	pathsOut := make([]string, 0)
	for _, line := range strings.Split(stdout.String(), "\n") {
		line = strings.TrimRight(line, "\r")
		if line == "" {
			continue
		}
		path := parsePorcelainPath(line)
		if path == "" {
			continue
		}
		path = filepath.ToSlash(path)
		if _, exists := seen[path]; exists {
			continue
		}
		seen[path] = struct{}{}
		pathsOut = append(pathsOut, path)
	}
	sort.Strings(pathsOut)
	return pathsOut, nil
}

func GitLogCommitsForPath(repoRoot, relativePath string) ([]string, error) {
	return gitPathList(repoRoot, "log", "--format=%H", "--reverse", "--", filepath.ToSlash(relativePath))
}

func ListScopeFiles(repoRoot string, scopes []string) ([]ScopeFile, error) {
	if err := EnsureGitMetadata(repoRoot); err != nil {
		return nil, err
	}

	trackedArgs := append([]string{"ls-files", "--cached", "--"}, scopes...)
	tracked, err := gitPathSet(repoRoot, trackedArgs...)
	if err != nil {
		return nil, err
	}
	ignoredArgs := append([]string{"ls-files", "--others", "--ignored", "--exclude-standard", "--"}, scopes...)
	ignored, err := gitPathSet(repoRoot, ignoredArgs...)
	if err != nil {
		return nil, err
	}

	seen := make(map[string]struct{})
	files := make([]ScopeFile, 0)
	for _, scope := range scopes {
		absoluteScope := filepath.Join(repoRoot, filepath.FromSlash(strings.TrimSuffix(scope, "/")))
		info, err := os.Stat(absoluteScope)
		if err != nil || !info.IsDir() {
			continue
		}
		walkErr := filepath.Walk(absoluteScope, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if info.IsDir() {
				return nil
			}
			rel := RelativeRepoPath(repoRoot, path)
			if _, skip := ignored[rel]; skip {
				return nil
			}
			if _, exists := seen[rel]; exists {
				return nil
			}
			seen[rel] = struct{}{}
			_, isTracked := tracked[rel]
			files = append(files, ScopeFile{
				Path:    rel,
				Tracked: isTracked,
			})
			return nil
		})
		if walkErr != nil {
			return nil, fmt.Errorf("walk scope %s: %w", scope, walkErr)
		}
	}

	sort.Slice(files, func(i, j int) bool {
		return files[i].Path < files[j].Path
	})
	return files, nil
}

func LatestGitCommitTimestamp(repoRoot, relativePath string) (*time.Time, error) {
	cmd := exec.Command("git", "-C", repoRoot, "log", "-1", "--format=%cI", "--", filepath.ToSlash(relativePath))
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("read git history for %s: %s", filepath.ToSlash(relativePath), strings.TrimSpace(stderr.String()))
	}

	value := strings.TrimSpace(stdout.String())
	if value == "" {
		return nil, nil
	}

	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return nil, fmt.Errorf("parse git timestamp for %s: %w", filepath.ToSlash(relativePath), err)
	}
	utc := parsed.UTC()
	return &utc, nil
}

func gitPathSet(repoRoot string, args ...string) (map[string]struct{}, error) {
	pathsList, err := gitPathList(repoRoot, args...)
	if err != nil {
		return nil, err
	}
	paths := make(map[string]struct{}, len(pathsList))
	for _, path := range pathsList {
		paths[path] = struct{}{}
	}
	return paths, nil
}

func gitPathList(repoRoot string, args ...string) ([]string, error) {
	cmd := exec.Command("git", append([]string{"-C", repoRoot}, args...)...)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("git %s: %s", strings.Join(args, " "), strings.TrimSpace(stderr.String()))
	}

	paths := make([]string, 0)
	for _, line := range strings.Split(stdout.String(), "\n") {
		path := strings.TrimSpace(line)
		if path == "" {
			continue
		}
		paths = append(paths, filepath.ToSlash(path))
	}
	return paths, nil
}

func parsePorcelainPath(line string) string {
	if len(line) < 4 {
		return ""
	}
	path := strings.TrimSpace(line[3:])
	if path == "" {
		return ""
	}
	if idx := strings.LastIndex(path, " -> "); idx >= 0 {
		path = path[idx+4:]
	}
	return path
}
