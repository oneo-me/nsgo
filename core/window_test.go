package core

import (
	"slices"
	"testing"
	"time"

	"github.com/nsgo-dev/nsgo/internal/driver"
)

// windows runs an app on a platform that opens windows, as the engine
// does: the first tree is the app's own, and each surface opened commits
// on its own.
type windows struct {
	t    *testing.T
	root *Scope
	app  *Tree
	// items are the items each surface was sent, by ID.
	items map[uint32][]driver.MountItem
}

func newWindows(t *testing.T, app func(cx *Scope)) *windows {
	w := &windows{t: t, items: map[uint32][]driver.MountItem{}}
	w.root = NewRoot(RootOptions{Post: func(func()) {}, Clock: NewManualClock(time.Unix(0, 0)),
		Windows: &WindowOptions{Width: 400, Height: 300}})
	w.app = NewTree(w.root, ApproxTextEngine{})
	w.app.Mount(AppRoot(app))
	w.commit()
	return w
}

// commit commits every surface, as the engine does, keeping their items.
func (w *windows) commit() {
	trees := append([]*Tree{w.app}, w.app.Surfaces()...)
	for _, t := range trees {
		if t.Closed() {
			w.items[t.ID()] = append(w.items[t.ID()], driver.MountItem{Kind: driver.ItemWindow,
				Window: &driver.WindowCommand{Action: driver.WindowDestroy}})
			t.Forget()
			continue
		}
		if b := t.Commit(); b != nil {
			w.items[t.ID()] = append(w.items[t.ID()], b.Items...)
		}
	}
}

// take returns the items surface id was sent of kind, and forgets them.
func (w *windows) take(id uint32, kind driver.ItemKind) []driver.MountItem {
	var out, kept []driver.MountItem
	for _, it := range w.items[id] {
		if it.Kind == kind {
			out = append(out, it)
		} else {
			kept = append(kept, it)
		}
	}
	w.items[id] = kept
	return out
}

// texts returns the texts surface id shows.
func (w *windows) texts(id uint32) []string {
	var out []string
	for _, it := range w.items[id] {
		if it.Kind == driver.ItemText {
			out = append(out, it.Text.Text)
		}
	}
	return out
}

func (w *windows) dispatch(id uint32, ev driver.Event) {
	t := w.app
	if id != 0 {
		i := slices.IndexFunc(w.app.Surfaces(), func(t *Tree) bool { return t.ID() == id })
		if i < 0 {
			w.t.Fatalf("no surface %d", id)
		}
		t = w.app.Surfaces()[i]
	}
	t.Dispatch(ev)
	w.commit()
}

func TestWindowsShareSignals(t *testing.T) {
	var count *Signal[int]
	var settings *Window
	w := newWindows(t, func(cx *Scope) {
		count = NewSignal(cx, 1)
		label := func(cx *Scope) Node {
			return Text{ContentFunc: func() string { return "count " + string(rune('0'+count.Get())) }}
		}
		cx.SetMainView(label)
		settings = cx.OpenWindow(WindowOptions{Title: "Settings", Width: 200, Height: 100}, label)
	})
	opened := w.take(0, driver.ItemOpenSurface)
	if len(opened) != 2 || opened[0].Surface.Width != 400 || opened[1].Surface.Title != "Settings" ||
		opened[1].Surface.Width != 200 {
		t.Fatalf("opened %+v", opened)
	}
	if w.texts(1)[0] != "count 1" || w.texts(2)[0] != "count 1" {
		t.Fatalf("texts %q %q", w.texts(1), w.texts(2))
	}
	if w.app.Surfaces()[1].Size() != (Size{Width: 200, Height: 100}) {
		t.Error("a window does not start at its size")
	}
	w.items = map[uint32][]driver.MountItem{}
	count.Set(2)
	w.commit()
	if w.texts(1)[0] != "count 2" || w.texts(2)[0] != "count 2" {
		t.Errorf("after a change: %q %q", w.texts(1), w.texts(2))
	}
	if settings.CurrentWindowOf() != settings {
		t.Error("a window's content is not in it")
	}
}

func TestFloatingWindowSurface(t *testing.T) {
	var floating *Window
	w := newWindows(t, func(cx *Scope) {
		floating = cx.OpenFloatingWindow(FloatingWindowOptions{Width: 240, Height: 120, X: 12, Y: 18,
			Focusable: true, TouchThrough: true}, func(*Scope) Node { return View{} })
	})
	opened := w.take(0, driver.ItemOpenSurface)
	if len(opened) != 1 || opened[0].Surface == nil || !opened[0].Surface.Overlay {
		t.Fatalf("floating surface = %+v", opened)
	}
	cfg := opened[0].Surface
	if cfg.Width != 240 || cfg.Height != 120 || cfg.Frame.X != 12 || cfg.Frame.Y != 18 || !cfg.Focusable || !cfg.TouchThrough {
		t.Fatalf("floating config = %+v", cfg)
	}
	floating.Close()
	w.commit()
	if len(w.take(1, driver.ItemWindow)) != 1 {
		t.Fatal("floating surface did not close")
	}
}

// CurrentWindowOf returns the window w's content finds as its own.
func (w *Window) CurrentWindowOf() *Window { return w.scope.CurrentWindow() }

func TestWindowCloseEndsTheAppByMode(t *testing.T) {
	for _, tc := range []struct {
		mode  ShutdownMode
		close uint32
		quits bool
	}{
		{ShutdownOnLastWindowClose, 2, false},
		{ShutdownOnMainWindowClose, 2, false},
		{ShutdownOnMainWindowClose, 1, true},
		{ShutdownOnExplicit, 1, false},
	} {
		w := newWindows(t, func(cx *Scope) {
			cx.SetShutdownMode(tc.mode)
			cx.SetMainView(func(cx *Scope) Node { return Text{Content: "main"} })
			cx.OpenWindow(WindowOptions{}, func(cx *Scope) Node { return Text{Content: "other"} })
		})
		w.dispatch(tc.close, driver.Event{Kind: driver.EventWindowClose})
		if d := w.take(tc.close, driver.ItemWindow); len(d) != 1 || d[0].Window.Action != driver.WindowDestroy {
			t.Errorf("mode %d closing %d: %+v", tc.mode, tc.close, d)
		}
		if quits := len(w.take(0, driver.ItemQuit)) > 0; quits != tc.quits {
			t.Errorf("mode %d closing %d: quits %v", tc.mode, tc.close, quits)
		}
	}
	w := newWindows(t, func(cx *Scope) {
		cx.SetMainView(func(cx *Scope) Node { return Text{Content: "main"} })
	})
	w.dispatch(1, driver.Event{Kind: driver.EventWindowClose})
	if len(w.take(0, driver.ItemQuit)) != 1 {
		t.Error("the last window's close does not end the app")
	}
}

func TestWindowClosingCancels(t *testing.T) {
	var main *Window
	w := newWindows(t, func(cx *Scope) {
		cx.SetMainView(func(cx *Scope) Node {
			win := cx.CurrentWindow()
			win.OnClosing(cx, func(c *WindowClosing) {
				c.Cancel()
				win.Hide()
			})
			return Text{Content: "main"}
		})
		main = cx.MainWindow()
	})
	w.dispatch(1, driver.Event{Kind: driver.EventWindowClose})
	cmds := w.take(1, driver.ItemWindow)
	if len(cmds) != 1 || cmds[0].Window.Action != driver.WindowHide || main.Closed() || main.Visible() {
		t.Fatalf("a canceled close: %+v closed %v", cmds, main.Closed())
	}
	if len(w.take(0, driver.ItemQuit)) != 0 {
		t.Error("a canceled close ended the app")
	}
	w.dispatch(0, driver.Event{Kind: driver.EventReopen})
	if cmds := w.take(1, driver.ItemWindow); len(cmds) != 1 || cmds[0].Window.Action != driver.WindowOrderFront ||
		!main.Visible() {
		t.Errorf("reopening does not show the main window: %+v", cmds)
	}
}

func TestWindowKeyAndOwner(t *testing.T) {
	var cx0 *Scope
	var owner *Scope
	w := newWindows(t, func(cx *Scope) {
		cx0 = cx
		cx.SetShutdownMode(ShutdownOnExplicit)
	})
	a := cx0.OpenWindow(WindowOptions{Key: "settings"}, nil)
	b := cx0.OpenWindow(WindowOptions{Key: "settings"}, nil)
	if a != b {
		t.Error("a key opened a second window")
	}
	owner = cx0.Child()
	c := owner.OpenWindow(WindowOptions{}, nil)
	w.commit()
	if len(w.take(0, driver.ItemOpenSurface)) != 2 {
		t.Error("windows opened")
	}
	owner.Dispose()
	w.commit()
	if !c.Closed() || len(w.take(c.t.ID(), driver.ItemWindow)) == 0 {
		t.Error("a window outlived its owner")
	}
	if len(w.app.Surfaces()) != 1 {
		t.Errorf("surfaces left: %d", len(w.app.Surfaces()))
	}
	// Methods of a closed or nil window do nothing.
	c.Show()
	var none *Window
	none.Focus()
	if none.Visible() || !none.Closed() {
		t.Error("a nil window shows")
	}
}

func TestTrayAndPanel(t *testing.T) {
	var chosen, clicked int
	var panel *Window
	w := newWindows(t, func(cx *Scope) {
		cx.SetShutdownMode(ShutdownOnExplicit)
		panel = cx.OpenPanel(PanelOptions{HideOnBlur: true}, func(cx *Scope) Node { return Text{Content: "panel"} })
		cx.NewTray(TrayOptions{Icon: []byte{1}, Template: true, Panel: panel,
			Menu: func() []MenuItem { return []MenuItem{{Label: "Quit", OnSelect: func() { chosen++ }}} }})
		cx.NewTray(TrayOptions{View: func(cx *Scope) Node { return Text{Content: "3"} }, OnClick: func() { clicked++ }})
	})
	trays := w.take(0, driver.ItemTray)
	if len(trays) != 2 || !trays[0].Tray.Template || trays[1].Tray.View != 2 {
		t.Fatalf("trays %+v %+v", trays[0].Tray, trays[1].Tray)
	}
	opened := w.take(0, driver.ItemOpenSurface)
	if len(opened) != 2 || opened[0].Surface.Kind != driver.SurfacePanel || !opened[0].Surface.Hidden ||
		!opened[0].Surface.HideOnBlur || opened[1].Surface.Kind != driver.SurfaceTrayIcon || opened[1].Surface.Tray != 2 {
		t.Fatalf("opened %+v %+v", opened[0].Surface, opened[1].Surface)
	}
	if w.texts(2)[0] != "3" {
		t.Error("a tray's view does not show")
	}
	w.dispatch(0, driver.Event{Kind: driver.EventTray, Count: 1})
	if cmds := w.take(1, driver.ItemWindow); len(cmds) != 1 || cmds[0].Window.Action != driver.WindowShow ||
		cmds[0].Window.Anchor != 1 || !panel.Visible() {
		t.Fatalf("a primary click does not show the panel under the icon: %+v", cmds)
	}
	w.dispatch(1, driver.Event{Kind: driver.EventWindowHidden})
	if panel.Visible() {
		t.Error("a hidden panel shows")
	}
	w.dispatch(0, driver.Event{Kind: driver.EventTray, Count: 1, Index: 1})
	menus := w.take(0, driver.ItemTrayMenu)
	if len(menus) != 1 || menus[0].Tray.ID != 1 || menus[0].Menu[0].Label != "Quit" {
		t.Fatalf("menus %+v", menus)
	}
	w.dispatch(0, driver.Event{Kind: driver.EventMenu, Count: menus[0].Menu[0].ID})
	w.dispatch(0, driver.Event{Kind: driver.EventTray, Count: 2})
	if chosen != 1 || clicked != 1 {
		t.Errorf("chosen %d clicked %d", chosen, clicked)
	}
}

func TestHotKeysDockAndLogin(t *testing.T) {
	var cx0, owner *Scope
	fired := 0
	var login []bool
	w := newWindows(t, func(cx *Scope) {
		cx0 = cx
		cx.SetDockVisible(false)
		cx.SetLaunchAtLogin(true)
		cx.LaunchAtLogin(func(on bool) { login = append(login, on) })
	})
	// Outside a running owner, what a scope makes is its own.
	owner = cx0.Child()
	owner.RegisterHotKey("Alt+Space", func() { fired++ })
	w.commit()
	keys := w.take(0, driver.ItemHotKeys)
	if len(keys) != 1 || keys[0].Keys[0].Key != "alt+space" || keys[0].Keys[0].ID == 0 {
		t.Fatalf("keys %+v", keys)
	}
	w.dispatch(0, driver.Event{Kind: driver.EventHotKey, Count: keys[0].Keys[0].ID})
	if fired != 1 {
		t.Error("a hot key did not fire")
	}
	if d := w.take(0, driver.ItemDock); len(d) != 1 || d[0].On {
		t.Errorf("dock %+v", d)
	}
	if s := w.take(0, driver.ItemSetLaunchAtLogin); len(s) != 1 || !s[0].On {
		t.Errorf("launch at login %+v", s)
	}
	asks := w.take(0, driver.ItemLaunchAtLogin)
	w.dispatch(0, driver.Event{Kind: driver.EventLaunchAtLogin, Count: asks[0].Count, Finished: true})
	if !slices.Equal(login, []bool{true}) {
		t.Errorf("login %v", login)
	}
	owner.Dispose()
	w.commit()
	if keys := w.take(0, driver.ItemHotKeys); len(keys) != 1 || len(keys[0].Keys) != 0 {
		t.Errorf("a hot key outlived its owner: %+v", keys)
	}
}

func TestMainViewWithoutWindows(t *testing.T) {
	var cx0 *Scope
	h := newHost(t, Size{Width: 100, Height: 100}, AppRoot(func(cx *Scope) {
		cx0 = cx
		cx.SetMainView(func(cx *Scope) Node { return Text{Content: "one", TestID: "one"} })
		if cx.OpenWindow(WindowOptions{}, nil) != nil || cx.NewTray(TrayOptions{}) != nil {
			t.Error("a platform without windows opened one")
		}
	}))
	h.commit()
	if !h.has("one") {
		t.Fatal("the main view does not show")
	}
	cx0.SetMainView(func(cx *Scope) Node { return Text{Content: "two", TestID: "two"} })
	h.commit()
	if h.has("one") || !h.has("two") {
		t.Error("a later main view does not replace it")
	}
}

func TestWindowConstraints(t *testing.T) {
	var cx *Scope
	w := newWindows(t, func(app *Scope) {
		app.OpenWindow(WindowOptions{Width: 1000, Height: 100, Constraints: WindowConstraints{
			MinWidth: 300, MinHeight: 200, MaxWidth: 100, MaxHeight: 800, NoResize: true}},
			func(c *Scope) Node { cx = c; return View{} })
	})
	cfg := w.take(0, driver.ItemOpenSurface)[0].Surface
	want := WindowConstraints{MinWidth: 300, MinHeight: 200, MaxWidth: 300, MaxHeight: 800, NoResize: true, NoMaximize: true}
	if cfg.Constraints != want {
		t.Fatalf("a window opens with %+v; want %+v", cfg.Constraints, want)
	}
	if cfg.Width != 300 || cfg.Height != 200 {
		t.Fatalf("a window opens at %v by %v; want its sizes kept to, at 300 by 200", cfg.Width, cfg.Height)
	}
	cx.SetWindowConstraints(WindowConstraints{MinWidth: -1, MaxHeight: 500, NoMinimize: true})
	w.commit()
	sent := w.take(cfg.ID, driver.ItemWindow)
	if len(sent) != 1 || sent[0].Window.Action != driver.WindowSetConstraints ||
		sent[0].Window.Constraints != (WindowConstraints{MaxHeight: 500, NoMinimize: true}) {
		t.Fatalf("the window was sent %+v", sent)
	}
}

func TestWindowConstraintsClamp(t *testing.T) {
	c := WindowConstraints{MinWidth: 300, MaxHeight: 500}.Normalized()
	for _, tc := range []struct{ in, want Size }{
		{Size{Width: 100, Height: 100}, Size{Width: 300, Height: 100}},
		{Size{Width: 4000, Height: 4000}, Size{Width: 4000, Height: 500}},
	} {
		if got := c.Clamp(tc.in); got != tc.want {
			t.Errorf("Clamp(%v) = %v; want %v", tc.in, got, tc.want)
		}
	}
}

// A window restored at a frame opens there at its size, and reports the
// frame the platform gives it, not the app's own.
func TestWindowFrame(t *testing.T) {
	saved := WindowFrame{X: 40, Y: 50, Move: true, Width: 900, Height: 700}
	var inside *Scope
	w := newWindows(t, func(cx *Scope) {
		cx.OpenWindow(WindowOptions{Frame: saved, Constraints: WindowConstraints{MaxWidth: 800}}, func(cx *Scope) Node {
			inside = cx
			return View{}
		})
		cx.OpenWindow(WindowOptions{Frame: WindowFrame{Width: 900, Height: 700}}, func(cx *Scope) Node { return View{} })
	})
	opened := w.take(0, driver.ItemOpenSurface)
	if len(opened) != 2 {
		t.Fatalf("opened %d surfaces", len(opened))
	}
	s := opened[0].Surface
	if s.Frame != saved || s.Width != 800 || s.Height != 700 {
		t.Fatalf("opened %+v", s)
	}
	if s := opened[1].Surface; s.Frame != (WindowFrame{}) || s.Width != 400 || s.Height != 300 {
		t.Fatalf("a frame that does not move opened %+v", s)
	}
	env := inside.Env()
	if env.WindowFrame() != (WindowFrame{}) || env.FullScreen() {
		t.Fatal("a window starts with a frame the platform has not reported")
	}
	moved := WindowFrame{X: 60, Y: 70, Move: true, Width: 800, Height: 700}
	w.app.Surfaces()[0].SetEnvironment(driver.Environment{FullScreen: true, WindowFrame: moved})
	if env.WindowFrame() != moved || !env.FullScreen() {
		t.Fatalf("reported frame %+v", env.WindowFrame())
	}
}

func TestOpenFilesWaitForAHandler(t *testing.T) {
	var posted []func()
	root := NewRoot(RootOptions{Post: func(fn func()) { posted = append(posted, fn) }, Clock: NewManualClock(time.Unix(0, 0))})
	tree := NewTree(root, ApproxTextEngine{})
	var opened [][]string
	handle := NewSignal(root, false)
	tree.Mount(func(cx *Scope) Node {
		return Show{When: handle.Get, Then: func(cx *Scope) Node {
			cx.OnOpenFiles(func(paths []string) { opened = append(opened, paths) })
			return nil
		}}
	})
	tree.Commit()
	open := func(paths ...string) {
		tree.Dispatch(driver.Event{Kind: driver.EventOpenFiles, Data: paths})
	}
	open("/a.flow")
	open("/b.flow")
	handle.Set(true)
	tree.Commit()
	for len(posted) > 0 {
		fn := posted[0]
		posted = posted[1:]
		fn()
	}
	open("/c.flow")
	if want := [][]string{{"/a.flow", "/b.flow"}, {"/c.flow"}}; !slices.EqualFunc(opened, want, slices.Equal) {
		t.Errorf("opened %q, want %q", opened, want)
	}
	handle.Set(false)
	tree.Commit()
	open("/d.flow")
	if len(opened) != 2 {
		t.Errorf("a disposed handler heard %q", opened[2:])
	}
}
