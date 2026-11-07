package ui

import (
	"testing"

	"github.com/oneo-me/nsgo/base"
	"github.com/oneo-me/nsgo/core"
)

// TestNavPage gives a page without a header of its own a themed one with
// a back button, below the status bar where the system has a header. On
// iOS the system's header stands in for it.
func TestNavPage(t *testing.T) {
	android := phoneEnv
	android.OS, android.HeaderHeight, android.SafeArea = core.OSAndroid, 44, core.Edges{Top: 47, Bottom: 34}
	for _, e := range []core.Environment{desktopEnv, android} {
		popped := false
		app, p := start(t, e, func(cx *core.Scope) core.Node {
			nav := base.NewNav(cx, "home", "detail")
			return nav.Stack(core.Style{FlexGrow: 1}, func(cx *core.Scope, route string, _ func() int) base.NavPage {
				return NavPage(cx, base.NavPage{Title: "Detail " + route, TestID: "page-" + route},
					NavHeader{TestID: "header", OnBack: func() { popped = true }, BackLabel: "Back"})
			})
		})
		header := app.Find("page-detail >> header")
		if header.Style.Background != p.Surface {
			t.Fatalf("%v: header = %x, want the surface", e.OS, header.Style.Background)
		}
		if e.OS != core.OSMacOS && header.Frame.Origin.Y != 47 {
			t.Fatalf("%v: header top = %v, want below the status bar", e.OS, header.Frame.Origin.Y)
		}
		app.Find("Detail detail")
		app.Press("role:button Back")
		if !popped {
			t.Fatalf("%v: the back button did nothing", e.OS)
		}
	}
}

// TestNavInset leaves room for the header only where the content runs under
// it, and the page's content starts under it there.
func TestNavInset(t *testing.T) {
	phone := phoneEnv
	phone.HeaderHeight, phone.SafeArea = 44, core.Edges{Top: 47, Bottom: 34}
	android := phone
	android.OS = core.OSAndroid
	for _, c := range []struct {
		env   core.Environment
		inset core.Points // past the status bar, when above 0
		top   core.Points
	}{{phone, 1, 0}, {android, 0, 47 + 44}, {desktopEnv, 0, 44}} {
		var inset core.Points
		app, _ := start(t, c.env, func(cx *core.Scope) core.Node {
			nav := base.NewNav(cx, "home")
			return nav.Stack(core.Style{FlexGrow: 1}, func(cx *core.Scope, _ string, _ func() int) base.NavPage {
				return NavPage(cx, base.NavPage{Title: "Home", Children: []core.Node{core.Func(func(cx *core.Scope) core.Node {
					return core.View{TestID: "content", Style: core.Style{FlexGrow: 1}, StyleFunc: func() core.Style {
						inset = NavInset(cx)
						return core.Style{}
					}}
				})}}, NavHeader{})
			})
		})
		if (inset > 47) != (c.inset > 0) {
			t.Fatalf("%v: inset = %v, want %v", c.env.OS, inset, c.inset)
		}
		if got := app.Find("content").Frame.Origin.Y; got != c.top {
			t.Fatalf("%v: content top = %v, want %v", c.env.OS, got, c.top)
		}
	}
}
