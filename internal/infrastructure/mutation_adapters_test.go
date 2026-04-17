package infrastructure

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWorkspaceNormalizeVerifyFilesReturnsTypedErrors(t *testing.T) {
	repoRoot := t.TempDir()
	workspace := NewWorkspace(repoRoot)

	_, err := workspace.NormalizeVerifyFiles([]string{"../outside_test.py"})
	if err == nil {
		t.Fatal("NormalizeVerifyFiles() error = nil, want typed error")
	}

	var typedErr *VerifyFilesNormalizationError
	if !errors.As(err, &typedErr) {
		t.Fatalf("NormalizeVerifyFiles() error type = %T, want *VerifyFilesNormalizationError", err)
	}
	if typedErr.Code != VerifyFilesInvalidPath {
		t.Fatalf("typedErr.Code = %q", typedErr.Code)
	}
	if len(typedErr.Paths) != 1 || typedErr.Paths[0] != "../outside_test.py" {
		t.Fatalf("typedErr.Paths = %#v", typedErr.Paths)
	}
}

func TestSpecCreatePlannerReturnsTypedMismatchError(t *testing.T) {
	repoRoot := copyInfrastructureFixtureRepo(t, "ready-spec")
	replaceInfrastructureFileText(
		t,
		filepath.Join(repoRoot, "runtime", "src", "domain", "session_execution", "SPEC.md"),
		"spec: session-lifecycle",
		"spec: redis-state",
	)

	planner := newSpecCreatePlanner(NewWorkspace(repoRoot))
	config, err := LoadProjectConfig(filepath.Join(repoRoot, ".specs"))
	if err != nil {
		t.Fatalf("LoadProjectConfig: %v", err)
	}

	_, err = planner.Prepare(SpecCreatePlanRequest{
		Charter: "runtime",
		Slug:    "session-lifecycle",
		Doc:     "runtime/src/domain/session_execution/SPEC.md",
		Scope:   []string{"runtime/src/domain/session_execution/"},
		Config:  config,
	})
	if err == nil {
		t.Fatal("Prepare() error = nil, want typed mismatch error")
	}

	var typedErr *SpecCreatePlanError
	if !errors.As(err, &typedErr) {
		t.Fatalf("Prepare() error type = %T, want *SpecCreatePlanError", err)
	}
	if typedErr.Code != SpecCreatePrimaryDocMismatch {
		t.Fatalf("typedErr.Code = %q", typedErr.Code)
	}
	if typedErr.DocPath != "runtime/src/domain/session_execution/SPEC.md" {
		t.Fatalf("typedErr.DocPath = %q", typedErr.DocPath)
	}
}

func TestRegistryStoreApplyConfigMutationNormalizesRemovePrefixAndReloadsSnapshot(t *testing.T) {
	repoRoot := copyInfrastructureFixtureRepo(t, "ready-spec")
	if err := os.WriteFile(filepath.Join(repoRoot, ".specs", "specctl.yaml"), []byte(`gherkin_tags:
  - runtime
  - domain
source_prefixes:
  - runtime/src/
  - ui/src/
formats:
  ui-spec:
    template: ui/src/routes/SPEC-FORMAT.md
    recommended_for: ui/src/routes/**
    description: UI spec
`), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	store := NewRegistryStore(NewWorkspace(repoRoot))
	result, err := store.ApplyConfigMutation(ConfigMutationRequest{
		Kind:  ConfigMutationRemovePrefix,
		Value: "./runtime/src",
	})
	if err != nil {
		t.Fatalf("ApplyConfigMutation: %v", err)
	}
	if result.Value != "runtime/src/" {
		t.Fatalf("result.Value = %q", result.Value)
	}
	if result.Snapshot == nil || result.Snapshot.Config == nil {
		t.Fatalf("snapshot = %#v", result.Snapshot)
	}
	if len(result.Snapshot.Config.SourcePrefixes) != 1 || result.Snapshot.Config.SourcePrefixes[0] != "ui/src/" {
		t.Fatalf("source_prefixes = %#v", result.Snapshot.Config.SourcePrefixes)
	}
	if _, exists := result.Snapshot.Config.Formats["ui-spec"]; !exists {
		t.Fatalf("formats = %#v", result.Snapshot.Config.Formats)
	}

	data, err := os.ReadFile(filepath.Join(repoRoot, ".specs", "specctl.yaml"))
	if err != nil {
		t.Fatalf("ReadFile(specctl.yaml): %v", err)
	}
	content := string(data)
	if strings.Contains(content, "runtime/src/") {
		t.Fatalf("persisted config still contains removed prefix:\n%s", content)
	}
	if !strings.Contains(content, "ui-spec:") {
		t.Fatalf("persisted config dropped formats:\n%s", content)
	}
}

func TestRegistryStoreApplyConfigMutationReturnsTypedFailureSnapshot(t *testing.T) {
	repoRoot := copyInfrastructureFixtureRepo(t, "ready-spec")
	store := NewRegistryStore(NewWorkspace(repoRoot))

	_, err := store.ApplyConfigMutation(ConfigMutationRequest{
		Kind:  ConfigMutationAddTag,
		Value: "manual",
	})
	if err == nil {
		t.Fatal("ApplyConfigMutation() error = nil, want typed failure")
	}

	var typedErr *ConfigMutationError
	if !errors.As(err, &typedErr) {
		t.Fatalf("ApplyConfigMutation() error type = %T, want *ConfigMutationError", err)
	}
	if typedErr.Code != ConfigMutationSemanticTagReserved {
		t.Fatalf("typedErr.Code = %q", typedErr.Code)
	}
	if typedErr.Snapshot == nil || typedErr.Snapshot.Config == nil {
		t.Fatalf("typedErr.Snapshot = %#v", typedErr.Snapshot)
	}
}

func TestRegistryStoreApplyConfigMutationNormalizesLegacySemanticTags(t *testing.T) {
	repoRoot := copyInfrastructureFixtureRepo(t, "ready-spec")
	if err := os.WriteFile(filepath.Join(repoRoot, ".specs", "specctl.yaml"), []byte(`gherkin_tags:
  - runtime
  - manual
source_prefixes:
  - runtime/src/
formats: {}
`), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	store := NewRegistryStore(NewWorkspace(repoRoot))
	result, err := store.ApplyConfigMutation(ConfigMutationRequest{
		Kind:  ConfigMutationAddTag,
		Value: "adapter",
	})
	if err != nil {
		t.Fatalf("ApplyConfigMutation: %v", err)
	}
	if result.Snapshot == nil || result.Snapshot.Config == nil {
		t.Fatalf("snapshot = %#v", result.Snapshot)
	}
	if strings.Join(result.Snapshot.Config.GherkinTags, ",") != "adapter,runtime" {
		t.Fatalf("gherkin_tags = %#v", result.Snapshot.Config.GherkinTags)
	}

	data, err := os.ReadFile(filepath.Join(repoRoot, ".specs", "specctl.yaml"))
	if err != nil {
		t.Fatalf("ReadFile(specctl.yaml): %v", err)
	}
	if strings.Contains(string(data), "\n  - manual\n") {
		t.Fatalf("persisted config still contains legacy semantic tag:\n%s", data)
	}

	_, findings, err := LoadProjectConfigLenient(filepath.Join(repoRoot, ".specs"))
	if err != nil {
		t.Fatalf("LoadProjectConfigLenient: %v", err)
	}
	if len(findings) != 0 {
		t.Fatalf("findings = %#v, want normalized config to stop warning", findings)
	}
}

func TestRegistryStoreApplyCharterEntryMutationReturnsTypedCycleError(t *testing.T) {
	repoRoot := copyInfrastructureFixtureRepo(t, "charter-dag")
	store := NewRegistryStore(NewWorkspace(repoRoot))

	_, err := store.ApplyCharterEntryMutation(CharterEntryMutationRequest{
		Charter:   "runtime",
		Slug:      "redis-state",
		Group:     "execution",
		Order:     10,
		DependsOn: []string{"session-lifecycle"},
		Notes:     "Storage and CAS guarantees",
	})
	if err == nil {
		t.Fatal("ApplyCharterEntryMutation() error = nil, want typed cycle failure")
	}

	var typedErr *CharterEntryMutationError
	if !errors.As(err, &typedErr) {
		t.Fatalf("ApplyCharterEntryMutation() error type = %T, want *CharterEntryMutationError", err)
	}
	if typedErr.Code != CharterEntryMutationCycle {
		t.Fatalf("typedErr.Code = %q", typedErr.Code)
	}
	if typedErr.Entry == nil || typedErr.Entry.Slug != "redis-state" {
		t.Fatalf("typedErr.Entry = %#v", typedErr.Entry)
	}
	if strings.Join(typedErr.Cycle, ",") != "redis-state,session-lifecycle" {
		t.Fatalf("typedErr.Cycle = %#v", typedErr.Cycle)
	}
	if typedErr.Snapshot == nil {
		t.Fatalf("typedErr.Snapshot = %#v", typedErr.Snapshot)
	}
}

func TestRegistryStoreApplyCharterEntryMutationReturnsTypedValidationError(t *testing.T) {
	repoRoot := copyInfrastructureFixtureRepo(t, "ready-spec")
	store := NewRegistryStore(NewWorkspace(repoRoot))

	_, err := store.ApplyCharterEntryMutation(CharterEntryMutationRequest{
		Charter: "runtime",
		Slug:    "session-lifecycle",
		Group:   "recovery",
		Order:   -1,
		Notes:   "Session FSM and cleanup behavior",
	})
	if err == nil {
		t.Fatal("ApplyCharterEntryMutation() error = nil, want typed validation failure")
	}

	var typedErr *CharterEntryMutationError
	if !errors.As(err, &typedErr) {
		t.Fatalf("ApplyCharterEntryMutation() error type = %T, want *CharterEntryMutationError", err)
	}
	if typedErr.Code != CharterEntryMutationValidation {
		t.Fatalf("typedErr.Code = %q", typedErr.Code)
	}
	if len(typedErr.Findings) == 0 {
		t.Fatalf("typedErr.Findings = %#v", typedErr.Findings)
	}
	if typedErr.Snapshot == nil {
		t.Fatalf("typedErr.Snapshot = %#v", typedErr.Snapshot)
	}
}

func TestRegistryStoreApplyCharterEntryMutationUnknownDependencyStaysValidation(t *testing.T) {
	repoRoot := copyInfrastructureFixtureRepo(t, "ready-spec")
	store := NewRegistryStore(NewWorkspace(repoRoot))

	_, err := store.ApplyCharterEntryMutation(CharterEntryMutationRequest{
		Charter:   "runtime",
		Slug:      "session-lifecycle",
		Group:     "recovery",
		Order:     20,
		DependsOn: []string{"missing-spec"},
		Notes:     "Session FSM and cleanup behavior",
	})
	if err == nil {
		t.Fatal("ApplyCharterEntryMutation() error = nil, want typed validation failure")
	}

	var typedErr *CharterEntryMutationError
	if !errors.As(err, &typedErr) {
		t.Fatalf("ApplyCharterEntryMutation() error type = %T, want *CharterEntryMutationError", err)
	}
	if typedErr.Code != CharterEntryMutationValidation {
		t.Fatalf("typedErr.Code = %q", typedErr.Code)
	}
	foundDependencyInvalid := false
	for _, finding := range typedErr.Findings {
		if finding.Code == "CHARTER_DEPENDENCY_INVALID" {
			foundDependencyInvalid = true
		}
		if finding.Code == "CHARTER_CYCLE_PRESENT" {
			t.Fatalf("typedErr.Findings = %#v, want no cycle finding", typedErr.Findings)
		}
	}
	if !foundDependencyInvalid {
		t.Fatalf("typedErr.Findings = %#v", typedErr.Findings)
	}
}

func copyInfrastructureFixtureRepo(t *testing.T, fixture string) string {
	t.Helper()

	srcRoot := filepath.Join("..", "..", "testdata", "v2", fixture)
	dstRoot := t.TempDir()

	if err := filepath.Walk(srcRoot, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, err := filepath.Rel(srcRoot, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}

		dstPath := filepath.Join(dstRoot, rel)
		if info.IsDir() {
			return os.MkdirAll(dstPath, info.Mode())
		}

		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(dstPath, data, info.Mode())
	}); err != nil {
		t.Fatalf("copy fixture repo: %v", err)
	}

	return dstRoot
}

func replaceInfrastructureFileText(t *testing.T, path, oldValue, newValue string) {
	t.Helper()

	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(%s): %v", path, err)
	}
	updated := strings.Replace(string(content), oldValue, newValue, 1)
	if updated == string(content) {
		t.Fatalf("did not find %q in %s", oldValue, path)
	}
	if err := os.WriteFile(path, []byte(updated), 0o644); err != nil {
		t.Fatalf("WriteFile(%s): %v", path, err)
	}
}
