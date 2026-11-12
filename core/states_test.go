package core

import (
	"math"
	"testing"

	"github.com/nsgo-dev/nsgo/internal/driver"
)

func press(PressEvent) {}

func TestFocusWithinFollowsTheFocusIntoOverlaysAndOut(t *testing.T) {
	var gone *Signal[bool]
	h := newHost(t, Size{Width: 390, Height: 844}, func(cx *Scope) Node {
		gone = NewSignal(cx, false)
		return View{Children: []Node{
			View{TestID: "frame", Class: "focus-within:bg-[#3366ff]", Style: Style{Background: RGB(0xcccccc),
				FocusWithin: &Style{Opacity: Alpha(1)}, Opacity: Alpha(0.5)}, Children: []Node{
				View{TestID: "field", OnPress: press},
			}},
			View{TestID: "holder", Class: "group/box focus-within:bg-[#00ff00]", Children: []Node{
				View{TestID: "mark", Class: "group-focus-within/box:bg-[#111111]"},
				Overlay{Children: []Node{View{TestID: "popup", OnPress: press}}},
				Show{When: func() bool { return !gone.Get() }, Then: func(*Scope) Node { return View{TestID: "brief", OnPress: press} }},
			}},
		}}
	})
	h.commit()
	focus := func(id string, on bool) {
		kind := driver.EventBlur
		if on {
			kind = driver.EventFocus
		}
		h.tree.Dispatch(driver.Event{Kind: kind, ID: h.find(id).id})
		h.commit()
	}
	frame, holder, mark := func() *node { return h.find("frame") }, func() *node { return h.find("holder") }, func() *node { return h.find("mark") }

	if !h.find("field").events().Has(driver.EventFocus) {
		t.Fatal("a pressable view with no focus style does not report its focus")
	}
	focus("field", true)
	if frame().style.Background != RGB(0x3366ff) || frame().style.Opacity.Value() != 1 {
		t.Errorf("focused inside: %x, opacity %v", frame().style.Background, frame().style.Opacity.Value())
	}
	// The new view's focus comes before the old one's blur, and the blur
	// changes nothing.
	focus("popup", true)
	focus("field", false)
	if frame().style.Background != RGB(0xcccccc) {
		t.Errorf("the frame keeps the focus: %x", frame().style.Background)
	}
	if holder().style.Background != RGB(0x00ff00) || mark().style.Background != RGB(0x111111) {
		t.Errorf("an overlay's focus is not within its holder: %x, %x", holder().style.Background, mark().style.Background)
	}
	focus("popup", false)
	if holder().style.Background != 0 {
		t.Errorf("the focus left, the holder stays %x", holder().style.Background)
	}
	focus("brief", true)
	if holder().style.Background != RGB(0x00ff00) {
		t.Errorf("focused: %x", holder().style.Background)
	}
	gone.Set(true)
	h.commit()
	if holder().style.Background != 0 || h.tree.focus != nil {
		t.Errorf("the focus went with its view: %x, %v", holder().style.Background, h.tree.focus)
	}
}

func TestPointerEventsInheritNone(t *testing.T) {
	h := newHost(t, Size{Width: 390, Height: 844}, func(cx *Scope) Node {
		return View{Children: []Node{
			View{TestID: "blocked", Class: "pointer-events-none", Children: []Node{
				View{TestID: "below", Children: []Node{View{TestID: "deep"}}},
			}},
			View{TestID: "around", Class: "pointer-events-none", Children: []Node{
				View{TestID: "off"},
				View{TestID: "on", Class: "pointer-events-auto", Children: []Node{View{TestID: "in"}}},
			}},
			View{TestID: "box", Style: Style{PointerEvents: PointerEventsBoxNone}, Children: []Node{View{TestID: "free"}}},
		}}
	})
	h.commit()
	for id, want := range map[string]PointerEvents{"blocked": PointerEventsNone, "below": PointerEventsAuto,
		"deep": PointerEventsAuto, "around": PointerEventsBoxNone, "off": PointerEventsNone, "on": PointerEventsAuto,
		"in": PointerEventsAuto, "box": PointerEventsBoxNone, "free": PointerEventsAuto} {
		if got := h.find(id).props.PointerEvents; got != want {
			t.Errorf("%s mounts %d, want %d", id, got, want)
		}
	}
}

func TestAriaAndDataVariants(t *testing.T) {
	var on, attr *Signal[bool]
	h := newHost(t, Size{Width: 390, Height: 844}, func(cx *Scope) Node {
		on, attr = NewSignal(cx, false), NewSignal(cx, false)
		data := func() PartStates {
			if attr.Get() {
				return PartStates{Data: []string{"range-start"}}
			}
			return PartStates{}
		}
		return View{Children: []Node{
			View{TestID: "toggle", Class: "group active:bg-[#111111] aria-pressed:bg-[#222222]", Pressed: on.Get,
				Style: Style{Pressed: &Style{BorderWidth: 2}}, Children: []Node{View{TestID: "icon", Class: "group-aria-pressed:bg-[#333333]"}}},
			View{TestID: "busy", Class: "aria-busy:bg-[#444444]", Busy: on.Get},
			View{TestID: "day", Class: "group data-range-start:bg-[#555555] data-other:bg-[#666666]", PartStates: data,
				Children: []Node{View{TestID: "dot", Class: "group-data-range-start:bg-[#777777]"}}},
		}}
	})
	h.commit()
	bg := func(id string) Color { return h.find(id).style.Background }
	for _, id := range []string{"toggle", "icon", "busy", "day", "dot"} {
		if c := bg(id); c != 0 {
			t.Errorf("%s starts %x", id, c)
		}
	}
	on.Set(true)
	attr.Set(true)
	h.commit()
	want(t, "aria-pressed", bg("toggle"), RGB(0x222222))
	want(t, "pressed style", h.find("toggle").style.BorderWidth, 2)
	want(t, "group-aria-pressed", bg("icon"), RGB(0x333333))
	want(t, "aria-busy", bg("busy"), RGB(0x444444))
	want(t, "data-*", bg("day"), RGB(0x555555))
	want(t, "group-data-*", bg("dot"), RGB(0x777777))
	// An on toggle keeps its look under a press, aria-* being after active:.
	h.tree.Dispatch(driver.Event{Kind: driver.EventPressDown, ID: h.find("toggle").id})
	h.commit()
	want(t, "over active:", bg("toggle"), RGB(0x222222))
	if got := unknownOf(defaultTailwind, "data-[state=open]:p-1 aria-current:p-1 data-A:p-1"); len(got) != 3 {
		t.Errorf("unknown %v", got)
	}
}

func TestShowsExpandedReachesTheTrigger(t *testing.T) {
	var open *Signal[bool]
	h := newHost(t, Size{Width: 390, Height: 844}, func(cx *Scope) Node {
		open = NewSignal(cx, false)
		return View{ShowsExpanded: true, Expanded: open.Get, Children: []Node{
			View{TestID: "trigger", Class: "aria-expanded:bg-[#123456]"},
		}}
	})
	h.commit()
	want(t, "closed", h.find("trigger").style.Background, Color(0))
	open.Set(true)
	h.commit()
	want(t, "open", h.find("trigger").style.Background, RGB(0x123456))
	want(t, "the prop", h.find("trigger").props.Expanded, true)
}

func TestStyleKeysOverTextAndImageProps(t *testing.T) {
	var lines *Signal[int]
	h := newHost(t, Size{Width: 390, Height: 844}, func(cx *Scope) Node {
		lines = NewSignal(cx, 2)
		return View{Children: []Node{
			Text{TestID: "t", Content: "x", MaxLines: 5, EllipsizeMode: EllipsizeHead,
				StyleFunc: func() Style { return Style{NumberOfLines: lines.Get()} }},
			Text{TestID: "open", Content: "x", Class: "line-clamp-2 line-clamp-none"},
			Image{TestID: "img", ResizeMode: Contain, Style: Style{ObjectFit: Center}},
		}}
	})
	h.commit()
	limits := func() (int, EllipsizeMode) { return h.find("t").textLimits() }
	if l, m := limits(); l != 2 || m != EllipsizeHead {
		t.Errorf("lines %d, mode %d", l, m)
	}
	lines.Set(NoLineLimit)
	h.commit()
	if l, _ := limits(); l != 0 {
		t.Errorf("no limit: lines %d", l)
	}
	if l, _ := h.find("open").textLimits(); l != 0 {
		t.Errorf("line-clamp-none: lines %d", l)
	}
	want(t, "object fit", h.find("img").image.sent.ResizeMode, Center)
}

func TestTransformListsAndProperties(t *testing.T) {
	size := Size{Width: 40, Height: 80}
	close := func(t *testing.T, got, want driver.Matrix3D) {
		t.Helper()
		for i := range got {
			if math.Abs(got[i]-want[i]) > 1e-6 {
				t.Fatalf("got %v, want %v", got, want)
			}
		}
	}
	steps, err := ParseTransform("translate(10%, 20px) rotate(90deg)")
	if err != nil {
		t.Fatal(err)
	}
	// In the fields' order, it stays in the fields.
	s := Style{Transforms: steps}
	f := s.frameTransform(size)
	if f.TranslateX != 4 || f.TranslateY != 20 || f.Rotate != 90 || f.Then != (driver.Matrix3D{}) {
		t.Errorf("fields %+v", f)
	}
	// In another order, a matrix keeps it.
	steps, _ = ParseTransform("rotate(90deg) translateX(10px)")
	s = Style{Transforms: steps}
	f = s.frameTransform(size)
	if f.Then == (driver.Matrix3D{}) {
		t.Fatalf("no matrix: %+v", f)
	}
	close(t, f.Matrix(Point{}), driver.Matrix3D{0, 1, 0, 0, -1, 0, 0, 0, 0, 0, 1, 0, 0, 10, 0, 1})
	if p := f.InPlane(); math.Abs(float64(p.Rotate)-90) > 1e-3 || math.Abs(float64(p.TranslateY)-10) > 1e-3 {
		t.Errorf("in plane %+v", p)
	}
	for _, bad := range []string{"rotate", "wobble(1)", "scale(1, 2, 3)"} {
		if _, err := ParseTransform(bad); err == nil {
			t.Errorf("%q read", bad)
		}
	}
	if l, err := ParseTransform("none"); err != nil || l == nil || len(l) != 0 {
		t.Errorf("none: %v, %v", l, err)
	}
	// Each CSS property replaces its own.
	base := Style{Transform: Transform{TranslateX: 8, Rotate: 12, Scale: 0.9, RotateX: 45}}
	if m := base.Merge(Style{Transform: Transform{Scale: 2}}); m.Transform != (Transform{TranslateX: 8, Rotate: 12, Scale: 2, RotateX: 45}) {
		t.Errorf("scale over: %+v", m.Transform)
	}
	list := []TransformStep{{Kind: TransformRotate, Value: 5}}
	m := base.Merge(Style{Transforms: list})
	if m.Transform != (Transform{TranslateX: 8, Rotate: 12, Scale: 0.9}) || len(m.Transforms) != 1 {
		t.Errorf("list over: %+v %+v", m.Transform, m.Transforms)
	}
}

func TestPhysicalBordersKeepTheirSides(t *testing.T) {
	s := Style{Borders: Edges{Left: 1, Right: 2}, BorderColors: EdgeColors{Left: RGB(0x111111)},
		CornerRadii: Corners{TopLeft: 6}, PhysicalBorders: PhysicalBorders{Widths: SideLeft | SideRight,
			Colors: SideLeft, Corners: SideLeft}}
	v := s.viewStyle(Size{Width: 10, Height: 10}, RGB(0), true)
	if v.Borders.Left != 1 || v.Borders.Right != 2 || v.BorderColors.Left != RGB(0x111111) || v.Corners.TopLeft != 6 {
		t.Errorf("physical: %+v %+v %+v", v.Borders, v.BorderColors, v.Corners)
	}
	s.PhysicalBorders = PhysicalBorders{}
	v = s.viewStyle(Size{Width: 10, Height: 10}, RGB(0), true)
	if v.Borders.Left != 2 || v.Borders.Right != 1 || v.Corners.TopRight != 6 {
		t.Errorf("logical: %+v %+v", v.Borders, v.Corners)
	}
	// The left of a physical side wins over an end that lands there.
	if l, r := mirrored[Points](1, 2, SideLeft); l != 1 || r != 0 {
		t.Errorf("mirrored %v %v", l, r)
	}
}

func TestSidedAlignFollowsTheDirection(t *testing.T) {
	want(t, "left ltr", sidedAlign(TextAlignLeft, false), TextAlignStart)
	want(t, "left rtl", sidedAlign(TextAlignLeft, true), TextAlignEnd)
	want(t, "right ltr", sidedAlign(TextAlignRight, false), TextAlignEnd)
	want(t, "right rtl", sidedAlign(TextAlignRight, true), TextAlignStart)
	want(t, "center", sidedAlign(TextAlignCenter, true), TextAlignCenter)
}

func TestCurrentColorInAStyle(t *testing.T) {
	mix, err := ParseColor("color-mix(in srgb, currentColor 50%, #0000ff)")
	if err != nil {
		t.Fatal(err)
	}
	g, _ := ParseGradients("linear-gradient(currentColor, #0000ff)")
	s := Style{Background: mix, Gradients: g, Filters: []Filter{DropShadow(Point{}, 1, CurrentColor)}}
	v := s.viewStyle(Size{Width: 10, Height: 10}, RGB(0x00ff00), false)
	want(t, "mix", v.Background, RGB(0x008080))
	if c := v.Effects.Gradients[0].Stops[0].Color; c != RGB(0x00ff00) {
		t.Errorf("stop %x", c)
	}
	want(t, "drop shadow", v.Effects.Filters[0].Shadow.Color, RGB(0x00ff00))
	sh, _ := ParseBoxShadows("1px 2px")
	want(t, "a shadow without a color", sh[0].Color, Color(CurrentColor))
}
