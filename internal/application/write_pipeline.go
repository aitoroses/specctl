package application

import (
	"errors"
	"fmt"
	pathpkg "path"
	"strings"

	"github.com/aitoroses/specctl/internal/domain"
	"github.com/aitoroses/specctl/internal/infrastructure"
)

func withSpecFocus(state SpecProjection, focus any) SpecProjection {
	if focus == nil {
		state.Focus = nil
		return state
	}
	state.Focus = focus
	return state
}

func (s *Service) specFailure(loaded *loadedSpec, code, message string, focus any, next []any) error {
	state := withSpecFocus(loaded.state, focus)
	return &Failure{
		Code:    code,
		Message: message,
		State:   state,
		Next:    coalesceNextActions(next),
	}
}

func validationFindingsFromMessage(path, target, message string) []infrastructure.ValidationFinding {
	return infrastructure.ValidationFindingsFromMessages(message, path, target)
}

func editFileAction(priority int, action, instructions, path string, createIfMissing bool) map[string]any {
	return map[string]any{
		"priority":          priority,
		"action":            action,
		"kind":              "edit_file",
		"instructions":      instructions,
		"path":              path,
		"create_if_missing": createIfMissing,
	}
}

func repairActionForPath(path string) string {
	if strings.EqualFold(pathpkg.Base(path), "CHARTER.yaml") {
		return "repair_charter_file"
	}
	if strings.EqualFold(pathpkg.Ext(path), ".md") {
		return "repair_design_doc"
	}
	return "repair_tracking_file"
}

func validationRepairNext(instructions string, paths ...string) []any {
	filtered := uniqueOrderedStrings(paths)
	next := make([]any, 0, len(filtered))
	for index, path := range filtered {
		if strings.TrimSpace(path) == "" {
			continue
		}
		next = append(next, editFileAction(index+1, repairActionForPath(path), instructions, path, false))
	}
	return next
}

func validationFailureWithState(state SpecProjection, message string, findings []infrastructure.ValidationFinding, next []any) error {
	state.Validation = validationProjection(findings)
	state = withSpecFocus(state, mergeValidationFocus(state.Focus, findings))
	return &Failure{
		Code:    "VALIDATION_FAILED",
		Message: message,
		State:   state,
		Next:    coalesceNextActions(next),
	}
}

func configValidationFailureWithState(state ConfigProjection, message string, findings []infrastructure.ValidationFinding, next []any) error {
	state.Validation = validationProjection(findings)
	state.Focus = mergeValidationFocus(state.Focus, findings)
	return &Failure{
		Code:    "VALIDATION_FAILED",
		Message: message,
		State:   state,
		Next:    coalesceNextActions(next),
	}
}

func charterValidationFailureWithState(state CharterProjection, message string, findings []infrastructure.ValidationFinding, next []any) error {
	state.Validation = validationProjection(findings)
	state.Focus = mergeValidationFocus(state.Focus, findings)
	return &Failure{
		Code:    "VALIDATION_FAILED",
		Message: message,
		State:   state,
		Next:    coalesceNextActions(next),
	}
}

func mergeValidationFocus(existing any, findings []infrastructure.ValidationFinding) map[string]any {
	focus := map[string]any{}
	if existingMap, ok := existing.(map[string]any); ok {
		for key, value := range existingMap {
			focus[key] = value
		}
	}
	focus["validation"] = map[string]any{"findings": findingsAsAny(findings)}
	return focus
}

func errorSeverityFindings(findings []infrastructure.ValidationFinding) []infrastructure.ValidationFinding {
	blocking := make([]infrastructure.ValidationFinding, 0, len(findings))
	for _, finding := range findings {
		if strings.EqualFold(strings.TrimSpace(finding.Severity), "error") {
			blocking = append(blocking, finding)
		}
	}
	return uniqueValidationFindings(blocking)
}

func (s *Service) blockingFindingsFromSnapshot(snapshot *infrastructure.RepoReadSnapshot) []infrastructure.ValidationFinding {
	if snapshot == nil {
		return nil
	}
	return errorSeverityFindings(s.repoReadStateFromSnapshot(snapshot).auditFindings)
}

func (s *Service) mutationValidationFindings(snapshot *infrastructure.RepoReadSnapshot, findings []infrastructure.ValidationFinding) []infrastructure.ValidationFinding {
	if blocking := s.blockingFindingsFromSnapshot(snapshot); len(blocking) > 0 {
		return blocking
	}
	return uniqueValidationFindings(findings)
}

func finalizeValidatedWrite[T any](
	s *Service,
	postSnapshot *infrastructure.RepoReadSnapshot,
	result map[string]any,
	stateBuilder func(*repoReadState) (T, error),
	nextBuilder func(T) []any,
	failureBuilder func([]infrastructure.ValidationFinding) error,
) (T, map[string]any, []any, error) {
	blocking := s.blockingFindingsFromSnapshot(postSnapshot)
	if len(blocking) > 0 {
		var zero T
		return zero, nil, nil, failureBuilder(blocking)
	}
	postState := s.repoReadStateFromSnapshot(postSnapshot)
	state, err := stateBuilder(postState)
	if err != nil {
		var zero T
		return zero, nil, nil, err
	}
	var next []any
	if nextBuilder != nil {
		next = nextBuilder(state)
	}
	return state, result, coalesceNextActions(next), nil
}

func (s *Service) finalizeSpecMutation(
	loaded *loadedSpec,
	updated *domain.TrackingFile,
	result map[string]any,
	focus any,
	nextBuilder func(SpecProjection) []any,
	extraWrites ...infrastructure.PlannedWrite,
) (SpecProjection, map[string]any, []any, error) {
	if err := updated.Validate(); err != nil {
		findings := validationFindingsFromMessage(loaded.relative, loaded.slug, err.Error())
		return SpecProjection{}, nil, nil, validationFailureWithState(
			loaded.state,
			fmt.Sprintf("Cannot apply the write because the resulting spec state is invalid: %v", err),
			findings,
			validationRepairNext(
				"Repair the invalid spec state before retrying the write.",
				loaded.relative,
				infrastructure.RelativeCharterPath(loaded.charterName),
			),
		)
	}

	mutation, err := s.registryStore().ApplyTrackingMutation(loaded.charterName, loaded.slug, updated, extraWrites)
	if err != nil {
		var mutationErr *infrastructure.TrackingMutationError
		if errors.As(err, &mutationErr) {
			return SpecProjection{}, nil, nil, validationFailureWithState(
				loaded.state,
				mutationErr.Message,
				mutationErr.Findings,
				validationRepairNext(
					"Repair the invalid repo state before retrying the write.",
					loaded.relative,
					infrastructure.RelativeCharterPath(loaded.charterName),
				),
			)
		}
		return SpecProjection{}, nil, nil, err
	}

	return finalizeValidatedWrite(
		s,
		mutation.Snapshot,
		result,
		func(repoState *repoReadState) (SpecProjection, error) {
			state, err := s.specProjectionFromRepoState(repoState, loaded.charterName+":"+loaded.slug)
			if err != nil {
				return SpecProjection{}, err
			}
			return withSpecFocus(state, focus), nil
		},
		nextBuilder,
		func(findings []infrastructure.ValidationFinding) error {
			return validationFailureWithState(
				loaded.state,
				"Cannot apply the write because the resulting repo state is invalid",
				findings,
				validationRepairNext(
					"Repair the invalid repo state before retrying the write.",
					loaded.relative,
					infrastructure.RelativeCharterPath(loaded.charterName),
				),
			)
		},
	)
}

func coalesceNextActions(next []any) []any {
	if next == nil {
		return []any{}
	}
	return next
}
