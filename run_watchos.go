//go:build ios && nsgowatch

package nsgo

import (
	"github.com/oneo-me/nsgo/core"
	"github.com/oneo-me/nsgo/internal/platform/watchos"
)

func run(root App, app func(*core.Scope), o options) {
	watchos.Run(rootOf(root, app), watchos.Options{ID: id, Fonts: o.fonts, Appearance: o.appearance})
}
