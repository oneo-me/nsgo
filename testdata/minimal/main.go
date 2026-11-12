// Command minimal shows one text: linked_test.go checks what an app links
// for no more than that.
package main

import (
	"github.com/nsgo-dev/nsgo"
	"github.com/nsgo-dev/nsgo/core"
)

func main() {
	nsgo.Run(func(cx *core.Scope) core.Node { return core.Text{Content: "Hello"} })
}
