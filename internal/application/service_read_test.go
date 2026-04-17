package application

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/aitoroses/specctl/internal/domain"
	"github.com/aitoroses/specctl/internal/infrastructure"
)

func TestReadContextLenientStoredState(t *testing.T) {
	t.Run("spec projection survives malformed tracking state", func(t *testing.T) {
		repoRoot := copyApplicationFixtureRepo(t, "malformed-gapful-spec")
		service := &Service{repoRoot: repoRoot, specsDir: filepath.Join(repoRoot, ".specs")}

		stateAny, next, err := service.ReadContext("runtime:session-lifecycle", "")
		if err != nil {
			t.Fatalf("ReadContext: %v", err)
		}
		if len(next) != 1 {
			t.Fatalf("next = %#v", next)
		}

		state, ok := stateAny.(SpecProjection)
		if !ok {
			t.Fatalf("state type = %T", stateAny)
		}
		if state.Slug != "session-lifecycle" {
			t.Fatalf("slug = %q", state.Slug)
		}
		if state.Validation.Valid || len(state.Validation.Findings) == 0 {
			t.Fatalf("validation = %#v", state.Validation)
		}
	})

	t.Run("charter projection survives malformed charter state", func(t *testing.T) {
		repoRoot := copyApplicationFixtureRepo(t, "charter-cycle")
		service := &Service{repoRoot: repoRoot, specsDir: filepath.Join(repoRoot, ".specs")}

		stateAny, next, err := service.ReadContext("runtime", "")
		if err != nil {
			t.Fatalf("ReadContext: %v", err)
		}
		if len(next) != 0 {
			t.Fatalf("next = %#v", next)
		}

		state, ok := stateAny.(CharterProjection)
		if !ok {
			t.Fatalf("state type = %T", stateAny)
		}
		if state.Name != "runtime" || len(state.OrderedSpecs) != 2 {
			t.Fatalf("state = %#v", state)
		}
		if state.Validation.Valid || len(state.Validation.Findings) == 0 {
			t.Fatalf("validation = %#v", state.Validation)
		}
	})

	t.Run("charter projection maps missing charter metadata to charter validation codes", func(t *testing.T) {
		repoRoot := copyApplicationFixtureRepo(t, "ready-spec")
		replaceFileText(t, filepath.Join(repoRoot, ".specs", "runtime", "CHARTER.yaml"), "title: Runtime System\n", "")
		replaceFileText(t, filepath.Join(repoRoot, ".specs", "runtime", "CHARTER.yaml"), "description: Specs for runtime control-plane and data-plane behavior\n", "")
		service := &Service{repoRoot: repoRoot, specsDir: filepath.Join(repoRoot, ".specs")}

		stateAny, next, err := service.ReadContext("runtime", "")
		if err != nil {
			t.Fatalf("ReadContext: %v", err)
		}
		if len(next) != 0 {
			t.Fatalf("next = %#v", next)
		}

		state, ok := stateAny.(CharterProjection)
		if !ok {
			t.Fatalf("state type = %T", stateAny)
		}
		if state.Name != "runtime" {
			t.Fatalf("state = %#v", state)
		}
		if state.Validation.Valid || len(state.Validation.Findings) == 0 {
			t.Fatalf("validation = %#v", state.Validation)
		}

		foundCharterCode := false
		for _, raw := range state.Validation.Findings {
			finding, ok := raw.(infrastructure.ValidationFinding)
			if !ok {
				t.Fatalf("finding type = %T", raw)
			}
			if finding.Code == "CHARTER_NAME_INVALID" {
				foundCharterCode = true
			}
			if finding.Code == "SPEC_STATUS_INVALID" {
				t.Fatalf("unexpected fallback finding %#v", finding)
			}
		}
		if !foundCharterCode {
			t.Fatalf("validation.findings = %#v", state.Validation.Findings)
		}
	})

	t.Run("charter projection keeps per-spec validation for missing tracking files", func(t *testing.T) {
		repoRoot := copyApplicationFixtureRepo(t, "charter-missing-tracking")
		service := &Service{repoRoot: repoRoot, specsDir: filepath.Join(repoRoot, ".specs")}

		stateAny, next, err := service.ReadContext("runtime", "")
		if err != nil {
			t.Fatalf("ReadContext: %v", err)
		}
		if len(next) != 0 {
			t.Fatalf("next = %#v", next)
		}

		state, ok := stateAny.(CharterProjection)
		if !ok {
			t.Fatalf("state type = %T", stateAny)
		}
		var spec *OrderedCharterSpecEntry
		for i := range state.OrderedSpecs {
			if state.OrderedSpecs[i].Slug == "session-lifecycle" {
				spec = &state.OrderedSpecs[i]
				break
			}
		}
		if spec == nil {
			t.Fatalf("ordered_specs = %#v", state.OrderedSpecs)
		}
		if spec.Validation.Valid {
			t.Fatalf("ordered_spec.validation = %#v", spec.Validation)
		}
		requireFindingCode(t, spec.Validation.Findings, "CHARTER_SPEC_MISSING")
	})
}

func TestReadDiffLenientStoredState(t *testing.T) {
	t.Run("spec diff returns canonical projection with validation findings", func(t *testing.T) {
		repoRoot := copyApplicationFixtureRepo(t, "malformed-gapful-spec")
		service := &Service{repoRoot: repoRoot, specsDir: filepath.Join(repoRoot, ".specs")}

		stateAny, next, err := service.ReadDiff("runtime:session-lifecycle", "")
		if err != nil {
			t.Fatalf("ReadDiff: %v", err)
		}
		if len(next) != 1 || next[0].(map[string]any)["action"] != "sync" {
			t.Fatalf("next = %#v", next)
		}

		state, ok := stateAny.(SpecDiffProjection)
		if !ok {
			t.Fatalf("state type = %T", stateAny)
		}
		if state.Target != "runtime:session-lifecycle" || state.Baseline != "checkpoint" || state.From != nil {
			t.Fatalf("state = %#v", state)
		}
		if state.To.Rev != 1 || state.To.Checkpoint != "a1b2c3f" || state.To.Status != "ready" {
			t.Fatalf("to = %#v", state.To)
		}
		if state.Model.Status.From != nil || state.Model.Status.To != "ready" {
			t.Fatalf("model.status = %#v", state.Model.Status)
		}
		if state.Validation.Valid || len(state.Validation.Findings) == 0 {
			t.Fatalf("validation = %#v", state.Validation)
		}
	})

	t.Run("charter diff returns canonical projection with validation findings", func(t *testing.T) {
		repoRoot := copyApplicationFixtureRepo(t, "charter-cycle")
		service := &Service{repoRoot: repoRoot, specsDir: filepath.Join(repoRoot, ".specs")}

		stateAny, next, err := service.ReadDiff("", "runtime")
		if err != nil {
			t.Fatalf("ReadDiff: %v", err)
		}
		if len(next) != 0 {
			t.Fatalf("next = %#v", next)
		}

		state, ok := stateAny.(CharterDiffProjection)
		if !ok {
			t.Fatalf("state type = %T", stateAny)
		}
		if state.Charter != "runtime" || len(state.OrderedSpecs) != 2 {
			t.Fatalf("state = %#v", state)
		}
		if state.OrderedSpecs[0].Slug != "redis-state" || state.OrderedSpecs[1].Slug != "session-lifecycle" {
			t.Fatalf("ordered_specs = %#v", state.OrderedSpecs)
		}
		if state.Validation.Valid || len(state.Validation.Findings) == 0 {
			t.Fatalf("validation = %#v", state.Validation)
		}
	})
}

func TestReadConfigAggregatesRepoValidation(t *testing.T) {
	t.Run("aggregates charter relationship findings", func(t *testing.T) {
		repoRoot := copyApplicationFixtureRepo(t, "ready-spec")
		replaceFileText(t, filepath.Join(repoRoot, ".specs", "runtime", "CHARTER.yaml"), "slug: session-lifecycle", "slug: missing-spec")
		service := &Service{repoRoot: repoRoot, specsDir: filepath.Join(repoRoot, ".specs")}

		state, err := service.ReadConfig()
		if err != nil {
			t.Fatalf("ReadConfig: %v", err)
		}
		if state.Validation.Valid || len(state.Validation.Findings) == 0 {
			t.Fatalf("validation = %#v", state.Validation)
		}
	})

	t.Run("audits config-managed design doc state", func(t *testing.T) {
		repoRoot := copyApplicationFixtureRepo(t, "ready-spec")
		replaceFileText(t, filepath.Join(repoRoot, "runtime", "src", "domain", "session_execution", "SPEC.md"), "charter: runtime", "charter: runtime\nformat: unknown-format")
		service := &Service{repoRoot: repoRoot, specsDir: filepath.Join(repoRoot, ".specs")}

		state, err := service.ReadConfig()
		if err != nil {
			t.Fatalf("ReadConfig: %v", err)
		}
		if state.Validation.Valid {
			t.Fatalf("validation = %#v", state.Validation)
		}
	})

	t.Run("survives malformed persisted config state", func(t *testing.T) {
		repoRoot := copyApplicationFixtureRepo(t, "ready-spec")
		replaceFileText(t, filepath.Join(repoRoot, ".specs", "specctl.yaml"), "  - domain\n", "  - domain\n  - manual\n")
		service := &Service{repoRoot: repoRoot, specsDir: filepath.Join(repoRoot, ".specs")}

		state, err := service.ReadConfig()
		if err != nil {
			t.Fatalf("ReadConfig: %v", err)
		}
		if state.Validation.Valid {
			t.Fatalf("validation = %#v", state.Validation)
		}
	})

	t.Run("surfaces tracking files whose charter file is missing", func(t *testing.T) {
		repoRoot := copyApplicationFixtureRepo(t, "ready-spec")
		if err := os.Remove(filepath.Join(repoRoot, ".specs", "runtime", "CHARTER.yaml")); err != nil {
			t.Fatalf("remove charter: %v", err)
		}
		service := &Service{repoRoot: repoRoot, specsDir: filepath.Join(repoRoot, ".specs")}

		state, err := service.ReadConfig()
		if err != nil {
			t.Fatalf("ReadConfig: %v", err)
		}
		requireFindingCode(t, state.Validation.Findings, "CHARTER_SPEC_MISSING")
	})

	t.Run("surfaces requirements that reference tags missing from config", func(t *testing.T) {
		repoRoot := copyApplicationFixtureRepo(t, "active-spec")
		replaceFileText(t, filepath.Join(repoRoot, ".specs", "runtime", "session-lifecycle.yaml"), "    tags:\n      - runtime\n      - e2e\n", "    tags:\n      - runtime\n      - adapter\n")
		service := &Service{repoRoot: repoRoot, specsDir: filepath.Join(repoRoot, ".specs")}

		state, err := service.ReadConfig()
		if err != nil {
			t.Fatalf("ReadConfig: %v", err)
		}
		requireFindingCode(t, state.Validation.Findings, "REQUIREMENT_TAG_NOT_CONFIGURED")
	})

	t.Run("surfaces missing scope directories from tracked specs", func(t *testing.T) {
		repoRoot := copyApplicationFixtureRepo(t, "ready-spec")
		replaceFileText(t, filepath.Join(repoRoot, ".specs", "runtime", "session-lifecycle.yaml"), "runtime/src/application/commands/", "runtime/src/application/missing/")
		service := &Service{repoRoot: repoRoot, specsDir: filepath.Join(repoRoot, ".specs")}

		state, err := service.ReadConfig()
		if err != nil {
			t.Fatalf("ReadConfig: %v", err)
		}
		requireFindingCode(t, state.Validation.Findings, "SCOPE_PATH_INVALID")
	})
}

func TestReadContextRegistrySeparatesConfigValidationFromAudit(t *testing.T) {
	repoRoot := copyApplicationFixtureRepo(t, "ready-spec")
	replaceFileText(t, filepath.Join(repoRoot, ".specs", "runtime", "CHARTER.yaml"), "slug: session-lifecycle", "slug: missing-spec")
	service := &Service{repoRoot: repoRoot, specsDir: filepath.Join(repoRoot, ".specs")}

	stateAny, next, err := service.ReadContext("", "")
	if err != nil {
		t.Fatalf("ReadContext: %v", err)
	}
	if len(next) != 0 {
		t.Fatalf("next = %#v", next)
	}

	state, ok := stateAny.(RegistryProjection)
	if !ok {
		t.Fatalf("state type = %T", stateAny)
	}
	if !state.Config.Validation.Valid {
		t.Fatalf("config.validation = %#v, want config-only validation", state.Config.Validation)
	}
	if len(state.Config.Validation.Findings) != 0 {
		t.Fatalf("config.validation.findings = %#v, want config-only validation", state.Config.Validation.Findings)
	}
	requireFindingCode(t, state.Audit.Findings, "CHARTER_SPEC_MISSING")

	configState, err := service.ReadConfig()
	if err != nil {
		t.Fatalf("ReadConfig: %v", err)
	}
	requireFindingCode(t, configState.Validation.Findings, "CHARTER_SPEC_MISSING")
}

func TestReadContextComputesScopeDrift(t *testing.T) {
	t.Run("verified spec becomes drifted after a scoped file changes after sync", func(t *testing.T) {
		repoRoot := copyApplicationFixtureRepo(t, "verified-spec")
		initGitRepoAtDate(t, repoRoot, "2026-03-28T12:00:00Z")
		service := &Service{repoRoot: repoRoot, specsDir: filepath.Join(repoRoot, ".specs")}

		stateAny, next, err := service.ReadContext("runtime:session-lifecycle", "")
		if err != nil {
			t.Fatalf("ReadContext initial: %v", err)
		}
		if len(next) != 0 {
			t.Fatalf("initial next = %#v", next)
		}

		state := stateAny.(SpecProjection)
		if state.ScopeDrift.Status != "clean" || len(state.ScopeDrift.FilesChangedSinceCheckpoint) != 0 {
			t.Fatalf("initial scope_drift = %#v", state.ScopeDrift)
		}
		if state.ScopeDrift.DriftSource != nil {
			t.Fatalf("initial drift_source = %#v", state.ScopeDrift.DriftSource)
		}
		encoded, err := json.Marshal(state)
		if err != nil {
			t.Fatalf("Marshal: %v", err)
		}
		if !strings.Contains(string(encoded), `"drift_source":null`) {
			t.Fatalf("serialized scope_drift = %s", string(encoded))
		}

		replaceFileText(t, filepath.Join(repoRoot, "runtime", "src", "domain", "session_execution", "SPEC.md"), "# Session Lifecycle", "# Session Lifecycle\n\n## Drift Review\n\nUpdated after sync.")
		runGitAtDate(t, repoRoot, "2026-03-30T09:30:00Z", "add", ".")
		runGitAtDate(t, repoRoot, "2026-03-30T09:30:00Z", "commit", "-m", "drifted scope change")

		stateAny, next, err = service.ReadContext("runtime:session-lifecycle", "")
		if err != nil {
			t.Fatalf("ReadContext drifted: %v", err)
		}
		if len(next) != 1 {
			t.Fatalf("drifted next = %#v", next)
		}

		state = stateAny.(SpecProjection)
		if state.ScopeDrift.Status != "drifted" {
			t.Fatalf("scope_drift.status = %q", state.ScopeDrift.Status)
		}
		if state.ScopeDrift.DriftSource == nil || *state.ScopeDrift.DriftSource != "design_doc" {
			t.Fatalf("drift_source = %#v", state.ScopeDrift.DriftSource)
		}
		focus, ok := state.Focus.(map[string]any)
		if !ok {
			t.Fatalf("focus = %#v", state.Focus)
		}
		scopeDrift, ok := focus["scope_drift"].(map[string]any)
		if !ok {
			t.Fatalf("focus.scope_drift = %#v", focus)
		}
		if _, exists := focus["drift"]; exists {
			t.Fatalf("focus = %#v, want no legacy drift key", focus)
		}
		if scopeDrift["status"] != "drifted" || scopeDrift["drift_source"] != "design_doc" {
			t.Fatalf("focus.scope_drift = %#v", scopeDrift)
		}
		if strings.Join(state.ScopeDrift.FilesChangedSinceCheckpoint, ",") != "runtime/src/domain/session_execution/SPEC.md" {
			t.Fatalf("files_changed_since_checkpoint = %#v", state.ScopeDrift.FilesChangedSinceCheckpoint)
		}
		if next[0].(map[string]any)["action"] != "review_diff" {
			t.Fatalf("next = %#v", next)
		}
	})

	t.Run("tracked drift returns executable requirement verification guidance", func(t *testing.T) {
		repoRoot := copyApplicationFixtureRepo(t, "verified-spec")
		initGitRepoAtDate(t, repoRoot, "2026-03-28T12:00:00Z")
		replaceFileText(t, filepath.Join(repoRoot, "runtime", "src", "domain", "session_execution", "SPEC.md"), "# Session Lifecycle", "# Session Lifecycle\n\n## Drift Review\n\nTracked drift update.\n")
		runGitAtDate(t, repoRoot, "2026-03-30T09:30:00Z", "add", ".")
		runGitAtDate(t, repoRoot, "2026-03-30T09:30:00Z", "commit", "-m", "tracked drift")

		service := &Service{repoRoot: repoRoot, specsDir: filepath.Join(repoRoot, ".specs")}
		_, _, next, err := service.AddDelta(DeltaAddRequest{
			Target:         "runtime:session-lifecycle",
			Intent:         domain.DeltaIntentAdd,
			Area:           "Tracked drift review",
			Current:        "The committed drift is not yet tracked",
			CurrentPresent: true,
			Targets:        "Track the committed design-doc change",
			TargetPresent:  true,
			Notes:          "Opened after the drift commit",
			NotesPresent:   true,
		})
		if err != nil {
			t.Fatalf("AddDelta: %v", err)
		}
		if len(next) < 2 || next[0].(map[string]any)["action"] != "write_spec_section" || next[1].(map[string]any)["action"] != "add_requirement" {
			t.Fatalf("delta add next = %#v", next)
		}
		runGitAtDate(t, repoRoot, "2026-03-30T09:35:00Z", "add", ".specs/runtime/session-lifecycle.yaml")
		runGitAtDate(t, repoRoot, "2026-03-30T09:35:00Z", "commit", "-m", "track committed drift")
		replaceFileText(t, filepath.Join(repoRoot, "runtime", "src", "domain", "session_execution", "SPEC.md"), "## Drift Review\n\nTracked drift update.\n", "## Requirement: Tracked drift review\n\n```gherkin requirement\n@runtime @e2e\nFeature: Tracked drift review\n```\n\n### Scenarios\n\n```gherkin scenario\nScenario: Committed drift is fully covered\n  Given the design doc changed after the checkpoint\n  When the covering delta is opened\n  Then the tracked drift continues through requirement verification\n```\n\n## Drift Review\n\nTracked drift update.\n")
		runGitAtDate(t, repoRoot, "2026-03-30T09:36:00Z", "add", "runtime/src/domain/session_execution/SPEC.md")
		runGitAtDate(t, repoRoot, "2026-03-30T09:36:00Z", "commit", "-m", "record tracked drift requirement")

		_, _, reqNext, err := service.AddRequirement(RequirementAddRequest{
			Target:  "runtime:session-lifecycle",
			DeltaID: "D-002",
			Gherkin: "@runtime @e2e\nFeature: Tracked drift review\n\n  Scenario: Committed drift is fully covered\n    Given the design doc changed after the checkpoint\n    When the covering delta is opened\n    Then the tracked drift continues through requirement verification\n",
		})
		if err != nil {
			t.Fatalf("AddRequirement: %v", err)
		}
		if len(reqNext) == 0 {
			t.Fatalf("req add next = %#v", reqNext)
		}

		stateAny, next, err := service.ReadContext("runtime:session-lifecycle", "")
		if err != nil {
			t.Fatalf("ReadContext: %v", err)
		}
		state := stateAny.(SpecProjection)
		if state.ScopeDrift.Status != "tracked" {
			t.Fatalf("scope_drift = %#v", state.ScopeDrift)
		}
		if len(next) != 1 {
			t.Fatalf("next = %#v", next)
		}
		action := next[0].(map[string]any)
		if action["action"] != "verify_requirement" && action["action"] != "write_e2e_test_and_verify" {
			t.Fatalf("next = %#v", next)
		}
		template := action["template"].(map[string]any)
		argv := template["argv"].([]string)
		if got := strings.Join(argv, " "); !strings.HasPrefix(got, "specctl req verify runtime:session-lifecycle REQ-002 --test-file runtime/tests/e2e/journeys/") {
			t.Fatalf("argv = %v", argv)
		}
		required := template["required_fields"].([]map[string]any)
		if len(required) != 0 {
			t.Fatalf("required_fields = %#v", required)
		}
	})

	t.Run("dirty working tree preserves repo-relative paths in stage guidance", func(t *testing.T) {
		repoRoot := copyApplicationFixtureRepo(t, "verified-spec")
		initGitRepoAtDate(t, repoRoot, "2026-03-28T12:00:00Z")
		docPath := filepath.Join(repoRoot, "runtime", "src", "domain", "session_execution", "SPEC.md")
		replaceFileText(t, docPath, "# Session Lifecycle", "# Session Lifecycle\n\n## Dirty Drift\n\nPending change.\n")

		service := &Service{repoRoot: repoRoot, specsDir: filepath.Join(repoRoot, ".specs")}
		stateAny, next, err := service.ReadContext("runtime:session-lifecycle", "")
		if err != nil {
			t.Fatalf("ReadContext: %v", err)
		}
		state := stateAny.(SpecProjection)
		if got := strings.Join(state.UncommittedChanges, ","); got != "runtime/src/domain/session_execution/SPEC.md" {
			t.Fatalf("uncommitted_changes = %#v", state.UncommittedChanges)
		}
		if len(next) < 1 {
			t.Fatalf("next = %#v", next)
		}
		template := next[0].(map[string]any)["template"].(map[string]any)
		argv := template["argv"].([]string)
		if got := strings.Join(argv, " "); got != "git add -- runtime/src/domain/session_execution/SPEC.md" {
			t.Fatalf("argv = %v", argv)
		}
	})

	t.Run("context warns when git metadata is unavailable", func(t *testing.T) {
		repoRoot := copyApplicationFixtureRepo(t, "verified-spec")
		service := &Service{repoRoot: repoRoot, specsDir: filepath.Join(repoRoot, ".specs")}

		stateAny, next, err := service.ReadContext("runtime:session-lifecycle", "")
		if err != nil {
			t.Fatalf("ReadContext: %v", err)
		}
		if len(next) != 1 {
			t.Fatalf("next = %#v", next)
		}

		state := stateAny.(SpecProjection)
		if state.ScopeDrift.Status != "unavailable" {
			t.Fatalf("scope_drift = %#v", state.ScopeDrift)
		}
		focus, ok := state.Focus.(map[string]any)
		if !ok {
			t.Fatalf("focus = %#v", state.Focus)
		}
		scopeDrift, ok := focus["scope_drift"].(map[string]any)
		if !ok {
			t.Fatalf("focus.scope_drift = %#v", focus)
		}
		if scopeDrift["status"] != "unavailable" {
			t.Fatalf("focus.scope_drift = %#v", scopeDrift)
		}
		if next[0].(map[string]any)["action"] != "sync_checkpoint" {
			t.Fatalf("next = %#v", next)
		}
		found := false
		for _, raw := range state.Validation.Findings {
			finding, ok := raw.(infrastructure.ValidationFinding)
			if ok && finding.Code == "CHECKPOINT_UNAVAILABLE" && finding.Severity == "warning" {
				found = true
			}
		}
		if !found {
			t.Fatalf("validation.findings = %#v", state.Validation.Findings)
		}
	})
}

func TestBuildContextRefreshMatchIssuesIncludesAllBlockingStatuses(t *testing.T) {
	state := SpecProjection{
		Requirements: []RequirementProjection{
			{
				ID:        "REQ-001",
				Title:     "Exact mismatch",
				Lifecycle: domain.RequirementLifecycleActive,
				Match: RequirementMatchProjection{
					Status:  "no_exact_match",
					Heading: stringPointer("Requirement: Exact mismatch"),
				},
			},
			{
				ID:        "REQ-002",
				Title:     "Missing requirement",
				Lifecycle: domain.RequirementLifecycleActive,
				Match: RequirementMatchProjection{
					Status: "missing_in_spec",
				},
			},
			{
				ID:        "REQ-003",
				Title:     "Duplicate requirement",
				Lifecycle: domain.RequirementLifecycleActive,
				Match: RequirementMatchProjection{
					Status:  "duplicate_in_spec",
					Heading: stringPointer("Requirement: Duplicate requirement"),
				},
			},
			{
				ID:        "REQ-004",
				Title:     "Inactive duplicate",
				Lifecycle: domain.RequirementLifecycleSuperseded,
				Match: RequirementMatchProjection{
					Status: "duplicate_in_spec",
				},
			},
		},
	}

	issues := buildContextRefreshMatchIssues(state)
	if len(issues) != 3 {
		t.Fatalf("issues = %#v", issues)
	}
	if got := issues[0]["status"]; got != "no_exact_match" {
		t.Fatalf("issues[0] = %#v", issues[0])
	}
	if got := issues[1]["status"]; got != "missing_in_spec" {
		t.Fatalf("issues[1] = %#v", issues[1])
	}
	if got := issues[2]["status"]; got != "duplicate_in_spec" {
		t.Fatalf("issues[2] = %#v", issues[2])
	}
}

func TestReadFileContextNormalizesPathsAndRejectsRepoEscapes(t *testing.T) {
	t.Run("normalizes in-repo paths before ownership matching", func(t *testing.T) {
		repoRoot := copyApplicationFixtureRepo(t, "ready-spec")
		service := &Service{repoRoot: repoRoot, specsDir: filepath.Join(repoRoot, ".specs")}

		stateAny, next, err := service.ReadContext("", "./runtime/src/domain/session_execution/../session_execution/SPEC.md")
		if err != nil {
			t.Fatalf("ReadContext: %v", err)
		}
		if len(next) != 0 {
			t.Fatalf("next = %#v", next)
		}

		state := stateAny.(FileContextProjection)
		if state.File != "runtime/src/domain/session_execution/SPEC.md" {
			t.Fatalf("file = %q", state.File)
		}
		if state.Resolution != "matched" || state.MatchSource == nil || *state.MatchSource != "design_doc" {
			t.Fatalf("state = %#v", state)
		}
		if state.GoverningSpec == nil || state.GoverningSpec.Slug != "session-lifecycle" {
			t.Fatalf("governing_spec = %#v", state.GoverningSpec)
		}
	})

	t.Run("returns only spec create when a file suggests a missing charter", func(t *testing.T) {
		repoRoot := copyApplicationFixtureRepo(t, "ready-spec")
		if err := os.MkdirAll(filepath.Join(repoRoot, "adapters", "src", "http"), 0o755); err != nil {
			t.Fatalf("mkdir adapters/src/http: %v", err)
		}
		if err := os.WriteFile(filepath.Join(repoRoot, "adapters", "src", "http", "client.py"), []byte("pass\n"), 0o644); err != nil {
			t.Fatalf("write adapters client: %v", err)
		}
		service := &Service{repoRoot: repoRoot, specsDir: filepath.Join(repoRoot, ".specs")}

		stateAny, next, err := service.ReadContext("", "adapters/src/http/client.py")
		if err != nil {
			t.Fatalf("ReadContext: %v", err)
		}
		if len(next) != 1 {
			t.Fatalf("next = %#v", next)
		}

		state := stateAny.(FileContextProjection)
		if state.File != "adapters/src/http/client.py" || state.Resolution != "unmatched" {
			t.Fatalf("state = %#v", state)
		}
		if next[0].(map[string]any)["action"] != "create_spec" {
			t.Fatalf("next = %#v", next)
		}
	})

	t.Run("rejects file paths that escape the repo root", func(t *testing.T) {
		repoRoot := copyApplicationFixtureRepo(t, "ready-spec")
		service := &Service{repoRoot: repoRoot, specsDir: filepath.Join(repoRoot, ".specs")}

		_, next, err := service.ReadContext("", "../outside.go")
		if err == nil {
			t.Fatal("ReadContext succeeded, want failure")
		}
		if next != nil {
			t.Fatalf("next = %#v", next)
		}

		failure, ok := err.(*Failure)
		if !ok {
			t.Fatalf("error type = %T", err)
		}
		if failure.Code != "INVALID_INPUT" {
			t.Fatalf("code = %q", failure.Code)
		}
		state, ok := failure.State.(FileContextProjection)
		if !ok {
			t.Fatalf("state type = %T", failure.State)
		}
		if state.File != "../outside.go" || state.Focus == nil {
			t.Fatalf("state = %#v", state)
		}
		focus := state.Focus.(map[string]any)
		if got := focus["invalid_paths"].([]string); len(got) != 1 || got[0] != "../outside.go" {
			t.Fatalf("invalid_paths = %#v", focus["invalid_paths"])
		}
	})

	t.Run("same-charter equal-scope ties remain ambiguous", func(t *testing.T) {
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

		service := &Service{repoRoot: repoRoot, specsDir: filepath.Join(repoRoot, ".specs")}
		stateAny, next, err := service.ReadContext("", "runtime/src/domain/shared/transport.py")
		if err != nil {
			t.Fatalf("ReadContext: %v", err)
		}
		state := stateAny.(FileContextProjection)
		if state.Resolution != "ambiguous" || state.GoverningSpec != nil {
			t.Fatalf("state = %#v", state)
		}
		if state.MatchSource == nil || *state.MatchSource != "scope" {
			t.Fatalf("match_source = %#v", state.MatchSource)
		}
		if state.Validation.Valid || len(state.Validation.Findings) != 1 {
			t.Fatalf("validation = %#v", state.Validation)
		}
		finding, ok := state.Validation.Findings[0].(infrastructure.ValidationFinding)
		if !ok || finding.Code != "AMBIGUOUS_FILE_OWNERSHIP" {
			t.Fatalf("validation.findings = %#v", state.Validation.Findings)
		}
		if got := []string{state.Matches[0].Slug, state.Matches[1].Slug}; strings.Join(got, ",") != "first-owner,second-owner" {
			t.Fatalf("matches = %#v", state.Matches)
		}
		if len(next) != 0 {
			t.Fatalf("next = %#v", next)
		}
	})
}

func TestReadDiffContracts(t *testing.T) {
	t.Run("initial baseline returns canonical empty delta and requirement buckets", func(t *testing.T) {
		repoRoot := copyApplicationFixtureRepo(t, "ready-spec")
		replaceFileText(t, filepath.Join(repoRoot, ".specs", "runtime", "session-lifecycle.yaml"), "status: ready", "status: active")
		replaceFileText(t, filepath.Join(repoRoot, ".specs", "runtime", "session-lifecycle.yaml"), "requirements: []\nchangelog: []\n", `requirements:
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
		service := &Service{repoRoot: repoRoot, specsDir: filepath.Join(repoRoot, ".specs")}

		stateAny, next, err := service.ReadDiff("runtime:session-lifecycle", "")
		if err != nil {
			t.Fatalf("ReadDiff: %v", err)
		}
		if len(next) != 1 || next[0].(map[string]any)["action"] != "sync" {
			t.Fatalf("next = %#v", next)
		}

		state, ok := stateAny.(SpecDiffProjection)
		if !ok {
			t.Fatalf("state type = %T", stateAny)
		}
		if state.Baseline != "checkpoint" || state.From != nil {
			t.Fatalf("baseline/from = %#v %#v", state.Baseline, state.From)
		}
		if state.To.Rev != 1 || state.To.Checkpoint != "a1b2c3f" || state.To.Status != "active" {
			t.Fatalf("to = %#v", state.To)
		}
		if state.Model.Status.From != nil || state.Model.Status.To != "active" {
			t.Fatalf("model.status = %#v", state.Model.Status)
		}
		if got := strings.Join(state.Model.SpecTags.Added, ","); got != "runtime,domain" || len(state.Model.SpecTags.Removed) != 0 {
			t.Fatalf("model.spec_tags = %#v", state.Model.SpecTags)
		}
		if state.Model.Documents.PrimaryFrom != nil || state.Model.Documents.PrimaryTo != "runtime/src/domain/session_execution/SPEC.md" {
			t.Fatalf("model.documents = %#v", state.Model.Documents)
		}
		if got := strings.Join(state.Model.Scope.Added, ","); got != "runtime/src/domain/session_execution/,runtime/src/application/commands/" || len(state.Model.Scope.Removed) != 0 {
			t.Fatalf("model.scope = %#v", state.Model.Scope)
		}
		if len(state.Model.Deltas.Opened) != 0 || len(state.Model.Deltas.Closed) != 0 || len(state.Model.Deltas.Deferred) != 0 || len(state.Model.Deltas.Resumed) != 0 {
			t.Fatalf("deltas = %#v", state.Model.Deltas)
		}
		if len(state.Model.Requirements.Added) != 0 || len(state.Model.Requirements.Verified) != 0 {
			t.Fatalf("requirements = %#v", state.Model.Requirements)
		}
		if state.DesignDoc.Changed || len(state.DesignDoc.SectionsChanged) != 0 {
			t.Fatalf("design_doc = %#v", state.DesignDoc)
		}
	})

	t.Run("checkpoint baseline tracking lookup surfaces checkpoint unavailable validation", func(t *testing.T) {
		repoRoot := copyApplicationFixtureRepo(t, "verified-spec")
		initGitRepoAtDate(t, repoRoot, "2026-03-28T12:00:00Z")
		replaceTrackingCheckpoint(t, filepath.Join(repoRoot, ".specs", "runtime", "session-lifecycle.yaml"), "deadbee")
		service := &Service{repoRoot: repoRoot, specsDir: filepath.Join(repoRoot, ".specs")}

		stateAny, next, err := service.ReadDiff("runtime:session-lifecycle", "")
		if err != nil {
			t.Fatalf("ReadDiff: %v", err)
		}
		if len(next) != 1 || next[0].(map[string]any)["action"] != "sync" {
			t.Fatalf("next = %#v", next)
		}

		state := stateAny.(SpecDiffProjection)
		if state.Baseline != "checkpoint" || state.From != nil {
			t.Fatalf("baseline/from = %#v %#v", state.Baseline, state.From)
		}
		if state.Validation.Valid {
			t.Fatalf("validation = %#v", state.Validation)
		}
		requireFindingCode(t, state.Validation.Findings, "CHECKPOINT_UNAVAILABLE")
	})

	t.Run("checkpoint baseline design doc lookup surfaces checkpoint unavailable validation", func(t *testing.T) {
		repoRoot := copyApplicationFixtureRepo(t, "verified-spec")
		trackingPath := filepath.Join(repoRoot, ".specs", "runtime", "session-lifecycle.yaml")
		replaceFileText(t, trackingPath, "primary: runtime/src/domain/session_execution/SPEC.md", "primary: runtime/src/domain/missing/SPEC.md")
		initGitRepoAtDate(t, repoRoot, "2026-03-28T12:00:00Z")
		headSHA := strings.TrimSpace(runGitAtDate(t, repoRoot, "2026-03-28T12:00:00Z", "rev-parse", "HEAD"))
		replaceFileText(t, trackingPath, "primary: runtime/src/domain/missing/SPEC.md", "primary: runtime/src/domain/session_execution/SPEC.md")
		replaceTrackingCheckpoint(t, trackingPath, headSHA)
		service := &Service{repoRoot: repoRoot, specsDir: filepath.Join(repoRoot, ".specs")}

		stateAny, next, err := service.ReadDiff("runtime:session-lifecycle", "")
		if err != nil {
			t.Fatalf("ReadDiff: %v", err)
		}
		if len(next) != 0 {
			t.Fatalf("next = %#v", next)
		}

		state := stateAny.(SpecDiffProjection)
		if state.Baseline != "checkpoint" || state.From == nil || state.From.Rev != 3 {
			t.Fatalf("baseline/from = %#v %#v", state.Baseline, state.From)
		}
		if !state.DesignDoc.Changed || len(state.DesignDoc.SectionsChanged) == 0 {
			t.Fatalf("design_doc = %#v", state.DesignDoc)
		}
		if !state.Validation.Valid || len(state.Validation.Findings) != 0 {
			t.Fatalf("validation = %#v", state.Validation)
		}
	})

	t.Run("duplicate headings diff by normalized heading plus ordinal occurrence", func(t *testing.T) {
		repoRoot := copyApplicationFixtureRepo(t, "verified-spec")
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
		initGitRepoAtDate(t, repoRoot, "2026-03-28T12:00:00Z")
		headSHA := runGitAtDate(t, repoRoot, "2026-03-28T12:00:00Z", "rev-parse", "HEAD")
		replaceTrackingCheckpoint(t, filepath.Join(repoRoot, ".specs", "runtime", "session-lifecycle.yaml"), strings.TrimSpace(headSHA))
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

		service := &Service{repoRoot: repoRoot, specsDir: filepath.Join(repoRoot, ".specs")}
		stateAny, next, err := service.ReadDiff("runtime:session-lifecycle", "")
		if err != nil {
			t.Fatalf("ReadDiff: %v", err)
		}
		if len(next) != 0 {
			t.Fatalf("next = %#v", next)
		}

		state := stateAny.(SpecDiffProjection)
		if !state.DesignDoc.Changed {
			t.Fatalf("design_doc = %#v", state.DesignDoc)
		}
		got := state.DesignDoc.SectionsChanged
		want := []DesignDocSectionDiff{
			{Heading: "Step", Type: "modified", Lines: [2]int{7, 9}},
			{Heading: "Step", Type: "modified", Lines: [2]int{10, 11}},
		}
		if len(got) != len(want) {
			t.Fatalf("sections = %#v, want %#v", got, want)
		}
		for i := range want {
			if got[i] != want[i] {
				t.Fatalf("sections[%d] = %#v, want %#v", i, got[i], want[i])
			}
		}
	})

	t.Run("charter diff marks design-doc-only drift as changed", func(t *testing.T) {
		repoRoot := copyApplicationFixtureRepo(t, "verified-spec")
		docPath := filepath.Join(repoRoot, "runtime", "src", "domain", "session_execution", "SPEC.md")
		initGitRepoAtDate(t, repoRoot, "2026-03-28T12:00:00Z")
		headSHA := strings.TrimSpace(runGitAtDate(t, repoRoot, "2026-03-28T12:00:00Z", "rev-parse", "HEAD"))
		replaceTrackingCheckpoint(t, filepath.Join(repoRoot, ".specs", "runtime", "session-lifecycle.yaml"), headSHA)
		replaceFileText(t, docPath, "# Session Lifecycle\n", "# Session Lifecycle\n\n## Drift Review\n\nUpdated after sync.\n")

		service := &Service{repoRoot: repoRoot, specsDir: filepath.Join(repoRoot, ".specs")}

		specStateAny, next, err := service.ReadDiff("runtime:session-lifecycle", "")
		if err != nil {
			t.Fatalf("ReadDiff spec: %v", err)
		}
		if len(next) != 0 {
			t.Fatalf("spec next = %#v", next)
		}
		specState := specStateAny.(SpecDiffProjection)
		if !specState.DesignDoc.Changed {
			t.Fatalf("spec design_doc = %#v", specState.DesignDoc)
		}

		charterStateAny, next, err := service.ReadDiff("", "runtime")
		if err != nil {
			t.Fatalf("ReadDiff charter: %v", err)
		}
		if len(next) != 0 {
			t.Fatalf("charter next = %#v", next)
		}
		charterState := charterStateAny.(CharterDiffProjection)
		if len(charterState.OrderedSpecs) != 1 {
			t.Fatalf("ordered_specs = %#v", charterState.OrderedSpecs)
		}
		if !charterState.OrderedSpecs[0].Changed {
			t.Fatalf("ordered_specs = %#v", charterState.OrderedSpecs)
		}
	})

	t.Run("diff returns directional next actions for drifted, tracked, and unavailable states", func(t *testing.T) {
		t.Run("design-doc drift offers semantic branches plus reviewed sync", func(t *testing.T) {
			repoRoot := copyApplicationFixtureRepo(t, "verified-spec")
			initGitRepoAtDate(t, repoRoot, "2026-03-28T12:00:00Z")
			if err := os.WriteFile(filepath.Join(repoRoot, "runtime", "src", "domain", "session_execution", "SPEC.md"), []byte(`---
spec: session-lifecycle
charter: runtime
---
# Session Lifecycle

## Overview
Document drift.
`), 0o644); err != nil {
				t.Fatalf("write design doc: %v", err)
			}
			runGitAtDate(t, repoRoot, "2026-03-30T09:30:00Z", "add", ".")
			runGitAtDate(t, repoRoot, "2026-03-30T09:30:00Z", "commit", "-m", "document drift")

			service := &Service{repoRoot: repoRoot, specsDir: filepath.Join(repoRoot, ".specs")}
			stateAny, next, err := service.ReadDiff("runtime:session-lifecycle", "")
			if err != nil {
				t.Fatalf("ReadDiff: %v", err)
			}
			state := stateAny.(SpecDiffProjection)
			reviewSurface := state.Focus.(map[string]any)["review_surface"].(map[string]any)
			if len(reviewSurface["sections_changed"].([]DesignDocSectionDiff)) == 0 {
				t.Fatalf("focus = %#v", state.Focus)
			}
			classification := reviewSurface["classification"].(map[string]any)
			if classification["housekeeping_candidate"] != true {
				t.Fatalf("classification = %#v", classification)
			}
			if len(next) != 6 || next[0].(map[string]any)["action"] != "delta_add_add" || next[1].(map[string]any)["action"] != "delta_add_change" || next[5].(map[string]any)["action"] != "sync" {
				t.Fatalf("next = %#v", next)
			}
			action := next[0].(map[string]any)
			if action["instructions"] != "The design document changed in 3 sections. Choose the semantic path that matches the observed contract change." {
				t.Fatalf("instructions = %#v", action["instructions"])
			}
			template := action["template"].(map[string]any)
			if got := strings.Join(template["argv"].([]string), " "); got != "specctl delta add runtime:session-lifecycle --intent add --area <area>" {
				t.Fatalf("argv = %v", template["argv"])
			}
			if got := requiredFieldNamesFromMaps(template["required_fields"].([]map[string]any)); strings.Join(got, ",") != "area,current,target,notes" {
				t.Fatalf("required_fields = %#v", template["required_fields"])
			}
			reviewAgain := next[1].(map[string]any)
			if reviewAgain["choose_when"] == nil {
				t.Fatalf("next option = %#v", reviewAgain)
			}
		})

		t.Run("code-only drift offers semantic branches plus sync", func(t *testing.T) {
			repoRoot := copyApplicationFixtureRepo(t, "verified-spec")
			initGitRepoAtDate(t, repoRoot, "2026-03-28T12:00:00Z")
			codePath := filepath.Join(repoRoot, "runtime", "src", "application", "commands", "handler.py")
			if err := os.WriteFile(codePath, []byte("def handle():\n    return 'drift'\n"), 0o644); err != nil {
				t.Fatalf("write code file: %v", err)
			}
			runGitAtDate(t, repoRoot, "2026-03-30T09:30:00Z", "add", "runtime/src/application/commands/handler.py")
			runGitAtDate(t, repoRoot, "2026-03-30T09:30:00Z", "commit", "-m", "code drift")

			service := &Service{repoRoot: repoRoot, specsDir: filepath.Join(repoRoot, ".specs")}
			stateAny, next, err := service.ReadDiff("runtime:session-lifecycle", "")
			if err != nil {
				t.Fatalf("ReadDiff: %v", err)
			}
			state := stateAny.(SpecDiffProjection)
			reviewSurface := state.Focus.(map[string]any)["review_surface"].(map[string]any)
			scopeCode := reviewSurface["scope_code"].(map[string]any)
			if got := strings.Join(scopeCode["changed_files"].([]string), ","); got != "runtime/src/application/commands/handler.py" {
				t.Fatalf("focus = %#v", state.Focus)
			}
			if len(next) != 6 || next[0].(map[string]any)["action"] != "delta_add_add" || next[5].(map[string]any)["action"] != "sync" {
				t.Fatalf("next = %#v", next)
			}
			action := next[0].(map[string]any)
			if action["instructions"] != "The committed drift is code-only. Review the changed files and choose whether it needs semantic tracking or only a checkpoint sync." {
				t.Fatalf("instructions = %#v", action["instructions"])
			}
			template := action["template"].(map[string]any)
			if got := strings.Join(template["argv"].([]string), " "); got != "specctl delta add runtime:session-lifecycle --intent add --area <area>" {
				t.Fatalf("argv = %v", template["argv"])
			}
			if got := requiredFieldNamesFromMaps(template["required_fields"].([]map[string]any)); strings.Join(got, ",") != "area,current,target,notes" {
				t.Fatalf("required_fields = %#v", template["required_fields"])
			}
			sync := next[5].(map[string]any)
			syncTemplate := sync["template"].(map[string]any)
			if got := strings.Join(syncTemplate["argv"].([]string), " "); got != "specctl sync runtime:session-lifecycle --checkpoint HEAD" {
				t.Fatalf("sync argv = %v", syncTemplate["argv"])
			}
		})

		t.Run("mixed drift also offers reviewed sync when nothing blocks it", func(t *testing.T) {
			repoRoot := copyApplicationFixtureRepo(t, "verified-spec")
			initGitRepoAtDate(t, repoRoot, "2026-03-28T12:00:00Z")
			if err := os.WriteFile(filepath.Join(repoRoot, "runtime", "src", "domain", "session_execution", "SPEC.md"), []byte(`---
spec: session-lifecycle
charter: runtime
---
# Session Lifecycle

## Overview
Mixed drift.
`), 0o644); err != nil {
				t.Fatalf("write design doc: %v", err)
			}
			codePath := filepath.Join(repoRoot, "runtime", "src", "application", "commands", "handler.py")
			if err := os.WriteFile(codePath, []byte("def handle():\n    return 'mixed'\n"), 0o644); err != nil {
				t.Fatalf("write code file: %v", err)
			}
			runGitAtDate(t, repoRoot, "2026-03-30T09:30:00Z", "add", ".")
			runGitAtDate(t, repoRoot, "2026-03-30T09:30:00Z", "commit", "-m", "mixed drift")

			service := &Service{repoRoot: repoRoot, specsDir: filepath.Join(repoRoot, ".specs")}
			stateAny, next, err := service.ReadDiff("runtime:session-lifecycle", "")
			if err != nil {
				t.Fatalf("ReadDiff: %v", err)
			}
			state := stateAny.(SpecDiffProjection)
			reviewSurface := state.Focus.(map[string]any)["review_surface"].(map[string]any)
			if _, ok := reviewSurface["changed_requirement_blocks"]; !ok {
				t.Fatalf("focus = %#v", state.Focus)
			}
			scopeCode := reviewSurface["scope_code"].(map[string]any)
			if got := strings.Join(scopeCode["changed_files"].([]string), ","); got != "runtime/src/application/commands/handler.py" {
				t.Fatalf("focus = %#v", state.Focus)
			}
			classification := reviewSurface["classification"].(map[string]any)
			if classification["housekeeping_candidate"] != true {
				t.Fatalf("classification = %#v", classification)
			}
			if len(next) != 6 || next[0].(map[string]any)["action"] != "delta_add_add" || next[4].(map[string]any)["action"] != "refresh_requirement" || next[5].(map[string]any)["action"] != "sync" {
				t.Fatalf("next = %#v", next)
			}
			action := next[0].(map[string]any)
			if action["instructions"] != "The committed drift spans design-doc and code changes. Review the changed sections and choose the correct workflow branch." {
				t.Fatalf("instructions = %#v", action["instructions"])
			}
			template := action["template"].(map[string]any)
			if got := strings.Join(template["argv"].([]string), " "); got != "specctl delta add runtime:session-lifecycle --intent add --area <area>" {
				t.Fatalf("argv = %v", template["argv"])
			}
			if got := requiredFieldNamesFromMaps(template["required_fields"].([]map[string]any)); strings.Join(got, ",") != "area,current,target,notes" {
				t.Fatalf("required_fields = %#v", template["required_fields"])
			}
		})

		t.Run("tracked drift continues the requirement chain", func(t *testing.T) {
			repoRoot := copyApplicationFixtureRepo(t, "verified-spec")
			initGitRepoAtDate(t, repoRoot, "2026-03-28T12:00:00Z")
			replaceFileText(t, filepath.Join(repoRoot, "runtime", "src", "domain", "session_execution", "SPEC.md"), "## Requirement: Compensation stage 4 failure cleanup\n\n```gherkin requirement\n@runtime @e2e\nFeature: Compensation stage 4 failure cleanup\n```\n\n### Scenarios\n\n```gherkin scenario\nScenario: Cleanup runs after stage 4 failure\n  Given stage 4 fails during compensation\n  When recovery completes\n  Then cleanup steps run in documented order\n```\n", "## Requirement: Compensation stage 4 failure cleanup\n\n```gherkin requirement\n@runtime @e2e\nFeature: Compensation stage 4 failure cleanup\n```\n\n### Scenarios\n\n```gherkin scenario\nScenario: Cleanup runs after stage 4 failure\n  Given stage 4 fails during compensation\n  When recovery completes\n  Then cleanup steps run in documented order\n```\n\n## Requirement: Tracked diff\n\n```gherkin requirement\n@runtime\nFeature: Tracked diff\n```\n\n### Scenarios\n\n```gherkin scenario\nScenario: Continue verification after diff review\n  Given the drift is already tracked by a delta\n  When the diff is inspected\n  Then the requirement chain continues\n```\n\n## Drift Review\n\nTracked diff.\n")
			runGitAtDate(t, repoRoot, "2026-03-30T09:30:00Z", "add", ".")
			runGitAtDate(t, repoRoot, "2026-03-30T09:30:00Z", "commit", "-m", "tracked diff")

			service := &Service{repoRoot: repoRoot, specsDir: filepath.Join(repoRoot, ".specs")}
			if _, _, _, err := service.AddDelta(DeltaAddRequest{
				Target:         "runtime:session-lifecycle",
				Intent:         domain.DeltaIntentAdd,
				Area:           "Tracked diff",
				Current:        "The committed design-doc drift is not yet tracked",
				CurrentPresent: true,
				Targets:        "Track the committed diff before convergence",
				TargetPresent:  true,
				Notes:          "Opened after the diff commit",
				NotesPresent:   true,
			}); err != nil {
				t.Fatalf("AddDelta: %v", err)
			}
			runGitAtDate(t, repoRoot, "2026-03-30T09:35:00Z", "add", ".specs/runtime/session-lifecycle.yaml")
			runGitAtDate(t, repoRoot, "2026-03-30T09:35:00Z", "commit", "-m", "track diff")
			if _, _, _, err := service.AddRequirement(RequirementAddRequest{
				Target:  "runtime:session-lifecycle",
				DeltaID: "D-002",
				Gherkin: "@runtime\nFeature: Tracked diff\n\n  Scenario: Continue verification after diff review\n    Given the drift is already tracked by a delta\n    When the diff is inspected\n    Then the requirement chain continues\n",
			}); err != nil {
				t.Fatalf("AddRequirement: %v", err)
			}

			_, next, err := service.ReadDiff("runtime:session-lifecycle", "")
			if err != nil {
				t.Fatalf("ReadDiff: %v", err)
			}
			if len(next) != 1 || next[0].(map[string]any)["action"] != "verify_requirement" {
				t.Fatalf("next = %#v", next)
			}
			action := next[0].(map[string]any)
			if action["instructions"] != "Verify the last tracing requirement before closing the delta." {
				t.Fatalf("instructions = %#v", action["instructions"])
			}
			template := action["template"].(map[string]any)
			if got := requiredFieldNamesFromMaps(template["required_fields"].([]map[string]any)); len(got) != 0 {
				t.Fatalf("required_fields = %#v", template["required_fields"])
			}
		})

		t.Run("fully verified tracked drift continues with revision bump", func(t *testing.T) {
			repoRoot := copyApplicationFixtureRepo(t, "verified-spec")
			initGitRepoAtDate(t, repoRoot, "2026-03-28T12:00:00Z")
			replaceFileText(t, filepath.Join(repoRoot, "runtime", "src", "domain", "session_execution", "SPEC.md"), "## Requirement: Compensation stage 4 failure cleanup\n\n```gherkin requirement\n@runtime @e2e\nFeature: Compensation stage 4 failure cleanup\n```\n\n### Scenarios\n\n```gherkin scenario\nScenario: Cleanup runs after stage 4 failure\n  Given stage 4 fails during compensation\n  When recovery completes\n  Then cleanup steps run in documented order\n```\n", "## Requirement: Compensation stage 4 failure cleanup\n\n```gherkin requirement\n@runtime @e2e\nFeature: Compensation stage 4 failure cleanup\n```\n\n### Scenarios\n\n```gherkin scenario\nScenario: Cleanup runs after stage 4 failure\n  Given stage 4 fails during compensation\n  When recovery completes\n  Then cleanup steps run in documented order\n```\n\n## Requirement: Tracked diff\n\n```gherkin requirement\n@runtime\nFeature: Tracked diff\n```\n\n### Scenarios\n\n```gherkin scenario\nScenario: Continue verification after diff review\n  Given the drift is already tracked by a delta\n  When the diff is inspected\n  Then the requirement chain continues\n```\n\n## Drift Review\n\nTracked diff.\n")
			runGitAtDate(t, repoRoot, "2026-03-30T09:30:00Z", "add", ".")
			runGitAtDate(t, repoRoot, "2026-03-30T09:30:00Z", "commit", "-m", "tracked diff")

			service := &Service{repoRoot: repoRoot, specsDir: filepath.Join(repoRoot, ".specs")}
			if _, _, _, err := service.AddDelta(DeltaAddRequest{
				Target:         "runtime:session-lifecycle",
				Intent:         domain.DeltaIntentAdd,
				Area:           "Tracked diff",
				Current:        "The committed design-doc drift is not yet tracked",
				CurrentPresent: true,
				Targets:        "Track the committed diff before convergence",
				TargetPresent:  true,
				Notes:          "Opened after the diff commit",
				NotesPresent:   true,
			}); err != nil {
				t.Fatalf("AddDelta: %v", err)
			}
			runGitAtDate(t, repoRoot, "2026-03-30T09:35:00Z", "add", ".specs/runtime/session-lifecycle.yaml")
			runGitAtDate(t, repoRoot, "2026-03-30T09:35:00Z", "commit", "-m", "track diff")
			if _, _, _, err := service.AddRequirement(RequirementAddRequest{
				Target:  "runtime:session-lifecycle",
				DeltaID: "D-002",
				Gherkin: "@runtime\nFeature: Tracked diff\n\n  Scenario: Continue verification after diff review\n    Given the drift is already tracked by a delta\n    When the diff is inspected\n    Then the requirement chain continues\n",
			}); err != nil {
				t.Fatalf("AddRequirement: %v", err)
			}
			if _, _, _, err := service.VerifyRequirement(RequirementVerifyRequest{
				Target:        "runtime:session-lifecycle",
				RequirementID: "REQ-002",
				TestFiles:     []string{"runtime/tests/domain/test_compensation_cleanup.py"},
			}); err != nil {
				t.Fatalf("VerifyRequirement: %v", err)
			}
			if _, _, _, err := service.CloseDelta(DeltaTransitionRequest{Target: "runtime:session-lifecycle", DeltaID: "D-002"}); err != nil {
				t.Fatalf("CloseDelta: %v", err)
			}

			_, next, err := service.ReadDiff("runtime:session-lifecycle", "")
			if err != nil {
				t.Fatalf("ReadDiff: %v", err)
			}
			if len(next) != 1 || next[0].(map[string]any)["action"] != "rev_bump" {
				t.Fatalf("next = %#v", next)
			}
			action := next[0].(map[string]any)
			if action["instructions"] != "The tracked drift is fully verified. Converge the checkpoint with a revision bump." {
				t.Fatalf("instructions = %#v", action["instructions"])
			}
			template := action["template"].(map[string]any)
			if got := strings.Join(template["argv"].([]string), " "); got != "specctl rev bump runtime:session-lifecycle --checkpoint HEAD" {
				t.Fatalf("argv = %v", template["argv"])
			}
			if got := requiredFieldNamesFromMaps(template["required_fields"].([]map[string]any)); strings.Join(got, ",") != "summary" {
				t.Fatalf("required_fields = %#v", template["required_fields"])
			}
		})

		t.Run("checkpoint unavailable continues with repair sync", func(t *testing.T) {
			repoRoot := copyApplicationFixtureRepo(t, "verified-spec")
			initGitRepoAtDate(t, repoRoot, "2026-03-28T12:00:00Z")
			replaceTrackingCheckpoint(t, filepath.Join(repoRoot, ".specs", "runtime", "session-lifecycle.yaml"), "deadbee")

			service := &Service{repoRoot: repoRoot, specsDir: filepath.Join(repoRoot, ".specs")}
			_, next, err := service.ReadDiff("runtime:session-lifecycle", "")
			if err != nil {
				t.Fatalf("ReadDiff: %v", err)
			}
			if len(next) != 1 || next[0].(map[string]any)["action"] != "sync" {
				t.Fatalf("next = %#v", next)
			}
			action := next[0].(map[string]any)
			if action["instructions"] != "Repair the missing checkpoint by re-anchoring to a resolvable commit." {
				t.Fatalf("instructions = %#v", action["instructions"])
			}
			template := action["template"].(map[string]any)
			if got := strings.Join(template["argv"].([]string), " "); got != "specctl sync runtime:session-lifecycle --checkpoint HEAD" {
				t.Fatalf("argv = %v", template["argv"])
			}
			if got := requiredFieldNamesFromMaps(template["required_fields"].([]map[string]any)); strings.Join(got, ",") != "summary" {
				t.Fatalf("required_fields = %#v", template["required_fields"])
			}
		})

		t.Run("multi-section design-doc drift groups headings by intent", func(t *testing.T) {
			repoRoot := copyApplicationFixtureRepo(t, "verified-spec")
			initGitRepoAtDate(t, repoRoot, "2026-03-28T12:00:00Z")
			docPath := filepath.Join(repoRoot, "runtime", "src", "domain", "session_execution", "SPEC.md")
			if err := os.WriteFile(docPath, []byte(`---
spec: session-lifecycle
charter: runtime
---
# Session Lifecycle

## Overview
Updated overview.

## Recovery Notes
Expanded recovery notes.
`), 0o644); err != nil {
				t.Fatalf("write design doc: %v", err)
			}
			runGitAtDate(t, repoRoot, "2026-03-30T09:30:00Z", "add", ".")
			runGitAtDate(t, repoRoot, "2026-03-30T09:30:00Z", "commit", "-m", "multi-section drift")

			service := &Service{repoRoot: repoRoot, specsDir: filepath.Join(repoRoot, ".specs")}
			_, next, err := service.ReadDiff("runtime:session-lifecycle", "")
			if err != nil {
				t.Fatalf("ReadDiff: %v", err)
			}
			if len(next) != 5 {
				t.Fatalf("next = %#v", next)
			}
			create := next[0].(map[string]any)
			if create["action"] != "delta_add_add" || create["instructions"] != "The design document changed in 4 sections. Choose the semantic path that matches the observed contract change." {
				t.Fatalf("create_delta = %#v", create)
			}
			reviewAgain := next[1].(map[string]any)
			if reviewAgain["choose_when"] == nil {
				t.Fatalf("review option = %#v", reviewAgain)
			}
			template := reviewAgain["template"].(map[string]any)
			if got := strings.Join(template["argv"].([]string), " "); got != "specctl delta add runtime:session-lifecycle --intent change --area <area>" {
				t.Fatalf("argv = %v", template["argv"])
			}
			if got := requiredFieldNamesFromMaps(template["required_fields"].([]map[string]any)); strings.Join(got, ",") != "area,current,target,notes,affects_requirements" {
				t.Fatalf("required_fields = %#v", template["required_fields"])
			}
		})
	})
}

func initGitRepoAtDate(t *testing.T, repoRoot, timestamp string) {
	t.Helper()

	runGitAtDate(t, repoRoot, timestamp, "init")
	runGitAtDate(t, repoRoot, timestamp, "config", "user.name", "Specctl Tests")
	runGitAtDate(t, repoRoot, timestamp, "config", "user.email", "specctl-tests@example.com")
	runGitAtDate(t, repoRoot, timestamp, "add", ".")
	runGitAtDate(t, repoRoot, timestamp, "commit", "-m", "fixture")
	head := strings.TrimSpace(runGitAtDate(t, repoRoot, timestamp, "rev-parse", "HEAD"))
	rewriteTrackingCheckpoints(t, repoRoot, head)
	runGitAtDate(t, repoRoot, timestamp, "add", ".")
	runGitAtDate(t, repoRoot, timestamp, "commit", "-m", "rewrite checkpoints")
}

func runGitAtDate(t *testing.T, repoRoot, timestamp string, args ...string) string {
	t.Helper()

	cmd := exec.Command("git", args...)
	cmd.Dir = repoRoot
	cmd.Env = append(cmd.Environ(), "GIT_AUTHOR_DATE="+timestamp, "GIT_COMMITTER_DATE="+timestamp)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v failed: %v\n%s", args, err, output)
	}
	return string(output)
}

func requireFindingCode(t *testing.T, findings []any, code string) {
	t.Helper()

	for _, raw := range findings {
		finding, ok := raw.(infrastructure.ValidationFinding)
		if ok && finding.Code == code {
			return
		}
	}
	t.Fatalf("expected finding %q in %#v", code, findings)
}

func requiredFieldNamesFromMaps(fields []map[string]any) []string {
	names := make([]string, 0, len(fields))
	for _, field := range fields {
		name, _ := field["name"].(string)
		names = append(names, name)
	}
	return names
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

func replaceTrackingCheckpoint(t *testing.T, path, checkpoint string) {
	t.Helper()

	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	lines := strings.Split(string(content), "\n")
	replaced := false
	for i, line := range lines {
		if strings.HasPrefix(line, "checkpoint: ") {
			lines[i] = "checkpoint: " + checkpoint
			replaced = true
			break
		}
	}
	if !replaced {
		t.Fatalf("did not find checkpoint in %s", path)
	}
	if err := os.WriteFile(path, []byte(strings.Join(lines, "\n")), 0644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}
