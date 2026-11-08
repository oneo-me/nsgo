// Command gallery-tsx is the gallery's Components written in TSX: a page
// per family of components, on nsgo/ui and nsgo/base, with knobs that set
// their props. tsx/ui gives TSX package ui's components, so they look as
// they do in Go; tsx/compiler compiles the embedded sources as it starts.
package main

import (
	"embed"

	"github.com/oneo-me/nsgo"
	"github.com/oneo-me/nsgo/core"
	"github.com/oneo-me/nsgo/desktop"
	"github.com/oneo-me/nsgo/tsx/compiler"
	_ "github.com/oneo-me/nsgo/tsx/editor"
	_ "github.com/oneo-me/nsgo/tsx/terminal"
	_ "github.com/oneo-me/nsgo/tsx/ui/chat"
	_ "github.com/oneo-me/nsgo/tsx/ui/richeditor"
	_ "github.com/oneo-me/nsgo/tsx/ui"
	"github.com/oneo-me/nsgo/ui"
)

//go:embed *.tsx *.ts demos/*.tsx demos/*.ts
var src embed.FS

var app = compiler.App(src, "App.tsx")

// Root is the app in ui's theme, whose roles its components read.
func Root(cx *core.Scope) core.Node {
	cx.SetTheme(ui.DefaultTheme())
	return app(cx)
}

func main() {
	nsgo.Run(Root, nsgo.WindowSize(1080, 760), nsgo.WindowMaterial(desktop.WindowMaterialSidebar))
}
