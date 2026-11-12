//go:build ios && !nsgoembed && !nsgowatch

package nsgo

import (
	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/internal/platform/ios"
)

func run(root App, app func(*core.Scope), o options) {
	updates := updateConfig()
	updates.Store = ios.InstalledBy()
	root, app = withUpdateChecks(root, app, updates, true)
	ios.Run(rootOf(root, app), ios.Options{ID: id, Components: o.components, Fonts: o.fonts, Appearance: o.appearance, Perf: o.monitor,
		Updates: updates})
}
