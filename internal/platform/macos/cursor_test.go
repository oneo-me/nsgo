//go:build darwin && !ios

package macos

import (
	"testing"

	"github.com/ebitengine/purego"

	"github.com/nsgo-dev/nsgo/internal/driver"
)

// TestDiagonalCursors asks AppKit for the frame cursors with the values
// NSCursor.h gives them.
func TestDiagonalCursors(t *testing.T) {
	if _, err := purego.Dlopen(appKitPath, purego.RTLD_NOW|purego.RTLD_GLOBAL); err != nil {
		t.Fatal(err)
	}
	pointer := nsCursor(driver.CursorPointer)
	nwse, nesw := nsCursor(driver.CursorNwseResize), nsCursor(driver.CursorNeswResize)
	if nwse == 0 || nesw == 0 || nwse == nesw || nwse == pointer || nesw == pointer {
		t.Errorf("pointer %v, nwse %v, nesw %v", pointer, nwse, nesw)
	}
}
