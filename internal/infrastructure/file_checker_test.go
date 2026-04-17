package infrastructure

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFindProjectRoot_FromCharterSpec(t *testing.T) {
	tmpDir := t.TempDir()
	specsDir := filepath.Join(tmpDir, ".specs", "runtime")
	os.MkdirAll(specsDir, 0755)
	specFile := filepath.Join(specsDir, "test.md")
	os.WriteFile(specFile, []byte("test"), 0644)

	root, err := FindProjectRoot(specFile)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if root != tmpDir {
		t.Errorf("expected %q, got %q", tmpDir, root)
	}
}

func TestFindProjectRoot_FromStandaloneSpec(t *testing.T) {
	tmpDir := t.TempDir()
	specsDir := filepath.Join(tmpDir, ".specs")
	os.MkdirAll(specsDir, 0755)
	specFile := filepath.Join(specsDir, "standalone.md")
	os.WriteFile(specFile, []byte("test"), 0644)

	root, err := FindProjectRoot(specFile)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if root != tmpDir {
		t.Errorf("expected %q, got %q", tmpDir, root)
	}
}

func TestFindProjectRoot_FallbackNoSpecsDir(t *testing.T) {
	tmpDir := t.TempDir()
	specFile := filepath.Join(tmpDir, "test.md")
	os.WriteFile(specFile, []byte("test"), 0644)

	root, err := FindProjectRoot(specFile)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if root != tmpDir {
		t.Errorf("expected fallback to %q, got %q", tmpDir, root)
	}
}

func TestDeduplicateRefs(t *testing.T) {
	refs := []FileLineReference{
		{Path: "a.py", Line: 1, Raw: "a.py:1"},
		{Path: "b.py", Line: 2, Raw: "b.py:2"},
		{Path: "a.py", Line: 1, Raw: "a.py:1"},
		{Path: "a.py", Line: 3, Raw: "a.py:3"},
	}
	result := DeduplicateRefs(refs)
	if len(result) != 3 {
		t.Errorf("expected 3 unique refs, got %d", len(result))
	}
}

func TestDeduplicateRefs_Empty(t *testing.T) {
	result := DeduplicateRefs(nil)
	if len(result) != 0 {
		t.Errorf("expected 0 refs, got %d", len(result))
	}
}

func TestExtractFileReferences_DeltaTable(t *testing.T) {
	content := `| D-001 | services.py | services.py:42 | new_services.py:10 | open | |
| D-002 | models.py | models.py:100 | models.py:100 | closed | |`

	refs := ExtractFileReferences(content)
	if len(refs) < 3 {
		t.Errorf("expected at least 3 file refs from delta table, got %d", len(refs))
	}
}
