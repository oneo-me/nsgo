package ui

import (
	"reflect"
	"testing"

	"github.com/oneo-me/nsgo/icons/lucide"
)

// TestFileIcon chooses a file's glyph and color by its whole name, then
// its extension, from the last element of a path.
func TestFileIcon(t *testing.T) {
	for _, c := range []struct {
		name string
		want fileKind
	}{
		{"src/go.mod", fileNames["go.mod"]},
		{`C:\repo\README.zh-CN.md`, fileNames["readme.md"]},
		{".env.local", fileNames[".env"]},
		{"main_test.go", fileKind{lucide.FileCheck, fileGo}},
		{"App.TSX", fileExtensions[".tsx"]},
		{"notes", fileKind{lucide.File, 0}},
	} {
		got := fileKindOf(c.name)
		if got.color != c.want.color || reflect.ValueOf(got.glyph).Pointer() != reflect.ValueOf(c.want.glyph).Pointer() {
			t.Errorf("%s: got %+v, want %+v", c.name, got, c.want)
		}
	}
}
