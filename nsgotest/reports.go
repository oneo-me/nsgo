package nsgotest

import (
	"encoding/json"
	"strings"
	"time"

	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/internal/driver"
	"github.com/nsgo-dev/nsgo/internal/testhost"
)

// The platform's reports: what a platform tells the app about its media,
// menus, modals and gestures, sent as the platform sends them.

// selector matches, as a screen reader would: an action the view offers,
// or ActionActivate on a pressable view without it, which presses it.
func (a *App) AccessibilityAction(selector, name string) {
	a.t.Helper()
	v := a.Find(selector)
	switch {
	case v.Props.HasAction(name) && v.Events.Has(driver.EventAccessibilityAction):
		a.dispatch(driver.Event{Kind: driver.EventAccessibilityAction, ID: v.ID, Text: name})
	case name == driver.ActionActivate && v.Events.Has(driver.EventPress):
		a.dispatch(driver.Event{Kind: driver.EventPress, ID: v.ID})
	default:
		a.t.Fatalf("nsgotest: %q offers no %q action", selector, name)
	}
}

// Report sends ev as the platform reports it and settles: an event the
// steps here do not make. Platforms report most kinds only to views whose
// Events ask for them, which the test checks.
func (a *App) Report(ev driver.Event) {
	a.t.Helper()
	if ev.ID != 0 && a.views[ev.ID] == nil {
		a.t.Fatalf("nsgotest: no view %d to report %v to", ev.ID, ev.Kind)
	}
	a.dispatch(ev)
}

// Pan drags the view selector matches, or the one around it or under its
// middle with a pan,
// through each translation in turn, then lets go.
func (a *App) Pan(selector string, translations ...driver.Point) {
	a.t.Helper()
	v := a.gestureView(selector, driver.EventPan)
	a.gesture(v, driver.EventPan, len(translations), func(ev *driver.Event, i int) {
		ev.Point = translations[i]
	})
}

// Drag puts a finger down at the middle of the view selector matches,
// moves it there plus each offset in turn, 16 ms apart, and lifts it, as
// the platform reports touches: OnTouch and the touch responder hear it.
// Pinch pinches the view selector matches, or the one around it with a
// pinch, through each scale in turn.
func (a *App) Pinch(selector string, scales ...float32) {
	a.t.Helper()
	v := a.gestureView(selector, driver.EventPinch)
	at, _ := v.center()
	a.gesture(v, driver.EventPinch, len(scales), func(ev *driver.Event, i int) { ev.Value, ev.Point = scales[i], at })
}

// Wheel turns the mouse wheel over the middle of the view selector
// matches, or the one around it that takes the wheel, moving the content
// by delta, with mods held.
func (a *App) Wheel(selector string, delta driver.Point, mods driver.Modifiers) {
	a.t.Helper()
	v := a.gestureView(selector, driver.EventWheel)
	at, _ := v.center()
	a.dispatch(driver.Event{Kind: driver.EventWheel, ID: v.ID, Phase: driver.GestureChanged, Point: at, Delta: delta, Modifiers: mods})
}

// TouchpadScroll scrolls two fingers on a touchpad over the middle of the
// view selector matches, or the one around it that takes the wheel,
// moving the content by each delta in turn, and lifts them at velocity,
// which the platform's momentum follows when zero. On a target it is the
// platform's trackpad gesture by the deltas' sum, without momentum.
func (a *App) TouchpadScroll(selector string, velocity driver.Point, deltas ...driver.Point) {
	a.t.Helper()
	v := a.gestureView(selector, driver.EventWheel)
	at, page := v.center()
	if a.target != nil {
		// The platform's trackpad gesture, by the deltas' sum, its offset
		// against the content's way.
		var sum driver.Point
		for _, d := range deltas {
			sum.X, sum.Y = sum.X-d.X, sum.Y-d.Y
		}
		a.step("touchpadScroll(%s, %v, %v)", selector, sum.X, sum.Y)
		a.target.check(a.target.surface.Scroll(page, sum))
		a.done()
		return
	}
	ev := driver.Event{Kind: driver.EventWheel, ID: v.ID, Point: at, Precise: true, Phase: driver.GestureBegan}
	a.dispatch(ev)
	for _, d := range deltas {
		ev.Phase, ev.Delta = driver.GestureChanged, d
		a.dispatch(ev)
	}
	ev.Phase, ev.Delta, ev.Velocity = driver.GestureEnded, driver.Point{}, velocity
	a.dispatch(ev)
}

// Rotate rotates the view selector matches, or the one around it with a
// rotation, through each angle in radians in turn.
func (a *App) Rotate(selector string, angles ...float32) {
	a.t.Helper()
	v := a.gestureView(selector, driver.EventRotation)
	a.gesture(v, driver.EventRotation, len(angles), func(ev *driver.Event, i int) { ev.Value = angles[i] })
}

func (a *App) gestureView(selector string, kind driver.EventKind) *View {
	a.t.Helper()
	m := a.Find(selector)
	for p := m; p != nil; p = p.Parent {
		if p.Events.Has(kind) {
			return p
		}
	}
	// Else what a pointer at its middle finds, as over a layer that takes
	// the gestures of what lies under it.
	if m != nil {
		_, at := m.center()
		if v := a.viewAt(at, func(v *View) bool { return v.Events.Has(kind) }); v != nil {
			return v
		}
	}
	a.t.Fatalf("nsgotest: no gesture view at %q", selector)
	return nil
}

// gesture sends a gesture of n steps: began with the first, changed with
// the rest and ended with the last.
func (a *App) gesture(v *View, kind driver.EventKind, n int, step func(*driver.Event, int)) {
	if n == 0 {
		return
	}
	send := func(phase driver.GesturePhase, i int) {
		ev := driver.Event{Kind: kind, ID: v.ID, Phase: phase}
		step(&ev, i)
		// A platform moves bound values before it reports the move.
		a.anim.Gesture(&ev)
		a.dispatch(ev)
	}
	send(driver.GestureBegan, 0)
	for i := 1; i < n; i++ {
		send(driver.GestureChanged, i)
	}
	send(driver.GestureEnded, n-1)
}

// ChooseMenu chooses the application menu item at path, its labels from
// the menu bar down, such as "File", "New".
// the menu bar down, such as "File", "New".
func (a *App) ChooseMenu(path ...string) {
	a.t.Helper()
	items := a.Menu
	for i, label := range path {
		var found *driver.MenuEntry
		for k := range items {
			if items[k].Label == label {
				found = &items[k]
			}
		}
		if found == nil {
			a.t.Fatalf("nsgotest: no menu item %q", path[:i+1])
		}
		if found.Disabled {
			a.t.Fatalf("nsgotest: the menu item %q is disabled", path[:i+1])
		}
		if i == len(path)-1 {
			if found.ID == 0 {
				a.t.Fatalf("nsgotest: menu item %q does nothing", path)
			}
			if a.target != nil {
				a.step("chooseMenu(%q)", path)
				a.target.menu(func(s testhost.Surface) error { return s.ChooseMenu(found.ID) })
				a.done()
				return
			}
			a.dispatch(driver.Event{Kind: driver.EventMenu, Count: found.ID})
			return
		}
		items = found.Items
	}
}

// ChoosePopup chooses the item at path in the context menu the app shows
// with PopupMenu, its labels from the menu down, which closes the menu.
func (a *App) ChoosePopup(path ...string) {
	a.t.Helper()
	if a.PopupMenu == nil {
		a.t.Fatalf("nsgotest: no context menu shows")
	}
	items := a.PopupMenu
	for i, label := range path {
		var found *driver.MenuEntry
		for k := range items {
			if items[k].Label == label {
				found = &items[k]
			}
		}
		if found == nil {
			a.t.Fatalf("nsgotest: no context menu item %q", path[:i+1])
		}
		if found.Disabled {
			a.t.Fatalf("nsgotest: the context menu item %q is disabled", path[:i+1])
		}
		if i == len(path)-1 {
			if a.target != nil {
				a.step("choosePopup(%q)", path)
				a.target.menu(func(s testhost.Surface) error { return s.ChoosePopup(found.ID) })
				a.PopupMenu, a.popupID = nil, 0
				a.done()
				return
			}
			if found.ID != 0 {
				a.post(driver.Event{Kind: driver.EventMenu, Count: found.ID})
			}
			a.ClosePopup()
			return
		}
		items = found.Items
	}
}

// ClosePopup closes the context menu the app shows without choosing an
// item, as Escape or a click elsewhere does.
func (a *App) ClosePopup() {
	a.t.Helper()
	if a.target != nil {
		a.step("closePopup()")
		a.target.menu(func(s testhost.Surface) error { return s.ChoosePopup(0) })
		a.PopupMenu, a.popupID = nil, 0
		a.done()
		return
	}
	if a.popupID != 0 {
		a.post(driver.Event{Kind: driver.EventMenuClosed, Count: a.popupID})
		a.PopupMenu, a.popupID = nil, 0
	}
	a.Settle()
}

// HeaderBack presses the back button in the system header of the screen
// selector matches, as iOS and Android draw it.
func (a *App) HeaderBack(selector string) {
	a.t.Helper()
	a.dispatch(driver.Event{Kind: driver.EventBack, ID: a.Find(selector).ID})
}

// SwipeClose swipes away the modal around what selector matches, as an
// iOS sheet is: the platform closes it and asks the app to remove it
// ("swipe").
func (a *App) SwipeClose(selector string) {
	a.t.Helper()
	a.dispatch(driver.Event{Kind: driver.EventRequestClose, ID: a.modal(selector).ID, Text: "swipe", Finished: true})
}

// PresentModal reports that the platform presents the modal around what
// selector matches at size, with insets covered, as an iOS sheet does.
func (a *App) PresentModal(selector string, size driver.Size, insets driver.Edges) {
	a.t.Helper()
	a.dispatch(driver.Event{Kind: driver.EventModalSize, ID: a.modal(selector).ID, Size: size, Insets: insets})
}

// modal returns the modal selector matches or holds what it matches.
func (a *App) modal(selector string) *View {
	a.t.Helper()
	v := a.Find(selector)
	for m := v; m != nil; m = m.Parent {
		if m.Kind == driver.KindModal {
			return m
		}
	}
	a.t.Fatalf("nsgotest: %q is in no modal", selector)
	return nil
}

// Load reports that the image selector matches loaded at pixels. An
// image that takes its size from what it loads matches before, at no
// size.
func (a *App) Load(selector string, pixels driver.Size) {
	a.t.Helper()
	sel, err := parseSelector(selector)
	if err != nil {
		a.t.Fatalf("nsgotest: %v", err)
	}
	found := a.resolveWith(sel, true)
	if len(found) != 1 {
		a.t.Fatalf("nsgotest: %d views match %q\n%s", len(found), selector, a.Dump())
	}
	v := found[0]
	for _, k := range []driver.EventKind{driver.EventLoadStart, driver.EventLoad, driver.EventLoadEnd} {
		if v.Events.Has(k) {
			a.dispatch(driver.Event{Kind: k, ID: v.ID, Size: pixels})
		}
	}
}

// Play reports that the video selector matches opened and loaded
// info, as a platform does: it starts loading, then loads.
func (a *App) Play(selector string, info core.VideoInfo) {
	a.t.Helper()
	v := a.Find(selector)
	for _, k := range []driver.EventKind{driver.EventLoadStart, driver.EventLoad} {
		if v.Events.Has(k) {
			a.dispatch(driver.Event{Kind: k, ID: v.ID, Duration: info.Duration, Size: info.Size})
		}
	}
}

// Progress reports that the video selector matches played to position:
// its progress, then its end when position reaches duration.
func (a *App) Progress(selector string, position, duration time.Duration) {
	a.t.Helper()
	v := a.Find(selector)
	if v.Events.Has(driver.EventProgress) {
		a.dispatch(driver.Event{Kind: driver.EventProgress, ID: v.ID, Time: position, Duration: duration})
	}
	if position >= duration && v.Events.Has(driver.EventEnd) {
		a.dispatch(driver.Event{Kind: driver.EventEnd, ID: v.ID})
	}
}

// Navigate reports that the web view selector matches loaded the page
// nav describes, as a platform does: it starts loading, then loads.
func (a *App) Navigate(selector string, nav driver.WebNavigation) {
	a.t.Helper()
	v := a.Find(selector)
	for _, loading := range []bool{true, false} {
		nav.Loading = loading
		kind := driver.EventLoad
		if loading {
			kind = driver.EventLoadStart
		}
		for _, k := range []driver.EventKind{kind, driver.EventNavigationStateChange} {
			if v.Events.Has(k) {
				n := nav
				a.dispatch(driver.Event{Kind: k, ID: v.ID, Navigation: &n})
			}
		}
	}
}

// PostMessage reports that the page in the web view selector matches
// posted data.
func (a *App) PostMessage(selector, data string) {
	a.t.Helper()
	v := a.Find(selector)
	if v.Events.Has(driver.EventMessage) {
		a.dispatch(driver.Event{Kind: driver.EventMessage, ID: v.ID, Text: data})
	}
}

// DragScroll drags the scroll view selector matches to offset at once and
// lets go at velocity (points per second along its axis, positive as the
// offset grows), as a platform does: the drag's events, then the glide or
// the snap its ScrollBehavior decides, clamped to its content and insets.
func (a *App) DragScroll(selector string, offset driver.Point, velocity float32) {
	a.t.Helper()
	v := a.Find(selector)
	if v.Scroll.Behavior.NoScroll {
		return
	}
	a.scrollEvent(v, driver.EventScrollDragStart)
	a.scrollView(v, offset)
	a.scrollEvent(v, driver.EventScrollDragEnd)
	b := &v.Scroll.Behavior
	main := func(p driver.Point) driver.Points {
		if v.Scroll.Horizontal {
			return p.X
		}
		return p.Y
	}
	viewport, content := v.Frame.Size.Height, v.Content.Height
	lo, hi := -b.ContentInset.Top, b.ContentInset.Bottom
	if v.Scroll.Horizontal {
		viewport, content = v.Frame.Size.Width, v.Content.Width
		lo, hi = -b.ContentInset.Left, b.ContentInset.Right
	}
	at, ms := main(offset), velocity/1000
	to := driver.Projected(at, ms, b.Deceleration())
	if snap, ok := b.SnapTarget(at, to, ms, viewport, content); ok {
		to = snap
	} else if ms == 0 {
		a.settle(v)
		return
	}
	to = min(max(to, lo), max(content-viewport, 0)+hi)
	if to == at {
		a.settle(v)
		return
	}
	a.scrollEvent(v, driver.EventMomentumStart)
	end := offset
	if v.Scroll.Horizontal {
		end.X = to
	} else {
		end.Y = to
	}
	a.scrollView(v, end)
	a.scrollEvent(v, driver.EventMomentumEnd)
	a.settle(v)
}

// settle follows scroll view v coming to rest, as platforms do: it
// reports EventScrollEnd and the snapping diff clamps that follow v
// settle.
func (a *App) settle(v *View) {
	a.scrollEvent(v, driver.EventScrollEnd)
	a.anim.Settle(v.ID)
	a.animate()
}

// PullToRefresh pulls the refreshable scroll view selector matches down
// past its top, as a user does to refresh it.
func (a *App) PullToRefresh(selector string) {
	a.t.Helper()
	v := a.Find(selector)
	if !v.Scroll.Refreshable || v.Scroll.Refreshing || v.Scroll.Horizontal || v.Scroll.Behavior.NoScroll {
		return
	}
	a.scrollEvent(v, driver.EventScrollDragStart)
	a.dispatch(driver.Event{Kind: driver.EventRefresh, ID: v.ID, Point: v.Offset, Count: v.Applied})
	a.scrollEvent(v, driver.EventScrollDragEnd)
}

// PinchZoom zooms the zoomable scroll view selector matches to scale, as a
// user's pinch does, within its limits.
func (a *App) PinchZoom(selector string, scale float32) {
	a.t.Helper()
	v := a.Find(selector)
	c := &v.Scroll
	if !c.Zooms() || c.NoPinchZoom {
		return
	}
	v.ZoomScale = min(max(scale, c.MinZoomScale), c.MaxZoomScale)
	a.dispatch(driver.Event{Kind: driver.EventScroll, ID: v.ID, Point: v.Offset, Value: v.ZoomScale, Count: v.Applied})
}

func (a *App) scrollEvent(v *View, kind driver.EventKind) {
	if v.Events.Has(kind) {
		a.dispatch(driver.Event{Kind: kind, ID: v.ID, Point: v.Offset, Value: v.ZoomScale, Count: v.Applied})
	}
}

// DropFiles drops files on the view selector matches, as a desktop or
// the web does: the drag enters, then the files drop.
// the web does: the drag enters, then the files drop.
func (a *App) DropFiles(selector string, uris ...string) {
	a.t.Helper()
	v := a.Find(selector)
	if v.Events.Has(driver.EventFileDragEnter) {
		a.dispatch(driver.Event{Kind: driver.EventFileDragEnter, ID: v.ID})
	}
	a.dispatch(driver.Event{Kind: driver.EventFileDrop, ID: v.ID, URIs: uris})
}

// Emit reports that the component of the native view selector matches
// emitted name with data. The view need only be mounted: headless, a
// component that sizes itself, such as the system's tab bar, has no size.
func (a *App) Emit(selector, name string, data any) {
	a.t.Helper()
	vs := a.QueryMounted(selector)
	if len(vs) != 1 {
		a.t.Fatalf("nsgotest: %d views match %q\n%s", len(vs), selector, a.Dump())
	}
	a.dispatch(driver.Event{Kind: driver.EventNative, ID: vs[0].ID, Text: name, Data: data})
}

// AnswerScript answers the oldest Ref.Evaluate the web view selector
// matches still waits for, as its page would: with result, JSON, or
// when errText is set the error the script threw.
func (a *App) AnswerScript(selector, result, errText string) {
	a.t.Helper()
	v := a.Find(selector)
	for i := v.answered; i < len(v.WebCommands); i++ {
		c := v.WebCommands[i]
		at := strings.Index(c.Value, "nsgo-evaluate:")
		if c.Command != driver.WebInjectJavaScript || at < 0 {
			continue
		}
		end := strings.IndexByte(c.Value[at+len("nsgo-evaluate:"):], ':')
		if end < 0 {
			continue
		}
		marker := c.Value[at : at+len("nsgo-evaluate:")+end+1]
		v.answered = i + 1
		answer := map[string]string{"json": result}
		if errText != "" {
			answer = map[string]string{"error": errText}
		}
		data, _ := json.Marshal(answer)
		a.PostMessage(selector, marker+string(data))
		return
	}
	a.t.Fatalf("nsgotest: %q waits for no script's answer", selector)
}
