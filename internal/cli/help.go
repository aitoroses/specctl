package cli

import (
	"strings"

	"github.com/spf13/cobra"
)

const helpErrorsAnnotation = "specctl_help_errors"

func commandLong(body string, errorCodes ...string) string {
	trimmed := strings.TrimRight(body, "\n")
	if len(errorCodes) == 0 {
		return trimmed
	}

	lines := make([]string, 0, len(errorCodes)+2)
	lines = append(lines, trimmed, "", "Errors:")
	for _, code := range errorCodes {
		lines = append(lines, "  "+code)
	}
	return strings.Join(lines, "\n")
}

func annotateHelpErrors(cmd *cobra.Command, errorCodes ...string) {
	if cmd.Annotations == nil {
		cmd.Annotations = map[string]string{}
	}
	cmd.Annotations[helpErrorsAnnotation] = strings.Join(errorCodes, "\n")
}

func declaredHelpErrors(cmd *cobra.Command) []string {
	if cmd == nil || cmd.Annotations == nil {
		return []string{}
	}
	raw := strings.TrimSpace(cmd.Annotations[helpErrorsAnnotation])
	if raw == "" {
		return []string{}
	}
	return strings.Split(raw, "\n")
}
