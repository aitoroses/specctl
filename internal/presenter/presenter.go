package presenter

import (
	"encoding/json"
	"io"

	"github.com/aitoroses/specctl/internal/application"
)

type Envelope struct {
	State  any       `json:"state"`
	Focus  any       `json:"focus"`
	Result any       `json:"result,omitempty"`
	Next   Directive `json:"next"`
	Error  *Error    `json:"error,omitempty"`
}

type Error struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type Failure struct {
	Code    string
	Message string
	State   any
	Focus   any
	Next    Directive
}

type Directive struct {
	Mode    string `json:"mode"`
	Steps   []any  `json:"steps,omitempty"`
	Options []any  `json:"options,omitempty"`
}

func (f *Failure) Error() string {
	return f.Message
}

func WriteJSON(w io.Writer, envelope Envelope) error {
	return json.NewEncoder(w).Encode(envelope)
}

func ReadEnvelope(state, focus any, next Directive) Envelope {
	return Envelope{
		State: CoalesceState(state),
		Focus: CoalesceFocus(focus),
		Next:  CoalesceDirective(next),
	}
}

func WriteEnvelope(state, focus, result any, next Directive) Envelope {
	return Envelope{
		State:  CoalesceState(state),
		Focus:  CoalesceFocus(focus),
		Result: result,
		Next:   CoalesceDirective(next),
	}
}

func ErrorEnvelope(err error) Envelope {
	envelope := Envelope{
		State: map[string]any{},
		Focus: map[string]any{},
		Next:  None(),
		Error: &Error{
			Code:    "INVALID_INPUT",
			Message: err.Error(),
		},
	}

	if failure, ok := err.(*Failure); ok {
		envelope.Error.Code = failure.Code
		envelope.State = CoalesceState(failure.State)
		envelope.Focus = CoalesceFocus(failure.Focus)
		envelope.Next = CoalesceDirective(failure.Next)
	}

	return envelope
}

func InvalidInput(message string, state any) error {
	normalizedState, focus := SplitStateFocus(state)
	return &Failure{
		Code:    "INVALID_INPUT",
		Message: message,
		State:   normalizedState,
		Focus:   focus,
		Next:    None(),
	}
}

func ApplicationError(err error) error {
	if err == nil {
		return nil
	}
	if failure, ok := err.(*application.Failure); ok {
		state, focus := SplitStateFocus(failure.State)
		next := None()
		if len(failure.Next) > 0 {
			switch failure.NextMode {
			case "choose_one":
				next = ChooseOne(failure.Next)
			case "choose_then_sequence":
				next = ChooseThenSequence(failure.Next)
			default:
				next = Sequence(failure.Next)
			}
		}
		return &Failure{
			Code:    failure.Code,
			Message: failure.Message,
			State:   state,
			Focus:   focus,
			Next:    next,
		}
	}
	return err
}

func CoalesceState(state any) any {
	if state == nil {
		return map[string]any{}
	}
	return state
}

func CoalesceFocus(focus any) any {
	if focus == nil {
		return map[string]any{}
	}
	return focus
}

func CoalesceDirective(next Directive) Directive {
	if next.Mode == "" || next.Mode == "none" {
		return None()
	}
	next.Steps = NormalizeNextActions(next.Steps)
	next.Options = NormalizeNextActions(next.Options)
	return next
}

func NormalizeNextActions(next []any) []any {
	if len(next) == 0 {
		return nil
	}

	normalized := make([]any, 0, len(next))
	for _, raw := range next {
		action, ok := raw.(map[string]any)
		if !ok {
			normalized = append(normalized, raw)
			continue
		}
		cloned := make(map[string]any, len(action)+1)
		for key, value := range action {
			cloned[key] = value
		}
		if why, ok := cloned["instructions"]; ok && cloned["why"] == nil {
			cloned["why"] = why
		}
		delete(cloned, "instructions")
		normalized = append(normalized, cloned)
	}
	return normalized
}

func None() Directive {
	return Directive{Mode: "none"}
}

func Sequence(steps []any) Directive {
	if len(steps) == 0 {
		return None()
	}
	return Directive{Mode: "sequence", Steps: steps}
}

func ChooseOne(options []any) Directive {
	if len(options) == 0 {
		return None()
	}
	return Directive{Mode: "choose_one", Options: options}
}

func ChooseThenSequence(options []any) Directive {
	if len(options) == 0 {
		return None()
	}
	return Directive{Mode: "choose_then_sequence", Options: options}
}

func DirectiveForReadMode(mode string, next []any) Directive {
	switch mode {
	case "none":
		return None()
	case "choose_one":
		return ChooseOne(next)
	case "choose_then_sequence":
		return ChooseThenSequence(next)
	default:
		return Sequence(next)
	}
}

func SplitStateFocus(state any) (any, any) {
	if state == nil {
		return map[string]any{}, map[string]any{}
	}

	raw, err := json.Marshal(state)
	if err != nil {
		return state, map[string]any{}
	}

	var decoded any
	if err := json.Unmarshal(raw, &decoded); err != nil {
		return state, map[string]any{}
	}

	object, ok := decoded.(map[string]any)
	if !ok {
		return decoded, map[string]any{}
	}

	focus, exists := object["focus"]
	if exists {
		delete(object, "focus")
	} else {
		focus = map[string]any{}
	}
	if focus == nil {
		focus = map[string]any{}
	}
	return object, focus
}
