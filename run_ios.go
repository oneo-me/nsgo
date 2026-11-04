//go:build ios && !nsgoembed

package nsgo

import (
	"github.com/oneo-me/nsgo/core"
	"github.com/oneo-me/nsgo/internal/platform/ios"
)

func run(root App, app func(*core.Scope), o options) {
	ios.Run(rootOf(root, app), ios.Options{ID: id, Components: o.components, Fonts: o.fonts, Appearance: o.appearance, Perf: o.monitor})
}
