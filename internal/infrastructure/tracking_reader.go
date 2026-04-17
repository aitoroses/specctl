package infrastructure

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/aitoroses/specctl/internal/domain"
	"gopkg.in/yaml.v3"
)

func ReadTrackingFile(path string) (*domain.TrackingFile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading tracking file: %w", err)
	}

	location, err := resolveRegistryLocation(path)
	if err != nil {
		return nil, err
	}
	if location.isCharter {
		return nil, fmt.Errorf("tracking file path must point to .specs/{charter}/{slug}.yaml")
	}

	config, err := LoadProjectConfig(filepath.Join(location.repoRoot, ".specs"))
	if err != nil {
		return nil, fmt.Errorf("loading specctl config: %w", err)
	}
	return readTrackingFileWithConfig(path, data, location, config)
}

func readTrackingFileWithConfig(path string, data []byte, location *registryLocation, config *ProjectConfig) (*domain.TrackingFile, error) {
	node, err := decodeYAMLNode(data)
	if err != nil {
		return nil, fmt.Errorf("parsing tracking YAML: %w", err)
	}
	if err := requireTrackingKeys(node); err != nil {
		return nil, err
	}

	var tracking domain.TrackingFile
	if err := strictUnmarshal(data, &tracking); err != nil {
		return nil, fmt.Errorf("parsing tracking YAML: %w", err)
	}
	tracking.FilePath = path

	if tracking.Charter != location.charter {
		return nil, fmt.Errorf("tracking charter %q does not match path charter %q", tracking.Charter, location.charter)
	}
	if tracking.Slug != location.slug {
		return nil, fmt.Errorf("tracking slug %q does not match path slug %q", tracking.Slug, location.slug)
	}

	primary, err := domain.NormalizeRepoPath(tracking.Documents.Primary)
	if err != nil {
		return nil, fmt.Errorf("documents.primary: %w", err)
	}
	if filepath.Ext(primary) != ".md" {
		return nil, fmt.Errorf("documents.primary must point to a markdown file")
	}
	primaryInfo, err := os.Stat(filepath.Join(location.repoRoot, filepath.FromSlash(primary)))
	if err != nil {
		return nil, fmt.Errorf("documents.primary does not exist: %s", primary)
	}
	if primaryInfo.IsDir() {
		return nil, fmt.Errorf("documents.primary must point to a markdown file")
	}
	if err := ValidateTrackedDesignDocumentWithConfig(location.repoRoot, primary, tracking.Charter, tracking.Slug, config); err != nil {
		return nil, err
	}
	tracking.Documents.Primary = primary

	normalizedSecondary := make([]string, 0, len(tracking.Documents.Secondary))
	for _, doc := range tracking.Documents.Secondary {
		normalized, err := domain.NormalizeRepoPath(doc)
		if err != nil {
			return nil, fmt.Errorf("documents.secondary %q: %w", doc, err)
		}
		if filepath.Ext(normalized) != ".md" {
			return nil, fmt.Errorf("documents.secondary %q must point to a markdown file", doc)
		}
		info, err := os.Stat(filepath.Join(location.repoRoot, filepath.FromSlash(normalized)))
		if err != nil {
			return nil, fmt.Errorf("documents.secondary does not exist: %s", normalized)
		}
		if info.IsDir() {
			return nil, fmt.Errorf("documents.secondary %q must point to a markdown file", normalized)
		}
		normalizedSecondary = append(normalizedSecondary, normalized)
	}
	tracking.Documents.Secondary = normalizedSecondary

	normalizedScope := make([]string, 0, len(tracking.Scope))
	for _, scope := range tracking.Scope {
		normalized, err := normalizeExistingRepoDir(location.repoRoot, scope)
		if err != nil {
			return nil, fmt.Errorf("scope: %w", err)
		}
		normalizedScope = append(normalizedScope, normalized)
	}
	tracking.Scope = normalizedScope

	for i := range tracking.Requirements {
		if normalized, err := domain.NormalizeRequirementBlock(tracking.Requirements[i].Gherkin); err == nil {
			tracking.Requirements[i].Gherkin = normalized
		}
		normalizedTestFiles := make([]string, 0, len(tracking.Requirements[i].TestFiles))
		for _, testFile := range tracking.Requirements[i].TestFiles {
			normalized, err := domain.NormalizeRepoPath(testFile)
			if err != nil {
				return nil, fmt.Errorf("requirement %s test_files: %w", tracking.Requirements[i].ID, err)
			}
			if tracking.Requirements[i].EffectiveVerification() == domain.RequirementVerificationVerified && !tracking.Requirements[i].IsManual() {
				if _, err := os.Stat(filepath.Join(location.repoRoot, filepath.FromSlash(normalized))); err != nil {
					return nil, fmt.Errorf("requirement %s test file does not exist: %s", tracking.Requirements[i].ID, normalized)
				}
			}
			normalizedTestFiles = append(normalizedTestFiles, normalized)
		}
		tracking.Requirements[i].TestFiles = normalizedTestFiles
		if err := domain.ValidateRequirementTagsConfigured(tracking.Requirements[i].Tags, config.GherkinTags); err != nil {
			return nil, fmt.Errorf("requirement %s %s", tracking.Requirements[i].ID, err)
		}
	}

	if err := tracking.Validate(); err != nil {
		return nil, err
	}

	return &tracking, nil
}

func normalizeExistingRepoDir(repoRoot, value string) (string, error) {
	normalized, err := domain.NormalizeRepoDir(value)
	if err != nil {
		return "", err
	}

	info, err := os.Stat(filepath.Join(repoRoot, filepath.FromSlash(strings.TrimSuffix(normalized, "/"))))
	if err != nil {
		return "", fmt.Errorf("directory does not exist: %s", normalized)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("must point to a directory: %s", normalized)
	}

	return normalized, nil
}

func decodeYAMLNode(data []byte) (*yaml.Node, error) {
	var node yaml.Node
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	if err := decoder.Decode(&node); err != nil {
		return nil, err
	}
	return &node, nil
}

func strictUnmarshal(data []byte, target any) error {
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	decoder.KnownFields(true)
	return decoder.Decode(target)
}

func requireTrackingKeys(node *yaml.Node) error {
	if len(node.Content) == 0 {
		return fmt.Errorf("tracking file must contain a YAML document")
	}
	root := node.Content[0]
	for _, key := range []string{
		"slug", "charter", "title", "status", "rev", "created", "updated", "last_verified_at", "checkpoint",
		"tags", "documents", "scope", "deltas", "requirements", "changelog",
	} {
		if !mappingHasKey(root, key) {
			return fmt.Errorf("tracking file is missing required key %q", key)
		}
	}
	documentsNode := mappingValue(root, "documents")
	if documentsNode == nil || !mappingHasKey(documentsNode, "primary") {
		return fmt.Errorf("tracking file is missing required key \"documents.primary\"")
	}
	for _, deltaNode := range sequenceEntries(mappingValue(root, "deltas")) {
		if err := requireMappingKeys(deltaNode, []string{"id", "area", "status", "origin_checkpoint", "current", "target", "notes"}, "every delta"); err != nil {
			return err
		}
	}
	for _, requirementNode := range sequenceEntries(mappingValue(root, "requirements")) {
		if err := requireMappingKeys(requirementNode, []string{"id", "title", "tags", "test_files", "gherkin", "lifecycle", "verification", "introduced_by"}, "every requirement"); err != nil {
			return err
		}
	}
	for _, changelogNode := range sequenceEntries(mappingValue(root, "changelog")) {
		if err := requireMappingKeys(changelogNode, []string{"rev", "date", "deltas_opened", "deltas_closed", "reqs_added", "reqs_verified", "summary"}, "every changelog entry"); err != nil {
			return err
		}
	}
	return nil
}

type registryLocation struct {
	repoRoot  string
	charter   string
	slug      string
	isCharter bool
}

func resolveRegistryLocation(path string) (*registryLocation, error) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("resolving tracking path: %w", err)
	}
	normalized := filepath.ToSlash(absPath)
	parts := strings.Split(normalized, "/")
	for i, part := range parts {
		if part != ".specs" {
			continue
		}
		if len(parts) < i+3 {
			break
		}
		repoRoot := "/"
		if i > 0 {
			repoRoot = filepath.Join(parts[:i]...)
			if !strings.HasPrefix(repoRoot, "/") {
				repoRoot = "/" + repoRoot
			}
		}
		charter := parts[i+1]
		fileName := parts[i+2]
		if fileName == "CHARTER.yaml" {
			return &registryLocation{
				repoRoot:  repoRoot,
				charter:   charter,
				isCharter: true,
			}, nil
		}
		if filepath.Ext(fileName) != ".yaml" {
			return nil, fmt.Errorf("tracking file path must end with .yaml: %s", path)
		}
		return &registryLocation{
			repoRoot: repoRoot,
			charter:  charter,
			slug:     strings.TrimSuffix(fileName, ".yaml"),
		}, nil
	}
	return nil, fmt.Errorf("tracking file path must live under .specs/{charter}/")
}

func mappingHasKey(node *yaml.Node, key string) bool {
	return mappingValue(node, key) != nil
}

func mappingValue(node *yaml.Node, key string) *yaml.Node {
	if node == nil || node.Kind != yaml.MappingNode {
		return nil
	}
	for i := 0; i < len(node.Content)-1; i += 2 {
		if node.Content[i].Value == key {
			return node.Content[i+1]
		}
	}
	return nil
}

func sequenceEntries(node *yaml.Node) []*yaml.Node {
	if node == nil || node.Kind != yaml.SequenceNode {
		return nil
	}
	return node.Content
}

func requireMappingKeys(node *yaml.Node, keys []string, subject string) error {
	for _, key := range keys {
		if !mappingHasKey(node, key) {
			return fmt.Errorf("%s must include %q", subject, key)
		}
	}
	return nil
}

// FileExistsAt checks whether a repo-relative path exists on disk.
func FileExistsAt(repoRoot, repoRelPath string) bool {
	absPath := filepath.Join(repoRoot, filepath.FromSlash(repoRelPath))
	_, err := os.Stat(absPath)
	return err == nil
}
