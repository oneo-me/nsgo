package ui

import "github.com/oneo-me/nsgo/core"

// BadgeVariant is a Badge's look.
type BadgeVariant uint8

const (
	// BadgeTinted fills the capsule with a tint of the tone. It is the
	// default.
	BadgeTinted BadgeVariant = iota
	// BadgeOutline draws only the capsule's rim.
	BadgeOutline
)

// Badge is a small label in a capsule, as Onevra's: Text, or what TextFunc
// returns, in Tone, or the one ToneFunc returns.
type Badge struct {
	Text     string
	TextFunc func() string
	Tone     Tone
	ToneFunc func() Tone
	Variant  BadgeVariant
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
			if b.Variant == BadgeOutline {
				return core.Style{BorderColor: Mix(colors.Background(), t.Color(cx), 0.5)}
			}
			if t == ToneNeutral {
				return core.Style{Background: colors.Surface(), BorderColor: colors.Border()}
			}
			return core.Style{Background: Mix(colors.Background(), t.Color(cx), 0.12), BorderColor: Mix(colors.Background(), t.Color(cx), 0.3)}
		}, Children: []core.Node{Textf(label, 11, Medium, fg)}}
}
