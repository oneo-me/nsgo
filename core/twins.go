package core

import (
	"fmt"
	"reflect"
	"strings"
)

// A field X that can change has a twin XFunc, X bound: what it returns is
// X again whenever what it reads changes. A Style, classes and Uniforms
// take XFunc's over X's; any other XFunc replaces X, so setting both is a
// mistake that mounting reports.

// twin is a replacing XFunc's field index and its X's.
type twin struct{ value, fn int }

// twinPackages are where the elements and components whose twins mounting
// checks live: this module's core, base, mobile and desktop. An app's own
// components keep their own rules.
var twinPackages = strings.TrimSuffix(reflect.TypeFor[View]().PkgPath(), "/core") + "/"

// checkTwins panics when el sets both a field and the XFunc that replaces
// it.
func (t *Tree) checkTwins(el Node) {
	v := reflect.ValueOf(el)
	if v.Kind() == reflect.Pointer && !v.IsNil() {
		v = v.Elem()
	}
	if v.Kind() != reflect.Struct {
		return
	}
	typ := v.Type()
	twins, ok := t.replacing[typ]
	if !ok {
		twins = replacingTwins(typ)
		if t.replacing == nil {
			t.replacing = map[reflect.Type][]twin{}
		}
		t.replacing[typ] = twins
	}
	for _, tw := range twins {
		if !v.Field(tw.fn).IsNil() && !v.Field(tw.value).IsZero() {
			name := typ.Field(tw.value).Name
			panic(fmt.Sprintf("%s sets both %s and %sFunc, which replaces it", typ, name, name))
		}
	}
}

// replacingTwins returns typ's twins whose XFunc replaces X, none for a
// type outside twinPackages.
func replacingTwins(typ reflect.Type) []twin {
	rel, ok := strings.CutPrefix(typ.PkgPath(), twinPackages)
	if !ok || !(rel == "core" || rel == "base" || strings.HasPrefix(rel, "base/") || rel == "mobile" || rel == "desktop") {
		return nil
	}
	var out []twin
	for i := range typ.NumField() {
		f := typ.Field(i)
		name, ok := strings.CutSuffix(f.Name, "Func")
		if !ok || !f.IsExported() || f.Type.Kind() != reflect.Func {
			continue
		}
		x, ok := typ.FieldByName(name)
		if !ok || len(x.Index) != 1 || x.Type == reflect.TypeFor[Style]() || x.Type == reflect.TypeFor[Uniforms]() ||
			strings.HasSuffix(name, "Class") {
			continue
		}
		out = append(out, twin{x.Index[0], i})
	}
	return out
}
