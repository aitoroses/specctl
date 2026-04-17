package infrastructure

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/aitoroses/specctl/internal/domain"
	"gopkg.in/yaml.v3"
)

func ReadCharter(path string) (*domain.Charter, error) {
	charter, err := ReadCharterStructure(path)
	if err != nil {
		return nil, err
	}

	entries, err := os.ReadDir(charter.DirPath)
	if err != nil {
		return nil, fmt.Errorf("reading charter directory: %w", err)
	}
	trackingSlugs := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".yaml" || entry.Name() == "CHARTER.yaml" {
			continue
		}
		trackingSlugs = append(trackingSlugs, entry.Name()[:len(entry.Name())-len(".yaml")])
	}

	if missing := charter.MissingTrackingSpecs(trackingSlugs); len(missing) > 0 {
		trackingPath := filepath.Join(charter.DirPath, missing[0]+".yaml")
		return nil, fmt.Errorf("charter spec %q does not have a tracking file at %s", missing[0], trackingPath)
	}
	for _, spec := range charter.Specs {
		trackingPath := filepath.Join(charter.DirPath, spec.Slug+".yaml")
		if _, err := ReadTrackingFile(trackingPath); err != nil {
			return nil, fmt.Errorf("charter spec %q tracking file is invalid: %w", spec.Slug, err)
		}
	}

	return charter, nil
}

func ReadCharterStructure(path string) (*domain.Charter, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading charter file: %w", err)
	}

	location, err := resolveRegistryLocation(path)
	if err != nil {
		return nil, err
	}
	if !location.isCharter {
		return nil, fmt.Errorf("charter path must point to .specs/{charter}/CHARTER.yaml")
	}

	node, err := decodeYAMLNode(data)
	if err != nil {
		return nil, fmt.Errorf("parsing charter YAML: %w", err)
	}
	if err := requireCharterKeys(node); err != nil {
		return nil, err
	}

	var charter domain.Charter
	if err := strictUnmarshal(data, &charter); err != nil {
		return nil, fmt.Errorf("parsing charter YAML: %w", err)
	}
	charter.DirPath = filepath.Dir(path)

	if charter.Name != location.charter {
		return nil, fmt.Errorf("charter name %q does not match directory %q", charter.Name, location.charter)
	}
	if err := charter.Validate(); err != nil {
		return nil, err
	}
	return &charter, nil
}

func ReadCharterLenient(path string) (*domain.Charter, []ValidationFinding, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, nil, fmt.Errorf("reading charter file: %w", err)
	}

	return readCharterLenientFromData(path, data)
}

func readCharterLenientFromData(path string, data []byte) (*domain.Charter, []ValidationFinding, error) {
	location, err := resolveRegistryLocation(path)
	if err != nil {
		return nil, nil, err
	}
	if !location.isCharter {
		return nil, nil, fmt.Errorf("charter path must point to .specs/{charter}/CHARTER.yaml")
	}

	relativePath := RelativeCharterPath(location.charter)
	findings := make([]ValidationFinding, 0)

	node, err := decodeYAMLNode(data)
	if err != nil {
		findings = append(findings, ValidationFinding{
			Code:     "CHARTER_NAME_INVALID",
			Severity: "error",
			Message:  fmt.Sprintf("parsing charter YAML: %v", err),
			Path:     relativePath,
		})
		charter := &domain.Charter{
			Name:    location.charter,
			DirPath: filepath.Dir(path),
		}
		return charter, findings, nil
	}
	if err := requireCharterKeys(node); err != nil {
		findings = append(findings, ValidationFindingsFromMessages(err.Error(), relativePath, "")...)
	}

	var charter domain.Charter
	if err := yaml.Unmarshal(data, &charter); err != nil {
		findings = append(findings, ValidationFinding{
			Code:     "CHARTER_NAME_INVALID",
			Severity: "error",
			Message:  fmt.Sprintf("parsing charter YAML: %v", err),
			Path:     relativePath,
		})
	}
	charter.DirPath = filepath.Dir(path)
	rawName := charter.Name
	if err := charter.Validate(); err != nil {
		findings = append(findings, ValidationFindingsFromMessages(err.Error(), relativePath, "")...)
	}
	if rawName != "" && rawName != location.charter {
		findings = append(findings, ValidationFindingFromError(fmt.Errorf("charter name %q does not match directory %q", rawName, location.charter), relativePath, "name"))
	}
	if charter.Name == "" || charter.Name != location.charter {
		charter.Name = location.charter
	}

	return &charter, uniqueFindings(findings), nil
}

func FindCharter(specsDir, name string) (string, error) {
	path := filepath.Join(specsDir, name, "CHARTER.yaml")
	if _, err := os.Stat(path); err != nil {
		return "", fmt.Errorf("charter not found at %s: %w", path, err)
	}
	return path, nil
}

func FindAllCharters(specsDir string) ([]string, error) {
	entries, err := os.ReadDir(specsDir)
	if err != nil {
		return nil, fmt.Errorf("reading specs directory: %w", err)
	}

	paths := make([]string, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		candidate := filepath.Join(specsDir, entry.Name(), "CHARTER.yaml")
		if _, err := os.Stat(candidate); err == nil {
			paths = append(paths, candidate)
		}
	}
	sort.Strings(paths)

	return paths, nil
}

func requireCharterKeys(node *yaml.Node) error {
	if len(node.Content) == 0 {
		return fmt.Errorf("charter file must contain a YAML document")
	}
	root := node.Content[0]
	for _, key := range []string{"name", "title", "description", "groups", "specs"} {
		if !mappingHasKey(root, key) {
			return fmt.Errorf("charter file is missing required key %q", key)
		}
	}
	for _, groupNode := range sequenceEntries(mappingValue(root, "groups")) {
		for _, key := range []string{"key", "title", "order"} {
			if !mappingHasKey(groupNode, key) {
				return fmt.Errorf("every charter group must include %q", key)
			}
		}
	}
	for _, specNode := range sequenceEntries(mappingValue(root, "specs")) {
		for _, key := range []string{"slug", "group", "order", "depends_on", "notes"} {
			if !mappingHasKey(specNode, key) {
				return fmt.Errorf("every charter spec must include %q", key)
			}
		}
	}
	return nil
}
