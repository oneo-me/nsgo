package tsx

import (
	"github.com/oneo-me/nsgo/tsx/internal/js"

	"github.com/oneo-me/nsgo/base"
	"github.com/oneo-me/nsgo/core"
)

// Navigation (NSH's navigation.rs): createNavStack is a base.Nav of JS
// routes, NavStack its stack, and NavPage the page a route renders,
// which the stack reads as it is a part, not a node.

// navKey holds a JS stack's base.Nav.
var navKey = js.NewSymbol("nsgo.nav")

func init() {
	moduleTable["nsgo/base"] = append(moduleTable["nsgo/base"],
		fn("createNavStack", (*runtime).createNavStack),
		faceExport("NavStack", navStackFace),
		fn("NavPage", (*runtime).navPage),
	)
}

// createNavStack is createNavStack(routes): routes() and the stack's
// changes.
func (r *runtime) createNavStack(call js.FunctionCall) js.Value {
	nav := base.NewNav(r.cx, r.arrayItems(arg(call, 0))...)
	o := r.vm.NewObject()
	_ = o.SetSymbol(navKey, nav)
	set := func(name string, f func(call js.FunctionCall)) {
		_ = o.Set(name, r.function(func(call js.FunctionCall) js.Value {
			f(call)
			return js.Undefined()
		}))
	}
	_ = o.Set("routes", r.accessor(func() js.Value {
		routes := nav.Routes()
		out := make([]any, len(routes))
		for i, rt := range routes {
			out[i] = rt
		}
		return r.vm.NewArray(out...)
	}))
	set("push", func(call js.FunctionCall) { nav.Push(arg(call, 0)) })
	set("pop", func(call js.FunctionCall) {
		n := 1
		if defined(arg(call, 0)) {
			n = int(arg(call, 0).ToInteger())
		}
		nav.Pop(n)
	})
	set("popToRoot", func(js.FunctionCall) { nav.PopToRoot() })
	set("replace", func(call js.FunctionCall) { nav.Replace(arg(call, 0)) })
	set("setRoutes", func(call js.FunctionCall) {
		nav.SetRoutesKeeping(strictEquals, r.arrayItems(arg(call, 0))...)
	})
	set("remove", func(call js.FunctionCall) {
		// The topmost of the pages of route, as NSH's remove.
		route := arg(call, 0)
		count := 0
		r.cx.Untrack(func() {
			for _, v := range nav.Routes() {
				if v.StrictEquals(route) {
					count++
				}
			}
		})
		seen := 0
		nav.Remove(func(v js.Value) bool {
			if v.StrictEquals(route) {
				seen++
				return seen == count
			}
			return false
		})
	})
	return o
}

func (r *runtime) navOf(v js.Value) *base.Nav[js.Value] {
	if o, ok := v.(*js.Object); ok {
		if s := o.GetSymbol(navKey); s != nil {
			if n, ok := s.Export().(*base.Nav[js.Value]); ok {
				return n
			}
		}
	}
	panic(r.vm.NewTypeError("nsgo: NavStack's state is not a createNavStack"))
}

func navStackFace(p *props) core.Node {
	r := p.r
	nav := r.navOf(p.get("state"))
	// A stack fills its parent unless its style sizes it.
	style := p.style()
	if style.Height.IsAuto() && style.FlexGrow == 0 {
		style.FlexGrow = 1
	}
	if c := p.class(); c != "" {
		// The stack takes no classes: they style a view around it.
		return core.View{Class: c, Style: core.Style{FlexGrow: 1}, Children: []core.Node{r.navStack(p, nav, style)}}
	}
	n := r.navStack(p, nav, style)
	return n
}

func (r *runtime) navStack(p *props, nav *base.Nav[js.Value], style core.Style) core.Node {
	render := p.peek("children")
	stack := nav.Stack(style, func(cx *core.Scope, route js.Value, depth func() int) base.NavPage {
		index := r.accessor(func() js.Value { return r.vm.ToValue(depth()) })
		var out js.Value
		cx.Run(func() { out = r.call(render, route, index) })
		if pg := r.pageOf(out); pg != nil {
			return pg.build()
		}
		// What the function rendered, in a page around it.
		return base.NavPage{Children: []core.Node{r.region(func() js.Value { return out })}}
	})
	if st, ok := stack.(core.Stack); ok {
		st.TestID = p.str("testID")
		return st
	}
	return stack
}

// page is a NavPage's call: what builds its screen.
type page struct{ build func() base.NavPage }

func (r *runtime) pageOf(v js.Value) *page {
	if o, ok := v.(*js.Object); ok {
		if s := o.GetSymbol(navKey); s != nil {
			if pg, ok := s.Export().(*page); ok {
				return pg
			}
		}
	}
	return nil
}

var headerItems = map[string]core.HeaderSystemItem{"add": core.HeaderItemAdd, "done": core.HeaderItemDone,
	"close": core.HeaderItemClose, "edit": core.HeaderItemEdit, "compose": core.HeaderItemCompose,
	"search": core.HeaderItemSearch, "share": core.HeaderItemShare, "trash": core.HeaderItemTrash,
	"save": core.HeaderItemSave, "cancel": core.HeaderItemCancel}

// screenAnimations are NSH's: "slide" is the platform's push, as the
// default.
var screenAnimations = map[string]core.ScreenAnimation{"slide": core.ScreenDefault, "fade": core.ScreenFade,
	"none": core.ScreenNone}

// navPage is NavPage(props): a page its stack reads.
func (r *runtime) navPage(call js.FunctionCall) js.Value {
	obj, ok := arg(call, 0).(*js.Object)
	if !ok {
		obj = r.vm.NewObject()
	}
	n := r.newFaceNode("NavPage", nil, obj)
	p := n.props
	o := r.vm.NewObject()
	_ = o.SetSymbol(navKey, &page{build: func() base.NavPage {
		s := base.NavPage{
			TitleFunc:         p.strFunc("title"),
			Header:            p.slot("header"),
			HeaderLeft:        p.slot("headerLeft"),
			HeaderRight:       p.slot("headerRight"),
			HeaderTitle:       p.slot("headerTitle"),
			HeaderHidden:      !p.boolOr("headerVisible", true),
			HeaderTransparent: p.bool("headerTransparent"),
			HeaderLargeTitle:  p.bool("headerLargeTitle"),
			HeaderBackHidden:  !p.boolOr("headerBackVisible", true),
			HeaderBackTitle:   p.str("headerBackTitle"),
			StatusBarFunc:     func() core.StatusBarSetting { return r.statusBar(p.get("statusBar")) },
			Class:             p.class(),
			StyleFunc:         p.style,
			Children:          p.children(),
			NoPopOnBackFunc:   func() bool { return !p.boolOr("popOnBack", true) },
			NoPopOnSwipeFunc:  func() bool { return !p.boolOr("popOnSwipe", true) },
			OnBackPress:       p.on("onBackPress"),
			OnDismiss:         p.on("onDismiss"),
			TestID:            p.str("testID"),
			HeaderClass:       p.str("headerClass"),
			Animation:         screenAnimations[p.str("animation")],
			TransitionShader:  p.transitionShader(),
		}
		s.IOS.HeaderRightSystemItem = headerItems[p.str("headerRightSystemItem")]
		if p.has("headerStyle") {
			s.HeaderStyleFunc = func() core.Style {
				h := r.options(p.get("headerStyle"))
				var st core.Style
				if c := h.get("backgroundColor"); defined(c) {
					st.Background = r.color("headerStyle", c)
				}
				if c := h.get("color"); defined(c) {
					st.Color = r.color("headerStyle", c)
				}
				if c := h.get("tintColor"); defined(c) {
					st.TintColor = r.color("headerStyle", c)
				}
				return st
			}
		}
		return s
	}})
	return o
}
