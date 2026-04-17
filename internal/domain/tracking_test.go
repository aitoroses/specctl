package domain

import (
	"strings"
	"testing"
)

func TestTrackingFileModel_ExactSchemaExample(t *testing.T) {
	checkpoint := "a1b2c3f"
	tracking := &TrackingFile{
		Slug:           "session-lifecycle",
		Charter:        "runtime",
		Title:          "Session Lifecycle",
		Status:         SpecStatusActive,
		Rev:            9,
		Created:        "2026-03-05",
		Updated:        "2026-03-30",
		LastVerifiedAt: "2026-03-28",
		Checkpoint:     checkpoint,
		Tags:           []string{"runtime", "domain", "fsm"},
		Documents:      Documents{Primary: "runtime/src/domain/session_execution/SPEC.md"},
		Scope: []string{
			"runtime/src/domain/session_execution/",
			"runtime/src/adapters/outbound/lifecycle/",
			"runtime/src/application/commands/",
		},
		Deltas: []Delta{
			{
				ID:               "D-007",
				Area:             "Turn cancellation path",
				Status:           DeltaStatusClosed,
				OriginCheckpoint: "94c2de1",
				Current:          "Turn cancellation is implemented but daemon propagation is undocumented",
				Target:           "Document propagation path and verify it with tests",
				Notes:            "Triggered by flaky cancellation cleanup in staging",
			},
			{
				ID:               "D-008",
				Area:             "Compensation stage 4",
				Status:           DeltaStatusInProgress,
				OriginCheckpoint: "a1b2c3f",
				Current:          "Stage 4 compensation exists in code but failure ordering is unclear",
				Target:           "Document ordering and verify failure cleanup",
				Notes:            "Multi-agent implementation split between runtime and workflow work",
			},
		},
		Requirements: []Requirement{
			{
				ID:           "REQ-005",
				Title:        "Turn cancellation propagates to daemon",
				Tags:         []string{"runtime", "domain"},
				TestFiles:    []string{"runtime/tests/domain/test_cancellation.py"},
				Gherkin:      "@runtime @domain\nFeature: Turn cancellation propagates to daemon",
				Lifecycle:    RequirementLifecycleActive,
				Verification: RequirementVerificationVerified,
				IntroducedBy: "D-007",
			},
			{
				ID:           "REQ-006",
				Title:        "Compensation stage 4 failure cleanup",
				Tags:         []string{"runtime", "e2e"},
				TestFiles:    []string{},
				Gherkin:      "@runtime @e2e\nFeature: Compensation stage 4 failure cleanup",
				Lifecycle:    RequirementLifecycleActive,
				Verification: RequirementVerificationUnverified,
				IntroducedBy: "D-008",
			},
		},
		Changelog: []ChangelogEntry{
			{
				Rev:          8,
				Date:         "2026-03-28",
				DeltasOpened: []string{},
				DeltasClosed: []string{"D-006"},
				ReqsAdded:    []string{},
				ReqsVerified: []string{"REQ-004"},
				Summary:      "Added BOOTING guard verification and closed the related gap",
			},
			{
				Rev:          9,
				Date:         "2026-03-30",
				DeltasOpened: []string{"D-008"},
				DeltasClosed: []string{"D-007"},
				ReqsAdded:    []string{"REQ-006"},
				ReqsVerified: []string{"REQ-005"},
				Summary:      "Split cancellation and compensation follow-up into separate tracked work",
			},
		},
	}

	if got := tracking.ComputedStatus(); got != SpecStatusActive {
		t.Fatalf("expected active status from SPEC.md tracking example, got %s", got)
	}
	if !tracking.Requirements[1].IsE2E() {
		t.Fatal("expected REQ-006 to carry the semantic @e2e flag")
	}
}

func TestTrackingFileStatusRecomputation(t *testing.T) {
	base := validTrackingFile()
	base.Requirements = nil
	base.Deltas = nil
	base.Status = SpecStatusDraft
	if got := base.ComputedStatus(); got != SpecStatusDraft {
		t.Fatalf("expected draft, got %s", got)
	}

	ready := validTrackingFile()
	ready.Requirements = nil
	ready.Status = SpecStatusReady
	if got := ready.ComputedStatus(); got != SpecStatusReady {
		t.Fatalf("expected ready, got %s", got)
	}

	active := validTrackingFile()
	active.Deltas[0].Status = DeltaStatusInProgress
	active.Status = SpecStatusActive
	if got := active.ComputedStatus(); got != SpecStatusActive {
		t.Fatalf("expected active, got %s", got)
	}

	verified := validTrackingFile()
	verified.Status = SpecStatusVerified
	verified.Deltas[0].Status = DeltaStatusClosed
	verified.Requirements[0].Verification = RequirementVerificationVerified
	verified.Requirements[0].TestFiles = []string{"runtime/tests/domain/test_handler.py"}
	if got := verified.ComputedStatus(); got != SpecStatusVerified {
		t.Fatalf("expected verified, got %s", got)
	}

	deferredOnly := validTrackingFile()
	deferredOnly.Status = SpecStatusDraft
	deferredOnly.Deltas[0].Status = DeltaStatusDeferred
	if got := deferredOnly.ComputedStatus(); got != SpecStatusDraft {
		t.Fatalf("expected deferred-only spec to compute to draft, got %s", got)
	}

	untracedLive := validTrackingFile()
	untracedLive.Requirements = nil
	untracedLive.Status = SpecStatusReady
	if got := untracedLive.ComputedStatus(); got != SpecStatusReady {
		t.Fatalf("expected untraced live delta to compute to ready, got %s", got)
	}

	sync := validTrackingFile()
	sync.Deltas[0].Status = DeltaStatusClosed
	sync.Requirements[0].Verification = RequirementVerificationVerified
	sync.Requirements[0].TestFiles = []string{"runtime/tests/domain/test_handler.py"}
	sync.SyncComputedStatus()
	if sync.Status != SpecStatusVerified {
		t.Fatalf("expected synced status to become verified, got %s", sync.Status)
	}
}

func TestNextIDs(t *testing.T) {
	tracking := validTrackingFile()
	nextDelta, err := tracking.AllocateNextDeltaID()
	if err != nil {
		t.Fatalf("next delta ID: %v", err)
	}
	if nextDelta != "D-002" {
		t.Fatalf("expected D-002, got %s", nextDelta)
	}

	nextRequirement, err := tracking.AllocateNextRequirementID()
	if err != nil {
		t.Fatalf("next requirement ID: %v", err)
	}
	if nextRequirement != "REQ-002" {
		t.Fatalf("expected REQ-002, got %s", nextRequirement)
	}

	if _, err := NextDeltaID([]Delta{
		{ID: "D-001", Area: "First", Status: DeltaStatusOpen, OriginCheckpoint: "a1b2c3f", Current: "Current", Target: "Target", Notes: ""},
		{ID: "D-003", Area: "Third", Status: DeltaStatusOpen, OriginCheckpoint: "a1b2c3f", Current: "Current", Target: "Target", Notes: ""},
	}); err == nil || !strings.Contains(err.Error(), "sequential") {
		t.Fatalf("expected gapful delta IDs to fail, got %v", err)
	}
	if _, err := NextRequirementID([]Requirement{
		{ID: "REQ-001", Title: "Feature one", Tags: []string{}, Gherkin: "Feature: Feature one", Lifecycle: RequirementLifecycleActive, Verification: RequirementVerificationUnverified, IntroducedBy: "D-001"},
		{ID: "REQ-003", Title: "Feature three", Tags: []string{}, Gherkin: "Feature: Feature three", Lifecycle: RequirementLifecycleActive, Verification: RequirementVerificationUnverified, IntroducedBy: "D-001"},
	}); err == nil || !strings.Contains(err.Error(), "sequential") {
		t.Fatalf("expected gapful requirement IDs to fail, got %v", err)
	}
}

func TestTrackingFileDeltaClosureHelpers(t *testing.T) {
	tests := []struct {
		name              string
		mutate            func(*TrackingFile)
		wantBlockingErr   string
		wantValidationErr string
		wantBlocks        []string
	}{
		{
			name: "missing traces rejected",
			mutate: func(tracking *TrackingFile) {
				tracking.Requirements = nil
			},
			wantBlockingErr:   "cannot be closed until required updates are resolved",
			wantValidationErr: "cannot be closed until required updates are resolved",
		},
		{
			name:              "unverified trace blocks close",
			mutate:            func(tracking *TrackingFile) {},
			wantBlocks:        []string{"REQ-001"},
			wantValidationErr: "cannot be closed until requirement REQ-001 is verified",
		},
		{
			name: "verified traces allow close",
			mutate: func(tracking *TrackingFile) {
				tracking.Requirements[0].Verification = RequirementVerificationVerified
				tracking.Requirements[0].TestFiles = []string{"runtime/tests/domain/test_handler.py"}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tracking := validTrackingFile()
			tt.mutate(tracking)

			blocking, err := tracking.BlockingRequirementsForDeltaClosure("D-001")
			if tt.wantBlockingErr != "" && (err == nil || !strings.Contains(err.Error(), tt.wantBlockingErr)) {
				t.Fatalf("expected error containing %q, got %v", tt.wantBlockingErr, err)
			}
			if tt.wantBlockingErr == "" && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			gotBlocks := make([]string, 0, len(blocking))
			for _, requirement := range blocking {
				gotBlocks = append(gotBlocks, requirement.ID)
			}
			if len(tt.wantBlocks) != len(gotBlocks) {
				t.Fatalf("blocking requirements = %v, want %v", gotBlocks, tt.wantBlocks)
			}
			for i := range tt.wantBlocks {
				if gotBlocks[i] != tt.wantBlocks[i] {
					t.Fatalf("blocking requirements = %v, want %v", gotBlocks, tt.wantBlocks)
				}
			}

			err = tracking.ValidateDeltaClosure("D-001")
			if tt.wantValidationErr != "" && (err == nil || !strings.Contains(err.Error(), tt.wantValidationErr)) {
				t.Fatalf("expected validation error containing %q, got %v", tt.wantValidationErr, err)
			}
			if tt.wantValidationErr == "" && err != nil {
				t.Fatalf("unexpected validation error: %v", err)
			}
		})
	}
}

func TestTrackingFileValidateRejectsNonNormalizedStoredPaths(t *testing.T) {
	tracking := validTrackingFile()
	tracking.Documents.Primary = "./runtime/src/domain/session_execution/../session_execution/SPEC.md"
	tracking.Scope = []string{"runtime/src/domain/session_execution"}
	tracking.Requirements[0].TestFiles = []string{"runtime/tests/domain/"}

	err := tracking.Validate()
	if err == nil {
		t.Fatal("expected path validation error")
	}
	for _, snippet := range []string{
		"documents.primary",
		"scope",
		"test_file",
	} {
		if !strings.Contains(err.Error(), snippet) {
			t.Fatalf("expected %q in validation error, got %v", snippet, err)
		}
	}
}

func TestTrackingFileValidateRejectsClosedDeltasWithoutVerifiedTraces(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func(*TrackingFile)
		wantErr string
	}{
		{
			name: "closed delta without traces",
			mutate: func(tracking *TrackingFile) {
				tracking.Deltas[0].Status = DeltaStatusClosed
				tracking.Requirements = nil
				tracking.Status = SpecStatusReady
			},
			wantErr: "cannot be closed until required updates are resolved",
		},
		{
			name: "closed delta with unverified trace",
			mutate: func(tracking *TrackingFile) {
				tracking.Deltas[0].Status = DeltaStatusClosed
				tracking.Status = SpecStatusReady
			},
			wantErr: "cannot be closed until requirement REQ-001 is verified",
		},
		{
			name: "requirement title mismatch",
			mutate: func(tracking *TrackingFile) {
				tracking.Requirements[0].Title = "Wrong title"
			},
			wantErr: "title must match the Gherkin Feature line",
		},
		{
			name: "requirement tag mismatch",
			mutate: func(tracking *TrackingFile) {
				tracking.Requirements[0].Tags = []string{"runtime"}
			},
			wantErr: "tags must match Gherkin tag lines",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tracking := validTrackingFile()
			tt.mutate(tracking)
			err := tracking.Validate()
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("expected %q, got %v", tt.wantErr, err)
			}
		})
	}
}

func validTrackingFile() *TrackingFile {
	checkpoint := "a1b2c3f"
	return &TrackingFile{
		Slug:           "session-lifecycle",
		Charter:        "runtime",
		Title:          "Session Lifecycle",
		Status:         SpecStatusActive,
		Rev:            1,
		Created:        "2026-03-05",
		Updated:        "2026-03-30",
		LastVerifiedAt: "2026-03-28",
		Checkpoint:     checkpoint,
		Tags:           []string{"runtime", "domain"},
		Documents:      Documents{Primary: "runtime/src/domain/session_execution/SPEC.md"},
		Scope:          []string{"runtime/src/domain/session_execution/", "runtime/src/application/commands/"},
		Deltas: []Delta{
			{
				ID:               "D-001",
				Area:             "Compensation stage 4",
				Status:           DeltaStatusOpen,
				OriginCheckpoint: checkpoint,
				Current:          "Current state",
				Target:           "Target state",
				Notes:            "",
			},
		},
		Requirements: []Requirement{
			{
				ID:           "REQ-001",
				Title:        "Compensation stage 4 failure cleanup",
				Tags:         []string{"runtime", "e2e"},
				Gherkin:      "@runtime @e2e\nFeature: Compensation stage 4 failure cleanup",
				Lifecycle:    RequirementLifecycleActive,
				Verification: RequirementVerificationUnverified,
				IntroducedBy: "D-001",
			},
		},
		Changelog: []ChangelogEntry{
			{
				Rev:     1,
				Date:    "2026-03-28",
				Summary: "Initial tracking state",
			},
		},
	}
}
