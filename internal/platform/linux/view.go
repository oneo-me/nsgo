//go:build linux && !android

package linux

import (
	"slices"
	"time"

	"github.com/nsgo-dev/nsgo/internal/driver"
	"github.com/nsgo-dev/nsgo/internal/platform/field"
	"github.com/nsgo-dev/nsgo/internal/platform/finger"
	"github.com/nsgo-dev/nsgo/internal/platform/hover"
	"github.com/nsgo-dev/nsgo/internal/platform/scrollbar"
)

// view is the main thread's copy of one mounted view, drawn by its
// widget.
type view struct {
	widget    uintptr // NsgoView
	id        driver.NodeID
	kind      driver.ViewKind
	parent    *view
	children  []*view
	frame     driver.Bounds
	style     driver.ViewStyle
	cursor    driver.Cursor // the cursor set on widget
	cursorSet bool
	events    driver.EventMask
	props     driver.ViewProps
	overflow  driver.Bounds // where its children reach outside it (ItemOverflow)
	deleted   bool

	// A text view's layout and where it goes in the view, its content
	// and the UTF-16 range of it a selection highlights.
	text             driver.TextLayout
	content          driver.Bounds
	plain            string
	spans            []driver.TextSpan
	selStart, selEnd int

	// glide is how a scroll view a finger let go comes to rest, and
	// refreshing whether it keeps the refresh room (touchscroll.go).
	glide      *scrollbar.Motion
	refreshing bool

	// modal is a modal's way in, and presented whether it came in yet.
	modal     driver.ModalAnimation
	presented bool

	// access is what GTK was last told of the view, and parts a canvas's
	// named parts (access.go).
	access accessState
	parts  []canvasPart

	// A scroll view's configuration, content size and offset.
	scroll driver.ScrollConfig
	size   driver.Size
	offset driver.Point
	wheel  scrollbar.Wheel // a snapping view's wheel turns
	// applied counts the core's ItemScrollTo and ItemScrollBy items,
	// reported with each offset; sticky are a scroll view's sticky cells,
	// and laid and shift a sticky cell's frame from the core and how far
	// it is moved from it.
	applied uint32
	sticky  []driver.StickyCell
	laid    driver.Bounds
	shift   driver.Point

	// An input's editor: entry is what is placed at the content, a
	// GtkEntry, or a GtkScrolledWindow holding the GtkTextView textView
	// of buffer; hint is a text view's placeholder label.
	entry, textView, buffer, hint uintptr
	provider                      uintptr // the editor's CSS, for the display
	input                         driver.InputConfig
	fs                            field.State
	// setting is the window setting the text, not the user; clipping an
	// insertion cut to maxLength.
	setting, clipping bool

	// An image view's configuration, source and placeholder.
	image               driver.ImageConfig
	source, placeholder imageSlot

	// A canvas's drawing, and what of it must be drawn again.
	drawing *driver.Drawing
	canvas  canvasDraw

	// A web view's page, a video's player, a native component and a
	// shader's GL area.
	web    *web
	video  *video
	native *component
	shader *glShaderView
	// fx is its effect, transition or navigation shader, fxArea the area
	// of the effect drawn over its children, its own or a screen
	// shader's (effect.go).
	fx     *effect
	fxArea uintptr

	// What animates: the style after the first, and the scroll offset.
	styled     bool
	born       uint64 // the batch that created it
	transition *transition
	// keyframeStart is when the keyframe animation of style began.
	keyframeStart time.Time
	scrolling     *scrolling
	// animated holds the values the animator binds over the style, nil
	// while none are bound.
	animated *driver.AnimatedValues
	// layout is a running layout animation: the frame moving, the view
	// appearing or leaving.
	layout *layoutAnim
	// dying views are deleted but still drawn while they leave; their
	// IDs may belong to new views already.
	dying bool
}

// detach takes v out of its parent's children.
func (v *view) detach() {
	if p := v.parent; p != nil {
		if i := slices.Index(p.children, v); i >= 0 {
			p.children = slices.Delete(p.children, i, i+1)
		}
		v.parent = nil
	}
}

// chrome is the desktop window's title bar and application menu.
type chrome struct {
	titleBar   driver.TitleBarStyle
	height     driver.Points // the app's title bar height
	fullScreen bool
	// constraints are the app's limits on the window.
	constraints driver.WindowConstraints
	menu        []driver.MenuEntry
	keys        map[string]menuRef
	// menuBar says when the bar shows; with MenuBarAutoHide, revealed is
	// set while a key showed it, and altAlone while Alt is down alone.
	menuBar            driver.MenuBarMode
	revealed, altAlone bool
}

// menuRef is a menu item's ID and role.
type menuRef struct {
	id        uint32
	role      driver.MenuRole
	notInText bool
}

// hidden reports whether the system title bar is hidden now.
func (w *window) hidden() bool {
	return w.chrome.titleBar == driver.TitleBarHidden && !w.chrome.fullScreen
}

// titleBarArea is what a hidden title bar still covers. GTK hides its
// decorations and draws no window controls of ours, so only the app's
// own bar height can be reported.
func (w *window) titleBarArea() driver.Edges {
	if !w.hidden() {
		return driver.Edges{}
	}
	return driver.Edges{Top: w.chrome.height}
}

// unsupported reports the roles GTK has no use for.
func unsupported(r driver.MenuRole) bool {
	return r == driver.MenuAbout || r == driver.MenuHide || r == driver.MenuHideOthers || r == driver.MenuShowAll
}

// shortcutOf returns e's shortcut, its role's when it has none.
func shortcutOf(e *driver.MenuEntry) string {
	if e.Shortcut != "" {
		return e.Shortcut
	}
	return e.Role.Shortcut(false)
}

// menuKeys gathers the shortcuts of entries' items, but the editing
// roles': the focused GtkEntry handles those keys itself.
func (w *window) menuKeys(entries []driver.MenuEntry) {
	for i := range entries {
		e := &entries[i]
		switch {
		case e.Separator, e.Disabled, unsupported(e.Role):
		case len(e.Items) > 0:
			w.menuKeys(e.Items)
		default:
			if k := shortcutOf(e); k != "" && !e.Role.EditsText() {
				if _, taken := w.chrome.keys[k]; !taken {
					w.chrome.keys[k] = menuRef{e.ID, e.Role, e.NotInText}
				}
			}
		}
	}
}

// hit returns the topmost view under p, in the coordinates of v's
// parent's content, that matches.
func hit(v *view, p driver.Point, match func(*view) bool) *view {
	if v.style.Hidden {
		return nil
	}
	self, children := v.reach(p)
	if children {
		local := driver.Point{X: p.X - v.frame.Origin.X + v.offset.X, Y: p.Y - v.frame.Origin.Y + v.offset.Y}
		for i := len(v.children) - 1; i >= 0; i-- {
			if h := hit(v.children[i], local, match); h != nil {
				return h
			}
		}
	}
	if self && match(v) {
		return v
	}
	return nil
}

// reach says what p, in the coordinates of v's parent's content, reaches
// of v: v itself and its children, as core.Reach decides.
func (v *view) reach(p driver.Point) (self, children bool) {
	return driver.Reach(v.frame, &v.props, v.overflow, p)
}

// under reports whether p reaches v or its children.
func (v *view) under(p driver.Point) bool {
	self, children := v.reach(p)
	return !v.style.Hidden && (self || children)
}

// scrollsAt appends the scroll views under p, outermost first; p is in
// the coordinates of v's parent's content.
func scrollsAt(v *view, p driver.Point, out []*view) []*view {
	out, _ = viewsAt(v, p, func(v *view) bool { return v.kind == driver.KindScroll }, out)
	return out
}

// viewsAt appends the views under p that match, outermost first: v and
// the child drawn on top at p, down to the innermost. It also reports
// whether anything under p takes the pointer: a view that takes none, or
// whose children take none, passes it to the views below it, so an
// overlay that lets presses through does not keep the wheel, a hover or
// touch from what it covers.
func viewsAt(v *view, p driver.Point, match func(*view) bool, out []*view) ([]*view, bool) {
	self, children := v.reach(p)
	if v.style.Hidden || v.style.Opacity <= 0 || !self && !children {
		return out, false
	}
	if self && match(v) {
		out = append(out, v)
	}
	took := self
	if children {
		local := driver.Point{X: p.X - v.frame.Origin.X + v.offset.X, Y: p.Y - v.frame.Origin.Y + v.offset.Y}
		for i := len(v.children) - 1; i >= 0; i-- {
			c := v.children[i]
			if !c.under(local) {
				continue
			}
			var childTook bool
			out, childTook = viewsAt(c, local, match, out)
			if childTook {
				return out, true
			}
		}
	}
	return out, took
}

// movePointer follows the pointer to surface point at, with modifiers
// state.
func (w *window) movePointer(at driver.Point, state uint32) {
	w.lastPointer, w.pointerIn, w.pointerState = at, true, state
	w.hoverViews()
	w.mouseMoved(state)
	if state&gdkButton1 != 0 {
		w.touch(w.mouse.Move(at))
	}
}

// touch reports the mouse as a finger, when ok.
func (w *window) touch(ev driver.Event, ok bool) {
	if ok {
		w.engine.Dispatch(ev)
	}
}

// listensTouches reports whether a view listens to touches.
func (w *window) listensTouches() bool {
	for _, v := range w.views {
		if v != nil && finger.Listens(v.events) {
			return true
		}
	}
	return false
}

// mouseButtons are a pointer state's button bits: GDK's second button is
// the middle one, and it has none for back and forward.
var mouseButtons = [5]uint32{gdkButton1, gdkButton3, gdkButton2}

// mouseMoved reports the pointer's move to the views under it that
// follow moves, innermost first.
func (w *window) mouseMoved(state uint32) {
	root := w.view(w.root)
	if root == nil {
		return
	}
	under, _ := viewsAt(root, w.lastPointer, func(v *view) bool { return v.events.Has(driver.EventMouseMove) }, nil)
	for i := len(under) - 1; i >= 0; i-- {
		v := under[i]
		b, _ := w.surfaceBounds(v)
		local := driver.Point{X: w.lastPointer.X - b.Origin.X, Y: w.lastPointer.Y - b.Origin.Y}
		w.engine.Dispatch(driver.Event{Kind: driver.EventMouseMove, ID: v.id, Point: local, Page: w.lastPointer,
			Modifiers: modifiers(state), Button: hover.Pressed(state, mouseButtons)})
	}
}

// hoverViews reports the pointer entering and leaving the views that
// follow hovers, as NSH's views hear it from the system.
func (w *window) hoverViews() {
	var under []driver.NodeID
	if root := w.view(w.root); root != nil && w.pointerIn {
		views, _ := viewsAt(root, w.lastPointer, func(v *view) bool { return hover.Follows(v.events) }, nil)
		for _, v := range views {
			under = append(under, v.id)
		}
	}
	for _, e := range w.hovered.Move(under) {
		if v := w.view(e.ID); v != nil {
			b, _ := w.surfaceBounds(v)
			e.Point = driver.Point{X: w.lastPointer.X - b.Origin.X, Y: w.lastPointer.Y - b.Origin.Y}
			e.Page, e.Modifiers, e.Button = w.lastPointer, modifiers(w.pointerState), hover.Pressed(w.pointerState, mouseButtons)
		}
		w.engine.Dispatch(e)
	}
}

// barDrag is the pointer dragging a scroll bar: from is where it pressed
// and origin the scroll view's, in the surface.
type barDrag struct {
	v            *view
	from, origin driver.Point
	drag         scrollbar.Drag
}

// bar returns a scroll view's scroll bar.
func (v *view) bar() scrollbar.Bar {
	return scrollbar.Bar{Horizontal: v.scroll.Horizontal, Frame: v.frame.Size, Content: v.size, Offset: v.offset,
		Inset: v.scroll.Behavior.ContentInset}
}

// hoverBars shows the bars of the scroll views under at, the pointer.
func (w *window) hoverBars(at driver.Point) {
	var under []*view
	if root := w.view(w.root); root != nil {
		under = scrollsAt(root, at, nil)
	}
	if !slices.Equal(under, w.bars) {
		w.bars = under
		gtkWidgetQueueDraw(w.surface)
	}
}

// showsBar reports whether v's scroll bar shows: while the pointer is
// over v or drags the bar.
func (w *window) showsBar(v *view) bool {
	return !v.scroll.HideIndicator && (w.barDrag != nil && w.barDrag.v == v || slices.Contains(w.bars, v))
}

// pressBar starts dragging the scroll bar under at, if there is one.
func (w *window) pressBar(at driver.Point) bool {
	root := w.view(w.root)
	if root == nil {
		return false
	}
	v, local := barAt(root, at)
	if v == nil {
		return false
	}
	w.barDrag = &barDrag{v: v, from: at, origin: driver.Point{X: at.X - local.X, Y: at.Y - local.Y}, drag: v.bar().Press(local)}
	w.interruptScroll(v)
	w.scrollEvent(v, driver.EventScrollDragStart)
	w.dragBar(at)
	return true
}

// dragBar scrolls the dragged bar's view to follow the pointer at at.
func (w *window) dragBar(at driver.Point) {
	d := w.barDrag
	if d.v.deleted {
		w.barDrag = nil
		return
	}
	w.interruptScroll(d.v) // the bar takes over
	w.scrollTo(d.v, d.drag.Offset(driver.Point{X: at.X - d.origin.X, Y: at.Y - d.origin.Y}))
	gtkWidgetQueueDraw(w.surface)
}

// barAt returns the innermost scroll view whose bar is under p, and p in
// that view; p is in the coordinates of v's parent's content.
func barAt(v *view, p driver.Point) (*view, driver.Point) {
	hit, at, _ := barIn(v, p)
	return hit, at
}

// barIn is barAt, which also reports whether anything under p takes the
// pointer, which a view that takes none passes to the views below it.
func barIn(v *view, p driver.Point) (*view, driver.Point, bool) {
	if v.style.Hidden || v.style.Opacity <= 0 {
		return nil, p, false
	}
	self, children := v.reach(p)
	if !self && !children {
		return nil, p, false
	}
	local := driver.Point{X: p.X - v.frame.Origin.X, Y: p.Y - v.frame.Origin.Y}
	took := self
	if children {
		inner := driver.Point{X: local.X + v.offset.X, Y: local.Y + v.offset.Y}
		for i := len(v.children) - 1; i >= 0; i-- {
			c := v.children[i]
			if !c.under(inner) {
				continue
			}
			hit, at, childTook := barIn(c, inner)
			if hit != nil {
				return hit, at, true
			}
			if childTook {
				took = true
				break
			}
		}
	}
	if self && v.kind == driver.KindScroll && !v.scroll.HideIndicator && !v.scroll.Behavior.NoScroll && v.bar().Contains(local) {
		return v, local, true
	}
	return nil, p, took
}

// press reports a click at p in the surface, with the modifiers and
// count of its press: on text with links, with the offset of the
// character under it; elsewhere the engine finds the view.
func (w *window) press(at driver.Point) {
	if root := w.view(w.root); root != nil {
		v := hit(root, at, func(v *view) bool {
			return v.events.Has(driver.EventPress) || v.kind == driver.KindInput || v.kind == driver.KindWeb
		})
		if v != nil && v.kind == driver.KindWeb {
			return // the page's, even where it has not shown yet
		}
		if v != nil && v.kind == driver.KindText {
			if l, ok := v.text.(*textLayout); ok {
				b, _ := w.surfaceBounds(v)
				x := at.X - b.Origin.X - v.content.Origin.X
				y := at.Y - b.Origin.Y - v.content.Origin.Y
				ev := w.down
				ev.Kind, ev.ID, ev.Index = driver.EventPress, v.id, l.indexAt(x, y)
				ev.Point, ev.Page = driver.Point{X: at.X - b.Origin.X, Y: at.Y - b.Origin.Y}, at
				w.engine.Dispatch(ev)
				return
			}
		}
	}
	ev := w.down
	ev.Page = at
	w.engine.PressAt(ev)
}

// pressOutside reports a press at p to each view that asks for presses
// outside it and does not contain the point.
func (w *window) pressOutside(at driver.Point) {
	for _, v := range w.views {
		if v == nil || !v.events.Has(driver.EventPressOutside) {
			continue
		}
		if b, shown := w.surfaceBounds(v); shown && !b.Contains(at) {
			w.engine.Dispatch(driver.Event{Kind: driver.EventPressOutside, ID: v.id, Point: at})
		}
	}
}

// contextMenu reports a secondary click at p to the view under it that
// asks for it, at p in the view's coordinates.
func (w *window) contextMenu(at driver.Point) {
	root := w.view(w.root)
	if root == nil {
		return
	}
	if v := hit(root, at, func(v *view) bool { return v.events.Has(driver.EventContextMenu) }); v != nil {
		b, _ := w.surfaceBounds(v)
		w.engine.Dispatch(driver.Event{Kind: driver.EventContextMenu, ID: v.id,
			Point: driver.Point{X: at.X - b.Origin.X, Y: at.Y - b.Origin.Y}, Page: at, Modifiers: w.down.Modifiers})
	}
}

// setCursor shows the cursor v's style names, or lets its parent's show.
func (w *window) setCursor(v *view) {
	if v.widget == 0 || v.cursorSet && v.cursor == v.style.Cursor {
		return
	}
	v.cursor, v.cursorSet = v.style.Cursor, true
	if v.cursor != driver.CursorAuto {
		gtkWidgetSetCursorFromName(v.widget, cstr(v.cursor.Name()))
		return
	}
	gtkWidgetSetCursorFromName(v.widget, nil)
}
