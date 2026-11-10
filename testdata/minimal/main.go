// Command minimal shows one text: linked_test.go checks what an app links
// for no more than that.
package main

import (
	"github.com/oneo-me/nsgo"
	"github.com/oneo-me/nsgo/core"
)

func main() {
	nsgo.Run(func(cx *core.Scope) core.Node { return core.Text{Content: "Hello"} })
}
