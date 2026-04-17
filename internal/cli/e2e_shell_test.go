package cli

import (
	"os/exec"
	"path/filepath"
	"testing"
)

func TestShellJourneysScript(t *testing.T) {
	t.Parallel()
	cwd, err := filepath.Abs(".")
	if err != nil {
		t.Fatalf("abs cwd: %v", err)
	}
	moduleRoot := filepath.Clean(filepath.Join(cwd, "..", ".."))

	cmd := exec.Command("sh", "test/e2e/specctl_journeys.sh")
	cmd.Dir = moduleRoot
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("shell journeys failed: %v\n%s", err, output)
	}
}
