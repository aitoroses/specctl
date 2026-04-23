package application

import (
	"errors"
	"fmt"
	"path"
	"sort"
	"strings"
	"time"

	"github.com/aitoroses/specctl/internal/domain"
	"github.com/aitoroses/specctl/internal/infrastructure"
)

type Service struct {
	repoRoot    string
	specsDir    string
	paths       infrastructure.PathAccess
	registry    infrastructure.RegistryAccess
	repoReads   infrastructure.RepoReadAccess
	checkpoints infrastructure.CheckpointAccess
	now         func() time.Time
}

type MissingSpecContext struct {
	Target        string `json:"target"`
	TrackingFile  any    `json:"tracking_file"`
	CharterExists bool   `json:"charter_exists"`
	Focus         any    `json:"focus,omitempty"`
}

type MissingCharterContext struct {
	Charter      string `json:"charter"`
	TrackingFile any    `json:"tracking_file"`
	Focus        any    `json:"focus,omitempty"`
}

type CharterCreateRequest struct {
	Charter     string
	Title       string
	Description string
	Groups      []domain.CharterGroup
}

type SpecCreateTemplateSeed struct {
	Target               string
	Group                string
	IncludeGroupField    bool
	IncludeGroupMetadata bool
	ChooseWhen           string
	Scope                string
	Priority             int
	Instructions         string
}

func OpenFromWorkingDir() (*Service, error) {
	adapters, err := infrastructure.OpenServiceAdaptersFromWorkingDir()
	if err != nil {
		return nil, err
	}
	return newServiceFromAdapters(adapters), nil
}

func (s *Service) ReadConfig() (ConfigProjection, error) {
	repoState, err := s.loadRepoReadState()
	if err != nil {
		return ConfigProjection{}, err
	}
	return newConfigProjection(s.repoRoot, repoState.config, repoState.auditFindings), nil
}

func (s *Service) projectSpec(tracking *domain.TrackingFile, charter *domain.Charter, config *infrastructure.ProjectConfig, findings []infrastructure.ValidationFinding) (SpecProjection, error) {
	inputs := s.repoReadAdapter().ResolveSpecProjectionInputs(tracking, config)
	return newSpecProjection(s.repoRoot, tracking, charter, findings, inputs)
}

func (s *Service) ReadContext(target, file string) (any, []any, error) {
	if file != "" {
		return s.readFileContext(file)
	}
	if target == "" {
		return s.readRegistryContext()
	}
	if strings.Contains(target, ":") {
		return s.readSpecContext(target)
	}
	return s.readCharterContext(target)
}

func (s *Service) CreateCharter(request CharterCreateRequest) (CharterProjection, map[string]any, []any, error) {
	exists, err := s.pathAdapter().CharterExists(request.Charter)
	if err != nil {
		return CharterProjection{}, nil, nil, fmt.Errorf("checking charter path: %w", err)
	}
	if exists {
		return CharterProjection{}, nil, nil, ErrCharterExists{Charter: request.Charter}
	}

	charter := domain.Charter{
		Name:        request.Charter,
		Title:       strings.TrimSpace(request.Title),
		Description: strings.TrimSpace(request.Description),
		Groups:      append([]domain.CharterGroup{}, request.Groups...),
		Specs:       []domain.CharterSpecEntry{},
	}
	if err := charter.Validate(); err != nil {
		findings := validationFindingsFromMessage(infrastructure.RelativeCharterPath(request.Charter), "", err.Error())
		return CharterProjection{}, nil, nil, s.charterCreateValidationFailure(
			&charter,
			"Cannot create the charter because the requested metadata is invalid",
			findings,
		)
	}

	mutation, err := s.registryStore().ApplyCharterMutation(&charter)
	if err != nil {
		var mutationErr *infrastructure.CharterMutationError
		if errors.As(err, &mutationErr) {
			if len(s.blockingFindingsFromSnapshot(mutationErr.PostSnapshot)) > 0 {
				findings := s.mutationValidationFindings(mutationErr.PostSnapshot, mutationErr.Findings)
				return CharterProjection{}, nil, nil, s.charterCreateValidationFailure(
					&charter,
					mutationErr.Message,
					findings,
				)
			}
			return CharterProjection{}, nil, nil, s.charterCreateValidationFailure(
				&charter,
				mutationErr.Message,
				s.mutationValidationFindings(mutationErr.PostSnapshot, mutationErr.Findings),
			)
		}
		return CharterProjection{}, nil, nil, err
	}
	result := map[string]any{
		"kind":           "charter",
		"tracking_file":  infrastructure.RelativeCharterPath(request.Charter),
		"created_groups": append([]domain.CharterGroup{}, request.Groups...),
	}
	next := buildCreateSpecNext(buildCreateSpecSeedForCharter(
		request.Charter+":<slug>",
		request.Groups,
		1,
		"Create the first spec in the new charter.",
		"Create the first spec in the new charter and define its first group.",
	))
	return finalizeValidatedWrite(
		s,
		mutation.Snapshot,
		result,
		func(repoState *repoReadState) (CharterProjection, error) {
			return s.charterProjectionFromRepoState(repoState, request.Charter)
		},
		func(CharterProjection) []any { return next },
		func(findings []infrastructure.ValidationFinding) error {
			return s.charterCreateValidationFailure(
				&charter,
				"Cannot apply the write because the resulting repo state is invalid",
				findings,
			)
		},
	)
}

func (s *Service) readRegistryContext() (RegistryProjection, []any, error) {
	repoState, err := s.loadRepoReadState()
	if err != nil {
		return RegistryProjection{}, nil, err
	}
	charters := make([]*domain.Charter, 0, len(repoState.charters))
	charterFindings := make(map[string][]infrastructure.ValidationFinding, len(repoState.charters))
	for name, charterState := range repoState.charters {
		charters = append(charters, charterState.charter)
		charterFindings[name] = append([]infrastructure.ValidationFinding{}, charterState.findings...)
	}
	sort.Slice(charters, func(i, j int) bool {
		return charters[i].Name < charters[j].Name
	})
	trackingBySpec := make(map[string]*domain.TrackingFile, len(repoState.trackings))
	specFindings := make(map[string][]infrastructure.ValidationFinding, len(repoState.trackings))
	for key, trackingState := range repoState.trackings {
		trackingBySpec[key] = trackingState.tracking
		specFindings[key] = repoState.specValidation(key)
	}
	projection := buildRegistryProjection(s.repoRoot, repoState.config, charters, trackingBySpec, repoState.configReadFindings, repoState.auditFindings, charterFindings, specFindings)
	projection.Focus = s.buildRegistryContextFocus(repoState)
	return projection, []any{}, nil
}

func (s *Service) readCharterContext(name string) (any, []any, error) {
	exists, err := s.pathAdapter().CharterExists(name)
	if err != nil {
		return nil, nil, fmt.Errorf("checking charter path: %w", err)
	}
	if !exists {
		return MissingCharterContext{
			Charter:      name,
			TrackingFile: nil,
			Focus:        map[string]any{"lookup": map[string]any{"reason": "charter_not_found"}},
		}, buildCreateCharterChoiceNext(name), nil
	}

	repoState, err := s.loadRepoReadState()
	if err != nil {
		return nil, nil, err
	}
	charterState := repoState.charterState(name)
	if charterState == nil {
		return MissingCharterContext{
			Charter:      name,
			TrackingFile: nil,
			Focus:        map[string]any{"lookup": map[string]any{"reason": "charter_not_found"}},
		}, buildCreateCharterChoiceNext(name), nil
	}
	state, err := s.charterProjectionFromRepoState(repoState, name)
	if err != nil {
		return nil, nil, err
	}
	state.Focus = s.buildCharterContextFocus(repoState, name)
	return state, []any{}, nil
}

type advisoryTarget struct {
	target                string
	title                 string
	charter               string
	deferred              int
	scopeDriftStatus      string
	driftSource           *string
	reviewRequired        bool
	correctnessBlocker    bool
	housekeepingCandidate bool
}

func (s *Service) buildRegistryContextFocus(repoState *repoReadState) any {
	targets := s.collectContextAdvisoryTargets(repoState, "")
	return buildContextAdvisoryFocus(targets, "")
}

func (s *Service) buildCharterContextFocus(repoState *repoReadState, charterName string) any {
	targets := s.collectContextAdvisoryTargets(repoState, charterName)
	return buildContextAdvisoryFocus(targets, charterName)
}

func (s *Service) collectContextAdvisoryTargets(repoState *repoReadState, charterName string) []advisoryTarget {
	targets := make([]advisoryTarget, 0)
	for key, trackingState := range repoState.trackings {
		if charterName != "" && trackingState.tracking.Charter != charterName {
			continue
		}
		charter := (*domain.Charter)(nil)
		if charterState := repoState.charterState(trackingState.tracking.Charter); charterState != nil {
			charter = charterState.charter
		}
		specState, err := s.projectSpec(trackingState.tracking, charter, repoState.config, repoState.specValidation(key))
		if err != nil {
			continue
		}
		specState = buildSpecContextStateAndNext(specState, key, s.repoRoot)
		deferred := 0
		for _, delta := range trackingState.tracking.Deltas {
			if delta.Status == domain.DeltaStatusDeferred {
				deferred++
			}
		}
		target := advisoryTarget{
			target:           key,
			title:            trackingState.tracking.Title,
			charter:          trackingState.tracking.Charter,
			deferred:         deferred,
			scopeDriftStatus: specState.ScopeDrift.Status,
			driftSource:      specState.ScopeDrift.DriftSource,
		}
		if specState.ScopeDrift.Status == "drifted" {
			classification := driftClassificationFocus(specState)
			target.reviewRequired, _ = classification["review_required"].(bool)
			target.correctnessBlocker, _ = classification["correctness_blocker"].(bool)
			target.housekeepingCandidate, _ = classification["housekeeping_candidate"].(bool)
		}
		if target.deferred > 0 || target.scopeDriftStatus == "drifted" {
			targets = append(targets, target)
		}
	}
	sort.Slice(targets, func(i, j int) bool {
		leftDrift := targets[i].scopeDriftStatus == "drifted"
		rightDrift := targets[j].scopeDriftStatus == "drifted"
		if leftDrift != rightDrift {
			return leftDrift
		}
		if targets[i].deferred != targets[j].deferred {
			return targets[i].deferred > targets[j].deferred
		}
		return targets[i].target < targets[j].target
	})
	return targets
}

func buildContextAdvisoryFocus(targets []advisoryTarget, charterName string) any {
	if len(targets) == 0 {
		return nil
	}

	focus := map[string]any{}
	deferredSummaries := make([]map[string]any, 0)
	totalDeferred := 0
	charters := make(map[string]struct{})
	driftSummaries := make([]map[string]any, 0)
	for _, target := range targets {
		if target.deferred > 0 {
			totalDeferred += target.deferred
			charters[target.charter] = struct{}{}
			entry := map[string]any{
				"target":         target.target,
				"title":          target.title,
				"deferred_count": target.deferred,
			}
			if charterName == "" {
				entry["charter"] = target.charter
			}
			deferredSummaries = append(deferredSummaries, entry)
		}
		if target.scopeDriftStatus == "drifted" {
			entry := map[string]any{
				"target":       target.target,
				"title":        target.title,
				"status":       target.scopeDriftStatus,
				"drift_source": target.driftSource,
			}
			if charterName == "" {
				entry["charter"] = target.charter
			}
			entry["review_required"] = target.reviewRequired
			entry["correctness_blocker"] = target.correctnessBlocker
			entry["housekeeping_candidate"] = target.housekeepingCandidate
			driftSummaries = append(driftSummaries, entry)
		}
	}

	if len(deferredSummaries) > 0 {
		summary := map[string]any{
			"count":          totalDeferred,
			"specs_affected": len(deferredSummaries),
			"targets":        deferredSummaries,
		}
		if charterName == "" {
			summary["charters_affected"] = len(charters)
		}
		focus["deferred_summary"] = summary
	}
	if len(driftSummaries) > 0 {
		focus["drift_summary"] = map[string]any{
			"count":   len(driftSummaries),
			"targets": driftSummaries,
		}
	}

	if target := recommendedAdvisoryTarget(targets); target != nil {
		review := map[string]any{
			"target": target.target,
		}
		if target.scopeDriftStatus == "drifted" {
			review["tool"] = "specctl_diff"
			review["reason"] = fmt.Sprintf("%s has active committed drift and should be reviewed before any semantic decision.", target.target)
		} else {
			review["tool"] = "specctl_context"
			if charterName == "" {
				review["reason"] = fmt.Sprintf("%s carries the highest deferred delta count (%d) in the current governed set.", target.target, target.deferred)
			} else {
				review["reason"] = fmt.Sprintf("%s carries the highest deferred delta count (%d) in charter %s.", target.target, target.deferred, charterName)
			}
		}
		focus["recommended_review"] = review
	}

	if len(focus) == 0 {
		return nil
	}
	return focus
}

func recommendedAdvisoryTarget(targets []advisoryTarget) *advisoryTarget {
	for i := range targets {
		if targets[i].scopeDriftStatus == "drifted" {
			return &targets[i]
		}
	}
	for i := range targets {
		if targets[i].deferred > 0 {
			return &targets[i]
		}
	}
	return nil
}

func (s *Service) readSpecContext(target string) (any, []any, error) {
	charterName, slug, ok := strings.Cut(target, ":")
	if !ok {
		return nil, nil, fmt.Errorf("invalid spec target %q", target)
	}

	trackingExists, err := s.pathAdapter().TrackingExists(charterName, slug)
	if err != nil {
		return nil, nil, fmt.Errorf("checking tracking path: %w", err)
	}
	if !trackingExists {
		charterExists, charterErr := s.pathAdapter().CharterExists(charterName)
		if charterErr != nil {
			return nil, nil, fmt.Errorf("checking charter path: %w", charterErr)
		}
		return MissingSpecContext{
			Target:        target,
			TrackingFile:  nil,
			CharterExists: charterExists,
			Focus:         map[string]any{"lookup": map[string]any{"reason": "spec_not_found"}},
		}, buildMissingSpecNext(target, charterExists), nil
	}

	repoState, err := s.loadRepoReadState()
	if err != nil {
		return nil, nil, err
	}
	trackingState := repoState.specTracking(target)
	if trackingState == nil {
		return MissingSpecContext{
			Target:        target,
			TrackingFile:  nil,
			CharterExists: repoState.charterState(charterName) != nil,
			Focus:         map[string]any{"lookup": map[string]any{"reason": "spec_not_found"}},
		}, buildMissingSpecNext(target, repoState.charterState(charterName) != nil), nil
	}

	var charter *domain.Charter
	if charterState := repoState.charterState(charterName); charterState != nil {
		charter = charterState.charter
	}

	state, err := s.projectSpec(trackingState.tracking, charter, repoState.config, repoState.specValidation(target))
	if err != nil {
		return nil, nil, err
	}
	state = buildSpecContextStateAndNext(state, target, s.repoRoot)
	return state, buildSpecContextNext(state, target, s.repoRoot), nil
}

func (s *Service) readFileContext(file string) (FileContextProjection, []any, error) {
	snapshot, err := s.repoReadAdapter().LoadRepoReadSnapshot()
	if err != nil {
		return FileContextProjection{}, nil, err
	}

	resolution, err := s.repoReadAdapter().ResolveFileOwnership(file, snapshot)
	if err != nil {
		state := invalidFileContextProjection(file)
		state.Focus = map[string]any{
			"invalid_paths": []string{state.File},
		}
		return FileContextProjection{}, nil, &Failure{
			Code:    "INVALID_INPUT",
			Message: err.Error(),
			State:   state,
			Next:    []any{},
		}
	}

	state := fileContextProjectionFromResolution(resolution)
	switch state.Resolution {
	case "unmatched", "no_match":
		state.Resolution = "unmatched"
		state.Focus = map[string]any{"ownership": map[string]any{"reason": "no_governing_spec"}}
		return state, buildFileContextNoMatchNext(resolution.CreatePlan), nil
	case "ambiguous":
		state.Focus = map[string]any{"ownership": map[string]any{"matches": state.Matches}}
		return state, buildFileContextAmbiguousNext(state), nil
	default:
		return state, []any{}, nil
	}
}

func invalidFileContextProjection(file string) FileContextProjection {
	trimmed := strings.TrimSpace(file)
	normalized := trimmed
	if trimmed != "" {
		normalized = strings.TrimPrefix(path.Clean(strings.ReplaceAll(trimmed, "\\", "/")), "./")
	}
	return FileContextProjection{
		File:       normalized,
		Resolution: "unmatched",
		Matches:    []FileMatchProjection{},
		Validation: validProjection(),
	}
}

func newServiceFromAdapters(adapters infrastructure.ServiceAdapters) *Service {
	return &Service{
		repoRoot:    adapters.RepoRoot,
		specsDir:    adapters.SpecsDir,
		paths:       adapters.Paths,
		registry:    adapters.Registry,
		repoReads:   adapters.RepoReads,
		checkpoints: adapters.Checkpoints,
	}
}

// SpecsDir returns the resolved specs directory path for this service.
func (s *Service) SpecsDir() string {
	s.ensureAdapters()
	return s.specsDir
}

func (s *Service) todayUTC() string {
	if s != nil && s.now != nil {
		return s.now().UTC().Format("2006-01-02")
	}
	return time.Now().UTC().Format("2006-01-02")
}

func (s *Service) pathAdapter() infrastructure.PathAccess {
	s.ensureAdapters()
	return s.paths
}

func (s *Service) registryStore() infrastructure.RegistryAccess {
	s.ensureAdapters()
	return s.registry
}

func (s *Service) repoReadAdapter() infrastructure.RepoReadAccess {
	s.ensureAdapters()
	return s.repoReads
}

func (s *Service) checkpointStore() infrastructure.CheckpointAccess {
	s.ensureAdapters()
	return s.checkpoints
}

func (s *Service) ensureAdapters() {
	if s.paths != nil && s.registry != nil && s.repoReads != nil && s.checkpoints != nil {
		return
	}
	adapters := infrastructure.NewServiceAdapters(s.repoRoot)
	s.paths = adapters.Paths
	s.registry = adapters.Registry
	s.repoReads = adapters.RepoReads
	s.checkpoints = adapters.Checkpoints
	if s.repoRoot == "" {
		s.repoRoot = adapters.RepoRoot
	}
	if s.specsDir == "" {
		s.specsDir = adapters.SpecsDir
	}
}

func buildMissingSpecNext(target string, charterExists bool) []any {
	next := make([]any, 0, 2)
	if !charterExists {
		charter, _, _ := strings.Cut(target, ":")
		next = append(next, buildCreateCharterNext(charter)...)
	}
	next = append(next, buildCreateSpecNext(SpecCreateTemplateSeed{
		Target:               target,
		Group:                "",
		IncludeGroupField:    true,
		IncludeGroupMetadata: charterExists,
		ChooseWhen:           ternaryInstructions(charterExists, "The charter already exists and the missing spec should be created inside it.", ""),
		Priority:             ternaryPriority(charterExists, 1, 2),
		Instructions:         ternaryInstructions(charterExists, "Create the tracking file first, then expand the design doc.", "After the charter exists, run spec create with membership flags."),
	})...)
	return next
}

func buildCreateCharterNext(charter string) []any {
	return []any{
		map[string]any{
			"priority":     1,
			"action":       "create_charter",
			"kind":         "run_command",
			"instructions": "Create the charter first; spec creation has no standalone fallback.",
			"template": map[string]any{
				"argv":           []string{"specctl", "charter", "create", charter},
				"stdin_format":   "yaml",
				"stdin_template": "title: <title>\ndescription: <description>\ngroups:\n  - key: <group_key>\n    title: <group_title>\n    order: <group_order>\n",
				"required_fields": []map[string]any{
					{"name": "title", "description": "Human-readable charter title"},
					{"name": "description", "description": "One-paragraph charter description"},
					{"name": "group_key", "description": "Initial group key"},
					{"name": "group_title", "description": "Initial group title"},
					{"name": "group_order", "description": "Integer group order"},
				},
			},
		},
	}
}

func buildCreateCharterChoiceNext(charter string) []any {
	next := buildCreateCharterNext(charter)
	if len(next) == 0 {
		return next
	}
	option, ok := next[0].(map[string]any)
	if !ok {
		return next
	}
	option["choose_when"] = "The requested charter does not exist yet and must be created before any spec can live under it."
	return next
}

func buildCreateSpecSeedForCharter(target string, groups []domain.CharterGroup, priority int, withGroupsInstruction, withoutGroupsInstruction string) SpecCreateTemplateSeed {
	group := firstGroupKey(groups)
	hasGroups := strings.TrimSpace(group) != ""
	return SpecCreateTemplateSeed{
		Target:               target,
		Group:                group,
		IncludeGroupField:    !hasGroups,
		IncludeGroupMetadata: !hasGroups,
		Priority:             priority,
		Instructions:         ternaryInstructions(hasGroups, withGroupsInstruction, withoutGroupsInstruction),
	}
}

func buildCreateSpecNext(seed SpecCreateTemplateSeed) []any {
	group := seed.Group
	if seed.IncludeGroupField && strings.TrimSpace(group) == "" {
		group = "<group>"
	}
	argv := []string{"specctl", "spec", "create", seed.Target, "--title", "<title>", "--doc", "<design_doc>", "--scope", seed.ScopePlaceholderOrDefault()}
	if seed.IncludeGroupField || strings.TrimSpace(seed.Group) != "" {
		argv = append(argv, "--group", group)
	}
	if seed.IncludeGroupMetadata {
		argv = append(argv, "--group-title", "<group_title>", "--group-order", "<group_order>")
	}
	argv = append(argv, "--order", "<order>", "--charter-notes", "<charter_notes>")
	required := make([]map[string]any, 0, 8)
	if strings.Contains(seed.Target, "<slug>") {
		required = append(required, map[string]any{"name": "slug", "description": "Kebab-case spec identifier inside the charter"})
	}
	required = append(required,
		map[string]any{"name": "title", "description": "Human-readable spec title"},
		map[string]any{"name": "design_doc", "description": "Repo-relative markdown path"},
	)
	if seed.Scope == "" {
		description := "Governed directory ending in /"
		if seed.IncludeGroupMetadata {
			description = "First repo-relative governed directory ending in /"
		}
		required = append(required, map[string]any{"name": "scope_dir_1", "description": description})
	}
	if seed.IncludeGroupField {
		required = append(required, map[string]any{"name": "group", "description": ternaryInstructions(seed.IncludeGroupMetadata, "Charter group key", "Existing charter group key")})
	}
	if seed.IncludeGroupMetadata {
		required = append(required,
			map[string]any{"name": "group_title", "description": "Required only when creating a new group"},
			map[string]any{"name": "group_order", "description": "Integer order for a newly created group"},
		)
	}
	orderDescription := "Integer order inside the group"
	charterNotesDescription := "Short planning note"
	if seed.IncludeGroupMetadata {
		orderDescription = "Integer order for the spec inside its group"
		charterNotesDescription = "Short planning note for the charter entry"
	}
	required = append(required,
		map[string]any{"name": "order", "description": orderDescription},
		map[string]any{"name": "charter_notes", "description": charterNotesDescription},
	)
	option := map[string]any{
		"priority":     seed.PriorityOrDefault(),
		"action":       "create_spec",
		"kind":         "run_command",
		"instructions": seed.InstructionsOrDefault(),
		"template": map[string]any{
			"argv":            argv,
			"required_fields": required,
		},
	}
	if chooseWhen := strings.TrimSpace(seed.ChooseWhen); chooseWhen != "" {
		option["choose_when"] = chooseWhen
	}
	return []any{option}
}

func fileContextProjectionFromResolution(resolution infrastructure.FileOwnershipResolution) FileContextProjection {
	projection := FileContextProjection{
		File:       resolution.File,
		Resolution: resolution.Resolution,
		Matches:    make([]FileMatchProjection, 0, len(resolution.Matches)),
		Validation: validProjection(),
	}
	if resolution.MatchSource != nil {
		projection.MatchSource = stringPointer(*resolution.MatchSource)
	}
	if resolution.GoverningSpec != nil {
		projection.GoverningSpec = &FileGoverningSpec{
			Slug:         resolution.GoverningSpec.Slug,
			Charter:      resolution.GoverningSpec.Charter,
			TrackingFile: resolution.GoverningSpec.TrackingFile,
			Documents:    resolution.GoverningSpec.Documents,
		}
	}
	for _, match := range resolution.Matches {
		projection.Matches = append(projection.Matches, FileMatchProjection{
			Slug:        match.Slug,
			Charter:     match.Charter,
			MatchSource: match.MatchSource,
			ScopePrefix: match.ScopePrefix,
		})
	}
	if len(resolution.ValidationFindings) > 0 {
		projection.Validation = projectionFromFindings(resolution.ValidationFindings)
	}
	return projection
}

func firstGroupKey(groups []domain.CharterGroup) string {
	if len(groups) == 0 {
		return ""
	}
	return groups[0].Key
}

func buildFileContextNoMatchNext(plan *infrastructure.SpecCreateSuggestion) []any {
	if plan == nil || strings.TrimSpace(plan.Target) == "" {
		return []any{}
	}
	return buildCreateSpecNext(SpecCreateTemplateSeed{
		Target:               plan.Target,
		IncludeGroupField:    true,
		IncludeGroupMetadata: false,
		ChooseWhen:           "No existing governing spec owns this file path, so establish ownership with a new spec.",
		Scope:                plan.Scope,
		Priority:             1,
		Instructions:         "No governing spec owns this path. Create a spec to establish ownership.",
	})
}

func buildFileContextAmbiguousNext(state FileContextProjection) []any {
	return []any{}
}

// findSupersededOrphans returns superseded requirements that still reference
// test files existing on disk which are NOT also referenced by any active
// requirement. These are cleanup candidates — the orphan test should be removed.
// repoRoot is used via infrastructure.FileExistsAt to check disk presence.
func findSupersededOrphans(state SpecProjection, repoRoot string) []map[string]any {
	// Build set of test files referenced by active requirements
	activeTestFiles := make(map[string]bool)
	for _, req := range state.Requirements {
		if req.Lifecycle == domain.RequirementLifecycleActive {
			for _, tf := range req.TestFiles {
				activeTestFiles[tf] = true
			}
		}
	}

	var orphans []map[string]any
	for _, req := range state.Requirements {
		if req.Lifecycle != domain.RequirementLifecycleSuperseded {
			continue
		}
		if len(req.TestFiles) == 0 {
			continue
		}
		var orphanFiles []string
		for _, tf := range req.TestFiles {
			// Skip files also referenced by active requirements
			if activeTestFiles[tf] {
				continue
			}
			if infrastructure.FileExistsAt(repoRoot, tf) {
				orphanFiles = append(orphanFiles, tf)
			}
		}
		if len(orphanFiles) == 0 {
			continue
		}
		supersededBy := ""
		if req.SupersededBy != nil {
			supersededBy = *req.SupersededBy
		}
		orphans = append(orphans, map[string]any{
			"requirement_id":    req.ID,
			"superseded_by":     supersededBy,
			"orphan_test_files": orphanFiles,
		})
	}
	return orphans
}

type specContextWarningAccumulator struct {
	deltaIDs         map[string]struct{}
	requirementIDs   map[string]struct{}
	replacementPairs map[string]specContextReplacementPair
}

type specContextReplacementPair struct {
	SupersededRequirementID  string
	ReplacementRequirementID string
	ReplacementDeltaID       string
}

func buildSpecContextWarnings(state SpecProjection) []SpecContextWarningProjection {
	warningsByKey := make(map[string]*specContextWarningAccumulator)
	for _, delta := range state.Deltas.Items {
		if delta.Status != domain.DeltaStatusDeferred || len(delta.AffectsRequirements) == 0 {
			continue
		}

		replacements := make([]specContextReplacementPair, 0, len(delta.AffectsRequirements))
		eligible := true
		for _, requirementID := range sortedUniqueStrings(delta.AffectsRequirements) {
			replacement, ok := deferredSupersededResidueReplacement(state, requirementID, delta.ID)
			if !ok {
				eligible = false
				break
			}
			replacements = append(replacements, replacement)
		}
		if !eligible || len(replacements) == 0 {
			continue
		}

		requirementIDs := make([]string, 0, len(replacements))
		for _, replacement := range replacements {
			requirementIDs = append(requirementIDs, replacement.SupersededRequirementID)
		}
		sort.Strings(requirementIDs)
		dedupeKey := "deferred_superseded_residue:" + strings.Join(requirementIDs, ",")

		accumulator := warningsByKey[dedupeKey]
		if accumulator == nil {
			accumulator = &specContextWarningAccumulator{
				deltaIDs:         make(map[string]struct{}),
				requirementIDs:   make(map[string]struct{}),
				replacementPairs: make(map[string]specContextReplacementPair),
			}
			warningsByKey[dedupeKey] = accumulator
		}
		accumulator.deltaIDs[delta.ID] = struct{}{}
		for _, replacement := range replacements {
			accumulator.requirementIDs[replacement.SupersededRequirementID] = struct{}{}
			accumulator.replacementPairs[replacement.SupersededRequirementID] = replacement
		}
	}

	if len(warningsByKey) == 0 {
		return []SpecContextWarningProjection{}
	}

	keys := make([]string, 0, len(warningsByKey))
	for key := range warningsByKey {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	warnings := make([]SpecContextWarningProjection, 0, len(keys))
	for _, key := range keys {
		accumulator := warningsByKey[key]
		requirementIDs := sortedKeys(accumulator.requirementIDs)
		replacementPairs := make([]map[string]string, 0, len(requirementIDs))
		replacementDeltaIDs := make([]string, 0, len(requirementIDs))
		replacementRequirementIDs := make([]string, 0, len(requirementIDs))
		for _, requirementID := range requirementIDs {
			replacement := accumulator.replacementPairs[requirementID]
			replacementPairs = append(replacementPairs, map[string]string{
				"superseded_requirement_id":  replacement.SupersededRequirementID,
				"replacement_requirement_id": replacement.ReplacementRequirementID,
				"replacement_delta_id":       replacement.ReplacementDeltaID,
			})
			replacementRequirementIDs = append(replacementRequirementIDs, replacement.ReplacementRequirementID)
			replacementDeltaIDs = append(replacementDeltaIDs, replacement.ReplacementDeltaID)
		}
		replacementRequirementIDs = sortedUniqueStrings(replacementRequirementIDs)
		replacementDeltaIDs = sortedUniqueStrings(replacementDeltaIDs)
		deltaIDs := sortedKeys(accumulator.deltaIDs)

		warnings = append(warnings, SpecContextWarningProjection{
			Kind:           "historical_residue",
			Code:           "DEFERRED_SUPERSEDED_RESIDUE",
			Severity:       "warning",
			Message:        "Deferred deltas only reference superseded requirements already replaced by later closed work. Review whether governed cleanup is still needed.",
			DeltaIDs:       deltaIDs,
			RequirementIDs: requirementIDs,
			Details: map[string]any{
				"dedupe_key":                  key,
				"replacement_pairs":           replacementPairs,
				"replacement_requirement_ids": replacementRequirementIDs,
				"replacement_delta_ids":       replacementDeltaIDs,
			},
		})
	}

	return warnings
}

func deferredSupersededResidueReplacement(state SpecProjection, requirementID, deferredDeltaID string) (specContextReplacementPair, bool) {
	requirement := requirementProjectionByID(state, requirementID)
	if requirement == nil || requirement.Lifecycle != domain.RequirementLifecycleSuperseded || requirement.SupersededBy == nil {
		return specContextReplacementPair{}, false
	}

	replacementRequirementID := strings.TrimSpace(*requirement.SupersededBy)
	if replacementRequirementID == "" {
		return specContextReplacementPair{}, false
	}

	replacementRequirement := requirementProjectionByID(state, replacementRequirementID)
	if replacementRequirement == nil || strings.TrimSpace(replacementRequirement.IntroducedBy) == "" {
		return specContextReplacementPair{}, false
	}
	if replacementRequirement.IntroducedBy == deferredDeltaID {
		return specContextReplacementPair{}, false
	}

	replacementDelta := deltaItemProjectionByID(state, replacementRequirement.IntroducedBy)
	if replacementDelta == nil || replacementDelta.Status != domain.DeltaStatusClosed {
		return specContextReplacementPair{}, false
	}

	return specContextReplacementPair{
		SupersededRequirementID:  requirement.ID,
		ReplacementRequirementID: replacementRequirement.ID,
		ReplacementDeltaID:       replacementDelta.ID,
	}, true
}

func requirementProjectionByID(state SpecProjection, requirementID string) *RequirementProjection {
	for i := range state.Requirements {
		if state.Requirements[i].ID == requirementID {
			return &state.Requirements[i]
		}
	}
	return nil
}

func deltaItemProjectionByID(state SpecProjection, deltaID string) *DeltaItemProjection {
	for i := range state.Deltas.Items {
		if state.Deltas.Items[i].ID == deltaID {
			return &state.Deltas.Items[i]
		}
	}
	return nil
}

func sortedUniqueStrings(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		if _, ok := seen[trimmed]; ok {
			continue
		}
		seen[trimmed] = struct{}{}
		result = append(result, trimmed)
	}
	sort.Strings(result)
	return result
}

func sortedKeys(values map[string]struct{}) []string {
	result := make([]string, 0, len(values))
	for value := range values {
		result = append(result, value)
	}
	sort.Strings(result)
	return result
}

func buildSpecContextStateAndNext(state SpecProjection, target, repoRoot string) SpecProjection {
	_ = target
	state.Warnings = buildSpecContextWarnings(state)
	orphans := findSupersededOrphans(state, repoRoot)
	if len(state.UncommittedChanges) > 0 {
		state.Focus = map[string]any{"working_tree": map[string]any{"status": "dirty"}}
		return state
	}
	if issues := buildContextRefreshMatchIssues(state); len(issues) > 0 {
		state.Focus = map[string]any{"requirement_match_issues": issues}
		return state
	}
	if requirement := firstContextStaleRequirement(state); requirement != nil {
		state.Focus = map[string]any{
			"blocking_requirement": map[string]any{
				"id":           requirement.ID,
				"title":        requirement.Title,
				"lifecycle":    requirement.Lifecycle,
				"verification": requirement.Verification,
			},
		}
		return state
	}
	if state.ScopeDrift.Status == "clean" {
		if len(orphans) > 0 {
			state.Focus = map[string]any{"superseded_orphans": orphans}
			return state
		}
		state.Focus = nil
		return state
	}
	scopeDriftFocus := map[string]any{
		"status":       state.ScopeDrift.Status,
		"drift_source": nullableProjectionAny(state.ScopeDrift.DriftSource),
	}
	if state.ScopeDrift.Status == "drifted" {
		for key, value := range driftClassificationFocus(state) {
			scopeDriftFocus[key] = value
		}
	}
	state.Focus = map[string]any{"scope_drift": scopeDriftFocus}
	if len(orphans) > 0 {
		state.Focus.(map[string]any)["superseded_orphans"] = orphans
	}
	return state
}

func buildSpecContextNext(state SpecProjection, target, repoRoot string) []any {
	next := make([]any, 0)
	priority := 1
	orphans := findSupersededOrphans(state, repoRoot)

	// Superseded requirements with orphan test files on disk — recommend cleanup
	if len(orphans) > 0 && state.ScopeDrift.Status == "clean" {
		for _, orphan := range orphans {
			for _, testFile := range orphan["orphan_test_files"].([]string) {
				next = append(next, map[string]any{
					"priority": priority,
					"action":   "cleanup_superseded_orphan",
					"kind":     "guidance",
					"instructions": fmt.Sprintf(
						"%s is superseded by %s but still references test file %s which exists on disk. Remove test_files from the superseded requirement and delete the orphaned test.",
						orphan["requirement_id"], orphan["superseded_by"], testFile,
					),
					"details": orphan,
				})
				priority++
			}
		}
		return next
	}

	if len(state.UncommittedChanges) > 0 {
		next = append(next,
			runCommandAction(priority, "stage_changes", "Stage the governed working-tree edits before asking specctl to classify drift.", []string{"git", "add", "--"}, state.UncommittedChanges, nil, "", ""),
			map[string]any{
				"priority":     priority + 1,
				"action":       "commit_changes",
				"kind":         "run_command",
				"instructions": "Commit the staged scope changes before re-running context.",
				"template": map[string]any{
					"argv": []string{"git", "commit", "-m", "<message>"},
					"required_fields": []map[string]any{
						{"name": "message", "description": "Commit message for the staged scope changes"},
					},
				},
			},
			runCommandAction(priority+2, "re_run_context", "After the commit exists, re-run context to classify committed drift.", []string{"specctl", "context", target}, nil, []map[string]any{}, "", ""),
		)
		priority += 3
	}
	if len(state.UncommittedChanges) == 0 {
		if refresh := buildSpecContextRefreshNext(state, target); len(refresh) > 0 {
			next = append(next, offsetNextPriorities(refresh, priority-1)...)
			return next
		}
		if repair := buildSpecContextRepairNext(state, target); len(repair) > 0 {
			next = append(next, offsetNextPriorities(repair, priority-1)...)
			return next
		}
	}

	switch state.ScopeDrift.Status {
	case "drifted":
		next = append(next, buildDriftGuidanceNext(state, target, true, true, priority)...)
	case "tracked":
		if continuation := buildTrackedDriftContinuationNext(state, target); len(continuation) > 0 {
			next = append(next, offsetNextPriorities(continuation, priority-1)...)
			return next
		}
		next = append(next, offsetNextPriorities(buildTrackedRevisionBumpNext(state, target), priority-1)...)
	case "unavailable":
		next = append(next, buildDriftGuidanceNext(state, target, true, true, priority)...)
	case "clean":
		// When scope is clean, surface unverified active requirements that need evidence
		for _, req := range state.Requirements {
			if req.Lifecycle == domain.RequirementLifecycleActive && req.Verification == domain.RequirementVerificationUnverified {
				next = append(next, map[string]any{
					"priority": priority,
					"action":   "verify_requirement",
					"kind":     "run_command",
					"instructions": fmt.Sprintf(
						"%s (%s) is active but unverified. Provide test file evidence via: specctl req verify %s %s --test-file <path>",
						req.ID, req.Title, target, req.ID,
					),
					"template": map[string]any{
						"argv": []string{"specctl", "req", "verify", target, req.ID, "--test-file", "<path>"},
						"required_fields": []map[string]any{
							{"name": "path", "description": "Repo-relative path to the test file that proves this requirement"},
						},
					},
				})
				priority++
			}
		}
	}

	if len(next) == 0 && state.ScopeDrift.Status == "clean" && len(state.UncommittedChanges) == 0 && len(state.Warnings) > 0 {
		next = append(next, map[string]any{
			"priority":     priority,
			"action":       "review_warnings",
			"kind":         "guidance",
			"instructions": "Review the advisory warnings and decide whether governed specctl cleanup follow-up is still needed. Do not edit tracking YAML manually.",
			"details": map[string]any{
				"warning_codes": warningCodes(state.Warnings),
			},
		})
	}

	return next
}

func warningCodes(warnings []SpecContextWarningProjection) []string {
	codes := make([]string, 0, len(warnings))
	for _, warning := range warnings {
		codes = append(codes, warning.Code)
	}
	return sortedUniqueStrings(codes)
}

func buildContextRefreshMatchIssues(state SpecProjection) []map[string]any {
	issues := make([]map[string]any, 0)
	for _, requirement := range state.Requirements {
		if requirement.Lifecycle != domain.RequirementLifecycleActive {
			continue
		}
		if !isBlockingRequirementMatchStatus(requirement.Match.Status) {
			continue
		}
		issue := map[string]any{
			"id":     requirement.ID,
			"status": requirement.Match.Status,
		}
		if requirement.Match.Heading != nil {
			issue["heading"] = *requirement.Match.Heading
		}
		issues = append(issues, issue)
	}
	return issues
}

func firstContextStaleRequirement(state SpecProjection) *RequirementProjection {
	for i := range state.Requirements {
		requirement := &state.Requirements[i]
		if requirement.Lifecycle != domain.RequirementLifecycleActive {
			continue
		}
		if requirement.Verification != domain.RequirementVerificationStale {
			continue
		}
		return requirement
	}
	return nil
}

func buildSpecContextRefreshNext(state SpecProjection, target string) []any {
	issues := buildContextRefreshMatchIssues(state)
	if len(issues) == 0 {
		return []any{}
	}
	requirementID, _ := issues[0]["id"].(string)
	options := buildRequirementMatchBlockingNext(target, requirementID)
	if len(options) == 0 {
		return []any{}
	}
	return []any{options[0]}
}

func buildSpecContextRepairNext(state SpecProjection, target string) []any {
	requirement := firstContextStaleRequirement(state)
	if requirement == nil {
		return []any{}
	}
	return []any{
		map[string]any{
			"priority":     1,
			"action":       "delta_add_repair",
			"kind":         "run_command",
			"instructions": "Choose this when the requirement remains correct and only fresh evidence is needed.",
			"choose_when":  "Requirement remains true but evidence is not trusted.",
			"template": map[string]any{
				"argv":         []string{"specctl", "delta", "add", target, "--intent", "repair", "--area", "<area>"},
				"stdin_format": "yaml",
				"stdin_template": strings.Join([]string{
					"current: <current>",
					"target: <target>",
					"notes: <notes>",
					"affects_requirements:",
					"  - " + requirement.ID,
					"",
				}, "\n"),
				"required_fields": []map[string]any{
					{"name": "area", "description": "Short delta area label"},
					{"name": "current", "description": "Current state"},
					{"name": "target", "description": "Target state"},
					{"name": "notes", "description": "Why this delta exists"},
				},
			},
		},
	}
}

func buildTrackedDriftContinuationNext(state SpecProjection, target string) []any {
	tracking := trackingFromSpecProjection(state)
	for _, deltaID := range trackedDeltaIDs(state, tracking) {
		delta := tracking.DeltaByID(deltaID)
		if delta == nil {
			continue
		}
		tracingRequirements := tracking.TracingRequirements(deltaID)
		if len(tracingRequirements) == 0 {
			return buildDeltaAddNext(target, tracking, *delta, state.FormatTemplate)
		}
		blocking, err := tracking.BlockingRequirementsForDeltaClosure(deltaID)
		if err != nil {
			continue
		}
		if len(blocking) > 0 {
			return buildDeltaCloseBlockingNext(target, tracking, blocking[0])
		}
		if next := buildDeltaCloseSuggestions(target, tracking, []string{deltaID}); len(next) > 0 {
			return next
		}
	}
	return []any{}
}

func buildTrackedRevisionBumpNext(state SpecProjection, target string) []any {
	if state.Status != domain.SpecStatusVerified {
		return []any{}
	}
	return []any{revisionBumpAction(1, target, "The tracked drift is fully verified. Converge the checkpoint with a revision bump.")}
}

func offsetNextPriorities(next []any, offset int) []any {
	if offset == 0 {
		return next
	}
	for _, raw := range next {
		action, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		priority, ok := action["priority"].(int)
		if !ok {
			continue
		}
		action["priority"] = priority + offset
	}
	return next
}

func buildDriftGuidanceNext(state SpecProjection, target string, includeReview, useCheckpointAction bool, startPriority int) []any {
	next := make([]any, 0, 2)
	priority := startPriority
	if includeReview && state.ScopeDrift.Status == "drifted" {
		review := runCommandAction(priority, "review_diff", driftReviewInstructions(scopeDriftSourceValue(state.ScopeDrift.DriftSource)), []string{"specctl", "diff", target}, nil, []map[string]any{}, "", "")
		review["choose_when"] = reviewChooseWhen(scopeDriftSourceValue(state.ScopeDrift.DriftSource))
		next = append(next, review)
		priority++
	}

	switch state.ScopeDrift.Status {
	case "drifted":
		if driftSyncCandidate(state) {
			sync := syncNextAction(priority, target, "If review confirms the change is clarification-only and the checkpoint just needs to move, re-anchor without bumping the revision.", "One-line reason the checkpoint is being re-anchored")
			sync["choose_when"] = syncChooseWhen(scopeDriftSourceValue(state.ScopeDrift.DriftSource))
			next = append(next, sync)
		}
	case "unavailable":
		if useCheckpointAction {
			next = append(next, syncCheckpointAction(priority, target, "Repair the missing checkpoint by re-anchoring to a resolvable commit.", "One-line reason the checkpoint is being repaired"))
		} else {
			next = append(next, syncNextAction(priority, target, "Repair the missing checkpoint by re-anchoring to a resolvable commit.", "One-line reason the checkpoint is being repaired"))
		}
	}

	return next
}

func trackedDeltaIDs(state SpecProjection, tracking *domain.TrackingFile) []string {
	ids := uniqueOrderedStrings(state.ScopeDrift.TrackedBy)
	if len(ids) > 0 {
		return ids
	}
	for _, delta := range tracking.Deltas {
		if delta.Status == domain.DeltaStatusDeferred || delta.Status == domain.DeltaStatusWithdrawn {
			continue
		}
		ids = append(ids, delta.ID)
	}
	return uniqueOrderedStrings(ids)
}

func driftClassificationFocus(state SpecProjection) map[string]any {
	return map[string]any{
		"review_required":        true,
		"correctness_blocker":    len(blockingActiveRequirementIDs(state)) > 0,
		"housekeeping_candidate": driftSyncCandidate(state),
	}
}

func driftSyncCandidate(state SpecProjection) bool {
	return len(blockingActiveRequirementIDs(state)) == 0 && state.Deltas.Open == 0 && state.Deltas.InProgress == 0
}

func trackingFromSpecProjection(state SpecProjection) *domain.TrackingFile {
	requirements := make([]domain.Requirement, 0, len(state.Requirements))
	for _, requirement := range state.Requirements {
		requirements = append(requirements, domain.Requirement{
			ID:           requirement.ID,
			Title:        requirement.Title,
			Tags:         append([]string{}, requirement.Tags...),
			TestFiles:    append([]string{}, requirement.TestFiles...),
			Gherkin:      requirement.Gherkin,
			Lifecycle:    requirement.Lifecycle,
			Verification: requirement.Verification,
			IntroducedBy: requirement.IntroducedBy,
			Supersedes:   derefString(requirement.Supersedes),
			SupersededBy: derefString(requirement.SupersededBy),
		})
	}
	deltas := make([]domain.Delta, 0, len(state.Deltas.Items))
	for _, delta := range state.Deltas.Items {
		deltas = append(deltas, domain.Delta{
			ID:                  delta.ID,
			Area:                delta.Area,
			Intent:              delta.Intent,
			Status:              delta.Status,
			OriginCheckpoint:    delta.OriginCheckpoint,
			Current:             delta.Current,
			Target:              delta.Target,
			Notes:               delta.Notes,
			AffectsRequirements: append([]string{}, delta.AffectsRequirements...),
			Updates:             append([]string{}, delta.Updates...),
		})
	}
	return &domain.TrackingFile{
		Slug:           state.Slug,
		Charter:        state.Charter,
		Title:          state.Title,
		Status:         state.Status,
		Rev:            state.Rev,
		Created:        state.Created,
		Updated:        state.Updated,
		LastVerifiedAt: state.LastVerifiedAt,
		Checkpoint:     state.Checkpoint,
		Tags:           append([]string{}, state.Tags...),
		Documents:      state.Documents,
		Scope:          append([]string{}, state.Scope...),
		Deltas:         deltas,
		Requirements:   requirements,
		Changelog:      append([]domain.ChangelogEntry{}, state.Changelog...),
	}
}

func derefString(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func runCommandAction(priority int, action, instructions string, prefix []string, suffix []string, required []map[string]any, stdinFormat, stdinTemplate string) map[string]any {
	argv := append([]string{}, prefix...)
	argv = append(argv, suffix...)
	if required == nil {
		required = []map[string]any{}
	}
	template := map[string]any{"argv": argv, "required_fields": required}
	if stdinFormat != "" {
		template["stdin_format"] = stdinFormat
		template["stdin_template"] = stdinTemplate
	}
	return map[string]any{
		"priority":     priority,
		"action":       action,
		"kind":         "run_command",
		"instructions": instructions,
		"template":     template,
	}
}

func revisionBumpAction(priority int, target, instructions string) map[string]any {
	return map[string]any{
		"priority":     priority,
		"action":       "rev_bump",
		"kind":         "run_command",
		"instructions": instructions,
		"template": map[string]any{
			"argv":            []string{"specctl", "rev", "bump", target, "--checkpoint", "HEAD"},
			"stdin_format":    "text",
			"stdin_template":  "<summary>\n",
			"required_fields": []map[string]any{{"name": "summary", "description": "One-line changelog summary"}},
		},
	}
}

func syncNextAction(priority int, target, instructions, summaryDescription string) map[string]any {
	return map[string]any{
		"priority":     priority,
		"action":       "sync",
		"kind":         "run_command",
		"instructions": instructions,
		"template": map[string]any{
			"argv":           []string{"specctl", "sync", target, "--checkpoint", "HEAD"},
			"stdin_format":   "text",
			"stdin_template": "<summary>\n",
			"required_fields": []map[string]any{
				{"name": "summary", "description": summaryDescription},
			},
		},
	}
}

func syncCheckpointAction(priority int, target, instructions, summaryDescription string) map[string]any {
	action := syncNextAction(priority, target, instructions, summaryDescription)
	action["action"] = "sync_checkpoint"
	return action
}

func driftReviewInstructions(source string) string {
	switch source {
	case "design_doc":
		return "Review the committed design-doc drift before deciding whether it needs semantic tracking or only a checkpoint re-anchor."
	case "scope_code":
		return "Review the committed code drift before choosing sync or new spec work."
	case "both":
		return "Review the mixed design-doc and code drift before deciding whether it needs semantic tracking or only a checkpoint re-anchor."
	default:
		return "The working-tree edit is now committed drift. Review the diff first."
	}
}

func reviewChooseWhen(source string) string {
	switch source {
	case "design_doc":
		return "Committed design-doc drift needs review before deciding whether it is semantic work or clarification-only housekeeping."
	case "scope_code":
		return "Committed scope changes need semantic review before choosing how to re-align tracking."
	case "both":
		return "Mixed committed drift needs review before deciding whether it changes governed behavior or only the checkpoint."
	default:
		return "Review the committed drift before classifying it."
	}
}

func syncChooseWhen(source string) string {
	switch source {
	case "design_doc":
		return "Review confirms the design-doc edit is clarification-only and the checkpoint just needs re-anchoring."
	case "scope_code":
		return "Review confirms the tracked contract remains correct and only the checkpoint must move."
	case "both":
		return "Review confirms the code and design-doc edits are clarification-only and the checkpoint just needs re-anchoring."
	default:
		return "Review confirms governed behavior is unchanged and only the checkpoint must move."
	}
}

func nullableProjectionAny(value *string) any {
	if value == nil || strings.TrimSpace(*value) == "" {
		return nil
	}
	return *value
}

func scopeDriftSourceValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func ternaryPriority(condition bool, whenTrue, whenFalse int) int {
	if condition {
		return whenTrue
	}
	return whenFalse
}

func ternaryInstructions(condition bool, whenTrue, whenFalse string) string {
	if condition {
		return whenTrue
	}
	return whenFalse
}

func (s SpecCreateTemplateSeed) ScopePlaceholderOrDefault() string {
	if strings.TrimSpace(s.Scope) != "" {
		return s.Scope
	}
	return "<scope_dir_1>/"
}

func (s SpecCreateTemplateSeed) PriorityOrDefault() int {
	if s.Priority > 0 {
		return s.Priority
	}
	return 1
}

func (s SpecCreateTemplateSeed) InstructionsOrDefault() string {
	if strings.TrimSpace(s.Instructions) != "" {
		return s.Instructions
	}
	return "Create the tracking file first, then expand the design doc."
}
