//go:build js && wasm

package nsgo

import (
	"os"

	"github.com/oneo-me/nsgo/internal/platform/web"
)

// run fills the page with app, or the element the page names in the
// app's environment (NSGO_MOUNT, a CSS selector), so a page shows
// several apps, each a Go program of its own; NSGO_HOST names the bridge
// to the page's host that nsgo.js, the loader, made.
func run(app App, o options) {
	web.Run(app, web.Options{ID: id, Components: o.components, Fonts: o.fonts, Appearance: o.appearance,
		Mount: os.Getenv("NSGO_MOUNT"), Host: os.Getenv("NSGO_HOST")})
}
