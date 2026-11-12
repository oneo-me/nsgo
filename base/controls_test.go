package base_test

import (
	"testing"
	"time"

	"github.com/nsgo-dev/nsgo/base"
	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/nsgotest"
)

func TestControlsReportWhatTheyAskForAndStyleTheirParts(t *testing.T) {
	var asked []bool
	app := nsgotest.Start(t, func(cx *core.Scope) core.Node {
		checked := core.NewSignal(cx, false)
		mixed := core.NewSignal(cx, true)
		on := core.NewSignal(cx, false)
		plan := core.NewSignal(cx, "a")
		radio := func(name string) core.Node {
			return base.Radio{Label: name, TestID: "radio-" + name, Style: core.Style{Height: core.Pt(20)}, Checked: func() bool { return plan.Get() == name },
				OnChange: func(bool) { plan.Set(name) }}
		}
		return core.View{Children: []core.Node{
			base.Checkbox{Label: "All", TestID: "check", Checked: checked.Get, Indeterminate: mixed.Get,
				OnChange: func(v bool) { asked = append(asked, v); checked.Set(v); mixed.Set(false) },
				IndicatorStyle: core.Style{Width: core.Pt(10), Height: core.Pt(10), Checked: &core.Style{Width: core.Pt(12)},
					Indeterminate: &core.Style{Width: core.Pt(14)}}},
			base.Switch{Label: "Wi-Fi", TestID: "switch", Checked: on.Get, OnChange: on.Set,
				TrackStyle: core.Style{Width: core.Pt(30), Height: core.Pt(10), Checked: &core.Style{Width: core.Pt(40)}}},
			base.RadioGroup{TestID: "plans", Children: []core.Node{radio("a"), radio("b")}},
		}}
	})
	indicator := func() core.Points { return app.Find("id:check").Children[0].Frame.Size.Width }
	if got := indicator(); got != 14 {
		t.Fatalf("indeterminate indicator is %v wide, want its Indeterminate style", got)
	}
	if p := app.Find("id:check").Children[0].Props; p.Checked || p.Indeterminate {
		t.Errorf("the indicator reports %+v, want its states for its style only", p)
	}
	app.Press("id:check")
	if got := indicator(); got != 12 {
		t.Errorf("checked indicator is %v wide", got)
	}
	app.Press("id:check")
	if len(asked) != 2 || !asked[0] || asked[1] {
		t.Errorf("asked %v, want checked from indeterminate, then unchecked", asked)
	}
	if app.Find("id:check").Props.Checked {
		t.Error("the checkbox still reports checked")
	}

	app.Press("id:switch")
	if v := app.Find("id:switch"); !v.Props.Checked || v.Children[0].Frame.Size.Width != 40 {
		t.Errorf("switch: checked %v, track %v wide", v.Props.Checked, v.Children[0].Frame.Size.Width)
	}

	if v := app.Find("id:plans"); v.Props.Role != core.RoleRadioGroup {
		t.Errorf("group role = %v", v.Props.Role)
	}
	app.Press("id:radio-b")
	a, b := app.Find("id:radio-a").Props, app.Find("id:radio-b").Props
	if a.Checked || !b.Checked || !b.Selected || b.Role != core.RoleRadio {
		t.Errorf("radios: a %+v, b %+v", a, b)
	}
}

func TestButtonTabsCollapsibleAndProgress(t *testing.T) {
	var log []string
	app := nsgotest.Start(t, func(cx *core.Scope) core.Node {
		open := core.NewSignal(cx, false)
		tab := core.NewSignal(cx, 0)
		value := core.NewSignal[float32](cx, 150)
		return core.View{Style: core.Style{Width: core.Pt(200)}, Children: []core.Node{
			base.Button{Label: "Toggle", Style: core.Style{Height: core.Pt(20)}, Role: core.RoleLink, Selected: open.Get, Expanded: open.Get,
				OnPressDown: func(core.PressEvent) { log = append(log, "down") },
				OnPressUp:   func(core.PressEvent) { log = append(log, "up") },
				OnPress:     func(core.PressEvent) { open.Update(func(o bool) bool { return !o }) }},
			base.Collapsible{Open: open.Get, TestID: "more",
				Content: base.Input{TestID: "kept"}},
			base.Tabs{TestID: "tabs", Children: []core.Node{
				base.Tab{Label: "One", Style: core.Style{Height: core.Pt(20)}, Selected: func() bool { return tab.Get() == 0 }, OnPress: func(core.PressEvent) { tab.Set(0) }},
				base.Tab{Label: "Two", Style: core.Style{Height: core.Pt(20)}, Selected: func() bool { return tab.Get() == 1 }, OnPress: func(core.PressEvent) { tab.Set(1) }},
			}},
			base.Progress{Label: "Upload", TestID: "progress", Value: value.Get,
				TrackStyle: core.Style{Width: core.Pt(100)}, IndicatorStyle: core.Style{Height: core.Pt(4)}},
		}}
	})
	if app.Exists("id:kept") {
		t.Fatal("closed content is shown")
	}
	app.PressDown("label:Toggle")
	app.PressUp("label:Toggle")
	app.Press("label:Toggle")
	if b := app.Find("label:Toggle").Props; b.Role != core.RoleLink || !b.Selected || !b.Expanded {
		t.Errorf("button props %+v", b)
	}
	if len(log) < 2 || log[0] != "down" || log[1] != "up" {
		t.Errorf("log = %v", log)
	}
	app.Type("id:kept", "hello")
	app.Press("label:Toggle")
	if app.Exists("id:kept") {
		t.Fatal("content still shown after closing")
	}
	app.Press("label:Toggle")
	if got := app.Find("id:kept").Text; got != "hello" {
		t.Errorf("reopened content has %q, want what was typed kept", got)
	}

	app.Press("label:Two")
	if one, two := app.Find("label:One").Props, app.Find("label:Two").Props; one.Selected || !two.Selected || two.Role != core.RoleTab {
		t.Errorf("tabs: %+v %+v", one, two)
	}
	if app.Find("id:tabs").Props.Role != core.RoleTabList {
		t.Error("tabs have no tab list role")
	}

	p := app.Find("id:progress")
	if v := p.Props.Value; v.Max != 100 || v.Now != 100 || p.Props.Role != core.RoleProgressBar {
		t.Errorf("progress value %+v role %v, want clamped to 100", v, p.Props.Role)
	}
	if w := p.Children[0].Children[0].Frame.Size.Width; w != 100 {
		t.Errorf("indicator %v wide, want the whole track", w)
	}
}

// TestCarouselLoopingMargin moves a looping carousel whose slides leave
// their spacing in a margin, so no gap sits between the runway and them.
func TestCarouselLoopingMargin(t *testing.T) {
	app := startDesktop(t, func(cx *core.Scope) core.Node {
		var slides []core.Node
		for range 5 {
			slides = append(slides, base.CarouselItem{Style: core.Style{Width: core.Pt(100), Margin: core.Edges{Right: 8}}})
		}
		return core.View{Style: core.Style{Width: core.Pt(200)}, Children: []core.Node{
			base.Carousel{Looping: true, Children: []core.Node{
				base.CarouselContent{TestID: "track", Style: core.Style{Height: core.Pt(50)}, Children: slides},
				base.CarouselNext{Label: "Next", Style: core.Style{Height: core.Pt(30)}},
			}},
		}}
	})
	rest := func(i int) core.Points { return core.Points(532 + 108*i) }
	if got := app.Find("track").Offset.X; got != rest(0) {
		t.Fatalf("rests at %v, want right after the runway", got)
	}
	for i := 1; i <= 5; i++ {
		app.Press("role:button Next")
		app.Advance(time.Second)
		if got, want := app.Find("track").Offset.X, rest(i%5); got != want {
			t.Fatalf("Next %d reached %v, want %v", i, got, want)
		}
	}
}

// TestCollapsibleRevealsMeasured opens a revealing collapsible to its
// content's height once the content is in, and takes the content out once
// it has closed.
func TestCollapsibleRevealsMeasured(t *testing.T) {
	var open *core.Signal[bool]
	app := nsgotest.Start(t, func(cx *core.Scope) core.Node {
		open = core.NewSignal(cx, false)
		return core.View{Style: core.Style{Width: core.Pt(200)}, Children: []core.Node{
			base.Collapsible{Open: open.Get, Reveal: func() bool { return true }, TestID: "more",
				Content: core.View{TestID: "tall", Style: core.Style{Height: core.Pt(600)}}},
		}}
	})
	if app.Exists("id:tall") {
		t.Fatal("closed content is in the tree")
	}
	open.Set(true)
	app.Settle()
	app.FastForward(time.Second)
	if got := app.Find("id:more").Frame.Size.Height; got != 600 {
		t.Fatalf("opened to %v high, want the content's 600", got)
	}
	open.Set(false)
	app.Settle()
	if got := app.FindMounted("id:more").Frame.Size.Height; got != 0 || len(app.QueryMounted("id:tall")) != 1 {
		t.Fatalf("closing is %v high: want 0 with the content kept while it springs", got)
	}
	app.FastForward(time.Second)
	if len(app.QueryMounted("id:tall")) != 0 {
		t.Fatal("the content stayed after closing")
	}
}
