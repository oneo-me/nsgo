package core_test

import (
	"runtime"
	"strconv"
	"strings"
	"testing"

	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/internal/devtools"
	"github.com/nsgo-dev/nsgo/nsgotest"
)

// line is the caller's line, as DevTools name a site in this file: its whole path and the line.
func line() string {
	_, file, n, _ := runtime.Caller(1)
	return file + ":" + strconv.Itoa(n)
}

type devApp struct {
	app                         *nsgotest.App
	surface                     devtools.Surface
	countSite, nameSite, funcAt string
	count                       *core.Signal[int]
}

// startDevApp starts an app under DevTools: a counter a button raises and a field, in a component of its own.
func startDevApp(t *testing.T) *devApp {
	t.Helper()
	devtools.Enabled = true
	t.Cleanup(func() { devtools.Enabled = false })
	d := &devApp{}
	_, file, funcLine, _ := runtime.Caller(0)
	d.funcAt = file + ":" + strconv.Itoa(funcLine+2)
	counter := core.Func(func(cx *core.Scope) core.Node {
		d.count, d.countSite = core.NewSignal(cx, 0), line()
		name := core.NewSignal(cx, "")
		d.nameSite = line()
		return core.View{Children: []core.Node{
			core.Text{ContentFunc: func() string { return "Count " + strconv.Itoa(d.count.Get()) }, TestID: "count"},
			core.View{TestID: "raise", Role: core.RoleButton, Label: "Raise", Style: core.Style{Width: core.Pt(40), Height: core.Pt(20)},
				OnPress: func(core.PressEvent) { d.count.Update(func(n int) int { return n + 1 }) }},
			core.Input{TestID: "name", Value: name.Get, OnChange: name.Set},
		}}
	})
	d.app = nsgotest.Start(t, func(cx *core.Scope) core.Node {
		return core.Group{counter, core.UpdateHighlights{}}
	}, nsgotest.Size(300, 200))
	surfaces := devtools.Surfaces()
	if len(surfaces) == 0 {
		t.Fatal("the tree did not register")
	}
	d.surface = surfaces[len(surfaces)-1]
	t.Cleanup(func() { devtools.Unregister(d.surface) })
	return d
}

// on runs fn on the graph's goroutine, as DevTools call a surface.
func (d *devApp) on(fn func()) {
	d.app.Post(fn)
	d.app.Settle()
}

func find(n devtools.Node, ok func(devtools.Node) bool) *devtools.Node {
	if ok(n) {
		return &n
	}
	for _, c := range n.Children {
		if f := find(c, ok); f != nil {
			return f
		}
	}
	return nil
}

func TestDevToolsShowTheElementsAndWhereTheyWereBuilt(t *testing.T) {
	d := startDevApp(t)
	var root devtools.Node
	d.on(func() { root = d.surface.Elements() })
	count := find(root, func(n devtools.Node) bool { return n.TestID == "count" })
	if count == nil {
		t.Fatalf("no count view in %+v", root)
	}
	if count.Kind != "Text" || count.Text != "Count 0" || count.Selector != "id:count" || count.Component != "func" || count.Site != d.funcAt {
		t.Errorf("count = %+v, want a Text built at %s", *count, d.funcAt)
	}
	if count.Bounds[2] <= 0 || count.Bounds[3] <= 0 {
		t.Errorf("count's bounds = %v", count.Bounds)
	}
	raise := find(root, func(n devtools.Node) bool { return n.TestID == "raise" })
	if raise == nil || raise.Role != "button" || raise.Label != "Raise" || len(raise.Style) == 0 {
		t.Errorf("raise = %+v", raise)
	}
	if find(root, func(n devtools.Node) bool { return strings.HasPrefix(n.TestID, "nsgo-") }) != nil {
		t.Error("the elements show DevTools' own views")
	}
}

func TestDevToolsShowAndEditTheState(t *testing.T) {
	d := startDevApp(t)
	var owners devtools.Owner
	d.on(func() { owners = d.surface.Owners() })
	var values []devtools.Value
	var walk func(o devtools.Owner)
	walk = func(o devtools.Owner) {
		values = append(values, o.Values...)
		for _, c := range o.Children {
			walk(c)
		}
	}
	walk(owners)
	var count *devtools.Value
	for i, v := range values {
		if v.Site == d.countSite {
			count = &values[i]
		}
	}
	if count == nil || count.Type != "int" || count.Text != "0" || !count.Editable {
		t.Fatalf("count = %+v among %+v", count, values)
	}
	var err error
	d.on(func() { err = d.surface.Set(count.ID, "41") })
	if err != nil || d.app.Find("id:count").Text != "Count 41" {
		t.Fatalf("set 41: %v, count shows %q", err, d.app.Find("id:count").Text)
	}
	d.on(func() { err = d.surface.Set(count.ID, "many") })
	if err == nil {
		t.Error("an int took \"many\"")
	}
}

func TestDevToolsRecordTestSteps(t *testing.T) {
	d := startDevApp(t)
	var steps []devtools.Step
	d.on(func() { d.surface.Record(true, func(s devtools.Step) { steps = append(steps, s) }) })
	d.app.Press("id:raise")
	d.app.Type("id:name", "Ada")
	d.app.Clear("id:name")
	d.on(func() { d.surface.Record(false, nil) })
	d.app.Press("id:raise")
	want := []devtools.Step{{Action: "Press", Selector: "id:raise"}, {Action: "Type", Selector: "id:name", Text: "A"},
		{Action: "Type", Selector: "id:name", Text: "d"}, {Action: "Type", Selector: "id:name", Text: "a"}, {Action: "Clear", Selector: "id:name"}}
	if len(steps) != len(want) {
		t.Fatalf("steps = %+v, want %+v", steps, want)
	}
	for i := range want {
		if steps[i] != want[i] {
			t.Errorf("step %d = %+v, want %+v", i, steps[i], want[i])
		}
	}
}

func TestDevToolsPickAViewOnTheSurface(t *testing.T) {
	d := startDevApp(t)
	var picked uint32
	d.on(func() { d.surface.Pick(true, nil, func(id uint32) { picked = id }) })
	// The press reaches DevTools' view over the surface, not the button under it.
	d.app.Press("id:nsgo-devtools-pick")
	if d.app.Find("id:count").Text != "Count 0" {
		t.Error("a press while picking reached the app")
	}
	if picked == 0 {
		t.Fatal("nothing was picked")
	}
	if d.app.Exists("id:nsgo-devtools-pick") {
		t.Error("picking did not end with the pick")
	}
}

func TestDevToolsNameTheComputationsThatRanMost(t *testing.T) {
	d := startDevApp(t)
	d.on(func() { d.surface.Hot(10) })
	for range 3 {
		d.app.Press("id:raise")
	}
	var hot []devtools.Computation
	d.on(func() { hot = d.surface.Hot(10) })
	if len(hot) == 0 || hot[0].Kind != "binding" || hot[0].Site != d.funcAt || hot[0].Runs < 3 {
		t.Fatalf("hot = %+v, want the count's binding at %s first", hot, d.funcAt)
	}
}
