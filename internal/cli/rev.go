package cli

import (
	"io"
	"strings"

	"github.com/aitoroses/specctl/internal/application"
	"github.com/spf13/cobra"
)

func newRevCmd() *cobra.Command {
	errorCodes := []string{"INVALID_INPUT"}
	cmd := &cobra.Command{
		Use:   "rev",
		Short: "Manage revision checkpoints",
		Long: commandLong(`Revision command family.

Stdin:
  rev bump reads the changelog summary from stdin.

Example:
  echo 'Closed the compensation cleanup work and synced the design doc.' | specctl rev bump runtime:session-lifecycle --checkpoint HEAD

Output:
  JSON only. Write commands return {state,result,next} or {error,state,next}.`,
			errorCodes...,
		),
	}
	annotateHelpErrors(cmd, errorCodes...)
	cmd.AddCommand(newRevBumpCmd())
	return cmd
}

func newRevBumpCmd() *cobra.Command {
	errorCodes := []string{"INVALID_INPUT", "VALIDATION_FAILED", "SPEC_NOT_FOUND", "CHECKPOINT_UNAVAILABLE"}
	cmd := &cobra.Command{
		Use:   "bump <charter:slug>",
		Short: "Advance rev, checkpoint, and changelog",
		Args:  validateSpecIdentifierArgs(1),
		Long: commandLong(`Bump rev, record an explicit checkpoint commit, and append one changelog entry.

Stdin:
  Text. One changelog summary paragraph.
  Required flags:
    --checkpoint <commit>

Example:
  echo 'Closed the compensation cleanup work and synced the design doc.' | specctl rev bump runtime:session-lifecycle --checkpoint HEAD

Output:
  JSON { state: context <spec> projection, result: { kind, previous_rev, rev, previous_checkpoint, checkpoint, changelog_entry }, next: [...] }.`,
			errorCodes...,
		),
		RunE: runRevBumpCmd,
	}
	annotateHelpErrors(cmd, errorCodes...)
	cmd.Flags().String("checkpoint", "", "Explicit git commit or ref to record")
	return cmd
}

func runRevBumpCmd(cmd *cobra.Command, args []string) error {
	data, err := io.ReadAll(cmd.InOrStdin())
	if err != nil {
		return err
	}
	checkpoint, _ := cmd.Flags().GetString("checkpoint")

	service, err := application.OpenFromWorkingDir()
	if err != nil {
		return err
	}

	state, result, next, err := service.BumpRevision(application.RevisionBumpRequest{
		Target:     args[0],
		Checkpoint: checkpoint,
		Summary:    strings.TrimRight(string(data), " \t\r\n"),
	})
	if err != nil {
		return applicationError(err)
	}
	responseState, focus := splitResponseState(state)
	return writeWriteEnvelope(cmd, responseState, focus, result, nextSequence(next))
}
