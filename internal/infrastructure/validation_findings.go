package infrastructure

import (
	"regexp"
	"strings"
)

var (
	canonicalDeltaIDPattern       = regexp.MustCompile(`^D-\d{3}$`)
	canonicalRequirementIDPattern = regexp.MustCompile(`^REQ-\d{3}$`)
)

func ValidationFindingsFromMessages(raw, path, target string) []ValidationFinding {
	messages := splitValidationMessages(strings.TrimSpace(raw), path, target)
	findings := make([]ValidationFinding, 0, len(messages))
	for _, message := range messages {
		if message == "" {
			continue
		}
		findings = append(findings, validationFindingFromMessage(message, path, target))
	}
	if len(findings) == 0 {
		findings = append(findings, validationFindingFromMessage(strings.TrimSpace(raw), path, target))
	}
	return findings
}

func ValidationFindingFromError(err error, path, target string) ValidationFinding {
	if err == nil {
		return validationFindingFromMessage("", path, target)
	}
	return validationFindingFromMessage(err.Error(), path, target)
}

func validationFindingFromMessage(message, path, target string) ValidationFinding {
	if finding, ok := catalogValidationFinding(message, path, target); ok {
		return finding
	}

	return ValidationFinding{
		Code:     "SPEC_STATUS_INVALID",
		Severity: "error",
		Message:  message,
		Path:     path,
		Target:   target,
	}
}

func splitValidationMessages(raw, path, target string) []string {
	if raw == "" {
		return nil
	}

	parts := strings.Split(raw, "; ")
	if len(parts) == 1 {
		return []string{strings.TrimSpace(raw)}
	}

	messages := make([]string, 0, len(parts))
	current := strings.TrimSpace(parts[0])
	for _, part := range parts[1:] {
		next := strings.TrimSpace(part)
		if current == "" {
			current = next
			continue
		}
		if isValidationMessageStart(next, path, target) {
			messages = append(messages, current)
			current = next
			continue
		}
		current += "; " + next
	}
	if current != "" {
		messages = append(messages, current)
	}
	return messages
}

func isValidationMessageStart(message, path, target string) bool {
	if message == "" {
		return false
	}
	_, ok := catalogValidationFinding(message, path, target)
	return ok
}

func catalogValidationFinding(message, path, target string) (ValidationFinding, bool) {
	isCharterPath := strings.HasSuffix(path, "/CHARTER.yaml")
	finding := ValidationFinding{
		Code:     "SPEC_STATUS_INVALID",
		Severity: "error",
		Message:  message,
		Path:     path,
	}
	if target != "" {
		finding.Target = target
	}

	switch {
	case message == "":
		return finding, true
	case isCharterPath && (message == `charter file is missing required key "name"` ||
		message == `charter file is missing required key "title"` ||
		message == `charter file is missing required key "description"`):
		finding.Code = "CHARTER_NAME_INVALID"
		finding.Target = "name"
		return finding, true
	case isCharterPath && message == `charter file is missing required key "groups"`:
		finding.Code = "CHARTER_GROUP_INVALID"
		finding.Target = "groups"
		return finding, true
	case isCharterPath && message == `charter file is missing required key "specs"`:
		finding.Code = "CHARTER_SPEC_MISSING"
		finding.Target = "specs"
		return finding, true
	case isCharterPath && (message == "title is required" || message == "description is required"):
		finding.Code = "CHARTER_NAME_INVALID"
		finding.Target = "name"
		return finding, true
	case strings.Contains(message, "frontmatter references unknown format"):
		finding.Code = "FORMAT_NOT_CONFIGURED"
		finding.Target = "documents.primary"
		return finding, true
	case strings.Contains(message, "frontmatter is invalid"),
		strings.Contains(message, "frontmatter is missing"):
		finding.Code = "PRIMARY_DOC_FRONTMATTER_INVALID"
		finding.Target = "documents.primary"
		return finding, true
	case strings.Contains(message, "frontmatter spec "),
		strings.Contains(message, "frontmatter charter "):
		finding.Code = "PRIMARY_DOC_FRONTMATTER_MISMATCH"
		finding.Target = "documents.primary"
		return finding, true
	case strings.Contains(message, "tracking file is missing required key \"slug\""),
		strings.Contains(message, "tracking slug "):
		finding.Code = "SPEC_SLUG_INVALID"
		finding.Target = "slug"
		return finding, true
	case strings.Contains(message, "tracking file is missing required key \"charter\""),
		strings.Contains(message, "tracking charter "),
		strings.HasPrefix(message, "charter must match"):
		finding.Code = "SPEC_CHARTER_INVALID"
		finding.Target = "charter"
		return finding, true
	case strings.Contains(message, "tracking file is missing required key \"title\""),
		message == "title is required":
		finding.Code = "SPEC_TITLE_INVALID"
		finding.Target = "title"
		return finding, true
	case strings.Contains(message, "tracking file is missing required key \"tags\""),
		strings.HasPrefix(message, "tag "),
		strings.HasPrefix(message, "duplicate tag "):
		finding.Code = "SPEC_TAG_INVALID"
		finding.Target = "tags"
		return finding, true
	case strings.Contains(message, "tracking file is missing required key \"status\""),
		strings.HasPrefix(message, "status must be one of"):
		finding.Code = "SPEC_STATUS_INVALID"
		finding.Target = "status"
		return finding, true
	case strings.Contains(message, "status must match derived lifecycle state"):
		finding.Code = "SPEC_STATUS_MISMATCH"
		finding.Target = "status"
		return finding, true
	case strings.Contains(message, "tracking file is missing required key \"rev\""),
		strings.HasPrefix(message, "rev must be >="),
		strings.HasPrefix(message, "created must use YYYY-MM-DD"),
		strings.HasPrefix(message, "updated must use YYYY-MM-DD"),
		strings.HasPrefix(message, "last_verified_at must use YYYY-MM-DD"),
		strings.HasPrefix(message, "changelog.date must use YYYY-MM-DD"),
		strings.HasPrefix(message, "changelog rev "),
		strings.HasPrefix(message, "changelog rev must be >="):
		finding.Code = "REV_INVALID"
		if finding.Target == "" {
			finding.Target = "rev"
		}
		return finding, true
	case strings.Contains(message, "tracking file is missing required key \"checkpoint\""),
		strings.Contains(message, "checkpoint must be a git SHA"):
		finding.Code = "CHECKPOINT_INVALID"
		finding.Target = "checkpoint"
		return finding, true
	case strings.Contains(message, "unable to resolve git revision"),
		strings.Contains(message, "fatal: ambiguous argument"),
		strings.Contains(message, "unknown revision or path not in the working tree"):
		finding.Code = "CHECKPOINT_UNAVAILABLE"
		finding.Target = "checkpoint"
		return finding, true
	case strings.Contains(message, "tracking file is missing required key \"documents\""),
		strings.Contains(message, "tracking file is missing required key \"documents.primary\""),
		strings.Contains(message, "documents.primary does not exist"),
		strings.Contains(message, "documents.primary must point to a markdown file"),
		strings.Contains(message, "documents.primary must be under one of scope[]"),
		strings.HasPrefix(message, "documents.primary:"):
		finding.Code = "PRIMARY_DOC_MISSING"
		finding.Target = "documents.primary"
		return finding, true
	case strings.Contains(message, "documents.secondary does not exist"),
		strings.Contains(message, "documents.secondary") && strings.Contains(message, "must point to a markdown file"),
		strings.Contains(message, "duplicates documents.primary"),
		strings.Contains(message, "duplicate documents.secondary"):
		finding.Code = "SECONDARY_DOC_MISSING"
		finding.Target = "documents.secondary"
		return finding, true
	case strings.Contains(message, "tracking file is missing required key \"scope\""),
		strings.HasPrefix(message, "scope must contain at least one directory"):
		finding.Code = "SCOPE_EMPTY"
		finding.Target = "scope"
		return finding, true
	case strings.HasPrefix(message, "scope "),
		strings.Contains(message, "scope:"):
		finding.Code = "SCOPE_PATH_INVALID"
		finding.Target = "scope"
		return finding, true
	case strings.Contains(message, "delta IDs must be sequential without gaps"):
		finding.Code = sequenceFindingCode(message, canonicalDeltaIDPattern, "DELTA_ID_INVALID")
		finding.Target = "deltas"
		return finding, true
	case strings.Contains(message, "requirement IDs must be sequential without gaps"):
		finding.Code = sequenceFindingCode(message, canonicalRequirementIDPattern, "REQUIREMENT_ID_INVALID")
		finding.Target = "requirements"
		return finding, true
	case strings.Contains(message, "tracking file is missing required key \"deltas\""),
		strings.Contains(message, "every delta must include"),
		strings.Contains(message, "origin_checkpoint must be a git SHA"),
		strings.HasPrefix(message, "delta "):
		finding.Code = "DELTA_FIELD_INVALID"
		finding.Target = "deltas"
		if strings.Contains(message, "delta ID ") || strings.Contains(message, "found D-") {
			finding.Code = "DELTA_ID_INVALID"
		}
		if strings.Contains(message, "sequential without gaps") {
			finding.Code = "IDS_NON_SEQUENTIAL"
		}
		if strings.Contains(message, "cannot be closed without tracing requirements") {
			finding.Code = "DELTA_UNTRACED"
		}
		return finding, true
	case strings.Contains(message, "tracking file is missing required key \"requirements\""),
		strings.Contains(message, "every requirement must include"),
		strings.HasPrefix(message, "requirement "):
		finding.Code = "REQUIREMENT_GHERKIN_INVALID"
		finding.Target = "requirements"
		if strings.Contains(message, "requirement ID ") || strings.Contains(message, "expected REQ-") {
			finding.Code = "REQUIREMENT_ID_INVALID"
		}
		if strings.Contains(message, "sequential without gaps") {
			finding.Code = "IDS_NON_SEQUENTIAL"
		}
		if strings.Contains(message, "traces unknown delta") || strings.Contains(message, "traces duplicate delta") || strings.Contains(message, "must trace at least one delta") {
			finding.Code = "REQUIREMENT_TRACE_INVALID"
		}
		if strings.Contains(message, "gherkin tag") && strings.Contains(message, "not configured in specctl.yaml") {
			finding.Code = "REQUIREMENT_TAG_NOT_CONFIGURED"
		} else if strings.Contains(message, "gherkin ") || strings.Contains(message, "Feature line") || strings.Contains(message, "Scenario") || strings.Contains(message, "Gherkin") || strings.Contains(message, "tags must match Gherkin") {
			finding.Code = "REQUIREMENT_GHERKIN_INVALID"
		}
		if strings.Contains(message, "test_file ") || strings.Contains(message, "test file does not exist") {
			finding.Code = "REQUIREMENT_TEST_FILE_MISSING"
		}
		if strings.Contains(message, "must declare test_files when verified") {
			finding.Code = "REQUIREMENT_MANUAL_INVALID"
		}
		return finding, true
	case strings.HasPrefix(message, "name must match"):
		finding.Code = "CHARTER_NAME_INVALID"
		finding.Target = "name"
		return finding, true
	case strings.HasPrefix(message, "group key "),
		strings.HasPrefix(message, "group "),
		strings.HasPrefix(message, "duplicate group key "):
		finding.Code = "CHARTER_GROUP_INVALID"
		finding.Target = "groups"
		return finding, true
	case strings.Contains(message, "every charter group must include"):
		finding.Code = "CHARTER_GROUP_INVALID"
		finding.Target = "groups"
		return finding, true
	case strings.Contains(message, "does not have a tracking file"),
		strings.Contains(message, "tracking file is not listed in charter specs"),
		strings.Contains(message, "is not listed in charter specs"),
		strings.Contains(message, "does not list spec"):
		finding.Code = "CHARTER_SPEC_MISSING"
		finding.Target = target
		return finding, true
	case strings.Contains(message, "every charter spec must include \"notes\""),
		strings.Contains(message, "notes is required"):
		finding.Code = "CHARTER_NOTES_INVALID"
		finding.Target = target
		return finding, true
	case strings.Contains(message, "every charter spec must include"),
		strings.HasPrefix(message, "duplicate spec slug "):
		finding.Code = "CHARTER_SPEC_MISSING"
		finding.Target = target
		return finding, true
	case strings.Contains(message, "references unknown group"),
		strings.Contains(message, "charter membership for "):
		finding.Code = "CHARTER_GROUP_MISSING"
		finding.Target = target
		return finding, true
	case strings.Contains(message, "depends on unknown spec"),
		strings.Contains(message, "duplicate dependency"),
		strings.Contains(message, "cannot depend on itself"):
		finding.Code = "CHARTER_DEPENDENCY_INVALID"
		finding.Target = target
		return finding, true
	case strings.Contains(message, "dependency cycle detected"):
		finding.Code = "CHARTER_CYCLE_PRESENT"
		finding.Target = "specs"
		return finding, true
	case strings.Contains(message, "gherkin_tags "),
		strings.Contains(message, "source_prefixes"),
		strings.Contains(message, "formats."):
		finding.Code = "CONFIG_FORMAT_INVALID"
		finding.Target = "config"
		if strings.Contains(message, "gherkin_tags ") {
			finding.Code = "CONFIG_TAG_INVALID"
			finding.Target = "gherkin_tags"
		}
		if strings.Contains(message, "source_prefixes") {
			finding.Code = "CONFIG_PREFIX_INVALID"
			finding.Target = "source_prefixes"
		}
		if strings.Contains(message, "reserved semantic tag") {
			finding.Code = "REDUNDANT_SEMANTIC_TAG"
			finding.Target = "gherkin_tags"
		}
		return finding, true
	}

	return ValidationFinding{}, false
}

func sequenceFindingCode(message string, validPattern *regexp.Regexp, invalidCode string) string {
	found := sequenceFoundIdentifier(message)
	if found != "" && !validPattern.MatchString(found) {
		return invalidCode
	}
	return "IDS_NON_SEQUENTIAL"
}

func sequenceFoundIdentifier(message string) string {
	_, after, ok := strings.Cut(message, "found ")
	if !ok {
		return ""
	}
	fields := strings.Fields(strings.TrimSpace(after))
	if len(fields) == 0 {
		return ""
	}
	return strings.Trim(fields[0], ".,;:")
}
