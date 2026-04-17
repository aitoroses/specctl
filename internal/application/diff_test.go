package application

import (
	"errors"
	"reflect"
	"testing"

	"github.com/aitoroses/specctl/internal/domain"
)

func TestDiffDesignDocSectionsTracksHeadingChanges(t *testing.T) {
	baseline := []byte(`---
spec: session-lifecycle
charter: runtime
---
# Session Lifecycle

## Stable Section
Stable baseline.

## Modified Section
Baseline content.

## Removed Section
To be removed.
`)
	current := []byte(`---
spec: session-lifecycle
charter: runtime
---
# Session Lifecycle

## Stable Section
Stable baseline.

## Modified Section
Current content changed.

## Added Section
Added content.
`)

	got := diffDesignDocSections(baseline, current)
	want := []DesignDocSectionDiff{
		{Heading: "Modified Section", Type: "modified", Lines: [2]int{10, 12}},
		{Heading: "Added Section", Type: "added", Lines: [2]int{13, 14}},
		{Heading: "Removed Section", Type: "removed", Lines: [2]int{13, 14}},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("diffDesignDocSections() = %#v, want %#v", got, want)
	}
}

func TestDiffDesignDocSectionsUsesHeadingOrdinalOccurrence(t *testing.T) {
	baseline := []byte(`---
spec: session-lifecycle
charter: runtime
---
# Session Lifecycle

## Step
First stays the same.

## Step
Second changes here.
`)
	current := []byte(`---
spec: session-lifecycle
charter: runtime
---
# Session Lifecycle

## Step
First stays the same.

## Step
Second now differs.
`)

	got := diffDesignDocSections(baseline, current)
	want := []DesignDocSectionDiff{
		{Heading: "Step", Type: "modified", Lines: [2]int{10, 11}},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("diffDesignDocSections() = %#v, want %#v", got, want)
	}
}

func TestDiffDesignDocSectionsNormalizesHeadingTextAndKeepsPhysicalLineNumbers(t *testing.T) {
	baseline := []byte(`---
spec: session-lifecycle
charter: runtime
format: runtime-spec
---
# Session Lifecycle

##   Deployment   Plan ###
Baseline content.

### Nested Detail
Unchanged nested section.
`)
	current := []byte(`---
spec: session-lifecycle
charter: runtime
format: runtime-spec
---
# Session Lifecycle

## deployment plan
Current content changed.

### Nested Detail
Unchanged nested section.
`)

	got := diffDesignDocSections(baseline, current)
	want := []DesignDocSectionDiff{
		{Heading: "deployment plan", Type: "modified", Lines: [2]int{8, 10}},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("diffDesignDocSections() = %#v, want %#v", got, want)
	}
}

func TestDiffDesignDocSectionsTreatsHeadingRenameAsRemoveAndAdd(t *testing.T) {
	baseline := []byte(`---
spec: session-lifecycle
charter: runtime
format: runtime-spec
---
# Session Lifecycle

## Deployment Plan
Keep the same body.
`)
	current := []byte(`---
spec: session-lifecycle
charter: runtime
format: runtime-spec
---
# Session Lifecycle

## Rollout Plan
Keep the same body.
`)

	got := diffDesignDocSections(baseline, current)
	want := []DesignDocSectionDiff{
		{Heading: "Deployment Plan", Type: "removed", Lines: [2]int{8, 9}},
		{Heading: "Rollout Plan", Type: "added", Lines: [2]int{8, 9}},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("diffDesignDocSections() = %#v, want %#v", got, want)
	}
}

func TestDiffDesignDocSectionsTreatsSplitMergeEditsAsRemoveAndAdd(t *testing.T) {
	baseline := []byte(`---
spec: session-lifecycle
charter: runtime
format: runtime-spec
---
# Session Lifecycle

## Combined Plan
Baseline content.
`)
	current := []byte(`---
spec: session-lifecycle
charter: runtime
format: runtime-spec
---
# Session Lifecycle

## Delivery Plan
Baseline content.

## Rollout Plan
Additional rollout detail.
`)

	got := diffDesignDocSections(baseline, current)
	want := []DesignDocSectionDiff{
		{Heading: "Combined Plan", Type: "removed", Lines: [2]int{8, 9}},
		{Heading: "Delivery Plan", Type: "added", Lines: [2]int{8, 10}},
		{Heading: "Rollout Plan", Type: "added", Lines: [2]int{11, 12}},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("diffDesignDocSections() = %#v, want %#v", got, want)
	}
}

func TestDiffDesignDocSectionsTreatsDuplicateHeadingRenameAsOccurrenceShift(t *testing.T) {
	baseline := []byte(`---
spec: session-lifecycle
charter: runtime
format: runtime-spec
---
# Session Lifecycle

## Step
First step body.

## Step
Second stable body.
`)
	current := []byte(`---
spec: session-lifecycle
charter: runtime
format: runtime-spec
---
# Session Lifecycle

## Renamed Step
First step body.

## Step
Second stable body.
`)

	got := diffDesignDocSections(baseline, current)
	want := []DesignDocSectionDiff{
		{Heading: "Renamed Step", Type: "added", Lines: [2]int{8, 10}},
		{Heading: "Step", Type: "modified", Lines: [2]int{11, 12}},
		{Heading: "Step", Type: "removed", Lines: [2]int{11, 12}},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("diffDesignDocSections() = %#v, want %#v", got, want)
	}
}

func TestDiffDesignDocSectionsShiftsDuplicateHeadingIdentityWhenAnEarlierOccurrenceIsInserted(t *testing.T) {
	baseline := []byte(`---
spec: session-lifecycle
charter: runtime
format: runtime-spec
---
# Session Lifecycle

## Step
Existing first body.

## Step
Existing second body.
`)
	current := []byte(`---
spec: session-lifecycle
charter: runtime
format: runtime-spec
---
# Session Lifecycle

## Step
Inserted first body.

## Step
Existing first body.

## Step
Existing second body.
`)

	got := diffDesignDocSections(baseline, current)
	want := []DesignDocSectionDiff{
		{Heading: "Step", Type: "modified", Lines: [2]int{8, 10}},
		{Heading: "Step", Type: "modified", Lines: [2]int{11, 13}},
		{Heading: "Step", Type: "added", Lines: [2]int{14, 15}},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("diffDesignDocSections() = %#v, want %#v", got, want)
	}
}

func TestDiffDesignDocSectionsUsesHeadingOccurrenceInsteadOfContentSimilarity(t *testing.T) {
	baseline := []byte(`---
spec: session-lifecycle
charter: runtime
format: runtime-spec
---
# Session Lifecycle

## Step
First body.

## Step
Second body.
`)
	current := []byte(`---
spec: session-lifecycle
charter: runtime
format: runtime-spec
---
# Session Lifecycle

## Step
Second body.

## Step
First body.
`)

	got := diffDesignDocSections(baseline, current)
	want := []DesignDocSectionDiff{
		{Heading: "Step", Type: "modified", Lines: [2]int{8, 10}},
		{Heading: "Step", Type: "modified", Lines: [2]int{11, 12}},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("diffDesignDocSections() = %#v, want %#v", got, want)
	}
}

func TestBuildSpecDiffProjectionInitialBaselineUsesCanonicalEmptyBuckets(t *testing.T) {
	current := &domain.TrackingFile{
		Slug:           "session-lifecycle",
		Charter:        "runtime",
		Title:          "Session Lifecycle",
		Status:         domain.SpecStatusActive,
		Rev:            1,
		Created:        "2026-03-05",
		Updated:        "2026-03-30",
		LastVerifiedAt: "2026-03-28",
		Checkpoint:     "a1b2c3f",
		Documents:      domain.Documents{Primary: "runtime/src/domain/session_execution/SPEC.md"},
		Tags:           []string{"runtime", "domain"},
		Scope:          []string{"runtime/src/domain/session_execution/", "runtime/src/application/commands/"},
		Deltas: []domain.Delta{{
			ID:               "D-001",
			Area:             "Compensation stage 4",
			Status:           domain.DeltaStatusClosed,
			OriginCheckpoint: "a1b2c3f",
			Current:          "Cleanup behavior is undocumented",
			Target:           "Document the compensation cleanup contract",
			Notes:            "Initial semantic baseline",
		}},
		Requirements: []domain.Requirement{{
			ID:           "REQ-001",
			Title:        "Cleanup is documented",
			Tags:         []string{"runtime"},
			Verification: domain.RequirementVerificationVerified,
			IntroducedBy: "D-001",
			Gherkin:      "@runtime\nFeature: Cleanup is documented",
		}},
		Changelog: []domain.ChangelogEntry{},
	}

	diff := buildSpecDiffProjection("runtime:session-lifecycle", current, nil, nil, nil, nil, nil, nil, "", nil)
	if diff.Model.Status.From != nil || diff.Model.Status.To != domain.SpecStatusVerified {
		t.Fatalf("status = %#v", diff.Model.Status)
	}
	if len(diff.Model.Deltas.Opened) != 0 || len(diff.Model.Deltas.Closed) != 0 || len(diff.Model.Deltas.Deferred) != 0 || len(diff.Model.Deltas.Resumed) != 0 {
		t.Fatalf("deltas = %#v", diff.Model.Deltas)
	}
	if len(diff.Model.Requirements.Added) != 0 || len(diff.Model.Requirements.Verified) != 0 {
		t.Fatalf("requirements = %#v", diff.Model.Requirements)
	}
	if got := diff.Model.SpecTags; !reflect.DeepEqual(got, DiffSetModel{Added: []string{"runtime", "domain"}, Removed: []string{}}) {
		t.Fatalf("spec_tags = %#v", got)
	}
	if got := diff.Model.Documents; got.PrimaryFrom != nil || got.PrimaryTo != "runtime/src/domain/session_execution/SPEC.md" {
		t.Fatalf("documents = %#v", got)
	}
	if got := diff.Model.Scope; !reflect.DeepEqual(got, DiffSetModel{Added: []string{"runtime/src/domain/session_execution/", "runtime/src/application/commands/"}, Removed: []string{}}) {
		t.Fatalf("scope = %#v", got)
	}
}

func TestDiffDeltasIncludesCanonicalPayloadForStatusTransitions(t *testing.T) {
	previous := []domain.Delta{
		{ID: "D-001", Area: "Closed work", OriginCheckpoint: "aaa1111", Status: domain.DeltaStatusOpen, Current: "Current closed", Target: "Target closed"},
		{ID: "D-002", Area: "Deferred work", OriginCheckpoint: "bbb2222", Status: domain.DeltaStatusInProgress, Current: "Current deferred", Target: "Target deferred"},
		{ID: "D-003", Area: "Resumed work", OriginCheckpoint: "ccc3333", Status: domain.DeltaStatusDeferred, Current: "Current resumed", Target: "Target resumed"},
	}
	current := []domain.Delta{
		{ID: "D-001", Area: "Closed work", OriginCheckpoint: "aaa1111", Status: domain.DeltaStatusClosed, Current: "Current closed", Target: "Target closed"},
		{ID: "D-002", Area: "Deferred work", OriginCheckpoint: "bbb2222", Status: domain.DeltaStatusDeferred, Current: "Current deferred", Target: "Target deferred"},
		{ID: "D-003", Area: "Resumed work", OriginCheckpoint: "ccc3333", Status: domain.DeltaStatusOpen, Current: "Current resumed", Target: "Target resumed"},
		{ID: "D-004", Area: "Opened work", OriginCheckpoint: "ddd4444", Status: domain.DeltaStatusOpen, Current: "Current opened", Target: "Target opened"},
	}

	diff := diffDeltas(previous, current)
	if got := diff.Opened; !reflect.DeepEqual(got, []DiffDeltaSummary{{
		ID:               "D-004",
		Area:             "Opened work",
		OriginCheckpoint: "ddd4444",
		Status:           domain.DeltaStatusOpen,
		Current:          "Current opened",
		Target:           "Target opened",
	}}) {
		t.Fatalf("opened = %#v", got)
	}
	if got := diff.Closed; !reflect.DeepEqual(got, []DiffDeltaSummary{{
		ID:               "D-001",
		Area:             "Closed work",
		OriginCheckpoint: "aaa1111",
		Status:           domain.DeltaStatusClosed,
		Current:          "Current closed",
		Target:           "Target closed",
	}}) {
		t.Fatalf("closed = %#v", got)
	}
	if got := diff.Deferred; !reflect.DeepEqual(got, []DiffDeltaSummary{{
		ID:               "D-002",
		Area:             "Deferred work",
		OriginCheckpoint: "bbb2222",
		Status:           domain.DeltaStatusDeferred,
		Current:          "Current deferred",
		Target:           "Target deferred",
	}}) {
		t.Fatalf("deferred = %#v", got)
	}
	if got := diff.Resumed; !reflect.DeepEqual(got, []DiffDeltaSummary{{
		ID:               "D-003",
		Area:             "Resumed work",
		OriginCheckpoint: "ccc3333",
		Status:           domain.DeltaStatusOpen,
		Current:          "Current resumed",
		Target:           "Target resumed",
	}}) {
		t.Fatalf("resumed = %#v", got)
	}
}

func TestCheckpointUnavailableDiffFindingUsesCanonicalCheckpointPayload(t *testing.T) {
	finding := checkpointUnavailableDiffFinding("/repo", "runtime", "session-lifecycle", errors.New("read runtime/src/domain/missing/SPEC.md at deadbee: fatal: path does not exist"))
	if finding.Code != "CHECKPOINT_UNAVAILABLE" || finding.Severity != "error" || finding.Path != ".specs/runtime/session-lifecycle.yaml" || finding.Target != "checkpoint" {
		t.Fatalf("finding = %#v", finding)
	}
}
