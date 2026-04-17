package application

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestContract_Context_CleanNone(t *testing.T) {
	repoRoot := copyApplicationFixtureRepo(t, "verified-spec")
	initGitRepoAtDate(t, repoRoot, "2026-03-28T12:00:00Z")
	headSHA := strings.TrimSpace(runGitAtDate(t, repoRoot, "2026-03-28T12:00:00Z", "rev-parse", "HEAD"))
	service := newApplicationContractService(repoRoot)

	placeholders := contractPlaceholders()
	placeholders["__HEAD_SHA__"] = headSHA
	assertReadContractFixtureCall(t, placeholders, func() (any, []any, error) {
		return service.ReadContext("runtime:session-lifecycle", "")
	})
}

func TestContract_Context_UncommittedGovernedChanges(t *testing.T) {
	repoRoot := copyApplicationFixtureRepo(t, "verified-spec")
	initGitRepoAtDate(t, repoRoot, "2026-03-28T12:00:00Z")
	headSHA := strings.TrimSpace(runGitAtDate(t, repoRoot, "2026-03-28T12:00:00Z", "rev-parse", "HEAD"))
	replaceFileText(t, filepath.Join(repoRoot, "runtime", "src", "domain", "session_execution", "SPEC.md"), "# Session Lifecycle", "# Session Lifecycle\n\n## Dirty Drift\n\nPending change.\n")
	service := newApplicationContractService(repoRoot)

	placeholders := contractPlaceholders()
	placeholders["__HEAD_SHA__"] = headSHA
	assertReadContractFixtureCall(t, placeholders, func() (any, []any, error) {
		return service.ReadContext("runtime:session-lifecycle", "")
	})
}

func TestContract_Context_DriftedDesignDoc(t *testing.T) {
	repoRoot := copyApplicationFixtureRepo(t, "verified-spec")
	initGitRepoAtDate(t, repoRoot, "2026-03-28T12:00:00Z")
	headSHA := strings.TrimSpace(runGitAtDate(t, repoRoot, "2026-03-28T12:00:00Z", "rev-parse", "HEAD"))
	replaceFileText(t, filepath.Join(repoRoot, "runtime", "src", "domain", "session_execution", "SPEC.md"), "# Session Lifecycle", "# Session Lifecycle\n\n## Drift Review\n\nUpdated after sync.\n")
	runGitAtDate(t, repoRoot, "2026-03-30T09:30:00Z", "add", ".")
	runGitAtDate(t, repoRoot, "2026-03-30T09:30:00Z", "commit", "-m", "drifted scope change")
	service := newApplicationContractService(repoRoot)

	placeholders := contractPlaceholders()
	placeholders["__HEAD_SHA__"] = headSHA
	assertReadContractFixtureCall(t, placeholders, func() (any, []any, error) {
		return service.ReadContext("runtime:session-lifecycle", "")
	})
}

func TestContract_Context_DriftedScopeCode(t *testing.T) {
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
		return service.ReadContext("runtime:session-lifecycle", "")
	})
}

func TestContract_Context_TrackedDriftContinuation(t *testing.T) {
	repoRoot := contractTrackedDriftRepo(t)
	headSHA := strings.TrimSpace(runGitAtDate(t, repoRoot, "2026-03-28T12:00:00Z", "rev-parse", "HEAD"))
	service := newApplicationContractService(repoRoot)

	placeholders := contractPlaceholders()
	placeholders["__HEAD_SHA__"] = headSHA
	assertReadContractFixtureCall(t, placeholders, func() (any, []any, error) {
		return service.ReadContext("runtime:session-lifecycle", "")
	})
}

func TestContract_Context_CheckpointUnavailable(t *testing.T) {
	repoRoot := copyApplicationFixtureRepo(t, "verified-spec")
	service := newApplicationContractService(repoRoot)

	assertReadContractFixtureCall(t, contractPlaceholders(), func() (any, []any, error) {
		return service.ReadContext("runtime:session-lifecycle", "")
	})
}

func TestContract_Context_RegistrySuccess(t *testing.T) {
	repoRoot := copyApplicationFixtureRepo(t, "ready-spec")
	service := newApplicationContractService(repoRoot)

	assertReadContractFixtureCall(t, contractPlaceholders(), func() (any, []any, error) {
		return service.ReadContext("", "")
	})
}

func TestContract_Context_CharterSuccess(t *testing.T) {
	repoRoot := copyApplicationFixtureRepo(t, "charter-dag")
	service := newApplicationContractService(repoRoot)

	assertReadContractFixtureCall(t, contractPlaceholders(), func() (any, []any, error) {
		return service.ReadContext("runtime", "")
	})
}

func TestContract_Context_CharterNotFound(t *testing.T) {
	repoRoot := copyApplicationFixtureRepo(t, "ready-spec")
	service := newApplicationContractService(repoRoot)

	assertReadContractFixtureCall(t, contractPlaceholders(), func() (any, []any, error) {
		return service.ReadContext("missing", "")
	})
}

func TestContract_Context_SpecNotFound_CharterMissing(t *testing.T) {
	repoRoot := copyApplicationFixtureRepo(t, "ready-spec")
	service := newApplicationContractService(repoRoot)

	assertReadContractFixtureCall(t, contractPlaceholders(), func() (any, []any, error) {
		return service.ReadContext("missing:missing-spec", "")
	})
}

func TestContract_Context_SpecNotFound_CharterExists(t *testing.T) {
	repoRoot := copyApplicationFixtureRepo(t, "ready-spec")
	service := newApplicationContractService(repoRoot)

	assertReadContractFixtureCall(t, contractPlaceholders(), func() (any, []any, error) {
		return service.ReadContext("runtime:missing-spec", "")
	})
}

func TestContract_Context_RefreshChooser(t *testing.T) {
	repoRoot := contractRefreshRepo(t)
	service := newApplicationContractService(repoRoot)

	assertReadContractFixtureCall(t, contractPlaceholders(), func() (any, []any, error) {
		return service.ReadContext("runtime:session-lifecycle", "")
	})
}

func TestContract_Context_RepairChooser(t *testing.T) {
	repoRoot := contractStaleRequirementRepo(t)
	service := newApplicationContractService(repoRoot)

	assertReadContractFixtureCall(t, contractPlaceholders(), func() (any, []any, error) {
		return service.ReadContext("runtime:session-lifecycle", "")
	})
}

func TestContract_Context_FileMatched(t *testing.T) {
	repoRoot := copyApplicationFixtureRepo(t, "ready-spec")
	service := newApplicationContractService(repoRoot)

	assertReadContractFixtureCall(t, contractPlaceholders(), func() (any, []any, error) {
		return service.ReadContext("", "./runtime/src/domain/session_execution/../session_execution/SPEC.md")
	})
}

func TestContract_Context_FileUnmatched(t *testing.T) {
	repoRoot := copyApplicationFixtureRepo(t, "ready-spec")
	if err := os.MkdirAll(filepath.Join(repoRoot, "adapters", "src", "http"), 0o755); err != nil {
		t.Fatalf("mkdir adapters/src/http: %v", err)
	}
	writeApplicationTestFile(t, filepath.Join(repoRoot, "adapters", "src", "http", "client.py"), []byte("pass\n"))
	service := newApplicationContractService(repoRoot)

	assertReadContractFixtureCall(t, contractPlaceholders(), func() (any, []any, error) {
		return service.ReadContext("", "adapters/src/http/client.py")
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
	writeApplicationTestFile(t, filepath.Join(repoRoot, "runtime", "src", "domain", "shared", "transport.py"), []byte("pass\n"))
	writeApplicationTestFile(t, filepath.Join(repoRoot, ".specs", "runtime", "CHARTER.yaml"), []byte(`name: runtime
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
		writeApplicationTestFile(t, filepath.Join(repoRoot, ".specs", "runtime", slug+".yaml"), []byte(`slug: `+slug+`
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
	}
	service := newApplicationContractService(repoRoot)

	assertReadContractFixtureCall(t, contractPlaceholders(), func() (any, []any, error) {
		return service.ReadContext("", "runtime/src/domain/shared/transport.py")
	})
}

func TestContract_Config_Read(t *testing.T) {
	repoRoot := copyApplicationFixtureRepo(t, "ready-spec")
	service := newApplicationContractService(repoRoot)

	assertReadContractFixtureCall(t, contractPlaceholders(), func() (any, []any, error) {
		state, err := service.ReadConfig()
		return state, nil, err
	})
}
