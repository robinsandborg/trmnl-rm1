//go:build !linux

package trmnl

import "errors"

func (a *App) runInstall(paths Paths, args []string) error {
	return errors.New("install-appliance is only supported on Linux")
}

func (a *App) runRestore(paths Paths) error {
	return errors.New("restore-stock is only supported on Linux")
}
