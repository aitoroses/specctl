package cli

import (
	"fmt"
	"io"
	"os"
	"runtime/debug"

	"github.com/aitoroses/specctl/internal/presenter"
	"github.com/spf13/cobra"
)

type nextDirective = presenter.Directive

func Execute(args []string, stdout, stderr io.Writer) int {
	return executeWithIO(args, os.Stdin, stdout, stderr)
}

func executeWithIO(args []string, stdin io.Reader, stdout, stderr io.Writer) (code int) {
	isMCP := len(args) > 0 && args[0] == "mcp"
	cmdName := commandNameFromArgs(args)

	defer func() {
		r := recover()
		if r == nil {
			return
		}
		stack := string(debug.Stack())
		fmt.Fprintf(stderr, "specctl panic in %s: %v\n%s\n", cmdName, r, stack)
		envelope := presenter.UnexpectedErrorEnvelope(
			fmt.Errorf("panic: %v", r),
			presenter.UnexpectedContext{
				Tool:       cmdName,
				Input:      args,
				PanicValue: r,
				Stack:      stack,
			},
		)
		if isMCP {
			fmt.Fprintf(stderr, "%s\n%s\n", envelope.Error.Message, issueHintBlock(envelope))
		} else {
			_ = presenter.WriteJSON(stdout, envelope)
		}
		code = 1
	}()

	rootCmd := NewRootCmd()
	rootCmd.SetIn(stdin)
	rootCmd.SetOut(stdout)
	rootCmd.SetErr(stderr)
	rootCmd.SetArgs(args)

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

func commandNameFromArgs(args []string) string {
	if len(args) == 0 {
		return "specctl"
	}
	return "specctl " + args[0]
}

// issueHintBlock formats the report_issue step from an unexpected envelope
// into a plain-text block suitable for stderr (used on the MCP path where
// we don't emit a JSON envelope).
func issueHintBlock(env presenter.Envelope) string {
	if len(env.Next.Steps) == 0 {
		return ""
	}
	step, ok := env.Next.Steps[0].(map[string]any)
	if !ok {
		return ""
	}
	tmpl, ok := step["template"].(map[string]any)
	if !ok {
		return ""
	}
	url, _ := tmpl["url"].(string)
	body, _ := tmpl["body"].(string)
	return fmt.Sprintf("This looks like a bug in specctl. Please open an issue:\n  %s\n\nInclude this context:\n---\n%s---", url, body)
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
