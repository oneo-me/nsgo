//go:build linux && !android

package main

import (
	"sync"

	"github.com/ebitengine/purego"

	"github.com/nsgo-dev/nsgo/core"
)

// gtkSlider is GTK's GtkScale, as a native component: it reports its
// value as "change" events.
type gtkSlider struct {
	scale   uintptr
	cx      *core.NativeContext
	handler uint64
	setting bool // the app sets the value: no event echoes it
}

// The GTK the Linux backend loaded, reached with purego. Every call runs
// on the main thread.
var (
	gtkOnce sync.Once
	gtkErr  error

	gtkScaleNewWithRange     func(int32, float64, float64, float64) uintptr
	gtkScaleSetDrawValue     func(uintptr, int32)
	gtkRangeSetValue         func(uintptr, float64)
	gtkRangeGetValue         func(uintptr) float64
	gObjectRefSink           func(uintptr) uintptr
	gObjectUnref             func(uintptr)
	gSignalConnectData       func(uintptr, string, uintptr, uintptr, uintptr, uint32) uint64
	gSignalHandlerDisconnect func(uintptr, uint64)

	sliderChanged uintptr // the "value-changed" callback
	gtkSliders    = map[uintptr]*gtkSlider{}
)

func openGTK() error {
	gtkOnce.Do(func() {
		gtk, err := purego.Dlopen("libgtk-4.so.1", purego.RTLD_NOW|purego.RTLD_GLOBAL)
		if err != nil {
			gtkErr = err
			return
		}
		gobject, err := purego.Dlopen("libgobject-2.0.so.0", purego.RTLD_NOW|purego.RTLD_GLOBAL)
		if err != nil {
			gtkErr = err
			return
		}
		purego.RegisterLibFunc(&gtkScaleNewWithRange, gtk, "gtk_scale_new_with_range")
		purego.RegisterLibFunc(&gtkScaleSetDrawValue, gtk, "gtk_scale_set_draw_value")
		purego.RegisterLibFunc(&gtkRangeSetValue, gtk, "gtk_range_set_value")
		purego.RegisterLibFunc(&gtkRangeGetValue, gtk, "gtk_range_get_value")
		purego.RegisterLibFunc(&gObjectRefSink, gobject, "g_object_ref_sink")
		purego.RegisterLibFunc(&gObjectUnref, gobject, "g_object_unref")
		purego.RegisterLibFunc(&gSignalConnectData, gobject, "g_signal_connect_data")
		purego.RegisterLibFunc(&gSignalHandlerDisconnect, gobject, "g_signal_handler_disconnect")
		sliderChanged = purego.NewCallback(func(scale, data uintptr) {
			if s := gtkSliders[scale]; s != nil && !s.setting {
				s.cx.Emit("change", gtkRangeGetValue(scale))
			}
		})
	})
	return gtkErr
}

func init() {
	components[sliderComponent] = func(cx *core.NativeContext) core.NativeComponent {
		if openGTK() != nil {
			return nil // the view stays empty
		}
		const horizontal = 0 // GTK_ORIENTATION_HORIZONTAL
		s := &gtkSlider{scale: gObjectRefSink(gtkScaleNewWithRange(horizontal, 0, 1, 0.01)), cx: cx}
		gtkScaleSetDrawValue(s.scale, 0)
		s.handler = gSignalConnectData(s.scale, "value-changed", sliderChanged, 0, 0, 0)
		gtkSliders[s.scale] = s
		cx.SetSize(core.Size{Width: 220, Height: 28})
		return s
	}
}

func (s *gtkSlider) View() any { return s.scale }

func (s *gtkSlider) SetProp(name string, value any) {
	if v, ok := value.(float64); ok && name == "value" {
		s.set(v)
	}
}

func (s *gtkSlider) Command(name string, args any) {
	if name == "reset" {
		s.set(0)
		s.cx.Emit("change", 0.0)
	}
}

func (s *gtkSlider) set(v float64) {
	s.setting = true
	gtkRangeSetValue(s.scale, v)
	s.setting = false
}

func (s *gtkSlider) Close() {
	delete(gtkSliders, s.scale)
	gSignalHandlerDisconnect(s.scale, s.handler)
	gObjectUnref(s.scale)
}
