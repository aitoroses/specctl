package application

import (
	"errors"
	"fmt"
	"path"
	"slices"
	"strings"

	"github.com/aitoroses/specctl/internal/domain"
	"github.com/aitoroses/specctl/internal/infrastructure"
)

type SpecCreateRequest struct {
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

type DeltaAddRequest struct {
	Target              string
	Intent              domain.DeltaIntent
	Area                string
	Current             string
	CurrentPresent      bool
	Targets             string
	TargetPresent       bool
	Notes               string
	NotesPresent        bool
	AffectsRequirements []string
}

type DeltaTransitionRequest struct {
	Target  string
	DeltaID string
}

type DeltaWithdrawRequest struct {
	Target  string
	DeltaID string
	Reason  string
}

type DeltaRebindRequest struct {
	Target  string
	DeltaID string
	From    string
	To      string
	Remove  bool
	Reason  string
}

type RequirementAddRequest struct {
	Target  string
	DeltaID string
	Gherkin string
}

type RequirementReplaceRequest struct {
	Target        string
	RequirementID string
	DeltaID       string
	Gherkin       string
}

type RequirementDeltaRequest struct {
	Target        string
	RequirementID string
	DeltaID       string
}

type RequirementRefreshRequest struct {
	Target        string
	RequirementID string
	Gherkin       string
}

type RequirementVerifyRequest struct {
	Target        string
	RequirementID string
	TestFiles     []string
}

type RevisionBumpRequest struct {
	Target     string
	Checkpoint string
	Summary    string
}

type SyncRequest struct {
	Target     string
	Checkpoint string
	Summary    string
}

type DocAddRequest struct {
	Target string
	Doc    string
}

type DocRemoveRequest struct {
	Target string
	Doc    string
}

type loadedSpec struct {
	charterName string
	slug        string
	tracking    *domain.TrackingFile
	charter     *domain.Charter
	config      *infrastructure.ProjectConfig
	state       SpecProjection
	relative    string
}

func (s *Service) CreateSpec(request SpecCreateRequest) (SpecProjection, map[string]any, []any, error) {
	charterName, slug, ok := strings.Cut(request.Target, ":")
	if !ok {
		return SpecProjection{}, nil, nil, fmt.Errorf("invalid spec target %q", request.Target)
	}

	trackingExists, err := s.pathAdapter().TrackingExists(charterName, slug)
	if err != nil {
		return SpecProjection{}, nil, nil, fmt.Errorf("checking tracking path: %w", err)
	}
	if trackingExists {
		return SpecProjection{}, nil, nil, &Failure{
			Code:    "SPEC_EXISTS",
			Message: fmt.Sprintf("spec %q already exists", request.Target),
			State: MissingSpecContext{
				Target:        request.Target,
				TrackingFile:  infrastructure.RelativeTrackingPath(s.repoRoot, charterName, slug),
				CharterExists: true,
			},
			Next: []any{},
		}
	}

	charterExists, err := s.pathAdapter().CharterExists(charterName)
	if err != nil {
		return SpecProjection{}, nil, nil, fmt.Errorf("checking charter path: %w", err)
	}
	if !charterExists {
		return SpecProjection{}, nil, nil, &Failure{
			Code:    "CHARTER_NOT_FOUND",
			Message: fmt.Sprintf("charter %q does not exist", charterName),
			State: MissingSpecContext{
				Target:        request.Target,
				TrackingFile:  infrastructure.RelativeTrackingPath(s.repoRoot, charterName, slug),
				CharterExists: false,
			},
			Next: []any{},
		}
	}

	config, err := s.registryStore().LoadProjectConfig()
	if err != nil {
		return SpecProjection{}, nil, nil, err
	}
	charter, err := s.registryStore().LoadCharterStructure(charterName)
	if err != nil {
		findings := validationFindingsFromMessage(infrastructure.RelativeCharterPath(charterName), slug, err.Error())
		return SpecProjection{}, nil, nil, specCreateTargetResolutionValidationFailure(
			"Cannot create the spec because the charter registry is invalid",
			request.Target,
			charterName,
			slug,
			findings,
			validationRepairNext(
				"Repair the charter registry before creating new specs in it.",
				infrastructure.RelativeCharterPath(charterName),
			),
		)
	}

	createPlan, err := s.registryStore().PrepareSpecCreate(infrastructure.SpecCreatePlanRequest{
		Charter: charterName,
		Slug:    slug,
		Doc:     request.Doc,
		Scope:   request.Scope,
		Config:  config,
	})
	if err != nil {
		var planErr *infrastructure.SpecCreatePlanError
		if errors.As(err, &planErr) {
			switch planErr.Code {
			case infrastructure.SpecCreateInvalidPath:
				return SpecProjection{}, nil, nil, specCreateFailure("INVALID_PATH", planErr.Error(), request.Target, charterName, slug, map[string]any{
					"invalid_paths": append([]string{}, planErr.InvalidPaths...),
				})
			case infrastructure.SpecCreateFormatAmbiguous:
				return SpecProjection{}, nil, nil, specCreateFailure("FORMAT_AMBIGUOUS", planErr.Error(), request.Target, charterName, slug, map[string]any{
					"design_doc": map[string]any{"path": planErr.DocPath},
				})
			case infrastructure.SpecCreatePrimaryDocMismatch:
				return SpecProjection{}, nil, nil, specCreateFailure("PRIMARY_DOC_FRONTMATTER_MISMATCH", planErr.Error(), request.Target, charterName, slug, map[string]any{
					"design_doc": map[string]any{"path": planErr.DocPath},
				})
			case infrastructure.SpecCreateFormatNotConfigured:
				return SpecProjection{}, nil, nil, specCreateFailure("FORMAT_NOT_CONFIGURED", planErr.Error(), request.Target, charterName, slug, map[string]any{
					"design_doc": map[string]any{"path": planErr.DocPath},
				})
			case infrastructure.SpecCreatePrimaryDocFrontmatterError:
				return SpecProjection{}, nil, nil, specCreateFailure("PRIMARY_DOC_FRONTMATTER_INVALID", planErr.Error(), request.Target, charterName, slug, map[string]any{
					"design_doc": map[string]any{"path": planErr.DocPath},
				})
			}
		}
		return SpecProjection{}, nil, nil, err
	}

	docPath := createPlan.DocPath
	scope := createPlan.Scope
	mutation := createPlan.Mutation
	tracking := s.specCreateTrackingCandidate(request, charterName, slug, docPath, scope)

	if charter.GroupByKey(request.Group) == nil {
		if request.GroupTitle == nil || request.GroupOrder == nil {
			missingFields := make([]string, 0, 2)
			if request.GroupTitle == nil {
				missingFields = append(missingFields, "group_title")
			}
			if request.GroupOrder == nil {
				missingFields = append(missingFields, "group_order")
			}
			return SpecProjection{}, nil, nil, specCreateFailure("GROUP_REQUIRED", fmt.Sprintf("charter group %q does not exist", request.Group), request.Target, charterName, slug, map[string]any{
				"input": map[string]any{"missing_fields": missingFields},
			})
		}
		if err := charter.EnsureGroup(domain.CharterGroup{
			Key:   request.Group,
			Title: strings.TrimSpace(*request.GroupTitle),
			Order: *request.GroupOrder,
		}); err != nil {
			findings := validationFindingsFromMessage(infrastructure.RelativeCharterPath(charterName), slug, err.Error())
			return SpecProjection{}, nil, nil, s.specCreateValidationFailure(
				"Cannot create the spec because the requested charter group is invalid",
				charter,
				tracking,
				config,
				findings,
				[]any{},
			)
		}
	}

	entry, err := domain.NewCharterSpecEntry(slug, request.Group, request.Order, normalizeSlugList(request.DependsOn), strings.TrimSpace(request.CharterNotes))
	if err != nil {
		findings := validationFindingsFromMessage(infrastructure.RelativeCharterPath(charterName), slug, err.Error())
		return SpecProjection{}, nil, nil, s.specCreateValidationFailure(
			"Cannot create the spec because the charter membership request is invalid",
			charter,
			tracking,
			config,
			findings,
			[]any{},
		)
	}
	charterWithEntry := cloneCharterForProjection(charter)
	upsertCharterSpecEntryForProjection(charterWithEntry, entry)
	if err := charter.ReplaceSpecEntry(entry); err != nil {
		findings := validationFindingsFromMessage(infrastructure.RelativeCharterPath(charterName), slug, err.Error())
		return SpecProjection{}, nil, nil, s.specCreateValidationFailure(
			"Cannot create the spec because the charter membership request is invalid",
			charterWithEntry,
			tracking,
			config,
			findings,
			[]any{},
		)
	}

	checkpoint, err := s.checkpointStore().ResolveCheckpoint("HEAD")
	if err != nil {
		return SpecProjection{}, nil, nil, specCreateFailure("CHECKPOINT_UNAVAILABLE", err.Error(), request.Target, charterName, slug, map[string]any{
			"spec_create": map[string]any{"checkpoint": "HEAD"},
		})
	}
	tracking.Checkpoint = checkpoint

	tracking.SyncComputedStatus()
	if err := tracking.Validate(); err != nil {
		findings := validationFindingsFromMessage(infrastructure.RelativeTrackingPath(s.repoRoot, charterName, slug), slug, err.Error())
		return SpecProjection{}, nil, nil, s.specCreateValidationFailure(
			"Cannot create the spec because the initial tracking state is invalid",
			charterWithEntry,
			tracking,
			config,
			findings,
			[]any{},
		)
	}

	mutationResult, err := s.registryStore().ApplySpecCreate(charter, tracking, mutation)
	if err != nil {
		var mutationErr *infrastructure.CharterMutationError
		if errors.As(err, &mutationErr) {
			return SpecProjection{}, nil, nil, s.specCreateValidationFailure(
				mutationErr.Message,
				charter,
				tracking,
				config,
				mutationErr.Findings,
				[]any{},
			)
		}
		return SpecProjection{}, nil, nil, err
	}
	result := map[string]any{
		"kind":              "spec",
		"tracking_file":     infrastructure.RelativeTrackingPath(s.repoRoot, charterName, slug),
		"design_doc":        docPath,
		"design_doc_action": mutation.Action,
		"selected_format":   mutation.SelectedFormat,
	}
	next := buildSpecCreateNext(request.Target, docPath, mutation.Action, mutation.SelectedFormat)
	return finalizeValidatedWrite(
		s,
		mutationResult.Snapshot,
		result,
		func(repoState *repoReadState) (SpecProjection, error) {
			state, err := s.specProjectionFromRepoState(repoState, request.Target)
			if err != nil {
				return SpecProjection{}, err
			}
			state.Focus = map[string]any{
				"spec": map[string]any{"target": request.Target},
			}
			return state, nil
		},
		func(SpecProjection) []any { return next },
		func(findings []infrastructure.ValidationFinding) error {
			return s.specCreateValidationFailure(
				"Cannot apply the write because the resulting repo state is invalid",
				charter,
				tracking,
				config,
				findings,
				[]any{},
			)
		},
	)
}

func (s *Service) AddDelta(request DeltaAddRequest) (SpecProjection, map[string]any, []any, error) {
	loaded, failure, err := s.loadSpecForMutation(request.Target)
	if err != nil || failure != nil {
		return SpecProjection{}, nil, nil, collapseFailure(err, failure)
	}
	if strings.TrimSpace(string(request.Intent)) == "" {
		return SpecProjection{}, nil, nil, &Failure{
			Code:    "INVALID_INPUT",
			Message: "delta add requires --intent <add|change|remove|repair>",
			State:   withSpecFocus(loaded.state, map[string]any{"delta_add": map[string]any{"reason": "invalid_delta_input"}}),
			Next:    []any{},
		}
	}
	if !domain.IsValidDeltaIntent(string(request.Intent)) {
		return SpecProjection{}, nil, nil, &Failure{
			Code:    "INVALID_INPUT",
			Message: "--intent must be one of add|change|remove|repair",
			State:   withSpecFocus(loaded.state, map[string]any{"delta_add": map[string]any{"reason": "invalid_delta_input"}}),
			Next:    []any{},
		}
	}
	if strings.TrimSpace(request.Area) == "" {
		return SpecProjection{}, nil, nil, invalidSpecInputFailure(loaded.state, "--area is required", "area")
	}
	missingFields := make([]string, 0, 3)
	if !request.CurrentPresent {
		missingFields = append(missingFields, "current")
	}
	if !request.TargetPresent {
		missingFields = append(missingFields, "target")
	}
	if !request.NotesPresent {
		missingFields = append(missingFields, "notes")
	}
	if len(missingFields) > 0 {
		return SpecProjection{}, nil, nil, invalidSpecInputFailure(loaded.state, "required stdin fields are missing", missingFields...)
	}
	affectsRequirements := uniqueOrderedStrings(normalizeSlugList(request.AffectsRequirements))
	switch request.Intent {
	case domain.DeltaIntentChange, domain.DeltaIntentRemove, domain.DeltaIntentRepair:
		if len(affectsRequirements) == 0 {
			state := loaded.state
			state.Focus = map[string]any{"delta_add": map[string]any{"reason": "affected_requirements_required"}}
			return SpecProjection{}, nil, nil, &Failure{
				Code:    "INVALID_INPUT",
				Message: "affects_requirements is required for this delta intent",
				State:   state,
				Next:    []any{},
			}
		}
		for _, requirementID := range affectsRequirements {
			requirement := loaded.tracking.RequirementByID(requirementID)
			if requirement == nil || !requirement.IsActive() {
				state := loaded.state
				state.Focus = map[string]any{"delta_add": map[string]any{"reason": "invalid_requirement_state", "affects_requirements": affectsRequirements}}
				return SpecProjection{}, nil, nil, &Failure{
					Code:    "INVALID_INPUT",
					Message: "affects_requirements must name active requirements",
					State:   state,
					Next:    []any{},
				}
			}
		}
		if request.Intent == domain.DeltaIntentRepair {
			if failure := s.repairIntentClosedDeltaConflict(loaded, affectsRequirements); failure != nil {
				return SpecProjection{}, nil, nil, failure
			}
		}
	default:
		affectsRequirements = nil
	}

	nextID, err := loaded.tracking.AllocateNextDeltaID()
	if err != nil {
		return SpecProjection{}, nil, nil, s.validationFailure(loaded, "Cannot allocate a new delta because the stored delta IDs are invalid")
	}

	delta, err := domain.NewDelta(nextID, request.Intent, strings.TrimSpace(request.Area), loaded.tracking.Checkpoint, strings.TrimSpace(request.Current), strings.TrimSpace(request.Targets), request.Notes, affectsRequirements)
	if err != nil {
		invalidFields := make([]string, 0, 2)
		if strings.TrimSpace(request.Current) == "" {
			invalidFields = append(invalidFields, "current")
		}
		if strings.TrimSpace(request.Targets) == "" {
			invalidFields = append(invalidFields, "target")
		}
		if len(invalidFields) > 0 {
			return SpecProjection{}, nil, nil, invalidSpecInputFailure(loaded.state, err.Error(), invalidFields...)
		}
		return SpecProjection{}, nil, nil, s.specFailure(
			loaded,
			"INVALID_INPUT",
			err.Error(),
			map[string]any{"delta": map[string]any{"id": nextID}},
			[]any{},
		)
	}

	updated := cloneTracking(loaded.tracking)
	updated.Deltas = append(updated.Deltas, delta)
	updated.SyncComputedStatus()
	updated.Updated = s.todayUTC()
	resultDelta := projectDelta(delta)
	result := map[string]any{
		"kind":       "delta",
		"delta":      resultDelta,
		"allocation": map[string]any{"previous_max": len(loaded.tracking.Deltas), "allocated": delta.ID},
	}
	return s.finalizeSpecMutation(
		loaded,
		updated,
		result,
		map[string]any{"delta": resultDelta},
		func(state SpecProjection) []any {
			return buildDeltaAddNext(request.Target, loaded.tracking, delta, state.FormatTemplate)
		},
	)
}

// repairIntentClosedDeltaConflict enforces the closed-delta invariant at
// delta add time: a repair intent can only progress via req stale, which
// retroactively invalidates any closed delta that referenced the target
// requirement as part of its closure evidence. Detecting the conflict
// here avoids burning a D-id and leaving deferred residue.
func (s *Service) repairIntentClosedDeltaConflict(loaded *loadedSpec, affectsRequirements []string) *Failure {
	conflicts := make([]map[string]any, 0)
	conflictingRequirements := make([]string, 0)
	conflictingDeltasSet := make(map[string]struct{})
	for _, requirementID := range affectsRequirements {
		hit := false
		for _, closed := range loaded.tracking.Deltas {
			if closed.Status != domain.DeltaStatusClosed {
				continue
			}
			for _, affected := range closed.AffectsRequirements {
				if affected != requirementID {
					continue
				}
				conflicts = append(conflicts, map[string]any{
					"closed_delta":      closed.ID,
					"requires_verified": requirementID,
				})
				conflictingDeltasSet[closed.ID] = struct{}{}
				hit = true
			}
		}
		if hit {
			conflictingRequirements = append(conflictingRequirements, requirementID)
		}
	}
	if len(conflicts) == 0 {
		return nil
	}
	conflictingDeltas := make([]string, 0, len(conflictingDeltasSet))
	for id := range conflictingDeltasSet {
		conflictingDeltas = append(conflictingDeltas, id)
	}
	slices.Sort(conflictingDeltas)
	state := loaded.state
	state.Focus = map[string]any{
		"delta_add": map[string]any{
			"reason":    "closed_delta_invariant",
			"conflicts": conflicts,
			"suggestion": map[string]any{
				"intent":    "change",
				"rationale": "Use --intent change with req replace to preserve closed-delta verification while introducing an updated successor requirement.",
			},
		},
	}
	return &Failure{
		Code: "VALIDATION_FAILED",
		Message: fmt.Sprintf(
			"Repair intent cannot be applied to %s: closed delta(s) %s require the requirement(s) verified; repair only allows stale, which the closed-delta invariant forbids.",
			strings.Join(conflictingRequirements, ", "),
			strings.Join(conflictingDeltas, ", "),
		),
		State: state,
		Next:  []any{},
	}
}

func (s *Service) StartDelta(request DeltaTransitionRequest) (SpecProjection, map[string]any, []any, error) {
	return s.transitionDelta(request, domain.DeltaStatusOpen, domain.DeltaStatusInProgress, "delta start")
}

func (s *Service) DeferDelta(request DeltaTransitionRequest) (SpecProjection, map[string]any, []any, error) {
	return s.transitionDelta(request, "", domain.DeltaStatusDeferred, "delta defer")
}

func (s *Service) ResumeDelta(request DeltaTransitionRequest) (SpecProjection, map[string]any, []any, error) {
	return s.transitionDelta(request, domain.DeltaStatusDeferred, domain.DeltaStatusOpen, "delta resume")
}

func (s *Service) CloseDelta(request DeltaTransitionRequest) (SpecProjection, map[string]any, []any, error) {
	loaded, failure, err := s.loadSpecForMutation(request.Target)
	if err != nil || failure != nil {
		return SpecProjection{}, nil, nil, collapseFailure(err, failure)
	}

	delta := loaded.tracking.DeltaByID(request.DeltaID)
	if delta == nil {
		return SpecProjection{}, nil, nil, s.specFailure(
			loaded,
			"DELTA_NOT_FOUND",
			fmt.Sprintf("delta %s not found", request.DeltaID),
			map[string]any{"delta": map[string]any{"id": request.DeltaID}},
			[]any{},
		)
	}
	if delta.Status != domain.DeltaStatusOpen && delta.Status != domain.DeltaStatusInProgress {
		return SpecProjection{}, nil, nil, s.specFailure(
			loaded,
			"DELTA_INVALID_STATE",
			fmt.Sprintf("delta %s cannot transition from %s to closed", request.DeltaID, delta.Status),
			map[string]any{
				"delta":      deltaFailureFocus(*delta),
				"transition": "close",
			},
			[]any{},
		)
	}

	tracing := loaded.tracking.RequirementsTouchedByDelta(*delta)
	if !loaded.tracking.DeltaUpdatesResolved(delta.ID) {
		// Provide actionable detail: which affected requirements are still active?
		var stillActive []string
		if delta.Intent == domain.DeltaIntentChange {
			for _, id := range delta.AffectsRequirements {
				req := loaded.tracking.RequirementByID(id)
				if req != nil && req.IsActive() {
					stillActive = append(stillActive, id)
				}
			}
		}
		message := fmt.Sprintf("delta %s cannot be closed until required updates are resolved", request.DeltaID)
		if len(stillActive) > 0 {
			message = fmt.Sprintf("delta %s cannot close: %s still active (expected superseded). Either supersede via req replace or remove from affects_requirements",
				request.DeltaID, strings.Join(stillActive, ", "))
		}
		return SpecProjection{}, nil, nil, &Failure{
			Code:    "DELTA_UPDATES_UNRESOLVED",
			Message: message,
			State: withSpecFocus(loaded.state, map[string]any{
				"delta":       deltaFailureFocus(*delta),
				"delta_close": map[string]any{"reason": "updates_unresolved", "still_active": stillActive},
			}),
			Next: []any{},
		}
	}
	matchBlockingIDs := make([]string, 0)
	for _, requirement := range tracing {
		if !requirement.IsActive() {
			continue
		}
		context := requirementContextForID(loaded.state, requirement.ID)
		if isBlockingRequirementMatchStatus(context.MatchStatus) {
			matchBlockingIDs = append(matchBlockingIDs, requirement.ID)
		}
	}
	if len(matchBlockingIDs) > 0 {
		return SpecProjection{}, nil, nil, &Failure{
			Code:    "REQUIREMENT_MATCH_BLOCKING",
			Message: "delta close is blocked by requirement match issues",
			State: withSpecFocus(loaded.state, map[string]any{
				"delta":                    deltaFailureFocus(*delta),
				"delta_close":              map[string]any{"reason": "match_blocking"},
				"blocking_requirements":    append([]string{}, matchBlockingIDs...),
				"requirement_match_issues": requirementMatchIssuesForState(loaded.state, matchBlockingIDs, true),
			}),
			NextMode: "choose_one",
			Next:     buildRequirementMatchBlockingNext(request.Target, matchBlockingIDs[0]),
		}
	}

	blocking := make([]map[string]any, 0)
	for _, requirement := range tracing {
		if requirement.IsActive() && requirement.EffectiveVerification() != domain.RequirementVerificationVerified {
			blocking = append(blocking, map[string]any{
				"id":           requirement.ID,
				"verification": requirement.EffectiveVerification(),
			})
		}
	}
	if len(blocking) > 0 {
		blockingRequirement := firstBlockingRequirement(tracing)
		return SpecProjection{}, nil, nil, s.specFailure(
			loaded,
			"UNVERIFIED_REQUIREMENTS",
			fmt.Sprintf("Cannot close %s: %s is not verified", request.DeltaID, blockingRequirement.ID),
			map[string]any{
				"delta":                 deltaFailureFocus(*delta),
				"delta_close":           map[string]any{"reason": "requirements_unverified"},
				"blocking_requirements": blocking,
			},
			buildDeltaCloseBlockingNext(request.Target, loaded.tracking, blockingRequirement),
		)
	}

	updated := cloneTracking(loaded.tracking)
	mutated := updated.DeltaByID(request.DeltaID)
	mutated.Status = domain.DeltaStatusClosed
	updated.SyncComputedStatus()
	updated.Updated = s.todayUTC()

	return s.finalizeSpecMutation(loaded, updated, map[string]any{
		"kind":  "delta",
		"delta": projectDelta(*mutated),
	}, map[string]any{"delta": projectDelta(*mutated)}, func(state SpecProjection) []any {
		return buildDeltaCloseNext(request.Target, *mutated, state)
	})
}

func (s *Service) RebindDeltaRequirements(request DeltaRebindRequest) (SpecProjection, map[string]any, []any, error) {
	loaded, failure, err := s.loadSpecForMutation(request.Target)
	if err != nil || failure != nil {
		return SpecProjection{}, nil, nil, collapseFailure(err, failure)
	}
	from := strings.TrimSpace(request.From)
	if from == "" {
		return SpecProjection{}, nil, nil, invalidSpecInputFailure(loaded.state, "--from is required", "from")
	}
	to := strings.TrimSpace(request.To)
	if request.Remove {
		if to != "" {
			return SpecProjection{}, nil, nil, invalidSpecInputFailure(loaded.state, "--remove is mutually exclusive with --to", "to")
		}
		if strings.TrimSpace(request.Reason) == "" {
			return SpecProjection{}, nil, nil, invalidSpecInputFailure(loaded.state, "--reason is required with --remove", "reason")
		}
	} else if to == "" {
		return SpecProjection{}, nil, nil, invalidSpecInputFailure(loaded.state, "--to or --remove is required", "to")
	}
	delta := loaded.tracking.DeltaByID(request.DeltaID)
	if delta == nil {
		return SpecProjection{}, nil, nil, s.specFailure(
			loaded,
			"DELTA_NOT_FOUND",
			fmt.Sprintf("delta %s not found", request.DeltaID),
			map[string]any{"delta": map[string]any{"id": request.DeltaID}},
			[]any{},
		)
	}
	switch delta.Status {
	case domain.DeltaStatusOpen, domain.DeltaStatusInProgress, domain.DeltaStatusDeferred:
		// allowed
	default:
		return SpecProjection{}, nil, nil, s.specFailure(
			loaded,
			"DELTA_INVALID_STATE",
			fmt.Sprintf("delta %s cannot be rebound while status is %s", request.DeltaID, delta.Status),
			map[string]any{
				"delta":      deltaFailureFocus(*delta),
				"transition": "rebind",
			},
			[]any{},
		)
	}
	if !slices.Contains(delta.AffectsRequirements, from) {
		return SpecProjection{}, nil, nil, s.specFailure(
			loaded,
			"DELTA_REQUIREMENT_NOT_AFFECTED",
			fmt.Sprintf("delta %s does not list %s in affects_requirements", request.DeltaID, from),
			map[string]any{"req_rebind": map[string]any{"reason": "requirement_not_in_delta"}},
			[]any{},
		)
	}
	if !request.Remove {
		if loaded.tracking.RequirementByID(to) == nil {
			return SpecProjection{}, nil, nil, s.specFailure(
				loaded,
				"INVALID_INPUT",
				fmt.Sprintf("replacement requirement %s not found", to),
				map[string]any{"req_rebind": map[string]any{"reason": "replacement_not_found"}},
				[]any{},
			)
		}
	}

	updated := cloneTracking(loaded.tracking)
	mutated := updated.DeltaByID(request.DeltaID)
	newAR := make([]string, 0, len(mutated.AffectsRequirements))
	for _, id := range mutated.AffectsRequirements {
		if id != from {
			newAR = append(newAR, id)
			continue
		}
		if !request.Remove {
			newAR = append(newAR, to)
		}
	}
	mutated.AffectsRequirements = newAR
	updated.SyncComputedStatus()
	updated.Updated = s.todayUTC()

	detail := map[string]any{"from": from}
	if request.Remove {
		detail["removed"] = true
		detail["reason"] = strings.TrimSpace(request.Reason)
	} else {
		detail["to"] = to
	}
	return s.finalizeSpecMutation(loaded, updated, map[string]any{
		"kind":   "delta",
		"delta":  projectDelta(*mutated),
		"rebind": detail,
	}, map[string]any{"delta": projectDelta(*mutated)}, func(_ SpecProjection) []any {
		return []any{}
	})
}

func (s *Service) WithdrawDelta(request DeltaWithdrawRequest) (SpecProjection, map[string]any, []any, error) {
	loaded, failure, err := s.loadSpecForMutation(request.Target)
	if err != nil || failure != nil {
		return SpecProjection{}, nil, nil, collapseFailure(err, failure)
	}
	reason := strings.TrimSpace(request.Reason)
	if reason == "" {
		return SpecProjection{}, nil, nil, invalidSpecInputFailure(loaded.state, "--reason is required", "reason")
	}
	delta := loaded.tracking.DeltaByID(request.DeltaID)
	if delta == nil {
		return SpecProjection{}, nil, nil, s.specFailure(
			loaded,
			"DELTA_NOT_FOUND",
			fmt.Sprintf("delta %s not found", request.DeltaID),
			map[string]any{"delta": map[string]any{"id": request.DeltaID}},
			[]any{},
		)
	}
	switch delta.Status {
	case domain.DeltaStatusOpen, domain.DeltaStatusInProgress, domain.DeltaStatusDeferred:
		// allowed
	default:
		return SpecProjection{}, nil, nil, s.specFailure(
			loaded,
			"DELTA_INVALID_STATE",
			fmt.Sprintf("delta %s cannot transition from %s to withdrawn", request.DeltaID, delta.Status),
			map[string]any{
				"delta":      deltaFailureFocus(*delta),
				"transition": "withdraw",
			},
			[]any{},
		)
	}

	updated := cloneTracking(loaded.tracking)
	mutated := updated.DeltaByID(request.DeltaID)
	mutated.Status = domain.DeltaStatusWithdrawn
	mutated.WithdrawnReason = reason
	updated.SyncComputedStatus()
	updated.Updated = s.todayUTC()

	return s.finalizeSpecMutation(loaded, updated, map[string]any{
		"kind":  "delta",
		"delta": projectDelta(*mutated),
	}, map[string]any{"delta": projectDelta(*mutated)}, func(_ SpecProjection) []any {
		return []any{}
	})
}

func (s *Service) AddRequirement(request RequirementAddRequest) (SpecProjection, map[string]any, []any, error) {
	loaded, failure, err := s.loadSpecForMutation(request.Target)
	if err != nil || failure != nil {
		return SpecProjection{}, nil, nil, collapseFailure(err, failure)
	}
	if strings.TrimSpace(request.DeltaID) == "" {
		return SpecProjection{}, nil, nil, s.specFailure(
			loaded,
			"INVALID_INPUT",
			"--delta is required",
			map[string]any{"input": map[string]any{"missing_fields": []string{"delta"}}},
			[]any{},
		)
	}

	deltaID := strings.TrimSpace(request.DeltaID)
	if !domain.IsValidDeltaID(deltaID) {
		return SpecProjection{}, nil, nil, s.specFailure(
			loaded,
			"INVALID_INPUT",
			fmt.Sprintf("delta ID must be D-NNN: %s", deltaID),
			map[string]any{"delta": map[string]any{"id": deltaID}},
			[]any{},
		)
	}
	delta := loaded.tracking.DeltaByID(deltaID)
	if delta == nil {
		return SpecProjection{}, nil, nil, s.specFailure(
			loaded,
			"DELTA_NOT_FOUND",
			fmt.Sprintf("unknown delta: %s", deltaID),
			map[string]any{"delta": map[string]any{"id": deltaID}},
			[]any{},
		)
	}
	if !slices.Contains(projectDelta(*delta).Updates, "add_requirement") {
		return SpecProjection{}, nil, nil, s.specFailure(
			loaded,
			"INVALID_INPUT",
			"the selected delta does not allow req add",
			map[string]any{"req_add": map[string]any{"reason": "update_not_allowed", "delta": deltaID}},
			[]any{},
		)
	}

	nextID, err := loaded.tracking.AllocateNextRequirementID()
	if err != nil {
		return SpecProjection{}, nil, nil, s.validationFailure(loaded, "Cannot allocate a new requirement because the stored requirement IDs are invalid")
	}

	gherkin := strings.TrimRight(request.Gherkin, " \t\r\n")
	requirement, context, matchErr := s.requirementFromSpecBlock(loaded, nextID, deltaID, gherkin, "req_add")
	if matchErr != nil {
		return SpecProjection{}, nil, nil, matchErr
	}
	requirement, err = domain.NewRequirement(nextID, deltaID, requirement.Gherkin)
	if err != nil {
		code := "INVALID_GHERKIN"
		if strings.Contains(err.Error(), "gherkin tag") {
			code = "INVALID_GHERKIN_TAG"
		}
		return SpecProjection{}, nil, nil, s.specFailure(
			loaded,
			code,
			err.Error(),
			map[string]any{"requirement": map[string]any{"id": nextID}},
			[]any{},
		)
	}
	if err := domain.ValidateRequirementTagsConfigured(requirement.Tags, loaded.config.GherkinTags); err != nil {
		missingTags := domain.MissingRequirementTags(requirement.Tags, loaded.config.GherkinTags)
		next := make([]any, 0, len(missingTags)+1)
		for i, tag := range missingTags {
			next = append(next, map[string]any{
				"priority": i + 1,
				"action":   "register_tag",
				"kind":     "run_command",
				"template": map[string]any{
					"argv":            []string{"specctl", "config", "add-tag", tag},
					"required_fields": []any{},
				},
				"why": fmt.Sprintf("Register missing gherkin tag %q before retrying req add.", tag),
			})
		}
		next = append(next, map[string]any{
			"priority": len(missingTags) + 1,
			"action":   "retry_req_add",
			"kind":     "run_command",
			"template": map[string]any{
				"argv":            []string{"specctl", "req", "add", request.Target, "--delta", request.DeltaID},
				"required_fields": []any{map[string]any{"name": "gherkin_requirement", "description": "Requirement-level Gherkin block from SPEC.md"}},
				"stdin_format":    "gherkin",
			},
			"why": "After registering the missing tags, retry the requirement add.",
		})
		return SpecProjection{}, nil, nil, s.specFailure(
			loaded,
			"INVALID_GHERKIN_TAG",
			err.Error(),
			map[string]any{"requirement": map[string]any{"id": nextID, "tags": requirement.Tags, "missing_tags": missingTags}},
			next,
		)
	}

	updated := cloneTracking(loaded.tracking)
	updated.Requirements = append(updated.Requirements, requirement)
	updated.SyncComputedStatus()
	updated.Updated = s.todayUTC()
	resultRequirement := projectRequirement(requirement, context)
	result := map[string]any{
		"kind":        "requirement",
		"requirement": resultRequirement,
		"allocation":  map[string]any{"previous_max": len(loaded.tracking.Requirements), "allocated": requirement.ID},
	}
	return s.finalizeSpecMutation(
		loaded,
		updated,
		result,
		map[string]any{"requirement": resultRequirement},
		func(_ SpecProjection) []any {
			return buildImplementAndVerifyNext(
				request.Target,
				updated,
				requirement,
				context,
				"Requirement is registered but not yet verified. Implement the behavior and write tests before verifying.",
				"Each scenario maps to a test case. Implementation goes in the scope directories.",
			)
		},
	)
}

func (s *Service) ReplaceRequirement(request RequirementReplaceRequest) (SpecProjection, map[string]any, []any, error) {
	loaded, failure, err := s.loadSpecForMutation(request.Target)
	if err != nil || failure != nil {
		return SpecProjection{}, nil, nil, collapseFailure(err, failure)
	}

	requirement := loaded.tracking.RequirementByID(request.RequirementID)
	if requirement == nil {
		return SpecProjection{}, nil, nil, s.specFailure(loaded, "REQUIREMENT_NOT_FOUND", fmt.Sprintf("requirement %s not found", request.RequirementID), map[string]any{"requirement": map[string]any{"id": request.RequirementID}}, []any{})
	}
	if !requirement.IsActive() {
		return SpecProjection{}, nil, nil, s.specFailure(loaded, "INVALID_INPUT", "req replace requires an active requirement", map[string]any{"req_replace": map[string]any{"reason": "invalid_requirement_state"}, "requirement": projectRequirement(*requirement, requirementContextForID(loaded.state, requirement.ID))}, []any{})
	}

	delta, err := s.requirementDeltaForUpdate(loaded, request.DeltaID, "replace_requirement", "replacement_required", "req_replace")
	if err != nil {
		return SpecProjection{}, nil, nil, err
	}
	if !slices.Contains(delta.AffectsRequirements, request.RequirementID) {
		return SpecProjection{}, nil, nil, s.specFailure(loaded, "DELTA_REQUIREMENT_NOT_AFFECTED",
			fmt.Sprintf("delta %s does not list %s in affects_requirements", delta.ID, request.RequirementID),
			map[string]any{"req_replace": map[string]any{"reason": "requirement_not_in_delta"}}, []any{})
	}

	nextID, err := loaded.tracking.AllocateNextRequirementID()
	if err != nil {
		return SpecProjection{}, nil, nil, s.validationFailure(loaded, "Cannot allocate a new requirement because the stored requirement IDs are invalid")
	}

	newRequirementSeed, context, matchErr := s.requirementFromSpecBlock(loaded, nextID, delta.ID, request.Gherkin, "req_replace")
	if matchErr != nil {
		return SpecProjection{}, nil, nil, matchErr
	}
	if newRequirementSeed.Gherkin == requirement.Gherkin {
		return SpecProjection{}, nil, nil, s.specFailure(loaded, "INVALID_INPUT", "req replace requires a different requirement block", map[string]any{"req_replace": map[string]any{"reason": "replacement_required"}}, []any{})
	}
	newRequirement, err := domain.NewRequirement(nextID, delta.ID, newRequirementSeed.Gherkin)
	if err != nil {
		return SpecProjection{}, nil, nil, err
	}

	updated := cloneTracking(loaded.tracking)
	mutatedOld := updated.RequirementByID(request.RequirementID)
	mutatedOld.Lifecycle = domain.RequirementLifecycleSuperseded
	mutatedOld.SupersededBy = nextID

	newRequirement.Lifecycle = domain.RequirementLifecycleActive
	newRequirement.Verification = domain.RequirementVerificationUnverified
	newRequirement.Supersedes = mutatedOld.ID
	newRequirement.SupersededBy = ""
	newRequirement.IntroducedBy = delta.ID
	updated.Requirements = append(updated.Requirements, newRequirement)

	// Clear resolved updates on the parent delta — the replace is done.
	if mutatedDelta := updated.DeltaByID(delta.ID); mutatedDelta != nil {
		mutatedDelta.Updates = nil
	}

	rebinds := autoRebindAffectsRequirements(updated, delta.ID, request.RequirementID, newRequirement.ID, loaded.config)

	updated.SyncComputedStatus()
	updated.Updated = s.todayUTC()

	resultRequirement := projectRequirement(newRequirement, context)
	result := map[string]any{
		"kind":        "requirement",
		"requirement": resultRequirement,
		"allocation":  map[string]any{"previous_max": len(loaded.tracking.Requirements), "allocated": newRequirement.ID},
	}
	if len(rebinds) > 0 {
		result["auto_rebinds"] = rebinds
	}
	return s.finalizeSpecMutation(
		loaded,
		updated,
		result,
		map[string]any{
			"replaced_requirement": projectRequirement(*mutatedOld, requirementContextForID(loaded.state, mutatedOld.ID)),
			"new_requirement":      resultRequirement,
		},
		func(_ SpecProjection) []any {
			return buildImplementAndVerifyNext(
				request.Target,
				updated,
				newRequirement,
				context,
				"Replacement requirement is registered but not yet verified. Implement the changed behavior and write tests before verifying.",
				"Update the implementation and tests to match the new requirement. Each scenario maps to a test case.",
			)
		},
	)
}

// autoRebindAffectsRequirements updates the affects_requirements list of
// every open | in-progress | deferred delta that still references the
// superseded requirement, replacing it with the new requirement id.
// Closed and withdrawn deltas are never touched. The parent delta of the
// replace is skipped — it keeps the old id so its trace survives.
// Gated by config.AutoRebindOnReplace; returns the list of rebinds made
// for inclusion in the response.
func autoRebindAffectsRequirements(tracking *domain.TrackingFile, parentDeltaID, fromReq, toReq string, config *infrastructure.ProjectConfig) []map[string]any {
	if config == nil || !config.AutoRebindOnReplace {
		return nil
	}
	rebinds := make([]map[string]any, 0)
	for i := range tracking.Deltas {
		d := &tracking.Deltas[i]
		if d.ID == parentDeltaID {
			continue
		}
		if d.Status != domain.DeltaStatusOpen && d.Status != domain.DeltaStatusInProgress && d.Status != domain.DeltaStatusDeferred {
			continue
		}
		for j, req := range d.AffectsRequirements {
			if req == fromReq {
				d.AffectsRequirements[j] = toReq
				rebinds = append(rebinds, map[string]any{
					"code":  "AUTO_REBIND_APPLIED",
					"delta": d.ID,
					"from":  fromReq,
					"to":    toReq,
				})
			}
		}
	}
	return rebinds
}

func (s *Service) WithdrawRequirement(request RequirementDeltaRequest) (SpecProjection, map[string]any, []any, error) {
	loaded, failure, err := s.loadSpecForMutation(request.Target)
	if err != nil || failure != nil {
		return SpecProjection{}, nil, nil, collapseFailure(err, failure)
	}
	requirement := loaded.tracking.RequirementByID(request.RequirementID)
	if requirement == nil {
		return SpecProjection{}, nil, nil, s.specFailure(loaded, "REQUIREMENT_NOT_FOUND", fmt.Sprintf("requirement %s not found", request.RequirementID), map[string]any{"requirement": map[string]any{"id": request.RequirementID}}, []any{})
	}
	if !requirement.IsActive() {
		return SpecProjection{}, nil, nil, s.specFailure(loaded, "INVALID_INPUT", "req withdraw requires an active requirement", map[string]any{"req_withdraw": map[string]any{"reason": "invalid_requirement_state"}}, []any{})
	}
	if _, err := s.requirementDeltaForUpdate(loaded, request.DeltaID, "withdraw_requirement", "withdrawal_required", "req_withdraw"); err != nil {
		return SpecProjection{}, nil, nil, err
	}

	updated := cloneTracking(loaded.tracking)
	mutated := updated.RequirementByID(request.RequirementID)
	mutated.Lifecycle = domain.RequirementLifecycleWithdrawn

	// Clear resolved updates on the parent delta — the withdrawal is done.
	if mutatedDelta := updated.DeltaByID(request.DeltaID); mutatedDelta != nil {
		mutatedDelta.Updates = nil
	}

	updated.SyncComputedStatus()
	updated.Updated = s.todayUTC()
	resultRequirement := projectRequirement(*mutated, requirementContextForID(loaded.state, mutated.ID))
	return s.finalizeSpecMutation(
		loaded,
		updated,
		map[string]any{"kind": "requirement", "requirement": resultRequirement},
		map[string]any{"requirement": resultRequirement},
		func(_ SpecProjection) []any {
			return buildDeltaCloseSuggestions(request.Target, updated, []string{request.DeltaID})
		},
	)
}

func (s *Service) StaleRequirement(request RequirementDeltaRequest) (SpecProjection, map[string]any, []any, error) {
	loaded, failure, err := s.loadSpecForMutation(request.Target)
	if err != nil || failure != nil {
		return SpecProjection{}, nil, nil, collapseFailure(err, failure)
	}
	requirement := loaded.tracking.RequirementByID(request.RequirementID)
	if requirement == nil {
		return SpecProjection{}, nil, nil, s.specFailure(loaded, "REQUIREMENT_NOT_FOUND", fmt.Sprintf("requirement %s not found", request.RequirementID), map[string]any{"requirement": map[string]any{"id": request.RequirementID}}, []any{})
	}
	if !requirement.IsActive() {
		return SpecProjection{}, nil, nil, s.specFailure(loaded, "INVALID_INPUT", "req stale requires an active requirement", map[string]any{"req_stale": map[string]any{"reason": "invalid_requirement_state"}}, []any{})
	}
	if _, err := s.requirementDeltaForUpdate(loaded, request.DeltaID, "stale_requirement", "stale_required", "req_stale"); err != nil {
		return SpecProjection{}, nil, nil, err
	}

	updated := cloneTracking(loaded.tracking)
	mutated := updated.RequirementByID(request.RequirementID)
	mutated.Verification = domain.RequirementVerificationStale
	updated.SyncComputedStatus()
	updated.Updated = s.todayUTC()
	resultRequirement := projectRequirement(*mutated, requirementContextForID(loaded.state, mutated.ID))
	return s.finalizeSpecMutation(
		loaded,
		updated,
		map[string]any{"kind": "requirement", "requirement": resultRequirement},
		map[string]any{"requirement": resultRequirement},
		func(_ SpecProjection) []any {
			return buildImplementAndVerifyNext(
				request.Target,
				updated,
				*mutated,
				requirementContextForID(loaded.state, mutated.ID),
				"Requirement evidence is stale. Fix the code or tests before verifying the requirement again.",
				"Fix the code or tests so the requirement holds again.",
			)
		},
	)
}

func (s *Service) RefreshRequirement(request RequirementRefreshRequest) (SpecProjection, map[string]any, []any, error) {
	loaded, failure, err := s.loadSpecForMutation(request.Target)
	if err != nil || failure != nil {
		return SpecProjection{}, nil, nil, collapseFailure(err, failure)
	}
	requirement := loaded.tracking.RequirementByID(request.RequirementID)
	if requirement == nil {
		return SpecProjection{}, nil, nil, s.specFailure(loaded, "REQUIREMENT_NOT_FOUND", fmt.Sprintf("requirement %s not found", request.RequirementID), map[string]any{"requirement": map[string]any{"id": request.RequirementID}}, []any{})
	}
	if !requirement.IsActive() {
		return SpecProjection{}, nil, nil, s.specFailure(loaded, "INVALID_INPUT", "req refresh requires an active requirement", map[string]any{"req_refresh": map[string]any{"reason": "invalid_requirement_state"}}, []any{})
	}

	resolved, context, matchErr := s.requirementBlockMatch(loaded, request.Gherkin, "")
	if matchErr != nil {
		return SpecProjection{}, nil, nil, matchErr
	}
	if resolved == requirement.Gherkin {
		return SpecProjection{}, nil, nil, s.specFailure(loaded, "INVALID_INPUT", "req refresh is only legal when the stored requirement block differs", map[string]any{"req_refresh": map[string]any{"reason": "match_refresh_not_needed"}}, []any{})
	}
	for _, deltaID := range loaded.tracking.DeltasTouchingRequirement(requirement.ID) {
		delta := loaded.tracking.DeltaByID(deltaID)
		if delta == nil || delta.Status == domain.DeltaStatusClosed || delta.Status == domain.DeltaStatusDeferred {
			continue
		}
		if delta.Intent == domain.DeltaIntentChange || delta.Intent == domain.DeltaIntentRemove || delta.Intent == domain.DeltaIntentRepair {
			return SpecProjection{}, nil, nil, s.specFailure(loaded, "INVALID_INPUT", "req refresh conflicts with already-recorded workflow state", map[string]any{"req_refresh": map[string]any{"reason": "recorded_workflow_conflict"}, "delta": projectDelta(*delta)}, []any{})
		}
	}

	title, _ := domain.DeriveRequirementTitle(resolved)
	tags, _ := domain.DeriveRequirementTags(resolved)
	updated := cloneTracking(loaded.tracking)
	mutated := updated.RequirementByID(request.RequirementID)
	mutated.Title = title
	mutated.Tags = tags
	mutated.Gherkin = resolved
	updated.SyncComputedStatus()
	updated.Updated = s.todayUTC()
	resultRequirement := projectRequirement(*mutated, context)
	return s.finalizeSpecMutation(
		loaded,
		updated,
		map[string]any{"kind": "requirement", "requirement": resultRequirement},
		map[string]any{"requirement": resultRequirement},
		func(state SpecProjection) []any {
			return buildTrackedDriftContinuationNext(state, request.Target)
		},
	)
}

func (s *Service) requirementDeltaForUpdate(loaded *loadedSpec, deltaID, requiredUpdate, reason, focusKey string) (*domain.Delta, error) {
	if strings.TrimSpace(deltaID) == "" {
		return nil, s.specFailure(loaded, "INVALID_INPUT", "--delta is required", map[string]any{"input": map[string]any{"missing_fields": []string{"delta"}}}, []any{})
	}
	delta := loaded.tracking.DeltaByID(strings.TrimSpace(deltaID))
	if delta == nil {
		return nil, s.specFailure(loaded, "DELTA_NOT_FOUND", fmt.Sprintf("unknown delta: %s", deltaID), map[string]any{"delta": map[string]any{"id": strings.TrimSpace(deltaID)}}, []any{})
	}
	if !slices.Contains(projectDelta(*delta).Updates, requiredUpdate) {
		return nil, s.specFailure(loaded, "INVALID_INPUT", fmt.Sprintf("the selected delta does not allow %s", strings.ReplaceAll(focusKey, "_", " ")), map[string]any{focusKey: map[string]any{"reason": reason, "delta": delta.ID}}, []any{})
	}
	return delta, nil
}

func (s *Service) requirementFromSpecBlock(loaded *loadedSpec, requirementID, deltaID, gherkin, workflow string) (domain.Requirement, infrastructure.RequirementDocContext, error) {
	resolved, context, err := s.requirementBlockMatch(loaded, gherkin, workflow)
	if err != nil {
		return domain.Requirement{}, infrastructure.RequirementDocContext{}, err
	}
	return domain.Requirement{ID: requirementID, Gherkin: resolved, IntroducedBy: deltaID}, context, nil
}

func (s *Service) requirementBlockMatch(loaded *loadedSpec, gherkin, workflow string) (string, infrastructure.RequirementDocContext, error) {
	normalized, err := domain.NormalizeRequirementBlock(gherkin)
	if err != nil {
		code := "INVALID_GHERKIN"
		if strings.Contains(err.Error(), "gherkin tag") {
			code = "INVALID_GHERKIN_TAG"
		}
		return "", infrastructure.RequirementDocContext{}, s.specFailure(loaded, code, err.Error(), map[string]any{}, []any{})
	}
	title, err := domain.DeriveRequirementTitle(normalized)
	if err != nil {
		return "", infrastructure.RequirementDocContext{}, s.specFailure(loaded, "INVALID_GHERKIN", err.Error(), map[string]any{}, []any{})
	}
	tags, err := domain.DeriveRequirementTags(normalized)
	if err != nil {
		code := "INVALID_GHERKIN"
		if strings.Contains(err.Error(), "gherkin tag") {
			code = "INVALID_GHERKIN_TAG"
		}
		return "", infrastructure.RequirementDocContext{}, s.specFailure(loaded, code, err.Error(), map[string]any{}, []any{})
	}
	if err := domain.ValidateRequirementTagsConfigured(tags, loaded.config.GherkinTags); err != nil {
		return "", infrastructure.RequirementDocContext{}, s.specFailure(loaded, "INVALID_GHERKIN_TAG", err.Error(), map[string]any{"requirement": map[string]any{"tags": tags}}, []any{})
	}

	contexts, err := infrastructure.ReadRequirementContexts(s.repoRoot, loaded.tracking.Documents.Primary)
	if err != nil {
		return "", infrastructure.RequirementDocContext{}, err
	}
	exactMatches := make([]infrastructure.RequirementContext, 0)
	titleMatches := make([]infrastructure.RequirementContext, 0)
	for _, context := range contexts {
		if context.Gherkin == normalized {
			exactMatches = append(exactMatches, context)
		}
		if context.Title == title {
			titleMatches = append(titleMatches, context)
		}
	}
	if len(exactMatches) == 1 {
		return normalized, infrastructure.RequirementDocContext{
			MatchStatus: "matched",
			Heading:     exactMatches[0].Heading,
			Scenarios:   append([]string{}, exactMatches[0].Scenarios...),
		}, nil
	}
	if len(exactMatches) > 1 || len(titleMatches) > 1 {
		return "", infrastructure.RequirementDocContext{}, s.specFailure(loaded, "REQUIREMENT_DUPLICATE_IN_SPEC", "requirement block appears multiple times in SPEC.md", map[string]any{"requirement_match_issues": []map[string]any{{"status": "duplicate_in_spec"}}}, []any{})
	}
	if len(titleMatches) == 1 {
		return "", infrastructure.RequirementDocContext{}, requirementNotInSpecFailure(loaded, workflow, normalized, "exact requirement block was not found in SPEC.md", "no_exact_match")
	}
	return "", infrastructure.RequirementDocContext{}, requirementNotInSpecFailure(loaded, workflow, normalized, "requirement block was not found in SPEC.md", "missing_in_spec")
}

func requirementContextForID(state SpecProjection, requirementID string) infrastructure.RequirementDocContext {
	for _, requirement := range state.Requirements {
		if requirement.ID != requirementID {
			continue
		}
		context := infrastructure.RequirementDocContext{
			MatchStatus: requirement.Match.Status,
			Scenarios:   append([]string{}, requirement.SpecContext.Scenarios...),
		}
		if requirement.Match.Heading != nil {
			context.Heading = *requirement.Match.Heading
		}
		return context
	}
	return infrastructure.RequirementDocContext{}
}

func (s *Service) VerifyRequirement(request RequirementVerifyRequest) (SpecProjection, map[string]any, []any, error) {
	loaded, failure, err := s.loadSpecForMutation(request.Target)
	if err != nil || failure != nil {
		return SpecProjection{}, nil, nil, collapseFailure(err, failure)
	}

	requirement := loaded.tracking.RequirementByID(request.RequirementID)
	if requirement == nil {
		return SpecProjection{}, nil, nil, s.specFailure(
			loaded,
			"REQUIREMENT_NOT_FOUND",
			fmt.Sprintf("requirement %s not found", request.RequirementID),
			map[string]any{"requirement": map[string]any{"id": request.RequirementID}},
			[]any{},
		)
	}
	requirementContext := requirementContextForID(loaded.state, requirement.ID)
	if !requirement.IsActive() {
		return SpecProjection{}, nil, nil, &Failure{
			Code:    "REQUIREMENT_INVALID_LIFECYCLE",
			Message: "req verify is legal only for active requirements",
			State: withSpecFocus(loaded.state, map[string]any{
				"req_verify":  map[string]any{"reason": "requirement_not_active"},
				"requirement": projectRequirement(*requirement, requirementContext),
			}),
			Next: []any{},
		}
	}
	if isBlockingRequirementMatchStatus(requirementContext.MatchStatus) {
		return SpecProjection{}, nil, nil, &Failure{
			Code:    "REQUIREMENT_MATCH_BLOCKING",
			Message: "req verify is blocked by requirement match issues",
			State: withSpecFocus(loaded.state, map[string]any{
				"req_verify":               map[string]any{"reason": "match_blocking"},
				"requirement":              projectRequirement(*requirement, requirementContext),
				"requirement_match_issues": requirementMatchIssuesForState(loaded.state, []string{requirement.ID}, true),
			}),
			NextMode: "choose_one",
			Next:     buildRequirementMatchBlockingNext(request.Target, requirement.ID),
		}
	}

	testFiles, err := s.pathAdapter().NormalizeVerifyFiles(request.TestFiles)
	if err != nil {
		var normalizeErr *infrastructure.VerifyFilesNormalizationError
		if errors.As(err, &normalizeErr) {
			focusKey := "test_files"
			if normalizeErr.Code == infrastructure.VerifyFilesInvalidPath {
				focusKey = "invalid_paths"
			}
			return SpecProjection{}, nil, nil, s.specFailure(
				loaded,
				"TEST_FILE_NOT_FOUND",
				normalizeErr.Error(),
				map[string]any{
					"requirement": *requirement,
					focusKey:      append([]string{}, normalizeErr.Paths...),
				},
				[]any{},
			)
		}
		return SpecProjection{}, nil, nil, err
	}
	if len(testFiles) == 0 && !requirement.IsManual() {
		return SpecProjection{}, nil, nil, &Failure{
			Code:    "TEST_FILES_REQUIRED",
			Message: "--test-file is required for non-manual requirements",
			State: withSpecFocus(loaded.state, map[string]any{
				"requirement": *requirement,
				"input":       map[string]any{"missing_fields": []string{"test_file"}},
			}),
			NextMode: "choose_one",
			Next: []any{
				map[string]any{
					"priority": 1,
					"action":   "provide_test_file",
					"kind":     "run_command",
					"template": map[string]any{
						"argv":            []string{"specctl", "req", "verify", request.Target, requirement.ID, "--test-file", "<path>"},
						"required_fields": []any{map[string]any{"name": "path", "description": "Repo-relative path to the test file that verifies this requirement"}},
					},
					"why": "Provide the test file that exercises the behavior described by this requirement.",
				},
				map[string]any{
					"priority":    2,
					"action":      "mark_as_manual",
					"kind":        "guidance",
					"choose_when": "The requirement describes behavior that is proven by infrastructure operation (e.g., process isolation, boot sequences), by code inspection (e.g., architectural import boundaries), or by integration evidence that cannot be captured in a unit test. Add @manual to the requirement's Gherkin tag line in SPEC.md, then run req refresh to sync, then retry req verify without --test-file.",
					"template": map[string]any{
						"steps": []string{
							"Add @manual to the requirement's @tag line in SPEC.md (e.g., '@e2e @testing @manual')",
							fmt.Sprintf("specctl req refresh %s %s  (pipe updated Gherkin on stdin)", request.Target, requirement.ID),
							fmt.Sprintf("specctl req verify %s %s  (no --test-file needed for @manual)", request.Target, requirement.ID),
						},
					},
					"why": "Requirements tagged @manual can be verified without test files. Use this for behaviors proven by infrastructure operation, architectural inspection, or integration evidence that cannot be captured in an isolated test.",
				},
			},
		}
	}

	if requirement.EffectiveVerification() == domain.RequirementVerificationVerified && stringSlicesEqual(mergeTestFiles(requirement.TestFiles, testFiles), requirement.TestFiles) {
		projected := projectRequirement(*requirement, requirementContext)
		state := withSpecFocus(loaded.state, map[string]any{"requirement": projected})
		return state, map[string]any{
			"kind":        "requirement",
			"requirement": projected,
		}, buildDeltaCloseSuggestions(request.Target, loaded.tracking, loaded.tracking.DeltasTouchingRequirement(requirement.ID)), nil
	}

	updated := cloneTracking(loaded.tracking)
	mutated := updated.RequirementByID(request.RequirementID)
	mutated.Verification = domain.RequirementVerificationVerified
	mutated.TestFiles = mergeTestFiles(mutated.TestFiles, testFiles)
	updated.SyncComputedStatus()
	updated.Updated = s.todayUTC()
	resultRequirement := projectRequirement(*mutated, requirementContext)
	result := map[string]any{
		"kind":        "requirement",
		"requirement": resultRequirement,
	}
	return s.finalizeSpecMutation(
		loaded,
		updated,
		result,
		map[string]any{"requirement": resultRequirement},
		func(_ SpecProjection) []any {
			return buildDeltaCloseSuggestions(request.Target, updated, updated.DeltasTouchingRequirement(mutated.ID))
		},
	)
}

func (s *Service) BumpRevision(request RevisionBumpRequest) (SpecProjection, map[string]any, []any, error) {
	loaded, failure, err := s.loadSpecForMutation(request.Target)
	if err != nil || failure != nil {
		return SpecProjection{}, nil, nil, collapseFailure(err, failure)
	}
	if strings.TrimSpace(request.Checkpoint) == "" {
		return SpecProjection{}, nil, nil, revBumpInvalidInputFailure(loaded.state, "--checkpoint is required", "missing_checkpoint")
	}
	if strings.TrimSpace(request.Summary) == "" {
		return SpecProjection{}, nil, nil, revBumpInvalidInputFailure(loaded.state, "summary is required", "missing_summary")
	}

	if loaded.tracking.Status != domain.SpecStatusVerified {
		return SpecProjection{}, nil, nil, revBumpInvalidInputFailure(loaded.state, "rev bump requires a verified spec", "status_not_verified")
	}
	if blocking := blockingActiveRequirementIDs(loaded.state); len(blocking) > 0 {
		return SpecProjection{}, nil, nil, &Failure{
			Code:    "REQUIREMENT_MATCH_BLOCKING",
			Message: "rev bump is blocked by requirement match issues",
			State: withSpecFocus(loaded.state, map[string]any{
				"rev_bump":                 map[string]any{"reason": "match_blocking"},
				"requirement_match_issues": requirementMatchIssuesForState(loaded.state, blocking, true),
			}),
			NextMode: "choose_one",
			Next:     buildRequirementMatchBlockingNext(request.Target, blocking[0]),
		}
	}

	resolvedCheckpoint, err := s.checkpointStore().ResolveCheckpoint(strings.TrimSpace(request.Checkpoint))
	if err != nil {
		return SpecProjection{}, nil, nil, revBumpCheckpointUnavailableFailure(loaded, request.Checkpoint, err.Error())
	}

	unbumped := closedDeltasNotInChangelog(loaded.tracking)
	unbumpedWithdrawn := withdrawnDeltasNotInChangelog(loaded.tracking)
	if len(unbumped) == 0 && len(unbumpedWithdrawn) == 0 {
		return SpecProjection{}, nil, nil, revBumpInvalidInputFailure(loaded.state, "rev bump requires closed deltas not yet recorded in the changelog", "no_semantic_changes")
	}

	comparison, err := s.checkpointStore().LoadSpecComparison(loaded.charterName, loaded.slug, loaded.tracking, loaded.tracking.Checkpoint)
	var baselineTracking *domain.TrackingFile
	if err == nil {
		baselineTracking = comparison.BaselineTracking
	}

	now := s.todayUTC()
	entry := buildChangelogEntry(baselineTracking, loaded.tracking, loaded.tracking.Rev+1, now, strings.TrimSpace(request.Summary))
	// Override baseline-computed fields with changelog-aware values.
	// The baseline comparison can miss items that existed before the first checkpoint.
	// The changelog is the authoritative record — anything not yet in any entry gets recorded.
	entry.DeltasClosed = unbumped
	entry.DeltasWithdrawn = unbumpedWithdrawn
	entry.DeltasOpened = openedDeltasNotInChangelog(loaded.tracking)
	entry.ReqsAdded = requirementsNotInChangelog(loaded.tracking)
	entry.ReqsVerified = verifiedRequirementsNotInChangelog(loaded.tracking)
	updated := cloneTracking(loaded.tracking)
	updated.Changelog = append(updated.Changelog, entry)
	updated.Rev++
	updated.Checkpoint = resolvedCheckpoint
	updated.LastVerifiedAt = now
	updated.Updated = now
	updated.SyncComputedStatus()

	return s.finalizeSpecMutation(loaded, updated, map[string]any{
		"kind":                "revision",
		"previous_rev":        loaded.tracking.Rev,
		"rev":                 updated.Rev,
		"previous_checkpoint": loaded.tracking.Checkpoint,
		"checkpoint":          resolvedCheckpoint,
		"changelog_entry":     entry,
	}, map[string]any{"changelog_entry": entry}, func(_ SpecProjection) []any { return []any{} })
}

func (s *Service) DocAdd(request DocAddRequest) (SpecProjection, map[string]any, []any, error) {
	loaded, failure, err := s.loadSpecForMutation(request.Target)
	if err != nil || failure != nil {
		return SpecProjection{}, nil, nil, collapseFailure(err, failure)
	}
	doc := strings.TrimSpace(request.Doc)
	if doc == "" {
		return SpecProjection{}, nil, nil, s.specFailure(loaded, "INVALID_INPUT", "--doc is required", map[string]any{}, []any{})
	}
	normalized, normErr := domain.NormalizeRepoPath(doc)
	if normErr != nil {
		return SpecProjection{}, nil, nil, s.specFailure(loaded, "INVALID_PATH", fmt.Sprintf("invalid document path: %s", normErr), map[string]any{"doc": doc}, []any{})
	}
	if !strings.HasSuffix(normalized, ".md") {
		return SpecProjection{}, nil, nil, s.specFailure(loaded, "INVALID_PATH", "document must point to a markdown file", map[string]any{"doc": normalized}, []any{})
	}
	if normalized == loaded.tracking.Documents.Primary {
		return SpecProjection{}, nil, nil, s.specFailure(loaded, "INVALID_INPUT", "document is already the primary document", map[string]any{"doc": normalized}, []any{})
	}
	for _, existing := range loaded.tracking.Documents.Secondary {
		if existing == normalized {
			return SpecProjection{}, nil, nil, s.specFailure(loaded, "INVALID_INPUT", fmt.Sprintf("document %q is already a secondary document", normalized), map[string]any{"doc": normalized}, []any{})
		}
	}

	updated := cloneTracking(loaded.tracking)
	updated.Documents.Secondary = append(updated.Documents.Secondary, normalized)
	updated.Updated = s.todayUTC()

	return s.finalizeSpecMutation(loaded, updated, map[string]any{
		"kind": "doc_add",
		"doc":  normalized,
	}, map[string]any{"doc": normalized}, func(_ SpecProjection) []any { return []any{} })
}

func (s *Service) DocRemove(request DocRemoveRequest) (SpecProjection, map[string]any, []any, error) {
	loaded, failure, err := s.loadSpecForMutation(request.Target)
	if err != nil || failure != nil {
		return SpecProjection{}, nil, nil, collapseFailure(err, failure)
	}
	doc := strings.TrimSpace(request.Doc)
	if doc == "" {
		return SpecProjection{}, nil, nil, s.specFailure(loaded, "INVALID_INPUT", "--doc is required", map[string]any{}, []any{})
	}
	normalized, normErr := domain.NormalizeRepoPath(doc)
	if normErr != nil {
		normalized = doc
	}

	found := -1
	for i, existing := range loaded.tracking.Documents.Secondary {
		if existing == normalized {
			found = i
			break
		}
	}
	if found == -1 {
		return SpecProjection{}, nil, nil, s.specFailure(loaded, "DOC_NOT_FOUND", fmt.Sprintf("document %q is not a secondary document", normalized), map[string]any{"doc": normalized}, []any{})
	}

	updated := cloneTracking(loaded.tracking)
	updated.Documents.Secondary = append(updated.Documents.Secondary[:found], updated.Documents.Secondary[found+1:]...)
	updated.Updated = s.todayUTC()

	return s.finalizeSpecMutation(loaded, updated, map[string]any{
		"kind": "doc_remove",
		"doc":  normalized,
	}, map[string]any{"doc": normalized}, func(_ SpecProjection) []any { return []any{} })
}

func (s *Service) Sync(request SyncRequest) (SpecProjection, map[string]any, []any, error) {
	loaded, failure, err := s.loadSpecForMutation(request.Target)
	if err != nil || failure != nil {
		return SpecProjection{}, nil, nil, collapseFailure(err, failure)
	}
	if strings.TrimSpace(request.Checkpoint) == "" {
		return SpecProjection{}, nil, nil, syncInvalidInputFailure(loaded.state, "--checkpoint is required", "missing_checkpoint")
	}

	resolvedCheckpoint, err := s.checkpointStore().ResolveCheckpoint(strings.TrimSpace(request.Checkpoint))
	if err != nil {
		return SpecProjection{}, nil, nil, syncCheckpointUnavailableFailure(loaded, request.Checkpoint, err.Error())
	}
	if strings.TrimSpace(request.Summary) == "" {
		return SpecProjection{}, nil, nil, syncInvalidInputFailure(loaded.state, "summary is required", "missing_summary")
	}
	if blocking := blockingActiveRequirementIDs(loaded.state); len(blocking) > 0 {
		return SpecProjection{}, nil, nil, syncInvalidInputFailure(withSpecFocus(loaded.state, map[string]any{
			"sync":                     map[string]any{"reason": "match_blocking"},
			"requirement_match_issues": requirementMatchIssuesForState(loaded.state, blocking, true),
		}), "sync is blocked by requirement match issues", "match_blocking")
	}
	for _, delta := range loaded.tracking.Deltas {
		if delta.Status == domain.DeltaStatusClosed || delta.Status == domain.DeltaStatusDeferred {
			continue
		}
		return SpecProjection{}, nil, nil, syncInvalidInputFailure(withSpecFocus(loaded.state, map[string]any{
			"sync":  map[string]any{"reason": "live_deltas_present"},
			"delta": deltaFailureFocus(delta),
		}), "sync is illegal while live deltas remain", "live_deltas_present")
	}
	now := s.todayUTC()
	updated := cloneTracking(loaded.tracking)
	updated.Checkpoint = resolvedCheckpoint
	updated.LastVerifiedAt = now
	updated.Updated = now
	updated.SyncComputedStatus()

	return s.finalizeSpecMutation(loaded, updated, map[string]any{
		"kind":                "sync",
		"previous_checkpoint": loaded.tracking.Checkpoint,
		"checkpoint":          resolvedCheckpoint,
	}, map[string]any{"checkpoint": resolvedCheckpoint}, func(_ SpecProjection) []any { return []any{} })
}

func (s *Service) transitionDelta(request DeltaTransitionRequest, from, to domain.DeltaStatus, transition string) (SpecProjection, map[string]any, []any, error) {
	loaded, failure, err := s.loadSpecForMutation(request.Target)
	if err != nil || failure != nil {
		return SpecProjection{}, nil, nil, collapseFailure(err, failure)
	}

	delta := loaded.tracking.DeltaByID(request.DeltaID)
	if delta == nil {
		return SpecProjection{}, nil, nil, s.specFailure(
			loaded,
			"DELTA_NOT_FOUND",
			fmt.Sprintf("delta %s not found", request.DeltaID),
			map[string]any{"delta": map[string]any{"id": request.DeltaID}},
			[]any{},
		)
	}

	valid := false
	switch transition {
	case "delta start":
		valid = delta.Status == domain.DeltaStatusOpen
	case "delta defer":
		valid = delta.Status == domain.DeltaStatusOpen || delta.Status == domain.DeltaStatusInProgress
	case "delta resume":
		valid = delta.Status == domain.DeltaStatusDeferred
	default:
		valid = from == "" || delta.Status == from
	}
	if !valid {
		return SpecProjection{}, nil, nil, s.specFailure(
			loaded,
			"DELTA_INVALID_STATE",
			fmt.Sprintf("delta %s cannot transition from %s", request.DeltaID, delta.Status),
			map[string]any{
				"delta":      deltaFailureFocus(*delta),
				"transition": strings.TrimPrefix(transition, "delta "),
			},
			[]any{},
		)
	}

	updated := cloneTracking(loaded.tracking)
	mutated := updated.DeltaByID(request.DeltaID)
	mutated.Status = to
	updated.SyncComputedStatus()
	updated.Updated = s.todayUTC()
	return s.finalizeSpecMutation(loaded, updated, map[string]any{
		"kind":  "delta",
		"delta": *mutated,
	}, map[string]any{"delta": *mutated}, func(_ SpecProjection) []any {
		if transition == "delta resume" {
			return []any{}
		}
		return buildDeltaTransitionNext(request.Target, *mutated)
	})
}

func (s *Service) loadSpecForMutation(target string) (*loadedSpec, *Failure, error) {
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
		return nil, &Failure{
			Code:    "SPEC_NOT_FOUND",
			Message: fmt.Sprintf("spec %q does not exist", target),
			State: MissingSpecContext{
				Target:        target,
				TrackingFile:  infrastructure.RelativeTrackingPath(s.repoRoot, charterName, slug),
				CharterExists: charterExists,
			},
			Next: []any{},
		}, nil
	}

	config, err := s.registryStore().LoadProjectConfig()
	if err != nil {
		return nil, nil, err
	}

	var charter *domain.Charter
	tracking, err := s.registryStore().LoadTracking(charterName, slug)
	if err != nil {
		rawTracking, findings, lenientErr := s.registryStore().LoadTrackingLenient(charterName, slug)
		if lenientErr != nil {
			return nil, nil, err
		}
		charter, failure, charterErr := s.loadCharterForSpecMutation(charterName, slug, rawTracking, config)
		if charterErr != nil || failure != nil {
			return nil, failure, charterErr
		}
		state, projectionErr := s.projectSpec(rawTracking, charter, config, findings)
		if projectionErr != nil {
			return nil, nil, err
		}
		state.Validation = validationProjection(findings)
		state.Focus = map[string]any{"validation": map[string]any{"findings": findingsAsAny(findings)}}
		return nil, &Failure{
			Code:    "VALIDATION_FAILED",
			Message: "Cannot apply the write because the stored spec state is invalid",
			State:   state,
			Next: validationRepairNext(
				"Repair the malformed tracking file before applying additional spec mutations.",
				infrastructure.RelativeTrackingPath(s.repoRoot, charterName, slug),
			),
		}, nil
	}

	charter, failure, err := s.loadCharterForSpecMutation(charterName, slug, tracking, config)
	if err != nil || failure != nil {
		return nil, failure, err
	}

	state, err := s.projectSpec(tracking, charter, config, nil)
	if err != nil {
		return nil, nil, err
	}

	return &loadedSpec{
		charterName: charterName,
		slug:        slug,
		tracking:    tracking,
		charter:     charter,
		config:      config,
		state:       state,
		relative:    s.pathAdapter().TrackingRelativePath(charterName, slug),
	}, nil, nil
}

func (s *Service) validationFailure(loaded *loadedSpec, message string) error {
	rawTracking, findings, err := s.registryStore().LoadTrackingLenient(loaded.charterName, loaded.slug)
	if err != nil {
		return err
	}
	state, err := s.projectSpec(rawTracking, loaded.charter, loaded.config, findings)
	if err != nil {
		return err
	}
	return validationFailureWithState(
		state,
		message,
		findings,
		validationRepairNext(
			"Repair the malformed tracking file through a migration or replacement spec before adding new items.",
			loaded.relative,
			infrastructure.RelativeCharterPath(loaded.charterName),
		),
	)
}

func (s *Service) loadCharterForSpecMutation(charterName, slug string, tracking *domain.TrackingFile, config *infrastructure.ProjectConfig) (*domain.Charter, *Failure, error) {
	charterExists, err := s.pathAdapter().CharterExists(charterName)
	if err != nil {
		return nil, nil, fmt.Errorf("checking charter path: %w", err)
	}
	if !charterExists {
		findings := []infrastructure.ValidationFinding{{
			Code:     "CHARTER_SPEC_MISSING",
			Severity: "error",
			Message:  fmt.Sprintf("charter file does not exist: %s", infrastructure.RelativeCharterPath(charterName)),
			Path:     infrastructure.RelativeCharterPath(charterName),
			Target:   slug,
		}}
		failure, failureErr := s.charterValidationFailure(charterName, slug, tracking, nil, config, "Cannot apply the write because charter membership is missing", findings)
		return nil, failure, failureErr
	}

	charter, err := s.registryStore().LoadCharterStructure(charterName)
	if err != nil {
		findings := validationFindingsFromMessage(infrastructure.RelativeCharterPath(charterName), slug, err.Error())
		failure, failureErr := s.charterValidationFailure(charterName, slug, tracking, nil, config, "Cannot apply the write because the charter registry is invalid", findings)
		return nil, failure, failureErr
	}

	membership := charter.SpecBySlug(slug)
	if membership == nil {
		findings := []infrastructure.ValidationFinding{{
			Code:     "CHARTER_SPEC_MISSING",
			Severity: "error",
			Message:  fmt.Sprintf("charter %q does not list spec %q", charterName, slug),
			Path:     infrastructure.RelativeCharterPath(charterName),
			Target:   slug,
		}}
		failure, failureErr := s.charterValidationFailure(charterName, slug, tracking, charter, config, "Cannot apply the write because the tracking file is not registered in the charter", findings)
		return nil, failure, failureErr
	}
	if charter.GroupByKey(membership.Group) == nil {
		findings := []infrastructure.ValidationFinding{{
			Code:     "CHARTER_GROUP_MISSING",
			Severity: "error",
			Message:  fmt.Sprintf("charter membership for %q references unknown group %q", slug, membership.Group),
			Path:     infrastructure.RelativeCharterPath(charterName),
			Target:   slug,
		}}
		failure, failureErr := s.charterValidationFailure(charterName, slug, tracking, charter, config, "Cannot apply the write because the charter membership is invalid", findings)
		return nil, failure, failureErr
	}

	return charter, nil, nil
}

func (s *Service) charterValidationFailure(charterName, slug string, tracking *domain.TrackingFile, charter *domain.Charter, config *infrastructure.ProjectConfig, message string, findings []infrastructure.ValidationFinding) (*Failure, error) {
	state, err := s.projectSpec(tracking, charter, config, findings)
	if err != nil {
		return nil, err
	}
	failure := validationFailureWithState(
		state,
		message,
		findings,
		validationRepairNext(
			"Repair the charter membership before applying additional spec mutations.",
			infrastructure.RelativeCharterPath(charterName),
			infrastructure.RelativeTrackingPath(s.repoRoot, charterName, slug),
		),
	)
	return failure.(*Failure), nil
}

func collapseFailure(err error, failure *Failure) error {
	if err != nil {
		return err
	}
	return failure
}

func cloneTracking(source *domain.TrackingFile) *domain.TrackingFile {
	cloned := *source
	cloned.Tags = append([]string{}, source.Tags...)
	cloned.Scope = append([]string{}, source.Scope...)
	cloned.Documents = domain.Documents{
		Primary:   source.Documents.Primary,
		Secondary: append([]string{}, source.Documents.Secondary...),
	}
	cloned.Deltas = append([]domain.Delta{}, source.Deltas...)
	cloned.Requirements = append([]domain.Requirement{}, source.Requirements...)
	cloned.Changelog = append([]domain.ChangelogEntry{}, source.Changelog...)
	return &cloned
}

func normalizeTagList(values []string) []string {
	trimmed := make([]string, 0, len(values))
	for _, value := range values {
		if v := strings.TrimSpace(value); v != "" {
			trimmed = append(trimmed, v)
		}
	}
	return trimmed
}

func normalizeSlugList(values []string) []string {
	trimmed := make([]string, 0, len(values))
	for _, value := range values {
		if v := strings.TrimSpace(value); v != "" {
			trimmed = append(trimmed, v)
		}
	}
	return trimmed
}

func uniqueOrderedStrings(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}

// mergeTestFiles combines existing and new test file paths, deduplicates,
// and returns a sorted result. This preserves prior verification evidence
// when new proof is added (e.g., during repair workflows).
func mergeTestFiles(existing, incoming []string) []string {
	seen := make(map[string]struct{}, len(existing)+len(incoming))
	for _, f := range existing {
		seen[f] = struct{}{}
	}
	for _, f := range incoming {
		seen[f] = struct{}{}
	}
	merged := make([]string, 0, len(seen))
	for f := range seen {
		merged = append(merged, f)
	}
	slices.Sort(merged)
	return merged
}

func stringSlicesEqual(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for i := range left {
		if left[i] != right[i] {
			return false
		}
	}
	return true
}

func isBlockingRequirementMatchStatus(status string) bool {
	switch status {
	case "no_exact_match", "missing_in_spec", "duplicate_in_spec":
		return true
	default:
		return false
	}
}

func validationProjection(findings []infrastructure.ValidationFinding) ValidationProjection {
	return ValidationProjection{
		Valid:    false,
		Findings: findingsAsAny(findings),
	}
}

func specCreateTargetResolutionValidationFailure(message, target, charterName, slug string, findings []infrastructure.ValidationFinding, next []any) error {
	state := specCreateFailureState(target, charterName, slug)
	state["validation"] = validationProjection(findings)
	state["focus"] = validationFailureFocus(findings)
	return &Failure{
		Code:    "VALIDATION_FAILED",
		Message: message,
		State:   state,
		Next:    coalesceNextActions(next),
	}
}

func findingsAsAny(findings []infrastructure.ValidationFinding) []any {
	values := make([]any, 0, len(findings))
	for _, finding := range findings {
		values = append(values, finding)
	}
	return values
}

func buildSpecCreateNext(target, docPath, action string, formatTemplate *string) []any {
	steps := []any{}

	if formatTemplate == nil || *formatTemplate == "" {
		steps = append(steps, map[string]any{
			"priority": 1,
			"action":   "create_format_template",
			"kind":     "ask_user_then_edit",
			"instructions": "Before creating the format template, interview the user about the system being specified. " +
				"The answers shape the template structure and the spec's quality bar.",
			"context": map[string]any{
				"interview": []string{
					"What kind of system is this spec governing? (e.g., CLI tool with JSON responses, REST API, UI component library, event-driven pipeline, infrastructure service)",
					"What are the distinct behavioral surfaces a caller or user observes? (e.g., commands, endpoints, screens, event handlers, lifecycle hooks)",
					"How do callers interact with each surface — what do they send and what do they receive? (e.g., JSON envelopes, file mutations, rendered output, events)",
					"Does the system have meaningful state transitions or lifecycle stages? (e.g., draft→active→closed, request→processing→complete)",
					"What should this spec enable — could someone reimplement the system from the spec alone, or is it primarily for tracking requirements?",
				},
				"after_interview": "Based on the answers, create a markdown format template adapted to this system. " +
					"Recommended five-layer structure per behavioral surface: " +
					"(1) Prose — what the surface does, why it exists, when it's used. Write from the external surface, not the implementation. " +
					"(2) Data Model — schema fields, types, relationships the surface reads or writes. " +
					"(3) Contracts — exact response shapes for success and each error code, with concrete field names and realistic values. " +
					"(4) Invariants — declarative rules that must always hold, stated as constraints not descriptions. " +
					"(5) Gherkin tracking — '## Requirement:' heading (h2) with ```gherkin requirement``` and ```gherkin scenario``` blocks for specctl governance. " +
					"The spec is the source of truth for the system's behavior. specctl tracks requirements via Gherkin for navigation and governance, " +
					"but the spec itself must be complete enough that an agent could reimplement the system from it alone. " +
					"Include HTML guidance comments in each section explaining what to write.",
				"config_schema": map[string]any{
					"file":     ".specs/<charter>.yaml",
					"location": "formats.<key>",
					"fields": map[string]string{
						"template":        "Path to the format template file, relative to the repo root (e.g., internal/SPEC-FORMAT.md)",
						"recommended_for": "Glob pattern for auto-selecting this format during spec create when the design doc path matches (e.g., 'internal/**', 'runtime/src/**')",
						"description":     "Human-readable description of what this format covers",
					},
					"frontmatter": "After configuring, set 'format: <key>' in the design doc YAML frontmatter to bind the spec to this format",
					"example":     "formats:\n  behavioral-spec:\n    template: internal/SPEC-FORMAT.md\n    recommended_for: \"internal/**\"\n    description: Five-layer behavioral specification",
				},
			},
		})
	}

	return steps
}

func buildDeltaAddNext(target string, tracking *domain.TrackingFile, delta domain.Delta, formatTemplate *string) []any {
	action := "add_requirement"
	argv := []string{"specctl", "req", "add", target, "--delta", delta.ID}
	why := "Record a net-new requirement for this delta."
	required := []map[string]any{{"name": "gherkin_requirement", "description": "Requirement-level Gherkin block from SPEC.md"}}
	switch delta.Intent {
	case domain.DeltaIntentChange:
		action = "replace_requirement"
		why = "Replace the active requirement that this change supersedes."
		argv = []string{"specctl", "req", "replace", target, firstAffectedRequirement(delta), "--delta", delta.ID}
	case domain.DeltaIntentRemove:
		action = "withdraw_requirement"
		why = "Withdraw the active requirement that this delta intentionally removes."
		argv = []string{"specctl", "req", "withdraw", target, firstAffectedRequirement(delta), "--delta", delta.ID}
		required = []map[string]any{}
	case domain.DeltaIntentRepair:
		action = "stale_requirement"
		why = "Mark the affected requirement stale before re-verifying it."
		argv = []string{"specctl", "req", "stale", target, firstAffectedRequirement(delta), "--delta", delta.ID}
		required = []map[string]any{}
	}

	steps := []any{}

	// For add/change intents, guide the agent to write the spec section first
	if delta.Intent == "" || delta.Intent == domain.DeltaIntentAdd || delta.Intent == domain.DeltaIntentChange {
		steps = append(steps, buildWriteSpecSectionStep(tracking, delta, formatTemplate))
	}

	steps = append(steps, map[string]any{
		"priority":     len(steps) + 1,
		"action":       action,
		"kind":         "run_command",
		"instructions": why,
		"template": map[string]any{
			"argv":            argv,
			"stdin_format":    ternaryString(len(required) > 0, "gherkin", ""),
			"stdin_template":  ternaryString(len(required) > 0, requirementBlockSeedGherkin(tracking), ""),
			"required_fields": required,
		},
	})

	return steps
}

func buildWriteSpecSectionStep(tracking *domain.TrackingFile, delta domain.Delta, formatTemplate *string) map[string]any {
	context := map[string]any{
		"design_doc": tracking.Documents.Primary,
		"delta_id":   delta.ID,
		"intent":     string(delta.Intent),
		"area":       delta.Area,
		"current":    delta.Current,
		"target":     delta.Target,
	}

	specWritingCore := "The design doc is the source of truth for the system's behavior. " +
		"specctl tracks requirements via Gherkin blocks for navigation and governance, " +
		"but the SPEC itself must be complete enough that an agent could reimplement the system from it alone. " +
		"Ask the user what observable behavior this delta introduces and why it matters. " +
		"Write a specification section with five layers: " +
		"(1) Prose — explain WHAT this behavioral surface does, WHY it exists, and WHEN it's used. " +
		"Write from the external surface — what a caller or user observes; " +
		"(2) Data Model — define schema fields this surface reads or writes, with types and relationships; " +
		"(3) Contracts — show exact JSON response shapes for success and each error code, " +
		"with concrete field names and realistic values. Do not leave response formats ambiguous; " +
		"(4) Invariants — list rules that must always hold as declarative statements; " +
		"(5) Gherkin tracking — a '## Requirement: <title>' heading (h2) with a ```gherkin requirement``` block " +
		"and ```gherkin scenario``` blocks. The Gherkin is for specctl tracking and quick navigation, " +
		"not the full specification. " +
		"Adapt the depth to the system being specified — a CLI needs command contracts with JSON shapes, " +
		"an API needs endpoint schemas, a UI needs component behavior and state transitions. " +
		"Optionally include a Journey subsection showing the surface's role in multi-step workflows."

	if formatTemplate != nil && *formatTemplate != "" {
		context["format_template"] = *formatTemplate
		context["guidance"] = "A format template is configured at " + *formatTemplate + ". " +
			"Read it and follow its structure. " + specWritingCore
	} else {
		context["format_template"] = nil
		context["guidance"] = "No format template is configured for this spec. " +
			"Consider creating one: add a file with the section structure (Prose, Data Model, Contracts, Invariants, Gherkin) " +
			"and guidance comments explaining what to write in each layer. " +
			"Then configure it in .specs/specctl.yaml under formats with template (file path), " +
			"recommended_for (glob pattern), and description. " +
			"Set format: <key> in the design doc frontmatter. " +
			"A format template ensures every spec section follows the same structure and quality bar. " +
			specWritingCore
	}

	return map[string]any{
		"priority":     1,
		"action":       "write_spec_section",
		"kind":         "edit_file",
		"path":         tracking.Documents.Primary,
		"instructions": "Write the specification section in the design doc before formalizing the requirement. Ask the user about the behavior first.",
		"context":      context,
	}
}

func buildDeltaTransitionNext(target string, delta domain.Delta) []any {
	if delta.Status != domain.DeltaStatusOpen {
		return []any{}
	}
	return []any{
		map[string]any{
			"priority":     1,
			"action":       "start_delta",
			"kind":         "run_command",
			"instructions": "Resume active work on the reopened live delta.",
			"template": map[string]any{
				"argv":            []string{"specctl", "delta", "start", target, delta.ID},
				"required_fields": []map[string]any{},
			},
		},
	}
}

func buildDeltaCloseNext(target string, delta domain.Delta, state SpecProjection) []any {
	if delta.Intent == domain.DeltaIntentRepair {
		if state.Status != domain.SpecStatusVerified {
			return buildTrackedDriftContinuationNext(state, target)
		}
		return []any{syncNextAction(1, target, "The repair workflow is complete. Re-anchor the checkpoint with sync.", "One-line reason the checkpoint is being re-anchored")}
	}
	if state.Status != domain.SpecStatusVerified {
		return []any{}
	}
	return []any{revisionBumpAction(1, target, "The tracked drift is fully verified. Converge the checkpoint with a revision bump.")}
}

func buildVerifyRequirementNext(target string, tracking *domain.TrackingFile, requirement domain.Requirement) []any {
	instructions := "Write the verification test and then mark the requirement verified."
	suggestedPath := suggestedTestPath(target, tracking, requirement)
	if requirement.IsE2E() {
		instructions = "Write an E2E journey test from the Gherkin scenarios and then verify the requirement."
	}
	if requirement.IsManual() {
		return []any{
			map[string]any{
				"priority":     1,
				"action":       "verify_requirement",
				"kind":         "run_command",
				"instructions": "This requirement is manual and can be verified without persisted test files.",
				"template": map[string]any{
					"argv":            []string{"specctl", "req", "verify", target, requirement.ID},
					"required_fields": []map[string]any{},
				},
			},
		}
	}

	return []any{
		map[string]any{
			"priority":     1,
			"action":       "verify_requirement",
			"kind":         "run_command",
			"instructions": instructions,
			"template": map[string]any{
				"argv":            []string{"specctl", "req", "verify", target, requirement.ID, "--test-file", suggestedPath},
				"required_fields": []map[string]any{},
			},
		},
	}
}

func buildImplementAndVerifyNext(target string, tracking *domain.TrackingFile, requirement domain.Requirement, context infrastructure.RequirementDocContext, why, guidance string) []any {
	next := []any{
		map[string]any{
			"priority":     1,
			"action":       "implement_and_test",
			"kind":         "edit_file",
			"instructions": why,
			"template": map[string]any{
				"action":      "edit_file",
				"description": "Implement the observable behavior and write test files",
			},
			"context": requirementWorkContext(requirement, tracking, context, guidance),
		},
	}
	return append(next, offsetNextPriorities(buildVerifyRequirementNext(target, tracking, requirement), 1)...)
}

func requirementWorkContext(requirement domain.Requirement, tracking *domain.TrackingFile, context infrastructure.RequirementDocContext, guidance string) map[string]any {
	scope := []string{}
	if tracking != nil {
		scope = append(scope, tracking.Scope...)
	}
	return map[string]any{
		"requirement":        requirement.ID,
		"title":              requirement.Title,
		"tags":               append([]string{}, requirement.Tags...),
		"scope":              scope,
		"scenarios":          append([]string{}, context.Scenarios...),
		"verification_level": requirementVerificationLevel(requirement),
		"guidance":           guidance,
	}
}

func requirementVerificationLevel(requirement domain.Requirement) string {
	switch {
	case requirement.IsManual():
		return "manual"
	case requirement.IsE2E():
		return "e2e"
	default:
		return "automated"
	}
}

func requirementNotInSpecFailure(loaded *loadedSpec, workflow, normalized, message, matchStatus string) error {
	focus := map[string]any{
		"requirement_match_issues": []map[string]any{{"status": matchStatus}},
	}
	if workflow != "" {
		focus[workflow] = map[string]any{"reason": "requirement_not_in_spec"}
	}
	nextMode := ""
	next := []any{}
	if option := buildWriteRequirementBlockNext(loaded, workflow, normalized); option != nil {
		nextMode = "choose_one"
		next = []any{option}
	}
	return &Failure{
		Code:     "REQUIREMENT_NOT_IN_SPEC",
		Message:  message,
		State:    withSpecFocus(loaded.state, focus),
		NextMode: nextMode,
		Next:     next,
	}
}

func buildWriteRequirementBlockNext(loaded *loadedSpec, workflow, normalized string) map[string]any {
	if loaded == nil || strings.TrimSpace(workflow) == "" {
		return nil
	}
	retryCommand := "retry the requirement command"
	chooseWhen := "The requirement should be tracked, but SPEC.md does not yet contain its parseable requirement block."
	switch workflow {
	case "req_add":
		retryCommand = "retry req add"
	case "req_replace":
		retryCommand = "retry req replace"
		chooseWhen = "The replacement requirement should be tracked, but SPEC.md does not yet contain its parseable requirement block."
	}
	return map[string]any{
		"priority":          1,
		"action":            "write_requirement_block",
		"choose_when":       chooseWhen,
		"kind":              "edit_file",
		"instructions":      "Write the parseable requirement block into SPEC.md, then retry the command.",
		"path":              loaded.tracking.Documents.Primary,
		"create_if_missing": false,
		"template": map[string]any{
			"action":      "edit_file",
			"description": "Add a requirement heading, a ```gherkin requirement``` block, and scenario blocks to the primary design doc",
		},
		"context": map[string]any{
			"design_doc":          loaded.tracking.Documents.Primary,
			"gherkin_requirement": normalized,
			"guidance":            specWritingGuidance(loaded),
			"retry":               retryCommand,
		},
	}
}

func specWritingGuidance(loaded *loadedSpec) string {
	core := "The design doc is the source of truth — specctl tracks requirements via Gherkin for navigation, " +
		"but the spec must fully define the behavior with five layers: " +
		"(1) Prose — what/why/when from the external surface; " +
		"(2) Data Model — schema fields, types, relationships; " +
		"(3) Contracts — exact JSON response shapes for success and each error code; " +
		"(4) Invariants — rules that must always hold; " +
		"(5) Gherkin — '## Requirement: <title>' heading (h2) with ```gherkin requirement``` and ```gherkin scenario``` blocks for tracking. " +
		"Then retry the command."
	if loaded.state.FormatTemplate != nil && *loaded.state.FormatTemplate != "" {
		return "A format template is configured at " + *loaded.state.FormatTemplate + ". Read it and follow its structure. " + core
	}
	return "No format template is configured. Consider creating one for consistent section structure. " + core
}

func buildRequirementMatchBlockingNext(target, requirementID string) []any {
	return []any{
		map[string]any{
			"priority":     1,
			"action":       "refresh_requirement",
			"kind":         "run_command",
			"instructions": "Choose this when the requirement identity is unchanged and only the exact tracked block needs refresh.",
			"choose_when":  "Same requirement identity; only match text changes.",
			"template": map[string]any{
				"argv":            []string{"specctl", "req", "refresh", target, requirementID},
				"stdin_format":    "gherkin",
				"stdin_template":  "@tag\nFeature: <feature>\n",
				"required_fields": []map[string]any{{"name": "gherkin_requirement", "description": "Requirement-level Gherkin block from SPEC.md"}},
			},
		},
		deltaIntentOption(2, "change", target, "Choose this when the current requirement is no longer the correct statement of truth.", "Existing requirement is no longer the correct statement of truth."),
		deltaIntentOption(3, "remove", target, "Choose this when the observable behavior was intentionally removed.", "Observable behavior is intentionally removed."),
	}
}

func blockingActiveRequirementIDs(state SpecProjection) []string {
	ids := make([]string, 0)
	for _, requirement := range state.Requirements {
		if requirement.Lifecycle != domain.RequirementLifecycleActive {
			continue
		}
		if isBlockingRequirementMatchStatus(requirement.Match.Status) {
			ids = append(ids, requirement.ID)
		}
	}
	return ids
}

func requirementMatchIssuesForState(state SpecProjection, requirementIDs []string, activeOnly bool) []map[string]any {
	filter := make(map[string]struct{}, len(requirementIDs))
	for _, id := range requirementIDs {
		filter[id] = struct{}{}
	}
	issues := make([]map[string]any, 0)
	for _, requirement := range state.Requirements {
		if len(filter) > 0 {
			if _, ok := filter[requirement.ID]; !ok {
				continue
			}
		}
		if activeOnly && requirement.Lifecycle != domain.RequirementLifecycleActive {
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

func buildDeltaCloseBlockingNext(target string, tracking *domain.TrackingFile, requirement domain.Requirement) []any {
	if requirement.IsManual() {
		return []any{
			map[string]any{
				"priority":     1,
				"action":       "verify_requirement",
				"kind":         "run_command",
				"instructions": "Verify the last tracing requirement before closing the delta.",
				"template": map[string]any{
					"argv":            []string{"specctl", "req", "verify", target, requirement.ID},
					"required_fields": []map[string]any{},
				},
			},
		}
	}

	return []any{
		map[string]any{
			"priority":     1,
			"action":       "verify_requirement",
			"kind":         "run_command",
			"instructions": "Verify the last tracing requirement before closing the delta.",
			"template": map[string]any{
				"argv":            []string{"specctl", "req", "verify", target, requirement.ID, "--test-file", suggestedTestPath(target, tracking, requirement)},
				"required_fields": []map[string]any{},
			},
		},
	}
}

func buildDeltaCloseSuggestions(target string, tracking *domain.TrackingFile, traces []string) []any {
	next := make([]any, 0)
	for i, trace := range uniqueOrderedStrings(traces) {
		delta := tracking.DeltaByID(trace)
		if delta == nil {
			continue
		}
		if delta.Status != domain.DeltaStatusOpen && delta.Status != domain.DeltaStatusInProgress {
			continue
		}
		blocking, err := tracking.BlockingRequirementsForDeltaClosure(trace)
		if err != nil || len(blocking) != 0 {
			continue
		}
		next = append(next, map[string]any{
			"priority":     i + 1,
			"action":       "close_delta",
			"kind":         "run_command",
			"instructions": "All tracing requirements are verified. Close the covered delta.",
			"template": map[string]any{
				"argv":            []string{"specctl", "delta", "close", target, trace},
				"required_fields": []map[string]any{},
			},
		})
	}
	return next
}

func projectDelta(delta domain.Delta) DeltaItemProjection {
	intent := delta.Intent
	if intent == "" {
		intent = domain.DeltaIntentAdd
	}
	updates := append([]string{}, delta.Updates...)
	if len(updates) == 0 {
		updates = inferDeltaUpdates(intent)
	}
	return DeltaItemProjection{
		ID:                  delta.ID,
		Area:                delta.Area,
		Intent:              intent,
		Status:              delta.Status,
		OriginCheckpoint:    delta.OriginCheckpoint,
		Current:             delta.Current,
		Target:              delta.Target,
		Notes:               delta.Notes,
		AffectsRequirements: append([]string{}, delta.AffectsRequirements...),
		Updates:             updates,
		WithdrawnReason:     delta.WithdrawnReason,
	}
}

func projectRequirement(requirement domain.Requirement, context infrastructure.RequirementDocContext) RequirementProjection {
	matchStatus := context.MatchStatus
	if matchStatus == "" {
		matchStatus = "missing_in_spec"
	}
	return RequirementProjection{
		ID:           requirement.ID,
		Title:        requirement.Title,
		Tags:         append([]string{}, requirement.Tags...),
		Lifecycle:    requirement.EffectiveLifecycle(),
		Verification: requirement.EffectiveVerification(),
		IntroducedBy: introducedByForRequirement(requirement),
		Supersedes:   nullableProjectionString(requirement.Supersedes),
		SupersededBy: nullableProjectionString(requirement.SupersededBy),
		TestFiles:    append([]string{}, requirement.TestFiles...),
		Gherkin:      requirement.Gherkin,
		Match: RequirementMatchProjection{
			Status:  matchStatus,
			Heading: nullableProjectionString(context.Heading),
		},
		SpecContext: RequirementSpecContextProjection{
			Scenarios: append([]string{}, context.Scenarios...),
		},
	}
}

func scenarioTitlesFromRequirement(gherkin string) []string {
	lines := strings.Split(strings.ReplaceAll(gherkin, "\r\n", "\n"), "\n")
	titles := make([]string, 0)
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, "Scenario:") {
			continue
		}
		titles = append(titles, strings.TrimSpace(strings.TrimPrefix(trimmed, "Scenario:")))
	}
	return titles
}

func invalidSpecInputFailure(state SpecProjection, message string, missingFields ...string) error {
	state.Focus = map[string]any{"input": map[string]any{"missing_fields": append([]string{}, missingFields...)}}
	return &Failure{
		Code:    "INVALID_INPUT",
		Message: message,
		State:   state,
		Next:    []any{},
	}
}

func revBumpInvalidInputFailure(state SpecProjection, message, reason string) error {
	focus := clonedFocusMap(state.Focus)
	focus["rev_bump"] = map[string]any{"reason": reason}
	state.Focus = focus
	return &Failure{
		Code:    "INVALID_INPUT",
		Message: message,
		State:   state,
		Next:    []any{},
	}
}

func syncInvalidInputFailure(state SpecProjection, message, reason string) error {
	focus := clonedFocusMap(state.Focus)
	focus["sync"] = map[string]any{"reason": reason}
	state.Focus = focus
	return &Failure{
		Code:    "INVALID_INPUT",
		Message: message,
		State:   state,
		Next:    []any{},
	}
}

func revBumpCheckpointUnavailableFailure(loaded *loadedSpec, checkpoint, message string) error {
	return &Failure{
		Code:    "CHECKPOINT_UNAVAILABLE",
		Message: message,
		State: withSpecFocus(loaded.state, map[string]any{
			"rev_bump": map[string]any{
				"checkpoint": checkpoint,
			},
		}),
		Next: []any{},
	}
}

func clonedFocusMap(raw any) map[string]any {
	focus := map[string]any{}
	if existing, ok := raw.(map[string]any); ok {
		for key, value := range existing {
			focus[key] = value
		}
	}
	return focus
}

func syncCheckpointUnavailableFailure(loaded *loadedSpec, checkpoint, message string) error {
	return &Failure{
		Code:    "CHECKPOINT_UNAVAILABLE",
		Message: message,
		State: withSpecFocus(loaded.state, map[string]any{
			"sync": map[string]any{
				"checkpoint": checkpoint,
			},
		}),
		Next: []any{},
	}
}

func firstBlockingRequirement(requirements []domain.Requirement) domain.Requirement {
	for _, requirement := range requirements {
		if requirement.EffectiveVerification() != domain.RequirementVerificationVerified {
			return requirement
		}
	}
	return requirements[0]
}

func deltaFailureFocus(delta domain.Delta) map[string]any {
	return map[string]any{
		"id":     delta.ID,
		"status": delta.Status,
	}
}

func specCreateFailure(code, message, target, charterName, slug string, focus map[string]any) error {
	payload := specCreateFailureState(target, charterName, slug)
	if len(focus) > 0 {
		payload["focus"] = focus
	}
	return &Failure{
		Code:    code,
		Message: message,
		State:   payload,
		Next:    []any{},
	}
}

func specCreateFailureState(target, charterName, slug string) map[string]any {
	return map[string]any{
		"target":         target,
		"tracking_file":  infrastructure.RelativeTrackingPath("", charterName, slug),
		"charter_exists": true,
	}
}

func validationFailureFocus(findings []infrastructure.ValidationFinding) map[string]any {
	return map[string]any{
		"validation": map[string]any{
			"findings": findingsAsAny(findings),
		},
	}
}

func (s *Service) specCreateTrackingCandidate(request SpecCreateRequest, charterName, slug, docPath string, scope []string) *domain.TrackingFile {
	now := s.todayUTC()
	return &domain.TrackingFile{
		Slug:           slug,
		Charter:        charterName,
		Title:          strings.TrimSpace(request.Title),
		Status:         domain.SpecStatusDraft,
		Rev:            1,
		Created:        now,
		Updated:        now,
		LastVerifiedAt: now,
		Tags:           uniqueOrderedStrings(normalizeTagList(request.Tags)),
		Documents:      domain.Documents{Primary: docPath},
		Scope:          append([]string{}, scope...),
		Deltas:         []domain.Delta{},
		Requirements:   []domain.Requirement{},
		Changelog:      []domain.ChangelogEntry{},
		FilePath:       s.pathAdapter().TrackingPath(charterName, slug),
	}
}

func (s *Service) specCreateValidationFailure(message string, charter *domain.Charter, tracking *domain.TrackingFile, config *infrastructure.ProjectConfig, findings []infrastructure.ValidationFinding, next []any) error {
	state, err := newSpecProjection(
		s.repoRoot,
		tracking,
		charter,
		findings,
		s.repoReadAdapter().ResolveSpecProjectionInputs(tracking, config),
	)
	if err != nil {
		return err
	}
	state = withSpecFocus(state, validationFailureFocus(findings))
	return &Failure{
		Code:    "VALIDATION_FAILED",
		Message: message,
		State:   state,
		Next:    coalesceNextActions(next),
	}
}

func (s *Service) charterCreateValidationFailure(charter *domain.Charter, message string, findings []infrastructure.ValidationFinding) error {
	state, err := newCharterProjection(
		s.repoRoot,
		charter,
		map[string]*domain.TrackingFile{},
		findings,
		map[string][]infrastructure.ValidationFinding{},
	)
	if err != nil {
		return err
	}
	state.Focus = validationFailureFocus(findings)
	return &Failure{
		Code:    "VALIDATION_FAILED",
		Message: message,
		State:   state,
		Next:    []any{},
	}
}

func cloneCharterForProjection(charter *domain.Charter) *domain.Charter {
	if charter == nil {
		return nil
	}
	cloned := &domain.Charter{
		Name:        charter.Name,
		Title:       charter.Title,
		Description: charter.Description,
		Groups:      append([]domain.CharterGroup{}, charter.Groups...),
		Specs:       append([]domain.CharterSpecEntry{}, charter.Specs...),
		DirPath:     charter.DirPath,
	}
	return cloned
}

func upsertCharterSpecEntryForProjection(charter *domain.Charter, entry domain.CharterSpecEntry) {
	if charter == nil {
		return
	}
	for i := range charter.Specs {
		if charter.Specs[i].Slug == entry.Slug {
			charter.Specs[i] = entry
			return
		}
	}
	charter.Specs = append(charter.Specs, entry)
}

func requirementBlockSeedGherkin(tracking *domain.TrackingFile) string {
	return requirementSeedTag("", tracking) + "\nFeature: <feature>\n"
}

func requirementSeedTag(target string, tracking *domain.TrackingFile) string {
	charter, _, _ := strings.Cut(target, ":")
	if tracking != nil && strings.TrimSpace(tracking.Charter) != "" {
		charter = tracking.Charter
	}
	charter = strings.TrimSpace(charter)
	if charter == "" {
		return "@tag"
	}
	return "@" + charter
}

func firstAffectedRequirement(delta domain.Delta) string {
	if len(delta.AffectsRequirements) == 0 {
		return "<requirement-id>"
	}
	return delta.AffectsRequirements[0]
}

func ternaryString(condition bool, whenTrue, whenFalse string) string {
	if condition {
		return whenTrue
	}
	return whenFalse
}

func suggestedTestPath(target string, tracking *domain.TrackingFile, requirement domain.Requirement) string {
	charter, _, _ := strings.Cut(target, ":")
	dir := suggestedRequirementTestDir(charter, tracking, requirement)
	return path.Join(dir, suggestedRequirementTestFile(dir, tracking, requirement))
}

func suggestedRequirementTestDir(charter string, tracking *domain.TrackingFile, requirement domain.Requirement) string {
	if requirement.IsE2E() {
		return path.Join(charter, "tests", "e2e", "journeys")
	}
	return conventionalUnitTestDir(charter, tracking)
}

func conventionalUnitTestDir(charter string, tracking *domain.TrackingFile) string {
	if tracking != nil {
		for _, scope := range tracking.Scope {
			normalized := strings.Trim(strings.TrimSpace(scope), "/")
			if normalized == "" {
				continue
			}
			prefix := strings.TrimSpace(charter) + "/src/"
			if strings.HasPrefix(normalized+"/", prefix) {
				remainder := strings.TrimPrefix(normalized+"/", prefix)
				if area, _, ok := strings.Cut(remainder, "/"); ok && strings.TrimSpace(area) != "" {
					if area == "domain" {
						return path.Join(charter, "tests", "domain")
					}
					return path.Join(charter, "tests", "unit")
				}
			}
		}
	}
	return path.Join(charter, "tests", "domain")
}

func suggestedRequirementTestFile(dir string, tracking *domain.TrackingFile, requirement domain.Requirement) string {
	slug := slugifyRequirementTitle(requirement.Title, "_")
	if inherited, ok := inferredRequirementTestFile(dir, tracking, requirement, slug); ok {
		return inherited
	}
	return "test_" + slug + ".py"
}

func inferredRequirementTestFile(dir string, tracking *domain.TrackingFile, requirement domain.Requirement, slug string) (string, bool) {
	if tracking == nil {
		return "", false
	}

	var (
		prefix  string
		suffix  string
		matched bool
	)
	for _, existing := range tracking.Requirements {
		if existing.IsE2E() != requirement.IsE2E() {
			continue
		}
		existingSlug := slugifyRequirementTitle(existing.Title, "_")
		if existingSlug == "" {
			continue
		}
		for _, testFile := range existing.TestFiles {
			if path.Dir(strings.TrimSpace(testFile)) != dir {
				continue
			}
			base := path.Base(testFile)
			index := strings.Index(base, existingSlug)
			if index < 0 {
				continue
			}
			candidatePrefix := base[:index]
			candidateSuffix := base[index+len(existingSlug):]
			if !matched {
				prefix = candidatePrefix
				suffix = candidateSuffix
				matched = true
				continue
			}
			if prefix != candidatePrefix || suffix != candidateSuffix {
				return "", false
			}
		}
	}
	if !matched {
		return "", false
	}
	return prefix + slug + suffix, true
}

func slugifyRequirementTitle(title, separator string) string {
	trimmed := strings.TrimSpace(strings.ToLower(title))
	if trimmed == "" {
		return "requirement"
	}

	var builder strings.Builder
	lastWasSeparator := false
	for _, r := range trimmed {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			builder.WriteRune(r)
			lastWasSeparator = false
			continue
		}
		if builder.Len() == 0 || lastWasSeparator {
			continue
		}
		builder.WriteString(separator)
		lastWasSeparator = true
	}

	result := strings.Trim(builder.String(), separator)
	if result == "" {
		return "requirement"
	}
	return result
}

type semanticDiff struct {
	StatusChanged  bool
	TagsChanged    bool
	DocumentMoved  bool
	ScopeChanged   bool
	OpenedDeltas   []string
	ClosedDeltas   []string
	DeferredDeltas []string
	ResumedDeltas  []string
	AddedReqs      []string
	VerifiedReqs   []string
	DesignDocSame  bool
}

func (d semanticDiff) hasSemanticChanges() bool {
	return d.StatusChanged ||
		d.TagsChanged ||
		d.DocumentMoved ||
		d.ScopeChanged ||
		len(d.OpenedDeltas) != 0 ||
		len(d.ClosedDeltas) != 0 ||
		len(d.DeferredDeltas) != 0 ||
		len(d.ResumedDeltas) != 0 ||
		len(d.AddedReqs) != 0 ||
		len(d.VerifiedReqs) != 0 ||
		!d.DesignDocSame
}

func buildSemanticDiff(baseline, current *domain.TrackingFile, normalizedBaselineDoc, normalizedCurrentDoc []byte) semanticDiff {
	normalizedBaseline := normalizeTrackingForSemanticDiff(baseline)
	normalizedCurrent := normalizeTrackingForSemanticDiff(current)
	tagDiff := diffStringSet(normalizedBaseline.Tags, normalizedCurrent.Tags)
	scopeDiff := diffStringSet(normalizedBaseline.Scope, normalizedCurrent.Scope)
	deltaDiff := diffDeltas(normalizedBaseline.Deltas, normalizedCurrent.Deltas)
	requirementDiff := diffRequirements(normalizedBaseline.Requirements, normalizedCurrent.Requirements)
	return semanticDiff{
		StatusChanged:  normalizedBaseline.ComputedStatus() != normalizedCurrent.ComputedStatus(),
		TagsChanged:    len(tagDiff.Added) != 0 || len(tagDiff.Removed) != 0,
		DocumentMoved:  normalizedBaseline.Documents.Primary != normalizedCurrent.Documents.Primary,
		ScopeChanged:   len(scopeDiff.Added) != 0 || len(scopeDiff.Removed) != 0,
		OpenedDeltas:   summaryIDs(deltaDiff.Opened),
		ClosedDeltas:   summaryIDs(deltaDiff.Closed),
		DeferredDeltas: summaryIDs(deltaDiff.Deferred),
		ResumedDeltas:  summaryIDs(deltaDiff.Resumed),
		AddedReqs:      requirementSummaryIDs(requirementDiff.Added),
		VerifiedReqs:   requirementSummaryIDs(requirementDiff.Verified),
		DesignDocSame:  string(normalizedBaselineDoc) == string(normalizedCurrentDoc),
	}
}

func normalizeTrackingForSemanticDiff(source *domain.TrackingFile) *domain.TrackingFile {
	if source == nil {
		return &domain.TrackingFile{}
	}
	normalized := cloneTracking(source)
	normalized.Status = ""
	normalized.Rev = 0
	normalized.Updated = ""
	normalized.LastVerifiedAt = ""
	normalized.Checkpoint = ""
	normalized.Changelog = nil
	normalized.SyncComputedStatus()
	return normalized
}

func summaryIDs(items []DiffDeltaSummary) []string {
	ids := make([]string, 0, len(items))
	for _, item := range items {
		ids = append(ids, item.ID)
	}
	return ids
}

func requirementSummaryIDs(items []DiffRequirementSummary) []string {
	ids := make([]string, 0, len(items))
	for _, item := range items {
		ids = append(ids, item.ID)
	}
	return ids
}

func closedDeltasNotInChangelog(tracking *domain.TrackingFile) []string {
	recorded := make(map[string]struct{})
	for _, entry := range tracking.Changelog {
		for _, id := range entry.DeltasClosed {
			recorded[id] = struct{}{}
		}
	}
	unbumped := make([]string, 0)
	for _, delta := range tracking.Deltas {
		if delta.Status == domain.DeltaStatusClosed {
			if _, ok := recorded[delta.ID]; !ok {
				unbumped = append(unbumped, delta.ID)
			}
		}
	}
	return unbumped
}

func withdrawnDeltasNotInChangelog(tracking *domain.TrackingFile) []string {
	recorded := make(map[string]struct{})
	for _, entry := range tracking.Changelog {
		for _, id := range entry.DeltasWithdrawn {
			recorded[id] = struct{}{}
		}
	}
	unbumped := make([]string, 0)
	for _, delta := range tracking.Deltas {
		if delta.Status == domain.DeltaStatusWithdrawn {
			if _, ok := recorded[delta.ID]; !ok {
				unbumped = append(unbumped, delta.ID)
			}
		}
	}
	return unbumped
}

func openedDeltasNotInChangelog(tracking *domain.TrackingFile) []string {
	recorded := make(map[string]struct{})
	for _, entry := range tracking.Changelog {
		for _, id := range entry.DeltasOpened {
			recorded[id] = struct{}{}
		}
	}
	result := make([]string, 0)
	for _, delta := range tracking.Deltas {
		if _, ok := recorded[delta.ID]; !ok {
			result = append(result, delta.ID)
		}
	}
	return result
}

func requirementsNotInChangelog(tracking *domain.TrackingFile) []string {
	recorded := make(map[string]struct{})
	for _, entry := range tracking.Changelog {
		for _, id := range entry.ReqsAdded {
			recorded[id] = struct{}{}
		}
	}
	result := make([]string, 0)
	for _, req := range tracking.Requirements {
		if _, ok := recorded[req.ID]; !ok {
			result = append(result, req.ID)
		}
	}
	return result
}

func verifiedRequirementsNotInChangelog(tracking *domain.TrackingFile) []string {
	recorded := make(map[string]struct{})
	for _, entry := range tracking.Changelog {
		for _, id := range entry.ReqsVerified {
			recorded[id] = struct{}{}
		}
	}
	result := make([]string, 0)
	for _, req := range tracking.Requirements {
		if req.EffectiveVerification() != domain.RequirementVerificationVerified {
			continue
		}
		if _, ok := recorded[req.ID]; !ok {
			result = append(result, req.ID)
		}
	}
	return result
}

func buildChangelogEntry(baseline, current *domain.TrackingFile, rev int, date, summary string) domain.ChangelogEntry {
	return domain.ChangelogEntry{
		Rev:             rev,
		Date:            date,
		DeltasOpened:    deltaIDsNotInBaseline(baseline, current),
		DeltasClosed:    deltaIDsClosedSinceBaseline(baseline, current),
		DeltasWithdrawn: deltaIDsWithdrawnSinceBaseline(baseline, current),
		ReqsAdded:       requirementIDsNotInBaseline(baseline, current),
		ReqsVerified:    requirementIDsVerifiedSinceBaseline(baseline, current),
		Summary:         summary,
	}
}

func deltaIDsNotInBaseline(baseline, current *domain.TrackingFile) []string {
	if baseline == nil {
		baseline = &domain.TrackingFile{}
	}
	baselineIDs := make(map[string]struct{}, len(baseline.Deltas))
	for _, delta := range baseline.Deltas {
		baselineIDs[delta.ID] = struct{}{}
	}
	ids := make([]string, 0)
	for _, delta := range current.Deltas {
		if _, exists := baselineIDs[delta.ID]; !exists {
			ids = append(ids, delta.ID)
		}
	}
	return ids
}

func deltaIDsClosedSinceBaseline(baseline, current *domain.TrackingFile) []string {
	if baseline == nil {
		baseline = &domain.TrackingFile{}
	}
	baselineStatus := make(map[string]domain.DeltaStatus, len(baseline.Deltas))
	for _, delta := range baseline.Deltas {
		baselineStatus[delta.ID] = delta.Status
	}
	ids := make([]string, 0)
	for _, delta := range current.Deltas {
		if delta.Status != domain.DeltaStatusClosed {
			continue
		}
		if baselineStatus[delta.ID] != domain.DeltaStatusClosed {
			ids = append(ids, delta.ID)
		}
	}
	return ids
}

func deltaIDsWithdrawnSinceBaseline(baseline, current *domain.TrackingFile) []string {
	if baseline == nil {
		baseline = &domain.TrackingFile{}
	}
	baselineStatus := make(map[string]domain.DeltaStatus, len(baseline.Deltas))
	for _, delta := range baseline.Deltas {
		baselineStatus[delta.ID] = delta.Status
	}
	ids := make([]string, 0)
	for _, delta := range current.Deltas {
		if delta.Status != domain.DeltaStatusWithdrawn {
			continue
		}
		if baselineStatus[delta.ID] != domain.DeltaStatusWithdrawn {
			ids = append(ids, delta.ID)
		}
	}
	return ids
}

func requirementIDsNotInBaseline(baseline, current *domain.TrackingFile) []string {
	if baseline == nil {
		baseline = &domain.TrackingFile{}
	}
	baselineIDs := make(map[string]struct{}, len(baseline.Requirements))
	for _, requirement := range baseline.Requirements {
		baselineIDs[requirement.ID] = struct{}{}
	}
	ids := make([]string, 0)
	for _, requirement := range current.Requirements {
		if _, exists := baselineIDs[requirement.ID]; !exists {
			ids = append(ids, requirement.ID)
		}
	}
	return ids
}

func requirementIDsVerifiedSinceBaseline(baseline, current *domain.TrackingFile) []string {
	if baseline == nil {
		baseline = &domain.TrackingFile{}
	}
	baselineVerified := make(map[string]bool, len(baseline.Requirements))
	for _, requirement := range baseline.Requirements {
		baselineVerified[requirement.ID] = requirement.EffectiveVerification() == domain.RequirementVerificationVerified
	}
	ids := make([]string, 0)
	for _, requirement := range current.Requirements {
		if requirement.EffectiveVerification() == domain.RequirementVerificationVerified && !baselineVerified[requirement.ID] {
			ids = append(ids, requirement.ID)
		}
	}
	return ids
}
