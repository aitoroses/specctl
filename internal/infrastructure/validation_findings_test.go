package infrastructure

import (
	"path/filepath"
	"testing"
)

func TestReadTrackingFileLenientUsesSpecCatalogCodes(t *testing.T) {
	tests := []struct {
		name     string
		fixture  string
		wantCode string
	}{
		{name: "gapful delta ids", fixture: "malformed-gapful-spec", wantCode: "IDS_NON_SEQUENTIAL"},
		{name: "bad delta id", fixture: "bad-id-spec", wantCode: "DELTA_ID_INVALID"},
		{name: "bad scope path", fixture: "malformed-bad-scope", wantCode: "SCOPE_PATH_INVALID"},
		{name: "mismatched frontmatter", fixture: "mismatched-frontmatter-spec", wantCode: "PRIMARY_DOC_FRONTMATTER_MISMATCH"},
		{name: "unknown design doc format", fixture: "unknown-format-spec", wantCode: "FORMAT_NOT_CONFIGURED"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join(fixtureRoot(tt.fixture), ".specs", "runtime", "session-lifecycle.yaml")
			_, findings, err := ReadTrackingFileLenient(path)
			if err != nil {
				t.Fatalf("ReadTrackingFileLenient(%q): %v", tt.fixture, err)
			}
			if len(findings) == 0 {
				t.Fatalf("expected findings for %q", tt.fixture)
			}
			if findings[0].Code != tt.wantCode {
				t.Fatalf("finding[0].Code = %q, want %q", findings[0].Code, tt.wantCode)
			}
			if findings[0].Code == "VALIDATION_ERROR" || findings[0].Code == "STATUS_MISMATCH" {
				t.Fatalf("non-spec validation code %#v", findings[0])
			}
		})
	}
}

func TestValidationFindingsFromMessagesUsesCharterCatalogCodes(t *testing.T) {
	findings := ValidationFindingsFromMessages(`charter "runtime" does not list spec "session-lifecycle"`, ".specs/runtime/CHARTER.yaml", "session-lifecycle")
	if len(findings) != 1 {
		t.Fatalf("findings = %#v", findings)
	}
	if findings[0].Code != "CHARTER_SPEC_MISSING" {
		t.Fatalf("findings[0].Code = %q", findings[0].Code)
	}
	if findings[0].Target != "session-lifecycle" {
		t.Fatalf("findings[0].Target = %q", findings[0].Target)
	}
}

func TestValidationFindingsFromMessagesUsesDependencyCatalogCode(t *testing.T) {
	findings := ValidationFindingsFromMessages(`spec "redis-state" depends on unknown spec "missing-spec"`, ".specs/runtime/CHARTER.yaml", "redis-state")
	if len(findings) != 1 {
		t.Fatalf("findings = %#v", findings)
	}
	if findings[0].Code != "CHARTER_DEPENDENCY_INVALID" {
		t.Fatalf("findings[0].Code = %q", findings[0].Code)
	}
	if findings[0].Target != "redis-state" {
		t.Fatalf("findings[0].Target = %q", findings[0].Target)
	}
}

func TestValidationFindingsFromMessagesKeepsSemicolonDelimitedSequenceErrorsWhole(t *testing.T) {
	findings := ValidationFindingsFromMessages(
		"delta IDs must be sequential without gaps; expected D-001, found D-01; title is required",
		".specs/runtime/session-lifecycle.yaml",
		"",
	)
	if len(findings) != 2 {
		t.Fatalf("findings = %#v", findings)
	}
	if findings[0].Code != "DELTA_ID_INVALID" || findings[0].Target != "deltas" {
		t.Fatalf("findings[0] = %#v", findings[0])
	}
	if findings[1].Code != "SPEC_TITLE_INVALID" || findings[1].Target != "title" {
		t.Fatalf("findings[1] = %#v", findings[1])
	}
}

func TestValidationFindingsFromMessagesUsesCharterCatalogCodesForMetadataErrors(t *testing.T) {
	tests := []struct {
		name string
		raw  string
	}{
		{
			name: "split semicolon validation errors",
			raw:  "title is required; description is required",
		},
		{
			name: "missing required keys",
			raw:  `charter file is missing required key "title"; charter file is missing required key "description"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			findings := ValidationFindingsFromMessages(tt.raw, ".specs/runtime/CHARTER.yaml", "")
			if len(findings) != 2 {
				t.Fatalf("findings = %#v", findings)
			}
			for _, finding := range findings {
				if finding.Code != "CHARTER_NAME_INVALID" {
					t.Fatalf("finding.Code = %q, want CHARTER_NAME_INVALID (%#v)", finding.Code, finding)
				}
				if finding.Code == "SPEC_STATUS_INVALID" {
					t.Fatalf("unexpected fallback finding %#v", finding)
				}
			}
		})
	}
}
