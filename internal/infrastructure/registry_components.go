package infrastructure

import (
	"errors"
	"fmt"
	"path"
	"slices"
	"sort"
	"strings"

	"github.com/aitoroses/specctl/internal/domain"
)

type projectConfigRegistry struct {
	workspace *Workspace
}

func newProjectConfigRegistry(workspace *Workspace) *projectConfigRegistry {
	return &projectConfigRegistry{workspace: workspace}
}

func (s *projectConfigRegistry) Load() (*ProjectConfig, error) {
	return LoadProjectConfig(s.workspace.SpecsDir())
}

type charterRegistry struct {
	workspace *Workspace
}

func newCharterRegistry(workspace *Workspace) *charterRegistry {
	return &charterRegistry{workspace: workspace}
}

func (s *charterRegistry) LoadStructure(charter string) (*domain.Charter, error) {
	return ReadCharterStructure(s.workspace.CharterPath(charter))
}

type charterMutationRegistry struct {
	charters *charterRegistry
	repoRead RepoReadAccess
}

func newCharterMutationRegistry(workspace *Workspace) *charterMutationRegistry {
	return &charterMutationRegistry{
		charters: newCharterRegistry(workspace),
		repoRead: NewRepoReadStore(workspace),
	}
}

func (s *charterMutationRegistry) Apply(charter *domain.Charter) (*CharterMutationResult, error) {
	current, err := s.repoRead.LoadRepoReadSnapshot()
	if err != nil {
		return nil, err
	}

	writes, err := plannedCharterWrites(s.charters.workspace, charter)
	if err != nil {
		return nil, err
	}
	snapshot, findings, err := previewRepoSnapshot(s.repoRead, writes)
	if err != nil {
		return nil, err
	}
	if len(findings) > 0 {
		return nil, &CharterMutationError{
			Code:         CharterMutationValidationFailed,
			Message:      "Cannot apply the write because the resulting repo state is invalid",
			Findings:     findings,
			Snapshot:     current,
			PostSnapshot: snapshot,
		}
	}

	if err := CommitWritesAtomically(writes); err != nil {
		return nil, err
	}
	return &CharterMutationResult{Snapshot: snapshot}, nil
}

type charterEntryMutationRegistry struct {
	charters *charterRegistry
	writes   *charterMutationRegistry
	repoRead RepoReadAccess
}

func newCharterEntryMutationRegistry(workspace *Workspace) *charterEntryMutationRegistry {
	return &charterEntryMutationRegistry{
		charters: newCharterRegistry(workspace),
		writes:   newCharterMutationRegistry(workspace),
		repoRead: NewRepoReadStore(workspace),
	}
}

func (s *charterEntryMutationRegistry) Apply(request CharterEntryMutationRequest) (*CharterEntryMutationResult, error) {
	charter, err := s.charters.LoadStructure(request.Charter)
	if err != nil {
		return nil, err
	}

	var createdGroup *domain.CharterGroup
	if charter.GroupByKey(request.Group) == nil {
		if request.GroupTitle == nil || request.GroupOrder == nil {
			missingFields := make([]string, 0, 2)
			if request.GroupTitle == nil {
				missingFields = append(missingFields, "group_title")
			}
			if request.GroupOrder == nil {
				missingFields = append(missingFields, "group_order")
			}
			snapshot, snapshotErr := s.repoRead.LoadRepoReadSnapshot()
			if snapshotErr != nil {
				return nil, snapshotErr
			}
			return nil, &CharterEntryMutationError{
				Code:          CharterEntryMutationGroupRequired,
				Message:       fmt.Sprintf("charter group %q does not exist", request.Group),
				Group:         request.Group,
				MissingFields: missingFields,
				Snapshot:      snapshot,
			}
		}
		group := domain.CharterGroup{
			Key:   request.Group,
			Title: strings.TrimSpace(*request.GroupTitle),
			Order: *request.GroupOrder,
		}
		if err := charter.EnsureGroup(group); err != nil {
			return nil, s.validationError(request, err)
		}
		createdGroup = &group
	}

	entry, err := domain.NewCharterSpecEntry(request.Slug, request.Group, request.Order, uniquePaths(request.DependsOn), strings.TrimSpace(request.Notes))
	if err != nil {
		return nil, s.validationError(request, err)
	}
	if err := charter.ReplaceSpecEntry(entry); err != nil {
		var cycleErr *domain.CharterCycleError
		if errors.As(err, &cycleErr) {
			snapshot, snapshotErr := s.repoRead.LoadRepoReadSnapshot()
			if snapshotErr != nil {
				return nil, snapshotErr
			}
			return nil, &CharterEntryMutationError{
				Code:     CharterEntryMutationCycle,
				Message:  err.Error(),
				Entry:    &entry,
				Cycle:    append([]string{}, cycleErr.Slugs...),
				Snapshot: snapshot,
			}
		}
		return nil, s.validationError(request, err)
	}

	mutation, err := s.writes.Apply(charter)
	if err != nil {
		var mutationErr *CharterMutationError
		if errors.As(err, &mutationErr) {
			return nil, &CharterEntryMutationError{
				Code:         CharterEntryMutationValidation,
				Message:      mutationErr.Message,
				Findings:     append([]ValidationFinding{}, mutationErr.Findings...),
				Snapshot:     mutationErr.Snapshot,
				PostSnapshot: mutationErr.PostSnapshot,
			}
		}
		return nil, err
	}
	return &CharterEntryMutationResult{
		Entry:        entry,
		CreatedGroup: createdGroup,
		Snapshot:     mutation.Snapshot,
	}, nil
}

func (s *charterEntryMutationRegistry) validationError(request CharterEntryMutationRequest, err error) error {
	if err == nil {
		return nil
	}

	var validationErr *domain.CharterValidationError
	if !errors.As(err, &validationErr) {
		return err
	}

	snapshot, snapshotErr := s.repoRead.LoadRepoReadSnapshot()
	if snapshotErr != nil {
		return snapshotErr
	}

	findings := make([]ValidationFinding, 0, len(validationErr.Messages))
	for _, message := range validationErr.Messages {
		findings = append(findings, ValidationFindingsFromMessages(
			message,
			RelativeCharterPath(request.Charter),
			request.Slug,
		)...)
	}

	return &CharterEntryMutationError{
		Code:     CharterEntryMutationValidation,
		Message:  err.Error(),
		Findings: findings,
		Snapshot: snapshot,
	}
}

type trackingRegistry struct {
	workspace *Workspace
	repoRead  RepoReadAccess
}

func newTrackingRegistry(workspace *Workspace) *trackingRegistry {
	return &trackingRegistry{
		workspace: workspace,
		repoRead:  NewRepoReadStore(workspace),
	}
}

func (s *trackingRegistry) Load(charter, slug string) (*domain.TrackingFile, error) {
	return ReadTrackingFile(s.workspace.TrackingPath(charter, slug))
}

func (s *trackingRegistry) LoadLenient(charter, slug string) (*domain.TrackingFile, []ValidationFinding, error) {
	return ReadTrackingFileLenient(s.workspace.TrackingPath(charter, slug))
}

func (s *trackingRegistry) Apply(charter, slug string, tracking *domain.TrackingFile, extraWrites []PlannedWrite) (*TrackingMutationResult, error) {
	if err := tracking.Validate(); err != nil {
		return nil, err
	}

	writes, err := s.plannedWrites(charter, slug, tracking, extraWrites)
	if err != nil {
		return nil, err
	}
	snapshot, findings, err := previewRepoSnapshot(s.repoRead, writes)
	if err != nil {
		return nil, err
	}
	if len(findings) > 0 {
		return nil, &TrackingMutationError{
			Message:  "Cannot apply the write because the resulting repo state is invalid",
			Findings: findings,
		}
	}
	if err := CommitWritesAtomically(writes); err != nil {
		return nil, err
	}
	return &TrackingMutationResult{Snapshot: snapshot}, nil
}

func (s *trackingRegistry) plannedWrites(charter, slug string, tracking *domain.TrackingFile, extraWrites []PlannedWrite) ([]PlannedWrite, error) {
	data, err := marshalYAML(tracking)
	if err != nil {
		return nil, err
	}

	writes := []PlannedWrite{{
		Path: s.workspace.TrackingPath(charter, slug),
		Data: data,
		Perm: 0644,
	}}
	if len(extraWrites) > 0 {
		writes = append(writes, extraWrites...)
	}
	return writes, nil
}

func (s *trackingRegistry) TagInUse(tag string) (bool, error) {
	trackingPaths, err := FindAllTrackingFiles(s.workspace.SpecsDir())
	if err != nil {
		return false, err
	}
	for _, path := range trackingPaths {
		tracking, readErr := ReadTrackingFile(path)
		if readErr != nil {
			return false, readErr
		}
		for _, requirement := range tracking.Requirements {
			for _, candidate := range requirement.Tags {
				if candidate == tag {
					return true, nil
				}
			}
		}
	}
	return false, nil
}

type configMutationRegistry struct {
	workspace *Workspace
	config    *projectConfigRegistry
	trackings *trackingRegistry
	repoRead  RepoReadAccess
}

func newConfigMutationRegistry(workspace *Workspace) *configMutationRegistry {
	return &configMutationRegistry{
		workspace: workspace,
		config:    newProjectConfigRegistry(workspace),
		trackings: newTrackingRegistry(workspace),
		repoRead:  NewRepoReadStore(workspace),
	}
}

func (s *configMutationRegistry) Apply(request ConfigMutationRequest) (*ConfigMutationResult, error) {
	snapshot, err := s.repoRead.LoadRepoReadSnapshot()
	if err != nil {
		return nil, err
	}
	config := snapshot.Config
	if config == nil {
		return nil, fmt.Errorf("project config is required")
	}

	switch request.Kind {
	case ConfigMutationAddTag:
		tag := strings.TrimSpace(request.Value)
		if slices.Contains(domain.SemanticRequirementTags(), tag) {
			return nil, &ConfigMutationError{
				Code:     ConfigMutationSemanticTagReserved,
				Message:  fmt.Sprintf("%q is a reserved semantic tag", tag),
				Mutation: request.Kind,
				Value:    tag,
				Snapshot: snapshot,
			}
		}
		if slices.Contains(config.GherkinTags, tag) {
			return nil, &ConfigMutationError{
				Code:     ConfigMutationTagExists,
				Message:  fmt.Sprintf("tag %q already exists", tag),
				Mutation: request.Kind,
				Value:    tag,
				Snapshot: snapshot,
			}
		}

		updated := cloneProjectConfigForMutation(config)
		updated.GherkinTags = append(updated.GherkinTags, tag)
		sort.Strings(updated.GherkinTags)
		return s.persistConfigMutation(request.Kind, tag, updated, snapshot)
	case ConfigMutationRemoveTag:
		tag := strings.TrimSpace(request.Value)
		if !slices.Contains(config.GherkinTags, tag) {
			return nil, &ConfigMutationError{
				Code:     ConfigMutationTagNotFound,
				Message:  fmt.Sprintf("tag %q does not exist", tag),
				Mutation: request.Kind,
				Value:    tag,
				Snapshot: snapshot,
			}
		}
		inUse, err := s.trackings.TagInUse(tag)
		if err != nil {
			return nil, err
		}
		if inUse {
			return nil, &ConfigMutationError{
				Code:     ConfigMutationTagInUse,
				Message:  fmt.Sprintf("tag %q is still used by at least one requirement", tag),
				Mutation: request.Kind,
				Value:    tag,
				Snapshot: snapshot,
			}
		}

		updated := cloneProjectConfigForMutation(config)
		filtered := make([]string, 0, len(updated.GherkinTags)-1)
		for _, candidate := range config.GherkinTags {
			if candidate != tag {
				filtered = append(filtered, candidate)
			}
		}
		updated.GherkinTags = filtered
		return s.persistConfigMutation(request.Kind, tag, updated, snapshot)
	case ConfigMutationAddPrefix:
		normalized, err := s.workspace.EnsureSourcePrefix(request.Value)
		if err != nil {
			return nil, &ConfigMutationError{
				Code:         ConfigMutationInvalidPath,
				Message:      err.Error(),
				Mutation:     request.Kind,
				Value:        strings.TrimSpace(request.Value),
				InvalidPaths: []string{strings.TrimSpace(request.Value)},
				Snapshot:     snapshot,
			}
		}
		if slices.Contains(config.SourcePrefixes, normalized) {
			return nil, &ConfigMutationError{
				Code:     ConfigMutationPrefixExists,
				Message:  fmt.Sprintf("prefix %q already exists", normalized),
				Mutation: request.Kind,
				Value:    normalized,
				Snapshot: snapshot,
			}
		}

		updated := cloneProjectConfigForMutation(config)
		updated.SourcePrefixes = append(updated.SourcePrefixes, normalized)
		sort.Strings(updated.SourcePrefixes)
		return s.persistConfigMutation(request.Kind, normalized, updated, snapshot)
	case ConfigMutationRemovePrefix:
		normalized, err := domain.NormalizeRepoDir(request.Value)
		if err != nil {
			return nil, &ConfigMutationError{
				Code:         ConfigMutationInvalidPath,
				Message:      err.Error(),
				Mutation:     request.Kind,
				Value:        strings.TrimSpace(request.Value),
				InvalidPaths: []string{strings.TrimSpace(request.Value)},
				Snapshot:     snapshot,
			}
		}
		if !slices.Contains(config.SourcePrefixes, normalized) {
			return nil, &ConfigMutationError{
				Code:     ConfigMutationPrefixNotFound,
				Message:  fmt.Sprintf("prefix %q does not exist", normalized),
				Mutation: request.Kind,
				Value:    normalized,
				Snapshot: snapshot,
			}
		}

		updated := cloneProjectConfigForMutation(config)
		filtered := make([]string, 0, len(updated.SourcePrefixes)-1)
		for _, candidate := range config.SourcePrefixes {
			if candidate != normalized {
				filtered = append(filtered, candidate)
			}
		}
		updated.SourcePrefixes = filtered
		return s.persistConfigMutation(request.Kind, normalized, updated, snapshot)
	default:
		return nil, fmt.Errorf("unsupported config mutation %q", request.Kind)
	}
}

func (s *configMutationRegistry) persistConfigMutation(kind ConfigMutationKind, value string, config *ProjectConfig, current *RepoReadSnapshot) (*ConfigMutationResult, error) {
	if err := config.Validate(); err != nil {
		findings := ValidationFindingsFromMessages(err.Error(), path.Join(".specs", "specctl.yaml"), "config")
		return nil, &ConfigMutationError{
			Code:               ConfigMutationValidationFailed,
			Message:            fmt.Sprintf("Cannot apply the config write because the resulting config state is invalid: %v", err),
			Mutation:           kind,
			Value:              value,
			ValidationFindings: findings,
			Snapshot:           current,
		}
	}

	writes, err := plannedConfigWrites(s.workspace, config)
	if err != nil {
		return nil, err
	}
	updatedSnapshot, findings, err := previewRepoSnapshot(s.repoRead, writes)
	if err != nil {
		return nil, err
	}
	if len(findings) > 0 {
		return nil, &ConfigMutationError{
			Code:               ConfigMutationValidationFailed,
			Message:            "Cannot apply the write because the resulting repo state is invalid",
			Mutation:           kind,
			Value:              value,
			ValidationFindings: findings,
			Snapshot:           current,
			PostSnapshot:       updatedSnapshot,
		}
	}

	if err := CommitWritesAtomically(writes); err != nil {
		return nil, err
	}
	return &ConfigMutationResult{
		Mutation: kind,
		Value:    value,
		Snapshot: updatedSnapshot,
	}, nil
}

func cloneProjectConfigForMutation(config *ProjectConfig) *ProjectConfig {
	if config == nil {
		return nil
	}
	return &ProjectConfig{
		GherkinTags:    append([]string{}, config.GherkinTags...),
		SourcePrefixes: append([]string{}, config.SourcePrefixes...),
		Formats:        CloneFormatsForProjection(config.Formats),
	}
}

type specCreatePlanner struct {
	workspace *Workspace
	repoRead  RepoReadAccess
}

func newSpecCreatePlanner(workspace *Workspace) *specCreatePlanner {
	return &specCreatePlanner{
		workspace: workspace,
		repoRead:  NewRepoReadStore(workspace),
	}
}

func (s *specCreatePlanner) Prepare(request SpecCreatePlanRequest) (SpecCreatePlan, error) {
	scope, err := s.workspace.NormalizeScopePaths(request.Scope)
	if err != nil {
		return SpecCreatePlan{}, &SpecCreatePlanError{
			Code:         SpecCreateInvalidPath,
			Message:      fmt.Sprintf("scope: %v", err),
			InvalidPaths: append([]string{}, request.Scope...),
		}
	}

	docPath, err := s.workspace.NormalizeDesignDocPath(request.Doc)
	if err != nil {
		return SpecCreatePlan{}, &SpecCreatePlanError{
			Code:         SpecCreateInvalidPath,
			Message:      fmt.Sprintf("design document: %v", err),
			DocPath:      strings.TrimSpace(request.Doc),
			InvalidPaths: []string{strings.TrimSpace(request.Doc)},
		}
	}

	selectedFormat, err := AutoSelectFormat(request.Config, docPath)
	if err != nil {
		return SpecCreatePlan{}, err
	}

	existingDoc, err := s.workspace.InspectCreateDesignDoc(docPath, request.Config)
	if err != nil {
		return SpecCreatePlan{}, err
	}

	mutation, err := BuildDesignDocMutation(docPath, existingDoc, request.Slug, request.Charter, selectedFormat)
	if err != nil {
		return SpecCreatePlan{}, err
	}

	return SpecCreatePlan{
		Scope:    scope,
		DocPath:  docPath,
		Mutation: mutation,
	}, nil
}

func (s *specCreatePlanner) Apply(charter *domain.Charter, tracking *domain.TrackingFile, mutation DesignDocMutation) (*SpecCreateMutationResult, error) {
	writes, err := PlanSpecCreateWrites(s.workspace, charter, tracking, mutation)
	if err != nil {
		return nil, err
	}
	snapshot, findings, err := previewRepoSnapshot(s.repoRead, writes)
	if err != nil {
		return nil, err
	}
	if len(findings) > 0 {
		return nil, &CharterMutationError{
			Code:     CharterMutationValidationFailed,
			Message:  "Cannot apply the write because the resulting repo state is invalid",
			Findings: findings,
		}
	}
	if err := CommitWritesAtomically(writes); err != nil {
		return nil, err
	}

	return &SpecCreateMutationResult{
		Snapshot: snapshot,
	}, nil
}

func plannedConfigWrites(workspace *Workspace, config *ProjectConfig) ([]PlannedWrite, error) {
	data, err := marshalYAML(config)
	if err != nil {
		return nil, err
	}
	return []PlannedWrite{{
		Path: workspace.ConfigPath(),
		Data: data,
		Perm: 0644,
	}}, nil
}

func plannedCharterWrites(workspace *Workspace, charter *domain.Charter) ([]PlannedWrite, error) {
	data, err := marshalYAML(charter)
	if err != nil {
		return nil, err
	}
	return []PlannedWrite{{
		Path: workspace.CharterPath(charter.Name),
		Data: data,
		Perm: 0644,
	}}, nil
}

func previewRepoSnapshot(repoRead RepoReadAccess, writes []PlannedWrite) (*RepoReadSnapshot, []ValidationFinding, error) {
	snapshot, err := repoRead.LoadRepoReadSnapshotWithWrites(writes)
	if err != nil {
		return nil, nil, err
	}
	return snapshot, repoSnapshotBlockingFindings(snapshot), nil
}

func repoSnapshotBlockingFindings(snapshot *RepoReadSnapshot) []ValidationFinding {
	if snapshot == nil {
		return nil
	}
	evaluation := EvaluateRepoSnapshot(snapshot)
	blocking := make([]ValidationFinding, 0, len(evaluation.AuditFindings))
	for _, finding := range evaluation.AuditFindings {
		if strings.EqualFold(strings.TrimSpace(finding.Severity), "error") {
			blocking = append(blocking, finding)
		}
	}
	return uniqueFindings(blocking)
}
