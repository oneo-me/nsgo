package core

import "github.com/oneo-me/nsgo/internal/driver"

// The status bar's settings are layered as UIKit's view controllers
// decide it, after NSH's status_bar.rs: the app's default at the bottom,
// which Scope.SetStatusBar patches, then the StatusBar of each screen and
// modal in the tree, in the order they were mounted. The latest wins field
// by field, so a pushed screen or a presented modal is over the screens
// below it, and popping or removing it takes its layer away. The commit
// sends the merged result as ItemStatusBar when it changed.

// Toggle is an option a layer turns on or off, or leaves to the layers
// below.
type Toggle uint8

const (
	ToggleInherit Toggle = iota
	ToggleOn
	ToggleOff
)

func (t Toggle) over(below Toggle) Toggle {
	if t == ToggleInherit {
		return below
	}
	return t
}

// StatusBarSetting is one layer of the status bar's settings. Fields left
// at their zero values keep the layers' below.
type StatusBarSetting struct {
	Foreground StatusBarStyle
	Hidden     Toggle
	// Animated animates the change to this layer's settings.
	Animated Toggle
}

func (s StatusBarSetting) over(below StatusBarSetting) StatusBarSetting {
	if s.Foreground == StatusBarInherit {
		s.Foreground = below.Foreground
	}
	s.Hidden = s.Hidden.over(below.Hidden)
	s.Animated = s.Animated.over(below.Animated)
	return s
}

func (s StatusBarSetting) config() driver.StatusBarConfig {
	c := driver.StatusBarConfig{Style: s.Foreground, Hidden: s.Hidden == ToggleOn, Animated: s.Animated == ToggleOn}
	if c.Style == StatusBarInherit {
		c.Style = StatusBarAuto
	}
	return c
}

type statusBars struct {
	base    StatusBarSetting
	layers  []*statusLayer
	shown   driver.StatusBarConfig // what the platform shows; Auto before any item
	started bool
}

// statusLayer is the setting of a screen or modal.
type statusLayer struct {
	n       *node
	setting StatusBarSetting
}

// SetStatusBar patches the app's default status bar, under the screens'
// and modals' settings, with the fields patch sets. Desktops and the web
// have no status bar.
func (cx *Scope) SetStatusBar(patch StatusBarSetting) {
	if t := cx.tree(); t != nil && !cx.denied(AccessRequest{Permission: AccessStatusBar}) {
		t.status.base = patch.over(t.status.base)
		t.changed = true
	}
}

// bindStatusBar gives n a layer of status bar settings: the static one,
// or fn's while it changes.
func (t *Tree) bindStatusBar(cx *Scope, n *node, s StatusBarSetting, fn func() StatusBarSetting) {
	if s == (StatusBarSetting{}) && fn == nil || cx.denied(AccessRequest{Permission: AccessStatusBar}) {
		return
	}
	l := &statusLayer{n: n, setting: s}
	t.status.layers = append(t.status.layers, l)
	t.changed = true
	if fn != nil {
		cx.renderEffect(func() {
			l.setting = fn()
			t.changed = true
		})
	}
}

// layered reports whether n's layer counts: it is in the tree, and a
// screen has not been popped nor a sheet swiped away.
func (n *node) layered() bool {
	if n.deleted {
		return false
	}
	if n.screen != nil && n.screen.popped {
		return false
	}
	return n.hoist == nil || n.hoist.modal == nil || !n.hoist.dismissed
}

// emitStatusBar sends the merged settings when they changed.
func (t *Tree) emitStatusBar() {
	st := &t.status
	merged := st.base
	live := st.layers[:0]
	for _, l := range st.layers {
		if l.n.deleted {
			continue
		}
		live = append(live, l)
		if l.n.layered() {
			merged = l.setting.over(merged)
		}
	}
	clear(st.layers[len(live):])
	st.layers = live
	c := merged.config()
	if c == st.shown || (!st.started && c.Style == StatusBarAuto && !c.Hidden) {
		return
	}
	st.shown, st.started = c, true
	t.commands = append(t.commands, driver.MountItem{Kind: driver.ItemStatusBar, StatusBar: &c})
}
