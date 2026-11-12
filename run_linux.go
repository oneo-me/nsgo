//go:build linux && !android

package nsgo

import (
	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/internal/platform/linux"
)

func run(root App, app func(*core.Scope), o options) {
	updates := updateConfig()
	updates.Store = linux.InstalledBy()
	root, app = withUpdateChecks(root, app, updates, false)
	linux.Run(rootOf(root, app), linux.Options{Title: name, Width: o.width, Height: o.height, ID: id,
		Components: o.components, Constraints: o.constraints, Fonts: o.fonts, Appearance: o.appearance, Perf: o.monitor,
		Updates: updates})
}
