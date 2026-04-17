package infrastructure

import (
	"fmt"
	"path/filepath"

	"github.com/aitoroses/specctl/internal/domain"
	"gopkg.in/yaml.v3"
)

type RegistryStore struct {
	projectConfig  *projectConfigRegistry
	charters       *charterRegistry
	charterWrites  *charterMutationRegistry
	charterEntries *charterEntryMutationRegistry
	trackings      *trackingRegistry
	configWrites   *configMutationRegistry
	specCreate     *specCreatePlanner
}

func NewRegistryStore(workspace *Workspace) *RegistryStore {
	return &RegistryStore{
		projectConfig:  newProjectConfigRegistry(workspace),
		charters:       newCharterRegistry(workspace),
		charterWrites:  newCharterMutationRegistry(workspace),
		charterEntries: newCharterEntryMutationRegistry(workspace),
		trackings:      newTrackingRegistry(workspace),
		configWrites:   newConfigMutationRegistry(workspace),
		specCreate:     newSpecCreatePlanner(workspace),
	}
}

func (s *RegistryStore) LoadProjectConfig() (*ProjectConfig, error) {
	return s.projectConfig.Load()
}

func (s *RegistryStore) LoadTracking(charter, slug string) (*domain.TrackingFile, error) {
	return s.trackings.Load(charter, slug)
}

func (s *RegistryStore) LoadTrackingLenient(charter, slug string) (*domain.TrackingFile, []ValidationFinding, error) {
	return s.trackings.LoadLenient(charter, slug)
}

func (s *RegistryStore) LoadCharterStructure(charter string) (*domain.Charter, error) {
	return s.charters.LoadStructure(charter)
}

func (s *RegistryStore) ApplyCharterMutation(charter *domain.Charter) (*CharterMutationResult, error) {
	return s.charterWrites.Apply(charter)
}

func (s *RegistryStore) ApplyCharterEntryMutation(request CharterEntryMutationRequest) (*CharterEntryMutationResult, error) {
	return s.charterEntries.Apply(request)
}

func (s *RegistryStore) ApplyConfigMutation(request ConfigMutationRequest) (*ConfigMutationResult, error) {
	return s.configWrites.Apply(request)
}

func (s *RegistryStore) ApplyTrackingMutation(charter, slug string, tracking *domain.TrackingFile, extraWrites []PlannedWrite) (*TrackingMutationResult, error) {
	return s.trackings.Apply(charter, slug, tracking, extraWrites)
}

func (s *RegistryStore) ApplySpecCreate(charter *domain.Charter, tracking *domain.TrackingFile, mutation DesignDocMutation) (*SpecCreateMutationResult, error) {
	return s.specCreate.Apply(charter, tracking, mutation)
}

func (s *RegistryStore) PrepareSpecCreate(request SpecCreatePlanRequest) (SpecCreatePlan, error) {
	return s.specCreate.Prepare(request)
}

func PlanSpecCreateWrites(workspace *Workspace, charter *domain.Charter, tracking *domain.TrackingFile, mutation DesignDocMutation) ([]PlannedWrite, error) {
	charterData, err := marshalYAML(charter)
	if err != nil {
		return nil, err
	}
	trackingData, err := marshalYAML(tracking)
	if err != nil {
		return nil, err
	}

	writes := []PlannedWrite{
		{Path: workspace.CharterPath(charter.Name), Data: charterData, Perm: 0644},
		{Path: workspace.TrackingPath(tracking.Charter, tracking.Slug), Data: trackingData, Perm: 0644},
	}
	if mutation.Action != "validated_existing" {
		writes = append(writes, PlannedWrite{
			Path: filepathForRepoFile(workspace.repoRoot, tracking.Documents.Primary),
			Data: mutation.Content,
			Perm: 0644,
		})
	}
	return writes, nil
}

func (s *RegistryStore) TagInUse(tag string) (bool, error) {
	return s.trackings.TagInUse(tag)
}

func marshalYAML(value any) ([]byte, error) {
	data, err := yaml.Marshal(value)
	if err != nil {
		return nil, fmt.Errorf("marshal YAML: %w", err)
	}
	return data, nil
}

func filepathForRepoFile(repoRoot, relativePath string) string {
	return filepath.Join(repoRoot, filepath.FromSlash(relativePath))
}

var _ RegistryAccess = (*RegistryStore)(nil)
