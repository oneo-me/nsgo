//go:build linux && !android

package linux

import (
	"fmt"
	"sync"
	"sync/atomic"
	"unsafe"

	"github.com/ebitengine/purego"
)

// The libraries NSGo needs. GTK 4 merged GDK into itself; Cairo and Pango
// are reached directly for drawing and text layout.
var (
	gtkLib        uintptr
	glibLib       uintptr
	gobjectLib    uintptr
	gioLib        uintptr
	pangoLib      uintptr
	pangocairoLib uintptr
	cairoLib      uintptr
)

var libsOnce sync.Once
var libsErr error

// openLibs loads the libraries, reporting which one is missing.
func openLibs() error {
	libsOnce.Do(func() {
		load := func(name string, candidates ...string) (uintptr, error) {
			var last error
			for _, candidate := range candidates {
				h, err := purego.Dlopen(candidate, purego.RTLD_NOW|purego.RTLD_GLOBAL)
				if err == nil {
					return h, nil
				}
				last = err
			}
			return 0, fmt.Errorf("%s: %w", name, last)
		}
		var err error
		if gtkLib, err = load("GTK 4", "libgtk-4.so.1", "libgtk-4.so.0", "libgtk-4.so"); err != nil {
			libsErr = err
			return
		}
		if glibLib, err = load("GLib", "libglib-2.0.so.0", "libglib-2.0.so"); err != nil {
			libsErr = err
			return
		}
		if gobjectLib, err = load("GObject", "libgobject-2.0.so.0", "libgobject-2.0.so"); err != nil {
			libsErr = err
			return
		}
		if gioLib, err = load("GIO", "libgio-2.0.so.0", "libgio-2.0.so"); err != nil {
			libsErr = err
			return
		}
		if pangoLib, err = load("Pango", "libpango-1.0.so.0", "libpango-1.0.so"); err != nil {
			libsErr = err
			return
		}
		if pangocairoLib, err = load("PangoCairo", "libpangocairo-1.0.so.0", "libpangocairo-1.0.so"); err != nil {
			libsErr = err
			return
		}
		if cairoLib, err = load("Cairo", "libcairo.so.2", "libcairo.so"); err != nil {
			libsErr = err
			return
		}
		func() {
			defer func() {
				if r := recover(); r != nil {
					libsErr = fmt.Errorf("GTK 4.14, Pango and Cairo are required: %v", r)
				}
			}()
			bind()
		}()
	})
	return libsErr
}

// The bound functions. Their types are the C signatures purego converts
// Go values to, so pointer arguments are uintptr and C strings are *byte.
var (
	// GLib.
	gMainLoopNew       func(uintptr, int32) uintptr
	gMainLoopRun       func(uintptr)
	gMainLoopQuit      func(uintptr)
	gMainLoopUnref     func(uintptr)
	gTimeoutAdd        func(uint32, uintptr, uintptr) uint32
	gSourceRemove      func(uint32)
	gIdleAdd           func(uintptr, uintptr) uint32
	gFree              func(uintptr)
	gErrorFree         func(uintptr)
	gObjectUnref       func(uintptr)
	gObjectRefSink     func(uintptr) uintptr
	gSignalConnectData func(uintptr, *byte, uintptr, uintptr, uintptr, uint32) uint64
	gSignalEmitByName  func(uintptr, *byte, ...any)
	gObjectGet         func(uintptr, ...any)
	gStrfreev          func(uintptr)
	gVariantNewString  func(*byte) uintptr
	gVariantNewBoolean func(int32) uintptr
	gVariantUnref      func(uintptr)

	// GTK.
	gtkInitCheck                           func() int32
	gtkWindowNew                           func() uintptr
	gtkWindowSetTitle                      func(uintptr, *byte)
	gtkWindowGetTitle                      func(uintptr) uintptr
	gtkWindowSetDefaultSize                func(uintptr, int32, int32)
	gtkWindowSetResizable                  func(uintptr, int32)
	gtkWindowSetChild                      func(uintptr, uintptr)
	gtkWindowSetFocus                      func(uintptr, uintptr)
	gtkWindowGetFocus                      func(uintptr) uintptr
	gtkWindowPresent                       func(uintptr)
	gtkWindowDestroy                       func(uintptr)
	gtkWindowClose                         func(uintptr)
	gtkWindowSetDecorated                  func(uintptr, int32)
	gtkWindowMinimize                      func(uintptr)
	gtkWindowMaximize                      func(uintptr)
	gtkWindowUnmaximize                    func(uintptr)
	gtkWindowFullscreen                    func(uintptr)
	gtkWindowUnfullscreen                  func(uintptr)
	gtkWindowIsFullscreen                  func(uintptr) int32
	gtkWindowIsMaximized                   func(uintptr) int32
	gtkWidgetShow                          func(uintptr)
	gtkWidgetSetVisible                    func(uintptr, int32)
	gtkWidgetAddController                 func(uintptr, uintptr)
	gtkWidgetQueueDraw                     func(uintptr)
	gtkWidgetQueueAllocate                 func(uintptr)
	gtkWidgetGetMapped                     func(uintptr) int32
	gtkWidgetGrabFocus                     func(uintptr)
	gtkWidgetGetRoot                       func(uintptr) uintptr
	gtkWidgetSetFocusable                  func(uintptr, int32)
	gtkWidgetGetWidth                      func(uintptr) int32
	gtkWidgetGetHeight                     func(uintptr) int32
	gtkWidgetGetNative                     func(uintptr) uintptr
	gtkWidgetGetScaleFactor                func(uintptr) int32
	gtkWidgetGetStyleContext               func(uintptr) uintptr
	gtkWidgetGetParent                     func(uintptr) uintptr
	gtkWidgetHasFocus                      func(uintptr) int32
	gtkWidgetActivateAction                func(uintptr, *byte, uintptr) int32
	gtkWidgetGetDefaultDirection           func() int32
	gtkWidgetSetSizeRequest                func(uintptr, int32, int32)
	gtkWidgetSetHexpand                    func(uintptr, int32)
	gtkWidgetSetVexpand                    func(uintptr, int32)
	gtkWidgetSetHalign                     func(uintptr, int32)
	gtkWidgetSetValign                     func(uintptr, int32)
	gtkWidgetSetOverflow                   func(uintptr, int32)
	gtkWidgetSetSensitive                  func(uintptr, int32)
	gtkWidgetSetCursorFromName             func(uintptr, *byte)
	gtkWidgetActivate                      func(uintptr) int32
	gtkNativeGetSurface                    func(uintptr) uintptr
	gtkBoxNew                              func(int32, int32) uintptr
	gtkBoxAppend                           func(uintptr, uintptr)
	gtkBoxPrepend                          func(uintptr, uintptr)
	gtkBoxRemove                           func(uintptr, uintptr)
	gtkEntryNew                            func() uintptr
	gtkEditableSetText                     func(uintptr, *byte)
	gtkEditableGetText                     func(uintptr) uintptr
	gtkEditableSetEditable                 func(uintptr, int32)
	gtkEditableSelectRegion                func(uintptr, int32, int32)
	gtkEditableDeleteSelection             func(uintptr) int32
	gtkEntrySetPlaceholderText             func(uintptr, *byte)
	gtkEntrySetVisibility                  func(uintptr, int32)
	gtkEntrySetAlignment                   func(uintptr, float32)
	gtkGestureClickNew                     func() uintptr
	gtkGestureSingleSetButton              func(uintptr, uint32)
	gtkGestureDragNew                      func() uintptr
	gtkGestureZoomNew                      func() uintptr
	gtkGestureRotateNew                    func() uintptr
	gtkGestureSingleGetCurrentButton       func(uintptr) uint32
	gtkEventControllerMotionNew            func() uintptr
	gtkEventControllerKeyNew               func() uintptr
	gtkEventControllerScrollNew            func(int32) uintptr
	gtkEventControllerScrollSetFlags       func(uintptr, int32)
	gtkEventControllerFocusNew             func() uintptr
	gtkEventControllerSetPropagationPhase  func(uintptr, int32)
	gtkEventControllerGetCurrentEventState func(uintptr) uint32
	gtkCssProviderNew                      func() uintptr
	gtkCssProviderLoadFromString           func(uintptr, *byte)
	gtkStyleContextAddProvider             func(uintptr, uintptr, uint32)
	gtkSettingsGetDefault                  func() uintptr
	gtkAlertDialogNew                      func(format *byte) uintptr
	gtkAlertDialogSetMessage               func(uintptr, *byte)
	gtkAlertDialogSetDetail                func(uintptr, *byte)
	gtkAlertDialogSetButtons               func(uintptr, uintptr)
	gtkAlertDialogSetCancelButton          func(uintptr, int32)
	gtkAlertDialogSetDefaultButton         func(uintptr, int32)
	gtkAlertDialogChoose                   func(uintptr, uintptr, uintptr, uintptr, uintptr)
	gtkAlertDialogChooseFinish             func(uintptr, uintptr, uintptr) int32
	gtkPopoverMenuBarNewFromModel          func(uintptr) uintptr
	gtkWindowGetDefaultSize                func(uintptr, *int32, *int32)

	// GDK, in libgtk-4.
	gdkDisplayGetDefault       func() uintptr
	gdkDisplayGetClipboard     func(uintptr) uintptr
	gdkClipboardSetText        func(uintptr, *byte)
	gdkClipboardReadTextAsync  func(uintptr, uintptr, uintptr, uintptr)
	gdkClipboardReadTextFinish func(uintptr, uintptr, uintptr) uintptr
	gdkSurfaceGetScale         func(uintptr) float64
	gdkToplevelBeginMove       func(uintptr, uintptr, int32, float64, float64, uint32)
	gtkGestureGetDevice        func(uintptr) uintptr

	// GIO.
	gMenuNew                   func() uintptr
	gMenuAppend                func(uintptr, *byte, *byte)
	gMenuAppendSubmenu         func(uintptr, *byte, uintptr)
	gMenuAppendItem            func(uintptr, uintptr)
	gMenuAppendSection         func(uintptr, *byte, uintptr)
	gMenuItemNew               func(*byte, *byte) uintptr
	gSimpleActionNew           func(*byte, uintptr) uintptr
	gSimpleActionNewStateful   func(*byte, uintptr, uintptr) uintptr
	gSimpleActionSetEnabled    func(uintptr, int32)
	gActionMapAddAction        func(uintptr, uintptr)
	gSimpleActionGroupNew      func() uintptr
	gtkWidgetInsertActionGroup func(uintptr, *byte, uintptr)
	gActionMapRemoveAction     func(uintptr, *byte)

	// Pango.
	pangoCairoFontMapNew           func() uintptr
	pangoFontMapCreateContext      func(uintptr) uintptr
	pangoFontMapUnref              func(uintptr)
	pangoLayoutNew                 func(uintptr) uintptr
	pangoLayoutSetAutoDir          func(uintptr, bool)
	pangoLayoutGetContext          func(uintptr) uintptr
	pangoContextSetBaseDir         func(uintptr, int32)
	pangoLayoutSetText             func(uintptr, *byte, int32)
	pangoLayoutSetWidth            func(uintptr, int32)
	pangoLayoutSetHeight           func(uintptr, int32)
	pangoLayoutSetWrap             func(uintptr, int32)
	pangoLayoutSetEllipsize        func(uintptr, int32)
	pangoLayoutSetAlignment        func(uintptr, int32)
	pangoLayoutSetJustify          func(uintptr, bool)
	pangoLayoutSetSpacing          func(uintptr, int32)
	pangoLayoutSetFontDescription  func(uintptr, uintptr)
	pangoLayoutSetAttributes       func(uintptr, uintptr)
	pangoLayoutContextChanged      func(uintptr)
	pangoLayoutGetPixelSize        func(uintptr, *int32, *int32)
	pangoLayoutGetLineCount        func(uintptr) int32
	pangoLayoutGetBaseline         func(uintptr) int32
	pangoLayoutXyToIndex           func(uintptr, int32, int32, *int32, *int32) int32
	pangoLayoutIndexToPos          func(uintptr, int32, *pangoRectangle)
	pangoLayoutIterNextCluster     func(uintptr) int32
	pangoLayoutIterGetIndex        func(uintptr) int32
	pangoLayoutIterGetClusterExt   func(uintptr, *pangoRectangle, *pangoRectangle)
	pangoFontDescriptionFromString func(*byte) uintptr
	pangoFontDescriptionNew        func() uintptr
	pangoFontDescriptionFree       func(uintptr)
	pangoFontDescriptionSetFamily  func(uintptr, *byte)
	pangoFontDescriptionSetSize    func(uintptr, int32)
	pangoFontDescriptionSetWeight  func(uintptr, int32)
	pangoFontDescriptionSetStyle   func(uintptr, int32)
	pangoAttrListNew               func() uintptr
	pangoAttrListInsert            func(uintptr, uintptr)
	pangoAttrListUnref             func(uintptr)
	pangoAttrFamilyNew             func(*byte) uintptr
	pangoAttrSizeNew               func(int32) uintptr
	pangoAttrWeightNew             func(int32) uintptr
	pangoAttrStyleNew              func(int32) uintptr
	pangoAttrForegroundNew         func(uint16, uint16, uint16) uintptr
	pangoAttrLetterSpacingNew      func(int32) uintptr
	pangoAttrLineHeightNew         func(int32) uintptr
	pangoAttrLineHeightAbsoluteNew func(int32) uintptr
	pangoAttrUnderlineNew          func(int32) uintptr
	pangoAttrStrikethroughNew      func(int32) uintptr
	pangoAttrUnderlineColorNew     func(uint16, uint16, uint16) uintptr
	pangoAttrStrikethroughColorNew func(uint16, uint16, uint16) uintptr
	pangoAttrShapeNew              func(*pangoRectangle, *pangoRectangle) uintptr
	pangoAttrFontFeaturesNew       func(*byte) uintptr
	pangoContextGetMetrics         func(context, desc, language uintptr) uintptr
	pangoFontMetricsUnref          func(uintptr)
	pangoFontMetricsUnderlinePos   func(uintptr) int32
	pangoFontMetricsUnderlineThick func(uintptr) int32
	pangoFontMetricsStrikePos      func(uintptr) int32
	pangoFontMetricsStrikeThick    func(uintptr) int32
	pangoLayoutGetExtents          func(uintptr, *pangoRectangle, *pangoRectangle)
	pangoLayoutGetIter             func(uintptr) uintptr
	pangoLayoutIterFree            func(uintptr)
	pangoLayoutIterNextLine        func(uintptr) int32
	pangoLayoutIterNextRun         func(uintptr) int32
	pangoLayoutIterGetLine         func(uintptr) uintptr
	pangoLayoutIterGetRun          func(uintptr) uintptr
	pangoLayoutIterGetBaseline     func(uintptr) int32
	pangoLayoutIterGetRunExtents   func(uintptr, *pangoRectangle, *pangoRectangle)
	pangoLayoutIterGetLineExtents  func(uintptr, *pangoRectangle, *pangoRectangle)
	pangoLayoutLineGetXRanges      func(uintptr, int32, int32, *uintptr, *int32)
	pangoCairoCreateLayout         func(uintptr) uintptr
	pangoCairoShowLayout           func(uintptr, uintptr)
	pangoCairoUpdateLayout         func(uintptr, uintptr)

	// Cairo.
	cairoImageSurfaceCreate    func(int32, int32, int32) uintptr
	cairoCreate                func(uintptr) uintptr
	cairoDestroy               func(uintptr)
	cairoSurfaceDestroy        func(uintptr)
	cairoSurfaceFlush          func(uintptr)
	cairoSurfaceMarkDirty      func(uintptr)
	cairoImageSurfaceGetData   func(uintptr) uintptr
	cairoImageSurfaceGetStride func(uintptr) int32
	cairoSetSourceRGBA         func(uintptr, float64, float64, float64, float64)
	cairoPaint                 func(uintptr)
	cairoPaintWithAlpha        func(uintptr, float64)
	cairoRectangle             func(uintptr, float64, float64, float64, float64)
	cairoArc                   func(uintptr, float64, float64, float64, float64, float64)
	cairoArcNegative           func(uintptr, float64, float64, float64, float64, float64)
	cairoSetFillRule           func(uintptr, int32)
	cairoIdentityMatrix        func(uintptr)
	cairoNewSubPath            func(uintptr)
	cairoNewPath               func(uintptr)
	cairoGetCurrentPoint       func(uintptr, *float64, *float64)
	cairoClosePath             func(uintptr)
	cairoFill                  func(uintptr)
	cairoFillPreserve          func(uintptr)
	cairoStroke                func(uintptr)
	cairoSave                  func(uintptr)
	cairoRestore               func(uintptr)
	cairoTranslate             func(uintptr, float64, float64)
	cairoScale                 func(uintptr, float64, float64)
	cairoRotate                func(uintptr, float64)
	cairoClip                  func(uintptr)
	cairoMoveTo                func(uintptr, float64, float64)
	cairoLineTo                func(uintptr, float64, float64)
	cairoCurveTo               func(uintptr, float64, float64, float64, float64, float64, float64)
	cairoSetLineWidth          func(uintptr, float64)
	cairoSetLineCap            func(uintptr, int32)
	cairoSetLineJoin           func(uintptr, int32)
	cairoPushGroup             func(uintptr)
	cairoPopGroupToSource      func(uintptr)
	cairoSetOperator           func(uintptr, int32)
	cairoSetSourceSurface      func(uintptr, uintptr, float64, float64)
	cairoSurfaceWriteToPNG     func(uintptr, *byte) int32
)

// newCallback returns a C function pointer for the package-level
// callback fn. purego keeps the function for the process's life.
func newCallback(fn any) uintptr { return purego.NewCallback(fn) }

// cPointer turns a C address into a Go pointer. It reads the uintptr
// through an unsafe.Pointer so the conversion is explicit to go vet.
func cPointer(p uintptr) unsafe.Pointer {
	return *(*unsafe.Pointer)(unsafe.Pointer(&p))
}

// Windows and views travel to C callbacks as integer handles, so no Go
// pointer is handed to C.
var (
	windowHandles sync.Map // uintptr -> *window
	nextHandle    atomic.Uintptr
)

// registerWindow gives w a handle C callbacks find it by.
func registerWindow(w *window) uintptr {
	h := nextHandle.Add(1)
	windowHandles.Store(h, w)
	return h
}

// windowOf returns the window a handler's data names, or nil.
func windowOf(data uintptr) *window {
	v, ok := windowHandles.Load(data)
	if !ok {
		return nil
	}
	return v.(*window)
}

// gSignalConnect connects fn to instance's signal. The callback must be
// one of the package-level functions passed through newCallback, so
// purego keeps it for the process's life.
func gSignalConnect(instance uintptr, signal string, fn uintptr, data uintptr) uint64 {
	return gSignalConnectData(instance, cstr(signal), fn, data, 0, 0)
}

// cstr returns s as a NUL-terminated C string.
func cstr(s string) *byte {
	b := append([]byte(s), 0)
	return &b[0]
}

// cStrings is a NULL-terminated array of C strings and the Go memory
// holding them: the array keeps only their addresses, which the collector
// does not follow, so bufs keeps the strings alive while it is reachable.
type cStrings struct {
	ptrs []uintptr
	bufs [][]byte
}

// stringArray returns ss as C strings. Callers keep the result reachable,
// with runtime.KeepAlive, while the C side reads it.
func stringArray(ss []string) *cStrings {
	c := &cStrings{ptrs: make([]uintptr, len(ss)+1), bufs: make([][]byte, len(ss))}
	for i, s := range ss {
		c.bufs[i] = append([]byte(s), 0)
		c.ptrs[i] = uintptr(unsafe.Pointer(&c.bufs[i][0]))
	}
	return c
}

// array is the address of the array's first element.
func (c *cStrings) array() uintptr { return uintptr(unsafe.Pointer(&c.ptrs[0])) }

// bind registers every symbol once the libraries are open.
func bind() {
	// GLib.
	purego.RegisterLibFunc(&gMainLoopNew, glibLib, "g_main_loop_new")
	purego.RegisterLibFunc(&gMainLoopRun, glibLib, "g_main_loop_run")
	purego.RegisterLibFunc(&gMainLoopQuit, glibLib, "g_main_loop_quit")
	purego.RegisterLibFunc(&gMainLoopUnref, glibLib, "g_main_loop_unref")
	purego.RegisterLibFunc(&gTimeoutAdd, glibLib, "g_timeout_add")
	purego.RegisterLibFunc(&gSourceRemove, glibLib, "g_source_remove")
	purego.RegisterLibFunc(&gIdleAdd, glibLib, "g_idle_add")
	purego.RegisterLibFunc(&gFree, glibLib, "g_free")
	purego.RegisterLibFunc(&gErrorFree, glibLib, "g_error_free")
	purego.RegisterLibFunc(&gStrfreev, glibLib, "g_strfreev")
	purego.RegisterLibFunc(&gVariantNewString, glibLib, "g_variant_new_string")
	purego.RegisterLibFunc(&gVariantNewBoolean, glibLib, "g_variant_new_boolean")
	purego.RegisterLibFunc(&gVariantUnref, glibLib, "g_variant_unref")
	purego.RegisterLibFunc(&gObjectUnref, gobjectLib, "g_object_unref")
	purego.RegisterLibFunc(&gObjectRefSink, gobjectLib, "g_object_ref_sink")
	purego.RegisterLibFunc(&gSignalConnectData, gobjectLib, "g_signal_connect_data")
	purego.RegisterLibFunc(&gSignalEmitByName, gobjectLib, "g_signal_emit_by_name")
	purego.RegisterLibFunc(&gObjectGet, gobjectLib, "g_object_get")

	// GTK.
	purego.RegisterLibFunc(&gtkInitCheck, gtkLib, "gtk_init_check")
	purego.RegisterLibFunc(&gtkWindowNew, gtkLib, "gtk_window_new")
	purego.RegisterLibFunc(&gtkWindowSetTitle, gtkLib, "gtk_window_set_title")
	purego.RegisterLibFunc(&gtkWindowGetTitle, gtkLib, "gtk_window_get_title")
	purego.RegisterLibFunc(&gtkWindowSetDefaultSize, gtkLib, "gtk_window_set_default_size")
	purego.RegisterLibFunc(&gtkWindowSetResizable, gtkLib, "gtk_window_set_resizable")
	purego.RegisterLibFunc(&gtkWindowGetDefaultSize, gtkLib, "gtk_window_get_default_size")
	purego.RegisterLibFunc(&gtkWindowSetChild, gtkLib, "gtk_window_set_child")
	purego.RegisterLibFunc(&gtkWindowSetFocus, gtkLib, "gtk_window_set_focus")
	purego.RegisterLibFunc(&gtkWindowGetFocus, gtkLib, "gtk_window_get_focus")
	purego.RegisterLibFunc(&gtkWindowPresent, gtkLib, "gtk_window_present")
	purego.RegisterLibFunc(&gtkWindowDestroy, gtkLib, "gtk_window_destroy")
	purego.RegisterLibFunc(&gtkWindowClose, gtkLib, "gtk_window_close")
	purego.RegisterLibFunc(&gtkWindowSetDecorated, gtkLib, "gtk_window_set_decorated")
	purego.RegisterLibFunc(&gtkWindowMinimize, gtkLib, "gtk_window_minimize")
	purego.RegisterLibFunc(&gtkWindowMaximize, gtkLib, "gtk_window_maximize")
	purego.RegisterLibFunc(&gtkWindowUnmaximize, gtkLib, "gtk_window_unmaximize")
	purego.RegisterLibFunc(&gtkWindowFullscreen, gtkLib, "gtk_window_fullscreen")
	purego.RegisterLibFunc(&gtkWindowUnfullscreen, gtkLib, "gtk_window_unfullscreen")
	purego.RegisterLibFunc(&gtkWindowIsFullscreen, gtkLib, "gtk_window_is_fullscreen")
	purego.RegisterLibFunc(&gtkWindowIsMaximized, gtkLib, "gtk_window_is_maximized")
	purego.RegisterLibFunc(&gtkWidgetShow, gtkLib, "gtk_widget_show")
	purego.RegisterLibFunc(&gtkWidgetSetVisible, gtkLib, "gtk_widget_set_visible")
	purego.RegisterLibFunc(&gtkWidgetAddController, gtkLib, "gtk_widget_add_controller")
	purego.RegisterLibFunc(&gtkWidgetQueueDraw, gtkLib, "gtk_widget_queue_draw")
	purego.RegisterLibFunc(&gtkWidgetQueueAllocate, gtkLib, "gtk_widget_queue_allocate")
	purego.RegisterLibFunc(&gtkWidgetGetMapped, gtkLib, "gtk_widget_get_mapped")
	purego.RegisterLibFunc(&gtkWidgetGrabFocus, gtkLib, "gtk_widget_grab_focus")
	purego.RegisterLibFunc(&gtkWidgetGetRoot, gtkLib, "gtk_widget_get_root")
	purego.RegisterLibFunc(&gtkWidgetSetFocusable, gtkLib, "gtk_widget_set_focusable")
	purego.RegisterLibFunc(&gtkWidgetGetWidth, gtkLib, "gtk_widget_get_width")
	purego.RegisterLibFunc(&gtkWidgetGetHeight, gtkLib, "gtk_widget_get_height")
	purego.RegisterLibFunc(&gtkWidgetGetNative, gtkLib, "gtk_widget_get_native")
	purego.RegisterLibFunc(&gtkWidgetGetScaleFactor, gtkLib, "gtk_widget_get_scale_factor")
	purego.RegisterLibFunc(&gtkWidgetGetStyleContext, gtkLib, "gtk_widget_get_style_context")
	purego.RegisterLibFunc(&gtkWidgetGetParent, gtkLib, "gtk_widget_get_parent")
	purego.RegisterLibFunc(&gtkWidgetHasFocus, gtkLib, "gtk_widget_has_focus")
	purego.RegisterLibFunc(&gtkWidgetActivateAction, gtkLib, "gtk_widget_activate_action")
	purego.RegisterLibFunc(&gtkWidgetGetDefaultDirection, gtkLib, "gtk_widget_get_default_direction")
	purego.RegisterLibFunc(&gtkWidgetSetSizeRequest, gtkLib, "gtk_widget_set_size_request")
	purego.RegisterLibFunc(&gtkWidgetSetHexpand, gtkLib, "gtk_widget_set_hexpand")
	purego.RegisterLibFunc(&gtkWidgetSetVexpand, gtkLib, "gtk_widget_set_vexpand")
	purego.RegisterLibFunc(&gtkWidgetSetHalign, gtkLib, "gtk_widget_set_halign")
	purego.RegisterLibFunc(&gtkWidgetSetValign, gtkLib, "gtk_widget_set_valign")
	purego.RegisterLibFunc(&gtkWidgetSetOverflow, gtkLib, "gtk_widget_set_overflow")
	purego.RegisterLibFunc(&gtkWidgetSetSensitive, gtkLib, "gtk_widget_set_sensitive")
	purego.RegisterLibFunc(&gtkWidgetSetCursorFromName, gtkLib, "gtk_widget_set_cursor_from_name")
	purego.RegisterLibFunc(&gtkWidgetActivate, gtkLib, "gtk_widget_activate")
	purego.RegisterLibFunc(&gtkNativeGetSurface, gtkLib, "gtk_native_get_surface")
	purego.RegisterLibFunc(&gtkBoxNew, gtkLib, "gtk_box_new")
	purego.RegisterLibFunc(&gtkBoxAppend, gtkLib, "gtk_box_append")
	purego.RegisterLibFunc(&gtkBoxPrepend, gtkLib, "gtk_box_prepend")
	purego.RegisterLibFunc(&gtkBoxRemove, gtkLib, "gtk_box_remove")
	purego.RegisterLibFunc(&gtkEntryNew, gtkLib, "gtk_entry_new")
	purego.RegisterLibFunc(&gtkEditableSetText, gtkLib, "gtk_editable_set_text")
	purego.RegisterLibFunc(&gtkEditableGetText, gtkLib, "gtk_editable_get_text")
	purego.RegisterLibFunc(&gtkEditableSetEditable, gtkLib, "gtk_editable_set_editable")
	purego.RegisterLibFunc(&gtkEditableSelectRegion, gtkLib, "gtk_editable_select_region")
	purego.RegisterLibFunc(&gtkEditableDeleteSelection, gtkLib, "gtk_editable_delete_selection")
	purego.RegisterLibFunc(&gtkEntrySetPlaceholderText, gtkLib, "gtk_entry_set_placeholder_text")
	purego.RegisterLibFunc(&gtkEntrySetVisibility, gtkLib, "gtk_entry_set_visibility")
	purego.RegisterLibFunc(&gtkEntrySetAlignment, gtkLib, "gtk_entry_set_alignment")
	purego.RegisterLibFunc(&gtkGestureClickNew, gtkLib, "gtk_gesture_click_new")
	purego.RegisterLibFunc(&gtkGestureSingleSetButton, gtkLib, "gtk_gesture_single_set_button")
	purego.RegisterLibFunc(&gtkGestureDragNew, gtkLib, "gtk_gesture_drag_new")
	purego.RegisterLibFunc(&gtkGestureZoomNew, gtkLib, "gtk_gesture_zoom_new")
	purego.RegisterLibFunc(&gtkGestureRotateNew, gtkLib, "gtk_gesture_rotate_new")
	purego.RegisterLibFunc(&gtkGestureSingleGetCurrentButton, gtkLib, "gtk_gesture_single_get_current_button")
	purego.RegisterLibFunc(&gtkEventControllerMotionNew, gtkLib, "gtk_event_controller_motion_new")
	purego.RegisterLibFunc(&gtkEventControllerKeyNew, gtkLib, "gtk_event_controller_key_new")
	purego.RegisterLibFunc(&gtkEventControllerScrollNew, gtkLib, "gtk_event_controller_scroll_new")
	purego.RegisterLibFunc(&gtkEventControllerScrollSetFlags, gtkLib, "gtk_event_controller_scroll_set_flags")
	purego.RegisterLibFunc(&gtkEventControllerFocusNew, gtkLib, "gtk_event_controller_focus_new")
	purego.RegisterLibFunc(&gtkEventControllerSetPropagationPhase, gtkLib, "gtk_event_controller_set_propagation_phase")
	purego.RegisterLibFunc(&gtkEventControllerGetCurrentEventState, gtkLib, "gtk_event_controller_get_current_event_state")
	purego.RegisterLibFunc(&gtkCssProviderNew, gtkLib, "gtk_css_provider_new")
	purego.RegisterLibFunc(&gtkCssProviderLoadFromString, gtkLib, "gtk_css_provider_load_from_string")
	purego.RegisterLibFunc(&gtkStyleContextAddProvider, gtkLib, "gtk_style_context_add_provider")
	purego.RegisterLibFunc(&gtkSettingsGetDefault, gtkLib, "gtk_settings_get_default")
	purego.RegisterLibFunc(&gtkAlertDialogNew, gtkLib, "gtk_alert_dialog_new")
	purego.RegisterLibFunc(&gtkAlertDialogSetMessage, gtkLib, "gtk_alert_dialog_set_message")
	purego.RegisterLibFunc(&gtkAlertDialogSetDetail, gtkLib, "gtk_alert_dialog_set_detail")
	purego.RegisterLibFunc(&gtkAlertDialogSetButtons, gtkLib, "gtk_alert_dialog_set_buttons")
	purego.RegisterLibFunc(&gtkAlertDialogSetCancelButton, gtkLib, "gtk_alert_dialog_set_cancel_button")
	purego.RegisterLibFunc(&gtkAlertDialogSetDefaultButton, gtkLib, "gtk_alert_dialog_set_default_button")
	purego.RegisterLibFunc(&gtkAlertDialogChoose, gtkLib, "gtk_alert_dialog_choose")
	purego.RegisterLibFunc(&gtkAlertDialogChooseFinish, gtkLib, "gtk_alert_dialog_choose_finish")
	purego.RegisterLibFunc(&gtkPopoverMenuBarNewFromModel, gtkLib, "gtk_popover_menu_bar_new_from_model")
	purego.RegisterLibFunc(&gtkGestureGetDevice, gtkLib, "gtk_gesture_get_device")

	// GDK.
	purego.RegisterLibFunc(&gdkDisplayGetDefault, gtkLib, "gdk_display_get_default")
	purego.RegisterLibFunc(&gdkDisplayGetClipboard, gtkLib, "gdk_display_get_clipboard")
	purego.RegisterLibFunc(&gdkClipboardSetText, gtkLib, "gdk_clipboard_set_text")
	purego.RegisterLibFunc(&gdkClipboardReadTextAsync, gtkLib, "gdk_clipboard_read_text_async")
	purego.RegisterLibFunc(&gdkClipboardReadTextFinish, gtkLib, "gdk_clipboard_read_text_finish")
	purego.RegisterLibFunc(&gdkSurfaceGetScale, gtkLib, "gdk_surface_get_scale")
	purego.RegisterLibFunc(&gdkToplevelBeginMove, gtkLib, "gdk_toplevel_begin_move")

	// GIO.
	purego.RegisterLibFunc(&gMenuNew, gioLib, "g_menu_new")
	purego.RegisterLibFunc(&gMenuAppend, gioLib, "g_menu_append")
	purego.RegisterLibFunc(&gMenuAppendSubmenu, gioLib, "g_menu_append_submenu")
	purego.RegisterLibFunc(&gMenuAppendItem, gioLib, "g_menu_append_item")
	purego.RegisterLibFunc(&gMenuAppendSection, gioLib, "g_menu_append_section")
	purego.RegisterLibFunc(&gMenuItemNew, gioLib, "g_menu_item_new")
	purego.RegisterLibFunc(&gSimpleActionNew, gioLib, "g_simple_action_new")
	purego.RegisterLibFunc(&gSimpleActionNewStateful, gioLib, "g_simple_action_new_stateful")
	purego.RegisterLibFunc(&gSimpleActionSetEnabled, gioLib, "g_simple_action_set_enabled")
	purego.RegisterLibFunc(&gActionMapAddAction, gioLib, "g_action_map_add_action")
	purego.RegisterLibFunc(&gSimpleActionGroupNew, gioLib, "g_simple_action_group_new")
	purego.RegisterLibFunc(&gtkWidgetInsertActionGroup, gtkLib, "gtk_widget_insert_action_group")
	purego.RegisterLibFunc(&gActionMapRemoveAction, gioLib, "g_action_map_remove_action")

	// Pango.
	purego.RegisterLibFunc(&pangoCairoFontMapNew, pangocairoLib, "pango_cairo_font_map_new")
	purego.RegisterLibFunc(&pangoFontMapCreateContext, pangoLib, "pango_font_map_create_context")
	purego.RegisterLibFunc(&pangoFontMapUnref, pangoLib, "g_object_unref") // PangoFontMap is a GObject
	purego.RegisterLibFunc(&pangoLayoutNew, pangoLib, "pango_layout_new")
	purego.RegisterLibFunc(&pangoLayoutSetAutoDir, pangoLib, "pango_layout_set_auto_dir")
	purego.RegisterLibFunc(&pangoLayoutGetContext, pangoLib, "pango_layout_get_context")
	purego.RegisterLibFunc(&pangoContextSetBaseDir, pangoLib, "pango_context_set_base_dir")
	purego.RegisterLibFunc(&pangoLayoutSetText, pangoLib, "pango_layout_set_text")
	purego.RegisterLibFunc(&pangoLayoutSetWidth, pangoLib, "pango_layout_set_width")
	purego.RegisterLibFunc(&pangoLayoutSetHeight, pangoLib, "pango_layout_set_height")
	purego.RegisterLibFunc(&pangoLayoutSetWrap, pangoLib, "pango_layout_set_wrap")
	purego.RegisterLibFunc(&pangoLayoutSetEllipsize, pangoLib, "pango_layout_set_ellipsize")
	purego.RegisterLibFunc(&pangoLayoutSetAlignment, pangoLib, "pango_layout_set_alignment")
	purego.RegisterLibFunc(&pangoLayoutSetJustify, pangoLib, "pango_layout_set_justify")
	purego.RegisterLibFunc(&pangoLayoutSetSpacing, pangoLib, "pango_layout_set_spacing")
	purego.RegisterLibFunc(&pangoLayoutSetFontDescription, pangoLib, "pango_layout_set_font_description")
	purego.RegisterLibFunc(&pangoLayoutSetAttributes, pangoLib, "pango_layout_set_attributes")
	purego.RegisterLibFunc(&pangoLayoutContextChanged, pangoLib, "pango_layout_context_changed")
	purego.RegisterLibFunc(&pangoLayoutGetPixelSize, pangoLib, "pango_layout_get_pixel_size")
	purego.RegisterLibFunc(&pangoLayoutGetLineCount, pangoLib, "pango_layout_get_line_count")
	purego.RegisterLibFunc(&pangoLayoutGetBaseline, pangoLib, "pango_layout_get_baseline")
	purego.RegisterLibFunc(&pangoLayoutXyToIndex, pangoLib, "pango_layout_xy_to_index")
	purego.RegisterLibFunc(&pangoLayoutIndexToPos, pangoLib, "pango_layout_index_to_pos")
	purego.RegisterLibFunc(&pangoLayoutIterNextCluster, pangoLib, "pango_layout_iter_next_cluster")
	purego.RegisterLibFunc(&pangoLayoutIterGetIndex, pangoLib, "pango_layout_iter_get_index")
	purego.RegisterLibFunc(&pangoLayoutIterGetClusterExt, pangoLib, "pango_layout_iter_get_cluster_extents")
	purego.RegisterLibFunc(&pangoFontDescriptionFromString, pangoLib, "pango_font_description_from_string")
	purego.RegisterLibFunc(&pangoFontDescriptionNew, pangoLib, "pango_font_description_new")
	purego.RegisterLibFunc(&pangoFontDescriptionFree, pangoLib, "pango_font_description_free")
	purego.RegisterLibFunc(&pangoFontDescriptionSetFamily, pangoLib, "pango_font_description_set_family")
	purego.RegisterLibFunc(&pangoFontDescriptionSetSize, pangoLib, "pango_font_description_set_size")
	purego.RegisterLibFunc(&pangoFontDescriptionSetWeight, pangoLib, "pango_font_description_set_weight")
	purego.RegisterLibFunc(&pangoFontDescriptionSetStyle, pangoLib, "pango_font_description_set_style")
	purego.RegisterLibFunc(&pangoAttrListNew, pangoLib, "pango_attr_list_new")
	purego.RegisterLibFunc(&pangoAttrListInsert, pangoLib, "pango_attr_list_insert")
	purego.RegisterLibFunc(&pangoAttrListUnref, pangoLib, "pango_attr_list_unref")
	purego.RegisterLibFunc(&pangoAttrFamilyNew, pangoLib, "pango_attr_family_new")
	purego.RegisterLibFunc(&pangoAttrSizeNew, pangoLib, "pango_attr_size_new")
	purego.RegisterLibFunc(&pangoAttrWeightNew, pangoLib, "pango_attr_weight_new")
	purego.RegisterLibFunc(&pangoAttrStyleNew, pangoLib, "pango_attr_style_new")
	purego.RegisterLibFunc(&pangoAttrForegroundNew, pangoLib, "pango_attr_foreground_new")
	purego.RegisterLibFunc(&pangoAttrLetterSpacingNew, pangoLib, "pango_attr_letter_spacing_new")
	purego.RegisterLibFunc(&pangoAttrLineHeightNew, pangoLib, "pango_attr_line_height_new")
	purego.RegisterLibFunc(&pangoAttrLineHeightAbsoluteNew, pangoLib, "pango_attr_line_height_new_absolute")
	purego.RegisterLibFunc(&pangoAttrUnderlineNew, pangoLib, "pango_attr_underline_new")
	purego.RegisterLibFunc(&pangoAttrStrikethroughNew, pangoLib, "pango_attr_strikethrough_new")
	purego.RegisterLibFunc(&pangoAttrUnderlineColorNew, pangoLib, "pango_attr_underline_color_new")
	purego.RegisterLibFunc(&pangoAttrStrikethroughColorNew, pangoLib, "pango_attr_strikethrough_color_new")
	purego.RegisterLibFunc(&pangoAttrShapeNew, pangoLib, "pango_attr_shape_new")
	purego.RegisterLibFunc(&pangoAttrFontFeaturesNew, pangoLib, "pango_attr_font_features_new")
	purego.RegisterLibFunc(&pangoContextGetMetrics, pangoLib, "pango_context_get_metrics")
	purego.RegisterLibFunc(&pangoFontMetricsUnref, pangoLib, "pango_font_metrics_unref")
	purego.RegisterLibFunc(&pangoFontMetricsUnderlinePos, pangoLib, "pango_font_metrics_get_underline_position")
	purego.RegisterLibFunc(&pangoFontMetricsUnderlineThick, pangoLib, "pango_font_metrics_get_underline_thickness")
	purego.RegisterLibFunc(&pangoFontMetricsStrikePos, pangoLib, "pango_font_metrics_get_strikethrough_position")
	purego.RegisterLibFunc(&pangoFontMetricsStrikeThick, pangoLib, "pango_font_metrics_get_strikethrough_thickness")
	purego.RegisterLibFunc(&pangoLayoutGetExtents, pangoLib, "pango_layout_get_extents")
	purego.RegisterLibFunc(&pangoLayoutGetIter, pangoLib, "pango_layout_get_iter")
	purego.RegisterLibFunc(&pangoLayoutIterFree, pangoLib, "pango_layout_iter_free")
	purego.RegisterLibFunc(&pangoLayoutIterNextLine, pangoLib, "pango_layout_iter_next_line")
	purego.RegisterLibFunc(&pangoLayoutIterNextRun, pangoLib, "pango_layout_iter_next_run")
	purego.RegisterLibFunc(&pangoLayoutIterGetLine, pangoLib, "pango_layout_iter_get_line_readonly")
	purego.RegisterLibFunc(&pangoLayoutIterGetRun, pangoLib, "pango_layout_iter_get_run_readonly")
	purego.RegisterLibFunc(&pangoLayoutIterGetBaseline, pangoLib, "pango_layout_iter_get_baseline")
	purego.RegisterLibFunc(&pangoLayoutIterGetRunExtents, pangoLib, "pango_layout_iter_get_run_extents")
	purego.RegisterLibFunc(&pangoLayoutIterGetLineExtents, pangoLib, "pango_layout_iter_get_line_extents")
	purego.RegisterLibFunc(&pangoLayoutLineGetXRanges, pangoLib, "pango_layout_line_get_x_ranges")
	purego.RegisterLibFunc(&pangoCairoCreateLayout, pangocairoLib, "pango_cairo_create_layout")
	purego.RegisterLibFunc(&pangoCairoShowLayout, pangocairoLib, "pango_cairo_show_layout")
	purego.RegisterLibFunc(&pangoCairoUpdateLayout, pangocairoLib, "pango_cairo_update_layout")

	// Cairo.
	purego.RegisterLibFunc(&cairoImageSurfaceCreate, cairoLib, "cairo_image_surface_create")
	purego.RegisterLibFunc(&cairoCreate, cairoLib, "cairo_create")
	purego.RegisterLibFunc(&cairoDestroy, cairoLib, "cairo_destroy")
	purego.RegisterLibFunc(&cairoSurfaceDestroy, cairoLib, "cairo_surface_destroy")
	purego.RegisterLibFunc(&cairoSurfaceFlush, cairoLib, "cairo_surface_flush")
	purego.RegisterLibFunc(&cairoSurfaceMarkDirty, cairoLib, "cairo_surface_mark_dirty")
	purego.RegisterLibFunc(&cairoImageSurfaceGetData, cairoLib, "cairo_image_surface_get_data")
	purego.RegisterLibFunc(&cairoImageSurfaceGetStride, cairoLib, "cairo_image_surface_get_stride")
	purego.RegisterLibFunc(&cairoSetSourceRGBA, cairoLib, "cairo_set_source_rgba")
	purego.RegisterLibFunc(&cairoPaint, cairoLib, "cairo_paint")
	purego.RegisterLibFunc(&cairoPaintWithAlpha, cairoLib, "cairo_paint_with_alpha")
	purego.RegisterLibFunc(&cairoRectangle, cairoLib, "cairo_rectangle")
	purego.RegisterLibFunc(&cairoArc, cairoLib, "cairo_arc")
	purego.RegisterLibFunc(&cairoArcNegative, cairoLib, "cairo_arc_negative")
	purego.RegisterLibFunc(&cairoSetFillRule, cairoLib, "cairo_set_fill_rule")
	purego.RegisterLibFunc(&cairoIdentityMatrix, cairoLib, "cairo_identity_matrix")
	purego.RegisterLibFunc(&cairoNewSubPath, cairoLib, "cairo_new_sub_path")
	purego.RegisterLibFunc(&cairoNewPath, cairoLib, "cairo_new_path")
	purego.RegisterLibFunc(&cairoGetCurrentPoint, cairoLib, "cairo_get_current_point")
	purego.RegisterLibFunc(&cairoClosePath, cairoLib, "cairo_close_path")
	purego.RegisterLibFunc(&cairoFill, cairoLib, "cairo_fill")
	purego.RegisterLibFunc(&cairoFillPreserve, cairoLib, "cairo_fill_preserve")
	purego.RegisterLibFunc(&cairoStroke, cairoLib, "cairo_stroke")
	purego.RegisterLibFunc(&cairoSave, cairoLib, "cairo_save")
	purego.RegisterLibFunc(&cairoRestore, cairoLib, "cairo_restore")
	purego.RegisterLibFunc(&cairoTranslate, cairoLib, "cairo_translate")
	purego.RegisterLibFunc(&cairoScale, cairoLib, "cairo_scale")
	purego.RegisterLibFunc(&cairoRotate, cairoLib, "cairo_rotate")
	purego.RegisterLibFunc(&cairoClip, cairoLib, "cairo_clip")
	purego.RegisterLibFunc(&cairoMoveTo, cairoLib, "cairo_move_to")
	purego.RegisterLibFunc(&cairoLineTo, cairoLib, "cairo_line_to")
	purego.RegisterLibFunc(&cairoCurveTo, cairoLib, "cairo_curve_to")
	purego.RegisterLibFunc(&cairoSetLineWidth, cairoLib, "cairo_set_line_width")
	purego.RegisterLibFunc(&cairoSetLineCap, cairoLib, "cairo_set_line_cap")
	purego.RegisterLibFunc(&cairoSetLineJoin, cairoLib, "cairo_set_line_join")
	purego.RegisterLibFunc(&cairoPushGroup, cairoLib, "cairo_push_group")
	purego.RegisterLibFunc(&cairoPopGroupToSource, cairoLib, "cairo_pop_group_to_source")
	purego.RegisterLibFunc(&cairoSetOperator, cairoLib, "cairo_set_operator")
	purego.RegisterLibFunc(&cairoSetSourceSurface, cairoLib, "cairo_set_source_surface")
	purego.RegisterLibFunc(&cairoSurfaceWriteToPNG, cairoLib, "cairo_surface_write_to_png")
	bindCanvas(cairoLib, pangocairoLib)
	bindNative()
	bindWidget()
	bindDecor()
	bindFileDrop()
	bindPaste()
	bindSystem()
	bindEditing()
	bindAccess()
	bindTouchScroll()
}
