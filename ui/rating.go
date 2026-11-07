package ui

import (
	"sync"

	"github.com/oneo-me/nsgo/base"
	"github.com/oneo-me/nsgo/core"
	"github.com/oneo-me/nsgo/icons/lucide"
)

// Rating is base.Rating in stars filled in the warning color.
type Rating struct {
	Value    func() int
	OnChange func(value int)
	Max      int
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
	const size = iconSize + 3
	star := func(_ int, filled func() bool) core.Node {
		return core.Icon{SVGFunc: func() *core.SVG {
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
			}}
	}
	return base.Rating{Value: r.Value, OnChange: r.OnChange, Max: r.Max, Readonly: r.Readonly, Disabled: r.Disabled, ValueText: r.ValueText,
		Label: r.Label, TestID: r.TestID, Star: star, StarStyle: core.Style{Padding: core.All(2)},
		Style: core.Style{Direction: core.Row, AlignItems: core.AlignCenter, AlignSelf: core.AlignStart, CornerRadius: RadiusSmall,
			Disabled: &core.Style{Opacity: core.Alpha(0.45)}}.Merge(r.Style),
		StyleFunc: func() core.Style {
			if r.Readonly {
				return core.Style{}
			}
			return core.Style{Focus: FocusRing(cx, 2)}
		}}
}
