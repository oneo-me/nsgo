package core

import (
	"fmt"
	"maps"
	"regexp"
	"slices"
	"strings"
	"sync"

	"go.yaml.in/yaml/v3"
)

// Palette is a theme's colors in one appearance. Its roles are Onevra's;
// Colors holds the ones a component library or an app adds, by name.
type Palette struct {
	// Background is the page; Surface a card or control on it, under
	// the pointer and held down.
	Background, Surface, SurfaceHover, SurfacePressed Color
	Border                                            Color
	Text, TextMuted                                   Color
	// Accent is the accent fill, AccentText what is drawn on it.
	Accent, AccentHover, AccentPressed, AccentText Color
	Danger, Warning, Success                       Color
	// Selection is selected text's, Focus a focus ring's and Scrollbar a
	// scroll indicator's.
	Selection, Focus, Scrollbar Color
	// Colors are roles by name past the ones above, such as "inset".
	Colors map[string]Color
}

// paletteRoles names the fields of Palette, as theme files and host
// tokens do.
var paletteRoles = map[string]func(*Palette) *Color{
	"background":     func(p *Palette) *Color { return &p.Background },
	"surface":        func(p *Palette) *Color { return &p.Surface },
	"surfaceHover":   func(p *Palette) *Color { return &p.SurfaceHover },
	"surfacePressed": func(p *Palette) *Color { return &p.SurfacePressed },
	"border":         func(p *Palette) *Color { return &p.Border },
	"text":           func(p *Palette) *Color { return &p.Text },
	"textMuted":      func(p *Palette) *Color { return &p.TextMuted },
	"accent":         func(p *Palette) *Color { return &p.Accent },
	"accentHover":    func(p *Palette) *Color { return &p.AccentHover },
	"accentPressed":  func(p *Palette) *Color { return &p.AccentPressed },
	"accentText":     func(p *Palette) *Color { return &p.AccentText },
	"danger":         func(p *Palette) *Color { return &p.Danger },
	"warning":        func(p *Palette) *Color { return &p.Warning },
	"success":        func(p *Palette) *Color { return &p.Success },
	"selection":      func(p *Palette) *Color { return &p.Selection },
	"focus":          func(p *Palette) *Color { return &p.Focus },
	"scrollbar":      func(p *Palette) *Color { return &p.Scrollbar },
}

// Color returns the role named name: a field's name in lower camel case
// ("surfaceHover") or one of Colors, and whether the palette has it.
func (p *Palette) Color(name string) (Color, bool) {
	if f := paletteRoles[name]; f != nil {
		return *f(p), true
	}
	c, ok := p.Colors[name]
	return c, ok
}

// ColorNames are the names Color takes: the roles' and Colors', sorted.
func (p *Palette) ColorNames() []string {
	names := slices.Collect(maps.Keys(paletteRoles))
	for n := range p.Colors {
		if paletteRoles[n] == nil {
			names = append(names, n)
		}
	}
	slices.Sort(names)
	return names
}

// SetColor sets the role named name, as Color names it.
func (p *Palette) SetColor(name string, c Color) {
	if f := paletteRoles[name]; f != nil {
		*f(p) = c
		return
	}
	if p.Colors == nil {
		p.Colors = map[string]Color{}
	}
	p.Colors[name] = c
	if _, known := themeColors.LoadOrStore(name, struct{}{}); !known {
		// Classes parsed before it was a color name read it now.
		classCache.Clear()
	}
}

// themeColors are the names of colors themes added to their palettes,
// which classes name: bg-inset, text-muted-foreground.
var themeColors sync.Map

// noteColors makes the colors a theme holds names classes know, whether
// SetColor or a literal Colors added them.
func (t *Theme) noteColors() {
	if t == nil {
		return
	}
	for _, p := range []*Palette{&t.Light, &t.Dark} {
		for name := range p.Colors {
			if _, known := themeColors.LoadOrStore(name, struct{}{}); !known {
				classCache.Clear()
			}
		}
	}
}

// clone returns p with a Colors of its own.
func (p Palette) clone() Palette {
	p.Colors = maps.Clone(p.Colors)
	return p
}

// Theme is a look in both appearances: a palette for each, the corner
// radius components start from and font families, empty for the
// system's. Scope.SetTheme gives it to a subtree; treat a theme given as
// read-only.
type Theme struct {
	Light, Dark Palette
	Radius      Points
	Sans, Mono  string
	// Tokens are the theme's design tokens that are no colors, by name:
	// lengths ("12px") and font families, as a shadcn/ui theme file's
	// theme section names them.
	Tokens map[string]string
	// Tailwind is the Tailwind theme the subtree's classes resolve
	// against; nil is Tailwind's default.
	Tailwind *Tailwind
}

// Clone returns a copy of t that shares nothing with it.
func (t *Theme) Clone() *Theme {
	c := *t
	c.Light, c.Dark = t.Light.clone(), t.Dark.clone()
	c.Tokens = maps.Clone(t.Tokens)
	return &c
}

// DefaultTheme returns Onevra's theme, which a tree shows until the app
// sets one: warm neutrals with a terracotta accent, lifted in dark.
func DefaultTheme() *Theme {
	text, dark := RGB(0x333333), RGB(0xeaeaea)
	t := &Theme{
		Light: Palette{
			Background: RGB(0xffffff), Surface: RGB(0xffffff), SurfaceHover: text.Faded(0.08), SurfacePressed: text.Faded(0.14),
			Border: text.Faded(0.16), Text: text, TextMuted: text.Faded(0.55),
			Accent: RGB(0x007aff), AccentHover: RGB(0x006be0), AccentPressed: RGB(0x005cbf), AccentText: RGB(0xffffff),
			Danger: RGB(0xb4444c), Warning: RGB(0xb45309), Success: RGB(0x059669),
			Selection: RGBA(0x007aff, 0.22), Focus: RGBA(0x007aff, 0.55), Scrollbar: text.Faded(0.30),
		},
		Dark: Palette{
			Background: RGB(0x1f1f1f), Surface: RGB(0x292929), SurfaceHover: dark.Faded(0.08), SurfacePressed: dark.Faded(0.14),
			Border: dark.Faded(0.16), Text: dark, TextMuted: dark.Faded(0.55),
			Accent: RGB(0x0a84ff), AccentHover: RGB(0x2793ff), AccentPressed: RGB(0x0974e0), AccentText: RGB(0xffffff),
			Danger: RGB(0xe58f95), Warning: RGB(0xe6b567), Success: RGB(0x79bea1),
			Selection: RGBA(0x0a84ff, 0.24), Focus: RGBA(0x0a84ff, 0.8), Scrollbar: dark.Faded(0.35),
		},
		Radius: 8,
	}
	for name, c := range shadcnLight {
		if paletteRoles[name] == nil {
			t.Light.SetColor(name, c)
		}
	}
	for name, c := range shadcnDark {
		if paletteRoles[name] == nil {
			t.Dark.SetColor(name, c)
		}
	}
	return t
}

// shadcnLight and shadcnDark are shadcn/ui's neutral tokens, NSH's
// defaults, which a theme file replaces. Those that name a role of
// Onevra's (background, border, accent) keep Onevra's.
var shadcnLight = map[string]Color{
	"foreground": RGB(0x0a0a0a), "card": RGB(0xffffff), "card-foreground": RGB(0x0a0a0a), "popover": RGB(0xffffff),
	"popover-foreground": RGB(0x0a0a0a), "primary": RGB(0x171717), "primary-foreground": RGB(0xfafafa),
	"secondary": RGB(0xf5f5f5), "secondary-foreground": RGB(0x171717), "muted": RGB(0xf5f5f5),
	"muted-foreground": RGB(0x737373), "accent-foreground": RGB(0x171717), "destructive": RGB(0xe7000b),
	"input": RGB(0xe5e5e5), "ring": RGB(0xa1a1a1), "chart-1": RGB(0xf54900), "chart-2": RGB(0x009689),
	"chart-3": RGB(0x104e64), "chart-4": RGB(0xffb900), "chart-5": RGB(0xfe9a00), "sidebar": RGB(0xfafafa),
	"sidebar-foreground": RGB(0x0a0a0a), "sidebar-primary": RGB(0x171717), "sidebar-primary-foreground": RGB(0xfafafa),
	"sidebar-accent": RGB(0xf5f5f5), "sidebar-accent-foreground": RGB(0x171717), "sidebar-border": RGB(0xe5e5e5),
	"sidebar-ring": RGB(0xa1a1a1),
}

var shadcnDark = map[string]Color{
	"foreground": RGB(0xfafafa), "card": RGB(0x171717), "card-foreground": RGB(0xfafafa), "popover": RGB(0x171717),
	"popover-foreground": RGB(0xfafafa), "primary": RGB(0xe5e5e5), "primary-foreground": RGB(0x171717),
	"secondary": RGB(0x262626), "secondary-foreground": RGB(0xfafafa), "muted": RGB(0x262626),
	"muted-foreground": RGB(0xa1a1a1), "accent-foreground": RGB(0xfafafa), "destructive": RGB(0xff6467),
	"input": RGBA(0xffffff, 0.15), "ring": RGB(0x737373), "chart-1": RGB(0x1447e6), "chart-2": RGB(0x00bc7d),
	"chart-3": RGB(0xfe9a00), "chart-4": RGB(0xad46ff), "chart-5": RGB(0xff2056), "sidebar": RGB(0x171717),
	"sidebar-foreground": RGB(0xfafafa), "sidebar-primary": RGB(0x1447e6), "sidebar-primary-foreground": RGB(0xfafafa),
	"sidebar-accent": RGB(0x262626), "sidebar-accent-foreground": RGB(0xfafafa), "sidebar-border": RGBA(0xffffff, 0.1),
	"sidebar-ring": RGB(0x737373),
}

// ParseTheme reads a theme file over DefaultTheme: radius, sans and
// mono, and light and dark maps of role names, as Palette.Color names
// them, to CSS colors. YAML reads JSON as well.
//
//	radius: 8
//	light:
//	  accent: "#007aff"
//	  inset: "#f1f1f1"
//	dark:
//	  accent: "#0a84ff"
//
// It reads a shadcn/ui theme file too, as NSH's apps name theirs: the
// CSS variables of cssVars' theme, light and dark sections, where a
// value may name another (var(--brand)) and dark's go over light's. A
// variable that is a color is a role of the palettes; the others are
// Tokens, radius and font-sans and font-mono setting Radius, Sans and
// Mono too.
func ParseTheme(data []byte) (*Theme, error) {
	var f struct {
		Radius  *float32          `yaml:"radius"`
		Sans    string            `yaml:"sans"`
		Mono    string            `yaml:"mono"`
		Light   map[string]string `yaml:"light"`
		Dark    map[string]string `yaml:"dark"`
		CSSVars struct {
			Theme map[string]string `yaml:"theme"`
			Light map[string]string `yaml:"light"`
			Dark  map[string]string `yaml:"dark"`
		} `yaml:"cssVars"`
	}
	if err := yaml.Unmarshal(data, &f); err != nil {
		return nil, fmt.Errorf("core: theme: %w", err)
	}
	t := DefaultTheme()
	if f.Radius != nil {
		t.Radius = Points(*f.Radius)
	}
	if f.Sans != "" {
		t.Sans = f.Sans
	}
	if f.Mono != "" {
		t.Mono = f.Mono
	}
	for _, m := range []struct {
		p      *Palette
		colors map[string]string
		name   string
	}{{&t.Light, f.Light, "light"}, {&t.Dark, f.Dark, "dark"}} {
		for name, v := range m.colors {
			c, err := ParseColor(v)
			if err != nil {
				return nil, fmt.Errorf("core: theme: %s.%s: %w", m.name, name, err)
			}
			m.p.SetColor(name, c)
		}
	}
	if v := f.CSSVars; len(v.Theme)+len(v.Light)+len(v.Dark) > 0 {
		if err := t.readVars(v.Theme, v.Light, v.Dark); err != nil {
			return nil, err
		}
	}
	return t, nil
}

// readVars sets t from a shadcn/ui theme's CSS variables.
func (t *Theme) readVars(theme, light, dark map[string]string) error {
	lightVars := maps.Clone(theme)
	if lightVars == nil {
		lightVars = map[string]string{}
	}
	maps.Copy(lightVars, light)
	darkVars := maps.Clone(lightVars)
	maps.Copy(darkVars, dark)
	for _, a := range []struct {
		p    *Palette
		vars map[string]string
	}{{&t.Light, lightVars}, {&t.Dark, darkVars}} {
		for name := range a.vars {
			v, err := resolveVar(a.vars, name, 0)
			if err != nil {
				return fmt.Errorf("core: theme: --%s: %w", name, err)
			}
			if c, err := ParseColor(v); err == nil {
				a.p.SetColor(name, c)
				continue
			}
			if a.p == &t.Light {
				if t.Tokens == nil {
					t.Tokens = map[string]string{}
				}
				t.Tokens[name] = v
			}
		}
	}
	if r, ok := t.Tokens["radius"]; ok {
		var px float32
		if _, err := fmt.Sscanf(strings.TrimSuffix(r, "px"), "%g", &px); err == nil {
			t.Radius = Points(px)
		}
	}
	if s, ok := t.Tokens["font-sans"]; ok {
		t.Sans = s
	}
	if m, ok := t.Tokens["font-mono"]; ok {
		t.Mono = m
	}
	return nil
}

// varRef finds var(--name) and var(--name, fallback).
var varRef = regexp.MustCompile(`var\(--([A-Za-z0-9_-]+)(?:\s*,\s*([^()]*))?\)`)

// resolveVar is variable name's value with the variables it names
// replaced by theirs.
func resolveVar(vars map[string]string, name string, depth int) (string, error) {
	if depth > 16 {
		return "", fmt.Errorf("variables name each other")
	}
	v, ok := vars[name]
	if !ok {
		return "", fmt.Errorf("no variable --%s", name)
	}
	var err error
	out := varRef.ReplaceAllStringFunc(v, func(m string) string {
		g := varRef.FindStringSubmatch(m)
		r, e := resolveVar(vars, g[1], depth+1)
		if e != nil {
			if g[2] != "" {
				return g[2]
			}
			err = e
		}
		return r
	})
	return out, err
}

// scopeTheme and scopeAppearance key the scope values SetTheme and
// SetSubtreeAppearance keep.
type (
	scopeTheme      struct{}
	scopeAppearance struct{}
)

// SetTheme gives cx and everything it builds from now on theme t; a
// nil t takes back what an earlier call gave, so the theme above shows.
// Called at the app's root it is the app's theme; a deeper call wins
// for its subtree, so a sidebar or a panel can take another.
func (cx *Scope) SetTheme(t *Theme) {
	t.noteColors()
	o := cx
	if s, ok := o.values[scopeTheme{}].(*Signal[*Theme]); ok {
		s.Set(t)
		return
	}
	if o.values == nil {
		o.values = map[any]any{}
	}
	o.values[scopeTheme{}] = NewSignal(o, t)
}

// SetSubtreeAppearance shows cx and everything it builds from now on
// light or dark whatever the appearance above it, the platform's views
// in it too where the platform scopes appearance to a view;
// AppearanceSystem follows the one above again. Scope.SetAppearance
// forces the app's.
func (cx *Scope) SetSubtreeAppearance(a Appearance) {
	o := cx
	if s, ok := o.values[scopeAppearance{}].(*Signal[Appearance]); ok {
		s.Set(a)
		return
	}
	if o.values == nil {
		o.values = map[any]any{}
	}
	o.values[scopeAppearance{}] = NewSignal(o, a)
	o.rt.subtreeAppearance = true
}

// Theme returns the theme of cx's subtree: the nearest SetTheme's, else
// DefaultTheme. Host tokens reach its colors through Palette. A binding
// that reads it runs again when it changes.
func (cx *Scope) Theme() *Theme {
	for s := cx.current(); s != nil; s = s.parent {
		if sig, ok := s.values[scopeTheme{}].(*Signal[*Theme]); ok {
			if t := sig.Get(); t != nil {
				return t
			}
		}
	}
	return cx.rt.defaultTheme()
}

// Dark reports whether cx's subtree shows dark: as the nearest
// SetSubtreeAppearance forces it, else as the app shows (Env.Dark).
func (cx *Scope) Dark() bool {
	switch subtreeAppearance(cx.current()) {
	case AppearanceLight:
		return false
	case AppearanceDark:
		return true
	}
	return cx.Env().Dark()
}

// subtreeAppearance is the appearance the nearest SetSubtreeAppearance
// above o forces, tracked, or AppearanceSystem.
func subtreeAppearance(o *Scope) Appearance {
	if !o.rt.subtreeAppearance {
		return AppearanceSystem
	}
	for s := o; s != nil; s = s.parent {
		if sig, ok := s.values[scopeAppearance{}].(*Signal[Appearance]); ok {
			if a := sig.Get(); a != AppearanceSystem {
				return a
			}
		}
	}
	return AppearanceSystem
}

// Palette returns the colors of cx's subtree: its theme's palette in the
// appearance it shows, with the host's tokens over it. A binding that
// reads it runs again when any of them changes.
func (cx *Scope) Palette() Palette {
	t, dark := cx.Theme(), cx.Dark()
	p := t.Light
	if dark {
		p = t.Dark
	}
	if tok := cx.Env().tokens.Get(); tok.any() {
		p = p.clone()
		tok.apply(&p, dark)
	}
	return p
}

func (rt *runtime) defaultTheme() *Theme {
	if rt.theme == nil {
		rt.theme = DefaultTheme()
	}
	return rt.theme
}

// hostColors are a host's Tokens read as colors.
type hostColors struct {
	values, dark map[string]Color
}

func (h hostColors) any() bool { return len(h.values) > 0 || len(h.dark) > 0 }

func (h hostColors) apply(p *Palette, dark bool) {
	for name, c := range h.values {
		p.SetColor(name, c)
	}
	if dark {
		for name, c := range h.dark {
			p.SetColor(name, c)
		}
	}
}

// parseTokens reads t's colors; a value that is no color is dropped.
func parseTokens(t Tokens) hostColors {
	read := func(m map[string]string) map[string]Color {
		var out map[string]Color
		for name, v := range m {
			c, err := ParseColor(v)
			if err != nil {
				continue
			}
			if out == nil {
				out = map[string]Color{}
			}
			out[name] = c
		}
		return out
	}
	return hostColors{values: read(t.Values), dark: read(t.Dark)}
}
