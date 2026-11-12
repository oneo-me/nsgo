package core

import (
	"slices"
	"testing"

	"github.com/nsgo-dev/nsgo/internal/driver"
)

func TestPresenceKeepsABranchUntilItsExitsEnd(t *testing.T) {
	var open *Signal[bool]
	var done []func()
	h := newHost(t, Size{Width: 100, Height: 200}, func(cx *Scope) Node {
		open = NewSignal(cx, true)
		return View{TestID: "box", Children: []Node{Presence{When: open.Get,
			Then: func(cx *Scope) Node {
				cx.OnExit(func(d func()) { done = append(done, d) })
				cx.OnExit(func(d func()) { done = append(done, d) })
				return Text{ContentFunc: func() string {
					if cx.Present() {
						return "sheet"
					}
					return "leaving"
				}}
			},
			Else: func(cx *Scope) Node { return Text{Content: "closed", TestID: "closed"} },
		}}}
	})
	h.commit()
	box := h.find("box")
	shown := func() (out []string) {
		for _, c := range box.sent.children {
			out = append(out, c.text.content)
		}
		return out
	}
	open.Set(false)
	h.commit()
	if got := shown(); !slices.Equal(got, []string{"leaving", "closed"}) {
		t.Fatalf("while the sheet leaves the box shows %q", got)
	}
	if len(done) != 2 {
		t.Fatalf("%d exits started, want 2", len(done))
	}
	done[0]()
	done[0]() // a second call counts once
	h.commit()
	if got := shown(); !slices.Equal(got, []string{"leaving", "closed"}) {
		t.Fatalf("with an exit playing the box shows %q", got)
	}
	done[1]()
	b := h.commit()
	if got := shown(); !slices.Equal(got, []string{"closed"}) {
		t.Fatalf("once the exits end the box shows %q", got)
	}
	if kinds(b)[driver.ItemDelete] != 1 {
		t.Errorf("the sheet's view was not deleted: %v", kinds(b))
	}

	// Without exits a branch goes at once, as Show's.
	open.Set(true)
	h.commit()
	if got := shown(); !slices.Equal(got, []string{"sheet"}) {
		t.Fatalf("closed left without exits, yet the box shows %q", got)
	}
}

func TestPresenceWaitsForTheLeavingBranch(t *testing.T) {
	var open *Signal[bool]
	var done func()
	builds := 0
	h := newHost(t, Size{Width: 100, Height: 200}, func(cx *Scope) Node {
		open = NewSignal(cx, true)
		return View{TestID: "box", Children: []Node{Presence{When: open.Get, Wait: true,
			Then: func(cx *Scope) Node {
				builds++
				cx.OnExit(func(d func()) { done = d })
				return Text{Content: "a", TestID: "a"}
			},
			Else: func(cx *Scope) Node { return Text{Content: "b"} },
		}}}
	})
	h.commit()
	box := h.find("box")
	shown := func() (out []string) {
		for _, c := range box.sent.children {
			out = append(out, c.text.content)
		}
		return out
	}
	open.Set(false)
	h.commit()
	open.Set(true)
	h.commit()
	if got := shown(); !slices.Equal(got, []string{"a"}) || builds != 1 {
		t.Fatalf("while a leaves the box shows %q after %d builds", got, builds)
	}
	done()
	h.commit()
	// What When says once a is gone is built: a again.
	if got := shown(); !slices.Equal(got, []string{"a"}) || builds != 2 {
		t.Fatalf("once a left the box shows %q after %d builds", got, builds)
	}
}

func TestMotionExitKeepsItsViewUntilItEnds(t *testing.T) {
	var open *Signal[bool]
	var m *Motion
	h := newHost(t, Size{Width: 100, Height: 200}, func(cx *Scope) Node {
		open = NewSignal(cx, true)
		return View{TestID: "box", Children: []Node{Presence{When: open.Get, Then: func(cx *Scope) Node {
			m = NewMotion(cx, nil)
			m.Exit(FadeOut.Spec())
			return View{TestID: "toast", Style: Style{Animated: m.Style()}}
		}}}}
	})
	h.commit()
	open.Set(false)
	b := h.commit()
	if !h.has("toast") {
		t.Fatal("the toast left before its exit")
	}
	for _, it := range b.Items {
		for _, op := range it.Animation {
			if op.Kind == driver.AnimStart {
				h.tree.Dispatch(driver.Event{Kind: driver.EventAnimation, Count: op.ID, Finished: true})
			}
		}
	}
	h.commit()
	if h.has("toast") {
		t.Fatal("the toast stayed after its exit")
	}
}
