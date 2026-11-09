package main

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"math"
	"time"

	"github.com/oneo-me/nsgo/core"
	"github.com/oneo-me/nsgo/icons/lucide"
	"github.com/oneo-me/nsgo/ui"
	"github.com/oneo-me/nsgo/widget"

	"github.com/oneo-me/nsgo/examples/gallery/locales"
)

// The gallery's widgets, as nsgo.yaml lists them: Activity comes in every
// family, each showing what package widget draws there, and Photo in the
// Home Screen's and desktop's, over a picture the gallery draws.
const (
	activityKind = "activity"
	photoKind    = "photo"
	// docsURL is what a tap on the medium Activity widget's link opens.
	docsURL = "https://oneo-me.github.io/nsgo/docs/building/#widgets"
)

// widgetState is what the gallery's widgets show: today's steps toward a
// goal, in an accent, with a timeline that walks on by itself.
type widgetState struct {
	steps, goal int
	accent      int
	timeline    bool
}

// widgetAccents are the accents the widgets take, each a shade lighter in
// dark mode.
var widgetAccents = []struct{ light, dark uint32 }{
	{0x007aff, 0x0a84ff}, // blue
	{0x28a745, 0x30d158}, // green
	{0xf08c00, 0xff9f0a}, // orange
	{0xe8365d, 0xff375f}, // pink
}

func (s widgetState) color() widget.Color {
	a := widgetAccents[s.accent%len(widgetAccents)]
	return widget.RGB(a.light).Dark(widget.RGB(a.dark))
}

// rings are the three rings of activity, from 0 to 1: steps toward the
// goal, exercise toward 30 minutes and standing toward 12 hours, both
// counted from the steps.
func (s widgetState) rings() (move, exercise, stand float64) {
	return float64(s.steps) / float64(max(s.goal, 1)), float64(s.minutes()) / 30, float64(s.hours()) / 12
}

func (s widgetState) minutes() int { return s.steps / 250 }
func (s widgetState) hours() int   { return min(s.steps/800, 12) }

// widgetUpdates writes the widgets' timelines on a goroutine of their own,
// in order, as encoding the photo and writing files would hold the engine.
// report hears each update's error, nil once it is written, on the engine.
type widgetUpdates struct {
	queue chan func() error
}

func newWidgetUpdates(cx *core.Scope, report func(error)) *widgetUpdates {
	u := &widgetUpdates{queue: make(chan func() error, 16)}
	go func() {
		for update := range u.queue {
			err := update()
			cx.Post(func() { report(err) })
		}
	}()
	cx.Cleanup(func() { close(u.queue) })
	return u
}

// show writes both kinds' timelines for s, or their placeholders when s is
// nil.
func (u *widgetUpdates) show(tr locales.Messages, s *widgetState, now time.Time) {
	if s == nil {
		u.queue <- func() error {
			if err := widget.Update(activityKind); err != nil {
				return err
			}
			return widget.Update(photoKind)
		}
		return
	}
	activity := activityTimeline(tr, *s, now)
	photo := photoEntry(tr, *s, now)
	u.queue <- func() error {
		if err := widget.Update(activityKind, activity...); err != nil {
			return err
		}
		photo.Background = widget.Image{Data: widgetPicture(s.accent), Cover: true}
		return widget.Update(photoKind, photo)
	}
}

// activityTimeline is the Activity widgets' timeline: now, and with
// timeline the next five minutes, a few hundred steps each, which the
// system shows as their minutes come without the app.
func activityTimeline(tr locales.Messages, s widgetState, now time.Time) []widget.Entry {
	entries := []widget.Entry{activityEntry(tr, s, now, now)}
	if s.timeline {
		next := s
		for i := 1; i <= 5; i++ {
			next.steps += 320
			at := now.Truncate(time.Minute).Add(time.Duration(i) * time.Minute)
			entries = append(entries, activityEntry(tr, next, at, now))
		}
	}
	return entries
}

// activityEntry is what each family of Activity shows of s from date on;
// written is when the app wrote it.
func activityEntry(tr locales.Messages, s widgetState, date, written time.Time) widget.Entry {
	accent := s.color()
	move, exercise, stand := s.rings()
	label := func(text string) widget.Text {
		return widget.Text{Content: text, Style: widget.Style{TextStyle: widget.Caption, Color: widget.Secondary}}
	}
	ring := func(value float64, size core.Points) widget.View {
		return widget.Gauge{Value: value, Style: widget.Style{Width: size, Height: size, Color: accent, Accented: true}}
	}
	steps := widget.Text{Content: tr.WidgetsSteps(s.steps), Style: widget.Style{FontVariant: core.FontTabularNums}}
	hour := date.Truncate(time.Hour).Add(time.Hour)

	// Small: a ring with the share inside, over the steps.
	small := widget.Column{Style: widget.Style{AlignItems: widget.AlignStart, Gap: 6, FillWidth: true, FillHeight: true}, Children: []widget.View{
		widget.Row{Style: widget.Style{Gap: 4, Color: accent}, Children: []widget.View{
			widget.Symbol{Name: "figure.walk"}, widget.Text{Content: tr.WidgetsActivity(), Style: widget.Style{TextStyle: widget.Headline}},
		}},
		widget.Spacer{},
		widget.Stack{Children: []widget.View{
			ring(move, 64),
			widget.Text{Content: tr.WidgetsPercent(int(math.Round(move * 100))), Style: widget.Style{FontSize: 15, FontWeight: core.FontWeightBold, FontDesign: widget.Rounded, MinScale: 0.6}},
		}},
		widget.Spacer{},
		widget.Text{Content: tr.WidgetsSteps(s.steps), Style: widget.Style{TextStyle: widget.Footnote, FontWeight: core.FontWeightSemibold, FontVariant: core.FontTabularNums, MaxLines: 1, MinScale: 0.7}},
	}}

	// Medium: the three rings as bars, a countdown and a link.
	bar := func(name string, value float64, detail string) widget.View {
		return widget.Column{Style: widget.Style{AlignItems: widget.AlignStart, Gap: 3}, Children: []widget.View{
			widget.Row{Style: widget.Style{FillWidth: true}, Children: []widget.View{
				label(name), widget.Spacer{}, widget.Text{Content: detail, Style: widget.Style{TextStyle: widget.Caption, FontVariant: core.FontTabularNums}},
			}},
			widget.Progress{Value: value, Style: widget.Style{Height: 6, Color: accent, Accented: true}},
		}}
	}
	bars := []widget.View{
		bar(tr.WidgetsMove(), move, tr.WidgetsPercent(int(math.Round(move*100)))),
		bar(tr.WidgetsExercise(), exercise, tr.WidgetsMinutes(s.minutes())),
		bar(tr.WidgetsStand(), stand, tr.WidgetsHours(s.hours())),
	}
	medium := widget.Row{Style: widget.Style{Gap: 16, FillWidth: true, FillHeight: true}, Children: []widget.View{
		widget.Column{Style: widget.Style{AlignItems: widget.AlignStart, Gap: 4, FillHeight: true}, Children: []widget.View{
			widget.Symbol{Name: "figure.walk", Style: widget.Style{TextStyle: widget.Title2, Color: accent}},
			widget.Spacer{},
			label(tr.WidgetsNextHourIn()),
			widget.Date{Time: hour, Format: widget.Timer, Style: widget.Style{TextStyle: widget.Title3, FontWeight: core.FontWeightSemibold, FontVariant: core.FontTabularNums}},
			widget.Text{Content: tr.WidgetsDocs(), Style: widget.Style{TextStyle: widget.Caption2, FontWeight: core.FontWeightSemibold, Color: accent, Padding: core.Edges{Top: 3, Right: 8, Bottom: 3, Left: 8},
				Background: widget.RGBA(0x8e8e93, 0.18), CornerRadius: 8, Link: docsURL}},
		}},
		widget.Column{Style: widget.Style{Gap: 8, FillWidth: true}, Children: bars},
	}}

	// Large: a header, a list of the day's parts and when it was written.
	row := func(symbol, title, detail, value string) widget.View {
		return widget.Row{Style: widget.Style{Gap: 10, FillWidth: true}, Children: []widget.View{
			widget.Stack{Style: widget.Style{Width: 32, Height: 32, Background: accent, CornerRadius: 8}, Children: []widget.View{
				widget.Symbol{Name: symbol, Style: widget.Style{FontSize: 15, Color: widget.RGB(0xffffff)}},
			}},
			widget.Column{Style: widget.Style{AlignItems: widget.AlignStart, Gap: 1}, Children: []widget.View{
				widget.Text{Content: title, Style: widget.Style{TextStyle: widget.Subheadline, FontWeight: core.FontWeightSemibold}},
				widget.Text{Content: detail, Style: widget.Style{TextStyle: widget.Caption, Color: widget.Secondary, MaxLines: 1}},
			}},
			widget.Spacer{},
			widget.Text{Content: value, Style: widget.Style{TextStyle: widget.Body, FontDesign: widget.Rounded, FontWeight: core.FontWeightSemibold, FontVariant: core.FontTabularNums}},
		}}
	}
	list := widget.Column{Style: widget.Style{Gap: 10, FillWidth: true}, Children: []widget.View{
		row("flame.fill", tr.WidgetsMove(), tr.WidgetsOfGoal(s.goal), tr.WidgetsPercent(int(math.Round(move*100)))),
		widget.Divider{},
		row("figure.run", tr.WidgetsExercise(), tr.WidgetsExerciseDetail(), tr.WidgetsMinutes(s.minutes())),
		widget.Divider{},
		row("figure.stand", tr.WidgetsStand(), tr.WidgetsStandDetail(), tr.WidgetsHours(s.hours())),
	}}
	header := widget.Row{Style: widget.Style{Gap: 8, FillWidth: true}, Children: []widget.View{
		ring(move, 36),
		widget.Column{Style: widget.Style{AlignItems: widget.AlignStart}, Children: []widget.View{
			widget.Text{Content: tr.WidgetsActivity(), Style: widget.Style{TextStyle: widget.Headline}},
			widget.Date{Time: date, Format: widget.Day, Style: widget.Style{TextStyle: widget.Caption, Color: widget.Secondary}},
		}},
		widget.Spacer{},
		widget.Text{Content: tr.WidgetsSteps(s.steps), Style: widget.Style{TextStyle: widget.Title3, FontWeight: core.FontWeightBold, FontVariant: core.FontTabularNums, Color: accent}},
	}}
	updated := widget.Row{Style: widget.Style{Gap: 4, Color: widget.Secondary, TextStyle: widget.Caption2}, Children: []widget.View{
		widget.Symbol{Name: "clock.arrow.circlepath"}, widget.Text{Content: tr.WidgetsWritten()}, widget.Date{Time: written, Format: widget.Relative},
	}}
	large := widget.Column{Style: widget.Style{Gap: 12, FillWidth: true, FillHeight: true, Justify: widget.AlignStart}, Children: []widget.View{
		header, widget.Progress{Value: move, Style: widget.Style{Height: 8, Color: accent}}, list, widget.Spacer{}, updated,
	}}

	// Extra large: the large widget beside tiles of the system's fonts
	// and colors.
	tile := func(caption string, content widget.View) widget.View {
		return widget.Column{Style: widget.Style{AlignItems: widget.AlignStart, Gap: 4, Padding: core.All(10), FillWidth: true, FillHeight: true,
			Background: widget.RGBA(0x8e8e93, 0.12), CornerRadius: 12}, Children: []widget.View{label(caption), content}}
	}
	quote := tr.WidgetsQuote()
	tiles := widget.Column{Style: widget.Style{Gap: 8, FillWidth: true, FillHeight: true}, Children: []widget.View{
		widget.Row{Style: widget.Style{Gap: 8}, Children: []widget.View{
			tile(tr.WidgetsRounded(), widget.Text{Content: tr.WidgetsSteps(s.steps), Style: widget.Style{TextStyle: widget.Title2, FontDesign: widget.Rounded, FontWeight: core.FontWeightBold, MinScale: 0.5, MaxLines: 1}}),
			tile(tr.WidgetsMonospaced(), widget.Date{Time: hour, Format: widget.Timer, Style: widget.Style{TextStyle: widget.Title2, FontDesign: widget.Monospaced}}),
		}},
		tile(tr.WidgetsSerif(), widget.Text{Content: quote, Style: widget.Style{TextStyle: widget.Body, FontDesign: widget.Serif, Italic: true, MaxLines: 2}}),
		widget.Row{Style: widget.Style{Gap: 8}, Children: []widget.View{
			// Small capitals change Latin letters alone.
			tile(tr.WidgetsSmallCaps(), widget.Text{Content: "NSGo Gallery", Style: widget.Style{TextStyle: widget.Headline, FontVariant: core.FontSmallCaps}}),
			tile(tr.WidgetsColors(), widget.Row{Style: widget.Style{Gap: 6}, Children: []widget.View{
				widget.Rectangle{Fill: accent, Style: widget.Style{Width: 20, Height: 20, CornerRadius: 10}},
				widget.Rectangle{Fill: widget.Primary, Style: widget.Style{Width: 20, Height: 20, CornerRadius: 10}},
				widget.Rectangle{Fill: widget.Secondary, Style: widget.Style{Width: 20, Height: 20, CornerRadius: 10}},
				widget.Rectangle{Fill: widget.Accent, Style: widget.Style{Width: 20, Height: 20, CornerRadius: 10}},
			}}),
		}},
	}}
	extraLarge := widget.Row{Style: widget.Style{Gap: 16, FillWidth: true, FillHeight: true}, Children: []widget.View{
		widget.Column{Style: widget.Style{FillWidth: true, FillHeight: true}, Children: []widget.View{large}},
		tiles,
	}}

	return widget.Entry{
		Date: date,
		View: small,
		Families: map[widget.Family]widget.View{
			widget.Medium:     medium,
			widget.Large:      large,
			widget.ExtraLarge: extraLarge,
			// The Lock Screen's are drawn in one color, with what is
			// accented in the system's tint.
			widget.Circular: widget.Stack{Children: []widget.View{
				widget.Gauge{Value: move, Style: widget.Style{FillWidth: true, FillHeight: true, Accented: true}},
				widget.Column{Children: []widget.View{
					widget.Symbol{Name: "figure.walk", Style: widget.Style{FontSize: 12}},
					widget.Text{Content: tr.WidgetsPercent(int(math.Round(move * 100))), Style: widget.Style{FontSize: 12, FontWeight: core.FontWeightBold, MinScale: 0.6}},
				}},
			}},
			widget.Rectangular: widget.Column{Style: widget.Style{AlignItems: widget.AlignStart, Gap: 2, FillWidth: true}, Children: []widget.View{
				widget.Row{Style: widget.Style{Gap: 4}, Children: []widget.View{
					widget.Symbol{Name: "figure.walk"}, widget.Text{Content: tr.WidgetsActivity(), Style: widget.Style{TextStyle: widget.Headline, Accented: true}},
				}},
				steps,
				widget.Progress{Value: move, Style: widget.Style{Height: 5, Accented: true}},
			}},
			widget.Inline: widget.Row{Style: widget.Style{Gap: 4}, Children: []widget.View{
				widget.Symbol{Name: "figure.walk"}, steps,
			}},
		},
	}
}

// photoEntry is what the Photo widgets show: the date over a picture, which
// show sets as the background since it takes a while to draw.
func photoEntry(tr locales.Messages, s widgetState, now time.Time) widget.Entry {
	white := widget.RGB(0xffffff)
	caption := widget.Column{Style: widget.Style{AlignItems: widget.AlignStart, Gap: 2, Color: white}, Children: []widget.View{
		widget.Date{Time: now, Format: widget.Day, Style: widget.Style{TextStyle: widget.Caption, FontWeight: core.FontWeightSemibold, Color: widget.RGBA(0xffffff, 0.8)}},
		widget.Text{Content: tr.WidgetsPhotoTitle(), Style: widget.Style{TextStyle: widget.Title3, FontWeight: core.FontWeightBold, MaxLines: 2}},
	}}
	wide := widget.Row{Style: widget.Style{FillWidth: true, Color: white}, Children: []widget.View{
		caption, widget.Spacer{},
		widget.Text{Content: tr.WidgetsSteps(s.steps), Style: widget.Style{TextStyle: widget.Headline, FontVariant: core.FontTabularNums,
			Padding: core.Edges{Top: 4, Right: 10, Bottom: 4, Left: 10}, Background: widget.RGBA(0x000000, 0.25), CornerRadius: 12}},
	}}
	bottom := func(v widget.View) widget.View {
		return widget.Column{Style: widget.Style{AlignItems: widget.AlignStart, FillWidth: true, FillHeight: true}, Children: []widget.View{widget.Spacer{}, v}}
	}
	return widget.Entry{
		View: bottom(caption),
		Families: map[widget.Family]widget.View{
			widget.Medium:     bottom(wide),
			widget.Large:      bottom(wide),
			widget.ExtraLarge: bottom(wide),
		},
	}
}

// widgetPicture draws the Photo widgets' picture in an accent: soft light
// over a gradient, 600 pixels square, as widgets show no more than 1024
// across. Each accent's is drawn once.
func widgetPicture(accent int) []byte {
	if p, ok := widgetPictures[accent]; ok {
		return p
	}
	const size = 600
	a := widgetAccents[accent%len(widgetAccents)].dark
	r, g, b := float64(a>>16&0xff), float64(a>>8&0xff), float64(a&0xff)
	img := image.NewRGBA(image.Rect(0, 0, size, size))
	for y := range size {
		for x := range size {
			fx, fy := float64(x)/size, float64(y)/size
			// Darker toward the bottom, where the caption is, with two
			// glows.
			shade := 0.95 - 0.6*fy
			glow := math.Exp(-((fx-0.75)*(fx-0.75)+(fy-0.25)*(fy-0.25))*10)*0.6 +
				math.Exp(-((fx-0.2)*(fx-0.2)+(fy-0.45)*(fy-0.45))*18)*0.35
			mix := func(c float64) uint8 { return uint8(min(255, c*shade+255*glow*0.5)) }
			img.Set(x, y, color.RGBA{mix(r), mix(g), mix(b), 255})
		}
	}
	var out bytes.Buffer
	png.Encode(&out, img)
	widgetPictures[accent] = out.Bytes()
	return out.Bytes()
}

// widgetPictures are the pictures widgetPicture drew, by accent, which only
// the updates' goroutine reads and writes.
var widgetPictures = map[int][]byte{}

// defaultWidgets is what the widgets show until the Widgets demo changes it.
var defaultWidgets = widgetState{steps: 6420, goal: 10000, timeline: true}

// showWidgets writes the widgets' timelines as the gallery starts, so they
// show the day's steps before the Widgets demo opens.
func showWidgets(cx *core.Scope) {
	if !widget.Available() {
		return
	}
	s := defaultWidgets
	newWidgetUpdates(cx, func(error) {}).show(locales.For(cx), &s, time.Now())
}

// widgetsDemo is package widget: what the gallery's widgets show, which
// every change writes as their timelines, and the families they come in.
func widgetsDemo(cx *core.Scope) core.Node {
	tr := locales.For(cx)
	steps := core.NewSignal(cx, float64(defaultWidgets.steps))
	goal := core.NewSignal(cx, float64(defaultWidgets.goal))
	accent := core.NewSignal(cx, defaultWidgets.accent)
	timeline := core.NewSignal(cx, defaultWidgets.timeline)
	status := core.NewSignal(cx, "")
	failed := core.NewSignal(cx, false)
	updates := newWidgetUpdates(cx, func(err error) {
		failed.Set(err != nil)
		if err != nil {
			status.Set(err.Error())
		} else {
			status.Set(tr.WidgetsWrittenAt(time.Now().Format("15:04:05")))
		}
	})
	state := func() widgetState {
		return widgetState{steps: int(steps.Get()), goal: int(goal.Get()), accent: accent.Get(), timeline: timeline.Get()}
	}
	cx.Effect(func() {
		s := state()
		updates.show(tr, &s, time.Now())
	})

	contentCard := func(cx *core.Scope) core.Node {
		tr := locales.For(cx)
		colors := ui.ColorsOf(cx)
		return Card{Title: tr.WidgetsContent(), Description: tr.WidgetsContentDetail(), Children: []core.Node{
			numberKnob(tr.WidgetsStepsKnob(), steps, 0, 20000, 10),
			numberKnob(tr.WidgetsGoalKnob(), goal, 1000, 20000, 500),
			choiceKnob(tr.WidgetsAccentKnob(), accent, options([]int{0, 1, 2, 3}, tr.WidgetsBlue(), tr.WidgetsGreen(), tr.WidgetsOrange(), tr.WidgetsPink())...),
			boolKnob(tr.WidgetsTimelineKnob(), timeline),
			core.View{Style: core.Style{Direction: core.Row, Gap: 8}, Children: []core.Node{
				ui.Button{Text: tr.WidgetsUpdateNow(), TestID: "widgets-update", OnPress: func() { s := state(); updates.show(tr, &s, time.Now()) }},
				ui.Button{Text: tr.WidgetsShowPlaceholders(), Variant: ui.ButtonSecondary, TestID: "widgets-placeholder", OnPress: func() { updates.show(tr, nil, time.Now()) }},
			}},
			core.View{Style: core.Style{Direction: core.Row, AlignItems: core.AlignCenter, Gap: 6}, Children: []core.Node{
				core.Dynamic(func(*core.Scope) core.Node {
					if failed.Get() {
						return ui.Icon(lucide.CircleAlert, 14, colors.Danger)
					}
					return ui.Icon(lucide.Check, 14, colors.Up)
				}),
				core.Text{ContentFunc: status.Get, TestID: "widgets-status", Style: core.Style{FlexGrow: 1, FlexShrink: 1, FontSize: 12}, StyleFunc: ui.Colored(colors.Muted)},
			}},
			ui.Text(tr.WidgetsHowToAdd(), 12, ui.Regular, colors.Faint),
		}}
	}

	familiesCard := func(cx *core.Scope) core.Node {
		tr := locales.For(cx)
		colors := ui.ColorsOf(cx)
		families := []struct {
			kind, family string
			detail       func() string
		}{
			{tr.WidgetsActivity(), "small", tr.WidgetsSmallDetail},
			{tr.WidgetsActivity(), "medium", tr.WidgetsMediumDetail},
			{tr.WidgetsActivity(), "large", tr.WidgetsLargeDetail},
			{tr.WidgetsActivity(), "extra_large", tr.WidgetsExtraLargeDetail},
			{tr.WidgetsActivity(), "circular", tr.WidgetsCircularDetail},
			{tr.WidgetsActivity(), "rectangular", tr.WidgetsRectangularDetail},
			{tr.WidgetsActivity(), "inline", tr.WidgetsInlineDetail},
			{tr.WidgetsPhoto(), "small · medium · large · extra_large", tr.WidgetsPhotoDetail},
		}
		rows := make([]core.Node, len(families))
		for i, f := range families {
			rows[i] = Row{First: i == 0, Children: []core.Node{
				core.View{Style: core.Style{FlexGrow: 1, FlexShrink: 1, Gap: 2}, Children: []core.Node{
					core.View{Style: core.Style{Direction: core.Row, Gap: 6, AlignItems: core.AlignCenter}, Children: []core.Node{
						ui.Text(f.kind, 13, ui.Medium, colors.Text), Code{Text: f.family},
					}},
					ui.Text(f.detail(), 12, ui.Regular, colors.Muted),
				}},
			}}
		}
		return Card{Title: tr.WidgetsFamilies(), Description: tr.WidgetsFamiliesDetail(), Children: rows}
	}

	return demoPage(cx, pageProps{}, contentCard, familiesCard)
}
