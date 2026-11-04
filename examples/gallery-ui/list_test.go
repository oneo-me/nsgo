package ui

import (
	"fmt"
	"slices"
	"testing"

	"github.com/oneo-me/nsgo/core"
)

// fruits is a list's rows in the tests.
func fruits(n int) []string {
	out := make([]string, n)
	for i := range out {
		out[i] = fmt.Sprintf("Fruit %d", i)
	}
	return out
}

// TestListSelects chooses a row by a press and moves the choice with the
// keys, activating the cursor's row with Enter, and the list builds only
// the rows near what shows.
func TestListSelects(t *testing.T) {
	each(t, func(t *testing.T, e core.Environment) {
		rows := fruits(500)
		var sel *Selection
		activated := -1
		app, p := start(t, e, func(cx *core.Scope) core.Node {
			sel = NewSelection(cx, false)
			return List{TestID: "list", Label: "Fruits", Count: func() int { return len(rows) }, Key: func(i int) any { return rows[i] },
				RowLabel: func(i int) string { return rows[i] }, Selection: sel, OnActivate: func(i int) { activated = i },
				Row: func(cx *core.Scope, r core.ListRow) core.Node {
					return core.Text{ContentFunc: func() string { return rows[r.Index()] }}
				}, Style: core.Style{Height: core.Pt(300)}}
		})
		if app.Exists("label:Fruit 300") {
			t.Fatal("the list built rows far from what shows")
		}
		app.Press("label:Fruit 2")
		if !app.Find("label:Fruit 2").Props.Selected || app.Find("label:Fruit 2").Style.Background != p.Selection {
			t.Fatal("a pressed row is not selected")
		}
		app.Key("down")
		app.Key("down")
		if got := sel.Keys(); !slices.Equal(got, []any{"Fruit 4"}) || !app.Find("label:Fruit 4").Props.Selected {
			t.Fatalf("after two downs the selection is %v, want Fruit 4", got)
		}
		app.Key("end")
		app.Settle()
		if !app.Find("label:Fruit 499").Props.Selected {
			t.Fatal("End did not choose and show the last row")
		}
		app.Key("enter")
		if activated != 499 {
			t.Fatalf("Enter activated %d, want 499", activated)
		}
	})
}

// TestListMultiple extends a multiple selection with Shift, chooses every
// row with Mod+A, and moves through a grid's lines.
func TestListMultiple(t *testing.T) {
	rows := fruits(40)
	var sel *Selection
	app, _ := start(t, desktopEnv, func(cx *core.Scope) core.Node {
		sel = NewSelection(cx, true)
		return List{Count: func() int { return len(rows) }, Key: func(i int) any { return rows[i] }, Columns: 4,
			RowLabel: func(i int) string { return rows[i] }, Selection: sel,
			Row: func(cx *core.Scope, r core.ListRow) core.Node {
				return core.Text{ContentFunc: func() string { return rows[r.Index()] }}
			}, Style: core.Style{Height: core.Pt(400)}}
	})
	app.Press("label:Fruit 1")
	app.Key("right")
	app.Key("down")
	if got := sel.Keys(); !slices.Equal(got, []any{"Fruit 6"}) {
		t.Fatalf("right and down chose %v, want Fruit 6 a line below", got)
	}
	app.Key("shift+right")
	if got := sel.Keys(); !slices.Equal(got, []any{"Fruit 6", "Fruit 7"}) {
		t.Fatalf("shift+right chose %v, want Fruit 6 and 7", got)
	}
	app.Key("shift+down")
	if got := sel.Keys(); !slices.Equal(got, []any{"Fruit 6", "Fruit 7", "Fruit 8", "Fruit 9", "Fruit 10", "Fruit 11"}) {
		t.Fatalf("shift+down chose %v, want Fruit 6 to Fruit 11", got)
	}
	app.Key("mod+a")
	if sel.Len() != 40 || sel.Cursor() != "Fruit 11" {
		t.Fatalf("mod+a chose %d rows with the cursor on %v", sel.Len(), sel.Cursor())
	}
	app.Key("space")
	if sel.Has("Fruit 11") || sel.Len() != 39 {
		t.Fatal("space did not unchoose the cursor's row")
	}
}

// TestListFocus takes the keys while the list has the keyboard focus on
// a desktop, ringing the cursor's row, and lets them go with it.
func TestListFocus(t *testing.T) {
	rows := fruits(10)
	var sel *Selection
	app, p := start(t, desktopEnv, func(cx *core.Scope) core.Node {
		sel = NewSelection(cx, false)
		return core.View{Children: []core.Node{
			List{TestID: "list", Count: func() int { return len(rows) }, RowLabel: func(i int) string { return rows[i] }, Selection: sel,
				Row: func(cx *core.Scope, r core.ListRow) core.Node {
					return core.Text{ContentFunc: func() string { return rows[r.Index()] }}
				}, Style: core.Style{Height: core.Pt(300)}},
			Button{Text: "After"},
		}}
	})
	app.Key("tab")
	app.Key("down")
	if got := sel.Keys(); !slices.Equal(got, []any{0}) {
		t.Fatalf("down from no row chose %v, want the first by index", got)
	}
	if app.Find("label:Fruit 0").Style.Outline.Color != p.Info {
		t.Fatal("the cursor's row shows no ring while the list has the focus")
	}
	app.Key("tab")
	if app.Find("label:Fruit 0").Style.Outline.Width != 0 {
		t.Fatal("the ring stays after the focus left")
	}
}

// TestTree expands and collapses branches from the chevrons and the
// arrows, moving to a parent with Left.
func TestTree(t *testing.T) {
	each(t, func(t *testing.T, e core.Environment) {
		kids := map[any][]any{"src": {"src/app", "src/lib"}, "src/app": {"main.go"}, "docs": {"guide.md"}}
		var open *core.Signal[[]string]
		var sel *Selection
		app, _ := start(t, e, func(cx *core.Scope) core.Node {
			open = core.NewSignalFunc(cx, []string{"src"}, slices.Equal)
			sel = NewSelection(cx, false)
			return Tree{TestID: "tree", Roots: func() []any { return []any{"src", "docs", "README.md"} },
				Children: func(k any) []any { return kids[k] }, ToggleLabel: "Expand", Selection: sel,
				Expanded: func(k any) bool { return slices.Contains(open.Get(), k.(string)) },
				OnExpand: func(k any, on bool) {
					next := slices.DeleteFunc(slices.Clone(open.Get()), func(s string) bool { return s == k })
					if on {
						next = append(next, k.(string))
					}
					open.Set(next)
				},
				RowLabel: func(k any) string { return k.(string) },
				Row: func(cx *core.Scope, item TreeItem) core.Node {
					return core.Text{ContentFunc: func() string { return fmt.Sprint(item.Key()) }}
				}, Style: core.Style{Height: core.Pt(300)}}
		})
		if !app.Exists("label:src/lib") || app.Exists("label:main.go") {
			t.Fatal("the tree shows other than src's children")
		}
		if !app.Find("label:src").Props.Expanded || app.Find("label:docs").Props.Expanded {
			t.Fatal("the rows do not report their expansion")
		}
		app.Press("label:src/app >> role:button Expand")
		app.Find("label:main.go")
		app.Press("label:src/app")
		app.Key("left")
		app.WaitForGone("label:main.go", 0)
		app.Key("left")
		if sel.Cursor() != "src" {
			t.Fatalf("left from a collapsed item moved to %v, want its parent", sel.Cursor())
		}
		app.Key("left")
		app.WaitForGone("label:src/app", 0)
		app.Key("right")
		app.Key("right")
		if sel.Cursor() != "src/app" {
			t.Fatalf("right into an expanded branch moved to %v, want its first child", sel.Cursor())
		}
	})
}
