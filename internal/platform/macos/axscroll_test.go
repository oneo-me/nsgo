//go:build darwin && !ios

package macos

import (
	"testing"

	"github.com/nsgo-dev/nsgo/internal/driver"
)

// A scroll bar's value is where the view is scrolled to along its axis,
// from 0 at the content's start to 1 at its end.
func TestScrollBarValues(t *testing.T) {
	v := &view{kind: driver.KindScroll, frame: driver.Bounds{Size: driver.Size{Width: 100, Height: 200}},
		sc: &scrollState{content: driver.Size{Width: 100, Height: 600}}}
	for _, c := range []struct {
		at   driver.Points
		want float64
	}{{0, 0}, {200, 0.5}, {400, 1}, {-30, 0}, {450, 1}} {
		v.sc.at.Y = c.at
		if got := v.axScrolled(); got != c.want {
			t.Errorf("at %v: %v, want %v", c.at, got, c.want)
		}
	}
	v.sc.config.Horizontal, v.sc.content = true, driver.Size{Width: 400, Height: 200}
	v.sc.at = driver.Point{X: 150}
	if got := v.axScrolled(); got != 0.5 {
		t.Errorf("horizontal: %v, want 0.5", got)
	}
	v.sc.content.Width = 50
	if got := v.axScrolled(); got != 0 {
		t.Errorf("content shorter than the view: %v", got)
	}
	if r, ok := v.axScrollRole(); !ok || r[0] != "AXScrollArea" {
		t.Errorf("role %v", r)
	}
}
