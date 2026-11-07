package ui

import (
	"github.com/oneo-me/nsgo/base"
	"strconv"

	"github.com/oneo-me/nsgo/core"
)

// AvatarGroupMember is one person of an AvatarGroup.
type AvatarGroupMember struct {
	Name   string
	Image  core.ImageSource
	Status AvatarStatus
	// Label is what assistive technology reads, Name when empty.
	Label string
}

// AvatarGroup is an overlapping row of avatars, the first Max shown and
// the rest counted on a chip. On desktops pointing at the row spreads it
// and names the avatar under the pointer in a tooltip.
type AvatarGroup struct {
	Members []AvatarGroupMember
	// Max is how many avatars show before the chip: 4 when 0, none when
	// negative.
	Max int
	// Size is each avatar's diameter, Avatar's standard when 0.
	Size core.Points
	// Label names the group for assistive technology.
	Label string
	// MoreLabel names the chip of n more members; "+n" when nil.
	MoreLabel func(n int) string
	TestID    string
	Style     core.Style
}

// avatarGroupName is the group a row's slots follow the hover of.
const avatarGroupName = "avatar-group"

// Render implements core.Node.
func (g AvatarGroup) Render(cx *core.Scope) core.Node {
	colors := ColorsOf(cx)
	d := avatarSize(g.Size)
	limit := g.Max
	if limit == 0 {
		limit = 4
	}
	limit = max(limit, -1)
	shown := min(max(limit, 0), len(g.Members))
	more := len(g.Members) - shown
	overlap := d / 6
	onDesktop := !Touch(cx)
	fan := onDesktop && !ReduceMotion(cx)
	slots := shown
	if more > 0 {
		slots++
	}
	slot := func(i int) core.Style {
		s := core.Style{Margin: core.Edges{Left: -overlap}}
		if fan {
			spread := (core.Points(i) - core.Points(slots-1)/2) * overlap * 2 / 3
			s.Transition = core.Transition{Duration: quick.Duration * 3 / 2, Easing: core.EaseOut, Properties: core.TransitionTransform}
			s.GroupStyle = &core.GroupStyle{Name: avatarGroupName, Hover: &core.Style{Transform: core.Transform{TranslateX: spread}}}
		}
		return s
	}
	var children []core.Node
	for i, m := range g.Members {
		node := Avatar{Name: m.Name, Image: m.Image, Status: m.Status, Size: d, Label: m.Label}.render(cx, 2, fan)
		if onDesktop {
			node = core.View{Tooltip: tooltipBubble(cx, m.Name), Children: []core.Node{node}}
		}
		children = append(children, core.View{Style: slot(i), Children: []core.Node{node}})
	}
	count := "+" + strconv.Itoa(more)
	label := count
	if g.MoreLabel != nil && more > 0 {
		label = g.MoreLabel(more)
	}
	size := max(8, 6+d*0.16)
	chip := core.View{Role: core.RoleImage, Label: label, Accessible: true,
		Style: core.Style{Width: core.Pt(d), Height: core.Pt(d), CornerRadius: d / 2, AlignItems: core.AlignCenter,
			JustifyContent: core.JustifyCenter, BorderWidth: 2},
		StyleFunc: func() core.Style {
			return core.Style{Background: colors.Hover(), Color: colors.Muted(), BorderColor: colors.Surface()}
		},
		Children: []core.Node{core.Text{Content: count, MaxLines: 1,
			Style: core.Style{FontSize: size - 1, LineHeight: size + 1, FontWeight: Medium}}}}
	return base.AvatarGroup{Limit: limit, Overflow: chip, OverflowStyle: slot(shown), TestID: g.TestID, Label: g.Label,
		Style: core.Style{Direction: core.Row, AlignSelf: core.AlignStart, FlexShrink: 0, Padding: core.Edges{Left: overlap},
			Group: avatarGroupName}.Merge(g.Style),
		Children: children}
}
