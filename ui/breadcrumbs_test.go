package ui

import (
	"testing"

	"github.com/oneo-me/nsgo/core"
)

// TestBreadcrumbs presses an ancestor, and one collapsed into the menu.
func TestBreadcrumbs(t *testing.T) {
	each(t, func(t *testing.T, e core.Environment) {
		var chosen []int
		app, _ := start(t, e, func(cx *core.Scope) core.Node {
			path := []string{"Home", "Documents", "Projects", "NSGo", "Notes"}
			return core.View{Style: core.Style{Gap: 12}, Children: []core.Node{
				Breadcrumbs{Items: path[:3], OnSelect: func(i int) { chosen = append(chosen, i) }},
				Breadcrumbs{Items: path, MaxItems: 3, MoreLabel: "More", OnSelect: func(i int) { chosen = append(chosen, i) }},
			}}
		})
		app.Press("role:link Documents")
		if app.Exists("role:link Projects") {
			t.Fatal("the current item is pressable")
		}
		app.Press("role:button More")
		app.Press("role:menuitem Projects")
		if len(chosen) != 2 || chosen[0] != 1 || chosen[1] != 2 {
			t.Fatalf("chose %v, want [1 2]", chosen)
		}
	})
}
