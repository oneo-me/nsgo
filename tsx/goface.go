package tsx

import (
	"fmt"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/dop251/goja"

	"github.com/oneo-me/nsgo/core"
)

// Go components in TSX (NSH's components.rs): a component is a type
// implementing core.Node whose exported fields are its props, each set
// from the prop of its name in lower camel case.
//
//   - func() T follows the prop as it changes; a plain T is read as the
//     component is made, and a change makes it again. A field X beside an
//     XFunc is the same prop, which XFunc follows.
//   - func(...) without results is an event, its arguments given to the
//     handler; func(...) core.Node, with or without a leading *core.Scope,
//     renders what the prop's function returns.
//   - core.Node is a slot, []core.Node named Children the children.
//   - Class and Style take the element's classes and its style with the
//     state styles (activeStyle, ...); XClass and XStyle a part's.
//   - Label is accessibilityLabel, Hint accessibilityHint; a bool NoX
//     is the prop x negated, true unless set.
//   - Named integer types read the names their enum table gives them;
//     structs read objects of their fields' props, slices arrays.

// Option adjusts how a Go component's props are named.
type Option func(*faceSpec)

// Rename names field's prop prop.
func Rename(field, prop string) Option {
	return func(s *faceSpec) { s.renames[field] = prop }
}

// Enum names the values of an integer type for the props of its type,
// which read and report them by name.
func Enum[T ~int | ~uint8 | ~uint16 | ~int32 | ~uint32](names map[string]T) Option {
	return func(s *faceSpec) {
		t := reflect.TypeFor[T]()
		m := map[string]int64{}
		for k, v := range names {
			m[k] = int64(v)
		}
		enumMu.Lock()
		enums[t] = m
		enumMu.Unlock()
	}
}

var (
	enumMu sync.RWMutex
	enums  = map[reflect.Type]map[string]int64{}
	// unions are the members of interface types, by the type tag their
	// JS objects carry ({ type: "paragraph", … }).
	unions = map[reflect.Type]map[string]reflect.Type{}
)

// Union names the types implementing interface type T for the props
// that hold one: a JS object's type tag picks its member, and a member
// given to JS carries its tag.
func Union[T any](members map[string]any) Option {
	return func(s *faceSpec) {
		m := map[string]reflect.Type{}
		for tag, zero := range members {
			m[tag] = reflect.TypeOf(zero)
		}
		enumMu.Lock()
		unions[reflect.TypeFor[T]()] = m
		enumMu.Unlock()
	}
}

// unionTag is the tag of a member of a union, "" for none.
func unionTag(iface, t reflect.Type) string {
	enumMu.RLock()
	defer enumMu.RUnlock()
	for tag, m := range unions[iface] {
		if m == t {
			return tag
		}
	}
	return ""
}

type faceSpec struct {
	renames map[string]string
	custom  map[string]func(p *props, v reflect.Value)
	// decls are the declarations of a custom field's props, and types
	// the types they name, declared with them.
	decls map[string][]string
	types map[string][]reflect.Type
	// events are the names of what events give their handlers, by field.
	events map[string][]string
}

func newSpec(opts []Option) *faceSpec {
	spec := &faceSpec{renames: map[string]string{}, custom: map[string]func(p *props, v reflect.Value){},
		decls: map[string][]string{}, types: map[string][]reflect.Type{}, events: map[string][]string{}}
	for _, o := range opts {
		o(spec)
	}
	return spec
}

// Event names the fields of the one object event field passes its
// handler, a name for each of its arguments, as NSH's #[args] does:
// Event("OnOpenChange", "open", "reason"). An event of no name given
// passes the event struct it has as it is, and a bare value as { value };
// Bounds, what every OnLayout has, as { bounds }.
func Event(field string, names ...string) Option {
	return func(s *faceSpec) { s.events[field] = names }
}

// eventFields are the fields the object of event field of type t holds,
// nil when its one argument is that object.
func eventFields(spec *faceSpec, component reflect.Type, field string, t reflect.Type) []string {
	names, named := spec.events[field]
	switch {
	case named && len(names) != t.NumIn():
		panic(fmt.Sprintf("tsx: %s.%s has %d arguments, not those of %q", component, field, t.NumIn(), names))
	case named:
		return names
	case t.NumIn() != 1:
		panic(fmt.Sprintf("tsx: %s.%s needs tsx.Event to name its arguments", component, field))
	case t.In(0) == boundsType:
		return []string{"bounds"}
	case t.In(0).Kind() == reflect.Struct || t.In(0) == errorType:
		return nil
	}
	if _, shaped := shapeOf(t.In(0)); shaped {
		return nil
	}
	return []string{"value"}
}

// isEvent reports whether field of type t is an event with something to
// report.
func isEvent(field string, t reflect.Type) bool {
	return strings.HasPrefix(field, "On") && t.Kind() == reflect.Func && t.NumOut() == 0 && t.NumIn() > 0
}

// eventSetter sets event field t from prop: what it reports goes to the
// handler as one object of the fields names, or as it is without any.
func eventSetter(t reflect.Type, prop string, names []string) func(p *props, v reflect.Value) {
	return func(p *props, v reflect.Value) {
		h := p.handler(prop)
		if h == nil {
			return
		}
		r := p.r
		v.Set(reflect.MakeFunc(t, func(in []reflect.Value) []reflect.Value {
			if names == nil {
				h(r.encode(in[0]))
				return nil
			}
			o := r.vm.NewObject()
			for i, a := range in {
				_ = o.Set(names[i], r.encode(a))
			}
			h(o)
			return nil
		}))
	}
}

// Flag sets the bool field from prop, a choice of names: true when it is
// on. It is declared as choices, a TypeScript union of the names:
// Flag("Unified", "layout", "unified", `"split" | "unified"`).
func Flag(field, prop, on, choices string) Option {
	return custom(field, func(p *props, v reflect.Value) { v.SetBool(p.str(prop) == on) }, prop+"?: "+choices)
}

// Omit leaves fields out of a Go component's TSX face: what JS has no use
// for, as a Go function of Go values.
func Omit(fields ...string) Option {
	return func(s *faceSpec) {
		for _, f := range fields {
			s.custom[f] = func(*props, reflect.Value) {}
			s.decls[f] = nil
		}
	}
}

// Preset sets field to value in every element of the component and
// leaves it out of its TSX face, as a Go function a face gives a TSX
// component: Preset("StyleOf", typography).
func Preset(field string, value any) Option {
	return func(s *faceSpec) {
		s.custom[field] = func(_ *props, v reflect.Value) { v.Set(reflect.ValueOf(value)) }
		s.decls[field] = nil
	}
}

// custom sets field with set, for props whose shape is not its field's,
// declared as decls ("data?: readonly unknown[]").
func custom(field string, set func(p *props, v reflect.Value), decls ...string) Option {
	return func(s *faceSpec) {
		s.custom[field] = set
		s.decls[field] = decls
	}
}

// registered are the modules apps registered: their exports by name.
var (
	registryMu sync.Mutex
	registered = map[string][]export{}
)

// Register makes Go components importable from module in TSX: each value
// of components is a component's zero value, a struct implementing
// core.Node, exported under its key. Register them before an app starts,
// as in an init function.
func Register(module string, components map[string]core.Node, opts ...Option) {
	spec := newSpec(opts)
	registryMu.Lock()
	defer registryMu.Unlock()
	for name, zero := range components {
		t := reflect.TypeOf(zero)
		for _, field := range reflect.VisibleFields(t) {
			if _, set := spec.custom[field.Name]; !set && field.IsExported() && isEvent(field.Name, field.Type) {
				eventFields(spec, t, field.Name, field.Type)
			}
		}
		f := goFace(t, spec)
		registeredTypes[module] = append(registeredTypes[module], registeredType{name, t, spec})
		registered[module] = append(registered[module], faceExport(name, f))
	}
}

// RegisterFunctions makes Go functions importable from module in TSX,
// and nativeModule(module) gives them too, as an embedding app's UI set
// exports its functions: each value of funcs is a func, exported under
// its key. JS's arguments convert to its parameters and its result back,
// structs as objects with lower-camel keys; a non-nil error it returns
// last throws. A func taking a context.Context first is async: it runs
// on a goroutine and JS gets a promise (classes.go). Register them before
// an app starts.
func RegisterFunctions(module string, funcs map[string]any) {
	registryMu.Lock()
	defer registryMu.Unlock()
	for name, fn := range funcs {
		t := reflect.TypeOf(fn)
		if t == nil || t.Kind() != reflect.Func {
			panic(fmt.Sprintf("tsx: RegisterFunctions: %s.%s is no func", module, name))
		}
		registeredFuncs[module] = append(registeredFuncs[module], registeredFunc{name, t})
		v, qualified := reflect.ValueOf(fn), module+"."+name
		registered[module] = append(registered[module], export{name, func(r *runtime) goja.Value {
			return r.function(func(call goja.FunctionCall) goja.Value { return r.callGo(qualified, v, call.Arguments) })
		}})
	}
}

// RegisterStatics gives component, which Register made importable from
// module, static functions, as NSH's TextView.parse: each value of funcs
// is a func, called with JS's arguments read as its parameters' types
// and its results given back as the component's props and events cross.
func RegisterStatics(module, component string, funcs map[string]any) {
	registryMu.Lock()
	defer registryMu.Unlock()
	for name, fn := range funcs {
		if reflect.TypeOf(fn) == nil || reflect.TypeOf(fn).Kind() != reflect.Func {
			panic(fmt.Sprintf("tsx: RegisterStatics: %s.%s.%s is no func", module, component, name))
		}
		statics[module] = append(statics[module], static{component, name, reflect.ValueOf(fn)})
	}
}

// static is a component's static function.
type static struct {
	component, name string
	fn              reflect.Value
}

var statics = map[string][]static{}

// addStatics sets module m's components' static functions.
func (r *runtime) addStatics(module string, m *goja.Object) {
	registryMu.Lock()
	list := statics[module]
	registryMu.Unlock()
	for _, s := range list {
		c, ok := m.Get(s.component).(*goja.Object)
		if !ok {
			continue
		}
		ft := s.fn.Type()
		_ = c.Set(s.name, r.function(func(call goja.FunctionCall) goja.Value {
			in := make([]reflect.Value, ft.NumIn())
			skip := 0
			if len(in) > 0 && ft.In(0) == scopeType {
				in[0], skip = reflect.ValueOf(r.cx), 1
			}
			for i := skip; i < len(in); i++ {
				t := ft.In(i)
				if ft.IsVariadic() && i == ft.NumIn()-1 {
					t = t.Elem()
					var rest []reflect.Value
					for j := i; j < len(call.Arguments); j++ {
						rest = append(rest, r.decode(call.Arguments[j], t))
					}
					return r.results(s.fn.Call(append(in[:i], rest...)), ft, nil)
				}
				in[i] = r.decode(call.Argument(i-skip), t)
			}
			return r.results(s.fn.Call(in), ft, nil)
		}))
	}
}

// shape is how a Go type crosses to JS when its JS shape is not its
// fields' (Shape).
type shape struct {
	name, decl string
	encode     func(any) map[string]any
	decode     func(map[string]any) any
	// none, when set, is what null and undefined read as.
	none any
}

// shapeNone has null and undefined read as v, a Shape's value that
// means none where T's zero value means something.
func shapeNone[T any](v T) {
	shapeMu.Lock()
	defer shapeMu.Unlock()
	s := shapes[reflect.TypeFor[T]()]
	s.none = v
	shapes[reflect.TypeFor[T]()] = s
}

var (
	shapeMu sync.RWMutex
	shapes  = map[reflect.Type]shape{}
)

// Shape gives the values of T a JS shape of their own, NSH's where T's
// fields differ from it: encode makes the object JS gets, its values
// crossing as any Go value does, decode, when set, reads one JS gives
// (exported, its numbers float64), and decl, a TypeScript declaration of
// name, declares it.
func Shape[T any](name, decl string, encode func(T) map[string]any, decode func(map[string]any) T) {
	shapeMu.Lock()
	defer shapeMu.Unlock()
	s := shape{name: name, decl: decl, encode: func(v any) map[string]any { return encode(v.(T)) }}
	if decode != nil {
		s.decode = func(m map[string]any) any { return decode(m) }
	}
	shapes[reflect.TypeFor[T]()] = s
}

func shapeOf(t reflect.Type) (shape, bool) {
	shapeMu.RLock()
	defer shapeMu.RUnlock()
	s, ok := shapes[t]
	return s, ok
}

// encodeShaped is a Shape's encoding as a JS object.
func (r *runtime) encodeShaped(m map[string]any) goja.Value {
	if m == nil {
		return goja.Null()
	}
	o := r.vm.NewObject()
	for k, v := range m {
		if v == nil {
			_ = o.Set(k, goja.Null())
			continue
		}
		_ = o.Set(k, r.encode(reflect.ValueOf(v)))
	}
	return o
}

// partKey holds what a part's call made: a value its parent collects.
var partKey = goja.NewSymbol("nsgo.part")

// part is a part's call: what builds its value from its props.
type part struct {
	t     reflect.Type
	build func() reflect.Value
}

// parts are the types registered as parts: values, not nodes, that a
// parent's slice of them collects from its children (ResizablePanel in
// a ResizablePanelGroup's Panels).
var parts = map[reflect.Type]bool{}

// registerPart exports a part: its call returns a marker of the value
// its props make, which a parent with a slice of its type collects.
func registerPart(module, name string, zero any, opts ...Option) {
	spec := newSpec(opts)
	t := reflect.TypeOf(zero)
	registryMu.Lock()
	defer registryMu.Unlock()
	parts[t] = true
	var once sync.Once
	var fields []field
	registered[module] = append(registered[module], export{name, func(r *runtime) goja.Value {
		return r.function(func(call goja.FunctionCall) goja.Value {
			obj, ok := arg(call, 0).(*goja.Object)
			if !ok {
				obj = r.vm.NewObject()
			}
			n := r.newFaceNode(name, nil, obj)
			p := n.props
			o := r.vm.NewObject()
			_ = o.SetSymbol(partKey, &part{t: t, build: func() reflect.Value {
				once.Do(func() { fields = planFields(t, spec) })
				v := reflect.New(t).Elem()
				for _, f := range fields {
					f.set(p, v.FieldByIndex(f.index))
				}
				return v
			}})
			return o
		})
	}})
}

// partsOf collects the values of parts of type t among children.
func (r *runtime) partsOf(children goja.Value, t reflect.Type) reflect.Value {
	out := reflect.MakeSlice(reflect.SliceOf(t), 0, 4)
	var walk func(v goja.Value, depth int)
	walk = func(v goja.Value, depth int) {
		if depth > maxDepth || !defined(v) {
			return
		}
		if isFunction(v) {
			walk(r.call(v), depth+1)
			return
		}
		if items := r.arrayItems(v); items != nil {
			for _, it := range items {
				walk(it, depth+1)
			}
			return
		}
		if o, ok := v.(*goja.Object); ok {
			if s := o.GetSymbol(partKey); s != nil {
				if pt, ok := s.Export().(*part); ok && pt.t == t {
					out = reflect.Append(out, pt.build())
				}
			}
		}
	}
	walk(children, 0)
	return out
}

// field is how one prop sets a field.
type field struct {
	index []int
	prop  string
	set   func(p *props, v reflect.Value)
}

// goFace returns the face of struct type t.
func goFace(t reflect.Type, spec *faceSpec) face {
	var once sync.Once
	var fields []field
	return func(p *props) core.Node {
		once.Do(func() { fields = planFields(t, spec) })
		v := reflect.New(t).Elem()
		for _, f := range fields {
			f.set(p, v.FieldByIndex(f.index))
		}
		return v.Interface().(core.Node)
	}
}

var (
	nodeType      = reflect.TypeFor[core.Node]()
	nodesType     = reflect.TypeFor[[]core.Node]()
	scopeType     = reflect.TypeFor[*core.Scope]()
	styleType     = reflect.TypeFor[core.Style]()
	refType       = reflect.TypeFor[*core.Ref]()
	valueType     = reflect.TypeFor[*core.AnimatedValue]()
	animNodeType  = reflect.TypeFor[core.AnimatedNode]()
	durationType  = reflect.TypeFor[time.Duration]()
	timingType    = reflect.TypeFor[core.TimingConfig]()
	colorType     = reflect.TypeFor[core.Color]()
	pointsType    = reflect.TypeFor[core.Points]()
	lengthType    = reflect.TypeFor[core.Length]()
	edgesType     = reflect.TypeFor[core.Edges]()
	pointType     = reflect.TypeFor[core.Point]()
	sizeType      = reflect.TypeFor[core.Size]()
	boundsType    = reflect.TypeFor[core.Bounds]()
	imageType     = reflect.TypeFor[core.ImageSource]()
	pressType     = reflect.TypeFor[core.PressEvent]()
	gojaValueType = reflect.TypeFor[goja.Value]()
	errorType     = reflect.TypeFor[error]()
	easingType    = reflect.TypeFor[core.Easing]()
	regexpType    = reflect.TypeFor[*regexp.Regexp]()
)

// propName is a field's prop: its rename, NSH's name or lower camel case.
func propName(name string, spec *faceSpec) string {
	if r, ok := spec.renames[name]; ok {
		return r
	}
	switch name {
	case "Label":
		return "accessibilityLabel"
	case "Hint":
		return "accessibilityHint"
	case "TestID":
		return "testID"
	case "TeX":
		return "tex"
	}
	if part, ok := strings.CutSuffix(name, "Label"); ok {
		return lowerCamel(part) + "AccessibilityLabel"
	}
	return lowerCamel(name)
}

// fieldName is the key of struct t's field f in JS: Fields' name, else
// dataName's, and whether it is a NoX field Fields names x, negated.
func fieldName(t reflect.Type, f reflect.StructField) (string, bool) {
	fieldMu.RLock()
	name, ok := fieldNames[t][f.Name]
	fieldMu.RUnlock()
	if !ok {
		return dataName(f.Name), false
	}
	return name, f.Type.Kind() == reflect.Bool && strings.HasPrefix(f.Name, "No")
}

// Fields names fields of the data struct T in JS by names, NSH's where
// its fields' names differ: a bool NoX field named x is negated, true
// unless set ({"NoResize": "resizable"}).
func Fields[T any](names map[string]string) {
	fieldMu.Lock()
	defer fieldMu.Unlock()
	t := reflect.TypeFor[T]()
	if fieldNames[t] == nil {
		fieldNames[t] = map[string]string{}
	}
	for f, n := range names {
		fieldNames[t][f] = n
	}
}

var fieldNames = map[reflect.Type]map[string]string{}

// dataName is a data struct's field as JS names it: in lower camel
// case, without a component prop's accessibility names (a completion
// item's Label is its label).
func dataName(name string) string {
	switch name {
	case "TestID":
		return "testID"
	case "TeX":
		return "tex"
	}
	return lowerCamel(name)
}

// lowerCamel lowers a name's first word: TestID is testID, URL url,
// OnPress onPress.
func lowerCamel(s string) string {
	r := []rune(s)
	i := 0
	for i < len(r) && unicode.IsUpper(r[i]) {
		i++
	}
	switch {
	case i == 0:
		return s
	case i == 1 || i == len(r):
		i = min(i, len(r))
	default:
		i-- // the last capital starts the next word
	}
	for j := range i {
		r[j] = unicode.ToLower(r[j])
	}
	return string(r)
}

// planFields says how each exported field of t is set.
func planFields(t reflect.Type, spec *faceSpec) []field {
	byName := map[string]reflect.StructField{}
	for _, f := range reflect.VisibleFields(t) {
		if f.IsExported() && !f.Anonymous {
			byName[f.Name] = f
		}
	}
	var out []field
	for _, f := range reflect.VisibleFields(t) {
		if !f.IsExported() || f.Anonymous {
			continue
		}
		name := f.Name
		// X beside XFunc is XFunc's prop.
		if _, ok := byName[name+"Func"]; ok {
			continue
		}
		base, isFunc := strings.CutSuffix(name, "Func")
		if !isFunc || f.Type.Kind() != reflect.Func || f.Type.NumIn() != 0 {
			base = name
		}
		prop := propName(base, spec)
		ft := f.Type
		plan := field{index: f.Index, prop: prop}
		if set, ok := spec.custom[name]; ok {
			plan.set = set
			out = append(out, plan)
			continue
		}
		switch {
		case base == "Style" && (ft == styleType || isGetter(ft, styleType)):
			plan.prop = "style"
			plan.set = func(p *props, v reflect.Value) { setStyle(p, v, "style", true) }
		case strings.HasSuffix(base, "Style") && (ft == styleType || isGetter(ft, styleType)):
			plan.set = func(p *props, v reflect.Value) { setStyle(p, v, prop, false) }
		case base == "Class" && ft.Kind() == reflect.String:
			plan.set = func(p *props, v reflect.Value) { v.SetString(p.class()) }
		case base == "Children" && ft == nodesType:
			plan.set = func(p *props, v reflect.Value) { v.Set(reflect.ValueOf(p.children())) }
		case ft == refType:
			plan.set = func(p *props, v reflect.Value) { v.Set(reflect.ValueOf(p.n.ref)) }
		case name == "Ref" && ft.Kind() == reflect.Pointer && ft.Elem().Kind() == reflect.Struct:
			// A ref of the component's own: the ref prop gets its methods.
			plan.set = func(p *props, v reflect.Value) {
				ref := reflect.New(ft.Elem())
				v.Set(ref)
				if target := p.peek("ref"); defined(target) {
					o := p.r.refObject(ref)
					_ = o.Set("node", p.n.value())
					p.r.applyRefValue(target, o, "")
				}
			}
		case ft.Kind() == reflect.Slice && parts[ft.Elem()]:
			elem := ft.Elem()
			plan.set = func(p *props, v reflect.Value) {
				if x := p.get(prop); defined(x) {
					v.Set(p.r.decode(x, ft))
					return
				}
				v.Set(p.r.partsOf(p.get("children"), elem))
			}
		case ft.Kind() == reflect.Bool && strings.HasPrefix(base, "No") && len(base) > 2 && unicode.IsUpper(rune(base[2])):
			plan.prop = lowerCamel(base[2:])
			if r, ok := spec.renames[name]; ok {
				plan.prop = r
			}
			pr := plan.prop
			plan.set = func(p *props, v reflect.Value) { v.SetBool(!p.boolOr(pr, true)) }
		case isEvent(name, ft):
			plan.set = eventSetter(ft, prop, eventFields(spec, t, name, ft))
		default:
			plan.set = setter(ft, prop)
		}
		out = append(out, plan)
	}
	return out
}

// isGetter reports whether t is func() r.
func isGetter(t, r reflect.Type) bool {
	return t.Kind() == reflect.Func && t.NumIn() == 0 && t.NumOut() == 1 && t.Out(0) == r
}

// setStyle sets a style field, or its Func, from prop: the element's
// style with its state styles when states.
func setStyle(p *props, v reflect.Value, prop string, states bool) {
	if !p.has(prop) && !(states && hasStates(p)) {
		return
	}
	read := func() core.Style { return p.r.style(p.get(prop)) }
	if states {
		read = p.style
	}
	if v.Type() == styleType {
		v.Set(reflect.ValueOf(read()))
		return
	}
	v.Set(reflect.ValueOf(read))
}

func hasStates(p *props) bool {
	for name := range stateProps {
		if p.has(name) {
			return true
		}
	}
	return false
}

// setter returns what sets a field of type t from prop.
func setter(t reflect.Type, prop string) func(p *props, v reflect.Value) {
	switch {
	case t == nodeType:
		return func(p *props, v reflect.Value) {
			if s := p.slot(prop); s != nil {
				v.Set(reflect.ValueOf(s))
			}
		}
	case t.Kind() == reflect.Func && t.NumOut() == 1 && t.Out(0) == nodeType:
		return renderSetter(t, prop)
	case t.Kind() == reflect.Func && t.NumIn() == 0 && t.NumOut() == 1:
		out := t.Out(0)
		return func(p *props, v reflect.Value) {
			if !p.has(prop) {
				return
			}
			r := p.r
			v.Set(reflect.MakeFunc(t, func([]reflect.Value) []reflect.Value {
				return []reflect.Value{r.decode(p.get(prop), out)}
			}))
		}
	case t.Kind() == reflect.Func:
		return func(p *props, v reflect.Value) {
			h := p.handler(prop)
			if h == nil {
				return
			}
			r := p.r
			v.Set(reflect.MakeFunc(t, func(in []reflect.Value) []reflect.Value {
				args := make([]goja.Value, len(in))
				for i, a := range in {
					args[i] = r.encode(a)
				}
				res := h(args...)
				out := make([]reflect.Value, t.NumOut())
				for i := range out {
					out[i] = r.decode(res, t.Out(i))
				}
				return out
			}))
		}
	}
	return func(p *props, v reflect.Value) {
		if x := p.get(prop); defined(x) {
			v.Set(p.r.decode(x, t))
		}
	}
}

// renderSetter sets a render function: the prop's function called with
// the arguments, what it returns shown in a region.
func renderSetter(t reflect.Type, prop string) func(p *props, v reflect.Value) {
	return func(p *props, v reflect.Value) {
		if !isFunction(p.get(prop)) {
			return
		}
		r := p.r
		v.Set(reflect.MakeFunc(t, func(in []reflect.Value) []reflect.Value {
			fn := p.peek(prop)
			// Arguments that are getters are read as the region renders,
			// so it renders again as they change.
			return []reflect.Value{reflect.ValueOf(core.Node(lazy{r, p.n, func() goja.Value {
				var args []goja.Value
				for _, a := range in {
					switch {
					case a.Type() == scopeType:
					case a.Kind() == reflect.Func && a.Type().NumIn() == 0 && a.Type().NumOut() == 1:
						args = append(args, r.encode(a.Call(nil)[0]))
					default:
						args = append(args, r.encode(a))
					}
				}
				return r.call(fn, args...)
			}}))}
		}))
	}
}

// decode reads v as a value of t.
func (r *runtime) decode(v goja.Value, t reflect.Type) reflect.Value {
	out := reflect.New(t).Elem()
	if !defined(v) {
		if s, ok := shapeOf(t); ok && s.none != nil {
			out.Set(reflect.ValueOf(s.none))
		}
		return out
	}
	if s, ok := shapeOf(t); ok && s.decode != nil {
		if m, isMap := v.Export().(map[string]any); isMap {
			out.Set(reflect.ValueOf(s.decode(m)))
		}
		return out
	}

	if def := classOf(t); def != nil {
		out.Set(r.instanceOf(def, v, "argument"))
		return out
	}
	switch t {
	case gojaValueType:
		out.Set(reflect.ValueOf(v))
		return out
	case styleType:
		out.Set(reflect.ValueOf(r.style(v)))
		return out
	case colorType:
		c, _ := core.ParseColor(v.String())
		out.Set(reflect.ValueOf(c))
		return out
	case pointsType:
		out.Set(reflect.ValueOf(points(v)))
		return out
	case lengthType:
		out.Set(reflect.ValueOf(r.length("", v)))
		return out
	case durationType:
		out.Set(reflect.ValueOf(millis(v.ToFloat())))
		return out
	case edgesType:
		out.Set(reflect.ValueOf(r.edges(v)))
		return out
	case pointType:
		out.Set(reflect.ValueOf(r.pointOf(v.ToObject(r.vm))))
		return out
	case sizeType:
		out.Set(reflect.ValueOf(r.sizeOf(v.ToObject(r.vm))))
		return out
	case boundsType:
		out.Set(reflect.ValueOf(r.anchorBounds(v)))
		return out
	case imageType:
		out.Set(reflect.ValueOf(r.imageSource(v)))
		return out
	case valueType:
		if a := r.animatedValueOf(v); a != nil {
			out.Set(reflect.ValueOf(a))
		}
		return out
	case animNodeType:
		if a := r.animatedNodeOf(v); a != nil {
			out.Set(reflect.ValueOf(a))
		}
		return out
	case nodeType:
		out.Set(reflect.ValueOf(core.Node(r.region(func() goja.Value { return v }))))
		return out
	case easingType:
		e, _ := r.easing(v)
		out.Set(reflect.ValueOf(e))
		return out
	case regexpType:
		// A pattern's source, or a RegExp's.
		src := v.String()
		if o, ok := v.(*goja.Object); ok && o.ClassName() == "RegExp" {
			src = o.Get("source").String()
		}
		if re, err := regexp.Compile(src); err == nil {
			out.Set(reflect.ValueOf(re))
		} else {
			r.warn("pattern %q: %v", src, err)
		}
		return out
	}
	enumMu.RLock()
	names, isEnum := enums[t]
	enumMu.RUnlock()
	if isEnum {
		if n, ok := names[v.String()]; ok {
			setInt(out, n)
		} else if _, isStr := v.Export().(string); !isStr {
			setInt(out, v.ToInteger())
		}
		return out
	}
	switch t.Kind() {
	case reflect.Bool:
		out.SetBool(v.ToBoolean())
	case reflect.String:
		out.SetString(v.String())
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		setInt(out, v.ToInteger())
	case reflect.Float32, reflect.Float64:
		out.SetFloat(v.ToFloat())
	case reflect.Slice:
		items := r.arrayItems(v)
		s := reflect.MakeSlice(t, len(items), len(items))
		for i, it := range items {
			s.Index(i).Set(r.decode(it, t.Elem()))
		}
		out.Set(s)
	case reflect.Map:
		o, ok := v.(*goja.Object)
		if !ok {
			break
		}
		m := reflect.MakeMap(t)
		for _, k := range o.Keys() {
			m.SetMapIndex(r.decode(r.vm.ToValue(k), t.Key()), r.decode(o.Get(k), t.Elem()))
		}
		out.Set(m)
	case reflect.Pointer:
		out.Set(reflect.New(t.Elem()))
		out.Elem().Set(r.decode(v, t.Elem()))
	case reflect.Struct:
		o, ok := v.(*goja.Object)
		if !ok {
			break
		}
		for _, f := range reflect.VisibleFields(t) {
			if !f.IsExported() || f.Anonymous {
				continue
			}
			name, negated := fieldName(t, f)
			x := o.Get(name)
			if defined(x) {
				if negated {
					out.FieldByIndex(f.Index).SetBool(!x.ToBoolean())
					continue
				}
				out.FieldByIndex(f.Index).Set(r.decode(x, f.Type))
			}
		}
	case reflect.Interface:
		enumMu.RLock()
		members := unions[t]
		enumMu.RUnlock()
		if o, ok := v.(*goja.Object); ok && members != nil {
			if m, ok := members[stringOf(o.Get("type"))]; ok {
				out.Set(r.decode(v, m))
			}
			break
		}
		if e := v.Export(); e != nil && reflect.TypeOf(e).AssignableTo(t) {
			out.Set(reflect.ValueOf(e))
		}
	case reflect.Func:
		if fn := v; isFunction(fn) && r.posting > 0 {
			out.Set(r.callback(fn, t))
		} else if isFunction(fn) {
			out.Set(reflect.MakeFunc(t, func(in []reflect.Value) []reflect.Value {
				args := make([]goja.Value, len(in))
				for i, a := range in {
					args[i] = r.encode(a)
				}
				res := r.call(fn, args...)
				outs := make([]reflect.Value, t.NumOut())
				for i := range outs {
					outs[i] = r.decode(res, t.Out(i))
				}
				return outs
			}))
		}
	}
	return out
}

func setInt(v reflect.Value, n int64) {
	if v.CanUint() {
		v.SetUint(uint64(n))
	} else {
		v.SetInt(n)
	}
}

// encode gives a Go value to JS: events as NSH's objects, enums by name,
// structs as objects of their fields' props.
func (r *runtime) encode(v reflect.Value) goja.Value {
	if !v.IsValid() {
		return goja.Undefined()
	}
	if s, ok := shapeOf(v.Type()); ok {
		return r.encodeShaped(s.encode(v.Interface()))
	}
	if def := classOf(v.Type()); def != nil {
		return r.instanceObject(def, v)
	}
	switch v.Type() {
	case pressType:
		return r.pressEvent(v.Interface().(core.PressEvent))
	case pointType:
		return r.point(v.Interface().(core.Point))
	case sizeType:
		return r.size(v.Interface().(core.Size))
	case boundsType:
		return r.bounds(v.Interface().(core.Bounds))
	case colorType:
		return r.vm.ToValue(hexColor(v.Interface().(core.Color)))
	case durationType:
		return r.vm.ToValue(float64(v.Interface().(time.Duration).Microseconds()) / 1000)
	case errorType:
		if v.IsNil() {
			return goja.Null()
		}
		return r.goError(v.Interface().(error))
	case scopeType:
		return goja.Undefined()
	}
	enumMu.RLock()
	names, isEnum := enums[v.Type()]
	enumMu.RUnlock()
	if isEnum {
		n := v.Convert(reflect.TypeFor[int64]()).Int()
		for k, x := range names {
			if x == n {
				return r.vm.ToValue(k)
			}
		}
	}
	switch v.Kind() {
	case reflect.Struct:
		o := r.vm.NewObject()
		for _, f := range reflect.VisibleFields(v.Type()) {
			if f.IsExported() && !f.Anonymous {
				name, negated := fieldName(v.Type(), f)
				if negated {
					_ = o.Set(name, !v.FieldByIndex(f.Index).Bool())
					continue
				}
				_ = o.Set(name, r.encode(v.FieldByIndex(f.Index)))
			}
		}
		return o
	case reflect.Slice:
		if v.IsNil() {
			return r.vm.NewArray()
		}
		out := make([]any, v.Len())
		for i := range out {
			out[i] = r.encode(v.Index(i))
		}
		return r.vm.NewArray(out...)
	case reflect.Pointer:
		if v.IsNil() {
			return goja.Null()
		}
		if v.Type().Elem().Kind() == reflect.Struct && v.Type().NumMethod() > 0 {
			return r.vm.ToValue(v.Interface()) // a ref: its methods
		}
		return r.encode(v.Elem())
	case reflect.Interface:
		if v.IsNil() {
			return goja.Null()
		}
		out := r.encode(v.Elem())
		if tag := unionTag(v.Type(), v.Elem().Type()); tag != "" {
			if o, ok := out.(*goja.Object); ok {
				_ = o.Set("type", tag)
			}
		}
		return out
	case reflect.Float32:
		return r.vm.ToValue(v.Float())
	}
	return r.vm.ToValue(v.Interface())
}

// registeredModules gives the VM's module table the registered modules.
func registeredExports(name string) ([]export, bool) {
	registryMu.Lock()
	defer registryMu.Unlock()
	e, ok := registered[name]
	return e, ok
}

// results is what a ref method returned as JS sees it: null for a false
// trailing ok, an object of several.
func (r *runtime) results(out []reflect.Value, ft reflect.Type, names []string) goja.Value {
	if n := len(out); n > 1 && ft.Out(n-1).Kind() == reflect.Bool {
		if !out[n-1].Bool() {
			return goja.Null()
		}
		out = out[:n-1]
	}
	if len(out) == 1 {
		return r.encode(out[0])
	}
	res := r.vm.NewObject()
	for j, v := range out {
		k := strconv.Itoa(j)
		if j < len(names) {
			k = names[j]
		}
		_ = res.Set(k, r.encode(v))
	}
	return res
}

// resultNames name the results of ref methods that return several, which
// JS gets as an object.
var resultNames = map[string][]string{"VisibleRows": {"start", "end"}, "VisibleColumns": {"start", "end"}}

// refObject is a component's ref as JS sees it: its methods, their
// arguments read as their types, and what they return in a promise, a
// trailing ok giving null when false.
func (r *runtime) refObject(ref reflect.Value) *goja.Object {
	o := r.vm.NewObject()
	t := ref.Type()
	for i := range t.NumMethod() {
		m := t.Method(i)
		fn := ref.Method(i)
		names := resultNames[m.Name]
		_ = o.Set(lowerCamel(m.Name), r.function(func(call goja.FunctionCall) goja.Value {
			ft := fn.Type()
			in := make([]reflect.Value, ft.NumIn())
			for j := range in {
				in[j] = r.decode(arg(call, j), ft.In(j))
			}
			out := fn.Call(in)
			if len(out) == 0 {
				return goja.Undefined()
			}
			// What a method returns comes in a promise, as NSH's queries,
			// which cross to the engine thread, do.
			p, resolve, _ := r.vm.NewPromise()
			_ = resolve(r.results(out, ft, names))
			return r.vm.ToValue(p)
		}))
	}
	return o
}
