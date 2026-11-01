package core

import "github.com/oneo-me/nsgo/internal/driver"

// Desktop windows with the platform's native chrome and their menus: the
// core's part of package desktop, which apps use. Platforms without
// desktop windows ignore all of it.

func (cx *Scope) window(c WindowCommand) {
	if t := cx.rt.tree; t != nil {
		t.commands = append(t.commands, driver.MountItem{Kind: driver.ItemWindow, Window: &c})
		t.changed = true
	}
}

// SetTitleBar is the core's part of desktop.SetTitleBar, which apps use.
func (cx *Scope) SetTitleBar(style TitleBarStyle, height Points) {
	cx.window(WindowCommand{Action: WindowSetTitleBar, TitleBar: style, Height: min(max(height, 0), 200)})
}

// MinimizeWindow is the core's part of desktop.MinimizeWindow, which apps use.
func (cx *Scope) MinimizeWindow() { cx.window(WindowCommand{Action: WindowMinimize}) }

// ToggleMaximize is the core's part of desktop.ToggleMaximize, which apps use.
func (cx *Scope) ToggleMaximize() { cx.window(WindowCommand{Action: WindowToggleMaximize}) }

// ToggleFullScreen is the core's part of desktop.ToggleFullScreen, which apps use.
func (cx *Scope) ToggleFullScreen() { cx.window(WindowCommand{Action: WindowToggleFullScreen}) }

// CloseWindow is the core's part of desktop.CloseWindow, which apps use.
func (cx *Scope) CloseWindow() { cx.window(WindowCommand{Action: WindowClose}) }

// SetWindowFrame is the core's part of desktop.SetWindowFrame, which apps use.
func (cx *Scope) SetWindowFrame(f WindowFrame) {
	f.Width, f.Height = max(f.Width, 0), max(f.Height, 0)
	cx.window(WindowCommand{Action: WindowSetFrame, Frame: f})
}

// SetWindowLevel is the core's part of desktop.SetWindowLevel, which apps use.
func (cx *Scope) SetWindowLevel(level WindowLevel) {
	cx.window(WindowCommand{Action: WindowSetLevel, Level: level})
}

// OrderFront is the core's part of desktop.OrderFront, which apps use.
func (cx *Scope) OrderFront() { cx.window(WindowCommand{Action: WindowOrderFront}) }

// SetMenuBar is the core's part of desktop.SetMenuBar, which apps use.
func (cx *Scope) SetMenuBar(mode MenuBarMode) {
	cx.window(WindowCommand{Action: WindowSetMenuBar, MenuBar: mode})
}

// SetWindowControls is the core's part of desktop.SetWindowControls, which apps use.
func (cx *Scope) SetWindowControls(c WindowControls) {
	cx.window(WindowCommand{Action: WindowSetControls, Controls: c})
}

// SetWindowMaterial is the core's part of desktop.SetWindowMaterial, which apps use.
func (cx *Scope) SetWindowMaterial(m WindowMaterial) {
	cx.window(WindowCommand{Action: WindowSetMaterial, Material: m})
}

// MenuItem is one item of a menu: an action, a submenu when it has
// Items, or a line between groups when Separator. Apps name it
// desktop.MenuItem.
type MenuItem struct {
	Label string
	// Shortcut is in OnKey's form, such as "mod+n".
	Shortcut  string
	Role      MenuRole
	Disabled  bool
	Checkable bool
	Checked   bool
	Separator bool
	Items     []MenuItem
	// When says whether the shortcut chooses the item while a text input
	// or a web view has the keyboard focus.
	When MenuWhen
	// OnSelect runs when the item is chosen, by a click or its shortcut.
	OnSelect func()
}

// MenuWhen says when a menu item's shortcut chooses it while a text
// input or a web view has the keyboard focus.
type MenuWhen uint8

const (
	// MenuAlways chooses it there too.
	MenuAlways MenuWhen = iota
	// MenuAuto leaves the input the keys it uses itself (EditsText).
	MenuAuto
	// MenuNotEditing never chooses it there.
	MenuNotEditing
)

type menuAction struct {
	owner *Scope
	fn    func()
}

// SetApplicationMenu is the core's part of desktop.SetApplicationMenu, which apps use.
func (cx *Scope) SetApplicationMenu(items []MenuItem) {
	t := cx.rt.tree
	if t == nil {
		return
	}
	t.menuGen++
	gen := t.menuGen
	t.menuActions = map[uint32]menuAction{}
	owner := cx.current()
	var entries []MenuEntry
	if items != nil {
		entries = t.menuEntries(items, owner, t.menuActions)
		owner.addCleanup(func() {
			if t.menuGen == gen {
				t.menuActions = nil
				t.commands = append(t.commands, driver.MountItem{Kind: driver.ItemMenu})
				t.changed = true
			}
		})
	}
	t.commands = append(t.commands, driver.MountItem{Kind: driver.ItemMenu, Menu: entries})
	t.changed = true
}

// popupMenu is the context menu PopupMenu shows: one at a time.
type popupMenu struct {
	id      uint32
	actions map[uint32]menuAction
	owner   *Scope
	closed  func()
}

// PopupMenu is the core's part of desktop.PopupMenu, which apps use.
func (cx *Scope) PopupMenu(items []MenuItem, at Point, closed func()) {
	t := cx.rt.tree
	if t == nil {
		return
	}
	owner := cx.current()
	if os := cx.Env().os.GetUntracked(); os != OSMacOS && os != OSWindows && os != OSLinux {
		if closed != nil {
			t.within(owner, closed)
		}
		return
	}
	t.popupClosed(t.popup.id)
	t.alertID++
	// The menu before it can no longer report a choice.
	t.popup = popupMenu{id: t.alertID, actions: map[uint32]menuAction{}, owner: owner, closed: closed}
	entries := t.menuEntries(items, owner, t.popup.actions)
	t.commands = append(t.commands, driver.MountItem{Kind: driver.ItemPopupMenu, Menu: entries, Point: at,
		Count: t.popup.id})
	t.changed = true
}

// popupClosed runs the closed callback of context menu id once it
// closed. Its items stay choosable until another menu replaces it, so a
// platform may report the item chosen after the menu closed.
func (t *Tree) popupClosed(id uint32) {
	p := t.popup
	if id == 0 || p.id != id {
		return
	}
	t.popup.id, t.popup.closed = 0, nil
	if p.closed != nil {
		owner := p.owner
		if owner.disposed {
			owner = t.root
		}
		t.within(owner, p.closed)
	}
}

func (t *Tree) menuEntries(items []MenuItem, owner *Scope, actions map[uint32]menuAction) []MenuEntry {
	out := make([]MenuEntry, 0, len(items))
	for _, it := range items {
		e := MenuEntry{Label: it.Label, Shortcut: it.Shortcut, Role: it.Role, Disabled: it.Disabled,
			Checkable: it.Checkable || it.Checked, Checked: it.Checked, Separator: it.Separator}
		if e.Label == "" {
			e.Label = it.Role.Label()
		}
		if e.Shortcut != "" {
			e.Shortcut = driver.NormalizeKey(e.Shortcut)
		}
		shortcut := e.Shortcut
		if shortcut == "" && it.Role != MenuRoleNone {
			shortcut = driver.NormalizeKey(it.Role.Shortcut(false))
		}
		e.NotInText = it.When == MenuNotEditing || it.When == MenuAuto && shortcut != "" && EditsText(shortcut)
		if len(it.Items) > 0 {
			e.Items = t.menuEntries(it.Items, owner, actions)
		}
		if it.OnSelect != nil {
			t.menuID++
			e.ID = t.menuID
			actions[e.ID] = menuAction{owner, it.OnSelect}
		}
		out = append(out, e)
	}
	return out
}

func (t *Tree) chooseMenu(id uint32) {
	a, ok := t.menuActions[id]
	if !ok {
		a, ok = t.popup.actions[id]
	}
	if ok && !a.owner.disposed {
		t.within(a.owner, a.fn)
	}
}
