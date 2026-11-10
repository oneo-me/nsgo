package core

import "github.com/oneo-me/nsgo/internal/driver"

// InputAccessoryView is the core's part of mobile.InputAccessoryView,
// which apps use: a bar iOS and Android show above the software keyboard,
// as React Native's.
type InputAccessoryView struct {
	// ID is what inputs name in InputAccessoryViewID.
	ID        string
	Style     Style
	StyleFunc func() Style
	Children  []Node
	TestID    string
}

// Render implements Node.
func (a InputAccessoryView) Render(*Scope) Node { return a }

func (a InputAccessoryView) mountIn(t *Tree, cx *Scope, parent *node) {
	t.mountAccessory(cx, parent, a)
}

type accessoryNode struct {
	name  string
	width Points // the surface's, which it takes
}

// mountAccessory builds an accessory where it is written, for its owner,
// context and inherited text, but no view holds its native view: inputs
// hand it to the keyboard.
func (t *Tree) mountAccessory(cx *Scope, parent *node, a InputAccessoryView) {
	n := t.newNode(driver.KindView, parent, cx)
	n.accessory = &accessoryNode{name: a.ID}
	t.bindStyle(cx, n, a.Style, a.StyleFunc)
	n.props = driver.ViewProps{TestID: a.TestID}
	t.accessories = append(t.accessories, n)
	if p := n.parent; p != nil {
		t.childrenChanged(p)
	}
	t.markAll() // the inputs naming it
	for _, c := range a.Children {
		t.mount(cx, n, c)
	}
}

// accessoryFor returns the view of the live accessory named id, or 0.
func (t *Tree) accessoryFor(id string) NodeID {
	if id == "" {
		return 0
	}
	for _, n := range t.accessories {
		if !n.deleted && n.accessory.name == id {
			return n.id
		}
	}
	return 0
}

// layoutAccessories lays each accessory out as a root as wide as the
// surface and as tall as its content.
func (t *Tree) layoutAccessories() {
	live := t.accessories[:0]
	for _, n := range t.accessories {
		if n.deleted {
			continue
		}
		live = append(live, n)
		if a := n.accessory; a.width != t.size.Width {
			a.width = t.size.Width
			n.lstyleOK = false
			t.markLayout(n)
		}
		if !n.layoutDirty {
			continue
		}
		t.layoutRoot(n, Size{Width: t.size.Width, Height: attachmentSpace}, false)
		l := &n.lstate.Rounded
		t.setFrame(n, Bounds{Size: Size{Width: t.size.Width, Height: Points(l.Size.Height)}})
		t.syncFrames(n)
	}
	clear(t.accessories[len(live):])
	t.accessories = live
}

// emitAccessories sends the accessories' views before the tree's, so the
// inputs naming them find them.
func (t *Tree) emitAccessories() {
	for _, n := range t.accessories {
		if !n.deleted {
			t.emit(n)
		}
	}
}
