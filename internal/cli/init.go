package cli

import (
	"github.com/aitoroses/specctl/internal/application"
	"github.com/aitoroses/specctl/internal/presenter"
	"github.com/spf13/cobra"
)

func newInitCmd() *cobra.Command {
	errorCodes := []string{"INVALID_INPUT"}
	cmd := &cobra.Command{
		Use:   "init",
		Short: "Initialize specctl governance in this repository",
		Args:  cobra.NoArgs,
		Long: commandLong(`Initialize specctl in the current repository.

Creates the .specs/ directory and a specctl.yaml config with auto-detected
source prefixes (scans for common source directories that exist on disk).

If .specs/specctl.yaml already exists, returns the current config without
changes (idempotent).

This is the recommended first step for new projects. After init, create a
charter with "specctl charter create <name>".

Stdin:
  This command does not read stdin.

Example:
  specctl init
  specctl init    # idempotent — safe to re-run

Output:
  JSON { state, focus, result, next }.
  result.kind = "init"
  result.created = true if .specs/ was created, false if already existed
  result.source_prefixes_detected = list of prefixes found on disk

Decision criteria:
  Use init as the first specctl command in any new project.
  Use init when specctl MCP server returns NOT_INITIALIZED.`,
			errorCodes...,
		),
		RunE: runInitCmd,
	}
	annotateHelpErrors(cmd, errorCodes...)
	return cmd
}

func runInitCmd(cmd *cobra.Command, _ []string) error {
	result, err := application.Init()
	if err != nil {
		return err
	}

	state := result["state"]
	delete(result, "state")

	next := buildInitNext(result)
	return writeWriteEnvelope(cmd, state, nil, result, presenter.Sequence(next))
}

func buildInitNext(result map[string]any) []any {
	return []any{
		map[string]any{
			"priority":     1,
			"action":       "create_charter",
			"kind":         "run_command",
			"instructions": "Create your first charter to group related specs.",
			"template": map[string]any{
				"argv":         []string{"specctl", "charter", "create", "<charter_name>"},
				"stdin_format": "yaml",
				"stdin_template": "title: <title>\ndescription: <description>\ngroups:\n  - key: <group_key>\n    title: <group_title>\n    order: <group_order>\n",
			},
		},
	}
}
