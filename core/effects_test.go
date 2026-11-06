package core

import (
	"testing"

	"github.com/oneo-me/nsgo/internal/driver"
)

// viewStyles returns the view styles a batch sends, by node.
func viewStyles(b *driver.MountBatch) map[NodeID]*driver.ViewStyle {
	out := map[NodeID]*driver.ViewStyle{}
	for _, it := range b.Items {
		if it.Kind == driver.ItemViewStyle {
			out[it.ID] = it.ViewStyle
		}
	}
	return out
}

func TestShapesResolve(t *testing.T) {
	red, blue := RGB(0xff0000), RGB(0x0000ff)
	s := Style{CornerRadii: Corners{TopLeft: 8, BottomRight: 4}, BorderCurve: BorderCurveContinuous,
		BorderWidth: 2, BorderColor: CurrentColor, BorderStyle: BorderDashed, OutlineWidth: 2, OutlineOffset: 1}
	vs := s.viewStyle(Size{Width: 100, Height: 50}, blue, false)
	if vs.Corners != (Corners{TopLeft: 8, BottomRight: 4}) || !vs.ContinuousCorners || vs.CornerRadius != 0 {
		t.Errorf("corners = %+v, continuous %v", vs.Corners, vs.ContinuousCorners)
	}
	if vs.BorderColor != blue || vs.BorderStyle != BorderDashed {
		t.Errorf("border = %x %v, want the text color dashed", vs.BorderColor, vs.BorderStyle)
	}
	if vs.Outline != (Outline{Width: 2, Offset: 1, Color: blue}) {
		t.Errorf("outline = %+v, want one in the text color", vs.Outline)
	}
	// Right to left, left and right swap, corners too.
	if rtl := s.viewStyle(Size{}, red, true); rtl.Corners != (Corners{TopRight: 8, BottomLeft: 4}) {
		t.Errorf("right to left corners = %+v", rtl.Corners)
	}
	// Uniform corners are one radius.
	uniform := Style{CornerRadii: AllCorners(6)}
	if u := uniform.viewStyle(Size{}, 0, false); u.CornerRadius != 6 || u.Corners != (Corners{}) {
		t.Errorf("uniform corners = %v %+v", u.CornerRadius, u.Corners)
	}
	// No border, in layout and paint.
	none := Style{BorderWidth: 4, BorderColor: red, BorderStyle: BorderNone}
	if vs := none.viewStyle(Size{}, 0, false); vs.BorderWidth != 0 || none.borders() != (Edges{}) {
		t.Errorf("border none draws %v, lays out %+v", vs.BorderWidth, none.borders())
	}
}

func TestTransformsResolveAgainstTheSize(t *testing.T) {
	s := Style{Transform: Transform{TranslateXPercent: 50, TranslateY: 3, TranslateYPercent: -10},
		TransformOrigin: ObjectPosition{X: PositionAxis{Offset: Pct(0)}, Y: PositionAxis{Offset: Pt(10)}},
		Perspective:     500}
	vs := s.viewStyle(Size{Width: 80, Height: 40}, 0, false)
	if tr := vs.Transform; tr.TranslateX != 40 || tr.TranslateY != -1 || tr.TranslateXPercent != 0 {
		t.Errorf("transform = %+v", tr)
	}
	if vs.Origin != (Point{X: -40, Y: -10}) || vs.ChildPerspective != 500 {
		t.Errorf("origin %+v, perspective %v", vs.Origin, vs.ChildPerspective)
	}
}

func TestEffectsKeepTheirPointerWhileEqual(t *testing.T) {
	width := (*Signal[Points])(nil)
	tint := (*Signal[Color])(nil)
	h := newHost(t, Size{Width: 200, Height: 200}, func(cx *Scope) Node {
		width = NewSignal[Points](cx, 100)
		tint = NewSignal(cx, RGB(0xff0000))
		return View{TestID: "box", StyleFunc: func() Style {
			return Style{Width: Pt(width.Get()), Height: Pt(50), Background: tint.Get(),
				Shadows:   []BoxShadow{{Blur: 8, Color: RGBA(0, 0.3)}, Ring(2, CurrentColor)},
				Gradients: []Gradient{LinearGradient(45, RGB(0xffffff), RGB(0x000000))}}
		}}
	})
	b := h.commit()
	n := h.find("box")
	vs := viewStyles(b)[n.id]
	if vs == nil || vs.Effects == nil || len(vs.Effects.Shadows) != 2 || len(vs.Effects.Gradients) != 1 {
		t.Fatalf("effects = %+v", vs)
	}
	if vs.Effects.Shadows[1].Color != RGB(0x000000) {
		t.Errorf("ring = %+v, want the text color", vs.Effects.Shadows[1])
	}
	// A change elsewhere sends the same effects again by pointer.
	tint.Set(RGB(0x00ff00))
	b = h.commit()
	again := viewStyles(b)[n.id]
	if again == nil || again.Effects != vs.Effects {
		t.Errorf("effects pointer changed with the background: %p, %p", again.Effects, vs.Effects)
	}
	// A size the gradient depends on sends new ones.
	width.Set(150)
	b = h.commit()
	if wider := viewStyles(b)[n.id]; wider == nil || wider.Effects == vs.Effects {
		t.Error("resized gradient was not sent")
	}
}

func TestStateBackgroundReplacesGradients(t *testing.T) {
	g := []Gradient{LinearGradient(180, RGB(0xffffff), RGB(0xdddddd))}
	s := Style{Gradients: g, Active: &Style{Background: RGB(0x333333)},
		Hover: &Style{Background: RGB(0x666666), Gradients: g}}
	if out := s.withStates(statePressed, driver.ViewProps{}, groupState{}); out.Gradients != nil {
		t.Errorf("pressed keeps gradients %v", out.Gradients)
	}
	if out := s.withStates(stateHovered, driver.ViewProps{}, groupState{}); len(out.Gradients) != 1 {
		t.Error("a hover with gradients of its own lost them")
	}
}

func TestGroupStylesFollowTheGroup(t *testing.T) {
	h := newHost(t, Size{Width: 200, Height: 200}, func(cx *Scope) Node {
		return View{TestID: "card", Style: Style{Group: "card"}, Children: []Node{
			View{TestID: "inner", Style: Style{Group: "row"}, Children: []Node{
				View{TestID: "icon", Style: Style{Opacity: Alpha(0.5),
					GroupStyle: &GroupStyle{Name: "card", Hover: &Style{Opacity: Alpha(1)}}}},
				View{TestID: "any", Style: Style{GroupStyle: &GroupStyle{Active: &Style{Background: RGB(0xff0000)}}}},
			}},
		}}
	})
	h.commit()
	card, inner, icon, anyGroup := h.find("card"), h.find("inner"), h.find("icon"), h.find("any")
	if m := card.events(); !m.Has(driver.EventMouseEnter) || !m.Has(driver.EventPressDown) {
		t.Fatalf("group mask = %b, want hover and press reported", m)
	}
	h.tree.Dispatch(driver.Event{Kind: driver.EventMouseEnter, ID: card.id})
	if icon.style.Opacity.Value() != 1 {
		t.Errorf("icon opacity = %v while its group is hovered", icon.style.Opacity.Value())
	}
	// An unnamed group style follows the nearest group, the row.
	h.tree.Dispatch(driver.Event{Kind: driver.EventPressDown, ID: card.id})
	if anyGroup.style.Background != 0 {
		t.Error("a press on the outer group styled the nearest one's member")
	}
	h.tree.Dispatch(driver.Event{Kind: driver.EventPressDown, ID: inner.id})
	if anyGroup.style.Background != RGB(0xff0000) {
		t.Errorf("background = %x while the nearest group is pressed", anyGroup.style.Background)
	}
	h.tree.Dispatch(driver.Event{Kind: driver.EventMouseLeave, ID: card.id})
	if icon.style.Opacity.Value() != 0.5 {
		t.Errorf("icon opacity = %v after the hover", icon.style.Opacity.Value())
	}
}

func TestZIndexOrdersNativeChildren(t *testing.T) {
	var raise *Signal[int]
	h := newHost(t, Size{Width: 100, Height: 100}, func(cx *Scope) Node {
		raise = NewSignal(cx, 0)
		return View{TestID: "parent", Children: []Node{
			View{TestID: "a", StyleFunc: func() Style { return Style{ZIndex: raise.Get()} }},
			View{TestID: "b"},
			View{TestID: "c", Style: Style{Position: Sticky}},
			View{TestID: "d"},
		}}
	})
	h.commit()
	order := func() []string {
		var ids []string
		for _, c := range h.find("parent").nativeChildren() {
			ids = append(ids, c.props.TestID)
		}
		return ids
	}
	if got := order(); got[3] != "c" {
		t.Errorf("order = %v, want the sticky view on top", got)
	}
	raise.Set(2)
	b := h.commit()
	if got := order(); got[3] != "a" {
		t.Errorf("order = %v, want a raised on top", got)
	}
	inserted := false
	for _, it := range b.Items {
		if it.Kind == driver.ItemInsert && it.ID == h.find("a").id {
			inserted = true
		}
	}
	if !inserted {
		t.Error("raising a sent no insert")
	}
}

func TestFixedViewsLeaveTheirScrollView(t *testing.T) {
	var fixed *Signal[bool]
	h := newHost(t, Size{Width: 300, Height: 400}, func(cx *Scope) Node {
		fixed = NewSignal(cx, true)
		return Scroll{TestID: "scroll", Style: Style{Height: Pt(200)}, Children: []Node{
			View{Style: Style{Height: Pt(1000)}, Children: []Node{
				View{TestID: "fab", StyleFunc: func() Style {
					s := Style{Width: Pt(40), Height: Pt(40), Right: Pt(16), Bottom: Pt(16)}
					if fixed.Get() {
						s.Position = Fixed
					}
					return s
				}},
			}},
		}}
	})
	b := h.commit()
	fab := h.find("fab")
	if fab.nativeParent() != h.tree.view {
		t.Fatalf("fixed view is in %v, want the root view", fab.nativeParent().props.TestID)
	}
	if f := frames(b)[fab.id]; f != (Bounds{Origin: Point{X: 244, Y: 344}, Size: Size{Width: 40, Height: 40}}) {
		t.Errorf("fixed frame = %+v", f)
	}
	fixed.Set(false)
	h.commit()
	if fab.hoist != nil || fab.nativeParent() == h.tree.view {
		t.Error("the view stayed in the root after it stopped being fixed")
	}
}

func TestStickyViewsPinInTheirScrollView(t *testing.T) {
	h := newHost(t, Size{Width: 300, Height: 400}, func(cx *Scope) Node {
		return Scroll{TestID: "scroll", Style: Style{Height: Pt(200)}, Children: []Node{
			View{Style: Style{Height: Pt(100)}},
			View{TestID: "section", Style: Style{Height: Pt(500)}, Children: []Node{
				View{Style: Style{Height: Pt(30)}},
				View{TestID: "header", Style: Style{Height: Pt(20), Position: Sticky, Top: Pt(10)}},
			}},
		}}
	})
	b := h.commit()
	header := h.find("header")
	var cells []driver.StickyCell
	for _, it := range b.Items {
		if it.Kind == driver.ItemStickyCells && it.ID == h.find("scroll").id {
			cells = it.Sticky
		}
	}
	// It starts at 130, sticks 10 below the top, and travels until its
	// end meets the section's at 600.
	want := driver.StickyCell{Cell: header.id, Start: 120, End: 120 + 450}
	if len(cells) != 1 || cells[0] != want {
		t.Errorf("cells = %+v, want %+v", cells, want)
	}
}

func TestTextInheritsWrappingSelectionAndLineHeights(t *testing.T) {
	h := newHost(t, Size{Width: 100, Height: 400}, func(cx *Scope) Node {
		return View{Style: Style{WhiteSpace: WhiteSpaceNoWrap, UserSelect: UserSelectText, LineHeightMultiple: 2,
			FontSize: 10}, Children: []Node{
			Text{TestID: "long", Content: "a long line that does not wrap at all"},
			Text{TestID: "off", Content: "x", Selectable: SelectableOff, Style: Style{FontSize: 20}},
			View{Style: Style{LineHeightEm: 1.5, FontSize: 20, WhiteSpace: WhiteSpaceNormal}, Children: []Node{
				Text{TestID: "em", Content: "y", Style: Style{FontSize: 40}},
			}},
		}}
	})
	h.commit()
	long, off, em := h.find("long"), h.find("off"), h.find("em")
	if !long.text.noWrap || long.props.Selectable != SelectableOn {
		t.Errorf("long: no wrap %v, selectable %v", long.text.noWrap, long.props.Selectable)
	}
	if long.frame.Size.Height > 20 {
		t.Errorf("unwrapped text is %v tall", long.frame.Size.Height)
	}
	if off.props.Selectable != SelectableOff || off.text.style.LineHeight != 40 {
		t.Errorf("off: selectable %v, line height %v (twice its own font)", off.props.Selectable, off.text.style.LineHeight)
	}
	// Em resolves where it is set: 1.5 of 20, whatever the text's font.
	if em.text.style.LineHeight != 30 || em.text.noWrap {
		t.Errorf("em: line height %v, no wrap %v", em.text.style.LineHeight, em.text.noWrap)
	}
}

func TestMinContentFitsTheLongestWord(t *testing.T) {
	h := newHost(t, Size{Width: 400, Height: 400}, func(cx *Scope) Node {
		return View{Style: Style{AlignItems: AlignStart}, Children: []Node{
			View{TestID: "box", Style: Style{Width: MinContent}, Children: []Node{
				Text{Content: "aa bbbbbb cc"},
			}},
			View{TestID: "max", Style: Style{Width: MaxContent}, Children: []Node{
				Text{Content: "aa bbbbbb cc"},
			}},
		}}
	})
	h.commit()
	box, maxBox := h.find("box"), h.find("max")
	word := h.tree.text.Prepare(&driver.AttributedText{Text: "bbbbbb", Style: new(textFields).textStyle()})
	want := ceil(word.Measure(Auto).Width)
	if box.frame.Size.Width != want {
		t.Errorf("min-content width = %v, want the longest word's %v", box.frame.Size.Width, want)
	}
	line := h.tree.text.Prepare(&driver.AttributedText{Text: "aa bbbbbb cc", Style: new(textFields).textStyle()})
	if w := ceil(line.Measure(Auto).Width); maxBox.frame.Size.Width != w {
		t.Errorf("max-content width = %v, want the whole line's %v", maxBox.frame.Size.Width, w)
	}
}

func TestDirectionsOverrideBySubtree(t *testing.T) {
	h := newHost(t, Size{Width: 200, Height: 200}, func(cx *Scope) Node {
		return View{Children: []Node{
			View{TestID: "rtl", Style: Style{LayoutDirection: DirectionRTL, Direction: Row}, Children: []Node{
				View{TestID: "first", Style: Style{Width: Pt(20), Height: Pt(20)}},
				Text{TestID: "inherit", Content: "abc"},
				Text{TestID: "auto", Content: "שלום abc", Style: Style{WritingDirection: WritingAuto}},
			}},
			Text{TestID: "ltr", Content: "abc"},
		}}
	})
	b := h.commit()
	if f := frames(b)[h.find("first").id]; f.Origin.X != 180 {
		t.Errorf("first child of a right-to-left row at %v, want the right edge", f.Origin.X)
	}
	if !h.find("inherit").text.style.RTL || h.find("ltr").text.style.RTL {
		t.Error("text direction does not follow the layout's")
	}
	if !h.find("auto").text.style.RTL {
		t.Error("auto direction ignores the first strong character")
	}
}

func TestVisibilityHidesWithoutLeavingLayout(t *testing.T) {
	h := newHost(t, Size{Width: 100, Height: 100}, func(cx *Scope) Node {
		return View{Children: []Node{
			View{TestID: "gone", Style: Style{Height: Pt(30), Visibility: VisibilityHidden}},
			View{TestID: "next", Style: Style{Height: Pt(30)}},
		}}
	})
	b := h.commit()
	if vs := viewStyles(b)[h.find("gone").id]; vs == nil || !vs.Hidden {
		t.Error("hidden view is shown")
	}
	if f := frames(b)[h.find("next").id]; f.Origin.Y != 30 {
		t.Errorf("next view at %v, want after the hidden one", f.Origin.Y)
	}
}

func TestAbsoluteViewsReachPastStaticParents(t *testing.T) {
	h := newHost(t, Size{Width: 200, Height: 200}, func(cx *Scope) Node {
		return View{Style: Style{Height: Pt(100), Padding: All(10)}, Children: []Node{
			View{TestID: "static", Style: Style{Position: Static, Top: Pt(30), Padding: All(5)}, Children: []Node{
				View{TestID: "abs", Style: Style{Position: Absolute, Right: Pt(0), Bottom: Pt(0), Width: Pt(20), Height: Pt(20)}},
			}},
		}}
	})
	b := h.commit()
	if f := frames(b)[h.find("static").id]; f.Origin != (Point{X: 10, Y: 10}) {
		t.Errorf("static view at %+v: its insets moved it", f.Origin)
	}
	// At the outer view's padding box's corner, 180, 80; the static view
	// is at 10, 10.
	if f := frames(b)[h.find("abs").id]; f.Origin != (Point{X: 170, Y: 70}) {
		t.Errorf("absolute view at %+v in its static parent", f.Origin)
	}
}
