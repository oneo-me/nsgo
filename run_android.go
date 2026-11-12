//go:build android

package nsgo

import (
	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/internal/platform/android"
)

func run(root App, app func(*core.Scope), o options) {
	updates := updateConfig()
	updates.Store = android.InstalledBy()
	root, app = withUpdateChecks(root, app, updates, false)
	android.Run(rootOf(root, app), android.Options{Components: o.components, Fonts: o.fonts, Appearance: o.appearance,
		Updates: updates})
}
