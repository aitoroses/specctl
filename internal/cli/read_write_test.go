package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
)

func TestReadCommandsReturnCanonicalJSON(t *testing.T) {
	t.Run("charter create returns create-spec template with slug-only argv placeholders", func(t *testing.T) {
		repoRoot := t.TempDir()
		if err := os.MkdirAll(filepath.Join(repoRoot, ".specs"), 0755); err != nil {
			t.Fatalf("mkdir specs dir: %v", err)
		}

		withWorkingDir(t, repoRoot, func() {
			stdout, stderr, exitCode := executeCLIWithInput("title: Runtime System\ndescription: Specs for runtime\ngroups:\n  - key: execution\n    title: Execution Engine\n    order: 10\n", "charter", "create", "runtime")
			if exitCode != 0 {
				t.Fatalf("exit code = %d, stderr=%s stdout=%s", exitCode, stderr, stdout)
			}

			var envelope struct {
				Next testNextMaps `json:"next"`
			}
			mustUnmarshalJSON(t, stdout, &envelope)
			next := requireNextAction(t, mapsToAny(envelope.Next), 0, "create_spec")
			template := requireTemplate(t, next)
			if argv := stringSliceFromAny(t, template["argv"]); strings.Join(argv, " ") != "specctl spec create runtime:<slug> --title <title> --doc <design_doc> --scope <scope_dir_1>/ --group execution --order <order> --charter-notes <charter_notes>" {
				t.Fatalf("template.argv = %v", argv)
			}
			if required := requiredFieldNames(t, template["required_fields"]); strings.Join(required, ",") != "slug,title,design_doc,scope_dir_1,order,charter_notes" {
				t.Fatalf("required_fields = %v", required)
			}
			if _, exists := template["stdin_format"]; exists {
				t.Fatalf("template.stdin_format = %#v, want omitted for run_command argv template", template["stdin_format"])
			}
			if _, exists := template["stdin_template"]; exists {
				t.Fatalf("template.stdin_template = %#v, want omitted for run_command argv template", template["stdin_template"])
			}
			if stderr != "" {
				t.Fatalf("expected empty stderr, got %q", stderr)
			}
		})
	})

	t.Run("charter create without groups returns executable first-group create-spec template", func(t *testing.T) {
		repoRoot := t.TempDir()
		if err := os.MkdirAll(filepath.Join(repoRoot, ".specs"), 0o755); err != nil {
			t.Fatalf("mkdir specs dir: %v", err)
		}

		withWorkingDir(t, repoRoot, func() {
			stdout, stderr, exitCode := executeCLIWithInput("title: Runtime System\ndescription: Specs for runtime\n", "charter", "create", "runtime")
			if exitCode != 0 {
				t.Fatalf("exit code = %d, stderr=%s stdout=%s", exitCode, stderr, stdout)
			}

			var envelope struct {
				Next testNextMaps `json:"next"`
			}
			mustUnmarshalJSON(t, stdout, &envelope)
			next := requireNextAction(t, mapsToAny(envelope.Next), 0, "create_spec")
			template := requireTemplate(t, next)
			if argv := stringSliceFromAny(t, template["argv"]); strings.Join(argv, " ") != "specctl spec create runtime:<slug> --title <title> --doc <design_doc> --scope <scope_dir_1>/ --group <group> --group-title <group_title> --group-order <group_order> --order <order> --charter-notes <charter_notes>" {
				t.Fatalf("template.argv = %v", argv)
			}
			if required := requiredFieldNames(t, template["required_fields"]); strings.Join(required, ",") != "slug,title,design_doc,scope_dir_1,group,group_title,group_order,order,charter_notes" {
				t.Fatalf("required_fields = %v", required)
			}
			if stderr != "" {
				t.Fatalf("expected empty stderr, got %q", stderr)
			}
		})
	})

	t.Run("context registry", func(t *testing.T) {
		repoRoot := copyFixtureRepo(t, "charter-dag")

		withWorkingDir(t, repoRoot, func() {
			stdout, stderr, exitCode := executeCLI("context")
			if exitCode != 0 {
				t.Fatalf("exit code = %d, stderr=%s", exitCode, stderr)
			}
			if stderr != "" {
				t.Fatalf("expected empty stderr, got %q", stderr)
			}

			var envelope struct {
				State struct {
					Specs    []map[string]any `json:"specs"`
					Charters []map[string]any `json:"charters"`
					Audit    map[string]any   `json:"audit"`
				} `json:"state"`
				Next testNext `json:"next"`
			}
			mustUnmarshalJSON(t, stdout, &envelope)

			if len(envelope.State.Specs) != 3 {
				t.Fatalf("spec count = %d, want 3", len(envelope.State.Specs))
			}
			if got := envelope.State.Specs[0]["slug"]; got != "redis-state" {
				t.Fatalf("first registry spec = %v, want redis-state", got)
			}
			if len(envelope.State.Charters) != 1 || envelope.State.Charters[0]["name"] != "runtime" {
				t.Fatalf("unexpected charters payload %#v", envelope.State.Charters)
			}
			if envelope.State.Audit["valid"] != true {
				t.Fatalf("expected audit.valid=true, got %#v", envelope.State.Audit)
			}
		})
	})

	t.Run("context registry keeps config validation separate from audit", func(t *testing.T) {
		repoRoot := copyFixtureRepoWithRegistry(t, "ready-spec")
		replaceInFile(t, filepath.Join(repoRoot, ".specs", "runtime", "CHARTER.yaml"), "slug: session-lifecycle", "slug: missing-spec")

		withWorkingDir(t, repoRoot, func() {
			stdout, stderr, exitCode := executeCLI("context")
			if exitCode != 0 {
				t.Fatalf("exit code = %d, stderr=%s stdout=%s", exitCode, stderr, stdout)
			}
			if stderr != "" {
				t.Fatalf("expected empty stderr, got %q", stderr)
			}

			var envelope struct {
				State struct {
					Config struct {
						Validation struct {
							Valid    bool `json:"valid"`
							Findings []struct {
								Code string `json:"code"`
							} `json:"findings"`
						} `json:"validation"`
					} `json:"config"`
					Audit struct {
						Valid    bool `json:"valid"`
						Findings []struct {
							Code string `json:"code"`
						} `json:"findings"`
					} `json:"audit"`
				} `json:"state"`
			}
			mustUnmarshalJSON(t, stdout, &envelope)
			if !envelope.State.Config.Validation.Valid {
				t.Fatalf("config.validation = %#v", envelope.State.Config.Validation)
			}
			if len(envelope.State.Config.Validation.Findings) != 0 {
				t.Fatalf("config.validation.findings = %#v, want config-only validation", envelope.State.Config.Validation.Findings)
			}
			if envelope.State.Audit.Valid {
				t.Fatalf("audit = %#v", envelope.State.Audit)
			}
			found := false
			for _, finding := range envelope.State.Audit.Findings {
				if finding.Code == "CHARTER_SPEC_MISSING" {
					found = true
				}
			}
			if !found {
				t.Fatalf("audit.findings = %#v", envelope.State.Audit.Findings)
			}

			stdout, stderr, exitCode = executeCLI("config")
			if exitCode != 0 {
				t.Fatalf("config exit code = %d, stderr=%s stdout=%s", exitCode, stderr, stdout)
			}
			var configEnvelope struct {
				State struct {
					Validation struct {
						Valid    bool `json:"valid"`
						Findings []struct {
							Code string `json:"code"`
						} `json:"findings"`
					} `json:"validation"`
				} `json:"state"`
			}
			mustUnmarshalJSON(t, stdout, &configEnvelope)
			if configEnvelope.State.Validation.Valid {
				t.Fatalf("config validation = %#v", configEnvelope.State.Validation)
			}
			found = false
			for _, finding := range configEnvelope.State.Validation.Findings {
				if finding.Code == "CHARTER_SPEC_MISSING" {
					found = true
				}
			}
			if !found {
				t.Fatalf("config validation findings = %#v", configEnvelope.State.Validation.Findings)
			}
		})
	})

	t.Run("context charter", func(t *testing.T) {
		repoRoot := copyFixtureRepo(t, "charter-dag")

		withWorkingDir(t, repoRoot, func() {
			stdout, stderr, exitCode := executeCLI("context", "runtime")
			if exitCode != 0 {
				t.Fatalf("exit code = %d, stderr=%s", exitCode, stderr)
			}

			var envelope struct {
				State struct {
					Name         string `json:"name"`
					TrackingFile string `json:"tracking_file"`
					OrderedSpecs []struct {
						Slug string `json:"slug"`
					} `json:"ordered_specs"`
				} `json:"state"`
			}
			mustUnmarshalJSON(t, stdout, &envelope)

			if envelope.State.Name != "runtime" {
				t.Fatalf("charter name = %q", envelope.State.Name)
			}
			if envelope.State.TrackingFile != ".specs/runtime/CHARTER.yaml" {
				t.Fatalf("tracking file = %q", envelope.State.TrackingFile)
			}
			want := []string{"redis-state", "recovery-projection", "session-lifecycle"}
			if got := orderedSlugs(envelope.State.OrderedSpecs); strings.Join(got, ",") != strings.Join(want, ",") {
				t.Fatalf("ordered specs = %v, want %v", got, want)
			}
			if stderr != "" {
				t.Fatalf("expected empty stderr, got %q", stderr)
			}
		})
	})

	t.Run("context charter keeps per-spec validation for missing tracking files", func(t *testing.T) {
		repoRoot := copyFixtureRepo(t, "charter-missing-tracking")

		withWorkingDir(t, repoRoot, func() {
			stdout, stderr, exitCode := executeCLI("context", "runtime")
			if exitCode != 0 {
				t.Fatalf("exit code = %d, stderr=%s stdout=%s", exitCode, stderr, stdout)
			}

			var envelope struct {
				State struct {
					OrderedSpecs []struct {
						Slug       string `json:"slug"`
						Validation struct {
							Valid    bool `json:"valid"`
							Findings []struct {
								Code string `json:"code"`
							} `json:"findings"`
						} `json:"validation"`
					} `json:"ordered_specs"`
				} `json:"state"`
			}
			mustUnmarshalJSON(t, stdout, &envelope)

			var spec *struct {
				Slug       string `json:"slug"`
				Validation struct {
					Valid    bool `json:"valid"`
					Findings []struct {
						Code string `json:"code"`
					} `json:"findings"`
				} `json:"validation"`
			}
			for i := range envelope.State.OrderedSpecs {
				if envelope.State.OrderedSpecs[i].Slug == "session-lifecycle" {
					spec = &envelope.State.OrderedSpecs[i]
					break
				}
			}
			if spec == nil {
				t.Fatalf("ordered_specs = %#v", envelope.State.OrderedSpecs)
			}
			if spec.Validation.Valid || len(spec.Validation.Findings) == 0 {
				t.Fatalf("ordered_spec.validation = %#v", spec.Validation)
			}
			if spec.Validation.Findings[0].Code != "CHARTER_SPEC_MISSING" {
				t.Fatalf("ordered_spec.validation.findings = %#v", spec.Validation.Findings)
			}
			if stderr != "" {
				t.Fatalf("expected empty stderr, got %q", stderr)
			}
		})
	})

	t.Run("context spec", func(t *testing.T) {
		repoRoot := seededSpecRepo(t)

		withWorkingDir(t, repoRoot, func() {
			stdout, stderr, exitCode := executeCLI("context", "runtime:session-lifecycle")
			if exitCode != 0 {
				t.Fatalf("exit code = %d, stderr=%s", exitCode, stderr)
			}

			var envelope struct {
				State struct {
					Slug              string `json:"slug"`
					Status            string `json:"status"`
					TrackingFile      string `json:"tracking_file"`
					Format            any    `json:"format"`
					CharterMembership struct {
						Group string `json:"group"`
					} `json:"charter_membership"`
					OpenDeltas []struct {
						ID string `json:"id"`
					} `json:"open_deltas"`
					ActionableUnverifiedRequirements []struct {
						ID string `json:"id"`
					} `json:"actionable_unverified_requirements"`
					InactiveUnverifiedRequirements []struct {
						ID string `json:"id"`
					} `json:"inactive_unverified_requirements"`
				} `json:"state"`
			}
			mustUnmarshalJSON(t, stdout, &envelope)

			if envelope.State.Slug != "session-lifecycle" || envelope.State.Status != "active" {
				t.Fatalf("unexpected state %#v", envelope.State)
			}
			if envelope.State.TrackingFile != ".specs/runtime/session-lifecycle.yaml" {
				t.Fatalf("tracking file = %q", envelope.State.TrackingFile)
			}
			if envelope.State.Format != nil {
				t.Fatalf("expected null format, got %#v", envelope.State.Format)
			}
			if envelope.State.CharterMembership.Group != "recovery" {
				t.Fatalf("membership = %#v", envelope.State.CharterMembership)
			}
			if len(envelope.State.OpenDeltas) != 1 || envelope.State.OpenDeltas[0].ID != "D-001" {
				t.Fatalf("unexpected open deltas %#v", envelope.State.OpenDeltas)
			}
			if len(envelope.State.ActionableUnverifiedRequirements) != 1 || envelope.State.ActionableUnverifiedRequirements[0].ID != "REQ-001" {
				t.Fatalf("unexpected actionable requirements %#v", envelope.State.ActionableUnverifiedRequirements)
			}
			if len(envelope.State.InactiveUnverifiedRequirements) != 0 {
				t.Fatalf("unexpected inactive requirements %#v", envelope.State.InactiveUnverifiedRequirements)
			}
			if stderr != "" {
				t.Fatalf("expected empty stderr, got %q", stderr)
			}
		})
	})

	t.Run("context spec returns validation findings instead of failing on malformed stored state", func(t *testing.T) {
		repoRoot := copyFixtureRepo(t, "malformed-gapful-spec")

		withWorkingDir(t, repoRoot, func() {
			stdout, stderr, exitCode := executeCLI("context", "runtime:session-lifecycle")
			if exitCode != 0 {
				t.Fatalf("exit code = %d, stderr=%s stdout=%s", exitCode, stderr, stdout)
			}

			var envelope struct {
				State struct {
					Slug       string `json:"slug"`
					Validation struct {
						Valid    bool `json:"valid"`
						Findings []struct {
							Code string `json:"code"`
						} `json:"findings"`
					} `json:"validation"`
				} `json:"state"`
			}
			mustUnmarshalJSON(t, stdout, &envelope)
			if envelope.State.Slug != "session-lifecycle" {
				t.Fatalf("slug = %q", envelope.State.Slug)
			}
			if envelope.State.Validation.Valid || len(envelope.State.Validation.Findings) == 0 {
				t.Fatalf("validation = %#v", envelope.State.Validation)
			}
		})
	})

	t.Run("context charter returns validation findings instead of failing on malformed stored state", func(t *testing.T) {
		repoRoot := copyFixtureRepo(t, "charter-missing-notes")

		withWorkingDir(t, repoRoot, func() {
			stdout, stderr, exitCode := executeCLI("context", "runtime")
			if exitCode != 0 {
				t.Fatalf("exit code = %d, stderr=%s stdout=%s", exitCode, stderr, stdout)
			}

			var envelope struct {
				State struct {
					Name       string `json:"name"`
					Validation struct {
						Valid    bool `json:"valid"`
						Findings []struct {
							Code string `json:"code"`
						} `json:"findings"`
					} `json:"validation"`
				} `json:"state"`
			}
			mustUnmarshalJSON(t, stdout, &envelope)
			if envelope.State.Name != "runtime" {
				t.Fatalf("name = %q", envelope.State.Name)
			}
			if envelope.State.Validation.Valid || len(envelope.State.Validation.Findings) == 0 {
				t.Fatalf("validation = %#v", envelope.State.Validation)
			}
		})
	})

	t.Run("context charter maps missing charter metadata to charter validation codes", func(t *testing.T) {
		repoRoot := copyFixtureRepoWithRegistry(t, "ready-spec")
		replaceInFile(t, filepath.Join(repoRoot, ".specs", "runtime", "CHARTER.yaml"), "title: Runtime System\n", "")
		replaceInFile(t, filepath.Join(repoRoot, ".specs", "runtime", "CHARTER.yaml"), "description: Specs for runtime control-plane and data-plane behavior\n", "")

		withWorkingDir(t, repoRoot, func() {
			stdout, stderr, exitCode := executeCLI("context", "runtime")
			if exitCode != 0 {
				t.Fatalf("exit code = %d, stderr=%s stdout=%s", exitCode, stderr, stdout)
			}

			var envelope struct {
				State struct {
					Validation struct {
						Valid    bool `json:"valid"`
						Findings []struct {
							Code string `json:"code"`
						} `json:"findings"`
					} `json:"validation"`
				} `json:"state"`
			}
			mustUnmarshalJSON(t, stdout, &envelope)
			if envelope.State.Validation.Valid || len(envelope.State.Validation.Findings) == 0 {
				t.Fatalf("validation = %#v", envelope.State.Validation)
			}

			foundCharterCode := false
			for _, finding := range envelope.State.Validation.Findings {
				if finding.Code == "CHARTER_NAME_INVALID" {
					foundCharterCode = true
				}
				if finding.Code == "SPEC_STATUS_INVALID" {
					t.Fatalf("unexpected fallback finding %#v", finding)
				}
			}
			if !foundCharterCode {
				t.Fatalf("validation.findings = %#v", envelope.State.Validation.Findings)
			}
			if stderr != "" {
				t.Fatalf("expected empty stderr, got %q", stderr)
			}
		})
	})

	t.Run("context file", func(t *testing.T) {
		repoRoot := seededSpecRepo(t)
		serviceFile := filepath.Join(repoRoot, "runtime", "src", "domain", "session_execution", "services.py")
		if err := os.WriteFile(serviceFile, []byte("def handle():\n    pass\n"), 0644); err != nil {
			t.Fatalf("write service file: %v", err)
		}

		withWorkingDir(t, repoRoot, func() {
			stdout, stderr, exitCode := executeCLI("context", "--file", "runtime/src/domain/session_execution/services.py")
			if exitCode != 0 {
				t.Fatalf("exit code = %d, stderr=%s", exitCode, stderr)
			}

			var envelope struct {
				State struct {
					File          string `json:"file"`
					Resolution    string `json:"resolution"`
					GoverningSpec struct {
						Slug string `json:"slug"`
					} `json:"governing_spec"`
					Matches []struct {
						ScopePrefix string `json:"scope_prefix"`
					} `json:"matches"`
				} `json:"state"`
			}
			mustUnmarshalJSON(t, stdout, &envelope)

			if envelope.State.File != "runtime/src/domain/session_execution/services.py" {
				t.Fatalf("file = %q", envelope.State.File)
			}
			if envelope.State.Resolution != "matched" {
				t.Fatalf("resolution = %q", envelope.State.Resolution)
			}
			if envelope.State.GoverningSpec.Slug != "session-lifecycle" {
				t.Fatalf("governing spec = %#v", envelope.State.GoverningSpec)
			}
			if len(envelope.State.Matches) == 0 || envelope.State.Matches[0].ScopePrefix != "runtime/src/domain/session_execution/" {
				t.Fatalf("matches = %#v", envelope.State.Matches)
			}
			if stderr != "" {
				t.Fatalf("expected empty stderr, got %q", stderr)
			}
		})
	})

	t.Run("context file exact design doc uses canonical match source", func(t *testing.T) {
		repoRoot := seededSpecRepo(t)

		withWorkingDir(t, repoRoot, func() {
			stdout, stderr, exitCode := executeCLI("context", "--file", "runtime/src/domain/session_execution/SPEC.md")
			if exitCode != 0 {
				t.Fatalf("exit code = %d, stderr=%s", exitCode, stderr)
			}

			var envelope struct {
				State struct {
					Resolution  string `json:"resolution"`
					MatchSource string `json:"match_source"`
					Matches     []struct {
						MatchSource string `json:"match_source"`
					} `json:"matches"`
				} `json:"state"`
			}
			mustUnmarshalJSON(t, stdout, &envelope)
			if envelope.State.Resolution != "matched" || envelope.State.MatchSource != "design_doc" {
				t.Fatalf("state = %#v", envelope.State)
			}
			if len(envelope.State.Matches) != 1 || envelope.State.Matches[0].MatchSource != "design_doc" {
				t.Fatalf("matches = %#v", envelope.State.Matches)
			}
		})
	})

	t.Run("context file exact design doc keeps lower-ranked scope matches", func(t *testing.T) {
		repoRoot := t.TempDir()
		if err := os.MkdirAll(filepath.Join(repoRoot, ".specs", "runtime"), 0755); err != nil {
			t.Fatalf("mkdir specs: %v", err)
		}
		if err := os.MkdirAll(filepath.Join(repoRoot, "runtime", "src", "domain", "session_execution"), 0755); err != nil {
			t.Fatalf("mkdir session execution: %v", err)
		}
		if err := os.WriteFile(filepath.Join(repoRoot, "runtime", "src", "domain", "session_execution", "SPEC.md"), []byte("---\nspec: session-lifecycle\ncharter: runtime\n---\n"), 0644); err != nil {
			t.Fatalf("write session design doc: %v", err)
		}
		if err := os.WriteFile(filepath.Join(repoRoot, "runtime", "src", "domain", "session_execution", "service.py"), []byte("pass\n"), 0644); err != nil {
			t.Fatalf("write service file: %v", err)
		}
		if err := os.WriteFile(filepath.Join(repoRoot, ".specs", "runtime", "CHARTER.yaml"), []byte(`name: runtime
title: Runtime System
description: Specs for runtime control-plane and data-plane behavior
groups:
  - key: recovery
    title: Recovery
    order: 10
specs:
  - slug: session-lifecycle
    group: recovery
    order: 20
    depends_on: []
    notes: Session lifecycle
  - slug: runtime-api-contract
    group: recovery
    order: 30
    depends_on: []
    notes: Runtime API contract
`), 0644); err != nil {
			t.Fatalf("write charter: %v", err)
		}
		if err := os.WriteFile(filepath.Join(repoRoot, ".specs", "runtime", "session-lifecycle.yaml"), []byte(`slug: session-lifecycle
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
changelog: []
`), 0644); err != nil {
			t.Fatalf("write tracking session-lifecycle: %v", err)
		}
		if err := os.WriteFile(filepath.Join(repoRoot, ".specs", "runtime", "runtime-api-contract.yaml"), []byte(`slug: runtime-api-contract
charter: runtime
title: Runtime API Contract
status: draft
rev: 1
created: 2026-03-05
updated: 2026-03-30
last_verified_at: 2026-03-28
checkpoint: a1b2c3f
tags:
  - runtime
documents:
  primary: runtime/src/domain/api/SPEC.md
scope:
  - runtime/src/domain/
deltas: []
requirements: []
changelog: []
`), 0644); err != nil {
			t.Fatalf("write tracking runtime-api-contract: %v", err)
		}

		withWorkingDir(t, repoRoot, func() {
			stdout, stderr, exitCode := executeCLI("context", "--file", "runtime/src/domain/session_execution/SPEC.md")
			if exitCode != 0 {
				t.Fatalf("exit code = %d, stderr=%s", exitCode, stderr)
			}

			var envelope struct {
				State struct {
					Resolution    string `json:"resolution"`
					MatchSource   string `json:"match_source"`
					GoverningSpec struct {
						Slug string `json:"slug"`
					} `json:"governing_spec"`
					Matches []struct {
						Slug        string `json:"slug"`
						MatchSource string `json:"match_source"`
					} `json:"matches"`
				} `json:"state"`
			}
			mustUnmarshalJSON(t, stdout, &envelope)
			if envelope.State.Resolution != "matched" || envelope.State.MatchSource != "design_doc" || envelope.State.GoverningSpec.Slug != "session-lifecycle" {
				t.Fatalf("state = %#v", envelope.State)
			}
			if len(envelope.State.Matches) != 2 {
				t.Fatalf("matches = %#v", envelope.State.Matches)
			}
			if envelope.State.Matches[0].Slug != "session-lifecycle" || envelope.State.Matches[0].MatchSource != "design_doc" {
				t.Fatalf("matches[0] = %#v", envelope.State.Matches[0])
			}
			if envelope.State.Matches[1].Slug != "runtime-api-contract" || envelope.State.Matches[1].MatchSource != "scope" {
				t.Fatalf("matches[1] = %#v", envelope.State.Matches[1])
			}
		})
	})

	t.Run("context file no-match returns canonical resolution and create-spec template", func(t *testing.T) {
		repoRoot := copyFixtureRepoWithRegistry(t, "ready-spec")
		if err := os.MkdirAll(filepath.Join(repoRoot, "runtime", "src", "domain", "new_protocol"), 0755); err != nil {
			t.Fatalf("mkdir new protocol: %v", err)
		}
		if err := os.WriteFile(filepath.Join(repoRoot, "runtime", "src", "domain", "new_protocol", "handler.go"), []byte("package main\n"), 0644); err != nil {
			t.Fatalf("write handler: %v", err)
		}

		withWorkingDir(t, repoRoot, func() {
			stdout, stderr, exitCode := executeCLI("context", "--file", "runtime/src/domain/new_protocol/handler.go")
			if exitCode != 0 {
				t.Fatalf("exit code = %d, stderr=%s", exitCode, stderr)
			}

			var envelope struct {
				State map[string]any `json:"state"`
				Next  testNextMaps   `json:"next"`
			}
			mustUnmarshalJSON(t, stdout, &envelope)
			if envelope.State["resolution"] != "unmatched" {
				t.Fatalf("state = %#v", envelope.State)
			}
			if _, exists := envelope.State["match_source"]; !exists || envelope.State["match_source"] != nil {
				t.Fatalf("match_source = %#v", envelope.State["match_source"])
			}
			if _, exists := envelope.State["governing_spec"]; !exists || envelope.State["governing_spec"] != nil {
				t.Fatalf("governing_spec = %#v", envelope.State["governing_spec"])
			}
			next := requireNextAction(t, mapsToAny(envelope.Next), 0, "create_spec")
			template := requireTemplate(t, next)
			if argv := stringSliceFromAny(t, template["argv"]); strings.Join(argv, " ") != "specctl spec create runtime:new-protocol --title <title> --doc <design_doc> --scope runtime/src/domain/new_protocol/ --group <group> --order <order> --charter-notes <charter_notes>" {
				t.Fatalf("template.argv = %v", argv)
			}
			if descriptions := requiredFieldDescriptions(t, template["required_fields"]); strings.Join(descriptions, "|") != "Human-readable spec title|Repo-relative markdown path|Existing charter group key|Integer order inside the group|Short planning note" {
				t.Fatalf("required_field_descriptions = %v", descriptions)
			}
		})
	})

	t.Run("context file rejects paths that escape the repo root", func(t *testing.T) {
		repoRoot := copyFixtureRepoWithRegistry(t, "ready-spec")

		withWorkingDir(t, repoRoot, func() {
			stdout, stderr, exitCode := executeCLI("context", "--file", "../outside.go")
			if exitCode == 0 {
				t.Fatalf("expected failure, stderr=%s stdout=%s", stderr, stdout)
			}

			envelope := requireFailureEnvelope(t, stdout, stderr)
			if envelope.Error.Code != "INVALID_INPUT" {
				t.Fatalf("error = %#v", envelope.Error)
			}
			if envelope.State["file"] != "../outside.go" {
				t.Fatalf("state = %#v", envelope.State)
			}
			if envelope.State["resolution"] != "unmatched" {
				t.Fatalf("resolution = %#v", envelope.State["resolution"])
			}
			focus := envelope.State["focus"].(map[string]any)
			if got := stringSliceFromAny(t, focus["invalid_paths"]); strings.Join(got, ",") != "../outside.go" {
				t.Fatalf("invalid_paths = %#v", focus["invalid_paths"])
			}
			if len(envelope.Next) != 0 {
				t.Fatalf("next = %#v", envelope.Next)
			}
		})
	})

	t.Run("context file ambiguous sorts matches deterministically", func(t *testing.T) {
		repoRoot := t.TempDir()
		if err := os.MkdirAll(filepath.Join(repoRoot, ".specs", "runtime"), 0755); err != nil {
			t.Fatalf("mkdir specs: %v", err)
		}
		if err := os.MkdirAll(filepath.Join(repoRoot, ".specs", "platform"), 0755); err != nil {
			t.Fatalf("mkdir platform specs: %v", err)
		}
		if err := os.MkdirAll(filepath.Join(repoRoot, "runtime", "src", "domain", "shared"), 0755); err != nil {
			t.Fatalf("mkdir shared: %v", err)
		}
		if err := os.WriteFile(filepath.Join(repoRoot, "runtime", "src", "domain", "shared", "transport.py"), []byte("pass\n"), 0644); err != nil {
			t.Fatalf("write transport: %v", err)
		}
		if err := os.WriteFile(filepath.Join(repoRoot, "runtime", "src", "domain", "shared", "SPEC.md"), []byte("---\nspec: session-lifecycle\ncharter: runtime\n---\n"), 0644); err != nil {
			t.Fatalf("write doc: %v", err)
		}
		if err := os.WriteFile(filepath.Join(repoRoot, ".specs", "runtime", "CHARTER.yaml"), []byte(`name: runtime
title: Runtime System
description: Specs for runtime control-plane and data-plane behavior
groups:
  - key: recovery
    title: Recovery
    order: 10
specs:
  - slug: session-lifecycle
    group: recovery
    order: 20
    depends_on: []
    notes: Session lifecycle
`), 0644); err != nil {
			t.Fatalf("write runtime charter: %v", err)
		}
		if err := os.WriteFile(filepath.Join(repoRoot, ".specs", "platform", "CHARTER.yaml"), []byte(`name: platform
title: Platform System
description: Specs for platform behavior
groups:
  - key: recovery
    title: Recovery
    order: 10
specs:
  - slug: runtime-api-contract
    group: recovery
    order: 20
    depends_on: []
    notes: Runtime API contract
`), 0644); err != nil {
			t.Fatalf("write platform charter: %v", err)
		}
		if err := os.WriteFile(filepath.Join(repoRoot, ".specs", "runtime", "session-lifecycle.yaml"), []byte(`slug: session-lifecycle
charter: runtime
title: session-lifecycle
status: draft
rev: 1
created: 2026-03-05
updated: 2026-03-30
last_verified_at: 2026-03-28
checkpoint: a1b2c3f
tags:
  - runtime
documents:
  primary: runtime/src/domain/shared/SPEC.md
scope:
  - runtime/src/domain/shared/
deltas: []
requirements: []
changelog: []
`), 0644); err != nil {
			t.Fatalf("write runtime tracking: %v", err)
		}
		if err := os.WriteFile(filepath.Join(repoRoot, ".specs", "platform", "runtime-api-contract.yaml"), []byte(`slug: runtime-api-contract
charter: platform
title: runtime-api-contract
status: draft
rev: 1
created: 2026-03-05
updated: 2026-03-30
last_verified_at: 2026-03-28
checkpoint: a1b2c3f
tags:
  - runtime
documents:
  primary: runtime/src/domain/shared/PLATFORM-SPEC.md
scope:
  - runtime/src/domain/shared/
deltas: []
requirements: []
changelog: []
`), 0644); err != nil {
			t.Fatalf("write platform tracking: %v", err)
		}

		withWorkingDir(t, repoRoot, func() {
			stdout, stderr, exitCode := executeCLI("context", "--file", "runtime/src/domain/shared/transport.py")
			if exitCode != 0 {
				t.Fatalf("exit code = %d, stderr=%s", exitCode, stderr)
			}

			var envelope struct {
				State struct {
					Resolution string `json:"resolution"`
					Matches    []struct {
						Slug string `json:"slug"`
					} `json:"matches"`
				} `json:"state"`
			}
			mustUnmarshalJSON(t, stdout, &envelope)
			if envelope.State.Resolution != "ambiguous" {
				t.Fatalf("resolution = %q", envelope.State.Resolution)
			}
			if got := orderedSlugs(envelope.State.Matches); strings.Join(got, ",") != "runtime-api-contract,session-lifecycle" {
				t.Fatalf("matches = %v", got)
			}
		})
	})

	t.Run("context file prefers strict longest scope prefix over broader same-charter match", func(t *testing.T) {
		repoRoot := t.TempDir()
		if err := os.MkdirAll(filepath.Join(repoRoot, ".specs", "runtime"), 0o755); err != nil {
			t.Fatalf("mkdir specs: %v", err)
		}
		if err := os.MkdirAll(filepath.Join(repoRoot, "runtime", "src", "domain"), 0o755); err != nil {
			t.Fatalf("mkdir domain: %v", err)
		}
		if err := os.WriteFile(filepath.Join(repoRoot, "runtime", "src", "domain", "service.go"), []byte("package domain\n"), 0o644); err != nil {
			t.Fatalf("write service: %v", err)
		}
		if err := os.WriteFile(filepath.Join(repoRoot, ".specs", "runtime", "CHARTER.yaml"), []byte(`name: runtime
title: Runtime System
description: Specs for runtime control-plane and data-plane behavior
groups:
  - key: platform
    title: Platform
    order: 10
specs:
  - slug: broad-platform
    group: platform
    order: 10
    depends_on: []
    notes: Broad runtime platform
  - slug: domain-platform
    group: platform
    order: 20
    depends_on: []
    notes: Domain runtime platform
`), 0o644); err != nil {
			t.Fatalf("write charter: %v", err)
		}
		if err := os.WriteFile(filepath.Join(repoRoot, ".specs", "runtime", "broad-platform.yaml"), []byte(`slug: broad-platform
charter: runtime
title: Broad Platform
status: draft
rev: 1
created: 2026-03-05
updated: 2026-03-30
last_verified_at: 2026-03-28
checkpoint: a1b2c3f
tags:
  - runtime
documents:
  primary: runtime/src/SPEC.md
scope:
  - runtime/src/
deltas: []
requirements: []
changelog: []
`), 0o644); err != nil {
			t.Fatalf("write broad tracking: %v", err)
		}
		if err := os.WriteFile(filepath.Join(repoRoot, ".specs", "runtime", "domain-platform.yaml"), []byte(`slug: domain-platform
charter: runtime
title: Domain Platform
status: draft
rev: 1
created: 2026-03-05
updated: 2026-03-30
last_verified_at: 2026-03-28
checkpoint: a1b2c3f
tags:
  - runtime
documents:
  primary: runtime/src/domain/SPEC.md
scope:
  - runtime/src/domain/
deltas: []
requirements: []
changelog: []
`), 0o644); err != nil {
			t.Fatalf("write narrow tracking: %v", err)
		}

		withWorkingDir(t, repoRoot, func() {
			stdout, stderr, exitCode := executeCLI("context", "--file", "runtime/src/domain/service.go")
			if exitCode != 0 {
				t.Fatalf("exit code = %d, stderr=%s", exitCode, stderr)
			}
			if stderr != "" {
				t.Fatalf("expected empty stderr, got %q", stderr)
			}

			var envelope struct {
				State struct {
					Resolution    string `json:"resolution"`
					GoverningSpec struct {
						Slug string `json:"slug"`
					} `json:"governing_spec"`
					Matches []struct {
						Slug        string `json:"slug"`
						ScopePrefix string `json:"scope_prefix"`
					} `json:"matches"`
				} `json:"state"`
			}
			mustUnmarshalJSON(t, stdout, &envelope)
			if envelope.State.Resolution != "matched" || envelope.State.GoverningSpec.Slug != "domain-platform" {
				t.Fatalf("state = %#v", envelope.State)
			}
			got := []string{envelope.State.Matches[0].Slug, envelope.State.Matches[1].Slug}
			if strings.Join(got, ",") != "domain-platform,broad-platform" {
				t.Fatalf("matches = %#v", envelope.State.Matches)
			}
			if envelope.State.Matches[0].ScopePrefix != "runtime/src/domain/" || envelope.State.Matches[1].ScopePrefix != "runtime/src/" {
				t.Fatalf("scope prefixes = %#v", envelope.State.Matches)
			}
		})
	})

	t.Run("context file keeps same-charter ties ambiguous", func(t *testing.T) {
		repoRoot := t.TempDir()
		if err := os.MkdirAll(filepath.Join(repoRoot, ".specs", "runtime"), 0o755); err != nil {
			t.Fatalf("mkdir specs: %v", err)
		}
		if err := os.MkdirAll(filepath.Join(repoRoot, "runtime", "src", "domain", "shared"), 0o755); err != nil {
			t.Fatalf("mkdir shared: %v", err)
		}
		if err := os.WriteFile(filepath.Join(repoRoot, "runtime", "src", "domain", "shared", "transport.py"), []byte("pass\n"), 0o644); err != nil {
			t.Fatalf("write transport: %v", err)
		}
		if err := os.WriteFile(filepath.Join(repoRoot, ".specs", "runtime", "CHARTER.yaml"), []byte(`name: runtime
title: Runtime System
description: Specs for runtime control-plane and data-plane behavior
groups:
  - key: recovery
    title: Recovery
    order: 10
specs:
  - slug: first-owner
    group: recovery
    order: 20
    depends_on: []
    notes: First owner
  - slug: second-owner
    group: recovery
    order: 30
    depends_on: []
    notes: Second owner
`), 0o644); err != nil {
			t.Fatalf("write charter: %v", err)
		}
		for _, slug := range []string{"first-owner", "second-owner"} {
			if err := os.WriteFile(filepath.Join(repoRoot, ".specs", "runtime", slug+".yaml"), []byte(`slug: `+slug+`
charter: runtime
title: `+slug+`
status: draft
rev: 1
created: 2026-03-05
updated: 2026-03-30
last_verified_at: 2026-03-28
checkpoint: a1b2c3f
tags:
  - runtime
documents:
  primary: runtime/src/domain/shared/`+slug+`/SPEC.md
scope:
  - runtime/src/domain/shared/
deltas: []
requirements: []
changelog: []
`), 0o644); err != nil {
				t.Fatalf("write tracking %s: %v", slug, err)
			}
		}

		withWorkingDir(t, repoRoot, func() {
			stdout, stderr, exitCode := executeCLI("context", "--file", "runtime/src/domain/shared/transport.py")
			if exitCode != 0 {
				t.Fatalf("exit code = %d, stderr=%s", exitCode, stderr)
			}
			if stderr != "" {
				t.Fatalf("expected empty stderr, got %q", stderr)
			}

			var envelope struct {
				State struct {
					Resolution    string `json:"resolution"`
					MatchSource   string `json:"match_source"`
					GoverningSpec any    `json:"governing_spec"`
					Validation    struct {
						Valid    bool `json:"valid"`
						Findings []struct {
							Code string `json:"code"`
						} `json:"findings"`
					} `json:"validation"`
					Matches []struct {
						Slug string `json:"slug"`
					} `json:"matches"`
				} `json:"state"`
				Next testNextMaps `json:"next"`
			}
			mustUnmarshalJSON(t, stdout, &envelope)
			if envelope.State.Resolution != "ambiguous" || envelope.State.MatchSource != "scope" || envelope.State.GoverningSpec != nil {
				t.Fatalf("state = %#v", envelope.State)
			}
			if envelope.State.Validation.Valid || len(envelope.State.Validation.Findings) != 1 || envelope.State.Validation.Findings[0].Code != "AMBIGUOUS_FILE_OWNERSHIP" {
				t.Fatalf("validation = %#v", envelope.State.Validation)
			}
			if got := orderedSlugs(envelope.State.Matches); strings.Join(got, ",") != "first-owner,second-owner" {
				t.Fatalf("matches = %#v", envelope.State.Matches)
			}
			if len(envelope.Next) != 0 {
				t.Fatalf("next = %#v", envelope.Next)
			}
		})
	})

	t.Run("config", func(t *testing.T) {
		repoRoot := seededSpecRepo(t)
		writeProjectConfigFixture(t, repoRoot, `gherkin_tags:
  - runtime
  - domain
source_prefixes:
  - runtime/src/
formats:
  ui-spec:
    template: ui/src/routes/SPEC-FORMAT.md
    recommended_for: ui/src/routes/**
    description: 8-section literate UI spec
`)
		if err := os.MkdirAll(filepath.Join(repoRoot, "ui", "src", "routes"), 0755); err != nil {
			t.Fatalf("mkdir formats dir: %v", err)
		}
		if err := os.WriteFile(filepath.Join(repoRoot, "ui", "src", "routes", "SPEC-FORMAT.md"), []byte("# format\n"), 0644); err != nil {
			t.Fatalf("write format doc: %v", err)
		}

		withWorkingDir(t, repoRoot, func() {
			stdout, stderr, exitCode := executeCLI("config")
			if exitCode != 0 {
				t.Fatalf("exit code = %d, stderr=%s", exitCode, stderr)
			}

			var envelope struct {
				State struct {
					SemanticTags []string       `json:"semantic_tags"`
					GherkinTags  []string       `json:"gherkin_tags"`
					Formats      map[string]any `json:"formats"`
					Validation   map[string]any `json:"validation"`
				} `json:"state"`
			}
			mustUnmarshalJSON(t, stdout, &envelope)

			if strings.Join(envelope.State.SemanticTags, ",") != "e2e,manual" {
				t.Fatalf("semantic tags = %v", envelope.State.SemanticTags)
			}
			if strings.Join(envelope.State.GherkinTags, ",") != "runtime,domain" {
				t.Fatalf("gherkin tags = %v", envelope.State.GherkinTags)
			}
			format, ok := envelope.State.Formats["ui-spec"].(map[string]any)
			if !ok {
				t.Fatalf("formats = %#v", envelope.State.Formats)
			}
			if format["template"] != "ui/src/routes/SPEC-FORMAT.md" || format["recommended_for"] != "ui/src/routes/**" || format["description"] != "8-section literate UI spec" {
				t.Fatalf("format = %#v", format)
			}
			if _, exists := format["RecommendedFor"]; exists {
				t.Fatalf("format = %#v", format)
			}
			if envelope.State.Validation["valid"] != true {
				t.Fatalf("validation = %#v", envelope.State.Validation)
			}
			if stderr != "" {
				t.Fatalf("expected empty stderr, got %q", stderr)
			}
		})
	})

	t.Run("config falls back to four default source prefixes without specctl yaml", func(t *testing.T) {
		repoRoot := t.TempDir()
		if err := os.MkdirAll(filepath.Join(repoRoot, ".specs"), 0o755); err != nil {
			t.Fatalf("mkdir specs: %v", err)
		}

		withWorkingDir(t, repoRoot, func() {
			stdout, stderr, exitCode := executeCLI("config")
			if exitCode != 0 {
				t.Fatalf("exit code = %d, stderr=%s stdout=%s", exitCode, stderr, stdout)
			}
			if stderr != "" {
				t.Fatalf("expected empty stderr, got %q", stderr)
			}

			var envelope struct {
				State struct {
					SourcePrefixes []string `json:"source_prefixes"`
				} `json:"state"`
			}
			mustUnmarshalJSON(t, stdout, &envelope)
			if got := strings.Join(envelope.State.SourcePrefixes, ","); got != "runtime/src/,ui/src/,ui/convex/,ui/server/" {
				t.Fatalf("source_prefixes = %#v", envelope.State.SourcePrefixes)
			}
		})
	})

	t.Run("config aggregates repo validation findings", func(t *testing.T) {
		repoRoot := copyFixtureRepoWithRegistry(t, "ready-spec")
		replaceInFile(t, filepath.Join(repoRoot, ".specs", "runtime", "CHARTER.yaml"), "slug: session-lifecycle", "slug: missing-spec")

		withWorkingDir(t, repoRoot, func() {
			stdout, stderr, exitCode := executeCLI("config")
			if exitCode != 0 {
				t.Fatalf("exit code = %d, stderr=%s", exitCode, stderr)
			}

			var envelope struct {
				State struct {
					Validation struct {
						Valid    bool `json:"valid"`
						Findings []struct {
							Code string `json:"code"`
						} `json:"findings"`
					} `json:"validation"`
				} `json:"state"`
			}
			mustUnmarshalJSON(t, stdout, &envelope)
			if envelope.State.Validation.Valid || len(envelope.State.Validation.Findings) == 0 {
				t.Fatalf("validation = %#v", envelope.State.Validation)
			}
		})
	})

	t.Run("config audits config-managed spec state", func(t *testing.T) {
		repoRoot := copyFixtureRepoWithRegistry(t, "ready-spec")
		replaceInFile(t, filepath.Join(repoRoot, "runtime", "src", "domain", "session_execution", "SPEC.md"), "charter: runtime", "charter: runtime\nformat: unknown-format")

		withWorkingDir(t, repoRoot, func() {
			stdout, stderr, exitCode := executeCLI("config")
			if exitCode != 0 {
				t.Fatalf("exit code = %d, stderr=%s", exitCode, stderr)
			}

			var envelope struct {
				State struct {
					Validation struct {
						Valid    bool `json:"valid"`
						Findings []struct {
							Code string `json:"code"`
						} `json:"findings"`
					} `json:"validation"`
				} `json:"state"`
			}
			mustUnmarshalJSON(t, stdout, &envelope)
			if envelope.State.Validation.Valid {
				t.Fatalf("validation = %#v", envelope.State.Validation)
			}
			found := false
			for _, finding := range envelope.State.Validation.Findings {
				if finding.Code == "FORMAT_NOT_CONFIGURED" {
					found = true
				}
			}
			if !found {
				t.Fatalf("findings = %#v", envelope.State.Validation.Findings)
			}
			if stderr != "" {
				t.Fatalf("expected empty stderr, got %q", stderr)
			}
		})
	})

	t.Run("config returns validation for malformed persisted config state", func(t *testing.T) {
		repoRoot := copyFixtureRepoWithRegistry(t, "ready-spec")
		replaceInFile(t, filepath.Join(repoRoot, ".specs", "specctl.yaml"), "  - domain\n", "  - domain\n  - manual\n")

		withWorkingDir(t, repoRoot, func() {
			stdout, stderr, exitCode := executeCLI("config")
			if exitCode != 0 {
				t.Fatalf("exit code = %d, stderr=%s", exitCode, stderr)
			}

			var envelope struct {
				State struct {
					SemanticTags []string `json:"semantic_tags"`
					GherkinTags  []string `json:"gherkin_tags"`
					Validation   struct {
						Valid    bool `json:"valid"`
						Findings []struct {
							Code string `json:"code"`
						} `json:"findings"`
					} `json:"validation"`
				} `json:"state"`
			}
			mustUnmarshalJSON(t, stdout, &envelope)
			if strings.Join(envelope.State.SemanticTags, ",") != "e2e,manual" {
				t.Fatalf("semantic_tags = %v", envelope.State.SemanticTags)
			}
			if strings.Join(envelope.State.GherkinTags, ",") != "runtime,domain" {
				t.Fatalf("gherkin_tags = %v", envelope.State.GherkinTags)
			}
			if envelope.State.Validation.Valid || len(envelope.State.Validation.Findings) == 0 {
				t.Fatalf("validation = %#v", envelope.State.Validation)
			}
			if stderr != "" {
				t.Fatalf("expected empty stderr, got %q", stderr)
			}
		})
	})

	t.Run("config warns once for persisted semantic tags and stops after normalization", func(t *testing.T) {
		t.Setenv("XDG_CACHE_HOME", t.TempDir())
		repoRoot := copyFixtureRepoWithRegistry(t, "ready-spec")
		replaceInFile(t, filepath.Join(repoRoot, ".specs", "specctl.yaml"), "  - domain\n", "  - domain\n  - manual\n")

		withWorkingDir(t, repoRoot, func() {
			stdout, stderr, exitCode := executeCLI("config")
			if exitCode != 0 {
				t.Fatalf("exit code = %d, stderr=%s", exitCode, stderr)
			}
			if stderr != "" {
				t.Fatalf("expected empty stderr, got %q", stderr)
			}
			first := parseEnvelope(t, stdout)
			requireFindingCode(t, requireValidationFailure(t, first.State), "REDUNDANT_SEMANTIC_TAG")

			stdout, stderr, exitCode = executeCLI("config")
			if exitCode != 0 {
				t.Fatalf("second exit code = %d, stderr=%s", exitCode, stderr)
			}
			if stderr != "" {
				t.Fatalf("expected empty stderr, got %q", stderr)
			}
			second := parseEnvelope(t, stdout)
			if validation := second.State["validation"].(map[string]any); validation["valid"] != true {
				t.Fatalf("second validation = %#v", validation)
			}

			stdout, stderr, exitCode = executeCLI("config", "add-tag", "adapter")
			if exitCode != 0 {
				t.Fatalf("add-tag exit code = %d, stderr=%s", exitCode, stderr)
			}

			content, err := os.ReadFile(filepath.Join(repoRoot, ".specs", "specctl.yaml"))
			if err != nil {
				t.Fatalf("read specctl.yaml: %v", err)
			}
			if strings.Contains(string(content), "\n  - manual\n") {
				t.Fatalf("expected normalized config, got:\n%s", content)
			}

			stdout, stderr, exitCode = executeCLI("config")
			if exitCode != 0 {
				t.Fatalf("third exit code = %d, stderr=%s", exitCode, stderr)
			}
			if stderr != "" {
				t.Fatalf("expected empty stderr, got %q", stderr)
			}
			third := parseEnvelope(t, stdout)
			if validation := third.State["validation"].(map[string]any); validation["valid"] != true {
				t.Fatalf("third validation = %#v", validation)
			}
		})
	})

	t.Run("config audits tracking files whose charter file is missing", func(t *testing.T) {
		repoRoot := copyFixtureRepoWithRegistry(t, "ready-spec")
		if err := os.Remove(filepath.Join(repoRoot, ".specs", "runtime", "CHARTER.yaml")); err != nil {
			t.Fatalf("remove charter: %v", err)
		}

		withWorkingDir(t, repoRoot, func() {
			stdout, stderr, exitCode := executeCLI("config")
			if exitCode != 0 {
				t.Fatalf("exit code = %d, stderr=%s stdout=%s", exitCode, stderr, stdout)
			}

			var envelope struct {
				State struct {
					Validation struct {
						Valid    bool `json:"valid"`
						Findings []struct {
							Code   string `json:"code"`
							Path   string `json:"path"`
							Target string `json:"target"`
						} `json:"findings"`
					} `json:"validation"`
				} `json:"state"`
			}
			mustUnmarshalJSON(t, stdout, &envelope)
			if envelope.State.Validation.Valid {
				t.Fatalf("validation = %#v", envelope.State.Validation)
			}
			found := false
			for _, finding := range envelope.State.Validation.Findings {
				if finding.Code == "CHARTER_SPEC_MISSING" && finding.Path == ".specs/runtime/CHARTER.yaml" && finding.Target == "session-lifecycle" {
					found = true
				}
			}
			if !found {
				t.Fatalf("findings = %#v", envelope.State.Validation.Findings)
			}
		})
	})

	t.Run("context missing spec returns canonical create-spec template fields", func(t *testing.T) {
		repoRoot := copyFixtureRepoWithRegistry(t, "ready-spec")

		withWorkingDir(t, repoRoot, func() {
			stdout, stderr, exitCode := executeCLI("context", "runtime:missing-spec")
			if exitCode != 0 {
				t.Fatalf("exit code = %d, stderr=%s", exitCode, stderr)
			}

			var envelope struct {
				Next testNextMaps `json:"next"`
			}
			mustUnmarshalJSON(t, stdout, &envelope)
			next := requireNextAction(t, mapsToAny(envelope.Next), 0, "create_spec")
			if next["priority"] != float64(1) {
				t.Fatalf("priority = %#v", next["priority"])
			}
			template := requireTemplate(t, next)
			if argv := stringSliceFromAny(t, template["argv"]); strings.Join(argv, " ") != "specctl spec create runtime:missing-spec --title <title> --doc <design_doc> --scope <scope_dir_1>/ --group <group> --group-title <group_title> --group-order <group_order> --order <order> --charter-notes <charter_notes>" {
				t.Fatalf("template.argv = %v", argv)
			}
			if required := requiredFieldNames(t, template["required_fields"]); strings.Join(required, ",") != "title,design_doc,scope_dir_1,group,group_title,group_order,order,charter_notes" {
				t.Fatalf("required_fields = %v", required)
			}
			if descriptions := requiredFieldDescriptions(t, template["required_fields"]); strings.Join(descriptions, "|") != "Human-readable spec title|Repo-relative markdown path|First repo-relative governed directory ending in /|Charter group key|Required only when creating a new group|Integer order for a newly created group|Integer order for the spec inside its group|Short planning note for the charter entry" {
				t.Fatalf("required_field_descriptions = %v", descriptions)
			}
			if _, exists := template["stdin_format"]; exists {
				t.Fatalf("template.stdin_format = %#v, want omitted for argv-only run_command template", template["stdin_format"])
			}
			if _, exists := template["stdin_template"]; exists {
				t.Fatalf("template.stdin_template = %#v, want omitted for argv-only run_command template", template["stdin_template"])
			}
		})
	})

	t.Run("context missing charter returns create-charter before create-spec", func(t *testing.T) {
		repoRoot := copyFixtureRepoWithRegistry(t, "ready-spec")

		withWorkingDir(t, repoRoot, func() {
			stdout, stderr, exitCode := executeCLI("context", "adapters:missing-spec")
			if exitCode != 0 {
				t.Fatalf("exit code = %d, stderr=%s", exitCode, stderr)
			}

			var envelope struct {
				Next testNextMaps `json:"next"`
			}
			mustUnmarshalJSON(t, stdout, &envelope)
			createCharter := requireNextAction(t, mapsToAny(envelope.Next), 0, "create_charter")
			if createCharter["priority"] != float64(1) {
				t.Fatalf("create_charter priority = %#v", createCharter["priority"])
			}
			charterTemplate := requireTemplate(t, createCharter)
			if charterTemplate["stdin_format"] != "yaml" {
				t.Fatalf("stdin_format = %#v", charterTemplate["stdin_format"])
			}
			if charterTemplate["stdin_template"] != "title: <title>\ndescription: <description>\ngroups:\n  - key: <group_key>\n    title: <group_title>\n    order: <group_order>\n" {
				t.Fatalf("stdin_template = %#v", charterTemplate["stdin_template"])
			}
			if required := requiredFieldNames(t, charterTemplate["required_fields"]); strings.Join(required, ",") != "title,description,group_key,group_title,group_order" {
				t.Fatalf("required_fields = %v", required)
			}

			createSpec := requireNextAction(t, mapsToAny(envelope.Next), 1, "create_spec")
			if createSpec["priority"] != float64(2) {
				t.Fatalf("create_spec priority = %#v", createSpec["priority"])
			}
			specTemplate := requireTemplate(t, createSpec)
			if argv := stringSliceFromAny(t, specTemplate["argv"]); strings.Join(argv, " ") != "specctl spec create adapters:missing-spec --title <title> --doc <design_doc> --scope <scope_dir_1>/ --group <group> --order <order> --charter-notes <charter_notes>" {
				t.Fatalf("template.argv = %v", argv)
			}
			if required := requiredFieldNames(t, specTemplate["required_fields"]); strings.Join(required, ",") != "title,design_doc,scope_dir_1,group,order,charter_notes" {
				t.Fatalf("required_fields = %v", required)
			}
			if descriptions := requiredFieldDescriptions(t, specTemplate["required_fields"]); strings.Join(descriptions, "|") != "Human-readable spec title|Repo-relative markdown path|Governed directory ending in /|Existing charter group key|Integer order inside the group|Short planning note" {
				t.Fatalf("required_field_descriptions = %v", descriptions)
			}
			if _, exists := specTemplate["stdin_format"]; exists {
				t.Fatalf("template.stdin_format = %#v, want omitted for argv-only run_command template", specTemplate["stdin_format"])
			}
			if _, exists := specTemplate["stdin_template"]; exists {
				t.Fatalf("template.stdin_template = %#v, want omitted for argv-only run_command template", specTemplate["stdin_template"])
			}
		})
	})

	t.Run("context file no-match under a missing charter still returns only spec create", func(t *testing.T) {
		repoRoot := copyFixtureRepoWithRegistry(t, "ready-spec")
		if err := os.MkdirAll(filepath.Join(repoRoot, "adapters", "src", "http"), 0o755); err != nil {
			t.Fatalf("mkdir adapters/src/http: %v", err)
		}
		if err := os.WriteFile(filepath.Join(repoRoot, "adapters", "src", "http", "client.py"), []byte("pass\n"), 0o644); err != nil {
			t.Fatalf("write adapters client: %v", err)
		}

		withWorkingDir(t, repoRoot, func() {
			stdout, stderr, exitCode := executeCLI("context", "--file", "adapters/src/http/client.py")
			if exitCode != 0 {
				t.Fatalf("exit code = %d, stderr=%s stdout=%s", exitCode, stderr, stdout)
			}
			if stderr != "" {
				t.Fatalf("expected empty stderr, got %q", stderr)
			}

			var envelope struct {
				State struct {
					File       string `json:"file"`
					Resolution string `json:"resolution"`
				} `json:"state"`
				Next testNextMaps `json:"next"`
			}
			mustUnmarshalJSON(t, stdout, &envelope)
			if envelope.State.File != "adapters/src/http/client.py" || envelope.State.Resolution != "unmatched" {
				t.Fatalf("state = %#v", envelope.State)
			}

			createSpec := requireNextAction(t, mapsToAny(envelope.Next), 0, "create_spec")
			if createSpec["priority"] != float64(1) {
				t.Fatalf("create_spec priority = %#v", createSpec["priority"])
			}
			template := requireTemplate(t, createSpec)
			if argv := stringSliceFromAny(t, template["argv"]); strings.Join(argv, " ") != "specctl spec create adapters:http --title <title> --doc <design_doc> --scope adapters/src/http/ --group <group> --order <order> --charter-notes <charter_notes>" {
				t.Fatalf("template.argv = %v", argv)
			}
			if required := requiredFieldNames(t, template["required_fields"]); strings.Join(required, ",") != "title,design_doc,group,order,charter_notes" {
				t.Fatalf("required_fields = %v", required)
			}
		})
	})
}

func TestCharterCreateWritesAtomically(t *testing.T) {
	repoRoot := t.TempDir()
	if err := os.MkdirAll(filepath.Join(repoRoot, ".specs"), 0755); err != nil {
		t.Fatalf("mkdir specs dir: %v", err)
	}

	withWorkingDir(t, repoRoot, func() {
		stdout, stderr, exitCode := executeCLIWithInput("title: Runtime System\ndescription: Specs for runtime\ngroups:\n  - key: execution\n    title: Execution Engine\n    order: 10\n", "charter", "create", "runtime")
		if exitCode != 0 {
			t.Fatalf("exit code = %d, stderr=%s", exitCode, stderr)
		}

		var envelope struct {
			State struct {
				Name         string `json:"name"`
				TrackingFile string `json:"tracking_file"`
			} `json:"state"`
			Result struct {
				Kind          string `json:"kind"`
				TrackingFile  string `json:"tracking_file"`
				CreatedGroups []struct {
					Key string `json:"key"`
				} `json:"created_groups"`
			} `json:"result"`
			Next testNext `json:"next"`
		}
		mustUnmarshalJSON(t, stdout, &envelope)

		if envelope.State.Name != "runtime" || envelope.Result.Kind != "charter" {
			t.Fatalf("unexpected response %#v", envelope)
		}
		if envelope.Result.TrackingFile != ".specs/runtime/CHARTER.yaml" {
			t.Fatalf("tracking file = %q", envelope.Result.TrackingFile)
		}
		if len(envelope.Result.CreatedGroups) != 1 || envelope.Result.CreatedGroups[0].Key != "execution" {
			t.Fatalf("created groups = %#v", envelope.Result.CreatedGroups)
		}
		if len(envelope.Next) == 0 {
			t.Fatal("expected next actions")
		}
		next := requireNextAction(t, envelope.Next, 0, "create_spec")
		template := requireTemplate(t, next)
		if argv := stringSliceFromAny(t, template["argv"]); strings.Join(argv, " ") != "specctl spec create runtime:<slug> --title <title> --doc <design_doc> --scope <scope_dir_1>/ --group execution --order <order> --charter-notes <charter_notes>" {
			t.Fatalf("template.argv = %v", argv)
		}
		if required := requiredFieldNames(t, template["required_fields"]); strings.Join(required, ",") != "slug,title,design_doc,scope_dir_1,order,charter_notes" {
			t.Fatalf("required_fields = %v", required)
		}
		if stderr != "" {
			t.Fatalf("expected empty stderr, got %q", stderr)
		}

		content, err := os.ReadFile(filepath.Join(repoRoot, ".specs", "runtime", "CHARTER.yaml"))
		if err != nil {
			t.Fatalf("read charter: %v", err)
		}
		if !strings.Contains(string(content), "name: runtime") {
			t.Fatalf("unexpected file contents:\n%s", content)
		}
	})
}

func TestCharterCreateValidationFailureLeavesNoPartialFile(t *testing.T) {
	repoRoot := t.TempDir()
	if err := os.MkdirAll(filepath.Join(repoRoot, ".specs"), 0755); err != nil {
		t.Fatalf("mkdir specs dir: %v", err)
	}

	withWorkingDir(t, repoRoot, func() {
		stdout, stderr, exitCode := executeCLIWithInput("title: Runtime System\nunknown: nope\n", "charter", "create", "runtime")
		if exitCode == 0 {
			t.Fatalf("expected failure, stdout=%q stderr=%q", stdout, stderr)
		}

		envelope := requireFailureEnvelope(t, stdout, stderr)
		if envelope.Error == nil || envelope.Error.Code != "INVALID_INPUT" {
			t.Fatalf("expected INVALID_INPUT, got %#v", envelope)
		}
		if _, err := os.Stat(filepath.Join(repoRoot, ".specs", "runtime", "CHARTER.yaml")); !os.IsNotExist(err) {
			t.Fatalf("expected no persisted charter file, got err=%v", err)
		}
		matches, err := filepath.Glob(filepath.Join(repoRoot, ".specs", "runtime", ".*.tmp"))
		if err != nil {
			t.Fatalf("glob temp files: %v", err)
		}
		if len(matches) != 0 {
			t.Fatalf("expected no temp files, got %v", matches)
		}
	})
}

func TestCharterCreateSemanticValidationFailureUsesStructuredEnvelope(t *testing.T) {
	repoRoot := t.TempDir()
	if err := os.MkdirAll(filepath.Join(repoRoot, ".specs"), 0o755); err != nil {
		t.Fatalf("mkdir specs dir: %v", err)
	}

	withWorkingDir(t, repoRoot, func() {
		stdout, stderr, exitCode := executeCLIWithInput("title: Runtime System\ndescription: Specs for runtime control-plane and data-plane behavior\ngroups:\n  - key: execution\n    title: Execution Engine\n    order: -1\n", "charter", "create", "runtime")
		if exitCode == 0 {
			t.Fatalf("expected failure, stdout=%q stderr=%q", stdout, stderr)
		}

		envelope := requireFailureEnvelope(t, stdout, stderr)
		if envelope.Error == nil || envelope.Error.Code != "VALIDATION_FAILED" {
			t.Fatalf("unexpected envelope %#v", envelope)
		}
		if envelope.State["name"] != "runtime" || envelope.State["tracking_file"] != ".specs/runtime/CHARTER.yaml" {
			t.Fatalf("state = %#v", envelope.State)
		}
		groups := envelope.State["groups"].([]any)
		if len(groups) != 1 || groups[0].(map[string]any)["order"] != float64(-1) {
			t.Fatalf("groups = %#v", groups)
		}
		findings := requireValidationFailure(t, envelope.State)
		first := findings[0].(map[string]any)
		if first["code"] != "CHARTER_GROUP_INVALID" {
			t.Fatalf("findings = %#v", findings)
		}
	})
}

func TestRequestDecoders(t *testing.T) {
	t.Run("charter add-spec strict stdin", func(t *testing.T) {
		cmd := newCharterAddSpecCmd()
		cmd.SetArgs([]string{"runtime", "session-lifecycle"})
		cmd.SetIn(strings.NewReader("group: recovery\norder: 20\ndepends_on:\n  - redis-state\nnotes: Session FSM\n"))

		request, err := decodeCharterAddSpecRequest(cmd, []string{"runtime", "session-lifecycle"})
		if err != nil {
			t.Fatalf("decodeCharterAddSpecRequest: %v", err)
		}
		if request.Group != "recovery" || request.Order == nil || *request.Order != 20 {
			t.Fatalf("unexpected request %#v", request)
		}
	})

	t.Run("charter add-spec rejects unknown keys", func(t *testing.T) {
		cmd := newCharterAddSpecCmd()
		cmd.SetIn(strings.NewReader("group: recovery\norder: 20\nnotes: Session FSM\nunknown: nope\n"))

		_, err := decodeCharterAddSpecRequest(cmd, []string{"runtime", "session-lifecycle"})
		if err == nil || !strings.Contains(err.Error(), "invalid stdin YAML") {
			t.Fatalf("expected strict decode error, got %v", err)
		}
	})

	t.Run("charter add-spec enforces required fields", func(t *testing.T) {
		cmd := newCharterAddSpecCmd()
		cmd.SetIn(strings.NewReader("group: recovery\nnotes: Session FSM\n"))

		_, err := decodeCharterAddSpecRequest(cmd, []string{"runtime", "session-lifecycle"})
		if err == nil || !strings.Contains(err.Error(), "order is required") {
			t.Fatalf("expected required field error, got %v", err)
		}
	})

	t.Run("charter add-spec leaves partial group metadata for persisted-state validation", func(t *testing.T) {
		cmd := newCharterAddSpecCmd()
		cmd.SetIn(strings.NewReader("group: recovery\ngroup_title: Recovery and Cleanup\norder: 20\nnotes: Session FSM\n"))

		request, err := decodeCharterAddSpecRequest(cmd, []string{"runtime", "session-lifecycle"})
		if err != nil {
			t.Fatalf("decodeCharterAddSpecRequest: %v", err)
		}
		if request.Group != "recovery" || request.GroupTitle != "Recovery and Cleanup" || request.GroupOrder != nil {
			t.Fatalf("unexpected request %#v", request)
		}
	})

	t.Run("spec create decodes typed request", func(t *testing.T) {
		cmd := newSpecCreateCmd()
		cmd.SetArgs([]string{"runtime:session-lifecycle"})
		if err := cmd.Flags().Set("title", "Session Lifecycle"); err != nil {
			t.Fatalf("set title: %v", err)
		}
		if err := cmd.Flags().Set("doc", "runtime/src/domain/session_execution/SPEC.md"); err != nil {
			t.Fatalf("set doc: %v", err)
		}
		if err := cmd.Flags().Set("scope", "runtime/src/domain/session_execution/"); err != nil {
			t.Fatalf("set scope: %v", err)
		}
		if err := cmd.Flags().Set("group", "recovery"); err != nil {
			t.Fatalf("set group: %v", err)
		}
		if err := cmd.Flags().Set("group-title", "Recovery"); err != nil {
			t.Fatalf("set group-title: %v", err)
		}
		if err := cmd.Flags().Set("group-order", "20"); err != nil {
			t.Fatalf("set group-order: %v", err)
		}
		if err := cmd.Flags().Set("order", "30"); err != nil {
			t.Fatalf("set order: %v", err)
		}
		if err := cmd.Flags().Set("charter-notes", "Session FSM"); err != nil {
			t.Fatalf("set charter-notes: %v", err)
		}
		if err := cmd.Flags().Set("depends-on", "redis-state"); err != nil {
			t.Fatalf("set depends-on: %v", err)
		}
		if err := cmd.Flags().Set("tag", "runtime"); err != nil {
			t.Fatalf("set tag: %v", err)
		}

		request, err := decodeSpecCreateRequest(cmd, []string{"runtime:session-lifecycle"})
		if err != nil {
			t.Fatalf("decodeSpecCreateRequest: %v", err)
		}
		if request.Target != "runtime:session-lifecycle" || request.Group != "recovery" || request.GroupTitle == nil || *request.GroupTitle != "Recovery" {
			t.Fatalf("unexpected request %#v", request)
		}
		if len(request.DependsOn) != 1 || request.DependsOn[0] != "redis-state" {
			t.Fatalf("depends_on = %v", request.DependsOn)
		}
	})
}

func TestSpecCreateWritesFocusedV2State(t *testing.T) {
	t.Run("fails with checkpoint unavailable before the first commit exists", func(t *testing.T) {
		repoRoot := charterOnlyRepoWithoutHead(t)

		withWorkingDir(t, repoRoot, func() {
			stdout, stderr, exitCode := executeCLI(
				"spec", "create", "runtime:session-lifecycle",
				"--title", "Session Lifecycle",
				"--doc", "runtime/src/domain/session_execution/SPEC.md",
				"--scope", "runtime/src/domain/session_execution/",
				"--group", "recovery",
				"--order", "20",
				"--charter-notes", "Session FSM and cleanup behavior",
			)
			if exitCode == 0 {
				t.Fatalf("expected failure, stdout=%q stderr=%q", stdout, stderr)
			}

			envelope := requireFailureEnvelope(t, stdout, stderr)
			if envelope.Error == nil || envelope.Error.Code != "CHECKPOINT_UNAVAILABLE" {
				t.Fatalf("unexpected envelope %#v", envelope)
			}
			if _, err := os.Stat(filepath.Join(repoRoot, ".specs", "runtime", "session-lifecycle.yaml")); !os.IsNotExist(err) {
				t.Fatalf("tracking file unexpectedly created, stat err=%v", err)
			}
		})
	})

	t.Run("bootstraps missing design doc", func(t *testing.T) {
		repoRoot := charterOnlyRepo(t)

		withWorkingDir(t, repoRoot, func() {
			stdout, stderr, exitCode := executeCLI(
				"spec", "create", "runtime:session-lifecycle",
				"--title", "Session Lifecycle",
				"--doc", "runtime/src/domain/session_execution/SPEC.md",
				"--scope", "runtime/src/domain/session_execution/",
				"--group", "recovery",
				"--order", "20",
				"--charter-notes", "Session FSM and cleanup behavior",
			)
			if exitCode != 0 {
				t.Fatalf("exit code = %d, stderr=%s", exitCode, stderr)
			}

			var envelope struct {
				State struct {
					Slug              string `json:"slug"`
					Status            string `json:"status"`
					CharterMembership struct {
						Group string `json:"group"`
						Order int    `json:"order"`
						Notes string `json:"notes"`
					} `json:"charter_membership"`
				} `json:"state"`
				Result struct {
					Kind            string `json:"kind"`
					DesignDocAction string `json:"design_doc_action"`
					DesignDoc       string `json:"design_doc"`
					SelectedFormat  any    `json:"selected_format"`
				} `json:"result"`
				Next testNextMaps `json:"next"`
			}
			mustUnmarshalJSON(t, stdout, &envelope)

			if envelope.Result.Kind != "spec" {
				t.Fatalf("result.kind = %q", envelope.Result.Kind)
			}
			if envelope.Result.DesignDocAction != "bootstrapped" {
				t.Fatalf("design_doc_action = %q", envelope.Result.DesignDocAction)
			}
			if envelope.Result.DesignDoc != "runtime/src/domain/session_execution/SPEC.md" {
				t.Fatalf("design_doc = %q", envelope.Result.DesignDoc)
			}
			if envelope.Result.SelectedFormat != nil {
				t.Fatalf("selected_format = %#v, want null when no configured format matches", envelope.Result.SelectedFormat)
			}
			if envelope.State.Status != "draft" {
				t.Fatalf("status = %q", envelope.State.Status)
			}
			if envelope.State.CharterMembership.Group != "recovery" || envelope.State.CharterMembership.Order != 20 {
				t.Fatalf("unexpected membership %#v", envelope.State.CharterMembership)
			}
			// spec create returns create_format_template when no format is configured
			if len(envelope.Next) != 1 {
				t.Fatalf("expected 1 next step, got %d: %#v", len(envelope.Next), envelope.Next)
			}
			if envelope.Next[0]["action"] != "create_format_template" {
				t.Fatalf("expected create_format_template, got %#v", envelope.Next[0])
			}

			docContent, err := os.ReadFile(filepath.Join(repoRoot, "runtime", "src", "domain", "session_execution", "SPEC.md"))
			if err != nil {
				t.Fatalf("read design doc: %v", err)
			}
			if !strings.Contains(string(docContent), "spec: session-lifecycle") {
				t.Fatalf("unexpected design doc:\n%s", docContent)
			}

			charterContent, err := os.ReadFile(filepath.Join(repoRoot, ".specs", "runtime", "CHARTER.yaml"))
			if err != nil {
				t.Fatalf("read charter: %v", err)
			}
			if !strings.Contains(string(charterContent), "slug: session-lifecycle") {
				t.Fatalf("charter missing new entry:\n%s", charterContent)
			}
		})
	})

	t.Run("prepends frontmatter to existing doc", func(t *testing.T) {
		repoRoot := charterOnlyRepo(t)
		if err := os.WriteFile(filepath.Join(repoRoot, "runtime", "src", "domain", "session_execution", "SPEC.md"), []byte("# Session Lifecycle\n"), 0644); err != nil {
			t.Fatalf("write existing doc: %v", err)
		}

		withWorkingDir(t, repoRoot, func() {
			stdout, stderr, exitCode := executeCLI(
				"spec", "create", "runtime:session-lifecycle",
				"--title", "Session Lifecycle",
				"--doc", "runtime/src/domain/session_execution/SPEC.md",
				"--scope", "runtime/src/domain/session_execution/",
				"--group", "recovery",
				"--order", "20",
				"--charter-notes", "Session FSM and cleanup behavior",
			)
			if exitCode != 0 {
				t.Fatalf("exit code = %d, stderr=%s", exitCode, stderr)
			}

			var envelope struct {
				Result struct {
					DesignDocAction string `json:"design_doc_action"`
				} `json:"result"`
			}
			mustUnmarshalJSON(t, stdout, &envelope)
			if envelope.Result.DesignDocAction != "prepended_frontmatter" {
				t.Fatalf("design_doc_action = %q", envelope.Result.DesignDocAction)
			}

			docContent, err := os.ReadFile(filepath.Join(repoRoot, "runtime", "src", "domain", "session_execution", "SPEC.md"))
			if err != nil {
				t.Fatalf("read design doc: %v", err)
			}
			if !strings.HasPrefix(string(docContent), "---\nspec: session-lifecycle\ncharter: runtime\n---\n# Session Lifecycle\n") {
				t.Fatalf("unexpected design doc:\n%s", docContent)
			}
		})
	})

	t.Run("rewrites matching frontmatter to insert auto-selected format", func(t *testing.T) {
		repoRoot := charterOnlyRepo(t)
		writeProjectConfigFixture(t, repoRoot, `gherkin_tags:
  - runtime
  - domain
source_prefixes:
  - runtime/src/
formats:
  runtime-spec:
    template: runtime/SPEC-FORMAT.md
    recommended_for: runtime/src/domain/**
    description: Runtime domain spec
`)
		if err := os.WriteFile(filepath.Join(repoRoot, "runtime", "SPEC-FORMAT.md"), []byte("# Runtime spec format\n"), 0644); err != nil {
			t.Fatalf("write format template: %v", err)
		}
		if err := os.WriteFile(filepath.Join(repoRoot, "runtime", "src", "domain", "session_execution", "SPEC.md"), []byte("---\nspec: session-lifecycle\ncharter: runtime\n---\n# Session Lifecycle\n"), 0644); err != nil {
			t.Fatalf("write existing doc: %v", err)
		}

		withWorkingDir(t, repoRoot, func() {
			stdout, stderr, exitCode := executeCLI(
				"spec", "create", "runtime:session-lifecycle",
				"--title", "Session Lifecycle",
				"--doc", "runtime/src/domain/session_execution/SPEC.md",
				"--scope", "runtime/src/domain/session_execution/",
				"--group", "recovery",
				"--order", "20",
				"--charter-notes", "Session FSM and cleanup behavior",
			)
			if exitCode != 0 {
				t.Fatalf("exit code = %d, stderr=%s", exitCode, stderr)
			}

			var envelope struct {
				Result struct {
					DesignDocAction string `json:"design_doc_action"`
					SelectedFormat  string `json:"selected_format"`
				} `json:"result"`
				Next testNextMaps `json:"next"`
			}
			mustUnmarshalJSON(t, stdout, &envelope)
			if envelope.Result.DesignDocAction != "rewritten_frontmatter" {
				t.Fatalf("design_doc_action = %q", envelope.Result.DesignDocAction)
			}
			if envelope.Result.SelectedFormat != "runtime-spec" {
				t.Fatalf("selected_format = %q", envelope.Result.SelectedFormat)
			}
			if len(envelope.Next) != 0 {
				t.Fatalf("next = %#v", envelope.Next)
			}

			docContent, err := os.ReadFile(filepath.Join(repoRoot, "runtime", "src", "domain", "session_execution", "SPEC.md"))
			if err != nil {
				t.Fatalf("read design doc: %v", err)
			}
			if !strings.HasPrefix(string(docContent), "---\nspec: session-lifecycle\ncharter: runtime\nformat: runtime-spec\n---\n# Session Lifecycle\n") {
				t.Fatalf("unexpected design doc:\n%s", docContent)
			}
		})
	})

	t.Run("auto-selects doublestar e2e format during create", func(t *testing.T) {
		repoRoot := charterOnlyRepo(t)
		writeProjectConfigFixture(t, repoRoot, `gherkin_tags:
  - runtime
  - e2e
source_prefixes:
  - runtime/src/
formats:
  e2e-context:
    template: runtime/tests/e2e/CONTEXT-FORMAT.md
    recommended_for: "**/tests/e2e/**"
    description: E2E context doc
`)
		if err := os.MkdirAll(filepath.Join(repoRoot, "runtime", "tests", "e2e", "journeys"), 0755); err != nil {
			t.Fatalf("mkdir e2e dir: %v", err)
		}
		if err := os.WriteFile(filepath.Join(repoRoot, "runtime", "tests", "e2e", "CONTEXT-FORMAT.md"), []byte("# E2E context format\n"), 0644); err != nil {
			t.Fatalf("write format template: %v", err)
		}

		withWorkingDir(t, repoRoot, func() {
			stdout, stderr, exitCode := executeCLI(
				"spec", "create", "runtime:session-context",
				"--title", "Session Context",
				"--doc", "runtime/tests/e2e/journeys/CONTEXT.md",
				"--scope", "runtime/tests/e2e/journeys/",
				"--group", "recovery",
				"--order", "30",
				"--charter-notes", "E2E context ownership",
			)
			if exitCode != 0 {
				t.Fatalf("exit code = %d, stderr=%s stdout=%s", exitCode, stderr, stdout)
			}

			var envelope struct {
				Result struct {
					DesignDocAction string `json:"design_doc_action"`
					SelectedFormat  string `json:"selected_format"`
				} `json:"result"`
			}
			mustUnmarshalJSON(t, stdout, &envelope)
			if envelope.Result.DesignDocAction != "bootstrapped" {
				t.Fatalf("design_doc_action = %q", envelope.Result.DesignDocAction)
			}
			if envelope.Result.SelectedFormat != "e2e-context" {
				t.Fatalf("selected_format = %q", envelope.Result.SelectedFormat)
			}

			docContent, err := os.ReadFile(filepath.Join(repoRoot, "runtime", "tests", "e2e", "journeys", "CONTEXT.md"))
			if err != nil {
				t.Fatalf("read design doc: %v", err)
			}
			if !strings.HasPrefix(string(docContent), "---\nspec: session-context\ncharter: runtime\nformat: e2e-context\n---\n") {
				t.Fatalf("unexpected design doc:\n%s", docContent)
			}
		})
	})

	t.Run("validates already-matching frontmatter without rewriting", func(t *testing.T) {
		repoRoot := charterOnlyRepo(t)
		docPath := filepath.Join(repoRoot, "runtime", "src", "domain", "session_execution", "SPEC.md")
		docContent := []byte("---\nspec: session-lifecycle\ncharter: runtime\n---\n# Session Lifecycle\n")
		if err := os.WriteFile(docPath, docContent, 0644); err != nil {
			t.Fatalf("write existing doc: %v", err)
		}
		originalMTime := mustParseTestTime(t, "2024-01-02T03:04:05Z")
		if err := os.Chtimes(docPath, originalMTime, originalMTime); err != nil {
			t.Fatalf("set doc mtime: %v", err)
		}

		withWorkingDir(t, repoRoot, func() {
			stdout, stderr, exitCode := executeCLI(
				"spec", "create", "runtime:session-lifecycle",
				"--title", "Session Lifecycle",
				"--doc", "runtime/src/domain/session_execution/SPEC.md",
				"--scope", "runtime/src/domain/session_execution/",
				"--group", "recovery",
				"--order", "20",
				"--charter-notes", "Session FSM and cleanup behavior",
			)
			if exitCode != 0 {
				t.Fatalf("exit code = %d, stderr=%s", exitCode, stderr)
			}

			var envelope struct {
				Result struct {
					DesignDocAction string `json:"design_doc_action"`
				} `json:"result"`
				Next testNextMaps `json:"next"`
			}
			mustUnmarshalJSON(t, stdout, &envelope)
			if envelope.Result.DesignDocAction != "validated_existing" {
				t.Fatalf("design_doc_action = %q", envelope.Result.DesignDocAction)
			}
			// spec create returns create_format_template when no format is configured
			if len(envelope.Next) != 1 {
				t.Fatalf("expected 1 next step, got %d: %#v", len(envelope.Next), envelope.Next)
			}
			if envelope.Next[0]["action"] != "create_format_template" {
				t.Fatalf("expected create_format_template, got %#v", envelope.Next[0])
			}

			updatedContent, err := os.ReadFile(docPath)
			if err != nil {
				t.Fatalf("read design doc: %v", err)
			}
			if string(updatedContent) != string(docContent) {
				t.Fatalf("design doc content changed:\n%s", updatedContent)
			}
			info, err := os.Stat(docPath)
			if err != nil {
				t.Fatalf("stat design doc: %v", err)
			}
			if !info.ModTime().Equal(originalMTime) {
				t.Fatalf("design doc mtime = %s, want %s", info.ModTime().UTC().Format(time.RFC3339Nano), originalMTime.UTC().Format(time.RFC3339Nano))
			}
		})
	})

	t.Run("rejects mismatched frontmatter", func(t *testing.T) {
		repoRoot := charterOnlyRepo(t)
		content := "---\nspec: other-spec\ncharter: runtime\n---\n# Session Lifecycle\n"
		if err := os.WriteFile(filepath.Join(repoRoot, "runtime", "src", "domain", "session_execution", "SPEC.md"), []byte(content), 0644); err != nil {
			t.Fatalf("write existing doc: %v", err)
		}

		withWorkingDir(t, repoRoot, func() {
			stdout, stderr, exitCode := executeCLI(
				"spec", "create", "runtime:session-lifecycle",
				"--title", "Session Lifecycle",
				"--doc", "runtime/src/domain/session_execution/SPEC.md",
				"--scope", "runtime/src/domain/session_execution/",
				"--group", "recovery",
				"--order", "20",
				"--charter-notes", "Session FSM and cleanup behavior",
			)
			if exitCode == 0 {
				t.Fatalf("expected failure, stdout=%q", stdout)
			}
			envelope := requireFailureEnvelope(t, stdout, stderr)
			if envelope.Error == nil || envelope.Error.Code != "PRIMARY_DOC_FRONTMATTER_MISMATCH" {
				t.Fatalf("unexpected error %#v", envelope)
			}
			focus := envelope.State["focus"].(map[string]any)["design_doc"].(map[string]any)
			if focus["path"] != "runtime/src/domain/session_execution/SPEC.md" {
				t.Fatalf("design_doc focus = %#v", focus)
			}
		})
	})

	t.Run("requires new group metadata when the group does not exist", func(t *testing.T) {
		repoRoot := charterOnlyRepo(t)

		withWorkingDir(t, repoRoot, func() {
			stdout, stderr, exitCode := executeCLI(
				"spec", "create", "runtime:session-lifecycle",
				"--title", "Session Lifecycle",
				"--doc", "runtime/src/domain/session_execution/SPEC.md",
				"--scope", "runtime/src/domain/session_execution/",
				"--group", "new-group",
				"--order", "20",
				"--charter-notes", "Session FSM and cleanup behavior",
			)
			if exitCode == 0 {
				t.Fatalf("expected failure, stdout=%q", stdout)
			}
			envelope := requireFailureEnvelope(t, stdout, stderr)
			if envelope.Error == nil || envelope.Error.Code != "GROUP_REQUIRED" {
				t.Fatalf("unexpected error %#v", envelope)
			}
			focus := envelope.State["focus"].(map[string]any)["input"].(map[string]any)
			if got := strings.Join(stringSliceFromAny(t, focus["missing_fields"]), ","); got != "group_title,group_order" {
				t.Fatalf("missing_fields = %#v", focus["missing_fields"])
			}
		})
	})

	t.Run("rejects ambiguous format auto-selection", func(t *testing.T) {
		repoRoot := charterOnlyRepo(t)
		writeProjectConfigFixture(t, repoRoot, `gherkin_tags:
  - runtime
  - domain
source_prefixes:
  - runtime/src/
formats:
  runtime-a:
    template: runtime/A.md
    recommended_for: runtime/src/domain/**
    description: Runtime format A
  runtime-b:
    template: runtime/B.md
    recommended_for: runtime/src/domain/**
    description: Runtime format B
`)
		if err := os.WriteFile(filepath.Join(repoRoot, "runtime", "A.md"), []byte("# A\n"), 0644); err != nil {
			t.Fatalf("write format template A: %v", err)
		}
		if err := os.WriteFile(filepath.Join(repoRoot, "runtime", "B.md"), []byte("# B\n"), 0644); err != nil {
			t.Fatalf("write format template B: %v", err)
		}

		withWorkingDir(t, repoRoot, func() {
			stdout, stderr, exitCode := executeCLI(
				"spec", "create", "runtime:session-lifecycle",
				"--title", "Session Lifecycle",
				"--doc", "runtime/src/domain/session_execution/SPEC.md",
				"--scope", "runtime/src/domain/session_execution/",
				"--group", "recovery",
				"--order", "20",
				"--charter-notes", "Session FSM and cleanup behavior",
			)
			if exitCode == 0 {
				t.Fatalf("expected failure, stdout=%q", stdout)
			}
			envelope := requireFailureEnvelope(t, stdout, stderr)
			if envelope.Error == nil || envelope.Error.Code != "FORMAT_AMBIGUOUS" {
				t.Fatalf("unexpected error %#v", envelope)
			}
			focus := envelope.State["focus"].(map[string]any)["design_doc"].(map[string]any)
			if focus["path"] != "runtime/src/domain/session_execution/SPEC.md" {
				t.Fatalf("design_doc focus = %#v", focus)
			}
		})
	})

	t.Run("rejects unknown existing frontmatter format", func(t *testing.T) {
		repoRoot := charterOnlyRepo(t)
		if err := os.WriteFile(filepath.Join(repoRoot, "runtime", "src", "domain", "session_execution", "SPEC.md"), []byte("---\nspec: session-lifecycle\ncharter: runtime\nformat: mystery\n---\n# Session Lifecycle\n"), 0644); err != nil {
			t.Fatalf("write existing doc: %v", err)
		}

		withWorkingDir(t, repoRoot, func() {
			stdout, stderr, exitCode := executeCLI(
				"spec", "create", "runtime:session-lifecycle",
				"--title", "Session Lifecycle",
				"--doc", "runtime/src/domain/session_execution/SPEC.md",
				"--scope", "runtime/src/domain/session_execution/",
				"--group", "recovery",
				"--order", "20",
				"--charter-notes", "Session FSM and cleanup behavior",
			)
			if exitCode == 0 {
				t.Fatalf("expected failure, stdout=%q", stdout)
			}
			envelope := requireFailureEnvelope(t, stdout, stderr)
			if envelope.Error == nil || envelope.Error.Code != "FORMAT_NOT_CONFIGURED" {
				t.Fatalf("unexpected error %#v", envelope)
			}
		})
	})

	t.Run("rejects semantic tracking validation as structured failure", func(t *testing.T) {
		repoRoot := charterOnlyRepo(t)

		withWorkingDir(t, repoRoot, func() {
			stdout, stderr, exitCode := executeCLI(
				"spec", "create", "runtime:session-lifecycle",
				"--title", "Session Lifecycle",
				"--doc", "runtime/src/domain/session_execution/SPEC.md",
				"--scope", "runtime/src/domain/session_execution/",
				"--group", "recovery",
				"--order", "20",
				"--charter-notes", "Session FSM and cleanup behavior",
				"--tag", "Invalid-Tag",
			)
			if exitCode == 0 {
				t.Fatalf("expected failure, stdout=%q stderr=%q", stdout, stderr)
			}

			envelope := requireFailureEnvelope(t, stdout, stderr)
			if envelope.Error == nil || envelope.Error.Code != "VALIDATION_FAILED" {
				t.Fatalf("unexpected envelope %#v", envelope)
			}
			if envelope.State["slug"] != "session-lifecycle" || envelope.State["charter"] != "runtime" || envelope.State["tracking_file"] != ".specs/runtime/session-lifecycle.yaml" {
				t.Fatalf("state = %#v", envelope.State)
			}
			membership := envelope.State["charter_membership"].(map[string]any)
			if membership["group"] != "recovery" || membership["order"] != float64(20) {
				t.Fatalf("charter_membership = %#v", membership)
			}
			findings := requireValidationFailure(t, envelope.State)
			foundTagInvalid := false
			for _, findingAny := range findings {
				finding := findingAny.(map[string]any)
				if finding["code"] == "SPEC_TAG_INVALID" {
					foundTagInvalid = true
				}
			}
			if !foundTagInvalid {
				t.Fatalf("findings = %#v", findings)
			}
			if _, err := os.Stat(filepath.Join(repoRoot, ".specs", "runtime", "session-lifecycle.yaml")); !os.IsNotExist(err) {
				t.Fatalf("expected no persisted tracking file, got err=%v", err)
			}
		})
	})

	t.Run("rejects charter membership validation as structured failure", func(t *testing.T) {
		repoRoot := copyFixtureRepoWithRegistry(t, "charter-dag")
		if err := os.MkdirAll(filepath.Join(repoRoot, "runtime", "src", "domain", "new_protocol"), 0o755); err != nil {
			t.Fatalf("mkdir design doc dir: %v", err)
		}

		withWorkingDir(t, repoRoot, func() {
			stdout, stderr, exitCode := executeCLI(
				"spec", "create", "runtime:new-protocol",
				"--title", "New Protocol",
				"--doc", "runtime/src/domain/new_protocol/SPEC.md",
				"--scope", "runtime/src/domain/new_protocol/",
				"--group", "recovery",
				"--order", "30",
				"--charter-notes", "Protocol planning",
				"--depends-on", "missing-spec",
			)
			if exitCode == 0 {
				t.Fatalf("expected failure, stdout=%q stderr=%q", stdout, stderr)
			}

			envelope := requireFailureEnvelope(t, stdout, stderr)
			if envelope.Error == nil || envelope.Error.Code != "VALIDATION_FAILED" {
				t.Fatalf("unexpected envelope %#v", envelope)
			}
			if envelope.State["slug"] != "new-protocol" || envelope.State["charter"] != "runtime" || envelope.State["tracking_file"] != ".specs/runtime/new-protocol.yaml" {
				t.Fatalf("state = %#v", envelope.State)
			}
			membership := envelope.State["charter_membership"].(map[string]any)
			if membership["group"] != "recovery" || membership["order"] != float64(30) {
				t.Fatalf("charter_membership = %#v", membership)
			}
			if got := strings.Join(stringSliceFromAny(t, membership["depends_on"]), ","); got != "missing-spec" {
				t.Fatalf("charter_membership.depends_on = %#v", membership["depends_on"])
			}
			findings := requireValidationFailure(t, envelope.State)
			if len(findings) == 0 {
				t.Fatalf("findings = %#v", findings)
			}
			if _, err := os.Stat(filepath.Join(repoRoot, ".specs", "runtime", "new-protocol.yaml")); !os.IsNotExist(err) {
				t.Fatalf("expected no persisted tracking file, got err=%v", err)
			}
		})
	})

	t.Run("rejects post-write repo audit failures without persisting partial spec state", func(t *testing.T) {
		repoRoot := copyFixtureRepoWithRegistry(t, "ready-spec")
		writeCLIAdjacentTestFile(t, filepath.Join(repoRoot, ".specs", "runtime", "stray-tracking.yaml"), []byte(`slug: stray-tracking
charter: runtime
title: Stray Tracking
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
changelog: []
`))
		if err := os.MkdirAll(filepath.Join(repoRoot, "runtime", "src", "domain", "new_protocol"), 0o755); err != nil {
			t.Fatalf("mkdir design doc dir: %v", err)
		}

		withWorkingDir(t, repoRoot, func() {
			stdout, stderr, exitCode := executeCLI(
				"spec", "create", "runtime:new-protocol",
				"--title", "New Protocol",
				"--doc", "runtime/src/domain/new_protocol/SPEC.md",
				"--scope", "runtime/src/domain/new_protocol/",
				"--group", "recovery",
				"--order", "30",
				"--charter-notes", "Protocol planning",
			)
			if exitCode == 0 {
				t.Fatalf("expected failure, stdout=%q stderr=%q", stdout, stderr)
			}

			envelope := requireFailureEnvelope(t, stdout, stderr)
			if envelope.Error == nil || envelope.Error.Code != "VALIDATION_FAILED" {
				t.Fatalf("unexpected envelope %#v", envelope)
			}
			if envelope.State["slug"] != "new-protocol" || envelope.State["charter"] != "runtime" || envelope.State["tracking_file"] != ".specs/runtime/new-protocol.yaml" {
				t.Fatalf("state = %#v", envelope.State)
			}
			membership := envelope.State["charter_membership"].(map[string]any)
			if membership["group"] != "recovery" || membership["order"] != float64(30) {
				t.Fatalf("charter_membership = %#v", membership)
			}
			requireFindingCode(t, envelope.State["focus"].(map[string]any)["validation"].(map[string]any)["findings"].([]any), "PRIMARY_DOC_FRONTMATTER_MISMATCH")
			if _, err := os.Stat(filepath.Join(repoRoot, ".specs", "runtime", "new-protocol.yaml")); !os.IsNotExist(err) {
				t.Fatalf("expected no persisted tracking file, got err=%v", err)
			}
			if _, err := os.Stat(filepath.Join(repoRoot, "runtime", "src", "domain", "new_protocol", "SPEC.md")); !os.IsNotExist(err) {
				t.Fatalf("expected no persisted design doc, got err=%v", err)
			}
			charterBytes, err := os.ReadFile(filepath.Join(repoRoot, ".specs", "runtime", "CHARTER.yaml"))
			if err != nil {
				t.Fatalf("read charter: %v", err)
			}
			if strings.Contains(string(charterBytes), "slug: new-protocol") {
				t.Fatalf("charter should not persist new entry:\n%s", charterBytes)
			}
		})
	})
}

func TestFocusedWriteCommandsEndToEnd(t *testing.T) {
	t.Run("focused writes reject orphaned or invalid charter membership", func(t *testing.T) {
		cases := []struct {
			name     string
			mutate   func(t *testing.T, repoRoot string)
			wantText string
		}{
			{
				name: "missing charter file",
				mutate: func(t *testing.T, repoRoot string) {
					if err := os.Remove(filepath.Join(repoRoot, ".specs", "runtime", "CHARTER.yaml")); err != nil {
						t.Fatalf("remove charter: %v", err)
					}
				},
				wantText: "charter file does not exist",
			},
			{
				name: "charter missing spec entry",
				mutate: func(t *testing.T, repoRoot string) {
					replaceInFile(t, filepath.Join(repoRoot, ".specs", "runtime", "CHARTER.yaml"), "specs:\n  - slug: session-lifecycle\n    group: recovery\n    order: 20\n    depends_on: []\n    notes: Session FSM and cleanup behavior\n", "specs: []\n")
				},
				wantText: "does not list spec",
			},
			{
				name: "invalid charter file",
				mutate: func(t *testing.T, repoRoot string) {
					replaceInFile(t, filepath.Join(repoRoot, ".specs", "runtime", "CHARTER.yaml"), "title: Runtime System", "title: \"\"")
				},
				wantText: "title is required",
			},
		}

		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				repoRoot := copyFixtureRepoWithRegistry(t, "ready-spec")
				tc.mutate(t, repoRoot)

				withWorkingDir(t, repoRoot, func() {
					stdout, stderr, exitCode := executeCLIWithInput("current: Current gap\ntarget: Target gap\nnotes: Explicitly tracked\n", "delta", "add", "runtime:session-lifecycle", "--intent", "add", "--area", "Heartbeat timeout")
					if exitCode == 0 {
						t.Fatalf("expected failure, stdout=%q", stdout)
					}

					envelope := requireFailureEnvelope(t, stdout, stderr)
					if envelope.Error == nil || envelope.Error.Code != "VALIDATION_FAILED" {
						t.Fatalf("unexpected error %#v", envelope)
					}

					requireSpecStateShape(t, envelope.State, "ready")
					if envelope.State["tracking_file"] != ".specs/runtime/session-lifecycle.yaml" {
						t.Fatalf("tracking_file = %#v", envelope.State["tracking_file"])
					}
					if envelope.State["charter_membership"] != nil {
						t.Fatalf("charter_membership = %#v, want null", envelope.State["charter_membership"])
					}

					findings := requireValidationFailure(t, envelope.State)
					if len(findings) == 0 {
						t.Fatal("expected validation findings")
					}
					first := findings[0].(map[string]any)
					requireValidationCatalogCode(t, first["code"])
					if !strings.Contains(first["message"].(string), tc.wantText) {
						t.Fatalf("unexpected finding %#v", first)
					}

					next := requireNextAction(t, envelope.Next, 0, "repair_charter_file")
					if next["kind"] != "edit_file" || next["path"] != ".specs/runtime/CHARTER.yaml" {
						t.Fatalf("next = %#v", next)
					}
				})
			})
		}
	})

	t.Run("delta add moves verified to ready", func(t *testing.T) {
		repoRoot := copyFixtureRepoWithRegistry(t, "verified-spec")
		initGitRepoAtDate(t, repoRoot, "2026-03-28T12:00:00Z")

		withWorkingDir(t, repoRoot, func() {
			stdout, stderr, exitCode := executeCLIWithInput("current: Current gap\ntarget: Target gap\nnotes: Explicitly tracked\n", "delta", "add", "runtime:session-lifecycle", "--intent", "add", "--area", "Heartbeat timeout")
			if exitCode != 0 {
				t.Fatalf("exit code = %d, stderr=%s", exitCode, stderr)
			}

			var envelope struct {
				State struct {
					Status            string `json:"status"`
					TrackingFile      string `json:"tracking_file"`
					CharterMembership struct {
						Group      string   `json:"group"`
						GroupTitle string   `json:"group_title"`
						GroupOrder int      `json:"group_order"`
						Order      int      `json:"order"`
						DependsOn  []string `json:"depends_on"`
						Notes      string   `json:"notes"`
					} `json:"charter_membership"`
					Validation struct {
						Valid    bool  `json:"valid"`
						Findings []any `json:"findings"`
					} `json:"validation"`
				} `json:"state"`
				Focus struct {
					Delta struct {
						ID     string `json:"id"`
						Status string `json:"status"`
					} `json:"delta"`
				} `json:"focus"`
				Result struct {
					Allocation struct {
						PreviousMax int    `json:"previous_max"`
						Allocated   string `json:"allocated"`
					} `json:"allocation"`
					Delta struct {
						ID      string `json:"id"`
						Area    string `json:"area"`
						Current string `json:"current"`
						Target  string `json:"target"`
						Notes   string `json:"notes"`
					} `json:"delta"`
				} `json:"result"`
				Next testNextMaps `json:"next"`
			}
			mustUnmarshalJSON(t, stdout, &envelope)
			if envelope.State.Status != "ready" || envelope.Result.Delta.ID != "D-002" || envelope.Focus.Delta.ID != "D-002" || envelope.Focus.Delta.Status != "open" {
				t.Fatalf("unexpected response %#v", envelope)
			}
			if envelope.Result.Delta.Area != "Heartbeat timeout" || envelope.Result.Delta.Current != "Current gap" || envelope.Result.Delta.Target != "Target gap" || envelope.Result.Delta.Notes != "Explicitly tracked" {
				t.Fatalf("delta payload = %#v", envelope.Result.Delta)
			}
			if envelope.State.TrackingFile != ".specs/runtime/session-lifecycle.yaml" {
				t.Fatalf("tracking_file = %q", envelope.State.TrackingFile)
			}
			if envelope.State.CharterMembership.Group != "recovery" || envelope.State.CharterMembership.GroupTitle != "Recovery and Cleanup" || envelope.State.CharterMembership.GroupOrder != 20 || envelope.State.CharterMembership.Order != 20 || len(envelope.State.CharterMembership.DependsOn) != 0 || envelope.State.CharterMembership.Notes != "Session FSM and cleanup behavior" {
				t.Fatalf("charter_membership = %#v", envelope.State.CharterMembership)
			}
			if !envelope.State.Validation.Valid || len(envelope.State.Validation.Findings) != 0 {
				t.Fatalf("validation = %#v", envelope.State.Validation)
			}
			if envelope.Result.Allocation.PreviousMax != 1 || envelope.Result.Allocation.Allocated != "D-002" {
				t.Fatalf("allocation = %#v", envelope.Result.Allocation)
			}
			if len(envelope.Next) != 2 || envelope.Next[0]["action"] != "write_spec_section" || envelope.Next[1]["action"] != "add_requirement" {
				t.Fatalf("next = %#v", envelope.Next)
			}
			template := requireTemplate(t, envelope.Next[1])
			if stdinFormat := template["stdin_format"]; stdinFormat != "gherkin" {
				t.Fatalf("stdin_format = %#v", stdinFormat)
			}
			if argv := stringSliceFromAny(t, template["argv"]); strings.Join(argv, " ") != "specctl req add runtime:session-lifecycle --delta D-002" {
				t.Fatalf("template.argv = %v", argv)
			}
			if required := requiredFieldNames(t, template["required_fields"]); strings.Join(required, ",") != "gherkin_requirement" {
				t.Fatalf("required_fields = %v", required)
			}
		})
	})

	t.Run("delta add seeds requirement template from the target charter", func(t *testing.T) {
		repoRoot := uiReadySpecRepo(t)

		withWorkingDir(t, repoRoot, func() {
			stdout, stderr, exitCode := executeCLIWithInput("current: Current state\ntarget: Target state\nnotes: Explicitly tracked\n", "delta", "add", "ui:thread-lifecycle", "--intent", "add", "--area", "Workspace tabs")
			if exitCode != 0 {
				t.Fatalf("exit code = %d, stderr=%s stdout=%s", exitCode, stderr, stdout)
			}
			if stderr != "" {
				t.Fatalf("expected empty stderr, got %q", stderr)
			}

			var envelope struct {
				Next testNextMaps `json:"next"`
			}
			mustUnmarshalJSON(t, stdout, &envelope)
			// Step 0 is write_spec_section, step 1 is add_requirement
			template := requireTemplate(t, envelope.Next[1])
			if stdinTemplate := template["stdin_template"]; stdinTemplate != "@ui\nFeature: <feature>\n" {
				t.Fatalf("stdin_template = %#v", stdinTemplate)
			}
		})
	})

	t.Run("delta add accepts explicitly empty notes", func(t *testing.T) {
		repoRoot := copyFixtureRepoWithRegistry(t, "ready-spec")

		withWorkingDir(t, repoRoot, func() {
			stdout, stderr, exitCode := executeCLIWithInput("current: Current gap\ntarget: Target gap\nnotes: \"\"\n", "delta", "add", "runtime:session-lifecycle", "--intent", "add", "--area", "Empty notes")
			if exitCode != 0 {
				t.Fatalf("exit code = %d, stderr=%s", exitCode, stderr)
			}

			var envelope struct {
				Result struct {
					Delta struct {
						Notes string `json:"notes"`
					} `json:"delta"`
				} `json:"result"`
			}
			mustUnmarshalJSON(t, stdout, &envelope)
			if envelope.Result.Delta.Notes != "" {
				t.Fatalf("notes = %q", envelope.Result.Delta.Notes)
			}
		})
	})

	t.Run("delta add blank required stdin fields use missing_fields focus", func(t *testing.T) {
		repoRoot := copyFixtureRepoWithRegistry(t, "ready-spec")

		withWorkingDir(t, repoRoot, func() {
			stdout, stderr, exitCode := executeCLIWithInput("current: \"   \"\ntarget: Target gap\nnotes: Explicitly tracked\n", "delta", "add", "runtime:session-lifecycle", "--intent", "add", "--area", "Blank current")
			if exitCode == 0 {
				t.Fatalf("expected failure, stdout=%q stderr=%q", stdout, stderr)
			}

			envelope := requireFailureEnvelope(t, stdout, stderr)
			if envelope.Error == nil || envelope.Error.Code != "INVALID_INPUT" {
				t.Fatalf("unexpected envelope %#v", envelope)
			}
			focus := envelope.State["focus"].(map[string]any)["input"].(map[string]any)
			if got := strings.Join(stringSliceFromAny(t, focus["missing_fields"]), ","); got != "current" {
				t.Fatalf("missing_fields = %#v", focus["missing_fields"])
			}
			if _, exists := focus["invalid_fields"]; exists {
				t.Fatalf("unexpected invalid_fields = %#v", focus["invalid_fields"])
			}
		})
	})

	t.Run("req add derives title and tags and moves ready to active", func(t *testing.T) {
		repoRoot := copyFixtureRepoWithRegistry(t, "ready-spec")
		initGitRepoAtDate(t, repoRoot, "2026-03-28T12:00:00Z")
		gherkin := "@runtime @e2e\nFeature: Compensation stage 4 failure cleanup\n\n  Scenario: Cleanup runs after stage 4 failure\n    Given stage 4 fails during compensation\n    When recovery completes\n    Then cleanup steps run in documented order\n"

		withWorkingDir(t, repoRoot, func() {
			stdout, stderr, exitCode := executeCLIWithInput(gherkin, "req", "add", "runtime:session-lifecycle", "--delta", "D-001")
			if exitCode != 0 {
				t.Fatalf("exit code = %d, stderr=%s", exitCode, stderr)
			}

			var envelope struct {
				State struct {
					Status            string `json:"status"`
					TrackingFile      string `json:"tracking_file"`
					CharterMembership struct {
						Group string `json:"group"`
					} `json:"charter_membership"`
					Validation struct {
						Valid    bool  `json:"valid"`
						Findings []any `json:"findings"`
					} `json:"validation"`
				} `json:"state"`
				Focus struct {
					Requirement struct {
						ID string `json:"id"`
					} `json:"requirement"`
				} `json:"focus"`
				Result struct {
					Allocation struct {
						PreviousMax int    `json:"previous_max"`
						Allocated   string `json:"allocated"`
					} `json:"allocation"`
					Requirement struct {
						Title string   `json:"title"`
						Tags  []string `json:"tags"`
					} `json:"requirement"`
				} `json:"result"`
				Next testNextMaps `json:"next"`
			}
			mustUnmarshalJSON(t, stdout, &envelope)
			if envelope.State.Status != "active" {
				t.Fatalf("status = %q", envelope.State.Status)
			}
			if envelope.State.TrackingFile != ".specs/runtime/session-lifecycle.yaml" || envelope.State.CharterMembership.Group != "recovery" {
				t.Fatalf("unexpected canonical state %#v", envelope.State)
			}
			if !envelope.State.Validation.Valid || len(envelope.State.Validation.Findings) != 0 {
				t.Fatalf("validation = %#v", envelope.State.Validation)
			}
			if envelope.Focus.Requirement.ID != "REQ-001" {
				t.Fatalf("focus.requirement = %#v", envelope.Focus.Requirement)
			}
			if envelope.Result.Requirement.Title != "Compensation stage 4 failure cleanup" {
				t.Fatalf("title = %q", envelope.Result.Requirement.Title)
			}
			if strings.Join(envelope.Result.Requirement.Tags, ",") != "runtime,e2e" {
				t.Fatalf("tags = %v", envelope.Result.Requirement.Tags)
			}
			if envelope.Result.Allocation.PreviousMax != 0 || envelope.Result.Allocation.Allocated != "REQ-001" {
				t.Fatalf("allocation = %#v", envelope.Result.Allocation)
			}
			if len(envelope.Next) != 2 || envelope.Next[0]["action"] != "implement_and_test" || envelope.Next[1]["action"] != "verify_requirement" {
				t.Fatalf("next = %#v", envelope.Next)
			}
			context := envelope.Next[0]["context"].(map[string]any)
			if context["verification_level"] != "e2e" {
				t.Fatalf("implement context = %#v", context)
			}
			template := requireTemplate(t, envelope.Next[1])
			argv := stringSliceFromAny(t, template["argv"])
			if strings.Join(argv, " ") != "specctl req verify runtime:session-lifecycle REQ-001 --test-file runtime/tests/e2e/journeys/test_compensation_stage_4_failure_cleanup.py" {
				t.Fatalf("next.argv = %v", argv)
			}
		})
	})

	t.Run("req add for manual verification suggests req verify without --test-file", func(t *testing.T) {
		repoRoot := copyFixtureRepoWithRegistry(t, "ready-spec")
		replaceInFile(t, filepath.Join(repoRoot, "runtime", "src", "domain", "session_execution", "SPEC.md"), "@runtime @e2e", "@runtime @manual")
		gherkin := "@runtime @manual\nFeature: Compensation stage 4 failure cleanup\n\n  Scenario: Cleanup runs after stage 4 failure\n    Given stage 4 fails during compensation\n    When recovery completes\n    Then cleanup steps run in documented order\n"

		withWorkingDir(t, repoRoot, func() {
			stdout, stderr, exitCode := executeCLIWithInput(gherkin, "req", "add", "runtime:session-lifecycle", "--delta", "D-001")
			if exitCode != 0 {
				t.Fatalf("exit code = %d, stderr=%s", exitCode, stderr)
			}

			var envelope struct {
				Next testNextMaps `json:"next"`
			}
			mustUnmarshalJSON(t, stdout, &envelope)

			next := requireNextAction(t, mapsToAny(envelope.Next), 1, "verify_requirement")
			template := requireTemplate(t, next)
			argv := stringSliceFromAny(t, template["argv"])
			if strings.Join(argv, " ") != "specctl req verify runtime:session-lifecycle REQ-001" {
				t.Fatalf("next.argv = %v", argv)
			}
		})
	})

	t.Run("req add on a non-runtime spec suggests a charter-aware verify path", func(t *testing.T) {
		repoRoot := uiReadySpecRepo(t)
		gherkin := "@ui\nFeature: Thread lifecycle\n\n  Scenario: The workspace tab restores a thread\n    Given a user reopens the workspace\n    When the active thread is restored\n    Then the thread tab shows the latest state\n"

		withWorkingDir(t, repoRoot, func() {
			stdout, stderr, exitCode := executeCLIWithInput(gherkin, "req", "add", "ui:thread-lifecycle", "--delta", "D-001")
			if exitCode != 0 {
				t.Fatalf("exit code = %d, stderr=%s stdout=%s", exitCode, stderr, stdout)
			}
			if stderr != "" {
				t.Fatalf("expected empty stderr, got %q", stderr)
			}

			var envelope struct {
				Next testNextMaps `json:"next"`
			}
			mustUnmarshalJSON(t, stdout, &envelope)
			template := requireTemplate(t, envelope.Next[1])
			argv := stringSliceFromAny(t, template["argv"])
			if strings.Join(argv, " ") != "specctl req verify ui:thread-lifecycle REQ-001 --test-file ui/tests/domain/test_thread_lifecycle.py" {
				t.Fatalf("next.argv = %v", argv)
			}
		})
	})

	t.Run("req add rejects malformed gherkin tags as invalid_gherkin_tag", func(t *testing.T) {
		repoRoot := copyFixtureRepoWithRegistry(t, "ready-spec")
		gherkin := "@Runtime\nFeature: Compensation stage 4 failure cleanup\n\n  Scenario: Cleanup runs after stage 4 failure\n    Given stage 4 fails during compensation\n    When recovery completes\n    Then cleanup steps run in documented order\n"

		withWorkingDir(t, repoRoot, func() {
			stdout, stderr, exitCode := executeCLIWithInput(gherkin, "req", "add", "runtime:session-lifecycle", "--delta", "D-001")
			if exitCode == 0 {
				t.Fatalf("expected failure, stdout=%q", stdout)
			}
			envelope := requireFailureEnvelope(t, stdout, stderr)
			if envelope.Error == nil || envelope.Error.Code != "INVALID_GHERKIN_TAG" {
				t.Fatalf("unexpected error %#v", envelope)
			}
		})
	})

	t.Run("req add missing requirement block suggests writing the block", func(t *testing.T) {
		repoRoot := copyFixtureRepoWithRegistry(t, "ready-spec")
		gherkin := "@runtime @e2e\nFeature: Newly documented behavior\n\n  Scenario: New behavior is tracked\n    Given the requirement block is missing from SPEC.md\n    When req add is attempted\n    Then specctl suggests writing the block first\n"

		withWorkingDir(t, repoRoot, func() {
			stdout, stderr, exitCode := executeCLIWithInput(gherkin, "req", "add", "runtime:session-lifecycle", "--delta", "D-001")
			if exitCode == 0 {
				t.Fatalf("expected failure, stdout=%q", stdout)
			}
			envelope := requireFailureEnvelope(t, stdout, stderr)
			if envelope.Error == nil || envelope.Error.Code != "REQUIREMENT_NOT_IN_SPEC" {
				t.Fatalf("unexpected envelope %#v", envelope)
			}
			if envelope.Focus["req_add"].(map[string]any)["reason"] != "requirement_not_in_spec" {
				t.Fatalf("focus = %#v", envelope.Focus)
			}
			next := requireNextAction(t, envelope.Next, 0, "write_requirement_block")
			if next["kind"] != "edit_file" || next["path"] != "runtime/src/domain/session_execution/SPEC.md" {
				t.Fatalf("next = %#v", next)
			}
		})
	})

	t.Run("req replace missing replacement block suggests writing the block", func(t *testing.T) {
		repoRoot := contractChangeDeltaRepo(t)
		gherkin := "@runtime @e2e\nFeature: Replacement behavior absent from doc\n\n  Scenario: The replacement block must be written first\n    Given the replacement requirement is not yet in SPEC.md\n    When req replace is attempted\n    Then specctl suggests writing the replacement block\n"

		withWorkingDir(t, repoRoot, func() {
			stdout, stderr, exitCode := executeCLIWithInput(gherkin, "req", "replace", "runtime:session-lifecycle", "REQ-001", "--delta", "D-002")
			if exitCode == 0 {
				t.Fatalf("expected failure, stdout=%q", stdout)
			}
			envelope := requireFailureEnvelope(t, stdout, stderr)
			if envelope.Error == nil || envelope.Error.Code != "REQUIREMENT_NOT_IN_SPEC" {
				t.Fatalf("unexpected envelope %#v", envelope)
			}
			if envelope.Focus["req_replace"].(map[string]any)["reason"] != "requirement_not_in_spec" {
				t.Fatalf("focus = %#v", envelope.Focus)
			}
			next := requireNextAction(t, envelope.Next, 0, "write_requirement_block")
			if next["kind"] != "edit_file" || next["path"] != "runtime/src/domain/session_execution/SPEC.md" {
				t.Fatalf("next = %#v", next)
			}
		})
	})

	t.Run("delta transition commands persist expected statuses", func(t *testing.T) {
		repoRoot := copyFixtureRepoWithRegistry(t, "ready-spec")
		initGitRepoAtDate(t, repoRoot, "2026-03-28T12:00:00Z")

		withWorkingDir(t, repoRoot, func() {
			stdout, stderr, exitCode := executeCLI("delta", "start", "runtime:session-lifecycle", "D-001")
			if exitCode != 0 {
				t.Fatalf("delta start failed: %s", stderr)
			}
			var startEnvelope struct {
				Result struct {
					Delta struct {
						Status string `json:"status"`
					} `json:"delta"`
				} `json:"result"`
			}
			mustUnmarshalJSON(t, stdout, &startEnvelope)
			if startEnvelope.Result.Delta.Status != "in-progress" {
				t.Fatalf("start status = %q", startEnvelope.Result.Delta.Status)
			}

			stdout, stderr, exitCode = executeCLI("delta", "defer", "runtime:session-lifecycle", "D-001")
			if exitCode != 0 {
				t.Fatalf("delta defer failed: %s", stderr)
			}
			var deferEnvelope struct {
				Result struct {
					Delta struct {
						Status string `json:"status"`
					} `json:"delta"`
				} `json:"result"`
			}
			mustUnmarshalJSON(t, stdout, &deferEnvelope)
			if deferEnvelope.Result.Delta.Status != "deferred" {
				t.Fatalf("defer status = %q", deferEnvelope.Result.Delta.Status)
			}

			stdout, stderr, exitCode = executeCLI("delta", "resume", "runtime:session-lifecycle", "D-001")
			if exitCode != 0 {
				t.Fatalf("delta resume failed: %s", stderr)
			}
			var resumeEnvelope struct {
				State struct {
					Validation struct {
						Valid bool `json:"valid"`
					} `json:"validation"`
				} `json:"state"`
				Result struct {
					Delta struct {
						Status string `json:"status"`
					} `json:"delta"`
				} `json:"result"`
				Next testNextMaps `json:"next"`
			}
			mustUnmarshalJSON(t, stdout, &resumeEnvelope)
			if resumeEnvelope.Result.Delta.Status != "open" {
				t.Fatalf("resume status = %q", resumeEnvelope.Result.Delta.Status)
			}
			if !resumeEnvelope.State.Validation.Valid {
				t.Fatalf("validation = %#v", resumeEnvelope.State.Validation)
			}
			if next := mapsToAny(resumeEnvelope.Next); len(next) != 0 {
				t.Fatalf("next = %#v", next)
			}
		})
	})

	t.Run("delta close reports unverified requirements", func(t *testing.T) {
		repoRoot := copyFixtureRepoWithRegistry(t, "active-spec")

		withWorkingDir(t, repoRoot, func() {
			stdout, stderr, exitCode := executeCLI("delta", "close", "runtime:session-lifecycle", "D-001")
			if exitCode == 0 {
				t.Fatalf("expected failure, stdout=%q", stdout)
			}
			envelope := requireFailureEnvelope(t, stdout, stderr)
			if envelope.Error == nil || envelope.Error.Code != "UNVERIFIED_REQUIREMENTS" {
				t.Fatalf("unexpected error %#v", envelope)
			}
			requireSpecStateShape(t, envelope.State, "active")
			focus := envelope.State["focus"].(map[string]any)
			delta := focus["delta"].(map[string]any)
			if delta["id"] != "D-001" || delta["status"] != "in-progress" || len(delta) != 2 {
				t.Fatalf("delta focus = %#v", delta)
			}
			blocking := focus["blocking_requirements"].([]any)
			if len(blocking) != 1 {
				t.Fatalf("blocking requirements = %#v", blocking)
			}
			next := envelope.Next[0].(map[string]any)
			template := requireTemplate(t, next)
			argv := stringSliceFromAny(t, template["argv"])
			if next["action"] != "verify_requirement" || !strings.HasPrefix(argv[len(argv)-1], "runtime/tests/e2e/journeys/") {
				t.Fatalf("next = %#v", next)
			}
		})
	})

	t.Run("delta close on a non-runtime e2e requirement suggests the charter e2e path", func(t *testing.T) {
		repoRoot := uiActiveSpecRepo(t)

		withWorkingDir(t, repoRoot, func() {
			stdout, stderr, exitCode := executeCLI("delta", "close", "ui:thread-lifecycle", "D-001")
			if exitCode == 0 {
				t.Fatalf("expected failure, stdout=%q stderr=%q", stdout, stderr)
			}

			envelope := requireFailureEnvelope(t, stdout, stderr)
			if envelope.Error == nil || envelope.Error.Code != "UNVERIFIED_REQUIREMENTS" {
				t.Fatalf("unexpected envelope %#v", envelope)
			}
			next := envelope.Next[0].(map[string]any)
			argv := stringSliceFromAny(t, requireTemplate(t, next)["argv"])
			if strings.Join(argv, " ") != "specctl req verify ui:thread-lifecycle REQ-001 --test-file ui/tests/e2e/journeys/test_thread_lifecycle.py" {
				t.Fatalf("next.argv = %v", argv)
			}
		})
	})

	t.Run("delta close on the last live delta returns verified", func(t *testing.T) {
		repoRoot := copyFixtureRepoWithRegistry(t, "verified-spec")
		replaceInFile(t, filepath.Join(repoRoot, ".specs", "runtime", "session-lifecycle.yaml"), "status: closed", "status: in-progress")
		replaceInFile(t, filepath.Join(repoRoot, ".specs", "runtime", "session-lifecycle.yaml"), "status: verified", "status: active")

		withWorkingDir(t, repoRoot, func() {
			stdout, stderr, exitCode := executeCLI("delta", "close", "runtime:session-lifecycle", "D-001")
			if exitCode != 0 {
				t.Fatalf("exit code = %d, stderr=%s", exitCode, stderr)
			}
			var envelope struct {
				State struct {
					Status string `json:"status"`
				} `json:"state"`
				Focus struct {
					Delta struct {
						Status string `json:"status"`
					} `json:"delta"`
				} `json:"focus"`
				Next testNextMaps `json:"next"`
			}
			mustUnmarshalJSON(t, stdout, &envelope)
			if envelope.State.Status != "verified" {
				t.Fatalf("status = %q", envelope.State.Status)
			}
			if envelope.Focus.Delta.Status != "closed" {
				t.Fatalf("focus.delta = %#v", envelope.Focus.Delta)
			}
			next := requireNextAction(t, mapsToAny(envelope.Next), 0, "rev_bump")
			template := requireTemplate(t, next)
			if template["stdin_format"] != "text" || template["stdin_template"] != "<summary>\n" {
				t.Fatalf("template = %#v", template)
			}
			if argv := stringSliceFromAny(t, template["argv"]); strings.Join(argv, " ") != "specctl rev bump runtime:session-lifecycle --checkpoint HEAD" {
				t.Fatalf("template.argv = %v", argv)
			}
			if required := requiredFieldNames(t, template["required_fields"]); strings.Join(required, ",") != "summary" {
				t.Fatalf("required_fields = %v", required)
			}
		})
	})

	t.Run("delta add on malformed IDs returns validation failed without partial writes", func(t *testing.T) {
		repoRoot := copyFixtureRepoWithRegistry(t, "ready-spec")
		replaceInFile(t, filepath.Join(repoRoot, ".specs", "runtime", "session-lifecycle.yaml"), "id: D-001", "id: D-003")
		before, err := os.ReadFile(filepath.Join(repoRoot, ".specs", "runtime", "session-lifecycle.yaml"))
		if err != nil {
			t.Fatalf("read tracking file: %v", err)
		}

		withWorkingDir(t, repoRoot, func() {
			stdout, stderr, exitCode := executeCLIWithInput("current: Current gap\ntarget: Target gap\nnotes: Explicitly tracked\n", "delta", "add", "runtime:session-lifecycle", "--intent", "add", "--area", "Broken allocation")
			if exitCode == 0 {
				t.Fatalf("expected failure, stdout=%q", stdout)
			}
			envelope := requireFailureEnvelope(t, stdout, stderr)
			if envelope.Error == nil || envelope.Error.Code != "VALIDATION_FAILED" {
				t.Fatalf("unexpected error %#v", envelope)
			}
			requireSpecStateShape(t, envelope.State, "ready")
			if envelope.State["charter_membership"] == nil {
				t.Fatalf("expected charter_membership in %#v", envelope.State)
			}
			findings := requireValidationFailure(t, envelope.State)
			requireValidationCatalogCode(t, findings[0].(map[string]any)["code"])
			if findings[0].(map[string]any)["target"] != "deltas" {
				t.Fatalf("unexpected finding target %#v", findings[0])
			}
			focus := envelope.State["focus"].(map[string]any)
			validation := focus["validation"].(map[string]any)
			if len(validation["findings"].([]any)) != len(findings) {
				t.Fatalf("focus.validation = %#v", validation)
			}

			after, err := os.ReadFile(filepath.Join(repoRoot, ".specs", "runtime", "session-lifecycle.yaml"))
			if err != nil {
				t.Fatalf("read tracking file: %v", err)
			}
			if string(before) != string(after) {
				t.Fatal("tracking file changed on validation failure")
			}
		})
	})

	t.Run("req verify enforces files for non-manual requirements", func(t *testing.T) {
		repoRoot := copyFixtureRepoWithRegistry(t, "active-spec")

		withWorkingDir(t, repoRoot, func() {
			stdout, stderr, exitCode := executeCLI("req", "verify", "runtime:session-lifecycle", "REQ-001")
			if exitCode == 0 {
				t.Fatalf("expected failure, stdout=%q", stdout)
			}
			envelope := requireFailureEnvelope(t, stdout, stderr)
			if envelope.Error == nil || envelope.Error.Code != "TEST_FILES_REQUIRED" {
				t.Fatalf("unexpected error %#v", envelope)
			}
		})
	})

	t.Run("req verify malformed test-file path reuses missing-file error surface", func(t *testing.T) {
		repoRoot := copyFixtureRepoWithRegistry(t, "active-spec")

		withWorkingDir(t, repoRoot, func() {
			stdout, stderr, exitCode := executeCLI("req", "verify", "runtime:session-lifecycle", "REQ-001", "--test-file", "../outside.py")
			if exitCode == 0 {
				t.Fatalf("expected failure, stdout=%q stderr=%q", stdout, stderr)
			}
			envelope := requireFailureEnvelope(t, stdout, stderr)
			if envelope.Error == nil || envelope.Error.Code != "TEST_FILE_NOT_FOUND" {
				t.Fatalf("unexpected error %#v", envelope)
			}
			focus := envelope.State["focus"].(map[string]any)
			if got := strings.Join(stringSliceFromAny(t, focus["invalid_paths"]), ","); got != "../outside.py" {
				t.Fatalf("invalid_paths = %#v", focus["invalid_paths"])
			}
		})
	})

	t.Run("req verify allows manual verification without files", func(t *testing.T) {
		repoRoot := manualRequirementRepo(t)

		withWorkingDir(t, repoRoot, func() {
			stdout, stderr, exitCode := executeCLI("req", "verify", "runtime:session-lifecycle", "REQ-001")
			if exitCode != 0 {
				t.Fatalf("exit code = %d, stderr=%s", exitCode, stderr)
			}
			var envelope struct {
				State struct{} `json:"state"`
				Focus struct {
					Requirement struct {
						Verification string   `json:"verification"`
						TestFiles    []string `json:"test_files"`
					} `json:"requirement"`
				} `json:"focus"`
				Result struct {
					Requirement struct {
						Verification string   `json:"verification"`
						TestFiles    []string `json:"test_files"`
					} `json:"requirement"`
				} `json:"result"`
				Next testNextMaps `json:"next"`
			}
			mustUnmarshalJSON(t, stdout, &envelope)
			if envelope.Result.Requirement.Verification != "verified" || len(envelope.Result.Requirement.TestFiles) != 0 {
				t.Fatalf("expected empty test_files, got %v", envelope.Result.Requirement.TestFiles)
			}
			if envelope.Focus.Requirement.Verification != "verified" || len(envelope.Focus.Requirement.TestFiles) != 0 {
				t.Fatalf("focus.requirement = %#v", envelope.Focus.Requirement)
			}
			next := requireNextAction(t, mapsToAny(envelope.Next), 0, "close_delta")
			if argv := stringSliceFromAny(t, requireTemplate(t, next)["argv"]); strings.Join(argv, " ") != "specctl delta close runtime:session-lifecycle D-001" {
				t.Fatalf("next.argv = %v", argv)
			}
		})
	})

	t.Run("req verify returns delta close next action after normalized file persistence", func(t *testing.T) {
		repoRoot := copyFixtureRepoWithRegistry(t, "active-spec")
		testPath := filepath.Join(repoRoot, "runtime", "tests", "e2e", "journeys")
		if err := os.MkdirAll(testPath, 0755); err != nil {
			t.Fatalf("mkdir test dir: %v", err)
		}
		if err := os.WriteFile(filepath.Join(testPath, "test_compensation_cleanup.py"), []byte("def test_cleanup():\n    assert True\n"), 0644); err != nil {
			t.Fatalf("write test file: %v", err)
		}

		withWorkingDir(t, repoRoot, func() {
			stdout, stderr, exitCode := executeCLI("req", "verify", "runtime:session-lifecycle", "REQ-001", "--test-file", "./runtime/tests/e2e/journeys/../journeys/test_compensation_cleanup.py")
			if exitCode != 0 {
				t.Fatalf("exit code = %d, stderr=%s", exitCode, stderr)
			}

			var envelope struct {
				State struct{} `json:"state"`
				Focus struct {
					Requirement struct {
						Verification string   `json:"verification"`
						TestFiles    []string `json:"test_files"`
					} `json:"requirement"`
				} `json:"focus"`
				Next testNextMaps `json:"next"`
			}
			mustUnmarshalJSON(t, stdout, &envelope)
			if envelope.Focus.Requirement.Verification != "verified" || strings.Join(envelope.Focus.Requirement.TestFiles, ",") != "runtime/tests/e2e/journeys/test_compensation_cleanup.py" {
				t.Fatalf("focus.requirement = %#v", envelope.Focus.Requirement)
			}
			next := requireNextAction(t, mapsToAny(envelope.Next), 0, "close_delta")
			if argv := stringSliceFromAny(t, requireTemplate(t, next)["argv"]); strings.Join(argv, " ") != "specctl delta close runtime:session-lifecycle D-001" {
				t.Fatalf("next.argv = %v", argv)
			}
		})
	})

	t.Run("req verify is idempotent for the same normalized file set", func(t *testing.T) {
		repoRoot := copyFixtureRepoWithRegistry(t, "verified-spec")
		path := filepath.Join(repoRoot, ".specs", "runtime", "session-lifecycle.yaml")
		before, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read tracking file: %v", err)
		}

		withWorkingDir(t, repoRoot, func() {
			stdout, stderr, exitCode := executeCLI("req", "verify", "runtime:session-lifecycle", "REQ-001", "--test-file", "./runtime/tests/domain/../domain/test_compensation_cleanup.py")
			if exitCode != 0 {
				t.Fatalf("exit code = %d, stderr=%s", exitCode, stderr)
			}
			var envelope struct {
				Result struct {
					Requirement struct {
						Verification string   `json:"verification"`
						TestFiles    []string `json:"test_files"`
					} `json:"requirement"`
				} `json:"result"`
			}
			mustUnmarshalJSON(t, stdout, &envelope)
			if envelope.Result.Requirement.Verification != "verified" || strings.Join(envelope.Result.Requirement.TestFiles, ",") != "runtime/tests/domain/test_compensation_cleanup.py" {
				t.Fatalf("unexpected result %#v", envelope.Result.Requirement)
			}

			after, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("read tracking file: %v", err)
			}
			if string(before) != string(after) {
				t.Fatal("tracking file changed on idempotent verify")
			}
		})
	})

	t.Run("req verify suppresses delta close suggestions for already-closed deltas", func(t *testing.T) {
		repoRoot := copyFixtureRepoWithRegistry(t, "verified-spec")

		withWorkingDir(t, repoRoot, func() {
			stdout, stderr, exitCode := executeCLI("req", "verify", "runtime:session-lifecycle", "REQ-001", "--test-file", "runtime/tests/domain/test_compensation_cleanup.py")
			if exitCode != 0 {
				t.Fatalf("exit code = %d, stderr=%s", exitCode, stderr)
			}
			var envelope struct {
				Next testNext `json:"next"`
			}
			mustUnmarshalJSON(t, stdout, &envelope)
			if len(envelope.Next) != 0 {
				t.Fatalf("expected no next actions, got %#v", envelope.Next)
			}
		})
	})

	t.Run("rev bump rejects non-verified specs", func(t *testing.T) {
		repoRoot := copyFixtureRepoWithRegistry(t, "active-spec")

		withWorkingDir(t, repoRoot, func() {
			stdout, stderr, exitCode := executeCLIWithInput("No-op summary\n", "rev", "bump", "runtime:session-lifecycle", "--checkpoint", "HEAD")
			if exitCode == 0 {
				t.Fatalf("expected failure, stdout=%q", stdout)
			}
			envelope := requireFailureEnvelope(t, stdout, stderr)
			if envelope.Error == nil || envelope.Error.Code != "INVALID_INPUT" {
				t.Fatalf("unexpected error %#v", envelope)
			}
			focus := envelope.State["focus"].(map[string]any)
			revBump := focus["rev_bump"].(map[string]any)
			if revBump["reason"] != "status_not_verified" {
				t.Fatalf("unexpected reason %#v", revBump)
			}
		})
	})

	t.Run("rev bump requires checkpoint with rev_bump focus reason", func(t *testing.T) {
		repoRoot := copyFixtureRepoWithRegistry(t, "verified-spec")

		withWorkingDir(t, repoRoot, func() {
			stdout, stderr, exitCode := executeCLIWithInput("Summary\n", "rev", "bump", "runtime:session-lifecycle")
			if exitCode == 0 {
				t.Fatalf("expected failure, stdout=%q", stdout)
			}
			envelope := requireFailureEnvelope(t, stdout, stderr)
			if envelope.Error == nil || envelope.Error.Code != "INVALID_INPUT" {
				t.Fatalf("unexpected error %#v", envelope)
			}
			focus := envelope.State["focus"].(map[string]any)
			if _, exists := focus["input"]; exists {
				t.Fatalf("focus = %#v", focus)
			}
			revBump := focus["rev_bump"].(map[string]any)
			if revBump["reason"] != "missing_checkpoint" {
				t.Fatalf("unexpected reason %#v", revBump)
			}
		})
	})

	t.Run("rev bump requires summary with rev_bump focus reason", func(t *testing.T) {
		repoRoot := copyFixtureRepoWithRegistry(t, "verified-spec")

		withWorkingDir(t, repoRoot, func() {
			stdout, stderr, exitCode := executeCLI("rev", "bump", "runtime:session-lifecycle", "--checkpoint", "HEAD")
			if exitCode == 0 {
				t.Fatalf("expected failure, stdout=%q", stdout)
			}
			envelope := requireFailureEnvelope(t, stdout, stderr)
			if envelope.Error == nil || envelope.Error.Code != "INVALID_INPUT" {
				t.Fatalf("unexpected error %#v", envelope)
			}
			focus := envelope.State["focus"].(map[string]any)
			if _, exists := focus["input"]; exists {
				t.Fatalf("focus = %#v", focus)
			}
			revBump := focus["rev_bump"].(map[string]any)
			if revBump["reason"] != "missing_summary" {
				t.Fatalf("unexpected reason %#v", revBump)
			}
		})
	})

	t.Run("rev bump rejects missing semantic changes", func(t *testing.T) {
		repoRoot := copyFixtureRepoWithRegistry(t, "verified-spec")
		headSHA := initGitRepo(t, repoRoot)
		replaceInFile(t, filepath.Join(repoRoot, ".specs", "runtime", "session-lifecycle.yaml"), "checkpoint: a1b2c3f", "checkpoint: "+headSHA)

		withWorkingDir(t, repoRoot, func() {
			stdout, stderr, exitCode := executeCLIWithInput("No-op summary\n", "rev", "bump", "runtime:session-lifecycle", "--checkpoint", "HEAD")
			if exitCode == 0 {
				t.Fatalf("expected failure, stdout=%q", stdout)
			}
			envelope := requireFailureEnvelope(t, stdout, stderr)
			if envelope.Error == nil || envelope.Error.Code != "INVALID_INPUT" {
				t.Fatalf("unexpected error %#v", envelope)
			}
			focus := envelope.State["focus"].(map[string]any)
			revBump := focus["rev_bump"].(map[string]any)
			if revBump["reason"] != "no_semantic_changes" {
				t.Fatalf("unexpected reason %#v", revBump)
			}
		})
	})

	t.Run("rev bump succeeds with corrupted stored checkpoint when unbumped deltas exist", func(t *testing.T) {
		repoRoot := copyFixtureRepoWithRegistry(t, "verified-spec")
		// Add an unbumped closed delta (D-002 not in changelog) and corrupt stored checkpoint
		trackingPath := filepath.Join(repoRoot, ".specs", "runtime", "session-lifecycle.yaml")
		replaceInFile(t, trackingPath, "checkpoint: a1b2c3f", "checkpoint: deadbee")
		replaceInFile(t, trackingPath, "    notes: Multi-agent implementation split between runtime and workflow work\n", "    notes: Multi-agent implementation split between runtime and workflow work\n  - id: D-002\n    area: Session timeout\n    intent: repair\n    status: closed\n    origin_checkpoint: deadbee\n    current: Requirement needs re-verification\n    target: Re-verify after repair\n    notes: Unbumped delta for test\n    affects_requirements:\n      - REQ-001\n")
		headSHA := initGitRepo(t, repoRoot)

		withWorkingDir(t, repoRoot, func() {
			stdout, stderr, exitCode := executeCLIWithInput("Bump despite corrupted stored checkpoint.\n", "rev", "bump", "runtime:session-lifecycle", "--checkpoint", "HEAD")
			if exitCode != 0 {
				t.Fatalf("exit code = %d, stderr=%s stdout=%s", exitCode, stderr, stdout)
			}
			if stderr != "" {
				t.Fatalf("stderr = %q", stderr)
			}
			var envelope struct {
				Result struct {
					Rev        int    `json:"rev"`
					Checkpoint string `json:"checkpoint"`
				} `json:"result"`
			}
			mustUnmarshalJSON(t, stdout, &envelope)
			if envelope.Result.Rev != 4 {
				t.Fatalf("expected rev 4, got %d", envelope.Result.Rev)
			}
			if envelope.Result.Checkpoint != headSHA {
				t.Fatalf("expected checkpoint %s, got %s", headSHA, envelope.Result.Checkpoint)
			}
		})
	})

	t.Run("rev bump records HEAD as checkpoint and advances last_verified_at", func(t *testing.T) {
		repoRoot := copyFixtureRepoWithRegistry(t, "verified-spec")
		// Add an unbumped closed delta (D-002 not in changelog) so the rev bump gate passes
		trackingPath := filepath.Join(repoRoot, ".specs", "runtime", "session-lifecycle.yaml")
		replaceInFile(t, trackingPath, "    notes: Multi-agent implementation split between runtime and workflow work\n", "    notes: Multi-agent implementation split between runtime and workflow work\n  - id: D-002\n    area: Session timeout\n    intent: repair\n    status: closed\n    origin_checkpoint: a1b2c3f\n    current: Requirement needs re-verification\n    target: Re-verify after repair\n    notes: Unbumped delta for rev bump test\n    affects_requirements:\n      - REQ-001\n")
		writeCLIAdjacentTestFile(t, filepath.Join(repoRoot, "runtime", "src", "domain", "session_execution", "SPEC.md"), []byte("---\nspec: session-lifecycle\ncharter: runtime\n---\n# Session Lifecycle\n\n## Requirement: Compensation stage 4 failure cleanup\n\n```gherkin requirement\n@runtime @e2e\nFeature: Compensation stage 4 failure cleanup\n```\n\n### Scenarios\n\n```gherkin scenario\nScenario: Cleanup runs after stage 4 failure\n  Given stage 4 fails during compensation\n  When recovery completes\n  Then cleanup steps run in documented order\n```\n\n## Review Notes\n\nUpdated compensation cleanup notes.\n"))
		headSHA := initGitRepo(t, repoRoot)

		withWorkingDir(t, repoRoot, func() {
			stdout, stderr, exitCode := executeCLIWithInput("Closed the compensation cleanup work and synced the design doc.\n", "rev", "bump", "runtime:session-lifecycle", "--checkpoint", "HEAD")
			if exitCode != 0 {
				t.Fatalf("exit code = %d, stderr=%s", exitCode, stderr)
			}

			var envelope struct {
				State struct {
					SyncedOn string `json:"last_verified_at"`
				} `json:"state"`
				Focus struct {
					ChangelogEntry struct {
						Rev int `json:"rev"`
					} `json:"changelog_entry"`
				} `json:"focus"`
				Result struct {
					Rev        int    `json:"rev"`
					Checkpoint string `json:"checkpoint"`
				} `json:"result"`
				Next testNext `json:"next"`
			}
			mustUnmarshalJSON(t, stdout, &envelope)
			if envelope.Result.Checkpoint != headSHA {
				t.Fatalf("checkpoint = %q, want %q", envelope.Result.Checkpoint, headSHA)
			}
			if envelope.State.SyncedOn == "2026-03-28" {
				t.Fatalf("last_verified_at did not advance: %q", envelope.State.SyncedOn)
			}
			if envelope.Focus.ChangelogEntry.Rev != envelope.Result.Rev {
				t.Fatalf("focus.changelog_entry.rev = %d, result.rev = %d", envelope.Focus.ChangelogEntry.Rev, envelope.Result.Rev)
			}
			if len(envelope.Next) != 0 {
				t.Fatalf("expected empty next, got %#v", envelope.Next)
			}
		})
	})

	t.Run("sync requires checkpoint", func(t *testing.T) {
		repoRoot := copyFixtureRepoWithRegistry(t, "verified-spec")

		withWorkingDir(t, repoRoot, func() {
			stdout, stderr, exitCode := executeCLIWithInput("Review complete\n", "sync", "runtime:session-lifecycle")
			if exitCode == 0 {
				t.Fatalf("expected failure, stdout=%q", stdout)
			}
			envelope := requireFailureEnvelope(t, stdout, stderr)
			if envelope.Error == nil || envelope.Error.Code != "INVALID_INPUT" {
				t.Fatalf("unexpected error %#v", envelope)
			}
			focus := envelope.State["focus"].(map[string]any)
			if _, exists := focus["rev_bump"]; exists {
				t.Fatalf("focus leaked rev_bump = %#v", focus)
			}
			sync := focus["sync"].(map[string]any)
			if sync["reason"] != "missing_checkpoint" {
				t.Fatalf("sync focus = %#v", sync)
			}
		})
	})

	t.Run("sync checkpoint unavailable uses sync focus", func(t *testing.T) {
		repoRoot := copyFixtureRepoWithRegistry(t, "verified-spec")

		withWorkingDir(t, repoRoot, func() {
			stdout, stderr, exitCode := executeCLI("sync", "runtime:session-lifecycle", "--checkpoint", "deadbee")
			if exitCode == 0 {
				t.Fatalf("expected failure, stdout=%q", stdout)
			}
			envelope := requireFailureEnvelope(t, stdout, stderr)
			if envelope.Error == nil || envelope.Error.Code != "CHECKPOINT_UNAVAILABLE" {
				t.Fatalf("unexpected error %#v", envelope)
			}
			focus := envelope.State["focus"].(map[string]any)
			if _, exists := focus["rev_bump"]; exists {
				t.Fatalf("focus leaked rev_bump = %#v", focus)
			}
			sync := focus["sync"].(map[string]any)
			if sync["checkpoint"] != "deadbee" {
				t.Fatalf("sync focus = %#v", sync)
			}
		})
	})

	t.Run("sync requires summary with sync focus reason", func(t *testing.T) {
		repoRoot := copyFixtureRepoWithRegistry(t, "verified-spec")

		withWorkingDir(t, repoRoot, func() {
			stdout, stderr, exitCode := executeCLI("sync", "runtime:session-lifecycle", "--checkpoint", "HEAD")
			if exitCode == 0 {
				t.Fatalf("expected failure, stdout=%q", stdout)
			}
			envelope := requireFailureEnvelope(t, stdout, stderr)
			if envelope.Error == nil || envelope.Error.Code != "INVALID_INPUT" {
				t.Fatalf("unexpected error %#v", envelope)
			}
			focus := envelope.State["focus"].(map[string]any)
			if _, exists := focus["input"]; exists {
				t.Fatalf("focus = %#v", focus)
			}
			sync := focus["sync"].(map[string]any)
			if sync["reason"] != "missing_summary" {
				t.Fatalf("sync focus = %#v", sync)
			}
		})
	})

	t.Run("sync uses summary stdin and re-anchors checkpoint without changing rev or changelog", func(t *testing.T) {
		repoRoot := copyFixtureRepoWithRegistry(t, "verified-spec")
		_ = initGitRepo(t, repoRoot)
		trackingPath := filepath.Join(repoRoot, ".specs", "runtime", "session-lifecycle.yaml")
		before, err := os.ReadFile(trackingPath)
		if err != nil {
			t.Fatalf("read tracking before sync: %v", err)
		}
		previousCheckpoint := ""
		expectedRev := ""
		expectedChangelogEntries := strings.Count(string(before), "\n  - rev: ")
		for _, line := range strings.Split(string(before), "\n") {
			if strings.HasPrefix(line, "checkpoint: ") {
				previousCheckpoint = strings.TrimPrefix(line, "checkpoint: ")
			}
			if strings.HasPrefix(line, "rev: ") {
				expectedRev = strings.TrimPrefix(line, "rev: ")
			}
		}
		if previousCheckpoint == "" || expectedRev == "" {
			t.Fatalf("tracking before sync missing checkpoint or rev: %s", string(before))
		}
		codePath := filepath.Join(repoRoot, "runtime", "src", "domain", "session_execution", "handler.py")
		if err := os.WriteFile(codePath, []byte("def handle():\n    return 'sync'\n"), 0o644); err != nil {
			t.Fatalf("write code file: %v", err)
		}
		commitAllChangesAtDate(t, repoRoot, "2026-03-31T09:30:00Z", "code drift")
		newHead := strings.TrimSpace(runGit(t, repoRoot, "rev-parse", "HEAD"))

		withWorkingDir(t, repoRoot, func() {
			stdout, stderr, exitCode := executeCLIWithInput("Spec reviewed and unchanged.\n", "sync", "runtime:session-lifecycle", "--checkpoint", "HEAD")
			if exitCode != 0 {
				t.Fatalf("exit code = %d, stderr=%s stdout=%s", exitCode, stderr, stdout)
			}

			var envelope struct {
				State struct {
					Rev            int    `json:"rev"`
					Checkpoint     string `json:"checkpoint"`
					LastVerifiedAt string `json:"last_verified_at"`
					Changelog      []any  `json:"changelog"`
				} `json:"state"`
				Result struct {
					Kind               string `json:"kind"`
					PreviousCheckpoint string `json:"previous_checkpoint"`
					Checkpoint         string `json:"checkpoint"`
				} `json:"result"`
				Next testNext `json:"next"`
			}
			mustUnmarshalJSON(t, stdout, &envelope)
			if envelope.Result.Kind != "sync" {
				t.Fatalf("result = %#v", envelope.Result)
			}
			if envelope.Result.PreviousCheckpoint != previousCheckpoint || envelope.Result.Checkpoint != newHead {
				t.Fatalf("result = %#v", envelope.Result)
			}
			if fmt.Sprintf("%d", envelope.State.Rev) != expectedRev || envelope.State.Checkpoint != newHead {
				t.Fatalf("state = %#v", envelope.State)
			}
			if envelope.State.LastVerifiedAt == "2026-03-28" {
				t.Fatalf("last_verified_at did not advance: %#v", envelope.State)
			}
			if len(envelope.State.Changelog) != expectedChangelogEntries {
				t.Fatalf("changelog = %#v", envelope.State.Changelog)
			}
			if len(envelope.Next) != 0 {
				t.Fatalf("expected empty next, got %#v", envelope.Next)
			}

			if stderr != "" {
				t.Fatalf("expected empty stderr, got %q", stderr)
			}
		})
	})

	t.Run("diff compares against the stored checkpoint baseline", func(t *testing.T) {
		repoRoot := copyFixtureRepoWithRegistry(t, "ready-spec")
		replaceInFile(t, filepath.Join(repoRoot, ".specs", "runtime", "session-lifecycle.yaml"), "status: ready", "status: active")
		replaceInFile(t, filepath.Join(repoRoot, ".specs", "runtime", "session-lifecycle.yaml"), "requirements: []\nchangelog: []\n", `requirements:
  - id: REQ-001
    title: Cleanup is documented
    tags:
      - runtime
    test_files: []
    lifecycle: active
    verification: unverified
    introduced_by: D-001
    gherkin: |
      @runtime
      Feature: Cleanup is documented
changelog: []
`)

		withWorkingDir(t, repoRoot, func() {
			stdout, stderr, exitCode := executeCLI("diff", "runtime:session-lifecycle")
			if exitCode != 0 {
				t.Fatalf("exit code = %d, stderr=%s", exitCode, stderr)
			}
			var envelope struct {
				State struct {
					Baseline string `json:"baseline"`
					From     any    `json:"from"`
					Model    struct {
						Deltas struct {
							Opened   []any `json:"opened"`
							Closed   []any `json:"closed"`
							Deferred []any `json:"deferred"`
							Resumed  []any `json:"resumed"`
						} `json:"deltas"`
						Requirements struct {
							Added    []any `json:"added"`
							Verified []any `json:"verified"`
						} `json:"requirements"`
					} `json:"model"`
					DesignDoc struct {
						Changed         bool  `json:"changed"`
						SectionsChanged []any `json:"sections_changed"`
					} `json:"design_doc"`
				} `json:"state"`
				Next testNextMaps `json:"next"`
			}
			mustUnmarshalJSON(t, stdout, &envelope)
			if envelope.State.Baseline != "checkpoint" || envelope.State.From == nil {
				t.Fatalf("unexpected diff state %#v", envelope.State)
			}
			if len(envelope.State.Model.Deltas.Opened) != 0 || len(envelope.State.Model.Deltas.Closed) != 0 || len(envelope.State.Model.Deltas.Deferred) != 0 || len(envelope.State.Model.Deltas.Resumed) != 0 {
				t.Fatalf("unexpected initial deltas %#v", envelope.State.Model.Deltas)
			}
			if len(envelope.State.Model.Requirements.Added) != 1 || len(envelope.State.Model.Requirements.Verified) != 0 {
				t.Fatalf("unexpected checkpoint requirements %#v", envelope.State.Model.Requirements)
			}
			if envelope.State.DesignDoc.Changed || len(envelope.State.DesignDoc.SectionsChanged) != 0 {
				t.Fatalf("unexpected checkpoint design_doc %#v", envelope.State.DesignDoc)
			}
			if len(envelope.Next) != 0 {
				t.Fatalf("next = %#v", envelope.Next)
			}
		})
	})

	t.Run("diff surfaces checkpoint unavailable when the stored baseline tracking file cannot be read", func(t *testing.T) {
		repoRoot := copyFixtureRepoWithRegistry(t, "verified-spec")
		initGitRepoAtDate(t, repoRoot, "2026-03-28T12:00:00Z")
		replaceInFile(t, filepath.Join(repoRoot, ".specs", "runtime", "session-lifecycle.yaml"), "checkpoint: a1b2c3f", "checkpoint: deadbee")

		withWorkingDir(t, repoRoot, func() {
			stdout, stderr, exitCode := executeCLI("diff", "runtime:session-lifecycle")
			if exitCode != 0 {
				t.Fatalf("exit code = %d, stderr=%s stdout=%s", exitCode, stderr, stdout)
			}

			var envelope struct {
				State struct {
					Baseline   string `json:"baseline"`
					From       any    `json:"from"`
					Validation struct {
						Valid    bool `json:"valid"`
						Findings []struct {
							Code string `json:"code"`
						} `json:"findings"`
					} `json:"validation"`
				} `json:"state"`
				Next testNext `json:"next"`
			}
			mustUnmarshalJSON(t, stdout, &envelope)
			if envelope.State.Baseline != "checkpoint" || envelope.State.From != nil {
				t.Fatalf("unexpected diff state %#v", envelope.State)
			}
			if envelope.State.Validation.Valid || len(envelope.State.Validation.Findings) == 0 || envelope.State.Validation.Findings[0].Code != "CHECKPOINT_UNAVAILABLE" {
				t.Fatalf("validation = %#v", envelope.State.Validation)
			}
			if len(envelope.Next) != 1 || envelope.Next[0].(map[string]any)["action"] != "sync" {
				t.Fatalf("next = %#v", envelope.Next)
			}
			if stderr != "" {
				t.Fatalf("expected empty stderr, got %q", stderr)
			}
		})
	})

	t.Run("diff surfaces checkpoint unavailable when the baseline design doc cannot be read", func(t *testing.T) {
		repoRoot := copyFixtureRepoWithRegistry(t, "verified-spec")
		trackingPath := filepath.Join(repoRoot, ".specs", "runtime", "session-lifecycle.yaml")
		replaceInFile(t, trackingPath, "primary: runtime/src/domain/session_execution/SPEC.md", "primary: runtime/src/domain/missing/SPEC.md")
		headSHA := initGitRepoAtDate(t, repoRoot, "2026-03-28T12:00:00Z")
		replaceInFile(t, trackingPath, "primary: runtime/src/domain/missing/SPEC.md", "primary: runtime/src/domain/session_execution/SPEC.md")
		replaceInFile(t, trackingPath, "checkpoint: a1b2c3f", "checkpoint: "+headSHA)

		withWorkingDir(t, repoRoot, func() {
			stdout, stderr, exitCode := executeCLI("diff", "runtime:session-lifecycle")
			if exitCode != 0 {
				t.Fatalf("exit code = %d, stderr=%s stdout=%s", exitCode, stderr, stdout)
			}

			var envelope struct {
				State struct {
					Baseline string `json:"baseline"`
					From     struct {
						Rev int `json:"rev"`
					} `json:"from"`
					DesignDoc struct {
						Changed         bool  `json:"changed"`
						SectionsChanged []any `json:"sections_changed"`
					} `json:"design_doc"`
					Validation struct {
						Valid    bool `json:"valid"`
						Findings []struct {
							Code string `json:"code"`
						} `json:"findings"`
					} `json:"validation"`
				} `json:"state"`
				Next testNext `json:"next"`
			}
			mustUnmarshalJSON(t, stdout, &envelope)
			if envelope.State.Baseline != "checkpoint" || envelope.State.From.Rev != 3 {
				t.Fatalf("unexpected diff state %#v", envelope.State)
			}
			if !envelope.State.DesignDoc.Changed || len(envelope.State.DesignDoc.SectionsChanged) == 0 {
				t.Fatalf("design_doc = %#v", envelope.State.DesignDoc)
			}
			if !envelope.State.Validation.Valid || len(envelope.State.Validation.Findings) != 0 {
				t.Fatalf("validation = %#v", envelope.State.Validation)
			}
			if len(envelope.Next) != 0 {
				t.Fatalf("next = %#v", envelope.Next)
			}
			if stderr != "" {
				t.Fatalf("expected empty stderr, got %q", stderr)
			}
		})
	})

	t.Run("diff ignores pure revision bookkeeping changes", func(t *testing.T) {
		repoRoot := copyFixtureRepoWithRegistry(t, "verified-spec")
		headSHA := initGitRepo(t, repoRoot)
		replaceInFile(t, filepath.Join(repoRoot, ".specs", "runtime", "session-lifecycle.yaml"), "checkpoint: a1b2c3f", "checkpoint: "+headSHA)

		withWorkingDir(t, repoRoot, func() {
			stdout, stderr, exitCode := executeCLI("diff", "runtime:session-lifecycle")
			if exitCode != 0 {
				t.Fatalf("exit code = %d, stderr=%s", exitCode, stderr)
			}
			var envelope struct {
				State struct {
					Model struct {
						Deltas struct {
							Opened []any `json:"opened"`
							Closed []any `json:"closed"`
						} `json:"deltas"`
						Requirements struct {
							Added    []any `json:"added"`
							Verified []any `json:"verified"`
						} `json:"requirements"`
					} `json:"model"`
					DesignDoc struct {
						Changed bool `json:"changed"`
					} `json:"design_doc"`
				} `json:"state"`
			}
			mustUnmarshalJSON(t, stdout, &envelope)
			if len(envelope.State.Model.Deltas.Opened) != 0 || len(envelope.State.Model.Deltas.Closed) != 0 || len(envelope.State.Model.Requirements.Added) != 0 || len(envelope.State.Model.Requirements.Verified) != 0 || envelope.State.DesignDoc.Changed {
				t.Fatalf("unexpected semantic diff %#v", envelope.State)
			}
		})
	})

	t.Run("diff delta summaries expose canonical payload fields for every transition bucket", func(t *testing.T) {
		repoRoot := copyFixtureRepoWithRegistry(t, "ready-spec")
		trackingPath := filepath.Join(repoRoot, ".specs", "runtime", "session-lifecycle.yaml")
		baseline := `slug: session-lifecycle
charter: runtime
title: Session Lifecycle
status: ready
rev: 2
created: 2026-03-05
updated: 2026-03-28
last_verified_at: 2026-03-28
checkpoint: a1b2c3f
tags:
  - runtime
  - domain
documents:
  primary: runtime/src/domain/session_execution/SPEC.md
scope:
  - runtime/src/domain/session_execution/
  - runtime/src/application/commands/
deltas:
  - id: D-001
    area: Closed work
    status: open
    origin_checkpoint: a1b2c3f
    current: Current closed
    target: Target closed
    notes: Baseline close
  - id: D-002
    area: Deferred work
    status: in-progress
    origin_checkpoint: a1b2c3f
    current: Current deferred
    target: Target deferred
    notes: Baseline defer
  - id: D-003
    area: Resumed work
    status: deferred
    origin_checkpoint: a1b2c3f
    current: Current resumed
    target: Target resumed
    notes: Baseline resume
requirements: []
changelog: []
`
		if err := os.WriteFile(trackingPath, []byte(baseline), 0o644); err != nil {
			t.Fatalf("write baseline tracking file: %v", err)
		}
		headSHA := initGitRepo(t, repoRoot)
		current := `slug: session-lifecycle
charter: runtime
title: Session Lifecycle
status: ready
rev: 3
created: 2026-03-05
updated: 2026-03-30
last_verified_at: 2026-03-30
checkpoint: ` + headSHA + `
tags:
  - runtime
  - domain
documents:
  primary: runtime/src/domain/session_execution/SPEC.md
scope:
  - runtime/src/domain/session_execution/
  - runtime/src/application/commands/
deltas:
  - id: D-001
    area: Closed work
    status: closed
    origin_checkpoint: a1b2c3f
    current: Current closed
    target: Target closed
    notes: Current close
  - id: D-002
    area: Deferred work
    status: deferred
    origin_checkpoint: a1b2c3f
    current: Current deferred
    target: Target deferred
    notes: Current defer
  - id: D-003
    area: Resumed work
    status: open
    origin_checkpoint: a1b2c3f
    current: Current resumed
    target: Target resumed
    notes: Current resume
  - id: D-004
    area: Opened work
    status: open
    origin_checkpoint: a1b2c3f
    current: Current opened
    target: Target opened
    notes: Current open
requirements: []
changelog: []
`
		if err := os.WriteFile(trackingPath, []byte(current), 0o644); err != nil {
			t.Fatalf("write current tracking file: %v", err)
		}

		withWorkingDir(t, repoRoot, func() {
			stdout, stderr, exitCode := executeCLI("diff", "runtime:session-lifecycle")
			if exitCode != 0 {
				t.Fatalf("exit code = %d, stderr=%s stdout=%s", exitCode, stderr, stdout)
			}

			var envelope struct {
				State struct {
					Model struct {
						Deltas struct {
							Opened []struct {
								ID      string `json:"id"`
								Area    string `json:"area"`
								Status  string `json:"status"`
								Current string `json:"current"`
								Target  string `json:"target"`
							} `json:"opened"`
							Closed []struct {
								ID      string `json:"id"`
								Area    string `json:"area"`
								Status  string `json:"status"`
								Current string `json:"current"`
								Target  string `json:"target"`
							} `json:"closed"`
							Deferred []struct {
								ID      string `json:"id"`
								Area    string `json:"area"`
								Status  string `json:"status"`
								Current string `json:"current"`
								Target  string `json:"target"`
							} `json:"deferred"`
							Resumed []struct {
								ID      string `json:"id"`
								Area    string `json:"area"`
								Status  string `json:"status"`
								Current string `json:"current"`
								Target  string `json:"target"`
							} `json:"resumed"`
						} `json:"deltas"`
					} `json:"model"`
				} `json:"state"`
			}
			mustUnmarshalJSON(t, stdout, &envelope)
			if got := envelope.State.Model.Deltas.Opened; len(got) != 1 || got[0].ID != "D-004" || got[0].Area != "Opened work" || got[0].Status != "open" || got[0].Current != "Current opened" || got[0].Target != "Target opened" {
				t.Fatalf("opened = %#v", got)
			}
			if got := envelope.State.Model.Deltas.Closed; len(got) != 1 || got[0].ID != "D-001" || got[0].Area != "Closed work" || got[0].Status != "closed" || got[0].Current != "Current closed" || got[0].Target != "Target closed" {
				t.Fatalf("closed = %#v", got)
			}
			if got := envelope.State.Model.Deltas.Deferred; len(got) != 1 || got[0].ID != "D-002" || got[0].Area != "Deferred work" || got[0].Status != "deferred" || got[0].Current != "Current deferred" || got[0].Target != "Target deferred" {
				t.Fatalf("deferred = %#v", got)
			}
			if got := envelope.State.Model.Deltas.Resumed; len(got) != 1 || got[0].ID != "D-003" || got[0].Area != "Resumed work" || got[0].Status != "open" || got[0].Current != "Current resumed" || got[0].Target != "Target resumed" {
				t.Fatalf("resumed = %#v", got)
			}
			if stderr != "" {
				t.Fatalf("expected empty stderr, got %q", stderr)
			}
		})
	})

	t.Run("diff reports design doc section add remove modify changes", func(t *testing.T) {
		repoRoot := copyFixtureRepoWithRegistry(t, "verified-spec")
		docPath := filepath.Join(repoRoot, "runtime", "src", "domain", "session_execution", "SPEC.md")
		if err := os.WriteFile(docPath, []byte(`---
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
`), 0644); err != nil {
			t.Fatalf("write baseline design doc: %v", err)
		}
		headSHA := initGitRepo(t, repoRoot)
		replaceInFile(t, filepath.Join(repoRoot, ".specs", "runtime", "session-lifecycle.yaml"), "checkpoint: a1b2c3f", "checkpoint: "+headSHA)
		if err := os.WriteFile(docPath, []byte(`---
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
`), 0644); err != nil {
			t.Fatalf("write current design doc: %v", err)
		}

		withWorkingDir(t, repoRoot, func() {
			stdout, stderr, exitCode := executeCLI("diff", "runtime:session-lifecycle")
			if exitCode != 0 {
				t.Fatalf("exit code = %d, stderr=%s stdout=%s", exitCode, stderr, stdout)
			}

			var envelope struct {
				State struct {
					DesignDoc struct {
						Changed         bool `json:"changed"`
						SectionsChanged []struct {
							Heading string `json:"heading"`
							Type    string `json:"type"`
							Lines   []int  `json:"lines"`
						} `json:"sections_changed"`
					} `json:"design_doc"`
				} `json:"state"`
			}
			mustUnmarshalJSON(t, stdout, &envelope)
			if !envelope.State.DesignDoc.Changed {
				t.Fatalf("design_doc = %#v", envelope.State.DesignDoc)
			}
			if len(envelope.State.DesignDoc.SectionsChanged) != 3 {
				t.Fatalf("sections = %#v", envelope.State.DesignDoc.SectionsChanged)
			}
			got := make([]string, 0, len(envelope.State.DesignDoc.SectionsChanged))
			for _, section := range envelope.State.DesignDoc.SectionsChanged {
				if len(section.Lines) != 2 {
					t.Fatalf("section lines = %#v", section.Lines)
				}
				got = append(got, fmt.Sprintf("%s:%s:%d,%d", section.Type, section.Heading, section.Lines[0], section.Lines[1]))
			}
			want := []string{
				"modified:Modified Section:10,12",
				"added:Added Section:13,14",
				"removed:Removed Section:13,14",
			}
			if strings.Join(got, "|") != strings.Join(want, "|") {
				t.Fatalf("sections = %v, want %v", got, want)
			}
		})
	})

	t.Run("diff renders heading renames as remove add pairs with physical line numbers", func(t *testing.T) {
		repoRoot := copyFixtureRepoWithRegistry(t, "verified-spec")
		docPath := filepath.Join(repoRoot, "runtime", "src", "domain", "session_execution", "SPEC.md")
		if err := os.WriteFile(docPath, []byte(`---
spec: session-lifecycle
charter: runtime
---
# Session Lifecycle

## Deployment Plan
Keep the same body.
`), 0644); err != nil {
			t.Fatalf("write baseline design doc: %v", err)
		}
		headSHA := initGitRepo(t, repoRoot)
		replaceInFile(t, filepath.Join(repoRoot, ".specs", "runtime", "session-lifecycle.yaml"), "checkpoint: a1b2c3f", "checkpoint: "+headSHA)
		if err := os.WriteFile(docPath, []byte(`---
spec: session-lifecycle
charter: runtime
---
# Session Lifecycle

## Rollout Plan
Keep the same body.
`), 0644); err != nil {
			t.Fatalf("write current design doc: %v", err)
		}

		withWorkingDir(t, repoRoot, func() {
			stdout, stderr, exitCode := executeCLI("diff", "runtime:session-lifecycle")
			if exitCode != 0 {
				t.Fatalf("exit code = %d, stderr=%s stdout=%s", exitCode, stderr, stdout)
			}

			var envelope struct {
				State struct {
					DesignDoc struct {
						Changed         bool `json:"changed"`
						SectionsChanged []struct {
							Heading string `json:"heading"`
							Type    string `json:"type"`
							Lines   []int  `json:"lines"`
						} `json:"sections_changed"`
					} `json:"design_doc"`
				} `json:"state"`
			}
			mustUnmarshalJSON(t, stdout, &envelope)
			if !envelope.State.DesignDoc.Changed {
				t.Fatalf("design_doc = %#v", envelope.State.DesignDoc)
			}
			if len(envelope.State.DesignDoc.SectionsChanged) != 2 {
				t.Fatalf("sections = %#v", envelope.State.DesignDoc.SectionsChanged)
			}
			got := make([]string, 0, len(envelope.State.DesignDoc.SectionsChanged))
			for _, section := range envelope.State.DesignDoc.SectionsChanged {
				if len(section.Lines) != 2 {
					t.Fatalf("section lines = %#v", section.Lines)
				}
				got = append(got, fmt.Sprintf("%s:%s:%d,%d", section.Type, section.Heading, section.Lines[0], section.Lines[1]))
			}
			want := []string{
				"removed:Deployment Plan:7,8",
				"added:Rollout Plan:7,8",
			}
			if strings.Join(got, "|") != strings.Join(want, "|") {
				t.Fatalf("sections = %v, want %v", got, want)
			}
			if stderr != "" {
				t.Fatalf("expected empty stderr, got %q", stderr)
			}
		})
	})

	t.Run("diff matches duplicate headings by ordinal occurrence instead of content similarity", func(t *testing.T) {
		repoRoot := copyFixtureRepoWithRegistry(t, "verified-spec")
		docPath := filepath.Join(repoRoot, "runtime", "src", "domain", "session_execution", "SPEC.md")
		if err := os.WriteFile(docPath, []byte(`---
spec: session-lifecycle
charter: runtime
---
# Session Lifecycle

## Step
First body.

## Step
Second body.
`), 0o644); err != nil {
			t.Fatalf("write baseline design doc: %v", err)
		}
		headSHA := initGitRepo(t, repoRoot)
		replaceInFile(t, filepath.Join(repoRoot, ".specs", "runtime", "session-lifecycle.yaml"), "checkpoint: a1b2c3f", "checkpoint: "+headSHA)
		if err := os.WriteFile(docPath, []byte(`---
spec: session-lifecycle
charter: runtime
---
# Session Lifecycle

## Step
Second body.

## Step
First body.
`), 0o644); err != nil {
			t.Fatalf("write current design doc: %v", err)
		}

		withWorkingDir(t, repoRoot, func() {
			stdout, stderr, exitCode := executeCLI("diff", "runtime:session-lifecycle")
			if exitCode != 0 {
				t.Fatalf("exit code = %d, stderr=%s stdout=%s", exitCode, stderr, stdout)
			}

			var envelope struct {
				State struct {
					DesignDoc struct {
						Changed         bool `json:"changed"`
						SectionsChanged []struct {
							Heading string `json:"heading"`
							Type    string `json:"type"`
							Lines   []int  `json:"lines"`
						} `json:"sections_changed"`
					} `json:"design_doc"`
				} `json:"state"`
			}
			mustUnmarshalJSON(t, stdout, &envelope)
			if !envelope.State.DesignDoc.Changed {
				t.Fatalf("design_doc = %#v", envelope.State.DesignDoc)
			}
			got := make([]string, 0, len(envelope.State.DesignDoc.SectionsChanged))
			for _, section := range envelope.State.DesignDoc.SectionsChanged {
				got = append(got, fmt.Sprintf("%s:%s:%d,%d", section.Type, section.Heading, section.Lines[0], section.Lines[1]))
			}
			want := []string{
				"modified:Step:7,9",
				"modified:Step:10,11",
			}
			if strings.Join(got, "|") != strings.Join(want, "|") {
				t.Fatalf("sections = %v, want %v", got, want)
			}
			if stderr != "" {
				t.Fatalf("expected empty stderr, got %q", stderr)
			}
		})
	})

	t.Run("diff treats duplicate heading insertion as occurrence shifts plus add", func(t *testing.T) {
		repoRoot := copyFixtureRepoWithRegistry(t, "verified-spec")
		docPath := filepath.Join(repoRoot, "runtime", "src", "domain", "session_execution", "SPEC.md")
		if err := os.WriteFile(docPath, []byte(`---
spec: session-lifecycle
charter: runtime
---
# Session Lifecycle

## Step
Existing first body.

## Step
Existing second body.
`), 0o644); err != nil {
			t.Fatalf("write baseline design doc: %v", err)
		}
		headSHA := initGitRepo(t, repoRoot)
		replaceInFile(t, filepath.Join(repoRoot, ".specs", "runtime", "session-lifecycle.yaml"), "checkpoint: a1b2c3f", "checkpoint: "+headSHA)
		if err := os.WriteFile(docPath, []byte(`---
spec: session-lifecycle
charter: runtime
---
# Session Lifecycle

## Step
Inserted first body.

## Step
Existing first body.

## Step
Existing second body.
`), 0o644); err != nil {
			t.Fatalf("write current design doc: %v", err)
		}

		withWorkingDir(t, repoRoot, func() {
			stdout, stderr, exitCode := executeCLI("diff", "runtime:session-lifecycle")
			if exitCode != 0 {
				t.Fatalf("exit code = %d, stderr=%s stdout=%s", exitCode, stderr, stdout)
			}

			var envelope struct {
				State struct {
					DesignDoc struct {
						Changed         bool `json:"changed"`
						SectionsChanged []struct {
							Heading string `json:"heading"`
							Type    string `json:"type"`
							Lines   []int  `json:"lines"`
						} `json:"sections_changed"`
					} `json:"design_doc"`
				} `json:"state"`
			}
			mustUnmarshalJSON(t, stdout, &envelope)
			if !envelope.State.DesignDoc.Changed {
				t.Fatalf("design_doc = %#v", envelope.State.DesignDoc)
			}
			got := make([]string, 0, len(envelope.State.DesignDoc.SectionsChanged))
			for _, section := range envelope.State.DesignDoc.SectionsChanged {
				got = append(got, fmt.Sprintf("%s:%s:%d,%d", section.Type, section.Heading, section.Lines[0], section.Lines[1]))
			}
			want := []string{
				"modified:Step:7,9",
				"modified:Step:10,12",
				"added:Step:13,14",
			}
			if strings.Join(got, "|") != strings.Join(want, "|") {
				t.Fatalf("sections = %v, want %v", got, want)
			}
			if stderr != "" {
				t.Fatalf("expected empty stderr, got %q", stderr)
			}
		})
	})

	t.Run("diff reads the baseline design doc from the checkpoint path after document moves", func(t *testing.T) {
		repoRoot := copyFixtureRepoWithRegistry(t, "verified-spec")
		originalDocPath := filepath.Join(repoRoot, "runtime", "src", "domain", "session_execution", "SPEC.md")
		originalDocContent := `---
spec: session-lifecycle
charter: runtime
---
# Session Lifecycle

## Stable Section
Baseline content.
`
		if err := os.WriteFile(originalDocPath, []byte(originalDocContent), 0644); err != nil {
			t.Fatalf("write baseline design doc: %v", err)
		}
		headSHA := initGitRepo(t, repoRoot)
		replaceInFile(t, filepath.Join(repoRoot, ".specs", "runtime", "session-lifecycle.yaml"), "checkpoint: a1b2c3f", "checkpoint: "+headSHA)
		replaceInFile(t, filepath.Join(repoRoot, ".specs", "runtime", "session-lifecycle.yaml"), "primary: runtime/src/domain/session_execution/SPEC.md", "primary: runtime/src/domain/session_flow/SPEC.md")
		newDocPath := filepath.Join(repoRoot, "runtime", "src", "domain", "session_flow", "SPEC.md")
		if err := os.MkdirAll(filepath.Dir(newDocPath), 0755); err != nil {
			t.Fatalf("mkdir moved design doc dir: %v", err)
		}
		if err := os.WriteFile(newDocPath, []byte(originalDocContent), 0644); err != nil {
			t.Fatalf("write moved design doc: %v", err)
		}

		withWorkingDir(t, repoRoot, func() {
			stdout, stderr, exitCode := executeCLI("diff", "runtime:session-lifecycle")
			if exitCode != 0 {
				t.Fatalf("exit code = %d, stderr=%s stdout=%s", exitCode, stderr, stdout)
			}

			var envelope struct {
				State struct {
					Model struct {
						Documents struct {
							PrimaryFrom string `json:"primary_from"`
							PrimaryTo   string `json:"primary_to"`
						} `json:"documents"`
					} `json:"model"`
					DesignDoc struct {
						Path            string `json:"path"`
						Changed         bool   `json:"changed"`
						SectionsChanged []any  `json:"sections_changed"`
					} `json:"design_doc"`
				} `json:"state"`
			}
			mustUnmarshalJSON(t, stdout, &envelope)
			if envelope.State.Model.Documents.PrimaryFrom != "runtime/src/domain/session_execution/SPEC.md" {
				t.Fatalf("documents.primary_from = %q", envelope.State.Model.Documents.PrimaryFrom)
			}
			if envelope.State.Model.Documents.PrimaryTo != "runtime/src/domain/session_flow/SPEC.md" {
				t.Fatalf("documents.primary_to = %q", envelope.State.Model.Documents.PrimaryTo)
			}
			if envelope.State.DesignDoc.Path != "runtime/src/domain/session_flow/SPEC.md" {
				t.Fatalf("design_doc.path = %q", envelope.State.DesignDoc.Path)
			}
			if envelope.State.DesignDoc.Changed {
				t.Fatalf("design_doc.changed = %#v, want false when only the tracked path moved", envelope.State.DesignDoc)
			}
			if len(envelope.State.DesignDoc.SectionsChanged) != 0 {
				t.Fatalf("design_doc.sections = %#v", envelope.State.DesignDoc.SectionsChanged)
			}
			if stderr != "" {
				t.Fatalf("expected empty stderr, got %q", stderr)
			}
		})
	})

	t.Run("diff charter preserves charter ordering", func(t *testing.T) {
		repoRoot := copyFixtureRepo(t, "charter-dag")

		withWorkingDir(t, repoRoot, func() {
			stdout, stderr, exitCode := executeCLI("diff", "--charter", "runtime")
			if exitCode != 0 {
				t.Fatalf("exit code = %d, stderr=%s", exitCode, stderr)
			}
			var envelope struct {
				State struct {
					OrderedSpecs []struct {
						Slug string `json:"slug"`
					} `json:"ordered_specs"`
				} `json:"state"`
			}
			mustUnmarshalJSON(t, stdout, &envelope)
			got := orderedSlugs(envelope.State.OrderedSpecs)
			want := []string{"redis-state", "recovery-projection", "session-lifecycle"}
			if strings.Join(got, ",") != strings.Join(want, ",") {
				t.Fatalf("ordered_specs = %v, want %v", got, want)
			}
		})
	})

	t.Run("diff charter marks design-doc-only drift as changed", func(t *testing.T) {
		repoRoot := copyFixtureRepoWithRegistry(t, "verified-spec")
		headSHA := initGitRepo(t, repoRoot)
		replaceInFile(t, filepath.Join(repoRoot, ".specs", "runtime", "session-lifecycle.yaml"), "checkpoint: a1b2c3f", "checkpoint: "+headSHA)
		replaceInFile(t, filepath.Join(repoRoot, "runtime", "src", "domain", "session_execution", "SPEC.md"), "# Session Lifecycle\n", "# Session Lifecycle\n\n## Drift Review\n\nUpdated after sync.\n")

		withWorkingDir(t, repoRoot, func() {
			stdout, stderr, exitCode := executeCLI("diff", "runtime:session-lifecycle")
			if exitCode != 0 {
				t.Fatalf("spec diff exit code = %d, stderr=%s stdout=%s", exitCode, stderr, stdout)
			}

			var specEnvelope struct {
				State struct {
					DesignDoc struct {
						Changed bool `json:"changed"`
					} `json:"design_doc"`
					ScopeCode struct {
						ChangedFiles []string `json:"changed_files"`
					} `json:"scope_code"`
				} `json:"state"`
			}
			mustUnmarshalJSON(t, stdout, &specEnvelope)
			if !specEnvelope.State.DesignDoc.Changed {
				t.Fatalf("design_doc = %#v", specEnvelope.State.DesignDoc)
			}
			if len(specEnvelope.State.ScopeCode.ChangedFiles) != 0 {
				t.Fatalf("scope_code = %#v", specEnvelope.State.ScopeCode)
			}

			stdout, stderr, exitCode = executeCLI("diff", "--charter", "runtime")
			if exitCode != 0 {
				t.Fatalf("charter diff exit code = %d, stderr=%s stdout=%s", exitCode, stderr, stdout)
			}

			var charterEnvelope struct {
				State struct {
					OrderedSpecs []struct {
						Slug    string `json:"slug"`
						Changed bool   `json:"changed"`
					} `json:"ordered_specs"`
				} `json:"state"`
			}
			mustUnmarshalJSON(t, stdout, &charterEnvelope)
			if len(charterEnvelope.State.OrderedSpecs) != 1 {
				t.Fatalf("ordered_specs = %#v", charterEnvelope.State.OrderedSpecs)
			}
			if charterEnvelope.State.OrderedSpecs[0].Slug != "session-lifecycle" || !charterEnvelope.State.OrderedSpecs[0].Changed {
				t.Fatalf("ordered_specs = %#v", charterEnvelope.State.OrderedSpecs)
			}
			if stderr != "" {
				t.Fatalf("expected empty stderr, got %q", stderr)
			}
		})
	})

	t.Run("diff returns validation findings instead of failing on malformed stored spec state", func(t *testing.T) {
		repoRoot := copyFixtureRepo(t, "malformed-gapful-spec")

		withWorkingDir(t, repoRoot, func() {
			stdout, stderr, exitCode := executeCLI("diff", "runtime:session-lifecycle")
			if exitCode != 0 {
				t.Fatalf("exit code = %d, stderr=%s stdout=%s", exitCode, stderr, stdout)
			}

			var envelope struct {
				State struct {
					To struct {
						Status string `json:"status"`
					} `json:"to"`
					Validation struct {
						Valid    bool `json:"valid"`
						Findings []struct {
							Code string `json:"code"`
						} `json:"findings"`
					} `json:"validation"`
				} `json:"state"`
				Next testNextMaps `json:"next"`
			}
			mustUnmarshalJSON(t, stdout, &envelope)
			if envelope.State.To.Status != "ready" {
				t.Fatalf("to = %#v", envelope.State.To)
			}
			if envelope.State.Validation.Valid || len(envelope.State.Validation.Findings) == 0 {
				t.Fatalf("validation = %#v", envelope.State.Validation)
			}
			if len(envelope.Next) != 1 || envelope.Next[0]["action"] != "sync" {
				t.Fatalf("next = %#v", envelope.Next)
			}
			if stderr != "" {
				t.Fatalf("expected empty stderr, got %q", stderr)
			}
		})
	})

	t.Run("diff charter returns validation findings instead of failing on malformed stored charter state", func(t *testing.T) {
		repoRoot := copyFixtureRepo(t, "charter-cycle")

		withWorkingDir(t, repoRoot, func() {
			stdout, stderr, exitCode := executeCLI("diff", "--charter", "runtime")
			if exitCode != 0 {
				t.Fatalf("exit code = %d, stderr=%s stdout=%s", exitCode, stderr, stdout)
			}

			var envelope struct {
				State struct {
					Validation struct {
						Valid    bool `json:"valid"`
						Findings []struct {
							Code string `json:"code"`
						} `json:"findings"`
					} `json:"validation"`
				} `json:"state"`
			}
			mustUnmarshalJSON(t, stdout, &envelope)
			if envelope.State.Validation.Valid || len(envelope.State.Validation.Findings) == 0 {
				t.Fatalf("validation = %#v", envelope.State.Validation)
			}
			if stderr != "" {
				t.Fatalf("expected empty stderr, got %q", stderr)
			}
		})
	})

	t.Run("diff charter serializes endpoint objects without status fields", func(t *testing.T) {
		repoRoot := copyFixtureRepo(t, "charter-dag")

		withWorkingDir(t, repoRoot, func() {
			stdout, stderr, exitCode := executeCLI("diff", "--charter", "runtime")
			if exitCode != 0 {
				t.Fatalf("exit code = %d, stderr=%s stdout=%s", exitCode, stderr, stdout)
			}

			var envelope map[string]any
			mustUnmarshalJSON(t, stdout, &envelope)
			state := envelope["state"].(map[string]any)
			orderedSpecs := state["ordered_specs"].([]any)
			if len(orderedSpecs) == 0 {
				t.Fatalf("ordered_specs = %#v", orderedSpecs)
			}
			first := orderedSpecs[0].(map[string]any)
			to := first["to"].(map[string]any)
			requireObjectKeys(t, to, "checkpoint", "rev")
			if _, exists := to["status"]; exists {
				t.Fatalf("to = %#v", to)
			}
			if first["from"] != nil {
				t.Fatalf("from = %#v, want nil for initial charter diff entry", first["from"])
			}
			if stderr != "" {
				t.Fatalf("expected empty stderr, got %q", stderr)
			}
		})
	})
}

func TestAdjacentCLICommands(t *testing.T) {
	t.Run("charter add-spec replaces one entry", func(t *testing.T) {
		repoRoot := copyFixtureRepoWithRegistry(t, "charter-dag")

		withWorkingDir(t, repoRoot, func() {
			stdout, stderr, exitCode := executeCLIWithInput("group: recovery\norder: 30\ndepends_on:\n  - redis-state\n  - recovery-projection\nnotes: Session FSM and cleanup behavior\n", "charter", "add-spec", "runtime", "session-lifecycle")
			if exitCode != 0 {
				t.Fatalf("exit code = %d, stderr=%s", exitCode, stderr)
			}
			if stderr != "" {
				t.Fatalf("expected empty stderr, got %q", stderr)
			}

			var envelope struct {
				State struct {
					OrderedSpecs []struct {
						Slug      string   `json:"slug"`
						DependsOn []string `json:"depends_on"`
					} `json:"ordered_specs"`
					Validation struct {
						Valid bool `json:"valid"`
					} `json:"validation"`
				} `json:"state"`
				Result struct {
					Kind         string `json:"kind"`
					CreatedGroup any    `json:"created_group"`
					Entry        struct {
						Slug      string   `json:"slug"`
						DependsOn []string `json:"depends_on"`
					} `json:"entry"`
				} `json:"result"`
				Next testNextMaps `json:"next"`
			}
			mustUnmarshalJSON(t, stdout, &envelope)
			if envelope.Result.Kind != "charter_entry" || envelope.Result.Entry.Slug != "session-lifecycle" || strings.Join(envelope.Result.Entry.DependsOn, ",") != "redis-state,recovery-projection" {
				t.Fatalf("unexpected result %#v", envelope.Result)
			}
			if !envelope.State.Validation.Valid {
				t.Fatalf("state.validation = %#v", envelope.State.Validation)
			}
			if envelope.Result.CreatedGroup != nil {
				t.Fatalf("created_group = %#v", envelope.Result.CreatedGroup)
			}
			if next := mapsToAny(envelope.Next); len(next) != 0 {
				t.Fatalf("next = %#v", next)
			}
		})
	})

	t.Run("charter add-spec ignores partial group metadata for an existing group", func(t *testing.T) {
		repoRoot := copyFixtureRepoWithRegistry(t, "ready-spec")

		withWorkingDir(t, repoRoot, func() {
			stdout, stderr, exitCode := executeCLIWithInput("group: recovery\ngroup_title: Should Be Ignored\norder: 30\ndepends_on: []\nnotes: Session FSM and cleanup behavior\n", "charter", "add-spec", "runtime", "session-lifecycle")
			if exitCode != 0 {
				t.Fatalf("exit code = %d, stderr=%s stdout=%s", exitCode, stderr, stdout)
			}
			if stderr != "" {
				t.Fatalf("expected empty stderr, got %q", stderr)
			}

			var envelope struct {
				Result struct {
					CreatedGroup any `json:"created_group"`
					Entry        struct {
						Group string `json:"group"`
						Order int    `json:"order"`
					} `json:"entry"`
				} `json:"result"`
			}
			mustUnmarshalJSON(t, stdout, &envelope)
			if envelope.Result.CreatedGroup != nil {
				t.Fatalf("created_group = %#v", envelope.Result.CreatedGroup)
			}
			if envelope.Result.Entry.Group != "recovery" || envelope.Result.Entry.Order != 30 {
				t.Fatalf("entry = %#v", envelope.Result.Entry)
			}
		})
	})

	t.Run("charter add-spec loads charter state before requiring missing group metadata", func(t *testing.T) {
		repoRoot := copyFixtureRepoWithRegistry(t, "ready-spec")

		withWorkingDir(t, repoRoot, func() {
			stdout, stderr, exitCode := executeCLIWithInput("group: missing-group\ngroup_title: Missing Group\norder: 30\ndepends_on: []\nnotes: Session FSM and cleanup behavior\n", "charter", "add-spec", "runtime", "session-lifecycle")
			if exitCode == 0 {
				t.Fatalf("expected failure, stdout=%q stderr=%q", stdout, stderr)
			}

			envelope := requireFailureEnvelope(t, stdout, stderr)
			if envelope.Error == nil || envelope.Error.Code != "GROUP_REQUIRED" {
				t.Fatalf("unexpected envelope %#v", envelope)
			}
			focus := envelope.State["focus"].(map[string]any)["input"].(map[string]any)
			if got := strings.Join(stringSliceFromAny(t, focus["missing_fields"]), ","); got != "group_order" {
				t.Fatalf("missing_fields = %#v", focus["missing_fields"])
			}
		})
	})

	t.Run("charter add-spec returns canonical charter validation failure for malformed spec tracking", func(t *testing.T) {
		repoRoot := copyFixtureRepoWithRegistry(t, "ready-spec")
		replaceInFile(t, filepath.Join(repoRoot, ".specs", "runtime", "session-lifecycle.yaml"), "status: ready", "status: invalid")

		withWorkingDir(t, repoRoot, func() {
			stdout, stderr, exitCode := executeCLIWithInput("group: recovery\norder: 30\ndepends_on: []\nnotes: Session FSM and cleanup behavior\n", "charter", "add-spec", "runtime", "session-lifecycle")
			if exitCode == 0 {
				t.Fatalf("expected failure, stdout=%q stderr=%q", stdout, stderr)
			}

			envelope := requireFailureEnvelope(t, stdout, stderr)
			if envelope.Error == nil || envelope.Error.Code != "VALIDATION_FAILED" {
				t.Fatalf("unexpected envelope %#v", envelope)
			}
			if envelope.State["name"] != "runtime" || envelope.State["tracking_file"] != ".specs/runtime/CHARTER.yaml" {
				t.Fatalf("state = %#v", envelope.State)
			}
			orderedSpecs := envelope.State["ordered_specs"].([]any)
			if len(orderedSpecs) != 1 {
				t.Fatalf("ordered_specs = %#v", orderedSpecs)
			}
			specState := orderedSpecs[0].(map[string]any)
			if specState["slug"] != "session-lifecycle" {
				t.Fatalf("ordered_spec = %#v", specState)
			}
			validation := specState["validation"].(map[string]any)
			if validation["valid"] != false {
				t.Fatalf("ordered_spec.validation = %#v", validation)
			}
			focus := envelope.State["focus"].(map[string]any)
			findings := focus["validation"].(map[string]any)["findings"].([]any)
			if len(findings) == 0 {
				t.Fatalf("focus.validation = %#v", focus)
			}
		})
	})

	t.Run("charter add-spec semantic validation failures include canonical charter validation", func(t *testing.T) {
		cases := []struct {
			name  string
			stdin string
		}{
			{
				name:  "invalid new group key",
				stdin: "group: bad_group\ngroup_title: Broken Group\ngroup_order: 30\norder: 30\ndepends_on: []\nnotes: Session FSM and cleanup behavior\n",
			},
			{
				name:  "negative order",
				stdin: "group: recovery\norder: -1\ndepends_on: []\nnotes: Session FSM and cleanup behavior\n",
			},
			{
				name:  "unknown dependency",
				stdin: "group: recovery\norder: 30\ndepends_on:\n  - missing-spec\nnotes: Session FSM and cleanup behavior\n",
			},
		}

		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				repoRoot := copyFixtureRepoWithRegistry(t, "ready-spec")

				withWorkingDir(t, repoRoot, func() {
					stdout, stderr, exitCode := executeCLIWithInput(tc.stdin, "charter", "add-spec", "runtime", "session-lifecycle")
					if exitCode == 0 {
						t.Fatalf("expected failure, stdout=%q stderr=%q", stdout, stderr)
					}

					envelope := requireFailureEnvelope(t, stdout, stderr)
					if envelope.Error == nil || envelope.Error.Code != "VALIDATION_FAILED" {
						t.Fatalf("unexpected envelope %#v", envelope)
					}
					if envelope.State["name"] != "runtime" || envelope.State["tracking_file"] != ".specs/runtime/CHARTER.yaml" {
						t.Fatalf("state = %#v", envelope.State)
					}
					validation := envelope.State["validation"].(map[string]any)
					findings := validation["findings"].([]any)
					if validation["valid"] != false || len(findings) == 0 {
						t.Fatalf("state.validation = %#v", validation)
					}
					focusFindings := envelope.State["focus"].(map[string]any)["validation"].(map[string]any)["findings"].([]any)
					if len(focusFindings) == 0 {
						t.Fatalf("focus.validation = %#v", envelope.State["focus"])
					}
				})
			})
		}
	})

	t.Run("charter add-spec cycle includes focus context", func(t *testing.T) {
		repoRoot := copyFixtureRepoWithRegistry(t, "charter-dag")

		withWorkingDir(t, repoRoot, func() {
			stdout, stderr, exitCode := executeCLIWithInput("group: execution\norder: 10\ndepends_on:\n  - session-lifecycle\nnotes: Storage and CAS guarantees\n", "charter", "add-spec", "runtime", "redis-state")
			if exitCode == 0 {
				t.Fatalf("expected failure, stdout=%q stderr=%q", stdout, stderr)
			}

			envelope := requireFailureEnvelope(t, stdout, stderr)
			if envelope.Error == nil || envelope.Error.Code != "CHARTER_CYCLE" {
				t.Fatalf("unexpected envelope %#v", envelope)
			}
			if envelope.State["name"] != "runtime" || envelope.State["tracking_file"] != ".specs/runtime/CHARTER.yaml" {
				t.Fatalf("state = %#v", envelope.State)
			}
			focus := envelope.State["focus"].(map[string]any)
			entry := focus["entry"].(map[string]any)
			if entry["slug"] != "redis-state" {
				t.Fatalf("focus.entry = %#v", entry)
			}
			if got := strings.Join(stringSliceFromAny(t, entry["depends_on"]), ","); got != "session-lifecycle" {
				t.Fatalf("focus.entry.depends_on = %#v", entry["depends_on"])
			}
			if got := strings.Join(stringSliceFromAny(t, focus["cycle"]), ","); got != "redis-state,session-lifecycle" {
				t.Fatalf("focus.cycle = %#v", focus["cycle"])
			}
		})
	})

	t.Run("charter remove-spec reports dependents", func(t *testing.T) {
		repoRoot := copyFixtureRepoWithRegistry(t, "charter-dag")
		replaceInFile(t, filepath.Join(repoRoot, ".specs", "runtime", "recovery-projection.yaml"), "status: draft", "status: invalid")
		writeCLIAdjacentTestFile(t, filepath.Join(repoRoot, ".specs", "runtime", "stray-tracking.yaml"), []byte(`slug: stray-tracking
charter: runtime
title: Stray Tracking
status: draft
rev: 1
created: 2026-03-05
updated: 2026-03-30
last_verified_at: 2026-03-28
checkpoint: a1b2c3f
tags:
  - runtime
documents:
  primary: runtime/src/domain/redis_state/SPEC.md
scope:
  - runtime/src/domain/redis_state/
deltas: []
requirements: []
changelog: []
`))

		withWorkingDir(t, repoRoot, func() {
			stdout, stderr, exitCode := executeCLI("charter", "remove-spec", "runtime", "redis-state")
			if exitCode == 0 {
				t.Fatalf("expected failure, stdout=%q", stdout)
			}
			envelope := requireFailureEnvelope(t, stdout, stderr)
			if envelope.Error == nil || envelope.Error.Code != "CHARTER_DEPENDENCY_EXISTS" {
				t.Fatalf("unexpected envelope %#v", envelope)
			}
			dependents := stringSliceFromAny(t, envelope.State["focus"].(map[string]any)["dependents"])
			if strings.Join(dependents, ",") != "recovery-projection,session-lifecycle" {
				t.Fatalf("dependents = %v", dependents)
			}
			validation := envelope.State["validation"].(map[string]any)
			if validation["valid"] != true {
				t.Fatalf("state.validation = %#v", validation)
			}
			assertCLIOrderedSpecValidation(t, stdout, "recovery-projection", false)
		})
	})

	t.Run("charter remove-spec succeeds while the tracking file remains detached", func(t *testing.T) {
		repoRoot := copyFixtureRepoWithRegistry(t, "ready-spec")

		withWorkingDir(t, repoRoot, func() {
			stdout, stderr, exitCode := executeCLI("charter", "remove-spec", "runtime", "session-lifecycle")
			if exitCode != 0 {
				t.Fatalf("exit code = %d, stderr=%s", exitCode, stderr)
			}
			if stderr != "" {
				t.Fatalf("expected empty stderr, got %q", stderr)
			}

			var envelope struct {
				State struct {
					OrderedSpecs []struct {
						Slug string `json:"slug"`
					} `json:"ordered_specs"`
					Validation struct {
						Valid bool `json:"valid"`
					} `json:"validation"`
				} `json:"state"`
				Result struct {
					RemovedSlug string `json:"removed_slug"`
				} `json:"result"`
			}
			mustUnmarshalJSON(t, stdout, &envelope)
			if envelope.Result.RemovedSlug != "session-lifecycle" {
				t.Fatalf("result = %#v", envelope.Result)
			}
			if !envelope.State.Validation.Valid || len(envelope.State.OrderedSpecs) != 0 {
				t.Fatalf("state = %#v", envelope.State)
			}
			if _, err := os.Stat(filepath.Join(repoRoot, ".specs", "runtime", "session-lifecycle.yaml")); err != nil {
				t.Fatalf("tracking file should remain after remove-spec: %v", err)
			}
		})
	})

	t.Run("charter remove-spec fails when the post-write audit still has errors", func(t *testing.T) {
		repoRoot := copyFixtureRepoWithRegistry(t, "ready-spec")
		replaceInFile(t, filepath.Join(repoRoot, ".specs", "runtime", "CHARTER.yaml"), "specs:\n  - slug: session-lifecycle\n    group: recovery\n    order: 20\n    depends_on: []\n    notes: Session FSM and cleanup behavior\n", "specs:\n  - slug: session-lifecycle\n    group: recovery\n    order: 20\n    depends_on: []\n    notes: Session FSM and cleanup behavior\n  - slug: missing-tracking\n    group: recovery\n    order: 30\n    depends_on: []\n    notes: Missing tracking file\n")

		withWorkingDir(t, repoRoot, func() {
			stdout, stderr, exitCode := executeCLI("charter", "remove-spec", "runtime", "session-lifecycle")
			if exitCode == 0 {
				t.Fatalf("expected failure, stdout=%q stderr=%q", stdout, stderr)
			}

			envelope := requireFailureEnvelope(t, stdout, stderr)
			if envelope.Error == nil || envelope.Error.Code != "VALIDATION_FAILED" {
				t.Fatalf("unexpected envelope %#v", envelope)
			}
			if envelope.State["name"] != "runtime" || envelope.State["tracking_file"] != ".specs/runtime/CHARTER.yaml" {
				t.Fatalf("state = %#v", envelope.State)
			}
			if validation := envelope.State["validation"].(map[string]any); validation["valid"] != false {
				t.Fatalf("state.validation = %#v", validation)
			}
			requireFindingCode(t, envelope.State["focus"].(map[string]any)["validation"].(map[string]any)["findings"].([]any), "CHARTER_SPEC_MISSING")
		})
	})

	t.Run("config add-tag rebuilds success state from persisted config only", func(t *testing.T) {
		repoRoot := copyFixtureRepoWithRegistry(t, "ready-spec")
		writeProjectConfigFixture(t, repoRoot, "gherkin_tags:\n  - runtime\n  - manual\nsource_prefixes:\n  - runtime/src/\nformats:\n  ui-spec:\n    template: ui/src/routes/SPEC-FORMAT.md\n    recommended_for: ui/src/routes/**\n    description: UI spec\n")
		if err := os.MkdirAll(filepath.Join(repoRoot, "ui", "src", "routes"), 0755); err != nil {
			t.Fatalf("mkdir ui routes: %v", err)
		}
		if err := os.WriteFile(filepath.Join(repoRoot, "ui", "src", "routes", "SPEC-FORMAT.md"), []byte("# UI\n"), 0644); err != nil {
			t.Fatalf("write format template: %v", err)
		}

		withWorkingDir(t, repoRoot, func() {
			stdout, stderr, exitCode := executeCLI("config", "add-tag", "adapter")
			if exitCode != 0 {
				t.Fatalf("exit code = %d, stderr=%s", exitCode, stderr)
			}
			if stderr != "" {
				t.Fatalf("expected empty stderr, got %q", stderr)
			}

			var envelope struct {
				State struct {
					GherkinTags []string       `json:"gherkin_tags"`
					Formats     map[string]any `json:"formats"`
					Validation  struct {
						Valid    bool `json:"valid"`
						Findings []struct {
							Code string `json:"code"`
						} `json:"findings"`
					} `json:"validation"`
					Focus struct {
						ConfigMutation struct {
							Kind  string `json:"kind"`
							Value string `json:"value"`
						} `json:"config_mutation"`
					}
				} `json:"state"`
				Focus struct {
					ConfigMutation struct {
						Kind  string `json:"kind"`
						Value string `json:"value"`
					} `json:"config_mutation"`
				} `json:"focus"`
				Result struct {
					Kind     string `json:"kind"`
					Mutation string `json:"mutation"`
					Tag      string `json:"tag"`
				} `json:"result"`
			}
			mustUnmarshalJSON(t, stdout, &envelope)
			if envelope.Result.Kind != "config" || envelope.Result.Mutation != "add-tag" || envelope.Result.Tag != "adapter" {
				t.Fatalf("unexpected result %#v", envelope.Result)
			}
			var rawEnvelope map[string]any
			mustUnmarshalJSON(t, stdout, &rawEnvelope)
			requireObjectKeys(t, rawEnvelope["result"].(map[string]any), "kind", "mutation", "tag")
			if !slices.Contains(envelope.State.GherkinTags, "adapter") {
				t.Fatalf("gherkin_tags = %v", envelope.State.GherkinTags)
			}
			if envelope.Focus.ConfigMutation.Kind != "add_tag" || envelope.Focus.ConfigMutation.Value != "adapter" {
				t.Fatalf("focus = %#v", envelope.Focus.ConfigMutation)
			}
			if !envelope.State.Validation.Valid || len(envelope.State.Validation.Findings) != 0 {
				t.Fatalf("validation = %#v", envelope.State.Validation)
			}
			format, ok := envelope.State.Formats["ui-spec"].(map[string]any)
			if !ok {
				t.Fatalf("formats = %#v", envelope.State.Formats)
			}
			if format["template"] != "ui/src/routes/SPEC-FORMAT.md" || format["recommended_for"] != "ui/src/routes/**" || format["description"] != "UI spec" {
				t.Fatalf("format = %#v", format)
			}

			content, err := os.ReadFile(filepath.Join(repoRoot, ".specs", "specctl.yaml"))
			if err != nil {
				t.Fatalf("read specctl.yaml: %v", err)
			}
			if strings.Contains(string(content), "\n  - manual\n") {
				t.Fatalf("expected persisted semantic tags to be normalized away:\n%s", content)
			}
		})
	})

	t.Run("config remove-prefix returns enriched validation projection", func(t *testing.T) {
		repoRoot := copyFixtureRepoWithRegistry(t, "ready-spec")

		withWorkingDir(t, repoRoot, func() {
			stdout, stderr, exitCode := executeCLI("config", "remove-prefix", "runtime/src/")
			if exitCode != 0 {
				t.Fatalf("exit code = %d, stderr=%s", exitCode, stderr)
			}
			if stderr != "" {
				t.Fatalf("expected empty stderr, got %q", stderr)
			}

			var envelope struct {
				State struct {
					SourcePrefixes []string `json:"source_prefixes"`
					Validation     struct {
						Valid    bool `json:"valid"`
						Findings []struct {
							Code string `json:"code"`
						} `json:"findings"`
					} `json:"validation"`
					Focus struct {
						ConfigMutation struct {
							Kind  string `json:"kind"`
							Value string `json:"value"`
						} `json:"config_mutation"`
					}
				} `json:"state"`
				Focus struct {
					ConfigMutation struct {
						Kind  string `json:"kind"`
						Value string `json:"value"`
					} `json:"config_mutation"`
				} `json:"focus"`
				Result struct {
					Kind     string `json:"kind"`
					Mutation string `json:"mutation"`
					Prefix   string `json:"prefix"`
				} `json:"result"`
			}
			mustUnmarshalJSON(t, stdout, &envelope)
			if envelope.Result.Kind != "config" || envelope.Result.Mutation != "remove-prefix" || envelope.Result.Prefix != "runtime/src/" {
				t.Fatalf("result = %#v", envelope.Result)
			}
			var rawEnvelope map[string]any
			mustUnmarshalJSON(t, stdout, &rawEnvelope)
			requireObjectKeys(t, rawEnvelope["result"].(map[string]any), "kind", "mutation", "prefix")
			if len(envelope.State.SourcePrefixes) != 0 {
				t.Fatalf("source_prefixes = %#v", envelope.State.SourcePrefixes)
			}
			if envelope.Focus.ConfigMutation.Kind != "remove_prefix" || envelope.Focus.ConfigMutation.Value != "runtime/src/" {
				t.Fatalf("focus = %#v", envelope.Focus.ConfigMutation)
			}
			if !envelope.State.Validation.Valid || len(envelope.State.Validation.Findings) != 0 {
				t.Fatalf("validation = %#v", envelope.State.Validation)
			}
		})
	})

	t.Run("config remove-prefix malformed path reuses prefix-not-found error surface", func(t *testing.T) {
		repoRoot := copyFixtureRepoWithRegistry(t, "ready-spec")

		withWorkingDir(t, repoRoot, func() {
			stdout, stderr, exitCode := executeCLI("config", "remove-prefix", "../outside")
			if exitCode == 0 {
				t.Fatalf("expected failure, stdout=%q stderr=%q", stdout, stderr)
			}

			envelope := requireFailureEnvelope(t, stdout, stderr)
			if envelope.Error == nil || envelope.Error.Code != "PREFIX_NOT_FOUND" {
				t.Fatalf("unexpected envelope %#v", envelope)
			}
			focus := envelope.State["focus"].(map[string]any)
			configMutation := focus["config_mutation"].(map[string]any)
			if configMutation["kind"] != "remove_prefix" || configMutation["value"] != "../outside" {
				t.Fatalf("config_mutation = %#v", configMutation)
			}
			if got := strings.Join(stringSliceFromAny(t, focus["invalid_paths"]), ","); got != "../outside" {
				t.Fatalf("invalid_paths = %#v", focus["invalid_paths"])
			}
		})
	})

	t.Run("config remove-tag returns exact result shape", func(t *testing.T) {
		repoRoot := copyFixtureRepoWithRegistry(t, "ready-spec")

		withWorkingDir(t, repoRoot, func() {
			stdout, stderr, exitCode := executeCLI("config", "remove-tag", "domain")
			if exitCode != 0 {
				t.Fatalf("exit code = %d, stderr=%s stdout=%s", exitCode, stderr, stdout)
			}

			var envelope map[string]any
			mustUnmarshalJSON(t, stdout, &envelope)
			result := envelope["result"].(map[string]any)
			requireObjectKeys(t, result, "kind", "mutation", "tag")
			if result["kind"] != "config" || result["mutation"] != "remove-tag" || result["tag"] != "domain" {
				t.Fatalf("result = %#v", result)
			}
			focus := envelope["focus"].(map[string]any)["config_mutation"].(map[string]any)
			if focus["kind"] != "remove_tag" || focus["value"] != "domain" {
				t.Fatalf("focus = %#v", focus)
			}
			if stderr != "" {
				t.Fatalf("expected empty stderr, got %q", stderr)
			}
		})
	})

	t.Run("config add-prefix returns exact result shape", func(t *testing.T) {
		repoRoot := copyFixtureRepoWithRegistry(t, "ready-spec")

		withWorkingDir(t, repoRoot, func() {
			stdout, stderr, exitCode := executeCLI("config", "add-prefix", "runtime/src/application")
			if exitCode != 0 {
				t.Fatalf("exit code = %d, stderr=%s stdout=%s", exitCode, stderr, stdout)
			}

			var envelope map[string]any
			mustUnmarshalJSON(t, stdout, &envelope)
			result := envelope["result"].(map[string]any)
			requireObjectKeys(t, result, "kind", "mutation", "prefix")
			if result["kind"] != "config" || result["mutation"] != "add-prefix" || result["prefix"] != "runtime/src/application/" {
				t.Fatalf("result = %#v", result)
			}
			focus := envelope["focus"].(map[string]any)["config_mutation"].(map[string]any)
			if focus["kind"] != "add_prefix" || focus["value"] != "runtime/src/application/" {
				t.Fatalf("focus = %#v", focus)
			}
			if stderr != "" {
				t.Fatalf("expected empty stderr, got %q", stderr)
			}
		})
	})

	t.Run("config add-tag fails when the post-write audit still has errors", func(t *testing.T) {
		repoRoot := copyFixtureRepoWithRegistry(t, "ready-spec")
		if err := os.Remove(filepath.Join(repoRoot, ".specs", "runtime", "CHARTER.yaml")); err != nil {
			t.Fatalf("remove charter: %v", err)
		}

		withWorkingDir(t, repoRoot, func() {
			stdout, stderr, exitCode := executeCLI("config", "add-tag", "adapter")
			if exitCode == 0 {
				t.Fatalf("expected failure, stdout=%q stderr=%q", stdout, stderr)
			}

			envelope := requireFailureEnvelope(t, stdout, stderr)
			if envelope.Error == nil || envelope.Error.Code != "VALIDATION_FAILED" {
				t.Fatalf("unexpected envelope %#v", envelope)
			}
			if gherkinTags := stringSliceFromAny(t, envelope.State["gherkin_tags"]); strings.Join(gherkinTags, ",") != "runtime,domain" {
				t.Fatalf("gherkin_tags = %#v", gherkinTags)
			}
			focus := envelope.State["focus"].(map[string]any)
			if mutation := focus["config_mutation"].(map[string]any); mutation["kind"] != "add_tag" || mutation["value"] != "adapter" {
				t.Fatalf("config_mutation = %#v", mutation)
			}
			requireFindingCode(t, focus["validation"].(map[string]any)["findings"].([]any), "CHARTER_SPEC_MISSING")
			configBytes, readErr := os.ReadFile(filepath.Join(repoRoot, ".specs", "specctl.yaml"))
			if readErr != nil {
				t.Fatalf("read config after failure: %v", readErr)
			}
			if strings.Contains(string(configBytes), "adapter") {
				t.Fatalf("config should remain unchanged:\n%s", configBytes)
			}
		})
	})

	t.Run("config add-tag rejects invalid tag without rewriting config", func(t *testing.T) {
		repoRoot := copyFixtureRepoWithRegistry(t, "ready-spec")
		configPath := filepath.Join(repoRoot, ".specs", "specctl.yaml")
		before, err := os.ReadFile(configPath)
		if err != nil {
			t.Fatalf("read config before mutation: %v", err)
		}

		withWorkingDir(t, repoRoot, func() {
			stdout, stderr, exitCode := executeCLI("config", "add-tag", "BADTAG")
			if exitCode == 0 {
				t.Fatalf("expected failure, stdout=%q", stdout)
			}

			envelope := requireFailureEnvelope(t, stdout, stderr)
			if envelope.Error == nil || envelope.Error.Code != "VALIDATION_FAILED" {
				t.Fatalf("unexpected envelope %#v", envelope)
			}
			if focus, ok := envelope.State["focus"].(map[string]any); !ok {
				t.Fatalf("focus = %#v", envelope.State["focus"])
			} else {
				mutation := focus["config_mutation"].(map[string]any)
				if mutation["kind"] != "add_tag" || mutation["value"] != "BADTAG" {
					t.Fatalf("config_mutation = %#v", mutation)
				}
				validation := focus["validation"].(map[string]any)
				findings := validation["findings"].([]any)
				if len(findings) != 1 {
					t.Fatalf("validation.findings = %#v", findings)
				}
				finding := findings[0].(map[string]any)
				if finding["code"] != "CONFIG_TAG_INVALID" || finding["target"] != "gherkin_tags" {
					t.Fatalf("validation.finding = %#v", finding)
				}
			}
			validation := envelope.State["validation"].(map[string]any)
			if validation["valid"] != true {
				t.Fatalf("validation = %#v", validation)
			}
			findings, ok := validation["findings"].([]any)
			if !ok || len(findings) != 0 {
				t.Fatalf("validation.findings = %#v", validation["findings"])
			}
			gherkinTags := stringSliceFromAny(t, envelope.State["gherkin_tags"])
			if strings.Join(gherkinTags, ",") != "runtime,domain" {
				t.Fatalf("gherkin_tags = %#v", gherkinTags)
			}

			after, readErr := os.ReadFile(configPath)
			if readErr != nil {
				t.Fatalf("read config after mutation: %v", readErr)
			}
			if string(after) != string(before) {
				t.Fatalf("config was rewritten on validation failure:\nBEFORE:\n%s\nAFTER:\n%s", before, after)
			}
		})
	})

	t.Run("hook classifies ignored unmatched and affected files", func(t *testing.T) {
		repoRoot := copyFixtureRepoWithRegistry(t, "ready-spec")
		if err := os.MkdirAll(filepath.Join(repoRoot, "runtime", "src", "orphan"), 0755); err != nil {
			t.Fatalf("mkdir orphan: %v", err)
		}
		if err := os.WriteFile(filepath.Join(repoRoot, "runtime", "src", "orphan", "worker.py"), []byte("pass\n"), 0644); err != nil {
			t.Fatalf("write orphan file: %v", err)
		}
		if err := os.MkdirAll(filepath.Join(repoRoot, "docs"), 0755); err != nil {
			t.Fatalf("mkdir docs: %v", err)
		}
		if err := os.WriteFile(filepath.Join(repoRoot, "docs", "notes.md"), []byte("# Notes\n"), 0644); err != nil {
			t.Fatalf("write ignored file: %v", err)
		}

		withWorkingDir(t, repoRoot, func() {
			stdout, stderr, exitCode := executeCLIWithInput("runtime/src/domain/session_execution/services.py\nruntime/src/orphan/worker.py\ndocs/notes.md\n", "hook")
			if exitCode != 0 {
				t.Fatalf("exit code = %d, stderr=%s", exitCode, stderr)
			}
			if stderr != "" {
				t.Fatalf("expected empty stderr, got %q", stderr)
			}

			var envelope struct {
				State struct {
					IgnoredFiles   []string `json:"ignored_files"`
					UnmatchedFiles []string `json:"unmatched_files"`
					AffectedSpecs  []struct {
						Slug         string   `json:"slug"`
						MatchedFiles []string `json:"matched_files"`
					} `json:"affected_specs"`
					Validation struct {
						Valid    bool `json:"valid"`
						Findings []struct {
							Code string `json:"code"`
						} `json:"findings"`
					} `json:"validation"`
				} `json:"state"`
			}
			mustUnmarshalJSON(t, stdout, &envelope)
			if strings.Join(envelope.State.IgnoredFiles, ",") != "docs/notes.md" {
				t.Fatalf("ignored_files = %v", envelope.State.IgnoredFiles)
			}
			if strings.Join(envelope.State.UnmatchedFiles, ",") != "runtime/src/orphan/worker.py" {
				t.Fatalf("unmatched_files = %v", envelope.State.UnmatchedFiles)
			}
			if len(envelope.State.AffectedSpecs) != 1 || envelope.State.AffectedSpecs[0].Slug != "session-lifecycle" {
				t.Fatalf("affected_specs = %#v", envelope.State.AffectedSpecs)
			}
			if !slices.Contains(envelope.State.AffectedSpecs[0].MatchedFiles, "runtime/src/domain/session_execution/services.py") {
				t.Fatalf("matched_files = %v", envelope.State.AffectedSpecs[0].MatchedFiles)
			}
			if envelope.State.Validation.Valid || len(envelope.State.Validation.Findings) != 1 || envelope.State.Validation.Findings[0].Code != "UNOWNED_SOURCE_FILE" {
				t.Fatalf("validation = %#v", envelope.State.Validation)
			}
		})
	})

	t.Run("hook respects default source prefixes without specctl yaml", func(t *testing.T) {
		repoRoot := t.TempDir()
		if err := os.MkdirAll(filepath.Join(repoRoot, ".specs", "runtime"), 0o755); err != nil {
			t.Fatalf("mkdir specs: %v", err)
		}
		if err := os.MkdirAll(filepath.Join(repoRoot, "ui", "convex", "session_execution"), 0o755); err != nil {
			t.Fatalf("mkdir ui/convex: %v", err)
		}
		if err := os.WriteFile(filepath.Join(repoRoot, "ui", "convex", "session_execution", "mutations.ts"), []byte("export {};\n"), 0o644); err != nil {
			t.Fatalf("write source file: %v", err)
		}
		if err := os.WriteFile(filepath.Join(repoRoot, ".specs", "runtime", "CHARTER.yaml"), []byte(`name: runtime
title: Runtime System
description: Specs for runtime control-plane and data-plane behavior
groups:
  - key: ui
    title: UI
    order: 10
specs:
  - slug: session-sync
    group: ui
    order: 10
    depends_on: []
    notes: Convex session sync behavior
`), 0o644); err != nil {
			t.Fatalf("write charter: %v", err)
		}
		if err := os.WriteFile(filepath.Join(repoRoot, ".specs", "runtime", "session-sync.yaml"), []byte(`slug: session-sync
charter: runtime
title: Session Sync
status: draft
rev: 1
created: 2026-03-05
updated: 2026-03-30
last_verified_at: 2026-03-28
checkpoint: a1b2c3f
tags:
  - runtime
documents:
  primary: ui/convex/session_execution/SPEC.md
scope:
  - ui/convex/session_execution/
deltas: []
requirements: []
changelog: []
`), 0o644); err != nil {
			t.Fatalf("write tracking: %v", err)
		}

		withWorkingDir(t, repoRoot, func() {
			stdout, stderr, exitCode := executeCLIWithInput("ui/convex/session_execution/mutations.ts\n", "hook")
			if exitCode != 0 {
				t.Fatalf("exit code = %d, stderr=%s stdout=%s", exitCode, stderr, stdout)
			}
			if stderr != "" {
				t.Fatalf("expected empty stderr, got %q", stderr)
			}

			var envelope struct {
				State struct {
					IgnoredFiles   []string `json:"ignored_files"`
					UnmatchedFiles []string `json:"unmatched_files"`
					AffectedSpecs  []struct {
						Slug         string   `json:"slug"`
						MatchedFiles []string `json:"matched_files"`
					} `json:"affected_specs"`
				} `json:"state"`
			}
			mustUnmarshalJSON(t, stdout, &envelope)
			if len(envelope.State.IgnoredFiles) != 0 || len(envelope.State.UnmatchedFiles) != 0 {
				t.Fatalf("state = %#v", envelope.State)
			}
			if len(envelope.State.AffectedSpecs) != 1 || envelope.State.AffectedSpecs[0].Slug != "session-sync" {
				t.Fatalf("affected_specs = %#v", envelope.State.AffectedSpecs)
			}
			if got := strings.Join(envelope.State.AffectedSpecs[0].MatchedFiles, ","); got != "ui/convex/session_execution/mutations.ts" {
				t.Fatalf("matched_files = %#v", envelope.State.AffectedSpecs[0].MatchedFiles)
			}
		})
	})

	t.Run("hook reuses longest-prefix ownership resolution", func(t *testing.T) {
		repoRoot := t.TempDir()
		if err := os.MkdirAll(filepath.Join(repoRoot, ".specs", "runtime"), 0o755); err != nil {
			t.Fatalf("mkdir specs: %v", err)
		}
		if err := os.MkdirAll(filepath.Join(repoRoot, "runtime", "src", "domain"), 0o755); err != nil {
			t.Fatalf("mkdir domain: %v", err)
		}
		if err := os.WriteFile(filepath.Join(repoRoot, ".specs", "specctl.yaml"), []byte("source_prefixes:\n  - runtime/src/\n"), 0o644); err != nil {
			t.Fatalf("write config: %v", err)
		}
		if err := os.WriteFile(filepath.Join(repoRoot, "runtime", "src", "domain", "service.go"), []byte("package domain\n"), 0o644); err != nil {
			t.Fatalf("write service: %v", err)
		}
		if err := os.WriteFile(filepath.Join(repoRoot, ".specs", "runtime", "CHARTER.yaml"), []byte(`name: runtime
title: Runtime System
description: Specs for runtime control-plane and data-plane behavior
groups:
  - key: platform
    title: Platform
    order: 10
specs:
  - slug: broad-platform
    group: platform
    order: 10
    depends_on: []
    notes: Broad runtime platform
  - slug: domain-platform
    group: platform
    order: 20
    depends_on: []
    notes: Domain runtime platform
`), 0o644); err != nil {
			t.Fatalf("write charter: %v", err)
		}
		if err := os.WriteFile(filepath.Join(repoRoot, ".specs", "runtime", "broad-platform.yaml"), []byte(`slug: broad-platform
charter: runtime
title: Broad Platform
status: draft
rev: 1
created: 2026-03-05
updated: 2026-03-30
last_verified_at: 2026-03-28
checkpoint: a1b2c3f
tags:
  - runtime
documents:
  primary: runtime/src/SPEC.md
scope:
  - runtime/src/
deltas: []
requirements: []
changelog: []
`), 0o644); err != nil {
			t.Fatalf("write broad tracking: %v", err)
		}
		if err := os.WriteFile(filepath.Join(repoRoot, ".specs", "runtime", "domain-platform.yaml"), []byte(`slug: domain-platform
charter: runtime
title: Domain Platform
status: draft
rev: 1
created: 2026-03-05
updated: 2026-03-30
last_verified_at: 2026-03-28
checkpoint: a1b2c3f
tags:
  - runtime
documents:
  primary: runtime/src/domain/SPEC.md
scope:
  - runtime/src/domain/
deltas: []
requirements: []
changelog: []
`), 0o644); err != nil {
			t.Fatalf("write narrow tracking: %v", err)
		}

		withWorkingDir(t, repoRoot, func() {
			stdout, stderr, exitCode := executeCLIWithInput("runtime/src/domain/service.go\n", "hook")
			if exitCode != 0 {
				t.Fatalf("exit code = %d, stderr=%s", exitCode, stderr)
			}
			if stderr != "" {
				t.Fatalf("expected empty stderr, got %q", stderr)
			}

			var envelope struct {
				State struct {
					UnmatchedFiles []string `json:"unmatched_files"`
					AffectedSpecs  []struct {
						Slug         string   `json:"slug"`
						MatchedFiles []string `json:"matched_files"`
					} `json:"affected_specs"`
				} `json:"state"`
			}
			mustUnmarshalJSON(t, stdout, &envelope)
			if len(envelope.State.UnmatchedFiles) != 0 {
				t.Fatalf("unmatched_files = %#v", envelope.State.UnmatchedFiles)
			}
			if len(envelope.State.AffectedSpecs) != 1 || envelope.State.AffectedSpecs[0].Slug != "domain-platform" {
				t.Fatalf("affected_specs = %#v", envelope.State.AffectedSpecs)
			}
			if got := strings.Join(envelope.State.AffectedSpecs[0].MatchedFiles, ","); got != "runtime/src/domain/service.go" {
				t.Fatalf("matched_files = %v", envelope.State.AffectedSpecs[0].MatchedFiles)
			}
		})
	})

	t.Run("hook marks managed tracking files as staged on the owning spec", func(t *testing.T) {
		repoRoot := copyFixtureRepoWithRegistry(t, "ready-spec")
		if err := os.WriteFile(filepath.Join(repoRoot, "runtime", "src", "domain", "session_execution", "services.py"), []byte("pass\n"), 0644); err != nil {
			t.Fatalf("write service file: %v", err)
		}

		withWorkingDir(t, repoRoot, func() {
			stdout, stderr, exitCode := executeCLIWithInput(".specs/runtime/session-lifecycle.yaml\nruntime/src/domain/session_execution/services.py\n", "hook")
			if exitCode != 0 {
				t.Fatalf("exit code = %d, stderr=%s", exitCode, stderr)
			}
			if stderr != "" {
				t.Fatalf("expected empty stderr, got %q", stderr)
			}

			var envelope struct {
				State struct {
					UnmatchedFiles []string `json:"unmatched_files"`
					AffectedSpecs  []struct {
						Slug               string   `json:"slug"`
						TrackingFileStaged bool     `json:"tracking_file_staged"`
						MatchedFiles       []string `json:"matched_files"`
					} `json:"affected_specs"`
				} `json:"state"`
			}
			mustUnmarshalJSON(t, stdout, &envelope)
			if len(envelope.State.UnmatchedFiles) != 0 {
				t.Fatalf("unmatched_files = %v", envelope.State.UnmatchedFiles)
			}
			if len(envelope.State.AffectedSpecs) != 1 || !envelope.State.AffectedSpecs[0].TrackingFileStaged {
				t.Fatalf("affected_specs = %#v", envelope.State.AffectedSpecs)
			}
			if got := strings.Join(envelope.State.AffectedSpecs[0].MatchedFiles, ","); got != "runtime/src/domain/session_execution/services.py" {
				t.Fatalf("matched_files = %v", envelope.State.AffectedSpecs[0].MatchedFiles)
			}
		})
	})

	t.Run("hook marks tracked design docs as staged without treating them as matched source files", func(t *testing.T) {
		repoRoot := copyFixtureRepoWithRegistry(t, "ready-spec")

		withWorkingDir(t, repoRoot, func() {
			stdout, stderr, exitCode := executeCLIWithInput("runtime/src/domain/session_execution/SPEC.md\n", "hook")
			if exitCode != 0 {
				t.Fatalf("exit code = %d, stderr=%s", exitCode, stderr)
			}
			if stderr != "" {
				t.Fatalf("expected empty stderr, got %q", stderr)
			}

			var envelope struct {
				State struct {
					UnmatchedFiles []string `json:"unmatched_files"`
					AffectedSpecs  []struct {
						Slug            string   `json:"slug"`
						DesignDocStaged bool     `json:"design_doc_staged"`
						MatchedFiles    []string `json:"matched_files"`
					} `json:"affected_specs"`
				} `json:"state"`
			}
			mustUnmarshalJSON(t, stdout, &envelope)
			if len(envelope.State.UnmatchedFiles) != 0 {
				t.Fatalf("unmatched_files = %v", envelope.State.UnmatchedFiles)
			}
			if len(envelope.State.AffectedSpecs) != 1 || envelope.State.AffectedSpecs[0].Slug != "session-lifecycle" || !envelope.State.AffectedSpecs[0].DesignDocStaged {
				t.Fatalf("affected_specs = %#v", envelope.State.AffectedSpecs)
			}
			if len(envelope.State.AffectedSpecs[0].MatchedFiles) != 0 {
				t.Fatalf("matched_files = %v", envelope.State.AffectedSpecs[0].MatchedFiles)
			}
		})
	})

	t.Run("hook leaves same-charter ownership ties unmatched", func(t *testing.T) {
		repoRoot := t.TempDir()
		if err := os.MkdirAll(filepath.Join(repoRoot, ".specs", "runtime"), 0o755); err != nil {
			t.Fatalf("mkdir specs: %v", err)
		}
		if err := os.MkdirAll(filepath.Join(repoRoot, "runtime", "src", "domain", "shared"), 0o755); err != nil {
			t.Fatalf("mkdir shared: %v", err)
		}
		if err := os.WriteFile(filepath.Join(repoRoot, ".specs", "specctl.yaml"), []byte("source_prefixes:\n  - runtime/src/\n"), 0o644); err != nil {
			t.Fatalf("write config: %v", err)
		}
		if err := os.WriteFile(filepath.Join(repoRoot, "runtime", "src", "domain", "shared", "transport.py"), []byte("pass\n"), 0o644); err != nil {
			t.Fatalf("write transport: %v", err)
		}
		if err := os.WriteFile(filepath.Join(repoRoot, ".specs", "runtime", "CHARTER.yaml"), []byte(`name: runtime
title: Runtime System
description: Specs for runtime control-plane and data-plane behavior
groups:
  - key: recovery
    title: Recovery
    order: 10
specs:
  - slug: first-owner
    group: recovery
    order: 20
    depends_on: []
    notes: First owner
  - slug: second-owner
    group: recovery
    order: 30
    depends_on: []
    notes: Second owner
`), 0o644); err != nil {
			t.Fatalf("write charter: %v", err)
		}
		for _, slug := range []string{"first-owner", "second-owner"} {
			if err := os.WriteFile(filepath.Join(repoRoot, ".specs", "runtime", slug+".yaml"), []byte(`slug: `+slug+`
charter: runtime
title: `+slug+`
status: draft
rev: 1
created: 2026-03-05
updated: 2026-03-30
last_verified_at: 2026-03-28
checkpoint: a1b2c3f
tags:
  - runtime
documents:
  primary: runtime/src/domain/shared/`+slug+`/SPEC.md
scope:
  - runtime/src/domain/shared/
deltas: []
requirements: []
changelog: []
`), 0o644); err != nil {
				t.Fatalf("write tracking %s: %v", slug, err)
			}
		}

		withWorkingDir(t, repoRoot, func() {
			stdout, stderr, exitCode := executeCLIWithInput("runtime/src/domain/shared/transport.py\n", "hook")
			if exitCode != 0 {
				t.Fatalf("exit code = %d, stderr=%s", exitCode, stderr)
			}
			if stderr != "" {
				t.Fatalf("expected empty stderr, got %q", stderr)
			}

			var envelope struct {
				State struct {
					UnmatchedFiles []string `json:"unmatched_files"`
					AffectedSpecs  []struct {
						Slug         string   `json:"slug"`
						MatchedFiles []string `json:"matched_files"`
					} `json:"affected_specs"`
					Validation struct {
						Valid    bool `json:"valid"`
						Findings []struct {
							Code string `json:"code"`
							Path string `json:"path"`
						} `json:"findings"`
					} `json:"validation"`
				} `json:"state"`
			}
			mustUnmarshalJSON(t, stdout, &envelope)
			if len(envelope.State.UnmatchedFiles) != 1 || envelope.State.UnmatchedFiles[0] != "runtime/src/domain/shared/transport.py" {
				t.Fatalf("unmatched_files = %v", envelope.State.UnmatchedFiles)
			}
			if len(envelope.State.AffectedSpecs) != 0 {
				t.Fatalf("affected_specs = %#v", envelope.State.AffectedSpecs)
			}
			if envelope.State.Validation.Valid || len(envelope.State.Validation.Findings) != 1 || envelope.State.Validation.Findings[0].Code != "UNOWNED_SOURCE_FILE" || envelope.State.Validation.Findings[0].Path != "runtime/src/domain/shared/transport.py" {
				t.Fatalf("validation = %#v", envelope.State.Validation)
			}
		})
	})

	t.Run("hook attributes managed charter and config files to affected specs", func(t *testing.T) {
		repoRoot := copyFixtureRepoWithRegistry(t, "charter-dag")
		if err := os.MkdirAll(filepath.Join(repoRoot, "runtime", "src", "domain", "session_execution"), 0755); err != nil {
			t.Fatalf("mkdir session_execution: %v", err)
		}
		if err := os.WriteFile(filepath.Join(repoRoot, "runtime", "src", "domain", "session_execution", "services.py"), []byte("pass\n"), 0644); err != nil {
			t.Fatalf("write service file: %v", err)
		}
		if err := os.MkdirAll(filepath.Join(repoRoot, "runtime", "src", "orphan"), 0755); err != nil {
			t.Fatalf("mkdir orphan: %v", err)
		}
		if err := os.WriteFile(filepath.Join(repoRoot, "runtime", "src", "orphan", "worker.py"), []byte("pass\n"), 0644); err != nil {
			t.Fatalf("write orphan file: %v", err)
		}

		withWorkingDir(t, repoRoot, func() {
			stdout, stderr, exitCode := executeCLIWithInput(".specs/runtime/CHARTER.yaml\n.specs/specctl.yaml\nruntime/src/domain/session_execution/services.py\nruntime/src/orphan/worker.py\n", "hook")
			if exitCode != 0 {
				t.Fatalf("exit code = %d, stderr=%s", exitCode, stderr)
			}
			if stderr != "" {
				t.Fatalf("expected empty stderr, got %q", stderr)
			}

			var envelope struct {
				State struct {
					UnmatchedFiles []string `json:"unmatched_files"`
					AffectedSpecs  []struct {
						Slug         string   `json:"slug"`
						MatchedFiles []string `json:"matched_files"`
					} `json:"affected_specs"`
				} `json:"state"`
			}
			mustUnmarshalJSON(t, stdout, &envelope)
			if strings.Join(envelope.State.UnmatchedFiles, ",") != "runtime/src/orphan/worker.py" {
				t.Fatalf("unmatched_files = %v", envelope.State.UnmatchedFiles)
			}
			got := make([]string, 0, len(envelope.State.AffectedSpecs))
			for _, spec := range envelope.State.AffectedSpecs {
				got = append(got, spec.Slug)
			}
			if strings.Join(got, ",") != "recovery-projection,redis-state,session-lifecycle" {
				t.Fatalf("affected_specs = %#v", envelope.State.AffectedSpecs)
			}
			for _, spec := range envelope.State.AffectedSpecs {
				switch spec.Slug {
				case "session-lifecycle":
					if got := strings.Join(spec.MatchedFiles, ","); got != "runtime/src/domain/session_execution/services.py" {
						t.Fatalf("session-lifecycle matched_files = %v", spec.MatchedFiles)
					}
				default:
					if len(spec.MatchedFiles) != 0 {
						t.Fatalf("%s matched_files = %v, want only source-file matches", spec.Slug, spec.MatchedFiles)
					}
				}
			}
		})
	})
}

func copyFixtureRepo(t *testing.T, fixture string) string {
	t.Helper()

	srcRoot := filepath.Join("..", "..", "testdata", "v2", fixture)
	dstRoot := t.TempDir()

	if err := deepCopyDir(srcRoot, dstRoot); err != nil {
		t.Fatalf("copy fixture repo: %v", err)
	}

	return dstRoot
}

func seededSpecRepo(t *testing.T) string {
	t.Helper()

	repoRoot := t.TempDir()
	if err := os.MkdirAll(filepath.Join(repoRoot, ".specs", "runtime"), 0755); err != nil {
		t.Fatalf("mkdir specs dir: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(repoRoot, "runtime", "src", "domain", "session_execution"), 0755); err != nil {
		t.Fatalf("mkdir spec dir: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(repoRoot, "runtime", "src", "application", "commands"), 0755); err != nil {
		t.Fatalf("mkdir scope dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(repoRoot, "runtime", "src", "domain", "session_execution", "SPEC.md"), []byte("---\nspec: session-lifecycle\ncharter: runtime\n---\n# Session Lifecycle\n\n## Requirement: Compensation stage 4 failure cleanup\n\n```gherkin requirement\n@runtime @e2e\nFeature: Compensation stage 4 failure cleanup\n```\n\n### Scenarios\n\n```gherkin scenario\nScenario: Cleanup runs after stage 4 failure\n  Given stage 4 fails during compensation\n  When recovery completes\n  Then cleanup steps run in documented order\n```\n"), 0644); err != nil {
		t.Fatalf("write design doc: %v", err)
	}
	if err := os.WriteFile(filepath.Join(repoRoot, ".specs", "runtime", "session-lifecycle.yaml"), []byte(`slug: session-lifecycle
charter: runtime
title: Session Lifecycle
status: active
rev: 2
created: 2026-03-05
updated: 2026-03-30
last_verified_at: 2026-03-28
checkpoint: a1b2c3f
tags:
  - runtime
  - domain
documents:
  primary: runtime/src/domain/session_execution/SPEC.md
scope:
  - runtime/src/domain/session_execution/
  - runtime/src/application/commands/
deltas:
  - id: D-001
    area: Compensation stage 4
    status: in-progress
    origin_checkpoint: a1b2c3f
    current: Stage 4 compensation exists in code but failure ordering is unclear
    target: Document ordering and verify failure cleanup
    notes: Multi-agent implementation split between runtime and workflow work
requirements:
  - id: REQ-001
    title: Compensation stage 4 failure cleanup
    tags:
      - runtime
      - e2e
    test_files: []
    lifecycle: active
    verification: unverified
    introduced_by: D-001
    gherkin: |
      @runtime @e2e
      Feature: Compensation stage 4 failure cleanup
changelog:
  - rev: 1
    date: 2026-03-28
    deltas_opened:
      - D-001
    deltas_closed: []
    reqs_added:
      - REQ-001
    reqs_verified: []
    summary: Opened the compensation cleanup work
`), 0644); err != nil {
		t.Fatalf("write tracking file: %v", err)
	}
	if err := os.WriteFile(filepath.Join(repoRoot, ".specs", "runtime", "CHARTER.yaml"), []byte(`name: runtime
title: Runtime System
description: Specs for runtime control-plane and data-plane behavior
groups:
  - key: execution
    title: Execution Engine
    order: 10
  - key: recovery
    title: Recovery and Cleanup
    order: 20
specs:
  - slug: session-lifecycle
    group: recovery
    order: 20
    depends_on: []
    notes: Session FSM and cleanup behavior
`), 0644); err != nil {
		t.Fatalf("write charter file: %v", err)
	}

	initGitRepoAtDate(t, repoRoot, "2026-03-28T12:00:00Z")
	return repoRoot
}

func charterOnlyRepo(t *testing.T) string {
	t.Helper()

	repoRoot := t.TempDir()
	if err := os.MkdirAll(filepath.Join(repoRoot, ".specs", "runtime"), 0755); err != nil {
		t.Fatalf("mkdir specs dir: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(repoRoot, "runtime", "src", "domain", "session_execution"), 0755); err != nil {
		t.Fatalf("mkdir spec dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(repoRoot, ".specs", "runtime", "CHARTER.yaml"), []byte(`name: runtime
title: Runtime System
description: Specs for runtime control-plane and data-plane behavior
groups:
  - key: recovery
    title: Recovery and Cleanup
    order: 20
specs: []
`), 0644); err != nil {
		t.Fatalf("write charter: %v", err)
	}
	initGitRepoAtDate(t, repoRoot, "2026-03-28T12:00:00Z")
	return repoRoot
}

func charterOnlyRepoWithoutHead(t *testing.T) string {
	t.Helper()

	repoRoot := t.TempDir()
	if err := os.MkdirAll(filepath.Join(repoRoot, ".specs", "runtime"), 0o755); err != nil {
		t.Fatalf("mkdir specs dir: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(repoRoot, "runtime", "src", "domain", "session_execution"), 0o755); err != nil {
		t.Fatalf("mkdir spec dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(repoRoot, ".specs", "runtime", "CHARTER.yaml"), []byte(`name: runtime
title: Runtime System
description: Specs for runtime control-plane and data-plane behavior
groups:
  - key: recovery
    title: Recovery and Cleanup
    order: 20
specs: []
`), 0o644); err != nil {
		t.Fatalf("write charter: %v", err)
	}
	runGitAtDate(t, repoRoot, "", "init")
	runGitAtDate(t, repoRoot, "", "config", "user.name", "Specctl Tests")
	runGitAtDate(t, repoRoot, "", "config", "user.email", "specctl-tests@example.com")
	return repoRoot
}

func manualRequirementRepo(t *testing.T) string {
	t.Helper()

	repoRoot := copyFixtureRepoWithRegistry(t, "active-spec")
	replaceInFile(t, filepath.Join(repoRoot, ".specs", "runtime", "session-lifecycle.yaml"), "- e2e", "- manual")
	replaceInFile(t, filepath.Join(repoRoot, ".specs", "runtime", "session-lifecycle.yaml"), "@runtime @e2e", "@runtime @manual")
	replaceInFile(t, filepath.Join(repoRoot, "runtime", "src", "domain", "session_execution", "SPEC.md"), "@runtime @e2e", "@runtime @manual")
	commitAllChangesAtDate(t, repoRoot, "2026-03-28T12:00:00Z", "manual requirement fixture")
	return repoRoot
}

func uiReadySpecRepo(t *testing.T) string {
	t.Helper()

	repoRoot := t.TempDir()
	if err := os.MkdirAll(filepath.Join(repoRoot, ".specs", "ui"), 0o755); err != nil {
		t.Fatalf("mkdir ui specs: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(repoRoot, "ui", "src", "domain", "threads"), 0o755); err != nil {
		t.Fatalf("mkdir ui source: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(repoRoot, "ui", "e2e", "tests", "journeys"), 0o755); err != nil {
		t.Fatalf("mkdir ui e2e: %v", err)
	}

	writeProjectConfigFixture(t, repoRoot, "gherkin_tags:\n  - ui\nsource_prefixes:\n  - ui/src/\nformats: {}\n")
	writeCLIAdjacentTestFile(t, filepath.Join(repoRoot, "ui", "src", "domain", "threads", "SPEC.md"), []byte("---\nspec: thread-lifecycle\ncharter: ui\n---\n# Thread Lifecycle\n\n## Requirement: Thread lifecycle\n\n```gherkin requirement\n@ui\nFeature: Thread lifecycle\n```\n\n### Scenarios\n\n```gherkin scenario\nScenario: The workspace tab restores a thread\n  Given a user reopens the workspace\n  When the active thread is restored\n  Then the thread tab shows the latest state\n```\n"))
	writeCLIAdjacentTestFile(t, filepath.Join(repoRoot, ".specs", "ui", "CHARTER.yaml"), []byte(`name: ui
title: UI Platform
description: Specs for the UI application
groups:
  - key: threads
    title: Threads
    order: 10
specs:
  - slug: thread-lifecycle
    group: threads
    order: 10
    depends_on: []
    notes: Thread lifecycle behavior
`))
	writeCLIAdjacentTestFile(t, filepath.Join(repoRoot, ".specs", "ui", "thread-lifecycle.yaml"), []byte(`slug: thread-lifecycle
charter: ui
title: Thread Lifecycle
status: ready
rev: 1
created: 2026-03-05
updated: 2026-03-30
last_verified_at: 2026-03-28
checkpoint: a1b2c3f
tags:
  - ui
documents:
  primary: ui/src/domain/threads/SPEC.md
scope:
  - ui/src/domain/threads/
deltas:
  - id: D-001
    area: Workspace tabs
    status: open
    origin_checkpoint: a1b2c3f
    current: Workspace tabs do not restore the active thread
    target: Restore the active thread tab when the workspace reloads
    notes: Needed for the next thread journey
requirements: []
changelog: []
`))
	initGitRepoAtDate(t, repoRoot, "2026-03-28T12:00:00Z")
	return repoRoot
}

func uiActiveSpecRepo(t *testing.T) string {
	t.Helper()

	repoRoot := uiReadySpecRepo(t)
	replaceInFile(t, filepath.Join(repoRoot, ".specs", "ui", "thread-lifecycle.yaml"), "status: ready", "status: active")
	replaceInFile(t, filepath.Join(repoRoot, ".specs", "ui", "thread-lifecycle.yaml"), "requirements: []", `requirements:
  - id: REQ-001
    title: Thread lifecycle
    tags:
      - ui
      - e2e
    test_files: []
    lifecycle: active
    verification: unverified
    introduced_by: D-001
    gherkin: |
      @ui @e2e
      Feature: Thread lifecycle`)
	replaceInFile(t, filepath.Join(repoRoot, "ui", "src", "domain", "threads", "SPEC.md"), "@ui\nFeature: Thread lifecycle", "@ui @e2e\nFeature: Thread lifecycle")
	commitAllChangesAtDate(t, repoRoot, "2026-03-28T12:00:00Z", "ui active fixture")
	return repoRoot
}

func writeProjectConfigFixture(t *testing.T, repoRoot, content string) {
	t.Helper()

	if err := os.WriteFile(filepath.Join(repoRoot, ".specs", "specctl.yaml"), []byte(content), 0644); err != nil {
		t.Fatalf("write specctl config: %v", err)
	}
}

func withWorkingDir(t *testing.T, dir string, fn func()) {
	t.Helper()

	previous, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir %s: %v", dir, err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(previous)
	})
	fn()
}

func requireFindingCode(t *testing.T, findings []any, code string) {
	t.Helper()

	for _, findingAny := range findings {
		if finding, ok := findingAny.(map[string]any); ok && finding["code"] == code {
			return
		}
	}
	t.Fatalf("findings = %#v, want code %q", findings, code)
}

func replaceInFile(t *testing.T, path, oldValue, newValue string) {
	t.Helper()

	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	updated := strings.Replace(string(content), oldValue, newValue, 1)
	if updated == string(content) && strings.HasPrefix(oldValue, "checkpoint: ") && strings.HasPrefix(newValue, "checkpoint: ") {
		updated = replaceLinePrefix(string(content), "checkpoint: ", newValue)
	}
	if updated == string(content) && strings.HasPrefix(oldValue, "origin_checkpoint: ") && strings.HasPrefix(newValue, "origin_checkpoint: ") {
		updated = replaceLinePrefix(string(content), "origin_checkpoint: ", newValue)
	}
	if updated == string(content) {
		t.Fatalf("did not find %q in %s", oldValue, path)
	}
	if err := os.WriteFile(path, []byte(updated), 0644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func replaceLinePrefix(content, prefix, replacement string) string {
	lines := strings.Split(content, "\n")
	for i, line := range lines {
		if strings.HasPrefix(strings.TrimSpace(line), prefix) {
			indent := line[:len(line)-len(strings.TrimLeft(line, " "))]
			lines[i] = indent + replacement
			return strings.Join(lines, "\n")
		}
	}
	return content
}

func writeCLIAdjacentTestFile(t *testing.T, path string, data []byte) {
	t.Helper()

	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func assertCLIOrderedSpecValidation(t *testing.T, stdout, slug string, wantValid bool) {
	t.Helper()

	var envelope struct {
		State struct {
			OrderedSpecs []struct {
				Slug       string `json:"slug"`
				Validation struct {
					Valid bool `json:"valid"`
				} `json:"validation"`
			} `json:"ordered_specs"`
		} `json:"state"`
	}
	mustUnmarshalJSON(t, stdout, &envelope)
	for _, entry := range envelope.State.OrderedSpecs {
		if entry.Slug != slug {
			continue
		}
		if entry.Validation.Valid != wantValid {
			t.Fatalf("ordered spec %s validation = %#v, want valid=%v", slug, entry.Validation, wantValid)
		}
		return
	}
	t.Fatalf("ordered spec %s missing from output", slug)
}

func initGitRepo(t *testing.T, repoRoot string) string {
	t.Helper()

	return initGitRepoAtDate(t, repoRoot, "2026-03-30T12:00:00Z")
}

func initGitRepoAtDate(t *testing.T, repoRoot, timestamp string) string {
	t.Helper()

	if !gitHeadExists(t, repoRoot) {
		runGitAtDate(t, repoRoot, timestamp, "init")
	}
	runGitAtDate(t, repoRoot, timestamp, "config", "user.name", "Specctl Tests")
	runGitAtDate(t, repoRoot, timestamp, "config", "user.email", "specctl-tests@example.com")
	commitAllChangesAtDate(t, repoRoot, timestamp, "fixture")
	head := strings.TrimSpace(runGitAtDate(t, repoRoot, timestamp, "rev-parse", "HEAD"))
	rewriteTrackingCheckpoints(t, repoRoot, head)
	commitAllChangesAtDate(t, repoRoot, timestamp, "rewrite checkpoints")
	return strings.TrimSpace(runGitAtDate(t, repoRoot, timestamp, "rev-parse", "HEAD"))
}

func gitHeadExists(t *testing.T, repoRoot string) bool {
	t.Helper()

	cmd := exec.Command("git", "rev-parse", "--verify", "HEAD")
	cmd.Dir = repoRoot
	return cmd.Run() == nil
}

func commitAllChangesAtDate(t *testing.T, repoRoot, timestamp, message string) {
	t.Helper()

	runGitAtDate(t, repoRoot, timestamp, "add", ".")
	if strings.TrimSpace(runGitAtDate(t, repoRoot, timestamp, "status", "--porcelain")) == "" {
		return
	}
	runGitAtDate(t, repoRoot, timestamp, "commit", "-m", message)
}

func runGit(t *testing.T, repoRoot string, args ...string) string {
	t.Helper()
	return runGitAtDate(t, repoRoot, "", args...)
}

func runGitAtDate(t *testing.T, repoRoot, timestamp string, args ...string) string {
	t.Helper()

	cmd := exec.Command("git", args...)
	cmd.Dir = repoRoot
	if timestamp != "" {
		cmd.Env = append(cmd.Environ(), "GIT_AUTHOR_DATE="+timestamp, "GIT_COMMITTER_DATE="+timestamp)
	}
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v failed: %v\n%s", args, err, output)
	}
	return string(output)
}

func mustUnmarshalJSON(t *testing.T, input string, target any) {
	t.Helper()
	if err := json.Unmarshal([]byte(input), target); err != nil {
		t.Fatalf("unmarshal %q: %v", input, err)
	}
}

func rewriteTrackingCheckpoints(t *testing.T, repoRoot, checkpoint string) {
	t.Helper()

	specsRoot := filepath.Join(repoRoot, ".specs")
	filepath.Walk(specsRoot, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || filepath.Ext(path) != ".yaml" || filepath.Base(path) == "CHARTER.yaml" {
			return err
		}
		content, readErr := os.ReadFile(path)
		if readErr != nil {
			t.Fatalf("read %s: %v", path, readErr)
		}
		updated := strings.ReplaceAll(string(content), "checkpoint: a1b2c3f", "checkpoint: "+checkpoint)
		updated = strings.ReplaceAll(updated, "origin_checkpoint: a1b2c3f", "origin_checkpoint: "+checkpoint)
		if updated != string(content) {
			if writeErr := os.WriteFile(path, []byte(updated), 0644); writeErr != nil {
				t.Fatalf("write %s: %v", path, writeErr)
			}
		}
		return nil
	})
}

func requireSpecStateShape(t *testing.T, state map[string]any, wantStatus string) {
	t.Helper()

	if got := state["status"]; got != wantStatus {
		t.Fatalf("status = %#v, want %q", got, wantStatus)
	}
	if got := state["tracking_file"]; got != ".specs/runtime/session-lifecycle.yaml" {
		t.Fatalf("tracking_file = %#v", got)
	}
	validation := state["validation"].(map[string]any)
	if _, ok := validation["valid"]; !ok {
		t.Fatalf("validation = %#v", validation)
	}
}

func requireValidationFailure(t *testing.T, state map[string]any) []any {
	t.Helper()

	validation := state["validation"].(map[string]any)
	if validation["valid"] != false {
		t.Fatalf("validation.valid = %#v, want false", validation["valid"])
	}
	findings, ok := validation["findings"].([]any)
	if !ok || len(findings) == 0 {
		t.Fatalf("validation.findings = %#v", validation["findings"])
	}
	return findings
}

func requireObjectKeys(t *testing.T, value map[string]any, want ...string) {
	t.Helper()

	got := make([]string, 0, len(value))
	for key := range value {
		got = append(got, key)
	}
	slices.Sort(got)
	expected := append([]string{}, want...)
	slices.Sort(expected)
	if !slices.Equal(got, expected) {
		t.Fatalf("keys = %v, want %v (value=%#v)", got, expected, value)
	}
}

func requireValidationCatalogCode(t *testing.T, code any) {
	t.Helper()

	allowed := map[string]struct{}{
		"SPEC_SLUG_INVALID":                {},
		"SPEC_CHARTER_INVALID":             {},
		"SPEC_TITLE_INVALID":               {},
		"SPEC_TAG_INVALID":                 {},
		"SPEC_STATUS_INVALID":              {},
		"SPEC_STATUS_MISMATCH":             {},
		"REV_INVALID":                      {},
		"CHECKPOINT_INVALID":               {},
		"PRIMARY_DOC_MISSING":              {},
		"PRIMARY_DOC_FRONTMATTER_INVALID":  {},
		"PRIMARY_DOC_FRONTMATTER_MISMATCH": {},
		"FORMAT_NOT_CONFIGURED":            {},
		"SCOPE_EMPTY":                      {},
		"SCOPE_PATH_INVALID":               {},
		"DELTA_ID_INVALID":                 {},
		"REQUIREMENT_ID_INVALID":           {},
		"IDS_NON_SEQUENTIAL":               {},
		"DELTA_FIELD_INVALID":              {},
		"DELTA_UNTRACED":                   {},
		"REQUIREMENT_GHERKIN_INVALID":      {},
		"REQUIREMENT_TRACE_INVALID":        {},
		"REQUIREMENT_TAG_NOT_CONFIGURED":   {},
		"REQUIREMENT_TEST_FILE_MISSING":    {},
		"REQUIREMENT_MANUAL_INVALID":       {},
		"CHARTER_NAME_INVALID":             {},
		"CHARTER_GROUP_INVALID":            {},
		"CHARTER_SPEC_MISSING":             {},
		"CHARTER_GROUP_MISSING":            {},
		"CHARTER_DEPENDENCY_INVALID":       {},
		"CHARTER_CYCLE_PRESENT":            {},
		"CHARTER_NOTES_INVALID":            {},
		"CONFIG_TAG_INVALID":               {},
		"CONFIG_PREFIX_INVALID":            {},
		"CONFIG_FORMAT_INVALID":            {},
		"REDUNDANT_SEMANTIC_TAG":           {},
		"AMBIGUOUS_FILE_OWNERSHIP":         {},
		"UNOWNED_SOURCE_FILE":              {},
		"CHECKPOINT_UNAVAILABLE":           {},
	}

	stringCode, ok := code.(string)
	if !ok {
		t.Fatalf("validation code = %#v", code)
	}
	if _, exists := allowed[stringCode]; !exists {
		t.Fatalf("validation code %q is outside the SPEC.md catalog", stringCode)
	}
}

func requireNextAction[T ~[]any](t *testing.T, next T, index int, wantAction string) map[string]any {
	t.Helper()

	if len(next) <= index {
		t.Fatalf("next = %#v, want index %d", next, index)
	}
	item, ok := next[index].(map[string]any)
	if !ok {
		t.Fatalf("next[%d] = %#v", index, next[index])
	}
	if item["action"] != wantAction {
		t.Fatalf("next[%d].action = %#v, want %q", index, item["action"], wantAction)
	}
	return item
}

func requireTemplate(t *testing.T, next map[string]any) map[string]any {
	t.Helper()

	template, ok := next["template"].(map[string]any)
	if !ok {
		t.Fatalf("template = %#v", next["template"])
	}
	if _, hasArgv := next["argv"]; hasArgv {
		t.Fatalf("next mixes executable argv with template: %#v", next)
	}
	return template
}

func requiredFieldNames(t *testing.T, value any) []string {
	t.Helper()

	fields, ok := value.([]any)
	if !ok {
		t.Fatalf("required_fields = %#v", value)
	}
	names := make([]string, 0, len(fields))
	for _, field := range fields {
		item, ok := field.(map[string]any)
		if !ok {
			t.Fatalf("required field = %#v", field)
		}
		name, ok := item["name"].(string)
		if !ok {
			t.Fatalf("required field name = %#v", item["name"])
		}
		names = append(names, name)
	}
	return names
}

func requiredFieldDescriptions(t *testing.T, value any) []string {
	t.Helper()

	fields, ok := value.([]any)
	if !ok {
		t.Fatalf("required_fields = %#v", value)
	}
	descriptions := make([]string, 0, len(fields))
	for _, field := range fields {
		item, ok := field.(map[string]any)
		if !ok {
			t.Fatalf("required field = %#v", field)
		}
		description, ok := item["description"].(string)
		if !ok {
			t.Fatalf("required field description = %#v", item["description"])
		}
		descriptions = append(descriptions, description)
	}
	return descriptions
}

func stringSliceFromAny(t *testing.T, value any) []string {
	t.Helper()

	items, ok := value.([]any)
	if !ok {
		t.Fatalf("value = %#v, want []any", value)
	}
	result := make([]string, 0, len(items))
	for _, item := range items {
		text, ok := item.(string)
		if !ok {
			t.Fatalf("slice item = %#v, want string", item)
		}
		result = append(result, text)
	}
	return result
}

func mapsToAny[T ~[]map[string]any](items T) []any {
	result := make([]any, 0, len(items))
	for _, item := range items {
		result = append(result, item)
	}
	return result
}

func mustParseTestTime(t *testing.T, value string) time.Time {
	t.Helper()

	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		t.Fatalf("parse time %q: %v", value, err)
	}
	return parsed
}

func orderedSlugs(items []struct {
	Slug string `json:"slug"`
}) []string {
	slugs := make([]string, 0, len(items))
	for _, item := range items {
		slugs = append(slugs, item.Slug)
	}
	return slugs
}
