package cli

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/spf13/cobra"
)

var (
	charterPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*$`)
	specIDPattern  = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*:[a-z0-9][a-z0-9-]*$`)
	deltaIDPattern = regexp.MustCompile(`^D-\d{3}$`)
	reqIDPattern   = regexp.MustCompile(`^REQ-\d{3}$`)
)

func validateContextArgs(cmd *cobra.Command, args []string) error {
	if len(args) > 1 {
		return invalidInputError("accepts 1 arg(s), received "+fmt.Sprintf("%d", len(args)), inputFocusedState(map[string]any{
			"args": args,
		}, map[string]any{
			"reason":   "too_many_args",
			"received": len(args),
			"max_args": 1,
		}))
	}

	fileTarget, _ := cmd.Flags().GetString("file")
	if fileTarget != "" {
		if len(args) != 0 {
			return invalidInputError("context accepts either [target] or --file, not both", inputFocusedState(map[string]any{
				"target": args[0],
				"file":   fileTarget,
			}, map[string]any{
				"reason": "target_and_file_conflict",
			}))
		}
		return nil
	}

	if len(args) == 0 {
		return nil
	}
	if isValidSpecID(args[0]) || isValidCharter(args[0]) {
		return nil
	}
	return invalidInputError("context target must be <charter> or <charter:slug>", inputFocusedState(minimalContextTargetState(args[0]), map[string]any{
		"reason":   "invalid_target",
		"expected": "<charter> or <charter:slug>",
	}))
}

func validateDiffArgs(cmd *cobra.Command, args []string) error {
	if len(args) > 1 {
		return invalidInputError("accepts 1 arg(s), received "+fmt.Sprintf("%d", len(args)), inputFocusedState(map[string]any{
			"args": args,
		}, map[string]any{
			"reason":   "too_many_args",
			"received": len(args),
			"max_args": 1,
		}))
	}

	charterTarget, _ := cmd.Flags().GetString("charter")
	if charterTarget != "" {
		if !isValidCharter(charterTarget) {
			return invalidInputError("diff --charter must be <charter>", inputFocusedState(minimalContextTargetState(charterTarget), map[string]any{
				"reason":   "invalid_charter",
				"expected": "<charter>",
			}))
		}
		if len(args) != 0 {
			return invalidInputError("diff accepts either <charter:slug> or --charter, not both", inputFocusedState(map[string]any{
				"target":  args[0],
				"charter": charterTarget,
			}, map[string]any{
				"reason": "target_and_charter_conflict",
			}))
		}
		return nil
	}

	if len(args) == 0 {
		return invalidInputError("diff requires either <charter:slug> or --charter <charter>", inputFocusedState(map[string]any{}, map[string]any{
			"reason":   "missing_target",
			"expected": "<charter:slug> or --charter <charter>",
		}))
	}
	if !isValidSpecID(args[0]) {
		return invalidInputError("diff target must be <charter:slug>", inputFocusedState(minimalContextTargetState(args[0]), map[string]any{
			"reason":   "invalid_target",
			"expected": "<charter:slug>",
		}))
	}
	return nil
}

func validateNoArgsReadCommand(command string) func(*cobra.Command, []string) error {
	return func(_ *cobra.Command, args []string) error {
		if len(args) == 0 {
			return nil
		}
		return invalidInputError(fmt.Sprintf("accepts 0 arg(s), received %d", len(args)), inputFocusedState(minimalCommandState(command, args), map[string]any{
			"reason":   "too_many_args",
			"received": len(args),
			"max_args": 0,
		}))
	}
}

func validateSpecIdentifierArgs(expected int) cobra.PositionalArgs {
	return func(_ *cobra.Command, args []string) error {
		if len(args) != expected {
			return invalidInputError(fmt.Sprintf("accepts %d arg(s), received %d", expected, len(args)), map[string]any{})
		}
		for _, arg := range args {
			if !isValidSpecID(arg) {
				return invalidInputError("spec target must be <charter:slug>", minimalSpecState(arg))
			}
		}
		return nil
	}
}

func validateDeltaCommandArgs(cmd *cobra.Command, args []string) error {
	if err := cobra.ExactArgs(2)(cmd, args); err != nil {
		return err
	}
	if !isValidSpecID(args[0]) {
		return invalidInputError("spec target must be <charter:slug>", minimalSpecState(args[0]))
	}
	if !deltaIDPattern.MatchString(args[1]) {
		return invalidInputError("delta ID must be D-NNN", minimalSpecState(args[0]))
	}
	return nil
}

func validateRequirementVerifyArgs(cmd *cobra.Command, args []string) error {
	if err := cobra.ExactArgs(2)(cmd, args); err != nil {
		return err
	}
	if !isValidSpecID(args[0]) {
		return invalidInputError("spec target must be <charter:slug>", minimalSpecState(args[0]))
	}
	if !reqIDPattern.MatchString(args[1]) {
		return invalidInputError("requirement ID must be REQ-NNN", minimalSpecState(args[0]))
	}
	return nil
}

func validateCharterNameArgs(expected int) cobra.PositionalArgs {
	return func(cmd *cobra.Command, args []string) error {
		if err := cobra.ExactArgs(expected)(cmd, args); err != nil {
			return err
		}
		for _, arg := range args {
			if !isValidCharter(arg) {
				return invalidInputError("charter name must match ^[a-z0-9][a-z0-9-]*$", minimalCharterState(arg))
			}
		}
		return nil
	}
}

func validateCharterAndSlugArgs(cmd *cobra.Command, args []string) error {
	if err := cobra.ExactArgs(2)(cmd, args); err != nil {
		return err
	}
	if !isValidCharter(args[0]) {
		return invalidInputError("charter name must match ^[a-z0-9][a-z0-9-]*$", minimalCharterState(args[0]))
	}
	if !isValidSlug(args[1]) {
		return invalidInputError("spec slug must match ^[a-z0-9][a-z0-9-]*$", minimalSpecState(args[0]+":"+args[1]))
	}
	return nil
}

func requireStringFlags(flags ...string) func(*cobra.Command, []string) error {
	return func(cmd *cobra.Command, _ []string) error {
		for _, name := range flags {
			value, _ := cmd.Flags().GetString(name)
			if strings.TrimSpace(value) == "" {
				return invalidInputError(fmt.Sprintf("--%s is required", name), commandState(cmd, nil))
			}
		}
		return nil
	}
}

func requireChangedFlags(flags ...string) func(*cobra.Command, []string) error {
	return func(cmd *cobra.Command, _ []string) error {
		for _, name := range flags {
			if !cmd.Flags().Changed(name) {
				return invalidInputError(fmt.Sprintf("--%s is required", name), commandState(cmd, nil))
			}
		}
		return nil
	}
}

func requireStringSliceFlags(flags ...string) func(*cobra.Command, []string) error {
	return func(cmd *cobra.Command, _ []string) error {
		for _, name := range flags {
			values, _ := cmd.Flags().GetStringSlice(name)
			if len(values) == 0 {
				return invalidInputError(fmt.Sprintf("--%s is required", name), commandState(cmd, nil))
			}
		}
		return nil
	}
}

func validateOptionalNewGroupFlags(cmd *cobra.Command, _ []string) error {
	groupTitleChanged := cmd.Flags().Changed("group-title")
	groupOrderChanged := cmd.Flags().Changed("group-order")
	if groupTitleChanged != groupOrderChanged {
		return invalidInputError("--group-title and --group-order must be provided together", commandState(cmd, nil))
	}
	if groupTitleChanged {
		title, _ := cmd.Flags().GetString("group-title")
		if strings.TrimSpace(title) == "" {
			return invalidInputError("--group-title is required when provided", commandState(cmd, nil))
		}
	}
	return nil
}

func chainRunE(validators ...func(*cobra.Command, []string) error) func(*cobra.Command, []string) error {
	return func(cmd *cobra.Command, args []string) error {
		for _, validator := range validators {
			if err := validator(cmd, args); err != nil {
				return err
			}
		}
		return nil
	}
}

func isValidSpecID(value string) bool {
	return specIDPattern.MatchString(value)
}

func isValidCharter(value string) bool {
	return charterPattern.MatchString(value)
}

func isValidSlug(value string) bool {
	return charterPattern.MatchString(value)
}
