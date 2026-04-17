package infrastructure

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/bmatcuk/doublestar/v4"
	"gopkg.in/yaml.v3"
)

type DesignDocMutation struct {
	Action         string
	SelectedFormat *string
	Content        []byte
}

type ExistingDesignDoc struct {
	Path        string
	Content     []byte
	Frontmatter *DesignDocFrontmatter
	Body        []byte
}

func InspectDesignDoc(repoRoot, relativePath string) (*ExistingDesignDoc, error) {
	normalizedPath, err := filepath.Abs(filepath.Join(repoRoot, filepath.FromSlash(relativePath)))
	if err != nil {
		return nil, fmt.Errorf("resolve design document path: %w", err)
	}

	data, err := os.ReadFile(normalizedPath)
	if err != nil {
		return nil, err
	}

	doc := &ExistingDesignDoc{
		Path:    normalizedPath,
		Content: data,
	}

	frontmatterBytes, body, hasFrontmatter, err := SplitFrontmatterForDiff(data)
	if err != nil {
		return doc, err
	}
	if !hasFrontmatter {
		return doc, nil
	}

	frontmatter, err := parseFrontmatter(frontmatterBytes)
	if err != nil {
		return doc, err
	}

	doc.Frontmatter = frontmatter
	doc.Body = body
	return doc, nil
}

func BuildDesignDocMutation(docPath string, existing *ExistingDesignDoc, slug, charter string, selectedFormat *string) (DesignDocMutation, error) {
	frontmatterBlock := encodeFrontmatter(slug, charter, selectedFormat)

	if existing == nil {
		return DesignDocMutation{
			Action:         "bootstrapped",
			SelectedFormat: cloneOptionalString(selectedFormat),
			Content:        frontmatterBlock,
		}, nil
	}

	if existing.Frontmatter == nil {
		content := append([]byte{}, frontmatterBlock...)
		content = append(content, existing.Content...)
		return DesignDocMutation{
			Action:         "prepended_frontmatter",
			SelectedFormat: cloneOptionalString(selectedFormat),
			Content:        content,
		}, nil
	}

	if existing.Frontmatter.Spec != slug {
		return DesignDocMutation{}, &SpecCreatePlanError{
			Code:    SpecCreatePrimaryDocMismatch,
			Message: fmt.Sprintf("primary design document frontmatter spec %q does not match tracking slug %q", existing.Frontmatter.Spec, slug),
			DocPath: docPath,
		}
	}
	if existing.Frontmatter.Charter != charter {
		return DesignDocMutation{}, &SpecCreatePlanError{
			Code:    SpecCreatePrimaryDocMismatch,
			Message: fmt.Sprintf("primary design document frontmatter charter %q does not match tracking charter %q", existing.Frontmatter.Charter, charter),
			DocPath: docPath,
		}
	}

	effectiveFormat := selectedFormat
	if existing.Frontmatter.Format != "" {
		format := existing.Frontmatter.Format
		effectiveFormat = &format
	}

	if !optionalStringsEqual(effectiveFormat, stringPointerOrNil(existing.Frontmatter.Format)) {
		content := append([]byte{}, encodeFrontmatter(slug, charter, effectiveFormat)...)
		content = append(content, existing.Body...)
		return DesignDocMutation{
			Action:         "rewritten_frontmatter",
			SelectedFormat: cloneOptionalString(effectiveFormat),
			Content:        content,
		}, nil
	}

	return DesignDocMutation{
		Action:         "validated_existing",
		SelectedFormat: cloneOptionalString(effectiveFormat),
		Content:        append([]byte{}, existing.Content...),
	}, nil
}

func AutoSelectFormat(config *ProjectConfig, relativePath string) (*string, error) {
	if config == nil {
		return nil, fmt.Errorf("specctl config is required")
	}

	normalized := filepath.ToSlash(relativePath)
	keys := make([]string, 0, len(config.Formats))
	for key := range config.Formats {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	matches := make([]string, 0)
	for _, key := range keys {
		format := config.Formats[key]
		if globMatches(format.RecommendedFor, normalized) {
			matches = append(matches, key)
		}
	}
	if len(matches) == 0 {
		return nil, nil
	}
	if len(matches) > 1 {
		return nil, &SpecCreatePlanError{
			Code:    SpecCreateFormatAmbiguous,
			Message: fmt.Sprintf("multiple configured formats match %s", normalized),
			DocPath: normalized,
		}
	}

	selected := matches[0]
	return &selected, nil
}

func SplitFrontmatterForDiff(data []byte) ([]byte, []byte, bool, error) {
	content := strings.ReplaceAll(string(data), "\r\n", "\n")
	if !strings.HasPrefix(content, "---\n") {
		return nil, []byte(content), false, nil
	}

	lines := strings.Split(content, "\n")
	frontmatterLines := make([]string, 0, len(lines))
	for i, line := range lines[1:] {
		if line == "---" {
			body := strings.Join(lines[i+2:], "\n")
			return []byte(strings.Join(frontmatterLines, "\n")), []byte(body), true, nil
		}
		frontmatterLines = append(frontmatterLines, line)
	}

	return nil, nil, true, fmt.Errorf("primary design document frontmatter is invalid: missing closing delimiter")
}

func parseFrontmatter(frontmatterBytes []byte) (*DesignDocFrontmatter, error) {
	var frontmatter DesignDocFrontmatter
	decoder := yaml.NewDecoder(strings.NewReader(string(frontmatterBytes)))
	if err := decoder.Decode(&frontmatter); err != nil {
		return nil, fmt.Errorf("primary design document frontmatter is invalid: %w", err)
	}
	if strings.TrimSpace(frontmatter.Spec) == "" {
		return nil, fmt.Errorf("primary design document frontmatter must include spec")
	}
	if strings.TrimSpace(frontmatter.Charter) == "" {
		return nil, fmt.Errorf("primary design document frontmatter must include charter")
	}
	return &frontmatter, nil
}

func encodeFrontmatter(slug, charter string, format *string) []byte {
	var builder strings.Builder
	builder.WriteString("---\n")
	builder.WriteString("spec: ")
	builder.WriteString(slug)
	builder.WriteString("\n")
	builder.WriteString("charter: ")
	builder.WriteString(charter)
	builder.WriteString("\n")
	if format != nil && strings.TrimSpace(*format) != "" {
		builder.WriteString("format: ")
		builder.WriteString(strings.TrimSpace(*format))
		builder.WriteString("\n")
	}
	builder.WriteString("---\n")
	return []byte(builder.String())
}

func globMatches(pattern, candidate string) bool {
	matched, err := doublestar.Match(pattern, candidate)
	return err == nil && matched
}

func cloneOptionalString(value *string) *string {
	if value == nil {
		return nil
	}
	cloned := *value
	return &cloned
}

func optionalStringsEqual(left, right *string) bool {
	switch {
	case left == nil && right == nil:
		return true
	case left == nil || right == nil:
		return false
	default:
		return *left == *right
	}
}

func stringPointerOrNil(value string) *string {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	cloned := value
	return &cloned
}
