package application

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/aitoroses/specctl/internal/domain"
	"github.com/aitoroses/specctl/internal/infrastructure"
)

func TestSpecWriteFailureEnvelopes(t *testing.T) {
	cases := []struct {
		name    string
		fixture string
		arrange func(t *testing.T, repoRoot string)
		invoke  func(t *testing.T, service *Service) error
		assert  func(t *testing.T, err error)
	}{
		{
			name: "malformed stored ids keep canonical charter membership",
			arrange: func(t *testing.T, repoRoot string) {
				replaceFileText(t, filepath.Join(repoRoot, ".specs", "runtime", "session-lifecycle.yaml"), "id: D-001", "id: D-003")
			},
			invoke: func(t *testing.T, service *Service) error {
				_, _, _, err := service.AddDelta(DeltaAddRequest{
					Target:         "runtime:session-lifecycle",
					Intent:         domain.DeltaIntentAdd,
					Area:           "Heartbeat timeout",
					Current:        "Current gap",
					CurrentPresent: true,
					Targets:        "Target gap",
					TargetPresent:  true,
					Notes:          "Explicitly tracked",
					NotesPresent:   true,
				})
				return err
			},
			assert: func(t *testing.T, err error) {
				failure := requireFailure(t, err, "VALIDATION_FAILED")
				state, ok := failure.State.(SpecProjection)
				if !ok {
					t.Fatalf("state type = %T, want SpecProjection", failure.State)
				}
				if state.CharterMembership == nil || state.CharterMembership.Group != "recovery" {
					t.Fatalf("charter_membership = %#v", state.CharterMembership)
				}
				if state.Validation.Valid || len(state.Validation.Findings) == 0 {
					t.Fatalf("validation = %#v", state.Validation)
				}
				focus := state.Focus.(map[string]any)
				validation := focus["validation"].(map[string]any)
				if len(validation["findings"].([]any)) == 0 {
					t.Fatalf("focus.validation = %#v", validation)
				}
			},
		},
		{
			name: "unresolved target returns minimal missing spec state",
			invoke: func(t *testing.T, service *Service) error {
				_, _, _, err := service.AddDelta(DeltaAddRequest{
					Target:         "runtime:missing-spec",
					Intent:         domain.DeltaIntentAdd,
					Area:           "Heartbeat timeout",
					Current:        "Current gap",
					CurrentPresent: true,
					Targets:        "Target gap",
					TargetPresent:  true,
					Notes:          "Explicitly tracked",
					NotesPresent:   true,
				})
				return err
			},
			assert: func(t *testing.T, err error) {
				failure := requireFailure(t, err, "SPEC_NOT_FOUND")
				state, ok := failure.State.(MissingSpecContext)
				if !ok {
					t.Fatalf("state type = %T, want MissingSpecContext", failure.State)
				}
				if state.Target != "runtime:missing-spec" || state.TrackingFile != ".specs/runtime/missing-spec.yaml" || !state.CharterExists {
					t.Fatalf("state = %#v", state)
				}
			},
		},
		{
			name: "post-mutation validation failure returns validation focus",
			invoke: func(t *testing.T, service *Service) error {
				loaded, failure, err := service.loadSpecForMutation("runtime:session-lifecycle")
				if err != nil {
					return err
				}
				if failure != nil {
					return failure
				}

				updated := cloneTracking(loaded.tracking)
				updated.Scope = nil
				_, _, _, err = service.finalizeSpecMutation(loaded, updated, map[string]any{"kind": "delta"}, nil, nil)
				return err
			},
			assert: func(t *testing.T, err error) {
				failure := requireFailure(t, err, "VALIDATION_FAILED")
				state, ok := failure.State.(SpecProjection)
				if !ok {
					t.Fatalf("state type = %T, want SpecProjection", failure.State)
				}
				if state.CharterMembership == nil || state.CharterMembership.Group != "recovery" {
					t.Fatalf("charter_membership = %#v", state.CharterMembership)
				}
				if state.Validation.Valid || len(state.Validation.Findings) == 0 {
					t.Fatalf("validation = %#v", state.Validation)
				}
				focus := state.Focus.(map[string]any)
				validation := focus["validation"].(map[string]any)
				if len(validation["findings"].([]any)) == 0 {
					t.Fatalf("focus.validation = %#v", validation)
				}
			},
		},
		{
			name: "post-mutation invalid delta id stays in the validation catalog",
			invoke: func(t *testing.T, service *Service) error {
				loaded, failure, err := service.loadSpecForMutation("runtime:session-lifecycle")
				if err != nil {
					return err
				}
				if failure != nil {
					return failure
				}

				updated := cloneTracking(loaded.tracking)
				updated.Deltas[0].ID = "D-01"
				_, _, _, err = service.finalizeSpecMutation(loaded, updated, map[string]any{"kind": "delta"}, nil, nil)
				return err
			},
			assert: func(t *testing.T, err error) {
				failure := requireFailure(t, err, "VALIDATION_FAILED")
				state, ok := failure.State.(SpecProjection)
				if !ok {
					t.Fatalf("state type = %T, want SpecProjection", failure.State)
				}
				if len(state.Validation.Findings) != 1 {
					t.Fatalf("validation = %#v", state.Validation)
				}
				switch finding := state.Validation.Findings[0].(type) {
				case map[string]any:
					if finding["code"] != "DELTA_ID_INVALID" || finding["target"] != "deltas" {
						t.Fatalf("finding = %#v", finding)
					}
				case infrastructure.ValidationFinding:
					if finding.Code != "DELTA_ID_INVALID" || finding.Target != "deltas" {
						t.Fatalf("finding = %#v", finding)
					}
				default:
					t.Fatalf("finding type = %T", state.Validation.Findings[0])
				}
			},
		},
		{
			name: "delta add blank required fields reuse missing_fields focus",
			invoke: func(t *testing.T, service *Service) error {
				_, _, _, err := service.AddDelta(DeltaAddRequest{
					Target:         "runtime:session-lifecycle",
					Intent:         domain.DeltaIntentAdd,
					Area:           "Heartbeat timeout",
					Current:        "   ",
					CurrentPresent: true,
					Targets:        "Target gap",
					TargetPresent:  true,
					Notes:          "Explicitly tracked",
					NotesPresent:   true,
				})
				return err
			},
			assert: func(t *testing.T, err error) {
				failure := requireFailure(t, err, "INVALID_INPUT")
				state, ok := failure.State.(SpecProjection)
				if !ok {
					t.Fatalf("state type = %T, want SpecProjection", failure.State)
				}
				if state.CharterMembership == nil || state.CharterMembership.Group != "recovery" {
					t.Fatalf("charter_membership = %#v", state.CharterMembership)
				}
				focus, ok := state.Focus.(map[string]any)
				if !ok {
					t.Fatalf("focus type = %T", state.Focus)
				}
				input, ok := focus["input"].(map[string]any)
				if !ok {
					t.Fatalf("focus.input = %#v", focus)
				}
				if !reflect.DeepEqual(input["missing_fields"], []string{"current"}) {
					t.Fatalf("focus.input = %#v", input)
				}
			},
		},
		{
			name:    "req verify malformed test file path reuses missing-file failure code",
			fixture: "active-spec",
			invoke: func(t *testing.T, service *Service) error {
				_, _, _, err := service.VerifyRequirement(RequirementVerifyRequest{
					Target:        "runtime:session-lifecycle",
					RequirementID: "REQ-001",
					TestFiles:     []string{"../outside.py"},
				})
				return err
			},
			assert: func(t *testing.T, err error) {
				failure := requireFailure(t, err, "TEST_FILE_NOT_FOUND")
				state, ok := failure.State.(SpecProjection)
				if !ok {
					t.Fatalf("state type = %T, want SpecProjection", failure.State)
				}
				if state.Slug != "session-lifecycle" || state.Charter != "runtime" {
					t.Fatalf("state = %#v", state)
				}
				focus, ok := state.Focus.(map[string]any)
				if !ok {
					t.Fatalf("focus type = %T", state.Focus)
				}
				if got := focus["invalid_paths"].([]string); !reflect.DeepEqual(got, []string{"../outside.py"}) {
					t.Fatalf("invalid_paths = %#v", focus["invalid_paths"])
				}
			},
		},
		{
			name: "req add malformed gherkin keeps canonical spec state",
			invoke: func(t *testing.T, service *Service) error {
				_, _, _, err := service.AddRequirement(RequirementAddRequest{
					Target:  "runtime:session-lifecycle",
					DeltaID: "D-001",
					Gherkin: "Scenario: Missing feature\n  Given a broken requirement\n  When validation runs\n  Then it fails\n",
				})
				return err
			},
			assert: func(t *testing.T, err error) {
				failure := requireFailure(t, err, "INVALID_GHERKIN")
				state, ok := failure.State.(SpecProjection)
				if !ok {
					t.Fatalf("state type = %T, want SpecProjection", failure.State)
				}
				if state.Slug != "session-lifecycle" || state.Charter != "runtime" {
					t.Fatalf("state = %#v", state)
				}
				focus, ok := state.Focus.(map[string]any)
				if !ok {
					t.Fatalf("focus type = %T", state.Focus)
				}
				if value, exists := focus["requirement"]; exists && value != nil {
					if requirement, ok := value.(map[string]any); !ok || len(requirement) != 0 {
						t.Fatalf("focus.requirement = %#v", value)
					}
				}
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fixture := tc.fixture
			if fixture == "" {
				fixture = "ready-spec"
			}
			repoRoot := copyApplicationFixtureRepo(t, fixture)
			if tc.arrange != nil {
				tc.arrange(t, repoRoot)
			}

			service := &Service{
				repoRoot: repoRoot,
				specsDir: filepath.Join(repoRoot, ".specs"),
			}

			tc.assert(t, tc.invoke(t, service))
		})
	}
}

func TestConfigRemovePrefixMalformedPathUsesPrefixNotFoundEnvelope(t *testing.T) {
	repoRoot := copyApplicationFixtureRepo(t, "ready-spec")
	service := &Service{
		repoRoot: repoRoot,
		specsDir: filepath.Join(repoRoot, ".specs"),
	}

	_, _, _, err := service.RemoveConfigPrefix("../outside")
	failure := requireFailure(t, err, "PREFIX_NOT_FOUND")
	state, ok := failure.State.(ConfigProjection)
	if !ok {
		t.Fatalf("state type = %T, want ConfigProjection", failure.State)
	}
	if !reflect.DeepEqual(state.SourcePrefixes, []string{"runtime/src/"}) {
		t.Fatalf("source_prefixes = %#v", state.SourcePrefixes)
	}
	if !state.Validation.Valid {
		t.Fatalf("validation = %#v", state.Validation)
	}
	focus, ok := state.Focus.(map[string]any)
	if !ok {
		t.Fatalf("focus type = %T", state.Focus)
	}
	configMutation := focus["config_mutation"].(map[string]any)
	if configMutation["kind"] != "remove_prefix" || configMutation["value"] != "../outside" {
		t.Fatalf("config_mutation = %#v", configMutation)
	}
	if got := focus["invalid_paths"].([]string); !reflect.DeepEqual(got, []string{"../outside"}) {
		t.Fatalf("invalid_paths = %#v", focus["invalid_paths"])
	}
}

func TestConfigMutationsFailWhenPostWriteAuditContainsErrors(t *testing.T) {
	cases := []struct {
		name   string
		invoke func(*Service) error
		value  string
	}{
		{
			name: "add tag",
			invoke: func(service *Service) error {
				_, _, _, err := service.AddConfigTag("adapter")
				return err
			},
			value: "adapter",
		},
		{
			name: "add prefix",
			invoke: func(service *Service) error {
				_, _, _, err := service.AddConfigPrefix("runtime/src/application")
				return err
			},
			value: "runtime/src/application/",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repoRoot := copyApplicationFixtureRepo(t, "ready-spec")
			if err := os.Remove(filepath.Join(repoRoot, ".specs", "runtime", "CHARTER.yaml")); err != nil {
				t.Fatalf("remove charter: %v", err)
			}

			service := &Service{
				repoRoot: repoRoot,
				specsDir: filepath.Join(repoRoot, ".specs"),
			}

			failure := requireFailure(t, tc.invoke(service), "VALIDATION_FAILED")
			state, ok := failure.State.(ConfigProjection)
			if !ok {
				t.Fatalf("state type = %T, want ConfigProjection", failure.State)
			}
			if state.Validation.Valid {
				t.Fatalf("state.validation = %#v", state.Validation)
			}
			switch tc.name {
			case "add tag":
				if !reflect.DeepEqual(state.GherkinTags, []string{"runtime", "domain"}) {
					t.Fatalf("gherkin_tags = %#v", state.GherkinTags)
				}
			case "add prefix":
				if !reflect.DeepEqual(state.SourcePrefixes, []string{"runtime/src/"}) {
					t.Fatalf("source_prefixes = %#v", state.SourcePrefixes)
				}
			}
			focus, ok := state.Focus.(map[string]any)
			if !ok {
				t.Fatalf("focus type = %T", state.Focus)
			}
			if mutation, ok := focus["config_mutation"].(map[string]any); !ok || mutation["value"] != tc.value {
				t.Fatalf("focus.config_mutation = %#v", focus["config_mutation"])
			}
			requireFindingCode(t, focus["validation"].(map[string]any)["findings"].([]any), "CHARTER_SPEC_MISSING")
			configBytes, readErr := os.ReadFile(filepath.Join(repoRoot, ".specs", "specctl.yaml"))
			if readErr != nil {
				t.Fatalf("read config: %v", readErr)
			}
			if strings.Contains(string(configBytes), "adapter") || strings.Contains(string(configBytes), "runtime/src/application/") {
				t.Fatalf("config should remain unchanged:\n%s", configBytes)
			}
		})
	}
}

func TestNonRuntimeNextActionsDeriveFromSpecContext(t *testing.T) {
	tracking := &domain.TrackingFile{
		Charter: "ui",
		Scope:   []string{"ui/src/domain/threads/"},
	}

	t.Run("delta add seeds the charter tag", func(t *testing.T) {
		next := buildDeltaAddNext("ui:thread-lifecycle", tracking, domain.Delta{ID: "D-001", Intent: domain.DeltaIntentAdd}, nil)
		// Step 0 is write_spec_section (edit_file), step 1 is add_requirement (run_command)
		template := next[1].(map[string]any)["template"].(map[string]any)
		if template["stdin_template"] != "@ui\nFeature: <feature>\n" {
			t.Fatalf("stdin_template = %#v", template["stdin_template"])
		}
	})

	t.Run("req add suggests a colocated unit test path", func(t *testing.T) {
		next := buildVerifyRequirementNext("ui:thread-lifecycle", tracking, domain.Requirement{
			ID:    "REQ-001",
			Title: "Thread lifecycle",
			Tags:  []string{"ui"},
		})
		argv := next[0].(map[string]any)["template"].(map[string]any)["argv"].([]string)
		if got := strings.Join(argv, " "); got != "specctl req verify ui:thread-lifecycle REQ-001 --test-file ui/tests/domain/test_thread_lifecycle.py" {
			t.Fatalf("next.argv = %v", argv)
		}
	})

	t.Run("non-domain scope suggests the unit test directory", func(t *testing.T) {
		next := buildVerifyRequirementNext("ui:thread-lifecycle", &domain.TrackingFile{
			Charter: "ui",
			Scope:   []string{"ui/src/application/threads/"},
		}, domain.Requirement{
			ID:    "REQ-001",
			Title: "Thread lifecycle",
			Tags:  []string{"ui"},
		})
		argv := next[0].(map[string]any)["template"].(map[string]any)["argv"].([]string)
		if got := strings.Join(argv, " "); got != "specctl req verify ui:thread-lifecycle REQ-001 --test-file ui/tests/unit/test_thread_lifecycle.py" {
			t.Fatalf("next.argv = %v", argv)
		}
	})

	t.Run("delta close suggests a charter e2e journey path", func(t *testing.T) {
		next := buildDeltaCloseBlockingNext("ui:thread-lifecycle", tracking, domain.Requirement{
			ID:    "REQ-001",
			Title: "Thread lifecycle",
			Tags:  []string{"ui", "e2e"},
		})
		argv := next[0].(map[string]any)["template"].(map[string]any)["argv"].([]string)
		if got := strings.Join(argv, " "); got != "specctl req verify ui:thread-lifecycle REQ-001 --test-file ui/tests/e2e/journeys/test_thread_lifecycle.py" {
			t.Fatalf("next.argv = %v", argv)
		}
	})

	t.Run("existing test files seed the next-action filename pattern", func(t *testing.T) {
		withPattern := &domain.TrackingFile{
			Charter: "custom",
			Scope:   []string{"custom/src/domain/flows/"},
			Requirements: []domain.Requirement{{
				ID:        "REQ-001",
				Title:     "Existing flow",
				Tags:      []string{"custom"},
				TestFiles: []string{"custom/tests/domain/test_existing_flow.py"},
				Verified:  true,
			}},
		}

		next := buildVerifyRequirementNext("custom:new-flow", withPattern, domain.Requirement{
			ID:    "REQ-002",
			Title: "New flow",
			Tags:  []string{"custom"},
		})
		argv := next[0].(map[string]any)["template"].(map[string]any)["argv"].([]string)
		if got := strings.Join(argv, " "); got != "specctl req verify custom:new-flow REQ-002 --test-file custom/tests/domain/test_new_flow.py" {
			t.Fatalf("next.argv = %v", argv)
		}
	})
}

func TestCharterWriteFailureEnvelopeKeepsCanonicalCharterState(t *testing.T) {
	repoRoot := copyApplicationFixtureRepo(t, "ready-spec")
	replaceFileText(t, filepath.Join(repoRoot, ".specs", "runtime", "session-lifecycle.yaml"), "status: ready", "status: invalid")

	service := &Service{
		repoRoot: repoRoot,
		specsDir: filepath.Join(repoRoot, ".specs"),
	}

	_, _, _, err := service.AddSpecToCharter(CharterAddSpecRequest{
		Charter:   "runtime",
		Slug:      "session-lifecycle",
		Group:     "recovery",
		Order:     20,
		DependsOn: []string{},
		Notes:     "Session FSM and cleanup behavior",
	})

	failure := requireFailure(t, err, "VALIDATION_FAILED")
	state, ok := failure.State.(CharterProjection)
	if !ok {
		t.Fatalf("state type = %T, want CharterProjection", failure.State)
	}
	if state.Name != "runtime" || state.TrackingFile != ".specs/runtime/CHARTER.yaml" {
		t.Fatalf("state = %#v", state)
	}
	if state.Validation.Valid != true {
		t.Fatalf("charter validation = %#v", state.Validation)
	}
	if len(state.OrderedSpecs) != 1 || state.OrderedSpecs[0].Slug != "session-lifecycle" || state.OrderedSpecs[0].Validation.Valid {
		t.Fatalf("ordered_specs = %#v", state.OrderedSpecs)
	}
	focus, ok := state.Focus.(map[string]any)
	if !ok {
		t.Fatalf("focus type = %T", state.Focus)
	}
	validation, ok := focus["validation"].(map[string]any)
	if !ok {
		t.Fatalf("focus.validation = %#v", state.Focus)
	}
	if len(validation["findings"].([]any)) == 0 {
		t.Fatalf("focus.validation = %#v", validation)
	}
	if len(failure.Next) != 2 {
		t.Fatalf("next = %#v", failure.Next)
	}
	next := failure.Next[0].(map[string]any)
	if next["kind"] != "edit_file" || next["path"] != ".specs/runtime/session-lifecycle.yaml" {
		t.Fatalf("next = %#v", next)
	}
}

func TestCharterAddSpecCycleFailureIncludesFocus(t *testing.T) {
	repoRoot := copyApplicationFixtureRepo(t, "charter-dag")

	service := &Service{
		repoRoot: repoRoot,
		specsDir: filepath.Join(repoRoot, ".specs"),
	}

	_, _, _, err := service.AddSpecToCharter(CharterAddSpecRequest{
		Charter:   "runtime",
		Slug:      "redis-state",
		Group:     "execution",
		Order:     10,
		DependsOn: []string{"session-lifecycle"},
		Notes:     "Storage and CAS guarantees",
	})

	failure := requireFailure(t, err, "CHARTER_CYCLE")
	state, ok := failure.State.(CharterProjection)
	if !ok {
		t.Fatalf("state type = %T, want CharterProjection", failure.State)
	}
	if state.Name != "runtime" || state.TrackingFile != ".specs/runtime/CHARTER.yaml" {
		t.Fatalf("state = %#v", state)
	}
	if len(state.OrderedSpecs) != 3 || state.OrderedSpecs[0].Slug != "redis-state" {
		t.Fatalf("ordered_specs = %#v", state.OrderedSpecs)
	}
	focus, ok := state.Focus.(map[string]any)
	if !ok {
		t.Fatalf("focus type = %T", state.Focus)
	}
	entry, ok := focus["entry"].(domain.CharterSpecEntry)
	if !ok {
		t.Fatalf("focus.entry = %#v", focus["entry"])
	}
	if entry.Slug != "redis-state" || !reflect.DeepEqual(entry.DependsOn, []string{"session-lifecycle"}) {
		t.Fatalf("focus.entry = %#v", entry)
	}
	cycle, ok := focus["cycle"].([]string)
	if !ok {
		t.Fatalf("focus.cycle = %#v", focus["cycle"])
	}
	if !reflect.DeepEqual(cycle, []string{"redis-state", "session-lifecycle"}) {
		t.Fatalf("focus.cycle = %#v", cycle)
	}
}

func TestCharterAddSpecSuccessKeepsCanonicalCharterState(t *testing.T) {
	repoRoot := copyApplicationFixtureRepo(t, "ready-spec")
	removeCharterEntry(t, filepath.Join(repoRoot, ".specs", "runtime", "CHARTER.yaml"), "session-lifecycle")

	service := &Service{
		repoRoot: repoRoot,
		specsDir: filepath.Join(repoRoot, ".specs"),
	}

	state, result, _, err := service.AddSpecToCharter(CharterAddSpecRequest{
		Charter:   "runtime",
		Slug:      "session-lifecycle",
		Group:     "recovery",
		Order:     30,
		DependsOn: []string{},
		Notes:     "Session FSM and cleanup behavior",
	})
	if err != nil {
		t.Fatalf("AddSpecToCharter: %v", err)
	}
	if result["kind"] != "charter_entry" {
		t.Fatalf("result = %#v", result)
	}
	if state.Name != "runtime" || state.TrackingFile != ".specs/runtime/CHARTER.yaml" {
		t.Fatalf("state = %#v", state)
	}
	if !state.Validation.Valid {
		t.Fatalf("state.validation = %#v", state.Validation)
	}
	if len(state.OrderedSpecs) != 1 || state.OrderedSpecs[0].Slug != "session-lifecycle" || !state.OrderedSpecs[0].Validation.Valid {
		t.Fatalf("ordered_specs = %#v", state.OrderedSpecs)
	}
}

func TestCharterAddSpecFailsWhenPostWriteAuditContainsErrors(t *testing.T) {
	repoRoot := copyApplicationFixtureRepo(t, "charter-dag")
	removeCharterEntry(t, filepath.Join(repoRoot, ".specs", "runtime", "CHARTER.yaml"), "session-lifecycle")
	replaceFileText(t, filepath.Join(repoRoot, ".specs", "runtime", "recovery-projection.yaml"), "status: draft", "status: invalid")
	writeApplicationTestFile(t, filepath.Join(repoRoot, ".specs", "runtime", "stray-tracking.yaml"), []byte(`slug: stray-tracking
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

	service := &Service{
		repoRoot: repoRoot,
		specsDir: filepath.Join(repoRoot, ".specs"),
	}

	_, _, _, err := service.AddSpecToCharter(CharterAddSpecRequest{
		Charter:   "runtime",
		Slug:      "session-lifecycle",
		Group:     "recovery",
		Order:     30,
		DependsOn: []string{"redis-state", "recovery-projection"},
		Notes:     "Session FSM and cleanup behavior",
	})
	failure := requireFailure(t, err, "VALIDATION_FAILED")
	state, ok := failure.State.(CharterProjection)
	if !ok {
		t.Fatalf("state type = %T, want CharterProjection", failure.State)
	}
	if state.Validation.Valid {
		t.Fatalf("state.validation = %#v", state.Validation)
	}
	requireFindingCode(t, state.Focus.(map[string]any)["validation"].(map[string]any)["findings"].([]any), "SPEC_STATUS_INVALID")
}

func TestCreateCharterValidationFailureUsesStructuredEnvelope(t *testing.T) {
	repoRoot := t.TempDir()
	if err := os.MkdirAll(filepath.Join(repoRoot, ".specs"), 0o755); err != nil {
		t.Fatalf("mkdir specs: %v", err)
	}

	service := &Service{
		repoRoot: repoRoot,
		specsDir: filepath.Join(repoRoot, ".specs"),
	}

	_, _, _, err := service.CreateCharter(CharterCreateRequest{
		Charter:     "runtime",
		Title:       "Runtime System",
		Description: "Specs for runtime control-plane and data-plane behavior",
		Groups: []domain.CharterGroup{
			{Key: "execution", Title: "Execution Engine", Order: -1},
		},
	})

	failure := requireFailure(t, err, "VALIDATION_FAILED")
	state, ok := failure.State.(CharterProjection)
	if !ok {
		t.Fatalf("state type = %T, want CharterProjection", failure.State)
	}
	if state.Name != "runtime" || state.TrackingFile != ".specs/runtime/CHARTER.yaml" {
		t.Fatalf("state = %#v", state)
	}
	if len(state.Groups) != 1 || state.Groups[0].Key != "execution" || state.Groups[0].Order != -1 {
		t.Fatalf("groups = %#v", state.Groups)
	}
	if state.Validation.Valid || len(state.Validation.Findings) == 0 {
		t.Fatalf("validation = %#v", state.Validation)
	}
	focus := state.Focus.(map[string]any)
	if len(focus["validation"].(map[string]any)["findings"].([]any)) == 0 {
		t.Fatalf("focus = %#v", focus)
	}
}

func TestCreateCharterFailsWhenPostWriteAuditContainsErrors(t *testing.T) {
	repoRoot := t.TempDir()
	if err := os.MkdirAll(filepath.Join(repoRoot, ".specs", "runtime"), 0o755); err != nil {
		t.Fatalf("mkdir specs: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(repoRoot, "runtime", "src", "domain", "session_execution"), 0o755); err != nil {
		t.Fatalf("mkdir runtime scope: %v", err)
	}
	writeApplicationTestFile(t, filepath.Join(repoRoot, "runtime", "src", "domain", "session_execution", "SPEC.md"), []byte("---\nspec: session-lifecycle\ncharter: runtime\n---\n# Session Lifecycle\n"))
	writeApplicationTestFile(t, filepath.Join(repoRoot, ".specs", "runtime", "session-lifecycle.yaml"), []byte(`slug: session-lifecycle
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
`))
	writeApplicationTestFile(t, filepath.Join(repoRoot, ".specs", "runtime", "stray-tracking.yaml"), []byte(`slug: stray-tracking
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

	service := &Service{
		repoRoot: repoRoot,
		specsDir: filepath.Join(repoRoot, ".specs"),
	}

	_, _, _, err := service.CreateCharter(CharterCreateRequest{
		Charter:     "runtime",
		Title:       "Runtime System",
		Description: "Specs for runtime control-plane and data-plane behavior",
	})

	failure := requireFailure(t, err, "VALIDATION_FAILED")
	state, ok := failure.State.(CharterProjection)
	if !ok {
		t.Fatalf("state type = %T, want CharterProjection", failure.State)
	}
	if state.Name != "runtime" {
		t.Fatalf("state.Name = %q, want runtime", state.Name)
	}
	if state.Focus == nil {
		t.Fatal("state.Focus is nil, want validation focus")
	}
	focus, ok := state.Focus.(map[string]any)
	if !ok {
		t.Fatalf("focus type = %T", state.Focus)
	}
	requireFindingCode(t, focus["validation"].(map[string]any)["findings"].([]any), "PRIMARY_DOC_FRONTMATTER_MISMATCH")
}

func TestCreateSpecValidationFailureUsesStructuredEnvelope(t *testing.T) {
	repoRoot := copyApplicationFixtureRepo(t, "charter-dag")
	initGitRepoAtDate(t, repoRoot, "2026-03-28T12:00:00Z")
	if err := os.MkdirAll(filepath.Join(repoRoot, "runtime", "src", "domain", "new_protocol"), 0o755); err != nil {
		t.Fatalf("mkdir design doc dir: %v", err)
	}

	service := &Service{
		repoRoot: repoRoot,
		specsDir: filepath.Join(repoRoot, ".specs"),
	}

	_, _, _, err := service.CreateSpec(SpecCreateRequest{
		Target:       "runtime:new-protocol",
		Title:        "New Protocol",
		Doc:          "runtime/src/domain/new_protocol/SPEC.md",
		Scope:        []string{"runtime/src/domain/new_protocol/"},
		Group:        "recovery",
		Order:        30,
		CharterNotes: "Protocol planning",
		Tags:         []string{"Invalid-Tag"},
	})

	failure := requireFailure(t, err, "VALIDATION_FAILED")
	state, ok := failure.State.(SpecProjection)
	if !ok {
		t.Fatalf("state type = %T, want SpecProjection", failure.State)
	}
	if state.Slug != "new-protocol" || state.Charter != "runtime" || state.TrackingFile != ".specs/runtime/new-protocol.yaml" {
		t.Fatalf("state = %#v", state)
	}
	if state.CharterMembership == nil || state.CharterMembership.Group != "recovery" || state.CharterMembership.Order != 30 {
		t.Fatalf("charter_membership = %#v", state.CharterMembership)
	}
	if state.Validation.Valid || len(state.Validation.Findings) == 0 {
		t.Fatalf("validation = %#v", state.Validation)
	}
	foundTagInvalid := false
	for _, finding := range state.Validation.Findings {
		if typed, ok := finding.(infrastructure.ValidationFinding); ok && typed.Code == "SPEC_TAG_INVALID" {
			foundTagInvalid = true
		}
	}
	if !foundTagInvalid {
		t.Fatalf("validation = %#v", state.Validation)
	}
	focus := state.Focus.(map[string]any)
	if len(focus["validation"].(map[string]any)["findings"].([]any)) == 0 {
		t.Fatalf("focus = %#v", focus)
	}
}

func TestCreateSpecCharterMembershipValidationUsesStructuredEnvelope(t *testing.T) {
	repoRoot := copyApplicationFixtureRepo(t, "charter-dag")
	initGitRepoAtDate(t, repoRoot, "2026-03-28T12:00:00Z")
	if err := os.MkdirAll(filepath.Join(repoRoot, "runtime", "src", "domain", "new_protocol"), 0o755); err != nil {
		t.Fatalf("mkdir design doc dir: %v", err)
	}

	service := &Service{
		repoRoot: repoRoot,
		specsDir: filepath.Join(repoRoot, ".specs"),
	}

	_, _, _, err := service.CreateSpec(SpecCreateRequest{
		Target:       "runtime:new-protocol",
		Title:        "New Protocol",
		Doc:          "runtime/src/domain/new_protocol/SPEC.md",
		Scope:        []string{"runtime/src/domain/new_protocol/"},
		Group:        "recovery",
		Order:        30,
		CharterNotes: "Protocol planning",
		DependsOn:    []string{"missing-spec"},
	})

	failure := requireFailure(t, err, "VALIDATION_FAILED")
	state, ok := failure.State.(SpecProjection)
	if !ok {
		t.Fatalf("state type = %T, want SpecProjection", failure.State)
	}
	if state.Slug != "new-protocol" || state.Charter != "runtime" || state.TrackingFile != ".specs/runtime/new-protocol.yaml" {
		t.Fatalf("state = %#v", state)
	}
	if state.CharterMembership == nil || state.CharterMembership.Group != "recovery" || state.CharterMembership.Order != 30 || !reflect.DeepEqual(state.CharterMembership.DependsOn, []string{"missing-spec"}) {
		t.Fatalf("charter_membership = %#v", state.CharterMembership)
	}
	if state.Validation.Valid || len(state.Validation.Findings) == 0 {
		t.Fatalf("validation = %#v", state.Validation)
	}
	focus := state.Focus.(map[string]any)
	if len(focus["validation"].(map[string]any)["findings"].([]any)) == 0 {
		t.Fatalf("focus = %#v", focus)
	}
	if _, statErr := os.Stat(filepath.Join(repoRoot, ".specs", "runtime", "new-protocol.yaml")); !os.IsNotExist(statErr) {
		t.Fatalf("expected no persisted tracking file, got err=%v", statErr)
	}
}

func TestCreateSpecFailsWhenPostWriteAuditContainsErrors(t *testing.T) {
	repoRoot := copyApplicationFixtureRepo(t, "ready-spec")
	initGitRepoAtDate(t, repoRoot, "2026-03-28T12:00:00Z")
	writeApplicationTestFile(t, filepath.Join(repoRoot, ".specs", "runtime", "stray-tracking.yaml"), []byte(`slug: stray-tracking
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

	service := &Service{
		repoRoot: repoRoot,
		specsDir: filepath.Join(repoRoot, ".specs"),
	}

	_, _, _, err := service.CreateSpec(SpecCreateRequest{
		Target:       "runtime:new-protocol",
		Title:        "New Protocol",
		Doc:          "runtime/src/domain/new_protocol/SPEC.md",
		Scope:        []string{"runtime/src/domain/new_protocol/"},
		Group:        "recovery",
		Order:        30,
		CharterNotes: "Protocol planning",
	})

	failure := requireFailure(t, err, "VALIDATION_FAILED")
	state, ok := failure.State.(SpecProjection)
	if !ok {
		t.Fatalf("state type = %T, want SpecProjection", failure.State)
	}
	if state.Slug != "new-protocol" || state.Charter != "runtime" || state.TrackingFile != ".specs/runtime/new-protocol.yaml" {
		t.Fatalf("state = %#v", state)
	}
	if state.CharterMembership == nil || state.CharterMembership.Group != "recovery" || state.CharterMembership.Order != 30 {
		t.Fatalf("charter_membership = %#v", state.CharterMembership)
	}
	if state.Validation.Valid {
		t.Fatalf("validation = %#v", state.Validation)
	}
	requireFindingCode(t, state.Focus.(map[string]any)["validation"].(map[string]any)["findings"].([]any), "PRIMARY_DOC_FRONTMATTER_MISMATCH")
	if _, statErr := os.Stat(filepath.Join(repoRoot, ".specs", "runtime", "new-protocol.yaml")); !os.IsNotExist(statErr) {
		t.Fatalf("expected no persisted tracking file, got err=%v", statErr)
	}
	if _, statErr := os.Stat(filepath.Join(repoRoot, "runtime", "src", "domain", "new_protocol", "SPEC.md")); !os.IsNotExist(statErr) {
		t.Fatalf("expected no persisted design doc, got err=%v", statErr)
	}
	charterBytes, readErr := os.ReadFile(filepath.Join(repoRoot, ".specs", "runtime", "CHARTER.yaml"))
	if readErr != nil {
		t.Fatalf("read charter: %v", readErr)
	}
	if strings.Contains(string(charterBytes), "slug: new-protocol") {
		t.Fatalf("charter should not persist new entry:\n%s", charterBytes)
	}
}

func TestCharterAddSpecSemanticValidationFailuresUseCanonicalCharterState(t *testing.T) {
	cases := []struct {
		name              string
		request           CharterAddSpecRequest
		requiredCode      string
		forbiddenFindings []string
	}{
		{
			name: "invalid new group key",
			request: CharterAddSpecRequest{
				Charter:    "runtime",
				Slug:       "session-lifecycle",
				Group:      "bad_group",
				GroupTitle: pointerTo("Broken Group"),
				GroupOrder: pointerTo(30),
				Order:      30,
				DependsOn:  []string{},
				Notes:      "Session FSM and cleanup behavior",
			},
			requiredCode: "CHARTER_GROUP_INVALID",
		},
		{
			name: "negative order",
			request: CharterAddSpecRequest{
				Charter:   "runtime",
				Slug:      "session-lifecycle",
				Group:     "recovery",
				Order:     -1,
				DependsOn: []string{},
				Notes:     "Session FSM and cleanup behavior",
			},
		},
		{
			name: "unknown dependency",
			request: CharterAddSpecRequest{
				Charter:   "runtime",
				Slug:      "session-lifecycle",
				Group:     "recovery",
				Order:     30,
				DependsOn: []string{"missing-spec"},
				Notes:     "Session FSM and cleanup behavior",
			},
			requiredCode:      "CHARTER_DEPENDENCY_INVALID",
			forbiddenFindings: []string{"CHARTER_CYCLE_PRESENT"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repoRoot := copyApplicationFixtureRepo(t, "ready-spec")
			service := &Service{
				repoRoot: repoRoot,
				specsDir: filepath.Join(repoRoot, ".specs"),
			}

			_, _, _, err := service.AddSpecToCharter(tc.request)
			failure := requireFailure(t, err, "VALIDATION_FAILED")
			state, ok := failure.State.(CharterProjection)
			if !ok {
				t.Fatalf("state type = %T, want CharterProjection", failure.State)
			}
			if state.Name != "runtime" || state.TrackingFile != ".specs/runtime/CHARTER.yaml" {
				t.Fatalf("state = %#v", state)
			}
			if state.Validation.Valid || len(state.Validation.Findings) == 0 {
				t.Fatalf("state.validation = %#v, want blocking charter validation findings", state.Validation)
			}
			if len(state.OrderedSpecs) != 1 || state.OrderedSpecs[0].Slug != "session-lifecycle" {
				t.Fatalf("ordered_specs = %#v", state.OrderedSpecs)
			}
			focus := state.Focus.(map[string]any)
			findings := focus["validation"].(map[string]any)["findings"].([]any)
			if len(findings) == 0 {
				t.Fatalf("focus.validation = %#v", focus)
			}
			if tc.requiredCode != "" {
				requireFindingCode(t, findings, tc.requiredCode)
			}
			for _, forbidden := range tc.forbiddenFindings {
				for _, findingAny := range findings {
					switch finding := findingAny.(type) {
					case infrastructure.ValidationFinding:
						if finding.Code == forbidden {
							t.Fatalf("findings = %#v", findings)
						}
					case map[string]any:
						if finding["code"] == forbidden {
							t.Fatalf("findings = %#v", findings)
						}
					default:
						t.Fatalf("finding type = %T", findingAny)
					}
				}
			}
			if len(failure.Next) != 0 {
				t.Fatalf("next = %#v", failure.Next)
			}
		})
	}
}

func TestCharterAddSpecIgnoresPartialGroupMetadataForExistingGroup(t *testing.T) {
	repoRoot := copyApplicationFixtureRepo(t, "ready-spec")
	service := &Service{
		repoRoot: repoRoot,
		specsDir: filepath.Join(repoRoot, ".specs"),
	}

	groupTitle := "Should Be Ignored"
	state, result, _, err := service.AddSpecToCharter(CharterAddSpecRequest{
		Charter:    "runtime",
		Slug:       "session-lifecycle",
		Group:      "recovery",
		GroupTitle: &groupTitle,
		Order:      30,
		DependsOn:  []string{},
		Notes:      "Session FSM and cleanup behavior",
	})
	if err != nil {
		t.Fatalf("AddSpecToCharter: %v", err)
	}
	if createdGroup, exists := result["created_group"]; exists && createdGroup != nil {
		if group, ok := createdGroup.(*domain.CharterGroup); !ok || group != nil {
			t.Fatalf("created_group = %#v", createdGroup)
		}
	}
	entry := result["entry"].(domain.CharterSpecEntry)
	if entry.Group != "recovery" || entry.Order != 30 {
		t.Fatalf("entry = %#v", entry)
	}
	if state.OrderedSpecs[0].Group.Title != "Recovery and Cleanup" {
		t.Fatalf("state = %#v", state.OrderedSpecs[0].Group)
	}
}

func TestCharterAddSpecRequiresOnlyMissingMetadataAfterLoadingCharterState(t *testing.T) {
	repoRoot := copyApplicationFixtureRepo(t, "ready-spec")
	service := &Service{
		repoRoot: repoRoot,
		specsDir: filepath.Join(repoRoot, ".specs"),
	}

	groupTitle := "Missing Group"
	_, _, _, err := service.AddSpecToCharter(CharterAddSpecRequest{
		Charter:    "runtime",
		Slug:       "session-lifecycle",
		Group:      "missing-group",
		GroupTitle: &groupTitle,
		Order:      30,
		DependsOn:  []string{},
		Notes:      "Session FSM and cleanup behavior",
	})
	failure := requireFailure(t, err, "GROUP_REQUIRED")
	state, ok := failure.State.(CharterProjection)
	if !ok {
		t.Fatalf("state type = %T", failure.State)
	}
	focus := state.Focus.(map[string]any)["input"].(map[string]any)
	missing := focus["missing_fields"].([]string)
	if len(missing) != 1 || missing[0] != "group_order" {
		t.Fatalf("missing_fields = %#v", focus["missing_fields"])
	}
}

func TestCharterRemoveSpecDependencyFailureKeepsCanonicalCharterState(t *testing.T) {
	repoRoot := copyApplicationFixtureRepo(t, "charter-dag")
	replaceFileText(t, filepath.Join(repoRoot, ".specs", "runtime", "recovery-projection.yaml"), "status: draft", "status: invalid")
	writeApplicationTestFile(t, filepath.Join(repoRoot, ".specs", "runtime", "stray-tracking.yaml"), []byte(`slug: stray-tracking
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

	service := &Service{
		repoRoot: repoRoot,
		specsDir: filepath.Join(repoRoot, ".specs"),
	}

	_, _, _, err := service.RemoveSpecFromCharter("runtime", "redis-state")
	failure := requireFailure(t, err, "CHARTER_DEPENDENCY_EXISTS")
	state, ok := failure.State.(CharterProjection)
	if !ok {
		t.Fatalf("state type = %T, want CharterProjection", failure.State)
	}
	assertCanonicalCharterValidationState(t, state, "redis-state", "recovery-projection")
}

func TestCharterRemoveSpecSuccessKeepsCanonicalCharterState(t *testing.T) {
	repoRoot := copyApplicationFixtureRepo(t, "ready-spec")

	service := &Service{
		repoRoot: repoRoot,
		specsDir: filepath.Join(repoRoot, ".specs"),
	}

	state, result, _, err := service.RemoveSpecFromCharter("runtime", "session-lifecycle")
	if err != nil {
		t.Fatalf("RemoveSpecFromCharter: %v", err)
	}
	if result["removed_slug"] != "session-lifecycle" {
		t.Fatalf("result = %#v", result)
	}
	if state.Name != "runtime" || state.TrackingFile != ".specs/runtime/CHARTER.yaml" {
		t.Fatalf("state = %#v", state)
	}
	if !state.Validation.Valid || len(state.OrderedSpecs) != 0 {
		t.Fatalf("state = %#v", state)
	}
	if _, statErr := os.Stat(filepath.Join(repoRoot, ".specs", "runtime", "session-lifecycle.yaml")); statErr != nil {
		t.Fatalf("tracking file should remain after remove-spec: %v", statErr)
	}
}

func TestCharterRemoveSpecFailsWhenPostWriteAuditContainsErrors(t *testing.T) {
	repoRoot := copyApplicationFixtureRepo(t, "ready-spec")
	replaceFileText(
		t,
		filepath.Join(repoRoot, ".specs", "runtime", "CHARTER.yaml"),
		"specs:\n  - slug: session-lifecycle\n    group: recovery\n    order: 20\n    depends_on: []\n    notes: Session FSM and cleanup behavior\n",
		"specs:\n  - slug: session-lifecycle\n    group: recovery\n    order: 20\n    depends_on: []\n    notes: Session FSM and cleanup behavior\n  - slug: missing-tracking\n    group: recovery\n    order: 30\n    depends_on: []\n    notes: Missing tracking file\n",
	)

	service := &Service{
		repoRoot: repoRoot,
		specsDir: filepath.Join(repoRoot, ".specs"),
	}

	_, _, _, err := service.RemoveSpecFromCharter("runtime", "session-lifecycle")
	failure := requireFailure(t, err, "VALIDATION_FAILED")
	state, ok := failure.State.(CharterProjection)
	if !ok {
		t.Fatalf("state type = %T, want CharterProjection", failure.State)
	}
	if state.Validation.Valid {
		t.Fatalf("state.validation = %#v", state.Validation)
	}
	if len(state.OrderedSpecs) != 2 || state.OrderedSpecs[0].Slug != "session-lifecycle" || state.OrderedSpecs[1].Slug != "missing-tracking" {
		t.Fatalf("ordered_specs = %#v", state.OrderedSpecs)
	}
	requireFindingCode(t, state.Focus.(map[string]any)["validation"].(map[string]any)["findings"].([]any), "CHARTER_SPEC_MISSING")
	charterBytes, readErr := os.ReadFile(filepath.Join(repoRoot, ".specs", "runtime", "CHARTER.yaml"))
	if readErr != nil {
		t.Fatalf("read charter: %v", readErr)
	}
	if !strings.Contains(string(charterBytes), "slug: session-lifecycle") {
		t.Fatalf("charter should remain unchanged:\n%s", charterBytes)
	}
}

func TestSpecCreateWritesSkipsValidatedExistingDesignDocWrite(t *testing.T) {
	charter := &domain.Charter{Name: "runtime"}
	tracking := &domain.TrackingFile{
		Slug:      "session-lifecycle",
		Charter:   "runtime",
		Title:     "Session Lifecycle",
		Documents: domain.Documents{Primary: "runtime/src/domain/session_execution/SPEC.md"},
	}

	writes, err := infrastructure.PlanSpecCreateWrites(
		infrastructure.NewWorkspace("/repo"),
		charter,
		tracking,
		infrastructure.DesignDocMutation{
			Action:  "validated_existing",
			Content: []byte("unchanged"),
		},
	)
	if err != nil {
		t.Fatalf("PlanSpecCreateWrites: %v", err)
	}

	if len(writes) != 2 {
		t.Fatalf("write count = %d, want 2", len(writes))
	}
	for _, write := range writes {
		if write.Path == "/repo/runtime/src/domain/session_execution/SPEC.md" {
			t.Fatalf("unexpected design doc write: %#v", write)
		}
	}
}

func copyApplicationFixtureRepo(t *testing.T, fixture string) string {
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

func replaceFileText(t *testing.T, path, oldValue, newValue string) {
	t.Helper()

	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	updated := strings.Replace(string(content), oldValue, newValue, 1)
	if updated == string(content) {
		t.Fatalf("did not find %q in %s", oldValue, path)
	}
	if err := os.WriteFile(path, []byte(updated), 0644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func removeCharterEntry(t *testing.T, path, slug string) {
	t.Helper()

	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	lines := strings.Split(string(content), "\n")
	updatedLines := make([]string, 0, len(lines))
	removing := false
	removed := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(line, "  - slug: ") {
			candidate := strings.TrimSpace(strings.TrimPrefix(line, "  - slug: "))
			removing = candidate == slug
			if removing {
				removed = true
				continue
			}
		}
		if removing {
			if strings.HasPrefix(line, "    ") || trimmed == "" {
				continue
			}
			removing = false
		}
		updatedLines = append(updatedLines, line)
	}
	if !removed {
		t.Fatalf("did not find charter entry for %q in %s", slug, path)
	}
	updated := strings.Join(updatedLines, "\n")
	if err := os.WriteFile(path, []byte(updated), 0644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func writeApplicationTestFile(t *testing.T, path string, data []byte) {
	t.Helper()

	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func assertCanonicalCharterValidationState(t *testing.T, state CharterProjection, expectedPresentSlug, expectedInvalidSlug string) {
	t.Helper()

	if state.Name != "runtime" || state.TrackingFile != ".specs/runtime/CHARTER.yaml" {
		t.Fatalf("state = %#v", state)
	}

	seenPresentSlug := false
	seenInvalidSlug := false
	for _, entry := range state.OrderedSpecs {
		if entry.Slug == expectedPresentSlug {
			seenPresentSlug = true
		}
		if entry.Slug == expectedInvalidSlug {
			seenInvalidSlug = true
			if entry.Validation.Valid {
				t.Fatalf("ordered spec validation = %#v, want invalid overlay for %s", entry.Validation, expectedInvalidSlug)
			}
		}
	}
	if !seenPresentSlug {
		t.Fatalf("ordered_specs = %#v, missing %s", state.OrderedSpecs, expectedPresentSlug)
	}
	if !seenInvalidSlug {
		t.Fatalf("ordered_specs = %#v, missing %s", state.OrderedSpecs, expectedInvalidSlug)
	}
}

func requireFailure(t *testing.T, err error, code string) *Failure {
	t.Helper()

	failure, ok := err.(*Failure)
	if !ok {
		t.Fatalf("error type = %T, want *Failure", err)
	}
	if failure.Code != code {
		t.Fatalf("failure code = %q, want %q", failure.Code, code)
	}
	return failure
}

func pointerTo[T any](value T) *T {
	return &value
}
