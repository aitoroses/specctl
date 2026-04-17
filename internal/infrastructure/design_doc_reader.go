package infrastructure

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/aitoroses/specctl/internal/domain"
	"gopkg.in/yaml.v3"
)

type DesignDocFrontmatter struct {
	Spec    string `yaml:"spec"`
	Charter string `yaml:"charter"`
	Format  string `yaml:"format"`
}

type RequirementContext struct {
	Title     string
	Heading   string
	Gherkin   string
	Scenarios []string
}

var (
	requirementHeadingRe = regexp.MustCompile(`(?m)^##\s+Requirement:\s+(.+?)\s*$`)
	requirementFenceRe   = regexp.MustCompile("(?s)^```gherkin requirement[^\\n]*\\n(.*?)\\n```")
	scenarioFenceRe      = regexp.MustCompile("(?s)```gherkin scenario[^\\n]*\\n(.*?)\\n```")
	scenarioHeadingRe    = regexp.MustCompile(`(?m)^\s*Scenario:\s+(.+?)\s*$`)
)

func ReadDesignDocFrontmatter(repoRoot, relativePath string) (*DesignDocFrontmatter, error) {
	config, err := LoadProjectConfig(filepath.Join(repoRoot, ".specs"))
	if err != nil {
		return nil, fmt.Errorf("loading specctl config: %w", err)
	}
	return ReadDesignDocFrontmatterWithConfig(repoRoot, relativePath, config)
}

func ReadDesignDocFrontmatterWithConfig(repoRoot, relativePath string, config *ProjectConfig) (*DesignDocFrontmatter, error) {
	data, err := readRepoFileWithNormalization(repoRoot, relativePath)
	if err != nil {
		return nil, err
	}
	return readDesignDocFrontmatterWithConfigData(relativePath, data, config)
}

func readDesignDocFrontmatterWithConfigData(relativePath string, data []byte, config *ProjectConfig) (*DesignDocFrontmatter, error) {
	if config == nil {
		return nil, fmt.Errorf("specctl config is required")
	}
	if _, err := domain.NormalizeRepoPath(relativePath); err != nil {
		return nil, fmt.Errorf("primary design document path: %w", err)
	}

	frontmatterBytes, err := extractFrontmatter(data)
	if err != nil {
		return nil, err
	}

	var frontmatter DesignDocFrontmatter
	decoder := yaml.NewDecoder(bytes.NewReader(frontmatterBytes))
	decoder.KnownFields(true)
	if err := decoder.Decode(&frontmatter); err != nil {
		return nil, fmt.Errorf("primary design document frontmatter is invalid: %w", err)
	}
	if strings.TrimSpace(frontmatter.Spec) == "" {
		return nil, fmt.Errorf("primary design document frontmatter must include spec")
	}
	if strings.TrimSpace(frontmatter.Charter) == "" {
		return nil, fmt.Errorf("primary design document frontmatter must include charter")
	}
	if frontmatter.Format != "" {
		if _, exists := config.Formats[frontmatter.Format]; !exists {
			return nil, fmt.Errorf("primary design document frontmatter references unknown format %q", frontmatter.Format)
		}
	}

	return &frontmatter, nil
}

func ValidateTrackedDesignDocument(repoRoot, relativePath, charter, slug string) error {
	config, err := LoadProjectConfig(filepath.Join(repoRoot, ".specs"))
	if err != nil {
		return fmt.Errorf("loading specctl config: %w", err)
	}
	return ValidateTrackedDesignDocumentWithConfig(repoRoot, relativePath, charter, slug, config)
}

func ValidateTrackedDesignDocumentWithConfig(repoRoot, relativePath, charter, slug string, config *ProjectConfig) error {
	data, err := readRepoFileWithNormalization(repoRoot, relativePath)
	if err != nil {
		return err
	}
	return validateTrackedDesignDocumentContentWithConfig(relativePath, charter, slug, config, data)
}

func validateTrackedDesignDocumentContentWithConfig(relativePath, charter, slug string, config *ProjectConfig, data []byte) error {
	frontmatter, err := readDesignDocFrontmatterWithConfigData(relativePath, data, config)
	if err != nil {
		return err
	}
	if frontmatter.Spec != slug {
		return fmt.Errorf("primary design document frontmatter spec %q does not match tracking slug %q", frontmatter.Spec, slug)
	}
	if frontmatter.Charter != charter {
		return fmt.Errorf("primary design document frontmatter charter %q does not match tracking charter %q", frontmatter.Charter, charter)
	}
	return nil
}

func readRepoFileWithNormalization(repoRoot, relativePath string) ([]byte, error) {
	normalizedPath, err := domain.NormalizeRepoPath(relativePath)
	if err != nil {
		return nil, fmt.Errorf("primary design document path: %w", err)
	}

	data, err := os.ReadFile(filepath.Join(repoRoot, filepath.FromSlash(normalizedPath)))
	if err != nil {
		return nil, fmt.Errorf("reading primary design document: %w", err)
	}
	return data, nil
}

func extractFrontmatter(data []byte) ([]byte, error) {
	content := strings.ReplaceAll(string(data), "\r\n", "\n")
	if !strings.HasPrefix(content, "---\n") {
		return nil, fmt.Errorf("primary design document frontmatter is missing")
	}

	lines := strings.Split(content, "\n")
	frontmatterLines := make([]string, 0, len(lines))
	for _, line := range lines[1:] {
		if line == "---" {
			return []byte(strings.Join(frontmatterLines, "\n")), nil
		}
		frontmatterLines = append(frontmatterLines, line)
	}

	return nil, fmt.Errorf("primary design document frontmatter is invalid: missing closing delimiter")
}

func ReadRequirementContexts(repoRoot, relativePath string) ([]RequirementContext, error) {
	data, err := readRepoFileWithNormalization(repoRoot, relativePath)
	if err != nil {
		return nil, err
	}
	return parseRequirementContexts(data)
}

func parseRequirementContexts(data []byte) ([]RequirementContext, error) {
	content := strings.ReplaceAll(string(data), "\r\n", "\n")
	if strings.HasPrefix(content, "---\n") {
		lines := strings.Split(content, "\n")
		for i, line := range lines[1:] {
			if line == "---" {
				content = strings.Join(lines[i+2:], "\n")
				break
			}
		}
	}

	matches := requirementHeadingRe.FindAllStringSubmatchIndex(content, -1)
	contexts := make([]RequirementContext, 0, len(matches))
	for i, match := range matches {
		title := strings.TrimSpace(content[match[2]:match[3]])
		start := match[1]
		end := len(content)
		if i+1 < len(matches) {
			end = matches[i+1][0]
		}
		block := content[start:end]
		requirementBlock, _ := extractRequirementBlock(block)
		contexts = append(contexts, RequirementContext{
			Title:     title,
			Heading:   "Requirement: " + title,
			Gherkin:   requirementBlock,
			Scenarios: scenarioTitlesFromScenarioBlocks(block),
		})
	}

	return contexts, nil
}

func MatchRequirementContexts(requirements []domain.Requirement, contexts []RequirementContext) map[string]RequirementDocContext {
	matches := make(map[string]RequirementDocContext, len(requirements))
	for _, requirement := range requirements {
		match := RequirementDocContext{MatchStatus: "missing_in_spec", Scenarios: []string{}}
		exactMatches := make([]RequirementContext, 0)
		titleMatches := make([]RequirementContext, 0)
		for _, context := range contexts {
			if context.Gherkin == requirement.Gherkin {
				exactMatches = append(exactMatches, context)
			}
			if context.Title == requirement.Title {
				titleMatches = append(titleMatches, context)
			}
		}

		switch {
		case len(exactMatches) == 1:
			match.MatchStatus = "matched"
			match.Heading = exactMatches[0].Heading
			match.Scenarios = append([]string{}, exactMatches[0].Scenarios...)
		case len(exactMatches) > 1:
			match.MatchStatus = "duplicate_in_spec"
			match.Heading = exactMatches[0].Heading
			match.Scenarios = append([]string{}, exactMatches[0].Scenarios...)
		case len(titleMatches) == 1:
			match.MatchStatus = "no_exact_match"
			match.Heading = titleMatches[0].Heading
			match.Scenarios = append([]string{}, titleMatches[0].Scenarios...)
		case len(titleMatches) > 1:
			match.MatchStatus = "duplicate_in_spec"
			match.Heading = titleMatches[0].Heading
			match.Scenarios = append([]string{}, titleMatches[0].Scenarios...)
		}
		matches[requirement.ID] = match
	}
	return matches
}

func extractRequirementBlock(section string) (string, bool) {
	headingEnd := strings.Index(section, "\n")
	if headingEnd < 0 {
		return "", false
	}
	trimmed := strings.TrimLeft(section[headingEnd+1:], "\n\t ")
	matches := requirementFenceRe.FindStringSubmatch(trimmed)
	if len(matches) != 2 {
		return "", false
	}
	normalized, err := domain.NormalizeRequirementBlock(matches[1])
	if err != nil {
		return "", false
	}
	return normalized, true
}

func scenarioTitlesFromScenarioBlocks(section string) []string {
	blockMatches := scenarioFenceRe.FindAllStringSubmatch(section, -1)
	titles := make([]string, 0, len(blockMatches))
	for _, blockMatch := range blockMatches {
		titles = append(titles, scenarioTitlesFromGherkin(blockMatch[1])...)
	}
	return titles
}

func scenarioTitlesFromGherkin(gherkin string) []string {
	matches := scenarioHeadingRe.FindAllStringSubmatch(strings.ReplaceAll(gherkin, "\r\n", "\n"), -1)
	titles := make([]string, 0, len(matches))
	for _, match := range matches {
		titles = append(titles, strings.TrimSpace(match[1]))
	}
	return titles
}
