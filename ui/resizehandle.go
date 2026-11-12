package ui

import (
	"time"

	"github.com/nsgo-dev/nsgo/base"
	"github.com/nsgo-dev/nsgo/core"
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

// HandleLine is a handle's line, which stays as it is. It is a StyleFunc.
func HandleLine(cx *core.Scope) func() core.Style { return Filled(ColorsOf(cx).Border) }

// ResizeHandle is base.ResizeHandle with ui's line.
type ResizeHandle base.ResizeHandle

// Render implements core.Node.
func (h ResizeHandle) Render(cx *core.Scope) core.Node {
	out := base.ResizeHandle(h)
	out.StyleFunc = over(HandleLine(cx), h.Style, h.StyleFunc)
	out.ThumbStyle = HandleThumb(cx).Merge(h.ThumbStyle)
	out.ThumbStyleFunc = over(HandleThumbColor(cx), h.ThumbStyle, h.ThumbStyleFunc)
	return out
}
