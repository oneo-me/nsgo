//go:build linux && !android

package linux

import (
	"log"
	"sync"

	"github.com/nsgo-dev/nsgo/internal/driver"
)

// component is a native view's component, as NSH's native.rs has it:
// the app's factory makes it on the view's first ItemNative, and the
// GtkWidget its View returns is hosted in the view's widget as a page or
// a video is, filling the view and taking its own pointer events. An
// unknown name leaves the view an empty box.
type component struct {
	hosted
	comp driver.NativeComponent // nil for an unknown name
}

// unknownComponents are the names warned about, once each.
var unknownComponents sync.Map

// configureNative makes v's component on its first item, then sets the
// props that changed.
func (w *window) configureNative(v *view, c *driver.NativeConfig) {
	if c == nil {
		return
	}
	if c.Bind {
		w.animator.BindNative(v.id, c.Bindings)
	}
	if v.native == nil {
		v.native = &component{}
		w.makeComponent(v, c.Component)
	}
	if comp := v.native.comp; comp != nil {
		for name, value := range c.Props {
			comp.SetProp(name, value)
		}
	}
}

// makeComponent calls name's factory for v and hosts the widget it
// returns.
func (w *window) makeComponent(v *view, name string) {
	factory := w.components[name]
	if factory == nil {
		if _, warned := unknownComponents.LoadOrStore(name, true); !warned {
			log.Printf("nsgo: no native component %q is registered; its views stay empty", name)
		}
		return
	}
	nc := v.native
	// The component may talk from any goroutine; what it says reaches the
	// engine from the main thread while its view lives.
	alive := func() bool { return !v.deleted && v.native == nc }
	emit := func(event string, data any) {
		w.post(func() {
			if alive() {
				w.engine.Dispatch(driver.Event{Kind: driver.EventNative, ID: v.id, Text: event, Data: data})
			}
		})
	}
	size := func(s driver.Size) {
		w.post(func() {
			if alive() {
				w.engine.Dispatch(driver.Event{Kind: driver.EventNativeSize, ID: v.id, Size: s})
			}
		})
	}
	nc.comp = factory(driver.NewNativeContext(emit, size))
	if nc.comp == nil {
		return
	}
	child, ok := nc.comp.View().(uintptr)
	if !ok {
		log.Printf("nsgo: native component %q: View returned %T, want a GtkWidget pointer as a uintptr", name, nc.comp.View())
		return
	}
	if child != 0 {
		w.host(v, &nc.hosted, child, true)
		w.natives[child] = v
	}
}

// closeNative lets v's component go: its widget leaves the view, then
// the component closes.
func (w *window) closeNative(v *view) {
	nc := v.native
	if nc.child != 0 {
		delete(w.natives, nc.child)
	}
	w.unhost(v, &nc.hosted)
	v.native = nil
	if nc.comp != nil {
		nc.comp.Close()
	}
}
