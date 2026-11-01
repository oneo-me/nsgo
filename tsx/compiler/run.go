//go:build !nsgodev

package compiler

import (
	"github.com/oneo-me/nsgo/core"
	"github.com/oneo-me/nsgo/tsx"
)

// run runs bundle code.
func run(cx *core.Scope, entry, code string) core.Node { return tsx.App(entry, code)(cx) }
