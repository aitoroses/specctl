package infrastructure

import "github.com/aitoroses/specctl/internal/domain"

type PathAccess interface {
	RepoRoot() string
	SpecsDir() string
	TrackingPath(charter, slug string) string
	TrackingRelativePath(charter, slug string) string
	TrackingExists(charter, slug string) (bool, error)
	CharterExists(charter string) (bool, error)
	NormalizeOwnershipPath(file string) (string, error)
	NormalizeDesignDocPath(value string) (string, error)
	NormalizeScopePaths(values []string) ([]string, error)
	NormalizeVerifyFiles(values []string) ([]string, error)
	EnsureSourcePrefix(prefix string) (string, error)
	ReadRepoFile(relativePath string) ([]byte, error)
	ReadRepoFileIfExists(relativePath string) ([]byte, bool, error)
}

type RegistryAccess interface {
	LoadProjectConfig() (*ProjectConfig, error)
	LoadTracking(charter, slug string) (*domain.TrackingFile, error)
	LoadTrackingLenient(charter, slug string) (*domain.TrackingFile, []ValidationFinding, error)
	LoadCharterStructure(charter string) (*domain.Charter, error)
	ApplyCharterMutation(charter *domain.Charter) (*CharterMutationResult, error)
	ApplyCharterEntryMutation(request CharterEntryMutationRequest) (*CharterEntryMutationResult, error)
	ApplyConfigMutation(request ConfigMutationRequest) (*ConfigMutationResult, error)
	ApplyTrackingMutation(charter, slug string, tracking *domain.TrackingFile, extraWrites []PlannedWrite) (*TrackingMutationResult, error)
	ApplySpecCreate(charter *domain.Charter, tracking *domain.TrackingFile, mutation DesignDocMutation) (*SpecCreateMutationResult, error)
	PrepareSpecCreate(request SpecCreatePlanRequest) (SpecCreatePlan, error)
	TagInUse(tag string) (bool, error)
}

type RepoReadAccess interface {
	LoadRepoReadSnapshot() (*RepoReadSnapshot, error)
	LoadRepoReadSnapshotWithWrites(writes []PlannedWrite) (*RepoReadSnapshot, error)
	ResolveSpecProjectionInputs(tracking *domain.TrackingFile, config *ProjectConfig) SpecProjectionInputs
	ResolveFileOwnership(file string, snapshot *RepoReadSnapshot) (FileOwnershipResolution, error)
	PrepareHookInputs(stdin string, snapshot *RepoReadSnapshot) (HookPreparedInput, error)
}

type CheckpointAccess interface {
	ResolveCheckpoint(ref string) (string, error)
	ReadGitFile(checkpoint, relativePath string) ([]byte, error)
	LoadTrackingAtRevision(charter, slug, checkpoint string) (*domain.TrackingFile, error)
	LoadSpecComparison(charter, slug string, current *domain.TrackingFile, checkpoint string) (SpecComparison, error)
	NormalizeDocumentForDiff(data []byte) []byte
}

type ServiceAdapters struct {
	RepoRoot    string
	SpecsDir    string
	Paths       PathAccess
	Registry    RegistryAccess
	RepoReads   RepoReadAccess
	Checkpoints CheckpointAccess
}

type RepoReadSnapshot struct {
	Config             *ProjectConfig
	ConfigReadFindings []ValidationFinding
	Trackings          map[string]RepoTrackingSnapshot
	Charters           map[string]RepoCharterSnapshot
}

type RepoTrackingSnapshot struct {
	Tracking *domain.TrackingFile
	Findings []ValidationFinding
}

type RepoCharterSnapshot struct {
	Charter  *domain.Charter
	Findings []ValidationFinding
}

type ResolvedDesignDocMetadata struct {
	Format         *string
	FormatTemplate *string
}

type SpecProjectionInputs struct {
	ScopeDrift         ScopeDriftSnapshot
	ScopeDriftFindings []ValidationFinding
	DesignDoc          *ResolvedDesignDocMetadata
	Requirements       map[string]RequirementDocContext
}

type RequirementDocContext struct {
	MatchStatus string
	Heading     string
	Scenarios   []string
}

type FileOwnershipResolution struct {
	File               string
	Resolution         string
	MatchSource        *string
	GoverningSpec      *FileOwnershipSpec
	Matches            []FileOwnershipMatch
	ValidationFindings []ValidationFinding
	CreatePlan         *SpecCreateSuggestion
}

type FileOwnershipSpec struct {
	Slug         string
	Charter      string
	TrackingFile string
	Documents    domain.Documents
}

type FileOwnershipMatch struct {
	Slug        string
	Charter     string
	MatchSource string
	ScopePrefix string
}

type SpecCreateSuggestion struct {
	Charter            string
	Target             string
	Scope              string
	CreateCharterFirst bool
}

type HookPreparedInput struct {
	InputFiles      []string
	ConsideredFiles []string
	IgnoredFiles    []string
	Entries         []HookPreparedEntry
}

type HookPreparedEntry struct {
	Path      string
	Managed   *ManagedHookClassification
	Ownership *FileOwnershipResolution
}

type ManagedHookClassification struct {
	Kind            string
	AffectedTargets []string
}

type SpecCreatePlanRequest struct {
	Charter string
	Slug    string
	Doc     string
	Scope   []string
	Config  *ProjectConfig
}

type SpecCreatePlan struct {
	Scope    []string
	DocPath  string
	Mutation DesignDocMutation
}

type SpecCreateMutationResult struct {
	Snapshot *RepoReadSnapshot
}

type SpecCreatePlanErrorCode string

const (
	SpecCreateInvalidPath                SpecCreatePlanErrorCode = "invalid_path"
	SpecCreateFormatAmbiguous            SpecCreatePlanErrorCode = "format_ambiguous"
	SpecCreateFormatNotConfigured        SpecCreatePlanErrorCode = "format_not_configured"
	SpecCreatePrimaryDocFrontmatterError SpecCreatePlanErrorCode = "primary_doc_frontmatter_invalid"
	SpecCreatePrimaryDocMismatch         SpecCreatePlanErrorCode = "primary_doc_frontmatter_mismatch"
)

type SpecCreatePlanError struct {
	Code         SpecCreatePlanErrorCode
	Message      string
	DocPath      string
	InvalidPaths []string
}

func (e *SpecCreatePlanError) Error() string {
	return e.Message
}

type VerifyFilesNormalizationErrorCode string

const (
	VerifyFilesInvalidPath VerifyFilesNormalizationErrorCode = "invalid_path"
	VerifyFilesMissing     VerifyFilesNormalizationErrorCode = "missing"
)

type VerifyFilesNormalizationError struct {
	Code    VerifyFilesNormalizationErrorCode
	Message string
	Paths   []string
}

func (e *VerifyFilesNormalizationError) Error() string {
	return e.Message
}

type CharterMutationResult struct {
	Snapshot *RepoReadSnapshot
}

type TrackingMutationResult struct {
	Snapshot *RepoReadSnapshot
}

type TrackingMutationError struct {
	Message  string
	Findings []ValidationFinding
}

func (e *TrackingMutationError) Error() string {
	return e.Message
}

type CharterMutationErrorCode string

const (
	CharterMutationValidationFailed CharterMutationErrorCode = "validation_failed"
)

type CharterMutationError struct {
	Code         CharterMutationErrorCode
	Message      string
	Findings     []ValidationFinding
	Snapshot     *RepoReadSnapshot
	PostSnapshot *RepoReadSnapshot
}

func (e *CharterMutationError) Error() string {
	return e.Message
}

type CharterEntryMutationRequest struct {
	Charter    string
	Slug       string
	Group      string
	GroupTitle *string
	GroupOrder *int
	Order      int
	DependsOn  []string
	Notes      string
}

type CharterEntryMutationResult struct {
	Entry        domain.CharterSpecEntry
	CreatedGroup *domain.CharterGroup
	Snapshot     *RepoReadSnapshot
}

type CharterEntryMutationErrorCode string

const (
	CharterEntryMutationGroupRequired CharterEntryMutationErrorCode = "group_required"
	CharterEntryMutationValidation    CharterEntryMutationErrorCode = "validation_failed"
	CharterEntryMutationCycle         CharterEntryMutationErrorCode = "charter_cycle"
)

type CharterEntryMutationError struct {
	Code          CharterEntryMutationErrorCode
	Message       string
	Group         string
	MissingFields []string
	Entry         *domain.CharterSpecEntry
	Cycle         []string
	Findings      []ValidationFinding
	Snapshot      *RepoReadSnapshot
	PostSnapshot  *RepoReadSnapshot
}

func (e *CharterEntryMutationError) Error() string {
	return e.Message
}

type ConfigMutationKind string

const (
	ConfigMutationAddTag          ConfigMutationKind = "add-tag"
	ConfigMutationRemoveTag       ConfigMutationKind = "remove-tag"
	ConfigMutationAddPrefix       ConfigMutationKind = "add-prefix"
	ConfigMutationRemovePrefix    ConfigMutationKind = "remove-prefix"
)

type ConfigMutationRequest struct {
	Kind  ConfigMutationKind
	Value string
}

type ConfigMutationResult struct {
	Mutation ConfigMutationKind
	Value    string
	Snapshot *RepoReadSnapshot
}

type ConfigMutationErrorCode string

const (
	ConfigMutationSemanticTagReserved  ConfigMutationErrorCode = "semantic_tag_reserved"
	ConfigMutationTagExists            ConfigMutationErrorCode = "tag_exists"
	ConfigMutationTagNotFound          ConfigMutationErrorCode = "tag_not_found"
	ConfigMutationTagInUse             ConfigMutationErrorCode = "tag_in_use"
	ConfigMutationPrefixExists        ConfigMutationErrorCode = "prefix_exists"
	ConfigMutationPrefixNotFound      ConfigMutationErrorCode = "prefix_not_found"
	ConfigMutationInvalidPath         ConfigMutationErrorCode = "invalid_path"
	ConfigMutationValidationFailed    ConfigMutationErrorCode = "validation_failed"
)

type ConfigMutationError struct {
	Code               ConfigMutationErrorCode
	Message            string
	Mutation           ConfigMutationKind
	Value              string
	InvalidPaths       []string
	ValidationFindings []ValidationFinding
	Snapshot           *RepoReadSnapshot
	PostSnapshot       *RepoReadSnapshot
}

func (e *ConfigMutationError) Error() string {
	return e.Message
}

type SpecComparison struct {
	BaselineTracking   *domain.TrackingFile
	CurrentDoc         []byte
	BaselineDoc        []byte
	NormalizedCurrent  []byte
	NormalizedBaseline []byte
	BaselineDocMissing bool
	BaselineDocError   error
}

func OpenServiceAdaptersFromWorkingDir() (ServiceAdapters, error) {
	workspace, err := OpenWorkspaceFromWorkingDir()
	if err != nil {
		return ServiceAdapters{}, err
	}
	return buildServiceAdapters(workspace), nil
}

func NewServiceAdapters(repoRoot string) ServiceAdapters {
	return buildServiceAdapters(NewWorkspace(repoRoot))
}

func buildServiceAdapters(workspace *Workspace) ServiceAdapters {
	registry := NewRegistryStore(workspace)
	repoReads := NewRepoReadStore(workspace)
	checkpoints := NewCheckpointStore(workspace)
	return ServiceAdapters{
		RepoRoot:    workspace.RepoRoot(),
		SpecsDir:    workspace.SpecsDir(),
		Paths:       workspace,
		Registry:    registry,
		RepoReads:   repoReads,
		Checkpoints: checkpoints,
	}
}
