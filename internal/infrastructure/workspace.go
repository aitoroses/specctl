package infrastructure

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/aitoroses/specctl/internal/domain"
)

type Workspace struct {
	repoRoot string
	specsDir string
}

type ScopeDriftSnapshot struct {
	Status                      string
	DriftSource                 string
	LastVerifiedAt              string
	TrackedBy                   []string
	FilesChangedSinceCheckpoint []string
	UncommittedChanges          []string
}

func OpenWorkspaceFromWorkingDir() (*Workspace, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("get working directory: %w", err)
	}
	repoRoot, err := FindRepoRoot(cwd)
	if err != nil {
		return nil, err
	}
	return NewWorkspace(repoRoot), nil
}

func NewWorkspace(repoRoot string) *Workspace {
	return &Workspace{
		repoRoot: repoRoot,
		specsDir: filepath.Join(repoRoot, ".specs"),
	}
}

func (w *Workspace) RepoRoot() string {
	return w.repoRoot
}

func (w *Workspace) SpecsDir() string {
	return w.specsDir
}

func (w *Workspace) TrackingPath(charter, slug string) string {
	return filepath.Join(w.specsDir, charter, slug+".yaml")
}

func (w *Workspace) CharterPath(charter string) string {
	return filepath.Join(w.specsDir, charter, "CHARTER.yaml")
}

func (w *Workspace) ConfigPath() string {
	return filepath.Join(w.specsDir, "specctl.yaml")
}

func (w *Workspace) TrackingRelativePath(charter, slug string) string {
	return RelativeTrackingPath(w.repoRoot, charter, slug)
}

func (w *Workspace) CharterRelativePath(charter string) string {
	return RelativeCharterPath(charter)
}

func (w *Workspace) TrackingExists(charter, slug string) (bool, error) {
	return pathExists(w.TrackingPath(charter, slug))
}

func (w *Workspace) CharterExists(charter string) (bool, error) {
	return pathExists(w.CharterPath(charter))
}

func (w *Workspace) NormalizeOwnershipPath(file string) (string, error) {
	return domain.NormalizeRepoPath(file)
}

func (w *Workspace) NormalizeDesignDocPath(value string) (string, error) {
	normalized, err := domain.NormalizeRepoPath(value)
	if err != nil {
		return "", err
	}
	if filepath.Ext(normalized) != ".md" {
		return "", fmt.Errorf("primary design document must be a markdown file")
	}
	return normalized, nil
}

func (w *Workspace) NormalizeScopePaths(values []string) ([]string, error) {
	normalized := make([]string, 0, len(values))
	for _, value := range values {
		dir, err := domain.NormalizeRepoDir(value)
		if err != nil {
			return nil, err
		}
		info, err := os.Stat(filepath.Join(w.repoRoot, filepath.FromSlash(strings.TrimSuffix(dir, "/"))))
		if err != nil {
			return nil, fmt.Errorf("scope directory does not exist: %s", dir)
		}
		if !info.IsDir() {
			return nil, fmt.Errorf("scope must point to a directory: %s", dir)
		}
		normalized = append(normalized, dir)
	}
	return uniqueOrdered(normalized), nil
}

func (w *Workspace) NormalizeVerifyFiles(values []string) ([]string, error) {
	normalized := make([]string, 0, len(values))
	for _, value := range values {
		path, err := domain.NormalizeRepoPath(value)
		if err != nil {
			return nil, &VerifyFilesNormalizationError{
				Code:    VerifyFilesInvalidPath,
				Message: err.Error(),
				Paths:   []string{value},
			}
		}
		info, err := os.Stat(filepath.Join(w.repoRoot, filepath.FromSlash(path)))
		if err != nil {
			return nil, &VerifyFilesNormalizationError{
				Code:    VerifyFilesMissing,
				Message: fmt.Sprintf("test file does not exist: %s", path),
				Paths:   []string{path},
			}
		}
		if info.IsDir() {
			return nil, &VerifyFilesNormalizationError{
				Code:    VerifyFilesMissing,
				Message: fmt.Sprintf("test file must point to a file: %s", path),
				Paths:   []string{path},
			}
		}
		normalized = append(normalized, path)
	}
	return uniqueOrdered(normalized), nil
}

func (w *Workspace) EnsureSourcePrefix(prefix string) (string, error) {
	normalized, err := domain.NormalizeRepoDir(prefix)
	if err != nil {
		return "", err
	}
	info, err := os.Stat(filepath.Join(w.repoRoot, filepath.FromSlash(strings.TrimSuffix(normalized, "/"))))
	if err != nil || !info.IsDir() {
		return "", fmt.Errorf("directory does not exist: %s", normalized)
	}
	return normalized, nil
}

func (w *Workspace) InspectCreateDesignDoc(docPath string, config *ProjectConfig) (*ExistingDesignDoc, error) {
	exists, err := pathExists(filepath.Join(w.repoRoot, filepath.FromSlash(docPath)))
	if err != nil {
		return nil, fmt.Errorf("checking design document path: %w", err)
	}
	if !exists {
		return nil, nil
	}

	doc, err := InspectDesignDoc(w.repoRoot, docPath)
	if err != nil {
		return nil, &SpecCreatePlanError{
			Code:    SpecCreatePrimaryDocFrontmatterError,
			Message: err.Error(),
			DocPath: docPath,
		}
	}
	if doc.Frontmatter != nil && doc.Frontmatter.Format != "" {
		if _, ok := config.Formats[doc.Frontmatter.Format]; !ok {
			return nil, &SpecCreatePlanError{
				Code:    SpecCreateFormatNotConfigured,
				Message: fmt.Sprintf("primary design document frontmatter references unknown format %q", doc.Frontmatter.Format),
				DocPath: docPath,
			}
		}
	}
	return doc, nil
}

func (w *Workspace) ReadRepoFile(relativePath string) ([]byte, error) {
	return os.ReadFile(filepath.Join(w.repoRoot, filepath.FromSlash(relativePath)))
}

func (w *Workspace) ReadRepoFileIfExists(relativePath string) ([]byte, bool, error) {
	path := filepath.Join(w.repoRoot, filepath.FromSlash(relativePath))
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	return data, true, nil
}

func (w *Workspace) SnapshotScopeDrift(tracking *domain.TrackingFile) (ScopeDriftSnapshot, []ValidationFinding) {
	drift := ScopeDriftSnapshot{
		Status:                      "clean",
		LastVerifiedAt:              tracking.LastVerifiedAt,
		TrackedBy:                   []string{},
		FilesChangedSinceCheckpoint: []string{},
		UncommittedChanges:          []string{},
	}

	if err := EnsureGitMetadata(w.repoRoot); err != nil {
		return unavailableScopeDrift(tracking), []ValidationFinding{{
			Code:     "CHECKPOINT_UNAVAILABLE",
			Severity: "warning",
			Message:  "git metadata is unavailable for scope drift detection",
			Path:     w.TrackingRelativePath(tracking.Charter, tracking.Slug),
			Target:   "checkpoint",
		}}
	}

	resolvedCheckpoint, err := ResolveGitRevision(w.repoRoot, tracking.Checkpoint)
	if err != nil {
		return unavailableScopeDrift(tracking), []ValidationFinding{{
			Code:     "CHECKPOINT_UNAVAILABLE",
			Severity: "warning",
			Message:  err.Error(),
			Path:     w.TrackingRelativePath(tracking.Charter, tracking.Slug),
			Target:   "checkpoint",
		}}
	}

	pathspecs := uniqueOrdered(append(append([]string{}, tracking.Scope...), tracking.Documents.Primary))
	changedFiles, err := GitDiffNameOnly(w.repoRoot, resolvedCheckpoint, "HEAD", pathspecs)
	if err != nil {
		return unavailableScopeDrift(tracking), []ValidationFinding{{
			Code:     "CHECKPOINT_UNAVAILABLE",
			Severity: "warning",
			Message:  err.Error(),
			Path:     w.TrackingRelativePath(tracking.Charter, tracking.Slug),
			Target:   "checkpoint",
		}}
	}
	drift.FilesChangedSinceCheckpoint = append([]string{}, changedFiles...)

	uncommittedChanges, err := GitStatusPorcelain(w.repoRoot, pathspecs)
	if err == nil {
		drift.UncommittedChanges = append([]string{}, uncommittedChanges...)
	}

	if len(changedFiles) == 0 {
		return drift, nil
	}

	docChanged := false
	codeChanged := false
	for _, file := range changedFiles {
		if file == tracking.Documents.Primary {
			docChanged = true
		} else {
			codeChanged = true
		}
	}
	drift.DriftSource = ClassifyDriftSource(docChanged, codeChanged)
	drift.TrackedBy = trackedCoverageDeltas(w.repoRoot, w.TrackingRelativePath(tracking.Charter, tracking.Slug), tracking, resolvedCheckpoint, changedFiles, pathspecs)
	if driftFullyCovered(changedFiles, drift.TrackedBy, w.repoRoot, w.TrackingRelativePath(tracking.Charter, tracking.Slug), tracking, resolvedCheckpoint, pathspecs) {
		drift.Status = "tracked"
		return drift, nil
	}
	drift.Status = "drifted"
	return drift, nil
}

func pathExists(path string) (bool, error) {
	_, err := os.Stat(path)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}

func uniqueOrdered(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}

func unavailableScopeDrift(tracking *domain.TrackingFile) ScopeDriftSnapshot {
	return ScopeDriftSnapshot{
		Status:                      "unavailable",
		LastVerifiedAt:              tracking.LastVerifiedAt,
		TrackedBy:                   []string{},
		FilesChangedSinceCheckpoint: []string{},
		UncommittedChanges:          []string{},
	}
}

func driftFullyCovered(changedFiles, trackedBy []string, repoRoot, trackingPath string, tracking *domain.TrackingFile, resolvedCheckpoint string, pathspecs []string) bool {
	if len(changedFiles) == 0 {
		return true
	}
	if len(trackedBy) == 0 {
		return false
	}
	currentChanged := stringSet(changedFiles)
	coveredFiles := make(map[string]struct{}, len(changedFiles))
	for _, deltaID := range trackedBy {
		delta := tracking.DeltaByID(deltaID)
		if delta == nil {
			continue
		}
		files := deltaCoverageFiles(repoRoot, trackingPath, tracking, *delta, resolvedCheckpoint, changedFiles, pathspecs)
		for _, file := range files {
			if _, changed := currentChanged[file]; !changed {
				continue
			}
			coveredFiles[file] = struct{}{}
		}
	}
	return len(coveredFiles) == len(currentChanged)
}

func trackedCoverageDeltas(repoRoot, trackingPath string, tracking *domain.TrackingFile, resolvedCheckpoint string, changedFiles, pathspecs []string) []string {
	currentChanged := stringSet(changedFiles)
	eligible := make([]string, 0, len(tracking.Deltas))
	for _, delta := range tracking.Deltas {
		files := deltaCoverageFiles(repoRoot, trackingPath, tracking, delta, resolvedCheckpoint, changedFiles, pathspecs)
		if len(files) == 0 {
			continue
		}
		for _, file := range files {
			if _, changed := currentChanged[file]; !changed {
				continue
			}
			eligible = append(eligible, delta.ID)
			break
		}
	}
	return uniqueOrdered(eligible)
}

func deltaCoverageFiles(repoRoot, trackingPath string, tracking *domain.TrackingFile, delta domain.Delta, resolvedCheckpoint string, changedFiles, pathspecs []string) []string {
	if delta.Status == domain.DeltaStatusDeferred || delta.Status == domain.DeltaStatusClosed || delta.Status == domain.DeltaStatusWithdrawn {
		return nil
	}
	resolvedOrigin, err := ResolveGitRevision(repoRoot, delta.OriginCheckpoint)
	if err != nil || resolvedOrigin != resolvedCheckpoint {
		return nil
	}
	introductionCommit, err := deltaIntroductionCommit(repoRoot, trackingPath, tracking, delta)
	if err != nil {
		return nil
	}
	if introductionCommit == "" {
		return append([]string{}, changedFiles...)
	}
	files, err := GitDiffNameOnly(repoRoot, resolvedCheckpoint, introductionCommit, pathspecs)
	if err != nil {
		return nil
	}
	return uniqueOrdered(files)
}

func deltaIntroductionCommit(repoRoot, trackingPath string, tracking *domain.TrackingFile, delta domain.Delta) (string, error) {
	commits, err := GitLogCommitsForPath(repoRoot, trackingPath)
	if err != nil {
		return "", err
	}
	marker := "id: " + delta.ID
	for _, commit := range commits {
		data, readErr := ReadGitFile(repoRoot, commit, trackingPath)
		if readErr != nil {
			continue
		}
		if strings.Contains(string(data), marker) {
			return commit, nil
		}
	}
	if tracking.DeltaByID(delta.ID) != nil {
		return "", nil
	}
	return "", nil
}

func stringSet(values []string) map[string]struct{} {
	set := make(map[string]struct{}, len(values))
	for _, value := range values {
		set[value] = struct{}{}
	}
	return set
}
