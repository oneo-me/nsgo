package driver

import (
	"math"
	"slices"
)

// ScrollBehavior is how a scroll view scrolls, as NSH's Scrollable props
// (React Native's ScrollView props) say: Scroll's Behavior. Its
// zero value scrolls freely, bounces and decelerates as the platform does.
type ScrollBehavior struct {
	// Paging stops at multiples of the viewport.
	Paging bool
	// SnapToInterval stops at multiples of this length; 0 turns it off.
	SnapToInterval Points
	// SnapToOffsets stops at these offsets.
	SnapToOffsets []Points
	// SnapToAlignment is the point of a SnapToInterval item that aligns
	// with the viewport.
	SnapToAlignment SnapAlignment
	// DisableIntervalMomentum moves a fling one snap point at most.
	DisableIntervalMomentum bool
	// DecelerationRate is the velocity a fling keeps per millisecond:
	// DecelerationNormal, DecelerationFast or between; 0 is the
	// platform's.
	DecelerationRate float32
	// NoScroll keeps the user from scrolling it (React Native's
	// scrollEnabled false); the app still can.
	NoScroll bool
	// NoBounces stops drags at the edges instead of rubber banding.
	NoBounces bool
	// ContentInset is room around the content that scrolls with it.
	ContentInset Edges
	// KeyboardDismissMode is whether dragging dismisses the software
	// keyboard.
	KeyboardDismissMode KeyboardDismissMode
	// KeyboardShouldPersistTaps is whether a tap while the software
	// keyboard shows dismisses it.
	KeyboardShouldPersistTaps KeyboardTaps
	// RefreshIndicator is the height of the app's own pull-to-refresh
	// indicator, drawn from a pull value, which replaces the platform's
	// spinner: a pull refreshes once it reaches it, and refreshing keeps
	// it above the content. 0 shows the platform's spinner.
	RefreshIndicator Points
}

// RefreshPull is how far past its top a released pull refreshes with the
// platform's spinner, and RefreshSpace the room the spinner keeps above
// the content while refreshing.
const (
	RefreshPull  Points = 64
	RefreshSpace Points = 56
)

// RefreshThreshold is how far past the top a released pull refreshes:
// the app's indicator's height, or the spinner's distance.
func (b *ScrollBehavior) RefreshThreshold() Points {
	if b.RefreshIndicator > 0 {
		return b.RefreshIndicator
	}
	return RefreshPull
}

// RefreshInset is the room kept above the content while it refreshes:
// the app's indicator's height, or the spinner's.
func (b *ScrollBehavior) RefreshInset() Points {
	if b.RefreshIndicator > 0 {
		return b.RefreshIndicator
	}
	return RefreshSpace
}

// Deceleration rates, as UIKit's.
const (
	DecelerationNormal float32 = 0.998
	DecelerationFast   float32 = 0.99
)

// SnapAlignment is where a snapped item sits in the viewport.
type SnapAlignment uint8

const (
	SnapStart SnapAlignment = iota
	SnapCenter
	SnapEnd
)

// KeyboardDismissMode is whether dragging a scroll view dismisses the
// software keyboard.
type KeyboardDismissMode uint8

// KeyboardTaps is what a tap in a scroll view does while the software
// keyboard shows.
type KeyboardTaps uint8

const (
	// KeyboardTapsNever dismisses the keyboard, and the tap reaches
	// nothing else.
	KeyboardTapsNever KeyboardTaps = iota
	// KeyboardTapsAlways never dismisses it.
	KeyboardTapsAlways
	// KeyboardTapsHandled dismisses it unless something handles the tap.
	KeyboardTapsHandled
)

// SnapPoints returns the offsets, in order, a view snapping or paging
// along a viewport and content of those lengths stops at, or none when it
// does not snap. Backends without SnapTarget's hook, such as the web's
// CSS scroll snapping, mark them.
func (b *ScrollBehavior) SnapPoints(viewport, content Points) []Points {
	limit := max(content-viewport, 0)
	var points []Points
	switch {
	case b.SnapToInterval > 0:
		interval := b.SnapToInterval
		var shift Points // aligns the item at k * interval with the viewport
		switch b.SnapToAlignment {
		case SnapCenter:
			shift = (viewport - interval) / 2
		case SnapEnd:
			shift = viewport - interval
		}
		n := max(int(math.Ceil(float64((limit+shift)/interval))), 0)
		for i := 0; i <= n; i++ {
			points = append(points, min(max(Points(i)*interval-shift, 0), limit))
		}
	case len(b.SnapToOffsets) > 0:
		for _, o := range b.SnapToOffsets {
			points = append(points, min(max(o, 0), limit))
		}
		slices.Sort(points)
	case b.Paging:
		page := max(viewport, 1)
		n := int(math.Ceil(float64(limit / page)))
		for i := 0; i <= n; i++ {
			points = append(points, min(Points(i)*page, limit))
		}
	}
	return points
}

// Snaps reports whether flings end at snap points: snapping or paging.
func (b *ScrollBehavior) Snaps() bool {
	return b.Paging || b.SnapToInterval > 0 || len(b.SnapToOffsets) > 0
}

// Deceleration returns the velocity a fling keeps per millisecond:
// DecelerationRate, else DecelerationFast when it snaps and
// DecelerationNormal otherwise.
func (b *ScrollBehavior) Deceleration() float32 {
	switch {
	case b.DecelerationRate > 0 && b.DecelerationRate < 1:
		return b.DecelerationRate
	case b.Snaps():
		return DecelerationFast
	}
	return DecelerationNormal
}

// Projected returns where a glide from offset at velocity (points per
// millisecond) comes to rest, decelerating by rate per millisecond.
func Projected(offset Points, velocity, rate float32) Points {
	return offset - Points(velocity/float32(math.Log(float64(rate))))
}

// SnapTarget returns the snap point a fling ends at, as React Native's,
// and false when the view neither snaps nor pages. All values are along
// the main axis: offset is where the drag ended, projected where the
// fling would glide to (Projected), velocity its speed in points per
// millisecond (positive: the offset grows), for a viewport and content of
// those lengths. Pages, and DisableIntervalMomentum, move one snap point;
// other snapping rounds the projected offset in the fling's direction.
// Platforms call it, or Android's port, as the user lets go.
func (b *ScrollBehavior) SnapTarget(offset, projected Points, velocity float32, viewport, content Points) (Points, bool) {
	points := b.SnapPoints(viewport, content)
	if len(points) == 0 {
		return 0, false
	}
	limit := max(content-viewport, 0)
	const slow = 0.3 // slower releases settle at the nearest point
	step := b.DisableIntervalMomentum || b.Paging && b.SnapToInterval <= 0 && len(b.SnapToOffsets) == 0
	from := projected
	if step {
		from = offset
	}
	var target Points
	found := false
	switch {
	case velocity > slow:
		for _, p := range points {
			if step && p > from+0.5 || !step && p >= from-0.5 {
				target, found = p, true
				break
			}
		}
		if !found {
			target, found = limit, true
		}
	case velocity < -slow:
		for i := len(points) - 1; i >= 0; i-- {
			if p := points[i]; step && p < from-0.5 || !step && p <= from+0.5 {
				target, found = p, true
				break
			}
		}
		if !found {
			target, found = 0, true
		}
	default:
		target = points[0]
		for _, p := range points[1:] {
			if abs32(p-from) < abs32(target-from) {
				target = p
			}
		}
	}
	return target, true
}

func abs32(p Points) Points {
	if p < 0 {
		return -p
	}
	return p
}

// Equal reports whether b and o scroll the same.
func (b *ScrollBehavior) Equal(o *ScrollBehavior) bool {
	return b.Paging == o.Paging && b.SnapToInterval == o.SnapToInterval &&
		slices.Equal(b.SnapToOffsets, o.SnapToOffsets) && b.SnapToAlignment == o.SnapToAlignment &&
		b.DisableIntervalMomentum == o.DisableIntervalMomentum && b.DecelerationRate == o.DecelerationRate &&
		b.NoScroll == o.NoScroll && b.NoBounces == o.NoBounces && b.ContentInset == o.ContentInset &&
		b.KeyboardDismissMode == o.KeyboardDismissMode && b.KeyboardShouldPersistTaps == o.KeyboardShouldPersistTaps &&
		b.RefreshIndicator == o.RefreshIndicator
}

// ZoomRequest is ItemZoomTo's: the scale to zoom to, and the content
// point (unzoomed) to show in the middle of the viewport, or nil to keep
// the middle where it is.
type ZoomRequest struct {
	Scale  float32
	Center *Point
}

// Zooms reports whether the content zooms: MaxZoomScale above
// MinZoomScale.
func (c *ScrollConfig) Zooms() bool { return c.MaxZoomScale > c.MinZoomScale }

// Equal reports whether c and o configure a scroll view the same.
func (c *ScrollConfig) Equal(o *ScrollConfig) bool {
	return c.Horizontal == o.Horizontal && c.HideIndicator == o.HideIndicator && c.Behavior.Equal(&o.Behavior) &&
		c.HeaderInset == o.HeaderInset &&
		c.Refreshable == o.Refreshable && c.Refreshing == o.Refreshing &&
		c.MinZoomScale == o.MinZoomScale && c.MaxZoomScale == o.MaxZoomScale && c.ZoomScale == o.ZoomScale &&
		c.NoBouncesZoom == o.NoBouncesZoom && c.NoPinchZoom == o.NoPinchZoom && c.CenterContent == o.CenterContent &&
		slices.EqualFunc(c.Edges[:], o.Edges[:], func(a, b ScrollEdge) bool { return a.Equal(&b) })
}

// ClampZoom returns z within c's zoom scales.
func (c *ScrollConfig) ClampZoom(z float32) float32 {
	return min(max(z, c.MinZoomScale), c.MaxZoomScale)
}

// EdgeEffect is how a scroll view's side sets the views over it, its
// bars, apart from the content scrolling under them, as iOS 26's scroll
// edge effects do.
type EdgeEffect uint8

const (
	// EdgeEffectAutomatic is soft under the bars; iOS 26 and later also
	// draw their own under their system bars.
	EdgeEffectAutomatic EdgeEffect = iota
	// EdgeEffectSoft blurs the content under the bars progressively, most
	// at the side, and fades it into the background it scrolls over.
	EdgeEffectSoft
	// EdgeEffectHard frosts the content under the bars evenly, with a
	// hairline along their far edge.
	EdgeEffectHard
	// EdgeEffectHidden draws none, under system bars too.
	EdgeEffectHidden
)

// Side is a side of a box, in the order of Edges.
type Side uint8

const (
	SideTop Side = iota
	SideRight
	SideBottom
	SideLeft
)

// ScrollEdge is a scroll view's effect at one of its sides
// (ScrollConfig.Edges).
type ScrollEdge struct {
	Effect EdgeEffect
	// Extent is how far into the scroll view the bars over this side
	// reach from it, 0 without bars; Bars are their views, which iOS 26
	// shapes its effect under (UIScrollEdgeElementContainerInteraction).
	Extent Points
	Bars   []NodeID
	// Tint is the background the content scrolls over, which the effect
	// fades it into, and Line a hard edge's hairline.
	Tint, Line Color
}

// Equal reports whether e and o draw the same.
func (e *ScrollEdge) Equal(o *ScrollEdge) bool {
	return e.Effect == o.Effect && e.Extent == o.Extent && slices.Equal(e.Bars, o.Bars) && e.Tint == o.Tint && e.Line == o.Line
}

// What a platform drawing an edge effect itself draws, alike everywhere:
// a soft edge blurs by SoftEdgeBlur at the side, down to nothing
// SoftEdgeTail past the bars, under the tint from SoftEdgeTint; a hard
// edge blurs by HardEdgeBlur under the bars, under the tint at
// HardEdgeTint, and draws a hairline HairlineWidth wide past them. Blurs
// are a Gaussian's standard deviation, as CSS's blur() takes.
const (
	SoftEdgeBlur  Points  = 10
	SoftEdgeTail  Points  = 16
	SoftEdgeTint  float32 = 0.6
	HardEdgeBlur  Points  = 6.4
	HardEdgeTint  float32 = 0.8
	HairlineWidth Points  = 0.5
)

// A platform that cannot blur what is behind a view, Linux, fades the
// content into the tint alone, more opaque: from UnblurredSoftTint at a
// soft edge's side, and at UnblurredHardTint under a hard edge's bars.
const (
	UnblurredSoftTint float32 = 0.85
	UnblurredHardTint float32 = 0.95
)

// Drawn returns the effect a platform drawing it itself draws: soft for
// an automatic side with bars, and none without bars.
func (e *ScrollEdge) Drawn() EdgeEffect {
	switch {
	case e.Extent <= 0 || e.Effect == EdgeEffectHidden:
		return EdgeEffectHidden
	case e.Effect == EdgeEffectAutomatic:
		return EdgeEffectSoft
	}
	return e.Effect
}

// Band returns how far from its side the drawn effect reaches: a soft
// edge's past the bars by SoftEdgeTail, 0 for none.
func (e *ScrollEdge) Band() Points {
	switch e.Drawn() {
	case EdgeEffectSoft:
		return e.Extent + SoftEdgeTail
	case EdgeEffectHard:
		return e.Extent
	}
	return 0
}

// BandRect returns the band of side s of a view of size size, in its own
// points.
func BandRect(s Side, size Size, band Points) Bounds {
	switch s {
	case SideRight:
		band = min(band, size.Width)
		return Bounds{Origin: Point{X: size.Width - band}, Size: Size{Width: band, Height: size.Height}}
	case SideBottom:
		band = min(band, size.Height)
		return Bounds{Origin: Point{Y: size.Height - band}, Size: Size{Width: size.Width, Height: band}}
	case SideLeft:
		return Bounds{Size: Size{Width: min(band, size.Width), Height: size.Height}}
	}
	return Bounds{Size: Size{Width: size.Width, Height: min(band, size.Height)}}
}

// EdgeLine returns the line of a gradient across a band from side s, from
// the side to the band's other end, in fractions of the band.
func EdgeLine(s Side) (x0, y0, x1, y1 float32) {
	switch s {
	case SideRight:
		return 1, 0.5, 0, 0.5
	case SideBottom:
		return 0.5, 1, 0.5, 0
	case SideLeft:
		return 0, 0.5, 1, 0.5
	}
	return 0.5, 0, 0.5, 1
}

// HairlineRect returns where a hard edge's hairline is in its band of
// size band: along the band's other end, outside it.
func HairlineRect(s Side, band Size) Bounds {
	w := HairlineWidth
	switch s {
	case SideRight:
		return Bounds{Origin: Point{X: -w}, Size: Size{Width: w, Height: band.Height}}
	case SideBottom:
		return Bounds{Origin: Point{Y: -w}, Size: Size{Width: band.Width, Height: w}}
	case SideLeft:
		return Bounds{Origin: Point{X: band.Width}, Size: Size{Width: w, Height: band.Height}}
	}
	return Bounds{Origin: Point{Y: band.Height}, Size: Size{Width: band.Width, Height: w}}
}

// BlurLevel is a step of a soft edge's blur, which grows toward the side:
// what is under it blurred, shown wholly from the side to Full of the way
// across the band, and fading out from there to nothing at Gone. Blur
// adds to the levels drawn before it, whose blurs it reads, as variances
// add up; Total is the whole blur where it shows wholly, for a platform
// whose levels each read the content alone.
type BlurLevel struct {
	Blur, Total Points
	Full, Gone  float32
}

// finestLevel is the least blur a level shows wholly: below it, what is
// mixed with the sharp content looks as a blur between them.
const finestLevel Points = 1.5

// SoftEdgeLevels returns the levels of a soft edge's blur, from the
// finest, as MyGo's progressive blurs: their blurs double from one level
// to the next up to SoftEdgeBlur, and between two a pixel shows both
// mixed, which the eye takes for a blur between them.
func SoftEdgeLevels() []BlurLevel {
	most := SoftEdgeBlur
	his := []Points{most}
	for h := most; h > finestLevel; {
		h /= 2
		his = append(his, h)
	}
	var levels []BlurLevel
	var lo Points
	for i := len(his) - 1; i >= 0; i-- {
		hi := his[i]
		// The blur wanted falls from most at the side to 0 at the band's
		// end: it is hi at 1-hi/most of the way, and lo at 1-lo/most.
		levels = append(levels, BlurLevel{
			Blur:  Points(math.Sqrt(float64(hi*hi - lo*lo))),
			Total: hi,
			Full:  float32(1 - hi/most),
			Gone:  float32(1 - lo/most),
		})
		lo = hi
	}
	return levels
}
