package infrastructure

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/aitoroses/specctl/internal/domain"
	"gopkg.in/yaml.v3"
)

type ValidationFinding struct {
	Code     string `json:"code"`
	Severity string `json:"severity"`
	Message  string `json:"message"`
	Path     string `json:"path"`
	Target   string `json:"target,omitempty"`
}

func ReadTrackingFileLenient(path string) (*domain.TrackingFile, []ValidationFinding, error) {
	location, err := resolveRegistryLocation(path)
	if err != nil {
		return nil, nil, err
	}
	config, _, err := LoadProjectConfigLenient(filepath.Join(location.repoRoot, ".specs"))
	if err != nil {
		return nil, nil, fmt.Errorf("loading specctl config: %w", err)
	}
	return ReadTrackingFileLenientWithConfig(path, config)
}

func ReadTrackingFileLenientWithConfig(path string, config *ProjectConfig) (*domain.TrackingFile, []ValidationFinding, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, nil, fmt.Errorf("reading tracking file: %w", err)
	}

	return readTrackingFileLenientWithConfigData(path, data, config, nil)
}

func readTrackingFileLenientWithConfigData(path string, data []byte, config *ProjectConfig, repoFiles map[string][]byte) (*domain.TrackingFile, []ValidationFinding, error) {
	location, err := resolveRegistryLocation(path)
	if err != nil {
		return nil, nil, err
	}
	if location.isCharter {
		return nil, nil, fmt.Errorf("tracking file path must point to .specs/{charter}/{slug}.yaml")
	}
	if config == nil {
		config = DefaultConfig()
	}

	relativePath := RelativeTrackingPath(location.repoRoot, location.charter, location.slug)
	findings := make([]ValidationFinding, 0)

	node, err := decodeYAMLNode(data)
	if err != nil {
		findings = append(findings, ValidationFinding{
			Code:     "SPEC_STATUS_INVALID",
			Severity: "error",
			Message:  fmt.Sprintf("parsing tracking YAML: %v", err),
			Path:     relativePath,
		})
		tracking := &domain.TrackingFile{
			Slug:     location.slug,
			Charter:  location.charter,
			FilePath: path,
		}
		return tracking, findings, nil
	}
	if err := requireTrackingKeys(node); err != nil {
		findings = append(findings, ValidationFindingsFromMessages(err.Error(), relativePath, "")...)
	}

	var tracking domain.TrackingFile
	if err := yaml.Unmarshal(data, &tracking); err != nil {
		findings = append(findings, ValidationFinding{
			Code:     "SPEC_STATUS_INVALID",
			Severity: "error",
			Message:  fmt.Sprintf("parsing tracking YAML: %v", err),
			Path:     relativePath,
		})
	}
	tracking.FilePath = path

	rawSlug := tracking.Slug
	rawCharter := tracking.Charter
	if normalized, normalizeErr := domain.NormalizeRepoPath(tracking.Documents.Primary); normalizeErr == nil {
		tracking.Documents.Primary = normalized
	}
	for i := range tracking.Documents.Secondary {
		if normalized, normalizeErr := domain.NormalizeRepoPath(tracking.Documents.Secondary[i]); normalizeErr == nil {
			tracking.Documents.Secondary[i] = normalized
		}
	}
	for i := range tracking.Scope {
		if normalized, normalizeErr := domain.NormalizeRepoDir(tracking.Scope[i]); normalizeErr == nil {
			tracking.Scope[i] = normalized
		}
	}
	for i := range tracking.Requirements {
		if normalized, normalizeErr := domain.NormalizeRequirementBlock(tracking.Requirements[i].Gherkin); normalizeErr == nil {
			tracking.Requirements[i].Gherkin = normalized
		}
		for j := range tracking.Requirements[i].TestFiles {
			if normalized, normalizeErr := domain.NormalizeRepoPath(tracking.Requirements[i].TestFiles[j]); normalizeErr == nil {
				tracking.Requirements[i].TestFiles[j] = normalized
			}
		}
	}

	if err := tracking.Validate(); err != nil {
		findings = append(findings, ValidationFindingsFromMessages(err.Error(), relativePath, "")...)
	}
	if rawCharter != "" && rawCharter != location.charter {
		findings = append(findings, ValidationFindingFromError(fmt.Errorf("tracking charter %q does not match path charter %q", rawCharter, location.charter), relativePath, "charter"))
	}
	if rawSlug != "" && rawSlug != location.slug {
		findings = append(findings, ValidationFindingFromError(fmt.Errorf("tracking slug %q does not match path slug %q", rawSlug, location.slug), relativePath, "slug"))
	}

	if tracking.Documents.Primary != "" {
		if err := validateTrackedDesignDocumentWithOverlay(location.repoRoot, tracking.Documents.Primary, location.charter, location.slug, config, repoFiles); err != nil {
			findings = append(findings, ValidationFindingFromError(err, relativePath, "documents.primary"))
		}
	}
	for _, doc := range tracking.Documents.Secondary {
		if doc == "" {
			continue
		}
		absPath := filepath.Join(location.repoRoot, filepath.FromSlash(doc))
		if _, err := os.Stat(absPath); err != nil {
			findings = append(findings, ValidationFinding{
				Code:     "SECONDARY_DOC_MISSING",
				Severity: "error",
				Message:  fmt.Sprintf("documents.secondary does not exist: %s", doc),
				Path:     relativePath,
				Target:   "documents.secondary",
			})
		}
	}

	for i := range tracking.Scope {
		if _, err := normalizeExistingRepoDir(location.repoRoot, tracking.Scope[i]); err != nil {
			findings = append(findings, ValidationFindingFromError(fmt.Errorf("scope: %w", err), relativePath, "scope"))
		}
	}
	for i := range tracking.Requirements {
		requirement := tracking.Requirements[i]
		if err := domain.ValidateRequirementTagsConfigured(requirement.Tags, config.GherkinTags); err != nil {
			findings = append(findings, ValidationFindingFromError(fmt.Errorf("requirement %s %s", requirement.ID, err), relativePath, "requirements"))
		}
		for _, testFile := range requirement.TestFiles {
			if requirement.EffectiveVerification() != domain.RequirementVerificationVerified || requirement.IsManual() {
				continue
			}
			if _, err := os.Stat(filepath.Join(location.repoRoot, filepath.FromSlash(testFile))); err != nil {
				findings = append(findings, ValidationFindingFromError(fmt.Errorf("requirement %s test file does not exist: %s", requirement.ID, testFile), relativePath, "requirements"))
			}
		}
	}

	if tracking.Charter == "" || tracking.Charter != location.charter {
		tracking.Charter = location.charter
	}
	if tracking.Slug == "" || tracking.Slug != location.slug {
		tracking.Slug = location.slug
	}

	return &tracking, uniqueFindings(findings), nil
}

func validateTrackedDesignDocumentWithOverlay(repoRoot, relativePath, charter, slug string, config *ProjectConfig, repoFiles map[string][]byte) error {
	if len(repoFiles) == 0 {
		return ValidateTrackedDesignDocumentWithConfig(repoRoot, relativePath, charter, slug, config)
	}

	normalizedPath, err := domain.NormalizeRepoPath(relativePath)
	if err != nil {
		return fmt.Errorf("primary design document path: %w", err)
	}
	if data, exists := repoFiles[normalizedPath]; exists {
		return validateTrackedDesignDocumentContentWithConfig(normalizedPath, charter, slug, config, data)
	}
	return ValidateTrackedDesignDocumentWithConfig(repoRoot, normalizedPath, charter, slug, config)
}

func uniqueFindings(findings []ValidationFinding) []ValidationFinding {
	seen := make(map[string]struct{}, len(findings))
	result := make([]ValidationFinding, 0, len(findings))
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
