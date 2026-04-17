package infrastructure

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/aitoroses/specctl/internal/domain"
	"gopkg.in/yaml.v3"
)

type CheckpointStore struct {
	workspace *Workspace
}

func NewCheckpointStore(workspace *Workspace) *CheckpointStore {
	return &CheckpointStore{workspace: workspace}
}

func (s *CheckpointStore) ResolveCheckpoint(ref string) (string, error) {
	return ResolveGitRevision(s.workspace.RepoRoot(), ref)
}

func (s *CheckpointStore) ReadGitFile(checkpoint, relativePath string) ([]byte, error) {
	return ReadGitFile(s.workspace.RepoRoot(), checkpoint, relativePath)
}

func (s *CheckpointStore) LoadTrackingAtRevision(charter, slug, checkpoint string) (*domain.TrackingFile, error) {
	data, err := s.ReadGitFile(checkpoint, s.workspace.TrackingRelativePath(charter, slug))
	if err != nil {
		return nil, err
	}

	var tracking domain.TrackingFile
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	decoder.KnownFields(true)
	if err := decoder.Decode(&tracking); err != nil {
		return nil, fmt.Errorf("decode tracking file at %s: %w", checkpoint, err)
	}
	return &tracking, nil
}

func (s *CheckpointStore) LoadSpecComparison(charter, slug string, current *domain.TrackingFile, checkpoint string) (SpecComparison, error) {
	baseline, err := s.LoadTrackingAtRevision(charter, slug, checkpoint)
	if err != nil {
		if !isGitFileMissingAtRevision(err) {
			return SpecComparison{}, err
		}
		baseline = nil
	}

	currentDoc, _, err := s.workspace.ReadRepoFileIfExists(current.Documents.Primary)
	if err != nil {
		return SpecComparison{}, err
	}

	baselineDocPath := current.Documents.Primary
	if baseline != nil && strings.TrimSpace(baseline.Documents.Primary) != "" {
		baselineDocPath = baseline.Documents.Primary
	}
	baselineDoc, baselineErr := s.ReadGitFile(checkpoint, baselineDocPath)
	baselineDocMissing := false
	if baselineErr != nil && isGitFileMissingAtRevision(baselineErr) {
		baselineDoc = nil
		baselineDocMissing = true
		baselineErr = nil
	}

	comparison := SpecComparison{
		BaselineTracking:   baseline,
		CurrentDoc:         currentDoc,
		BaselineDoc:        baselineDoc,
		NormalizedCurrent:  s.NormalizeDocumentForDiff(currentDoc),
		NormalizedBaseline: s.NormalizeDocumentForDiff(baselineDoc),
		BaselineDocMissing: baselineDocMissing,
		BaselineDocError:   baselineErr,
	}
	if baselineErr != nil {
		comparison.BaselineDoc = nil
		comparison.NormalizedBaseline = nil
	}
	return comparison, nil
}

func (s *CheckpointStore) NormalizeDocumentForDiff(data []byte) []byte {
	if len(data) == 0 {
		return nil
	}
	frontmatter, body, hasFrontmatter, err := SplitFrontmatterForDiff(data)
	if err != nil || !hasFrontmatter {
		return bytes.TrimSpace(data)
	}
	_ = frontmatter
	return bytes.TrimSpace(body)
}

func isGitFileMissingAtRevision(err error) bool {
	if err == nil {
		return false
	}
	message := err.Error()
	return strings.Contains(message, "exists on disk, but not in") || strings.Contains(message, "does not exist in")
}

var _ CheckpointAccess = (*CheckpointStore)(nil)
