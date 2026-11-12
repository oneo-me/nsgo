package wgsl

import (
	"fmt"

	"github.com/nsgo-dev/nsgo/internal/shaderir"
)

// kind is a WGSL type's kind; a vector's or a matrix's scalar is a kind
// too.
type kind int

const (
	kNone kind = iota
	kBool
	kI32
	kU32
	kF32
	kAbsInt   // an abstract integer: a literal's or a const's, before it takes a type
	kAbsFloat // an abstract float
	kVec
	kMat
	kArray
)

// ty is a WGSL type. The IR has no boolean vectors, so a vec<bool> is a
// float vector of 0 and 1 there (see boolVector).
type ty struct {
	k    kind
	elem kind // a vector's or a matrix's scalar
	n    int  // a vector's components, a matrix's columns, an array's length
	rows int  // a matrix's rows
	arr  *ty  // an array's element
}

var (
	tNone     = ty{}
	tBool     = ty{k: kBool}
	tI32      = ty{k: kI32}
	tU32      = ty{k: kU32}
	tF32      = ty{k: kF32}
	tAbsInt   = ty{k: kAbsInt}
	tAbsFloat = ty{k: kAbsFloat}
)

func vec(elem kind, n int) ty { return ty{k: kVec, elem: elem, n: n} }

func (t ty) isScalar() bool { return t.k >= kBool && t.k <= kAbsFloat }

// scalar is t's own kind for a scalar, else its components'.
func (t ty) scalar() kind {
	switch t.k {
	case kVec, kMat:
		return t.elem
	case kArray:
		return t.arr.scalar()
	}
	return t.k
}

func (t ty) abstract() bool {
	s := t.scalar()
	return (t.isScalar() || t.k == kVec) && (s == kAbsInt || s == kAbsFloat)
}

func isFloat(k kind) bool   { return k == kF32 || k == kAbsFloat }
func isInt(k kind) bool     { return k == kI32 || k == kU32 || k == kAbsInt }
func isNumeric(k kind) bool { return isFloat(k) || isInt(k) }

// withScalar is t with its scalar kind changed to s.
func (t ty) withScalar(s kind) ty {
	switch t.k {
	case kVec, kMat:
		t.elem = s
		return t
	}
	return ty{k: s}
}

// concrete is the type an abstract value takes where nothing gives it
// one: i32 for integers, f32 for floats.
func (t ty) concrete() ty {
	switch t.scalar() {
	case kAbsInt:
		return t.withScalar(kI32)
	case kAbsFloat:
		return t.withScalar(kF32)
	}
	return t
}

func (t ty) equal(u ty) bool {
	if t.k != u.k || t.elem != u.elem || t.n != u.n || t.rows != u.rows {
		return false
	}
	if t.k == kArray {
		return t.arr.equal(*u.arr)
	}
	return true
}

func scalarName(k kind) string {
	switch k {
	case kBool:
		return "bool"
	case kI32:
		return "i32"
	case kU32:
		return "u32"
	case kF32:
		return "f32"
	case kAbsInt:
		return "abstract-int"
	case kAbsFloat:
		return "abstract-float"
	}
	return "void"
}

func (t ty) String() string {
	switch t.k {
	case kVec:
		return fmt.Sprintf("vec%d<%s>", t.n, scalarName(t.elem))
	case kMat:
		return fmt.Sprintf("mat%dx%d<%s>", t.n, t.rows, scalarName(t.elem))
	case kArray:
		return fmt.Sprintf("array<%s, %d>", t.arr, t.n)
	}
	return scalarName(t.k)
}

// ir is t in the IR: a concrete type, a vec<bool> a float vector.
func (t ty) ir() shaderir.Type {
	t = t.concrete()
	switch t.k {
	case kBool:
		return shaderir.Type{Main: shaderir.Bool}
	case kI32:
		return shaderir.Type{Main: shaderir.Int}
	case kU32:
		return shaderir.Type{Main: shaderir.Uint}
	case kF32:
		return shaderir.Type{Main: shaderir.Float}
	case kVec:
		s := shaderir.Float
		switch t.elem {
		case kI32:
			s = shaderir.Int
		case kU32:
			s = shaderir.Uint
		}
		return shaderir.Type{Main: shaderir.VectorOf(s, t.n)}
	case kMat:
		return shaderir.Type{Main: [...]shaderir.BasicType{2: shaderir.Mat2, 3: shaderir.Mat3, 4: shaderir.Mat4}[t.n]}
	case kArray:
		return shaderir.Type{Main: shaderir.Array, Sub: []shaderir.Type{t.arr.ir()}, Length: t.n}
	}
	return shaderir.Type{}
}

// boolVector reports whether t is a vector of booleans, which the IR
// holds as floats of 0 and 1.
func (t ty) boolVector() bool { return t.k == kVec && t.elem == kBool }

// layout returns t's alignment and size in a uniform buffer, in bytes, as
// WGSL lays it out.
func (t ty) layout() (align, size int) {
	switch t.k {
	case kI32, kU32, kF32:
		return 4, 4
	case kVec:
		switch t.n {
		case 2:
			return 8, 8
		case 3:
			return 16, 12
		}
		return 16, 16
	case kArray:
		a, s := t.arr.layout()
		stride := (s + a - 1) / a * a
		stride = (stride + 15) / 16 * 16
		return max(a, 16), stride * t.n
	}
	return 4, 4
}

// lanes is how many lanes t's value takes among a program's uniforms.
func (t ty) lanes() int { return t.ir().DwordCount() }

// builtinTypes are the types WGSL names without a template, its
// predeclared aliases among them.
var builtinTypes = map[string]ty{
	"bool": tBool, "i32": tI32, "u32": tU32, "f32": tF32,
	"vec2f": vec(kF32, 2), "vec3f": vec(kF32, 3), "vec4f": vec(kF32, 4),
	"vec2i": vec(kI32, 2), "vec3i": vec(kI32, 3), "vec4i": vec(kI32, 4),
	"vec2u": vec(kU32, 2), "vec3u": vec(kU32, 3), "vec4u": vec(kU32, 4),
	"mat2x2f": {k: kMat, elem: kF32, n: 2, rows: 2}, "mat3x3f": {k: kMat, elem: kF32, n: 3, rows: 3},
	"mat4x4f": {k: kMat, elem: kF32, n: 4, rows: 4},
}

// matShapes are the matrix types by name, with their columns and rows.
var matShapes = map[string][2]int{
	"mat2x2": {2, 2}, "mat2x3": {2, 3}, "mat2x4": {2, 4}, "mat3x2": {3, 2}, "mat3x3": {3, 3}, "mat3x4": {3, 4},
	"mat4x2": {4, 2}, "mat4x3": {4, 3}, "mat4x4": {4, 4},
	"mat2x3f": {2, 3}, "mat2x4f": {2, 4}, "mat3x2f": {3, 2}, "mat3x4f": {3, 4}, "mat4x2f": {4, 2}, "mat4x3f": {4, 3},
}
