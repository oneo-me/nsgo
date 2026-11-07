package base_test

import (
	"slices"
	"strconv"
	"testing"
	"time"

	"github.com/oneo-me/nsgo/base"
	"github.com/oneo-me/nsgo/core"
	"github.com/oneo-me/nsgo/nsgotest"
)

// listRows are the rows the list tests show.
var listRows = []string{"Apple", "Apricot", "Banana", "Blueberry", "Cherry"}

// rowsOf is a List of listRows' props, sized by the test.
func rowsOf(sel *base.Selection, activate func(int)) base.List {
	return base.List{TestID: "list", Label: "Fruits", Count: func() int { return len(listRows) }, Key: func(i int) any { return listRows[i] },
		RowLabel: func(i int) string { return listRows[i] }, Selection: sel, OnActivate: activate,
		Row: func(_ *core.Scope, r core.ListRow) core.Node {
			return core.Text{ContentFunc: func() string { return listRows[r.Index()] }}
		}, Style: core.Style{Height: core.Pt(300)}, ItemStyle: core.Style{Height: core.Pt(30)}}
}

// TestListHasNoLook leaves everything but layout's needs to the app, and
// reads what the app styles a row with from its states.
func TestListHasNoLook(t *testing.T) {
	selected := core.RGB(0x123456)
	var sel *base.Selection
	app := startDesktop(t, func(cx *core.Scope) core.Node {
		sel = base.NewSelection(cx, false)
		l := rowsOf(sel, nil)
		l.ItemStyle.Selected = &core.Style{Background: selected}
		return l
	})
	row := app.Find("label:Apple")
	if row.Style.Background != 0 || row.Style.CornerRadius != 0 {
		t.Fatalf("a row has a look of its own: %+v", row.Style)
	}
	app.Press("label:Banana")
	if got := app.Find("label:Banana").Style.Background; got != selected {
		t.Fatalf("a chosen row's background is %v, want the app's", got)
	}
}

// TestListRoles says a list of choices is one to assistive technology, its
// rows options, and rows that only activate are buttons.
func TestListRoles(t *testing.T) {
	app := startDesktop(t, func(cx *core.Scope) core.Node {
		return core.View{Style: core.Style{FlexGrow: 1}, Children: []core.Node{
			func() core.Node {
				l := rowsOf(base.NewSelection(cx, false), nil)
				l.TestID, l.Label = "choices", "Choices"
				return l
			}(),
			func() core.Node {
				l := rowsOf(nil, func(int) {})
				l.TestID, l.Label = "actions", "Actions"
				return l
			}(),
		}}
	})
	app.Find("id:choices >> role:option Apple")
	app.Find("id:actions >> role:button Apple")
	if !app.Exists("role:listbox Choices") {
		t.Fatal("a list of choices is no listbox")
	}
	if app.Exists("role:listbox Actions") {
		t.Fatal("a list with no selection is a listbox")
	}
}

// TestListPlainRows takes no keys and no presses without a Selection or
// OnActivate.
func TestListPlainRows(t *testing.T) {
	app := startDesktop(t, func(cx *core.Scope) core.Node { return rowsOf(nil, nil) })
	if v := app.Find("label:Apple"); v.Props.Role != 0 {
		t.Fatalf("a plain row has role %v", v.Props.Role)
	}
}

// TestListTypeahead moves the cursor by what was typed, repeats of a letter
// cycling through its rows, and starts over after a pause.
func TestListTypeahead(t *testing.T) {
	var sel *base.Selection
	app := startDesktop(t, func(cx *core.Scope) core.Node {
		sel = base.NewSelection(cx, false)
		return rowsOf(sel, nil)
	})
	cursor := func(want string) {
		t.Helper()
		if got := sel.Keys(); !slices.Equal(got, []any{want}) {
			t.Fatalf("the selection is %v, want %s", got, want)
		}
	}
	app.Press("label:Cherry")
	app.Key("b")
	cursor("Banana")
	app.Key("b")
	cursor("Blueberry")
	app.Key("l")
	cursor("Blueberry")
	app.Advance(time.Second)
	app.Key("a")
	cursor("Apple")
	app.Key("p")
	cursor("Apple")
	app.Key("r")
	cursor("Apricot")
	app.Key("z")
	cursor("Apricot")
}

// TestListScreenReader has the screen reader's focus follow the cursor as
// the keys move it.
func TestListScreenReader(t *testing.T) {
	app := nsgotest.Start(t, func(cx *core.Scope) core.Node { return rowsOf(base.NewSelection(cx, false), nil) },
		nsgotest.Environment(core.Environment{Idiom: core.IdiomDesktop, ScreenReader: true}))
	app.Press("label:Apple")
	app.Key("down")
	if v := app.AccessibilityFocus; v == nil || v.Props.Label != "Apricot" {
		t.Fatalf("the screen reader's focus is on %+v, want Apricot", v)
	}
}

// TestListWithoutLabels binds no typeahead, whose letters stay the app's.
func TestListWithoutLabels(t *testing.T) {
	var sel *base.Selection
	heard := 0
	app := startDesktop(t, func(cx *core.Scope) core.Node {
		sel = base.NewSelection(cx, false)
		cx.OnKey("b", func() { heard++ })
		l := rowsOf(sel, nil)
		l.RowLabel = nil
		return l
	})
	app.Press("text:Apple")
	app.Key("b")
	if heard != 1 || !slices.Equal(sel.Keys(), []any{"Apple"}) {
		t.Fatalf("b reached the app %d times, the selection is %v", heard, sel.Keys())
	}
}

// TestListTaps follows a touch screen: a tap activates unless the list is
// Multiple, which toggles, or TapSelects, whose second tap activates.
func TestListTaps(t *testing.T) {
	phone := nsgotest.Environment(core.Environment{Idiom: core.IdiomPhone})
	for _, c := range []struct {
		name               string
		multiple, tapFirst bool
		activates          []string
		keys               []any
	}{
		{name: "one", activates: []string{"0"}, keys: []any{"Apple"}},
		{name: "multiple", multiple: true, keys: []any{"Apple"}},
		{name: "tapSelects", tapFirst: true, keys: []any{"Apple"}},
	} {
		t.Run(c.name, func(t *testing.T) {
			var sel *base.Selection
			var activated []string
			app := nsgotest.Start(t, func(cx *core.Scope) core.Node {
				sel = base.NewSelection(cx, c.multiple)
				l := rowsOf(sel, func(i int) { activated = append(activated, strconv.Itoa(i)) })
				l.TapSelects = c.tapFirst
				return l
			}, phone)
			app.Press("label:Apple")
			if !slices.Equal(activated, c.activates) || !slices.Equal(sel.Keys(), c.keys) {
				t.Fatalf("a tap activated %v and chose %v", activated, sel.Keys())
			}
		})
	}
}

// TestListEmpty shows Empty with no rows, at the app's size.
func TestListEmpty(t *testing.T) {
	app := startDesktop(t, func(cx *core.Scope) core.Node {
		return base.List{TestID: "list", Empty: core.Text{Content: "Nothing"}, Style: core.Style{Height: core.Pt(100)}}
	})
	app.Find("text:Nothing")
	if got := app.Find("id:list").Frame.Size.Height; got != 100 {
		t.Fatalf("the list is %v tall, want the app's 100", got)
	}
}
