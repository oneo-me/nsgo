package main

import (
	_ "embed"
	"fmt"
	"slices"

	"github.com/oneo-me/nsgo/core"

	ui "github.com/oneo-me/nsgo/examples/gallery-ui"
	"github.com/oneo-me/nsgo/examples/gallery/locales"
)

//go:embed assets/bounce.gif
var bounceGIF []byte

//go:embed assets/spinner.webp
var spinnerWebP []byte

//go:embed assets/mark.svg
var markSVG []byte

//go:embed assets/star.svg
var starSVG []byte

// Fixed photos, in color, large enough to stay sharp at 3x.
const widePhoto = "https://picsum.photos/id/1015/1200/600"

func photoURL(id, w, h int) string { return fmt.Sprintf("https://picsum.photos/id/%d/%d/%d", id, w, h) }

// remoteSVG is the W3C's SVG logo.
const remoteSVG = "https://www.w3.org/Icons/SVG/svg-logo.svg"

// logoSource is the gallery's logo at 64 points.
func logoSource() core.ImageSource {
	return core.ImageSource{Data: logoPNG, Size: core.Size{Width: 64, Height: 64}}
}

// imageFrame is every photo's frame: rounded, a hairline border, a raised
// fill while it loads, frameInk's colors.
func imageFrame(cx *core.Scope, s core.Style) core.Style {
	return core.Style{CornerRadius: ui.RadiusControl, BorderCurve: core.BorderCurveContinuous, BorderWidth: 1}.Merge(s)
}

// frameInk is an image frame's colors, for its StyleFunc.
func frameInk(cx *core.Scope) func() core.Style {
	colors := ui.ColorsOf(cx)
	return func() core.Style { return core.Style{BorderColor: colors.Border(), Background: colors.Raised()} }
}

// imageNote is a quiet label under a tile, code for API values.
func imageNote(cx *core.Scope, s string, isCode bool) core.Node {
	colors := ui.ColorsOf(cx)
	st := core.Style{FontSize: 13, TextAlign: core.TextAlignCenter}
	if isCode {
		st.FontFamily = ui.Mono
	}
	return core.Text{Content: s, MaxLines: 1, Style: st, StyleFunc: ui.Colored(colors.Muted)}
}

// imageTile is something centered on a raised square, a note below.
func imageTile(cx *core.Scope, noteText string, isCode bool, height core.Points, child core.Node) core.Node {
	colors := ui.ColorsOf(cx)
	fin := ui.FinOf(cx)
	if height == 0 {
		height = 76
	}
	return core.View{Style: core.Style{FlexGrow: 1, FlexBasis: core.Pt(0), Gap: 8}, Children: []core.Node{
		core.View{StyleFunc: func() core.Style {
			return fin.Raised().Over(core.Style{Height: core.Pt(height), CornerRadius: ui.RadiusControl, BorderCurve: core.BorderCurveContinuous, Background: colors.Raised(),
				BorderWidth: 1, BorderColor: colors.Border(), AlignItems: core.AlignCenter, JustifyContent: core.JustifyCenter, Overflow: core.Hidden})
		}, Children: []core.Node{child}},
		imageNote(cx, noteText, isCode),
	}}
}

func resizeModes(cx *core.Scope) core.Node {
	tr := locales.For(cx)
	modes := []struct {
		name string
		mode core.ResizeMode
	}{{"Cover", core.Cover}, {"Contain", core.Contain}, {"Stretch", core.Stretch}, {"Center", core.Center}}
	var rows []core.Node
	for i := 0; i < len(modes); i += 2 {
		var cells []core.Node
		for _, m := range modes[i : i+2] {
			cells = append(cells, core.View{Style: core.Style{FlexGrow: 1, FlexBasis: core.Pt(0), Gap: 8}, Children: []core.Node{
				core.Image{Source: core.Src(widePhoto), ResizeMode: m.mode, Label: m.name, Style: imageFrame(cx, core.Style{Height: core.Pt(140)}), StyleFunc: frameInk(cx)},
				imageNote(cx, m.name, true),
			}})
		}
		rows = append(rows, core.View{Style: core.Style{Direction: core.Row, Gap: 12}, Children: cells})
	}
	return ui.Card{Title: tr.ImagesResizeModes(), Description: tr.ImagesA21ImageIn(), Children: rows}
}

func imageCorners(cx *core.Scope) core.Node {
	tr := locales.For(cx)
	colors := ui.ColorsOf(cx)
	sq := imageFrame(cx, core.Style{Width: core.Pt(88), Height: core.Pt(88)})
	item := func(s core.Style, ink func() core.Style, name string) core.Node {
		return core.View{Style: core.Style{AlignItems: core.AlignCenter, Gap: 8}, Children: []core.Node{
			core.Image{Source: core.Src(widePhoto), Style: sq.Merge(s), StyleFunc: ink}, imageNote(cx, name, false)}}
	}
	return ui.Card{Title: tr.ImagesCornersAndBorders(), Description: tr.ImagesRadiiPerCornerBordersOn(), Children: []core.Node{
		core.View{Style: core.Style{Direction: core.Row, JustifyContent: core.JustifySpaceBetween}, Children: []core.Node{
			item(core.Style{CornerRadius: 44}, frameInk(cx), tr.ImagesCircle()),
			item(core.Style{CornerRadius: 18, BorderWidth: 2}, func() core.Style { return frameInk(cx)().Merge(core.Style{BorderColor: colors.Accent()}) }, tr.ImagesBorder()),
			item(core.Style{CornerRadii: core.Corners{TopLeft: 40, BottomRight: 40}}, frameInk(cx), tr.ImagesTwoCorners()),
		}},
	}}
}

func assetsCard(cx *core.Scope) core.Node {
	tr := locales.For(cx)
	colors := ui.ColorsOf(cx)
	tile := func(name string, tint func() core.Color) core.Node {
		var ink func() core.Style
		if tint != nil {
			ink = func() core.Style { return core.Style{TintColor: tint()} }
		}
		return imageTile(cx, name, false, 0, core.Image{Source: logoSource(), Style: core.Style{Width: core.Pt(56), Height: core.Pt(56)}, StyleFunc: ink})
	}
	return ui.Card{Title: tr.ImagesEmbeddedImagesAndTint(), Description: tr.ImagesLogoPngFromGoEmbed(), Children: []core.Node{
		core.View{Style: core.Style{Direction: core.Row, Gap: 12}, Children: []core.Node{
			tile(tr.ImagesOriginal(), nil), tile(tr.ImagesAccent(), colors.Accent), tile(tr.ImagesInfo(), colors.Info), tile(tr.ImagesDanger(), colors.Danger),
		}},
	}}
}

// svgCard is SVG images: a one-color star tinted at three sizes and as
// drawn, a two-color logo in two resize modes, and an SVG from a URL.
func svgCard(cx *core.Scope) core.Node {
	tr := locales.For(cx)
	colors := ui.ColorsOf(cx)
	star := core.ImageSource{Data: starSVG, Size: core.Size{Width: 24, Height: 24}}
	mark := core.ImageSource{Data: markSVG, Size: core.Size{Width: 120, Height: 60}}
	var sizes []core.Node
	for _, s := range []core.Points{18, 36, 72} {
		sizes = append(sizes, imageTile(cx, fmt.Sprintf("%.0f pt", s), false, 96, core.Image{Source: star, TestID: fmt.Sprintf("svg-star-%.0f", s),
			StyleFunc: func() core.Style {
				return core.Style{Width: core.Pt(s), Height: core.Pt(s), TintColor: colors.Accent()}
			}}))
	}
	sizes = append(sizes, imageTile(cx, tr.ImagesAsDrawn(), false, 96, core.Image{Source: star, TestID: "svg-star-plain", Style: core.Style{Width: core.Pt(36), Height: core.Pt(36)}}))
	cell := func(child core.Node, n string, isCode bool) core.Node {
		return core.View{Style: core.Style{FlexGrow: 1, FlexBasis: core.Pt(0), Gap: 8}, Children: []core.Node{child, imageNote(cx, n, isCode)}}
	}
	return ui.Card{Title: tr.ImagesSvgImages(), Description: tr.ImagesStarSvgMarkSvgAnd(), Children: []core.Node{
		core.View{Style: core.Style{Direction: core.Row, Gap: 12}, Children: sizes},
		core.View{Style: core.Style{Direction: core.Row, Gap: 12}, Children: []core.Node{
			cell(core.Image{Source: mark, ResizeMode: core.Contain, TestID: "svg-mark-contain", Style: imageFrame(cx, core.Style{Height: core.Pt(96)}), StyleFunc: frameInk(cx)}, "Contain", true),
			cell(core.Image{Source: mark, ResizeMode: core.Cover, TestID: "svg-mark-cover", Style: imageFrame(cx, core.Style{Height: core.Pt(96)}), StyleFunc: frameInk(cx)}, "Cover", true),
			cell(core.Image{Source: core.Src(remoteSVG), ResizeMode: core.Contain, TestID: "svg-remote", Style: imageFrame(cx, core.Style{Height: core.Pt(96)}), StyleFunc: frameInk(cx)}, tr.ImagesRemote(), false),
		}},
	}}
}

func animatedImages(cx *core.Scope) core.Node {
	tr := locales.For(cx)
	gif := core.ImageSource{Data: bounceGIF, Size: core.Size{Width: 72, Height: 72}}
	webp := core.ImageSource{Data: spinnerWebP, Size: core.Size{Width: 72, Height: 72}}
	return ui.Card{Title: tr.ImagesAnimatedGifAndWebp(), Description: tr.ImagesDecodedAndPlayedNatively(), Children: []core.Node{
		core.View{Style: core.Style{Direction: core.Row, Gap: 12}, Children: []core.Node{
			imageTile(cx, "bounce.gif", true, 96, core.Image{Source: gif, TestID: "gif"}),
			imageTile(cx, "spinner.webp", true, 96, core.Image{Source: webp, TestID: "webp"}),
			imageTile(cx, tr.ImagesScaledRound(), false, 96, core.Image{Source: gif, Style: core.Style{Width: core.Pt(48), Height: core.Pt(48), CornerRadius: 24}}),
		}},
	}}
}

// placeholders are network images behind a placeholder, fading in;
// Reload asks for images that are not cached yet.
func placeholders(cx *core.Scope) core.Node {
	tr := locales.For(cx)
	seed := core.NewSignal(cx, 1)
	fades := []struct{ id, ms int }{{1043, 0}, {1080, 300}, {1069, 1000}}
	var cells []core.Node
	for _, f := range fades {
		cells = append(cells, core.View{Style: core.Style{FlexGrow: 1, FlexBasis: core.Pt(0), Gap: 8}, Children: []core.Node{
			// A new size per reload: a URL not in the cache yet. A
			// FadeDuration of 0 is the platform's; a nanosecond is none.
			core.Image{SourceFunc: func() core.ImageSource { return core.Src(photoURL(f.id, 600+seed.Get(), 600+seed.Get())) },
				Placeholder: logoSource(), FadeDuration: ms(f.ms) + 1, ResizeMode: core.Cover, Style: imageFrame(cx, core.Style{Height: core.Pt(100)}), StyleFunc: frameInk(cx)},
			imageNote(cx, fmt.Sprintf("%d ms", f.ms), false),
		}})
	}
	return ui.Card{Title: tr.ImagesPlaceholderAndFade(), Description: tr.ImagesAPlaceholderThenAFade(), Actions: []core.Node{ui.Button{Text: tr.ImagesReload(), Variant: ui.ButtonSecondary, Small: true, OnPress: func() { seed.Update(func(n int) int { return n + 1 }) }}}, Children: []core.Node{
		core.View{Style: core.Style{Direction: core.Row, Gap: 12}, Children: cells},
	}, Footer: []core.Node{
		ui.Caption{Label: tr.ImagesLoads(), Note: func() string { return itoa(seed.Get()) }},
	}}
}

func blurCard(cx *core.Scope) core.Node {
	tr := locales.For(cx)
	var cells []core.Node
	for _, r := range []core.Points{0, 2, 6, 16} {
		n := tr.ImagesNone()
		if r > 0 {
			n = fmt.Sprintf("%.0f pt", r)
		}
		cells = append(cells, core.View{Style: core.Style{FlexGrow: 1, FlexBasis: core.Pt(0), Gap: 8}, Children: []core.Node{
			core.Image{Source: core.Src(widePhoto), BlurRadius: r, Style: imageFrame(cx, core.Style{Height: core.Pt(76)}), StyleFunc: frameInk(cx)}, imageNote(cx, n, false),
		}})
	}
	return ui.Card{Title: tr.ImagesBlurRadius(), Description: tr.ImagesBlurradiusAppliedWhileDecoding(), Children: []core.Node{
		core.View{Style: core.Style{Direction: core.Row, Gap: 8}, Children: cells},
	}}
}

func imageInfo(cx *core.Scope) core.Node {
	tr := locales.For(cx)
	big := photoURL(1018, 2400, 1600)
	size, prefetched := core.NewSignal(cx, "—"), core.NewSignal(cx, "—")
	progress := core.NewSignal(cx, 0.0)
	status := core.NewSignal(cx, tr.ImagesNotShown())
	show := core.NewSignal(cx, false)
	return ui.Card{Title: tr.ImagesSizePrefetchProgress(), Description: "ImageSize · PrefetchImage · OnProgress", Children: []core.Node{
		core.View{Style: core.Style{Direction: core.Row, Gap: 8}, Children: []core.Node{
			ui.Cell{Child: ui.Button{Text: tr.ImagesSize(), Variant: ui.ButtonSecondary, OnPress: func() {
				cx.ImageSize(core.Src(big), func(px core.Size, err error) {
					if err != nil {
						size.Set(err.Error())
						return
					}
					size.Set(fmt.Sprintf("%.0f×%.0f px", px.Width, px.Height))
				})
			}}},
			ui.Cell{Child: ui.Button{Text: tr.ImagesPrefetch(), Variant: ui.ButtonSecondary, OnPress: func() {
				prefetched.Set("loading…")
				cx.PrefetchImage(core.Src(big), func(ok bool) {
					if ok {
						prefetched.Set(tr.ImagesInTheCache())
					} else {
						prefetched.Set("failed")
					}
				})
			}}},
			ui.Cell{Child: core.Dynamic(func(*core.Scope) core.Node {
				if show.Get() {
					return ui.Button{Text: tr.ImagesHide(), Variant: ui.ButtonSecondary, OnPress: func() { show.Set(false) }}
				}
				return ui.Button{Text: tr.ImagesShow(), Variant: ui.ButtonAccent, OnPress: func() { show.Set(true) }}
			})},
		}},
		core.View{Style: core.Style{Gap: 8}, Children: []core.Node{
			ui.Caption{Label: "ImageSize", Note: size.Get},
			ui.Caption{Label: "PrefetchImage", Note: prefetched.Get},
			ui.Caption{Label: "OnProgress", Note: func() string { return fmt.Sprintf("%.0f%% · %s", progress.Get()*100, status.Get()) }},
			core.Dynamic(func(*core.Scope) core.Node {
				t := ui.ToneAccent
				if progress.Get() >= 1 {
					t = ui.ToneUp
				}
				return ui.Meter{Value: progress.Get, Tone: t}
			}),
		}},
		core.Show{When: show.Get, Else: func(*core.Scope) core.Node {
			return imageNote(cx, tr.ImagesPrefetchThenShowNoDownload(), false)
		},
			Then: func(*core.Scope) core.Node {
				return ui.Appear{Children: []core.Node{core.Image{Source: core.Src(big), Style: imageFrame(cx, core.Style{Height: core.Pt(170)}), StyleFunc: frameInk(cx),
					OnLoadStart: func() { status.Set("loading") },
					OnProgress: func(loaded, total int64) {
						progress.Set(float64(loaded) / float64(max(1, total)))
					},
					OnLoad: func(core.Size) {
						if progress.GetUntracked() == 0 {
							status.Set(tr.ImagesLoadedNoDownload())
						} else {
							status.Set("loaded")
						}
						progress.Set(1)
					}}}}
			}},
	}}
}

func imageEvents(cx *core.Scope) core.Node {
	tr := locales.For(cx)
	colors := ui.ColorsOf(cx)
	events := core.NewSignalFunc(cx, []string(nil), slices.Equal)
	log := func(l string) { events.Update(func(e []string) []string { return append(slices.Clone(e), l) }) }
	loaded, failed := core.NewSignal(cx, "…"), core.NewSignal(cx, tr.ImagesLoading())
	thumb := imageFrame(cx, core.Style{Width: core.Pt(96), Height: core.Pt(64)})
	return ui.Card{Title: tr.ImagesEvents(), Description: tr.ImagesLoadStartLoadErrorLoad(), Children: []core.Node{
		core.View{Style: core.Style{Direction: core.Row, Gap: 12, AlignItems: core.AlignCenter}, Children: []core.Node{
			core.Image{Source: core.Src(photoURL(1084, 600, 400)), Style: thumb, StyleFunc: frameInk(cx),
				OnLoadStart: func() { log("OnLoadStart") },
				OnLoad:      func(px core.Size) { log("OnLoad"); loaded.Set(fmt.Sprintf("%.0f×%.0f", px.Width, px.Height)) },
				OnError:     func(err error) { loaded.Set(err.Error()) },
				OnLoadEnd:   func() { log("OnLoadEnd") }},
			core.View{Style: core.Style{FlexGrow: 1, FlexShrink: 1, Gap: 4}, Children: []core.Node{
				ui.Text("picsum.photos", 15, ui.Regular, colors.Text),
				ui.Textf(func() string { return tr.ImagesLoaded() + loaded.Get() + tr.ImagesPx() }, 13, ui.Regular, colors.Up),
			}},
		}},
		core.View{StyleFunc: func() core.Style {
			return core.Style{Direction: core.Row, Gap: 12, AlignItems: core.AlignCenter, Borders: core.Edges{Top: 1}, BorderColors: core.EdgeColors{Top: colors.Line()}, Padding: core.Edges{Top: 12}}
		}, Children: []core.Node{
			core.View{StyleFunc: func() core.Style {
				return thumb.Merge(frameInk(cx)()).Merge(core.Style{AlignItems: core.AlignCenter, JustifyContent: core.JustifyCenter, BorderColor: ui.Alpha(colors.Danger(), 0.4)})
			}, Children: []core.Node{
				core.Image{Source: core.Src("https://example.invalid/missing.png"), OnError: func(err error) { failed.Set(err.Error()) },
					Style:     thumb.Merge(core.Style{Position: core.Absolute, Left: core.Pt(-1), Top: core.Pt(-1), BorderWidth: 0, Background: transparent}),
					StyleFunc: func() core.Style { return core.Style{BorderColor: colors.Border()} }},
				ui.Text("×", 20, ui.Regular, colors.Danger),
			}},
			core.View{Style: core.Style{FlexGrow: 1, FlexShrink: 1, Gap: 4}, Children: []core.Node{
				ui.Text("example.invalid", 15, ui.Regular, colors.Text),
				core.Text{ContentFunc: failed.Get, MaxLines: 3, Style: core.Style{FontSize: 13}, StyleFunc: ui.Colored(colors.Danger)},
			}},
		}},
		core.View{Style: core.Style{Direction: core.Row, Wrap: core.Wrap, Gap: 8, AlignItems: core.AlignCenter}, Children: []core.Node{core.Dynamic(func(*core.Scope) core.Node {
			e := events.Get()
			if len(e) == 0 {
				return imageNote(cx, tr.ImagesNoLoadEventsYet(), false)
			}
			var badges []core.Node
			for _, l := range e {
				t := ui.ToneNeutral
				if l == "OnLoad" {
					t = ui.ToneUp
				}
				badges = append(badges, ui.Badge{Text: l, Tone: t})
			}
			return core.Group(badges)
		})}},
	}}
}

// imagesDemo is core.Image: resize modes, corners, embedded and tinted
// images, SVG, animated GIF and WebP, placeholders and fades, blur, sizes,
// prefetching, progress and the load events.
func imagesDemo(cx *core.Scope) core.Node {
	return demoPage(cx, pageProps{columns: 3}, resizeModes, imageCorners, assetsCard, svgCard, animatedImages, placeholders, blurCard, imageInfo, imageEvents)
}
