package core

import (
	"testing"

	"github.com/nsgo-dev/nsgo/internal/driver"
)

func TestThemeSubtrees(t *testing.T) {
	side := DefaultTheme()
	side.Light.Background, side.Dark.Background = RGB(0x111111), RGB(0x222222)
	var appBg, sideBg, lightBg Color
	var forced *Signal[Appearance]
	h := newHost(t, Size{Width: 100, Height: 100}, func(cx *Scope) Node {
		forced = NewSignal(cx, AppearanceDark)
		return View{TestID: "app", StyleFunc: func() Style { appBg = cx.Palette().Background; return Style{} },
			Children: []Node{Func(func(cx *Scope) Node {
				cx.SetTheme(side)
				return View{TestID: "side", StyleFunc: func() Style { sideBg = cx.Palette().Background; return Style{} },
					Children: []Node{Func(func(cx *Scope) Node {
						cx.SetSubtreeAppearance(forced.GetUntracked())
						cx.Effect(func() { cx.SetSubtreeAppearance(forced.Get()) })
						return View{TestID: "forced", StyleFunc: func() Style { lightBg = cx.Palette().Background; return Style{} }}
					})}}
			})}}
	})
	h.commit()
	def := DefaultTheme()
	if appBg != def.Light.Background || sideBg != RGB(0x111111) || lightBg != RGB(0x222222) {
		t.Fatalf("backgrounds = %v %v %v", appBg, sideBg, lightBg)
	}
	if a := h.find("forced").appearance; a != AppearanceDark {
		t.Fatalf("forced view's appearance = %v", a)
	}

	// The app turns dark; the forced subtree goes light when asked.
	h.tree.SetEnvironment(Environment{Dark: true})
	forced.Set(AppearanceLight)
	h.drain()
	b := h.commit()
	if appBg != def.Dark.Background || sideBg != RGB(0x222222) || lightBg != RGB(0x111111) {
		t.Fatalf("dark backgrounds = %v %v %v", appBg, sideBg, lightBg)
	}
	id := h.find("forced").id
	sent := false
	for _, it := range b.Items {
		if it.Kind == driver.ItemViewProps && it.ID == id {
			sent = it.ViewProps.Appearance == AppearanceLight
		}
	}
	if !sent || h.find("side").appearance != AppearanceSystem {
		t.Fatalf("props sent = %v, side = %v", sent, h.find("side").appearance)
	}
}

func TestParseTheme(t *testing.T) {
	th, err := ParseTheme([]byte("radius: 12\nlight:\n  accent: \"#112233\"\n  inset: \"#445566\"\ndark:\n  text: white\n"))
	if err != nil {
		t.Fatal(err)
	}
	if th.Radius != 12 || th.Light.Accent != RGB(0x112233) || th.Light.Colors["inset"] != RGB(0x445566) ||
		th.Dark.Text != RGB(0xffffff) || th.Dark.Accent != DefaultTheme().Dark.Accent {
		t.Fatalf("theme = %+v", th)
	}
	if _, err := ParseTheme([]byte(`{"light": {"accent": "nope"}}`)); err == nil {
		t.Fatal("a bad color parsed")
	}
}

// TestShadcnThemesNameTokens reads a shadcn/ui theme file, as NSH's apps
// name theirs, and classes name its colors.
func TestShadcnThemesNameTokens(t *testing.T) {
	th, err := ParseTheme([]byte(`{"cssVars": {
		"theme": {"radius": "12px", "brand-fill": "#c8f26d"},
		"light": {"brand": "#4f6e0c", "muted-foreground": "#5f636a", "ring": "var(--brand)",
			"brand-tint": "color-mix(in srgb, var(--brand) 10%, transparent)"},
		"dark": {"brand": "#c8f26d"}}}`))
	if err != nil {
		t.Fatal(err)
	}
	if th.Radius != 12 || th.Tokens["radius"] != "12px" {
		t.Errorf("radius %v, tokens %v", th.Radius, th.Tokens)
	}
	if c, _ := th.Light.Color("ring"); c != RGB(0x4f6e0c) {
		t.Errorf("light ring %#x", c)
	}
	if c, _ := th.Dark.Color("ring"); c != RGB(0xc8f26d) {
		t.Errorf("dark ring follows dark's brand: %#x", c)
	}
	if c, _ := th.Light.Color("brand-tint"); c != RGBA(0x4f6e0c, 0.1) {
		t.Errorf("brand-tint %#x", c)
	}
	env := classEnv{palette: &th.Light}
	s, _, _ := parseClasses("bg-brand text-muted-foreground/50 border-(--brand-fill)").resolve(&env)
	if s.Background != RGB(0x4f6e0c) || s.Color != RGBA(0x5f636a, 0.5) || s.BorderColor != RGB(0xc8f26d) {
		t.Errorf("classes %#x %#x %#x", s.Background, s.Color, s.BorderColor)
	}
}
