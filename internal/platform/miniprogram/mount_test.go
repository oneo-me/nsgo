package miniprogram

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/nsgo-dev/nsgo/internal/driver"
)

// fakeHost records what the mounter hands the mini program.
type fakeHost struct {
	batches  [][]any
	events   []driver.Event
	services []driver.ItemKind
	ticks    int
}

func (h *fakeHost) submit(ops []byte) {
	var batch []any
	if err := json.Unmarshal(ops, &batch); err != nil {
		panic(string(ops) + ": " + err.Error())
	}
	h.batches = append(h.batches, batch)
}
func (h *fakeHost) dispatch(ev driver.Event)           { h.events = append(h.events, ev) }
func (h *fakeHost) service(it *driver.MountItem)       { h.services = append(h.services, it.Kind) }
func (h *fakeHost) imageURL(driver.ImageSource) string { return "img" }
func (h *fakeHost) paint(driver.NodeID, []driver.DrawOp, driver.Size) string {
	return "data:image/png;base64,"
}
func (h *fakeHost) tick() { h.ticks++ }

// last returns the operations of the last batch with code, as JSON.
func (h *fakeHost) ops(code string) []string {
	var out []string
	for _, b := range h.batches {
		for _, op := range b {
			if o := op.([]any); o[0] == code {
				data, _ := json.Marshal(o[1:])
				out = append(out, string(data))
			}
		}
	}
	return out
}

func frame(id driver.NodeID, x, y, w, h driver.Points) driver.MountItem {
	return driver.MountItem{Kind: driver.ItemFrame, ID: id, Frame: driver.Bounds{Origin: driver.Point{X: x, Y: y}, Size: driver.Size{Width: w, Height: h}}}
}

// mounted is a root view 1 holding view 2 and text 3.
func mounted() (*mounter, *fakeHost) {
	h := &fakeHost{}
	m := newMounter(h)
	m.mount(&driver.MountBatch{Root: 1, Items: []driver.MountItem{
		{Kind: driver.ItemCreate, ID: 1, View: driver.KindView},
		{Kind: driver.ItemCreate, ID: 2, View: driver.KindView},
		{Kind: driver.ItemCreate, ID: 3, View: driver.KindText},
		{Kind: driver.ItemInsert, ID: 2, Parent: 1},
		{Kind: driver.ItemInsert, ID: 3, Parent: 2},
		frame(1, 0, 0, 375, 600),
		frame(2, 10, 20, 200, 100),
		frame(3, 5, 6, 100, 20),
	}})
	return m, h
}

func TestBatchesBecomeOperations(t *testing.T) {
	_, h := mounted()
	if len(h.batches) != 1 {
		t.Fatalf("batches = %d", len(h.batches))
	}
	got := strings.Join(h.ops("c"), " ") + " " + strings.Join(h.ops("root"), " ") + " " + strings.Join(h.ops("i"), " ")
	if want := `[1,"view"] [2,"view"] [3,"text"] [1] [1,2,null] [2,3,null]`; got != want {
		t.Fatalf("structure = %s, want %s", got, want)
	}
	if f := h.ops("f"); f[len(f)-1] != "[3,5,6,100,20]" {
		t.Fatalf("frames = %v", f)
	}
}

func TestChildrenArePlacedInsideTheBorder(t *testing.T) {
	m, h := mounted()
	m.mount(&driver.MountBatch{Items: []driver.MountItem{
		{Kind: driver.ItemViewStyle, ID: 2, ViewStyle: &driver.ViewStyle{Opacity: 1, BorderWidth: 2, BorderColor: driver.RGB(0),
			CornerRadius: 4, Background: driver.RGB(0xffffff)}},
	}})
	f := h.ops("f")
	if f[len(f)-1] != "[3,3,4,100,20]" {
		t.Fatalf("child frame = %v", f)
	}
	s := h.ops("s")
	if !strings.Contains(s[len(s)-1], "border-width:2px") || !strings.Contains(s[len(s)-1], "border-radius:4px") {
		t.Fatalf("style = %v", s)
	}
}

func TestPressesReachTheViewsThatAskForThem(t *testing.T) {
	m, h := mounted()
	m.mount(&driver.MountBatch{Items: []driver.MountItem{
		{Kind: driver.ItemEvents, ID: 2, Events: 1<<driver.EventPress | 1<<driver.EventPressDown | 1<<driver.EventPressUp},
	}})
	if e := h.ops("e"); e[len(e)-1] != "[2,3]" {
		t.Fatalf("events = %v", e)
	}
	m.press(2, driver.EventPress, driver.Point{X: 30, Y: 50})
	m.press(3, driver.EventPress, driver.Point{X: 30, Y: 50})
	if len(h.events) != 1 || h.events[0].ID != 2 || h.events[0].Point != (driver.Point{X: 20, Y: 30}) {
		t.Fatalf("events = %+v", h.events)
	}
}

func TestTextRunsAndLinks(t *testing.T) {
	m, h := mounted()
	e := NewTextEngine(&approx{})
	at := plain("go to docs")
	at.Spans = []driver.TextSpan{{Start: 6, End: 10, Style: at.Style, Link: true}}
	l := e.Prepare(at).Layout(driver.Size{Width: 100, Height: 12})
	m.mount(&driver.MountBatch{Items: []driver.MountItem{
		{Kind: driver.ItemText, ID: 3, Text: &driver.TextContent{Text: at.Text, Layout: l, Spans: at.Spans}},
		{Kind: driver.ItemEvents, ID: 3, Events: 1 << driver.EventPress},
	}})
	if tx := h.ops("t"); len(tx) != 1 || !strings.Contains(tx[0], `"go to docs"`) {
		t.Fatalf("text = %v", tx)
	}
	// The text is at 15, 26 on the page; "docs" starts 30 points in.
	m.press(3, driver.EventPress, driver.Point{X: 15 + 5, Y: 30})
	m.press(3, driver.EventPress, driver.Point{X: 15 + 37, Y: 30})
	if len(h.events) != 1 || h.events[0].Index < 6 {
		t.Fatalf("link presses = %+v", h.events)
	}
}

func TestStaleValuesAreNotShown(t *testing.T) {
	h := &fakeHost{}
	m := newMounter(h)
	m.mount(&driver.MountBatch{Root: 1, Items: []driver.MountItem{
		{Kind: driver.ItemCreate, ID: 1, View: driver.KindInput},
		{Kind: driver.ItemEvents, ID: 1, Events: 1 << driver.EventChangeText},
	}})
	m.edited(1, "ab")
	m.mount(&driver.MountBatch{Items: []driver.MountItem{
		{Kind: driver.ItemInputValue, ID: 1, Value: "a", Count: 0},
		{Kind: driver.ItemInputValue, ID: 1, Value: "ab", Count: 1},
		{Kind: driver.ItemInputValue, ID: 1, Value: "AB", Count: 1},
	}})
	if v := h.ops("iv"); len(v) != 1 || v[0] != `[1,"AB"]` {
		t.Fatalf("values = %v", v)
	}
	if len(h.events) != 1 || h.events[0].Count != 1 || h.events[0].Text != "ab" {
		t.Fatalf("edits = %+v", h.events)
	}
}

func TestScrollsCountWhatTheyApplied(t *testing.T) {
	h := &fakeHost{}
	m := newMounter(h)
	m.mount(&driver.MountBatch{Root: 1, Items: []driver.MountItem{
		{Kind: driver.ItemCreate, ID: 1, View: driver.KindScroll},
		{Kind: driver.ItemCreate, ID: 2, View: driver.KindView},
		{Kind: driver.ItemInsert, ID: 2, Parent: 1},
		{Kind: driver.ItemEvents, ID: 1, Events: 1 << driver.EventScroll},
		{Kind: driver.ItemScrollTo, ID: 1, Point: driver.Point{Y: 100}},
		{Kind: driver.ItemStickyCells, ID: 1, Sticky: []driver.StickyCell{{Cell: 2, Start: 0, End: 1000}}},
	}})
	m.scrolled(1, driver.Point{Y: 40})
	if len(h.events) != 1 || h.events[0].Count != 1 || h.events[0].Point.Y != 40 {
		t.Fatalf("scroll = %+v", h.events)
	}
	if a := h.ops("a"); len(a) == 0 || !strings.Contains(a[len(a)-1], "translate(0px, 40px)") {
		t.Fatalf("sticky = %v", a)
	}
}

func TestDragsPanOnceTheyActivate(t *testing.T) {
	m, h := mounted()
	m.mount(&driver.MountBatch{Items: []driver.MountItem{
		{Kind: driver.ItemEvents, ID: 2, Events: 1 << driver.EventPan},
	}})
	if e := h.ops("e"); e[len(e)-1] != "[2,24]" {
		t.Fatalf("events = %v", e)
	}
	f := func(x, y driver.Points) []finger { return []finger{{id: 7, page: driver.Point{X: x, Y: y}}} }
	m.touch(3, 1, driver.GestureBegan, f(50, 50), f(50, 50), 0)
	m.touch(2, 1, driver.GestureBegan, f(50, 50), f(50, 50), 0) // bubbled: dropped
	m.touch(3, 2, driver.GestureChanged, f(55, 50), f(55, 50), 10*time.Millisecond)
	if len(h.events) != 0 {
		t.Fatalf("panned before the slop: %+v", h.events)
	}
	m.touch(3, 3, driver.GestureChanged, f(70, 50), f(70, 50), 20*time.Millisecond)
	m.touch(3, 4, driver.GestureEnded, f(80, 50), nil, 30*time.Millisecond)
	if len(h.events) != 2 || h.events[0].Phase != driver.GestureBegan || h.events[1].Phase != driver.GestureEnded ||
		h.events[1].Point.X != 30 || h.events[1].Velocity.X <= 0 {
		t.Fatalf("pan = %+v", h.events)
	}
}

func TestModalsAreAnimatedInAndBackGoesThroughTheApp(t *testing.T) {
	m, h := mounted()
	m.mount(&driver.MountBatch{Items: []driver.MountItem{
		{Kind: driver.ItemCreate, ID: 4, View: driver.KindModal},
		{Kind: driver.ItemModal, ID: 4, Modal: &driver.ModalConfig{Animation: driver.ModalFade}},
		{Kind: driver.ItemInsert, ID: 4, Parent: 2},
		{Kind: driver.ItemHistory, Count: 1},
		{Kind: driver.ItemAlert, Alert: &driver.AlertConfig{Title: "Hi"}},
	}})
	if ka := h.ops("ka"); len(ka) != 1 || !strings.Contains(ka[0], `"opacity":0`) {
		t.Fatalf("presented = %v", ka)
	}
	if bk := h.ops("bk"); len(bk) != 1 || bk[0] != "[0,true]" {
		t.Fatalf("back = %v", bk)
	}
	if len(h.services) != 1 || h.services[0] != driver.ItemAlert {
		t.Fatalf("services = %v", h.services)
	}
}

func TestStringsAreJSON(t *testing.T) {
	for _, s := range []string{"a\"b\\c\n\t", "\x01", "你好  ", string([]byte{0xff})} {
		var back string
		if err := json.Unmarshal(appendString(nil, s), &back); err != nil {
			t.Fatalf("%q: %v", s, err)
		}
		if s != string([]byte{0xff}) && back != s {
			t.Fatalf("%q came back %q", s, back)
		}
	}
}
