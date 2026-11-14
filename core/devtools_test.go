package core_test

import (
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/internal/devtools"
	"github.com/nsgo-dev/nsgo/internal/driver"
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

func TestDevToolsShowMemosAndReaders(t *testing.T) {
	devtools.Enabled = true
	t.Cleanup(func() { devtools.Enabled = false })
	var site string
	app := nsgotest.Start(t, func(cx *core.Scope) core.Node {
		n := core.NewSignal(cx, 3)
		unread := core.NewSignal(cx, "nobody reads me")
		_ = unread
		var double *core.Memo[int]
		double, site = core.NewMemo(cx, func() int { return n.Get() * 2 }), line()
		return core.Text{ContentFunc: func() string { return strconv.Itoa(double.Get()) }}
	}, nsgotest.Size(100, 50))
	s := devtools.Surfaces()[len(devtools.Surfaces())-1]
	t.Cleanup(func() { devtools.Unregister(s) })
	var owners devtools.Owner
	app.Post(func() { owners = s.Owners() })
	app.Settle()
	var memo *devtools.Computation
	readers := map[string]int{}
	var walk func(o devtools.Owner)
	walk = func(o devtools.Owner) {
		for i, c := range o.Computations {
			if c.Kind == "memo" && c.Site == site {
				memo = &o.Computations[i]
			}
		}
		for _, v := range o.Values {
			readers[v.Text] = v.Readers
		}
		for _, c := range o.Children {
			walk(c)
		}
	}
	walk(owners)
	if memo == nil || memo.Value != "6" || memo.Readers != 1 {
		t.Fatalf("memo = %+v, want 6 read by the text, made at %s", memo, site)
	}
	if readers["3"] != 1 || readers["nobody reads me"] != 0 {
		t.Errorf("readers = %v", readers)
	}
}

func TestCommitsNameWhatTheyRanWhileDevToolsRecord(t *testing.T) {
	driver.TraceCauses, devtools.Enabled = true, true
	defer func() { driver.TraceCauses, devtools.Enabled = false, false }()
	root := core.NewRoot(core.RootOptions{Post: func(func()) {}, Clock: core.NewManualClock(time.Unix(0, 0))})
	tree := core.NewTree(root, core.ApproxTextEngine{})
	tree.Resize(core.Size{Width: 100, Height: 100})
	var count *core.Signal[int]
	_, file, at, _ := runtime.Caller(0)
	tree.Mount(func(cx *core.Scope) core.Node {
		count = core.NewSignal(cx, 0)
		return core.Text{ContentFunc: func() string { return strconv.Itoa(count.Get()) }}
	})
	tree.Commit()
	count.Set(7)
	c := tree.Commit().Causes
	// The binding is made by the tree, so it names the component that built its text.
	want := file + ":" + strconv.Itoa(at+1)
	if len(c.Ran) != 1 || c.Ran[0].Kind != "binding" || c.Ran[0].Site != want || c.Ran[0].Runs != 1 || c.Ran[0].Took <= 0 {
		t.Fatalf("ran = %+v, want the text's binding at %s", c.Ran, want)
	}
	if c.Layout <= 0 {
		t.Errorf("layout took %v", c.Layout)
	}
	if c := tree.Commit(); c != nil {
		t.Errorf("an idle commit sent %+v", c)
	}
}

func TestDevToolsRecordAScrollAsOneStep(t *testing.T) {
	devtools.Enabled = true
	t.Cleanup(func() { devtools.Enabled = false })
	app := nsgotest.Start(t, func(cx *core.Scope) core.Node {
		rows := make([]core.Node, 40)
		for i := range rows {
			rows[i] = core.Text{Content: "row " + strconv.Itoa(i), Style: core.Style{Height: core.Pt(30)}}
		}
		return core.View{Style: core.Style{FlexGrow: 1}, Children: []core.Node{
			core.Scroll{TestID: "list", Style: core.Style{Height: core.Pt(200)}, Children: rows},
			core.View{TestID: "done", Role: core.RoleButton, Style: core.Style{Height: core.Pt(20)}, OnPress: func(core.PressEvent) {}},
		}}
	}, nsgotest.Size(200, 300))
	s := devtools.Surfaces()[len(devtools.Surfaces())-1]
	t.Cleanup(func() { devtools.Unregister(s) })
	var steps []devtools.Step
	app.Post(func() { s.Record(true, func(st devtools.Step) { steps = append(steps, st) }) })
	app.Settle()
	app.ScrollTo("id:list", core.Point{Y: 100})
	app.ScrollTo("id:list", core.Point{Y: 250})
	app.Press("id:done")
	want := []devtools.Step{{Action: "ScrollTo", Selector: "id:list", Offset: [2]float32{0, 250}}, {Action: "Press", Selector: "id:done"}}
	if len(steps) != len(want) || steps[0] != want[0] || steps[1] != want[1] {
		t.Fatalf("steps = %+v, want %+v", steps, want)
	}
	// A scroll left when recording ends is a step too.
	app.ScrollTo("id:list", core.Point{Y: 30})
	app.Post(func() { s.Record(false, nil) })
	app.Settle()
	if last := steps[len(steps)-1]; last.Action != "ScrollTo" || last.Offset != [2]float32{0, 30} {
		t.Errorf("steps = %+v, want the last scroll recorded as recording ended", steps)
	}
}

func TestDevToolsShowClassesAndTheBox(t *testing.T) {
	devtools.Enabled = true
	t.Cleanup(func() { devtools.Enabled = false })
	app := nsgotest.Start(t, func(cx *core.Scope) core.Node {
		return core.View{Children: []core.Node{
			core.View{TestID: "card", Class: "p-2 border", Style: core.Style{Margin: core.All(4), Width: core.Pt(100), Height: core.Pt(50)}},
		}}
	}, nsgotest.Size(200, 200))
	s := devtools.Surfaces()[len(devtools.Surfaces())-1]
	t.Cleanup(func() { devtools.Unregister(s) })
	var root devtools.Node
	app.Post(func() { root = s.Elements() })
	app.Settle()
	card := find(root, func(n devtools.Node) bool { return n.TestID == "card" })
	if card == nil || card.Classes != "p-2 border" {
		t.Fatalf("card = %+v", card)
	}
	if card.Box.Margin != [4]float32{4, 4, 4, 4} || card.Box.Padding != [4]float32{8, 8, 8, 8} || card.Box.Border != [4]float32{1, 1, 1, 1} {
		t.Errorf("box = %+v, want a 4 margin, 8 padding and 1 border", card.Box)
	}
}

// The selector DevTools record for a text finds it again in nsgotest, quotes and line breaks in it too, and a text too long to show whole is marked clipped, cut where a character starts.
func TestDevToolsSelectTextsAsTheyRead(t *testing.T) {
	devtools.Enabled = true
	t.Cleanup(func() { devtools.Enabled = false })
	texts := []string{`Say "hi"`, "Line one\nLine two", strings.Repeat("é", 150)}
	app := nsgotest.Start(t, func(cx *core.Scope) core.Node {
		var views []core.Node
		for _, s := range texts {
			views = append(views, core.Text{Content: s})
		}
		return core.View{Children: views}
	}, nsgotest.Size(300, 300))
	s := devtools.Surfaces()[len(devtools.Surfaces())-1]
	t.Cleanup(func() { devtools.Unregister(s) })
	var root devtools.Node
	app.Post(func() { root = s.Elements() })
	app.Settle()
	for _, want := range texts {
		n := find(root, func(n devtools.Node) bool {
			return n.Kind == "Text" && strings.HasPrefix(want, strings.TrimSuffix(n.Text, "…"))
		})
		if n == nil {
			t.Fatalf("no Text for %q in %+v", want, root)
		}
		if got := app.Find(n.Selector).Text; got != want {
			t.Errorf("%s finds %q, want %q", n.Selector, got, want)
		}
		if clipped := len(want) > 200; n.Clipped != clipped || clipped && !utf8.ValidString(n.Text) {
			t.Errorf("%q shows as %q, clipped %v", want, n.Text, n.Clipped)
		}
	}
}
