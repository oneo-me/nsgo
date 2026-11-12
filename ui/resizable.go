package ui

import (
	"github.com/nsgo-dev/nsgo/base"
	"github.com/nsgo-dev/nsgo/core"
)

// phone reports whether the device is a phone.
func phone(cx *core.Scope) bool { return core.Untracked(cx, cx.Env().Idiom) == core.IdiomPhone }

// The panels are base's types under these names: they take every field
// of base's, and a part style the app sets merges over ui's.

// ResizablePanelGroup is base.ResizablePanelGroup with ui's
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

// over is a part's style function: ui's, then the app's style
// and style function over it, as base merges a style function over a
// style. Either function may be nil.
func over(look func() core.Style, style core.Style, app func() core.Style) func() core.Style {
	if look == nil {
		return app
	}
	return func() core.Style {
		s := look().Merge(style)
		if app != nil {
			s = s.Merge(app())
		}
		return s
	}
}
