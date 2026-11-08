package wgsl

import (
	"go/constant"
	"math"
	"strings"

	"github.com/oneo-me/nsgo/internal/shaderir"
)

// call checks a call: a user's function, a type's constructor, bitcast or
// a built-in function. asStmt allows a function without a result.
func (c *checker) call(x *callExpr, out *[]shaderir.Stmt, asStmt bool) value {
	name := x.fn.name
	if sym := c.scope.lookup(name); sym != nil {
		switch sym.kind {
		case symFn:
			return c.callFn(sym.fn, x, out)
		case symType:
			if sym.t.k == kNone {
				fail(x.at(), "a shader constructs no %s", name)
			}
			return c.construct(sym.t, x, out)
		}
		fail(x.at(), "%s is not a function", name)
	}
	if bt, ok := builtinTypes[name]; ok {
		return c.construct(bt, x, out)
	}
	switch name {
	case "bitcast":
		return c.bitcast(x, out)
	case "vec2", "vec3", "vec4":
		if x.fn.args == nil {
			return c.vector(ty{k: kVec, n: int(name[3] - '0')}, false, x, out)
		}
		return c.construct(c.resolveType(x.fn), x, out)
	case "array":
		if x.fn.args == nil {
			return c.arrayOf(nil, x, out)
		}
		return c.construct(c.resolveType(x.fn), x, out)
	case "f16":
		fail(x.at(), "f16 is not supported")
	}
	if _, ok := matShapes[name]; ok {
		fn := x.fn
		if fn.args == nil && !strings.HasSuffix(name, "f") {
			// The components' type is f32, the only one.
			fn = &typeExpr{name: name, args: []any{&typeExpr{name: "f32", pos: fn.pos}}, pos: fn.pos}
		}
		return c.construct(c.resolveType(fn), x, out)
	}
	if strings.HasPrefix(name, "__") && x.fn.pos >= c.user {
		return c.intrinsic(x, out)
	}
	if f := builtinFuncs[name]; f != nil {
		args := make([]value, len(x.args))
		for i, a := range x.args {
			args[i] = c.expr(a, out)
		}
		return f(c, args, x.fn.pos, out)
	}
	if strings.HasPrefix(name, "texture") || name == "arrayLength" || strings.HasPrefix(name, "atomic") {
		fail(x.at(), "a shader takes no resources: read pictures with sample_content, sample_backdrop, sample_from and sample_to")
	}
	fail(x.at(), "%s is not declared", name)
	return value{}
}

func (c *checker) callFn(f *fnInfo, x *callExpr, out *[]shaderir.Stmt) value {
	if len(x.args) != len(f.params) {
		fail(x.at(), "%s takes %d arguments, not %d", f.decl.name, len(f.params), len(x.args))
	}
	args := []shaderir.Expr{{Type: shaderir.FunctionExpr, Index: f.index}}
	for i, a := range x.args {
		v := c.convert(c.expr(a, out), f.params[i], a.at())
		args = append(args, c.ir(v))
	}
	if c.cur == nil {
		fail(x.at(), "the call is not a constant")
	}
	c.cur.info.calls = append(c.cur.info.calls, f)
	return value{t: f.ret, e: mk(shaderir.Expr{Type: shaderir.Call, Exprs: args}, f.ret)}
}

// intrinsic reads what only the built-ins' source may: a picture's texel
// at a pixel (__texel(k, p)) and its size in pixels (__picture_size(k)).
func (c *checker) intrinsic(x *callExpr, out *[]shaderir.Stmt) value {
	k := 0
	if len(x.args) > 0 {
		v := c.constant(x.args[0])
		n, _ := constant.Int64Val(v.c)
		k = int(n)
	}
	c.pictures = max(c.pictures, k+1)
	if c.cur != nil {
		c.cur.info.textures |= 1 << k
	}
	v2 := vec(kF32, 2)
	switch x.fn.name {
	case "__texel":
		p := c.convert(c.expr(x.args[1], out), v2, x.at())
		// The texel's center, which every language's read floors.
		center := mk(shaderir.Expr{Type: shaderir.Binary, Op: shaderir.Add, Exprs: []shaderir.Expr{c.ir(p), floatNumber(0.5)}}, v2)
		return value{t: vec(kF32, 4), e: builtinCall(shaderir.TexelAt, vec(kF32, 4), shaderir.Expr{Type: shaderir.TextureVariable, Index: k}, center)}
	case "__picture_size":
		sizes := shaderir.Expr{Type: shaderir.UniformVariable, Index: 2}
		return value{t: v2, e: mk(shaderir.Expr{Type: shaderir.Index, Exprs: []shaderir.Expr{sizes, number(constant.MakeInt64(int64(k)), kI32)}}, v2)}
	}
	fail(x.at(), "%s is not declared", x.fn.name)
	return value{}
}

func (c *checker) construct(t ty, x *callExpr, out *[]shaderir.Stmt) value {
	switch t.k {
	case kVec:
		return c.vector(t, true, x, out)
	case kMat:
		return c.matrix(t, x, out)
	case kArray:
		return c.arrayOf(&t, x, out)
	}
	// A scalar: its zero, or a conversion.
	if len(x.args) == 0 {
		return value{t: t, c: zeroOf(t.k)}
	}
	if len(x.args) != 1 {
		fail(x.at(), "%s takes one value", t)
	}
	v := c.expr(x.args[0], out)
	return c.convertScalar(v, t, x.at())
}

func zeroOf(k kind) constant.Value {
	switch k {
	case kBool:
		return constant.MakeBool(false)
	case kF32, kAbsFloat:
		return constant.MakeFloat64(0)
	}
	return constant.MakeInt64(0)
}

// convertScalar converts v to the scalar t, as t(v) does.
func (c *checker) convertScalar(v value, t ty, pos int) value {
	if !v.t.isScalar() {
		fail(pos, "cannot convert a %s to a %s", v.t, t)
	}
	if v.c != nil {
		return value{t: t, c: foldConvert(v.c, v.t.k, t.k, pos)}
	}
	if v.t.equal(t) {
		return v
	}
	x := c.ir(v)
	switch t.k {
	case kBool:
		zero := number(zeroOf(v.t.k), v.t.k)
		return value{t: t, e: shaderir.Expr{Type: shaderir.Binary, Op: shaderir.NotEqualOp, Exprs: []shaderir.Expr{x, zero}}}
	case kF32:
		return value{t: t, e: builtinCall(shaderir.FloatF, t, x)}
	case kI32:
		return value{t: t, e: builtinCall(shaderir.IntF, t, x)}
	case kU32:
		return value{t: t, e: builtinCall(shaderir.UintF, t, x)}
	}
	fail(pos, "cannot convert a %s to a %s", v.t, t)
	return value{}
}

// foldConvert converts a constant as t(x) does: floats to integers
// truncated and clamped, as WGSL's conversions saturate.
func foldConvert(x constant.Value, from, to kind, pos int) constant.Value {
	if from == kBool {
		b := constant.BoolVal(x)
		switch to {
		case kBool:
			return x
		case kF32:
			if b {
				return constant.MakeFloat64(1)
			}
			return constant.MakeFloat64(0)
		}
		if b {
			return constant.MakeInt64(1)
		}
		return constant.MakeInt64(0)
	}
	switch to {
	case kBool:
		return constant.MakeBool(constant.Sign(x) != 0)
	case kF32:
		return constant.ToFloat(x)
	case kI32, kU32:
		f, _ := constant.Float64Val(constant.ToFloat(x))
		f = math.Trunc(f)
		if x.Kind() == constant.Int {
			v, _ := constant.Int64Val(x)
			if to == kU32 {
				return constant.MakeInt64(int64(uint32(v)))
			}
			return constant.MakeInt64(int64(int32(v)))
		}
		if to == kU32 {
			return constant.MakeInt64(int64(math.Max(0, math.Min(f, math.MaxUint32))))
		}
		return constant.MakeInt64(int64(math.Max(math.MinInt32, math.Min(f, math.MaxInt32))))
	}
	fail(pos, "cannot convert to a %s", ty{k: to})
	return nil
}

// vector constructs t, whose components' type is inferred from the
// arguments unless typed.
func (c *checker) vector(t ty, typed bool, x *callExpr, out *[]shaderir.Stmt) value {
	args := make([]value, len(x.args))
	count := 0
	for i, a := range x.args {
		args[i] = c.expr(a, out)
		switch args[i].t.k {
		case kVec:
			count += args[i].t.n
		case kBool, kI32, kU32, kF32, kAbsInt, kAbsFloat:
			count++
		default:
			fail(a.at(), "a vector takes no %s", args[i].t)
		}
	}
	if !typed {
		// The components take the arguments' type, a concrete one first.
		t.elem = kNone
		for _, a := range args {
			if s := a.t.scalar(); !a.t.abstract() {
				t.elem = s
				break
			}
		}
		if t.elem == kNone {
			for _, a := range args {
				if s := a.t.scalar(); t.elem == kNone || s == kAbsFloat {
					t.elem = s
				}
			}
		}
		if t.elem == kNone {
			fail(x.at(), "%s() needs its components' type: write vec%df", x.fn.name, t.n)
		}
	}
	if len(args) == 0 {
		comps := make([]constant.Value, t.n)
		for i := range comps {
			comps[i] = zeroOf(t.elem)
		}
		return c.vectorOfConstants(t, comps, x.at())
	}
	// A conversion of a vector of another type.
	if len(args) == 1 && args[0].t.k == kVec {
		a := args[0]
		if a.t.n != t.n {
			fail(x.at(), "a %s takes %d components, not a %s", t, t.n, a.t)
		}
		return c.convertVector(a, t, x.at(), out)
	}
	// A splat of one scalar.
	if len(args) == 1 {
		a := c.convertElem(args[0], t.elem, x.args[0].at(), true)
		if a.c != nil {
			comps := make([]constant.Value, t.n)
			for i := range comps {
				comps[i] = a.c
			}
			return c.vectorOfConstants(t, comps, x.at())
		}
		if t.elem == kBool {
			a = value{t: tF32, e: selectNumber(c.ir(a), 1, 0)}
		}
		return value{t: t, e: builtinCall(vecFunc(t), t, c.ir(a))}
	}
	if count != t.n {
		fail(x.at(), "a %s takes %d components, not %d", t, t.n, count)
	}
	var irArgs []shaderir.Expr
	var comps []constant.Value
	allConst := true
	for i, a := range args {
		if a.t.k == kVec {
			a = c.convertVector(a, a.t.withScalar(t.elem), x.args[i].at(), out)
			allConst = allConst && a.comps != nil
			if a.comps != nil {
				comps = append(comps, a.comps...)
			}
		} else {
			a = c.convertElem(a, t.elem, x.args[i].at(), false)
			allConst = allConst && a.c != nil
			if a.c != nil {
				comps = append(comps, a.c)
			}
			if t.elem == kBool && a.c == nil {
				a = value{t: tF32, e: selectNumber(c.ir(a), 1, 0)}
			}
		}
		if t.elem == kBool && a.c != nil {
			irArgs = append(irArgs, floatNumber(map[bool]float64{true: 1}[constant.BoolVal(a.c)]))
			continue
		}
		irArgs = append(irArgs, c.ir(a))
	}
	if allConst {
		return c.vectorOfConstants(t, comps, x.at())
	}
	t = t.concrete()
	return value{t: t, e: builtinCall(vecFunc(t), t, irArgs...)}
}

// convertElem converts a vector's component to elem: an abstract one
// implicitly, a concrete one only where it is elem already, or, in a
// splat of one scalar, as a conversion would.
func (c *checker) convertElem(a value, elem kind, pos int, splat bool) value {
	if a.t.k == elem {
		return a
	}
	if a.t.abstract() {
		return c.convert(a, ty{k: elem}, pos)
	}
	fail(pos, "a vector of %s takes no %s", scalarName(elem), a.t)
	return value{}
}

func (c *checker) vectorOfConstants(t ty, comps []constant.Value, pos int) value {
	if t.abstract() {
		return value{t: t, comps: comps}
	}
	if t.elem == kBool {
		var args []shaderir.Expr
		for _, x := range comps {
			args = append(args, floatNumber(map[bool]float64{true: 1}[constant.BoolVal(x)]))
		}
		return value{t: t, e: builtinCall(vecFunc(t), t, args...)}
	}
	return c.convert(value{t: t.withScalar(abstractOf(t.elem)), comps: comps}, t, pos)
}

func abstractOf(k kind) kind {
	if isFloat(k) {
		return kAbsFloat
	}
	return kAbsInt
}

// convertVector converts the vector a to t, as t(a) does.
func (c *checker) convertVector(a value, t ty, pos int, out *[]shaderir.Stmt) value {
	if a.t.equal(t) {
		return a
	}
	if a.comps != nil {
		if t.elem != kBool {
			comps := make([]constant.Value, len(a.comps))
			for i, x := range a.comps {
				if t.abstract() {
					comps[i] = c.convertConst(x, ty{k: a.t.elem}, ty{k: t.elem}, pos)
				} else {
					comps[i] = foldConvert(x, a.t.elem, t.elem, pos)
				}
			}
			return c.vectorOfConstants(t, comps, pos)
		}
		a = c.concrete(a, pos)
	}
	switch {
	case t.elem == kBool:
		// Each component is != 0.
		a = c.reuse(a, out)
		var args []shaderir.Expr
		for i := range t.n {
			cmp := shaderir.Expr{Type: shaderir.Binary, Op: shaderir.NotEqualOp, Exprs: []shaderir.Expr{component(c.ir(a), i, a.t), number(zeroOf(a.t.elem), a.t.elem)}}
			args = append(args, selectNumber(cmp, 1, 0))
		}
		return value{t: t, e: builtinCall(vecFunc(t), t, args...)}
	case a.t.boolVector() && t.elem == kF32:
		return value{t: t, e: a.e}
	}
	return value{t: t, e: builtinCall(vecFunc(t), t, c.ir(a))}
}

func (c *checker) matrix(t ty, x *callExpr, out *[]shaderir.Stmt) value {
	f := [...]shaderir.BuiltinFunc{2: shaderir.Mat2F, 3: shaderir.Mat3F, 4: shaderir.Mat4F}[t.n]
	var args []shaderir.Expr
	switch {
	case len(x.args) == 0:
		for range t.n * t.rows {
			args = append(args, floatNumber(0))
		}
	case len(x.args) == 1:
		v := c.expr(x.args[0], out)
		if !v.t.equal(t) {
			fail(x.at(), "a %s takes columns, its components or a %s", t, t)
		}
		return v
	case len(x.args) == t.n:
		col := vec(kF32, t.rows)
		for _, a := range x.args {
			args = append(args, c.ir(c.convert(c.expr(a, out), col, a.at())))
		}
	case len(x.args) == t.n*t.rows:
		for _, a := range x.args {
			args = append(args, c.ir(c.convert(c.expr(a, out), tF32, a.at())))
		}
	default:
		fail(x.at(), "a %s takes %d columns or %d components", t, t.n, t.n*t.rows)
	}
	return value{t: t, e: builtinCall(f, t, args...)}
}

// arrayOf constructs an array in a local, element by element.
func (c *checker) arrayOf(t *ty, x *callExpr, out *[]shaderir.Stmt) value {
	args := make([]value, len(x.args))
	for i, a := range x.args {
		args[i] = c.expr(a, out)
	}
	var at ty
	if t != nil {
		at = *t
		if len(args) != 0 && len(args) != at.n {
			fail(x.at(), "a %s takes %d elements, not %d", at, at.n, len(args))
		}
	} else {
		if len(args) == 0 {
			fail(x.at(), "array() needs its elements' type and length")
		}
		e := args[0].t
		for _, a := range args {
			if !a.t.abstract() {
				e = a.t
				break
			}
		}
		e = e.concrete()
		at = ty{k: kArray, n: len(args), arr: &e}
	}
	if c.cur == nil {
		fail(x.at(), "an array is not a constant")
	}
	idx := c.newLocal(at)
	arr := local(idx, at)
	if len(args) == 0 {
		*out = append(*out, shaderir.Stmt{Type: shaderir.Init, InitIndex: idx})
	}
	for i, a := range args {
		v := c.convert(a, *at.arr, x.args[i].at())
		elem := mk(shaderir.Expr{Type: shaderir.Index, Exprs: []shaderir.Expr{arr, number(constant.MakeInt64(int64(i)), kI32)}}, *at.arr)
		*out = append(*out, assign(elem, c.ir(v)))
	}
	return value{t: at, e: arr}
}

func (c *checker) bitcast(x *callExpr, out *[]shaderir.Stmt) value {
	if len(x.fn.args) != 1 || len(x.args) != 1 {
		fail(x.at(), "bitcast<T>(x) takes a type and a value")
	}
	t := c.templateType(x.fn.args[0], x.fn.pos)
	v := c.concrete(c.expr(x.args[0], out), x.args[0].at())
	n := func(t ty) int {
		if t.k == kVec {
			return t.n
		}
		return 1
	}
	from, to := v.t.scalar(), t.scalar()
	ok := func(k kind) bool { return k == kI32 || k == kU32 || k == kF32 }
	if !ok(from) || !ok(to) || n(v.t) != n(t) || v.t.k == kArray || t.k == kArray || v.t.k == kMat || t.k == kMat {
		fail(x.at(), "cannot bitcast a %s to a %s", v.t, t)
	}
	if from == to {
		return value{t: t, e: c.ir(v)}
	}
	var f shaderir.BuiltinFunc
	switch {
	case from == kF32 && to == kI32:
		f = shaderir.FloatBitsToInt
	case from == kF32 && to == kU32:
		f = shaderir.FloatBitsToUint
	case from == kI32 && to == kF32:
		f = shaderir.IntBitsToFloat
	case from == kU32 && to == kF32:
		f = shaderir.UintBitsToFloat
	case t.k == kVec:
		// Between i32 and u32, a conversion keeps the bits.
		f = vecFunc(t)
	case to == kU32:
		f = shaderir.UintF
	default:
		f = shaderir.IntF
	}
	return value{t: t, e: builtinCall(f, t, c.ir(v))}
}
