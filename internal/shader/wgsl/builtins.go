package wgsl

import (
	"sync"

	"github.com/nsgo-dev/nsgo/internal/shaderir"
)

type builtinFunc func(c *checker, args []value, pos int, out *[]shaderir.Stmt) value

// builtin returns the WGSL built-in function a fragment shader may call
// by name, of those the IR's languages share, or nil. The table is built
// on first use, so apps that compile no shader link none of it.
func builtin(name string) builtinFunc {
	builtinTable.Do(initBuiltins)
	return builtinTable.funcs[name]
}

var builtinTable struct {
	sync.Once
	funcs map[string]builtinFunc
}

func initBuiltins() {
	builtinFuncs := map[string]builtinFunc{
		"select": selectFunc, "all": allAny(true), "any": allAny(false),
		"length": floatScalar(shaderir.Length, 1), "distance": floatScalar(shaderir.Distance, 2), "dot": floatScalar(shaderir.Dot, 2),
		"cross": crossFunc, "transpose": transposeFunc,
		"min": numeric(shaderir.Min, 2, false), "max": numeric(shaderir.Max, 2, false), "clamp": numeric(shaderir.Clamp, 3, false),
		"abs": numeric(shaderir.Abs, 1, false), "sign": numeric(shaderir.Sign, 1, true),
		"atan2": floats(shaderir.Atan2, 2), "pow": floats(shaderir.Pow, 2), "step": floats(shaderir.Step, 2),
		"reflect": vectors(shaderir.Reflect, 2), "smoothstep": floats(shaderir.Smoothstep, 3),
		"faceForward": vectors(shaderir.Faceforward, 3), "normalize": vectors(shaderir.Normalize, 1),
		"mix": mixFunc, "refract": refractFunc, "fma": fmaFunc, "saturate": saturateFunc,
		"degrees": scaled(57.29577951308232), "radians": scaled(0.017453292519943295),
		"ldexp": ldexpFunc,
		"asinh": lowered("asinh"), "acosh": lowered("acosh"), "atanh": lowered("atanh"),
	}
	for name, f := range map[string]shaderir.BuiltinFunc{
		"sin": shaderir.Sin, "cos": shaderir.Cos, "tan": shaderir.Tan, "asin": shaderir.Asin, "acos": shaderir.Acos,
		"atan": shaderir.Atan, "exp": shaderir.Exp, "exp2": shaderir.Exp2, "log": shaderir.Log, "log2": shaderir.Log2,
		"sqrt": shaderir.Sqrt, "inverseSqrt": shaderir.Inversesqrt, "fract": shaderir.Fract, "ceil": shaderir.Ceil,
		"floor": shaderir.Floor, "trunc": shaderir.Trunc, "round": shaderir.RoundEven,
		"sinh": shaderir.Sinh, "cosh": shaderir.Cosh, "tanh": shaderir.Tanh,
		"dpdx": shaderir.Dfdx, "dpdy": shaderir.Dfdy, "fwidth": shaderir.Fwidth,
		"dpdxCoarse": shaderir.Dfdx, "dpdyCoarse": shaderir.Dfdy, "fwidthCoarse": shaderir.Fwidth,
		"dpdxFine": shaderir.Dfdx, "dpdyFine": shaderir.Dfdy, "fwidthFine": shaderir.Fwidth,
	} {
		builtinFuncs[name] = floats(f, 1)
	}
	builtinTable.funcs = builtinFuncs
}

func arity(args []value, n int, pos int, name string) {
	if len(args) != n {
		fail(pos, "%s takes %d arguments, not %d", name, n, len(args))
	}
}

// common gives args one type, an abstract argument taking a concrete
// one's, and, for floats, an integer abstract one a float's.
func (c *checker) common(args []value, pos int, float bool) ty {
	var t ty
	found := false
	for _, a := range args {
		if !a.t.abstract() {
			t, found = a.t, true
			break
		}
	}
	if !found {
		t = args[0].t
		for _, a := range args {
			if a.t.scalar() == kAbsFloat {
				t = a.t
			}
		}
		if float {
			t = t.withScalar(kF32)
		}
		t = t.concrete()
	}
	for i := range args {
		args[i] = c.convert(args[i], t, pos)
	}
	return t
}

func callOf(c *checker, f shaderir.BuiltinFunc, t ty, args []value) value {
	var irArgs []shaderir.Expr
	for _, a := range args {
		irArgs = append(irArgs, c.ir(a))
	}
	return value{t: t, e: builtinCall(f, t, irArgs...)}
}

// floats is a function of n arguments of one float type, scalar or
// vector, returning that type.
func floats(f shaderir.BuiltinFunc, n int) builtinFunc {
	return func(c *checker, args []value, pos int, out *[]shaderir.Stmt) value {
		arity(args, n, pos, string(f))
		t := c.common(args, pos, true)
		if t.scalar() != kF32 || (t.k != kF32 && t.k != kVec) {
			fail(pos, "%s takes f32 or vectors of it, not a %s", f, t)
		}
		return callOf(c, f, t, args)
	}
}

// vectors is floats for float vectors only.
func vectors(f shaderir.BuiltinFunc, n int) builtinFunc {
	return func(c *checker, args []value, pos int, out *[]shaderir.Stmt) value {
		arity(args, n, pos, string(f))
		t := c.common(args, pos, true)
		if t.k != kVec || t.elem != kF32 {
			fail(pos, "%s takes vectors of f32, not a %s", f, t)
		}
		return callOf(c, f, t, args)
	}
}

// floatScalar is a function of float vectors returning an f32.
func floatScalar(f shaderir.BuiltinFunc, n int) builtinFunc {
	return func(c *checker, args []value, pos int, out *[]shaderir.Stmt) value {
		arity(args, n, pos, string(f))
		t := c.common(args, pos, true)
		if t.scalar() != kF32 || (t.k != kF32 && t.k != kVec) {
			fail(pos, "%s takes f32 or vectors of it, not a %s", f, t)
		}
		if t.k == kF32 && f == shaderir.Dot {
			fail(pos, "dot takes vectors")
		}
		return callOf(c, f, tF32, args)
	}
}

// numeric is a function of numbers of one type, floats or integers;
// signed refuses u32.
func numeric(f shaderir.BuiltinFunc, n int, signed bool) builtinFunc {
	return func(c *checker, args []value, pos int, out *[]shaderir.Stmt) value {
		arity(args, n, pos, string(f))
		t := c.common(args, pos, false)
		s := t.scalar()
		if !isNumeric(s) || (t.k != kVec && !t.isScalar()) || signed && s == kU32 {
			fail(pos, "%s takes numbers, not a %s", f, t)
		}
		if f == shaderir.Abs && s == kU32 {
			return args[0]
		}
		return callOf(c, f, t, args)
	}
}

func selectFunc(c *checker, args []value, pos int, out *[]shaderir.Stmt) value {
	arity(args, 3, pos, "select")
	cond := args[2]
	vals := args[:2]
	t := c.common(vals, pos, false)
	switch {
	case cond.t.k == kBool:
		if cond.c != nil {
			if b := cond.c.ExactString() == "true"; b {
				return vals[1]
			}
			return vals[0]
		}
		e := shaderir.Expr{Type: shaderir.Selection, Exprs: []shaderir.Expr{c.ir(cond), c.ir(vals[1]), c.ir(vals[0])}}
		return value{t: t, e: mk(e, t)}
	case cond.t.boolVector() && t.k == kVec && t.n == cond.t.n:
		f, tv := c.reuse(vals[0], out), c.reuse(vals[1], out)
		cv := c.reuse(cond, out)
		if t.elem == kF32 {
			return value{t: t, e: builtinCall(shaderir.Mix, t, c.ir(f), c.ir(tv), c.ir(cv))}
		}
		var comps []shaderir.Expr
		for i := range t.n {
			pick := mk(shaderir.Expr{Type: shaderir.Binary, Op: shaderir.GreaterThanOp, Exprs: []shaderir.Expr{component(c.ir(cv), i, cv.t), floatNumber(0.5)}}, tBool)
			comps = append(comps, mk(shaderir.Expr{Type: shaderir.Selection, Exprs: []shaderir.Expr{pick, component(c.ir(tv), i, t), component(c.ir(f), i, t)}}, ty{k: t.elem}))
		}
		return value{t: t, e: builtinCall(vecFunc(t), t, comps...)}
	}
	fail(pos, "select takes a bool, or a vector of bools as wide as its values, not a %s", cond.t)
	return value{}
}

func allAny(all bool) builtinFunc {
	return func(c *checker, args []value, pos int, out *[]shaderir.Stmt) value {
		arity(args, 1, pos, "all")
		v := args[0]
		if v.t.k == kBool {
			return v
		}
		if !v.t.boolVector() {
			fail(pos, "all and any take bools, not a %s", v.t)
		}
		// The components are 0 and 1: all are 1 where their sum is n, any
		// where it is above 0.
		ones := builtinCall(vecFunc(vec(kF32, v.t.n)), vec(kF32, v.t.n), floatNumber(1))
		sum := builtinCall(shaderir.Dot, tF32, c.ir(v), ones)
		bound := 0.5
		if all {
			bound = float64(v.t.n) - 0.5
		}
		return value{t: tBool, e: shaderir.Expr{Type: shaderir.Binary, Op: shaderir.GreaterThanOp, Exprs: []shaderir.Expr{sum, floatNumber(bound)}}}
	}
}

func crossFunc(c *checker, args []value, pos int, out *[]shaderir.Stmt) value {
	arity(args, 2, pos, "cross")
	t := c.common(args, pos, true)
	if !t.equal(vec(kF32, 3)) {
		fail(pos, "cross takes vec3f, not a %s", t)
	}
	return callOf(c, shaderir.Cross, t, args)
}

func transposeFunc(c *checker, args []value, pos int, out *[]shaderir.Stmt) value {
	arity(args, 1, pos, "transpose")
	if args[0].t.k != kMat {
		fail(pos, "transpose takes a matrix, not a %s", args[0].t)
	}
	return callOf(c, shaderir.Transpose, args[0].t, args)
}

// splat widens a scalar to t, a vector, as the languages that need one
// type of their arguments take it.
func (c *checker) splat(v value, t ty, pos int) value {
	if t.k != kVec || v.t.k == kVec {
		return v
	}
	v = c.convert(v, ty{k: t.elem}, pos)
	return value{t: t, e: builtinCall(vecFunc(t), t, c.ir(v))}
}

func mixFunc(c *checker, args []value, pos int, out *[]shaderir.Stmt) value {
	arity(args, 3, pos, "mix")
	t := c.common(args[:2], pos, true)
	if t.scalar() != kF32 || t.k != kF32 && t.k != kVec {
		fail(pos, "mix takes f32 or vectors of it, not a %s", t)
	}
	f := args[2]
	if f.t.k == kVec || t.k == kF32 {
		f = c.convert(f, t, pos)
	} else {
		f = c.splat(c.convert(f, tF32, pos), t, pos)
	}
	return callOf(c, shaderir.Mix, t, []value{args[0], args[1], f})
}

func refractFunc(c *checker, args []value, pos int, out *[]shaderir.Stmt) value {
	arity(args, 3, pos, "refract")
	t := c.common(args[:2], pos, true)
	if t.k != kVec || t.elem != kF32 {
		fail(pos, "refract takes vectors of f32, not a %s", t)
	}
	return callOf(c, shaderir.Refract, t, []value{args[0], args[1], c.convert(args[2], tF32, pos)})
}

func fmaFunc(c *checker, args []value, pos int, out *[]shaderir.Stmt) value {
	arity(args, 3, pos, "fma")
	t := c.common(args, pos, true)
	if t.scalar() != kF32 || t.k != kF32 && t.k != kVec {
		fail(pos, "fma takes f32 or vectors of it, not a %s", t)
	}
	m := c.arith("*", args[0], args[1], pos, out)
	return c.arith("+", m, args[2], pos, out)
}

func saturateFunc(c *checker, args []value, pos int, out *[]shaderir.Stmt) value {
	arity(args, 1, pos, "saturate")
	t := c.common(args, pos, true)
	if t.scalar() != kF32 || t.k != kF32 && t.k != kVec {
		fail(pos, "saturate takes f32 or vectors of it, not a %s", t)
	}
	lo := c.splat(value{t: tF32, e: floatNumber(0)}, t, pos)
	hi := c.splat(value{t: tF32, e: floatNumber(1)}, t, pos)
	return callOf(c, shaderir.Clamp, t, []value{args[0], lo, hi})
}

// scaled multiplies its float argument by k, as degrees and radians do.
func scaled(k float64) builtinFunc {
	return func(c *checker, args []value, pos int, out *[]shaderir.Stmt) value {
		arity(args, 1, pos, "degrees")
		t := c.common(args, pos, true)
		if t.scalar() != kF32 || t.k != kF32 && t.k != kVec {
			fail(pos, "degrees and radians take f32 or vectors of it, not a %s", t)
		}
		return value{t: t, e: mk(shaderir.Expr{Type: shaderir.Binary, Op: shaderir.ComponentWiseMul, Exprs: []shaderir.Expr{c.ir(args[0]), floatNumber(k)}}, t)}
	}
}

func ldexpFunc(c *checker, args []value, pos int, out *[]shaderir.Stmt) value {
	arity(args, 2, pos, "ldexp")
	x := c.concrete(args[0], pos)
	if x.t.abstract() || x.t.scalar() != kAbsFloat && x.t.scalar() != kF32 {
		x = c.convert(args[0], args[0].t.withScalar(kF32), pos)
	}
	e := c.concrete(args[1], pos)
	if e.t.scalar() != kI32 {
		fail(pos, "ldexp's exponent is an i32, not a %s", e.t)
	}
	ft := e.t.withScalar(kF32)
	var fe shaderir.Expr
	if ft.k == kVec {
		fe = builtinCall(vecFunc(ft), ft, c.ir(e))
	} else {
		fe = builtinCall(shaderir.FloatF, ft, c.ir(e))
	}
	p := builtinCall(shaderir.Exp2, ft, fe)
	return value{t: x.t, e: mk(shaderir.Expr{Type: shaderir.Binary, Op: shaderir.ComponentWiseMul, Exprs: []shaderir.Expr{c.ir(x), p}}, x.t)}
}

// lowered writes the inverse hyperbolic functions with logarithms.
func lowered(name string) builtinFunc {
	return func(c *checker, args []value, pos int, out *[]shaderir.Stmt) value {
		arity(args, 1, pos, name)
		t := c.common(args, pos, true)
		if t.scalar() != kF32 || t.k != kF32 && t.k != kVec {
			fail(pos, "%s takes f32 or vectors of it, not a %s", name, t)
		}
		x := c.reuse(args[0], out)
		one := c.splat(value{t: tF32, e: floatNumber(1)}, t, pos)
		bin := func(op shaderir.Op, a, b shaderir.Expr) shaderir.Expr {
			return mk(shaderir.Expr{Type: shaderir.Binary, Op: op, Exprs: []shaderir.Expr{a, b}}, t)
		}
		xi := c.ir(x)
		switch name {
		case "asinh", "acosh":
			op := shaderir.Add
			if name == "acosh" {
				op = shaderir.Sub
			}
			r := builtinCall(shaderir.Sqrt, t, bin(op, bin(shaderir.ComponentWiseMul, xi, xi), c.ir(one)))
			return value{t: t, e: builtinCall(shaderir.Log, t, bin(shaderir.Add, xi, r))}
		}
		q := bin(shaderir.Div, bin(shaderir.Add, c.ir(one), xi), bin(shaderir.Sub, c.ir(one), xi))
		return value{t: t, e: bin(shaderir.ComponentWiseMul, builtinCall(shaderir.Log, t, q), floatNumber(0.5))}
	}
}
