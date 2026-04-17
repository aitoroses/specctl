package infrastructure

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestDefaultConfigMatchesV2Defaults(t *testing.T) {
	config := DefaultConfig()

	wantTags := []string{"runtime", "domain", "ui", "integration", "contract", "workflow"}
	if !reflect.DeepEqual(config.GherkinTags, wantTags) {
		t.Fatalf("gherkin tags = %v, want %v", config.GherkinTags, wantTags)
	}

	wantPrefixes := []string{"runtime/src/", "ui/src/", "ui/convex/", "ui/server/"}
	if !reflect.DeepEqual(config.SourcePrefixes, wantPrefixes) {
		t.Fatalf("source prefixes = %v, want %v", config.SourcePrefixes, wantPrefixes)
	}
}

func TestLoadProjectConfigOmitsPersistedSemanticTags(t *testing.T) {
	specsDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(specsDir, "specctl.yaml"), []byte(`gherkin_tags:
  - runtime
  - e2e
  - contract
  - manual
source_prefixes:
  - runtime/src/
formats: {}
`), 0644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	config, err := LoadProjectConfig(specsDir)
	if err != nil {
		t.Fatalf("LoadProjectConfig: %v", err)
	}

	want := []string{"runtime", "contract"}
	if !reflect.DeepEqual(config.GherkinTags, want) {
		t.Fatalf("gherkin tags = %v, want %v", config.GherkinTags, want)
	}
}

func TestLoadProjectConfigRejectsMalformedConfigInsteadOfDefaulting(t *testing.T) {
	specsDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(specsDir, "specctl.yaml"), []byte(`gherkin_tags:
  - runtime
source_prefixes:
  - ../runtime/src/
formats: {}
`), 0644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	config, err := LoadProjectConfig(specsDir)
	if err == nil || !strings.Contains(err.Error(), "source_prefixes[0]") {
		t.Fatalf("expected source_prefixes validation error, got config=%v err=%v", config, err)
	}
}

func TestLoadProjectConfigLenientReturnsValidationFindings(t *testing.T) {
	specsDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(specsDir, "specctl.yaml"), []byte(`gherkin_tags:
  - runtime
source_prefixes:
  - ../runtime/src/
formats: {}
`), 0644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	config, findings, err := LoadProjectConfigLenient(specsDir)
	if err != nil {
		t.Fatalf("LoadProjectConfigLenient: %v", err)
	}
	if config == nil {
		t.Fatal("expected config")
	}
	if len(findings) == 0 || findings[0].Code != "CONFIG_PREFIX_INVALID" {
		t.Fatalf("findings = %#v", findings)
	}
}

func TestLoadProjectConfigLenientWarnsOnRedundantSemanticTags(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	specsDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(specsDir, "specctl.yaml"), []byte(`gherkin_tags:
  - runtime
  - manual
source_prefixes:
  - runtime/src/
formats: {}
`), 0644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	config, findings, err := LoadProjectConfigLenient(specsDir)
	if err != nil {
		t.Fatalf("LoadProjectConfigLenient: %v", err)
	}
	if !reflect.DeepEqual(config.GherkinTags, []string{"runtime"}) {
		t.Fatalf("gherkin tags = %v", config.GherkinTags)
	}
	if len(findings) == 0 || findings[0].Code != "REDUNDANT_SEMANTIC_TAG" || findings[0].Severity != "warning" {
		t.Fatalf("findings = %#v", findings)
	}
}

func TestLoadProjectConfigLenientWarnsOnRedundantSemanticTagsOnlyOncePerPersistedState(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	specsDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(specsDir, "specctl.yaml"), []byte(`gherkin_tags:
  - runtime
  - manual
source_prefixes:
  - runtime/src/
formats: {}
`), 0644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	_, firstFindings, err := LoadProjectConfigLenient(specsDir)
	if err != nil {
		t.Fatalf("LoadProjectConfigLenient first: %v", err)
	}
	if len(firstFindings) == 0 || firstFindings[0].Code != "REDUNDANT_SEMANTIC_TAG" {
		t.Fatalf("first findings = %#v", firstFindings)
	}

	_, secondFindings, err := LoadProjectConfigLenient(specsDir)
	if err != nil {
		t.Fatalf("LoadProjectConfigLenient second: %v", err)
	}
	if len(secondFindings) != 0 {
		t.Fatalf("second findings = %#v, want none after first warning", secondFindings)
	}
}

func TestFormatConfigJSONUsesCanonicalSnakeCase(t *testing.T) {
	data, err := json.Marshal(FormatConfig{
		Template:       "ui/src/routes/SPEC-FORMAT.md",
		RecommendedFor: "ui/src/routes/**",
		Description:    "UI spec",
	})
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	var got map[string]any
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if got["template"] != "ui/src/routes/SPEC-FORMAT.md" {
		t.Fatalf("template = %#v", got["template"])
	}
	if got["recommended_for"] != "ui/src/routes/**" {
		t.Fatalf("recommended_for = %#v", got["recommended_for"])
	}
	if got["description"] != "UI spec" {
		t.Fatalf("description = %#v", got["description"])
	}
	if _, exists := got["RecommendedFor"]; exists {
		t.Fatalf("unexpected CamelCase key in %#v", got)
	}
}
