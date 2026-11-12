package finger

import (
	"testing"

	"github.com/nsgo-dev/nsgo/internal/driver"
)

func TestMouseIsFingerZeroWhileAViewListens(t *testing.T) {
	var m Mouse
	if _, ok := m.Down(false, driver.Point{}); ok {
		t.Fatal("a press reported while no view listens")
	}
	if _, ok := m.Move(driver.Point{X: 1}); ok {
		t.Fatal("a move reported after an unreported press")
	}
	ev, ok := m.Down(true, driver.Point{X: 5, Y: 6})
	if !ok || ev.Phase != driver.GestureBegan || ev.Touches[0].PagePosition != (driver.Point{X: 5, Y: 6}) || !ev.Touches[0].Changed {
		t.Fatalf("down = %+v, %v", ev, ok)
	}
	if ev, ok := m.Move(driver.Point{X: 7}); !ok || ev.Phase != driver.GestureChanged {
		t.Fatalf("move = %+v, %v", ev, ok)
	}
	if ev, ok := m.Up(driver.Point{X: 7}); !ok || ev.Phase != driver.GestureEnded {
		t.Fatalf("up = %+v, %v", ev, ok)
	}
	if _, ok := m.Up(driver.Point{}); ok {
		t.Fatal("a second up reported")
	}
}
