package hover

import (
	"slices"
	"testing"

	"github.com/nsgo-dev/nsgo/internal/driver"
)

func TestMove(t *testing.T) {
	var tr Tracker
	ev := func(k driver.EventKind, id driver.NodeID) driver.Event { return driver.Event{Kind: k, ID: id} }
	steps := []struct {
		under []driver.NodeID
		want  []driver.Event
	}{
		{[]driver.NodeID{1, 2}, []driver.Event{ev(driver.EventMouseEnter, 1), ev(driver.EventMouseEnter, 2)}},
		{[]driver.NodeID{1, 2}, nil},
		{[]driver.NodeID{1, 3, 4}, []driver.Event{ev(driver.EventMouseLeave, 2), ev(driver.EventMouseEnter, 3), ev(driver.EventMouseEnter, 4)}},
		{nil, []driver.Event{ev(driver.EventMouseLeave, 4), ev(driver.EventMouseLeave, 3), ev(driver.EventMouseLeave, 1)}},
	}
	for i, s := range steps {
		if got := tr.Move(s.under); !same(got, s.want) {
			t.Fatalf("step %d: got %v, want %v", i, got, s.want)
		}
	}
	tr.Move([]driver.NodeID{1, 2})
	tr.Forget(2)
	if got := tr.Move(nil); !same(got, []driver.Event{ev(driver.EventMouseLeave, 1)}) {
		t.Fatalf("after Forget: got %v", got)
	}
}

func TestPressedNamesTheFirstButtonHeld(t *testing.T) {
	bits := [5]uint32{1, 2, 4}
	for state, want := range map[uint32]driver.MouseButton{0: driver.MouseButtonNone, 1: driver.MouseButtonLeft,
		2: driver.MouseButtonRight, 4: driver.MouseButtonMiddle, 6: driver.MouseButtonRight, 8: driver.MouseButtonNone} {
		if got := Pressed(state, bits); got != want {
			t.Errorf("Pressed(%d) = %v, want %v", state, got, want)
		}
	}
}

func same(a, b []driver.Event) bool {
	return slices.EqualFunc(a, b, func(x, y driver.Event) bool { return x.Kind == y.Kind && x.ID == y.ID })
}
