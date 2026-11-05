package terminal

import (
	"strings"
	"time"

	"github.com/oneo-me/nsgo/base/terminal/internal/vt"
	"github.com/oneo-me/nsgo/base/terminal/locales"
	"github.com/oneo-me/nsgo/core"
)

// Ref reaches a View's selection and clipboard, for the app's menus: a
// base.ContextMenu around the view opens at the pointer on desktops and,
// after a long press, as a sheet on touch screens, where the press first
// selects the word under the finger.
type Ref struct{ v *view }

// Selected reports whether text is selected; it is a signal, so a menu
// item's Disabled follows it.
func (r *Ref) Selected() bool { return r.v != nil && r.v.selected.Get() }

// Copy puts the selection on the clipboard.
func (r *Ref) Copy() {
	if r.v != nil {
		r.v.copy()
	}
}

// Paste pastes what the clipboard holds, bracketed as the program asks.
func (r *Ref) Paste() {
	if r.v != nil {
		r.v.paste()
	}
}

// SelectAll selects the scrollback and the screen.
func (r *Ref) SelectAll() {
	if r.v != nil {
		r.v.selectAll()
	}
}

// ClearSelection lets go of the selection.
func (r *Ref) ClearSelection() {
	if v := r.v; v != nil && v.t != nil {
		v.t.mu.Lock()
		if v.t.term != nil {
			v.t.term.SetSelection(nil)
		}
		v.t.mu.Unlock()
		v.t.redraw()
	}
}

// Focus gives the view the keyboard focus, which shows a software
// keyboard.
func (r *Ref) Focus() {
	if r.v != nil {
		r.v.sinkRef.Focus()
	}
}

// DefaultKeyBar is the keys the key bar shows when View.KeyBarKeys is
// nil: those a software keyboard lacks, Control and Alt applying to the
// next key.
var DefaultKeyBar = []string{"escape", "tab", "ctrl", "alt", "left", "up", "down", "right", "|", "~", "/", "-"}

// keepKeyboard keeps a software keyboard showing through taps on the
// view's scroll views, which by default dismiss it: the key bar's keys
// would go before their press ended.
var keepKeyboard = core.ScrollBehavior{KeyboardShouldPersistTaps: core.KeyboardTapsAlways}

// keyBar is the bar of keys over a software keyboard, while the view has
// the focus on a touch screen.
func (v *view) keyBar() core.Node {
	p := &v.p
	names := p.KeyBarKeys
	if names == nil {
		names = DefaultKeyBar
	}
	return core.Show{When: func() bool {
		return !p.NoKeyBar && v.focused.Get() && !v.cx.Env().Desktop()
	}, Then: func(cx *core.Scope) core.Node {
		m := locales.For(cx)
		var keys []core.Node
		for _, name := range names {
			text, label := keyCap(m, name)
			on := func() bool { return false }
			switch name {
			case "ctrl":
				on = v.ctrlOn.Get
			case "alt":
				on = v.altOn.Get
			}
			keys = append(keys, core.View{
				Role: core.RoleButton, Label: label, NoFocus: true, TestID: v.testID("key-" + name),
				ClassFunc: func() string {
					if on() {
						return strings.TrimSpace(p.KeyClass + " " + p.KeyOnClass)
					}
					return p.KeyClass
				},
				StyleFunc: func() core.Style {
					s := merge(p.KeyStyle, p.KeyStyleFunc)
					if on() {
						s = s.Merge(merge(p.KeyOnStyle, p.KeyOnStyleFunc))
					}
					return s
				},
				OnPress:  func(core.PressEvent) { v.barKey(name) },
				Children: []core.Node{core.Text{Content: text}},
			})
		}
		return core.Scroll{Horizontal: true, HideIndicator: true, Behavior: keepKeyboard, Class: p.KeyBarClass, Style: p.KeyBarStyle, StyleFunc: p.KeyBarStyleFunc,
			Children: []core.Node{core.View{Style: core.Style{Direction: core.Row}, Children: keys}}}
	}}
}

// testID is a part's test ID: the view's, a dash and the part's, none
// without the view's.
func (v *view) testID(part string) string {
	if v.p.TestID == "" {
		return ""
	}
	return v.p.TestID + "-" + part
}

// merge is a part's style with its StyleFunc's over it.
func merge(s core.Style, fn func() core.Style) core.Style {
	if fn == nil {
		return s
	}
	return s.Merge(fn())
}

// keyCap is what a key of the bar shows and says.
func keyCap(m locales.Messages, name string) (text, label string) {
	switch name {
	case "escape":
		return m.KeyEscape(), m.KeyEscape()
	case "tab":
		return m.KeyTab(), m.KeyTab()
	case "ctrl":
		return m.KeyControl(), m.KeyControl()
	case "alt":
		return m.KeyAlt(), m.KeyAlt()
	case "left":
		return "←", m.KeyLeft()
	case "right":
		return "→", m.KeyRight()
	case "up":
		return "↑", m.KeyUp()
	case "down":
		return "↓", m.KeyDown()
	case "home":
		return m.KeyHome(), m.KeyHome()
	case "end":
		return m.KeyEnd(), m.KeyEnd()
	case "pageup":
		return "⇞", m.KeyPageUp()
	case "pagedown":
		return "⇟", m.KeyPageDown()
	}
	return name, name
}

// barKey presses a key of the bar: Control and Alt latch for the next
// key, which takes them; a character types as if typed.
func (v *view) barKey(name string) {
	switch name {
	case "ctrl":
		v.ctrlOn.Set(!v.ctrlOn.GetUntracked())
		return
	case "alt":
		v.altOn.Set(!v.altOn.GetUntracked())
		return
	}
	if _, ok := keys[name]; ok && len(name) > 1 {
		v.key(v.latched() + name)
		return
	}
	v.typed(name)
}

// latched returns the modifiers Control and Alt latched for the next key,
// as core names them, and lets go of them.
func (v *view) latched() string {
	var mods string
	if v.ctrlOn.GetUntracked() {
		mods += "ctrl+"
		v.ctrlOn.Set(false)
	}
	if v.altOn.GetUntracked() {
		mods += "alt+"
		v.altOn.Set(false)
	}
	return mods
}

// longPress selects the word under a finger, as a double click does.
func (v *view) longPress(e core.PressEvent) {
	t := v.t
	if t == nil || v.cx.Env().Desktop() {
		return
	}
	t.mu.Lock()
	defer t.redraw()
	defer t.mu.Unlock()
	if t.term == nil || !v.ensureGesture() {
		return
	}
	col, row, px, py := v.cellAt(e.Position)
	ref, ok := t.term.CellAt(col, row)
	if !ok {
		return
	}
	now := uint64(time.Now().UnixNano())
	v.gesture.Reset(t.term)
	v.gesture.Press(t.term, ref, px, py, now)
	if s, ok := v.gesture.Press(t.term, ref, px, py, now+1); ok {
		t.term.SetSelection(&s)
	}
	v.gesture.Release(t.term, &ref)
}

// ensureGesture makes the selection gesture; t.mu is held.
func (v *view) ensureGesture() bool {
	if v.gesture == nil {
		g, err := vt.NewGesture(uint64(500*time.Millisecond), 5*float64(v.ratio))
		if err != nil {
			return false
		}
		v.gesture = g
	}
	return true
}

// paste pastes the clipboard's text.
func (v *view) paste() {
	v.cx.ReadClipboard(func(text string) {
		if text != "" && v.t != nil {
			v.t.Paste(text)
		}
	})
}

// keyboardOverlap is how far a software keyboard covers the view's bottom,
// which the grid leaves to it.
func (v *view) keyboardOverlap() core.Points {
	env := v.cx.Env()
	kb := env.KeyboardHeight()
	b := v.bounds.Get()
	if kb <= 0 || b.Size.Height <= 0 {
		return 0
	}
	bottom := b.Origin.Y + b.Size.Height
	return min(max(bottom-(env.Window().Height-kb), 0), b.Size.Height)
}
