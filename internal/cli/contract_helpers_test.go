package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"testing"
	"time"
)

func mustParseJSONValue(t *testing.T, raw string) any {
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

	if placeholders == nil {
		placeholders = map[string]string{}
	}
	placeholders["__TODAY__"] = time.Now().UTC().Format("2006-01-02")

	_, currentFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve contract fixture directory: runtime.Caller failed")
	}
	contractDir := filepath.Join(filepath.Dir(currentFile), "testdata", "contracts")
	if err := os.MkdirAll(contractDir, 0o755); err != nil {
		t.Fatalf("mkdir contract fixtures: %v", err)
	}

	fixturePath := filepath.Join(contractDir, t.Name()+".json")
	if os.Getenv("SPECCTL_UPDATE_CONTRACT") == "1" {
		if err := os.WriteFile(fixturePath, prettyContractJSON(t, replacePlaceholderValues(output, placeholders, false)), 0o644); err != nil {
			t.Fatalf("write contract fixture %s: %v", fixturePath, err)
		}
	}

	expectedRaw, err := os.ReadFile(fixturePath)
	if err != nil {
		t.Fatalf("read contract fixture %s: %v", fixturePath, err)
	}

	want := mustParseJSONValue(t, replacePlaceholderValues(string(expectedRaw), placeholders, true))
	got := mustParseJSONValue(t, output)
	assertJSONShape(t, "$", want, got)
}

func replacePlaceholderValues(raw string, placeholders map[string]string, useActualValues bool) string {
	result := raw
	for placeholder, actual := range placeholders {
		if useActualValues {
			result = bytes.NewBufferString(result).String()
			result = string(bytes.ReplaceAll([]byte(result), []byte(placeholder), []byte(actual)))
			continue
		}
		result = bytes.NewBufferString(result).String()
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
