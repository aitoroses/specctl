package infrastructure

func ClassifyDriftSource(docChanged, codeChanged bool) string {
	switch {
	case docChanged && codeChanged:
		return "both"
	case docChanged:
		return "design_doc"
	case codeChanged:
		return "scope_code"
	default:
		return ""
	}
}
