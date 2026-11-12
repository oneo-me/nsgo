package ui

import (
	"strconv"
	"sync"

	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/icons/lucide"
)

// Rating is a score from 0 to Max, 5 when 0, in stars filled in the
// warning color. A press on a star gives that score, or 0 when it is the
// score already, and on desktops the pointer previews the score under it.
// Assistive technology reads one adjustable element, and it is a Tab stop
// the arrow keys adjust.
type Rating struct {
	Value    func() int
	OnChange func(value int)
	Max      int
	// Readonly shows the score and takes nothing.
	Readonly bool
	Disabled func() bool
	// ValueText is what assistive technology reads as the value, such as
	// "3 stars", the number when nil.
	ValueText func(value int) string
	Label     string
	TestID    string
	Style     core.Style
}

var (
	starOnce sync.Once
	star     *core.SVG
)

// filledStar is Lucide's star filled in the text's color.
func filledStar() *core.SVG {
	starOnce.Do(func() {
		star = core.MustParseSVG([]byte(`<svg viewBox="0 0 24 24" fill="currentColor" stroke="currentColor" stroke-width="2" stroke-linejoin="round">` +
			`<path d="M11.525 2.295a.53.53 0 0 1 .95 0l2.31 4.679a2.123 2.123 0 0 0 1.595 1.16l5.166.756a.53.53 0 0 1 .294.904l-3.736 3.638a2.123 2.123 0 0 0-.611 1.878l.882 5.14a.53.53 0 0 1-.771.56l-4.618-2.428a2.122 2.122 0 0 0-1.973 0L6.396 21.01a.53.53 0 0 1-.77-.56l.881-5.139a2.122 2.122 0 0 0-.611-1.879L2.16 9.795a.53.53 0 0 1 .294-.906l5.165-.755a2.122 2.122 0 0 0 1.597-1.16z"/></svg>`))
	})
	return star
}

// Render implements core.Node.
func (r Rating) Render(cx *core.Scope) core.Node {
	colors := ColorsOf(cx)
	env := cx.Env()
	onDesktop := core.Untracked(cx, env.Desktop)
	limit := r.Max
	if limit <= 0 {
		limit = 5
	}
	value := func() int {
		if r.Value == nil {
			return 0
		}
		return min(max(r.Value(), 0), limit)
	}
	locked := func() bool { return r.Readonly || is(r.Disabled) }
	set := func(v int) {
		if v = min(max(v, 0), limit); v != core.Untracked(cx, value) && !core.Untracked(cx, locked) && r.OnChange != nil {
			r.OnChange(v)
		}
	}
	by := func(d int) func() { return func() { set(core.Untracked(cx, value) + d) } }
	hover := core.NewSignal(cx, 0)
	shown := func() int {
		if h := hover.Get(); h > 0 && !locked() {
			return h
		}
		return value()
	}
	const size = iconSize + 3
	var stars []core.Node
	for i := 1; i <= limit; i++ {
		filled := func() bool { return i <= shown() }
		v := core.View{Style: core.Style{Padding: core.All(2)}, Children: []core.Node{core.Icon{SVGFunc: func() *core.SVG {
			if filled() {
				return filledStar()
			}
			return lucide.Star()
		}, Style: core.Style{Width: core.Pt(size), Height: core.Pt(size)},
			StyleFunc: func() core.Style {
				if filled() {
					return core.Style{Color: colors.Warn()}
				}
				return core.Style{Color: colors.Faint()}
			}}}}
		if r.TestID != "" {
			v.TestID = r.TestID + "-" + strconv.Itoa(i)
		}
		if !r.Readonly {
			v.NoFocus = true
			v.OnPress = func(core.PressEvent) {
				if core.Untracked(cx, value) == i {
					set(0)
				} else {
					set(i)
				}
			}
			if onDesktop {
				v.OnMouseEnter = func(core.MouseEvent) { hover.Set(i) }
				v.OnMouseLeave = func(core.MouseEvent) { hover.Set(0) }
			}
		}
		stars = append(stars, v)
	}
	focused := core.NewSignal(cx, false)
	stars = append(stars, core.Show{When: focused.Get, Then: func(cx *core.Scope) core.Node {
		// Left and Right trade places in a right-to-left layout.
		sideways := func(d int) func() {
			return func() {
				if core.Untracked(cx, env.RTL) {
					by(-d)()
				} else {
					by(d)()
				}
			}
		}
		arrow := core.KeyOptions{When: core.KeyAuto, Repeat: true}
		cx.OnKeyWith("right", arrow, sideways(1))
		cx.OnKeyWith("left", arrow, sideways(-1))
		cx.OnKeyWith("up", arrow, by(1))
		cx.OnKeyWith("down", arrow, by(-1))
		cx.OnKeyWith("home", core.KeyOptions{When: core.KeyAuto}, func() { set(0) })
		cx.OnKeyWith("end", core.KeyOptions{When: core.KeyAuto}, func() { set(limit) })
		return nil
	}})
	view := core.View{TestID: r.TestID, Accessible: true, Role: core.RoleAdjustable, Label: r.Label, Disabled: r.Disabled,
		AccessibilityValue: func() core.AccessibilityValue {
			v := core.AccessibilityValue{Min: 0, Max: float32(limit), Now: float32(value())}
			if r.ValueText != nil {
				v.Text = r.ValueText(value())
			}
			return v
		},
		Style: core.Style{Direction: core.Row, AlignItems: core.AlignCenter, AlignSelf: core.AlignStart, CornerRadius: RadiusSmall,
			Disabled: &core.Style{Opacity: core.Alpha(0.45)}}.Merge(r.Style),
		StyleFunc: func() core.Style {
			if r.Readonly {
				return core.Style{}
			}
			return core.Style{Focus: FocusRing(cx, 2)}
		}, Children: stars}
	if !r.Readonly {
		view.AccessibilityActions = []core.AccessibilityAction{{Name: core.ActionIncrement}, {Name: core.ActionDecrement}}
		view.OnAccessibilityAction = func(name string) {
			switch name {
			case core.ActionIncrement:
				by(1)()
			case core.ActionDecrement:
				by(-1)()
			}
		}
		if onDesktop {
			view.OnPress = func(core.PressEvent) {}
			view.OnFocus = func() { focused.Set(true) }
			view.OnBlur = func() { focused.Set(false) }
		}
	}
	return view
}
