package specctl

import "embed"

//go:embed SPEC.md
var ExampleSpec string

//go:embed SPEC_FORMAT.md
var ExampleFormat string

//go:embed .specs/specctl.yaml
var ExampleConfig string

//go:embed .specs/specctl/CHARTER.yaml
var ExampleCharter string

//go:embed .specs/specctl/cli.yaml
var ExampleTracking string

// ExampleFiles returns metadata about the embedded example files.
func ExampleFiles() []map[string]any {
	return []map[string]any{
		{"path": "SPEC.md", "role": "design_document", "lines": countLines(ExampleSpec)},
		{"path": "SPEC_FORMAT.md", "role": "format_template", "lines": countLines(ExampleFormat)},
		{"path": ".specs/specctl.yaml", "role": "config", "lines": countLines(ExampleConfig)},
		{"path": ".specs/specctl/CHARTER.yaml", "role": "charter", "lines": countLines(ExampleCharter)},
		{"path": ".specs/specctl/cli.yaml", "role": "tracking", "lines": countLines(ExampleTracking)},
	}
}

func countLines(s string) int {
	n := 0
	for _, c := range s {
		if c == '\n' {
			n++
		}
	}
	if len(s) > 0 && s[len(s)-1] != '\n' {
		n++
	}
	return n
}

// Ensure embed import is used.
var _ embed.FS
