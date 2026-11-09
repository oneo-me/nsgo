package core

import (
	"math"
	"reflect"
	"testing"
	"time"

	"github.com/oneo-me/nsgo/internal/driver"
)

func TestNativeSendsChangedPropsCommandsAndEvents(t *testing.T) {
	var ref Ref
	var zoom *Signal[float64]
	var events []string
	h := newHost(t, Size{Width: 300, Height: 400}, func(cx *Scope) Node {
		zoom = NewSignal(cx, 3.0)
		return Native{
			Component: "map",
			PropsFunc: func() map[string]any {
				p := map[string]any{"zoom": zoom.Get(), "city": "Oslo"}
				if zoom.Get() > 5 {
					delete(p, "city")
				}
				return p
			},
			Ref:     &ref,
			TestID:  "map",
			OnEvent: func(name string, data any) { events = append(events, name) },
		}
	})
	b := h.commit()
	var cfg *driver.NativeConfig
	for _, it := range b.Items {
		if it.Kind == driver.ItemNative {
			cfg = it.Native
		}
	}
	n := h.find("map")
	if n.kind != driver.KindNative || cfg == nil || cfg.Component != "map" ||
		!reflect.DeepEqual(cfg.Props, map[string]any{"zoom": 3.0, "city": "Oslo"}) {
		t.Fatalf("kind %v, config %+v", n.kind, cfg)
	}
	if k := kinds(h.commit()); k[driver.ItemNative] != 0 {
		t.Fatalf("resent unchanged props: %v", k)
	}
	zoom.Set(7)
	ref.Command("fly", []float64{1, 2})
	b = h.commit()
	var cmd *driver.MountItem
	cfg = nil
	for i, it := range b.Items {
		switch it.Kind {
		case driver.ItemNative:
			cfg = it.Native
		case driver.ItemNativeCommand:
			cmd = &b.Items[i]
		}
	}
	if cfg == nil || !reflect.DeepEqual(cfg.Props, map[string]any{"zoom": 7.0, "city": nil}) {
		t.Fatalf("changes %+v", cfg)
	}
	if cmd == nil || cmd.Value != "fly" || !reflect.DeepEqual(cmd.Data, []float64{1, 2}) {
		t.Fatalf("command %+v", cmd)
	}

	h.tree.Dispatch(driver.Event{Kind: driver.EventNative, ID: n.id, Text: "moved"})
	h.tree.Dispatch(driver.Event{Kind: driver.EventNativeSize, ID: n.id, Size: Size{Width: 120, Height: 80}})
	h.commit()
	if len(events) != 1 || events[0] != "moved" {
		t.Fatalf("events %v", events)
	}
	// Stretched to the column's width, it keeps the ratio it asked for.
	if n.frame.Size != (Size{Width: 300, Height: 200}) {
		t.Fatalf("frame %v, want the ratio the component asked for", n.frame)
	}
}

func TestInputAccessoryViewIsADetachedRootNamedByInputs(t *testing.T) {
	h := newHost(t, Size{Width: 320, Height: 600}, func(cx *Scope) Node {
		return View{Children: []Node{
			Input{InputAccessoryViewID: "bar", TestID: "field"},
			InputAccessoryView{ID: "bar", TestID: "bar", Children: []Node{
				View{Style: Style{Height: Pt(44)}},
			}},
		}}
	})
	b := h.commit()
	bar := h.find("bar")
	var accessory NodeID
	created, inserted := -1, false
	for i, it := range b.Items {
		switch {
		case it.Kind == driver.ItemCreate && it.ID == bar.id:
			created = i
		case it.Kind == driver.ItemInsert && it.ID == bar.id:
			inserted = true
		case it.Kind == driver.ItemInput:
			if created < 0 {
				t.Fatal("the input named its accessory before it was created")
			}
			accessory = it.Input.Accessory
		}
	}
	if accessory != bar.id || inserted {
		t.Fatalf("accessory %v (want %v), inserted %v", accessory, bar.id, inserted)
	}
	if bar.frame != (Bounds{Size: Size{Width: 320, Height: 44}}) {
		t.Fatalf("frame %v", bar.frame)
	}
	h.tree.Resize(Size{Width: 400, Height: 600})
	h.commit()
	if bar.frame.Size.Width != 400 {
		t.Fatalf("frame %v after a resize", bar.frame)
	}
}

func TestFileDropReachesHandlers(t *testing.T) {
	var got []string
	entered, left := 0, 0
	h := newHost(t, Size{Width: 300, Height: 300}, func(cx *Scope) Node {
		return View{TestID: "drop", OnFileDrop: func(uris []string) { got = uris },
			OnFileDragEnter: func() { entered++ }, OnFileDragLeave: func() { left++ }}
	})
	b := h.commit()
	n := h.find("drop")
	for _, it := range b.Items {
		if it.Kind == driver.ItemEvents && it.ID == n.id {
			if !it.Events.Has(driver.EventFileDrop) || !it.Events.Has(driver.EventFileDragEnter) || !it.Events.Has(driver.EventFileDragLeave) {
				t.Fatalf("mask %b", it.Events)
			}
		}
	}
	h.tree.Dispatch(driver.Event{Kind: driver.EventFileDragEnter, ID: n.id})
	h.tree.Dispatch(driver.Event{Kind: driver.EventFileDragLeave, ID: n.id})
	h.tree.Dispatch(driver.Event{Kind: driver.EventFileDrop, ID: n.id, URIs: []string{"file:///tmp/a%20b.txt"}})
	if entered != 1 || left != 1 || len(got) != 1 || got[0] != "file:///tmp/a%20b.txt" {
		t.Fatalf("enter %d leave %d drop %v", entered, left, got)
	}
}

func TestAnimatedValuesBindAndRunOnAnAnimator(t *testing.T) {
	var fade *AnimatedValue
	var finished []bool
	var cx0 *Scope
	h := newHost(t, Size{Width: 300, Height: 300}, func(cx *Scope) Node {
		cx0 = cx
		fade = NewAnimatedValue(cx, 0)
		slide := Interpolate(fade, Interpolation{Input: []float64{0, 1}, Output: []float64{100, 0}})
		color := Interpolate(fade, Interpolation{Input: []float64{0, 1}, Colors: []Color{RGB(0x000000), RGB(0xffffff)}})
		return View{TestID: "box", Style: Style{Width: Pt(10), Height: Pt(10),
			Animated: &AnimatedStyle{Opacity: fade, TranslateY: slide, Background: color}}}
	})
	var reports []driver.Event
	a := driver.NewAnimator(func(e driver.Event) { reports = append(reports, e) })
	feed := func(b *driver.MountBatch) {
		for _, it := range b.Items {
			if it.Kind == driver.ItemAnimation {
				a.Apply(it.Animation)
			}
		}
	}
	feed(h.commit())
	box := h.find("box").id
	vals, ok := a.Values(box)
	if !ok || vals.Opacity != 0 || vals.TranslateY != 100 || vals.Background != RGB(0) {
		t.Fatalf("values %+v %v", vals, ok)
	}

	cx0.StartAnimation(Timing(fade, 1, TimingConfig{Duration: time.Second}), func(f bool) { finished = append(finished, f) })
	feed(h.commit())
	a.Step(10 * time.Second) // the first step starts it
	a.Step(10*time.Second + 500*time.Millisecond)
	vals, _ = a.Values(box)
	if math.Abs(float64(vals.Opacity)-0.5) > 1e-3 || math.Abs(float64(vals.TranslateY)-50) > 0.1 {
		t.Fatalf("halfway %+v", vals)
	}
	changed := 0
	a.Changed(func(id NodeID, v driver.AnimatedValues) {
		if id == box {
			changed++
		}
	})
	if changed != 1 {
		t.Fatalf("changed %d", changed)
	}
	a.Step(11 * time.Second)
	if a.Running() || len(reports) != 1 || !reports[0].Finished {
		t.Fatalf("running %v reports %+v", a.Running(), reports)
	}
	for _, e := range reports {
		h.tree.Dispatch(e)
	}
	if len(finished) != 1 || !finished[0] {
		t.Fatalf("finished %v", finished)
	}
	vals, _ = a.Values(box)
	if vals.Opacity != 1 || vals.Background != RGB(0xffffff) {
		t.Fatalf("end %+v", vals)
	}
}

func TestLayoutTransitionMovesTheFrameItsStyleChanges(t *testing.T) {
	var full *Signal[bool]
	h := newHost(t, Size{Width: 300, Height: 300}, func(cx *Scope) Node {
		full = NewSignal(cx, false)
		return View{Children: []Node{View{TestID: "bar", StyleFunc: func() Style {
			s := Style{Width: Pct(20), Height: Pt(10),
				Transition: Transition{Duration: 200 * time.Millisecond, Easing: EaseOut, Properties: TransitionLayout}}
			if full.Get() {
				s.Width = Pct(100)
			}
			return s
		}}}}
	})
	bar := h.find("bar").id
	frame := func(b *driver.MountBatch) *driver.MountItem {
		for i := range b.Items {
			if it := &b.Items[i]; it.Kind == driver.ItemFrame && it.ID == bar {
				return it
			}
		}
		t.Fatal("no frame for the bar")
		return nil
	}
	if it := frame(h.commit()); it.Transition != nil {
		t.Fatalf("a new view moved: %+v", it.Transition)
	}
	full.Set(true)
	it := frame(h.commit())
	want := driver.LayoutTransition{Curve: driver.LayoutBezier, Bezier: EaseOut, Duration: 200 * time.Millisecond}
	if it.Frame.Size.Width != 300 || it.Transition == nil || *it.Transition != want {
		t.Fatalf("frame %v transition %+v", it.Frame, it.Transition)
	}
	// The parent's resize alone moves it at once, as CSS's percentage
	// width does not transition when its container grows.
	h.tree.Resize(Size{Width: 400, Height: 300})
	if it := frame(h.commit()); it.Frame.Size.Width != 400 || it.Transition != nil {
		t.Fatalf("resized %v transition %+v", it.Frame, it.Transition)
	}
	full.Set(false)
	if it := frame(h.commit()); it.Transition == nil {
		t.Fatal("shrinking did not move")
	}
	h.tree.Resize(Size{Width: 300, Height: 300})
	if it := frame(h.commit()); it.Transition != nil {
		t.Fatalf("a later resize moved: %+v", it.Transition)
	}
}

func TestLayoutAnimationRidesTheNextBatch(t *testing.T) {
	var wide *Signal[bool]
	var cx0 *Scope
	h := newHost(t, Size{Width: 300, Height: 300}, func(cx *Scope) Node {
		cx0 = cx
		wide = NewSignal(cx, false)
		return View{TestID: "box", StyleFunc: func() Style {
			if wide.Get() {
				return Style{Width: Pt(200), Height: Pt(10)}
			}
			return Style{Width: Pt(100), Height: Pt(10)}
		}}
	})
	if b := h.commit(); b.LayoutAnimation != nil {
		t.Fatal("animated without asking")
	}
	cx0.AnimateLayout(LayoutSpringPreset)
	wide.Set(true)
	b := h.commit()
	if b.LayoutAnimation == nil || b.LayoutAnimation.Update.Curve != driver.LayoutSpring {
		t.Fatalf("layout animation %+v", b.LayoutAnimation)
	}
	wide.Set(false)
	if b := h.commit(); b.LayoutAnimation != nil {
		t.Fatal("the animation applied twice")
	}
	tr := LayoutEaseInEaseOutPreset.Update
	if p, done := tr.At(150 * time.Millisecond); math.Abs(p-0.5) > 1e-3 || done {
		t.Fatalf("ease at half: %v %v", p, done)
	}
	over := false
	for i := range 70 {
		if p, _ := LayoutSpringPreset.Update.At(time.Duration(i) * 10 * time.Millisecond); p > 1 {
			over = true
		}
	}
	if p, done := LayoutSpringPreset.Update.At(700 * time.Millisecond); !over || p != 1 || !done {
		t.Fatalf("spring: overshot %v, end %v %v", over, p, done)
	}
}

func TestProjectDecayIsWhereDecayStops(t *testing.T) {
	for _, c := range []DecayConfig{{Velocity: 1200}, {Velocity: -800, Deceleration: 0.99},
		{Velocity: 3000, Clamp: true, Min: 0, Max: 400}} {
		samples := SampleDecay(50, c)
		end := samples[len(samples)-1]
		if got := ProjectDecay(50, c); math.Abs(got-end) > 1 {
			t.Errorf("%+v: projected %v, decay stops at %v", c, got, end)
		}
	}
}

func TestRubberBandClamp(t *testing.T) {
	if got := RubberBandClamp(50, 0, 100, 300); got != 50 {
		t.Errorf("inside: %v", got)
	}
	past := RubberBandClamp(160, 0, 100, 300)
	if past <= 100 || past >= 160 {
		t.Errorf("past the end: %v", past)
	}
	if far := RubberBandClamp(1e9, 0, 100, 300); far >= 400 || far <= past {
		t.Errorf("far past: %v", far)
	}
	if before := RubberBandClamp(-60, 0, 100, 300); math.Abs(before+past-100) > 1e-9 {
		t.Errorf("before the start: %v, want %v", before, -(past - 100))
	}
}

func TestMatchedViewsCountAsOne(t *testing.T) {
	var open *Signal[bool]
	var cx0 *Scope
	h := newHost(t, Size{Width: 300, Height: 300}, func(cx *Scope) Node {
		cx0 = cx
		open = NewSignal(cx, false)
		return Show{When: open.Get,
			Then: func(cx *Scope) Node {
				return View{TestID: "page", Children: []Node{
					View{TestID: "hero", MatchID: "photo", Style: Style{Width: Pt(300), Height: Pt(200)}},
				}}
			},
			Else: func(cx *Scope) Node {
				return View{TestID: "thumb", MatchID: "photo", Style: Style{Width: Pt(40), Height: Pt(40)}}
			},
		}
	})
	h.commit()
	thumb := h.find("thumb").id
	matches := func(b *driver.MountBatch) (out []driver.MountItem) {
		deleted := false
		for _, it := range b.Items {
			switch it.Kind {
			case driver.ItemDelete:
				deleted = true
			case driver.ItemMatch:
				if deleted {
					t.Fatal("a match came after a delete")
				}
				out = append(out, it)
			}
		}
		return out
	}
	// Without a layout animation the views come and go as any.
	open.Set(true)
	if m := matches(h.commit()); len(m) != 0 {
		t.Fatalf("matched without a layout animation: %v", m)
	}
	open.Set(false)
	h.commit()
	thumb = h.find("thumb").id

	cx0.AnimateLayout(LayoutEaseInEaseOutPreset)
	open.Set(true)
	m := matches(h.commit())
	if len(m) != 1 || m[0].ID != h.find("hero").id || m[0].Before != thumb {
		t.Fatalf("matches %+v, want the hero taking thumb %d's place", m, thumb)
	}
}
