package driver

import "math"

// Matrix3D is a 4×4 transform of column vectors, stored column by column
// as CSS's matrix3d() lists it and Core Animation's CATransform3D lays it
// out: a point (x, y, z) maps to (M[0]x + M[4]y + M[8]z + M[12], ...)
// divided by M[3]x + M[7]y + M[11]z + M[15].
type Matrix3D [16]float64

// Identity3D changes nothing.
var Identity3D = Matrix3D{1, 0, 0, 0, 0, 1, 0, 0, 0, 0, 1, 0, 0, 0, 0, 1}

// Mul returns m · n: n applied first, then m.
func (m Matrix3D) Mul(n Matrix3D) Matrix3D {
	var out Matrix3D
	for c := range 4 {
		for r := range 4 {
			var v float64
			for k := range 4 {
				v += m[k*4+r] * n[c*4+k]
			}
			out[c*4+r] = v
		}
	}
	return out
}

// Translate3D moves by x, y and z.
func Translate3D(x, y, z float64) Matrix3D {
	m := Identity3D
	m[12], m[13], m[14] = x, y, z
	return m
}

// Perspective3D sees the plane z = 0 from d away, as CSS's perspective().
func Perspective3D(d float64) Matrix3D {
	m := Identity3D
	if d > 0 {
		m[11] = -1 / d
	}
	return m
}

// Affine reports whether m is a 2D transform, and its a, b, c, d, e, f as
// SVG's matrix() lists them.
func (m Matrix3D) Affine() (a, b, c, d, e, f float64, ok bool) {
	ok = m[2] == 0 && m[3] == 0 && m[6] == 0 && m[7] == 0 && m[8] == 0 && m[9] == 0 &&
		m[10] == 1 && m[11] == 0 && m[14] == 0 && m[15] == 1
	return m[0], m[1], m[4], m[5], m[12], m[13], ok
}

// Apply maps a point of the plane through m, with perspective.
func (m Matrix3D) Apply(x, y float64) (float64, float64) {
	w := m[3]*x + m[7]*y + m[15]
	if w == 0 {
		w = 1e-9
	}
	return (m[0]*x + m[4]*y + m[12]) / w, (m[1]*x + m[5]*y + m[13]) / w
}

// Invert returns m's inverse, and false when it has none.
func (m Matrix3D) Invert() (Matrix3D, bool) {
	var inv Matrix3D
	inv[0] = m[5]*m[10]*m[15] - m[5]*m[11]*m[14] - m[9]*m[6]*m[15] + m[9]*m[7]*m[14] + m[13]*m[6]*m[11] - m[13]*m[7]*m[10]
	inv[4] = -m[4]*m[10]*m[15] + m[4]*m[11]*m[14] + m[8]*m[6]*m[15] - m[8]*m[7]*m[14] - m[12]*m[6]*m[11] + m[12]*m[7]*m[10]
	inv[8] = m[4]*m[9]*m[15] - m[4]*m[11]*m[13] - m[8]*m[5]*m[15] + m[8]*m[7]*m[13] + m[12]*m[5]*m[11] - m[12]*m[7]*m[9]
	inv[12] = -m[4]*m[9]*m[14] + m[4]*m[10]*m[13] + m[8]*m[5]*m[14] - m[8]*m[6]*m[13] - m[12]*m[5]*m[10] + m[12]*m[6]*m[9]
	inv[1] = -m[1]*m[10]*m[15] + m[1]*m[11]*m[14] + m[9]*m[2]*m[15] - m[9]*m[3]*m[14] - m[13]*m[2]*m[11] + m[13]*m[3]*m[10]
	inv[5] = m[0]*m[10]*m[15] - m[0]*m[11]*m[14] - m[8]*m[2]*m[15] + m[8]*m[3]*m[14] + m[12]*m[2]*m[11] - m[12]*m[3]*m[10]
	inv[9] = -m[0]*m[9]*m[15] + m[0]*m[11]*m[13] + m[8]*m[1]*m[15] - m[8]*m[3]*m[13] - m[12]*m[1]*m[11] + m[12]*m[3]*m[9]
	inv[13] = m[0]*m[9]*m[14] - m[0]*m[10]*m[13] - m[8]*m[1]*m[14] + m[8]*m[2]*m[13] + m[12]*m[1]*m[10] - m[12]*m[2]*m[9]
	inv[2] = m[1]*m[6]*m[15] - m[1]*m[7]*m[14] - m[5]*m[2]*m[15] + m[5]*m[3]*m[14] + m[13]*m[2]*m[7] - m[13]*m[3]*m[6]
	inv[6] = -m[0]*m[6]*m[15] + m[0]*m[7]*m[14] + m[4]*m[2]*m[15] - m[4]*m[3]*m[14] - m[12]*m[2]*m[7] + m[12]*m[3]*m[6]
	inv[10] = m[0]*m[5]*m[15] - m[0]*m[7]*m[13] - m[4]*m[1]*m[15] + m[4]*m[3]*m[13] + m[12]*m[1]*m[7] - m[12]*m[3]*m[5]
	inv[14] = -m[0]*m[5]*m[14] + m[0]*m[6]*m[13] + m[4]*m[1]*m[14] - m[4]*m[2]*m[13] - m[12]*m[1]*m[6] + m[12]*m[2]*m[5]
	inv[3] = -m[1]*m[6]*m[11] + m[1]*m[7]*m[10] + m[5]*m[2]*m[11] - m[5]*m[3]*m[10] - m[9]*m[2]*m[7] + m[9]*m[3]*m[6]
	inv[7] = m[0]*m[6]*m[11] - m[0]*m[7]*m[10] - m[4]*m[2]*m[11] + m[4]*m[3]*m[10] + m[8]*m[2]*m[7] - m[8]*m[3]*m[6]
	inv[11] = -m[0]*m[5]*m[11] + m[0]*m[7]*m[9] + m[4]*m[1]*m[11] - m[4]*m[3]*m[9] - m[8]*m[1]*m[7] + m[8]*m[3]*m[5]
	inv[15] = m[0]*m[5]*m[10] - m[0]*m[6]*m[9] - m[4]*m[1]*m[10] + m[4]*m[2]*m[9] + m[8]*m[1]*m[6] - m[8]*m[2]*m[5]
	det := m[0]*inv[0] + m[1]*inv[4] + m[2]*inv[8] + m[3]*inv[12]
	if det == 0 || math.IsNaN(det) {
		return Matrix3D{}, false
	}
	for i := range inv {
		inv[i] /= det
	}
	return inv, true
}

// Matrix returns t as a matrix acting around origin, a point in the
// view's own coordinates (its top left at 0, 0).
func (t Transform) Matrix(origin Point) Matrix3D {
	ox, oy := float64(origin.X), float64(origin.Y)
	m := Translate3D(ox, oy, 0).Mul(Perspective3D(float64(t.Perspective)))
	m = m.Mul(Translate3D(float64(t.TranslateX), float64(t.TranslateY), 0))
	if t.Rotate != 0 {
		s, c := math.Sincos(float64(t.Rotate) * math.Pi / 180)
		m = m.Mul(Matrix3D{c, s, 0, 0, -s, c, 0, 0, 0, 0, 1, 0, 0, 0, 0, 1})
	}
	if t.RotateX != 0 {
		s, c := math.Sincos(float64(t.RotateX) * math.Pi / 180)
		m = m.Mul(Matrix3D{1, 0, 0, 0, 0, c, s, 0, 0, -s, c, 0, 0, 0, 0, 1})
	}
	if t.RotateY != 0 {
		s, c := math.Sincos(float64(t.RotateY) * math.Pi / 180)
		m = m.Mul(Matrix3D{c, 0, -s, 0, 0, 1, 0, 0, s, 0, c, 0, 0, 0, 0, 1})
	}
	if t.RotateZ != 0 {
		s, c := math.Sincos(float64(t.RotateZ) * math.Pi / 180)
		m = m.Mul(Matrix3D{c, s, 0, 0, -s, c, 0, 0, 0, 0, 1, 0, 0, 0, 0, 1})
	}
	if t.SkewX != 0 || t.SkewY != 0 {
		m = m.Mul(Matrix3D{1, math.Tan(float64(t.SkewY) * math.Pi / 180), 0, 0,
			math.Tan(float64(t.SkewX) * math.Pi / 180), 1, 0, 0, 0, 0, 1, 0, 0, 0, 0, 1})
	}
	sx, sy := t.Scales()
	if sx != 1 || sy != 1 {
		m = m.Mul(Matrix3D{float64(sx), 0, 0, 0, 0, float64(sy), 0, 0, 0, 0, 1, 0, 0, 0, 0, 1})
	}
	return m.Mul(Translate3D(-ox, -oy, 0))
}

// ChildPerspective returns the matrix a parent's perspective of depth d
// puts before the transform of a child whose top left is at offset from
// the parent's, seen from the parent's center of size: CSS's perspective
// with its default origin.
func ChildPerspective(d Points, parent Size, offset Point) Matrix3D {
	if d <= 0 {
		return Identity3D
	}
	cx, cy := float64(parent.Width/2-offset.X), float64(parent.Height/2-offset.Y)
	return Translate3D(cx, cy, 0).Mul(Perspective3D(float64(d))).Mul(Translate3D(-cx, -cy, 0))
}
