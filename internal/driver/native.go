package driver

// NativeConfig is what ItemNative sends: the component's name and the
// props that changed since the last item, nil for one removed. The first
// item of a view carries all of them.
type NativeConfig struct {
	Component string
	Props     map[string]any
	// Bindings, when Bind is set, replace the props animated nodes drive,
	// which the platform's Animator hands the component on the frames
	// they change.
	Bind     bool
	Bindings []NativeBinding
}

// NativeBinding binds a component's prop Name to animated node Node;
// Color says the node outputs a color.
type NativeBinding struct {
	Name  string
	Node  uint32
	Color bool
}

// AnimatedPropSetter is implemented by a component that takes its
// animated props as numbers and colors, on the main thread on the frames
// they change; without it SetProp receives them, a number as a float64
// and a color as a Color.
type AnimatedPropSetter interface {
	SetAnimatedNumber(name string, v float64)
	SetAnimatedColor(name string, c Color)
}

// SetAnimatedProp hands component c an animated prop's value, as
// ChangedNatives reports it.
func SetAnimatedProp(c NativeComponent, b NativeBinding, num float64, col Color) {
	if c == nil {
		return
	}
	if s, ok := c.(AnimatedPropSetter); ok {
		if b.Color {
			s.SetAnimatedColor(b.Name, col)
		} else {
			s.SetAnimatedNumber(b.Name, num)
		}
		return
	}
	if b.Color {
		c.SetProp(b.Name, col)
	} else {
		c.SetProp(b.Name, num)
	}
}

// NativeComponents are the components an app registers with its
// platform, by name.
type NativeComponents map[string]NativeFactory

// NativeFactory makes a component's view for one Native element, on the
// platform's main thread; nil leaves the box empty.
type NativeFactory func(cx *NativeContext) NativeComponent

// NativeComponent is one Native element's platform view. Its methods run
// on the main thread.
type NativeComponent interface {
	// View is the view that fills the box: an NSView or UIView pointer as
	// a uintptr on Apple platforms, a child HWND on Windows, a GtkWidget
	// pointer on Linux, a DOM element (syscall/js.Value) on the web, and
	// on Android the name of a View class the runtime builds by
	// reflection.
	View() any
	// SetProp sets a prop; nil removes it.
	SetProp(name string, value any)
	// Command runs a command Ref.Command sent.
	Command(name string, args any)
	// Close releases the component when its element goes.
	Close()
}

// NativeContext is how a component talks back to its element. Its
// methods may be called from any goroutine.
type NativeContext struct {
	emit func(name string, data any)
	size func(Size)
}

// NewNativeContext returns the context a platform hands a factory: emit
// reports an event, size the size the component asks for.
func NewNativeContext(emit func(name string, data any), size func(Size)) *NativeContext {
	return &NativeContext{emit: emit, size: size}
}

// Emit sends an event to the element's OnEvent.
func (c *NativeContext) Emit(name string, data any) { c.emit(name, data) }

// SetSize asks for a size: the element measures to it where its style
// leaves a dimension open, as an image does to its natural size. A
// dimension of 0 asks for the other alone.
func (c *NativeContext) SetSize(s Size) { c.size(s) }

// AndroidViewProp is one setter call on a component's view: set<Name>(Value),
// Name's first letter upper-cased.
type AndroidViewProp struct {
	Name  string
	Value any
}

// AndroidPropMapper is implemented by an Android component that chooses the
// setter calls a prop makes, in order; without it each prop calls its own
// setter with its value.
type AndroidPropMapper interface {
	ViewProps(name string, value any) []AndroidViewProp
}

// AndroidEventMapper is implemented by an Android component that receives the
// listener callbacks of its view itself, to emit what it likes; without
// it each callback is emitted as is.
type AndroidEventMapper interface {
	ViewEvent(name string, args []any)
}

// TabBarComponent is the native component iOS and Android register for
// core.TabBar: the system's tab bar. Its props are "items", a list of
// {"title", "icon"} maps, "selectedIndex" and "tint", the selected tab's
// Color as a number (0: the system's); it emits "change" with the index
// the user chose.
const TabBarComponent = "nsgo/tab-bar"
