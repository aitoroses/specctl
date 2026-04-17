package presenter

import (
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
