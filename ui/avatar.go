package ui

import (
	"github.com/oneo-me/nsgo/base"

	"github.com/oneo-me/nsgo/core"
)

// AvatarStatus is the presence an avatar's corner dot shows.
type AvatarStatus uint8

const (
	// AvatarNoStatus shows no dot.
	AvatarNoStatus AvatarStatus = iota
	// AvatarOnline is a dot of the up tone.
	AvatarOnline
	// AvatarOffline is a faint dot.
	AvatarOffline
)

// Avatar is a round mark for a person or an account: its Image, or the
// initials of Name while the image loads, when it fails and without one.
// Assistive technology reads it as an image named Label, else Name: give
// Label the status when one shows.
type Avatar struct {
	Name   string
	Image  core.ImageSource
	Status AvatarStatus
	// Size is the diameter, 32 points when 0. The initials and the dot
	// follow it.
	Size   core.Points
	Label  string
	TestID string
	Style  core.Style
}

// Render implements core.Node.
func (a Avatar) Render(cx *core.Scope) core.Node { return a.render(cx, 0, false) }

// avatarSize is the diameter of an avatar of size, the standard when 0.
func avatarSize(size core.Points) core.Points {
	if size > 0 {
		return size
	}
	return 32
}

// render draws the avatar with a ring of the surface's color, as wide as
// ring, which overlapping avatars cut into the one before with; lift
// raises it in the accent's ring under the pointer.
func (a Avatar) render(cx *core.Scope, ring core.Points, lift bool) core.Node {
	colors := ColorsOf(cx)
	d := avatarSize(a.Size)
	label := a.Label
	if label == "" {
		label = a.Name
	}
	s := core.Style{Width: core.Pt(d), Height: core.Pt(d), FlexShrink: 0, AlignItems: core.AlignCenter, JustifyContent: core.JustifyCenter,
		CornerRadius: d / 2, BorderWidth: ring}
	if ring > 0 {
		s.Transition = quick
		s.Transition.Properties |= core.TransitionTransform
	}
	size := max(8, 6+d*0.16)
	var children []core.Node
	if a.Status != AvatarNoStatus {
		dot := min(14, max(8, 4+d/6))
		inset := max(0, (d-48)/10) - ring
		edge := core.Points(2)
		if d < 32 {
			edge = 1.5
		}
		tone := colors.Up
		if a.Status == AvatarOffline {
			tone = colors.Faint
		}
		children = append(children, core.View{Style: core.Style{Position: core.Absolute, Right: core.Pt(inset), Bottom: core.Pt(inset),
			Width: core.Pt(dot), Height: core.Pt(dot), CornerRadius: dot / 2, BorderWidth: edge},
			StyleFunc: func() core.Style { return core.Style{Background: tone(), BorderColor: colors.Surface()} }})
	}
	return base.Avatar{TestID: a.TestID, Src: a.Image, Name: a.Name, Label: label, Style: s, ImageResizeMode: core.Cover,
		ImageStyle: core.Style{CornerRadius: d/2 - ring},
		FallbackStyle: core.Style{AlignItems: core.AlignCenter, JustifyContent: core.JustifyCenter,
			FontSize: size, LineHeight: size + 2, FontWeight: Medium},
		StyleFunc: func() core.Style {
			st := core.Style{Background: colors.Hover(), BorderColor: colors.Surface(), Color: colors.Text()}
			if lift {
				st.Hover = &core.Style{BorderColor: colors.Accent(), Transform: core.Transform{TranslateY: -2}}
			}
			return st.Merge(a.Style)
		}, Children: children}
}
