package core

import (
	"errors"
	"fmt"
	"math"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/oneo-me/nsgo/internal/driver"
)

func init() { driver.CheckCommits = true }

// host runs a tree headless: posts queue until drain, text is approximate.
type host struct {
	t     *testing.T
	root  *Scope
	tree  *Tree
	mu    sync.Mutex
	posts []func()
	clock *ManualClock
	errs  []error
	// frameWanted is set while the graph waits for a display frame.
	frameWanted bool
}

func newHost(t *testing.T, size Size, app func(cx *Scope) Node) *host {
	h := &host{t: t, clock: NewManualClock(time.Unix(0, 0))}
	h.root = NewRoot(RootOptions{
		Post: func(fn func()) {
			h.mu.Lock()
			h.posts = append(h.posts, fn)
			h.mu.Unlock()
		},
		Clock:        h.clock,
		Report:       func(err error) { h.errs = append(h.errs, err) },
		RequestFrame: func() { h.frameWanted = true },
	})
	h.tree = NewTree(h.root, ApproxTextEngine{})
	h.tree.Resize(size)
	if app != nil {
		h.tree.Mount(app)
	}
	return h
}

func (h *host) drain() {
	for {
		h.mu.Lock()
		posts := h.posts
		h.posts = nil
		h.mu.Unlock()
		if len(posts) == 0 {
			return
		}
		for _, fn := range posts {
			fn()
		}
	}
}

// frame runs the display frame the graph waits for, as a host's frame
// clock does when the display reaches it.
func (h *host) frame() {
	if h.frameWanted {
		h.frameWanted = false
		h.root.RunFrame()
	}
}

// advanceFrame moves the clock one frame and runs the frame it makes due.
func (h *host) advanceFrame() {
	h.clock.Advance(driver.FrameInterval)
	h.frame()
	h.drain()
}

func (h *host) waitPost() {
	for {
		h.mu.Lock()
		n := len(h.posts)
		h.mu.Unlock()
		if n > 0 {
			return
		}
		time.Sleep(time.Millisecond)
	}
}

// paced runs the display frames paced lists wait for, committing each.
func (h *host) paced() {
	for h.tree.Pacing() {
		h.frame()
		h.commit()
	}
}

func (h *host) commit() *driver.MountBatch {
	h.t.Helper()
	b := h.tree.Commit()
	if b == nil {
		return &driver.MountBatch{}
	}
	return b
}

func (h *host) find(testID string) *node {
	for _, n := range h.tree.nodes {
		if n != nil && !n.deleted && n.props.TestID == testID {
			return n
		}
	}
	h.t.Fatalf("no node %q", testID)
	return nil
}

func kinds(b *driver.MountBatch) map[driver.ItemKind]int {
	out := map[driver.ItemKind]int{}
	for _, it := range b.Items {
		out[it.Kind]++
	}
	return out
}

func TestSignalsMemosAndEffects(t *testing.T) {
	cx := NewRoot(RootOptions{})
	count := NewSignal(cx, 1)
	doubled := NewMemo(cx, func() int { return count.Get() * 2 })
	parity := NewMemo(cx, func() bool { return doubled.Get()%4 == 0 })
	var seen []int
	var parities []bool
	stop := cx.Effect(func() { seen = append(seen, doubled.Get()) })
	cx.Effect(func() { parities = append(parities, parity.Get()) })
	count.Set(2)
	count.Set(2)
	cx.Batch(func() {
		count.Set(3)
		count.Set(4)
	})
	stop()
	count.Set(5)
	if want := []int{2, 4, 8}; !slices.Equal(seen, want) {
		t.Errorf("effect runs = %v, want %v", seen, want)
	}
	// 2 → false, 4 → true, 8 → true (unchanged: not run), 10 → false
	if want := []bool{false, true, false}; !slices.Equal(parities, want) {
		t.Errorf("parity runs = %v, want %v", parities, want)
	}
}

func TestSignalFuncNotifiesSlices(t *testing.T) {
	cx := NewRoot(RootOptions{})
	list := NewSignalFunc(cx, []int{1}, nil)
	runs := 0
	cx.Effect(func() { list.Get(); runs++ })
	list.Update(func(s []int) []int { return append(s, 2) })
	if runs != 2 {
		t.Fatalf("runs = %d, want 2", runs)
	}
}

// TestReadersUnlinkInAnyOrder has readers of the same sources leave in
// orders other than they came, each move of a reader into a freed slot
// keeping its place in the others' lists.
func TestReadersUnlinkInAnyOrder(t *testing.T) {
	cx := NewRoot(RootOptions{})
	a, b, gate := NewSignal(cx, 0), NewSignal(cx, 0), NewSignal(cx, true)
	runs := make([]int, 6)
	for i := range runs {
		cx.Effect(func() {
			runs[i]++
			// Odd readers read the sources the other way round, and the
			// first two stop reading a once the gate closes.
			if i%2 == 1 {
				b.Get()
			}
			if i >= 2 || gate.Get() {
				a.Get()
			}
			if i%2 == 0 {
				b.Get()
			}
		})
	}
	gate.Set(false)
	a.Set(1)
	b.Set(1)
	if want := []int{3, 3, 3, 3, 3, 3}; !slices.Equal(runs, want) {
		t.Fatalf("runs after the gate = %v, want %v", runs, want)
	}
	a.Set(2)
	if want := []int{3, 3, 4, 4, 4, 4}; !slices.Equal(runs, want) {
		t.Errorf("runs = %v, want %v: readers that left a ran", runs, want)
	}
}

func TestOwnersDisposeAndCleanUp(t *testing.T) {
	cx := NewRoot(RootOptions{})
	on := NewSignal(cx, true)
	n := NewSignal(cx, 0)
	var log []string
	cx.Effect(func() {
		if !on.Get() {
			return
		}
		cx.Effect(func() { log = append(log, "inner "+strconv.Itoa(n.Get())) })
		cx.Cleanup(func() { log = append(log, "cleanup") })
	})
	n.Set(1)
	on.Set(false)
	n.Set(2) // the inner effect is gone
	want := []string{"inner 0", "inner 1", "cleanup"}
	if !slices.Equal(log, want) {
		t.Fatalf("log = %q, want %q", log, want)
	}
}

func TestEffectRerunsOnOwnWrite(t *testing.T) {
	cx := NewRoot(RootOptions{})
	n := NewSignal(cx, 0)
	cx.Effect(func() {
		if v := n.Get(); v < 3 {
			n.Set(v + 1)
		}
	})
	if n.GetUntracked() != 3 {
		t.Fatalf("n = %d, want 3", n.GetUntracked())
	}
}

var themeKey = NewKey("theme", "light")

func TestContextAndPost(t *testing.T) {
	var seen []string
	h := newHost(t, Size{Width: 100, Height: 100}, func(cx *Scope) Node {
		themeKey.Provide(cx, "dark")
		return Func(func(cx *Scope) Node {
			seen = append(seen, themeKey.Use(cx))
			label := NewSignal(cx, "loading")
			go cx.Post(func() { label.Set("done") })
			return Text{ContentFunc: label.Get, TestID: "label"}
		})
	})
	if !slices.Equal(seen, []string{"dark"}) {
		t.Fatalf("context = %q", seen)
	}
	h.commit()
	h.waitPost()
	h.drain()
	if got := h.find("label").text.content; got != "done" {
		t.Fatalf("label = %q", got)
	}
	if themeKey.Use(h.root) != "light" {
		t.Fatal("a key provided below leaked to the root")
	}
}

func TestAfterFuncRunsOnClock(t *testing.T) {
	h := newHost(t, Size{Width: 100, Height: 100}, func(cx *Scope) Node {
		n := NewSignal(cx, 0)
		cx.AfterFunc(time.Second, func() { n.Set(1) })
		return Text{ContentFunc: func() string { return strconv.Itoa(n.Get()) }, TestID: "n"}
	})
	h.clock.Advance(999 * time.Millisecond)
	h.drain()
	if h.find("n").text.content != "0" {
		t.Fatal("fired early")
	}
	h.clock.Advance(time.Millisecond)
	h.drain()
	if h.find("n").text.content != "1" {
		t.Fatal("did not fire")
	}
}

func TestTransitionEndFollowsTheNewTransition(t *testing.T) {
	var ending *Signal[bool]
	ended := 0
	h := newHost(t, Size{Width: 100, Height: 100}, func(cx *Scope) Node {
		ending = NewSignal(cx, false)
		return Overlay{TestID: "o", OnTransitionEnd: func() { ended++ },
			ClassFunc: func() string {
				if ending.Get() {
					return "opacity-0 transition-opacity duration-200 delay-100"
				}
				return ""
			}}
	})
	h.commit()
	h.clock.Advance(time.Second)
	h.drain()
	if ended != 0 {
		t.Fatal("ended with no change")
	}
	ending.Set(true)
	h.commit()
	if got := h.find("o").style.Opacity.Value(); got != 0 {
		t.Fatalf("ending class not applied: opacity %v", got)
	}
	h.clock.Advance(299 * time.Millisecond)
	h.drain()
	if ended != 0 {
		t.Fatal("ended before the delay and duration")
	}
	h.clock.Advance(time.Millisecond)
	h.drain()
	if ended != 1 {
		t.Fatalf("ended %d times", ended)
	}
	// Back with no transition: at once after the commit.
	ending.Set(false)
	h.commit()
	h.clock.Advance(0)
	h.drain()
	if ended != 2 {
		t.Fatalf("ended %d times without a duration", ended)
	}
}

func frames(b *driver.MountBatch) map[NodeID]Bounds {
	out := map[NodeID]Bounds{}
	for _, it := range b.Items {
		if it.Kind == driver.ItemFrame {
			out[it.ID] = it.Frame
		}
	}
	return out
}

func TestColumnStretchesAndPads(t *testing.T) {
	h := newHost(t, Size{Width: 200, Height: 300}, func(cx *Scope) Node {
		return View{Style: Style{Padding: All(10), Gap: 5, FlexGrow: 1}, TestID: "root", Children: []Node{
			Text{Content: "hello", Style: Style{FontSize: 10}, TestID: "text"},
			Input{Style: Style{Height: Pt(40)}, TestID: "input"},
		}}
	})
	h.commit()
	cases := map[string]Bounds{
		"root":  {Size: Size{Width: 200, Height: 300}},
		"text":  {Origin: Point{X: 10, Y: 10}, Size: Size{Width: 180, Height: 12}},
		"input": {Origin: Point{X: 10, Y: 27}, Size: Size{Width: 180, Height: 40}},
	}
	for id, want := range cases {
		if got := h.find(id).frame; got != want {
			t.Errorf("%s = %v, want %v", id, got, want)
		}
	}
}

func TestRowGrowsShrinksAndCenters(t *testing.T) {
	h := newHost(t, Size{Width: 100, Height: 50}, func(cx *Scope) Node {
		return View{Style: Style{Direction: Row, Height: Pt(50), AlignItems: AlignCenter}, Children: []Node{
			View{Style: Style{Width: Pt(30), Height: Pt(20)}, TestID: "fixed"},
			View{Style: Style{FlexGrow: 1, Height: Pt(10)}, TestID: "grow"},
			View{Style: Style{Width: Pt(90), FlexShrink: 1, Height: Pt(10), MinWidth: Pt(40)}, TestID: "shrink"},
		}}
	})
	h.commit()
	cases := map[string]Bounds{
		"fixed":  {Origin: Point{X: 0, Y: 15}, Size: Size{Width: 30, Height: 20}},
		"grow":   {Origin: Point{X: 30, Y: 20}, Size: Size{Width: 0, Height: 10}},
		"shrink": {Origin: Point{X: 30, Y: 20}, Size: Size{Width: 70, Height: 10}},
	}
	for id, want := range cases {
		if got := h.find(id).frame; got != want {
			t.Errorf("%s = %v, want %v", id, got, want)
		}
	}
}

func TestWrapAbsoluteAndPercent(t *testing.T) {
	h := newHost(t, Size{Width: 100, Height: 100}, func(cx *Scope) Node {
		return View{Style: Style{Direction: Row, Wrap: Wrap, Gap: 10, Height: Pct(100)}, Children: []Node{
			View{Style: Style{Width: Pct(50), Height: Pt(10)}, TestID: "a"},
			View{Style: Style{Width: Pct(50), Height: Pt(10)}, TestID: "b"},
			View{Style: Style{Position: Absolute, Right: Pt(5), Bottom: Pt(5), Width: Pt(20), Height: Pt(20)}, TestID: "abs"},
		}}
	})
	h.commit()
	cases := map[string]Bounds{
		"a":   {Origin: Point{X: 0, Y: 0}, Size: Size{Width: 50, Height: 10}},
		"b":   {Origin: Point{X: 0, Y: 20}, Size: Size{Width: 50, Height: 10}},
		"abs": {Origin: Point{X: 75, Y: 75}, Size: Size{Width: 20, Height: 20}},
	}
	for id, want := range cases {
		if got := h.find(id).frame; got != want {
			t.Errorf("%s = %v, want %v", id, got, want)
		}
	}
}

func TestTextInheritsAndWraps(t *testing.T) {
	h := newHost(t, Size{Width: 30, Height: 100}, func(cx *Scope) Node {
		return View{Style: Style{FontSize: 10, Color: RGB(0xff0000)}, Children: []Node{
			Text{Content: "aaaa bbbb", TestID: "text"},
		}}
	})
	h.commit()
	n := h.find("text")
	if n.text.style.Font.Size != 10 || n.text.style.Color != RGB(0xff0000) {
		t.Errorf("style = %+v, want inherited size and color", n.text.style)
	}
	if got, want := n.frame.Size, (Size{Width: 30, Height: 24}); got != want {
		t.Errorf("size = %v, want %v (stretched, two lines)", got, want)
	}
}

func TestCommitSendsOnlyChanges(t *testing.T) {
	var count *Signal[int]
	h := newHost(t, Size{Width: 100, Height: 100}, func(cx *Scope) Node {
		count = NewSignal(cx, 0)
		return View{Children: []Node{
			Text{ContentFunc: func() string { return strconv.Itoa(count.Get()) }, TestID: "n"},
			View{OnPress: func(PressEvent) { count.Update(func(n int) int { return n + 1 }) }, Style: Style{Height: Pt(30)}, TestID: "button"},
		}}
	})
	first := h.commit()
	if k := kinds(first); k[driver.ItemCreate] != 4 || k[driver.ItemInsert] != 3 {
		t.Fatalf("first batch kinds = %v", k)
	}
	if h.tree.Commit() != nil {
		t.Fatal("an unchanged tree commits again")
	}
	button := h.find("button")
	if got := h.tree.HitTest(Point{X: 50, Y: 40}); got != button.id {
		t.Fatalf("hit test = %d, want %d", got, button.id)
	}
	h.tree.Dispatch(driver.Event{Kind: driver.EventPress, ID: button.id})
	b := h.commit()
	if k := kinds(b); k[driver.ItemText] != 1 || len(b.Items) != 1 {
		t.Errorf("batch after press = %v, want one text item", k)
	}
}

func TestShowAndEachMountAndUnmount(t *testing.T) {
	var open *Signal[bool]
	var items *Signal[[]string]
	h := newHost(t, Size{Width: 100, Height: 200}, func(cx *Scope) Node {
		open = NewSignal(cx, true)
		items = NewSignalFunc(cx, []string{"a", "b", "c"}, nil)
		return View{TestID: "list", Children: []Node{
			Show{When: open.Get, Then: func(cx *Scope) Node { return Text{Content: "open", TestID: "open"} }},
			Each(items.Get, func(s string) string { return s }, func(cx *Scope, item func() string, index func() int) Node {
				return Text{ContentFunc: func() string { return fmt.Sprint(index(), item()) }, TestID: "row-" + item()}
			}),
		}}
	})
	h.commit()
	list := h.find("list")
	order := func() (out []string) {
		for _, c := range list.sent.children {
			out = append(out, c.text.content)
		}
		return out
	}
	if got := order(); !slices.Equal(got, []string{"open", "0a", "1b", "2c"}) {
		t.Fatalf("children = %q", got)
	}

	open.Set(false)
	b := h.commit()
	if k := kinds(b); k[driver.ItemDelete] != 1 {
		t.Errorf("closing kinds = %v, want one delete", k)
	}

	c := h.find("row-c")
	items.Set([]string{"c", "a", "d"})
	b = h.commit()
	k := kinds(b)
	if k[driver.ItemDelete] != 1 || k[driver.ItemCreate] != 1 {
		t.Errorf("reorder kinds = %v, want b deleted and d created", k)
	}
	if h.find("row-c") != c {
		t.Error("row c was rebuilt instead of kept")
	}
	if got := order(); !slices.Equal(got, []string{"0c", "1a", "2d"}) {
		t.Errorf("children = %q", got)
	}
	// c moved before a: one insert for c, one for the new d.
	if k[driver.ItemInsert] != 2 {
		t.Errorf("inserts = %d, want 2", k[driver.ItemInsert])
	}
}

// TestEachKeepsRowsOfEqualItems reads plain items again unchanged, which
// leaves their rows as they are; items that may point elsewhere update
// their rows every time.
func TestEachKeepsRowsOfEqualItems(t *testing.T) {
	type pos struct{ x, y int }
	var ints *Signal[[]pos]
	var ptrs *Signal[[]*pos]
	intRuns, ptrRuns := 0, 0
	h := newHost(t, Size{Width: 100, Height: 200}, func(cx *Scope) Node {
		ints = NewSignalFunc(cx, []pos{{1, 2}, {3, 4}}, nil)
		ptrs = NewSignalFunc(cx, []*pos{{5, 6}}, nil)
		return View{Children: []Node{
			Each(ints.Get, func(p pos) int { return p.x }, func(cx *Scope, item func() pos, _ func() int) Node {
				return Dynamic(func(cx *Scope) Node { intRuns++; return Text{Content: fmt.Sprint(item())} })
			}),
			Each(ptrs.Get, func(p *pos) int { return p.x }, func(cx *Scope, item func() *pos, _ func() int) Node {
				return Dynamic(func(cx *Scope) Node { ptrRuns++; return Text{Content: fmt.Sprint(*item())} })
			}),
		}}
	})
	h.commit()
	ints.Set([]pos{{1, 2}, {3, 4}})
	ptrs.Set(ptrs.GetUntracked())
	h.commit()
	if intRuns != 2 || ptrRuns != 2 {
		t.Errorf("rows built %d and %d times, want plain items' 2 and a pointer's 2", intRuns, ptrRuns)
	}
	ints.Set([]pos{{1, 2}, {3, 5}})
	h.commit()
	if intRuns != 3 {
		t.Errorf("rows built %d times, want 3 once an item changed", intRuns)
	}
}

func TestCatchShowsFallbackAndResets(t *testing.T) {
	var fail *Signal[bool]
	var reset func()
	h := newHost(t, Size{Width: 100, Height: 100}, func(cx *Scope) Node {
		fail = NewSignal(cx, false)
		return Catch{
			Try: func(cx *Scope) Node {
				return Dynamic(func(cx *Scope) Node {
					if fail.Get() {
						panic(errors.New("boom"))
					}
					return Text{Content: "fine", TestID: "fine"}
				})
			},
			Fallback: func(cx *Scope, err error, r func()) Node {
				reset = r
				return Text{Content: err.Error()[:len("panic: boom")], TestID: "fallback"}
			},
		}
	})
	h.commit()
	fail.Set(true)
	h.commit()
	if got := h.find("fallback").text.content; got != "panic: boom" {
		t.Fatalf("fallback = %q", got)
	}
	fail.Set(false)
	reset()
	h.commit()
	h.find("fine")
	if len(h.errs) != 0 {
		t.Fatalf("reported %v", h.errs)
	}
}

func TestInputEditIsNotMountedBack(t *testing.T) {
	var name *Signal[string]
	h := newHost(t, Size{Width: 100, Height: 100}, func(cx *Scope) Node {
		name = NewSignal(cx, "")
		upper := func(s string) {
			if len(s) <= 3 {
				name.Set(s)
			}
		}
		return Input{Value: name.Get, OnChange: upper, TestID: "input"}
	})
	h.commit()
	in := h.find("input")

	h.tree.Dispatch(driver.Event{Kind: driver.EventChangeText, ID: in.id, Text: "Go", Count: 1})
	if name.Get() != "Go" {
		t.Fatalf("signal = %q", name.Get())
	}
	if b := h.tree.Commit(); b != nil {
		t.Fatalf("edit mounted back: %+v", b.Items)
	}

	// The app refuses a fourth character: the field goes back.
	h.tree.Dispatch(driver.Event{Kind: driver.EventChangeText, ID: in.id, Text: "Gooo", Count: 2})
	b := h.commit()
	if len(b.Items) != 1 || b.Items[0].Kind != driver.ItemInputValue || b.Items[0].Value != "Go" || b.Items[0].Count != 2 {
		t.Fatalf("items = %+v, want the value Go after edit 2", b.Items)
	}
}

func TestScrollContentSize(t *testing.T) {
	h := newHost(t, Size{Width: 100, Height: 100}, func(cx *Scope) Node {
		return Scroll{Style: Style{FlexGrow: 1}, TestID: "scroll", Children: []Node{
			View{Style: Style{Height: Pt(80)}},
			View{Style: Style{Height: Pt(80)}},
		}}
	})
	h.commit()
	s := h.find("scroll")
	if got, want := s.scroll.content, (Size{Width: 100, Height: 160}); got != want {
		t.Fatalf("content = %v, want %v", got, want)
	}
}

func TestIncreasingRun(t *testing.T) {
	got := increasingRun([]int{2, 0, -1, 1, 3})
	want := []bool{false, true, false, true, true}
	if !slices.Equal(got, want) {
		t.Fatalf("run = %v, want %v", got, want)
	}
}

func TestStateStyles(t *testing.T) {
	var disabled *Signal[bool]
	h := newHost(t, Size{Width: 100, Height: 100}, func(cx *Scope) Node {
		disabled = NewSignal(cx, false)
		return View{
			Style: Style{Background: RGB(0x0000ff),
				Hover:    &Style{Background: RGB(0x00ff00)},
				Active:   &Style{Background: RGB(0xff0000)},
				Disabled: &Style{Opacity: Alpha(0.5)}},
			OnPress:  func(PressEvent) {},
			Disabled: disabled.Get,
			TestID:   "button",
		}
	})
	h.commit()
	n := h.find("button")
	if m := n.events(); !m.Has(driver.EventPressDown) || !m.Has(driver.EventMouseEnter) {
		t.Fatalf("mask = %b, want press down and hover reported", m)
	}
	bg := func() Color { return n.style.Background }
	h.tree.Dispatch(driver.Event{Kind: driver.EventMouseEnter, ID: n.id})
	if bg() != RGB(0x00ff00) {
		t.Errorf("hovered = %x", bg())
	}
	h.tree.Dispatch(driver.Event{Kind: driver.EventPressDown, ID: n.id})
	if bg() != RGB(0xff0000) {
		t.Errorf("pressed = %x, want active over hover", bg())
	}
	h.tree.Dispatch(driver.Event{Kind: driver.EventPressUp, ID: n.id})
	h.tree.Dispatch(driver.Event{Kind: driver.EventMouseLeave, ID: n.id})
	if bg() != RGB(0x0000ff) {
		t.Errorf("released = %x", bg())
	}
	disabled.Set(true)
	h.tree.Dispatch(driver.Event{Kind: driver.EventPressDown, ID: n.id})
	if bg() != RGB(0x0000ff) || n.style.Opacity.Value() != 0.5 {
		t.Errorf("disabled: background %x, opacity %v", bg(), n.style.Opacity.Value())
	}
	b := h.commit()
	if k := kinds(b); k[driver.ItemViewStyle] != 1 {
		t.Errorf("kinds = %v, want one view style", k)
	}
}

// TestActiveStyleInsideAButton keeps a press on a button's face the
// button's: a view inside it whose Active style alone follows presses
// would take them on the platforms that press the innermost such view.
func TestActiveStyleInsideAButton(t *testing.T) {
	face := View{Style: Style{Active: &Style{Opacity: Alpha(0.5)}}, TestID: "face"}
	h := newHost(t, Size{Width: 100, Height: 100}, func(cx *Scope) Node {
		return View{Children: []Node{
			View{OnPress: func(PressEvent) {}, Children: []Node{face}},
			View{Children: []Node{View{Style: face.Style, TestID: "alone"}}},
		}}
	})
	h.commit()
	if m := h.find("face").events(); m.Has(driver.EventPressDown) || m.Has(driver.EventPressUp) {
		t.Errorf("the face inside the button follows presses: %b", m)
	}
	if m := h.find("alone").events(); !m.Has(driver.EventPressDown) {
		t.Errorf("a view outside any button does not follow presses for its Active style: %b", m)
	}
}

func TestEnvironment(t *testing.T) {
	var widths []Points
	var darks []bool
	var materials []WindowMaterial
	var states []Lifecycle
	h := newHost(t, Size{Width: 100, Height: 100}, func(cx *Scope) Node {
		cx.Effect(func() { states = append(states, cx.Env().Lifecycle()) })
		cx.Effect(func() { widths = append(widths, cx.Env().Window().Width) })
		cx.Effect(func() { darks = append(darks, cx.Env().Dark()) })
		cx.Effect(func() { materials = append(materials, cx.Env().WindowMaterial()) })
		return Text{Content: "Aa", Style: Style{FontSize: 10}, TestID: "text"}
	})
	h.commit()
	h.tree.Resize(Size{Width: 200, Height: 100})
	h.tree.SetEnvironment(Environment{Dark: true, FontScale: 2, WindowMaterial: WindowMaterialSidebar})
	h.tree.SetEnvironment(Environment{Dark: true, FontScale: 2, WindowMaterial: WindowMaterialSidebar}) // unchanged: no runs
	h.commit()
	if !slices.Equal(widths, []Points{100, 200}) || !slices.Equal(darks, []bool{false, true}) ||
		!slices.Equal(materials, []WindowMaterial{WindowMaterialNone, WindowMaterialSidebar}) {
		t.Fatalf("widths %v, darks %v, materials %v", widths, darks, materials)
	}
	if got := h.find("text").text.style.Font.Size; got != 20 {
		t.Fatalf("font size = %v, want scaled to 20", got)
	}
	h.tree.SetEnvironment(Environment{Dark: true, FontScale: 2, WindowMaterial: WindowMaterialSidebar, Lifecycle: LifecycleBackground})
	h.commit()
	if !slices.Equal(states, []Lifecycle{LifecycleActive, LifecycleBackground}) {
		t.Fatalf("lifecycle %v", states)
	}
}

func TestPlaceFlipsAndClamps(t *testing.T) {
	area := Bounds{Size: Size{Width: 100, Height: 100}}
	anchor := Bounds{Origin: Point{X: 40, Y: 80}, Size: Size{Width: 20, Height: 10}}
	p, side, room := place(anchor, Size{Width: 30, Height: 30}, area, PlaceBottom, 0, 4, false)
	if side != PlaceTop || p != (Point{X: 35, Y: 46}) {
		t.Errorf("no room below: %v %v, want flipped above at {35 46}", p, side)
	}
	if room != (Size{Width: 100, Height: 76}) {
		t.Errorf("room above %v, want 100×76", room)
	}
	p, _, _ = place(Bounds{Origin: Point{X: 90, Y: 10}, Size: Size{Width: 10, Height: 10}}, Size{Width: 30, Height: 10}, area, PlaceBottom, AlignStart, 0, false)
	if p.X != 70 {
		t.Errorf("x = %v, want clamped to 70", p.X)
	}
}

func TestOverlayModalAndStack(t *testing.T) {
	var anchor Ref
	var log []string
	var routes *Signal[[]string]
	h := newHost(t, Size{Width: 200, Height: 300}, func(cx *Scope) Node {
		routes = NewSignalFunc(cx, []string{"home", "detail"}, nil)
		return View{Style: Style{FlexGrow: 1}, Children: []Node{
			View{Ref: &anchor, Style: Style{Height: Pt(40), Margin: Edges{Top: 20}}, TestID: "anchor"},
			Overlay{Anchor: &anchor, Offset: 5, Style: Style{Width: Pt(50), Height: Pt(30)}, TestID: "popover",
				OnCancel:       func() { log = append(log, "cancel") },
				OnPressOutside: func(Point) { log = append(log, "outside") }},
			Stack{Style: Style{FlexGrow: 1}, Children: []Node{
				Each(routes.Get, func(r string) string { return r }, func(cx *Scope, r func() string, _ func() int) Node {
					return Screen{Title: r(), TestID: "screen-" + r(),
						OnDismiss: func() {
							log = append(log, "dismiss "+r())
							routes.Update(func(rs []string) []string { return rs[:len(rs)-1] })
						}}
				}),
			}},
		}}
	})
	h.commit()
	pop := h.find("popover")
	if got, want := pop.frame, (Bounds{Origin: Point{X: 75, Y: 65}, Size: Size{Width: 50, Height: 30}}); got != want {
		t.Errorf("popover = %v, want centered below the anchor", got)
	}
	if pop.nativeParent() != h.tree.view {
		t.Error("the overlay is not hoisted to the root view")
	}
	if !h.find("screen-home").screen.hidden || h.find("screen-detail").screen.hidden {
		t.Error("the stack does not show only its top screen")
	}
	h.tree.Dispatch(driver.Event{Kind: driver.EventPressOutside, ID: pop.id, Point: Point{X: 100, Y: 40}})
	h.tree.Dispatch(driver.Event{Kind: driver.EventPressOutside, ID: pop.id, Point: Point{X: 5, Y: 200}})
	h.tree.Dispatch(driver.Event{Kind: driver.EventBack})
	h.commit()
	if want := []string{"outside", "cancel"}; !slices.Equal(log, want) {
		t.Fatalf("log = %q, want a press on the anchor ignored and back sent to the overlay", log)
	}
	log = nil
	// Without the overlay's cancel, back pops the stack.
	pop.h.cancel = nil
	h.tree.Dispatch(driver.Event{Kind: driver.EventBack})
	h.commit()
	if want := []string{"dismiss detail"}; !slices.Equal(log, want) {
		t.Fatalf("log = %q", log)
	}
	if h.find("screen-home").screen.hidden {
		t.Error("home is not shown after popping")
	}
	h.tree.Dispatch(driver.Event{Kind: driver.EventBack})
	if len(log) != 1 {
		t.Error("the last screen popped")
	}
}

func TestModalIsALayerOfItsOwn(t *testing.T) {
	var closed string
	h := newHost(t, Size{Width: 200, Height: 300}, func(cx *Scope) Node {
		return View{Style: Style{Padding: All(20)}, Children: []Node{
			Modal{TestID: "modal", Style: Style{Padding: All(10)}, OnRequestClose: func(r string) { closed = r },
				Children: []Node{Text{Content: "hi", TestID: "text"}}},
		}}
	})
	h.commit()
	m := h.find("modal")
	if m.frame != (Bounds{Size: Size{Width: 200, Height: 300}}) || m.nativeParent() != h.tree.view {
		t.Errorf("modal = %v, want the root's size, hosted by it", m.frame)
	}
	if got := h.find("text").frame.Origin; got != (Point{X: 10, Y: 10}) {
		t.Errorf("text at %v, want inside the modal's padding", got)
	}
	h.tree.Dispatch(driver.Event{Kind: driver.EventBack})
	if closed != "cancel" {
		t.Errorf("close reason = %q", closed)
	}
}

func TestImageSizesFromItsSource(t *testing.T) {
	h := newHost(t, Size{Width: 300, Height: 300}, func(cx *Scope) Node {
		return View{Style: Style{AlignItems: AlignStart}, Children: []Node{
			Image{Source: ImageSource{URI: "asset:/logo.png", Size: Size{Width: 40, Height: 20}, Scales: ScalesOf(1, 2, 3)}, TestID: "natural"},
			Image{Source: ImageSource{URI: "asset:/single.png", Size: Size{Width: 40, Height: 20}}, TestID: "single"},
			Image{Source: ImageSource{URI: "https://x/y.png", Size: Size{Width: 40, Height: 20}}, Style: Style{Width: Pt(100)}, TestID: "wide"},
			Image{Source: Src("https://x/z.png"), TestID: "unknown"},
		}}
	})
	h.tree.SetEnvironment(Environment{PixelRatio: 2})
	b := h.commit()
	sizes := map[string]Size{"natural": {Width: 40, Height: 20}, "wide": {Width: 100, Height: 50}, "unknown": {}}
	for id, want := range sizes {
		if got := h.find(id).frame.Size; got != want {
			t.Errorf("%s = %v, want %v", id, got, want)
		}
	}
	for _, it := range b.Items {
		if it.Kind == driver.ItemImage && it.ID == h.find("natural").id && it.Image.Source.URI != "asset:/logo@2x.png" {
			t.Errorf("source = %q, want the @2x variant", it.Image.Source.URI)
		}
		if it.Kind == driver.ItemImage && it.ID == h.find("single").id && it.Image.Source.URI != "asset:/single.png" {
			t.Errorf("source = %q, want the plain file", it.Image.Source.URI)
		}
	}
}

func TestImageScalesPick(t *testing.T) {
	for _, c := range []struct {
		set   ImageScales
		ratio float32
		want  float32
	}{
		{0, 2, 0},
		{ScalesOf(1, 2, 3), 1, 1},
		{ScalesOf(1, 2, 3), 1.5, 2},
		{ScalesOf(1, 2, 3), 2.5, 3},
		{ScalesOf(1, 2, 3), 4, 3},
		{ScalesOf(1, 3), 2, 3},
		{ScalesOf(2.5, -1), 2, 0},
	} {
		if got := c.set.Pick(c.ratio); got != c.want {
			t.Errorf("%08b at %v = %v, want %v", c.set, c.ratio, got, c.want)
		}
	}
}

func TestListMountsAWindowAndReusesCells(t *testing.T) {
	var count *Signal[int]
	ends := 0
	h := newHost(t, Size{Width: 200, Height: 300}, func(cx *Scope) Node {
		count = NewSignal(cx, 10000)
		return List{Count: count.Get, Style: Style{FlexGrow: 1}, TestID: "list",
			OnLoadMore: func() { ends++ },
			Row: func(cx *Scope, row ListRow) Node {
				return Text{ContentFunc: func() string { return "row " + strconv.Itoa(row.Index()) },
					Style: Style{Height: Pt(30)}}
			}}
	})
	h.commit()
	list := h.find("list")
	ln := list.list
	mounted := func() (n int, first, last int) {
		first = -1
		for i := range ln.byIndex {
			n++
			if first < 0 || i < first {
				first = i
			}
			last = max(last, i)
		}
		return
	}
	n, first, last := mounted()
	if first != 0 || n > 40 || n < 10 {
		t.Fatalf("mounted %d rows %d..%d, want a window from 0", n, first, last)
	}
	// Real sizes replaced the estimate.
	if got := ln.st.rows.sum(10); got != 300 {
		t.Errorf("first ten rows = %v points, want 300 measured", got)
	}
	measured := 0
	for _, k := range ln.st.sizes {
		if !isNaN(k) {
			measured++
		}
	}
	if got, want := list.scroll.content.Height, Points(30*measured+50*(10000-measured)); measured < 10 || got != want {
		t.Errorf("content = %v with %d rows measured, want measured rows plus estimates (%v)", got, measured, want)
	}
	cells := len(h.tree.nodes)
	h.tree.Dispatch(driver.Event{Kind: driver.EventScroll, ID: list.id, Point: Point{Y: 3000}})
	h.commit()
	_, first, last = mounted()
	// Row 65 or so sits at 3000; the window starts a viewport above it.
	if first > 66 || last < 70 {
		t.Errorf("after scrolling to 3000, rows %d..%d are mounted", first, last)
	}
	// The window now holds ~30 measured rows instead of ~20 estimated
	// ones: 10 more cells of 2 nodes. Rebuilding would add ~60.
	if grew := len(h.tree.nodes) - cells; grew > 24 {
		t.Errorf("scrolling created %d nodes, want cells reused", grew)
	}
	row := ln.byIndex[first+1]
	if row == nil || row.node.children[0].text.content != "row "+strconv.Itoa(first+1) {
		t.Errorf("a reused cell shows the wrong row")
	}
	count.Set(20)
	h.commit()
	// The viewport binds at once, its overscan on the next frames.
	if n, _, _ := mounted(); n < 10 {
		t.Errorf("after shrinking to 20 rows: %d mounted, want the viewport's 10", n)
	}
	h.paced()
	if n, _, last := mounted(); last != 19 || n != 20 {
		t.Errorf("after shrinking to 20 rows: %d mounted, last %d", n, last)
	}
	if ends != 1 {
		t.Errorf("OnLoadMore ran %d times, want once", ends)
	}
}

func TestFenwick(t *testing.T) {
	f := newFenwick([]Points{10, 20, 30, 40})
	if f.sum(3) != 60 || f.find(0) != 0 || f.find(29) != 1 || f.find(30) != 2 || f.find(1000) != 4 {
		t.Fatalf("sum %v, find %d %d %d %d", f.sum(3), f.find(0), f.find(29), f.find(30), f.find(1000))
	}
	f.set(1, 5)
	if f.sum(2) != 15 || f.find(14) != 1 {
		t.Fatal("set did not update the sums")
	}
}

func TestAnimateStepsOnTheClock(t *testing.T) {
	var x *Signal[float64]
	var steps []float64
	h := newHost(t, Size{Width: 100, Height: 100}, func(cx *Scope) Node {
		x = NewSignal(cx, 0.0)
		cx.Animate(100*time.Millisecond, Linear, func(p float64) { x.Set(p); steps = append(steps, p) })
		return View{StyleFunc: func() Style {
			return Style{Transform: Transform{TranslateX: Points(x.Get() * 100)}, Transition: Transition{}}
		}, TestID: "box"}
	})
	for range 10 {
		h.advanceFrame()
	}
	if last := steps[len(steps)-1]; last != 1 || len(steps) < 6 {
		t.Fatalf("steps = %v, want frames ending at 1", steps)
	}
	b := h.commit()
	for _, it := range b.Items {
		if it.Kind == driver.ItemViewStyle && it.ID == h.find("box").id && it.ViewStyle.Transform.TranslateX != 100 {
			t.Errorf("transform = %+v", it.ViewStyle.Transform)
		}
	}
	if got := EaseInOut.At(0.5); math.Abs(got-0.5) > 1e-6 {
		t.Errorf("ease-in-out at half = %v", got)
	}
}

func TestSpansStyleRangesAndLink(t *testing.T) {
	var name *Signal[string]
	pressed := 0
	h := newHost(t, Size{Width: 300, Height: 100}, func(cx *Scope) Node {
		name = NewSignal(cx, "Ada")
		return Text{Style: Style{FontSize: 10}, TestID: "text", Spans: []Span{
			{Content: "Hi "},
			{ContentFunc: name.Get, Style: Style{FontWeight: FontWeightBold}},
			{Content: ", see ", Style: Style{}},
			{Content: "docs", Style: Style{Color: RGB(0x0969da)}, OnPress: func(PressEvent) { pressed++ }},
		}}
	})
	h.commit()
	n := h.find("text")
	at := n.text.text
	if at.Text != "Hi Ada, see docs" || len(at.Spans) != 4 {
		t.Fatalf("text = %q with %d spans", at.Text, len(at.Spans))
	}
	if sp := at.Spans[1]; sp.Start != 3 || sp.End != 6 || sp.Style.Font.Weight != FontWeightBold || sp.Style.Font.Size != 10 {
		t.Errorf("bold span = %+v", sp)
	}
	if !at.Spans[3].Link || !n.events().Has(driver.EventPress) {
		t.Error("the link span is not reported pressable")
	}
	h.tree.Dispatch(driver.Event{Kind: driver.EventPress, ID: n.id, Index: 1})
	h.tree.Dispatch(driver.Event{Kind: driver.EventPress, ID: n.id, Index: 13})
	if pressed != 1 {
		t.Errorf("pressed = %d, want only the press on the link", pressed)
	}
	name.Set("Grace")
	h.commit()
	if n.text.text.Text != "Hi Grace, see docs" || n.text.text.Spans[3].Start != 14 {
		t.Errorf("after the change: %q %+v", n.text.text.Text, n.text.text.Spans[3])
	}
}

func TestTextPressOutsideItsLinks(t *testing.T) {
	var link, text int
	h := newHost(t, Size{Width: 300, Height: 100}, func(cx *Scope) Node {
		return Text{TestID: "text", OnPress: func(PressEvent) { text++ }, Spans: []Span{
			{Content: "Read "},
			{Content: "docs", OnPress: func(PressEvent) { link++ }},
		}}
	})
	h.commit()
	n := h.find("text")
	h.tree.Dispatch(driver.Event{Kind: driver.EventPress, ID: n.id, Index: 1})
	h.tree.Dispatch(driver.Event{Kind: driver.EventPress, ID: n.id, Index: 6})
	if link != 1 || text != 1 {
		t.Errorf("link pressed %d times and text %d, want once each", link, text)
	}
}

func TestCanvasDrawsAtItsSizeAndOnChange(t *testing.T) {
	var level *Signal[float32]
	draws := 0
	h := newHost(t, Size{Width: 200, Height: 100}, func(cx *Scope) Node {
		level = NewSignal(cx, float32(0.5))
		return Canvas{Style: Style{Height: Pt(40)}, TestID: "meter", Draw: func(d *Drawing, size Size) {
			draws++
			d.SetFillColor(RGB(0x1f883d)).FillRect(0, 0, float32(size.Width)*level.Get(), float32(size.Height))
		}}
	})
	drawing := func(b *driver.MountBatch) *Drawing {
		for _, it := range b.Items {
			if it.Kind == driver.ItemCanvas {
				return it.Drawing
			}
		}
		return nil
	}
	d := drawing(h.commit())
	if d == nil || draws != 1 {
		t.Fatalf("first commit drew %d times, drawing %v", draws, d)
	}
	rect := func(d *Drawing) DrawOp {
		for _, op := range d.Ops {
			if op.Kind == DrawRect {
				return op
			}
		}
		return DrawOp{}
	}
	if r := rect(d); r.W != 100 || r.H != 40 {
		t.Fatalf("rect = %+v, want half of 200 by 40", r)
	}
	level.Set(1)
	if d := drawing(h.commit()); d == nil || rect(d).W != 200 {
		t.Fatal("a change did not redraw")
	}
}

func TestAnEmptyFirstDrawingSendsNothing(t *testing.T) {
	var on *Signal[bool]
	h := newHost(t, Size{Width: 200, Height: 100}, func(cx *Scope) Node {
		on = NewSignal(cx, false)
		return Canvas{Style: Style{Height: Pt(40)}, Draw: func(d *Drawing, size Size) {
			if on.Get() {
				d.FillRect(0, 0, 10, 10)
			}
		}}
	})
	count := func(b *driver.MountBatch) (n int) {
		for _, it := range b.Items {
			if it.Kind == driver.ItemCanvas {
				n++
			}
		}
		return n
	}
	if n := count(h.commit()); n != 0 {
		t.Fatalf("an empty first drawing sent %d canvas items", n)
	}
	on.Set(true)
	if n := count(h.commit()); n != 1 {
		t.Fatalf("the first real drawing sent %d canvas items", n)
	}
	on.Set(false)
	if n := count(h.commit()); n != 1 {
		t.Fatal("an empty drawing after one that drew should clear the canvas")
	}
}

func TestDrawingsPaintWithTheirViewsStyle(t *testing.T) {
	var fill *Signal[Paint]
	draws := 0
	h := newHost(t, Size{Width: 200, Height: 100}, func(cx *Scope) Node {
		fill = NewSignal(cx, PaintColor(RGB(0x0000ff)))
		return View{
			StyleFunc: func() Style {
				return Style{Color: RGB(0xff0000), Fill: fill.Get(), StrokeWidth: 3, StrokeDash: []Points{4}}
			},
			Children: []Node{Canvas{Style: Style{Height: Pt(40)}, Draw: func(d *Drawing, size Size) {
				draws++
				d.FillFromView(PaintNone).StrokeFromView(PaintCurrentColor).LineWidthFromView().LineDashFromView()
				d.FillRect(0, 0, 10, 10)
			}}},
		}
	})
	sent := func(b *driver.MountBatch) *Drawing {
		for _, it := range b.Items {
			if it.Kind == driver.ItemCanvas {
				return it.Drawing
			}
		}
		return nil
	}
	d := sent(h.commit())
	if d == nil {
		t.Fatal("no drawing sent")
	}
	if fill, stroke := d.Ops[0].Color, d.Ops[1].Color; fill != RGB(0x0000ff) || stroke != RGB(0xff0000) {
		t.Fatalf("fill %08x, stroke %08x: want the view's blue and its text color", fill, stroke)
	}
	if w, dash := d.Ops[2].W, d.Ops[3].Dash; w != 3 || dash == nil || len(*dash) != 2 || (*dash)[1] != 4 {
		t.Fatalf("width %v, dash %v: want 3 and 4 4", w, dash)
	}
	fill.Set(Paint{})
	d = sent(h.commit())
	if d == nil || draws != 1 {
		t.Fatalf("a paint change sent %v after %d draws: want the drawing again, not drawn again", d, draws)
	}
	if d.Ops[0].Color != 0 {
		t.Fatalf("an unset fill painted %08x, want the initial none", d.Ops[0].Color)
	}
}

func TestCanvasFillsShapesWithItsBackground(t *testing.T) {
	var bg *Signal[Color]
	h := newHost(t, Size{Width: 100, Height: 100}, func(cx *Scope) Node {
		bg = NewSignal(cx, RGB(0x00ff00))
		return Canvas{
			Style:     Style{Height: Pt(40), Gradients: []Gradient{{Angle: 180, Stops: []ColorStop{{Color: RGB(0xff0000)}, {Color: RGB(0x0000ff)}}}}},
			StyleFunc: func() Style { return Style{Background: bg.Get()} },
			Draw: func(d *Drawing, size Size) {
				d.BeginPath().Rect(0, 0, 10, 10)
				d.FillBackgroundFromView(Bounds{Origin: Point{X: 10, Y: 20}, Size: Size{Width: 40, Height: 20}})
			},
		}
	})
	b := h.commit()
	var d *Drawing
	for _, it := range b.Items {
		if it.Kind == driver.ItemCanvas {
			d = it.Drawing
		}
	}
	if d == nil {
		t.Fatal("no drawing sent")
	}
	for _, vs := range viewStyles(b) {
		if vs.Background != 0 || !vs.Effects.Empty() {
			t.Fatalf("the view draws its background %08x %v, want it only inside the drawing's shapes", vs.Background, vs.Effects)
		}
	}
	var fills []string
	for _, op := range d.Ops {
		switch op.Kind {
		case driver.DrawFillGradient:
			g := op.Gradient
			fills = append(fills, fmt.Sprintf("gradient %v,%v-%v,%v", g.X1, g.Y1, g.X2, g.Y2))
		case driver.DrawFill:
			fills = append(fills, "fill")
		case driver.DrawFillColor:
			fills = append(fills, fmt.Sprintf("%08x", uint32(op.Color)))
		}
	}
	want := "00000000 00ff00ff fill ff0000ff gradient 30,20-30,40 fill"
	if got := strings.Join(fills, " "); got != want {
		t.Fatalf("fills %q, want the color then the gradient over the area: %q", got, want)
	}
	bg.Set(0)
	b = h.commit()
	for _, it := range b.Items {
		if it.Kind == driver.ItemCanvas {
			d = it.Drawing
		}
	}
	for _, op := range d.Ops {
		if op.Kind == driver.DrawFillColor && op.Color == RGB(0x00ff00) {
			t.Fatal("a cleared background still fills")
		}
	}
}

func TestKeyBindingsFollowOwners(t *testing.T) {
	var log []string
	var open *Signal[bool]
	h := newHost(t, Size{Width: 100, Height: 100}, func(cx *Scope) Node {
		open = NewSignal(cx, true)
		cx.OnKey("Cmd+S", func() { log = append(log, "save") })
		return Show{When: open.Get, Then: func(cx *Scope) Node {
			cx.OnKey("shift+mod+s", func() { log = append(log, "save as") })
			return View{}
		}}
	})
	b := h.commit()
	var keys []driver.KeyBinding
	for _, it := range b.Items {
		if it.Kind == driver.ItemKeymap {
			keys = it.Keys
		}
	}
	if !slices.Equal(keys, []driver.KeyBinding{{Key: "meta+s", InText: true}, {Key: "mod+shift+s", InText: true}}) {
		t.Fatalf("keymap = %+v", keys)
	}
	h.tree.Dispatch(driver.Event{Kind: driver.EventKey, Text: "mod+shift+s"})
	open.Set(false)
	h.commit()
	h.tree.Dispatch(driver.Event{Kind: driver.EventKey, Text: "mod+shift+s"})
	h.tree.Dispatch(driver.Event{Kind: driver.EventKey, Text: "meta+s"})
	if want := []string{"save as", "save"}; !slices.Equal(log, want) {
		t.Fatalf("log = %q, want %q", log, want)
	}
}

func TestChangesInsideRegionsRelayout(t *testing.T) {
	var label *Signal[string]
	var open *Signal[bool]
	h := newHost(t, Size{Width: 300, Height: 300}, func(cx *Scope) Node {
		label = NewSignal(cx, "a")
		open = NewSignal(cx, false)
		return View{Style: Style{AlignItems: AlignStart}, Children: []Node{
			Each(func() []int { return []int{1} }, func(i int) int { return i }, func(cx *Scope, _ func() int, _ func() int) Node {
				return View{TestID: "row", Children: []Node{Text{ContentFunc: label.Get, Style: Style{FontSize: 10}}}}
			}),
			Show{When: open.Get, Then: func(cx *Scope) Node {
				return Modal{TestID: "modal", Children: []Node{Text{Content: "hi"}}}
			}},
		}}
	})
	h.commit()
	label.Set("a much longer label")
	h.commit()
	if got := h.find("row").frame.Size.Width; got < 100 {
		t.Errorf("row width = %v after its text grew", got)
	}
	open.Set(true)
	h.commit()
	if got := h.find("modal").frame.Size; got != (Size{Width: 300, Height: 300}) {
		t.Errorf("modal opened at %v, want the root's size", got)
	}
}

func TestSelectionProps(t *testing.T) {
	var color *Signal[Color]
	h := newHost(t, Size{Width: 100, Height: 100}, func(cx *Scope) Node {
		color = NewSignal(cx, RGB(0x00ff00))
		return View{Selectable: true, StyleFunc: func() Style { return Style{SelectionColor: color.Get()} },
			TestID: "area", Children: []Node{
				Text{Content: "in", TestID: "in"},
				Text{Content: "out", Selectable: SelectableOff, TestID: "out"},
			}}
	})
	b := h.commit()
	props := map[NodeID]driver.ViewProps{}
	for _, it := range b.Items {
		if it.Kind == driver.ItemViewProps {
			props[it.ID] = *it.ViewProps
		}
	}
	if !props[h.find("area").id].SelectionArea {
		t.Error("the area view is not sent as an area")
	}
	if p := props[h.find("in").id]; p.SelectionColor != RGB(0x00ff00) || p.Selectable != SelectableAuto {
		t.Errorf("text props = %+v, want the inherited selection color", p)
	}
	if p := props[h.find("out").id]; p.Selectable != SelectableOff {
		t.Errorf("text props = %+v, want selectable off", p)
	}
	color.Set(RGB(0x0000ff))
	b = h.commit()
	if k := kinds(b); k[driver.ItemViewProps] != 2 || k[driver.ItemText] != 0 {
		t.Errorf("a new selection color sends %v, want the texts' props only", k)
	}
}

func TestHiddenSubtreeHidesItsViewsOverlaysAndModals(t *testing.T) {
	var shown *Signal[bool]
	var cancels int
	h := newHost(t, Size{Width: 200, Height: 300}, func(cx *Scope) Node {
		shown = NewSignal(cx, true)
		return View{Children: []Node{
			View{StyleFunc: func() Style {
				if shown.Get() {
					return Style{}
				}
				return Style{Display: DisplayNone}
			}, TestID: "parent", Children: []Node{
				View{Style: Style{Width: Pt(40), Height: Pt(20)}, TestID: "child", Children: []Node{
					Text{Content: "hi", TestID: "text"},
				}},
				Overlay{Style: Style{Width: Pt(50), Height: Pt(30), Top: Pt(10)}, TestID: "overlay",
					OnCancel: func() { cancels++ }, Children: []Node{
						View{Style: Style{Width: Pt(10), Height: Pt(10)}, TestID: "in-overlay"},
					}},
				Modal{TestID: "modal", Children: []Node{
					View{Style: Style{Width: Pt(10), Height: Pt(10)}, TestID: "in-modal"},
				}},
			}},
		}}
	})
	h.commit()
	if h.find("overlay").frame.Size != (Size{Width: 50, Height: 30}) || h.find("modal").frame.Size != (Size{Width: 200, Height: 300}) {
		t.Fatal("the overlay and modal are not laid out while shown")
	}
	shown.Set(false)
	b := h.commit()
	for _, id := range []string{"parent", "child", "text", "overlay", "in-overlay", "modal", "in-modal"} {
		if f := h.find(id).frame; f != (Bounds{}) {
			t.Errorf("%s frame = %v, want empty", id, f)
		}
	}
	if box := h.find("text").text.box; box != (Bounds{}) {
		t.Errorf("text box = %v, want empty", box)
	}
	hidden := map[NodeID]bool{}
	for _, it := range b.Items {
		if it.Kind == driver.ItemViewStyle {
			hidden[it.ID] = it.ViewStyle.Hidden
		}
	}
	for _, id := range []string{"overlay", "modal"} {
		if !hidden[h.find(id).id] {
			t.Errorf("%s is not sent hidden", id)
		}
	}
	// Back passes the hidden modal and overlay by.
	h.tree.Dispatch(driver.Event{Kind: driver.EventBack})
	if cancels != 0 || !slices.ContainsFunc(h.commit().Items, func(it driver.MountItem) bool { return it.Kind == driver.ItemBackUnhandled }) {
		t.Error("back went to a hidden modal or overlay")
	}
	shown.Set(true)
	h.commit()
	if h.find("overlay").frame.Size != (Size{Width: 50, Height: 30}) || h.find("in-modal").frame.Size != (Size{Width: 10, Height: 10}) {
		t.Error("the overlay and modal do not come back")
	}
}

func TestListLoadsMoreAtEitherEnd(t *testing.T) {
	var count *Signal[int]
	ends, starts := 0, 0
	h := newHost(t, Size{Width: 200, Height: 300}, func(cx *Scope) Node {
		count = NewSignal(cx, 5)
		return List{Count: count.Get, Style: Style{FlexGrow: 1}, TestID: "list",
			OnLoadMore:        func() { ends++ },
			OnLoadMoreAtStart: func() { starts++ },
			Row: func(cx *Scope, row ListRow) Node {
				return View{Style: Style{Height: Pt(30)}}
			}}
	})
	h.commit()
	if ends != 1 || starts != 0 {
		t.Fatalf("a short first page: %d loads at the end and %d at the start, want one at the end only", ends, starts)
	}
	list := h.find("list")
	scroll := func(y Points) {
		h.tree.Dispatch(driver.Event{Kind: driver.EventScroll, ID: list.id, Point: Point{Y: y}})
		h.commit()
	}
	count.Set(200)
	h.commit()
	scroll(3000)
	if ends != 1 {
		t.Errorf("loaded %d times at the end, want once per count", ends)
	}
	scroll(200)
	scroll(150)
	if starts != 1 {
		t.Errorf("loaded %d times at the start, want once per approach", starts)
	}
	count.Set(220)
	h.commit()
	if starts != 2 {
		t.Errorf("a count change near the start loaded %d times, want again", starts)
	}
	scroll(1500)
	scroll(0)
	if starts != 3 {
		t.Errorf("a second approach loaded %d times in all, want 3", starts)
	}
	scroll(1e6)
	if ends != 2 {
		t.Errorf("reaching the end of 220 rows loaded %d times in all, want 2", ends)
	}
}

// TestNewViewsSendOnlyWhatDiffers sends a new view's style, props and
// events only when they differ from a new native view's, as NSH does:
// they were a sixth of the items opening the gallery's DataTable.
func TestNewViewsSendOnlyWhatDiffers(t *testing.T) {
	h := newHost(t, Size{Width: 100, Height: 100}, func(cx *Scope) Node {
		return View{Children: []Node{
			View{},
			View{Style: Style{Background: RGB(0xff0000)}, TestID: "styled"},
			View{OnPress: func(PressEvent) {}},
		}}
	})
	b := h.commit()
	if k := kinds(b); k[driver.ItemCreate] < 4 || k[driver.ItemViewStyle] != 1 || k[driver.ItemViewProps] != 1 || k[driver.ItemEvents] != 1 {
		t.Fatalf("kinds = %v, want one style, props and events", k)
	}
}
