//go:build linux && !android

package nsgo

import (
	"github.com/nsgo-dev/nsgo/internal/engine"
	"github.com/nsgo-dev/nsgo/internal/platform/linux"
)

func newView(app App, o options, h engine.HostOptions) platformView {
	v, err := linux.NewView(app, linux.Options{Title: name, ID: id, Components: o.components, Fonts: o.fonts,
		Appearance: o.appearance, Host: h})
	if err != nil {
		panic("nsgo: " + err.Error())
	}
	return v
}
