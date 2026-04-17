package cli

import (
	"github.com/aitoroses/specctl/internal/application"
	"github.com/spf13/cobra"
)

func newContextCmd() *cobra.Command {
	errorCodes := []string{"INVALID_INPUT"}
	cmd := &cobra.Command{
		Use:   "context [target]",
		Short: "Return registry, charter, spec, or ownership context",
		Args:  validateContextArgs,
		Long: commandLong(`Primary read command for registry, charter, spec, or file ownership context.

Stdin:
  This command does not read stdin.
  Flags:
    --file <path>  Resolve ownership for one repo-relative path instead of a target argument.

Example:
  specctl context runtime:session-lifecycle
  specctl context runtime
  specctl context --file runtime/src/domain/session_execution/services.py

Output:
  JSON { state: context projection, next: [...] }.`,
			errorCodes...,
		),
		RunE: runContextCmd,
	}
	cmd.Flags().String("file", "", "Resolve ownership for one repo-relative path")
	annotateHelpErrors(cmd, errorCodes...)
	return cmd
}

func runContextCmd(cmd *cobra.Command, args []string) error {
	service, err := application.OpenFromWorkingDir()
	if err != nil {
		return err
	}

	fileTarget, _ := cmd.Flags().GetString("file")
	target := ""
	if len(args) == 1 {
		target = args[0]
	}

	state, next, err := service.ReadContext(target, fileTarget)
	if err != nil {
		return applicationError(err)
	}
	responseState, focus := splitResponseState(state)
	return writeReadEnvelope(cmd, responseState, focus, contextNextDirective(state, next))
}
