package infrastructure

import (
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"testing"
)

func TestGitStatusPorcelainPreservesLeadingPathCharacter(t *testing.T) {
	repoRoot := t.TempDir()
	runGitCommand(t, repoRoot, "init")
	runGitCommand(t, repoRoot, "config", "user.name", "Specctl Tests")
	runGitCommand(t, repoRoot, "config", "user.email", "specctl-tests@example.com")

	filePath := filepath.Join(repoRoot, "runtime", "src", "domain", "session_execution", "SPEC.md")
	if err := os.MkdirAll(filepath.Dir(filePath), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filePath, []byte("# Session Lifecycle\n"), 0o644); err != nil {
		t.Fatalf("write baseline: %v", err)
	}
	runGitCommand(t, repoRoot, "add", ".")
	runGitCommand(t, repoRoot, "commit", "-m", "baseline")

	if err := os.WriteFile(filePath, []byte("# Session Lifecycle\n\nUpdated.\n"), 0o644); err != nil {
		t.Fatalf("write dirty file: %v", err)
	}

	paths, err := GitStatusPorcelain(repoRoot, []string{"runtime/src/domain/session_execution/"})
	if err != nil {
		t.Fatalf("GitStatusPorcelain: %v", err)
	}
	if want := []string{"runtime/src/domain/session_execution/SPEC.md"}; !reflect.DeepEqual(paths, want) {
		t.Fatalf("paths = %#v, want %#v", paths, want)
	}
}

func runGitCommand(t *testing.T, repoRoot string, args ...string) {
	t.Helper()

	cmd := exec.Command("git", append([]string{"-C", repoRoot}, args...)...)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v failed: %v\n%s", args, err, output)
	}
}
