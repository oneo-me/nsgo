package richeditor_test

import (
	"strings"
	"testing"

	"github.com/oneo-me/nsgo/base/richeditor"
	"github.com/oneo-me/nsgo/core"
	"github.com/oneo-me/nsgo/nsgotest"
)

func session(t *testing.T, markdown string) *richeditor.Session {
	t.Helper()
	exts := append(richeditor.StarterKit(richeditor.StarterOptions{}), richeditor.TableKit(richeditor.TableOptions{}),
		richeditor.Mention(richeditor.MentionOptions{}))
	s, err := richeditor.NewSession(richeditor.SessionConfig{Extensions: exts, Markdown: markdown})
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func desktop() nsgotest.Option {
	return nsgotest.Environment(core.Environment{OS: core.OSMacOS, Idiom: core.IdiomDesktop})
}

func view(s *richeditor.Session, more ...func(*richeditor.View)) func(*core.Scope) core.Node {
	return func(cx *core.Scope) core.Node {
		v := richeditor.View{Editor: s, TestID: "rich", Label: "Doc", Placeholder: "Write…", Style: core.Style{Width: core.Pt(500), Height: core.Pt(300)}}
		for _, f := range more {
			f(&v)
		}
		return v
	}
}

func TestViewShowsBlocksAndMarks(t *testing.T) {
	s := session(t, "# Title\n\nSome **bold** text\n\n- one\n- two\n\n1. first\n")
	bold := core.Style{FontWeight: core.FontWeightBold}
	app := nsgotest.Start(t, view(s, func(v *richeditor.View) {
		v.Styles.Marks = map[string]core.Style{"bold": bold}
		v.Styles.Nodes = map[string]core.Style{"heading": {FontSize: 24}, "heading[level=1]": {FontSize: 30}}
	}), desktop())
	app.Find("text:Title")
	if got := app.Find("text:Title").TextStyle.Font.Size; got != 30 {
		t.Fatalf("heading font %v", got)
	}
	if style := app.SpanStyle("text:bold"); style.Font.Weight != core.FontWeightBold {
		t.Fatalf("span %+v", style)
	}
	if n := len(app.Query(`text:"•"`)); n != 2 {
		t.Fatalf("%d bullets", n)
	}
	app.Find(`text:"1."`)
}

func TestViewTypesAndMoves(t *testing.T) {
	s := session(t, "")
	app := nsgotest.Start(t, view(s, func(v *richeditor.View) { v.AutoFocus = true }), desktop())
	app.Find("text:Write…")
	field := "role:textbox"
	app.Press("id:rich-editor-content")
	app.Type(field, "hi there")
	if got := s.Text(); got != "hi there" {
		t.Fatalf("typed %q", got)
	}
	if app.Exists("text:Write…") {
		t.Fatal("the placeholder stays")
	}
	app.Key("enter")
	app.Type(field, "next")
	if got := s.Text(); got != "hi there\n\nnext" {
		t.Fatalf("typed %q", got)
	}
	app.Key("left")
	app.Key("left")
	if got := s.State().Selection.Head(); got != 13 {
		t.Fatalf("head %d", got)
	}
	app.Key("up")
	app.Settle()
	if got := s.State().Selection.FromPos().Parent().TextContent(); got != "hi there" {
		t.Fatalf("up went to %q", got)
	}
	app.Key("shift+end")
	if s.State().Selection.Empty() {
		t.Fatal("shift+end selected nothing")
	}
	app.Settle()
	if len(app.Query("id:rich-editor-selection")) == 0 {
		t.Fatal("no selection box")
	}
	app.Key("mod+b")
	if md := s.Markdown(); !strings.Contains(md, "**") {
		t.Fatalf("no bold: %q", md)
	}
	app.Key("mod+z")
	if md := s.Markdown(); strings.Contains(md, "**") {
		t.Fatalf("undo left %q", md)
	}
}

func TestViewPressesAndWords(t *testing.T) {
	s := session(t, "one two three\n\nsecond line\n")
	app := nsgotest.Start(t, view(s), desktop())
	app.Settle()
	app.Press("text:one two three")
	app.Press("text:one two three")
	if got := selected(s); got == "" || strings.Contains(got, " ") {
		t.Fatalf("a double press selected %q", got)
	}
}

func selected(s *richeditor.Session) string {
	sel := s.State().Selection
	return s.State().Doc.TextBetween(sel.From(), sel.To(), "\n", "")
}

func TestViewComposes(t *testing.T) {
	s := session(t, "ab\n")
	app := nsgotest.Start(t, view(s), desktop())
	app.Press("id:rich-editor-content")
	s.Dispatch(s.State().Tr().SetSelection(richeditor.TextSelectionAt(s.State().Doc, 2, 2)))
	app.Compose("role:textbox", "ni")
	app.Find("text:anib")
	if got := s.Text(); got != "ab" {
		t.Fatalf("the document does not wait for the composition: %q", got)
	}
	app.Compose("role:textbox", "你")
	app.Type("role:textbox", "你")
	app.Compose("role:textbox", "")
	if got := s.Text(); got != "a你b" {
		t.Fatalf("committed %q", got)
	}
	if richeditor.UndoDepth(s.State()) != 1 {
		t.Fatalf("depth %d", richeditor.UndoDepth(s.State()))
	}
}

func TestViewDrawsTablesAndBubbleMenu(t *testing.T) {
	s := session(t, "| a | b |\n| --- | --- |\n| c | d |\n\nend\n")
	var shown *core.Signal[bool]
	app := nsgotest.Start(t, func(cx *core.Scope) core.Node {
		shown = core.NewSignal(cx, false)
		return view(s, func(v *richeditor.View) {
			v.BubbleMenu = func() core.Node {
				if shown.Get() {
					return core.Text{Content: "menu"}
				}
				return nil
			}
		})(cx)
	}, desktop())
	app.Settle()
	a, d := app.Find(`text:"a"`).Parent, app.Find(`text:"d"`).Parent
	if a.Frame.Origin.X == d.Frame.Origin.X || a.Frame.Origin.Y == d.Frame.Origin.Y {
		t.Fatalf("cells stacked: %+v %+v", a.Frame, d.Frame)
	}
	doc := s.State().Doc
	first := 2
	s.Run(richeditor.SetCellSelection(first, first+doc.Child(0).Child(0).Child(0).NodeSize()))
	app.Settle()
	if n := len(app.Query("id:rich-editor-selected-cell")); n != 2 {
		t.Fatalf("%d selected cells", n)
	}
	if !s.Run(richeditor.MergeCells) {
		t.Fatal("merge")
	}
	app.Settle()
	if n := len(app.Query("id:rich-editor-table-grip")); n != 4 {
		t.Fatalf("%d grips", n)
	}
	shown.Set(true)
	app.Settle()
	app.Find("text:menu")
}

func TestViewMovesAColumnByItsGrip(t *testing.T) {
	s := session(t, "| a | b |\n| --- | --- |\n| c | d |\n")
	app := nsgotest.Start(t, view(s), desktop())
	s.Dispatch(s.State().Tr().SetSelection(richeditor.TextSelectionAt(s.State().Doc, 4, 4)))
	app.Settle()
	grips := app.Query("id:rich-editor-table-grip")
	if len(grips) != 4 {
		t.Fatalf("%d grips", len(grips))
	}
	width := app.Find("id:rich-editor-content").Frame.Size.Width
	app.Drag("id:rich-editor-table-grip >> nth=0", core.Point{X: 0, Y: 0}, core.Point{X: width * 0.5, Y: 0}, core.Point{X: width * 0.75, Y: 0})
	app.Settle()
	row := s.State().Doc.Child(0).Child(0)
	if got := row.Child(0).TextContent(); got != "b" {
		t.Fatalf("the first title is %q after the drag: %s", got, s.State().Doc)
	}
	if _, ok := s.State().Selection.(*richeditor.CellSelection); !ok {
		t.Fatalf("selection %T, want the moved column's cells", s.State().Selection)
	}
}

func TestViewResizesAColumnByItsBand(t *testing.T) {
	// Three rows: a band's middle is then clear of the rows' bands.
	s := session(t, "| a | b |\n| --- | --- |\n| c | d |\n| e | f |\n")
	app := nsgotest.Start(t, view(s), desktop())
	app.Settle()
	app.Drag("id:rich-editor-table-band >> nth=0", core.Point{}, core.Point{X: -60, Y: 0})
	app.Settle()
	widths := richeditor.ColumnWidths(s.State().Doc.Child(0))
	if widths[0] != 190 {
		t.Fatalf("widths %v, want the first 250 less 60", widths)
	}
	if got := richeditor.UndoDepth(s.State()); got != 1 {
		t.Fatalf("a drag is one undo step, took %d", got)
	}
}

func TestViewDrawsAndFillsTheGapCursor(t *testing.T) {
	s := session(t, "| a | b |\n| --- | --- |\n| c | d |\n")
	app := nsgotest.Start(t, view(s), desktop())
	app.Press("id:rich-editor-content")
	s.Dispatch(s.State().Tr().SetSelection(richeditor.NewGapCursor(s.State().Doc.Resolve(0))))
	app.Settle()
	gap := app.Find("id:rich-editor-gap-cursor")
	table := app.Find(`text:"a"`).Parent.Parent
	if gap.Frame.Origin.Y > table.Frame.Origin.Y || gap.Frame.Size.Width != 20 {
		t.Fatalf("gap cursor %+v over the table %+v", gap.Frame, table.Frame)
	}
	app.Compose("role:textbox", "x")
	app.Type("role:textbox", "x")
	app.Compose("role:textbox", "")
	doc := s.State().Doc
	if doc.Child(0).Type.Name != "paragraph" || doc.Child(0).TextContent() != "x" || doc.Child(1).Type.Name != "table" {
		t.Fatalf("doc %s", doc)
	}
	if app.Exists("id:rich-editor-gap-cursor") {
		t.Fatal("the gap cursor stays after typing")
	}
}

func TestViewShowsWidgetDecorations(t *testing.T) {
	widgets := &richeditor.Plugin{Key: "widgets", Decorations: func(st *richeditor.State) []richeditor.Decoration {
		return []richeditor.Decoration{
			{From: 3, To: 3, Key: "after", Widget: func(*core.Scope) core.Node {
				return core.View{TestID: "widget", Style: core.Style{Width: core.Pt(30), Height: core.Pt(10)}}
			}},
			{From: 3, To: 3, Key: "before", Side: -1, Widget: func(*core.Scope) core.Node {
				return core.View{TestID: "widget-before", Style: core.Style{Width: core.Pt(30), Height: core.Pt(10)}}
			}},
		}
	}}
	exts := append(richeditor.StarterKit(richeditor.StarterOptions{}), richeditor.Extension{Name: "widgets", Plugins: func(*richeditor.Schema) []*richeditor.Plugin { return []*richeditor.Plugin{widgets} }})
	s, err := richeditor.NewSession(richeditor.SessionConfig{Extensions: exts, Markdown: "abcd\n"})
	if err != nil {
		t.Fatal(err)
	}
	app := nsgotest.Start(t, view(s), desktop())
	app.Press("id:rich-editor-content")
	s.Dispatch(s.State().Tr().SetSelection(richeditor.TextSelectionAt(s.State().Doc, 3, 3)))
	app.Settle()
	before, after := app.Find("id:widget-before"), app.Find("id:widget")
	caret := app.Find("id:rich-editor-caret")
	bx, ax, cx := pageX(before), pageX(after), pageX(caret)
	if !(bx+30 <= cx+1 && cx <= ax+1) {
		t.Fatalf("caret at %v between the widgets at %v and %v", cx, bx, ax)
	}
	app.Type("role:textbox", "Z")
	if got := s.Text(); got != "abZcd" {
		t.Fatalf("typed %q: widgets took positions", got)
	}
}

// pageX is a view's left in the surface.
func pageX(v *nsgotest.View) core.Points {
	x := v.Frame.Origin.X
	for p := v.Parent; p != nil; p = p.Parent {
		x += p.Frame.Origin.X - p.Offset.X
	}
	return x
}

func TestViewDragsABlockByAHandle(t *testing.T) {
	s := session(t, "one\n\ntwo\n\nthree\n")
	app := nsgotest.Start(t, view(s), desktop())
	app.Settle()
	if _, ok := s.PosAtPageCoords(0, 0); !ok {
		t.Fatal("no page geometry")
	}
	start, ok := s.PageCoordsAtPos(2)
	if !ok {
		t.Fatal("no page coordinates")
	}
	if pos, _ := s.PosAtPageCoords(start.X, start.Y+start.Height/2); pos != 2 {
		t.Fatalf("page point maps to %d", pos)
	}
	var drop richeditor.Rect2
	app.Post(func() {
		if !s.DragBlock(2, start.X-20, start.Y) {
			t.Error("the drag did not start")
		}
		end := s.State().Doc.Content.Size - 1
		last, _ := s.PageCoordsAtPos(end)
		drop = last
		s.DragTo(last.X-20, last.Y+last.Height-1)
	})
	app.Settle()
	cursor := app.Find("id:rich-editor-drop-cursor")
	if y := float64(cursor.Frame.Origin.Y); y < drop.Y {
		t.Fatalf("drop cursor at %v, above the last line at %v", y, drop.Y)
	}
	app.Post(func() {
		if !s.EndDrag() {
			t.Error("not dropped")
		}
	})
	app.Settle()
	if got := s.Text(); got != "two\n\nthree\n\none" {
		t.Fatalf("dropped to %q", got)
	}
	if app.Exists("id:rich-editor-drop-cursor") {
		t.Fatal("the drop cursor stays")
	}
	if got := richeditor.UndoDepth(s.State()); got != 1 {
		t.Fatalf("a drop is one undo step, took %d", got)
	}
}
