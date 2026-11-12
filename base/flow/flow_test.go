package flow

import (
	"math"
	"testing"
	"time"

	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/internal/driver"
	"github.com/nsgo-dev/nsgo/nsgotest"
)

// card is a node's face of a fixed width, as a styled card is.
func card(title string, width core.Points) core.Node {
	return core.View{Style: core.Style{Width: core.Pt(width), Padding: core.All(12), Gap: 4},
		Children: []core.Node{core.Text{Content: title}, core.Text{Content: title + " details"}}}
}

func sample() Data {
	return Data{
		Nodes: []Node{
			{ID: "plan", Label: "Plan", Content: card("Plan", 240)},
			{ID: "build", Label: "Build", Content: card("Build", 240)},
			{ID: "test", Label: "Test", Content: card("Test", 240)},
			{ID: "ship", Label: "Ship", Muted: true, Content: card("Ship", 240)},
			{ID: "fixed", Label: "Fixed", Width: 90, Height: 40},
		},
		Edges: []Edge{
			{Source: "plan", Target: "build"},
			{Source: "plan", Target: "test", Dashed: true, Label: "depends on"},
			{Source: "build", Target: "ship"},
			{Source: "test", Target: "ship"},
			{Source: "ship", Target: "ship"},
			{Source: "ship", Target: "missing"},
		},
	}
}

type harness struct {
	app           *nsgotest.App
	state         State
	data          *core.Signal[int]
	current       Data
	selected      *core.Signal[string]
	picks, opened []string
	direction     Direction
}

func newHarness(t *testing.T) *harness {
	t.Helper()
	h := &harness{current: sample()}
	h.app = nsgotest.Start(t, func(cx *core.Scope) core.Node {
		h.data = core.NewSignal(cx, 0)
		h.selected = core.NewSignal(cx, "")
		return View{TestID: "diagram", Label: "Pipeline", State: &h.state,
			Data:     func() Data { h.data.Get(); return h.current },
			Selected: h.selected.Get,
			OnSelect: func(id string) { h.picks = append(h.picks, id); h.selected.Set(id) },
			OnOpen:   func(id string) { h.opened = append(h.opened, id) },
			Style:    core.Style{Width: core.Pt(900), Height: core.Pt(700)}}
	}, nsgotest.Size(900, 700), nsgotest.Environment(driver.Environment{ReduceMotion: true, Idiom: driver.IdiomDesktop}))
	h.app.Settle()
	if !h.state.Ready() {
		t.Fatalf("diagram did not become ready\n%s", h.app.Dump())
	}
	return h
}

// change shows h.current again.
func (h *harness) change() {
	h.app.Post(func() { h.data.Update(func(n int) int { return n + 1 }) })
	h.app.Settle()
}

// at is where the middle of node id shows in the view.
func (h *harness) at(t *testing.T, id string) driver.Point {
	t.Helper()
	x, y, ok := h.state.Position(id)
	if !ok {
		t.Fatalf("%s is not laid out", id)
	}
	v := h.state.v
	w, ht := v.nodes[v.index[id]].size()
	c := h.state.cam
	return driver.Point{X: core.Points(c.x + (float64(x)+w/2)*c.zoom), Y: core.Points(c.y + (float64(y)+ht/2)*c.zoom)}
}

func TestViewMeasuresAndLaysOut(t *testing.T) {
	h := newHarness(t)
	v := h.state.v
	if len(v.nodes) != 5 || len(v.edges) != 4 {
		t.Fatalf("view kept %d nodes and %d edges", len(v.nodes), len(v.edges))
	}
	for _, n := range v.nodes {
		if !n.measured {
			t.Fatalf("node %s is unmeasured", n.id)
		}
	}
	if n := v.nodes[v.index["plan"]]; n.width != 240 || n.height <= 0 {
		t.Fatalf("card measured %vx%v", n.width, n.height)
	}
	if n := v.nodes[v.index["fixed"]]; n.width != 90 || n.height != 40 {
		t.Fatalf("fixed node measured %vx%v", n.width, n.height)
	}
	_, planY, _ := h.state.Position("plan")
	_, buildY, _ := h.state.Position("build")
	_, shipY, _ := h.state.Position("ship")
	if !(planY < buildY && buildY < shipY) {
		t.Fatalf("ranks at %v, %v, %v", planY, buildY, shipY)
	}
	if _, _, ok := h.state.Position("missing"); ok {
		t.Fatal("unknown node is laid out")
	}
	// The fitted diagram shows whole, at no more than its natural size.
	z := float64(h.state.Zoom())
	if z <= 0 || z > 1 {
		t.Fatalf("fitted zoom %v", z)
	}
	c := h.state.cam
	if c.x < 0 || c.y < 0 || c.x+v.result.width*z > 900 || c.y+v.result.height*z > 700 {
		t.Fatalf("diagram shows at %+v, %vx%v", c, v.result.width, v.result.height)
	}
	if !h.app.Exists("label:Pipeline") || !h.app.Exists("label:Plan") {
		t.Fatalf("diagram or node lacks its label\n%s", h.app.Dump())
	}
}

func TestViewSelectsAndOpens(t *testing.T) {
	h := newHarness(t)
	h.app.PressAt("id:diagram", h.at(t, "build"))
	if len(h.picks) != 1 || h.picks[0] != "build" {
		t.Fatalf("selected %v", h.picks)
	}
	if !h.app.Find("label:Build").Props.Selected {
		t.Fatalf("the selected node does not say so\n%s", h.app.Dump())
	}
	h.app.Advance(time.Second)
	test := h.at(t, "test")
	h.app.PressAt("id:diagram", test)
	h.app.PressAt("id:diagram", test)
	if len(h.opened) != 1 || h.opened[0] != "test" {
		t.Fatalf("opened %v", h.opened)
	}
	// Pressing empty space clears the selection without opening anything.
	h.app.Advance(time.Second)
	h.app.PressAt("id:diagram", driver.Point{X: 4, Y: 690})
	if last := h.picks[len(h.picks)-1]; last != "" || len(h.opened) != 1 {
		t.Fatalf("empty press selected %q and opened %v", last, h.opened)
	}
	// Assistive technology and the keyboard press the node itself.
	h.app.AccessibilityAction("label:Plan", driver.ActionActivate)
	if last := h.picks[len(h.picks)-1]; last != "plan" {
		t.Fatalf("activating selected %q", last)
	}
}

func TestViewPansAndZooms(t *testing.T) {
	h := newHarness(t)
	before := h.state.cam
	h.app.Pan("id:diagram", driver.Point{X: 20, Y: -20}, driver.Point{X: 60, Y: -50})
	if c := h.state.cam; c.x-before.x != 60 || c.y-before.y != -50 {
		t.Fatalf("pan moved the camera from %+v to %+v", before, c)
	}
	if len(h.picks) != 0 {
		t.Fatal("panning selected")
	}
	zoom := h.state.Zoom()
	h.app.Post(h.state.ZoomIn)
	h.app.Settle()
	if h.state.Zoom() <= zoom {
		t.Fatal("zoom in did not zoom")
	}
	h.app.Post(h.state.Fit)
	h.app.Settle()
	if h.state.Zoom() != zoom || h.state.userMoved {
		t.Fatalf("fit zoom %v, want %v", h.state.Zoom(), zoom)
	}
	// A double press on empty space zooms in there.
	h.app.Advance(time.Second)
	h.app.PressAt("id:diagram", driver.Point{X: 4, Y: 690})
	h.app.PressAt("id:diagram", driver.Point{X: 4, Y: 690})
	if got := h.state.Zoom(); math.Abs(float64(got/zoom)-2) > 1e-3 {
		t.Fatalf("double press zoomed to %v from %v", got, zoom)
	}
}

func TestViewWheelAndTouchpad(t *testing.T) {
	h := newHarness(t)
	zoom := float64(h.state.Zoom())
	// A wheel's notch zooms in by ×1.5 over 120 points.
	h.app.Wheel("id:diagram", driver.Point{Y: 120}, driver.Modifiers{})
	if got := float64(h.state.Zoom()); math.Abs(got/zoom-1.5) > 1e-3 {
		t.Fatalf("a notch zoomed %v to %v", zoom, got)
	}
	// Two fingers pan; a sideways wheel pans across.
	before := h.state.cam
	h.app.TouchpadScroll("id:diagram", driver.Point{}, driver.Point{X: 10, Y: 5}, driver.Point{X: 20, Y: -15})
	if c := h.state.cam; c.x-before.x != 30 || c.y-before.y != -10 || c.zoom != before.zoom {
		t.Fatalf("touchpad moved the camera from %+v to %+v", before, c)
	}
	before = h.state.cam
	h.app.Wheel("id:diagram", driver.Point{X: -40}, driver.Modifiers{})
	if c := h.state.cam; c.x-before.x != -40 || c.zoom != before.zoom {
		t.Fatalf("sideways wheel moved the camera from %+v to %+v", before, c)
	}
	// A notch with Control held zooms too.
	zoom = float64(h.state.Zoom())
	h.app.Wheel("id:diagram", driver.Point{Y: -120}, driver.Modifiers{Control: true})
	if got := float64(h.state.Zoom()); math.Abs(got/zoom-1/1.5) > 1e-3 {
		t.Fatalf("a Control-scroll zoomed %v to %v", zoom, got)
	}
	// A pinch zooms around its center.
	zoom = float64(h.state.Zoom())
	h.app.Pinch("id:diagram", 1, 2)
	if got := float64(h.state.Zoom()); math.Abs(got/zoom-2) > 1e-3 && got != maxZoom {
		t.Fatalf("a pinch zoomed %v to %v", zoom, got)
	}
}

func TestViewRelaysOutWhenDataChanges(t *testing.T) {
	h := newHarness(t)
	h.current.Nodes = append(h.current.Nodes, Node{ID: "docs", Label: "Docs", Content: card("Docs", 240)})
	h.current.Edges = append(h.current.Edges, Edge{Source: "ship", Target: "docs"})
	h.change()
	_, shipY, _ := h.state.Position("ship")
	_, docsY, ok := h.state.Position("docs")
	if !ok || docsY <= shipY {
		t.Fatalf("new node at %v below %v", docsY, shipY)
	}
	if !h.state.v.nodes[h.state.v.index["docs"]].measured {
		t.Fatal("new node is unmeasured")
	}
	// A node given a new size is laid out again at it.
	h.current.Nodes[4].Width = 160
	h.change()
	if n := h.state.v.nodes[h.state.v.index["fixed"]]; n.width != 160 {
		t.Fatalf("resized node measured %v wide", n.width)
	}
	h.current = Data{}
	h.change()
	if !h.app.Exists("text:—") {
		t.Fatalf("empty state is missing\n%s", h.app.Dump())
	}
}

func TestViewShownAgainKeepsCamera(t *testing.T) {
	var state State
	shown := (*core.Signal[bool])(nil)
	app := nsgotest.Start(t, func(cx *core.Scope) core.Node {
		shown = core.NewSignal(cx, true)
		return core.Show{When: shown.Get, Then: func(*core.Scope) core.Node {
			return View{State: &state, Data: sample, Style: core.Style{Width: core.Pt(900), Height: core.Pt(700)}}
		}}
	}, nsgotest.Size(900, 700), nsgotest.Environment(driver.Environment{ReduceMotion: true, Idiom: driver.IdiomDesktop}))
	app.Settle()
	app.Post(state.ZoomIn)
	app.Settle()
	camera := state.cam
	app.Post(func() { shown.Set(false) })
	app.Settle()
	app.Post(func() { shown.Set(true) })
	app.Settle()
	if !state.Ready() || state.cam != camera {
		t.Fatalf("diagram shown again moved: camera %+v from %+v", state.cam, camera)
	}
}
