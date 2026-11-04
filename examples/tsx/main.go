// Command tsx is an app written in TSX and styled with Tailwind CSS
// classes: today's tasks and a focus timer. tsx/compiler compiles the
// embedded sources as it starts.
package main

import (
	"embed"

	"github.com/oneo-me/nsgo"
	"github.com/oneo-me/nsgo/tsx/compiler"
)

//go:embed *.tsx *.ts
var src embed.FS

// Root is the app: what App.tsx mounts.
var Root = compiler.App(src, "App.tsx")

func main() { nsgo.Run(Root, nsgo.WindowSize(960, 720)) }
