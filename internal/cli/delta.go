package cli

import (
	"strings"

	"github.com/aitoroses/specctl/internal/application"
	"github.com/aitoroses/specctl/internal/domain"
	"github.com/spf13/cobra"
)

func newDeltaCmd() *cobra.Command {
	errorCodes := []string{"INVALID_INPUT"}
	cmd := &cobra.Command{
		Use:   "delta",
		Short: "Manage tracked deltas",
		Long: commandLong(`Delta command family.

Stdin:
  Only delta add reads stdin. Other delta commands do not read stdin.

Example:
  cat <<'EOF' | specctl delta add runtime:runtime-api-contract --intent add --area "Authentication handshake"
  current: No heartbeat mechanism exists
  target: Add 30s heartbeat with 60s timeout guard
  notes: Observed in production after stalled active sessions
  EOF

Output:
  JSON only. Write commands return {state,result,next} or {error,state,next}.`,
			errorCodes...,
		),
	}
	annotateHelpErrors(cmd, errorCodes...)
	cmd.AddCommand(
		newDeltaAddCmd(),
		newDeltaStartCmd(),
		newDeltaDeferCmd(),
		newDeltaResumeCmd(),
		newDeltaCloseCmd(),
	)
	return cmd
}

func newDeltaAddCmd() *cobra.Command {
	errorCodes := []string{"INVALID_INPUT", "VALIDATION_FAILED", "SPEC_NOT_FOUND"}
	cmd := &cobra.Command{
		Use:   "add <charter:slug>",
		Short: "Add a tracked delta",
		Args:  validateSpecIdentifierArgs(1),
		Long: commandLong(`Add a delta and immediately recompute derived spec status.

Stdin:
  YAML object:
    current: string, required
    target:  string, required
    notes:   string, required
    affects_requirements: list, required only when --intent is change, remove, or repair
  Required flags:
    --intent <add|change|remove|repair>
    --area <text>

Example:
  cat <<'EOF' | specctl delta add runtime:runtime-api-contract --intent add --area "Authentication handshake"
  current: No heartbeat mechanism exists
  target: Add 30s heartbeat with 60s timeout guard
  notes: Observed in production after stalled active sessions
  EOF

Output:
  JSON { state: context <spec> projection, result: { kind, delta, allocation }, next: [...] }.`,
			errorCodes...,
		),
		RunE: runDeltaAddCmd,
	}
	annotateHelpErrors(cmd, errorCodes...)
	cmd.Flags().String("intent", "", "Delta intent: add|change|remove|repair")
	cmd.Flags().String("area", "", "Human-readable gap area")
	return cmd
}

func newDeltaStartCmd() *cobra.Command {
	return newDeltaTransitionCmd(
		"start <charter:slug> <delta-id>",
		"Move open -> in-progress",
		"specctl delta start runtime:session-lifecycle D-008",
		[]string{"INVALID_INPUT", "VALIDATION_FAILED", "SPEC_NOT_FOUND", "DELTA_NOT_FOUND", "DELTA_INVALID_STATE"},
		"delta start",
	)
}

func newDeltaDeferCmd() *cobra.Command {
	return newDeltaTransitionCmd(
		"defer <charter:slug> <delta-id>",
		"Move open|in-progress -> deferred",
		"specctl delta defer runtime:session-lifecycle D-008",
		[]string{"INVALID_INPUT", "VALIDATION_FAILED", "SPEC_NOT_FOUND", "DELTA_NOT_FOUND", "DELTA_INVALID_STATE"},
		"delta defer",
	)
}

func newDeltaResumeCmd() *cobra.Command {
	return newDeltaTransitionCmd(
		"resume <charter:slug> <delta-id>",
		"Move deferred -> open",
		"specctl delta resume runtime:session-lifecycle D-008",
		[]string{"INVALID_INPUT", "VALIDATION_FAILED", "SPEC_NOT_FOUND", "DELTA_NOT_FOUND", "DELTA_INVALID_STATE"},
		"delta resume",
	)
}

func newDeltaCloseCmd() *cobra.Command {
	return newDeltaTransitionCmd(
		"close <charter:slug> <delta-id>",
		"Move open|in-progress -> closed when every tracing requirement is verified",
		"specctl delta close runtime:session-lifecycle D-008",
		[]string{"INVALID_INPUT", "VALIDATION_FAILED", "SPEC_NOT_FOUND", "DELTA_NOT_FOUND", "DELTA_INVALID_STATE", "DELTA_UPDATES_UNRESOLVED", "UNVERIFIED_REQUIREMENTS", "REQUIREMENT_MATCH_BLOCKING"},
		"delta close",
	)
}

func newDeltaTransitionCmd(use, short, example string, errorCodes []string, command string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   use,
		Short: short,
		Args:  validateDeltaCommandArgs,
		Long: commandLong(short+`

Transition one existing delta.

Stdin:
  This command does not read stdin.

Example:
  `+example+`

Output:
  JSON { state: context <spec> projection, result: { kind, delta }, next: [...] }.`,
			errorCodes...,
		),
		RunE: runDeltaTransitionCmd(command),
	}
	annotateHelpErrors(cmd, errorCodes...)
	return cmd
}

type deltaAddRequest struct {
	Current             string   `yaml:"current"`
	Target              string   `yaml:"target"`
	Notes               string   `yaml:"notes"`
	AffectsRequirements []string `yaml:"affects_requirements"`
}

func runDeltaAddCmd(cmd *cobra.Command, args []string) error {
	body, present, err := readStdinYAML[deltaAddRequest](cmd)
	if err != nil {
		return err
	}

	area, _ := cmd.Flags().GetString("area")
	intent, _ := cmd.Flags().GetString("intent")
	service, err := application.OpenFromWorkingDir()
	if err != nil {
		return err
	}

	state, result, next, err := service.AddDelta(application.DeltaAddRequest{
		Target:              args[0],
		Intent:              domain.DeltaIntent(strings.TrimSpace(intent)),
		Area:                strings.TrimSpace(area),
		Current:             body.Current,
		CurrentPresent:      present["current"],
		Targets:             body.Target,
		TargetPresent:       present["target"],
		Notes:               body.Notes,
		NotesPresent:        present["notes"],
		AffectsRequirements: body.AffectsRequirements,
	})
	if err != nil {
		return applicationError(err)
	}
	responseState, focus := splitResponseState(state)
	return writeWriteEnvelope(cmd, responseState, focus, result, nextSequence(next))
}

func runDeltaTransitionCmd(command string) func(*cobra.Command, []string) error {
	return func(cmd *cobra.Command, args []string) error {
		service, err := application.OpenFromWorkingDir()
		if err != nil {
			return err
		}

		request := application.DeltaTransitionRequest{
			Target:  args[0],
			DeltaID: args[1],
		}

		var state any
		var result any
		var next []any
		switch command {
		case "delta start":
			state, result, next, err = service.StartDelta(request)
		case "delta defer":
			state, result, next, err = service.DeferDelta(request)
		case "delta resume":
			state, result, next, err = service.ResumeDelta(request)
		case "delta close":
			state, result, next, err = service.CloseDelta(request)
		default:
			return invalidInputError("unknown delta transition", minimalSpecState(args[0]))
		}
		if err != nil {
			return applicationError(err)
		}
		responseState, focus := splitResponseState(state)
		return writeWriteEnvelope(cmd, responseState, focus, result, nextSequence(next))
	}
}
