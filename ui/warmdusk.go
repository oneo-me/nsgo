package ui

import "github.com/oneo-me/nsgo/core"

// WarmDuskTheme returns Warm Dusk (暖暮), a built-in theme: Onevra's warm
// paper and charcoal with its terracotta accent, which an app gives to
// SetTheme as it does Theme.
func WarmDuskTheme() *core.Theme {
	t := core.DefaultTheme()
	text, dark := core.RGB(0x33332f), core.RGB(0xeeeae2)
	t.Light = core.Palette{
		Background: core.RGB(0xfcfbf9), Surface: core.RGB(0xfefefd), SurfaceHover: core.RGB(0xeceae4), SurfacePressed: core.RGB(0xd9d7d2),
		Border: core.RGB(0xdeded5), Text: text, TextMuted: core.RGB(0x72726a),
		Accent: core.RGB(0xbd481b), AccentHover: core.RGB(0xa63f18), AccentPressed: core.RGB(0x8e3614), AccentText: core.RGB(0xffffff),
		Danger: core.RGB(0xb4444c), Warning: core.RGB(0xb45309), Success: core.RGB(0x059669),
		Selection: core.RGBA(0xe6652b, 0.22), Focus: core.RGBA(0xb94716, 0.55), Scrollbar: text.Faded(0.30),
		Colors: map[string]core.Color{
			"inset": core.RGB(0xf2f1ee), "raised": core.RGB(0xfefefd), "field": core.RGB(0xfdfdfc), "fieldBorder": core.RGB(0xdeded5),
			"line": core.RGB(0xe6e5de), "faint": core.RGB(0xa29e95), "accentFill": core.RGB(0xbd481b),
			"light": core.RGB(0x33332f), "onLight": core.RGB(0xfcfbf9), "track": core.RGB(0xc4c4bc), "knob": core.RGB(0xffffff),
			"code": core.RGB(0x3a3631), "info": core.RGB(0x3d63d1), "down": core.RGB(0xb4444c),
		},
	}
	t.Dark = core.Palette{
		Background: core.RGB(0x211f1c), Surface: core.RGB(0x2b2925), SurfaceHover: core.RGB(0x36332e), SurfacePressed: core.RGB(0x403c36),
		Border: core.RGB(0x504b43), Text: dark, TextMuted: core.RGB(0xbdb6aa),
		Accent: core.RGB(0xed986e), AccentHover: core.RGB(0xefa47f), AccentPressed: core.RGB(0xd18661), AccentText: core.RGB(0x291b14),
		Danger: core.RGB(0xe58f95), Warning: core.RGB(0xe6b567), Success: core.RGB(0x79bea1),
		Selection: core.RGBA(0xed986e, 0.24), Focus: core.RGBA(0xed986e, 0.8), Scrollbar: dark.Faded(0.35),
		Colors: map[string]core.Color{
			"inset": core.RGB(0x2a2825), "raised": core.RGB(0x36332e), "field": core.RGB(0x262421), "fieldBorder": core.RGB(0x504b43),
			"line": core.RGB(0x3d3933), "faint": core.RGB(0x857e73), "accentFill": core.RGB(0xed986e),
			"light": core.RGB(0xeeeae2), "onLight": core.RGB(0x211f1c), "track": core.RGB(0x68635b), "knob": core.RGB(0xf7f5f1),
			"code": core.RGB(0xe2dcd2), "info": core.RGB(0x7ea8ff), "down": core.RGB(0xe58f95),
		},
	}
	return t
}
