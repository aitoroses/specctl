package cli

import (
	"io"
	"strings"

	"github.com/aitoroses/specctl/internal/application"
	"github.com/spf13/cobra"
)

func newReqCmd() *cobra.Command {
	errorCodes := []string{"INVALID_INPUT"}
	cmd := &cobra.Command{
		Use:   "req",
		Short: "Manage tracked requirements",
		Long: commandLong(`Requirement command family.

Stdin:
  req add, req replace, and req refresh read requirement-level Gherkin from stdin.
  req withdraw, req stale, and req verify do not read stdin.

Example:
  cat <<'EOF' | specctl req add runtime:session-lifecycle --delta D-008
  @runtime @e2e
  Feature: Compensation stage 4 failure cleanup

    Scenario: Cleanup runs after stage 4 failure
      Given stage 4 fails during compensation
      When recovery completes
      Then cleanup steps run in documented order
  EOF

	Output:
  JSON only. Write commands return {state,result,next} or {error,state,next}.`,
			errorCodes...,
		),
	}
	annotateHelpErrors(cmd, errorCodes...)
	cmd.AddCommand(newReqAddCmd(), newReqReplaceCmd(), newReqWithdrawCmd(), newReqStaleCmd(), newReqRefreshCmd(), newReqVerifyCmd())
	return cmd
}

func newReqAddCmd() *cobra.Command {
	errorCodes := []string{"INVALID_INPUT", "VALIDATION_FAILED", "SPEC_NOT_FOUND", "DELTA_NOT_FOUND", "INVALID_GHERKIN", "INVALID_GHERKIN_TAG"}
	cmd := &cobra.Command{
		Use:   "add <charter:slug>",
		Short: "Add a tracked requirement from Gherkin",
		Args:  validateSpecIdentifierArgs(1),
		Long: commandLong(`Add one requirement. Title and tags are derived from the Gherkin payload.

A requirement is an observable behavior that the system must exhibit,
written from the external surface. Each requirement should be testable:
the agent implements the behavior, then verifies it with test files.
Scenarios describing the requirement belong in the spec's SPEC.md block
so that specctl can match them during verification.

Stdin:
  Text. Requirement-level Gherkin block with tag lines plus one Feature line.
  Write Gherkin from the external surface — describe what a user or caller
  observes, not internal implementation details.
  Required flags:
    --delta <delta-id>

Example:
  cat <<'EOF' | specctl req add runtime:session-lifecycle --delta D-008
  @runtime @e2e
  Feature: Compensation stage 4 failure cleanup
  EOF

Output:
  JSON { state: context <spec> projection, result: { kind, requirement, allocation }, next: [...] }.`,
			errorCodes...,
		),
		RunE: runReqAddCmd,
	}
	annotateHelpErrors(cmd, errorCodes...)
	cmd.Flags().String("delta", "", "Delta ID that introduced this requirement")
	return cmd
}

func newReqReplaceCmd() *cobra.Command {
	errorCodes := []string{"INVALID_INPUT", "VALIDATION_FAILED", "SPEC_NOT_FOUND", "REQUIREMENT_NOT_FOUND", "DELTA_NOT_FOUND", "INVALID_GHERKIN", "INVALID_GHERKIN_TAG", "REQUIREMENT_NOT_IN_SPEC", "REQUIREMENT_DUPLICATE_IN_SPEC"}
	cmd := &cobra.Command{
		Use:   "replace <charter:slug> <requirement-id>",
		Short: "Replace one tracked requirement with a new active requirement",
		Args:  validateRequirementVerifyArgs,
		Long: commandLong(`Replace one active requirement with a new active requirement.

The old requirement is superseded and the new one becomes active.
Write Gherkin from the external surface — describe observable behavior,
not internal details. The replacement requirement must be implemented
and verified with test files.

Stdin:
  Requirement-level Gherkin block for the replacement requirement.
  Required flags:
    --delta <delta-id>`,
			errorCodes...,
		),
		RunE: runReqReplaceCmd,
	}
	annotateHelpErrors(cmd, errorCodes...)
	cmd.Flags().String("delta", "", "Delta ID that requires replacement")
	return cmd
}

func newReqWithdrawCmd() *cobra.Command {
	errorCodes := []string{"INVALID_INPUT", "VALIDATION_FAILED", "SPEC_NOT_FOUND", "REQUIREMENT_NOT_FOUND", "DELTA_NOT_FOUND"}
	cmd := &cobra.Command{
		Use:   "withdraw <charter:slug> <requirement-id>",
		Short: "Withdraw one active requirement",
		Args:  validateRequirementVerifyArgs,
		Long: commandLong(`Withdraw one active requirement. The requirement is marked withdrawn
and no longer needs implementation or verification. Use this when a behavior
is intentionally removed from the system. Requires --delta <delta-id>.`, errorCodes...),
		RunE:  runReqWithdrawCmd,
	}
	annotateHelpErrors(cmd, errorCodes...)
	cmd.Flags().String("delta", "", "Delta ID that requires withdrawal")
	return cmd
}

func newReqStaleCmd() *cobra.Command {
	errorCodes := []string{"INVALID_INPUT", "VALIDATION_FAILED", "SPEC_NOT_FOUND", "REQUIREMENT_NOT_FOUND", "DELTA_NOT_FOUND"}
	cmd := &cobra.Command{
		Use:   "stale <charter:slug> <requirement-id>",
		Short: "Mark one active requirement stale",
		Args:  validateRequirementVerifyArgs,
		Long: commandLong(`Mark one active requirement stale. A stale requirement needs
re-implementation or re-verification because the underlying code changed.
The agent should implement the updated behavior and then verify it.
Requires --delta <delta-id>.`, errorCodes...),
		RunE:  runReqStaleCmd,
	}
	annotateHelpErrors(cmd, errorCodes...)
	cmd.Flags().String("delta", "", "Delta ID that requires stale evidence")
	return cmd
}

func newReqRefreshCmd() *cobra.Command {
	errorCodes := []string{"INVALID_INPUT", "VALIDATION_FAILED", "SPEC_NOT_FOUND", "REQUIREMENT_NOT_FOUND", "INVALID_GHERKIN", "INVALID_GHERKIN_TAG", "REQUIREMENT_NOT_IN_SPEC", "REQUIREMENT_DUPLICATE_IN_SPEC"}
	cmd := &cobra.Command{
		Use:   "refresh <charter:slug> <requirement-id>",
		Short: "Refresh the stored requirement block in place",
		Args:  validateRequirementVerifyArgs,
		Long: commandLong(`Refresh one active requirement's stored Gherkin block from stdin.
Use this to update the Gherkin text without changing the requirement's
lifecycle or verification state. Write from the external surface.`, errorCodes...),
		RunE:  runReqRefreshCmd,
	}
	annotateHelpErrors(cmd, errorCodes...)
	return cmd
}

func newReqVerifyCmd() *cobra.Command {
	errorCodes := []string{"INVALID_INPUT", "VALIDATION_FAILED", "SPEC_NOT_FOUND", "REQUIREMENT_NOT_FOUND", "REQUIREMENT_INVALID_LIFECYCLE", "TEST_FILES_REQUIRED", "TEST_FILE_NOT_FOUND", "REQUIREMENT_MATCH_BLOCKING"}
	cmd := &cobra.Command{
		Use:   "verify <charter:slug> <requirement-id>",
		Short: "Mark a requirement as verified",
		Args:  validateRequirementVerifyArgs,
		Long: commandLong(`Mark one requirement as verified and replace its persisted test_files set.

Verification confirms that the observable behavior described by the
requirement is implemented and tested. Each test file should exercise
the external-surface behavior, not internal implementation details.

Stdin:
  This command does not read stdin.
  Flags:
    --test-file <path>  Repeatable. Required unless the requirement carries @manual.

  @manual requirements:
    Requirements tagged @manual can be verified without --test-file.
    Use @manual for behaviors that are proven by:
    - Infrastructure operation (e.g., process isolation, boot sequences)
    - Code inspection (e.g., architectural import boundaries)
    - Integration evidence that cannot be captured in an isolated test
    To mark a requirement as manual, add @manual to its Gherkin tag line
    in SPEC.md, then run req refresh to sync the stored tags.

Example:
  specctl req verify runtime:session-lifecycle REQ-001 --test-file runtime/tests/domain/test_compensation_cleanup.py

Output:
  JSON { state: context <spec> projection, result: { kind, requirement }, next: [...] }.`,
			errorCodes...,
		),
		RunE: runReqVerifyCmd,
	}
	annotateHelpErrors(cmd, errorCodes...)
	cmd.Flags().StringSlice("test-file", nil, "Repo-relative verification test file")
	return cmd
}

func runReqAddCmd(cmd *cobra.Command, args []string) error {
	data, err := io.ReadAll(cmd.InOrStdin())
	if err != nil {
		return err
	}
	deltaID, _ := cmd.Flags().GetString("delta")

	service, err := application.OpenFromWorkingDir()
	if err != nil {
		return err
	}

	state, result, next, err := service.AddRequirement(application.RequirementAddRequest{
		Target:  args[0],
		DeltaID: strings.TrimSpace(deltaID),
		Gherkin: strings.TrimRight(string(data), " \t\r\n"),
	})
	if err != nil {
		return applicationError(err)
	}
	responseState, focus := splitResponseState(state)
	return writeWriteEnvelope(cmd, responseState, focus, result, nextSequence(next))
}

func runReqReplaceCmd(cmd *cobra.Command, args []string) error {
	data, err := io.ReadAll(cmd.InOrStdin())
	if err != nil {
		return err
	}
	deltaID, _ := cmd.Flags().GetString("delta")
	service, err := application.OpenFromWorkingDir()
	if err != nil {
		return err
	}
	state, result, next, err := service.ReplaceRequirement(application.RequirementReplaceRequest{
		Target:        args[0],
		RequirementID: args[1],
		DeltaID:       strings.TrimSpace(deltaID),
		Gherkin:       strings.TrimRight(string(data), " \t\r\n"),
	})
	if err != nil {
		return applicationError(err)
	}
	responseState, focus := splitResponseState(state)
	return writeWriteEnvelope(cmd, responseState, focus, result, nextSequence(next))
}

func runReqWithdrawCmd(cmd *cobra.Command, args []string) error {
	deltaID, _ := cmd.Flags().GetString("delta")
	service, err := application.OpenFromWorkingDir()
	if err != nil {
		return err
	}
	state, result, next, err := service.WithdrawRequirement(application.RequirementDeltaRequest{
		Target:        args[0],
		RequirementID: args[1],
		DeltaID:       strings.TrimSpace(deltaID),
	})
	if err != nil {
		return applicationError(err)
	}
	responseState, focus := splitResponseState(state)
	return writeWriteEnvelope(cmd, responseState, focus, result, nextSequence(next))
}

func runReqStaleCmd(cmd *cobra.Command, args []string) error {
	deltaID, _ := cmd.Flags().GetString("delta")
	service, err := application.OpenFromWorkingDir()
	if err != nil {
		return err
	}
	state, result, next, err := service.StaleRequirement(application.RequirementDeltaRequest{
		Target:        args[0],
		RequirementID: args[1],
		DeltaID:       strings.TrimSpace(deltaID),
	})
	if err != nil {
		return applicationError(err)
	}
	responseState, focus := splitResponseState(state)
	return writeWriteEnvelope(cmd, responseState, focus, result, nextSequence(next))
}

func runReqRefreshCmd(cmd *cobra.Command, args []string) error {
	data, err := io.ReadAll(cmd.InOrStdin())
	if err != nil {
		return err
	}
	service, err := application.OpenFromWorkingDir()
	if err != nil {
		return err
	}
	state, result, next, err := service.RefreshRequirement(application.RequirementRefreshRequest{
		Target:        args[0],
		RequirementID: args[1],
		Gherkin:       strings.TrimRight(string(data), " \t\r\n"),
	})
	if err != nil {
		return applicationError(err)
	}
	responseState, focus := splitResponseState(state)
	return writeWriteEnvelope(cmd, responseState, focus, result, nextSequence(next))
}

func runReqVerifyCmd(cmd *cobra.Command, args []string) error {
	testFiles, _ := cmd.Flags().GetStringSlice("test-file")

	service, err := application.OpenFromWorkingDir()
	if err != nil {
		return err
	}

	state, result, next, err := service.VerifyRequirement(application.RequirementVerifyRequest{
		Target:        args[0],
		RequirementID: args[1],
		TestFiles:     testFiles,
	})
	if err != nil {
		return applicationError(err)
	}
	responseState, focus := splitResponseState(state)
	return writeWriteEnvelope(cmd, responseState, focus, result, nextSequence(next))
}
