package updater

import (
	"errors"

	"github.com/nsgo-dev/nsgo/core"
)

var errInstallerNotStarted = errors.New("updater: platform has no self-installer")

// Install downloads' verified package through the platform's installer.
// Linux asks its package manager for elevation; Android opens the package
// installer. The callback runs on the app's graph goroutine.
func Install(cx *core.Scope, path string, done func(error)) {
	core.InstallUpdate(cx, path, done)
}
