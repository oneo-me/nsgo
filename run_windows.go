//go:build windows

package nsgo

import (
	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/internal/platform/windows"
)

func run(root App, app func(*core.Scope), o options) {
	windows.Run(rootOf(root, app), windows.Options{Title: name, Width: o.width, Height: o.height, Frame: o.frame, ID: id,
		Components: o.components, Material: o.material, Constraints: o.constraints, Fonts: o.fonts, Appearance: o.appearance, Perf: o.monitor,
		Updates: updateConfig()})
}
