//go:build darwin && !ios

package nsgo

import (
	"github.com/oneo-me/nsgo/core"
	"github.com/oneo-me/nsgo/internal/platform/macos"
)

// run runs the app on its own surface, which opens its windows.
func run(root App, app func(*core.Scope), o options) {
	macos.Run(core.AppRoot(appOf(root, app)), macos.Options{
		Title: name, Width: o.width, Height: o.height, ID: id,
		Components: o.components, Perf: o.monitor, Material: o.material, Fonts: o.fonts,
		Appearance: o.appearance, HideDock: o.hideDock,
	})
}
