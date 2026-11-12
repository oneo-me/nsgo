package base

import (
	"github.com/nsgo-dev/nsgo/base/locales"
	"github.com/nsgo-dev/nsgo/core"
)

// ColorChoice is a row of a color's menu.
type ColorChoice struct {
	Text string
	// Checked marks the one in use, nil in a menu of actions.
	Checked  func() bool
	OnSelect func()
}

// ColorMenuParts are what a color's menu draws.
type ColorMenuParts struct {
	// Label names the menu; Current is what its trigger says, nil for an
	// action's.
	Label   string
	Current func() string
	Choices []ColorChoice
}

// drawnColorMenu is parts in draw's menu, or a plain one.
func drawnColorMenu(draw func(ColorMenuParts) core.Node, parts ColorMenuParts) core.Node {
	if draw != nil {
		return draw(parts)
	}
	items := make([]core.Node, len(parts.Choices))
	for i, c := range parts.Choices {
		items[i] = MenuItem{Label: c.Text, Checked: c.Checked, OnSelect: c.OnSelect, Children: []core.Node{core.Text{Content: c.Text}}}
	}
	face := parts.Current
	if face == nil {
		face = func() string { return parts.Label }
	}
	return Menu{Label: parts.Label, Trigger: core.Text{ContentFunc: face}, Children: items}
}

// notationName is n's name, with the alpha the picker shows.
func (s *colorState) notationName(n colorNotation) string {
	if s.noAlpha {
		return n.name
	}
	return n.name + "A"
}

// ColorNotationMenu chooses the notation ColorChannels spell the color in,
// which leaves the color as it is. Draw draws the menu, a plain one when
// nil.
type ColorNotationMenu struct {
	Draw func(ColorMenuParts) core.Node
}

// Render implements core.Node.
func (m ColorNotationMenu) Render(cx *core.Scope) core.Node {
	picker := colorStateOf(cx)
	choices := make([]ColorChoice, len(colorNotations))
	for i, n := range colorNotations {
		choices[i] = ColorChoice{Text: picker.notationName(n), Checked: func() bool { return picker.notation.Get() == i },
			OnSelect: func() { picker.notation.Set(i) }}
	}
	return drawnColorMenu(m.Draw, ColorMenuParts{Label: locales.ColorFormat(cx), Choices: choices,
		Current: func() string { return picker.notationName(colorNotations[picker.notation.Get()]) }})
}

// ColorCopyMenu copies the color in a notation or as hex. Draw draws the
// menu, a plain one when nil.
type ColorCopyMenu struct {
	Draw func(ColorMenuParts) core.Node
}

// Render implements core.Node.
func (m ColorCopyMenu) Render(cx *core.Scope) core.Node {
	picker := colorStateOf(cx)
	copier := func(f ColorFormat) func() {
		return func() { cx.SetClipboard(picker.now().format(f)) }
	}
	choices := make([]ColorChoice, 0, len(colorNotations)+1)
	for _, n := range colorNotations {
		choices = append(choices, ColorChoice{Text: picker.notationName(n), OnSelect: copier(n.format)})
	}
	choices = append(choices, ColorChoice{Text: "HEX", OnSelect: copier(ColorFormatHex)})
	return drawnColorMenu(m.Draw, ColorMenuParts{Label: locales.ColorCopy(cx), Choices: choices})
}
