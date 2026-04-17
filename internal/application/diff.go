package application

import (
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/aitoroses/specctl/internal/domain"
	"github.com/aitoroses/specctl/internal/infrastructure"
)

type DiffEndpoint struct {
	Rev        int               `json:"rev"`
	Checkpoint string            `json:"checkpoint"`
	Status     domain.SpecStatus `json:"status"`
}

type CharterDiffEndpoint struct {
	Rev        int    `json:"rev"`
	Checkpoint string `json:"checkpoint"`
}

type DiffStatusModel struct {
	From *domain.SpecStatus `json:"from"`
	To   domain.SpecStatus  `json:"to"`
}

type DiffSetModel struct {
	Added   []string `json:"added"`
	Removed []string `json:"removed"`
}

type DiffDocumentsModel struct {
	PrimaryFrom *string `json:"primary_from"`
	PrimaryTo   string  `json:"primary_to"`
}

type DiffDeltaSummary struct {
	ID               string             `json:"id"`
	Area             string             `json:"area"`
	OriginCheckpoint string             `json:"origin_checkpoint"`
	Status           domain.DeltaStatus `json:"status"`
	Current          string             `json:"current"`
	Target           string             `json:"target"`
}

type DiffRequirementSummary struct {
	ID    string   `json:"id"`
	Title string   `json:"title"`
	Tags  []string `json:"tags"`
}

type SpecDiffProjection struct {
	Target       string                          `json:"target"`
	Baseline     string                          `json:"baseline"`
	Comparison   string                          `json:"comparison"`
	DriftSource  *string                         `json:"drift_source"`
	From         *DiffEndpoint                   `json:"from"`
	To           DiffEndpoint                    `json:"to"`
	Model        SpecDiffModel                   `json:"model"`
	DesignDoc    DesignDocDiff                   `json:"design_doc"`
	ScopeCode    ScopeCodeDiff                   `json:"scope_code"`
	Requirements DiffRequirementIssuesProjection `json:"requirements"`
	Validation   ValidationProjection            `json:"validation"`
	Focus        any                             `json:"focus,omitempty"`
}

type DiffRequirementIssuesProjection struct {
	MatchIssues []DiffRequirementMatchIssue `json:"match_issues"`
}

type DiffRequirementMatchIssue struct {
	ID     string `json:"id"`
	Status string `json:"status"`
}

type SpecDiffModel struct {
	Status       DiffStatusModel          `json:"status"`
	SpecTags     DiffSetModel             `json:"spec_tags"`
	Documents    DiffDocumentsModel       `json:"documents"`
	Scope        DiffSetModel             `json:"scope"`
	Deltas       SpecDiffDeltaModel       `json:"deltas"`
	Requirements SpecDiffRequirementModel `json:"requirements"`
}

type SpecDiffDeltaModel struct {
	Opened   []DiffDeltaSummary `json:"opened"`
	Closed   []DiffDeltaSummary `json:"closed"`
	Deferred []DiffDeltaSummary `json:"deferred"`
	Resumed  []DiffDeltaSummary `json:"resumed"`
}

type SpecDiffRequirementModel struct {
	Added    []DiffRequirementSummary `json:"added"`
	Verified []DiffRequirementSummary `json:"verified"`
}

type DesignDocDiff struct {
	Path            string                 `json:"path"`
	Changed         bool                   `json:"changed"`
	SectionsChanged []DesignDocSectionDiff `json:"sections_changed"`
}

type ScopeCodeDiff struct {
	ChangedFiles []string `json:"changed_files"`
}

type DesignDocSectionDiff struct {
	Heading string `json:"heading"`
	Type    string `json:"type"`
	Lines   [2]int `json:"lines"`
}

type CharterDiffProjection struct {
	Charter      string                      `json:"charter"`
	OrderedSpecs []CharterDiffSpecProjection `json:"ordered_specs"`
	Validation   ValidationProjection        `json:"validation"`
}

type CharterDiffSpecProjection struct {
	Slug       string               `json:"slug"`
	DependsOn  []string             `json:"depends_on"`
	From       *CharterDiffEndpoint `json:"from"`
	To         CharterDiffEndpoint  `json:"to"`
	Changed    bool                 `json:"changed"`
	Validation ValidationProjection `json:"validation"`
}

func (s *Service) ReadDiff(target, charterName string) (any, []any, error) {
	if charterName != "" {
		return s.readCharterDiff(charterName)
	}
	return s.readSpecDiff(target)
}

func (s *Service) readSpecDiff(target string) (SpecDiffProjection, []any, error) {
	charterName, slug, ok := strings.Cut(target, ":")
	if !ok {
		return SpecDiffProjection{}, nil, fmt.Errorf("invalid spec target %q", target)
	}

	trackingExists, err := s.pathAdapter().TrackingExists(charterName, slug)
	if err != nil {
		return SpecDiffProjection{}, nil, fmt.Errorf("checking tracking path: %w", err)
	}
	if !trackingExists {
		charterExists, charterErr := s.pathAdapter().CharterExists(charterName)
		if charterErr != nil {
			return SpecDiffProjection{}, nil, fmt.Errorf("checking charter path: %w", charterErr)
		}
		return SpecDiffProjection{}, nil, &Failure{
			Code:    "SPEC_NOT_FOUND",
			Message: "spec not found",
			State: MissingSpecContext{
				Target:        target,
				TrackingFile:  infrastructure.RelativeTrackingPath(s.repoRoot, charterName, slug),
				CharterExists: charterExists,
			},
			Next: []any{},
		}
	}

	repoState, err := s.loadRepoReadState()
	if err != nil {
		return SpecDiffProjection{}, nil, err
	}
	trackingState := repoState.specTracking(target)
	if trackingState == nil {
		return SpecDiffProjection{}, nil, &Failure{
			Code:    "SPEC_NOT_FOUND",
			Message: "spec not found",
			State: MissingSpecContext{
				Target:        target,
				TrackingFile:  infrastructure.RelativeTrackingPath(s.repoRoot, charterName, slug),
				CharterExists: repoState.charterState(charterName) != nil,
			},
			Next: []any{},
		}
	}

	current := trackingState.tracking
	findings := append([]infrastructure.ValidationFinding{}, repoState.specValidation(target)...)

	var (
		baselineTracking *domain.TrackingFile
		comparison       infrastructure.SpecComparison
		changedFiles     []string
		driftSource      string
	)
	var from *DiffEndpoint
	baselineDocAvailable := true
	comparison, err = s.checkpointStore().LoadSpecComparison(charterName, slug, current, current.Checkpoint)
	if err != nil {
		findings = append(findings, checkpointUnavailableDiffFinding(s.repoRoot, charterName, slug, err))
	} else {
		baselineTracking = comparison.BaselineTracking
		if comparison.BaselineDocError != nil {
			baselineDocAvailable = false
			findings = append(findings, checkpointUnavailableDiffFinding(s.repoRoot, charterName, slug, comparison.BaselineDocError))
		}
	}
	if baselineTracking != nil && baselineTracking.Rev > 0 {
		status := baselineTracking.ComputedStatus()
		from = &DiffEndpoint{
			Rev:        baselineTracking.Rev,
			Checkpoint: baselineTracking.Checkpoint,
			Status:     status,
		}
	}
	if files, diffErr := infrastructure.GitDiffNameOnly(s.repoRoot, current.Checkpoint, "HEAD", append(append([]string{}, current.Scope...), current.Documents.Primary)); diffErr == nil {
		changedFiles = append([]string{}, files...)
		docChanged := false
		codeChanged := false
		for _, file := range changedFiles {
			if file == current.Documents.Primary {
				docChanged = true
			} else {
				codeChanged = true
			}
		}
		driftSource = infrastructure.ClassifyDriftSource(docChanged, codeChanged)
	}

	diff := buildSpecDiffProjection(
		target,
		current,
		baselineTracking,
		comparison.CurrentDoc,
		comparison.BaselineDoc,
		comparison.NormalizedCurrent,
		comparison.NormalizedBaseline,
		changedFiles,
		driftSource,
		uniqueValidationFindings(findings),
	)
	if !baselineDocAvailable {
		diff.DesignDoc.Changed = false
		diff.DesignDoc.SectionsChanged = []DesignDocSectionDiff{}
	}
	diff.From = from
	specState, err := s.specProjectionFromRepoState(repoState, target)
	if err != nil {
		return SpecDiffProjection{}, nil, err
	}
	diff.Requirements.MatchIssues = buildDiffRequirementMatchIssues(specState.Requirements)
	diff.Focus = buildSpecDiffFocus(diff, specState)
	return diff, buildSpecDiffNext(diff, specState, target), nil
}

func (s *Service) readCharterDiff(name string) (CharterDiffProjection, []any, error) {
	stateAny, _, err := s.readCharterContext(name)
	if err != nil {
		return CharterDiffProjection{}, nil, err
	}
	state, ok := stateAny.(CharterProjection)
	if !ok {
		return CharterDiffProjection{}, nil, &Failure{
			Code:    "CHARTER_NOT_FOUND",
			Message: "charter not found",
			State: MissingCharterContext{
				Charter:      name,
				TrackingFile: infrastructure.RelativeCharterPath(name),
			},
			Next: []any{},
		}
	}

	repoState, err := s.loadRepoReadState()
	if err != nil {
		return CharterDiffProjection{}, nil, err
	}

	ordered := make([]CharterDiffSpecProjection, 0, len(state.OrderedSpecs))
	for _, entry := range state.OrderedSpecs {
		trackingState := repoState.specTracking(name + ":" + entry.Slug)
		if trackingState == nil {
			continue
		}
		tracking := trackingState.tracking
		current := CharterDiffEndpoint{
			Rev:        tracking.Rev,
			Checkpoint: tracking.Checkpoint,
		}

		var from *CharterDiffEndpoint
		changed := true
		comparison, err := s.checkpointStore().LoadSpecComparison(name, entry.Slug, tracking, tracking.Checkpoint)
		if err == nil && comparison.BaselineTracking != nil && comparison.BaselineTracking.Rev > 0 {
			baselineTracking := comparison.BaselineTracking
			from = &CharterDiffEndpoint{
				Rev:        baselineTracking.Rev,
				Checkpoint: baselineTracking.Checkpoint,
			}
			diff := buildSemanticDiff(baselineTracking, tracking, comparison.NormalizedBaseline, comparison.NormalizedCurrent)
			changed = diff.hasSemanticChanges()
		}

		ordered = append(ordered, CharterDiffSpecProjection{
			Slug:       entry.Slug,
			DependsOn:  append([]string{}, entry.DependsOn...),
			From:       from,
			To:         current,
			Changed:    changed,
			Validation: projectionFromFindings(repoState.specValidation(name + ":" + entry.Slug)),
		})
	}

	return CharterDiffProjection{
		Charter:      name,
		OrderedSpecs: ordered,
		Validation:   projectionFromFindings(repoState.charterValidation(name)),
	}, []any{}, nil
}

func buildSpecDiffProjection(target string, current, baseline *domain.TrackingFile, currentDoc, baselineDoc, normalizedCurrentDoc, normalizedBaselineDoc []byte, changedFiles []string, driftSource string, findings []infrastructure.ValidationFinding) SpecDiffProjection {
	normalizedCurrent := normalizeTrackingForSemanticDiff(current)
	currentStatus := normalizedCurrent.ComputedStatus()
	model := initialBaselineSpecDiffModel(normalizedCurrent, currentStatus)
	if baseline != nil && baseline.Rev > 0 {
		normalizedBaseline := normalizeTrackingForSemanticDiff(baseline)
		status := normalizedBaseline.ComputedStatus()
		model = SpecDiffModel{
			Status:       DiffStatusModel{From: &status, To: currentStatus},
			SpecTags:     diffStringSet(normalizedBaseline.Tags, normalizedCurrent.Tags),
			Documents:    DiffDocumentsModel{PrimaryFrom: &normalizedBaseline.Documents.Primary, PrimaryTo: normalizedCurrent.Documents.Primary},
			Scope:        diffStringSet(normalizedBaseline.Scope, normalizedCurrent.Scope),
			Deltas:       diffDeltas(normalizedBaseline.Deltas, normalizedCurrent.Deltas),
			Requirements: diffRequirements(normalizedBaseline.Requirements, normalizedCurrent.Requirements),
		}
	}

	designDocChanged := false
	designDocSections := []DesignDocSectionDiff{}
	if baseline != nil && baseline.Rev > 0 {
		designDocChanged = string(normalizedBaselineDoc) != string(normalizedCurrentDoc)
		designDocSections = diffDesignDocSections(baselineDoc, currentDoc)
	}

	return SpecDiffProjection{
		Target:      target,
		Baseline:    "checkpoint",
		Comparison:  current.Checkpoint + "..HEAD",
		DriftSource: nullableProjectionString(driftSource),
		From:        nil,
		To: DiffEndpoint{
			Rev:        current.Rev,
			Checkpoint: current.Checkpoint,
			Status:     currentStatus,
		},
		Model: model,
		DesignDoc: DesignDocDiff{
			Path:            current.Documents.Primary,
			Changed:         designDocChanged,
			SectionsChanged: designDocSections,
		},
		ScopeCode: ScopeCodeDiff{ChangedFiles: filterScopeCodeChangedFiles(changedFiles, current.Documents.Primary)},
		Requirements: DiffRequirementIssuesProjection{
			MatchIssues: []DiffRequirementMatchIssue{},
		},
		Validation: projectionFromFindings(findings),
	}
}

func buildDiffRequirementMatchIssues(requirements []RequirementProjection) []DiffRequirementMatchIssue {
	issues := make([]DiffRequirementMatchIssue, 0)
	for _, requirement := range requirements {
		if requirement.Lifecycle != domain.RequirementLifecycleActive {
			continue
		}
		if !isBlockingRequirementMatchStatus(requirement.Match.Status) {
			continue
		}
		issues = append(issues, DiffRequirementMatchIssue{
			ID:     requirement.ID,
			Status: requirement.Match.Status,
		})
	}
	return issues
}

func buildSpecDiffFocus(diff SpecDiffProjection, state SpecProjection) any {
	if state.ScopeDrift.Status != "drifted" {
		return nil
	}

	reviewSurface := map[string]any{}
	reviewSurface["classification"] = driftClassificationFocus(state)
	if diff.DesignDoc.Changed && len(diff.DesignDoc.SectionsChanged) > 0 {
		reviewSurface["sections_changed"] = append([]DesignDocSectionDiff{}, diff.DesignDoc.SectionsChanged...)
		if blocks := changedRequirementBlocks(diff.DesignDoc.SectionsChanged); len(blocks) > 0 {
			reviewSurface["changed_requirement_blocks"] = blocks
		}
	}
	if len(diff.ScopeCode.ChangedFiles) > 0 {
		reviewSurface["scope_code"] = map[string]any{
			"changed_files": append([]string{}, diff.ScopeCode.ChangedFiles...),
		}
	}
	if len(reviewSurface) == 0 {
		return nil
	}
	return map[string]any{"review_surface": reviewSurface}
}

func changedRequirementBlocks(sections []DesignDocSectionDiff) []DesignDocSectionDiff {
	blocks := make([]DesignDocSectionDiff, 0)
	for _, section := range sections {
		if strings.HasPrefix(section.Heading, "Requirement: ") {
			blocks = append(blocks, section)
		}
	}
	return blocks
}

func initialBaselineSpecDiffModel(current *domain.TrackingFile, currentStatus domain.SpecStatus) SpecDiffModel {
	return SpecDiffModel{
		Status:    DiffStatusModel{From: nil, To: currentStatus},
		SpecTags:  diffStringSet(nil, current.Tags),
		Documents: DiffDocumentsModel{PrimaryFrom: nil, PrimaryTo: current.Documents.Primary},
		Scope:     diffStringSet(nil, current.Scope),
		Deltas: SpecDiffDeltaModel{
			Opened:   []DiffDeltaSummary{},
			Closed:   []DiffDeltaSummary{},
			Deferred: []DiffDeltaSummary{},
			Resumed:  []DiffDeltaSummary{},
		},
		Requirements: SpecDiffRequirementModel{
			Added:    []DiffRequirementSummary{},
			Verified: []DiffRequirementSummary{},
		},
	}
}

type parsedDesignDocSection struct {
	heading    string
	headingKey string
	occurrence int
	startLine  int
	endLine    int
	content    []byte
}

var atxHeadingPattern = regexp.MustCompile(`^[ \t]{0,3}(#{1,6})[ \t]+(.+?)[ \t]*#*[ \t]*$`)

func diffDesignDocSections(baselineDoc, currentDoc []byte) []DesignDocSectionDiff {
	baselineSections := parseDesignDocSections(baselineDoc)
	currentSections := parseDesignDocSections(currentDoc)
	diffs := make([]DesignDocSectionDiff, 0, len(baselineSections)+len(currentSections))

	type sectionIdentity struct {
		headingKey string
		occurrence int
	}

	baselineByIdentity := make(map[sectionIdentity]parsedDesignDocSection, len(baselineSections))
	currentByIdentity := make(map[sectionIdentity]parsedDesignDocSection, len(currentSections))
	identities := make([]sectionIdentity, 0, len(baselineSections)+len(currentSections))
	seenIdentities := make(map[sectionIdentity]struct{}, len(baselineSections)+len(currentSections))

	for _, section := range baselineSections {
		identity := sectionIdentity{headingKey: section.headingKey, occurrence: section.occurrence}
		baselineByIdentity[identity] = section
		identities = append(identities, identity)
		seenIdentities[identity] = struct{}{}
	}
	for _, section := range currentSections {
		identity := sectionIdentity{headingKey: section.headingKey, occurrence: section.occurrence}
		currentByIdentity[identity] = section
		if _, exists := seenIdentities[identity]; exists {
			continue
		}
		identities = append(identities, identity)
	}

	sort.Slice(identities, func(i, j int) bool {
		if identities[i].headingKey != identities[j].headingKey {
			return identities[i].headingKey < identities[j].headingKey
		}
		return identities[i].occurrence < identities[j].occurrence
	})

	for _, identity := range identities {
		baselineSection, inBaseline := baselineByIdentity[identity]
		currentSection, inCurrent := currentByIdentity[identity]
		switch {
		case inBaseline && inCurrent:
			if string(baselineSection.content) == string(currentSection.content) {
				continue
			}
			diffs = append(diffs, DesignDocSectionDiff{
				Heading: currentSection.heading,
				Type:    "modified",
				Lines:   [2]int{currentSection.startLine, currentSection.endLine},
			})
		case inBaseline:
			diffs = append(diffs, DesignDocSectionDiff{
				Heading: baselineSection.heading,
				Type:    "removed",
				Lines:   [2]int{baselineSection.startLine, baselineSection.endLine},
			})
		case inCurrent:
			diffs = append(diffs, DesignDocSectionDiff{
				Heading: currentSection.heading,
				Type:    "added",
				Lines:   [2]int{currentSection.startLine, currentSection.endLine},
			})
		}
	}

	sort.Slice(diffs, func(i, j int) bool {
		left := diffs[i]
		right := diffs[j]
		if left.Lines[0] != right.Lines[0] {
			return left.Lines[0] < right.Lines[0]
		}
		if left.Lines[1] != right.Lines[1] {
			return left.Lines[1] < right.Lines[1]
		}
		if left.Heading != right.Heading {
			return left.Heading < right.Heading
		}
		return left.Type < right.Type
	})
	return diffs
}

func parseDesignDocSections(data []byte) []parsedDesignDocSection {
	content := normalizeDocText(data)
	bodyLines, bodyStartLine := designDocBodyLines(content)
	if len(bodyLines) == 0 {
		return []parsedDesignDocSection{}
	}

	type heading struct {
		line  int
		title string
	}

	headings := make([]heading, 0)
	for index, line := range bodyLines {
		title, ok := parseATXHeading(line)
		if !ok {
			continue
		}
		headings = append(headings, heading{
			line:  bodyStartLine + index,
			title: title,
		})
	}
	if len(headings) == 0 {
		return []parsedDesignDocSection{}
	}

	totalLines := len(contentLines(content))
	occurrences := make(map[string]int, len(headings))
	sections := make([]parsedDesignDocSection, 0, len(headings))
	for i, sectionHeading := range headings {
		endLine := totalLines
		if i+1 < len(headings) {
			endLine = headings[i+1].line - 1
		}
		startIndex := sectionHeading.line - bodyStartLine
		endIndex := endLine - bodyStartLine
		if startIndex < 0 {
			startIndex = 0
		}
		if endIndex >= len(bodyLines) {
			endIndex = len(bodyLines) - 1
		}
		sectionLines := []string{}
		if startIndex <= endIndex && startIndex < len(bodyLines) {
			sectionLines = append(sectionLines, bodyLines[startIndex:endIndex+1]...)
		}
		headingKey := normalizeHeadingTitle(sectionHeading.title)
		occurrences[headingKey]++
		sections = append(sections, parsedDesignDocSection{
			heading:    sectionHeading.title,
			headingKey: headingKey,
			occurrence: occurrences[headingKey],
			startLine:  sectionHeading.line,
			endLine:    endLine,
			content:    []byte(strings.Join(sectionLines, "\n")),
		})
	}
	return sections
}

func designDocBodyLines(content string) ([]string, int) {
	lines := contentLines(content)
	if len(lines) == 0 || lines[0] != "---" {
		return lines, 1
	}
	for index := 1; index < len(lines); index++ {
		if lines[index] == "---" {
			return lines[index+1:], index + 2
		}
	}
	return lines, 1
}

func parseATXHeading(line string) (string, bool) {
	matches := atxHeadingPattern.FindStringSubmatch(line)
	if len(matches) != 3 {
		return "", false
	}
	title := strings.TrimSpace(matches[2])
	if title == "" {
		return "", false
	}
	return title, true
}

func normalizeHeadingTitle(title string) string {
	return strings.ToLower(strings.Join(strings.Fields(strings.TrimSpace(title)), " "))
}

func normalizeDocText(data []byte) string {
	return strings.ReplaceAll(string(data), "\r\n", "\n")
}

func contentLines(content string) []string {
	lines := strings.Split(content, "\n")
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	return lines
}

func diffStringSet(previous, current []string) DiffSetModel {
	prevSet := make(map[string]struct{}, len(previous))
	currentSet := make(map[string]struct{}, len(current))
	for _, value := range previous {
		prevSet[value] = struct{}{}
	}
	for _, value := range current {
		currentSet[value] = struct{}{}
	}

	added := make([]string, 0)
	for _, value := range current {
		if _, exists := prevSet[value]; !exists {
			added = append(added, value)
		}
	}
	removed := make([]string, 0)
	for _, value := range previous {
		if _, exists := currentSet[value]; !exists {
			removed = append(removed, value)
		}
	}
	return DiffSetModel{Added: added, Removed: removed}
}

func filterScopeCodeChangedFiles(changedFiles []string, designDocPath string) []string {
	filtered := make([]string, 0, len(changedFiles))
	for _, file := range changedFiles {
		if file == designDocPath {
			continue
		}
		filtered = append(filtered, file)
	}
	return filtered
}

func diffDeltas(previous, current []domain.Delta) SpecDiffDeltaModel {
	prev := make(map[string]domain.Delta, len(previous))
	for _, delta := range previous {
		prev[delta.ID] = delta
	}
	model := SpecDiffDeltaModel{
		Opened:   []DiffDeltaSummary{},
		Closed:   []DiffDeltaSummary{},
		Deferred: []DiffDeltaSummary{},
		Resumed:  []DiffDeltaSummary{},
	}
	for _, delta := range current {
		if old, exists := prev[delta.ID]; !exists {
			model.Opened = append(model.Opened, summarizeDelta(delta))
		} else {
			if old.Status != domain.DeltaStatusClosed && delta.Status == domain.DeltaStatusClosed {
				model.Closed = append(model.Closed, summarizeDelta(delta))
			}
			if old.Status != domain.DeltaStatusDeferred && delta.Status == domain.DeltaStatusDeferred {
				model.Deferred = append(model.Deferred, summarizeDelta(delta))
			}
			if old.Status == domain.DeltaStatusDeferred && delta.Status == domain.DeltaStatusOpen {
				model.Resumed = append(model.Resumed, summarizeDelta(delta))
			}
		}
	}
	return model
}

func diffRequirements(previous, current []domain.Requirement) SpecDiffRequirementModel {
	prev := make(map[string]domain.Requirement, len(previous))
	for _, requirement := range previous {
		prev[requirement.ID] = requirement
	}
	model := SpecDiffRequirementModel{
		Added:    []DiffRequirementSummary{},
		Verified: []DiffRequirementSummary{},
	}
	for _, requirement := range current {
		if old, exists := prev[requirement.ID]; !exists {
			model.Added = append(model.Added, summarizeRequirement(requirement))
		} else if old.EffectiveVerification() != domain.RequirementVerificationVerified && requirement.EffectiveVerification() == domain.RequirementVerificationVerified {
			model.Verified = append(model.Verified, summarizeRequirement(requirement))
		}
	}
	return model
}

func summarizeDeltas(deltas []domain.Delta) []DiffDeltaSummary {
	summaries := make([]DiffDeltaSummary, 0, len(deltas))
	for _, delta := range deltas {
		summaries = append(summaries, summarizeDelta(delta))
	}
	return summaries
}

func summarizeDelta(delta domain.Delta) DiffDeltaSummary {
	return DiffDeltaSummary{
		ID:               delta.ID,
		Area:             delta.Area,
		OriginCheckpoint: delta.OriginCheckpoint,
		Status:           delta.Status,
		Current:          delta.Current,
		Target:           delta.Target,
	}
}

func summarizeRequirement(requirement domain.Requirement) DiffRequirementSummary {
	return DiffRequirementSummary{
		ID:    requirement.ID,
		Title: requirement.Title,
		Tags:  append([]string{}, requirement.Tags...),
	}
}

func checkpointUnavailableDiffFinding(repoRoot, charter, slug string, err error) infrastructure.ValidationFinding {
	return infrastructure.ValidationFinding{
		Code:     "CHECKPOINT_UNAVAILABLE",
		Severity: "error",
		Message:  err.Error(),
		Path:     infrastructure.RelativeTrackingPath(repoRoot, charter, slug),
		Target:   "checkpoint",
	}
}

func buildSpecDiffNext(diff SpecDiffProjection, state SpecProjection, target string) []any {
	switch state.ScopeDrift.Status {
	case "drifted":
		// If all work is done (unbumped closed deltas, no open deltas, verified),
		// suggest rev bump instead of semantic diff options.
		if state.Status == domain.SpecStatusVerified && len(state.OpenDeltas) == 0 && hasUnbumpedClosedDeltas(state) {
			return buildTrackedRevisionBumpNext(state, target)
		}
		return buildDiffDriftNext(diff, state, target)
	case "tracked":
		if continuation := buildTrackedDriftContinuationNext(state, target); len(continuation) > 0 {
			return continuation
		}
		return buildTrackedRevisionBumpNext(state, target)
	case "unavailable":
		return buildDriftGuidanceNext(state, target, false, false, 1)
	default:
		return []any{}
	}
}

func hasUnbumpedClosedDeltas(state SpecProjection) bool {
	recorded := make(map[string]struct{})
	for _, entry := range state.Changelog {
		for _, id := range entry.DeltasClosed {
			recorded[id] = struct{}{}
		}
	}
	for _, delta := range state.Deltas.Items {
		if delta.Status == domain.DeltaStatusClosed {
			if _, ok := recorded[delta.ID]; !ok {
				return true
			}
		}
	}
	return false
}

func buildDiffDriftNext(diff SpecDiffProjection, state SpecProjection, target string) []any {
	source := scopeDriftSourceValue(state.ScopeDrift.DriftSource)
	switch source {
	case "scope_code":
		return buildSemanticDiffOptions(target, "The committed drift is code-only. Review the changed files and choose whether it needs semantic tracking or only a checkpoint sync.", driftSyncCandidate(state), syncChooseWhen(source))
	case "both":
		return buildSemanticDiffOptions(target, "The committed drift spans design-doc and code changes. Review the changed sections and choose the correct workflow branch.", driftSyncCandidate(state), syncChooseWhen(source))
	case "design_doc":
		return buildSemanticDiffOptions(target, fmt.Sprintf("The design document changed in %d sections. Choose the semantic path that matches the observed contract change.", len(diff.DesignDoc.SectionsChanged)), driftSyncCandidate(state), syncChooseWhen(source))
	default:
		return buildDriftGuidanceNext(state, target, false, false, 1)
	}
}

func buildSemanticDiffOptions(target, why string, includeSync bool, syncChooseWhenText string) []any {
	options := []any{
		deltaIntentOption(1, "add", target, why, "Choose this when the diff introduces net-new observable behavior."),
		deltaIntentOption(2, "change", target, why, "Choose this when an active requirement is no longer the correct statement of truth."),
		deltaIntentOption(3, "remove", target, why, "Choose this when observable behavior was intentionally removed."),
		deltaIntentOption(4, "repair", target, why, "Choose this when behavior remains true but its evidence or tracked wording needs repair."),
		map[string]any{
			"priority":     5,
			"action":       "refresh_requirement",
			"kind":         "run_command",
			"instructions": "Choose this when requirement identity is unchanged and only the exact tracked block needs refreshing.",
			"choose_when":  "Same requirement identity; only match text changes.",
			"template": map[string]any{
				"argv":            []string{"specctl", "req", "refresh", target, "<requirement-id>"},
				"stdin_format":    "gherkin",
				"stdin_template":  "@tag\nFeature: <feature>\n",
				"required_fields": []map[string]any{{"name": "requirement_id", "description": "Tracked requirement to refresh"}, {"name": "gherkin_requirement", "description": "Requirement-level Gherkin block from SPEC.md"}},
			},
		},
	}
	if includeSync {
		options = append(options, map[string]any{
			"priority":     6,
			"action":       "sync",
			"kind":         "run_command",
			"instructions": "Choose this only when review confirms the drift is clarification-only and does not require semantic spec work.",
			"choose_when":  syncChooseWhenText,
			"template": map[string]any{
				"argv":           []string{"specctl", "sync", target, "--checkpoint", "HEAD"},
				"stdin_format":   "text",
				"stdin_template": "<summary>\n",
				"required_fields": []map[string]any{
					{"name": "summary", "description": "One-line reason the checkpoint is being re-anchored"},
				},
			},
		})
	}
	return options
}

func deltaIntentOption(priority int, intent, target, why, chooseWhen string) map[string]any {
	stdinTemplate := "current: <current>\ntarget: <target>\nnotes: <notes>\n"
	required := []map[string]any{
		{"name": "area", "description": "Short delta area label"},
		{"name": "current", "description": "Current state"},
		{"name": "target", "description": "Target state"},
		{"name": "notes", "description": "Why this delta exists"},
	}
	if intent == "change" || intent == "remove" || intent == "repair" {
		stdinTemplate += "affects_requirements:\n  - <requirement-id>\n"
		required = append(required, map[string]any{"name": "affects_requirements", "description": "Active requirement IDs affected by this workflow branch"})
	}
	return map[string]any{
		"priority":     priority,
		"action":       "delta_add_" + intent,
		"kind":         "run_command",
		"instructions": why,
		"choose_when":  chooseWhen,
		"template": map[string]any{
			"argv":            []string{"specctl", "delta", "add", target, "--intent", intent, "--area", "<area>"},
			"stdin_format":    "yaml",
			"stdin_template":  stdinTemplate,
			"required_fields": required,
		},
	}
}
