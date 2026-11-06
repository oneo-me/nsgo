package driver

import "math"

// Points is a length in platform-independent points (iOS points, Android dp,
// CSS pixels at a zoom of 1).
type Points float32

// Auto marks a length the layout computes, such as a text engine's
// unbounded width. Styles use [Length] instead.
var Auto = Points(float32(math.NaN()))

// IsAuto reports whether p is Auto.
func (p Points) IsAuto() bool { return p != p }

// Point is a position in points.
type Point struct{ X, Y Points }

// Size is a width and a height in points.
type Size struct{ Width, Height Points }

// Bounds is a rectangle: its origin relative to the parent's and its size.
type Bounds struct {
	Origin Point
	Size   Size
}

// Contains reports whether p lies inside b.
func (b Bounds) Contains(p Point) bool {
	return p.X >= b.Origin.X && p.Y >= b.Origin.Y &&
		p.X < b.Origin.X+b.Size.Width && p.Y < b.Origin.Y+b.Size.Height
}

// Edges holds a length per side.
type Edges struct{ Top, Right, Bottom, Left Points }

// All returns edges of the same length on every side.
func All(p Points) Edges { return Edges{p, p, p, p} }

// Symmetric returns edges with vertical and horizontal lengths.
func Symmetric(vertical, horizontal Points) Edges {
	return Edges{vertical, horizontal, vertical, horizontal}
}

// Horizontal returns the left and right edges together.
func (e Edges) Horizontal() Points { return e.Left + e.Right }

// Vertical returns the top and bottom edges together.
func (e Edges) Vertical() Points { return e.Top + e.Bottom }

// Length is a style's length. Its zero value is auto: the layout decides.
type Length struct {
	value float32
	unit  unit
}

type unit uint8

const (
	unitAuto unit = iota
	unitPoints
	unitPercent
	unitMinContent
	unitMaxContent
	unitFitContent
	unitAutoMinimum
)

// The intrinsic sizes, for widths and heights: the narrowest a box gets
// without overflowing (its longest word), the widest its content asks for
// (no wrapping), and the widest it asks for up to the space it has.
var (
	MinContent = Length{unit: unitMinContent}
	MaxContent = Length{unit: unitMaxContent}
	FitContent = Length{unit: unitFitContent}
)

// AutoMinimum is CSS's automatic minimum, for a minimum size: a flex
// item keeps its content's size, where an unset minimum is 0, as in Yoga.
var AutoMinimum = Length{unit: unitAutoMinimum}

// Pt returns a length in points.
func Pt(p Points) Length { return Length{float32(p), unitPoints} }

// Pct returns a length in percent of the containing block's size.
func Pct(percent float32) Length { return Length{percent, unitPercent} }

// IsAuto reports whether l is auto.
func (l Length) IsAuto() bool { return l.unit == unitAuto }

// Points returns l's value in points, and whether l is a length in
// points.
func (l Length) Points() (Points, bool) {
	return Points(l.value), l.unit == unitPoints
}

// Percent returns l's value in percent, and whether l is a percentage.
func (l Length) Percent() (float32, bool) {
	return l.value, l.unit == unitPercent
}

// Resolve returns l in points against base, or Auto. A percentage of an
// unknown base is Auto.
func (l Length) Resolve(base Points) Points {
	switch l.unit {
	case unitPoints:
		return Points(l.value)
	case unitPercent:
		if base.IsAuto() {
			return Auto
		}
		return base * Points(l.value) / 100
	}
	return Auto
}

// Color is an sRGB color packed as 0xRRGGBBAA, the wire form of mount
// items. Its zero value is unset: no background, or text color inherited
// from the parent.
type Color uint32

// RGB returns an opaque color from 0xRRGGBB.
func RGB(hex uint32) Color { return Color(hex<<8 | 0xff) }

// RGBA returns a color from 0xRRGGBB and an alpha from 0 to 1.
func RGBA(hex uint32, alpha float32) Color {
	a := uint32(min(max(alpha, 0), 1)*255 + 0.5)
	return Color(hex<<8 | a)
}

// Faded returns c with its alpha multiplied by f, from 0 to 1.
func (c Color) Faded(f float32) Color {
	a := uint32(float32(c&0xff)*min(max(f, 0), 1) + 0.5)
	return c&^0xff | Color(a)
}

// Components returns the red, green, blue and alpha channels from 0 to 1.
func (c Color) Components() (r, g, b, a float64) {
	return float64(c>>24&0xff) / 255, float64(c>>16&0xff) / 255,
		float64(c>>8&0xff) / 255, float64(c&0xff) / 255
}
