package core

import (
	"fmt"
	"slices"
	"testing"
	"time"

	"github.com/nsgo-dev/nsgo/internal/driver"
)

// finger reports finger 0 at p in phase at time ms.
func finger(phase GesturePhase, p Point, ms int) driver.Event {
	return driver.Event{Kind: driver.EventTouch, Phase: phase, Time: time.Duration(ms) * time.Millisecond,
		Touches: []Touch{{ID: 0, PagePosition: p, Changed: true}}}
}

func TestTouchesReachTheViewsAroundTheFinger(t *testing.T) {
	var heard []string
	h := newHost(t, Size{Width: 200, Height: 200}, func(cx *Scope) Node {
		return View{TestID: "outer", Style: Style{Padding: All(20)},
			OnTouch: func(e TouchEvent) {
				heard = append(heard, fmt.Sprintf("outer %v %v", e.Phase, e.Touches[0].Position))
			},
			Children: []Node{View{TestID: "inner", Style: Style{Height: Pt(50)},
				OnTouch: func(e TouchEvent) {
					heard = append(heard, fmt.Sprintf("inner %v %v", e.Phase, e.Touches[0].Position))
				}}}}
	})
	h.commit()
	if !h.find("inner").events().Has(driver.EventTouch) {
		t.Fatal("a view with OnTouch does not report touches")
	}
	h.tree.Dispatch(finger(GestureBegan, Point{X: 30, Y: 40}, 0))
	h.tree.Dispatch(finger(GestureEnded, Point{X: 35, Y: 40}, 10))
	want := []string{"inner 1 {10 20}", "outer 1 {30 40}", "inner 3 {15 20}", "outer 3 {35 40}"}
	if !slices.Equal(heard, want) {
		t.Fatalf("heard %q, want %q", heard, want)
	}
	if !(driver.Event{Kind: driver.EventTouch, Phase: GestureChanged}).Mergeable() || (driver.Event{Kind: driver.EventTouch, Phase: GestureEnded}).Mergeable() {
		t.Fatal("only touch moves merge")
	}
}

func TestTheResponderIsNegotiatedAsReactNativeDoes(t *testing.T) {
	var log []string
	responder := func(name string, claims []TouchClaim) *Responder {
		say := func(what string) func(ResponderEvent) {
			return func(e ResponderEvent) { log = append(log, fmt.Sprintf("%s %s %v", name, what, e.Gesture.Delta)) }
		}
		return &Responder{Claims: claims, OnGrant: say("grant"), OnReject: say("reject"), OnMove: say("move"),
			OnRelease: say("release"), OnTerminate: say("terminate")}
	}
	h := newHost(t, Size{Width: 200, Height: 200}, func(cx *Scope) Node {
		// The list claims a move of more than 10 points before the row
		// inside it, which claims the touches as they start.
		far := func(e ResponderEvent) bool { return e.Gesture.Delta.Y > 10 }
		return View{TestID: "list", Responder: responder("list", []TouchClaim{{OnMove: true, Capture: true, Ask: far}}),
			Children: []Node{View{TestID: "row", Style: Style{Height: Pt(50), Active: &Style{Opacity: Alpha(0.5)}},
				Responder: responder("row", []TouchClaim{{}})}}}
	})
	h.commit()
	h.tree.Dispatch(finger(GestureBegan, Point{X: 20, Y: 20}, 0))
	if h.find("row").states&statePressed == 0 {
		t.Error("the responder is not held pressed")
	}
	h.tree.Dispatch(finger(GestureChanged, Point{X: 20, Y: 25}, 16))
	h.tree.Dispatch(finger(GestureChanged, Point{X: 20, Y: 40}, 32))
	h.tree.Dispatch(finger(GestureChanged, Point{X: 20, Y: 50}, 48))
	h.tree.Dispatch(finger(GestureEnded, Point{X: 20, Y: 50}, 64))
	want := []string{
		"row grant {0 0}", "row move {0 5}",
		"row terminate {0 20}", "list grant {0 0}", "list move {0 0}",
		"list move {0 10}", "list release {0 10}",
	}
	if !slices.Equal(log, want) {
		t.Fatalf("log %q\nwant %q", log, want)
	}
	b := h.commit()
	var blocks []NodeID
	for _, it := range b.Items {
		if it.Kind == driver.ItemBlockNative {
			blocks = append(blocks, it.ID)
		}
	}
	if !slices.Equal(blocks, []NodeID{h.find("row").id, h.find("list").id, 0}) {
		t.Fatalf("blocks %v", blocks)
	}
	if h.find("row").states&statePressed != 0 || h.find("list").states&statePressed != 0 {
		t.Error("a view stays pressed after the touches ended")
	}
}

func TestAResponderThatKeepsTheTouchesRejectsAClaim(t *testing.T) {
	var log []string
	say := func(what string) func(ResponderEvent) { return func(ResponderEvent) { log = append(log, what) } }
	h := newHost(t, Size{Width: 200, Height: 200}, func(cx *Scope) Node {
		return View{Responder: &Responder{Claims: []TouchClaim{{OnMove: true, Capture: true}}, OnGrant: say("outer grant"),
			OnReject: say("outer reject")},
			Children: []Node{View{Style: Style{Height: Pt(50)}, Responder: &Responder{Claims: []TouchClaim{{}}, Keep: true,
				OnGrant: say("inner grant"), OnTerminate: say("inner terminate")}}}}
	})
	h.commit()
	h.tree.Dispatch(finger(GestureBegan, Point{X: 20, Y: 20}, 0))
	h.tree.Dispatch(finger(GestureChanged, Point{X: 20, Y: 40}, 16))
	if want := []string{"inner grant", "outer reject"}; !slices.Equal(log, want) {
		t.Fatalf("log %q, want %q", log, want)
	}
}

func TestAViewWhosePanActivatesBecomesTheResponder(t *testing.T) {
	var log []string
	say := func(what string) func(ResponderEvent) { return func(ResponderEvent) { log = append(log, what) } }
	h := newHost(t, Size{Width: 200, Height: 200}, func(cx *Scope) Node {
		return View{Responder: &Responder{Claims: []TouchClaim{{}}, OnGrant: say("outer grant"), OnTerminate: say("outer terminate")},
			Children: []Node{View{TestID: "slider", Style: Style{Height: Pt(50)},
				PanActivation: PanActivation{ActiveOffsetX: 10, FailOffsetY: 10},
				Responder: &Responder{Claims: []TouchClaim{{OnPan: true}, {}}, OnGrant: say("slider grant"),
					OnRelease: say("slider release")}}}}
	})
	h.commit()
	slider := h.find("slider")
	if !slider.events().Has(driver.EventPan) || slider.props.PanActivation.ActiveOffsetX != 10 {
		t.Fatal("a view claiming on its pan does not follow pans")
	}
	h.tree.Dispatch(finger(GestureBegan, Point{X: 20, Y: 20}, 0))
	h.tree.Dispatch(finger(GestureChanged, Point{X: 35, Y: 20}, 16))
	h.tree.Dispatch(driver.Event{Kind: driver.EventPan, ID: slider.id, Phase: GestureBegan, Point: Point{X: 15, Y: 0}})
	h.tree.Dispatch(finger(GestureEnded, Point{X: 40, Y: 20}, 32))
	want := []string{"outer grant", "outer terminate", "slider grant", "slider release"}
	if !slices.Equal(log, want) {
		t.Fatalf("log %q, want %q", log, want)
	}
}

func TestCaptureRunsFirstAndAResponderMayLetNativeDragsGo(t *testing.T) {
	var heard []string
	note := func(s string, stop bool) func(TouchEvent) {
		return func(e TouchEvent) {
			heard = append(heard, s)
			if stop {
				e.StopPropagation()
			}
		}
	}
	h := newHost(t, Size{Width: 200, Height: 200}, func(cx *Scope) Node {
		return View{OnTouchCapture: note("outer capture", false), OnTouch: note("outer", false),
			Children: []Node{View{TestID: "inner", Style: Style{Height: Pt(50)}, OnTouchCapture: note("inner capture", false),
				OnTouch:   note("inner", true),
				Responder: &Responder{Claims: []TouchClaim{{}}, BlockNative: func(ResponderEvent) bool { return false }}}}}
	})
	h.commit()
	h.tree.Dispatch(finger(GestureBegan, Point{X: 20, Y: 20}, 0))
	if want := []string{"outer capture", "inner capture", "inner"}; !slices.Equal(heard, want) {
		t.Errorf("heard %q, want %q", heard, want)
	}
	for _, it := range h.commit().Items {
		if it.Kind == driver.ItemBlockNative && it.ID != 0 {
			t.Error("a responder whose BlockNative says no blocked the scroll views")
		}
	}
}
