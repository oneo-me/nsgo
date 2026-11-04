package tsx_test

import (
	"os"
	"path/filepath"
	"testing"
)

// nshDir is the path of NSH's checkout joined with elem: $NSH_DIR when
// set, else nsh/ beside the repository, else nsh/ inside it. It skips the
// test when that path does not exist.
func nshDir(t *testing.T, elem ...string) string {
	t.Helper()
	roots := []string{filepath.Join("..", "..", "nsh"), filepath.Join("..", "nsh")}
	if dir := os.Getenv("NSH_DIR"); dir != "" {
		roots = []string{dir}
	}
	for _, root := range roots {
		p := filepath.Join(append([]string{root}, elem...)...)
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	t.Skip("NSH is not checked out beside the repository (see README.md#development)")
	return ""
}
