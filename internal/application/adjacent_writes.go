package application

import (
	"errors"
	"fmt"
	"slices"
	"sort"
	"strings"

	"github.com/aitoroses/specctl/internal/domain"
	"github.com/aitoroses/specctl/internal/infrastructure"
)

type CharterAddSpecRequest struct {
	Charter    string
	Slug       string
	Group      string
	GroupTitle *string
	GroupOrder *int
	Order      int
	DependsOn  []string
	Notes      string
}

func (s *Service) AddSpecToCharter(request CharterAddSpecRequest) (CharterProjection, map[string]any, []any, error) {
	charterExists, err := s.pathAdapter().CharterExists(request.Charter)
	if err != nil {
		return CharterProjection{}, nil, nil, fmt.Errorf("checking charter path: %w", err)
	}
	if !charterExists {
		return CharterProjection{}, nil, nil, &Failure{
			Code:    "CHARTER_NOT_FOUND",
			Message: fmt.Sprintf("charter %q does not exist", request.Charter),
			State: MissingCharterContext{
				Charter:      request.Charter,
				TrackingFile: infrastructure.RelativeCharterPath(request.Charter),
			},
			Next: []any{},
		}
	}

	trackingExists, err := s.pathAdapter().TrackingExists(request.Charter, request.Slug)
	if err != nil {
		return CharterProjection{}, nil, nil, fmt.Errorf("checking tracking path: %w", err)
	}
	if !trackingExists {
		return CharterProjection{}, nil, nil, &Failure{
			Code:    "SPEC_NOT_FOUND",
			Message: fmt.Sprintf("spec %q does not exist", request.Charter+":"+request.Slug),
			State: MissingSpecContext{
				Target:        request.Charter + ":" + request.Slug,
				TrackingFile:  infrastructure.RelativeTrackingPath(s.repoRoot, request.Charter, request.Slug),
				CharterExists: true,
			},
			Next: []any{},
		}
	}

	if _, err := s.registryStore().LoadTracking(request.Charter, request.Slug); err != nil {
		_, findings, lenientErr := s.registryStore().LoadTrackingLenient(request.Charter, request.Slug)
		if lenientErr == nil && len(findings) > 0 {
			failure, failureErr := s.charterAddSpecValidationFailure(
				request.Charter,
				request.Slug,
				"Cannot apply the charter write because the stored spec state is invalid",
				findings,
			)
			if failureErr != nil {
				return CharterProjection{}, nil, nil, failureErr
			}
			return CharterProjection{}, nil, nil, failure
		}
		return CharterProjection{}, nil, nil, err
	}
	preState, err := s.canonicalCharterProjection(request.Charter)
	if err != nil {
		return CharterProjection{}, nil, nil, err
	}

	mutation, err := s.registryStore().ApplyCharterEntryMutation(infrastructure.CharterEntryMutationRequest{
		Charter:    request.Charter,
		Slug:       request.Slug,
		Group:      request.Group,
		GroupTitle: request.GroupTitle,
		GroupOrder: request.GroupOrder,
		Order:      request.Order,
		DependsOn:  request.DependsOn,
		Notes:      request.Notes,
	})
	if err != nil {
		var mutationErr *infrastructure.CharterEntryMutationError
		if errors.As(err, &mutationErr) {
			state, stateErr := s.charterProjectionFromRepoState(s.repoReadStateFromSnapshot(mutationErr.Snapshot), request.Charter)
			if stateErr != nil {
				return CharterProjection{}, nil, nil, stateErr
			}
			if mutationErr.Code == infrastructure.CharterEntryMutationGroupRequired {
				state.Focus = map[string]any{"input": map[string]any{"missing_fields": mutationErr.MissingFields}}
				return CharterProjection{}, nil, nil, &Failure{
					Code:    "GROUP_REQUIRED",
					Message: mutationErr.Error(),
					State:   state,
					Next:    []any{},
				}
			}
			if mutationErr.Code == infrastructure.CharterEntryMutationValidation {
				findings := s.mutationValidationFindings(mutationErr.PostSnapshot, mutationErr.Findings)
				return CharterProjection{}, nil, nil, charterValidationFailureWithState(state, mutationErr.Error(), findings, nil)
			}
			if mutationErr.Code == infrastructure.CharterEntryMutationCycle {
				focus := map[string]any{"entry": domain.CharterSpecEntry{}}
				if mutationErr.Entry != nil {
					focus["entry"] = *mutationErr.Entry
				}
				if len(mutationErr.Cycle) > 0 {
					focus["cycle"] = append([]string{}, mutationErr.Cycle...)
				}
				state.Focus = focus
				return CharterProjection{}, nil, nil, &Failure{
					Code:    "CHARTER_CYCLE",
					Message: mutationErr.Error(),
					State:   state,
					Next:    []any{},
				}
			}
		}
		return CharterProjection{}, nil, nil, err
	}
	result := map[string]any{
		"kind":          "charter_entry",
		"entry":         mutation.Entry,
		"created_group": mutation.CreatedGroup,
	}
	return finalizeValidatedWrite(
		s,
		mutation.Snapshot,
		result,
		func(repoState *repoReadState) (CharterProjection, error) {
			return s.charterProjectionFromRepoState(repoState, request.Charter)
		},
		nil,
		func(findings []infrastructure.ValidationFinding) error {
			return charterValidationFailureWithState(
				preState,
				"Cannot apply the write because the resulting repo state is invalid",
				findings,
				nil,
			)
		},
	)
}

func (s *Service) RemoveSpecFromCharter(charterName, slug string) (CharterProjection, map[string]any, []any, error) {
	charterExists, err := s.pathAdapter().CharterExists(charterName)
	if err != nil {
		return CharterProjection{}, nil, nil, fmt.Errorf("checking charter path: %w", err)
	}
	if !charterExists {
		return CharterProjection{}, nil, nil, &Failure{
			Code:    "CHARTER_NOT_FOUND",
			Message: fmt.Sprintf("charter %q does not exist", charterName),
			State: MissingCharterContext{
				Charter:      charterName,
				TrackingFile: infrastructure.RelativeCharterPath(charterName),
			},
			Next: []any{},
		}
	}

	charter, err := s.registryStore().LoadCharterStructure(charterName)
	if err != nil {
		return CharterProjection{}, nil, nil, err
	}

	if charter.SpecBySlug(slug) == nil {
		return CharterProjection{}, nil, nil, &Failure{
			Code:    "SPEC_NOT_FOUND",
			Message: fmt.Sprintf("spec %q is not registered in charter %q", slug, charterName),
			State: MissingSpecContext{
				Target:        charterName + ":" + slug,
				TrackingFile:  infrastructure.RelativeTrackingPath(s.repoRoot, charterName, slug),
				CharterExists: true,
			},
			Next: []any{},
		}
	}

	dependents := make([]string, 0)
	for _, entry := range charter.Specs {
		if entry.Slug == slug {
			continue
		}
		if slices.Contains(entry.DependsOn, slug) {
			dependents = append(dependents, entry.Slug)
		}
	}
	sort.Strings(dependents)
	if len(dependents) > 0 {
		state, stateErr := s.canonicalCharterProjection(charterName)
		if stateErr != nil {
			return CharterProjection{}, nil, nil, stateErr
		}
		state.Focus = map[string]any{"dependents": dependents}
		return CharterProjection{}, nil, nil, &Failure{
			Code:    "CHARTER_DEPENDENCY_EXISTS",
			Message: fmt.Sprintf("cannot remove %s because other charter entries depend on it", slug),
			State:   state,
			Next:    []any{},
		}
	}

	updatedSpecs := make([]domain.CharterSpecEntry, 0, len(charter.Specs)-1)
	for _, entry := range charter.Specs {
		if entry.Slug != slug {
			updatedSpecs = append(updatedSpecs, entry)
		}
	}
	charter.Specs = updatedSpecs
	preState, err := s.canonicalCharterProjection(charterName)
	if err != nil {
		return CharterProjection{}, nil, nil, err
	}
	mutation, err := s.registryStore().ApplyCharterMutation(charter)
	if err != nil {
		var mutationErr *infrastructure.CharterMutationError
		if errors.As(err, &mutationErr) {
			findings := s.mutationValidationFindings(mutationErr.PostSnapshot, mutationErr.Findings)
			return CharterProjection{}, nil, nil, charterValidationFailureWithState(
				preState,
				mutationErr.Message,
				findings,
				nil,
			)
		}
		return CharterProjection{}, nil, nil, err
	}
	return finalizeValidatedWrite(
		s,
		mutation.Snapshot,
		map[string]any{
			"kind":         "charter_entry",
			"removed_slug": slug,
		},
		func(repoState *repoReadState) (CharterProjection, error) {
			return s.charterProjectionFromRepoState(repoState, charterName)
		},
		nil,
		func(findings []infrastructure.ValidationFinding) error {
			return charterValidationFailureWithState(
				preState,
				"Cannot apply the write because the resulting repo state is invalid",
				findings,
				nil,
			)
		},
	)
}

func (s *Service) charterAddSpecValidationFailure(charterName, slug, message string, findings []infrastructure.ValidationFinding) (*Failure, error) {
	repoState, err := s.loadRepoReadState()
	if err != nil {
		return nil, err
	}

	state, err := s.charterProjectionFromRepoState(repoState, charterName)
	if err != nil {
		return nil, err
	}
	state.Focus = map[string]any{
		"validation": map[string]any{
			"findings": findingsAsAny(uniqueValidationFindings(findings)),
		},
	}
	return &Failure{
		Code:    "VALIDATION_FAILED",
		Message: message,
		State:   state,
		Next: validationRepairNext(
			"Repair the malformed tracking file before changing charter membership.",
			infrastructure.RelativeTrackingPath(s.repoRoot, charterName, slug),
			infrastructure.RelativeCharterPath(charterName),
		),
	}, nil
}

func (s *Service) AddConfigTag(tag string) (ConfigProjection, map[string]any, []any, error) {
	return s.applyConfigMutation(infrastructure.ConfigMutationRequest{
		Kind:  infrastructure.ConfigMutationAddTag,
		Value: tag,
	})
}

func (s *Service) RemoveConfigTag(tag string) (ConfigProjection, map[string]any, []any, error) {
	return s.applyConfigMutation(infrastructure.ConfigMutationRequest{
		Kind:  infrastructure.ConfigMutationRemoveTag,
		Value: tag,
	})
}

func (s *Service) AddConfigPrefix(prefix string) (ConfigProjection, map[string]any, []any, error) {
	return s.applyConfigMutation(infrastructure.ConfigMutationRequest{
		Kind:  infrastructure.ConfigMutationAddPrefix,
		Value: prefix,
	})
}

func (s *Service) RemoveConfigPrefix(prefix string) (ConfigProjection, map[string]any, []any, error) {
	return s.applyConfigMutation(infrastructure.ConfigMutationRequest{
		Kind:  infrastructure.ConfigMutationRemovePrefix,
		Value: prefix,
	})
}

func (s *Service) ReadHook(stdin string) (HookProjection, error) {
	snapshot, err := s.repoReadAdapter().LoadRepoReadSnapshot()
	if err != nil {
		return HookProjection{}, err
	}
	repoState := s.repoReadStateFromSnapshot(snapshot)
	trackingByKey := make(map[string]*domain.TrackingFile, len(repoState.trackings))
	for key, state := range repoState.trackings {
		trackingByKey[key] = state.tracking
	}

	prepared, err := s.repoReadAdapter().PrepareHookInputs(stdin, snapshot)
	if err != nil {
		return HookProjection{}, err
	}

	unmatched := make([]string, 0)
	findings := make([]any, 0)
	affected := make(map[string]*HookAffectedSpec)

	for _, preparedEntry := range prepared.Entries {
		if preparedEntry.Managed != nil {
			s.applyManagedHookEntry(trackingByKey, affected, preparedEntry.Managed)
			continue
		}
		if preparedEntry.Ownership == nil || preparedEntry.Ownership.Resolution != "matched" || preparedEntry.Ownership.GoverningSpec == nil {
			unmatched = append(unmatched, preparedEntry.Path)
			findings = append(findings, infrastructure.ValidationFinding{
				Code:     "UNOWNED_SOURCE_FILE",
				Severity: "error",
				Message:  fmt.Sprintf("no governing spec owns %s", preparedEntry.Path),
				Path:     preparedEntry.Path,
			})
			continue
		}

		key := preparedEntry.Ownership.GoverningSpec.Charter + ":" + preparedEntry.Ownership.GoverningSpec.Slug
		affectedEntry := affected[key]
		if affectedEntry == nil {
			tracking := findTrackingByTarget(trackingByKey, key)
			affectedEntry = &HookAffectedSpec{
				Slug:               preparedEntry.Ownership.GoverningSpec.Slug,
				Charter:            preparedEntry.Ownership.GoverningSpec.Charter,
				Status:             tracking.Status,
				TrackingFile:       preparedEntry.Ownership.GoverningSpec.TrackingFile,
				TrackingFileStaged: false,
				DesignDoc:          tracking.Documents.Primary,
				DesignDocStaged:    false,
				MatchedFiles:       []string{},
			}
			affected[key] = affectedEntry
		}
		if preparedEntry.Path == affectedEntry.TrackingFile {
			affectedEntry.TrackingFileStaged = true
			continue
		}
		if preparedEntry.Path == affectedEntry.DesignDoc {
			affectedEntry.DesignDocStaged = true
			continue
		}
		affectedEntry.MatchedFiles = append(affectedEntry.MatchedFiles, preparedEntry.Path)
	}

	affectedSpecs := make([]HookAffectedSpec, 0, len(affected))
	for _, entry := range affected {
		entry.MatchedFiles = uniqueOrderedStrings(entry.MatchedFiles)
		affectedSpecs = append(affectedSpecs, *entry)
	}
	sort.Slice(affectedSpecs, func(i, j int) bool {
		if affectedSpecs[i].Charter != affectedSpecs[j].Charter {
			return affectedSpecs[i].Charter < affectedSpecs[j].Charter
		}
		return affectedSpecs[i].Slug < affectedSpecs[j].Slug
	})

	validation := validProjection()
	if len(findings) > 0 {
		validation.Valid = false
		validation.Findings = findings
	}

	return HookProjection{
		InputFiles:      prepared.InputFiles,
		ConsideredFiles: prepared.ConsideredFiles,
		IgnoredFiles:    prepared.IgnoredFiles,
		UnmatchedFiles:  uniqueOrderedStrings(unmatched),
		AffectedSpecs:   affectedSpecs,
		Validation:      validation,
	}, nil
}

func (s *Service) applyManagedHookEntry(trackingByKey map[string]*domain.TrackingFile, affected map[string]*HookAffectedSpec, managed *infrastructure.ManagedHookClassification) {
	for _, target := range managed.AffectedTargets {
		charter, slug, ok := strings.Cut(target, ":")
		if !ok {
			continue
		}
		entry := ensureHookAffectedSpec(s.repoRoot, affected, charter, slug, trackingByKey[target])
		if managed.Kind == "tracking" {
			entry.TrackingFileStaged = true
		}
	}
}

func (s *Service) configMutationState(repoState *repoReadState, config *infrastructure.ProjectConfig, mutation, value string, configFindings []infrastructure.ValidationFinding) (ConfigProjection, error) {
	state := newConfigProjection(s.repoRoot, config, uniqueValidationFindings(repoState.auditFindings))
	state.Focus = map[string]any{
		"config_mutation": map[string]any{"kind": configMutationOutputKind(mutation), "value": value},
	}
	if len(configFindings) > 0 {
		state.Focus.(map[string]any)["validation"] = map[string]any{"findings": findingsAsAny(uniqueValidationFindings(configFindings))}
	}
	return state, nil
}

func configMutationOutputKind(mutation string) string {
	return strings.ReplaceAll(mutation, "-", "_")
}

func (s *Service) applyConfigMutation(request infrastructure.ConfigMutationRequest) (ConfigProjection, map[string]any, []any, error) {
	preRepoState, err := s.loadRepoReadState()
	if err != nil {
		return ConfigProjection{}, nil, nil, err
	}
	result, err := s.registryStore().ApplyConfigMutation(request)
	if err != nil {
		var mutationErr *infrastructure.ConfigMutationError
		if errors.As(err, &mutationErr) {
			findings := s.mutationValidationFindings(mutationErr.PostSnapshot, mutationErr.ValidationFindings)
			state, stateErr := s.configMutationState(
				s.repoReadStateFromSnapshot(mutationErr.Snapshot),
				mutationErr.Snapshot.Config,
				string(mutationErr.Mutation),
				mutationErr.Value,
				findings,
			)
			if stateErr != nil {
				return ConfigProjection{}, nil, nil, stateErr
			}
			if len(mutationErr.InvalidPaths) > 0 {
				state.Focus.(map[string]any)["invalid_paths"] = append([]string{}, mutationErr.InvalidPaths...)
			}

			code := "VALIDATION_FAILED"
			switch mutationErr.Code {
			case infrastructure.ConfigMutationSemanticTagReserved:
				code = "SEMANTIC_TAG_RESERVED"
			case infrastructure.ConfigMutationTagExists:
				code = "TAG_EXISTS"
			case infrastructure.ConfigMutationTagNotFound:
				code = "TAG_NOT_FOUND"
			case infrastructure.ConfigMutationTagInUse:
				code = "TAG_IN_USE"
			case infrastructure.ConfigMutationPrefixExists:
				code = "PREFIX_EXISTS"
			case infrastructure.ConfigMutationPrefixNotFound:
				code = "PREFIX_NOT_FOUND"
			case infrastructure.ConfigMutationInvalidPath:
				if mutationErr.Mutation == infrastructure.ConfigMutationRemovePrefix {
					code = "PREFIX_NOT_FOUND"
				} else {
					code = "INVALID_PATH"
				}
			}
			return ConfigProjection{}, nil, nil, &Failure{
				Code:    code,
				Message: mutationErr.Error(),
				State:   state,
				Next:    []any{},
			}
		}
		return ConfigProjection{}, nil, nil, err
	}

	payload := map[string]any{
		"kind":     "config",
		"mutation": string(result.Mutation),
	}
	switch result.Mutation {
	case infrastructure.ConfigMutationAddTag, infrastructure.ConfigMutationRemoveTag:
		payload["tag"] = result.Value
	case infrastructure.ConfigMutationAddPrefix, infrastructure.ConfigMutationRemovePrefix:
		payload["prefix"] = result.Value
	}
	return finalizeValidatedWrite(
		s,
		result.Snapshot,
		payload,
		func(repoState *repoReadState) (ConfigProjection, error) {
			return s.configMutationState(repoState, repoState.config, string(result.Mutation), result.Value, nil)
		},
		nil,
		func(findings []infrastructure.ValidationFinding) error {
			preState, stateErr := s.configMutationState(preRepoState, preRepoState.config, string(result.Mutation), result.Value, nil)
			if stateErr != nil {
				return stateErr
			}
			return configValidationFailureWithState(
				preState,
				"Cannot apply the write because the resulting repo state is invalid",
				findings,
				nil,
			)
		},
	)
}

func findTrackingByTarget(trackings map[string]*domain.TrackingFile, target string) *domain.TrackingFile {
	if tracking := trackings[target]; tracking != nil {
		return tracking
	}
	charter, slug, _ := strings.Cut(target, ":")
	return &domain.TrackingFile{Slug: slug, Charter: charter}
}

func ensureHookAffectedSpec(repoRoot string, affected map[string]*HookAffectedSpec, charter, slug string, tracking *domain.TrackingFile) *HookAffectedSpec {
	key := charter + ":" + slug
	if existing := affected[key]; existing != nil {
		return existing
	}
	if tracking == nil {
		tracking = &domain.TrackingFile{Slug: slug, Charter: charter}
	}
	entry := &HookAffectedSpec{
		Slug:               slug,
		Charter:            charter,
		Status:             tracking.Status,
		TrackingFile:       infrastructure.RelativeTrackingPath(repoRoot, charter, slug),
		TrackingFileStaged: false,
		DesignDoc:          tracking.Documents.Primary,
		DesignDocStaged:    false,
		MatchedFiles:       []string{},
	}
	affected[key] = entry
	return entry
}
