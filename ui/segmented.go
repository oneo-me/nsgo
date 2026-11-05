package ui

import (
	"fmt"

	"github.com/oneo-me/nsgo/base"
	"github.com/oneo-me/nsgo/core"
)

// Segmented is Onevra's segmented control: equal segments in a recessed
// capsule, the selected one on a raised thumb that slides to it, 28 pt
// tall: base.Tabs, a tab per segment, in ui's look. Small is the
// same control.
type Segmented[T comparable] struct {
	Options  []Option[T]
	Value    func() T
	OnChange func(T)
	Small    bool
}

// Render implements core.Node.
func (sc Segmented[T]) Render(cx *core.Scope) core.Node {
	colors := ColorsOf(cx)
	width := core.NewSignal(cx, core.Points(0))
	x := core.NewAnimatedValue(cx, 0)
	index := func() int {
		v := sc.Value()
		for i, o := range sc.Options {
			if o.Value == v {
				return i
			}
		}
		return 0
	}
	// The track's padding: the segments share what is left of the
	// measured width.
	const pad = 2
	segment := func() core.Points { return (width.Get() - 2*pad) / core.Points(max(1, len(sc.Options))) }
	height := ButtonHeight + 2*pad
	// The first placement jumps; later ones slide.
	placed := false
	env := cx.Env()
	cx.Effect(func() {
		to := float64(core.Points(index()) * segment())
		if width.Get() <= 0 {
			return
		}
		if placed && !env.ReduceMotion() {
			Spring(cx, x, to, SettleSpring)
		} else {
			x.Set(to)
		}
		placed = true
	})
	children := []core.Node{core.View{PointerEvents: core.PointerEventsNone, StyleFunc: func() core.Style {
		return FinOf(cx).Pill().Over(core.Style{Position: core.Absolute, Top: core.Pt(pad), Left: core.Pt(pad), Bottom: core.Pt(pad),
			Width: core.Pt(max(0, segment())), CornerRadius: (height - 2*pad) / 2,
			Animated: &core.AnimatedStyle{TranslateX: x}})
	}}}
	for _, o := range sc.Options {
		selected := func() bool { return sc.Value() == o.Value }
		label := []core.Node{core.Text{Content: o.Label, MaxLines: 1, StyleFunc: func() core.Style {
			c, w := Alpha(colors.Text(), 0.6), Regular
			if selected() {
				c, w = colors.Text(), Semibold
			}
			return core.Style{FontSize: 13, FontWeight: w, Color: c}
		}}}
		if o.Count != nil {
			label = append(label, Textf(func() string { return fmt.Sprint(o.Count()) }, 12, Regular, colors.Faint))
		}
		children = append(children, base.Tab{Selected: selected, OnPress: func(core.PressEvent) { sc.OnChange(o.Value) }, Label: o.Label,
			Style: core.Style{FlexGrow: 1, FlexBasis: core.Pt(0), Direction: core.Row, AlignItems: core.AlignCenter, JustifyContent: core.JustifyCenter, Gap: 5,
				Padding: core.Symmetric(0, 12), Cursor: core.CursorPointer},
			Children: label})
	}
	return core.View{OnLayout: func(b core.Bounds) { width.Set(b.Size.Width) }, Children: []core.Node{base.Tabs{
		Style:     core.Style{Height: core.Pt(height), Direction: core.Row, Padding: core.All(pad), CornerRadius: height / 2},
		StyleFunc: func() core.Style { return core.Style{Background: Alpha(colors.Text(), 0.06)} },
		Children:  children,
	}}}
}
