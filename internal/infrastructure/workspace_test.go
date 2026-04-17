package infrastructure

import (
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/aitoroses/specctl/internal/domain"
	"gopkg.in/yaml.v3"
)

func TestSnapshotScopeDriftUsesCommittedCoverageState(t *testing.T) {
	t.Run("transitions clean to drifted to tracked to drifted as committed coverage changes", func(t *testing.T) {
		repoRoot := copyWorkspaceFixtureRepo(t, "verified-spec")
		initWorkspaceGitRepo(t, repoRoot, "2026-03-28T12:00:00Z")
		workspace := NewWorkspace(repoRoot)

		tracking := mustReadTracking(t, repoRoot)
		originCheckpoint := tracking.Checkpoint
		drift, findings := workspace.SnapshotScopeDrift(tracking)
		if drift.Status != "clean" || len(drift.TrackedBy) != 0 || len(findings) != 0 {
			t.Fatalf("initial drift = %#v, findings = %#v", drift, findings)
		}

		docPath := filepath.Join(repoRoot, "runtime", "src", "domain", "session_execution", "SPEC.md")
		if err := os.WriteFile(docPath, []byte("---\nspec: session-lifecycle\ncharter: runtime\n---\n# Session Lifecycle\n\n## Drift Review\n\nDocumented drift.\n"), 0o644); err != nil {
			t.Fatalf("write design doc: %v", err)
		}
		runGitAtDate(t, repoRoot, "2026-03-30T09:30:00Z", "add", ".")
		runGitAtDate(t, repoRoot, "2026-03-30T09:30:00Z", "commit", "-m", "document drift")

		tracking = mustReadTracking(t, repoRoot)
		drift, findings = workspace.SnapshotScopeDrift(tracking)
		if drift.Status != "drifted" || len(drift.TrackedBy) != 0 || len(findings) != 0 {
			t.Fatalf("drift after design-doc commit = %#v, findings = %#v", drift, findings)
		}

		tracking.Deltas = append(tracking.Deltas, domain.Delta{
			ID:               "D-002",
			Area:             "Drift review",
			Status:           domain.DeltaStatusOpen,
			OriginCheckpoint: originCheckpoint,
			Current:          "The updated drift notes are not yet tracked",
			Target:           "Track the committed design-doc change",
			Notes:            "Opened after the document drift commit",
		})
		tracking.SyncComputedStatus()
		writeTrackingFixtureFile(t, tracking.FilePath, tracking)
		runGitAtDate(t, repoRoot, "2026-03-30T09:35:00Z", "add", ".specs/runtime/session-lifecycle.yaml")
		runGitAtDate(t, repoRoot, "2026-03-30T09:35:00Z", "commit", "-m", "track committed drift")

		tracking = mustReadTracking(t, repoRoot)
		drift, findings = workspace.SnapshotScopeDrift(tracking)
		if drift.Status != "tracked" || strings.Join(drift.TrackedBy, ",") != "D-002" || len(findings) != 0 {
			t.Fatalf("drift after delta commit = %#v, findings = %#v", drift, findings)
		}

		codePath := filepath.Join(repoRoot, "runtime", "src", "application", "commands", "handler.py")
		if err := os.WriteFile(codePath, []byte("def handle():\n    return 'new drift'\n"), 0o644); err != nil {
			t.Fatalf("write code file: %v", err)
		}
		runGitAtDate(t, repoRoot, "2026-03-30T09:40:00Z", "add", "runtime/src/application/commands/handler.py")
		runGitAtDate(t, repoRoot, "2026-03-30T09:40:00Z", "commit", "-m", "new uncovered code drift")

		tracking = mustReadTracking(t, repoRoot)
		drift, findings = workspace.SnapshotScopeDrift(tracking)
		if drift.Status != "drifted" || strings.Join(drift.TrackedBy, ",") != "D-002" || len(findings) != 0 {
			t.Fatalf("drift after uncovered commit = %#v, findings = %#v", drift, findings)
		}
	})

	t.Run("closed deltas still cover committed drift until the next sync point", func(t *testing.T) {
		repoRoot := copyWorkspaceFixtureRepo(t, "verified-spec")
		initWorkspaceGitRepo(t, repoRoot, "2026-03-28T12:00:00Z")
		workspace := NewWorkspace(repoRoot)
		tracking := mustReadTracking(t, repoRoot)
		originCheckpoint := tracking.Checkpoint

		docPath := filepath.Join(repoRoot, "runtime", "src", "domain", "session_execution", "SPEC.md")
		if err := os.WriteFile(docPath, []byte("---\nspec: session-lifecycle\ncharter: runtime\n---\n# Session Lifecycle\n\n## Drift Review\n\nClosed delta still covers this.\n"), 0o644); err != nil {
			t.Fatalf("write design doc: %v", err)
		}
		runGitAtDate(t, repoRoot, "2026-03-30T09:30:00Z", "add", ".")
		runGitAtDate(t, repoRoot, "2026-03-30T09:30:00Z", "commit", "-m", "document drift")

		tracking.Deltas = append(tracking.Deltas, domain.Delta{
			ID:               "D-002",
			Area:             "Closed drift review",
			Status:           domain.DeltaStatusClosed,
			OriginCheckpoint: originCheckpoint,
			Current:          "The drift is committed but not yet re-anchored",
			Target:           "Close the drift cycle before the next sync point",
			Notes:            "Closed deltas still count as coverage",
		})
		testPath := filepath.Join(repoRoot, "runtime", "tests", "domain", "test_closed_drift_review.py")
		if err := os.WriteFile(testPath, []byte("def test_closed_drift_review():\n    assert True\n"), 0o644); err != nil {
			t.Fatalf("write closed drift test file: %v", err)
		}
		tracking.Requirements = append(tracking.Requirements, domain.Requirement{
			ID:           "REQ-002",
			Title:        "Closed drift review",
			Tags:         []string{"runtime"},
			TestFiles:    []string{"runtime/tests/domain/test_closed_drift_review.py"},
			Gherkin:      "@runtime\nFeature: Closed drift review",
			Lifecycle:    domain.RequirementLifecycleActive,
			Verification: domain.RequirementVerificationVerified,
			IntroducedBy: "D-002",
		})
		tracking.SyncComputedStatus()
		writeTrackingFixtureFile(t, tracking.FilePath, tracking)
		runGitAtDate(t, repoRoot, "2026-03-30T09:35:00Z", "add", ".specs/runtime/session-lifecycle.yaml")
		runGitAtDate(t, repoRoot, "2026-03-30T09:35:00Z", "commit", "-m", "commit closed delta")

		tracking = mustReadTracking(t, repoRoot)
		drift, findings := workspace.SnapshotScopeDrift(tracking)
		// Closed deltas do NOT cover drift — they represent completed work awaiting rev bump.
		// Drift should be "drifted", not "tracked".
		if drift.Status != "drifted" || len(drift.TrackedBy) != 0 || len(findings) != 0 {
			t.Fatalf("drift = %#v, findings = %#v", drift, findings)
		}
	})

	t.Run("unresolvable delta origins remove coverage without making the spec unavailable", func(t *testing.T) {
		repoRoot := copyWorkspaceFixtureRepo(t, "verified-spec")
		initWorkspaceGitRepo(t, repoRoot, "2026-03-28T12:00:00Z")
		workspace := NewWorkspace(repoRoot)

		docPath := filepath.Join(repoRoot, "runtime", "src", "domain", "session_execution", "SPEC.md")
		if err := os.WriteFile(docPath, []byte("---\nspec: session-lifecycle\ncharter: runtime\n---\n# Session Lifecycle\n\n## Drift Review\n\nBroken delta origin.\n"), 0o644); err != nil {
			t.Fatalf("write design doc: %v", err)
		}
		runGitAtDate(t, repoRoot, "2026-03-30T09:30:00Z", "add", ".")
		runGitAtDate(t, repoRoot, "2026-03-30T09:30:00Z", "commit", "-m", "document drift")

		tracking := mustReadTracking(t, repoRoot)
		tracking.Deltas = append(tracking.Deltas, domain.Delta{
			ID:               "D-002",
			Area:             "Broken origin",
			Status:           domain.DeltaStatusOpen,
			OriginCheckpoint: "deadbee",
			Current:          "The stored origin no longer exists",
			Target:           "Open a fresh delta from the live checkpoint",
			Notes:            "Broken origin must not make the whole spec unavailable",
		})
		tracking.SyncComputedStatus()
		writeTrackingFixtureFile(t, tracking.FilePath, tracking)
		runGitAtDate(t, repoRoot, "2026-03-30T09:35:00Z", "add", ".specs/runtime/session-lifecycle.yaml")
		runGitAtDate(t, repoRoot, "2026-03-30T09:35:00Z", "commit", "-m", "commit broken delta")

		tracking = mustReadTracking(t, repoRoot)
		drift, findings := workspace.SnapshotScopeDrift(tracking)
		if drift.Status != "drifted" || len(drift.TrackedBy) != 0 || len(findings) != 0 {
			t.Fatalf("drift = %#v, findings = %#v", drift, findings)
		}
	})
}

func TestClassifyDriftSource(t *testing.T) {
	tests := []struct {
		name        string
		docChanged  bool
		codeChanged bool
		want        string
	}{
		{name: "no drift", want: ""},
		{name: "design doc only", docChanged: true, want: "design_doc"},
		{name: "scope code only", codeChanged: true, want: "scope_code"},
		{name: "both", docChanged: true, codeChanged: true, want: "both"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ClassifyDriftSource(tt.docChanged, tt.codeChanged); got != tt.want {
				t.Fatalf("ClassifyDriftSource(%t, %t) = %q, want %q", tt.docChanged, tt.codeChanged, got, tt.want)
			}
		})
	}
}

func copyWorkspaceFixtureRepo(t *testing.T, fixture string) string {
	t.Helper()

	dst := t.TempDir()
	src := fixtureRoot(fixture)
	if err := filepath.Walk(src, func(path string, info fs.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if info.IsDir() {
			return os.MkdirAll(target, info.Mode())
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(target, data, info.Mode())
	}); err != nil {
		t.Fatalf("copy fixture %s: %v", fixture, err)
	}
	return dst
}

func initWorkspaceGitRepo(t *testing.T, repoRoot, timestamp string) string {
	t.Helper()

	runGitAtDate(t, repoRoot, timestamp, "init")
	runGitAtDate(t, repoRoot, timestamp, "config", "user.name", "Specctl Tests")
	runGitAtDate(t, repoRoot, timestamp, "config", "user.email", "specctl-tests@example.com")
	runGitAtDate(t, repoRoot, timestamp, "add", ".")
	runGitAtDate(t, repoRoot, timestamp, "commit", "-m", "fixture")
	headSHA := strings.TrimSpace(runGitAtDate(t, repoRoot, timestamp, "rev-parse", "HEAD"))
	rewriteWorkspaceTrackingCheckpoints(t, repoRoot, headSHA)
	runGitAtDate(t, repoRoot, timestamp, "add", ".")
	runGitAtDate(t, repoRoot, timestamp, "commit", "-m", "rewrite checkpoints")
	return strings.TrimSpace(runGitAtDate(t, repoRoot, timestamp, "rev-parse", "HEAD"))
}

func runGitAtDate(t *testing.T, repoRoot, timestamp string, args ...string) string {
	t.Helper()

	cmd := exec.Command("git", args...)
	cmd.Dir = repoRoot
	cmd.Env = append(cmd.Environ(), "GIT_AUTHOR_DATE="+timestamp, "GIT_COMMITTER_DATE="+timestamp)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v failed: %v\n%s", args, err, output)
	}
	return string(output)
}

func rewriteWorkspaceTrackingCheckpoints(t *testing.T, repoRoot, checkpoint string) {
	t.Helper()

	specsRoot := filepath.Join(repoRoot, ".specs")
	if err := filepath.Walk(specsRoot, func(path string, info fs.FileInfo, err error) error {
		if err != nil || info.IsDir() || filepath.Ext(path) != ".yaml" || filepath.Base(path) == "CHARTER.yaml" {
			return err
		}
		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		updated := strings.ReplaceAll(string(content), "checkpoint: a1b2c3f", "checkpoint: "+checkpoint)
		updated = strings.ReplaceAll(updated, "origin_checkpoint: a1b2c3f", "origin_checkpoint: "+checkpoint)
		return os.WriteFile(path, []byte(updated), info.Mode())
	}); err != nil {
		t.Fatalf("rewrite checkpoints: %v", err)
	}
}

func mustReadTracking(t *testing.T, repoRoot string) *domain.TrackingFile {
	t.Helper()

	tracking, err := ReadTrackingFile(filepath.Join(repoRoot, ".specs", "runtime", "session-lifecycle.yaml"))
	if err != nil {
		t.Fatalf("ReadTrackingFile: %v", err)
	}
	return tracking
}

func writeTrackingFixtureFile(t *testing.T, path string, tracking *domain.TrackingFile) {
	t.Helper()

	data, err := yaml.Marshal(tracking)
	if err != nil {
		t.Fatalf("marshal tracking: %v", err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("write tracking: %v", err)
	}
}
