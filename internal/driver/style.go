package driver

import (
	"math"
	"time"
)

// FontWeight is a CSS font weight from 100 to 900; 0 is unset.
type FontWeight uint16

const (
	FontWeightLight    FontWeight = 300
	FontWeightNormal   FontWeight = 400
	FontWeightMedium   FontWeight = 500
	FontWeightSemibold FontWeight = 600
	FontWeightBold     FontWeight = 700
)

// TextAlign aligns lines of text.
type TextAlign uint8

const (
	TextAlignStart TextAlign = iota + 1
	TextAlignCenter
	TextAlignEnd
	// TextAlignJustify stretches every line but a paragraph's last to the
	// box's width, aligned to the start.
	TextAlignJustify
)

// VerticalAlign is where an attachment sits in its line, as CSS's
// vertical-align places a replaced inline box, against the font of the
// text around it. Its line grows to hold it wherever it reaches past that
// font.
type VerticalAlign uint8

const (
	// VerticalAlignBaseline puts its bottom on the baseline; it is the
	// default.
	VerticalAlignBaseline VerticalAlign = iota + 1
	// VerticalAlignTextTop puts its top at the font's ascent.
	VerticalAlignTextTop
	// VerticalAlignTextBottom puts its bottom at the font's descent.
	VerticalAlignTextBottom
	// VerticalAlignMiddle puts its middle half the font's x height above
	// the baseline.
	VerticalAlignMiddle
)

// TextDecoration draws lines on text; the values combine.
type TextDecoration uint8

const (
	TextDecorationNone        TextDecoration = 1 << iota
	TextDecorationUnderline                  // underline
	TextDecorationLineThrough                // strikethrough
)

// TextDecorationStyle is how a text's decoration lines are drawn; the
// zero value is solid.
type TextDecorationStyle uint8

const (
	DecorationSolid TextDecorationStyle = iota + 1
	DecorationDouble
	DecorationDotted
	DecorationDashed
	DecorationWavy
)

// DefaultFontSize is the font size of text no ancestor sizes: 14, as
// NSH's and MyGo's text defaults.
const DefaultFontSize Points = 14

// EdgeColors is a color for each side of a box.
type EdgeColors struct{ Top, Right, Bottom, Left Color }

// AllColors returns edge colors of the same color on every side.
func AllColors(c Color) EdgeColors { return EdgeColors{c, c, c, c} }

// Or returns e with its unset sides c.
func (e EdgeColors) Or(c Color) EdgeColors {
	for _, side := range []*Color{&e.Top, &e.Right, &e.Bottom, &e.Left} {
		if *side == 0 {
			*side = c
		}
	}
	return e
}

// Transform moves, scales, rotates and skews a view without changing its
// layout, around its transform origin (its center unless the style says),
// in CSS's order: perspective, translate, rotate, rotateX, rotateY, skew,
// then scale. The zero value changes nothing.
type Transform struct {
	TranslateX, TranslateY Points
	// TranslateXPercent and TranslateYPercent move the view by percentages
	// of its own width and height, which the core adds to TranslateX and
	// TranslateY each time the size changes.
	TranslateXPercent, TranslateYPercent float32
	// Scale scales both axes; ScaleX and ScaleY one each. 0 is unset (1).
	Scale, ScaleX, ScaleY float32
	// Rotate is in degrees, clockwise. RotateX and RotateY turn the view
	// out of its plane around the horizontal and vertical axes, which
	// shows depth only with a Perspective, its own or its parent's.
	Rotate, RotateX, RotateY float32
	// RotateZ turns the view in its plane after RotateX and RotateY: the
	// transform property's rotateZ, where Rotate is CSS's rotate property,
	// which goes before them.
	RotateZ float32
	// SkewX and SkewY slant the view along each axis, in degrees.
	SkewX, SkewY float32
	// Perspective is the distance the view is seen from, for its own
	// rotations out of its plane; 0 is none.
	Perspective Points
}

// Flat reports whether t stays in the view's plane, a 2D affine
// transform.
func (t Transform) Flat() bool { return t.RotateX == 0 && t.RotateY == 0 }

// Scales returns the x and y scale factors.
func (t Transform) Scales() (x, y float32) {
	x, y = 1, 1
	if t.Scale != 0 {
		x, y = t.Scale, t.Scale
	}
	if t.ScaleX != 0 {
		x = t.ScaleX
	}
	if t.ScaleY != 0 {
		y = t.ScaleY
	}
	return x, y
}

// Transition animates a view's look when its style changes: the platform
// runs it on the main thread, with no work for the engine per frame.
type Transition struct {
	Duration time.Duration
	Delay    time.Duration
	Easing   Curve
	// Properties lists what animates; every property but
	// TransitionLayout when 0.
	Properties TransitionProperty
}

// Layout returns how t moves a view's frame (TransitionLayout).
func (t Transition) Layout() *LayoutTransition {
	return &LayoutTransition{Curve: LayoutBezier, Bezier: t.Easing, Duration: t.Duration, Delay: t.Delay}
}

// TransitionProperty is a set of animatable properties.
type TransitionProperty uint8

const (
	TransitionOpacity TransitionProperty = 1 << iota
	TransitionBackground
	TransitionTransform
	TransitionBorder
	TransitionShadow
	// TransitionLayout moves and resizes the view from where it shows
	// when a change to its own style changes its frame. Its children take
	// their new frames inside it at once.
	TransitionLayout
)

// Curve is an easing curve: CSS's cubic Bézier from (0,0) to (1,1)
// through two control points. Its zero value is linear.
type Curve struct{ X1, Y1, X2, Y2 float32 }

// The standard curves, as CSS names them.
var (
	Linear    = Curve{}
	Ease      = Curve{0.25, 0.1, 0.25, 1}
	EaseIn    = Curve{0.42, 0, 1, 1}
	EaseOut   = Curve{0, 0, 0.58, 1}
	EaseInOut = Curve{0.42, 0, 0.58, 1}
)

// At returns the curve's progress at time fraction x from 0 to 1.
func (c Curve) At(x float64) float64 {
	if c == (Curve{}) || x <= 0 || x >= 1 {
		return math.Min(math.Max(x, 0), 1)
	}
	x1, y1, x2, y2 := float64(c.X1), float64(c.Y1), float64(c.X2), float64(c.Y2)
	bez := func(t, p1, p2 float64) float64 {
		u := 1 - t
		return 3*u*u*t*p1 + 3*u*t*t*p2 + t*t*t
	}
	// Find t with bez(t) == x on the x axis by bisection, then map to y.
	lo, hi := 0.0, 1.0
	for range 40 {
		mid := (lo + hi) / 2
		if bez(mid, x1, x2) < x {
			lo = mid
		} else {
			hi = mid
		}
	}
	return bez((lo+hi)/2, y1, y2)
}

// FrameInterval is a frame at 60 Hz, in whole microseconds as NSH's:
// what an app assumes where the display's own pace is unknown. Sixty of
// them reach a second, so a second of frames counts 60.
const FrameInterval = 16667 * time.Microsecond

// Cursor is the pointer's look over a view (cursor). Auto leaves the
// look to the view under it or its nearest ancestor that names one; a
// pressable view's Auto is a pointer on the web.
type Cursor uint8

const (
	CursorAuto Cursor = iota
	CursorDefault
	CursorPointer
	CursorText
	CursorWait
	CursorProgress
	CursorHelp
	CursorCrosshair
	CursorMove
	CursorGrab
	CursorGrabbing
	CursorNotAllowed
	CursorZoomIn
	CursorZoomOut
	CursorColResize
	CursorRowResize
	CursorNone
)

var cursorNames = [...]string{
	"auto", "default", "pointer", "text", "wait", "progress", "help", "crosshair",
	"move", "grab", "grabbing", "not-allowed", "zoom-in", "zoom-out",
	"col-resize", "row-resize", "none",
}

// Name returns the cursor's CSS name.
func (c Cursor) Name() string {
	if int(c) < len(cursorNames) {
		return cursorNames[c]
	}
	return cursorNames[CursorAuto]
}
