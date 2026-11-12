//go:build windows

package windows

import (
	"testing"
	"time"

	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/internal/driver"
)

// TestSelectionAreaSelectsAcrossTexts double-clicks a word of a selection
// area, extends the selection into the next text with Shift, checks the
// highlight is drawn and clears it with Escape.
func TestSelectionAreaSelectsAcrossTexts(t *testing.T) {
	if testing.Short() {
		t.Skip("opens a window")
	}
	title := "nsgo-select"
	app := func(cx *core.Scope) core.Node {
		return core.View{Selectable: true, Style: core.Style{Width: driver.Pt(320), Height: driver.Pt(200), Padding: driver.All(10),
			Gap: 10, Background: driver.RGB(0xffffff)}, Children: []core.Node{
			core.Text{Content: "Hello world", Style: core.Style{FontSize: 16}},
			core.Text{Content: "Second line", Style: core.Style{FontSize: 16, SelectionColor: driver.RGB(0xff0000)}},
		}}
	}
	hwnd := startApp(t, title, app, 320, 200)
	var red driver.Point
	var first, second *view
	for deadline := time.Now().Add(5 * time.Second); second == nil && time.Now().Before(deadline); time.Sleep(20 * time.Millisecond) {
		onMain(t, hwnd, func(w *window) {
			for _, v := range w.views {
				if v != nil && v.kind == driver.KindText && v.plain == "Second line" {
					second = v
				}
			}
		})
	}
	onMain(t, hwnd, func(w *window) {
		for _, v := range w.views {
			if v != nil && v.kind == driver.KindText {
				switch v.plain {
				case "Hello world":
					first = v
				case "Second line":
					second = v
				}
			}
		}
		if first == nil || second == nil {
			t.Error("texts not mounted")
			return
		}
		b, _ := w.surfaceBounds(first)
		at := driver.Point{X: b.Origin.X + 10, Y: b.Origin.Y + b.Size.Height/2}
		w.selectPress(at, 2, false)
		w.selectUp()
		if got := w.textSelection.Model.Text(w.textSelection); got != "Hello" {
			t.Errorf("double click selected %q, want Hello", got)
		}
		b, _ = w.surfaceBounds(second)
		at = driver.Point{X: b.Origin.X + b.Size.Width - 2, Y: b.Origin.Y + b.Size.Height/2}
		w.selectPress(at, 1, true)
		w.selectUp()
		if got := w.textSelection.Model.Text(w.textSelection); got != "Hello world\nSecond line" {
			t.Errorf("Shift-click selected %q", got)
		}
		red = driver.Point{X: b.Origin.X + 3, Y: b.Origin.Y + 2}
	})
	// The second text's highlight is drawn in its selection color.
	scale := 1.0
	if dpi, _, _ := getDpiForSystem.Call(); dpi != 0 {
		scale = float64(dpi) / 96
	}
	px := func(p driver.Points) int32 { return int32(float64(p) * scale) }
	if c, ok := waitForPixel(t, hwnd, px(red.X), px(red.Y), 0xff0000, 3*time.Second); !ok {
		t.Errorf("pixel under the highlight = %06x, want red", c)
	}
	onMain(t, hwnd, func(w *window) {
		w.selectKey(vkEscape)
		if w.textSelection != nil {
			t.Error("Escape left the selection")
		}
	})
}

// TestAlertIsDrawnInTheSurface shows an alert, which the window draws
// over the views with its default button in the accent color, and
// answers it with a click on its second button.
func TestAlertIsDrawnInTheSurface(t *testing.T) {
	if testing.Short() {
		t.Skip("opens a window")
	}
	title := "nsgo-alert"
	app := func(cx *core.Scope) core.Node {
		cx.Post(func() {
			cx.ShowAlert(core.AlertConfig{Title: "Delete file?", Message: "It cannot be undone.",
				Actions: []core.AlertAction{{Label: "Cancel", Role: core.AlertCancel}, {Label: "Delete", Role: core.AlertDefault}}},
				func(i int) { cx.SetTitle(title + "|answered " + string(rune('0'+i))) })
		})
		return core.View{Style: core.Style{FlexGrow: 1, Background: driver.RGB(0xffffff)}}
	}
	hwnd := startApp(t, title, app, 400, 300)
	var accent driver.Point
	deadline := time.Now().Add(5 * time.Second)
	for accent == (driver.Point{}) && time.Now().Before(deadline) {
		onMain(t, hwnd, func(w *window) {
			if s := w.alert; s != nil && len(s.buttons) == 2 {
				r := s.buttons[1]
				accent = driver.Point{X: driver.Points(r.left + 4), Y: driver.Points(r.top + 4)}
			}
		})
		time.Sleep(50 * time.Millisecond)
	}
	if accent == (driver.Point{}) {
		t.Fatal("the alert was not drawn")
	}
	scale := 1.0
	if dpi, _, _ := getDpiForSystem.Call(); dpi != 0 {
		scale = float64(dpi) / 96
	}
	px := func(p driver.Points) int32 { return int32(float64(p) * scale) }
	abgr := regDWORD(`Software\Microsoft\Windows\CurrentVersion\Explorer\Accent`, "AccentColorMenu", 0xffb85f00)
	want := uint32(abgr&0xff<<16 | abgr>>8&0xff<<8 | abgr>>16&0xff)
	if c, ok := waitForPixel(t, hwnd, px(accent.X), px(accent.Y), want, 3*time.Second); !ok {
		t.Errorf("default button = %06x, want the accent %06x", c, want)
	}
	// The scrim darkens the white view.
	if c, ok := waitForPixel(t, hwnd, 3, 3, 0xb2b2b2, time.Second); !ok {
		t.Errorf("scrim = %06x", c)
	}
	onMain(t, hwnd, func(w *window) {
		s := w.alert
		r := s.buttons[1]
		at := uintptr(uint32(px(driver.Points((r.top+r.bottom)/2)))<<16 | uint32(px(driver.Points((r.left+r.right)/2))))
		postMessageW.Call(hwnd, wmLButtonDown, mkLButton, at)
		postMessageW.Call(hwnd, wmLButtonUp, 0, at)
	})
	waitForTitle(t, hwnd, title+"|answered 1", 5*time.Second)
}

// TestBlendModesAndBackdropFiltersReadTheBackdrop multiplies a cyan view
// over yellow, which shows green, and blurs what is behind a view over a
// red and blue edge, which mixes them there.
func TestBlendModesAndBackdropFiltersReadTheBackdrop(t *testing.T) {
	if testing.Short() {
		t.Skip("opens a window")
	}
	app := func(cx *core.Scope) core.Node {
		return core.View{Style: core.Style{FlexGrow: 1, Background: driver.RGB(0xffff00), Direction: core.Row}, Children: []core.Node{
			core.View{Style: core.Style{Width: driver.Pt(60), Height: driver.Pt(60), Background: driver.RGB(0x00ffff), BlendMode: core.BlendMultiply}},
			core.View{Style: core.Style{Width: driver.Pt(100), Height: driver.Pt(100)}, Children: []core.Node{
				core.View{Style: core.Style{Position: core.Absolute, Left: driver.Pt(0), Top: driver.Pt(0), Width: driver.Pt(50), Height: driver.Pt(100), Background: driver.RGB(0xff0000)}},
				core.View{Style: core.Style{Position: core.Absolute, Left: driver.Pt(50), Top: driver.Pt(0), Width: driver.Pt(50), Height: driver.Pt(100), Background: driver.RGB(0x0000ff)}},
				core.View{Style: core.Style{Position: core.Absolute, Left: driver.Pt(20), Top: driver.Pt(20), Width: driver.Pt(60), Height: driver.Pt(60),
					BackdropFilters: []core.Filter{core.Blur(12)}}},
			}},
		}}
	}
	hwnd := startApp(t, "nsgo-backdrop", app, 300, 200)
	scale := 1.0
	if dpi, _, _ := getDpiForSystem.Call(); dpi != 0 {
		scale = float64(dpi) / 96
	}
	px := func(p float64) int32 { return int32(p * scale) }
	if c, ok := waitForPixel(t, hwnd, px(30), px(30), 0x00ff00, 5*time.Second); !ok {
		t.Errorf("multiplied view = %06x, want green", c)
	}
	// At the edge, the blur mixes red and blue: purple.
	if c, ok := waitForPixel(t, hwnd, px(110), px(50), 0x800080, 3*time.Second); !ok {
		t.Errorf("blurred edge = %06x, want purple", c)
	}
	// Outside the view, the edge stays sharp.
	if c, ok := waitForPixel(t, hwnd, px(108), px(10), 0xff0000, time.Second); !ok {
		t.Errorf("edge outside the view = %06x, want red", c)
	}
}

// TestTouchDragsScrollAndPullsToRefresh drags a scroll view with a finger,
// which glides once let go, then pulls it past its top, which refreshes
// and keeps room above the content until the app is done.
func TestTouchDragsScrollAndPullsToRefresh(t *testing.T) {
	if testing.Short() {
		t.Skip("opens a window")
	}
	var refreshing *core.Signal[bool]
	var root *core.Scope
	refreshed := make(chan bool, 1)
	app := func(cx *core.Scope) core.Node {
		root = cx
		refreshing = core.NewSignal(cx, false)
		var rows []core.Node
		for range 40 {
			rows = append(rows, core.View{Style: core.Style{Height: driver.Pt(40), Background: driver.RGB(0xdddddd)}})
		}
		return core.Scroll{Style: core.Style{Width: driver.Pt(200), Height: driver.Pt(300)}, Children: rows,
			OnRefresh: func() { refreshing.Set(true); refreshed <- true }, Refreshing: refreshing.Get}
	}
	hwnd := startApp(t, "nsgo-touch", app, 200, 300)
	var scroll *view
	deadline := time.Now().Add(5 * time.Second)
	for scroll == nil && time.Now().Before(deadline) {
		onMain(t, hwnd, func(w *window) {
			for _, v := range w.views {
				if v != nil && v.kind == driver.KindScroll && v.size.Height > 0 {
					scroll = v
				}
			}
		})
		time.Sleep(20 * time.Millisecond)
	}
	if scroll == nil {
		t.Fatal("no scroll view")
	}
	drag := func(from driver.Point, dys ...driver.Points) {
		onMain(t, hwnd, func(w *window) {
			w.touchDown(1, from)
			for _, dy := range dys {
				from.Y += dy
				time.Sleep(10 * time.Millisecond)
				w.touchMove(from)
			}
			w.touchUp()
		})
	}
	drag(driver.Point{X: 100, Y: 250}, -20, -20, -20, -20, -20)
	var at, rest driver.Point
	onMain(t, hwnd, func(w *window) { at = scroll.offset })
	if at.Y < 70 {
		t.Fatalf("offset %v after an 80-point drag past the slop", at)
	}
	time.Sleep(time.Second)
	onMain(t, hwnd, func(w *window) { rest = scroll.offset })
	if rest.Y <= at.Y {
		t.Errorf("the view rests at %v, want past %v: no glide", rest, at)
	}
	onMain(t, hwnd, func(w *window) { w.scrollTo(scroll, driver.Point{}) })
	drag(driver.Point{X: 100, Y: 50}, 20, 40, 40, 40, 40)
	select {
	case <-refreshed:
	case <-time.After(3 * time.Second):
		t.Fatal("the pull did not refresh")
	}
	time.Sleep(500 * time.Millisecond)
	onMain(t, hwnd, func(w *window) { rest = scroll.offset })
	if want := -scroll.scroll.Behavior.RefreshInset(); rest.Y != want {
		t.Errorf("refreshing, the view rests at %v, want %v", rest.Y, want)
	}
	root.Post(func() { refreshing.Set(false) })
	time.Sleep(600 * time.Millisecond)
	onMain(t, hwnd, func(w *window) { rest = scroll.offset })
	if rest.Y != 0 {
		t.Errorf("done refreshing, the view rests at %v, want 0", rest.Y)
	}
}
