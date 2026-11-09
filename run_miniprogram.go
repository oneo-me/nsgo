//go:build js && wasm && nsgominiprogram

package nsgo

import (
	"github.com/oneo-me/nsgo/core"
	"github.com/oneo-me/nsgo/internal/platform/miniprogram"
)

// run fills the mini program's page with app.
func run(root App, app func(*core.Scope), o options) {
	miniprogram.Run(rootOf(root, app), miniprogram.Options{ID: id, Appearance: o.appearance})
}
