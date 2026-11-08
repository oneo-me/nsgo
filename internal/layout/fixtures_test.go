package layout

import (
	"archive/tar"
	"compress/gzip"
	"encoding/xml"
	"fmt"
	"io"
	"os"
	"path"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// The fixture tests run Taffy's XML test suite (testdata/README.md) the way
// Taffy's tests/xml.rs does: same styles, same Ahem text measurement, same
// viewport, rounding and tolerances.

// fixtureCategories are the fixture directories, in the order reported.
var fixtureCategories = []string{"block", "blockflex", "blockgrid", "contain", "flex", "grid", "gridflex", "leaf"}

// testNode is a fixture node: Taffy's TaffyTree node with a TestNodeContext.
type testNode struct {
	style    Style
	parent   *testNode
	children []*testNode
	// content measures the node's content (Taffy's TestNodeContext); nil is
	// no context.
	content func(known Size[Opt], available Size[AvailableSpace]) Size[float32]
	// count is the number of times the node was measured.
	count int
	// onMeasure, if set, sees every measurement's inputs.
	onMeasure func(known Size[Opt], available Size[AvailableSpace])
	state     State
}

func (n *testNode) Style() *Style       { return &n.style }
func (n *testNode) ChildCount() int     { return len(n.children) }
func (n *testNode) Child(i int) Node    { return n.children[i] }
func (n *testNode) LayoutState() *State { return &n.state }

// Measure is Taffy's test_measure_function.
func (n *testNode) Measure(known Size[Opt], available Size[AvailableSpace]) Size[float32] {
	if n.onMeasure != nil {
		n.onMeasure(known, available)
	}
	if optsBoth(known) {
		return optsUnwrapOr(known, Size[float32]{})
	}
	if n.content == nil {
		return optsUnwrapOr(known, Size[float32]{})
	}
	n.count++
	size := n.content(known, available)
	return optsUnwrapOr(known, size)
}

// ahemText is text in the Ahem font, where every glyph is a 10px square;
// zero-width spaces are the only break opportunities.
type ahemText struct {
	text     string
	vertical bool
}

func (a *ahemText) measure(known Size[Opt], available Size[AvailableSpace]) Size[float32] {
	const hWidth, hHeight = 10, 10
	inlineAxis := Horizontal
	if a.vertical {
		inlineAxis = Vertical
	}
	blockAxis := inlineAxis.other()
	lines := strings.Split(a.text, "​")
	minLineLength, maxLineLength := 0, 0
	for _, l := range lines {
		minLineLength = max(minLineLength, len(l))
		maxLineLength += len(l)
	}
	inlineSize := known.get(inlineAxis)
	var inline float32
	if inlineSize.Valid {
		inline = inlineSize.Value
	} else {
		switch av := available.get(inlineAxis); av.Kind {
		case SpaceMinContent:
			inline = float32(minLineLength) * hWidth
		case SpaceMaxContent:
			inline = float32(maxLineLength) * hWidth
		default:
			inline = fmin(av.Value, float32(maxLineLength)*hWidth)
		}
	}
	inline = fmax(inline, float32(minLineLength)*hWidth)
	var block float32
	if b := known.get(blockAxis); b.Valid {
		block = b.Value
	} else {
		inlineLineLength := int(floor32(inline / hWidth))
		lineCount, current := 1, 0
		for _, l := range lines {
			if current+len(l) > inlineLineLength {
				if current > 0 {
					lineCount++
				}
				current = len(l)
			} else {
				current += len(l)
			}
		}
		block = float32(lineCount) * hHeight
	}
	if a.vertical {
		return Size[float32]{block, inline}
	}
	return Size[float32]{inline, block}
}

// xmlElement is a parsed XML element.
type xmlElement struct {
	name     string
	attrs    map[string]string
	children []*xmlElement
	// text is the first child's text, when the first child is text.
	text    string
	hasText bool
}

func (e *xmlElement) child(name string) *xmlElement {
	for _, c := range e.children {
		if c.name == name {
			return c
		}
	}
	return nil
}

func parseXML(data []byte) (*xmlElement, error) {
	d := xml.NewDecoder(strings.NewReader(string(data)))
	var stack []*xmlElement
	var root *xmlElement
	// first tracks, per open element, whether it has seen a child yet.
	var first []bool
	for {
		tok, err := d.Token()
		if err == io.EOF {
			return root, nil
		}
		if err != nil {
			return nil, err
		}
		switch t := tok.(type) {
		case xml.StartElement:
			e := &xmlElement{name: t.Name.Local, attrs: map[string]string{}}
			for _, a := range t.Attr {
				e.attrs[a.Name.Local] = a.Value
			}
			if len(stack) > 0 {
				parent := stack[len(stack)-1]
				parent.children = append(parent.children, e)
				first[len(first)-1] = false
			} else {
				root = e
			}
			stack = append(stack, e)
			first = append(first, true)
		case xml.EndElement:
			stack = stack[:len(stack)-1]
			first = first[:len(first)-1]
		case xml.CharData:
			if len(stack) > 0 {
				if first[len(first)-1] {
					e := stack[len(stack)-1]
					e.text += string(t)
					e.hasText = true
				}
			}
		case xml.Comment:
			if len(first) > 0 {
				first[len(first)-1] = false
			}
		}
	}
}

// buildStyle is the build_style of Taffy's xml.rs.
func buildStyle(e *xmlElement) (Style, error) {
	var s Style
	var errs []error
	attr := func(name string, parse func(string) error) {
		if v, ok := e.attrs[name]; ok {
			if err := parse(v); err != nil {
				errs = append(errs, fmt.Errorf("%s: %w", name, err))
			}
		}
	}
	set := func(dst *Length, p func(string) (Length, error)) func(string) error {
		return func(v string) (err error) { *dst, err = p(v); return }
	}
	keyword := func(dst any) func(string) error {
		return func(v string) (err error) {
			switch d := dst.(type) {
			case *Display:
				*d, err = parseDisplay(v)
			case *Direction:
				*d, err = parseDirection(v)
			case *BoxSizing:
				*d, err = parseBoxSizing(v)
			case *Overflow:
				*d, err = parseOverflow(v)
			case *Contain:
				*d, err = parseContain(v)
			case *Position:
				*d, err = parsePosition(v)
			case *TextAlign:
				*d, err = parseTextAlign(v)
			case *FlexDirection:
				*d, err = parseFlexDirection(v)
			case *FlexWrap:
				*d, err = parseFlexWrap(v)
			case *GridAutoFlow:
				*d, err = parseGridAutoFlow(v)
			case *AlignContent:
				*d, err = parseAlignContent(v)
			case *GridPlacement:
				*d, err = ParseGridPlacement(v)
			}
			return
		}
	}
	number := func(dst *float32) func(string) error {
		return func(v string) (err error) { *dst, err = parseFloat(v); return }
	}

	attr("display", keyword(&s.Display))
	attr("direction", keyword(&s.Direction))
	attr("box-sizing", keyword(&s.BoxSizing))
	attr("overflow-x", keyword(&s.Overflow.X))
	attr("overflow-y", keyword(&s.Overflow.Y))
	attr("scrollbar-width", number(&s.ScrollbarWidth))
	attr("contain", keyword(&s.Contain))
	attr("position", keyword(&s.Position))

	attr("width", set(&s.Size.Width, parseDimension))
	attr("height", set(&s.Size.Height, parseDimension))
	attr("min-width", set(&s.MinSize.Width, parseLengthPercentageAuto))
	attr("min-height", set(&s.MinSize.Height, parseLengthPercentageAuto))
	attr("max-width", set(&s.MaxSize.Width, parseLengthPercentageAuto))
	attr("max-height", set(&s.MaxSize.Height, parseLengthPercentageAuto))
	attr("top", set(&s.Inset.Top, parseLengthPercentageAuto))
	attr("left", set(&s.Inset.Left, parseLengthPercentageAuto))
	attr("bottom", set(&s.Inset.Bottom, parseLengthPercentageAuto))
	attr("right", set(&s.Inset.Right, parseLengthPercentageAuto))
	attr("margin-top", set(&s.Margin.Top, parseLengthPercentageAuto))
	attr("margin-left", set(&s.Margin.Left, parseLengthPercentageAuto))
	attr("margin-bottom", set(&s.Margin.Bottom, parseLengthPercentageAuto))
	attr("margin-right", set(&s.Margin.Right, parseLengthPercentageAuto))
	attr("padding-top", set(&s.Padding.Top, parseLengthPercentage))
	attr("padding-left", set(&s.Padding.Left, parseLengthPercentage))
	attr("padding-bottom", set(&s.Padding.Bottom, parseLengthPercentage))
	attr("padding-right", set(&s.Padding.Right, parseLengthPercentage))
	attr("border-top", set(&s.Border.Top, parseLengthPercentage))
	attr("border-left", set(&s.Border.Left, parseLengthPercentage))
	attr("border-bottom", set(&s.Border.Bottom, parseLengthPercentage))
	attr("border-right", set(&s.Border.Right, parseLengthPercentage))
	attr("column-gap", set(&s.Gap.Width, parseLengthPercentage))
	attr("row-gap", set(&s.Gap.Height, parseLengthPercentage))

	// Taffy's harness parses aspect-ratio leniently: a bad value is none.
	if v, ok := e.attrs["aspect-ratio"]; ok {
		if f, err := parseFloat(v); err == nil {
			s.AspectRatio = Some(f)
		}
	}
	attr("align-items", func(v string) (err error) { s.AlignItems, err = parseAlign(v); return })
	attr("align-self", func(v string) (err error) { s.AlignSelf, err = parseSelfAlign(v); return })
	attr("justify-items", func(v string) (err error) { s.JustifyItems, err = parseAlign(v); return })
	attr("justify-self", func(v string) (err error) { s.JustifySelf, err = parseSelfAlign(v); return })
	attr("align-content", keyword(&s.AlignContent))
	attr("justify-content", keyword(&s.JustifyContent))

	attr("text-align", keyword(&s.TextAlign))
	attr("flex-direction", keyword(&s.FlexDirection))
	attr("flex-wrap", keyword(&s.FlexWrap))
	attr("flex-grow", number(&s.FlexGrow))
	attr("flex-shrink", func(v string) error {
		f, err := parseFloat(v)
		s.FlexShrink = Some(f)
		return err
	})
	attr("flex-basis", set(&s.FlexBasis, parseDimension))

	attr("grid-auto-flow", keyword(&s.GridAutoFlow))
	attr("grid-template-rows", func(v string) (err error) {
		s.GridTemplateRows, s.GridTemplateRowNames, err = ParseGridTemplate(v)
		return
	})
	attr("grid-template-columns", func(v string) (err error) {
		s.GridTemplateColumns, s.GridTemplateColumnNames, err = ParseGridTemplate(v)
		return
	})
	attr("grid-auto-rows", func(v string) (err error) { s.GridAutoRows, err = ParseGridAutoTracks(v); return })
	attr("grid-auto-columns", func(v string) (err error) { s.GridAutoColumns, err = ParseGridAutoTracks(v); return })
	attr("grid-row-start", keyword(&s.GridRow.Start))
	attr("grid-row-end", keyword(&s.GridRow.End))
	attr("grid-column-start", keyword(&s.GridColumn.Start))
	attr("grid-column-end", keyword(&s.GridColumn.End))

	if len(errs) > 0 {
		return s, errs[0]
	}
	return s, nil
}

// expected is a node of a fixture's expectations.
type expected struct {
	x, y, width, height       float32
	scrollWidth, scrollHeight Opt
	rows, columns             string
	hasRows, hasColumns       bool
	children                  []*expected
}

func buildTree(in, exp *xmlElement) (*testNode, *expected, error) {
	style, err := buildStyle(in)
	if err != nil {
		return nil, nil, err
	}
	n := &testNode{style: style}
	e := &expected{}
	for _, f := range []struct {
		name string
		dst  *float32
	}{{"x", &e.x}, {"y", &e.y}, {"width", &e.width}, {"height", &e.height}} {
		v, err := strconv.ParseFloat(exp.attrs[f.name], 32)
		if err != nil {
			return nil, nil, fmt.Errorf("expectation %s: %w", f.name, err)
		}
		*f.dst = float32(v)
	}
	sw, errW := strconv.ParseFloat(exp.attrs["scroll_width"], 32)
	sh, errH := strconv.ParseFloat(exp.attrs["scroll_height"], 32)
	if errW == nil && errH == nil {
		e.scrollWidth, e.scrollHeight = Some(float32(sw)), Some(float32(sh))
	}
	e.rows, e.hasRows = exp.attrs["resolved-rows"]
	e.columns, e.hasColumns = exp.attrs["resolved-columns"]

	if len(in.children) == 0 {
		if in.hasText {
			text := &ahemText{
				text:     strings.TrimSpace(in.text),
				vertical: strings.Contains(in.attrs["writing-mode"], "vertical"),
			}
			n.content = text.measure
		}
		return n, e, nil
	}
	for i, c := range in.children {
		if i >= len(exp.children) {
			break
		}
		cn, ce, err := buildTree(c, exp.children[i])
		if err != nil {
			return nil, nil, err
		}
		n.children = append(n.children, cn)
		e.children = append(e.children, ce)
	}
	return n, e, nil
}

// fixture is one XML fixture.
type fixture struct {
	category, name string
	data           []byte
}

func loadFixtures(t testing.TB) []fixture {
	f, err := os.Open("testdata/taffy-xml.tar.gz")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	tr := tar.NewReader(gz)
	var fixtures []fixture
	for {
		h, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		if h.Typeflag != tar.TypeReg || path.Ext(h.Name) != ".xml" {
			continue
		}
		data, err := io.ReadAll(tr)
		if err != nil {
			t.Fatal(err)
		}
		dir, file := path.Split(h.Name)
		fixtures = append(fixtures, fixture{
			category: path.Base(dir),
			name:     strings.TrimSuffix(file, ".xml"),
			data:     data,
		})
	}
	sort.Slice(fixtures, func(i, j int) bool {
		if fixtures[i].category != fixtures[j].category {
			return fixtures[i].category < fixtures[j].category
		}
		return fixtures[i].name < fixtures[j].name
	})
	return fixtures
}

// skipReason says why a fixture does not apply to NSH's feature set: Taffy
// compiles balanced-wrapping fixtures only with flexbox_balance, and float
// fixtures need float_layout, both disabled in NSH.
func skipReason(data []byte) string {
	s := string(data)
	switch {
	case strings.Contains(s, "balance") || strings.Contains(s, "flex-line-count"):
		return "needs flexbox_balance, which NSH disables"
	case strings.Contains(s, ` float="`) || strings.Contains(s, ` clear="`):
		return "needs float_layout, which NSH disables"
	}
	return ""
}

func TestFixtures(t *testing.T) {
	fixtures := loadFixtures(t)
	type tally struct{ pass, fail, skip int }
	tallies := map[string]*tally{}
	for _, c := range fixtureCategories {
		tallies[c] = &tally{}
	}
	for _, c := range fixtureCategories {
		t.Run(c, func(t *testing.T) {
			for _, f := range fixtures {
				if f.category != c {
					continue
				}
				ok := t.Run(f.name, func(t *testing.T) {
					if reason := skipReason(f.data); reason != "" {
						tallies[c].skip++
						t.Skip(reason)
					}
					runFixture(t, f.data)
				})
				if ok {
					tallies[c].pass++
				} else {
					tallies[c].fail++
				}
			}
		})
	}
	var b strings.Builder
	for _, c := range fixtureCategories {
		tl := tallies[c]
		tl.pass -= tl.skip // skipped subtests report ok
		if tl.pass+tl.fail+tl.skip == 0 {
			continue
		}
		fmt.Fprintf(&b, "\n%-10s %4d passed %4d failed %4d skipped", c, tl.pass, tl.fail, tl.skip)
	}
	t.Log(b.String())
}

func runFixture(t *testing.T, data []byte) {
	doc, err := parseXML(data)
	if err != nil {
		t.Fatal(err)
	}
	useRounding := true
	if v, ok := doc.attrs["use-rounding"]; ok {
		useRounding, err = strconv.ParseBool(v)
		if err != nil {
			t.Fatal(err)
		}
	}
	viewport := doc.child("viewport")
	input := doc.child("input")
	expectations := doc.child("expectations")
	available := Size[AvailableSpace]{MaxContentSpace, MaxContentSpace}
	if v, ok := viewport.attrs["width"]; ok {
		if available.Width, err = parseAvailableSpace(v); err != nil {
			t.Fatal(err)
		}
	}
	if v, ok := viewport.attrs["height"]; ok {
		if available.Height, err = parseAvailableSpace(v); err != nil {
			t.Fatal(err)
		}
	}
	root, want, err := buildTree(input.children[0], expectations.children[0])
	if err != nil {
		t.Fatal(err)
	}

	Compute(root, available)
	if useRounding {
		Round(root, 1)
	}

	if !matches(root, want, useRounding) {
		var b strings.Builder
		fmt.Fprintf(&b, "\nINPUT\n%s\nEXPECTED\n", data)
		printExpected(&b, want, "")
		b.WriteString("ACTUAL\n")
		printActual(&b, root, useRounding, "")
		t.Error(b.String())
	}
}

func finalLayout(n *testNode, rounded bool) Layout {
	if rounded {
		return n.state.Rounded
	}
	return n.state.Unrounded
}

func near(a, b float32) bool { return abs32(a-b) < 0.1 }

func matches(n *testNode, e *expected, rounded bool) bool {
	l := finalLayout(n, rounded)
	if !near(l.Location.X, e.x) || !near(l.Location.Y, e.y) || !near(l.Size.Width, e.width) ||
		!near(l.Size.Height, e.height) {
		return false
	}
	if e.scrollWidth.Valid && (!near(l.ScrollWidth(), e.scrollWidth.Value) ||
		!near(l.ScrollHeight(), e.scrollHeight.Value)) {
		return false
	}
	if g := n.state.grid; g != nil {
		if e.hasRows && !trackListsMatch(e.rows, g.resolvedTrackList(Vertical, 1)) {
			return false
		}
		if e.hasColumns && !trackListsMatch(e.columns, g.resolvedTrackList(Horizontal, 1)) {
			return false
		}
	}
	if len(n.children) != len(e.children) {
		return false
	}
	for i := range n.children {
		if !matches(n.children[i], e.children[i], rounded) {
			return false
		}
	}
	return true
}

// trackListsMatch compares resolved track lists such as "[a] 10px 20.5px
// [b c]", line names exactly and sizes within 0.1.
func trackListsMatch(want, got string) bool {
	parse := func(s string) []string {
		s = strings.TrimSpace(s)
		if s == "none" {
			return nil
		}
		var toks []string
		for s != "" {
			s = strings.TrimLeft(s, " ")
			if s == "" {
				break
			}
			if s[0] == '[' {
				end := strings.IndexByte(s, ']')
				if end < 0 {
					return []string{"<bad>"}
				}
				toks = append(toks, "["+strings.Join(strings.Fields(s[1:end]), " ")+"]")
				s = s[end+1:]
				continue
			}
			end := strings.IndexByte(s, ' ')
			if end < 0 {
				end = len(s)
			}
			toks = append(toks, s[:end])
			s = s[end:]
		}
		return toks
	}
	w, g := parse(want), parse(got)
	if len(w) != len(g) {
		return false
	}
	for i := range w {
		if strings.HasPrefix(w[i], "[") || strings.HasPrefix(g[i], "[") {
			if w[i] != g[i] {
				return false
			}
			continue
		}
		a, errA := strconv.ParseFloat(strings.TrimSuffix(w[i], "px"), 32)
		b, errB := strconv.ParseFloat(strings.TrimSuffix(g[i], "px"), 32)
		if errA != nil || errB != nil || !strings.HasSuffix(w[i], "px") || !strings.HasSuffix(g[i], "px") ||
			!near(float32(a), float32(b)) {
			return false
		}
	}
	return true
}

func printExpected(b *strings.Builder, e *expected, indent string) {
	fmt.Fprintf(b, "%s[x: %-4g y: %-4g w: %-4g h: %-4g]", indent, e.x, e.y, e.width, e.height)
	if e.scrollWidth.Valid {
		fmt.Fprintf(b, " [scroll_w: %g scroll_h: %g]", e.scrollWidth.Value, e.scrollHeight.Value)
	}
	if e.hasRows {
		fmt.Fprintf(b, " [rows: %s]", e.rows)
	}
	if e.hasColumns {
		fmt.Fprintf(b, " [columns: %s]", e.columns)
	}
	b.WriteString("\n")
	for _, c := range e.children {
		printExpected(b, c, indent+"    ")
	}
}

func printActual(b *strings.Builder, n *testNode, rounded bool, indent string) {
	l := finalLayout(n, rounded)
	fmt.Fprintf(b, "%s[x: %-4g y: %-4g w: %-4g h: %-4g] [scroll_w: %g scroll_h: %g]", indent, l.Location.X,
		l.Location.Y, l.Size.Width, l.Size.Height, l.ScrollWidth(), l.ScrollHeight())
	if g := n.state.grid; g != nil {
		fmt.Fprintf(b, " [rows: %s] [columns: %s]", g.resolvedTrackList(Vertical, 1),
			g.resolvedTrackList(Horizontal, 1))
	}
	b.WriteString("\n")
	for _, c := range n.children {
		printActual(b, c, rounded, indent+"    ")
	}
}

// BenchmarkFixtures lays out every applicable fixture from a cold cache.
func BenchmarkFixtures(b *testing.B) {
	type tree struct {
		root      *testNode
		available Size[AvailableSpace]
	}
	var trees []tree
	for _, f := range loadFixtures(b) {
		if skipReason(f.data) != "" {
			continue
		}
		doc, err := parseXML(f.data)
		if err != nil {
			b.Fatal(err)
		}
		available := Size[AvailableSpace]{MaxContentSpace, MaxContentSpace}
		viewport := doc.child("viewport")
		if v, ok := viewport.attrs["width"]; ok {
			available.Width, _ = parseAvailableSpace(v)
		}
		if v, ok := viewport.attrs["height"]; ok {
			available.Height, _ = parseAvailableSpace(v)
		}
		root, _, err := buildTree(doc.child("input").children[0], doc.child("expectations").children[0])
		if err != nil {
			b.Fatal(err)
		}
		trees = append(trees, tree{root, available})
	}
	var reset func(n *testNode)
	reset = func(n *testNode) {
		n.state = State{}
		for _, c := range n.children {
			reset(c)
		}
	}
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		for _, t := range trees {
			b.StopTimer()
			reset(t.root)
			b.StartTimer()
			Compute(t.root, t.available)
			Round(t.root, 1)
		}
	}
}
