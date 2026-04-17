package application

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"
	"time"
)

var contractFixtureDate = time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)

type contractResponseEnvelope struct {
	State  any                    `json:"state"`
	Focus  any                    `json:"focus"`
	Result any                    `json:"result,omitempty"`
	Next   contractNextDirective  `json:"next"`
	Error  *contractResponseError `json:"error,omitempty"`
}

type contractResponseError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type contractNextDirective struct {
	Mode    string `json:"mode"`
	Steps   []any  `json:"steps,omitempty"`
	Options []any  `json:"options,omitempty"`
}

func mustParseApplicationJSONValue(t *testing.T, raw string) any {
	t.Helper()

	var value any
	if err := json.Unmarshal([]byte(raw), &value); err != nil {
		t.Fatalf("parse json: %v\n%s", err, raw)
	}
	return value
}

func assertJSONShape(t *testing.T, path string, want, got any) {
	t.Helper()

	switch wantValue := want.(type) {
	case map[string]any:
		gotValue, ok := got.(map[string]any)
		if !ok {
			t.Fatalf("path=%s expected=%T actual=%T", path, want, got)
		}

		for key := range wantValue {
			if _, exists := gotValue[key]; !exists {
				t.Fatalf("path=%s.%s expected=%#v actual=<missing>", path, key, wantValue[key])
			}
		}
		for key := range gotValue {
			if _, exists := wantValue[key]; !exists {
				t.Fatalf("path=%s.%s expected=<absent> actual=%#v", path, key, gotValue[key])
			}
		}

		keys := make([]string, 0, len(wantValue))
		for key := range wantValue {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			assertJSONShape(t, path+"."+key, wantValue[key], gotValue[key])
		}
	case []any:
		gotValue, ok := got.([]any)
		if !ok {
			t.Fatalf("path=%s expected=%T actual=%T", path, want, got)
		}
		if len(wantValue) != len(gotValue) {
			t.Fatalf("path=%s expected_len=%d actual_len=%d", path, len(wantValue), len(gotValue))
		}
		for index := range wantValue {
			assertJSONShape(t, fmt.Sprintf("%s[%d]", path, index), wantValue[index], gotValue[index])
		}
	case nil:
		if got != nil {
			t.Fatalf("path=%s expected=null actual=%#v", path, got)
		}
	case string, bool, float64:
		if fmt.Sprintf("%T", want) != fmt.Sprintf("%T", got) {
			t.Fatalf("path=%s expected=%T actual=%T", path, want, got)
		}
		if want != got {
			t.Fatalf("path=%s expected=%#v actual=%#v", path, want, got)
		}
	default:
		t.Fatalf("path=%s unsupported expected type %T", path, want)
	}
}

func assertContractFixture(t *testing.T, output string, placeholders map[string]string) {
	t.Helper()

	_, currentFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve contract fixture directory: runtime.Caller failed")
	}
	contractDir := filepath.Join(filepath.Dir(currentFile), "testdata", "contracts")
	if err := os.MkdirAll(contractDir, 0o755); err != nil {
		t.Fatalf("mkdir contract fixtures: %v", err)
	}

	fixtureName := strings.NewReplacer("/", "__", " ", "_").Replace(t.Name()) + ".json"
	fixturePath := filepath.Join(contractDir, fixtureName)
	if os.Getenv("SPECCTL_UPDATE_CONTRACT") == "1" {
		if err := os.WriteFile(fixturePath, prettyContractJSON(t, replacePlaceholderValues(output, placeholders, false)), 0o644); err != nil {
			t.Fatalf("write contract fixture %s: %v", fixturePath, err)
		}
	}

	expectedRaw, err := os.ReadFile(fixturePath)
	if err != nil {
		t.Fatalf("read contract fixture %s: %v", fixturePath, err)
	}

	want := mustParseApplicationJSONValue(t, replacePlaceholderValues(string(expectedRaw), placeholders, true))
	got := mustParseApplicationJSONValue(t, output)
	assertJSONShape(t, "$", want, got)
}

func replacePlaceholderValues(raw string, placeholders map[string]string, useActualValues bool) string {
	result := raw
	for placeholder, actual := range placeholders {
		if useActualValues {
			result = string(bytes.ReplaceAll([]byte(result), []byte(placeholder), []byte(actual)))
			continue
		}
		result = string(bytes.ReplaceAll([]byte(result), []byte(actual), []byte(placeholder)))
	}
	return result
}

func prettyContractJSON(t *testing.T, raw string) []byte {
	t.Helper()

	var value any
	if err := json.Unmarshal([]byte(raw), &value); err != nil {
		t.Fatalf("pretty contract json: %v\n%s", err, raw)
	}

	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		t.Fatalf("marshal contract json: %v", err)
	}
	return append(data, '\n')
}

func marshalWriteContractOutput(t *testing.T, state, result any, next []any, err error) string {
	t.Helper()

	envelope := contractEnvelopeForError(t, err)
	if envelope == nil {
		responseState, focus := splitContractResponseState(state)
		envelope = &contractResponseEnvelope{
			State:  contractCoalesceState(responseState),
			Focus:  contractCoalesceFocus(focus),
			Result: result,
			Next:   contractNextSequence(next),
		}
	}

	data, marshalErr := json.Marshal(envelope)
	if marshalErr != nil {
		t.Fatalf("marshal contract envelope: %v", marshalErr)
	}
	return string(data)
}

func marshalReadContractOutput(t *testing.T, state any, next []any, err error) string {
	t.Helper()

	envelope := contractEnvelopeForError(t, err)
	if envelope == nil {
		responseState, focus := splitContractResponseState(state)
		envelope = &contractResponseEnvelope{
			State: contractCoalesceState(responseState),
			Focus: contractCoalesceFocus(focus),
			Next:  readContractNextDirective(state, next),
		}
	}

	data, marshalErr := json.Marshal(envelope)
	if marshalErr != nil {
		t.Fatalf("marshal contract envelope: %v", marshalErr)
	}
	return string(data)
}

func readContractNextDirective(state any, next []any) contractNextDirective {
	switch ReadSurfaceNextMode(state, next) {
	case "none":
		return contractNextNone()
	case "choose_one":
		return contractNextChooseOne(next)
	case "choose_then_sequence":
		return contractNextChooseThenSequence(next)
	default:
		return contractNextSequence(next)
	}
}

func marshalSpecWriteContractCall(t *testing.T, invoke func() (SpecProjection, map[string]any, []any, error)) string {
	t.Helper()

	state, result, next, err := invoke()
	return marshalWriteContractOutput(t, state, result, next, err)
}

func assertReadContractFixtureCall(t *testing.T, placeholders map[string]string, invoke func() (any, []any, error)) {
	t.Helper()

	state, next, err := invoke()
	output := marshalReadContractOutput(t, state, next, err)
	assertContractFixture(t, output, placeholders)
}

func contractEnvelopeForError(t *testing.T, err error) *contractResponseEnvelope {
	t.Helper()

	if err == nil {
		return nil
	}

	failure, ok := err.(*Failure)
	if !ok {
		t.Fatalf("error type = %T, want *Failure", err)
	}

	state, focus := splitContractResponseState(failure.State)
	next := contractNextNone()
	if len(failure.Next) > 0 {
		switch failure.NextMode {
		case "choose_one":
			next = contractNextChooseOne(failure.Next)
		case "choose_then_sequence":
			next = contractNextChooseThenSequence(failure.Next)
		default:
			next = contractNextSequence(failure.Next)
		}
	}

	return &contractResponseEnvelope{
		State: contractCoalesceState(state),
		Focus: contractCoalesceFocus(focus),
		Next:  next,
		Error: &contractResponseError{
			Code:    failure.Code,
			Message: failure.Message,
		},
	}
}

func splitContractResponseState(state any) (any, any) {
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

func contractCoalesceState(state any) any {
	if state == nil {
		return map[string]any{}
	}
	return state
}

func contractCoalesceFocus(focus any) any {
	if focus == nil {
		return map[string]any{}
	}
	return focus
}

func contractNextNone() contractNextDirective {
	return contractNextDirective{Mode: "none"}
}

func contractNextSequence(steps []any) contractNextDirective {
	if len(steps) == 0 {
		return contractNextNone()
	}
	return contractNextDirective{Mode: "sequence", Steps: normalizeContractNextActions(steps)}
}

func contractNextChooseOne(options []any) contractNextDirective {
	if len(options) == 0 {
		return contractNextNone()
	}
	return contractNextDirective{Mode: "choose_one", Options: normalizeContractNextActions(options)}
}

func contractNextChooseThenSequence(options []any) contractNextDirective {
	if len(options) == 0 {
		return contractNextNone()
	}
	return contractNextDirective{Mode: "choose_then_sequence", Options: normalizeContractNextActions(options)}
}

func normalizeContractNextActions(next []any) []any {
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

func contractPlaceholders() map[string]string {
	return map[string]string{
		"__TODAY__": contractFixtureDate.Format("2006-01-02"),
	}
}

func newApplicationContractService(repoRoot string) *Service {
	return &Service{
		repoRoot: repoRoot,
		specsDir: filepath.Join(repoRoot, ".specs"),
		now: func() time.Time {
			return contractFixtureDate
		},
	}
}
