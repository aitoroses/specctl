package presenter

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/aitoroses/specctl/internal/application"
)

// IssueRepoURL is the destination for unexpected-error reports.
const IssueRepoURL = "https://github.com/aitoroses/specctl/issues/new"

// BuildVersion is overridable at link time (-ldflags "-X ...BuildVersion=v1.2.3").
var BuildVersion = "dev"

const (
	codeInternalError = "INTERNAL_ERROR"
	codeInternalPanic = "INTERNAL_PANIC"
	codeMarshalFailed = "ENVELOPE_ENCODE_FAILED"
)

// UnexpectedContext carries the call-site information bundled into a
// report_issue hint so the agent can paste a complete bug report.
type UnexpectedContext struct {
	Tool       string
	Input      any
	PanicValue any
	Stack      string
}

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

// UnexpectedErrorEnvelope produces an envelope for errors that look like
// bugs in specctl rather than user input problems. It embeds a report_issue
// step so MCP clients (or CLI consumers) can prompt the user to file a bug
// with the captured tool/input/error context.
func UnexpectedErrorEnvelope(err error, ctx UnexpectedContext) Envelope {
	code := codeInternalError
	if ctx.PanicValue != nil {
		code = codeInternalPanic
	}

	tool := ctx.Tool
	if tool == "" {
		tool = "specctl"
	}

	message := err.Error()
	if message == "" {
		message = "unexpected error"
	}
	summary := fmt.Sprintf("unexpected error in %s: %s", tool, message)

	step := map[string]any{
		"action": "report_issue",
		"kind":   "report",
		"why":    "This looks like a bug in specctl. Open an issue with the details below so it can be fixed.",
		"template": map[string]any{
			"url":   IssueRepoURL,
			"title": fmt.Sprintf("[bug] %s: %s", tool, truncate(message, 80)),
			"body":  buildIssueBody(tool, ctx.Input, code, message, ctx.Stack),
		},
	}

	return Envelope{
		State: map[string]any{},
		Focus: map[string]any{},
		Next:  Sequence([]any{step}),
		Error: &Error{
			Code:    code,
			Message: summary,
		},
	}
}

// ClassifyError returns an envelope for any error coming out of the
// application layer. *Failure values keep their tipified codes; anything
// else is treated as unexpected and gets the report_issue hint.
func ClassifyError(err error, ctx UnexpectedContext) Envelope {
	if err == nil {
		return Envelope{State: map[string]any{}, Focus: map[string]any{}, Next: None()}
	}
	classified := ApplicationError(err)
	if _, ok := classified.(*Failure); ok {
		return ErrorEnvelope(classified)
	}
	return UnexpectedErrorEnvelope(classified, ctx)
}

func buildIssueBody(tool string, input any, code, message, stack string) string {
	stackBlock := "n/a"
	if strings.TrimSpace(stack) != "" {
		stackBlock = "```\n" + stack + "\n```"
	}
	inputJSON := redactedInputJSON(input)
	return fmt.Sprintf(`## What I was trying to do
<describe the goal here>

## Tool / args
- tool: %s
- input:
%s

## Error
- code: %s
- message: %s

## Stack (if panic)
%s

## Environment
- specctl version: %s
`,
		tool,
		"```json\n"+inputJSON+"\n```",
		code,
		message,
		stackBlock,
		BuildVersion,
	)
}

// redactedInputJSON serialises the input for the issue body and truncates
// long strings so we never paste a 50KB Gherkin block (or a leaked secret)
// into a public issue.
func redactedInputJSON(input any) string {
	if input == nil {
		return "null"
	}
	raw, err := json.Marshal(input)
	if err != nil {
		return fmt.Sprintf("(unserialisable: %v)", err)
	}
	var decoded any
	if err := json.Unmarshal(raw, &decoded); err != nil {
		return string(raw)
	}
	redacted := redactValue(decoded)
	out, err := json.MarshalIndent(redacted, "", "  ")
	if err != nil {
		return string(raw)
	}
	return string(out)
}

func redactValue(value any) any {
	switch typed := value.(type) {
	case string:
		return truncate(typed, 500)
	case []any:
		out := make([]any, len(typed))
		for i, item := range typed {
			out[i] = redactValue(item)
		}
		return out
	case map[string]any:
		out := make(map[string]any, len(typed))
		for key, item := range typed {
			out[key] = redactValue(item)
		}
		return out
	default:
		return typed
	}
}

func truncate(s string, max int) string {
	if max <= 0 || len(s) <= max {
		return s
	}
	return s[:max] + fmt.Sprintf("…(+%d chars)", len(s)-max)
}

// MarshalEnvelope encodes an envelope to JSON, returning a fixed fallback
// payload (and a non-nil error) when encoding fails. Callers should always
// be able to write the returned bytes to the wire, even on failure.
func MarshalEnvelope(envelope Envelope) ([]byte, error) {
	data, err := json.Marshal(envelope)
	if err != nil {
		fallback := Envelope{
			State: map[string]any{},
			Focus: map[string]any{},
			Next:  None(),
			Error: &Error{
				Code:    codeMarshalFailed,
				Message: "failed to encode response: " + err.Error(),
			},
		}
		fallbackBytes, marshalErr := json.Marshal(fallback)
		if marshalErr != nil {
			return []byte(`{"state":{},"focus":{},"next":{"mode":"none"},"error":{"code":"ENVELOPE_ENCODE_FAILED","message":"failed to encode response"}}`), err
		}
		return fallbackBytes, err
	}
	return data, nil
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
