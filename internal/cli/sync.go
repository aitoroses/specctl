package cli

import (
	"io"
	"strings"

	"github.com/aitoroses/specctl/internal/application"
	"github.com/spf13/cobra"
)

func newSyncCmd() *cobra.Command {
	errorCodes := []string{"INVALID_INPUT", "VALIDATION_FAILED", "SPEC_NOT_FOUND", "CHECKPOINT_UNAVAILABLE"}
	cmd := &cobra.Command{
		Use:   "sync <charter:slug>",
		Short: "Re-anchor checkpoint drift without bumping rev",
		Args:  validateSpecIdentifierArgs(1),
		Long: commandLong(`Re-anchor checkpoint drift without creating a new revision.

Stdin:
  Text. Required one-line summary explaining why the checkpoint is being re-anchored.
  Required flags:
    --checkpoint <commit>

Example:
  echo 'Review confirmed the drift is clarification-only and the checkpoint can move.' | specctl sync runtime:session-lifecycle --checkpoint HEAD

Output:
  JSON { state: context <spec> projection, result: { kind, previous_checkpoint, checkpoint }, next: [] }.`,
			errorCodes...,
		),
		RunE: runSyncCmd,
	}
	annotateHelpErrors(cmd, errorCodes...)
	cmd.Flags().String("checkpoint", "", "Explicit git commit or ref to record")
	return cmd
}

func runSyncCmd(cmd *cobra.Command, args []string) error {
	data, err := io.ReadAll(cmd.InOrStdin())
	if err != nil {
		return err
	}
	checkpoint, _ := cmd.Flags().GetString("checkpoint")

	service, err := application.OpenFromWorkingDir()
	if err != nil {
		return err
	}

	state, result, next, err := service.Sync(application.SyncRequest{
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
