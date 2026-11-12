package base

import "github.com/nsgo-dev/nsgo/core"

// Presentation is how a Menu, a ContextMenu, a Select, a Combobox or a
// DatePicker shows what it opens. Its zero value, the popup, is their
// form on every device: base changes it by none, and the app chooses the
// sheet where it wants one, as on a touch screen.
type Presentation uint8

const (
	// PresentPopup opens a popup at the trigger, or at the pointer for a
	// context menu, never past the room on its side, its submenus
	// cascading beside it.
	PresentPopup Presentation = iota
	// PresentSheet opens a sheet at the bottom of the window over a
	// backdrop, as touch screens show menus: submenus replace the items,
	// and a search field waits for a tap before the keyboard comes.
	PresentSheet
)

// onSheetKey says whether the menu or the dropdown around a scope
// presents as a sheet.
var onSheetKey = core.NewKey("sheet", false)

// OnSheet reports whether cx renders inside a menu, or a Select's, a
// Combobox's or a DatePicker's dropdown, presented as a sheet, for a
// styled form's look: a sheet's rows are taller, for instance.
func OnSheet(cx *core.Scope) bool { return onSheetKey.Use(cx) }
