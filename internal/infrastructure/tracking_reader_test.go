package infrastructure

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/aitoroses/specctl/internal/domain"
)

func TestReadTrackingFile_V2Fixtures(t *testing.T) {
	tests := []struct {
		name       string
		fixture    string
		wantStatus domain.SpecStatus
	}{
		{name: "empty", fixture: "empty-spec", wantStatus: domain.SpecStatusDraft},
		{name: "ready", fixture: "ready-spec", wantStatus: domain.SpecStatusReady},
		{name: "active", fixture: "active-spec", wantStatus: domain.SpecStatusActive},
		{name: "verified", fixture: "verified-spec", wantStatus: domain.SpecStatusVerified},
		{name: "deferred-only", fixture: "deferred-only-spec", wantStatus: domain.SpecStatusDraft},
		{name: "normalized-paths", fixture: "normalized-path-spec", wantStatus: domain.SpecStatusDraft},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join(fixtureRoot(tt.fixture), ".specs", "runtime", "session-lifecycle.yaml")
			tracking, err := ReadTrackingFile(path)
			if err != nil {
				t.Fatalf("ReadTrackingFile: %v", err)
			}
			if tracking.Status != tt.wantStatus {
				t.Fatalf("status = %s, want %s", tracking.Status, tt.wantStatus)
			}
			if strings.Contains(tracking.Documents.Primary, "./") || strings.Contains(tracking.Documents.Primary, "..") {
				t.Fatalf("documents.primary %q was not normalized", tracking.Documents.Primary)
			}
			for _, scope := range tracking.Scope {
				if !strings.HasSuffix(scope, "/") {
					t.Fatalf("scope %q was not normalized as a directory", scope)
				}
			}
			for _, requirement := range tracking.Requirements {
				for _, testFile := range requirement.TestFiles {
					if strings.Contains(testFile, "./") || strings.Contains(testFile, "..") {
						t.Fatalf("test file %q was not normalized", testFile)
					}
				}
			}
		})
	}
}

func TestReadTrackingFile_RejectsMalformedTrackingFiles(t *testing.T) {
	tests := []struct {
		name    string
		fixture string
		want    string
	}{
		{name: "missing required key", fixture: "missing-required-key", want: "missing required key"},
		{name: "bad ID", fixture: "bad-id-spec", want: "expected D-001, found D-01"},
		{name: "gapful IDs", fixture: "malformed-gapful-spec", want: "sequential"},
		{name: "missing notes", fixture: "malformed-missing-notes", want: `every delta must include "notes"`},
		{name: "bad scope", fixture: "malformed-bad-scope", want: "scope"},
		{name: "missing frontmatter", fixture: "missing-frontmatter-spec", want: "frontmatter is missing"},
		{name: "invalid frontmatter", fixture: "invalid-frontmatter-spec", want: "frontmatter is invalid"},
		{name: "mismatched frontmatter", fixture: "mismatched-frontmatter-spec", want: "does not match tracking slug"},
		{name: "unknown format", fixture: "unknown-format-spec", want: "unknown format"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join(fixtureRoot(tt.fixture), ".specs", "runtime", "session-lifecycle.yaml")
			_, err := ReadTrackingFile(path)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("expected error containing %q, got %v", tt.want, err)
			}
		})
	}
}

func TestReadTrackingFileLenientReturnsProjectionAndFindings(t *testing.T) {
	path := filepath.Join(fixtureRoot("malformed-gapful-spec"), ".specs", "runtime", "session-lifecycle.yaml")

	tracking, findings, err := ReadTrackingFileLenient(path)
	if err != nil {
		t.Fatalf("ReadTrackingFileLenient: %v", err)
	}
	if tracking == nil || tracking.Slug != "session-lifecycle" {
		t.Fatalf("tracking = %#v", tracking)
	}
	if len(findings) == 0 || findings[0].Code != "IDS_NON_SEQUENTIAL" {
		t.Fatalf("findings = %#v", findings)
	}
}

func TestReadTrackingFileRejectsMissingNestedKeys(t *testing.T) {
	repoRoot := t.TempDir()
	path := writeTrackingFixture(t, repoRoot, trackingFixtureOptions{
		TrackingYAML: `slug: session-lifecycle
charter: runtime
title: Session Lifecycle
status: active
rev: 1
created: 2026-03-05
updated: 2026-03-30
last_verified_at: 2026-03-28
checkpoint: a1b2c3f
tags:
  - runtime
documents:
  primary: runtime/src/domain/session_execution/SPEC.md
scope:
  - runtime/src/domain/session_execution/
deltas: []
requirements:
  - id: REQ-001
    title: Session lifecycle requirement
    tags:
      - runtime
    lifecycle: active
    verification: unverified
    introduced_by: D-001
    gherkin: |
      @runtime
      Feature: Session lifecycle requirement

        Scenario: Works
          Given a
          When b
          Then c
changelog: []`,
	})

	_, err := ReadTrackingFile(path)
	if err == nil || !strings.Contains(err.Error(), `every requirement must include "test_files"`) {
		t.Fatalf("expected missing nested key error, got %v", err)
	}
}

func TestReadTrackingFileRejectsMissingPrimaryDocument(t *testing.T) {
	repoRoot := t.TempDir()
	path := writeTrackingFixture(t, repoRoot, trackingFixtureOptions{
		TrackingYAML: `slug: session-lifecycle
charter: runtime
title: Session Lifecycle
status: active
rev: 1
created: 2026-03-05
updated: 2026-03-30
last_verified_at: 2026-03-28
checkpoint: a1b2c3f
tags:
  - runtime
documents:
  primary: runtime/src/domain/session_execution/SPEC.md
scope:
  - runtime/src/domain/session_execution/
deltas: []
requirements: []
changelog: []`,
		CreateDesignDoc: false,
	})

	_, err := ReadTrackingFile(path)
	if err == nil || !strings.Contains(err.Error(), "documents.primary does not exist") {
		t.Fatalf("expected missing primary document error, got %v", err)
	}
}

func TestReadTrackingFileRejectsScopeEntryThatIsAFile(t *testing.T) {
	repoRoot := t.TempDir()
	path := writeTrackingFixture(t, repoRoot, trackingFixtureOptions{
		TrackingYAML: `slug: session-lifecycle
charter: runtime
title: Session Lifecycle
status: active
rev: 1
created: 2026-03-05
updated: 2026-03-30
last_verified_at: 2026-03-28
checkpoint: a1b2c3f
tags:
  - runtime
documents:
  primary: runtime/src/domain/session_execution/SPEC.md
scope:
  - runtime/src/domain/session_execution/SPEC.md
deltas: []
requirements: []
changelog: []`,
		CreateDesignDoc: true,
	})

	_, err := ReadTrackingFile(path)
	if err == nil || !strings.Contains(err.Error(), "must point to a directory") {
		t.Fatalf("expected scope directory error, got %v", err)
	}
}

func TestReadTrackingFileNormalizesVerifiedTestFiles(t *testing.T) {
	repoRoot := t.TempDir()
	path := writeTrackingFixture(t, repoRoot, trackingFixtureOptions{
		TrackingYAML: `slug: session-lifecycle
charter: runtime
title: Session Lifecycle
status: verified
rev: 1
created: 2026-03-05
updated: 2026-03-30
last_verified_at: 2026-03-28
checkpoint: a1b2c3f
tags:
  - runtime
documents:
  primary: ./runtime/src/domain/session_execution/../session_execution/SPEC.md
scope:
  - ./runtime/src/domain/session_execution
deltas:
  - id: D-001
    area: Compensation stage 4
    status: closed
    origin_checkpoint: a1b2c3f
    current: Current state
    target: Target state
    notes: Explicitly provided
requirements:
  - id: REQ-001
    title: Compensation stage 4 failure cleanup
    tags:
      - runtime
      - e2e
    test_files:
      - ./runtime/tests/domain/../domain/test_compensation_cleanup.py
    lifecycle: active
    verification: verified
    introduced_by: D-001
    gherkin: |
      @runtime @e2e
      Feature: Compensation stage 4 failure cleanup

        Scenario: Cleanup runs after stage 4 failure
          Given stage 4 fails during compensation
          When recovery completes
          Then cleanup steps run in documented order
changelog: []`,
		CreateDesignDoc: true,
		CreateTestFile:  true,
	})

	tracking, err := ReadTrackingFile(path)
	if err != nil {
		t.Fatalf("ReadTrackingFile: %v", err)
	}
	if got := tracking.Requirements[0].TestFiles[0]; got != "runtime/tests/domain/test_compensation_cleanup.py" {
		t.Fatalf("test file = %q, want normalized repo path", got)
	}
}

func TestReadTrackingFileAllowsVerifiedManualRequirementWithoutTestFiles(t *testing.T) {
	repoRoot := t.TempDir()
	path := writeTrackingFixture(t, repoRoot, trackingFixtureOptions{
		TrackingYAML: `slug: session-lifecycle
charter: runtime
title: Session Lifecycle
status: verified
rev: 1
created: 2026-03-05
updated: 2026-03-30
last_verified_at: 2026-03-28
checkpoint: a1b2c3f
tags:
  - runtime
documents:
  primary: runtime/src/domain/session_execution/SPEC.md
scope:
  - runtime/src/domain/session_execution/
deltas:
  - id: D-001
    area: Session lifecycle cleanup
    status: closed
    origin_checkpoint: a1b2c3f
    current: Current state
    target: Target state
    notes: Explicitly provided
requirements:
  - id: REQ-001
    title: Session lifecycle cleanup
    tags:
      - runtime
      - manual
    test_files: []
    lifecycle: active
    verification: verified
    introduced_by: D-001
    gherkin: |
      @runtime @manual
      Feature: Session lifecycle cleanup

        Scenario: Manual verification remains valid
          Given the workflow has been exercised manually
          When the reviewer confirms the expected result
          Then the requirement is verified
changelog: []`,
		CreateDesignDoc: true,
	})

	tracking, err := ReadTrackingFile(path)
	if err != nil {
		t.Fatalf("ReadTrackingFile: %v", err)
	}
	if !tracking.Requirements[0].IsManual() {
		t.Fatal("expected manual requirement to remain manual")
	}
	if len(tracking.Requirements[0].TestFiles) != 0 {
		t.Fatalf("manual requirement test files = %v, want empty", tracking.Requirements[0].TestFiles)
	}
}

func TestReadTrackingFileFailsOnMalformedConfig(t *testing.T) {
	repoRoot := t.TempDir()
	writeProjectConfig(t, repoRoot, `gherkin_tags:
  - runtime
source_prefixes:
  - ../runtime/src/
formats: {}
`)
	path := writeTrackingFixture(t, repoRoot, trackingFixtureOptions{})

	_, err := ReadTrackingFile(path)
	if err == nil || !strings.Contains(err.Error(), "loading specctl config") {
		t.Fatalf("expected config loading error, got %v", err)
	}
	if !strings.Contains(err.Error(), "source_prefixes[0]") {
		t.Fatalf("expected config validation details, got %v", err)
	}
}

func TestReadTrackingFileRejectsMissingVerifiedTestFile(t *testing.T) {
	repoRoot := t.TempDir()
	path := writeTrackingFixture(t, repoRoot, trackingFixtureOptions{
		TrackingYAML: `slug: session-lifecycle
charter: runtime
title: Session Lifecycle
status: verified
rev: 1
created: 2026-03-05
updated: 2026-03-30
last_verified_at: 2026-03-28
checkpoint: a1b2c3f
tags:
  - runtime
documents:
  primary: runtime/src/domain/session_execution/SPEC.md
scope:
  - runtime/src/domain/session_execution/
deltas:
  - id: D-001
    area: Compensation stage 4
    status: closed
    origin_checkpoint: a1b2c3f
    current: Current state
    target: Target state
    notes: Explicitly provided
requirements:
  - id: REQ-001
    title: Compensation stage 4 failure cleanup
    tags:
      - runtime
      - e2e
    test_files:
      - runtime/tests/domain/test_compensation_cleanup.py
    lifecycle: active
    verification: verified
    introduced_by: D-001
    gherkin: |
      @runtime @e2e
      Feature: Compensation stage 4 failure cleanup

        Scenario: Cleanup runs after stage 4 failure
          Given stage 4 fails during compensation
          When recovery completes
          Then cleanup steps run in documented order
changelog: []`,
		CreateDesignDoc: true,
	})

	_, err := ReadTrackingFile(path)
	if err == nil || !strings.Contains(err.Error(), "test file does not exist") {
		t.Fatalf("expected missing test file error, got %v", err)
	}
}

func TestReadTrackingFileRejectsUnconfiguredRequirementTags(t *testing.T) {
	repoRoot := t.TempDir()
	writeProjectConfig(t, repoRoot, `gherkin_tags:
  - runtime
source_prefixes:
  - runtime/src/
formats: {}
`)
	path := writeTrackingFixture(t, repoRoot, trackingFixtureOptions{
		TrackingYAML: `slug: session-lifecycle
charter: runtime
title: Session Lifecycle
status: draft
rev: 1
created: 2026-03-05
updated: 2026-03-30
last_verified_at: 2026-03-28
checkpoint: a1b2c3f
tags:
  - runtime
documents:
  primary: runtime/src/domain/session_execution/SPEC.md
scope:
  - runtime/src/domain/session_execution/
deltas:
  - id: D-001
    area: Session lifecycle cleanup
    status: open
    origin_checkpoint: a1b2c3f
    current: Current state
    target: Target state
    notes: Explicitly provided
requirements:
  - id: REQ-001
    title: Session lifecycle cleanup
    tags:
      - runtime
      - adapter
    test_files: []
    lifecycle: active
    verification: unverified
    introduced_by: D-001
    gherkin: |
      @runtime @adapter
      Feature: Session lifecycle cleanup

        Scenario: Tag validation remains strict
          Given the config omits adapter
          When the tracking file is loaded
          Then validation fails
changelog: []`,
		CreateDesignDoc: true,
	})

	_, err := ReadTrackingFile(path)
	if err == nil || !strings.Contains(err.Error(), `"adapter" is not configured`) {
		t.Fatalf("expected unconfigured tag error, got %v", err)
	}
}

func TestReadTrackingFileAllowsSemanticRequirementTagsWithoutConfigEntries(t *testing.T) {
	repoRoot := t.TempDir()
	writeProjectConfig(t, repoRoot, `gherkin_tags:
  - runtime
source_prefixes:
  - runtime/src/
formats: {}
`)
	path := writeTrackingFixture(t, repoRoot, trackingFixtureOptions{
		TrackingYAML: `slug: session-lifecycle
charter: runtime
title: Session Lifecycle
status: active
rev: 1
created: 2026-03-05
updated: 2026-03-30
last_verified_at: 2026-03-28
checkpoint: a1b2c3f
tags:
  - runtime
documents:
  primary: runtime/src/domain/session_execution/SPEC.md
scope:
  - runtime/src/domain/session_execution/
deltas:
  - id: D-001
    area: Session lifecycle cleanup
    status: open
    origin_checkpoint: a1b2c3f
    current: Current state
    target: Target state
    notes: Explicitly provided
requirements:
  - id: REQ-001
    title: Session lifecycle cleanup
    tags:
      - runtime
      - e2e
      - manual
    test_files: []
    lifecycle: active
    verification: unverified
    introduced_by: D-001
    gherkin: |
      @runtime @e2e @manual
      Feature: Session lifecycle cleanup

        Scenario: Semantic tags stay valid
          Given the config omits semantic tags
          When the tracking file is loaded
          Then validation still passes
changelog: []`,
		CreateDesignDoc: true,
	})

	if _, err := ReadTrackingFile(path); err != nil {
		t.Fatalf("semantic tags should remain valid: %v", err)
	}
}

func TestReadCharter_V2Fixtures(t *testing.T) {
	charterPath := filepath.Join(fixtureRoot("charter-dag"), ".specs", "runtime", "CHARTER.yaml")
	charter, err := ReadCharter(charterPath)
	if err != nil {
		t.Fatalf("ReadCharter: %v", err)
	}
	if charter.Description == "" {
		t.Fatal("expected charter description to be populated")
	}
	ordered, err := charter.OrderedSpecs()
	if err != nil {
		t.Fatalf("OrderedSpecs: %v", err)
	}
	got := []string{ordered[0].Slug, ordered[1].Slug, ordered[2].Slug}
	want := []string{"redis-state", "recovery-projection", "session-lifecycle"}
	for i := range got {
		if got[i] != want[i] {
			t.Fatalf("ordered specs = %v, want %v", got, want)
		}
	}
}

func TestReadCharter_RejectsSchemaViolations(t *testing.T) {
	tests := []struct {
		name    string
		fixture string
		want    string
	}{
		{name: "cycle", fixture: "charter-cycle", want: "cycle"},
		{name: "missing description", fixture: "charter-missing-description", want: "description"},
		{name: "missing notes", fixture: "charter-missing-notes", want: "notes"},
		{name: "unknown group", fixture: "charter-unknown-group", want: "unknown group"},
		{name: "bad dependency", fixture: "charter-bad-dependency", want: "unknown spec"},
		{name: "missing tracking", fixture: "charter-missing-tracking", want: "does not have a tracking file"},
		{name: "mismatched tracking", fixture: "charter-mismatched-tracking", want: "does not match path slug"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join(fixtureRoot(tt.fixture), ".specs", "runtime", "CHARTER.yaml")
			_, err := ReadCharter(path)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("expected error containing %q, got %v", tt.want, err)
			}
		})
	}
}

func TestReadCharterLenientReturnsProjectionAndFindings(t *testing.T) {
	path := filepath.Join(fixtureRoot("charter-cycle"), ".specs", "runtime", "CHARTER.yaml")

	charter, findings, err := ReadCharterLenient(path)
	if err != nil {
		t.Fatalf("ReadCharterLenient: %v", err)
	}
	if charter == nil || charter.Name != "runtime" {
		t.Fatalf("charter = %#v", charter)
	}
	if len(findings) == 0 || findings[0].Code != "CHARTER_CYCLE_PRESENT" {
		t.Fatalf("findings = %#v", findings)
	}
}

func TestReadCharterLenientUnknownDependencyDoesNotReportCycle(t *testing.T) {
	path := filepath.Join(fixtureRoot("charter-bad-dependency"), ".specs", "runtime", "CHARTER.yaml")

	charter, findings, err := ReadCharterLenient(path)
	if err != nil {
		t.Fatalf("ReadCharterLenient: %v", err)
	}
	if charter == nil || charter.Name != "runtime" {
		t.Fatalf("charter = %#v", charter)
	}

	foundDependencyInvalid := false
	for _, finding := range findings {
		if finding.Code == "CHARTER_DEPENDENCY_INVALID" {
			foundDependencyInvalid = true
		}
		if finding.Code == "CHARTER_CYCLE_PRESENT" {
			t.Fatalf("findings = %#v, want no cycle finding for unknown dependency", findings)
		}
	}
	if !foundDependencyInvalid {
		t.Fatalf("findings = %#v", findings)
	}
}

func TestReadCharterLenientUsesCharterCatalogCodesForMissingMetadata(t *testing.T) {
	repoRoot := t.TempDir()
	charterPath := writeCharterFixture(t, repoRoot, `name: runtime
groups:
  - key: execution
    title: Execution Engine
    order: 10
specs:
  - slug: redis-state
    group: execution
    order: 10
    depends_on: []
    notes: Storage and CAS guarantees
`)

	_, findings, err := ReadCharterLenient(charterPath)
	if err != nil {
		t.Fatalf("ReadCharterLenient: %v", err)
	}
	if len(findings) == 0 {
		t.Fatal("expected charter validation findings")
	}
	for _, finding := range findings {
		if finding.Code != "CHARTER_NAME_INVALID" {
			t.Fatalf("finding.Code = %q, want CHARTER_NAME_INVALID (%#v)", finding.Code, finding)
		}
		if finding.Code == "SPEC_STATUS_INVALID" {
			t.Fatalf("unexpected fallback finding %#v", finding)
		}
	}
}

func TestReadCharterAllowsDetachedTrackingFile(t *testing.T) {
	repoRoot := t.TempDir()
	charterPath := writeCharterFixture(t, repoRoot, `name: runtime
title: Runtime System
description: Specs for runtime control-plane and data-plane behavior
groups:
  - key: execution
    title: Execution Engine
    order: 10
specs:
  - slug: redis-state
    group: execution
    order: 10
    depends_on: []
    notes: Storage and CAS guarantees
`)

	writeAdditionalTrackingFile(t, repoRoot, "rogue-spec")

	charter, err := ReadCharter(charterPath)
	if err != nil {
		t.Fatalf("ReadCharter: %v", err)
	}
	if len(charter.Specs) != 1 || charter.Specs[0].Slug != "redis-state" {
		t.Fatalf("charter specs = %#v", charter.Specs)
	}
}

func fixtureRoot(name string) string {
	return filepath.Join("..", "..", "testdata", "v2", name)
}

type trackingFixtureOptions struct {
	TrackingYAML    string
	CreateDesignDoc bool
	CreateTestFile  bool
}

func writeTrackingFixture(t *testing.T, repoRoot string, opts trackingFixtureOptions) string {
	t.Helper()

	if err := os.MkdirAll(filepath.Join(repoRoot, ".specs", "runtime"), 0755); err != nil {
		t.Fatalf("mkdir tracking dir: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(repoRoot, "runtime", "src", "domain", "session_execution"), 0755); err != nil {
		t.Fatalf("mkdir design doc dir: %v", err)
	}
	if opts.CreateDesignDoc || opts.TrackingYAML == "" {
		if err := os.WriteFile(
			filepath.Join(repoRoot, "runtime", "src", "domain", "session_execution", "SPEC.md"),
			[]byte("---\nspec: session-lifecycle\ncharter: runtime\n---\n# Session Lifecycle\n"),
			0644,
		); err != nil {
			t.Fatalf("write design doc: %v", err)
		}
	}
	if opts.CreateTestFile {
		if err := os.MkdirAll(filepath.Join(repoRoot, "runtime", "tests", "domain"), 0755); err != nil {
			t.Fatalf("mkdir test dir: %v", err)
		}
		if err := os.WriteFile(filepath.Join(repoRoot, "runtime", "tests", "domain", "test_compensation_cleanup.py"), []byte("def test_cleanup():\n    pass\n"), 0644); err != nil {
			t.Fatalf("write test file: %v", err)
		}
	}
	if opts.TrackingYAML == "" {
		opts.TrackingYAML = `slug: session-lifecycle
charter: runtime
title: Session Lifecycle
status: draft
rev: 1
created: 2026-03-05
updated: 2026-03-30
last_verified_at: 2026-03-28
checkpoint: a1b2c3f
tags:
  - runtime
documents:
  primary: runtime/src/domain/session_execution/SPEC.md
scope:
  - runtime/src/domain/session_execution/
deltas: []
requirements: []
changelog: []`
	}

	path := filepath.Join(repoRoot, ".specs", "runtime", "session-lifecycle.yaml")
	if err := os.WriteFile(path, []byte(opts.TrackingYAML), 0644); err != nil {
		t.Fatalf("write tracking file: %v", err)
	}
	return path
}

func writeCharterFixture(t *testing.T, repoRoot, charterYAML string) string {
	t.Helper()

	if err := os.MkdirAll(filepath.Join(repoRoot, ".specs", "runtime"), 0755); err != nil {
		t.Fatalf("mkdir charter dir: %v", err)
	}
	path := filepath.Join(repoRoot, ".specs", "runtime", "CHARTER.yaml")
	if err := os.WriteFile(path, []byte(charterYAML), 0644); err != nil {
		t.Fatalf("write charter: %v", err)
	}
	writeAdditionalTrackingFile(t, repoRoot, "redis-state")
	return path
}

func writeAdditionalTrackingFile(t *testing.T, repoRoot, slug string) {
	t.Helper()

	if err := os.MkdirAll(filepath.Join(repoRoot, ".specs", "runtime"), 0755); err != nil {
		t.Fatalf("mkdir tracking dir: %v", err)
	}
	specDir := filepath.Join(repoRoot, "runtime", "src", "domain", strings.ReplaceAll(slug, "-", "_"))
	if err := os.MkdirAll(specDir, 0755); err != nil {
		t.Fatalf("mkdir spec dir: %v", err)
	}
	docPath := filepath.Join(specDir, "SPEC.md")
	docContent := fmt.Sprintf("---\nspec: %s\ncharter: runtime\n---\n# %s\n", slug, slug)
	if err := os.WriteFile(docPath, []byte(docContent), 0644); err != nil {
		t.Fatalf("write design doc: %v", err)
	}

	trackingPath := filepath.Join(repoRoot, ".specs", "runtime", slug+".yaml")
	trackingYAML := fmt.Sprintf(`slug: %s
charter: runtime
title: %s
status: draft
rev: 1
created: 2026-03-05
updated: 2026-03-30
last_verified_at: 2026-03-28
checkpoint: a1b2c3f
tags:
  - runtime
documents:
  primary: runtime/src/domain/%s/SPEC.md
scope:
  - runtime/src/domain/%s/
deltas: []
requirements: []
changelog: []
`, slug, slug, strings.ReplaceAll(slug, "-", "_"), strings.ReplaceAll(slug, "-", "_"))
	if err := os.WriteFile(trackingPath, []byte(trackingYAML), 0644); err != nil {
		t.Fatalf("write tracking: %v", err)
	}
}

func writeProjectConfig(t *testing.T, repoRoot, content string) {
	t.Helper()

	if err := os.MkdirAll(filepath.Join(repoRoot, ".specs"), 0755); err != nil {
		t.Fatalf("mkdir specs dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(repoRoot, ".specs", "specctl.yaml"), []byte(content), 0644); err != nil {
		t.Fatalf("write specctl config: %v", err)
	}
}
