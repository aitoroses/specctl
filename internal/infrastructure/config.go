package infrastructure

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/aitoroses/specctl/internal/domain"
	"gopkg.in/yaml.v3"
)

var (
	configTagPattern          = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*$`)
	formatKeyPattern          = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*$`)
	semanticTagSet            = map[string]struct{}{"e2e": {}, "manual": {}}
	defaultGherkinTags        = []string{"runtime", "domain", "ui", "integration", "contract", "workflow"}
	defaultPrefixes           = []string{"runtime/src/", "ui/src/", "ui/convex/", "ui/server/"}
)

type warningCacheDocument struct {
	FingerprintByPath map[string]string `json:"fingerprint_by_path"`
}

// ProjectConfig holds project-level specctl configuration.
type ProjectConfig struct {
	GherkinTags    []string                `yaml:"gherkin_tags"`
	SourcePrefixes []string                `yaml:"source_prefixes"`
	Formats        map[string]FormatConfig `yaml:"formats"`
}

type projectConfigDocument struct {
	GherkinTags    *[]string               `yaml:"gherkin_tags"`
	SourcePrefixes *[]string               `yaml:"source_prefixes"`
	Formats        map[string]FormatConfig `yaml:"formats"`
}

type FormatConfig struct {
	Template       string `yaml:"template" json:"template"`
	RecommendedFor string `yaml:"recommended_for" json:"recommended_for"`
	Description    string `yaml:"description" json:"description"`
}

func DefaultConfig() *ProjectConfig {
	return &ProjectConfig{
		GherkinTags:    slices.Clone(defaultGherkinTags),
		SourcePrefixes: slices.Clone(defaultPrefixes),
		Formats:        map[string]FormatConfig{},
	}
}

func DefaultGherkinTags() []string {
	return slices.Clone(defaultGherkinTags)
}

func IsDefaultGherkinTag(tag string) bool {
	return slices.Contains(defaultGherkinTags, tag)
}

func LoadProjectConfig(specsDir string) (*ProjectConfig, error) {
	path := filepath.Join(specsDir, "specctl.yaml")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return DefaultConfig(), nil
		}
		return nil, fmt.Errorf("reading %s: %w", path, err)
	}

	return loadProjectConfigFromData(path, data)
}

func LoadProjectConfigLenient(specsDir string) (*ProjectConfig, []ValidationFinding, error) {
	path := filepath.Join(specsDir, "specctl.yaml")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return DefaultConfig(), []ValidationFinding{}, nil
		}
		return nil, nil, fmt.Errorf("reading %s: %w", path, err)
	}

	return loadProjectConfigLenientFromData(path, data, true)
}

func loadProjectConfigFromData(path string, data []byte) (*ProjectConfig, error) {
	document, err := decodeProjectConfigDocumentStrict(path, data)
	if err != nil {
		return nil, err
	}

	config := configFromDocument(document)
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("validating %s: %w", path, err)
	}
	return config, nil
}

func loadProjectConfigLenientFromData(path string, data []byte, emitRedundantWarnings bool) (*ProjectConfig, []ValidationFinding, error) {
	document, findings, err := decodeProjectConfigDocumentLenient(path, data)
	if err != nil {
		return nil, nil, err
	}
	if document == nil {
		return DefaultConfig(), findings, nil
	}

	config := configFromDocument(document)
	if document.GherkinTags != nil {
		redundantFindings := redundantSemanticTagFindings(*document.GherkinTags)
		if emitRedundantWarnings && shouldEmitRedundantSemanticTagWarnings(path, data, redundantFindings) {
			findings = append(findings, redundantFindings...)
		}
	}
	if err := config.Validate(); err != nil {
		findings = append(findings, ValidationFindingsFromMessages(err.Error(), filepath.ToSlash(filepath.Join(".specs", "specctl.yaml")), "config")...)
	}

	return config, findings, nil
}

func decodeProjectConfigDocumentStrict(path string, data []byte) (*projectConfigDocument, error) {
	var document projectConfigDocument
	if err := strictUnmarshal(data, &document); err != nil {
		return nil, fmt.Errorf("parsing %s: %w", path, err)
	}
	return &document, nil
}

func decodeProjectConfigDocumentLenient(path string, data []byte) (*projectConfigDocument, []ValidationFinding, error) {
	var document projectConfigDocument
	findings := make([]ValidationFinding, 0)
	if err := yaml.Unmarshal(data, &document); err != nil {
		findings = append(findings, ValidationFinding{
			Code:     "CONFIG_FORMAT_INVALID",
			Severity: "error",
			Message:  fmt.Sprintf("parsing %s: %v", path, err),
			Path:     filepath.ToSlash(filepath.Join(".specs", "specctl.yaml")),
			Target:   "config",
		})
		return nil, findings, nil
	}

	return &document, findings, nil
}

func configFromDocument(document *projectConfigDocument) *ProjectConfig {
	config := DefaultConfig()
	if document == nil {
		return config
	}
	if document.GherkinTags != nil {
		config.GherkinTags = sanitizePersistedTags(*document.GherkinTags)
	}
	if document.SourcePrefixes != nil {
		config.SourcePrefixes = slices.Clone(*document.SourcePrefixes)
	}
	if document.Formats != nil {
		config.Formats = cloneFormats(document.Formats)
	}
	return config
}

func (c *ProjectConfig) Validate() error {
	if c == nil {
		return fmt.Errorf("project config is required")
	}

	seenTags := make(map[string]struct{}, len(c.GherkinTags))
	for _, tag := range c.GherkinTags {
		if !configTagPattern.MatchString(tag) {
			return fmt.Errorf("gherkin_tags %q must match ^[a-z0-9][a-z0-9-]*$", tag)
		}
		if _, reserved := semanticTagSet[tag]; reserved {
			return fmt.Errorf("gherkin_tags %q is a reserved semantic tag", tag)
		}
		if _, exists := seenTags[tag]; exists {
			return fmt.Errorf("gherkin_tags contains duplicate value %q", tag)
		}
		seenTags[tag] = struct{}{}
	}

	seenPrefixes := make(map[string]struct{}, len(c.SourcePrefixes))
	for i, prefix := range c.SourcePrefixes {
		normalized, err := domain.NormalizeRepoDir(prefix)
		if err != nil {
			return fmt.Errorf("source_prefixes[%d]: %w", i, err)
		}
		if normalized != prefix {
			return fmt.Errorf("source_prefixes[%d] must be stored as a normalized repo-relative directory ending in /", i)
		}
		if _, exists := seenPrefixes[prefix]; exists {
			return fmt.Errorf("source_prefixes contains duplicate value %q", prefix)
		}
		seenPrefixes[prefix] = struct{}{}
	}

	for key, format := range c.Formats {
		if !formatKeyPattern.MatchString(key) {
			return fmt.Errorf("formats key %q must match ^[a-z0-9][a-z0-9-]*$", key)
		}
		if err := domain.ValidateStoredRepoFilePath(format.Template); err != nil {
			return fmt.Errorf("formats.%s.template %s", key, err)
		}
		if strings.TrimSpace(format.RecommendedFor) == "" {
			return fmt.Errorf("formats.%s.recommended_for is required", key)
		}
		if strings.TrimSpace(format.Description) == "" {
			return fmt.Errorf("formats.%s.description is required", key)
		}
	}

	if c.Formats == nil {
		c.Formats = map[string]FormatConfig{}
	}

	return nil
}

func sanitizePersistedTags(tags []string) []string {
	filtered := make([]string, 0, len(tags))
	for _, tag := range tags {
		if _, reserved := semanticTagSet[tag]; reserved {
			continue
		}
		filtered = append(filtered, tag)
	}
	return filtered
}

func redundantSemanticTagFindings(tags []string) []ValidationFinding {
	findings := make([]ValidationFinding, 0)
	seen := make(map[string]struct{})
	for _, tag := range tags {
		if _, reserved := semanticTagSet[tag]; !reserved {
			continue
		}
		if _, exists := seen[tag]; exists {
			continue
		}
		seen[tag] = struct{}{}
		findings = append(findings, ValidationFinding{
			Code:     "REDUNDANT_SEMANTIC_TAG",
			Severity: "warning",
			Message:  fmt.Sprintf("gherkin_tags %q is a reserved semantic tag", tag),
			Path:     filepath.ToSlash(filepath.Join(".specs", "specctl.yaml")),
			Target:   "gherkin_tags",
		})
	}
	return findings
}

func shouldEmitRedundantSemanticTagWarnings(path string, data []byte, findings []ValidationFinding) bool {
	cachePath, err := redundantSemanticTagWarningCachePath()
	if err != nil {
		return len(findings) > 0
	}
	document, err := loadWarningCacheDocument(cachePath)
	if err != nil {
		return len(findings) > 0
	}
	if document.FingerprintByPath == nil {
		document.FingerprintByPath = map[string]string{}
	}

	normalizedPath := filepath.Clean(path)
	if len(findings) == 0 {
		delete(document.FingerprintByPath, normalizedPath)
		_ = writeWarningCacheDocument(cachePath, document)
		return false
	}

	fingerprint := string(data)
	if document.FingerprintByPath[normalizedPath] == fingerprint {
		return false
	}
	document.FingerprintByPath[normalizedPath] = fingerprint
	if err := writeWarningCacheDocument(cachePath, document); err != nil {
		return true
	}
	return true
}

func redundantSemanticTagWarningCachePath() (string, error) {
	cacheDir, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(cacheDir, "specctl", "redundant-semantic-tags.json"), nil
}

func loadWarningCacheDocument(path string) (*warningCacheDocument, error) {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return &warningCacheDocument{FingerprintByPath: map[string]string{}}, nil
	}
	if err != nil {
		return nil, err
	}

	var document warningCacheDocument
	if err := json.Unmarshal(data, &document); err != nil {
		return &warningCacheDocument{FingerprintByPath: map[string]string{}}, nil
	}
	if document.FingerprintByPath == nil {
		document.FingerprintByPath = map[string]string{}
	}
	return &document, nil
}

func writeWarningCacheDocument(path string, document *warningCacheDocument) error {
	if document == nil {
		document = &warningCacheDocument{FingerprintByPath: map[string]string{}}
	}
	data, err := json.Marshal(document)
	if err != nil {
		return err
	}
	return WriteFileAtomically(path, data, 0o644)
}

func cloneFormats(formats map[string]FormatConfig) map[string]FormatConfig {
	cloned := make(map[string]FormatConfig, len(formats))
	for key, value := range formats {
		cloned[key] = value
	}
	return cloned
}

func CloneFormatsForProjection(formats map[string]FormatConfig) map[string]FormatConfig {
	return cloneFormats(formats)
}
