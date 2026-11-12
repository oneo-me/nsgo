package desktop

import "github.com/nsgo-dev/nsgo/core"

type (
	// MenuItem is one item of a menu: an action, a submenu when it has
	// Items, or a line between groups when Separator. The application
	// menu's top-level items are the menu bar's menus; on macOS the first
	// is the app menu.
	MenuItem = core.MenuItem
	// MenuRole is a standard menu item the platform carries out, labelling
	// it and giving it its usual shortcut when the item has none of its own.
	MenuRole = core.MenuRole
)

const (
	MenuRoleNone         = core.MenuRoleNone
	MenuAbout            = core.MenuAbout
	MenuHide             = core.MenuHide
	MenuHideOthers       = core.MenuHideOthers
	MenuShowAll          = core.MenuShowAll
	MenuQuit             = core.MenuQuit
	MenuUndo             = core.MenuUndo
	MenuRedo             = core.MenuRedo
	MenuCut              = core.MenuCut
	MenuCopy             = core.MenuCopy
	MenuPaste            = core.MenuPaste
	MenuSelectAll        = core.MenuSelectAll
	MenuMinimize         = core.MenuMinimize
	MenuZoom             = core.MenuZoom
	MenuToggleFullScreen = core.MenuToggleFullScreen
	MenuClose            = core.MenuClose
)

// SetApplicationMenu replaces the application menu: macOS's menu bar, the
// window's menu bar on Windows and Linux. nil restores the platform's
// default. The items' handlers run under the current owner; when it goes,
// the default menu comes back.
func SetApplicationMenu(cx *core.Scope, items []MenuItem) { cx.SetApplicationMenu(items) }

// PopupMenu shows items as the platform's own context menu, its top left
// corner at at, in the points of the root view, as a press's or
// OnContextMenu's PagePosition. closed, when not nil, runs once it closes,
// after the OnSelect of an item chosen. A menu shown before closes first.
// Desktops show it; elsewhere closed runs at once.
func PopupMenu(cx *core.Scope, items []MenuItem, at core.Point, closed func()) {
	cx.PopupMenu(items, at, closed)
}
