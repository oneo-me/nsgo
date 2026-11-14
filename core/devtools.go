package core

import (
	"cmp"
	"fmt"
	"reflect"
	goruntime "runtime"
	"slices"
	"strconv"
	"strings"
	"time"
	"weak"

	"github.com/nsgo-dev/nsgo/internal/devtools"
	"github.com/nsgo-dev/nsgo/internal/driver"
)

// devState is what the graph records for DevTools while devtools.Enabled: the components its scopes render, the signals each owner made and its computations, with where each was made.
type devState struct {
	components map[*Scope]devComponent
	values     map[*Scope][]*devValue
	byID       map[uint64]*devValue
	comps      map[*computation]*devComp
	next       uint64
	// sweepAt is how many signals byID holds when devSignal next drops those no code holds any more (sweep).
	sweepAt int
	// commit is what ran since the last commit took it, by kind and site.
	commit map[ranKey]*driver.Ran
}

type ranKey struct{ kind, site string }

type devComponent struct{ name, site string }

type devValue struct {
	id      uint64
	site    string
	typ     string
	get     func() string
	readers func() int
	// gone reports whether nothing holds the signal any more: DevTools hold it weakly, so a signal its code dropped, as a list's item deleted, goes.
	gone func() bool
	// set writes a value given as text; nil for a type DevTools do not edit.
	set func(string) error
}

type devComp struct {
	site        string
	runs, since int
	took        time.Duration // since the page last asked (Hot)
	// value is a memo's value as text; nil for other computations.
	value func() string
}

func newDevState() *devState {
	return &devState{components: map[*Scope]devComponent{}, values: map[*Scope][]*devValue{}, byID: map[uint64]*devValue{}, comps: map[*computation]*devComp{},
		sweepAt: minSweep, commit: map[ranKey]*driver.Ran{}}
}

// minSweep is the fewest signals devSignal lets byID hold before it sweeps.
const minSweep = 1024

// devOwner is the owner DevTools file a signal made with cx under: the owner running now where that is cx or under it, as a computation of cx's, else cx itself, as a list's row made while the list's binding runs, so the row's signals go with the row.
func (cx *Scope) devOwner() *Scope {
	o := cx.current()
	for s := o; s != nil; s = s.parent {
		if s == cx {
			return o
		}
	}
	return cx
}

// devSignal records s, made under owner, from the caller of the signal's constructor. It holds s weakly, so recording it keeps it no longer than its code does.
func devSignal[T any](owner *Scope, s *Signal[T]) {
	d := s.rt.dev
	d.next++
	w := weak.Make(s)
	v := &devValue{id: d.next, site: callerSite(3), typ: reflect.TypeFor[T]().String(),
		get: func() string {
			if s := w.Value(); s != nil {
				return clip(fmt.Sprint(any(s.value)))
			}
			return ""
		},
		readers: func() int {
			if s := w.Value(); s != nil {
				return len(s.subs.list)
			}
			return 0
		},
		gone: func() bool { return w.Value() == nil }}
	if parse := parserFor[T](); parse != nil {
		v.set = func(text string) error {
			x, err := parse(text)
			if err != nil {
				return err
			}
			if s := w.Value(); s != nil {
				s.Set(x)
			}
			return nil
		}
	}
	d.values[owner] = append(d.values[owner], v)
	d.byID[v.id] = v
	if len(d.byID) >= d.sweepAt {
		d.sweep()
	}
}

// sweep drops the signals no code holds any more. It runs once byID has doubled since the last sweep, and before Owners lists them.
func (d *devState) sweep() {
	for o, vs := range d.values {
		kept := vs[:0]
		for _, v := range vs {
			if v.gone() {
				delete(d.byID, v.id)
			} else {
				kept = append(kept, v)
			}
		}
		clear(vs[len(kept):])
		if len(kept) == 0 {
			delete(d.values, o)
		} else {
			d.values[o] = kept
		}
	}
	d.sweepAt = max(2*len(d.byID), minSweep)
}

// parserFor returns how to read a T from text, for strings, booleans and numbers; nil for other types.
func parserFor[T any]() func(string) (T, error) {
	var zero T
	t := reflect.TypeFor[T]()
	conv := func(f func(string) (reflect.Value, error)) func(string) (T, error) {
		return func(s string) (T, error) {
			v, err := f(s)
			if err != nil {
				return zero, err
			}
			return v.Convert(t).Interface().(T), nil
		}
	}
	switch t.Kind() {
	case reflect.String:
		return conv(func(s string) (reflect.Value, error) { return reflect.ValueOf(s), nil })
	case reflect.Bool:
		return conv(func(s string) (reflect.Value, error) { b, err := strconv.ParseBool(s); return reflect.ValueOf(b), err })
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return conv(func(s string) (reflect.Value, error) {
			n, err := strconv.ParseInt(s, 10, t.Bits())
			return reflect.ValueOf(n), err
		})
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return conv(func(s string) (reflect.Value, error) {
			n, err := strconv.ParseUint(s, 10, t.Bits())
			return reflect.ValueOf(n), err
		})
	case reflect.Float32, reflect.Float64:
		return conv(func(s string) (reflect.Value, error) {
			f, err := strconv.ParseFloat(s, t.Bits())
			return reflect.ValueOf(f), err
		})
	}
	return nil
}

// memo has DevTools show memo c's value, which value gives as text.
func (d *devState) memo(c *computation, value func() string) {
	if dc := d.comps[c]; dc != nil {
		dc.value = value
	}
}

// computation records c, made by the caller of its constructor. A binding is made by the tree once the component that built its element returned, so it takes that component's site when read (siteOf).
func (d *devState) computation(c *computation) {
	dc := &devComp{site: callerSite(3)}
	// A binding the tree made has only NSGo's code above it: it takes its component's site. A TSX app's bindings are made as its JS runs, which names their line.
	if c.kind == renderEffect && !isPath(dc.site) {
		dc.site = ""
	}
	d.comps[c] = dc
}

// isPath reports whether site names a file, rather than NSGo's function for what it made itself.
func isPath(site string) bool {
	return strings.Contains(site, ".") && strings.Contains(site, ":") && !strings.Contains(site, "(")
}

// siteOf is where c was made: its own site, or for a binding the site of the component around it.
func (d *devState) siteOf(c *computation, dc *devComp) string {
	if dc.site != "" {
		return dc.site
	}
	for o := c.parent; o != nil; o = o.parent {
		if comp, ok := d.components[o]; ok && comp.site != "" {
			return comp.site
		}
	}
	return ""
}

// ran counts a run of c that took took.
func (d *devState) ran(c *computation, took time.Duration) {
	dc := d.comps[c]
	if dc == nil {
		return
	}
	dc.runs++
	dc.since++
	dc.took += took
	k := ranKey{compKindName(c.kind), d.siteOf(c, dc)}
	r := d.commit[k]
	if r == nil {
		r = &driver.Ran{Kind: k.kind, Site: k.site}
		d.commit[k] = r
	}
	r.Runs++
	r.Took += took
}

// takeRan returns what ran since it was last asked, the longest first, at most n, and forgets it.
func (d *devState) takeRan(n int) []driver.Ran {
	out := make([]driver.Ran, 0, len(d.commit))
	for _, r := range d.commit {
		out = append(out, *r)
	}
	clear(d.commit)
	slices.SortFunc(out, func(a, b driver.Ran) int {
		if a.Took != b.Took {
			return cmp.Compare(b.Took, a.Took)
		}
		return strings.Compare(a.Site, b.Site)
	})
	return out[:min(len(out), n)]
}

// component records that s renders el.
func (d *devState) component(s *Scope, el Node) {
	c := devComponent{name: reflect.TypeOf(el).String()}
	if f, ok := el.(Func); ok {
		c.name = "func"
		c.site = funcSite(f)
	}
	d.components[s] = c
}

// forget drops what s recorded, as it goes.
func (d *devState) forget(s *Scope) {
	delete(d.components, s)
	for _, v := range d.values[s] {
		delete(d.byID, v.id)
	}
	delete(d.values, s)
	if s.comp != nil {
		delete(d.comps, s.comp)
	}
}

// callerSite is the site of the first caller outside core and NSGo's internal packages, skip frames up, as a whole path.
func callerSite(skip int) string {
	var pcs [causeDepth]uintptr
	n := goruntime.Callers(skip+1, pcs[:])
	return appFrame(pcs[:n], pathSite)
}

// funcSite is the line f's code starts at.
func funcSite(f any) string {
	fn := goruntime.FuncForPC(reflect.ValueOf(f).Pointer())
	if fn == nil {
		return ""
	}
	// A function of the packages that run apps is named, not placed: DevTools show it as NSGo's own.
	if runsApps(fn.Name()) {
		return strings.TrimPrefix(fn.Name(), nsgoPackage)
	}
	file, line := fn.FileLine(fn.Entry())
	return pathSite(file, line)
}

// clip shortens a value's text for DevTools.
func clip(s string) string {
	if len(s) > 200 {
		return s[:200] + "…"
	}
	return s
}

// devSurface is a tree as DevTools read and drive it.
type devSurface struct{ t *Tree }

func (s *devSurface) ID() int           { return int(s.t.id) }
func (s *devSurface) Post(fn func())    { s.t.root.Post(fn) }
func (s *devSurface) Highlight(on bool) { s.highlights(func(h *highlights) { h.updates = on }) }

func (s *devSurface) highlights(fn func(*highlights)) {
	if h := s.t.highlights; h != nil {
		fn(h)
	}
}

func (s *devSurface) Inspect(id uint32) {
	s.highlights(func(h *highlights) { h.inspect.Set(s.t.inspected(s.t.node(NodeID(id)))) })
}

func (s *devSurface) Pick(on bool, hover, picked func(id uint32)) {
	s.highlights(func(h *highlights) {
		h.hover, h.picked = hover, picked
		h.picking.Set(on)
		if !on {
			h.inspect.Set(inspection{})
		}
	})
}

func (s *devSurface) Record(on bool, step func(devtools.Step)) {
	if s.t.record != nil {
		s.t.devFlushScrolls()
	}
	s.t.record, s.t.devScrolls = nil, nil
	if on {
		s.t.record = step
	}
}

func (s *devSurface) Set(signal uint64, value string) error {
	d := s.t.rt.dev
	v := d.byID[signal]
	switch {
	case v == nil || v.gone():
		return fmt.Errorf("no signal %d", signal)
	case v.set == nil:
		return fmt.Errorf("DevTools do not edit a %s", v.typ)
	}
	return v.set(value)
}

func (s *devSurface) Hot(n int) []devtools.Computation {
	// Computations of one kind made at one site, as a list's rows' bindings, count as one.
	runs := map[ranKey]*devtools.Computation{}
	for c, dc := range s.t.rt.dev.comps {
		if dc.since == 0 {
			continue
		}
		if !s.ownedByDevTools(&c.Scope) {
			k := ranKey{compKindName(c.kind), s.t.rt.dev.siteOf(c, dc)}
			r := runs[k]
			if r == nil {
				r = &devtools.Computation{Kind: k.kind, Site: k.site}
				runs[k] = r
			}
			r.Runs += dc.since
			r.Took += int64(dc.took)
		}
		dc.since, dc.took = 0, 0
	}
	var out []devtools.Computation
	for _, r := range runs {
		out = append(out, *r)
	}
	slices.SortFunc(out, func(a, b devtools.Computation) int {
		if a.Runs != b.Runs {
			return b.Runs - a.Runs
		}
		return strings.Compare(a.Site, b.Site)
	})
	return out[:min(len(out), n)]
}

// ownedByDevTools reports whether o is in DevTools' own overlay's scope.
func (s *devSurface) ownedByDevTools(o *Scope) bool {
	h := s.t.highlights
	if h == nil {
		return false
	}
	for ; o != nil; o = o.parent {
		if o == h.owner {
			return true
		}
	}
	return false
}

func compKindName(k compKind) string {
	switch k {
	case memoKind:
		return "memo"
	case renderEffect:
		return "binding"
	}
	return "effect"
}

var kindNames = map[driver.ViewKind]string{
	driver.KindView: "View", driver.KindText: "Text", driver.KindInput: "Input", driver.KindScroll: "Scroll",
	driver.KindImage: "Image", driver.KindModal: "Modal", driver.KindCanvas: "Canvas", driver.KindWeb: "WebView",
	driver.KindVideo: "Video", driver.KindNative: "Native", driver.KindShader: "Shader",
}

// Elements returns the tree's views from its root, leaving out DevTools' own.
func (s *devSurface) Elements() devtools.Node {
	t := s.t
	var walk func(n *node) []devtools.Node
	walk = func(n *node) []devtools.Node {
		var out []devtools.Node
		for _, c := range n.children {
			if c.deleted || t.highlights != nil && t.highlights.owns(c) {
				continue
			}
			if c.id == 0 {
				out = append(out, walk(c)...)
				continue
			}
			out = append(out, s.element(c, walk))
		}
		return out
	}
	return s.element(t.view, walk)
}

func (s *devSurface) element(n *node, walk func(*node) []devtools.Node) devtools.Node {
	t := s.t
	b := t.surfaceBounds(n)
	e := devtools.Node{ID: uint32(n.id), Kind: kindNames[n.kind], TestID: n.props.TestID, Role: driver.RoleName(n.props.Role),
		Label: n.props.Label, Bounds: [4]float32{float32(b.Origin.X), float32(b.Origin.Y), float32(b.Size.Width), float32(b.Size.Height)},
		Hidden: n.hidden || n.style.Display == DisplayNone, Style: styleProps(n.style), Selector: selectorFor(n)}
	if e.Kind == "" {
		e.Kind = "View"
	}
	if n.cls != nil && n.cls.list != nil {
		e.Classes = strings.TrimSpace(n.cls.list.class)
	}
	l := &n.lstate.Rounded
	e.Box = devtools.Box{Margin: fourSides(l.Margin.Top, l.Margin.Right, l.Margin.Bottom, l.Margin.Left),
		Border: fourSides(l.Border.Top, l.Border.Right, l.Border.Bottom, l.Border.Left), Padding: fourSides(l.Padding.Top, l.Padding.Right, l.Padding.Bottom, l.Padding.Left)}
	if n.text != nil {
		e.Text = clip(n.text.content)
	}
	if n.input != nil {
		e.Text = clip(n.input.value)
	}
	if d := t.rt.dev; d != nil {
		for o := n.owner; o != nil; o = o.parent {
			if c, ok := d.components[o]; ok {
				e.Component, e.Site = c.name, c.site
				break
			}
		}
	}
	e.Children = walk(n)
	return e
}

func fourSides(top, right, bottom, left float32) [4]float32 {
	return [4]float32{top, right, bottom, left}
}

// styleProps are the style's fields a view sets, by their Go names; a state's style (Hovered, Disabled) lists the fields it sets.
func styleProps(st Style) []devtools.Prop {
	var out []devtools.Prop
	v := reflect.ValueOf(st)
	for i := range v.NumField() {
		f := v.Field(i)
		if !v.Type().Field(i).IsExported() || f.IsZero() || f.Kind() == reflect.Func {
			continue
		}
		value := fmt.Sprint(f.Interface())
		if c, ok := f.Interface().(Color); ok {
			value = fmt.Sprintf("#%08x", uint32(c))
		}
		if f.Kind() == reflect.Pointer && f.Elem().Kind() == reflect.Struct {
			if s, ok := f.Interface().(*Style); ok {
				var set []string
				for _, p := range styleProps(*s) {
					set = append(set, p.Name+": "+p.Value)
				}
				value = "{" + strings.Join(set, ", ") + "}"
			}
		}
		out = append(out, devtools.Prop{Name: v.Type().Field(i).Name, Value: clip(value)})
	}
	return out
}

// selectorFor is how an nsgotest step names n: its test ID, else its role and label, else its text; "" when none is set.
func selectorFor(n *node) string {
	switch {
	case n.props.TestID != "":
		return "id:" + n.props.TestID
	case n.props.Role != 0 && driver.RoleName(n.props.Role) != "" && n.props.Label != "":
		return "role:" + driver.RoleName(n.props.Role) + " " + n.props.Label
	case n.props.Label != "":
		return "label:" + n.props.Label
	case n.text != nil && strings.TrimSpace(n.text.content) != "":
		return "text:" + strconv.Quote(strings.TrimSpace(n.text.content))
	}
	return ""
}

// maxOwners bounds the owners Owners returns, as a table's cells can make thousands.
const maxOwners = 4000

// Owners returns the graph's owners from its root, the values and computations each holds; owners holding none and none under them are left out.
func (s *devSurface) Owners() devtools.Owner {
	d := s.t.rt.dev
	d.sweep()
	count := 0
	var walk func(o *Scope) (devtools.Owner, bool)
	walk = func(o *Scope) (devtools.Owner, bool) {
		count++
		out := devtools.Owner{Name: "scope"}
		if c, ok := d.components[o]; ok {
			out.Name, out.Site = c.name, c.site
		}
		if o.comp != nil {
			kind := compKindName(o.comp.kind)
			out.Name = kind
			if dc := d.comps[o.comp]; dc != nil {
				out.Site = d.siteOf(o.comp, dc)
				c := devtools.Computation{Kind: kind, Site: out.Site, Runs: dc.runs}
				if dc.value != nil {
					c.Value = clip(dc.value())
					if o.comp.subs != nil {
						c.Readers = len(o.comp.subs.list)
					}
				}
				out.Computations = []devtools.Computation{c}
			}
		}
		for _, v := range d.values[o] {
			text := v.get()
			out.Values = append(out.Values, devtools.Value{ID: v.id, Site: v.site, Type: v.typ, Text: text, Editable: v.set != nil, Readers: v.readers()})
		}
		for c := o.first; c != nil && count < maxOwners; c = c.next {
			if h := s.t.highlights; h != nil && c == h.owner {
				continue
			}
			if child, ok := walk(c); ok {
				out.Children = append(out.Children, child)
			}
		}
		keep := len(out.Values) > 0 || len(out.Children) > 0 || out.Site != "" || o.parent == nil
		if _, ok := d.components[o]; ok {
			keep = true
		}
		return out, keep
	}
	root, _ := walk(s.t.rt.root)
	root.Name = "root"
	return root
}

// devRecordPress records a press on n while the tree records steps.
func (t *Tree) devRecordPress(n *node) {
	if t.highlights != nil && t.highlights.owns(n) {
		return
	}
	t.record(devtools.Step{Action: "Press", Selector: selectorFor(n)})
}

// devRecordScroll keeps where scroll view n came to: a scroll is one step however many offsets it passes, recorded once the user does something else, the view comes to rest or recording ends.
func (t *Tree) devRecordScroll(n *node, offset Point) {
	if t.highlights != nil && t.highlights.owns(n) {
		return
	}
	if t.devScrolls == nil {
		t.devScrolls = map[*node]Point{}
	}
	t.devScrolls[n] = offset
}

// devFlushScrolls records the scrolls kept as ScrollTo steps.
func (t *Tree) devFlushScrolls() {
	views := make([]*node, 0, len(t.devScrolls))
	for n := range t.devScrolls {
		views = append(views, n)
	}
	// Views in the order of their IDs, so a recording reads the same each time.
	slices.SortFunc(views, func(a, b *node) int { return cmp.Compare(a.id, b.id) })
	for _, n := range views {
		p := t.devScrolls[n]
		t.record(devtools.Step{Action: "ScrollTo", Selector: selectorFor(n), Offset: [2]float32{float32(p.X), float32(p.Y)}})
	}
	clear(t.devScrolls)
}

// devRecordEdit records an edit of n's text from before to after as Clear and Type steps.
func (t *Tree) devRecordEdit(n *node, before, after string) {
	sel := selectorFor(n)
	switch {
	case after == "":
		t.record(devtools.Step{Action: "Clear", Selector: sel})
	case strings.HasPrefix(after, before):
		t.record(devtools.Step{Action: "Type", Selector: sel, Text: after[len(before):]})
	default:
		t.record(devtools.Step{Action: "Clear", Selector: sel})
		t.record(devtools.Step{Action: "Type", Selector: sel, Text: after})
	}
}
