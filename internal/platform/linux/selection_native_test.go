//go:build linux && !android

package linux

import (
	"testing"
	"time"

	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/internal/driver"
	"github.com/nsgo-dev/nsgo/internal/engine"
)

// TestSelectionAreaSelectsAcrossTexts double-clicks a word of a selection
// area, extends the selection into the next text with Shift, selects all
// with Ctrl+A and clears with Escape.
func TestSelectionAreaSelectsAcrossTexts(t *testing.T) {
	if testing.Short() {
		t.Skip("opens a window")
	}
	title := "nsgo-select"
	app := func(cx *core.Scope) core.Node {
		cx.SetTitle(title + "|ready")
		return core.View{Selectable: true, Style: core.Style{Width: driver.Pt(320), Height: driver.Pt(200), Padding: driver.All(10),
			Gap: 10, Background: driver.RGB(0xffffff)}, Children: []core.Node{
			core.Text{Content: "Hello world", Style: core.Style{FontSize: 16}},
			core.Text{Content: "Second line", Style: core.Style{FontSize: 16}},
		}}
	}
	w := startTestWindow(t, app, Options{Title: title, Width: 320, Height: 200})
	waitForTitle(t, w, title+"|ready")
	text := func(content string) *view {
		for _, v := range w.views {
			if v != nil && v.kind == driver.KindText && v.plain == content {
				return v
			}
		}
		t.Errorf("no text %q", content)
		return nil
	}
	w.onLoop(t, func() {
		first, second := text("Hello world"), text("Second line")
		if first == nil || second == nil {
			return
		}
		b, _ := w.surfaceBounds(first)
		at := driver.Point{X: b.Origin.X + 10, Y: b.Origin.Y + b.Size.Height/2}
		w.pointerDown(at, 1, 0, 0, 2)
		w.pointerUp(at)
		if got := w.selecting.Model.Text(w.selecting); got != "Hello" {
			t.Errorf("double click selected %q, want Hello", got)
		}
		b, _ = w.surfaceBounds(second)
		at = driver.Point{X: b.Origin.X + b.Size.Width - 2, Y: b.Origin.Y + b.Size.Height/2}
		w.pointerDown(at, 1, 0, gdkShift, 1)
		w.pointerUp(at)
		if got := w.selecting.Model.Text(w.selecting); got != "Hello world\nSecond line" {
			t.Errorf("Shift-click selected %q", got)
		}
		if first.selStart != 0 || first.selEnd != 11 || second.selEnd != 11 {
			t.Errorf("highlights %d-%d and %d-%d", first.selStart, first.selEnd, second.selStart, second.selEnd)
		}
		w.selectKey(gdkEscape, 0)
		if w.selecting != nil || first.selEnd != 0 {
			t.Error("Escape left the selection")
		}
		w.pointerDown(at, 1, 0, 0, 1)
		w.pointerUp(at)
		w.selectKey('a', gdkControl)
		if got := w.selecting.Model.Text(w.selecting); got != "Hello world\nSecond line" {
			t.Errorf("Ctrl+A selected %q", got)
		}
	})
}

// TestModalSlidesIn opens a modal, which slides up from below its frame
// and rests there.
func TestModalSlidesIn(t *testing.T) {
	if testing.Short() {
		t.Skip("opens a window")
	}
	title := "nsgo-modal"
	var open *core.Signal[bool]
	var root *core.Scope
	app := func(cx *core.Scope) core.Node {
		root = cx
		open = core.NewSignal(cx, false)
		cx.SetTitle(title + "|ready")
		return core.View{Style: core.Style{Width: driver.Pt(320), Height: driver.Pt(200), Background: driver.RGB(0xffffff)}, Children: []core.Node{
			core.Show{When: open.Get, Then: func(*core.Scope) core.Node {
				return core.Modal{Children: []core.Node{core.View{Style: core.Style{FlexGrow: 1, Background: driver.RGB(0x2563eb)}}}}
			}},
		}}
	}
	w := startTestWindow(t, app, Options{Title: title, Width: 320, Height: 200})
	waitForTitle(t, w, title+"|ready")
	var modal *view
	root.Post(func() { open.Set(true) })
	deadline := time.Now().Add(3 * time.Second)
	for modal == nil && time.Now().Before(deadline) {
		w.onLoop(t, func() {
			for _, v := range w.views {
				if v != nil && v.kind == driver.KindModal {
					modal = v
				}
			}
		})
		time.Sleep(10 * time.Millisecond)
	}
	if modal == nil {
		t.Fatal("no modal")
	}
	var shown, rest driver.Bounds
	w.onLoop(t, func() { shown, rest = modal.shownFrame(), modal.frame })
	if shown.Origin.Y <= rest.Origin.Y {
		t.Errorf("the modal shows at %v as it opens, want below %v", shown, rest)
	}
	time.Sleep(500 * time.Millisecond)
	w.onLoop(t, func() { shown = modal.shownFrame() })
	if shown != rest {
		t.Errorf("the modal rests at %v, want %v", shown, rest)
	}
}

// TestBlendModeMixesWithItsParent draws a cyan view multiplied over its
// yellow parent, which shows green, beside a normal one.
func TestBlendModeMixesWithItsParent(t *testing.T) {
	if testing.Short() {
		t.Skip("opens a window")
	}
	title := "nsgo-blend"
	app := func(cx *core.Scope) core.Node {
		cx.SetTitle(title + "|ready")
		return core.View{Style: core.Style{Width: driver.Pt(200), Height: driver.Pt(100), Background: driver.RGB(0xffff00),
			Direction: core.Row}, Children: []core.Node{
			core.View{Style: core.Style{Width: driver.Pt(50), Height: driver.Pt(50), Background: driver.RGB(0x00ffff), BlendMode: core.BlendMultiply}},
			core.View{Style: core.Style{Width: driver.Pt(50), Height: driver.Pt(50), Background: driver.RGB(0x00ffff)}},
		}}
	}
	w := startTestWindow(t, app, Options{Title: title, Width: 200, Height: 100})
	waitForTitle(t, w, title+"|ready")
	waitPixels(t, w, "green under the multiplied view, cyan under the other", func(p pixels) bool {
		g, c := p.at(25, 25), p.at(75, 25) // BGRA
		return g[0] < 30 && g[1] > 225 && g[2] < 30 && c[0] > 225 && c[1] > 225 && c[2] < 30
	})
}

// TestTouchDragsScrollAndPullsToRefresh drags a scroll view with a finger,
// which glides once let go, then pulls it past its top, which refreshes
// and keeps room above the content until the app is done; a value bound
// to the pull follows it.
func TestTouchDragsScrollAndPullsToRefresh(t *testing.T) {
	if testing.Short() {
		t.Skip("opens a window")
	}
	title := "nsgo-touch"
	var refreshing *core.Signal[bool]
	var root *core.Scope
	refreshed := make(chan bool, 1)
	app := func(cx *core.Scope) core.Node {
		root = cx
		refreshing = core.NewSignal(cx, false)
		cx.SetTitle(title + "|ready")
		var rows []core.Node
		for range 40 {
			rows = append(rows, core.View{Style: core.Style{Height: driver.Pt(40), Background: driver.RGB(0xdddddd)}})
		}
		return core.Scroll{Style: core.Style{Width: driver.Pt(200), Height: driver.Pt(300)}, Children: rows,
			OnRefresh: func() { refreshing.Set(true); refreshed <- true }, Refreshing: refreshing.Get}
	}
	w := startTestWindow(t, app, Options{Title: title, Width: 200, Height: 300})
	waitForTitle(t, w, title+"|ready")
	var scroll *view
	w.onLoop(t, func() {
		for _, v := range w.views {
			if v != nil && v.kind == driver.KindScroll {
				scroll = v
			}
		}
	})
	if scroll == nil {
		t.Fatal("no scroll view")
	}
	drag := func(from driver.Point, dys ...driver.Points) {
		w.onLoop(t, func() {
			w.touchDown(from)
			for _, dy := range dys {
				from.Y += dy
				time.Sleep(10 * time.Millisecond)
				w.touchMove(from)
			}
			w.touchUp()
		})
	}
	drag(driver.Point{X: 100, Y: 250}, -20, -20, -20, -20, -20)
	var at driver.Point
	w.onLoop(t, func() { at = scroll.offset })
	if at.Y < 70 {
		t.Fatalf("offset %v after an 80-point drag past the slop", at)
	}
	time.Sleep(time.Second)
	var rest driver.Point
	w.onLoop(t, func() { rest = scroll.offset })
	if rest.Y <= at.Y {
		t.Errorf("the view rests at %v, want past %v: no glide", rest, at)
	}
	// Back to the top, then a pull past it.
	w.onLoop(t, func() { w.scrollTo(scroll, driver.Point{}) })
	drag(driver.Point{X: 100, Y: 50}, 20, 40, 40, 40, 40)
	select {
	case <-refreshed:
	case <-time.After(3 * time.Second):
		t.Fatal("the pull did not refresh")
	}
	time.Sleep(500 * time.Millisecond)
	w.onLoop(t, func() { rest = scroll.offset })
	if want := -scroll.scroll.Behavior.RefreshInset(); rest.Y != want {
		t.Errorf("refreshing, the view rests at %v, want %v", rest.Y, want)
	}
	root.Post(func() { refreshing.Set(false) })
	time.Sleep(600 * time.Millisecond)
	w.onLoop(t, func() { rest = scroll.offset })
	if rest.Y != 0 {
		t.Errorf("done refreshing, the view rests at %v, want 0", rest.Y)
	}
}

// TestEmbeddedViewScopesItsAppearance makes a view for a host in dark,
// whose controls take a style scoped to the view, and back.
func TestEmbeddedViewScopesItsAppearance(t *testing.T) {
	if testing.Short() {
		t.Skip("opens a window")
	}
	title := "nsgo-scheme"
	app := func(cx *core.Scope) core.Node {
		cx.SetTitle(title + "|ready")
		return core.View{}
	}
	w := startTestWindow(t, app, Options{Title: title, Width: 100, Height: 100})
	waitForTitle(t, w, title+"|ready")
	w.onLoop(t, func() {
		v, err := NewView(func(cx *core.Scope) core.Node { return core.Input{} }, Options{Appearance: driver.AppearanceDark})
		if err != nil {
			t.Fatal(err)
		}
		e := v.w
		if e.schemeProvider == 0 || e.surfaceName == "" {
			t.Fatal("a view forced dark has no scoped style")
		}
		v.SetHostEnvironment(engine.HostEnvironment{Appearance: driver.AppearanceLight})
		if e.scoped != driver.AppearanceDark {
			t.Errorf("the host's light won over the view's dark: %v", e.scoped)
		}
		e.forced = driver.AppearanceSystem
		v.SetHostEnvironment(engine.HostEnvironment{})
		if e.schemeProvider != 0 {
			t.Error("a view following the host keeps its scoped style")
		}
		gObjectRefSink(e.surface)
		v.Close()
	})
}
