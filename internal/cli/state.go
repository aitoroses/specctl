package cli

import (
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

func minimalSpecState(target string) map[string]any {
	charter, slug, ok := strings.Cut(target, ":")
	if !ok {
		return map[string]any{}
	}
	return map[string]any{
		"target":        target,
		"tracking_file": filepath.ToSlash(filepath.Join(".specs", charter, slug+".yaml")),
	}
}

func minimalCharterState(charter string) map[string]any {
	return map[string]any{
		"charter":       charter,
		"tracking_file": filepath.ToSlash(filepath.Join(".specs", charter, "CHARTER.yaml")),
	}
}

func inputFocusedState(state map[string]any, input map[string]any) map[string]any {
	if state == nil {
		state = map[string]any{}
	}
	if input == nil {
		return state
	}
	state["focus"] = map[string]any{"input": input}
	return state
}

func minimalContextTargetState(target string) map[string]any {
	target = strings.TrimSpace(target)
	if target == "" {
		return map[string]any{}
	}
	if strings.Contains(target, ":") {
		state := minimalSpecState(target)
		if len(state) > 0 {
			return state
		}
	}
	if isValidCharter(target) {
		return minimalCharterState(target)
	}
	return map[string]any{"target": target}
}

func commandState(cmd *cobra.Command, fallback any) any {
	if len(cmd.Flags().Args()) == 0 {
		return fallback
	}

	target := cmd.Flags().Args()[0]
	switch {
	case strings.Contains(target, ":"):
		return minimalSpecState(target)
	case target != "":
		return minimalCharterState(target)
	default:
		return fallback
	}
}

func minimalCommandState(command string, args []string) map[string]any {
	state := map[string]any{
		"command": command,
	}
	if len(args) > 0 {
		state["args"] = append([]string{}, args...)
	}
	return state
}
