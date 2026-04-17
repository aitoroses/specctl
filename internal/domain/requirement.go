package domain

import (
	"fmt"
	"regexp"
	"slices"
	"strconv"
	"strings"
)

var (
	reqIDPattern   = regexp.MustCompile(`^REQ-(\d{3})$`)
	featureLineRe  = regexp.MustCompile(`(?m)^\s*Feature:\s+(.+?)\s*$`)
	scenarioLineRe = regexp.MustCompile(`(?m)^\s*Scenario:\s+.+$`)
	semanticTags   = []string{"e2e", "manual"}
)

type RequirementLifecycle string

const (
	RequirementLifecycleActive     RequirementLifecycle = "active"
	RequirementLifecycleSuperseded RequirementLifecycle = "superseded"
	RequirementLifecycleWithdrawn  RequirementLifecycle = "withdrawn"
)

type RequirementVerification string

const (
	RequirementVerificationUnverified RequirementVerification = "unverified"
	RequirementVerificationVerified   RequirementVerification = "verified"
	RequirementVerificationStale      RequirementVerification = "stale"
)

type Requirement struct {
	ID            string                  `yaml:"id" json:"id"`
	Title         string                  `yaml:"title" json:"title"`
	Traces        []string                `yaml:"-" json:"-"`
	Tags          []string                `yaml:"tags" json:"tags"`
	TestFiles     []string                `yaml:"test_files" json:"test_files"`
	Verified      bool                    `yaml:"-" json:"-"`
	Gherkin       string                  `yaml:"gherkin" json:"gherkin"`
	Lifecycle     RequirementLifecycle    `yaml:"lifecycle,omitempty" json:"lifecycle,omitempty"`
	Verification  RequirementVerification `yaml:"verification,omitempty" json:"verification,omitempty"`
	IntroducedBy  string                  `yaml:"introduced_by,omitempty" json:"introduced_by,omitempty"`
	Supersedes    string                  `yaml:"supersedes,omitempty" json:"supersedes,omitempty"`
	SupersededBy  string                  `yaml:"superseded_by,omitempty" json:"superseded_by,omitempty"`
}

func NewRequirement(id string, deltaID string, gherkin string) (Requirement, error) {
	normalized, err := NormalizeRequirementBlock(gherkin)
	if err != nil {
		return Requirement{}, err
	}
	title, err := DeriveRequirementTitle(normalized)
	if err != nil {
		return Requirement{}, err
	}
	tags, err := DeriveRequirementTags(normalized)
	if err != nil {
		return Requirement{}, err
	}

	requirement := Requirement{
		ID:           id,
		Title:        title,
		Tags:         tags,
		TestFiles:    []string{},
		Gherkin:      normalized,
		Lifecycle:    RequirementLifecycleActive,
		Verification: RequirementVerificationUnverified,
		IntroducedBy: deltaID,
	}
	if err := validateRequirement(requirement); err != nil {
		return Requirement{}, err
	}
	return requirement, nil
}

func IsValidReqID(id string) bool {
	return reqIDPattern.MatchString(id)
}

func ParseReqIDNumber(id string) (int, error) {
	matches := reqIDPattern.FindStringSubmatch(id)
	if matches == nil {
		return 0, fmt.Errorf("invalid requirement ID format: %s (expected REQ-NNN)", id)
	}

	n, err := strconv.Atoi(matches[1])
	if err != nil {
		return 0, fmt.Errorf("invalid requirement ID number: %s", id)
	}
	return n, nil
}

func ExpectedReqID(n int) string {
	return fmt.Sprintf("REQ-%03d", n)
}

func ExtractGherkinTags(line string) []string {
	fields := strings.Fields(strings.TrimSpace(line))
	tags := make([]string, 0, len(fields))
	for _, field := range fields {
		if !strings.HasPrefix(field, "@") || len(field) == 1 {
			continue
		}
		tags = append(tags, strings.TrimPrefix(field, "@"))
	}
	return tags
}

func DeriveRequirementTitle(gherkin string) (string, error) {
	matches := featureLineRe.FindStringSubmatch(gherkin)
	if len(matches) != 2 {
		return "", fmt.Errorf("gherkin must contain a Feature line")
	}
	return strings.TrimSpace(matches[1]), nil
}

func NormalizeRequirementBlock(gherkin string) (string, error) {
	normalized := strings.ReplaceAll(gherkin, "\r\n", "\n")
	lines := strings.Split(normalized, "\n")
	cleaned := make([]string, 0, len(lines))
	for _, line := range lines {
		cleaned = append(cleaned, strings.TrimRight(line, " \t"))
	}
	for len(cleaned) > 0 && strings.TrimSpace(cleaned[0]) == "" {
		cleaned = cleaned[1:]
	}
	for len(cleaned) > 0 && strings.TrimSpace(cleaned[len(cleaned)-1]) == "" {
		cleaned = cleaned[:len(cleaned)-1]
	}
	if len(cleaned) == 0 {
		return "", fmt.Errorf("gherkin is required")
	}
	requirementLines := make([]string, 0, len(cleaned))
	for _, line := range cleaned {
		trimmed := strings.TrimSpace(line)
		switch {
		case trimmed == "":
			continue
		case strings.HasPrefix(trimmed, "@"):
			requirementLines = append(requirementLines, trimmed)
		case strings.HasPrefix(trimmed, "Feature:"):
			requirementLines = append(requirementLines, trimmed)
			return strings.Join(requirementLines, "\n"), nil
		case strings.HasPrefix(trimmed, "Scenario:"):
			break
		}
	}
	return "", fmt.Errorf("gherkin must contain a Feature line")
}

func DeriveRequirementTags(gherkin string) ([]string, error) {
	lines := strings.Split(gherkin, "\n")
	tags := make([]string, 0)
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || !strings.HasPrefix(trimmed, "@") {
			continue
		}

		for _, tag := range ExtractGherkinTags(trimmed) {
			if !tagPattern.MatchString(tag) {
				return nil, fmt.Errorf("gherkin tag %q must match ^[a-z0-9][a-z0-9-]*$", tag)
			}
			tags = append(tags, tag)
		}
	}
	return tags, nil
}

func (r Requirement) HasTag(tag string) bool {
	for _, candidate := range r.Tags {
		if candidate == tag {
			return true
		}
	}
	return false
}

func (r Requirement) IsE2E() bool {
	return r.HasTag("e2e")
}

func (r Requirement) IsManual() bool {
	return r.HasTag("manual")
}

func (r Requirement) EffectiveLifecycle() RequirementLifecycle {
	if r.Lifecycle != "" {
		return r.Lifecycle
	}
	if strings.TrimSpace(r.SupersededBy) != "" {
		return RequirementLifecycleSuperseded
	}
	return RequirementLifecycleActive
}

func (r Requirement) EffectiveVerification() RequirementVerification {
	if r.Verification != "" {
		return r.Verification
	}
	if r.Verified {
		return RequirementVerificationVerified
	}
	return RequirementVerificationUnverified
}

func (r Requirement) IsActive() bool {
	return r.EffectiveLifecycle() == RequirementLifecycleActive
}

func (r Requirement) IsVerified() bool {
	return r.EffectiveVerification() == RequirementVerificationVerified
}

func SemanticRequirementTags() []string {
	return slices.Clone(semanticTags)
}

func ValidateRequirementTagsConfigured(tags []string, configured []string) error {
	allowed := make(map[string]struct{}, len(tags)+len(configured))
	for _, tag := range semanticTags {
		allowed[tag] = struct{}{}
	}
	for _, tag := range configured {
		allowed[tag] = struct{}{}
	}

	for _, tag := range tags {
		if _, ok := allowed[tag]; ok {
			continue
		}
		return fmt.Errorf("gherkin tag %q is not configured in specctl.yaml", tag)
	}
	return nil
}

func MissingRequirementTags(tags []string, configured []string) []string {
	allowed := make(map[string]struct{}, len(configured)+len(semanticTags))
	for _, tag := range semanticTags {
		allowed[tag] = struct{}{}
	}
	for _, tag := range configured {
		allowed[tag] = struct{}{}
	}
	missing := make([]string, 0)
	for _, tag := range tags {
		if _, ok := allowed[tag]; !ok {
			missing = append(missing, tag)
		}
	}
	return missing
}

func ValidateRequirementSequence(requirements []Requirement) error {
	for i, requirement := range requirements {
		expected := ExpectedReqID(i + 1)
		if requirement.ID != expected {
			return fmt.Errorf("requirement IDs must be sequential without gaps; expected %s, found %s", expected, requirement.ID)
		}
		if err := validateRequirement(requirement); err != nil {
			return err
		}
	}
	return nil
}

func NextRequirementID(requirements []Requirement) (string, error) {
	if err := ValidateRequirementSequence(requirements); err != nil {
		return "", err
	}
	return ExpectedReqID(len(requirements) + 1), nil
}

func validateRequirement(requirement Requirement) error {
	if !IsValidReqID(requirement.ID) {
		return fmt.Errorf("requirement ID %q must match REQ-NNN", requirement.ID)
	}
	if strings.TrimSpace(requirement.Title) == "" {
		return fmt.Errorf("requirement %s title is required", requirement.ID)
	}
	if strings.TrimSpace(requirement.IntroducedBy) == "" && len(requirement.Traces) > 0 {
		requirement.IntroducedBy = requirement.Traces[0]
	}
	if strings.TrimSpace(requirement.IntroducedBy) == "" {
		return fmt.Errorf("requirement %s introduced_by is required", requirement.ID)
	}
	if strings.TrimSpace(requirement.Gherkin) == "" {
		return fmt.Errorf("requirement %s gherkin is required", requirement.ID)
	}
	normalized, err := NormalizeRequirementBlock(requirement.Gherkin)
	if err != nil {
		return fmt.Errorf("requirement %s %s", requirement.ID, err.Error())
	}
	derivedTitle, err := DeriveRequirementTitle(normalized)
	if err != nil {
		return fmt.Errorf("requirement %s %s", requirement.ID, err.Error())
	}
	derivedTags, err := DeriveRequirementTags(normalized)
	if err != nil {
		return fmt.Errorf("requirement %s %s", requirement.ID, err.Error())
	}
	if requirement.Title != derivedTitle {
		return fmt.Errorf("requirement %s title must match the Gherkin Feature line", requirement.ID)
	}
	if requirement.Gherkin != normalized {
		return fmt.Errorf("requirement %s gherkin must be normalized requirement-block text", requirement.ID)
	}
	if !slices.Equal(requirement.Tags, derivedTags) {
		return fmt.Errorf("requirement %s tags must match Gherkin tag lines", requirement.ID)
	}
	switch requirement.EffectiveLifecycle() {
	case RequirementLifecycleActive, RequirementLifecycleSuperseded, RequirementLifecycleWithdrawn:
	default:
		return fmt.Errorf("requirement %s has invalid lifecycle %q", requirement.ID, requirement.Lifecycle)
	}
	switch requirement.EffectiveVerification() {
	case RequirementVerificationUnverified, RequirementVerificationVerified, RequirementVerificationStale:
	default:
		return fmt.Errorf("requirement %s has invalid verification %q", requirement.ID, requirement.Verification)
	}
	if requirement.EffectiveLifecycle() != RequirementLifecycleActive && requirement.EffectiveVerification() == RequirementVerificationStale {
		return fmt.Errorf("requirement %s stale verification is legal only for active requirements", requirement.ID)
	}
	return nil
}
