package base

import (
	"slices"
	"testing"

	"github.com/nsgo-dev/nsgo/core"
)

// measured is panels at sizes in a container of their sum, gpui-base's
// default range each.
func measured(sizes ...core.Points) *resizeState {
	s := &resizeState{sizes: sizes}
	for range sizes {
		s.ranges = append(s.ranges, [2]core.Points{PanelMinSize, PanelMaxSize})
		s.fixed = append(s.fixed, false)
	}
	for _, x := range sizes {
		s.container += x
	}
	return s
}

func TestResizeStateFollowsGpuiBase(t *testing.T) {
	s := measured(150, 250)
	s.ranges[0] = [2]core.Points{120, 300}
	s.resizeAtHandle(0, 380)
	if want := []core.Points{300, 100}; !slices.Equal(s.sizes, want) {
		t.Errorf("clamped to its maximum: %v, want %v", s.sizes, want)
	}
	s.resizeAtHandle(0, 10)
	if want := []core.Points{120, 280}; !slices.Equal(s.sizes, want) {
		t.Errorf("clamped to its minimum: %v, want %v", s.sizes, want)
	}

	s = measured(200, 150, 150)
	s.resizeAtHandle(0, 320)
	if want := []core.Points{300, 100, 100}; !slices.Equal(s.sizes, want) {
		t.Errorf("growing takes from the panels after in turn: %v, want %v", s.sizes, want)
	}

	s = measured(200, 200, 200)
	s.resizeAtHandle(1, 50)
	if want := []core.Points{150, 100, 350}; !slices.Equal(s.sizes, want) {
		t.Errorf("shrinking past the minimum takes from the panels before: %v, want %v", s.sizes, want)
	}

	s = measured(240, 240, 120)
	s.fixed[0] = true
	s.resizeAtHandle(1, 50)
	if want := []core.Points{240, 100, 260}; !slices.Equal(s.sizes, want) {
		t.Errorf("a fixed panel before gives nothing: %v, want %v", s.sizes, want)
	}
}
