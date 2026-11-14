//go:build darwin && !ios

package macos

import (
	"slices"
	"testing"

	"github.com/ebitengine/purego/objc"

	"github.com/nsgo-dev/nsgo/internal/driver"
	"github.com/nsgo-dev/nsgo/internal/platform/apple"
)

// TestMenuSyncsInPlace checks that a menu set again keeps the items the
// menu bar shows, which setMainMenu: would redraw, and takes the new
// one's states, actions and items.
func TestMenuSyncsInPlace(t *testing.T) {
	onMain(func() { menuSyncsInPlace(t) })
}

func menuSyncsInPlace(t *testing.T) {
	loadAppKit()
	s := &surface{}
	s.ensureMenuTarget()
	defer delete(menuTargets, s.menuTarget)
	view := func(checked bool, first uint32, more ...driver.MenuEntry) []driver.MenuEntry {
		items := append([]driver.MenuEntry{
			{Label: "Sidebar", Checkable: true, Checked: checked, ID: first},
			{Separator: true},
			{Label: "Button", ID: first + 1},
		}, more...)
		return []driver.MenuEntry{{Label: "View", Items: items}}
	}
	bar := s.submenu("", view(true, 1))
	defer bar.Send(selRelease)
	holder := bar.Send(selItemAtIndex, 0)
	sub := holder.Send(sel("submenu"))
	sidebar := sub.Send(selItemAtIndex, 0)
	// An item AppKit adds itself, as it does Start Dictation to Edit.
	foreign := objc.ID(apple.Class("NSMenuItem")).Send(selAlloc).Send(sel("initWithTitle:action:keyEquivalent:"),
		apple.NSString("Dictation"), objc.SEL(0), apple.NSString(""))
	sub.Send(sel("addItem:"), foreign)
	foreign.Send(selRelease)

	next := append(view(false, 3, driver.MenuEntry{Label: "Link", ID: 5}),
		driver.MenuEntry{Label: "Help", Items: []driver.MenuEntry{{Label: "Docs", ID: 6}}})
	m := s.submenu("", next)
	syncMenu(bar, m)
	m.Send(selRelease)

	if got := bar.Send(selItemAtIndex, 0); got != holder || got.Send(sel("submenu")) != sub || sub.Send(selItemAtIndex, 0) != sidebar {
		t.Fatal("the menu set again replaced items it could keep")
	}
	if n := objc.Send[int](bar, sel("numberOfItems")); n != 2 {
		t.Fatalf("the bar has %d menus, want 2", n)
	}
	if state, tag := objc.Send[int](sidebar, sel("state")), objc.Send[int](sidebar, sel("tag")); state != 0 || tag != 3 {
		t.Errorf("the checked item has state %d and tag %d, want 0 and 3", state, tag)
	}
	var titles []string
	for i := range objc.Send[int](sub, sel("numberOfItems")) {
		titles = append(titles, apple.GoString(sub.Send(selItemAtIndex, i).Send(sel("title"))))
	}
	if want := []string{"Sidebar", "", "Button", "Link", "Dictation"}; !slices.Equal(titles, want) {
		t.Errorf("the View menu has %q, want %q", titles, want)
	}
	help := bar.Send(selItemAtIndex, 1).Send(sel("submenu"))
	if help == 0 || apple.GoString(help.Send(selItemAtIndex, 0).Send(sel("title"))) != "Docs" {
		t.Error("the added menu is missing its item")
	}
}
