package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestRootHelpListsOnlyV2Commands(t *testing.T) {
	output := executeHelp(t, NewRootCmd(), "--help")
	for _, expected := range []string{"init", "context", "diff", "hook", "spec", "delta", "req", "rev", "sync", "charter", "config"} {
		if !strings.Contains(output, expected) {
			t.Fatalf("expected root help to list %q\n%s", expected, output)
		}
	}
	for _, rejected := range []string{"check", "validate", "lint", "status", "trace", "deps", "refs"} {
		if strings.Contains(output, "\n  "+rejected) {
			t.Fatalf("did not expect root help to list %q\n%s", rejected, output)
		}
	}
	assertAgentFirstSections(t, output)
}

func TestCommandHelpIncludesSpecSurface(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		snippets []string
		rejects  []string
	}{
		{
			name:     "context",
			args:     []string{"context", "--help"},
			snippets: []string{"Usage:\n  specctl context [target]", "--file", "specctl context --file"},
			rejects:  []string{"VALIDATION_FAILED"},
		},
		{
			name:     "diff",
			args:     []string{"diff", "--help"},
			snippets: []string{"Usage:\n  specctl diff [target]", "--charter", "specctl diff --charter runtime"},
			rejects:  []string{"VALIDATION_FAILED"},
		},
		{
			name:     "hook",
			args:     []string{"hook", "--help"},
			snippets: []string{"Usage:\n  specctl hook", "Text lines. Each line is one staged repo-relative path.", "Informational hook"},
			rejects:  []string{"VALIDATION_FAILED", "commit-time enforcement"},
		},
		{
			name:     "spec family",
			args:     []string{"spec", "--help"},
			snippets: []string{"Usage:\n  specctl spec", "spec create", "specctl spec create runtime:runtime-api-contract"},
		},
		{
			name:     "spec create",
			args:     []string{"spec", "create", "--help"},
			snippets: []string{"Usage:\n  specctl spec create <charter:slug>", "--scope", "--group-title", "--charter-notes", "CHECKPOINT_UNAVAILABLE"},
		},
		{
			name:     "delta family",
			args:     []string{"delta", "--help"},
			snippets: []string{"Usage:\n  specctl delta", "add         Add a tracked delta", "close       Move open|in-progress -> closed"},
		},
		{
			name:     "delta add",
			args:     []string{"delta", "add", "--help"},
			snippets: []string{"Usage:\n  specctl delta add <charter:slug>", "current: string, required", "--area"},
		},
		{
			name:     "delta start",
			args:     []string{"delta", "start", "--help"},
			snippets: []string{"Usage:\n  specctl delta start <charter:slug> <delta-id>", "Move open -> in-progress"},
		},
		{
			name:     "delta defer",
			args:     []string{"delta", "defer", "--help"},
			snippets: []string{"Usage:\n  specctl delta defer <charter:slug> <delta-id>", "Move open|in-progress -> deferred"},
		},
		{
			name:     "delta resume",
			args:     []string{"delta", "resume", "--help"},
			snippets: []string{"Usage:\n  specctl delta resume <charter:slug> <delta-id>", "Move deferred -> open"},
		},
		{
			name:     "delta close",
			args:     []string{"delta", "close", "--help"},
			snippets: []string{"Usage:\n  specctl delta close <charter:slug> <delta-id>", "UNVERIFIED_REQUIREMENTS"},
		},
		{
			name:     "req family",
			args:     []string{"req", "--help"},
			snippets: []string{"Usage:\n  specctl req", "req add", "req verify"},
		},
		{
			name:     "req add",
			args:     []string{"req", "add", "--help"},
			snippets: []string{"Usage:\n  specctl req add <charter:slug>", "--delta", "Title and tags are derived from the Gherkin payload."},
		},
		{
			name:     "req verify",
			args:     []string{"req", "verify", "--help"},
			snippets: []string{"Usage:\n  specctl req verify <charter:slug> <requirement-id>", "--test-file", "runtime/tests/domain/test_compensation_cleanup.py", "TEST_FILES_REQUIRED"},
		},
		{
			name:     "rev family",
			args:     []string{"rev", "--help"},
			snippets: []string{"Usage:\n  specctl rev", "rev bump", "reads the changelog summary from stdin"},
		},
		{
			name:     "rev bump",
			args:     []string{"rev", "bump", "--help"},
			snippets: []string{"Usage:\n  specctl rev bump <charter:slug>", "--checkpoint", "One changelog summary paragraph."},
		},
		{
			name:     "sync",
			args:     []string{"sync", "--help"},
			snippets: []string{"Usage:\n  specctl sync <charter:slug>", "--checkpoint", "Required one-line summary explaining why the checkpoint is being re-anchored."},
		},
		{
			name:     "charter family",
			args:     []string{"charter", "--help"},
			snippets: []string{"Usage:\n  specctl charter", "charter create", "charter add-spec"},
		},
		{
			name:     "charter create",
			args:     []string{"charter", "create", "--help"},
			snippets: []string{"Usage:\n  specctl charter create <charter>", "groups: list, optional", "CHARTER_EXISTS"},
		},
		{
			name:     "charter add-spec",
			args:     []string{"charter", "add-spec", "--help"},
			snippets: []string{"Usage:\n  specctl charter add-spec <charter> <slug>", "group_title", "CHARTER_CYCLE"},
		},
		{
			name:     "charter remove-spec",
			args:     []string{"charter", "remove-spec", "--help"},
			snippets: []string{"Usage:\n  specctl charter remove-spec <charter> <slug>", "CHARTER_DEPENDENCY_EXISTS"},
		},
		{
			name:     "config family",
			args:     []string{"config", "--help"},
			snippets: []string{"Usage:\n  specctl config", "specctl config add-tag runtime", "add-prefix", "add-tag", "remove-tag"},
		},
		{
			name:     "config add-tag",
			args:     []string{"config", "add-tag", "--help"},
			snippets: []string{"Usage:\n  specctl config add-tag <tag>", "result: { kind, mutation, tag }", "SEMANTIC_TAG_RESERVED"},
		},
		{
			name:     "config remove-tag",
			args:     []string{"config", "remove-tag", "--help"},
			snippets: []string{"Usage:\n  specctl config remove-tag <tag>", "result: { kind, mutation, tag }", "TAG_IN_USE"},
		},
		{
			name:     "config add-prefix",
			args:     []string{"config", "add-prefix", "--help"},
			snippets: []string{"Usage:\n  specctl config add-prefix <path>", "result: { kind, mutation, prefix }", "PREFIX_EXISTS"},
		},
		{
			name:     "config remove-prefix",
			args:     []string{"config", "remove-prefix", "--help"},
			snippets: []string{"Usage:\n  specctl config remove-prefix <path>", "result: { kind, mutation, prefix }", "PREFIX_NOT_FOUND"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			output := executeHelp(t, NewRootCmd(), tt.args...)
			assertAgentFirstSections(t, output)
			for _, snippet := range tt.snippets {
				if !strings.Contains(output, snippet) {
					t.Fatalf("expected %q in help output\n%s", snippet, output)
				}
			}
			for _, reject := range tt.rejects {
				if strings.Contains(output, reject) {
					t.Fatalf("did not expect %q in help output\n%s", reject, output)
				}
			}
		})
	}
}

func TestFocusedCommandsExposeV2Flags(t *testing.T) {
	root := NewRootCmd()

	specCreate := mustFindCommand(t, root, "spec", "create")
	for _, flag := range []string{"title", "doc", "scope", "group", "group-title", "group-order", "order", "charter-notes", "depends-on", "tag"} {
		if specCreate.Flags().Lookup(flag) == nil {
			t.Fatalf("expected spec create to expose --%s", flag)
		}
	}

	context := mustFindCommand(t, root, "context")
	if context.Flags().Lookup("file") == nil {
		t.Fatal("expected context to expose --file")
	}

	diff := mustFindCommand(t, root, "diff")
	if diff.Flags().Lookup("charter") == nil {
		t.Fatal("expected diff to expose --charter")
	}
}

func TestHelpFailureCatalogsMatchDeclaredMetadata(t *testing.T) {
	tests := []struct {
		name string
		args []string
		path []string
	}{
		{name: "root", args: []string{"--help"}},
		{name: "context", args: []string{"context", "--help"}, path: []string{"context"}},
		{name: "diff", args: []string{"diff", "--help"}, path: []string{"diff"}},
		{name: "hook", args: []string{"hook", "--help"}, path: []string{"hook"}},
		{name: "spec", args: []string{"spec", "--help"}, path: []string{"spec"}},
		{name: "spec create", args: []string{"spec", "create", "--help"}, path: []string{"spec", "create"}},
		{name: "delta", args: []string{"delta", "--help"}, path: []string{"delta"}},
		{name: "delta add", args: []string{"delta", "add", "--help"}, path: []string{"delta", "add"}},
		{name: "delta start", args: []string{"delta", "start", "--help"}, path: []string{"delta", "start"}},
		{name: "delta defer", args: []string{"delta", "defer", "--help"}, path: []string{"delta", "defer"}},
		{name: "delta resume", args: []string{"delta", "resume", "--help"}, path: []string{"delta", "resume"}},
		{name: "delta close", args: []string{"delta", "close", "--help"}, path: []string{"delta", "close"}},
		{name: "req", args: []string{"req", "--help"}, path: []string{"req"}},
		{name: "req add", args: []string{"req", "add", "--help"}, path: []string{"req", "add"}},
		{name: "req verify", args: []string{"req", "verify", "--help"}, path: []string{"req", "verify"}},
		{name: "rev", args: []string{"rev", "--help"}, path: []string{"rev"}},
		{name: "rev bump", args: []string{"rev", "bump", "--help"}, path: []string{"rev", "bump"}},
		{name: "sync", args: []string{"sync", "--help"}, path: []string{"sync"}},
		{name: "charter", args: []string{"charter", "--help"}, path: []string{"charter"}},
		{name: "charter create", args: []string{"charter", "create", "--help"}, path: []string{"charter", "create"}},
		{name: "charter add-spec", args: []string{"charter", "add-spec", "--help"}, path: []string{"charter", "add-spec"}},
		{name: "charter remove-spec", args: []string{"charter", "remove-spec", "--help"}, path: []string{"charter", "remove-spec"}},
		{name: "config", args: []string{"config", "--help"}, path: []string{"config"}},
		{name: "config add-tag", args: []string{"config", "add-tag", "--help"}, path: []string{"config", "add-tag"}},
		{name: "config remove-tag", args: []string{"config", "remove-tag", "--help"}, path: []string{"config", "remove-tag"}},
		{name: "config add-prefix", args: []string{"config", "add-prefix", "--help"}, path: []string{"config", "add-prefix"}},
		{name: "config remove-prefix", args: []string{"config", "remove-prefix", "--help"}, path: []string{"config", "remove-prefix"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := NewRootCmd()
			output := executeHelp(t, root, tt.args...)
			cmd := root
			if len(tt.path) > 0 {
				cmd = mustFindCommand(t, root, tt.path...)
			}
			wantCodes := declaredHelpErrors(cmd)
			if gotCodes := helpErrorCodes(t, output); !reflect.DeepEqual(gotCodes, wantCodes) {
				t.Fatalf("help errors = %v, want %v\n%s", gotCodes, wantCodes, output)
			}
		})
	}
}

func TestDiffHelpFailureCatalogMatchesImplementedBehavior(t *testing.T) {
	root := NewRootCmd()
	output := executeHelp(t, root, "diff", "--help")
	helpCodes := helpErrorCodes(t, output)
	wantCodes := declaredHelpErrors(mustFindCommand(t, root, "diff"))
	if !reflect.DeepEqual(helpCodes, wantCodes) {
		t.Fatalf("diff help errors = %v, want %v\n%s", helpCodes, wantCodes, output)
	}

	observed := make(map[string]struct{}, len(wantCodes))

	stdout, stderr, exitCode := executeCLI("diff")
	if exitCode == 0 {
		t.Fatalf("expected invalid input failure, stdout=%q stderr=%q", stdout, stderr)
	}
	envelope := requireFailureEnvelope(t, stdout, stderr)
	if envelope.Error == nil {
		t.Fatalf("expected invalid input envelope, got %#v", envelope)
	}
	observed[envelope.Error.Code] = struct{}{}

	repoRoot := seededSpecRepo(t)
	withWorkingDir(t, repoRoot, func() {
		stdout, stderr, exitCode = executeCLI("diff", "runtime:missing-spec")
		if exitCode == 0 {
			t.Fatalf("expected missing spec failure, stdout=%q stderr=%q", stdout, stderr)
		}
		envelope = requireFailureEnvelope(t, stdout, stderr)
		if envelope.Error == nil {
			t.Fatalf("expected missing spec envelope, got %#v", envelope)
		}
		observed[envelope.Error.Code] = struct{}{}
	})

	emptyRepo := t.TempDir()
	if err := os.MkdirAll(emptyRepo+"/.specs", 0o755); err != nil {
		t.Fatalf("mkdir empty repo .specs: %v", err)
	}
	withWorkingDir(t, emptyRepo, func() {
		stdout, stderr, exitCode = executeCLI("diff", "--charter", "runtime")
		if exitCode == 0 {
			t.Fatalf("expected missing charter failure, stdout=%q stderr=%q", stdout, stderr)
		}
		envelope = requireFailureEnvelope(t, stdout, stderr)
		if envelope.Error == nil {
			t.Fatalf("expected missing charter envelope, got %#v", envelope)
		}
		observed[envelope.Error.Code] = struct{}{}
	})

	gotCodes := make([]string, 0, len(observed))
	for _, code := range helpCodes {
		if _, ok := observed[code]; ok {
			gotCodes = append(gotCodes, code)
		}
	}
	if !reflect.DeepEqual(gotCodes, wantCodes) {
		t.Fatalf("diff implemented errors = %v, want %v", gotCodes, wantCodes)
	}
}

func TestJourneyCommandHelpErrorsCoverObservedFailures(t *testing.T) {
	newSpecArgs := []string{"spec", "create", "runtime:new-spec", "--doc", "runtime/src/domain/session_execution/SPEC.md", "--scope", "runtime/src/domain/session_execution/", "--group", "recovery", "--order", "10", "--charter-notes", "Session FSM"}
	newSpecArgsWithTitle := []string{"spec", "create", "runtime:new-spec", "--title", "New Spec", "--doc", "runtime/src/domain/session_execution/SPEC.md", "--scope", "runtime/src/domain/session_execution/", "--group", "recovery", "--order", "10", "--charter-notes", "Session FSM"}
	emptyRepo := func(t *testing.T) string {
		t.Helper()
		repoRoot := t.TempDir()
		if err := os.MkdirAll(filepath.Join(repoRoot, ".specs"), 0o755); err != nil {
			t.Fatalf("mkdir .specs: %v", err)
		}
		return repoRoot
	}

	tests := []struct {
		name      string
		path      []string
		helpArgs  []string
		args      []string
		stdin     string
		repoSetup func(*testing.T) string
		wantCode  string
	}{
		{
			name:     "charter create invalid input",
			path:     []string{"charter", "create"},
			helpArgs: []string{"charter", "create", "--help"},
			args:     []string{"charter", "create", "Runtime"},
			stdin:    "title: Runtime System\ndescription: Specs for runtime\n",
			wantCode: "INVALID_INPUT",
		},
		{
			name:      "charter create exists",
			path:      []string{"charter", "create"},
			helpArgs:  []string{"charter", "create", "--help"},
			args:      []string{"charter", "create", "runtime"},
			stdin:     "title: Runtime System\ndescription: Specs for runtime\n",
			repoSetup: func(t *testing.T) string { return copyFixtureRepoWithRegistry(t, "ready-spec") },
			wantCode:  "CHARTER_EXISTS",
		},
		{
			name:      "spec create invalid input",
			path:      []string{"spec", "create"},
			helpArgs:  []string{"spec", "create", "--help"},
			args:      newSpecArgs,
			repoSetup: func(t *testing.T) string { return copyFixtureRepoWithRegistry(t, "ready-spec") },
			wantCode:  "INVALID_INPUT",
		},
		{
			name:      "spec create missing charter",
			path:      []string{"spec", "create"},
			helpArgs:  []string{"spec", "create", "--help"},
			args:      append([]string{}, newSpecArgsWithTitle...),
			repoSetup: emptyRepo,
			wantCode:  "CHARTER_NOT_FOUND",
		},
		{
			name:      "delta add invalid input",
			path:      []string{"delta", "add"},
			helpArgs:  []string{"delta", "add", "--help"},
			args:      []string{"delta", "add", "runtime:session-lifecycle"},
			stdin:     "current: Current gap\ntarget: Target gap\nnotes: Explicitly tracked\n",
			repoSetup: func(t *testing.T) string { return copyFixtureRepoWithRegistry(t, "ready-spec") },
			wantCode:  "INVALID_INPUT",
		},
		{
			name:      "delta add missing spec",
			path:      []string{"delta", "add"},
			helpArgs:  []string{"delta", "add", "--help"},
			args:      []string{"delta", "add", "runtime:session-lifecycle", "--intent", "add", "--area", "Heartbeat timeout"},
			stdin:     "current: Current gap\ntarget: Target gap\nnotes: Explicitly tracked\n",
			repoSetup: emptyRepo,
			wantCode:  "SPEC_NOT_FOUND",
		},
		{
			name:      "req add missing delta",
			path:      []string{"req", "add"},
			helpArgs:  []string{"req", "add", "--help"},
			args:      []string{"req", "add", "runtime:session-lifecycle"},
			stdin:     "@runtime\nFeature: Cleanup\n\n  Scenario: One\n    Given x\n    When y\n    Then z\n",
			repoSetup: func(t *testing.T) string { return copyFixtureRepoWithRegistry(t, "ready-spec") },
			wantCode:  "INVALID_INPUT",
		},
		{
			name:      "req add missing spec",
			path:      []string{"req", "add"},
			helpArgs:  []string{"req", "add", "--help"},
			args:      []string{"req", "add", "runtime:session-lifecycle", "--delta", "D-001"},
			stdin:     "@runtime\nFeature: Cleanup\n\n  Scenario: One\n    Given x\n    When y\n    Then z\n",
			repoSetup: emptyRepo,
			wantCode:  "SPEC_NOT_FOUND",
		},
		{
			name:      "req verify missing test files",
			path:      []string{"req", "verify"},
			helpArgs:  []string{"req", "verify", "--help"},
			args:      []string{"req", "verify", "runtime:session-lifecycle", "REQ-001"},
			repoSetup: func(t *testing.T) string { return copyFixtureRepoWithRegistry(t, "active-spec") },
			wantCode:  "TEST_FILES_REQUIRED",
		},
		{
			name:      "req verify missing requirement",
			path:      []string{"req", "verify"},
			helpArgs:  []string{"req", "verify", "--help"},
			args:      []string{"req", "verify", "runtime:session-lifecycle", "REQ-999", "--test-file", "runtime/tests/domain/test_compensation_cleanup.py"},
			repoSetup: func(t *testing.T) string { return copyFixtureRepoWithRegistry(t, "active-spec") },
			wantCode:  "REQUIREMENT_NOT_FOUND",
		},
		{
			name:     "req verify inactive lifecycle",
			path:     []string{"req", "verify"},
			helpArgs: []string{"req", "verify", "--help"},
			args:     []string{"req", "verify", "runtime:session-lifecycle", "REQ-001", "--test-file", "runtime/tests/domain/test_compensation_cleanup.py"},
			repoSetup: func(t *testing.T) string {
				repoRoot := contractRemoveDeltaRepo(t)
				withWorkingDir(t, repoRoot, func() {
					stdout, stderr, exitCode := executeCLI("req", "withdraw", "runtime:session-lifecycle", "REQ-001", "--delta", "D-002")
					if exitCode != 0 {
						t.Fatalf("req withdraw failed: exit=%d stderr=%q stdout=%q", exitCode, stderr, stdout)
					}
				})
				return repoRoot
			},
			wantCode: "REQUIREMENT_INVALID_LIFECYCLE",
		},
		{
			name:      "delta close unverified requirements",
			path:      []string{"delta", "close"},
			helpArgs:  []string{"delta", "close", "--help"},
			args:      []string{"delta", "close", "runtime:session-lifecycle", "D-001"},
			repoSetup: func(t *testing.T) string { return copyFixtureRepoWithRegistry(t, "active-spec") },
			wantCode:  "UNVERIFIED_REQUIREMENTS",
		},
		{
			name:      "delta close missing delta",
			path:      []string{"delta", "close"},
			helpArgs:  []string{"delta", "close", "--help"},
			args:      []string{"delta", "close", "runtime:session-lifecycle", "D-999"},
			repoSetup: func(t *testing.T) string { return copyFixtureRepoWithRegistry(t, "active-spec") },
			wantCode:  "DELTA_NOT_FOUND",
		},
		{
			name:      "rev bump invalid input",
			path:      []string{"rev", "bump"},
			helpArgs:  []string{"rev", "bump", "--help"},
			args:      []string{"rev", "bump", "runtime:session-lifecycle"},
			stdin:     "Summary\n",
			repoSetup: func(t *testing.T) string { return copyFixtureRepoWithRegistry(t, "verified-spec") },
			wantCode:  "INVALID_INPUT",
		},
		{
			name:      "rev bump missing spec",
			path:      []string{"rev", "bump"},
			helpArgs:  []string{"rev", "bump", "--help"},
			args:      []string{"rev", "bump", "runtime:session-lifecycle", "--checkpoint", "HEAD"},
			stdin:     "Summary\n",
			repoSetup: emptyRepo,
			wantCode:  "SPEC_NOT_FOUND",
		},
		{
			name:      "sync invalid input",
			path:      []string{"sync"},
			helpArgs:  []string{"sync", "--help"},
			args:      []string{"sync", "runtime:session-lifecycle"},
			repoSetup: func(t *testing.T) string { return copyFixtureRepoWithRegistry(t, "verified-spec") },
			wantCode:  "INVALID_INPUT",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := NewRootCmd()
			helpCodes := helpErrorCodes(t, executeHelp(t, root, tt.helpArgs...))
			declared := declaredHelpErrors(mustFindCommand(t, root, tt.path...))
			if !reflect.DeepEqual(helpCodes, declared) {
				t.Fatalf("help errors = %v, want %v", helpCodes, declared)
			}

			run := func() (string, string, int) {
				return executeCLIWithInput(tt.stdin, tt.args...)
			}
			if tt.repoSetup != nil {
				repoRoot := tt.repoSetup(t)
				withWorkingDir(t, repoRoot, func() {
					stdout, stderr, exitCode := run()
					if exitCode == 0 {
						t.Fatalf("expected non-zero exit code, stdout=%q stderr=%q", stdout, stderr)
					}
					envelope := requireFailureEnvelope(t, stdout, stderr)
					if envelope.Error == nil {
						t.Fatalf("expected failure envelope, got %#v", envelope)
					}
					if envelope.Error.Code != tt.wantCode {
						t.Fatalf("error code = %q, want %q", envelope.Error.Code, tt.wantCode)
					}
					if !containsString(helpCodes, envelope.Error.Code) {
						t.Fatalf("help codes %v do not include observed code %q", helpCodes, envelope.Error.Code)
					}
				})
				return
			}

			stdout, stderr, exitCode := run()
			envelope := requireFailureEnvelope(t, stdout, stderr)
			if envelope.Error == nil {
				t.Fatalf("expected failure envelope, got %#v", envelope)
			}
			if exitCode == 0 {
				t.Fatalf("expected non-zero exit code, stdout=%q stderr=%q", stdout, stderr)
			}
			if envelope.Error.Code != tt.wantCode {
				t.Fatalf("error code = %q, want %q", envelope.Error.Code, tt.wantCode)
			}
			if !containsString(helpCodes, envelope.Error.Code) {
				t.Fatalf("help codes %v do not include observed code %q", helpCodes, envelope.Error.Code)
			}
		})
	}
}

func TestContract_InvalidInputFocusedContract(t *testing.T) {
	tests := []struct {
		name            string
		args            []string
		wantErr         string
		wantStateKey    string
		wantFocusReason string
	}{
		{name: "context invalid target", args: []string{"context", "runtime/session-lifecycle"}, wantErr: "context target must be <charter> or <charter:slug>", wantStateKey: "target", wantFocusReason: "invalid_target"},
		{name: "context file conflicts with target", args: []string{"context", "runtime", "--file", "runtime/src/domain/session_execution/services.py"}, wantErr: "context accepts either [target] or --file, not both", wantStateKey: "target", wantFocusReason: "target_and_file_conflict"},
		{name: "diff requires target or charter", args: []string{"diff"}, wantErr: "diff requires either <charter:slug> or --charter <charter>", wantFocusReason: "missing_target"},
		{name: "diff rejects charter positional target", args: []string{"diff", "runtime"}, wantErr: "diff target must be <charter:slug>", wantStateKey: "charter", wantFocusReason: "invalid_target"},
		{name: "diff rejects target plus charter flag", args: []string{"diff", "runtime:session-lifecycle", "--charter", "runtime"}, wantErr: "diff accepts either <charter:slug> or --charter, not both", wantStateKey: "target", wantFocusReason: "target_and_charter_conflict"},
		{name: "spec create requires spec identifier", args: []string{"spec", "create", "runtime"}, wantErr: "spec target must be <charter:slug>"},
		{name: "spec create requires title", args: []string{"spec", "create", "runtime:session-lifecycle", "--doc", "runtime/src/domain/session_execution/SPEC.md", "--scope", "runtime/src/domain/session_execution/", "--group", "recovery", "--order", "10", "--charter-notes", "Session FSM"}, wantErr: "--title is required"},
		{name: "spec create requires scope", args: []string{"spec", "create", "runtime:session-lifecycle", "--title", "Session Lifecycle", "--doc", "runtime/src/domain/session_execution/SPEC.md", "--group", "recovery", "--order", "10", "--charter-notes", "Session FSM"}, wantErr: "--scope is required"},
		{name: "spec create requires explicit order flag", args: []string{"spec", "create", "runtime:session-lifecycle", "--title", "Session Lifecycle", "--doc", "runtime/src/domain/session_execution/SPEC.md", "--scope", "runtime/src/domain/session_execution/", "--group", "recovery", "--charter-notes", "Session FSM"}, wantErr: "--order is required"},
		{name: "spec create pairs new group flags", args: []string{"spec", "create", "runtime:session-lifecycle", "--title", "Session Lifecycle", "--doc", "runtime/src/domain/session_execution/SPEC.md", "--scope", "runtime/src/domain/session_execution/", "--group", "recovery", "--group-title", "Recovery", "--order", "10", "--charter-notes", "Session FSM"}, wantErr: "--group-title and --group-order must be provided together"},
		{name: "charter create validates name", args: []string{"charter", "create", "Runtime"}, wantErr: "charter name must match"},
		{name: "charter add-spec validates slug", args: []string{"charter", "add-spec", "runtime", "SessionLifecycle"}, wantErr: "spec slug must match"},
		{name: "charter remove-spec validates charter", args: []string{"charter", "remove-spec", "Runtime", "session-lifecycle"}, wantErr: "charter name must match"},
		{name: "delta start validates delta id", args: []string{"delta", "start", "runtime:session-lifecycle", "D-1"}, wantErr: "delta ID must be D-NNN"},
		{name: "req verify validates req id", args: []string{"req", "verify", "runtime:session-lifecycle", "REQ-1"}, wantErr: "requirement ID must be REQ-NNN"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stdout, stderr, exitCode := executeCLI(tt.args...)
			if exitCode == 0 {
				t.Fatalf("expected non-zero exit code, got stdout=%q stderr=%q", stdout, stderr)
			}

			envelope := requireFailureEnvelope(t, stdout, stderr)
			if envelope.Error == nil || !strings.Contains(envelope.Error.Message, tt.wantErr) {
				t.Fatalf("expected error containing %q, got %#v", tt.wantErr, envelope)
			}
			if envelope.Error.Code != "INVALID_INPUT" {
				t.Fatalf("error code = %q, want INVALID_INPUT", envelope.Error.Code)
			}
			if strings.Contains(stdout, "Usage:") {
				t.Fatalf("expected JSON-only validation envelope, got %q", stdout)
			}
			if tt.wantStateKey != "" {
				if _, exists := envelope.State[tt.wantStateKey]; !exists {
					t.Fatalf("expected state.%s in %#v", tt.wantStateKey, envelope.State)
				}
			}
			if tt.wantFocusReason != "" {
				input, ok := envelope.Focus["input"].(map[string]any)
				if !ok {
					t.Fatalf("focus = %#v", envelope.Focus)
				}
				if input["reason"] != tt.wantFocusReason {
					t.Fatalf("focus = %#v", envelope.Focus)
				}
			}
		})
	}
}

func TestFocusedWriteValidationUsesSpecState(t *testing.T) {
	repoRoot := seededSpecRepo(t)
	withWorkingDir(t, repoRoot, func() {
		t.Run("delta add requires area", func(t *testing.T) {
			stdout, stderr, exitCode := executeCLIWithInput("current: Current\ntarget: Target\nnotes: Present\n", "delta", "add", "runtime:session-lifecycle", "--intent", "add")
			if exitCode == 0 {
				t.Fatalf("expected failure, stdout=%q stderr=%q", stdout, stderr)
			}
			envelope := requireFailureEnvelope(t, stdout, stderr)
			if envelope.Error == nil || envelope.Error.Code != "INVALID_INPUT" || !strings.Contains(envelope.Error.Message, "--area is required") {
				t.Fatalf("unexpected envelope %#v", envelope)
			}
			focus := envelope.State["focus"].(map[string]any)["input"].(map[string]any)
			if focus["missing_fields"].([]any)[0] != "area" {
				t.Fatalf("focus = %#v", focus)
			}
		})

		t.Run("req add requires delta", func(t *testing.T) {
			stdout, stderr, exitCode := executeCLIWithInput("@runtime\nFeature: Title\n\n  Scenario: One\n    Given x\n    When y\n    Then z\n", "req", "add", "runtime:session-lifecycle")
			if exitCode == 0 {
				t.Fatalf("expected failure, stdout=%q stderr=%q", stdout, stderr)
			}
			envelope := requireFailureEnvelope(t, stdout, stderr)
			if envelope.Error == nil || envelope.Error.Code != "INVALID_INPUT" || !strings.Contains(envelope.Error.Message, "--delta is required") {
				t.Fatalf("unexpected envelope %#v", envelope)
			}
			focus := envelope.State["focus"].(map[string]any)["input"].(map[string]any)
			if got := strings.Join(stringSliceFromAny(t, focus["missing_fields"]), ","); got != "delta" {
				t.Fatalf("focus = %#v", focus)
			}
		})

		t.Run("req add validates delta format", func(t *testing.T) {
			stdout, stderr, exitCode := executeCLIWithInput("@runtime\nFeature: Title\n\n  Scenario: One\n    Given x\n    When y\n    Then z\n", "req", "add", "runtime:session-lifecycle", "--delta", "bad")
			if exitCode == 0 {
				t.Fatalf("expected failure, stdout=%q stderr=%q", stdout, stderr)
			}
			envelope := requireFailureEnvelope(t, stdout, stderr)
			if envelope.Error == nil || envelope.Error.Code != "INVALID_INPUT" || !strings.Contains(envelope.Error.Message, "delta ID must be D-NNN") {
				t.Fatalf("unexpected envelope %#v", envelope)
			}
			if envelope.State["focus"].(map[string]any)["delta"].(map[string]any)["id"] != "bad" {
				t.Fatalf("focus = %#v", envelope.State["focus"])
			}
		})

		t.Run("rev bump requires checkpoint", func(t *testing.T) {
			stdout, stderr, exitCode := executeCLIWithInput("Summary\n", "rev", "bump", "runtime:session-lifecycle")
			if exitCode == 0 {
				t.Fatalf("expected failure, stdout=%q stderr=%q", stdout, stderr)
			}
			envelope := requireFailureEnvelope(t, stdout, stderr)
			if envelope.Error == nil || envelope.Error.Code != "INVALID_INPUT" || !strings.Contains(envelope.Error.Message, "--checkpoint is required") {
				t.Fatalf("unexpected envelope %#v", envelope)
			}
			focus := envelope.State["focus"].(map[string]any)["rev_bump"].(map[string]any)
			if focus["reason"] != "missing_checkpoint" {
				t.Fatalf("focus = %#v", focus)
			}
		})

		t.Run("sync requires checkpoint", func(t *testing.T) {
			stdout, stderr, exitCode := executeCLI("sync", "runtime:session-lifecycle")
			if exitCode == 0 {
				t.Fatalf("expected failure, stdout=%q stderr=%q", stdout, stderr)
			}
			envelope := requireFailureEnvelope(t, stdout, stderr)
			if envelope.Error == nil || envelope.Error.Code != "INVALID_INPUT" || !strings.Contains(envelope.Error.Message, "--checkpoint is required") {
				t.Fatalf("unexpected envelope %#v", envelope)
			}
			focus := envelope.State["focus"].(map[string]any)
			if _, exists := focus["rev_bump"]; exists {
				t.Fatalf("focus leaked rev_bump = %#v", focus)
			}
			if focus["sync"].(map[string]any)["reason"] != "missing_checkpoint" {
				t.Fatalf("focus = %#v", focus)
			}
		})
	})
}

func TestCommandExecutionFailuresEmitJSONEnvelope(t *testing.T) {
	stdout, stderr, exitCode := executeCLIWithInput("", "charter", "create", "Runtime")
	if exitCode == 0 {
		t.Fatalf("expected invalid command to fail, got stdout=%q stderr=%q", stdout, stderr)
	}

	envelope := requireFailureEnvelope(t, stdout, stderr)
	if envelope.Error == nil {
		t.Fatalf("expected error envelope, got %#v", envelope)
	}
	if envelope.Error.Code != "INVALID_INPUT" {
		t.Fatalf("error code = %q, want INVALID_INPUT", envelope.Error.Code)
	}
	if !strings.Contains(envelope.Error.Message, "charter name must match") {
		t.Fatalf("unexpected error message: %#v", envelope.Error)
	}
	if envelope.State["charter"] != "Runtime" {
		t.Fatalf("expected minimal charter state, got %#v", envelope.State)
	}
	if _, exists := envelope.State["args"]; exists {
		t.Fatalf("expected command-specific failure state, got %#v", envelope.State)
	}
	if envelope.Next == nil {
		t.Fatalf("expected next to be present, got %#v", envelope)
	}
}

func assertAgentFirstSections(t *testing.T, output string) {
	t.Helper()
	for _, section := range []string{"Stdin:", "Example:", "Output:", "Errors:"} {
		if !strings.Contains(output, section) {
			t.Fatalf("expected %q in help output\n%s", section, output)
		}
	}
}

func helpErrorCodes(t *testing.T, output string) []string {
	t.Helper()

	lines := strings.Split(output, "\n")
	start := -1
	for i, line := range lines {
		if line == "Errors:" {
			start = i + 1
			break
		}
	}
	if start == -1 {
		t.Fatalf("Errors section not found in help output\n%s", output)
	}

	codes := make([]string, 0)
	for _, line := range lines[start:] {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			if len(codes) > 0 {
				break
			}
			continue
		}
		if !strings.HasPrefix(line, "  ") {
			if len(codes) > 0 {
				break
			}
			continue
		}
		codes = append(codes, trimmed)
	}
	return codes
}

func executeHelp(t *testing.T, cmd *cobra.Command, args ...string) string {
	t.Helper()
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stdout)
	cmd.SetArgs(args)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute help: %v", err)
	}
	return stdout.String()
}

func mustFindCommand(t *testing.T, cmd *cobra.Command, path ...string) *cobra.Command {
	t.Helper()
	current := cmd
	for _, name := range path {
		next, _, err := current.Find([]string{name})
		if err != nil {
			t.Fatalf("find %q: %v", name, err)
		}
		current = next
	}
	return current
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

type testEnvelope struct {
	State map[string]any `json:"state"`
	Focus map[string]any `json:"focus"`
	Next  testNext       `json:"next"`
	Error *struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

type testNext []any

func (n *testNext) UnmarshalJSON(data []byte) error {
	var sequence []any
	if err := json.Unmarshal(data, &sequence); err == nil {
		*n = sequence
		return nil
	}

	var directive struct {
		Mode    string `json:"mode"`
		Steps   []any  `json:"steps"`
		Options []any  `json:"options"`
	}
	if err := json.Unmarshal(data, &directive); err != nil {
		return err
	}

	switch directive.Mode {
	case "none", "":
		*n = []any{}
	case "sequence":
		*n = append([]any{}, directive.Steps...)
	case "choose_one", "choose_then_sequence":
		*n = append([]any{}, directive.Options...)
	default:
		return fmt.Errorf("unsupported next.mode %q", directive.Mode)
	}
	return nil
}

type testNextMaps []map[string]any

func (n *testNextMaps) UnmarshalJSON(data []byte) error {
	var sequence []map[string]any
	if err := json.Unmarshal(data, &sequence); err == nil {
		*n = sequence
		return nil
	}

	var directive struct {
		Mode    string           `json:"mode"`
		Steps   []map[string]any `json:"steps"`
		Options []map[string]any `json:"options"`
	}
	if err := json.Unmarshal(data, &directive); err != nil {
		return err
	}

	switch directive.Mode {
	case "none", "":
		*n = []map[string]any{}
	case "sequence":
		*n = append([]map[string]any{}, directive.Steps...)
	case "choose_one", "choose_then_sequence":
		*n = append([]map[string]any{}, directive.Options...)
	default:
		return fmt.Errorf("unsupported next.mode %q", directive.Mode)
	}
	return nil
}

func executeCLI(args ...string) (string, string, int) {
	return executeCLIWithInput("", args...)
}

func executeCLIWithInput(input string, args ...string) (string, string, int) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	exitCode := executeWithIO(args, strings.NewReader(input), &stdout, &stderr)
	return stdout.String(), stderr.String(), exitCode
}

func parseEnvelope(t *testing.T, output string) testEnvelope {
	t.Helper()

	var envelope testEnvelope
	if err := json.Unmarshal([]byte(output), &envelope); err != nil {
		t.Fatalf("expected JSON envelope, got %q: %v", output, err)
	}
	if envelope.State == nil {
		t.Fatalf("expected state to be present in %q", output)
	}
	if envelope.Focus == nil {
		t.Fatalf("expected focus to be present in %q", output)
	}
	if envelope.Next == nil {
		t.Fatalf("expected next to be present in %q", output)
	}
	envelope.State["focus"] = envelope.Focus
	return envelope
}

func requireFailureEnvelope(t *testing.T, stdout, stderr string) testEnvelope {
	t.Helper()

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	return parseEnvelope(t, stdout)
}
