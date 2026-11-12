//go:build !nsgodev

package compiler

import (
	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/tsx"
)

// run runs bundle code.
func run(cx *core.Scope, entry, code string) core.Node { return tsx.App(entry, code)(cx) }
