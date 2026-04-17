package cli

import (
	specctl "github.com/aitoroses/specctl"
	"github.com/spf13/cobra"
)

func newExampleCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "example",
		Short: "Show the built-in governed specification example",
		Args:  cobra.NoArgs,
		Long: commandLong(`Show the built-in governed specification example.

specctl uses its own behavioral spec as the canonical example of a fully
governed specification. This command returns the complete example embedded
at compile time: the design document (SPEC.md), format template, project
config, charter, and tracking file.

Use this to understand:
  - How a five-layer behavioral spec is structured (prose, data model,
    contracts, invariants, Gherkin tracking)
  - How specctl.yaml configures tags, prefixes, and format templates
  - How a charter organizes specs into groups with ordering
  - How a tracking file records deltas, requirements, and changelog
  - What a fully verified, rev-bumped spec looks like end-to-end

Stdin:
  This command does not read stdin.

Output:
  JSON envelope with state, focus, result, next.
  focus.files[]       metadata: path, role, line count per embedded file
  result.*            full file contents as string fields:
    design_document   SPEC.md (the behavioral specification)
    format_template   SPEC_FORMAT.md (section structure guide)
    config            .specs/specctl.yaml (project configuration)
    charter           .specs/specctl/CHARTER.yaml (spec registry)
    tracking          .specs/specctl/cli.yaml (lifecycle state)

No flags. No filesystem access required.
This command never fails — the example is compiled into the binary.

Example:
  specctl example
  specctl example | jq '.result.design_document'
  specctl example | jq '.focus.files'`),
		RunE: runExampleCmd,
	}
	return cmd
}

func runExampleCmd(cmd *cobra.Command, args []string) error {
	state := map[string]any{
		"kind":    "example",
		"version": "specctl:cli",
	}
	focus := map[string]any{
		"description": "Complete governed specification example — specctl's own behavioral spec",
		"files":       specctl.ExampleFiles(),
	}
	result := map[string]any{
		"design_document": specctl.ExampleSpec,
		"format_template": specctl.ExampleFormat,
		"config":          specctl.ExampleConfig,
		"charter":         specctl.ExampleCharter,
		"tracking":        specctl.ExampleTracking,
	}
	return writeWriteEnvelope(cmd, state, focus, result, nextNone())
}
