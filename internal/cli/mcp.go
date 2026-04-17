package cli

import (
	"context"

	specctlmcp "github.com/aitoroses/specctl/internal/mcp"
	"github.com/spf13/cobra"
)

func newMcpCmd() *cobra.Command {
	return &cobra.Command{
		Use:    "mcp",
		Short:  "Run the specctl MCP stdio server",
		Args:   cobra.NoArgs,
		Hidden: false,
		Long: commandLong(`Run the specctl MCP server over stdio.

This subcommand starts a long-lived MCP server intended to be launched by an
MCP client such as Codex or mcporter.

Stdin:
  MCP JSON-RPC messages over stdio.

Output:
  MCP JSON-RPC messages over stdio.`,
			"INVALID_INPUT",
		),
		RunE: func(cmd *cobra.Command, args []string) error {
			return specctlmcp.RunStdio(context.Background())
		},
	}
}
