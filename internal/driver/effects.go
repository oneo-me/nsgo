package driver

import (
	"math"
	"slices"
	"time"
)

// BorderStyle is how a border or an outline is drawn. The zero value is
// solid.
type BorderStyle uint8

const (
	BorderSolid BorderStyle = iota + 1
	// BorderDashed draws dashes three widths long, three apart.
	BorderDashed
	// BorderDotted draws round dots a width across, a width apart.
	BorderDotted
	// BorderNone and BorderHidden draw no border, and layout gives it
	// no width, whatever the widths say.
	BorderNone
	BorderHidden
)

// Dashes returns the dash pattern of a line of width drawn in style, on
// then off, with round caps for dots; ok is false for solid lines.
func (s BorderStyle) Dashes(width Points) (on, off Points, round, ok bool) {
	switch s {
	case BorderDashed:
		return 3 * width, 3 * width, false, true
	case BorderDotted:
		return 0, 2 * width, true, true
	}
	return 0, 0, false, false
}

// BoxShadow is one shadow of a view's box, as one of CSS's box-shadow
// layers: its shape is the box's, grown by Spread, moved by Offset and
// blurred by Blur, CSS's blur radius (twice the Gaussian's standard
// deviation). An inset shadow falls inside the box, over its background,
// from the edges inwards.
type BoxShadow struct {
	Offset Point
	Blur   Points
	Spread Points
	Color  Color
	Inset  bool
}

// Ring returns the shadow Tailwind's ring utilities draw: a solid line
// width wide around the box, outside its border, taking no space.
func Ring(width Points, c Color) BoxShadow { return BoxShadow{Spread: width, Color: c} }

// Outline is a line around a view's border box, outside layout, Offset
// away from it (inside when negative), following its corners.
type Outline struct {
	Width  Points
	Offset Points
	Color  Color
	// Style is solid when 0; none and hidden draw nothing.
	Style BorderStyle
}

// Visible reports whether the outline draws anything.
func (o Outline) Visible() bool {
	return o.Width > 0 && o.Color&0xff != 0 && o.Style != BorderNone && o.Style != BorderHidden
}

// Filter is one of CSS's filter functions. Filters apply in order; the
// constructors in core make them.
type Filter struct {
	Kind FilterKind
	// Amount is the function's argument: the blur's radius in points,
	// hue rotation's degrees, opacity's alpha, otherwise a factor, where
	// 1 leaves brightness, contrast and saturation as they are and gives
	// the full effect of grayscale, invert and sepia.
	Amount float32
	// Shadow is a drop shadow's offset, blur and color.
	Shadow BoxShadow
}

// FilterKind is which filter function a Filter is.
type FilterKind uint8

const (
	FilterBlur FilterKind = iota + 1
	FilterBrightness
	FilterContrast
	FilterGrayscale
	FilterHueRotate
	FilterInvert
	FilterOpacity
	FilterSaturate
	FilterSepia
	// FilterDropShadow draws a shadow of what the view draws, in its
	// shape rather than its box's.
	FilterDropShadow
)

// ColorMatrix is a 4×5 color matrix in rows, over unpremultiplied red,
// green, blue and alpha from 0 to 1 and a constant 1, as SVG's
// feColorMatrix.
type ColorMatrix [20]float32

// IdentityColorMatrix changes nothing.
var IdentityColorMatrix = ColorMatrix{1, 0, 0, 0, 0, 0, 1, 0, 0, 0, 0, 0, 1, 0, 0, 0, 0, 0, 1, 0}

// Then returns the matrix that applies m, then n.
func (m ColorMatrix) Then(n ColorMatrix) ColorMatrix {
	var out ColorMatrix
	for r := range 4 {
		for c := range 5 {
			var v float32
			for k := range 4 {
				v += n[r*5+k] * m[k*5+c]
			}
			if c == 4 {
				v += n[r*5+4]
			}
			out[r*5+c] = v
		}
	}
	return out
}

// Matrix returns the color matrix of a color filter, as CSS's Filter
// Effects define them, and false for a blur or a drop shadow.
func (f Filter) Matrix() (ColorMatrix, bool) {
	a := f.Amount
	rgb := func(r, g, b [3]float32) ColorMatrix {
		return ColorMatrix{r[0], r[1], r[2], 0, 0, g[0], g[1], g[2], 0, 0, b[0], b[1], b[2], 0, 0, 0, 0, 0, 1, 0}
	}
	switch f.Kind {
	case FilterBrightness:
		return rgb([3]float32{a, 0, 0}, [3]float32{0, a, 0}, [3]float32{0, 0, a}), true
	case FilterContrast:
		t := (1 - a) / 2
		return ColorMatrix{a, 0, 0, 0, t, 0, a, 0, 0, t, 0, 0, a, 0, t, 0, 0, 0, 1, 0}, true
	case FilterGrayscale:
		g := 1 - min(max(a, 0), 1)
		return rgb([3]float32{0.2126 + 0.7874*g, 0.7152 - 0.7152*g, 0.0722 - 0.0722*g},
			[3]float32{0.2126 - 0.2126*g, 0.7152 + 0.2848*g, 0.0722 - 0.0722*g},
			[3]float32{0.2126 - 0.2126*g, 0.7152 - 0.7152*g, 0.0722 + 0.9278*g}), true
	case FilterSepia:
		s := 1 - min(max(a, 0), 1)
		return rgb([3]float32{0.393 + 0.607*s, 0.769 - 0.769*s, 0.189 - 0.189*s},
			[3]float32{0.349 - 0.349*s, 0.686 + 0.314*s, 0.168 - 0.168*s},
			[3]float32{0.272 - 0.272*s, 0.534 - 0.534*s, 0.131 + 0.869*s}), true
	case FilterSaturate:
		return rgb([3]float32{0.213 + 0.787*a, 0.715 - 0.715*a, 0.072 - 0.072*a},
			[3]float32{0.213 - 0.213*a, 0.715 + 0.285*a, 0.072 - 0.072*a},
			[3]float32{0.213 - 0.213*a, 0.715 - 0.715*a, 0.072 + 0.928*a}), true
	case FilterHueRotate:
		s64, c64 := math.Sincos(float64(a) * math.Pi / 180)
		s, c := float32(s64), float32(c64)
		return rgb([3]float32{0.213 + c*0.787 - s*0.213, 0.715 - c*0.715 - s*0.715, 0.072 - c*0.072 + s*0.928},
			[3]float32{0.213 - c*0.213 + s*0.143, 0.715 + c*0.285 + s*0.140, 0.072 - c*0.072 - s*0.283},
			[3]float32{0.213 - c*0.213 - s*0.787, 0.715 - c*0.715 + s*0.715, 0.072 + c*0.928 + s*0.072}), true
	case FilterInvert:
		v := min(max(a, 0), 1)
		k := 1 - 2*v
		return ColorMatrix{k, 0, 0, 0, v, 0, k, 0, 0, v, 0, 0, k, 0, v, 0, 0, 0, 1, 0}, true
	case FilterOpacity:
		m := IdentityColorMatrix
		m[18] = min(max(a, 0), 1)
		return m, true
	}
	return ColorMatrix{}, false
}

// SplitFilters returns what platforms that cannot chain filters draw of
// filters: their color filters as one matrix (ok false when there are
// none), the widest blur and the drop shadows.
func SplitFilters(filters []Filter) (m ColorMatrix, ok bool, blur Points, shadows []BoxShadow) {
	m = IdentityColorMatrix
	for _, f := range filters {
		switch f.Kind {
		case FilterBlur:
			blur = max(blur, Points(f.Amount))
		case FilterDropShadow:
			shadows = append(shadows, f.Shadow)
		default:
			if fm, isColor := f.Matrix(); isColor {
				m, ok = m.Then(fm), true
			}
		}
	}
	return m, ok, blur, shadows
}

// BlendMode is how a view's pixels mix with what is drawn behind it, as
// CSS's mix-blend-mode.
type BlendMode uint8

const (
	BlendNormal BlendMode = iota
	BlendMultiply
	BlendScreen
	BlendOverlay
	BlendDarken
	BlendLighten
	BlendColorDodge
	BlendColorBurn
	BlendHardLight
	BlendSoftLight
	BlendDifference
	BlendExclusion
	BlendHue
	BlendSaturation
	BlendColor
	BlendLuminosity
	BlendPlusLighter
	// BlendPlusDarker is a linear burn: the sum of the colors less white.
	BlendPlusDarker
)

var blendNames = [...]string{"normal", "multiply", "screen", "overlay", "darken", "lighten", "color-dodge",
	"color-burn", "hard-light", "soft-light", "difference", "exclusion", "hue", "saturation", "color",
	"luminosity", "plus-lighter", "plus-darker"}

// Name returns the blend mode's CSS name.
func (b BlendMode) Name() string {
	if int(b) < len(blendNames) {
		return blendNames[b]
	}
	return blendNames[0]
}

// BackdropMaterial is the system material iOS draws a backdrop blur
// with, from the thinnest.
type BackdropMaterial uint8

const (
	BackdropUltraThin BackdropMaterial = iota
	BackdropThin
	BackdropRegular
	BackdropThick
	BackdropChrome
)

// GlassEffect is Apple's Liquid Glass, drawn in place of the backdrop
// blur where the system has it (iOS and macOS 26).
type GlassEffect uint8

const (
	GlassNone GlassEffect = iota
	GlassRegular
	GlassClear
)

// Effects are the rarer parts of a view's look. They travel behind a
// pointer, so ViewStyle stays comparable: the core sends the same
// pointer while they stay Equal.
type Effects struct {
	// Shadows are drawn under the box, or inside it over the background
	// when inset; the first on top, as CSS orders them.
	Shadows []BoxShadow
	// Gradients are drawn over the background, under the border, the
	// first on top.
	Gradients []GradientPaint
	// Filters change what the view and its children draw, in order.
	Filters []Filter
	// BackdropFilters change what is behind the view, under its
	// background; platforms draw their blur.
	BackdropFilters []Filter
	// Mask, when set, is drawn as much as its alpha says, backdrop
	// included.
	Mask *GradientPaint
}

// Empty reports whether e has nothing to draw.
func (e *Effects) Empty() bool {
	return e == nil || len(e.Shadows) == 0 && len(e.Gradients) == 0 && len(e.Filters) == 0 &&
		len(e.BackdropFilters) == 0 && e.Mask == nil
}

// Equal reports whether e and o draw the same.
func (e *Effects) Equal(o *Effects) bool {
	if e == o {
		return true
	}
	if e.Empty() || o.Empty() {
		return e.Empty() && o.Empty()
	}
	return slices.Equal(e.Shadows, o.Shadows) && slices.Equal(e.Filters, o.Filters) &&
		slices.Equal(e.BackdropFilters, o.BackdropFilters) && e.Mask.Equal(o.Mask) &&
		slices.EqualFunc(e.Gradients, o.Gradients, func(a, b GradientPaint) bool { return a.Equal(&b) })
}

// BackdropBlur is the blur radius of the backdrop filters, 0 for none.
func (e *Effects) BackdropBlur() Points {
	if e == nil {
		return 0
	}
	var blur Points
	for _, f := range e.BackdropFilters {
		if f.Kind == FilterBlur {
			blur = max(blur, Points(f.Amount))
		}
	}
	return blur
}

// Keyframes are a keyframe animation's steps: Tailwind's animate-spin,
// animate-ping, animate-pulse and animate-bounce.
type Keyframes uint8

const (
	// KeyframesSpin turns the view once a second, for loading indicators.
	KeyframesSpin Keyframes = iota + 1
	// KeyframesPing scales the view to twice its size as it fades out, as
	// a radar ping or a notification badge's halo.
	KeyframesPing
	// KeyframesPulse fades the view to half and back every two seconds,
	// for skeleton loaders.
	KeyframesPulse
	// KeyframesBounce drops the view a quarter of its height and back.
	KeyframesBounce
)

// KeyframeAnimation runs Keyframes over a view on the platform's frame
// clock, with no work for the engine per frame. Its zero Duration and
// Easing are the keyframes' own (Tailwind's); a zero Iterations repeats
// forever.
type KeyframeAnimation struct {
	Keyframes  Keyframes
	Duration   time.Duration
	Delay      time.Duration
	Easing     Curve
	Iterations int
}

// Keyframe is one step of a keyframe animation: at Offset of each cycle
// the view's opacity is multiplied by Opacity, it is scaled by Scale,
// turned Rotate degrees and moved down TranslateY times its height, on
// top of its own transform, and Easing takes it to the next step.
type Keyframe struct {
	Offset     float32
	Opacity    float32
	Scale      float32
	Rotate     float32
	TranslateY float32
	Easing     Curve
}

// Cycle returns a's duration and steps, with their defaults filled in.
func (a KeyframeAnimation) Cycle() (time.Duration, []Keyframe) {
	d := a.Duration
	ease := a.Easing
	var frames []Keyframe
	still := Keyframe{Opacity: 1, Scale: 1}
	at := func(offset float32, k Keyframe) Keyframe { k.Offset = offset; return k }
	switch a.Keyframes {
	case KeyframesSpin:
		d = cmpOr(d, time.Second)
		end := still
		end.Rotate = 360
		frames = []Keyframe{at(0, still), at(1, end)}
	case KeyframesPing:
		d = cmpOr(d, time.Second)
		ease = curveOr(ease, Curve{0, 0, 0.2, 1})
		out := Keyframe{Opacity: 0, Scale: 2}
		frames = []Keyframe{at(0, still), at(0.75, out), at(1, out)}
	case KeyframesPulse:
		d = cmpOr(d, 2*time.Second)
		ease = curveOr(ease, Curve{0.4, 0, 0.6, 1})
		half := still
		half.Opacity = 0.5
		frames = []Keyframe{at(0, still), at(0.5, half), at(1, still)}
	case KeyframesBounce:
		d = cmpOr(d, time.Second)
		up := still
		up.TranslateY = -0.25
		// Falling speeds up and rising slows down, whatever Easing says.
		up.Easing = Curve{0.8, 0, 1, 1}
		down := still
		down.Easing = Curve{0, 0, 0.2, 1}
		return d, []Keyframe{at(0, up), at(0.5, down), at(1, up)}
	default:
		return 0, nil
	}
	for i := range frames {
		frames[i].Easing = ease
	}
	return d, frames
}

func cmpOr(d, or time.Duration) time.Duration {
	if d > 0 {
		return d
	}
	return or
}

func curveOr(c, or Curve) Curve {
	if c != (Curve{}) {
		return c
	}
	return or
}

// At returns the step a's view is at t into its run (Delay included),
// and false once it has finished.
func (a KeyframeAnimation) At(t time.Duration) (Keyframe, bool) {
	d, frames := a.Cycle()
	if d <= 0 || len(frames) == 0 {
		return Keyframe{Opacity: 1, Scale: 1}, false
	}
	t -= a.Delay
	if t < 0 {
		return frames[0], true
	}
	cycle := int(t / d)
	if a.Iterations > 0 && cycle >= a.Iterations {
		return frames[len(frames)-1], false
	}
	x := float32(t%d) / float32(d)
	for i := 1; i < len(frames); i++ {
		a, b := frames[i-1], frames[i]
		if x > b.Offset {
			continue
		}
		f := float32(0)
		if b.Offset > a.Offset {
			f = float32(a.Easing.At(float64((x - a.Offset) / (b.Offset - a.Offset))))
		}
		lerp := func(p, q float32) float32 { return p + (q-p)*f }
		return Keyframe{Offset: x, Opacity: lerp(a.Opacity, b.Opacity), Scale: lerp(a.Scale, b.Scale),
			Rotate: lerp(a.Rotate, b.Rotate), TranslateY: lerp(a.TranslateY, b.TranslateY)}, true
	}
	return frames[len(frames)-1], true
}
