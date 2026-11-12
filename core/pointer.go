package core

import "github.com/nsgo-dev/nsgo/internal/driver"

// Pointer events: where a press, the mouse or a secondary click was and
// which modifier keys were held, as NSH's PressEvent, MouseEvent and
// ContextMenuEvent report them. Positions are in points.

// The mouse buttons of a MouseEvent; none is the zero value.
const (
	MouseButtonNone    = driver.MouseButtonNone
	MouseButtonLeft    = driver.MouseButtonLeft
	MouseButtonRight   = driver.MouseButtonRight
	MouseButtonMiddle  = driver.MouseButtonMiddle
	MouseButtonBack    = driver.MouseButtonBack
	MouseButtonForward = driver.MouseButtonForward
)

// PressEvent is where the pointer was at a press. Backends that know them
// report the modifier keys held when the press began and the click count;
// a press from assistive technology or the keyboard is at the view's
// origin with no keys and one click.
type PressEvent struct {
	// Position is in the pressed view, its transforms undone; for a
	// pressed span, in its text's view.
	Position Point
	// PagePosition is in the root view the view is in: the surface, or
	// its modal.
	PagePosition Point
	Modifiers    Modifiers
	// Clicks counts consecutive clicks at the same place: 1 for a click,
	// 2 for a double click.
	Clicks int
}

// MouseEvent is where the mouse is over a view, as GPUI's MouseMoveEvent
// says, or was last when it leaves.
type MouseEvent struct {
	Position     Point
	PagePosition Point
	Modifiers    Modifiers
	// PressedButton is the button held down, none as the mouse hovers.
	PressedButton MouseButton
}

// ContextMenuEvent is where a context menu was asked for. PagePosition is
// where a menu for it opens.
type ContextMenuEvent struct {
	Position     Point
	PagePosition Point
	Modifiers    Modifiers
}

func pressOf(e driver.Event) PressEvent {
	return PressEvent{Position: e.Point, PagePosition: e.Page, Modifiers: e.Modifiers, Clicks: max(int(e.Count), 1)}
}

func mouseOf(e driver.Event) MouseEvent {
	return MouseEvent{Position: e.Point, PagePosition: e.Page, Modifiers: e.Modifiers, PressedButton: e.Button}
}

// overflowArea returns the area n's children cover outside its bounds,
// in its own coordinates and holding them, or zero Bounds when they stay
// inside or n clips them: UIKit, AppKit and Android look for a touch's
// target only inside a view's bounds, so platforms look in the children
// of a view with an area too. Their own areas count, transforms do not.
func (n *node) overflowArea(kids []*node) Bounds {
	x, y := n.style.overflow()
	if n.scroll != nil || x == Hidden || y == Hidden || x == OverflowScroll || y == OverflowScroll || len(kids) == 0 {
		return Bounds{}
	}
	w, h := n.frame.Size.Width, n.frame.Size.Height
	x0, y0, x1, y1 := Points(0), Points(0), w, h
	grow := func(b Bounds) {
		x0, y0 = min(x0, b.Origin.X), min(y0, b.Origin.Y)
		x1, y1 = max(x1, b.Origin.X+b.Size.Width), max(y1, b.Origin.Y+b.Size.Height)
	}
	for _, c := range kids {
		if c.style.Display == DisplayNone || c.hidden || c.attach != nil {
			continue
		}
		if c.frame.Size.Width > 0 && c.frame.Size.Height > 0 {
			grow(c.frame)
		}
		if a := c.sent.overflow; a.Size.Width > 0 {
			grow(Bounds{Origin: Point{X: c.frame.Origin.X + a.Origin.X, Y: c.frame.Origin.Y + a.Origin.Y}, Size: a.Size})
		}
	}
	const epsilon = 0.5
	if x0 >= -epsilon && y0 >= -epsilon && x1 <= w+epsilon && y1 <= h+epsilon {
		return Bounds{}
	}
	return Bounds{Origin: Point{X: x0, Y: y0}, Size: Size{Width: x1 - x0, Height: y1 - y0}}
}
