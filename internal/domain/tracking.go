package domain

import (
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strings"
	"time"
)

type SpecStatus string

const (
	SpecStatusDraft    SpecStatus = "draft"
	SpecStatusReady    SpecStatus = "ready"
	SpecStatusActive   SpecStatus = "active"
	SpecStatusVerified SpecStatus = "verified"
)

var (
	slugPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*$`)
	tagPattern  = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*$`)
	shaPattern  = regexp.MustCompile(`^[0-9a-f]{7,40}$`)
)

type Documents struct {
	Primary   string   `yaml:"primary" json:"primary"`
	Secondary []string `yaml:"secondary,omitempty" json:"secondary,omitempty"`
}

type ChangelogEntry struct {
	Rev             int      `yaml:"rev" json:"rev"`
	Date            string   `yaml:"date" json:"date"`
	DeltasOpened    []string `yaml:"deltas_opened" json:"deltas_opened"`
	DeltasClosed    []string `yaml:"deltas_closed" json:"deltas_closed"`
	DeltasWithdrawn []string `yaml:"deltas_withdrawn,omitempty" json:"deltas_withdrawn,omitempty"`
	ReqsAdded       []string `yaml:"reqs_added" json:"reqs_added"`
	ReqsVerified    []string `yaml:"reqs_verified" json:"reqs_verified"`
	Summary         string   `yaml:"summary" json:"summary"`
}

type TrackingFile struct {
	Slug           string           `yaml:"slug"`
	Charter        string           `yaml:"charter"`
	Title          string           `yaml:"title"`
	Status         SpecStatus       `yaml:"status"`
	Rev            int              `yaml:"rev"`
	Created        string           `yaml:"created"`
	Updated        string           `yaml:"updated"`
	LastVerifiedAt string           `yaml:"last_verified_at"`
	Checkpoint     string           `yaml:"checkpoint"`
	Tags           []string         `yaml:"tags"`
	Documents      Documents        `yaml:"documents"`
	Scope          []string         `yaml:"scope"`
	Deltas         []Delta          `yaml:"deltas"`
	Requirements   []Requirement    `yaml:"requirements"`
	Changelog      []ChangelogEntry `yaml:"changelog"`
	FilePath       string           `yaml:"-"`
}

func ValidSpecStatuses() []SpecStatus {
	return []SpecStatus{SpecStatusDraft, SpecStatusReady, SpecStatusActive, SpecStatusVerified}
}

func IsValidSpecStatus(s string) bool {
	return slices.Contains(ValidSpecStatuses(), SpecStatus(s))
}

func (t *TrackingFile) DeltaByID(id string) *Delta {
	for i := range t.Deltas {
		if t.Deltas[i].ID == id {
			return &t.Deltas[i]
		}
	}
	return nil
}

func (t *TrackingFile) RequirementByID(id string) *Requirement {
	for i := range t.Requirements {
		if t.Requirements[i].ID == id {
			return &t.Requirements[i]
		}
	}
	return nil
}

func (t *TrackingFile) LiveDeltas() []Delta {
	live := make([]Delta, 0, len(t.Deltas))
	for _, delta := range t.Deltas {
		if delta.Status == DeltaStatusDeferred || delta.Status == DeltaStatusWithdrawn {
			continue
		}
		live = append(live, delta)
	}
	return live
}

func (t *TrackingFile) TracingRequirements(deltaID string) []Requirement {
	requirements := make([]Requirement, 0)
	for _, requirement := range t.Requirements {
		if requirement.IntroducedBy == deltaID {
			requirements = append(requirements, requirement)
		}
	}
	return requirements
}

func (t *TrackingFile) ComputedStatus() SpecStatus {
	liveDeltas := t.LiveDeltas()
	if len(liveDeltas) == 0 {
		return SpecStatusDraft
	}

	for _, delta := range liveDeltas {
		if !t.deltaUpdatesResolved(delta) {
			return SpecStatusReady
		}
	}

	for _, delta := range liveDeltas {
		if delta.Status != DeltaStatusClosed {
			return SpecStatusActive
		}
		for _, requirement := range t.RequirementsTouchedByDelta(delta) {
			if !requirement.IsActive() {
				continue
			}
			if requirement.EffectiveVerification() != RequirementVerificationVerified {
				return SpecStatusActive
			}
		}
	}

	return SpecStatusVerified
}

func (t *TrackingFile) SyncComputedStatus() {
	t.Status = t.ComputedStatus()
}

func (t *TrackingFile) AllocateNextDeltaID() (string, error) {
	return NextDeltaID(t.Deltas)
}

func (t *TrackingFile) AllocateNextRequirementID() (string, error) {
	return NextRequirementID(t.Requirements)
}

func (t *TrackingFile) BlockingRequirementsForDeltaClosure(deltaID string) ([]Requirement, error) {
	delta := t.DeltaByID(deltaID)
	if delta == nil {
		return nil, fmt.Errorf("delta %s not found", deltaID)
	}
	if !t.deltaUpdatesResolved(*delta) {
		return nil, fmt.Errorf("delta %s cannot be closed until required updates are resolved", deltaID)
	}

	tracingRequirements := t.RequirementsTouchedByDelta(*delta)
	blocking := make([]Requirement, 0, len(tracingRequirements))
	for _, requirement := range tracingRequirements {
		if !requirement.IsActive() {
			continue
		}
		if requirement.EffectiveVerification() != RequirementVerificationVerified {
			blocking = append(blocking, requirement)
		}
	}
	return blocking, nil
}

func (t *TrackingFile) ValidateDeltaClosure(deltaID string) error {
	blocking, err := t.BlockingRequirementsForDeltaClosure(deltaID)
	if err != nil {
		return err
	}
	if len(blocking) == 0 {
		return nil
	}

	messages := make([]string, 0, len(blocking))
	for _, requirement := range blocking {
		messages = append(messages, fmt.Sprintf("delta %s cannot be closed until requirement %s is verified", deltaID, requirement.ID))
	}
	return errors.New(strings.Join(messages, "; "))
}

func (t *TrackingFile) Validate() error {
	var errs validationErrors

	if !slugPattern.MatchString(t.Slug) {
		errs.add("slug must match ^[a-z0-9][a-z0-9-]*$")
	}
	if !slugPattern.MatchString(t.Charter) {
		errs.add("charter must match ^[a-z0-9][a-z0-9-]*$")
	}
	if strings.TrimSpace(t.Title) == "" {
		errs.add("title is required")
	}
	if !IsValidSpecStatus(string(t.Status)) {
		errs.add(fmt.Sprintf("status must be one of %v", ValidSpecStatuses()))
	}
	if t.Rev < 1 {
		errs.add("rev must be >= 1")
	}
	validateDateField(&errs, "created", t.Created)
	validateDateField(&errs, "updated", t.Updated)
	validateDateField(&errs, "last_verified_at", t.LastVerifiedAt)

	if !shaPattern.MatchString(t.Checkpoint) {
		errs.add("checkpoint must be a git SHA")
	}
	if err := ValidateStoredRepoFilePath(t.Documents.Primary); err != nil {
		errs.add(fmt.Sprintf("documents.primary %s", err))
	} else if !strings.HasSuffix(t.Documents.Primary, ".md") {
		errs.add("documents.primary must point to a markdown file")
	}
	seenDocs := make(map[string]struct{})
	for _, doc := range t.Documents.Secondary {
		if err := ValidateStoredRepoFilePath(doc); err != nil {
			errs.add(fmt.Sprintf("documents.secondary %q %s", doc, err))
			continue
		}
		if !strings.HasSuffix(doc, ".md") {
			errs.add(fmt.Sprintf("documents.secondary %q must point to a markdown file", doc))
			continue
		}
		if doc == t.Documents.Primary {
			errs.add(fmt.Sprintf("documents.secondary %q duplicates documents.primary", doc))
			continue
		}
		if _, exists := seenDocs[doc]; exists {
			errs.add(fmt.Sprintf("duplicate documents.secondary %q", doc))
			continue
		}
		seenDocs[doc] = struct{}{}
	}
	if len(t.Scope) == 0 {
		errs.add("scope must contain at least one directory")
	} else {
		primaryWithinScope := false
		for _, scope := range t.Scope {
			if err := ValidateStoredRepoDirPath(scope); err != nil {
				errs.add(fmt.Sprintf("scope %q %s", scope, err))
				continue
			}
			if strings.HasPrefix(t.Documents.Primary, scope) {
				primaryWithinScope = true
			}
		}
		// Root-level docs (no directory prefix) are always valid — scope governs code, not the doc location
		isRootDoc := t.Documents.Primary != "" && !strings.Contains(t.Documents.Primary, "/")
		if t.Documents.Primary != "" && !primaryWithinScope && !isRootDoc {
			errs.add("documents.primary must be under one of scope[]")
		}
	}

	seenTags := make(map[string]struct{}, len(t.Tags))
	for _, tag := range t.Tags {
		if !tagPattern.MatchString(tag) {
			errs.add(fmt.Sprintf("tag %q must match ^[a-z0-9][a-z0-9-]*$", tag))
			continue
		}
		if _, exists := seenTags[tag]; exists {
			errs.add(fmt.Sprintf("duplicate tag %q", tag))
			continue
		}
		seenTags[tag] = struct{}{}
	}

	if err := ValidateDeltaSequence(t.Deltas); err != nil {
		errs.add(err.Error())
	}
	if err := ValidateRequirementSequence(t.Requirements); err != nil {
		errs.add(err.Error())
	}

	deltaIDs := make(map[string]struct{}, len(t.Deltas))
	requirementIDs := make(map[string]struct{}, len(t.Requirements))
	for _, delta := range t.Deltas {
		deltaIDs[delta.ID] = struct{}{}
	}
	for _, requirement := range t.Requirements {
		requirementIDs[requirement.ID] = struct{}{}
	}

	for _, delta := range t.Deltas {
		for _, reqID := range delta.AffectsRequirements {
			if _, exists := requirementIDs[reqID]; !exists {
				errs.add(fmt.Sprintf("delta %s affects_requirements references unknown requirement %s", delta.ID, reqID))
			}
		}
	}

	for _, requirement := range t.Requirements {
		if _, exists := deltaIDs[requirement.IntroducedBy]; !exists {
			errs.add(fmt.Sprintf("requirement %s introduced_by unknown delta %s", requirement.ID, requirement.IntroducedBy))
		}
		for _, testFile := range requirement.TestFiles {
			if err := ValidateStoredRepoFilePath(testFile); err != nil {
				errs.add(fmt.Sprintf("requirement %s test_file %q %s", requirement.ID, testFile, err))
			}
		}
		if requirement.EffectiveVerification() == RequirementVerificationVerified && !requirement.IsManual() && requirement.EffectiveLifecycle() != RequirementLifecycleSuperseded && len(requirement.TestFiles) == 0 {
			errs.add(fmt.Sprintf("requirement %s must declare test_files when verified", requirement.ID))
		}
	}

	for _, delta := range t.Deltas {
		if delta.Status != DeltaStatusClosed {
			continue
		}
		if err := t.ValidateDeltaClosure(delta.ID); err != nil {
			errs.add(err.Error())
		}
	}

	for _, entry := range t.Changelog {
		if entry.Rev < 1 {
			errs.add("changelog rev must be >= 1")
		}
		validateDateField(&errs, "changelog.date", entry.Date)
		if strings.TrimSpace(entry.Summary) == "" {
			errs.add(fmt.Sprintf("changelog rev %d summary is required", entry.Rev))
		}
	}

	if computed := t.ComputedStatus(); t.Status != computed {
		errs.add(fmt.Sprintf("status must match derived lifecycle state: expected %s, found %s", computed, t.Status))
	}

	return errs.err()
}

type validationErrors struct {
	messages []string
}

func (v *validationErrors) add(message string) {
	v.messages = append(v.messages, message)
}

func (v *validationErrors) err() error {
	if len(v.messages) == 0 {
		return nil
	}
	return errors.New(strings.Join(v.messages, "; "))
}

func validateDateField(errs *validationErrors, field, value string) {
	if _, err := time.Parse("2006-01-02", value); err != nil {
		errs.add(fmt.Sprintf("%s must use YYYY-MM-DD", field))
	}
}

func (t *TrackingFile) RequirementsTouchedByDelta(delta Delta) []Requirement {
	ids := make([]string, 0, len(delta.AffectsRequirements)+1)
	ids = append(ids, delta.AffectsRequirements...)
	for _, requirement := range t.Requirements {
		if requirement.IntroducedBy == delta.ID {
			ids = append(ids, requirement.ID)
		}
	}
	if len(ids) == 0 {
		return nil
	}
	unique := uniqueStrings(ids)
	requirements := make([]Requirement, 0, len(unique))
	for _, id := range unique {
		requirement := t.RequirementByID(id)
		if requirement != nil {
			requirements = append(requirements, *requirement)
		}
	}
	return requirements
}

func (t *TrackingFile) deltaUpdatesResolved(delta Delta) bool {
	switch delta.Intent {
	case DeltaIntentChange:
		if len(delta.AffectsRequirements) == 0 {
			return false
		}
		for _, id := range delta.AffectsRequirements {
			requirement := t.RequirementByID(id)
			if requirement == nil || requirement.EffectiveLifecycle() != RequirementLifecycleSuperseded {
				return false
			}
		}
		for _, requirement := range t.Requirements {
			if requirement.IntroducedBy == delta.ID {
				return true
			}
		}
		return false
	case DeltaIntentRemove:
		if len(delta.AffectsRequirements) == 0 {
			return false
		}
		for _, id := range delta.AffectsRequirements {
			requirement := t.RequirementByID(id)
			if requirement == nil || requirement.EffectiveLifecycle() != RequirementLifecycleWithdrawn {
				return false
			}
		}
		return true
	case DeltaIntentRepair:
		if len(delta.AffectsRequirements) == 0 {
			return false
		}
		for _, id := range delta.AffectsRequirements {
			requirement := t.RequirementByID(id)
			if requirement == nil || requirement.EffectiveVerification() == RequirementVerificationUnverified {
				return false
			}
		}
		return true
	default:
		return len(t.TracingRequirements(delta.ID)) > 0
	}
}

func (t *TrackingFile) DeltaUpdatesResolved(deltaID string) bool {
	delta := t.DeltaByID(deltaID)
	if delta == nil {
		return false
	}
	return t.deltaUpdatesResolved(*delta)
}

func (t *TrackingFile) DeltasTouchingRequirement(requirementID string) []string {
	ids := make([]string, 0)
	requirement := t.RequirementByID(requirementID)
	if requirement != nil && requirement.IntroducedBy != "" {
		ids = append(ids, requirement.IntroducedBy)
	}
	for _, delta := range t.Deltas {
		if slices.Contains(delta.AffectsRequirements, requirementID) {
			ids = append(ids, delta.ID)
		}
	}
	return uniqueStrings(ids)
}

func uniqueStrings(values []string) []string {
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
