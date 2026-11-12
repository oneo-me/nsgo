package ui

import (
	"testing"

	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/icons/lucide"
)

func TestReadIcon(t *testing.T) {
	Icons(map[string]func() *core.SVG{"plus": lucide.Plus})
	if readIcon("plus") != lucide.Plus() {
		t.Error("a name reads as the icon Icons gave")
	}
	if readIcon(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 4 4"><rect width="4" height="4"/></svg>`) == nil {
		t.Error("an SVG document reads as an icon")
	}
	if readIcon("unknown") != nil || readIcon(nil) != nil {
		t.Error("what names no icon reads as none")
	}
}
