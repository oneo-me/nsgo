//go:build darwin && !ios

package macos

import (
	"testing"

	"github.com/oneo-me/nsgo/internal/driver"
)

func TestMaterialCode(t *testing.T) {
	cases := map[driver.WindowMaterial]int{
		driver.WindowMaterialWindow:       12,
		driver.WindowMaterialContent:      18,
		driver.WindowMaterialSidebar:      7,
		driver.WindowMaterialHeader:       10,
		driver.WindowMaterialTitlebar:     3,
		driver.WindowMaterialUnderWindow:  21,
		driver.WindowMaterialUnderPage:    22,
		driver.WindowMaterialMenu:         5,
		driver.WindowMaterialPopover:      6,
		driver.WindowMaterialHUD:          13,
		driver.WindowMaterialSheet:        11,
		driver.WindowMaterialTooltip:      17,
		driver.WindowMaterialSelection:    4,
		driver.WindowMaterialFullScreenUI: 15,
		driver.WindowMaterialMica:         21,
		driver.WindowMaterialAcrylic:      13,
		driver.WindowMaterialTabbed:       21,
	}
	for m, want := range cases {
		if got, ok := materialCode(m); !ok || got != want {
			t.Errorf("materialCode(%d) = %d, %v; want %d, true", m, got, ok, want)
		}
	}
	for _, m := range []driver.WindowMaterial{driver.WindowMaterialNone, driver.WindowMaterial(100)} {
		if code, ok := materialCode(m); ok {
			t.Errorf("materialCode(%d) = %d, true; want no material", m, code)
		}
	}
}

func TestViewsClipAsTheirStyleSays(t *testing.T) {
	if clipsBounds(nil) {
		t.Fatal("a view not yet registered clips, masking its layer for good")
	}
	plain := &view{}
	if clipsBounds(plain) {
		t.Fatal("a view whose style does not clip clips")
	}
	plain.style.Clip = true
	if !clipsBounds(plain) {
		t.Fatal("a view whose style clips does not")
	}
}
