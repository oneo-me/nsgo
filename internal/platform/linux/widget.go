//go:build linux && !android

package linux

import (
	"fmt"
	"math"
	"sync"

	"github.com/ebitengine/purego"

	"github.com/oneo-me/nsgo/internal/driver"
)

// NsgoView is the GTK widget of one node, as NSH's NshView is: a
// GtkWidget subclass registered from Go. The core lays everything out,
// so a view never measures itself: its parent places it at the frame
// the core gave it (size_allocate, with the frame's origin, the scroll
// offset and the view's transform in one GskTransform), and it draws
// itself in snapshot: shadow and background, its content, its children
// (clipped to the rounded bounds when it clips), then its border. Native
// widgets a view hosts (a GtkEntry, a page, a video) are children of its
// widget placed after the app's children, so they stack, clip, fade and
// transform with the rest.
//
// One more NsgoView without a node is the surface: it fills the window
// below the menu bar, paints the root's background, holds the root view
// and takes the pointer.

// The class struct offsets the backend writes vfuncs at, from GTK 4's
// gtkwidget.h on 64-bit targets: GObjectClass is 136 bytes, then
// GtkWidgetClass adds 25 vfuncs, a private pointer and 8 padding slots.
// registerViewType checks the sizes against GLib's own before writing.
const (
	objectClassSize   = 136
	widgetClassSize   = objectClassSize + 34*8
	offDispose        = 40 // GObjectClass.dispose
	offSizeAllocate   = objectClassSize + 8*8
	offSnapshot       = objectClassSize + 23*8
	offContains       = objectClassSize + 24*8
	gtkPickDefault    = 0
	gtkOrientationH   = 0
	gtkOrientationV   = 1
	textOverhang      = 2 // points glyphs may paint outside their view
	indicatorAlpha    = 0.35
	indicatorThick    = 4.0
	indicatorInset    = 2.0
	indicatorMinimum  = 24.0
	shadowBlurPerUnit = 2 // GSK blurs by twice a shadow radius, as NSH does
)

// gTypeQuery is GLib's GTypeQuery.
type gTypeQuery struct {
	typ          uintptr
	name         uintptr
	classSize    uint32
	instanceSize uint32
}

// GDK and graphene values, laid out as C has them.
type (
	gdkRGBA        struct{ r, g, b, a float32 }
	grapheneRect   struct{ x, y, w, h float32 }
	graphenePoint  struct{ x, y float32 }
	grapheneSize   struct{ w, h float32 }
	gskRoundedRect struct {
		bounds grapheneRect
		corner [4]grapheneSize
	}
	gskShadow struct {
		color          gdkRGBA
		dx, dy, radius float32
	}
)

var (
	viewType      uintptr
	viewTypeOnce  sync.Once
	viewTypeErr   error
	parentDispose uintptr // GtkWidget's dispose, chained up to
)

// widgetRef is what a widget's vfuncs find: its window and its view, nil
// for the surface.
type widgetRef struct {
	w *window
	v *view
}

// widgets maps NsgoView instances to their views. GTK calls the vfuncs on
// the main thread; the lock only keeps tests' windows apart.
var widgets = struct {
	sync.Mutex
	m map[uintptr]widgetRef
}{m: map[uintptr]widgetRef{}}

func lookupWidget(widget uintptr) (widgetRef, bool) {
	widgets.Lock()
	r, ok := widgets.m[widget]
	widgets.Unlock()
	return r, ok
}

func setWidget(widget uintptr, r widgetRef) {
	widgets.Lock()
	widgets.m[widget] = r
	widgets.Unlock()
}

func forgetWidget(widget uintptr) {
	widgets.Lock()
	delete(widgets.m, widget)
	widgets.Unlock()
}

// registerViewType registers NsgoView once per process.
func registerViewType() error {
	viewTypeOnce.Do(func() {
		var widget, unowned gTypeQuery
		gTypeQueryFn(gtkWidgetGetType(), &widget)
		gTypeQueryFn(gInitiallyUnownedGetType(), &unowned)
		if unowned.classSize != objectClassSize || widget.classSize != widgetClassSize {
			viewTypeErr = fmt.Errorf("unexpected GtkWidgetClass layout: %d bytes over %d", widget.classSize, unowned.classSize)
			return
		}
		viewType = gTypeRegisterStaticSimple(gtkWidgetGetType(), cstr("NsgoView"), widget.classSize,
			cbViewClassInit, widget.instanceSize, 0, 0)
		if viewType == 0 {
			viewTypeErr = fmt.Errorf("GLib did not register NsgoView")
			return
		}
		// Range roles expose their value to AT-SPI only on widgets that
		// are GtkAccessibleRange; its one method keeps its default.
		var info gInterfaceInfo
		gTypeAddInterfaceStatic(viewType, gtkAccessibleRangeGetType(), &info)
	})
	return viewTypeErr
}

// setSlot writes the function pointer fn at offset in a class struct.
func setSlot(class uintptr, offset uintptr, fn uintptr) {
	*(*uintptr)(cPointer(class + offset)) = fn
}

func viewClassInitCallback(class, data uintptr) {
	parent := gTypeClassPeekParent(class)
	parentDispose = *(*uintptr)(cPointer(parent + offDispose))
	setSlot(class, offDispose, cbViewDispose)
	setSlot(class, offSizeAllocate, cbViewSizeAllocate)
	setSlot(class, offSnapshot, cbViewSnapshot)
	setSlot(class, offContains, cbViewContains)
}

// viewDisposeCallback unparents the children, as GTK asks of widgets
// that parent their own, and chains up.
func viewDisposeCallback(object uintptr) {
	for c := gtkWidgetGetFirstChild(object); c != 0; c = gtkWidgetGetFirstChild(object) {
		gtkWidgetUnparent(c)
	}
	forgetWidget(object)
	if parentDispose != 0 {
		purego.SyscallN(parentDispose, object)
	}
}

func viewSizeAllocateCallback(widget uintptr, width, height, baseline int32) {
	r, ok := lookupWidget(widget)
	if !ok || r.w.closing {
		return
	}
	if r.v == nil {
		r.w.allocateSurface(width, height)
		r.w.revealCanvases()
		return
	}
	r.w.allocateView(r.v)
	r.w.revealCanvases()
}

func viewSnapshotCallback(widget, snapshot uintptr) {
	r, ok := lookupWidget(widget)
	if !ok || r.w.closing {
		return
	}
	if r.v == nil {
		r.w.snapshotSurface(snapshot)
		return
	}
	r.w.snapshotView(r.v, snapshot)
}

// viewContainsCallback picks a view inside its frame, unless it is
// hidden.
func viewContainsCallback(widget uintptr, x, y float64) int32 {
	r, ok := lookupWidget(widget)
	if !ok {
		return 0
	}
	if r.v == nil {
		return boolInt(x >= 0 && y >= 0 && x < float64(gtkWidgetGetWidth(widget)) && y < float64(gtkWidgetGetHeight(widget)))
	}
	v := r.v
	if v.style.Hidden || v.deleted {
		return 0
	}
	f := v.shownFrame()
	return boolInt(x >= 0 && y >= 0 && x < float64(f.Size.Width) && y < float64(f.Size.Height))
}

// newViewWidget makes an NsgoView for v (nil: the surface) and keeps a
// reference to it, which dropWidget releases.
func (w *window) newViewWidget(v *view) uintptr {
	role := int32(-1)
	if v != nil {
		if r, ok := w.roles[v.id]; ok {
			role = r
		}
	}
	widget := newAccessibleWidget(viewType, role)
	gObjectRefSink(widget)
	setWidget(widget, widgetRef{w, v})
	return widget
}

// dropWidget takes v's widget out of the tree and releases it.
func (w *window) dropWidget(v *view) {
	if v.widget == 0 {
		return
	}
	w.unparentWidget(v.widget)
	forgetWidget(v.widget)
	gObjectUnref(v.widget)
	v.widget = 0
}

// unparentWidget takes widget from its parent, which places its
// children again.
func (w *window) unparentWidget(widget uintptr) {
	if parent := gtkWidgetGetParent(widget); parent != 0 {
		gtkWidgetUnparent(widget)
		gtkWidgetQueueAllocate(parent)
		gtkWidgetQueueDraw(parent)
	}
}

// insertWidget puts v's widget into p's before the widget of before, or
// after p's other children but before the native widgets p hosts.
func (w *window) insertWidget(p, v, before *view) {
	if v.widget == 0 || p.widget == 0 {
		return
	}
	if gtkWidgetGetParent(v.widget) != 0 {
		gtkWidgetUnparent(v.widget)
	}
	next := uintptr(0)
	if before != nil && before.widget != 0 && gtkWidgetGetParent(before.widget) == p.widget {
		next = before.widget
	} else {
		next = firstNative(p.widget)
	}
	gtkWidgetInsertBefore(v.widget, p.widget, next)
	gtkWidgetQueueAllocate(p.widget)
	gtkWidgetQueueDraw(p.widget)
}

// firstNative returns the first child of parent that is no NsgoView: the
// first native widget it hosts, or 0.
func firstNative(parent uintptr) uintptr {
	for c := gtkWidgetGetFirstChild(parent); c != 0; c = gtkWidgetGetNextSibling(c) {
		if _, ok := lookupWidget(c); !ok {
			return c
		}
	}
	return 0
}

// moved tells GTK that v's frame, transform or visibility changed: its
// parent places it again.
func (w *window) moved(v *view) {
	if v.widget == 0 {
		return
	}
	if parent := gtkWidgetGetParent(v.widget); parent != 0 {
		gtkWidgetQueueAllocate(parent)
	}
	gtkWidgetQueueDraw(v.widget)
}

// changed tells GTK that something v draws changed.
func (w *window) changed(v *view) {
	if v.widget != 0 {
		gtkWidgetQueueDraw(v.widget)
	}
}

// showWidget hides v's widget, and with it what it hosts, while v is
// hidden.
func (w *window) showWidget(v *view) {
	if v.widget != 0 {
		gtkWidgetSetChildVisible(v.widget, boolInt(!v.style.Hidden))
	}
}

// allocateSurface reports the size GTK gave the surface and places the
// root view in it.
func (w *window) allocateSurface(width, height int32) {
	size := driver.Size{Width: driver.Points(width), Height: driver.Points(height)}
	if !w.started {
		// The surface exists now, so its scale and direction are known.
		w.started = true
		w.updateEnvironment()
		if w.embedded && w.engine == nil {
			w.lastWidth, w.lastHeight = size.Width, size.Height
			w.startEngine(w.app, w.opts, size)
		}
	}
	if size.Width != w.lastWidth || size.Height != w.lastHeight {
		w.lastWidth, w.lastHeight = size.Width, size.Height
		if w.engine != nil {
			w.engine.Resize(size)
		}
	}
	if root := w.view(w.root); root != nil && root.widget != 0 && gtkWidgetGetParent(root.widget) == w.surface {
		w.place(root, 0, 0)
	}
}

// allocateView places v's children at their frames less v's scroll
// offset, and the native widgets it hosts.
func (w *window) allocateView(v *view) {
	ox, oy := float32(math.Round(float64(v.offset.X))), float32(math.Round(float64(v.offset.Y)))
	for c := gtkWidgetGetFirstChild(v.widget); c != 0; c = gtkWidgetGetNextSibling(c) {
		if r, ok := lookupWidget(c); ok {
			if r.v != nil {
				w.place(r.v, ox, oy)
			}
			continue
		}
		if part := v.part(c); part != nil {
			allocateNative(c, part.bounds)
			continue
		}
		switch {
		case c == v.fxArea && c != 0:
			allocateNative(c, w.effectArea(v))
		case c == v.entry, c == v.hint:
			allocateNative(c, v.input.Content)
		case v.hosted() != nil && c == v.hosted().child:
			b := driver.Bounds{Size: v.shownFrame().Size}
			if v.web != nil {
				// WebKit sizes a page's surface from its widget: mapped at
				// 0 × 0 it may never present a frame, so a page maps once
				// its view has a size.
				gtkWidgetSetChildVisible(c, boolInt(b.Size.Width >= 1 && b.Size.Height >= 1))
			}
			allocateNative(c, b)
		}
	}
}

// place allocates v's widget at the frame it shows, less its parent's
// scroll offset ox, oy, with its transform around its center and the
// scale of a layout animation after it.
func (w *window) place(v *view, ox, oy float32) {
	if gtkWidgetGetChildVisible(v.widget) == 0 {
		return
	}
	f := v.shownFrame()
	t := gskMatrixTransform(0, w.placement(v, ox, oy))
	width := int32(math.Ceil(max(float64(f.Size.Width), 0)))
	height := int32(math.Ceil(max(float64(f.Size.Height), 0)))
	gtkWidgetAllocate(v.widget, width, height, -1, t)
}

// allocateNative gives a hosted widget b, or its minimum size where that
// is larger: the view clips what overflows.
func allocateNative(widget uintptr, b driver.Bounds) {
	var minW, natW, minH, natH int32
	gtkWidgetMeasure(widget, gtkOrientationH, -1, &minW, &natW, nil, nil)
	gtkWidgetMeasure(widget, gtkOrientationV, -1, &minH, &natH, nil, nil)
	width := max(int32(math.Ceil(float64(b.Size.Width))), minW, 0)
	height := max(int32(math.Ceil(float64(b.Size.Height))), minH, 0)
	t := gskTransformTranslate(0, &graphenePoint{float32(b.Origin.X), float32(b.Origin.Y)})
	gtkWidgetAllocate(widget, width, height, -1, t)
}

// snapshotSurface paints the root's background over the surface, then
// the root view.
func (w *window) snapshotSurface(snap uintptr) {
	bg := driver.RGB(0xffffff)
	root := w.view(w.root)
	if root != nil {
		bg = background(root)
	}
	rect := grapheneRect{0, 0, float32(gtkWidgetGetWidth(w.surface)), float32(gtkWidgetGetHeight(w.surface))}
	c := rgba(bg)
	gtkSnapshotAppendColor(snap, &c, &rect)
	if root != nil && root.widget != 0 && gtkWidgetGetParent(root.widget) == w.surface {
		gtkWidgetSnapshotChild(w.surface, root.widget, snap)
	}
	w.snapshotPerf(snap, rect.w)
}

// rgba is c as GDK's color.
func rgba(c driver.Color) gdkRGBA {
	r, g, b, a := c.Components()
	return gdkRGBA{float32(r), float32(g), float32(b), float32(a)}
}

// roundedRect is a rectangle with radius r on every corner, clamped to
// half its sides.
func roundedRect(x, y, width, height, r float32) gskRoundedRect {
	r = max(min(r, width/2, height/2), 0)
	c := grapheneSize{r, r}
	return gskRoundedRect{bounds: grapheneRect{x, y, width, height}, corner: [4]grapheneSize{c, c, c, c}}
}

// The bindings of NsgoView and its drawing.
var (
	gTypeQueryFn                 func(uintptr, *gTypeQuery)
	gTypeRegisterStaticSimple    func(uintptr, *byte, uint32, uintptr, uint32, uintptr, uint32) uintptr
	gTypeClassPeekParent         func(uintptr) uintptr
	gInitiallyUnownedGetType     func() uintptr
	gtkWidgetGetType             func() uintptr
	gtkWidgetSetParent           func(uintptr, uintptr)
	gtkWidgetUnparent            func(uintptr)
	gtkWidgetInsertBefore        func(uintptr, uintptr, uintptr)
	gtkWidgetGetFirstChild       func(uintptr) uintptr
	gtkWidgetGetNextSibling      func(uintptr) uintptr
	gtkWidgetAllocate            func(uintptr, int32, int32, int32, uintptr)
	gtkWidgetMeasure             func(uintptr, int32, int32, *int32, *int32, *int32, *int32)
	gtkWidgetSnapshotChild       func(uintptr, uintptr, uintptr)
	gtkWidgetSetChildVisible     func(uintptr, int32)
	gtkWidgetGetChildVisible     func(uintptr) int32
	gtkWidgetPick                func(uintptr, float64, float64, int32) uintptr
	gtkWidgetComputePoint        func(uintptr, uintptr, *graphenePoint, *graphenePoint) int32
	gtkWidgetComputeBounds       func(uintptr, uintptr, *grapheneRect) int32
	gtkWidgetPaintableNew        func(uintptr) uintptr
	gtkNativeGetSurfaceTransform func(uintptr, *float64, *float64)
	gdkPaintableSnapshot         func(uintptr, uintptr, float64, float64)
	gtkSnapshotNew               func() uintptr
	gtkSnapshotFreeToNode        func(uintptr) uintptr
	gtkSnapshotAppendColor       func(uintptr, *gdkRGBA, *grapheneRect)
	gtkSnapshotAppendCairo       func(uintptr, *grapheneRect) uintptr
	gtkSnapshotAppendBorder      func(uintptr, *gskRoundedRect, *[4]float32, *[4]gdkRGBA)
	gtkSnapshotAppendOutset      func(uintptr, *gskRoundedRect, *gdkRGBA, float32, float32, float32, float32)
	gtkSnapshotPushRoundedClip   func(uintptr, *gskRoundedRect)
	gtkSnapshotPushOpacity       func(uintptr, float64)
	gtkSnapshotPushClip          func(uintptr, *grapheneRect)
	gtkSnapshotPushBlur          func(uintptr, float64)
	gtkSnapshotPushShadow        func(uintptr, *gskShadow, uintptr)
	gtkSnapshotPop               func(uintptr)
	gtkSnapshotPushBlend         func(uintptr, int32)
	gtkSnapshotAppendNode        func(uintptr, uintptr)
	gskRenderNodeDraw            func(uintptr, uintptr)
	gskRenderNodeUnref           func(uintptr)
	gskTransformTranslate        func(uintptr, *graphenePoint) uintptr
	gtkEventControllerLegacyNew  func() uintptr
	gdkEventGetEventType         func(uintptr) int32
	gdkEventGetPosition          func(uintptr, *float64, *float64) int32
	gdkEventGetModifierState     func(uintptr) uint32
	gdkEventGetDevice            func(uintptr) uintptr
	gdkButtonEventGetButton      func(uintptr) uint32
	gdkScrollEventGetDirection   func(uintptr) int32
	gdkScrollEventGetDeltas      func(uintptr, *float64, *float64)
	gdkScrollEventGetUnit        func(uintptr) int32 // from GTK 4.8
)

// bindWidget registers the bindings of NsgoView; bind calls it.
func bindWidget() {
	gobject := func(fn any, name string) { purego.RegisterLibFunc(fn, gobjectLib, name) }
	gobject(&gTypeQueryFn, "g_type_query")
	gobject(&gTypeRegisterStaticSimple, "g_type_register_static_simple")
	gobject(&gTypeClassPeekParent, "g_type_class_peek_parent")
	gobject(&gInitiallyUnownedGetType, "g_initially_unowned_get_type")
	gtk := func(fn any, name string) { purego.RegisterLibFunc(fn, gtkLib, name) }
	gtk(&gtkWidgetGetType, "gtk_widget_get_type")
	gtk(&gtkWidgetSetParent, "gtk_widget_set_parent")
	gtk(&gtkWidgetUnparent, "gtk_widget_unparent")
	gtk(&gtkWidgetInsertBefore, "gtk_widget_insert_before")
	gtk(&gtkWidgetGetFirstChild, "gtk_widget_get_first_child")
	gtk(&gtkWidgetGetNextSibling, "gtk_widget_get_next_sibling")
	gtk(&gtkWidgetAllocate, "gtk_widget_allocate")
	gtk(&gtkWidgetMeasure, "gtk_widget_measure")
	gtk(&gtkWidgetSnapshotChild, "gtk_widget_snapshot_child")
	gtk(&gtkWidgetSetChildVisible, "gtk_widget_set_child_visible")
	gtk(&gtkWidgetGetChildVisible, "gtk_widget_get_child_visible")
	gtk(&gtkWidgetPick, "gtk_widget_pick")
	gtk(&gtkWidgetComputePoint, "gtk_widget_compute_point")
	gtk(&gtkWidgetComputeBounds, "gtk_widget_compute_bounds")
	gtk(&gtkWidgetPaintableNew, "gtk_widget_paintable_new")
	gtk(&gtkNativeGetSurfaceTransform, "gtk_native_get_surface_transform")
	gtk(&gdkPaintableSnapshot, "gdk_paintable_snapshot")
	gtk(&gtkSnapshotNew, "gtk_snapshot_new")
	gtk(&gtkSnapshotFreeToNode, "gtk_snapshot_free_to_node")
	gtk(&gtkSnapshotAppendColor, "gtk_snapshot_append_color")
	gtk(&gtkSnapshotAppendCairo, "gtk_snapshot_append_cairo")
	gtk(&gtkSnapshotAppendBorder, "gtk_snapshot_append_border")
	gtk(&gtkSnapshotAppendOutset, "gtk_snapshot_append_outset_shadow")
	gtk(&gtkSnapshotPushRoundedClip, "gtk_snapshot_push_rounded_clip")
	gtk(&gtkSnapshotPushOpacity, "gtk_snapshot_push_opacity")
	gtk(&gtkSnapshotPushClip, "gtk_snapshot_push_clip")
	gtk(&gtkSnapshotPushBlur, "gtk_snapshot_push_blur")
	gtk(&gtkSnapshotPushShadow, "gtk_snapshot_push_shadow")
	gtk(&gtkSnapshotPop, "gtk_snapshot_pop")
	gtk(&gtkSnapshotPushBlend, "gtk_snapshot_push_blend")
	gtk(&gtkSnapshotAppendNode, "gtk_snapshot_append_node")
	gtk(&gskRenderNodeDraw, "gsk_render_node_draw")
	gtk(&gskRenderNodeUnref, "gsk_render_node_unref")
	gtk(&gskTransformTranslate, "gsk_transform_translate")
	gtk(&gtkEventControllerLegacyNew, "gtk_event_controller_legacy_new")
	gtk(&gdkEventGetEventType, "gdk_event_get_event_type")
	gtk(&gdkEventGetPosition, "gdk_event_get_position")
	gtk(&gdkEventGetModifierState, "gdk_event_get_modifier_state")
	gtk(&gdkEventGetDevice, "gdk_event_get_device")
	gtk(&gdkButtonEventGetButton, "gdk_button_event_get_button")
	gtk(&gdkScrollEventGetDirection, "gdk_scroll_event_get_direction")
	gtk(&gdkScrollEventGetDeltas, "gdk_scroll_event_get_deltas")
	if _, err := purego.Dlsym(gtkLib, "gdk_scroll_event_get_unit"); err == nil {
		gtk(&gdkScrollEventGetUnit, "gdk_scroll_event_get_unit")
	}
}
