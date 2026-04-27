package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/aitoroses/specctl/internal/application"
	"github.com/aitoroses/specctl/internal/presenter"
)

func TestListTools(t *testing.T) {
	repoRoot := tempSpecRepo(t)
	clientSession := connectTestClient(t, repoRoot)
	defer clientSession.Close()

	tools, err := clientSession.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatalf("ListTools failed: %v", err)
	}

	names := make([]string, 0, len(tools.Tools))
	for _, tool := range tools.Tools {
		names = append(names, tool.Name)
	}
	slices.Sort(names)
	want := []string{
		"specctl_charter_add_spec",
		"specctl_charter_create",
		"specctl_charter_remove_spec",
		"specctl_config",
		"specctl_config_add_prefix",
		"specctl_config_add_tag",
		"specctl_config_remove_prefix",
		"specctl_config_remove_tag",
		"specctl_context",
		"specctl_delta_add",
		"specctl_delta_close",
		"specctl_delta_defer",
		"specctl_delta_rebind_requirements",
		"specctl_delta_resume",
		"specctl_delta_start",
		"specctl_delta_withdraw",
		"specctl_diff",
		"specctl_doc_add",
		"specctl_doc_remove",
		"specctl_init",
		"specctl_requirement_add",
		"specctl_requirement_refresh",
		"specctl_requirement_replace",
		"specctl_requirement_stale",
		"specctl_requirement_verify",
		"specctl_requirement_withdraw",
		"specctl_revision_bump",
		"specctl_spec_create",
		"specctl_sync",
	}
	if strings.Join(names, ",") != strings.Join(want, ",") {
		t.Fatalf("tool names = %v, want %v", names, want)
	}
}

func TestContextMissingCharterAddsMCPHint(t *testing.T) {
	repoRoot := tempSpecRepo(t)
	clientSession := connectTestClient(t, repoRoot)
	defer clientSession.Close()

	envelope := callToolEnvelope(t, clientSession, "specctl_context", map[string]any{
		"target": "runtime:session-lifecycle",
	})

	next := requireMap(t, envelope["next"])
	if next["mode"] != "sequence" {
		t.Fatalf("next.mode = %#v, want sequence", next["mode"])
	}
	steps := requireSlice(t, next["steps"])
	if len(steps) != 2 {
		t.Fatalf("steps = %#v, want create_charter then create_spec", steps)
	}
	first := requireMap(t, steps[0])
	second := requireMap(t, steps[1])
	if first["action"] != "create_charter" {
		t.Fatalf("first.action = %#v, want create_charter", first["action"])
	}
	if second["action"] != "create_spec" {
		t.Fatalf("second.action = %#v, want create_spec", second["action"])
	}
	mcpHint := requireMap(t, first["mcp"])
	if mcpHint["tool"] != "specctl_charter_create" {
		t.Fatalf("step.mcp.tool = %#v, want specctl_charter_create", mcpHint["tool"])
	}
	if requireMap(t, second["mcp"])["tool"] != "specctl_spec_create" {
		t.Fatalf("second.mcp.tool = %#v, want specctl_spec_create", requireMap(t, second["mcp"])["tool"])
	}
}

func TestDeltaAddSurfacesUnsupportedPrerequisite(t *testing.T) {
	repoRoot := charterOnlyRepo(t)
	clientSession := connectTestClient(t, repoRoot)
	defer clientSession.Close()

	callToolEnvelope(t, clientSession, "specctl_spec_create", map[string]any{
		"spec":          "runtime:session-lifecycle",
		"title":         "Session Lifecycle",
		"doc":           "runtime/src/domain/session_execution/SPEC.md",
		"scope":         []string{"runtime/src/domain/session_execution/"},
		"group":         "recovery",
		"order":         20,
		"charter_notes": "Session FSM and cleanup behavior",
	})

	envelope := callToolEnvelope(t, clientSession, "specctl_delta_add", map[string]any{
		"spec":    "runtime:session-lifecycle",
		"intent":  "add",
		"area":    "Heartbeat timeout",
		"current": "Current gap",
		"desired": "Target gap",
		"notes":   "Explicitly tracked",
	})

	next := requireMap(t, envelope["next"])
	if next["mode"] != "sequence" {
		t.Fatalf("next.mode = %#v, want sequence", next["mode"])
	}
	steps := requireSlice(t, next["steps"])
	if len(steps) != 1 {
		t.Fatalf("steps = %#v, want first unsupported blocker only", steps)
	}
	step := requireMap(t, steps[0])
	if step["action"] != "write_spec_section" {
		t.Fatalf("step.action = %#v, want write_spec_section", step["action"])
	}
	mcpHint := requireMap(t, step["mcp"])
	if mcpHint["available"] != false {
		t.Fatalf("step.mcp.available = %#v, want false", mcpHint["available"])
	}
	if mcpHint["reason"] != "unsupported_in_v1" {
		t.Fatalf("step.mcp.reason = %#v, want unsupported_in_v1", mcpHint["reason"])
	}
}

func TestRequirementVerifyHintCarriesSuggestedTestFiles(t *testing.T) {
	hint, ok := mcpHintForAction(map[string]any{
		"action": "verify_requirement",
		"template": map[string]any{
			"argv": []string{
				"specctl",
				"req",
				"verify",
				"runtime:session-lifecycle",
				"REQ-001",
				"--test-file",
				"runtime/tests/domain/test_compensation_cleanup.py",
			},
		},
	})
	if !ok {
		t.Fatalf("expected MCP hint for verify_requirement")
	}
	input := requireMap(t, hint["input"])
	if input["spec"] != "runtime:session-lifecycle" {
		t.Fatalf("input.spec = %#v, want runtime:session-lifecycle", input["spec"])
	}
	files := stringSlice(input["test_files"])
	if len(files) != 1 || files[0] != "runtime/tests/domain/test_compensation_cleanup.py" {
		t.Fatalf("input.test_files = %#v", files)
	}
}

func TestConfigAddTagThenReadShowsRegisteredTag(t *testing.T) {
	repoRoot := tempSpecRepo(t)
	clientSession := connectTestClient(t, repoRoot)
	defer clientSession.Close()

	addEnv := callToolEnvelope(t, clientSession, "specctl_config_add_tag", map[string]any{
		"tag": "webhook",
	})
	if errField, ok := addEnv["error"]; ok && errField != nil {
		t.Fatalf("unexpected error: %#v", errField)
	}
	result := requireMap(t, addEnv["result"])
	if result["kind"] != "config" {
		t.Fatalf("result.kind = %#v, want config", result["kind"])
	}
	if result["tag"] != "webhook" {
		t.Fatalf("result.tag = %#v, want webhook", result["tag"])
	}

	readEnv := callToolEnvelope(t, clientSession, "specctl_config", map[string]any{})
	state := requireMap(t, readEnv["state"])
	tags := stringSlice(state["gherkin_tags"])
	if !slices.Contains(tags, "webhook") {
		t.Fatalf("gherkin_tags = %#v, want to contain webhook", tags)
	}
}

func TestConfigAddTagSemanticReserved(t *testing.T) {
	repoRoot := tempSpecRepo(t)
	clientSession := connectTestClient(t, repoRoot)
	defer clientSession.Close()

	envelope := callToolEnvelope(t, clientSession, "specctl_config_add_tag", map[string]any{
		"tag": "e2e",
	})
	errField := requireMap(t, envelope["error"])
	if errField["code"] != "SEMANTIC_TAG_RESERVED" {
		t.Fatalf("error.code = %#v, want SEMANTIC_TAG_RESERVED", errField["code"])
	}
}

func TestConfigAddTagDuplicateReturnsTagExists(t *testing.T) {
	repoRoot := tempSpecRepo(t)
	clientSession := connectTestClient(t, repoRoot)
	defer clientSession.Close()

	if envelope := callToolEnvelope(t, clientSession, "specctl_config_add_tag", map[string]any{
		"tag": "webhook",
	}); envelope["error"] != nil {
		t.Fatalf("first add failed: %#v", envelope["error"])
	}

	envelope := callToolEnvelope(t, clientSession, "specctl_config_add_tag", map[string]any{
		"tag": "webhook",
	})
	errField := requireMap(t, envelope["error"])
	if errField["code"] != "TAG_EXISTS" {
		t.Fatalf("error.code = %#v, want TAG_EXISTS", errField["code"])
	}
}

func TestConfigAddPrefixInvalidPath(t *testing.T) {
	repoRoot := tempSpecRepo(t)
	clientSession := connectTestClient(t, repoRoot)
	defer clientSession.Close()

	envelope := callToolEnvelope(t, clientSession, "specctl_config_add_prefix", map[string]any{
		"prefix": "does-not-exist/",
	})
	errField := requireMap(t, envelope["error"])
	if errField["code"] != "INVALID_PATH" {
		t.Fatalf("error.code = %#v, want INVALID_PATH", errField["code"])
	}
}

func TestCharterRemoveSpecRemovesMembership(t *testing.T) {
	repoRoot := charterOnlyRepo(t)
	clientSession := connectTestClient(t, repoRoot)
	defer clientSession.Close()

	if envelope := callToolEnvelope(t, clientSession, "specctl_spec_create", map[string]any{
		"spec":          "runtime:session-lifecycle",
		"title":         "Session Lifecycle",
		"doc":           "runtime/src/domain/session_execution/SPEC.md",
		"scope":         []string{"runtime/src/domain/session_execution/"},
		"group":         "recovery",
		"order":         20,
		"charter_notes": "Session FSM and cleanup behavior",
	}); envelope["error"] != nil {
		t.Fatalf("spec_create failed: %#v", envelope["error"])
	}

	envelope := callToolEnvelope(t, clientSession, "specctl_charter_remove_spec", map[string]any{
		"charter": "runtime",
		"slug":    "session-lifecycle",
	})
	if envelope["error"] != nil {
		t.Fatalf("charter_remove_spec failed: %#v", envelope["error"])
	}
	result := requireMap(t, envelope["result"])
	if result["removed_slug"] != "session-lifecycle" {
		t.Fatalf("result.removed_slug = %#v, want session-lifecycle", result["removed_slug"])
	}
}

func TestCharterAddSpecMissingCharter(t *testing.T) {
	repoRoot := tempSpecRepo(t)
	clientSession := connectTestClient(t, repoRoot)
	defer clientSession.Close()

	envelope := callToolEnvelope(t, clientSession, "specctl_charter_add_spec", map[string]any{
		"charter": "runtime",
		"slug":    "session-lifecycle",
		"group":   "recovery",
		"order":   20,
		"notes":   "Session FSM and cleanup behavior",
	})
	errField := requireMap(t, envelope["error"])
	if errField["code"] != "CHARTER_NOT_FOUND" {
		t.Fatalf("error.code = %#v, want CHARTER_NOT_FOUND", errField["code"])
	}
}

func TestCharterAddSpecRoundtripAfterRemove(t *testing.T) {
	repoRoot := charterOnlyRepo(t)
	clientSession := connectTestClient(t, repoRoot)
	defer clientSession.Close()

	if envelope := callToolEnvelope(t, clientSession, "specctl_spec_create", map[string]any{
		"spec":          "runtime:session-lifecycle",
		"title":         "Session Lifecycle",
		"doc":           "runtime/src/domain/session_execution/SPEC.md",
		"scope":         []string{"runtime/src/domain/session_execution/"},
		"group":         "recovery",
		"order":         20,
		"charter_notes": "Session FSM and cleanup behavior",
	}); envelope["error"] != nil {
		t.Fatalf("spec_create failed: %#v", envelope["error"])
	}
	if envelope := callToolEnvelope(t, clientSession, "specctl_charter_remove_spec", map[string]any{
		"charter": "runtime",
		"slug":    "session-lifecycle",
	}); envelope["error"] != nil {
		t.Fatalf("charter_remove_spec failed: %#v", envelope["error"])
	}

	envelope := callToolEnvelope(t, clientSession, "specctl_charter_add_spec", map[string]any{
		"charter": "runtime",
		"slug":    "session-lifecycle",
		"group":   "recovery",
		"order":   20,
		"notes":   "Re-added after remove",
	})
	if envelope["error"] != nil {
		t.Fatalf("charter_add_spec failed: %#v", envelope["error"])
	}
	result := requireMap(t, envelope["result"])
	if result["kind"] != "charter_entry" {
		t.Fatalf("result.kind = %#v, want charter_entry", result["kind"])
	}
}

func connectTestClient(t *testing.T, repoRoot string) *sdk.ClientSession {
	t.Helper()

	var server *Server
	withWorkingDir(t, repoRoot, func() {
		service, err := application.OpenFromWorkingDir()
		if err != nil {
			t.Fatalf("OpenFromWorkingDir: %v", err)
		}
		server = NewServer(service)
	})

	clientTransport, serverTransport := sdk.NewInMemoryTransports()
	serverSession, err := server.server.Connect(context.Background(), serverTransport, nil)
	if err != nil {
		t.Fatalf("server.Connect: %v", err)
	}
	t.Cleanup(func() { serverSession.Close() })

	client := sdk.NewClient(&sdk.Implementation{Name: "test-client", Version: "v1"}, nil)
	clientSession, err := client.Connect(context.Background(), clientTransport, nil)
	if err != nil {
		t.Fatalf("client.Connect: %v", err)
	}
	return clientSession
}

func callToolEnvelope(t *testing.T, clientSession *sdk.ClientSession, name string, args map[string]any) map[string]any {
	t.Helper()

	result, err := clientSession.CallTool(context.Background(), &sdk.CallToolParams{
		Name:      name,
		Arguments: args,
	})
	if err != nil {
		t.Fatalf("CallTool(%s) failed: %v", name, err)
	}
	if len(result.Content) != 1 {
		t.Fatalf("CallTool(%s) content = %#v, want one text block", name, result.Content)
	}
	text, ok := result.Content[0].(*sdk.TextContent)
	if !ok {
		t.Fatalf("CallTool(%s) content[0] = %T, want *TextContent", name, result.Content[0])
	}
	var envelope map[string]any
	if err := json.Unmarshal([]byte(text.Text), &envelope); err != nil {
		t.Fatalf("unmarshal tool envelope: %v\n%s", err, text.Text)
	}
	return envelope
}

func tempSpecRepo(t *testing.T) string {
	t.Helper()

	repoRoot := t.TempDir()
	if err := os.MkdirAll(filepath.Join(repoRoot, ".specs"), 0o755); err != nil {
		t.Fatalf("mkdir specs: %v", err)
	}
	initGitRepoAtDate(t, repoRoot, "2026-03-28T12:00:00Z")
	return repoRoot
}

func charterOnlyRepo(t *testing.T) string {
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
	initGitRepoAtDate(t, repoRoot, "2026-03-28T12:00:00Z")
	return repoRoot
}

func initGitRepoAtDate(t *testing.T, repoRoot, timestamp string) string {
	t.Helper()

	runGitAtDate(t, repoRoot, timestamp, "init")
	runGitAtDate(t, repoRoot, timestamp, "config", "user.name", "Specctl Tests")
	runGitAtDate(t, repoRoot, timestamp, "config", "user.email", "specctl-tests@example.com")
	runGitAtDate(t, repoRoot, timestamp, "add", ".")
	runGitAtDate(t, repoRoot, timestamp, "commit", "--allow-empty", "-m", "fixture")
	return strings.TrimSpace(runGitAtDate(t, repoRoot, timestamp, "rev-parse", "HEAD"))
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

func withWorkingDir(t *testing.T, dir string, fn func()) {
	t.Helper()

	original, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd: %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("Chdir(%s): %v", dir, err)
	}
	defer func() {
		if err := os.Chdir(original); err != nil {
			t.Fatalf("restore cwd: %v", err)
		}
	}()
	fn()
}

func requireMap(t *testing.T, value any) map[string]any {
	t.Helper()
	m, ok := value.(map[string]any)
	if !ok {
		t.Fatalf("value = %#v, want map[string]any", value)
	}
	return m
}

func requireSlice(t *testing.T, value any) []any {
	t.Helper()
	s, ok := value.([]any)
	if !ok {
		t.Fatalf("value = %#v, want []any", value)
	}
	return s
}

func TestWrapHandlerRecoversPanic(t *testing.T) {
	var stderrBuf bytes.Buffer
	restore := SetPanicLogger(&stderrBuf)
	defer restore()

	s := &Server{}
	wrapped := wrapHandler(
		"specctl_test_panic",
		s,
		func(ctx context.Context, req *sdk.CallToolRequest, in struct{}) (*sdk.CallToolResult, any, error) {
			panic("boom")
		},
	)

	result, _, err := wrapped(context.Background(), nil, struct{}{})
	if err != nil {
		t.Fatalf("wrapped handler should swallow panic, got err = %v", err)
	}
	if result == nil || !result.IsError {
		t.Fatalf("expected IsError result, got %#v", result)
	}
	text, ok := result.Content[0].(*sdk.TextContent)
	if !ok {
		t.Fatalf("result content[0] = %T, want *TextContent", result.Content[0])
	}
	var envelope map[string]any
	if jsonErr := json.Unmarshal([]byte(text.Text), &envelope); jsonErr != nil {
		t.Fatalf("unmarshal envelope: %v\n%s", jsonErr, text.Text)
	}
	errField := requireMap(t, envelope["error"])
	if errField["code"] != "INTERNAL_PANIC" {
		t.Fatalf("error.code = %v, want INTERNAL_PANIC", errField["code"])
	}
	next := requireMap(t, envelope["next"])
	if next["mode"] != "sequence" {
		t.Fatalf("next.mode = %v, want sequence", next["mode"])
	}
	steps := requireSlice(t, next["steps"])
	if len(steps) != 1 {
		t.Fatalf("expected 1 step, got %d", len(steps))
	}
	step := requireMap(t, steps[0])
	if step["action"] != "report_issue" {
		t.Fatalf("step.action = %v, want report_issue", step["action"])
	}
	tmpl := requireMap(t, step["template"])
	if tmpl["url"] != presenter.IssueRepoURL {
		t.Fatalf("template.url = %v, want %s", tmpl["url"], presenter.IssueRepoURL)
	}
	if !strings.Contains(stderrBuf.String(), "specctl mcp panic in specctl_test_panic") {
		t.Fatalf("expected stderr panic log, got %q", stderrBuf.String())
	}
	mcpField := requireMap(t, step["mcp"])
	if mcpField["available"] != true {
		t.Fatalf("report_issue mcp.available = %#v, want true (must survive adaptEnvelopeForMCP intact)", mcpField["available"])
	}
	if mcpField["kind"] != "external_link" {
		t.Fatalf("report_issue mcp.kind = %#v, want external_link", mcpField["kind"])
	}
}

func TestWrapHandlerHandlesNilPanic(t *testing.T) {
	var stderrBuf bytes.Buffer
	restore := SetPanicLogger(&stderrBuf)
	defer restore()

	s := &Server{}
	wrapped := wrapHandler(
		"specctl_nil_panic",
		s,
		func(ctx context.Context, req *sdk.CallToolRequest, in struct{}) (*sdk.CallToolResult, any, error) {
			panic(nil)
		},
	)
	result, _, err := wrapped(context.Background(), nil, struct{}{})
	if err != nil {
		t.Fatalf("wrapped should swallow nil panic, got %v", err)
	}
	if result == nil || !result.IsError {
		t.Fatalf("expected error envelope from nil panic, got %#v", result)
	}
	envelope := decodeEnvelope(t, result)
	if requireMap(t, envelope["error"])["code"] != "INTERNAL_PANIC" {
		t.Fatalf("nil panic should still surface as INTERNAL_PANIC, got %#v", envelope["error"])
	}
}

func TestWrapHandlerSurvivesConsecutivePanics(t *testing.T) {
	var stderrBuf bytes.Buffer
	restore := SetPanicLogger(&stderrBuf)
	defer restore()

	s := &Server{}
	count := 0
	wrapped := wrapHandler(
		"specctl_repeat_panic",
		s,
		func(ctx context.Context, req *sdk.CallToolRequest, in struct{}) (*sdk.CallToolResult, any, error) {
			count++
			panic(fmt.Sprintf("boom %d", count))
		},
	)
	for i := 0; i < 5; i++ {
		result, _, err := wrapped(context.Background(), nil, struct{}{})
		if err != nil {
			t.Fatalf("iter %d: err = %v, want nil", i, err)
		}
		if !result.IsError {
			t.Fatalf("iter %d: expected IsError=true", i)
		}
	}
	if count != 5 {
		t.Fatalf("count = %d, want 5", count)
	}
}

func TestWrapHandlerCapsLargeStack(t *testing.T) {
	long := strings.Repeat("Z", maxStackBytes*3)
	truncated := truncateStack(long, maxStackBytes)
	if len(truncated) > maxStackBytes+128 {
		t.Fatalf("truncated stack length = %d, want <= %d", len(truncated), maxStackBytes+128)
	}
	if !strings.Contains(truncated, "stack truncated") {
		t.Fatalf("expected truncation marker in stack, got tail = %q", truncated[len(truncated)-80:])
	}
}

func TestToolErrorAddsIssueHintForUntypedError(t *testing.T) {
	s := &Server{}
	req := &sdk.CallToolRequest{Params: &sdk.CallToolParamsRaw{Name: "specctl_test_unknown"}}

	result, _, err := s.toolError(req, map[string]any{"spec": "x:y"}, errors.New("git failed"))
	if err != nil {
		t.Fatalf("toolError must return nil err, got %v", err)
	}
	if !result.IsError {
		t.Fatal("expected IsError=true")
	}
	envelope := decodeEnvelope(t, result)
	errField := requireMap(t, envelope["error"])
	if errField["code"] != "INTERNAL_ERROR" {
		t.Fatalf("error.code = %v, want INTERNAL_ERROR", errField["code"])
	}
	steps := requireSlice(t, requireMap(t, envelope["next"])["steps"])
	if len(steps) == 0 {
		t.Fatal("expected report_issue step for untyped error")
	}
	step := requireMap(t, steps[0])
	if step["action"] != "report_issue" {
		t.Fatalf("step.action = %v, want report_issue", step["action"])
	}
}

func TestToolErrorKnownFailureHasNoIssueHint(t *testing.T) {
	s := &Server{}
	req := &sdk.CallToolRequest{Params: &sdk.CallToolParamsRaw{Name: "specctl_config_add_tag"}}

	failure := &application.Failure{Code: "TAG_EXISTS", Message: "tag already registered"}
	result, _, _ := s.toolError(req, map[string]any{"tag": "webhook"}, failure)
	envelope := decodeEnvelope(t, result)

	errField := requireMap(t, envelope["error"])
	if errField["code"] != "TAG_EXISTS" {
		t.Fatalf("error.code = %v, want TAG_EXISTS", errField["code"])
	}
	next := requireMap(t, envelope["next"])
	for _, raw := range append(requireSliceOrEmpty(next["steps"]), requireSliceOrEmpty(next["options"])...) {
		step, _ := raw.(map[string]any)
		if step["action"] == "report_issue" {
			t.Fatalf("typed failure should not carry report_issue hint: %#v", step)
		}
	}
}

func TestPanickingHandlerKeepsSessionAlive(t *testing.T) {
	repoRoot := tempSpecRepo(t)
	var server *Server
	withWorkingDir(t, repoRoot, func() {
		service, err := application.OpenFromWorkingDir()
		if err != nil {
			t.Fatalf("OpenFromWorkingDir: %v", err)
		}
		server = NewServer(service)
	})

	var stderrBuf bytes.Buffer
	restore := SetPanicLogger(&stderrBuf)
	defer restore()

	addTool(server, &sdk.Tool{
		Name:        "specctl_panic_probe",
		Description: "Test-only tool that panics on call.",
	}, func(ctx context.Context, req *sdk.CallToolRequest, in struct{}) (*sdk.CallToolResult, any, error) {
		panic("test panic")
	})

	clientTransport, serverTransport := sdk.NewInMemoryTransports()
	serverSession, err := server.server.Connect(context.Background(), serverTransport, nil)
	if err != nil {
		t.Fatalf("server.Connect: %v", err)
	}
	t.Cleanup(func() { serverSession.Close() })

	client := sdk.NewClient(&sdk.Implementation{Name: "test-client", Version: "v1"}, nil)
	clientSession, err := client.Connect(context.Background(), clientTransport, nil)
	if err != nil {
		t.Fatalf("client.Connect: %v", err)
	}
	defer clientSession.Close()

	envelope := callToolEnvelope(t, clientSession, "specctl_panic_probe", map[string]any{})
	errField := requireMap(t, envelope["error"])
	if errField["code"] != "INTERNAL_PANIC" {
		t.Fatalf("error.code = %v, want INTERNAL_PANIC", errField["code"])
	}

	// Session must still be usable after a panic.
	tools, listErr := clientSession.ListTools(context.Background(), nil)
	if listErr != nil {
		t.Fatalf("ListTools after panic failed: %v", listErr)
	}
	if len(tools.Tools) == 0 {
		t.Fatal("ListTools returned no tools after panic")
	}
}

func decodeEnvelope(t *testing.T, result *sdk.CallToolResult) map[string]any {
	t.Helper()
	if len(result.Content) == 0 {
		t.Fatal("CallToolResult has no content")
	}
	text, ok := result.Content[0].(*sdk.TextContent)
	if !ok {
		t.Fatalf("content[0] = %T, want *TextContent", result.Content[0])
	}
	var envelope map[string]any
	if err := json.Unmarshal([]byte(text.Text), &envelope); err != nil {
		t.Fatalf("unmarshal envelope: %v\n%s", err, text.Text)
	}
	return envelope
}

func requireSliceOrEmpty(value any) []any {
	if value == nil {
		return nil
	}
	if slice, ok := value.([]any); ok {
		return slice
	}
	return nil
}
