package presenter

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/aitoroses/specctl/internal/application"
)

func TestErrorEnvelopeFromInvalidInput(t *testing.T) {
	err := InvalidInput("bad input", map[string]any{
		"slug": "demo",
		"focus": map[string]any{
			"field": "title",
		},
	})

	envelope := ErrorEnvelope(err)
	if envelope.Error == nil {
		t.Fatalf("expected error envelope")
	}
	if envelope.Error.Code != "INVALID_INPUT" {
		t.Fatalf("error code = %q, want INVALID_INPUT", envelope.Error.Code)
	}
	state := envelope.State.(map[string]any)
	if state["slug"] != "demo" {
		t.Fatalf("state.slug = %#v, want demo", state["slug"])
	}
	focus := envelope.Focus.(map[string]any)
	if focus["field"] != "title" {
		t.Fatalf("focus.field = %#v, want title", focus["field"])
	}
}

func TestApplicationErrorMapsFailure(t *testing.T) {
	err := ApplicationError(&application.Failure{
		Code:    "SPEC_NOT_FOUND",
		Message: "missing spec",
		State: map[string]any{
			"target": "runtime:missing",
			"focus": map[string]any{
				"lookup": map[string]any{"reason": "spec_not_found"},
			},
		},
		NextMode: "choose_one",
		Next: []any{
			map[string]any{
				"action":       "create_spec",
				"instructions": "create the spec",
			},
		},
	})

	failure, ok := err.(*Failure)
	if !ok {
		t.Fatalf("ApplicationError returned %T, want *Failure", err)
	}
	if failure.Code != "SPEC_NOT_FOUND" {
		t.Fatalf("code = %q, want SPEC_NOT_FOUND", failure.Code)
	}
	if failure.Next.Mode != "choose_one" {
		t.Fatalf("next mode = %q, want choose_one", failure.Next.Mode)
	}
	envelope := ErrorEnvelope(failure)
	option := envelope.Next.Options[0].(map[string]any)
	if option["why"] != "create the spec" {
		t.Fatalf("next why = %#v, want instructions copied to why", option["why"])
	}
	if _, exists := option["instructions"]; exists {
		t.Fatalf("instructions should be removed after normalization: %#v", option)
	}
}

func TestUnexpectedErrorEnvelopeIncludesIssueHint(t *testing.T) {
	envelope := UnexpectedErrorEnvelope(
		errors.New("git checkout failed: ref not found"),
		UnexpectedContext{Tool: "specctl_revision_bump", Input: map[string]any{"spec": "x:y"}},
	)
	if envelope.Error == nil || envelope.Error.Code != "INTERNAL_ERROR" {
		t.Fatalf("error = %#v, want code INTERNAL_ERROR", envelope.Error)
	}
	if envelope.Next.Mode != "sequence" || len(envelope.Next.Steps) != 1 {
		t.Fatalf("next = %#v, want sequence with 1 step", envelope.Next)
	}
	step := envelope.Next.Steps[0].(map[string]any)
	if step["action"] != "report_issue" {
		t.Fatalf("step.action = %#v, want report_issue", step["action"])
	}
	tmpl := step["template"].(map[string]any)
	if tmpl["url"] != IssueRepoURL {
		t.Fatalf("template.url = %#v, want %s", tmpl["url"], IssueRepoURL)
	}
	body, _ := tmpl["body"].(string)
	if !strings.Contains(body, "specctl_revision_bump") {
		t.Fatalf("body missing tool name: %s", body)
	}
	if !strings.Contains(body, "git checkout failed") {
		t.Fatalf("body missing error message: %s", body)
	}
}

func TestUnexpectedErrorEnvelopeMarksPanicCode(t *testing.T) {
	envelope := UnexpectedErrorEnvelope(
		errors.New("panic: runtime error: invalid memory address"),
		UnexpectedContext{Tool: "specctl_init", PanicValue: "boom", Stack: "goroutine 1: ..."},
	)
	if envelope.Error.Code != "INTERNAL_PANIC" {
		t.Fatalf("error.code = %q, want INTERNAL_PANIC", envelope.Error.Code)
	}
	step := envelope.Next.Steps[0].(map[string]any)
	body := step["template"].(map[string]any)["body"].(string)
	if !strings.Contains(body, "goroutine 1") {
		t.Fatalf("body should embed stack: %s", body)
	}
}

func TestClassifyErrorKeepsTypedFailureCode(t *testing.T) {
	failure := &Failure{Code: "TAG_EXISTS", Message: "tag already registered"}
	envelope := ClassifyError(failure, UnexpectedContext{Tool: "specctl_config_add_tag"})
	if envelope.Error.Code != "TAG_EXISTS" {
		t.Fatalf("error.code = %q, want TAG_EXISTS", envelope.Error.Code)
	}
	if len(envelope.Next.Steps) != 0 || len(envelope.Next.Options) != 0 {
		t.Fatalf("typed failure should not carry report_issue hint: %#v", envelope.Next)
	}
}

func TestClassifyErrorWrapsApplicationFailure(t *testing.T) {
	envelope := ClassifyError(
		&application.Failure{Code: "CHARTER_NOT_FOUND", Message: "missing"},
		UnexpectedContext{Tool: "specctl_charter_add_spec"},
	)
	if envelope.Error.Code != "CHARTER_NOT_FOUND" {
		t.Fatalf("error.code = %q, want CHARTER_NOT_FOUND", envelope.Error.Code)
	}
	for _, raw := range envelope.Next.Steps {
		step, _ := raw.(map[string]any)
		if step["action"] == "report_issue" {
			t.Fatalf("typed application failure should not include report_issue: %#v", step)
		}
	}
}

func TestClassifyErrorAddsHintForUntypedError(t *testing.T) {
	envelope := ClassifyError(errors.New("yaml: unexpected token"), UnexpectedContext{Tool: "specctl_context"})
	if envelope.Error.Code != "INTERNAL_ERROR" {
		t.Fatalf("error.code = %q, want INTERNAL_ERROR", envelope.Error.Code)
	}
	if len(envelope.Next.Steps) == 0 {
		t.Fatalf("expected report_issue hint, got none")
	}
}

func TestRedactedInputJSONTruncatesLongStrings(t *testing.T) {
	long := strings.Repeat("a", 600)
	got := redactedInputJSON(map[string]any{"gherkin": long})
	if !strings.Contains(got, "(+100 chars)") {
		t.Fatalf("expected truncation marker in %s", got)
	}
}

func TestMarshalEnvelopeFallbackOnUnencodableState(t *testing.T) {
	bad := Envelope{
		State: map[string]any{"chan": make(chan int)},
		Focus: map[string]any{},
		Next:  None(),
	}
	data, err := MarshalEnvelope(bad)
	if err == nil {
		t.Fatalf("expected encode error")
	}
	var fallback map[string]any
	if jsonErr := json.Unmarshal(data, &fallback); jsonErr != nil {
		t.Fatalf("fallback bytes not valid JSON: %v", jsonErr)
	}
	errField, ok := fallback["error"].(map[string]any)
	if !ok {
		t.Fatalf("fallback missing error field: %v", fallback)
	}
	if errField["code"] != "ENVELOPE_ENCODE_FAILED" {
		t.Fatalf("fallback code = %v, want ENVELOPE_ENCODE_FAILED", errField["code"])
	}
}

func TestMarshalEnvelopeHappyPath(t *testing.T) {
	env := Envelope{State: map[string]any{"ok": true}, Focus: map[string]any{}, Next: None()}
	data, err := MarshalEnvelope(env)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("not valid JSON: %v", err)
	}
}

func TestClassifyErrorNilReturnsCleanEnvelope(t *testing.T) {
	envelope := ClassifyError(nil, UnexpectedContext{Tool: "specctl_context"})
	if envelope.Error != nil {
		t.Fatalf("classify(nil).Error = %#v, want nil", envelope.Error)
	}
	if envelope.Next.Mode != "none" {
		t.Fatalf("classify(nil).Next.Mode = %q, want none", envelope.Next.Mode)
	}
}

func TestRedactedInputJSONHandlesUnserializableValue(t *testing.T) {
	got := redactedInputJSON(map[string]any{"ch": make(chan int)})
	if !strings.Contains(got, "unserialisable") {
		t.Fatalf("expected unserialisable marker, got %q", got)
	}
}

func TestRedactedInputJSONHandlesNil(t *testing.T) {
	if got := redactedInputJSON(nil); got != "null" {
		t.Fatalf("redact(nil) = %q, want null", got)
	}
}
