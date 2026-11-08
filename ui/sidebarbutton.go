package ui

import (
	"github.com/oneo-me/nsgo/base"
	"github.com/oneo-me/nsgo/core"
)

// SidebarButtonVariant is a SidebarButton's look.
type SidebarButtonVariant uint8

const (
	// SidebarButtonPlain is a Sidebar item's: no fill but while Selected.
	// It is the default.
	SidebarButtonPlain SidebarButtonVariant = iota
	// SidebarButtonField is a search field's: always a quiet fill, deeper
	// while pressed, its text muted, for what opens a search.
	SidebarButtonField
)

// SidebarButton is a row in a sidebar's place, above or beside a Sidebar's
// items: an Icon and Text, with the Shortcut's keys at its end. It is a
// stop of Tab of its own.
type SidebarButton struct {
	Text string
	Icon func() *core.SVG
	// Shortcut is the keys that press it, such as "mod+k", shown at its
	// end but on touch screens. The app binds them.
	Shortcut string
	Variant  SidebarButtonVariant
	// Selected fills it as a Sidebar's selected item while it returns
	// true.
	Selected func() bool
	OnPress  func()
	TestID   string
}

// Render implements core.Node.
func (b SidebarButton) Render(cx *core.Scope) core.Node {
	colors := ColorsOf(cx)
	ink := colors.Text
	if b.Variant == SidebarButtonField {
		ink = colors.Muted
	}
	label := Text(b.Text, 13, Regular, ink)
	label.MaxLines, label.Style.FlexGrow, label.Style.FlexShrink = 1, 1, 1
	var children []core.Node
	if b.Icon != nil {
		children = append(children, Icon(b.Icon, 16, colors.Muted))
	}
	children = append(children, label)
	if b.Shortcut != "" && !Touch(cx) {
		children = append(children, core.Text{Content: cx.KeyLabel(b.Shortcut), MaxLines: 1,
			Style: TypeCaption.Merge(core.Style{FlexShrink: 0}), StyleFunc: Colored(colors.Muted)})
	}
	return base.Button{Label: b.Text, Selected: b.Selected, TestID: b.TestID,
		OnPress: func(core.PressEvent) {
			if b.OnPress != nil {
				b.OnPress()
			}
		},
		Style: core.Style{Direction: core.Row, AlignItems: core.AlignCenter, Gap: 8, MinHeight: core.Pt(32), MinWidth: core.Pt(0), FlexShrink: 0,
			Padding: core.Symmetric(0, 8), CornerRadius: 8, Cursor: core.CursorDefault},
		StyleFunc: func() core.Style {
			s := core.Style{Selected: &core.Style{Background: Alpha(colors.Text(), 0.1)}, Focus: FocusRing(cx, -2)}
			if b.Variant == SidebarButtonField {
				s.Background, s.Active = Alpha(colors.Text(), 0.06), &core.Style{Background: Alpha(colors.Text(), 0.1)}
			}
			return s
		},
		Children: children}
}
