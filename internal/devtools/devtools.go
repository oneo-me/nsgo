// Package devtools is what nsgo dev -devtools reads from a running app and sends it: core registers each surface's tree here while Enabled, and nsgo's dev client serves them to the DevTools page. Its types are what the page receives as JSON.
package devtools

import "sync"

// Enabled has core record what DevTools show, and register its trees: nsgo's dev client sets it under nsgo dev -devtools, before any graph starts. While it is unset, core records nothing and pays one check where it would.
var Enabled bool

// ScriptSite, which tsx sets while Enabled, is the line of TSX its JS runs now, through the bundles' source maps; ok is false while none runs. A site that falls in tsx's Go code takes it, so DevTools name the app's TSX rather than the Go that ran it.
var ScriptSite func() (file string, line int, ok bool)

// Surface is one surface's tree, as DevTools read and drive it. Every method but Post runs on the surface's graph goroutine: call it through Post.
type Surface interface {
	// ID numbers the surface within the app, 0 for the main window's.
	ID() int
	// Post runs fn on the surface's graph goroutine, from any goroutine.
	Post(fn func())
	// Elements returns the surface's views as a tree, from its root view.
	Elements() Node
	// Owners returns the graph's owners as a tree, from its root, with the values and computations each holds.
	Owners() Owner
	// Inspect outlines view id on the surface, or nothing for 0.
	Inspect(id uint32)
	// Highlight turns the outlines of what each commit changed on or off.
	Highlight(on bool)
	// Pick has the next press on the surface choose a view rather than reach the app, reporting each view the pointer is over to hover and the one pressed to picked; off ends it.
	Pick(on bool, hover, picked func(id uint32))
	// Record reports what the user does on the surface as test steps, until off.
	Record(on bool, step func(Step))
	// Set writes value, as text, to the signal numbered signal: strings, booleans and numbers only.
	Set(signal uint64, value string) error
	// Hot returns the computations that ran most since the last call, at most n.
	Hot(n int) []Computation
}

// Node is a view and the views in it.
type Node struct {
	ID uint32 `json:"id"`
	// Kind is the view's kind: View, Text, Input, Scroll, Image and the others.
	Kind string `json:"kind"`
	// Component is the nearest component around the view, as its Go type (ui.Button), and Site the line of the function that built it, as /path/to/file.go:line, where there is one.
	Component string     `json:"component,omitempty"`
	Site      string     `json:"site,omitempty"`
	TestID    string     `json:"testID,omitempty"`
	Role      string     `json:"role,omitempty"`
	Label     string     `json:"label,omitempty"`
	Text      string     `json:"text,omitempty"`
	Selector  string     `json:"selector,omitempty"`
	Bounds    [4]float32 `json:"bounds"`
	Hidden    bool       `json:"hidden,omitempty"`
	// Classes are the view's Tailwind classes, and Box its margin, border and padding as laid out.
	Classes  string `json:"classes,omitempty"`
	Box      Box    `json:"box"`
	Style    []Prop `json:"style,omitempty"`
	Children []Node `json:"children,omitempty"`
}

// Box is a view's margin, border and padding as laid out, each top, right, bottom and left.
type Box struct {
	Margin  [4]float32 `json:"margin"`
	Border  [4]float32 `json:"border"`
	Padding [4]float32 `json:"padding"`
}

// Prop is a style property a view sets, by its Go name.
type Prop struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// Owner is an owner of the graph: a component's scope, a computation's, a region's or the root, with the values and computations it holds and the owners under it.
type Owner struct {
	Name         string        `json:"name"`
	Site         string        `json:"site,omitempty"`
	Values       []Value       `json:"values,omitempty"`
	Computations []Computation `json:"computations,omitempty"`
	Children     []Owner       `json:"children,omitempty"`
}

// Value is a signal: where it was made, its Go type and its value as text, and whether Set takes it.
type Value struct {
	ID       uint64 `json:"id"`
	Site     string `json:"site,omitempty"`
	Type     string `json:"type"`
	Text     string `json:"text"`
	Editable bool   `json:"editable,omitempty"`
	// Readers are the computations that read it now: none for a value no binding shows.
	Readers int `json:"readers"`
}

// Computation is a memo, a binding or an effect: where it was made and how often it ran.
type Computation struct {
	Kind string `json:"kind"`
	Site string `json:"site,omitempty"`
	Runs int    `json:"runs"`
	// Took is the time its runs took, in nanoseconds, not counting what their writes ran next.
	Took int64 `json:"took,omitempty"`
	// Value is a memo's value as text, and Readers the computations that read it.
	Value   string `json:"value,omitempty"`
	Readers int    `json:"readers,omitempty"`
}

// Step is what the user did, as a test step: Press, Type or Clear on the view Selector names, Type with Text, or ScrollTo with the Offset a scroll view came to.
type Step struct {
	Action   string     `json:"action"`
	Selector string     `json:"selector"`
	Text     string     `json:"text,omitempty"`
	Offset   [2]float32 `json:"offset,omitempty"`
}

var (
	mu       sync.Mutex
	surfaces []Surface
)

// Register adds s to the surfaces DevTools show.
func Register(s Surface) {
	mu.Lock()
	defer mu.Unlock()
	surfaces = append(surfaces, s)
}

// Unregister removes s.
func Unregister(s Surface) {
	mu.Lock()
	defer mu.Unlock()
	for i, x := range surfaces {
		if x == s {
			surfaces = append(surfaces[:i], surfaces[i+1:]...)
			return
		}
	}
}

// Surfaces returns the surfaces registered, the main window's first.
func Surfaces() []Surface {
	mu.Lock()
	defer mu.Unlock()
	return append([]Surface(nil), surfaces...)
}
