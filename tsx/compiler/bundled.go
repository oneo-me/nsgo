//go:build nsgo_bundled

package compiler

import (
	"fmt"
	"io/fs"
)

// compileAtStart fails: the nsgo tool builds apps whose bundles it
// compiled ahead without the compiler, and gave none for entry.
func compileAtStart(_ fs.FS, entry string) (string, error) {
	return "", fmt.Errorf("compiler: %s was not compiled ahead, and the app was built without the compiler", entry)
}
