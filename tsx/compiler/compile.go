//go:build !nsgo_bundled

package compiler

import (
	"io/fs"

	"github.com/oneo-me/nsgo/tsx/bundler"
)

// compileAtStart compiles an app's TSX as it starts, without a bundle
// compiled ahead.
func compileAtStart(fsys fs.FS, entry string) (string, error) { return bundler.Compile(fsys, entry) }
