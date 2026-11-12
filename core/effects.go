package core

import "github.com/nsgo-dev/nsgo/internal/driver"

// The look beyond React Native's view styles: per-corner radii, dashed
// borders, outlines, layered shadows, gradients, filters, blend modes and
// keyframe animations, as CSS draws them.

type (
	// Corners holds a radius per corner of a box.
	Corners = driver.Corners
	// BorderStyle is how a border or an outline is drawn; the zero value is
	// solid.
	BorderStyle = driver.BorderStyle
	// BoxShadow is one of CSS's box-shadow layers: the box's shape grown
	// by Spread, moved by Offset and blurred by Blur (CSS's blur radius),
	// inside the box when Inset.
	BoxShadow = driver.BoxShadow
	// Gradient is a linear, radial or conic gradient image, as CSS's.
	Gradient = driver.Gradient
	// GradientKind is a gradient's geometry.
	GradientKind = driver.GradientKind
	// GradientCorner points a linear gradient at a corner.
	GradientCorner = driver.GradientCorner
	// GradientExtent is how far a radial gradient's last stop reaches.
	GradientExtent = driver.GradientExtent
	// ColorSpace is where a gradient's colors mix between stops.
	ColorSpace = driver.ColorSpace
	// ColorStop is a color at a place along a gradient.
	ColorStop = driver.ColorStop
	// Filter is one of CSS's filter functions; Blur, Brightness and the
	// other constructors make them.
	Filter = driver.Filter
	// FilterKind is which filter function a Filter is.
	FilterKind = driver.FilterKind
	// BlendMode is how a view's pixels mix with what is behind it.
	BlendMode = driver.BlendMode
	// BackdropMaterial is the system material iOS draws a backdrop blur
	// with.
	BackdropMaterial = driver.BackdropMaterial
	// GlassEffect is Apple's Liquid Glass.
	GlassEffect = driver.GlassEffect
	// Keyframes are a keyframe animation's steps.
	Keyframes = driver.Keyframes
	// KeyframeAnimation runs Keyframes over a view on the platform.
	KeyframeAnimation = driver.KeyframeAnimation
	// Keyframe is one step of a keyframe animation.
	Keyframe = driver.Keyframe
	// GradientPaint is a gradient resolved against a box, what platforms
	// draw.
	GradientPaint = driver.GradientPaint
	// ColorMatrix is a 4×5 color matrix in rows, as SVG's feColorMatrix.
	ColorMatrix = driver.ColorMatrix
	// Matrix3D is a 4×4 transform, stored column by column as CSS's
	// matrix3d() lists it.
	Matrix3D = driver.Matrix3D
	// Outline is a line around a view's border box, as platforms draw it.
	Outline = driver.Outline
	// Effects are the shadows, gradients, filters and mask a view draws.
	Effects = driver.Effects
)

const (
	BorderSolid  = driver.BorderSolid
	BorderDashed = driver.BorderDashed
	BorderDotted = driver.BorderDotted
	BorderNone   = driver.BorderNone
	BorderHidden = driver.BorderHidden
)

const (
	GradientLinear = driver.GradientLinear
	GradientRadial = driver.GradientRadial
	GradientConic  = driver.GradientConic
)

const (
	ToTopLeft     = driver.ToTopLeft
	ToTopRight    = driver.ToTopRight
	ToBottomRight = driver.ToBottomRight
	ToBottomLeft  = driver.ToBottomLeft
)

const (
	ExtentFarthestCorner = driver.ExtentFarthestCorner
	ExtentClosestSide    = driver.ExtentClosestSide
	ExtentClosestCorner  = driver.ExtentClosestCorner
	ExtentFarthestSide   = driver.ExtentFarthestSide
)

const (
	ColorSpaceOklab      = driver.ColorSpaceOklab
	ColorSpaceSRGB       = driver.ColorSpaceSRGB
	ColorSpaceLinearSRGB = driver.ColorSpaceLinearSRGB
	ColorSpaceOklch      = driver.ColorSpaceOklch
)

const (
	FilterBlur       = driver.FilterBlur
	FilterBrightness = driver.FilterBrightness
	FilterContrast   = driver.FilterContrast
	FilterGrayscale  = driver.FilterGrayscale
	FilterHueRotate  = driver.FilterHueRotate
	FilterInvert     = driver.FilterInvert
	FilterOpacity    = driver.FilterOpacity
	FilterSaturate   = driver.FilterSaturate
	FilterSepia      = driver.FilterSepia
	FilterDropShadow = driver.FilterDropShadow
)

const (
	BlendNormal      = driver.BlendNormal
	BlendMultiply    = driver.BlendMultiply
	BlendScreen      = driver.BlendScreen
	BlendOverlay     = driver.BlendOverlay
	BlendDarken      = driver.BlendDarken
	BlendLighten     = driver.BlendLighten
	BlendColorDodge  = driver.BlendColorDodge
	BlendColorBurn   = driver.BlendColorBurn
	BlendHardLight   = driver.BlendHardLight
	BlendSoftLight   = driver.BlendSoftLight
	BlendDifference  = driver.BlendDifference
	BlendExclusion   = driver.BlendExclusion
	BlendHue         = driver.BlendHue
	BlendSaturation  = driver.BlendSaturation
	BlendColor       = driver.BlendColor
	BlendLuminosity  = driver.BlendLuminosity
	BlendPlusLighter = driver.BlendPlusLighter
	BlendPlusDarker  = driver.BlendPlusDarker
)

const (
	BackdropUltraThin = driver.BackdropUltraThin
	BackdropThin      = driver.BackdropThin
	BackdropRegular   = driver.BackdropRegular
	BackdropThick     = driver.BackdropThick
	BackdropChrome    = driver.BackdropChrome
)

const (
	GlassNone    = driver.GlassNone
	GlassRegular = driver.GlassRegular
	GlassClear   = driver.GlassClear
)

const (
	KeyframesSpin   = driver.KeyframesSpin
	KeyframesPing   = driver.KeyframesPing
	KeyframesPulse  = driver.KeyframesPulse
	KeyframesBounce = driver.KeyframesBounce
)

// The intrinsic sizes, for Width, Height and their limits: the narrowest a
// box gets without overflowing (its longest word), the widest its content
// asks for (no wrapping), and that up to the space it has.
var (
	MinContent = driver.MinContent
	MaxContent = driver.MaxContent
	FitContent = driver.FitContent
)

// AutoMinimum is CSS's automatic minimum, for MinWidth and MinHeight: a
// flex item keeps its content's size, where an unset minimum is 0.
var AutoMinimum = driver.AutoMinimum

// CurrentColor, in a style's colors other than Color, is the text color
// the view has: its own, else the one it inherits, else black, as CSS's
// currentColor. It is the one color value set aside: it never draws as
// itself.
const CurrentColor = driver.CurrentColor

// Transparent is a color that draws nothing but, unlike 0, is set: a
// border Transparent colors draws none, where one left 0 draws in the
// text color.
const Transparent Color = 0x00000100

// SetColor returns c as a style field takes a color the app set:
// Transparent when c is fully transparent black, 0, which would leave the
// field unset.
func SetColor(c Color) Color { return cmpOr(c, Transparent) }

// AllCorners returns corners of the same radius.
func AllCorners(r Points) Corners { return driver.AllCorners(r) }

// Ring returns the shadow Tailwind's ring utilities draw: a solid line
// width wide around the box, outside its border, taking no space. A ring
// offset is a ring in the offset's color over a wider one, as Tailwind
// stacks them:
//
//	Shadows: []BoxShadow{Ring(2, white), Ring(4, blue)}
func Ring(width Points, c Color) BoxShadow { return driver.Ring(width, c) }

// LinearGradient returns a gradient along angle, in degrees clockwise from
// up (90 runs left to right), through stops spread evenly.
func LinearGradient(angle float32, colors ...Color) Gradient {
	return Gradient{Kind: GradientLinear, Angle: angle, Stops: stopsOf(colors)}
}

// RadialGradient returns an elliptical gradient from the center out to
// the farthest corner, through stops spread evenly.
func RadialGradient(colors ...Color) Gradient {
	return Gradient{Kind: GradientRadial, Stops: stopsOf(colors)}
}

// ConicGradient returns a gradient around the center, starting at angle in
// degrees clockwise from up, through stops spread evenly over a turn.
func ConicGradient(angle float32, colors ...Color) Gradient {
	return Gradient{Kind: GradientConic, Angle: angle, Stops: stopsOf(colors)}
}

func stopsOf(colors []Color) []ColorStop {
	stops := make([]ColorStop, len(colors))
	for i, c := range colors {
		stops[i].Color = c
	}
	return stops
}

// Blur blurs what a view draws (in Filters) or what is behind it (in
// BackdropFilters) by radius, CSS's blur().
func Blur(radius Points) Filter { return Filter{Kind: FilterBlur, Amount: float32(radius)} }

// Brightness multiplies the colors by f: 0 is black, 1 unchanged.
func Brightness(f float32) Filter { return Filter{Kind: FilterBrightness, Amount: f} }

// Contrast scales the colors away from grey by f: 0 is grey, 1 unchanged.
func Contrast(f float32) Filter { return Filter{Kind: FilterContrast, Amount: f} }

// Grayscale takes f of the color away: 1 is grey.
func Grayscale(f float32) Filter { return Filter{Kind: FilterGrayscale, Amount: f} }

// HueRotate turns the hues by degrees.
func HueRotate(degrees float32) Filter { return Filter{Kind: FilterHueRotate, Amount: degrees} }

// Invert inverts f of the colors: 1 is a negative.
func Invert(f float32) Filter { return Filter{Kind: FilterInvert, Amount: f} }

// Fade multiplies the alpha by f, CSS's opacity().
func Fade(f float32) Filter { return Filter{Kind: FilterOpacity, Amount: f} }

// Saturate scales the saturation by f: 0 is grey, 1 unchanged.
func Saturate(f float32) Filter { return Filter{Kind: FilterSaturate, Amount: f} }

// Sepia tones f of the colors sepia.
func Sepia(f float32) Filter { return Filter{Kind: FilterSepia, Amount: f} }

// DropShadow draws a shadow of what the view draws, in its shape rather
// than its box's: offset, blur radius and color.
func DropShadow(offset Point, blur Points, c Color) Filter {
	return Filter{Kind: FilterDropShadow, Shadow: BoxShadow{Offset: offset, Blur: blur, Color: c}}
}
