package nsgotest

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/oneo-me/nsgo/internal/driver"
)

// selector is a parsed selector: its parts in order, each looking inside
// what the one before matched.
type selector struct {
	parts  []part
	source string
}

// part is one matcher of a selector and the match it keeps.
type part struct {
	kind string // id, label, text, role, view or "" for a bare one
	text textMatch
	role string
	// named is set when a role part names its view.
	named bool
	view  driver.NodeID
	nth   int // -1 keeps every match
}

// textMatch is the text a matcher wants: a substring (text:), the whole
// text, spaces around it aside (label:, bare), or a regular expression's
// match.
type textMatch struct {
	want  string
	whole bool
	re    *regexp.Regexp
}

func (m textMatch) matches(s string) bool {
	switch {
	case m.re != nil:
		return m.re.MatchString(s)
	case m.whole:
		return strings.TrimSpace(s) == m.want
	}
	return strings.Contains(s, m.want)
}

// parseText reads a matcher's text: /pattern/flags is a regular
// expression, with JavaScript's i, m and s flags; anything else is
// literal.
func parseText(s string, whole bool, source string) (textMatch, error) {
	if rest, ok := strings.CutPrefix(s, "/"); ok {
		if end := strings.LastIndex(rest, "/"); end >= 0 {
			pattern, flags := rest[:end], rest[end+1:]
			var inline string
			for _, f := range flags {
				switch f {
				case 'i', 'm', 's':
					inline += string(f)
				case 'u', 'g', 'y', 'd':
				default:
					return textMatch{}, fmt.Errorf("%q: %q is not a regular expression flag", source, f)
				}
			}
			if inline != "" {
				pattern = "(?" + inline + ")" + pattern
			}
			re, err := regexp.Compile(pattern)
			if err != nil {
				return textMatch{}, fmt.Errorf("%q: %v", source, err)
			}
			return textMatch{re: re}, nil
		}
	}
	if whole {
		s = strings.TrimSpace(s)
	}
	return textMatch{want: s, whole: whole}, nil
}

func parseSelector(s string) (*selector, error) {
	sel := &selector{source: s}
	for _, piece := range strings.Split(s, " >> ") {
		piece = strings.TrimSpace(piece)
		if n, ok := strings.CutPrefix(piece, "nth="); ok {
			at, err := strconv.Atoi(n)
			if err != nil || at < 0 {
				return nil, fmt.Errorf("%q: %q is not an index", s, piece)
			}
			if len(sel.parts) == 0 {
				return nil, fmt.Errorf("%q: nth= follows what it picks from", s)
			}
			sel.parts[len(sel.parts)-1].nth = at
			continue
		}
		p := part{nth: -1}
		kind, arg, _ := strings.Cut(piece, ":")
		var err error
		switch kind {
		case "id":
			p.kind, p.text = kind, textMatch{want: arg, whole: true}
		case "label":
			p.kind = kind
			p.text, err = parseText(arg, true, s)
		case "text":
			p.kind = kind
			p.text, err = parseText(arg, false, s)
		case "role":
			p.kind = kind
			role, name, named := strings.Cut(arg, " ")
			p.role, p.named = role, named
			if named {
				p.text, err = parseText(name, true, s)
			}
		case "view":
			p.kind = kind
			id, perr := strconv.ParseUint(arg, 10, 32)
			if perr != nil {
				return nil, fmt.Errorf("%q: %q is not a view id", s, arg)
			}
			p.view = driver.NodeID(id)
		default:
			p.text = textMatch{want: strings.TrimSpace(piece), whole: true}
		}
		if err != nil {
			return nil, err
		}
		sel.parts = append(sel.parts, p)
	}
	if len(sel.parts) == 0 {
		return nil, fmt.Errorf("%q: an empty selector", s)
	}
	return sel, nil
}

// clip is the area a view's parent lets show, in surface points; all
// lets everything show.
type clip struct {
	all  bool
	area driver.Bounds
}

func (c clip) shows(b driver.Bounds) bool {
	if c.all {
		return true
	}
	return b.Origin.X < c.area.Origin.X+c.area.Size.Width && c.area.Origin.X < b.Origin.X+b.Size.Width &&
		b.Origin.Y < c.area.Origin.Y+c.area.Size.Height && c.area.Origin.Y < b.Origin.Y+b.Size.Height
}

func (c clip) within(b driver.Bounds) clip {
	if c.all {
		return clip{area: b}
	}
	x0, y0 := max(c.area.Origin.X, b.Origin.X), max(c.area.Origin.Y, b.Origin.Y)
	x1 := min(c.area.Origin.X+c.area.Size.Width, b.Origin.X+b.Size.Width)
	y1 := min(c.area.Origin.Y+c.area.Size.Height, b.Origin.Y+b.Size.Height)
	return clip{area: driver.Bounds{Origin: driver.Point{X: x0, Y: y0},
		Size: driver.Size{Width: max(x1-x0, 0), Height: max(y1-y0, 0)}}}
}

// shown returns the views a user sees, in tree order, as Playwright
// decides visibility: those with an area that nothing around them hides
// or clips away (Style.Clip, a scroll view the user cannot scroll, the
// window). A scroll view the user can scroll shows all of its content
// after its start, which scrolling brings in; a modal shows over the
// window wherever it is in the tree. Header views and the bar above the
// keyboard show where the platform puts them. Opacity and transforms do
// not hide.
func (a *App) shown() []*View { return a.shownWith(false) }

// shownWith is shown, with images of no size where they would show when
// unsized: an image that sizes itself as it loads.
func (a *App) shownWith(unsized bool) []*View {
	var out []*View
	window := clip{area: driver.Bounds{Size: a.size}}
	if r := a.views[a.rootID]; r != nil && r.Frame.Size.Width > 0 {
		// The surface the platform gives, which a phone fixes.
		window.area.Size = r.Frame.Size
	}
	var visit func(v *View, origin driver.Point, c clip, placed bool)
	visit = func(v *View, origin driver.Point, c clip, placed bool) {
		if v.Style.Hidden {
			return
		}
		if v.Kind == driver.KindModal {
			origin, c, placed = driver.Point{}, window, true
		}
		b := v.Frame
		b.Origin.X += origin.X
		b.Origin.Y += origin.Y
		if placed || b.Size.Width > 0 && b.Size.Height > 0 && c.shows(b) {
			out = append(out, v)
		} else if unsized && v.Kind == driver.KindImage && c.shows(driver.Bounds{Origin: b.Origin, Size: driver.Size{Width: max(b.Size.Width, 1), Height: max(b.Size.Height, 1)}}) {
			out = append(out, v)
		}
		inner := c
		content := b.Origin
		switch {
		case v.Kind == driver.KindScroll:
			content.X -= v.Offset.X
			content.Y -= v.Offset.Y
			switch {
			case v.Scroll.Behavior.NoScroll:
				inner = c.within(b)
			case c.shows(b) || placed:
				// Any content scrolls into view, but nothing before its
				// start along its axis, where lists park unused cells.
				const far = 1e9
				area := driver.Bounds{Origin: driver.Point{X: -far, Y: content.Y}, Size: driver.Size{Width: 2 * far, Height: 2 * far}}
				if v.Scroll.Horizontal {
					area.Origin = driver.Point{X: content.X, Y: -far}
				}
				inner = clip{area: area}
			default:
				inner = clip{area: driver.Bounds{}}
			}
		case v.Style.Clip && v.ID != a.rootID:
			inner = c.within(b)
		}
		for _, k := range v.Children {
			visit(k, content, inner, false)
		}
	}
	if r := a.views[a.rootID]; r != nil {
		visit(r, driver.Point{}, window, false)
	}
	// The views the platform places itself: the header views of the
	// screens shown and the bar above the keyboard of the focused field.
	shownSet := map[*View]bool{}
	for _, v := range out {
		shownSet[v] = true
	}
	for _, v := range append([]*View(nil), out...) {
		var extra []driver.NodeID
		if v.Screen != (driver.ScreenConfig{}) && !v.Screen.HeaderHidden {
			extra = append(extra, v.Screen.Left, v.Screen.Right, v.Screen.TitleView)
		}
		if v.Focused && v.Kind == driver.KindInput {
			extra = append(extra, v.Input.Accessory)
		}
		for _, id := range extra {
			if r := a.views[id]; id != 0 && r != nil && r.Parent == nil && !shownSet[r] {
				visit(r, driver.Point{}, clip{all: true}, true)
			}
		}
	}
	return out
}

// byLayer returns the views shown in the order a user's eyes find them:
// the topmost modal's first, then the rest in tree order.
func (a *App) byLayer() []*View { return a.byLayerWith(false) }

func (a *App) byLayerWith(unsized bool) []*View {
	views := a.shownWith(unsized)
	var top *View
	for _, v := range views {
		if v.Kind == driver.KindModal {
			top = v
		}
	}
	if top == nil {
		return views
	}
	first := make([]*View, 0, len(views))
	var rest []*View
	for _, v := range views {
		if v.within(top) {
			first = append(first, v)
		} else {
			rest = append(rest, v)
		}
	}
	return append(first, rest...)
}

// resolve returns the views shown that sel matches, by layer, each part
// looking inside what the one before matched.
func (a *App) resolve(sel *selector) []*View { return a.resolveWith(sel, false) }

// resolveWith is resolve, with unsized images when unsized.
func (a *App) resolveWith(sel *selector, unsized bool) []*View {
	layers := a.byLayerWith(unsized)
	var scope []*View
	for i, p := range sel.parts {
		var found []*View
		inside := func(v *View) bool {
			if i == 0 {
				return true
			}
			for _, s := range scope {
				if v != s && v.within(s) {
					return true
				}
			}
			return false
		}
		match := func(test func(*View) bool) {
			for _, v := range layers {
				if inside(v) && test(v) {
					found = append(found, v)
				}
			}
		}
		switch p.kind {
		case "id":
			match(func(v *View) bool { return v.Props.TestID == p.text.want })
		case "label":
			match(func(v *View) bool { return v.Props.Label != "" && p.text.matches(v.Props.Label) })
		case "text":
			match(func(v *View) bool { return v.showsText(p.text) })
		case "role":
			match(func(v *View) bool {
				return v.Props.Role != driver.RoleNone && roleName(v.Props.Role) == p.role &&
					(!p.named || p.text.matches(v.Props.Label) || p.text.re == nil && strings.Contains(v.textWithin(), p.text.want) ||
						p.text.re != nil && p.text.re.MatchString(v.textWithin()))
			})
		case "view":
			match(func(v *View) bool { return v.ID == p.view })
		default:
			match(func(v *View) bool { return v.Props.TestID == p.text.want })
			if len(found) == 0 {
				match(func(v *View) bool { return v.Props.Label != "" && p.text.matches(v.Props.Label) })
			}
			if len(found) == 0 {
				match(func(v *View) bool { return v.showsText(p.text) })
			}
		}
		if p.nth >= 0 {
			if p.nth < len(found) {
				found = found[p.nth : p.nth+1]
			} else {
				found = nil
			}
		}
		scope = found
	}
	return scope
}

// showsText reports whether v reads text: a text view's text, a canvas
// node's label, an input's text or, while it is empty, its placeholder.
func (v *View) showsText(m textMatch) bool {
	switch {
	case v.Node != nil:
		return m.matches(v.Node.Label)
	case v.Kind == driver.KindText:
		return m.matches(v.Text)
	case v.Kind == driver.KindInput:
		return v.Text != "" && m.matches(v.Text) || v.Text == "" && v.Input.Placeholder != "" && m.matches(v.Input.Placeholder)
	}
	return false
}

func (v *View) textWithin() string {
	var b strings.Builder
	var walk func(*View)
	walk = func(v *View) {
		if v.Kind == driver.KindText {
			if b.Len() > 0 {
				b.WriteByte(' ')
			}
			b.WriteString(strings.TrimSpace(v.Text))
		}
		for _, c := range v.Children {
			walk(c)
		}
	}
	walk(v)
	return b.String()
}

var roles = map[driver.Role]string{
	driver.RoleButton: "button", driver.RoleLink: "link", driver.RoleHeader: "header", driver.RoleImage: "image",
	driver.RoleCheckbox: "checkbox", driver.RoleSwitch: "switch", driver.RoleRadio: "radio", driver.RoleTab: "tab",
	driver.RoleText: "text", driver.RoleTextField: "textbox", driver.RoleAdjustable: "adjustable",
	driver.RoleProgressBar: "progressbar", driver.RoleRadioGroup: "radiogroup", driver.RoleTabList: "tablist",
	driver.RoleMenu: "menu", driver.RoleMenuItem: "menuitem", driver.RoleDialog: "dialog", driver.RoleAlert: "alert",
	driver.RoleTable: "table", driver.RoleGrid: "grid", driver.RoleRowGroup: "rowgroup", driver.RoleRow: "row",
	driver.RoleColumnHeader: "columnheader", driver.RoleCell: "cell",
}

func roleName(r driver.Role) string { return roles[r] }

// disabled reports whether v, or a view around it up to until (the root
// when nil), takes no input.
func (v *View) disabled(until *View) bool {
	for p := v; p != nil; p = p.Parent {
		if p.Props.Disabled || p.Kind == driver.KindInput && p.Input.Disabled {
			return true
		}
		if p == until {
			return false
		}
	}
	return false
}

// upTo returns v or its nearest ancestor that ok accepts.
func (v *View) upTo(ok func(*View) bool) *View {
	for p := v; p != nil; p = p.Parent {
		if ok(p) {
			return p
		}
	}
	return nil
}

// downTo returns v or the first view inside it, in tree order, that ok
// accepts.
func (v *View) downTo(ok func(*View) bool) *View {
	if ok(v) {
		return v
	}
	for _, c := range v.Children {
		if c.Style.Hidden {
			continue
		}
		if f := c.downTo(ok); f != nil {
			return f
		}
	}
	return nil
}

// pick resolves selector to the one view its matches lead to through
// target (several matches leading to one view are one), failing the test
// when none or several do; what names the view in the message, and a
// target that reports disabled fails as well.
func (a *App) pick(selector, what string, target func(*View) (*View, bool)) *View {
	a.t.Helper()
	sel, err := parseSelector(selector)
	if err != nil {
		a.t.Fatalf("nsgotest: %v", err)
	}
	var out []*View
	off := 0
	for _, v := range a.resolve(sel) {
		t, ok := target(v)
		switch {
		case t == nil:
		case !ok:
			off++
		case !containsView(out, t):
			out = append(out, t)
		}
	}
	switch {
	case len(out) == 1:
		return out[0]
	case len(out) > 1:
		a.t.Fatalf("nsgotest: %q matches %d %ss: say which (%q), or look inside a view around it (\"id:list >> %s\")\n%s",
			selector, len(out), what, selector+" >> nth=0", selector, a.Dump())
	case off > 0:
		a.t.Fatalf("nsgotest: %q is a disabled %s\n%s", selector, what, a.Dump())
	default:
		a.t.Fatalf("nsgotest: no %s matches %q\n%s", what, selector, a.Dump())
	}
	return nil
}

func containsView(vs []*View, v *View) bool {
	for _, o := range vs {
		if o == v {
			return true
		}
	}
	return false
}
