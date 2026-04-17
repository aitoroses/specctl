package cli

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

// TestMain provides package-level setup/teardown for cli tests.
func TestMain(m *testing.M) {
	code := m.Run()

	// Cleanup cached resources
	cleanupFixtureTemplates()
	cleanupCachedBinary()

	os.Exit(code)
}

// --- Cached binary build ---
// buildSpecctlBinary compiles the specctl binary once per test run.

var (
	cachedBinaryOnce sync.Once
	cachedBinaryPath string
	cachedBinaryDir  string
	cachedBinaryErr  error
)

func buildSpecctlBinary(t *testing.T) string {
	t.Helper()
	cachedBinaryOnce.Do(func() {
		cwd, err := os.Getwd()
		if err != nil {
			cachedBinaryErr = fmt.Errorf("getwd: %w", err)
			return
		}
		moduleRoot := filepath.Clean(filepath.Join(cwd, "..", ".."))
		dir, err := os.MkdirTemp("", "specctl-binary-*")
		if err != nil {
			cachedBinaryErr = fmt.Errorf("tempdir: %w", err)
			return
		}
		cachedBinaryDir = dir
		binaryPath := filepath.Join(dir, "specctl")
		cmd := exec.Command("go", "build", "-o", binaryPath, "./cmd/specctl")
		cmd.Dir = moduleRoot
		output, err := cmd.CombinedOutput()
		if err != nil {
			cachedBinaryErr = fmt.Errorf("go build failed: %v\n%s", err, output)
			return
		}
		cachedBinaryPath = binaryPath
	})
	if cachedBinaryErr != nil {
		t.Fatalf("buildSpecctlBinary: %v", cachedBinaryErr)
	}
	return cachedBinaryPath
}

func cleanupCachedBinary() {
	if cachedBinaryDir != "" {
		os.RemoveAll(cachedBinaryDir)
	}
}

// --- Cached fixture repo templates ---
// copyFixtureRepoWithRegistry initializes a git repo from a fixture.
// The first call per fixture name builds a template (copy + git init + commits).
// Subsequent calls deep-copy the template without running git commands.

var (
	fixtureTemplateMu    sync.Mutex
	fixtureTemplateCache = map[string]string{} // fixture name → template dir
)

func copyFixtureRepoWithRegistry(t *testing.T, fixture string) string {
	t.Helper()

	templateDir := getOrCreateFixtureTemplate(t, fixture)

	// Deep-copy the template (with .git/) to a fresh temp dir — no git commands needed
	dstRoot := t.TempDir()
	if err := deepCopyDir(templateDir, dstRoot); err != nil {
		t.Fatalf("copy fixture template: %v", err)
	}
	return dstRoot
}

func getOrCreateFixtureTemplate(t *testing.T, fixture string) string {
	t.Helper()

	fixtureTemplateMu.Lock()
	defer fixtureTemplateMu.Unlock()

	if dir, ok := fixtureTemplateCache[fixture]; ok {
		return dir
	}

	// Build the template: copy fixtures → init git → commit → rewrite checkpoints → commit
	srcRoot := filepath.Join("..", "..", "testdata", "v2", fixture)
	templateDir, err := os.MkdirTemp("", "specctl-fixture-*")
	if err != nil {
		t.Fatalf("fixture template tempdir: %v", err)
	}

	if err := deepCopyDir(srcRoot, templateDir); err != nil {
		t.Fatalf("copy fixture to template: %v", err)
	}

	// Initialize git (same as the old initGitRepoAtDate path)
	timestamp := "2026-03-28T12:00:00Z"
	runGitInDir(t, templateDir, timestamp, "init")
	runGitInDir(t, templateDir, timestamp, "config", "user.name", "Specctl Tests")
	runGitInDir(t, templateDir, timestamp, "config", "user.email", "specctl-tests@example.com")
	commitAllInDir(t, templateDir, timestamp, "fixture")
	head := strings.TrimSpace(runGitInDir(t, templateDir, timestamp, "rev-parse", "HEAD"))
	rewriteCheckpointsInDir(t, templateDir, head)
	commitAllInDir(t, templateDir, timestamp, "rewrite checkpoints")

	fixtureTemplateCache[fixture] = templateDir
	return templateDir
}

// runGitInDir executes a git command — used only during template creation.
func runGitInDir(t *testing.T, dir, timestamp string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	if timestamp != "" {
		cmd.Env = append(cmd.Environ(), "GIT_AUTHOR_DATE="+timestamp, "GIT_COMMITTER_DATE="+timestamp)
	}
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v failed: %v\n%s", args, err, output)
	}
	return string(output)
}

func commitAllInDir(t *testing.T, dir, timestamp, message string) {
	t.Helper()
	runGitInDir(t, dir, timestamp, "add", ".")
	if strings.TrimSpace(runGitInDir(t, dir, timestamp, "status", "--porcelain")) == "" {
		return
	}
	runGitInDir(t, dir, timestamp, "commit", "-m", message)
}

func rewriteCheckpointsInDir(t *testing.T, repoRoot, checkpoint string) {
	t.Helper()
	specsRoot := filepath.Join(repoRoot, ".specs")
	filepath.Walk(specsRoot, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || filepath.Ext(path) != ".yaml" || filepath.Base(path) == "CHARTER.yaml" {
			return err
		}
		content, readErr := os.ReadFile(path)
		if readErr != nil {
			t.Fatalf("read %s: %v", path, readErr)
		}
		updated := strings.ReplaceAll(string(content), "checkpoint: a1b2c3f", "checkpoint: "+checkpoint)
		if updated != string(content) {
			if err := os.WriteFile(path, []byte(updated), info.Mode()); err != nil {
				t.Fatalf("write %s: %v", path, err)
			}
		}
		return nil
	})
}

func deepCopyDir(src, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}
		dstPath := filepath.Join(dst, rel)
		if info.IsDir() {
			return os.MkdirAll(dstPath, info.Mode())
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(dstPath, data, info.Mode())
	})
}

func cleanupFixtureTemplates() {
	fixtureTemplateMu.Lock()
	defer fixtureTemplateMu.Unlock()
	for _, dir := range fixtureTemplateCache {
		os.RemoveAll(dir)
	}
	fixtureTemplateCache = map[string]string{}
}
