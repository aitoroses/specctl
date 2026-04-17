package cli

import (
	"strings"

	"github.com/aitoroses/specctl/internal/application"
	"github.com/aitoroses/specctl/internal/domain"
	"github.com/aitoroses/specctl/internal/presenter"
	"github.com/spf13/cobra"
)

func newCharterCmd() *cobra.Command {
	errorCodes := []string{"INVALID_INPUT"}
	cmd := &cobra.Command{
		Use:   "charter",
		Short: "Manage charter files",
		Long: commandLong(`Charter command family.

Stdin:
  charter create and charter add-spec read YAML from stdin. charter remove-spec does not read stdin.

Example:
  echo 'title: Runtime System
description: Specs for runtime control-plane and data-plane behavior' | specctl charter create runtime

Output:
  JSON only. Write commands return {state,focus,result,next} or {error,state,focus,next}.`,
			errorCodes...,
		),
	}
	annotateHelpErrors(cmd, errorCodes...)
	cmd.AddCommand(newCharterCreateCmd(), newCharterAddSpecCmd(), newCharterRemoveSpecCmd())
	return cmd
}

func newCharterCreateCmd() *cobra.Command {
	errorCodes := []string{"INVALID_INPUT", "VALIDATION_FAILED", "CHARTER_EXISTS"}
	cmd := &cobra.Command{
		Use:   "create <charter>",
		Short: "Create a charter directory and CHARTER.yaml",
		Args:  validateCharterNameArgs(1),
		Long: commandLong(`Create a charter directory and its initial CHARTER.yaml file.

Stdin:
  YAML object:
    title: string, required
    description: string, required
    groups: list, optional
      - key: string, required
        title: string, required
        order: int >= 0, required

Example:
  echo 'title: Runtime System
description: Specs for runtime control-plane and data-plane behavior
groups:
  - key: execution
    title: Execution Engine
    order: 10' | specctl charter create runtime

Output:
  JSON { state: context <charter> projection, focus: {}, result: { kind, tracking_file, created_groups }, next: [...] }.`,
			errorCodes...,
		),
		RunE: runCharterCreateCmd,
	}
	annotateHelpErrors(cmd, errorCodes...)
	return cmd
}

func newCharterAddSpecCmd() *cobra.Command {
	errorCodes := []string{"INVALID_INPUT", "VALIDATION_FAILED", "CHARTER_NOT_FOUND", "SPEC_NOT_FOUND", "GROUP_REQUIRED", "CHARTER_CYCLE"}
	cmd := &cobra.Command{
		Use:   "add-spec <charter> <slug>",
		Short: "Create or replace one charter membership entry",
		Args:  validateCharterAndSlugArgs,
		Long: commandLong(`Create or replace one charter membership entry.

Stdin:
  YAML object:
    group: string, required
    group_title: string, conditionally required for a new group
    group_order: int >= 0, conditionally required for a new group
    order: int >= 0, required
    depends_on: list, optional, defaults to []
    notes: string, required

Example:
  cat <<'EOF' | specctl charter add-spec runtime session-lifecycle
  group: recovery
  group_title: Recovery and Cleanup
  group_order: 20
  order: 30
  depends_on:
    - redis-state
  notes: Session FSM and cleanup behavior
  EOF

Output:
  JSON { state: context <charter> projection, focus: {}, result: { kind, entry, created_group }, next: [...] }.`,
			errorCodes...,
		),
		RunE: runCharterAddSpecCmd,
	}
	annotateHelpErrors(cmd, errorCodes...)
	return cmd
}

func newCharterRemoveSpecCmd() *cobra.Command {
	errorCodes := []string{"INVALID_INPUT", "VALIDATION_FAILED", "CHARTER_NOT_FOUND", "SPEC_NOT_FOUND", "CHARTER_DEPENDENCY_EXISTS"}
	cmd := &cobra.Command{
		Use:   "remove-spec <charter> <slug>",
		Short: "Remove one charter membership entry",
		Args:  validateCharterAndSlugArgs,
		Long: commandLong(`Remove one charter membership entry without deleting the tracking file.

Stdin:
  This command does not read stdin.

Example:
  specctl charter remove-spec runtime session-lifecycle

Output:
  JSON { state: context <charter> projection, focus: {}, result: { kind, removed_slug }, next: [...] }.`,
			errorCodes...,
		),
		RunE: runCharterRemoveSpecCmd,
	}
	annotateHelpErrors(cmd, errorCodes...)
	return cmd
}

type charterCreateRequest struct {
	Title       string                      `yaml:"title"`
	Description string                      `yaml:"description"`
	Groups      []charterCreateRequestGroup `yaml:"groups"`
}

type charterCreateRequestGroup struct {
	Key   string `yaml:"key"`
	Title string `yaml:"title"`
	Order int    `yaml:"order"`
}

type charterAddSpecRequest struct {
	Group      string   `yaml:"group"`
	GroupTitle string   `yaml:"group_title"`
	GroupOrder *int     `yaml:"group_order"`
	Order      *int     `yaml:"order"`
	DependsOn  []string `yaml:"depends_on"`
	Notes      string   `yaml:"notes"`
}

func runCharterCreateCmd(cmd *cobra.Command, args []string) error {
	request, err := decodeCharterCreateRequest(cmd, args)
	if err != nil {
		return err
	}

	service, err := application.OpenFromWorkingDir()
	if err != nil {
		return err
	}

	state, result, next, err := service.CreateCharter(request)
	if err != nil {
		if charterExists, ok := err.(application.ErrCharterExists); ok {
			return &presenter.Failure{
				Code:    "CHARTER_EXISTS",
				Message: err.Error(),
				State: map[string]any{
					"charter":       charterExists.Charter,
					"tracking_file": ".specs/" + charterExists.Charter + "/CHARTER.yaml",
				},
				Focus: map[string]any{},
				Next:  nextNone(),
			}
		}
		return applicationError(err)
	}

	responseState, focus := splitResponseState(state)
	return writeWriteEnvelope(cmd, responseState, focus, result, nextSequence(next))
}

func runCharterAddSpecCmd(cmd *cobra.Command, args []string) error {
	request, err := decodeCharterAddSpecRequest(cmd, args)
	if err != nil {
		return err
	}

	service, err := application.OpenFromWorkingDir()
	if err != nil {
		return err
	}

	state, result, next, err := service.AddSpecToCharter(application.CharterAddSpecRequest{
		Charter:    args[0],
		Slug:       args[1],
		Group:      request.Group,
		GroupTitle: optionalTrimmedString(request.GroupTitle),
		GroupOrder: request.GroupOrder,
		Order:      *request.Order,
		DependsOn:  request.DependsOn,
		Notes:      request.Notes,
	})
	if err != nil {
		return applicationError(err)
	}
	responseState, focus := splitResponseState(state)
	return writeWriteEnvelope(cmd, responseState, focus, result, nextSequence(next))
}

func runCharterRemoveSpecCmd(cmd *cobra.Command, args []string) error {
	service, err := application.OpenFromWorkingDir()
	if err != nil {
		return err
	}

	state, result, next, err := service.RemoveSpecFromCharter(args[0], args[1])
	if err != nil {
		return applicationError(err)
	}
	responseState, focus := splitResponseState(state)
	return writeWriteEnvelope(cmd, responseState, focus, result, nextSequence(next))
}

func decodeCharterCreateRequest(cmd *cobra.Command, args []string) (application.CharterCreateRequest, error) {
	body, _, err := readStdinYAML[charterCreateRequest](cmd)
	if err != nil {
		return application.CharterCreateRequest{}, err
	}

	groups := make([]domain.CharterGroup, 0, len(body.Groups))
	for i, group := range body.Groups {
		if strings.TrimSpace(group.Key) == "" {
			return application.CharterCreateRequest{}, invalidInputError("groups["+itoa(i)+"].key is required", minimalCharterState(args[0]))
		}
		if strings.TrimSpace(group.Title) == "" {
			return application.CharterCreateRequest{}, invalidInputError("groups["+itoa(i)+"].title is required", minimalCharterState(args[0]))
		}
		groups = append(groups, domain.CharterGroup{
			Key:   group.Key,
			Title: group.Title,
			Order: group.Order,
		})
	}

	request := application.CharterCreateRequest{
		Charter:     args[0],
		Title:       body.Title,
		Description: body.Description,
		Groups:      groups,
	}
	if strings.TrimSpace(request.Title) == "" {
		return application.CharterCreateRequest{}, invalidInputError("title is required", minimalCharterState(args[0]))
	}
	if strings.TrimSpace(request.Description) == "" {
		return application.CharterCreateRequest{}, invalidInputError("description is required", minimalCharterState(args[0]))
	}
	return request, nil
}

func decodeCharterAddSpecRequest(cmd *cobra.Command, args []string) (charterAddSpecRequest, error) {
	request, _, err := readStdinYAML[charterAddSpecRequest](cmd)
	if err != nil {
		return charterAddSpecRequest{}, err
	}
	state := minimalSpecState(args[0] + ":" + args[1])
	if strings.TrimSpace(request.Group) == "" {
		return charterAddSpecRequest{}, invalidInputError("group is required", state)
	}
	if request.Order == nil {
		return charterAddSpecRequest{}, invalidInputError("order is required", state)
	}
	if strings.TrimSpace(request.Notes) == "" {
		return charterAddSpecRequest{}, invalidInputError("notes is required", state)
	}
	return request, nil
}

func optionalTrimmedString(value string) *string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}
