package main

import (
	"time"

	"github.com/oneo-me/nsgo/core"

	ui "github.com/oneo-me/nsgo/examples/gallery-ui"
)

// specimenStage is the stage every specimen stands on: the card's light,
// without its drop shadow; read it in a StyleFunc.
func specimenStage(cx *core.Scope) core.Style {
	colors := ui.ColorsOf(cx)
	hl := ui.Inner(0, 1, 0, core.RGB(0xffffff))
	if cx.Dark() {
		hl = ui.Inner(0, 1, 0, ui.White(0.05))
	}
	return core.Style{Height: core.Pt(104), AlignItems: core.AlignCenter, JustifyContent: core.JustifyCenter, CornerRadius: ui.RadiusControl,
		BorderCurve: core.BorderCurveContinuous, Background: colors.Surface(), BorderWidth: 1, BorderColor: colors.Line(),
		Gradients: ui.FinOf(cx).Card().Gradients, Shadows: []core.BoxShadow{hl}}
}

// specimen is a box the styles are tried on, s over it, for a StyleFunc.
func specimen(cx *core.Scope, s core.Style) core.Style {
	return core.Style{Width: core.Pt(112), Height: core.Pt(76), AlignItems: core.AlignCenter, JustifyContent: core.JustifyCenter,
		Background: ui.ColorsOf(cx).Raised()}.Merge(s)
}

// specimenOf is a specimen with s over it.
func specimenOf(cx *core.Scope, s core.Style) func() core.Style {
	return func() core.Style { return specimen(cx, s) }
}

func specimenLabel(s string, c func() core.Color) core.Node { return ui.Text(s, 13, ui.Semibold, c) }

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
	colors := ui.ColorsOf(cx)
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
	texts := []core.Node{ui.Textf(lbl, 13, ui.Medium, colors.Text)}
	if p.code != "" {
		texts = append(texts, core.Text{Content: p.code, MaxLines: 1, Style: core.Style{FontSize: 12, FontFamily: ui.Mono}, StyleFunc: ui.Colored(colors.Muted)})
	}
	if p.note != "" {
		texts = append(texts, core.Text{Content: p.note, MaxLines: 1, Style: core.Style{FontSize: 12}, StyleFunc: ui.Colored(colors.Faint)})
	}
	return core.View{Style: core.Style{FlexBasis: core.Pct(40), FlexGrow: 1, Gap: 8}, Children: []core.Node{
		st, core.View{Style: core.Style{Gap: 3, Padding: core.Symmetric(0, 2)}, Children: texts},
	}}
}

func grid(children ...core.Node) core.Node {
	return core.View{Style: core.Style{Direction: core.Row, Wrap: core.Wrap, ColumnGap: 12, RowGap: 16}, Children: children}
}

func cornersCard(cx *core.Scope) core.Node {
	colors := ui.ColorsOf(cx)
	tints := ui.TintsOf(cx)
	shapes := []struct {
		label string
		style core.Style
		paint func() core.Style
	}{
		{"Circular", core.Style{Width: core.Pt(60), Height: core.Pt(60), CornerRadius: 20}, ui.Filled(colors.Info)},
		{"Continuous", core.Style{Width: core.Pt(60), Height: core.Pt(60), CornerRadius: 20, BorderCurve: core.BorderCurveContinuous}, ui.Filled(colors.Info)},
		{"One corner", core.Style{Width: core.Pt(60), Height: core.Pt(60), CornerRadii: core.Corners{TopLeft: 4, TopRight: 20, BottomRight: 20, BottomLeft: 20},
			BorderCurve: core.BorderCurveContinuous, BorderWidth: 2.5}, func() core.Style { return core.Style{BorderColor: colors.Accent()} }},
		{"Pill", core.Style{Width: core.Pt(60), Height: core.Pt(28), CornerRadius: 14, BorderCurve: core.BorderCurveContinuous}, ui.Filled(colors.Up)},
	}
	var row []core.Node
	for _, s := range shapes {
		row = append(row, core.View{Style: core.Style{AlignItems: core.AlignCenter, Gap: 10}, Children: []core.Node{
			core.View{Style: core.Style{Height: core.Pt(64), JustifyContent: core.JustifyCenter}, Children: []core.Node{core.View{Style: s.style, StyleFunc: s.paint}}},
			ui.Text(s.label, 12, ui.Regular, colors.Muted),
		}})
	}
	return ui.Card{Icon: ui.IconStyles, Title: "Corners", Description: "CornerRadius · BorderCurve · per corner", Children: []core.Node{
		core.View{Style: core.Style{Direction: core.Row, JustifyContent: core.JustifySpaceBetween}, Children: row},
		ui.Caption{Label: "Continuous corners ease into the edges", Note: label("Radius 20 pt")},
		grid(
			tile(cx, tileProps{label: "Top corners", code: "CornerRadii", note: "28 pt"},
				core.View{StyleFunc: func() core.Style {
					return specimen(cx, core.Style{CornerRadii: core.Corners{TopLeft: 28, TopRight: 28}, BorderWidth: 1.5, BorderColor: colors.Accent()})
				},
					Children: []core.Node{specimenLabel("28 pt", colors.Accent)}}),
			tile(cx, tileProps{label: "Mixed radii", note: "44 · 6 · 28 · 0 pt, clockwise"},
				core.View{StyleFunc: func() core.Style {
					return specimen(cx, core.Style{Background: tints.Info(), CornerRadii: core.Corners{TopLeft: 44, TopRight: 6, BottomRight: 28}, BorderWidth: 2, BorderColor: colors.Info()})
				},
					Children: []core.Node{specimenLabel("Mixed", colors.Info)}}),
		),
	}}
}

func bordersCard(cx *core.Scope) core.Node {
	colors := ui.ColorsOf(cx)
	return ui.Card{Icon: ui.IconComponents, Title: "Borders", Description: "Per side · dashed · dotted · outlines", Children: []core.Node{
		grid(
			tile(cx, tileProps{label: "Per-side borders", note: "6 · 2 · 4 · 1 pt, clockwise"},
				core.View{StyleFunc: func() core.Style {
					return specimen(cx, core.Style{Borders: core.Edges{Top: 6, Right: 2, Bottom: 4, Left: 1},
						BorderColors: core.EdgeColors{Top: colors.Danger(), Right: colors.Up(), Bottom: colors.Info(), Left: colors.Warn()}})
				}}),
			tile(cx, tileProps{label: "Dashed / dotted", code: "BorderStyle"},
				core.View{Style: core.Style{Gap: 8}, Children: []core.Node{
					core.View{StyleFunc: func() core.Style {
						return core.Style{Width: core.Pt(112), Height: core.Pt(28), CornerRadius: 8, BorderWidth: 1.5, BorderColor: colors.Info(), BorderStyle: core.BorderDashed}
					}},
					core.View{StyleFunc: func() core.Style {
						return core.Style{Width: core.Pt(112), Height: core.Pt(28), CornerRadius: 8, BorderWidth: 3, BorderColor: colors.Warn(), BorderStyle: core.BorderDotted}
					}},
				}}),
			tile(cx, tileProps{label: "Rings", code: "Ring", note: "Box shadows of no blur"},
				core.View{StyleFunc: func() core.Style {
					return specimen(cx, core.Style{CornerRadius: 14, Shadows: []core.BoxShadow{core.Ring(3, colors.Surface()), core.Ring(5, colors.Accent())}})
				}}),
			tile(cx, tileProps{label: "Outline", code: "OutlineOffset", note: "Outside the border box"},
				core.View{StyleFunc: func() core.Style {
					return specimen(cx, core.Style{CornerRadius: 14, OutlineWidth: 2, OutlineColor: colors.Info(), OutlineOffset: 4, OutlineStyle: core.BorderDashed})
				}}),
		),
	}}
}

// shadowsCard is shadows that read on a dark page: colored glows rather
// than black.
func shadowsCard(cx *core.Scope) core.Node {
	colors := ui.ColorsOf(cx)
	tints := ui.TintsOf(cx)
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
	return ui.Card{Icon: ui.IconSpark, Title: "Shadows and glow", Description: "ShadowColor · box shadows · text shadow", Children: []core.Node{
		grid(
			tile(cx, tileProps{label: "Glow", code: "ShadowColor", note: "A colored shadow"},
				core.View{StyleFunc: func() core.Style {
					return specimen(cx, core.Style{CornerRadius: 16, BorderCurve: core.BorderCurveContinuous, Background: colors.AccentFill(),
						ShadowColor: ui.Alpha(colors.AccentFill(), 0.55), ShadowRadius: 9, ShadowOffset: core.Point{Y: 6}})
				}, Children: []core.Node{specimenLabel("Glow", colors.OnAccent)}}),
			tile(cx, tileProps{label: "Offset shadow", code: "ShadowOffset", note: "4, 8 pt · radius 10"},
				core.View{StyleFunc: func() core.Style {
					return specimen(cx, core.Style{Background: offsetBg(), CornerRadii: core.Corners{TopLeft: 30, BottomRight: 30}, BorderWidth: 1, BorderColor: colors.Info(),
						ShadowColor: ui.Alpha(colors.Info(), 0.45), ShadowRadius: 5, ShadowOffset: core.Point{X: 4, Y: 8}})
				}, Children: []core.Node{specimenLabel("Offset", colors.Info)}}),
			tile(cx, tileProps{label: "Image corners", note: "Two radii and a shadow"},
				core.Image{Source: core.Src("https://picsum.photos/seed/nsh/300/220"), Style: core.Style{Width: core.Pt(112), Height: core.Pt(76),
					CornerRadii: core.Corners{TopLeft: 28, BottomRight: 28}, ShadowRadius: 6, ShadowOffset: core.Point{Y: 8}},
					StyleFunc: func() core.Style { return core.Style{Background: colors.Raised(), ShadowColor: ui.Black(shade())} }}),
			tile(cx, tileProps{label: "Text shadow", code: "TextShadowColor"},
				core.Text{Content: "Shadow", StyleFunc: func() core.Style {
					return core.Style{FontSize: 24, FontWeight: core.FontWeight(800), Color: colors.Text(), LetterSpacing: -0.5,
						TextShadowColor: ui.Alpha(colors.Danger(), 0.9), TextShadowRadius: 4}
				}}),
			tile(cx, tileProps{label: "Text corners", note: "A span's background and radius"},
				core.Text{Spans: []core.Span{{Content: " Text corners ", StyleFunc: func() core.Style { return core.Style{Background: tints.Up(), CornerRadius: 10} }}},
					Style: core.Style{FontSize: 15, FontWeight: ui.Semibold, LineHeight: 30}, StyleFunc: ui.Colored(colors.Up)}),
			tile(cx, tileProps{label: "Layered", code: "Shadows", note: "Two box shadows and an inset one"},
				core.View{StyleFunc: func() core.Style {
					return specimen(cx, core.Style{CornerRadius: 16, Background: colors.Surface(), Shadows: []core.BoxShadow{
						ui.Inner(0, -2, 6, ui.Alpha(colors.Accent(), 0.35)), ui.Shadow(0, 1, 2, 0, ui.Black(0.08)), ui.Shadow(0, 10, 24, -4, ui.Black(shade()))}})
				},
					Children: []core.Node{specimenLabel("Layered", colors.Text)}}),
		),
	}}
}

// layersCard is clipping, stacking order and opacity, each a press away.
func layersCard(cx *core.Scope) core.Node {
	colors := ui.ColorsOf(cx)
	tints := ui.TintsOf(cx)
	clip := core.NewSignal(cx, true)
	front := core.NewSignal(cx, 0)
	opacity := core.NewAnimatedValue(cx, 0.45)
	level := core.NewSignal(cx, "0.45")
	stack := []func() core.Color{colors.Danger, colors.Info, colors.Up}
	var cards []core.Node
	for i, c := range stack {
		cards = append(cards, core.View{StyleFunc: func() core.Style {
			shade := float32(0.18)
			if cx.Dark() {
				shade = 0.5
			}
			s := core.Style{Position: core.Absolute, Left: core.Pt(4 + core.Points(i)*28), Top: core.Pt(2 + core.Points(i)*14), Width: core.Pt(64), Height: core.Pt(52),
				CornerRadius: 12, BorderCurve: core.BorderCurveContinuous, Background: c(), ShadowColor: ui.Black(shade), ShadowRadius: 3, ShadowOffset: core.Point{Y: 3}}
			if front.Get() == i {
				s.BorderWidth, s.BorderColor, s.ZIndex = 2, colors.Text(), 10
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
	return ui.Card{Icon: ui.IconLists, Title: "Clipping and layers", Description: "Overflow · ZIndex · Opacity", Actions: []core.Node{ui.Switch{Checked: clip.Get, OnChange: clip.Set, Label: "Clip"}}, Children: []core.Node{
		grid(
			tile(cx, tileProps{labelFn: func() string {
				if clip.Get() {
					return "Clipped"
				}
				return "Visible"
			}, code: "Overflow: Hidden / Visible", note: ui.Hint(cx, "Tap to switch", "Click to switch"), onPress: func() { toggled(clip) }},
				core.View{StyleFunc: func() core.Style {
					o := core.Hidden
					if !clip.Get() {
						o = core.Visible
					}
					return specimen(cx, core.Style{Background: tints.Warn(), CornerRadius: 22, BorderWidth: 1.5, BorderColor: colors.Warn(), Overflow: o})
				}, Children: []core.Node{
					core.View{Style: core.Style{Position: core.Absolute, Left: core.Pt(-26), Top: core.Pt(-26), Width: core.Pt(72), Height: core.Pt(72), CornerRadius: 36}, StyleFunc: ui.Filled(colors.Danger)},
					specimenLabel(ui.Hint(cx, "Tap", "Click"), colors.Warn),
				}}),
			tile(cx, tileProps{label: "Stacking order", code: "ZIndex", note: ui.Hint(cx, "Tap to raise the next", "Click to raise the next"), onPress: func() { front.Update(func(n int) int { return (n + 1) % 3 }) }},
				core.View{Style: core.Style{Width: core.Pt(130), Height: core.Pt(84)}, Children: cards}),
		),
		core.View{Style: core.Style{Direction: core.Row, AlignItems: core.AlignCenter, Gap: 12}, Children: []core.Node{
			core.View{Style: core.Style{Width: core.Pt(64), Height: core.Pt(64), CornerRadius: 14, BorderCurve: core.BorderCurveContinuous,
				AlignItems: core.AlignCenter, JustifyContent: core.JustifyCenter, Animated: &core.AnimatedStyle{Opacity: opacity}}, StyleFunc: ui.Filled(colors.Info),
				Children: []core.Node{ui.Textf(level.Get, 13, ui.Bold, onBlue)}},
			core.View{Style: core.Style{FlexGrow: 1, FlexShrink: 1, Gap: 8}, Children: []core.Node{
				ui.Caption{Label: "Opacity", Note: label("Animated natively")},
				ui.Segmented[string]{Options: []ui.Option[string]{{Value: "0.2", Label: "0.2"}, {Value: "0.45", Label: "0.45"}, {Value: "1", Label: "1"}}, Value: level.Get, OnChange: func(v string) {
					level.Set(v)
					to := map[string]float64{"0.2": 0.2, "0.45": 0.45, "1": 1}[v]
					ui.Timing(cx, opacity, to, core.TimingConfig{Duration: ms(260), Easing: ui.EaseOutCubic})
				}},
			}},
		}},
	}}
}

// flipCard is two faces back to back: each hides while turned away.
func flipCard(cx *core.Scope) core.Node {
	colors := ui.ColorsOf(cx)
	angle := core.NewAnimatedValue(cx, 0)
	flipped := false
	flip := func() {
		flipped = !flipped
		to := 0.0
		if flipped {
			to = 180
		}
		ui.Spring(cx, angle, to, core.SpringConfig{Stiffness: 140, Damping: 16})
	}
	// A face's background, gradient and rim, light then dark.
	type paint struct{ bg, from, to, border core.Color }
	face := func(light, dark paint, rotate core.AnimatedNode, badgeText string, t ui.Tone, degrees, title, body string) core.Node {
		return core.View{Style: core.Style{Position: core.Absolute, Left: core.Pt(0), Top: core.Pt(0), Right: core.Pt(0), Bottom: core.Pt(0), CornerRadius: ui.RadiusCard,
			BorderCurve: core.BorderCurveContinuous, BorderWidth: 1, Padding: core.All(16), JustifyContent: core.JustifySpaceBetween, BackfaceHidden: true,
			Transform: core.Transform{Perspective: 800}, Animated: &core.AnimatedStyle{RotateY: rotate}}, StyleFunc: func() core.Style {
			p, sh := light, []core.BoxShadow{ui.Inner(0, 1, 0, core.RGB(0xffffff)), ui.Shadow(0, 8, 20, 0, ui.Paper(0.10))}
			if cx.Dark() {
				p, sh = dark, []core.BoxShadow{ui.Inner(0, 1, 0, ui.White(0.07)), ui.Shadow(0, 10, 24, 0, ui.Black(0.4))}
			}
			return core.Style{BorderColor: p.border, Background: p.bg, Gradients: []core.Gradient{core.LinearGradient(180, p.from, p.to)}, Shadows: sh}
		}, Children: []core.Node{
			core.View{Style: core.Style{Direction: core.Row, JustifyContent: core.JustifySpaceBetween}, Children: []core.Node{ui.Badge{Text: badgeText, Tone: t}, ui.Text(degrees, 13, ui.Regular, colors.Muted)}},
			core.View{Style: core.Style{Gap: 2}, Children: []core.Node{ui.Text(title, 20, ui.Semibold, colors.Text), ui.Text(body, 13, ui.Regular, colors.Muted)}},
		}}
	}
	front := face(paint{core.RGB(0xeef2fc), core.RGB(0xf0f4fd), core.RGB(0xebf0fb), core.RGB(0xcdd8f3)},
		paint{core.RGB(0x1a2134), core.RGB(0x1e263b), core.RGB(0x1a2134), core.RGB(0x2b3a5c)},
		angle, "front", ui.ToneInfo, "0°", ui.Hint(cx, "Tap to flip", "Click to flip"), "A spring turns it on the UI thread")
	back := face(paint{core.RGB(0xfbefe6), core.RGB(0xfcf2ea), core.RGB(0xfaebe0), core.RGB(0xf0cdb3)},
		paint{core.RGB(0x2a1d14), core.RGB(0x2f2116), core.RGB(0x291c13), core.RGB(0x4e3423)},
		core.Add(angle, core.Constant(180)), "back", ui.ToneAccent, "180°", "BackfaceHidden", "Each face hides while turned away")
	return ui.Press{OnPress: flip, Scale: 0.98, Label: "Flip the card", Children: []core.Node{core.View{Style: core.Style{Height: core.Pt(132)}, Children: []core.Node{front, back}}}}
}

func transformsCard(cx *core.Scope) core.Node {
	colors := ui.ColorsOf(cx)
	tints := ui.TintsOf(cx)
	// A slow sway on the platform, so the perspective reads as depth.
	sway := core.NewAnimatedValue(cx, -30)
	sine := ease(core.EasingSin, core.EaseModeInOut)
	run := cx.StartAnimation(core.Loop(core.Sequence(
		core.Timing(sway, 30, core.TimingConfig{Duration: ms(2200), Easing: sine}),
		core.Timing(sway, -30, core.TimingConfig{Duration: ms(2200), Easing: sine}),
	), 0), nil)
	cx.Cleanup(run.Stop)
	return ui.Card{Icon: ui.IconPhysics, Title: "3D transforms", Description: "Perspective · RotateX/Y · Skew · origin", Children: []core.Node{
		flipCard(cx),
		grid(
			tile(cx, tileProps{label: "Perspective", code: "RotateY", note: "Swaying, driven natively"},
				core.View{StyleFunc: func() core.Style {
					return specimen(cx, core.Style{CornerRadius: 14, Background: tints.Info(), BorderWidth: 1, BorderColor: colors.Info(),
						Transform: core.Transform{Perspective: 400}, Animated: &core.AnimatedStyle{RotateY: sway}})
				}, Children: []core.Node{specimenLabel("Y axis", colors.Info)}}),
			tile(cx, tileProps{label: "Tilt", code: "RotateX: 40"},
				core.View{StyleFunc: func() core.Style {
					return specimen(cx, core.Style{CornerRadius: 14, Background: tints.Accent(), BorderWidth: 1, BorderColor: colors.Accent(),
						Transform: core.Transform{Perspective: 400, RotateX: 40}})
				}, Children: []core.Node{specimenLabel("X axis", colors.Accent)}}),
			tile(cx, tileProps{label: "Skew", code: "SkewX: -12"},
				core.View{StyleFunc: func() core.Style {
					return specimen(cx, core.Style{CornerRadius: 10, Background: tints.Warn(), BorderWidth: 1, BorderColor: colors.Warn(),
						Transform: core.Transform{SkewX: -12}})
				}, Children: []core.Node{specimenLabel("Skew", colors.Warn)}}),
			tile(cx, tileProps{label: "Origin", code: "TransformOrigin", note: "Top left · rotated 30°"},
				core.View{Style: core.Style{Width: core.Pt(112), Height: core.Pt(84)}, Children: []core.Node{
					core.View{StyleFunc: func() core.Style {
						return core.Style{Position: core.Absolute, Left: core.Pt(30), Top: core.Pt(14), Width: core.Pt(54), Height: core.Pt(54), CornerRadius: 6,
							BorderWidth: 1, BorderColor: colors.Faint(), BorderStyle: core.BorderDashed}
					}},
					core.View{StyleFunc: func() core.Style {
						return core.Style{Position: core.Absolute, Left: core.Pt(30), Top: core.Pt(14), Width: core.Pt(54), Height: core.Pt(54), CornerRadius: 6,
							Background: ui.Alpha(colors.Danger(), 0.55), Transform: core.Transform{Rotate: 30},
							TransformOrigin: core.ObjectPosition{X: core.PositionAxis{Offset: core.Pct(0)}, Y: core.PositionAxis{Offset: core.Pct(0)}}}
					}},
					core.View{Style: core.Style{Position: core.Absolute, Left: core.Pt(26), Top: core.Pt(10), Width: core.Pt(8), Height: core.Pt(8), CornerRadius: 4}, StyleFunc: ui.Filled(colors.Text)},
				}}),
		),
	}}
}

// numeralsCard is figures in their proportional and tabular widths, then
// small capitals and oldstyle figures: the font's OpenType features.
func numeralsCard(cx *core.Scope) core.Node {
	colors := ui.ColorsOf(cx)
	figure := func(s string, v core.FontVariant) core.Node {
		t := ui.Text(s, 22, ui.Medium, colors.Text)
		t.Style.FontVariant = v
		return t
	}
	column := func(head core.Node, v core.FontVariant) core.Node {
		return core.View{Style: core.Style{FlexGrow: 1, FlexShrink: 1, FlexBasis: core.Pct(0), Gap: 4}, Children: []core.Node{
			head, figure("1,111.11", v), figure("8,888.88", v)}}
	}
	tabular := ui.Text("tabular-nums", 12, ui.Regular, colors.Accent)
	tabular.Style.FontFamily = ui.Mono
	feature := func(label string, v core.FontVariant, name string) core.Node {
		l := ui.Text(label, 18, ui.Regular, colors.Text)
		l.Style.FontVariant, l.Style.FlexGrow, l.Style.FlexShrink = v, 1, 1
		n := ui.Text(name, 13, ui.Regular, colors.Faint)
		n.Style.FontFamily = ui.Mono
		return core.View{Style: core.Style{Direction: core.Row, AlignItems: core.AlignBaseline, Gap: 8}, Children: []core.Node{l, n}}
	}
	return ui.Card{Icon: ui.IconText, Title: "Numerals", Description: "FontVariant", Children: []core.Node{
		core.View{Style: core.Style{Direction: core.Row, Gap: 12}, Children: []core.Node{
			column(ui.Text("Proportional", 12, ui.Regular, colors.Muted), 0),
			core.View{Style: core.Style{Width: core.Pt(1)}, StyleFunc: ui.Filled(colors.Line)},
			column(tabular, core.FontTabularNums),
		}},
		core.View{Style: core.Style{Height: core.Pt(1)}, StyleFunc: ui.Filled(colors.Line)},
		feature("Small Caps", core.FontSmallCaps, "small-caps"),
		feature("Oldstyle 1234567890", core.FontOldstyleNums, "oldstyle-nums"),
	}}
}

// rightToLeft is rows, start margins and text under a right-to-left
// layout: the app's language decides it, and LayoutDirection sets a
// view's own.
func rightToLeft(cx *core.Scope) core.Node {
	colors := ui.ColorsOf(cx)
	tints := ui.TintsOf(cx)
	rtl := core.NewSignal(cx, false)
	tones := []func() core.Color{colors.Danger, colors.Info, colors.Up}
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
		boxes = append(boxes, core.View{Style: s, StyleFunc: ui.Filled(c), Children: []core.Node{ui.Text(itoa(i+1), 15, ui.Bold, onTile)}})
	}
	chip := func(c, bg func() core.Color, n int) core.Node {
		return core.View{Style: core.Style{Width: core.Pt(40), Height: core.Pt(40), CornerRadius: 10, BorderCurve: core.BorderCurveContinuous,
			AlignItems: core.AlignCenter, JustifyContent: core.JustifyCenter}, StyleFunc: ui.Filled(bg), Children: []core.Node{ui.Text(itoa(n), 13, ui.Regular, c)}}
	}
	var chips []core.Node
	for n := 1; n <= 12; n++ {
		chips = append(chips, chip(colors.Info, tints.Info, n))
	}
	monoNote := func(s string) func() string { return label(s) }
	return ui.Card{Icon: ui.IconNav, Title: "Right to left", Description: "", Actions: []core.Node{ui.Button{Text: "Flip", Variant: ui.ButtonSecondary, Small: true, OnPress: func() { toggled(rtl) }}}, Children: []core.Node{
		core.View{StyleFunc: func() core.Style {
			d := core.DirectionLTR
			if rtl.Get() {
				d = core.DirectionRTL
			}
			return core.Style{LayoutDirection: d, Gap: 12}
		}, Children: []core.Node{
			ui.Caption{LabelFunc: func() string {
				if rtl.Get() {
					return "LayoutDirection: RTL"
				}
				return "LayoutDirection: LTR"
			}},
			core.View{Style: core.Style{Direction: core.Row, Gap: 8}, Children: boxes},
			core.Text{Content: "Text aligns to the start side; the first box has a start margin of 16 and a round top-start corner.",
				Style: core.Style{FontSize: 15, LineHeight: 21}, StyleFunc: ui.Colored(colors.Text)},
			ui.Caption{Label: "Horizontal scroll view", Note: monoNote("Scroll")},
			core.Scroll{Horizontal: true, HideIndicator: true, Style: core.Style{Height: core.Pt(40)}, Children: []core.Node{
				core.View{Style: core.Style{Direction: core.Row, Gap: 6}, Children: chips}}},
			ui.Caption{Label: "Horizontal list", Note: monoNote("List")},
			core.List{Count: func() int { return 30 }, Horizontal: true, EstimatedSize: 46, HideIndicator: true, Gap: 6, Style: core.Style{Height: core.Pt(40)},
				Row: func(cx *core.Scope, r core.ListRow) core.Node {
					colors := ui.ColorsOf(cx)
					tints := ui.TintsOf(cx)
					return core.View{StyleFunc: func() core.Style {
						return core.Style{Width: core.Pt(40), Height: core.Pt(40), CornerRadius: 10, AlignItems: core.AlignCenter, JustifyContent: core.JustifyCenter, Background: tints.Accent()}
					},
						Children: []core.Node{ui.Textf(func() string { return itoa(r.Index() + 1) }, 13, ui.Regular, colors.Accent)}}
				}},
			core.View{Style: core.Style{Height: core.Pt(1)}, StyleFunc: ui.Filled(colors.Line)},
			core.Text{Content: "TextAlignStart: the start side, right to left", Style: core.Style{FontSize: 13, TextAlign: core.TextAlignStart}, StyleFunc: ui.Colored(colors.Muted)},
			core.Text{Content: "مرحبا بالعالم — WritingDirection RTL", Style: core.Style{FontSize: 13, WritingDirection: core.WritingRTL}, StyleFunc: ui.Colored(colors.Muted)},
		}},
	}}
}

func gradientsCard(cx *core.Scope) core.Node {
	colors := ui.ColorsOf(cx)
	tints := ui.TintsOf(cx)
	blue, yellow, pink := core.RGB(0x2563eb), core.RGB(0xfacc15), core.RGB(0xec4899)
	return ui.Card{Icon: ui.IconStyles, Title: "Gradients", Description: "Linear, radial and conic, mixed in Oklab", Actions: []core.Node{ui.Badge{Text: "NSGo"}}, Children: []core.Node{
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
						Stops: []core.ColorStop{{Color: colors.Raised(), At: core.Pt(0)}, {Color: colors.Raised(), At: core.Pt(8)}, {Color: tints.Accent(), At: core.Pt(8)}, {Color: tints.Accent(), At: core.Pt(16)}}}}}
				}},
			}}),
		),
	}}
}

func filtersCard(cx *core.Scope) core.Node {
	colors := ui.ColorsOf(cx)
	paint := []core.Gradient{core.LinearGradient(135, core.RGB(0xf43f5e), core.RGB(0xf59e0b), core.RGB(0x10b981), core.RGB(0x3b82f6))}
	art := func(name string, f ...core.Filter) core.Node {
		return core.View{Style: core.Style{AlignItems: core.AlignCenter, Gap: 6}, Children: []core.Node{
			core.View{Style: core.Style{Width: core.Pt(56), Height: core.Pt(56), CornerRadius: 12, Gradients: paint, Filters: f}},
			ui.Text(name, 12, ui.Regular, colors.Muted),
		}}
	}
	stripes := func() core.Style {
		accent, surface := colors.Accent(), colors.Surface()
		return core.Style{Gradients: []core.Gradient{{Angle: 90, Repeating: true, Space: core.ColorSpaceSRGB, Stops: []core.ColorStop{
			{Color: accent, At: core.Pt(0)}, {Color: accent, At: core.Pt(6)}, {Color: surface, At: core.Pt(6)}, {Color: surface, At: core.Pt(12)}}}}}
	}
	pane := func(name string, s func() core.Style) core.Node {
		return core.View{Style: core.Style{Width: core.Pt(88), Height: core.Pt(60), CornerRadius: 12, AlignItems: core.AlignCenter, JustifyContent: core.JustifyCenter},
			StyleFunc: s, Children: []core.Node{ui.Text(name, 12, ui.Semibold, colors.Text)}}
	}
	return ui.Card{Icon: ui.IconImage, Title: "Filters, backdrops and masks", Description: "Filters · BackdropFilters · Mask · BlendMode", Actions: []core.Node{ui.Badge{Text: "NSGo"}}, Children: []core.Node{
		core.View{Style: core.Style{Direction: core.Row, Wrap: core.Wrap, Gap: 10}, Children: []core.Node{
			art("none"), art("grayscale", core.Grayscale(1)), art("sepia", core.Sepia(1)), art("hue 120°", core.HueRotate(120)),
			art("invert", core.Invert(1)), art("blur 4", core.Blur(4)),
		}},
		core.View{Style: core.Style{Direction: core.Row, Wrap: core.Wrap, Gap: 10, Padding: core.All(12), CornerRadius: ui.RadiusControl}, StyleFunc: stripes, Children: []core.Node{
			pane("frosted", func() core.Style {
				return core.Style{Background: ui.Alpha(colors.Surface(), 0.4), BackdropFilters: []core.Filter{core.Blur(8)}}
			}),
			pane("glass", func() core.Style {
				return core.Style{Background: ui.Alpha(colors.Surface(), 0.3), BackdropFilters: []core.Filter{core.Blur(12)}, GlassEffect: core.GlassRegular, CornerRadius: 24}
			}),
			pane("masked", func() core.Style {
				return core.Style{Background: colors.Info(), Mask: &core.Gradient{Angle: 90, Stops: []core.ColorStop{{Color: core.RGB(0)}, {Color: 0}}}}
			}),
			pane("multiply", func() core.Style { return core.Style{Background: core.RGB(0x60a5fa), BlendMode: core.BlendMultiply} }),
		}},
	}}
}

func keyframesCard(cx *core.Scope) core.Node {
	colors := ui.ColorsOf(cx)
	dot := func(name string, k core.Keyframes, s func() core.Style) core.Node {
		return core.View{Style: core.Style{AlignItems: core.AlignCenter, Gap: 8, Width: core.Pt(64)}, Children: []core.Node{
			core.View{Style: core.Style{Width: core.Pt(32), Height: core.Pt(32), Margin: core.Edges{Top: 8}, Animation: core.KeyframeAnimation{Keyframes: k}}, StyleFunc: s},
			ui.Text(name, 12, ui.Regular, colors.Muted),
		}}
	}
	return ui.Card{Icon: ui.IconAnimation, Title: "Keyframes and group styles", Description: "Animation · GroupStyle", Actions: []core.Node{ui.Badge{Text: "NSGo"}}, Children: []core.Node{
		core.View{Style: core.Style{Direction: core.Row, JustifyContent: core.JustifySpaceAround}, Children: []core.Node{
			dot("spin", core.KeyframesSpin, func() core.Style {
				raised := colors.Raised()
				return core.Style{CornerRadius: 16, Borders: core.All(4), BorderColors: core.EdgeColors{Top: colors.Accent(), Right: raised, Bottom: raised, Left: raised}}
			}),
			dot("ping", core.KeyframesPing, func() core.Style { return core.Style{Background: colors.Accent(), CornerRadius: 16} }),
			dot("pulse", core.KeyframesPulse, func() core.Style { return core.Style{Background: colors.Accent(), CornerRadius: 6} }),
			dot("bounce", core.KeyframesBounce, func() core.Style { return core.Style{Background: colors.Accent(), CornerRadius: 16} }),
		}},
		core.View{OnPress: func(core.PressEvent) {}, Role: core.RoleButton, Label: "Hover this row",
			StyleFunc: func() core.Style {
				return core.Style{Group: "row", Direction: core.Row, AlignItems: core.AlignCenter, Gap: 10, Padding: core.All(12), CornerRadius: ui.RadiusControl,
					BorderWidth: 1, BorderColor: colors.Line(), Hover: &core.Style{BorderColor: colors.Accent()}}
			},
			Children: []core.Node{
				core.View{StyleFunc: func() core.Style {
					return core.Style{Width: core.Pt(28), Height: core.Pt(28), CornerRadius: 14, Background: colors.Raised(), Transition: core.Transition{Duration: 150 * time.Millisecond},
						GroupStyle: &core.GroupStyle{Name: "row", Hover: &core.Style{Background: colors.Accent()}, Active: &core.Style{Transform: core.Transform{Scale: 0.85}}}}
				}},
				core.Text{Content: ui.Hint(cx, "Press this row: its dot follows", "Hover this row: its dot and title follow"), StyleFunc: func() core.Style {
					return core.Style{FontSize: 14, Color: colors.Muted(),
						GroupStyle: &core.GroupStyle{Name: "row", Hover: &core.Style{Color: colors.Text()}}}
				}},
			}},
	}}
}

// stylesDemo is how views draw: corners, borders, shadows, clipping,
// stacking, 3D transforms, numerals and direction, then NSGo's
// gradients, filters, keyframes and group styles.
func stylesDemo(cx *core.Scope) core.Node {
	return demoPage(cx, pageProps{}, cornersCard, bordersCard, shadowsCard, layersCard, transformsCard, numeralsCard, rightToLeft,
		gradientsCard, filtersCard, keyframesCard)
}
