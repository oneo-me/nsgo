// Package scrollbar is the scroll logic of the backends that keep scroll
// offsets of their own: where a scroll view's thumb is, whether a point is
// on its bar, and where dragging the thumb scrolls to; the offsets its
// content inset allows; and where a snapping view settles after a wheel
// or a thumb drag. The backends show a bar while its scroll view moves,
// while the pointer is over it, or while its thumb is dragged. macOS
// takes the motion too (motion.go): the rubber band, eased snaps, the
// indicator's geometry and fade, and trackpad events shown in step with
// the display.
package scrollbar

import (
	"github.com/nsgo-dev/nsgo/internal/driver"
)

const (
	// Thickness is the thumb's, Inset its distance from the edge.
	Thickness driver.Points = 6
	Inset     driver.Points = 3
	// MinLength is the shortest a thumb gets on long content.
	MinLength driver.Points = 24
	// Grab is the width of the strip along the edge where a press takes
	// the bar.
	Grab driver.Points = 12
)

// Bar is a scroll view's scroll bar.
type Bar struct {
	Horizontal bool
	// Frame is the scroll view's size, Content its content's and Offset
	// how far the content is scrolled.
	Frame, Content driver.Size
	Offset         driver.Point
	// Inset is the content inset, which scrolls as part of the content.
	Inset driver.Edges
}

// along returns the bar's lengths along its axis: the view's, the
// content's, the offset and the thumb's.
func (b Bar) along() (frame, content, offset, thumb driver.Points) {
	frame, content, offset = b.Frame.Height, b.Content.Height+b.Inset.Top+b.Inset.Bottom, b.Offset.Y+b.Inset.Top
	if b.Horizontal {
		frame, content, offset = b.Frame.Width, b.Content.Width+b.Inset.Left+b.Inset.Right, b.Offset.X+b.Inset.Left
	}
	if content <= frame || frame <= 0 {
		return frame, content, offset, 0
	}
	return frame, content, offset, min(max(frame*frame/content, MinLength), frame)
}

// Thumb returns the thumb's rectangle in the scroll view, or false when
// the content fits and there is nothing to scroll.
func (b Bar) Thumb() (driver.Bounds, bool) {
	frame, content, offset, length := b.along()
	if length == 0 {
		return driver.Bounds{}, false
	}
	at := min(max(offset/(content-frame), 0), 1) * (frame - length)
	if b.Horizontal {
		return driver.Bounds{Origin: driver.Point{X: at, Y: b.Frame.Height - Inset - Thickness},
			Size: driver.Size{Width: length, Height: Thickness}}, true
	}
	return driver.Bounds{Origin: driver.Point{X: b.Frame.Width - Inset - Thickness, Y: at},
		Size: driver.Size{Width: Thickness, Height: length}}, true
}

// Indicator returns the overlay indicator's rectangle in the scroll
// view, how far it travels along its track, and false when the content
// fits. It is macOS's: IndicatorMargin at each end, no shorter than
// indicatorMin, and shrinking while the content is pulled past an edge.
func (b Bar) Indicator() (rect driver.Bounds, travel driver.Points, ok bool) {
	frame, content, offset, _ := b.along()
	start, length, ok := Indicator(frame, content, offset)
	if !ok {
		return driver.Bounds{}, 0, false
	}
	if b.Horizontal {
		rect = driver.Bounds{Origin: driver.Point{X: start, Y: b.Frame.Height - Inset - Thickness},
			Size: driver.Size{Width: length, Height: Thickness}}
	} else {
		rect = driver.Bounds{Origin: driver.Point{X: b.Frame.Width - Inset - Thickness, Y: start},
			Size: driver.Size{Width: Thickness, Height: length}}
	}
	return rect, frame - 2*IndicatorMargin - length, true
}

// Contains reports whether p, in the scroll view, is on the bar: in the
// strip along its edge, while there is something to scroll.
func (b Bar) Contains(p driver.Point) bool {
	if _, ok := b.Thumb(); !ok || !(driver.Bounds{Size: b.Frame}).Contains(p) {
		return false
	}
	if b.Horizontal {
		return p.Y >= b.Frame.Height-Grab
	}
	return p.X >= b.Frame.Width-Grab
}

// Drag follows the pointer pressing a bar.
type Drag struct {
	bar   Bar
	from  driver.Points // where the pointer took the thumb, along the axis
	start driver.Points // the offset then
	track driver.Points // how far the thumb travels along its track
	span  driver.Points // how far the content can move
}

// Shift moves the drag along with the core moving the offset by d along
// the axis (ItemScrollBy), so it continues from the content it was on.
func (d *Drag) Shift(by driver.Points) { d.start += by }

// Press starts dragging the bar from p, in the scroll view. Off the
// thumb, the thumb first jumps to center under p.
func (b Bar) Press(p driver.Point) Drag {
	frame, content, offset, length := b.along()
	at := p.Y
	if b.Horizontal {
		at = p.X
	}
	d := Drag{bar: b, from: at, start: offset, track: frame - length, span: content - frame}
	if thumb := min(max(offset/(content-frame), 0), 1) * d.track; length > 0 && d.track > 0 &&
		(at < thumb || at >= thumb+length) {
		d.start = (at - length/2) / d.track * d.span
	}
	return d
}

// PressIndicator starts dragging the overlay indicator of Indicator from
// p, in the scroll view. Off the thumb, the thumb first jumps to center
// under p.
func (b Bar) PressIndicator(p driver.Point) Drag {
	frame, content, offset, _ := b.along()
	at, start, length := p.Y, driver.Points(0), driver.Points(0)
	if b.Horizontal {
		at = p.X
	}
	if rect, travel, ok := b.Indicator(); ok {
		start, length = rect.Origin.Y, rect.Size.Height
		if b.Horizontal {
			start, length = rect.Origin.X, rect.Size.Width
		}
		d := Drag{bar: b, from: at, start: offset, track: travel, span: content - frame}
		if travel > 0 && (at < start || at >= start+length) {
			d.start = (at - IndicatorMargin - length/2) / travel * d.span
		}
		return d
	}
	return Drag{bar: b, from: at, start: offset}
}

// Offset returns where the content scrolls to with the pointer at p, in
// the scroll view, clamped to what it can show.
func (d Drag) Offset(p driver.Point) driver.Point {
	out := d.bar.Offset
	if d.track <= 0 || d.span <= 0 {
		return out
	}
	at := p.Y
	if d.bar.Horizontal {
		at = p.X
	}
	to := min(max(d.start+(at-d.from)*d.span/d.track, 0), d.span)
	if d.bar.Horizontal {
		out.X = to - d.bar.Inset.Left
	} else {
		out.Y = to - d.bar.Inset.Top
	}
	return out
}
