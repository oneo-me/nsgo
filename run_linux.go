//go:build linux && !android

package nsgo

import (
	"github.com/oneo-me/nsgo/core"
	"github.com/oneo-me/nsgo/internal/platform/linux"
)

func run(root App, app func(*core.Scope), o options) {
	linux.Run(rootOf(root, app), linux.Options{Title: name, Width: o.width, Height: o.height, ID: id,
		Components: o.components, Constraints: o.constraints, Fonts: o.fonts, Appearance: o.appearance, Perf: o.monitor})
}
