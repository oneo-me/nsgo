package main

import (
	"time"

	"github.com/oneo-me/nsgo/core"
)

// specimenStage is the stage every specimen stands on: the card's light,
// without its drop shadow; read it in a StyleFunc.
func specimenStage(cx *core.Scope) core.Style {
	colors := colorsOf(cx)
	hl := inner(0, 1, 0, core.RGB(0xffffff))
	if cx.Dark() {
		hl = inner(0, 1, 0, white(0.05))
	}
	return core.Style{Height: core.Pt(104), AlignItems: core.AlignCenter, JustifyContent: core.JustifyCenter, CornerRadius: radiusControl,
		BorderCurve: core.BorderCurveContinuous, Background: colors.surface(), BorderWidth: 1, BorderColor: colors.line(),
		Gradients: finOf(cx).card().gradients, Shadows: []core.BoxShadow{hl}}
}

// specimen is a box the styles are tried on, s over it, for a StyleFunc.
func specimen(cx *core.Scope, s core.Style) core.Style {
	return core.Style{Width: core.Pt(112), Height: core.Pt(76), AlignItems: core.AlignCenter, JustifyContent: core.JustifyCenter,
		Background: colorsOf(cx).raised()}.Merge(s)
}

// specimenOf is a specimen with s over it.
func specimenOf(cx *core.Scope, s core.Style) func() core.Style {
	return func() core.Style { return specimen(cx, s) }
}

func specimenLabel(s string, c func() core.Color) core.Node { return text(s, 13, semibold, c) }

// tileProps are a specimen tile's texts: a label, the style it shows as
// code and a plain note.
type tileProps struct {
	label, code, note string
	labelFn           func() string
	onPress           func()
}

// tile is one cell of a two-column grid: the specimen on a stage and its
// texts.
func tile(cx *core.Scope, p tileProps, children ...core.Node) core.Node {
	colors := colorsOf(cx)
	st := core.View{StyleFunc: func() core.Style { return specimenStage(cx) }, Children: children}
	if p.onPress != nil {
		st.OnPress = func(core.PressEvent) { p.onPress() }
		st.Role = core.RoleButton
		st.Label = p.label
	}
	lbl := p.labelFn
	if lbl == nil {
		lbl = label(p.label)
	}
	texts := []core.Node{textf(lbl, 13, medium, colors.text)}
	if p.code != "" {
		texts = append(texts, core.Text{Content: p.code, MaxLines: 1, Style: core.Style{FontSize: 12, FontFamily: mono}, StyleFunc: colored(colors.muted)})
	}
	if p.note != "" {
		texts = append(texts, core.Text{Content: p.note, MaxLines: 1, Style: core.Style{FontSize: 12}, StyleFunc: colored(colors.faint)})
	}
	return core.View{Style: core.Style{FlexBasis: core.Pct(40), FlexGrow: 1, Gap: 8}, Children: []core.Node{
		st, core.View{Style: core.Style{Gap: 3, Padding: core.Symmetric(0, 2)}, Children: texts},
	}}
}

func grid(children ...core.Node) core.Node {
	return core.View{Style: core.Style{Direction: core.Row, Wrap: core.Wrap, ColumnGap: 12, RowGap: 16}, Children: children}
}

func cornersCard(cx *core.Scope) core.Node {
	colors := colorsOf(cx)
	tints := tintsOf(cx)
	shapes := []struct {
		label string
		style core.Style
		paint func() core.Style
	}{
		{"Circular", core.Style{Width: core.Pt(60), Height: core.Pt(60), CornerRadius: 20}, filled(colors.info)},
		{"Continuous", core.Style{Width: core.Pt(60), Height: core.Pt(60), CornerRadius: 20, BorderCurve: core.BorderCurveContinuous}, filled(colors.info)},
		{"One corner", core.Style{Width: core.Pt(60), Height: core.Pt(60), CornerRadii: core.Corners{TopLeft: 4, TopRight: 20, BottomRight: 20, BottomLeft: 20},
			BorderCurve: core.BorderCurveContinuous, BorderWidth: 2.5}, func() core.Style { return core.Style{BorderColor: colors.accent()} }},
		{"Pill", core.Style{Width: core.Pt(60), Height: core.Pt(28), CornerRadius: 14, BorderCurve: core.BorderCurveContinuous}, filled(colors.up)},
	}
	var row []core.Node
	for _, s := range shapes {
		row = append(row, core.View{Style: core.Style{AlignItems: core.AlignCenter, Gap: 10}, Children: []core.Node{
			core.View{Style: core.Style{Height: core.Pt(64), JustifyContent: core.JustifyCenter}, Children: []core.Node{core.View{Style: s.style, StyleFunc: s.paint}}},
			text(s.label, 12, regular, colors.muted),
		}})
	}
	return card(cx, cardProps{icon: iconStyles, title: "Corners", subtitle: "CornerRadius · BorderCurve · per corner"},
		core.View{Style: core.Style{Direction: core.Row, JustifyContent: core.JustifySpaceBetween}, Children: row},
		caption(cx, "Continuous corners ease into the edges", label("Radius 20 pt")),
		grid(
			tile(cx, tileProps{label: "Top corners", code: "CornerRadii", note: "28 pt"},
				core.View{StyleFunc: func() core.Style {
					return specimen(cx, core.Style{CornerRadii: core.Corners{TopLeft: 28, TopRight: 28}, BorderWidth: 1.5, BorderColor: colors.accent()})
				},
					Children: []core.Node{specimenLabel("28 pt", colors.accent)}}),
			tile(cx, tileProps{label: "Mixed radii", note: "44 · 6 · 28 · 0 pt, clockwise"},
				core.View{StyleFunc: func() core.Style {
					return specimen(cx, core.Style{Background: tints.info(), CornerRadii: core.Corners{TopLeft: 44, TopRight: 6, BottomRight: 28}, BorderWidth: 2, BorderColor: colors.info()})
				},
					Children: []core.Node{specimenLabel("Mixed", colors.info)}}),
		),
	)
}

func bordersCard(cx *core.Scope) core.Node {
	colors := colorsOf(cx)
	return card(cx, cardProps{icon: iconComponents, title: "Borders", subtitle: "Per side · dashed · dotted · outlines"},
		grid(
			tile(cx, tileProps{label: "Per-side borders", note: "6 · 2 · 4 · 1 pt, clockwise"},
				core.View{StyleFunc: func() core.Style {
					return specimen(cx, core.Style{Borders: core.Edges{Top: 6, Right: 2, Bottom: 4, Left: 1},
						BorderColors: core.EdgeColors{Top: colors.danger(), Right: colors.up(), Bottom: colors.info(), Left: colors.warn()}})
				}}),
			tile(cx, tileProps{label: "Dashed / dotted", code: "BorderStyle"},
				core.View{Style: core.Style{Gap: 8}, Children: []core.Node{
					core.View{StyleFunc: func() core.Style {
						return core.Style{Width: core.Pt(112), Height: core.Pt(28), CornerRadius: 8, BorderWidth: 1.5, BorderColor: colors.info(), BorderStyle: core.BorderDashed}
					}},
					core.View{StyleFunc: func() core.Style {
						return core.Style{Width: core.Pt(112), Height: core.Pt(28), CornerRadius: 8, BorderWidth: 3, BorderColor: colors.warn(), BorderStyle: core.BorderDotted}
					}},
				}}),
			tile(cx, tileProps{label: "Rings", code: "Ring", note: "Box shadows of no blur"},
				core.View{StyleFunc: func() core.Style {
					return specimen(cx, core.Style{CornerRadius: 14, Shadows: []core.BoxShadow{core.Ring(3, colors.surface()), core.Ring(5, colors.accent())}})
				}}),
			tile(cx, tileProps{label: "Outline", code: "OutlineOffset", note: "Outside the border box"},
				core.View{StyleFunc: func() core.Style {
					return specimen(cx, core.Style{CornerRadius: 14, OutlineWidth: 2, OutlineColor: colors.info(), OutlineOffset: 4, OutlineStyle: core.BorderDashed})
				}}),
		),
	)
}

// shadowsCard is shadows that read on a dark page: colored glows rather
// than black.
func shadowsCard(cx *core.Scope) core.Node {
	colors := colorsOf(cx)
	tints := tintsOf(cx)
	offsetBg := func() core.Color {
		if cx.Dark() {
			return core.RGB(0x1d2740)
		}
		return core.RGB(0xe7edfb)
	}
	shade := func() float32 {
		if cx.Dark() {
			return 0.6
		}
		return 0.22
	}
	return card(cx, cardProps{icon: iconSpark, title: "Shadows and glow", subtitle: "ShadowColor · box shadows · text shadow"},
		grid(
			tile(cx, tileProps{label: "Glow", code: "ShadowColor", note: "A colored shadow"},
				core.View{StyleFunc: func() core.Style {
					return specimen(cx, core.Style{CornerRadius: 16, BorderCurve: core.BorderCurveContinuous, Background: colors.accentFill(),
						ShadowColor: alpha(colors.accentFill(), 0.55), ShadowRadius: 9, ShadowOffset: core.Point{Y: 6}})
				}, Children: []core.Node{specimenLabel("Glow", colors.onAccent)}}),
			tile(cx, tileProps{label: "Offset shadow", code: "ShadowOffset", note: "4, 8 pt · radius 10"},
				core.View{StyleFunc: func() core.Style {
					return specimen(cx, core.Style{Background: offsetBg(), CornerRadii: core.Corners{TopLeft: 30, BottomRight: 30}, BorderWidth: 1, BorderColor: colors.info(),
						ShadowColor: alpha(colors.info(), 0.45), ShadowRadius: 5, ShadowOffset: core.Point{X: 4, Y: 8}})
				}, Children: []core.Node{specimenLabel("Offset", colors.info)}}),
			tile(cx, tileProps{label: "Image corners", note: "Two radii and a shadow"},
				core.Image{Source: core.Src("https://picsum.photos/seed/nsh/300/220"), Style: core.Style{Width: core.Pt(112), Height: core.Pt(76),
					CornerRadii: core.Corners{TopLeft: 28, BottomRight: 28}, ShadowRadius: 6, ShadowOffset: core.Point{Y: 8}},
					StyleFunc: func() core.Style { return core.Style{Background: colors.raised(), ShadowColor: black(shade())} }}),
			tile(cx, tileProps{label: "Text shadow", code: "TextShadowColor"},
				core.Text{Content: "Shadow", StyleFunc: func() core.Style {
					return core.Style{FontSize: 24, FontWeight: core.FontWeight(800), Color: colors.text(), LetterSpacing: -0.5,
						TextShadowColor: alpha(colors.danger(), 0.9), TextShadowRadius: 4}
				}}),
			tile(cx, tileProps{label: "Text corners", note: "A span's background and radius"},
				core.Text{Spans: []core.Span{{Content: " Text corners ", StyleFunc: func() core.Style { return core.Style{Background: tints.up(), CornerRadius: 10} }}},
					Style: core.Style{FontSize: 15, FontWeight: semibold, LineHeight: 30}, StyleFunc: colored(colors.up)}),
			tile(cx, tileProps{label: "Layered", code: "Shadows", note: "Two box shadows and an inset one"},
				core.View{StyleFunc: func() core.Style {
					return specimen(cx, core.Style{CornerRadius: 16, Background: colors.surface(), Shadows: []core.BoxShadow{
						inner(0, -2, 6, alpha(colors.accent(), 0.35)), shadow(0, 1, 2, 0, black(0.08)), shadow(0, 10, 24, -4, black(shade()))}})
				},
					Children: []core.Node{specimenLabel("Layered", colors.text)}}),
		),
	)
}

// layersCard is clipping, stacking order and opacity, each a press away.
func layersCard(cx *core.Scope) core.Node {
	colors := colorsOf(cx)
	tints := tintsOf(cx)
	clip := core.NewSignal(cx, true)
	front := core.NewSignal(cx, 0)
	opacity := core.NewAnimatedValue(cx, 0.45)
	level := core.NewSignal(cx, "0.45")
	stack := []func() core.Color{colors.danger, colors.info, colors.up}
	var cards []core.Node
	for i, c := range stack {
		cards = append(cards, core.View{StyleFunc: func() core.Style {
			shade := float32(0.18)
			if cx.Dark() {
				shade = 0.5
			}
			s := core.Style{Position: core.Absolute, Left: core.Pt(4 + core.Points(i)*28), Top: core.Pt(2 + core.Points(i)*14), Width: core.Pt(64), Height: core.Pt(52),
				CornerRadius: 12, BorderCurve: core.BorderCurveContinuous, Background: c(), ShadowColor: black(shade), ShadowRadius: 3, ShadowOffset: core.Point{Y: 3}}
			if front.Get() == i {
				s.BorderWidth, s.BorderColor, s.ZIndex = 2, colors.text(), 10
			}
			return s
		}})
	}
	onBlue := func() core.Color {
		if cx.Dark() {
			return core.RGB(0x0c1020)
		}
		return core.RGB(0xffffff)
	}
	return card(cx, cardProps{icon: iconLists, title: "Clipping and layers", subtitle: "Overflow · ZIndex · Opacity", right: toggle(clip.Get, clip.Set, "Clip")},
		grid(
			tile(cx, tileProps{labelFn: func() string {
				if clip.Get() {
					return "Clipped"
				}
				return "Visible"
			}, code: "Overflow: Hidden / Visible", note: hint(cx, "Tap to switch", "Click to switch"), onPress: func() { toggled(clip) }},
				core.View{StyleFunc: func() core.Style {
					o := core.Hidden
					if !clip.Get() {
						o = core.Visible
					}
					return specimen(cx, core.Style{Background: tints.warn(), CornerRadius: 22, BorderWidth: 1.5, BorderColor: colors.warn(), Overflow: o})
				}, Children: []core.Node{
					core.View{Style: core.Style{Position: core.Absolute, Left: core.Pt(-26), Top: core.Pt(-26), Width: core.Pt(72), Height: core.Pt(72), CornerRadius: 36}, StyleFunc: filled(colors.danger)},
					specimenLabel(hint(cx, "Tap", "Click"), colors.warn),
				}}),
			tile(cx, tileProps{label: "Stacking order", code: "ZIndex", note: hint(cx, "Tap to raise the next", "Click to raise the next"), onPress: func() { front.Update(func(n int) int { return (n + 1) % 3 }) }},
				core.View{Style: core.Style{Width: core.Pt(130), Height: core.Pt(84)}, Children: cards}),
		),
		core.View{Style: core.Style{Direction: core.Row, AlignItems: core.AlignCenter, Gap: 12}, Children: []core.Node{
			core.View{Style: core.Style{Width: core.Pt(64), Height: core.Pt(64), CornerRadius: 14, BorderCurve: core.BorderCurveContinuous,
				AlignItems: core.AlignCenter, JustifyContent: core.JustifyCenter, Animated: &core.AnimatedStyle{Opacity: opacity}}, StyleFunc: filled(colors.info),
				Children: []core.Node{textf(level.Get, 13, bold, onBlue)}},
			core.View{Style: core.Style{FlexGrow: 1, FlexShrink: 1, Gap: 8}, Children: []core.Node{
				caption(cx, "Opacity", label("Animated natively")),
				segmented([]option[string]{{value: "0.2", label: "0.2"}, {value: "0.45", label: "0.45"}, {value: "1", label: "1"}}, level.Get, func(v string) {
					level.Set(v)
					to := map[string]float64{"0.2": 0.2, "0.45": 0.45, "1": 1}[v]
					timing(cx, opacity, to, core.TimingConfig{Duration: ms(260), Easing: easeOutCubic})
				}, false),
			}},
		}},
	)
}

// flipCard is two faces back to back: each hides while turned away.
func flipCard(cx *core.Scope) core.Node {
	colors := colorsOf(cx)
	angle := core.NewAnimatedValue(cx, 0)
	flipped := false
	flip := func() {
		flipped = !flipped
		to := 0.0
		if flipped {
			to = 180
		}
		spring(cx, angle, to, core.SpringConfig{Stiffness: 140, Damping: 16})
	}
	// A face's background, gradient and rim, light then dark.
	type paint struct{ bg, from, to, border core.Color }
	face := func(light, dark paint, rotate core.AnimatedNode, badgeText string, t tone, degrees, title, body string) core.Node {
		return core.View{Style: core.Style{Position: core.Absolute, Left: core.Pt(0), Top: core.Pt(0), Right: core.Pt(0), Bottom: core.Pt(0), CornerRadius: radiusCard,
			BorderCurve: core.BorderCurveContinuous, BorderWidth: 1, Padding: core.All(16), JustifyContent: core.JustifySpaceBetween, BackfaceHidden: true,
			Transform: core.Transform{Perspective: 800}, Animated: &core.AnimatedStyle{RotateY: rotate}}, StyleFunc: func() core.Style {
			p, sh := light, []core.BoxShadow{inner(0, 1, 0, core.RGB(0xffffff)), shadow(0, 8, 20, 0, paper(0.10))}
			if cx.Dark() {
				p, sh = dark, []core.BoxShadow{inner(0, 1, 0, white(0.07)), shadow(0, 10, 24, 0, black(0.4))}
			}
			return core.Style{BorderColor: p.border, Background: p.bg, Gradients: []core.Gradient{core.LinearGradient(180, p.from, p.to)}, Shadows: sh}
		}, Children: []core.Node{
			core.View{Style: core.Style{Direction: core.Row, JustifyContent: core.JustifySpaceBetween}, Children: []core.Node{badge(cx, badgeText, t), text(degrees, 13, regular, colors.muted)}},
			core.View{Style: core.Style{Gap: 2}, Children: []core.Node{text(title, 20, semibold, colors.text), text(body, 13, regular, colors.muted)}},
		}}
	}
	front := face(paint{core.RGB(0xeef2fc), core.RGB(0xf0f4fd), core.RGB(0xebf0fb), core.RGB(0xcdd8f3)},
		paint{core.RGB(0x1a2134), core.RGB(0x1e263b), core.RGB(0x1a2134), core.RGB(0x2b3a5c)},
		angle, "front", toneInfo, "0°", hint(cx, "Tap to flip", "Click to flip"), "A spring turns it on the UI thread")
	back := face(paint{core.RGB(0xfbefe6), core.RGB(0xfcf2ea), core.RGB(0xfaebe0), core.RGB(0xf0cdb3)},
		paint{core.RGB(0x2a1d14), core.RGB(0x2f2116), core.RGB(0x291c13), core.RGB(0x4e3423)},
		core.Add(angle, core.Constant(180)), "back", toneAccent, "180°", "BackfaceHidden", "Each face hides while turned away")
	return press(pressProps{onPress: flip, scale: 0.98, label: "Flip the card"}, core.View{Style: core.Style{Height: core.Pt(132)}, Children: []core.Node{front, back}})
}

func transformsCard(cx *core.Scope) core.Node {
	colors := colorsOf(cx)
	tints := tintsOf(cx)
	// A slow sway on the platform, so the perspective reads as depth.
	sway := core.NewAnimatedValue(cx, -30)
	sine := ease(core.EasingSin, core.EaseModeInOut)
	run := cx.StartAnimation(core.Loop(core.Sequence(
		core.Timing(sway, 30, core.TimingConfig{Duration: ms(2200), Easing: sine}),
		core.Timing(sway, -30, core.TimingConfig{Duration: ms(2200), Easing: sine}),
	), 0), nil)
	cx.Cleanup(run.Stop)
	return card(cx, cardProps{icon: iconPhysics, title: "3D transforms", subtitle: "Perspective · RotateX/Y · Skew · origin"},
		flipCard(cx),
		grid(
			tile(cx, tileProps{label: "Perspective", code: "RotateY", note: "Swaying, driven natively"},
				core.View{StyleFunc: func() core.Style {
					return specimen(cx, core.Style{CornerRadius: 14, Background: tints.info(), BorderWidth: 1, BorderColor: colors.info(),
						Transform: core.Transform{Perspective: 400}, Animated: &core.AnimatedStyle{RotateY: sway}})
				}, Children: []core.Node{specimenLabel("Y axis", colors.info)}}),
			tile(cx, tileProps{label: "Tilt", code: "RotateX: 40"},
				core.View{StyleFunc: func() core.Style {
					return specimen(cx, core.Style{CornerRadius: 14, Background: tints.accent(), BorderWidth: 1, BorderColor: colors.accent(),
						Transform: core.Transform{Perspective: 400, RotateX: 40}})
				}, Children: []core.Node{specimenLabel("X axis", colors.accent)}}),
			tile(cx, tileProps{label: "Skew", code: "SkewX: -12"},
				core.View{StyleFunc: func() core.Style {
					return specimen(cx, core.Style{CornerRadius: 10, Background: tints.warn(), BorderWidth: 1, BorderColor: colors.warn(),
						Transform: core.Transform{SkewX: -12}})
				}, Children: []core.Node{specimenLabel("Skew", colors.warn)}}),
			tile(cx, tileProps{label: "Origin", code: "TransformOrigin", note: "Top left · rotated 30°"},
				core.View{Style: core.Style{Width: core.Pt(112), Height: core.Pt(84)}, Children: []core.Node{
					core.View{StyleFunc: func() core.Style {
						return core.Style{Position: core.Absolute, Left: core.Pt(30), Top: core.Pt(14), Width: core.Pt(54), Height: core.Pt(54), CornerRadius: 6,
							BorderWidth: 1, BorderColor: colors.faint(), BorderStyle: core.BorderDashed}
					}},
					core.View{StyleFunc: func() core.Style {
						return core.Style{Position: core.Absolute, Left: core.Pt(30), Top: core.Pt(14), Width: core.Pt(54), Height: core.Pt(54), CornerRadius: 6,
							Background: alpha(colors.danger(), 0.55), Transform: core.Transform{Rotate: 30},
							TransformOrigin: core.ObjectPosition{X: core.PositionAxis{Offset: core.Pct(0)}, Y: core.PositionAxis{Offset: core.Pct(0)}}}
					}},
					core.View{Style: core.Style{Position: core.Absolute, Left: core.Pt(26), Top: core.Pt(10), Width: core.Pt(8), Height: core.Pt(8), CornerRadius: 4}, StyleFunc: filled(colors.text)},
				}}),
		),
	)
}

// numeralsCard is figures in their proportional and tabular widths, then
// small capitals and oldstyle figures: the font's OpenType features.
func numeralsCard(cx *core.Scope) core.Node {
	colors := colorsOf(cx)
	figure := func(s string, v core.FontVariant) core.Node {
		t := text(s, 22, medium, colors.text)
		t.Style.FontVariant = v
		return t
	}
	column := func(head core.Node, v core.FontVariant) core.Node {
		return core.View{Style: core.Style{FlexGrow: 1, FlexShrink: 1, FlexBasis: core.Pct(0), Gap: 4}, Children: []core.Node{
			head, figure("1,111.11", v), figure("8,888.88", v)}}
	}
	tabular := text("tabular-nums", 12, regular, colors.accent)
	tabular.Style.FontFamily = mono
	feature := func(label string, v core.FontVariant, name string) core.Node {
		l := text(label, 18, regular, colors.text)
		l.Style.FontVariant, l.Style.FlexGrow, l.Style.FlexShrink = v, 1, 1
		n := text(name, 13, regular, colors.faint)
		n.Style.FontFamily = mono
		return core.View{Style: core.Style{Direction: core.Row, AlignItems: core.AlignBaseline, Gap: 8}, Children: []core.Node{l, n}}
	}
	return card(cx, cardProps{icon: iconText, title: "Numerals", subtitle: "FontVariant"},
		core.View{Style: core.Style{Direction: core.Row, Gap: 12}, Children: []core.Node{
			column(text("Proportional", 12, regular, colors.muted), 0),
			core.View{Style: core.Style{Width: core.Pt(1)}, StyleFunc: filled(colors.line)},
			column(tabular, core.FontTabularNums),
		}},
		core.View{Style: core.Style{Height: core.Pt(1)}, StyleFunc: filled(colors.line)},
		feature("Small Caps", core.FontSmallCaps, "small-caps"),
		feature("Oldstyle 1234567890", core.FontOldstyleNums, "oldstyle-nums"),
	)
}

// rightToLeft is rows, start margins and text under a right-to-left
// layout: the app's language decides it, and LayoutDirection sets a
// view's own.
func rightToLeft(cx *core.Scope) core.Node {
	colors := colorsOf(cx)
	tints := tintsOf(cx)
	rtl := core.NewSignal(cx, false)
	tones := []func() core.Color{colors.danger, colors.info, colors.up}
	onTile := func() core.Color {
		if cx.Dark() {
			return core.RGB(0x151412)
		}
		return core.RGB(0xffffff)
	}
	var boxes []core.Node
	for i, c := range tones {
		s := core.Style{Width: core.Pt(44), Height: core.Pt(44), CornerRadius: 10, CornerRadii: core.Corners{TopLeft: 22, TopRight: 10, BottomRight: 10, BottomLeft: 10},
			AlignItems: core.AlignCenter, JustifyContent: core.JustifyCenter}
		if i == 0 {
			s.Margin = core.Edges{Left: 16}
		}
		boxes = append(boxes, core.View{Style: s, StyleFunc: filled(c), Children: []core.Node{text(itoa(i+1), 15, bold, onTile)}})
	}
	chip := func(c, bg func() core.Color, n int) core.Node {
		return core.View{Style: core.Style{Width: core.Pt(40), Height: core.Pt(40), CornerRadius: 10, BorderCurve: core.BorderCurveContinuous,
			AlignItems: core.AlignCenter, JustifyContent: core.JustifyCenter}, StyleFunc: filled(bg), Children: []core.Node{text(itoa(n), 13, regular, c)}}
	}
	var chips []core.Node
	for n := 1; n <= 12; n++ {
		chips = append(chips, chip(colors.info, tints.info, n))
	}
	monoNote := func(s string) func() string { return label(s) }
	return card(cx, cardProps{icon: iconNav, title: "Right to left", subtitle: "", right: button(buttonProps{label: "Flip", variant: secondary, small: true}, func() { toggled(rtl) })},
		core.View{StyleFunc: func() core.Style {
			d := core.DirectionLTR
			if rtl.Get() {
				d = core.DirectionRTL
			}
			return core.Style{LayoutDirection: d, Gap: 12}
		}, Children: []core.Node{
			captionf(cx, func() string {
				if rtl.Get() {
					return "LayoutDirection: RTL"
				}
				return "LayoutDirection: LTR"
			}),
			core.View{Style: core.Style{Direction: core.Row, Gap: 8}, Children: boxes},
			core.Text{Content: "Text aligns to the start side; the first box has a start margin of 16 and a round top-start corner.",
				Style: core.Style{FontSize: 15, LineHeight: 21}, StyleFunc: colored(colors.text)},
			caption(cx, "Horizontal scroll view", monoNote("Scroll")),
			core.Scroll{Horizontal: true, HideIndicator: true, Style: core.Style{Height: core.Pt(40)}, Children: []core.Node{
				core.View{Style: core.Style{Direction: core.Row, Gap: 6}, Children: chips}}},
			caption(cx, "Horizontal list", monoNote("List")),
			core.List{Count: func() int { return 30 }, Horizontal: true, EstimatedSize: 46, HideIndicator: true, Gap: 6, Style: core.Style{Height: core.Pt(40)},
				Row: func(cx *core.Scope, r core.ListRow) core.Node {
					colors := colorsOf(cx)
					tints := tintsOf(cx)
					return core.View{StyleFunc: func() core.Style {
						return core.Style{Width: core.Pt(40), Height: core.Pt(40), CornerRadius: 10, AlignItems: core.AlignCenter, JustifyContent: core.JustifyCenter, Background: tints.accent()}
					},
						Children: []core.Node{textf(func() string { return itoa(r.Index() + 1) }, 13, regular, colors.accent)}}
				}},
			core.View{Style: core.Style{Height: core.Pt(1)}, StyleFunc: filled(colors.line)},
			core.Text{Content: "TextAlignStart: the start side, right to left", Style: core.Style{FontSize: 13, TextAlign: core.TextAlignStart}, StyleFunc: colored(colors.muted)},
			core.Text{Content: "مرحبا بالعالم — WritingDirection RTL", Style: core.Style{FontSize: 13, WritingDirection: core.WritingRTL}, StyleFunc: colored(colors.muted)},
		}},
	)
}

func gradientsCard(cx *core.Scope) core.Node {
	colors := colorsOf(cx)
	tints := tintsOf(cx)
	blue, yellow, pink := core.RGB(0x2563eb), core.RGB(0xfacc15), core.RGB(0xec4899)
	return card(cx, cardProps{icon: iconStyles, title: "Gradients", subtitle: "Linear, radial and conic, mixed in Oklab", right: badge(cx, "NSGo", toneNeutral)},
		grid(
			tile(cx, tileProps{label: "Linear", code: "LinearGradient(90, …)"}, core.View{StyleFunc: specimenOf(cx, core.Style{CornerRadius: 14, Gradients: []core.Gradient{core.LinearGradient(90, blue, pink)}})}),
			tile(cx, tileProps{label: "To a corner", code: "ToTopRight"}, core.View{StyleFunc: specimenOf(cx, core.Style{CornerRadius: 14,
				Gradients: []core.Gradient{{ToCorner: core.ToTopRight, Stops: []core.ColorStop{{Color: blue}, {Color: yellow}}}}})}),
			tile(cx, tileProps{label: "Oklab", note: "No gray in the middle"}, core.View{StyleFunc: specimenOf(cx, core.Style{CornerRadius: 14, Gradients: []core.Gradient{core.LinearGradient(90, blue, yellow)}})}),
			tile(cx, tileProps{label: "sRGB", code: "ColorSpaceSRGB"}, core.View{StyleFunc: specimenOf(cx, core.Style{CornerRadius: 14,
				Gradients: []core.Gradient{{Angle: 90, Space: core.ColorSpaceSRGB, Stops: []core.ColorStop{{Color: blue}, {Color: yellow}}}}})}),
			tile(cx, tileProps{label: "Radial", code: "RadialGradient"}, core.View{StyleFunc: specimenOf(cx, core.Style{CornerRadius: 14, Gradients: []core.Gradient{core.RadialGradient(yellow, pink, blue)}})}),
			tile(cx, tileProps{label: "Conic, repeating", code: "ConicGradient · Repeating"}, core.View{Style: core.Style{Gap: 8, Direction: core.Row}, Children: []core.Node{
				core.View{Style: core.Style{Width: core.Pt(64), Height: core.Pt(64), CornerRadius: 32, Gradients: []core.Gradient{core.ConicGradient(0, pink, yellow, blue, pink)}}},
				core.View{StyleFunc: func() core.Style {
					return core.Style{Width: core.Pt(64), Height: core.Pt(64), CornerRadius: 12, Gradients: []core.Gradient{{Angle: 45, Repeating: true, Space: core.ColorSpaceSRGB,
						Stops: []core.ColorStop{{Color: colors.raised(), At: core.Pt(0)}, {Color: colors.raised(), At: core.Pt(8)}, {Color: tints.accent(), At: core.Pt(8)}, {Color: tints.accent(), At: core.Pt(16)}}}}}
				}},
			}}),
		),
	)
}

func filtersCard(cx *core.Scope) core.Node {
	colors := colorsOf(cx)
	paint := []core.Gradient{core.LinearGradient(135, core.RGB(0xf43f5e), core.RGB(0xf59e0b), core.RGB(0x10b981), core.RGB(0x3b82f6))}
	art := func(name string, f ...core.Filter) core.Node {
		return core.View{Style: core.Style{AlignItems: core.AlignCenter, Gap: 6}, Children: []core.Node{
			core.View{Style: core.Style{Width: core.Pt(56), Height: core.Pt(56), CornerRadius: 12, Gradients: paint, Filters: f}},
			text(name, 12, regular, colors.muted),
		}}
	}
	stripes := func() core.Style {
		accent, surface := colors.accent(), colors.surface()
		return core.Style{Gradients: []core.Gradient{{Angle: 90, Repeating: true, Space: core.ColorSpaceSRGB, Stops: []core.ColorStop{
			{Color: accent, At: core.Pt(0)}, {Color: accent, At: core.Pt(6)}, {Color: surface, At: core.Pt(6)}, {Color: surface, At: core.Pt(12)}}}}}
	}
	pane := func(name string, s func() core.Style) core.Node {
		return core.View{Style: core.Style{Width: core.Pt(88), Height: core.Pt(60), CornerRadius: 12, AlignItems: core.AlignCenter, JustifyContent: core.JustifyCenter},
			StyleFunc: s, Children: []core.Node{text(name, 12, semibold, colors.text)}}
	}
	return card(cx, cardProps{icon: iconImage, title: "Filters, backdrops and masks", subtitle: "Filters · BackdropFilters · Mask · BlendMode", right: badge(cx, "NSGo", toneNeutral)},
		core.View{Style: core.Style{Direction: core.Row, Wrap: core.Wrap, Gap: 10}, Children: []core.Node{
			art("none"), art("grayscale", core.Grayscale(1)), art("sepia", core.Sepia(1)), art("hue 120°", core.HueRotate(120)),
			art("invert", core.Invert(1)), art("blur 4", core.Blur(4)),
		}},
		core.View{Style: core.Style{Direction: core.Row, Wrap: core.Wrap, Gap: 10, Padding: core.All(12), CornerRadius: radiusControl}, StyleFunc: stripes, Children: []core.Node{
			pane("frosted", func() core.Style {
				return core.Style{Background: alpha(colors.surface(), 0.4), BackdropFilters: []core.Filter{core.Blur(8)}}
			}),
			pane("glass", func() core.Style {
				return core.Style{Background: alpha(colors.surface(), 0.3), BackdropFilters: []core.Filter{core.Blur(12)}, GlassEffect: core.GlassRegular, CornerRadius: 24}
			}),
			pane("masked", func() core.Style {
				return core.Style{Background: colors.info(), Mask: &core.Gradient{Angle: 90, Stops: []core.ColorStop{{Color: core.RGB(0)}, {Color: 0}}}}
			}),
			pane("multiply", func() core.Style { return core.Style{Background: core.RGB(0x60a5fa), BlendMode: core.BlendMultiply} }),
		}},
	)
}

func keyframesCard(cx *core.Scope) core.Node {
	colors := colorsOf(cx)
	dot := func(name string, k core.Keyframes, s func() core.Style) core.Node {
		return core.View{Style: core.Style{AlignItems: core.AlignCenter, Gap: 8, Width: core.Pt(64)}, Children: []core.Node{
			core.View{Style: core.Style{Width: core.Pt(32), Height: core.Pt(32), Margin: core.Edges{Top: 8}, Animation: core.KeyframeAnimation{Keyframes: k}}, StyleFunc: s},
			text(name, 12, regular, colors.muted),
		}}
	}
	return card(cx, cardProps{icon: iconAnimation, title: "Keyframes and group styles", subtitle: "Animation · GroupStyle", right: badge(cx, "NSGo", toneNeutral)},
		core.View{Style: core.Style{Direction: core.Row, JustifyContent: core.JustifySpaceAround}, Children: []core.Node{
			dot("spin", core.KeyframesSpin, func() core.Style {
				raised := colors.raised()
				return core.Style{CornerRadius: 16, Borders: core.All(4), BorderColors: core.EdgeColors{Top: colors.accent(), Right: raised, Bottom: raised, Left: raised}}
			}),
			dot("ping", core.KeyframesPing, func() core.Style { return core.Style{Background: colors.accent(), CornerRadius: 16} }),
			dot("pulse", core.KeyframesPulse, func() core.Style { return core.Style{Background: colors.accent(), CornerRadius: 6} }),
			dot("bounce", core.KeyframesBounce, func() core.Style { return core.Style{Background: colors.accent(), CornerRadius: 16} }),
		}},
		core.View{OnPress: func(core.PressEvent) {}, Role: core.RoleButton, Label: "Hover this row",
			StyleFunc: func() core.Style {
				return core.Style{Group: "row", Direction: core.Row, AlignItems: core.AlignCenter, Gap: 10, Padding: core.All(12), CornerRadius: radiusControl,
					BorderWidth: 1, BorderColor: colors.line(), Hover: &core.Style{BorderColor: colors.accent()}}
			},
			Children: []core.Node{
				core.View{StyleFunc: func() core.Style {
					return core.Style{Width: core.Pt(28), Height: core.Pt(28), CornerRadius: 14, Background: colors.raised(), Transition: core.Transition{Duration: 150 * time.Millisecond},
						GroupStyle: &core.GroupStyle{Name: "row", Hover: &core.Style{Background: colors.accent()}, Active: &core.Style{Transform: core.Transform{Scale: 0.85}}}}
				}},
				core.Text{Content: hint(cx, "Press this row: its dot follows", "Hover this row: its dot and title follow"), StyleFunc: func() core.Style {
					return core.Style{FontSize: 14, Color: colors.muted(),
						GroupStyle: &core.GroupStyle{Name: "row", Hover: &core.Style{Color: colors.text()}}}
				}},
			}},
	)
}

// stylesDemo is how views draw: corners, borders, shadows, clipping,
// stacking, 3D transforms, numerals and direction, then NSGo's
// gradients, filters, keyframes and group styles.
func stylesDemo(cx *core.Scope) core.Node {
	return demoPage(cx, pageProps{}, cornersCard, bordersCard, shadowsCard, layersCard, transformsCard, numeralsCard, rightToLeft,
		gradientsCard, filtersCard, keyframesCard)
}
