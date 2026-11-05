package ui

import (
	"testing"

	"github.com/oneo-me/nsgo/base"
	"github.com/oneo-me/nsgo/core"
)

// TestNavPage gives a page without a header of its own a themed one with
// a back button, below the status bar where the system has a header.
func TestNavPage(t *testing.T) {
	phone := phoneEnv
	phone.HeaderHeight, phone.SafeArea = 44, core.Edges{Top: 47, Bottom: 34}
	for _, e := range []core.Environment{desktopEnv, phone} {
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
		if e.OS == core.OSIOS && header.Frame.Origin.Y != 47 {
			t.Fatalf("header top = %v, want below the status bar", header.Frame.Origin.Y)
		}
		app.Find("Detail detail")
		app.Press("role:button Back")
		if !popped {
			t.Fatalf("%v: the back button did nothing", e.OS)
		}
	}
}
