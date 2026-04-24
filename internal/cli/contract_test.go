package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestContract_Context_SpecClean(t *testing.T) {
	repoRoot := copyFixtureRepoWithRegistry(t, "verified-spec")
	headSHA := strings.TrimSpace(runGit(t, repoRoot, "rev-parse", "HEAD"))

	withWorkingDir(t, repoRoot, func() {
		stdout, stderr, exitCode := executeCLI("context", "runtime:session-lifecycle")
		if exitCode != 0 {
			t.Fatalf("exit code = %d, stderr=%s stdout=%s", exitCode, stderr, stdout)
		}
		if stderr != "" {
			t.Fatalf("stderr = %q", stderr)
		}

		assertContractFixture(t, stdout, map[string]string{"__HEAD_SHA__": headSHA})
	})
}

func TestContract_Context_CleanNone_InactiveSupersededUnverified(t *testing.T) {
	repoRoot := contractInactiveSupersededRequirementRepo(t)
	headSHA := strings.TrimSpace(runGit(t, repoRoot, "rev-parse", "HEAD"))

	withWorkingDir(t, repoRoot, func() {
		stdout, stderr, exitCode := executeCLI("context", "runtime:session-lifecycle")
		if exitCode != 0 {
			t.Fatalf("exit code = %d, stderr=%s stdout=%s", exitCode, stderr, stdout)
		}
		if stderr != "" {
			t.Fatalf("stderr = %q", stderr)
		}

		assertContractFixture(t, stdout, map[string]string{"__HEAD_SHA__": headSHA})
	})
}

func TestContract_Context_InvalidInput(t *testing.T) {
	stdout, stderr, exitCode := executeCLI("context", "runtime/session-lifecycle")
	if exitCode == 0 {
		t.Fatalf("expected failure, stdout=%s", stdout)
	}
	if stderr != "" {
		t.Fatalf("stderr = %q", stderr)
	}

	assertContractFixture(t, stdout, nil)
}

func TestContract_Diff_SemanticReview(t *testing.T) {
	repoRoot := copyFixtureRepoWithRegistry(t, "verified-spec")
	docPath := filepath.Join(repoRoot, "runtime", "src", "domain", "session_execution", "SPEC.md")
	headSHA := initGitRepoAtDate(t, repoRoot, "2026-03-28T12:00:00Z")
	replaceInFile(t, filepath.Join(repoRoot, ".specs", "runtime", "session-lifecycle.yaml"), "checkpoint: a1b2c3f", "checkpoint: "+headSHA)
	writeCLIAdjacentTestFile(t, docPath, []byte("---\nspec: session-lifecycle\ncharter: runtime\n---\n# Session Lifecycle\n\n## Overview\nUpdated overview.\n\n## Recovery Notes\nExpanded recovery notes.\n"))
	commitAllChangesAtDate(t, repoRoot, "2026-03-30T09:30:00Z", "multi-section drift")

	withWorkingDir(t, repoRoot, func() {
		stdout, stderr, exitCode := executeCLI("diff", "runtime:session-lifecycle")
		if exitCode != 0 {
			t.Fatalf("exit code = %d, stderr=%s stdout=%s", exitCode, stderr, stdout)
		}
		if stderr != "" {
			t.Fatalf("stderr = %q", stderr)
		}

		assertContractFixture(t, stdout, nil)
	})
}

func TestContract_Diff_InvalidInput(t *testing.T) {
	stdout, stderr, exitCode := executeCLI("diff")
	if exitCode == 0 {
		t.Fatalf("expected failure, stdout=%s", stdout)
	}
	if stderr != "" {
		t.Fatalf("stderr = %q", stderr)
	}

	assertContractFixture(t, stdout, nil)
}

func TestContract_Hook_Integration(t *testing.T) {
	repoRoot := copyFixtureRepoWithRegistry(t, "charter-dag")
	if err := os.MkdirAll(filepath.Join(repoRoot, "runtime", "src", "orphan"), 0o755); err != nil {
		t.Fatalf("mkdir orphan: %v", err)
	}
	writeCLIAdjacentTestFile(t, filepath.Join(repoRoot, "runtime", "src", "orphan", "worker.py"), []byte("pass\n"))
	if err := os.MkdirAll(filepath.Join(repoRoot, "runtime", "src", "domain", "session_execution"), 0o755); err != nil {
		t.Fatalf("mkdir service dir: %v", err)
	}
	writeCLIAdjacentTestFile(t, filepath.Join(repoRoot, "runtime", "src", "domain", "session_execution", "services.py"), []byte("pass\n"))
	if err := os.MkdirAll(filepath.Join(repoRoot, "docs"), 0o755); err != nil {
		t.Fatalf("mkdir docs: %v", err)
	}
	writeCLIAdjacentTestFile(t, filepath.Join(repoRoot, "docs", "notes.md"), []byte("# Notes\n"))

	withWorkingDir(t, repoRoot, func() {
		stdout, stderr, exitCode := executeCLIWithInput(".specs/runtime/CHARTER.yaml\n.specs/runtime/session-lifecycle.yaml\n.specs/specctl.yaml\nruntime/src/domain/session_execution/services.py\nruntime/src/orphan/worker.py\ndocs/notes.md\n", "hook")
		if exitCode != 0 {
			t.Fatalf("exit code = %d, stderr=%s stdout=%s", exitCode, stderr, stdout)
		}
		if stderr != "" {
			t.Fatalf("stderr = %q", stderr)
		}

		assertContractFixture(t, stdout, nil)
	})
}

func TestContract_Hook_InvalidInput(t *testing.T) {
	stdout, stderr, exitCode := executeCLI("hook", "unexpected")
	if exitCode == 0 {
		t.Fatalf("expected failure, stdout=%s", stdout)
	}
	if stderr != "" {
		t.Fatalf("stderr = %q", stderr)
	}

	assertContractFixture(t, stdout, nil)
}

func TestContract_Config_Read(t *testing.T) {
	repoRoot := copyFixtureRepoWithRegistry(t, "ready-spec")

	withWorkingDir(t, repoRoot, func() {
		stdout, stderr, exitCode := executeCLI("config")
		if exitCode != 0 {
			t.Fatalf("exit code = %d, stderr=%s stdout=%s", exitCode, stderr, stdout)
		}
		if stderr != "" {
			t.Fatalf("stderr = %q", stderr)
		}

		assertContractFixture(t, stdout, nil)
	})
}

func TestContract_Config_InvalidInput(t *testing.T) {
	stdout, stderr, exitCode := executeCLI("config", "unexpected")
	if exitCode == 0 {
		t.Fatalf("expected failure, stdout=%s", stdout)
	}
	if stderr != "" {
		t.Fatalf("stderr = %q", stderr)
	}

	assertContractFixture(t, stdout, nil)
}

func TestContract_CharterCreate_Success(t *testing.T) {
	repoRoot := t.TempDir()
	if err := os.MkdirAll(filepath.Join(repoRoot, ".specs"), 0o755); err != nil {
		t.Fatalf("mkdir specs dir: %v", err)
	}

	withWorkingDir(t, repoRoot, func() {
		stdout, stderr, exitCode := executeCLIWithInput("title: Runtime System\ndescription: Specs for runtime\ngroups:\n  - key: execution\n    title: Execution Engine\n    order: 10\n", "charter", "create", "runtime")
		if exitCode != 0 {
			t.Fatalf("exit code = %d, stderr=%s stdout=%s", exitCode, stderr, stdout)
		}
		if stderr != "" {
			t.Fatalf("stderr = %q", stderr)
		}

		assertContractFixture(t, stdout, nil)
	})
}

func TestContract_CharterAddSpec_Success(t *testing.T) {
	repoRoot := copyFixtureRepoWithRegistry(t, "ready-spec")

	withWorkingDir(t, repoRoot, func() {
		stdout, stderr, exitCode := executeCLIWithInput("group: recovery\norder: 30\ndepends_on: []\nnotes: Session FSM and cleanup behavior\n", "charter", "add-spec", "runtime", "session-lifecycle")
		if exitCode != 0 {
			t.Fatalf("exit code = %d, stderr=%s stdout=%s", exitCode, stderr, stdout)
		}
		if stderr != "" {
			t.Fatalf("stderr = %q", stderr)
		}

		assertContractFixture(t, stdout, nil)
	})
}

func TestContract_SpecCreate_Success(t *testing.T) {
	repoRoot := charterOnlyRepo(t)

	withWorkingDir(t, repoRoot, func() {
		stdout, stderr, exitCode := executeCLI("spec", "create", "runtime:session-lifecycle", "--title", "Session Lifecycle", "--doc", "runtime/src/domain/session_execution/SPEC.md", "--scope", "runtime/src/domain/session_execution/", "--group", "recovery", "--order", "20", "--charter-notes", "Session FSM and cleanup behavior")
		if exitCode != 0 {
			t.Fatalf("exit code = %d, stderr=%s stdout=%s", exitCode, stderr, stdout)
		}
		if stderr != "" {
			t.Fatalf("stderr = %q", stderr)
		}

		assertContractFixture(t, stdout, nil)
	})
}

func TestContract_DeltaAdd_Success(t *testing.T) {
	repoRoot := copyFixtureRepoWithRegistry(t, "ready-spec")

	withWorkingDir(t, repoRoot, func() {
		stdout, stderr, exitCode := executeCLIWithInput("current: Current gap\ntarget: Target gap\nnotes: Explicitly tracked\n", "delta", "add", "runtime:session-lifecycle", "--intent", "add", "--area", "Heartbeat timeout")
		if exitCode != 0 {
			t.Fatalf("exit code = %d, stderr=%s stdout=%s", exitCode, stderr, stdout)
		}
		if stderr != "" {
			t.Fatalf("stderr = %q", stderr)
		}

		assertContractFixture(t, stdout, nil)
	})
}

func TestContract_DeltaClose_Success(t *testing.T) {
	repoRoot := copyFixtureRepoWithRegistry(t, "verified-spec")
	replaceInFile(t, filepath.Join(repoRoot, ".specs", "runtime", "session-lifecycle.yaml"), "status: closed", "status: in-progress")
	replaceInFile(t, filepath.Join(repoRoot, ".specs", "runtime", "session-lifecycle.yaml"), "status: verified", "status: active")

	withWorkingDir(t, repoRoot, func() {
		stdout, stderr, exitCode := executeCLI("delta", "close", "runtime:session-lifecycle", "D-001")
		if exitCode != 0 {
			t.Fatalf("exit code = %d, stderr=%s stdout=%s", exitCode, stderr, stdout)
		}
		if stderr != "" {
			t.Fatalf("stderr = %q", stderr)
		}

		assertContractFixture(t, stdout, nil)
	})
}

func TestContract_DeltaResume_Success(t *testing.T) {
	repoRoot := copyFixtureRepoWithRegistry(t, "ready-spec")

	withWorkingDir(t, repoRoot, func() {
		_, stderr, exitCode := executeCLI("delta", "start", "runtime:session-lifecycle", "D-001")
		if exitCode != 0 {
			t.Fatalf("delta start failed: exit=%d stderr=%s", exitCode, stderr)
		}
		_, stderr, exitCode = executeCLI("delta", "defer", "runtime:session-lifecycle", "D-001")
		if exitCode != 0 {
			t.Fatalf("delta defer failed: exit=%d stderr=%s", exitCode, stderr)
		}

		stdout, resumeStderr, resumeExitCode := executeCLI("delta", "resume", "runtime:session-lifecycle", "D-001")
		if resumeExitCode != 0 {
			t.Fatalf("delta resume failed: exit=%d stderr=%s stdout=%s", resumeExitCode, resumeStderr, stdout)
		}
		if resumeStderr != "" {
			t.Fatalf("stderr = %q", resumeStderr)
		}

		assertContractFixture(t, stdout, nil)
	})
}

func TestContract_DeltaWithdraw_Success(t *testing.T) {
	repoRoot := copyFixtureRepoWithRegistry(t, "ready-spec")

	withWorkingDir(t, repoRoot, func() {
		stdout, stderr, exitCode := executeCLI("delta", "withdraw", "runtime:session-lifecycle", "D-001", "--reason", "Opened in error; replacement tracked separately")
		if exitCode != 0 {
			t.Fatalf("delta withdraw failed: exit=%d stderr=%s stdout=%s", exitCode, stderr, stdout)
		}
		if stderr != "" {
			t.Fatalf("stderr = %q", stderr)
		}

		assertContractFixture(t, stdout, nil)
	})
}

func TestContract_ReqAdd_Success(t *testing.T) {
	repoRoot := copyFixtureRepoWithRegistry(t, "ready-spec")
	gherkin := "@runtime @e2e\nFeature: Compensation stage 4 failure cleanup\n\n  Scenario: Cleanup runs after stage 4 failure\n    Given stage 4 fails during compensation\n    When recovery completes\n    Then cleanup steps run in documented order\n"

	withWorkingDir(t, repoRoot, func() {
		stdout, stderr, exitCode := executeCLIWithInput(gherkin, "req", "add", "runtime:session-lifecycle", "--delta", "D-001")
		if exitCode != 0 {
			t.Fatalf("exit code = %d, stderr=%s stdout=%s", exitCode, stderr, stdout)
		}
		if stderr != "" {
			t.Fatalf("stderr = %q", stderr)
		}

		assertContractFixture(t, stdout, nil)
	})
}

func TestContract_ReqAdd_RequirementNotInSpec(t *testing.T) {
	repoRoot := copyFixtureRepoWithRegistry(t, "ready-spec")
	gherkin := "@runtime @e2e\nFeature: Undocumented cleanup branch\n\n  Scenario: Cleanup works for a new branch\n    Given a new cleanup branch exists\n    When recovery completes\n    Then the branch is cleaned up deterministically\n"

	withWorkingDir(t, repoRoot, func() {
		stdout, stderr, exitCode := executeCLIWithInput(gherkin, "req", "add", "runtime:session-lifecycle", "--delta", "D-001")
		if exitCode == 0 {
			t.Fatalf("expected failure, stdout=%s", stdout)
		}
		if stderr != "" {
			t.Fatalf("stderr = %q", stderr)
		}

		assertContractFixture(t, stdout, nil)
	})
}

func TestContract_ReqVerify_Success(t *testing.T) {
	repoRoot := manualRequirementRepo(t)

	withWorkingDir(t, repoRoot, func() {
		stdout, stderr, exitCode := executeCLI("req", "verify", "runtime:session-lifecycle", "REQ-001")
		if exitCode != 0 {
			t.Fatalf("exit code = %d, stderr=%s stdout=%s", exitCode, stderr, stdout)
		}
		if stderr != "" {
			t.Fatalf("stderr = %q", stderr)
		}

		assertContractFixture(t, stdout, nil)
	})
}

func TestContract_DeltaAdd_ChangeSuccess(t *testing.T) {
	repoRoot := contractActiveRequirementRepo(t)

	withWorkingDir(t, repoRoot, func() {
		stdout, stderr, exitCode := executeCLIWithInput("current: Current cleanup wording is outdated\ntarget: Replace the tracked cleanup contract\nnotes: Behavior shifted during rollout\naffects_requirements:\n  - REQ-001\n", "delta", "add", "runtime:session-lifecycle", "--intent", "change", "--area", "Compensation cleanup rewrite")
		if exitCode != 0 {
			t.Fatalf("exit code = %d, stderr=%s stdout=%s", exitCode, stderr, stdout)
		}
		if stderr != "" {
			t.Fatalf("stderr = %q", stderr)
		}

		assertContractFixture(t, stdout, nil)
	})
}

func TestContract_DeltaAdd_ChangeMissingAffects(t *testing.T) {
	repoRoot := copyFixtureRepoWithRegistry(t, "active-spec")

	withWorkingDir(t, repoRoot, func() {
		stdout, stderr, exitCode := executeCLIWithInput("current: Current cleanup wording is outdated\ntarget: Replace the tracked cleanup contract\nnotes: Behavior shifted during rollout\n", "delta", "add", "runtime:session-lifecycle", "--intent", "change", "--area", "Compensation cleanup rewrite")
		if exitCode == 0 {
			t.Fatalf("expected failure, stdout=%s", stdout)
		}
		if stderr != "" {
			t.Fatalf("stderr = %q", stderr)
		}

		assertContractFixture(t, stdout, nil)
	})
}

func TestContract_Context_ExactMatchDriftRefreshChooser(t *testing.T) {
	repoRoot := contractContextExactMatchDriftRepo(t)

	withWorkingDir(t, repoRoot, func() {
		stdout, stderr, exitCode := executeCLI("context", "runtime:session-lifecycle")
		if exitCode != 0 {
			t.Fatalf("exit code = %d, stderr=%s stdout=%s", exitCode, stderr, stdout)
		}
		if stderr != "" {
			t.Fatalf("stderr = %q", stderr)
		}

		assertContractFixture(t, stdout, nil)
	})
}

func TestContract_Context_StaleActiveRequirementRepairChooser(t *testing.T) {
	repoRoot := contractContextStaleRequirementRepo(t)

	withWorkingDir(t, repoRoot, func() {
		stdout, stderr, exitCode := executeCLI("context", "runtime:session-lifecycle")
		if exitCode != 0 {
			t.Fatalf("exit code = %d, stderr=%s stdout=%s", exitCode, stderr, stdout)
		}
		if stderr != "" {
			t.Fatalf("stderr = %q", stderr)
		}

		assertContractFixture(t, stdout, nil)
	})
}

func TestContract_DeltaClose_UpdatesUnresolved(t *testing.T) {
	repoRoot := contractChangeDeltaRepo(t)
	replaceInFile(t, filepath.Join(repoRoot, ".specs", "runtime", "session-lifecycle.yaml"), "verification: unverified", "verification: verified")
	replaceInFile(t, filepath.Join(repoRoot, ".specs", "runtime", "session-lifecycle.yaml"), "test_files: []", "test_files:\n      - runtime/tests/domain/test_compensation_cleanup.py")
	writeCLIAdjacentTestFile(t, filepath.Join(repoRoot, "runtime", "tests", "domain", "test_compensation_cleanup.py"), []byte("def test_cleanup():\n    assert True\n"))

	withWorkingDir(t, repoRoot, func() {
		stdout, stderr, exitCode := executeCLI("delta", "close", "runtime:session-lifecycle", "D-002")
		if exitCode == 0 {
			t.Fatalf("expected failure, stdout=%s", stdout)
		}
		if stderr != "" {
			t.Fatalf("stderr = %q", stderr)
		}

		assertContractFixture(t, stdout, nil)
	})
}

func TestContract_DeltaClose_MatchBlocking(t *testing.T) {
	repoRoot := contractReplaceFlowRepo(t)

	withWorkingDir(t, repoRoot, func() {
		_, stderr, exitCode := executeCLIWithInput(contractReplacementRequirementBlock(), "req", "replace", "runtime:session-lifecycle", "REQ-001", "--delta", "D-002")
		if exitCode != 0 {
			t.Fatalf("req replace failed: exit=%d stderr=%s", exitCode, stderr)
		}

		stdout, closeStderr, closeExitCode := executeCLIWithInput("runtime/tests/domain/test_compensation_cleanup.py\n", "req", "verify", "runtime:session-lifecycle", "REQ-002", "--test-file", "runtime/tests/domain/test_compensation_cleanup.py")
		if closeExitCode != 0 {
			t.Fatalf("req verify failed: exit=%d stderr=%s stdout=%s", closeExitCode, closeStderr, stdout)
		}
		if closeStderr != "" {
			t.Fatalf("stderr = %q", closeStderr)
		}
	})

	replaceSessionLifecycleDoc(t, repoRoot, contractRequirementSection("Compensation stage 4 failure cleanup", contractRequirementBlock("@runtime @e2e", "Compensation stage 4 failure cleanup (edited)", "Cleanup runs after stage 4 failure", "stage 4 fails during compensation", "recovery completes", "cleanup steps run in documented order")))

	withWorkingDir(t, repoRoot, func() {
		stdout, stderr, exitCode := executeCLI("delta", "close", "runtime:session-lifecycle", "D-002")
		if exitCode == 0 {
			t.Fatalf("expected failure, stdout=%s", stdout)
		}
		if stderr != "" {
			t.Fatalf("stderr = %q", stderr)
		}

		assertContractFixture(t, stdout, nil)
	})
}

func TestContract_DeltaClose_RepairTerminal(t *testing.T) {
	repoRoot := contractRepairTerminalRepo(t)

	withWorkingDir(t, repoRoot, func() {
		stdout, stderr, exitCode := executeCLI("delta", "close", "runtime:session-lifecycle", "D-002")
		if exitCode != 0 {
			t.Fatalf("exit code = %d, stderr=%s stdout=%s", exitCode, stderr, stdout)
		}
		if stderr != "" {
			t.Fatalf("stderr = %q", stderr)
		}

		assertContractFixture(t, stdout, nil)
	})
}

func TestContract_ReqReplace_Success(t *testing.T) {
	repoRoot := contractReplaceFlowRepo(t)

	withWorkingDir(t, repoRoot, func() {
		stdout, stderr, exitCode := executeCLIWithInput(contractReplacementRequirementBlock(), "req", "replace", "runtime:session-lifecycle", "REQ-001", "--delta", "D-002")
		if exitCode != 0 {
			t.Fatalf("exit code = %d, stderr=%s stdout=%s", exitCode, stderr, stdout)
		}
		if stderr != "" {
			t.Fatalf("stderr = %q", stderr)
		}

		assertContractFixture(t, stdout, nil)
	})
}

func TestContract_ReqReplace_RequirementNotInSpec(t *testing.T) {
	repoRoot := contractChangeDeltaRepo(t)

	withWorkingDir(t, repoRoot, func() {
		stdout, stderr, exitCode := executeCLIWithInput(contractReplacementRequirementBlock(), "req", "replace", "runtime:session-lifecycle", "REQ-001", "--delta", "D-002")
		if exitCode == 0 {
			t.Fatalf("expected failure, stdout=%s", stdout)
		}
		if stderr != "" {
			t.Fatalf("stderr = %q", stderr)
		}

		assertContractFixture(t, stdout, nil)
	})
}

func TestContract_ReqWithdraw_Success(t *testing.T) {
	repoRoot := contractRemoveDeltaRepo(t)

	withWorkingDir(t, repoRoot, func() {
		stdout, stderr, exitCode := executeCLI("req", "withdraw", "runtime:session-lifecycle", "REQ-001", "--delta", "D-002")
		if exitCode != 0 {
			t.Fatalf("exit code = %d, stderr=%s stdout=%s", exitCode, stderr, stdout)
		}
		if stderr != "" {
			t.Fatalf("stderr = %q", stderr)
		}

		assertContractFixture(t, stdout, nil)
	})
}

func TestContract_ReqStale_Success(t *testing.T) {
	repoRoot := contractRepairDeltaRepo(t)

	withWorkingDir(t, repoRoot, func() {
		stdout, stderr, exitCode := executeCLI("req", "stale", "runtime:session-lifecycle", "REQ-001", "--delta", "D-002")
		if exitCode != 0 {
			t.Fatalf("exit code = %d, stderr=%s stdout=%s", exitCode, stderr, stdout)
		}
		if stderr != "" {
			t.Fatalf("stderr = %q", stderr)
		}

		assertContractFixture(t, stdout, nil)
	})
}

func TestContract_ReqRefresh_Success(t *testing.T) {
	repoRoot := contractRefreshRepo(t)

	withWorkingDir(t, repoRoot, func() {
		stdout, stderr, exitCode := executeCLIWithInput(contractRequirementBlock("@runtime @e2e\n@critical", "Compensation stage 4 failure cleanup", "", "", "", ""), "req", "refresh", "runtime:session-lifecycle", "REQ-001")
		if exitCode != 0 {
			t.Fatalf("exit code = %d, stderr=%s stdout=%s", exitCode, stderr, stdout)
		}
		if stderr != "" {
			t.Fatalf("stderr = %q", stderr)
		}

		assertContractFixture(t, stdout, nil)
	})
}

func TestContract_ReqVerify_Superseded(t *testing.T) {
	repoRoot := contractReplaceFlowRepo(t)

	withWorkingDir(t, repoRoot, func() {
		_, stderr, exitCode := executeCLIWithInput(contractReplacementRequirementBlock(), "req", "replace", "runtime:session-lifecycle", "REQ-001", "--delta", "D-002")
		if exitCode != 0 {
			t.Fatalf("req replace failed: exit=%d stderr=%s", exitCode, stderr)
		}
	})

	withWorkingDir(t, repoRoot, func() {
		stdout, stderr, exitCode := executeCLI("req", "verify", "runtime:session-lifecycle", "REQ-001", "--test-file", "runtime/tests/domain/test_compensation_cleanup.py")
		if exitCode == 0 {
			t.Fatalf("expected failure, stdout=%s", stdout)
		}
		if stderr != "" {
			t.Fatalf("stderr = %q", stderr)
		}

		assertContractFixture(t, stdout, nil)
	})
}

func TestContract_ReqVerify_Withdrawn(t *testing.T) {
	repoRoot := contractRemoveDeltaRepo(t)

	withWorkingDir(t, repoRoot, func() {
		_, stderr, exitCode := executeCLI("req", "withdraw", "runtime:session-lifecycle", "REQ-001", "--delta", "D-002")
		if exitCode != 0 {
			t.Fatalf("req withdraw failed: exit=%d stderr=%s", exitCode, stderr)
		}
	})

	withWorkingDir(t, repoRoot, func() {
		stdout, stderr, exitCode := executeCLI("req", "verify", "runtime:session-lifecycle", "REQ-001", "--test-file", "runtime/tests/domain/test_compensation_cleanup.py")
		if exitCode == 0 {
			t.Fatalf("expected failure, stdout=%s", stdout)
		}
		if stderr != "" {
			t.Fatalf("stderr = %q", stderr)
		}

		assertContractFixture(t, stdout, nil)
	})
}

func TestContract_ReqVerify_MatchBlocking(t *testing.T) {
	repoRoot := contractRefreshRepo(t)

	withWorkingDir(t, repoRoot, func() {
		stdout, stderr, exitCode := executeCLI("req", "verify", "runtime:session-lifecycle", "REQ-001", "--test-file", "runtime/tests/domain/test_compensation_cleanup.py")
		if exitCode == 0 {
			t.Fatalf("expected failure, stdout=%s", stdout)
		}
		if stderr != "" {
			t.Fatalf("stderr = %q", stderr)
		}

		assertContractFixture(t, stdout, nil)
	})
}

func TestContract_RevBump_MatchBlocking(t *testing.T) {
	repoRoot := contractVerifiedMatchBlockingRepo(t)
	headSHA := strings.TrimSpace(runGit(t, repoRoot, "rev-parse", "HEAD"))

	withWorkingDir(t, repoRoot, func() {
		stdout, stderr, exitCode := executeCLIWithInput("Document drift still needs semantic resolution.\n", "rev", "bump", "runtime:session-lifecycle", "--checkpoint", "HEAD")
		if exitCode == 0 {
			t.Fatalf("expected failure, stdout=%s", stdout)
		}
		if stderr != "" {
			t.Fatalf("stderr = %q", stderr)
		}

		assertContractFixture(t, stdout, map[string]string{"__HEAD_SHA__": headSHA})
	})
}

func TestContract_Sync_SemanticWorkRequired(t *testing.T) {
	repoRoot := contractDriftedDesignDocRepo(t)
	headSHA := strings.TrimSpace(runGit(t, repoRoot, "rev-parse", "HEAD"))

	withWorkingDir(t, repoRoot, func() {
		stdout, stderr, exitCode := executeCLIWithInput("Review confirmed the design-doc edit is clarification-only.\n", "sync", "runtime:session-lifecycle", "--checkpoint", "HEAD")
		if exitCode != 0 {
			t.Fatalf("expected success, stdout=%s", stdout)
		}
		if stderr != "" {
			t.Fatalf("stderr = %q", stderr)
		}

		assertContractFixture(t, stdout, map[string]string{"__HEAD_SHA__": headSHA})
	})
}

func TestContract_Sync_LiveDeltasPresent(t *testing.T) {
	repoRoot := contractRepairDeltaRepo(t)

	withWorkingDir(t, repoRoot, func() {
		stdout, stderr, exitCode := executeCLIWithInput("Trying to sync before closing live work.\n", "sync", "runtime:session-lifecycle", "--checkpoint", "HEAD")
		if exitCode == 0 {
			t.Fatalf("expected failure, stdout=%s", stdout)
		}
		if stderr != "" {
			t.Fatalf("stderr = %q", stderr)
		}

		assertContractFixture(t, stdout, nil)
	})
}

func TestContract_Sync_MatchBlocking(t *testing.T) {
	repoRoot := contractVerifiedMatchBlockingRepo(t)
	newHead := strings.TrimSpace(runGit(t, repoRoot, "rev-parse", "HEAD"))

	withWorkingDir(t, repoRoot, func() {
		stdout, stderr, exitCode := executeCLIWithInput("Trying to sync while match integrity is broken.\n", "sync", "runtime:session-lifecycle", "--checkpoint", "HEAD")
		if exitCode == 0 {
			t.Fatalf("expected failure, stdout=%s", stdout)
		}
		if stderr != "" {
			t.Fatalf("stderr = %q", stderr)
		}

		assertContractFixture(t, stdout, map[string]string{"__HEAD_SHA__": newHead})
	})
}

func TestContract_RevBump_Success(t *testing.T) {
	repoRoot := copyFixtureRepoWithRegistry(t, "verified-spec")
	// Add an unbumped closed delta (D-002 repair, not in changelog) so the rev bump gate passes
	trackingPath := filepath.Join(repoRoot, ".specs", "runtime", "session-lifecycle.yaml")
	replaceInFile(t, trackingPath, "    notes: Multi-agent implementation split between runtime and workflow work\n", "    notes: Multi-agent implementation split between runtime and workflow work\n  - id: D-002\n    area: Session timeout\n    intent: repair\n    status: closed\n    origin_checkpoint: 151fab7af14cd163a5da5e60fca7cb8891b08d6a\n    current: Requirement needs re-verification\n    target: Re-verify after repair\n    notes: Unbumped delta for rev bump test\n    affects_requirements:\n      - REQ-001\n")
	replaceSessionLifecycleDoc(t, repoRoot,
		contractRequirementSection("Compensation stage 4 failure cleanup", contractRequirementBlock("@runtime @e2e", "Compensation stage 4 failure cleanup", "Cleanup runs after stage 4 failure", "stage 4 fails during compensation", "recovery completes", "cleanup steps run in documented order"))+
			"\n## Review Notes\n\nUpdated compensation cleanup notes.\n")
	headSHA := initGitRepo(t, repoRoot)

	withWorkingDir(t, repoRoot, func() {
		stdout, stderr, exitCode := executeCLIWithInput("Closed the compensation cleanup work and synced the design doc.\n", "rev", "bump", "runtime:session-lifecycle", "--checkpoint", "HEAD")
		if exitCode != 0 {
			t.Fatalf("exit code = %d, stderr=%s stdout=%s", exitCode, stderr, stdout)
		}
		if stderr != "" {
			t.Fatalf("stderr = %q", stderr)
		}

		assertContractFixture(t, stdout, map[string]string{"__HEAD_SHA__": headSHA})
	})
}

func TestContract_Sync_Success(t *testing.T) {
	repoRoot := copyFixtureRepoWithRegistry(t, "verified-spec")
	_ = initGitRepo(t, repoRoot)
	writeCLIAdjacentTestFile(t, filepath.Join(repoRoot, "runtime", "src", "domain", "session_execution", "handler.py"), []byte("def handle():\n    return 'sync'\n"))
	commitAllChangesAtDate(t, repoRoot, "2026-03-31T09:30:00Z", "code drift")
	newHead := strings.TrimSpace(runGit(t, repoRoot, "rev-parse", "HEAD"))

	withWorkingDir(t, repoRoot, func() {
		stdout, stderr, exitCode := executeCLIWithInput("Spec reviewed and unchanged.\n", "sync", "runtime:session-lifecycle", "--checkpoint", "HEAD")
		if exitCode != 0 {
			t.Fatalf("exit code = %d, stderr=%s stdout=%s", exitCode, stderr, stdout)
		}
		if stderr != "" {
			t.Fatalf("stderr = %q", stderr)
		}

		assertContractFixture(t, stdout, map[string]string{"__HEAD_SHA__": newHead})
	})
}

func TestContract_ConfigAddTag_Success(t *testing.T) {
	repoRoot := copyFixtureRepoWithRegistry(t, "ready-spec")

	withWorkingDir(t, repoRoot, func() {
		stdout, stderr, exitCode := executeCLI("config", "add-tag", "adapter")
		if exitCode != 0 {
			t.Fatalf("exit code = %d, stderr=%s stdout=%s", exitCode, stderr, stdout)
		}
		if stderr != "" {
			t.Fatalf("stderr = %q", stderr)
		}

		assertContractFixture(t, stdout, nil)
	})
}

func TestContract_ConfigRemoveTag_Success(t *testing.T) {
	repoRoot := copyFixtureRepoWithRegistry(t, "ready-spec")

	withWorkingDir(t, repoRoot, func() {
		stdout, stderr, exitCode := executeCLI("config", "remove-tag", "domain")
		if exitCode != 0 {
			t.Fatalf("exit code = %d, stderr=%s stdout=%s", exitCode, stderr, stdout)
		}
		if stderr != "" {
			t.Fatalf("stderr = %q", stderr)
		}

		assertContractFixture(t, stdout, nil)
	})
}

func TestContract_ConfigAddPrefix_Success(t *testing.T) {
	repoRoot := copyFixtureRepoWithRegistry(t, "ready-spec")

	withWorkingDir(t, repoRoot, func() {
		stdout, stderr, exitCode := executeCLI("config", "add-prefix", "runtime/src/application")
		if exitCode != 0 {
			t.Fatalf("exit code = %d, stderr=%s stdout=%s", exitCode, stderr, stdout)
		}
		if stderr != "" {
			t.Fatalf("stderr = %q", stderr)
		}

		assertContractFixture(t, stdout, nil)
	})
}

func TestContract_ConfigRemovePrefix_Success(t *testing.T) {
	repoRoot := copyFixtureRepoWithRegistry(t, "ready-spec")

	withWorkingDir(t, repoRoot, func() {
		stdout, stderr, exitCode := executeCLI("config", "remove-prefix", "runtime/src/")
		if exitCode != 0 {
			t.Fatalf("exit code = %d, stderr=%s stdout=%s", exitCode, stderr, stdout)
		}
		if stderr != "" {
			t.Fatalf("stderr = %q", stderr)
		}

		assertContractFixture(t, stdout, nil)
	})
}

func TestContract_CharterRemoveSpec_Success(t *testing.T) {
	repoRoot := copyFixtureRepoWithRegistry(t, "ready-spec")

	withWorkingDir(t, repoRoot, func() {
		stdout, stderr, exitCode := executeCLI("charter", "remove-spec", "runtime", "session-lifecycle")
		if exitCode != 0 {
			t.Fatalf("exit code = %d, stderr=%s stdout=%s", exitCode, stderr, stdout)
		}
		if stderr != "" {
			t.Fatalf("stderr = %q", stderr)
		}

		assertContractFixture(t, stdout, nil)
	})
}

func TestContract_Context_SpecNotFound(t *testing.T) {
	repoRoot := copyFixtureRepoWithRegistry(t, "ready-spec")

	withWorkingDir(t, repoRoot, func() {
		stdout, stderr, exitCode := executeCLI("context", "runtime:missing-spec")
		if exitCode != 0 {
			t.Fatalf("exit code = %d, stderr=%s stdout=%s", exitCode, stderr, stdout)
		}
		if stderr != "" {
			t.Fatalf("stderr = %q", stderr)
		}

		assertContractFixture(t, stdout, nil)
	})
}

func TestContract_Context_FileNoMatch(t *testing.T) {
	repoRoot := copyFixtureRepoWithRegistry(t, "ready-spec")
	if err := os.MkdirAll(filepath.Join(repoRoot, "adapters", "src", "http"), 0o755); err != nil {
		t.Fatalf("mkdir adapters/src/http: %v", err)
	}
	writeCLIAdjacentTestFile(t, filepath.Join(repoRoot, "adapters", "src", "http", "client.py"), []byte("pass\n"))

	withWorkingDir(t, repoRoot, func() {
		stdout, stderr, exitCode := executeCLI("context", "--file", "adapters/src/http/client.py")
		if exitCode != 0 {
			t.Fatalf("exit code = %d, stderr=%s stdout=%s", exitCode, stderr, stdout)
		}
		if stderr != "" {
			t.Fatalf("stderr = %q", stderr)
		}

		assertContractFixture(t, stdout, nil)
	})
}

func TestContract_Context_FileAmbiguous(t *testing.T) {
	repoRoot := t.TempDir()
	if err := os.MkdirAll(filepath.Join(repoRoot, ".specs", "runtime"), 0o755); err != nil {
		t.Fatalf("mkdir specs: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(repoRoot, "runtime", "src", "domain", "shared"), 0o755); err != nil {
		t.Fatalf("mkdir shared: %v", err)
	}
	writeCLIAdjacentTestFile(t, filepath.Join(repoRoot, "runtime", "src", "domain", "shared", "transport.py"), []byte("pass\n"))
	writeProjectConfigFixture(t, repoRoot, "gherkin_tags:\n  - runtime\nsource_prefixes:\n  - runtime/src/\nformats: {}\n")
	writeCLIAdjacentTestFile(t, filepath.Join(repoRoot, ".specs", "runtime", "CHARTER.yaml"), []byte(`name: runtime
title: Runtime System
description: Specs for runtime control-plane and data-plane behavior
groups:
  - key: recovery
    title: Recovery
    order: 10
specs:
  - slug: first-owner
    group: recovery
    order: 20
    depends_on: []
    notes: First owner
  - slug: second-owner
    group: recovery
    order: 30
    depends_on: []
    notes: Second owner
`))
	for _, slug := range []string{"first-owner", "second-owner"} {
		if err := os.MkdirAll(filepath.Join(repoRoot, "runtime", "src", "domain", "shared", slug), 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", slug, err)
		}
		writeCLIAdjacentTestFile(t, filepath.Join(repoRoot, ".specs", "runtime", slug+".yaml"), []byte(`slug: `+slug+`
charter: runtime
title: `+slug+`
status: draft
rev: 1
created: 2026-03-05
updated: 2026-03-30
last_verified_at: 2026-03-28
checkpoint: a1b2c3f
tags:
  - runtime
documents:
  primary: runtime/src/domain/shared/`+slug+`/SPEC.md
scope:
  - runtime/src/domain/shared/
deltas: []
requirements: []
changelog: []
`))
		writeCLIAdjacentTestFile(t, filepath.Join(repoRoot, "runtime", "src", "domain", "shared", slug, "SPEC.md"), []byte("---\nspec: "+slug+"\ncharter: runtime\n---\n# "+slug+"\n"))
	}
	initGitRepoAtDate(t, repoRoot, "2026-03-28T12:00:00Z")

	withWorkingDir(t, repoRoot, func() {
		stdout, stderr, exitCode := executeCLI("context", "--file", "runtime/src/domain/shared/transport.py")
		if exitCode != 0 {
			t.Fatalf("exit code = %d, stderr=%s stdout=%s", exitCode, stderr, stdout)
		}
		if stderr != "" {
			t.Fatalf("stderr = %q", stderr)
		}

		assertContractFixture(t, stdout, nil)
	})
}

func contractActiveRequirementRepo(t *testing.T) string {
	t.Helper()

	repoRoot := copyFixtureRepoWithRegistry(t, "active-spec")
	if err := os.MkdirAll(filepath.Join(repoRoot, "runtime", "tests", "domain"), 0o755); err != nil {
		t.Fatalf("mkdir runtime/tests/domain: %v", err)
	}
	replaceSessionLifecycleDoc(t, repoRoot, contractRequirementSection("Compensation stage 4 failure cleanup", contractRequirementBlock("@runtime @e2e", "Compensation stage 4 failure cleanup", "Cleanup runs after stage 4 failure", "stage 4 fails during compensation", "recovery completes", "cleanup steps run in documented order")))
	replaceTrackedRequirementBlockOnly(t, repoRoot, "active")
	writeCLIAdjacentTestFile(t, filepath.Join(repoRoot, "runtime", "tests", "domain", "test_compensation_cleanup.py"), []byte("def test_cleanup():\n    assert True\n"))
	return repoRoot
}

func contractChangeDeltaRepo(t *testing.T) string {
	t.Helper()

	repoRoot := contractActiveRequirementRepo(t)
	appendDelta(t, repoRoot, `  - id: D-002
    area: Compensation cleanup rewrite
    intent: change
    status: open
    origin_checkpoint: a1b2c3f
    current: Cleanup wording is outdated
    target: Replace the tracked cleanup contract
    notes: Match the new requirement block exactly
    affects_requirements:
      - REQ-001
    updates:
      - replace_requirement
`)
	return repoRoot
}

func contractReplaceFlowRepo(t *testing.T) string {
	t.Helper()

	repoRoot := contractChangeDeltaRepo(t)
	replaceSessionLifecycleDoc(t, repoRoot,
		contractRequirementSection("Compensation stage 4 failure cleanup", contractRequirementBlock("@runtime @e2e", "Compensation stage 4 failure cleanup", "Cleanup runs after stage 4 failure", "stage 4 fails during compensation", "recovery completes", "cleanup steps run in documented order"))+
			"\n"+
			contractRequirementSection("Compensation stage 4 failure cleanup replacement", contractRequirementBlock("@runtime @e2e", "Compensation stage 4 failure cleanup replacement", "Cleanup runs after stage 4 failure", "stage 4 fails during compensation", "recovery completes", "cleanup steps run in documented order")))
	return repoRoot
}

func contractInactiveSupersededRequirementRepo(t *testing.T) string {
	t.Helper()

	repoRoot := copyFixtureRepoWithRegistry(t, "verified-spec")
	replaceSessionLifecycleDoc(t, repoRoot,
		contractRequirementSection("Compensation stage 4 failure cleanup", contractRequirementBlock("@runtime @e2e", "Compensation stage 4 failure cleanup", "Cleanup runs after stage 4 failure", "stage 4 fails during compensation", "recovery completes", "cleanup steps run in documented order"))+
			"\n"+
			contractRequirementSection("Compensation stage 4 failure cleanup replacement", contractRequirementBlock("@runtime @e2e", "Compensation stage 4 failure cleanup replacement", "Cleanup runs after stage 4 failure", "stage 4 fails during compensation", "recovery completes", "cleanup steps run in documented order")),
	)
	writeCLIAdjacentTestFile(t, filepath.Join(repoRoot, "runtime", "tests", "domain", "test_compensation_cleanup_replacement.py"), []byte("def test_cleanup_replacement():\n    assert True\n"))

	trackingPath := filepath.Join(repoRoot, ".specs", "runtime", "session-lifecycle.yaml")
	data, err := os.ReadFile(trackingPath)
	if err != nil {
		t.Fatalf("read tracking file: %v", err)
	}
	checkpoint := "a1b2c3f"
	for _, line := range strings.Split(string(data), "\n") {
		if strings.HasPrefix(line, "checkpoint: ") {
			checkpoint = strings.TrimSpace(strings.TrimPrefix(line, "checkpoint: "))
			break
		}
	}

	replacement := fmt.Sprintf(`slug: session-lifecycle
charter: runtime
title: Session Lifecycle
status: verified
rev: 4
created: 2026-03-05
updated: 2026-03-30
last_verified_at: 2026-03-30
checkpoint: %s
tags:
  - runtime
  - domain
documents:
  primary: runtime/src/domain/session_execution/SPEC.md
scope:
  - runtime/src/domain/session_execution/
  - runtime/src/application/commands/
deltas:
  - id: D-001
    area: Compensation stage 4
    status: closed
    origin_checkpoint: %s
    current: Stage 4 compensation exists in code but failure ordering is unclear
    target: Document ordering and verify failure cleanup
    notes: Multi-agent implementation split between runtime and workflow work
  - id: D-002
    area: Compensation cleanup rewrite
    intent: change
    status: closed
    origin_checkpoint: %s
    current: Cleanup wording is outdated
    target: Replace the tracked cleanup contract
    notes: Replacement requirement is verified; superseded predecessor remains historical only
    affects_requirements:
      - REQ-001
    updates:
      - replace_requirement
requirements:
  - id: REQ-001
    title: Compensation stage 4 failure cleanup
    tags:
      - runtime
      - e2e
    test_files: []
    gherkin: |
      @runtime @e2e
      Feature: Compensation stage 4 failure cleanup
    lifecycle: superseded
    verification: unverified
    introduced_by: D-001
    superseded_by: REQ-002
  - id: REQ-002
    title: Compensation stage 4 failure cleanup replacement
    tags:
      - runtime
      - e2e
    test_files:
      - runtime/tests/domain/test_compensation_cleanup_replacement.py
    gherkin: |
      @runtime @e2e
      Feature: Compensation stage 4 failure cleanup replacement
    lifecycle: active
    verification: verified
    introduced_by: D-002
    supersedes: REQ-001
changelog:
  - rev: 2
    date: 2026-03-28
    deltas_opened:
      - D-001
    deltas_closed:
      - D-001
    reqs_added:
      - REQ-001
    reqs_verified:
      - REQ-001
    summary: Closed the compensation cleanup work
  - rev: 3
    date: 2026-03-30
    deltas_opened:
      - D-002
    deltas_closed:
      - D-002
    reqs_added:
      - REQ-002
    reqs_verified:
      - REQ-002
    summary: Verified the replacement cleanup requirement while preserving the superseded predecessor as history
`, checkpoint, checkpoint, checkpoint)
	writeCLIAdjacentTestFile(t, trackingPath, []byte(replacement))
	return repoRoot
}

func contractRemoveDeltaRepo(t *testing.T) string {
	t.Helper()

	repoRoot := contractActiveRequirementRepo(t)
	appendDelta(t, repoRoot, `  - id: D-002
    area: Compensation cleanup removal
    intent: remove
    status: open
    origin_checkpoint: a1b2c3f
    current: Cleanup behavior is still documented
    target: Remove the obsolete cleanup contract
    notes: Behavior is intentionally removed
    affects_requirements:
      - REQ-001
    updates:
      - withdraw_requirement
`)
	return repoRoot
}

func contractRepairDeltaRepo(t *testing.T) string {
	t.Helper()

	repoRoot := contractActiveRequirementRepo(t)
	replaceInFile(t, filepath.Join(repoRoot, ".specs", "runtime", "session-lifecycle.yaml"), "verification: unverified", "verification: verified")
	replaceInFile(t, filepath.Join(repoRoot, ".specs", "runtime", "session-lifecycle.yaml"), "test_files: []", "test_files:\n      - runtime/tests/domain/test_compensation_cleanup.py")
	appendDelta(t, repoRoot, `  - id: D-002
    area: Compensation cleanup repair
    intent: repair
    status: open
    origin_checkpoint: a1b2c3f
    current: Evidence is stale after a regression
    target: Re-verify the existing cleanup behavior
    notes: Same requirement identity, fresh evidence needed
    affects_requirements:
      - REQ-001
    updates:
      - stale_requirement
`)
	replaceInFile(t, filepath.Join(repoRoot, ".specs", "runtime", "session-lifecycle.yaml"), "status: ready", "status: active")
	return repoRoot
}

func contractRepairTerminalRepo(t *testing.T) string {
	t.Helper()

	repoRoot := contractRepairDeltaRepo(t)
	replaceInFile(t, filepath.Join(repoRoot, ".specs", "runtime", "session-lifecycle.yaml"), "status: in-progress", "status: closed")
	return repoRoot
}

func contractRefreshRepo(t *testing.T) string {
	t.Helper()

	repoRoot := contractActiveRequirementRepo(t)
	replaceSessionLifecycleDoc(t, repoRoot, contractRequirementSection("Compensation stage 4 failure cleanup", contractRequirementBlock("@runtime @e2e\n@critical", "Compensation stage 4 failure cleanup", "Cleanup runs after stage 4 failure", "stage 4 fails during compensation", "recovery completes", "cleanup steps run in documented order")))
	writeProjectConfigFixture(t, repoRoot, "gherkin_tags:\n  - runtime\n  - critical\nsource_prefixes:\n  - runtime/src/\nformats: {}\n")
	return repoRoot
}

func contractVerifiedMatchBlockingRepo(t *testing.T) string {
	t.Helper()

	repoRoot := copyFixtureRepoWithRegistry(t, "verified-spec")
	replaceTrackedRequirementBlockOnly(t, repoRoot, "verified")
	replaceSessionLifecycleDoc(t, repoRoot, contractRequirementSection("Compensation stage 4 failure cleanup", contractRequirementBlock("@runtime @e2e\n@critical", "Compensation stage 4 failure cleanup", "Cleanup runs after stage 4 failure", "stage 4 fails during compensation", "recovery completes", "cleanup steps run in documented order")))
	writeProjectConfigFixture(t, repoRoot, "gherkin_tags:\n  - runtime\n  - critical\nsource_prefixes:\n  - runtime/src/\nformats: {}\n")
	return repoRoot
}

func contractContextExactMatchDriftRepo(t *testing.T) string {
	t.Helper()

	repoRoot := copyFixtureRepoWithRegistry(t, "verified-spec")
	replaceTrackedRequirementMatchIssueSet(t, repoRoot)
	replaceSessionLifecycleDoc(t, repoRoot,
		contractRequirementSection("Compensation stage 4 failure cleanup", contractRequirementBlock("@runtime @e2e\n@critical", "Compensation stage 4 failure cleanup", "Cleanup runs after stage 4 failure", "stage 4 fails during compensation", "recovery completes", "cleanup steps run in documented order"))+
			"\n"+
			contractRequirementSection("Duplicate requirement", contractRequirementBlock("@runtime", "Duplicate requirement", "Duplicate requirement remains visible", "a duplicate block is rendered", "the spec is parsed", "duplicate tracking is detected"))+
			"\n"+
			contractRequirementSection("Duplicate requirement", contractRequirementBlock("@runtime", "Duplicate requirement", "Duplicate requirement remains visible", "a duplicate block is rendered", "the spec is parsed", "duplicate tracking is detected")),
	)
	writeProjectConfigFixture(t, repoRoot, "gherkin_tags:\n  - runtime\n  - critical\nsource_prefixes:\n  - runtime/src/\nformats: {}\n")
	commitAllChangesAtDate(t, repoRoot, "2026-03-31T09:30:00Z", "exact match drift")
	headSHA := strings.TrimSpace(runGit(t, repoRoot, "rev-parse", "HEAD"))
	replaceInFile(t, filepath.Join(repoRoot, ".specs", "runtime", "session-lifecycle.yaml"), "checkpoint: a1b2c3f", "checkpoint: "+headSHA)
	replaceInFile(t, filepath.Join(repoRoot, ".specs", "runtime", "session-lifecycle.yaml"), "last_verified_at: 2026-03-28", "last_verified_at: 2026-03-31")
	replaceInFile(t, filepath.Join(repoRoot, ".specs", "runtime", "session-lifecycle.yaml"), "updated: 2026-03-30", "updated: 2026-03-31")
	return repoRoot
}

func contractContextStaleRequirementRepo(t *testing.T) string {
	t.Helper()

	repoRoot := copyFixtureRepoWithRegistry(t, "active-spec")
	replaceTrackedRequirementBlockOnly(t, repoRoot, "active")
	replaceInFile(t, filepath.Join(repoRoot, ".specs", "runtime", "session-lifecycle.yaml"), "verification: unverified", "verification: stale")
	return repoRoot
}

func contractDriftedDesignDocRepo(t *testing.T) string {
	t.Helper()

	repoRoot := copyFixtureRepoWithRegistry(t, "verified-spec")
	replaceTrackedRequirementBlockOnly(t, repoRoot, "verified")
	headSHA := strings.TrimSpace(runGit(t, repoRoot, "rev-parse", "HEAD"))
	replaceInFile(t, filepath.Join(repoRoot, ".specs", "runtime", "session-lifecycle.yaml"), "checkpoint: a1b2c3f", "checkpoint: "+headSHA)
	replaceSessionLifecycleDoc(t, repoRoot,
		contractRequirementSection("Compensation stage 4 failure cleanup", contractRequirementBlock("@runtime @e2e", "Compensation stage 4 failure cleanup", "Cleanup runs after stage 4 failure", "stage 4 fails during compensation", "recovery completes", "cleanup steps run in documented order"))+
			"\n## Review Notes\n\nReviewed recovery ordering prose without changing the tracked requirement contract.\n")
	commitAllChangesAtDate(t, repoRoot, "2026-03-31T09:30:00Z", "design doc drift")
	return repoRoot
}

func appendDelta(t *testing.T, repoRoot, deltaYAML string) {
	t.Helper()

	trackingPath := filepath.Join(repoRoot, ".specs", "runtime", "session-lifecycle.yaml")
	data, err := os.ReadFile(trackingPath)
	if err != nil {
		t.Fatalf("read tracking file: %v", err)
	}
	updated := strings.Replace(string(data), "requirements:", strings.TrimRight(deltaYAML, "\n")+"\nrequirements:", 1)
	updated = strings.Replace(updated, "status: active", "status: ready", 1)
	updated = strings.Replace(updated, "status: verified", "status: ready", 1)
	writeCLIAdjacentTestFile(t, trackingPath, []byte(updated))
}

func replaceTrackedRequirementBlockOnly(t *testing.T, repoRoot, state string) {
	t.Helper()

	trackingPath := filepath.Join(repoRoot, ".specs", "runtime", "session-lifecycle.yaml")
	data, err := os.ReadFile(trackingPath)
	if err != nil {
		t.Fatalf("read tracking file: %v", err)
	}
	replacement := fmt.Sprintf(`requirements:
  - id: REQ-001
    title: Compensation stage 4 failure cleanup
    tags:
      - runtime
      - e2e
    test_files:%s
    gherkin: |
      @runtime @e2e
      Feature: Compensation stage 4 failure cleanup
    lifecycle: active
    verification: %s
    introduced_by: D-001
`, contractRequirementTestFiles(state), contractRequirementVerification(state))
	content := string(data)
	start := strings.Index(content, "requirements:\n")
	end := strings.Index(content, "changelog:\n")
	if start < 0 || end < 0 || end <= start {
		t.Fatalf("replace requirements section in %s", trackingPath)
	}
	writeCLIAdjacentTestFile(t, trackingPath, []byte(content[:start]+replacement+content[end:]))
}

func replaceTrackedRequirementMatchIssueSet(t *testing.T, repoRoot string) {
	t.Helper()

	if err := os.MkdirAll(filepath.Join(repoRoot, "runtime", "tests", "domain"), 0o755); err != nil {
		t.Fatalf("mkdir test dir: %v", err)
	}
	for path, body := range map[string]string{
		filepath.Join(repoRoot, "runtime", "tests", "domain", "test_compensation_cleanup.py"):  "def test_compensation_cleanup():\n    assert True\n",
		filepath.Join(repoRoot, "runtime", "tests", "domain", "test_missing_requirement.py"):   "def test_missing_requirement():\n    assert True\n",
		filepath.Join(repoRoot, "runtime", "tests", "domain", "test_duplicate_requirement.py"): "def test_duplicate_requirement():\n    assert True\n",
	} {
		writeCLIAdjacentTestFile(t, path, []byte(body))
	}

	trackingPath := filepath.Join(repoRoot, ".specs", "runtime", "session-lifecycle.yaml")
	data, err := os.ReadFile(trackingPath)
	if err != nil {
		t.Fatalf("read tracking file: %v", err)
	}
	replacement := `requirements:
  - id: REQ-001
    title: Compensation stage 4 failure cleanup
    tags:
      - runtime
      - e2e
    test_files:
      - runtime/tests/domain/test_compensation_cleanup.py
    gherkin: |
      @runtime @e2e
      Feature: Compensation stage 4 failure cleanup
    lifecycle: active
    verification: verified
    introduced_by: D-001
  - id: REQ-002
    title: Missing requirement
    tags:
      - runtime
    test_files:
      - runtime/tests/domain/test_missing_requirement.py
    gherkin: |
      @runtime
      Feature: Missing requirement
    lifecycle: active
    verification: verified
    introduced_by: D-001
  - id: REQ-003
    title: Duplicate requirement
    tags:
      - runtime
    test_files:
      - runtime/tests/domain/test_duplicate_requirement.py
    gherkin: |
      @runtime
      Feature: Duplicate requirement
    lifecycle: active
    verification: verified
    introduced_by: D-001
`
	content := string(data)
	start := strings.Index(content, "requirements:\n")
	end := strings.Index(content, "changelog:\n")
	if start < 0 || end < 0 || end <= start {
		t.Fatalf("replace requirements section in %s", trackingPath)
	}
	writeCLIAdjacentTestFile(t, trackingPath, []byte(content[:start]+replacement+content[end:]))
}

func contractRequirementTestFiles(state string) string {
	if state == "verified" {
		return "\n      - runtime/tests/domain/test_compensation_cleanup.py"
	}
	return " []"
}

func contractRequirementVerification(state string) string {
	if state == "verified" {
		return "verified"
	}
	return "unverified"
}

func replaceSessionLifecycleDoc(t *testing.T, repoRoot, sections string) {
	t.Helper()

	docPath := filepath.Join(repoRoot, "runtime", "src", "domain", "session_execution", "SPEC.md")
	content := "---\nspec: session-lifecycle\ncharter: runtime\n---\n# Session Lifecycle\n\n" + strings.TrimSpace(sections) + "\n"
	writeCLIAdjacentTestFile(t, docPath, []byte(content))
}

func contractRequirementSection(title, requirementBlock string) string {
	return "## Requirement: " + title + "\n\n```gherkin requirement\n" + strings.TrimSpace(requirementBlock) + "\n```\n\n### Scenarios\n\n```gherkin scenario\nScenario: Cleanup runs after stage 4 failure\n  Given stage 4 fails during compensation\n  When recovery completes\n  Then cleanup steps run in documented order\n```\n"
}

func contractRequirementBlock(tags, title, scenarioTitle, given, when, then string) string {
	_ = scenarioTitle
	_ = given
	_ = when
	_ = then
	return strings.Join([]string{tags, "Feature: " + title}, "\n")
}

func contractReplacementRequirementBlock() string {
	return "@runtime @e2e\nFeature: Compensation stage 4 failure cleanup replacement"
}
