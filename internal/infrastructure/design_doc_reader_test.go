package infrastructure

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReadDesignDocFrontmatterWithConfiguredFormat(t *testing.T) {
	fixture := fixtureRoot("normalized-path-spec")
	frontmatter, err := ReadDesignDocFrontmatter(fixture, "runtime/src/domain/session_execution/SPEC.md")
	if err != nil {
		t.Fatalf("ReadDesignDocFrontmatter: %v", err)
	}
	if frontmatter.Spec != "session-lifecycle" {
		t.Fatalf("spec = %q, want session-lifecycle", frontmatter.Spec)
	}
	if frontmatter.Charter != "runtime" {
		t.Fatalf("charter = %q, want runtime", frontmatter.Charter)
	}
	if frontmatter.Format != "runtime-spec" {
		t.Fatalf("format = %q, want runtime-spec", frontmatter.Format)
	}
}

func TestValidateTrackedDesignDocumentRejectsFrontmatterMismatch(t *testing.T) {
	fixture := fixtureRoot("mismatched-frontmatter-spec")
	err := ValidateTrackedDesignDocument(fixture, filepath.ToSlash("runtime/src/domain/session_execution/SPEC.md"), "runtime", "session-lifecycle")
	if err == nil {
		t.Fatal("expected frontmatter mismatch error")
	}
}

func TestReadDesignDocFrontmatterRejectsUnknownConfiguredFormat(t *testing.T) {
	fixture := fixtureRoot("unknown-format-spec")
	_, err := ReadDesignDocFrontmatter(fixture, "runtime/src/domain/session_execution/SPEC.md")
	if err == nil || !strings.Contains(err.Error(), "unknown format") {
		t.Fatalf("expected unknown format error, got %v", err)
	}
}

func TestReadDesignDocFrontmatterRejectsUnknownKeys(t *testing.T) {
	repoRoot := t.TempDir()
	if err := os.MkdirAll(filepath.Join(repoRoot, ".specs"), 0755); err != nil {
		t.Fatalf("mkdir .specs: %v", err)
	}
	if err := os.WriteFile(filepath.Join(repoRoot, ".specs", "specctl.yaml"), []byte("gherkin_tags: []\nsource_prefixes: []\nformats: {}\n"), 0644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(repoRoot, "runtime", "src", "domain", "session_execution"), 0755); err != nil {
		t.Fatalf("mkdir doc dir: %v", err)
	}
	content := "---\nspec: session-lifecycle\ncharter: runtime\nformat: \"\"\nextra: true\n---\n# Session Lifecycle\n"
	if err := os.WriteFile(filepath.Join(repoRoot, "runtime", "src", "domain", "session_execution", "SPEC.md"), []byte(content), 0644); err != nil {
		t.Fatalf("write design doc: %v", err)
	}

	_, err := ReadDesignDocFrontmatter(repoRoot, "runtime/src/domain/session_execution/SPEC.md")
	if err == nil || !strings.Contains(err.Error(), "field extra not found") {
		t.Fatalf("expected strict frontmatter error, got %v", err)
	}
}

func TestReadDesignDocFrontmatterFailsOnMalformedConfig(t *testing.T) {
	repoRoot := t.TempDir()
	writeProjectConfig(t, repoRoot, `gherkin_tags:
  - runtime
source_prefixes:
  - ../runtime/src/
formats: {}
`)
	if err := os.MkdirAll(filepath.Join(repoRoot, "runtime", "src", "domain", "session_execution"), 0755); err != nil {
		t.Fatalf("mkdir doc dir: %v", err)
	}
	content := "---\nspec: session-lifecycle\ncharter: runtime\n---\n# Session Lifecycle\n"
	if err := os.WriteFile(filepath.Join(repoRoot, "runtime", "src", "domain", "session_execution", "SPEC.md"), []byte(content), 0644); err != nil {
		t.Fatalf("write design doc: %v", err)
	}

	_, err := ReadDesignDocFrontmatter(repoRoot, "runtime/src/domain/session_execution/SPEC.md")
	if err == nil || !strings.Contains(err.Error(), "loading specctl config") {
		t.Fatalf("expected config loading error, got %v", err)
	}
}
