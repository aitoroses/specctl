package application

import (
	"sort"

	"github.com/aitoroses/specctl/internal/domain"
	"github.com/aitoroses/specctl/internal/infrastructure"
)

type ValidationProjection struct {
	Valid    bool  `json:"valid"`
	Findings []any `json:"findings"`
}

type RegistryProjection struct {
	Specs    []RegistrySpecSummary    `json:"specs"`
	Charters []RegistryCharterSummary `json:"charters"`
	Config   ValidationContainer      `json:"config"`
	Audit    ValidationProjection     `json:"audit"`
	Focus    any                      `json:"focus,omitempty"`
}

type RegistrySpecSummary struct {
	Slug         string                `json:"slug"`
	Charter      string                `json:"charter"`
	Title        string                `json:"title"`
	Status       domain.SpecStatus     `json:"status"`
	TrackingFile string                `json:"tracking_file"`
	Documents    domain.Documents      `json:"documents"`
	Scope        []string              `json:"scope"`
	Deltas       DeltaCountsProjection `json:"deltas"`
	Validation   ValidationProjection  `json:"validation"`
}

type RegistryCharterSummary struct {
	Name       string               `json:"name"`
	Title      string               `json:"title"`
	SpecCount  int                  `json:"spec_count"`
	Validation ValidationProjection `json:"validation"`
}

type ValidationContainer struct {
	Validation ValidationProjection                 `json:"validation"`
	Warnings   []infrastructure.SourcePrefixWarning `json:"warnings,omitempty"`
}

type SpecProjection struct {
	Slug                             string                         `json:"slug"`
	Charter                          string                         `json:"charter"`
	Title                            string                         `json:"title"`
	Status                           domain.SpecStatus              `json:"status"`
	Rev                              int                            `json:"rev"`
	Created                          string                         `json:"created"`
	Updated                          string                         `json:"updated"`
	LastVerifiedAt                   string                         `json:"last_verified_at"`
	Checkpoint                       string                         `json:"checkpoint"`
	TrackingFile                     string                         `json:"tracking_file"`
	Tags                             []string                       `json:"tags"`
	Format                           *string                        `json:"format"`
	FormatTemplate                   *string                        `json:"format_template"`
	Documents                        domain.Documents               `json:"documents"`
	CharterMembership                *CharterMembershipProjection   `json:"charter_membership"`
	Scope                            []string                       `json:"scope"`
	Deltas                           DeltaProjection                `json:"deltas"`
	OpenDeltas                       []OpenDeltaProjection          `json:"open_deltas"`
	Requirements                     []RequirementProjection        `json:"requirements"`
	ActionableUnverifiedRequirements []RequirementSummaryProjection `json:"actionable_unverified_requirements"`
	InactiveUnverifiedRequirements   []RequirementSummaryProjection `json:"inactive_unverified_requirements"`
	Changelog                        []domain.ChangelogEntry        `json:"changelog"`
	ScopeDrift                       ScopeDriftProjection           `json:"scope_drift"`
	UncommittedChanges               []string                       `json:"uncommitted_changes"`
	Warnings                         []SpecContextWarningProjection `json:"warnings,omitempty"`
	Validation                       ValidationProjection           `json:"validation"`
	Focus                            any                            `json:"focus,omitempty"`

	// OrphanGherkinBlocks is the raw list of SPEC.md gherkin blocks
	// parsed from the design doc that did not match any tracking
	// requirement by gherkin or title. Carried on the projection so
	// buildSpecContextWarnings can emit SPEC_ORPHAN_GHERKIN_BLOCK
	// advisories; not serialized directly because the user-visible
	// surface is the aggregated warning, not the raw blocks.
	OrphanGherkinBlocks []infrastructure.RequirementContext `json:"-"`
}

type SpecContextWarningProjection struct {
	Kind           string         `json:"kind"`
	Code           string         `json:"code"`
	Severity       string         `json:"severity"`
	Message        string         `json:"message"`
	DeltaIDs       []string       `json:"delta_ids"`
	RequirementIDs []string       `json:"requirement_ids"`
	Details        map[string]any `json:"details"`
}

type CharterMembershipProjection struct {
	Group      string   `json:"group"`
	GroupTitle string   `json:"group_title"`
	GroupOrder int      `json:"group_order"`
	Order      int      `json:"order"`
	DependsOn  []string `json:"depends_on"`
	Notes      string   `json:"notes"`
}

type DeltaCountsProjection struct {
	Open       int `json:"open"`
	InProgress int `json:"in_progress"`
	Closed     int `json:"closed"`
	Deferred   int `json:"deferred"`
	Withdrawn  int `json:"withdrawn"`
}

type DeltaProjection struct {
	Open       int                   `json:"open"`
	InProgress int                   `json:"in_progress"`
	Closed     int                   `json:"closed"`
	Deferred   int                   `json:"deferred"`
	Withdrawn  int                   `json:"withdrawn"`
	Items      []DeltaItemProjection `json:"items"`
}

type DeltaItemProjection struct {
	ID                  string             `json:"id"`
	Area                string             `json:"area"`
	Intent              domain.DeltaIntent `json:"intent"`
	Status              domain.DeltaStatus `json:"status"`
	OriginCheckpoint    string             `json:"origin_checkpoint"`
	Current             string             `json:"current"`
	Target              string             `json:"target"`
	Notes               string             `json:"notes"`
	AffectsRequirements []string           `json:"affects_requirements"`
	Updates             []string           `json:"updates"`
	WithdrawnReason     string             `json:"withdrawn_reason,omitempty"`
}

type OpenDeltaProjection struct {
	ID     string             `json:"id"`
	Area   string             `json:"area"`
	Status domain.DeltaStatus `json:"status"`
}

type RequirementSummaryProjection struct {
	ID           string                         `json:"id"`
	Title        string                         `json:"title"`
	Tags         []string                       `json:"tags"`
	Lifecycle    domain.RequirementLifecycle    `json:"lifecycle"`
	Verification domain.RequirementVerification `json:"verification"`
	IntroducedBy string                         `json:"introduced_by"`
	TestFiles    []string                       `json:"test_files"`
}

type RequirementProjection struct {
	ID           string                           `json:"id"`
	Title        string                           `json:"title"`
	Tags         []string                         `json:"tags"`
	Lifecycle    domain.RequirementLifecycle      `json:"lifecycle"`
	Verification domain.RequirementVerification   `json:"verification"`
	IntroducedBy string                           `json:"introduced_by"`
	Supersedes   *string                          `json:"supersedes"`
	SupersededBy *string                          `json:"superseded_by"`
	TestFiles    []string                         `json:"test_files"`
	Gherkin      string                           `json:"gherkin"`
	Match        RequirementMatchProjection       `json:"match"`
	SpecContext  RequirementSpecContextProjection `json:"spec_context"`
}

type RequirementMatchProjection struct {
	Status  string  `json:"status"`
	Heading *string `json:"heading"`
}

type RequirementSpecContextProjection struct {
	Scenarios []string `json:"scenarios"`
}

type ScopeDriftProjection struct {
	Status                      string   `json:"status"`
	Checkpoint                  string   `json:"checkpoint"`
	DriftSource                 *string  `json:"drift_source"`
	LastVerifiedAt              string   `json:"last_verified_at"`
	TrackedBy                   []string `json:"tracked_by"`
	FilesChangedSinceCheckpoint []string `json:"files_changed_since_checkpoint"`
}

type CharterProjection struct {
	Name         string                    `json:"name"`
	Title        string                    `json:"title"`
	Description  string                    `json:"description"`
	TrackingFile string                    `json:"tracking_file"`
	Groups       []domain.CharterGroup     `json:"groups"`
	OrderedSpecs []OrderedCharterSpecEntry `json:"ordered_specs"`
	Validation   ValidationProjection      `json:"validation"`
	Focus        any                       `json:"focus,omitempty"`
}

type OrderedCharterSpecEntry struct {
	Slug         string               `json:"slug"`
	Group        domain.CharterGroup  `json:"group"`
	Order        int                  `json:"order"`
	DependsOn    []string             `json:"depends_on"`
	Notes        string               `json:"notes"`
	Status       domain.SpecStatus    `json:"status"`
	TrackingFile string               `json:"tracking_file"`
	Validation   ValidationProjection `json:"validation"`
}

type FileContextProjection struct {
	File          string                `json:"file"`
	Resolution    string                `json:"resolution"`
	MatchSource   *string               `json:"match_source"`
	GoverningSpec *FileGoverningSpec    `json:"governing_spec"`
	Matches       []FileMatchProjection `json:"matches"`
	Validation    ValidationProjection  `json:"validation"`
	Focus         any                   `json:"focus,omitempty"`
}

type FileGoverningSpec struct {
	Slug         string           `json:"slug"`
	Charter      string           `json:"charter"`
	TrackingFile string           `json:"tracking_file"`
	Documents    domain.Documents `json:"documents"`
}

type FileMatchProjection struct {
	Slug        string `json:"slug"`
	Charter     string `json:"charter"`
	MatchSource string `json:"match_source"`
	ScopePrefix string `json:"scope_prefix,omitempty"`
}

type ConfigProjection struct {
	SemanticTags   []string                               `json:"semantic_tags"`
	GherkinTags    []string                               `json:"gherkin_tags"`
	SourcePrefixes []string                               `json:"source_prefixes"`
	Formats        map[string]infrastructure.FormatConfig `json:"formats"`
	Validation     ValidationProjection                   `json:"validation"`
	Warnings       []infrastructure.SourcePrefixWarning   `json:"warnings,omitempty"`
	Focus          any                                    `json:"focus,omitempty"`
}

type HookProjection struct {
	InputFiles      []string             `json:"input_files"`
	ConsideredFiles []string             `json:"considered_files"`
	IgnoredFiles    []string             `json:"ignored_files"`
	UnmatchedFiles  []string             `json:"unmatched_files"`
	AffectedSpecs   []HookAffectedSpec   `json:"affected_specs"`
	Validation      ValidationProjection `json:"validation"`
}

type HookAffectedSpec struct {
	Slug               string            `json:"slug"`
	Charter            string            `json:"charter"`
	Status             domain.SpecStatus `json:"status"`
	TrackingFile       string            `json:"tracking_file"`
	TrackingFileStaged bool              `json:"tracking_file_staged"`
	DesignDoc          string            `json:"design_doc"`
	DesignDocStaged    bool              `json:"design_doc_staged"`
	MatchedFiles       []string          `json:"matched_files"`
}

func cloneOptionalString(value *string) *string {
	if value == nil {
		return nil
	}
	cloned := *value
	return &cloned
}

func nullableProjectionString(value string) *string {
	if value == "" {
		return nil
	}
	return cloneOptionalString(&value)
}

func validProjection() ValidationProjection {
	return ValidationProjection{
		Valid:    true,
		Findings: []any{},
	}
}

func projectionFromFindings(findings []infrastructure.ValidationFinding) ValidationProjection {
	if len(findings) == 0 {
		return validProjection()
	}
	return ValidationProjection{
		Valid:    false,
		Findings: findingsToAny(findings),
	}
}

func newConfigProjection(repoRoot string, config *infrastructure.ProjectConfig, findings []infrastructure.ValidationFinding) ConfigProjection {
	projection := ConfigProjection{
		SemanticTags:   domain.SemanticRequirementTags(),
		GherkinTags:    append([]string{}, config.GherkinTags...),
		SourcePrefixes: append([]string{}, config.SourcePrefixes...),
		Formats:        infrastructure.CloneFormatsForProjection(config.Formats),
		Validation:     projectionFromFindings(findings),
	}
	if warnings := infrastructure.ValidateSourcePrefixes(repoRoot, config.SourcePrefixes); len(warnings) > 0 {
		projection.Warnings = warnings
	}
	return projection
}

func newSpecProjection(repoRoot string, tracking *domain.TrackingFile, charter *domain.Charter, findings []infrastructure.ValidationFinding, inputs infrastructure.SpecProjectionInputs) (SpecProjection, error) {
	validationFindings := append([]infrastructure.ValidationFinding{}, findings...)
	validationFindings = append(validationFindings, inputs.ScopeDriftFindings...)

	projection := SpecProjection{
		Slug:           tracking.Slug,
		Charter:        tracking.Charter,
		Title:          tracking.Title,
		Status:         tracking.Status,
		Rev:            tracking.Rev,
		Created:        tracking.Created,
		Updated:        tracking.Updated,
		LastVerifiedAt: tracking.LastVerifiedAt,
		Checkpoint:     tracking.Checkpoint,
		TrackingFile:   infrastructure.RelativeTrackingPath(repoRoot, tracking.Charter, tracking.Slug),
		Tags:           append([]string{}, tracking.Tags...),
		Documents:      tracking.Documents,
		Scope:          append([]string{}, tracking.Scope...),
		Requirements:   buildRequirementProjections(tracking.Requirements, inputs.Requirements),
		Changelog:      append([]domain.ChangelogEntry{}, tracking.Changelog...),
		ScopeDrift: ScopeDriftProjection{
			Status:                      inputs.ScopeDrift.Status,
			Checkpoint:                  tracking.Checkpoint,
			DriftSource:                 nullableProjectionString(inputs.ScopeDrift.DriftSource),
			LastVerifiedAt:              inputs.ScopeDrift.LastVerifiedAt,
			TrackedBy:                   append([]string{}, inputs.ScopeDrift.TrackedBy...),
			FilesChangedSinceCheckpoint: append([]string{}, inputs.ScopeDrift.FilesChangedSinceCheckpoint...),
		},
		UncommittedChanges: append([]string{}, inputs.ScopeDrift.UncommittedChanges...),
		Warnings:            []SpecContextWarningProjection{},
		Validation:          projectionFromFindings(validationFindings),
		OrphanGherkinBlocks: append([]infrastructure.RequirementContext{}, inputs.OrphanGherkinBlocks...),
	}

	if inputs.DesignDoc != nil {
		projection.Format = cloneOptionalString(inputs.DesignDoc.Format)
		projection.FormatTemplate = cloneOptionalString(inputs.DesignDoc.FormatTemplate)
	}

	if charter != nil {
		if membership := charter.SpecBySlug(tracking.Slug); membership != nil {
			group := charter.GroupByKey(membership.Group)
			if group != nil {
				projection.CharterMembership = &CharterMembershipProjection{
					Group:      membership.Group,
					GroupTitle: group.Title,
					GroupOrder: group.Order,
					Order:      membership.Order,
					DependsOn:  append([]string{}, membership.DependsOn...),
					Notes:      membership.Notes,
				}
			}
		}
	}

	projection.Deltas = buildDeltaProjection(tracking.Deltas)
	projection.OpenDeltas = buildOpenDeltas(tracking.Deltas)
	projection.ActionableUnverifiedRequirements = buildActionableUnverifiedRequirements(tracking.Requirements)
	projection.InactiveUnverifiedRequirements = buildInactiveUnverifiedRequirements(tracking.Requirements)
	return projection, nil
}

func newCharterProjection(repoRoot string, charter *domain.Charter, trackingBySlug map[string]*domain.TrackingFile, charterFindings []infrastructure.ValidationFinding, trackingFindings map[string][]infrastructure.ValidationFinding) (CharterProjection, error) {
	ordering := domain.BuildLenientCharterOrdering(charter)
	projection := CharterProjection{
		Name:         charter.Name,
		Title:        charter.Title,
		Description:  charter.Description,
		TrackingFile: infrastructure.RelativeCharterPath(charter.Name),
		Groups:       append([]domain.CharterGroup{}, charter.Groups...),
		OrderedSpecs: make([]OrderedCharterSpecEntry, 0, len(ordering.Specs)),
		Validation:   projectionFromFindings(charterFindings),
	}

	for _, entry := range ordering.Specs {
		group := charter.GroupByKey(entry.Group)
		tracking := trackingBySlug[entry.Slug]
		groupProjection := domain.CharterGroup{Key: entry.Group}
		if group != nil {
			groupProjection = *group
		}
		status := domain.SpecStatus("")
		if tracking != nil {
			status = tracking.Status
		}
		projection.OrderedSpecs = append(projection.OrderedSpecs, OrderedCharterSpecEntry{
			Slug:         entry.Slug,
			Group:        groupProjection,
			Order:        entry.Order,
			DependsOn:    append([]string{}, entry.DependsOn...),
			Notes:        entry.Notes,
			Status:       status,
			TrackingFile: infrastructure.RelativeTrackingPath(repoRoot, charter.Name, entry.Slug),
			Validation:   projectionFromFindings(trackingFindings[entry.Slug]),
		})
	}

	return projection, nil
}

func buildRegistryProjection(repoRoot string, config *infrastructure.ProjectConfig, charters []*domain.Charter, trackingBySpec map[string]*domain.TrackingFile, configFindings []infrastructure.ValidationFinding, auditFindings []infrastructure.ValidationFinding, charterFindings map[string][]infrastructure.ValidationFinding, specFindings map[string][]infrastructure.ValidationFinding) RegistryProjection {
	charterOrder := make(map[string]map[string]int, len(charters))
	charterSummaries := make([]RegistryCharterSummary, 0, len(charters))
	for _, charter := range charters {
		charterOrder[charter.Name] = domain.BuildLenientCharterOrdering(charter).Index
		charterSummaries = append(charterSummaries, RegistryCharterSummary{
			Name:       charter.Name,
			Title:      charter.Title,
			SpecCount:  len(charter.Specs),
			Validation: projectionFromFindings(charterFindings[charter.Name]),
		})
	}
	sort.Slice(charterSummaries, func(i, j int) bool {
		return charterSummaries[i].Name < charterSummaries[j].Name
	})

	specs := make([]RegistrySpecSummary, 0, len(trackingBySpec))
	for key, tracking := range trackingBySpec {
		specs = append(specs, RegistrySpecSummary{
			Slug:         tracking.Slug,
			Charter:      tracking.Charter,
			Title:        tracking.Title,
			Status:       tracking.Status,
			TrackingFile: infrastructure.RelativeTrackingPath(repoRoot, tracking.Charter, tracking.Slug),
			Documents:    tracking.Documents,
			Scope:        append([]string{}, tracking.Scope...),
			Deltas:       buildDeltaCounts(tracking.Deltas),
			Validation:   projectionFromFindings(specFindings[key]),
		})
		_ = key
	}

	sort.Slice(specs, func(i, j int) bool {
		left := specs[i]
		right := specs[j]
		if left.Charter != right.Charter {
			return left.Charter < right.Charter
		}
		leftOrder := charterOrder[left.Charter][left.Slug]
		rightOrder := charterOrder[right.Charter][right.Slug]
		if leftOrder != rightOrder {
			return leftOrder < rightOrder
		}
		return left.Slug < right.Slug
	})

	configContainer := ValidationContainer{
		Validation: projectionFromFindings(configFindings),
	}
	if config != nil {
		if warnings := infrastructure.ValidateSourcePrefixes(repoRoot, config.SourcePrefixes); len(warnings) > 0 {
			configContainer.Warnings = warnings
		}
	}

	return RegistryProjection{
		Specs:    specs,
		Charters: charterSummaries,
		Config:   configContainer,
		Audit:    projectionFromFindings(auditFindings),
	}
}

func stringPointer(value string) *string {
	return &value
}

func buildDeltaProjection(deltas []domain.Delta) DeltaProjection {
	counts := buildDeltaCounts(deltas)
	return DeltaProjection{
		Open:       counts.Open,
		InProgress: counts.InProgress,
		Closed:     counts.Closed,
		Deferred:   counts.Deferred,
		Withdrawn:  counts.Withdrawn,
		Items:      buildDeltaItemProjections(deltas),
	}
}

func buildDeltaCounts(deltas []domain.Delta) DeltaCountsProjection {
	counts := DeltaCountsProjection{}
	for _, delta := range deltas {
		switch delta.Status {
		case domain.DeltaStatusOpen:
			counts.Open++
		case domain.DeltaStatusInProgress:
			counts.InProgress++
		case domain.DeltaStatusClosed:
			counts.Closed++
		case domain.DeltaStatusDeferred:
			counts.Deferred++
		case domain.DeltaStatusWithdrawn:
			counts.Withdrawn++
		}
	}
	return counts
}

func buildOpenDeltas(deltas []domain.Delta) []OpenDeltaProjection {
	open := make([]OpenDeltaProjection, 0)
	for _, delta := range deltas {
		if delta.Status == domain.DeltaStatusClosed || delta.Status == domain.DeltaStatusDeferred || delta.Status == domain.DeltaStatusWithdrawn {
			continue
		}
		open = append(open, OpenDeltaProjection{
			ID:     delta.ID,
			Area:   delta.Area,
			Status: delta.Status,
		})
	}
	return open
}

func buildActionableUnverifiedRequirements(requirements []domain.Requirement) []RequirementSummaryProjection {
	actionable := make([]RequirementSummaryProjection, 0)
	for _, requirement := range requirements {
		if requirement.EffectiveLifecycle() != domain.RequirementLifecycleActive {
			continue
		}
		if requirement.EffectiveVerification() == domain.RequirementVerificationVerified {
			continue
		}
		actionable = append(actionable, RequirementSummaryProjection{
			ID:           requirement.ID,
			Title:        requirement.Title,
			Tags:         append([]string{}, requirement.Tags...),
			Lifecycle:    requirement.EffectiveLifecycle(),
			Verification: requirement.EffectiveVerification(),
			IntroducedBy: introducedByForRequirement(requirement),
			TestFiles:    append([]string{}, requirement.TestFiles...),
		})
	}
	return actionable
}

func buildInactiveUnverifiedRequirements(requirements []domain.Requirement) []RequirementSummaryProjection {
	inactive := make([]RequirementSummaryProjection, 0)
	for _, requirement := range requirements {
		if requirement.EffectiveLifecycle() == domain.RequirementLifecycleActive {
			continue
		}
		if requirement.EffectiveVerification() == domain.RequirementVerificationVerified {
			continue
		}
		inactive = append(inactive, RequirementSummaryProjection{
			ID:           requirement.ID,
			Title:        requirement.Title,
			Tags:         append([]string{}, requirement.Tags...),
			Lifecycle:    requirement.EffectiveLifecycle(),
			Verification: requirement.EffectiveVerification(),
			IntroducedBy: introducedByForRequirement(requirement),
			TestFiles:    append([]string{}, requirement.TestFiles...),
		})
	}
	return inactive
}

func buildRequirementProjections(requirements []domain.Requirement, contexts map[string]infrastructure.RequirementDocContext) []RequirementProjection {
	cloned := make([]RequirementProjection, 0, len(requirements))
	for _, requirement := range requirements {
		context := contexts[requirement.ID]
		matchStatus := context.MatchStatus
		if matchStatus == "" {
			matchStatus = "missing_in_spec"
		}
		cloned = append(cloned, RequirementProjection{
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
		})
	}
	return cloned
}

func buildDeltaItemProjections(deltas []domain.Delta) []DeltaItemProjection {
	items := make([]DeltaItemProjection, 0, len(deltas))
	for _, delta := range deltas {
		intent := delta.Intent
		if intent == "" {
			intent = domain.DeltaIntentAdd
		}
		updates := append([]string{}, delta.Updates...)
		if len(updates) == 0 {
			updates = inferDeltaUpdates(intent)
		}
		items = append(items, DeltaItemProjection{
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
		})
	}
	return items
}

func introducedByForRequirement(requirement domain.Requirement) string {
	return requirement.IntroducedBy
}

func inferDeltaUpdates(intent domain.DeltaIntent) []string {
	switch intent {
	case domain.DeltaIntentChange:
		return []string{"replace_requirement"}
	case domain.DeltaIntentRemove:
		return []string{"withdraw_requirement"}
	case domain.DeltaIntentRepair:
		return []string{"stale_requirement"}
	default:
		return []string{"add_requirement"}
	}
}
