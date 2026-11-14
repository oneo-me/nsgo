package core

import (
	"slices"
	"testing"

	"github.com/nsgo-dev/nsgo/internal/driver"
)

func TestGesturesReportAndMerge(t *testing.T) {
	var pans []PanEvent
	var scales []float32
	h := newHost(t, Size{Width: 100, Height: 100}, func(cx *Scope) Node {
		return View{TestID: "card", Style: Style{Width: Pt(50), Height: Pt(50)},
			OnPan:         func(e PanEvent) { pans = append(pans, e) },
			PanActivation: PanActivation{ActiveOffsetX: 20},
			OnPinch:       func(e GestureEvent) { scales = append(scales, e.Value) }}
	})
	b := h.commit()
	n := h.find("card")
	for _, it := range b.Items {
		if it.ID == n.id && it.Kind == driver.ItemEvents && (!it.Events.Has(driver.EventPan) || !it.Events.Has(driver.EventPinch) || it.Events.Has(driver.EventRotation)) {
			t.Fatalf("events = %b", it.Events)
		}
		if it.ID == n.id && it.Kind == driver.ItemViewProps && it.ViewProps.PanActivation.ActiveOffsetX != 20 {
			t.Fatalf("props = %+v", it.ViewProps)
		}
	}
	h.tree.Dispatch(driver.Event{Kind: driver.EventPan, ID: n.id, Phase: GestureBegan, Point: Point{X: 20, Y: 0}})
	h.tree.Dispatch(driver.Event{Kind: driver.EventPan, ID: n.id, Phase: GestureEnded, Point: Point{X: 30, Y: 5}, Velocity: Point{X: 100, Y: 0}})
	h.tree.Dispatch(driver.Event{Kind: driver.EventPinch, ID: n.id, Phase: GestureChanged, Value: 1.5})
	if len(pans) != 2 || pans[1].Translation != (Point{X: 30, Y: 5}) || pans[1].Velocity.X != 100 || pans[1].Phase != GestureEnded {
		t.Fatalf("pans = %+v", pans)
	}
	if !slices.Equal(scales, []float32{1.5}) {
		t.Fatalf("scales = %v", scales)
	}
	if !(driver.Event{Kind: driver.EventPan, Phase: GestureChanged}).Mergeable() || (driver.Event{Kind: driver.EventPan, Phase: GestureEnded}).Mergeable() {
		t.Fatal("only a gesture's moves merge")
	}
}

func TestPanActivationDecides(t *testing.T) {
	horizontal := PanActivation{ActiveOffsetX: 10, FailOffsetY: 5}
	for _, c := range []struct {
		a    PanActivation
		d    Point
		want int
	}{
		{PanActivation{}, Point{X: 3, Y: 4}, 0},
		{PanActivation{}, Point{X: 0, Y: -12}, 1},
		{horizontal, Point{X: -12, Y: 2}, 1},
		{horizontal, Point{X: 4, Y: 6}, -1},
		{horizontal, Point{X: 6, Y: 2}, 0},
	} {
		if got := c.a.Decide(c.d); got != c.want {
			t.Errorf("%+v.Decide(%v) = %d, want %d", c.a, c.d, got, c.want)
		}
	}
}

func TestWindowCommands(t *testing.T) {
	lights := WindowControls{TrafficLights: Point{X: 20, Y: 18}}
	h := newHost(t, Size{Width: 200, Height: 100}, func(cx *Scope) Node {
		cx.SetTitleBar(TitleBarHidden, 40)
		cx.SetWindowControls(lights)
		cx.SetWindowMaterial(WindowMaterialSidebar)
		return View{TestID: "bar", Desktop: DesktopViewProps{WindowDrag: WindowDragOn}}
	})
	b := h.commit()
	var cmds []WindowCommand
	for _, it := range b.Items {
		if it.Kind == driver.ItemWindow {
			cmds = append(cmds, *it.Window)
		}
	}
	if want := []WindowCommand{
		{Action: WindowSetTitleBar, TitleBar: TitleBarHidden, Height: 40},
		{Action: WindowSetControls, Controls: lights},
		{Action: WindowSetMaterial, Material: WindowMaterialSidebar},
	}; !slices.Equal(cmds, want) {
		t.Fatalf("window commands = %+v, want %+v", cmds, want)
	}
	if p := h.find("bar").props; p.WindowDrag != WindowDragOn {
		t.Fatalf("props = %+v", p)
	}
}

// TestReopenOrdersTheOneSurfaceFront: where the platform opens no windows of the app's, as on Windows and Linux, a reopen no handler hears brings the app's one window to the front, and one a handler hears leaves it to the handler.
func TestReopenOrdersTheOneSurfaceFront(t *testing.T) {
	var cx0 *Scope
	h := newHost(t, Size{Width: 200, Height: 100}, func(cx *Scope) Node {
		cx0 = cx
		return Text{Content: "main"}
	})
	h.commit()
	orders := func() int {
		n := 0
		for _, it := range h.commit().Items {
			if it.Kind == driver.ItemWindow && it.Window.Action == WindowOrderFront {
				n++
			}
		}
		return n
	}
	h.tree.Dispatch(driver.Event{Kind: driver.EventReopen})
	if n := orders(); n != 1 {
		t.Fatalf("a reopen without a handler ordered the window front %d times, want 1", n)
	}
	heard := 0
	cx0.OnReopen(func() { heard++ })
	h.tree.Dispatch(driver.Event{Kind: driver.EventReopen})
	if n := orders(); n != 0 || heard != 1 {
		t.Fatalf("a reopen with a handler: %d orders front, heard %d times", n, heard)
	}
}

func TestApplicationMenuFollowsItsOwner(t *testing.T) {
	var log []string
	var open *Signal[bool]
	h := newHost(t, Size{Width: 100, Height: 100}, func(cx *Scope) Node {
		open = NewSignal(cx, true)
		return Show{When: open.Get, Then: func(cx *Scope) Node {
			cx.SetApplicationMenu([]MenuItem{
				{Label: "App", Items: []MenuItem{{Role: MenuQuit}}},
				{Label: "File", Items: []MenuItem{
					{Label: "New", Shortcut: "Cmd+N", OnSelect: func() { log = append(log, "new") }},
					{Separator: true},
					{Label: "Wrap", Checked: true},
				}},
			})
			return View{}
		}}
	})
	var menu []MenuEntry
	for _, it := range h.commit().Items {
		if it.Kind == driver.ItemMenu {
			menu = it.Menu
		}
	}
	if len(menu) != 2 || menu[0].Items[0].Label != "Quit" || menu[0].Items[0].ID != 0 {
		t.Fatalf("menu = %+v", menu)
	}
	file := menu[1].Items
	if file[0].Shortcut != "meta+n" || file[0].ID == 0 || !file[1].Separator || !file[2].Checkable {
		t.Fatalf("file = %+v", file)
	}
	h.tree.Dispatch(driver.Event{Kind: driver.EventMenu, Count: file[0].ID})
	open.Set(false)
	restored := false
	for _, it := range h.commit().Items {
		restored = restored || it.Kind == driver.ItemMenu && it.Menu == nil
	}
	h.tree.Dispatch(driver.Event{Kind: driver.EventMenu, Count: file[0].ID})
	if !restored || !slices.Equal(log, []string{"new"}) {
		t.Fatalf("restored = %v, log = %q", restored, log)
	}
}

// TestApplicationMenuSetAgainKeepsTheBar sends a menu an effect sets
// again in place of the one before, without the default between them,
// which a menu bar would show for a moment.
func TestApplicationMenuSetAgainKeepsTheBar(t *testing.T) {
	var checked *Signal[bool]
	h := newHost(t, Size{Width: 100, Height: 100}, func(cx *Scope) Node {
		checked = NewSignal(cx, true)
		cx.Effect(func() {
			cx.SetApplicationMenu([]MenuItem{{Label: "View", Items: []MenuItem{{Label: "Sidebar", Checked: checked.Get()}}}})
		})
		return View{}
	})
	h.commit()
	checked.Set(false)
	var menus [][]MenuEntry
	for _, it := range h.commit().Items {
		if it.Kind == driver.ItemMenu {
			menus = append(menus, it.Menu)
		}
	}
	if len(menus) != 1 || menus[0] == nil || menus[0][0].Items[0].Checked {
		t.Fatalf("menus sent = %+v, want the new one alone", menus)
	}
}

func TestBordersPerSide(t *testing.T) {
	red, blue := RGB(0xff0000), RGB(0x0000ff)
	for _, c := range []struct {
		s    Style
		want driver.ViewStyle
	}{
		{Style{BorderWidth: 2, BorderColor: red}, driver.ViewStyle{BorderWidth: 2, BorderColor: red}},
		{Style{Borders: All(3), BorderColor: red}, driver.ViewStyle{BorderWidth: 3, BorderColor: red}},
		{Style{Borders: Edges{Bottom: 1}, BorderColor: red},
			driver.ViewStyle{Borders: Edges{Bottom: 1}, BorderColors: AllColors(red)}},
		{Style{BorderWidth: 2, BorderColor: red, BorderColors: EdgeColors{Left: blue}},
			driver.ViewStyle{Borders: All(2), BorderColors: EdgeColors{Top: red, Right: red, Bottom: red, Left: blue}}},
	} {
		got := c.s.viewStyle(Size{}, 0, false)
		got.Opacity = 0
		if got != c.want {
			t.Errorf("%+v: got %+v, want %+v", c.s, got, c.want)
		}
	}
}

func TestCursorStyleReachesTheView(t *testing.T) {
	s := Style{Cursor: CursorColResize}
	if got := s.viewStyle(Size{}, 0, false).Cursor; got != CursorColResize {
		t.Fatalf("view cursor = %v, want the style's", got)
	}
	if got := (Style{Cursor: CursorRowResize}).Merge(Style{Cursor: CursorPointer}).Cursor; got != CursorPointer {
		t.Fatalf("merged cursor = %v, want the later style's", got)
	}
	if got := CursorRowResize.Name(); got != "row-resize" {
		t.Fatalf("cursor name = %q, want its CSS name", got)
	}
}

func TestMenuWhenLeavesInputsTheirKeys(t *testing.T) {
	h := newHost(t, Size{Width: 100, Height: 100}, func(cx *Scope) Node {
		cx.SetApplicationMenu([]MenuItem{{Label: "Edit", Items: []MenuItem{
			{Label: "Find", Shortcut: "mod+f", When: MenuAuto},
			{Label: "Select line", Shortcut: "mod+a", When: MenuAuto},
			{Label: "Run", Shortcut: "mod+r", When: MenuNotEditing},
			{Label: "Save", Shortcut: "mod+s"},
		}}})
		return View{}
	})
	var menu []MenuEntry
	for _, it := range h.commit().Items {
		if it.Kind == driver.ItemMenu {
			menu = it.Menu
		}
	}
	var got []bool
	for _, e := range menu[0].Items {
		got = append(got, e.NotInText)
	}
	if !slices.Equal(got, []bool{false, true, true, false}) {
		t.Errorf("NotInText = %v: auto leaves select all, notEditing every key", got)
	}
}
