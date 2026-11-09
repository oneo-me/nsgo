//go:build ios

package watchos

import "github.com/oneo-me/nsgo/internal/driver"

// A watch has the memory for the views on its screen, not for a long
// page's: SwiftUI keeps a view's state and its Canvas's pixels whether it
// shows or not. So the host makes only the views a scroll view shows or
// nearly does, within a screen of its viewport either way; the rest are
// culled, kept here alone until a scroll brings them near.

// cull marks the views out of reach of their scroll views' viewports and
// tells the host which changed.
func (s *surface) cull() {
	root := s.view(s.root)
	if root == nil {
		return
	}
	s.cullIn(root, driver.Point{}, driver.Bounds{}, false)
}

// cullIn walks v, its parent's content at origin on the surface; reach,
// when bounded, is the surface rectangle its views must touch to be made.
func (s *surface) cullIn(v *view, origin driver.Point, reach driver.Bounds, bounded bool) {
	f := v.shownFrame()
	at := driver.Point{X: origin.X + f.Origin.X, Y: origin.Y + f.Origin.Y}
	if bounded {
		area := driver.Bounds{Origin: at, Size: f.Size}
		ink := v.sentInk
		ink.Origin.X += at.X
		ink.Origin.Y += at.Y
		area = union(area, ink)
		if o := v.overflow; o.Size.Width > 0 || o.Size.Height > 0 {
			o.Origin.X += at.X
			o.Origin.Y += at.Y
			area = union(area, o)
		}
		culled := !touches(area, reach)
		if culled != v.culled {
			v.culled = culled
			s.w.op(opCull, v.id)
			s.w.flag(culled)
		}
		if culled {
			return
		}
	}
	if v.kind == driver.KindScroll {
		// Its viewport, grown by a screen of it each way.
		port := driver.Bounds{Origin: driver.Point{X: at.X - f.Size.Width, Y: at.Y - f.Size.Height},
			Size: driver.Size{Width: 3 * f.Size.Width, Height: 3 * f.Size.Height}}
		if bounded {
			port = intersect(port, reach)
		}
		reach, bounded = port, true
	}
	content := driver.Point{X: at.X - v.offset.X, Y: at.Y - v.offset.Y}
	for _, c := range v.children {
		s.cullIn(c, content, reach, bounded)
	}
}

func touches(a, b driver.Bounds) bool {
	return a.Origin.X <= b.Origin.X+b.Size.Width && b.Origin.X <= a.Origin.X+a.Size.Width &&
		a.Origin.Y <= b.Origin.Y+b.Size.Height && b.Origin.Y <= a.Origin.Y+a.Size.Height
}

func intersect(a, b driver.Bounds) driver.Bounds {
	x0, y0 := max(a.Origin.X, b.Origin.X), max(a.Origin.Y, b.Origin.Y)
	x1 := min(a.Origin.X+a.Size.Width, b.Origin.X+b.Size.Width)
	y1 := min(a.Origin.Y+a.Size.Height, b.Origin.Y+b.Size.Height)
	return driver.Bounds{Origin: driver.Point{X: x0, Y: y0}, Size: driver.Size{Width: max(x1-x0, 0), Height: max(y1-y0, 0)}}
}
