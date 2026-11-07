package main

import (
	"slices"
	"testing"

	"github.com/oneo-me/nsgo/base"
	"github.com/oneo-me/nsgo/core"
	"github.com/oneo-me/nsgo/nsgotest"
	"github.com/oneo-me/nsgo/ui"
)

// TestControlsOnThePlatform drives base's and ui's newer controls with the
// platform's own input under nsgo test -target: a slider, a toolbar's
// roving focus, a one-time code and a bar of the app's that scrolls.
func TestControlsOnThePlatform(t *testing.T) {
	var value float64
	var bold *core.Signal[bool]
	var code string
	scroller := new(core.Ref)
	var offset core.Points
	var open *core.Signal[bool]
	var rating *core.Signal[int]
	var tags *core.Signal[[]string]
	var selection *ui.Selection
	app := nsgotest.Start(t, func(cx *core.Scope) core.Node {
		cx.SetTheme(ui.DefaultTheme())
		bold = core.NewSignal(cx, false)
		open, rating = core.NewSignal(cx, false), core.NewSignal(cx, 0)
		selection = ui.NewSelection(cx, false)
		tags = core.NewSignalFunc(cx, nil, slices.Equal[[]string])
		return core.View{Style: core.Style{FlexGrow: 1, Padding: core.All(16), Gap: 16}, Children: []core.Node{
			ui.TagInput{Label: "Tags", Tags: tags.Get, OnChange: tags.Set},
			ui.Rating{TestID: "rating", Label: "Score", Value: rating.Get, OnChange: rating.Set},
			ui.Tree{TestID: "tree", Label: "Files", Selection: selection, ToggleLabel: "Expand", Style: core.Style{Height: core.Pt(90)},
				Roots: func() []any { return []any{"src"} },
				Children: func(k any) []any {
					if k == "src" {
						return []any{"main.go"}
					}
					return nil
				},
				Expanded: func(any) bool { return open.Get() }, OnExpand: func(_ any, on bool) { open.Set(on) },
				RowLabel: func(k any) string { return k.(string) }},
			base.Slider{TestID: "slider", Label: "Level", OnChange: func(v base.SliderValue) { value = v.End },
				Style:          core.Style{Width: core.Pt(200)},
				TrackStyle:     core.Style{Height: core.Pt(24), JustifyContent: core.JustifyCenter},
				IndicatorStyle: core.Style{Height: core.Pt(4), Background: core.RGB(0xc8c8c8)},
				ThumbStyle:     core.Style{Width: core.Pt(16), Height: core.Pt(16), Background: core.RGB(0x0000ff)}},
			ui.Toolbar{TestID: "bar", Label: "Format", Children: []core.Node{
				ui.ToggleButton{Text: "B", Label: "Bold", TestID: "bold", On: bold.Get, OnChange: bold.Set},
				ui.ToggleButton{Text: "I", Label: "Italic", TestID: "italic"},
			}},
			ui.OtpInput{Length: 4, Label: "Code", TestID: "code", OnChange: func(v string) { code = v }},
			core.View{Style: core.Style{Width: core.Pt(200), Height: core.Pt(120)}, Children: []core.Node{
				core.Scroll{Ref: scroller, TestID: "pages", HideIndicator: true, Style: core.Style{FlexGrow: 1},
					OnScroll: func(e core.ScrollEvent) { offset = e.Offset.Y },
					Children: []core.Node{core.View{Style: core.Style{Height: core.Pt(1200)}}}},
				base.Scrollbar{Scroll: func() *core.Ref { return scroller }, Label: "Pages", TestID: "bar-pages", Mode: func() base.ScrollbarMode { return base.ScrollbarAlways },
					Style:      core.Style{Position: core.Absolute, Top: core.Pt(0), Bottom: core.Pt(0), Right: core.Pt(0), Width: core.Pt(12)},
					ThumbStyle: core.Style{Background: core.RGB(0x0000ff)}},
			}},
		}}
	}, nsgotest.Size(420, 720))
	app.Type("label:Tags", "go,")
	if !slices.Equal(tags.Get(), []string{"go"}) {
		t.Errorf("tags = %v", tags.Get())
	}
	app.Press("rating-3")
	if rating.Get() != 3 {
		t.Errorf("a press on the third star gave %d", rating.Get())
	}
	app.Press("label:src >> role:button Expand")
	app.Find("label:main.go")
	app.Press("label:main.go")
	if selection.Cursor() != "main.go" {
		t.Errorf("a press on a row selected %v", selection.Cursor())
	}
	app.Press("bold")
	if !bold.Get() || !app.Find("bold").Props.Pressed {
		t.Error("a press does not turn the toolbar's toggle on")
	}
	app.Type("label:Code", "12a3")
	if code != "123" {
		t.Errorf("code = %q, want the digits", code)
	}
	app.Drag("label:Level", core.Point{X: 60})
	if value < 20 {
		t.Errorf("a drag of the thumb gave %v", value)
	}
	app.Press("bar-pages")
	app.Settle()
	if offset == 0 {
		t.Error("a press on the bar's track did not scroll its target")
	}
}
