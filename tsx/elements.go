package tsx

import (
	"strings"
	"time"

	"github.com/dop251/goja"

	"github.com/oneo-me/nsgo/core"
)

// The primitives that are Go components (NSH's list.rs and
// platform_modules.rs): Overlay, WebView, Video, List, Dialog and
// InputAccessoryView, each a face over core's element of the same
// meaning.

// hostFaces are the host tags compiled JSX makes in place that are faces.
var hostFaces = map[string]face{
	"overlay": overlayFace,
	"webview": webViewFace,
	"video":   videoFace,
}

var placements = map[string]core.Placement{"top": core.PlaceTop, "bottom": core.PlaceBottom,
	"left": core.PlaceLeft, "right": core.PlaceRight}

var placementNames = map[core.Placement]string{core.PlaceTop: "top", core.PlaceBottom: "bottom",
	core.PlaceLeft: "left", core.PlaceRight: "right"}

var overlayAligns = map[string]core.Align{"start": core.AlignStart, "center": core.AlignCenter, "end": core.AlignEnd}

func overlayFace(p *props) core.Node {
	r := p.r
	o := core.Overlay{
		Placement:     placements[p.str("placement")],
		Align:         overlayAligns[p.str("align")],
		Offset:        p.points("offset"),
		Margin:        p.points("margin"),
		Class:         p.class(),
		StyleFunc:     p.styleFunc(),
		Children:      p.children(),
		PointerEvents: pointerEvents[p.str("pointerEvents")],
		OnCancel:      p.on("onCancel"),
		Role:          roles[p.str("role")],
		Label:         p.str("accessibilityLabel"),
		TestID:        p.str("testID"),
	}
	switch a := p.get("anchor"); {
	case r.nodeOf(a) != nil:
		o.Anchor = r.nodeOf(a).ref
	case defined(a):
		o.AnchorBounds = func() core.Bounds { return r.anchorBounds(p.get("anchor")) }
	}
	if h := p.handler("onPressOutside"); h != nil {
		o.OnPressOutside = func(at core.Point) {
			e := r.vm.NewObject()
			_ = e.Set("pagePosition", r.point(at))
			h(e)
		}
	}
	if h := p.handler("onPosition"); h != nil {
		o.OnPosition = func(pos core.OverlayPosition) {
			e := r.vm.NewObject()
			_ = e.Set("placement", placementNames[pos.Placement])
			_ = e.Set("bounds", r.bounds(pos.Bounds))
			_ = e.Set("available", r.size(pos.Available))
			h(e)
		}
	}
	o.Ref = p.n.ref
	return o
}

// anchorBounds reads an anchor that is a point or bounds.
func (r *runtime) anchorBounds(v goja.Value) core.Bounds {
	o, ok := v.(*goja.Object)
	if !ok {
		return core.Bounds{}
	}
	if origin, ok := o.Get("origin").(*goja.Object); ok {
		b := core.Bounds{Origin: r.pointOf(origin)}
		if size, ok := o.Get("size").(*goja.Object); ok {
			b.Size = r.sizeOf(size)
		}
		return b
	}
	return core.Bounds{Origin: r.pointOf(o)}
}

func (r *runtime) pointOf(o *goja.Object) core.Point {
	return core.Point{X: core.Points(opts{o}.float("x", 0)), Y: core.Points(opts{o}.float("y", 0))}
}

func (r *runtime) sizeOf(o *goja.Object) core.Size {
	return core.Size{Width: core.Points(opts{o}.float("width", 0)), Height: core.Points(opts{o}.float("height", 0))}
}

func webViewFace(p *props) core.Node {
	r := p.r
	w := core.WebView{
		SourceFunc: func() core.WebSource {
			if html := p.get("html"); defined(html) {
				return core.WebHTML(html.String(), p.str("baseUrl"))
			}
			return core.WebSrc(p.str("src"))
		},
		InjectedJavaScript:              p.str("injectedJavaScript"),
		InjectedJavaScriptBeforeContent: p.str("injectedJavaScriptBeforeContentLoaded"),
		UserAgent:                       p.str("userAgent"),
		NoScroll:                        !p.boolOr("scrollable", true),
		NoJavaScript:                    !p.boolOr("javaScriptEnabled", true),
		BackForwardGestures:             p.bool("allowsBackForwardNavigationGestures"),
		Class:                           p.class(),
		StyleFunc:                       p.styleFunc(),
		OnError:                         p.onError("onError"),
		OnLayout:                        p.onLayout(),
		Ref:                             p.n.ref,
		Label:                           p.str("accessibilityLabel"),
		TestID:                          p.str("testID"),
	}
	nav := func(name string) func(core.WebNavigation) {
		h := p.handler(name)
		if h == nil {
			return nil
		}
		return func(n core.WebNavigation) { h(r.webNavigation(n)) }
	}
	w.OnLoadStart, w.OnLoad, w.OnNavigationStateChange = nav("onLoadStart"), nav("onLoad"), nav("onNavigationStateChange")
	if h := p.handler("onMessage"); h != nil {
		w.OnMessage = func(data string) {
			e := r.vm.NewObject()
			_ = e.Set("data", data)
			h(e)
		}
	}
	if h := p.handler("onDownload"); h != nil {
		w.OnDownload = func(d core.WebDownload) { h(r.webDownload(d)) }
	}
	return w
}

// webDownload is { state, id, url, filename, path }, with bytes once
// finished and message when it failed.
func (r *runtime) webDownload(d core.WebDownload) *goja.Object {
	o := r.vm.NewObject()
	_ = o.Set("state", [...]string{"started", "finished", "failed"}[d.State])
	_ = o.Set("id", d.ID)
	_ = o.Set("url", d.URL)
	_ = o.Set("filename", d.Filename)
	_ = o.Set("path", d.Path)
	switch d.State {
	case core.WebDownloadFinished:
		_ = o.Set("bytes", d.Bytes)
	case core.WebDownloadFailed:
		_ = o.Set("message", d.Message)
	}
	return o
}

func (r *runtime) webNavigation(n core.WebNavigation) *goja.Object {
	o := r.vm.NewObject()
	_ = o.Set("url", n.URL)
	_ = o.Set("title", n.Title)
	_ = o.Set("loading", n.Loading)
	_ = o.Set("canGoBack", n.CanGoBack)
	_ = o.Set("canGoForward", n.CanGoForward)
	return o
}

var resizeModes = map[string]core.ResizeMode{"cover": core.Cover, "contain": core.Contain,
	"stretch": core.Stretch, "center": core.Center, "scale-down": core.ScaleDown}

var audioSessions = map[string]core.AudioSession{"default": core.AudioSessionDefault,
	"playback": core.AudioSessionPlayback, "mixWithOthers": core.AudioSessionMixWithOthers}

func videoFace(p *props) core.Node {
	r := p.r
	v := core.Video{
		SourceFunc:       p.strFunc("src"),
		PausedFunc:       func() bool { return p.bool("paused") },
		MutedFunc:        func() bool { return p.bool("muted") },
		VolumeFunc:       func() float32 { return float32(p.float("volume")) },
		RateFunc:         func() float32 { return float32(p.float("rate")) },
		Repeat:           p.bool("repeat"),
		Controls:         p.bool("controls"),
		ProgressInterval: time.Duration(p.float("progressUpdateInterval") * float64(time.Millisecond)),
		AudioSession:     audioSessions[p.str("audioSession")],
		Class:            p.class(),
		StyleFunc:        p.styleFunc(),
		OnLoadStart:      p.on("onLoadStart"),
		OnEnd:            p.on("onEnd"),
		OnError:          p.onError("onError"),
		OnPress:          p.onPress("onPress"),
		OnLayout:         p.onLayout(),
		Ref:              p.n.ref,
		Label:            p.str("accessibilityLabel"),
		TestID:           p.str("testID"),
	}
	if !p.has("volume") {
		v.VolumeFunc = nil
	}
	if h := p.handler("onLoad"); h != nil {
		v.OnLoad = func(info core.VideoInfo) {
			e := r.vm.NewObject()
			_ = e.Set("duration", info.Duration.Seconds())
			_ = e.Set("size", r.size(info.Size))
			h(e)
		}
	}
	if h := p.handler("onProgress"); h != nil {
		v.OnProgress = func(position, duration time.Duration) {
			e := r.vm.NewObject()
			_ = e.Set("currentTime", position.Seconds())
			_ = e.Set("duration", duration.Seconds())
			h(e)
		}
	}
	if h := p.handler("onBuffer"); h != nil {
		v.OnBuffer = func(buffering bool) {
			e := r.vm.NewObject()
			_ = e.Set("isBuffering", buffering)
			h(e)
		}
	}
	return v
}

// list is `list(props)`, what <List> compiles to: a lazy list of each's
// items, a row rendered by children(item, index) per cell.
func (r *runtime) list(call goja.FunctionCall) goja.Value {
	obj := arg(call, 0).ToObject(r.vm)
	n := r.newFaceNode("List", listFace, obj)
	n.horizontal = opts{obj}.bool("horizontal", false)
	r.applyRef(obj.Get("ref"), n, "")
	r.mountFace(n)
	return n.value()
}

var listViewports = map[string]core.ListViewport{"own": core.ListViewportOwn, "parent": core.ListViewportParent}

func listFace(p *props) core.Node {
	r := p.r
	items := core.NewMemoFunc(r.cx, func() []goja.Value { return r.arrayItems(p.get("each")) }, nil)
	item := func(i int) goja.Value {
		if all := items.Get(); i >= 0 && i < len(all) {
			return all[i]
		}
		return goja.Undefined()
	}
	l := core.List{
		Count:                    func() int { return len(items.Get()) },
		EstimatedSize:            p.points("estimatedItemSize"),
		NumColumns:               p.int("numColumns"),
		Horizontal:               p.bool("horizontal"),
		HideIndicator:            !p.boolOr("scrollbar", true),
		Bar:                      r.scrollbarTheme(p.get("scrollbar")),
		Inverted:                 p.bool("inverted"),
		StickyHeaderIndicesFunc:  func() []int { return r.ints(p.get("stickyHeaderIndices")) },
		InitialScrollIndex:       p.int("initialScrollIndex"),
		OnLoadMore:               p.on("onLoadMore"),
		LoadMoreThreshold:        float32(p.float("loadMoreThreshold")),
		OnLoadMoreAtStart:        p.on("onLoadMoreAtStart"),
		LoadMoreAtStartThreshold: float32(p.float("loadMoreAtStartThreshold")),
		Viewport:                 listViewports[p.str("viewport")],
		BehaviorFunc:             func() core.ScrollBehavior { return r.scrollBehavior(p.get) },
		OnScroll:                 r.scrollEventHandler(p.get("onScroll")),
		ScrollEventThrottle:      time.Duration(p.float("scrollEventThrottle") * float64(time.Millisecond)),
		OnScrollDragStart:        r.scrollHandler(p.handler("onScrollDragStart")),
		OnScrollDragEnd:          r.scrollHandler(p.handler("onScrollDragEnd")),
		OnMomentumStart:          r.scrollHandler(p.handler("onMomentumStart")),
		OnMomentumEnd:            r.scrollHandler(p.handler("onMomentumEnd")),
		OnScrollEnd:              r.scrollHandler(p.handler("onScrollEnd")),
		OnRefresh:                p.on("onRefresh"),
		Refreshing:               func() bool { return p.bool("refreshing") },
		OnLayout:                 p.onLayout(),
		// A list's padding is its content's: contentStyle's paddings and
		// contentClass go on it.
		Class: strings.TrimSpace(p.class() + " " + p.str("contentClass")),
		StyleFunc: func() core.Style {
			s := p.style()
			if c := p.get("contentStyle"); defined(c) {
				if pad := r.style(c).Padding; pad != (core.Edges{}) {
					s.Padding = pad
				}
			}
			return s
		},
		Ref:    p.n.ref,
		TestID: p.str("testID"),
	}
	if c := p.get("contentOffset"); defined(c) {
		l.ContentOffset = r.pointOf(c.ToObject(r.vm))
	}
	if v := r.animatedValueOf(p.get("pullValue")); v != nil {
		l.PullValue = v
	}
	if v := r.animatedValueOf(p.get("scrollValue")); v != nil {
		if l.Horizontal {
			l.OffsetValues.X = v
		} else {
			l.OffsetValues.Y = v
		}
	}
	switch off := p.get("stickyHeaderOffset"); {
	case r.animatedNodeOf(off) != nil:
		l.StickyHeaderOffsetValue = r.animatedNodeOf(off)
	case defined(off):
		l.StickyHeaderOffset = core.Points(off.ToFloat())
	}
	if m, ok := p.get("maintainVisibleContentPosition").(*goja.Object); ok {
		l.MaintainVisibleContentPosition = &core.VisibleContentPosition{
			AutoscrollToBottomThreshold: core.Points(opts{m}.float("autoscrollToBottomThreshold", 0))}
	}
	if c, ok := p.get("contentStyle").(*goja.Object); ok {
		o := opts{c}
		l.Gap, l.ColumnGap = core.Points(o.float("gap", o.float("rowGap", 0))), core.Points(o.float("columnGap", 0))
	}
	if c, ok := p.get("viewabilityConfig").(*goja.Object); ok {
		o := opts{c}
		l.ViewabilityConfig = core.ViewabilityConfig{
			ViewAreaCoveragePercentThreshold: float32(o.float("viewAreaCoverageThreshold", 0)) * 100,
			ItemVisiblePercentThreshold:      float32(o.float("itemVisibleThreshold", 0)) * 100,
			MinimumViewTime:                  time.Duration(o.float("minimumViewTime", 0) * float64(time.Millisecond)),
		}
	}
	keyOfItem := func(i int) any { return keyOf(item(i)) }
	if isFunction(p.get("keyExtractor")) {
		keyOfItem = func(i int) any {
			k := p.handler("keyExtractor")
			return stringOf(k(item(i), r.vm.ToValue(i)))
		}
	}
	l.Key = keyOfItem
	l.Row = func(cx *core.Scope, row core.ListRow) core.Node {
		render := p.peek("children")
		index := r.accessor(func() goja.Value { return r.vm.ToValue(row.Index()) })
		// A row the list is about to take away keeps its last item, as
		// its bindings may read it once more as the items shrink.
		last := goja.Undefined()
		it := r.accessor(func() goja.Value {
			if i := row.Index(); i >= 0 && i < len(items.Get()) {
				last = item(i)
			}
			return last
		})
		return r.region(func() goja.Value { return r.call(render, it, index) })
	}
	listSlot := func(name string) core.Node {
		if !p.has(name) {
			return nil
		}
		return r.region(func() goja.Value {
			v := p.get(name)
			if isFunction(v) {
				return r.call(v)
			}
			return v
		})
	}
	l.Header, l.Footer, l.Empty = listSlot("header"), listSlot("footer"), listSlot("empty")
	if p.has("separator") {
		l.Separator = func(*core.Scope) core.Node { return listSlot("separator") }
	}
	if h := p.handler("onViewableItemsChange"); h != nil {
		l.OnViewableItemsChange = func(v core.ViewableItems) {
			tokens := func(ts []core.ViewToken) goja.Value {
				out := make([]any, len(ts))
				for i, t := range ts {
					o := r.vm.NewObject()
					_ = o.Set("item", item(t.Index))
					_ = o.Set("key", r.tokenKey(t, item(t.Index)))
					_ = o.Set("index", t.Index)
					_ = o.Set("isViewable", t.IsViewable)
					out[i] = o
				}
				return r.vm.NewArray(out...)
			}
			e := r.vm.NewObject()
			_ = e.Set("viewableItems", tokens(v.Viewable))
			_ = e.Set("changed", tokens(v.Changed))
			h(e)
		}
	}
	return l
}

// tokenKey is a viewability token's key: the keyExtractor's, else the
// item's key or id, else its index.
func (r *runtime) tokenKey(t core.ViewToken, item goja.Value) string {
	if s, ok := t.Key.(string); ok {
		return s
	}
	if o, ok := item.(*goja.Object); ok {
		for _, k := range []string{"key", "id"} {
			if v := o.Get(k); defined(v) {
				return v.String()
			}
		}
	}
	return r.vm.ToValue(t.Index).String()
}

// ints reads an array of numbers.
func (r *runtime) ints(v goja.Value) []int {
	items := r.arrayItems(v)
	if items == nil {
		return nil
	}
	out := make([]int, len(items))
	for i, it := range items {
		out[i] = int(it.ToInteger())
	}
	return out
}
