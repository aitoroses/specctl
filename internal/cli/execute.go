package cli

import (
	"io"
	"os"

	"github.com/aitoroses/specctl/internal/presenter"
	"github.com/spf13/cobra"
)

type nextDirective = presenter.Directive

func Execute(args []string, stdout, stderr io.Writer) int {
	return executeWithIO(args, os.Stdin, stdout, stderr)
}

func executeWithIO(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	rootCmd := NewRootCmd()
	rootCmd.SetIn(stdin)
	rootCmd.SetOut(stdout)
	rootCmd.SetErr(stderr)
	rootCmd.SetArgs(args)
	isMCP := len(args) > 0 && args[0] == "mcp"

	if err := rootCmd.Execute(); err != nil {
		if isMCP {
			_, _ = io.WriteString(stderr, err.Error())
			return 1
		}
		_ = presenter.WriteJSON(stdout, presenter.ErrorEnvelope(err))
		return 1
	}

	return 0
}

func writeReadEnvelope(cmd *cobra.Command, state, focus any, next nextDirective) error {
	return presenter.WriteJSON(
		cmd.OutOrStdout(),
		presenter.ReadEnvelope(state, focus, next),
	)
}

func writeWriteEnvelope(cmd *cobra.Command, state, focus, result any, next nextDirective) error {
	return presenter.WriteJSON(
		cmd.OutOrStdout(),
		presenter.WriteEnvelope(state, focus, result, next),
	)
}

func invalidInputError(message string, state any) error {
	return presenter.InvalidInput(message, state)
}

func coalesceState(state any) any {
	return presenter.CoalesceState(state)
}

func coalesceFocus(focus any) any {
	return presenter.CoalesceFocus(focus)
}

func coalesceNextDirective(next nextDirective) nextDirective {
	return presenter.CoalesceDirective(next)
}

func normalizeNextActions(next []any) []any {
	return presenter.NormalizeNextActions(next)
}

func nextNone() nextDirective {
	return presenter.None()
}

func nextSequence(steps []any) nextDirective {
	return presenter.Sequence(steps)
}

func nextChooseOne(options []any) nextDirective {
	return presenter.ChooseOne(options)
}

func nextChooseThenSequence(options []any) nextDirective {
	return presenter.ChooseThenSequence(options)
}

func splitResponseState(state any) (any, any) {
	return presenter.SplitStateFocus(state)
}
