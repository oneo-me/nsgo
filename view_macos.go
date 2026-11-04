//go:build darwin && !ios

package nsgo

import (
	"github.com/oneo-me/nsgo/internal/engine"
	"github.com/oneo-me/nsgo/internal/platform/macos"
)

func newView(app App, o options, h engine.HostOptions) platformView {
	return macos.NewView(app, macos.Options{Title: name, Width: o.width, Height: o.height, ID: id, Components: o.components,
		Fonts: o.fonts, Appearance: o.appearance, Host: h})
}
