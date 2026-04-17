package infrastructure

import (
	"fmt"
	"path"
	"sort"
	"strings"
)

type RepoSnapshotEvaluation struct {
	AuditFindings   []ValidationFinding
	CharterFindings map[string][]ValidationFinding
	SpecFindings    map[string][]ValidationFinding
}

func EvaluateRepoSnapshot(snapshot *RepoReadSnapshot) RepoSnapshotEvaluation {
	evaluation := RepoSnapshotEvaluation{
		CharterFindings: map[string][]ValidationFinding{},
		SpecFindings:    map[string][]ValidationFinding{},
	}
	if snapshot == nil {
		return evaluation
	}

	for key, tracking := range snapshot.Trackings {
		evaluation.SpecFindings[key] = append([]ValidationFinding{}, tracking.Findings...)
	}
	for name, charter := range snapshot.Charters {
		evaluation.CharterFindings[name] = append([]ValidationFinding{}, charter.Findings...)
	}

	for _, name := range sortedRepoSnapshotCharterNames(snapshot) {
		charter := snapshot.Charters[name]
		findings := append([]ValidationFinding{}, evaluation.CharterFindings[name]...)
		declared := map[string]struct{}{}
		if charter.Charter != nil {
			declared = make(map[string]struct{}, len(charter.Charter.Specs))
			for _, entry := range charter.Charter.Specs {
				declared[entry.Slug] = struct{}{}
				if _, exists := snapshot.Trackings[name+":"+entry.Slug]; exists {
					continue
				}
				findings = append(findings, ValidationFindingsFromMessages(
					fmt.Sprintf("charter spec %q does not have a tracking file at %s", entry.Slug, path.Join(".specs", name, entry.Slug+".yaml")),
					RelativeCharterPath(name),
					entry.Slug,
				)...)
			}
		}
		evaluation.CharterFindings[name] = uniqueFindings(findings)
	}

	for _, target := range sortedRepoSnapshotTrackingTargets(snapshot) {
		charterName, slug, _ := strings.Cut(target, ":")
		findings := append([]ValidationFinding{}, evaluation.SpecFindings[target]...)
		_, charterExists := snapshot.Charters[charterName]
		if !charterExists {
			findings = append(findings, ValidationFinding{
				Code:     "CHARTER_SPEC_MISSING",
				Severity: "error",
				Message:  "charter file does not exist",
				Path:     RelativeCharterPath(charterName),
				Target:   slug,
			})
			evaluation.SpecFindings[target] = uniqueFindings(findings)
			continue
		}
		for _, finding := range evaluation.CharterFindings[charterName] {
			if finding.Target == "" || finding.Target == slug || finding.Target == "specs" || finding.Target == "groups" {
				findings = append(findings, finding)
			}
		}
		evaluation.SpecFindings[target] = uniqueFindings(findings)
	}

	findings := append([]ValidationFinding{}, snapshot.ConfigReadFindings...)
	for _, name := range sortedRepoSnapshotCharterNames(snapshot) {
		findings = append(findings, evaluation.CharterFindings[name]...)
	}
	for _, target := range sortedRepoSnapshotTrackingTargets(snapshot) {
		findings = append(findings, evaluation.SpecFindings[target]...)
	}
	evaluation.AuditFindings = uniqueFindings(findings)
	return evaluation
}

func sortedRepoSnapshotCharterNames(snapshot *RepoReadSnapshot) []string {
	names := make([]string, 0, len(snapshot.Charters))
	for name := range snapshot.Charters {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func sortedRepoSnapshotTrackingTargets(snapshot *RepoReadSnapshot) []string {
	targets := make([]string, 0, len(snapshot.Trackings))
	for target := range snapshot.Trackings {
		targets = append(targets, target)
	}
	sort.Strings(targets)
	return targets
}
