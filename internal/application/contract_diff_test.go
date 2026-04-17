package application

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestContract_Diff_NoChanges(t *testing.T) {
	repoRoot := copyApplicationFixtureRepo(t, "verified-spec")
	initGitRepoAtDate(t, repoRoot, "2026-03-28T12:00:00Z")
	headSHA := strings.TrimSpace(runGitAtDate(t, repoRoot, "2026-03-28T12:00:00Z", "rev-parse", "HEAD"))
	service := newApplicationContractService(repoRoot)

	placeholders := contractPlaceholders()
	placeholders["__HEAD_SHA__"] = headSHA
	assertReadContractFixtureCall(t, placeholders, func() (any, []any, error) {
		return service.ReadDiff("runtime:session-lifecycle", "")
	})
}

func TestContract_Diff_CharterSuccess(t *testing.T) {
	repoRoot := copyApplicationFixtureRepo(t, "charter-dag")
	initGitRepoAtDate(t, repoRoot, "2026-03-28T12:00:00Z")
	headSHA := strings.TrimSpace(runGitAtDate(t, repoRoot, "2026-03-28T12:00:00Z", "rev-parse", "HEAD"))
	service := newApplicationContractService(repoRoot)

	placeholders := contractPlaceholders()
	placeholders["__HEAD_SHA__"] = headSHA
	assertReadContractFixtureCall(t, placeholders, func() (any, []any, error) {
		return service.ReadDiff("", "runtime")
	})
}

func TestContract_Diff_TrackedDriftContinuation(t *testing.T) {
	repoRoot := contractTrackedDriftRepo(t)
	headSHA := strings.TrimSpace(runGitAtDate(t, repoRoot, "2026-03-28T12:00:00Z", "rev-parse", "HEAD"))
	service := newApplicationContractService(repoRoot)

	placeholders := contractPlaceholders()
	placeholders["__HEAD_SHA__"] = headSHA
	assertReadContractFixtureCall(t, placeholders, func() (any, []any, error) {
		return service.ReadDiff("runtime:session-lifecycle", "")
	})
}

func TestContract_Diff_CheckpointUnavailable(t *testing.T) {
	repoRoot := copyApplicationFixtureRepo(t, "verified-spec")
	initGitRepoAtDate(t, repoRoot, "2026-03-28T12:00:00Z")
	replaceTrackingCheckpoint(t, filepath.Join(repoRoot, ".specs", "runtime", "session-lifecycle.yaml"), "deadbee")
	service := newApplicationContractService(repoRoot)

	assertReadContractFixtureCall(t, contractPlaceholders(), func() (any, []any, error) {
		return service.ReadDiff("runtime:session-lifecycle", "")
	})
}

func TestContract_Diff_SemanticReview_ScopeCode(t *testing.T) {
	repoRoot := copyApplicationFixtureRepo(t, "verified-spec")
	initGitRepoAtDate(t, repoRoot, "2026-03-28T12:00:00Z")
	headSHA := strings.TrimSpace(runGitAtDate(t, repoRoot, "2026-03-28T12:00:00Z", "rev-parse", "HEAD"))
	codePath := filepath.Join(repoRoot, "runtime", "src", "application", "commands", "handler.py")
	writeApplicationTestFile(t, codePath, []byte("def handle():\n    return 'drift'\n"))
	runGitAtDate(t, repoRoot, "2026-03-30T09:30:00Z", "add", "runtime/src/application/commands/handler.py")
	runGitAtDate(t, repoRoot, "2026-03-30T09:30:00Z", "commit", "-m", "code drift")
	service := newApplicationContractService(repoRoot)

	placeholders := contractPlaceholders()
	placeholders["__HEAD_SHA__"] = headSHA
	assertReadContractFixtureCall(t, placeholders, func() (any, []any, error) {
		return service.ReadDiff("runtime:session-lifecycle", "")
	})
}

func TestContract_Diff_SemanticReview_Both(t *testing.T) {
	repoRoot := copyApplicationFixtureRepo(t, "verified-spec")
	initGitRepoAtDate(t, repoRoot, "2026-03-28T12:00:00Z")
	headSHA := strings.TrimSpace(runGitAtDate(t, repoRoot, "2026-03-28T12:00:00Z", "rev-parse", "HEAD"))
	replaceFileText(t, filepath.Join(repoRoot, "runtime", "src", "domain", "session_execution", "SPEC.md"), "# Session Lifecycle\n", "# Session Lifecycle\n\n## Requirement: Tracked drift review\n\n```gherkin requirement\n@runtime @e2e\nFeature: Tracked drift review\n```\n\n### Scenarios\n\n```gherkin scenario\nScenario: Committed drift is fully covered\n  Given the design doc changed after the checkpoint\n  When the covering delta is opened\n  Then the tracked drift continues through requirement verification\n```\n\n## Drift Review\n\nTracked diff.\n")
	codePath := filepath.Join(repoRoot, "runtime", "src", "application", "commands", "handler.py")
	writeApplicationTestFile(t, codePath, []byte("def handle():\n    return 'mixed'\n"))
	runGitAtDate(t, repoRoot, "2026-03-30T09:30:00Z", "add", ".")
	runGitAtDate(t, repoRoot, "2026-03-30T09:30:00Z", "commit", "-m", "mixed drift")
	service := newApplicationContractService(repoRoot)

	placeholders := contractPlaceholders()
	placeholders["__HEAD_SHA__"] = headSHA
	assertReadContractFixtureCall(t, placeholders, func() (any, []any, error) {
		return service.ReadDiff("runtime:session-lifecycle", "")
	})
}
