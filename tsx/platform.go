package tsx

import (
	"github.com/dop251/goja"

	"github.com/oneo-me/nsgo/core"
	"github.com/oneo-me/nsgo/desktop"
	"github.com/oneo-me/nsgo/mobile"
)

// nsgo/mobile and nsgo/desktop (NSH's platform_modules.rs, desktop.rs):
// importable everywhere, their calls doing nothing where they do not
// apply, as the Go packages behind them do.

func init() {
	moduleTable["nsgo/mobile"] = []export{
		{"isAvailable", func(r *runtime) goja.Value { return r.vm.ToValue(mobile.IsAvailable(r.cx)) }},
		faceExport("InputAccessoryView", accessoryFace),
		faceExport("TabBar", tabBarFace),
		{"BackHandler", (*runtime).backHandler},
	}
	moduleTable["nsgo/desktop"] = []export{
		{"isAvailable", func(r *runtime) goja.Value { return r.vm.ToValue(desktop.IsAvailable(r.cx)) }},
		{"titleBar", func(r *runtime) goja.Value {
			return r.accessor(func() goja.Value {
				e := desktop.TitleBar(r.cx)
				o := r.vm.NewObject()
				_ = o.Set("height", js32(e.Top))
				_ = o.Set("left", js32(e.Left))
				_ = o.Set("right", js32(e.Right))
				return o
			})
		}},
		{"Menu", (*runtime).menuModule},
		{"Window", (*runtime).windowModule},
	}
}

func accessoryFace(p *props) core.Node {
	return core.InputAccessoryView{ID: p.str("nativeID"), StyleFunc: p.style, Children: p.children(),
		TestID: p.str("testID")}
}

func tabBarFace(p *props) core.Node {
	r := p.r
	return mobile.TabBar{
		ItemsFunc: func() []core.TabBarItem {
			var items []core.TabBarItem
			for _, it := range r.arrayItems(p.get("items")) {
				o := r.options(it)
				items = append(items, core.TabBarItem{Title: o.str("title", ""), Icon: o.str("icon", "")})
			}
			return items
		},
		Selected: func() int { return p.int("selectedIndex") },
		OnChange: func(i int) {
			if h := p.handler("onChange"); h != nil {
				e := r.vm.NewObject()
				_ = e.Set("index", i)
				h(e)
			}
		},
		Style:  core.Untracked(r.cx, p.style),
		TestID: p.str("testID"),
	}
}

// backHandler is BackHandler: onBackPress(listener) adds a listener of
// Android's back button, removed by what it returns or with the owner it
// was added under; exitApp ends the app.
func (r *runtime) backHandler() goja.Value {
	o := r.vm.NewObject()
	_ = o.Set("onBackPress", r.function(func(call goja.FunctionCall) goja.Value {
		fn := arg(call, 0)
		owner := r.cx.Child()
		owner.Run(func() {
			owner.OnBackPress(func() bool {
				res := r.call(fn)
				return isBool(res) && res.ToBoolean()
			})
		})
		return r.function(func(goja.FunctionCall) goja.Value {
			owner.Dispose()
			return goja.Undefined()
		})
	}))
	_ = o.Set("exitApp", r.function(func(goja.FunctionCall) goja.Value {
		mobile.ExitApp(r.cx)
		return goja.Undefined()
	}))
	return o
}

var menuRoles = map[string]core.MenuRole{"about": core.MenuAbout, "hide": core.MenuHide,
	"hideOthers": core.MenuHideOthers, "showAll": core.MenuShowAll, "quit": core.MenuQuit, "undo": core.MenuUndo,
	"redo": core.MenuRedo, "cut": core.MenuCut, "copy": core.MenuCopy, "paste": core.MenuPaste,
	"selectAll": core.MenuSelectAll, "minimize": core.MenuMinimize, "zoom": core.MenuZoom,
	"toggleFullScreen": core.MenuToggleFullScreen, "close": core.MenuClose}

// menuItems reads a menu's items, falsy ones left out.
func (r *runtime) menuItems(v goja.Value) []core.MenuItem {
	var out []core.MenuItem
	for _, it := range r.arrayItems(v) {
		if !it.ToBoolean() {
			continue
		}
		o := r.options(it)
		if o.str("type", "") == "separator" {
			out = append(out, core.MenuItem{Separator: true})
			continue
		}
		item := core.MenuItem{Label: o.str("label", ""), Shortcut: o.str("shortcut", ""),
			Role: menuRoles[o.str("role", "")], Disabled: o.bool("disabled", false), Checkable: o.has("checked"),
			Checked: o.bool("checked", false), Items: r.menuItems(o.get("submenu")),
			When: map[string]core.MenuWhen{"auto": core.MenuAuto, "notEditing": core.MenuNotEditing}[o.str("when", "")]}
		if fn := o.get("onSelect"); isFunction(fn) {
			item.OnSelect = func() { r.call(fn) }
		}
		out = append(out, item)
	}
	return out
}

func (r *runtime) menuModule() goja.Value {
	o := r.vm.NewObject()
	_ = o.Set("setApplicationMenu", r.function(func(call goja.FunctionCall) goja.Value {
		desktop.SetApplicationMenu(r.cx, r.menuItems(arg(call, 0)))
		return goja.Undefined()
	}))
	_ = o.Set("popup", r.function(func(call goja.FunctionCall) goja.Value {
		p, resolve, _ := r.vm.NewPromise()
		var at core.Point
		if a, ok := arg(call, 1).(*goja.Object); ok {
			at = r.pointOf(a)
		}
		desktop.PopupMenu(r.cx, r.menuItems(arg(call, 0)), at, func() { r.inJS(func() { _ = resolve(goja.Undefined()) }) })
		return r.vm.ToValue(p)
	}))
	return o
}

func (r *runtime) windowModule() goja.Value {
	o := r.vm.NewObject()
	set := func(name string, f func(call goja.FunctionCall)) {
		_ = o.Set(name, r.function(func(call goja.FunctionCall) goja.Value {
			f(call)
			return goja.Undefined()
		}))
	}
	set("setTitle", func(call goja.FunctionCall) { desktop.SetTitle(r.cx, arg(call, 0).String()) })
	set("setTitleBar", func(call goja.FunctionCall) {
		style := map[string]core.TitleBarStyle{"default": core.TitleBarDefault, "hidden": core.TitleBarHidden}[stringOf(arg(call, 0))]
		var h core.Points
		if defined(arg(call, 1)) {
			h = core.Points(arg(call, 1).ToFloat())
		}
		desktop.SetTitleBar(r.cx, style, h)
	})
	set("setMenuBar", func(call goja.FunctionCall) {
		desktop.SetMenuBar(r.cx, map[string]core.MenuBarMode{"shown": core.MenuBarShown,
			"autoHide": core.MenuBarAutoHide}[stringOf(arg(call, 0))])
	})
	set("minimize", func(goja.FunctionCall) { desktop.MinimizeWindow(r.cx) })
	set("toggleMaximize", func(goja.FunctionCall) { desktop.ToggleMaximize(r.cx) })
	set("toggleFullScreen", func(goja.FunctionCall) { desktop.ToggleFullScreen(r.cx) })
	set("setFrame", func(call goja.FunctionCall) {
		f := r.options(arg(call, 0))
		origin, size := r.options(f.get("origin")), r.options(f.get("size"))
		desktop.SetWindowFrame(r.cx, core.WindowFrame{X: core.Points(origin.float("x", 0)),
			Y: core.Points(origin.float("y", 0)), Move: origin.o != nil, Width: core.Points(size.float("width", 0)),
			Height: core.Points(size.float("height", 0)), Animated: f.bool("animated", false)})
	})
	set("setLevel", func(call goja.FunctionCall) {
		desktop.SetWindowLevel(r.cx, map[string]core.WindowLevel{"normal": core.WindowLevelNormal,
			"floating": core.WindowLevelFloating, "desktopIcon": core.WindowLevelDesktop}[stringOf(arg(call, 0))])
	})
	set("setConstraints", func(call goja.FunctionCall) {
		c := r.options(arg(call, 0))
		desktop.SetWindowConstraints(r.cx, core.WindowConstraints{MinWidth: core.Points(c.float("minWidth", 0)),
			MinHeight: core.Points(c.float("minHeight", 0)), MaxWidth: core.Points(c.float("maxWidth", 0)),
			MaxHeight: core.Points(c.float("maxHeight", 0)), NoResize: !c.bool("resizable", true),
			NoMinimize: !c.bool("minimizable", true), NoMaximize: !c.bool("maximizable", true)})
	})
	set("orderFront", func(goja.FunctionCall) { desktop.OrderFront(r.cx) })
	set("close", func(goja.FunctionCall) { desktop.CloseWindow(r.cx) })
	return o
}
