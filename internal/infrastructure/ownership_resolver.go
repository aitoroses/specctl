package infrastructure

import (
	"bufio"
	"path"
	"sort"
	"strings"

	"github.com/aitoroses/specctl/internal/domain"
)

type fileOwnershipCandidate struct {
	tracking    *domain.TrackingFile
	matchSource string
	scopePrefix string
	score       int
}

func (s *RepoReadStore) ResolveFileOwnership(file string, snapshot *RepoReadSnapshot) (FileOwnershipResolution, error) {
	normalized, err := s.workspace.NormalizeOwnershipPath(file)
	if err != nil {
		return FileOwnershipResolution{}, err
	}

	trackings := snapshotTrackings(snapshot)
	charterOrder := snapshotCharterOrder(snapshot)
	resolution := resolveFileOwnership(s.workspace.RepoRoot(), normalized, trackings, charterOrder)
	if resolution.Resolution == "no_match" {
		resolution.CreatePlan = planSpecCreateSuggestion(snapshot, normalized)
	}
	return resolution, nil
}

func (s *RepoReadStore) PrepareHookInputs(stdin string, snapshot *RepoReadSnapshot) (HookPreparedInput, error) {
	trackings := snapshotTrackings(snapshot)
	docPaths := make(map[string]struct{}, len(trackings))
	for _, tracking := range trackings {
		docPaths[tracking.Documents.Primary] = struct{}{}
	}

	inputFiles := make([]string, 0)
	considered := make([]string, 0)
	ignored := make([]string, 0)
	entries := make([]HookPreparedEntry, 0)

	scanner := bufio.NewScanner(strings.NewReader(stdin))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		normalized, normalizeErr := s.workspace.NormalizeOwnershipPath(line)
		if normalizeErr != nil {
			ignored = append(ignored, line)
			continue
		}
		inputFiles = append(inputFiles, normalized)

		consider := strings.HasPrefix(normalized, ".specs/") || hasHookPath(docPaths, normalized) || hasMatchingSourcePrefix(snapshot.Config, normalized)
		if !consider {
			ignored = append(ignored, normalized)
			continue
		}
		considered = append(considered, normalized)

		if managed := classifyManagedHookPath(snapshot, normalized); managed != nil {
			entries = append(entries, HookPreparedEntry{
				Path:    normalized,
				Managed: managed,
			})
			continue
		}

		ownership := resolveFileOwnership(s.workspace.RepoRoot(), normalized, trackings, snapshotCharterOrder(snapshot))
		entries = append(entries, HookPreparedEntry{
			Path:      normalized,
			Ownership: &ownership,
		})
	}
	if err := scanner.Err(); err != nil {
		return HookPreparedInput{}, err
	}

	return HookPreparedInput{
		InputFiles:      uniquePaths(inputFiles),
		ConsideredFiles: uniquePaths(considered),
		IgnoredFiles:    uniquePaths(ignored),
		Entries:         entries,
	}, nil
}

func resolveFileOwnership(repoRoot, normalized string, trackings []*domain.TrackingFile, charterOrder map[string]map[string]int) FileOwnershipResolution {
	matches := make([]fileOwnershipCandidate, 0)
	for _, tracking := range trackings {
		if tracking.Documents.Primary == normalized {
			matches = append(matches, fileOwnershipCandidate{
				tracking:    tracking,
				matchSource: "design_doc",
				score:       1_000_000,
			})
			continue
		}
		longest := ""
		for _, scope := range tracking.Scope {
			if strings.HasPrefix(normalized, scope) && len(scope) > len(longest) {
				longest = scope
			}
		}
		if longest != "" {
			matches = append(matches, fileOwnershipCandidate{
				tracking:    tracking,
				matchSource: "scope",
				scopePrefix: longest,
				score:       len(longest),
			})
		}
	}

	sort.Slice(matches, func(i, j int) bool {
		return fileCandidateLess(matches[i], matches[j], charterOrder)
	})

	resolution := FileOwnershipResolution{
		File:               normalized,
		Resolution:         "no_match",
		Matches:            []FileOwnershipMatch{},
		ValidationFindings: []ValidationFinding{},
	}
	for _, match := range matches {
		resolution.Matches = append(resolution.Matches, FileOwnershipMatch{
			Slug:        match.tracking.Slug,
			Charter:     match.tracking.Charter,
			MatchSource: match.matchSource,
			ScopePrefix: match.scopePrefix,
		})
	}

	if len(matches) == 0 {
		return resolution
	}

	best := matches[0]
	resolution.MatchSource = stringPointer(best.matchSource)
	if winner, ok := selectGoverningCandidate(matches); ok {
		resolution.Resolution = "matched"
		resolution.GoverningSpec = &FileOwnershipSpec{
			Slug:         winner.tracking.Slug,
			Charter:      winner.tracking.Charter,
			TrackingFile: RelativeTrackingPath(repoRoot, winner.tracking.Charter, winner.tracking.Slug),
			Documents:    winner.tracking.Documents,
		}
		return resolution
	}

	resolution.Resolution = "ambiguous"
	resolution.ValidationFindings = []ValidationFinding{{
		Code:     "AMBIGUOUS_FILE_OWNERSHIP",
		Severity: "error",
		Message:  normalized + " matches more than one declared spec scope",
		Path:     normalized,
	}}
	return resolution
}

func selectGoverningCandidate(matches []fileOwnershipCandidate) (fileOwnershipCandidate, bool) {
	if len(matches) == 0 {
		return fileOwnershipCandidate{}, false
	}

	best := matches[0]
	contenders := make([]fileOwnershipCandidate, 0, len(matches))
	for _, match := range matches {
		if match.matchSource != best.matchSource {
			break
		}
		if match.score != best.score {
			break
		}
		contenders = append(contenders, match)
	}
	if len(contenders) == 1 {
		return best, true
	}
	return fileOwnershipCandidate{}, false
}

func fileCandidateLess(left, right fileOwnershipCandidate, charterOrder map[string]map[string]int) bool {
	if left.matchSource != right.matchSource {
		return left.matchSource == "design_doc"
	}
	if left.score != right.score {
		return left.score > right.score
	}
	if left.tracking.Charter == right.tracking.Charter {
		leftOrder := fileCandidateOrder(left, charterOrder)
		rightOrder := fileCandidateOrder(right, charterOrder)
		if leftOrder != rightOrder {
			return leftOrder < rightOrder
		}
	}
	if left.tracking.Charter != right.tracking.Charter {
		return left.tracking.Charter < right.tracking.Charter
	}
	return left.tracking.Slug < right.tracking.Slug
}

func fileCandidateOrder(candidate fileOwnershipCandidate, charterOrder map[string]map[string]int) int {
	orders := charterOrder[candidate.tracking.Charter]
	if orders == nil {
		return 0
	}
	if order, exists := orders[candidate.tracking.Slug]; exists {
		return order
	}
	return 0
}

func planSpecCreateSuggestion(snapshot *RepoReadSnapshot, file string) *SpecCreateSuggestion {
	if strings.HasPrefix(file, ".specs/") {
		return nil
	}

	segments := strings.Split(file, "/")
	if len(segments) < 2 {
		return nil
	}

	scope := path.Dir(file)
	if scope == "." || scope == "" {
		return nil
	}
	slug := strings.ReplaceAll(path.Base(scope), "_", "-")
	if strings.TrimSpace(slug) == "" {
		return nil
	}

	charter := segments[0]
	scope += "/"
	_, charterExists := snapshot.Charters[charter]
	return &SpecCreateSuggestion{
		Charter:            charter,
		Target:             charter + ":" + slug,
		Scope:              scope,
		CreateCharterFirst: !charterExists,
	}
}

func snapshotTrackings(snapshot *RepoReadSnapshot) []*domain.TrackingFile {
	trackings := make([]*domain.TrackingFile, 0, len(snapshot.Trackings))
	for _, tracking := range snapshot.Trackings {
		trackings = append(trackings, tracking.Tracking)
	}
	return trackings
}

func snapshotCharterOrder(snapshot *RepoReadSnapshot) map[string]map[string]int {
	order := make(map[string]map[string]int, len(snapshot.Charters))
	for name, charter := range snapshot.Charters {
		order[name] = domain.BuildLenientCharterOrdering(charter.Charter).Index
	}
	return order
}

func classifyManagedHookPath(snapshot *RepoReadSnapshot, value string) *ManagedHookClassification {
	if value == ".specs/specctl.yaml" {
		targets := make([]string, 0, len(snapshot.Trackings))
		for target := range snapshot.Trackings {
			targets = append(targets, target)
		}
		sort.Strings(targets)
		return &ManagedHookClassification{Kind: "config", AffectedTargets: targets}
	}

	parts := strings.Split(value, "/")
	if len(parts) != 3 || parts[0] != ".specs" {
		return nil
	}
	if parts[2] == "CHARTER.yaml" {
		charter, exists := snapshot.Charters[parts[1]]
		if !exists {
			return &ManagedHookClassification{Kind: "charter", AffectedTargets: []string{}}
		}
		ordering := domain.BuildLenientCharterOrdering(charter.Charter)
		targets := make([]string, 0, len(ordering.Specs))
		for _, spec := range ordering.Specs {
			targets = append(targets, parts[1]+":"+spec.Slug)
		}
		return &ManagedHookClassification{Kind: "charter", AffectedTargets: uniquePaths(targets)}
	}
	if path.Ext(parts[2]) != ".yaml" {
		return nil
	}
	return &ManagedHookClassification{
		Kind:            "tracking",
		AffectedTargets: []string{parts[1] + ":" + strings.TrimSuffix(parts[2], ".yaml")},
	}
}

func hasMatchingSourcePrefix(config *ProjectConfig, value string) bool {
	if config == nil {
		return false
	}
	for _, prefix := range config.SourcePrefixes {
		if strings.HasPrefix(value, prefix) {
			return true
		}
	}
	return false
}

func hasHookPath(values map[string]struct{}, value string) bool {
	_, exists := values[value]
	return exists
}

func uniquePaths(values []string) []string {
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

func stringPointer(value string) *string {
	return &value
}

var _ RepoReadAccess = (*RepoReadStore)(nil)
