package tsx

import (
	"time"

	"github.com/dop251/goja"

	"github.com/oneo-me/nsgo/core"
)

// The methods of a node (NSH's YNode, `_core.d.ts`): each a command to
// the view through its ref. The VM's field name mapper names them as JS
// does (Focus is focus).

// Focus focuses an <Input>.
func (n *node) Focus() {
	if n.ref != nil {
		n.ref.Focus()
	}
}

// Blur unfocuses an <Input>.
func (n *node) Blur() {
	if n.ref != nil {
		n.ref.Blur()
	}
}

// SetSelection selects start..end of an <Input>, a caret without end.
func (n *node) SetSelection(start int, end goja.Value) {
	if n.ref == nil {
		return
	}
	e := start
	if defined(end) {
		e = int(end.ToInteger())
	}
	n.ref.SetSelection(start, e)
}

// Command calls command name of a native component with args.
func (n *node) Command(name string, args goja.Value) {
	if n.ref == nil {
		return
	}
	var v any
	if defined(args) {
		v = args.Export()
	}
	n.ref.Command(name, v)
}

// ScrollTo scrolls a <Scroll> or a List to { x, y, animated }.
func (n *node) ScrollTo(options goja.Value) {
	if n.ref == nil {
		return
	}
	o := n.r.options(options)
	n.ref.ScrollTo(core.Point{X: core.Points(o.float("x", 0)), Y: core.Points(o.float("y", 0))}, o.bool("animated", true))
}

// ZoomTo zooms a zoomable <Scroll> to { scale, x, y, animated }.
func (n *node) ZoomTo(options goja.Value) {
	if n.ref == nil {
		return
	}
	o := n.r.options(options)
	var center *core.Point
	if o.has("x") || o.has("y") {
		center = &core.Point{X: core.Points(o.float("x", 0)), Y: core.Points(o.float("y", 0))}
	}
	n.ref.ZoomTo(float32(o.float("scale", 1)), center, o.bool("animated", true))
}

// ScrollToOffset scrolls a List to { offset } along its axis.
func (n *node) ScrollToOffset(options goja.Value) {
	if n.ref == nil {
		return
	}
	o := n.r.options(options)
	at := core.Point{Y: core.Points(o.float("offset", 0))}
	if n.horizontal {
		at = core.Point{X: at.Y}
	}
	n.ref.ScrollTo(at, o.bool("animated", true))
}

// ScrollToIndex scrolls item { index } of a List into view, placed by
// viewPosition and viewOffset.
func (n *node) ScrollToIndex(options goja.Value) {
	if n.ref == nil {
		return
	}
	o := n.r.options(options)
	n.ref.ScrollToIndexAt(int(o.float("index", 0)), float32(o.float("viewPosition", 0)),
		core.Points(o.float("viewOffset", 0)), o.bool("animated", true))
}

// ScrollToEnd scrolls a <Scroll> or a List to its content's end.
func (n *node) ScrollToEnd(options goja.Value) {
	if n.ref != nil {
		n.ref.ScrollToEnd(n.r.options(options).bool("animated", true))
	}
}

// Measure resolves with { bounds, pagePosition } once this turn's changes
// are laid out, or null when the view is not in the tree.
func (n *node) Measure() *goja.Promise {
	p, resolve, _ := n.r.vm.NewPromise()
	n.r.cx.Post(func() {
		var frame, page core.Bounds
		ok := n.ref != nil
		if ok {
			frame, ok = n.ref.Frame()
			page, _ = n.ref.Bounds()
		}
		if !ok {
			_ = resolve(goja.Null())
			return
		}
		o := n.r.vm.NewObject()
		_ = o.Set("bounds", n.r.bounds(frame))
		_ = o.Set("pagePosition", n.r.point(page.Origin))
		_ = resolve(o)
	})
	return p
}

// Reload reloads a WebView's page.
func (n *node) Reload() {
	if n.ref != nil {
		n.ref.Reload()
	}
}

// GoBack goes back in a WebView's history.
func (n *node) GoBack() {
	if n.ref != nil {
		n.ref.GoBack()
	}
}

// GoForward goes forward in a WebView's history.
func (n *node) GoForward() {
	if n.ref != nil {
		n.ref.GoForward()
	}
}

// StopLoading stops a WebView loading.
func (n *node) StopLoading() {
	if n.ref != nil {
		n.ref.StopLoading()
	}
}

// InjectJavaScript runs script in a WebView's page.
func (n *node) InjectJavaScript(script string) {
	if n.ref != nil {
		n.ref.InjectJavaScript(script)
	}
}

// PostMessage sends data to a WebView's page.
func (n *node) PostMessage(data string) {
	if n.ref != nil {
		n.ref.PostMessage(data)
	}
}

// Seek jumps a Video to seconds.
func (n *node) Seek(seconds float64) {
	if n.ref != nil {
		n.ref.Seek(time.Duration(seconds * float64(time.Second)))
	}
}

// opts reads an options object: absent keys take the defaults given.
type opts struct{ o *goja.Object }

func (r *runtime) options(v goja.Value) opts {
	o, _ := v.(*goja.Object)
	return opts{o}
}

func (o opts) get(k string) goja.Value {
	if o.o == nil {
		return nil
	}
	return o.o.Get(k)
}

func (o opts) has(k string) bool { return defined(o.get(k)) }

func (o opts) float(k string, def float64) float64 {
	if v := o.get(k); defined(v) {
		return v.ToFloat()
	}
	return def
}

func (o opts) bool(k string, def bool) bool {
	if v := o.get(k); defined(v) {
		return v.ToBoolean()
	}
	return def
}

func (o opts) str(k, def string) string {
	if v := o.get(k); defined(v) {
		return v.String()
	}
	return def
}

// defined reports whether v is a value: not missing, undefined or null.
func defined(v goja.Value) bool { return v != nil && !goja.IsUndefined(v) && !goja.IsNull(v) }

func (r *runtime) bounds(b core.Bounds) *goja.Object {
	o := r.vm.NewObject()
	_ = o.Set("origin", r.point(b.Origin))
	_ = o.Set("size", r.size(b.Size))
	return o
}
