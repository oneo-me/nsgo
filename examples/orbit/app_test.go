package main

import (
	"testing"

	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/internal/driver"
	"github.com/nsgo-dev/nsgo/nsgotest"
)

// TestMain hands the main thread to the platform under nsgo test -target.
func TestMain(m *testing.M) { nsgotest.Main(m) }

func start(t *testing.T) *nsgotest.App {
	if target := nsgotest.Target(); target == "ios_simulator" || target == "android" {
		t.Skipf("needs a desktop window, which %s has not", target)
	}
	return nsgotest.Start(t, Root, nsgotest.Size(1180, 800),
		nsgotest.Environment(core.Environment{OS: core.OSMacOS, Idiom: core.IdiomDesktop, Locale: "en-US"}))
}

// front checks that card i alone lets its controls take the pointer, on
// top of the others,
// and that the header counts it.
func front(t *testing.T, app *nsgotest.App, i int) {
	t.Helper()
	app.Settle()
	id := func(j int) string { return "id:card-" + string(rune('0'+j)) }
	for j := range instruments {
		takes := app.Find(id(j)).Props.PointerEvents != driver.PointerEventsBoxOnly
		if takes != (j == i) {
			t.Errorf("card %d takes the pointer: %v, with card %d in front", j, takes, i)
		}
	}
	cards := app.Find(id(i)).Parent.Children
	if top := cards[len(cards)-1]; top != app.Find(id(i)) {
		t.Errorf("card %d is not stacked on top", i)
	}
	if got, want := app.Find("id:index").Text, string([]rune{'0', rune('1' + i)}); got != want {
		t.Errorf("index = %q, want %q", got, want)
	}
}

func TestKeysTurnTheRing(t *testing.T) {
	app := start(t)
	front(t, app, 0)
	app.Key("right")
	front(t, app, 1)
	app.Key("left")
	app.Key("left")
	front(t, app, 5)
}

func TestTheDockTurnsTheShorterWay(t *testing.T) {
	app := start(t)
	app.Press("id:chip-4")
	front(t, app, 4)
	app.Press("id:next")
	app.Press("id:next")
	front(t, app, 0)
	app.Press("id:prev")
	front(t, app, 5)
}

func TestADragTurnsTheRing(t *testing.T) {
	app := start(t)
	// Dragging left turns the next cards in: two cards' worth.
	app.Pan("id:space", driver.Point{X: -100}, driver.Point{X: -2 * step / dragRate})
	front(t, app, 2)
	app.Pan("id:space", driver.Point{X: 80}, driver.Point{X: step / dragRate})
	front(t, app, 1)
}

func TestTypesetFollowsTheField(t *testing.T) {
	app := start(t)
	app.Press("id:chip-1")
	app.Clear("id:typeset-input")
	app.Type("id:typeset-input", "Orbit")
	if got := app.Find("id:typeset-preview").Text; got != "Orbit" {
		t.Errorf("preview = %q, want Orbit", got)
	}
	app.Clear("id:typeset-input")
	if got := app.Find("id:typeset-preview").Text; got != "…" {
		t.Errorf("empty preview = %q, want …", got)
	}
}

func TestSwitchesLightTheMeter(t *testing.T) {
	app := start(t)
	app.Press("id:chip-3")
	if !app.Exists("text:2 of 4") {
		t.Fatalf("two channels start live:\n%s", app.Dump())
	}
	app.Press("label:Beacon")
	if !app.Exists("text:3 of 4") {
		t.Errorf("Beacon did not go live:\n%s", app.Dump())
	}
}

func TestTheButtonTakesPressesUnderTheGlass(t *testing.T) {
	app := start(t)
	app.Press("id:chip-4")
	app.Press("id:loupe-press")
	app.Press("id:loupe-press")
	if !app.Exists("text:2") {
		t.Errorf("presses were not counted:\n%s", app.Dump())
	}
}


func TestASwipeTurnsTheRing(t *testing.T) {
	headless(t)
	app := start(t)
	// Two cards' worth to the left, then one back to the right.
	app.TouchpadScroll("id:orbit", driver.Point{}, driver.Point{X: -20}, driver.Point{X: -2*step/dragRate + 20})
	front(t, app, 2)
	app.TouchpadScroll("id:orbit", driver.Point{}, driver.Point{X: 40}, driver.Point{X: step/dragRate - 40})
	front(t, app, 1)
	// A swipe along the window leaves the ring alone.
	app.TouchpadScroll("id:orbit", driver.Point{}, driver.Point{Y: -40}, driver.Point{X: -300, Y: -400})
	front(t, app, 1)
}

// headless skips a test of events the headless model makes, which a
// platform's own gestures replace under nsgo test -target.
func headless(t *testing.T) {
	if target := nsgotest.Target(); target != "" {
		t.Skipf("checks the headless model's events, not %s's", target)
	}
}
