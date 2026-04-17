package application

import (
	"github.com/aitoroses/specctl/internal/infrastructure"
)

// Init bootstraps specctl governance in the current working directory.
// Delegates filesystem operations to infrastructure.InitWorkspace.
func Init() (map[string]any, error) {
	result, err := infrastructure.InitWorkspace()
	if err != nil {
		return nil, err
	}

	return map[string]any{
		"state": map[string]any{
			"initialized":     true,
			"specs_dir":       ".specs",
			"config_path":     ".specs/specctl.yaml",
			"source_prefixes": result.Config.SourcePrefixes,
			"gherkin_tags":    result.Config.GherkinTags,
		},
		"kind":                     "init",
		"created":                  result.Created,
		"source_prefixes_detected": result.DetectedPrefixes,
	}, nil
}
