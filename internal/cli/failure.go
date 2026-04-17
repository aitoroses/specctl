package cli

import "github.com/aitoroses/specctl/internal/presenter"

func applicationError(err error) error {
	return presenter.ApplicationError(err)
}
