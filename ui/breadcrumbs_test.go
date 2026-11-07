package ui

import (
	"slices"
	"testing"

	"github.com/oneo-me/nsgo/core"
)

// TestBreadcrumbs presses an ancestor, and one collapsed into the menu.
func TestBreadcrumbs(t *testing.T) {
	each(t, func(t *testing.T, e core.Environment) {
		var chosen []int
		choose := func(i int) { chosen = append(chosen, i) }
		app, _ := start(t, e, func(cx *core.Scope) core.Node {
			return core.View{Style: core.Style{Gap: 12}, Children: []core.Node{
				Breadcrumbs{Items: []string{"Home", "Documents", "Projects"}, OnSelect: choose},
				core.View{Style: core.Style{Width: core.Pt(180)}, Children: []core.Node{
					Breadcrumbs{Items: []string{"Disk", "Library", "Archive", "Photos", "Album"}, OnSelect: choose}}},
			}}
		})
		app.Press("role:link Documents")
		if app.Exists("role:link Projects") {
			t.Fatal("the current item is pressable")
		}
		app.Press("role:button More")
		app.Press("role:menuitem Archive")
		if len(chosen) != 2 || chosen[0] != 1 || chosen[1] != 2 {
			t.Fatalf("chose %v, want [1 2]", chosen)
		}
	})
}

// TestBreadcrumbsFit collapses as few ancestors as the width asks, and
// shows them again when the width returns.
func TestBreadcrumbsFit(t *testing.T) {
	each(t, func(t *testing.T, e core.Environment) {
		path := []string{"Home", "Documents", "Projects", "NSGo", "Notes"}
		for _, c := range []struct {
			width core.Points
			links []string
		}{{600, path[:4]}, {260, []string{"Home", "NSGo"}}, {180, path[:1]}, {60, path[:1]}} {
			var width *core.Signal[core.Points]
			app, _ := start(t, e, func(cx *core.Scope) core.Node {
				width = core.NewSignal(cx, c.width)
				return core.View{StyleFunc: func() core.Style { return core.Style{Width: core.Pt(width.Get())} }, Children: []core.Node{
					Breadcrumbs{Items: path, TestID: "path"}}}
			})
			if got := app.Find("path").Frame.Size.Width; got != c.width {
				t.Fatalf("the path is %v wide in %v", got, c.width)
			}
			for _, name := range path[:4] {
				if got, want := app.Exists("role:link "+name), slices.Contains(c.links, name); got != want {
					t.Errorf("%v wide: link %s shown = %v, want %v", c.width, name, got, want)
				}
			}
			if got, want := app.Exists("role:button More"), len(c.links) < 4; got != want {
				t.Errorf("%v wide: ellipsis shown = %v, want %v", c.width, got, want)
			}
			app.Post(func() { width.Set(600) })
			app.Settle()
			if !app.Exists("role:link Documents") || app.Exists("role:button More") {
				t.Errorf("%v wide: the path stays collapsed at 600", c.width)
			}
		}
	})
}
