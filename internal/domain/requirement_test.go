package domain

import (
	"reflect"
	"strings"
	"testing"
)

func TestIsValidReqID(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"REQ-001", true},
		{"REQ-010", true},
		{"REQ-999", true},
		{"REQ-01", false},
		{"REQ-0001", false},
		{"REQ001", false},
		{"req-001", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := IsValidReqID(tt.input)
			if got != tt.want {
				t.Errorf("IsValidReqID(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestParseReqIDNumber(t *testing.T) {
	n, err := ParseReqIDNumber("REQ-007")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if n != 7 {
		t.Errorf("expected 7, got %d", n)
	}

	_, err = ParseReqIDNumber("invalid")
	if err == nil {
		t.Error("expected error for invalid ID")
	}
}

func TestExtractGherkinTags(t *testing.T) {
	tests := []struct {
		input string
		want  []string
	}{
		{"@runtime @domain", []string{"runtime", "domain"}},
		{"@e2e @cross-boundary @manual", []string{"e2e", "cross-boundary", "manual"}},
		{"no tags here", []string{}},
		{"@single", []string{"single"}},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := ExtractGherkinTags(tt.input)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ExtractGherkinTags(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestDeriveRequirementTitleAndTags(t *testing.T) {
	gherkin := "@runtime @e2e\n@cross-boundary\nFeature: Recovery handshake\n\n  Scenario: Recover after disconnect\n    Given a dropped session\n    When recovery starts\n    Then the handshake completes"

	title, err := DeriveRequirementTitle(gherkin)
	if err != nil {
		t.Fatalf("DeriveRequirementTitle: %v", err)
	}
	if title != "Recovery handshake" {
		t.Fatalf("title = %q, want %q", title, "Recovery handshake")
	}

	tags, err := DeriveRequirementTags(gherkin)
	if err != nil {
		t.Fatalf("DeriveRequirementTags: %v", err)
	}
	wantTags := []string{"runtime", "e2e", "cross-boundary"}
	if !reflect.DeepEqual(tags, wantTags) {
		t.Fatalf("tags = %v, want %v", tags, wantTags)
	}
}

func TestValidateRequirementSequenceRejectsDerivedMismatches(t *testing.T) {
	tests := []struct {
		name        string
		requirement Requirement
		want        string
	}{
		{
			name: "title mismatch",
			requirement: Requirement{
				ID:           "REQ-001",
				Title:        "Wrong title",
				Tags:         []string{"runtime"},
				Gherkin:      "@runtime\nFeature: Correct title",
				Lifecycle:    RequirementLifecycleActive,
				Verification: RequirementVerificationUnverified,
				IntroducedBy: "D-001",
			},
			want: "title must match",
		},
		{
			name: "tag mismatch",
			requirement: Requirement{
				ID:           "REQ-001",
				Title:        "Correct title",
				Tags:         []string{"runtime", "domain"},
				Gherkin:      "@runtime\nFeature: Correct title",
				Lifecycle:    RequirementLifecycleActive,
				Verification: RequirementVerificationUnverified,
				IntroducedBy: "D-001",
			},
			want: "tags must match",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateRequirementSequence([]Requirement{tt.requirement})
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("expected %q, got %v", tt.want, err)
			}
		})
	}
}

func TestNewRequirementDerivesTitleAndTags(t *testing.T) {
	requirement, err := NewRequirement(
		"REQ-009",
		"D-009",
		"@runtime @e2e\nFeature: Recovery handshake\n\n  Scenario: Recover after disconnect\n    Given a dropped session\n    When recovery starts\n    Then the handshake completes",
	)
	if err != nil {
		t.Fatalf("NewRequirement: %v", err)
	}

	if requirement.Title != "Recovery handshake" {
		t.Fatalf("title = %q, want %q", requirement.Title, "Recovery handshake")
	}
	wantTags := []string{"runtime", "e2e"}
	if !reflect.DeepEqual(requirement.Tags, wantTags) {
		t.Fatalf("tags = %v, want %v", requirement.Tags, wantTags)
	}
	if requirement.Verified {
		t.Fatal("new requirements must start unverified")
	}
	if len(requirement.TestFiles) != 0 {
		t.Fatalf("expected no persisted test files on new requirement, got %v", requirement.TestFiles)
	}
}

func TestValidateRequirementTagsConfigured(t *testing.T) {
	if err := ValidateRequirementTagsConfigured([]string{"runtime", "e2e", "manual"}, []string{"runtime", "domain"}); err != nil {
		t.Fatalf("semantic tags should always be valid: %v", err)
	}

	err := ValidateRequirementTagsConfigured([]string{"runtime", "adapter"}, []string{"runtime", "domain"})
	if err == nil || !strings.Contains(err.Error(), `"adapter" is not configured`) {
		t.Fatalf("expected unconfigured tag failure, got %v", err)
	}
}
