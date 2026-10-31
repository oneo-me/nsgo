package core

import (
	"slices"
	"testing"

	"github.com/oneo-me/nsgo/internal/driver"
)

// resolveFor resolves classes as a node in env would.
func resolveFor(t *testing.T, classes string, env classEnv) (Style, Style, classProps) {
	t.Helper()
	l := defaultTailwind.parse(classes)
	if len(l.unknown) > 0 {
		t.Fatalf("unknown classes %q", l.unknown)
	}
	return l.resolve(&env)
}

var narrow = classEnv{window: Size{Width: 390, Height: 844}, os: OSIOS}

func TestClassesSetWhatTailwindSets(t *testing.T) {
	s, _, _ := resolveFor(t, "flex flex-col items-center justify-between gap-4 p-4 px-2 mt-auto w-1/2 h-12 "+
		"bg-white text-lg font-bold text-zinc-900 rounded-lg border-2 border-red-500 opacity-50 z-10", narrow)
	want := Style{Display: DisplayFlex, Direction: Column, AlignItems: AlignCenter, JustifyContent: JustifySpaceBetween,
		Gap: 16, Padding: Edges{Top: 16, Right: 8, Bottom: 16, Left: 8}, AutoMargin: SideTop, Width: Pct(50),
		Height: Pt(48), Background: RGB(0xffffff), FontSize: 18, LineHeightMultiple: 1.75 / 1.125, FontWeight: 700,
		Color: tailwindPalette["zinc"][9], CornerRadius: 8, BorderWidth: 2, BorderColor: tailwindPalette["red"][5],
		Opacity: Alpha(0.5), ZIndex: 10}
	if s.Transition != want.Transition || !styleEqual(s, want) {
		t.Errorf("got\n%+v\nwant\n%+v", s, want)
	}
}

func styleEqual(a, b Style) bool {
	return a.Display == b.Display && a.Direction == b.Direction && a.AlignItems == b.AlignItems &&
		a.JustifyContent == b.JustifyContent && a.Gap == b.Gap && a.Padding == b.Padding &&
		a.AutoMargin == b.AutoMargin && a.Width == b.Width && a.Height == b.Height && a.Background == b.Background &&
		a.FontSize == b.FontSize && a.LineHeightMultiple == b.LineHeightMultiple && a.FontWeight == b.FontWeight &&
		a.Color == b.Color && a.CornerRadius == b.CornerRadius && a.BorderWidth == b.BorderWidth &&
		a.BorderColor == b.BorderColor && a.Opacity == b.Opacity && a.ZIndex == b.ZIndex
}

func TestFontVariantAndDecorationClasses(t *testing.T) {
	for classes, want := range map[string]FontVariant{
		"tabular-nums":                        FontTabularNums,
		"small-caps oldstyle-nums":            FontSmallCaps | FontOldstyleNums,
		"oldstyle-nums lining-nums":           FontLiningNums,
		"small-caps tabular-nums normal-nums": FontSmallCaps,
	} {
		s, _, _ := resolveFor(t, classes, narrow)
		if got := s.FontVariant &^ FontVariantNormal; got != want || s.FontVariant&FontVariantNormal == 0 {
			t.Errorf("%s: variant %v, want %v set", classes, s.FontVariant, want)
		}
	}
	for classes, want := range map[string][2]Length{
		"decoration-2 underline-offset-4":                                       {Pt(2), Pt(4)},
		"decoration-[3px] -underline-offset-2":                                  {Pt(3), Pt(-2)},
		"decoration-[10%] underline-offset-[0.5em]":                             {Pct(10), Pct(50)},
		"decoration-2 decoration-auto underline-offset-1 underline-offset-auto": {},
	} {
		s, _, _ := resolveFor(t, classes, narrow)
		if got := [2]Length{s.TextDecorationThickness, s.TextUnderlineOffset}; got != want {
			t.Errorf("%s: thickness and offset %v, want %v", classes, got, want)
		}
	}
	if s, _, _ := resolveFor(t, "decoration-red-500", narrow); s.TextDecorationColor != tailwindPalette["red"][5] ||
		!s.TextDecorationThickness.IsAuto() {
		t.Errorf("decoration-red-500 sets %v and %v", s.TextDecorationColor, s.TextDecorationThickness)
	}
}

func TestLaterClassesWinAndVariantsComeAfter(t *testing.T) {
	s, _, _ := resolveFor(t, "p-4 p-2", narrow)
	if s.Padding.Top != 8 {
		t.Errorf("p-4 p-2 pads %v", s.Padding.Top)
	}
	dark := narrow
	dark.dark = true
	s, _, _ = resolveFor(t, "dark:bg-black bg-white", dark)
	if s.Background != RGB(0) {
		t.Errorf("dark:bg-black bg-white is %#x in dark", s.Background)
	}
	s, _, _ = resolveFor(t, "dark:bg-black bg-white", narrow)
	if s.Background != RGB(0xffffff) {
		t.Errorf("dark:bg-black bg-white is %#x in light", s.Background)
	}
}

func TestBreakpointsFollowTheWindow(t *testing.T) {
	wide := narrow
	wide.window.Width = 800
	if s, _, _ := resolveFor(t, "w-full md:w-1/2 lg:w-1/3", wide); s.Width != Pct(50) {
		t.Errorf("at 800 points the width is %v", s.Width)
	}
	if s, _, _ := resolveFor(t, "w-full md:w-1/2 lg:w-1/3", narrow); s.Width != Pct(100) {
		t.Errorf("at 390 points the width is %v", s.Width)
	}
	if s, _, _ := resolveFor(t, "max-sm:hidden", narrow); s.Display != DisplayNone {
		t.Error("max-sm:hidden shows on a narrow")
	}
	if s, _, _ := resolveFor(t, "min-[300px]:p-1", narrow); s.Padding.Top != 4 {
		t.Error("min-[300px]:p-1 does not apply at 390 points")
	}
}

func TestStateVariantsAreStateStyles(t *testing.T) {
	s, _, _ := resolveFor(t, "bg-white hover:bg-zinc-100 active:bg-zinc-200 disabled:opacity-50 "+
		"group-hover:text-red-500 group-hover/item:underline", narrow)
	if s.Hover == nil || s.Hover.Background != tailwindPalette["zinc"][1] {
		t.Errorf("hover: %+v", s.Hover)
	}
	if s.Active == nil || s.Active.Background != tailwindPalette["zinc"][2] {
		t.Errorf("active: %+v", s.Active)
	}
	if s.Disabled == nil || s.Disabled.Opacity != Alpha(0.5) {
		t.Errorf("disabled: %+v", s.Disabled)
	}
	if g := s.GroupStyle; g == nil || g.Name != "item" || g.Hover == nil || g.Hover.TextDecoration != TextDecorationUnderline {
		t.Errorf("the named group's hover: %+v", g)
	}
}

func TestColorModifiersAndArbitraryValues(t *testing.T) {
	s, _, _ := resolveFor(t, "bg-black/50 text-[#123456] w-[37px] p-[13px] border-[3px] rounded-[6px] "+
		"text-[22px] leading-[1.2] tracking-[0.1em] grid-cols-[200px_1fr]", narrow)
	if s.Background != 0x00000080 {
		t.Errorf("bg-black/50 is %#x", s.Background)
	}
	if s.Color != RGB(0x123456) || s.Width != Pt(37) || s.Padding.Left != 13 || s.BorderWidth != 3 ||
		s.CornerRadius != 6 || s.FontSize != 22 || s.LineHeightMultiple != 1.2 {
		t.Errorf("arbitrary values: %+v", s)
	}
	if s.LetterSpacing < 2.19 || s.LetterSpacing > 2.21 {
		t.Errorf("tracking-[0.1em] at 22 points is %v", s.LetterSpacing)
	}
	if s.GridTemplateColumns != "200px 1fr" {
		t.Errorf("grid columns %q", s.GridTemplateColumns)
	}
	if s, _, _ := resolveFor(t, "bg-transparent", narrow); s.Background == 0 || s.Background&0xff != 0 {
		t.Errorf("bg-transparent is %#x: unset or visible", s.Background)
	}
}

func TestShadowsRingsFiltersAndGradients(t *testing.T) {
	s, _, _ := resolveFor(t, "shadow-md ring-2 ring-blue-500 blur-sm brightness-150 "+
		"bg-linear-to-r from-red-500 via-white to-blue-500/50", narrow)
	if len(s.Shadows) != 3 || s.Shadows[0].Spread != 2 || s.Shadows[0].Color != tailwindPalette["blue"][5] {
		t.Errorf("shadows %+v", s.Shadows)
	}
	if len(s.Filters) != 2 || s.Filters[0].Kind != FilterBlur || s.Filters[0].Amount != 8 ||
		s.Filters[1].Kind != FilterBrightness || s.Filters[1].Amount != 1.5 {
		t.Errorf("filters %+v", s.Filters)
	}
	if len(s.Gradients) != 1 {
		t.Fatalf("gradients %+v", s.Gradients)
	}
	g := s.Gradients[0]
	if g.Angle != 90 || len(g.Stops) != 3 || g.Stops[0].Color != tailwindPalette["red"][5] ||
		g.Stops[2].Color != withAlpha(tailwindPalette["blue"][5], 0.5) {
		t.Errorf("gradient %+v", g)
	}
}

func TestTransformsTransitionsAndAnimations(t *testing.T) {
	s, _, _ := resolveFor(t, "scale-95 -rotate-45 translate-x-4 -translate-y-1/2 origin-top-left "+
		"transition-colors duration-300 ease-out animate-spin", narrow)
	tr := s.Transform
	if tr.Scale != 0.95 || tr.Rotate != -45 || tr.TranslateX != 16 || tr.TranslateXPercent != 0 ||
		tr.TranslateY != 0 || tr.TranslateYPercent != -50 {
		t.Errorf("transform %+v", tr)
	}
	if s.Transition.Duration.Milliseconds() != 300 || s.Transition.Properties != TransitionBackground|TransitionBorder ||
		s.Transition.Easing != (Curve{X1: 0, Y1: 0, X2: 0.2, Y2: 1}) {
		t.Errorf("transition %+v", s.Transition)
	}
	if s.Animation.Keyframes != KeyframesSpin {
		t.Errorf("animation %+v", s.Animation)
	}
	s, _, _ = resolveFor(t, "translate-x-[20px] translate-y-full", narrow)
	if tr := s.Transform; tr.TranslateX != 20 || tr.TranslateXPercent != 0 || tr.TranslateY != 0 || tr.TranslateYPercent != 100 {
		t.Errorf("translate-x-[20px] translate-y-full: %+v", tr)
	}
}

func TestImportantClassesAndProps(t *testing.T) {
	_, important, p := resolveFor(t, "!p-1 line-clamp-2 pointer-events-none truncate", narrow)
	if important.Padding.Top != 4 {
		t.Errorf("!p-1: %+v", important.Padding)
	}
	if p.lines != 1 || p.ellipsize != EllipsizeTail || p.pointerEvents != PointerEventsNone {
		t.Errorf("props %+v", p)
	}
}

func TestUnknownClassesAreReported(t *testing.T) {
	l := defaultTailwind.parse("p-4 order-2 peer-hover:p-1 p-4.3 hover:bogus")
	if want := []string{"order-2", "peer-hover:p-1", "p-4.3", "hover:bogus"}; !slices.Equal(l.unknown, want) {
		t.Errorf("unknown %q, want %q", l.unknown, want)
	}
}

func TestThemeColorClassesFollowThePalette(t *testing.T) {
	pal := Palette{Accent: RGB(0x123456), TextMuted: RGB(0x654321)}
	env := narrow
	env.palette = &pal
	s, _, _ := resolveFor(t, "bg-accent text-text-muted/50", env)
	if s.Background != RGB(0x123456) || s.Color != withAlpha(RGB(0x654321), 0.5) {
		t.Errorf("theme colors: %#x %#x", s.Background, s.Color)
	}
}

func TestViewsTakeClassesUnderTheirStyle(t *testing.T) {
	h := newHost(t, Size{Width: 390, Height: 844}, func(cx *Scope) Node {
		return View{TestID: "box", Class: "p-4 bg-white dark:bg-black", Style: Style{Padding: Edges{Top: 1}}, Children: []Node{
			Text{TestID: "t", Class: "text-lg line-clamp-2", Content: "hi"},
		}}
	})
	h.commit()
	// The style's top padding over the classes', side by side.
	box := h.find("box")
	if box.style.Padding != (Edges{Top: 1, Right: 16, Bottom: 16, Left: 16}) || box.style.Background != RGB(0xffffff) {
		t.Errorf("the box's padding %+v and background %#x", box.style.Padding, box.style.Background)
	}
	if lines, _ := h.find("t").textLimits(); lines != 2 {
		t.Errorf("the text has %d lines", lines)
	}
	h.tree.SetEnvironment(driver.Environment{Dark: true})
	h.commit()
	if h.find("box").style.Background != RGB(0) {
		t.Errorf("in dark the box is %#x", h.find("box").style.Background)
	}
}

func TestClassFuncFollowsClassAndChanges(t *testing.T) {
	var on *Signal[bool]
	h := newHost(t, Size{Width: 390, Height: 844}, func(cx *Scope) Node {
		on = NewSignal(cx, false)
		return View{Children: []Node{
			View{TestID: "box", Class: "p-4 bg-white dark:bg-zinc-900", ClassFunc: func() string {
				if on.Get() {
					return "p-2 bg-black"
				}
				return ""
			}},
			Overlay{Class: "p-4", Children: []Node{View{TestID: "in"}}},
			Canvas{TestID: "canvas", Class: "w-8 h-8"},
		}}
	})
	h.commit()
	if box := h.find("box"); box.style.Padding.Top != 16 || box.style.Background != RGB(0xffffff) {
		t.Errorf("off: padding %v, background %#x", box.style.Padding.Top, box.style.Background)
	}
	on.Set(true)
	h.commit()
	if box := h.find("box"); box.style.Padding.Top != 8 || box.style.Background != RGB(0) {
		t.Errorf("on: padding %v, background %#x", box.style.Padding.Top, box.style.Background)
	}
	on.Set(false)
	h.tree.SetEnvironment(driver.Environment{Dark: true})
	h.commit()
	if box := h.find("box"); box.style.Padding.Top != 16 || box.style.Background != tailwindPalette["zinc"][9] {
		t.Errorf("off again in dark: padding %v, background %#x", box.style.Padding.Top, box.style.Background)
	}
	if p := h.find("in").parent.style.Padding.Top; p != 16 {
		t.Errorf("the overlay's padding is %v", p)
	}
	if s := h.find("canvas").style; s.Width != Pt(32) || s.Height != Pt(32) {
		t.Errorf("the canvas is %v by %v", s.Width, s.Height)
	}
}

func TestSpacingAndPositionsFollowTheChildren(t *testing.T) {
	var items *Signal[[]string]
	h := newHost(t, Size{Width: 390, Height: 844}, func(cx *Scope) Node {
		items = NewSignalFunc(cx, []string{"a", "b"}, nil)
		return View{TestID: "list", Class: "space-y-2", Children: []Node{
			Each(items.Get, func(s string) string { return s }, func(cx *Scope, item func() string, _ func() int) Node {
				return View{TestID: item(), Class: "h-4 first:bg-red-500 last:bg-blue-500"}
			}),
		}}
	})
	h.commit()
	if a, b := h.find("a"), h.find("b"); a.style.Margin.Bottom != 8 || b.style.Margin.Bottom != 0 ||
		a.style.Background != tailwindPalette["red"][5] || b.style.Background != tailwindPalette["blue"][5] {
		t.Errorf("a %+v %#x, b %+v %#x", a.style.Margin, a.style.Background, b.style.Margin, b.style.Background)
	}
	items.Set([]string{"a", "b", "c"})
	h.commit()
	if b, c := h.find("b"), h.find("c"); b.style.Margin.Bottom != 8 || b.style.Background != 0 ||
		c.style.Background != tailwindPalette["blue"][5] {
		t.Errorf("after an append: b %+v %#x, c %#x", b.style.Margin, b.style.Background, c.style.Background)
	}
}
