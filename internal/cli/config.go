package cli

import (
	"github.com/aitoroses/specctl/internal/application"
	"github.com/spf13/cobra"
)

func newConfigCmd() *cobra.Command {
	errorCodes := []string{"INVALID_INPUT"}
	cmd := &cobra.Command{
		Use:   "config",
		Short: "Manage specctl project config",
		Args:  validateNoArgsReadCommand("config"),
		Long: commandLong(`Project config read and write command family.

Stdin:
  This command family does not read stdin.

Example:
  specctl config
  specctl config add-tag runtime

Output:
  JSON { state: config projection, next: [...] } for reads and {state,result,next} or {error,state,next} for writes.`,
			errorCodes...,
		),
		RunE: runConfigCmd,
	}
	annotateHelpErrors(cmd, errorCodes...)
	cmd.AddCommand(
		newConfigAddTagCmd(),
		newConfigRemoveTagCmd(),
		newConfigAddPrefixCmd(),
		newConfigRemovePrefixCmd(),
	)
	return cmd
}

func runConfigCmd(cmd *cobra.Command, _ []string) error {
	service, err := application.OpenFromWorkingDir()
	if err != nil {
		return err
	}

	state, err := service.ReadConfig()
	if err != nil {
		return err
	}
	responseState, focus := splitResponseState(state)
	return writeReadEnvelope(cmd, responseState, focus, nextNone())
}

func newConfigAddTagCmd() *cobra.Command {
	errorCodes := []string{"INVALID_INPUT", "VALIDATION_FAILED", "TAG_EXISTS", "SEMANTIC_TAG_RESERVED"}
	cmd := &cobra.Command{
		Use:   "add-tag <tag>",
		Short: "Register a gherkin tag so @tag is accepted in requirement blocks",
		Args:  cobra.ExactArgs(1),
		Long: commandLong(`Register a tag in the gherkin_tags config set.

Tags control which @-prefixed labels are legal in requirement Gherkin
blocks. When req add encounters @webhook but "webhook" is not in
gherkin_tags, it rejects with INVALID_GHERKIN_TAG. Use this command
to register the tag first.

Semantic tags (e2e, manual) are built-in and cannot be added or removed.

Stdin:
  This command does not read stdin.

Example:
  specctl config add-tag webhook

Output:
  JSON { state: config projection, result: { kind, mutation, tag }, next: [...] }.`,
			errorCodes...,
		),
		RunE: runConfigAddTagCmd,
	}
	annotateHelpErrors(cmd, errorCodes...)
	return cmd
}

func newConfigRemoveTagCmd() *cobra.Command {
	errorCodes := []string{"INVALID_INPUT", "VALIDATION_FAILED", "TAG_NOT_FOUND", "TAG_IN_USE"}
	cmd := &cobra.Command{
		Use:   "remove-tag <tag>",
		Short: "Remove a gherkin tag if no requirement uses it",
		Args:  cobra.ExactArgs(1),
		Long: commandLong(`Remove a tag from the gherkin_tags config set.

The tag is removed only if no tracked requirement currently uses it.
If any requirement's Gherkin block contains @tag, removal is rejected
with TAG_IN_USE.

Stdin:
  This command does not read stdin.

Example:
  specctl config remove-tag webhook

Output:
  JSON { state: config projection, result: { kind, mutation, tag }, next: [...] }.`,
			errorCodes...,
		),
		RunE: runConfigRemoveTagCmd,
	}
	annotateHelpErrors(cmd, errorCodes...)
	return cmd
}

func newConfigAddPrefixCmd() *cobra.Command {
	errorCodes := []string{"INVALID_INPUT", "VALIDATION_FAILED", "INVALID_PATH", "PREFIX_EXISTS"}
	cmd := &cobra.Command{
		Use:   "add-prefix <path>",
		Short: "Add one source prefix if the directory exists",
		Args:  cobra.ExactArgs(1),
		Long: commandLong(`Add one normalized source prefix.

Stdin:
  This command does not read stdin.

Example:
  specctl config add-prefix runtime/src/

Output:
  JSON { state: config projection, result: { kind, mutation, prefix }, next: [...] }.`,
			errorCodes...,
		),
		RunE: runConfigAddPrefixCmd,
	}
	annotateHelpErrors(cmd, errorCodes...)
	return cmd
}

func newConfigRemovePrefixCmd() *cobra.Command {
	errorCodes := []string{"INVALID_INPUT", "VALIDATION_FAILED", "PREFIX_NOT_FOUND"}
	cmd := &cobra.Command{
		Use:   "remove-prefix <path>",
		Short: "Remove one source prefix",
		Args:  cobra.ExactArgs(1),
		Long: commandLong(`Remove one persisted source prefix.

Stdin:
  This command does not read stdin.

Example:
  specctl config remove-prefix runtime/src/

Output:
  JSON { state: config projection, result: { kind, mutation, prefix }, next: [...] }.`,
			errorCodes...,
		),
		RunE: runConfigRemovePrefixCmd,
	}
	annotateHelpErrors(cmd, errorCodes...)
	return cmd
}

func runConfigAddTagCmd(cmd *cobra.Command, args []string) error {
	service, err := application.OpenFromWorkingDir()
	if err != nil {
		return err
	}
	state, result, next, err := service.AddConfigTag(args[0])
	if err != nil {
		return applicationError(err)
	}
	responseState, focus := splitResponseState(state)
	return writeWriteEnvelope(cmd, responseState, focus, result, nextSequence(next))
}

func runConfigRemoveTagCmd(cmd *cobra.Command, args []string) error {
	service, err := application.OpenFromWorkingDir()
	if err != nil {
		return err
	}
	state, result, next, err := service.RemoveConfigTag(args[0])
	if err != nil {
		return applicationError(err)
	}
	responseState, focus := splitResponseState(state)
	return writeWriteEnvelope(cmd, responseState, focus, result, nextSequence(next))
}

func runConfigAddPrefixCmd(cmd *cobra.Command, args []string) error {
	service, err := application.OpenFromWorkingDir()
	if err != nil {
		return err
	}
	state, result, next, err := service.AddConfigPrefix(args[0])
	if err != nil {
		return applicationError(err)
	}
	responseState, focus := splitResponseState(state)
	return writeWriteEnvelope(cmd, responseState, focus, result, nextSequence(next))
}

func runConfigRemovePrefixCmd(cmd *cobra.Command, args []string) error {
	service, err := application.OpenFromWorkingDir()
	if err != nil {
		return err
	}
	state, result, next, err := service.RemoveConfigPrefix(args[0])
	if err != nil {
		return applicationError(err)
	}
	responseState, focus := splitResponseState(state)
	return writeWriteEnvelope(cmd, responseState, focus, result, nextSequence(next))
}

