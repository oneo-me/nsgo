package core

import (
	"slices"
	"testing"
	"time"

	"github.com/oneo-me/nsgo/internal/driver"
)

func TestSnapTargets(t *testing.T) {
	snap := func(b ScrollBehavior, offset, projected Points, v float32, viewport, content Points) Points {
		t.Helper()
		at, ok := b.SnapTarget(offset, projected, v, viewport, content)
		if !ok {
			t.Fatalf("%+v does not snap", b)
		}
		return at
	}
	// Pages of 300 in 1000 points of content (the last page is short):
	// one page per fling, wherever the fling would glide.
	paging := ScrollBehavior{Paging: true}
	for _, c := range []struct {
		offset, projected Points
		v                 float32
		want              Points
	}{{100, 100, 0, 0}, {100, 900, 1, 300}, {650, 900, 1, 700}, {250, 0, -1, 0}} {
		if got := snap(paging, c.offset, c.projected, c.v, 300, 1000); got != c.want {
			t.Errorf("paging from %v: %v, want %v", c.offset, got, c.want)
		}
	}
	// Items of 120 centered in a 300 viewport: k * 120 - 90.
	interval := ScrollBehavior{SnapToInterval: 120, SnapToAlignment: SnapCenter}
	if got := snap(interval, 200, 200, 0, 300, 2000); got != 150 {
		t.Errorf("settled at %v, want 150", got)
	}
	// A fast fling glides past several items, rounded forward.
	if got := snap(interval, 200, 700, 2, 300, 2000); got != 750 {
		t.Errorf("fling to %v, want 750", got)
	}
	one := interval
	one.DisableIntervalMomentum = true
	if got := snap(one, 200, 700, 2, 300, 2000); got != 270 {
		t.Errorf("one step to %v, want 270", got)
	}
	offsets := ScrollBehavior{SnapToOffsets: []Points{900, 0, 500}}
	if got := snap(offsets, 520, 600, 2, 300, 2000); got != 900 {
		t.Errorf("offsets fling to %v, want 900", got)
	}
	if got := snap(offsets, 520, 520, 0, 300, 2000); got != 500 {
		t.Errorf("offsets settle at %v, want 500", got)
	}
	if _, ok := (&ScrollBehavior{}).SnapTarget(10, 10, 1, 300, 2000); ok {
		t.Error("a free scroll view snapped")
	}
	if r := (&ScrollBehavior{}).Deceleration(); r != driver.DecelerationNormal {
		t.Errorf("rate %v", r)
	}
	if r := paging.Deceleration(); r != DecelerationFast {
		t.Errorf("paging rate %v", r)
	}
	if p := driver.Projected(100, 1, driver.DecelerationNormal); p < 590 || p > 600 {
		t.Errorf("projected %v", p)
	}
}

func TestScrollConfigEventsAndCommands(t *testing.T) {
	var ref Ref
	var snapped *Signal[bool]
	var refreshing *Signal[bool]
	var zoom *Signal[float32]
	var events []string
	var last ScrollEvent
	record := func(name string) func(ScrollEvent) {
		return func(e ScrollEvent) { events = append(events, name); last = e }
	}
	h := newHost(t, Size{Width: 300, Height: 400}, func(cx *Scope) Node {
		snapped = NewSignal(cx, false)
		refreshing = NewSignal(cx, false)
		zoom = NewSignal(cx, float32(1))
		return Scroll{
			TestID: "scroll",
			Ref:    &ref,
			BehaviorFunc: func() ScrollBehavior {
				if snapped.Get() {
					return ScrollBehavior{SnapToOffsets: []Points{0, 300}, ContentInset: Edges{Top: 20}}
				}
				return ScrollBehavior{Paging: true}
			},
			ContentOffset:     Point{Y: 120},
			Refreshing:        refreshing.Get,
			OnRefresh:         func() { events = append(events, "refresh"); refreshing.Set(true) },
			OnScroll:          record("scroll"),
			OnScrollDragStart: record("drag"),
			OnMomentumEnd:     record("rest"),
			MaxZoomScale:      4,
			ZoomScaleFunc:     zoom.Get,
			Style:             Style{Height: Pt(200)},
			Children:          []Node{View{Style: Style{Height: Pt(1000)}}},
		}
	})
	b := h.commit()
	var cfg *driver.ScrollConfig
	var mask driver.EventMask
	var scrollTo []driver.MountItem
	for _, it := range b.Items {
		switch it.Kind {
		case driver.ItemScrollConfig:
			cfg = it.Scroll
		case driver.ItemEvents:
			if it.ID == h.find("scroll").id {
				mask = it.Events
			}
		case driver.ItemScrollTo:
			scrollTo = append(scrollTo, it)
		}
	}
	if cfg == nil || !cfg.Behavior.Paging || cfg.MinZoomScale != 1 || cfg.MaxZoomScale != 4 || cfg.ZoomScale != 1 ||
		!cfg.Refreshable || cfg.Refreshing || !cfg.Zooms() {
		t.Fatalf("config %+v", cfg)
	}
	if !mask.Has(driver.EventScroll) || !mask.Has(driver.EventScrollDragStart) || !mask.Has(driver.EventMomentumEnd) ||
		!mask.Has(driver.EventRefresh) || mask.Has(driver.EventScrollDragEnd) || mask.Has(driver.EventMomentumStart) {
		t.Fatalf("mask %b", mask)
	}
	if len(scrollTo) != 1 || scrollTo[0].Point != (Point{Y: 120}) || scrollTo[0].Animated {
		t.Fatalf("initial offset %+v", scrollTo)
	}
	if k := kinds(h.commit()); k[driver.ItemScrollConfig] != 0 {
		t.Fatalf("resent an unchanged config: %v", k)
	}

	snapped.Set(true)
	zoom.Set(9)
	b = h.commit()
	cfg = nil
	for _, it := range b.Items {
		if it.Kind == driver.ItemScrollConfig {
			cfg = it.Scroll
		}
	}
	if cfg == nil || cfg.Behavior.Paging || len(cfg.Behavior.SnapToOffsets) != 2 || cfg.Behavior.ContentInset.Top != 20 ||
		cfg.ZoomScale != 4 {
		t.Fatalf("changed config %+v", cfg)
	}

	id := h.find("scroll").id
	h.tree.Dispatch(driver.Event{Kind: driver.EventScrollDragStart, ID: id, Point: Point{Y: 130}})
	h.tree.Dispatch(driver.Event{Kind: driver.EventScroll, ID: id, Point: Point{X: 10, Y: 300}, Value: 2})
	h.tree.Dispatch(driver.Event{Kind: driver.EventMomentumEnd, ID: id, Point: Point{X: 10, Y: 300}})
	want := ScrollEvent{Offset: Point{X: 10, Y: 300}, ContentSize: Size{Width: 600, Height: 2000},
		Size: Size{Width: 300, Height: 200}, ZoomScale: 2}
	if len(events) != 3 || events[0] != "drag" || events[1] != "scroll" || events[2] != "rest" || last != want {
		t.Fatalf("events %v, last %+v, want %+v", events, last, want)
	}

	h.tree.Dispatch(driver.Event{Kind: driver.EventRefresh, ID: id, Point: Point{Y: -70}})
	b = h.commit()
	cfg = nil
	for _, it := range b.Items {
		if it.Kind == driver.ItemScrollConfig {
			cfg = it.Scroll
		}
	}
	if events[len(events)-1] != "refresh" || cfg == nil || !cfg.Refreshing {
		t.Fatalf("refresh: events %v, config %+v", events, cfg)
	}
	// A refresh the app does not take is sent back as not refreshing.
	refreshing.Set(false)
	h.commit()
	h.tree.Dispatch(driver.Event{Kind: driver.EventRefresh, ID: id})
	refreshing.Set(false)
	b = h.commit()
	if k := kinds(b); k[driver.ItemScrollConfig] != 1 {
		t.Fatalf("refresh not answered: %v", k)
	}

	ref.ZoomTo(10, &Point{X: 50, Y: 60}, true)
	ref.ZoomTo(0, nil, false)
	var zooms []*driver.ZoomRequest
	for _, it := range h.commit().Items {
		if it.Kind == driver.ItemZoomTo {
			zooms = append(zooms, it.Zoom)
			if it.Animated != (len(zooms) == 1) {
				t.Fatalf("zoom %d animated %v", len(zooms), it.Animated)
			}
		}
	}
	if len(zooms) != 2 || zooms[0].Scale != 4 || *zooms[0].Center != (Point{X: 50, Y: 60}) ||
		zooms[1].Scale != 1 || zooms[1].Center != nil {
		t.Fatalf("zooms %+v", zooms)
	}
}

func TestScrollEventsFollowTheThrottle(t *testing.T) {
	var got []Points
	h := newHost(t, Size{Width: 300, Height: 200}, func(cx *Scope) Node {
		return Scroll{TestID: "scroll", ScrollEventThrottle: 100 * time.Millisecond,
			OnScroll: func(e ScrollEvent) { got = append(got, e.Offset.Y) },
			Children: []Node{View{Style: Style{Height: Pt(2000)}}}}
	})
	h.commit()
	id := h.find("scroll").id
	scroll := func(y Points) { h.tree.Dispatch(driver.Event{Kind: driver.EventScroll, ID: id, Point: Point{Y: y}}) }
	scroll(10)
	h.clock.Advance(30 * time.Millisecond)
	scroll(20)
	scroll(30)
	if !slices.Equal(got, []Points{10}) {
		t.Fatalf("within the throttle: reported %v, want [10]", got)
	}
	h.clock.Advance(70 * time.Millisecond)
	h.drain()
	if !slices.Equal(got, []Points{10, 30}) {
		t.Fatalf("after the throttle: reported %v, want [10 30]", got)
	}
	h.clock.Advance(150 * time.Millisecond)
	scroll(40)
	if !slices.Equal(got, []Points{10, 30, 40}) {
		t.Fatalf("a scroll after a quiet interval: reported %v, want [10 30 40]", got)
	}
}
