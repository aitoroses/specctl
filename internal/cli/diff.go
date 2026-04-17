package cli

import (
	"github.com/aitoroses/specctl/internal/application"
	"github.com/spf13/cobra"
)

func newDiffCmd() *cobra.Command {
	errorCodes := []string{"INVALID_INPUT", "SPEC_NOT_FOUND", "CHARTER_NOT_FOUND"}
	cmd := &cobra.Command{
		Use:   "diff [target]",
		Short: "Return a semantic diff against the stored checkpoint",
		Args:  validateDiffArgs,
		Long: commandLong(`Structured semantic diff for one spec or one charter.

Stdin:
  This command does not read stdin.
  Flags:
    --charter <charter>  Diff every spec in one charter.

Example:
  specctl diff runtime:session-lifecycle
  specctl diff --charter runtime

Output:
  JSON { state: diff projection, next: [...] }.`,
			errorCodes...,
		),
		RunE: runDiffCmd,
	}
	cmd.Flags().String("charter", "", "Diff every spec in one charter")
	annotateHelpErrors(cmd, errorCodes...)
	return cmd
}

func runDiffCmd(cmd *cobra.Command, args []string) error {
	service, err := application.OpenFromWorkingDir()
	if err != nil {
		return err
	}

	charterTarget, _ := cmd.Flags().GetString("charter")
	target := ""
	if len(args) == 1 {
		target = args[0]
	}

	state, next, err := service.ReadDiff(target, charterTarget)
	if err != nil {
		return applicationError(err)
	}
	responseState, focus := splitResponseState(state)
	mode := diffNextDirective(state, next)
	if charterTarget != "" {
		mode = nextNone()
	}
	return writeReadEnvelope(cmd, responseState, focus, mode)
}
