//go:build !nsgo_bundled

package compiler

import (
	"github.com/dop251/goja/ast"
	"github.com/dop251/goja/token"
)

// The bindings of a module, enough to resolve tags and refs as NSH's
// compiler does with Oxc's scopes: what a name means at a place is its
// innermost declaration around it.

// scopeRange is where a scope's names are visible.
type scopeRange struct {
	start, end int
	function   bool // a function's or the program's, where var declares
}

// declaration is one name a scope declares.
type declaration struct {
	scope scopeRange
	fixed bool // const, a function or a class: a ref cannot assign it
}

// bindings are a module's declarations by name.
type bindings struct {
	scopes []scopeRange
	names  map[string][]declaration
}

func newBindings(prog *ast.Program, length int) *bindings {
	b := &bindings{names: map[string][]declaration{}}
	b.scopes = append(b.scopes, scopeRange{0, length, true})
	walk(prog, func(n ast.Node) bool {
		switch n := n.(type) {
		case *ast.FunctionLiteral, *ast.ArrowFunctionLiteral:
			b.scopes = append(b.scopes, scopeRange{off(n.Idx0()), off(n.Idx1()), true})
		case *ast.BlockStatement:
			b.scopes = append(b.scopes, scopeRange{off(n.Idx0()), off(n.Idx1()), false})
		}
		return true
	})
	walk(prog, func(n ast.Node) bool {
		switch n := n.(type) {
		case *ast.VariableStatement:
			for _, d := range n.List {
				b.pattern(d.Target, off(n.Idx0()), true, false)
			}
		case *ast.LexicalDeclaration:
			for _, d := range n.List {
				b.pattern(d.Target, off(n.Idx0()), false, n.Token == token.CONST)
			}
		case *ast.FunctionDeclaration:
			if id := n.Function.Name; id != nil {
				b.declare(id.Name.String(), max(off(n.Idx0())-1, 0), false, true)
			}
		case *ast.ClassDeclaration:
			if id := n.Class.Name; id != nil {
				b.declare(id.Name.String(), max(off(n.Idx0())-1, 0), false, true)
			}
		case *ast.FunctionLiteral:
			b.params(n.ParameterList, off(n.Idx0()))
		case *ast.ArrowFunctionLiteral:
			b.params(n.ParameterList, off(n.Idx0()))
		case *ast.CatchStatement:
			if n.Parameter != nil {
				b.pattern(n.Parameter, off(n.Body.Idx0()), false, false)
			}
		}
		return true
	})
	return b
}

func (b *bindings) params(list *ast.ParameterList, at int) {
	if list == nil {
		return
	}
	for _, p := range list.List {
		b.pattern(p.Target, at, true, false)
	}
	if list.Rest != nil {
		b.pattern(list.Rest, at, true, false)
	}
}

// pattern declares the names a binding target names.
func (b *bindings) pattern(target ast.Expression, at int, function, fixed bool) {
	switch t := target.(type) {
	case *ast.Identifier:
		b.declare(t.Name.String(), at, function, fixed)
	case *ast.ObjectPattern:
		for _, p := range t.Properties {
			switch p := p.(type) {
			case *ast.PropertyShort:
				b.declare(p.Name.Name.String(), at, function, fixed)
			case *ast.PropertyKeyed:
				b.pattern(p.Value, at, function, fixed)
			}
		}
		if t.Rest != nil {
			b.pattern(t.Rest, at, function, fixed)
		}
	case *ast.ArrayPattern:
		for _, e := range t.Elements {
			b.pattern(e, at, function, fixed)
		}
		if t.Rest != nil {
			b.pattern(t.Rest, at, function, fixed)
		}
	case *ast.AssignExpression:
		b.pattern(t.Left, at, function, fixed)
	}
}

// declare declares name in the innermost scope around at: a function's
// for var and parameters, a block's otherwise.
func (b *bindings) declare(name string, at int, function, fixed bool) {
	best := b.scopes[0]
	for _, s := range b.scopes {
		if s.start <= at && at < s.end && (s.function || !function) && s.end-s.start < best.end-best.start {
			best = s
		}
	}
	b.names[name] = append(b.names[name], declaration{best, fixed})
}

// lookup returns the innermost declaration of name visible at at.
func (b *bindings) lookup(name string, at int) (declaration, bool) {
	var found declaration
	ok := false
	for _, d := range b.names[name] {
		if d.scope.start <= at && at < d.scope.end && (!ok || d.scope.end-d.scope.start < found.scope.end-found.scope.start) {
			found, ok = d, true
		}
	}
	return found, ok
}
