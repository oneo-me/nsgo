package ui

import (
	"strconv"
	"strings"
	"unicode"

	"github.com/oneo-me/nsgo/core"
)

// Badge is a small label in a tinted capsule, as Onevra's: Text, or what
// TextFunc returns, in Tone, or the one ToneFunc returns. Outline draws
// only its rim.
type Badge struct {
	Text     string
	TextFunc func() string
	Tone     Tone
	ToneFunc func() Tone
	Outline  bool
}

// Render implements core.Node.
func (b Badge) Render(cx *core.Scope) core.Node {
	colors := ColorsOf(cx)
	label := b.TextFunc
	if label == nil {
		label = constant(b.Text)
	}
	tone := b.ToneFunc
	if tone == nil {
		tone = func() Tone { return b.Tone }
	}
	// A tinted tone's text leans to the text's color, and its rim to the
	// page's, as Onevra's badges do.
	fg := func() core.Color {
		if tone() == ToneNeutral {
			return colors.Text()
		}
		return Mix(tone().Color(cx), colors.Text(), 0.3)
	}
	return core.View{Style: core.Style{Padding: core.Symmetric(2, 8), AlignSelf: core.AlignStart, CornerRadius: 12, JustifyContent: core.JustifyCenter, BorderWidth: 1},
		StyleFunc: func() core.Style {
			t := tone()
			if b.Outline {
				return core.Style{BorderColor: Mix(colors.Background(), t.Color(cx), 0.5)}
			}
			if t == ToneNeutral {
				return core.Style{Background: colors.Surface(), BorderColor: colors.Border()}
			}
			return core.Style{Background: Mix(colors.Background(), t.Color(cx), 0.12), BorderColor: Mix(colors.Background(), t.Color(cx), 0.3)}
		}, Children: []core.Node{Textf(label, 11, Medium, fg)}}
}

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
	loaded, failed := core.NewSignal(cx, false), core.NewSignal(cx, false)
	size := max(8, 6+d*0.16)
	children := []core.Node{core.Text{Content: initials(a.Name), MaxLines: 1,
		Style: core.Style{FontSize: size, LineHeight: size + 2, FontWeight: Medium},
		StyleFunc: func() core.Style {
			if loaded.Get() {
				return core.Style{Visibility: core.VisibilityHidden}
			}
			return core.Style{Color: colors.Text()}
		}}}
	if hasImage(a.Image) {
		children = append(children, core.Show{When: func() bool { return !failed.Get() }, Then: func(cx *core.Scope) core.Node {
			return core.Image{Source: a.Image, ResizeMode: core.Cover, OnLoad: func(core.Size) { loaded.Set(true) },
				OnError: func(error) { failed.Set(true) },
				Style: core.Style{Position: core.Absolute, Top: core.Pt(0), Right: core.Pt(0), Bottom: core.Pt(0), Left: core.Pt(0),
					CornerRadius: d/2 - ring}}
		}})
	}
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
	return core.View{TestID: a.TestID, Role: core.RoleImage, Label: label, Accessible: true, Style: s,
		StyleFunc: func() core.Style {
			st := core.Style{Background: colors.Hover(), BorderColor: colors.Surface()}
			if lift {
				st.Hover = &core.Style{BorderColor: colors.Accent(), Transform: core.Transform{TranslateY: -2}}
			}
			return st.Merge(a.Style)
		}, Children: children}
}

// hasImage reports whether src names a picture.
func hasImage(src core.ImageSource) bool { return src.URI != "" || len(src.Data) > 0 }

// initials are the capitals of the first two words of name.
func initials(name string) string {
	var b strings.Builder
	for i, word := range strings.Fields(name) {
		if i == 2 {
			break
		}
		for _, r := range word {
			b.WriteRune(unicode.ToUpper(r))
			break
		}
	}
	return b.String()
}

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
	limit = min(max(limit, 0), len(g.Members))
	more := len(g.Members) - limit
	overlap := d / 6
	onDesktop := !touch(cx)
	fan := onDesktop && !reduceMotion(cx)
	slots := limit
	if more > 0 {
		slots++
	}
	slot := func(i int, child core.Node) core.Node {
		s := core.Style{Margin: core.Edges{Left: -overlap}}
		if fan {
			spread := (core.Points(i) - core.Points(slots-1)/2) * overlap * 2 / 3
			s.Transition = core.Transition{Duration: quick.Duration * 3 / 2, Easing: core.EaseOut, Properties: core.TransitionTransform}
			s.GroupStyle = &core.GroupStyle{Name: avatarGroupName, Hover: &core.Style{Transform: core.Transform{TranslateX: spread}}}
		}
		return core.View{Style: s, Children: []core.Node{child}}
	}
	var children []core.Node
	for i, m := range g.Members[:limit] {
		node := Avatar{Name: m.Name, Image: m.Image, Status: m.Status, Size: d, Label: m.Label}.render(cx, 2, fan)
		if onDesktop {
			node = core.View{Tooltip: tooltipBubble(cx, m.Name), Children: []core.Node{node}}
		}
		children = append(children, slot(i, node))
	}
	if more > 0 {
		count := "+" + strconv.Itoa(more)
		label := count
		if g.MoreLabel != nil {
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
		children = append(children, slot(limit, chip))
	}
	return core.View{TestID: g.TestID, Label: g.Label,
		Style: core.Style{Direction: core.Row, AlignSelf: core.AlignStart, FlexShrink: 0, Padding: core.Edges{Left: overlap},
			Group: avatarGroupName}.Merge(g.Style),
		Children: children}
}
