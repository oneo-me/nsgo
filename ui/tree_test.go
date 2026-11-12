package ui

import (
	"fmt"
	"slices"
	"testing"

	"github.com/nsgo-dev/nsgo/core"
)

// TestTree expands and collapses branches from the chevrons and the
// arrows, moving to a parent with Left.
func TestTree(t *testing.T) {
	each(t, func(t *testing.T, e core.Environment) {
		kids := map[any][]any{"src": {"src/app", "src/lib"}, "src/app": {"main.go"}, "docs": {"guide.md"}}
		var open *core.Signal[[]string]
		var sel *Selection
		app, p := start(t, e, func(cx *core.Scope) core.Node {
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
		app.Press(`text:"src/app"`)
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
		app.Press(`text:"docs"`)
		if app.Exists("label:guide.md") {
			t.Fatal("a press expanded a branch")
		}
		if got := app.Find(`text:"docs"`).Parent.Parent.Style.Background; got != Alpha(p.Text, 0.1) {
			t.Fatalf("a selected row's background is %v, want the sidebar's", got)
		}
		app.Press(`text:"docs"`)
		app.Find("label:guide.md")
	})
}
