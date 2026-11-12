//go:build windows

package nsgo

import (
	"github.com/nsgo-dev/nsgo/internal/engine"
	"github.com/nsgo-dev/nsgo/internal/platform/windows"
)

func newView(app App, o options, h engine.HostOptions) platformView {
	v, err := windows.NewView(app, windows.Options{Title: name, ID: id, Components: o.components, Fonts: o.fonts,
		Appearance: o.appearance, Host: h, Parent: o.parent})
	if err != nil {
		panic("nsgo: " + err.Error())
	}
	return v
}
