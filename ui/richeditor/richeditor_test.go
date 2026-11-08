package richeditor_test

import (
	"strings"
	"testing"

	"github.com/oneo-me/nsgo/base/richeditor"
	"github.com/oneo-me/nsgo/core"
	"github.com/oneo-me/nsgo/nsgotest"
	"github.com/oneo-me/nsgo/ui"
	uirich "github.com/oneo-me/nsgo/ui/richeditor"
)

// TestViewTakesTyping lays a document out in ui's look and types into it.
func TestViewTakesTyping(t *testing.T) {
	s, err := richeditor.NewSession(richeditor.SessionConfig{
		Extensions: append(richeditor.StarterKit(richeditor.StarterOptions{}), richeditor.TableKit(richeditor.TableOptions{})),
		Markdown:   "# Title\n\nSome **bold** text\n\n| a | b |\n| --- | --- |\n| c | d |\n"})
	if err != nil {
		t.Fatal(err)
	}
	app := nsgotest.Start(t, func(cx *core.Scope) core.Node {
		cx.SetTheme(ui.DefaultTheme())
		return uirich.View{Editor: s, Label: "Doc", Style: core.Style{Width: core.Pt(360), Height: core.Pt(400)}}
	})
	app.Find("text:Title")
	if h := app.Find("text:Title").Frame.Size.Height; h < 30 {
		t.Fatalf("a heading's line is %v tall", h)
	}
	if a, d := app.Find(`text:"a"`).Parent, app.Find(`text:"d"`).Parent; a.Frame.Origin.X == d.Frame.Origin.X {
		t.Fatalf("the table's cells stack: %+v %+v", a.Frame, d.Frame)
	}
	app.Press("text:Some bold text")
	app.Settle()
	if _, ok := s.State().Selection.(*richeditor.TextSelection); !ok || s.State().Selection.FromPos().Parent().TextContent() != "Some bold text" {
		t.Fatalf("a press landed in %q", s.State().Selection.FromPos().Parent().TextContent())
	}
	app.Type("role:textbox", "!")
	if !strings.Contains(s.Text(), "!") {
		t.Fatalf("typing left %q", s.Text())
	}
}
