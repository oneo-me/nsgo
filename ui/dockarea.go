package ui

import (
	"github.com/oneo-me/nsgo/base"
	"github.com/oneo-me/nsgo/base/dock"
	"github.com/oneo-me/nsgo/core"
	"github.com/oneo-me/nsgo/icons/lucide"
)

// DockArea is dock.Area in ui's look: tab bars on the inset
// color with the selected tab on the surface, hairline handles, the
// selection where a dragged tab would land, and drawers and a sheet as
// the touch-screen popups are. A tab shows its key unless RenderTab says
// else; DockTabLabel is the label with a close button.
type DockArea dock.Area

// Render implements core.Node.
func (d DockArea) Render(cx *core.Scope) core.Node {
	colors := ColorsOf(cx)
	out := dock.Area(d)
	out.StyleFunc = over(Filled(colors.Background), d.Style, d.StyleFunc)
	out.GroupStyleFunc = over(Filled(colors.Surface), d.GroupStyle, d.GroupStyleFunc)
	out.TabBarStyle = core.Style{AlignItems: core.AlignStretch, MinHeight: core.Pt(ButtonHeight + 4), Borders: core.Edges{Bottom: 1}}.Merge(d.TabBarStyle)
	out.TabBarStyleFunc = over(func() core.Style {
		return core.Style{Background: colors.Inset(), BorderColors: core.EdgeColors{Bottom: colors.Line()}}
	}, d.TabBarStyle, d.TabBarStyleFunc)
	out.TabStyle = TypeLabel.Merge(core.Style{Direction: core.Row, AlignItems: core.AlignCenter, Gap: 4, Padding: core.Symmetric(0, 12),
		Borders: core.Edges{Top: 2, Right: 1}, Margin: core.Edges{Bottom: -1}}).Merge(d.TabStyle)
	out.TabStyleFunc = over(func() core.Style {
		c := colors.Now()
		return core.Style{Color: c.Muted, BorderColors: core.EdgeColors{Top: transparent, Right: c.Line},
			Hover:    &core.Style{Color: c.Text, Background: c.Hover},
			Selected: &core.Style{Color: c.Text, Background: c.Surface, BorderColors: core.EdgeColors{Top: c.Accent, Right: c.Line}}}
	}, d.TabStyle, d.TabStyleFunc)
	out.ContentStyleFunc = over(Filled(colors.Surface), d.ContentStyle, d.ContentStyleFunc)
	out.DropIndicatorStyle = core.Style{BorderWidth: 2, CornerRadius: RadiusControl}.Merge(d.DropIndicatorStyle)
	out.DropIndicatorStyleFunc = over(func() core.Style {
		return core.Style{Background: colors.Now().Selection, BorderColor: colors.Info()}
	}, d.DropIndicatorStyle, d.DropIndicatorStyleFunc)
	out.HandleStyleFunc = over(HandleLine(cx), d.HandleStyle, d.HandleStyleFunc)
	out.HandleThumbStyle = HandleThumb(cx).Merge(d.HandleThumbStyle)
	out.HandleThumbStyleFunc = over(HandleThumbColor(cx), d.HandleThumbStyle, d.HandleThumbStyleFunc)
	out.DockStyleFunc = over(Filled(colors.Surface), d.DockStyle, d.DockStyleFunc)
	out.DrawerStyleFunc = over(func() core.Style { return sheetPanel(cx, core.PlaceLeft) }, d.DrawerStyle, d.DrawerStyleFunc)
	out.SheetStyle = core.Style{Height: core.Pct(60)}.Merge(d.SheetStyle)
	out.SheetStyleFunc = over(func() core.Style { return sheetPanel(cx, core.PlaceBottom) }, d.SheetStyle, d.SheetStyleFunc)
	out.BackdropStyle = scrim.Merge(d.BackdropStyle)
	if out.RenderTab == nil {
		out.RenderTab = func(cx *core.Scope, tab dock.Tab) core.Node { return DockTabLabel{Tab: tab, Text: tab.Key} }
	}
	return out
}

// DockTabLabel is what a DockArea's tab shows: an icon, the text, and
// while its panel may close, a button named CloseLabel that closes it.
type DockTabLabel struct {
	Tab        dock.Tab
	Text       string
	Icon       func() *core.SVG
	CloseLabel string
}

// Render implements core.Node.
func (l DockTabLabel) Render(cx *core.Scope) core.Node {
	colors := ColorsOf(cx)
	var out []core.Node
	if l.Icon != nil {
		out = append(out, Icon(l.Icon, iconSize, Inherited))
	}
	out = append(out, line(l.Text, core.Style{}, 1))
	if l.CloseLabel != "" && l.Tab.Closable != nil {
		out = append(out, core.Show{When: l.Tab.Closable, Then: func(cx *core.Scope) core.Node {
			return base.Button{Label: l.CloseLabel, OnPress: func(core.PressEvent) { l.Tab.Close() },
				Style: core.Style{Width: core.Pt(iconSize + 2), Height: core.Pt(iconSize + 2), AlignItems: core.AlignCenter,
					JustifyContent: core.JustifyCenter, CornerRadius: RadiusSmall, Margin: core.Edges{Right: -4}},
				StyleFunc: func() core.Style {
					return core.Style{Color: colors.Muted(), Hover: &core.Style{Background: colors.Pressed(), Color: colors.Text()}}
				},
				Children: []core.Node{Icon(lucide.X, iconSize-4, Inherited)}}
		}})
	}
	return core.View{Style: core.Style{Direction: core.Row, AlignItems: core.AlignCenter, Gap: 6}, Children: out}
}
