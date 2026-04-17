package dashboard

import (
	"sort"
	"strings"

	"github.com/aitoroses/specctl/internal/application"
	"github.com/aitoroses/specctl/internal/domain"
)

// OverviewResponse is the top-level dashboard summary.
type OverviewResponse struct {
	TotalCharters     int              `json:"total_charters"`
	TotalSpecs        int              `json:"total_specs"`
	TotalRequirements int              `json:"total_requirements"`
	VerifiedPct       float64          `json:"verified_pct"`
	OpenDeltas        int              `json:"open_deltas"`
	Charters          []CharterSummary `json:"charters"`
}

// CharterSpecEntry is a lightweight spec reference within a charter summary.
type CharterSpecEntry struct {
	Slug        string  `json:"slug"`
	Title       string  `json:"title"`
	VerifiedPct float64 `json:"verified_pct"`
}

// CharterSummary is a per-charter roll-up used in overview and charter list.
type CharterSummary struct {
	Name        string             `json:"name"`
	Title       string             `json:"title"`
	SpecCount   int                `json:"spec_count"`
	VerifiedPct float64            `json:"verified_pct"`
	OpenDeltas  int                `json:"open_deltas"`
	Specs       []CharterSpecEntry `json:"specs"`
}

// RequirementDetail is a simplified requirement for the dashboard.
type RequirementDetail struct {
	ID           string   `json:"id"`
	Title        string   `json:"title"`
	Lifecycle    string   `json:"lifecycle"`
	Verification string   `json:"verification"`
	Gherkin      string   `json:"gherkin"`
	Tags         []string `json:"tags"`
	TestFiles    []string `json:"test_files"`
}

// DeltaDetail is a single delta for the dashboard.
type DeltaDetail struct {
	ID                   string   `json:"id"`
	Area                 string   `json:"area"`
	Intent               string   `json:"intent"`
	Status               string   `json:"status"`
	Current              string   `json:"current"`
	Target               string   `json:"target"`
	Notes                string   `json:"notes"`
	AffectsRequirements  []string `json:"affects_requirements"`
}

// ChangelogEntry is a revision history entry.
type ChangelogEntry struct {
	Rev        int    `json:"rev"`
	Checkpoint string `json:"checkpoint"`
	Summary    string `json:"summary"`
	Date       string `json:"date"`
}

// DocumentsDetail holds primary and secondary doc paths.
type DocumentsDetail struct {
	Primary   string   `json:"primary"`
	Secondary []string `json:"secondary,omitempty"`
}

// SpecDetail is a dashboard-focused view of a single spec.
type SpecDetail struct {
	Slug         string              `json:"slug"`
	Charter      string              `json:"charter"`
	Title        string              `json:"title"`
	Status       string              `json:"status"`
	Rev          int                 `json:"rev"`
	Checkpoint   string              `json:"checkpoint"`
	VerifiedPct  float64             `json:"verified_pct"`
	OpenDeltas   int                 `json:"open_deltas"`
	Scope        []string            `json:"scope"`
	Documents    DocumentsDetail     `json:"documents"`
	Deltas       []DeltaDetail       `json:"deltas"`
	Requirements []RequirementDetail `json:"requirements"`
	Changelog    []ChangelogEntry    `json:"changelog"`
	Tags         []string            `json:"tags"`
	Created      string              `json:"created"`
	Updated      string              `json:"updated"`
}

// GraphNode represents a spec in the dependency graph.
type GraphNode struct {
	ID          string  `json:"id"`
	Label       string  `json:"label"`
	Charter     string  `json:"charter"`
	Status      string  `json:"status"`
	VerifiedPct float64 `json:"verified_pct"`
	ReqCount    int     `json:"req_count"`
	Group       string  `json:"group"`
}

// GraphEdge represents a dependency between two specs.
type GraphEdge struct {
	Source string `json:"source"`
	Target string `json:"target"`
}

// GraphResponse is the full dependency graph payload.
type GraphResponse struct {
	Nodes []GraphNode `json:"nodes"`
	Edges []GraphEdge `json:"edges"`
}


// countActiveVerified returns the number of active requirements and how many
// of those are verified.
func countActiveVerified(spec application.SpecProjection) (active, verified int) {
	for _, req := range spec.Requirements {
		if req.Lifecycle == domain.RequirementLifecycleActive {
			active++
			if req.Verification == domain.RequirementVerificationVerified {
				verified++
			}
		}
	}
	return
}

// verifiedPct computes the fraction (0.0–1.0) of active requirements that are verified.
func verifiedPct(active, verified int) float64 {
	if active == 0 {
		return 0
	}
	return float64(verified) / float64(active)
}

// buildOverview builds the top-level dashboard overview from loaded data.
func buildOverview(registry application.RegistryProjection, specs map[string]application.SpecProjection) OverviewResponse {
	charters := buildCharters(registry, specs)

	totalReqs := 0
	totalActive := 0
	totalVerified := 0
	openDeltas := 0

	for _, spec := range specs {
		totalReqs += len(spec.Requirements)
		a, v := countActiveVerified(spec)
		totalActive += a
		totalVerified += v
		openDeltas += spec.Deltas.Open + spec.Deltas.InProgress
	}

	return OverviewResponse{
		TotalCharters:     len(registry.Charters),
		TotalSpecs:        len(registry.Specs),
		TotalRequirements: totalReqs,
		VerifiedPct:       verifiedPct(totalActive, totalVerified),
		OpenDeltas:        openDeltas,
		Charters:          charters,
	}
}

// buildCharters builds per-charter summaries.
func buildCharters(registry application.RegistryProjection, specs map[string]application.SpecProjection) []CharterSummary {
	type agg struct {
		summary  CharterSummary
		active   int
		verified int
	}

	aggMap := make(map[string]*agg, len(registry.Charters))
	for _, c := range registry.Charters {
		aggMap[c.Name] = &agg{
			summary: CharterSummary{
				Name:  c.Name,
				Title: c.Title,
				Specs: []CharterSpecEntry{},
			},
		}
	}

	for _, spec := range specs {
		a := aggMap[spec.Charter]
		if a == nil {
			continue
		}
		a.summary.SpecCount++
		active, verified := countActiveVerified(spec)
		a.active += active
		a.verified += verified
		a.summary.OpenDeltas += spec.Deltas.Open + spec.Deltas.InProgress
		a.summary.Specs = append(a.summary.Specs, CharterSpecEntry{
			Slug:        spec.Slug,
			Title:       spec.Title,
			VerifiedPct: verifiedPct(active, verified),
		})
	}

	result := make([]CharterSummary, 0, len(aggMap))
	for _, a := range aggMap {
		a.summary.VerifiedPct = verifiedPct(a.active, a.verified)
		sort.Slice(a.summary.Specs, func(i, j int) bool {
			return a.summary.Specs[i].Slug < a.summary.Specs[j].Slug
		})
		result = append(result, a.summary)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].Name < result[j].Name
	})
	return result
}

// buildGraph builds the dependency graph for all specs.
func buildGraph(specs map[string]application.SpecProjection) GraphResponse {
	nodes := make([]GraphNode, 0, len(specs))
	edges := make([]GraphEdge, 0)

	for key, spec := range specs {
		active, verified := countActiveVerified(spec)

		group := ""
		if spec.CharterMembership != nil {
			group = spec.CharterMembership.Group
		}

		nodes = append(nodes, GraphNode{
			ID:          key,
			Label:       spec.Title,
			Charter:     spec.Charter,
			Status:      string(spec.Status),
			VerifiedPct: verifiedPct(active, verified),
			ReqCount:    len(spec.Requirements),
			Group:       group,
		})

		if spec.CharterMembership != nil {
			for _, dep := range spec.CharterMembership.DependsOn {
				dep = strings.TrimSpace(dep)
				if dep == "" {
					continue
				}
				edges = append(edges, GraphEdge{
					Source: key,
					Target: spec.Charter + ":" + dep,
				})
			}
		}
	}

	sort.Slice(nodes, func(i, j int) bool {
		return nodes[i].ID < nodes[j].ID
	})

	return GraphResponse{
		Nodes: nodes,
		Edges: edges,
	}
}

// buildSpecDetails converts the raw spec projections into dashboard SpecDetail objects.
func buildSpecDetails(specs map[string]application.SpecProjection) map[string]SpecDetail {
	result := make(map[string]SpecDetail, len(specs))
	for key, spec := range specs {
		active, verified := countActiveVerified(spec)

		reqs := make([]RequirementDetail, 0, len(spec.Requirements))
		for _, req := range spec.Requirements {
			tags := req.Tags
			if tags == nil {
				tags = []string{}
			}
			testFiles := req.TestFiles
			if testFiles == nil {
				testFiles = []string{}
			}
			reqs = append(reqs, RequirementDetail{
				ID:           req.ID,
				Title:        req.Title,
				Lifecycle:    string(req.Lifecycle),
				Verification: string(req.Verification),
				Gherkin:      req.Gherkin,
				Tags:         tags,
				TestFiles:    testFiles,
			})
		}

		tags := spec.Tags
		if tags == nil {
			tags = []string{}
		}

		// Build deltas
		deltas := make([]DeltaDetail, 0, len(spec.Deltas.Items))
		for _, d := range spec.Deltas.Items {
			affects := d.AffectsRequirements
			if affects == nil {
				affects = []string{}
			}
			deltas = append(deltas, DeltaDetail{
				ID:                  d.ID,
				Area:                d.Area,
				Intent:              string(d.Intent),
				Status:              string(d.Status),
				Current:             d.Current,
				Target:              d.Target,
				Notes:               d.Notes,
				AffectsRequirements: affects,
			})
		}

		// Build changelog
		changelog := make([]ChangelogEntry, 0, len(spec.Changelog))
		for _, c := range spec.Changelog {
			changelog = append(changelog, ChangelogEntry{
				Rev:        c.Rev,
				Checkpoint: spec.Checkpoint, // use spec-level checkpoint
				Summary:    c.Summary,
				Date:       c.Date,
			})
		}

		// Build scope
		scope := spec.Scope
		if scope == nil {
			scope = []string{}
		}

		// Build documents
		docs := DocumentsDetail{
			Primary: spec.Documents.Primary,
		}
		if spec.Documents.Secondary != nil {
			docs.Secondary = spec.Documents.Secondary
		}

		result[key] = SpecDetail{
			Slug:         spec.Slug,
			Charter:      spec.Charter,
			Title:        spec.Title,
			Status:       string(spec.Status),
			Rev:          spec.Rev,
			Checkpoint:   spec.Checkpoint,
			VerifiedPct:  verifiedPct(active, verified),
			OpenDeltas:   spec.Deltas.Open + spec.Deltas.InProgress,
			Scope:        scope,
			Documents:    docs,
			Deltas:       deltas,
			Requirements: reqs,
			Changelog:    changelog,
			Tags:         tags,
			Created:      spec.Created,
			Updated:      spec.Updated,
		}
	}
	return result
}
