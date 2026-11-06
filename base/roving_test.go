package base_test

import (
	"testing"

	"github.com/oneo-me/nsgo/base"
	"github.com/oneo-me/nsgo/core"
	"github.com/oneo-me/nsgo/nsgotest"
)

func focused(app *nsgotest.App, ids ...string) string {
	for _, id := range ids {
		if app.Find("id:" + id).Focused {
			return id
		}
	}
	return ""
}

func TestRadioGroupIsOneStopItsArrowsMoveThrough(t *testing.T) {
	var plan *core.Signal[string]
	var locked *core.Signal[bool]
	app := nsgotest.Start(t, func(cx *core.Scope) core.Node {
		plan = core.NewSignal(cx, "b")
		locked = core.NewSignal(cx, true)
		radio := func(name string, disabled func() bool) core.Node {
			return base.Radio{TestID: name, Label: name, Disabled: disabled, Style: core.Style{Height: core.Pt(20)},
				Checked: func() bool { return plan.Get() == name }, OnChange: func(bool) { plan.Set(name) }}
		}
		return core.View{Children: []core.Node{
			base.RadioGroup{Children: []core.Node{radio("a", nil), radio("b", nil), radio("off", locked.Get), radio("c", nil)}},
			base.Button{TestID: "after", OnPress: func(core.PressEvent) {}, Style: core.Style{Height: core.Pt(20)}},
		}}
	}, nsgotest.Environment(core.Environment{OS: core.OSMacOS, Idiom: core.IdiomDesktop}))
	ids := []string{"a", "b", "off", "c", "after"}

	app.Key("Tab")
	if got := focused(app, ids...); got != "b" {
		t.Fatalf("Tab enters at %q, want the checked radio", got)
	}
	app.Key("down")
	if got := focused(app, ids...); got != "c" || plan.GetUntracked() != "c" {
		t.Fatalf("down reached %q and checked %q, want c past the disabled radio", got, plan.GetUntracked())
	}
	app.Key("right")
	if got := focused(app, ids...); got != "a" || plan.GetUntracked() != "a" {
		t.Fatalf("right reached %q and checked %q, want a around the end", got, plan.GetUntracked())
	}
	app.Key("up")
	if got := focused(app, ids...); got != "c" {
		t.Fatalf("up reached %q, want c around the start", got)
	}
	app.Key("Tab")
	if got := focused(app, ids...); got != "after" {
		t.Fatalf("Tab went to %q, want out of the group", got)
	}
}

func TestTabsTakeHomeEndAndSwapLeftAndRightRightToLeft(t *testing.T) {
	var current *core.Signal[string]
	presses := 0
	app := nsgotest.Start(t, func(cx *core.Scope) core.Node {
		current = core.NewSignal(cx, "a")
		tab := func(name string) core.Node {
			return base.Tab{TestID: name, Label: name, Style: core.Style{Height: core.Pt(20)},
				Selected: func() bool { return current.Get() == name },
				OnPress:  func(core.PressEvent) { presses++; current.Set(name) }}
		}
		return base.Tabs{Children: []core.Node{tab("a"), tab("b"), tab("c")}}
	}, nsgotest.Environment(core.Environment{OS: core.OSMacOS, Idiom: core.IdiomDesktop}))
	ids := []string{"a", "b", "c"}
	app.Key("Tab")
	app.Key("end")
	if got := focused(app, ids...); got != "c" || current.GetUntracked() != "c" || presses != 1 {
		t.Fatalf("end reached %q, selected %q, %d presses", got, current.GetUntracked(), presses)
	}
	app.Key("home")
	if got := focused(app, ids...); got != "a" {
		t.Fatalf("home reached %q", got)
	}
	app.SetEnvironment(core.Environment{OS: core.OSMacOS, Idiom: core.IdiomDesktop, RTL: true})
	app.Key("left")
	if got := focused(app, ids...); got != "b" {
		t.Fatalf("left right to left reached %q, want the next tab", got)
	}
	app.Key("up")
	if got := focused(app, ids...); got != "a" {
		t.Fatalf("up right to left reached %q, want the previous tab", got)
	}
}

func TestTabsIndicatorFollowsTheSelectedTab(t *testing.T) {
	var current *core.Signal[string]
	app := nsgotest.Start(t, func(cx *core.Scope) core.Node {
		current = core.NewSignal(cx, "a")
		tab := func(name string, width core.Points) core.Node {
			return base.Tab{TestID: name, Style: core.Style{Width: core.Pt(width), Height: core.Pt(20)},
				Selected: func() bool { return current.Get() == name },
				OnPress:  func(core.PressEvent) { current.Set(name) }}
		}
		return base.Tabs{TestID: "list", Style: core.Style{Direction: core.Row},
			IndicatorStyle: core.Style{Bottom: core.Pt(0), Height: core.Pt(2), Margin: core.Edges{Left: 4, Right: 4}},
			Children:       []core.Node{tab("a", 40), tab("b", 60)}}
	})
	bar := func() core.Bounds { return app.Find("id:list").Children[0].Frame }
	if f := bar(); f.Origin.X != 4 || f.Size.Width != 32 {
		t.Fatalf("the indicator is at %v, want on the first tab inside its margins", f)
	}
	app.Press("id:b")
	if f := bar(); f.Origin.X != 44 || f.Size.Width != 52 {
		t.Fatalf("the indicator is at %v, want on the second tab", f)
	}
	current.Set("none")
	app.Settle()
	if f := bar(); f.Size.Width != 0 {
		t.Fatalf("the indicator is at %v with no tab selected, want hidden", f)
	}
}

func TestControlsPassTheirHitSlop(t *testing.T) {
	slop := core.Edges{Top: 4, Left: 2}
	app := nsgotest.Start(t, func(cx *core.Scope) core.Node {
		return core.View{Children: []core.Node{
			base.Button{TestID: "button", HitSlop: slop},
			base.Checkbox{TestID: "checkbox", HitSlop: slop},
			base.Radio{TestID: "radio", HitSlop: slop},
			base.Switch{TestID: "switch", HitSlop: slop},
		}}
	})
	for _, id := range []string{"button", "checkbox", "radio", "switch"} {
		if got := app.FindMounted("id:" + id).Props.HitSlop; got != slop {
			t.Errorf("%s: hit slop %v", id, got)
		}
	}
}
