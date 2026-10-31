package core

import (
	"slices"
	"testing"

	"github.com/oneo-me/nsgo/internal/driver"
)

// phone is an environment with a system header and a status bar 40
// points tall.
var phone = Environment{OS: OSIOS, SafeArea: Edges{Top: 40, Bottom: 20}, HeaderHeight: 44, LargeHeaderHeight: 96}

func newPhone(t *testing.T, env Environment, app func(cx *Scope) Node) *host {
	h := newHost(t, Size{Width: 300, Height: 600}, nil)
	h.tree.SetEnvironment(env)
	h.tree.Mount(app)
	return h
}

func screenItems(b *driver.MountBatch) map[NodeID]driver.ScreenConfig {
	out := map[NodeID]driver.ScreenConfig{}
	for _, it := range b.Items {
		if it.Kind == driver.ItemScreen {
			out[it.ID] = *it.Screen
		}
	}
	return out
}

func TestScreenSystemHeader(t *testing.T) {
	var routes *Signal[[]string]
	var insets Edges
	var popped []string
	h := newPhone(t, phone, func(cx *Scope) Node {
		routes = NewSignalFunc(cx, []string{"Home", "Detail"}, nil)
		return Stack{Style: Style{FlexGrow: 1}, Children: []Node{
			Each(routes.Get, func(r string) string { return r }, func(cx *Scope, r func() string, _ func() int) Node {
				name := r()
				s := Screen{Title: name, TestID: name, HeaderBackTitle: "Back to " + name,
					Header: View{TestID: "drawn-" + name},
					OnDismiss: func() {
						popped = append(popped, name)
						routes.Update(func(rs []string) []string { return rs[:len(rs)-1] })
					},
					Children: []Node{Func(func(cx *Scope) Node {
						if name == "Detail" {
							cx.Effect(func() { insets = Insets(cx) })
						}
						return View{TestID: "content-" + name, Style: Style{Height: Pt(10)}}
					})}}
				if name == "Detail" {
					s.HeaderBackTitle = ""
					s.HeaderRight = View{TestID: "action", Style: Style{Width: Pt(30), Height: Pt(20)}}
					s.HeaderStyle = Style{Background: RGB(0x0a141e), Color: RGB(0xffffff), TintColor: RGB(0x0000ff)}
				}
				return s
			}),
		}}
	})
	b := h.commit()
	for _, id := range []string{"drawn-Home", "drawn-Detail"} {
		for _, n := range h.tree.nodes {
			if n != nil && !n.deleted && n.props.TestID == id {
				t.Errorf("%s mounted: the system header replaces Header", id)
			}
		}
	}
	detail, action := h.find("Detail"), h.find("action")
	c, ok := screenItems(b)[detail.id]
	want := driver.ScreenConfig{Title: "Detail", CanGoBack: true, BackTitle: "Back to Home",
		Background: RGB(0x0a141e), TitleColor: RGB(0xffffff), TintColor: RGB(0x0000ff),
		Right: action.parent.id, HeaderHeight: 84}
	if !ok || c != want {
		t.Errorf("detail's header = %+v, want %+v", c, want)
	}
	if home := screenItems(b)[h.find("Home").id]; home.CanGoBack || home.HeaderHeight != 84 {
		t.Errorf("home's header = %+v", home)
	}
	if got := h.find("content-Detail").frame.Origin.Y; got != 84 {
		t.Errorf("content at y %v, want below the bar and the status bar", got)
	}
	if insets.Top != 0 || insets.Bottom != 20 {
		t.Errorf("insets = %v, want the bottom only", insets)
	}
	if action.frame.Size != (Size{Width: 30, Height: 20}) || slices.Contains(detail.nativeChildren(), action) {
		t.Errorf("header view = %v, want its own size outside the screen's children", action.frame)
	}
	for i, it := range b.Items {
		if it.Kind == driver.ItemScreen && it.ID == detail.id {
			if !slices.ContainsFunc(b.Items[:i], func(x driver.MountItem) bool { return x.Kind == driver.ItemCreate && x.ID == action.id }) {
				t.Error("the header view is created after the screen naming it")
			}
		}
	}

	// The back button of a screen not on top does nothing.
	h.tree.Dispatch(driver.Event{Kind: driver.EventBack, ID: h.find("Home").id})
	h.tree.Dispatch(driver.Event{Kind: driver.EventBack, ID: detail.id})
	h.commit()
	if !slices.Equal(popped, []string{"Detail"}) {
		t.Errorf("popped = %q, want only the top screen", popped)
	}
}

func TestScreenHeaderOptions(t *testing.T) {
	var insets Edges
	h := newPhone(t, phone, func(cx *Scope) Node {
		return View{Style: Style{FlexGrow: 1}, Children: []Node{
			View{Style: Style{Height: Pt(100)}},
			Stack{Style: Style{FlexGrow: 1}, Children: []Node{
				Screen{Title: "Large", TestID: "large", HeaderLargeTitle: true, Children: []Node{
					Scroll{TestID: "large-content", Style: Style{FlexGrow: 1}, Children: []Node{View{Style: Style{Height: Pt(900)}}}}}},
			}},
			Stack{Style: Style{Height: Pt(100)}, Children: []Node{
				Screen{TestID: "clear", HeaderTransparent: true, Children: []Node{Func(func(cx *Scope) Node {
					cx.Effect(func() { insets = Insets(cx) })
					return View{TestID: "clear-content"}
				})}},
			}},
			Stack{Style: Style{Height: Pt(100)}, Children: []Node{
				Screen{TestID: "bare", HeaderHidden: true, Children: []Node{View{TestID: "bare-content"}}},
			}},
		}}
	})
	b := h.commit()
	items := screenItems(b)
	// Below the status bar, a stack's header covers only its bar.
	if got := items[h.find("large").id]; !got.LargeTitle || got.HeaderHeight != 96 {
		t.Errorf("large = %+v", got)
	}
	// A large title lays the content under it, insetting the scroll
	// view at its top, which it collapses into.
	if got := h.find("large-content").frame.Origin.Y; got != 0 {
		t.Errorf("large title content at %v, want under the header", got)
	}
	var config *driver.ScrollConfig
	for _, it := range b.Items {
		if it.Kind == driver.ItemScrollConfig && it.ID == h.find("large-content").id {
			config = it.Scroll
		}
	}
	if config == nil || config.HeaderInset != 96 || config.Behavior.ContentInset.Top != 96 {
		t.Errorf("the scroll view under the large title has %+v", config)
	}
	if got := items[h.find("clear").id].HeaderHeight; got != 44 || h.find("clear-content").frame.Origin.Y != 0 {
		t.Errorf("transparent header %v, content at %v, want the content under it", got, h.find("clear-content").frame.Origin.Y)
	}
	if insets.Top != 44 {
		t.Errorf("insets under a transparent header = %v, want its height on top", insets)
	}
	if got := items[h.find("bare").id]; !got.HeaderHidden || got.HeaderHeight != 0 || h.find("bare-content").frame.Origin.Y != 0 {
		t.Errorf("hidden header = %+v", got)
	} // The platform's measure replaces the estimate.
	h.tree.Dispatch(driver.Event{Kind: driver.EventScreenHeader, ID: h.find("clear").id, Value: 50})
	// The clear screen is below the status bar: its header is its bar.
	b2 := h.commit()
	if got := screenItems(b2)[h.find("clear").id].HeaderHeight; got != 50 {
		t.Errorf("measured header %v, want 50", got)
	}
	if insets.Top != 50 {
		t.Errorf("insets after the measure %v", insets)
	}
}

func TestScreenHeaderWithoutNativeHeader(t *testing.T) {
	h := newHost(t, Size{Width: 300, Height: 600}, func(cx *Scope) Node {
		return Stack{Style: Style{FlexGrow: 1}, Children: []Node{
			Screen{Title: "Home", Header: View{TestID: "drawn", Style: Style{Height: Pt(50)}},
				HeaderRight: View{TestID: "action"}, Children: []Node{View{TestID: "content"}}},
		}}
	})
	b := h.commit()
	if kinds(b)[driver.ItemScreen] != 0 {
		t.Error("ItemScreen sent without header heights")
	}
	if got := h.find("content").frame.Origin.Y; got != 50 {
		t.Errorf("content at %v, want below the drawn header", got)
	}
	for _, n := range h.tree.nodes {
		if n != nil && n.props.TestID == "action" {
			t.Error("a header view mounted without a system header")
		}
	}
}

func statusItems(b *driver.MountBatch) (out []driver.StatusBarConfig) {
	for _, it := range b.Items {
		if it.Kind == driver.ItemStatusBar {
			out = append(out, *it.StatusBar)
		}
	}
	return out
}

func TestStatusBarLayers(t *testing.T) {
	var routes *Signal[[]string]
	var open, light *Signal[bool]
	var root *Scope
	h := newHost(t, Size{Width: 300, Height: 600}, func(cx *Scope) Node {
		root = cx
		routes = NewSignalFunc(cx, []string{"home"}, nil)
		open, light = NewSignal(cx, false), NewSignal(cx, true)
		return View{Style: Style{FlexGrow: 1}, Children: []Node{
			Stack{Style: Style{FlexGrow: 1}, Children: []Node{
				Each(routes.Get, func(r string) string { return r }, func(cx *Scope, r func() string, _ func() int) Node {
					s := Screen{TestID: r(), OnDismiss: func() { routes.Set([]string{"home"}) }}
					if r() == "photo" {
						s.StatusBar = StatusBarSetting{Hidden: ToggleOn}
					}
					return s
				}),
			}},
			Show{When: open.Get, Then: func(*Scope) Node {
				return Modal{StatusBarFunc: func() StatusBarSetting {
					if light.Get() {
						return StatusBarSetting{Foreground: StatusBarLight, Animated: ToggleOn}
					}
					return StatusBarSetting{Foreground: StatusBarDark}
				}}
			}},
		}}
	})
	if got := statusItems(h.commit()); len(got) != 0 {
		t.Fatalf("status bar = %v before anything set it", got)
	}
	steps := []struct {
		do   func()
		want []driver.StatusBarConfig
	}{
		{func() { root.SetStatusBar(StatusBarSetting{Foreground: StatusBarDark}) }, []driver.StatusBarConfig{{Style: StatusBarDark}}},
		{func() { routes.Set([]string{"home", "photo"}) }, []driver.StatusBarConfig{{Style: StatusBarDark, Hidden: true}}},
		{func() { open.Set(true) }, []driver.StatusBarConfig{{Style: StatusBarLight, Hidden: true, Animated: true}}},
		{func() { light.Set(false) }, []driver.StatusBarConfig{{Style: StatusBarDark, Hidden: true}}},
		{func() { light.Set(true); light.Set(false) }, nil},
		{func() { open.Set(false) }, nil},
		{func() { h.tree.Dispatch(driver.Event{Kind: driver.EventBack}) }, []driver.StatusBarConfig{{Style: StatusBarDark}}},
	}
	for i, s := range steps {
		s.do()
		if got := statusItems(h.commit()); !slices.Equal(got, s.want) {
			t.Errorf("step %d: status bar = %v, want %v", i, got, s.want)
		}
	}
}

func TestModalSheetSizeAndSwipe(t *testing.T) {
	var reasons []string
	var insets Edges
	h := newPhone(t, phone, func(cx *Scope) Node {
		return View{Style: Style{FlexGrow: 1}, Children: []Node{
			Modal{TestID: "under"},
			Modal{TestID: "sheet", Presentation: ModalPageSheet, IOS: IOSModalProps{SupportedOrientations: OrientationPortrait},
				OnRequestClose: func(r string) { reasons = append(reasons, r) },
				Children: []Node{Func(func(cx *Scope) Node {
					cx.Effect(func() { insets = Insets(cx) })
					return View{TestID: "body", Style: Style{FlexGrow: 1}}
				})}},
		}}
	})
	b := h.commit()
	sheet := h.find("sheet")
	for _, it := range b.Items {
		if it.Kind == driver.ItemModal && it.ID == sheet.id &&
			(it.Modal.Presentation != ModalPageSheet || it.Modal.Orientations != OrientationPortrait) {
			t.Errorf("modal config = %+v", *it.Modal)
		}
	}
	h.tree.Dispatch(driver.Event{Kind: driver.EventModalSize, ID: sheet.id, Size: Size{Width: 300, Height: 540}, Insets: Edges{Bottom: 20}})
	h.commit()
	if sheet.frame != (Bounds{Size: Size{Width: 300, Height: 540}}) || h.find("body").frame.Size.Height != 540 {
		t.Errorf("sheet = %v, want the size the platform reported", sheet.frame)
	}
	if insets != (Edges{Bottom: 20}) {
		t.Errorf("insets = %v, want the sheet's", insets)
	}
	// Swiped away, the sheet asks to close and stays hidden; back goes to
	// the modal under it.
	h.tree.Dispatch(driver.Event{Kind: driver.EventRequestClose, ID: sheet.id, Text: "swipe", Finished: true})
	h.commit()
	if !sheet.hidden || sheet.viewStyle().Hidden != true {
		t.Error("a swiped sheet is not hidden")
	}
	h.tree.Dispatch(driver.Event{Kind: driver.EventBack})
	if !slices.Equal(reasons, []string{"swipe"}) {
		t.Errorf("reasons = %q, want back to skip the swiped sheet", reasons)
	}
}

func TestAndroidModalStartsBelowTheStatusBar(t *testing.T) {
	var below, under Edges
	env := Environment{OS: OSAndroid, SafeArea: Edges{Top: 24, Bottom: 16}}
	h := newPhone(t, env, func(cx *Scope) Node {
		probe := func(dst *Edges) Node {
			return Func(func(cx *Scope) Node { cx.Effect(func() { *dst = Insets(cx) }); return nil })
		}
		return View{Children: []Node{
			Modal{TestID: "below", Children: []Node{probe(&below)}},
			Modal{TestID: "under", Android: AndroidModalProps{StatusBarTranslucent: true}, Children: []Node{probe(&under)}},
		}}
	})
	h.commit()
	if got := h.find("below").frame; got != (Bounds{Origin: Point{X: 0, Y: 24}, Size: Size{Width: 300, Height: 576}}) || below != (Edges{Bottom: 16}) {
		t.Errorf("modal = %v with insets %v, want below the status bar", got, below)
	}
	if got := h.find("under").frame; got != (Bounds{Size: Size{Width: 300, Height: 600}}) || under != env.SafeArea {
		t.Errorf("translucent modal = %v with insets %v, want under the status bar", got, under)
	}
}

func TestBackPreviewNamesTheScreenBackPops(t *testing.T) {
	var cx0 *Scope
	var open *Signal[bool]
	h := newPhone(t, phone, func(cx *Scope) Node {
		cx0 = cx
		open = NewSignal(cx, false)
		return View{Style: Style{FlexGrow: 1}, Children: []Node{
			Stack{Style: Style{FlexGrow: 1}, Children: []Node{
				Screen{TestID: "home"},
				Screen{TestID: "detail"},
			}},
			Show{When: open.Get, Then: func(*Scope) Node {
				return Overlay{OnCancel: func() {}, Children: []Node{View{}}}
			}},
		}}
	})
	h.commit()
	preview := func() NodeID {
		h.tree.Dispatch(driver.Event{Kind: driver.EventBackStarted})
		for _, it := range h.commit().Items {
			if it.Kind == driver.ItemBackPreview {
				return it.ID
			}
		}
		t.Fatal("no ItemBackPreview")
		return 0
	}
	if got := preview(); got != h.find("detail").id {
		t.Errorf("preview = %v, want the top screen", got)
	}
	if f := h.find("home").frame; f.Size.Width == 0 {
		t.Errorf("home's frame = %v while a back gesture may uncover it", f)
	}
	h.tree.Dispatch(driver.Event{Kind: driver.EventBackCancelled})
	h.commit()
	if f := h.find("home").frame; f.Size.Width != 0 {
		t.Errorf("home's frame = %v after the gesture was cancelled", f)
	}
	open.Set(true)
	if got := preview(); got != 0 {
		t.Errorf("preview = %v under an overlay hearing cancel, want none", got)
	}
	open.Set(false)
	cx0.OnBackPress(func() bool { return false })
	if got := preview(); got != 0 {
		t.Errorf("preview = %v with a back handler, want none", got)
	}
}

func TestScreenHeaderSystemItem(t *testing.T) {
	h := newPhone(t, phone, func(cx *Scope) Node {
		return Stack{Style: Style{FlexGrow: 1}, Children: []Node{
			Screen{Title: "Bare", TestID: "bare", IOS: IOSScreenProps{HeaderRightSystemItem: HeaderItemAdd}},
			Screen{Title: "Add", TestID: "add", IOS: IOSScreenProps{HeaderRightSystemItem: HeaderItemAdd},
				HeaderRight: View{TestID: "action", OnPress: func(PressEvent) {}}},
		}}
	})
	items := screenItems(h.commit())
	if c := items[h.find("add").id]; c.RightSystemItem != HeaderItemAdd || c.Right != h.find("action").parent.id {
		t.Errorf("with HeaderRight: item %v, right %v; want add over the action", c.RightSystemItem, c.Right)
	}
	if c := items[h.find("bare").id]; c.RightSystemItem != 0 {
		t.Errorf("without HeaderRight: item %v, want none, as nothing would take its press", c.RightSystemItem)
	}
}

func TestExitAppLeavesAsBackDoes(t *testing.T) {
	var scope *Scope
	h := newPhone(t, phone, func(cx *Scope) Node {
		scope = cx
		return View{}
	})
	h.commit()
	scope.ExitApp()
	if k := kinds(h.commit()); k[driver.ItemBackUnhandled] != 1 {
		t.Errorf("ExitApp sent %v, want one ItemBackUnhandled", k)
	}
}
