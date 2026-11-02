package bundler

import (
	"reflect"

	"github.com/dop251/goja/ast"
)

var nodeType = reflect.TypeFor[ast.Node]()

// walk calls fn for n and the nodes under it, depth first; fn returning
// false skips a node's children. goja's AST has no visitor, so walk reads
// the nodes' fields.
func walk(n ast.Node, fn func(ast.Node) bool) {
	v := reflect.ValueOf(n)
	if !v.IsValid() || v.Kind() == reflect.Pointer && v.IsNil() {
		return
	}
	if !fn(n) {
		return
	}
	walkFields(v, fn)
}

func walkFields(v reflect.Value, fn func(ast.Node) bool) {
	for v.Kind() == reflect.Pointer || v.Kind() == reflect.Interface {
		if v.IsNil() {
			return
		}
		v = v.Elem()
	}
	if v.Kind() != reflect.Struct {
		return
	}
	t := v.Type()
	for i := range v.NumField() {
		// The hoisted declarations repeat nodes the body holds.
		if t.Field(i).Name == "DeclarationList" {
			continue
		}
		walkValue(v.Field(i), fn)
	}
}

func walkValue(f reflect.Value, fn func(ast.Node) bool) {
	switch f.Kind() {
	case reflect.Interface, reflect.Pointer:
		if f.IsNil() {
			return
		}
		if n, ok := f.Interface().(ast.Node); ok {
			walk(n, fn)
			return
		}
		walkFields(f, fn)
	case reflect.Slice:
		for i := range f.Len() {
			walkValue(f.Index(i), fn)
		}
	case reflect.Struct:
		if f.CanAddr() && f.Addr().Type().Implements(nodeType) {
			walk(f.Addr().Interface().(ast.Node), fn)
			return
		}
		walkFields(f, fn)
	}
}
