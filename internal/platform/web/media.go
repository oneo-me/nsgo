//go:build js && wasm

package web

import (
	"fmt"
	"log"
	"net/url"
	"strings"
	"syscall/js"
	"time"

	"github.com/oneo-me/nsgo/internal/driver"
	"github.com/oneo-me/nsgo/internal/platform/webstyle"
)

// The animation graph runs in Go on the page's main thread, stepped by
// requestAnimationFrame while it runs; bound values are written into the
// views' inline styles. Layout animations are Web Animations, which play
// over those styles without replacing them, as NSH's web runtime does.

// startTicking asks for animation frames until nothing animates.
func (s *surface) startTicking() {
	if s.ticking {
		return
	}
	s.ticking = true
	if s.frame.IsUndefined() {
		s.frame = js.FuncOf(func(_ js.Value, args []js.Value) any {
			s.tick(time.Duration(args[0].Float() * float64(time.Millisecond)))
			return nil
		})
	}
	js.Global().Call("requestAnimationFrame", s.frame)
}

func (s *surface) tick(now time.Duration) {
	s.ticking = false
	s.animator.Step(now)
	s.applyAnimated()
	if s.animator.Running() {
		s.staleEffects()
	}
	s.changedShaders()
	s.drawShaders()
	if s.frameWanted {
		s.frameWanted = false
		if s.engine != nil {
			s.engine.Frame()
		}
	}
	if s.animator.Running() || s.frameWanted {
		s.startTicking()
	}
}

// applyAnimated writes the animated values that changed.
func (s *surface) applyAnimated() {
	s.animator.Changed(func(id driver.NodeID, vals driver.AnimatedValues) {
		v := s.view(id)
		if v == nil {
			return
		}
		v.animated = vals
		// What animates goes inline over the view's class; once nothing
		// does, the class shows.
		style := map[string]string{"opacity": "", "transform": "", "background-color": "", "box-shadow": "",
			"transition": "", "-webkit-text-fill-color": "", "border-color": "", "will-change": ""}
		if vals.Set != 0 {
			vs := vals.Apply(v.style)
			style["opacity"], style["transform"] = webstyle.Opacity(vs.Opacity), webstyle.Transform(vs.Transform)
			style["background-color"], style["box-shadow"] = webstyle.Fill(vs.Background), webstyle.Shadow(&vs)
			// A bound value moves at once; the view's transition is for
			// its style's changes.
			style["transition"] = "none"
			if vs.Opacity == 1 {
				style["opacity"] = "1"
			}
			if style["transform"] == "" {
				style["transform"] = "none"
			}
			if vals.Has(driver.PropTranslateX) || vals.Has(driver.PropTranslateY) {
				style["will-change"] = "transform"
			}
			if style["background-color"] == "" {
				style["background-color"] = "transparent"
			}
			if style["box-shadow"] == "" {
				style["box-shadow"] = "none"
			}
			if vals.Has(driver.PropBorderColor) {
				style["border-color"] = webstyle.Color(vs.BorderColor)
			}
			// A bound color fills every glyph, the spans' too, which
			// inherit the fill color over their own.
			if vals.Has(driver.PropTextColor) {
				style["-webkit-text-fill-color"] = webstyle.Color(vals.TextColor)
			}
		}
		v.css().set(style)
	})
	s.changedCanvases()
	s.changedNatives()
	s.replaceBound()
}

// layoutBatch is what a batch with a layout animation tracks.
type layoutBatch struct {
	anim    *driver.LayoutAnimation
	created map[driver.NodeID]bool
}

// timing returns the Web Animations timing of a layout transition: its
// curve, a spring sampled into CSS's linear() easing.
func timing(tr *driver.LayoutTransition) map[string]any {
	easing := "linear"
	switch tr.Curve {
	case driver.LayoutEaseIn:
		easing = "ease-in"
	case driver.LayoutEaseOut:
		easing = "ease-out"
	case driver.LayoutEaseInEaseOut, driver.LayoutKeyboard:
		easing = "ease-in-out"
	case driver.LayoutBezier:
		if c := tr.Bezier; c != (driver.Curve{}) {
			easing = fmt.Sprintf("cubic-bezier(%g, %g, %g, %g)", c.X1, c.Y1, c.X2, c.Y2)
		}
	case driver.LayoutSpring:
		curve := *tr
		curve.Delay, curve.Duration = 0, time.Second
		var pts []string
		for i := range 41 {
			p, _ := curve.At(time.Duration(i) * time.Second / 40)
			pts = append(pts, fmt.Sprintf("%.4f", p))
		}
		easing = "linear(" + strings.Join(pts, ", ") + ")"
	}
	return map[string]any{"duration": float64(tr.Duration) / 1e6, "delay": float64(tr.Delay) / 1e6,
		"easing": easing, "fill": "backwards"}
}

// moveFrame animates v from where it shows to the item's frame, when the
// batch animates layout or the item carries a transition.
func (s *surface) moveFrame(v *view, it *driver.MountItem, lb *layoutBatch) {
	tr := it.Transition
	if lb != nil && lb.anim.Update != nil && !lb.created[v.id] {
		tr = lb.anim.Update
	}
	if tr == nil || !v.framed {
		return
	}
	f := it.Frame
	// Where it shows: mid animation, the computed style's.
	cs := js.Global().Call("getComputedStyle", v.el)
	from := map[string]any{"left": cs.Get("left").String(), "top": cs.Get("top").String(),
		"width": cs.Get("width").String(), "height": cs.Get("height").String()}
	if v.layoutAnim.Truthy() {
		v.layoutAnim.Call("cancel")
	}
	to := map[string]any{"left": webstyle.Px(f.Origin.X), "top": webstyle.Px(f.Origin.Y), "width": webstyle.Px(f.Size.Width), "height": webstyle.Px(f.Size.Height)}
	v.layoutAnim = v.el.Call("animate", []any{from, to}, timing(tr))
}

// appear animates a view the batch created into view.
func (s *surface) appear(v *view, lb *layoutBatch, parent driver.NodeID) {
	if lb == nil || lb.anim.Create == nil || !lb.created[v.id] || lb.created[parent] {
		return
	}
	tr := lb.anim.Create
	from := map[string]any{"opacity": 0}
	if tr.Property == driver.LayoutScale {
		from = map[string]any{"scale": "0"}
	}
	v.appearAnim = v.el.Call("animate", []any{from, map[string]any{}}, timing(tr))
}

// match moves v, which the batch created, from where old, which it
// deletes, shows, on the batch's Update rather than appearing, and hides
// old at once (driver.ItemMatch). Both rectangles are the page's, so the
// difference between them carries v's frame to where old shows.
func (s *surface) match(v, old *view, lb *layoutBatch) {
	if lb == nil || lb.anim.Update == nil || old == nil || !v.framed || !old.el.Get("isConnected").Bool() ||
		!v.el.Get("isConnected").Bool() {
		return
	}
	if v.appearAnim.Truthy() {
		v.appearAnim.Call("cancel")
		v.appearAnim = js.Undefined()
	}
	was, now := old.el.Call("getBoundingClientRect"), v.el.Call("getBoundingClientRect")
	f := v.frame
	dx := driver.Points(was.Get("left").Float() - now.Get("left").Float())
	dy := driver.Points(was.Get("top").Float() - now.Get("top").Float())
	from := map[string]any{"left": webstyle.Px(f.Origin.X + dx), "top": webstyle.Px(f.Origin.Y + dy),
		"width": webstyle.Px(driver.Points(was.Get("width").Float())), "height": webstyle.Px(driver.Points(was.Get("height").Float()))}
	to := map[string]any{"left": webstyle.Px(f.Origin.X), "top": webstyle.Px(f.Origin.Y), "width": webstyle.Px(f.Size.Width), "height": webstyle.Px(f.Size.Height)}
	setStyle(old.el, map[string]string{"visibility": "hidden"})
	if v.layoutAnim.Truthy() {
		v.layoutAnim.Call("cancel")
	}
	v.layoutAnim = v.el.Call("animate", []any{from, to}, timing(lb.anim.Update))
}

// vanish keeps a deleted view's element where it is while it fades or
// shrinks away, unless a view the batch deleted around it does. It
// reports whether it kept it.
func (s *surface) vanish(v *view, lb *layoutBatch) bool {
	if lb == nil || lb.anim.Delete == nil || !v.el.Get("parentElement").Truthy() {
		return false
	}
	if v.el.Call("closest", "[data-nsgo-ghost]").Truthy() {
		return true // it goes with its ancestor
	}
	el := v.el
	el.Get("dataset").Set("nsgoGhost", "1")
	el.Set("nsgoID", js.Undefined())
	setStyle(el, map[string]string{"pointer-events": "none"})
	tr := lb.anim.Delete
	to := map[string]any{"opacity": 0}
	if tr.Property == driver.LayoutScale {
		to = map[string]any{"scale": "0"}
	}
	t := timing(tr)
	t["fill"] = "forwards"
	anim := el.Call("animate", []any{map[string]any{}, to}, t)
	var done js.Func
	done = js.FuncOf(func(js.Value, []js.Value) any {
		el.Call("remove")
		done.Release()
		return nil
	})
	anim.Set("onfinish", done)
	return true
}

// Native components fill their view with a DOM element the app makes.

func (s *surface) configureNative(v *view, c *driver.NativeConfig) {
	if c.Bind {
		s.animator.BindNative(v.id, c.Bindings)
	}
	if v.component == nil && !v.componentTried {
		v.componentTried = true
		factory := s.components[c.Component]
		if factory == nil {
			if !s.unknown[c.Component] {
				s.unknown[c.Component] = true
				log.Printf("nsgo: no native component %q is registered", c.Component)
			}
			return
		}
		e := s.engine
		id := v.id
		v.component = factory(driver.NewNativeContext(
			func(name string, data any) {
				e.Dispatch(driver.Event{Kind: driver.EventNative, ID: id, Text: name, Data: data})
			},
			func(size driver.Size) { e.Dispatch(driver.Event{Kind: driver.EventNativeSize, ID: id, Size: size}) },
		))
		if v.component == nil {
			return
		}
		if el, ok := v.component.View().(js.Value); ok && el.Truthy() {
			setStyle(el, map[string]string{"position": "absolute", "left": "0", "top": "0", "width": "100%",
				"height": "100%", "box-sizing": "border-box", "margin": "0"})
			v.el.Call("appendChild", el)
		}
	}
	if v.component == nil {
		return
	}
	for name, value := range c.Props {
		v.component.SetProp(name, value)
	}
}

// File drops: files dragged from the desktop onto a view that listens.
// Browsers hide files' paths, so each file arrives as a blob: URL, which
// fetch and net/http read, with the file's name as its fragment.

func (s *surface) listenFiles(ls *listeners) {
	over := func(e js.Value) {
		if !hasFiles(e) {
			return
		}
		v := s.dropView(e.Get("target"))
		s.enterDrag(v)
		if v != nil {
			e.Call("preventDefault")
			e.Get("dataTransfer").Set("dropEffect", "copy")
		} else {
			e.Get("dataTransfer").Set("dropEffect", "none")
		}
	}
	ls.on(s.host, "dragenter", over)
	ls.on(s.host, "dragover", over)
	ls.on(s.host, "dragleave", func(e js.Value) {
		// Leaving for a child is entering it; only leaving the host ends.
		if rel := e.Get("relatedTarget"); !rel.Truthy() || !s.host.Call("contains", rel).Bool() {
			s.enterDrag(nil)
		}
	})
	ls.on(s.host, "drop", func(e js.Value) {
		v := s.dropView(e.Get("target"))
		s.drag = nil
		if v == nil || !hasFiles(e) {
			return
		}
		e.Call("preventDefault")
		files := e.Get("dataTransfer").Get("files")
		var uris []string
		for i := range files.Length() {
			f := files.Index(i)
			blob := js.Global().Get("URL").Call("createObjectURL", f).String()
			uris = append(uris, blob+"#"+url.PathEscape(f.Get("name").String()))
		}
		if len(uris) > 0 {
			s.send(v, driver.Event{Kind: driver.EventFileDrop, URIs: uris})
		}
	})
}

func hasFiles(e js.Value) bool {
	dt := e.Get("dataTransfer")
	if !dt.Truthy() {
		return false
	}
	types := dt.Get("types")
	for i := range types.Length() {
		if types.Index(i).String() == "Files" {
			return true
		}
	}
	return false
}

// dropView returns the innermost view holding el that takes files.
func (s *surface) dropView(el js.Value) *view {
	for ; el.Truthy() && !el.Equal(s.host); el = el.Get("parentElement") {
		v := s.viewOf(el)
		if v == nil {
			continue
		}
		if v.events.Has(driver.EventFileDrop) {
			if v.disabled {
				return nil
			}
			return v
		}
		if v.kind == driver.KindModal {
			return nil
		}
	}
	return nil
}

func (s *surface) enterDrag(v *view) {
	if s.drag == v {
		return
	}
	if old := s.drag; old != nil {
		s.send(old, driver.Event{Kind: driver.EventFileDragLeave})
	}
	s.drag = v
	if v != nil {
		s.send(v, driver.Event{Kind: driver.EventFileDragEnter})
	}
}
