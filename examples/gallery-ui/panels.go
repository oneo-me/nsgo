package ui

import (
	"time"

	"github.com/oneo-me/nsgo/base"
	"github.com/oneo-me/nsgo/core"
)

// HandleThumb is the thumb of a resize handle on phones, as
// gpui-component draws it: none at rest, so the hairlines stay quiet; a
// long solid pill while pressed or dragged. Elsewhere handles have none,
// the pointer's resize cursor showing where to drag.
func HandleThumb(cx *core.Scope) core.Style {
	if !phone(cx) {
		return core.Style{}
	}
	return core.Style{Height: core.Pt(44), Width: core.Pt(3), CornerRadius: 1.5, Opacity: core.Alpha(0.001),
		Transform:  core.Transform{ScaleY: 0.001},
		Transition: core.Transition{Duration: 150 * time.Millisecond, Easing: core.EaseOut},
		GroupStyle: &core.GroupStyle{
			Hover:  &core.Style{Opacity: core.Alpha(0.4), Transform: core.Transform{ScaleY: 0.45}},
			Active: &core.Style{Opacity: core.Alpha(0.9), Transform: core.Transform{ScaleY: 1}},
		}}
}

// HandleThumbColor is the thumb's color, for a ThumbStyleFunc: none
// where HandleThumb is none.
func HandleThumbColor(cx *core.Scope) func() core.Style {
	if !phone(cx) {
		return nil
	}
	return Filled(ColorsOf(cx).Muted)
}

// phone reports whether the device is a phone.
func phone(cx *core.Scope) bool { return core.Untracked(cx, cx.Env().Idiom) == core.IdiomPhone }

// HandleLine is a handle's line, which stays as it is. It is a StyleFunc.
func HandleLine(cx *core.Scope) func() core.Style { return Filled(ColorsOf(cx).Border) }

// The panels are base's types under these names: they take every field
// of base's, and a part style the app sets merges over the gallery's.

// ResizablePanelGroup is base.ResizablePanelGroup with the gallery's
// handles: a hairline, with a thumb on phones.
type ResizablePanelGroup base.ResizablePanelGroup

// Render implements core.Node.
func (g ResizablePanelGroup) Render(cx *core.Scope) core.Node {
	out := base.ResizablePanelGroup(g)
	out.HandleStyleFunc = over(HandleLine(cx), g.HandleStyle, g.HandleStyleFunc)
	out.HandleThumbStyle = HandleThumb(cx).Merge(g.HandleThumbStyle)
	out.HandleThumbStyleFunc = over(HandleThumbColor(cx), g.HandleThumbStyle, g.HandleThumbStyleFunc)
	return out
}

// ResizeHandle is base.ResizeHandle with the gallery's line.
type ResizeHandle base.ResizeHandle

// Render implements core.Node.
func (h ResizeHandle) Render(cx *core.Scope) core.Node {
	out := base.ResizeHandle(h)
	out.StyleFunc = over(HandleLine(cx), h.Style, h.StyleFunc)
	out.ThumbStyle = HandleThumb(cx).Merge(h.ThumbStyle)
	out.ThumbStyleFunc = over(HandleThumbColor(cx), h.ThumbStyle, h.ThumbStyleFunc)
	return out
}

// over is a part's style function: the gallery's, then the app's style
// and style function over it, as base merges a style function over a
// style. Either function may be nil.
func over(gallery func() core.Style, style core.Style, app func() core.Style) func() core.Style {
	if gallery == nil {
		return app
	}
	return func() core.Style {
		s := gallery().Merge(style)
		if app != nil {
			s = s.Merge(app())
		}
		return s
	}
}

// DockArea is base.DockArea in the gallery's look: tab bars on the inset
// color with the selected tab on the surface, hairline handles, the
// selection where a dragged tab would land, and drawers and a sheet as
// the touch-screen popups are. A tab shows its key unless RenderTab says
// else; DockTabLabel is the label with a close button.
type DockArea base.DockArea

// Render implements core.Node.
func (d DockArea) Render(cx *core.Scope) core.Node {
	colors := ColorsOf(cx)
	out := base.DockArea(d)
	out.StyleFunc = over(Filled(colors.Background), d.Style, d.StyleFunc)
	out.GroupStyleFunc = over(Filled(colors.Surface), d.GroupStyle, d.GroupStyleFunc)
	out.TabBarStyle = core.Style{AlignItems: core.AlignStretch, MinHeight: core.Pt(ButtonHeight + 4), Borders: core.Edges{Bottom: 1}}.Merge(d.TabBarStyle)
	out.TabBarStyleFunc = over(func() core.Style {
		return core.Style{Background: colors.Inset(), BorderColors: core.EdgeColors{Bottom: colors.Line()}}
	}, d.TabBarStyle, d.TabBarStyleFunc)
	out.TabStyle = typeLabel.Merge(core.Style{Direction: core.Row, AlignItems: core.AlignCenter, Gap: 4, Padding: core.Symmetric(0, 12),
		Cursor: core.CursorPointer, Borders: core.Edges{Top: 2, Right: 1}, Margin: core.Edges{Bottom: -1}}).Merge(d.TabStyle)
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
		out.RenderTab = func(cx *core.Scope, tab base.DockTab) core.Node { return DockTabLabel{Tab: tab, Text: tab.Key} }
	}
	return out
}

// DockTabLabel is what a DockArea's tab shows: an icon, the text, and
// while its panel may close, a button named CloseLabel that closes it.
type DockTabLabel struct {
	Tab        base.DockTab
	Text       string
	Icon       IconName
	CloseLabel string
}

// Render implements core.Node.
func (l DockTabLabel) Render(cx *core.Scope) core.Node {
	colors := ColorsOf(cx)
	var out []core.Node
	if l.Icon != "" {
		out = append(out, Icon(l.Icon, iconSize, inherited))
	}
	out = append(out, line(l.Text, core.Style{}, 1))
	if l.CloseLabel != "" && l.Tab.Closable != nil {
		out = append(out, core.Show{When: l.Tab.Closable, Then: func(cx *core.Scope) core.Node {
			return base.Button{Label: l.CloseLabel, OnPress: func(core.PressEvent) { l.Tab.Close() },
				Style: core.Style{Width: core.Pt(iconSize + 2), Height: core.Pt(iconSize + 2), AlignItems: core.AlignCenter,
					JustifyContent: core.JustifyCenter, CornerRadius: radiusSmall, Margin: core.Edges{Right: -4}},
				StyleFunc: func() core.Style {
					return core.Style{Color: colors.Muted(), Hover: &core.Style{Background: colors.Pressed(), Color: colors.Text()}}
				},
				Children: []core.Node{Icon(IconClose, iconSize-4, inherited)}}
		}})
	}
	return core.View{Style: core.Style{Direction: core.Row, AlignItems: core.AlignCenter, Gap: 6}, Children: out}
}
