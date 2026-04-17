package cli

import (
	"strings"

	"github.com/aitoroses/specctl/internal/application"
	"github.com/spf13/cobra"
)

func newSpecCmd() *cobra.Command {
	errorCodes := []string{"INVALID_INPUT"}
	cmd := &cobra.Command{
		Use:   "spec",
		Short: "Manage tracking files",
		Long: commandLong(`Tracking-file command family.

Stdin:
  This command family does not read stdin directly. See subcommand help.

Example:
  specctl spec create runtime:runtime-api-contract --title "Runtime API Contract" --doc runtime/src/adapters/inbound/http/SPEC.md --scope runtime/src/adapters/inbound/http/ --group inbound-http --group-title "Inbound HTTP" --group-order 30 --order 10 --charter-notes "HTTP request/response contract"

Output:
  JSON only. Read command families return {state,next}. Write commands return {state,result,next} or {error,state,next}.`,
			errorCodes...,
		),
	}
	annotateHelpErrors(cmd, errorCodes...)
	cmd.AddCommand(newSpecCreateCmd())
	cmd.AddCommand(newSpecDocAddCmd())
	cmd.AddCommand(newSpecDocRemoveCmd())
	return cmd
}

func newSpecCreateCmd() *cobra.Command {
	errorCodes := []string{
		"INVALID_INPUT",
		"VALIDATION_FAILED",
		"SPEC_EXISTS",
		"CHARTER_NOT_FOUND",
		"GROUP_REQUIRED",
		"INVALID_PATH",
		"FORMAT_AMBIGUOUS",
		"FORMAT_NOT_CONFIGURED",
		"PRIMARY_DOC_FRONTMATTER_INVALID",
		"PRIMARY_DOC_FRONTMATTER_MISMATCH",
		"CHECKPOINT_UNAVAILABLE",
	}
	cmd := &cobra.Command{
		Use:   "create <charter:slug>",
		Short: "Create a tracking file and charter membership",
		Args:  validateSpecIdentifierArgs(1),
		PreRunE: chainRunE(
			requireStringFlags("title", "doc", "group", "charter-notes"),
			requireStringSliceFlags("scope"),
			requireChangedFlags("order"),
			validateOptionalNewGroupFlags,
		),
		Long: commandLong(`Create a tracking file, charter membership entry, and primary design document contract.

Stdin:
  This command does not read stdin.
  Required flags:
    --title <text>
    --doc <path/to/SPEC.md>
    --scope <dir/>           Repeat once per governed directory.
    --group <group-key>
    --order <int>
    --charter-notes <text>
  Conditional flags for a new group:
    --group-title <text>
    --group-order <int>
  Optional flags:
    --depends-on <slug>      Repeatable.
    --tag <tag>              Repeatable informational spec tag.

Example:
  specctl spec create runtime:runtime-api-contract --title "Runtime API Contract" --doc runtime/src/adapters/inbound/http/SPEC.md --scope runtime/src/adapters/inbound/http/ --scope runtime/src/application/contracts/ --group inbound-http --group-title "Inbound HTTP" --group-order 30 --order 10 --charter-notes "HTTP request/response contract"

Output:
  JSON { state: context <spec> projection, result: { kind, tracking_file, design_doc, design_doc_action, selected_format }, next: [...] }.`,
			errorCodes...,
		),
		RunE: runSpecCreateCmd,
	}
	annotateHelpErrors(cmd, errorCodes...)
	cmd.Flags().String("title", "", "Human-readable spec title")
	cmd.Flags().String("doc", "", "Repo-relative primary design document path")
	cmd.Flags().StringSlice("scope", nil, "Repo-relative governed directory ending in /")
	cmd.Flags().String("group", "", "Charter group key")
	cmd.Flags().String("group-title", "", "Display title for a newly created group")
	cmd.Flags().Int("group-order", 0, "Order for a newly created group")
	cmd.Flags().Int("order", 0, "Order for the spec inside its group")
	cmd.Flags().String("charter-notes", "", "Short planning note for the charter entry")
	cmd.Flags().StringSlice("depends-on", nil, "Dependency slugs in the same charter")
	cmd.Flags().StringSlice("tag", nil, "Informational spec tag")
	return cmd
}

type specCreateRequest struct {
	Target       string
	Title        string
	Doc          string
	Scope        []string
	Group        string
	GroupTitle   *string
	GroupOrder   *int
	Order        int
	CharterNotes string
	DependsOn    []string
	Tags         []string
}

func runSpecCreateCmd(cmd *cobra.Command, args []string) error {
	request, err := decodeSpecCreateRequest(cmd, args)
	if err != nil {
		return err
	}

	service, err := application.OpenFromWorkingDir()
	if err != nil {
		return err
	}

	state, result, next, err := service.CreateSpec(application.SpecCreateRequest(request))
	if err != nil {
		return applicationError(err)
	}
	responseState, focus := splitResponseState(state)
	return writeWriteEnvelope(cmd, responseState, focus, result, nextSequence(next))
}

func decodeSpecCreateRequest(cmd *cobra.Command, args []string) (specCreateRequest, error) {
	scope, _ := cmd.Flags().GetStringSlice("scope")
	title, _ := cmd.Flags().GetString("title")
	doc, _ := cmd.Flags().GetString("doc")
	group, _ := cmd.Flags().GetString("group")
	notes, _ := cmd.Flags().GetString("charter-notes")
	order, _ := cmd.Flags().GetInt("order")
	dependsOn, _ := cmd.Flags().GetStringSlice("depends-on")
	tags, _ := cmd.Flags().GetStringSlice("tag")

	request := specCreateRequest{
		Target:       args[0],
		Title:        title,
		Doc:          doc,
		Scope:        append([]string{}, scope...),
		Group:        group,
		Order:        order,
		CharterNotes: notes,
		DependsOn:    append([]string{}, dependsOn...),
		Tags:         append([]string{}, tags...),
	}

	groupTitle, _ := cmd.Flags().GetString("group-title")
	if cmd.Flags().Changed("group-title") {
		trimmed := strings.TrimSpace(groupTitle)
		request.GroupTitle = &trimmed
	}

	groupOrder, _ := cmd.Flags().GetInt("group-order")
	if cmd.Flags().Changed("group-order") {
		request.GroupOrder = &groupOrder
	}

	return request, nil
}

func newSpecDocAddCmd() *cobra.Command {
	errorCodes := []string{
		"INVALID_INPUT",
		"INVALID_PATH",
		"VALIDATION_FAILED",
	}
	cmd := &cobra.Command{
		Use:   "doc-add <charter:slug>",
		Short: "Add a secondary document reference",
		Args:  validateSpecIdentifierArgs(1),
		PreRunE: chainRunE(
			requireStringFlags("doc"),
		),
		Long: commandLong(`Add a secondary document reference to a spec's documents.secondary list.

Stdin:
  This command does not read stdin.
  Required flags:
    --doc <path/to/JOURNEYS.md>   Repo-relative path to the secondary document.

Example:
  specctl spec doc-add ui:work-thread --doc ui/src/routes/_app/work/JOURNEYS.md

Output:
  JSON { state, result: { kind: "doc_add", doc }, next: [...] }.`,
			errorCodes...,
		),
		RunE: runSpecDocAddCmd,
	}
	annotateHelpErrors(cmd, errorCodes...)
	cmd.Flags().String("doc", "", "Repo-relative path to the secondary document")
	return cmd
}

func runSpecDocAddCmd(cmd *cobra.Command, args []string) error {
	doc, _ := cmd.Flags().GetString("doc")
	service, err := application.OpenFromWorkingDir()
	if err != nil {
		return err
	}
	state, result, next, err := service.DocAdd(application.DocAddRequest{
		Target: args[0],
		Doc:    doc,
	})
	if err != nil {
		return applicationError(err)
	}
	responseState, focus := splitResponseState(state)
	return writeWriteEnvelope(cmd, responseState, focus, result, nextSequence(next))
}

func newSpecDocRemoveCmd() *cobra.Command {
	errorCodes := []string{
		"INVALID_INPUT",
		"DOC_NOT_FOUND",
		"VALIDATION_FAILED",
	}
	cmd := &cobra.Command{
		Use:   "doc-remove <charter:slug>",
		Short: "Remove a secondary document reference",
		Args:  validateSpecIdentifierArgs(1),
		PreRunE: chainRunE(
			requireStringFlags("doc"),
		),
		Long: commandLong(`Remove a secondary document reference from a spec's documents.secondary list.

Stdin:
  This command does not read stdin.
  Required flags:
    --doc <path/to/JOURNEYS.md>   Repo-relative path to the secondary document.

Example:
  specctl spec doc-remove ui:work-thread --doc ui/src/routes/_app/work/JOURNEYS.md

Output:
  JSON { state, result: { kind: "doc_remove", doc }, next: [...] }.`,
			errorCodes...,
		),
		RunE: runSpecDocRemoveCmd,
	}
	annotateHelpErrors(cmd, errorCodes...)
	cmd.Flags().String("doc", "", "Repo-relative path to the secondary document")
	return cmd
}

func runSpecDocRemoveCmd(cmd *cobra.Command, args []string) error {
	doc, _ := cmd.Flags().GetString("doc")
	service, err := application.OpenFromWorkingDir()
	if err != nil {
		return err
	}
	state, result, next, err := service.DocRemove(application.DocRemoveRequest{
		Target: args[0],
		Doc:    doc,
	})
	if err != nil {
		return applicationError(err)
	}
	responseState, focus := splitResponseState(state)
	return writeWriteEnvelope(cmd, responseState, focus, result, nextSequence(next))
}
