package infrastructure

import (
	"os"
	"path/filepath"
	"testing"
)

func TestEvaluateRepoSnapshotPropagatesMissingCharterToSpecAudit(t *testing.T) {
	repoRoot := copyInfrastructureFixtureRepo(t, "ready-spec")
	if err := os.Remove(filepath.Join(repoRoot, ".specs", "runtime", "CHARTER.yaml")); err != nil {
		t.Fatalf("remove charter: %v", err)
	}

	snapshot, err := NewRepoReadStore(NewWorkspace(repoRoot)).LoadRepoReadSnapshot()
	if err != nil {
		t.Fatalf("LoadRepoReadSnapshot: %v", err)
	}

	evaluation := EvaluateRepoSnapshot(snapshot)
	if !containsValidationFinding(evaluation.SpecFindings["runtime:session-lifecycle"], "CHARTER_SPEC_MISSING", ".specs/runtime/CHARTER.yaml", "session-lifecycle") {
		t.Fatalf("spec findings = %#v", evaluation.SpecFindings["runtime:session-lifecycle"])
	}
	if !containsValidationFinding(evaluation.AuditFindings, "CHARTER_SPEC_MISSING", ".specs/runtime/CHARTER.yaml", "session-lifecycle") {
		t.Fatalf("audit findings = %#v", evaluation.AuditFindings)
	}
	if got := repoSnapshotBlockingFindings(snapshot); !containsValidationFinding(got, "CHARTER_SPEC_MISSING", ".specs/runtime/CHARTER.yaml", "session-lifecycle") {
		t.Fatalf("blocking findings = %#v", got)
	}
}

func TestEvaluateRepoSnapshotPropagatesRelationshipFindings(t *testing.T) {
	repoRoot := copyInfrastructureFixtureRepo(t, "ready-spec")
	if err := os.Remove(filepath.Join(repoRoot, ".specs", "runtime", "session-lifecycle.yaml")); err != nil {
		t.Fatalf("remove tracking: %v", err)
	}

	snapshot, err := NewRepoReadStore(NewWorkspace(repoRoot)).LoadRepoReadSnapshot()
	if err != nil {
		t.Fatalf("LoadRepoReadSnapshot: %v", err)
	}

	evaluation := EvaluateRepoSnapshot(snapshot)
	if !containsValidationFinding(evaluation.CharterFindings["runtime"], "CHARTER_SPEC_MISSING", ".specs/runtime/CHARTER.yaml", "session-lifecycle") {
		t.Fatalf("charter findings = %#v", evaluation.CharterFindings["runtime"])
	}
	if !containsValidationFinding(evaluation.AuditFindings, "CHARTER_SPEC_MISSING", ".specs/runtime/CHARTER.yaml", "session-lifecycle") {
		t.Fatalf("audit findings = %#v", evaluation.AuditFindings)
	}
}

func containsValidationFinding(findings []ValidationFinding, code, path, target string) bool {
	for _, finding := range findings {
		if finding.Code == code && finding.Path == path && finding.Target == target {
			return true
		}
	}
	return false
}
