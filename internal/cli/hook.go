package cli

import (
	"io"

	"github.com/aitoroses/specctl/internal/application"
	"github.com/spf13/cobra"
)

func newHookCmd() *cobra.Command {
	errorCodes := []string{"INVALID_INPUT"}
	cmd := &cobra.Command{
		Use:   "hook",
		Short: "Classify staged paths for informational hook reporting",
		Args:  validateNoArgsReadCommand("hook"),
		Long: commandLong(`Informational hook integration for staged file paths.

Stdin:
  Text lines. Each line is one staged repo-relative path.

Example:
  cat <<'EOF' | specctl hook
  runtime/src/domain/session_execution/services.py
  .specs/runtime/session-lifecycle.yaml
  EOF

Output:
  JSON { state: hook projection, next: [...] }.`,
			errorCodes...,
		),
		RunE: runHookCmd,
	}
	annotateHelpErrors(cmd, errorCodes...)
	return cmd
}

func runHookCmd(cmd *cobra.Command, _ []string) error {
	data, err := io.ReadAll(cmd.InOrStdin())
	if err != nil {
		return err
	}

	service, err := application.OpenFromWorkingDir()
	if err != nil {
		return err
	}
	state, err := service.ReadHook(string(data))
	if err != nil {
		return err
	}
	responseState, focus := splitResponseState(state)
	return writeReadEnvelope(cmd, responseState, focus, nextNone())
}
