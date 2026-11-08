package layout

import "math"

// Size is a width and a height.
type Size[T any] struct {
	Width, Height T
}

// Rect holds one value per side of a box.
type Rect[T any] struct {
	Left, Right, Top, Bottom T
}

// Point is a position.
type Point[T any] struct {
	X, Y T
}

// Line is a start and an end, such as a grid item's start and end lines or
// a range on one axis.
type Line[T any] struct {
	Start, End T
}

// AbsoluteAxis is the horizontal or vertical axis.
type AbsoluteAxis uint8

const (
	Horizontal AbsoluteAxis = iota
	Vertical
)

// other returns the axis perpendicular to a.
func (a AbsoluteAxis) other() AbsoluteAxis {
	if a == Horizontal {
		return Vertical
	}
	return Horizontal
}

// AbstractAxis is the inline or block axis. Taffy only supports the
// horizontal-tb writing mode, so the inline axis is always horizontal.
type AbstractAxis uint8

const (
	Inline AbstractAxis = iota
	Block
)

func (a AbstractAxis) other() AbstractAxis {
	if a == Inline {
		return Block
	}
	return Inline
}

func (a AbstractAxis) asAbs() AbsoluteAxis {
	if a == Inline {
		return Horizontal
	}
	return Vertical
}

// Opt is a float32 that may be absent, Rust's Option<f32>. The zero Opt is
// absent.
type Opt struct {
	Value float32
	Valid bool
}

// Some returns a present Opt.
func Some(v float32) Opt { return Opt{v, true} }

// None is the absent Opt.
var None Opt

// Or returns the value, or d when it is absent.
func (o Opt) Or(d float32) float32 {
	if o.Valid {
		return o.Value
	}
	return d
}

// OrOpt returns o if it is present and d otherwise.
func (o Opt) OrOpt(d Opt) Opt {
	if o.Valid {
		return o
	}
	return d
}

// The maybe-math of Taffy's MaybeMath trait: an absent left operand stays
// absent, an absent right operand leaves the left one unchanged.

func (o Opt) min(b Opt) Opt {
	if o.Valid && b.Valid {
		return Some(fmin(o.Value, b.Value))
	}
	return o
}

func (o Opt) max(b Opt) Opt {
	if o.Valid && b.Valid {
		return Some(fmax(o.Value, b.Value))
	}
	return o
}

func (o Opt) clamp(lo, hi Opt) Opt {
	if !o.Valid {
		return o
	}
	return Some(fclamp(o.Value, lo, hi))
}

func (o Opt) add(b Opt) Opt {
	if o.Valid && b.Valid {
		return Some(o.Value + b.Value)
	}
	return o
}

func (o Opt) sub(b Opt) Opt {
	if o.Valid && b.Valid {
		return Some(o.Value - b.Value)
	}
	return o
}

func (o Opt) minF(b float32) Opt {
	if o.Valid {
		return Some(fmin(o.Value, b))
	}
	return o
}

func (o Opt) maxF(b float32) Opt {
	if o.Valid {
		return Some(fmax(o.Value, b))
	}
	return o
}

func (o Opt) clampF(lo, hi float32) Opt {
	if o.Valid {
		return Some(fmax(fmin(o.Value, hi), lo))
	}
	return o
}

func (o Opt) addF(b float32) Opt {
	if o.Valid {
		return Some(o.Value + b)
	}
	return o
}

func (o Opt) subF(b float32) Opt {
	if o.Valid {
		return Some(o.Value - b)
	}
	return o
}

// fmin and fmax are Rust's f32::min and f32::max: a NaN operand yields the
// other operand.
func fmin(a, b float32) float32 {
	switch {
	case a < b:
		return a
	case b < a:
		return b
	case a != a:
		return b
	case b != b:
		return a
	case math.Float32bits(a)>>31 != 0:
		return a
	}
	return b
}

func fmax(a, b float32) float32 {
	switch {
	case a > b:
		return a
	case b > a:
		return b
	case a != a:
		return b
	case b != b:
		return a
	case math.Float32bits(a)>>31 != 0:
		return b
	}
	return a
}

// fclamp clamps v to the present bounds, max first then min, as Taffy does.
func fclamp(v float32, lo, hi Opt) float32 {
	if hi.Valid {
		v = fmin(v, hi.Value)
	}
	if lo.Valid {
		v = fmax(v, lo.Value)
	}
	return v
}

func fminOpt(a float32, b Opt) float32 {
	if b.Valid {
		return fmin(a, b.Value)
	}
	return a
}

func fmaxOpt(a float32, b Opt) float32 {
	if b.Valid {
		return fmax(a, b.Value)
	}
	return a
}

func faddOpt(a float32, b Opt) float32 {
	if b.Valid {
		return a + b.Value
	}
	return a
}

func fsubOpt(a float32, b Opt) float32 {
	if b.Valid {
		return a - b.Value
	}
	return a
}

// round rounds half up, as Taffy's rounding does.
func floor32(v float32) float32 { return float32(math.Floor(float64(v))) }

func ceil32(v float32) float32 { return float32(math.Ceil(float64(v))) }

func abs32(v float32) float32 { return math.Float32frombits(math.Float32bits(v) &^ (1 << 31)) }

func isFinite32(v float32) bool { return !math.IsInf(float64(v), 0) && v == v }

var (
	inf32    = float32(math.Inf(1))
	negInf32 = float32(math.Inf(-1))
)

// epsilon32 is Rust's f32::EPSILON.
const epsilon32 = 1.1920929e-07

// Size helpers.

func (s Size[T]) get(axis AbsoluteAxis) T {
	if axis == Horizontal {
		return s.Width
	}
	return s.Height
}

func (s *Size[T]) set(axis AbsoluteAxis, v T) {
	if axis == Horizontal {
		s.Width = v
	} else {
		s.Height = v
	}
}

func (s Size[T]) getAbstract(axis AbstractAxis) T { return s.get(axis.asAbs()) }

func (s *Size[T]) setAbstract(axis AbstractAxis, v T) { s.set(axis.asAbs(), v) }

func (s Size[T]) withAbstract(axis AbstractAxis, v T) Size[T] {
	s.setAbstract(axis, v)
	return s
}

func (s Size[T]) transpose() Size[T] { return Size[T]{s.Height, s.Width} }

// main and cross read the size along a flex container's axes.
func (s Size[T]) main(dir FlexDirection) T {
	if dir.isRow() {
		return s.Width
	}
	return s.Height
}

func (s Size[T]) cross(dir FlexDirection) T {
	if dir.isRow() {
		return s.Height
	}
	return s.Width
}

func (s *Size[T]) setMain(dir FlexDirection, v T) {
	if dir.isRow() {
		s.Width = v
	} else {
		s.Height = v
	}
}

func (s *Size[T]) setCross(dir FlexDirection, v T) {
	if dir.isRow() {
		s.Height = v
	} else {
		s.Width = v
	}
}

func (s Size[T]) withMain(dir FlexDirection, v T) Size[T] {
	s.setMain(dir, v)
	return s
}

func (s Size[T]) withCross(dir FlexDirection, v T) Size[T] {
	s.setCross(dir, v)
	return s
}

// fromCross returns a size with v on dir's cross axis and None on its main
// axis.
func fromCross(dir FlexDirection, v Opt) Size[Opt] {
	var s Size[Opt]
	s.setCross(dir, v)
	return s
}

func sizeAdd(a, b Size[float32]) Size[float32] {
	return Size[float32]{a.Width + b.Width, a.Height + b.Height}
}

func sizeSub(a, b Size[float32]) Size[float32] {
	return Size[float32]{a.Width - b.Width, a.Height - b.Height}
}

// sizeMax and sizeMin are Taffy's Size::f32_max and Size::f32_min.
func sizeMax(a, b Size[float32]) Size[float32] {
	return Size[float32]{fmax(a.Width, b.Width), fmax(a.Height, b.Height)}
}

func sizeMin(a, b Size[float32]) Size[float32] {
	return Size[float32]{fmin(a.Width, b.Width), fmin(a.Height, b.Height)}
}

func sizeHasNonZeroArea(s Size[float32]) bool { return s.Width > 0 && s.Height > 0 }

// sizeSome turns a size into one with both dimensions present.
func sizeSome(s Size[float32]) Size[Opt] { return Size[Opt]{Some(s.Width), Some(s.Height)} }

// sizeClamp, sizeMaxOpt, sizeMinOpt, sizeAddOpt and sizeSubOpt are
// Size<f32>'s maybe-math with a Size<Option<f32>> operand.
func sizeClamp(v Size[float32], lo, hi Size[Opt]) Size[float32] {
	return Size[float32]{fclamp(v.Width, lo.Width, hi.Width), fclamp(v.Height, lo.Height, hi.Height)}
}

func sizeMaxOpt(a Size[float32], b Size[Opt]) Size[float32] {
	return Size[float32]{fmaxOpt(a.Width, b.Width), fmaxOpt(a.Height, b.Height)}
}

func sizeMinOpt(a Size[float32], b Size[Opt]) Size[float32] {
	return Size[float32]{fminOpt(a.Width, b.Width), fminOpt(a.Height, b.Height)}
}

func sizeAddOpt(a Size[float32], b Size[Opt]) Size[float32] {
	return Size[float32]{faddOpt(a.Width, b.Width), faddOpt(a.Height, b.Height)}
}

func sizeSubOpt(a Size[float32], b Size[Opt]) Size[float32] {
	return Size[float32]{fsubOpt(a.Width, b.Width), fsubOpt(a.Height, b.Height)}
}

// The opts* helpers are Size<Option<f32>>'s methods.

var noneSize Size[Opt]

func optsOr(a, b Size[Opt]) Size[Opt] {
	return Size[Opt]{a.Width.OrOpt(b.Width), a.Height.OrOpt(b.Height)}
}

func optsUnwrapOr(a Size[Opt], b Size[float32]) Size[float32] {
	return Size[float32]{a.Width.Or(b.Width), a.Height.Or(b.Height)}
}

func optsBoth(a Size[Opt]) bool { return a.Width.Valid && a.Height.Valid }

func optsMin(a, b Size[Opt]) Size[Opt] {
	return Size[Opt]{a.Width.min(b.Width), a.Height.min(b.Height)}
}

func optsMax(a, b Size[Opt]) Size[Opt] {
	return Size[Opt]{a.Width.max(b.Width), a.Height.max(b.Height)}
}

func optsClamp(v, lo, hi Size[Opt]) Size[Opt] {
	return Size[Opt]{v.Width.clamp(lo.Width, hi.Width), v.Height.clamp(lo.Height, hi.Height)}
}

func optsAdd(a, b Size[Opt]) Size[Opt] {
	return Size[Opt]{a.Width.add(b.Width), a.Height.add(b.Height)}
}

func optsSub(a, b Size[Opt]) Size[Opt] {
	return Size[Opt]{a.Width.sub(b.Width), a.Height.sub(b.Height)}
}

func optsMinF(a Size[Opt], b Size[float32]) Size[Opt] {
	return Size[Opt]{a.Width.minF(b.Width), a.Height.minF(b.Height)}
}

func optsMaxF(a Size[Opt], b Size[float32]) Size[Opt] {
	return Size[Opt]{a.Width.maxF(b.Width), a.Height.maxF(b.Height)}
}

func optsAddF(a Size[Opt], b Size[float32]) Size[Opt] {
	return Size[Opt]{a.Width.addF(b.Width), a.Height.addF(b.Height)}
}

func optsSubF(a Size[Opt], b Size[float32]) Size[Opt] {
	return Size[Opt]{a.Width.subF(b.Width), a.Height.subF(b.Height)}
}

// optsAspectRatio is Size::maybe_apply_aspect_ratio: with a ratio, a size
// missing exactly one dimension gets it from the other.
func optsAspectRatio(s Size[Opt], ratio Opt) Size[Opt] {
	if !ratio.Valid {
		return s
	}
	switch {
	case s.Width.Valid && !s.Height.Valid:
		return Size[Opt]{s.Width, Some(s.Width.Value / ratio.Value)}
	case !s.Width.Valid && s.Height.Valid:
		return Size[Opt]{Some(s.Height.Value * ratio.Value), s.Height}
	}
	return s
}

// Rect helpers.

func (r Rect[T]) transpose() Rect[T] {
	return Rect[T]{Left: r.Top, Right: r.Bottom, Top: r.Left, Bottom: r.Right}
}

func (r Rect[T]) horizontal() Line[T] { return Line[T]{r.Left, r.Right} }

func (r Rect[T]) vertical() Line[T] { return Line[T]{r.Top, r.Bottom} }

func (r Rect[T]) mainStart(dir FlexDirection) T {
	if dir.isRow() {
		return r.Left
	}
	return r.Top
}

func (r Rect[T]) mainEnd(dir FlexDirection) T {
	if dir.isRow() {
		return r.Right
	}
	return r.Bottom
}

func (r Rect[T]) crossStart(dir FlexDirection) T {
	if dir.isRow() {
		return r.Top
	}
	return r.Left
}

func (r Rect[T]) crossEnd(dir FlexDirection) T {
	if dir.isRow() {
		return r.Bottom
	}
	return r.Right
}

func rectAdd(a, b Rect[float32]) Rect[float32] {
	return Rect[float32]{a.Left + b.Left, a.Right + b.Right, a.Top + b.Top, a.Bottom + b.Bottom}
}

func rectHSum(r Rect[float32]) float32 { return r.Left + r.Right }

func rectVSum(r Rect[float32]) float32 { return r.Top + r.Bottom }

func rectSumAxes(r Rect[float32]) Size[float32] {
	return Size[float32]{r.Left + r.Right, r.Top + r.Bottom}
}

// rectAxisSum is Rect::grid_axis_sum.
func rectAxisSum(r Rect[float32], axis AbsoluteAxis) float32 {
	if axis == Horizontal {
		return r.Left + r.Right
	}
	return r.Top + r.Bottom
}

func rectMainSum(r Rect[float32], dir FlexDirection) float32 {
	if dir.isRow() {
		return r.Left + r.Right
	}
	return r.Top + r.Bottom
}

func rectCrossSum(r Rect[float32], dir FlexDirection) float32 {
	if dir.isRow() {
		return r.Top + r.Bottom
	}
	return r.Left + r.Right
}

// rectUnion returns the smallest rectangle containing a and b, reading the
// fields as edge coordinates.
func rectUnion(a, b Rect[float32]) Rect[float32] {
	return Rect[float32]{
		Left:   fmin(a.Left, b.Left),
		Right:  fmax(a.Right, b.Right),
		Top:    fmin(a.Top, b.Top),
		Bottom: fmax(a.Bottom, b.Bottom),
	}
}

func rectUnwrapOr(r Rect[Opt], d float32) Rect[float32] {
	return Rect[float32]{r.Left.Or(d), r.Right.Or(d), r.Top.Or(d), r.Bottom.Or(d)}
}

// Point helpers.

func (p Point[T]) transpose() Point[T] { return Point[T]{p.Y, p.X} }

func (p Point[T]) getAbstract(axis AbstractAxis) T {
	if axis == Inline {
		return p.X
	}
	return p.Y
}

func (p *Point[T]) setAbstract(axis AbstractAxis, v T) {
	if axis == Inline {
		p.X = v
	} else {
		p.Y = v
	}
}

func (p Point[T]) main(dir FlexDirection) T {
	if dir.isRow() {
		return p.X
	}
	return p.Y
}

func (p Point[T]) cross(dir FlexDirection) T {
	if dir.isRow() {
		return p.Y
	}
	return p.X
}

func pointAdd(a, b Point[float32]) Point[float32] { return Point[float32]{a.X + b.X, a.Y + b.Y} }

// Line helpers.

func lineSum(l Line[float32]) float32 { return l.Start + l.End }
