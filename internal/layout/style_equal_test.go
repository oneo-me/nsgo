package layout

import (
	"reflect"
	"testing"
)

// TestStyleEqualSeesEveryField changes each field of a style, and each
// field inside it, and wants Equal to see it.
func TestStyleEqualSeesEveryField(t *testing.T) {
	typ := reflect.TypeFor[Style]()
	for i := range typ.NumField() {
		var s Style
		if !setLeaf(reflect.ValueOf(&s).Elem().Field(i)) {
			t.Fatalf("cannot set %s", typ.Field(i).Name)
		}
		var zero Style
		if s.Equal(&zero) || zero.Equal(&s) {
			t.Errorf("Equal misses %s", typ.Field(i).Name)
		}
	}
}

// setLeaf makes v differ from its zero value at its first leaf.
func setLeaf(v reflect.Value) bool {
	switch v.Kind() {
	case reflect.Bool:
		v.SetBool(true)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		v.SetInt(1)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		v.SetUint(1)
	case reflect.Float32, reflect.Float64:
		v.SetFloat(1)
	case reflect.String:
		v.SetString("a")
	case reflect.Struct:
		for i := range v.NumField() {
			if setLeaf(v.Field(i)) {
				return true
			}
		}
		return false
	case reflect.Slice:
		v.Set(reflect.MakeSlice(v.Type(), 1, 1))
	case reflect.Pointer:
		v.Set(reflect.New(v.Type().Elem()))
	default:
		return false
	}
	return true
}

// TestStyleEqualComparesLists compares grid lists by their contents.
func TestStyleEqualComparesLists(t *testing.T) {
	mk := func() Style {
		return Style{Display: DisplayGrid,
			GridTemplateColumns: []GridTemplateComponent{{Repeat: &Repetition{Count: AutoFill,
				Tracks: []TrackSizing{Track(Length{Value: 40})}, LineNames: [][]string{{"a"}, {"b"}}}}},
			GridTemplateAreas: GridTemplateAreas{Areas: []GridTemplateArea{{Name: "main", RowEnd: 2, ColumnEnd: 2}}, Rows: 1, Columns: 1}}
	}
	a, b := mk(), mk()
	if !a.Equal(&b) {
		t.Fatal("two styles made alike differ")
	}
	b.GridTemplateColumns[0].Repeat.LineNames[1][0] = "c"
	if a.Equal(&b) {
		t.Fatal("a repetition's line name changed unseen")
	}
	empty := Style{GridAutoRows: []TrackSizing{}}
	var none Style
	if !empty.Equal(&none) {
		t.Fatal("an empty list differs from none")
	}
}
