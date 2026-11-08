package wgsl

import (
	"go/constant"
	"go/token"
	"math"
	"strings"

	"github.com/oneo-me/nsgo/internal/shaderir"
)

// value is an expression checked: its type, and a constant (c, a scalar;
// comps, an abstract vector's components) or its IR.
type value struct {
	t     ty
	c     constant.Value
	comps []constant.Value
	e     shaderir.Expr
	// ref is a variable's, or a part of it, which mutable ones assign.
	ref, mutable bool
	// fb is a bool held as a float of 0 or 1: a part of a vec<bool>.
	fb bool
}

func number(c constant.Value, k kind) shaderir.Expr {
	switch k {
	case kBool:
		return shaderir.Expr{Type: shaderir.NumberExpr, Const: c}
	case kF32, kAbsFloat:
		return shaderir.Expr{Type: shaderir.NumberExpr, Const: constant.ToFloat(c)}
	}
	return shaderir.Expr{Type: shaderir.NumberExpr, Const: constant.ToInt(c), Unsigned: k == kU32}
}

func floatNumber(x float64) shaderir.Expr {
	return shaderir.Expr{Type: shaderir.NumberExpr, Const: constant.MakeFloat64(x)}
}

// vecFunc is the IR's constructor of vectors of t's shape.
func vecFunc(t ty) shaderir.BuiltinFunc {
	n := t.n
	switch t.concrete().elem {
	case kI32:
		return [...]shaderir.BuiltinFunc{2: shaderir.IVec2F, 3: shaderir.IVec3F, 4: shaderir.IVec4F}[n]
	case kU32:
		return [...]shaderir.BuiltinFunc{2: shaderir.UVec2F, 3: shaderir.UVec3F, 4: shaderir.UVec4F}[n]
	}
	return [...]shaderir.BuiltinFunc{2: shaderir.Vec2F, 3: shaderir.Vec3F, 4: shaderir.Vec4F}[n]
}

func builtinCall(f shaderir.BuiltinFunc, t ty, args ...shaderir.Expr) shaderir.Expr {
	return mk(shaderir.Expr{Type: shaderir.Call, Exprs: append([]shaderir.Expr{{Type: shaderir.BuiltinFuncExpr, BuiltinFunc: f}}, args...)}, t)
}

// ir is v's IR: a constant written as a literal of its type.
func (c *checker) ir(v value) shaderir.Expr {
	switch {
	case v.c != nil:
		return number(v.c, v.t.concrete().k)
	case v.comps != nil:
		t := v.t.concrete()
		var args []shaderir.Expr
		for _, x := range v.comps {
			args = append(args, number(x, t.elem))
		}
		return builtinCall(vecFunc(t), t, args...)
	case v.fb:
		return mk(shaderir.Expr{Type: shaderir.Binary, Op: shaderir.GreaterThanOp, Exprs: []shaderir.Expr{v.e, floatNumber(0.5)}}, tBool)
	}
	return v.e
}

// reusable reports whether v may be written twice without a local: a
// constant, a variable, or a part of one at a constant index.
func reusable(v value) bool {
	if v.c != nil || v.comps != nil {
		return true
	}
	var simple func(e *shaderir.Expr) bool
	simple = func(e *shaderir.Expr) bool {
		switch e.Type {
		case shaderir.NumberExpr, shaderir.LocalVariable, shaderir.UniformVariable:
			return true
		case shaderir.FieldSelector:
			return simple(&e.Exprs[0])
		case shaderir.Index:
			return simple(&e.Exprs[0]) && e.Exprs[1].Type == shaderir.NumberExpr
		}
		return false
	}
	return simple(&v.e)
}

// reuse returns v as a value to write more than once, in a local unless
// it is reusable.
func (c *checker) reuse(v value, out *[]shaderir.Stmt) value {
	if reusable(v) {
		return v
	}
	return c.temp(v, out)
}

func (c *checker) temp(v value, out *[]shaderir.Stmt) value {
	if c.cur == nil {
		panic(bail{errorAt(0, "the expression is not a constant")})
	}
	t := v.t.concrete()
	idx := c.newLocal(t)
	*out = append(*out, assign(local(idx, t), c.ir(v)))
	return value{t: t, e: local(idx, t)}
}

// convertConst converts the constant x of kind from to to's kind, as an
// abstract value converts, or fails.
func (c *checker) convertConst(x constant.Value, from, to ty, pos int) constant.Value {
	switch to.k {
	case kBool:
		if from.k == kBool {
			return x
		}
	case kF32, kAbsFloat:
		if isNumeric(from.k) && (from.k == to.k || from.k == kAbsInt || from.k == kAbsFloat) {
			f := constant.ToFloat(x)
			if v, _ := constant.Float64Val(f); to.k == kF32 && math.Abs(v) > math.MaxFloat32 {
				fail(pos, "the value %s does not fit an f32", x)
			}
			return f
		}
	case kI32, kU32, kAbsInt:
		if from.k == to.k || from.k == kAbsInt || from.k == kAbsFloat && to.k == kAbsInt {
			i := constant.ToInt(x)
			if i.Kind() != constant.Int {
				fail(pos, "the value %s is not an integer", x)
			}
			lo, hi := int64(math.MinInt32), int64(math.MaxInt32)
			if to.k == kU32 {
				lo, hi = 0, math.MaxUint32
			}
			if to.k != kAbsInt {
				if v, ok := constant.Int64Val(i); !ok || v < lo || v > hi {
					fail(pos, "the value %s does not fit a %s", x, to)
				}
			}
			return i
		}
	}
	fail(pos, "cannot use a %s as a %s", from, to)
	return nil
}

// convert converts v to t where WGSL converts it on its own: an abstract
// value takes t's type.
func (c *checker) convert(v value, t ty, pos int) value {
	if v.t.equal(t) {
		return v
	}
	switch {
	case v.c != nil && t.isScalar():
		return value{t: t, c: c.convertConst(v.c, v.t, t, pos)}
	case v.comps != nil && t.k == kVec && t.n == v.t.n:
		comps := make([]constant.Value, len(v.comps))
		for i, x := range v.comps {
			comps[i] = c.convertConst(x, ty{k: v.t.elem}, ty{k: t.elem}, pos)
		}
		if t.abstract() {
			return value{t: t, comps: comps}
		}
		var args []shaderir.Expr
		for _, x := range comps {
			args = append(args, number(x, t.elem))
		}
		if t.elem == kBool {
			fail(pos, "cannot use a %s as a %s", v.t, t)
		}
		return value{t: t, e: builtinCall(vecFunc(t), t, args...)}
	}
	fail(pos, "cannot use a %s as a %s", v.t, t)
	return value{}
}

// concrete gives an abstract v the type WGSL gives it where nothing
// else does.
func (c *checker) concrete(v value, pos int) value {
	if !v.t.abstract() {
		return v
	}
	return c.convert(v, v.t.concrete(), pos)
}

func (c *checker) expr(e expr, out *[]shaderir.Stmt) value {
	switch x := e.(type) {
	case *litExpr:
		return literal(x.tok)
	case *identExpr:
		return c.ident(x, out)
	case *unaryExpr:
		return c.unary(x, out)
	case *binaryExpr:
		return c.binary(x, out)
	case *callExpr:
		v := c.call(x, out, false)
		if v.t.k == kNone {
			fail(x.at(), "%s returns no value", x.fn.name)
		}
		return v
	case *memberExpr:
		return c.member(x, out)
	case *indexExpr:
		return c.index(x, out)
	}
	fail(e.at(), "the expression is not supported")
	return value{}
}

func literal(t lexeme) value {
	switch {
	case t.text == "true" || t.text == "false":
		return value{t: tBool, c: constant.MakeBool(t.text == "true")}
	case t.kind == tInt:
		x := constant.MakeFromLiteral(t.text, token.INT, 0)
		switch t.suffix {
		case "i":
			return checkedInt(x, tI32, t.pos)
		case "u":
			return checkedInt(x, tU32, t.pos)
		}
		return value{t: tAbsInt, c: x}
	}
	if t.suffix == "h" {
		fail(t.pos, "f16 is not supported")
	}
	x := constant.MakeFromLiteral(normalFloat(t.text), token.FLOAT, 0)
	if x.Kind() == constant.Unknown {
		fail(t.pos, "the number %s is not valid", t.text)
	}
	if t.suffix == "f" {
		return value{t: tF32, c: constant.ToFloat(x)}
	}
	return value{t: tAbsFloat, c: constant.ToFloat(x)}
}

// normalFloat writes a WGSL float as Go reads it: a hexadecimal float
// needs an exponent there.
func normalFloat(s string) string {
	if (strings.HasPrefix(s, "0x") || strings.HasPrefix(s, "0X")) && !strings.ContainsAny(s, "pP") {
		return s + "p0"
	}
	return s
}

func checkedInt(x constant.Value, t ty, pos int) value {
	lo, hi := int64(math.MinInt32), int64(math.MaxInt32)
	if t.k == kU32 {
		lo, hi = 0, math.MaxUint32
	}
	if v, ok := constant.Int64Val(x); !ok || v < lo || v > hi {
		fail(pos, "the value %s does not fit a %s", x, t)
	}
	return value{t: t, c: x}
}

func (c *checker) ident(x *identExpr, out *[]shaderir.Stmt) value {
	sym := c.scope.lookup(x.name)
	if sym == nil {
		switch x.name {
		case "uniforms":
			fail(x.pos, "uniforms needs struct Uniforms { ... } declared")
		}
		fail(x.pos, "%s is not declared", x.name)
	}
	switch sym.kind {
	case symLocal:
		return value{t: sym.t, e: local(sym.local, sym.t), ref: true, mutable: sym.mutable}
	case symConst:
		if sym.val != nil {
			return *sym.val
		}
		saved := c.scope
		c.scope = c.global
		v := c.expr(sym.init, out)
		c.scope = saved
		if sym.t.k != kNone {
			v = c.convert(v, sym.t, x.pos)
		}
		v.ref = false
		return v
	case symFn:
		fail(x.pos, "the function %s is not called", x.name)
	case symType:
		fail(x.pos, "the type %s is not called", x.name)
	case symBuiltin:
		fail(x.pos, "%s is read by its members", x.name)
	}
	return value{}
}

func (c *checker) unary(x *unaryExpr, out *[]shaderir.Stmt) value {
	v := c.expr(x.x, out)
	t := v.t
	switch x.op {
	case "-":
		s := t.scalar()
		if !isNumeric(s) || s == kU32 || t.k == kArray {
			fail(x.pos, "- takes a signed number, not a %s", t)
		}
		if v.c != nil {
			return value{t: t, c: c.foldUnary(token.SUB, v.c, t, x.pos)}
		}
		if v.comps != nil {
			comps := make([]constant.Value, len(v.comps))
			for i, y := range v.comps {
				comps[i] = constant.UnaryOp(token.SUB, y, 0)
			}
			return value{t: t, comps: comps}
		}
		return value{t: t, e: mk(shaderir.Expr{Type: shaderir.Unary, Op: shaderir.Sub, Exprs: []shaderir.Expr{c.ir(v)}}, t)}
	case "!":
		switch {
		case t.k == kBool:
			if v.c != nil {
				return value{t: t, c: constant.MakeBool(!constant.BoolVal(v.c))}
			}
			return value{t: t, e: not(c.ir(v))}
		case t.boolVector():
			return value{t: t, e: mk(shaderir.Expr{Type: shaderir.Binary, Op: shaderir.Sub, Exprs: []shaderir.Expr{floatNumber(1), c.ir(v)}}, t)}
		}
		fail(x.pos, "! takes a bool, not a %s", t)
	case "~":
		if !isInt(t.scalar()) || t.k == kArray {
			fail(x.pos, "~ takes an integer, not a %s", t)
		}
		if v.c != nil {
			var prec uint
			if t.k == kU32 {
				prec = 32
			}
			r := constant.UnaryOp(token.XOR, v.c, prec)
			if t.k == kI32 {
				r = constant.UnaryOp(token.XOR, v.c, 0)
			}
			return value{t: t, c: r}
		}
		v = c.concrete(v, x.pos)
		return value{t: v.t, e: mk(shaderir.Expr{Type: shaderir.Unary, Op: shaderir.ComplementOp, Exprs: []shaderir.Expr{c.ir(v)}}, v.t)}
	}
	fail(x.pos, "the operator %s is not supported", x.op)
	return value{}
}

func (c *checker) foldUnary(op token.Token, x constant.Value, t ty, pos int) constant.Value {
	r := constant.UnaryOp(op, x, 0)
	if !t.abstract() {
		return c.convertConst(r, t, t, pos)
	}
	return r
}

func (c *checker) binary(x *binaryExpr, out *[]shaderir.Stmt) value {
	a := c.expr(x.x, out)
	switch x.op {
	case "&&", "||":
		if a.t.k != kBool {
			fail(x.pos, "%s takes bools, not a %s", x.op, a.t)
		}
		// The right side's statements run only where it is read.
		var pre []shaderir.Stmt
		b := c.expr(x.y, &pre)
		if b.t.k != kBool {
			fail(x.y.at(), "%s takes bools, not a %s", x.op, b.t)
		}
		if a.c != nil && b.c != nil {
			if x.op == "&&" {
				return value{t: tBool, c: constant.MakeBool(constant.BoolVal(a.c) && constant.BoolVal(b.c))}
			}
			return value{t: tBool, c: constant.MakeBool(constant.BoolVal(a.c) || constant.BoolVal(b.c))}
		}
		*out = append(*out, pre...) // pure: only locals the right side reads
		op := shaderir.AndAnd
		if x.op == "||" {
			op = shaderir.OrOr
		}
		return value{t: tBool, e: shaderir.Expr{Type: shaderir.Binary, Op: op, Exprs: []shaderir.Expr{c.ir(a), c.ir(b)}}}
	}
	b := c.expr(x.y, out)
	switch x.op {
	case "&", "|", "^", "<<", ">>":
		return c.bitwise(x.op, a, b, x.pos, out)
	case "==", "!=", "<", "<=", ">", ">=":
		return c.compare(x.op, a, b, x.pos, out)
	}
	return c.arith(x.op, a, b, x.pos, out)
}

// unify gives a and b one scalar kind where they differ only by being
// abstract, converting the abstract side.
func (c *checker) unify(a, b value, pos int) (value, value) {
	sa, sb := a.t.scalar(), b.t.scalar()
	if sa == sb {
		return a, b
	}
	abs := func(k kind) bool { return k == kAbsInt || k == kAbsFloat }
	switch {
	case abs(sa) && abs(sb):
		// An abstract integer with an abstract float is a float.
		if sa == kAbsInt {
			a = c.convert(a, a.t.withScalar(kAbsFloat), pos)
		} else {
			b = c.convert(b, b.t.withScalar(kAbsFloat), pos)
		}
	case abs(sa):
		a = c.convert(a, a.t.withScalar(sb), pos)
	case abs(sb):
		b = c.convert(b, b.t.withScalar(sa), pos)
	default:
		fail(pos, "the operands are a %s and a %s", a.t, b.t)
	}
	return a, b
}

var arithOps = map[string]struct {
	tok token.Token
	op  shaderir.Op
}{
	"+": {token.ADD, shaderir.Add}, "-": {token.SUB, shaderir.Sub}, "*": {token.MUL, shaderir.ComponentWiseMul},
	"/": {token.QUO, shaderir.Div}, "%": {token.REM, shaderir.ModOp},
}

func (c *checker) arith(op string, a, b value, pos int, out *[]shaderir.Stmt) value {
	o, ok := arithOps[op]
	if !ok {
		fail(pos, "the operator %s is not supported", op)
	}
	a, b = c.unify(a, b, pos)
	s := a.t.scalar()
	if !isNumeric(s) || a.t.k == kArray || b.t.k == kArray {
		fail(pos, "%s takes numbers, not a %s and a %s", op, a.t, b.t)
	}
	if a.c != nil && b.c != nil {
		return value{t: a.t, c: c.fold(o.tok, a.c, b.c, a.t, pos)}
	}
	if a.comps != nil || b.comps != nil {
		a, b = c.concrete(a, pos), c.concrete(b, pos)
	}
	if a.c != nil && b.c == nil || b.c != nil && a.c == nil {
		a, b = c.concrete(a, pos), c.concrete(b, pos)
		// A constant takes the other operand's type.
	}
	a, b = c.concrete(a, pos), c.concrete(b, pos)
	var t ty
	irop := o.op
	switch {
	case a.t.k == kMat || b.t.k == kMat:
		t = c.matrixResult(op, a.t, b.t, pos)
		if op == "*" {
			irop = shaderir.MatrixMul
		}
	case a.t.equal(b.t):
		t = a.t
	case a.t.k == kVec && b.t.isScalar() && a.t.elem == b.t.k:
		t = a.t
	case b.t.k == kVec && a.t.isScalar() && b.t.elem == a.t.k:
		t = b.t
	default:
		fail(pos, "%s takes operands of one type, not a %s and a %s", op, a.t, b.t)
	}
	if op == "%" && isFloat(s) {
		// WGSL's remainder truncates: x - y * trunc(x / y).
		a, b = c.reuse(a, out), c.reuse(b, out)
		q := mk(shaderir.Expr{Type: shaderir.Binary, Op: shaderir.Div, Exprs: []shaderir.Expr{c.ir(a), c.ir(b)}}, t)
		tq := builtinCall(shaderir.Trunc, t, q)
		m := mk(shaderir.Expr{Type: shaderir.Binary, Op: shaderir.ComponentWiseMul, Exprs: []shaderir.Expr{c.ir(b), tq}}, t)
		return value{t: t, e: mk(shaderir.Expr{Type: shaderir.Binary, Op: shaderir.Sub, Exprs: []shaderir.Expr{c.ir(a), m}}, t)}
	}
	return value{t: t, e: mk(shaderir.Expr{Type: shaderir.Binary, Op: irop, Exprs: []shaderir.Expr{c.ir(a), c.ir(b)}}, t)}
}

func (c *checker) matrixResult(op string, a, b ty, pos int) ty {
	switch {
	case a.k == kMat && b.k == kMat && a.equal(b) && (op == "+" || op == "-" || op == "*"):
		return a
	case op == "*" && a.k == kMat && b.k == kVec && b.elem == kF32 && b.n == a.n:
		return b
	case op == "*" && b.k == kMat && a.k == kVec && a.elem == kF32 && a.n == b.rows:
		return a
	case op == "*" && a.k == kMat && b.k == kF32:
		return a
	case op == "*" && b.k == kMat && a.k == kF32:
		return b
	}
	fail(pos, "%s is not defined on a %s and a %s", op, a, b)
	return ty{}
}

// fold computes a constant operation, which fails where a concrete type
// overflows, as WGSL's constant expressions do.
func (c *checker) fold(op token.Token, x, y constant.Value, t ty, pos int) constant.Value {
	if !t.isScalar() {
		fail(pos, "the operation is not a constant")
	}
	if isInt(t.k) {
		if op == token.QUO || op == token.REM {
			if constant.Sign(y) == 0 {
				fail(pos, "division by zero")
			}
		}
		if op == token.QUO {
			op = token.QUO_ASSIGN // integer division, truncated
		}
		r := constant.BinaryOp(x, op, y)
		if t.k != kAbsInt {
			return c.convertConst(r, t, t, pos)
		}
		return r
	}
	if op == token.REM {
		fx, _ := constant.Float64Val(x)
		fy, _ := constant.Float64Val(y)
		if fy == 0 {
			fail(pos, "division by zero")
		}
		return constant.MakeFloat64(fx - fy*math.Trunc(fx/fy))
	}
	if op == token.QUO && constant.Sign(y) == 0 {
		fail(pos, "division by zero")
	}
	r := constant.ToFloat(constant.BinaryOp(x, op, y))
	if t.k == kF32 {
		return c.convertConst(r, t, t, pos)
	}
	return r
}

func (c *checker) bitwise(op string, a, b value, pos int, out *[]shaderir.Stmt) value {
	if op == "<<" || op == ">>" {
		if !isInt(a.t.scalar()) || a.t.k == kArray {
			fail(pos, "%s takes an integer, not a %s", op, a.t)
		}
		if b.t.scalar() != kU32 && b.t.scalar() != kAbsInt || b.t.k == kArray {
			fail(pos, "%s shifts by a u32, not a %s", op, b.t)
		}
		if b.t.k == kAbsInt {
			b = c.convert(b, tU32, pos)
		}
		if a.c != nil && b.c != nil {
			n, _ := constant.Uint64Val(b.c)
			tok := token.SHL
			if op == ">>" {
				tok = token.SHR
			}
			r := constant.Shift(a.c, tok, uint(n))
			if a.t.k == kU32 && tok == token.SHL {
				r = constant.BinaryOp(r, token.AND, constant.MakeUint64(math.MaxUint32))
			}
			if a.t.k == kI32 && tok == token.SHL {
				u, _ := constant.Int64Val(r)
				r = constant.MakeInt64(int64(int32(u)))
			}
			return value{t: a.t, c: r}
		}
		a = c.concrete(a, pos)
		irop := shaderir.LeftShift
		if op == ">>" {
			irop = shaderir.RightShift
		}
		return value{t: a.t, e: mk(shaderir.Expr{Type: shaderir.Binary, Op: irop, Exprs: []shaderir.Expr{c.ir(a), c.ir(b)}}, a.t)}
	}
	a, b = c.unify(a, b, pos)
	if !a.t.equal(b.t) {
		fail(pos, "%s takes operands of one type, not a %s and a %s", op, a.t, b.t)
	}
	t := a.t
	switch {
	case t.k == kBool:
		if a.c != nil && b.c != nil {
			x, y := constant.BoolVal(a.c), constant.BoolVal(b.c)
			r := map[string]bool{"&": x && y, "|": x || y, "^": x != y}[op]
			return value{t: tBool, c: constant.MakeBool(r)}
		}
		irop := map[string]shaderir.Op{"&": shaderir.AndAnd, "|": shaderir.OrOr, "^": shaderir.NotEqualOp}[op]
		return value{t: tBool, e: shaderir.Expr{Type: shaderir.Binary, Op: irop, Exprs: []shaderir.Expr{c.ir(a), c.ir(b)}}}
	case t.boolVector():
		x, y := c.ir(a), c.ir(b)
		switch op {
		case "&":
			return value{t: t, e: mk(shaderir.Expr{Type: shaderir.Binary, Op: shaderir.ComponentWiseMul, Exprs: []shaderir.Expr{x, y}}, t)}
		case "|":
			return value{t: t, e: builtinCall(shaderir.Max, t, x, y)}
		}
		d := mk(shaderir.Expr{Type: shaderir.Binary, Op: shaderir.Sub, Exprs: []shaderir.Expr{x, y}}, t)
		return value{t: t, e: builtinCall(shaderir.Abs, t, d)}
	case isInt(t.scalar()) && t.k != kArray:
		tok := map[string]token.Token{"&": token.AND, "|": token.OR, "^": token.XOR}[op]
		if a.c != nil && b.c != nil {
			return value{t: t, c: constant.BinaryOp(a.c, tok, b.c)}
		}
		a, b = c.concrete(a, pos), c.concrete(b, pos)
		irop := map[string]shaderir.Op{"&": shaderir.And, "|": shaderir.Or, "^": shaderir.Xor}[op]
		return value{t: a.t, e: mk(shaderir.Expr{Type: shaderir.Binary, Op: irop, Exprs: []shaderir.Expr{c.ir(a), c.ir(b)}}, a.t)}
	}
	fail(pos, "%s takes integers or bools, not a %s", op, t)
	return value{}
}

var compareOps = map[string]struct {
	tok token.Token
	op  shaderir.Op
}{
	"==": {token.EQL, shaderir.EqualOp}, "!=": {token.NEQ, shaderir.NotEqualOp}, "<": {token.LSS, shaderir.LessThanOp},
	"<=": {token.LEQ, shaderir.LessThanEqualOp}, ">": {token.GTR, shaderir.GreaterThanOp}, ">=": {token.GEQ, shaderir.GreaterThanEqualOp},
}

func (c *checker) compare(op string, a, b value, pos int, out *[]shaderir.Stmt) value {
	o := compareOps[op]
	a, b = c.unify(a, b, pos)
	if !a.t.equal(b.t) || a.t.k == kArray || a.t.k == kMat {
		fail(pos, "%s compares values of one type, not a %s and a %s", op, a.t, b.t)
	}
	if (a.t.k == kBool || a.t.boolVector()) && op != "==" && op != "!=" {
		fail(pos, "%s does not compare bools", op)
	}
	if a.t.isScalar() {
		if a.c != nil && b.c != nil {
			return value{t: tBool, c: constant.MakeBool(constant.Compare(a.c, o.tok, b.c))}
		}
		a, b = c.concrete(a, pos), c.concrete(b, pos)
		return value{t: tBool, e: shaderir.Expr{Type: shaderir.Binary, Op: o.op, Exprs: []shaderir.Expr{c.ir(a), c.ir(b)}}}
	}
	// Vectors compare component by component into a vec<bool>.
	a, b = c.reuse(c.concrete(a, pos), out), c.reuse(c.concrete(b, pos), out)
	t := vec(kBool, a.t.n)
	var args []shaderir.Expr
	for i := range a.t.n {
		x, y := component(c.ir(a), i, a.t), component(c.ir(b), i, b.t)
		cmp := shaderir.Expr{Type: shaderir.Binary, Op: o.op, Exprs: []shaderir.Expr{x, y}}
		args = append(args, selectNumber(cmp, 1, 0))
	}
	return value{t: t, e: builtinCall(vecFunc(t), t, args...)}
}

// component is e's component i: a vector's, by swizzle.
func component(e shaderir.Expr, i int, t ty) shaderir.Expr {
	return mk(shaderir.Expr{Type: shaderir.FieldSelector, Exprs: []shaderir.Expr{e, {Type: shaderir.SwizzlingExpr, Swizzling: "xyzw"[i : i+1]}}}, ty{k: t.elem})
}

// selectNumber is a float of x where cond holds, else of y.
func selectNumber(cond shaderir.Expr, x, y float64) shaderir.Expr {
	return shaderir.Expr{Type: shaderir.Selection, Exprs: []shaderir.Expr{cond, floatNumber(x), floatNumber(y)}}
}

var swizzleSets = []string{"xyzw", "rgba"}

func (c *checker) member(x *memberExpr, out *[]shaderir.Stmt) value {
	if id, ok := x.x.(*identExpr); ok {
		if sym := c.scope.lookup(id.name); sym != nil && sym.kind == symBuiltin {
			return c.builtinMember(id.name, x)
		}
	}
	v := c.expr(x.x, out)
	if v.t.k != kVec {
		fail(x.pos, "a %s has no member %s", v.t, x.name)
	}
	var sw strings.Builder
	for _, set := range swizzleSets {
		sw.Reset()
		for _, r := range x.name {
			i := strings.IndexRune(set, r)
			if i < 0 || i >= v.t.n {
				break
			}
			sw.WriteByte("xyzw"[i])
		}
		if sw.Len() == len(x.name) {
			break
		}
	}
	if sw.Len() != len(x.name) || len(x.name) > 4 {
		fail(x.pos, "a %s has no member %s", v.t, x.name)
	}
	s := sw.String()
	if v.comps != nil {
		var comps []constant.Value
		for _, r := range s {
			comps = append(comps, v.comps[strings.IndexRune("xyzw", r)])
		}
		if len(comps) == 1 {
			return value{t: ty{k: v.t.elem}, c: comps[0]}
		}
		return value{t: vec(v.t.elem, len(comps)), comps: comps}
	}
	t := ty{k: v.t.elem}
	if len(s) > 1 {
		t = vec(v.t.elem, len(s))
	}
	e := mk(shaderir.Expr{Type: shaderir.FieldSelector, Exprs: []shaderir.Expr{c.ir(v), {Type: shaderir.SwizzlingExpr, Swizzling: s}}}, t)
	r := value{t: t, e: e}
	if len(s) == 1 {
		// Only a single component is assigned to.
		r.ref, r.mutable = v.ref, v.mutable
		r.fb = v.t.boolVector()
	}
	return r
}

// builtinMember reads nsh's members and the uniforms. The view's box is
// the surface's size less the overflow on each side.
func (c *checker) builtinMember(name string, x *memberExpr) value {
	uniform := func(i int, t ty) shaderir.Expr {
		return mk(shaderir.Expr{Type: shaderir.UniformVariable, Index: i}, t)
	}
	nshPart := func(sw string) shaderir.Expr {
		return mk(shaderir.Expr{Type: shaderir.FieldSelector, Exprs: []shaderir.Expr{uniform(c.nsh, vec(kF32, 3)), {Type: shaderir.SwizzlingExpr, Swizzling: sw}}}, tF32)
	}
	if name == "uniforms" {
		u, ok := c.uniforms[x.name]
		if !ok {
			fail(x.pos, "struct Uniforms has no member %s", x.name)
		}
		return value{t: u.t, e: uniform(u.index, u.t)}
	}
	switch x.name {
	case "size":
		v2 := vec(kF32, 2)
		twice := mk(shaderir.Expr{Type: shaderir.Binary, Op: shaderir.ComponentWiseMul, Exprs: []shaderir.Expr{floatNumber(2), nshPart("z")}}, tF32)
		return value{t: v2, e: mk(shaderir.Expr{Type: shaderir.Binary, Op: shaderir.Sub, Exprs: []shaderir.Expr{uniform(0, v2), twice}}, v2)}
	case "time":
		if c.cur != nil {
			c.cur.info.readsTime = true
		}
		return value{t: tF32, e: nshPart("x")}
	case "scale":
		return value{t: tF32, e: uniform(1, tF32)}
	case "progress":
		return value{t: tF32, e: nshPart("y")}
	case "overflow":
		return value{t: tF32, e: nshPart("z")}
	}
	fail(x.pos, "nsh has no member %s: it has size, time, scale, progress and overflow", x.name)
	return value{}
}

func (c *checker) index(x *indexExpr, out *[]shaderir.Stmt) value {
	v := c.expr(x.x, out)
	i := c.expr(x.i, out)
	if !i.t.isScalar() || !isInt(i.t.k) {
		fail(x.i.at(), "an index is an integer, not a %s", i.t)
	}
	var n int
	var t ty
	switch v.t.k {
	case kVec:
		n, t = v.t.n, ty{k: v.t.elem}
	case kMat:
		n, t = v.t.n, vec(kF32, v.t.rows)
	case kArray:
		n, t = v.t.n, *v.t.arr
	default:
		fail(x.pos, "a %s has no index", v.t)
	}
	if i.c != nil {
		k, ok := constant.Int64Val(constant.ToInt(i.c))
		if !ok || k < 0 || k >= int64(n) {
			fail(x.i.at(), "the index %s is out of the %s's range", i.c, v.t)
		}
		if v.comps != nil {
			return value{t: t, c: v.comps[k]}
		}
		i = value{t: tI32, c: constant.MakeInt64(k)}
	} else {
		// Indices out of range read in range, as WGSL's runtimes have
		// them.
		i = c.concrete(i, x.i.at())
		hi := value{t: i.t, c: constant.MakeInt64(int64(n - 1))}
		if i.t.k == kU32 {
			i = value{t: i.t, e: builtinCall(shaderir.Min, i.t, c.ir(i), c.ir(hi))}
		} else {
			i = value{t: i.t, e: builtinCall(shaderir.Clamp, i.t, c.ir(i), number(constant.MakeInt64(0), kI32), c.ir(hi))}
		}
	}
	if v.comps != nil {
		v = c.concrete(v, x.pos)
	}
	e := mk(shaderir.Expr{Type: shaderir.Index, Exprs: []shaderir.Expr{c.ir(v), c.ir(i)}}, t)
	return value{t: t, e: e, ref: v.ref, mutable: v.mutable, fb: v.t.boolVector()}
}
