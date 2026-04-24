package application

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/aitoroses/specctl/internal/domain"
)

func TestContract_Hook_Integration(t *testing.T) {
	repoRoot := copyApplicationFixtureRepo(t, "charter-dag")
	if err := os.MkdirAll(filepath.Join(repoRoot, "runtime", "src", "orphan"), 0o755); err != nil {
		t.Fatalf("mkdir orphan: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(repoRoot, "runtime", "src", "domain", "session_execution"), 0o755); err != nil {
		t.Fatalf("mkdir session execution: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(repoRoot, "docs"), 0o755); err != nil {
		t.Fatalf("mkdir docs: %v", err)
	}
	writeApplicationTestFile(t, filepath.Join(repoRoot, "runtime", "src", "orphan", "worker.py"), []byte("pass\n"))
	writeApplicationTestFile(t, filepath.Join(repoRoot, "runtime", "src", "domain", "session_execution", "services.py"), []byte("pass\n"))
	writeApplicationTestFile(t, filepath.Join(repoRoot, "docs", "notes.md"), []byte("# Notes\n"))

	service := newApplicationContractService(repoRoot)
	state, err := service.ReadHook(".specs/runtime/CHARTER.yaml\n.specs/runtime/session-lifecycle.yaml\n.specs/specctl.yaml\nruntime/src/domain/session_execution/services.py\nruntime/src/orphan/worker.py\ndocs/notes.md\n")
	output := marshalReadContractOutput(t, state, nil, err)
	assertContractFixture(t, output, contractPlaceholders())
}

func TestContract_SpecCreate_InvalidPath(t *testing.T) {
	repoRoot := copyApplicationFixtureRepo(t, "ready-spec")
	if err := os.MkdirAll(filepath.Join(repoRoot, "runtime", "src", "domain", "drain_controller"), 0o755); err != nil {
		t.Fatalf("mkdir scope dir: %v", err)
	}

	service := newApplicationContractService(repoRoot)
	request := baseSpecCreateRequest("runtime:drain-controller", "runtime/src/domain/drain_controller/SPEC.md", []string{"../outside/"})
	output := marshalSpecWriteContractCall(t, func() (SpecProjection, map[string]any, []any, error) {
		return service.CreateSpec(request)
	})
	assertContractFixture(t, output, contractPlaceholders())
}

func TestContract_SpecCreate_FormatAmbiguous(t *testing.T) {
	repoRoot := copyApplicationFixtureRepo(t, "ready-spec")
	if err := os.MkdirAll(filepath.Join(repoRoot, "runtime", "src", "domain", "tests", "e2e", "session_execution"), 0o755); err != nil {
		t.Fatalf("mkdir scope dir: %v", err)
	}
	writeProjectConfigFixture(t, repoRoot, "gherkin_tags:\n  - runtime\nsource_prefixes:\n  - runtime/src/\nformats:\n  runtime-spec:\n    template: runtime/src/domain/SPEC-FORMAT.md\n    recommended_for: runtime/src/domain/**\n    description: Runtime domain spec\n  e2e-context:\n    template: runtime/tests/e2e/CONTEXT-FORMAT.md\n    recommended_for: \"**/tests/e2e/**\"\n    description: E2E context doc\n")

	service := newApplicationContractService(repoRoot)
	request := baseSpecCreateRequest(
		"runtime:session-e2e-context",
		"runtime/src/domain/tests/e2e/session_execution/CONTEXT.md",
		[]string{"runtime/src/domain/tests/e2e/session_execution/"},
	)
	output := marshalSpecWriteContractCall(t, func() (SpecProjection, map[string]any, []any, error) {
		return service.CreateSpec(request)
	})
	assertContractFixture(t, output, contractPlaceholders())
}

func TestContract_SpecCreate_FormatNotConfigured(t *testing.T) {
	repoRoot := copyApplicationFixtureRepo(t, "ready-spec")
	if err := os.MkdirAll(filepath.Join(repoRoot, "runtime", "src", "domain", "drain_controller"), 0o755); err != nil {
		t.Fatalf("mkdir scope dir: %v", err)
	}
	writeApplicationTestFile(t, filepath.Join(repoRoot, "runtime", "src", "domain", "drain_controller", "SPEC.md"), []byte("---\nspec: drain-controller\ncharter: runtime\nformat: runtime-spec\n---\n# Drain Controller\n"))

	service := newApplicationContractService(repoRoot)
	request := baseSpecCreateRequest("runtime:drain-controller", "runtime/src/domain/drain_controller/SPEC.md", []string{"runtime/src/domain/drain_controller/"})
	output := marshalSpecWriteContractCall(t, func() (SpecProjection, map[string]any, []any, error) {
		return service.CreateSpec(request)
	})
	assertContractFixture(t, output, contractPlaceholders())
}

func TestContract_SpecCreate_PrimaryDocFrontmatterInvalid(t *testing.T) {
	repoRoot := copyApplicationFixtureRepo(t, "ready-spec")
	if err := os.MkdirAll(filepath.Join(repoRoot, "runtime", "src", "domain", "drain_controller"), 0o755); err != nil {
		t.Fatalf("mkdir scope dir: %v", err)
	}
	writeApplicationTestFile(t, filepath.Join(repoRoot, "runtime", "src", "domain", "drain_controller", "SPEC.md"), []byte("---\nspec: drain-controller\ncharter runtime\n# broken\n"))

	service := newApplicationContractService(repoRoot)
	request := baseSpecCreateRequest("runtime:drain-controller", "runtime/src/domain/drain_controller/SPEC.md", []string{"runtime/src/domain/drain_controller/"})
	output := marshalSpecWriteContractCall(t, func() (SpecProjection, map[string]any, []any, error) {
		return service.CreateSpec(request)
	})
	assertContractFixture(t, output, contractPlaceholders())
}

func TestContract_SpecCreate_PrimaryDocFrontmatterMismatch(t *testing.T) {
	repoRoot := copyApplicationFixtureRepo(t, "ready-spec")
	if err := os.MkdirAll(filepath.Join(repoRoot, "runtime", "src", "domain", "drain_controller"), 0o755); err != nil {
		t.Fatalf("mkdir scope dir: %v", err)
	}
	writeApplicationTestFile(t, filepath.Join(repoRoot, "runtime", "src", "domain", "drain_controller", "SPEC.md"), []byte("---\nspec: different-spec\ncharter: runtime\n---\n# Drain Controller\n"))

	service := newApplicationContractService(repoRoot)
	request := baseSpecCreateRequest("runtime:drain-controller", "runtime/src/domain/drain_controller/SPEC.md", []string{"runtime/src/domain/drain_controller/"})
	output := marshalSpecWriteContractCall(t, func() (SpecProjection, map[string]any, []any, error) {
		return service.CreateSpec(request)
	})
	assertContractFixture(t, output, contractPlaceholders())
}

func TestContract_SpecCreate_CheckpointUnavailable(t *testing.T) {
	repoRoot := copyApplicationFixtureRepo(t, "ready-spec")
	if err := os.MkdirAll(filepath.Join(repoRoot, "runtime", "src", "domain", "drain_controller"), 0o755); err != nil {
		t.Fatalf("mkdir scope dir: %v", err)
	}

	service := newApplicationContractService(repoRoot)
	request := baseSpecCreateRequest("runtime:drain-controller", "runtime/src/domain/drain_controller/SPEC.md", []string{"runtime/src/domain/drain_controller/"})
	output := marshalSpecWriteContractCall(t, func() (SpecProjection, map[string]any, []any, error) {
		return service.CreateSpec(request)
	})
	assertContractFixture(t, output, contractPlaceholders())
}

func TestContract_DeltaAdd_AffectedRequirementsRequired(t *testing.T) {
	repoRoot := copyApplicationFixtureRepo(t, "active-spec")
	service := newApplicationContractService(repoRoot)

	output := marshalSpecWriteContractCall(t, func() (SpecProjection, map[string]any, []any, error) {
		return service.AddDelta(DeltaAddRequest{
			Target:         "runtime:session-lifecycle",
			Intent:         domain.DeltaIntentChange,
			Area:           "Compensation cleanup rewrite",
			Current:        "Current cleanup wording is outdated",
			CurrentPresent: true,
			Targets:        "Replace the tracked cleanup contract",
			TargetPresent:  true,
			Notes:          "Behavior shifted during rollout",
			NotesPresent:   true,
		})
	})
	assertContractFixture(t, output, contractPlaceholders())
}

func TestContract_DeltaAdd_MissingIntent(t *testing.T) {
	repoRoot := copyApplicationFixtureRepo(t, "ready-spec")
	service := newApplicationContractService(repoRoot)

	output := marshalSpecWriteContractCall(t, func() (SpecProjection, map[string]any, []any, error) {
		return service.AddDelta(DeltaAddRequest{
			Target:         "runtime:session-lifecycle",
			Area:           "Heartbeat timeout",
			Current:        "Current gap",
			CurrentPresent: true,
			Targets:        "Target gap",
			TargetPresent:  true,
			Notes:          "Explicitly tracked",
			NotesPresent:   true,
		})
	})
	assertContractFixture(t, output, contractPlaceholders())
}

func TestContract_DeltaAdd_InvalidRequirementState(t *testing.T) {
	repoRoot := copyApplicationFixtureRepo(t, "active-spec")
	service := newApplicationContractService(repoRoot)

	output := marshalSpecWriteContractCall(t, func() (SpecProjection, map[string]any, []any, error) {
		return service.AddDelta(DeltaAddRequest{
			Target:              "runtime:session-lifecycle",
			Intent:              domain.DeltaIntentChange,
			Area:                "Compensation cleanup rewrite",
			Current:             "Current cleanup wording is outdated",
			CurrentPresent:      true,
			Targets:             "Replace the tracked cleanup contract",
			TargetPresent:       true,
			Notes:               "Behavior shifted during rollout",
			NotesPresent:        true,
			AffectsRequirements: []string{"REQ-999"},
		})
	})
	assertContractFixture(t, output, contractPlaceholders())
}

func TestContract_DeltaAdd_SpecNotFound(t *testing.T) {
	repoRoot := copyApplicationFixtureRepo(t, "ready-spec")
	service := newApplicationContractService(repoRoot)

	output := marshalSpecWriteContractCall(t, func() (SpecProjection, map[string]any, []any, error) {
		return service.AddDelta(DeltaAddRequest{
			Target:         "runtime:missing-spec",
			Intent:         domain.DeltaIntentAdd,
			Area:           "Heartbeat timeout",
			Current:        "Current gap",
			CurrentPresent: true,
			Targets:        "Target gap",
			TargetPresent:  true,
			Notes:          "Explicitly tracked",
			NotesPresent:   true,
		})
	})
	assertContractFixture(t, output, contractPlaceholders())
}

func TestContract_DeltaAdd_AddSuccess(t *testing.T) {
	repoRoot := copyApplicationFixtureRepo(t, "verified-spec")
	service := newApplicationContractService(repoRoot)

	output := marshalSpecWriteContractCall(t, func() (SpecProjection, map[string]any, []any, error) {
		return service.AddDelta(DeltaAddRequest{
			Target:         "runtime:session-lifecycle",
			Intent:         domain.DeltaIntentAdd,
			Area:           "Tracked drift review",
			Current:        "The committed drift is not yet tracked",
			CurrentPresent: true,
			Targets:        "Track the committed design-doc change",
			TargetPresent:  true,
			Notes:          "Opened after the drift commit",
			NotesPresent:   true,
		})
	})
	assertContractFixture(t, output, contractPlaceholders())
}

func TestContract_DeltaAdd_ChangeSuccess(t *testing.T) {
	repoRoot := contractActiveRequirementRepo(t)
	service := newApplicationContractService(repoRoot)

	output := marshalSpecWriteContractCall(t, func() (SpecProjection, map[string]any, []any, error) {
		return service.AddDelta(DeltaAddRequest{
			Target:              "runtime:session-lifecycle",
			Intent:              domain.DeltaIntentChange,
			Area:                "Compensation cleanup rewrite",
			Current:             "Current cleanup wording is outdated",
			CurrentPresent:      true,
			Targets:             "Replace the tracked cleanup contract",
			TargetPresent:       true,
			Notes:               "Behavior shifted during rollout",
			NotesPresent:        true,
			AffectsRequirements: []string{"REQ-001"},
		})
	})
	assertContractFixture(t, output, contractPlaceholders())
}

func TestContract_DeltaAdd_RemoveSuccess(t *testing.T) {
	repoRoot := contractActiveRequirementRepo(t)
	service := newApplicationContractService(repoRoot)

	output := marshalSpecWriteContractCall(t, func() (SpecProjection, map[string]any, []any, error) {
		return service.AddDelta(DeltaAddRequest{
			Target:              "runtime:session-lifecycle",
			Intent:              domain.DeltaIntentRemove,
			Area:                "Compensation cleanup removal",
			Current:             "Cleanup is still part of the contract",
			CurrentPresent:      true,
			Targets:             "Remove the cleanup behavior from the contract",
			TargetPresent:       true,
			Notes:               "The behavior is being retired",
			NotesPresent:        true,
			AffectsRequirements: []string{"REQ-001"},
		})
	})
	assertContractFixture(t, output, contractPlaceholders())
}

func TestContract_DeltaAdd_RepairSuccess(t *testing.T) {
	repoRoot := contractActiveRequirementRepo(t)
	service := newApplicationContractService(repoRoot)

	output := marshalSpecWriteContractCall(t, func() (SpecProjection, map[string]any, []any, error) {
		return service.AddDelta(DeltaAddRequest{
			Target:              "runtime:session-lifecycle",
			Intent:              domain.DeltaIntentRepair,
			Area:                "Compensation cleanup repair",
			Current:             "Evidence is stale after a regression",
			CurrentPresent:      true,
			Targets:             "Re-verify the existing cleanup behavior",
			TargetPresent:       true,
			Notes:               "Same requirement identity, fresh evidence needed",
			NotesPresent:        true,
			AffectsRequirements: []string{"REQ-001"},
		})
	})
	assertContractFixture(t, output, contractPlaceholders())
}

func TestContract_DeltaAdd_RepairClosedDeltaConflict(t *testing.T) {
	repoRoot := contractActiveRequirementRepo(t)
	replaceTrackedRequirementBlockOnly(t, repoRoot, "verified")
	appendDelta(t, repoRoot, `  - id: D-002
    area: Compensation cleanup first repair
    intent: repair
    status: closed
    origin_checkpoint: a1b2c3f
    current: Evidence gap after a regression
    target: Re-verify the tracked cleanup behavior
    notes: Prior repair is historical; REQ-001 stayed active and verified
    affects_requirements:
      - REQ-001
    updates:
      - stale_requirement
`)
	replaceFileText(t, filepath.Join(repoRoot, ".specs", "runtime", "session-lifecycle.yaml"), "status: ready", "status: active")
	service := newApplicationContractService(repoRoot)

	output := marshalSpecWriteContractCall(t, func() (SpecProjection, map[string]any, []any, error) {
		return service.AddDelta(DeltaAddRequest{
			Target:              "runtime:session-lifecycle",
			Intent:              domain.DeltaIntentRepair,
			Area:                "Compensation cleanup second repair",
			Current:             "Evidence stale again",
			CurrentPresent:      true,
			Targets:             "Re-verify against new fixtures",
			TargetPresent:       true,
			Notes:               "Should be rejected before allocation",
			NotesPresent:        true,
			AffectsRequirements: []string{"REQ-001"},
		})
	})
	assertContractFixture(t, output, contractPlaceholders())
}

func TestContract_DeltaAdd_ValidationFailed(t *testing.T) {
	repoRoot := copyApplicationFixtureRepo(t, "ready-spec")
	replaceFileText(t, filepath.Join(repoRoot, ".specs", "runtime", "session-lifecycle.yaml"), "id: D-001", "id: D-003")
	service := newApplicationContractService(repoRoot)

	output := marshalSpecWriteContractCall(t, func() (SpecProjection, map[string]any, []any, error) {
		return service.AddDelta(DeltaAddRequest{
			Target:         "runtime:session-lifecycle",
			Intent:         domain.DeltaIntentAdd,
			Area:           "Heartbeat timeout",
			Current:        "Current gap",
			CurrentPresent: true,
			Targets:        "Target gap",
			TargetPresent:  true,
			Notes:          "Explicitly tracked",
			NotesPresent:   true,
		})
	})
	assertContractFixture(t, output, contractPlaceholders())
}

func TestContract_DeltaTransitions_SpecNotFound(t *testing.T) {
	for _, tc := range deltaTransitionCases() {
		t.Run(tc.name, func(t *testing.T) {
			repoRoot := copyApplicationFixtureRepo(t, "ready-spec")
			service := newApplicationContractService(repoRoot)
			output := marshalSpecWriteContractCall(t, func() (SpecProjection, map[string]any, []any, error) {
				return tc.invoke(service, DeltaTransitionRequest{
					Target:  "runtime:missing-spec",
					DeltaID: "D-001",
				})
			})
			assertContractFixture(t, output, contractPlaceholders())
		})
	}
}

func TestContract_DeltaTransitions_DeltaNotFound(t *testing.T) {
	for _, tc := range deltaTransitionCases() {
		t.Run(tc.name, func(t *testing.T) {
			repoRoot := copyApplicationFixtureRepo(t, "ready-spec")
			service := newApplicationContractService(repoRoot)
			output := marshalSpecWriteContractCall(t, func() (SpecProjection, map[string]any, []any, error) {
				return tc.invoke(service, DeltaTransitionRequest{
					Target:  "runtime:session-lifecycle",
					DeltaID: "D-999",
				})
			})
			assertContractFixture(t, output, contractPlaceholders())
		})
	}
}

func TestContract_DeltaTransitions_InvalidState(t *testing.T) {
	cases := []struct {
		name    string
		fixture string
		deltaID string
		invoke  deltaTransitionInvoke
	}{
		{name: "Start", fixture: "active-spec", deltaID: "D-001", invoke: (*Service).StartDelta},
		{name: "Defer", fixture: "verified-spec", deltaID: "D-001", invoke: (*Service).DeferDelta},
		{name: "Resume", fixture: "active-spec", deltaID: "D-001", invoke: (*Service).ResumeDelta},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repoRoot := copyApplicationFixtureRepo(t, tc.fixture)
			service := newApplicationContractService(repoRoot)
			output := marshalSpecWriteContractCall(t, func() (SpecProjection, map[string]any, []any, error) {
				return tc.invoke(service, DeltaTransitionRequest{
					Target:  "runtime:session-lifecycle",
					DeltaID: tc.deltaID,
				})
			})
			assertContractFixture(t, output, contractPlaceholders())
		})
	}
}

func TestContract_DeltaTransitions_Success(t *testing.T) {
	cases := []struct {
		name    string
		fixture string
		deltaID string
		invoke  deltaTransitionInvoke
	}{
		{name: "Start", fixture: "ready-spec", deltaID: "D-001", invoke: (*Service).StartDelta},
		{name: "Defer", fixture: "ready-spec", deltaID: "D-001", invoke: (*Service).DeferDelta},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repoRoot := copyApplicationFixtureRepo(t, tc.fixture)
			service := newApplicationContractService(repoRoot)
			output := marshalSpecWriteContractCall(t, func() (SpecProjection, map[string]any, []any, error) {
				return tc.invoke(service, DeltaTransitionRequest{
					Target:  "runtime:session-lifecycle",
					DeltaID: tc.deltaID,
				})
			})
			assertContractFixture(t, output, contractPlaceholders())
		})
	}
}

func TestContract_DeltaTransitions_ValidationFailed(t *testing.T) {
	for _, tc := range deltaTransitionCases() {
		t.Run(tc.name, func(t *testing.T) {
			repoRoot := copyApplicationFixtureRepo(t, "ready-spec")
			replaceFileText(t, filepath.Join(repoRoot, ".specs", "runtime", "session-lifecycle.yaml"), "id: D-001", "id: D-003")
			service := newApplicationContractService(repoRoot)
			output := marshalSpecWriteContractCall(t, func() (SpecProjection, map[string]any, []any, error) {
				return tc.invoke(service, DeltaTransitionRequest{
					Target:  "runtime:session-lifecycle",
					DeltaID: "D-001",
				})
			})
			assertContractFixture(t, output, contractPlaceholders())
		})
	}
}

func TestContract_DeltaWithdraw_Success(t *testing.T) {
	repoRoot := copyApplicationFixtureRepo(t, "ready-spec")
	service := newApplicationContractService(repoRoot)

	output := marshalSpecWriteContractCall(t, func() (SpecProjection, map[string]any, []any, error) {
		return service.WithdrawDelta(DeltaWithdrawRequest{
			Target:  "runtime:session-lifecycle",
			DeltaID: "D-001",
			Reason:  "Opened in error; supersession planned in a separate delta.",
		})
	})
	assertContractFixture(t, output, contractPlaceholders())
}

func TestContract_DeltaWithdraw_MissingReason(t *testing.T) {
	repoRoot := copyApplicationFixtureRepo(t, "ready-spec")
	service := newApplicationContractService(repoRoot)

	output := marshalSpecWriteContractCall(t, func() (SpecProjection, map[string]any, []any, error) {
		return service.WithdrawDelta(DeltaWithdrawRequest{
			Target:  "runtime:session-lifecycle",
			DeltaID: "D-001",
		})
	})
	assertContractFixture(t, output, contractPlaceholders())
}

func TestContract_DeltaWithdraw_ClosedRejected(t *testing.T) {
	repoRoot := contractActiveRequirementRepo(t)
	replaceTrackedRequirementBlockOnly(t, repoRoot, "verified")
	replaceFileText(t, filepath.Join(repoRoot, ".specs", "runtime", "session-lifecycle.yaml"), "status: in-progress", "status: closed")
	replaceFileText(t, filepath.Join(repoRoot, ".specs", "runtime", "session-lifecycle.yaml"), "status: active", "status: verified")
	service := newApplicationContractService(repoRoot)

	output := marshalSpecWriteContractCall(t, func() (SpecProjection, map[string]any, []any, error) {
		return service.WithdrawDelta(DeltaWithdrawRequest{
			Target:  "runtime:session-lifecycle",
			DeltaID: "D-001",
			Reason:  "Attempting to withdraw a closed delta should fail",
		})
	})
	assertContractFixture(t, output, contractPlaceholders())
}

func TestContract_DeltaClose_RepairTerminal(t *testing.T) {
	repoRoot := contractRepairTerminalRepo(t)
	service := newApplicationContractService(repoRoot)

	output := marshalSpecWriteContractCall(t, func() (SpecProjection, map[string]any, []any, error) {
		return service.CloseDelta(DeltaTransitionRequest{
			Target:  "runtime:session-lifecycle",
			DeltaID: "D-002",
		})
	})
	assertContractFixture(t, output, contractPlaceholders())
}

func TestContract_DeltaClose_RevisionBumpTerminal(t *testing.T) {
	repoRoot := contractCloseToRevBumpRepo(t)
	service := newApplicationContractService(repoRoot)

	output := marshalSpecWriteContractCall(t, func() (SpecProjection, map[string]any, []any, error) {
		return service.CloseDelta(DeltaTransitionRequest{
			Target:  "runtime:session-lifecycle",
			DeltaID: "D-002",
		})
	})
	assertContractFixture(t, output, contractPlaceholders())
}

func TestContract_DeltaClose_SpecNotFound(t *testing.T) {
	repoRoot := copyApplicationFixtureRepo(t, "ready-spec")
	service := newApplicationContractService(repoRoot)

	output := marshalSpecWriteContractCall(t, func() (SpecProjection, map[string]any, []any, error) {
		return service.CloseDelta(DeltaTransitionRequest{
			Target:  "runtime:missing-spec",
			DeltaID: "D-001",
		})
	})
	assertContractFixture(t, output, contractPlaceholders())
}

func TestContract_DeltaClose_DeltaNotFound(t *testing.T) {
	repoRoot := copyApplicationFixtureRepo(t, "ready-spec")
	service := newApplicationContractService(repoRoot)

	output := marshalSpecWriteContractCall(t, func() (SpecProjection, map[string]any, []any, error) {
		return service.CloseDelta(DeltaTransitionRequest{
			Target:  "runtime:session-lifecycle",
			DeltaID: "D-999",
		})
	})
	assertContractFixture(t, output, contractPlaceholders())
}

func TestContract_DeltaClose_InvalidState(t *testing.T) {
	repoRoot := copyApplicationFixtureRepo(t, "verified-spec")
	service := newApplicationContractService(repoRoot)

	output := marshalSpecWriteContractCall(t, func() (SpecProjection, map[string]any, []any, error) {
		return service.CloseDelta(DeltaTransitionRequest{
			Target:  "runtime:session-lifecycle",
			DeltaID: "D-001",
		})
	})
	assertContractFixture(t, output, contractPlaceholders())
}

func TestContract_DeltaClose_UpdatesUnresolved(t *testing.T) {
	repoRoot := contractChangeDeltaRepo(t)
	replaceFileText(t, filepath.Join(repoRoot, ".specs", "runtime", "session-lifecycle.yaml"), "verification: unverified", "verification: verified")
	replaceFileText(t, filepath.Join(repoRoot, ".specs", "runtime", "session-lifecycle.yaml"), "test_files: []", "test_files:\n      - runtime/tests/domain/test_compensation_cleanup.py")
	service := newApplicationContractService(repoRoot)

	output := marshalSpecWriteContractCall(t, func() (SpecProjection, map[string]any, []any, error) {
		return service.CloseDelta(DeltaTransitionRequest{
			Target:  "runtime:session-lifecycle",
			DeltaID: "D-002",
		})
	})
	assertContractFixture(t, output, contractPlaceholders())
}

func TestContract_DeltaClose_UnverifiedRequirements(t *testing.T) {
	repoRoot := copyApplicationFixtureRepo(t, "active-spec")
	service := newApplicationContractService(repoRoot)

	output := marshalSpecWriteContractCall(t, func() (SpecProjection, map[string]any, []any, error) {
		return service.CloseDelta(DeltaTransitionRequest{
			Target:  "runtime:session-lifecycle",
			DeltaID: "D-001",
		})
	})
	assertContractFixture(t, output, contractPlaceholders())
}

func TestContract_DeltaClose_MatchBlocking(t *testing.T) {
	repoRoot := contractReplaceFlowRepo(t)
	service := newApplicationContractService(repoRoot)

	if _, _, _, err := service.ReplaceRequirement(RequirementReplaceRequest{
		Target:        "runtime:session-lifecycle",
		RequirementID: "REQ-001",
		DeltaID:       "D-002",
		Gherkin:       contractReplacementRequirementBlock(),
	}); err != nil {
		t.Fatalf("ReplaceRequirement: %v", err)
	}
	if _, _, _, err := service.VerifyRequirement(RequirementVerifyRequest{
		Target:        "runtime:session-lifecycle",
		RequirementID: "REQ-002",
		TestFiles:     []string{"runtime/tests/domain/test_compensation_cleanup.py"},
	}); err != nil {
		t.Fatalf("VerifyRequirement: %v", err)
	}
	replaceSessionLifecycleDoc(t, repoRoot, contractRequirementSection("Compensation stage 4 failure cleanup", contractRequirementBlock("@runtime @e2e", "Compensation stage 4 failure cleanup (edited)")))

	output := marshalSpecWriteContractCall(t, func() (SpecProjection, map[string]any, []any, error) {
		return service.CloseDelta(DeltaTransitionRequest{
			Target:  "runtime:session-lifecycle",
			DeltaID: "D-002",
		})
	})
	assertContractFixture(t, output, contractPlaceholders())
}

func TestContract_DeltaClose_ValidationFailed(t *testing.T) {
	repoRoot := copyApplicationFixtureRepo(t, "ready-spec")
	replaceFileText(t, filepath.Join(repoRoot, ".specs", "runtime", "session-lifecycle.yaml"), "id: D-001", "id: D-003")
	service := newApplicationContractService(repoRoot)

	output := marshalSpecWriteContractCall(t, func() (SpecProjection, map[string]any, []any, error) {
		return service.CloseDelta(DeltaTransitionRequest{
			Target:  "runtime:session-lifecycle",
			DeltaID: "D-001",
		})
	})
	assertContractFixture(t, output, contractPlaceholders())
}

func TestContract_SpecCreate_ValidationFailed(t *testing.T) {
	repoRoot := copyApplicationFixtureRepo(t, "charter-dag")
	initGitRepoAtDate(t, repoRoot, "2026-03-28T12:00:00Z")
	if err := os.MkdirAll(filepath.Join(repoRoot, "runtime", "src", "domain", "new_protocol"), 0o755); err != nil {
		t.Fatalf("mkdir design doc dir: %v", err)
	}

	service := newApplicationContractService(repoRoot)
	output := marshalSpecWriteContractCall(t, func() (SpecProjection, map[string]any, []any, error) {
		return service.CreateSpec(SpecCreateRequest{
			Target:       "runtime:new-protocol",
			Title:        "New Protocol",
			Doc:          "runtime/src/domain/new_protocol/SPEC.md",
			Scope:        []string{"runtime/src/domain/new_protocol/"},
			Group:        "recovery",
			Order:        30,
			CharterNotes: "Protocol planning",
			Tags:         []string{"Invalid-Tag"},
		})
	})
	assertContractFixture(t, output, contractPlaceholders())
}

func TestContract_WriteValidationFailed_CommonSurface(t *testing.T) {
	cases := []struct {
		name   string
		output func(t *testing.T) string
	}{
		{
			name: "ReqWithdraw",
			output: func(t *testing.T) string {
				repoRoot := contractRemoveDeltaRepo(t)
				replaceFileText(t, filepath.Join(repoRoot, ".specs", "runtime", "session-lifecycle.yaml"), "id: D-001", "id: D-003")
				service := newApplicationContractService(repoRoot)
				return marshalSpecWriteContractCall(t, func() (SpecProjection, map[string]any, []any, error) {
					return service.WithdrawRequirement(RequirementDeltaRequest{
						Target:        "runtime:session-lifecycle",
						RequirementID: "REQ-001",
						DeltaID:       "D-002",
					})
				})
			},
		},
		{
			name: "ReqRefresh",
			output: func(t *testing.T) string {
				repoRoot := contractRefreshRepo(t)
				replaceFileText(t, filepath.Join(repoRoot, ".specs", "runtime", "session-lifecycle.yaml"), "id: D-001", "id: D-003")
				service := newApplicationContractService(repoRoot)
				return marshalSpecWriteContractCall(t, func() (SpecProjection, map[string]any, []any, error) {
					return service.RefreshRequirement(RequirementRefreshRequest{
						Target:        "runtime:session-lifecycle",
						RequirementID: "REQ-001",
						Gherkin:       "@runtime @e2e\n@critical\nFeature: Compensation stage 4 failure cleanup",
					})
				})
			},
		},
		{
			name: "ReqVerify",
			output: func(t *testing.T) string {
				repoRoot := contractActiveRequirementRepo(t)
				replaceFileText(t, filepath.Join(repoRoot, ".specs", "runtime", "session-lifecycle.yaml"), "id: D-001", "id: D-003")
				service := newApplicationContractService(repoRoot)
				return marshalSpecWriteContractCall(t, func() (SpecProjection, map[string]any, []any, error) {
					return service.VerifyRequirement(RequirementVerifyRequest{
						Target:        "runtime:session-lifecycle",
						RequirementID: "REQ-001",
						TestFiles:     []string{"runtime/tests/domain/test_compensation_cleanup.py"},
					})
				})
			},
		},
		{
			name: "RevBump",
			output: func(t *testing.T) string {
				repoRoot := copyApplicationFixtureRepo(t, "verified-spec")
				replaceFileText(t, filepath.Join(repoRoot, ".specs", "runtime", "session-lifecycle.yaml"), "id: D-001", "id: D-003")
				service := newApplicationContractService(repoRoot)
				return marshalSpecWriteContractCall(t, func() (SpecProjection, map[string]any, []any, error) {
					return service.BumpRevision(RevisionBumpRequest{
						Target:     "runtime:session-lifecycle",
						Checkpoint: "HEAD",
						Summary:    "Malformed state should block alignment.",
					})
				})
			},
		},
		{
			name: "Sync",
			output: func(t *testing.T) string {
				repoRoot := copyApplicationFixtureRepo(t, "verified-spec")
				replaceFileText(t, filepath.Join(repoRoot, ".specs", "runtime", "session-lifecycle.yaml"), "id: D-001", "id: D-003")
				service := newApplicationContractService(repoRoot)
				return marshalSpecWriteContractCall(t, func() (SpecProjection, map[string]any, []any, error) {
					return service.Sync(SyncRequest{
						Target:     "runtime:session-lifecycle",
						Checkpoint: "HEAD",
						Summary:    "Malformed state should block alignment.",
					})
				})
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assertContractFixture(t, tc.output(t), contractPlaceholders())
		})
	}
}

func TestContract_ReqWithdraw_Success(t *testing.T) {
	repoRoot := contractRemoveDeltaRepo(t)
	service := newApplicationContractService(repoRoot)

	output := marshalSpecWriteContractCall(t, func() (SpecProjection, map[string]any, []any, error) {
		return service.WithdrawRequirement(RequirementDeltaRequest{
			Target:        "runtime:session-lifecycle",
			RequirementID: "REQ-001",
			DeltaID:       "D-002",
		})
	})
	assertContractFixture(t, output, contractPlaceholders())
}

func TestContract_ReqRefresh_MatchRefreshNotNeeded(t *testing.T) {
	repoRoot := contractActiveRequirementRepo(t)
	service := newApplicationContractService(repoRoot)

	output := marshalSpecWriteContractCall(t, func() (SpecProjection, map[string]any, []any, error) {
		return service.RefreshRequirement(RequirementRefreshRequest{
			Target:        "runtime:session-lifecycle",
			RequirementID: "REQ-001",
			Gherkin:       "@runtime @e2e\nFeature: Compensation stage 4 failure cleanup",
		})
	})
	assertContractFixture(t, output, contractPlaceholders())
}

func TestContract_ReqRefresh_Success(t *testing.T) {
	repoRoot := contractRefreshRepo(t)
	service := newApplicationContractService(repoRoot)

	output := marshalSpecWriteContractCall(t, func() (SpecProjection, map[string]any, []any, error) {
		return service.RefreshRequirement(RequirementRefreshRequest{
			Target:        "runtime:session-lifecycle",
			RequirementID: "REQ-001",
			Gherkin:       "@runtime @e2e\n@critical\nFeature: Compensation stage 4 failure cleanup",
		})
	})
	assertContractFixture(t, output, contractPlaceholders())
}

func TestContract_ReqRefresh_RecordedWorkflowConflict(t *testing.T) {
	repoRoot := contractRefreshConflictRepo(t)
	service := newApplicationContractService(repoRoot)

	output := marshalSpecWriteContractCall(t, func() (SpecProjection, map[string]any, []any, error) {
		return service.RefreshRequirement(RequirementRefreshRequest{
			Target:        "runtime:session-lifecycle",
			RequirementID: "REQ-001",
			Gherkin:       "@runtime @e2e\n@critical\nFeature: Compensation stage 4 failure cleanup",
		})
	})
	assertContractFixture(t, output, contractPlaceholders())
}

func TestContract_ReqVerify_Success(t *testing.T) {
	repoRoot := contractActiveRequirementRepo(t)
	service := newApplicationContractService(repoRoot)

	output := marshalSpecWriteContractCall(t, func() (SpecProjection, map[string]any, []any, error) {
		return service.VerifyRequirement(RequirementVerifyRequest{
			Target:        "runtime:session-lifecycle",
			RequirementID: "REQ-001",
			TestFiles:     []string{"runtime/tests/domain/test_compensation_cleanup.py"},
		})
	})
	assertContractFixture(t, output, contractPlaceholders())
}

func TestContract_ReqAdd_RequirementNotInSpec(t *testing.T) {
	repoRoot := copyApplicationFixtureRepo(t, "ready-spec")
	service := newApplicationContractService(repoRoot)

	output := marshalSpecWriteContractCall(t, func() (SpecProjection, map[string]any, []any, error) {
		return service.AddRequirement(RequirementAddRequest{
			Target:  "runtime:session-lifecycle",
			DeltaID: "D-001",
			Gherkin: "@runtime @e2e\nFeature: Undocumented cleanup branch\n\n  Scenario: Cleanup works for a new branch\n    Given a new cleanup branch exists\n    When recovery completes\n    Then the branch is cleaned up deterministically",
		})
	})
	assertContractFixture(t, output, contractPlaceholders())
}

func TestContract_ReqAdd_AlreadyTracked(t *testing.T) {
	repoRoot := copyApplicationFixtureRepo(t, "active-spec")
	service := newApplicationContractService(repoRoot)

	// REQ-001 already tracks "Compensation stage 4 failure cleanup" with lifecycle active.
	// Attempting to add the same Feature title should fail.
	output := marshalSpecWriteContractCall(t, func() (SpecProjection, map[string]any, []any, error) {
		return service.AddRequirement(RequirementAddRequest{
			Target:  "runtime:session-lifecycle",
			DeltaID: "D-001",
			Gherkin: "@runtime @e2e\nFeature: Compensation stage 4 failure cleanup",
		})
	})
	assertContractFixture(t, output, contractPlaceholders())
}

func TestContract_ReqAdd_WithdrawnAllowsReuse(t *testing.T) {
	repoRoot := contractRemoveDeltaRepo(t)
	service := newApplicationContractService(repoRoot)

	// Withdraw REQ-001 via the remove delta, then re-add with the same Feature title.
	if _, _, _, err := service.WithdrawRequirement(RequirementDeltaRequest{
		Target:        "runtime:session-lifecycle",
		RequirementID: "REQ-001",
		DeltaID:       "D-002",
	}); err != nil {
		t.Fatalf("WithdrawRequirement: %v", err)
	}

	output := marshalSpecWriteContractCall(t, func() (SpecProjection, map[string]any, []any, error) {
		return service.AddRequirement(RequirementAddRequest{
			Target:  "runtime:session-lifecycle",
			DeltaID: "D-001",
			Gherkin: "@runtime @e2e\nFeature: Compensation stage 4 failure cleanup",
		})
	})
	assertContractFixture(t, output, contractPlaceholders())
}

func TestContract_ReqReplace_AutoRebindOtherDeltas(t *testing.T) {
	repoRoot := contractReplaceFlowRepo(t)
	appendDeltaBeforeRequirements(t, repoRoot, `  - id: D-003
    area: Downstream cleanup follow-up
    intent: repair
    status: open
    origin_checkpoint: a1b2c3f
    current: Evidence is soft
    target: Firm up evidence after the replacement lands
    notes: Independent delta that still anchors to REQ-001
    affects_requirements:
      - REQ-001
    updates:
      - stale_requirement
`)
	replaceFileText(t, filepath.Join(repoRoot, ".specs", "specctl.yaml"), "source_prefixes:", "auto_rebind_on_replace: true\nsource_prefixes:")
	service := newApplicationContractService(repoRoot)

	output := marshalSpecWriteContractCall(t, func() (SpecProjection, map[string]any, []any, error) {
		return service.ReplaceRequirement(RequirementReplaceRequest{
			Target:        "runtime:session-lifecycle",
			RequirementID: "REQ-001",
			DeltaID:       "D-002",
			Gherkin:       contractReplacementRequirementBlock(),
		})
	})
	assertContractFixture(t, output, contractPlaceholders())
}

func TestContract_DeltaRebind_Success(t *testing.T) {
	repoRoot := contractReplaceFlowRepo(t)
	appendDeltaBeforeRequirements(t, repoRoot, `  - id: D-003
    area: Downstream cleanup follow-up
    intent: repair
    status: open
    origin_checkpoint: a1b2c3f
    current: Evidence is soft
    target: Firm up evidence after the replacement lands
    notes: Rebinds to REQ-002 once the replacement lands
    affects_requirements:
      - REQ-001
    updates:
      - stale_requirement
`)
	service := newApplicationContractService(repoRoot)
	if _, _, _, err := service.ReplaceRequirement(RequirementReplaceRequest{
		Target:        "runtime:session-lifecycle",
		RequirementID: "REQ-001",
		DeltaID:       "D-002",
		Gherkin:       contractReplacementRequirementBlock(),
	}); err != nil {
		t.Fatalf("ReplaceRequirement: %v", err)
	}

	output := marshalSpecWriteContractCall(t, func() (SpecProjection, map[string]any, []any, error) {
		return service.RebindDeltaRequirements(DeltaRebindRequest{
			Target:  "runtime:session-lifecycle",
			DeltaID: "D-003",
			From:    "REQ-001",
			To:      "REQ-002",
			Reason:  "Scope preserved; anchor rebound after supersession landed.",
		})
	})
	assertContractFixture(t, output, contractPlaceholders())
}

func TestContract_ReqReplace_AutoRebindDisabledByDefault(t *testing.T) {
	repoRoot := contractReplaceFlowRepo(t)
	appendDeltaBeforeRequirements(t, repoRoot, `  - id: D-003
    area: Downstream cleanup follow-up
    intent: repair
    status: open
    origin_checkpoint: a1b2c3f
    current: Evidence is soft
    target: Firm up evidence after the replacement lands
    notes: Should NOT be auto-rebound because auto_rebind_on_replace is off by default
    affects_requirements:
      - REQ-001
    updates:
      - stale_requirement
`)
	service := newApplicationContractService(repoRoot)

	output := marshalSpecWriteContractCall(t, func() (SpecProjection, map[string]any, []any, error) {
		return service.ReplaceRequirement(RequirementReplaceRequest{
			Target:        "runtime:session-lifecycle",
			RequirementID: "REQ-001",
			DeltaID:       "D-002",
			Gherkin:       contractReplacementRequirementBlock(),
		})
	})
	assertContractFixture(t, output, contractPlaceholders())
}

func TestContract_DeltaRebind_ClosedRejected(t *testing.T) {
	repoRoot := contractInactiveSupersededRequirementRepo(t)
	service := newApplicationContractService(repoRoot)

	output := marshalSpecWriteContractCall(t, func() (SpecProjection, map[string]any, []any, error) {
		return service.RebindDeltaRequirements(DeltaRebindRequest{
			Target:  "runtime:session-lifecycle",
			DeltaID: "D-002",
			From:    "REQ-001",
			To:      "REQ-002",
		})
	})
	assertContractFixture(t, output, contractPlaceholders())
}

func TestContract_ReqReplace_RequirementNotInSpec(t *testing.T) {
	repoRoot := contractChangeDeltaRepo(t)
	service := newApplicationContractService(repoRoot)

	output := marshalSpecWriteContractCall(t, func() (SpecProjection, map[string]any, []any, error) {
		return service.ReplaceRequirement(RequirementReplaceRequest{
			Target:        "runtime:session-lifecycle",
			RequirementID: "REQ-001",
			DeltaID:       "D-002",
			Gherkin:       contractReplacementRequirementBlock(),
		})
	})
	assertContractFixture(t, output, contractPlaceholders())
}

func TestContract_ReqVerify_InactiveLifecycle(t *testing.T) {
	repoRoot := contractRemoveDeltaRepo(t)
	service := newApplicationContractService(repoRoot)

	if _, _, _, err := service.WithdrawRequirement(RequirementDeltaRequest{
		Target:        "runtime:session-lifecycle",
		RequirementID: "REQ-001",
		DeltaID:       "D-002",
	}); err != nil {
		t.Fatalf("WithdrawRequirement: %v", err)
	}

	output := marshalSpecWriteContractCall(t, func() (SpecProjection, map[string]any, []any, error) {
		return service.VerifyRequirement(RequirementVerifyRequest{
			Target:        "runtime:session-lifecycle",
			RequirementID: "REQ-001",
			TestFiles:     []string{"runtime/tests/domain/test_compensation_cleanup.py"},
		})
	})
	assertContractFixture(t, output, contractPlaceholders())
}

func TestContract_ReqVerify_TestFilesRequired(t *testing.T) {
	repoRoot := contractActiveRequirementRepo(t)
	service := newApplicationContractService(repoRoot)

	output := marshalSpecWriteContractCall(t, func() (SpecProjection, map[string]any, []any, error) {
		return service.VerifyRequirement(RequirementVerifyRequest{
			Target:        "runtime:session-lifecycle",
			RequirementID: "REQ-001",
		})
	})
	assertContractFixture(t, output, contractPlaceholders())
}

func TestContract_ReqVerify_TestFileNotFound(t *testing.T) {
	repoRoot := contractActiveRequirementRepo(t)
	service := newApplicationContractService(repoRoot)

	output := marshalSpecWriteContractCall(t, func() (SpecProjection, map[string]any, []any, error) {
		return service.VerifyRequirement(RequirementVerifyRequest{
			Target:        "runtime:session-lifecycle",
			RequirementID: "REQ-001",
			TestFiles:     []string{"runtime/tests/domain/missing_test.py"},
		})
	})
	assertContractFixture(t, output, contractPlaceholders())
}

func TestContract_ReqVerify_MatchBlocking(t *testing.T) {
	repoRoot := contractRefreshRepo(t)
	service := newApplicationContractService(repoRoot)

	output := marshalSpecWriteContractCall(t, func() (SpecProjection, map[string]any, []any, error) {
		return service.VerifyRequirement(RequirementVerifyRequest{
			Target:        "runtime:session-lifecycle",
			RequirementID: "REQ-001",
			TestFiles:     []string{"runtime/tests/domain/test_compensation_cleanup.py"},
		})
	})
	assertContractFixture(t, output, contractPlaceholders())
}

func TestContract_RevBump_Success(t *testing.T) {
	repoRoot := contractRevBumpReadyRepo(t)
	headSHA := strings.TrimSpace(runGitAtDate(t, repoRoot, "2026-03-30T09:36:00Z", "rev-parse", "HEAD"))
	service := newApplicationContractService(repoRoot)

	output := marshalSpecWriteContractCall(t, func() (SpecProjection, map[string]any, []any, error) {
		return service.BumpRevision(RevisionBumpRequest{
			Target:     "runtime:session-lifecycle",
			Checkpoint: "HEAD",
			Summary:    "Track the reviewed drift in a new revision.",
		})
	})
	placeholders := contractPlaceholders()
	placeholders["__HEAD_SHA__"] = headSHA
	assertContractFixture(t, output, placeholders)
}

func TestContract_RevBump_WithDeltasWithdrawn(t *testing.T) {
	repoRoot := contractDeferredSupersededResidueRepo(t)
	trackingPath := filepath.Join(repoRoot, ".specs", "runtime", "session-lifecycle.yaml")
	replaceFileText(t, trackingPath, "area: Deferred cleanup residue\n    intent: change\n    status: deferred", "area: Deferred cleanup residue\n    intent: change\n    status: withdrawn\n    withdrawn_reason: Retroactively retracted; decomposition is no longer planned.")
	initGitRepoAtDate(t, repoRoot, "2026-03-30T09:36:00Z")
	headSHA := strings.TrimSpace(runGitAtDate(t, repoRoot, "2026-03-30T09:36:00Z", "rev-parse", "HEAD"))
	service := newApplicationContractService(repoRoot)

	output := marshalSpecWriteContractCall(t, func() (SpecProjection, map[string]any, []any, error) {
		return service.BumpRevision(RevisionBumpRequest{
			Target:     "runtime:session-lifecycle",
			Checkpoint: "HEAD",
			Summary:    "Record the withdrawn follow-up in the changelog.",
		})
	})
	placeholders := contractPlaceholders()
	placeholders["__HEAD_SHA__"] = headSHA
	assertContractFixture(t, output, placeholders)
}

func TestContract_RevBump_StatusNotVerified(t *testing.T) {
	repoRoot := copyApplicationFixtureRepo(t, "active-spec")
	service := newApplicationContractService(repoRoot)

	output := marshalSpecWriteContractCall(t, func() (SpecProjection, map[string]any, []any, error) {
		return service.BumpRevision(RevisionBumpRequest{
			Target:     "runtime:session-lifecycle",
			Checkpoint: "HEAD",
			Summary:    "Document drift still needs semantic resolution.",
		})
	})
	assertContractFixture(t, output, contractPlaceholders())
}

func TestContract_RevBump_MissingCheckpoint(t *testing.T) {
	repoRoot := contractRevBumpReadyRepo(t)
	service := newApplicationContractService(repoRoot)

	output := marshalSpecWriteContractCall(t, func() (SpecProjection, map[string]any, []any, error) {
		return service.BumpRevision(RevisionBumpRequest{
			Target:  "runtime:session-lifecycle",
			Summary: "Track the reviewed drift in a new revision.",
		})
	})
	assertContractFixture(t, output, contractPlaceholders())
}

func TestContract_RevBump_MissingSummary(t *testing.T) {
	repoRoot := contractRevBumpReadyRepo(t)
	service := newApplicationContractService(repoRoot)

	output := marshalSpecWriteContractCall(t, func() (SpecProjection, map[string]any, []any, error) {
		return service.BumpRevision(RevisionBumpRequest{
			Target:     "runtime:session-lifecycle",
			Checkpoint: "HEAD",
		})
	})
	assertContractFixture(t, output, contractPlaceholders())
}

func TestContract_RevBump_NoSemanticChanges(t *testing.T) {
	repoRoot := copyApplicationFixtureRepo(t, "verified-spec")
	initGitRepoAtDate(t, repoRoot, "2026-03-28T12:00:00Z")
	headSHA := strings.TrimSpace(runGitAtDate(t, repoRoot, "2026-03-28T12:00:00Z", "rev-parse", "HEAD"))
	service := newApplicationContractService(repoRoot)

	output := marshalSpecWriteContractCall(t, func() (SpecProjection, map[string]any, []any, error) {
		return service.BumpRevision(RevisionBumpRequest{
			Target:     "runtime:session-lifecycle",
			Checkpoint: "HEAD",
			Summary:    "Document drift still needs semantic resolution.",
		})
	})
	placeholders := contractPlaceholders()
	placeholders["__HEAD_SHA__"] = headSHA
	assertContractFixture(t, output, placeholders)
}

func TestContract_RevBump_CheckpointUnavailable(t *testing.T) {
	repoRoot := copyApplicationFixtureRepo(t, "verified-spec")
	service := newApplicationContractService(repoRoot)

	output := marshalSpecWriteContractCall(t, func() (SpecProjection, map[string]any, []any, error) {
		return service.BumpRevision(RevisionBumpRequest{
			Target:     "runtime:session-lifecycle",
			Checkpoint: "HEAD",
			Summary:    "Document drift still needs semantic resolution.",
		})
	})
	assertContractFixture(t, output, contractPlaceholders())
}

func TestContract_RevBump_MatchBlocking(t *testing.T) {
	repoRoot := contractVerifiedMatchBlockingRepo(t)
	service := newApplicationContractService(repoRoot)

	output := marshalSpecWriteContractCall(t, func() (SpecProjection, map[string]any, []any, error) {
		return service.BumpRevision(RevisionBumpRequest{
			Target:     "runtime:session-lifecycle",
			Checkpoint: "HEAD",
			Summary:    "Document drift still needs semantic resolution.",
		})
	})
	assertContractFixture(t, output, contractPlaceholders())
}

func TestContract_Sync_Success(t *testing.T) {
	repoRoot := contractCodeOnlyDriftRepo(t)
	headSHA := strings.TrimSpace(runGitAtDate(t, repoRoot, "2026-03-30T09:30:00Z", "rev-parse", "HEAD"))
	service := newApplicationContractService(repoRoot)

	output := marshalSpecWriteContractCall(t, func() (SpecProjection, map[string]any, []any, error) {
		return service.Sync(SyncRequest{
			Target:     "runtime:session-lifecycle",
			Checkpoint: "HEAD",
			Summary:    "Code drift reviewed; spec remains accurate.",
		})
	})
	placeholders := contractPlaceholders()
	placeholders["__HEAD_SHA__"] = headSHA
	assertContractFixture(t, output, placeholders)
}

func TestContract_Sync_MissingCheckpoint(t *testing.T) {
	repoRoot := contractCodeOnlyDriftRepo(t)
	service := newApplicationContractService(repoRoot)

	output := marshalSpecWriteContractCall(t, func() (SpecProjection, map[string]any, []any, error) {
		return service.Sync(SyncRequest{
			Target:  "runtime:session-lifecycle",
			Summary: "Code drift reviewed; spec remains accurate.",
		})
	})
	assertContractFixture(t, output, contractPlaceholders())
}

func TestContract_Sync_MissingSummary(t *testing.T) {
	repoRoot := contractCodeOnlyDriftRepo(t)
	service := newApplicationContractService(repoRoot)

	output := marshalSpecWriteContractCall(t, func() (SpecProjection, map[string]any, []any, error) {
		return service.Sync(SyncRequest{
			Target:     "runtime:session-lifecycle",
			Checkpoint: "HEAD",
		})
	})
	assertContractFixture(t, output, contractPlaceholders())
}

func TestContract_Sync_SemanticWorkRequired(t *testing.T) {
	repoRoot := contractDriftedDesignDocRepo(t)
	headSHA := strings.TrimSpace(runGitAtDate(t, repoRoot, "2026-03-31T09:30:00Z", "rev-parse", "HEAD"))
	service := newApplicationContractService(repoRoot)

	output := marshalSpecWriteContractCall(t, func() (SpecProjection, map[string]any, []any, error) {
		return service.Sync(SyncRequest{
			Target:     "runtime:session-lifecycle",
			Checkpoint: "HEAD",
			Summary:    "Spec is still accurate after review.",
		})
	})
	placeholders := contractPlaceholders()
	placeholders["__HEAD_SHA__"] = headSHA
	assertContractFixture(t, output, placeholders)
}

func TestContract_Sync_LiveDeltasPresent(t *testing.T) {
	repoRoot := copyApplicationFixtureRepo(t, "active-spec")
	initGitRepoAtDate(t, repoRoot, "2026-03-28T12:00:00Z")
	headSHA := strings.TrimSpace(runGitAtDate(t, repoRoot, "2026-03-28T12:00:00Z", "rev-parse", "HEAD"))
	service := newApplicationContractService(repoRoot)

	output := marshalSpecWriteContractCall(t, func() (SpecProjection, map[string]any, []any, error) {
		return service.Sync(SyncRequest{
			Target:     "runtime:session-lifecycle",
			Checkpoint: "HEAD",
			Summary:    "Spec is still accurate after review.",
		})
	})
	placeholders := contractPlaceholders()
	placeholders["__HEAD_SHA__"] = headSHA
	assertContractFixture(t, output, placeholders)
}

func TestContract_Sync_MatchBlocking(t *testing.T) {
	repoRoot := contractVerifiedMatchBlockingRepo(t)
	initGitRepoAtDate(t, repoRoot, "2026-03-28T12:00:00Z")
	headSHA := strings.TrimSpace(runGitAtDate(t, repoRoot, "2026-03-28T12:00:00Z", "rev-parse", "HEAD"))
	service := newApplicationContractService(repoRoot)

	output := marshalSpecWriteContractCall(t, func() (SpecProjection, map[string]any, []any, error) {
		return service.Sync(SyncRequest{
			Target:     "runtime:session-lifecycle",
			Checkpoint: "HEAD",
			Summary:    "Spec is still accurate after review.",
		})
	})
	placeholders := contractPlaceholders()
	placeholders["__HEAD_SHA__"] = headSHA
	assertContractFixture(t, output, placeholders)
}

func TestContract_Sync_CheckpointUnavailable(t *testing.T) {
	repoRoot := copyApplicationFixtureRepo(t, "verified-spec")
	service := newApplicationContractService(repoRoot)

	output := marshalSpecWriteContractCall(t, func() (SpecProjection, map[string]any, []any, error) {
		return service.Sync(SyncRequest{
			Target:     "runtime:session-lifecycle",
			Checkpoint: "HEAD",
			Summary:    "Spec is still accurate after review.",
		})
	})
	assertContractFixture(t, output, contractPlaceholders())
}

type deltaTransitionInvoke func(*Service, DeltaTransitionRequest) (SpecProjection, map[string]any, []any, error)

type deltaTransitionCase struct {
	name   string
	invoke deltaTransitionInvoke
}

func deltaTransitionCases() []deltaTransitionCase {
	return []deltaTransitionCase{
		{name: "Start", invoke: (*Service).StartDelta},
		{name: "Defer", invoke: (*Service).DeferDelta},
		{name: "Resume", invoke: (*Service).ResumeDelta},
	}
}

func baseSpecCreateRequest(target, doc string, scope []string) SpecCreateRequest {
	return SpecCreateRequest{
		Target:       target,
		Title:        "Drain Controller",
		Doc:          doc,
		Scope:        append([]string{}, scope...),
		Group:        "recovery",
		Order:        30,
		CharterNotes: "Drain coordination behavior",
	}
}

func contractActiveRequirementRepo(t *testing.T) string {
	t.Helper()

	repoRoot := copyApplicationFixtureRepo(t, "active-spec")
	if err := os.MkdirAll(filepath.Join(repoRoot, "runtime", "tests", "domain"), 0o755); err != nil {
		t.Fatalf("mkdir runtime/tests/domain: %v", err)
	}
	replaceSessionLifecycleDoc(t, repoRoot, contractRequirementSection("Compensation stage 4 failure cleanup", contractRequirementBlock("@runtime @e2e", "Compensation stage 4 failure cleanup")))
	replaceTrackedRequirementBlockOnly(t, repoRoot, "active")
	writeApplicationTestFile(t, filepath.Join(repoRoot, "runtime", "tests", "domain", "test_compensation_cleanup.py"), []byte("def test_cleanup():\n    assert True\n"))
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
		contractRequirementSection("Compensation stage 4 failure cleanup", contractRequirementBlock("@runtime @e2e", "Compensation stage 4 failure cleanup"))+
			"\n"+
			contractRequirementSection("Compensation stage 4 failure cleanup replacement", contractRequirementBlock("@runtime @e2e", "Compensation stage 4 failure cleanup replacement")))
	return repoRoot
}

func contractInactiveSupersededRequirementRepo(t *testing.T) string {
	t.Helper()

	repoRoot := copyApplicationFixtureRepo(t, "verified-spec")
	replaceSessionLifecycleDoc(t, repoRoot,
		contractRequirementSection("Compensation stage 4 failure cleanup", contractRequirementBlock("@runtime @e2e", "Compensation stage 4 failure cleanup"))+
			"\n"+
			contractRequirementSection("Compensation stage 4 failure cleanup replacement", contractRequirementBlock("@runtime @e2e", "Compensation stage 4 failure cleanup replacement")),
	)
	writeApplicationTestFile(t, filepath.Join(repoRoot, "runtime", "tests", "domain", "test_compensation_cleanup_replacement.py"), []byte("def test_cleanup_replacement():\n    assert True\n"))

	trackingPath := filepath.Join(repoRoot, ".specs", "runtime", "session-lifecycle.yaml")
	content, err := os.ReadFile(trackingPath)
	if err != nil {
		t.Fatalf("read tracking file: %v", err)
	}
	checkpoint := "a1b2c3f"
	for _, line := range strings.Split(string(content), "\n") {
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
	writeApplicationTestFile(t, trackingPath, []byte(replacement))
	return repoRoot
}

func contractDeferredSupersededResidueRepo(t *testing.T) string {
	t.Helper()

	repoRoot := copyApplicationFixtureRepo(t, "verified-spec")
	replaceSessionLifecycleDoc(t, repoRoot,
		contractRequirementSection("Compensation stage 4 failure cleanup", contractRequirementBlock("@runtime @e2e", "Compensation stage 4 failure cleanup"))+
			"\n"+
			contractRequirementSection("Compensation stage 4 failure cleanup replacement", contractRequirementBlock("@runtime @e2e", "Compensation stage 4 failure cleanup replacement")),
	)
	writeApplicationTestFile(t, filepath.Join(repoRoot, "runtime", "tests", "domain", "test_compensation_cleanup_replacement.py"), []byte("def test_cleanup_replacement():\n    assert True\n"))

	trackingPath := filepath.Join(repoRoot, ".specs", "runtime", "session-lifecycle.yaml")
	content, err := os.ReadFile(trackingPath)
	if err != nil {
		t.Fatalf("read tracking file: %v", err)
	}
	checkpoint := "a1b2c3f"
	for _, line := range strings.Split(string(content), "\n") {
		if strings.HasPrefix(line, "checkpoint: ") {
			checkpoint = strings.TrimSpace(strings.TrimPrefix(line, "checkpoint: "))
			break
		}
	}

	replacement := fmt.Sprintf(`slug: session-lifecycle
charter: runtime
title: Session Lifecycle
status: verified
rev: 5
created: 2026-03-05
updated: 2026-03-31
last_verified_at: 2026-03-31
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
    notes: Initial tracked behavior
  - id: D-002
    area: Deferred cleanup residue
    intent: change
    status: deferred
    origin_checkpoint: %s
    current: Deferred follow-up still points at the old cleanup contract
    target: Revisit whether more cleanup is needed
    notes: Historical residue kept after replacement landed
    affects_requirements:
      - REQ-001
    updates:
      - replace_requirement
  - id: D-003
    area: Compensation cleanup rewrite
    intent: change
    status: closed
    origin_checkpoint: %s
    current: Cleanup wording is outdated
    target: Replace the tracked cleanup contract
    notes: Replacement requirement is verified; predecessor remains historical only
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
    introduced_by: D-003
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
    date: 2026-03-29
    deltas_opened:
      - D-002
    deltas_closed: []
    reqs_added: []
    reqs_verified: []
    summary: Deferred a follow-up against the original cleanup requirement
  - rev: 4
    date: 2026-03-30
    deltas_opened:
      - D-003
    deltas_closed:
      - D-003
    reqs_added:
      - REQ-002
    reqs_verified:
      - REQ-002
    summary: Verified the replacement cleanup requirement while preserving the superseded predecessor as history
`, checkpoint, checkpoint, checkpoint, checkpoint)
	writeApplicationTestFile(t, trackingPath, []byte(replacement))
	return repoRoot
}

func contractDeferredSupersededResidueDedupedRepo(t *testing.T) string {
	t.Helper()

	repoRoot := copyApplicationFixtureRepo(t, "verified-spec")
	replaceSessionLifecycleDoc(t, repoRoot,
		contractRequirementSection("Compensation stage 4 failure cleanup", contractRequirementBlock("@runtime @e2e", "Compensation stage 4 failure cleanup"))+
			"\n"+
			contractRequirementSection("Compensation stage 4 failure cleanup replacement", contractRequirementBlock("@runtime @e2e", "Compensation stage 4 failure cleanup replacement")),
	)
	writeApplicationTestFile(t, filepath.Join(repoRoot, "runtime", "tests", "domain", "test_compensation_cleanup_replacement.py"), []byte("def test_cleanup_replacement():\n    assert True\n"))

	trackingPath := filepath.Join(repoRoot, ".specs", "runtime", "session-lifecycle.yaml")
	content, err := os.ReadFile(trackingPath)
	if err != nil {
		t.Fatalf("read tracking file: %v", err)
	}
	checkpoint := "a1b2c3f"
	for _, line := range strings.Split(string(content), "\n") {
		if strings.HasPrefix(line, "checkpoint: ") {
			checkpoint = strings.TrimSpace(strings.TrimPrefix(line, "checkpoint: "))
			break
		}
	}

	replacement := fmt.Sprintf(`slug: session-lifecycle
charter: runtime
title: Session Lifecycle
status: verified
rev: 6
created: 2026-03-05
updated: 2026-03-31
last_verified_at: 2026-03-31
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
    notes: Initial tracked behavior
  - id: D-002
    area: Deferred cleanup residue one
    intent: change
    status: deferred
    origin_checkpoint: %s
    current: Deferred follow-up still points at the old cleanup contract
    target: Revisit whether more cleanup is needed
    notes: Historical residue kept after replacement planning started
    affects_requirements:
      - REQ-001
    updates:
      - replace_requirement
  - id: D-003
    area: Deferred cleanup residue two
    intent: change
    status: deferred
    origin_checkpoint: %s
    current: Another deferred follow-up still points at the old cleanup contract
    target: Decide whether any cleanup remains
    notes: Additional historical residue against the same superseded requirement
    affects_requirements:
      - REQ-001
    updates:
      - replace_requirement
  - id: D-004
    area: Compensation cleanup rewrite
    intent: change
    status: closed
    origin_checkpoint: %s
    current: Cleanup wording is outdated
    target: Replace the tracked cleanup contract
    notes: Replacement requirement is verified; predecessor remains historical only
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
    introduced_by: D-004
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
    date: 2026-03-29
    deltas_opened:
      - D-002
      - D-003
    deltas_closed: []
    reqs_added: []
    reqs_verified: []
    summary: Deferred follow-ups remained attached to the original cleanup requirement
  - rev: 4
    date: 2026-03-30
    deltas_opened:
      - D-004
    deltas_closed:
      - D-004
    reqs_added:
      - REQ-002
    reqs_verified:
      - REQ-002
    summary: Verified the replacement cleanup requirement while preserving the superseded predecessor as history
`, checkpoint, checkpoint, checkpoint, checkpoint, checkpoint)
	writeApplicationTestFile(t, trackingPath, []byte(replacement))
	return repoRoot
}

func contractDeferredInventoryNoWarningRepo(t *testing.T) string {
	t.Helper()

	repoRoot := copyApplicationFixtureRepo(t, "verified-spec")
	trackingPath := filepath.Join(repoRoot, ".specs", "runtime", "session-lifecycle.yaml")
	content, err := os.ReadFile(trackingPath)
	if err != nil {
		t.Fatalf("read tracking file: %v", err)
	}
	checkpoint := "a1b2c3f"
	for _, line := range strings.Split(string(content), "\n") {
		if strings.HasPrefix(line, "checkpoint: ") {
			checkpoint = strings.TrimSpace(strings.TrimPrefix(line, "checkpoint: "))
			break
		}
	}

	replacement := fmt.Sprintf(`slug: session-lifecycle
charter: runtime
title: Session Lifecycle
status: verified
rev: 3
created: 2026-03-05
updated: 2026-03-29
last_verified_at: 2026-03-28
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
    area: Deferred follow-up
    intent: change
    status: deferred
    origin_checkpoint: %s
    current: Additional review may be needed
    target: Revisit the active requirement later
    notes: Deferred inventory only
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
    test_files:
      - runtime/tests/domain/test_compensation_cleanup.py
    gherkin: |
      @runtime @e2e
      Feature: Compensation stage 4 failure cleanup
    lifecycle: active
    verification: verified
    introduced_by: D-001
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
    date: 2026-03-29
    deltas_opened:
      - D-002
    deltas_closed: []
    reqs_added: []
    reqs_verified: []
    summary: Deferred follow-up inventory was recorded without changing the verified requirement
`, checkpoint, checkpoint, checkpoint)
	writeApplicationTestFile(t, trackingPath, []byte(replacement))
	return repoRoot
}

func contractRepairDeltaRepo(t *testing.T) string {
	t.Helper()

	repoRoot := contractActiveRequirementRepo(t)
	replaceFileText(t, filepath.Join(repoRoot, ".specs", "runtime", "session-lifecycle.yaml"), "verification: unverified", "verification: verified")
	replaceFileText(t, filepath.Join(repoRoot, ".specs", "runtime", "session-lifecycle.yaml"), "test_files: []", "test_files:\n      - runtime/tests/domain/test_compensation_cleanup.py")
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
	replaceFileText(t, filepath.Join(repoRoot, ".specs", "runtime", "session-lifecycle.yaml"), "status: ready", "status: active")
	return repoRoot
}

func contractRepairTerminalRepo(t *testing.T) string {
	t.Helper()

	repoRoot := contractRepairDeltaRepo(t)
	replaceFileText(t, filepath.Join(repoRoot, ".specs", "runtime", "session-lifecycle.yaml"), "status: in-progress", "status: closed")
	return repoRoot
}

func contractRefreshRepo(t *testing.T) string {
	t.Helper()

	repoRoot := contractActiveRequirementRepo(t)
	replaceSessionLifecycleDoc(t, repoRoot, contractRequirementSection("Compensation stage 4 failure cleanup", contractRequirementBlock("@runtime @e2e\n@critical", "Compensation stage 4 failure cleanup")))
	writeProjectConfigFixture(t, repoRoot, "gherkin_tags:\n  - runtime\n  - critical\nsource_prefixes:\n  - runtime/src/\nformats: {}\n")
	return repoRoot
}

func contractRefreshConflictRepo(t *testing.T) string {
	t.Helper()

	repoRoot := contractChangeDeltaRepo(t)
	replaceSessionLifecycleDoc(t, repoRoot, contractRequirementSection("Compensation stage 4 failure cleanup", contractRequirementBlock("@runtime @e2e\n@critical", "Compensation stage 4 failure cleanup")))
	writeProjectConfigFixture(t, repoRoot, "gherkin_tags:\n  - runtime\n  - critical\nsource_prefixes:\n  - runtime/src/\nformats: {}\n")
	return repoRoot
}

func contractVerifiedMatchBlockingRepo(t *testing.T) string {
	t.Helper()

	repoRoot := copyApplicationFixtureRepo(t, "verified-spec")
	replaceTrackedRequirementBlockOnly(t, repoRoot, "verified")
	replaceSessionLifecycleDoc(t, repoRoot, contractRequirementSection("Compensation stage 4 failure cleanup", contractRequirementBlock("@runtime @e2e\n@critical", "Compensation stage 4 failure cleanup")))
	writeProjectConfigFixture(t, repoRoot, "gherkin_tags:\n  - runtime\n  - critical\nsource_prefixes:\n  - runtime/src/\nformats: {}\n")
	return repoRoot
}

func contractDriftedDesignDocRepo(t *testing.T) string {
	t.Helper()

	repoRoot := copyApplicationFixtureRepo(t, "verified-spec")
	replaceTrackedRequirementBlockOnly(t, repoRoot, "verified")
	initGitRepoAtDate(t, repoRoot, "2026-03-28T12:00:00Z")
	replaceSessionLifecycleDoc(t, repoRoot,
		contractRequirementSection("Compensation stage 4 failure cleanup", contractRequirementBlock("@runtime @e2e", "Compensation stage 4 failure cleanup"))+
			"\n## Review Notes\n\nReviewed recovery ordering prose without changing the tracked requirement contract.\n")
	runGitAtDate(t, repoRoot, "2026-03-31T09:30:00Z", "add", ".")
	runGitAtDate(t, repoRoot, "2026-03-31T09:30:00Z", "commit", "-m", "design doc drift")
	return repoRoot
}

func contractTrackedDriftRepo(t *testing.T) string {
	t.Helper()

	repoRoot := copyApplicationFixtureRepo(t, "verified-spec")
	initGitRepoAtDate(t, repoRoot, "2026-03-28T12:00:00Z")
	replaceFileText(t, filepath.Join(repoRoot, "runtime", "src", "domain", "session_execution", "SPEC.md"), "## Requirement: Compensation stage 4 failure cleanup\n\n```gherkin requirement\n@runtime @e2e\nFeature: Compensation stage 4 failure cleanup\n```\n\n### Scenarios\n\n```gherkin scenario\nScenario: Cleanup runs after stage 4 failure\n  Given stage 4 fails during compensation\n  When recovery completes\n  Then cleanup steps run in documented order\n```\n", "## Requirement: Compensation stage 4 failure cleanup\n\n```gherkin requirement\n@runtime @e2e\nFeature: Compensation stage 4 failure cleanup\n```\n\n### Scenarios\n\n```gherkin scenario\nScenario: Cleanup runs after stage 4 failure\n  Given stage 4 fails during compensation\n  When recovery completes\n  Then cleanup steps run in documented order\n```\n\n## Requirement: Tracked drift review\n\n```gherkin requirement\n@runtime @e2e\nFeature: Tracked drift review\n```\n\n### Scenarios\n\n```gherkin scenario\nScenario: Committed drift is fully covered\n  Given the design doc changed after the checkpoint\n  When the covering delta is opened\n  Then the tracked drift continues through requirement verification\n```\n\n## Drift Review\n\nTracked drift update.\n")
	runGitAtDate(t, repoRoot, "2026-03-30T09:30:00Z", "add", ".")
	runGitAtDate(t, repoRoot, "2026-03-30T09:30:00Z", "commit", "-m", "tracked drift")

	service := newApplicationContractService(repoRoot)
	if _, _, _, err := service.AddDelta(DeltaAddRequest{
		Target:         "runtime:session-lifecycle",
		Intent:         domain.DeltaIntentAdd,
		Area:           "Tracked drift review",
		Current:        "The committed drift is not yet tracked",
		CurrentPresent: true,
		Targets:        "Track the committed design-doc change",
		TargetPresent:  true,
		Notes:          "Opened after the drift commit",
		NotesPresent:   true,
	}); err != nil {
		t.Fatalf("AddDelta: %v", err)
	}
	runGitAtDate(t, repoRoot, "2026-03-30T09:35:00Z", "add", ".specs/runtime/session-lifecycle.yaml")
	runGitAtDate(t, repoRoot, "2026-03-30T09:35:00Z", "commit", "-m", "track drift")
	if _, _, _, err := service.AddRequirement(RequirementAddRequest{
		Target:  "runtime:session-lifecycle",
		DeltaID: "D-002",
		Gherkin: "@runtime @e2e\nFeature: Tracked drift review\n\n  Scenario: Committed drift is fully covered\n    Given the design doc changed after the checkpoint\n    When the covering delta is opened\n    Then the tracked drift continues through requirement verification\n",
	}); err != nil {
		t.Fatalf("AddRequirement: %v", err)
	}
	return repoRoot
}

func contractStaleRequirementRepo(t *testing.T) string {
	t.Helper()

	repoRoot := contractActiveRequirementRepo(t)
	replaceFileText(t, filepath.Join(repoRoot, ".specs", "runtime", "session-lifecycle.yaml"), "verification: unverified", "verification: stale")
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
    current: Cleanup is still part of the contract
    target: Remove the cleanup behavior from the contract
    notes: The behavior is being retired
    affects_requirements:
      - REQ-001
    updates:
      - withdraw_requirement
`)
	return repoRoot
}

func contractCloseToRevBumpRepo(t *testing.T) string {
	t.Helper()

	repoRoot := copyApplicationFixtureRepo(t, "verified-spec")
	replaceTrackedRequirementBlockOnly(t, repoRoot, "verified")
	replaceSessionLifecycleDoc(t, repoRoot,
		contractRequirementSection("Compensation stage 4 failure cleanup", contractRequirementBlock("@runtime @e2e", "Compensation stage 4 failure cleanup"))+
			"\n"+
			contractRequirementSection("Compensation stage 4 failure cleanup replacement", contractRequirementBlock("@runtime @e2e", "Compensation stage 4 failure cleanup replacement")))
	service := newApplicationContractService(repoRoot)
	if _, _, _, err := service.AddDelta(DeltaAddRequest{
		Target:              "runtime:session-lifecycle",
		Intent:              domain.DeltaIntentChange,
		Area:                "Compensation cleanup rewrite",
		Current:             "Cleanup wording is outdated",
		CurrentPresent:      true,
		Targets:             "Replace the tracked cleanup contract",
		TargetPresent:       true,
		Notes:               "Match the new requirement block exactly",
		NotesPresent:        true,
		AffectsRequirements: []string{"REQ-001"},
	}); err != nil {
		t.Fatalf("AddDelta: %v", err)
	}
	if _, _, _, err := service.ReplaceRequirement(RequirementReplaceRequest{
		Target:        "runtime:session-lifecycle",
		RequirementID: "REQ-001",
		DeltaID:       "D-002",
		Gherkin:       contractReplacementRequirementBlock(),
	}); err != nil {
		t.Fatalf("ReplaceRequirement: %v", err)
	}
	if _, _, _, err := service.VerifyRequirement(RequirementVerifyRequest{
		Target:        "runtime:session-lifecycle",
		RequirementID: "REQ-002",
		TestFiles:     []string{"runtime/tests/domain/test_compensation_cleanup.py"},
	}); err != nil {
		t.Fatalf("VerifyRequirement: %v", err)
	}
	return repoRoot
}

func contractRevBumpReadyRepo(t *testing.T) string {
	t.Helper()

	repoRoot := contractTrackedDriftRepo(t)
	service := newApplicationContractService(repoRoot)
	if _, _, _, err := service.VerifyRequirement(RequirementVerifyRequest{
		Target:        "runtime:session-lifecycle",
		RequirementID: "REQ-002",
		TestFiles:     []string{"runtime/tests/domain/test_compensation_cleanup.py"},
	}); err != nil {
		t.Fatalf("VerifyRequirement: %v", err)
	}
	if _, _, _, err := service.CloseDelta(DeltaTransitionRequest{
		Target:  "runtime:session-lifecycle",
		DeltaID: "D-002",
	}); err != nil {
		t.Fatalf("CloseDelta: %v", err)
	}
	return repoRoot
}

func contractCodeOnlyDriftRepo(t *testing.T) string {
	t.Helper()

	repoRoot := copyApplicationFixtureRepo(t, "verified-spec")
	initGitRepoAtDate(t, repoRoot, "2026-03-28T12:00:00Z")
	codePath := filepath.Join(repoRoot, "runtime", "src", "application", "commands", "handler.py")
	writeApplicationTestFile(t, codePath, []byte("def handle():\n    return 'drift'\n"))
	runGitAtDate(t, repoRoot, "2026-03-30T09:30:00Z", "add", "runtime/src/application/commands/handler.py")
	runGitAtDate(t, repoRoot, "2026-03-30T09:30:00Z", "commit", "-m", "code drift")
	return repoRoot
}

// appendDeltaBeforeRequirements inserts the delta YAML immediately before
// the top-level `requirements:` section. Unlike appendDelta it matches
// `\nrequirements:` so it does not accidentally splice into an inner
// `affects_requirements:` field on a preceding delta, and it leaves the
// spec status untouched so callers can assert the post-append computed state.
func appendDeltaBeforeRequirements(t *testing.T, repoRoot, deltaYAML string) {
	t.Helper()

	trackingPath := filepath.Join(repoRoot, ".specs", "runtime", "session-lifecycle.yaml")
	data, err := os.ReadFile(trackingPath)
	if err != nil {
		t.Fatalf("read tracking file: %v", err)
	}
	marker := "\nrequirements:"
	if !strings.Contains(string(data), marker) {
		t.Fatalf("%s does not contain top-level requirements section", trackingPath)
	}
	updated := strings.Replace(string(data), marker, "\n"+strings.TrimRight(deltaYAML, "\n")+marker, 1)
	writeApplicationTestFile(t, trackingPath, []byte(updated))
}

// appendDelta is retained for existing callers that expect its
// side-effect of forcing status to "ready" after appending an open
// delta. New tests should prefer appendDeltaBeforeRequirements, which
// matches `\nrequirements:` (not the bare substring) and leaves the
// computed spec status untouched. The bare-substring match here can
// splice into an inner `affects_requirements:` field when an earlier
// delta carries one — `appendDeltaBeforeRequirements` is immune to
// that because of the leading newline.
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
	writeApplicationTestFile(t, trackingPath, []byte(updated))
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
	writeApplicationTestFile(t, trackingPath, []byte(content[:start]+replacement+content[end:]))
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
	writeApplicationTestFile(t, docPath, []byte(content))
}

func contractRequirementSection(title, requirementBlock string) string {
	return "## Requirement: " + title + "\n\n```gherkin requirement\n" + strings.TrimSpace(requirementBlock) + "\n```\n\n### Scenarios\n\n```gherkin scenario\nScenario: Cleanup runs after stage 4 failure\n  Given stage 4 fails during compensation\n  When recovery completes\n  Then cleanup steps run in documented order\n```\n"
}

func contractRequirementBlock(tags, title string) string {
	return strings.Join([]string{tags, "Feature: " + title}, "\n")
}

func contractReplacementRequirementBlock() string {
	return "@runtime @e2e\nFeature: Compensation stage 4 failure cleanup replacement"
}

func writeProjectConfigFixture(t *testing.T, repoRoot, content string) {
	t.Helper()

	writeApplicationTestFile(t, filepath.Join(repoRoot, ".specs", "specctl.yaml"), []byte(content))
}
