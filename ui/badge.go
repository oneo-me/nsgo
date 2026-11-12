package ui

import "github.com/nsgo-dev/nsgo/core"

// BadgeVariant is a Badge's look.
type BadgeVariant uint8

const (
	// BadgeNeutral is the bordered surface. It is the default.
	BadgeNeutral BadgeVariant = iota
	// BadgeAccent tints the capsule with the accent.
	BadgeAccent
	// BadgeInfo tints the capsule with the info color.
	BadgeInfo
	// BadgeWarn tints the capsule with the warning color.
	BadgeWarn
	// BadgeDanger tints the capsule with the danger color.
	BadgeDanger
	// BadgeUp tints the capsule with the color of a rise.
	BadgeUp
	// BadgeDown tints the capsule with the color of a fall.
	BadgeDown
	// BadgeOutline draws only the capsule's rim.
	BadgeOutline
)

// tint is the color a variant tints the capsule with, and false for a
// variant with none.
func (v BadgeVariant) tint(colors Colors) (core.Color, bool) {
	switch v {
	case BadgeAccent:
		return colors.Accent(), true
	case BadgeInfo:
		return colors.Info(), true
	case BadgeWarn:
		return colors.Warn(), true
	case BadgeDanger:
		return colors.Danger(), true
	case BadgeUp:
		return colors.Up(), true
	case BadgeDown:
		return colors.Down(), true
	}
	return 0, false
}

// Badge is a small label in a capsule: Text, or what TextFunc
// returns, in Variant, or the one VariantFunc returns.
type Badge struct {
	Text        string
	TextFunc    func() string
	Variant     BadgeVariant
	VariantFunc func() BadgeVariant
}

// Render implements core.Node.
func (b Badge) Render(cx *core.Scope) core.Node {
	colors := ColorsOf(cx)
	label := b.TextFunc
	if label == nil {
		label = constant(b.Text)
	}
	variant := b.VariantFunc
	if variant == nil {
		variant = func() BadgeVariant { return b.Variant }
	}
	// A tinted badge's text leans to the text's color, and its rim to the
	// page's.
	fg := func() core.Color {
		if tint, ok := variant().tint(colors); ok {
			return Mix(tint, colors.Text(), 0.3)
		}
		return colors.Text()
	}
	return core.View{Style: core.Style{Padding: core.Symmetric(2, 8), AlignSelf: core.AlignStart, CornerRadius: 12, JustifyContent: core.JustifyCenter, BorderWidth: 1},
		StyleFunc: func() core.Style {
			v := variant()
			if v == BadgeOutline {
				return core.Style{BorderColor: Mix(colors.Background(), colors.Muted(), 0.5)}
			}
			tint, ok := v.tint(colors)
			if !ok {
				return core.Style{Background: colors.Surface(), BorderColor: colors.Border()}
			}
			return core.Style{Background: Mix(colors.Background(), tint, 0.12), BorderColor: Mix(colors.Background(), tint, 0.3)}
		}, Children: []core.Node{Textf(label, 11, Medium, fg)}}
}
