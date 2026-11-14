package core

import "testing"

// exists reports whether a live node has testID.
func (h *host) exists(testID string) bool {
	for _, n := range h.tree.nodes {
		if n != nil && !n.deleted && n.props.TestID == testID {
			return true
		}
	}
	return false
}

func TestSuspenseShowsFallbackUntilTheFirstValue(t *testing.T) {
	fetch, next, _ := loads(t)
	var r *Resource[string]
	h := newHost(t, Size{Width: 200, Height: 200}, func(cx *Scope) Node {
		return Suspense{
			Content: func(cx *Scope) Node {
				r = NewResource[int](cx, nil, fetch)
				return Text{ContentFunc: func() string { v, _ := r.Get(); return v }, TestID: "content"}
			},
			Fallback: func(*Scope) Node { return Text{Content: "loading", TestID: "fallback"} },
		}
	})
	h.commit()
	l := next()
	// Content is mounted, so it reads, but hidden and out of layout.
	if !h.exists("fallback") || !h.find("content").hidden || h.find("content").style.Display != DisplayNone {
		t.Fatal("the first load did not show the fallback over hidden content")
	}
	h.end(l, loadReply{v: "ready"})
	h.commit()
	if h.exists("fallback") || h.find("content").hidden || h.find("content").text.content != "ready" {
		t.Fatal("the loaded value did not show the content")
	}
	// A refetch keeps the value shown.
	r.Refetch()
	again := next()
	h.commit()
	if h.exists("fallback") || h.find("content").hidden {
		t.Fatal("a refetch went back to the fallback")
	}
	h.end(again, loadReply{v: "fresh"})
}

func TestSuspenseWaitsForWhatContentReadsNow(t *testing.T) {
	fetch, next, _ := loads(t)
	var shown *Signal[bool]
	h := newHost(t, Size{Width: 200, Height: 200}, func(cx *Scope) Node {
		// Made outside Content, read inside it.
		r := NewResource[int](cx, nil, fetch)
		shown = NewSignal(cx, true)
		return Suspense{
			Content: func(cx *Scope) Node {
				return View{Children: []Node{
					Text{Content: "always", TestID: "always"},
					Show{When: shown.Get, Then: func(*Scope) Node {
						return Text{ContentFunc: func() string { v, _ := r.Get(); return v }}
					}},
				}}
			},
			Fallback: func(*Scope) Node { return Text{Content: "loading", TestID: "fallback"} },
		}
	})
	h.commit()
	next()
	if !h.exists("fallback") {
		t.Fatal("a resource read in content did not suspend it")
	}
	// The view that read it goes: nothing under Content waits for it any more.
	shown.Set(false)
	h.commit()
	if h.exists("fallback") || h.find("always").hidden {
		t.Fatal("the suspense waited for a resource nothing reads")
	}
}

func TestSuspenseHidesViewsContentAddsWhileWaiting(t *testing.T) {
	fetch, next, _ := loads(t)
	var more *Signal[bool]
	h := newHost(t, Size{Width: 200, Height: 200}, func(cx *Scope) Node {
		more = NewSignal(cx, false)
		return Suspense{Content: func(cx *Scope) Node {
			r := NewResource[int](cx, nil, fetch)
			return Group{
				Text{ContentFunc: func() string { v, _ := r.Get(); return v }},
				Show{When: more.Get, Then: func(*Scope) Node { return Text{Content: "late", TestID: "late"} }},
			}
		}}
	})
	h.commit()
	l := next()
	more.Set(true)
	h.commit()
	if !h.find("late").hidden {
		t.Fatal("a view content added while waiting showed")
	}
	h.end(l, loadReply{v: "ready"})
	h.commit()
	if h.find("late").hidden {
		t.Fatal("the view stayed hidden once the suspense was ready")
	}
}

func TestSuspenseInsideContentTakesWhatIsReadUnderIt(t *testing.T) {
	fetch, next, _ := loads(t)
	h := newHost(t, Size{Width: 200, Height: 200}, func(cx *Scope) Node {
		return Suspense{
			Content: func(cx *Scope) Node {
				return View{Children: []Node{
					Text{Content: "outer", TestID: "outer"},
					Suspense{
						Content: func(cx *Scope) Node {
							r := NewResource[int](cx, nil, fetch)
							return Text{ContentFunc: func() string { v, _ := r.Get(); return v }}
						},
						Fallback: func(*Scope) Node { return Text{Content: "inner loading", TestID: "inner"} },
					},
				}}
			},
			Fallback: func(*Scope) Node { return Text{Content: "outer loading", TestID: "outer-fallback"} },
		}
	})
	h.commit()
	next()
	if h.exists("outer-fallback") || h.find("outer").hidden || !h.exists("inner") {
		t.Fatal("the outer suspense waited for what the inner one holds")
	}
}

func TestSuspenseHidesTheViewsContentHoists(t *testing.T) {
	fetch, next, _ := loads(t)
	var open *Signal[bool]
	h := newHost(t, Size{Width: 200, Height: 200}, func(cx *Scope) Node {
		open = NewSignal(cx, false)
		return Suspense{Content: func(cx *Scope) Node {
			r := NewResource[int](cx, nil, fetch)
			return Group{
				Text{ContentFunc: func() string { v, _ := r.Get(); return v }, TestID: "content"},
				Overlay{TestID: "overlay", Children: []Node{Text{Content: "over"}}},
				View{TestID: "fixed", Style: Style{Position: Fixed, Bottom: Pt(0)}, Children: []Node{Text{Content: "bar"}}},
				Show{When: open.Get, Then: func(*Scope) Node {
					return Modal{TestID: "modal", Children: []Node{Text{Content: "dialog"}}}
				}},
			}
		}}
	})
	h.commit()
	l := next()
	// Mounted with Content, before the suspense first waited.
	for _, id := range []string{"content", "overlay", "fixed"} {
		if !h.find(id).hidden {
			t.Fatalf("%s showed while the first value loaded", id)
		}
	}
	// Mounted while it waits.
	open.Set(true)
	h.commit()
	if !h.find("modal").hidden {
		t.Fatal("a modal content opened while waiting showed")
	}
	h.end(l, loadReply{v: "ready"})
	h.commit()
	for _, id := range []string{"content", "overlay", "fixed", "modal"} {
		if h.find(id).hidden {
			t.Fatalf("%s stayed hidden once the suspense was ready", id)
		}
	}
}
