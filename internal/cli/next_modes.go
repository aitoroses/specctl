package cli

import (
	"github.com/aitoroses/specctl/internal/application"
	"github.com/aitoroses/specctl/internal/presenter"
)

func contextNextDirective(state any, next []any) nextDirective {
	return nextDirectiveForReadMode(application.ReadSurfaceNextMode(state, next), next)
}

func diffNextDirective(state any, next []any) nextDirective {
	return nextDirectiveForReadMode(application.ReadSurfaceNextMode(state, next), next)
}

func nextDirectiveForReadMode(mode string, next []any) nextDirective {
	return presenter.DirectiveForReadMode(mode, next)
}
