package infrastructure

import (
	"strings"
	"testing"
)

func TestAutoSelectFormatUsesDoublestarSemantics(t *testing.T) {
	config := &ProjectConfig{
		Formats: map[string]FormatConfig{
			"ui-spec": {
				Template:       "ui/src/routes/SPEC-FORMAT.md",
				RecommendedFor: "ui/src/routes/**",
				Description:    "UI routes spec",
			},
			"runtime-spec": {
				Template:       "runtime/src/domain/SPEC-FORMAT.md",
				RecommendedFor: "runtime/src/domain/**",
				Description:    "Runtime domain spec",
			},
			"e2e-context": {
				Template:       "runtime/tests/e2e/CONTEXT-FORMAT.md",
				RecommendedFor: "**/tests/e2e/**",
				Description:    "E2E context doc",
			},
		},
	}

	cases := []struct {
		name string
		path string
		want string
	}{
		{name: "ui route", path: "ui/src/routes/settings/SPEC.md", want: "ui-spec"},
		{name: "runtime domain", path: "runtime/src/domain/session_execution/SPEC.md", want: "runtime-spec"},
		{name: "nested e2e path", path: "runtime/tests/e2e/journeys/CONTEXT.md", want: "e2e-context"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			selected, err := AutoSelectFormat(config, tc.path)
			if err != nil {
				t.Fatalf("AutoSelectFormat(%q): %v", tc.path, err)
			}
			if selected == nil || *selected != tc.want {
				t.Fatalf("AutoSelectFormat(%q) = %v, want %q", tc.path, selected, tc.want)
			}
		})
	}
}

func TestAutoSelectFormatReturnsNilWhenNoConfiguredPatternMatches(t *testing.T) {
	config := &ProjectConfig{
		Formats: map[string]FormatConfig{
			"runtime-spec": {
				Template:       "runtime/src/domain/SPEC-FORMAT.md",
				RecommendedFor: "runtime/src/domain/**",
				Description:    "Runtime domain spec",
			},
			"e2e-context": {
				Template:       "runtime/tests/e2e/CONTEXT-FORMAT.md",
				RecommendedFor: "**/tests/e2e/**",
				Description:    "E2E context doc",
			},
		},
	}

	selected, err := AutoSelectFormat(config, "docs/architecture/ADR-0001.md")
	if err != nil {
		t.Fatalf("AutoSelectFormat() error = %v, want nil", err)
	}
	if selected != nil {
		t.Fatalf("AutoSelectFormat() = %v, want nil when no pattern matches", selected)
	}
}

func TestAutoSelectFormatRejectsAmbiguousDoublestarMatches(t *testing.T) {
	config := &ProjectConfig{
		Formats: map[string]FormatConfig{
			"runtime-spec": {
				Template:       "runtime/src/domain/SPEC-FORMAT.md",
				RecommendedFor: "runtime/src/domain/**",
				Description:    "Runtime domain spec",
			},
			"e2e-context": {
				Template:       "runtime/tests/e2e/CONTEXT-FORMAT.md",
				RecommendedFor: "**/tests/e2e/**",
				Description:    "E2E context doc",
			},
		},
	}

	selected, err := AutoSelectFormat(config, "runtime/src/domain/tests/e2e/session_execution/CONTEXT.md")
	if err == nil || !strings.Contains(err.Error(), "multiple configured formats match") {
		t.Fatalf("AutoSelectFormat() error = %v, want ambiguous match error", err)
	}
	if selected != nil {
		t.Fatalf("AutoSelectFormat() = %v, want nil on ambiguity", selected)
	}
}
