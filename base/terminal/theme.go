package terminal

import (
	"github.com/nsgo-dev/nsgo/base/terminal/internal/vt"
	"github.com/nsgo-dev/nsgo/core"
)

// Theme is the colors of a terminal, which programs may change (OSC 4,
// 10, 11, 12) until the terminal resets. GhosttyTheme returns Ghostty's
// themes. A zero color is unset.
type Theme struct {
	Foreground, Background core.Color
	// Cursor colors the cursor: the foreground when unset. CursorText
	// colors the text under a block cursor: the background when unset.
	Cursor, CursorText core.Color
	// Selection highlights the selected text: the foreground, a third
	// opaque, when unset. SelectionText colors the selected text: its own
	// colors when unset.
	Selection, SelectionText core.Color
	// Search, SearchText, SearchSelected and SearchSelectedText are
	// Ghostty's search colors, which theme files set; the view does not
	// search yet.
	Search, SearchText, SearchSelected, SearchSelectedText core.Color
	// Palette holds the 16 colors of programs: black, red, green, yellow,
	// blue, magenta, cyan and white, then their bright kinds.
	Palette [16]core.Color
}

// DarkTheme returns Visual Studio Code's dark terminal colors.
func DarkTheme() *Theme {
	return &Theme{
		Foreground: core.RGB(0xcccccc), Background: core.RGB(0x1e1e1e),
		Palette: [16]core.Color{
			core.RGB(0x000000), core.RGB(0xcd3131), core.RGB(0x0dbc79), core.RGB(0xe5e510),
			core.RGB(0x2472c8), core.RGB(0xbc3fbc), core.RGB(0x11a8cd), core.RGB(0xe5e5e5),
			core.RGB(0x666666), core.RGB(0xf14c4c), core.RGB(0x23d18b), core.RGB(0xf5f543),
			core.RGB(0x3b8eea), core.RGB(0xd670d6), core.RGB(0x29b8db), core.RGB(0xffffff),
		},
	}
}

// LightTheme returns Visual Studio Code's light terminal colors.
func LightTheme() *Theme {
	return &Theme{
		Foreground: core.RGB(0x333333), Background: core.RGB(0xffffff),
		Palette: [16]core.Color{
			core.RGB(0x000000), core.RGB(0xcd3131), core.RGB(0x00bc00), core.RGB(0x949800),
			core.RGB(0x0451a5), core.RGB(0xbc05bc), core.RGB(0x0598bc), core.RGB(0x555555),
			core.RGB(0x666666), core.RGB(0xcd3131), core.RGB(0x14ce14), core.RGB(0xb5ba00),
			core.RGB(0x0451a5), core.RGB(0xbc05bc), core.RGB(0x0598bc), core.RGB(0xa5a5a5),
		},
	}
}

var themes = [2]*Theme{LightTheme(), DarkTheme()}

func defaultTheme(dark bool) *Theme {
	if dark {
		return themes[1]
	}
	return themes[0]
}

// dark reports whether the theme's background is dark.
func (th *Theme) dark() bool {
	b := rgb(th.Background)
	return 299*int(b.R)+587*int(b.G)+114*int(b.B) < 128*1000
}

// palette returns the 256 colors of the theme: its 16, the 6×6×6 cube and
// the 24 grays of xterm.
func (th *Theme) palette() *[256]vt.RGB {
	var p [256]vt.RGB
	for i, c := range th.Palette {
		p[i] = rgb(c)
	}
	levels := [6]uint8{0, 95, 135, 175, 215, 255}
	for i := range 216 {
		p[16+i] = vt.RGB{R: levels[i/36], G: levels[i/6%6], B: levels[i%6]}
	}
	for i := range 24 {
		v := uint8(8 + 10*i)
		p[232+i] = vt.RGB{R: v, G: v, B: v}
	}
	return &p
}

// apply makes the theme the terminal's defaults.
func (th *Theme) apply(t *vt.Terminal) {
	var cursor *vt.RGB
	if th.Cursor != 0 {
		c := rgb(th.Cursor)
		cursor = &c
	}
	t.SetColors(rgb(th.Foreground), rgb(th.Background), cursor, th.palette())
}

func rgb(c core.Color) vt.RGB { return vt.RGB{R: uint8(c >> 24), G: uint8(c >> 16), B: uint8(c >> 8)} }

func color(c vt.RGB) core.Color { return core.RGB(uint32(c.R)<<16 | uint32(c.G)<<8 | uint32(c.B)) }
