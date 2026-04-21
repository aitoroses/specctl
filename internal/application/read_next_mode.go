package application

func ReadSurfaceNextMode(state any, next []any) string {
	switch projection := state.(type) {
	case MissingCharterContext:
		return "choose_one"
	case MissingSpecContext:
		if !projection.CharterExists && len(next) > 1 {
			return "sequence"
		}
		return "choose_one"
	case FileContextProjection:
		if projection.Resolution == "unmatched" {
			return "choose_one"
		}
		if projection.Resolution == "ambiguous" {
			return "none"
		}
		return "sequence"
	case SpecProjection:
		switch firstReadAction(next) {
		case "refresh_requirement":
			return "choose_one"
		case "delta_add_repair":
			return "choose_then_sequence"
		case "review_warnings":
			if projection.ScopeDrift.Status == "clean" && len(projection.UncommittedChanges) == 0 {
				return "sequence"
			}
		}
		switch projection.ScopeDrift.Status {
		case "clean":
			if len(projection.UncommittedChanges) == 0 {
				return "none"
			}
			return "sequence"
		case "tracked", "unavailable":
			return "sequence"
		case "drifted":
			if len(next) > 1 {
				return "choose_one"
			}
			return "sequence"
		default:
			return "sequence"
		}
	case SpecDiffProjection:
		if projection.DriftSource == nil {
			return "sequence"
		}
		if !isReadSemanticChoiceSet(next) {
			return "sequence"
		}
		switch *projection.DriftSource {
		case "design_doc", "both":
			if len(next) > 0 {
				return "choose_then_sequence"
			}
		case "scope_code":
			if len(next) > 0 {
				return "choose_one"
			}
		}
		return "sequence"
	default:
		return "sequence"
	}
}

func firstReadAction(next []any) string {
	if len(next) == 0 {
		return ""
	}
	action, ok := next[0].(map[string]any)
	if !ok {
		return ""
	}
	name, _ := action["action"].(string)
	return name
}

func isReadSemanticChoiceSet(next []any) bool {
	switch firstReadAction(next) {
	case "delta_add_add", "delta_add_change", "delta_add_remove", "delta_add_repair", "refresh_requirement", "sync":
		return true
	default:
		return false
	}
}
