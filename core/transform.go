package core

import (
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/oneo-me/nsgo/internal/driver"
)

// TransformKind is a function of CSS's transform property.
type TransformKind uint8

const (
	TransformTranslateX TransformKind = iota + 1
	TransformTranslateY
	TransformScaleX
	TransformScaleY
	TransformRotate
	TransformRotateX
	TransformRotateY
	TransformSkewX
	TransformSkewY
	TransformPerspective
)

// TransformStep is one function of a transform list: Value is in points,
// degrees or a factor, and for Percent a translation's percentage of the
// view's own size.
type TransformStep struct {
	Kind    TransformKind
	Value   float32
	Percent bool
}

// ParseTransform reads CSS's transform: its functions, or none, which is
// an empty list.
func ParseTransform(s string) ([]TransformStep, error) {
	s = strings.TrimSpace(s)
	steps := []TransformStep{}
	if s == "none" {
		return steps, nil
	}
	bad := fmt.Errorf("core: %q is no transform", s)
	for rest := s; rest != ""; rest = strings.TrimSpace(rest) {
		open, end := strings.IndexByte(rest, '('), strings.IndexByte(rest, ')')
		if open <= 0 || end < open {
			return nil, bad
		}
		name, args := rest[:open], strings.FieldsFunc(rest[open+1:end], func(r rune) bool { return r == ',' || r == ' ' })
		rest = rest[end+1:]
		step, ok := transformSteps(name, args)
		if !ok {
			return nil, bad
		}
		steps = append(steps, step...)
	}
	return steps, nil
}

// transformSteps reads a function: an axis pair is two steps.
func transformSteps(name string, args []string) ([]TransformStep, bool) {
	if len(args) == 0 || len(args) > 2 {
		return nil, false
	}
	var read func(string) (TransformStep, bool)
	switch name {
	case "translate", "translateX", "translateY":
		read = translateStep
	case "scale", "scaleX", "scaleY":
		read = func(s string) (TransformStep, bool) {
			f, err := strconv.ParseFloat(s, 32)
			return TransformStep{Value: float32(f)}, err == nil
		}
	case "rotate", "rotateZ", "rotateX", "rotateY", "skew", "skewX", "skewY":
		read = func(s string) (TransformStep, bool) {
			a, ok := cssAngle(s)
			return TransformStep{Value: a}, ok
		}
	case "perspective":
		read = func(s string) (TransformStep, bool) {
			l, ok := cssLength(s)
			return TransformStep{Value: float32(l)}, ok
		}
	default:
		return nil, false
	}
	one := func(kind TransformKind, arg string) (TransformStep, bool) {
		st, ok := read(arg)
		st.Kind = kind
		return st, ok
	}
	kinds := map[string]TransformKind{"translateX": TransformTranslateX, "translateY": TransformTranslateY,
		"scaleX": TransformScaleX, "scaleY": TransformScaleY, "rotate": TransformRotate, "rotateZ": TransformRotate,
		"rotateX": TransformRotateX, "rotateY": TransformRotateY, "skewX": TransformSkewX, "skewY": TransformSkewY,
		"perspective": TransformPerspective}
	if kind, ok := kinds[name]; ok {
		if len(args) != 1 {
			return nil, false
		}
		st, ok := one(kind, args[0])
		return []TransformStep{st}, ok
	}
	x, y := map[string]TransformKind{"translate": TransformTranslateX, "scale": TransformScaleX, "skew": TransformSkewX}[name],
		map[string]TransformKind{"translate": TransformTranslateY, "scale": TransformScaleY, "skew": TransformSkewY}[name]
	second := "0"
	if len(args) == 2 {
		second = args[1]
	} else if name == "scale" {
		second = args[0]
	}
	sx, ok := one(x, args[0])
	sy, ok2 := one(y, second)
	return []TransformStep{sx, sy}, ok && ok2
}

// translateStep reads a length or a percentage.
func translateStep(s string) (TransformStep, bool) {
	if p, ok := strings.CutSuffix(s, "%"); ok {
		f, err := strconv.ParseFloat(p, 32)
		return TransformStep{Value: float32(f), Percent: true}, err == nil
	}
	l, ok := cssLength(s)
	return TransformStep{Value: float32(l)}, ok
}

// matrix returns the step's matrix in a view of size.
func (st TransformStep) matrix(size Size) driver.Matrix3D {
	v := float64(st.Value)
	m := driver.Identity3D
	sin, cos := math.Sincos(v * math.Pi / 180)
	switch st.Kind {
	case TransformTranslateX:
		if st.Percent {
			v *= float64(size.Width) / 100
		}
		m[12] = v
	case TransformTranslateY:
		if st.Percent {
			v *= float64(size.Height) / 100
		}
		m[13] = v
	case TransformScaleX:
		m[0] = v
	case TransformScaleY:
		m[5] = v
	case TransformRotate:
		m[0], m[1], m[4], m[5] = cos, sin, -sin, cos
	case TransformRotateX:
		m[5], m[6], m[9], m[10] = cos, sin, -sin, cos
	case TransformRotateY:
		m[0], m[2], m[8], m[10] = cos, -sin, sin, cos
	case TransformSkewX:
		m[4] = sin / cos
	case TransformSkewY:
		m[1] = sin / cos
	case TransformPerspective:
		return driver.Perspective3D(v)
	}
	return m
}

// frameTransform is s's transform for a view of size, its percentages
// resolved. A list in the order Transform's fields apply them, with
// nothing else set, goes in those fields, which transitions move one by
// one and every platform has; any other goes as a matrix.
func (s *Style) frameTransform(size Size) Transform {
	t := s.Transform
	t.TranslateX += Points(t.TranslateXPercent) * size.Width / 100
	t.TranslateY += Points(t.TranslateYPercent) * size.Height / 100
	t.TranslateXPercent, t.TranslateYPercent = 0, 0
	if len(s.Transforms) == 0 {
		return t
	}
	if t == (Transform{}) {
		if f, ok := transformFields(s.Transforms, size); ok {
			return f
		}
	}
	m := driver.Identity3D
	for _, st := range s.Transforms {
		m = m.Mul(st.matrix(size))
	}
	t.Then = m
	return t
}

// transformFields puts steps in Transform's fields, if they are in the
// fields' order, each once.
func transformFields(steps []TransformStep, size Size) (Transform, bool) {
	order := [...]TransformKind{TransformPerspective, TransformTranslateX, TransformTranslateY, TransformRotate,
		TransformRotateX, TransformRotateY, TransformSkewX, TransformSkewY, TransformScaleX, TransformScaleY}
	var t Transform
	at := 0
	for _, st := range steps {
		for at < len(order) && order[at] != st.Kind {
			at++
		}
		if at == len(order) {
			return Transform{}, false
		}
		at++
		switch st.Kind {
		case TransformPerspective:
			t.Perspective = Points(st.Value)
		case TransformTranslateX:
			t.TranslateX = Points(st.matrix(size)[12])
		case TransformTranslateY:
			t.TranslateY = Points(st.matrix(size)[13])
		case TransformRotate:
			t.Rotate = st.Value
		case TransformRotateX:
			t.RotateX = st.Value
		case TransformRotateY:
			t.RotateY = st.Value
		case TransformSkewX:
			t.SkewX = st.Value
		case TransformSkewY:
			t.SkewY = st.Value
		case TransformScaleX:
			t.ScaleX = st.Value
		case TransformScaleY:
			t.ScaleY = st.Value
		}
	}
	return t, true
}
