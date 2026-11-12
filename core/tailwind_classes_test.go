package core

import (
	"slices"
	"strings"
	"testing"
	"time"
)

// Tailwind classes: one test each, its assertions on Style. State classes
// are state styles rather than applied states, and a transform is fields
// rather than a list of steps.

// twEnv is a phone's light environment with DefaultTheme's palette, the
// design tokens these tests theme with.
func twEnv() classEnv {
	pal := DefaultTheme().Light
	return classEnv{window: Size{Width: 390, Height: 844}, os: OSIOS, palette: &pal, tw: defaultTailwind}
}

func twDark() classEnv {
	e := twEnv()
	pal := DefaultTheme().Dark
	e.dark, e.palette = true, &pal
	return e
}

func twWide(width Points) classEnv {
	e := twEnv()
	e.window = Size{Width: width, Height: 844}
	return e
}

// tw resolves classes in env, failing on unknown ones.
func tw(t *testing.T, classes string, env ...classEnv) (Style, classProps) {
	t.Helper()
	e := twEnv()
	if len(env) > 0 {
		e = env[0]
	}
	l := e.tw.parse(classes)
	if len(l.unknown) > 0 {
		t.Fatalf("%q: unknown classes %q", classes, l.unknown)
	}
	normal, important, p := l.resolve(&e)
	return mergeStyles(normal, important), p
}

func styled(t *testing.T, classes string, env ...classEnv) Style {
	t.Helper()
	s, _ := tw(t, classes, env...)
	return s
}

func unknownOf(theme *tailwindTheme, classes string) []string {
	return theme.parse(classes).unknown
}

func want[T comparable](t *testing.T, what string, got, want T) {
	t.Helper()
	if got != want {
		t.Errorf("%s: got %v, want %v", what, got, want)
	}
}

func TestSpacingAndLayout(t *testing.T) {
	s := styled(t, "flex flex-row items-center justify-between gap-3 p-4 px-2 -mt-1 w-1/2 h-12 min-w-0")
	want(t, "direction", s.Direction, Row)
	want(t, "align", s.AlignItems, AlignCenter)
	want(t, "justify", s.JustifyContent, JustifySpaceBetween)
	want(t, "gap", s.Gap, 12)
	want(t, "padding", s.Padding, Edges{Top: 16, Right: 8, Bottom: 16, Left: 8})
	want(t, "margin", s.Margin.Top, -4)
	want(t, "width", s.Width, Pct(50))
	want(t, "height", s.Height, Pt(48))
	want(t, "flex-[2]", styled(t, "flex-[2]").FlexGrow, 2)
	s = styled(t, "flex-[3_0_50%]")
	want(t, "flex-[3_0_50%]", [3]any{s.FlexGrow, s.FlexShrink, s.FlexBasis}, [3]any{float32(3), float32(0), Pct(50)})
	s = styled(t, "p-[13px] m-auto inset-0 absolute size-10 basis-1/3 flex-1")
	want(t, "p-[13px]", s.Padding.Left, 13)
	want(t, "m-auto", s.AutoMargin, SideTop|SideRight|SideBottom|SideLeft)
	want(t, "absolute", s.Position, Absolute)
	want(t, "inset-0", s.Right, Pt(0))
	want(t, "size-10", s.Height, Pt(40))
	want(t, "flex-1", s.FlexGrow, 1)
}

func TestPhysicalClassesNameTheOtherSideRightToLeft(t *testing.T) {
	rtl := twEnv()
	rtl.rtl = true
	s := styled(t, "pl-4 mr-2 left-1", rtl)
	want(t, "pl-4", s.Padding.Right, 16)
	want(t, "mr-2", s.Margin.Left, 8)
	want(t, "left-1", s.Right, Pt(4))
	want(t, "pl-4 ltr", styled(t, "pl-4").Padding.Left, 16)
	if !defaultTailwind.parse("pl-4 mr-2 left-1").env {
		t.Error("physical classes do not follow the direction")
	}
	if defaultTailwind.parse("ps-4 me-2 start-1 rounded-s-lg border-e-2 text-end text-center px-2").env {
		t.Error("logical classes follow the direction")
	}
}

func TestSpaceGivesTheChildrenMargins(t *testing.T) {
	s, p := tw(t, "gap-2 space-x-4 -space-y-px")
	want(t, "gap", s.Gap, 8)
	want(t, "margin", s.Margin.Right, 0)
	want(t, "space-x", [2]any{p.hasSpaceX, p.spaceX}, [2]any{true, Points(16)})
	want(t, "space-y", [2]any{p.hasSpaceY, p.spaceY}, [2]any{true, Points(-1)})
	want(t, "reverse", p.reverseX, false)
	_, p = tw(t, "space-y-reverse space-y-2")
	want(t, "space-y-reverse", p.reverseY, true)
}

func TestWidthsWithoutAColorAreCurrentColor(t *testing.T) {
	red := tailwindPalette["red"][5]
	s := styled(t, "border-t text-red-500 inset-ring")
	want(t, "border", borderTop(s), red)
	s = styled(t, "border-sky-500 border-2 outline-red-500 outline-dashed")
	want(t, "border-sky-500", borderTop(s), tailwindPalette["sky"][5])
	want(t, "outline-red-500", s.OutlineColor, red)
}

func TestLaterClassesWin(t *testing.T) {
	s := styled(t, "p-4 p-2 bg-red-500 bg-blue-500")
	want(t, "padding", s.Padding.Top, 8)
	want(t, "background", s.Background, RGB(0x2b7fff))
	s = styled(t, "active:bg-red-500 bg-blue-500")
	if s.Active == nil || s.Active.Background != RGB(0xfb2c36) {
		t.Errorf("active: %+v", s.Active)
	}
}

func TestColorsAndOpacity(t *testing.T) {
	s := styled(t, "bg-white/50 text-zinc-900 border border-[#ff0000] opacity-75")
	want(t, "bg-white/50", s.Background, Color(0xffffff80))
	want(t, "text", s.Color, Color(0x18181bff))
	want(t, "border", s.BorderWidth, 1)
	want(t, "border color", s.BorderColor, Color(0xff0000ff))
	want(t, "opacity", s.Opacity, Alpha(0.75))
	for _, classes := range []string{"border-2 border-none", "border-none border-2", "border-hidden border-t-4"} {
		s := styled(t, classes)
		if s.BorderStyle != BorderNone && s.BorderStyle != BorderHidden {
			t.Errorf("%s: border style %v", classes, s.BorderStyle)
		}
	}
	s = styled(t, "border border-none md:border-solid", twWide(800))
	want(t, "md:border-solid", s.BorderStyle, BorderSolid)
	s = styled(t, "bg-[rgb(0_0_255)]/25 text-black/[0.5]")
	want(t, "bg-[rgb]/25", s.Background, Color(0x0000ff40))
	want(t, "text-black/[0.5]", s.Color, Color(0x00000080))
}

func TestThemeTokens(t *testing.T) {
	light, dark := DefaultTheme().Light, DefaultTheme().Dark
	token := func(p Palette, name string) Color {
		c, ok := p.Color(name)
		if !ok {
			t.Fatalf("no token %s", name)
		}
		return c
	}
	s := styled(t, "bg-primary text-muted-foreground border border-border rounded-(--radius)")
	want(t, "bg-primary", s.Background, token(light, "primary"))
	want(t, "text-muted-foreground", s.Color, token(light, "muted-foreground"))
	want(t, "border-border", s.BorderColor, token(light, "border"))
	s = styled(t, "bg-primary text-(--muted-foreground)", twDark())
	want(t, "dark bg-primary", s.Background, token(dark, "primary"))
	want(t, "dark text-(--muted-foreground)", s.Color, token(dark, "muted-foreground"))
	s = styled(t, "bg-primary/50 border-t-2 border-t-ring")
	want(t, "bg-primary/50", s.Background, token(light, "primary").Faded(0.5))
	want(t, "border-t-ring", borderTop(s), token(light, "ring"))
	s, _ = tw(t, "shadow-md shadow-primary ring-2 ring-ring bg-linear-to-r from-primary to-accent caret-primary placeholder:text-muted-foreground")
	want(t, "caret", s.CaretColor, token(light, "primary"))
	want(t, "placeholder", s.PlaceholderColor, token(light, "muted-foreground"))
	if !slices.ContainsFunc(s.Shadows, func(b BoxShadow) bool { return b.Color == token(light, "ring") }) {
		t.Errorf("no ring of the ring token: %+v", s.Shadows)
	}
	if !slices.ContainsFunc(s.Shadows, func(b BoxShadow) bool { return b.Color>>8 == token(light, "primary")>>8 }) {
		t.Errorf("no shadow of the primary token: %+v", s.Shadows)
	}
	g := s.Gradients[0]
	want(t, "from-primary", g.Stops[0].Color, token(light, "primary"))
	want(t, "to-accent", g.Stops[len(g.Stops)-1].Color, token(light, "accent"))
	want(t, "dark:bg-accent", styled(t, "dark:bg-accent", twDark()).Background, token(dark, "accent"))
	want(t, "bg-primary bg-red-500", styled(t, "bg-primary bg-red-500", twDark()).Background, RGB(0xfb2c36))
	if l := defaultTailwind.parse("bg-primary p-4"); !l.env {
		t.Error("tokens do not follow the theme")
	}
	if l := defaultTailwind.parse("bg-red-500"); l.env {
		t.Error("a palette color follows the theme")
	}
	want(t, "unknown", len(unknownOf(defaultTailwind, "bg-nope text-(--nope) w-primary")), 3)
	if got := UnknownClasses("p-4 bg-nope flex-row"); !slices.Equal(got, []string{"bg-nope"}) {
		t.Errorf("UnknownClasses = %v", got)
	}
}

func TestTailwindColorsWinOverTokensAndTokensComeFromTheTheme(t *testing.T) {
	tws, err := ParseTailwind([]byte(`{ "colors": { "primary": "#ff0000" } }`))
	if err != nil {
		t.Fatal(err)
	}
	env := twEnv()
	env.tw = tws.t
	env.palette.SetColor("chart-up", RGB(0x00ff00))
	want(t, "bg-primary", styled(t, "bg-primary", env).Background, RGB(0xff0000))
	want(t, "bg-chart-up", styled(t, "bg-chart-up", env).Background, RGB(0x00ff00))
}

func TestTypography(t *testing.T) {
	s := styled(t, "text-lg font-semibold italic tracking-wide leading-tight underline decoration-wavy decoration-red-500 uppercase text-center tabular-nums")
	want(t, "size", s.FontSize, 18)
	want(t, "weight", s.FontWeight, 600)
	want(t, "italic", s.FontStyle, FontStyleItalic)
	want(t, "leading", s.LineHeightMultiple, 1.25)
	want(t, "underline", s.TextDecoration&TextDecorationUnderline != 0, true)
	want(t, "wavy", s.TextDecorationStyle, DecorationWavy)
	want(t, "decoration color", s.TextDecorationColor, RGB(0xfb2c36))
	want(t, "align", s.TextAlign, TextAlignCenter)
	want(t, "tabular", s.FontVariant&FontTabularNums != 0, true)
	s = styled(t, "text-base leading-[1.5]")
	want(t, "leading-[1.5]", [2]any{s.LineHeightMultiple, s.LineHeight}, [2]any{float32(1.5), Points(0)})
	want(t, "text-sm/[1.25]", styled(t, "text-sm/[1.25]").LineHeightMultiple, 1.25)
	want(t, "leading-[150%]", styled(t, "leading-[150%]").LineHeightEm, 1.5)
	want(t, "leading-[2em]", styled(t, "leading-[2em]").LineHeightEm, 2)
	want(t, "leading-[22px]", styled(t, "leading-[22px]").LineHeight, 22)
	want(t, "leading-6", styled(t, "leading-6").LineHeight, 24)
	s = styled(t, "leading-tight text-lg")
	want(t, "leading-tight text-lg", [2]any{s.FontSize, s.LineHeightMultiple}, [2]any{Points(18), float32(1.25)})
	want(t, "md:text-lg", styled(t, "leading-tight md:text-lg", twWide(800)).LineHeightMultiple, 1.25)
	want(t, "text-lg", styled(t, "text-lg").LineHeightMultiple, 1.75/1.125)
	want(t, "text-lg/6", styled(t, "text-lg/6").LineHeight, 24)
	want(t, "normal-nums", styled(t, "small-caps tabular-nums normal-nums").FontVariant&^FontVariantNormal, FontSmallCaps)
	s = styled(t, "text-sm/6 font-mono")
	want(t, "text-sm/6", [2]any{s.FontSize, s.LineHeight}, [2]any{Points(14), Points(24)})
	want(t, "font-mono", s.FontFamily, "monospace")
	want(t, "truncate", styled(t, "truncate").NumberOfLines, 1)
}

func TestShadowsAndRings(t *testing.T) {
	s := styled(t, "shadow-md")
	want(t, "layers", len(s.Shadows), 2)
	want(t, "offset", s.Shadows[0].Offset.Y, 4)
	want(t, "spread", s.Shadows[0].Spread, -1)
	s = styled(t, "shadow-lg shadow-blue-500/50 ring-2 ring-offset-2 ring-red-500")
	if len(s.Shadows) != 4 {
		t.Fatalf("ring offset, ring and two layers: %+v", s.Shadows)
	}
	want(t, "offset spread", s.Shadows[0].Spread, 2)
	want(t, "offset color", s.Shadows[0].Color, RGB(0xffffff))
	want(t, "ring spread", s.Shadows[1].Spread, 4)
	want(t, "ring color", s.Shadows[1].Color, RGB(0xfb2c36))
	want(t, "shadow color", s.Shadows[2].Color, Color(0x2b7fff80))
	want(t, "shadow-red-500", styled(t, "shadow-sm shadow-red-500").Shadows[0].Color, RGB(0xfb2c36))
	want(t, "shadow-none", len(styled(t, "shadow-md shadow-none").Shadows), 0)
	want(t, "shadow-xl/20", styled(t, "shadow-xl/20").Shadows[0].Color, Color(0x00000033))
	want(t, "shadow-red-500/50 shadow-xl/20", styled(t, "shadow-red-500/50 shadow-xl/20").Shadows[0].Color, Color(0xfb2c361a))
	want(t, "arbitrary", styled(t, "shadow-[0_35px_35px_red]/20").Shadows[0].Color, Color(0xff000033))
	red := RGB(0xfb2c36)
	s = styled(t, "text-red-500 shadow-md shadow-current/50 ring-2 ring-offset-2 ring-offset-current")
	want(t, "ring-offset-current", s.Shadows[0].Color, red)
	want(t, "shadow-current/50", s.Shadows[2].Color, red.Faded(0.5))
	s = styled(t, "text-red-500 bg-current/50 decoration-current")
	want(t, "bg-current/50", s.Background, red.Faded(0.5))
	want(t, "decoration-current", s.TextDecorationColor, red)
	s = styled(t, "text-red-500 drop-shadow-md drop-shadow-current")
	want(t, "drop-shadow-current", s.Filters[0].Shadow.Color, red)
	s = styled(t, "drop-shadow-blue-500/50 drop-shadow-lg")
	want(t, "drop-shadow-blue-500/50", s.Filters[0].Shadow.Color, Color(0x2b7fff80))
}

func TestGradients(t *testing.T) {
	s := styled(t, "bg-linear-to-r from-blue-500 via-purple-500 via-40% to-pink-500")
	g := s.Gradients[0]
	want(t, "angle", g.Angle, 90)
	blue, purple, pink := RGB(0x2b7fff), RGB(0xad46ff), RGB(0xf6339a)
	want(t, "first", [2]any{g.Stops[0].Color, g.Stops[0].At}, [2]any{blue, Pct(0)})
	via := slices.IndexFunc(g.Stops, func(s ColorStop) bool { return s.Color == purple })
	if via < 0 || g.Stops[via].At != Pct(40) {
		t.Errorf("via-40%%: %+v", g.Stops)
	}
	last := g.Stops[len(g.Stops)-1]
	want(t, "last", [2]any{last.Color, last.At}, [2]any{pink, Pct(100)})
	want(t, "in oklab", g.Space, ColorSpaceOklab)
	g = styled(t, "bg-linear-to-br from-red-500").Gradients[0]
	want(t, "to-br", g.ToCorner, ToBottomRight)
	want(t, "transparent end", g.Stops[1].Color&0xff, 0)
	g = styled(t, "bg-[linear-gradient(45deg,red,blue)]").Gradients[0]
	want(t, "arbitrary", [2]any{g.Angle, len(g.Stops)}, [2]any{float32(45), 2})
	g = styled(t, "bg-[repeating-linear-gradient(90deg,red_0px,blue_10px)]").Gradients[0]
	want(t, "repeating", [2]any{g.Repeating, g.Stops[1].At}, [2]any{true, Pt(10)})
}

func TestStateBackgroundsShowOverGradients(t *testing.T) {
	s := styled(t, "bg-linear-to-b from-zinc-700 to-zinc-800 active:bg-zinc-600")
	if len(s.Gradients) == 0 || s.Active == nil || s.Active.Background != tailwindPalette["zinc"][6] {
		t.Fatalf("active: %+v", s.Active)
	}
	if s.Active.Gradients == nil || len(s.Active.Gradients) != 0 {
		t.Errorf("the pressed background does not hide the gradient: %+v", s.Active.Gradients)
	}
	s = styled(t, "bg-linear-to-b from-zinc-700 active:bg-zinc-600 active:from-zinc-500")
	if len(s.Active.Gradients) != 1 {
		t.Errorf("a state's own gradient: %+v", s.Active.Gradients)
	}
	if s := styled(t, "bg-zinc-600 bg-linear-to-b from-zinc-700"); len(s.Gradients) != 1 {
		t.Error("a plain color hides the gradient")
	}
}

func TestTransforms(t *testing.T) {
	tr := styled(t, "-translate-x-1/2 translate-y-2 rotate-45 scale-110").Transform
	want(t, "translate-x", tr.TranslateXPercent, -50)
	want(t, "translate-y", tr.TranslateY, 8)
	want(t, "rotate", tr.Rotate, 45)
	want(t, "scale", tr.Scale, 1.1)
	tr = styled(t, "translate-x-2 rotate-12 scale-90 rotate-x-45 skew-x-6 transform-none").Transform
	want(t, "transform-none", tr, Transform{TranslateX: 8, Rotate: 12, Scale: 0.9})
	tr = styled(t, "translate-x-2 rotate-12 scale-90 rotate-x-45 translate-none scale-none rotate-none").Transform
	want(t, "-none", tr, Transform{RotateX: 45})
	s := styled(t, "perspective-near rotate-x-45")
	want(t, "perspective", [2]any{s.Perspective, s.Transform}, [2]any{Points(300), Transform{RotateX: 45}})
	tr = styled(t, "translate-x-1 translate-y-2 rotate-3 scale-x-50 scale-y-75 rotate-x-6 rotate-y-12 skew-x-1 skew-y-2").Transform
	want(t, "every step", tr, Transform{TranslateX: 4, TranslateY: 8, Rotate: 3, ScaleX: 0.5, ScaleY: 0.75, RotateX: 6,
		RotateY: 12, SkewX: 1, SkewY: 2})
}

func TestEffectsAndInteraction(t *testing.T) {
	s := styled(t, "blur-sm brightness-125 backdrop-blur-md mix-blend-multiply outline-2 outline-offset-2 outline-blue-500 invisible cursor-pointer")
	want(t, "filters", len(s.Filters), 2)
	want(t, "blur", s.Filters[0], Filter{Kind: FilterBlur, Amount: 8})
	want(t, "brightness", s.Filters[1], Filter{Kind: FilterBrightness, Amount: 1.25})
	want(t, "backdrop", len(s.BackdropFilters), 1)
	want(t, "backdrop blur", s.BackdropFilters[0], Filter{Kind: FilterBlur, Amount: 12})
	want(t, "blend", s.BlendMode, BlendMultiply)
	f := styled(t, "drop-shadow blur-sm").Filters
	if len(f) != 3 || f[0].Kind != FilterBlur || f[1].Kind != FilterDropShadow || f[2].Kind != FilterDropShadow {
		t.Errorf("drop-shadow blur-sm: %+v", f)
	}
	want(t, "drop-shadow-md", len(styled(t, "drop-shadow-md").Filters), 1)
	want(t, "two drops", len(styled(t, "drop-shadow-[0_1px_1px_#000,0_2px_4px_#f00]").Filters), 2)
	want(t, "drop-shadow-none", len(styled(t, "drop-shadow drop-shadow-none").Filters), 0)
	want(t, "outline", [2]any{s.OutlineWidth, s.OutlineOffset}, [2]any{Points(2), Points(2)})
	want(t, "invisible", s.Visibility, VisibilityHidden)
	outline := func(classes string) [2]any {
		s := styled(t, classes)
		style := s.OutlineStyle
		if style == 0 {
			style = BorderSolid
		}
		return [2]any{style, s.OutlineWidth}
	}
	want(t, "outline-2", outline("outline-2"), [2]any{BorderSolid, Points(2)})
	want(t, "outline", outline("outline"), [2]any{BorderSolid, Points(1)})
	want(t, "outline-dashed outline-2", outline("outline-dashed outline-2"), [2]any{BorderDashed, Points(2)})
	want(t, "outline-2 outline-dotted", outline("outline-2 outline-dotted"), [2]any{BorderDotted, Points(2)})
	want(t, "outline-none", outline("outline-none outline-2")[1], any(Points(0)))
	want(t, "outline-hidden", outline("outline-2 outline-hidden")[1], any(Points(0)))
	want(t, "outline-dashed", outline("outline-dashed"), [2]any{BorderDashed, Points(3)})
	want(t, "outline-solid", outline("outline-solid"), [2]any{BorderSolid, Points(3)})
	f2 := styled(t, "outline-2 focus:outline-dashed")
	if f2.Focus == nil || f2.Focus.OutlineStyle != BorderDashed {
		t.Errorf("focus:outline-dashed: %+v", f2.Focus)
	}
	want(t, "cursor", s.Cursor, CursorPointer)
}

func TestTransitionsAndAnimations(t *testing.T) {
	tr := styled(t, "transition-colors duration-300 ease-out delay-75").Transition
	want(t, "duration", tr.Duration, 300*time.Millisecond)
	want(t, "delay", tr.Delay, 75*time.Millisecond)
	want(t, "easing", tr.Easing, Curve{X1: 0, Y1: 0, X2: 0.2, Y2: 1})
	want(t, "colors", tr.Properties&TransitionBackground != 0, true)
	want(t, "not transform", tr.Properties&TransitionTransform != 0, false)
	want(t, "transition", styled(t, "transition").Transition.Duration, 150*time.Millisecond)
	want(t, "transition leaves layout", styled(t, "transition").Transition.Properties&TransitionLayout != 0, false)
	want(t, "transition-all moves layout", styled(t, "transition-all").Transition.Properties&TransitionLayout != 0, true)
	want(t, "animate-spin", styled(t, "animate-spin").Animation.Keyframes, KeyframesSpin)
}

func TestDisplay(t *testing.T) {
	dir := func(classes string, env ...classEnv) FlexDirection { return styled(t, classes, env...).Direction }
	want(t, "flex", styled(t, "flex").Display, DisplayFlex)
	want(t, "flex row", dir("flex"), Row)
	want(t, "inline-flex", dir("inline-flex"), Row)
	want(t, "flex-col flex", dir("flex-col flex"), Column)
	want(t, "flex flex-col", dir("flex flex-col"), Column)
	want(t, "flex-col-reverse flex", dir("flex-col-reverse flex"), ColumnReverse)
	if d := dir("items-center"); d != 0 && d != Column {
		t.Errorf("items-center: %v", d)
	}
	want(t, "md:flex", dir("hidden md:flex", twWide(800)), Row)
	want(t, "flex-col md:flex", dir("flex-col md:flex", twWide(800)), Column)
	want(t, "hidden", styled(t, "hidden md:flex").Display, DisplayNone)
	want(t, "block", styled(t, "block").Display, DisplayBlock)
	want(t, "flow-root", styled(t, "flow-root").Display, DisplayBlock)
	want(t, "inline-block", styled(t, "inline-block").Display, DisplayBlock)
	want(t, "inline", styled(t, "inline").Display, DisplayBlock)
	want(t, "md:inline-block", styled(t, "hidden md:inline-block", twWide(800)).Display, DisplayBlock)
}

func TestGrid(t *testing.T) {
	s := styled(t, "grid grid-cols-3 gap-2 col-span-2")
	want(t, "grid", s.Display, DisplayGrid)
	want(t, "cols", s.GridTemplateColumns, "repeat(3, minmax(0, 1fr))")
	want(t, "span", s.GridColumn.Start, GridSpan(2))
	s = styled(t, "grid grid-cols-[[a]_40px_[b]_1fr] col-[a/b] row-start-[main]")
	want(t, "named tracks", s.GridTemplateColumns, "[a] 40px [b] 1fr")
	want(t, "col start", s.GridColumn.Start, GridNamed("a", 0))
	want(t, "col end", s.GridColumn.End, GridNamed("b", 0))
	want(t, "row start", s.GridRow.Start, GridNamed("main", 0))
	want(t, "unknown", len(unknownOf(defaultTailwind, "col-[1_2] row-start-[1a] auto-rows-[3zz] -col-start-[a]")), 2)
	s = styled(t, "col-[1_2] auto-rows-[3zz]")
	want(t, "invalid placements set nothing", [2]any{s.GridColumn, s.GridAutoRows}, [2]any{GridLines{}, GridTracks("")})
}

func TestAlignmentFollowsTailwindCSS(t *testing.T) {
	for class, w := range map[string][2]any{
		"justify-start": {Align(0), JustifyStart}, "justify-end": {Align(0), JustifyEnd},
		"justify-center": {Align(0), JustifyCenter}, "justify-between": {Align(0), JustifySpaceBetween},
		"justify-around": {Align(0), JustifySpaceAround}, "justify-evenly": {Align(0), JustifySpaceEvenly},
		"justify-stretch": {Align(0), JustifyStretch}, "justify-normal": {Align(0), Justify(0)},
		// align-content: normal is stretch in flex and grid containers.
		"content-normal": {AlignStretch, Justify(0)}, "content-start": {AlignStart, Justify(0)},
		"content-end": {AlignEnd, Justify(0)}, "content-center": {AlignCenter, Justify(0)},
		"content-between": {AlignSpaceBetween, Justify(0)}, "content-around": {AlignSpaceAround, Justify(0)},
		"content-evenly": {AlignSpaceEvenly, Justify(0)}, "content-stretch": {AlignStretch, Justify(0)},
		"place-content-center": {AlignCenter, JustifyCenter}, "place-content-start": {AlignStart, JustifyStart},
		"place-content-end": {AlignEnd, JustifyEnd}, "place-content-between": {AlignSpaceBetween, JustifySpaceBetween},
		"place-content-stretch": {AlignStretch, JustifyStretch},
	} {
		s := styled(t, class)
		want(t, class, [2]any{s.AlignContent, s.JustifyContent}, w)
	}
	for class, w := range map[string][2]Align{
		"items-start": {AlignStart, 0}, "items-end": {AlignEnd, 0}, "items-center": {AlignCenter, 0},
		"items-baseline": {AlignBaseline, 0}, "items-stretch": {AlignStretch, 0},
		"justify-items-start": {0, AlignStart}, "justify-items-end": {0, AlignEnd},
		"justify-items-center": {0, AlignCenter}, "justify-items-stretch": {0, AlignStretch},
		"justify-items-normal": {0, 0}, "place-items-center": {AlignCenter, AlignCenter},
		"place-items-start": {AlignStart, AlignStart}, "place-items-end": {AlignEnd, AlignEnd},
		"place-items-stretch": {AlignStretch, AlignStretch},
	} {
		s := styled(t, class)
		want(t, class, [2]Align{s.AlignItems, s.JustifyItems}, w)
	}
	for class, w := range map[string][2]Align{
		"self-auto": {0, 0}, "self-start": {AlignStart, 0}, "self-end": {AlignEnd, 0},
		"self-center": {AlignCenter, 0}, "self-stretch": {AlignStretch, 0}, "self-baseline": {AlignBaseline, 0},
		"justify-self-auto": {0, 0}, "justify-self-start": {0, AlignStart}, "justify-self-end": {0, AlignEnd},
		"justify-self-center": {0, AlignCenter}, "justify-self-stretch": {0, AlignStretch},
		"place-self-auto": {0, 0}, "place-self-start": {AlignStart, AlignStart},
		"place-self-end": {AlignEnd, AlignEnd}, "place-self-center": {AlignCenter, AlignCenter},
		"place-self-stretch": {AlignStretch, AlignStretch},
	} {
		s := styled(t, class)
		want(t, class, [2]Align{s.AlignSelf, s.JustifySelf}, w)
	}
}

func TestGridUtilitiesFollowTailwindCSS(t *testing.T) {
	s := styled(t, "col-[number:2] row-span-[number:3] auto-cols-[length:13px]")
	want(t, "col", s.GridColumn.Start, GridLine(2))
	want(t, "row span", s.GridRow.Start, GridSpan(3))
	want(t, "auto cols", s.GridAutoColumns, "13px")
	want(t, "-col-[2]", styled(t, "-col-[2]").GridColumn.Start, GridLine(-2))
	want(t, "auto-rows-[number:2]", styled(t, "auto-rows-[number:2]").GridAutoRows, "")
	negatives := "-grid-cols-2 -grid-rows-[13px] -col-span-2 -row-span-1 -auto-cols-[13px] -auto-rows-fr -col-auto"
	want(t, "negatives", len(unknownOf(defaultTailwind, negatives)), 7)
}

func TestVariants(t *testing.T) {
	l := defaultTailwind.parse("bg-white dark:bg-black md:p-8 hover:opacity-80 group-active:scale-95 ios:pt-1 android:pt-2 rtl:ml-2")
	if len(l.unknown) > 0 || !l.env {
		t.Errorf("unknown %q, env %v", l.unknown, l.env)
	}
	android := twDark()
	android.os = OSAndroid
	want(t, "dark:", styled(t, "bg-white dark:bg-black", android).Background, RGB(0))
	want(t, "md: narrow", styled(t, "p-2 md:p-8").Padding.Top, 8)
	want(t, "md: wide", styled(t, "p-2 md:p-8", twWide(1024)).Padding.Top, 32)
	want(t, "max-md:", styled(t, "p-2 max-md:p-1").Padding.Top, 4)
	want(t, "android:", styled(t, "pt-1 ios:pt-3 android:pt-5", android).Padding.Top, 20)
	web := twEnv()
	web.os = OSWeb
	want(t, "web:", styled(t, "pt-1 ios:pt-3 web:pt-5", web).Padding.Top, 20)
	s := styled(t, "w-screen h-screen")
	want(t, "screen", [2]Length{s.Width, s.Height}, [2]Length{Pt(390), Pt(844)})
}

func TestCheckedVariants(t *testing.T) {
	s := styled(t, "bg-white checked:bg-black")
	if s.Checked == nil || s.Checked.Background != RGB(0) {
		t.Errorf("checked: %+v", s.Checked)
	}
	// selected: and the interaction states go over a value state.
	s = styled(t, "selected:bg-white hover:bg-white checked:bg-black")
	if s.Selected == nil || s.Hover == nil || s.Checked == nil {
		t.Fatalf("states: %+v", s)
	}
}

func TestIndeterminateVariants(t *testing.T) {
	s := styled(t, "bg-white checked:bg-black indeterminate:bg-red-500")
	if s.Indeterminate == nil || s.Indeterminate.Background != RGB(0xfb2c36) || s.Checked.Background != RGB(0) {
		t.Errorf("indeterminate: %+v %+v", s.Indeterminate, s.Checked)
	}
	if len(unknownOf(defaultTailwind, "indeterminate:bg-black group-indeterminate:opacity-50")) > 0 {
		t.Error("unknown")
	}
}

func TestGroups(t *testing.T) {
	want(t, "groups", strings.Join(defaultTailwind.parse("group/card p-2").groups, ","), "card")
	s := styled(t, "opacity-100 group-hover/card:opacity-50")
	if s.GroupStyle == nil || s.GroupStyle.Name != "card" || s.GroupStyle.Hover == nil ||
		s.GroupStyle.Hover.Opacity != Alpha(0.5) {
		t.Errorf("group-hover/card: %+v", s.GroupStyle)
	}
	last := twEnv()
	last.position = positionLast
	s = styled(t, "last:opacity-25 group-hover/card:opacity-50", last)
	want(t, "last:", s.Opacity, Alpha(0.25))
}

func TestUnknownUtilitiesAndVariantsAreReported(t *testing.T) {
	want(t, "unknown", strings.Join(unknownOf(defaultTailwind, "p-4 bg-nope-500 flexx hover:wat antialiased"), " "),
		"bg-nope-500 flexx hover:wat")
}

func TestClassesWithoutANativeMeaningAreUnknown(t *testing.T) {
	allUnknown := func(classes string) {
		t.Helper()
		got := unknownOf(defaultTailwind, classes)
		if !slices.Equal(got, strings.Fields(classes)) {
			t.Errorf("known of %q: %q", classes, slices.DeleteFunc(strings.Fields(classes), func(c string) bool {
				return slices.Contains(got, c)
			}))
		}
	}
	allUnknown("transform-3d order-1 order-first -order-2 order-[3] select-all border-double outline-double")
	allUnknown("min-w-fit min-h-max max-w-min max-h-fit")
	allUnknown("snap-x snap-center scroll-smooth scroll-mt-4 scroll-px-2 overscroll-none touch-none isolate md:snap-x scroll-anything")
	allUnknown("-gap-4 -gap-x-2 -space-x-1/2 -w-4 -h-2 -size-4 -min-w-0 -max-w-sm -basis-4 -flex-1 -grow -opacity-50 -text-lg -text-red-500 -bg-red-500 -rounded-lg -border-2 -blur-sm -p-4 -px-2 -leading-6 -shadow-md -ring-2 -from-red-500 -perspective-near -duration-100")
	allUnknown("p-1/2 p-full px-1/3 m-full m-1/2 -m-full mt-1/4 gap-1/2 gap-full gap-x-full space-y-1/2")
	allUnknown("z-1.5 z-[1.5] -z-auto inset-shadow")
	for _, known := range []string{
		"antialiased subpixel-antialiased box-border isolation-auto appearance-none appearance-auto will-change-transform will-change-[opacity] md:antialiased",
		"-mt-2 -mx-px -m-4 -top-1/2 -inset-x-4 -start-2 -z-10 z-[100] -z-[5] -translate-x-1/2 -rotate-6 -rotate-x-12 -scale-x-100 -skew-y-3 -tracking-wide -underline-offset-2 -outline-offset-2 -hue-rotate-30 -backdrop-hue-rotate-15 -bg-linear-45 -bg-conic-90 -col-start-2 -row-end-1 -space-y-2 w-1/2 inset-full basis-1/3 translate-full",
	} {
		if u := unknownOf(defaultTailwind, known); len(u) > 0 {
			t.Errorf("unknown: %q", u)
		}
	}
	want(t, "-z-10", styled(t, "-z-10").ZIndex, -10)
	want(t, "z-[100]", styled(t, "z-[100]").ZIndex, 100)
	ext, err := ParseTailwind([]byte(`{"insetShadow": {"": "inset 0 1px 2px rgb(0 0 0 / 0.1)"}}`))
	if err != nil {
		t.Fatal(err)
	}
	if u := unknownOf(ext.t, "inset-shadow"); len(u) > 0 {
		t.Errorf("an app's bare inset shadow: %q", u)
	}
}

func TestPlaceholderAndProps(t *testing.T) {
	s, _ := tw(t, "placeholder:text-zinc-400 selection:bg-blue-500 line-clamp-2 pointer-events-none select-none object-contain")
	want(t, "placeholder", s.PlaceholderColor, RGB(0x9f9fa9))
	want(t, "selection", s.SelectionColor, RGB(0x2b7fff))
	want(t, "caret", styled(t, "caret-blue-500").CaretColor, RGB(0x2b7fff))
	want(t, "object-contain", s.ObjectFit, Contain)
	want(t, "object-none", styled(t, "object-none").ObjectFit, Center)
	want(t, "object-scale-down", styled(t, "object-scale-down").ObjectFit, ScaleDown)
	want(t, "line-clamp", s.NumberOfLines, 2)
	want(t, "pointer-events", s.PointerEvents, PointerEventsNone)
	want(t, "select-none", s.UserSelect, UserSelectNone)
	got := unknownOf(defaultTailwind, "selection:text-blue-500 placeholder:italic placeholder:bg-red-500 selection:opacity-50 placeholder:text-muted-foreground selection:bg-primary")
	want(t, "pseudo-elements", strings.Join(got, " "),
		"selection:text-blue-500 placeholder:italic placeholder:bg-red-500 selection:opacity-50")
}

func TestFontTokensAreTailwindsSansAndMono(t *testing.T) {
	want(t, "font-mono", styled(t, "font-mono").FontFamily, "monospace")
	want(t, "font-[family-name:Fira_Code]", styled(t, "font-[family-name:Fira_Code]").FontFamily, "Fira Code")
}

func TestCurrentColorsFollowTheCascadeAndInherit(t *testing.T) {
	for _, classes := range []string{
		"text-red-500 border-2 border-blue-500 border-current ring ring-blue-500 ring-current outline-2 outline-blue-500 outline-current",
		"text-red-500 border-current border-2 ring-current ring outline-current outline-2",
	} {
		s := styled(t, classes)
		want(t, classes+" border", borderTop(s), s.Color)
		want(t, classes+" outline", s.OutlineColor, s.Color)
		for _, sh := range s.Shadows {
			want(t, classes+" ring", sh.Color, s.Color)
		}
	}
	want(t, "text-inherit", styled(t, "text-red-500 text-inherit").Color, 0)
	want(t, "text-current", styled(t, "text-red-500 text-current").Color, 0)
}

func TestArbitraryLengthsRequireCSSUnitsAndUseTheConfiguredRem(t *testing.T) {
	ext, err := ParseTailwind([]byte("rem: 20"))
	if err != nil {
		t.Fatal(err)
	}
	env := twEnv()
	env.tw = ext.t
	want(t, "w-[2rem]", styled(t, "w-[2rem]", env).Width, Pt(40))
	want(t, "w-[2pt]", styled(t, "w-[2pt]", env).Width, Pt(8.0/3))
	for _, classes := range []string{"w-[100]", "p-[8]", "text-[14]", "rounded-[8]", "rotate-1.5", "scale-x-12.5"} {
		want(t, classes, len(unknownOf(ext.t, classes)), 1)
	}
	want(t, "inset-s-4", styled(t, "inset-s-4").Left, styled(t, "start-4").Left)
}

func TestConfiguredRemReachesCompositeCSSValues(t *testing.T) {
	ext, err := ParseTailwind([]byte("rem: 20"))
	if err != nil {
		t.Fatal(err)
	}
	env := twEnv()
	env.tw = ext.t
	sh := styled(t, "shadow-[0_1rem_2rem_3px_#000]", env).Shadows[0]
	want(t, "shadow", [3]Points{sh.Offset.Y, sh.Blur, sh.Spread}, [3]Points{20, 40, 3})
}

func TestObjectPositions(t *testing.T) {
	at := func(class string) ObjectPosition { return styled(t, class).ObjectPosition }
	pos := func(s string) ObjectPosition {
		p, err := ParseObjectPosition(s)
		if err != nil {
			t.Fatal(err)
		}
		return p
	}
	want(t, "center", at("object-center"), pos("center"))
	want(t, "top", at("object-top"), pos("top"))
	want(t, "bottom", at("object-bottom"), pos("bottom"))
	want(t, "left", at("object-left"), pos("left"))
	want(t, "right", at("object-right"), pos("right"))
	for a, b := range map[string]string{"object-top-left": "object-left-top", "object-top-right": "object-right-top",
		"object-bottom-left": "object-left-bottom", "object-bottom-right": "object-right-bottom"} {
		want(t, a, at(a), at(b))
	}
	want(t, "top-left", at("object-top-left"), pos("left top"))
	want(t, "bottom-right", at("object-bottom-right"), pos("right bottom"))
	want(t, "object-[25%_75%]", at("object-[25%_75%]"), pos("25% 75%"))
	want(t, "object-[right_10px_bottom_20%]", at("object-[right_10px_bottom_20%]"), pos("right 10px bottom 20%"))
	want(t, "unknown", strings.Join(unknownOf(defaultTailwind, "object-[sideways] object-[1px_2px_3px] object-middle"), " "),
		"object-[sideways] object-[1px_2px_3px] object-middle")
}

func TestVariantsSortAsTailwindSortsThem(t *testing.T) {
	pad := func(classes string, env classEnv) Points { return styled(t, classes, env).Padding.Left }
	want(t, "max-md max-lg", pad("max-md:p-2 max-lg:p-4", twWide(600)), 8)
	want(t, "max-lg max-md", pad("max-lg:p-4 max-md:p-2", twWide(600)), 8)
	want(t, "max-sm max-[600px]", pad("max-sm:p-2 max-[600px]:p-4", twWide(500)), 8)
	dark := twDark()
	s := styled(t, "dark:hover:p-2 dark:p-4", dark)
	if s.Padding.Left != 16 || s.Hover == nil || s.Hover.Padding.Left != 8 {
		t.Errorf("dark:hover: %v %+v", s.Padding.Left, s.Hover)
	}
}

func TestBareValuesFollowTailwindsGrammar(t *testing.T) {
	for _, c := range strings.Fields(`p-.5 p-+4 m-1e1 border-1.5 ring-2.5 outline-0.5 duration-1.5 brightness-1.5 grow-1.5
		line-clamp-1.5 z-+4 grid-cols-+2 decoration-px outline-px outline-offset-px underline-offset-px w-prose min-w-prose
		size-screen w-1.5/2 from-10.5% p-[-4px] w-[-10px] gap-[-2px] rounded-[-4px] text-[-4px]`) {
		if len(unknownOf(defaultTailwind, c)) == 0 {
			t.Errorf("%s is no Tailwind class (or no valid CSS)", c)
		}
	}
	for _, c := range strings.Fields(`p-0.5 p-3.25 opacity-2.5 bg-red-500/2.5 max-w-prose w-dvh size-dvw basis-3xs -m-[4px]
		z-[+4] perspective-none bg-mauve-500 text-taupe-950 border-olive-200 ring-mist-400 shadow-current
		inset-shadow-current text-shadow-current drop-shadow-current drop-shadow-red-500 ring-offset-current
		decoration-current caret-current from-current via-current to-current bg-current/50 shadow-xl/20
		drop-shadow-md/20 text-shadow-md/20`) {
		if u := unknownOf(defaultTailwind, c); len(u) > 0 {
			t.Errorf("%s is unknown", c)
		}
	}
}

func TestStickyIsPositionSticky(t *testing.T) {
	want(t, "sticky", styled(t, "sticky top-0").Position, Sticky)
	want(t, "sticky absolute", styled(t, "sticky absolute").Position, Absolute)
}

func TestSVGPaintUtilitiesFollowTailwindCSS(t *testing.T) {
	s := styled(t, "fill-red-500/50 stroke-blue-500 stroke-2")
	want(t, "fill", s.Fill, PaintColor(Color(0xfb2c3680)))
	want(t, "stroke", s.Stroke, PaintColor(RGB(0x2b7fff)))
	want(t, "stroke-2", s.StrokeWidth, 2)
	s = styled(t, "fill-none stroke-current stroke-[1.5px]")
	want(t, "fill-none", s.Fill, PaintNone)
	want(t, "stroke-current", s.Stroke, PaintCurrentColor)
	want(t, "stroke-[1.5px]", s.StrokeWidth, 1.5)
	s = styled(t, "fill-current stroke-none stroke-0 stroke-[#00ff00]")
	want(t, "fill-current", s.Fill, PaintCurrentColor)
	want(t, "stroke-[#00ff00]", s.Stroke, PaintColor(RGB(0x00ff00)))
	want(t, "stroke-0", s.StrokeWidth, 0)
	want(t, "fill-inherit", styled(t, "fill-red-500 fill-inherit").Fill, Paint{})
	primary, _ := DefaultTheme().Light.Color("primary")
	want(t, "stroke-primary", styled(t, "stroke-primary").Stroke, PaintColor(primary))
}

func TestEnvInArbitrarySpacing(t *testing.T) {
	s := styled(t, "pb-[env(safe-area-inset-bottom)] pt-[calc(env(safe-area-inset-top)+8px)] "+
		"mb-[env(keyboard-inset-height)] ml-[calc(4px_+_env(safe-area-inset-left))] px-[env(safe-area-inset-right,_4px)]")
	want(t, "padding env", s.PaddingEnv, EnvAll(EnvSafeArea))
	want(t, "padding top", s.Padding.Top, 8)
	want(t, "margin env", s.MarginEnv, EnvEdges{Bottom: EnvKeyboard, Left: EnvSafeArea})
	want(t, "margin left", s.Margin.Left, 4)
	want(t, "later wins", styled(t, "pb-[env(keyboard-inset-height)] pb-2").PaddingEnv.Bottom, 0)
	got := unknownOf(defaultTailwind, "ps-[env(safe-area-inset-left)] -mb-[env(keyboard-inset-height)] pb-[env(nope)] "+
		"pb-[calc(env(safe-area-inset-top)*2)] w-[env(safe-area-inset-top)]")
	want(t, "unsupported", len(got), 5)
}

func TestTailwindExtension(t *testing.T) {
	ext, err := ParseTailwind([]byte(`
colors: { brand: { 500: "#7c3aed", DEFAULT: "#6d28d9" }, accent: "#f59e0b" }
spacing: 5
breakpoints: { tablet: 600 }
fontFamily: { display: Inter }
fontSize: { huge: [48, 1.1] }
fontWeight: { heavy: 850 }
radius: { card: 14 }
shadow: { card: "0 2px 8px rgb(0 0 0 / 0.12)" }
tracking: { label: 0.08 }
leading: { dense: 1.15 }
blur: { glass: 20 }
aspect: { photo: 1.5 }
ease: { snappy: [0.2, 0, 0, 1] }
container: { content: 680 }
`))
	if err != nil {
		t.Fatal(err)
	}
	env := twEnv()
	env.tw = ext.t
	s := styled(t, "bg-brand-500 text-brand border-accent p-2 font-display text-huge font-heavy rounded-card shadow-card "+
		"tracking-label blur-glass aspect-photo ease-snappy max-w-content", env)
	want(t, "bg-brand-500", s.Background, RGB(0x7c3aed))
	want(t, "text-brand", s.Color, RGB(0x6d28d9))
	want(t, "border-accent", s.BorderColor, RGB(0xf59e0b))
	want(t, "spacing", s.Padding.Top, 10)
	want(t, "font-display", s.FontFamily, "Inter")
	want(t, "text-huge", [2]any{s.FontSize, s.LineHeightMultiple}, [2]any{Points(48), float32(1.1)})
	want(t, "font-heavy", s.FontWeight, 850)
	want(t, "rounded-card", s.CornerRadius, 14)
	want(t, "shadow-card", len(s.Shadows), 1)
	want(t, "blur-glass", s.Filters[0], Filter{Kind: FilterBlur, Amount: 20})
	want(t, "aspect-photo", s.AspectRatio, 1.5)
	want(t, "ease-snappy", s.Transition.Easing, Curve{X1: 0.2, Y1: 0, X2: 0, Y2: 1})
	want(t, "max-w-content", s.MaxWidth, Pt(680))
	want(t, "tablet: narrow", styled(t, "p-1 tablet:p-2", env).Padding.Top, 5)
	wide := twWide(700)
	wide.tw = ext.t
	want(t, "tablet: wide", styled(t, "p-1 tablet:p-2", wide).Padding.Top, 10)
	want(t, "default theme", len(unknownOf(defaultTailwind, "bg-brand-500")), 1)
	for _, bad := range []string{"colours: {}", "spacing: wide", `colors: { brand: "nope" }`, "ease: { x: [1, 2] }",
		`shadow: { x: "nope" }`} {
		if _, err := ParseTailwind([]byte(bad)); err == nil {
			t.Errorf("%s: no error", bad)
		}
	}
}

func TestThemeTailwindReachesTheSubtreesClasses(t *testing.T) {
	ext, err := ParseTailwind([]byte(`colors: { brand: "#7c3aed" }`))
	if err != nil {
		t.Fatal(err)
	}
	h := newHost(t, Size{Width: 390, Height: 844}, func(cx *Scope) Node {
		th := DefaultTheme().Clone()
		th.Tailwind = ext
		cx.SetTheme(th)
		return View{TestID: "box", Class: "bg-brand", Children: []Node{Text{TestID: "t", Spans: []Span{{Content: "x", Class: "text-brand"}}}}}
	})
	h.commit()
	want(t, "bg-brand", h.find("box").style.Background, RGB(0x7c3aed))
}

// borderTop is the top border's color: 0 is currentColor, the text's.
func borderTop(s Style) Color {
	return cmpOr(s.BorderColors.Top, cmpOr(s.BorderColor, s.Color))
}

func TestObjectFitClassesReachImagesAndVideos(t *testing.T) {
	h := newHost(t, Size{Width: 390, Height: 844}, func(cx *Scope) Node {
		return View{Children: []Node{
			Image{TestID: "img", Class: "object-contain object-top"},
			Image{TestID: "own", Class: "object-contain", ResizeMode: Stretch},
			Video{TestID: "video", Class: "object-cover"},
		}}
	})
	h.commit()
	want(t, "image", h.find("img").image.sent.ResizeMode, Contain)
	top, _ := ParseObjectPosition("top")
	want(t, "image position", h.find("img").image.sent.Position, top)
	want(t, "a class over the element's mode", h.find("own").image.sent.ResizeMode, Contain)
	want(t, "video", h.find("video").video.sent.ResizeMode, Cover)
}
