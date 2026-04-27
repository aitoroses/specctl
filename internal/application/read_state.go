package application

import (
	"fmt"
	"sort"
	"strings"

	"github.com/aitoroses/specctl/internal/domain"
	"github.com/aitoroses/specctl/internal/infrastructure"
)

type trackingReadState struct {
	tracking *domain.TrackingFile
	findings []infrastructure.ValidationFinding
}

type charterReadState struct {
	charter  *domain.Charter
	findings []infrastructure.ValidationFinding
}

type repoReadState struct {
	repoRoot           string
	config             *infrastructure.ProjectConfig
	configReadFindings []infrastructure.ValidationFinding
	auditFindings      []infrastructure.ValidationFinding
	trackings          map[string]*trackingReadState
	charters           map[string]*charterReadState
}

func (s *Service) loadRepoReadState() (*repoReadState, error) {
	snapshot, err := s.repoReadAdapter().LoadRepoReadSnapshot()
	if err != nil {
		return nil, err
	}
	return s.repoReadStateFromSnapshot(snapshot), nil
}

func (s *Service) repoReadStateFromSnapshot(snapshot *infrastructure.RepoReadSnapshot) *repoReadState {
	evaluation := infrastructure.EvaluateRepoSnapshot(snapshot)
	state := &repoReadState{
		repoRoot:           s.repoRoot,
		config:             snapshot.Config,
		configReadFindings: append([]infrastructure.ValidationFinding{}, snapshot.ConfigReadFindings...),
		auditFindings:      append([]infrastructure.ValidationFinding{}, evaluation.AuditFindings...),
		trackings:          map[string]*trackingReadState{},
		charters:           map[string]*charterReadState{},
	}

	for key, tracking := range snapshot.Trackings {
		findings := append([]infrastructure.ValidationFinding{}, tracking.Findings...)
		if canonical, exists := evaluation.SpecFindings[key]; exists {
			findings = append([]infrastructure.ValidationFinding{}, canonical...)
		}
		state.trackings[key] = &trackingReadState{
			tracking: tracking.Tracking,
			findings: findings,
		}
	}

	for name, charter := range snapshot.Charters {
		findings := append([]infrastructure.ValidationFinding{}, charter.Findings...)
		if canonical, exists := evaluation.CharterFindings[name]; exists {
			findings = append([]infrastructure.ValidationFinding{}, canonical...)
		}
		state.charters[name] = &charterReadState{
			charter:  charter.Charter,
			findings: findings,
		}
	}
	return state
}

func (s *Service) canonicalCharterProjection(name string) (CharterProjection, error) {
	repoState, err := s.loadRepoReadState()
	if err != nil {
		return CharterProjection{}, err
	}
	return s.charterProjectionFromRepoState(repoState, name)
}

func (s *Service) charterProjectionFromRepoState(repoState *repoReadState, name string) (CharterProjection, error) {
	charterState := repoState.charterState(name)
	if charterState == nil {
		return CharterProjection{}, &Failure{
			Code:    "CHARTER_NOT_FOUND",
			Message: fmt.Sprintf("charter %q does not exist", name),
			State:   map[string]any{"charter": name},
		}
	}

	trackingBySlug := repoState.trackingBySlug(name)
	trackingFindings := make(map[string][]infrastructure.ValidationFinding, len(charterState.charter.Specs))
	for _, entry := range charterState.charter.Specs {
		trackingFindings[entry.Slug] = repoState.specValidation(name + ":" + entry.Slug)
	}

	return newCharterProjection(
		s.repoRoot,
		charterState.charter,
		trackingBySlug,
		repoState.charterValidation(name),
		trackingFindings,
	)
}

func (s *Service) specProjectionFromRepoState(repoState *repoReadState, target string) (SpecProjection, error) {
	trackingState := repoState.specTracking(target)
	if trackingState == nil {
		return SpecProjection{}, &Failure{
			Code:    "SPEC_NOT_FOUND",
			Message: fmt.Sprintf("spec %q does not exist", target),
			State:   map[string]any{"spec": target},
		}
	}
	charterName, _, _ := strings.Cut(target, ":")
	var charter *domain.Charter
	if charterState := repoState.charterState(charterName); charterState != nil {
		charter = charterState.charter
	}
	return s.projectSpec(trackingState.tracking, charter, repoState.config, repoState.specValidation(target))
}

func (s *repoReadState) sortedCharterNames() []string {
	names := make([]string, 0, len(s.charters))
	for name := range s.charters {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func (s *repoReadState) sortedTrackingTargets() []string {
	targets := make([]string, 0, len(s.trackings))
	for target := range s.trackings {
		targets = append(targets, target)
	}
	sort.Strings(targets)
	return targets
}

func (s *repoReadState) specTracking(target string) *trackingReadState {
	return s.trackings[target]
}

func (s *repoReadState) charterState(name string) *charterReadState {
	return s.charters[name]
}

func (s *repoReadState) trackingBySlug(charterName string) map[string]*domain.TrackingFile {
	bySlug := make(map[string]*domain.TrackingFile)
	for key, tracking := range s.trackings {
		name, slug, _ := strings.Cut(key, ":")
		if name != charterName {
			continue
		}
		bySlug[slug] = tracking.tracking
	}
	return bySlug
}

func (s *repoReadState) charterValidation(name string) []infrastructure.ValidationFinding {
	charterState := s.charterState(name)
	if charterState == nil {
		return []infrastructure.ValidationFinding{{
			Code:     "CHARTER_SPEC_MISSING",
			Severity: "error",
			Message:  "charter file does not exist",
			Path:     infrastructure.RelativeCharterPath(name),
		}}
	}
	return append([]infrastructure.ValidationFinding{}, charterState.findings...)
}

func (s *repoReadState) specValidation(target string) []infrastructure.ValidationFinding {
	charterName, slug, _ := strings.Cut(target, ":")
	trackingState := s.specTracking(target)
	if trackingState != nil {
		return append([]infrastructure.ValidationFinding{}, trackingState.findings...)
	}
	charterState := s.charterState(charterName)
	if charterState == nil {
		return []infrastructure.ValidationFinding{{
			Code:     "CHARTER_SPEC_MISSING",
			Severity: "error",
			Message:  "charter file does not exist",
			Path:     infrastructure.RelativeCharterPath(charterName),
			Target:   slug,
		}}
	}
	findings := make([]infrastructure.ValidationFinding, 0)
	for _, finding := range charterState.findings {
		if finding.Target == "" || finding.Target == slug || finding.Target == "specs" || finding.Target == "groups" {
			findings = append(findings, finding)
		}
	}
	return uniqueValidationFindings(findings)
}

func (s *repoReadState) charterOrder() map[string]map[string]int {
	order := make(map[string]map[string]int, len(s.charters))
	for name, charterState := range s.charters {
		order[name] = domain.BuildLenientCharterOrdering(charterState.charter).Index
	}
	return order
}

func uniqueValidationFindings(findings []infrastructure.ValidationFinding) []infrastructure.ValidationFinding {
	seen := make(map[string]struct{}, len(findings))
	result := make([]infrastructure.ValidationFinding, 0, len(findings))
	for _, finding := range findings {
		key := finding.Code + "\x00" + finding.Path + "\x00" + finding.Target + "\x00" + finding.Message
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		result = append(result, finding)
	}
	return result
}

func findingsToAny(findings []infrastructure.ValidationFinding) []any {
	items := make([]any, 0, len(findings))
	for _, finding := range findings {
		items = append(items, finding)
	}
	return items
}
