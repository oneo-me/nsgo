package core

import (
	"strings"

	"github.com/oneo-me/nsgo/internal/driver"
)

// Importance is importantForAccessibility, Android's name for Accessible
// and AccessibilityElementsHidden in one prop.
type Importance uint8

const (
	// ImportantAuto leaves it to the view's other props.
	ImportantAuto Importance = iota
	// ImportantYes is Accessible.
	ImportantYes
	// ImportantNo makes the view no element; its descendants still are.
	ImportantNo
	// ImportantNoHideDescendants hides the view and its subtree.
	ImportantNoHideDescendants
)

// SetAccessibilityFocus moves the screen reader's focus to the element,
// as after a change the user should hear about first.
func (r *Ref) SetAccessibilityFocus() {
	if r.n != nil && !r.n.deleted {
		r.t.command(driver.ItemAccessibilityFocus, r.n)
	}
}

// a11y is what an element's accessibility props are bound from.
type a11y struct {
	labelFunc  func() string
	value      func() AccessibilityValue
	expanded   func() bool
	expandable func() bool
	hidden     func() bool
	actions    []AccessibilityAction
	accessible bool
	important  Importance
}

// bindAccessibility sets n's accessibility props from a and binds those
// that change.
func (t *Tree) bindAccessibility(cx *Scope, n *node, a a11y) {
	p := &n.props
	switch {
	case a.important == ImportantNo || a.important == ImportantNoHideDescendants:
		p.Accessible = driver.AccessibleNo
	case a.accessible || a.important == ImportantYes:
		p.Accessible = AccessibleYes
	}
	if len(a.actions) > 0 {
		p.Actions = &a.actions
	}
	if a.labelFunc != nil {
		cx.renderEffect(func() {
			p.Label = a.labelFunc()
			t.touch(n)
		})
	}
	if a.value != nil {
		cx.renderEffect(func() {
			p.Value = a.value()
			t.touch(n)
		})
	}
	if a.expanded != nil {
		p.Expandable = true
		cx.renderEffect(func() {
			if a.expandable != nil {
				p.Expandable = a.expandable()
			}
			p.Expanded = a.expanded()
			t.touch(n)
			t.restyleExpanded(n)
		})
	}
	always := a.important == ImportantNoHideDescendants
	p.ElementsHidden = always
	if a.hidden != nil && !always {
		cx.renderEffect(func() {
			p.ElementsHidden = a.hidden()
			t.touch(n)
		})
	}
}

// sentProps returns the props n sends: a group without a label of its
// own reads the text inside it.
func (n *node) sentProps() driver.ViewProps {
	p := n.props
	p.Appearance = n.appearance
	if n.input != nil && n.input.config.Disabled {
		p.Disabled = true
	}
	if p.Accessible == AccessibleYes && p.Label == "" && n.text == nil {
		var b strings.Builder
		n.appendText(&b)
		p.Label = b.String()
	}
	return p
}

// appendText appends the text of the texts inside n that assistive
// technology would read, separated by spaces.
func (n *node) appendText(b *strings.Builder) {
	for _, c := range n.children {
		if c.deleted || c.style.Display == DisplayNone || c.props.ElementsHidden {
			continue
		}
		var s string
		switch {
		case c.props.Label != "":
			s = c.props.Label
		case c.text != nil:
			s = strings.ReplaceAll(c.text.text.Text, string(driver.AttachmentRune), "")
		}
		if s = strings.TrimSpace(s); s != "" {
			if b.Len() > 0 {
				b.WriteByte(' ')
			}
			b.WriteString(s)
			continue
		}
		c.appendText(b)
	}
}
