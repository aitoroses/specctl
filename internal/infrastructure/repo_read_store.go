package infrastructure

import (
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/aitoroses/specctl/internal/domain"
)

type RepoReadStore struct {
	workspace *Workspace
}

func NewRepoReadStore(workspace *Workspace) *RepoReadStore {
	return &RepoReadStore{workspace: workspace}
}

func (s *RepoReadStore) LoadRepoReadSnapshot() (*RepoReadSnapshot, error) {
	return s.loadRepoReadSnapshotWithOverlay(nil)
}

func (s *RepoReadStore) LoadRepoReadSnapshotWithWrites(writes []PlannedWrite) (*RepoReadSnapshot, error) {
	overlay := make(map[string][]byte, len(writes))
	for _, write := range writes {
		overlay[filepath.Clean(write.Path)] = append([]byte{}, write.Data...)
	}
	return s.loadRepoReadSnapshotWithOverlay(overlay)
}

func (s *RepoReadStore) loadRepoReadSnapshotWithOverlay(overlay map[string][]byte) (*RepoReadSnapshot, error) {
	configData, exists, err := s.readSpecsFile("specctl.yaml", overlay)
	if err != nil {
		return nil, err
	}
	config, configFindings, err := LoadProjectConfigLenient(s.workspace.SpecsDir())
	if overlay != nil {
		switch {
		case err != nil && !exists:
			return nil, err
		case exists:
			config, configFindings, err = loadProjectConfigLenientFromData(s.workspace.ConfigPath(), configData, false)
			if err != nil {
				return nil, err
			}
		}
	} else if err != nil {
		return nil, err
	}

	snapshot := &RepoReadSnapshot{
		Config:             config,
		ConfigReadFindings: append([]ValidationFinding{}, configFindings...),
		Trackings:          map[string]RepoTrackingSnapshot{},
		Charters:           map[string]RepoCharterSnapshot{},
	}

	trackingPaths, err := findTrackingPathsWithOverlay(s.workspace.SpecsDir(), overlay)
	if err != nil {
		return nil, err
	}
	overlayRepoFiles := repoFileOverlayMap(s.workspace.RepoRoot(), overlay)
	for _, trackingPath := range trackingPaths {
		trackingData, trackingExists, readErr := readFileWithOverlay(trackingPath, overlay)
		if readErr != nil {
			return nil, readErr
		}
		if !trackingExists {
			continue
		}
		tracking, findings, readErr := readTrackingFileLenientWithConfigData(trackingPath, trackingData, config, overlayRepoFiles)
		if readErr != nil {
			return nil, readErr
		}
		key := tracking.Charter + ":" + tracking.Slug
		snapshot.Trackings[key] = RepoTrackingSnapshot{
			Tracking: tracking,
			Findings: append([]ValidationFinding{}, findings...),
		}
	}

	charterPaths, err := findCharterPathsWithOverlay(s.workspace.SpecsDir(), overlay)
	if err != nil {
		return nil, err
	}
	for _, charterPath := range charterPaths {
		charterData, charterExists, readErr := readFileWithOverlay(charterPath, overlay)
		if readErr != nil {
			return nil, readErr
		}
		if !charterExists {
			continue
		}
		charter, findings, readErr := readCharterLenientFromData(charterPath, charterData)
		if readErr != nil {
			return nil, readErr
		}
		snapshot.Charters[charter.Name] = RepoCharterSnapshot{
			Charter:  charter,
			Findings: append([]ValidationFinding{}, findings...),
		}
	}

	return snapshot, nil
}

func (s *RepoReadStore) readSpecsFile(name string, overlay map[string][]byte) ([]byte, bool, error) {
	return readFileWithOverlay(filepath.Join(s.workspace.SpecsDir(), name), overlay)
}

func (s *RepoReadStore) ResolveSpecProjectionInputs(tracking *domain.TrackingFile, config *ProjectConfig) SpecProjectionInputs {
	inputs := SpecProjectionInputs{}
	if tracking == nil {
		return inputs
	}

	inputs.ScopeDrift, inputs.ScopeDriftFindings = s.workspace.SnapshotScopeDrift(tracking)
	if config == nil || tracking.Documents.Primary == "" {
		return inputs
	}

	contexts, err := ReadRequirementContexts(s.workspace.RepoRoot(), tracking.Documents.Primary)
	if err == nil {
		inputs.Requirements = MatchRequirementContexts(tracking.Requirements, contexts)
		inputs.OrphanGherkinBlocks = orphanGherkinBlocks(tracking.Requirements, contexts)
	}

	frontmatter, err := ReadDesignDocFrontmatterWithConfig(s.workspace.RepoRoot(), tracking.Documents.Primary, config)
	if err != nil || frontmatter.Format == "" {
		return inputs
	}

	format := frontmatter.Format
	formatConfig, exists := config.Formats[format]
	if !exists {
		return inputs
	}
	template := formatConfig.Template
	inputs.DesignDoc = &ResolvedDesignDocMetadata{
		Format:         &format,
		FormatTemplate: &template,
	}
	return inputs
}

var _ RepoReadAccess = (*RepoReadStore)(nil)

func findTrackingPathsWithOverlay(specsDir string, overlay map[string][]byte) ([]string, error) {
	paths, err := FindAllTrackingFiles(specsDir)
	if err != nil {
		return nil, err
	}
	if len(overlay) == 0 {
		return paths, nil
	}

	seen := make(map[string]struct{}, len(paths))
	for _, path := range paths {
		seen[filepath.Clean(path)] = struct{}{}
	}
	for path := range overlay {
		cleaned := filepath.Clean(path)
		if filepath.Ext(cleaned) != ".yaml" || filepath.Base(cleaned) == "CHARTER.yaml" {
			continue
		}
		if filepath.Dir(filepath.Dir(cleaned)) != filepath.Clean(specsDir) {
			continue
		}
		if _, exists := seen[cleaned]; exists {
			continue
		}
		paths = append(paths, cleaned)
	}
	sort.Strings(paths)
	return paths, nil
}

func findCharterPathsWithOverlay(specsDir string, overlay map[string][]byte) ([]string, error) {
	paths, err := FindAllCharters(specsDir)
	if err != nil {
		return nil, err
	}
	if len(overlay) == 0 {
		return paths, nil
	}

	seen := make(map[string]struct{}, len(paths))
	for _, path := range paths {
		seen[filepath.Clean(path)] = struct{}{}
	}
	for path := range overlay {
		cleaned := filepath.Clean(path)
		if filepath.Base(cleaned) != "CHARTER.yaml" {
			continue
		}
		if filepath.Dir(filepath.Dir(cleaned)) != filepath.Clean(specsDir) {
			continue
		}
		if _, exists := seen[cleaned]; exists {
			continue
		}
		paths = append(paths, cleaned)
	}
	sort.Strings(paths)
	return paths, nil
}

func readFileWithOverlay(path string, overlay map[string][]byte) ([]byte, bool, error) {
	cleaned := filepath.Clean(path)
	if data, exists := overlay[cleaned]; exists {
		return append([]byte{}, data...), true, nil
	}
	data, err := os.ReadFile(cleaned)
	if os.IsNotExist(err) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	return data, true, nil
}

func repoFileOverlayMap(repoRoot string, overlay map[string][]byte) map[string][]byte {
	if len(overlay) == 0 {
		return nil
	}
	files := make(map[string][]byte)
	for absPath, data := range overlay {
		rel, err := filepath.Rel(repoRoot, absPath)
		if err != nil {
			continue
		}
		rel = filepath.ToSlash(rel)
		if strings.HasPrefix(rel, "../") || rel == ".." {
			continue
		}
		files[rel] = append([]byte{}, data...)
	}
	if len(files) == 0 {
		return nil
	}
	return files
}
