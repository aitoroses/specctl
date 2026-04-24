package cli

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestFocusedWriteBinaryTransport(t *testing.T) {
	t.Parallel()
	binary := buildSpecctlBinary(t)

	t.Run("success writes JSON only to stdout", func(t *testing.T) {
		repoRoot := copyFixtureRepoWithRegistry(t, "verified-spec")

		stdout, stderr, exitCode := runSpecctlBinary(t, binary, repoRoot, "current: Current gap\ntarget: Target gap\nnotes: Explicitly tracked\n", "delta", "add", "runtime:session-lifecycle", "--intent", "add", "--area", "Heartbeat timeout")
		if exitCode != 0 {
			t.Fatalf("exit code = %d, stderr=%q stdout=%q", exitCode, stderr, stdout)
		}
		if stderr != "" {
			t.Fatalf("expected empty stderr, got %q", stderr)
		}

		var envelope struct {
			State struct {
				Status string `json:"status"`
			} `json:"state"`
			Focus struct {
				Delta struct {
					ID string `json:"id"`
				} `json:"delta"`
			} `json:"focus"`
			Result struct {
				Kind  string `json:"kind"`
				Delta struct {
					ID string `json:"id"`
				} `json:"delta"`
			} `json:"result"`
			Next testNextMaps `json:"next"`
		}
		mustUnmarshalJSON(t, stdout, &envelope)
		if envelope.Result.Kind != "delta" {
			t.Fatalf("result.kind = %q", envelope.Result.Kind)
		}
		if envelope.State.Status != "ready" {
			t.Fatalf("state.status = %q", envelope.State.Status)
		}
		if envelope.Focus.Delta.ID != envelope.Result.Delta.ID {
			t.Fatalf("focus/result delta mismatch: %#v %#v", envelope.Focus.Delta, envelope.Result.Delta)
		}
		if len(envelope.Next) == 0 || envelope.Next[0]["action"] != "write_spec_section" {
			t.Fatalf("next = %#v", envelope.Next)
		}
	})

	t.Run("failure writes JSON only to stdout", func(t *testing.T) {
		repoRoot := copyFixtureRepoWithRegistry(t, "active-spec")

		stdout, stderr, exitCode := runSpecctlBinary(t, binary, repoRoot, "", "delta", "close", "runtime:session-lifecycle", "D-001")
		if exitCode == 0 {
			t.Fatalf("expected failure, stdout=%q stderr=%q", stdout, stderr)
		}
		if stderr != "" {
			t.Fatalf("expected empty stderr, got %q", stderr)
		}

		envelope := parseBinaryEnvelope(t, stdout)
		if envelope.Error == nil || envelope.Error.Code != "UNVERIFIED_REQUIREMENTS" {
			t.Fatalf("unexpected envelope %#v", envelope)
		}
		focus := envelope.State["focus"].(map[string]any)
		if focus["delta"].(map[string]any)["id"] != "D-001" {
			t.Fatalf("delta focus = %#v", focus["delta"])
		}
	})

	t.Run("sync failures use sync focus instead of rev_bump", func(t *testing.T) {
		repoRoot := copyFixtureRepoWithRegistry(t, "verified-spec")

		stdout, stderr, exitCode := runSpecctlBinary(t, binary, repoRoot, "", "sync", "runtime:session-lifecycle")
		if exitCode == 0 {
			t.Fatalf("expected failure, stdout=%q stderr=%q", stdout, stderr)
		}
		if stderr != "" {
			t.Fatalf("expected empty stderr, got %q", stderr)
		}

		envelope := parseBinaryEnvelope(t, stdout)
		if envelope.Error == nil || envelope.Error.Code != "INVALID_INPUT" {
			t.Fatalf("unexpected envelope %#v", envelope)
		}
		focus := envelope.State["focus"].(map[string]any)
		if _, exists := focus["rev_bump"]; exists {
			t.Fatalf("focus leaked rev_bump = %#v", focus)
		}
		if focus["sync"].(map[string]any)["reason"] != "missing_checkpoint" {
			t.Fatalf("sync focus = %#v", focus["sync"])
		}
	})
}

func TestShellPipelineFocusedLifecycleJourney(t *testing.T) {
	t.Parallel()
	binary := buildSpecctlBinary(t)
	repoRoot := newBinaryJourneyRepo(t)

	stdout, stderr, exitCode := runSpecctlShellPipeline(t, binary, repoRoot, "title: Runtime System\ndescription: Specs for runtime control-plane and data-plane behavior\ngroups:\n  - key: recovery\n    title: Recovery and Cleanup\n    order: 20\n", "charter", "create", "runtime")
	if exitCode != 0 {
		t.Fatalf("charter create failed: exit=%d stderr=%q stdout=%q", exitCode, stderr, stdout)
	}
	if stderr != "" {
		t.Fatalf("charter create stderr = %q", stderr)
	}
	var charterCreate struct {
		State struct {
			Name string `json:"name"`
		} `json:"state"`
		Result struct {
			Kind string `json:"kind"`
		} `json:"result"`
	}
	mustUnmarshalJSON(t, stdout, &charterCreate)
	if charterCreate.Result.Kind != "charter" || charterCreate.State.Name != "runtime" {
		t.Fatalf("charter create envelope = %#v", charterCreate)
	}

	stdout, stderr, exitCode = runSpecctlShellPipeline(t, binary, repoRoot, "", "spec", "create", "runtime:session-lifecycle", "--title", "Session Lifecycle", "--doc", "runtime/src/domain/session_execution/SPEC.md", "--scope", "runtime/src/domain/session_execution/", "--group", "recovery", "--order", "20", "--charter-notes", "Session FSM and cleanup behavior")
	if exitCode != 0 {
		t.Fatalf("spec create failed: exit=%d stderr=%q stdout=%q", exitCode, stderr, stdout)
	}
	if stderr != "" {
		t.Fatalf("spec create stderr = %q", stderr)
	}
	var specCreate struct {
		State struct {
			Status string `json:"status"`
		} `json:"state"`
		Result struct {
			Kind string `json:"kind"`
		} `json:"result"`
	}
	mustUnmarshalJSON(t, stdout, &specCreate)
	if specCreate.Result.Kind != "spec" || specCreate.State.Status != "draft" {
		t.Fatalf("spec create envelope = %#v", specCreate)
	}

	stdout, stderr, exitCode = runSpecctlShellPipeline(t, binary, repoRoot, "current: Failure cleanup is undocumented\ntarget: Capture the compensation cleanup contract\nnotes: Needed before recovery work lands\n", "delta", "add", "runtime:session-lifecycle", "--intent", "add", "--area", "Compensation stage 4")
	if exitCode != 0 {
		t.Fatalf("delta add failed: exit=%d stderr=%q stdout=%q", exitCode, stderr, stdout)
	}
	if stderr != "" {
		t.Fatalf("delta add stderr = %q", stderr)
	}
	var deltaAdd struct {
		State struct {
			Status string `json:"status"`
		} `json:"state"`
		Result struct {
			Kind  string `json:"kind"`
			Delta struct {
				ID string `json:"id"`
			} `json:"delta"`
		} `json:"result"`
	}
	mustUnmarshalJSON(t, stdout, &deltaAdd)
	if deltaAdd.Result.Kind != "delta" || deltaAdd.Result.Delta.ID != "D-001" || deltaAdd.State.Status != "ready" {
		t.Fatalf("delta add envelope = %#v", deltaAdd)
	}

	stdout, stderr, exitCode = runSpecctlShellPipeline(t, binary, repoRoot, "@runtime\nFeature: Compensation cleanup\n\n  Scenario: Cleanup runs after a failure\n    Given stage 4 fails during compensation\n    When recovery completes\n    Then cleanup steps run in documented order\n", "req", "add", "runtime:session-lifecycle", "--delta", "D-001")
	if exitCode != 0 {
		t.Fatalf("req add failed: exit=%d stderr=%q stdout=%q", exitCode, stderr, stdout)
	}
	if stderr != "" {
		t.Fatalf("req add stderr = %q", stderr)
	}
	var reqAdd struct {
		State struct {
			Status string `json:"status"`
		} `json:"state"`
		Result struct {
			Kind        string `json:"kind"`
			Requirement struct {
				ID string `json:"id"`
			} `json:"requirement"`
		} `json:"result"`
	}
	mustUnmarshalJSON(t, stdout, &reqAdd)
	if reqAdd.Result.Kind != "requirement" || reqAdd.Result.Requirement.ID != "REQ-001" || reqAdd.State.Status != "active" {
		t.Fatalf("req add envelope = %#v", reqAdd)
	}

	testPath := filepath.Join(repoRoot, "runtime", "tests", "domain")
	if err := os.MkdirAll(testPath, 0o755); err != nil {
		t.Fatalf("mkdir test dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(testPath, "test_compensation_cleanup.py"), []byte("def test_cleanup():\n    assert True\n"), 0o644); err != nil {
		t.Fatalf("write test file: %v", err)
	}

	stdout, stderr, exitCode = runSpecctlShellPipeline(t, binary, repoRoot, "", "req", "verify", "runtime:session-lifecycle", "REQ-001", "--test-file", "runtime/tests/domain/test_compensation_cleanup.py")
	if exitCode != 0 {
		t.Fatalf("req verify failed: exit=%d stderr=%q stdout=%q", exitCode, stderr, stdout)
	}
	if stderr != "" {
		t.Fatalf("req verify stderr = %q", stderr)
	}
	var reqVerify struct {
		Result struct {
			Kind        string `json:"kind"`
			Requirement struct {
				Verification string `json:"verification"`
			} `json:"requirement"`
		} `json:"result"`
	}
	mustUnmarshalJSON(t, stdout, &reqVerify)
	if reqVerify.Result.Kind != "requirement" || reqVerify.Result.Requirement.Verification != "verified" {
		t.Fatalf("req verify envelope = %#v", reqVerify)
	}

	stdout, stderr, exitCode = runSpecctlShellPipeline(t, binary, repoRoot, "", "delta", "close", "runtime:session-lifecycle", "D-001")
	if exitCode != 0 {
		t.Fatalf("delta close failed: exit=%d stderr=%q stdout=%q", exitCode, stderr, stdout)
	}
	if stderr != "" {
		t.Fatalf("delta close stderr = %q", stderr)
	}
	var deltaClose struct {
		State struct {
			Status string `json:"status"`
		} `json:"state"`
		Result struct {
			Kind  string `json:"kind"`
			Delta struct {
				Status string `json:"status"`
			} `json:"delta"`
		} `json:"result"`
	}
	mustUnmarshalJSON(t, stdout, &deltaClose)
	if deltaClose.Result.Kind != "delta" || deltaClose.Result.Delta.Status != "closed" || deltaClose.State.Status != "verified" {
		t.Fatalf("delta close envelope = %#v", deltaClose)
	}

	headSHA := initGitRepo(t, repoRoot)

	stdout, stderr, exitCode = runSpecctlShellPipeline(t, binary, repoRoot, "Closed the compensation cleanup work and synced the design doc.\n", "rev", "bump", "runtime:session-lifecycle", "--checkpoint", "HEAD")
	if exitCode != 0 {
		t.Fatalf("rev bump failed: exit=%d stderr=%q stdout=%q", exitCode, stderr, stdout)
	}
	if stderr != "" {
		t.Fatalf("rev bump stderr = %q", stderr)
	}
	var revBump struct {
		State struct {
			Status string `json:"status"`
		} `json:"state"`
		Result struct {
			Kind       string `json:"kind"`
			Checkpoint string `json:"checkpoint"`
		} `json:"result"`
	}
	mustUnmarshalJSON(t, stdout, &revBump)
	if revBump.Result.Kind != "revision" || revBump.Result.Checkpoint != headSHA || revBump.State.Status != "verified" {
		t.Fatalf("rev bump envelope = %#v", revBump)
	}
}

func TestBinaryFullSpecLifecycleJourney(t *testing.T) {
	t.Parallel()
	binary := buildSpecctlBinary(t)
	repoRoot := newBinaryJourneyRepo(t)

	stdout, stderr, exitCode := runSpecctlBinary(t, binary, repoRoot, "", "context", "runtime")
	if exitCode != 0 {
		t.Fatalf("context runtime failed: exit=%d stderr=%q stdout=%q", exitCode, stderr, stdout)
	}
	missingCharter := parseBinaryEnvelope(t, stdout)
	createCharter := requireNextAction(t, missingCharter.Next, 0, "create_charter")
	charterTemplate := requireTemplate(t, createCharter)
	if argv := stringSliceFromAny(t, charterTemplate["argv"]); strings.Join(argv, " ") != "specctl charter create runtime" {
		t.Fatalf("create_charter.template.argv = %v", argv)
	}
	if charterTemplate["stdin_template"] != "title: <title>\ndescription: <description>\ngroups:\n  - key: <group_key>\n    title: <group_title>\n    order: <group_order>\n" {
		t.Fatalf("create_charter.stdin_template = %#v", charterTemplate["stdin_template"])
	}
	if descriptions := requiredFieldDescriptions(t, charterTemplate["required_fields"]); strings.Join(descriptions, "|") != "Human-readable charter title|One-paragraph charter description|Initial group key|Initial group title|Integer group order" {
		t.Fatalf("create_charter.required_field_descriptions = %v", descriptions)
	}

	stdout, stderr, exitCode = runSpecctlBinary(t, binary, repoRoot, "title: Runtime System\ndescription: Specs for runtime control-plane and data-plane behavior\ngroups:\n  - key: recovery\n    title: Recovery and Cleanup\n    order: 20\n", "charter", "create", "runtime")
	if exitCode != 0 {
		t.Fatalf("charter create failed: exit=%d stderr=%q stdout=%q", exitCode, stderr, stdout)
	}
	if stderr != "" {
		t.Fatalf("charter create stderr = %q", stderr)
	}
	var charterCreate struct {
		State struct {
			Name         string `json:"name"`
			TrackingFile string `json:"tracking_file"`
			Validation   struct {
				Valid bool `json:"valid"`
			} `json:"validation"`
		} `json:"state"`
		Result struct {
			Kind         string `json:"kind"`
			TrackingFile string `json:"tracking_file"`
		} `json:"result"`
		Next testNextMaps `json:"next"`
	}
	mustUnmarshalJSON(t, stdout, &charterCreate)
	if charterCreate.State.Name != "runtime" || charterCreate.State.TrackingFile != ".specs/runtime/CHARTER.yaml" || !charterCreate.State.Validation.Valid || charterCreate.Result.Kind != "charter" || charterCreate.Result.TrackingFile != ".specs/runtime/CHARTER.yaml" {
		t.Fatalf("charter create envelope = %#v", charterCreate)
	}
	createFirstSpec := requireNextAction(t, mapsToAny(charterCreate.Next), 0, "create_spec")
	createFirstSpecTemplate := requireTemplate(t, createFirstSpec)
	if argv := stringSliceFromAny(t, createFirstSpecTemplate["argv"]); strings.Join(argv, " ") != "specctl spec create runtime:<slug> --title <title> --doc <design_doc> --scope <scope_dir_1>/ --group recovery --order <order> --charter-notes <charter_notes>" {
		t.Fatalf("charter create next argv = %v", argv)
	}
	if descriptions := requiredFieldDescriptions(t, createFirstSpecTemplate["required_fields"]); strings.Join(descriptions, "|") != "Kebab-case spec identifier inside the charter|Human-readable spec title|Repo-relative markdown path|Governed directory ending in /|Integer order inside the group|Short planning note" {
		t.Fatalf("charter create next required_field_descriptions = %v", descriptions)
	}

	stdout, stderr, exitCode = runSpecctlBinary(t, binary, repoRoot, "", "context", "runtime:session-lifecycle")
	if exitCode != 0 {
		t.Fatalf("context missing spec failed: exit=%d stderr=%q stdout=%q", exitCode, stderr, stdout)
	}
	missingSpec := parseBinaryEnvelope(t, stdout)
	createSpec := requireNextAction(t, missingSpec.Next, 0, "create_spec")
	specTemplate := requireTemplate(t, createSpec)
	if argv := stringSliceFromAny(t, specTemplate["argv"]); strings.Join(argv, " ") != "specctl spec create runtime:session-lifecycle --title <title> --doc <design_doc> --scope <scope_dir_1>/ --group <group> --group-title <group_title> --group-order <group_order> --order <order> --charter-notes <charter_notes>" {
		t.Fatalf("create_spec.template.argv = %v", argv)
	}
	if descriptions := requiredFieldDescriptions(t, specTemplate["required_fields"]); strings.Join(descriptions, "|") != "Human-readable spec title|Repo-relative markdown path|First repo-relative governed directory ending in /|Charter group key|Required only when creating a new group|Integer order for a newly created group|Integer order for the spec inside its group|Short planning note for the charter entry" {
		t.Fatalf("create_spec.required_field_descriptions = %v", descriptions)
	}

	stdout, stderr, exitCode = runSpecctlBinary(t, binary, repoRoot, "", "spec", "create", "runtime:session-lifecycle", "--title", "Session Lifecycle", "--doc", "runtime/src/domain/session_execution/SPEC.md", "--scope", "runtime/src/domain/session_execution/", "--group", "recovery", "--order", "20", "--charter-notes", "Session FSM and cleanup behavior")
	if exitCode != 0 {
		t.Fatalf("spec create failed: exit=%d stderr=%q stdout=%q", exitCode, stderr, stdout)
	}
	if stderr != "" {
		t.Fatalf("spec create stderr = %q", stderr)
	}
	var specCreate struct {
		State struct {
			Slug         string   `json:"slug"`
			Status       string   `json:"status"`
			TrackingFile string   `json:"tracking_file"`
			Scope        []string `json:"scope"`
			Documents    struct {
				Primary string `json:"primary"`
			} `json:"documents"`
			CharterMembership struct {
				Group string `json:"group"`
				Order int    `json:"order"`
				Notes string `json:"notes"`
			} `json:"charter_membership"`
			ScopeDrift struct {
				Status string `json:"status"`
			} `json:"scope_drift"`
			Validation struct {
				Valid    bool `json:"valid"`
				Findings []struct {
					Code string `json:"code"`
				} `json:"findings"`
			} `json:"validation"`
		} `json:"state"`
		Result struct {
			Kind            string `json:"kind"`
			TrackingFile    string `json:"tracking_file"`
			DesignDoc       string `json:"design_doc"`
			DesignDocAction string `json:"design_doc_action"`
		} `json:"result"`
		Next testNextMaps `json:"next"`
	}
	mustUnmarshalJSON(t, stdout, &specCreate)
	if specCreate.State.Slug != "session-lifecycle" || specCreate.State.Status != "draft" || specCreate.State.TrackingFile != ".specs/runtime/session-lifecycle.yaml" || specCreate.State.Documents.Primary != "runtime/src/domain/session_execution/SPEC.md" || strings.Join(specCreate.State.Scope, ",") != "runtime/src/domain/session_execution/" || specCreate.State.CharterMembership.Group != "recovery" || specCreate.State.CharterMembership.Order != 20 || specCreate.State.CharterMembership.Notes != "Session FSM and cleanup behavior" || specCreate.State.ScopeDrift.Status != "clean" || !specCreate.State.Validation.Valid || len(specCreate.State.Validation.Findings) != 0 || specCreate.Result.Kind != "spec" || specCreate.Result.TrackingFile != ".specs/runtime/session-lifecycle.yaml" || specCreate.Result.DesignDoc != "runtime/src/domain/session_execution/SPEC.md" || specCreate.Result.DesignDocAction != "validated_existing" {
		t.Fatalf("spec create envelope = %#v", specCreate)
	}
	// spec create returns create_format_template when no format is configured
	if len(specCreate.Next) != 1 || specCreate.Next[0]["action"] != "create_format_template" {
		t.Fatalf("spec create next = %#v", specCreate.Next)
	}

	stdout, stderr, exitCode = runSpecctlBinary(t, binary, repoRoot, "", "diff", "runtime:session-lifecycle")
	if exitCode != 0 {
		t.Fatalf("initial diff failed: exit=%d stderr=%q stdout=%q", exitCode, stderr, stdout)
	}
	var initialDiff struct {
		State struct {
			Baseline string `json:"baseline"`
			From     any    `json:"from"`
		} `json:"state"`
	}
	mustUnmarshalJSON(t, stdout, &initialDiff)
	if initialDiff.State.Baseline != "checkpoint" || initialDiff.State.From != nil {
		t.Fatalf("initial diff state = %#v", initialDiff.State)
	}

	stdout, stderr, exitCode = runSpecctlBinary(t, binary, repoRoot, "current: Failure cleanup is undocumented\ntarget: Capture the compensation cleanup contract\nnotes: Needed before recovery work lands\n", "delta", "add", "runtime:session-lifecycle", "--intent", "add", "--area", "Compensation stage 4")
	if exitCode != 0 {
		t.Fatalf("delta add failed: exit=%d stderr=%q stdout=%q", exitCode, stderr, stdout)
	}
	if stderr != "" {
		t.Fatalf("delta add stderr = %q", stderr)
	}
	var deltaAdd struct {
		State struct {
			Status string `json:"status"`
		} `json:"state"`
		Focus struct {
			Delta struct {
				ID     string `json:"id"`
				Status string `json:"status"`
			} `json:"delta"`
		} `json:"focus"`
		Result struct {
			Kind  string `json:"kind"`
			Delta struct {
				ID     string `json:"id"`
				Status string `json:"status"`
			} `json:"delta"`
		} `json:"result"`
		Next testNextMaps `json:"next"`
	}
	mustUnmarshalJSON(t, stdout, &deltaAdd)
	if deltaAdd.State.Status != "ready" || deltaAdd.Focus.Delta.ID != "D-001" || deltaAdd.Focus.Delta.Status != "open" || deltaAdd.Result.Kind != "delta" || deltaAdd.Result.Delta.ID != "D-001" || deltaAdd.Result.Delta.Status != "open" {
		t.Fatalf("delta add envelope = %#v", deltaAdd)
	}
	if len(deltaAdd.Next) == 0 || deltaAdd.Next[0]["action"] != "write_spec_section" {
		t.Fatalf("delta add next = %#v", deltaAdd.Next)
	}
	if argv := stringSliceFromAny(t, requireTemplate(t, deltaAdd.Next[1])["argv"]); strings.Join(argv, " ") != "specctl req add runtime:session-lifecycle --delta D-001" {
		t.Fatalf("delta add template.argv = %v", argv)
	}

	stdout, stderr, exitCode = runSpecctlBinary(t, binary, repoRoot, "@runtime\nFeature: Compensation cleanup\n\n  Scenario: Cleanup runs after a failure\n    Given stage 4 fails during compensation\n    When recovery completes\n    Then cleanup steps run in documented order\n", "req", "add", "runtime:session-lifecycle", "--delta", "D-001")
	if exitCode != 0 {
		t.Fatalf("req add failed: exit=%d stderr=%q stdout=%q", exitCode, stderr, stdout)
	}
	if stderr != "" {
		t.Fatalf("req add stderr = %q", stderr)
	}
	var reqAdd struct {
		State struct {
			Status string `json:"status"`
		} `json:"state"`
		Focus struct {
			Requirement struct {
				ID           string `json:"id"`
				Verification string `json:"verification"`
			} `json:"requirement"`
		} `json:"focus"`
		Result struct {
			Kind        string `json:"kind"`
			Requirement struct {
				ID string `json:"id"`
			} `json:"requirement"`
		} `json:"result"`
		Next testNextMaps `json:"next"`
	}
	mustUnmarshalJSON(t, stdout, &reqAdd)
	if reqAdd.State.Status != "active" || reqAdd.Focus.Requirement.ID != "REQ-001" || reqAdd.Focus.Requirement.Verification != "unverified" || reqAdd.Result.Kind != "requirement" || reqAdd.Result.Requirement.ID != "REQ-001" {
		t.Fatalf("req add envelope = %#v", reqAdd)
	}
	if len(reqAdd.Next) != 2 || reqAdd.Next[0]["action"] != "implement_and_test" || reqAdd.Next[1]["action"] != "verify_requirement" {
		t.Fatalf("req add next = %#v", reqAdd.Next)
	}
	if argv := stringSliceFromAny(t, requireTemplate(t, reqAdd.Next[1])["argv"]); strings.Join(argv, " ") != "specctl req verify runtime:session-lifecycle REQ-001 --test-file runtime/tests/domain/test_compensation_cleanup.py" {
		t.Fatalf("req add next argv = %v", argv)
	}

	stdout, stderr, exitCode = runSpecctlBinary(t, binary, repoRoot, "", "delta", "close", "runtime:session-lifecycle", "D-001")
	if exitCode == 0 {
		t.Fatalf("delta close should have failed before verification: stdout=%q stderr=%q", stdout, stderr)
	}
	if stderr != "" {
		t.Fatalf("delta close failure stderr = %q", stderr)
	}
	failure := parseBinaryEnvelope(t, stdout)
	if failure.Error == nil || failure.Error.Code != "UNVERIFIED_REQUIREMENTS" {
		t.Fatalf("unexpected close failure %#v", failure)
	}
	if failure.State["status"] != "active" {
		t.Fatalf("close failure state = %#v", failure.State)
	}
	focus := failure.State["focus"].(map[string]any)
	if focus["delta"].(map[string]any)["id"] != "D-001" || focus["delta"].(map[string]any)["status"] != "open" {
		t.Fatalf("close failure focus = %#v", focus)
	}
	verifyNext := requireNextAction(t, failure.Next, 0, "verify_requirement")
	if argv := stringSliceFromAny(t, requireTemplate(t, verifyNext)["argv"]); strings.Join(argv, " ") != "specctl req verify runtime:session-lifecycle REQ-001 --test-file runtime/tests/domain/test_compensation_cleanup.py" {
		t.Fatalf("close failure next argv = %v", argv)
	}

	testPath := filepath.Join(repoRoot, "runtime", "tests", "domain")
	if err := os.MkdirAll(testPath, 0o755); err != nil {
		t.Fatalf("mkdir test dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(testPath, "test_compensation_cleanup.py"), []byte("def test_cleanup():\n    assert True\n"), 0o644); err != nil {
		t.Fatalf("write test file: %v", err)
	}

	stdout, stderr, exitCode = runSpecctlBinary(t, binary, repoRoot, "", "req", "verify", "runtime:session-lifecycle", "REQ-001", "--test-file", "runtime/tests/domain/test_compensation_cleanup.py")
	if exitCode != 0 {
		t.Fatalf("req verify failed: exit=%d stderr=%q stdout=%q", exitCode, stderr, stdout)
	}
	if stderr != "" {
		t.Fatalf("req verify stderr = %q", stderr)
	}
	var reqVerify struct {
		State struct {
			Status string `json:"status"`
		} `json:"state"`
		Focus struct {
			Requirement struct {
				ID           string `json:"id"`
				Verification string `json:"verification"`
			} `json:"requirement"`
		} `json:"focus"`
		Result struct {
			Kind        string `json:"kind"`
			Requirement struct {
				ID           string `json:"id"`
				Verification string `json:"verification"`
			} `json:"requirement"`
		} `json:"result"`
		Next testNextMaps `json:"next"`
	}
	mustUnmarshalJSON(t, stdout, &reqVerify)
	if reqVerify.State.Status != "active" || reqVerify.Focus.Requirement.ID != "REQ-001" || reqVerify.Focus.Requirement.Verification != "verified" || reqVerify.Result.Kind != "requirement" || reqVerify.Result.Requirement.ID != "REQ-001" || reqVerify.Result.Requirement.Verification != "verified" {
		t.Fatalf("req verify envelope = %#v", reqVerify)
	}
	if len(reqVerify.Next) == 0 || reqVerify.Next[0]["action"] != "close_delta" {
		t.Fatalf("req verify next = %#v", reqVerify.Next)
	}
	if argv := stringSliceFromAny(t, requireTemplate(t, reqVerify.Next[0])["argv"]); strings.Join(argv, " ") != "specctl delta close runtime:session-lifecycle D-001" {
		t.Fatalf("req verify next argv = %v", argv)
	}

	stdout, stderr, exitCode = runSpecctlBinary(t, binary, repoRoot, "", "delta", "close", "runtime:session-lifecycle", "D-001")
	if exitCode != 0 {
		t.Fatalf("delta close failed: exit=%d stderr=%q stdout=%q", exitCode, stderr, stdout)
	}
	if stderr != "" {
		t.Fatalf("delta close stderr = %q", stderr)
	}
	var closeEnvelope struct {
		State struct {
			Status string `json:"status"`
		} `json:"state"`
		Focus struct {
			Delta struct {
				ID     string `json:"id"`
				Status string `json:"status"`
			} `json:"delta"`
		} `json:"focus"`
		Result struct {
			Kind  string `json:"kind"`
			Delta struct {
				ID     string `json:"id"`
				Status string `json:"status"`
			} `json:"delta"`
		} `json:"result"`
		Next testNextMaps `json:"next"`
	}
	mustUnmarshalJSON(t, stdout, &closeEnvelope)
	if closeEnvelope.Result.Kind != "delta" || closeEnvelope.Result.Delta.ID != "D-001" || closeEnvelope.Result.Delta.Status != "closed" || closeEnvelope.State.Status != "verified" || closeEnvelope.Focus.Delta.ID != "D-001" || closeEnvelope.Focus.Delta.Status != "closed" {
		t.Fatalf("unexpected close envelope %#v", closeEnvelope)
	}
	if len(closeEnvelope.Next) == 0 || closeEnvelope.Next[0]["action"] != "rev_bump" {
		t.Fatalf("delta close next = %#v", closeEnvelope.Next)
	}
	if argv := stringSliceFromAny(t, requireTemplate(t, closeEnvelope.Next[0])["argv"]); strings.Join(argv, " ") != "specctl rev bump runtime:session-lifecycle --checkpoint HEAD" {
		t.Fatalf("delta close template.argv = %v", argv)
	}

	headSHA := initGitRepo(t, repoRoot)

	stdout, stderr, exitCode = runSpecctlBinary(t, binary, repoRoot, "Closed the compensation cleanup work and synced the design doc.\n", "rev", "bump", "runtime:session-lifecycle", "--checkpoint", "HEAD")
	if exitCode != 0 {
		t.Fatalf("rev bump failed: exit=%d stderr=%q stdout=%q", exitCode, stderr, stdout)
	}
	if stderr != "" {
		t.Fatalf("rev bump stderr = %q", stderr)
	}
	var bumpEnvelope struct {
		State struct {
			Status     string `json:"status"`
			ScopeDrift struct {
				Status string `json:"status"`
			} `json:"scope_drift"`
		} `json:"state"`
		Focus struct {
			ChangelogEntry struct {
				Rev int `json:"rev"`
			} `json:"changelog_entry"`
		} `json:"focus"`
		Result struct {
			Kind       string `json:"kind"`
			Rev        int    `json:"rev"`
			Checkpoint string `json:"checkpoint"`
		} `json:"result"`
		Next testNext `json:"next"`
	}
	mustUnmarshalJSON(t, stdout, &bumpEnvelope)
	if bumpEnvelope.Result.Kind != "revision" || bumpEnvelope.State.Status != "verified" || bumpEnvelope.State.ScopeDrift.Status != "clean" || len(bumpEnvelope.Next) != 0 {
		t.Fatalf("rev bump envelope = %#v", bumpEnvelope)
	}
	if bumpEnvelope.Result.Checkpoint != headSHA {
		t.Fatalf("checkpoint = %q, want %q", bumpEnvelope.Result.Checkpoint, headSHA)
	}
	if bumpEnvelope.Focus.ChangelogEntry.Rev != bumpEnvelope.Result.Rev {
		t.Fatalf("focus.changelog_entry.rev = %d, result.rev = %d", bumpEnvelope.Focus.ChangelogEntry.Rev, bumpEnvelope.Result.Rev)
	}

	stdout, stderr, exitCode = runSpecctlBinary(t, binary, repoRoot, "", "diff", "runtime:session-lifecycle")
	if exitCode != 0 {
		t.Fatalf("post-bump diff failed: exit=%d stderr=%q stdout=%q", exitCode, stderr, stdout)
	}
	var postBumpDiff struct {
		State struct {
			Baseline string `json:"baseline"`
			Model    struct {
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
	mustUnmarshalJSON(t, stdout, &postBumpDiff)
	if postBumpDiff.State.Baseline != "checkpoint" || len(postBumpDiff.State.Model.Deltas.Opened) != 0 || len(postBumpDiff.State.Model.Deltas.Closed) != 0 || len(postBumpDiff.State.Model.Requirements.Added) != 0 || len(postBumpDiff.State.Model.Requirements.Verified) != 0 || postBumpDiff.State.DesignDoc.Changed {
		t.Fatalf("post-bump diff state = %#v", postBumpDiff.State)
	}

	stdout, stderr, exitCode = runSpecctlBinary(t, binary, repoRoot, "", "context", "runtime:session-lifecycle")
	if exitCode != 0 {
		t.Fatalf("final context failed: exit=%d stderr=%q stdout=%q", exitCode, stderr, stdout)
	}
	if stderr != "" {
		t.Fatalf("final context stderr = %q", stderr)
	}
	var finalContext struct {
		State struct {
			Status     string `json:"status"`
			Checkpoint string `json:"checkpoint"`
			ScopeDrift struct {
				Status                      string   `json:"status"`
				FilesChangedSinceCheckpoint []string `json:"files_changed_since_checkpoint"`
			} `json:"scope_drift"`
			Validation struct {
				Valid bool `json:"valid"`
			} `json:"validation"`
		} `json:"state"`
		Next testNext `json:"next"`
	}
	mustUnmarshalJSON(t, stdout, &finalContext)
	if finalContext.State.Status != "verified" || finalContext.State.Checkpoint != headSHA || finalContext.State.ScopeDrift.Status != "clean" || len(finalContext.State.ScopeDrift.FilesChangedSinceCheckpoint) != 0 || !finalContext.State.Validation.Valid || len(finalContext.Next) != 0 {
		t.Fatalf("final context = %#v", finalContext)
	}

	stdout, stderr, exitCode = runSpecctlBinary(t, binary, repoRoot, "No-op summary\n", "rev", "bump", "runtime:session-lifecycle", "--checkpoint", "HEAD")
	if exitCode == 0 {
		t.Fatalf("expected no-semantic-changes failure, stdout=%q stderr=%q", stdout, stderr)
	}
	if stderr != "" {
		t.Fatalf("repeat rev bump stderr = %q", stderr)
	}
	repeatFailure := parseBinaryEnvelope(t, stdout)
	if repeatFailure.Error == nil || repeatFailure.Error.Code != "INVALID_INPUT" {
		t.Fatalf("unexpected repeat rev bump envelope %#v", repeatFailure)
	}
	revBump := repeatFailure.State["focus"].(map[string]any)["rev_bump"].(map[string]any)
	if revBump["reason"] != "no_semantic_changes" {
		t.Fatalf("rev_bump focus = %#v", revBump)
	}
}

func TestBinaryDriftReviewJourney(t *testing.T) {
	t.Parallel()
	binary := buildSpecctlBinary(t)
	repoRoot := copyFixtureRepoWithRegistry(t, "verified-spec")
	headSHA := initGitRepoAtDate(t, repoRoot, "2026-03-28T12:00:00Z")
	replaceInFile(t, filepath.Join(repoRoot, ".specs", "runtime", "session-lifecycle.yaml"), "checkpoint: a1b2c3f", "checkpoint: "+headSHA)

	stdout, stderr, exitCode := runSpecctlBinary(t, binary, repoRoot, "", "context", "runtime:session-lifecycle")
	if exitCode != 0 {
		t.Fatalf("context failed: exit=%d stderr=%q stdout=%q", exitCode, stderr, stdout)
	}
	var initialContext struct {
		State struct {
			Status     string `json:"status"`
			Checkpoint string `json:"checkpoint"`
			ScopeDrift struct {
				Status                      string   `json:"status"`
				FilesChangedSinceCheckpoint []string `json:"files_changed_since_checkpoint"`
			} `json:"scope_drift"`
			Validation struct {
				Valid bool `json:"valid"`
			} `json:"validation"`
		} `json:"state"`
		Next testNext `json:"next"`
	}
	mustUnmarshalJSON(t, stdout, &initialContext)
	if initialContext.State.Status != "verified" || initialContext.State.Checkpoint != headSHA || initialContext.State.ScopeDrift.Status != "clean" || !initialContext.State.Validation.Valid {
		t.Fatalf("initial context = %#v", initialContext)
	}
	if len(initialContext.State.ScopeDrift.FilesChangedSinceCheckpoint) != 0 || len(initialContext.Next) != 0 {
		t.Fatalf("initial context drift/next = %#v %#v", initialContext.State.ScopeDrift, initialContext.Next)
	}

	docPath := filepath.Join(repoRoot, "runtime", "src", "domain", "session_execution", "SPEC.md")
	if err := os.WriteFile(docPath, []byte("---\nspec: session-lifecycle\ncharter: runtime\n---\n# Session Lifecycle\n\n## Requirement: Compensation stage 4 failure cleanup\n\n```gherkin requirement\n@runtime @e2e\nFeature: Compensation stage 4 failure cleanup\n```\n\n### Scenarios\n\n```gherkin scenario\nScenario: Cleanup runs after stage 4 failure\n  Given stage 4 fails during compensation\n  When recovery completes\n  Then cleanup steps run in documented order\n```\n\n## Requirement: Drift review\n\n```gherkin requirement\n@runtime\nFeature: Drift review\n```\n\n### Scenarios\n\n```gherkin scenario\nScenario: Review records the new design doc behavior\n  Given the design doc changed after the checkpoint\n  When the drift review is recorded\n  Then the new behavior is tracked by a requirement\n```\n\n## Drift Review\n\nUpdated drift notes.\n"), 0o644); err != nil {
		t.Fatalf("write design doc: %v", err)
	}
	runGitAtDate(t, repoRoot, "2026-03-30T09:30:00Z", "add", ".")
	runGitAtDate(t, repoRoot, "2026-03-30T09:30:00Z", "commit", "-m", "document drift")

	stdout, stderr, exitCode = runSpecctlBinary(t, binary, repoRoot, "", "context", "runtime:session-lifecycle")
	if exitCode != 0 {
		t.Fatalf("drifted context failed: exit=%d stderr=%q stdout=%q", exitCode, stderr, stdout)
	}
	var driftedContext struct {
		State struct {
			Status     string `json:"status"`
			ScopeDrift struct {
				Status                      string   `json:"status"`
				FilesChangedSinceCheckpoint []string `json:"files_changed_since_checkpoint"`
			} `json:"scope_drift"`
			Validation struct {
				Valid bool `json:"valid"`
			} `json:"validation"`
		} `json:"state"`
		Next testNext `json:"next"`
	}
	mustUnmarshalJSON(t, stdout, &driftedContext)
	if driftedContext.State.Status != "verified" || driftedContext.State.ScopeDrift.Status != "drifted" || !driftedContext.State.Validation.Valid {
		t.Fatalf("drifted context = %#v", driftedContext)
	}
	if strings.Join(driftedContext.State.ScopeDrift.FilesChangedSinceCheckpoint, ",") != "runtime/src/domain/session_execution/SPEC.md" || len(driftedContext.Next) == 0 {
		t.Fatalf("drifted context files/next = %#v %#v", driftedContext.State.ScopeDrift.FilesChangedSinceCheckpoint, driftedContext.Next)
	}

	stdout, stderr, exitCode = runSpecctlBinary(t, binary, repoRoot, "", "diff", "runtime:session-lifecycle")
	if exitCode != 0 {
		t.Fatalf("diff failed: exit=%d stderr=%q stdout=%q", exitCode, stderr, stdout)
	}
	var diffEnvelope struct {
		State struct {
			DesignDoc struct {
				Changed bool `json:"changed"`
			} `json:"design_doc"`
		} `json:"state"`
	}
	mustUnmarshalJSON(t, stdout, &diffEnvelope)
	if !diffEnvelope.State.DesignDoc.Changed {
		t.Fatalf("design_doc.changed = false")
	}

	stdout, stderr, exitCode = runSpecctlBinary(t, binary, repoRoot, "current: Drift found during review\ntarget: Bring the spec back to current behavior\nnotes: Review uncovered new work\n", "delta", "add", "runtime:session-lifecycle", "--intent", "add", "--area", "Drift review")
	if exitCode != 0 {
		t.Fatalf("delta add failed: exit=%d stderr=%q stdout=%q", exitCode, stderr, stdout)
	}
	var addEnvelope struct {
		State struct {
			Status string `json:"status"`
		} `json:"state"`
		Focus struct {
			Delta struct {
				ID     string `json:"id"`
				Status string `json:"status"`
			} `json:"delta"`
		} `json:"focus"`
		Result struct {
			Kind  string `json:"kind"`
			Delta struct {
				ID string `json:"id"`
			} `json:"delta"`
		} `json:"result"`
		Next testNextMaps `json:"next"`
	}
	mustUnmarshalJSON(t, stdout, &addEnvelope)
	if addEnvelope.State.Status != "ready" || addEnvelope.Focus.Delta.ID != "D-002" || addEnvelope.Focus.Delta.Status != "open" || addEnvelope.Result.Kind != "delta" || addEnvelope.Result.Delta.ID != "D-002" {
		t.Fatalf("delta add envelope = %#v", addEnvelope)
	}
	if len(addEnvelope.Next) == 0 || addEnvelope.Next[0]["action"] != "write_spec_section" {
		t.Fatalf("delta add next = %#v", addEnvelope.Next)
	}

	testDir := filepath.Join(repoRoot, "runtime", "tests", "domain")
	if err := os.MkdirAll(testDir, 0o755); err != nil {
		t.Fatalf("mkdir test dir: %v", err)
	}
	testFile := filepath.Join(testDir, "test_drift_review.py")
	if err := os.WriteFile(testFile, []byte("def test_drift_review():\n    assert True\n"), 0o644); err != nil {
		t.Fatalf("write drift test file: %v", err)
	}

	stdout, stderr, exitCode = runSpecctlBinary(t, binary, repoRoot, "@runtime\nFeature: Drift review\n\n  Scenario: Review records the new design doc behavior\n    Given the design doc changed after the checkpoint\n    When the drift review is recorded\n    Then the new behavior is tracked by a requirement\n", "req", "add", "runtime:session-lifecycle", "--delta", "D-002")
	if exitCode != 0 {
		t.Fatalf("req add failed: exit=%d stderr=%q stdout=%q", exitCode, stderr, stdout)
	}
	envelope := parseBinaryEnvelope(t, stdout)
	if envelope.State["status"] != "active" {
		t.Fatalf("req add state = %#v", envelope.State)
	}
	reqFocus := envelope.State["focus"].(map[string]any)["requirement"].(map[string]any)
	if reqFocus["id"] != "REQ-002" || reqFocus["verification"] != "unverified" {
		t.Fatalf("req add focus = %#v", reqFocus)
	}
	verifyNext := requireNextAction(t, envelope.Next, 1, "verify_requirement")
	if argv := stringSliceFromAny(t, requireTemplate(t, verifyNext)["argv"]); strings.Join(argv, " ") != "specctl req verify runtime:session-lifecycle REQ-002 --test-file runtime/tests/domain/test_drift_review.py" {
		t.Fatalf("verify next = %#v", verifyNext)
	}

	stdout, stderr, exitCode = runSpecctlBinary(t, binary, repoRoot, "", "req", "verify", "runtime:session-lifecycle", "REQ-002", "--test-file", "runtime/tests/domain/test_drift_review.py")
	if exitCode != 0 {
		t.Fatalf("req verify failed: exit=%d stderr=%q stdout=%q", exitCode, stderr, stdout)
	}
	var verifyEnvelope struct {
		State struct {
			Status string `json:"status"`
		} `json:"state"`
		Focus struct {
			Requirement struct {
				ID           string `json:"id"`
				Verification string `json:"verification"`
			} `json:"requirement"`
		} `json:"focus"`
		Result struct {
			Kind        string `json:"kind"`
			Requirement struct {
				ID           string `json:"id"`
				Verification string `json:"verification"`
			} `json:"requirement"`
		} `json:"result"`
		Next testNext `json:"next"`
	}
	mustUnmarshalJSON(t, stdout, &verifyEnvelope)
	if verifyEnvelope.State.Status != "active" || verifyEnvelope.Focus.Requirement.ID != "REQ-002" || verifyEnvelope.Focus.Requirement.Verification != "verified" || verifyEnvelope.Result.Kind != "requirement" || verifyEnvelope.Result.Requirement.ID != "REQ-002" || verifyEnvelope.Result.Requirement.Verification != "verified" {
		t.Fatalf("req verify envelope = %#v", verifyEnvelope)
	}
	closeDelta := requireNextAction(t, verifyEnvelope.Next, 0, "close_delta")
	if argv := stringSliceFromAny(t, requireTemplate(t, closeDelta)["argv"]); strings.Join(argv, " ") != "specctl delta close runtime:session-lifecycle D-002" {
		t.Fatalf("close_delta argv = %#v", argv)
	}

	stdout, stderr, exitCode = runSpecctlBinary(t, binary, repoRoot, "", "delta", "close", "runtime:session-lifecycle", "D-002")
	if exitCode != 0 {
		t.Fatalf("delta close failed: exit=%d stderr=%q stdout=%q", exitCode, stderr, stdout)
	}
	var closeReviewEnvelope struct {
		State struct {
			Status string `json:"status"`
		} `json:"state"`
		Focus struct {
			Delta struct {
				ID     string `json:"id"`
				Status string `json:"status"`
			} `json:"delta"`
		} `json:"focus"`
		Result struct {
			Kind  string `json:"kind"`
			Delta struct {
				ID     string `json:"id"`
				Status string `json:"status"`
			} `json:"delta"`
		} `json:"result"`
		Next testNext `json:"next"`
	}
	mustUnmarshalJSON(t, stdout, &closeReviewEnvelope)
	if closeReviewEnvelope.State.Status != "verified" || closeReviewEnvelope.Focus.Delta.ID != "D-002" || closeReviewEnvelope.Focus.Delta.Status != "closed" || closeReviewEnvelope.Result.Kind != "delta" || closeReviewEnvelope.Result.Delta.ID != "D-002" || closeReviewEnvelope.Result.Delta.Status != "closed" {
		t.Fatalf("delta close envelope = %#v", closeReviewEnvelope)
	}
	revBumpNext := requireNextAction(t, closeReviewEnvelope.Next, 0, "rev_bump")
	if argv := stringSliceFromAny(t, requireTemplate(t, revBumpNext)["argv"]); strings.Join(argv, " ") != "specctl rev bump runtime:session-lifecycle --checkpoint HEAD" {
		t.Fatalf("rev_bump template argv = %v", argv)
	}

	runGitAtDate(t, repoRoot, "2026-03-30T10:00:00Z", "add", ".")
	runGitAtDate(t, repoRoot, "2026-03-30T10:00:00Z", "commit", "-m", "drift review")
	headSHA = strings.TrimSpace(runGit(t, repoRoot, "rev-parse", "HEAD"))
	stdout, stderr, exitCode = runSpecctlBinary(t, binary, repoRoot, "Captured drift review follow-up.\n", "rev", "bump", "runtime:session-lifecycle", "--checkpoint", "HEAD")
	if exitCode != 0 {
		t.Fatalf("rev bump failed: exit=%d stderr=%q stdout=%q", exitCode, stderr, stdout)
	}
	var bumpEnvelope struct {
		State struct {
			Status     string `json:"status"`
			ScopeDrift struct {
				Status string `json:"status"`
			} `json:"scope_drift"`
		} `json:"state"`
		Focus struct {
			ChangelogEntry struct {
				Rev int `json:"rev"`
			} `json:"changelog_entry"`
		} `json:"focus"`
		Result struct {
			Rev        int    `json:"rev"`
			Kind       string `json:"kind"`
			Checkpoint string `json:"checkpoint"`
		} `json:"result"`
		Next testNext `json:"next"`
	}
	mustUnmarshalJSON(t, stdout, &bumpEnvelope)
	if bumpEnvelope.Result.Kind != "revision" || bumpEnvelope.Result.Checkpoint != headSHA || bumpEnvelope.State.Status != "verified" || bumpEnvelope.State.ScopeDrift.Status != "clean" || bumpEnvelope.Focus.ChangelogEntry.Rev != bumpEnvelope.Result.Rev || len(bumpEnvelope.Next) != 0 {
		t.Fatalf("rev bump envelope = %#v", bumpEnvelope)
	}

	stdout, stderr, exitCode = runSpecctlBinary(t, binary, repoRoot, "", "diff", "runtime:session-lifecycle")
	if exitCode != 0 {
		t.Fatalf("diff after bump failed: exit=%d stderr=%q stdout=%q", exitCode, stderr, stdout)
	}
	var postBump struct {
		State struct {
			DesignDoc struct {
				Changed bool `json:"changed"`
			} `json:"design_doc"`
		} `json:"state"`
	}
	mustUnmarshalJSON(t, stdout, &postBump)
	if postBump.State.DesignDoc.Changed {
		t.Fatalf("diff after bump = %#v", postBump.State)
	}

	stdout, stderr, exitCode = runSpecctlBinary(t, binary, repoRoot, "", "context", "runtime:session-lifecycle")
	if exitCode != 0 {
		t.Fatalf("final context failed: exit=%d stderr=%q stdout=%q", exitCode, stderr, stdout)
	}
	var finalContext struct {
		State struct {
			Status     string `json:"status"`
			Checkpoint string `json:"checkpoint"`
			ScopeDrift struct {
				Status                      string   `json:"status"`
				FilesChangedSinceCheckpoint []string `json:"files_changed_since_checkpoint"`
			} `json:"scope_drift"`
		} `json:"state"`
		Next testNext `json:"next"`
	}
	mustUnmarshalJSON(t, stdout, &finalContext)
	if finalContext.State.Status != "verified" || finalContext.State.Checkpoint != headSHA || finalContext.State.ScopeDrift.Status != "clean" || len(finalContext.State.ScopeDrift.FilesChangedSinceCheckpoint) != 0 || len(finalContext.Next) != 0 {
		t.Fatalf("final context = %#v", finalContext)
	}
}

func TestBinaryAdditionalDriftJourneys(t *testing.T) {
	t.Parallel()
	binary := buildSpecctlBinary(t)

	t.Run("code-first drift converges with sync", func(t *testing.T) {
		repoRoot := copyFixtureRepoWithRegistry(t, "verified-spec")
		headSHA := initGitRepoAtDate(t, repoRoot, "2026-03-28T12:00:00Z")
		replaceInFile(t, filepath.Join(repoRoot, ".specs", "runtime", "session-lifecycle.yaml"), "checkpoint: a1b2c3f", "checkpoint: "+headSHA)
		codePath := filepath.Join(repoRoot, "runtime", "src", "domain", "session_execution", "handler.py")
		if err := os.WriteFile(codePath, []byte("def handle():\n    return 'code-first drift'\n"), 0o644); err != nil {
			t.Fatalf("write code file: %v", err)
		}
		runGitAtDate(t, repoRoot, "2026-03-30T09:30:00Z", "add", ".")
		runGitAtDate(t, repoRoot, "2026-03-30T09:30:00Z", "commit", "-m", "code drift")
		newHead := strings.TrimSpace(runGit(t, repoRoot, "rev-parse", "HEAD"))

		stdout, stderr, exitCode := runSpecctlBinary(t, binary, repoRoot, "", "context", "runtime:session-lifecycle")
		if exitCode != 0 {
			t.Fatalf("context failed: exit=%d stderr=%q stdout=%q", exitCode, stderr, stdout)
		}
		var driftedContext struct {
			Focus struct {
				ScopeDrift struct {
					DriftSource string `json:"drift_source"`
				} `json:"scope_drift"`
			} `json:"focus"`
			State struct {
				ScopeDrift struct {
					Status      string `json:"status"`
					DriftSource string `json:"drift_source"`
				} `json:"scope_drift"`
			} `json:"state"`
			Next struct {
				Mode    string `json:"mode"`
				Steps   []any  `json:"steps"`
				Options []any  `json:"options"`
			} `json:"next"`
		}
		mustUnmarshalJSON(t, stdout, &driftedContext)
		if driftedContext.State.ScopeDrift.Status != "drifted" || driftedContext.State.ScopeDrift.DriftSource != "scope_code" {
			t.Fatalf("context = %#v", driftedContext)
		}
		if driftedContext.Focus.ScopeDrift.DriftSource != "scope_code" {
			t.Fatalf("focus = %#v", driftedContext.Focus)
		}
		if driftedContext.Next.Mode != "choose_one" || len(driftedContext.Next.Options) != 2 {
			t.Fatalf("next = %#v", driftedContext.Next)
		}
		reviewNext := requireNextAction(t, testNext(driftedContext.Next.Options), 0, "review_diff")
		reviewTemplate := requireTemplate(t, reviewNext)
		if argv := stringSliceFromAny(t, reviewTemplate["argv"]); strings.Join(argv, " ") != "specctl diff runtime:session-lifecycle" {
			t.Fatalf("review template.argv = %v", argv)
		}
		syncNext := requireNextAction(t, testNext(driftedContext.Next.Options), 1, "sync")
		syncTemplate := requireTemplate(t, syncNext)
		if argv := stringSliceFromAny(t, syncTemplate["argv"]); strings.Join(argv, " ") != "specctl sync runtime:session-lifecycle --checkpoint HEAD" {
			t.Fatalf("sync template.argv = %v", argv)
		}
		if required := requiredFieldNames(t, syncTemplate["required_fields"]); strings.Join(required, ",") != "summary" {
			t.Fatalf("required_fields = %v", required)
		}

		stdout, stderr, exitCode = runSpecctlBinary(t, binary, repoRoot, "Code reviewed, spec unchanged.\n", "sync", "runtime:session-lifecycle", "--checkpoint", "HEAD")
		if exitCode != 0 {
			t.Fatalf("sync failed: exit=%d stderr=%q stdout=%q", exitCode, stderr, stdout)
		}
		var syncEnvelope struct {
			State struct {
				Checkpoint     string `json:"checkpoint"`
				LastVerifiedAt string `json:"last_verified_at"`
				ScopeDrift     struct {
					Status string `json:"status"`
				} `json:"scope_drift"`
			} `json:"state"`
			Result struct {
				Kind       string `json:"kind"`
				Checkpoint string `json:"checkpoint"`
			} `json:"result"`
			Next testNext `json:"next"`
		}
		mustUnmarshalJSON(t, stdout, &syncEnvelope)
		if syncEnvelope.Result.Kind != "sync" || syncEnvelope.Result.Checkpoint != newHead || syncEnvelope.State.Checkpoint != newHead || syncEnvelope.State.ScopeDrift.Status != "clean" || syncEnvelope.State.LastVerifiedAt == "2026-03-28" || len(syncEnvelope.Next) != 0 {
			t.Fatalf("sync = %#v", syncEnvelope)
		}
		if stderr != "" {
			t.Fatalf("sync stderr = %q", stderr)
		}
	})

	t.Run("checkpoint repair converges with sync", func(t *testing.T) {
		repoRoot := copyFixtureRepoWithRegistry(t, "verified-spec")
		_ = initGitRepoAtDate(t, repoRoot, "2026-03-28T12:00:00Z")
		trackingPath := filepath.Join(repoRoot, ".specs", "runtime", "session-lifecycle.yaml")
		replaceInFile(t, trackingPath, "checkpoint: a1b2c3f", "checkpoint: deadbee")

		stdout, stderr, exitCode := runSpecctlBinary(t, binary, repoRoot, "", "context", "runtime:session-lifecycle")
		if exitCode != 0 {
			t.Fatalf("context failed: exit=%d stderr=%q stdout=%q", exitCode, stderr, stdout)
		}
		var unavailableContext struct {
			Focus struct {
				ScopeDrift struct {
					Status string `json:"status"`
				} `json:"scope_drift"`
			} `json:"focus"`
			State struct {
				ScopeDrift struct {
					Status string `json:"status"`
				} `json:"scope_drift"`
			} `json:"state"`
			Next struct {
				Mode  string `json:"mode"`
				Steps []any  `json:"steps"`
			} `json:"next"`
		}
		mustUnmarshalJSON(t, stdout, &unavailableContext)
		if unavailableContext.State.ScopeDrift.Status != "unavailable" || unavailableContext.Focus.ScopeDrift.Status != "unavailable" {
			t.Fatalf("context = %#v", unavailableContext)
		}
		if unavailableContext.Next.Mode != "sequence" || len(unavailableContext.Next.Steps) != 1 {
			t.Fatalf("next = %#v", unavailableContext.Next)
		}
		syncNext := requireNextAction(t, testNext(unavailableContext.Next.Steps), 0, "sync_checkpoint")
		syncTemplate := requireTemplate(t, syncNext)
		if argv := stringSliceFromAny(t, syncTemplate["argv"]); strings.Join(argv, " ") != "specctl sync runtime:session-lifecycle --checkpoint HEAD" {
			t.Fatalf("sync template.argv = %v", argv)
		}
		if required := requiredFieldNames(t, syncTemplate["required_fields"]); strings.Join(required, ",") != "summary" {
			t.Fatalf("required_fields = %v", required)
		}

		headSHA := strings.TrimSpace(runGit(t, repoRoot, "rev-parse", "HEAD"))
		stdout, stderr, exitCode = runSpecctlBinary(t, binary, repoRoot, "Repair missing checkpoint.\n", "sync", "runtime:session-lifecycle", "--checkpoint", "HEAD")
		if exitCode != 0 {
			t.Fatalf("sync failed: exit=%d stderr=%q stdout=%q", exitCode, stderr, stdout)
		}
		var syncEnvelope struct {
			State struct {
				Checkpoint string `json:"checkpoint"`
				ScopeDrift struct {
					Status string `json:"status"`
				} `json:"scope_drift"`
			} `json:"state"`
			Result struct {
				Kind               string `json:"kind"`
				PreviousCheckpoint string `json:"previous_checkpoint"`
				Checkpoint         string `json:"checkpoint"`
			} `json:"result"`
		}
		mustUnmarshalJSON(t, stdout, &syncEnvelope)
		if syncEnvelope.Result.Kind != "sync" || syncEnvelope.Result.PreviousCheckpoint != "deadbee" || syncEnvelope.Result.Checkpoint != headSHA || syncEnvelope.State.Checkpoint != headSHA || syncEnvelope.State.ScopeDrift.Status != "clean" {
			t.Fatalf("sync = %#v", syncEnvelope)
		}
		if stderr != "" {
			t.Fatalf("sync stderr = %q", stderr)
		}
	})

	t.Run("tracked drift regresses when uncovered changes arrive", func(t *testing.T) {
		repoRoot := copyFixtureRepoWithRegistry(t, "verified-spec")
		headSHA := initGitRepoAtDate(t, repoRoot, "2026-03-28T12:00:00Z")
		replaceInFile(t, filepath.Join(repoRoot, ".specs", "runtime", "session-lifecycle.yaml"), "checkpoint: a1b2c3f", "checkpoint: "+headSHA)
		docPath := filepath.Join(repoRoot, "runtime", "src", "domain", "session_execution", "SPEC.md")
		if err := os.WriteFile(docPath, []byte("---\nspec: session-lifecycle\ncharter: runtime\n---\n# Session Lifecycle\n\n## Requirement: Compensation stage 4 failure cleanup\n\n```gherkin requirement\n@runtime @e2e\nFeature: Compensation stage 4 failure cleanup\n```\n\n### Scenarios\n\n```gherkin scenario\nScenario: Cleanup runs after stage 4 failure\n  Given stage 4 fails during compensation\n  When recovery completes\n  Then cleanup steps run in documented order\n```\n\n## Requirement: Tracked diff\n\n```gherkin requirement\n@runtime\nFeature: Tracked diff\n```\n\n### Scenarios\n\n```gherkin scenario\nScenario: Continue verification after diff review\n  Given the drift is already tracked by a delta\n  When the diff is inspected\n  Then the requirement chain continues\n```\n\n## Drift Review\n\nTracked drift.\n"), 0o644); err != nil {
			t.Fatalf("write design doc: %v", err)
		}
		runGitAtDate(t, repoRoot, "2026-03-30T09:30:00Z", "add", ".")
		runGitAtDate(t, repoRoot, "2026-03-30T09:30:00Z", "commit", "-m", "tracked design drift")

		stdout, stderr, exitCode := runSpecctlBinary(t, binary, repoRoot, "current: The committed design-doc drift is not yet tracked\ntarget: Track the committed diff before convergence\nnotes: Opened after the diff commit\n", "delta", "add", "runtime:session-lifecycle", "--intent", "add", "--area", "Tracked diff")
		if exitCode != 0 {
			t.Fatalf("delta add failed: exit=%d stderr=%q stdout=%q", exitCode, stderr, stdout)
		}
		runGitAtDate(t, repoRoot, "2026-03-30T09:35:00Z", "add", ".specs/runtime/session-lifecycle.yaml")
		runGitAtDate(t, repoRoot, "2026-03-30T09:35:00Z", "commit", "-m", "track diff")

		stdout, stderr, exitCode = runSpecctlBinary(t, binary, repoRoot, "", "context", "runtime:session-lifecycle")
		if exitCode != 0 {
			t.Fatalf("tracked context failed: exit=%d stderr=%q stdout=%q", exitCode, stderr, stdout)
		}
		var trackedContext struct {
			State struct {
				ScopeDrift struct {
					Status    string   `json:"status"`
					TrackedBy []string `json:"tracked_by"`
				} `json:"scope_drift"`
			} `json:"state"`
		}
		mustUnmarshalJSON(t, stdout, &trackedContext)
		if trackedContext.State.ScopeDrift.Status != "tracked" || len(trackedContext.State.ScopeDrift.TrackedBy) == 0 {
			t.Fatalf("tracked context = %#v", trackedContext)
		}

		codePath := filepath.Join(repoRoot, "runtime", "src", "domain", "session_execution", "handler.py")
		if err := os.WriteFile(codePath, []byte("def handle():\n    return 'uncovered drift'\n"), 0o644); err != nil {
			t.Fatalf("write code file: %v", err)
		}
		runGitAtDate(t, repoRoot, "2026-03-30T10:00:00Z", "add", ".")
		runGitAtDate(t, repoRoot, "2026-03-30T10:00:00Z", "commit", "-m", "uncovered drift")

		stdout, stderr, exitCode = runSpecctlBinary(t, binary, repoRoot, "", "context", "runtime:session-lifecycle")
		if exitCode != 0 {
			t.Fatalf("regressed context failed: exit=%d stderr=%q stdout=%q", exitCode, stderr, stdout)
		}
		var regressedContext struct {
			State struct {
				ScopeDrift struct {
					Status      string `json:"status"`
					DriftSource string `json:"drift_source"`
				} `json:"scope_drift"`
			} `json:"state"`
			Next testNext `json:"next"`
		}
		mustUnmarshalJSON(t, stdout, &regressedContext)
		if regressedContext.State.ScopeDrift.Status != "drifted" || regressedContext.State.ScopeDrift.DriftSource != "both" {
			t.Fatalf("regressed context = %#v", regressedContext)
		}
		if len(regressedContext.Next) != 1 {
			t.Fatalf("next = %#v", regressedContext.Next)
		}
		requireNextAction(t, regressedContext.Next, 0, "review_diff")
		if stderr != "" {
			t.Fatalf("context stderr = %q", stderr)
		}
	})

	t.Run("multi-section design-doc drift tells the agent to group by intent", func(t *testing.T) {
		repoRoot := copyFixtureRepoWithRegistry(t, "verified-spec")
		headSHA := initGitRepoAtDate(t, repoRoot, "2026-03-28T12:00:00Z")
		replaceInFile(t, filepath.Join(repoRoot, ".specs", "runtime", "session-lifecycle.yaml"), "checkpoint: a1b2c3f", "checkpoint: "+headSHA)
		docPath := filepath.Join(repoRoot, "runtime", "src", "domain", "session_execution", "SPEC.md")
		if err := os.WriteFile(docPath, []byte("---\nspec: session-lifecycle\ncharter: runtime\n---\n# Session Lifecycle\n\n## Overview\nUpdated overview.\n\n## Recovery Notes\nExpanded recovery notes.\n"), 0o644); err != nil {
			t.Fatalf("write design doc: %v", err)
		}
		runGitAtDate(t, repoRoot, "2026-03-30T09:30:00Z", "add", ".")
		runGitAtDate(t, repoRoot, "2026-03-30T09:30:00Z", "commit", "-m", "multi-section drift")

		stdout, stderr, exitCode := runSpecctlBinary(t, binary, repoRoot, "", "diff", "runtime:session-lifecycle")
		if exitCode != 0 {
			t.Fatalf("diff failed: exit=%d stderr=%q stdout=%q", exitCode, stderr, stdout)
		}
		var diffEnvelope struct {
			State struct {
				ScopeCode struct {
					ChangedFiles []string `json:"changed_files"`
				} `json:"scope_code"`
			} `json:"state"`
			Next testNext `json:"next"`
		}
		mustUnmarshalJSON(t, stdout, &diffEnvelope)
		if len(diffEnvelope.State.ScopeCode.ChangedFiles) != 0 {
			t.Fatalf("scope_code = %#v", diffEnvelope.State.ScopeCode)
		}
		create := requireNextAction(t, diffEnvelope.Next, 0, "delta_add_add")
		if why, _ := create["why"].(string); !strings.Contains(why, "Choose the semantic path that matches the observed contract change.") {
			t.Fatalf("create_delta = %#v", create)
		}
		change := requireNextAction(t, diffEnvelope.Next, 1, "delta_add_change")
		if _, ok := change["choose_when"].(string); !ok {
			t.Fatalf("delta_add_change = %#v", change)
		}
		if stderr != "" {
			t.Fatalf("diff stderr = %q", stderr)
		}
	})
}

func TestBinaryExistingDocFrontmatterJourney(t *testing.T) {
	t.Parallel()
	binary := buildSpecctlBinary(t)
	repoRoot := newBinaryJourneyRepo(t)

	stdout, stderr, exitCode := runSpecctlBinary(t, binary, repoRoot, "title: Runtime System\ndescription: Specs for runtime control-plane and data-plane behavior\ngroups:\n  - key: recovery\n    title: Recovery and Cleanup\n    order: 20\n", "charter", "create", "runtime")
	if exitCode != 0 {
		t.Fatalf("charter create failed: exit=%d stderr=%q stdout=%q", exitCode, stderr, stdout)
	}
	if stderr != "" {
		t.Fatalf("charter create stderr = %q", stderr)
	}

	docPath := filepath.Join(repoRoot, "runtime", "src", "domain", "session_execution", "SPEC.md")
	if err := os.WriteFile(docPath, []byte("# Session Lifecycle\n\nExisting draft notes.\n"), 0o644); err != nil {
		t.Fatalf("write design doc: %v", err)
	}

	stdout, stderr, exitCode = runSpecctlBinary(t, binary, repoRoot, "", "spec", "create", "runtime:session-lifecycle", "--title", "Session Lifecycle", "--doc", "runtime/src/domain/session_execution/SPEC.md", "--scope", "runtime/src/domain/session_execution/", "--group", "recovery", "--order", "20", "--charter-notes", "Session FSM and cleanup behavior")
	if exitCode != 0 {
		t.Fatalf("spec create failed: exit=%d stderr=%q stdout=%q", exitCode, stderr, stdout)
	}
	if stderr != "" {
		t.Fatalf("spec create stderr = %q", stderr)
	}
	var createEnvelope struct {
		State struct {
			Slug      string `json:"slug"`
			Documents struct {
				Primary string `json:"primary"`
			} `json:"documents"`
			CharterMembership struct {
				Group string `json:"group"`
				Notes string `json:"notes"`
			} `json:"charter_membership"`
		} `json:"state"`
		Result struct {
			Kind            string `json:"kind"`
			DesignDocAction string `json:"design_doc_action"`
			SelectedFormat  any    `json:"selected_format"`
		} `json:"result"`
		Next testNextMaps `json:"next"`
	}
	mustUnmarshalJSON(t, stdout, &createEnvelope)
	if createEnvelope.Result.Kind != "spec" || createEnvelope.Result.DesignDocAction != "prepended_frontmatter" {
		t.Fatalf("spec create envelope = %#v", createEnvelope)
	}
	if createEnvelope.Result.SelectedFormat != nil {
		t.Fatalf("selected_format = %#v, want null when no configured format matches", createEnvelope.Result.SelectedFormat)
	}
	if createEnvelope.State.Slug != "session-lifecycle" || createEnvelope.State.Documents.Primary != "runtime/src/domain/session_execution/SPEC.md" {
		t.Fatalf("spec create state = %#v", createEnvelope.State)
	}
	if createEnvelope.State.CharterMembership.Group != "recovery" || createEnvelope.State.CharterMembership.Notes != "Session FSM and cleanup behavior" {
		t.Fatalf("charter_membership = %#v", createEnvelope.State.CharterMembership)
	}
	// spec create returns create_format_template when no format is configured
	if len(createEnvelope.Next) != 1 || createEnvelope.Next[0]["action"] != "create_format_template" {
		t.Fatalf("spec create next = %#v", createEnvelope.Next)
	}

	content, err := os.ReadFile(docPath)
	if err != nil {
		t.Fatalf("read design doc: %v", err)
	}
	if !strings.HasPrefix(string(content), "---\nspec: session-lifecycle\ncharter: runtime\n---\n") {
		t.Fatalf("design doc content = %q", string(content))
	}

	initGitRepo(t, repoRoot)

	stdout, stderr, exitCode = runSpecctlBinary(t, binary, repoRoot, "", "context", "runtime:session-lifecycle")
	if exitCode != 0 {
		t.Fatalf("context failed: exit=%d stderr=%q stdout=%q", exitCode, stderr, stdout)
	}
	if stderr != "" {
		t.Fatalf("context stderr = %q", stderr)
	}
	var contextEnvelope struct {
		State struct {
			Slug         string `json:"slug"`
			Status       string `json:"status"`
			TrackingFile string `json:"tracking_file"`
			Documents    struct {
				Primary string `json:"primary"`
			} `json:"documents"`
			Validation struct {
				Valid bool `json:"valid"`
			} `json:"validation"`
		} `json:"state"`
		Next testNext `json:"next"`
	}
	mustUnmarshalJSON(t, stdout, &contextEnvelope)
	if contextEnvelope.State.Slug != "session-lifecycle" || contextEnvelope.State.Status != "draft" || contextEnvelope.State.TrackingFile != ".specs/runtime/session-lifecycle.yaml" {
		t.Fatalf("context state = %#v", contextEnvelope.State)
	}
	if contextEnvelope.State.Documents.Primary != "runtime/src/domain/session_execution/SPEC.md" || !contextEnvelope.State.Validation.Valid || len(contextEnvelope.Next) != 2 {
		t.Fatalf("context state/next = %#v %#v", contextEnvelope.State, contextEnvelope.Next)
	}
	requireNextAction(t, contextEnvelope.Next, 0, "review_diff")
	requireNextAction(t, contextEnvelope.Next, 1, "sync")
}

func TestBinaryExampleCommand(t *testing.T) {
	t.Parallel()
	binary := buildSpecctlBinary(t)
	repoRoot := copyFixtureRepoWithRegistry(t, "verified-spec")

	stdout, stderr, exitCode := runSpecctlBinary(t, binary, repoRoot, "", "example")
	if exitCode != 0 {
		t.Fatalf("example failed: exit=%d stderr=%q stdout=%q", exitCode, stderr, stdout)
	}
	if stderr != "" {
		t.Fatalf("example stderr = %q", stderr)
	}

	var envelope struct {
		State struct {
			Kind string `json:"kind"`
		} `json:"state"`
		Focus struct {
			Files []struct {
				Path string `json:"path"`
				Role string `json:"role"`
			} `json:"files"`
		} `json:"focus"`
		Result struct {
			DesignDocument string `json:"design_document"`
			FormatTemplate string `json:"format_template"`
			Config         string `json:"config"`
			Charter        string `json:"charter"`
			Tracking       string `json:"tracking"`
		} `json:"result"`
		Next testNext `json:"next"`
	}
	mustUnmarshalJSON(t, stdout, &envelope)
	if envelope.State.Kind != "example" {
		t.Fatalf("state.kind = %q", envelope.State.Kind)
	}
	if len(envelope.Focus.Files) != 5 {
		t.Fatalf("focus.files = %#v", envelope.Focus.Files)
	}
	if envelope.Result.DesignDocument == "" || envelope.Result.FormatTemplate == "" || envelope.Result.Config == "" || envelope.Result.Charter == "" || envelope.Result.Tracking == "" {
		t.Fatalf("result = %#v", envelope.Result)
	}
	if len(envelope.Next) != 0 {
		t.Fatalf("next = %#v", envelope.Next)
	}
}

func TestBinaryErrorRecoveryJourney(t *testing.T) {
	t.Parallel()
	binary := buildSpecctlBinary(t)
	repoRoot := copyFixtureRepoWithRegistry(t, "active-spec")

	stdout, stderr, exitCode := runSpecctlBinary(t, binary, repoRoot, "", "context", "runtime:session-lifecycle")
	if exitCode != 0 {
		t.Fatalf("context failed: exit=%d stderr=%q stdout=%q", exitCode, stderr, stdout)
	}
	contextEnvelope := parseBinaryEnvelope(t, stdout)
	if contextEnvelope.State["status"] != "active" {
		t.Fatalf("context state = %#v", contextEnvelope.State)
	}

	stdout, stderr, exitCode = runSpecctlBinary(t, binary, repoRoot, "", "delta", "close", "runtime:session-lifecycle", "D-001")
	if exitCode == 0 {
		t.Fatalf("delta close should fail: stdout=%q stderr=%q", stdout, stderr)
	}
	failure := parseBinaryEnvelope(t, stdout)
	if failure.Error == nil || failure.Error.Code != "UNVERIFIED_REQUIREMENTS" {
		t.Fatalf("unexpected failure %#v", failure)
	}
	if failure.State["status"] != "active" {
		t.Fatalf("failure state = %#v", failure.State)
	}
	focus := failure.State["focus"].(map[string]any)
	if focus["delta"].(map[string]any)["id"] != "D-001" || focus["delta"].(map[string]any)["status"] != "in-progress" {
		t.Fatalf("failure focus = %#v", focus)
	}
	verifyNext := requireNextAction(t, failure.Next, 0, "verify_requirement")
	if argv := stringSliceFromAny(t, requireTemplate(t, verifyNext)["argv"]); !strings.HasPrefix(strings.Join(argv, " "), "specctl req verify runtime:session-lifecycle REQ-001 --test-file runtime/tests/e2e/journeys/") {
		t.Fatalf("verify next argv = %v", argv)
	}

	testDir := filepath.Join(repoRoot, "runtime", "tests", "e2e", "journeys")
	if err := os.MkdirAll(testDir, 0o755); err != nil {
		t.Fatalf("mkdir test dir: %v", err)
	}
	testFile := filepath.Join(testDir, "test_compensation_cleanup.py")
	if err := os.WriteFile(testFile, []byte("def test_recovery():\n    assert True\n"), 0o644); err != nil {
		t.Fatalf("write test file: %v", err)
	}

	stdout, stderr, exitCode = runSpecctlBinary(t, binary, repoRoot, "", "req", "verify", "runtime:session-lifecycle", "REQ-001", "--test-file", "runtime/tests/e2e/journeys/test_compensation_cleanup.py")
	if exitCode != 0 {
		t.Fatalf("req verify failed: exit=%d stderr=%q stdout=%q", exitCode, stderr, stdout)
	}
	var verifyEnvelope struct {
		State struct {
			Status string `json:"status"`
		} `json:"state"`
		Focus struct {
			Requirement struct {
				ID           string `json:"id"`
				Verification string `json:"verification"`
			} `json:"requirement"`
		} `json:"focus"`
		Result struct {
			Kind        string `json:"kind"`
			Requirement struct {
				ID           string `json:"id"`
				Verification string `json:"verification"`
			} `json:"requirement"`
		} `json:"result"`
		Next testNext `json:"next"`
	}
	mustUnmarshalJSON(t, stdout, &verifyEnvelope)
	if verifyEnvelope.State.Status != "active" || verifyEnvelope.Focus.Requirement.ID != "REQ-001" || verifyEnvelope.Focus.Requirement.Verification != "verified" || verifyEnvelope.Result.Kind != "requirement" || verifyEnvelope.Result.Requirement.ID != "REQ-001" || verifyEnvelope.Result.Requirement.Verification != "verified" {
		t.Fatalf("verify envelope = %#v", verifyEnvelope)
	}
	closeNext := requireNextAction(t, verifyEnvelope.Next, 0, "close_delta")
	if argv := stringSliceFromAny(t, requireTemplate(t, closeNext)["argv"]); strings.Join(argv, " ") != "specctl delta close runtime:session-lifecycle D-001" {
		t.Fatalf("close next argv = %v", argv)
	}
	stdout, stderr, exitCode = runSpecctlBinary(t, binary, repoRoot, "", "delta", "close", "runtime:session-lifecycle", "D-001")
	if exitCode != 0 {
		t.Fatalf("delta close retry failed: exit=%d stderr=%q stdout=%q", exitCode, stderr, stdout)
	}
	var closeEnvelope struct {
		State struct {
			Status string `json:"status"`
		} `json:"state"`
		Focus struct {
			Delta struct {
				ID     string `json:"id"`
				Status string `json:"status"`
			} `json:"delta"`
		} `json:"focus"`
		Result struct {
			Kind  string `json:"kind"`
			Delta struct {
				ID     string `json:"id"`
				Status string `json:"status"`
			} `json:"delta"`
		} `json:"result"`
		Next testNextMaps `json:"next"`
	}
	mustUnmarshalJSON(t, stdout, &closeEnvelope)
	if closeEnvelope.State.Status != "verified" || closeEnvelope.Focus.Delta.ID != "D-001" || closeEnvelope.Focus.Delta.Status != "closed" || closeEnvelope.Result.Kind != "delta" || closeEnvelope.Result.Delta.ID != "D-001" || closeEnvelope.Result.Delta.Status != "closed" {
		t.Fatalf("close envelope = %#v", closeEnvelope)
	}
	if len(closeEnvelope.Next) == 0 || closeEnvelope.Next[0]["action"] != "rev_bump" {
		t.Fatalf("close next = %#v", closeEnvelope.Next)
	}

	stdout, stderr, exitCode = runSpecctlBinary(t, binary, repoRoot, "", "context", "runtime:session-lifecycle")
	if exitCode != 0 {
		t.Fatalf("final context failed: exit=%d stderr=%q stdout=%q", exitCode, stderr, stdout)
	}
	var finalContext struct {
		State struct {
			Status string `json:"status"`
		} `json:"state"`
		Next testNext `json:"next"`
	}
	mustUnmarshalJSON(t, stdout, &finalContext)
	if finalContext.State.Status != "verified" || len(finalContext.Next) != 0 {
		t.Fatalf("final context = %#v", finalContext)
	}
}

func TestBinaryCharterLifecycleJourney(t *testing.T) {
	t.Parallel()
	binary := buildSpecctlBinary(t)
	repoRoot := newBinaryJourneyRepo(t)
	if err := os.MkdirAll(filepath.Join(repoRoot, "adapters", "inbound"), 0o755); err != nil {
		t.Fatalf("mkdir inbound: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(repoRoot, "adapters", "shared"), 0o755); err != nil {
		t.Fatalf("mkdir shared: %v", err)
	}

	stdout, stderr, exitCode := runSpecctlBinary(t, binary, repoRoot, "", "context", "adapters")
	if exitCode != 0 {
		t.Fatalf("context adapters failed: exit=%d stderr=%q stdout=%q", exitCode, stderr, stdout)
	}
	missingCharter := parseBinaryEnvelope(t, stdout)
	createCharter := requireNextAction(t, missingCharter.Next, 0, "create_charter")
	createCharterTemplate := requireTemplate(t, createCharter)
	if argv := stringSliceFromAny(t, createCharterTemplate["argv"]); strings.Join(argv, " ") != "specctl charter create adapters" {
		t.Fatalf("create_charter.template.argv = %v", argv)
	}
	if createCharterTemplate["stdin_template"] != "title: <title>\ndescription: <description>\ngroups:\n  - key: <group_key>\n    title: <group_title>\n    order: <group_order>\n" {
		t.Fatalf("create_charter.stdin_template = %#v", createCharterTemplate["stdin_template"])
	}
	if required := requiredFieldDescriptions(t, createCharterTemplate["required_fields"]); strings.Join(required, "|") != "Human-readable charter title|One-paragraph charter description|Initial group key|Initial group title|Integer group order" {
		t.Fatalf("create_charter.required_field_descriptions = %v", required)
	}

	stdout, stderr, exitCode = runSpecctlBinary(t, binary, repoRoot, "title: Adapter Layer\ndescription: Specs for adapter contracts\ngroups:\n  - key: inbound\n    title: Inbound Adapters\n    order: 10\n", "charter", "create", "adapters")
	if exitCode != 0 {
		t.Fatalf("charter create failed: exit=%d stderr=%q stdout=%q", exitCode, stderr, stdout)
	}
	var charterCreate struct {
		State struct {
			Name       string `json:"name"`
			Validation struct {
				Valid bool `json:"valid"`
			} `json:"validation"`
		} `json:"state"`
		Result struct {
			Kind string `json:"kind"`
		} `json:"result"`
		Next testNextMaps `json:"next"`
	}
	mustUnmarshalJSON(t, stdout, &charterCreate)
	if charterCreate.State.Name != "adapters" || !charterCreate.State.Validation.Valid || charterCreate.Result.Kind != "charter" {
		t.Fatalf("charter create envelope = %#v", charterCreate)
	}
	createSpec := requireNextAction(t, mapsToAny(charterCreate.Next), 0, "create_spec")
	template := requireTemplate(t, createSpec)
	if required := requiredFieldNames(t, template["required_fields"]); strings.Join(required, ",") != "slug,title,design_doc,scope_dir_1,order,charter_notes" {
		t.Fatalf("required_fields = %v", required)
	}
	if required := requiredFieldDescriptions(t, template["required_fields"]); strings.Join(required, "|") != "Kebab-case spec identifier inside the charter|Human-readable spec title|Repo-relative markdown path|Governed directory ending in /|Integer order inside the group|Short planning note" {
		t.Fatalf("required_field_descriptions = %v", required)
	}
	if _, exists := template["stdin_format"]; exists {
		t.Fatalf("template.stdin_format = %#v, want omitted for create_spec argv template", template["stdin_format"])
	}
	if _, exists := template["stdin_template"]; exists {
		t.Fatalf("template.stdin_template = %#v, want omitted for create_spec argv template", template["stdin_template"])
	}

	stdout, stderr, exitCode = runSpecctlBinary(t, binary, repoRoot, "", "spec", "create", "adapters:http-contract", "--title", "HTTP Contract", "--doc", "adapters/inbound/SPEC.md", "--scope", "adapters/inbound/", "--group", "inbound", "--order", "10", "--charter-notes", "HTTP surface")
	if exitCode != 0 {
		t.Fatalf("spec create 1 failed: exit=%d stderr=%q stdout=%q", exitCode, stderr, stdout)
	}
	var createHTTP struct {
		State struct {
			Status string `json:"status"`
		} `json:"state"`
		Result struct {
			Kind            string `json:"kind"`
			DesignDocAction string `json:"design_doc_action"`
		} `json:"result"`
		Next testNextMaps `json:"next"`
	}
	mustUnmarshalJSON(t, stdout, &createHTTP)
	if createHTTP.State.Status != "draft" || createHTTP.Result.Kind != "spec" || createHTTP.Result.DesignDocAction != "bootstrapped" {
		t.Fatalf("create http envelope = %#v", createHTTP)
	}
	// spec create returns create_format_template when no format is configured
	if len(createHTTP.Next) != 1 || createHTTP.Next[0]["action"] != "create_format_template" {
		t.Fatalf("create http next = %#v", createHTTP.Next)
	}
	stdout, stderr, exitCode = runSpecctlBinary(t, binary, repoRoot, "", "spec", "create", "adapters:transport-bridge", "--title", "Transport Bridge", "--doc", "adapters/shared/SPEC.md", "--scope", "adapters/shared/", "--group", "inbound", "--order", "20", "--charter-notes", "Transport integration")
	if exitCode != 0 {
		t.Fatalf("spec create 2 failed: exit=%d stderr=%q stdout=%q", exitCode, stderr, stdout)
	}
	var createTransport struct {
		State struct {
			Status string `json:"status"`
		} `json:"state"`
		Result struct {
			Kind string `json:"kind"`
		} `json:"result"`
	}
	mustUnmarshalJSON(t, stdout, &createTransport)
	if createTransport.State.Status != "draft" || createTransport.Result.Kind != "spec" {
		t.Fatalf("create transport envelope = %#v", createTransport)
	}

	stdout, stderr, exitCode = runSpecctlBinary(t, binary, repoRoot, "group: inbound\norder: 20\ndepends_on:\n  - http-contract\nnotes: Transport integration\n", "charter", "add-spec", "adapters", "transport-bridge")
	if exitCode != 0 {
		t.Fatalf("charter add-spec failed: exit=%d stderr=%q stdout=%q", exitCode, stderr, stdout)
	}
	var addSpecEnvelope struct {
		State struct {
			OrderedSpecs []struct {
				Slug      string   `json:"slug"`
				Order     int      `json:"order"`
				DependsOn []string `json:"depends_on"`
				Status    string   `json:"status"`
			} `json:"ordered_specs"`
			Validation struct {
				Valid bool `json:"valid"`
			} `json:"validation"`
		} `json:"state"`
		Result struct {
			Kind  string `json:"kind"`
			Entry struct {
				Slug      string   `json:"slug"`
				Order     int      `json:"order"`
				DependsOn []string `json:"depends_on"`
			} `json:"entry"`
		} `json:"result"`
		Next testNext `json:"next"`
	}
	mustUnmarshalJSON(t, stdout, &addSpecEnvelope)
	if addSpecEnvelope.Result.Kind != "charter_entry" || addSpecEnvelope.Result.Entry.Slug != "transport-bridge" || addSpecEnvelope.Result.Entry.Order != 20 || strings.Join(addSpecEnvelope.Result.Entry.DependsOn, ",") != "http-contract" || !addSpecEnvelope.State.Validation.Valid {
		t.Fatalf("charter add-spec envelope = %#v", addSpecEnvelope)
	}
	if len(addSpecEnvelope.State.OrderedSpecs) != 2 || addSpecEnvelope.State.OrderedSpecs[0].Slug != "http-contract" || addSpecEnvelope.State.OrderedSpecs[1].Slug != "transport-bridge" || strings.Join(addSpecEnvelope.State.OrderedSpecs[1].DependsOn, ",") != "http-contract" || addSpecEnvelope.State.OrderedSpecs[1].Status != "draft" {
		t.Fatalf("charter add-spec state = %#v", addSpecEnvelope.State)
	}
	if len(addSpecEnvelope.Next) != 0 {
		t.Fatalf("next = %#v", addSpecEnvelope.Next)
	}

	stdout, stderr, exitCode = runSpecctlBinary(t, binary, repoRoot, "", "context", "adapters")
	if exitCode != 0 {
		t.Fatalf("context charter failed: exit=%d stderr=%q stdout=%q", exitCode, stderr, stdout)
	}
	var charterEnvelope struct {
		State struct {
			OrderedSpecs []struct {
				Slug      string   `json:"slug"`
				DependsOn []string `json:"depends_on"`
				Status    string   `json:"status"`
			} `json:"ordered_specs"`
		} `json:"state"`
	}
	mustUnmarshalJSON(t, stdout, &charterEnvelope)
	if len(charterEnvelope.State.OrderedSpecs) != 2 || charterEnvelope.State.OrderedSpecs[0].Slug != "http-contract" || charterEnvelope.State.OrderedSpecs[1].Slug != "transport-bridge" || strings.Join(charterEnvelope.State.OrderedSpecs[1].DependsOn, ",") != "http-contract" || charterEnvelope.State.OrderedSpecs[0].Status != "draft" || charterEnvelope.State.OrderedSpecs[1].Status != "draft" {
		t.Fatalf("ordered_specs = %#v", charterEnvelope.State.OrderedSpecs)
	}

	stdout, stderr, exitCode = runSpecctlBinary(t, binary, repoRoot, "", "diff", "--charter", "adapters")
	if exitCode != 0 {
		t.Fatalf("diff charter failed: exit=%d stderr=%q stdout=%q", exitCode, stderr, stdout)
	}
	var diffEnvelope struct {
		State struct {
			OrderedSpecs []struct {
				Slug      string   `json:"slug"`
				DependsOn []string `json:"depends_on"`
			} `json:"ordered_specs"`
		} `json:"state"`
	}
	mustUnmarshalJSON(t, stdout, &diffEnvelope)
	if len(diffEnvelope.State.OrderedSpecs) != 2 || diffEnvelope.State.OrderedSpecs[0].Slug != "http-contract" || diffEnvelope.State.OrderedSpecs[1].Slug != "transport-bridge" || strings.Join(diffEnvelope.State.OrderedSpecs[1].DependsOn, ",") != "http-contract" {
		t.Fatalf("diff ordered_specs = %#v", diffEnvelope.State.OrderedSpecs)
	}

	stdout, stderr, exitCode = runSpecctlBinary(t, binary, repoRoot, "", "charter", "remove-spec", "adapters", "http-contract")
	if exitCode == 0 {
		t.Fatalf("expected dependent removal failure, stdout=%q stderr=%q", stdout, stderr)
	}
	failure := parseBinaryEnvelope(t, stdout)
	if failure.Error == nil || failure.Error.Code != "CHARTER_DEPENDENCY_EXISTS" {
		t.Fatalf("unexpected removal failure %#v", failure)
	}
	dependents := stringSliceFromAny(t, failure.State["focus"].(map[string]any)["dependents"])
	if strings.Join(dependents, ",") != "transport-bridge" {
		t.Fatalf("dependents = %v", dependents)
	}
	if len(failure.Next) != 0 {
		t.Fatalf("removal next = %#v", failure.Next)
	}
}

func TestBinaryFileResolutionJourney(t *testing.T) {
	t.Parallel()
	binary := buildSpecctlBinary(t)

	t.Run("no match returns canonical create-spec transcript", func(t *testing.T) {
		repoRoot := copyFixtureRepoWithRegistry(t, "ready-spec")
		if err := os.MkdirAll(filepath.Join(repoRoot, "runtime", "src", "domain", "new_protocol"), 0o755); err != nil {
			t.Fatalf("mkdir new protocol: %v", err)
		}
		if err := os.WriteFile(filepath.Join(repoRoot, "runtime", "src", "domain", "new_protocol", "handler.go"), []byte("package main\n"), 0o644); err != nil {
			t.Fatalf("write handler: %v", err)
		}

		stdout, stderr, exitCode := runSpecctlBinary(t, binary, repoRoot, "", "context", "--file", "runtime/src/domain/new_protocol/handler.go")
		if exitCode != 0 {
			t.Fatalf("context --file failed: exit=%d stderr=%q stdout=%q", exitCode, stderr, stdout)
		}
		if stderr != "" {
			t.Fatalf("context --file stderr = %q", stderr)
		}
		var envelope struct {
			State map[string]any `json:"state"`
			Next  testNextMaps   `json:"next"`
		}
		mustUnmarshalJSON(t, stdout, &envelope)
		if envelope.State["resolution"] != "unmatched" || envelope.State["match_source"] != nil || envelope.State["governing_spec"] != nil {
			t.Fatalf("state = %#v", envelope.State)
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

	t.Run("no match under a missing charter still returns only spec create", func(t *testing.T) {
		repoRoot := copyFixtureRepoWithRegistry(t, "ready-spec")
		if err := os.MkdirAll(filepath.Join(repoRoot, "adapters", "src", "http"), 0o755); err != nil {
			t.Fatalf("mkdir adapters/src/http: %v", err)
		}
		if err := os.WriteFile(filepath.Join(repoRoot, "adapters", "src", "http", "client.py"), []byte("pass\n"), 0o644); err != nil {
			t.Fatalf("write adapters client: %v", err)
		}

		stdout, stderr, exitCode := runSpecctlBinary(t, binary, repoRoot, "", "context", "--file", "adapters/src/http/client.py")
		if exitCode != 0 {
			t.Fatalf("context --file failed: exit=%d stderr=%q stdout=%q", exitCode, stderr, stdout)
		}
		if stderr != "" {
			t.Fatalf("context --file stderr = %q", stderr)
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
	})

	t.Run("ambiguous keeps both ordered matches and manual review paths", func(t *testing.T) {
		repoRoot := t.TempDir()
		if err := os.MkdirAll(filepath.Join(repoRoot, ".specs", "runtime"), 0o755); err != nil {
			t.Fatalf("mkdir specs: %v", err)
		}
		if err := os.MkdirAll(filepath.Join(repoRoot, ".specs", "platform"), 0o755); err != nil {
			t.Fatalf("mkdir platform specs: %v", err)
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
  - slug: session-lifecycle
    group: recovery
    order: 20
    depends_on: []
    notes: Session lifecycle
`), 0o644); err != nil {
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
`), 0o644); err != nil {
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
  primary: runtime/src/domain/session-lifecycle/SPEC.md
scope:
  - runtime/src/domain/shared/
deltas: []
requirements: []
changelog: []
`), 0o644); err != nil {
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
  primary: runtime/src/domain/runtime-api-contract/SPEC.md
scope:
  - runtime/src/domain/shared/
deltas: []
requirements: []
changelog: []
`), 0o644); err != nil {
			t.Fatalf("write platform tracking: %v", err)
		}

		stdout, stderr, exitCode := runSpecctlBinary(t, binary, repoRoot, "", "context", "--file", "runtime/src/domain/shared/transport.py")
		if exitCode != 0 {
			t.Fatalf("context --file failed: exit=%d stderr=%q stdout=%q", exitCode, stderr, stdout)
		}
		if stderr != "" {
			t.Fatalf("context --file stderr = %q", stderr)
		}
		var envelope struct {
			State struct {
				Resolution  string `json:"resolution"`
				MatchSource string `json:"match_source"`
				Matches     []struct {
					Slug        string `json:"slug"`
					MatchSource string `json:"match_source"`
					ScopePrefix string `json:"scope_prefix"`
				} `json:"matches"`
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
		if envelope.State.Resolution != "ambiguous" || envelope.State.MatchSource != "scope" {
			t.Fatalf("state = %#v", envelope.State)
		}
		if len(envelope.State.Matches) != 2 || envelope.State.Matches[0].Slug != "runtime-api-contract" || envelope.State.Matches[1].Slug != "session-lifecycle" {
			t.Fatalf("matches = %#v", envelope.State.Matches)
		}
		if envelope.State.Matches[0].MatchSource != "scope" || envelope.State.Matches[1].MatchSource != "scope" || envelope.State.Matches[0].ScopePrefix != "runtime/src/domain/shared/" || envelope.State.Matches[1].ScopePrefix != "runtime/src/domain/shared/" {
			t.Fatalf("matches = %#v", envelope.State.Matches)
		}
		if envelope.State.Validation.Valid || len(envelope.State.Validation.Findings) != 1 || envelope.State.Validation.Findings[0].Code != "AMBIGUOUS_FILE_OWNERSHIP" {
			t.Fatalf("validation = %#v", envelope.State.Validation)
		}
		if len(envelope.Next) != 0 {
			t.Fatalf("next = %#v", envelope.Next)
		}
	})

	t.Run("same-charter ties stay ambiguous", func(t *testing.T) {
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

		stdout, stderr, exitCode := runSpecctlBinary(t, binary, repoRoot, "", "context", "--file", "runtime/src/domain/shared/transport.py")
		if exitCode != 0 {
			t.Fatalf("context --file failed: exit=%d stderr=%q stdout=%q", exitCode, stderr, stdout)
		}
		if stderr != "" {
			t.Fatalf("context --file stderr = %q", stderr)
		}
		var envelope struct {
			State struct {
				Resolution    string `json:"resolution"`
				MatchSource   string `json:"match_source"`
				GoverningSpec any    `json:"governing_spec"`
				Matches       []struct {
					Slug string `json:"slug"`
				} `json:"matches"`
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
		if envelope.State.Resolution != "ambiguous" || envelope.State.MatchSource != "scope" || envelope.State.GoverningSpec != nil {
			t.Fatalf("state = %#v", envelope.State)
		}
		if len(envelope.State.Matches) != 2 || envelope.State.Matches[0].Slug != "first-owner" || envelope.State.Matches[1].Slug != "second-owner" {
			t.Fatalf("matches = %#v", envelope.State.Matches)
		}
		if envelope.State.Validation.Valid || len(envelope.State.Validation.Findings) != 1 || envelope.State.Validation.Findings[0].Code != "AMBIGUOUS_FILE_OWNERSHIP" {
			t.Fatalf("validation = %#v", envelope.State.Validation)
		}
		if len(envelope.Next) != 0 {
			t.Fatalf("next = %#v", envelope.Next)
		}
	})
}

func TestBinaryHookSameCharterTieLeavesFileUnmatched(t *testing.T) {
	t.Parallel()
	binary := buildSpecctlBinary(t)
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

	stdout, stderr, exitCode := runSpecctlBinary(t, binary, repoRoot, "runtime/src/domain/shared/transport.py\n", "hook")
	if exitCode != 0 {
		t.Fatalf("hook failed: exit=%d stderr=%q stdout=%q", exitCode, stderr, stdout)
	}
	if stderr != "" {
		t.Fatalf("hook stderr = %q", stderr)
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
				} `json:"findings"`
			} `json:"validation"`
		} `json:"state"`
	}
	mustUnmarshalJSON(t, stdout, &envelope)
	if len(envelope.State.UnmatchedFiles) != 1 || envelope.State.UnmatchedFiles[0] != "runtime/src/domain/shared/transport.py" {
		t.Fatalf("unmatched_files = %#v", envelope.State.UnmatchedFiles)
	}
	if len(envelope.State.AffectedSpecs) != 0 {
		t.Fatalf("affected_specs = %#v", envelope.State.AffectedSpecs)
	}
	if envelope.State.Validation.Valid || len(envelope.State.Validation.Findings) != 1 || envelope.State.Validation.Findings[0].Code != "UNOWNED_SOURCE_FILE" {
		t.Fatalf("validation = %#v", envelope.State.Validation)
	}
}

func TestBinaryHookIntegrationJourney(t *testing.T) {
	t.Parallel()
	binary := buildSpecctlBinary(t)
	repoRoot := copyFixtureRepoWithRegistry(t, "charter-dag")
	if err := os.MkdirAll(filepath.Join(repoRoot, "runtime", "src", "orphan"), 0o755); err != nil {
		t.Fatalf("mkdir orphan: %v", err)
	}
	if err := os.WriteFile(filepath.Join(repoRoot, "runtime", "src", "orphan", "worker.py"), []byte("pass\n"), 0o644); err != nil {
		t.Fatalf("write orphan: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(repoRoot, "runtime", "src", "domain", "session_execution"), 0o755); err != nil {
		t.Fatalf("mkdir service dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(repoRoot, "runtime", "src", "domain", "session_execution", "services.py"), []byte("pass\n"), 0o644); err != nil {
		t.Fatalf("write service file: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(repoRoot, "docs"), 0o755); err != nil {
		t.Fatalf("mkdir docs: %v", err)
	}
	if err := os.WriteFile(filepath.Join(repoRoot, "docs", "notes.md"), []byte("# Notes\n"), 0o644); err != nil {
		t.Fatalf("write docs: %v", err)
	}

	stdout, stderr, exitCode := runSpecctlBinary(t, binary, repoRoot, ".specs/runtime/CHARTER.yaml\n.specs/runtime/session-lifecycle.yaml\n.specs/specctl.yaml\nruntime/src/domain/session_execution/services.py\nruntime/src/orphan/worker.py\ndocs/notes.md\n", "hook")
	if exitCode != 0 {
		t.Fatalf("hook failed: exit=%d stderr=%q stdout=%q", exitCode, stderr, stdout)
	}
	if stderr != "" {
		t.Fatalf("hook stderr = %q", stderr)
	}
	var hookEnvelope struct {
		State struct {
			InputFiles      []string `json:"input_files"`
			ConsideredFiles []string `json:"considered_files"`
			IgnoredFiles    []string `json:"ignored_files"`
			UnmatchedFiles  []string `json:"unmatched_files"`
			AffectedSpecs   []struct {
				Charter            string   `json:"charter"`
				Status             string   `json:"status"`
				TrackingFile       string   `json:"tracking_file"`
				Slug               string   `json:"slug"`
				DesignDoc          string   `json:"design_doc"`
				DesignDocStaged    bool     `json:"design_doc_staged"`
				TrackingFileStaged bool     `json:"tracking_file_staged"`
				MatchedFiles       []string `json:"matched_files"`
			} `json:"affected_specs"`
			Validation struct {
				Valid    bool `json:"valid"`
				Findings []struct {
					Code string `json:"code"`
				} `json:"findings"`
			} `json:"validation"`
		} `json:"state"`
		Next testNext `json:"next"`
	}
	mustUnmarshalJSON(t, stdout, &hookEnvelope)
	if strings.Join(hookEnvelope.State.InputFiles, ",") != ".specs/runtime/CHARTER.yaml,.specs/runtime/session-lifecycle.yaml,.specs/specctl.yaml,runtime/src/domain/session_execution/services.py,runtime/src/orphan/worker.py,docs/notes.md" {
		t.Fatalf("input_files = %#v", hookEnvelope.State.InputFiles)
	}
	if strings.Join(hookEnvelope.State.ConsideredFiles, ",") != ".specs/runtime/CHARTER.yaml,.specs/runtime/session-lifecycle.yaml,.specs/specctl.yaml,runtime/src/domain/session_execution/services.py,runtime/src/orphan/worker.py" {
		t.Fatalf("considered_files = %#v", hookEnvelope.State.ConsideredFiles)
	}
	if len(hookEnvelope.State.IgnoredFiles) != 1 || hookEnvelope.State.IgnoredFiles[0] != "docs/notes.md" {
		t.Fatalf("ignored_files = %#v", hookEnvelope.State.IgnoredFiles)
	}
	if len(hookEnvelope.State.UnmatchedFiles) != 1 || hookEnvelope.State.UnmatchedFiles[0] != "runtime/src/orphan/worker.py" {
		t.Fatalf("unmatched_files = %#v", hookEnvelope.State.UnmatchedFiles)
	}
	if len(hookEnvelope.State.AffectedSpecs) != 3 {
		t.Fatalf("affected_specs = %#v", hookEnvelope.State.AffectedSpecs)
	}
	if hookEnvelope.State.Validation.Valid || len(hookEnvelope.State.Validation.Findings) != 1 || hookEnvelope.State.Validation.Findings[0].Code != "UNOWNED_SOURCE_FILE" {
		t.Fatalf("validation = %#v", hookEnvelope.State.Validation)
	}
	if len(hookEnvelope.Next) != 0 {
		t.Fatalf("hook next = %#v", hookEnvelope.Next)
	}
	for _, spec := range hookEnvelope.State.AffectedSpecs {
		if spec.Charter != "runtime" || spec.TrackingFile == "" || spec.DesignDoc == "" || spec.DesignDocStaged {
			t.Fatalf("affected spec metadata = %#v", spec)
		}
		if spec.Slug == "session-lifecycle" {
			if !spec.TrackingFileStaged || strings.Join(spec.MatchedFiles, ",") != "runtime/src/domain/session_execution/services.py" {
				t.Fatalf("session-lifecycle hook state = %#v", spec)
			}
		} else if spec.TrackingFileStaged || len(spec.MatchedFiles) != 0 {
			t.Fatalf("unexpected staged tracking file on %#v", spec)
		}
	}
}

func TestBinaryLenientReadJourneys(t *testing.T) {
	t.Parallel()
	binary := buildSpecctlBinary(t)

	t.Run("spec context returns validation findings for malformed tracking state", func(t *testing.T) {
		repoRoot := copyFixtureRepo(t, "malformed-gapful-spec")
		stdout, stderr, exitCode := runSpecctlBinary(t, binary, repoRoot, "", "context", "runtime:session-lifecycle")
		if exitCode != 0 {
			t.Fatalf("context failed: exit=%d stderr=%q stdout=%q", exitCode, stderr, stdout)
		}
		if stderr != "" {
			t.Fatalf("context stderr = %q", stderr)
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
			Next testNext `json:"next"`
		}
		mustUnmarshalJSON(t, stdout, &envelope)
		if envelope.State.Slug != "session-lifecycle" || envelope.State.Validation.Valid || len(envelope.State.Validation.Findings) == 0 || len(envelope.Next) != 1 {
			t.Fatalf("envelope = %#v", envelope)
		}
		requireNextAction(t, envelope.Next, 0, "sync_checkpoint")
	})

	t.Run("charter context returns validation findings for malformed charter state", func(t *testing.T) {
		repoRoot := copyFixtureRepo(t, "charter-cycle")
		stdout, stderr, exitCode := runSpecctlBinary(t, binary, repoRoot, "", "context", "runtime")
		if exitCode != 0 {
			t.Fatalf("context failed: exit=%d stderr=%q stdout=%q", exitCode, stderr, stdout)
		}
		if stderr != "" {
			t.Fatalf("context stderr = %q", stderr)
		}
		var envelope struct {
			State struct {
				Name         string `json:"name"`
				OrderedSpecs []struct {
					Slug string `json:"slug"`
				} `json:"ordered_specs"`
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
		if envelope.State.Name != "runtime" || len(envelope.State.OrderedSpecs) != 2 || envelope.State.Validation.Valid || len(envelope.State.Validation.Findings) == 0 || len(envelope.Next) != 0 {
			t.Fatalf("envelope = %#v", envelope)
		}
	})
}

func TestBinaryCharterCreateWithoutGroupsSeedsExecutableCreateSpecTemplate(t *testing.T) {
	t.Parallel()
	binary := buildSpecctlBinary(t)
	repoRoot := newBinaryJourneyRepo(t)

	stdout, stderr, exitCode := runSpecctlBinary(t, binary, repoRoot, "title: Runtime System\ndescription: Specs for runtime control-plane and data-plane behavior\n", "charter", "create", "runtime")
	if exitCode != 0 {
		t.Fatalf("charter create failed: exit=%d stderr=%q stdout=%q", exitCode, stderr, stdout)
	}
	if stderr != "" {
		t.Fatalf("charter create stderr = %q", stderr)
	}

	var charterCreate struct {
		Next testNextMaps `json:"next"`
	}
	mustUnmarshalJSON(t, stdout, &charterCreate)
	createSpec := requireNextAction(t, mapsToAny(charterCreate.Next), 0, "create_spec")
	template := requireTemplate(t, createSpec)
	if argv := stringSliceFromAny(t, template["argv"]); strings.Join(argv, " ") != "specctl spec create runtime:<slug> --title <title> --doc <design_doc> --scope <scope_dir_1>/ --group <group> --group-title <group_title> --group-order <group_order> --order <order> --charter-notes <charter_notes>" {
		t.Fatalf("template.argv = %v", argv)
	}
	if descriptions := requiredFieldDescriptions(t, template["required_fields"]); strings.Join(descriptions, "|") != "Kebab-case spec identifier inside the charter|Human-readable spec title|Repo-relative markdown path|First repo-relative governed directory ending in /|Charter group key|Required only when creating a new group|Integer order for a newly created group|Integer order for the spec inside its group|Short planning note for the charter entry" {
		t.Fatalf("required_field_descriptions = %v", descriptions)
	}
}

func TestBinaryCharterCreateValidationFailureUsesStructuredEnvelope(t *testing.T) {
	t.Parallel()
	binary := buildSpecctlBinary(t)
	repoRoot := newBinaryJourneyRepo(t)

	stdout, stderr, exitCode := runSpecctlBinary(t, binary, repoRoot, "title: Runtime System\ndescription: Specs for runtime control-plane and data-plane behavior\ngroups:\n  - key: execution\n    title: Execution Engine\n    order: -1\n", "charter", "create", "runtime")
	if exitCode == 0 {
		t.Fatalf("expected failure, stdout=%q stderr=%q", stdout, stderr)
	}
	if stderr != "" {
		t.Fatalf("charter create stderr = %q", stderr)
	}

	failure := requireFailureEnvelope(t, stdout, stderr)
	if failure.Error == nil || failure.Error.Code != "VALIDATION_FAILED" {
		t.Fatalf("failure envelope = %#v", failure)
	}
	if failure.State["name"] != "runtime" || failure.State["tracking_file"] != ".specs/runtime/CHARTER.yaml" {
		t.Fatalf("state = %#v", failure.State)
	}
	groups := failure.State["groups"].([]any)
	if len(groups) != 1 || groups[0].(map[string]any)["order"] != float64(-1) {
		t.Fatalf("groups = %#v", groups)
	}
	findings := requireValidationFailure(t, failure.State)
	if findings[0].(map[string]any)["code"] != "CHARTER_GROUP_INVALID" {
		t.Fatalf("findings = %#v", findings)
	}
}

func TestBinarySpecCreateValidationFailureUsesCanonicalEnvelope(t *testing.T) {
	t.Parallel()
	binary := buildSpecctlBinary(t)
	repoRoot := charterOnlyRepo(t)

	stdout, stderr, exitCode := runSpecctlBinary(
		t,
		binary,
		repoRoot,
		"",
		"spec",
		"create",
		"runtime:session-lifecycle",
		"--title",
		"Session Lifecycle",
		"--doc",
		"runtime/src/domain/session_execution/SPEC.md",
		"--scope",
		"runtime/src/domain/session_execution/",
		"--group",
		"recovery",
		"--order",
		"20",
		"--charter-notes",
		"Session FSM and cleanup behavior",
		"--tag",
		"Invalid-Tag",
	)
	if exitCode == 0 {
		t.Fatalf("expected failure, stdout=%q stderr=%q", stdout, stderr)
	}
	if stderr != "" {
		t.Fatalf("spec create stderr = %q", stderr)
	}

	failure := requireFailureEnvelope(t, stdout, stderr)
	if failure.Error == nil || failure.Error.Code != "VALIDATION_FAILED" {
		t.Fatalf("failure envelope = %#v", failure)
	}
	if failure.State["slug"] != "session-lifecycle" || failure.State["charter"] != "runtime" || failure.State["tracking_file"] != ".specs/runtime/session-lifecycle.yaml" {
		t.Fatalf("state = %#v", failure.State)
	}
	membership := failure.State["charter_membership"].(map[string]any)
	if membership["group"] != "recovery" || membership["order"] != float64(20) {
		t.Fatalf("charter_membership = %#v", membership)
	}
	findings := requireValidationFailure(t, failure.State)
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
}

func TestBinarySpecCreateCharterMembershipValidationFailureUsesCanonicalEnvelope(t *testing.T) {
	t.Parallel()
	binary := buildSpecctlBinary(t)
	repoRoot := charterOnlyRepo(t)
	if err := os.MkdirAll(filepath.Join(repoRoot, "runtime", "src", "domain", "session_execution"), 0o755); err != nil {
		t.Fatalf("mkdir design doc dir: %v", err)
	}

	stdout, stderr, exitCode := runSpecctlBinary(
		t,
		binary,
		repoRoot,
		"",
		"spec",
		"create",
		"runtime:session-lifecycle",
		"--title",
		"Session Lifecycle",
		"--doc",
		"runtime/src/domain/session_execution/SPEC.md",
		"--scope",
		"runtime/src/domain/session_execution/",
		"--group",
		"recovery",
		"--order",
		"20",
		"--charter-notes",
		"Session FSM and cleanup behavior",
		"--depends-on",
		"missing-spec",
	)
	if exitCode == 0 {
		t.Fatalf("expected failure, stdout=%q stderr=%q", stdout, stderr)
	}
	if stderr != "" {
		t.Fatalf("spec create stderr = %q", stderr)
	}

	failure := requireFailureEnvelope(t, stdout, stderr)
	if failure.Error == nil || failure.Error.Code != "VALIDATION_FAILED" {
		t.Fatalf("failure envelope = %#v", failure)
	}
	if failure.State["slug"] != "session-lifecycle" || failure.State["charter"] != "runtime" || failure.State["tracking_file"] != ".specs/runtime/session-lifecycle.yaml" {
		t.Fatalf("state = %#v", failure.State)
	}
	membership := failure.State["charter_membership"].(map[string]any)
	if membership["group"] != "recovery" || membership["order"] != float64(20) {
		t.Fatalf("charter_membership = %#v", membership)
	}
	if got := strings.Join(stringSliceFromAny(t, membership["depends_on"]), ","); got != "missing-spec" {
		t.Fatalf("charter_membership.depends_on = %#v", membership["depends_on"])
	}
	findings := requireValidationFailure(t, failure.State)
	foundDependencyInvalid := false
	for _, findingAny := range findings {
		finding := findingAny.(map[string]any)
		if finding["code"] == "CHARTER_DEPENDENCY_INVALID" {
			foundDependencyInvalid = true
		}
	}
	if !foundDependencyInvalid {
		t.Fatalf("findings = %#v", findings)
	}
}

func TestBinaryCharterAddSpecMalformedTrackingFailureEnvelope(t *testing.T) {
	t.Parallel()
	binary := buildSpecctlBinary(t)
	repoRoot := copyFixtureRepoWithRegistry(t, "ready-spec")
	replaceInFile(t, filepath.Join(repoRoot, ".specs", "runtime", "session-lifecycle.yaml"), "status: ready", "status: invalid")

	stdout, stderr, exitCode := runSpecctlBinary(t, binary, repoRoot, "group: recovery\norder: 30\ndepends_on: []\nnotes: Session FSM and cleanup behavior\n", "charter", "add-spec", "runtime", "session-lifecycle")
	if exitCode == 0 {
		t.Fatalf("expected failure, stdout=%q stderr=%q", stdout, stderr)
	}
	if stderr != "" {
		t.Fatalf("charter add-spec stderr = %q", stderr)
	}

	failure := requireFailureEnvelope(t, stdout, stderr)
	if failure.Error == nil || failure.Error.Code != "VALIDATION_FAILED" {
		t.Fatalf("failure envelope = %#v", failure)
	}
	if failure.State["name"] != "runtime" || failure.State["tracking_file"] != ".specs/runtime/CHARTER.yaml" {
		t.Fatalf("state = %#v", failure.State)
	}
	orderedSpecs := failure.State["ordered_specs"].([]any)
	if len(orderedSpecs) != 1 {
		t.Fatalf("ordered_specs = %#v", orderedSpecs)
	}
	specState := orderedSpecs[0].(map[string]any)
	if specState["slug"] != "session-lifecycle" || specState["validation"].(map[string]any)["valid"] != false {
		t.Fatalf("ordered_spec = %#v", specState)
	}
	focus := failure.State["focus"].(map[string]any)
	findings := focus["validation"].(map[string]any)["findings"].([]any)
	if len(findings) == 0 {
		t.Fatalf("focus.validation = %#v", focus)
	}
}

func TestBinaryCharterAddSpecSemanticValidationFailuresUseCanonicalEnvelope(t *testing.T) {
	t.Parallel()
	binary := buildSpecctlBinary(t)
	cases := []struct {
		name              string
		stdin             string
		requiredCode      string
		forbiddenFindings []string
	}{
		{
			name:         "invalid new group key",
			stdin:        "group: bad_group\ngroup_title: Broken Group\ngroup_order: 30\norder: 30\ndepends_on: []\nnotes: Session FSM and cleanup behavior\n",
			requiredCode: "CHARTER_GROUP_INVALID",
		},
		{
			name:  "negative order",
			stdin: "group: recovery\norder: -1\ndepends_on: []\nnotes: Session FSM and cleanup behavior\n",
		},
		{
			name:              "unknown dependency",
			stdin:             "group: recovery\norder: 30\ndepends_on:\n  - missing-spec\nnotes: Session FSM and cleanup behavior\n",
			requiredCode:      "CHARTER_DEPENDENCY_INVALID",
			forbiddenFindings: []string{"CHARTER_CYCLE_PRESENT"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repoRoot := copyFixtureRepoWithRegistry(t, "ready-spec")

			stdout, stderr, exitCode := runSpecctlBinary(t, binary, repoRoot, tc.stdin, "charter", "add-spec", "runtime", "session-lifecycle")
			if exitCode == 0 {
				t.Fatalf("expected failure, stdout=%q stderr=%q", stdout, stderr)
			}
			if stderr != "" {
				t.Fatalf("charter add-spec stderr = %q", stderr)
			}

			failure := requireFailureEnvelope(t, stdout, stderr)
			if failure.Error == nil || failure.Error.Code != "VALIDATION_FAILED" {
				t.Fatalf("failure envelope = %#v", failure)
			}
			if failure.State["name"] != "runtime" || failure.State["tracking_file"] != ".specs/runtime/CHARTER.yaml" {
				t.Fatalf("state = %#v", failure.State)
			}
			validation := failure.State["validation"].(map[string]any)
			findings := validation["findings"].([]any)
			if validation["valid"] != false || len(findings) == 0 {
				t.Fatalf("state.validation = %#v", validation)
			}
			focusFindings := failure.State["focus"].(map[string]any)["validation"].(map[string]any)["findings"].([]any)
			if len(focusFindings) == 0 {
				t.Fatalf("focus.validation = %#v", failure.State["focus"])
			}
			if tc.requiredCode != "" {
				foundRequired := false
				for _, findingAny := range focusFindings {
					finding := findingAny.(map[string]any)
					if finding["code"] == tc.requiredCode {
						foundRequired = true
					}
				}
				if !foundRequired {
					t.Fatalf("focus.validation.findings = %#v", focusFindings)
				}
			}
			for _, forbidden := range tc.forbiddenFindings {
				for _, findingAny := range focusFindings {
					finding := findingAny.(map[string]any)
					if finding["code"] == forbidden {
						t.Fatalf("focus.validation.findings = %#v", focusFindings)
					}
				}
			}
		})
	}
}

func TestBinaryCharterAddSpecCycleIncludesFocus(t *testing.T) {
	t.Parallel()
	binary := buildSpecctlBinary(t)
	repoRoot := copyFixtureRepoWithRegistry(t, "charter-dag")

	stdout, stderr, exitCode := runSpecctlBinary(t, binary, repoRoot, "group: execution\norder: 10\ndepends_on:\n  - session-lifecycle\nnotes: Storage and CAS guarantees\n", "charter", "add-spec", "runtime", "redis-state")
	if exitCode == 0 {
		t.Fatalf("expected failure, stdout=%q stderr=%q", stdout, stderr)
	}
	if stderr != "" {
		t.Fatalf("charter add-spec stderr = %q", stderr)
	}

	failure := requireFailureEnvelope(t, stdout, stderr)
	if failure.Error == nil || failure.Error.Code != "CHARTER_CYCLE" {
		t.Fatalf("failure envelope = %#v", failure)
	}
	focus := failure.State["focus"].(map[string]any)
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
}

func TestBinaryDeltaAddBlankRequiredFieldsUseMissingFieldsFocus(t *testing.T) {
	t.Parallel()
	binary := buildSpecctlBinary(t)
	repoRoot := copyFixtureRepoWithRegistry(t, "ready-spec")

	stdout, stderr, exitCode := runSpecctlBinary(t, binary, repoRoot, "current: \"   \"\ntarget: Target gap\nnotes: Explicitly tracked\n", "delta", "add", "runtime:session-lifecycle", "--intent", "add", "--area", "Blank current")
	if exitCode == 0 {
		t.Fatalf("expected failure, stdout=%q stderr=%q", stdout, stderr)
	}
	if stderr != "" {
		t.Fatalf("delta add stderr = %q", stderr)
	}

	failure := requireFailureEnvelope(t, stdout, stderr)
	if failure.Error == nil || failure.Error.Code != "INVALID_INPUT" {
		t.Fatalf("failure envelope = %#v", failure)
	}
	input := failure.State["focus"].(map[string]any)["input"].(map[string]any)
	if got := strings.Join(stringSliceFromAny(t, input["missing_fields"]), ","); got != "current" {
		t.Fatalf("missing_fields = %#v", input["missing_fields"])
	}
	if _, exists := input["invalid_fields"]; exists {
		t.Fatalf("unexpected invalid_fields = %#v", input["invalid_fields"])
	}
}

func TestBinaryReqVerifyMalformedPathReusesMissingFileEnvelope(t *testing.T) {
	t.Parallel()
	binary := buildSpecctlBinary(t)
	repoRoot := copyFixtureRepoWithRegistry(t, "active-spec")

	stdout, stderr, exitCode := runSpecctlBinary(t, binary, repoRoot, "", "req", "verify", "runtime:session-lifecycle", "REQ-001", "--test-file", "../outside.py")
	if exitCode == 0 {
		t.Fatalf("expected failure, stdout=%q stderr=%q", stdout, stderr)
	}
	if stderr != "" {
		t.Fatalf("req verify stderr = %q", stderr)
	}

	failure := requireFailureEnvelope(t, stdout, stderr)
	if failure.Error == nil || failure.Error.Code != "TEST_FILE_NOT_FOUND" {
		t.Fatalf("failure envelope = %#v", failure)
	}
	focus := failure.State["focus"].(map[string]any)
	if got := strings.Join(stringSliceFromAny(t, focus["invalid_paths"]), ","); got != "../outside.py" {
		t.Fatalf("invalid_paths = %#v", focus["invalid_paths"])
	}
}

func TestBinaryConfigMutationValidation(t *testing.T) {
	// Cannot use t.Parallel() — subtests use t.Setenv
	binary := buildSpecctlBinary(t)

	t.Run("successful config writes rebuild state from persisted config only", func(t *testing.T) {
		repoRoot := copyFixtureRepoWithRegistry(t, "ready-spec")
		writeProjectConfigFixture(t, repoRoot, "gherkin_tags:\n  - runtime\n  - manual\nsource_prefixes:\n  - runtime/src/\nformats:\n  ui-spec:\n    template: ui/src/routes/SPEC-FORMAT.md\n    recommended_for: ui/src/routes/**\n    description: UI spec\n")
		if err := os.MkdirAll(filepath.Join(repoRoot, "ui", "src", "routes"), 0o755); err != nil {
			t.Fatalf("mkdir ui routes: %v", err)
		}
		if err := os.WriteFile(filepath.Join(repoRoot, "ui", "src", "routes", "SPEC-FORMAT.md"), []byte("# UI\n"), 0o644); err != nil {
			t.Fatalf("write format template: %v", err)
		}

		stdout, stderr, exitCode := runSpecctlBinary(t, binary, repoRoot, "", "config", "add-tag", "adapter")
		if exitCode != 0 {
			t.Fatalf("config add-tag failed: exit=%d stderr=%q stdout=%q", exitCode, stderr, stdout)
		}
		if stderr != "" {
			t.Fatalf("config add-tag stderr = %q", stderr)
		}
		var envelope struct {
			State struct {
				GherkinTags []string `json:"gherkin_tags"`
				Validation  struct {
					Valid    bool `json:"valid"`
					Findings []struct {
						Code string `json:"code"`
					} `json:"findings"`
				} `json:"validation"`
			} `json:"state"`
			Result struct {
				Kind     string `json:"kind"`
				Mutation string `json:"mutation"`
				Tag      string `json:"tag"`
			} `json:"result"`
		}
		mustUnmarshalJSON(t, stdout, &envelope)
		if envelope.Result.Kind != "config" || envelope.Result.Mutation != "add-tag" || envelope.Result.Tag != "adapter" {
			t.Fatalf("result = %#v", envelope.Result)
		}
		var rawEnvelope map[string]any
		mustUnmarshalJSON(t, stdout, &rawEnvelope)
		requireObjectKeys(t, rawEnvelope["result"].(map[string]any), "kind", "mutation", "tag")
		if strings.Join(envelope.State.GherkinTags, ",") != "adapter,runtime" {
			t.Fatalf("state.gherkin_tags = %#v", envelope.State.GherkinTags)
		}
		if !envelope.State.Validation.Valid || len(envelope.State.Validation.Findings) != 0 {
			t.Fatalf("state.validation = %#v", envelope.State.Validation)
		}
	})

	t.Run("invalid config tag fails before writing specctl yaml", func(t *testing.T) {
		repoRoot := copyFixtureRepoWithRegistry(t, "ready-spec")
		configPath := filepath.Join(repoRoot, ".specs", "specctl.yaml")
		before, err := os.ReadFile(configPath)
		if err != nil {
			t.Fatalf("read config before mutation: %v", err)
		}

		stdout, stderr, exitCode := runSpecctlBinary(t, binary, repoRoot, "", "config", "add-tag", "BADTAG")
		if exitCode == 0 {
			t.Fatalf("expected failure, stdout=%q", stdout)
		}
		failure := requireFailureEnvelope(t, stdout, stderr)
		if failure.Error == nil || failure.Error.Code != "VALIDATION_FAILED" {
			t.Fatalf("failure envelope = %#v", failure)
		}
		gherkinTags := stringSliceFromAny(t, failure.State["gherkin_tags"])
		if strings.Join(gherkinTags, ",") != "runtime,domain" {
			t.Fatalf("state.gherkin_tags = %v", gherkinTags)
		}
		validation := failure.State["validation"].(map[string]any)
		if validation["valid"] != true {
			t.Fatalf("state.validation = %#v", validation)
		}
		findings, ok := validation["findings"].([]any)
		if !ok || len(findings) != 0 {
			t.Fatalf("state.validation.findings = %#v", validation["findings"])
		}
		focus := failure.State["focus"].(map[string]any)
		mutation := focus["config_mutation"].(map[string]any)
		if mutation["kind"] != "add_tag" || mutation["value"] != "BADTAG" {
			t.Fatalf("config_mutation = %#v", mutation)
		}
		focusFindings := focus["validation"].(map[string]any)["findings"].([]any)
		if len(focusFindings) != 1 || focusFindings[0].(map[string]any)["code"] != "CONFIG_TAG_INVALID" {
			t.Fatalf("validation.findings = %#v", focusFindings)
		}

		after, readErr := os.ReadFile(configPath)
		if readErr != nil {
			t.Fatalf("read config after mutation: %v", readErr)
		}
		if string(after) != string(before) {
			t.Fatalf("config was rewritten on validation failure:\nBEFORE:\n%s\nAFTER:\n%s", before, after)
		}
	})

	t.Run("post-write audit errors surface validation_failed with canonical config state", func(t *testing.T) {
		repoRoot := copyFixtureRepoWithRegistry(t, "ready-spec")
		if err := os.Remove(filepath.Join(repoRoot, ".specs", "runtime", "CHARTER.yaml")); err != nil {
			t.Fatalf("remove charter: %v", err)
		}

		stdout, stderr, exitCode := runSpecctlBinary(t, binary, repoRoot, "", "config", "add-tag", "adapter")
		if exitCode == 0 {
			t.Fatalf("expected failure, stdout=%q stderr=%q", stdout, stderr)
		}
		failure := requireFailureEnvelope(t, stdout, stderr)
		if failure.Error == nil || failure.Error.Code != "VALIDATION_FAILED" {
			t.Fatalf("failure envelope = %#v", failure)
		}
		if gherkinTags := stringSliceFromAny(t, failure.State["gherkin_tags"]); strings.Join(gherkinTags, ",") != "runtime,domain" {
			t.Fatalf("state.gherkin_tags = %v", gherkinTags)
		}
		if validation := failure.State["validation"].(map[string]any); validation["valid"] != false {
			t.Fatalf("state.validation = %#v", validation)
		}
		focus := failure.State["focus"].(map[string]any)
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

	t.Run("redundant semantic tag warning is emitted only once across binary invocations", func(t *testing.T) {
		t.Setenv("XDG_CACHE_HOME", t.TempDir())
		repoRoot := copyFixtureRepoWithRegistry(t, "ready-spec")
		replaceInFile(t, filepath.Join(repoRoot, ".specs", "specctl.yaml"), "  - domain\n", "  - domain\n  - manual\n")

		stdout, stderr, exitCode := runSpecctlBinary(t, binary, repoRoot, "", "config")
		if exitCode != 0 {
			t.Fatalf("first config failed: exit=%d stderr=%q stdout=%q", exitCode, stderr, stdout)
		}
		if stderr != "" {
			t.Fatalf("first config stderr = %q", stderr)
		}
		first := parseBinaryEnvelope(t, stdout)
		requireFindingCode(t, first.State["validation"].(map[string]any)["findings"].([]any), "REDUNDANT_SEMANTIC_TAG")

		stdout, stderr, exitCode = runSpecctlBinary(t, binary, repoRoot, "", "config")
		if exitCode != 0 {
			t.Fatalf("second config failed: exit=%d stderr=%q stdout=%q", exitCode, stderr, stdout)
		}
		if stderr != "" {
			t.Fatalf("second config stderr = %q", stderr)
		}
		second := parseBinaryEnvelope(t, stdout)
		if validation := second.State["validation"].(map[string]any); validation["valid"] != true {
			t.Fatalf("second validation = %#v", validation)
		}

		stdout, stderr, exitCode = runSpecctlBinary(t, binary, repoRoot, "", "config", "add-tag", "adapter")
		if exitCode != 0 {
			t.Fatalf("add-tag failed: exit=%d stderr=%q stdout=%q", exitCode, stderr, stdout)
		}

		stdout, stderr, exitCode = runSpecctlBinary(t, binary, repoRoot, "", "config")
		if exitCode != 0 {
			t.Fatalf("third config failed: exit=%d stderr=%q stdout=%q", exitCode, stderr, stdout)
		}
		if stderr != "" {
			t.Fatalf("third config stderr = %q", stderr)
		}
		third := parseBinaryEnvelope(t, stdout)
		if validation := third.State["validation"].(map[string]any); validation["valid"] != true {
			t.Fatalf("third validation = %#v", validation)
		}
	})

	t.Run("remove-prefix malformed path reuses prefix-not-found envelope", func(t *testing.T) {
		repoRoot := copyFixtureRepoWithRegistry(t, "ready-spec")

		stdout, stderr, exitCode := runSpecctlBinary(t, binary, repoRoot, "", "config", "remove-prefix", "../outside")
		if exitCode == 0 {
			t.Fatalf("expected failure, stdout=%q stderr=%q", stdout, stderr)
		}
		if stderr != "" {
			t.Fatalf("config remove-prefix stderr = %q", stderr)
		}

		failure := requireFailureEnvelope(t, stdout, stderr)
		if failure.Error == nil || failure.Error.Code != "PREFIX_NOT_FOUND" {
			t.Fatalf("failure envelope = %#v", failure)
		}
		focus := failure.State["focus"].(map[string]any)
		mutation := focus["config_mutation"].(map[string]any)
		if mutation["kind"] != "remove_prefix" || mutation["value"] != "../outside" {
			t.Fatalf("config_mutation = %#v", mutation)
		}
		if got := strings.Join(stringSliceFromAny(t, focus["invalid_paths"]), ","); got != "../outside" {
			t.Fatalf("invalid_paths = %#v", focus["invalid_paths"])
		}
	})
}

func TestBinaryCharterWritePostWriteValidationFailures(t *testing.T) {
	t.Parallel()
	binary := buildSpecctlBinary(t)
	repoRoot := copyFixtureRepoWithRegistry(t, "ready-spec")

	stdout, stderr, exitCode := runSpecctlBinary(t, binary, repoRoot, "", "charter", "remove-spec", "runtime", "session-lifecycle")
	if exitCode != 0 {
		t.Fatalf("expected success, stdout=%q stderr=%q", stdout, stderr)
	}
	if stderr != "" {
		t.Fatalf("stderr = %q", stderr)
	}
	envelope := parseBinaryEnvelope(t, stdout)
	if validation := envelope.State["validation"].(map[string]any); validation["valid"] != true {
		t.Fatalf("state.validation = %#v", validation)
	}
	if _, err := os.Stat(filepath.Join(repoRoot, ".specs", "runtime", "session-lifecycle.yaml")); err != nil {
		t.Fatalf("tracking file should remain after remove-spec: %v", err)
	}
}

func TestBinaryNonRuntimeNextActionsUseSpecContext(t *testing.T) {
	t.Parallel()
	binary := buildSpecctlBinary(t)
	repoRoot := uiReadySpecRepo(t)

	stdout, stderr, exitCode := runSpecctlBinary(t, binary, repoRoot, "current: Current state\ntarget: Target state\nnotes: Explicitly tracked\n", "delta", "add", "ui:thread-lifecycle", "--intent", "add", "--area", "Workspace tabs")
	if exitCode != 0 {
		t.Fatalf("delta add failed: exit=%d stderr=%q stdout=%q", exitCode, stderr, stdout)
	}
	if stderr != "" {
		t.Fatalf("delta add stderr = %q", stderr)
	}

	var deltaAdd struct {
		Next testNextMaps `json:"next"`
	}
	mustUnmarshalJSON(t, stdout, &deltaAdd)
	if stdinTemplate := requireTemplate(t, deltaAdd.Next[1])["stdin_template"]; stdinTemplate != "@ui\nFeature: <feature>\n" {
		t.Fatalf("stdin_template = %#v", stdinTemplate)
	}

	stdout, stderr, exitCode = runSpecctlBinary(t, binary, repoRoot, "@ui\nFeature: Thread lifecycle\n\n  Scenario: The workspace tab restores a thread\n    Given a user reopens the workspace\n    When the active thread is restored\n    Then the thread tab shows the latest state\n", "req", "add", "ui:thread-lifecycle", "--delta", "D-001")
	if exitCode != 0 {
		t.Fatalf("req add failed: exit=%d stderr=%q stdout=%q", exitCode, stderr, stdout)
	}
	if stderr != "" {
		t.Fatalf("req add stderr = %q", stderr)
	}

	var reqAdd struct {
		Next testNextMaps `json:"next"`
	}
	mustUnmarshalJSON(t, stdout, &reqAdd)
	argv := stringSliceFromAny(t, requireTemplate(t, reqAdd.Next[1])["argv"])
	if strings.Join(argv, " ") != "specctl req verify ui:thread-lifecycle REQ-001 --test-file ui/tests/domain/test_thread_lifecycle.py" {
		t.Fatalf("next.argv = %v", argv)
	}
}

func TestBinaryDefaultConfigPrefixesAndHook(t *testing.T) {
	t.Parallel()
	binary := buildSpecctlBinary(t)
	repoRoot := t.TempDir()
	if err := os.MkdirAll(filepath.Join(repoRoot, ".specs", "runtime"), 0o755); err != nil {
		t.Fatalf("mkdir specs: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(repoRoot, "ui", "server", "session_execution"), 0o755); err != nil {
		t.Fatalf("mkdir ui/server: %v", err)
	}
	if err := os.WriteFile(filepath.Join(repoRoot, "ui", "server", "session_execution", "handler.ts"), []byte("export {};\n"), 0o644); err != nil {
		t.Fatalf("write handler: %v", err)
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
    notes: Server session sync behavior
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
  primary: ui/server/session_execution/SPEC.md
scope:
  - ui/server/session_execution/
deltas: []
requirements: []
changelog: []
`), 0o644); err != nil {
		t.Fatalf("write tracking: %v", err)
	}

	stdout, stderr, exitCode := runSpecctlBinary(t, binary, repoRoot, "", "config")
	if exitCode != 0 {
		t.Fatalf("config failed: exit=%d stderr=%q stdout=%q", exitCode, stderr, stdout)
	}
	if stderr != "" {
		t.Fatalf("config stderr = %q", stderr)
	}
	var configEnvelope struct {
		State struct {
			SourcePrefixes []string `json:"source_prefixes"`
		} `json:"state"`
	}
	mustUnmarshalJSON(t, stdout, &configEnvelope)
	if got := strings.Join(configEnvelope.State.SourcePrefixes, ","); got != "runtime/src/,ui/src/,ui/convex/,ui/server/" {
		t.Fatalf("source_prefixes = %#v", configEnvelope.State.SourcePrefixes)
	}

	stdout, stderr, exitCode = runSpecctlBinary(t, binary, repoRoot, "ui/server/session_execution/handler.ts\n", "hook")
	if exitCode != 0 {
		t.Fatalf("hook failed: exit=%d stderr=%q stdout=%q", exitCode, stderr, stdout)
	}
	if stderr != "" {
		t.Fatalf("hook stderr = %q", stderr)
	}
	var hookEnvelope struct {
		State struct {
			IgnoredFiles   []string `json:"ignored_files"`
			UnmatchedFiles []string `json:"unmatched_files"`
			AffectedSpecs  []struct {
				Slug         string   `json:"slug"`
				MatchedFiles []string `json:"matched_files"`
			} `json:"affected_specs"`
		} `json:"state"`
	}
	mustUnmarshalJSON(t, stdout, &hookEnvelope)
	if len(hookEnvelope.State.IgnoredFiles) != 0 || len(hookEnvelope.State.UnmatchedFiles) != 0 {
		t.Fatalf("hook state = %#v", hookEnvelope.State)
	}
	if len(hookEnvelope.State.AffectedSpecs) != 1 || hookEnvelope.State.AffectedSpecs[0].Slug != "session-sync" {
		t.Fatalf("affected_specs = %#v", hookEnvelope.State.AffectedSpecs)
	}
	if got := strings.Join(hookEnvelope.State.AffectedSpecs[0].MatchedFiles, ","); got != "ui/server/session_execution/handler.ts" {
		t.Fatalf("matched_files = %#v", hookEnvelope.State.AffectedSpecs[0].MatchedFiles)
	}
}

func TestBinaryDiffDeltaSummariesExposeCanonicalPayload(t *testing.T) {
	t.Parallel()
	binary := buildSpecctlBinary(t)
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

	stdout, stderr, exitCode := runSpecctlBinary(t, binary, repoRoot, "", "diff", "runtime:session-lifecycle")
	if exitCode != 0 {
		t.Fatalf("diff failed: exit=%d stderr=%q stdout=%q", exitCode, stderr, stdout)
	}
	if stderr != "" {
		t.Fatalf("diff stderr = %q", stderr)
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
}

func runSpecctlBinary(t *testing.T, binary, repoRoot, stdin string, args ...string) (string, string, int) {
	t.Helper()

	cmd := exec.Command(binary, args...)
	cmd.Dir = repoRoot
	cmd.Stdin = strings.NewReader(stdin)
	var stdout strings.Builder
	var stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err == nil {
		return stdout.String(), stderr.String(), 0
	}
	if exitErr, ok := err.(*exec.ExitError); ok {
		return stdout.String(), stderr.String(), exitErr.ExitCode()
	}
	t.Fatalf("run %v: %v", args, err)
	return "", "", 0
}

func runSpecctlShellPipeline(t *testing.T, binary, repoRoot, stdin string, args ...string) (string, string, int) {
	t.Helper()

	stdinPath := filepath.Join(t.TempDir(), "stdin.txt")
	if err := os.WriteFile(stdinPath, []byte(stdin), 0o644); err != nil {
		t.Fatalf("write stdin file: %v", err)
	}

	// Use sh -c (not -lc) so the shell does not source /etc/profile and
	// its profile.d/ chain. Some sandbox environments ship a bash_completion
	// snippet that uses bash-only `&>` redirection under POSIX sh and leaks
	// a "nvm" token to stdout when the login profile loads, which corrupts
	// the JSON envelope we parse below. The test's intent is shell pipeline
	// exec (stdin pipe, argv, exit code), not login-shell profile sourcing.
	shellArgs := append([]string{"-c", `cat "$SPECCTL_STDIN" | "$SPECCTL_BIN" "$@"`, "specctl-shell"}, args...)
	cmd := exec.Command("sh", shellArgs...)
	cmd.Dir = repoRoot
	cmd.Env = append(os.Environ(), "SPECCTL_BIN="+binary, "SPECCTL_STDIN="+stdinPath)

	var stdout strings.Builder
	var stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err == nil {
		return stdout.String(), stderr.String(), 0
	}
	if exitErr, ok := err.(*exec.ExitError); ok {
		return stdout.String(), stderr.String(), exitErr.ExitCode()
	}
	t.Fatalf("run shell pipeline %v: %v", args, err)
	return "", "", 0
}

func parseBinaryEnvelope(t *testing.T, output string) testEnvelope {
	t.Helper()

	var envelope testEnvelope
	if err := json.Unmarshal([]byte(output), &envelope); err != nil {
		t.Fatalf("unmarshal %q: %v", output, err)
	}
	envelope.State["focus"] = envelope.Focus
	return envelope
}

func newBinaryJourneyRepo(t *testing.T) string {
	t.Helper()

	repoRoot := t.TempDir()
	if err := os.MkdirAll(filepath.Join(repoRoot, ".specs"), 0o755); err != nil {
		t.Fatalf("mkdir specs: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(repoRoot, "runtime", "src", "domain", "session_execution"), 0o755); err != nil {
		t.Fatalf("mkdir source tree: %v", err)
	}
	writeCLIAdjacentTestFile(t, filepath.Join(repoRoot, ".specs", "specctl.yaml"), []byte("gherkin_tags:\n  - runtime\nsource_prefixes:\n  - runtime/src/\nformats: {}\n"))
	writeCLIAdjacentTestFile(t, filepath.Join(repoRoot, "runtime", "src", "domain", "session_execution", "SPEC.md"), []byte("---\nspec: session-lifecycle\ncharter: runtime\n---\n# Session Lifecycle\n\n## Requirement: Compensation cleanup\n\n```gherkin requirement\n@runtime\nFeature: Compensation cleanup\n```\n\n### Scenarios\n\n```gherkin scenario\nScenario: Cleanup runs after a failure\n  Given stage 4 fails during compensation\n  When recovery completes\n  Then cleanup steps run in documented order\n```\n"))
	initGitRepoAtDate(t, repoRoot, "2026-03-28T12:00:00Z")
	return repoRoot
}
