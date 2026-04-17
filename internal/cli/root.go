package cli

import "github.com/spf13/cobra"

func NewRootCmd() *cobra.Command {
	errorCodes := []string{"INVALID_INPUT"}
	rootCmd := &cobra.Command{
		Use:           "specctl",
		Short:         "specctl v2 specification engine",
		SilenceErrors: true,
		SilenceUsage:  true,
		Long: commandLong(`Agent-facing YAML specification engine.

First time? Run "specctl example" to see a complete governed specification
embedded in the binary — the tool's own spec, format template, config,
charter, and tracking file. No setup required.

Stdin:
  Root command does not read stdin.

Example:
  specctl example                                  # see a full governed spec
  specctl context runtime:session-lifecycle         # read spec state

Output:
  JSON only. Read commands return {state,focus,next}. Write commands return {state,focus,result,next} or {error,state,focus,next}.`,
			errorCodes...,
		),
	}

	rootCmd.CompletionOptions.DisableDefaultCmd = true
	rootCmd.SetHelpCommand(&cobra.Command{Hidden: true})
	annotateHelpErrors(rootCmd, errorCodes...)
	rootCmd.AddCommand(
		newMcpCmd(),
		newInitCmd(),
		newContextCmd(),
		newDiffCmd(),
		newHookCmd(),
		newSpecCmd(),
		newDeltaCmd(),
		newReqCmd(),
		newRevCmd(),
		newSyncCmd(),
		newCharterCmd(),
		newConfigCmd(),
		newExampleCmd(),
		newDashboardCmd(),
	)

	return rootCmd
}
